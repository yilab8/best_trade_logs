package main

import (
	"context"
	"fmt"
	"log"
	"time"

	domain "best_trade_logs/internal/domain/trade"
	tradesvc "best_trade_logs/internal/service/trade"
)

func maybeSeed(ctx context.Context, svc *tradesvc.Service, enabled bool) error {
	if !enabled {
		return nil
	}

	existing, err := svc.List(ctx)
	if err != nil {
		return fmt.Errorf("check existing trades: %w", err)
	}
	if len(existing) > 0 {
		log.Printf("seed: skipped because %d trades already exist", len(existing))
		return nil
	}

	samples := sampleTrades()
	for _, tr := range samples {
		if err := svc.Create(ctx, tr); err != nil {
			return fmt.Errorf("create sample trade: %w", err)
		}
	}
	log.Printf("seed: inserted %d sample trades", len(samples))
	return nil
}

func sampleTrades() []*domain.Trade {
	now := time.Now().UTC()

	floatPtr := func(v float64) *float64 { return &v }

	exit1Date := now.AddDate(0, 0, -5)
	entry1Date := exit1Date.AddDate(0, 0, -7)

	first := &domain.Trade{
		Instrument: "AAPL",
		Market:     "NASDAQ",
		Direction:  domain.DirectionLong,
		Setup:      "Earnings breakout",
		Entry: domain.EntryDetail{
			Date:         entry1Date,
			Price:        180.50,
			Quantity:     100,
			Fees:         4.5,
			StopLoss:     floatPtr(172.00),
			Target:       floatPtr(195.00),
			Notes:        "Entry on strong gap continuation above pre-market range.",
			RiskPerShare: nil,
		},
		Exit: &domain.ExitDetail{
			Date:     exit1Date,
			Price:    190.20,
			Quantity: 100,
			Fees:     5.25,
			Reason:   "Scaled out as price tagged measured move target.",
			Notes:    "Could have kept a runner but respected plan.",
		},
		RiskManagement: domain.RiskManagement{
			Thesis:          "Institutional participation expected post-earnings beat.",
			Plan:            "Enter on first pullback with volume confirmation.",
			Checklist:       "Market uptrend, sector strength, catalyst in play.",
			MaxRiskAmount:   800,
			PositionSizing:  "1R = 8 points, 100 shares.",
			ContingencyPlan: "Cut half if intraday VWAP lost.",
		},
		FollowUps: []domain.FollowUp{
			{
				DaysAfter: 7,
				Price:     192.50,
				Notes:     "Price consolidated above breakout level.",
				LoggedAt:  exit1Date.AddDate(0, 0, 7),
			},
			{
				DaysAfter: 30,
				Price:     205.10,
				Notes:     "Another leg higher once indices reclaimed highs.",
				LoggedAt:  exit1Date.AddDate(0, 0, 30),
			},
		},
		Review: domain.TradeReview{
			OutcomeSummary: "Plan followed, partials executed cleanly.",
			Psychology:     "Calm at open thanks to prep; minor FOMO on runner.",
			Improvements:   "Consider leaving a 10% runner when trend context strong.",
			Tags:           []string{"Earnings", "Breakout", "Swing"},
		},
		MarketContext:    "S&P 500 reclaiming 50DMA with tech leadership.",
		AdditionalNotes:  "Watch for post-breakout digestion patterns.",
		ExecutionScore:   floatPtr(8.5),
		ConfidenceBefore: floatPtr(7.5),
		ConfidenceAfter:  floatPtr(9.0),
	}

	entry2Date := now.AddDate(0, 0, -3)
	second := &domain.Trade{
		Instrument: "CL Futures",
		Market:     "NYMEX",
		Direction:  domain.DirectionShort,
		Setup:      "Daily lower high after failed breakout",
		Entry: domain.EntryDetail{
			Date:         entry2Date,
			Price:        78.40,
			Quantity:     2,
			Fees:         6.0,
			StopLoss:     floatPtr(80.10),
			Target:       floatPtr(73.80),
			Notes:        "Shorted retest of broken trendline with weakening momentum.",
			RiskPerShare: nil,
		},
		RiskManagement: domain.RiskManagement{
			Thesis:          "Supply rebuild and dollar strength could pressure crude.",
			Plan:            "Add on breakdown below 77.50 if volume accelerates.",
			Checklist:       "Macro alignment, inventory trend, sentiment extremes.",
			MaxRiskAmount:   650,
			PositionSizing:  "2 contracts = ~$3,400 exposure per point.",
			ContingencyPlan: "Stop and reassess if 4H closes above 79.80.",
		},
		Review: domain.TradeReview{
			OutcomeSummary: "Open position with partial profit potential.",
			Psychology:     "Confident after pre-market plan review.",
			Improvements:   "Trail stop intraday once price moves 1R.",
			Tags:           []string{"Trend", "Commodities"},
		},
		MarketContext:    "Dollar index bouncing, energy sector lagging broader market.",
		AdditionalNotes:  "Monitor OPEC headlines and EIA release mid-week.",
		ExecutionScore:   floatPtr(7.0),
		ConfidenceBefore: floatPtr(8.0),
	}

	return []*domain.Trade{first, second}
}
