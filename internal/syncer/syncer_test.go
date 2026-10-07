package syncer

import (
	"bitsoWrap/internal/bitso"
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func testSyncer(t *testing.T) *Syncer {
	t.Helper()
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	return &Syncer{User: "icf", Loc: loc}
}

func TestFechaUsesMexicoCityRules(t *testing.T) {
	s := testSyncer(t)
	cases := map[string]string{
		// 2021: había horario de verano en julio (UTC-5)
		"2021-07-01T15:00:00+0000": "2021-07-01 10:00:00",
		// 2023: ya no hay horario de verano (UTC-6); datos_b.py restaba 5 aquí
		"2023-07-01T15:00:00+00:00": "2023-07-01 09:00:00",
		"2026-10-02T13:57:29+0000":  "2026-10-02 07:57:29",
	}
	for in, want := range cases {
		tm, err := parseBitsoTime(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.fecha(tm); got != want {
			t.Errorf("%s -> %s, se esperaba %s", in, got, want)
		}
	}
}

func TestTradeUpsertLegacyShape(t *testing.T) {
	s := testSyncer(t)
	u, err := s.tradeUpsert(bitso.UserTrade{
		Book: "sol_mxn", Major: "0.0015", Minor: "-3.328035", MajorCurrency: "sol", MinorCurrency: "mxn",
		Price: "2218.69", Side: "buy", FeesCurrency: "sol", FeesAmount: "0.00000900",
		Tid: "202163149", Oid: "SiDgGMWce7426MQk", CreatedAt: "2026-10-02T13:57:29+0000",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := bson.M{"book": "mxn", "coin": "sol", "minor": 3.328035, "major": 0.0015, "fee": 0.000009,
		"price": 2218.69, "tid": "202163149", "fecha": "2026-10-02 07:57:29", "exchange": "bitso", "user": "icf"}
	for k, v := range want {
		if u.Set[k] != v {
			t.Errorf("%s = %v, se esperaba %v", k, u.Set[k], v)
		}
	}
}

func TestWithdrawalDestino(t *testing.T) {
	s := testSyncer(t)
	crypto, _ := s.withdrawalUpsert(bitso.Withdrawal{Wid: "w1", Amount: "3.01", CreatedAt: "2026-08-26T20:13:36+00:00",
		Details: json.RawMessage(`{"withdrawal_address":"0xabc","tx_hash":"0xhash","fee":"0.01"}`)})
	if crypto.Set["destino"] != "0xabc" || crypto.Set["tx_hash"] != "0xhash" || crypto.Set["fee"] != 0.01 {
		t.Errorf("retiro cripto mal mapeado: %+v", crypto.Set)
	}

	spei, _ := s.withdrawalUpsert(bitso.Withdrawal{Wid: "w2", Amount: "100", CreatedAt: "2026-08-26T20:13:36+00:00",
		Details: json.RawMessage(`{"beneficiary_clabe":"012345678901234567"}`)})
	if spei.Set["destino"] != "012345678901234567" {
		t.Errorf("retiro SPEI mal mapeado: %+v", spei.Set)
	}

	none, _ := s.withdrawalUpsert(bitso.Withdrawal{Wid: "w3", Amount: "1", CreatedAt: "2026-08-26T20:13:36+00:00"})
	if none.Set["destino"] != nil {
		t.Errorf("destino debe ser null sin details: %v", none.Set["destino"])
	}
}

func TestIsSnapshotDay(t *testing.T) {
	cases := map[string]bool{
		"2026-10-15": true, "2026-10-31": true, "2026-02-28": true,
		"2028-02-28": false, "2028-02-29": true, "2026-10-01": false, "2026-10-30": false,
	}
	for d, want := range cases {
		tm, _ := time.Parse("2006-01-02", d)
		if got := isSnapshotDay(tm); got != want {
			t.Errorf("%s -> %v, se esperaba %v", d, got, want)
		}
	}
}

func TestPageDone(t *testing.T) {
	if !pageDone(map[string]string{"a": "complete"}, map[string]string{"a": "complete"}) {
		t.Error("todo conocido con mismo status debe detener")
	}
	if pageDone(map[string]string{"a": "complete"}, map[string]string{"a": "pending"}) {
		t.Error("cambio de status debe seguir")
	}
	if pageDone(map[string]string{"a": "", "b": ""}, map[string]string{"a": ""}) {
		t.Error("id nuevo debe seguir")
	}
}
