package storage

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"best_trade_logs/internal/domain/trade"
)

// ErrNotFound is returned when a trade is not found in the repository.
var ErrNotFound = errors.New("trade not found")

// InMemoryTradeRepository provides an in-memory implementation for testing purposes.
type InMemoryTradeRepository struct {
	mu     sync.RWMutex
	trades map[string]*trade.Trade
}

// NewInMemoryTradeRepository constructs an empty repository.
func NewInMemoryTradeRepository() *InMemoryTradeRepository {
	return &InMemoryTradeRepository{trades: make(map[string]*trade.Trade)}
}

// Create stores a new trade. If the trade does not have an ID it is generated using the timestamp.
func (r *InMemoryTradeRepository) Create(_ context.Context, tr *trade.Trade) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tr.ID == "" {
		tr.ID = generateID()
	}
	now := time.Now().UTC()
	if tr.CreatedAt.IsZero() {
		tr.CreatedAt = now
	}
	tr.UpdatedAt = now

	cp := *tr
	r.trades[tr.ID] = &cp
	return nil
}

// Update updates an existing trade.
func (r *InMemoryTradeRepository) Update(_ context.Context, tr *trade.Trade) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tr.ID == "" {
		return ErrNotFound
	}
	if _, ok := r.trades[tr.ID]; !ok {
		return ErrNotFound
	}
	cp := *tr
	cp.UpdatedAt = time.Now().UTC()
	r.trades[tr.ID] = &cp
	return nil
}

// Delete removes a trade from the repository.
func (r *InMemoryTradeRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.trades[id]; !ok {
		return ErrNotFound
	}
	delete(r.trades, id)
	return nil
}

// GetByID retrieves a trade by its identifier.
func (r *InMemoryTradeRepository) GetByID(_ context.Context, id string) (*trade.Trade, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tr, ok := r.trades[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *tr
	return &cp, nil
}

// List returns the trades sorted by creation date descending.
func (r *InMemoryTradeRepository) List(_ context.Context) ([]*trade.Trade, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]*trade.Trade, 0, len(r.trades))
	for _, tr := range r.trades {
		cp := *tr
		results = append(results, &cp)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
	return results, nil
}

func generateID() string {
	return time.Now().UTC().Format("20060102T150405.000000000")
}
