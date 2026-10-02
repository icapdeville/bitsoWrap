package handlers

import (
	"bitsoWrap/internal/bitso"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type WrapperOrderResponse struct {
	Success bool `json:"success"`

	// Datos de ÉXITO
	Oid   string `json:"oid,omitempty"`
	Side  string `json:"side,omitempty"`
	Type  string `json:"type,omitempty"`
	Major string `json:"major,omitempty"`
	Price string `json:"price,omitempty"`

	// Datos de ERROR (provenientes de Bitso)
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

const SlippagePercent = 0.00066

type OrderParams map[string]string

func PlaceOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var params OrderParams
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "Error al decodificar el cuerpo JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	tickerClient := bitso.NewClient("", "")

	if params["type"] == "limit" && params["side"] == "buy" {

		precisionStr := params["mr"]
		if precisionStr == "" {
			http.Error(w, "Falta el parámetro de precisión 'mr'", http.StatusBadRequest)
			return
		}

		majorPrecision, err := strconv.Atoi(precisionStr)
		if err != nil {
			http.Error(w, "El valor de precisión no es un número entero válido", http.StatusBadRequest)
			return
		}

		precisionP := params["pr"]
		if precisionP == "" {
			http.Error(w, "Falta el parámetro de precisión 'pr'", http.StatusBadRequest)
			return
		}

		pricePrecision, err := strconv.Atoi(precisionP)
		if err != nil {
			http.Error(w, "El valor de precisión no es un número entero válido", http.StatusBadRequest)
			return
		}

		minorAmountStr := params["amount"]

		price, err := tickerClient.GetBidPrice(params["book"])
		if err != nil {
			http.Error(w, fmt.Sprintf("Error al obtener precio de referencia: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		adjustedPrice := price * (1.0 - SlippagePercent)
		minorAmount, _ := strconv.ParseFloat(minorAmountStr, 64)
		majorAmount := minorAmount / adjustedPrice

		keysToDelete := []string{"minor", "amount", "mr", "pr"}

		for _, key := range keysToDelete {
			delete(params, key)
		}

		params["major"] = fmt.Sprintf("%.*f", majorPrecision, majorAmount)
		params["price"] = fmt.Sprintf("%.*f", pricePrecision, adjustedPrice)

	} else if params["type"] == "limit" && params["side"] == "sell" {

		majorAmountStr := params["amount"]
		price, err := tickerClient.GetAskPrice(params["book"])
		if err != nil {
			http.Error(w, fmt.Sprintf("Error al obtener precio ASK: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		adjustedPrice := price * (1.0 + SlippagePercent)
		pricePrecision, err := strconv.Atoi(params["pr"])
		if err != nil {
			http.Error(w, "Falta o es inválido el parámetro de precisión 'pr'", http.StatusBadRequest)
			return
		}

		keysToDelete := []string{"minor", "amount", "mr", "pr"}
		for _, key := range keysToDelete {
			delete(params, key)
		}

		params["major"] = majorAmountStr
		params["price"] = fmt.Sprintf("%.*f", pricePrecision, adjustedPrice)

	}

	key := r.Header.Get("X-API-KEY")
	secret := r.Header.Get("X-API-SECRET")

	if key == "" || secret == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Faltan credenciales"})
		return
	}

	client := bitso.NewClient(key, secret)
	responseBody, err := client.PlaceOrder(params)

	// Un *APIError trae el body de Bitso con el código de error; se sigue
	// procesando para devolver error_code/error_message como antes.
	var apiErr *bitso.APIError
	if err != nil && (!errors.As(err, &apiErr) || len(responseBody) == 0) {
		http.Error(w, fmt.Sprintf("Error en la petición a Bitso: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	var bitsoResp bitso.BitsoOrderResponse
	if err := json.Unmarshal(responseBody, &bitsoResp); err != nil {
		http.Error(w, "Error al procesar la respuesta de Bitso", http.StatusInternalServerError)
		return
	}

	finalResponse := WrapperOrderResponse{
		Success: bitsoResp.Success,
	}

	if bitsoResp.Success {

		finalResponse.Oid = bitsoResp.Payload.Oid
		finalResponse.Side = params["side"]
		finalResponse.Type = params["type"]
		finalResponse.Major = params["major"]
		finalResponse.Price = params["price"]

	} else {
		if bitsoResp.Error != nil {
			finalResponse.ErrorCode = bitsoResp.Error.Code
			finalResponse.ErrorMessage = bitsoResp.Error.Message
			fmt.Printf("Bitso Error: Code=%s, Message=%s\n", finalResponse.ErrorCode, finalResponse.ErrorMessage)
		} else {
			finalResponse.ErrorMessage = "La orden falló, pero Bitso no proporcionó detalles del error."
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(finalResponse)

}
