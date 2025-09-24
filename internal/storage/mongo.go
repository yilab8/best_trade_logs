//go:build mongodb

package storage

import (
	"context"
	"time"

	"best_trade_logs/internal/domain/trade"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoTradeRepository persists trades in MongoDB.
type MongoTradeRepository struct {
	collection *mongo.Collection
}

// NewMongoTradeRepository constructs a Mongo backed repository.
func NewMongoTradeRepository(client *mongo.Client, database, collection string) (*MongoTradeRepository, error) {
	coll := client.Database(database).Collection(collection)
	return &MongoTradeRepository{collection: coll}, nil
}

// Create inserts a new trade document.
func (r *MongoTradeRepository) Create(ctx context.Context, tr *trade.Trade) error {
	if tr.ID == "" {
		tr.ID = primitive.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	if tr.CreatedAt.IsZero() {
		tr.CreatedAt = now
	}
	tr.UpdatedAt = now
	_, err := r.collection.InsertOne(ctx, tr)
	return err
}

// Update replaces an existing trade document.
func (r *MongoTradeRepository) Update(ctx context.Context, tr *trade.Trade) error {
	if tr.ID == "" {
		return ErrNotFound
	}
	tr.UpdatedAt = time.Now().UTC()
	filter := bson.M{"_id": tr.ID}
	result, err := r.collection.ReplaceOne(ctx, filter, tr, options.Replace().SetUpsert(false))
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a trade document.
func (r *MongoTradeRepository) Delete(ctx context.Context, id string) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID fetches a trade document by id.
func (r *MongoTradeRepository) GetByID(ctx context.Context, id string) (*trade.Trade, error) {
	var tr trade.Trade
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&tr)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &tr, nil
}

// List returns trades sorted by creation date (desc).
func (r *MongoTradeRepository) List(ctx context.Context) ([]*trade.Trade, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*trade.Trade
	for cursor.Next(ctx) {
		var tr trade.Trade
		if err := cursor.Decode(&tr); err != nil {
			return nil, err
		}
		results = append(results, &tr)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
