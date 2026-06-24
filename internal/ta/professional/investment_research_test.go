package professional

import (
	"testing"

	"hissebot/internal/ta/value"
)

func TestBuildNAVBridgeCreatesPartialProxy(t *testing.T) {
	portfolio := 4_500_000_000.0
	got := buildNAVBridge(
		ValuationAnalysis{NetDebt: 500_000_000, MarketCap: 3_000_000_000, PaidCapital: 650_000_000},
		KAPAssetInventorySummary{PortfolioSummary: KAPAssetPortfolioSummary{TotalRealEstateValueExclVATTRY: &portfolio}},
	)
	if got.Status != "partial_nav_proxy" || got.EstimatedNAVTRY != 4_000_000_000 {
		t.Fatalf("unexpected NAV bridge: %+v", got)
	}
	if got.EstimatedNAVPerShare <= 0 || got.MarketCapToNAVPremiumPct >= 0 {
		t.Fatalf("expected per-share NAV and discount: %+v", got)
	}
}

func TestBuildInstitutionalMemoBlocksDirectBuyWhenCoreFixesMissing(t *testing.T) {
	memo := buildInstitutionalMemo(
		[]InvestorReadiness{{Segment: "institutional_professional_decision", CoveragePct: 55}},
		ValuationTransparency{
			CurrentPrice:       6.00,
			NAVStatus:          "partial_portfolio_total_available_not_full_nav",
			BaseIntrinsicValue: 5.16,
			BearIntrinsicValue: 4.50,
			PriceToBasePct:     16.2,
			NAVBridge:          NAVBridge{Status: "partial_nav_proxy"},
		},
		AssetDueDiligence{
			InventoryComputed:     true,
			EventCount:            12,
			DisplayAssetCount:     5,
			RentalAssetCount:      0,
			ValuationLinkedStatus: "not_linked_to_valuation_portfolio_totals_missing",
		},
		FinancialQualityBridge{RedFlags: []string{"negative_fcf"}},
		DecisionFramework{CurrentDecision: "BEKLE"},
		KAPPDFIngestSummary{TotalDocuments: 10, AnalysisUsableCount: 8, ReviewRequiredCount: 2},
		value.Report{
			DecisionLabel:  "BEKLE",
			Confidence:     59,
			MarginOfSafety: value.MarginOfSafetyReport{Computed: true, BasePct: -14, RequiredPct: 30},
		},
		true,
	)
	if memo.DirectBuyEligible || memo.InvestmentCommitteeReady || memo.BrokeragePublishableReady {
		t.Fatalf("memo should block institutional use: %+v", memo)
	}
	for _, want := range []string{"rapor_karari_dogrudan_al_degildir", "tam_nad_mutabakati_yok", "kira_doluluk_eslesmesi_yok"} {
		if !containsString(memo.BlockingIssues, want) {
			t.Fatalf("missing blocker %q in %+v", want, memo.BlockingIssues)
		}
	}
	if len(memo.RequiredFixes) < 4 || memo.Recommendation != "INSUFFICIENT_DATA" || memo.WorkflowStatus != "research_backlog_or_watchlist" {
		t.Fatalf("unexpected memo fix plan: %+v", memo)
	}
	if memo.PositionSizeSuggestion == "" || memo.InvestmentHorizon == "" || memo.LiquidityConsideration == "" || memo.PortfolioFit == "" {
		t.Fatalf("memo missing committee decision fields: %+v", memo)
	}
	if len(memo.KeyAssumptions) == 0 || len(memo.ApprovalConditions) == 0 || len(memo.RejectionConditions) == 0 {
		t.Fatalf("memo missing assumptions/conditions: %+v", memo)
	}
}
