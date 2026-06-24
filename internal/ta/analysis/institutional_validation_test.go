package analysis

import (
	"strings"
	"testing"

	"hissebot/internal/ta/contrarian"
	"hissebot/internal/ta/investorqa"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/value"
)

func TestValidateInstitutionalReadinessPassesMeasuredDailyReport(t *testing.T) {
	result := institutionalFixture()
	report := ValidateInstitutionalReadiness(result)
	if report.Status != "pass" {
		t.Fatalf("status = %s score=%.0f checks=%+v", report.Status, report.Score, report.Checks)
	}
	if report.Score < 95 {
		t.Fatalf("score = %.0f, want high pass", report.Score)
	}
}

func TestValidateInstitutionalReadinessFailsWeakBacktestAndBrokenText(t *testing.T) {
	result := institutionalFixture()
	daily := result.Timeframes["1D"]
	daily.Professional.Backtest.OutOfSampleReturn = -0.05
	daily.TradePlan.Reasoning = append(daily.TradePlan.Reasoning, "Spe hiez, de ir alm plai ylan")
	result.Timeframes["1D"] = daily

	report := ValidateInstitutionalReadiness(result)
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %+v", report)
	}
	if !hasInstitutionalCheck(report, "walk_forward_backtest", "fail") {
		t.Fatalf("missing failed walk-forward check: %+v", report.Checks)
	}
	if !hasInstitutionalCheck(report, "visual_text_quality", "fail") {
		t.Fatalf("missing failed visual text check: %+v", report.Checks)
	}
}

func TestValidateInstitutionalReadinessUsesCryptoDataStack(t *testing.T) {
	result := institutionalFixture()
	result.AssetType = ohlcv.AssetTypeCrypto
	result.Professional.Company = professional.CompanyProfile{Sector: "Crypto Assets", Industry: "Digital Asset", SectorSource: "asset_type_crypto", ClassificationConfidence: 1}
	result.Professional.Peers = professional.PeerComparison{Sector: "Crypto Assets", ValuationSignal: "not_applicable"}
	result.Professional.Coverage = professional.CoverageReport{
		Score:     50,
		Available: []string{"tradingview_ohlcv_price_volume", "technical_indicators"},
		Missing:   []string{"onchain_mvrv_nupl_sopr_realized_cap"},
	}
	result.Professional.DataQuality = 50

	report := ValidateInstitutionalReadiness(result)
	if report.Status != "limited" {
		t.Fatalf("status = %s checks=%+v", report.Status, report.Checks)
	}
	if !hasInstitutionalCheck(report, "crypto_data_stack", "limited") {
		t.Fatalf("missing crypto data stack check: %+v", report.Checks)
	}
	if hasInstitutionalCheck(report, "peer_universe", "fail") {
		t.Fatalf("crypto report should not run equity peer failure: %+v", report.Checks)
	}
}

func TestValidateInstitutionalReadinessUsesCommodityDataStack(t *testing.T) {
	result := institutionalFixture()
	result.AssetType = ohlcv.AssetTypeCommodity
	result.Professional.Company = professional.CompanyProfile{Sector: "Precious Metals", Industry: "Spot Gold", SectorSource: "asset_type_commodity", ClassificationConfidence: 1}
	result.Professional.Peers = professional.PeerComparison{Sector: "Precious Metals", ValuationSignal: "not_applicable"}
	result.Professional.Coverage = professional.CoverageReport{
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
	}
	result.Professional.DataQuality = 100

	report := ValidateInstitutionalReadiness(result)
	if report.Status != "pass" {
		t.Fatalf("status = %s checks=%+v", report.Status, report.Checks)
	}
	if !hasInstitutionalCheck(report, "commodity_data_stack", "pass") {
		t.Fatalf("missing commodity data stack pass: %+v", report.Checks)
	}
	if hasInstitutionalCheck(report, "value_investing", "fail") {
		t.Fatalf("commodity report should not run equity value investing failure: %+v", report.Checks)
	}
}

func TestValidateInstitutionalReadinessLimitsBankMissingCoreMetrics(t *testing.T) {
	result := institutionalFixture()
	result.Symbol = "ISCTR"
	result.Professional.Company = professional.CompanyProfile{
		Sector:                   "MALİ KURULUŞLAR",
		Industry:                 "BANKALAR",
		SectorSource:             "tradingview_sector_industry",
		ClassificationConfidence: 0.90,
	}
	result.Professional.Valuation = professional.ValuationAnalysis{
		SectorModel: "bank_equity_model",
		Flags:       []string{"bank_sector_requires_regulatory_capital_and_asset_quality_model"},
	}
	result.Professional.SectorFinancials = professional.SectorFinancialAnalysis{
		Profile:      "bank",
		ProfileLabel: "Banka",
		Score:        61,
		Warnings:     []string{"bank_reports_need_npl_capital_adequacy_regulatory_ratios"},
	}
	result.Professional.Peers = professional.PeerComparison{PeerCount: 8, Warnings: []string{"bank_peer_outlier_filter_applied_pb_2"}}
	result.InvestorQA.InstitutionalViews.OverallStatus = "fail"
	result.InvestorQA.InstitutionalViews.OverallQualityStatus = "pass"

	report := ValidateInstitutionalReadiness(result)
	if report.Status != "fail" {
		t.Fatalf("status = %s checks=%+v", report.Status, report.Checks)
	}
	if !hasInstitutionalCheck(report, "peer_universe", "fail") {
		t.Fatalf("missing failed peer universe: %+v", report.Checks)
	}
	if !hasInstitutionalCheck(report, "value_investing", "fail") {
		t.Fatalf("missing failed value investing: %+v", report.Checks)
	}
	if !hasInstitutionalCheck(report, "institutional_data_gates", "limited") {
		t.Fatalf("missing limited institutional gates: %+v", report.Checks)
	}
}

func institutionalFixture() SymbolAnalysis {
	return SymbolAnalysis{
		Symbol:       "TEST",
		CompanyName:  "Test A.S.",
		AnalysisDate: "2026-06-13",
		Currency:     "TRY",
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				LastClose: 100,
				TradePlan: ohlcv.TradePlan{
					Direction:       "neutral",
					Rejected:        true,
					RejectReason:    "short selling is not supported for this instrument",
					ConfidenceScore: 0.2,
					Reasoning:       []string{"Bearish evidence does not create a spot equity long setup."},
				},
				Professional: professional.TimeframeReport{
					Backtest: professional.BacktestResult{
						Strategy:            "test",
						ExecutionModel:      "signal_at_close_execute_next_open",
						BacktestSafe:        true,
						LookbackBars:        260,
						Trades:              40,
						OutOfSampleTrades:   12,
						WinRate:             0.55,
						Expectancy:          0.02,
						OutOfSampleReturn:   0.01,
						LookaheadViolations: 0,
					},
					SignalStats: professional.SignalStats{
						CurrentRegime:        "uptrend_positive_momentum",
						SampleSize:           30,
						ForwardBars:          20,
						WinRate:              0.56,
						AverageForwardReturn: 0.02,
						ProbabilityScore:     66,
					},
				},
			},
		},
		Professional: professional.Report{
			Coverage:    professional.CoverageReport{Score: 100},
			DataQuality: 100,
			Company: professional.CompanyProfile{
				Sector:                   "Elektronik Teknoloji",
				Industry:                 "Uzay ve Savunma",
				SectorSource:             "tradingview_sector_industry",
				ClassificationConfidence: 0.88,
			},
			Peers: professional.PeerComparison{PeerCount: 3},
			ValueInvesting: value.Report{
				Computed:      true,
				CurrentPrice:  100,
				Decision:      "MAKUL",
				DecisionLabel: "Fiyat içsel değere yakın fakat güvenlik marjı sınırlı",
				SectorModel: value.SectorModelReport{
					Model: "owner_earnings_dcf",
					Label: "Operasyonel şirket / owner earnings modeli",
				},
				IntrinsicValue: value.IntrinsicValueReport{
					Computed:   true,
					Bear:       95,
					Base:       120,
					Bull:       145,
					Confidence: 82,
				},
				MarginOfSafety: value.MarginOfSafetyReport{
					Computed:    true,
					BasePct:     16.7,
					RequiredPct: 25,
				},
				QualityScore: 78,
				Confidence:   82,
			},
		},
		Behavioral: contrarian.Report{
			SourceCoverage: contrarian.SourceCoverage{
				NewsItemCount:      10,
				KAPDisclosureCount: 5,
				RecentTextCount:    4,
				AnalyzedTextCount:  4,
				HasRecentSentiment: true,
			},
			Contrarian: contrarian.ContrarianReport{
				QualityGate:   contrarian.QualityGate{Status: "limited", CanAffectBuySignal: false},
				PlainLanguage: "Sentiment sınırlı; alım sinyali üretmez.",
			},
			Sentiment: contrarian.SentimentReport{PlainLanguage: "Söylem nötr."},
		},
		InvestorQA: investorqa.Report{
			Computed:      true,
			Decision:      "TAKIP",
			DecisionLabel: "Fiyat içsel değere yakın fakat güvenlik marjı sınırlı",
			Score:         76,
			Confidence:    78,
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed:             true,
				OverallStatus:        "pass",
				OverallQualityStatus: "pass",
				EliteCandidate: investorqa.EliteGate{
					Computed: true,
					Status:   "pass",
					Score:    82,
					Label:    "Üç aksiyon kapısı geçti",
					Summary:  "Bu hisse değer yatırım tezi, kurumsal portföy uygunluğu ve trading edge sinyali açısından aynı anda başarılı adaydır.",
				},
				Summary:        "Yatırım/işlem uygunluğu: değer yatırım geçti, kurumsal portföy geçti, trading edge geçti; genel geçti. Rapor kalitesi: geçti.",
				QualitySummary: "Rapor kalitesi: değer yatırım geçti, kurumsal portföy geçti, trading edge geçti; genel geçti.",
				ValueInvesting: investorqa.PersonaView{
					Name:                "Değer Yatırım Standardı",
					Status:              "pass",
					Score:               82,
					DecisionLabel:       "Değer yatırım standardında yatırım komitesine girebilir",
					ReportQualityStatus: "pass",
					ReportQualityScore:  90,
				},
				Portfolio: investorqa.PersonaView{
					Name:                "Kurumsal Portföy Standardı",
					Status:              "pass",
					Score:               84,
					DecisionLabel:       "Kurumsal portföy ön elemesine uygun",
					ReportQualityStatus: "pass",
					ReportQualityScore:  90,
				},
				TradingEdge: investorqa.PersonaView{
					Name:                "Trading Edge Standardı",
					Status:              "pass",
					Score:               80,
					DecisionLabel:       "Aktif trading araştırma masasına aday",
					ReportQualityStatus: "pass",
					ReportQualityScore:  90,
				},
			},
			Questions: []investorqa.QuestionAnswer{
				{Question: "q1", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q2", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q3", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q4", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q5", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q6", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q7", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q8", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q9", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q10", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q11", Answer: "a", Status: "pass", Confidence: 80},
				{Question: "q12", Answer: "a", Status: "pass", Confidence: 80},
			},
			Quality:   investorqa.QualityReport{Score: 75},
			Liquidity: investorqa.LiquidityReport{Score: 80},
			ModelRisk: investorqa.ModelRiskReport{Score: 76},
		},
		Disclaimer: ohlcv.Disclaimer,
	}
}

func hasInstitutionalCheck(report InstitutionalValidation, name, status string) bool {
	for _, check := range report.Checks {
		if check.Name == name && strings.EqualFold(check.Status, status) {
			return true
		}
	}
	return false
}
