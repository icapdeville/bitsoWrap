package bitso

import (
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// WalletBooks es el libro con que se valúa cada moneda de las wallets fuera
// de Bitso. Un libro en USD se pasa a MXN con usd_mxn. Las monedas que no
// estén aquí usan <coin>_mxn y, si no existe, <coin>_usd (el propio USD sale
// así de usd_mxn).
var WalletBooks = map[string]string{
	"btc": "btc_mxn",
	"sol": "sol_mxn",
	"xrp": "xrp_usd",
}

// PriceMXN es el bid en MXN de una unidad de coin y el libro del que sale. A
// diferencia de NetValueMXN no descuenta comisión: lo de las wallets no se
// vende en Bitso, sólo se usa su precio.
func (m Market) PriceMXN(coin string) (price decimal.Decimal, book string, ok bool) {
	books := []string{coin + "_mxn", coin + "_usd"}
	if b, ok := WalletBooks[coin]; ok {
		books = []string{b}
	}
	for _, book := range books {
		bid, ok := m.bids[book]
		if !ok {
			continue
		}
		if strings.HasSuffix(book, "_usd") {
			usdMxn, ok := m.bids["usd_mxn"]
			if !ok {
				continue
			}
			bid = bid.Mul(usdMxn)
		}
		return bid, book, true
	}
	return decimal.Zero, "", false
}

// Holding es la cantidad de una moneda guardada fuera de Bitso.
type Holding struct {
	Coin     string
	Cantidad decimal.Decimal
}

// WalletValue es una moneda de una wallet valuada en MXN.
type WalletValue struct {
	Coin      string  `json:"coin"`
	Saldo     float64 `json:"saldo"`
	Libro     string  `json:"libro,omitempty"`
	PrecioMXN float64 `json:"precio_mxn"`
	SaldoMXN  float64 `json:"saldo_mxn"`
	// SinPrecio indica que no hay libro para valuarla (saldo_mxn = 0).
	SinPrecio bool `json:"sin_precio,omitempty"`
}

type WalletValues struct {
	Fecha    time.Time     `json:"fecha"`
	TotalMXN float64       `json:"total_mxn"`
	Saldos   []WalletValue `json:"saldos"`
}

// ValueHoldings valúa en MXN las monedas de una wallet, de mayor a menor valor.
func ValueHoldings(hs []Holding, m Market) WalletValues {
	out := WalletValues{Fecha: time.Now().UTC(), Saldos: []WalletValue{}}
	sum := decimal.Zero
	for _, h := range hs {
		v := WalletValue{Coin: h.Coin, Saldo: toFloat(h.Cantidad, 8)}
		if price, book, ok := m.PriceMXN(h.Coin); ok {
			value := price.Mul(h.Cantidad).Round(2)
			sum = sum.Add(value)
			v.Libro, v.PrecioMXN, v.SaldoMXN = book, toFloat(price, 8), toFloat(value, 2)
		} else {
			v.SinPrecio = true
		}
		out.Saldos = append(out.Saldos, v)
	}
	out.TotalMXN = toFloat(sum, 2)
	sort.SliceStable(out.Saldos, func(i, j int) bool { return out.Saldos[i].SaldoMXN > out.Saldos[j].SaldoMXN })
	return out
}

// GetWalletValues valúa las monedas con los tickers públicos de Bitso (no necesita key).
func (c *BitsoClient) GetWalletValues(hs []Holding) (*WalletValues, error) {
	tickers, err := c.GetTickers()
	if err != nil {
		return nil, err
	}
	v := ValueHoldings(hs, NewMarket(tickers.Payload, nil))
	return &v, nil
}
