package handlers

import (
    "bitsoWrap/internal/bitso"
    "encoding/json"
    "log"
    "net/http"
)

func GetOpenOrdersHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
        return
    }

    key := r.Header.Get("X-API-KEY")
    secret := r.Header.Get("X-API-SECRET")
    if key == "" || secret == "" {
        respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Faltan credenciales"})
        return
    }

    client := bitso.NewClient(key, secret)

    query := r.URL.Query()
    params := make(map[string]string)
    for k, v := range query {
        if len(v) > 0 {
            params[k] = v[0]
        }
    }

    resp, err := client.ListOpenOrders(params)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if !resp.Success {
        log.Printf("Bitso API returned success=false: params=%+v resp=%+v", params, resp)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadGateway)
        json.NewEncoder(w).Encode(resp)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}