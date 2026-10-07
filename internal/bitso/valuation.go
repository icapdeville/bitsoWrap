package bitso

import (
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Market tiene los precios (bid) y comisiones (taker) por libro para valuar saldos.
type Market struct {
	bids map[string]decimal.Decimal
	fees map[string]decimal.Decimal
}

func NewMarket(tickers []TickerPayload, fees []BookFee) Market {
	m := Market{bids: map[string]decimal.Decimal{}, fees: map[string]decimal.Decimal{}}
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

// NetValueMXN es lo que se recibiría en MXN al vender: bid × (1 − comisión) × total.
// Si la moneda no tiene libro en MXN se valúa en USD y se convierte con usd_mxn.
// price es el bid en MXN por unidad (sin descontar comisión).
func (m Market) NetValueMXN(coin string, total decimal.Decimal) (value, price decimal.Decimal, ok bool) {
	one := decimal.NewFromInt(1)
	if bid, ok := m.bids[coin+"_mxn"]; ok {
		return bid.Mul(one.Sub(m.fees[coin+"_mxn"])).Mul(total).Round(2), bid, true
	}
	bid, ok := m.bids[coin+"_usd"]
	usdMxn, okFx := m.bids["usd_mxn"]
	if ok && okFx {
		price = bid.Mul(usdMxn)
		return bid.Mul(one.Sub(m.fees[coin+"_usd"])).Mul(total).Mul(usdMxn).Round(2), price, true
	}
	return decimal.Zero, decimal.Zero, false
}

// ValuedBalance es el saldo de una moneda con su valor en MXN.
type ValuedBalance struct {
	Coin       string  `json:"coin"`
	Saldo      float64 `json:"saldo"`
	Disponible float64 `json:"disponible"`
	PrecioMXN  float64 `json:"precio_mxn"`
	SaldoMXN   float64 `json:"saldo_mxn"`
	// SinPrecio indica que Bitso ya no tiene libro para valuar la moneda (saldo_mxn = 0).
	SinPrecio bool `json:"sin_precio,omitempty"`
}

type ValuedBalances struct {
	Fecha    time.Time       `json:"fecha"`
	TotalMXN float64         `json:"total_mxn"`
	Saldos   []ValuedBalance `json:"saldos"`
}

// ValueBalances valúa en MXN los saldos con total > 0, ordenados de mayor a menor valor.
func ValueBalances(balances []BalanceItem, m Market) ValuedBalances {
	out := ValuedBalances{Fecha: time.Now().UTC(), Saldos: []ValuedBalance{}}
	sum := decimal.Zero
	for _, b := range balances {
		total, err := decimal.NewFromString(b.Total)
		if err != nil || !total.IsPositive() {
			continue
		}
		available, _ := decimal.NewFromString(b.Available)
		coin := strings.ToLower(b.Currency)
		value, price := decimal.Zero, decimal.Zero
		sinPrecio := false

		if coin == "mxn" {
			value, price = total.Round(2), decimal.NewFromInt(1)
		} else if v, p, ok := m.NetValueMXN(coin, total); ok {
			value, price = v, p
		} else {
			sinPrecio = true
		}
		sum = sum.Add(value)
		out.Saldos = append(out.Saldos, ValuedBalance{
			Coin:       coin,
			Saldo:      toFloat(total, 8),
			Disponible: toFloat(available, 8),
			PrecioMXN:  toFloat(price, 8),
			SaldoMXN:   toFloat(value, 2),
			SinPrecio:  sinPrecio,
		})
	}
	out.TotalMXN = toFloat(sum, 2)
	sort.SliceStable(out.Saldos, func(i, j int) bool {
		return out.Saldos[i].SaldoMXN > out.Saldos[j].SaldoMXN
	})
	return out
}

func toFloat(d decimal.Decimal, places int32) float64 {
	f, _ := d.Round(places).Float64()
	return f
}

// GetValuedBalances consulta saldo, tickers y comisiones y devuelve los saldos valuados en MXN.
func (c *BitsoClient) GetValuedBalances() (*ValuedBalances, error) {
	bal, err := c.Balances()
	if err != nil {
		return nil, err
	}
	tickers, err := c.GetTickers()
	if err != nil {
		return nil, err
	}
	fees, err := c.GetFees()
	if err != nil {
		return nil, err
	}
	v := ValueBalances(bal.Payload.Balances, NewMarket(tickers.Payload, fees.Payload.Fees))
	return &v, nil
}
