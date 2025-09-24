package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	tradesvc "best_trade_logs/internal/service/trade"
	"best_trade_logs/internal/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	repo, cleanup, err := setupRepository(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to setup repository: %v", err)
	}
	defer cleanup()

	svc := tradesvc.NewService(repo)
	if err := maybeSeed(ctx, svc, cfg.SeedSampleData); err != nil {
		log.Printf("sample data seeding failed: %v", err)
	}
	server, err := web.NewServer(svc)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      server.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Best Trade Logs listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
