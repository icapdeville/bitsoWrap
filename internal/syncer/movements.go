package syncer

import (
	"bitsoWrap/internal/bitso"
	"bitsoWrap/internal/store"
	"context"
	"encoding/json"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Syncer) fundingUpsert(f bitso.Funding) (store.Upsert, error) {
	created, err := parseBitsoTime(f.CreatedAt)
	if err != nil {
		return store.Upsert{}, err
	}
	return store.Upsert{
		Filter: bson.D{{Key: "user", Value: s.User}, {Key: "exchange", Value: store.Exchange}, {Key: "fid", Value: f.Fid}},
		Set: bson.M{
			"user":       s.User,
			"exchange":   store.Exchange,
			"fecha":      s.fecha(created),
			"created_at": created,
			"monto":      absFloat(f.Amount, 8),
			"coin":       f.Currency,
			"metodo":     f.MethodName,
			"eid":        "",
			"fid":        f.Fid,
			"status":     f.Status,
			"method":     f.Method,
			"network":    f.Network,
			"asset":      f.Asset,
			"fee":        absFloat(f.Fee, 8),
		},
	}, nil
}

type withdrawalDetails struct {
	BeneficiaryClabe  string `json:"beneficiary_clabe"`
	WithdrawalAddress string `json:"withdrawal_address"`
	TxHash            string `json:"tx_hash"`
	Fee               string `json:"fee"`
}

func (s *Syncer) withdrawalUpsert(w bitso.Withdrawal) (store.Upsert, error) {
	created, err := parseBitsoTime(w.CreatedAt)
	if err != nil {
		return store.Upsert{}, err
	}

	var d withdrawalDetails
	if len(w.Details) > 0 {
		// details cambia según el método; si no se puede leer se ignora.
		_ = json.Unmarshal(w.Details, &d)
	}
	var destino any
	switch {
	case d.BeneficiaryClabe != "":
		destino = d.BeneficiaryClabe
	case d.WithdrawalAddress != "":
		destino = d.WithdrawalAddress
	}

	return store.Upsert{
		Filter: bson.D{{Key: "user", Value: s.User}, {Key: "exchange", Value: store.Exchange}, {Key: "wid", Value: w.Wid}},
		Set: bson.M{
			"user":       s.User,
			"exchange":   store.Exchange,
			"fecha":      s.fecha(created),
			"created_at": created,
			"wid":        w.Wid,
			"coin":       w.Currency,
			"metodo":     w.MethodName,
			"monto":      absFloat(w.Amount, 8),
			"destino":    destino,
			"status":     w.Status,
			"method":     w.Method,
			"network":    w.Network,
			"asset":      w.Asset,
			"fee":        absFloat(d.Fee, 8),
			"tx_hash":    d.TxHash,
		},
	}, nil
}

func (s *Syncer) syncFundings(ctx context.Context) error {
	total := 0
	err := s.Client.WalkFundings(nil, func(page []bitso.Funding) (bool, error) {
		ups := make([]store.Upsert, 0, len(page))
		statuses := make(map[string]string, len(page))
		ids := make([]string, 0, len(page))
		for _, f := range page {
			u, err := s.fundingUpsert(f)
			if err != nil {
				log.Printf("sync: fondeo %s omitido: %v", f.Fid, err)
				continue
			}
			ups = append(ups, u)
			ids = append(ids, f.Fid)
			statuses[f.Fid] = f.Status
		}
		return s.writePage(ctx, store.Abonos, "fid", ids, statuses, ups, &total)
	})
	log.Printf("sync: abonos procesados=%d", total)
	return err
}

func (s *Syncer) syncWithdrawals(ctx context.Context) error {
	total := 0
	err := s.Client.WalkWithdrawals(nil, func(page []bitso.Withdrawal) (bool, error) {
		ups := make([]store.Upsert, 0, len(page))
		statuses := make(map[string]string, len(page))
		ids := make([]string, 0, len(page))
		for _, w := range page {
			u, err := s.withdrawalUpsert(w)
			if err != nil {
				log.Printf("sync: retiro %s omitido: %v", w.Wid, err)
				continue
			}
			ups = append(ups, u)
			ids = append(ids, w.Wid)
			statuses[w.Wid] = w.Status
		}
		return s.writePage(ctx, store.Retiros, "wid", ids, statuses, ups, &total)
	})
	log.Printf("sync: retiros procesados=%d", total)
	return err
}

// writePage guarda la página y decide si seguir paginando: se detiene cuando
// todo lo de la página ya estaba guardado con el mismo status (salvo en modo Full).
func (s *Syncer) writePage(ctx context.Context, coll, idField string, ids []string, statuses map[string]string, ups []store.Upsert, total *int) (bool, error) {
	known, err := s.Store.KnownStatus(ctx, coll, s.User, idField, ids)
	if err != nil {
		return false, err
	}
	if _, err := s.Store.UpsertMany(ctx, coll, ups); err != nil {
		return false, err
	}
	*total += len(ups)
	return s.Full || !pageDone(statuses, known), nil
}
