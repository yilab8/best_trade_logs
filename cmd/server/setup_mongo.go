//go:build mongodb

package main

import (
	"context"
	"fmt"
	"time"

	"best_trade_logs/internal/storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupRepository(ctx context.Context, cfg config) (storage.TradeRepository, func(), error) {
	if cfg.MongoURI == "" {
		return nil, nil, fmt.Errorf("mongo URI not provided; set MONGO_URI or use --mongo-uri flag")
	}
	if cfg.MongoDatabase == "" {
		return nil, nil, fmt.Errorf("mongo database not provided; set MONGO_DB or use --mongo-db flag")
	}

	client, err := mongo.NewClient(options.Client().ApplyURI(cfg.MongoURI))

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

	repo, err := storage.NewMongoTradeRepository(client, cfg.MongoDatabase, cfg.MongoCollection)

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
