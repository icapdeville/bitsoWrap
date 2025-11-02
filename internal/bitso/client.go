package bitso

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
)


type BitsoClient struct {
    Key    string
    Secret string
    BaseURL string
}


func NewClient(key, secret string) *BitsoClient {
    return &BitsoClient{
        Key:    key,
        Secret: secret,
        BaseURL: "https://api.bitso.com/v3",
    }
}


func (c *BitsoClient) Request(endpoint, method string, params map[string]string, private bool) ([]byte, error) {
	var body io.Reader
	var jsonData []byte
	requestURL := c.BaseURL + "/" + endpoint

	if method == "GET" && len(params) > 0 {
		query := url.Values{} 
		for k, v := range params {
			query.Add(k, v)
		}
		requestURL += "?" + query.Encode() 
	}

	if method == "POST" {
		jsonData, _ = json.Marshal(params)
		body = bytes.NewBuffer(jsonData)
	}

	nonce := fmt.Sprintf("%d", time.Now().UnixNano()/int64(time.Millisecond/10))
    signature := ""
    if private {
        payloadStr := "" 
        
        if method == "POST" {
            payloadStr = string(jsonData)
        }
        
		requestPath := "/v3/" + endpoint + "/"
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

    return io.ReadAll(resp.Body)
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
