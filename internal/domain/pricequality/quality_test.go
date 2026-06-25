package pricequality

import (
	"testing"
	"time"
)

func TestReconcileWithOfficialFinalClosePromotesVerifiedClose(t *testing.T) {
	ts := time.Date(2026, 6, 20, 1, 25, 0, 0, time.UTC)
	report := SymbolReport{
		Symbol:                "ASELS",
		Status:                StatusProvisionalLastPrice,
		ReadyForDecision:      true,
		ReadyForVerifiedClose: false,
		MissingFields:         []string{"official_final_close"},
		BlockingReasons:       []string{"official_final_close_missing"},
		Candidates: []CloseCandidate{
			{
				Source:      "market_ws",
				SourceType:  "market_ws",
				Close:       402.50,
				Timestamp:   &ts,
				FetchedAt:   &ts,
				TradingDate: "2026-06-20",
			},
		},
	}

	got := ReconcileWithOfficialFinalClose(report, 402.50, "2026-06-19", ts, ts, "bist_thb_official_bulletin", "official.json", DefaultConflictToleranceBps)

	if got.Status != StatusReadyForVerifiedClose || !got.ReadyForVerifiedClose {
		t.Fatalf("report = %+v, want verified close", got)
	}
	if got.SelectedClose == nil || got.SelectedClose.SourceType != "official_final_close" {
		t.Fatalf("selected close = %+v, want official final close", got.SelectedClose)
	}
	if contains(got.BlockingReasons, "official_final_close_missing") {
		t.Fatalf("official close blocker remained: %+v", got.BlockingReasons)
	}
}

func TestReconcileWithAnalysisClosePreservesConflictGate(t *testing.T) {
	ts := time.Date(2026, 6, 19, 18, 0, 0, 0, time.UTC)
	report := SymbolReport{
		Symbol: "ASELS",
		Candidates: []CloseCandidate{
			{Source: "official", SourceType: "official_final_close", Close: 100, Timestamp: &ts, TradingDate: "2026-06-19", Official: true, Final: true},
		},
	}

	got := ReconcileWithAnalysisClose(report, 102, ts, ts, "analysis_provider", DefaultConflictToleranceBps)

	if !got.Conflict || got.Status != StatusStaleOrConflicting {
		t.Fatalf("report = %+v, want conflict gate", got)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
