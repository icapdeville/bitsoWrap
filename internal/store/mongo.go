package store

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	Trades  = "trades"
	Abonos  = "abonos"
	Retiros = "retiros"
	Balance = "balance"
	Orders  = "orders"
	// Cantidades de las wallets fuera de Bitso (cold wallet, Mercado Pago…),
	// capturadas a mano, y su valuación los días de snapshot.
	Wallets        = "wallets"
	WalletsBalance = "wallets_balance"
	Exchange       = "bitso"
)

type Store struct {
	client *mongo.Client
	db     *mongo.Database
}

func Connect(ctx context.Context, uri, dbName string) (*Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetTimeout(30 * time.Second))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	return &Store{client: client, db: client.Database(dbName)}, nil
}

func (s *Store) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func (s *Store) Collection(name string) *mongo.Collection {
	return s.db.Collection(name)
}

// EnsureIndexes crea los índices de las claves de upsert. No son únicos para
// no fallar con los registros legacy (p. ej. abonos manuales con fid vacío).
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := map[string]bson.D{
		Trades:         {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "tid", Value: 1}},
		Abonos:         {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "fid", Value: 1}},
		Retiros:        {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "wid", Value: 1}},
		Orders:         {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "oid", Value: 1}},
		Balance:        {{Key: "user", Value: 1}, {Key: "coin", Value: 1}, {Key: "fecha", Value: 1}},
		Wallets:        {{Key: "user", Value: 1}, {Key: "wallet", Value: 1}, {Key: "coin", Value: 1}},
		WalletsBalance: {{Key: "user", Value: 1}, {Key: "wallet", Value: 1}, {Key: "fecha", Value: 1}},
	}
	for coll, keys := range indexes {
		if _, err := s.Collection(coll).Indexes().CreateOne(ctx, mongo.IndexModel{Keys: keys}); err != nil {
			return fmt.Errorf("índice %s: %w", coll, err)
		}
	}
	return nil
}

// Upsert es un documento a escribir con $set sobre el filtro dado.
type Upsert struct {
	Filter bson.D
	Set    bson.M
}

// UpsertMany aplica los upserts en un solo BulkWrite.
func (s *Store) UpsertMany(ctx context.Context, coll string, ups []Upsert) (*mongo.BulkWriteResult, error) {
	if len(ups) == 0 {
		return &mongo.BulkWriteResult{}, nil
	}
	models := make([]mongo.WriteModel, 0, len(ups))
	for _, u := range ups {
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(u.Filter).
			SetUpdate(bson.M{"$set": u.Set}).
			SetUpsert(true))
	}
	return s.Collection(coll).BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
}

// KnownStatus devuelve, para los ids que ya existen en la colección, su campo
// "status" (vacío si no tiene). Sirve para saber cuándo detener la paginación.
func (s *Store) KnownStatus(ctx context.Context, coll, user, idField string, ids []string) (map[string]string, error) {
	filter := bson.D{
		{Key: "user", Value: user},
		{Key: "exchange", Value: Exchange},
		{Key: idField, Value: bson.M{"$in": ids}},
	}
	opts := options.Find().SetProjection(bson.M{idField: 1, "status": 1, "_id": 0})
	cur, err := s.Collection(coll).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	known := make(map[string]string, len(ids))
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		id, _ := doc[idField].(string)
		status, _ := doc["status"].(string)
		known[id] = status
	}
	return known, cur.Err()
}

// SaldoDia es el valor en MXN de todas las monedas en un snapshot de balance.
type SaldoDia struct {
	Fecha    string  `bson:"_id" json:"fecha"`
	TotalMXN float64 `bson:"total_mxn" json:"total_mxn"`
}

// SaldosHistoricos suma por fecha los snapshots de balance desde `desde`
// (AAAA-MM-DD). Cada moneda aporta su `neto`; el MXN, su `total`. Una moneda
// que no se pudo valuar al guardar el snapshot no suma.
func (s *Store) SaldosHistoricos(ctx context.Context, user, desde string) ([]SaldoDia, error) {
	valor := bson.M{"$ifNull": bson.A{
		"$bitso.neto",
		bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$coin", "mxn"}}, "$bitso.total", 0}},
	}}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user": user, "fecha": bson.M{"$gte": desde}}}},
		{{Key: "$group", Value: bson.M{"_id": "$fecha", "total_mxn": bson.M{"$sum": valor}}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	cur, err := s.Collection(Balance).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	out := []SaldoDia{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].TotalMXN = math.Round(out[i].TotalMXN*100) / 100
	}
	return out, nil
}

// WalletCoin es una moneda de una wallet tal como se guarda.
type WalletCoin struct {
	Wallet      string    `bson:"wallet" json:"wallet"`
	Coin        string    `bson:"coin" json:"coin"`
	Cantidad    float64   `bson:"cantidad" json:"cantidad"`
	Actualizado time.Time `bson:"actualizado" json:"actualizado"`
}

// WalletCoins devuelve las monedas de una wallet del usuario; con wallet
// vacío, las de todas.
func (s *Store) WalletCoins(ctx context.Context, user, wallet string) ([]WalletCoin, error) {
	filter := bson.M{"user": user}
	if wallet != "" {
		filter["wallet"] = wallet
	}
	opts := options.Find().SetSort(bson.D{{Key: "wallet", Value: 1}, {Key: "coin", Value: 1}})
	cur, err := s.Collection(Wallets).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []WalletCoin{}
	return out, cur.All(ctx, &out)
}

// SetWallet fija la cantidad de cada moneda dada; 0 la quita. Las que no
// vienen en el mapa no cambian.
func (s *Store) SetWallet(ctx context.Context, user, wallet string, cantidades map[string]float64, now time.Time) error {
	coll := s.Collection(Wallets)
	for coin, cant := range cantidades {
		filter := bson.M{"user": user, "wallet": wallet, "coin": coin}
		if cant == 0 {
			if _, err := coll.DeleteOne(ctx, filter); err != nil {
				return err
			}
			continue
		}
		set := bson.M{"user": user, "wallet": wallet, "coin": coin, "cantidad": cant, "actualizado": now}
		if _, err := coll.UpdateOne(ctx, filter, bson.M{"$set": set}, options.UpdateOne().SetUpsert(true)); err != nil {
			return err
		}
	}
	return nil
}

// WalletHistoricos suma por fecha la valuación guardada de una wallet desde
// `desde` (AAAA-MM-DD).
func (s *Store) WalletHistoricos(ctx context.Context, user, wallet, desde string) ([]SaldoDia, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user": user, "wallet": wallet, "fecha": bson.M{"$gte": desde}}}},
		{{Key: "$group", Value: bson.M{"_id": "$fecha", "total_mxn": bson.M{"$sum": "$valor_mxn"}}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	cur, err := s.Collection(WalletsBalance).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	out := []SaldoDia{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].TotalMXN = math.Round(out[i].TotalMXN*100) / 100
	}
	return out, nil
}
