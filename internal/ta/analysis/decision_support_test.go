package analysis

import (
	"strings"
	"testing"

	"hissebot/internal/domain/marketdata"
	"hissebot/internal/domain/pricequality"
	"hissebot/internal/ta/investorqa"
	"hissebot/internal/ta/macro"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	valuepkg "hissebot/internal/ta/value"
)

func TestBuildDecisionSupportIssuesDirectDecisionsWhileBlockingTrading(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "TEST",
		AssetType: ohlcv.AssetTypeEquity,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe:         "1D",
				LastClose:         10,
				NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 9.5},
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 11},
				TradePlan:         ohlcv.TradePlan{Rejected: true, RejectReason: "risk_reward_below_threshold"},
				Professional: professional.TimeframeReport{
					Backtest: professional.BacktestResult{
						BacktestSafe:      true,
						Trades:            14,
						OutOfSampleTrades: 2,
						Expectancy:        0.01,
						OutOfSampleReturn: -0.01,
					},
					SignalStats: professional.SignalStats{SampleSize: 9, InsufficientData: true},
					Technical: professional.TechnicalEvidence{
						SignalGate: professional.TechnicalSignalGate{
							Status:     "fail",
							Actionable: false,
							Blockers:   []string{"backtest/OOS istatistiği aktif sinyal için yetersiz"},
						},
					},
				},
			},
		},
		Professional: professional.Report{
			Coverage: professional.CoverageReport{
				Score:   45,
				Missing: []string{"financial_statements", "peer_comparison_min_3", "recent_kap_news_disclosures"},
			},
			EvidencePolicy: professional.EvidencePolicyReport{
				Status:         "blocked",
				BlockingIssues: []string{"coverage_score_below_80", "financial_statements_or_intrinsic_value_missing"},
			},
			DataGovernance: professional.FinancialDataGovernance{
				AvailabilityStatus:    "missing_financial_statements",
				BacktestSafe:          false,
				ProductionReady:       false,
				FinanciallyConsistent: false,
			},
			Company: professional.CompanyProfile{ClassificationConfidence: 0.40},
			Peers:   professional.PeerComparison{PeerCount: 1},
		},
		InvestorQA: investorqa.Report{
			Computed:   true,
			Confidence: 51,
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed: true,
				FinancialTransactionUse: investorqa.TransactionUseGate{
					Status: "fail",
				},
				Portfolio: investorqa.PersonaView{
					Status:              "limited",
					ReportQualityStatus: "limited",
				},
				TradingEdge: investorqa.PersonaView{
					Status:               "fail",
					TransactionUseStatus: "fail",
				},
			},
		},
		InstitutionalValidation: InstitutionalValidation{
			Status:  "fail",
			Score:   51,
			Summary: "Rapor güvenlik ve doğrulama kapısı başarısız.",
			Checks: []InstitutionalValidationCheck{
				{Name: "walk_forward_backtest", Status: "limited", Message: "Backtest örnek sayısı sınırlı.", Details: []string{"trades=14", "out_of_sample_trades=2"}},
			},
		},
	}

	report := BuildDecisionSupport(result)
	if report == nil {
		t.Fatal("decision support report is nil")
	}
	if report.Status != "decision_issued_with_limitations" {
		t.Fatalf("expected decision_issued_with_limitations, got %s", report.Status)
	}
	if useCaseAllowed(report, "Büyük yatırımcı karar raporu") || useCaseAllowed(report, "Küçük yatırımcı doğrudan AL/SAT raporu") {
		t.Fatalf("audience decision classes must not pass failed central gates: %+v", report.UseCaseMatrix)
	}
	if report.Institutional.Decision == "" || report.Retail.Signal == "" {
		t.Fatalf("central engine must still issue explicit reject/wait decisions: institutional=%+v retail=%+v", report.Institutional, report.Retail)
	}
	if !strings.Contains(report.Retail.OneLineAnswer, "sinyal güveni") || strings.Contains(report.Retail.OneLineAnswer, "; güven ") {
		t.Fatalf("retail one-line answer must label signal confidence clearly: %s", report.Retail.OneLineAnswer)
	}
	if !strings.Contains(report.Institutional.OneLineAnswer, "kurumsal karar güveni") {
		t.Fatalf("institutional one-line answer must label decision confidence clearly: %s", report.Institutional.OneLineAnswer)
	}
	for _, name := range []string{"AL/SAT sinyali", "Production trading / otomatik emir", "Tek başına yatırım tavsiyesi"} {
		if useCaseAllowed(report, name) {
			t.Fatalf("%s should be blocked: %+v", name, report.UseCaseMatrix)
		}
	}
	if !missingInputExists(report, "financial_statements") {
		t.Fatalf("financial statements missing input not reported: %+v", report.MissingInputs)
	}
	if !strings.Contains(report.BatchRefresh.Command, "analyze --all") {
		t.Fatalf("batch refresh command missing analyze --all: %+v", report.BatchRefresh)
	}
}

func TestBuildDecisionSupportBlocksBuyWhenCloseIsNotVerified(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "TEST",
		AssetType: ohlcv.AssetTypeEquity,
		PriceQuality: &pricequality.SymbolReport{
			Symbol:                "TEST",
			Status:                pricequality.StatusProvisionalLastPrice,
			ReadyForVerifiedClose: false,
			LatestTradingDate:     "2026-06-18",
			BlockingReasons:       []string{"official_final_close_missing"},
		},
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				LastClose: 10,
				TradePlan: ohlcv.TradePlan{
					EntryMin:        9.9,
					EntryMax:        10.1,
					StopLoss:        9.4,
					TakeProfit1:     11.2,
					RiskRewardRatio: 2,
				},
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						SignalGate: professional.TechnicalSignalGate{Status: "pass", Actionable: true},
					},
					Backtest:    professional.BacktestResult{BacktestSafe: true, Trades: 40, OutOfSampleTrades: 12, Expectancy: 0.02, OutOfSampleReturn: 0.03},
					SignalStats: professional.SignalStats{SampleSize: 40, WinRate: 0.55, AverageForwardReturn: 0.02},
				},
			},
		},
		Professional: professional.Report{
			EvidencePolicy: professional.EvidencePolicyReport{Status: "pass"},
			DataGovernance: professional.FinancialDataGovernance{
				AvailabilityStatus:    "verified_publish_dates",
				BacktestSafe:          true,
				ProductionReady:       true,
				FinanciallyConsistent: true,
				LatestPeriod:          "2026-Q1",
			},
			Company: professional.CompanyProfile{Sector: "Teknoloji", Industry: "Yazılım", ClassificationConfidence: 0.95},
			ValueInvesting: valuepkg.Report{
				Computed:   true,
				Confidence: 90,
			},
		},
		InvestorQA: investorqa.Report{
			Computed: true,
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed: true,
				FinancialTransactionUse: investorqa.TransactionUseGate{
					Status: "pass",
				},
				Portfolio: investorqa.PersonaView{
					Status:              "pass",
					ReportQualityStatus: "pass",
					Score:               88,
				},
				TradingEdge: investorqa.PersonaView{
					Status:               "pass",
					TransactionUseStatus: "pass",
				},
			},
		},
		InstitutionalValidation: InstitutionalValidation{Status: "pass", Score: 90},
	}

	report := BuildDecisionSupport(result)
	if report.Status != "decision_issued_with_limitations" {
		t.Fatalf("unreconciled price must issue a limited decision, got %s", report.Status)
	}
	if useCaseAllowed(report, "AL/SAT sinyali") || useCaseAllowed(report, "Production trading / otomatik emir") {
		t.Fatalf("buy/production use-cases must be blocked without verified close: %+v", report.UseCaseMatrix)
	}
	gate := actionGate(report, "verified_price_close")
	if gate == nil || gate.Status != "limited" || !gate.Blocking {
		t.Fatalf("verified price gate must be limited/blocking, got %+v", gate)
	}
	if !missingInputExists(report, "decision_price_reconciliation") {
		t.Fatalf("decision price reconciliation input not reported: %+v", report.MissingInputs)
	}
}

func TestBuildDecisionSupportBlocksBuyWhenForecastModelIsNotDecisionGrade(t *testing.T) {
	result := centralClassificationFixture()
	result.Symbol = "ASELS"
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

	report := BuildDecisionSupport(result)
	if useCaseAllowed(report, "AL/SAT sinyali") || useCaseAllowed(report, "Production trading / otomatik emir") {
		t.Fatalf("not-decision-grade forecast must block trading use cases: %+v", report.UseCaseMatrix)
	}
	gate := actionGate(report, "next_session_forecast_model")
	if gate == nil || gate.Status != "fail" || !gate.Blocking {
		t.Fatalf("forecast model gate must fail/block, got %+v", gate)
	}
	if !missingInputExists(report, "next_session_forecast_model_validation") {
		t.Fatalf("forecast model validation missing input not reported: %+v", report.MissingInputs)
	}
}

func TestBuildDecisionSupportLimitsBankWhenCoreMetricsMissing(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "ISCTR",
		AssetType: ohlcv.AssetTypeEquity,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				LastClose: 10,
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						SignalGate: professional.TechnicalSignalGate{Status: "pass", Actionable: true},
					},
					Backtest:    professional.BacktestResult{BacktestSafe: true, Trades: 40, OutOfSampleTrades: 12, Expectancy: 0.02, OutOfSampleReturn: 0.03},
					SignalStats: professional.SignalStats{SampleSize: 40, WinRate: 0.55, AverageForwardReturn: 0.02},
				},
			},
		},
		Professional: professional.Report{
			EvidencePolicy: professional.EvidencePolicyReport{Status: "pass"},
			DataGovernance: professional.FinancialDataGovernance{
				AvailabilityStatus:    "verified_publish_dates",
				BacktestSafe:          true,
				ProductionReady:       false,
				FinanciallyConsistent: true,
				LatestPeriod:          "2026-Q1",
			},
			Company: professional.CompanyProfile{Sector: "Banka", Industry: "Bankacılık", ClassificationConfidence: 0.95},
			Valuation: professional.ValuationAnalysis{
				SectorModel: "bank_equity_model",
				Flags:       []string{"bank_sector_requires_regulatory_capital_and_asset_quality_model"},
			},
			SectorFinancials: professional.SectorFinancialAnalysis{
				Profile:  "financials",
				Warnings: []string{"bank_specific_regulatory_metrics_missing"},
			},
			ValueInvesting: valuepkg.Report{
				Computed:   true,
				Confidence: 90,
			},
		},
		InvestorQA: investorqa.Report{
			Computed: true,
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed: true,
				FinancialTransactionUse: investorqa.TransactionUseGate{
					Status: "pass",
				},
				Portfolio: investorqa.PersonaView{
					Status:              "pass",
					ReportQualityStatus: "pass",
					Score:               88,
				},
				TradingEdge: investorqa.PersonaView{
					Status:               "pass",
					TransactionUseStatus: "pass",
				},
			},
		},
		InstitutionalValidation: InstitutionalValidation{Status: "pass", Score: 90},
	}

	report := BuildDecisionSupport(result)
	if report.Status != "decision_issued_with_limitations" {
		t.Fatalf("bank missing core metrics must issue a limited decision, got %s", report.Status)
	}
	if useCaseAllowed(report, "Kurumsal portföy pozisyonu") {
		t.Fatalf("portfolio use-case must be blocked when bank core metrics are missing: %+v", report.UseCaseMatrix)
	}
	gate := actionGate(report, "value_investing")
	if gate == nil || gate.Status != "limited" {
		t.Fatalf("value gate must be limited by bank core metrics, got %+v", gate)
	}
	if !strings.Contains(gate.Current, "bank_core_metrics=missing") {
		t.Fatalf("value gate current must explain missing bank metrics, got %q", gate.Current)
	}
}

func TestBuildDecisionSupportDoesNotTreatREITAsBank(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "ALGYO",
		AssetType: ohlcv.AssetTypeEquity,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				LastClose: 4.21,
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						SignalGate: professional.TechnicalSignalGate{Status: "fail", Actionable: false},
					},
					Backtest:    professional.BacktestResult{BacktestSafe: true, Trades: 24, OutOfSampleTrades: 8, Expectancy: 0.0067, OutOfSampleReturn: -0.0172},
					SignalStats: professional.SignalStats{SampleSize: 16, WinRate: 0.875, AverageForwardReturn: 0.2037},
				},
			},
		},
		Professional: professional.Report{
			EvidencePolicy: professional.EvidencePolicyReport{
				Status:         "blocked",
				BlockingIssues: []string{"reit_nav_bridge_missing"},
			},
			DataGovernance: professional.FinancialDataGovernance{
				AvailabilityStatus:    "lookahead_safe_conservative_available_at",
				BacktestSafe:          true,
				ProductionReady:       false,
				FinanciallyConsistent: true,
			},
			Company: professional.CompanyProfile{
				Sector:                   "MALİ KURULUŞLAR",
				Industry:                 "GAYRİMENKUL YATIRIM ORTAKLIKLARI",
				ClassificationConfidence: 0.97,
			},
			Valuation: professional.ValuationAnalysis{SectorModel: "reit_nav_proxy_model"},
			SectorFinancials: professional.SectorFinancialAnalysis{
				Profile:      "reit_nav",
				ProfileLabel: "Gayrimenkul yatirim ortakligi",
				Warnings:     []string{"reit_reports_need_appraisal_based_nav_and_portfolio_occupancy_for_full_view"},
			},
			ValueInvesting: valuepkg.Report{
				Computed:   false,
				Confidence: 79.4,
				SectorModel: valuepkg.SectorModelReport{
					Model: "gyo_nav_proxy",
					Label: "GYO / net aktif değer proxy modeli",
				},
			},
		},
		InvestorQA: investorqa.Report{
			Computed: true,
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed:                true,
				FinancialTransactionUse: investorqa.TransactionUseGate{Status: "fail"},
				Portfolio:               investorqa.PersonaView{Status: "limited", ReportQualityStatus: "pass"},
				TradingEdge:             investorqa.PersonaView{Status: "fail", TransactionUseStatus: "fail"},
			},
		},
		InstitutionalValidation: InstitutionalValidation{Status: "fail", Score: 59},
	}

	report := BuildDecisionSupport(result)
	if missingInputExists(report, "bank_metric_capital_adequacy_ratio") {
		t.Fatalf("REIT must not request bank core metrics: %+v", report.MissingInputs)
	}
	gate := actionGate(report, "value_investing")
	if gate != nil && strings.Contains(gate.Current, "bank_core_metrics=missing") {
		t.Fatalf("REIT value gate must not mention bank metrics: %+v", gate)
	}
	if !missingInputExists(report, "reit_nav_bridge_missing") {
		t.Fatalf("REIT NAV bridge missing input should remain: %+v", report.MissingInputs)
	}
}

func TestMacroContextGateUsesAnalysisReadyNotFullEVDSArchive(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "ISCTR",
		AssetType: ohlcv.AssetTypeEquity,
		Professional: professional.Report{
			Coverage: professional.CoverageReport{Score: 82},
			Market: professional.MarketContext{
				BenchmarkAvailable: true,
				LiveSnapshot:       &marketdata.LiveMarketSnapshot{ActiveUsers: 1},
				GDP:                macro.GDPContext{Computed: true},
			},
			TCMBContext: professional.TCMBContextReport{Computed: true},
			TCMBEVDSContext: professional.TCMBEVDSContextReport{
				Computed:           false,
				AnalysisReady:      true,
				SeriesFileCount:    582,
				CatalogSeriesCount: 49616,
			},
		},
	}

	if !macroContextOK(result) {
		t.Fatal("macro gate should pass when required EVDS analysis inputs are ready, even if the full EVDS archive is partial")
	}
	current := macroContextCurrent(result)
	for _, want := range []string{"evds_ready=true", "evds_full_archive=false", "evds_files=582/49616"} {
		if !strings.Contains(current, want) {
			t.Fatalf("macro current summary missing %q: %s", want, current)
		}
	}

	result.Professional.TCMBEVDSContext.AnalysisReady = false
	if macroContextOK(result) {
		t.Fatal("macro gate must fail when EVDS analysis inputs are not ready")
	}
}

func TestBuildDecisionSupportUsesMicrostructureForExecutionNotRetailDecision(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "TEST",
		AssetType: ohlcv.AssetTypeEquity,
		PriceQuality: &pricequality.SymbolReport{
			Symbol:                "TEST",
			Status:                pricequality.StatusReadyForVerifiedClose,
			ReadyForDecision:      true,
			ReadyForVerifiedClose: true,
			LatestTradingDate:     "2026-06-19",
		},
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				LastClose: 10,
				TradePlan: ohlcv.TradePlan{
					Direction:       "long",
					EntryMin:        9.9,
					EntryMax:        10.1,
					StopLoss:        9.4,
					TakeProfit1:     11.2,
					RiskRewardRatio: 2,
				},
				Professional: professional.TimeframeReport{
					Technical:   professional.TechnicalEvidence{SignalGate: professional.TechnicalSignalGate{Status: "pass", Actionable: true, Score: 82}},
					Backtest:    professional.BacktestResult{BacktestSafe: true, Trades: 40, OutOfSampleTrades: 12, Expectancy: 0.02, OutOfSampleReturn: 0.03},
					SignalStats: professional.SignalStats{SampleSize: 40, WinRate: 0.55, AverageForwardReturn: 0.02},
				},
			},
		},
		Professional: professional.Report{
			Coverage:       professional.CoverageReport{Score: 92},
			DataQuality:    92,
			EvidencePolicy: professional.EvidencePolicyReport{Status: "pass"},
			DataGovernance: professional.FinancialDataGovernance{
				AvailabilityStatus:    "verified_publish_dates",
				BacktestSafe:          true,
				ProductionReady:       true,
				FinanciallyConsistent: true,
				LatestPeriod:          "2026-Q1",
			},
			Company:          professional.CompanyProfile{Sector: "Sanayi", Industry: "İmalat", ClassificationConfidence: 0.95},
			SectorFinancials: professional.SectorFinancialAnalysis{Profile: "manufacturing_general"},
			Valuation:        professional.ValuationAnalysis{FairValue: professional.FairValueRange{Base: 12}},
			Market: professional.MarketContext{
				BenchmarkAvailable: true,
				LiveSnapshot:       &marketdata.LiveMarketSnapshot{ActiveUsers: 1},
				GDP:                macro.GDPContext{Computed: true},
			},
			TCMBContext: professional.TCMBContextReport{Computed: true},
			TCMBEVDSContext: professional.TCMBEVDSContextReport{
				AnalysisReady: true,
				ForecastImpact: professional.TCMBMacroForecastImpact{
					Computed:    true,
					Direction:   "positive",
					Severity:    "low",
					Confidence:  90,
					DecisionUse: "score_and_gate_input",
				},
			},
			ValueInvesting: valuepkg.Report{
				Computed: true, Confidence: 90,
				SectorModel:    valuepkg.SectorModelReport{Model: "owner_earnings_dcf"},
				IntrinsicValue: valuepkg.IntrinsicValueReport{Computed: true, Base: 10},
			},
		},
		InvestorQA: investorqa.Report{
			Computed:  true,
			Score:     80,
			ModelRisk: investorqa.ModelRiskReport{Score: 80},
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed:                true,
				FinancialTransactionUse: investorqa.TransactionUseGate{Status: "pass"},
				ValueInvesting:          investorqa.PersonaView{Status: "pass", Score: 80},
				Portfolio:               investorqa.PersonaView{Status: "pass", ReportQualityStatus: "pass", Score: 90},
				TradingEdge:             investorqa.PersonaView{Status: "pass", TransactionUseStatus: "pass"},
			},
		},
		InstitutionalValidation: InstitutionalValidation{Status: "pass", Score: 90},
	}

	report := BuildDecisionSupport(result)
	if !useCaseAllowed(report, "AL/SAT sinyali") {
		t.Fatalf("retail AL/SAT decision should not require order-book execution data: %+v", report.UseCaseMatrix)
	}
	gate := actionGate(report, "market_microstructure_gate")
	if gate == nil || gate.Status != "fail" || !gate.Blocking {
		t.Fatalf("missing microstructure gate must fail/block, got %+v", gate)
	}

	result.Professional.Market.Microstructure = &professional.MarketMicrostructureContext{
		Computed: true,
		Status:   "pass",
		Score:    100,
		Quote:    professional.MarketMicrostructureQuote{Available: true, Last: 10, Bid: 9.99, Ask: 10.01},
		OrderBook: professional.MarketOrderBookContext{
			Available: true,
			BidLevels: 2,
			AskLevels: 2,
			BestBid:   9.99,
			BestAsk:   10.01,
			SpreadBps: 20,
		},
		Depth:                 professional.MarketDepthContext{Available: true, Levels: 10},
		BrokerageDistribution: professional.MarketBrokerageDistribution{Available: true, ResultCount: 12},
		Custody:               professional.MarketCustodyDistribution{Available: true, ResultCount: 8},
		Liquidity: professional.MarketMicrostructureLiquidity{
			TopOfBookAvailable:     true,
			SpreadBps:              20,
			AutomaticOrderReady:    true,
			MicrostructureComplete: true,
			DecisionUsable:         true,
		},
	}
	report = BuildDecisionSupport(result)
	if !useCaseAllowed(report, "AL/SAT sinyali") {
		t.Fatalf("AL/SAT should pass when all gates including microstructure pass: %+v", report.UseCaseMatrix)
	}
	gate = actionGate(report, "market_microstructure_gate")
	if gate == nil || gate.Status != "pass" || gate.Blocking {
		t.Fatalf("microstructure gate should pass, got %+v", gate)
	}
}

func TestBuildDecisionSupportBlocksBuyOnSevereMacroHeadwind(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "TEST",
		AssetType: ohlcv.AssetTypeEquity,
		PriceQuality: &pricequality.SymbolReport{
			Symbol:                "TEST",
			Status:                pricequality.StatusReadyForVerifiedClose,
			ReadyForVerifiedClose: true,
			LatestTradingDate:     "2026-06-19",
		},
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				LastClose: 10,
				TradePlan: ohlcv.TradePlan{
					Direction:       "long",
					EntryMin:        9.9,
					EntryMax:        10.1,
					StopLoss:        9.4,
					TakeProfit1:     11.2,
					RiskRewardRatio: 2,
				},
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						SignalGate: professional.TechnicalSignalGate{Status: "pass", Actionable: true, Score: 82},
					},
					Backtest:    professional.BacktestResult{BacktestSafe: true, Trades: 40, OutOfSampleTrades: 12, Expectancy: 0.02, OutOfSampleReturn: 0.03},
					SignalStats: professional.SignalStats{SampleSize: 40, WinRate: 0.55, AverageForwardReturn: 0.02},
				},
			},
		},
		Professional: professional.Report{
			Coverage:       professional.CoverageReport{Score: 92},
			EvidencePolicy: professional.EvidencePolicyReport{Status: "pass"},
			DataGovernance: professional.FinancialDataGovernance{
				AvailabilityStatus:    "verified_publish_dates",
				BacktestSafe:          true,
				ProductionReady:       true,
				FinanciallyConsistent: true,
				LatestPeriod:          "2026-Q1",
			},
			Company: professional.CompanyProfile{Sector: "Sanayi", Industry: "İmalat", ClassificationConfidence: 0.95},
			Market: professional.MarketContext{
				BenchmarkAvailable: true,
				LiveSnapshot:       &marketdata.LiveMarketSnapshot{ActiveUsers: 1},
				GDP:                macro.GDPContext{Computed: true},
			},
			TCMBContext: professional.TCMBContextReport{Computed: true},
			TCMBEVDSContext: professional.TCMBEVDSContextReport{
				AnalysisReady:   true,
				PointInTimeSafe: true,
				ScoreEligible:   true,
				ForecastImpact: professional.TCMBMacroForecastImpact{
					Computed:        true,
					Direction:       "negative",
					Label:           "negatif makro rüzgar",
					Severity:        "high",
					Confidence:      90,
					PressureScore:   -82,
					ScoreAdjustment: -6.5,
					DecisionUse:     "blocking_headwind",
					Summary:         "Makro rejim fiyat beklentisini baskılıyor.",
					Drivers:         []string{"policy_rate -2.50: sıkı faiz rejimi"},
					Blockers:        []string{"macro_severe_headwind"},
				},
			},
			ValueInvesting: valuepkg.Report{
				Computed:   true,
				Confidence: 90,
			},
		},
		InvestorQA: investorqa.Report{
			Computed: true,
			InstitutionalViews: investorqa.InstitutionalPersonaViews{
				Computed:                true,
				FinancialTransactionUse: investorqa.TransactionUseGate{Status: "pass"},
				Portfolio:               investorqa.PersonaView{Status: "pass", ReportQualityStatus: "pass", Score: 90},
				TradingEdge:             investorqa.PersonaView{Status: "pass", TransactionUseStatus: "pass"},
			},
		},
		InstitutionalValidation: InstitutionalValidation{Status: "pass", Score: 90},
	}

	report := BuildDecisionSupport(result)
	if useCaseAllowed(report, "AL/SAT sinyali") {
		t.Fatalf("severe macro headwind must block AL/SAT: %+v", report.UseCaseMatrix)
	}
	if useCaseAllowed(report, "Production trading / otomatik emir") {
		t.Fatalf("severe macro headwind must block production trading: %+v", report.UseCaseMatrix)
	}
	gate := actionGate(report, "macro_regime_confirmation")
	if gate == nil || gate.Status != "limited" || !gate.Blocking {
		t.Fatalf("macro gate must be limited/blocking, got %+v", gate)
	}
	if !strings.Contains(gate.Current, "blocking_headwind") {
		t.Fatalf("macro gate current must expose decision use, got %q", gate.Current)
	}
}

func useCaseAllowed(report *DecisionSupportReport, name string) bool {
	for _, item := range report.UseCaseMatrix {
		if item.UseCase == name {
			return item.Allowed
		}
	}
	return false
}

func actionGate(report *DecisionSupportReport, name string) *DecisionActionGate {
	for i := range report.ActionGates {
		if report.ActionGates[i].Name == name {
			return &report.ActionGates[i]
		}
	}
	return nil
}

func missingInputExists(report *DecisionSupportReport, key string) bool {
	for _, item := range report.MissingInputs {
		if item.Key == key {
			return true
		}
	}
	return false
}
