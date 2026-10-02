package bitso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) *BitsoClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient("key", "secret")
	c.BaseURL = srv.URL + "/v3"
	PageDelay = 0
	return c
}

func TestSignatureIncludesQuery(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bitso "), ":")
		if len(parts) != 3 || parts[0] != "key" {
			t.Fatalf("Authorization inválido: %q", r.Header.Get("Authorization"))
		}
		mac := hmac.New(sha256.New, []byte("secret"))
		mac.Write([]byte(parts[1] + r.Method + r.URL.RequestURI()))
		if want := hex.EncodeToString(mac.Sum(nil)); parts[2] != want {
			t.Fatalf("firma no coincide para %s", r.URL.RequestURI())
		}
		fmt.Fprint(w, `{"success":true,"payload":[]}`)
	})

	if _, err := c.ListFundings(map[string]string{"status": "complete", "limit": "5"}); err != nil {
		t.Fatal(err)
	}
}

func TestAPIErrorNotRetriedOn4xx(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"success":false,"error":{"code":"0201","message":"Invalid Nonce"}}`)
	})

	_, err := c.ListUserTrades(nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "0201" || apiErr.StatusCode != 400 {
		t.Fatalf("se esperaba APIError 0201, llegó %v", err)
	}
	if calls != 1 {
		t.Fatalf("no debe reintentar 4xx, hubo %d llamadas", calls)
	}
}

func TestRetryOn429(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"success":true,"payload":{"balances":[]}}`)
	})

	if _, err := c.Balances(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("se esperaban 2 llamadas, hubo %d", calls)
	}
}

func TestSuccessFalseIsError(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":false,"error":{"code":"0301","message":"Unknown OrderBook"}}`)
	})

	_, err := c.ListOpenOrders(nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "0301" {
		t.Fatalf("se esperaba APIError 0301, llegó %v", err)
	}
}

func TestPostNotRetried(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	if _, err := c.PlaceOrder(map[string]string{"book": "sol_mxn"}); err == nil {
		t.Fatal("se esperaba error")
	}
	if calls != 1 {
		t.Fatalf("un POST no debe reintentarse, hubo %d llamadas", calls)
	}
}

func TestWalkUserTradesPaginates(t *testing.T) {
	// 250 trades con tid 1..250; sort=asc y marker devuelven los posteriores.
	var markers []string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		markers = append(markers, q.Get("marker"))
		start, _ := strconv.Atoi(q.Get("marker"))
		limit, _ := strconv.Atoi(q.Get("limit"))

		var page []map[string]interface{}
		for tid := start + 1; tid <= 250 && len(page) < limit; tid++ {
			page = append(page, map[string]interface{}{"tid": tid, "book": "sol_mxn"})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "payload": page})
	})

	var got []string
	err := c.WalkUserTrades(map[string]string{"sort": "asc"}, func(page []UserTrade) (bool, error) {
		for _, tr := range page {
			got = append(got, string(tr.Tid))
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 250 || got[0] != "1" || got[249] != "250" {
		t.Fatalf("trades inesperados: %d (%v...)", len(got), got[:3])
	}
	if strings.Join(markers, ",") != ",100,200" {
		t.Fatalf("markers inesperados: %v", markers)
	}
}

func TestWalkStopsWhenCallbackReturnsFalse(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var page []map[string]string
		for i := 0; i < 100; i++ {
			page = append(page, map[string]string{"fid": fmt.Sprintf("f%d-%d", calls, i)})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "payload": page})
	})

	err := c.WalkFundings(nil, func(page []Funding) (bool, error) { return false, nil })
	if err != nil || calls != 1 {
		t.Fatalf("debió detenerse tras 1 página: calls=%d err=%v", calls, err)
	}
}

func TestFlexString(t *testing.T) {
	var tr UserTrade
	for _, in := range []string{`{"tid":123}`, `{"tid":"123"}`} {
		if err := json.Unmarshal([]byte(in), &tr); err != nil || tr.Tid != "123" {
			t.Fatalf("%s -> %q, %v", in, tr.Tid, err)
		}
	}
	out, _ := json.Marshal(tr)
	if !strings.Contains(string(out), `"tid":"123"`) {
		t.Fatalf("tid debe serializarse como string: %s", out)
	}
}

func TestNonceIsStrictlyIncreasing(t *testing.T) {
	prev := int64(0)
	for i := 0; i < 1000; i++ {
		n, _ := strconv.ParseInt(nextNonce(), 10, 64)
		if n <= prev {
			t.Fatalf("nonce no creciente: %d <= %d", n, prev)
		}
		prev = n
	}
}
