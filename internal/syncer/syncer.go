// Package syncer copia movimientos y saldos de Bitso a MongoDB con el mismo
// esquema que usaba datos_b.py (colecciones trades, abonos, retiros, balance y orders).
package syncer

import (
	"bitsoWrap/internal/bitso"
	"bitsoWrap/internal/store"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/shopspring/decimal"
)

type Syncer struct {
	Client *bitso.BitsoClient
	Store  *store.Store
	User   string
	Loc    *time.Location
	// Full recorre todo el historial en lugar de detenerse en lo ya guardado.
	Full bool
	// ForceBalance guarda el snapshot de saldos aunque no sea día 15 ni fin de mes.
	ForceBalance bool
	Now          func() time.Time
}

// Run ejecuta una sincronización completa. Si una fuente falla, sigue con las demás.
func (s *Syncer) Run(ctx context.Context) error {
	if s.Now == nil {
		s.Now = time.Now
	}
	start := s.Now()
	log.Printf("sync: inicio (user=%s full=%v)", s.User, s.Full)

	if err := s.Store.EnsureIndexes(ctx); err != nil {
		return err
	}

	var errs []error
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"trades", s.syncTrades},
		{"abonos", s.syncFundings},
		{"retiros", s.syncWithdrawals},
		{"balance", s.syncBalance},
		{"wallets", s.syncWallets},
	}
	for _, step := range steps {
		if err := step.fn(ctx); err != nil {
			log.Printf("sync: %s falló: %v", step.name, err)
			errs = append(errs, fmt.Errorf("%s: %w", step.name, err))
		}
	}

	log.Printf("sync: fin en %s", s.Now().Sub(start).Round(time.Second))
	return errors.Join(errs...)
}

// parseBitsoTime acepta los formatos que usa Bitso: "+0000" (trades) y "+00:00" (fundings).
func parseBitsoTime(v string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("fecha inválida: %q", v)
}

// fecha convierte a hora local con el formato legacy "YYYY-MM-DD HH:MM:SS".
func (s *Syncer) fecha(t time.Time) string {
	return t.In(s.Loc).Format("2006-01-02 15:04:05")
}

// absFloat convierte un monto string de Bitso a float positivo redondeado.
func absFloat(v string, places int32) float64 {
	d, err := decimal.NewFromString(v)
	if err != nil {
		return 0
	}
	f, _ := d.Abs().Round(places).Float64()
	return f
}

// pageDone indica si todos los ids de la página ya están guardados con el mismo status.
func pageDone(statuses map[string]string, known map[string]string) bool {
	for id, st := range statuses {
		k, ok := known[id]
		if !ok || k != st {
			return false
		}
	}
	return true
}

func round(f float64, places int32) float64 {
	r, _ := decimal.NewFromFloat(f).Round(places).Float64()
	return r
}
