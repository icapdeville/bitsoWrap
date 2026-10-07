package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSaldosRequiresWrapperToken(t *testing.T) {
	t.Setenv("WRAPPER_TOKEN", "secreto")
	t.Setenv("BITSO_API_KEY", "k")
	t.Setenv("BITSO_API_SECRET", "s")

	for _, tc := range []struct {
		token string
		ok    bool
	}{{"", false}, {"otro", false}, {"secreto", true}} {
		req := httptest.NewRequest(http.MethodGet, "/saldo", nil)
		if tc.token != "" {
			req.Header.Set("X-Wrapper-Token", tc.token)
		}
		rec := httptest.NewRecorder()
		_, ok := saldosClient(rec, req)
		if ok != tc.ok {
			t.Errorf("token %q: ok=%v, se esperaba %v", tc.token, ok, tc.ok)
		}
		if !tc.ok && rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, se esperaba 401", tc.token, rec.Code)
		}
	}
}
