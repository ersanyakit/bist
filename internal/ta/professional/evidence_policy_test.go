package professional

import (
	"testing"
	"time"

	"hissebot/internal/ta/value"
)

func TestStrictEvidencePolicySuppressesUnsupportedInvestmentOutputs(t *testing.T) {
	report := Report{
		Coverage: CoverageReport{
			Score:     45,
			Available: []string{"financial_statements"},
			Missing:   []string{"kap_pdf_ingest"},
		},
		Company: CompanyProfile{ClassificationConfidence: 0.45},
		Scenarios: []Scenario{
			{Name: "base", PriceTarget: 120, ReturnPct: 20},
		},
		DataGovernance: FinancialDataGovernance{
			DataMode:              "research",
			BacktestSafe:          false,
			FinanciallyConsistent: false,
		},
		ValueInvesting: value.Report{
			Computed:      true,
			Decision:      "UCUZ",
			DecisionLabel: "İçsel değere göre güvenlik marjı var",
			DataQuality:   55,
			Confidence:    60,
			IntrinsicValue: value.IntrinsicValueReport{
				Computed: true,
				Base:     100,
			},
			MarginOfSafety: value.MarginOfSafetyReport{
				Computed:    true,
				BasePct:     25,
				RequiredPct: 20,
			},
		},
		InvestmentResearch: InvestmentResearchReview{
			DecisionFramework: DecisionFramework{CurrentDecision: "AL"},
			InvestmentStory:   InvestmentStory{CoreThesis: "old thesis", Catalysts: []string{"generic catalyst"}},
			InstitutionalMemo: InstitutionalMemo{
				Recommendation:           "BUY",
				Decision:                 "AL",
				DirectBuyEligible:        true,
				InvestmentCommitteeReady: true,
				ReadinessScore:           90,
			},
		},
	}

	ApplyStrictEvidencePolicy(&report)

	if report.EvidencePolicy.Status != "blocked" {
		t.Fatalf("expected blocked policy, got %+v", report.EvidencePolicy)
	}
	if report.ValueInvesting.Computed {
		t.Fatalf("value investing output should be suppressed: %+v", report.ValueInvesting)
	}
	if len(report.Scenarios) != 0 {
		t.Fatalf("scenario price targets should be suppressed: %+v", report.Scenarios)
	}
	if report.InvestmentResearch.InstitutionalMemo.Recommendation != "INSUFFICIENT_DATA" {
		t.Fatalf("recommendation should be insufficient data: %+v", report.InvestmentResearch.InstitutionalMemo)
	}
	if report.InvestmentResearch.DecisionFramework.CurrentDecision != "INSUFFICIENT_DATA" {
		t.Fatalf("decision framework should be insufficient data: %+v", report.InvestmentResearch.DecisionFramework)
	}
}

func TestStrictEvidencePolicyKeepsOutputsWhenEvidencePasses(t *testing.T) {
	report := Report{
		Coverage: CoverageReport{
			Score:     95,
			Available: []string{"financial_statements", "kap_pdf_ingest"},
		},
		Company: CompanyProfile{ClassificationConfidence: 0.95},
		Peers:   PeerComparison{PeerCount: 3},
		KAPPDFIngest: KAPPDFIngestSummary{
			Computed:            true,
			TotalDocuments:      2,
			AnalysisUsableCount: 2,
			ReviewRequiredCount: 0,
			RejectedCount:       0,
		},
		Scenarios: []Scenario{
			{Name: "base", PriceTarget: 120, ReturnPct: 20},
		},
		DataGovernance: FinancialDataGovernance{
			DataMode:              "production",
			BacktestSafe:          true,
			FinanciallyConsistent: true,
			Source:                "test_financials",
			LineageEvents:         3,
		},
		ValueInvesting: value.Report{
			Computed:      true,
			Decision:      "MAKUL",
			DecisionLabel: "Fiyat içsel değere yakın",
			DataQuality:   90,
			Confidence:    85,
			IntrinsicValue: value.IntrinsicValueReport{
				Computed: true,
				Base:     100,
			},
			MarginOfSafety: value.MarginOfSafetyReport{
				Computed:    true,
				BasePct:     10,
				RequiredPct: 20,
			},
		},
		InvestmentResearch: InvestmentResearchReview{
			DecisionFramework: DecisionFramework{CurrentDecision: "BEKLE"},
			InstitutionalMemo: InstitutionalMemo{Recommendation: "WATCH", Decision: "BEKLE"},
		},
	}

	ApplyStrictEvidencePolicy(&report)

	if report.EvidencePolicy.Status != "pass" {
		t.Fatalf("expected pass policy, got %+v", report.EvidencePolicy)
	}
	if !report.ValueInvesting.Computed {
		t.Fatalf("value investing output should remain available")
	}
	if len(report.Scenarios) != 1 {
		t.Fatalf("scenario output should remain available: %+v", report.Scenarios)
	}
	if report.InvestmentResearch.InstitutionalMemo.Recommendation != "WATCH" {
		t.Fatalf("recommendation should remain unchanged: %+v", report.InvestmentResearch.InstitutionalMemo)
	}
}

func TestStrictEvidencePolicyAllowsCurrentValuationWhenLatestFinancialPeriodIsTimeSafe(t *testing.T) {
	asOf := time.Date(2026, 6, 18, 6, 0, 0, 0, time.UTC)
	published := time.Date(2026, 4, 28, 15, 31, 38, 0, time.UTC)
	report := Report{
		Coverage: CoverageReport{
			Score:     92,
			Available: []string{"financial_statements", "kap_pdf_ingest", "corporate_actions"},
		},
		Company: CompanyProfile{ClassificationConfidence: 0.97},
		Peers:   PeerComparison{PeerCount: 3},
		KAPPDFIngest: KAPPDFIngestSummary{
			Computed:                    true,
			TotalDocuments:              560,
			AnalysisUsableCount:         286,
			ReviewRequiredCount:         274,
			RejectedCount:               78,
			DecisionRelevantDocuments:   337,
			DecisionRelevantUsableCount: 180,
		},
		Scenarios: []Scenario{{Name: "base", PriceTarget: 420, ReturnPct: 5}},
		DataGovernance: FinancialDataGovernance{
			AsOf:                  asOf,
			DataMode:              "research",
			BacktestSafe:          false,
			FinanciallyConsistent: true,
			Source:                "isyatirim",
			LatestPeriod:          "2026-Q1",
			LatestPublishDate:     &published,
			LatestAvailableAt:     &published,
			LineageEvents:         21,
		},
		ValueInvesting: value.Report{
			Computed:    true,
			DataQuality: 100,
			Confidence:  78,
			IntrinsicValue: value.IntrinsicValueReport{
				Computed: true,
				Base:     390,
			},
			MarginOfSafety: value.MarginOfSafetyReport{
				Computed:    true,
				BasePct:     -2,
				RequiredPct: 20,
			},
		},
		InvestmentResearch: InvestmentResearchReview{
			DecisionFramework: DecisionFramework{CurrentDecision: "RED"},
			InstitutionalMemo: InstitutionalMemo{
				Recommendation: "AVOID",
				Decision:       "RED",
			},
		},
	}

	ApplyStrictEvidencePolicy(&report)

	if report.EvidencePolicy.Status != "pass" {
		t.Fatalf("expected policy pass with latest period time-safe, got %+v", report.EvidencePolicy)
	}
	if !report.ValueInvesting.Computed {
		t.Fatalf("value investing should stay computed when current evidence is sufficient: %+v", report.ValueInvesting)
	}
}

func TestStrictEvidencePolicyMarksMarketOnlyValuationNotApplicable(t *testing.T) {
	report := Report{
		Coverage: CoverageReport{
			Score: 100,
			Available: []string{
				"tradingview_ohlcv_price_volume",
				"technical_indicators",
				"support_resistance",
				"walk_forward_price_backtest",
				"usd_index_dxy_real_yield_macro",
				"futures_cot_open_interest_positioning",
				"gold_etf_physical_flow",
				"central_bank_geopolitical_news",
			},
		},
		Company: CompanyProfile{
			SectorSource:             "asset_type_commodity",
			ClassificationConfidence: 1,
		},
		Scenarios: []Scenario{{Name: "base", PriceTarget: 2400, ReturnPct: 0}},
		DataGovernance: FinancialDataGovernance{
			DataMode:              "research",
			BacktestSafe:          true,
			FinanciallyConsistent: true,
			Source:                "tradingview_ohlcv+commodity_context",
		},
		Valuation: ValuationAnalysis{
			FairValue: FairValueRange{
				Bear:       2200,
				Base:       2400,
				Bull:       2600,
				Drivers:    []string{"technical_range", "commodity_context_available"},
				Confidence: 0.6,
			},
		},
	}

	ApplyStrictEvidencePolicy(&report)

	if report.EvidencePolicy.Status != "pass" {
		t.Fatalf("expected market-only policy pass, got %+v", report.EvidencePolicy)
	}
	if report.EvidencePolicy.ValuationTargetsAllowed {
		t.Fatalf("equity valuation targets should remain disallowed for market-only assets")
	}
	if !report.EvidencePolicy.ScenarioTargetsAllowed || !report.EvidencePolicy.RecommendationAllowed {
		t.Fatalf("scenario and recommendation gates should stay available when coverage passes: %+v", report.EvidencePolicy)
	}
	if report.Valuation.FairValue.Base != 2400 || report.Valuation.FairValue.Confidence == 0 {
		t.Fatalf("technical/macro range should not be wiped for market-only assets: %+v", report.Valuation.FairValue)
	}
	if report.ValueInvesting.Decision != "NOT_APPLICABLE" {
		t.Fatalf("value investing should be not applicable, got %+v", report.ValueInvesting)
	}
	if len(report.Scenarios) != 1 {
		t.Fatalf("scenario bands should remain available: %+v", report.Scenarios)
	}
}

func TestStrictEvidencePolicyReportsOnlyActualValuationConfidenceGap(t *testing.T) {
	report := Report{
		Coverage: CoverageReport{
			Score:     95,
			Available: []string{"financial_statements", "kap_pdf_ingest"},
		},
		Company: CompanyProfile{ClassificationConfidence: 0.95},
		Peers:   PeerComparison{PeerCount: 4},
		KAPPDFIngest: KAPPDFIngestSummary{
			Computed:            true,
			TotalDocuments:      10,
			AnalysisUsableCount: 9,
		},
		DataGovernance: FinancialDataGovernance{
			DataMode:              "production",
			BacktestSafe:          true,
			FinanciallyConsistent: true,
			Source:                "financials/bilanco.json",
			LineageEvents:         2,
		},
		ValueInvesting: value.Report{
			Computed:    true,
			DataQuality: 90,
			Confidence:  60,
			IntrinsicValue: value.IntrinsicValueReport{
				Computed: true,
				Base:     100,
			},
		},
		InvestmentResearch: InvestmentResearchReview{
			ValuationBridge: ValuationTransparency{BaseIntrinsicValue: 100},
			InstitutionalMemo: InstitutionalMemo{
				Recommendation: "AVOID",
				Decision:       "AVOID",
			},
		},
	}

	ApplyStrictEvidencePolicy(&report)

	if report.EvidencePolicy.Status != "blocked" || report.EvidencePolicy.RecommendationAllowed {
		t.Fatalf("expected valuation confidence block to suppress recommendation, got %+v", report.EvidencePolicy)
	}
	if !containsString(report.EvidencePolicy.BlockingIssues, "valuation_confidence_below_70") {
		t.Fatalf("expected valuation confidence blocker, got %+v", report.EvidencePolicy.BlockingIssues)
	}
	for _, missing := range report.InvestmentResearch.ValuationBridge.MissingInputs {
		if missing == "finansal tablolar için dönem, para birimi, birim, konsolide/solo ve denetim bilgisi" {
			t.Fatalf("valuation-only blocker should not report financial statement metadata as missing: %+v", report.InvestmentResearch.ValuationBridge.MissingInputs)
		}
	}
	if !containsString(report.InvestmentResearch.ValuationBridge.MissingInputs, "değerleme güveni 70/100 üstüne çıkmalı") {
		t.Fatalf("missing issue-specific valuation confidence fix: %+v", report.InvestmentResearch.ValuationBridge.MissingInputs)
	}
	if report.InvestmentResearch.InstitutionalMemo.Recommendation != "INSUFFICIENT_DATA" {
		t.Fatalf("blocked policy should suppress recommendation: %+v", report.InvestmentResearch.InstitutionalMemo)
	}
}

func TestStrictEvidencePolicyBlocksMissingClassificationAndZeroPeers(t *testing.T) {
	report := strictPolicyPassFixture()
	report.Company = CompanyProfile{}
	report.Peers = PeerComparison{PeerCount: 0}

	ApplyStrictEvidencePolicy(&report)

	if report.EvidencePolicy.Status != "blocked" {
		t.Fatalf("expected missing classification/zero peers to block, got %+v", report.EvidencePolicy)
	}
	if !containsString(report.EvidencePolicy.BlockingIssues, "sector_classification_missing") {
		t.Fatalf("missing classification blocker absent: %+v", report.EvidencePolicy.BlockingIssues)
	}
	if !containsString(report.EvidencePolicy.BlockingIssues, "peer_sample_below_3") {
		t.Fatalf("zero peer blocker absent: %+v", report.EvidencePolicy.BlockingIssues)
	}
	if report.EvidencePolicy.ValuationTargetsAllowed {
		t.Fatalf("valuation targets must be blocked without classification and peers")
	}
}

func TestStrictEvidencePolicyBlocksREITWithoutNAVBridge(t *testing.T) {
	report := strictPolicyPassFixture()
	report.InvestmentResearch.ValuationBridge = ValuationTransparency{
		NAVStatus: "nav_not_reconciled_portfolio_totals_missing",
		NAVBridge: NAVBridge{
			Status:      "nav_not_reconciled",
			DataQuality: "missing_portfolio_total",
		},
	}

	ApplyStrictEvidencePolicy(&report)

	if report.EvidencePolicy.Status != "blocked" {
		t.Fatalf("expected REIT NAV bridge gap to block, got %+v", report.EvidencePolicy)
	}
	if !containsString(report.EvidencePolicy.BlockingIssues, "reit_nav_bridge_missing") {
		t.Fatalf("REIT NAV blocker absent: %+v", report.EvidencePolicy.BlockingIssues)
	}
	if report.EvidencePolicy.ValuationTargetsAllowed || len(report.Scenarios) != 0 {
		t.Fatalf("NAV gap should suppress valuation and scenario targets: %+v scenarios=%+v", report.EvidencePolicy, report.Scenarios)
	}
}

func strictPolicyPassFixture() Report {
	return Report{
		Coverage: CoverageReport{
			Score:     95,
			Available: []string{"financial_statements", "kap_pdf_ingest"},
		},
		Company: CompanyProfile{ClassificationConfidence: 0.95},
		Peers:   PeerComparison{PeerCount: 4},
		KAPPDFIngest: KAPPDFIngestSummary{
			Computed:            true,
			TotalDocuments:      10,
			AnalysisUsableCount: 9,
		},
		Scenarios: []Scenario{{Name: "base", PriceTarget: 120, ReturnPct: 20}},
		DataGovernance: FinancialDataGovernance{
			DataMode:              "production",
			BacktestSafe:          true,
			FinanciallyConsistent: true,
			Source:                "financials/bilanco.json",
			LineageEvents:         2,
		},
		ValueInvesting: value.Report{
			Computed:    true,
			DataQuality: 90,
			Confidence:  80,
			IntrinsicValue: value.IntrinsicValueReport{
				Computed: true,
				Base:     100,
			},
		},
		InvestmentResearch: InvestmentResearchReview{
			ValuationBridge: ValuationTransparency{BaseIntrinsicValue: 100, NAVStatus: "not_applicable"},
			InstitutionalMemo: InstitutionalMemo{
				Recommendation: "WATCH",
				Decision:       "WATCH",
			},
		},
	}
}
