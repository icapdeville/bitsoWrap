package handlers

import (
	"net/http"
)

// GetWithdrawalsHandler expone /withdrawals. Acepta limit, marker, wids, origin_ids, status, method, currency.
func GetWithdrawalsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	client, ok := clientFromHeaders(w, r)
	if !ok {
		return
	}

	resp, err := client.ListWithdrawals(queryParams(r))
	if err != nil {
		respondBitsoError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
