package trade

import (
	"math"
	"time"
)

// Direction represents the direction of a trade (long or short).
type Direction string

const (
	DirectionLong  Direction = "LONG"
	DirectionShort Direction = "SHORT"
)

// EntryDetail captures information about entering a trade.
type EntryDetail struct {
	Date         time.Time `bson:"date"`
	Price        float64   `bson:"price"`
	Quantity     float64   `bson:"quantity"`
	Fees         float64   `bson:"fees"`
	StopLoss     *float64  `bson:"stop_loss"`
	Target       *float64  `bson:"target"`
	RiskPerShare *float64  `bson:"risk_per_share"`
	Notes        string    `bson:"notes"`
}

// ExitDetail captures information when closing a trade.
type ExitDetail struct {
	Date     time.Time `bson:"date"`
	Price    float64   `bson:"price"`
	Quantity float64   `bson:"quantity"`
	Fees     float64   `bson:"fees"`
	Reason   string    `bson:"reason"`
	Notes    string    `bson:"notes"`
}

// RiskManagement stores the parameters that helped manage the trade.
type RiskManagement struct {
	Thesis          string  `bson:"thesis"`
	Plan            string  `bson:"plan"`
	Checklist       string  `bson:"checklist"`
	MaxRiskAmount   float64 `bson:"max_risk_amount"`
	PositionSizing  string  `bson:"position_sizing"`
	ContingencyPlan string  `bson:"contingency_plan"`
}

// FollowUp holds post-trade tracking information.
type FollowUp struct {
	DaysAfter int       `bson:"days_after"`
	Price     float64   `bson:"price"`
	Notes     string    `bson:"notes"`
	LoggedAt  time.Time `bson:"logged_at"`
}

// TradeReview gathers lessons learnt from the trade.
type TradeReview struct {
	OutcomeSummary string   `bson:"outcome_summary"`
	Psychology     string   `bson:"psychology"`
	Improvements   string   `bson:"improvements"`
	Tags           []string `bson:"tags"`
}

// Trade is the aggregate root representing a single trade.
type Trade struct {
	ID               string         `bson:"_id,omitempty"`
	Instrument       string         `bson:"instrument"`
	Market           string         `bson:"market"`
	Direction        Direction      `bson:"direction"`
	Setup            string         `bson:"setup"`
	Entry            EntryDetail    `bson:"entry"`
	Exit             *ExitDetail    `bson:"exit"`
	RiskManagement   RiskManagement `bson:"risk_management"`
	FollowUps        []FollowUp     `bson:"follow_ups"`
	Review           TradeReview    `bson:"review"`
	CreatedAt        time.Time      `bson:"created_at"`
	UpdatedAt        time.Time      `bson:"updated_at"`
	AdditionalNotes  string         `bson:"additional_notes"`
	MarketContext    string         `bson:"market_context"`
	ExecutionScore   *float64       `bson:"execution_score"`
	ConfidenceBefore *float64       `bson:"confidence_before"`
	ConfidenceAfter  *float64       `bson:"confidence_after"`
}

// GrossExposure calculates the notional size of the trade at entry.
func (t Trade) GrossExposure() float64 {
	return math.Abs(t.Entry.Price * t.Entry.Quantity)
}

// RiskPerShare calculates the assumed risk per share based on stop loss.
func (t Trade) RiskPerShare() float64 {
	if t.Entry.RiskPerShare != nil {
		return *t.Entry.RiskPerShare
	}
	if t.Entry.StopLoss == nil {
		return 0
	}
	stop := *t.Entry.StopLoss
	if t.Direction == DirectionLong {
		return t.Entry.Price - stop
	}
	return stop - t.Entry.Price
}

// TotalRiskAmount calculates the nominal risk of the trade.
func (t Trade) TotalRiskAmount() float64 {
	return t.RiskPerShare() * t.Entry.Quantity
}

// HasExited indicates whether the trade has been closed.
func (t Trade) HasExited() bool {
	return t.Exit != nil
}

// GrossResult calculates the gross profit or loss (before fees).
func (t Trade) GrossResult() float64 {
	if t.Exit == nil {
		return 0
	}
	pnl := (t.Exit.Price - t.Entry.Price) * t.Entry.Quantity
	if t.Direction == DirectionShort {
		pnl = (t.Entry.Price - t.Exit.Price) * t.Entry.Quantity
	}
	return pnl
}

// NetResult accounts for both entry and exit fees.
func (t Trade) NetResult() float64 {
	if t.Exit == nil {
		return -t.Entry.Fees
	}
	return t.GrossResult() - t.Entry.Fees - t.Exit.Fees
}

// ResultPercent expresses the net result as a percentage of gross exposure.
func (t Trade) ResultPercent() float64 {
	exposure := t.GrossExposure()
	if exposure == 0 {
		return 0
	}
	return (t.NetResult() / exposure) * 100
}

// RMultiple calculates the result in terms of risk multiples.
func (t Trade) RMultiple() float64 {
	risk := t.TotalRiskAmount()
	if risk == 0 {
		return 0
	}
	return t.NetResult() / risk
}

// FollowUpChangePercent returns the percentage change between the exit price
// and a follow-up observation at the specified number of days.
func (t Trade) FollowUpChangePercent(daysAfter int) (float64, bool) {
	if t.Exit == nil {
		return 0, false
	}
	for _, f := range t.FollowUps {
		if f.DaysAfter == daysAfter {
			if t.Exit.Price == 0 {
				return 0, true
			}
			change := ((f.Price - t.Exit.Price) / t.Exit.Price) * 100
			if t.Direction == DirectionShort {
				change = ((t.Exit.Price - f.Price) / t.Exit.Price) * 100
			}
			return change, true
		}
	}
	return 0, false
}

// UnrealizedResult calculates P/L using the latest close price provided.
func (t Trade) UnrealizedResult(closePrice float64) float64 {
	if t.HasExited() {
		return t.NetResult()
	}
	pnl := (closePrice - t.Entry.Price) * t.Entry.Quantity
	if t.Direction == DirectionShort {
		pnl = (t.Entry.Price - closePrice) * t.Entry.Quantity
	}
	return pnl - t.Entry.Fees
}

// UnrealizedPercent calculates the unrealized return percentage.
func (t Trade) UnrealizedPercent(closePrice float64) float64 {
	exposure := t.GrossExposure()
	if exposure == 0 {
		return 0
	}
	return (t.UnrealizedResult(closePrice) / exposure) * 100
}

// EffectiveRewardTarget calculates the R multiple of the target price when provided.
func (t Trade) EffectiveRewardTarget() float64 {
	if t.Entry.Target == nil {
		return 0
	}
	target := *t.Entry.Target
	pnl := (target - t.Entry.Price) * t.Entry.Quantity
	if t.Direction == DirectionShort {
		pnl = (t.Entry.Price - target) * t.Entry.Quantity
	}
	risk := t.TotalRiskAmount()
	if risk == 0 {
		return 0
	}
	return pnl / risk
}
