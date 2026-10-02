package handlers

import (
	"net/http"
)

// GetFundingsHandler expone /fundings (depósitos). Acepta limit, marker, fids, status, method.
func GetFundingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	client, ok := clientFromHeaders(w, r)
	if !ok {
		return
	}

	resp, err := client.ListFundings(queryParams(r))
	if err != nil {
		respondBitsoError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
