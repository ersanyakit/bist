package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/datasource"
)

func TestBuildForecastLedgerEntriesCreatesPointInTimeRecords(t *testing.T) {
	records := testForecastDailyRecords(36)
	from := mustForecastDate(t, records[31].TradingDate)
	to := mustForecastDate(t, records[35].TradingDate)
	builder := func(ctx context.Context, symbol string, asOf datasource.DailyBulletinRecord, forecastFor string, prefixRecords []datasource.DailyBulletinRecord) (forecastAuditRangeForecastResult, error) {
		return forecastAuditRangeForecastResult{
			Forecast: testForecast(asOf.Close, forecastFor),
			Context:  "test_context",
		}, nil
	}

	entries, err := buildForecastLedgerEntries(context.Background(), "ASELS", from, to, records, builder)
	if err != nil {
		t.Fatalf("build entries: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("entries=%d, want 5", len(entries))
	}
	first := entries[0]
	if first.Symbol != "ASELS" || first.AsOfDate >= first.TargetDate || first.Target != "close_t1" {
		t.Fatalf("unexpected identity fields: %+v", first)
	}
	if !first.OnlyUsesDataThroughAsOf || !first.ActualUsedOnlyForVerification {
		t.Fatalf("point-in-time flags missing: %+v", first)
	}
	if first.DataSnapshotHash == "" || first.EntryID == "" {
		t.Fatalf("snapshot/id missing: %+v", first)
	}
	if first.IntervalLow == 0 || first.IntervalHigh == 0 || first.PredictedClose == 0 {
		t.Fatalf("forecast values missing: %+v", first)
	}
}

func TestForecastLedgerSuppressesInvalidScenarioPointWhileKeepingRawForecast(t *testing.T) {
	records := testForecastDailyRecords(31)
	asOf := records[len(records)-1]
	forecast := testForecast(asOf.Close, "2026-06-10")
	forecast.PointForecastPublishable = false
	forecast.PointForecastStatus = "not_published"
	forecast.PointForecastSuppressionReason = "backtest_close_mape_above_2pct:2.60"
	forecast.Status = "model_validation_failed"
	forecast.Quality = "not_decision_grade"
	forecast.BacktestMetrics.CloseMAPE = 2.6
	forecast.BacktestMetrics.DirectionAccuracy = 43

	entry := forecastLedgerEntryFromForecast("ASELS", asOf, "2026-06-10", records, forecastAuditRangeForecastResult{
		Forecast: forecast,
		Context:  "test_context",
	})

	if entry.PredictedClose <= 0 {
		t.Fatalf("exact model point forecast must stay available: %+v", entry)
	}
	if entry.PublishedPredictedClose != nil {
		t.Fatalf("non-publishable forecast must not expose published_predicted_close: %+v", entry)
	}
	if entry.ScenarioPredictedClose != 0 || entry.ScenarioPointAvailable {
		t.Fatalf("invalid scenario point must not be emitted when validation fails: %+v", entry)
	}
	if entry.RawForecast.PredictedClose <= 0 {
		t.Fatalf("raw forecast should remain available for internal audit/debug: %+v", entry.RawForecast)
	}
	if entry.ScenarioPointStatus != "band_only_validation_failed" || entry.ScenarioPointSuppressionReason == "" {
		t.Fatalf("expected band-only scenario suppression reason: %+v", entry)
	}
}

func TestBuildForecastVerificationEventsComputesOfficialCloseMetrics(t *testing.T) {
	entry := forecastLedgerEntry{
		SchemaVersion:            forecastLedgerSchemaVersion,
		EntryID:                  "entry-1",
		Symbol:                   "ASELS",
		AsOfDate:                 "2026-06-01",
		TargetDate:               "2026-06-02",
		PredictedClose:           103,
		LastClose:                100,
		IntervalLow:              101,
		IntervalHigh:             103.5,
		DataSnapshotHash:         "hash",
		PointForecastPublishable: true,
	}
	records := []datasource.DailyBulletinRecord{{Symbol: "ASELS", TradingDate: "2026-06-02", Close: 104, SourcePath: "bist.csv"}}
	from := mustForecastDate(t, "2026-06-02")
	to := mustForecastDate(t, "2026-06-02")

	events := buildForecastVerificationEvents([]forecastLedgerEntry{entry}, records, from, to)
	if len(events) != 1 {
		t.Fatalf("events=%d, want 1", len(events))
	}
	got := events[0]
	if got.Status != "verified" || got.ActualClose != 104 || got.CloseAbsErrorPct == 0 {
		t.Fatalf("verification metrics missing: %+v", got)
	}
	if got.CloseDirectionHit == nil || !*got.CloseDirectionHit {
		t.Fatalf("expected direction hit: %+v", got)
	}
	if got.IntervalHit == nil || *got.IntervalHit {
		t.Fatalf("expected interval miss: %+v", got)
	}
}

func TestBuildForecastVerificationEventsKeepsScenarioOnlyMetrics(t *testing.T) {
	entry := forecastLedgerEntry{
		SchemaVersion:                  forecastLedgerSchemaVersion,
		EntryID:                        "entry-1",
		Symbol:                         "ASELS",
		AsOfDate:                       "2026-06-01",
		TargetDate:                     "2026-06-02",
		PredictedClose:                 103,
		ScenarioPredictedClose:         103,
		LastClose:                      100,
		IntervalLow:                    98,
		IntervalHigh:                   105,
		DataSnapshotHash:               "hash",
		PointForecastPublishable:       false,
		ScenarioPointAvailable:         true,
		ScenarioPointStatus:            "research_scenario_point",
		PointForecastSuppressionReason: "backtest_close_mape_above_2pct:2.60",
	}
	records := []datasource.DailyBulletinRecord{{Symbol: "ASELS", TradingDate: "2026-06-02", Close: 104, SourcePath: "bist.csv"}}
	from := mustForecastDate(t, "2026-06-02")
	to := mustForecastDate(t, "2026-06-02")

	events := buildForecastVerificationEvents([]forecastLedgerEntry{entry}, records, from, to)
	if len(events) != 1 {
		t.Fatalf("events=%d, want 1", len(events))
	}
	got := events[0]
	if got.Status != "verified_scenario_only" || got.VerificationMode != "scenario_only" {
		t.Fatalf("expected scenario-only verification: %+v", got)
	}
	if got.PredictedClose != 103 || got.ScenarioPredictedClose != 103 {
		t.Fatalf("exact/scenario close fields missing: %+v", got)
	}
	if got.ScenarioCloseAbsErrorPct == 0 || got.CloseAbsErrorPct != got.ScenarioCloseAbsErrorPct {
		t.Fatalf("scenario audit metrics missing: %+v", got)
	}
}

func TestBuildForecastVerificationEventsKeepsBandOnlyOutOfPointMetrics(t *testing.T) {
	entry := forecastLedgerEntry{
		SchemaVersion:                  forecastLedgerSchemaVersion,
		EntryID:                        "entry-1",
		Symbol:                         "ASELS",
		AsOfDate:                       "2026-06-01",
		TargetDate:                     "2026-06-02",
		PredictedClose:                 0,
		ScenarioPredictedClose:         0,
		LastClose:                      100,
		IntervalLow:                    95,
		IntervalHigh:                   105,
		DecisionIntervalLow:            98,
		DecisionIntervalHigh:           105,
		DecisionIntervalWidthPct:       7,
		DecisionIntervalStatus:         "candidate_validation_failed",
		DecisionIntervalReason:         "conformal_q80_close_error_pct:3.50",
		DataSnapshotHash:               "hash",
		PointForecastPublishable:       false,
		ScenarioPointAvailable:         false,
		ScenarioPointStatus:            "band_only",
		ScenarioPointSuppressionReason: "scenario_point_close_mape_above_2pct:2.60",
		PointForecastSuppressionReason: "backtest_close_mape_above_2pct:2.60",
	}
	records := []datasource.DailyBulletinRecord{{Symbol: "ASELS", TradingDate: "2026-06-02", Close: 104, SourcePath: "bist.csv"}}
	from := mustForecastDate(t, "2026-06-02")
	to := mustForecastDate(t, "2026-06-02")

	events := buildForecastVerificationEvents([]forecastLedgerEntry{entry}, records, from, to)
	if len(events) != 1 {
		t.Fatalf("events=%d, want 1", len(events))
	}
	got := events[0]
	if got.Status != "verified_band_only" || got.VerificationMode != "band_only" {
		t.Fatalf("expected band-only verification: %+v", got)
	}
	if got.CloseAbsErrorPct != 0 || got.ScenarioCloseAbsErrorPct != 0 || got.CloseDirectionHit != nil {
		t.Fatalf("band-only verification must not emit point error metrics: %+v", got)
	}
	if got.IntervalHit == nil || !*got.IntervalHit {
		t.Fatalf("expected interval hit for band-only verification: %+v", got)
	}
	if got.DecisionIntervalHit == nil || !*got.DecisionIntervalHit {
		t.Fatalf("expected decision interval hit for band-only verification: %+v", got)
	}
	if got.DecisionIntervalStatus != "candidate_validation_failed" || got.DecisionIntervalWidthPct != 7 {
		t.Fatalf("decision interval metadata was not carried forward: %+v", got)
	}
}

func TestBuildForecastErrorAuditReportAttributesLargeErrors(t *testing.T) {
	directionHit := false
	intervalHit := false
	events := []forecastVerificationEvent{
		{
			EntryID:                   "entry-1",
			Symbol:                    "ASELS",
			AsOfDate:                  "2026-06-01",
			TargetDate:                "2026-06-02",
			Status:                    "verified",
			PredictedClose:            103,
			ScenarioPredictedClose:    103,
			ActualClose:               100,
			CloseAbsErrorPct:          3,
			CloseDirectionHit:         &directionHit,
			IntervalHit:               &intervalHit,
			PointForecastPublishable:  true,
			BacktestCloseMAPE:         2.5,
			BacktestDirectionAccuracy: 50,
		},
	}
	from := mustForecastDate(t, "2026-06-01")
	to := mustForecastDate(t, "2026-06-30")

	report := buildForecastErrorAuditReport("ASELS", from, to, events, 1)
	if report.Summary.VerifiedEvents != 1 || report.Summary.ErrorEvents != 1 {
		t.Fatalf("summary mismatch: %+v", report.Summary)
	}
	if report.Summary.PublishedEvents != 1 || report.Summary.ScenarioErrorEvents != 0 {
		t.Fatalf("published/scenario counters mixed: %+v", report.Summary)
	}
	if len(report.Events) != 1 || report.Events[0].PrimaryCause != "direction_model_miss" {
		t.Fatalf("unexpected attribution: %+v", report.Events)
	}
	if len(report.ScenarioEvents) != 0 || report.Events[0].EventType != "published_forecast_error" || !report.Events[0].ForecastPublished {
		t.Fatalf("published attribution classified incorrectly: %+v", report)
	}
	if report.Events[0].RecommendedFix == "" {
		t.Fatalf("recommended fix missing: %+v", report.Events[0])
	}
}

func TestBuildForecastErrorAuditReportSeparatesScenarioOnlyDrift(t *testing.T) {
	directionHit := false
	intervalHit := true
	events := []forecastVerificationEvent{
		{
			EntryID:                   "entry-1",
			Symbol:                    "ASELS",
			AsOfDate:                  "2026-06-01",
			TargetDate:                "2026-06-02",
			Status:                    "verified_scenario_only",
			VerificationMode:          "scenario_only",
			PredictedClose:            0,
			ScenarioPredictedClose:    103,
			ActualClose:               100,
			CloseAbsErrorPct:          3,
			ScenarioCloseAbsErrorPct:  3,
			CloseDirectionHit:         &directionHit,
			ScenarioCloseDirectionHit: &directionHit,
			IntervalHit:               &intervalHit,
			PointForecastPublishable:  false,
			ScenarioPointAvailable:    true,
			SuppressionReason:         "backtest_close_mape_above_2pct:2.60",
			BacktestCloseMAPE:         2.6,
			BacktestDirectionAccuracy: 43,
		},
	}
	from := mustForecastDate(t, "2026-06-01")
	to := mustForecastDate(t, "2026-06-30")

	report := buildForecastErrorAuditReport("ASELS", from, to, events, 1)
	if report.Summary.ErrorEvents != 0 || report.Summary.ScenarioErrorEvents != 1 {
		t.Fatalf("scenario drift must not be counted as production error: %+v", report.Summary)
	}
	if report.Summary.DirectionMisses != 0 || report.Summary.ScenarioDirectionMisses != 1 {
		t.Fatalf("scenario direction miss must stay separate: %+v", report.Summary)
	}
	if len(report.Events) != 0 || len(report.ScenarioEvents) != 1 {
		t.Fatalf("scenario events must be separated from production events: %+v", report)
	}
	got := report.ScenarioEvents[0]
	if got.EventType != "scenario_drift" || got.ForecastPublished || got.PrimaryCause != "suppressed_scenario_residual_error" {
		t.Fatalf("unexpected scenario attribution: %+v", got)
	}
}

func TestBuildForecastErrorAuditReportKeepsBandOnlyOutOfErrorEvents(t *testing.T) {
	intervalHit := true
	events := []forecastVerificationEvent{
		{
			EntryID:                        "entry-1",
			Symbol:                         "ASELS",
			AsOfDate:                       "2026-06-01",
			TargetDate:                     "2026-06-02",
			Status:                         "verified_band_only",
			VerificationMode:               "band_only",
			ActualClose:                    104,
			IntervalLow:                    95,
			IntervalHigh:                   105,
			IntervalHit:                    &intervalHit,
			PointForecastPublishable:       false,
			ScenarioPointAvailable:         false,
			ScenarioPointSuppressionReason: "scenario_point_close_mape_above_2pct:2.60",
		},
	}
	from := mustForecastDate(t, "2026-06-01")
	to := mustForecastDate(t, "2026-06-30")

	report := buildForecastErrorAuditReport("ASELS", from, to, events, 1)
	if report.Summary.BandOnlyEvents != 1 || report.Summary.ErrorEvents != 0 || report.Summary.ScenarioErrorEvents != 0 {
		t.Fatalf("band-only interval hit must not create point error events: %+v", report.Summary)
	}
	if len(report.Events) != 0 || len(report.ScenarioEvents) != 0 {
		t.Fatalf("band-only interval hit should not create attribution events: %+v", report)
	}
}

func TestBuildForecastActualVsAIReportSeparatesActualPredictionAndBand(t *testing.T) {
	intervalHit := true
	intervalMiss := false
	publishedDirectionHit := true
	scenarioDirectionHit := false
	events := []forecastVerificationEvent{
		{
			EntryID:                   "entry-1",
			Symbol:                    "ASELS",
			AsOfDate:                  "2026-06-01",
			TargetDate:                "2026-06-02",
			Status:                    "verified",
			VerificationMode:          "published_point",
			PredictedClose:            103,
			ActualClose:               104,
			LastClose:                 100,
			IntervalLow:               101,
			IntervalHigh:              105,
			DecisionIntervalLow:       102,
			DecisionIntervalHigh:      105,
			DecisionIntervalHit:       &intervalHit,
			DecisionIntervalWidthPct:  3,
			DecisionIntervalStatus:    "active",
			DecisionIntervalReason:    "conformal_q75_close_error_pct:1.50",
			CloseAbsErrorPct:          0.96,
			CloseDirectionHit:         &publishedDirectionHit,
			IntervalHit:               &intervalHit,
			PointForecastPublishable:  true,
			OfficialCloseSource:       "bist_thb_official_bulletin",
			BacktestCloseMAPE:         1.1,
			BacktestDirectionAccuracy: 60,
		},
		{
			EntryID:                   "entry-2",
			Symbol:                    "ASELS",
			AsOfDate:                  "2026-06-02",
			TargetDate:                "2026-06-03",
			Status:                    "verified_scenario_only",
			VerificationMode:          "scenario_only",
			ScenarioPredictedClose:    102,
			ActualClose:               100,
			LastClose:                 104,
			IntervalLow:               99,
			IntervalHigh:              105,
			DecisionIntervalLow:       98,
			DecisionIntervalHigh:      103,
			DecisionIntervalHit:       &intervalHit,
			DecisionIntervalWidthPct:  4,
			DecisionIntervalStatus:    "candidate_validation_failed",
			DecisionIntervalReason:    "conformal_q80_close_error_pct:2.50",
			ScenarioCloseAbsErrorPct:  2,
			ScenarioCloseDirectionHit: &scenarioDirectionHit,
			IntervalHit:               &intervalHit,
			PointForecastPublishable:  false,
			ScenarioPointAvailable:    true,
			SuppressionReason:         "backtest_close_mape_above_2pct:2.60",
		},
		{
			EntryID:                        "entry-3",
			Symbol:                         "ASELS",
			AsOfDate:                       "2026-06-03",
			TargetDate:                     "2026-06-04",
			Status:                         "verified_band_only",
			VerificationMode:               "band_only",
			ActualClose:                    110,
			LastClose:                      100,
			IntervalLow:                    95,
			IntervalHigh:                   105,
			DecisionIntervalLow:            98,
			DecisionIntervalHigh:           106,
			DecisionIntervalHit:            &intervalMiss,
			DecisionIntervalWidthPct:       8,
			DecisionIntervalStatus:         "candidate_validation_failed",
			DecisionIntervalReason:         "conformal_q80_close_error_pct:4.00",
			IntervalHit:                    &intervalMiss,
			PointForecastPublishable:       false,
			ScenarioPointAvailable:         false,
			ScenarioPointSuppressionReason: "scenario_point_close_mape_above_2pct:2.60",
		},
	}
	from := mustForecastDate(t, "2026-06-01")
	to := mustForecastDate(t, "2026-06-30")

	report := buildForecastActualVsAIReport("ASELS", from, to, "verification.jsonl", events)
	if report.Summary.VerifiedEvents != 3 || report.Summary.PublishedPointEvents != 1 || report.Summary.ScenarioOnlyEvents != 1 || report.Summary.BandOnlyEvents != 1 {
		t.Fatalf("summary counters mixed: %+v", report.Summary)
	}
	if report.Summary.IntervalHits != 2 || report.Summary.IntervalMisses != 1 || report.Summary.RowsWithoutPointClosePrediction != 1 {
		t.Fatalf("interval/band summary mismatch: %+v", report.Summary)
	}
	if report.Summary.DecisionIntervalHits != 2 || report.Summary.DecisionIntervalMisses != 1 || report.Summary.AverageDecisionIntervalWidthPct != 5 {
		t.Fatalf("decision interval summary mismatch: %+v", report.Summary)
	}
	if len(report.Rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(report.Rows))
	}
	if report.Rows[0].ActualClose == nil || *report.Rows[0].ActualClose != 104 || report.Rows[0].AIReportedClose == nil || *report.Rows[0].AIReportedClose != 103 {
		t.Fatalf("published row missing actual/ai close: %+v", report.Rows[0])
	}
	if report.Rows[1].PublishedPredictedClose != nil || report.Rows[1].ScenarioPredictedClose == nil || *report.Rows[1].ScenarioPredictedClose != 102 {
		t.Fatalf("scenario-only row mixed with published prediction: %+v", report.Rows[1])
	}
	if report.Rows[2].AIReportedClose != nil || report.Rows[2].PublishedCloseAbsErrorPct != nil || report.Rows[2].ScenarioOnlyCloseAbsErrorPct != nil {
		t.Fatalf("band-only row must not expose point forecast metrics: %+v", report.Rows[2])
	}
	md := forecastActualVsAIMarkdown(report)
	for _, want := range []string{"2026-06-02", "104.00", "103.00", "scenario_only", "band_only", "Decision band", "102.00-105.00", "scenario_point_close_mape_above_2pct:2.60"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown report missing %q:\n%s", want, md)
		}
	}
}

func testForecastDailyRecords(n int) []datasource.DailyBulletinRecord {
	out := make([]datasource.DailyBulletinRecord, 0, n)
	price := 100.0
	for i := 0; i < n; i++ {
		date := time.Date(2026, 5, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		price += 0.5
		out = append(out, datasource.DailyBulletinRecord{
			Symbol:      "ASELS",
			TradingDate: date,
			Open:        price - 0.2,
			High:        price + 0.4,
			Low:         price - 0.5,
			Close:       price,
			Volume:      1_000_000,
			SourcePath:  date + ".csv",
		})
	}
	return out
}

func testForecast(lastClose float64, forecastFor string) analysis.NextSessionForecast {
	return analysis.NextSessionForecast{
		Computed:                    true,
		ForecastFor:                 forecastFor,
		LastClose:                   lastClose,
		PredictedOpen:               lastClose + 0.5,
		PredictedClose:              lastClose + 1,
		RawPredictedOpen:            lastClose + 0.5,
		RawPredictedClose:           lastClose + 1,
		ExpectedLow:                 lastClose - 1,
		ExpectedHigh:                lastClose + 2,
		Confidence:                  70,
		ConfidenceLabel:             "medium",
		PredictedCloseDirection:     "yükseliş",
		PointForecastPublishable:    true,
		PointForecastStatus:         "publishable",
		BacktestSamples:             30,
		BacktestCloseMAEPct:         1.2,
		BacktestDirectionHitRatePct: 60,
		Model:                       "test_model",
		DecisionForecast: analysis.NextSessionDecisionForecast{
			CloseRangeLow:             lastClose - 1,
			CloseRangeHigh:            lastClose + 2,
			TradeSignalAllowed:        true,
			Confidence:                "medium",
			ExpectedIntradayDirection: "up",
			VolatilityRegime:          "normal",
		},
		BacktestMetrics: analysis.NextSessionBacktestMetrics{
			Samples:           20,
			CloseMAPE:         1.2,
			DirectionAccuracy: 60,
		},
	}
}

func mustForecastDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse date %s: %v", value, err)
	}
	return parsed
}
