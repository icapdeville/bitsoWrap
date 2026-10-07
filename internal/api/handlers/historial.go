package handlers

import (
	"bitsoWrap/internal/store"
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// La conexión a Mongo se abre en la primera consulta y se reintenta en la
// siguiente si falla: el resto del wrapper no depende de Mongo.
var mongoConn struct {
	sync.Mutex
	st *store.Store
}

func mongoStore(ctx context.Context) (*store.Store, error) {
	mongoConn.Lock()
	defer mongoConn.Unlock()
	if mongoConn.st != nil {
		return mongoConn.st, nil
	}
	db := os.Getenv("MONGO_DB")
	if db == "" {
		db = "defi"
	}
	st, err := store.Connect(ctx, os.ExpandEnv(os.Getenv("MONGODB_URI")), db)
	if err != nil {
		return nil, err
	}
	mongoConn.st = st
	return st, nil
}

// SaldoHistorialHandler devuelve el saldo total en MXN de cada snapshot que
// guarda el sync (día 15 y último del mes) desde la fecha dada.
//
//	GET /saldo/historial?desde=2025-11-01  -> {"data": [{"fecha","total_mxn"}]}
func SaldoHistorialHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	if !wrapperTokenOK(w, r) {
		return
	}
	historial(w, r, (*store.Store).SaldosHistoricos)
}

// historial responde los totales por fecha que da fn desde ?desde=AAAA-MM-DD.
func historial(w http.ResponseWriter, r *http.Request, fn func(*store.Store, context.Context, string, string) ([]store.SaldoDia, error)) {
	desde := r.URL.Query().Get("desde")
	if desde == "" {
		desde = "0000-00-00"
	} else if _, err := time.Parse("2006-01-02", desde); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "desde debe ser AAAA-MM-DD"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	st, ok := dbOrError(ctx, w)
	if !ok {
		return
	}
	data, err := fn(st, ctx, bitsoUser(), desde)
	if err != nil {
		log.Printf("historial: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Error al consultar el historial"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": data})
}

// dbOrError devuelve la conexión a Mongo o responde 503.
func dbOrError(ctx context.Context, w http.ResponseWriter) (*store.Store, bool) {
	if os.Getenv("MONGODB_URI") == "" {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Falta MONGODB_URI"})
		return nil, false
	}
	st, err := mongoStore(ctx)
	if err != nil {
		log.Printf("mongo: %v", err)
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Sin conexión a MongoDB"})
		return nil, false
	}
	return st, true
}

// bitsoUser es el valor del campo "user" en las colecciones de Mongo.
func bitsoUser() string {
	if u := os.Getenv("BITSO_USER"); u != "" {
		return u
	}
	return "icf"
}
