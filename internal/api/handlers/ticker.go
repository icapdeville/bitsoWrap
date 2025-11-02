package handlers

import (
    "bitsoWrap/internal/bitso" 
    "net/http"
    "fmt"
)

func TickerHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
        return
    }

    client := bitso.NewClient("", "") 
    
    book := r.URL.Query().Get("book")
    if book == "" {
        http.Error(w, "Falta el parámetro 'book'", http.StatusBadRequest)
        return
    }

    rawResponse, err := client.GetTicker(book)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error al obtener ticker: %s", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(rawResponse)
}
