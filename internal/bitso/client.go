package bitso

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://api.bitso.com/v3"
	defaultTimeout = 30 * time.Second
	maxRetries     = 3
)

type BitsoClient struct {
	Key     string
	Secret  string
	BaseURL string
	HTTP    *http.Client
	// Debug imprime los bodies de request/response (nunca los headers).
	Debug bool
}

func NewClient(key, secret string) *BitsoClient {
	return &BitsoClient{
		Key:     key,
		Secret:  secret,
		BaseURL: defaultBaseURL,
		HTTP:    &http.Client{Timeout: defaultTimeout},
		Debug:   os.Getenv("BITSO_DEBUG") == "1",
	}
}

// NewClientFromEnv crea un cliente con BITSO_API_KEY y BITSO_API_SECRET.
func NewClientFromEnv() (*BitsoClient, error) {
	key := os.Getenv("BITSO_API_KEY")
	secret := os.Getenv("BITSO_API_SECRET")
	if key == "" || secret == "" {
		return nil, errors.New("faltan BITSO_API_KEY o BITSO_API_SECRET")
	}
	return NewClient(key, secret), nil
}

// Bitso exige que el nonce sea estrictamente creciente por API key.
// Usar solo el reloj en ms puede repetir valores con peticiones concurrentes.
var (
	nonceMu   sync.Mutex
	lastNonce int64
)

func nextNonce() string {
	nonceMu.Lock()
	defer nonceMu.Unlock()
	n := time.Now().UnixMilli()
	if n <= lastNonce {
		n = lastNonce + 1
	}
	lastNonce = n
	return strconv.FormatInt(n, 10)
}

// Request hace la petición a Bitso. Si la respuesta no es 2xx devuelve el body
// junto con un *APIError, para que quien llama pueda seguir leyendo el detalle.
// Las peticiones GET se reintentan ante 429/5xx o errores de red; los POST no,
// para no duplicar órdenes.
func (c *BitsoClient) Request(endpoint, method string, params map[string]string, private bool) ([]byte, error) {
	requestPath := "/v3/" + endpoint
	requestURL := c.BaseURL + "/" + endpoint

	var jsonData []byte
	if method == http.MethodGet && len(params) > 0 {
		queryVals := url.Values{}
		for k, v := range params {
			queryVals.Add(k, v)
		}
		qs := queryVals.Encode()
		requestURL += "?" + qs
		requestPath += "?" + qs
	}
	if method == http.MethodPost {
		var err error
		jsonData, err = json.Marshal(params)
		if err != nil {
			return nil, err
		}
	}

	attempts := 1
	if method == http.MethodGet {
		attempts = maxRetries + 1
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			wait := time.Duration(1<<(i-1)) * time.Second
			var apiErr *APIError
			if errors.As(lastErr, &apiErr) && apiErr.RetryAfter > 0 {
				wait = apiErr.RetryAfter
			}
			log.Printf("Bitso %s %s reintento %d en %s: %v", method, endpoint, i, wait, lastErr)
			time.Sleep(wait)
		}

		body, err := c.do(method, requestURL, requestPath, jsonData, private)
		if err == nil {
			return body, nil
		}
		lastErr = err

		var apiErr *APIError
		if errors.As(err, &apiErr) && !apiErr.Retryable() {
			return body, err
		}
		if i == attempts-1 {
			return body, err
		}
	}
	return nil, lastErr
}

func (c *BitsoClient) do(method, requestURL, requestPath string, jsonData []byte, private bool) ([]byte, error) {
	var body io.Reader
	if jsonData != nil {
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}

	if private {
		nonce := nextNonce()
		msg := nonce + method + requestPath + string(jsonData)
		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write([]byte(msg))
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("Authorization", fmt.Sprintf("Bitso %s:%s:%s", c.Key, nonce, signature))
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Debug && jsonData != nil {
		log.Printf("Bitso REQUEST %s %s body=%s", method, requestPath, jsonData)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("Bitso %s %s status=%d %s", method, requestPath, resp.StatusCode, time.Since(start).Round(time.Millisecond))
	if c.Debug {
		log.Printf("Bitso RESPONSE body=%s", respBody)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return respBody, newAPIError(resp, respBody)
	}
	return respBody, nil
}

// APIError representa una respuesta no exitosa de Bitso.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("bitso: HTTP %d código %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("bitso: HTTP %d: %s", e.StatusCode, e.Message)
}

func (e *APIError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

func newAPIError(resp *http.Response, body []byte) *APIError {
	apiErr := &APIError{StatusCode: resp.StatusCode}

	var parsed struct {
		Error *BitsoError `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != nil {
		apiErr.Code = parsed.Error.Code
		apiErr.Message = parsed.Error.Message
	} else {
		apiErr.Message = strings.TrimSpace(string(body))
		if len(apiErr.Message) > 200 {
			apiErr.Message = apiErr.Message[:200]
		}
	}

	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		apiErr.RetryAfter = time.Duration(s) * time.Second
	}
	return apiErr
}

// getJSON hace un GET y decodifica la respuesta en out. Si Bitso responde
// success=false con HTTP 200 también se devuelve un *APIError.
func (c *BitsoClient) getJSON(endpoint string, params map[string]string, private bool, out envelope) error {
	data, err := c.Request(endpoint, http.MethodGet, params, private)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("bitso: error al decodificar %s: %w", endpoint, err)
	}
	if !out.ok() {
		apiErr := &APIError{StatusCode: http.StatusOK, Message: "success=false"}
		if e := out.err(); e != nil {
			apiErr.Code = e.Code
			apiErr.Message = e.Message
		}
		return apiErr
	}
	return nil
}

func (c *BitsoClient) GetBalance() ([]byte, error) {
	return c.Request("balance", http.MethodGet, nil, true)
}

func (c *BitsoClient) Balances() (*BitsoResponse, error) {
	var resp BitsoResponse
	if err := c.getJSON("balance", nil, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *BitsoClient) GetTicker(book string) ([]byte, error) {
	params := map[string]string{
		"book": book,
	}

	return c.Request("ticker", http.MethodGet, params, false)
}

// GetTickers devuelve el ticker de todos los libros en una sola llamada.
func (c *BitsoClient) GetTickers() (*TickersResponse, error) {
	var resp TickersResponse
	if err := c.getJSON("ticker", nil, false, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *BitsoClient) PlaceOrder(orderParams map[string]string) ([]byte, error) {
	return c.Request("orders", http.MethodPost, orderParams, true)
}

func (c *BitsoClient) GetUserTrades(params map[string]string) ([]byte, error) {
	return c.Request("user_trades", http.MethodGet, params, true)
}

func (c *BitsoClient) ListUserTrades(params map[string]string) (*UserTradesResponse, error) {
	var resp UserTradesResponse
	if err := c.getJSON("user_trades", params, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *BitsoClient) GetOpenOrders(params map[string]string) ([]byte, error) {
	return c.Request("open_orders", http.MethodGet, params, true)
}

func (c *BitsoClient) ListOpenOrders(params map[string]string) (*OpenOrdersResponse, error) {
	var resp OpenOrdersResponse
	if err := c.getJSON("open_orders", params, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListFundings consulta /fundings (depósitos). Parámetros: limit, marker, fids, status, method.
func (c *BitsoClient) ListFundings(params map[string]string) (*FundingsResponse, error) {
	var resp FundingsResponse
	if err := c.getJSON("fundings", params, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListWithdrawals consulta /withdrawals. Parámetros: limit, marker, wids, origin_ids, status, method, currency.
func (c *BitsoClient) ListWithdrawals(params map[string]string) (*WithdrawalsResponse, error) {
	var resp WithdrawalsResponse
	if err := c.getJSON("withdrawals", params, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFees devuelve las comisiones vigentes de la cuenta por libro.
func (c *BitsoClient) GetFees() (*FeesResponse, error) {
	var resp FeesResponse
	if err := c.getJSON("fees", nil, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
