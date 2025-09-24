package storage

import (
	"context"
	"testing"
	"time"

	"best_trade_logs/internal/domain/trade"
)

func TestInMemoryRepositoryCRUD(t *testing.T) {
	repo := NewInMemoryTradeRepository()
	ctx := context.Background()

	entry := trade.EntryDetail{Price: 10, Quantity: 100}
	tr := &trade.Trade{Instrument: "TSLA", Entry: entry, CreatedAt: time.Now().Add(-time.Hour)}
	if err := repo.Create(ctx, tr); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if tr.ID == "" {
		t.Fatalf("expected ID to be set")
	}

	stored, err := repo.GetByID(ctx, tr.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if stored.Instrument != "TSLA" {
		t.Fatalf("unexpected instrument: %v", stored.Instrument)
	}

	stored.Instrument = "AAPL"
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(list))
	}
	if list[0].Instrument != "AAPL" {
		t.Fatalf("expected updated trade in list")
	}

	if err := repo.Delete(ctx, tr.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, err := repo.GetByID(ctx, tr.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
