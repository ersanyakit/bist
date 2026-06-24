package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/datasource"
)

func TestForecastAuditKurusExactAccuracyAndError(t *testing.T) {
	actual := forecastAuditPriceKurus(402.50)
	predicted := forecastAuditPriceKurus(402.75)
	errKurus := actual - predicted

	if actual != 40250 || predicted != 40275 {
		t.Fatalf("kurus conversion actual=%d predicted=%d", actual, predicted)
	}
	if errKurus != -25 {
		t.Fatalf("errKurus=%d, want -25", errKurus)
	}
	if got := forecastAuditKurusTL(errKurus); got != -0.25 {
		t.Fatalf("forecastAuditKurusTL=%v, want -0.25", got)
	}
	if got := forecastAuditAbsErrorPctFromKurus(errKurus, actual); got != 0.06 {
		t.Fatalf("abs error pct=%v, want 0.06", got)
	}
	if got := forecastAuditExactAccuracyPct(errKurus); got != 0 {
		t.Fatalf("exact accuracy=%v, want 0", got)
	}
	if got := forecastAuditExactWrongPct(errKurus); got != 100 {
		t.Fatalf("exact wrong=%v, want 100", got)
	}
	if got := forecastAuditClosenessScorePct(0.06); got != 99.94 {
		t.Fatalf("closeness score=%v, want 99.94", got)
	}
}

func TestForecastAuditKurusExactHit(t *testing.T) {
	errKurus := forecastAuditPriceKurus(395.00) - forecastAuditPriceKurus(395.00)
	if got := forecastAuditExactAccuracyPct(errKurus); got != 100 {
		t.Fatalf("exact accuracy=%v, want 100", got)
	}
	if got := forecastAuditExactWrongPct(errKurus); got != 0 {
		t.Fatalf("exact wrong=%v, want 0", got)
	}
	if got := forecastAuditAbsErrorPctFromKurus(errKurus, forecastAuditPriceKurus(395.00)); got != 0 {
		t.Fatalf("abs error pct=%v, want 0", got)
	}
}

func TestForecastAuditDirectionUsesForecastTolerance(t *testing.T) {
	if got := forecastAuditDirection(402.60, 402.50); got != "yatay" {
		t.Fatalf("sub-tolerance direction = %s, want yatay", got)
	}
	if got := forecastAuditDirection(402.75, 402.50); got != "yukari" {
		t.Fatalf("above-tolerance direction = %s, want yukari", got)
	}
	if got := forecastAuditDirection(402.25, 402.50); got != "asagi" {
		t.Fatalf("below-tolerance direction = %s, want asagi", got)
	}
}

func TestForecastAuditRealBISTOfficialActualIsExactWhenObserved(t *testing.T) {
	root := firstExistingPath("data/bist/unprocessed", "../../data/bist/unprocessed")
	if root == "" {
		t.Skip("local BIST bulletin data not available")
	}
	provider := datasource.NewBISTBulletinProvider(root)
	records, err := provider.FetchDailyBulletinRecordsRange(context.Background(), "ASELS", "", "2026-06-19", 420)
	if err != nil {
		if errors.Is(err, datasource.ErrSymbolNotFound) {
			t.Skipf("ASELS BIST bulletin fixture not available: %v", err)
		}
		t.Fatalf("fetch real BIST records: %v", err)
	}
	report, err := buildForecastAuditReport("ASELS", time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), "2026-06-19", records)
	if err != nil {
		t.Fatalf("build forecast audit report: %v", err)
	}
	if !report.OfficialResult.Available || !report.OfficialResult.Authoritative {
		t.Fatalf("official actual must be available and authoritative: %+v", report.OfficialResult)
	}
	if report.ReportDecisionStatus != "official_actual_verified" {
		t.Fatalf("report decision status=%q, want official_actual_verified", report.ReportDecisionStatus)
	}
	if report.ModelDecisionStatus == "" {
		t.Fatal("expected model decision status to be reported separately")
	}
	if report.OfficialResult.Open != 408.75 || report.OfficialResult.Close != 402.50 {
		t.Fatalf("official actual mismatch: %+v", report.OfficialResult)
	}
	if !report.DataScope.OnlyUsesDataThroughAsOf || !report.DataScope.ActualUsedOnlyForValidation {
		t.Fatalf("point-in-time audit must keep actual only for validation: %+v", report.DataScope)
	}
	if report.Forecast.ActualOpen != report.OfficialResult.Open || report.Forecast.ActualClose != report.OfficialResult.Close {
		t.Fatalf("attached actual must match official result: forecast=%+v official=%+v", report.Forecast, report.OfficialResult)
	}
}

func TestForecastAuditRangeSeparatesOfficialActualFromModelBacktest(t *testing.T) {
	root := firstExistingPath("data/bist/unprocessed", "../../data/bist/unprocessed")
	if root == "" {
		t.Skip("local BIST bulletin data not available")
	}
	provider := datasource.NewBISTBulletinProvider(root)
	records, err := provider.FetchDailyBulletinRecordsRange(context.Background(), "ASELS", "", "2026-06-19", 420)
	if err != nil {
		if errors.Is(err, datasource.ErrSymbolNotFound) {
			t.Skipf("ASELS BIST bulletin fixture not available: %v", err)
		}
		t.Fatalf("fetch real BIST records: %v", err)
	}
	report, err := buildForecastAuditRangeReport(
		"ASELS",
		time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
		records,
	)
	if err != nil {
		t.Fatalf("build forecast audit range report: %v", err)
	}
	if report.Summary.OfficialObservedRows == 0 || report.Summary.OfficialResultHitPct != 100 {
		t.Fatalf("official observed summary must be exact and separate: %+v", report.Summary)
	}
	if report.Summary.OpenExactHitPct == 100 || report.Summary.CloseExactHitPct == 100 {
		t.Fatalf("model exact metrics must not be treated as official actual extraction: %+v", report.Summary)
	}
	last := report.Rows[len(report.Rows)-1]
	if last.ActualDate != "2026-06-19" || last.ActualOpen != 408.75 || last.ActualClose != 402.50 {
		t.Fatalf("last official actual row mismatch: %+v", last)
	}
	if last.ReportDecisionStatus != "official_actual_verified" {
		t.Fatalf("official row decision status=%q", last.ReportDecisionStatus)
	}
	if last.ModelDecisionStatus == "" || last.ModelDecisionStatus == "model_status_unknown" {
		t.Fatalf("model decision status must be reported separately: %+v", last)
	}
	if report.Summary.ModelSuppressedRows == 0 {
		t.Fatalf("failed/non-decision-grade model forecasts must be suppressed: %+v", report.Summary)
	}
	if last.ModelForecastPublishable || last.ModelForecastPublishStatus != "not_published" {
		t.Fatalf("last non-decision-grade model forecast must not be published: %+v", last)
	}
	if last.PublishedPredictedOpen != nil || last.PublishedPredictedClose != nil {
		t.Fatalf("suppressed model forecast must not expose published prices: %+v", last)
	}
}

func TestForecastAuditRangeSummaryPrioritizesBandQualityOverExactHit(t *testing.T) {
	rows := []forecastAuditRangeRow{
		{
			ActualDate:                "2026-06-18",
			OfficialResultAvailable:   true,
			PredictedOpen:             100.40,
			PredictedClose:            100.75,
			OpenErrorPct:              0.40,
			CloseErrorPct:             0.75,
			OpenClosenessScorePct:     99.60,
			CloseClosenessScorePct:    99.25,
			OpenDirectionHit:          true,
			CloseDirectionHit:         true,
			ModelForecastPublishable:  true,
			TradeSignalAllowed:        true,
			ForecastContext:           "technical_ohlcv_bist_only",
			VolatilityRegime:          "normal",
			ExpectedIntradayDirection: "up",
		},
		{
			ActualDate:                "2026-06-19",
			OfficialResultAvailable:   true,
			PredictedOpen:             101.80,
			PredictedClose:            101.50,
			OpenErrorPct:              1.80,
			CloseErrorPct:             1.50,
			OpenClosenessScorePct:     98.20,
			CloseClosenessScorePct:    98.50,
			OpenDirectionHit:          false,
			CloseDirectionHit:         true,
			ModelForecastPublishable:  false,
			TradeSignalAllowed:        false,
			ForecastContext:           "technical_ohlcv_bist_only",
			VolatilityRegime:          "high",
			ExpectedIntradayDirection: "up",
		},
	}
	summary := buildForecastAuditRangeSummary(rows)
	if summary.CloseExactHitPct != 0 {
		t.Fatalf("exact hit should stay secondary and zero for non-exact rows: %+v", summary)
	}
	if summary.CloseWithin100Pct != 50 || summary.CloseWithin200Pct != 100 {
		t.Fatalf("close band metrics wrong: %+v", summary)
	}
	if summary.CloseDirectionHitPct != 100 || summary.TradeAllowedPct != 50 {
		t.Fatalf("direction/trade metrics wrong: %+v", summary)
	}
	if summary.ModelPublishedPct != 50 || summary.ModelSuppressedPct != 50 {
		t.Fatalf("publish/suppression metrics wrong: %+v", summary)
	}
	if summary.ForecastQualityGrade != "usable_with_gate" {
		t.Fatalf("quality grade=%q, want usable_with_gate: %+v", summary.ForecastQualityGrade, summary)
	}
	if len(summary.RegimePerformance) != 2 {
		t.Fatalf("regime buckets missing: %+v", summary.RegimePerformance)
	}

	report := forecastAuditRangeReport{Symbol: "ASELS", Summary: summary, Rows: rows}
	md := forecastAuditRangeMarkdown(report)
	for _, want := range []string{"Birincil kalite", "İkincil exact denetim", "Rejim Performansı", "Yayınlanmadı/no-trade", "Yayın A", "Gün Gün Hata Nedenleri"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q\n%s", want, md)
		}
	}
	if strings.Contains(md, "Tahmin Açılış") || strings.Contains(md, "Tahmin Kapanış") || strings.Contains(md, "İç Model A/K") {
		t.Fatalf("markdown must not label suppressed raw model values as published forecasts\n%s", md)
	}
	html := forecastAuditRangeHTML(report)
	for _, want := range []string{"Birincil kalite", "Exact fiyat isabeti ana başarı metriği değildir", "Rejim Performansı", "Yayın kapısı", "Gün Gün Hata Nedenleri"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, "Tahmin Açılış") || strings.Contains(html, "Tahmin Kapanış") {
		t.Fatalf("html must not label suppressed raw model values as published forecasts\n%s", html)
	}
}

func TestForecastAuditOutcomeDistinguishesDirectionOnlyAndNoTrade(t *testing.T) {
	directionOnly := forecastAuditRangeRow{CloseErrorPct: 1.75, CloseDirectionHit: true, ModelForecastPublishable: true, TradeSignalAllowed: true}
	if got := forecastAuditOverallResultText(directionOnly); !strings.Contains(got, "Yön uyumlu") || !strings.Contains(got, "bandı dışında") {
		t.Fatalf("direction-only outcome=%q", got)
	}
	noTrade := forecastAuditRangeRow{CloseErrorPct: 0.75, CloseDirectionHit: true, ModelForecastPublishable: false, TradeSignalAllowed: false}
	if got := forecastAuditOverallResultText(noTrade); !strings.Contains(got, "Yayınlanmadı/no-trade") || !strings.Contains(got, "band içinde") {
		t.Fatalf("no-trade outcome=%q", got)
	}
}

func TestForecastAuditRangeSuppressesUnpublishablePredictedFields(t *testing.T) {
	records := make([]datasource.DailyBulletinRecord, 0, 32)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; len(records) < 32; i++ {
		day := start.AddDate(0, 0, i)
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		closePrice := 100 + float64(len(records))
		records = append(records, datasource.DailyBulletinRecord{
			TradingDate: day.Format("2006-01-02"),
			Open:        closePrice - 0.25,
			High:        closePrice + 1,
			Low:         closePrice - 1,
			Close:       closePrice,
			Volume:      1000000,
			SourcePath:  "fixture/" + day.Format("2006-01-02") + ".csv",
		})
	}
	from, err := time.Parse("2006-01-02", records[30].TradingDate)
	if err != nil {
		t.Fatalf("parse fixture date: %v", err)
	}
	builder := func(ctx context.Context, symbol string, asOf datasource.DailyBulletinRecord, forecastFor string, prefixRecords []datasource.DailyBulletinRecord) (forecastAuditRangeForecastResult, error) {
		return forecastAuditRangeForecastResult{
			Forecast: analysis.NextSessionForecast{
				Computed:       true,
				ForecastFor:    forecastFor,
				LastClose:      asOf.Close,
				PredictedOpen:  asOf.Close + 5,
				PredictedClose: asOf.Close + 6,
				BacktestMetrics: analysis.NextSessionBacktestMetrics{
					Samples:            60,
					CloseMAPE:          2.75,
					DirectionAccuracy:  45,
					TradeSignalAllowed: false,
				},
				DecisionForecast: analysis.NextSessionDecisionForecast{
					Confidence:         "low",
					TradeSignalAllowed: false,
				},
				Model: "fixture_unpublishable",
			},
			Context: "test_fixture",
		}, nil
	}
	report, err := buildForecastAuditRangeReportWithBuilder(context.Background(), "ASELS", from, from, records, builder)
	if err != nil {
		t.Fatalf("build forecast audit range report: %v", err)
	}
	if report.Summary.PublishableReportStatus != "no_publishable_forecast" {
		t.Fatalf("publishable report status=%q", report.Summary.PublishableReportStatus)
	}
	row := report.Rows[0]
	if row.ModelForecastPublishable || row.PublishedPredictedOpen != nil || row.PublishedPredictedClose != nil {
		t.Fatalf("suppressed row exposed published forecast: %+v", row)
	}
	if row.PredictedOpen != 0 || row.PredictedClose != 0 || row.PredictedOpenDirection != "" || row.PredictedCloseDirection != "" {
		t.Fatalf("suppressed row must not expose decision predicted fields: %+v", row)
	}
	if row.ScenarioPredictedOpen <= 0 || row.ScenarioPredictedClose <= 0 ||
		row.ScenarioPredictedOpenDirection == "" || row.ScenarioPredictedCloseDirection == "" {
		t.Fatalf("suppressed scenario audit fields missing: %+v", row)
	}
	md := forecastAuditRangeMarkdown(report)
	if !strings.Contains(md, "Yayınlanmadı/no-trade") || strings.Contains(md, "İç Model A/K") {
		t.Fatalf("markdown should show no published forecast and hide scenario prices\n%s", md)
	}
	html := forecastAuditRangeHTML(report)
	if !strings.Contains(html, "Yayınlanmadı/no-trade") || strings.Contains(html, "İç Model A/K") {
		t.Fatalf("html should show no published forecast and hide scenario prices\n%s", html)
	}
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if stat, err := os.Stat(filepath.Clean(path)); err == nil && stat.IsDir() {
			return filepath.Clean(path)
		}
	}
	return ""
}
