package bitso

import (
	"testing"

	"github.com/shopspring/decimal"
)

func testMarket() Market {
	return NewMarket(
		[]TickerPayload{{Book: "sol_mxn", Bid: "2000"}, {Book: "ada_usd", Bid: "0.5"}, {Book: "usd_mxn", Bid: "18"}},
		[]BookFee{{Book: "sol_mxn", TakerFeeDecimal: "0.0078"}, {Book: "ada_usd", TakerFeeDecimal: "0.0036"}},
	)
}

func TestNetValueMXN(t *testing.T) {
	m := testMarket()
	d := decimal.RequireFromString

	// 2000 × (1 − 0.0078) × 0.5 = 992.20
	if v, p, ok := m.NetValueMXN("sol", d("0.5")); !ok || !v.Equal(d("992.2")) || !p.Equal(d("2000")) {
		t.Errorf("sol = %s @ %s", v, p)
	}
	// 0.5 × (1 − 0.0036) × 100 × 18 = 896.76 ; precio 0.5 × 18 = 9
	if v, p, ok := m.NetValueMXN("ada", d("100")); !ok || !v.Equal(d("896.76")) || !p.Equal(d("9")) {
		t.Errorf("ada = %s @ %s", v, p)
	}
	if _, _, ok := m.NetValueMXN("shib", d("1")); ok {
		t.Error("una moneda sin libro no debe tener valor")
	}
}

func TestValueBalances(t *testing.T) {
	v := ValueBalances([]BalanceItem{
		{Currency: "mxn", Total: "100.555", Available: "100.555"},
		{Currency: "sol", Total: "0.5", Available: "0.4"},
		{Currency: "shib", Total: "1000", Available: "1000"},
		{Currency: "btc", Total: "0", Available: "0"},
	}, testMarket())

	if len(v.Saldos) != 3 {
		t.Fatalf("se esperaban 3 saldos (sin btc en cero), llegaron %d", len(v.Saldos))
	}
	if v.Saldos[0].Coin != "sol" || v.Saldos[0].SaldoMXN != 992.2 || v.Saldos[0].Disponible != 0.4 {
		t.Errorf("primero debe ser sol: %+v", v.Saldos[0])
	}
	if v.Saldos[2].Coin != "shib" || !v.Saldos[2].SinPrecio {
		t.Errorf("shib debe ir al final sin precio: %+v", v.Saldos[2])
	}
	// 992.20 + 100.56
	if v.TotalMXN != 1092.76 {
		t.Errorf("total = %v", v.TotalMXN)
	}
}

func TestValueHoldings(t *testing.T) {
	m := NewMarket([]TickerPayload{
		{Book: "btc_mxn", Bid: "2000000"}, {Book: "sol_mxn", Bid: "3000"},
		{Book: "xrp_mxn", Bid: "999"}, {Book: "xrp_usd", Bid: "2.5"}, {Book: "usd_mxn", Bid: "18"},
	}, nil)
	d := decimal.RequireFromString
	v := ValueHoldings([]Holding{
		{Coin: "btc", Cantidad: d("0.01")},  // 20000
		{Coin: "sol", Cantidad: d("2")},     // 6000
		{Coin: "xrp", Cantidad: d("100")},   // por xrp_usd: 2.5 × 18 × 100 = 4500 (no xrp_mxn)
		{Coin: "shib", Cantidad: d("1000")}, // sin libro
		{Coin: "usd", Cantidad: d("10")},    // por usd_mxn: 180
	}, m)
	if v.TotalMXN != 30680 {
		t.Errorf("total = %v, se esperaba 30680", v.TotalMXN)
	}
	if v.Saldos[0].Coin != "btc" || v.Saldos[2].Libro != "xrp_usd" || v.Saldos[2].PrecioMXN != 45 {
		t.Errorf("orden o libro inesperado: %+v", v.Saldos)
	}
	if last := v.Saldos[4]; last.Coin != "shib" || !last.SinPrecio {
		t.Errorf("shib debe ir sin precio: %+v", last)
	}
}
