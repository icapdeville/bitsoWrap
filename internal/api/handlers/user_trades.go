package handlers

import (
	"bitsoWrap/internal/bitso"
	"encoding/json"
	"log"
	"net/http"
)

func GetUserTradesHandler(w http.ResponseWriter, r *http.Request) {

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

	trades, err := client.ListUserTrades(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var respMap map[string]interface{}
	b, _ := json.Marshal(trades)
	_ = json.Unmarshal(b, &respMap)
	if v, ok := respMap["success"]; ok {
		if success, _ := v.(bool); !success {
			log.Printf("Bitso API returned success=false: params=%+v resp=%+v", params, respMap)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(respMap)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trades)
}
