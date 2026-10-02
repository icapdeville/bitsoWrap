package handlers

import (
	"bitsoWrap/internal/bitso"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// respondBitsoError responde con el error de Bitso (502) o con un error interno (500).
func respondBitsoError(w http.ResponseWriter, err error) {
	var apiErr *bitso.APIError
	if errors.As(err, &apiErr) {
		log.Printf("Bitso error: %v", apiErr)
		respondJSON(w, http.StatusBadGateway, map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":        apiErr.Code,
				"message":     apiErr.Message,
				"http_status": apiErr.StatusCode,
			},
		})
		return
	}
	log.Printf("Error al conectar con Bitso: %v", err)
	respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Error al conectar con Bitso"})
}

// clientFromHeaders crea el cliente con X-API-KEY / X-API-SECRET o responde 401.
func clientFromHeaders(w http.ResponseWriter, r *http.Request) (*bitso.BitsoClient, bool) {
	key := r.Header.Get("X-API-KEY")
	secret := r.Header.Get("X-API-SECRET")
	if key == "" || secret == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Faltan credenciales"})
		return nil, false
	}
	return bitso.NewClient(key, secret), true
}

// queryParams toma el primer valor de cada parámetro del query string.
func queryParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	return params
}

func CalculateDecimalPrecision(valueStr string) int {
	trimmedStr := strings.TrimRight(valueStr, "0")

	parts := strings.Split(trimmedStr, ".")

	if len(parts) < 2 {
		return 0
	}

	precision := len(parts[1])

	return precision
}
