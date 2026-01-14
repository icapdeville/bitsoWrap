package bitso

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type BitsoClient struct {
	Key     string
	Secret  string
	BaseURL string
}

func NewClient(key, secret string) *BitsoClient {
	return &BitsoClient{
		Key:     key,
		Secret:  secret,
		BaseURL: "https://api.bitso.com/v3",
	}
}

func (c *BitsoClient) Request(endpoint, method string, params map[string]string, private bool) ([]byte, error) {
	var body io.Reader
	var jsonData []byte
	requestURL := c.BaseURL + "/" + endpoint

	// build query values once so we can reuse both for URL and for signing
	queryVals := url.Values{}
	if method == "GET" && len(params) > 0 {
		for k, v := range params {
			queryVals.Add(k, v)
		}
		qs := queryVals.Encode()
		requestURL += "?" + qs
		fmt.Println(requestURL)
	}

	if method == "POST" {
		jsonData, _ = json.Marshal(params)
		body = bytes.NewBuffer(jsonData)
	}

	// use milliseconds as nonce (stable and readable)
	nonce := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	signature := ""
	if private {
		payloadStr := ""

		if method == "POST" {
			payloadStr = string(jsonData)
		}

		// include query string in requestPath for GET requests (Bitso requires this)
		requestPath := "/v3/" + endpoint
		if method == "GET" && len(queryVals) > 0 {
			requestPath += "?" + queryVals.Encode()
		}
		msg := nonce + method + requestPath + payloadStr

		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write([]byte(msg))
		signature = hex.EncodeToString(mac.Sum(nil))
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}

	if private {
		auth := fmt.Sprintf("Bitso %s:%s:%s", c.Key, nonce, signature)
		req.Header.Set("Authorization", auth)
	}

	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// antes de enviar la petición HTTP:
	log.Printf("Bitso REQUEST: %s %s headers=%+v body=%s", method, requestURL, req.Header, string(jsonData))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// después de leer la respuesta:
	log.Printf("Bitso RESPONSE status=%d body=%s", resp.StatusCode, string(respBody))

	return respBody, nil
}

func (c *BitsoClient) GetBalance() ([]byte, error) {
	return c.Request("balance", "GET", nil, true)
}

func (c *BitsoClient) GetTicker(book string) ([]byte, error) {
	params := map[string]string{
		"book": book,
	}

	return c.Request("ticker", "GET", params, false)
}

func (c *BitsoClient) PlaceOrder(orderParams map[string]string) ([]byte, error) {

	return c.Request("orders", "POST", orderParams, true)
}

func (c *BitsoClient) GetUserTrades(params map[string]string) ([]byte, error) {

	return c.Request("user_trades", "GET", params, true)
}

func (c *BitsoClient) ListUserTrades(params map[string]string) (*UserTradesResponse, error) {
	data, err := c.GetUserTrades(params)
	if err != nil {
		return nil, err
	}

	var resp UserTradesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *BitsoClient) GetOpenOrders(params map[string]string) ([]byte, error) {
	return c.Request("open_orders", "GET", params, true)
}

func (c *BitsoClient) ListOpenOrders(params map[string]string) (*OpenOrdersResponse, error) {
	data, err := c.GetOpenOrders(params)
	if err != nil {
		return nil, err
	}

	var resp OpenOrdersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
