package fintradebench

import (
	"testing"
	"time"

	"hissebot/internal/ta/indicators"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
)

func TestAnalyzeBuildsHybridSignalCoverage(t *testing.T) {
	candles := makeFinTradeBenchCandles(220)
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	report := Analyze(Input{
		Symbol:       "TEST",
		AsOf:         candles[len(candles)-1].Time,
		LastClose:    candles[len(candles)-1].Close,
		Candles:      candles,
		Indicators:   snapshot,
		Professional: makeBenchmarkProfessionalReport(),
	})

	if !report.Computed {
		t.Fatalf("expected computed report")
	}
	if report.HybridReadinessScore <= 70 {
		t.Fatalf("hybrid readiness too low: %+v", report)
	}
	if !hasAvailableSignal(report.TradingSignals, "ema_20") || !hasAvailableSignal(report.TradingSignals, "long_term_mean_reversion") {
		t.Fatalf("expected core trading signals: %+v", report.TradingSignals)
	}
	if !hasAvailableSignal(report.FundamentalSignals, "return_equity") || !hasAvailableSignal(report.FundamentalSignals, "debt_assets") {
		t.Fatalf("expected fundamental availability signals: %+v", report.FundamentalSignals)
	}
	if report.DocumentEvidence.Status != "passed" || report.DocumentEvidence.CoverageScore != 80 {
		t.Fatalf("expected passed document evidence: %+v", report.DocumentEvidence)
	}
	if !hasAvailableSignal(report.DocumentSignals, "kap_pdf_usable_coverage") || !hasAvailableSignal(report.DocumentSignals, "kap_pdf_decision_relevant_coverage") {
		t.Fatalf("expected document evidence signals: %+v", report.DocumentSignals)
	}
	if report.GoldenIndicatorCoverage.F1 <= 0.70 {
		t.Fatalf("expected useful golden indicator F1: %+v", report.GoldenIndicatorCoverage)
	}
	if report.CalculationValidation.Status != "passed" || report.CalculationValidation.Failed != 0 {
		t.Fatalf("expected calculation validation to pass: %+v", report.CalculationValidation)
	}
	if report.NumericalAudit.Status == "failed" {
		t.Fatalf("did not expect numerical audit failure: %+v", report.NumericalAudit)
	}
	if report.PointInTime.Status != "passed" {
		t.Fatalf("unexpected point-in-time status: %+v", report.PointInTime)
	}
}

func TestAnalyzeFlagsMismatchedCalculatedIndicator(t *testing.T) {
	candles := makeFinTradeBenchCandles(220)
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshot.EMA20 += 10

	report := Analyze(Input{
		Symbol:       "TEST",
		AsOf:         candles[len(candles)-1].Time,
		LastClose:    candles[len(candles)-1].Close,
		Candles:      candles,
		Indicators:   snapshot,
		Professional: makeBenchmarkProfessionalReport(),
	})

	if report.CalculationValidation.Status != "failed" {
		t.Fatalf("expected calculation validation failure: %+v", report.CalculationValidation)
	}
	if !hasFailedCalculationCheck(report.CalculationValidation.Checks, "ema_20") {
		t.Fatalf("expected ema_20 failure: %+v", report.CalculationValidation.Checks)
	}
	if report.NumericalAudit.Status != "failed" || !containsString(report.NumericalAudit.Contradictions, "calculation:ema_20") {
		t.Fatalf("expected numerical audit contradiction: %+v", report.NumericalAudit)
	}
}

func TestAnalyzeFlagsMarketDataAfterAsOf(t *testing.T) {
	candles := makeFinTradeBenchCandles(60)
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	report := Analyze(Input{
		Symbol:     "TEST",
		AsOf:       candles[len(candles)-2].Time,
		LastClose:  candles[len(candles)-1].Close,
		Candles:    candles,
		Indicators: snapshot,
	})
	if report.PointInTime.Status != "failed" {
		t.Fatalf("expected failed point-in-time status: %+v", report.PointInTime)
	}
}

func makeFinTradeBenchCandles(n int) []ohlcv.Candle {
	start := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	out := make([]ohlcv.Candle, 0, n)
	price := 100.0
	for i := 0; i < n; i++ {
		price *= 1.001
		open := price * 0.998
		closePrice := price
		out = append(out, ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   closePrice * 1.01,
			Low:    open * 0.99,
			Close:  closePrice,
			Volume: 1_000_000 + float64(i)*1_000,
		})
	}
	return out
}

func makeBenchmarkProfessionalReport() professional.Report {
	return professional.Report{
		Valuation: professional.ValuationAnalysis{
			LatestYear:        2026,
			LatestQuarter:     "Q1",
			MarketCap:         1_000,
			OperatingCashTTM:  80,
			SalesTTM:          900,
			NetIncomeTTM:      120,
			Equity:            500,
			TotalAssets:       1_200,
			TotalDebt:         180,
			DebtDataAvailable: true,
			NetDebt:           100,
			Ratios: map[string]float64{
				"PB":         2.0,
				"PE":         8.333333,
				"NetDebt_Eq": 0.20,
				"ROA":        0.10,
				"ROE":        0.24,
			},
		},
		KAPPDFIngest: professional.KAPPDFIngestSummary{
			Computed:                    true,
			Symbol:                      "TEST",
			TotalDocuments:              10,
			SourcePDFCount:              12,
			AnalysisUsableCount:         8,
			DecisionRelevantDocuments:   5,
			DecisionRelevantUsableCount: 4,
			ReviewRequiredCount:         1,
			RejectedCount:               1,
			AverageQuality:              0.82,
			Summary:                     "KAP PDF ingest kaniti bagli: 10 PDF, 8 analize uygun.",
			TypeCounts: []professional.KAPPDFTypeCount{
				{Type: "financial_report", Label: "Finansal rapor", Count: 5},
				{Type: "material_disclosure", Label: "Ozel durum aciklamasi", Count: 3},
			},
			ImportantDocuments: []professional.KAPPDFDocumentSummary{
				{
					FileName:       "TEST_2026_Q1.pdf",
					DocumentType:   "financial_report",
					DocumentLabel:  "Finansal rapor",
					TextLength:     12000,
					QualityScore:   0.90,
					ParseStatus:    "trusted",
					AnalysisUsable: true,
					ContentSnippet: "Gelir tablosu ve bilanco kalemleri.",
				},
			},
		},
	}
}

func hasAvailableSignal(signals []Signal, name string) bool {
	for _, signal := range signals {
		if signal.Name == name && signal.Available {
			return true
		}
	}
	return false
}

func hasFailedCalculationCheck(checks []CalculationCheck, name string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == "failed" {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
