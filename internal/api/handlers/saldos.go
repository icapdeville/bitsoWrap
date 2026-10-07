package handlers

import (
	"bitsoWrap/internal/bitso"
	"crypto/subtle"
	"net/http"
	"os"
	"sync"
	"time"
)

const saldosCacheTTL = 60 * time.Second

var saldosCache struct {
	sync.Mutex
	entries map[string]saldosEntry
}

type saldosEntry struct {
	at   time.Time
	data *bitso.ValuedBalances
}

// SaldosHandler devuelve los saldos por moneda valuados en MXN.
//
//	GET /saldos  -> {"fecha", "total_mxn", "saldos": [{"coin","saldo","disponible","precio_mxn","saldo_mxn"}]}
func SaldosHandler(w http.ResponseWriter, r *http.Request) {
	data, ok := valuedBalances(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"fecha":     data.Fecha,
		"total_mxn": data.TotalMXN,
		"saldos":    data.Saldos,
	})
}

// SaldoHandler devuelve solo el saldo total de Bitso en MXN.
//
//	GET /saldo  -> {"fecha", "total_mxn"}
func SaldoHandler(w http.ResponseWriter, r *http.Request) {
	data, ok := valuedBalances(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"fecha":     data.Fecha,
		"total_mxn": data.TotalMXN,
	})
}

func valuedBalances(w http.ResponseWriter, r *http.Request) (*bitso.ValuedBalances, bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return nil, false
	}

	client, ok := saldosClient(w, r)
	if !ok {
		return nil, false
	}

	saldosCache.Lock()
	defer saldosCache.Unlock()
	if saldosCache.entries == nil {
		saldosCache.entries = map[string]saldosEntry{}
	}
	if e, ok := saldosCache.entries[client.Key]; ok && time.Since(e.at) < saldosCacheTTL {
		return e.data, true
	}

	data, err := client.GetValuedBalances()
	if err != nil {
		respondBitsoError(w, err)
		return nil, false
	}
	saldosCache.entries[client.Key] = saldosEntry{at: time.Now(), data: data}
	return data, true
}

// saldosClient usa las credenciales de los headers si vienen; si no, la key de
// solo lectura del entorno (BITSO_API_KEY/SECRET). En ese caso, si está definido
// WRAPPER_TOKEN, se exige en el header X-Wrapper-Token.
func saldosClient(w http.ResponseWriter, r *http.Request) (*bitso.BitsoClient, bool) {
	if r.Header.Get("X-API-KEY") != "" || r.Header.Get("X-API-SECRET") != "" {
		return clientFromHeaders(w, r)
	}

	if !wrapperTokenOK(w, r) {
		return nil, false
	}

	client, err := bitso.NewClientFromEnv()
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Faltan credenciales"})
		return nil, false
	}
	return client, true
}

// wrapperTokenOK exige X-Wrapper-Token cuando WRAPPER_TOKEN está definido; si
// no coincide, responde 401.
func wrapperTokenOK(w http.ResponseWriter, r *http.Request) bool {
	token := os.Getenv("WRAPPER_TOKEN")
	if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Wrapper-Token")), []byte(token)) == 1 {
		return true
	}
	respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Token inválido"})
	return false
}
