package professional

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/kapingest"
	tamacro "hissebot/internal/ta/macro"
	"hissebot/internal/ta/ohlcv"
	valuepkg "hissebot/internal/ta/value"
)

func TestAnalyzeTimeframeBuildsRiskLiquidityAndStats(t *testing.T) {
	candles := testCandles(140)
	report := AnalyzeTimeframe(TimeframeInput{
		Timeframe:  "1D",
		Candles:    candles,
		LastClose:  candles[len(candles)-1].Close,
		LastVolume: candles[len(candles)-1].Volume,
		Indicators: ohlcv.IndicatorSnapshot{
			SMA20:              120,
			SMA50:              115,
			SMA200:             100,
			RSI14:              58,
			ATR14:              4,
			MACDHistogram:      1.2,
			ADX14:              27,
			StochasticK:        55,
			ROC12:              3,
			VolumeSMA20:        125000,
			ChaikinMoneyFlow20: 0.12,
			BollingerUpper:     130,
			BollingerMiddle:    122,
			BollingerLower:     114,
		},
		IndicatorSignals: []ohlcv.IndicatorResult{
			{Name: "SMA20", Category: "trend", Signal: "bullish", Value: 120, Confidence: 0.82, Computed: true, Source: "local", Evidence: []string{"close above sma20"}},
			{Name: "Relative Strength", Category: "market", Signal: "proxy_only", Confidence: 0.9, Computed: false, Source: "requires benchmark"},
		},
		Patterns: []ohlcv.PatternResult{
			{Name: "Bull Flag", Category: "continuation", Direction: "bullish", Confidence: 0.72, StartIndex: 100, EndIndex: 130, VolumeConfirmed: true, Evidence: []string{"consolidation after impulse"}},
		},
		TradePlan: ohlcv.TradePlan{
			Direction:   "long",
			EntryMin:    120,
			EntryMax:    122,
			StopLoss:    115,
			TakeProfit1: 135,
		},
	}, Options{PortfolioValue: 100000, RiskPerTradePct: 1})
	if report.Liquidity.AverageValueTraded20TRY <= 0 {
		t.Fatalf("expected liquidity metrics, got %+v", report.Liquidity)
	}
	if report.PositionSizing.Quantity <= 0 {
		t.Fatalf("expected positive position size, got %+v", report.PositionSizing)
	}
	if report.Backtest.LookbackBars != len(candles) {
		t.Fatalf("expected backtest to use candles, got %+v", report.Backtest)
	}
	if report.Backtest.ExecutionModel != "signal_at_close_execute_next_open" || !report.Backtest.BacktestSafe {
		t.Fatalf("expected event-time backtest metadata, got %+v", report.Backtest)
	}
	if len(report.Backtest.CandidateStrategies) < 2 {
		t.Fatalf("expected multiple candidate strategies, got %+v", report.Backtest)
	}
	if report.SignalStats.CurrentRegime == "" {
		t.Fatalf("expected current regime, got %+v", report.SignalStats)
	}
	if report.Technical.Score.Total <= 0 {
		t.Fatalf("expected technical evidence score, got %+v", report.Technical)
	}
	if len(report.Technical.SelectedIndicators) != 1 {
		t.Fatalf("expected one selected computed indicator, got %+v", report.Technical.SelectedIndicators)
	}
	if len(report.Technical.SelectedPatterns) != 1 {
		t.Fatalf("expected one selected pattern, got %+v", report.Technical.SelectedPatterns)
	}
	if report.Technical.SignalCounts["proxy_only"] != 1 {
		t.Fatalf("expected proxy-only signals to be counted, got %+v", report.Technical.SignalCounts)
	}
}

func TestAnalyzeEquityNewsSentimentReadsRecentKAPAndNewsItems(t *testing.T) {
	root := t.TempDir()
	symbolDir := filepath.Join(root, "ASELS")
	if err := os.MkdirAll(symbolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{
  "source": "test",
  "ticker": "ASELS",
  "items": [
    {
      "title": "ASELS yeni ihracat sözleşmesi imzaladı",
      "summary": "Savunma elektroniği teslimat ve büyüme beklentisi pozitif.",
      "published_at": "2026-06-20T09:00:00Z",
      "source": "kap"
    },
    {
      "title": "ASELS hakkında dava bildirimi",
      "summary": "Olası ceza riski takip edilecek.",
      "published_at": "2026-06-18T09:00:00Z",
      "source": "kap"
    },
    {
      "title": "Eski bildirim",
      "summary": "Eski sozlesme haberi",
      "published_at": "2025-01-01T09:00:00Z",
      "source": "kap"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(symbolDir, "news_sentiment.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	kapRaw := `[
  {
    "title": "Yeni İş İlişkisi",
    "summary": "Sözleşme İmzalanması",
    "publish_date": "2026-06-19T09:00:00Z",
    "source": "kap_disclosures"
  }
]`
	if err := os.WriteFile(filepath.Join(symbolDir, "kap_disclosures.json"), []byte(kapRaw), 0o644); err != nil {
		t.Fatal(err)
	}

	report := analyzeEquityNewsSentiment(root, "ASELS", time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC))
	if !report.Computed || report.SourcePath == "" {
		t.Fatalf("expected computed news sentiment report: %+v", report)
	}
	if !strings.Contains(report.SourcePath, "news_sentiment.json") || !strings.Contains(report.SourcePath, "kap_disclosures.json") {
		t.Fatalf("expected merged sentiment sources: %+v", report)
	}
	if report.RecentItemCount != 3 || report.PositiveCount != 2 || report.NegativeCount != 1 {
		t.Fatalf("unexpected sentiment counts: %+v", report)
	}
	if report.Score <= 0 {
		t.Fatalf("expected positive weighted score from stronger contract/export wording: %+v", report)
	}
	if len(report.Items) != 3 {
		t.Fatalf("expected recent item details only: %+v", report.Items)
	}
}

func TestTechnicalVolumeConfirmationRequiresVolumeAndMoneyFlowAgreement(t *testing.T) {
	weak := TimeframeInput{
		LastVolume: 48,
		Indicators: ohlcv.IndicatorSnapshot{
			VolumeSMA20:        100,
			ChaikinMoneyFlow20: -0.18,
			MFI14:              63,
		},
	}
	if technicalVolumeConfirmed(weak, "bullish", nil) {
		t.Fatalf("low volume and negative CMF must not confirm bullish volume")
	}

	confirmed := weak
	confirmed.LastVolume = 120
	confirmed.Indicators.ChaikinMoneyFlow20 = 0.06
	if !technicalVolumeConfirmed(confirmed, "bullish", nil) {
		t.Fatalf("expanded volume with positive CMF should confirm bullish volume")
	}
}

func TestTechnicalScoreDoesNotRewardMissingPatternsOrNegativeMoneyFlow(t *testing.T) {
	score := technicalScore(TimeframeInput{
		LastClose:  100,
		LastVolume: 48,
		Indicators: ohlcv.IndicatorSnapshot{
			VolumeSMA20:        100,
			ChaikinMoneyFlow20: -0.18,
			MFI14:              63,
		},
	})
	if score.Volume != 0 {
		t.Fatalf("negative money flow with low volume should not receive volume score, got %.2f", score.Volume)
	}
	if score.Pattern != 0 {
		t.Fatalf("missing confirmed patterns should not receive pattern score, got %.2f", score.Pattern)
	}
}

func TestConfirmingTechnicalIndicatorsIgnoresMarketStructureProxies(t *testing.T) {
	got := confirmingTechnicalIndicators([]TechnicalIndicator{
		{Name: "BPR Indicator", Category: "smart_money", Group: "smart_money_ict_indikatorleri", Signal: "bullish", Confidence: 0.82, Source: "snapshot.market_structure"},
		{Name: "MACD", Category: "momentum", Group: "momentum_oscillator_indikatorleri", Signal: "bullish", Confidence: 0.80, Source: "snapshot.macd"},
		{Name: "EMA20", Category: "trend", Group: "trend_indikatorleri", Signal: "bullish", Confidence: 0.72, Source: "snapshot.ema20"},
	}, "bullish")
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "BPR") {
		t.Fatalf("market-structure proxy must not count as actionable indicator confirmation: %+v", got)
	}
	if !strings.Contains(joined, "MACD") || !strings.Contains(joined, "EMA20") {
		t.Fatalf("real computed indicators should remain confirmations: %+v", got)
	}
}

func TestLoadMarketMicrostructureReadsSymbolMarketWSFiles(t *testing.T) {
	equitiesDir := filepath.Join(t.TempDir(), "equities")
	marketDir := filepath.Join(equitiesDir, "TEST", "market_ws")
	writeMicrostructureJSON(t, filepath.Join(marketDir, "live_symbol_snapshot.json"), `[
		{"symbol":"TEST","last":10.00,"bid":10.00,"ask":10.01,"high":10.40,"low":9.80,"volume":1200000,"updated_at":"2026-06-20T01:25:56Z"}
	]`)
	writeMicrostructureJSON(t, filepath.Join(marketDir, "order_book.json"), `[
		{"updated_at":"2026-06-20T01:26:10Z","data":{"bids":[{"level":0,"price":10.00,"volume":1000,"lot":10},{"level":1,"price":9.99,"volume":500,"lot":5}],"asks":[{"level":0,"price":10.01,"volume":800,"lot":8},{"level":1,"price":10.02,"volume":300,"lot":3}]}}
	]`)
	writeMicrostructureJSON(t, filepath.Join(marketDir, "kdm2_data.json"), `[
		{"updated_at":"2026-06-20T01:26:10Z","data":{"depth_levels":[{"price":10.00,"lots":100,"volume_percent":60,"buy_percent":65,"sell_percent":35},{"price":10.01,"lots":80,"volume_percent":40,"buy_percent":35,"sell_percent":65}]}}
	]`)
	writeMicrostructureJSON(t, filepath.Join(marketDir, "akd_data.json"), `[
		{"updated_at":"2026-06-20T01:26:10Z","data":{"data":{"results":[{"brokerage":"ABC","net":{"size":120,"percentage":0.12,"cost":10.00},"total":{"size":1000,"percentage":0.40,"cost":10.02,"volume":10020}},{"brokerage":"XYZ","net":{"size":-90,"percentage":-0.09,"cost":10.01},"total":{"size":700,"percentage":0.28,"cost":10.03,"volume":7021}}]}}}
	]`)
	writeMicrostructureJSON(t, filepath.Join(marketDir, "custodian_data.json"), `[
		{"updated_at":"2026-06-20T01:26:10Z","data":{"data":{"date":"2026-06-19","results":[{"custodian":"CIY","value":1000,"percentage":0.42},{"custodian":"DBY","value":300,"percentage":0.12}]}}}
	]`)

	micro := loadMarketMicrostructure(equitiesDir, "TEST")
	if micro == nil || !micro.Computed || micro.Status != "pass" {
		t.Fatalf("expected pass microstructure, got %+v", micro)
	}
	if !micro.Liquidity.AutomaticOrderReady || !micro.Liquidity.DecisionUsable {
		t.Fatalf("expected decision/order ready microstructure, got %+v", micro.Liquidity)
	}
	if micro.OrderBook.BidLevels != 2 || micro.OrderBook.AskLevels != 2 || micro.OrderBook.SpreadBps <= 0 || micro.OrderBook.SpreadBps > marketMicrostructureMaxSpreadBps {
		t.Fatalf("unexpected order book context: %+v", micro.OrderBook)
	}
	if micro.Depth.Levels != 2 || !micro.BrokerageDistribution.Available || micro.BrokerageDistribution.ResultCount != 2 || !micro.Custody.Available || micro.Custody.ResultCount != 2 {
		t.Fatalf("missing parsed depth/akd/takas: %+v", micro)
	}

	live := liveSnapshotFromMarketMicrostructure("TEST", micro)
	if live == nil || !live.HasData() || live.Symbols["TEST"].Last != 10 {
		t.Fatalf("expected live snapshot fallback, got %+v", live)
	}
	ctx := buildMarketContext(SymbolInput{EquitiesDir: equitiesDir, Symbol: "TEST", AssetType: ohlcv.AssetTypeEquity}, Options{})
	if ctx.Microstructure == nil || !ctx.Microstructure.Computed {
		t.Fatalf("market context did not include microstructure: %+v", ctx.Microstructure)
	}
	if ctx.LiveSnapshot == nil || ctx.LiveSnapshot.Symbols["TEST"].Last != 10 {
		t.Fatalf("market context did not use symbol live snapshot fallback: %+v", ctx.LiveSnapshot)
	}
}

func TestLoadMarketMicrostructureUsesDatedScanFiles(t *testing.T) {
	equitiesDir := filepath.Join(t.TempDir(), "equities")
	marketDir := filepath.Join(equitiesDir, "TEST", "market_ws")
	dated := filepath.Join(marketDir, "2026-06-17")
	writeMicrostructureJSON(t, filepath.Join(marketDir, "live_symbol_snapshot.json"), `[
		{"symbol":"TEST","last":10,"volume":1000000,"updated_at":"2026-06-18T10:00:00Z"}
	]`)
	writeMicrostructureJSON(t, filepath.Join(dated, "akd_scan_data.json"), `[
		{"data":{"brokers":[{"broker_code":"ABC","net_size":1000,"net_percentage":20,"net_cost":9.9}]},"updated_at":"2026-06-17T18:00:00Z"}
	]`)
	writeMicrostructureJSON(t, filepath.Join(dated, "custody_scan_foreign_data.json"), `[
		{"data":{"foreign_brokers":[{"broker_code":"XYZ","lot_count":2000,"percentage":3}],"foreign_pct":3,"trade_date":"2026-06-17"},"updated_at":"2026-06-17T18:00:00Z"}
	]`)
	writeMicrostructureJSON(t, filepath.Join(dated, "custody_scan_institution_data.json"), `[
		{"data":{"broker_code":"FON","lot_count":3000,"percentage":5,"trade_date":"2026-06-17"},"updated_at":"2026-06-17T18:00:00Z"}
	]`)
	writeMicrostructureJSON(t, filepath.Join(dated, "custody_scan_anomaly_data.json"), `[
		{"data":{"broker_code":"ANM","lot_change":-500},"updated_at":"2026-06-17T18:00:00Z"}
	]`)
	writeMicrostructureJSON(t, filepath.Join(dated, "equilibrium_data.json"), `[
		{"data":{"equilibrium_price_or_last_lot":"10","equilibrium_match_quantity":"500","equilibrium_bid_remainder":"100","equilibrium_ask_remainder":"300"},"updated_at":"2026-06-17T18:00:00Z"}
	]`)

	micro := loadMarketMicrostructure(equitiesDir, "TEST")
	if !micro.BrokerageDistribution.Available || !micro.Custody.Available || !micro.Equilibrium.Available {
		t.Fatalf("dated scan data was ignored: %+v", micro)
	}
	if micro.Custody.ForeignShare != 3 || micro.Custody.InstitutionalShare != 5 || micro.Custody.AnomalyBroker != "ANM" {
		t.Fatalf("custody scan values missing: %+v", micro.Custody)
	}
	if !micro.Liquidity.DecisionUsable || micro.Liquidity.AutomaticOrderReady {
		t.Fatalf("scan data should support decisions but not automatic orders: %+v", micro.Liquidity)
	}
}

func TestTechnicalValidationFailureBlocksSignalGate(t *testing.T) {
	gate := buildTechnicalSignalGate(TimeframeInput{
		Timeframe:  "1D",
		LastClose:  100,
		LastVolume: 150,
		Indicators: ohlcv.IndicatorSnapshot{
			VolumeSMA20:        100,
			ChaikinMoneyFlow20: 0.12,
			MFI14:              65,
		},
		TradePlan: ohlcv.TradePlan{
			Direction:       "long",
			EntryMin:        100,
			EntryMax:        101,
			StopLoss:        96,
			TakeProfit1:     108,
			TakeProfit2:     112,
			RiskRewardRatio: 2.5,
		},
		TechnicalValidation: TechnicalValidationReport{
			Status:           "fail",
			Score:            35,
			GateEligible:     false,
			PatternConfirmed: 1,
			PatternDrawn:     0,
			PatternNotDrawn:  1,
			Blockers:         []string{"aktif formasyon grafikte işaretlenemedi: invalid_window"},
		},
	}, TechnicalScore{}, []TechnicalIndicator{
		{Name: "MACD", Category: "momentum", Group: "momentum", Signal: "bullish", Confidence: 0.82, Source: "snapshot.macd"},
		{Name: "EMA20", Category: "trend", Group: "trend", Signal: "bullish", Confidence: 0.74, Source: "snapshot.ema20"},
	}, []TechnicalPattern{
		{Name: "Bullish pattern", Direction: "bullish", Confidence: 0.72, VolumeConfirmed: true},
	}, BacktestResult{
		BacktestSafe:        true,
		Trades:              40,
		OutOfSampleTrades:   12,
		Expectancy:          0.03,
		OutOfSampleReturn:   0.03,
		LookaheadViolations: 0,
	}, SignalStats{
		SampleSize:           40,
		WinRate:              0.58,
		AverageForwardReturn: 0.02,
	}, PriceAdjustmentReview{BacktestSafe: true})

	if gate.Actionable || gate.Status != "fail" {
		t.Fatalf("technical validation failure must block active signal: %+v", gate)
	}
	if !strings.Contains(strings.Join(gate.Blockers, " "), "grafikte işaretlenemedi") {
		t.Fatalf("validation blocker missing from gate: %+v", gate.Blockers)
	}
}

func TestValuationUsesTTMAndBalanceSheet(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	fin := financialFile{Data: map[string]financialField{
		"2OA": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(150)}}},
		"2N":  {Years: map[string][]*float64{"2026": {nil, nil, nil, value(1000)}}},
		"1BL": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(1500)}}},
		"1AA": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(100)}}},
		"2AA": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(50)}}},
		"2BA": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(30)}}},
		"3C": {
			Years: map[string][]*float64{
				"2025": {value(400), nil, nil, value(80)},
				"2026": {nil, nil, nil, value(120)},
			},
		},
		"3DF": {
			Years: map[string][]*float64{
				"2025": {value(100), nil, nil, value(20)},
				"2026": {nil, nil, nil, value(30)},
			},
		},
		"3L": {
			Years: map[string][]*float64{
				"2025": {value(50), nil, nil, value(10)},
				"2026": {nil, nil, nil, value(15)},
			},
		},
	}}
	valuation := buildValuation(fin, 10, latestPeriod(fin), "BIST Genel", "")
	if valuation.SalesTTM != 440 {
		t.Fatalf("expected sales TTM 440, got %.2f", valuation.SalesTTM)
	}
	if valuation.MarketCap != 1500 {
		t.Fatalf("expected market cap 1500, got %.2f", valuation.MarketCap)
	}
	if valuation.Ratios["PB"] != 1.5 {
		t.Fatalf("expected PB 1.5, got %.2f", valuation.Ratios["PB"])
	}
}

func TestBankPeerRatioOutlierFilterExcludesDistortingMultiples(t *testing.T) {
	if !peerRatioUsableForMedian("PE", 9.5, true) {
		t.Fatalf("expected normal bank PE to be usable")
	}
	if peerRatioUsableForMedian("PE", 22.5, true) {
		t.Fatalf("expected high bank PE outlier to be filtered")
	}
	if peerRatioUsableForMedian("PB", 6.3, true) {
		t.Fatalf("expected high bank PB outlier to be filtered")
	}
	if !peerRatioUsableForMedian("PB", 6.3, false) {
		t.Fatalf("non-bank PB should use generic median filter")
	}
}

func TestStrictEvidencePolicyBlocksBankValuationWithoutCoreMetrics(t *testing.T) {
	report := Report{
		Coverage: CoverageReport{Score: 100, Available: []string{"financials", "ohlcv", "kap"}},
		Company:  CompanyProfile{Sector: "MALİ KURULUŞLAR", Industry: "BANKALAR", ClassificationConfidence: 0.95},
		Peers:    PeerComparison{PeerCount: 5},
		Valuation: ValuationAnalysis{
			SectorModel: "bank_equity_model",
			Flags:       []string{"bank_sector_requires_regulatory_capital_and_asset_quality_model"},
		},
		SectorFinancials: SectorFinancialAnalysis{Profile: "bank", ProfileLabel: "Banka", Warnings: []string{"bank_reports_need_npl_capital_adequacy_regulatory_ratios"}},
		DataGovernance: FinancialDataGovernance{
			FinanciallyConsistent: true,
			BacktestSafe:          true,
			LineageEvents:         1,
		},
		KAPPDFIngest: KAPPDFIngestSummary{Computed: true, TotalDocuments: 50, AnalysisUsableCount: 40},
		ValueInvesting: valuepkg.Report{
			Computed:    true,
			DataQuality: 95,
			Confidence:  95,
			IntrinsicValue: valuepkg.IntrinsicValueReport{
				Computed: true,
				Base:     20,
			},
		},
	}

	policy := evaluateStrictEvidencePolicy(report)
	if policy.ValuationTargetsAllowed {
		t.Fatalf("bank valuation should be blocked without core metrics: %+v", policy)
	}
	if !containsString(policy.BlockingIssues, "bank_regulatory_metrics_missing") {
		t.Fatalf("missing bank blocking issue: %+v", policy.BlockingIssues)
	}
}

func TestBankRegulatoryMetricsFromKAPTablesSatisfyStrictEvidence(t *testing.T) {
	date := "2026-03-31"
	sectorFin := enrichBankRegulatoryMetricsFromKAP(SectorFinancialAnalysis{
		Applicable:   true,
		Profile:      "bank",
		ProfileLabel: "Banka",
		Warnings:     []string{"bank_reports_need_npl_capital_adequacy_and_regulatory_ratios_for_full_view"},
	}, nil, []kapingest.ExtractedFinancialTable{{
		Ticker:         "ISCTR",
		SourceFile:     "data/equities/ISCTR/kap/attachments/2026/report.pdf",
		DocumentDate:   &date,
		Period:         &date,
		Confidence:     0.95,
		ReviewRequired: false,
		Certification: kapingest.EvidenceCertification{
			Status:         kapingest.EvidenceStatusCertified,
			AnalysisUsable: true,
		},
		Rows: []kapingest.FinancialTableRow{
			{RowIndex: 1, Cells: []string{"Sermaye Yeterlilik Rasyosu", "%15,2"}},
			{RowIndex: 2, Cells: []string{"CET1", "%11,7"}},
			{RowIndex: 3, Cells: []string{"Takipteki Krediler Oranı", "%3,3"}},
			{RowIndex: 4, Cells: []string{"Karşılık Kapsam Oranı", "%72,0"}},
			{RowIndex: 5, Cells: []string{"Net Faiz Marjı", "%1,1"}},
			{RowIndex: 6, Cells: []string{"Likidite Karşılama Oranı", "%145,0"}},
			{RowIndex: 7, Cells: []string{"Krediler/Mevduat", "%75,3"}},
			{RowIndex: 8, Cells: []string{"Mevduat Maliyeti", "%3,2"}},
			{RowIndex: 9, Cells: []string{"Kredi-Mevduat Spread", "%4,1"}},
		},
	}})
	if !bankRegulatoryMetricsComplete(sectorFin) {
		t.Fatalf("expected complete bank regulatory metrics: %+v", sectorFin)
	}
	valuation := ValuationAnalysis{SectorModel: "bank_equity_model", Flags: []string{bankRegulatoryValuationFlag}}
	updateBankRegulatoryValuationFlag(&valuation, sectorFin)
	if containsString(valuation.Flags, bankRegulatoryValuationFlag) {
		t.Fatalf("expected missing regulatory flag removed: %+v", valuation.Flags)
	}
	report := bankStrictEvidenceTestReport()
	report.Valuation = valuation
	report.SectorFinancials = sectorFin
	policy := evaluateStrictEvidencePolicy(report)
	if !policy.ValuationTargetsAllowed {
		t.Fatalf("bank valuation should pass with certified core metrics: %+v", policy)
	}
	if containsString(policy.BlockingIssues, "bank_regulatory_metrics_missing") {
		t.Fatalf("unexpected bank regulatory block: %+v", policy.BlockingIssues)
	}
}

func TestBankRegulatoryMetricsReviewRequiredDoNotSatisfyStrictEvidence(t *testing.T) {
	date := "2026-03-31"
	sectorFin := enrichBankRegulatoryMetricsFromKAP(SectorFinancialAnalysis{
		Applicable:   true,
		Profile:      "bank",
		ProfileLabel: "Banka",
	}, nil, []kapingest.ExtractedFinancialTable{{
		Ticker:         "ISCTR",
		SourceFile:     "data/equities/ISCTR/kap/attachments/2026/report.pdf",
		DocumentDate:   &date,
		Period:         &date,
		Confidence:     0.95,
		ReviewRequired: true,
		Certification: kapingest.EvidenceCertification{
			Status:              kapingest.EvidenceStatusReviewRequired,
			AnalysisUsable:      false,
			RequiresHumanReview: true,
		},
		Rows: []kapingest.FinancialTableRow{
			{RowIndex: 1, Cells: []string{"Sermaye Yeterlilik Rasyosu", "%15,2"}},
			{RowIndex: 2, Cells: []string{"CET1", "%11,7"}},
			{RowIndex: 3, Cells: []string{"Takipteki Krediler Oranı", "%3,3"}},
			{RowIndex: 4, Cells: []string{"Karşılık Kapsam Oranı", "%72,0"}},
			{RowIndex: 5, Cells: []string{"Net Faiz Marjı", "%1,1"}},
			{RowIndex: 6, Cells: []string{"Likidite Karşılama Oranı", "%145,0"}},
			{RowIndex: 7, Cells: []string{"Krediler/Mevduat", "%75,3"}},
		},
	}})
	if bankRegulatoryMetricsComplete(sectorFin) {
		t.Fatalf("review-required bank metrics must not satisfy strict evidence: %+v", sectorFin)
	}
	report := bankStrictEvidenceTestReport()
	report.SectorFinancials = sectorFin
	report.Valuation.Flags = []string{bankRegulatoryValuationFlag}
	policy := evaluateStrictEvidencePolicy(report)
	if policy.ValuationTargetsAllowed {
		t.Fatalf("bank valuation should remain blocked with review-required metrics: %+v", policy)
	}
}

func bankStrictEvidenceTestReport() Report {
	return Report{
		Coverage: CoverageReport{Score: 100, Available: []string{"financials", "ohlcv", "kap"}},
		Company:  CompanyProfile{Sector: "MALİ KURULUŞLAR", Industry: "BANKALAR", ClassificationConfidence: 0.95},
		Peers:    PeerComparison{PeerCount: 5},
		Valuation: ValuationAnalysis{
			SectorModel: "bank_equity_model",
		},
		DataGovernance: FinancialDataGovernance{
			FinanciallyConsistent: true,
			BacktestSafe:          true,
			LineageEvents:         1,
		},
		KAPPDFIngest: KAPPDFIngestSummary{Computed: true, TotalDocuments: 50, AnalysisUsableCount: 40},
		ValueInvesting: valuepkg.Report{
			Computed:    true,
			DataQuality: 95,
			Confidence:  95,
			IntrinsicValue: valuepkg.IntrinsicValueReport{
				Computed: true,
				Base:     20,
			},
		},
	}
}

func TestFinancialDataGovernanceRejectsMissingPublishDates(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	fin := financialFile{
		Ticker:   "TEST",
		Source:   "isyatirim",
		Currency: "TRY",
		Data: map[string]financialField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(10)}}},
		},
	}

	got := financialDataGovernance("", fin, latestPeriod(fin), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), true, domain.FinancialStatementVersionStore{}, false, normalizeOptions(Options{}))
	if got.BacktestSafe {
		t.Fatalf("expected missing publish dates to be unsafe, got %+v", got)
	}
	if got.AvailabilityStatus != "unsafe_missing_or_future_available_at" {
		t.Fatalf("status = %q", got.AvailabilityStatus)
	}
	if len(got.MissingPublishPeriods) != 1 || got.MissingPublishPeriods[0] != "2026-Q1" {
		t.Fatalf("missing publish periods = %#v", got.MissingPublishPeriods)
	}
	if len(got.MissingAvailableAtPeriods) != 1 || got.MissingAvailableAtPeriods[0] != "2026-Q1" {
		t.Fatalf("missing available-at periods = %#v", got.MissingAvailableAtPeriods)
	}
}

func TestFinancialDataGovernanceAllowsConservativeAvailableAt(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	availableAt := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	fin := financialFile{
		Ticker:   "TEST",
		Source:   "legacy_import",
		Currency: "TRY",
		Periods: map[string]domain.FinancialPeriod{
			"2026-Q1": {
				Key:                "2026-Q1",
				FiscalYear:         2026,
				FiscalQuarter:      1,
				PeriodEnd:          domain.FiscalPeriodEnd(2026, 1),
				AvailableAt:        &availableAt,
				AvailabilitySource: "local_json_import_at",
			},
		},
		Data: map[string]financialField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(10)}}},
		},
	}

	got := financialDataGovernance("", fin, latestPeriod(fin), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), true, domain.FinancialStatementVersionStore{}, false, normalizeOptions(Options{}))
	if !got.BacktestSafe {
		t.Fatalf("expected available-at period to be safe, got %+v", got)
	}
	if got.AvailabilityStatus != "lookahead_safe_conservative_available_at" {
		t.Fatalf("status = %q", got.AvailabilityStatus)
	}
	if got.PublishDateCoverage != 0 || got.AvailableAtCoverage != 1 {
		t.Fatalf("coverage publish=%.2f available=%.2f", got.PublishDateCoverage, got.AvailableAtCoverage)
	}
}

func TestFinancialDataGovernanceAcceptsPublishedPeriodsBeforeAsOf(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	publishDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	fin := financialFile{
		Ticker:   "TEST",
		Source:   "isyatirim",
		Currency: "TRY",
		Periods: map[string]domain.FinancialPeriod{
			"2026-Q1": {
				Key:           "2026-Q1",
				FiscalYear:    2026,
				FiscalQuarter: 1,
				PeriodEnd:     domain.FiscalPeriodEnd(2026, 1),
				PublishDate:   &publishDate,
			},
		},
		Data: map[string]financialField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(10)}}},
		},
	}

	got := financialDataGovernance("", fin, latestPeriod(fin), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), true, domain.FinancialStatementVersionStore{}, false, normalizeOptions(Options{}))
	if !got.BacktestSafe {
		t.Fatalf("expected published period to be safe, got %+v", got)
	}
	if got.AvailabilityStatus != "verified_publish_dates" {
		t.Fatalf("status = %q", got.AvailabilityStatus)
	}
	if got.PublishDateCoverage != 1 {
		t.Fatalf("coverage = %.2f", got.PublishDateCoverage)
	}
}

func TestFinancialDataGovernanceRejectsPublishDateBeforePeriodEnd(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	publishDate := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)
	fin := financialFile{
		Ticker:   "TEST",
		Source:   "isyatirim",
		Currency: "TRY",
		Periods: map[string]domain.FinancialPeriod{
			"2026-Q1": {
				Key:           "2026-Q1",
				FiscalYear:    2026,
				FiscalQuarter: 1,
				PeriodEnd:     domain.FiscalPeriodEnd(2026, 1),
				PublishDate:   &publishDate,
			},
		},
		Data: map[string]financialField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, value(10)}}},
		},
	}

	got := financialDataGovernance("", fin, latestPeriod(fin), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), true, domain.FinancialStatementVersionStore{}, false, normalizeOptions(Options{}))
	if got.BacktestSafe {
		t.Fatalf("publish date before period end must be unsafe, got %+v", got)
	}
	if got.AvailabilityStatus != "unsafe_missing_or_future_available_at" {
		t.Fatalf("status = %q", got.AvailabilityStatus)
	}
	if len(got.InvalidChronologyPeriods) != 1 || got.InvalidChronologyPeriods[0] != "2026-Q1" {
		t.Fatalf("invalid chronology periods = %#v", got.InvalidChronologyPeriods)
	}
	if !containsString(got.Warnings, "financial_period_chronology_invalid") || !containsString(got.Warnings, "publish_date_before_period_end") {
		t.Fatalf("expected chronology warnings, got %+v", got.Warnings)
	}
}

func TestFinancialDataGovernanceProductionModeQuarantinesUnverifiedPeriods(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	publishDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	availableAt := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	fin := financialFile{
		Ticker:   "TEST",
		Source:   "isyatirim",
		Currency: "TRY",
		Periods: map[string]domain.FinancialPeriod{
			"2026-Q1": {
				Key:           "2026-Q1",
				FiscalYear:    2026,
				FiscalQuarter: 1,
				PeriodEnd:     domain.FiscalPeriodEnd(2026, 1),
				PublishDate:   &publishDate,
			},
			"2026-Q2": {
				Key:                "2026-Q2",
				FiscalYear:         2026,
				FiscalQuarter:      2,
				PeriodEnd:          domain.FiscalPeriodEnd(2026, 2),
				AvailableAt:        &availableAt,
				AvailabilitySource: "fetched_at",
			},
		},
		Data: map[string]financialField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, value(12), value(10)}}},
		},
	}

	got := financialDataGovernance("", fin, latestPeriod(fin), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), true, domain.FinancialStatementVersionStore{}, false, normalizeOptions(Options{DataMode: "production"}))
	if !got.ProductionReady {
		t.Fatalf("expected production-ready verified subset, got %+v", got)
	}
	if got.AvailabilityStatus != "production_verified_subset_with_quarantine" {
		t.Fatalf("status = %q", got.AvailabilityStatus)
	}
	if got.ProductionEligiblePeriodCount != 1 || got.ProductionQuarantinedPeriodCount != 1 {
		t.Fatalf("production counts eligible=%d quarantined=%d", got.ProductionEligiblePeriodCount, got.ProductionQuarantinedPeriodCount)
	}
}

func TestProductionValuationPeriodUsesLatestVerifiedPublishDate(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	publishDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	availableAt := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	fin := financialFile{
		Periods: map[string]domain.FinancialPeriod{
			"2026-Q1": {
				Key:           "2026-Q1",
				FiscalYear:    2026,
				FiscalQuarter: 1,
				PeriodEnd:     domain.FiscalPeriodEnd(2026, 1),
				PublishDate:   &publishDate,
			},
			"2026-Q2": {
				Key:                "2026-Q2",
				FiscalYear:         2026,
				FiscalQuarter:      2,
				PeriodEnd:          domain.FiscalPeriodEnd(2026, 2),
				AvailableAt:        &availableAt,
				AvailabilitySource: "fetched_at",
			},
		},
		Data: map[string]financialField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, value(12), value(10)}}},
		},
	}

	got := valuationPeriod(fin, normalizeOptions(Options{DataMode: "production"}), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if got.Year != 2026 || got.Quarter != 1 {
		t.Fatalf("valuation period = %+v, want 2026 Q1", got)
	}
}

func TestValuationPeriodSkipsFuturePeriodEndsInResearchMode(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	fin := financialFile{Data: map[string]financialField{
		"2O": {
			Years: map[string][]*float64{
				"2025": {value(381), nil, nil, nil},
				"2026": {value(498), nil, nil, nil},
			},
		},
	}}

	got := valuationPeriod(fin, normalizeOptions(Options{DataMode: "research"}), time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC))
	if got.Year != 2025 || got.Quarter != 4 {
		t.Fatalf("valuation period = %+v, want 2025 Q4", got)
	}
	got = valuationPeriod(fin, normalizeOptions(Options{DataMode: "research"}), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if got.Year != 2026 || got.Quarter != 4 {
		t.Fatalf("valuation period = %+v, want 2026 Q4 after period end", got)
	}
}

func TestBankValuationSuppressesIndustrialMultiples(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	fin := financialFile{Data: map[string]financialField{
		"2OA": {DescTR: "16.1 Ödenmiş Sermaye", Years: map[string][]*float64{"2026": {value(100), nil, nil, nil}}},
		"2N":  {DescTR: "XIV. Satış amaçlı elde tutulan borçlar", Years: map[string][]*float64{"2026": {value(9999), nil, nil, nil}}},
		"2O":  {DescTR: "XVI. Özkaynaklar", Years: map[string][]*float64{"2026": {value(1000), nil, nil, nil}}},
		"1Z":  {DescTR: "Aktif Toplamı", Years: map[string][]*float64{"2026": {value(5000), nil, nil, nil}}},
		"3C":  {DescTR: "III. Net Faiz Geliri/Gideri", Years: map[string][]*float64{"2026": {value(300), nil, nil, nil}}},
		"3Z":  {DescTR: "XXIII. Net Dönem Karı/Zararı", Years: map[string][]*float64{"2026": {value(100), nil, nil, nil}}},
	}}
	valuation := buildValuation(fin, 20, latestPeriod(fin), "Banka", "")
	if valuation.SectorModel != "bank_equity_model" {
		t.Fatalf("sector model = %q", valuation.SectorModel)
	}
	if _, ok := valuation.Ratios["EV_EBITDA"]; ok {
		t.Fatalf("bank valuation must suppress EV_EBITDA, got %+v", valuation.Ratios)
	}
	if _, ok := valuation.Ratios["EV_Sales"]; ok {
		t.Fatalf("bank valuation must suppress EV_Sales, got %+v", valuation.Ratios)
	}
	if _, ok := valuation.Ratios["NetDebt_Eq"]; ok {
		t.Fatalf("bank valuation must suppress NetDebt_Eq, got %+v", valuation.Ratios)
	}
	if valuation.Ratios["PB"] == 0 || valuation.Ratios["PE"] == 0 {
		t.Fatalf("bank valuation should keep PB/PE, got %+v", valuation.Ratios)
	}
	quality := buildFinancialQualityBridge(valuation, SectorFinancialAnalysis{}, valuepkg.Report{}, false)
	for _, metric := range quality.Metrics {
		if metric.Name == "Net borç/özsermaye" {
			t.Fatalf("bank financial quality must not render net debt/equity: %+v", quality.Metrics)
		}
	}
}

func TestKAPBankIndustryUsesBankFinancialProfile(t *testing.T) {
	value := func(v float64) *float64 { return &v }
	fin := financialFile{
		FinancialGroup: "UFRS_K",
		Data: map[string]financialField{
			"2OA": {DescTR: "16.1 Ödenmiş Sermaye", Years: map[string][]*float64{"2026": {value(100), nil, nil, nil}}},
			"2N":  {DescTR: "XIV. Satış amaçlı elde tutulan borçlar", Years: map[string][]*float64{"2026": {value(9999), nil, nil, nil}}},
			"2O":  {DescTR: "XVI. Özkaynaklar", Years: map[string][]*float64{"2026": {value(1000), nil, nil, nil}}},
			"1Z":  {DescTR: "Aktif Toplamı", Years: map[string][]*float64{"2026": {value(5000), nil, nil, nil}}},
			"1AF": {DescTR: "VI. Krediler", Years: map[string][]*float64{"2026": {value(2500), nil, nil, nil}}},
			"2A":  {DescTR: "I. Mevduat", Years: map[string][]*float64{"2026": {value(3000), nil, nil, nil}}},
			"3C":  {DescTR: "III. Net Faiz Geliri/Gideri", Years: map[string][]*float64{"2026": {value(300), nil, nil, nil}}},
			"3Z":  {DescTR: "XXIII. Net Dönem Karı/Zararı", Years: map[string][]*float64{"2026": {value(100), nil, nil, nil}}},
		},
	}
	profile := CompanyProfile{Sector: "MALİ KURULUŞLAR", Industry: "BANKALAR"}
	context := financialContextText(profile, fin.FinancialGroup)
	valuation := buildValuation(fin, 20, latestPeriod(fin), context, "")
	if valuation.SectorModel != "bank_equity_model" {
		t.Fatalf("sector model = %q, want bank_equity_model", valuation.SectorModel)
	}
	sectorFin := analyzeSectorFinancials(fin, latestPeriod(fin), profile, valuation)
	if sectorFin.Profile != "bank" {
		t.Fatalf("sector financial profile = %q, want bank: %+v", sectorFin.Profile, sectorFin)
	}
	if sectorFin.FieldSchema != "bank_ufrs_k" {
		t.Fatalf("field schema = %q, want bank_ufrs_k", sectorFin.FieldSchema)
	}
	for _, metric := range sectorFin.Metrics {
		if metric.Name == "current_ratio" || metric.Name == "inventory_turnover" {
			t.Fatalf("bank profile must not compute industrial metric %+v", metric)
		}
		for _, field := range metric.SourceFields {
			if field == "2N" || field == "1BL" || field == "3L" {
				t.Fatalf("bank metric must not use industrial field code %q: %+v", field, metric)
			}
		}
	}
	if len(sectorFin.SuppressedMetric) == 0 {
		t.Fatalf("expected suppressed metrics for bank profile")
	}
}

func TestCryptoSectorFinancialsAreNotApplicable(t *testing.T) {
	report := analyzeCryptoSymbol(SymbolInput{
		Symbol:    "BTCUSDT",
		AssetType: ohlcv.AssetTypeCrypto,
		Currency:  "USDT",
		LastClose: 100000,
	}, normalizeOptions(Options{}))
	if report.SectorFinancials.Applicable {
		t.Fatalf("crypto sector financials should be disabled: %+v", report.SectorFinancials)
	}
	if report.SectorFinancials.Profile != "not_applicable" {
		t.Fatalf("profile = %q", report.SectorFinancials.Profile)
	}
}

func TestCryptoContextImprovesCoverageWhenAvailable(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	contextDir := filepath.Join(root, "crypto", "BTCUSDT")
	if err := os.MkdirAll(contextDir, 0o755); err != nil {
		t.Fatalf("mkdir context dir: %v", err)
	}
	contextJSON := []byte(`{
  "as_of": "2026-06-17T00:00:00Z",
  "onchain": {"available": true, "source": "glassnode", "score": 80, "summary": "MVRV and NUPL loaded"},
  "derivatives": {"available": true, "source": "binance_futures", "score": 75, "summary": "funding and OI loaded"},
  "exchange_flow": {"available": false},
  "news_sentiment": {"available": false}
}`)
	if err := os.WriteFile(filepath.Join(contextDir, "crypto_context.json"), contextJSON, 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}

	report := analyzeCryptoSymbol(SymbolInput{
		EquitiesDir: equitiesDir,
		Symbol:      "BTCUSDT",
		AssetType:   ohlcv.AssetTypeCrypto,
		Currency:    "USDT",
		LastClose:   100000,
	}, normalizeOptions(Options{}))

	if !report.CryptoContext.Computed {
		t.Fatalf("expected crypto context to be loaded: %+v", report.CryptoContext)
	}
	if !containsString(report.Coverage.Available, "onchain_mvrv_nupl_sopr_realized_cap") {
		t.Fatalf("onchain should be available: %+v", report.Coverage)
	}
	if !containsString(report.Coverage.Available, "derivatives_funding_open_interest_liquidations") {
		t.Fatalf("derivatives should be available: %+v", report.Coverage)
	}
	if containsString(report.Coverage.Missing, "onchain_mvrv_nupl_sopr_realized_cap") {
		t.Fatalf("onchain should not remain missing: %+v", report.Coverage)
	}
	if !containsString(report.Coverage.Missing, "exchange_flow_reserve_netflow") {
		t.Fatalf("exchange flow should remain missing: %+v", report.Coverage)
	}
	if report.DataGovernance.AvailabilityStatus != "technical_plus_crypto_context" {
		t.Fatalf("availability = %s", report.DataGovernance.AvailabilityStatus)
	}
}

func TestFinalizeCoverageKeepsStaleGDPWarningWhenComputed(t *testing.T) {
	warning := "GSYH dosyası 2026 yılında çekilmiş olsa da son gerçekleşen gözlem 2024."
	coverage := finalizeCoverage(
		CoverageReport{},
		SymbolInput{AssetType: ohlcv.AssetTypeEquity},
		CompanyProfile{},
		ValuationAnalysis{},
		PeerComparison{},
		MarketContext{GDP: tamacro.GDPContext{Computed: true, DataQualityWarning: warning}},
		DisclosureReview{RecentDisclosureStatus: "available"},
	)
	if !containsString(coverage.Available, "tuik_gdp_macro_context") {
		t.Fatalf("computed GDP context should remain available: %+v", coverage)
	}
	if !containsString(coverage.Warnings, warning) {
		t.Fatalf("stale GDP warning should remain visible: %+v", coverage)
	}
}

func TestCommoditySectorFinancialsAreNotApplicable(t *testing.T) {
	report := analyzeCommoditySymbol(SymbolInput{
		Symbol:    "XAUUSD",
		AssetType: ohlcv.AssetTypeCommodity,
		Currency:  "USD",
		LastClose: 2400,
	}, normalizeOptions(Options{}))
	if report.Company.Sector != "Precious Metals" {
		t.Fatalf("sector = %s", report.Company.Sector)
	}
	if report.SectorFinancials.Applicable {
		t.Fatalf("commodity sector financials should be disabled: %+v", report.SectorFinancials)
	}
	if !containsString(report.Coverage.Missing, "usd_index_dxy_real_yield_macro") {
		t.Fatalf("commodity macro source should be listed as improvable: %+v", report.Coverage)
	}
}

func TestCommodityContextImprovesCoverageWhenAvailable(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	contextDir := filepath.Join(root, "commodities", "XAUUSD")
	if err := os.MkdirAll(contextDir, 0o755); err != nil {
		t.Fatalf("mkdir context dir: %v", err)
	}
	contextJSON := []byte(`{
  "as_of": "2026-06-17T00:00:00Z",
  "macro": {"available": true, "source": "FRED DFII10 + USD index", "score": 90, "summary": "DXY/USD index and real yield loaded"},
  "futures_positioning": {"available": true, "source": "CFTC COT", "score": 85, "summary": "COT and open interest loaded"},
  "gold_etf_physical_flow": {"available": true, "source": "World Gold Council", "score": 85, "summary": "ETF and physical flow loaded"},
  "central_bank_geopolitical_news": {"available": true, "source": "World Gold Council + news", "score": 80, "summary": "central bank and geopolitical context loaded"}
}`)
	if err := os.WriteFile(filepath.Join(contextDir, "commodity_context.json"), contextJSON, 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}

	report := analyzeCommoditySymbol(SymbolInput{
		EquitiesDir: equitiesDir,
		Symbol:      "XAUUSD",
		AssetType:   ohlcv.AssetTypeCommodity,
		Currency:    "USD",
		LastClose:   2400,
	}, normalizeOptions(Options{}))

	if !report.CommodityContext.Computed {
		t.Fatalf("expected commodity context to be loaded: %+v", report.CommodityContext)
	}
	if !containsString(report.Coverage.Available, "usd_index_dxy_real_yield_macro") {
		t.Fatalf("macro should be available: %+v", report.Coverage)
	}
	if !containsString(report.Coverage.Available, "futures_cot_open_interest_positioning") {
		t.Fatalf("futures positioning should be available: %+v", report.Coverage)
	}
	if !containsString(report.Coverage.Available, "gold_etf_physical_flow") {
		t.Fatalf("gold flow should be available: %+v", report.Coverage)
	}
	if !containsString(report.Coverage.Available, "central_bank_geopolitical_news") {
		t.Fatalf("central bank/geopolitical context should be available: %+v", report.Coverage)
	}
	if len(report.Coverage.Missing) != 0 {
		t.Fatalf("coverage should be complete: %+v", report.Coverage)
	}
	if report.DataQuality != 100 {
		t.Fatalf("data quality = %.1f, want 100", report.DataQuality)
	}
	if report.DataGovernance.AvailabilityStatus != "technical_plus_commodity_context" {
		t.Fatalf("availability = %s", report.DataGovernance.AvailabilityStatus)
	}
	if !report.DataGovernance.ProductionReady {
		t.Fatalf("commodity context should make data governance production-ready: %+v", report.DataGovernance)
	}
	if len(report.Disclosure.RequiredSources) != 0 || len(report.Disclosure.RiskFlags) != 0 {
		t.Fatalf("disclosure should not list missing commodity sources: %+v", report.Disclosure)
	}
}

func TestAllCurrentKAPSectorsHaveFinancialProfiles(t *testing.T) {
	sectors := []string{
		"BİLGİ HİZMET FAALİYETLERİ",
		"TELEKOMÜNİKASYON",
		"YAYIMCILIK",
		"ELEKTRİK GAZ VE BUHAR",
		"SPOR EĞLENCE BOŞ ZAMANLARI DEĞERLENDİRME HİZMETLERİ",
		"SPOR FAALİYETLERİ EĞLENCE VE OYUN FAALİYETLERİ",
		"YARATICI SANATLAR GÖSTERİ SANATLARI VE EĞLENCE FAALİYETLERİ",
		"İNSAN SAĞLIĞI VE SOSYAL HİZMETLER",
		"GAYRİMENKUL FAALİYETLERİ",
		"DİĞER MADENCİLİK VE TAŞ OCAKÇILIĞI",
		"HAM PETROL VE DOĞAL GAZ ÇIKARTILMASI",
		"KÖMÜR VE LİNYİT MADENCİLİĞİ",
		"METAL CEVHERİ MADENCİLİĞİ",
		"ARACI KURUMLAR",
		"BANKALAR",
		"FİNANSAL KİRALAMA VE FAKTORİNG ŞİRKETLERİ",
		"FİNANSMAN ŞİRKETLERİ",
		"GAYRİMENKUL YATIRIM ORTAKLIKLARI",
		"GİRİŞİM SERMAYESİ YATIRIM ORTAKLIKLARI",
		"HOLDİNGLER VE YATIRIM ŞİRKETLERİ",
		"MENKUL KIYMET YATIRIM ORTAKLIKLARI",
		"SİGORTA ŞİRKETLERİ",
		"VARLIK YÖNETİM ŞİRKETLERİ",
		"HUKUK VE MUHASEBE FAALİYETLERİ",
		"MİMARLIK VE MÜHENDİSLİK FAALİYETLERİ; TEKNİK MUAYENE VE ANALİZ",
		"REKLAMCILIK VE PAZAR ARAŞTIRMASI",
		"KONAKLAMA",
		"YİYECEK VE İÇECEK HİZMETLERİ",
		"BALIKÇILIK VE SU ÜRÜNLERİ",
		"TARIM VE HAYVANCILIK AVCILIK VE İLGİLİ HİZMET FAALİYETLERİ",
		"BİLİŞİM",
		"SAVUNMA",
		"PERAKENDE TİCARET",
		"TOPTAN TİCARET",
		"ULAŞTIRMA VE DEPOLAMA",
		"BÜRO YÖNETİMİ, BÜRO DESTEĞİ VE DİĞER ŞİRKET DESTEK FAALİYETLERİ",
		"KİRALAMA VE LEASING FAALİYETLERİ",
		"SEYAHAT ACENTESİ, TUR OPERATÖRÜ VE DİĞER REZERVASYON HİZMETLERİ İLE İLGİLİ FAALİYETLER",
		"ANA METAL SANAYİ",
		"DİĞER İMALAT SANAYİİ",
		"GIDA, İÇECEK VE TÜTÜN",
		"KAĞIT VE KAĞIT ÜRÜNLERİ BASIM",
		"KİMYA İLAÇ PETROL LASTİK VE PLASTİK ÜRÜNLER",
		"METAL EŞYA MAKİNE ELEKTRİKLİ CİHAZLAR VE ULAŞIM ARAÇLARI",
		"ORMAN ÜRÜNLERİ VE MOBİLYA",
		"TAŞ VE TOPRAĞA DAYALI",
		"TEKSTİL, GİYİM EŞYASI VE DERİ",
		"İNŞAAT VE BAYINDIRLIK İŞLERİ",
	}
	for _, sector := range sectors {
		profile := sectorFinancialProfileFor(CompanyProfile{Sector: "KAP", Industry: sector}, "")
		if profile.ID == "" || profile.ID == "generic_operating_company" {
			t.Fatalf("sector %q mapped to fallback profile %+v", sector, profile)
		}
	}
}

func TestKAPMemberTypeIGSDoesNotClassifyIndustrialCompanyAsREIT(t *testing.T) {
	kap := map[string]any{
		"financialType":  "SIR",
		"kapMemberType":  "IGS",
		"kapMemberTitle": "ASELSAN ELEKTRONİK SANAYİ VE TİCARET A.Ş.",
		"stockCode":      "ASELS",
	}
	sector := inferSectorFromKAP("ASELS", "ASELSAN ELEKTRONİK SANAYİ VE TİCARET A.Ş.", kap)
	if sector != "Savunma ve Elektronik" {
		t.Fatalf("sector = %q, want Savunma ve Elektronik", sector)
	}
	if sameSector("TRGYO", "TORUNLAR GAYRİMENKUL YATIRIM ORTAKLIĞI A.Ş.", sector) {
		t.Fatalf("GYO peer should not match ASELS sector")
	}
	if !sameSector("KAREL", "KAREL ELEKTRONİK SANAYİ VE TİCARET A.Ş.", sector) {
		t.Fatalf("electronics peer should match ASELS sector")
	}
}

func TestCompanyProfileUsesClassificationStoreBeforeNameHeuristic(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "data", "equities")
	seedDir := filepath.Join(root, "data", "seed")
	if err := os.MkdirAll(filepath.Join(equitiesDir, "ASELS"), 0o755); err != nil {
		t.Fatalf("mkdir equity: %v", err)
	}
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("mkdir seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(equitiesDir, "ASELS", "equity.json"), []byte(`{
		"ticker": "ASELS",
		"name": "ASELSAN ELEKTRONİK SANAYİ VE TİCARET A.Ş.",
		"kap_info": {"financialType":"SIR","kapMemberType":"IGS","kapMemberTitle":"ASELSAN ELEKTRONİK SANAYİ VE TİCARET A.Ş."}
	}`), 0o644); err != nil {
		t.Fatalf("write equity: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "sector_classifications.json"), []byte(`{
		"entries": {
			"ASELS": {
				"sector": "Savunma ve Elektronik",
				"industry": "Savunma Elektroniği",
				"peer_group": "bist_defense_electronics",
				"peer_symbols": ["ALTNY","SDTTR"],
				"source": "manual_peer_universe",
				"confidence": 0.85
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("write classification: %v", err)
	}

	profile := companyProfile(SymbolInput{EquitiesDir: equitiesDir, Symbol: "ASELS"})
	if profile.Sector != "Savunma ve Elektronik" || profile.PeerGroup != "bist_defense_electronics" {
		t.Fatalf("profile classification not loaded: %+v", profile)
	}
	if profile.SectorSource != "manual_peer_universe" || profile.ClassificationConfidence != 0.85 {
		t.Fatalf("classification metadata = %+v", profile)
	}
	if len(profile.PeerSymbols) != 2 || profile.PeerSymbols[0] != "ALTNY" {
		t.Fatalf("peer symbols = %+v", profile.PeerSymbols)
	}
}

func TestDCFUsesAssumptionStoreWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "assumptions.json")
	err := os.WriteFile(file, []byte(`{
		"default": {
			"risk_free_rate": 0.10,
			"beta": 1.2,
			"equity_risk_premium": 0.06,
			"cost_of_debt": 0.12,
			"tax_rate": 0.20,
			"wacc": 0.14,
			"terminal_growth": 0.03,
			"fcf_growth": 0.04
		}
	}`), 0o644)
	if err != nil {
		t.Fatalf("write assumptions: %v", err)
	}
	valuation := ValuationAnalysis{FreeCashFlowTTM: 100, NetDebt: 10, PaidCapital: 10}
	dcf := buildDCF(valuation, "BIST Genel", file)
	if !dcf.Computed {
		t.Fatalf("expected DCF to compute, got %+v", dcf)
	}
	if dcf.AssumptionSource != file || dcf.WACC != 0.14 || dcf.TerminalGrowth != 0.03 {
		t.Fatalf("assumptions not loaded: %+v", dcf)
	}
}

func TestTCMBContextRequiresAllDocumentCategories(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "manifest.jsonl")
	if err := os.WriteFile(manifest, []byte(`{"category":"ppk_kararlari","url":"https://example.com/a.pdf","fetched_at":"2026-06-19T10:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := buildTCMBContext(root)
	if report.Computed {
		t.Fatalf("single TCMB category must not be complete: %+v", report)
	}
	if !containsString(report.RequiredCategoriesMissing, "faiz_kararlari") {
		t.Fatalf("missing required categories not reported: %+v", report.RequiredCategoriesMissing)
	}
}

func TestTCMBContextRequiresUsableTextIndexForAllDocumentCategories(t *testing.T) {
	root := t.TempDir()
	var manifest strings.Builder
	for _, id := range requiredTCMBDocumentCategoryIDs() {
		manifest.WriteString(`{"category":"` + id + `","url":"https://example.com/` + id + `.pdf","fetched_at":"2026-06-19T10:00:00Z"}` + "\n")
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.jsonl"), []byte(manifest.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	withoutText := buildTCMBContext(root)
	if withoutText.Computed {
		t.Fatalf("TCMB context must not compute without text index: %+v", withoutText)
	}
	if !containsString(withoutText.Warnings, "tcmb_text_index_missing") {
		t.Fatalf("missing text index warning not reported: %+v", withoutText.Warnings)
	}

	var index strings.Builder
	for _, id := range requiredTCMBDocumentCategoryIDs() {
		index.WriteString(`{"category":"` + id + `","status":"ok","text_length":500,"extracted_at":"2026-06-20T10:00:00Z"}` + "\n")
	}
	if err := os.WriteFile(filepath.Join(root, "text_index.jsonl"), []byte(index.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	withText := buildTCMBContext(root)
	if !withText.Computed {
		t.Fatalf("TCMB context should compute with usable text for all required categories: %+v", withText)
	}
	if withText.TextUsableCount != len(requiredTCMBDocumentCategoryIDs()) {
		t.Fatalf("unexpected usable text count: %+v", withText)
	}
}

func TestTCMBEVDSContextRequiresSeriesFilesToMatchCatalog(t *testing.T) {
	root := t.TempDir()
	seriesDir := filepath.Join(root, "series", "bie_dkdovytl")
	if err := os.MkdirAll(seriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(`{
		"fetched_at": "2026-06-19T10:00:00Z",
		"stats": {"data_groups": 1, "series": 2}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seriesDir, "TP.DK.USD.A.YTL.json"), []byte(`{"points":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	partial := buildTCMBEVDSContext(root)
	if partial.Computed {
		t.Fatalf("partial EVDS series archive must not be complete: %+v", partial)
	}
	if !containsString(partial.Warnings, "tcmb_evds_series_partial") {
		t.Fatalf("partial warning missing: %+v", partial.Warnings)
	}

	if err := os.WriteFile(filepath.Join(seriesDir, "TP.DK.EUR.A.YTL.json"), []byte(`{"points":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	complete := buildTCMBEVDSContext(root)
	if !complete.Computed {
		t.Fatalf("complete EVDS archive should pass: %+v", complete)
	}
}

func TestTCMBEVDSContextSeparatesCatalogMatchedAndExtraSeriesFiles(t *testing.T) {
	root := t.TempDir()
	seriesDir := filepath.Join(root, "series", "bie_dkdovytl")
	if err := os.MkdirAll(seriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(`{
		"fetched_at": "2026-06-19T10:00:00Z",
		"stats": {"data_groups": 1, "series": 2},
		"series": [
			{"DATAGROUP_CODE": "bie_dkdovytl", "SERIE_CODE": "TP.DK.USD.A.YTL"},
			{"DATAGROUP_CODE": "bie_dkdovytl", "SERIE_CODE": "TP.DK.EUR.A.YTL"}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TP_DK_USD_A_YTL.json", "TP_DK_EUR_A_YTL.json", "TP_DK_GBP_A_YTL.json"} {
		if err := os.WriteFile(filepath.Join(seriesDir, name), []byte(`{"points":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report := buildTCMBEVDSContext(root)
	if !report.Computed {
		t.Fatalf("catalog-matched EVDS archive should pass even with extra local files: %+v", report)
	}
	if report.SeriesFileCount != 3 || report.CatalogMatchedSeriesFileCount != 2 || report.ExtraSeriesFileCount != 1 {
		t.Fatalf("EVDS file stats must separate total, catalog-matched and extra files: %+v", report)
	}
	if !containsString(report.Warnings, "tcmb_evds_series_extra_files") {
		t.Fatalf("extra local files warning missing: %+v", report.Warnings)
	}
}

func writeMicrostructureJSON(t *testing.T, path, payload string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestIncomeQualityGapFlagTriggersWhenFCFNegativeAndNetIncomePositive(t *testing.T) {
	v := func(val float64) *float64 { return &val }
	// Industrial ASELS-like setup: net income 100, FCF -50 (ratio = -0.50 < -0.30 threshold)
	fin := financialFile{
		Data: map[string]financialField{
			// revenue (3C)
			"3C": {Years: map[string][]*float64{"2025": {v(400), nil, nil, v(80)}, "2026": {nil, nil, nil, v(120)}}},
			// net income (3L for industrial)
			"3L": {Years: map[string][]*float64{"2025": {v(100), nil, nil, v(20)}, "2026": {nil, nil, nil, v(30)}}},
			// free cash flow (4CB for industrial)
			"4CB": {Years: map[string][]*float64{"2025": {v(-200), nil, nil, v(-40)}, "2026": {nil, nil, nil, v(-60)}}},
			// equity
			"2OA": {Years: map[string][]*float64{"2026": {nil, nil, nil, v(500)}}},
			// paid capital
			"2N": {Years: map[string][]*float64{"2026": {nil, nil, nil, v(228)}}},
			// total assets (1Z)
			"1BL": {Years: map[string][]*float64{"2026": {nil, nil, nil, v(1200)}}},
		},
	}
	valuation := buildValuation(fin, 10, latestPeriod(fin), "Savunma", "")

	found := false
	for _, flag := range valuation.Flags {
		if flag == "income_quality_gap_fcf_negative_net_income_positive" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected income_quality_gap flag; got flags: %v", valuation.Flags)
	}
}

func TestDeferredTaxAndFCFQualityMetricsForDefenseIndustrial(t *testing.T) {
	v := func(val float64) *float64 { return &val }
	// ASELS-like setup: deferred tax 40 out of net income 80 (ratio=0.50 → "weak")
	// FCF -60 out of net income 80 (ratio=-0.75 → "critical")
	fin := financialFile{
		FinancialGroup: "UFRS",
		Data: map[string]financialField{
			"3C":  {Years: map[string][]*float64{"2025": {v(400), nil, nil, v(80)}, "2026": {nil, nil, nil, v(120)}}},
			"3L":  {Years: map[string][]*float64{"2025": {v(200), nil, nil, v(40)}, "2026": {nil, nil, nil, v(60)}}},
			"3DF": {Years: map[string][]*float64{"2025": {v(80), nil, nil, v(16)}, "2026": {nil, nil, nil, v(24)}}},
			"4CB": {Years: map[string][]*float64{"2025": {v(-160), nil, nil, v(-32)}, "2026": {nil, nil, nil, v(-48)}}},
			"3IC": {Years: map[string][]*float64{"2025": {v(100), nil, nil, v(20)}, "2026": {nil, nil, nil, v(30)}}},
			"2OA": {Years: map[string][]*float64{"2026": {nil, nil, nil, v(500)}}},
			"2N":  {Years: map[string][]*float64{"2026": {nil, nil, nil, v(228)}}},
			"1BL": {Years: map[string][]*float64{"2026": {nil, nil, nil, v(1200)}}},
		},
	}
	profile := CompanyProfile{Sector: "SANAYİ", Industry: "SAVUNMA"}
	valuation := buildValuation(fin, 10, latestPeriod(fin), "Savunma", "")
	analysis := analyzeSectorFinancials(fin, latestPeriod(fin), profile, valuation)

	metricsByID := map[string]SectorFinancialMetric{}
	for _, m := range analysis.Metrics {
		metricsByID[m.Name] = m
	}

	dtMetric, ok := metricsByID["deferred_tax_quality"]
	if !ok {
		t.Fatalf("deferred_tax_quality metric missing; got: %v", analysis.Metrics)
	}
	if dtMetric.Status != "weak" {
		t.Fatalf("expected deferred_tax_quality=weak (ratio>0.30), got %q (value=%.3f)", dtMetric.Status, dtMetric.Value)
	}

	fcfMetric, ok := metricsByID["fcf_income_quality"]
	if !ok {
		t.Fatalf("fcf_income_quality metric missing; got: %v", analysis.Metrics)
	}
	if fcfMetric.Status != "critical" {
		t.Fatalf("expected fcf_income_quality=critical (FCF<0), got %q (value=%.3f)", fcfMetric.Status, fcfMetric.Value)
	}
}

func testCandles(count int) []ohlcv.Candle {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	price := 100.0
	for i := 0; i < count; i++ {
		open := price
		closePrice := price + 0.25 + float64(i%9)*0.03
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   closePrice + 1,
			Low:    open - 1,
			Close:  closePrice,
			Volume: 100000 + float64(i*1000),
		}
		price = closePrice
	}
	return candles
}
