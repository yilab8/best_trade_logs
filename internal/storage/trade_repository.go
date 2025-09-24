package storage

import (
	"context"

	"best_trade_logs/internal/domain/trade"
)

// TradeRepository describes the persistence operations required by the service layer.
type TradeRepository interface {
	Create(ctx context.Context, tr *trade.Trade) error
	Update(ctx context.Context, tr *trade.Trade) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*trade.Trade, error)
	List(ctx context.Context) ([]*trade.Trade, error)
}
