package handlers

import (
	"bitsoWrap/internal/bitso"
	"net/http"
	"encoding/json"
	"github.com/shopspring/decimal"

)


func BalanceHandler(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("X-API-KEY")
	secret := r.Header.Get("X-API-SECRET")

	if key == "" || secret == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Faltan credenciales"})
		return
	}

	client := bitso.NewClient(key, secret)
	data, err := client.GetBalance()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Error al conectar con Bitso"})
		return
	}

	var bitsoResp bitso.BitsoResponse
	if err := json.Unmarshal(data, &bitsoResp); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Error al parsear respuesta de Bitso"})
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
