package bitso

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func (c *BitsoClient) GetBidPrice(book string) (float64, error) {

	rawResponse, err := c.GetTicker(book)
	if err != nil {
		return 0, err
	}

	var tickerResp TickerResponse
	if err := json.Unmarshal(rawResponse, &tickerResp); err != nil {
		return 0, fmt.Errorf("error al decodificar respuesta del ticker: %w", err)
	}

	price, err := strconv.ParseFloat(tickerResp.Payload.Bid, 64)
	if err != nil {
		return 0, fmt.Errorf("error al convertir precio 'bid' a float: %w", err)
	}

	return price, nil
}

func (c *BitsoClient) GetAskPrice(book string) (float64, error) {
	rawResponse, err := c.GetTicker(book)
	if err != nil {
		return 0, err
	}

	var tickerResp TickerResponse
	if err := json.Unmarshal(rawResponse, &tickerResp); err != nil {
		return 0, fmt.Errorf("error al decodificar respuesta del ticker: %w", err)
	}

	price, err := strconv.ParseFloat(tickerResp.Payload.Ask, 64)
	if err != nil {
		return 0, fmt.Errorf("error al convertir precio 'bid' a float: %w", err)
	}

	return price, nil
}
