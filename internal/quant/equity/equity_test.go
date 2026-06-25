package equity

import "testing"

func TestEvaluateEquityProfile(t *testing.T) {
	got := Evaluate(Input{
		VerifiedCloseAvailable:   true,
		BenchmarkAvailable:       true,
		FinancialEvidencePresent: true,
		KAPPDFEvidencePresent:    true,
		Beta60:                   0.95,
		RelativeStrength60Pct:    4,
		Return60Pct:              8,
		AnnualizedVolatilityPct:  28,
		HistoricalVaR95Pct:       3,
		MaxDrawdownLossPct:       18,
	})
	if !got.Computed || got.Status != "pass" {
		t.Fatalf("unexpected equity status: %+v", got)
	}
	if got.AnnualizationDays != AnnualizationDays || got.MarketClock != "exchange_sessions" {
		t.Fatalf("unexpected equity clock: %+v", got)
	}
	if got.RiskBudgetMultiplier <= 0.5 || len(got.MissingData) != 0 {
		t.Fatalf("unexpected equity risk budget: %+v", got)
	}
}
