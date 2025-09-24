//go:build mongodb

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"best_trade_logs/internal/storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupRepository(ctx context.Context) (storage.TradeRepository, func(), error) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return nil, nil, fmt.Errorf("MONGO_URI not provided")
	}
	db := os.Getenv("MONGO_DB")
	if db == "" {
		return nil, nil, fmt.Errorf("MONGO_DB not provided")
	}
	collection := os.Getenv("MONGO_COLLECTION")
	if collection == "" {
		collection = "trades"
	}

	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, err
	}
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Connect(connectCtx); err != nil {
		return nil, nil, err
	}
	if err := client.Ping(connectCtx, nil); err != nil {
		_ = client.Disconnect(connectCtx)
		return nil, nil, err
	}

	repo, err := storage.NewMongoTradeRepository(client, db, collection)
	if err != nil {
		_ = client.Disconnect(connectCtx)
		return nil, nil, err
	}
	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(shutdownCtx)
	}
	return repo, cleanup, nil
}
