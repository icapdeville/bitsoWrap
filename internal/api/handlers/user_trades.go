package handlers

import (
	"net/http"
)

func GetUserTradesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	client, ok := clientFromHeaders(w, r)
	if !ok {
		return
	}

	trades, err := client.ListUserTrades(queryParams(r))
	if err != nil {
		respondBitsoError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, trades)
}
