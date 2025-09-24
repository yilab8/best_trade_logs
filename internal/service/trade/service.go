package trade

import (
	"context"
	"sort"
	"strings"
	"time"

	domain "best_trade_logs/internal/domain/trade"
	"best_trade_logs/internal/storage"
)

// Service coordinates higher-level trade workflows.
type Service struct {
	repo storage.TradeRepository
}

// NewService creates a trade service with the provided repository.
func NewService(repo storage.TradeRepository) *Service {
	return &Service{repo: repo}
}

// Create persists a new trade.
func (s *Service) Create(ctx context.Context, tr *domain.Trade) error {
	tr.CreatedAt = time.Now().UTC()
	tr.UpdatedAt = tr.CreatedAt
	normalize(tr)
	return s.repo.Create(ctx, tr)
}

// Update modifies an existing trade.
func (s *Service) Update(ctx context.Context, tr *domain.Trade) error {
	tr.UpdatedAt = time.Now().UTC()
	normalize(tr)
	return s.repo.Update(ctx, tr)
}

// Delete removes a trade by ID.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Get fetches a trade by ID.
func (s *Service) Get(ctx context.Context, id string) (*domain.Trade, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all trades sorted by creation date desc.
func (s *Service) List(ctx context.Context) ([]*domain.Trade, error) {
	trades, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(trades, func(i, j int) bool {
		return trades[i].CreatedAt.After(trades[j].CreatedAt)
	})
	return trades, nil
}

// AddFollowUp records a follow-up observation for the trade.
func (s *Service) AddFollowUp(ctx context.Context, tradeID string, followUp domain.FollowUp) error {
	tr, err := s.repo.GetByID(ctx, tradeID)
	if err != nil {
		return err
	}
	followUp.LoggedAt = time.Now().UTC()
	tr.FollowUps = append(tr.FollowUps, followUp)
	tr.UpdatedAt = followUp.LoggedAt
	normalize(tr)
	return s.repo.Update(ctx, tr)
}

func normalize(tr *domain.Trade) {
	if tr.Review.Tags != nil {
		cleaned := make([]string, 0, len(tr.Review.Tags))
		for _, tag := range tr.Review.Tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				cleaned = append(cleaned, strings.ToLower(tag))
			}
		}
		tr.Review.Tags = cleaned
	}
}
