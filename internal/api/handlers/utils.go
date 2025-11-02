package handlers

import (
    "encoding/json"
    "net/http"
	"strings"

)

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(payload)
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


