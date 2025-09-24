//go:build !mongodb

package main

import (
	"context"

	"best_trade_logs/internal/storage"
)

func setupRepository(context.Context) (storage.TradeRepository, func(), error) {
	repo := storage.NewInMemoryTradeRepository()
	cleanup := func() {}
	return repo, cleanup, nil
}
