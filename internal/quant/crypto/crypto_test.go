package crypto

import "testing"

func TestEvaluateCryptoProfile(t *testing.T) {
	got := Evaluate(Input{
		OnChainAvailable:        true,
		DerivativesAvailable:    true,
		ExchangeFlowAvailable:   true,
		NewsSentimentAvailable:  true,
		ContextCoverageScore:     85,
		BenchmarkAvailable:      true,
		Beta60:                  1.05,
		RelativeStrength60Pct:   6,
		Return60Pct:             18,
		AnnualizedVolatilityPct: 72,
		HistoricalVaR95Pct:      5,
		MaxDrawdownLossPct:      32,
	})
	if !got.Computed || got.Status != "pass" {
		t.Fatalf("unexpected crypto status: %+v", got)
	}
	if got.AnnualizationDays != AnnualizationDays || got.MarketClock != "24_7" {
		t.Fatalf("unexpected crypto clock: %+v", got)
	}
	if got.RiskBudgetMultiplier <= 0 || got.MarketStructureLabel != "complete" {
		t.Fatalf("unexpected crypto profile: %+v", got)
	}
}
