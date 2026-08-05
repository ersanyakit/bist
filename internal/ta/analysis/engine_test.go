package analysis

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"hissebot/internal/domain/pricequality"
	"hissebot/internal/ta/chart"
	"hissebot/internal/ta/datasource"
	"hissebot/internal/ta/formations"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/supportresistance"
	"hissebot/internal/ta/value"
	"hissebot/internal/ta/vapcontext"
)

func TestAnalyzeSymbolBuildsCompleteTimeframeContract(t *testing.T) {
	engine := NewEngine(datasource.NewMockProvider(), testRenderer{}, EngineOptions{
		Timeframes: []string{"1D"},
		Limit:      260,
	})
	result, err := engine.AnalyzeSymbol(context.Background(), SymbolRequest{
		Symbol:      "TEST",
		CompanyName: "Test A.S.",
		Currency:    "TRY",
	})
	if err != nil {
		t.Fatalf("analyze symbol: %v", err)
	}
	if result.Symbol != "TEST" || result.CompanyName != "Test A.S." || result.Currency != "TRY" {
		t.Fatalf("unexpected symbol metadata: %+v", result)
	}
	if !strings.Contains(result.AnalysisDate, "_") || len(result.AnalysisDate) != len("2006-01-02_15-04-05") {
		t.Fatalf("analysis date should include time for unique output folders, got %q", result.AnalysisDate)
	}
	if result.Disclaimer == "" {
		t.Fatal("expected disclaimer")
	}
	tf, ok := result.Timeframes["1D"]
	if !ok {
		t.Fatal("missing 1D analysis")
	}
	if tf.CandleCount != 260 || len(tf.Candles) != 260 {
		t.Fatalf("unexpected candle count: %d/%d", tf.CandleCount, len(tf.Candles))
	}
	if tf.LastClose <= 0 || tf.LastVolume <= 0 {
		t.Fatalf("invalid last market data: close=%.2f volume=%.2f", tf.LastClose, tf.LastVolume)
	}
	if tf.Indicators.RSI14 < 0 || tf.Indicators.RSI14 > 100 {
		t.Fatalf("RSI out of range: %.2f", tf.Indicators.RSI14)
	}
	if tf.Indicators.ATR14 <= 0 {
		t.Fatalf("expected positive ATR, got %.2f", tf.Indicators.ATR14)
	}
	if len(tf.IndicatorSignals) == 0 {
		t.Fatal("expected indicator scan output")
	}
	if len(tf.PatternScans) == 0 {
		t.Fatal("expected full pattern scan output")
	}
	if tf.Professional.Technical.Validation.Status == "" {
		t.Fatal("expected technical validation report")
	}
	if tf.Professional.Technical.Validation.ChartOverlayStatus == "" {
		t.Fatalf("expected chart overlay validation status: %+v", tf.Professional.Technical.Validation)
	}
	if tf.TrendBias != "bullish" && tf.TrendBias != "bearish" && tf.TrendBias != "neutral" {
		t.Fatalf("unexpected trend bias %q", tf.TrendBias)
	}
	if tf.Score < 0 || tf.Score > 100 || result.OverallScore < 0 || result.OverallScore > 100 {
		t.Fatalf("score out of range: tf=%.2f overall=%.2f", tf.Score, result.OverallScore)
	}
	if chartBytes := result.Charts["1D"]; string(chartBytes) != "png" {
		t.Fatalf("expected renderer bytes to be stored, got %q", string(chartBytes))
	}
	if result.PriceQuality == nil {
		t.Fatal("expected equity analysis to carry a price quality gate")
	}
	if !result.FinTradeBench.Computed || len(result.FinTradeBench.TradingSignals) == 0 {
		t.Fatalf("expected FinTradeBench evidence layer: %+v", result.FinTradeBench)
	}
	if !result.Quant.Computed || result.Quant.SampleCount == 0 || result.Quant.Decision.Score <= 0 {
		t.Fatalf("expected quant risk/return layer: %+v", result.Quant)
	}
	if result.Quant.Risk.HistoricalVaR95Pct <= 0 || result.Quant.Risk.AnnualizedVolatilityPct <= 0 {
		t.Fatalf("expected quant risk metrics: %+v", result.Quant.Risk)
	}
	if !result.StatEconomic.Computed || result.StatEconomic.CompositeScore <= 0 {
		t.Fatalf("expected statistical/economic layer: %+v", result.StatEconomic)
	}
	if result.StatEconomic.Regime.GARCHVolatilityForecastPct <= 0 || result.StatEconomic.DataIntegrity.Score <= 0 {
		t.Fatalf("expected stat/economic diagnostics: %+v", result.StatEconomic)
	}
	if !result.Advanced.Computed || result.Advanced.CompositeScore <= 0 || len(result.Advanced.Phases) < 10 {
		t.Fatalf("expected advanced phase analysis: %+v", result.Advanced)
	}
	if result.Advanced.Production.ReportHash == "" {
		t.Fatalf("expected advanced production audit hash: %+v", result.Advanced.Production)
	}
	if !result.NextSessionForecast.Computed || result.NextSessionForecast.PredictedOpen <= 0 || result.NextSessionForecast.PredictedClose <= 0 {
		t.Fatalf("expected symbol-level next-session open/close forecast: %+v", result.NextSessionForecast)
	}
	if result.NextSessionForecast.PredictedOpen != tf.NextSessionForecast.PredictedOpen ||
		result.NextSessionForecast.PredictedClose != tf.NextSessionForecast.PredictedClose {
		t.Fatalf("symbol forecast must mirror daily forecast: symbol=%+v daily=%+v", result.NextSessionForecast, tf.NextSessionForecast)
	}
	if result.DecisionSupport == nil {
		t.Fatal("expected equity analysis to carry decision support gate")
	}
	if tf.TradePlan.Direction != "neutral" {
		assertAnalysisTradePlanSane(t, tf)
	}
}

func TestFetchBISTBulletinRecordsForAnalysisUsesAsOfNotForecastFor(t *testing.T) {
	provider := &recordingBulletinRangeProvider{}
	engine := NewEngine(nil, nil, EngineOptions{
		AsOf: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC),
	})

	_, err := engine.fetchBISTBulletinRecordsForAnalysis(context.Background(), provider, SymbolAnalysis{
		Symbol: "ASELS",
		NextSessionForecast: NextSessionForecast{
			ForecastFor: "2026-06-19",
		},
	})
	if err != nil {
		t.Fatalf("fetch BIST records: %v", err)
	}
	if provider.toDate != "2026-06-18" {
		t.Fatalf("toDate=%q, want as-of date 2026-06-18", provider.toDate)
	}
}

func TestBuildQuantAnalysisComputesRiskReturnAndBenchmark(t *testing.T) {
	candles := make([]ohlcv.Candle, 0, 80)
	price := 100.0
	for i := 0; i < 80; i++ {
		ret := 0.001 + 0.008*math.Sin(float64(i)/5)
		price *= 1 + ret
		candles = append(candles, ohlcv.Candle{
			Time:          time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Open:          price * 0.995,
			High:          price * 1.01,
			Low:           price * 0.99,
			Close:         price,
			Volume:        1_000_000,
			AdjustedClose: price,
			IsAdjusted:    true,
		})
	}
	result := SymbolAnalysis{
		Symbol:    "TEST",
		AssetType: ohlcv.AssetTypeEquity,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {Timeframe: "1D", Candles: candles, LastClose: price},
		},
		Professional: professional.Report{
			Market: professional.MarketContext{
				BenchmarkSymbol:    "XU100",
				BenchmarkAvailable: true,
				RelativeStrength20: 0.03,
				RelativeStrength60: 0.04,
				Beta60:             0.92,
				Alpha60:            0.08,
				Correlation60:      0.64,
			},
			DataGovernance: professional.FinancialDataGovernance{
				ProductionReady: true,
			},
		},
		PriceQuality: &pricequality.SymbolReport{ReadyForVerifiedClose: true},
	}

	got := BuildQuantAnalysis(result)
	if !got.Computed || got.SampleCount != 79 {
		t.Fatalf("quant not computed: %+v", got)
	}
	if got.MarketClock != "exchange_sessions" || got.AnnualizationDays != 252 {
		t.Fatalf("unexpected equity quant clock: %+v", got)
	}
	if got.Return.Return20DPct == 0 || got.Risk.HistoricalVaR95Pct <= 0 || got.Risk.SharpeRatio == 0 {
		t.Fatalf("quant metrics missing: return=%+v risk=%+v", got.Return, got.Risk)
	}
	if !got.Benchmark.Available || got.Benchmark.Beta60 != 0.92 || got.Benchmark.RelativeStrength60Pct != 4 {
		t.Fatalf("benchmark metrics missing: %+v", got.Benchmark)
	}
	if got.EquityProfile == nil || got.EquityProfile.Status != "pass" || got.CryptoProfile != nil {
		t.Fatalf("unexpected equity/crypto profile: equity=%+v crypto=%+v", got.EquityProfile, got.CryptoProfile)
	}
	if got.Decision.Score <= 0 || got.Decision.Summary == "" {
		t.Fatalf("decision missing: %+v", got.Decision)
	}
}

func TestBuildQuantAnalysisUsesCryptoClockAndProfile(t *testing.T) {
	candles := make([]ohlcv.Candle, 0, 120)
	price := 50_000.0
	for i := 0; i < 120; i++ {
		ret := 0.0015 + 0.010*math.Sin(float64(i)/6)
		price *= 1 + ret
		candles = append(candles, ohlcv.Candle{
			Time:   time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Open:   price * 0.998,
			High:   price * 1.015,
			Low:    price * 0.985,
			Close:  price,
			Volume: 8_000,
		})
	}
	result := SymbolAnalysis{
		Symbol:    "BTCUSDT",
		AssetType: ohlcv.AssetTypeCrypto,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {Timeframe: "1D", Candles: candles, LastClose: price},
		},
		Professional: professional.Report{
			Market: professional.MarketContext{
				BenchmarkSymbol:    "BTC",
				BenchmarkAvailable: true,
				RelativeStrength60: 0.06,
				Beta60:             1.05,
			},
			Coverage: professional.CoverageReport{Score: 85},
			CryptoContext: professional.CryptoContextReport{
				OnChain:       professional.CryptoContextSection{Available: true},
				Derivatives:   professional.CryptoContextSection{Available: true},
				ExchangeFlow:  professional.CryptoContextSection{Available: true},
				NewsSentiment: professional.CryptoContextSection{Available: true},
			},
		},
	}

	got := BuildQuantAnalysis(result)
	if !got.Computed || got.MarketClock != "24_7" || got.AnnualizationDays != 365 {
		t.Fatalf("unexpected crypto quant identity: %+v", got)
	}
	if got.CryptoProfile == nil || got.CryptoProfile.Status != "pass" || got.EquityProfile != nil {
		t.Fatalf("unexpected crypto/equity profile: crypto=%+v equity=%+v", got.CryptoProfile, got.EquityProfile)
	}
	if got.Risk.VolatilityRegime == "" || got.Decision.Score <= 0 {
		t.Fatalf("crypto quant metrics missing: %+v", got)
	}
}

func TestComputeNextSessionForecastPredictsOpenCloseAndSkipsWeekend(t *testing.T) {
	candles := make([]ohlcv.Candle, 0, 25)
	start := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	price := 100.0
	for i := 0; i < 25; i++ {
		open := price * 1.001
		closePrice := open * 1.002
		candles = append(candles, ohlcv.Candle{
			Time: start.AddDate(0, 0, i), Open: open, High: closePrice + 1, Low: open - 1, Close: closePrice, Volume: 1_000_000,
		})
		price = closePrice
	}
	// Make the final observation Friday so the equity forecast targets Monday.
	candles[len(candles)-1].Time = time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	forecast := computeNextSessionForecast(candles, ohlcv.IndicatorSnapshot{
		ATR14: 2.5, RSI14: 61, MACDHistogram: 0.4, ChaikinMoneyFlow20: 0.10, EMA20: price - 2, Supertrend: price - 4,
	}, "bullish", ohlcv.AssetTypeEquity)
	if !forecast.Computed || forecast.ForecastFor != "2026-06-22" {
		t.Fatalf("unexpected forecast identity: %+v", forecast)
	}
	if forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		t.Fatalf("open/close point estimates missing: %+v", forecast)
	}
	if forecast.ExpectedLow > forecast.PredictedOpen || forecast.ExpectedLow > forecast.PredictedClose ||
		forecast.ExpectedHigh < forecast.PredictedOpen || forecast.ExpectedHigh < forecast.PredictedClose {
		t.Fatalf("point estimates must stay inside expected band: %+v", forecast)
	}
	if forecast.HistoricalSamples != 20 || forecast.Confidence <= 0 || forecast.Model == "" {
		t.Fatalf("forecast evidence metadata missing: %+v", forecast)
	}
	if !strings.Contains(forecast.Model, "separate_open_gap_close_intraday") {
		t.Fatalf("forecast must use separated open/close model, got %q", forecast.Model)
	}
}

func TestNextSessionDecisionForecastBacktestBlocksWeakModel(t *testing.T) {
	candles := makeVolatileForecastTestCandles(55)
	forecast, err := ComputeNextSessionForecastFromCandles(candles, ohlcv.AssetTypeEquity)
	if err != nil {
		t.Fatalf("compute forecast: %v", err)
	}
	if len(forecast.BacktestTable) != nextSessionForecastDecisionBacktestWindow {
		t.Fatalf("backtest rows=%d, want %d", len(forecast.BacktestTable), nextSessionForecastDecisionBacktestWindow)
	}
	if forecast.BacktestMetrics.Samples != nextSessionForecastDecisionBacktestWindow {
		t.Fatalf("backtest metrics missing: %+v", forecast.BacktestMetrics)
	}
	if forecast.BacktestMetrics.OpenMAE <= 0 || forecast.BacktestMetrics.OpenMAPE <= 0 ||
		forecast.BacktestMetrics.CloseMAE <= 0 || forecast.BacktestMetrics.CloseMAPE <= 0 {
		t.Fatalf("MAE/MAPE metrics missing: %+v", forecast.BacktestMetrics)
	}
	if forecast.BacktestMetrics.HitRatioWithin050Pct < 0 || forecast.BacktestMetrics.HitRatioWithin100Pct < 0 || forecast.BacktestMetrics.HitRatioWithin200Pct < 0 {
		t.Fatalf("hit ratios invalid: %+v", forecast.BacktestMetrics)
	}
	forecast = ApplyNextSessionForecastPublishState(forecast)
	if forecast.BacktestMetrics.CloseMAPE > nextSessionDecisionMaxCloseMAPEPct && forecast.DecisionForecast.TradeSignalAllowed {
		t.Fatalf("close MAPE %.2f should block trade signal: %+v", forecast.BacktestMetrics.CloseMAPE, forecast.DecisionForecast)
	}
	if forecast.BacktestMetrics.DirectionAccuracy < nextSessionDecisionMinDirectionAccuracyPct && !forecast.DecisionForecast.DirectionModelUnreliable {
		t.Fatalf("direction accuracy %.2f should mark direction model unreliable: %+v", forecast.BacktestMetrics.DirectionAccuracy, forecast.DecisionForecast)
	}
}

func TestApplyNextSessionForecastPublishStateSuppressesDecisionPointPrices(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:                    true,
		ForecastFor:                 "2026-06-22",
		LastClose:                   100,
		PredictedOpen:               101,
		PredictedClose:              102,
		BacktestSamples:             10,
		BacktestCloseMAEPct:         0.8,
		BacktestDirectionHitRatePct: 60,
		Confidence:                  70,
		BiasReasons:                 []string{"test"},
	}

	got := ApplyNextSessionForecastPublishState(forecast)
	if got.PointForecastPublishable || got.PublishedPredictedOpen != nil || got.PublishedPredictedClose != nil {
		t.Fatalf("blocked forecast must not publish point prices: %+v", got)
	}
	if got.DecisionForecast.OpenForecast != 0 || got.DecisionForecast.CloseForecast != 0 {
		t.Fatalf("blocked forecast leaked decision point prices: %+v", got.DecisionForecast)
	}
	if got.DecisionForecast.ExpectedIntradayDirection != "uncertain" || got.DecisionForecast.TradeSignalAllowed {
		t.Fatalf("blocked forecast should be uncertain/no trade: %+v", got.DecisionForecast)
	}
	if got.PredictedOpen != 101 || got.PredictedClose != 102 {
		t.Fatalf("scenario prices should remain available for non-decision context: %+v", got)
	}
}

func TestDecisionForecastUsesBISTTickAndSchemaFields(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-22",
		LastClose:         402.50,
		RawPredictedOpen:  402.63,
		RawPredictedClose: 404.88,
		RawExpectedLow:    396.12,
		RawExpectedHigh:   411.37,
		Confidence:        70,
		BiasReasons:       []string{"test reason"},
	}
	forecast = applyTradablePriceStepToNextSessionForecast(forecast, ohlcv.AssetTypeEquity, "ASELS")
	forecast.PointForecastPublishable = true
	forecast = syncNextSessionDecisionForecast(forecast, "ASELS")
	if forecast.DecisionForecast.Ticker != "ASELS" || forecast.DecisionForecast.Date != "2026-06-22" {
		t.Fatalf("schema identity missing: %+v", forecast.DecisionForecast)
	}
	for _, value := range []float64{forecast.DecisionForecast.OpenForecast, forecast.DecisionForecast.CloseForecast, forecast.DecisionForecast.OpenRangeLow, forecast.DecisionForecast.CloseRangeHigh} {
		if math.Abs(value/0.25-math.Round(value/0.25)) > 1e-9 {
			t.Fatalf("value %.4f is not rounded to ASELS 0.25 tick: %+v", value, forecast.DecisionForecast)
		}
	}
	if forecast.DecisionForecast.Confidence == "" || forecast.DecisionForecast.ExpectedIntradayDirection == "" {
		t.Fatalf("schema decision fields missing: %+v", forecast.DecisionForecast)
	}
}

func TestFilterCandlesThroughAsOfCutsFutureSessions(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC), Close: 100},
		{Time: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), Close: 101},
		{Time: time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC), Close: 102},
	}
	got := filterCandlesThroughAsOf(candles, time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC))
	if len(got) != 2 {
		t.Fatalf("expected 2 candles through as-of, got %d", len(got))
	}
	if got[len(got)-1].Time.Format("2006-01-02") != "2026-06-18" {
		t.Fatalf("future candle leaked into as-of window: %+v", got)
	}
}

func TestBISTBulletinContextUsesAnalysisLatestButKeepsForecastActual(t *testing.T) {
	result := SymbolAnalysis{
		Symbol:    "ASELS",
		AssetType: ohlcv.AssetTypeEquity,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Candles:   []ohlcv.Candle{{Time: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), Close: 410.75}},
				LastClose: 410.75,
			},
		},
		NextSessionForecast: NextSessionForecast{
			Computed:    true,
			ForecastFor: "2026-06-19",
			LastClose:   410.75,
		},
	}
	records := []datasource.DailyBulletinRecord{
		{TradingDate: "2026-06-18", Open: 414, Close: 410.75},
		{TradingDate: "2026-06-19", Open: 408.75, Close: 402.50, SourcePath: "thb202606191.csv"},
	}
	got := buildBISTBulletinContext(result, records)
	if got.LatestRecord.TradingDate != "2026-06-18" {
		t.Fatalf("analysis latest should stay at 2026-06-18, got %+v", got.LatestRecord)
	}
	if !got.ForecastActualAvailable || got.ForecastActualRecord.TradingDate != "2026-06-19" {
		t.Fatalf("forecast actual should be kept for validation: %+v", got)
	}
	if !got.OfficialCloseConfirmed {
		t.Fatalf("official close should be confirmed against analysis-date record: %+v", got)
	}
}

func TestBISTBulletinMicrostructureOverlayDetectsPositiveExhaustion(t *testing.T) {
	records := make([]datasource.DailyBulletinRecord, 0, 25)
	start := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		date := start.AddDate(0, 0, i)
		records = append(records, datasource.DailyBulletinRecord{
			TradingDate:   date.Format("2006-01-02"),
			Open:          100,
			High:          110,
			Low:           90,
			Close:         92,
			PreviousClose: 100,
			VWAP:          98,
			Volume:        10_000_000,
		})
	}
	records = append(records, datasource.DailyBulletinRecord{
		TradingDate:   "2026-06-18",
		Open:          393.50,
		High:          412.50,
		Low:           390.00,
		Close:         410.75,
		PreviousClose: 395.00,
		ChangePct:     3.987,
		VWAP:          400.111,
		Volume:        25_136_272,
	})
	forecast := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-19",
		LastClose:         410.75,
		RawPredictedOpen:  413.01,
		RawPredictedClose: 415.10,
		PredictedOpen:     413.00,
		PredictedClose:    415.00,
		RawExpectedLow:    397.50,
		RawExpectedHigh:   424.00,
		ExpectedLow:       397.50,
		ExpectedHigh:      424.00,
		Confidence:        70,
		Model:             "atr_gap_intraday_ewma_v1",
	}
	got := ApplyBISTBulletinRecordsToNextSessionForecastForAudit(forecast, records, ohlcv.AssetTypeEquity, "ASELS")
	if got.PredictedClose >= forecast.LastClose {
		t.Fatalf("expected microstructure overlay to switch close forecast below last close: %+v", got)
	}
	if got.DirectionBias != "düşüş" {
		t.Fatalf("expected bearish mean-reversion direction, got %+v", got)
	}
	if !strings.Contains(got.Model, "bist_microstructure_v2") {
		t.Fatalf("expected model overlay marker, got %q", got.Model)
	}
	reasons := strings.Join(got.BiasReasons, "\n")
	if !strings.Contains(reasons, "BIST bülten mikro yapı") {
		t.Fatalf("expected BIST microstructure reason, got %q", reasons)
	}
}

func TestBISTBulletinOverlayRequiresDecisionGradeValidation(t *testing.T) {
	records := make([]datasource.DailyBulletinRecord, 0, 25)
	start := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		date := start.AddDate(0, 0, i)
		records = append(records, datasource.DailyBulletinRecord{
			TradingDate:   date.Format("2006-01-02"),
			Open:          100,
			High:          110,
			Low:           90,
			Close:         92,
			PreviousClose: 100,
			VWAP:          98,
			Volume:        10_000_000,
		})
	}
	records = append(records, datasource.DailyBulletinRecord{
		TradingDate:   "2026-06-18",
		Open:          393.50,
		High:          412.50,
		Low:           390.00,
		Close:         410.75,
		PreviousClose: 395.00,
		ChangePct:     3.987,
		VWAP:          400.111,
		Volume:        25_136_272,
	})
	forecast := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-19",
		LastClose:         410.75,
		RawPredictedOpen:  413.01,
		RawPredictedClose: 415.10,
		PredictedOpen:     413.00,
		PredictedClose:    415.00,
		RawExpectedLow:    397.50,
		RawExpectedHigh:   424.00,
		ExpectedLow:       397.50,
		ExpectedHigh:      424.00,
		Confidence:        70,
		Model:             "separate_open_gap_close_intraday_v2",
	}

	got := ApplyBISTBulletinRecordsToNextSessionForecast(forecast, records, ohlcv.AssetTypeEquity, "ASELS")
	if got.PredictedOpen != 413.00 || got.PredictedClose != 415.00 {
		t.Fatalf("production overlay must not rewrite prices without enough rolling validation: %+v", got)
	}
	if strings.Contains(got.Model, "bist_microstructure_v2") {
		t.Fatalf("production model should not include rejected overlay marker: %+v", got)
	}
	if !containsString(got.Warnings, "bist_bulletin_overlay_validation_failed") {
		t.Fatalf("expected explicit rejected-overlay warning: %+v", got.Warnings)
	}
}

func TestBISTOpenAnchorKeepsCloseModelSeparate(t *testing.T) {
	base := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-22",
		LastClose:         100,
		RawPredictedOpen:  100.25,
		RawPredictedClose: 103.40,
		PredictedOpen:     100.25,
		PredictedClose:    103.40,
		RawExpectedLow:    97,
		RawExpectedHigh:   104,
		ExpectedLow:       97,
		ExpectedHigh:      104,
		Confidence:        60,
		Model:             "separate_open_gap_close_intraday_v2",
	}
	overlay := base
	overlay.RawPredictedOpen = 101.75
	overlay.PredictedOpen = 101.75
	overlay.RawPredictedClose = 101.75
	overlay.PredictedClose = 101.75

	got := applyBISTBulletinOpenAnchorForecast(base, overlay, nextSessionForecastBacktestMetrics{
		samples:             60,
		openMAEPct:          0.70,
		closeMAEPct:         1.40,
		directionHitRatePct: 58,
	}, ohlcv.AssetTypeEquity, "ASELS", false)

	if got.PredictedOpen != 101.75 {
		t.Fatalf("open anchor should use BIST overlay open: %+v", got)
	}
	if got.PredictedClose != 103.40 {
		t.Fatalf("open anchor must keep close separate from open and round to tick: %+v", got)
	}
	if got.PredictedOpen == got.PredictedClose {
		t.Fatalf("open anchor must not copy open into close: %+v", got)
	}
	if !strings.Contains(got.Model, "bist_open_anchor_open_only_v1") {
		t.Fatalf("expected open-only anchor marker, got %q", got.Model)
	}
	if !containsString(got.Warnings, "bist_bulletin_open_anchor_used_close_kept_separate") {
		t.Fatalf("expected separate-close warning: %+v", got.Warnings)
	}

	closeFallback := applyBISTBulletinOpenAnchorForecast(base, overlay, nextSessionForecastBacktestMetrics{
		samples:             60,
		openMAEPct:          0.60,
		closeMAEPct:         1.10,
		directionHitRatePct: 62,
	}, ohlcv.AssetTypeEquity, "ASELS", true)
	if closeFallback.PredictedOpen != closeFallback.PredictedClose {
		t.Fatalf("close fallback candidate should explicitly anchor close to open: %+v", closeFallback)
	}
	if !strings.Contains(closeFallback.Model, "bist_open_anchor_close_fallback_v1") {
		t.Fatalf("expected close-fallback marker, got %q", closeFallback.Model)
	}
}

func TestWeakValidatedForecastDampsPointAndWidensInterval(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-22",
		LastClose:         100,
		RawPredictedOpen:  104,
		RawPredictedClose: 110,
		PredictedOpen:     104,
		PredictedClose:    110,
		RawExpectedLow:    98,
		RawExpectedHigh:   112,
		ExpectedLow:       98,
		ExpectedHigh:      112,
		CloseP10:          99,
		CloseP50:          110,
		CloseP90:          111,
		Confidence:        60,
		ConfidenceLabel:   "medium",
		DirectionBias:     "yükseliş",
		BiasStrength:      "orta",
		Model:             "separate_open_gap_close_intraday_v2",
	}

	got := calibrateWeakValidatedNextSessionForecast(forecast, nextSessionForecastBacktestMetrics{
		samples:             60,
		closeMAEPct:         2.60,
		directionHitRatePct: 43,
	}, ohlcv.AssetTypeEquity, "ASELS")

	if got.PredictedClose >= 105 {
		t.Fatalf("weak validation should damp close point toward last close: %+v", got)
	}
	if got.CloseP10 >= forecast.CloseP10 || got.CloseP90 <= forecast.CloseP90 {
		t.Fatalf("weak validation should widen close interval: before=%+v after=%+v", forecast, got)
	}
	if !strings.Contains(got.Model, "weak_validation_calibration_v1") {
		t.Fatalf("expected weak validation model marker: %+v", got)
	}
	if !containsString(got.Warnings, "weak_validation_point_forecast_damped_interval_widened") {
		t.Fatalf("expected weak validation warning: %+v", got.Warnings)
	}
}

func TestDecisionIntervalUsesConformalResidualBand(t *testing.T) {
	rows := make([]NextSessionBacktestRow, 0, nextSessionPointForecastMinBacktestSamples)
	for i := 0; i < nextSessionPointForecastMinBacktestSamples; i++ {
		rows = append(rows, NextSessionBacktestRow{ClosePctError: 1 + float64(i)*0.10})
	}
	forecast := NextSessionForecast{
		Computed:       true,
		ForecastFor:    "2026-06-22",
		LastClose:      100,
		PredictedClose: 100,
		ExpectedLow:    90,
		ExpectedHigh:   110,
		Model:          "separate_open_gap_close_intraday_v2",
	}

	got := calibrateNextSessionDecisionInterval(forecast, nextSessionForecastBacktestMetrics{
		samples:             nextSessionPointForecastMinBacktestSamples,
		closeMAEPct:         1.70,
		directionHitRatePct: 58,
		rows:                rows,
	}, ohlcv.AssetTypeEquity, "ASELS")

	if got.DecisionIntervalStatus != "active" || got.DecisionIntervalLow <= 0 || got.DecisionIntervalHigh <= got.DecisionIntervalLow {
		t.Fatalf("expected active decision interval: %+v", got)
	}
	if got.DecisionIntervalWidthPct <= 0 || got.DecisionIntervalWidthPct >= 10 {
		t.Fatalf("expected narrow conformal decision interval, got width %.2f in %+v", got.DecisionIntervalWidthPct, got)
	}
	if !strings.Contains(got.DecisionIntervalReason, "conformal_q75_close_error_pct") {
		t.Fatalf("expected q75 conformal reason: %+v", got)
	}
	if !strings.Contains(got.Model, "decision_interval_conformal_v1") {
		t.Fatalf("expected decision interval model marker: %+v", got)
	}

	weak := calibrateNextSessionDecisionInterval(forecast, nextSessionForecastBacktestMetrics{
		samples:             nextSessionPointForecastMinBacktestSamples,
		closeMAEPct:         2.60,
		directionHitRatePct: 43,
		rows:                rows,
	}, ohlcv.AssetTypeEquity, "ASELS")
	if weak.DecisionIntervalStatus != "candidate_validation_failed" {
		t.Fatalf("weak validation interval must stay candidate-only: %+v", weak)
	}
	if !strings.Contains(weak.DecisionIntervalReason, "conformal_q80_close_error_pct") {
		t.Fatalf("expected q80 candidate interval reason: %+v", weak)
	}
	if !containsString(weak.Warnings, "decision_interval_candidate_validation_failed") {
		t.Fatalf("expected candidate validation warning: %+v", weak.Warnings)
	}
}

func TestApplyTradablePriceStepToNextSessionForecastUsesBISTTick(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-22",
		LastClose:         402.50,
		PredictedOpen:     403.93,
		PredictedClose:    405.01,
		ExpectedLow:       389.48,
		ExpectedHigh:      415.52,
		DirectionBias:     "hafif pozitif",
		BiasStrength:      "zayıf-orta",
		Confidence:        52,
		HistoricalSamples: 20,
		Model:             "atr_gap_intraday_ewma_fundamental_v2",
	}

	got := applyTradablePriceStepToNextSessionForecast(forecast, ohlcv.AssetTypeEquity, "ASELS")
	if got.RawPredictedOpen != 403.93 || got.RawPredictedClose != 405.01 {
		t.Fatalf("raw forecast prices must be preserved: %+v", got)
	}
	if got.PredictedOpen != 404.00 || got.TradablePredictedOpen != 404.00 {
		t.Fatalf("open forecast must be rounded to nearest BIST tick: %+v", got)
	}
	if got.PredictedClose != 405.00 || got.TradablePredictedClose != 405.00 {
		t.Fatalf("close forecast must be rounded to nearest BIST tick: %+v", got)
	}
	if got.ExpectedLow != 389.50 || got.ExpectedHigh != 415.50 {
		t.Fatalf("expected band must be rounded to tradable BIST levels: %+v", got)
	}
	if got.OpenChangePct != 0.37 || got.CloseChangePct != 0.62 {
		t.Fatalf("change percentages must use tradable prices: %+v", got)
	}
	if got.PredictedOpenDirection != "yükseliş" || got.PredictedCloseDirection != "yükseliş" || got.DirectionTolerancePct != nextSessionDirectionTolerancePct {
		t.Fatalf("forecast directions must use tradable prices: %+v", got)
	}
	if got.TickSize != 0.25 || got.RoundingMethod != "nearest_tick" || got.PriceStepRule == "" {
		t.Fatalf("BIST tick metadata missing: %+v", got)
	}
}

func TestAttachNextSessionForecastValidationTracksActualAndDirection(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:       true,
		ForecastFor:    "2026-06-22",
		LastClose:      402.50,
		PredictedOpen:  404.00,
		PredictedClose: 405.00,
	}
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC), Open: 408.75, High: 412.75, Low: 400.50, Close: 402.50},
		{Time: time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), Open: 403.50, High: 407.00, Low: 401.00, Close: 406.00},
	}

	got := attachNextSessionForecastValidation(forecast, candles, ohlcv.AssetTypeEquity, "bist_thb_official_bulletin")
	if !got.ActualAvailable || got.ActualOpen != 403.50 || got.ActualClose != 406.00 {
		t.Fatalf("actual prices not attached: %+v", got)
	}
	if got.OpenForecastErrorTL != -0.50 || got.CloseForecastErrorTL != 1.00 {
		t.Fatalf("forecast TL errors wrong: %+v", got)
	}
	if got.OpenAbsErrorPctVsActual != 0.12 || got.CloseAbsErrorPctVsActual != 0.25 {
		t.Fatalf("absolute percent errors wrong: %+v", got)
	}
	if got.OpenDirectionHit == nil || !*got.OpenDirectionHit || got.CloseDirectionHit == nil || !*got.CloseDirectionHit {
		t.Fatalf("direction hit fields wrong: %+v", got)
	}
}

func TestNextSessionForecastDirectionHitUsesPublishTolerance(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:       true,
		ForecastFor:    "2026-06-22",
		LastClose:      100.00,
		PredictedOpen:  100.04,
		PredictedClose: 100.04,
	}
	got := AttachActualToNextSessionForecast(forecast, 100.03, 100.03, "bist_thb_official_bulletin", "")
	if got.OpenDirectionHit == nil || !*got.OpenDirectionHit || got.CloseDirectionHit == nil || !*got.CloseDirectionHit {
		t.Fatalf("sub-tolerance moves should both be flat direction hits: %+v", got)
	}
}

func TestApplyFundamentalContextToNextSessionForecastAddsProfessionalOverlay(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-22",
		LastClose:         100,
		PredictedOpen:     100.40,
		PredictedClose:    100.60,
		OpenChangePct:     0.40,
		CloseChangePct:    0.60,
		ExpectedLow:       98,
		ExpectedHigh:      102,
		DirectionBias:     "yükseliş",
		BiasStrength:      "orta",
		Confidence:        60,
		HistoricalSamples: 20,
		Model:             "atr_gap_intraday_ewma_v1",
		BiasReasons:       []string{"Fiyat EMA20 üzerinde"},
	}
	result := SymbolAnalysis{
		Symbol:              "TEST",
		AssetType:           ohlcv.AssetTypeEquity,
		Currency:            "TL",
		NextSessionForecast: forecast,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {Timeframe: "1D", NextSessionForecast: forecast},
		},
		Professional: professional.Report{
			DataQuality: 88,
			ValueInvesting: value.Report{
				Computed:       true,
				Decision:       "BUY",
				DecisionLabel:  "AL",
				MarginOfSafety: value.MarginOfSafetyReport{Computed: true, BasePct: 24, RequiredPct: 20},
				QualityScore:   78,
				Confidence:     70,
			},
			KAPPDFIngest: professional.KAPPDFIngestSummary{Computed: true, TotalDocuments: 10, AnalysisUsableCount: 8},
			NewsSentiment: professional.EquityNewsSentimentReport{
				Computed:        true,
				RecentItemCount: 3,
				PositiveCount:   2,
				NegativeCount:   0,
				NeutralCount:    1,
				Score:           44,
				Signal:          "positive",
			},
			InvestmentResearch: professional.InvestmentResearchReview{
				Computed: true,
				FinancialQuality: professional.FinancialQualityBridge{
					RedFlags: []string{"net borç/FAVÖK artışı izlenmeli"},
				},
			},
			VAPFreeFloat: vapcontext.FreeFloatReport{
				Computed: true, FreeFloatRatioPct: 31.5, RatioChange20DPP: -0.7, SupplySignal: "azalan arz", LiquidityRisk: "düşük",
			},
			VAPIndexPortfolio: vapcontext.IndexPortfolioReport{
				Computed: true, SelectedIndex: "BIST TEKNOLOJİ", Change1MPct: 3, RelativeMomentum: 1.5,
			},
			TCMBEVDSContext: professional.TCMBEVDSContextReport{
				ForecastImpact: professional.TCMBMacroForecastImpact{
					Computed: true, Direction: "positive", Label: "destekleyici", Severity: "moderate", Confidence: 72, ScoreAdjustment: 3,
				},
			},
		},
	}

	got := ApplyFundamentalContextToNextSessionForecast(result)
	if !strings.Contains(got.NextSessionForecast.Model, "atr_gap_intraday_ewma_fundamental_v2") {
		t.Fatalf("expected fundamental forecast model, got %+v", got.NextSessionForecast)
	}
	if got.NextSessionForecast.PredictedClose <= forecast.PredictedClose {
		t.Fatalf("expected constructive professional overlay to lift close estimate: before=%+v after=%+v", forecast, got.NextSessionForecast)
	}
	reasons := strings.Join(got.NextSessionForecast.BiasReasons, "\n")
	for _, want := range []string{"Temel bağlam: değer yatırım", "KAP/haber duyarlılığı", "Temel bağlam: VAP", "Makro bağlam: TCMB/EVDS"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("missing %q in reasons:\n%s", want, reasons)
		}
	}
	if got.NextSessionForecast.PredictedCloseDirection == "" {
		t.Fatalf("expected explicit close direction after professional overlay: %+v", got.NextSessionForecast)
	}
	if daily := got.Timeframes["1D"]; daily.NextSessionForecast.PredictedClose != got.NextSessionForecast.PredictedClose {
		t.Fatalf("daily forecast must mirror symbol forecast: symbol=%+v daily=%+v", got.NextSessionForecast, daily.NextSessionForecast)
	}
	reapplied := ApplyFundamentalContextToNextSessionForecast(got)
	if reapplied.NextSessionForecast.PredictedClose != got.NextSessionForecast.PredictedClose {
		t.Fatalf("fundamental overlay must be idempotent: first=%+v second=%+v", got.NextSessionForecast, reapplied.NextSessionForecast)
	}
}

func TestApplyNextSessionForecastQualityContextDowngradesFailedTechnicalGate(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:          true,
		ForecastFor:       "2026-06-22",
		LastClose:         402.50,
		PredictedOpen:     404.47,
		PredictedClose:    406.26,
		OpenChangePct:     0.49,
		CloseChangePct:    0.93,
		ExpectedLow:       389.48,
		ExpectedHigh:      415.52,
		Status:            "mathematically_consistent",
		Quality:           "technical_model",
		DirectionBias:     "yükseliş",
		BiasStrength:      "orta",
		Confidence:        67,
		ConfidenceLabel:   "low_to_medium",
		HistoricalSamples: 20,
		Model:             "atr_gap_intraday_ewma_v1",
		BiasReasons:       []string{"MACD histogramı pozitif (momentum yükselen)", "CMF20=-0.137 satış baskısı (karşı sinyal)"},
	}
	result := SymbolAnalysis{
		NextSessionForecast: forecast,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				Indicators: ohlcv.IndicatorSnapshot{
					ChaikinMoneyFlow20: -0.137,
					MACDHistogram:      -5.4714,
				},
				TradePlan: ohlcv.TradePlan{Rejected: true, RiskRewardRatio: 1.28, RejectReason: "risk_reward_too_low"},
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						SignalGate: professional.TechnicalSignalGate{
							Status:             "fail",
							Actionable:         false,
							Score:              38,
							VolumeConfirmed:    false,
							VolumeConfirmation: "hacim teyidi yok",
						},
					},
				},
				NextSessionForecast: forecast,
			},
		},
	}

	got := ApplyNextSessionForecastQualityContext(result).NextSessionForecast
	if got.PredictedClose >= forecast.LastClose || got.PredictedCloseDirection != "düşüş" {
		t.Fatalf("failed bearish technical gate must recalibrate conflicting bullish forecast direction: before=%+v after=%+v", forecast, got)
	}
	if got.DirectionBias != "hafif negatif" || got.BiasStrength != "zayıf-orta" {
		t.Fatalf("forecast label should be downgraded by failed gates: %+v", got)
	}
	if got.Quality != "not_decision_grade" ||
		got.Status != "technical_decision_context_failed" ||
		got.TechnicalDecisionStatus != "failed" ||
		got.TradePlanStatus != "technical_signal_gate_failed" {
		t.Fatalf("forecast technical gate state not downgraded: %+v", got)
	}
	if got.Confidence != 35 || got.ConfidenceLabel != "low" {
		t.Fatalf("forecast quality/confidence not downgraded: %+v", got)
	}
	reasons := strings.Join(got.BiasReasons, "\n")
	for _, want := range []string{"Teknik karar kapısı: durum=failed", "Günlük teknik kapı ham tahmin yönünü düzeltti", "teknik sinyal kapısı geçmedi", "CMF20=-0.137 satış baskısı", "R/R 1.28 < 1.50"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("missing downgrade reason %q in:\n%s", want, reasons)
		}
	}
	if strings.Contains(reasons, "MACD histogramı pozitif") || !strings.Contains(reasons, "Günlük MACD histogramı -5.4714") {
		t.Fatalf("MACD forecast reason must match latest daily indicator:\n%s", reasons)
	}
}

func TestApplyNextSessionForecastQualityContextPreservesHardModelFailure(t *testing.T) {
	forecast := NextSessionForecast{
		Computed:        true,
		ForecastFor:     "2026-06-19",
		LastClose:       410.75,
		PredictedOpen:   407.75,
		PredictedClose:  402.75,
		OpenChangePct:   -0.73,
		CloseChangePct:  -1.95,
		Status:          "model_validation_failed",
		Quality:         "not_decision_grade",
		DirectionBias:   "düşüş",
		BiasStrength:    "validasyon zayıf",
		Confidence:      35,
		ConfidenceLabel: "low",
		Model:           "atr_gap_intraday_ewma_v1_bist_history_analog_knn_v1",
	}
	result := SymbolAnalysis{
		NextSessionForecast: forecast,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				Indicators: ohlcv.IndicatorSnapshot{
					ChaikinMoneyFlow20: -0.141,
					MACDHistogram:      1.2,
				},
				TradePlan: ohlcv.TradePlan{Rejected: true, RiskRewardRatio: 1.2, RejectReason: "risk_reward_too_low"},
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						SignalGate: professional.TechnicalSignalGate{
							Status:             "fail",
							Actionable:         false,
							VolumeConfirmed:    false,
							VolumeConfirmation: "hacim teyidi yok",
						},
					},
				},
				NextSessionForecast: forecast,
			},
		},
	}

	got := ApplyNextSessionForecastQualityContext(result).NextSessionForecast
	if got.Status != "model_validation_failed" || got.Quality != "not_decision_grade" {
		t.Fatalf("hard model failure must be preserved, got %+v", got)
	}
	reasons := strings.Join(got.BiasReasons, "\n")
	if !strings.Contains(reasons, "model karar/emir seviyesi olarak kullanılamaz") {
		t.Fatalf("hard failure reason should stay explicit:\n%s", reasons)
	}
}

func TestNextSessionForecastTechnicalContextRejectsFailedTradePlan(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC), Open: 390, High: 398, Low: 388, Close: 395, Volume: 1_000_000},
		{Time: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), Open: 395, High: 414, Low: 392, Close: 410.75, Volume: 2_000_000},
	}
	forecast := NextSessionForecast{
		Computed:       true,
		LastClose:      410.75,
		PredictedOpen:  413,
		PredictedClose: 415,
		OpenChangePct:  0.55,
		CloseChangePct: 1.03,
		ExpectedLow:    400,
		ExpectedHigh:   420,
		Status:         "mathematically_consistent",
		Quality:        "technical_model",
		DirectionBias:  "yükseliş",
		BiasStrength:   "orta",
		Confidence:     67,
		Model:          "atr_gap_intraday_ewma_v1",
	}
	got := applyNextSessionTechnicalDecisionContext(
		forecast,
		candles,
		ohlcv.IndicatorSnapshot{ATR14: 12, RSI14: 58, MACD: 3, MACDSignal: 2, MACDHistogram: 1, EMA20: 390, EMA50: 380, Supertrend: 385, ChaikinMoneyFlow20: 0.10, StochasticK: 65},
		nil,
		[]ohlcv.PatternResult{{Name: "Bullish Engulfing", Direction: "bullish", Confidence: 0.84, Actionable: true, Tradeable: true, EndIndex: 1}},
		nil,
		supportresistance.Result{},
		ohlcv.TradePlan{Rejected: true, RejectReason: "risk_reward_too_low", RiskRewardRatio: 1.2},
		ohlcv.AssetTypeEquity,
		"ASELS",
	)
	if got.TechnicalDecisionStatus != "failed" || got.Quality != "not_decision_grade" || got.Status != "technical_decision_context_failed" {
		t.Fatalf("failed trade plan must block next-session decision: %+v", got)
	}
	if got.Confidence > 35 {
		t.Fatalf("failed technical context must cap confidence: %+v", got)
	}
	reasons := strings.Join(got.BiasReasons, "\n")
	if !strings.Contains(reasons, "işlem planı reddedildi") || !strings.Contains(reasons, "Teknik karar kapısı") {
		t.Fatalf("missing technical gate evidence:\n%s", reasons)
	}
}

func TestNextSessionForecastUsesFullIndicatorUniverseForDirection(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC), Open: 100, High: 103, Low: 99, Close: 101, Volume: 1_000_000},
		{Time: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), Open: 101, High: 104, Low: 100, Close: 102, Volume: 1_100_000},
	}
	forecast := NextSessionForecast{
		Computed:          true,
		LastClose:         102,
		RawPredictedOpen:  102.8,
		RawPredictedClose: 104,
		PredictedOpen:     102.8,
		PredictedClose:    104,
		OpenChangePct:     0.78,
		CloseChangePct:    1.96,
		RawExpectedLow:    100,
		RawExpectedHigh:   105,
		ExpectedLow:       100,
		ExpectedHigh:      105,
		Status:            "mathematically_consistent",
		Quality:           "technical_model",
		DirectionBias:     "yükseliş",
		BiasStrength:      "orta",
		Confidence:        67,
		Model:             "separate_open_gap_close_intraday_v2",
	}
	got := applyNextSessionTechnicalDecisionContext(
		forecast,
		candles,
		ohlcv.IndicatorSnapshot{ATR14: 2, RSI14: 65, MACD: 1, MACDSignal: 0.5, MACDHistogram: 0.5, EMA20: 100, EMA50: 99, Supertrend: 98},
		[]ohlcv.IndicatorResult{
			{Name: "RSI", Signal: "bearish", Confidence: 0.86, Computed: true},
			{Name: "MACD Bearish Divergence", Signal: "bearish", Confidence: 0.82, Computed: true},
			{Name: "CMF", Signal: "bearish", Confidence: 0.78, Computed: true},
			{Name: "Proxy", Signal: "bullish", Confidence: 1, Computed: false},
		},
		nil,
		nil,
		supportresistance.Result{},
		ohlcv.TradePlan{},
		ohlcv.AssetTypeEquity,
		"ASELS",
	)
	if got.IndicatorConsensus != "bearish" || got.CloseChangePct >= 0 {
		t.Fatalf("full bearish indicator universe should recalibrate bullish point forecast: %+v", got)
	}
	reasons := strings.Join(got.BiasReasons, "\n")
	if !strings.Contains(reasons, "Full indikatör evreni etkisi") || !strings.Contains(got.Model, "full_signal_universe_v1") {
		t.Fatalf("full indicator universe evidence/model marker missing:\nmodel=%s\n%s", got.Model, reasons)
	}
}

func TestNextSessionForecastUsesPatternCandidatesInCalculation(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC), Open: 100, High: 103, Low: 99, Close: 101, Volume: 1_000_000},
		{Time: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), Open: 101, High: 104, Low: 100, Close: 102, Volume: 1_100_000},
	}
	forecast := NextSessionForecast{
		Computed:          true,
		LastClose:         102,
		RawPredictedOpen:  102.6,
		RawPredictedClose: 103.5,
		PredictedOpen:     102.6,
		PredictedClose:    103.5,
		OpenChangePct:     0.59,
		CloseChangePct:    1.47,
		RawExpectedLow:    100,
		RawExpectedHigh:   105,
		ExpectedLow:       100,
		ExpectedHigh:      105,
		Status:            "mathematically_consistent",
		Quality:           "technical_model",
		DirectionBias:     "yükseliş",
		BiasStrength:      "orta",
		Confidence:        65,
		Model:             "separate_open_gap_close_intraday_v2",
	}
	got := applyNextSessionTechnicalDecisionContext(
		forecast,
		candles,
		ohlcv.IndicatorSnapshot{ATR14: 2},
		nil,
		nil,
		[]ohlcv.PatternResult{{
			Name:                 "Bearish RSI Divergence",
			Direction:            "bearish",
			Confidence:           0.80,
			CalibratedConfidence: 0.78,
			SignalScore:          0.82,
			EndIndex:             1,
			RejectionReasons:     []string{"calibrated_confidence_below_threshold"},
		}},
		supportresistance.Result{},
		ohlcv.TradePlan{},
		ohlcv.AssetTypeEquity,
		"ASELS",
	)
	if got.PatternConsensus != "bearish" || got.CloseChangePct >= 0 {
		t.Fatalf("bearish pattern candidate should enter next-session calculation: %+v", got)
	}
	reasons := strings.Join(got.BiasReasons, "\n")
	if !strings.Contains(reasons, "Full formasyon evreni etkisi") {
		t.Fatalf("pattern candidate evidence missing:\n%s", reasons)
	}
}

func TestIntegratedOverallScoreAppliesOnlyEligibleTCMBAdjustment(t *testing.T) {
	baseReport := professional.Report{
		DataQuality: 100,
		Valuation: professional.ValuationAnalysis{
			Ratios: map[string]float64{},
		},
	}
	withoutMacro := integratedOverallScore(50, baseReport, ohlcv.AssetTypeEquity)

	eligible := baseReport
	eligible.TCMBEVDSContext = professional.TCMBEVDSContextReport{
		ScoreEligible:   true,
		ScoreAdjustment: 5,
	}
	withMacro := integratedOverallScore(50, eligible, ohlcv.AssetTypeEquity)
	if math.Abs((withMacro-withoutMacro)-5) > 1e-9 {
		t.Fatalf("eligible TCMB adjustment not applied: base=%.2f macro=%.2f", withoutMacro, withMacro)
	}

	ineligible := eligible
	ineligible.TCMBEVDSContext.ScoreEligible = false
	if got := integratedOverallScore(50, ineligible, ohlcv.AssetTypeEquity); got != withoutMacro {
		t.Fatalf("ineligible TCMB adjustment must not affect score: base=%.2f got=%.2f", withoutMacro, got)
	}

	bounded := eligible
	bounded.TCMBEVDSContext.ScoreAdjustment = 30
	if got := integratedOverallScore(50, bounded, ohlcv.AssetTypeEquity); math.Abs((got-withoutMacro)-8) > 1e-9 {
		t.Fatalf("TCMB adjustment must be capped at 8 points: base=%.2f got=%.2f", withoutMacro, got)
	}
}

func TestPriceQualityInspectionFailureStillBlocksReport(t *testing.T) {
	report := priceQualityInspectionFailure("FENER", errors.New("boom"))

	if report == nil {
		t.Fatal("expected fallback price quality report")
	}
	if report.Symbol != "FENER" {
		t.Fatalf("symbol = %q", report.Symbol)
	}
	if report.Status != pricequality.StatusMissingPriceData {
		t.Fatalf("status = %q", report.Status)
	}
	if report.ReadyForVerifiedClose {
		t.Fatal("fallback report must not be verified close ready")
	}
	if len(report.BlockingReasons) == 0 || !strings.Contains(report.BlockingReasons[0], "price_quality_inspection_failed") {
		t.Fatalf("blocking reasons missing inspection failure: %+v", report.BlockingReasons)
	}
}

func TestBISTBulletinOfficialClosePromotesPriceQualityToVerified(t *testing.T) {
	weekend := time.Date(2026, 6, 20, 1, 25, 0, 0, time.UTC)
	result := SymbolAnalysis{
		Symbol:    "ASELS",
		AssetType: ohlcv.AssetTypeEquity,
		PriceQuality: &pricequality.SymbolReport{
			Symbol:                "ASELS",
			Status:                pricequality.StatusProvisionalLastPrice,
			ReadyForDecision:      true,
			ReadyForVerifiedClose: false,
			LatestTradingDate:     "2026-06-20",
			MissingFields:         []string{"official_final_close"},
			BlockingReasons:       []string{"official_final_close_missing"},
			Candidates: []pricequality.CloseCandidate{
				{
					Source:      "market_ws",
					SourceType:  "market_ws",
					Close:       402.50,
					Timestamp:   &weekend,
					FetchedAt:   &weekend,
					TradingDate: "2026-06-20",
				},
			},
		},
		BISTBulletin: BISTBulletinContext{
			Computed: true,
			LatestRecord: datasource.DailyBulletinRecord{
				Symbol: "ASELS", TradingDate: "2026-06-19", Close: 402.50,
				SourceFormat: "csv", SourcePath: "data/bist/unprocessed/bulten_verileri/2026/06/20260619_s1/extracted/thb202606191.csv",
			},
			OfficialCloseConfirmed: true,
		},
	}

	got := reconcilePriceQualityWithBISTBulletin(result.PriceQuality, result)
	if got == nil || !got.ReadyForVerifiedClose || got.Status != pricequality.StatusReadyForVerifiedClose {
		t.Fatalf("BIST bulletin close should verify price quality: %+v", got)
	}
	if got.SelectedClose == nil || got.SelectedClose.SourceType != "official_final_close" || got.SelectedClose.Source != "bist_thb_official_bulletin" {
		t.Fatalf("selected close should be official BIST bulletin: %+v", got.SelectedClose)
	}
	if got.LatestTradingDate != "2026-06-19" {
		t.Fatalf("latest date = %s, want official BIST date", got.LatestTradingDate)
	}
	if containsString(got.BlockingReasons, "official_final_close_missing") {
		t.Fatalf("official close blocker remained: %+v", got.BlockingReasons)
	}
}

func TestUnverifiedCloseBlocksTechnicalSignalGate(t *testing.T) {
	// Missing price data triggers hard-fail: gate blocked and score capped.
	result := SymbolAnalysis{
		AssetType: ohlcv.AssetTypeEquity,
		PriceQuality: &pricequality.SymbolReport{
			Status:                pricequality.StatusMissingPriceData,
			ReadyForVerifiedClose: false,
			BlockingReasons:       []string{"official_final_close_missing"},
		},
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				Score:     88,
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						Summary: "88/100 technical evidence score",
						SignalGate: professional.TechnicalSignalGate{
							Status:         "pass",
							Actionable:     true,
							Score:          92,
							Label:          "Aktif işlem sinyali kanıt kapısından geçti",
							PriceStructure: "giriş 10.00-10.20, stop 9.50",
						},
					},
				},
			},
		},
	}

	got := applyPriceQualityToTechnicalGates(result)
	tf := got.Timeframes["1D"]
	gate := tf.Professional.Technical.SignalGate
	if tf.Score > 45 {
		t.Fatalf("missing price data should cap timeframe score, got %.2f", tf.Score)
	}
	if gate.Status != "fail" || gate.Actionable {
		t.Fatalf("missing price data must block technical signal gate: %+v", gate)
	}
	if gate.Score > 45 {
		t.Fatalf("missing price data should cap gate score, got %.2f", gate.Score)
	}
	if !containsString(gate.Blockers, "price_quality: official_final_close_missing") {
		t.Fatalf("price quality blocker missing: %+v", gate.Blockers)
	}
	if !containsString(tf.Professional.Technical.Guardrails, "official_final_close_missing") {
		t.Fatalf("guardrail missing: %+v", tf.Professional.Technical.Guardrails)
	}
}

func TestProvisionalPriceAddsWarningButKeepsGate(t *testing.T) {
	// Provisional (stale but not missing/conflicting) price: gate stays open, only warning added.
	result := SymbolAnalysis{
		AssetType: ohlcv.AssetTypeEquity,
		PriceQuality: &pricequality.SymbolReport{
			Status:                pricequality.StatusProvisionalLastPrice,
			ReadyForVerifiedClose: false,
			BlockingReasons:       []string{"official_final_close_missing"},
		},
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				Score:     88,
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						Summary: "88/100 technical evidence score",
						SignalGate: professional.TechnicalSignalGate{
							Status:     "pass",
							Actionable: true,
							Score:      92,
						},
					},
				},
			},
		},
	}

	got := applyPriceQualityToTechnicalGates(result)
	tf := got.Timeframes["1D"]
	gate := tf.Professional.Technical.SignalGate
	if tf.Score != 88 {
		t.Fatalf("provisional price must not cap score, got %.2f", tf.Score)
	}
	if gate.Status != "pass" || !gate.Actionable {
		t.Fatalf("provisional price must not block technical gate: %+v", gate)
	}
	if !containsString(tf.Professional.Technical.Guardrails, "official_final_close_missing") {
		t.Fatalf("guardrail missing: %+v", tf.Professional.Technical.Guardrails)
	}
}

func TestNormalizeYTDWindowFiltersToCurrentYear(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2025, 12, 30, 6, 0, 0, 0, time.UTC), Close: 10},
		{Time: time.Date(2026, 1, 2, 6, 0, 0, 0, time.UTC), Close: 11},
		{Time: time.Date(2026, 6, 19, 6, 0, 0, 0, time.UTC), Close: 15},
	}

	got := normalizeTimeframeWindow(candles, "YTD")
	if len(got) != 2 {
		t.Fatalf("YTD candle count = %d, want 2", len(got))
	}
	if got[0].Time.Year() != 2026 || got[0].Close != 11 {
		t.Fatalf("YTD first candle = %+v", got[0])
	}
	if daily := normalizeTimeframeWindow(candles, "1D"); len(daily) != len(candles) {
		t.Fatalf("1D should not be filtered, got %d candles", len(daily))
	}
}

func TestTimeframeFetchLimitExpandsContextWindows(t *testing.T) {
	if got := timeframeFetchLimit("YTD", 260); got < 320 {
		t.Fatalf("YTD fetch limit = %d, want at least 320", got)
	}
	if got := timeframeFetchLimit("ALL", 260); got != 260 {
		t.Fatalf("ALL fetch limit = %d, want standard limit 260", got)
	}
	if got := timeframeFetchLimit("1D", 260); got != 260 {
		t.Fatalf("1D fetch limit = %d, want 260", got)
	}
}

func TestChartCandlesForRenderLimitsMonthlyHistory(t *testing.T) {
	candles := make([]ohlcv.Candle, 260)
	start := time.Date(2004, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range candles {
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, i, 0),
			Open:   float64(i + 1),
			High:   float64(i + 2),
			Low:    float64(i),
			Close:  float64(i + 1),
			Volume: 1000,
		}
	}

	got := chartCandlesForRender(candles, "1M", formations.DrawingObjects{})

	if len(got) != 72 {
		t.Fatalf("monthly render candle count = %d, want 72", len(got))
	}
	if !got[0].Time.Equal(candles[188].Time) || !got[len(got)-1].Time.Equal(candles[len(candles)-1].Time) {
		t.Fatalf("monthly render should keep the latest window, got %s..%s", got[0].Time.Format("2006-01-02"), got[len(got)-1].Time.Format("2006-01-02"))
	}
}

func TestAnalyzeCryptoSymbolUsesCryptoProfessionalContext(t *testing.T) {
	engine := NewEngine(datasource.NewMockProvider(), testRenderer{}, EngineOptions{
		Timeframes: []string{"1D"},
		Limit:      260,
	})
	result, err := engine.AnalyzeSymbol(context.Background(), SymbolRequest{Symbol: "BTC"})
	if err != nil {
		t.Fatalf("analyze crypto symbol: %v", err)
	}
	if result.AssetType != ohlcv.AssetTypeCrypto {
		t.Fatalf("asset type = %s, want crypto", result.AssetType)
	}
	if result.Symbol != "BTCUSDT" || result.Currency != "USDT" {
		t.Fatalf("unexpected crypto metadata: %+v", result)
	}
	if result.Professional.Company.Sector != "Crypto Assets" {
		t.Fatalf("sector = %s", result.Professional.Company.Sector)
	}
	if result.Professional.Peers.ValuationSignal != "not_applicable" {
		t.Fatalf("valuation signal = %s", result.Professional.Peers.ValuationSignal)
	}
	if result.Professional.Valuation.Ratios["PE"] != 0 || result.Professional.Valuation.Ratios["PB"] != 0 {
		t.Fatalf("equity valuation ratios should not be populated: %+v", result.Professional.Valuation.Ratios)
	}
	if math.Abs(result.OverallScore-result.Timeframes["1D"].Score) > 0.000001 {
		t.Fatalf("crypto overall score should stay technical: overall=%.4f tf=%.4f", result.OverallScore, result.Timeframes["1D"].Score)
	}
}

func TestAnalyzeCommoditySymbolUsesCommodityProfessionalContext(t *testing.T) {
	engine := NewEngine(datasource.NewMockProvider(), testRenderer{}, EngineOptions{
		Timeframes: []string{"1D"},
		Limit:      260,
	})
	result, err := engine.AnalyzeSymbol(context.Background(), SymbolRequest{Symbol: "XAU"})
	if err != nil {
		t.Fatalf("analyze commodity symbol: %v", err)
	}
	if result.AssetType != ohlcv.AssetTypeCommodity {
		t.Fatalf("asset type = %s, want commodity", result.AssetType)
	}
	if result.Symbol != "XAUUSD" || result.Currency != "USD" {
		t.Fatalf("unexpected commodity metadata: %+v", result)
	}
	if result.Professional.Company.Sector != "Precious Metals" {
		t.Fatalf("sector = %s", result.Professional.Company.Sector)
	}
	if result.Professional.Peers.ValuationSignal != "not_applicable" {
		t.Fatalf("valuation signal = %s", result.Professional.Peers.ValuationSignal)
	}
	if result.Professional.Valuation.Ratios["PE"] != 0 || result.Professional.Valuation.Ratios["PB"] != 0 {
		t.Fatalf("equity valuation ratios should not be populated: %+v", result.Professional.Valuation.Ratios)
	}
	if math.Abs(result.OverallScore-result.Timeframes["1D"].Score) > 0.000001 {
		t.Fatalf("commodity overall score should stay technical: overall=%.4f tf=%.4f", result.OverallScore, result.Timeframes["1D"].Score)
	}
}

func TestFirstTrendlineStartIndexUsesMarketSessionDate(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 15, 21, 0, 0, 0, time.UTC)},
		{Time: time.Date(2026, 6, 16, 21, 0, 0, 0, time.UTC)},
	}
	drawings := formations.DrawingObjects{Lines: []formations.LineObject{
		{Type: "trendline", StartTime: "2026-06-17", StartPrice: 100, EndTime: "2026-06-18", EndPrice: 110},
	}}
	got, ok := firstTrendlineStartIndex(candles, drawings)
	if !ok || got != 1 {
		t.Fatalf("firstTrendlineStartIndex() = %d, %v; want 1, true", got, ok)
	}
}

func TestAnalyzeSymbolContinuesWhenOneTimeframeFails(t *testing.T) {
	engine := NewEngine(partialFailProvider{delegate: datasource.NewMockProvider(), failTimeframe: "1D"}, testRenderer{}, EngineOptions{
		Timeframes: []string{"1D", "1W"},
		Limit:      260,
	})
	result, err := engine.AnalyzeSymbol(context.Background(), SymbolRequest{Symbol: "TEST"})
	if err != nil {
		t.Fatalf("analyze symbol: %v", err)
	}
	if _, ok := result.Timeframes["1D"]; ok {
		t.Fatalf("1D should have failed")
	}
	if _, ok := result.Timeframes["1W"]; !ok {
		t.Fatalf("1W should have succeeded")
	}
	if result.TimeframeErrors["1D"] == "" {
		t.Fatalf("expected 1D timeframe error: %+v", result.TimeframeErrors)
	}
	if result.OverallScore <= 0 {
		t.Fatalf("expected score from successful timeframe, got %.2f", result.OverallScore)
	}
}

func TestAnalyzeSymbolRejectsTimeframeWithLargeCalendarGap(t *testing.T) {
	engine := NewEngine(gapProvider{delegate: datasource.NewMockProvider()}, testRenderer{}, EngineOptions{
		Timeframes: []string{"1D", "1M"},
		Limit:      260,
	})
	result, err := engine.AnalyzeSymbol(context.Background(), SymbolRequest{Symbol: "TEST"})
	if err != nil {
		t.Fatalf("analyze symbol: %v", err)
	}
	if _, ok := result.Timeframes["1M"]; ok {
		t.Fatalf("1M should have been rejected for temporal gap")
	}
	if _, ok := result.Timeframes["1D"]; !ok {
		t.Fatalf("1D should still succeed")
	}
	if !strings.Contains(result.TimeframeErrors["1M"], "candle temporal gap") {
		t.Fatalf("1M error = %q, want temporal gap", result.TimeframeErrors["1M"])
	}
}

func TestBankProfessionalDetectionDoesNotTreatREITFinancialSectorAsBank(t *testing.T) {
	pro := professional.Report{
		Company: professional.CompanyProfile{
			Sector:   "MALİ KURULUŞLAR",
			Industry: "GAYRİMENKUL YATIRIM ORTAKLIKLARI",
		},
		SectorFinancials: professional.SectorFinancialAnalysis{
			Profile:      "reit_nav",
			ProfileLabel: "Gayrimenkul yatırım ortaklığı",
		},
	}
	if isBankProfessionalReport(pro) {
		t.Fatalf("REIT under financial sector must not be treated as bank: %+v", pro)
	}
}

func TestPatternOverlayCoverageBlocksInvalidActivePatternWindow(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 16, 6, 0, 0, 0, time.UTC), Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 1000},
		{Time: time.Date(2026, 6, 17, 6, 0, 0, 0, time.UTC), Open: 10.5, High: 12, Low: 10, Close: 11.5, Volume: 1200},
	}
	drawn, notDrawn, blockers := patternOverlayCoverage(candles, []ohlcv.PatternResult{
		{Name: "valid_pattern", Direction: "bullish", StartIndex: 0, EndIndex: 1, StartTime: candles[0].Time, EndTime: candles[1].Time},
		{Name: "invalid_window", Direction: "bullish", StartIndex: 1, EndIndex: 4},
	})

	if drawn != 1 || notDrawn != 1 {
		t.Fatalf("overlay coverage = drawn %d notDrawn %d, want 1/1", drawn, notDrawn)
	}
	if len(blockers) != 1 || !strings.Contains(blockers[0], "invalid_window") {
		t.Fatalf("missing invalid pattern blocker: %+v", blockers)
	}
}

type testRenderer struct{}

func (testRenderer) RenderPNG(context.Context, chart.RenderInput) ([]byte, error) {
	return []byte("png"), nil
}

func makeVolatileForecastTestCandles(count int) []ohlcv.Candle {
	candles := make([]ohlcv.Candle, 0, count)
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	closePrice := 100.0
	for i := 0; i < count; i++ {
		shock := 1.0
		if i%4 == 0 {
			shock = -1.0
		}
		gap := shock * (0.008 + 0.002*float64(i%3))
		intraday := -shock * (0.025 + 0.004*float64(i%5))
		open := closePrice * (1 + gap)
		nextClose := open * (1 + intraday)
		high := math.Max(open, nextClose) * 1.018
		low := math.Min(open, nextClose) * 0.982
		volume := 1_000_000.0 * (1 + 0.20*float64(i%7))
		candles = append(candles, ohlcv.Candle{
			Time:           start.AddDate(0, 0, i),
			Open:           open,
			High:           high,
			Low:            low,
			Close:          nextClose,
			Volume:         volume,
			AdjustedOpen:   open,
			AdjustedHigh:   high,
			AdjustedLow:    low,
			AdjustedClose:  nextClose,
			AdjustedVolume: volume,
		})
		closePrice = nextClose
	}
	return candles
}

type recordingBulletinRangeProvider struct {
	toDate string
}

func (p *recordingBulletinRangeProvider) FetchDailyBulletinRecords(context.Context, string, int) ([]datasource.DailyBulletinRecord, error) {
	return []datasource.DailyBulletinRecord{{Symbol: "ASELS", TradingDate: "2026-06-18", Close: 394.75}}, nil
}

func (p *recordingBulletinRangeProvider) FetchDailyBulletinRecordsRange(_ context.Context, symbol, fromDate, toDate string, limit int) ([]datasource.DailyBulletinRecord, error) {
	p.toDate = toDate
	return []datasource.DailyBulletinRecord{{Symbol: symbol, TradingDate: toDate, Close: 394.75}}, nil
}

type partialFailProvider struct {
	delegate      datasource.MarketDataProvider
	failTimeframe string
}

func (p partialFailProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	return p.delegate.SearchSymbol(ctx, symbol)
}

func (p partialFailProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if timeframe == p.failTimeframe {
		return nil, errors.New("forced timeframe failure")
	}
	return p.delegate.FetchOHLCV(ctx, instrument, timeframe, limit)
}

type gapProvider struct {
	delegate datasource.MarketDataProvider
}

func (p gapProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	return p.delegate.SearchSymbol(ctx, symbol)
}

func (p gapProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if timeframe != "1M" {
		return p.delegate.FetchOHLCV(ctx, instrument, timeframe, limit)
	}
	start := time.Date(2020, 1, 1, 6, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, 0, 13)
	for i := 0; i < 12; i++ {
		price := 10 + float64(i)
		candles = append(candles, ohlcv.Candle{
			Time:   start.AddDate(0, i, 0),
			Open:   price,
			High:   price + 1,
			Low:    price - 1,
			Close:  price + 0.5,
			Volume: 1000,
		})
	}
	candles = append(candles, ohlcv.Candle{
		Time:   start.AddDate(4, 0, 0),
		Open:   30,
		High:   31,
		Low:    29,
		Close:  30.5,
		Volume: 1000,
	})
	return candles, nil
}

func assertAnalysisTradePlanSane(t *testing.T, tf TimeframeAnalysis) {
	t.Helper()
	plan := tf.TradePlan
	if plan.EntryMin <= 0 || plan.EntryMax <= 0 || plan.StopLoss <= 0 || plan.TakeProfit1 <= 0 || plan.TakeProfit2 <= 0 {
		t.Fatalf("trade plan contains non-positive levels: %+v", plan)
	}
	if plan.EntryMin > plan.EntryMax {
		t.Fatalf("invalid entry range: %+v", plan)
	}
	switch plan.Direction {
	case "long":
		if plan.StopLoss >= plan.EntryMin || plan.TakeProfit1 <= plan.EntryMax || plan.TakeProfit2 <= plan.TakeProfit1 {
			t.Fatalf("invalid long trade plan: %+v", plan)
		}
	case "short":
		if plan.StopLoss <= plan.EntryMax || plan.TakeProfit1 >= plan.EntryMin || plan.TakeProfit2 >= plan.TakeProfit1 {
			t.Fatalf("invalid short trade plan: %+v", plan)
		}
	default:
		t.Fatalf("unexpected trade direction %q", plan.Direction)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
