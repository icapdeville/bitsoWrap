package syncer

import (
	"bitsoWrap/internal/bitso"
	"bitsoWrap/internal/store"
	"context"
	"log"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Holdings convierte lo guardado de una wallet a cantidades para valuar.
func Holdings(coins []store.WalletCoin) []bitso.Holding {
	hs := make([]bitso.Holding, 0, len(coins))
	for _, c := range coins {
		hs = append(hs, bitso.Holding{Coin: c.Coin, Cantidad: decimal.NewFromFloat(c.Cantidad)})
	}
	return hs
}

func (s *Syncer) walletUpserts(wallet string, v bitso.WalletValues, fecha string) []store.Upsert {
	ups := make([]store.Upsert, 0, len(v.Saldos))
	for _, c := range v.Saldos {
		if c.SinPrecio {
			log.Printf("sync: wallet %s sin precio para valuar %s", wallet, c.Coin)
		}
		ups = append(ups, store.Upsert{
			Filter: bson.D{{Key: "user", Value: s.User}, {Key: "wallet", Value: wallet}, {Key: "coin", Value: c.Coin}, {Key: "fecha", Value: fecha}},
			Set: bson.M{"user": s.User, "wallet": wallet, "coin": c.Coin, "fecha": fecha, "cantidad": c.Saldo,
				"libro": c.Libro, "precio_mxn": c.PrecioMXN, "valor_mxn": c.SaldoMXN},
		})
	}
	return ups
}

// syncWallets guarda la valuación de cada wallet fuera de Bitso los mismos
// días que el snapshot de saldos de Bitso.
func (s *Syncer) syncWallets(ctx context.Context) error {
	now := s.Now().In(s.Loc)
	if !s.ForceBalance && !isSnapshotDay(now) {
		return nil
	}
	coins, err := s.Store.WalletCoins(ctx, s.User, "")
	if err != nil || len(coins) == 0 {
		return err
	}
	porWallet := map[string][]store.WalletCoin{}
	for _, c := range coins {
		porWallet[c.Wallet] = append(porWallet[c.Wallet], c)
	}

	tickers, err := s.Client.GetTickers()
	if err != nil {
		return err
	}
	m := bitso.NewMarket(tickers.Payload, nil)
	fecha := now.Format("2006-01-02")
	var ups []store.Upsert
	for wallet, cs := range porWallet {
		v := bitso.ValueHoldings(Holdings(cs), m)
		ups = append(ups, s.walletUpserts(wallet, v, fecha)...)
		log.Printf("sync: wallet %s %s monedas=%d total=%.2f", wallet, fecha, len(v.Saldos), v.TotalMXN)
	}
	_, err = s.Store.UpsertMany(ctx, store.WalletsBalance, ups)
	return err
}
