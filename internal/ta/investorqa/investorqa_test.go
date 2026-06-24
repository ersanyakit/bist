package investorqa

import (
	"strings"
	"testing"
	"time"

	"hissebot/internal/ta/corporateactions"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/value"
)

func TestAnalyzeEquityAnswersInvestorQuestions(t *testing.T) {
	report := Analyze(Input{
		Symbol:       "TEST",
		Currency:     "TRY",
		AssetType:    ohlcv.AssetTypeEquity,
		OverallScore: 62,
		OverallBias:  "neutral",
		Professional: professional.Report{
			Coverage:    professional.CoverageReport{Score: 90, Available: []string{"financials", "ohlcv"}},
			DataQuality: 88,
			Company:     professional.CompanyProfile{Sector: "Sanayi", Industry: "Savunma", PeerGroup: "bist_defense"},
			Market:      professional.MarketContext{BenchmarkSymbol: "XU100", BenchmarkAvailable: true, RelativeStrength20: 0.04, RelativeStrength60: 0.03, Beta60: 0.9},
			Peers:       professional.PeerComparison{PeerCount: 5, ValuationSignal: "discount"},
			Valuation: professional.ValuationAnalysis{
				NetIncomeTTM:     100,
				FreeCashFlowTTM:  90,
				NetDebt:          20,
				Ratios:           map[string]float64{"ROE": 0.20, "Net_Margin": 0.15, "NetDebt_Eq": 0.2},
				FairValue:        professional.FairValueRange{Base: 130, Confidence: 0.75},
				MarketCap:        900,
				Equity:           500,
				EnterpriseValue:  920,
				OperatingCashTTM: 110,
			},
			Disclosure:     professional.DisclosureReview{KAPCompanyCardAvailable: true, RecentDisclosureStatus: "available", RecentDisclosureCount: 3},
			DataGovernance: professional.FinancialDataGovernance{FinanciallyConsistent: true, LineageEvents: 4, ProductionReady: true, BacktestSafe: true},
			Scenarios:      []professional.Scenario{{Name: "bear", PriceTarget: 80, ReturnPct: -20}, {Name: "base", PriceTarget: 120, ReturnPct: 20}, {Name: "bull", PriceTarget: 150, ReturnPct: 50}},
			ValueInvesting: value.Report{
				Computed:     true,
				CurrentPrice: 100,
				IntrinsicValue: value.IntrinsicValueReport{
					Computed:   true,
					Base:       140,
					Confidence: 80,
				},
				MarginOfSafety: value.MarginOfSafetyReport{Computed: true, BasePct: 28.5, RequiredPct: 25},
				CapitalAllocation: value.CapitalAllocationReport{
					Score: 78,
				},
				Moat:         value.MoatReport{Score: 82},
				ValueScore:   80,
				QualityScore: 78,
				Confidence:   80,
			},
		},
		Timeframes: map[string]Timeframe{
			"1D": {
				LastClose: 100,
				Score:     61,
				TrendBias: "bullish",
				Indicators: ohlcv.IndicatorSnapshot{
					RSI14:              55,
					MACDHistogram:      0.3,
					ChaikinMoneyFlow20: 0.1,
					VolumeSMA20:        1_000_000,
				},
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 105},
				NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 95},
				TradePlan:         ohlcv.TradePlan{Direction: "long", EntryMin: 100, EntryMax: 102, StopLoss: 95, TakeProfit1: 115, TakeProfit2: 125},
				Liquidity:         professional.LiquidityProfile{AverageValueTraded20TRY: 200_000_000, CapacityTRYAt10PctADV: 20_000_000, DaysToExit1MTRY: 0.5, VolumeVsAverage20: 1.1},
				Backtest:          professional.BacktestResult{BacktestSafe: true, Trades: 35, OutOfSampleTrades: 10, Expectancy: 0.02, OutOfSampleReturn: 0.01},
			},
		},
	})

	if !report.Computed {
		t.Fatalf("expected computed report")
	}
	if report.Decision != "AL" && report.Decision != "ALIM_ADAYI" {
		t.Fatalf("decision = %s, report=%+v", report.Decision, report)
	}
	if len(report.Questions) < 16 {
		t.Fatalf("questions = %d, want full investor QA", len(report.Questions))
	}
	if report.Quality.Score <= 0 || report.Liquidity.Score <= 0 || report.ModelRisk.Score <= 0 {
		t.Fatalf("subscores missing: %+v %+v %+v", report.Quality, report.Liquidity, report.ModelRisk)
	}
}

func TestAnalyzeEquityBlocksBuyWhenFinalCloseIsNotVerified(t *testing.T) {
	input := strongEquityInput()
	input.PriceVerification = PriceVerification{
		Known:                 true,
		Status:                "stale_or_conflicting",
		ReadyForVerifiedClose: false,
		LatestTradingDate:     "2026-06-19",
		SelectedClose:         402.50,
		SelectedTradingDate:   "2026-06-19",
		BlockingReasons:       []string{"official_final_close_missing", "stale_price_source_present"},
		MissingFields:         []string{"official_final_close"},
	}
	report := Analyze(input)

	if report.Decision != "BEKLE" {
		t.Fatalf("decision = %s, want BEKLE", report.Decision)
	}
	if !strings.Contains(strings.ToLower(report.DecisionLabel), "kapanış/karar fiyatı") {
		t.Fatalf("decision label should explain decision-price gate: %q", report.DecisionLabel)
	}
	if report.Confidence > 64 {
		t.Fatalf("confidence = %.1f, want capped by price verification", report.Confidence)
	}
	if !strings.Contains(strings.ToLower(report.TopRisk), "kapanış") {
		t.Fatalf("top risk should mention close verification, got %q", report.TopRisk)
	}
	var buy ActionSignal
	for _, action := range report.ActionMatrix {
		if action.Action == "AL" {
			buy = action
			break
		}
	}
	if buy.CurrentSignal {
		t.Fatalf("buy signal should not be current: %+v", buy)
	}
	if buy.EntryMin != 0 || buy.StopLoss != 0 || buy.Target1 != 0 {
		t.Fatalf("verified close blocker should prevent active buy levels: %+v", buy)
	}
	blockerText := strings.ToLower(strings.Join(append(buy.Blockers, buy.Evidence...), " "))
	if !strings.Contains(blockerText, "resmi/final kapanış doğrulanmadı") {
		t.Fatalf("buy blocker should include price verification: %+v", buy)
	}
	foundCheck := false
	for _, check := range report.Checks {
		if check.Name == "verified_price_close" {
			foundCheck = true
			if check.Status != "fail" {
				t.Fatalf("price check status = %s, want fail", check.Status)
			}
		}
	}
	if !foundCheck {
		t.Fatalf("missing verified_price_close check: %+v", report.Checks)
	}
}

func TestAnalyzeEquityMissingIntrinsicValueDoesNotActivateSell(t *testing.T) {
	input := strongEquityInput()
	input.Professional.ValueInvesting = value.Report{
		Computed:      false,
		Decision:      "INSUFFICIENT_DATA",
		DecisionLabel: "Kanıt kapısı geçmedi",
	}
	input.Professional.Coverage = professional.CoverageReport{Score: 0}
	input.Professional.DataQuality = 0
	input.Professional.DataGovernance = professional.FinancialDataGovernance{}
	input.Professional.Valuation.NetIncomeTTM = 100
	input.Professional.Valuation.FreeCashFlowTTM = -20
	input.Professional.Valuation.Ratios = map[string]float64{"ROE": -0.05, "Net_Margin": -0.02, "NetDebt_Eq": 3}
	input.Timeframes["1D"] = Timeframe{
		LastClose: 5625,
		Score:     30,
		TrendBias: "bearish",
		Indicators: ohlcv.IndicatorSnapshot{
			RSI14:              42,
			MACDHistogram:      10,
			ChaikinMoneyFlow20: -0.40,
		},
		NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 5617.5},
		NearestResistance: &ohlcv.SupportResistanceLevel{Price: 5963.75},
		TradePlan:         ohlcv.TradePlan{Direction: "neutral", Rejected: true, RejectReason: "Aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor"},
		Liquidity:         professional.LiquidityProfile{AverageValueTraded20TRY: 200_000_000, CapacityTRYAt10PctADV: 20_000_000, DaysToExit1MTRY: 0.5},
		Backtest:          professional.BacktestResult{BacktestSafe: true, Trades: 11, OutOfSampleTrades: 2, Expectancy: -0.01, OutOfSampleReturn: -0.02},
		TechnicalGate:     professional.TechnicalSignalGate{Status: "fail", Actionable: false, Label: "Aktif işlem sinyali yok"},
	}
	report := Analyze(input)

	if report.Decision != "BEKLE" {
		t.Fatalf("decision = %s, want BEKLE", report.Decision)
	}
	if !strings.Contains(strings.ToLower(report.DecisionLabel), "kanıtı eksik") {
		t.Fatalf("decision label should explain missing evidence: %q", report.DecisionLabel)
	}
	if report.Quality.Score >= 40 || report.ModelRisk.Score >= 40 {
		t.Fatalf("test fixture must exercise low quality/model risk path, got quality=%.1f model=%.1f", report.Quality.Score, report.ModelRisk.Score)
	}
	for _, action := range report.ActionMatrix {
		if action.Action == "SAT_RISK_AZALT" && action.CurrentSignal {
			t.Fatalf("missing intrinsic value should not activate sell/risk reduce: %+v", action)
		}
	}
}

func TestAnalyzeCryptoDoesNotPretendDCFValue(t *testing.T) {
	report := Analyze(Input{
		Symbol:       "CHZUSDT",
		Currency:     "USDT",
		AssetType:    ohlcv.AssetTypeCrypto,
		OverallScore: 35,
		OverallBias:  "bearish",
		Professional: professional.Report{
			Coverage: professional.CoverageReport{
				Score:     50,
				Available: []string{"tradingview_ohlcv_price_volume", "technical_indicators"},
				Missing:   []string{"onchain_mvrv_nupl_sopr_realized_cap"},
			},
			DataQuality: 50,
			Company:     professional.CompanyProfile{Sector: "Crypto Assets", Industry: "Digital Asset"},
			Market:      professional.MarketContext{BenchmarkAvailable: true},
			Peers:       professional.PeerComparison{ValuationSignal: "not_applicable"},
			Scenarios:   []professional.Scenario{{Name: "bear", PriceTarget: 0.02, ReturnPct: -20}, {Name: "base", PriceTarget: 0.025, ReturnPct: 0}, {Name: "bull", PriceTarget: 0.03, ReturnPct: 20}},
		},
		Timeframes: map[string]Timeframe{"1D": {LastClose: 0.025, Score: 35, TrendBias: "bearish", TradePlan: ohlcv.TradePlan{Rejected: true}, Backtest: professional.BacktestResult{BacktestSafe: true, Trades: 12}}},
	})

	if report.Decision != "BEKLE" {
		t.Fatalf("decision = %s", report.Decision)
	}
	found := false
	for _, item := range report.Questions {
		if item.Question == "Bugünkü fiyat teknik olarak avantajlı mı?" {
			found = true
			if item.Status != "limited" || item.Answer == "" {
				t.Fatalf("crypto valuation answer wrong: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("missing crypto technical price question")
	}
}

func TestAnalyzeBankFAQSuppressesIndustrialDebtAndCashMetrics(t *testing.T) {
	report := Analyze(Input{
		Symbol:       "ISCTR",
		Currency:     "TRY",
		AssetType:    ohlcv.AssetTypeEquity,
		OverallScore: 58,
		OverallBias:  "neutral",
		Professional: professional.Report{
			Coverage:         professional.CoverageReport{Score: 90, Available: []string{"financials", "ohlcv"}},
			DataQuality:      90,
			Company:          professional.CompanyProfile{Sector: "MALİ KURULUŞLAR", Industry: "BANKALAR", PeerGroup: "bist_bankalar"},
			SectorFinancials: professional.SectorFinancialAnalysis{Profile: "bank", ProfileLabel: "Banka"},
			Market:           professional.MarketContext{BenchmarkSymbol: "XU100", BenchmarkAvailable: true, RelativeStrength20: 0.02, Beta60: 1.1},
			Peers:            professional.PeerComparison{PeerCount: 5, ValuationSignal: "discount", Medians: map[string]float64{"PE": 5, "PB": 1}},
			Valuation: professional.ValuationAnalysis{
				SectorModel:     "bank_equity_model",
				NetIncomeTTM:    98_000_000_000,
				FreeCashFlowTTM: 0,
				NetDebt:         -902_000_000_000,
				Ratios:          map[string]float64{"PE": 3.9, "PB": 0.8, "ROE": 0.19, "ROA": 0.017, "NetDebt_Eq": 0},
				FairValue:       professional.FairValueRange{Base: 19.5, Confidence: 0.8},
			},
			Disclosure:     professional.DisclosureReview{KAPCompanyCardAvailable: true, RecentDisclosureStatus: "available"},
			DataGovernance: professional.FinancialDataGovernance{FinanciallyConsistent: true, ProductionReady: true, BacktestSafe: true},
			ValueInvesting: value.Report{
				Computed:     true,
				CurrentPrice: 15.2,
				SectorModel:  value.SectorModelReport{Model: "bank_equity_model", Label: "Banka / özsermaye getirisi modeli"},
				IntrinsicValue: value.IntrinsicValueReport{
					Computed: true,
					Base:     19.5,
				},
				MarginOfSafety: value.MarginOfSafetyReport{Computed: true, BasePct: 22, RequiredPct: 25},
				Confidence:     80,
			},
		},
		Timeframes: map[string]Timeframe{
			"1D": {
				LastClose: 15.2,
				Score:     55,
				TrendBias: "neutral",
				Liquidity: professional.LiquidityProfile{AverageValueTraded20TRY: 1_000_000_000, CapacityTRYAt10PctADV: 100_000_000},
				Backtest:  professional.BacktestResult{BacktestSafe: true, Trades: 20, OutOfSampleTrades: 8},
			},
		},
	})

	allAnswers := strings.ToLower(joinQuestionAnswers(report.Questions))
	for _, forbidden := range []string{
		"fcf/net kâr oranı",
		"net borç -902",
		"net borç/özsermaye 0.00",
		"hbdd",
		"hbk",
	} {
		if strings.Contains(allAnswers, forbidden) {
			t.Fatalf("bank FAQ leaked industrial metric %q: %s", forbidden, allAnswers)
		}
	}
	if !strings.Contains(allAnswers, "bankada klasik serbest nakit akımı") {
		t.Fatalf("bank cash-flow FAQ did not explain bank-specific metric scope: %s", allAnswers)
	}
	if !strings.Contains(allAnswers, "sanayi tipi net borç") {
		t.Fatalf("bank debt FAQ did not explain net debt is not applicable: %s", allAnswers)
	}
}

func TestAnalyzeBankMissingCoreMetricsProducesWaitDecision(t *testing.T) {
	report := Analyze(bankMissingCoreMetricsInput(nil))

	if report.Decision != "BEKLE" {
		t.Fatalf("decision = %s, want BEKLE", report.Decision)
	}
	if report.Confidence > 70 {
		t.Fatalf("confidence = %.1f, want capped at 70", report.Confidence)
	}
	if report.ModelRisk.Score > 64 {
		t.Fatalf("model risk = %.1f, want capped by bank missing metrics", report.ModelRisk.Score)
	}
	if !strings.Contains(strings.Join(report.ModelRisk.PrimaryLimitations, " "), "bank_regulatory_metrics_missing") {
		t.Fatalf("missing bank limitation: %+v", report.ModelRisk.PrimaryLimitations)
	}

	var buy ActionSignal
	for _, action := range report.ActionMatrix {
		if action.Action == "AL" {
			buy = action
			break
		}
	}
	if buy.EntryMin != 0 || buy.StopLoss != 0 || buy.Target1 != 0 {
		t.Fatalf("non-actionable technical plan leaked active levels: %+v", buy)
	}
	if strings.Contains(strings.ToLower(buy.Trigger), "giriş bölgesi") {
		t.Fatalf("non-actionable trigger should not expose entry zone as buy condition: %q", buy.Trigger)
	}
	if !strings.Contains(strings.ToLower(strings.Join(append(buy.Blockers, buy.Evidence...), " ")), "paper-trade") {
		t.Fatalf("paper-trade warning missing: %+v", buy)
	}
	if !strings.Contains(strings.ToLower(report.Scenario.PositioningAnswer), "paper-trade") {
		t.Fatalf("positioning answer should be paper-trade, got %q", report.Scenario.PositioningAnswer)
	}
	for _, question := range report.Questions {
		if question.Question == "Bugünkü fiyat gerçek değere göre ucuz mu?" && question.Confidence > 35 {
			t.Fatalf("suppressed value question confidence = %.1f, want <=35: %+v", question.Confidence, question)
		}
	}
}

func TestAnalyzeREITDoesNotUseBankMetricDecision(t *testing.T) {
	report := Analyze(Input{
		Symbol:       "ALGYO",
		Currency:     "TRY",
		AssetType:    ohlcv.AssetTypeEquity,
		OverallScore: 47,
		OverallBias:  "neutral",
		Professional: professional.Report{
			Coverage:    professional.CoverageReport{Score: 100, Available: []string{"financials", "kap_pdf_ingest", "kap_asset_inventory"}},
			DataQuality: 80,
			Company: professional.CompanyProfile{
				Sector:    "MALİ KURULUŞLAR",
				Industry:  "GAYRİMENKUL YATIRIM ORTAKLIKLARI",
				PeerGroup: "bist_gayrimenkulyatirimortakliklari",
			},
			SectorFinancials: professional.SectorFinancialAnalysis{
				Profile:      "reit_nav",
				ProfileLabel: "Gayrimenkul yatirim ortakligi",
				Score:        51,
				Warnings:     []string{"reit_reports_need_appraisal_based_nav_and_portfolio_occupancy_for_full_view"},
			},
			Valuation: professional.ValuationAnalysis{
				SectorModel: "reit_nav_proxy_model",
				Ratios:      map[string]float64{"PB": 0.49, "ROE": -0.13, "ROA": -0.08, "NetDebt_Eq": 0.25},
				Flags:       []string{"reit_sector_requires_nav_and_portfolio_appraisal_model"},
			},
			ValueInvesting: value.Report{
				Computed:   false,
				Confidence: 79,
				SectorModel: value.SectorModelReport{
					Model: "gyo_nav_proxy",
					Label: "GYO / net aktif değer proxy modeli",
				},
			},
		},
		Timeframes: map[string]Timeframe{
			"1D": {
				LastClose: 4.21,
				Score:     34,
				TrendBias: "bearish",
				Backtest:  professional.BacktestResult{BacktestSafe: true, Trades: 24, OutOfSampleTrades: 8},
			},
		},
	})

	allText := strings.ToLower(report.DecisionLabel + " " + report.OneLineAnswer + " " + strings.Join(report.ModelRisk.PrimaryLimitations, " "))
	for _, forbidden := range []string{"banka", "syr", "cet1", "npl", "nim", "lcr"} {
		if strings.Contains(allText, forbidden) {
			t.Fatalf("REIT report leaked bank-specific decision text %q: %s", forbidden, allText)
		}
	}
	if report.Decision == "TAKIP" && strings.Contains(strings.ToLower(report.DecisionLabel), "ana metrik") {
		t.Fatalf("REIT decision should not be limited by bank core metrics: %q", report.DecisionLabel)
	}
}

func TestAnalyzeBankDividendCorporateActionIsNotReportedAsZero(t *testing.T) {
	announcement := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	amount := 0.54
	input := bankMissingCoreMetricsInput([]corporateactions.Action{
		{
			Type:             corporateactions.TypeDividend,
			Status:           corporateactions.StatusReview,
			AnnouncementDate: &announcement,
			Title:            "Kar payı dağıtım işlemleri",
			CashAmount:       &amount,
			Currency:         "TRY",
			Confidence:       0.60,
			ReviewRequired:   true,
			Source:           "kap_pdf_document_intelligence",
		},
	})
	report := Analyze(input)
	answers := strings.ToLower(joinQuestionAnswers(report.Questions))
	if !strings.Contains(answers, "temettü kaydı") {
		t.Fatalf("dividend corporate action was not surfaced: %s", answers)
	}
	for _, forbidden := range []string{
		"anlamlı nakit temettü akışı görünmüyor",
		"temettü ödeme sıklığı hesaplanamadı",
	} {
		if strings.Contains(answers, forbidden) {
			t.Fatalf("dividend answer still reports missing/zero flow %q: %s", forbidden, answers)
		}
	}
}

func TestShareholderDividendTextExcludesEmployeeProfitShareProvision(t *testing.T) {
	if shareholderDividendText("çalışanlara dağıtılacak kar payı için ayrılan karşılık") {
		t.Fatalf("employee profit-share provision should not be treated as shareholder dividend")
	}
	if !shareholderDividendText("Kar payı dağıtım işlemleri") {
		t.Fatalf("shareholder dividend distribution text should be detected")
	}
}

func bankMissingCoreMetricsInput(actions []corporateactions.Action) Input {
	return Input{
		Symbol:       "ISCTR",
		Currency:     "TRY",
		AssetType:    ohlcv.AssetTypeEquity,
		OverallScore: 82,
		OverallBias:  "bullish",
		Professional: professional.Report{
			Coverage:         professional.CoverageReport{Score: 100, Available: []string{"financials", "ohlcv", "kap"}},
			DataQuality:      100,
			Company:          professional.CompanyProfile{Sector: "MALİ KURULUŞLAR", Industry: "BANKALAR", PeerGroup: "bist_bankalar"},
			SectorFinancials: professional.SectorFinancialAnalysis{Profile: "bank", ProfileLabel: "Banka", Score: 61, Warnings: []string{"bank_reports_need_npl_capital_adequacy_regulatory_ratios"}},
			Market:           professional.MarketContext{BenchmarkSymbol: "XU100", BenchmarkAvailable: true, RelativeStrength20: 0.04, RelativeStrength60: 0.03, Beta60: 1.1},
			Peers:            professional.PeerComparison{PeerCount: 8, ValuationSignal: "discount", Medians: map[string]float64{"PE": 5, "PB": 1}},
			Valuation: professional.ValuationAnalysis{
				SectorModel:  "bank_equity_model",
				MarketCap:    380_000_000_000,
				NetIncomeTTM: 98_000_000_000,
				Equity:       420_000_000_000,
				Ratios:       map[string]float64{"PE": 3.9, "PB": 0.8, "ROE": 0.19, "ROA": 0.017},
				FairValue:    professional.FairValueRange{Base: 19.5, Confidence: 0.95, Drivers: []string{"peer_median_pb", "peer_median_pe"}},
				Flags:        []string{"bank_sector_requires_regulatory_capital_and_asset_quality_model"},
			},
			Disclosure:       professional.DisclosureReview{KAPCompanyCardAvailable: true, RecentDisclosureStatus: "available", RecentDisclosureCount: 5},
			DataGovernance:   professional.FinancialDataGovernance{FinanciallyConsistent: true, ProductionReady: false, BacktestSafe: true},
			CorporateActions: corporateactions.ActionSet{Actions: actions},
			Scenarios:        []professional.Scenario{{Name: "bear", PriceTarget: 14, ReturnPct: -8}, {Name: "base", PriceTarget: 19.5, ReturnPct: 28}, {Name: "bull", PriceTarget: 25, ReturnPct: 64}},
			ValueInvesting: value.Report{
				Computed:     true,
				CurrentPrice: 15.2,
				SectorModel:  value.SectorModelReport{Model: "bank_equity_model", Label: "Banka / özsermaye getirisi modeli"},
				IntrinsicValue: value.IntrinsicValueReport{
					Computed:   true,
					Base:       19.5,
					Confidence: 95,
				},
				MarginOfSafety: value.MarginOfSafetyReport{Computed: true, BasePct: 28, RequiredPct: 25},
				CapitalAllocation: value.CapitalAllocationReport{
					Score:         100,
					Dilution5YPct: 455,
				},
				Moat:         value.MoatReport{Score: 85},
				ValueScore:   90,
				QualityScore: 90,
				Confidence:   95,
				DataQuality:  95,
			},
		},
		Timeframes: map[string]Timeframe{
			"1D": {
				LastClose: 15.2,
				Score:     66,
				TrendBias: "bullish",
				Indicators: ohlcv.IndicatorSnapshot{
					MACDHistogram:      0.1,
					ChaikinMoneyFlow20: 0.1,
				},
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 16.4},
				NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 14.8},
				TradePlan:         ohlcv.TradePlan{Direction: "long", EntryMin: 15.1, EntryMax: 15.4, StopLoss: 14.8, TakeProfit1: 16.4, TakeProfit2: 17.4, RiskRewardRatio: 2.2},
				Liquidity:         professional.LiquidityProfile{AverageValueTraded20TRY: 6_700_000_000, CapacityTRYAt10PctADV: 670_000_000, DaysToExit1MTRY: 1},
				Backtest:          professional.BacktestResult{BacktestSafe: true, Trades: 5, OutOfSampleTrades: 2, OutOfSampleReturn: -0.02},
				TechnicalGate:     professional.TechnicalSignalGate{Status: "limited", Actionable: false, Label: "İzleme / paper-trade adayı"},
			},
		},
	}
}

func strongEquityInput() Input {
	return Input{
		Symbol:       "TEST",
		Currency:     "TRY",
		AssetType:    ohlcv.AssetTypeEquity,
		OverallScore: 62,
		OverallBias:  "neutral",
		Professional: professional.Report{
			Coverage:    professional.CoverageReport{Score: 90, Available: []string{"financials", "ohlcv"}},
			DataQuality: 88,
			Company:     professional.CompanyProfile{Sector: "Sanayi", Industry: "Savunma", PeerGroup: "bist_defense"},
			Market:      professional.MarketContext{BenchmarkSymbol: "XU100", BenchmarkAvailable: true, RelativeStrength20: 0.04, RelativeStrength60: 0.03, Beta60: 0.9},
			Peers:       professional.PeerComparison{PeerCount: 5, ValuationSignal: "discount"},
			Valuation: professional.ValuationAnalysis{
				NetIncomeTTM:     100,
				FreeCashFlowTTM:  90,
				NetDebt:          20,
				Ratios:           map[string]float64{"ROE": 0.20, "Net_Margin": 0.15, "NetDebt_Eq": 0.2},
				FairValue:        professional.FairValueRange{Base: 130, Confidence: 0.75},
				MarketCap:        900,
				Equity:           500,
				EnterpriseValue:  920,
				OperatingCashTTM: 110,
			},
			Disclosure:     professional.DisclosureReview{KAPCompanyCardAvailable: true, RecentDisclosureStatus: "available", RecentDisclosureCount: 3},
			DataGovernance: professional.FinancialDataGovernance{FinanciallyConsistent: true, LineageEvents: 4, ProductionReady: true, BacktestSafe: true},
			Scenarios:      []professional.Scenario{{Name: "bear", PriceTarget: 80, ReturnPct: -20}, {Name: "base", PriceTarget: 120, ReturnPct: 20}, {Name: "bull", PriceTarget: 150, ReturnPct: 50}},
			ValueInvesting: value.Report{
				Computed:     true,
				CurrentPrice: 100,
				IntrinsicValue: value.IntrinsicValueReport{
					Computed:   true,
					Base:       140,
					Confidence: 80,
				},
				MarginOfSafety: value.MarginOfSafetyReport{Computed: true, BasePct: 28.5, RequiredPct: 25},
				CapitalAllocation: value.CapitalAllocationReport{
					Score: 78,
				},
				Moat:         value.MoatReport{Score: 82},
				ValueScore:   80,
				QualityScore: 78,
				Confidence:   80,
			},
		},
		Timeframes: map[string]Timeframe{
			"1D": {
				LastClose: 100,
				Score:     61,
				TrendBias: "bullish",
				Indicators: ohlcv.IndicatorSnapshot{
					RSI14:              55,
					MACDHistogram:      0.3,
					ChaikinMoneyFlow20: 0.1,
					VolumeSMA20:        1_000_000,
				},
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 105},
				NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 95},
				TradePlan:         ohlcv.TradePlan{Direction: "long", EntryMin: 100, EntryMax: 102, StopLoss: 95, TakeProfit1: 115, TakeProfit2: 125},
				Liquidity:         professional.LiquidityProfile{AverageValueTraded20TRY: 200_000_000, CapacityTRYAt10PctADV: 20_000_000, DaysToExit1MTRY: 0.5, VolumeVsAverage20: 1.1},
				Backtest:          professional.BacktestResult{BacktestSafe: true, Trades: 35, OutOfSampleTrades: 10, Expectancy: 0.02, OutOfSampleReturn: 0.01},
			},
		},
	}
}

func joinQuestionAnswers(items []QuestionAnswer) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Question+" "+item.Answer)
	}
	return strings.Join(parts, " ")
}

func TestPortfolioTransactionUseDoesNotAuthorizeStandaloneTrade(t *testing.T) {
	status, answer := personaTransactionUse(PersonaView{
		Name:   "Kurumsal Portföy Standardı",
		Status: "pass",
	})
	if status != "limited" {
		t.Fatalf("status = %s, want limited", status)
	}
	lower := strings.ToLower(answer)
	if !strings.Contains(lower, "ön eleme") || !strings.Contains(lower, "tek başına doğrudan al/sat emri için hayır") {
		t.Fatalf("answer does not distinguish pre-screen from trade authorization: %s", answer)
	}
}

func TestAnalyzeCommodityUsesGoldMacroFramework(t *testing.T) {
	report := Analyze(Input{
		Symbol:       "XAUUSD",
		Currency:     "USD",
		AssetType:    ohlcv.AssetTypeCommodity,
		OverallScore: 48,
		OverallBias:  "neutral",
		Professional: professional.Report{
			Coverage: professional.CoverageReport{
				Score:     100,
				Available: []string{"tradingview_ohlcv_price_volume", "usd_index_dxy_real_yield_macro", "futures_cot_open_interest_positioning", "gold_etf_physical_flow"},
			},
			DataQuality: 92,
			Company:     professional.CompanyProfile{Sector: "Precious Metals", Industry: "Spot Gold", SectorSource: "asset_type_commodity", ClassificationConfidence: 1},
			Market:      professional.MarketContext{BenchmarkSymbol: "DXY / ABD reel faiz / COMEX COT / altin ETF akisi"},
			CommodityContext: professional.CommodityContextReport{
				Computed: true,
				Macro: professional.CommodityContextSection{
					Available: true,
					Score:     90,
				},
				FuturesPositioning: professional.CommodityContextSection{
					Available: true,
					Score:     88,
				},
				GoldETFPhysicalFlow: professional.CommodityContextSection{
					Available: true,
					Score:     84,
				},
				CentralBankGeopoliticalNews: professional.CommodityContextSection{
					Available: true,
					Score:     80,
				},
			},
			DataGovernance: professional.FinancialDataGovernance{BacktestSafe: true, ProductionReady: true, UniverseSourceAvailable: true},
		},
		Timeframes: map[string]Timeframe{
			"1D": {
				LastClose:      4333,
				Score:          44,
				TrendBias:      "neutral",
				NearestSupport: &ohlcv.SupportResistanceLevel{Price: 4274},
				TradePlan:      ohlcv.TradePlan{Rejected: true, RejectReason: "short selling is not supported for this instrument"},
				Liquidity:      professional.LiquidityProfile{AverageValueTraded20TRY: 500_000_000, CapacityTRYAt10PctADV: 50_000_000, DaysToExit1MTRY: 0.5},
				Backtest:       professional.BacktestResult{BacktestSafe: true, Trades: 35, OutOfSampleTrades: 12, Expectancy: 0.01, OutOfSampleReturn: 0.01},
			},
		},
	})

	if strings.Contains(report.Macro.Benchmark, "XU100") {
		t.Fatalf("commodity macro benchmark should not use XU100: %+v", report.Macro)
	}
	if report.Macro.GDPImpact != "" || report.Macro.GDPInterpretation != "" || report.Macro.GDPScore != 0 {
		t.Fatalf("commodity macro should not use GDP fields: %+v", report.Macro)
	}
	if report.Macro.Score < 80 {
		t.Fatalf("commodity macro score should use commodity context, got %+v", report.Macro)
	}
	valueGate := report.InstitutionalViews.ValueInvesting
	if valueGate.Status != "not_applicable" {
		t.Fatalf("value gate status = %s, want not_applicable", valueGate.Status)
	}
	text := strings.ToLower(strings.Join(append(append([]string{}, valueGate.MustHave...), append(valueGate.Blockers, valueGate.TransactionUseAnswer)...), " "))
	for _, forbidden := range []string{"owner earnings", "moat", "pay sulandır"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("commodity value gate contains equity term %q: %s", forbidden, text)
		}
	}
	allText := strings.ToLower(report.Macro.Regime + " " + report.InstitutionalViews.Portfolio.Evidence[2].Label + " " + report.InstitutionalViews.Portfolio.Evidence[2].Value)
	if strings.Contains(allText, "xu100") || strings.Contains(allText, "gsyh") {
		t.Fatalf("commodity portfolio evidence should not include equity macro terms: %s", allText)
	}
}
