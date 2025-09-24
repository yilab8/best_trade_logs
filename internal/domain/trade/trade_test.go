package trade

import (
	"math"
	"testing"
	"time"
)

func TestNetResultLong(t *testing.T) {
	exit := &ExitDetail{Date: time.Now(), Price: 120, Quantity: 10, Fees: 2}
	tr := Trade{
		Direction: DirectionLong,
		Entry:     EntryDetail{Price: 100, Quantity: 10, Fees: 1},
		Exit:      exit,
	}
	got := tr.NetResult()
	want := ((120.0 - 100.0) * 10.0) - 1.0 - 2.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("unexpected net result: got %v want %v", got, want)
	}
}

func TestNetResultShort(t *testing.T) {
	exit := &ExitDetail{Date: time.Now(), Price: 80, Quantity: 5, Fees: 3}
	tr := Trade{
		Direction: DirectionShort,
		Entry:     EntryDetail{Price: 100, Quantity: 5, Fees: 1.5},
		Exit:      exit,
	}
	got := tr.NetResult()
	want := ((100.0 - 80.0) * 5.0) - 1.5 - 3.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("unexpected net result: got %v want %v", got, want)
	}
}

func TestRMultiple(t *testing.T) {
	stop := 95.0
	tr := Trade{
		Direction: DirectionLong,
		Entry:     EntryDetail{Price: 100, Quantity: 10, Fees: 0.5, StopLoss: &stop},
	}
	exit := &ExitDetail{Price: 115, Quantity: 10, Fees: 0.5}
	tr.Exit = exit
	if got := tr.RiskPerShare(); got != 5 {
		t.Fatalf("expected risk per share 5, got %v", got)
	}
	wantRisk := 50.0
	if got := tr.TotalRiskAmount(); got != wantRisk {
		t.Fatalf("expected total risk %v, got %v", wantRisk, got)
	}
	wantR := (((115.0 - 100.0) * 10.0) - 1.0) / wantRisk
	if math.Abs(tr.RMultiple()-wantR) > 1e-9 {
		t.Fatalf("unexpected r multiple: got %v want %v", tr.RMultiple(), wantR)
	}
}

func TestFollowUpChangePercent(t *testing.T) {
	exit := &ExitDetail{Price: 100, Quantity: 10}
	tr := Trade{
		Direction: DirectionLong,
		Entry:     EntryDetail{Price: 80, Quantity: 10},
		Exit:      exit,
		FollowUps: []FollowUp{{DaysAfter: 7, Price: 120}},
	}
	pct, ok := tr.FollowUpChangePercent(7)
	if !ok {
		t.Fatalf("expected follow up data")
	}
	if math.Abs(pct-20.0) > 1e-9 {
		t.Fatalf("unexpected follow up pct: got %v", pct)
	}
}

func TestUnrealizedResultForOpenTrade(t *testing.T) {
	tr := Trade{
		Direction: DirectionShort,
		Entry:     EntryDetail{Price: 50, Quantity: 100, Fees: 5},
	}
	got := tr.UnrealizedResult(40)
	want := ((50.0 - 40.0) * 100.0) - 5.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("unexpected unrealized result: got %v want %v", got, want)
	}
}
