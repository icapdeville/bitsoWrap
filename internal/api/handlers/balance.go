package handlers

import (
	"bitsoWrap/internal/bitso"
	"net/http"

	"github.com/shopspring/decimal"
)

func BalanceHandler(w http.ResponseWriter, r *http.Request) {
	client, ok := clientFromHeaders(w, r)
	if !ok {
		return
	}

	bitsoResp, err := client.Balances()
	if err != nil {
		respondBitsoError(w, err)
		return
	}

	var filteredBalances []bitso.BalanceItem

	for _, balance := range bitsoResp.Payload.Balances {
		total, err := decimal.NewFromString(balance.Total)
		if err != nil {
			continue
		}

		if total.GreaterThan(decimal.Zero) {
			filteredBalances = append(filteredBalances, balance)
		}
	}

	finalPayload := map[string]interface{}{
		"success": bitsoResp.Success,
		"payload": map[string]interface{}{
			"balances": filteredBalances,
		},
	}

	respondJSON(w, http.StatusOK, finalPayload)
}
