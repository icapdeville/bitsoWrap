package syncer

import (
	"bitsoWrap/internal/bitso"
	"bitsoWrap/internal/store"
	"context"
	"log"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// isSnapshotDay: el snapshot de saldos se guarda el día 15 y el último día del mes.
func isSnapshotDay(t time.Time) bool {
	return t.Day() == 15 || t.AddDate(0, 0, 1).Day() == 1
}

func (s *Syncer) balanceUpserts(balances []bitso.BalanceItem, m bitso.Market, fecha string) []store.Upsert {
	var ups []store.Upsert
	for _, b := range balances {
		coin := strings.ToLower(b.Currency)
		total, err := decimal.NewFromString(b.Total)
		if err != nil || !total.IsPositive() {
			continue
		}
		totalF, _ := total.Round(8).Float64()
		bitsoDoc := bson.M{"total": totalF}
		if coin != "mxn" {
			if neto, _, ok := m.NetValueMXN(coin, total); ok {
				bitsoDoc["neto"], _ = neto.Float64()
			} else {
				log.Printf("sync: sin precio para valuar %s", coin)
			}
		}
		ups = append(ups, store.Upsert{
			Filter: bson.D{{Key: "user", Value: s.User}, {Key: "coin", Value: coin}, {Key: "fecha", Value: fecha}},
			Set:    bson.M{"user": s.User, "coin": coin, "fecha": fecha, "bitso": bitsoDoc},
		})
	}
	return ups
}

func (s *Syncer) syncBalance(ctx context.Context) error {
	now := s.Now().In(s.Loc)
	if !s.ForceBalance && !isSnapshotDay(now) {
		return nil
	}

	bal, err := s.Client.Balances()
	if err != nil {
		return err
	}
	tickers, err := s.Client.GetTickers()
	if err != nil {
		return err
	}
	fees, err := s.Client.GetFees()
	if err != nil {
		return err
	}

	m := bitso.NewMarket(tickers.Payload, fees.Payload.Fees)
	ups := s.balanceUpserts(bal.Payload.Balances, m, now.Format("2006-01-02"))
	if _, err := s.Store.UpsertMany(ctx, store.Balance, ups); err != nil {
		return err
	}
	log.Printf("sync: balance %s monedas=%d", now.Format("2006-01-02"), len(ups))
	return nil
}
