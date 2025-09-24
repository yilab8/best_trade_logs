package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	domain "best_trade_logs/internal/domain/trade"
	tradesvc "best_trade_logs/internal/service/trade"
	"best_trade_logs/internal/storage"
)

func TestBuildTradeFromFormParsesExit(t *testing.T) {
	form := url.Values{}
	form.Set("instrument", "AAPL")
	form.Set("direction", "LONG")
	form.Set("entry_date", "2023-01-02")
	form.Set("entry_price", "100")
	form.Set("entry_quantity", "10")
	form.Set("entry_fees", "2")
	form.Set("exit_date", "2023-01-05")
	form.Set("exit_price", "110")
	form.Set("exit_quantity", "10")
	form.Set("exit_fees", "1")

	req := httptest.NewRequest(http.MethodPost, "/trades", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}

	tr, errs := buildTradeFromForm(req)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if tr.Exit == nil {
		t.Fatalf("expected exit to be parsed")
	}
	if tr.Exit.Price != 110 {
		t.Fatalf("unexpected exit price: %v", tr.Exit.Price)
	}
}

func TestHandleCreateTradePersists(t *testing.T) {
	repo := storage.NewInMemoryTradeRepository()
	svc := tradesvc.NewService(repo)
	server, err := NewServer(svc)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	form := url.Values{}
	form.Set("instrument", "EURUSD")
	form.Set("direction", "SHORT")
	form.Set("entry_date", "2023-01-02")
	form.Set("entry_price", "1.1")
	form.Set("entry_quantity", "1000")

	req := httptest.NewRequest(http.MethodPost, "/trades", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.handleCreateTrade(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	trades, err := repo.List(req.Context())
	if err != nil {
		t.Fatalf("list trades: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
}

func TestHandleUpdateTradeKeepsFollowUps(t *testing.T) {
	repo := storage.NewInMemoryTradeRepository()
	svc := tradesvc.NewService(repo)
	server, err := NewServer(svc)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	tr := &domain.Trade{Instrument: "BTCUSD", Entry: domain.EntryDetail{Date: time.Now(), Price: 20000, Quantity: 1}}
	if err := svc.Create(testContext(), tr); err != nil {
		t.Fatalf("create: %v", err)
	}
	follow := domain.FollowUp{DaysAfter: 7, Price: 22000}
	if err := svc.AddFollowUp(testContext(), tr.ID, follow); err != nil {
		t.Fatalf("add follow up: %v", err)
	}

	form := url.Values{}
	form.Set("instrument", "BTCUSD")
	form.Set("direction", "LONG")
	form.Set("entry_date", tr.Entry.Date.Format("2006-01-02"))
	form.Set("entry_price", "21000")
	form.Set("entry_quantity", "1")

	req := httptest.NewRequest(http.MethodPost, "/trades/"+tr.ID+"/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.handleUpdateTrade(rec, req, tr.ID)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	updated, err := svc.Get(req.Context(), tr.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(updated.FollowUps) != 1 {
		t.Fatalf("expected follow ups to persist")
	}
}

func testContext() context.Context {
	return httptest.NewRequest(http.MethodGet, "/", nil).Context()
}
