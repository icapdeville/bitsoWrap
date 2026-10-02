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

type market struct {
	bids map[string]decimal.Decimal // book -> bid
	fees map[string]decimal.Decimal // book -> taker fee (decimal)
}

func newMarket(tickers []bitso.TickerPayload, fees []bitso.BookFee) market {
	m := market{bids: map[string]decimal.Decimal{}, fees: map[string]decimal.Decimal{}}
	for _, t := range tickers {
		if bid, err := decimal.NewFromString(t.Bid); err == nil && bid.IsPositive() {
			m.bids[t.Book] = bid
		}
	}
	for _, f := range fees {
		if fee, err := decimal.NewFromString(f.TakerFeeDecimal); err == nil {
			m.fees[f.Book] = fee
		}
	}
	return m
}

// netValue es lo que se recibiría en MXN al vender: bid × (1 − comisión) × total.
// Si la moneda no tiene libro en MXN se valúa en USD y se convierte con usd_mxn.
func (m market) netValue(coin string, total decimal.Decimal) (decimal.Decimal, bool) {
	one := decimal.NewFromInt(1)
	if bid, ok := m.bids[coin+"_mxn"]; ok {
		return bid.Mul(one.Sub(m.fees[coin+"_mxn"])).Mul(total).Round(2), true
	}
	bid, ok := m.bids[coin+"_usd"]
	usdMxn, okFx := m.bids["usd_mxn"]
	if ok && okFx {
		return bid.Mul(one.Sub(m.fees[coin+"_usd"])).Mul(total).Mul(usdMxn).Round(2), true
	}
	return decimal.Zero, false
}

func (s *Syncer) balanceUpserts(balances []bitso.BalanceItem, m market, fecha string) []store.Upsert {
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
			if neto, ok := m.netValue(coin, total); ok {
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

	m := newMarket(tickers.Payload, fees.Payload.Fees)
	ups := s.balanceUpserts(bal.Payload.Balances, m, now.Format("2006-01-02"))
	if _, err := s.Store.UpsertMany(ctx, store.Balance, ups); err != nil {
		return err
	}
	log.Printf("sync: balance %s monedas=%d", now.Format("2006-01-02"), len(ups))
	return nil
}
