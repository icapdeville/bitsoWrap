package syncer

import (
	"bitsoWrap/internal/bitso"
	"bitsoWrap/internal/store"
	"context"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// legacyOrderStatus son los status del bot anterior ("c"/"v"); esas órdenes no se tocan.
var legacyOrderStatus = []string{"c", "v"}

func (s *Syncer) tradeUpsert(t bitso.UserTrade) (store.Upsert, error) {
	created, err := parseBitsoTime(t.CreatedAt)
	if err != nil {
		return store.Upsert{}, err
	}
	tid := string(t.Tid)
	return store.Upsert{
		Filter: bson.D{{Key: "user", Value: s.User}, {Key: "exchange", Value: store.Exchange}, {Key: "tid", Value: tid}},
		Set: bson.M{
			"user":       s.User,
			"exchange":   store.Exchange,
			"fecha":      s.fecha(created),
			"created_at": created,
			"side":       t.Side,
			"book":       t.MinorCurrency,
			"coin":       t.MajorCurrency,
			"major":      absFloat(t.Major, 8),
			"minor":      absFloat(t.Minor, 8),
			"fee_c":      t.FeesCurrency,
			"fee":        absFloat(t.FeesAmount, 8),
			"price":      absFloat(t.Price, 8),
			"oid":        t.Oid,
			"tid":        tid,
		},
	}, nil
}

func (s *Syncer) syncTrades(ctx context.Context) error {
	total := 0
	oids := map[string]struct{}{}

	err := s.Client.WalkUserTrades(nil, func(page []bitso.UserTrade) (bool, error) {
		ups := make([]store.Upsert, 0, len(page))
		statuses := make(map[string]string, len(page))
		ids := make([]string, 0, len(page))
		for _, t := range page {
			u, err := s.tradeUpsert(t)
			if err != nil {
				log.Printf("sync: trade %s omitido: %v", t.Tid, err)
				continue
			}
			ups = append(ups, u)
			ids = append(ids, string(t.Tid))
			statuses[string(t.Tid)] = ""
			oids[t.Oid] = struct{}{}
		}
		return s.writePage(ctx, store.Trades, "tid", ids, statuses, ups, &total)
	})
	log.Printf("sync: trades procesados=%d", total)
	if err != nil {
		return err
	}

	return s.syncExecutedOrders(ctx, oids)
}

// syncExecutedOrders arma una orden ejecutada por oid sumando sus trades guardados.
func (s *Syncer) syncExecutedOrders(ctx context.Context, oids map[string]struct{}) error {
	if len(oids) == 0 {
		return nil
	}
	list := make([]string, 0, len(oids))
	for oid := range oids {
		list = append(list, oid)
	}

	// Se omiten las órdenes legacy del bot anterior.
	legacy := map[string]bool{}
	cur, err := s.Store.Collection(store.Orders).Find(ctx, bson.D{
		{Key: "user", Value: s.User},
		{Key: "exchange", Value: store.Exchange},
		{Key: "oid", Value: bson.M{"$in": list}},
		{Key: "status", Value: bson.M{"$in": legacyOrderStatus}},
	})
	if err != nil {
		return err
	}
	for cur.Next(ctx) {
		var doc struct {
			Oid string `bson:"oid"`
		}
		if err := cur.Decode(&doc); err == nil {
			legacy[doc.Oid] = true
		}
	}
	cur.Close(ctx)

	pipeline := bson.A{
		bson.M{"$match": bson.M{"user": s.User, "exchange": store.Exchange, "oid": bson.M{"$in": list}}},
		bson.M{"$sort": bson.M{"fecha": 1}},
		bson.M{"$group": bson.M{
			"_id":        "$oid",
			"fecha":      bson.M{"$first": "$fecha"},
			"created_at": bson.M{"$first": "$created_at"},
			"book":       bson.M{"$first": "$book"},
			"coin":       bson.M{"$first": "$coin"},
			"side":       bson.M{"$first": "$side"},
			"fee_c":      bson.M{"$first": "$fee_c"},
			"major":      bson.M{"$sum": "$major"},
			"minor":      bson.M{"$sum": "$minor"},
			"fee":        bson.M{"$sum": "$fee"},
			"trades":     bson.M{"$sum": 1},
		}},
	}
	cur, err = s.Store.Collection(store.Trades).Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	var ups []store.Upsert
	for cur.Next(ctx) {
		var o struct {
			Oid       string  `bson:"_id"`
			Fecha     string  `bson:"fecha"`
			CreatedAt any     `bson:"created_at"`
			Book      string  `bson:"book"`
			Coin      string  `bson:"coin"`
			Side      string  `bson:"side"`
			FeeC      string  `bson:"fee_c"`
			Major     float64 `bson:"major"`
			Minor     float64 `bson:"minor"`
			Fee       float64 `bson:"fee"`
			Trades    int     `bson:"trades"`
		}
		if err := cur.Decode(&o); err != nil {
			return err
		}
		if legacy[o.Oid] {
			continue
		}
		set := bson.M{
			"user":     s.User,
			"exchange": store.Exchange,
			"oid":      o.Oid,
			"fecha":    o.Fecha,
			"book":     o.Book,
			"coin":     o.Coin,
			"side":     o.Side,
			"fee_c":    o.FeeC,
			"major":    round(o.Major, 8),
			"minor":    round(o.Minor, 8),
			"fee":      round(o.Fee, 8),
			"trades":   o.Trades,
			"status":   "executed",
		}
		if o.Major != 0 {
			set["price"] = round(o.Minor/o.Major, 8)
		}
		if o.CreatedAt != nil {
			set["created_at"] = o.CreatedAt
		}
		ups = append(ups, store.Upsert{
			Filter: bson.D{{Key: "user", Value: s.User}, {Key: "exchange", Value: store.Exchange}, {Key: "oid", Value: o.Oid}},
			Set:    set,
		})
	}
	if err := cur.Err(); err != nil {
		return err
	}

	if _, err := s.Store.UpsertMany(ctx, store.Orders, ups); err != nil {
		return err
	}
	log.Printf("sync: órdenes ejecutadas=%d (legacy omitidas=%d)", len(ups), len(legacy))
	return nil
}
