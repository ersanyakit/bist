package labels

import (
	"math"
	"testing"
	"time"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/forecastpolicy"
)

func TestDefaultOptionsUseSharedForecastDirectionTolerance(t *testing.T) {
	if got, want := DefaultOptions().DirectionThreshold, forecastpolicy.NextSessionDirectionToleranceReturn(); got != want {
		t.Fatalf("direction threshold=%v, want %v", got, want)
	}
}

func TestBuildTargetNextSessionLabels(t *testing.T) {
	bars := []features.MarketBar{
		bar("2026-06-18", 100, 105, 99, 100),
		bar("2026-06-19", 102, 110, 101, 108),
	}
	asOf, _ := time.Parse("2006-01-02", "2026-06-18")
	target, ok := BuildTarget("ASELS", bars, asOf, Options{DirectionThreshold: 0.01, ProfitTaking: 0.05, StopLoss: 0.02, MaxHoldingBars: 1})
	if !ok {
		t.Fatal("target unavailable")
	}
	if math.Abs(target.OpenReturn-0.02) > 1e-9 {
		t.Fatalf("open return=%v", target.OpenReturn)
	}
	if target.Direction != "up" {
		t.Fatalf("direction=%s", target.Direction)
	}
	if target.TripleBarrierLabel != "take_profit" {
		t.Fatalf("triple barrier=%s", target.TripleBarrierLabel)
	}
	if target.MetaLabel != 1 {
		t.Fatalf("meta label=%d", target.MetaLabel)
	}
}

func TestDirectionThresholdUsesATR(t *testing.T) {
	bars := []features.MarketBar{
		bar("2026-06-15", 100, 105, 95, 100),
		bar("2026-06-16", 100, 106, 94, 101),
		bar("2026-06-17", 101, 108, 98, 102),
	}
	got := DirectionThreshold(bars, Options{DirectionThreshold: 0.001, UseATRThreshold: true, ThresholdMultiplier: 0.5})
	if got <= 0.001 {
		t.Fatalf("threshold=%v, expected ATR-adjusted", got)
	}
}

func bar(date string, open, high, low, close float64) features.MarketBar {
	t, _ := time.Parse("2006-01-02", date)
	return features.MarketBar{Symbol: "ASELS", Time: t, Open: open, High: high, Low: low, Close: close, Volume: 1000}
}
