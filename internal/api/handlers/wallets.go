package handlers

import (
	"bitsoWrap/internal/bitso"
	"bitsoWrap/internal/store"
	"bitsoWrap/internal/syncer"
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

// Valuación en caché por wallet: el panel no necesita más de un precio por
// minuto. Un PUT invalida la de su wallet.
var walletCache struct {
	sync.Mutex
	entries map[string]walletEntry
}

type walletEntry struct {
	at   time.Time
	data *bitso.WalletValues
}

var (
	coinRe   = regexp.MustCompile(`^[a-z0-9]{1,10}$`)
	walletRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
)

// walletName toma {wallet} de la ruta o responde 400.
func walletName(w http.ResponseWriter, r *http.Request) (string, bool) {
	wallet := r.PathValue("wallet")
	if !walletRe.MatchString(wallet) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Nombre de wallet inválido"})
		return "", false
	}
	return wallet, true
}

// GetWalletHandler devuelve las monedas de una wallet fuera de Bitso valuadas en MXN.
//
//	GET /wallets/cold  -> {"fecha","total_mxn","saldos":[{"coin","saldo","libro","precio_mxn","saldo_mxn"}]}
func GetWalletHandler(w http.ResponseWriter, r *http.Request) {
	wallet, ok := walletName(w, r)
	if !ok || !wrapperTokenOK(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	st, ok := dbOrError(ctx, w)
	if !ok {
		return
	}

	walletCache.Lock()
	defer walletCache.Unlock()
	if e, ok := walletCache.entries[wallet]; ok && time.Since(e.at) < saldosCacheTTL {
		respondWallet(w, e.data)
		return
	}
	valueAndRespond(ctx, w, st, wallet)
}

// PutWalletHandler fija la cantidad de cada moneda dada (0 la quita); las
// demás no cambian. Exige token siempre: sin WRAPPER_TOKEN no se puede escribir.
//
//	PUT /wallets/cold  {"btc":0.05,"sol":12,"xrp":0}
func PutWalletHandler(w http.ResponseWriter, r *http.Request) {
	wallet, ok := walletName(w, r)
	if !ok {
		return
	}
	token := os.Getenv("WRAPPER_TOKEN")
	if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Wrapper-Token")), []byte(token)) != 1 {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Token inválido"})
		return
	}
	var body map[string]float64
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil || len(body) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": `Se espera {"moneda": cantidad, ...}`})
		return
	}
	for coin, cant := range body {
		if !coinRe.MatchString(coin) || cant < 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Moneda o cantidad inválida: " + coin})
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	st, ok := dbOrError(ctx, w)
	if !ok {
		return
	}

	walletCache.Lock()
	defer walletCache.Unlock()
	if err := st.SetWallet(ctx, bitsoUser(), wallet, body, time.Now().UTC()); err != nil {
		log.Printf("wallet %s: %v", wallet, err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudo guardar"})
		return
	}
	delete(walletCache.entries, wallet)
	valueAndRespond(ctx, w, st, wallet)
}

// valueAndRespond valúa la wallet, la guarda en caché y la responde. Se llama
// con walletCache tomado.
func valueAndRespond(ctx context.Context, w http.ResponseWriter, st *store.Store, wallet string) {
	coins, err := st.WalletCoins(ctx, bitsoUser(), wallet)
	if err != nil {
		log.Printf("wallet %s: %v", wallet, err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Error al consultar la wallet"})
		return
	}
	// Los tickers son públicos: no hace falta la key de Bitso.
	data, err := bitso.NewClient("", "").GetWalletValues(syncer.Holdings(coins))
	if err != nil {
		respondBitsoError(w, err)
		return
	}
	if walletCache.entries == nil {
		walletCache.entries = map[string]walletEntry{}
	}
	walletCache.entries[wallet] = walletEntry{at: time.Now(), data: data}
	respondWallet(w, data)
}

func respondWallet(w http.ResponseWriter, v *bitso.WalletValues) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"fecha":     v.Fecha,
		"total_mxn": v.TotalMXN,
		"saldos":    v.Saldos,
	})
}

// WalletHistorialHandler devuelve el total en MXN de cada snapshot de la
// wallet que guarda el sync (día 15 y último del mes).
//
//	GET /wallets/cold/historial?desde=2025-11-01  -> {"data": [{"fecha","total_mxn"}]}
func WalletHistorialHandler(w http.ResponseWriter, r *http.Request) {
	wallet, ok := walletName(w, r)
	if !ok || !wrapperTokenOK(w, r) {
		return
	}
	historial(w, r, func(st *store.Store, ctx context.Context, user, desde string) ([]store.SaldoDia, error) {
		return st.WalletHistoricos(ctx, user, wallet, desde)
	})
}
