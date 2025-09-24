//go:build !mongodb

package main

import (
	"context"

	"best_trade_logs/internal/storage"
)

func setupRepository(_ context.Context, _ config) (storage.TradeRepository, func(), error) {

	repo := storage.NewInMemoryTradeRepository()
	cleanup := func() {}
	return repo, cleanup, nil
}
