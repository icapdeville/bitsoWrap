package handlers

import (
	"bitsoWrap/internal/bitso"
	"net/http"
)

// TickerHandler devuelve el ticker de un libro (?book=sol_mxn) o de todos si no se manda book.
func TickerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	client := bitso.NewClient("", "")

	book := r.URL.Query().Get("book")
	if book == "" {
		tickers, err := client.GetTickers()
		if err != nil {
			respondBitsoError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, tickers)
		return
	}

	rawResponse, err := client.GetTicker(book)
	if err != nil {
		respondBitsoError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(rawResponse)
}
