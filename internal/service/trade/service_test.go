package trade

import (
	"context"
	"testing"
	"time"

	domain "best_trade_logs/internal/domain/trade"
	"best_trade_logs/internal/storage"
)

func TestServiceCreateAndList(t *testing.T) {
	repo := storage.NewInMemoryTradeRepository()
	svc := NewService(repo)

	tr := &domain.Trade{Instrument: "EURUSD", Entry: domain.EntryDetail{Price: 1.1, Quantity: 1000}}
	if err := svc.Create(context.Background(), tr); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if tr.CreatedAt.IsZero() || tr.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should be set")
	}

	trades, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
}

func TestServiceAddFollowUp(t *testing.T) {
	repo := storage.NewInMemoryTradeRepository()
	svc := NewService(repo)

	tr := &domain.Trade{Instrument: "AAPL", Entry: domain.EntryDetail{Price: 150, Quantity: 10}}
	if err := svc.Create(context.Background(), tr); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	fu := domain.FollowUp{DaysAfter: 7, Price: 165}
	if err := svc.AddFollowUp(context.Background(), tr.ID, fu); err != nil {
		t.Fatalf("add follow up failed: %v", err)
	}

	stored, err := svc.Get(context.Background(), tr.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(stored.FollowUps) != 1 {
		t.Fatalf("expected 1 follow up")
	}
	if stored.FollowUps[0].LoggedAt.IsZero() {
		t.Fatalf("expected loggedAt to be set")
	}
}

func TestNormalizeTags(t *testing.T) {
	repo := storage.NewInMemoryTradeRepository()
	svc := NewService(repo)

	tr := &domain.Trade{Instrument: "BTCUSD", Entry: domain.EntryDetail{Price: 20000, Quantity: 1}, Review: domain.TradeReview{Tags: []string{" Breakout ", "Momentum", ""}}}
	if err := svc.Create(context.Background(), tr); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if len(tr.Review.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tr.Review.Tags))
	}
	if tr.Review.Tags[0] != "breakout" {
		t.Fatalf("expected tags to be lower-cased and trimmed")
	}
}

func TestUpdateKeepsCreatedAt(t *testing.T) {
	repo := storage.NewInMemoryTradeRepository()
	svc := NewService(repo)

	tr := &domain.Trade{Instrument: "ETHUSD", Entry: domain.EntryDetail{Price: 1200, Quantity: 5}}
	if err := svc.Create(context.Background(), tr); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	created := tr.CreatedAt

	time.Sleep(10 * time.Millisecond)
	tr.Instrument = "ETHUSDT"
	if err := svc.Update(context.Background(), tr); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !tr.CreatedAt.Equal(created) {
		t.Fatalf("createdAt should remain unchanged")
	}
	if !tr.UpdatedAt.After(created) {
		t.Fatalf("updatedAt should be later than createdAt")
	}
}
