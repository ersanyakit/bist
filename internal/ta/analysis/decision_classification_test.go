package analysis

import (
	"strings"
	"testing"

	"hissebot/internal/domain/marketdata"
	"hissebot/internal/kapingest"
	"hissebot/internal/services/pricequality"
	"hissebot/internal/ta/investorqa"
	"hissebot/internal/ta/macro"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/value"
)

func TestDecisionClassificationCannotPassLargeInvestorWhenPortfolioFails(t *testing.T) {
	result := centralClassificationFixture()
	result.InvestorQA.InstitutionalViews.Portfolio.Status = "fail"
	classification := ClassifyDecision(result)
	if classification.Classes.LargeInvestor.Qualified {
		t.Fatalf("large investor class passed while portfolio failed: %+v", classification.Classes.LargeInvestor)
	}
	if classification.Classes.LargeInvestor.Decision != "REDDET" {
		t.Fatalf("large investor decision = %s, want REDDET", classification.Classes.LargeInvestor.Decision)
	}
}

func TestDecisionClassificationCannotPassRetailWithoutSignalAndPlan(t *testing.T) {
	result := centralClassificationFixture()
	daily := result.Timeframes["1D"]
	daily.Professional.Technical.SignalGate.Actionable = false
	daily.TradePlan.Rejected = true
	result.Timeframes["1D"] = daily
	classification := ClassifyDecision(result)
	if classification.Classes.RetailDirect.Qualified {
		t.Fatalf("retail class passed without signal/plan: %+v", classification.Classes.RetailDirect)
	}
	for _, gate := range []string{"technical_signal", "active_trade_plan"} {
		if !containsExactString(classification.Classes.RetailDirect.FailedGates, gate) {
			t.Fatalf("missing failed gate %s: %+v", gate, classification.Classes.RetailDirect)
		}
	}
	if strings.Contains(classification.Classes.RetailDirect.Summary, "active_trade_plan") ||
		strings.Contains(classification.Classes.RetailDirect.Summary, "technical_signal") {
		t.Fatalf("summary leaked internal gate keys: %s", classification.Classes.RetailDirect.Summary)
	}
	if !strings.Contains(classification.Classes.RetailDirect.Summary, "Aktif işlem planı") ||
		!strings.Contains(classification.Classes.RetailDirect.Summary, "Günlük teknik sinyal") {
		t.Fatalf("summary should explain gates in Turkish: %s", classification.Classes.RetailDirect.Summary)
	}
	if !containsExactString(classification.Classes.RetailDirect.FailedGateLabels, "Aktif işlem planı") {
		t.Fatalf("failed gate labels missing readable label: %+v", classification.Classes.RetailDirect)
	}
}

func TestDecisionClassificationBlocksValuationAboveFiftyPercentDivergence(t *testing.T) {
	result := centralClassificationFixture()
	result.Professional.Valuation.FairValue.Base = 20
	result.Professional.ValueInvesting.IntrinsicValue.Base = 10
	classification := ClassifyDecision(result)
	if classification.ValuationConsistency.Publishable {
		t.Fatalf("valuation should not be publishable: %+v", classification.ValuationConsistency)
	}
	if classification.EffectiveModelRisk >= 50 {
		t.Fatalf("model risk must be capped below 50, got %.1f", classification.EffectiveModelRisk)
	}
	result.DecisionClassification = classification
	result = ApplyDecisionClassification(result)
	if result.Professional.EvidencePolicy.ValuationTargetsAllowed || result.Professional.EvidencePolicy.RecommendationAllowed {
		t.Fatalf("valuation publication flags remained open: %+v", result.Professional.EvidencePolicy)
	}
}

func TestDecisionClassificationBlocksRetailWhenNextSessionForecastIsNotDecisionGrade(t *testing.T) {
	result := centralClassificationFixture()
	result.NextSessionForecast = NextSessionForecast{
		Computed:                    true,
		ForecastFor:                 "2026-06-22",
		LastClose:                   402.50,
		PredictedOpen:               404,
		PredictedClose:              405,
		Status:                      "model_validation_failed",
		Quality:                     "not_decision_grade",
		Confidence:                  35,
		TechnicalDecisionStatus:     "failed",
		BacktestSamples:             60,
		BacktestDirectionHitRatePct: 43.33,
		BacktestCloseMAEPct:         2.73,
		Model:                       "atr_gap_intraday_ewma_v1",
	}
	classification := ClassifyDecision(result)
	gate, ok := classificationGateForDecisionTest(classification, "next_session_forecast_model")
	if !ok {
		t.Fatalf("next-session forecast gate missing: %+v", classification.Gates)
	}
	if gate.Passed {
		t.Fatalf("next-session forecast gate must fail for not-decision-grade model: %+v", gate)
	}
	if !containsExactString(classification.Classes.RetailDirect.FailedGates, "next_session_forecast_model") {
		t.Fatalf("retail class did not inherit forecast model failure: %+v", classification.Classes.RetailDirect)
	}
	if !containsExactString(classification.Classes.TradingEdge.FailedGates, "next_session_forecast_model") {
		t.Fatalf("trading class did not inherit forecast model failure: %+v", classification.Classes.TradingEdge)
	}
	if containsExactString(classification.Classes.LargeInvestor.FailedGates, "next_session_forecast_model") {
		t.Fatalf("large-investor long-term class must not be blocked directly by short-term forecast gate: %+v", classification.Classes.LargeInvestor)
	}
}

func TestDecisionClassificationDoesNotPassForecastModelOnlyBecauseOfficialActualExists(t *testing.T) {
	result := centralClassificationFixture()
	result.NextSessionForecast = NextSessionForecast{
		Computed:                    true,
		ForecastFor:                 "2026-06-19",
		LastClose:                   410.75,
		PredictedOpen:               407.75,
		PredictedClose:              402.75,
		ActualAvailable:             true,
		ActualOpen:                  408.75,
		ActualClose:                 402.50,
		ValidationStatus:            "actual_session_observed",
		ActualSource:                "bist_thb_official_bulletin",
		ActualSourcePath:            "data/bist/unprocessed/bulten_verileri/2026/06/20260619_s1/extracted/thb202606191.csv",
		Status:                      "model_validation_failed",
		Quality:                     "not_decision_grade",
		Confidence:                  35,
		TechnicalDecisionStatus:     "failed",
		BacktestSamples:             60,
		BacktestDirectionHitRatePct: 43.33,
		BacktestCloseMAEPct:         2.73,
		Model:                       "atr_gap_intraday_ewma_v1",
	}
	classification := ClassifyDecision(result)
	gate, ok := classificationGateForDecisionTest(classification, "next_session_forecast_model")
	if !ok {
		t.Fatalf("next-session forecast gate missing: %+v", classification.Gates)
	}
	if gate.Passed || gate.Score >= 50 {
		t.Fatalf("official actual must not pass a failed forecast model gate: %+v", gate)
	}
	if !containsExactString(classification.Classes.RetailDirect.FailedGates, "next_session_forecast_model") {
		t.Fatalf("retail class did not inherit forecast model failure: %+v", classification.Classes.RetailDirect)
	}
}

func TestDecisionClassificationPassesSharedGatesTogether(t *testing.T) {
	classification := ClassifyDecision(centralClassificationFixture())
	if !classification.Classes.LargeInvestor.Qualified {
		t.Fatalf("large investor should pass: %+v", classification.Classes.LargeInvestor)
	}
	if !classification.Classes.RetailDirect.Qualified {
		t.Fatalf("retail direct should pass: %+v", classification.Classes.RetailDirect)
	}
	if len(classification.ConsistencyViolations) > 0 {
		t.Fatalf("central classification is internally inconsistent: %+v", classification.ConsistencyViolations)
	}
}

func TestDecisionClassificationFailsFinancialIntegrityWhenKAPPDFCurrentPeriodMetricsAreMissing(t *testing.T) {
	result := centralClassificationFixture()
	result.Professional.RawKAPData = &professional.KAPRawDataBundle{
		Computed: true,
		FinancialFacts: []kapingest.ExtractedFinancialFact{
			rawKAPFinancialFactForDecisionTest("balance", "Toplam Varlıklar", "total_assets", 22_000_000, "2019-09-30"),
			rawKAPFinancialFactForDecisionTest("balance", "Dönen Varlıklar", "current_assets", 120_000_000, "2026-03-31"),
			rawKAPFinancialFactForDecisionTest("balance", "Özkaynaklar", "equity", 280_000_000, "2026-03-31"),
			rawKAPFinancialFactForDecisionTest("balance", "Kısa Vadeli Yükümlülükler", "current_liabilities", 130_000_000, "2026-03-31"),
			rawKAPFinancialFactForDecisionTest("income", "Hasılat", "revenue", 34_000_000, "2026-03-31"),
			rawKAPFinancialFactForDecisionTest("income", "Net Dönem Karı", "net_income", 5_500_000, "2026-03-31"),
		},
	}

	classification := ClassifyDecision(result)
	gate, ok := classificationGateForDecisionTest(classification, "financial_integrity")
	if !ok {
		t.Fatalf("financial integrity gate missing: %+v", classification.Gates)
	}
	if gate.Passed {
		t.Fatalf("financial integrity should fail with stale total assets: %+v", gate)
	}
	if !containsExactString(classification.Classes.LargeInvestor.FailedGates, "financial_integrity") {
		t.Fatalf("large investor class did not inherit financial gate failure: %+v", classification.Classes.LargeInvestor)
	}
	evidence := strings.Join(gate.Evidence, "\n")
	if !strings.Contains(evidence, "kap_pdf_current_period_missing_total_assets_target_2026-03-31_latest_2019-09-30") {
		t.Fatalf("missing raw KAP period mismatch evidence: %s", evidence)
	}
}

func TestDecisionClassificationFailsFinancialIntegrityWhenGovernanceIsNotDecisionSafe(t *testing.T) {
	result := centralClassificationFixture()
	result.Professional.DataGovernance.BacktestSafe = false
	result.Professional.DataGovernance.LatestAvailableAt = nil
	result.Professional.DataGovernance.LatestPublishDate = nil

	classification := ClassifyDecision(result)
	gate, ok := classificationGateForDecisionTest(classification, "financial_integrity")
	if !ok {
		t.Fatalf("financial integrity gate missing: %+v", classification.Gates)
	}
	if gate.Passed {
		t.Fatalf("financial integrity should fail when financial statements are not decision-safe: %+v", gate)
	}
	if !containsExactString(classification.Classes.RetailDirect.FailedGates, "financial_integrity") {
		t.Fatalf("retail class did not inherit financial integrity failure: %+v", classification.Classes.RetailDirect)
	}
}

func classificationGateForDecisionTest(classification DecisionClassification, key string) (ClassificationGate, bool) {
	for _, gate := range classification.Gates {
		if gate.Key == key {
			return gate, true
		}
	}
	return ClassificationGate{}, false
}

func rawKAPFinancialFactForDecisionTest(statement, original, normalized string, value float64, period string) kapingest.ExtractedFinancialFact {
	return kapingest.ExtractedFinancialFact{
		StatementType:      statement,
		LineItemOriginal:   original,
		LineItemNormalized: normalized,
		Value:              value,
		Period:             &period,
		Confidence:         0.95,
	}
}

func centralClassificationFixture() SymbolAnalysis {
	metrics := []professional.SectorFinancialMetric{
		{Name: "gross_margin"}, {Name: "ebit_margin"}, {Name: "fcf_conversion"},
		{Name: "cash_ratio"}, {Name: "net_debt_to_equity"},
	}
	return SymbolAnalysis{
		Symbol:    "TECH",
		AssetType: ohlcv.AssetTypeEquity,
		PriceQuality: &pricequality.SymbolReport{
			Status: pricequality.StatusReadyForVerifiedClose, ReadyForDecision: true, ReadyForVerifiedClose: true,
		},
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Score: 82,
				TradePlan: ohlcv.TradePlan{
					Direction: "long", EntryMin: 9.8, EntryMax: 10,
					StopLoss: 9.2, TakeProfit1: 11.5, RiskRewardRatio: 2,
				},
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{SignalGate: professional.TechnicalSignalGate{Status: "pass", Actionable: true, Score: 82}},
					Backtest: professional.BacktestResult{
						BacktestSafe: true, Trades: 50, OutOfSampleTrades: 15,
						Expectancy: .02, OutOfSampleReturn: .03,
					},
				},
			},
		},
		Professional: professional.Report{
			Coverage:       professional.CoverageReport{Score: 90},
			DataQuality:    90,
			EvidencePolicy: professional.EvidencePolicyReport{Status: "pass", ValuationTargetsAllowed: true, RecommendationAllowed: true},
			Company: professional.CompanyProfile{
				Sector: "TEKNOLOJİ", Industry: "BİLİŞİM", ClassificationConfidence: .97,
			},
			SectorFinancials: professional.SectorFinancialAnalysis{Profile: "technology", Metrics: metrics},
			DataGovernance: professional.FinancialDataGovernance{
				FinanciallyConsistent: true, ReconciliationFailureCount: 0, LatestPeriod: "2026-Q1", BacktestSafe: true,
			},
			ValueInvesting: value.Report{
				Computed: true, Confidence: 85,
				SectorModel:    value.SectorModelReport{Model: "technology_growth_quality"},
				IntrinsicValue: value.IntrinsicValueReport{Computed: true, Base: 10},
			},
			Valuation: professional.ValuationAnalysis{FairValue: professional.FairValueRange{Base: 12}},
			Market: professional.MarketContext{
				BenchmarkAvailable: true,
				LiveSnapshot:       &marketdata.LiveMarketSnapshot{},
				GDP:                macro.GDPContext{Computed: true, Score: 70},
			},
			TCMBContext: professional.TCMBContextReport{Computed: true},
			TCMBEVDSContext: professional.TCMBEVDSContextReport{
				AnalysisReady: true,
				ForecastImpact: professional.TCMBMacroForecastImpact{
					Computed: true, Confidence: 80, Direction: "neutral", DecisionUse: "decision_input",
				},
			},
		},
		InvestorQA: investorqa.Report{
			Computed: true, Score: 80, Confidence: 80,
			ModelRisk: investorqa.ModelRiskReport{Score: 80},
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed:       true,
				ValueInvesting: investorqa.PersonaView{Status: "pass", Score: 82},
				Portfolio: investorqa.PersonaView{
					Status: "pass", ReportQualityStatus: "pass", Score: 84, Confidence: 82,
				},
				TradingEdge: investorqa.PersonaView{
					Status: "pass", TransactionUseStatus: "pass", Score: 80,
				},
			},
		},
	}
}
