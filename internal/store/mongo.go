package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	Trades   = "trades"
	Abonos   = "abonos"
	Retiros  = "retiros"
	Balance  = "balance"
	Orders   = "orders"
	Exchange = "bitso"
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
		Trades:  {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "tid", Value: 1}},
		Abonos:  {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "fid", Value: 1}},
		Retiros: {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "wid", Value: 1}},
		Orders:  {{Key: "user", Value: 1}, {Key: "exchange", Value: 1}, {Key: "oid", Value: 1}},
		Balance: {{Key: "user", Value: 1}, {Key: "coin", Value: 1}, {Key: "fecha", Value: 1}},
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
