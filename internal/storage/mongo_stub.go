//go:build !mongodb

package storage

import (
	"context"
	"errors"

	"best_trade_logs/internal/domain/trade"
)

// ErrMongoUnavailable indicates that the binary was built without MongoDB support.
var ErrMongoUnavailable = errors.New("mongoDB support not built; rebuild with -tags mongodb")

// MongoTradeRepository is a stub implementation used when MongoDB support is disabled.
type MongoTradeRepository struct{}

// NewMongoTradeRepository returns an error indicating MongoDB support is unavailable.
func NewMongoTradeRepository(_ interface{}, _ string, _ string) (*MongoTradeRepository, error) {
	return nil, ErrMongoUnavailable
}

// Create returns an error because MongoDB is unavailable.
func (r *MongoTradeRepository) Create(context.Context, *trade.Trade) error {
	return ErrMongoUnavailable
}

// Update returns an error because MongoDB is unavailable.
func (r *MongoTradeRepository) Update(context.Context, *trade.Trade) error {
	return ErrMongoUnavailable
}

// Delete returns an error because MongoDB is unavailable.
func (r *MongoTradeRepository) Delete(context.Context, string) error {
	return ErrMongoUnavailable
}

// GetByID returns an error because MongoDB is unavailable.
func (r *MongoTradeRepository) GetByID(context.Context, string) (*trade.Trade, error) {
	return nil, ErrMongoUnavailable
}

// List returns an error because MongoDB is unavailable.
func (r *MongoTradeRepository) List(context.Context) ([]*trade.Trade, error) {
	return nil, ErrMongoUnavailable
}
