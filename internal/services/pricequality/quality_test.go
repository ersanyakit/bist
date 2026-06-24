package pricequality

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/util"
)

func TestBuildReportsVerifiedCloseOnlyWithOfficialFinalSource(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	mkdir(t, filepath.Join(equitiesDir, "AAA", "price"))
	writeFile(t, filepath.Join(equitiesDir, "AAA", "equity.json"), `{"ticker":"AAA"}`)
	sourceTime := time.Date(2026, 6, 18, 15, 0, 0, 0, time.UTC)
	writeJSON(t, filepath.Join(equitiesDir, "AAA", "price", "official_close.json"), map[string]any{
		"source":           "bist_official",
		"symbol":           "AAA",
		"trading_date":     "2026-06-18",
		"close":            100.5,
		"is_final_close":   true,
		"source_timestamp": sourceTime,
		"fetched_at":       sourceTime.Add(10 * time.Minute),
	})
	writeJSON(t, filepath.Join(equitiesDir, "AAA", "ohlcv.json"), map[string]any{
		"source":     "tradingview",
		"symbol":     "BIST:AAA",
		"close":      100.5,
		"time":       sourceTime.Unix(),
		"fetched_at": sourceTime.Add(5 * time.Minute),
	})

	report, err := Build(context.Background(), Options{
		EquitiesDir: equitiesDir,
		StaleAfter:  48 * time.Hour,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.FinalClose.ReadySymbols != 1 {
		t.Fatalf("ready symbols = %d, want 1", report.FinalClose.ReadySymbols)
	}
	if len(report.Symbols) != 1 || report.Symbols[0].Status != StatusReadyForVerifiedClose {
		t.Fatalf("unexpected symbol report = %+v", report.Symbols)
	}
	if !report.QualityGates.FinalCloseSourceAvailable {
		t.Fatalf("expected final close source gate to be available")
	}
}

func TestBuildBlocksStaleAndConflictingPriceSources(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	mkdir(t, filepath.Join(equitiesDir, "AAA", "charts"))
	mkdir(t, filepath.Join(equitiesDir, "AAA", "market_ws"))
	mkdir(t, filepath.Join(equitiesDir, "BBB"))
	writeFile(t, filepath.Join(equitiesDir, "AAA", "equity.json"), `{"ticker":"AAA"}`)
	writeFile(t, filepath.Join(equitiesDir, "BBB", "equity.json"), `{"ticker":"BBB"}`)
	oldTime := time.Date(2026, 6, 10, 6, 0, 0, 0, time.UTC)
	writeJSON(t, filepath.Join(equitiesDir, "AAA", "charts", "D.json"), map[string]any{
		"source":     "tradingview",
		"ticker":     "AAA",
		"interval":   "D",
		"fetched_at": oldTime.Add(2 * time.Hour),
		"candles": []map[string]any{
			{"time": oldTime.Unix(), "close": 10.0},
		},
	})
	writeJSON(t, filepath.Join(equitiesDir, "AAA", "charts", "D_adjusted.json"), map[string]any{
		"source":       "hissebot_corporate_actions_adjustment",
		"ticker":       "AAA",
		"interval":     "D",
		"generated_at": oldTime.Add(2 * time.Hour),
		"candles": []map[string]any{
			{"time": oldTime.Format(time.RFC3339), "close": 10.0},
		},
	})
	writeJSON(t, filepath.Join(equitiesDir, "AAA", "market_ws", "symbols_summary_data.json"), []map[string]any{
		{
			"symbol":     "AAA",
			"updated_at": oldTime.Add(3 * time.Hour),
			"data": map[string]any{
				"code":  "AAA",
				"close": 8.0,
			},
		},
	})

	report, err := Build(context.Background(), Options{
		EquitiesDir:          equitiesDir,
		StaleAfter:           24 * time.Hour,
		ConflictToleranceBps: 10,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalSymbols != 2 {
		t.Fatalf("total symbols = %d, want 2", report.TotalSymbols)
	}
	if report.FinalClose.BlockedSymbols != 2 {
		t.Fatalf("blocked symbols = %d, want 2", report.FinalClose.BlockedSymbols)
	}
	if report.Reconciliation.ConflictSymbols != 1 {
		t.Fatalf("conflict symbols = %d, want 1", report.Reconciliation.ConflictSymbols)
	}
	if report.Reconciliation.StaleSymbols != 1 {
		t.Fatalf("stale symbols = %d, want 1", report.Reconciliation.StaleSymbols)
	}
	if report.CandidateSources.AdjustedDailyCharts != 1 {
		t.Fatalf("adjusted chart symbols = %d, want 1", report.CandidateSources.AdjustedDailyCharts)
	}
	if got := report.MissingCounts["missing_price_data"]; got != 1 {
		t.Fatalf("missing price data count = %d, want 1", got)
	}
	var aaa SymbolReport
	for _, symbol := range report.Symbols {
		if symbol.Symbol == "AAA" {
			aaa = symbol
		}
	}
	if aaa.Status != StatusStaleOrConflicting || !aaa.Conflict || !aaa.Stale {
		t.Fatalf("unexpected AAA report = %+v", aaa)
	}
}

func TestReconcileWithAnalysisCloseUsesCurrentAnalysisCandidateButKeepsGateBlocked(t *testing.T) {
	oldTime := time.Date(2026, 6, 18, 7, 0, 0, 0, time.UTC)
	report := SymbolReport{
		Symbol: "AAA",
		Status: StatusProvisionalLastPrice,
		Candidates: []CloseCandidate{
			{
				Source:      "market_ws",
				SourceType:  "market_ws",
				Close:       393.25,
				Timestamp:   &oldTime,
				FetchedAt:   &oldTime,
				TradingDate: "2026-06-18",
			},
		},
		MissingFields: []string{"official_final_close"},
	}
	candleTime := time.Date(2026, 6, 19, 6, 0, 0, 0, time.UTC)
	reconciled := ReconcileWithAnalysisClose(report, 402.50, candleTime, candleTime.Add(2*time.Hour), "analysis_provider", DefaultConflictToleranceBps)

	if reconciled.SelectedClose == nil {
		t.Fatalf("selected close missing: %+v", reconciled)
	}
	if reconciled.SelectedClose.SourceType != "analysis_ohlcv" || reconciled.SelectedClose.Close != 402.50 {
		t.Fatalf("selected close = %+v, want analysis_ohlcv 402.50", reconciled.SelectedClose)
	}
	if reconciled.ReadyForVerifiedClose {
		t.Fatalf("analysis close must not be treated as official verified close: %+v", reconciled)
	}
	if reconciled.Status != StatusProvisionalLastPrice {
		t.Fatalf("status = %s, want provisional", reconciled.Status)
	}
	if !containsString(reconciled.BlockingReasons, "official_final_close_missing") {
		t.Fatalf("official final close blocker missing: %+v", reconciled.BlockingReasons)
	}
}

func TestReconcileWithOfficialFinalCloseUsesOfficialCandidateOverWeekendSnapshot(t *testing.T) {
	weekend := time.Date(2026, 6, 20, 1, 25, 0, 0, time.UTC)
	report := SymbolReport{
		Symbol: "ASELS",
		Status: StatusProvisionalLastPrice,
		Candidates: []CloseCandidate{
			{
				Source:      "market_ws",
				SourceType:  "market_ws",
				Close:       402.50,
				Timestamp:   &weekend,
				FetchedAt:   &weekend,
				TradingDate: "2026-06-20",
			},
		},
		MissingFields:   []string{"official_final_close"},
		BlockingReasons: []string{"official_final_close_missing"},
	}
	officialTime := time.Date(2026, 6, 19, 15, 10, 0, 0, time.UTC)
	reconciled := ReconcileWithOfficialFinalClose(report, 402.50, "2026-06-19", officialTime, officialTime, "bist_thb_official_bulletin", "thb202606191.csv", DefaultConflictToleranceBps)

	if reconciled.SelectedClose == nil {
		t.Fatalf("selected close missing: %+v", reconciled)
	}
	if reconciled.SelectedClose.SourceType != "official_final_close" || !reconciled.SelectedClose.Official || !reconciled.SelectedClose.Final {
		t.Fatalf("selected close is not official final: %+v", reconciled.SelectedClose)
	}
	if reconciled.LatestTradingDate != "2026-06-19" {
		t.Fatalf("latest trading date = %s, want selected official close date", reconciled.LatestTradingDate)
	}
	if reconciled.Status != StatusReadyForVerifiedClose || !reconciled.ReadyForVerifiedClose {
		t.Fatalf("official close should verify price quality: %+v", reconciled)
	}
	if containsString(reconciled.BlockingReasons, "official_final_close_missing") || containsString(reconciled.MissingFields, "official_final_close") {
		t.Fatalf("official close blocker remained: %+v", reconciled)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := util.WriteJSON(path, value); err != nil {
		t.Fatal(err)
	}
}
