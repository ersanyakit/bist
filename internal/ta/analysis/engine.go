// internal/ta/analysis/engine.go
package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/domain/pricequality"
	"hissebot/internal/ta/chart"
	"hissebot/internal/ta/contrarian"
	"hissebot/internal/ta/corporateactions"
	"hissebot/internal/ta/datasource"
	"hissebot/internal/ta/fintradebench"
	"hissebot/internal/ta/forecastpolicy"
	"hissebot/internal/ta/formations"
	"hissebot/internal/ta/indicators"
	"hissebot/internal/ta/investorqa"
	"hissebot/internal/ta/localize"
	taml "hissebot/internal/ta/ml"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/patterns"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/risk"
	"hissebot/internal/ta/supportresistance"
	"hissebot/pkg/mathutil"
)

type Engine struct {
	provider datasource.MarketDataProvider
	renderer chart.ChartRenderer
	options  EngineOptions
}

var chartRenderDateLocation = time.FixedZone("TRT", 3*60*60)
var errPriceQualityProviderNotConfigured = errors.New("price quality provider not configured")

type PriceQualityProvider interface {
	InspectSymbol(ctx context.Context, symbol string) (*pricequality.SymbolReport, error)
}

type FormationsProvider interface {
	LoadTickerSnapshot(ctx context.Context, symbol string) (any, error)
}

type EngineOptions struct {
	Timeframes               []string
	Limit                    int
	EquitiesDir              string
	DataMode                 string
	BenchmarkSymbol          string
	ValuationAssumptionsFile string
	MacroGDPFile             string
	TCMBDir                  string
	TCMBEVDSDir              string
	VAPDir                   string
	VAPIndexPortfolioFile    string
	MarketSnapshotFile       string
	AsOf                     time.Time
	PortfolioValue           float64
	RiskPerTradePct          float64
	PeerLimit                int
	SkipKAPPDFIngest         bool
	PriceQualityProvider     PriceQualityProvider
	FormationsProvider       FormationsProvider
}

type SymbolRequest struct {
	Symbol      string
	CompanyName string
	Currency    string
	AssetType   string
}

type SymbolAnalysis struct {
	Symbol                  string                       `json:"symbol"`
	Exchange                string                       `json:"exchange,omitempty"`
	AssetType               string                       `json:"asset_type"`
	CompanyName             string                       `json:"company_name"`
	AnalysisDate            string                       `json:"analysis_date"`
	Currency                string                       `json:"currency"`
	Timeframes              map[string]TimeframeAnalysis `json:"timeframes"`
	TimeframeErrors         map[string]string            `json:"timeframe_errors,omitempty"`
	OverallScore            float64                      `json:"overall_score"`
	OverallBias             string                       `json:"overall_bias"`
	MTFAlignment            string                       `json:"mtf_alignment"` // "aligned", "mixed", "conflicting"
	NextSessionForecast     NextSessionForecast          `json:"next_session_forecast"`
	MLForecast              taml.ForecastReport          `json:"ml_forecast"`
	Professional            professional.Report          `json:"professional"`
	Quant                   QuantAnalysis                `json:"quant"`
	StatEconomic            StatEconomicAnalysis         `json:"stat_economic"`
	Advanced                AdvancedAnalysis             `json:"advanced_analysis"`
	FinTradeBench           fintradebench.Report         `json:"fintradebench"`
	Behavioral              contrarian.Report            `json:"behavioral"`
	InvestorQA              investorqa.Report            `json:"investor_qa"`
	MatriksFormations       any                          `json:"matriks_formations,omitempty"`
	PriceQuality            *pricequality.SymbolReport   `json:"price_quality,omitempty"`
	BISTBulletin            BISTBulletinContext          `json:"bist_bulletin,omitempty"`
	InstitutionalValidation InstitutionalValidation      `json:"institutional_validation"`
	DecisionClassification  DecisionClassification       `json:"decision_classification"`
	DecisionSupport         *DecisionSupportReport       `json:"decision_support,omitempty"`
	Disclaimer              string                       `json:"disclaimer"`
	Charts                  map[string][]byte            `json:"-"`
}

type BISTBulletinContext struct {
	Computed                   bool                           `json:"computed"`
	Source                     string                         `json:"source,omitempty"`
	RecordCount                int                            `json:"record_count,omitempty"`
	CoverageStart              string                         `json:"coverage_start,omitempty"`
	CoverageEnd                string                         `json:"coverage_end,omitempty"`
	LatestRecord               datasource.DailyBulletinRecord `json:"latest_record,omitempty"`
	ForecastActualAvailable    bool                           `json:"forecast_actual_available"`
	ForecastActualRecord       datasource.DailyBulletinRecord `json:"forecast_actual_record,omitempty"`
	OfficialCloseConfirmed     bool                           `json:"official_close_confirmed"`
	OfficialCloseDeltaPct      float64                        `json:"official_close_delta_pct,omitempty"`
	LatestObservedSpreadBps    float64                        `json:"latest_observed_spread_bps,omitempty"`
	LatestOpeningSessionVolume float64                        `json:"latest_opening_session_volume,omitempty"`
	LatestClosingSessionVolume float64                        `json:"latest_closing_session_volume,omitempty"`
	LatestVWAP                 float64                        `json:"latest_vwap,omitempty"`
	Warnings                   []string                       `json:"warnings,omitempty"`
	records                    []datasource.DailyBulletinRecord
}

type TimeframeAnalysis struct {
	Timeframe           string                         `json:"timeframe"`
	Candles             []ohlcv.Candle                 `json:"candles"`
	LastClose           float64                        `json:"last_close"`
	LastVolume          float64                        `json:"last_volume"`
	CandleCount         int                            `json:"candle_count"`
	Indicators          ohlcv.IndicatorSnapshot        `json:"indicators"`
	IndicatorSignals    []ohlcv.IndicatorResult        `json:"indicator_signals"`
	Patterns            []ohlcv.PatternResult          `json:"patterns"`
	PatternCandidates   []ohlcv.PatternResult          `json:"pattern_candidates,omitempty"`
	PatternScans        []ohlcv.PatternScanResult      `json:"pattern_scans"`
	SupportLevels       []ohlcv.SupportResistanceLevel `json:"support_levels"`
	ResistanceLevels    []ohlcv.SupportResistanceLevel `json:"resistance_levels"`
	NearestSupport      *ohlcv.SupportResistanceLevel  `json:"nearest_support,omitempty"`
	NearestResistance   *ohlcv.SupportResistanceLevel  `json:"nearest_resistance,omitempty"`
	TradePlan           ohlcv.TradePlan                `json:"trade_plan"`
	Geometry            formations.Result              `json:"geometry"`
	Professional        professional.TimeframeReport   `json:"professional"`
	TrendBias           string                         `json:"trend_bias"`
	Score               float64                        `json:"score"`
	NextSessionForecast NextSessionForecast            `json:"next_session_forecast"`
}

type NextSessionForecast struct {
	Computed                        bool                        `json:"computed"`
	ForecastFor                     string                      `json:"forecast_for,omitempty"`
	LastClose                       float64                     `json:"last_close,omitempty"`
	PredictedOpen                   float64                     `json:"predicted_open,omitempty"`
	PredictedClose                  float64                     `json:"predicted_close,omitempty"`
	PointForecastPublishable        bool                        `json:"point_forecast_publishable"`
	PointForecastStatus             string                      `json:"point_forecast_status,omitempty"`
	PointForecastSuppressionReason  string                      `json:"point_forecast_suppression_reason,omitempty"`
	PublishedPredictedOpen          *float64                    `json:"published_predicted_open,omitempty"`
	PublishedPredictedClose         *float64                    `json:"published_predicted_close,omitempty"`
	RawPredictedOpen                float64                     `json:"raw_predicted_open,omitempty"`
	RawPredictedClose               float64                     `json:"raw_predicted_close,omitempty"`
	TradablePredictedOpen           float64                     `json:"tradable_predicted_open,omitempty"`
	TradablePredictedClose          float64                     `json:"tradable_predicted_close,omitempty"`
	OpenChangePct                   float64                     `json:"open_change_pct,omitempty"`
	CloseChangePct                  float64                     `json:"close_change_pct,omitempty"`
	PredictedOpenDirection          string                      `json:"predicted_open_direction,omitempty"`
	PredictedCloseDirection         string                      `json:"predicted_close_direction,omitempty"`
	DirectionTolerancePct           float64                     `json:"direction_tolerance_pct,omitempty"`
	ExpectedLow                     float64                     `json:"expected_low"`
	ExpectedHigh                    float64                     `json:"expected_high"`
	DecisionIntervalLow             float64                     `json:"decision_interval_low,omitempty"`
	DecisionIntervalHigh            float64                     `json:"decision_interval_high,omitempty"`
	DecisionIntervalWidthPct        float64                     `json:"decision_interval_width_pct,omitempty"`
	DecisionIntervalStatus          string                      `json:"decision_interval_status,omitempty"`
	DecisionIntervalReason          string                      `json:"decision_interval_reason,omitempty"`
	OpenP10                         float64                     `json:"open_p10,omitempty"`
	OpenP50                         float64                     `json:"open_p50,omitempty"`
	OpenP90                         float64                     `json:"open_p90,omitempty"`
	CloseP10                        float64                     `json:"close_p10,omitempty"`
	CloseP50                        float64                     `json:"close_p50,omitempty"`
	CloseP90                        float64                     `json:"close_p90,omitempty"`
	UpsideProbabilityPct            float64                     `json:"upside_probability_pct,omitempty"`
	FlatProbabilityPct              float64                     `json:"flat_probability_pct,omitempty"`
	DownsideProbabilityPct          float64                     `json:"downside_probability_pct,omitempty"`
	ForecastDistributionSamples     int                         `json:"forecast_distribution_samples,omitempty"`
	InvalidationLevel               float64                     `json:"invalidation_level,omitempty"`
	InvalidationReason              string                      `json:"invalidation_reason,omitempty"`
	RawExpectedLow                  float64                     `json:"raw_expected_low,omitempty"`
	RawExpectedHigh                 float64                     `json:"raw_expected_high,omitempty"`
	TradableExpectedLow             float64                     `json:"tradable_expected_low,omitempty"`
	TradableExpectedHigh            float64                     `json:"tradable_expected_high,omitempty"`
	TickSize                        float64                     `json:"tick_size,omitempty"`
	RoundingMethod                  string                      `json:"rounding_method,omitempty"`
	PriceStepRule                   string                      `json:"price_step_rule,omitempty"`
	OpeningAuctionEquilibriumPrice  *float64                    `json:"opening_auction_equilibrium_price"`
	OrderBookImbalance              *float64                    `json:"order_book_imbalance"`
	AuctionVolumePressure           *float64                    `json:"auction_volume_pressure"`
	MicrostructureAdjustment        *float64                    `json:"microstructure_adjustment"`
	ValidationStatus                string                      `json:"validation_status,omitempty"`
	ValidationSource                string                      `json:"validation_source,omitempty"`
	ActualAvailable                 bool                        `json:"actual_available"`
	ActualOpen                      float64                     `json:"actual_open,omitempty"`
	ActualClose                     float64                     `json:"actual_close,omitempty"`
	ActualSource                    string                      `json:"actual_source,omitempty"`
	ActualSourcePath                string                      `json:"actual_source_path,omitempty"`
	ActualOpenErrorPct              float64                     `json:"actual_open_error_pct,omitempty"`
	ActualCloseErrorPct             float64                     `json:"actual_close_error_pct,omitempty"`
	OpenForecastErrorTL             float64                     `json:"open_forecast_error_tl,omitempty"`
	CloseForecastErrorTL            float64                     `json:"close_forecast_error_tl,omitempty"`
	OpenAbsErrorPctVsActual         float64                     `json:"open_abs_error_pct_vs_actual,omitempty"`
	CloseAbsErrorPctVsActual        float64                     `json:"close_abs_error_pct_vs_actual,omitempty"`
	OpenAbsErrorPctVsPreviousClose  float64                     `json:"open_abs_error_pct_vs_previous_close,omitempty"`
	CloseAbsErrorPctVsPreviousClose float64                     `json:"close_abs_error_pct_vs_previous_close,omitempty"`
	OpenDirectionHit                *bool                       `json:"open_direction_hit,omitempty"`
	CloseDirectionHit               *bool                       `json:"close_direction_hit,omitempty"`
	BacktestSamples                 int                         `json:"backtest_samples,omitempty"`
	BacktestSource                  string                      `json:"backtest_source,omitempty"`
	BacktestOpenMAEPct              float64                     `json:"backtest_open_mae_pct,omitempty"`
	BacktestCloseMAEPct             float64                     `json:"backtest_close_mae_pct,omitempty"`
	BacktestDirectionHitRatePct     float64                     `json:"backtest_direction_hit_rate_pct,omitempty"`
	TechnicalDecisionScore          float64                     `json:"technical_decision_score,omitempty"`
	TechnicalDecisionStatus         string                      `json:"technical_decision_status,omitempty"`
	IndicatorConsensus              string                      `json:"indicator_consensus,omitempty"`
	PatternConsensus                string                      `json:"pattern_consensus,omitempty"`
	TradePlanStatus                 string                      `json:"trade_plan_status,omitempty"`
	PivotS1                         float64                     `json:"pivot_s1,omitempty"`
	PivotR1                         float64                     `json:"pivot_r1,omitempty"`
	Status                          string                      `json:"status,omitempty"`
	Quality                         string                      `json:"quality,omitempty"`
	DirectionBias                   string                      `json:"direction_bias"`
	BiasStrength                    string                      `json:"bias_strength"`
	Confidence                      float64                     `json:"confidence"`
	ConfidenceLabel                 string                      `json:"confidence_label,omitempty"`
	HistoricalSamples               int                         `json:"historical_samples"`
	Model                           string                      `json:"model,omitempty"`
	DecisionForecast                NextSessionDecisionForecast `json:"decision_forecast,omitempty"`
	BacktestTable                   []NextSessionBacktestRow    `json:"backtest_table,omitempty"`
	BacktestMetrics                 NextSessionBacktestMetrics  `json:"backtest_metrics,omitempty"`
	DirectionModelUnreliable        bool                        `json:"direction_model_unreliable,omitempty"`
	InsufficientData                bool                        `json:"insufficient_data,omitempty"`
	BiasReasons                     []string                    `json:"bias_reasons"`
	Warnings                        []string                    `json:"warnings,omitempty"`
}

type NextSessionDecisionForecast struct {
	Date                      string   `json:"date,omitempty"`
	Ticker                    string   `json:"ticker,omitempty"`
	OpenForecast              float64  `json:"open_forecast,omitempty"`
	OpenRangeLow              float64  `json:"open_range_low,omitempty"`
	OpenRangeHigh             float64  `json:"open_range_high,omitempty"`
	CloseForecast             float64  `json:"close_forecast,omitempty"`
	CloseRangeLow             float64  `json:"close_range_low,omitempty"`
	CloseRangeHigh            float64  `json:"close_range_high,omitempty"`
	ExpectedIntradayDirection string   `json:"expected_intraday_direction,omitempty"`
	VolatilityRegime          string   `json:"volatility_regime,omitempty"`
	Confidence                string   `json:"confidence,omitempty"`
	TradeSignalAllowed        bool     `json:"trade_signal_allowed"`
	ReasoningFactors          []string `json:"reasoning_factors,omitempty"`
	RiskWarnings              []string `json:"risk_warnings,omitempty"`
	DirectionModelUnreliable  bool     `json:"direction_model_unreliable,omitempty"`
	InsufficientData          bool     `json:"insufficient_data,omitempty"`
}

type NextSessionBacktestRow struct {
	Date               string  `json:"date"`
	PreviousClose      float64 `json:"previous_close,omitempty"`
	ActualOpen         float64 `json:"actual_open"`
	PredictedOpen      float64 `json:"predicted_open"`
	OpenAbsError       float64 `json:"open_abs_error"`
	OpenPctError       float64 `json:"open_pct_error"`
	ActualClose        float64 `json:"actual_close"`
	PredictedClose     float64 `json:"predicted_close"`
	CloseAbsError      float64 `json:"close_abs_error"`
	ClosePctError      float64 `json:"close_pct_error"`
	ActualDirection    string  `json:"actual_direction"`
	PredictedDirection string  `json:"predicted_direction"`
	DirectionCorrect   bool    `json:"direction_correct"`
}

type NextSessionBacktestMetrics struct {
	Samples                  int     `json:"samples,omitempty"`
	OpenMAE                  float64 `json:"open_mae,omitempty"`
	OpenMAPE                 float64 `json:"open_mape,omitempty"`
	CloseMAE                 float64 `json:"close_mae,omitempty"`
	CloseMAPE                float64 `json:"close_mape,omitempty"`
	DirectionAccuracy        float64 `json:"direction_accuracy,omitempty"`
	HitRatioWithin050Pct     float64 `json:"hit_ratio_within_0_50_pct,omitempty"`
	HitRatioWithin100Pct     float64 `json:"hit_ratio_within_1_00_pct,omitempty"`
	HitRatioWithin200Pct     float64 `json:"hit_ratio_within_2_00_pct,omitempty"`
	TradeSignalAllowed       bool    `json:"trade_signal_allowed"`
	DirectionModelUnreliable bool    `json:"direction_model_unreliable,omitempty"`
}

func NewEngine(provider datasource.MarketDataProvider, renderer chart.ChartRenderer, options EngineOptions) *Engine {
	if options.Limit <= 0 {
		options.Limit = 1000
	}
	if options.BenchmarkSymbol == "" {
		options.BenchmarkSymbol = "XU100"
	}
	if options.PortfolioValue <= 0 {
		options.PortfolioValue = 100000
	}
	if options.RiskPerTradePct <= 0 {
		options.RiskPerTradePct = 1
	}
	if options.PeerLimit <= 0 {
		options.PeerLimit = 20
	}
	options.DataMode = strings.ToLower(strings.TrimSpace(options.DataMode))
	if options.DataMode == "" {
		options.DataMode = "decision"
	}
	if len(options.Timeframes) == 0 {
		options.Timeframes = []string{"1D", "1W", "1M", "3M", "6M", "1Y", "YTD", "ALL"}
	}
	return &Engine{provider: provider, renderer: renderer, options: options}
}

func engineAnalysisDate(asOf time.Time) string {
	if !asOf.IsZero() {
		return asOf.UTC().Format("2006-01-02") + "_retro_" + time.Now().Format("15-04-05")
	}
	return time.Now().Format("2006-01-02_15-04-05")
}

func engineAnalysisDay(asOf time.Time) string {
	if !asOf.IsZero() {
		return asOf.UTC().Format("2006-01-02")
	}
	return time.Now().Format("2006-01-02")
}

func filterCandlesThroughAsOf(candles []ohlcv.Candle, asOf time.Time) []ohlcv.Candle {
	if asOf.IsZero() || len(candles) == 0 {
		return candles
	}
	cutoff := endOfUTCDay(asOf)
	out := make([]ohlcv.Candle, 0, len(candles))
	for _, candle := range candles {
		if candle.Time.IsZero() || candle.Time.UTC().After(cutoff) {
			continue
		}
		out = append(out, candle)
	}
	return out
}

func endOfUTCDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
}

func (e *Engine) AnalyzeSymbol(ctx context.Context, req SymbolRequest) (SymbolAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return SymbolAnalysis{}, fmt.Errorf("analyze symbol canceled: %w", err)
	}
	symbol := ohlcv.NormalizeSymbol(req.Symbol)
	if symbol == "" {
		return SymbolAnalysis{}, fmt.Errorf("normalize symbol: %w", ErrInvalidSymbol)
	}
	instrument, err := e.provider.SearchSymbol(ctx, symbol)
	if err != nil {
		return SymbolAnalysis{}, fmt.Errorf("search symbol %s: %w", symbol, err)
	}
	if req.CompanyName != "" {
		instrument.CompanyName = req.CompanyName
	}
	if req.Currency != "" {
		instrument.Currency = req.Currency
	}
	if req.AssetType != "" {
		instrument.AssetType = ohlcv.NormalizeAssetType(req.AssetType)
	}
	instrument.AssetType = ohlcv.NormalizeAssetType(instrument.AssetType)
	result := SymbolAnalysis{
		Symbol:          instrument.Symbol,
		Exchange:        instrument.Exchange,
		AssetType:       instrument.AssetType,
		CompanyName:     instrument.CompanyName,
		AnalysisDate:    engineAnalysisDate(e.options.AsOf),
		Currency:        instrument.Currency,
		Timeframes:      map[string]TimeframeAnalysis{},
		TimeframeErrors: map[string]string{},
		Charts:          map[string][]byte{},
		Disclaimer:      ohlcv.Disclaimer,
	}
	if !ohlcv.IsCryptoAssetType(result.AssetType) && !ohlcv.IsCommodityAssetType(result.AssetType) {
		result = e.attachExternalQualityContext(ctx, result)
	}
	corporateActions := corporateactions.ActionSet{Symbol: result.Symbol, Status: "not_applicable"}
	if !ohlcv.IsCryptoAssetType(result.AssetType) && !ohlcv.IsCommodityAssetType(result.AssetType) {
		corporateActions = corporateactions.Load(e.options.EquitiesDir, result.Symbol)
	}
	for _, timeframe := range e.options.Timeframes {
		tf, chartPNG, err := e.analyzeTimeframe(ctx, instrument, timeframe, corporateActions)
		if err != nil {
			result.TimeframeErrors[timeframe] = err.Error()
			continue
		}
		result.Timeframes[timeframe] = tf
		result.Charts[timeframe] = chartPNG
	}
	if len(result.Timeframes) == 0 {
		return SymbolAnalysis{}, fmt.Errorf("analyze %s: no usable timeframes: %v", symbol, result.TimeframeErrors)
	}
	if len(result.TimeframeErrors) == 0 {
		result.TimeframeErrors = nil
	}
	if daily, ok := result.Timeframes["1D"]; ok {
		result.NextSessionForecast = daily.NextSessionForecast
	}
	result = e.attachBISTBulletinContext(ctx, result)
	result.PriceQuality = reconcilePriceQualityWithAnalysisClose(result.PriceQuality, result)
	result.PriceQuality = reconcilePriceQualityWithBISTBulletin(result.PriceQuality, result)
	result = applyPriceQualityToTechnicalGates(result)
	result = ApplyBISTEquityTickSizeToTechnicalLevels(result)
	technicalOverall := averageTimeframeScore(result.Timeframes)
	result.OverallScore = technicalOverall
	result.Professional = e.professionalReport(ctx, result, corporateActions)
	result.Quant = BuildQuantAnalysis(result)
	result.StatEconomic = BuildStatEconomicAnalysis(result)
	result.Advanced = BuildAdvancedAnalysis(result)
	if ftbTF, ok := selectFinTradeBenchTimeframe(result.Timeframes); ok {
		result.FinTradeBench = fintradebench.Analyze(fintradebench.Input{
			Symbol:       result.Symbol,
			AssetType:    result.AssetType,
			AsOf:         e.options.AsOf,
			LastClose:    ftbTF.LastClose,
			Candles:      ftbTF.Candles,
			Indicators:   ftbTF.Indicators,
			Professional: result.Professional,
		})
	}
	result = ApplyFundamentalContextToNextSessionForecast(result)
	result = ApplyBISTBulletinContextToNextSessionForecast(result)
	result = ApplyBISTEquityTickSizeToTechnicalLevels(result)
	result.Behavioral = e.behavioralReport(result)
	result.OverallScore = integratedOverallScore(technicalOverall, result.Professional, result.AssetType)
	if !ohlcv.IsCryptoAssetType(result.AssetType) && !ohlcv.IsCommodityAssetType(result.AssetType) {
		result.OverallScore = applyQuantAdjustment(result.OverallScore, result.Quant)
	}
	result.MTFAlignment = multiTimeframeAlignment(result.Timeframes)
	result.OverallScore = applyMTFAlignmentAdjustment(result.OverallScore, result.MTFAlignment)
	result.OverallBias = overallBias(result.Timeframes, result.OverallScore)
	result.InvestorQA = e.investorQAReport(result)
	result.InstitutionalValidation = ValidateInstitutionalReadiness(result)
	result.DecisionClassification = ClassifyDecision(result)
	result = ApplyDecisionClassification(result)
	result.DecisionSupport = BuildDecisionSupport(result)
	result = ApplyNextSessionForecastQualityContext(result)
	return result, nil
}

func (e *Engine) attachExternalQualityContext(ctx context.Context, result SymbolAnalysis) SymbolAnalysis {
	if e.options.FormationsProvider != nil {
		if snapshot, err := e.options.FormationsProvider.LoadTickerSnapshot(ctx, result.Symbol); err == nil {
			result.MatriksFormations = snapshot
		}
	}
	if e.options.PriceQualityProvider != nil {
		priceReport, err := e.options.PriceQualityProvider.InspectSymbol(ctx, result.Symbol)
		if err == nil && priceReport != nil {
			result.PriceQuality = priceReport
		} else {
			result.PriceQuality = priceQualityInspectionFailure(result.Symbol, err)
		}
	} else {
		result.PriceQuality = priceQualityInspectionFailure(result.Symbol, errPriceQualityProviderNotConfigured)
	}
	return result
}

func priceQualityInspectionFailure(symbol string, err error) *pricequality.SymbolReport {
	reason := "price_quality_inspection_failed"
	if err != nil {
		reason += ": " + err.Error()
	}
	return &pricequality.SymbolReport{
		Symbol:                ohlcv.NormalizeSymbol(symbol),
		Status:                pricequality.StatusMissingPriceData,
		ReadyForVerifiedClose: false,
		MissingFields:         []string{"price_quality_report"},
		BlockingReasons:       []string{reason, "official_final_close_missing"},
	}
}

func reconcilePriceQualityWithAnalysisClose(priceReport *pricequality.SymbolReport, result SymbolAnalysis) *pricequality.SymbolReport {
	if priceReport == nil || ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return priceReport
	}
	daily, ok := result.Timeframes["1D"]
	if !ok || daily.LastClose <= 0 || len(daily.Candles) == 0 {
		return priceReport
	}
	candleTime := daily.Candles[len(daily.Candles)-1].Time
	reconciled := pricequality.ReconcileWithAnalysisClose(*priceReport, daily.LastClose, candleTime, time.Now(), "analysis_provider", pricequality.DefaultConflictToleranceBps)
	return &reconciled
}

func reconcilePriceQualityWithBISTBulletin(priceReport *pricequality.SymbolReport, result SymbolAnalysis) *pricequality.SymbolReport {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return priceReport
	}
	if !result.BISTBulletin.Computed || result.BISTBulletin.LatestRecord.Close <= 0 || result.BISTBulletin.LatestRecord.TradingDate == "" {
		return priceReport
	}
	base := pricequality.SymbolReport{Symbol: ohlcv.NormalizeSymbol(result.Symbol)}
	if priceReport != nil {
		base = *priceReport
		if strings.TrimSpace(base.Symbol) == "" {
			base.Symbol = ohlcv.NormalizeSymbol(result.Symbol)
		}
	}
	sourceTimestamp := time.Time{}
	if parsed, err := time.Parse("2006-01-02", result.BISTBulletin.LatestRecord.TradingDate); err == nil {
		sourceTimestamp = parsed
	}
	reconciled := pricequality.ReconcileWithOfficialFinalClose(
		base,
		result.BISTBulletin.LatestRecord.Close,
		result.BISTBulletin.LatestRecord.TradingDate,
		sourceTimestamp,
		time.Now(),
		"bist_thb_official_bulletin",
		result.BISTBulletin.LatestRecord.SourcePath,
		pricequality.DefaultConflictToleranceBps,
	)
	return &reconciled
}

func applyPriceQualityToTechnicalGates(result SymbolAnalysis) SymbolAnalysis {
	if result.PriceQuality == nil || result.PriceQuality.ReadyForDecision || result.PriceQuality.ReadyForVerifiedClose || ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return result
	}
	// Hard-fail teknik kapıları yalnızca fiyat verisi tamamen eksik veya kaynaklar
	// aktif çeliştiğinde uygula. Salt stale (çelişki yok, veri mevcut) durumunda
	// kapıyı düşürmek yerine uyarı ekle; mevcut fiyat bilgisi hâlâ kullanılabilir.
	hardFail := result.PriceQuality.Status == pricequality.StatusMissingPriceData || result.PriceQuality.Conflict
	const hardBlocker = "güncel karar fiyatı kaynaklarla mutabık değil; teknik seviyeler yalnızca ön izleme/paper-trade bağlamıdır"
	const staleWarning = "fiyat kaynağı güncel değil; teknik sinyal güncel olmayan veriyle üretildi — pozisyon açmadan önce fiyatı doğrula"
	for key, tf := range result.Timeframes {
		gate := tf.Professional.Technical.SignalGate
		if hardFail {
			// Fiyat yok veya kaynaklar arası çelişki: kapıyı tamamen kapat.
			tf.Score = math.Min(tf.Score, 45)
			if gate.Status == "" {
				gate.Status = "fail"
			}
			gate.Status = "fail"
			gate.Actionable = false
			gate.Score = math.Min(gate.Score, 45)
			gate.Label = "Güncel karar fiyatı doğrulanmadı; aktif teknik işlem sinyali yok"
			gate.Blockers = appendUniqueAnalysisString(gate.Blockers, hardBlocker)
			for _, reason := range result.PriceQuality.BlockingReasons {
				if strings.TrimSpace(reason) != "" {
					gate.Blockers = appendUniqueAnalysisString(gate.Blockers, "price_quality: "+reason)
				}
			}
			if gate.PriceStructure != "" {
				gate.PriceStructure = "ön izleme/paper-trade: " + gate.PriceStructure
			}
			tf.Professional.Technical.Guardrails = appendUniqueAnalysisString(tf.Professional.Technical.Guardrails, "decision_price_not_reconciled")
			if tf.Professional.Technical.Summary != "" && !strings.Contains(tf.Professional.Technical.Summary, "karar fiyatı doğrulanmadı") {
				tf.Professional.Technical.Summary += "; karar fiyatı doğrulanmadı"
			}
		} else {
			// Stale ama çelişkisiz: kapıyı açık bırak, yalnızca uyarı ekle.
			gate.Blockers = appendUniqueAnalysisString(gate.Blockers, staleWarning)
			if tf.Professional.Technical.Summary != "" && !strings.Contains(tf.Professional.Technical.Summary, "fiyat kaynağı güncel değil") {
				tf.Professional.Technical.Summary += "; fiyat kaynağı güncel değil, sinyal geçici veriyle üretildi"
			}
		}
		// Her iki durumda da resmi kapanış eksik uyarısını ve evidence kaydını ekle.
		tf.Professional.Technical.Guardrails = appendUniqueAnalysisString(tf.Professional.Technical.Guardrails, "official_final_close_missing")
		gate.Evidence = appendUniqueAnalysisString(gate.Evidence, fmt.Sprintf(
			"price_quality.status=%s decision_ready=%t official_ready=%t conflict=%t stale=%t",
			result.PriceQuality.Status,
			result.PriceQuality.ReadyForDecision,
			result.PriceQuality.ReadyForVerifiedClose,
			result.PriceQuality.Conflict,
			result.PriceQuality.Stale,
		))
		tf.Professional.Technical.SignalGate = gate
		result.Timeframes[key] = tf
	}
	return result
}

const bistBulletinForecastValidationLimit = 320

func (e *Engine) attachBISTBulletinContext(ctx context.Context, result SymbolAnalysis) SymbolAnalysis {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return result
	}
	provider, ok := e.provider.(datasource.BulletinRecordProvider)
	if !ok || provider == nil {
		return result
	}
	records, err := e.fetchBISTBulletinRecordsForAnalysis(ctx, provider, result)
	if err != nil {
		result.BISTBulletin = BISTBulletinContext{
			Computed: false,
			Source:   "bist_thb_official_bulletin",
			Warnings: []string{"bist_bulletin_records_unavailable: " + err.Error()},
		}
		return result
	}
	result.BISTBulletin = buildBISTBulletinContext(result, records)
	return ApplyBISTBulletinContextToNextSessionForecast(result)
}

func (e *Engine) fetchBISTBulletinRecordsForAnalysis(ctx context.Context, provider datasource.BulletinRecordProvider, result SymbolAnalysis) ([]datasource.DailyBulletinRecord, error) {
	if rangeProvider, ok := provider.(datasource.BulletinRecordRangeProvider); ok && !e.options.AsOf.IsZero() {
		toDate := e.options.AsOf.UTC().Format("2006-01-02")
		return rangeProvider.FetchDailyBulletinRecordsRange(ctx, result.Symbol, "", toDate, bistBulletinForecastValidationLimit)
	}
	return provider.FetchDailyBulletinRecords(ctx, result.Symbol, bistBulletinForecastValidationLimit)
}

func buildBISTBulletinContext(result SymbolAnalysis, records []datasource.DailyBulletinRecord) BISTBulletinContext {
	ctx := BISTBulletinContext{
		Computed:    len(records) > 0,
		Source:      "bist_thb_official_bulletin",
		RecordCount: len(records),
		records:     append([]datasource.DailyBulletinRecord{}, records...),
	}
	if len(records) == 0 {
		return ctx
	}
	ctx.CoverageStart = records[0].TradingDate
	ctx.CoverageEnd = records[len(records)-1].TradingDate
	ctx.LatestRecord = latestBISTRecordForAnalysis(result, records)
	ctx.LatestVWAP = ctx.LatestRecord.VWAP
	ctx.LatestOpeningSessionVolume = ctx.LatestRecord.OpeningSessionVolume
	ctx.LatestClosingSessionVolume = ctx.LatestRecord.ClosingSessionVolume
	if ctx.LatestRecord.RemainingBid > 0 && ctx.LatestRecord.RemainingAsk > 0 {
		mid := (ctx.LatestRecord.RemainingBid + ctx.LatestRecord.RemainingAsk) / 2
		if mid > 0 {
			ctx.LatestObservedSpreadBps = roundForecastMetric(10000 * (ctx.LatestRecord.RemainingAsk - ctx.LatestRecord.RemainingBid) / mid)
		}
	}
	if daily, ok := result.Timeframes["1D"]; ok {
		latestDate := dailyLastCandleDate(daily)
		if latestDate != "" && latestDate == ctx.LatestRecord.TradingDate && daily.LastClose > 0 && ctx.LatestRecord.Close > 0 {
			ctx.OfficialCloseDeltaPct = roundForecastMetric(100 * (daily.LastClose/ctx.LatestRecord.Close - 1))
			ctx.OfficialCloseConfirmed = math.Abs(ctx.OfficialCloseDeltaPct) <= 0.01
			if !ctx.OfficialCloseConfirmed {
				ctx.Warnings = appendUniqueAnalysisString(ctx.Warnings, fmt.Sprintf("analysis_close_differs_from_bist_bulletin: %.2f%%", ctx.OfficialCloseDeltaPct))
			}
		}
	}
	if result.NextSessionForecast.ForecastFor != "" {
		if record, ok := findBISTBulletinRecordByDate(records, result.NextSessionForecast.ForecastFor); ok {
			ctx.ForecastActualAvailable = true
			ctx.ForecastActualRecord = record
		}
	}
	return ctx
}

func latestBISTRecordForAnalysis(result SymbolAnalysis, records []datasource.DailyBulletinRecord) datasource.DailyBulletinRecord {
	if len(records) == 0 {
		return datasource.DailyBulletinRecord{}
	}
	analysisDate := ""
	if daily, ok := result.Timeframes["1D"]; ok {
		analysisDate = dailyLastCandleDate(daily)
	}
	if analysisDate == "" {
		return records[len(records)-1]
	}
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].TradingDate <= analysisDate {
			return records[i]
		}
	}
	return records[len(records)-1]
}

func dailyLastCandleDate(daily TimeframeAnalysis) string {
	if len(daily.Candles) == 0 {
		return ""
	}
	return daily.Candles[len(daily.Candles)-1].Time.Format("2006-01-02")
}

// ApplyBISTBulletinContextToNextSessionForecast revalidates forecast prices with
// official BIST THB records when that data is available.
func ApplyBISTBulletinContextToNextSessionForecast(result SymbolAnalysis) SymbolAnalysis {
	if !result.BISTBulletin.Computed || len(result.BISTBulletin.records) == 0 {
		return result
	}
	forecast := result.NextSessionForecast
	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		if daily, ok := result.Timeframes["1D"]; ok {
			forecast = daily.NextSessionForecast
		}
	}
	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		return result
	}
	candles := bistBulletinRecordsToCandles(result.BISTBulletin.records)
	if len(candles) > 0 {
		forecast = attachNextSessionForecastValidation(forecast, candles, result.AssetType, "bist_thb_official_bulletin")
	}
	forecast = ApplyBISTBulletinRecordsToNextSessionForecast(forecast, result.BISTBulletin.records, result.AssetType, result.Symbol)
	forecast = ApplyBISTBulletinBacktestToNextSessionForecast(forecast, result.BISTBulletin.records, result.AssetType)
	if result.BISTBulletin.ForecastActualAvailable && result.BISTBulletin.ForecastActualRecord.Open > 0 && result.BISTBulletin.ForecastActualRecord.Close > 0 {
		record := result.BISTBulletin.ForecastActualRecord
		forecast.ValidationStatus = forecastActualObserved
		forecast.ValidationSource = "bist_thb_official_bulletin"
		forecast = attachNextSessionForecastActual(forecast, record.Open, record.Close, "bist_thb_official_bulletin", record.SourcePath)
		if record.OpeningSessionPrice > 0 {
			value := record.OpeningSessionPrice
			forecast.OpeningAuctionEquilibriumPrice = &value
		}
	}
	if result.BISTBulletin.LatestRecord.RemainingBid > 0 && result.BISTBulletin.LatestRecord.RemainingAsk > 0 {
		bid := result.BISTBulletin.LatestRecord.RemainingBid
		ask := result.BISTBulletin.LatestRecord.RemainingAsk
		mid := (bid + ask) / 2
		if mid > 0 {
			spreadBps := roundForecastMetric(10000 * (ask - bid) / mid)
			forecast.MicrostructureAdjustment = nil
			forecast.Warnings = appendUniqueAnalysisString(forecast.Warnings, fmt.Sprintf("latest_bist_bulletin_spread_bps=%.2f", spreadBps))
		}
	}
	forecast.Warnings = appendUniqueAnalysisString(forecast.Warnings, "bist_official_bulletin_records_used_for_validation")
	result.NextSessionForecast = forecast
	if daily, ok := result.Timeframes["1D"]; ok {
		daily.NextSessionForecast = forecast
		result.Timeframes["1D"] = daily
	}
	return result
}

func findBISTBulletinRecordByDate(records []datasource.DailyBulletinRecord, date string) (datasource.DailyBulletinRecord, bool) {
	date = strings.TrimSpace(date)
	if date == "" {
		return datasource.DailyBulletinRecord{}, false
	}
	for _, record := range records {
		if record.TradingDate == date {
			return record, true
		}
	}
	return datasource.DailyBulletinRecord{}, false
}

func bistBulletinRecordsToCandles(records []datasource.DailyBulletinRecord) []ohlcv.Candle {
	candles := make([]ohlcv.Candle, 0, len(records))
	for _, record := range records {
		if record.Close <= 0 {
			continue
		}
		date, err := time.Parse("2006-01-02", record.TradingDate)
		if err != nil {
			continue
		}
		open := record.Open
		if open <= 0 {
			open = record.Close
		}
		low := record.Low
		if low <= 0 {
			low = math.Min(open, record.Close)
		}
		high := record.High
		if high <= 0 {
			high = math.Max(open, record.Close)
		}
		candles = append(candles, ohlcv.Candle{
			Time:           date.UTC(),
			Open:           open,
			High:           high,
			Low:            low,
			Close:          record.Close,
			Volume:         record.Volume,
			AdjustedOpen:   open,
			AdjustedHigh:   high,
			AdjustedLow:    low,
			AdjustedClose:  record.Close,
			AdjustedVolume: record.Volume,
		})
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].Time.Before(candles[j].Time) })
	return candles
}

func averageTimeframeScore(timeframes map[string]TimeframeAnalysis) float64 {
	if len(timeframes) == 0 {
		return 0
	}
	total := 0.0
	for _, tf := range timeframes {
		total += tf.Score
	}
	return mathutil.Clamp(mathutil.SafeDiv(total, float64(len(timeframes))), 0, 100)
}

func selectFinTradeBenchTimeframe(timeframes map[string]TimeframeAnalysis) (TimeframeAnalysis, bool) {
	for _, key := range []string{"1D", "1W", "1M"} {
		if tf, ok := timeframes[key]; ok {
			return tf, true
		}
	}
	keys := make([]string, 0, len(timeframes))
	for key := range timeframes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return TimeframeAnalysis{}, false
	}
	return timeframes[keys[0]], true
}

func appendUniqueAnalysisString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return items
		}
	}
	return append(items, value)
}

var ErrInvalidSymbol = fmt.Errorf("invalid symbol")

func (e *Engine) analyzeTimeframe(ctx context.Context, instrument ohlcv.Instrument, timeframe string, corporateActions corporateactions.ActionSet) (TimeframeAnalysis, []byte, error) {
	candles, err := e.provider.FetchOHLCV(ctx, instrument, timeframe, timeframeFetchLimit(timeframe, e.options.Limit))
	if err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("fetch ohlcv: %w", err)
	}
	candles = filterCandlesThroughAsOf(candles, e.options.AsOf)
	candles = normalizeTimeframeWindow(candles, timeframe)
	if len(candles) == 0 {
		return TimeframeAnalysis{}, nil, fmt.Errorf("empty candles: %w", indicators.ErrInsufficientData)
	}
	if err := validateTimeframeCandleContinuity(candles, timeframe); err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("invalid candle continuity: %w", err)
	}
	priceAdjustmentReport := corporateactions.AdjustmentReport{Symbol: instrument.Symbol, Timeframe: timeframe, PriceSeries: "raw", BacktestSafe: true}
	if !ohlcv.IsCryptoAssetType(instrument.AssetType) && !ohlcv.IsCommodityAssetType(instrument.AssetType) {
		candles, priceAdjustmentReport = corporateactions.Apply(candles, corporateActions, timeframe)
	}
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("calculate indicators: %w", err)
	}
	sr, err := supportresistance.Analyze(candles, snapshot.ATR14, 200)
	if err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("calculate support resistance: %w", err)
	}
	lastClose := candles[len(candles)-1].EffectiveClose()
	lastVolume := candles[len(candles)-1].EffectiveVolume()
	indicatorScan, err := indicators.ScanIndicators(ctx, indicators.ScannerInput{
		Timeframe:  timeframe,
		Candles:    candles,
		Snapshot:   snapshot,
		LastClose:  lastClose,
		LastVolume: lastVolume,
	})
	if err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("scan indicators: %w", err)
	}
	patternInput := patterns.ScannerInput{
		Timeframe:         timeframe,
		Candles:           candles,
		Indicators:        snapshot,
		SupportResistance: sr,
	}
	scanOutput, err := patterns.Scan(ctx, patternInput)
	if err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("scan patterns: %w", err)
	}
	allPatterns := scanOutput.Patterns
	bias := trendBias(candles, snapshot, allPatterns)
	plan, err := risk.BuildTradePlan(risk.Input{
		LastPrice:  lastClose,
		ATR:        snapshot.ATR14,
		Bias:       bias,
		Patterns:   allPatterns,
		Levels:     sr,
		Indicators: snapshot,
	})
	if err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("build trade plan: %w", err)
	}
	geometry, geometryErr := formations.Analyze(candles, formations.Options{
		Symbol:        instrument.Symbol,
		Timeframe:     timeframe,
		AnalysisDate:  engineAnalysisDay(e.options.AsOf),
		PivotLookback: geometryPivotLookback(timeframe),
		MaxLevels:     5,
	})
	if geometryErr != nil {
		geometry = formations.Result{}
	}
	technicalValidation := buildTechnicalValidationReport(candles, snapshot, indicatorScan, patternInput, scanOutput, geometry)
	score := scoreTimeframe(candles, snapshot, allPatterns, sr, plan, bias)
	professionalReport := professional.AnalyzeTimeframe(professional.TimeframeInput{
		Timeframe:           timeframe,
		Candles:             candles,
		Indicators:          snapshot,
		IndicatorSignals:    indicatorScan.Indicators,
		Patterns:            allPatterns,
		PatternScans:        scanOutput.PatternScans,
		TechnicalValidation: technicalValidation,
		TradePlan:           plan,
		LastClose:           lastClose,
		LastVolume:          lastVolume,
		CorporateActions:    corporateActions,
		PriceAdjustment:     priceAdjustmentReport,
	}, professional.Options{
		BenchmarkSymbol: e.options.BenchmarkSymbol,
		PortfolioValue:  e.options.PortfolioValue,
		RiskPerTradePct: e.options.RiskPerTradePct,
		PeerLimit:       e.options.PeerLimit,
	})
	plan = applyTechnicalSignalGateToTradePlan(plan, professionalReport.Technical.SignalGate)
	nsf := NextSessionForecast{}
	if timeframe == "1D" {
		nsf = computeNextSessionForecastWithTechnicalContext(ctx, candles, snapshot, bias, instrument.AssetType, indicatorScan.Indicators, allPatterns, scanOutput.PatternCandidates, sr, plan, instrument.Symbol)
	}
	tf := TimeframeAnalysis{
		Timeframe:           timeframe,
		Candles:             candles,
		LastClose:           lastClose,
		LastVolume:          lastVolume,
		CandleCount:         len(candles),
		Indicators:          snapshot,
		IndicatorSignals:    indicatorScan.Indicators,
		Patterns:            allPatterns,
		PatternCandidates:   scanOutput.PatternCandidates,
		PatternScans:        scanOutput.PatternScans,
		SupportLevels:       sr.SupportLevels,
		ResistanceLevels:    sr.ResistanceLevels,
		NearestSupport:      sr.NearestSupport,
		NearestResistance:   sr.NearestResistance,
		TradePlan:           plan,
		Geometry:            geometry,
		Professional:        professionalReport,
		TrendBias:           bias,
		Score:               score,
		NextSessionForecast: nsf,
	}
	levels := append([]ohlcv.SupportResistanceLevel{}, sr.SupportLevels...)
	levels = append(levels, sr.ResistanceLevels...)
	chartCandles := chartCandlesForRender(candles, timeframe, geometry.DrawingObjects)
	png, err := e.renderer.RenderPNG(ctx, chart.RenderInput{
		Symbol:     instrument.Symbol,
		Timeframe:  timeframe,
		Candles:    chartCandles,
		Indicators: snapshot,
		Levels:     levels,
		Patterns:   allPatterns,
		TradePlan:  plan,
		Drawings:   geometry.DrawingObjects,
		Disclaimer: ohlcv.Disclaimer,
	})
	if err != nil {
		return TimeframeAnalysis{}, nil, fmt.Errorf("render chart png: %w", err)
	}
	return tf, png, nil
}

func applyTechnicalSignalGateToTradePlan(plan ohlcv.TradePlan, gate professional.TechnicalSignalGate) ohlcv.TradePlan {
	if gate.Status == "" || (statusPass(gate.Status) && gate.Actionable) {
		return plan
	}
	if plan.EntryMin <= 0 && plan.EntryMax <= 0 && plan.StopLoss <= 0 && plan.TakeProfit1 <= 0 {
		return plan
	}
	plan.Rejected = true
	plan.RejectReason = "technical signal gate not passed"
	plan.Reasoning = appendUniqueAnalysisString(plan.Reasoning, "Technical signal gate did not pass; generated levels are monitoring/paper-trade levels only.")
	return plan
}

func buildTechnicalValidationReport(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, indicatorOutput indicators.ScannerOutput, patternInput patterns.ScannerInput, patternOutput patterns.ScannerOutput, geometry formations.Result) professional.TechnicalValidationReport {
	indicatorReport := indicators.ValidateIndicatorSystem(candles, snapshot, indicatorOutput)
	patternReport := patterns.ValidatePatternSystem(patternInput, patternOutput)
	indicatorErrors, indicatorWarnings := indicatorValidationIssueCounts(indicatorReport)
	patternErrors, patternWarnings := patternValidationIssueCounts(patternReport)
	computed, proxyOnly, externalRequired := indicatorReadinessCounts(indicatorOutput.Indicators)
	drawn, notDrawn, overlayBlockers := patternOverlayCoverage(candles, patternOutput.Patterns)
	geometryDrawings := countGeometryPatternDrawings(geometry)

	out := professional.TechnicalValidationReport{
		IndicatorChecked:          indicatorReport.Checked,
		IndicatorComputed:         computed,
		IndicatorProxyOnly:        proxyOnly,
		IndicatorExternalRequired: externalRequired,
		IndicatorWarnings:         indicatorWarnings,
		IndicatorErrors:           indicatorErrors,
		PatternChecked:            patternReport.Checked,
		PatternConfirmed:          len(patternOutput.Patterns),
		PatternCandidates:         len(patternOutput.PatternCandidates),
		PatternDrawn:              drawn,
		PatternNotDrawn:           notDrawn,
		PatternWarnings:           patternWarnings,
		PatternErrors:             patternErrors,
		GeometryPatternDrawings:   geometryDrawings,
		Evidence: []string{
			fmt.Sprintf("indikatör formül/doğrulama: %d kontrol, %d hesaplanan, %d hata, %d uyarı", indicatorReport.Checked, computed, indicatorErrors, indicatorWarnings),
			fmt.Sprintf("formasyon kural/doğrulama: %d kontrol, %d aktif, %d aday, %d hata, %d uyarı", patternReport.Checked, len(patternOutput.Patterns), len(patternOutput.PatternCandidates), patternErrors, patternWarnings),
			fmt.Sprintf("grafik işleme: %d aktif formasyonun %d tanesi mum aralığıyla çizilebilir; geometri çizimi %d", len(patternOutput.Patterns), drawn, geometryDrawings),
		},
	}
	out.IndicatorFormulaStatus = "pass"
	if indicatorWarnings > 0 {
		out.IndicatorFormulaStatus = "limited"
	}
	if indicatorErrors > 0 {
		out.IndicatorFormulaStatus = "fail"
		out.Blockers = append(out.Blockers, "indikatör hesap doğrulaması hata üretti: "+indicatorReport.Summary())
	}
	out.PatternStatus = "pass"
	if len(patternOutput.Patterns) == 0 {
		out.PatternStatus = "limited"
		out.Evidence = append(out.Evidence, "aktif formasyon yok; bu bir hesap hatası değil fakat sinyal gücünü sınırlar")
	} else if patternWarnings > 0 {
		out.PatternStatus = "limited"
	}
	if patternErrors > 0 {
		out.PatternStatus = "fail"
		out.Blockers = append(out.Blockers, "formasyon doğrulaması hata üretti: "+patternReport.Summary())
	}
	out.ChartOverlayStatus = "pass"
	if len(patternOutput.Patterns) == 0 {
		out.ChartOverlayStatus = "no_confirmed_pattern"
	} else if notDrawn > 0 {
		out.ChartOverlayStatus = "fail"
		out.Blockers = append(out.Blockers, overlayBlockers...)
	}
	if proxyOnly > 0 || externalRequired > 0 {
		out.Evidence = append(out.Evidence, fmt.Sprintf("proxy/dış veri indikatörleri aktif teyitten hariç tutuldu: proxy=%d dış_veri=%d", proxyOnly, externalRequired))
	}

	out.Score = technicalValidationScoreFromCounts(out)
	out.Status = technicalValidationStatus(out)
	out.GateEligible = indicatorErrors == 0 && patternErrors == 0 && notDrawn == 0
	out.Summary = fmt.Sprintf("Durum %s, skor %.0f/100. İndikatör: %s; formasyon: %s; grafik: %s.", out.Status, out.Score, out.IndicatorFormulaStatus, out.PatternStatus, out.ChartOverlayStatus)
	return out
}

func indicatorValidationIssueCounts(report indicators.ValidationReport) (errorsCount int, warningsCount int) {
	for _, issue := range report.Issues {
		switch issue.Severity {
		case indicators.ValidationSeverityError:
			errorsCount++
		case indicators.ValidationSeverityWarning:
			warningsCount++
		}
	}
	return errorsCount, warningsCount
}

func patternValidationIssueCounts(report patterns.ValidationReport) (errorsCount int, warningsCount int) {
	for _, issue := range report.Issues {
		switch issue.Severity {
		case patterns.ValidationSeverityError:
			errorsCount++
		case patterns.ValidationSeverityWarning:
			warningsCount++
		}
	}
	return errorsCount, warningsCount
}

func indicatorReadinessCounts(results []ohlcv.IndicatorResult) (computed int, proxyOnly int, externalRequired int) {
	for _, result := range results {
		signal := strings.ToLower(strings.TrimSpace(result.Signal))
		source := strings.ToLower(strings.TrimSpace(result.Source))
		if result.Computed {
			computed++
			continue
		}
		if signal == "requires_external_data" || strings.Contains(source, "external") {
			externalRequired++
			continue
		}
		proxyOnly++
	}
	return computed, proxyOnly, externalRequired
}

func patternOverlayCoverage(candles []ohlcv.Candle, patterns []ohlcv.PatternResult) (drawn int, notDrawn int, blockers []string) {
	for _, pattern := range patterns {
		if patternOverlayDrawable(candles, pattern) {
			drawn++
			continue
		}
		notDrawn++
		blockers = append(blockers, "aktif formasyon grafikte işaretlenemedi: "+emptyPatternName(pattern.Name))
	}
	return drawn, notDrawn, blockers
}

func patternOverlayDrawable(candles []ohlcv.Candle, pattern ohlcv.PatternResult) bool {
	if len(candles) == 0 || strings.TrimSpace(pattern.Name) == "" {
		return false
	}
	if pattern.StartIndex < 0 || pattern.EndIndex < pattern.StartIndex || pattern.EndIndex >= len(candles) {
		return false
	}
	if pattern.StartTime.IsZero() || pattern.EndTime.IsZero() {
		return true
	}
	start := candles[pattern.StartIndex].Time
	end := candles[pattern.EndIndex].Time
	if !start.IsZero() && !pattern.StartTime.Equal(start) {
		return false
	}
	if !end.IsZero() && !pattern.EndTime.Equal(end) {
		return false
	}
	return true
}

func emptyPatternName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "adsız formasyon"
	}
	return name
}

func countGeometryPatternDrawings(geometry formations.Result) int {
	if len(geometry.Patterns) > 0 {
		return len(geometry.Patterns)
	}
	seen := map[string]struct{}{}
	for _, line := range geometry.DrawingObjects.Lines {
		key := geometryPatternDrawingKey(line.Label)
		if key == "" {
			key = geometryPatternDrawingKey(line.ID)
		}
		if key != "" {
			seen[key] = struct{}{}
		}
	}
	for _, fill := range geometry.DrawingObjects.Fills {
		key := geometryPatternDrawingKey(fill.ID)
		if key != "" {
			seen[key] = struct{}{}
		}
	}
	return len(seen)
}

func geometryPatternDrawingKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, "_upper")
	value = strings.TrimSuffix(value, "_lower")
	value = strings.TrimSuffix(value, "_fill")
	value = strings.TrimSuffix(value, " upper")
	value = strings.TrimSuffix(value, " lower")
	for _, marker := range []string{"channel", "triangle", "wedge", "flag", "pennant", "consolidation"} {
		if strings.Contains(value, marker) {
			return value
		}
	}
	return ""
}

func technicalValidationScoreFromCounts(report professional.TechnicalValidationReport) float64 {
	score := 100.0
	score -= float64(report.IndicatorErrors) * 30
	score -= float64(report.PatternErrors) * 30
	score -= float64(report.PatternNotDrawn) * 25
	score -= float64(report.IndicatorWarnings+report.PatternWarnings) * 4
	score -= math.Min(float64(report.IndicatorProxyOnly+report.IndicatorExternalRequired), 12)
	if report.PatternConfirmed == 0 {
		score -= 8
	}
	return mathutil.Clamp(score, 0, 100)
}

func technicalValidationStatus(report professional.TechnicalValidationReport) string {
	if report.IndicatorErrors > 0 || report.PatternErrors > 0 || report.PatternNotDrawn > 0 {
		return "fail"
	}
	if report.IndicatorWarnings > 0 || report.PatternWarnings > 0 || report.IndicatorProxyOnly > 0 || report.IndicatorExternalRequired > 0 || report.PatternConfirmed == 0 {
		return "limited"
	}
	return "pass"
}

func timeframeFetchLimit(timeframe string, limit int) int {
	if limit <= 0 {
		limit = 1000
	}
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "YTD":
		return maxInt(limit, 320)
	default:
		return limit
	}
}

func normalizeTimeframeWindow(candles []ohlcv.Candle, timeframe string) []ohlcv.Candle {
	if len(candles) == 0 || !strings.EqualFold(strings.TrimSpace(timeframe), "YTD") {
		return candles
	}
	latest := candles[len(candles)-1].Time
	if latest.IsZero() {
		return candles
	}
	start := time.Date(latest.Year(), time.January, 1, 0, 0, 0, 0, latest.Location())
	first := 0
	for first < len(candles) && candles[first].Time.Before(start) {
		first++
	}
	if first >= len(candles) {
		return candles
	}
	return candles[first:]
}

func validateTimeframeCandleContinuity(candles []ohlcv.Candle, timeframe string) error {
	if len(candles) < 3 {
		return nil
	}
	maxGap := maxAllowedCandleGap(timeframe)
	if maxGap <= 0 {
		return nil
	}
	previous := candles[0].Time
	for i := 1; i < len(candles); i++ {
		current := candles[i].Time
		if previous.IsZero() || current.IsZero() {
			return fmt.Errorf("%s candle %d has zero timestamp", timeframe, i)
		}
		if !current.After(previous) {
			return fmt.Errorf("%s candles are not strictly increasing at %s then %s", timeframe, previous.Format("2006-01-02"), current.Format("2006-01-02"))
		}
		gap := current.Sub(previous)
		if gap > maxGap {
			return fmt.Errorf("%s candle temporal gap %s exceeds %s between %s and %s", timeframe, gap, maxGap, previous.Format("2006-01-02"), current.Format("2006-01-02"))
		}
		previous = current
	}
	return nil
}

func maxAllowedCandleGap(timeframe string) time.Duration {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1D", "D":
		return 14 * 24 * time.Hour
	case "1W", "W":
		return 28 * 24 * time.Hour
	case "1M", "M":
		return 75 * 24 * time.Hour
	default:
		return 0
	}
}

func chartCandlesForRender(candles []ohlcv.Candle, timeframe string, drawings formations.DrawingObjects) []ohlcv.Candle {
	limit := chartRenderLimit(timeframe)
	if limit <= 0 || len(candles) <= limit {
		return candles
	}
	start := len(candles) - limit
	if firstDrawingIdx, ok := firstTrendlineStartIndex(candles, drawings); ok {
		padding := chartRenderPadding(timeframe)
		targetStart := maxInt(0, firstDrawingIdx-padding)
		if len(candles)-targetStart <= limit {
			start = targetStart
		} else if firstDrawingIdx >= len(candles)-limit {
			start = maxInt(0, firstDrawingIdx-padding)
		}
	}
	return candles[start:]
}

func chartRenderLimit(timeframe string) int {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1H", "60", "30", "15", "5":
		return 240
	case "1D", "D":
		return 420
	case "1W", "W":
		return 260
	case "1M", "M":
		return 72
	case "3M":
		return 96
	case "6M":
		return 72
	case "1Y":
		return 60
	case "YTD", "ALL":
		return 0
	default:
		return 320
	}
}

func chartRenderPadding(timeframe string) int {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1H", "60", "30", "15", "5":
		return 30
	case "1D", "D":
		return 35
	case "1W", "W":
		return 20
	default:
		return 30
	}
}

func firstTrendlineStartIndex(candles []ohlcv.Candle, drawings formations.DrawingObjects) (int, bool) {
	index := make(map[string]int, len(candles))
	for i, candle := range candles {
		if candle.Time.IsZero() {
			continue
		}
		index[chartRenderCandleDate(candle.Time)] = i
		index[candle.Time.Format("2006-01-02")] = i
	}
	first := len(candles)
	for _, line := range drawings.Lines {
		if line.Type != "trendline" || strings.TrimSpace(line.StartTime) == "" {
			continue
		}
		if idx, ok := index[line.StartTime]; ok && idx < first {
			first = idx
		}
	}
	return first, first < len(candles)
}

func chartRenderCandleDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(chartRenderDateLocation).Format("2006-01-02")
}

func geometryPivotLookback(timeframe string) int {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1H", "60", "30", "15", "5":
		return 8
	case "1W", "W", "1M", "M", "3M", "6M", "1Y", "YTD", "ALL":
		return 4
	default:
		return 5
	}
}

func (e *Engine) professionalReport(ctx context.Context, result SymbolAnalysis, corporateActions corporateactions.ActionSet) professional.Report {
	var benchmarkCandles []ohlcv.Candle
	benchmarkError := ""
	benchmarkSymbol := strings.TrimSpace(e.options.BenchmarkSymbol)
	if (ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType)) && (benchmarkSymbol == "" || strings.EqualFold(benchmarkSymbol, "XU100")) {
		benchmarkSymbol = ""
	}
	if benchmarkSymbol != "" {
		benchmarkInstrument := ohlcv.Instrument{
			Symbol:    benchmarkSymbol,
			Exchange:  "BIST",
			Currency:  result.Currency,
			AssetType: ohlcv.AssetTypeEquity,
		}
		if strings.Contains(benchmarkSymbol, ":") || ohlcv.InferAssetTypeFromSymbol(benchmarkSymbol) == ohlcv.AssetTypeCrypto {
			if found, err := e.provider.SearchSymbol(ctx, benchmarkSymbol); err == nil {
				benchmarkInstrument = found
			}
		}
		candles, err := e.provider.FetchOHLCV(ctx, benchmarkInstrument, "1D", e.options.Limit)
		if err != nil {
			benchmarkError = err.Error()
		} else {
			benchmarkCandles = filterCandlesThroughAsOf(candles, e.options.AsOf)
		}
	}
	sectorBenchmarkSymbol := e.sectorBenchmarkSymbol(result.Symbol)
	var sectorBenchmarkCandles []ohlcv.Candle
	sectorBenchmarkError := ""
	if sectorBenchmarkSymbol != "" && !strings.EqualFold(sectorBenchmarkSymbol, benchmarkSymbol) {
		sectorBenchmarkInstrument := ohlcv.Instrument{
			Symbol:    sectorBenchmarkSymbol,
			Exchange:  "BIST",
			Currency:  result.Currency,
			AssetType: ohlcv.AssetTypeEquity,
		}
		candles, err := e.provider.FetchOHLCV(ctx, sectorBenchmarkInstrument, "1D", e.options.Limit)
		if err != nil {
			sectorBenchmarkError = err.Error()
		} else {
			sectorBenchmarkCandles = filterCandlesThroughAsOf(candles, e.options.AsOf)
		}
	}
	lastClose := 0.0
	asOf := time.Now().UTC()
	var dailyCandles []ohlcv.Candle
	if tf, ok := result.Timeframes["1D"]; ok {
		lastClose = tf.LastClose
		dailyCandles = tf.Candles
		if len(dailyCandles) > 0 && !dailyCandles[len(dailyCandles)-1].Time.IsZero() {
			asOf = dailyCandles[len(dailyCandles)-1].Time.UTC()
		}
	} else {
		for _, tf := range result.Timeframes {
			if lastClose == 0 {
				lastClose = tf.LastClose
				dailyCandles = tf.Candles
				if len(dailyCandles) > 0 && !dailyCandles[len(dailyCandles)-1].Time.IsZero() {
					asOf = dailyCandles[len(dailyCandles)-1].Time.UTC()
				}
			}
		}
	}
	return professional.AnalyzeSymbol(professional.SymbolInput{
		EquitiesDir:            e.options.EquitiesDir,
		Symbol:                 result.Symbol,
		CompanyName:            result.CompanyName,
		Currency:               result.Currency,
		AssetType:              result.AssetType,
		AsOf:                   asOf,
		LastClose:              lastClose,
		DailyCandles:           dailyCandles,
		BenchmarkCandles:       benchmarkCandles,
		BenchmarkError:         benchmarkError,
		SectorBenchmarkSymbol:  sectorBenchmarkSymbol,
		SectorBenchmarkCandles: sectorBenchmarkCandles,
		SectorBenchmarkError:   sectorBenchmarkError,
		CorporateActions:       corporateActions,
		Options: professional.Options{
			BenchmarkSymbol:          e.options.BenchmarkSymbol,
			DataMode:                 e.options.DataMode,
			ValuationAssumptionsFile: e.options.ValuationAssumptionsFile,
			MacroGDPFile:             e.options.MacroGDPFile,
			TCMBDir:                  e.options.TCMBDir,
			TCMBEVDSDir:              e.options.TCMBEVDSDir,
			VAPDir:                   e.options.VAPDir,
			VAPIndexPortfolioFile:    e.options.VAPIndexPortfolioFile,
			MarketSnapshotFile:       e.options.MarketSnapshotFile,
			PortfolioValue:           e.options.PortfolioValue,
			RiskPerTradePct:          e.options.RiskPerTradePct,
			PeerLimit:                e.options.PeerLimit,
			SkipKAPPDFIngest:         e.options.SkipKAPPDFIngest,
		},
	})
}

type sectorBenchmarkClassificationFile struct {
	Entries map[string]sectorBenchmarkClassification `json:"entries"`
}

type sectorBenchmarkClassification struct {
	Sector    string `json:"sector"`
	Industry  string `json:"industry"`
	PeerGroup string `json:"peer_group"`
}

func (e *Engine) sectorBenchmarkSymbol(symbol string) string {
	symbol = ohlcv.NormalizeSymbol(symbol)
	if symbol == "" {
		return ""
	}
	path := filepath.Join(filepath.Dir(e.options.EquitiesDir), "seed", "sector_classifications.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var file sectorBenchmarkClassificationFile
	if json.Unmarshal(raw, &file) != nil || len(file.Entries) == 0 {
		return ""
	}
	item, ok := file.Entries[symbol]
	if !ok {
		return ""
	}
	return sectorBenchmarkFromText(item.Sector, item.Industry, item.PeerGroup)
}

func sectorBenchmarkFromText(parts ...string) string {
	text := strings.ToLower(strings.Join(parts, " "))
	switch {
	case strings.Contains(text, "banka") || strings.Contains(text, "bankac"):
		return "XBANK"
	case strings.Contains(text, "sigorta"):
		return "XSGRT"
	case strings.Contains(text, "gayrimenkul yatırım ortak") || strings.Contains(text, "gayrimenkul yatirim ortak"):
		return "XGMYO"
	case strings.Contains(text, "holding"):
		return "XHOLD"
	case strings.Contains(text, "bilişim") || strings.Contains(text, "bilisim"):
		return "XBLSM"
	case strings.Contains(text, "teknoloji"):
		return "XUTEK"
	case strings.Contains(text, "elektrik"):
		return "XELKT"
	case strings.Contains(text, "iletişim") || strings.Contains(text, "iletisim"):
		return "XILTM"
	case strings.Contains(text, "inşaat") || strings.Contains(text, "insaat"):
		return "XINSA"
	case strings.Contains(text, "madencilik") || strings.Contains(text, "maden"):
		return "XMADN"
	case strings.Contains(text, "spor"):
		return "XSPOR"
	case strings.Contains(text, "ticaret") || strings.Contains(text, "perakende"):
		return "XTCRT"
	case strings.Contains(text, "turizm"):
		return "XTRZM"
	case strings.Contains(text, "ulaştırma") || strings.Contains(text, "ulastirma"):
		return "XULAS"
	case strings.Contains(text, "gıda") || strings.Contains(text, "gida") || strings.Contains(text, "içecek") || strings.Contains(text, "icecek"):
		return "XGIDA"
	case strings.Contains(text, "kimya") || strings.Contains(text, "petrol") || strings.Contains(text, "plastik"):
		return "XKMYA"
	case strings.Contains(text, "taş") || strings.Contains(text, "tas") || strings.Contains(text, "toprak"):
		return "XTAST"
	case strings.Contains(text, "metal ana"):
		return "XMANA"
	case strings.Contains(text, "metal eşya") || strings.Contains(text, "metal esya") || strings.Contains(text, "makine"):
		return "XMESY"
	case strings.Contains(text, "kağıt") || strings.Contains(text, "kagit"):
		return "XKAGT"
	case strings.Contains(text, "tekstil") || strings.Contains(text, "deri"):
		return "XTEKS"
	case strings.Contains(text, "mali") || strings.Contains(text, "finansal"):
		return "XUMAL"
	case strings.Contains(text, "imalat") || strings.Contains(text, "sanayi"):
		return "XUSIN"
	case strings.Contains(text, "hizmet"):
		return "XUHIZ"
	default:
		return ""
	}
}

func (e *Engine) behavioralReport(result SymbolAnalysis) contrarian.Report {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		for _, key := range []string{"1W", "1M", "3M", "6M", "1Y"} {
			if candidate, exists := result.Timeframes[key]; exists {
				tf = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return contrarian.Report{}
	}
	asOf := time.Now().UTC()
	if len(tf.Candles) > 0 && !tf.Candles[len(tf.Candles)-1].Time.IsZero() {
		asOf = tf.Candles[len(tf.Candles)-1].Time.UTC()
	}
	return contrarian.Analyze(contrarian.Input{
		EquityDir:          symbolDataDir(e.options.EquitiesDir, result.AssetType, result.Symbol),
		Symbol:             result.Symbol,
		AssetType:          result.AssetType,
		AsOf:               asOf,
		LastClose:          tf.LastClose,
		Candles:            tf.Candles,
		Indicators:         tf.Indicators,
		NearestSupport:     tf.NearestSupport,
		NearestResistance:  tf.NearestResistance,
		TrendBias:          tf.TrendBias,
		RequireQualityGate: true,
	})
}

// volatilityRegime classifies current market volatility as "low", "normal", or "high"
// using ATR% relative to price and Bollinger Band width.
func volatilityRegime(snapshot ohlcv.IndicatorSnapshot) string {
	atrPct := snapshot.AdditionalIndicators["Average True Range Percent"]
	bbWidth := snapshot.AdditionalIndicators["Bollinger Band Width"]
	if atrPct > 3.5 || bbWidth > 12.0 {
		return "high"
	}
	if atrPct < 0.8 && bbWidth < 3.5 {
		return "low"
	}
	return "normal"
}

// indicatorConfluence counts how many valid independent indicator categories agree on direction.
// Missing zero-valued indicators are skipped instead of being treated as bearish/bullish votes.
func indicatorConfluence(snapshot ohlcv.IndicatorSnapshot, lastClose float64) (int, int) {
	bull, bear := 0, 0

	// 1. Price vs key moving averages
	if lastClose > 0 && snapshot.EMA50 > 0 {
		if lastClose > snapshot.EMA50 {
			bull++
		} else if lastClose < snapshot.EMA50 {
			bear++
		}
	}
	// 2. MA alignment (golden/death cross region)
	if snapshot.EMA20 > 0 && snapshot.EMA50 > 0 {
		if snapshot.EMA20 > snapshot.EMA50 {
			bull++
		} else if snapshot.EMA20 < snapshot.EMA50 {
			bear++
		}
	}
	// 3. RSI momentum
	if snapshot.RSI14 > 0 && snapshot.RSI14 <= 100 {
		if snapshot.RSI14 > 55 {
			bull++
		} else if snapshot.RSI14 < 45 {
			bear++
		}
	}
	// 4. MACD (signal cross)
	if snapshot.MACD != 0 || snapshot.MACDSignal != 0 || snapshot.MACDHistogram != 0 {
		if snapshot.MACD > snapshot.MACDSignal {
			bull++
		} else if snapshot.MACD < snapshot.MACDSignal {
			bear++
		} else if snapshot.MACDHistogram > 0 {
			bull++
		} else if snapshot.MACDHistogram < 0 {
			bear++
		}
	}
	// 5. Supertrend direction (close vs supertrend level)
	if snapshot.Supertrend > 0 && lastClose > snapshot.Supertrend {
		bull++
	} else if snapshot.Supertrend > 0 {
		bear++
	}
	// 6. Ichimoku cloud trend
	if snapshot.IchimokuCloudTrend > 0 {
		bull++
	} else if snapshot.IchimokuCloudTrend < 0 {
		bear++
	}
	// 7. Volume money flow (CMF)
	if snapshot.ChaikinMoneyFlow20 > 0.02 {
		bull++
	} else if snapshot.ChaikinMoneyFlow20 < -0.02 {
		bear++
	}
	// 8. Stochastic position
	if snapshot.StochasticK > 0 && snapshot.StochasticK <= 100 {
		if snapshot.StochasticK > 55 {
			bull++
		} else if snapshot.StochasticK < 45 {
			bear++
		}
	}
	return bull, bear
}

func ComputeNextSessionForecast(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, bias, assetType string) NextSessionForecast {
	return computeNextSessionForecastModel(candles, snapshot, bias, assetType, true)
}

func ComputeNextSessionForecastFromCandles(candles []ohlcv.Candle, assetType string) (NextSessionForecast, error) {
	return ComputeNextSessionForecastFromCandlesContext(context.TODO(), candles, assetType)
}

func ComputeNextSessionForecastFromCandlesContext(ctx context.Context, candles []ohlcv.Candle, assetType string) (NextSessionForecast, error) {
	if len(candles) == 0 {
		return NextSessionForecast{}, fmt.Errorf("next-session forecast requires candles: %w", indicators.ErrInsufficientData)
	}
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		return NextSessionForecast{}, fmt.Errorf("calculate indicators for next-session forecast: %w", err)
	}
	sr, err := supportresistance.Analyze(candles, snapshot.ATR14, 200)
	if err != nil {
		return NextSessionForecast{}, fmt.Errorf("calculate support/resistance for next-session forecast: %w", err)
	}
	lastClose := candles[len(candles)-1].EffectiveClose()
	lastVolume := candles[len(candles)-1].EffectiveVolume()
	indicatorScan, err := indicators.ScanIndicators(ctx, indicators.ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Snapshot:   snapshot,
		LastClose:  lastClose,
		LastVolume: lastVolume,
	})
	if err != nil {
		return NextSessionForecast{}, fmt.Errorf("scan indicators for next-session forecast: %w", err)
	}
	patternOutput, err := patterns.Scan(ctx, patterns.ScannerInput{
		Timeframe:         "1D",
		Candles:           candles,
		Indicators:        snapshot,
		SupportResistance: sr,
	})
	if err != nil {
		return NextSessionForecast{}, fmt.Errorf("scan patterns for next-session forecast: %w", err)
	}
	detectedPatterns := patternOutput.Patterns
	bias := trendBias(candles, snapshot, detectedPatterns)
	plan, err := risk.BuildTradePlan(risk.Input{
		LastPrice:  lastClose,
		ATR:        snapshot.ATR14,
		Bias:       bias,
		Patterns:   detectedPatterns,
		Levels:     sr,
		Indicators: snapshot,
	})
	if err != nil {
		return NextSessionForecast{}, fmt.Errorf("build trade plan for next-session forecast: %w", err)
	}
	return computeNextSessionForecastWithTechnicalContext(ctx, candles, snapshot, bias, assetType, indicatorScan.Indicators, detectedPatterns, patternOutput.PatternCandidates, sr, plan, ""), nil
}

func AttachActualToNextSessionForecast(f NextSessionForecast, actualOpen, actualClose float64, source, sourcePath string) NextSessionForecast {
	f.ValidationStatus = forecastActualObserved
	if strings.TrimSpace(source) == "" {
		source = "actual_session"
	}
	f.ValidationSource = source
	return attachNextSessionForecastActual(f, actualOpen, actualClose, source, sourcePath)
}

func ApplyBISTBulletinRecordsToNextSessionForecast(f NextSessionForecast, records []datasource.DailyBulletinRecord, assetType, symbol string) NextSessionForecast {
	return applyBISTBulletinRecordsToNextSessionForecast(f, records, assetType, symbol, true)
}

func ApplyBISTBulletinRecordsToNextSessionForecastForAudit(f NextSessionForecast, records []datasource.DailyBulletinRecord, assetType, symbol string) NextSessionForecast {
	return applyBISTBulletinRecordsToNextSessionForecast(f, records, assetType, symbol, false)
}

func applyBISTBulletinRecordsToNextSessionForecast(f NextSessionForecast, records []datasource.DailyBulletinRecord, assetType, symbol string, validateOverlay bool) NextSessionForecast {
	if !f.Computed || f.LastClose <= 0 || len(records) < 20 || !nextSessionForecastUsesBISTPriceStep(assetType, symbol) {
		return f
	}
	known := bistBulletinRecordsBeforeForecast(records, f.ForecastFor)
	if len(known) < 20 {
		return f
	}
	base := f
	overlayApplied := false
	if historical, ok := bistBulletinHistoricalAnalogForecast(f, known, assetType, symbol); ok {
		f = historical
		overlayApplied = true
	}
	latest := known[len(known)-1]
	signal, ok := bistBulletinMicrostructureSignal(f, known, latest)
	if ok {
		overlayApplied = true
		f.RawPredictedOpen = roundForecastPrice(signal.openTarget)
		f.RawPredictedClose = roundForecastPrice(signal.closeTarget)
		f.PredictedOpen = f.RawPredictedOpen
		f.PredictedClose = f.RawPredictedClose
		if f.RawExpectedLow <= 0 {
			f.RawExpectedLow = f.ExpectedLow
		}
		if f.RawExpectedHigh <= 0 {
			f.RawExpectedHigh = f.ExpectedHigh
		}
		if f.RawExpectedLow > 0 {
			f.RawExpectedLow = roundForecastPrice(math.Min(f.RawExpectedLow, math.Min(f.RawPredictedOpen, f.RawPredictedClose)))
		}
		if f.RawExpectedHigh > 0 {
			f.RawExpectedHigh = roundForecastPrice(math.Max(f.RawExpectedHigh, math.Max(f.RawPredictedOpen, f.RawPredictedClose)))
		}
		f.ExpectedLow = f.RawExpectedLow
		f.ExpectedHigh = f.RawExpectedHigh
		if f.LastClose > 0 {
			f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
			f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
		}
		switch {
		case f.CloseChangePct >= 0.35:
			f.DirectionBias = "yükseliş"
		case f.CloseChangePct <= -0.35:
			f.DirectionBias = "düşüş"
		default:
			f.DirectionBias = "yatay"
		}
		f.BiasStrength = signal.strength
		f.Confidence = roundForecastMetric(math.Min(f.Confidence, signal.confidenceCap))
		f.ConfidenceLabel = nextSessionConfidenceLabel(f.Confidence)
		f.Model = withForecastModelOverlay(f.Model, "bist_microstructure_v2")
		f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, signal.reason)
		f.Warnings = appendUniqueAnalysisString(f.Warnings, "bist_bulletin_microstructure_forecast_overlay_applied")
	}
	if !overlayApplied {
		return syncNextSessionDecisionForecast(f, symbol)
	}
	if validateOverlay {
		allowed, baseline, overlay := bistBulletinOverlayValidationAllowsUse(known, assetType)
		if !allowed {
			closeFallbackAnchor := nextSessionForecastBISTBulletinOpenAnchorBacktest(known, assetType, forecastRollingBacktestWindow)
			openOnlyAnchor := nextSessionForecastBISTBulletinOpenAnchorOpenOnlyBacktest(known, assetType, forecastRollingBacktestWindow)
			closeFallbackAllowed := bistBulletinOpenAnchorValidationAllowsUse(baseline, overlay, closeFallbackAnchor)
			openOnlyAllowed := bistBulletinOpenAnchorValidationAllowsUse(baseline, overlay, openOnlyAnchor)
			if closeFallbackAllowed || openOnlyAllowed {
				selectedAnchor := closeFallbackAnchor
				useCloseFallback := closeFallbackAllowed
				if openOnlyAllowed && (!closeFallbackAllowed || bistBulletinBacktestPreferred(openOnlyAnchor, closeFallbackAnchor)) {
					selectedAnchor = openOnlyAnchor
					useCloseFallback = false
				}
				f = applyBISTBulletinOpenAnchorForecast(base, f, selectedAnchor, assetType, symbol, useCloseFallback)
				f = applyTradablePriceStepToNextSessionForecast(f, assetType, symbol)
				return syncNextSessionDecisionForecast(f, symbol)
			}
			base.BiasReasons = appendUniqueAnalysisString(base.BiasReasons, fmt.Sprintf(
				"BIST analog/mikro yapı katmanı kullanılmadı: son %d örnekte overlay kapanış yön uyumu %.2f%%, kapanış MAE %.2f%%; baseline yön uyumu %.2f%%, kapanış MAE %.2f%%.",
				overlay.samples,
				overlay.directionHitRatePct,
				overlay.closeMAEPct,
				baseline.directionHitRatePct,
				baseline.closeMAEPct,
			))
			base.Warnings = appendUniqueAnalysisString(base.Warnings, "bist_bulletin_overlay_validation_failed")
			base = applyTradablePriceStepToNextSessionForecast(base, assetType, symbol)
			return syncNextSessionDecisionForecast(base, symbol)
		}
		f = dampWeakBISTBulletinOverlayForecast(f, overlay)
	}
	f = applyTradablePriceStepToNextSessionForecast(f, assetType, symbol)
	return syncNextSessionDecisionForecast(f, symbol)
}

func ApplyBISTBulletinBacktestToNextSessionForecast(f NextSessionForecast, records []datasource.DailyBulletinRecord, assetType string) NextSessionForecast {
	useOverlay := nextSessionForecastUsesBISTOverlay(f)
	useOpenAnchor := nextSessionForecastUsesBISTOpenAnchor(f)
	backtestSource := "bist_thb_official_bulletin_selected_baseline"
	metrics := nextSessionForecastBISTBulletinVariantBacktest(records, assetType, forecastRollingBacktestWindow, false)
	if useOpenAnchor {
		backtestSource = "bist_thb_official_bulletin_selected_open_anchor_close_fallback"
		metrics = nextSessionForecastBISTBulletinOpenAnchorBacktest(records, assetType, forecastRollingBacktestWindow)
		if strings.Contains(strings.ToLower(f.Model), "bist_open_anchor_open_only") {
			backtestSource = "bist_thb_official_bulletin_selected_open_anchor_open_only"
			metrics = nextSessionForecastBISTBulletinOpenAnchorOpenOnlyBacktest(records, assetType, forecastRollingBacktestWindow)
		}
	} else if useOverlay {
		backtestSource = "bist_thb_official_bulletin_selected_overlay"
		metrics = nextSessionForecastBISTBulletinVariantBacktest(records, assetType, forecastRollingBacktestWindow, true)
	}
	if metrics.samples > 0 {
		f.BacktestSamples = metrics.samples
		f.BacktestSource = backtestSource
		f.BacktestOpenMAEPct = roundForecastMetric(metrics.openMAEPct)
		f.BacktestCloseMAEPct = roundForecastMetric(metrics.closeMAEPct)
		f.BacktestDirectionHitRatePct = roundForecastMetric(metrics.directionHitRatePct)
		f.BacktestTable = metrics.rows
		f.BacktestMetrics = NextSessionBacktestMetrics{
			Samples:                  metrics.samples,
			OpenMAE:                  roundForecastPrice(metrics.openMAE),
			OpenMAPE:                 roundForecastMetric(metrics.openMAEPct),
			CloseMAE:                 roundForecastPrice(metrics.closeMAE),
			CloseMAPE:                roundForecastMetric(metrics.closeMAEPct),
			DirectionAccuracy:        roundForecastMetric(metrics.directionHitRatePct),
			HitRatioWithin050Pct:     roundForecastMetric(metrics.hit050Pct),
			HitRatioWithin100Pct:     roundForecastMetric(metrics.hit100Pct),
			HitRatioWithin200Pct:     roundForecastMetric(metrics.hit200Pct),
			TradeSignalAllowed:       metrics.closeMAEPct <= nextSessionDecisionMaxCloseMAPEPct && metrics.directionHitRatePct >= nextSessionDecisionMinDirectionAccuracyPct,
			DirectionModelUnreliable: metrics.directionHitRatePct < nextSessionDecisionMinDirectionAccuracyPct,
		}
		f.DirectionModelUnreliable = f.BacktestMetrics.DirectionModelUnreliable
		if f.DirectionModelUnreliable {
			f.Warnings = appendUniqueAnalysisString(f.Warnings, "direction_model_unreliable")
		}
		if metrics.samples >= nextSessionPointForecastMinBacktestSamples &&
			(metrics.directionHitRatePct < nextSessionPointForecastMinDirectionHitPct ||
				metrics.closeMAEPct > nextSessionPointForecastMaxCloseMAEPct) {
			f.Status = "model_validation_failed"
			f.Quality = "not_decision_grade"
			f.Confidence = roundForecastMetric(math.Min(f.Confidence, 35))
			f.ConfidenceLabel = nextSessionConfidenceLabel(f.Confidence)
			f.BiasStrength = "validasyon zayıf"
			f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, fmt.Sprintf(
				"Model validasyonu başarısız: son %d resmi BIST örneğinde kapanış yön uyumu %.2f%%, kapanış MAE %.2f%%. Bu tahmin karar/emir seviyesi olarak kullanılamaz.",
				metrics.samples,
				metrics.directionHitRatePct,
				metrics.closeMAEPct,
			))
			f.Warnings = appendUniqueAnalysisString(f.Warnings, "forecast_model_validation_failed_not_decision_grade")
		} else if metrics.samples >= 20 && metrics.directionHitRatePct < nextSessionPointForecastMinDirectionHitPct {
			f.Warnings = appendUniqueAnalysisString(f.Warnings, fmt.Sprintf(
				"backtest_low_direction_hit_%.0fpct", metrics.directionHitRatePct,
			))
			f.Confidence = roundForecastMetric(math.Min(f.Confidence, 55))
			f.ConfidenceLabel = nextSessionConfidenceLabel(f.Confidence)
		}
		f = calibrateWeakValidatedNextSessionForecast(f, metrics, assetType, "")
		f = calibrateNextSessionDecisionInterval(f, metrics, assetType, "")
	}
	return syncNextSessionDecisionForecast(f, f.DecisionForecast.Ticker)
}

func nextSessionForecastUsesBISTOverlay(f NextSessionForecast) bool {
	model := strings.ToLower(f.Model)
	return strings.Contains(model, "bist_history_analog") ||
		strings.Contains(model, "bist_microstructure") ||
		strings.Contains(model, "bist_open_anchor")
}

func nextSessionForecastUsesBISTOpenAnchor(f NextSessionForecast) bool {
	return strings.Contains(strings.ToLower(f.Model), "bist_open_anchor")
}

func calibrateWeakValidatedNextSessionForecast(f NextSessionForecast, metrics nextSessionForecastBacktestMetrics, assetType, symbol string) NextSessionForecast {
	if !f.Computed || f.LastClose <= 0 || metrics.samples < nextSessionPointForecastMinBacktestSamples {
		return f
	}
	model := strings.ToLower(f.Model)
	if strings.Contains(model, "weak_validation_calibration_v1") {
		return f
	}
	closeWeak := metrics.closeMAEPct > nextSessionPointForecastMaxCloseMAEPct
	directionWeak := metrics.directionHitRatePct > 0 && metrics.directionHitRatePct < nextSessionPointForecastMinDirectionHitPct
	if !closeWeak && !directionWeak {
		return f
	}

	factor := 0.50
	switch {
	case closeWeak && directionWeak:
		factor = 0.20
	case closeWeak:
		factor = 0.35
	}
	rawOpen := forecastRawPredictedOpen(f)
	rawClose := forecastRawPredictedClose(f)
	if rawOpen > 0 {
		f.RawPredictedOpen = roundForecastPrice(f.LastClose + factor*(rawOpen-f.LastClose))
		f.PredictedOpen = f.RawPredictedOpen
	}
	if rawClose > 0 {
		f.RawPredictedClose = roundForecastPrice(f.LastClose + factor*(rawClose-f.LastClose))
		f.PredictedClose = f.RawPredictedClose
	}

	center := firstPositiveFloat(f.RawPredictedClose, f.PredictedClose, f.LastClose)
	if center > 0 {
		spanPct := 0.035
		if metrics.closeMAEPct > 0 {
			spanPct = mathutil.Clamp(metrics.closeMAEPct*2.75/100, 0.025, 0.12)
		}
		span := f.LastClose * spanPct
		rawLow := forecastRawExpectedLow(f)
		rawHigh := forecastRawExpectedHigh(f)
		if rawLow <= 0 {
			rawLow = center - span
		}
		if rawHigh <= 0 {
			rawHigh = center + span
		}
		f.RawExpectedLow = roundForecastPrice(math.Min(rawLow, center-span))
		f.RawExpectedHigh = roundForecastPrice(math.Max(rawHigh, center+span))
		f.ExpectedLow = f.RawExpectedLow
		f.ExpectedHigh = f.RawExpectedHigh
		f.CloseP10 = f.RawExpectedLow
		f.CloseP50 = roundForecastPrice(center)
		f.CloseP90 = f.RawExpectedHigh
	}
	if f.LastClose > 0 && f.PredictedOpen > 0 {
		f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
	}
	if f.LastClose > 0 && f.PredictedClose > 0 {
		f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
	}
	f.DirectionBias = downgradedNextSessionDirection(f.DirectionBias, f.CloseChangePct)
	f.BiasStrength = "validasyon zayıf"
	f.Confidence = roundForecastMetric(math.Min(f.Confidence, 35))
	f.ConfidenceLabel = nextSessionConfidenceLabel(f.Confidence)
	f.Model = withForecastModelOverlay(f.Model, "weak_validation_calibration_v1")
	f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, fmt.Sprintf(
		"Zayıf rolling validasyon kalibrasyonu: son %d örnekte kapanış MAPE %.2f%%, yön uyumu %.2f%%; point hareketi %.0f%% katsayıyla sönümlendi ve scenario bandı genişletildi.",
		metrics.samples,
		metrics.closeMAEPct,
		metrics.directionHitRatePct,
		factor*100,
	))
	f.Warnings = appendUniqueAnalysisString(f.Warnings, "weak_validation_point_forecast_damped_interval_widened")
	return applyTradablePriceStepToNextSessionForecast(f, assetType, symbol)
}

func calibrateNextSessionDecisionInterval(f NextSessionForecast, metrics nextSessionForecastBacktestMetrics, assetType, symbol string) NextSessionForecast {
	if !f.Computed || f.LastClose <= 0 || f.PredictedClose <= 0 || metrics.samples < nextSessionPointForecastMinBacktestSamples {
		return f
	}
	errorsPct := make([]float64, 0, len(metrics.rows))
	for _, row := range metrics.rows {
		if row.ClosePctError <= 0 || math.IsNaN(row.ClosePctError) || math.IsInf(row.ClosePctError, 0) {
			continue
		}
		errorsPct = append(errorsPct, row.ClosePctError)
	}
	if len(errorsPct) < nextSessionPointForecastMinBacktestSamples {
		return f
	}
	sort.Float64s(errorsPct)
	validationOK := metrics.closeMAEPct <= nextSessionDecisionMaxCloseMAPEPct &&
		metrics.directionHitRatePct >= nextSessionDecisionMinDirectionAccuracyPct
	quantile := 0.75
	status := "active"
	if !validationOK {
		quantile = 0.80
		status = "candidate_validation_failed"
	}
	halfPct := mathutil.Clamp(percentileSorted(errorsPct, quantile)/100, 0.008, 0.055)
	low := f.PredictedClose * (1 - halfPct)
	high := f.PredictedClose * (1 + halfPct)
	if low <= 0 || high <= low {
		return f
	}
	low = roundForecastPrice(low)
	high = roundForecastPrice(high)
	if nextSessionForecastUsesBISTPriceStep(assetType, symbol) {
		low = roundForecastPriceToTick(low, bistEquityTickSize(low))
		high = roundForecastPriceToTick(high, bistEquityTickSize(high))
		if low > high {
			low, high = high, low
		}
	}
	f.DecisionIntervalLow = low
	f.DecisionIntervalHigh = high
	f.DecisionIntervalWidthPct = roundForecastMetric(100 * (high - low) / f.LastClose)
	f.DecisionIntervalStatus = status
	f.DecisionIntervalReason = fmt.Sprintf("conformal_q%.0f_close_error_pct:%.2f", quantile*100, halfPct*100)
	f.Model = withForecastModelOverlay(f.Model, "decision_interval_conformal_v1")
	f.Warnings = appendUniqueAnalysisString(f.Warnings, "decision_interval_conformal_calibrated")
	if !validationOK {
		f.Warnings = appendUniqueAnalysisString(f.Warnings, "decision_interval_candidate_validation_failed")
	}
	return f
}

type bistBulletinAnalogPrediction struct {
	openReturn         float64
	closeReturn        float64
	neighborCount      int
	sampleCount        int
	averageDistance    float64
	nearestDate        string
	nearestNextDate    string
	nearestDistance    float64
	nearestOpenReturn  float64
	nearestCloseReturn float64
}

type bistBulletinAnalogNeighbor struct {
	index       int
	distance    float64
	weight      float64
	openReturn  float64
	closeReturn float64
}

func bistBulletinHistoricalAnalogForecast(f NextSessionForecast, records []datasource.DailyBulletinRecord, assetType, symbol string) (NextSessionForecast, bool) {
	if !f.Computed || len(records) < 80 {
		return f, false
	}
	prediction, ok := bistBulletinAnalogPredict(records)
	if !ok || prediction.neighborCount == 0 {
		return f, false
	}
	latest := records[len(records)-1]
	if latest.Close <= 0 {
		return f, false
	}
	openTarget := roundForecastPrice(latest.Close * (1 + prediction.openReturn))
	closeTarget := roundForecastPrice(latest.Close * (1 + prediction.closeReturn))
	if openTarget <= 0 || closeTarget <= 0 {
		return f, false
	}
	out := f
	out.LastClose = roundForecastPrice(latest.Close)
	out.RawPredictedOpen = openTarget
	out.RawPredictedClose = closeTarget
	out.PredictedOpen = openTarget
	out.PredictedClose = closeTarget
	atr := bistBulletinATR(records, 14)
	if atr <= 0 {
		atr = math.Max(math.Abs(closeTarget-latest.Close), latest.Close*0.02)
	}
	half := math.Max(0.70*atr, math.Abs(closeTarget-latest.Close))
	out.RawExpectedLow = roundForecastPrice(math.Min(latest.Close-half, math.Min(openTarget, closeTarget)))
	out.RawExpectedHigh = roundForecastPrice(math.Max(latest.Close+half, math.Max(openTarget, closeTarget)))
	out.ExpectedLow = out.RawExpectedLow
	out.ExpectedHigh = out.RawExpectedHigh
	if out.LastClose > 0 {
		out.OpenChangePct = roundForecastMetric(100 * (out.PredictedOpen/out.LastClose - 1))
		out.CloseChangePct = roundForecastMetric(100 * (out.PredictedClose/out.LastClose - 1))
	}
	switch {
	case out.CloseChangePct >= 0.35:
		out.DirectionBias = "yükseliş"
	case out.CloseChangePct <= -0.35:
		out.DirectionBias = "düşüş"
	default:
		out.DirectionBias = "yatay"
	}
	out.BiasStrength = "veri-bazlı"
	out.Model = withForecastModelOverlay(out.Model, "bist_history_analog_knn_v1")
	out.Quality = "official_bist_history_analog"
	out.HistoricalSamples = prediction.sampleCount
	out.Confidence = roundForecastMetric(math.Min(out.Confidence, bistBulletinAnalogConfidenceCap(prediction.averageDistance, prediction.neighborCount)))
	out.ConfidenceLabel = nextSessionConfidenceLabel(out.Confidence)
	out.BiasReasons = appendUniqueAnalysisString(out.BiasReasons, fmt.Sprintf(
		"BIST resmi bülten geçmişi: %d aday içinden en yakın %d benzer seans kullanıldı; en yakın örnek %s -> %s, uzaklık %.3f.",
		prediction.sampleCount,
		prediction.neighborCount,
		prediction.nearestDate,
		prediction.nearestNextDate,
		prediction.nearestDistance,
	))
	out.Warnings = appendUniqueAnalysisString(out.Warnings, "bist_official_history_analog_forecast_used")
	out = applyTradablePriceStepToNextSessionForecast(out, assetType, symbol)
	return out, true
}

func bistBulletinAnalogConfidenceCap(averageDistance float64, neighbors int) float64 {
	if neighbors < 8 {
		return 40
	}
	switch {
	case averageDistance <= 0.35:
		return 62
	case averageDistance <= 0.55:
		return 56
	case averageDistance <= 0.80:
		return 50
	default:
		return 44
	}
}

func bistBulletinAnalogPredict(records []datasource.DailyBulletinRecord) (bistBulletinAnalogPrediction, bool) {
	targetIdx := len(records) - 1
	if targetIdx < 60 || records[targetIdx].Close <= 0 {
		return bistBulletinAnalogPrediction{}, false
	}
	targetFeatures, ok := bistBulletinAnalogFeatures(records, targetIdx)
	if !ok {
		return bistBulletinAnalogPrediction{}, false
	}
	neighbors := make([]bistBulletinAnalogNeighbor, 0, targetIdx)
	for idx := 25; idx < targetIdx; idx++ {
		current := records[idx]
		next := records[idx+1]
		if current.Close <= 0 || next.Open <= 0 || next.Close <= 0 {
			continue
		}
		features, ok := bistBulletinAnalogFeatures(records, idx)
		if !ok {
			continue
		}
		distance := bistBulletinAnalogDistance(targetFeatures, features)
		if math.IsNaN(distance) || math.IsInf(distance, 0) {
			continue
		}
		age := float64(targetIdx - idx)
		recency := math.Exp(-age / 150)
		weight := recency / math.Pow(0.20+distance, 2)
		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			continue
		}
		neighbors = append(neighbors, bistBulletinAnalogNeighbor{
			index:       idx,
			distance:    distance,
			weight:      weight,
			openReturn:  mathutil.Clamp(next.Open/current.Close-1, -0.12, 0.12),
			closeReturn: mathutil.Clamp(next.Close/current.Close-1, -0.12, 0.12),
		})
	}
	if len(neighbors) < 12 {
		return bistBulletinAnalogPrediction{}, false
	}
	sort.Slice(neighbors, func(i, j int) bool {
		if neighbors[i].distance == neighbors[j].distance {
			return neighbors[i].index > neighbors[j].index
		}
		return neighbors[i].distance < neighbors[j].distance
	})
	k := int(math.Round(math.Sqrt(float64(len(neighbors))) * 1.8))
	if k < 12 {
		k = 12
	}
	if k > 36 {
		k = 36
	}
	if k > len(neighbors) {
		k = len(neighbors)
	}
	totalWeight := 0.0
	openReturn := 0.0
	closeReturn := 0.0
	avgDistance := 0.0
	for _, neighbor := range neighbors[:k] {
		totalWeight += neighbor.weight
		openReturn += neighbor.openReturn * neighbor.weight
		closeReturn += neighbor.closeReturn * neighbor.weight
		avgDistance += neighbor.distance
	}
	if totalWeight <= 0 {
		return bistBulletinAnalogPrediction{}, false
	}
	nearest := neighbors[0]
	nextDate := ""
	if nearest.index+1 < len(records) {
		nextDate = records[nearest.index+1].TradingDate
	}
	return bistBulletinAnalogPrediction{
		openReturn:         openReturn / totalWeight,
		closeReturn:        closeReturn / totalWeight,
		neighborCount:      k,
		sampleCount:        len(neighbors),
		averageDistance:    avgDistance / float64(k),
		nearestDate:        records[nearest.index].TradingDate,
		nearestNextDate:    nextDate,
		nearestDistance:    nearest.distance,
		nearestOpenReturn:  nearest.openReturn,
		nearestCloseReturn: nearest.closeReturn,
	}, true
}

func bistBulletinAnalogFeatures(records []datasource.DailyBulletinRecord, idx int) ([]float64, bool) {
	if idx < 20 || idx >= len(records) {
		return nil, false
	}
	record := records[idx]
	prevClose := record.PreviousClose
	if prevClose <= 0 && idx > 0 {
		prevClose = records[idx-1].Close
	}
	if record.Open <= 0 || record.Close <= 0 || prevClose <= 0 {
		return nil, false
	}
	dayReturn := record.Close/prevClose - 1
	gapReturn := record.Open/prevClose - 1
	intradayReturn := record.Close/record.Open - 1
	rangePct := 0.0
	closePosition := 0.0
	if record.High > record.Low && prevClose > 0 {
		rangePct = (record.High - record.Low) / prevClose
		closePosition = ((record.Close-record.Low)/(record.High-record.Low) - 0.5) * 2
	}
	vwapGap := 0.0
	if record.VWAP > 0 {
		vwapGap = record.Close/record.VWAP - 1
	}
	volumeRatio := bistBulletinLogVolumeRatio(records, idx, 20)
	mom5 := bistBulletinMomentum(records, idx, 5)
	mom20 := bistBulletinMomentum(records, idx, 20)
	atrPct := 0.0
	if atr := bistBulletinATR(records[:idx+1], 14); atr > 0 {
		atrPct = atr / record.Close
	}
	cmf20 := 0.0
	if cmf, ok := bistBulletinCMF(records[:idx+1], 20); ok {
		cmf20 = cmf
	}
	openAuctionPressure := 0.0
	if record.OpeningSessionVolume > 0 && record.Volume > 0 {
		openAuctionPressure = mathutil.Clamp(record.OpeningSessionVolume/record.Volume, 0, 1)
	}
	closeAuctionPressure := 0.0
	if record.ClosingSessionVolume > 0 && record.Volume > 0 {
		closeAuctionPressure = mathutil.Clamp(record.ClosingSessionVolume/record.Volume, 0, 1)
	}
	return []float64{
		mathutil.Clamp(dayReturn/0.045, -4, 4),
		mathutil.Clamp(gapReturn/0.025, -4, 4),
		mathutil.Clamp(intradayReturn/0.045, -4, 4),
		mathutil.Clamp(rangePct/0.060, 0, 4),
		mathutil.Clamp(closePosition, -1, 1),
		mathutil.Clamp(vwapGap/0.025, -4, 4),
		mathutil.Clamp(volumeRatio, -3, 3),
		mathutil.Clamp(mom5/0.090, -4, 4),
		mathutil.Clamp(mom20/0.180, -4, 4),
		mathutil.Clamp(atrPct/0.055, 0, 4),
		mathutil.Clamp(cmf20/0.50, -2, 2),
		mathutil.Clamp(openAuctionPressure/0.08, 0, 4),
		mathutil.Clamp(closeAuctionPressure/0.08, 0, 4),
	}, true
}

func bistBulletinAnalogDistance(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return math.Inf(1)
	}
	sum := 0.0
	for i := range a {
		delta := a[i] - b[i]
		sum += delta * delta
	}
	return math.Sqrt(sum / float64(len(a)))
}

func bistBulletinLogVolumeRatio(records []datasource.DailyBulletinRecord, idx, period int) float64 {
	if idx <= 0 || period <= 0 || idx >= len(records) || records[idx].Volume <= 0 {
		return 0
	}
	start := idx - period
	if start < 0 {
		start = 0
	}
	total := 0.0
	count := 0
	for i := start; i < idx; i++ {
		if records[i].Volume > 0 {
			total += records[i].Volume
			count++
		}
	}
	if count == 0 || total <= 0 {
		return 0
	}
	avg := total / float64(count)
	if avg <= 0 {
		return 0
	}
	return math.Log(records[idx].Volume / avg)
}

func bistBulletinMomentum(records []datasource.DailyBulletinRecord, idx, period int) float64 {
	if period <= 0 || idx-period < 0 || idx >= len(records) {
		return 0
	}
	base := records[idx-period].Close
	current := records[idx].Close
	if base <= 0 || current <= 0 {
		return 0
	}
	return current/base - 1
}

type bistBulletinMicrostructureAdjustment struct {
	openTarget    float64
	closeTarget   float64
	strength      string
	confidenceCap float64
	reason        string
}

func bistBulletinMicrostructureSignal(f NextSessionForecast, records []datasource.DailyBulletinRecord, latest datasource.DailyBulletinRecord) (bistBulletinMicrostructureAdjustment, bool) {
	if latest.Close <= 0 || latest.Open <= 0 {
		return bistBulletinMicrostructureAdjustment{}, false
	}
	// VWAP fallback: BIST bülteninde VWAP eksik olduğunda (H+L+C)/3 tahmini kullanılır.
	vwap := latest.VWAP
	if vwap <= 0 && latest.High > 0 && latest.Low > 0 {
		vwap = (latest.High + latest.Low + latest.Close) / 3
	}
	if vwap <= 0 {
		vwap = latest.Close
	}
	changePct := latest.ChangePct
	if changePct == 0 && latest.PreviousClose > 0 {
		changePct = 100 * (latest.Close/latest.PreviousClose - 1)
	}
	vwapGapPct := 100 * (latest.Close/vwap - 1)
	intradayPct := 100 * (latest.Close/latest.Open - 1)
	cmf20, _ := bistBulletinCMF(records, 20)
	atr := bistBulletinATR(records, 14)
	if atr <= 0 {
		atr = math.Max((forecastRawExpectedHigh(f)-forecastRawExpectedLow(f))/1.4, latest.Close*0.025)
	}
	if atr <= 0 {
		atr = latest.Close * 0.025
	}
	vwapGap := math.Abs(latest.Close - vwap)
	// Yükseliş sonrası kar satışı / mean-reversion sinyali.
	// Gevşetilmiş eşikler: 3/4 koşul yeterli, CMF opsiyonel.
	upSignals := 0
	if changePct >= 1.5 {
		upSignals++
	}
	if vwapGapPct >= 1.0 {
		upSignals++
	}
	if intradayPct >= 1.2 {
		upSignals++
	}
	if cmf20 < -0.02 {
		upSignals++
	}
	if upSignals >= 3 {
		openShift := mathutil.Clamp(math.Max(0.15*atr, 0.20*vwapGap), 0.05*atr, 0.45*atr)
		closeShift := mathutil.Clamp(math.Max(0.35*atr, 0.75*vwapGap), 0.10*atr, 0.95*atr)
		return bistBulletinMicrostructureAdjustment{
			openTarget:    latest.Close - openShift,
			closeTarget:   latest.Close - closeShift,
			strength:      "zayıf-orta",
			confidenceCap: 58,
			reason: fmt.Sprintf(
				"BIST bülten mikro yapı: son seans güçlü yükseldi (%.2f%%), VWAP'a göre %.2f%% uzaklaştı; ertesi seans için kar satışı/mean-reversion ayarı uygulandı.",
				changePct, vwapGapPct,
			),
		}, true
	}
	// Düşüş sonrası tepki / mean-reversion sinyali.
	downSignals := 0
	if changePct <= -1.5 {
		downSignals++
	}
	if vwapGapPct <= -1.0 {
		downSignals++
	}
	if intradayPct <= -1.2 {
		downSignals++
	}
	if cmf20 > 0.02 {
		downSignals++
	}
	if downSignals >= 3 {
		openShift := mathutil.Clamp(math.Max(0.15*atr, 0.20*vwapGap), 0.05*atr, 0.45*atr)
		closeShift := mathutil.Clamp(math.Max(0.35*atr, 0.75*vwapGap), 0.10*atr, 0.95*atr)
		return bistBulletinMicrostructureAdjustment{
			openTarget:    latest.Close + openShift,
			closeTarget:   latest.Close + closeShift,
			strength:      "zayıf-orta",
			confidenceCap: 58,
			reason: fmt.Sprintf(
				"BIST bülten mikro yapı: son seans güçlü düştü (%.2f%%), VWAP'a göre %.2f%% uzaklaştı; ertesi seans için tepki/mean-reversion ayarı uygulandı.",
				changePct, vwapGapPct,
			),
		}, true
	}
	return bistBulletinMicrostructureAdjustment{}, false
}

func bistBulletinRecordsBeforeForecast(records []datasource.DailyBulletinRecord, forecastFor string) []datasource.DailyBulletinRecord {
	forecastFor = strings.TrimSpace(forecastFor)
	out := make([]datasource.DailyBulletinRecord, 0, len(records))
	for _, record := range records {
		if record.Close <= 0 || record.TradingDate == "" {
			continue
		}
		if forecastFor != "" && record.TradingDate >= forecastFor {
			continue
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TradingDate < out[j].TradingDate })
	return out
}

func bistBulletinCMF(records []datasource.DailyBulletinRecord, period int) (float64, bool) {
	if period <= 0 || len(records) == 0 {
		return 0, false
	}
	start := len(records) - period
	if start < 0 {
		start = 0
	}
	mfv := 0.0
	volumeSum := 0.0
	for _, record := range records[start:] {
		high := record.High
		low := record.Low
		closePrice := record.Close
		volume := record.Volume
		if high <= low || closePrice <= 0 || volume <= 0 {
			continue
		}
		multiplier := ((closePrice - low) - (high - closePrice)) / (high - low)
		mfv += multiplier * volume
		volumeSum += volume
	}
	if volumeSum <= 0 {
		return 0, false
	}
	return mfv / volumeSum, true
}

func bistBulletinATR(records []datasource.DailyBulletinRecord, period int) float64 {
	if period <= 0 || len(records) < 2 {
		return 0
	}
	start := len(records) - period
	if start < 1 {
		start = 1
	}
	total := 0.0
	count := 0
	for i := start; i < len(records); i++ {
		record := records[i]
		prevClose := records[i-1].Close
		if record.High <= 0 || record.Low <= 0 || prevClose <= 0 {
			continue
		}
		tr := math.Max(record.High-record.Low, math.Max(math.Abs(record.High-prevClose), math.Abs(record.Low-prevClose)))
		if tr <= 0 {
			continue
		}
		total += tr
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func withForecastModelOverlay(model, overlay string) string {
	model = strings.TrimSpace(model)
	overlay = strings.TrimSpace(overlay)
	if overlay == "" {
		return model
	}
	if model == "" {
		return overlay
	}
	if strings.Contains(strings.ToLower(model), strings.ToLower(overlay)) {
		return model
	}
	return model + "_" + overlay
}

func computeNextSessionForecastModel(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, bias, assetType string, includeValidation bool) NextSessionForecast {
	if len(candles) == 0 {
		return NextSessionForecast{}
	}
	lastCandle := candles[len(candles)-1]
	lastClose := lastCandle.EffectiveClose()
	if lastClose <= 0 || math.IsNaN(lastClose) || math.IsInf(lastClose, 0) {
		return NextSessionForecast{}
	}
	atr := snapshot.ATR14
	if atr <= 0 {
		atr = lastClose * 0.015
	}
	var reasons []string

	// directional bias label
	directionBias := "yatay"
	switch bias {
	case "bullish":
		directionBias = "yükseliş"
	case "bearish":
		directionBias = "düşüş"
	}

	// bias strength: count confirming signals
	confirmCount := 0
	contraCount := 0

	if snapshot.MACDHistogram > 0 {
		if bias == "bullish" {
			confirmCount++
			reasons = append(reasons, "MACD histogramı pozitif (momentum yükselen)")
		} else if bias == "bearish" {
			contraCount++
		}
	} else if snapshot.MACDHistogram < 0 {
		if bias == "bearish" {
			confirmCount++
			reasons = append(reasons, "MACD histogramı negatif (momentum düşen)")
		} else if bias == "bullish" {
			contraCount++
		}
	}

	if snapshot.RSI14 > 55 {
		if bias == "bullish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("RSI14=%.1f yükselen momentum", snapshot.RSI14))
		}
	} else if snapshot.RSI14 < 45 {
		if bias == "bearish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("RSI14=%.1f düşen momentum", snapshot.RSI14))
		}
	}

	if snapshot.ChaikinMoneyFlow20 > 0.02 {
		if bias == "bullish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("CMF20=%.3f alım baskısı var", snapshot.ChaikinMoneyFlow20))
		}
	} else if snapshot.ChaikinMoneyFlow20 < -0.02 {
		if bias == "bearish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("CMF20=%.3f satış baskısı var", snapshot.ChaikinMoneyFlow20))
		} else if bias == "bullish" {
			contraCount++
			reasons = append(reasons, fmt.Sprintf("CMF20=%.3f satış baskısı (karşı sinyal)", snapshot.ChaikinMoneyFlow20))
		}
	}

	if snapshot.EMA20 > 0 && lastClose > snapshot.EMA20 {
		if bias == "bullish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("Fiyat EMA20 (%.2f) üzerinde", snapshot.EMA20))
		}
	} else if snapshot.EMA20 > 0 && lastClose < snapshot.EMA20 {
		if bias == "bearish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("Fiyat EMA20 (%.2f) altında", snapshot.EMA20))
		}
	}

	if snapshot.Supertrend > 0 {
		if lastClose > snapshot.Supertrend && bias == "bullish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("Supertrend yükseliş tarafında (%.2f)", snapshot.Supertrend))
		} else if lastClose < snapshot.Supertrend && bias == "bearish" {
			confirmCount++
			reasons = append(reasons, fmt.Sprintf("Supertrend düşüş tarafında (%.2f)", snapshot.Supertrend))
		}
	}

	biasStrength := "zayıf"
	if confirmCount >= 4 {
		biasStrength = "güçlü"
	} else if confirmCount >= 2 {
		biasStrength = "orta"
	}
	if contraCount > confirmCount {
		biasStrength = "çelişkili"
		directionBias = "yatay"
	}

	if bias == "neutral" || directionBias == "yatay" {
		reasons = append(reasons, "Yön sinyalleri çelişkili veya yetersiz; yatay/konsolidasyon beklentisi")
	}

	direction := 0.0
	if directionBias == "yükseliş" {
		direction = 1
	} else if directionBias == "düşüş" {
		direction = -1
	}
	strengthFactor := 0.08
	switch biasStrength {
	case "orta":
		strengthFactor = 0.18
	case "güçlü":
		strengthFactor = 0.28
	case "çelişkili":
		strengthFactor = 0
	}
	atrPct := atr / lastClose
	technicalDrift := direction * strengthFactor * atrPct
	gapMean, intradayMean, samples := weightedNextSessionReturns(candles, 20)
	openModel := nextSessionOpenForecastModel(candles, snapshot, lastClose, atr, gapMean, technicalDrift)
	closeModel := nextSessionCloseForecastModel(candles, snapshot, lastClose, atr, intradayMean, openModel.Return, technicalDrift)
	predictedOpen := lastClose * (1 + openModel.Return)
	predictedClose := predictedOpen * (1 + closeModel.IntradayReturn)
	openClamp := math.Max(0.85*atr, lastClose*0.012)
	closeClamp := math.Max(1.75*atr, lastClose*0.025)
	predictedOpen = math.Max(lastClose-openClamp, math.Min(lastClose+openClamp, predictedOpen))
	predictedClose = math.Max(lastClose-closeClamp, math.Min(lastClose+closeClamp, predictedClose))
	openSpan := math.Max(0.45*atr, math.Abs(predictedOpen-lastClose)*1.35)
	closeSpan := math.Max(0.95*atr, math.Abs(predictedClose-predictedOpen)*1.75)
	expectedLow := math.Min(predictedOpen-openSpan, predictedClose-closeSpan)
	expectedHigh := math.Max(predictedOpen+openSpan, predictedClose+closeSpan)
	confidence := 35 + math.Min(float64(samples), 20)*1.5 + float64(confirmCount)*3 - float64(contraCount)*4
	if openModel.Uncertain || closeModel.Uncertain {
		confidence = math.Min(confidence, 42)
	}
	confidence = mathutil.Clamp(confidence, 25, 72)
	warnings := []string{"point_forecast_is_not_a_price_guarantee"}
	warnings = append(warnings, openModel.Warnings...)
	warnings = append(warnings, closeModel.Warnings...)
	if samples < 10 {
		warnings = append(warnings, "limited_historical_open_close_samples")
	}
	reasons = append(reasons, openModel.Reasons...)
	reasons = append(reasons, closeModel.Reasons...)
	rawPredictedOpen := roundForecastPrice(predictedOpen)
	rawPredictedClose := roundForecastPrice(predictedClose)
	rawExpectedLow := roundForecastPrice(expectedLow)
	rawExpectedHigh := roundForecastPrice(expectedHigh)
	volRegime := nextSessionVolatilityRegime(snapshot, lastClose)
	f := NextSessionForecast{
		Computed:          true,
		ForecastFor:       nextForecastSessionDate(lastCandle.Time, assetType),
		LastClose:         roundForecastPrice(lastClose),
		PredictedOpen:     rawPredictedOpen,
		PredictedClose:    rawPredictedClose,
		RawPredictedOpen:  rawPredictedOpen,
		RawPredictedClose: rawPredictedClose,
		OpenChangePct:     roundForecastMetric(100 * (rawPredictedOpen/lastClose - 1)),
		CloseChangePct:    roundForecastMetric(100 * (rawPredictedClose/lastClose - 1)),
		ExpectedLow:       rawExpectedLow,
		ExpectedHigh:      rawExpectedHigh,
		RawExpectedLow:    rawExpectedLow,
		RawExpectedHigh:   rawExpectedHigh,
		Status:            "mathematically_consistent",
		Quality:           "technical_model",
		DirectionBias:     directionBias,
		BiasStrength:      biasStrength,
		Confidence:        roundForecastMetric(confidence),
		ConfidenceLabel:   nextSessionConfidenceLabel(confidence),
		HistoricalSamples: samples,
		Model:             "separate_open_gap_close_intraday_v2",
		BiasReasons:       reasons,
		Warnings:          warnings,
		InsufficientData:  samples < 10,
	}
	f.DecisionForecast.VolatilityRegime = volRegime
	if snapshot.PivotS1 > 0 {
		f.PivotS1 = roundForecastPrice(snapshot.PivotS1)
	}
	if snapshot.PivotR1 > 0 {
		f.PivotR1 = roundForecastPrice(snapshot.PivotR1)
	}
	f = attachNextSessionForecastScenario(f, candles)
	f = applyTradablePriceStepToNextSessionForecast(f, assetType, "")
	if includeValidation {
		f = attachNextSessionForecastValidation(f, candles, assetType, "ohlcv_provider")
	}
	return f
}

func computeNextSessionForecast(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, bias, assetType string) NextSessionForecast {
	return ComputeNextSessionForecast(candles, snapshot, bias, assetType)
}

func computeNextSessionForecastWithTechnicalContext(ctx context.Context, candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, bias, assetType string, indicatorSignals []ohlcv.IndicatorResult, pats []ohlcv.PatternResult, patternCandidates []ohlcv.PatternResult, sr supportresistance.Result, plan ohlcv.TradePlan, symbol string) NextSessionForecast {
	forecast := computeNextSessionForecastModel(candles, snapshot, bias, assetType, true)
	return applyNextSessionTechnicalDecisionContext(forecast, candles, snapshot, indicatorSignals, pats, patternCandidates, sr, plan, assetType, symbol)
}

func nextSessionForecastFastPatterns(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot) ([]ohlcv.PatternResult, error) {
	out := []ohlcv.PatternResult{}
	candlestick, err := patterns.DetectCandlestick(candles, snapshot.VolumeSMA20)
	if err != nil {
		return nil, err
	}
	out = append(out, candlestick...)
	if len(candles) >= 20 {
		chartPatterns, err := patterns.DetectChartPatterns(candles, snapshot.VolumeSMA20)
		if err != nil {
			return nil, err
		}
		out = append(out, chartPatterns...)
	}
	return out, nil
}

func nextSessionForecastPatterns(ctx context.Context, candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, sr supportresistance.Result) ([]ohlcv.PatternResult, error) {
	scanCandles, offset := nextSessionPatternScanWindow(candles)
	scanOutput, err := patterns.Scan(ctx, patterns.ScannerInput{
		Timeframe:         "1D",
		Candles:           scanCandles,
		Indicators:        snapshot,
		SupportResistance: sr,
	})
	if err == nil {
		return shiftNextSessionPatternIndexes(scanOutput.Patterns, offset), nil
	}
	fallback, fallbackErr := nextSessionForecastFastPatterns(candles, snapshot)
	if fallbackErr != nil {
		return nil, fmt.Errorf("full pattern scan failed: %w; fallback failed: %v", err, fallbackErr)
	}
	return fallback, nil
}

func nextSessionPatternScanWindow(candles []ohlcv.Candle) ([]ohlcv.Candle, int) {
	const maxForecastPatternCandles = 180
	if len(candles) <= maxForecastPatternCandles {
		return candles, 0
	}
	offset := len(candles) - maxForecastPatternCandles
	return candles[offset:], offset
}

func shiftNextSessionPatternIndexes(patterns []ohlcv.PatternResult, offset int) []ohlcv.PatternResult {
	if offset <= 0 || len(patterns) == 0 {
		return patterns
	}
	out := append([]ohlcv.PatternResult{}, patterns...)
	for i := range out {
		if out[i].StartIndex >= 0 {
			out[i].StartIndex += offset
		}
		if out[i].EndIndex >= 0 {
			out[i].EndIndex += offset
		}
		if out[i].SetupCompleteIndex >= 0 {
			out[i].SetupCompleteIndex += offset
		}
		if out[i].TriggerIndex >= 0 {
			out[i].TriggerIndex += offset
		}
	}
	return out
}

func applyNextSessionTechnicalDecisionContext(f NextSessionForecast, candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, indicatorSignals []ohlcv.IndicatorResult, pats []ohlcv.PatternResult, patternCandidates []ohlcv.PatternResult, sr supportresistance.Result, plan ohlcv.TradePlan, assetType, symbol string) NextSessionForecast {
	if !f.Computed || len(candles) == 0 || f.LastClose <= 0 {
		return f
	}
	lastClose := f.LastClose
	if lastClose <= 0 {
		lastClose = candles[len(candles)-1].EffectiveClose()
	}
	snapshotBull, snapshotBear := indicatorConfluence(snapshot, lastClose)
	indicatorBull := snapshotBull
	indicatorBear := snapshotBear
	indicatorScore := nextSessionConfluenceScore(snapshotBull, snapshotBear)
	indicatorUniverse := nextSessionIndicatorUniverseDecisionScore(indicatorSignals)
	if indicatorUniverse.Directional > 0 {
		indicatorBull = indicatorUniverse.Bullish
		indicatorBear = indicatorUniverse.Bearish
		indicatorScore = indicatorUniverse.Score
	}
	indicatorConsensus := nextSessionConsensusLabel(indicatorScore, 0.25)
	patternUniverse := nextSessionPatternDecisionScore(candles, pats, patternCandidates)
	patternScore := patternUniverse.Score
	activePatterns := patternUniverse.Active
	candidatePatterns := patternUniverse.Candidates
	patternNames := patternUniverse.Names
	patternConsensus := nextSessionConsensusLabel(patternScore, 0.20)
	planScore, planDirection, planStatus := nextSessionPlanDecisionScore(plan)
	levelScore, levelReason := nextSessionLevelDecisionScore(lastClose, snapshot.ATR14, sr)
	featureUniverseScore := mathutil.Clamp(0.55*indicatorScore+0.35*patternScore+0.07*planScore+0.03*levelScore, -1, 1)
	if calibrated, ok := applyNextSessionFullSignalUniverseCalibration(f, featureUniverseScore, snapshot.ATR14, assetType, symbol); ok {
		f = calibrated
	}
	decisionScore := mathutil.Clamp(0.50*indicatorScore+0.35*patternScore+0.10*planScore+0.05*levelScore, -1, 1)
	decisionStatus := "pass"
	blockers := []string{}

	predictedDirection := nextSessionForecastDirectionFromChange(f.CloseChangePct)
	consensusDirection := nextSessionConsensusLabel(decisionScore, 0.30)
	if nextSessionDirectionsConflict(predictedDirection, consensusDirection) {
		f = recalibrateNextSessionForecastToDirection(
			f,
			consensusDirection,
			math.Abs(decisionScore),
			snapshot.ATR14,
			assetType,
			symbol,
			fmt.Sprintf("Teknik konsensüs ham tahmin yönünü düzeltti: fiyat modeli=%s, teknik konsensüs=%s, skor=%.0f/100.", predictedDirection, consensusDirection, decisionScore*100),
		)
		predictedDirection = nextSessionForecastDirectionFromChange(f.CloseChangePct)
	}

	// Count hard direction conflicts between the predicted direction and active signals.
	// A single conflict is normal; 3+ simultaneous conflicts signal a real contradiction.
	conflictCount := 0
	if predictedDirection != "neutral" && indicatorConsensus != "neutral" && predictedDirection != indicatorConsensus {
		conflictCount++
	}
	if predictedDirection != "neutral" && patternConsensus != "neutral" && predictedDirection != patternConsensus {
		conflictCount++
	}
	if predictedDirection != "neutral" && planDirection != "neutral" && predictedDirection != planDirection {
		conflictCount++
	}
	if consensusDirection != "neutral" && predictedDirection != "neutral" && consensusDirection != predictedDirection {
		conflictCount++
	}
	if conflictCount >= 3 {
		blockers = append(blockers, fmt.Sprintf("%d teknik sinyal tahmin yönüyle çelişiyor", conflictCount))
	}
	if plan.Rejected && plan.RiskRewardRatio > 0 {
		blockers = append(blockers, "işlem planı reddedildi: "+emptyAnalysisLabel(plan.RejectReason, "sebep belirtilmedi"))
	}

	if len(blockers) > 0 {
		decisionStatus = "failed"
		f.Status = "technical_decision_context_failed"
		f.Quality = "not_decision_grade"
		f.Confidence = roundForecastMetric(math.Min(f.Confidence, 35))
		f.ConfidenceLabel = nextSessionConfidenceLabel(f.Confidence)
		f.BiasStrength = "karar uygun değil"
		f.Warnings = appendUniqueAnalysisString(f.Warnings, "technical_indicator_pattern_trade_plan_gate_failed")
	} else if math.Abs(decisionScore) < 0.25 {
		decisionStatus = "weak"
		f.Quality = "provisional"
		f.Confidence = roundForecastMetric(math.Min(f.Confidence, 45))
		f.ConfidenceLabel = nextSessionConfidenceLabel(f.Confidence)
		f.BiasStrength = downgradedNextSessionStrength(f.BiasStrength, []string{"teknik karar skoru zayıf"})
		f.Warnings = appendUniqueAnalysisString(f.Warnings, "technical_decision_score_weak")
	}

	f.TechnicalDecisionScore = roundForecastMetric(decisionScore * 100)
	f.TechnicalDecisionStatus = decisionStatus
	f.IndicatorConsensus = indicatorConsensus
	f.PatternConsensus = patternConsensus
	f.TradePlanStatus = planStatus
	f.Model = withForecastModelOverlay(f.Model, "indicator_pattern_gate_v1")
	f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, fmt.Sprintf(
		"Teknik karar kapısı: durum=%s, skor=%.0f/100, full_indikatör=%s (%d/%d yönlü, %d computed), formasyon=%s (%d aktif, %d aday), işlem planı=%s.",
		decisionStatus,
		f.TechnicalDecisionScore,
		indicatorConsensus,
		indicatorBull,
		indicatorBear,
		indicatorUniverse.Computed,
		patternConsensus,
		activePatterns,
		candidatePatterns,
		planStatus,
	))
	if len(indicatorUniverse.Names) > 0 {
		f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, "Full indikatör evreni etkisi: "+strings.Join(indicatorUniverse.Names, "; "))
	}
	if len(patternNames) > 0 {
		f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, "Full formasyon evreni etkisi: "+strings.Join(patternNames, "; "))
	}
	if levelReason != "" {
		f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, levelReason)
	}
	if len(blockers) > 0 {
		f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, "Karar kapısı engelleri: "+strings.Join(blockers, "; "))
	}
	f = applyTradablePriceStepToNextSessionForecast(f, assetType, symbol)
	return syncNextSessionDecisionForecast(f, symbol)
}

func nextSessionConfluenceScore(bull, bear int) float64 {
	total := bull + bear
	if total <= 0 {
		return 0
	}
	return mathutil.Clamp(float64(bull-bear)/float64(total), -1, 1)
}

type nextSessionSignalUniverseScore struct {
	Score       float64
	Bullish     int
	Bearish     int
	Computed    int
	Directional int
	Names       []string
}

type nextSessionPatternUniverseScore struct {
	Score      float64
	Bullish    int
	Bearish    int
	Active     int
	Candidates int
	Names      []string
}

type nextSessionWeightedEvidence struct {
	Text   string
	Weight float64
}

func nextSessionIndicatorUniverseDecisionScore(indicators []ohlcv.IndicatorResult) nextSessionSignalUniverseScore {
	out := nextSessionSignalUniverseScore{}
	if len(indicators) == 0 {
		return out
	}
	bullWeight := 0.0
	bearWeight := 0.0
	evidence := []nextSessionWeightedEvidence{}
	for _, indicator := range indicators {
		if !indicator.Computed {
			continue
		}
		out.Computed++
		direction := nextSessionIndicatorSignalDirection(indicator.Signal)
		if direction == 0 || indicator.Confidence <= 0 {
			continue
		}
		weight := mathutil.Clamp(indicator.Confidence, 0.05, 1)
		out.Directional++
		if direction > 0 {
			out.Bullish++
			bullWeight += weight
		} else {
			out.Bearish++
			bearWeight += weight
		}
		evidence = append(evidence, nextSessionWeightedEvidence{
			Text:   fmt.Sprintf("%s %s %.0f%%", indicator.Name, localize.Direction(indicator.Signal), 100*weight),
			Weight: weight,
		})
	}
	total := bullWeight + bearWeight
	if total <= 0 {
		return out
	}
	out.Score = mathutil.Clamp((bullWeight-bearWeight)/total, -1, 1)
	out.Names = topNextSessionWeightedEvidence(evidence, 6)
	return out
}

func nextSessionIndicatorSignalDirection(signal string) int {
	value := strings.ToLower(strings.TrimSpace(signal))
	switch {
	case value == "bullish" || value == "buy" || value == "long" || value == "positive" || value == "up":
		return 1
	case value == "bearish" || value == "sell" || value == "short" || value == "negative" || value == "down":
		return -1
	default:
		return 0
	}
}

func nextSessionDirectionsConflict(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	return (a == "bullish" && b == "bearish") || (a == "bearish" && b == "bullish")
}

func applyNextSessionFullSignalUniverseCalibration(f NextSessionForecast, score, atr float64, assetType, symbol string) (NextSessionForecast, bool) {
	if !f.Computed || f.LastClose <= 0 || math.Abs(score) < 0.08 {
		return f, false
	}
	if atr <= 0 {
		atr = f.LastClose * 0.015
	}
	atrPct := safeRatio(atr, f.LastClose)
	if atrPct <= 0 {
		atrPct = 0.015
	}
	rawOpen := forecastRawPredictedOpen(f)
	rawClose := forecastRawPredictedClose(f)
	if rawOpen <= 0 || rawClose <= 0 {
		return f, false
	}
	move := score * mathutil.Clamp(0.45*atrPct, 0.0015, 0.018)
	openAdj := 0.35 * move
	closeAdj := 0.85 * move
	openClamp := math.Max(1.05*atr, f.LastClose*0.015)
	closeClamp := math.Max(2.10*atr, f.LastClose*0.030)
	nextOpen := mathutil.Clamp(rawOpen*(1+openAdj), f.LastClose-openClamp, f.LastClose+openClamp)
	nextClose := mathutil.Clamp(rawClose*(1+closeAdj), f.LastClose-closeClamp, f.LastClose+closeClamp)
	if math.Abs(nextOpen-rawOpen) < 1e-9 && math.Abs(nextClose-rawClose) < 1e-9 {
		return f, false
	}

	f.RawPredictedOpen = roundForecastPrice(nextOpen)
	f.RawPredictedClose = roundForecastPrice(nextClose)
	f.PredictedOpen = f.RawPredictedOpen
	f.PredictedClose = f.RawPredictedClose
	lowCandidate := math.Min(f.RawPredictedOpen, f.RawPredictedClose)
	highCandidate := math.Max(f.RawPredictedOpen, f.RawPredictedClose)
	rawLow := forecastRawExpectedLow(f)
	rawHigh := forecastRawExpectedHigh(f)
	if rawLow <= 0 {
		rawLow = f.LastClose - math.Max(0.85*atr, f.LastClose*0.018)
	}
	if rawHigh <= 0 {
		rawHigh = f.LastClose + math.Max(0.85*atr, f.LastClose*0.018)
	}
	f.RawExpectedLow = roundForecastPrice(math.Min(rawLow, lowCandidate))
	f.RawExpectedHigh = roundForecastPrice(math.Max(rawHigh, highCandidate))
	f.ExpectedLow = f.RawExpectedLow
	f.ExpectedHigh = f.RawExpectedHigh
	if f.LastClose > 0 {
		f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
		f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
	}
	switch {
	case f.CloseChangePct >= 0.35:
		f.DirectionBias = "yükseliş"
	case f.CloseChangePct <= -0.35:
		f.DirectionBias = "düşüş"
	default:
		f.DirectionBias = "yatay"
	}
	f.Model = withForecastModelOverlay(f.Model, "full_signal_universe_v1")
	f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, fmt.Sprintf(
		"Full sinyal evreni fiyat kalibrasyonu: skor=%.0f/100, ATR=%.2f%%, açılış ayarı=%+.2f%%, kapanış ayarı=%+.2f%%.",
		score*100,
		atrPct*100,
		openAdj*100,
		closeAdj*100,
	))
	f.Warnings = appendUniqueAnalysisString(f.Warnings, "full_indicator_pattern_universe_price_overlay_applied")
	return applyTradablePriceStepToNextSessionForecast(f, assetType, symbol), true
}

func recalibrateNextSessionForecastToDirection(f NextSessionForecast, direction string, strength, atr float64, assetType, symbol, reason string) NextSessionForecast {
	if !f.Computed || f.LastClose <= 0 {
		return f
	}
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction != "bullish" && direction != "bearish" {
		return f
	}
	if atr <= 0 {
		atr = f.LastClose * 0.015
	}
	strength = mathutil.Clamp(strength, 0.30, 1)
	sign := 1.0
	biasLabel := "yükseliş"
	if direction == "bearish" {
		sign = -1
		biasLabel = "düşüş"
	}
	openMove := sign * atr * (0.08 + 0.12*strength)
	closeMove := sign * atr * (0.16 + 0.24*strength)
	rawOpen := roundForecastPrice(f.LastClose + openMove)
	rawClose := roundForecastPrice(f.LastClose + closeMove)
	if rawOpen <= 0 || rawClose <= 0 {
		return f
	}
	f.RawPredictedOpen = rawOpen
	f.RawPredictedClose = rawClose
	f.PredictedOpen = rawOpen
	f.PredictedClose = rawClose
	lowCandidate := math.Min(rawOpen, rawClose)
	highCandidate := math.Max(rawOpen, rawClose)
	rawLow := forecastRawExpectedLow(f)
	rawHigh := forecastRawExpectedHigh(f)
	if rawLow <= 0 {
		rawLow = math.Min(f.LastClose-0.7*atr, lowCandidate)
	}
	if rawHigh <= 0 {
		rawHigh = math.Max(f.LastClose+0.7*atr, highCandidate)
	}
	f.RawExpectedLow = roundForecastPrice(math.Min(rawLow, lowCandidate))
	f.RawExpectedHigh = roundForecastPrice(math.Max(rawHigh, highCandidate))
	f.ExpectedLow = f.RawExpectedLow
	f.ExpectedHigh = f.RawExpectedHigh
	if f.LastClose > 0 {
		f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
		f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
	}
	f.DirectionBias = biasLabel
	f.BiasStrength = downgradedNextSessionStrength(f.BiasStrength, []string{"teknik konsensüs yön kalibrasyonu"})
	f.Model = withForecastModelOverlay(f.Model, "technical_consensus_direction_calibration_v1")
	if strings.TrimSpace(reason) != "" {
		f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, reason)
	}
	f.Warnings = appendUniqueAnalysisString(f.Warnings, "technical_consensus_recalibrated_forecast_direction")
	return applyTradablePriceStepToNextSessionForecast(f, assetType, symbol)
}

func nextSessionForecastDirectionFromChange(changePct float64) string {
	return forecastpolicy.BiasFromChangePct(changePct)
}

func nextSessionConsensusLabel(score, threshold float64) string {
	switch {
	case score >= threshold:
		return "bullish"
	case score <= -threshold:
		return "bearish"
	default:
		return "neutral"
	}
}

func nextSessionPatternDecisionScore(candles []ohlcv.Candle, pats []ohlcv.PatternResult, candidates []ohlcv.PatternResult) nextSessionPatternUniverseScore {
	out := nextSessionPatternUniverseScore{}
	if len(candles) == 0 || (len(pats) == 0 && len(candidates) == 0) {
		return out
	}
	lastIndex := len(candles) - 1
	bullish := 0.0
	bearish := 0.0
	seen := map[string]struct{}{}
	evidence := []nextSessionWeightedEvidence{}
	for _, pattern := range pats {
		weight, direction, ok := nextSessionPatternContribution(pattern, lastIndex, true)
		if !ok {
			continue
		}
		seen[nextSessionPatternUniverseKey(pattern)] = struct{}{}
		if direction > 0 {
			bullish += weight
			out.Bullish++
		} else {
			bearish += weight
			out.Bearish++
		}
		out.Active++
		evidence = append(evidence, nextSessionWeightedEvidence{
			Text:   fmt.Sprintf("%s %s %.0f%%", localize.PatternName(pattern.Name), localize.Direction(pattern.Direction), 100*weight),
			Weight: weight,
		})
	}
	for _, pattern := range candidates {
		key := nextSessionPatternUniverseKey(pattern)
		if _, ok := seen[key]; ok {
			continue
		}
		weight, direction, ok := nextSessionPatternContribution(pattern, lastIndex, false)
		if !ok {
			continue
		}
		seen[key] = struct{}{}
		if direction > 0 {
			bullish += weight
			out.Bullish++
		} else {
			bearish += weight
			out.Bearish++
		}
		out.Candidates++
		evidence = append(evidence, nextSessionWeightedEvidence{
			Text:   fmt.Sprintf("%s aday %s %.0f%%", localize.PatternName(pattern.Name), localize.Direction(pattern.Direction), 100*weight),
			Weight: weight,
		})
	}
	total := bullish + bearish
	if total <= 0 {
		return out
	}
	out.Score = mathutil.Clamp((bullish-bearish)/total, -1, 1)
	out.Names = topNextSessionWeightedEvidence(evidence, 6)
	return out
}

func nextSessionPatternContribution(pattern ohlcv.PatternResult, lastIndex int, active bool) (float64, int, bool) {
	direction := nextSessionPatternDirection(pattern.Direction)
	if direction == 0 {
		return 0, 0, false
	}
	weight := mathutil.Clamp(firstPositiveFloat(pattern.SignalScore, pattern.CalibratedConfidence, pattern.Confidence), 0, 1)
	if weight <= 0 {
		return 0, 0, false
	}
	if pattern.EndIndex > 0 {
		distance := lastIndex - pattern.EndIndex
		if distance < 0 || distance > 5 {
			return 0, 0, false
		}
		weight *= mathutil.Clamp(1-float64(distance)*0.12, 0.35, 1)
	}
	if active {
		if !pattern.Actionable && !pattern.Tradeable && pattern.TradeValue == "" {
			weight *= 0.70
		}
	} else {
		weight *= 0.55
		if pattern.Actionable || pattern.Tradeable {
			weight *= 1.20
		}
		if hasNextSessionPatternRejection(pattern, "not_current_completed_pattern") {
			weight *= 0.65
		}
		if hasNextSessionPatternRejection(pattern, "calibrated_confidence_below_threshold") {
			weight *= 0.70
		}
		if hasNextSessionPatternRejection(pattern, "backtest_metadata_not_ready") {
			weight *= 0.85
		}
	}
	if pattern.VolumeConfirmed {
		weight *= 1.20
	}
	if strings.EqualFold(pattern.TradeValue, "high") || strings.EqualFold(pattern.TradeValue, "strong") {
		weight *= 1.12
	}
	if pattern.BacktestReady && pattern.BacktestSampleSize > 0 {
		switch {
		case pattern.BacktestWinRate > 0 && pattern.BacktestWinRate < 0.45:
			weight *= 0.55
		case pattern.BacktestWinRate >= 0.55:
			weight *= 1.10
		}
		if pattern.BacktestExpectancyR < 0 {
			weight *= 0.65
		} else if pattern.BacktestExpectancyR > 0.05 {
			weight *= 1.08
		}
	}
	return mathutil.Clamp(weight, 0, 1.25), direction, true
}

func nextSessionPatternDirection(direction string) int {
	value := strings.ToLower(strings.TrimSpace(direction))
	switch {
	case strings.Contains(value, "bull") || strings.Contains(value, "long") || strings.Contains(value, "up"):
		return 1
	case strings.Contains(value, "bear") || strings.Contains(value, "short") || strings.Contains(value, "down"):
		return -1
	default:
		return 0
	}
}

func nextSessionPatternUniverseKey(pattern ohlcv.PatternResult) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(pattern.Name)),
		strings.ToLower(strings.TrimSpace(pattern.Direction)),
		fmt.Sprintf("%d", pattern.EndIndex),
	}, "|")
}

func hasNextSessionPatternRejection(pattern ohlcv.PatternResult, reason string) bool {
	for _, item := range pattern.RejectionReasons {
		if strings.EqualFold(strings.TrimSpace(item), reason) {
			return true
		}
	}
	return false
}

func topNextSessionWeightedEvidence(items []nextSessionWeightedEvidence, limit int) []string {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		if math.Abs(items[i].Weight-items[j].Weight) > 1e-9 {
			return items[i].Weight > items[j].Weight
		}
		return items[i].Text < items[j].Text
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Text)
	}
	return out
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func nextSessionPlanDecisionScore(plan ohlcv.TradePlan) (float64, string, string) {
	if plan.Rejected {
		return 0, "neutral", "rejected"
	}
	direction := "neutral"
	score := 0.0
	switch strings.ToLower(strings.TrimSpace(plan.Direction)) {
	case "long", "bullish", "buy", "al":
		direction = "bullish"
		score = 1
	case "short", "bearish", "sell", "sat":
		direction = "bearish"
		score = -1
	default:
		return 0, "neutral", "not_actionable"
	}
	if plan.RiskRewardRatio > 0 && plan.RiskRewardRatio < 1.5 {
		score *= 0.35
	}
	if plan.ConfidenceScore > 0 {
		score *= nextSessionTradePlanConfidenceFactor(plan.ConfidenceScore)
	}
	return score, direction, "actionable_" + direction
}

func nextSessionTradePlanConfidenceFactor(value float64) float64 {
	if value <= 0 {
		return 1
	}
	if value > 1 {
		value = value / 100
	}
	return mathutil.Clamp(value, 0.25, 1)
}

func nextSessionLevelDecisionScore(lastClose, atr float64, sr supportresistance.Result) (float64, string) {
	if lastClose <= 0 {
		return 0, ""
	}
	if atr <= 0 {
		atr = lastClose * 0.02
	}
	score := 0.0
	reasons := []string{}
	if sr.NearestResistance != nil && sr.NearestResistance.Price > lastClose {
		dist := (sr.NearestResistance.Price - lastClose) / math.Max(atr, mathutil.Epsilon)
		if dist <= 0.75 {
			score -= mathutil.Clamp((0.75-dist)/0.75, 0, 1)
			reasons = append(reasons, fmt.Sprintf("yakın direnç %.2f TL", sr.NearestResistance.Price))
		}
	}
	if sr.NearestSupport != nil && sr.NearestSupport.Price < lastClose {
		dist := (lastClose - sr.NearestSupport.Price) / math.Max(atr, mathutil.Epsilon)
		if dist <= 0.75 {
			score += mathutil.Clamp((0.75-dist)/0.75, 0, 1)
			reasons = append(reasons, fmt.Sprintf("yakın destek %.2f TL", sr.NearestSupport.Price))
		}
	}
	if len(reasons) == 0 {
		return 0, ""
	}
	return mathutil.Clamp(score, -1, 1), "Destek/direnç karar etkisi: " + strings.Join(reasons, "; ")
}

// ApplyFundamentalContextToNextSessionForecast keeps the next-session model
// anchored in short-term price behavior, then applies a bounded fundamental
// overlay after the professional report has been assembled.
func ApplyFundamentalContextToNextSessionForecast(result SymbolAnalysis) SymbolAnalysis {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return result
	}
	forecast := result.NextSessionForecast
	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		if daily, ok := result.Timeframes["1D"]; ok {
			forecast = daily.NextSessionForecast
		}
	}
	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		return result
	}
	if nextSessionForecastHasFundamentalContext(forecast) || !nextSessionProfessionalContextAvailable(result.Professional) {
		forecast = applyTradablePriceStepToNextSessionForecast(forecast, result.AssetType, result.Symbol)
		result.NextSessionForecast = forecast
		if daily, ok := result.Timeframes["1D"]; ok {
			daily.NextSessionForecast = forecast
			result.Timeframes["1D"] = daily
		}
		return ApplyNextSessionForecastQualityContext(result)
	}
	adjustment := nextSessionFundamentalAdjustment(result.Professional, forecast.LastClose)
	if len(adjustment.reasons) == 0 {
		forecast = applyTradablePriceStepToNextSessionForecast(forecast, result.AssetType, result.Symbol)
		result.NextSessionForecast = forecast
		if daily, ok := result.Timeframes["1D"]; ok {
			daily.NextSessionForecast = forecast
			result.Timeframes["1D"] = daily
		}
		return ApplyNextSessionForecastQualityContext(result)
	}

	openReturnAdj := mathutil.Clamp(adjustment.openBps/10000, -0.030, 0.030)
	closeReturnAdj := mathutil.Clamp(adjustment.closeBps/10000, -0.050, 0.050)
	rawPredictedOpen := forecastRawPredictedOpen(forecast)
	rawPredictedClose := forecastRawPredictedClose(forecast)
	forecast.RawPredictedOpen = roundForecastPrice(rawPredictedOpen * (1 + openReturnAdj))
	forecast.RawPredictedClose = roundForecastPrice(rawPredictedClose * (1 + closeReturnAdj))
	forecast.PredictedOpen = forecast.RawPredictedOpen
	forecast.PredictedClose = forecast.RawPredictedClose
	if forecast.LastClose > 0 {
		forecast.OpenChangePct = roundForecastMetric(100 * (forecast.PredictedOpen/forecast.LastClose - 1))
		forecast.CloseChangePct = roundForecastMetric(100 * (forecast.PredictedClose/forecast.LastClose - 1))
	}
	lowCandidate := math.Min(forecast.RawPredictedOpen, forecast.RawPredictedClose)
	highCandidate := math.Max(forecast.RawPredictedOpen, forecast.RawPredictedClose)
	rawExpectedLow := forecastRawExpectedLow(forecast)
	rawExpectedHigh := forecastRawExpectedHigh(forecast)
	if rawExpectedLow <= 0 || rawExpectedHigh <= 0 {
		center := forecast.LastClose
		if center <= 0 {
			center = (lowCandidate + highCandidate) / 2
		}
		span := math.Max(math.Abs(highCandidate-center), center*0.015)
		forecast.RawExpectedLow = roundForecastPrice(math.Min(lowCandidate, center-span))
		forecast.RawExpectedHigh = roundForecastPrice(math.Max(highCandidate, center+span))
	} else {
		forecast.RawExpectedLow = roundForecastPrice(math.Min(rawExpectedLow, lowCandidate))
		forecast.RawExpectedHigh = roundForecastPrice(math.Max(rawExpectedHigh, highCandidate))
	}
	forecast.ExpectedLow = forecast.RawExpectedLow
	forecast.ExpectedHigh = forecast.RawExpectedHigh
	forecast = applyTradablePriceStepToNextSessionForecast(forecast, result.AssetType, result.Symbol)
	if forecast.LastClose > 0 {
		switch {
		case forecast.CloseChangePct >= 0.35:
			forecast.DirectionBias = "yükseliş"
		case forecast.CloseChangePct <= -0.35:
			forecast.DirectionBias = "düşüş"
		default:
			forecast.DirectionBias = "yatay"
		}
	}
	forecast.Confidence = roundForecastMetric(mathutil.Clamp(forecast.Confidence+adjustment.confidenceAdj, 20, 78))
	forecast.ConfidenceLabel = nextSessionConfidenceLabel(forecast.Confidence)
	forecast.Model = withForecastModelOverlay(forecast.Model, "atr_gap_intraday_ewma_fundamental_v2")
	if forecast.Status == "" {
		forecast.Status = "mathematically_consistent"
	}
	if forecast.Quality == "" || forecast.Quality == "technical_model" {
		forecast.Quality = "fundamental_overlay"
	}
	for _, reason := range adjustment.reasons {
		forecast.BiasReasons = appendUniqueAnalysisString(forecast.BiasReasons, reason)
	}
	forecast.Warnings = appendUniqueAnalysisString(forecast.Warnings, "fundamental_context_is_slow_moving_not_intraday_price_guarantee")

	result.NextSessionForecast = forecast
	if daily, ok := result.Timeframes["1D"]; ok {
		daily.NextSessionForecast = forecast
		result.Timeframes["1D"] = daily
	}
	return ApplyNextSessionForecastQualityContext(result)
}

// ApplyNextSessionForecastQualityContext downgrades interpretation when the
// computed price points are internally consistent but trading/decision gates do
// not support a strong directional label.
func ApplyNextSessionForecastQualityContext(result SymbolAnalysis) SymbolAnalysis {
	forecast := result.NextSessionForecast
	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		if daily, ok := result.Timeframes["1D"]; ok {
			forecast = daily.NextSessionForecast
		}
	}
	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		return result
	}
	if forecast.Status == "" {
		forecast.Status = "mathematically_consistent"
	}
	if forecast.Quality == "" {
		forecast.Quality = "technical_model"
	}
	forecast = reconcileNextSessionForecastTechnicalSignalGate(forecast, result)
	issues := nextSessionForecastQualityIssues(result)
	if len(issues) > 0 {
		hardFailure := nextSessionForecastHardDecisionFailure(forecast)
		if hardFailure {
			if forecast.Quality == "" || forecast.Quality == "technical_model" || forecast.Quality == "provisional" {
				forecast.Quality = "not_decision_grade"
			}
		} else {
			forecast.Status = "mathematically_consistent"
			forecast.Quality = "provisional"
		}
		forecast.Confidence = roundForecastMetric(math.Min(forecast.Confidence, nextSessionForecastConfidenceCap(result, issues)))
		forecast.ConfidenceLabel = nextSessionConfidenceLabel(forecast.Confidence)
		forecast.DirectionBias = downgradedNextSessionDirection(forecast.DirectionBias, forecast.CloseChangePct)
		forecast.BiasStrength = downgradedNextSessionStrength(forecast.BiasStrength, issues)
		forecast.BiasReasons = removeNextSessionQualityGateReasons(forecast.BiasReasons)
		if hardFailure {
			forecast.BiasReasons = appendUniqueAnalysisString(forecast.BiasReasons, "Kalite kapısı: "+strings.Join(issues, "; ")+". Tahmin fiyatı ayrı denetlenir; model karar/emir seviyesi olarak kullanılamaz.")
		} else {
			forecast.BiasReasons = appendUniqueAnalysisString(forecast.BiasReasons, "Kalite kapısı: "+strings.Join(issues, "; ")+". Tahmin fiyatı matematiksel olarak tutarlı, fakat yön/güç yorumu şartlı okunmalı.")
		}
		forecast.Warnings = appendUniqueAnalysisString(forecast.Warnings, "forecast_quality_provisional_gate_context")
	} else {
		forecast.ConfidenceLabel = nextSessionConfidenceLabel(forecast.Confidence)
	}
	if daily, ok := result.Timeframes["1D"]; ok {
		if forecast.OpenP50 <= 0 || forecast.CloseP50 <= 0 || forecast.ForecastDistributionSamples == 0 {
			forecast = attachNextSessionForecastScenario(forecast, daily.Candles)
			forecast = normalizeNextSessionForecastScenario(forecast, result.AssetType, result.Symbol)
		}
		forecast = syncNextSessionForecastTechnicalSignalGateReason(forecast, daily)
		forecast = syncNextSessionForecastDailySignalReasons(forecast, daily.Indicators)
	}
	forecast = applyNextSessionForecastDirectionFields(forecast)
	forecast = ApplyNextSessionForecastPublishState(forecast)
	forecast = syncNextSessionDecisionForecast(forecast, result.Symbol)

	result.NextSessionForecast = forecast
	if daily, ok := result.Timeframes["1D"]; ok {
		daily.NextSessionForecast = forecast
		result.Timeframes["1D"] = daily
	}
	return result
}

func nextSessionForecastHardDecisionFailure(forecast NextSessionForecast) bool {
	return strings.EqualFold(forecast.Status, "model_validation_failed") ||
		strings.EqualFold(forecast.Status, "technical_decision_context_failed") ||
		strings.EqualFold(forecast.Quality, "not_decision_grade")
}

func reconcileNextSessionForecastTechnicalSignalGate(forecast NextSessionForecast, result SymbolAnalysis) NextSessionForecast {
	daily, ok := result.Timeframes["1D"]
	if !ok {
		return forecast
	}
	gate := daily.Professional.Technical.SignalGate
	if gate.Status == "" || (statusPass(gate.Status) && gate.Actionable) {
		return forecast
	}
	lastClose := forecast.LastClose
	if lastClose <= 0 {
		lastClose = daily.LastClose
	}
	bull, bear := indicatorConfluence(daily.Indicators, lastClose)
	indicatorConsensus := nextSessionConsensusLabel(nextSessionConfluenceScore(bull, bear), 0.25)
	predictedDirection := nextSessionForecastDirectionFromChange(forecast.CloseChangePct)
	if nextSessionDirectionsConflict(predictedDirection, indicatorConsensus) {
		forecast = recalibrateNextSessionForecastToDirection(
			forecast,
			indicatorConsensus,
			math.Abs(nextSessionConfluenceScore(bull, bear)),
			daily.Indicators.ATR14,
			result.AssetType,
			result.Symbol,
			fmt.Sprintf("Günlük teknik kapı ham tahmin yönünü düzeltti: fiyat modeli=%s, indikatör konsensüsü=%s (%d/%d).", predictedDirection, indicatorConsensus, bull, bear),
		)
	} else if predictedDirection != "neutral" && indicatorConsensus == "neutral" && !gate.Actionable {
		forecast = dampNextSessionForecastTowardLastClose(
			forecast,
			0.35,
			result.AssetType,
			result.Symbol,
			"Günlük teknik kapı aksiyon üretmedi; ham yön tahmini son kapanışa yaklaştırıldı.",
		)
	}
	forecast.TechnicalDecisionStatus = "failed"
	forecast.TradePlanStatus = "technical_signal_gate_failed"
	if gate.Score > 0 {
		forecast.TechnicalDecisionScore = roundForecastMetric(gate.Score)
	}
	if !strings.EqualFold(forecast.Status, "model_validation_failed") {
		forecast.Status = "technical_decision_context_failed"
	}
	forecast.Quality = "not_decision_grade"
	forecast.Confidence = roundForecastMetric(math.Min(forecast.Confidence, 35))
	forecast.ConfidenceLabel = nextSessionConfidenceLabel(forecast.Confidence)
	forecast.BiasStrength = "karar uygun değil"
	forecast.Warnings = appendUniqueAnalysisString(forecast.Warnings, "technical_signal_gate_not_passed")
	return forecast
}

func dampNextSessionForecastTowardLastClose(f NextSessionForecast, factor float64, assetType, symbol, reason string) NextSessionForecast {
	if !f.Computed || f.LastClose <= 0 {
		return f
	}
	factor = mathutil.Clamp(factor, 0, 1)
	rawOpen := forecastRawPredictedOpen(f)
	rawClose := forecastRawPredictedClose(f)
	if rawOpen <= 0 || rawClose <= 0 {
		return f
	}
	f.RawPredictedOpen = roundForecastPrice(f.LastClose + factor*(rawOpen-f.LastClose))
	f.RawPredictedClose = roundForecastPrice(f.LastClose + factor*(rawClose-f.LastClose))
	f.PredictedOpen = f.RawPredictedOpen
	f.PredictedClose = f.RawPredictedClose
	if f.LastClose > 0 {
		f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
		f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
	}
	f.DirectionBias = downgradedNextSessionDirection(f.DirectionBias, f.CloseChangePct)
	f.BiasStrength = downgradedNextSessionStrength(f.BiasStrength, []string{"teknik kapı aksiyon üretmedi"})
	f.Model = withForecastModelOverlay(f.Model, "technical_gate_direction_damping_v1")
	if strings.TrimSpace(reason) != "" {
		f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, reason)
	}
	f.Warnings = appendUniqueAnalysisString(f.Warnings, "technical_gate_damped_forecast_direction")
	return applyTradablePriceStepToNextSessionForecast(f, assetType, symbol)
}

// ApplyNextSessionForecastPublishState separates the internally computed point
// estimate from the price that may be presented as a published forecast.
func ApplyNextSessionForecastPublishState(forecast NextSessionForecast) NextSessionForecast {
	forecast.PointForecastPublishable = false
	forecast.PointForecastStatus = "not_published"
	forecast.PointForecastSuppressionReason = ""
	forecast.PublishedPredictedOpen = nil
	forecast.PublishedPredictedClose = nil

	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		forecast.PointForecastSuppressionReason = "forecast_not_computed"
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.ActualAvailable {
		forecast.PointForecastStatus = "audit_only"
		forecast.PointForecastSuppressionReason = "official_actual_available"
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.BacktestMetrics.Samples >= nextSessionForecastDecisionBacktestWindow &&
		forecast.BacktestMetrics.CloseMAPE > nextSessionDecisionMaxCloseMAPEPct {
		forecast.PointForecastSuppressionReason = fmt.Sprintf("backtest_close_mape_above_2pct:%.2f", forecast.BacktestMetrics.CloseMAPE)
		forecast.Quality = "not_decision_grade"
		forecast.Confidence = roundForecastMetric(math.Min(forecast.Confidence, 35))
		forecast.ConfidenceLabel = nextSessionConfidenceLabel(forecast.Confidence)
		forecast.Warnings = appendUniqueAnalysisString(forecast.Warnings, "trade_signal_blocked_by_close_mape_above_2pct")
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if strings.EqualFold(forecast.Status, "model_validation_failed") ||
		strings.EqualFold(forecast.Status, "technical_decision_context_failed") ||
		strings.EqualFold(forecast.Quality, "not_decision_grade") {
		forecast.PointForecastSuppressionReason = "forecast_not_decision_grade"
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.TechnicalDecisionStatus != "" && !strings.EqualFold(forecast.TechnicalDecisionStatus, "pass") {
		forecast.PointForecastSuppressionReason = "technical_decision_gate_not_passed"
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.BacktestSamples < nextSessionPointForecastMinBacktestSamples {
		forecast.PointForecastSuppressionReason = fmt.Sprintf("backtest_samples_below_30:%d", forecast.BacktestSamples)
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.BacktestDirectionHitRatePct < nextSessionPointForecastMinDirectionHitPct {
		forecast.PointForecastSuppressionReason = fmt.Sprintf("backtest_direction_hit_below_55pct:%.2f", forecast.BacktestDirectionHitRatePct)
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.BacktestCloseMAEPct <= 0 {
		forecast.PointForecastSuppressionReason = "backtest_close_mae_missing"
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.BacktestCloseMAEPct > nextSessionPointForecastMaxCloseMAEPct {
		forecast.PointForecastSuppressionReason = fmt.Sprintf("backtest_close_mae_above_1_25pct:%.2f", forecast.BacktestCloseMAEPct)
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if strings.EqualFold(forecast.Quality, "provisional") {
		forecast.PointForecastStatus = "scenario_only"
		forecast.PointForecastSuppressionReason = "forecast_quality_provisional"
		return syncNextSessionDecisionForecast(forecast, "")
	}
	if forecast.Confidence > 0 && forecast.Confidence < 55 {
		forecast.PointForecastSuppressionReason = fmt.Sprintf("forecast_confidence_below_55:%.0f", forecast.Confidence)
		return syncNextSessionDecisionForecast(forecast, "")
	}

	open := forecast.PredictedOpen
	closePrice := forecast.PredictedClose
	forecast.PointForecastPublishable = true
	forecast.PointForecastStatus = "published"
	forecast.PublishedPredictedOpen = &open
	forecast.PublishedPredictedClose = &closePrice
	return syncNextSessionDecisionForecast(forecast, "")
}

func syncNextSessionDecisionForecast(f NextSessionForecast, ticker string) NextSessionForecast {
	confidence := "low"
	if f.Confidence >= 65 && !f.DirectionModelUnreliable {
		confidence = "high"
	} else if f.Confidence >= 45 && !strings.EqualFold(f.Quality, "not_decision_grade") {
		confidence = "medium"
	}
	tradeAllowed := f.PointForecastPublishable
	if f.BacktestMetrics.Samples >= nextSessionForecastDecisionBacktestWindow {
		if f.BacktestMetrics.CloseMAPE > nextSessionDecisionMaxCloseMAPEPct {
			tradeAllowed = false
			confidence = "low"
		}
		if f.BacktestMetrics.DirectionAccuracy < nextSessionDecisionMinDirectionAccuracyPct {
			f.DirectionModelUnreliable = true
			tradeAllowed = false
		}
	}
	riskWarnings := append([]string{}, f.Warnings...)
	if f.DirectionModelUnreliable {
		riskWarnings = appendUniqueAnalysisString(riskWarnings, "direction_model_unreliable")
	}
	if !tradeAllowed && strings.TrimSpace(f.PointForecastSuppressionReason) != "" {
		riskWarnings = appendUniqueAnalysisString(riskWarnings, "trade_signal_blocked:"+f.PointForecastSuppressionReason)
	}
	openForecast, closeForecast := 0.0, 0.0
	openRangeLow, openRangeHigh := 0.0, 0.0
	closeRangeLow, closeRangeHigh := 0.0, 0.0
	expectedDirection := "uncertain"
	if tradeAllowed {
		openForecast = f.PredictedOpen
		closeForecast = f.PredictedClose
		if f.PublishedPredictedOpen != nil {
			openForecast = *f.PublishedPredictedOpen
		}
		if f.PublishedPredictedClose != nil {
			closeForecast = *f.PublishedPredictedClose
		}
		if openForecast > 0 && closeForecast > 0 {
			openRangeLow = firstPositiveFloat(f.OpenP10, f.ExpectedLow)
			openRangeHigh = firstPositiveFloat(f.OpenP90, f.ExpectedHigh)
			closeRangeLow = firstPositiveFloat(f.CloseP10, f.ExpectedLow)
			closeRangeHigh = firstPositiveFloat(f.CloseP90, f.ExpectedHigh)
			expectedDirection = nextSessionForecastDirectionForJSON(closeForecast, f.LastClose, true)
		}
	}
	if nextSessionForecastIsUncertain(f) {
		expectedDirection = "uncertain"
		confidence = "low"
		tradeAllowed = false
		openForecast, closeForecast = 0, 0
		openRangeLow, openRangeHigh = 0, 0
		closeRangeLow, closeRangeHigh = 0, 0
	}
	f.DecisionForecast = NextSessionDecisionForecast{
		Date:                      f.ForecastFor,
		Ticker:                    strings.TrimSpace(ticker),
		OpenForecast:              openForecast,
		OpenRangeLow:              openRangeLow,
		OpenRangeHigh:             openRangeHigh,
		CloseForecast:             closeForecast,
		CloseRangeLow:             closeRangeLow,
		CloseRangeHigh:            closeRangeHigh,
		ExpectedIntradayDirection: expectedDirection,
		VolatilityRegime:          emptyAnalysisLabel(f.DecisionForecast.VolatilityRegime, "normal"),
		Confidence:                confidence,
		TradeSignalAllowed:        tradeAllowed,
		ReasoningFactors:          append([]string{}, f.BiasReasons...),
		RiskWarnings:              riskWarnings,
		DirectionModelUnreliable:  f.DirectionModelUnreliable,
		InsufficientData:          f.InsufficientData,
	}
	return f
}

func nextSessionForecastIsUncertain(f NextSessionForecast) bool {
	for _, warning := range f.Warnings {
		if strings.Contains(strings.ToLower(warning), "uncertain") {
			return true
		}
	}
	vol := strings.ToLower(strings.TrimSpace(f.DecisionForecast.VolatilityRegime))
	return vol == "extreme"
}

func nextSessionForecastQualityIssues(result SymbolAnalysis) []string {
	daily, ok := result.Timeframes["1D"]
	if !ok {
		return nil
	}
	issues := []string{}
	if strings.EqualFold(result.NextSessionForecast.Status, "model_validation_failed") {
		issues = appendUniqueAnalysisString(issues, "sonraki seans tahmin modeli geriye dönük doğrulamayı geçmedi")
	}
	gate := daily.Professional.Technical.SignalGate
	if gate.Status != "" && !statusPass(gate.Status) {
		issues = appendUniqueAnalysisString(issues, "teknik sinyal kapısı geçmedi")
	}
	if gate.Status != "" && !gate.Actionable {
		issues = appendUniqueAnalysisString(issues, "aktif işlem sinyali yok")
	}
	if !gate.VolumeConfirmed && gate.VolumeConfirmation != "" {
		issues = appendUniqueAnalysisString(issues, "hacim teyidi yok")
	}
	if daily.Indicators.ChaikinMoneyFlow20 < -0.02 {
		issues = appendUniqueAnalysisString(issues, fmt.Sprintf("CMF20=%.3f satış baskısı", daily.Indicators.ChaikinMoneyFlow20))
	}
	plan := daily.TradePlan
	if plan.RiskRewardRatio > 0 && plan.RiskRewardRatio < 1.5 {
		issues = appendUniqueAnalysisString(issues, fmt.Sprintf("R/R %.2f < 1.50", plan.RiskRewardRatio))
	}
	if !nextSessionTradePlanReady(plan) {
		issues = appendUniqueAnalysisString(issues, "uygulanabilir aktif işlem planı yok")
	}
	if result.DecisionSupport != nil && !decisionSupportAllowsLiveBuySell(*result.DecisionSupport) {
		issues = appendUniqueAnalysisString(issues, "AL/SAT karar kapısı geçmedi")
	}
	return issues
}

func removeNextSessionQualityGateReasons(reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		trimmed := strings.TrimSpace(reason)
		if strings.HasPrefix(trimmed, "Kalite kapısı:") || strings.HasPrefix(trimmed, "Teknik karar kapısı:") {
			continue
		}
		out = append(out, reason)
	}
	return out
}

func syncNextSessionForecastTechnicalSignalGateReason(forecast NextSessionForecast, daily TimeframeAnalysis) NextSessionForecast {
	gate := daily.Professional.Technical.SignalGate
	if gate.Status == "" {
		return forecast
	}
	forecast.BiasReasons = removeNextSessionTechnicalGateReasons(forecast.BiasReasons)
	status := forecast.TechnicalDecisionStatus
	if status == "" {
		status = gate.Status
	}
	if gate.Status != "" && (!statusPass(gate.Status) || !gate.Actionable) {
		status = "failed"
	}
	planStatus := forecast.TradePlanStatus
	if planStatus == "" {
		planStatus = "unknown"
	}
	detail := fmt.Sprintf(
		"Teknik karar kapısı: durum=%s, skor=%.0f/100, profesyonel sinyal kapısı=%s, aktif=%t, işlem planı=%s.",
		status,
		forecast.TechnicalDecisionScore,
		emptyAnalysisLabel(gate.Status, "unknown"),
		gate.Actionable,
		planStatus,
	)
	if len(gate.Blockers) > 0 {
		detail += " Engeller: " + strings.Join(limitAnalysisStrings(gate.Blockers, 3), "; ") + "."
	}
	forecast.BiasReasons = appendUniqueAnalysisString(forecast.BiasReasons, detail)
	return forecast
}

func removeNextSessionTechnicalGateReasons(reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if strings.HasPrefix(strings.TrimSpace(reason), "Teknik karar kapısı:") {
			continue
		}
		out = append(out, reason)
	}
	return out
}

func limitAnalysisStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func syncNextSessionForecastDailySignalReasons(forecast NextSessionForecast, indicators ohlcv.IndicatorSnapshot) NextSessionForecast {
	reasons := make([]string, 0, len(forecast.BiasReasons)+1)
	for _, reason := range forecast.BiasReasons {
		if strings.Contains(strings.ToLower(strings.TrimSpace(reason)), "macd histogram") {
			continue
		}
		reasons = append(reasons, reason)
	}
	switch {
	case indicators.MACDHistogram > 0:
		reasons = appendUniqueAnalysisString(reasons, fmt.Sprintf("Günlük MACD histogramı %.4f: kısa vadeli momentum pozitif.", indicators.MACDHistogram))
	case indicators.MACDHistogram < 0:
		reasons = appendUniqueAnalysisString(reasons, fmt.Sprintf("Günlük MACD histogramı %.4f: kısa vadeli momentum negatif.", indicators.MACDHistogram))
	case indicators.MACD != 0 || indicators.MACDSignal != 0:
		reasons = appendUniqueAnalysisString(reasons, "Günlük MACD histogramı nötr; momentum tek başına karar üretmiyor.")
	}
	forecast.BiasReasons = reasons
	return forecast
}

func nextSessionForecastConfidenceCap(result SymbolAnalysis, issues []string) float64 {
	capValue := 58.0
	daily, ok := result.Timeframes["1D"]
	if ok {
		gate := daily.Professional.Technical.SignalGate
		if gate.Status != "" && !statusPass(gate.Status) {
			capValue = math.Min(capValue, 55)
		}
		if !gate.VolumeConfirmed || daily.Indicators.ChaikinMoneyFlow20 < -0.02 {
			capValue = math.Min(capValue, 54)
		}
		if daily.TradePlan.RiskRewardRatio > 0 && daily.TradePlan.RiskRewardRatio < 1.5 {
			capValue = math.Min(capValue, 53)
		}
		if !nextSessionTradePlanReady(daily.TradePlan) {
			capValue = math.Min(capValue, 52)
		}
	}
	if len(issues) >= 5 {
		capValue = math.Min(capValue, 52)
	}
	if result.DecisionSupport != nil && !decisionSupportAllowsLiveBuySell(*result.DecisionSupport) {
		capValue = math.Min(capValue, 52)
	}
	return capValue
}

func nextSessionTradePlanReady(plan ohlcv.TradePlan) bool {
	return !plan.Rejected &&
		strings.TrimSpace(plan.Direction) != "" &&
		!strings.EqualFold(plan.Direction, "neutral") &&
		plan.EntryMin > 0 &&
		plan.EntryMax > 0 &&
		plan.StopLoss > 0 &&
		plan.RiskRewardRatio >= 1.5
}

func decisionSupportAllowsLiveBuySell(report DecisionSupportReport) bool {
	if strings.TrimSpace(report.Retail.Status) != "" {
		return report.Retail.Actionable && statusPass(report.Retail.Status)
	}
	for _, gate := range report.ActionGates {
		if gate.Name == "daily_technical_signal_gate" && statusPass(gate.Status) {
			return true
		}
	}
	return false
}

func downgradedNextSessionDirection(current string, closeChangePct float64) string {
	lower := strings.ToLower(strings.TrimSpace(current))
	switch {
	case closeChangePct > 0.15:
		if lower == "düşüş" {
			return "yatay-pozitif"
		}
		return "hafif pozitif"
	case closeChangePct < -0.15:
		if lower == "yükseliş" {
			return "yatay-negatif"
		}
		return "hafif negatif"
	default:
		return "yatay"
	}
}

func downgradedNextSessionStrength(current string, issues []string) string {
	if len(issues) >= 5 {
		return "zayıf-orta"
	}
	if strings.EqualFold(strings.TrimSpace(current), "güçlü") {
		return "orta"
	}
	return "zayıf-orta"
}

type nextSessionFundamentalOverlay struct {
	openBps       float64
	closeBps      float64
	confidenceAdj float64
	reasons       []string
}

func (o *nextSessionFundamentalOverlay) add(openBps, closeBps, confidenceAdj float64, reason string) {
	o.openBps += openBps
	o.closeBps += closeBps
	o.confidenceAdj += confidenceAdj
	o.reasons = appendUniqueAnalysisString(o.reasons, reason)
}

func (o *nextSessionFundamentalOverlay) cap() {
	o.openBps = mathutil.Clamp(o.openBps, -60, 60)
	o.closeBps = mathutil.Clamp(o.closeBps, -90, 90)
	o.confidenceAdj = mathutil.Clamp(o.confidenceAdj, -12, 8)
}

func nextSessionFundamentalAdjustment(pro professional.Report, lastClose float64) nextSessionFundamentalOverlay {
	overlay := nextSessionFundamentalOverlay{}
	applyValueInvestingForecastOverlay(&overlay, pro, lastClose)
	applyBuffettChecklistForecastOverlay(&overlay, pro)
	applyFinancialQualityForecastOverlay(&overlay, pro)
	applyKAPPDFForecastOverlay(&overlay, pro)
	applyNewsSentimentForecastOverlay(&overlay, pro)
	applyVAPForecastOverlay(&overlay, pro)
	applyMacroForecastOverlay(&overlay, pro)
	overlay.cap()
	return overlay
}

func applyValueInvestingForecastOverlay(overlay *nextSessionFundamentalOverlay, pro professional.Report, lastClose float64) {
	v := pro.ValueInvesting
	if v.MarginOfSafety.Computed {
		margin := mathutil.Clamp(v.MarginOfSafety.BasePct, -35, 35)
		closeBps := margin * 0.55
		openBps := closeBps * 0.45
		qualityBps, qualityConf := valueQualityForecastAdjustment(v.QualityScore)
		closeBps += qualityBps
		openBps += qualityBps * 0.45
		confidenceAdj := mathutil.Clamp((v.Confidence-50)/18, -3, 3) + qualityConf
		overlay.add(openBps, closeBps, confidenceAdj, fmt.Sprintf(
			"Temel bağlam: değer yatırım marjı %.1f%%, kalite %.0f/100, karar %s.",
			v.MarginOfSafety.BasePct,
			v.QualityScore,
			emptyAnalysisLabel(v.DecisionLabel, v.Decision),
		))
		return
	}
	if pro.Valuation.FairValue.Base > 0 && lastClose > 0 {
		upside := mathutil.Clamp((pro.Valuation.FairValue.Base/lastClose-1)*100, -35, 35)
		closeBps := upside * 0.35
		overlay.add(closeBps*0.40, closeBps, mathutil.Clamp((pro.Valuation.FairValue.Confidence-45)/25, -2, 2), fmt.Sprintf(
			"Temel bağlam: baz adil değer %.2f, son fiyata göre %.1f%% fark.",
			pro.Valuation.FairValue.Base,
			(pro.Valuation.FairValue.Base/lastClose-1)*100,
		))
		return
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok {
		closeBps := mathutil.Clamp(baseReturn, -35, 35) * 0.25
		overlay.add(closeBps*0.35, closeBps, 0.5, fmt.Sprintf("Temel bağlam: baz senaryo beklenen getirisi %.1f%%.", baseReturn))
	}
}

func valueQualityForecastAdjustment(score float64) (bps float64, confidenceAdj float64) {
	switch {
	case score >= 75:
		return 9, 1.5
	case score >= 65:
		return 5, 1
	case score > 0 && score < 45:
		return -9, -2
	case score > 0 && score < 55:
		return -5, -1
	default:
		return 0, 0
	}
}

func applyBuffettChecklistForecastOverlay(overlay *nextSessionFundamentalOverlay, pro professional.Report) {
	checklist := pro.ValueInvesting.BuffettChecklist
	if !checklist.Computed {
		return
	}
	switch strings.ToLower(strings.TrimSpace(checklist.Status)) {
	case "pass":
		overlay.add(0, 5, 1, fmt.Sprintf(
			"Temel bağlam: Buffett/değer yatırımı filtresi geçti (%.0f/100); uzun vadeli finansal kalite kapanış yön senaryosuna sınırlı pozitif ağırlık verdi.",
			checklist.Score,
		))
	case "fail":
		overlay.add(0, -7, -1.5, fmt.Sprintf(
			"Temel bağlam: Buffett/değer yatırımı filtresi başarısız (%.0f/100); değerleme/kalite zayıflığı kapanış yön senaryosunu aşağı çekti.",
			checklist.Score,
		))
	case "limited":
		overlay.add(0, -3, -0.5, fmt.Sprintf(
			"Temel bağlam: Buffett/değer yatırımı filtresi sınırlı (%.0f/100); eksik/şartlı finansal kanıt kapanış güvenini düşürdü.",
			checklist.Score,
		))
	default:
		overlay.add(0, 0, -0.5, fmt.Sprintf(
			"Temel bağlam: Buffett/değer yatırımı filtresi %s (%.0f/100); sonuç yön etkisi yerine güven denetimine alındı.",
			emptyAnalysisLabel(checklist.Status, "belirsiz"),
			checklist.Score,
		))
	}
}

func applyFinancialQualityForecastOverlay(overlay *nextSessionFundamentalOverlay, pro professional.Report) {
	if pro.DataQuality > 0 {
		switch {
		case pro.DataQuality < 55:
			overlay.add(-4, -9, -4, fmt.Sprintf("Temel bağlam: finansal veri kalitesi düşük (%.0f/100), tahmin güveni aşağı çekildi.", pro.DataQuality))
		case pro.DataQuality >= 80:
			overlay.add(0, 0, 2, fmt.Sprintf("Temel bağlam: finansal veri kalitesi güçlü (%.0f/100), kanıt güveni destekliyor.", pro.DataQuality))
		}
	}
	for _, flag := range pro.Valuation.Flags {
		if flag == "income_quality_gap_fcf_negative_net_income_positive" {
			overlay.add(0, -5, -2, "Temel bağlam: FCF negatif ancak net kar pozitif — muhasebe karının nakde dönüşümü gerçekleşmemiş; gelir kalitesi uyarısı kapanış yön güvenini düşürüyor.")
			break
		}
	}
	redFlags := pro.InvestmentResearch.FinancialQuality.RedFlags
	if len(redFlags) == 0 {
		return
	}
	penalty := math.Min(float64(len(redFlags)), 3)
	overlay.add(-3*penalty, -7*penalty, -1.5*penalty, fmt.Sprintf(
		"Temel bağlam: finansal kalite katmanında %d kırmızı bayrak var (%s).",
		len(redFlags),
		redFlags[0],
	))
}

func applyKAPPDFForecastOverlay(overlay *nextSessionFundamentalOverlay, pro professional.Report) {
	kap := pro.KAPPDFIngest
	if !kap.Computed || kap.TotalDocuments <= 0 {
		return
	}
	usableRatio := mathutil.SafeDiv(float64(kap.AnalysisUsableCount), float64(kap.TotalDocuments))
	confidenceAdj := mathutil.Clamp(usableRatio*4, 0, 4)
	if kap.AnalysisUsableCount == 0 {
		confidenceAdj = -4
	}
	if kap.LowQualityCount+kap.RejectedCount > kap.AnalysisUsableCount {
		confidenceAdj -= 2
	}
	overlay.add(0, 0, confidenceAdj, fmt.Sprintf(
		"Temel bağlam: KAP/PDF kanıt kapsamı %d/%d analize uygun belge ile tahmin güvenine dahil edildi.",
		kap.AnalysisUsableCount,
		kap.TotalDocuments,
	))
}

func applyNewsSentimentForecastOverlay(overlay *nextSessionFundamentalOverlay, pro professional.Report) {
	news := pro.NewsSentiment
	if !news.Computed || news.RecentItemCount == 0 {
		return
	}
	closeBps := mathutil.Clamp(news.Score*0.18, -18, 18)
	openBps := closeBps * 0.35
	confidenceAdj := mathutil.Clamp(float64(news.RecentItemCount)/8, 0, 2)
	if math.Abs(news.Score) < 15 {
		confidenceAdj *= 0.5
	}
	if news.NegativeCount > news.PositiveCount && closeBps > 0 {
		closeBps *= 0.35
		openBps = closeBps * 0.35
	}
	overlay.add(openBps, closeBps, confidenceAdj, fmt.Sprintf(
		"KAP/haber duyarlılığı: skor %.0f/100, pozitif=%d negatif=%d nötr=%d; kısa vadeli kapanış yön senaryosuna sınırlı ağırlıkla eklendi.",
		news.Score,
		news.PositiveCount,
		news.NegativeCount,
		news.NeutralCount,
	))
}

func applyVAPForecastOverlay(overlay *nextSessionFundamentalOverlay, pro professional.Report) {
	freeFloat := pro.VAPFreeFloat
	if freeFloat.Computed {
		openBps := 0.0
		closeBps := 0.0
		confidenceAdj := 1.0
		supply := strings.ToLower(freeFloat.SupplySignal)
		switch {
		case strings.Contains(supply, "azalan"):
			openBps += 3
			closeBps += 7
		case strings.Contains(supply, "artan"):
			openBps -= 3
			closeBps -= 7
		}
		switch {
		case freeFloat.RatioChange20DPP >= 0.50:
			openBps -= 2
			closeBps -= 5
		case freeFloat.RatioChange20DPP <= -0.50:
			openBps += 2
			closeBps += 5
		}
		risk := strings.ToLower(freeFloat.LiquidityRisk)
		switch {
		case strings.Contains(risk, "yüksek"):
			openBps -= 4
			closeBps -= 8
			confidenceAdj -= 2
		case strings.Contains(risk, "düşük"):
			openBps += 2
			closeBps += 3
			confidenceAdj += 1
		}
		overlay.add(openBps, closeBps, confidenceAdj, fmt.Sprintf(
			"Temel bağlam: VAP fiili dolaşım %.2f%%, 20 gözlem değişimi %+.2f puan, arz sinyali %s.",
			freeFloat.FreeFloatRatioPct,
			freeFloat.RatioChange20DPP,
			emptyAnalysisLabel(freeFloat.SupplySignal, "bilinmiyor"),
		))
	}

	portfolio := pro.VAPIndexPortfolio
	if !portfolio.Computed {
		return
	}
	closeBps := mathutil.Clamp(portfolio.RelativeMomentum*2.0, -8, 8) + mathutil.Clamp(portfolio.Change1MPct*0.8, -8, 8)
	signal := strings.ToLower(portfolio.Signal)
	if strings.Contains(signal, "göreli direnç") || strings.Contains(signal, "direnç var") {
		closeBps += 4
	}
	if strings.Contains(signal, "daral") {
		closeBps -= 4
	}
	if strings.Contains(signal, "geniş") || strings.Contains(signal, "art") {
		closeBps += 4
	}
	overlay.add(closeBps*0.35, closeBps, 1, fmt.Sprintf(
		"Temel bağlam: VAP %s portföyü aylık %+.2f%%, BIST100'e göre %+.2f puan.",
		emptyAnalysisLabel(portfolio.SelectedIndex, "endeks"),
		portfolio.Change1MPct,
		portfolio.RelativeMomentum,
	))
}

func applyMacroForecastOverlay(overlay *nextSessionFundamentalOverlay, pro professional.Report) {
	impact := pro.TCMBEVDSContext.ForecastImpact
	if !impact.Computed {
		return
	}
	if strings.EqualFold(impact.DecisionUse, "audit_only") {
		overlay.add(0, 0, -2, fmt.Sprintf("Makro bağlam: TCMB/EVDS etki katmanı denetim modunda; yön etkisi fiyat tahminine taşınmadı (%s).", emptyAnalysisLabel(impact.Label, impact.Direction)))
		return
	}
	closeBps := mathutil.Clamp(impact.ScoreAdjustment*1.8, -14, 14)
	if closeBps == 0 && impact.PressureScore != 0 {
		closeBps = mathutil.Clamp(impact.PressureScore*0.12, -14, 14)
	}
	if strings.EqualFold(impact.Direction, "negative") {
		closeBps = math.Min(closeBps, -4)
	} else if strings.EqualFold(impact.Direction, "positive") {
		closeBps = math.Max(closeBps, 4)
	}
	if strings.EqualFold(impact.Severity, "high") {
		closeBps *= 1.25
	}
	confidenceAdj := mathutil.Clamp((impact.Confidence-50)/25, -3, 3)
	overlay.add(closeBps*0.40, closeBps, confidenceAdj, fmt.Sprintf(
		"Makro bağlam: TCMB/EVDS %s, etki %s/%s, güven %.0f/100.",
		emptyAnalysisLabel(impact.Label, impact.Direction),
		emptyAnalysisLabel(impact.Direction, "nötr"),
		emptyAnalysisLabel(impact.Severity, "belirsiz"),
		impact.Confidence,
	))
}

func nextSessionForecastHasFundamentalContext(forecast NextSessionForecast) bool {
	if strings.Contains(strings.ToLower(forecast.Model), "fundamental") {
		return true
	}
	for _, reason := range forecast.BiasReasons {
		lower := strings.ToLower(strings.TrimSpace(reason))
		if strings.HasPrefix(lower, "temel bağlam:") || strings.HasPrefix(lower, "makro bağlam:") {
			return true
		}
	}
	return false
}

func nextSessionProfessionalContextAvailable(pro professional.Report) bool {
	return hasProfessionalReport(pro) ||
		pro.ValueInvesting.Computed ||
		pro.ValueInvesting.MarginOfSafety.Computed ||
		pro.KAPPDFIngest.Computed ||
		pro.NewsSentiment.Computed ||
		pro.InvestmentResearch.Computed ||
		pro.TCMBEVDSContext.ForecastImpact.Computed ||
		pro.VAPFreeFloat.Computed ||
		pro.VAPIndexPortfolio.Computed ||
		pro.Coverage.Score > 0
}

func emptyAnalysisLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return "nötr"
}

func weightedNextSessionReturns(candles []ohlcv.Candle, limit int) (gapMean, intradayMean float64, samples int) {
	if len(candles) < 2 || limit <= 0 {
		return 0, 0, 0
	}
	start := len(candles) - limit
	if start < 1 {
		start = 1
	}
	weightedGap := 0.0
	weightedIntraday := 0.0
	totalWeight := 0.0
	for i := start; i < len(candles); i++ {
		previousClose := candles[i-1].EffectiveClose()
		open := candles[i].EffectiveOpen()
		closePrice := candles[i].EffectiveClose()
		if previousClose <= 0 || open <= 0 || closePrice <= 0 {
			continue
		}
		gap := mathutil.Clamp(open/previousClose-1, -0.10, 0.10)
		intraday := mathutil.Clamp(closePrice/open-1, -0.10, 0.10)
		weight := float64(samples + 1)
		weightedGap += gap * weight
		weightedIntraday += intraday * weight
		totalWeight += weight
		samples++
	}
	if totalWeight == 0 {
		return 0, 0, 0
	}
	return weightedGap / totalWeight, weightedIntraday / totalWeight, samples
}

type nextSessionOpenModelResult struct {
	Return    float64
	Reasons   []string
	Warnings  []string
	Uncertain bool
}

type nextSessionCloseModelResult struct {
	IntradayReturn float64
	Reasons        []string
	Warnings       []string
	Uncertain      bool
}

func nextSessionOpenForecastModel(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, lastClose, atr, gapMean, technicalDrift float64) nextSessionOpenModelResult {
	out := nextSessionOpenModelResult{}
	if len(candles) < 20 || lastClose <= 0 {
		out.Warnings = append(out.Warnings, "insufficient_data_for_open_model")
		out.Uncertain = true
		return out
	}
	atrPct := safeRatio(atr, lastClose)
	ret1 := candleReturn(candles, 1)
	ret3 := candleReturn(candles, 3)
	ret5 := candleReturn(candles, 5)
	ret10 := candleReturn(candles, 10)
	volumeImpulse := candleVolumeImpulse(candles, 20)
	indicatorScore := nextSessionIndicatorScore(snapshot, lastClose)
	openReturn := 0.58*gapMean +
		0.12*ret1 +
		0.10*ret3 +
		0.07*ret5 +
		0.08*indicatorScore*atrPct +
		0.05*mathutil.Clamp(volumeImpulse, -2, 2)*atrPct +
		0.12*technicalDrift
	openReturn = mathutil.Clamp(openReturn, -1.35*atrPct, 1.35*atrPct)
	out.Return = openReturn
	out.Reasons = append(out.Reasons,
		fmt.Sprintf("Açılış modeli: gap_ewma=%+.2f%%, getiri1/3/5/10=%+.2f/%+.2f/%+.2f/%+.2f%%, hacim_impulse=%.2f, teknik_skor=%.2f.",
			100*gapMean, 100*ret1, 100*ret3, 100*ret5, 100*ret10, volumeImpulse, indicatorScore),
	)
	out.Warnings = append(out.Warnings,
		"open_model_uses_separate_gap_overnight_features",
		"index_global_futures_opening_auction_inputs_unavailable_zero_weight",
	)
	if atrPct >= 0.035 || math.Abs(ret1) >= 1.25*atrPct || math.Abs(openReturn) >= 0.95*atrPct {
		out.Uncertain = true
		out.Warnings = append(out.Warnings, "open_model_uncertain_large_gap_or_high_volatility")
	}
	return out
}

func nextSessionCloseForecastModel(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, lastClose, atr, intradayMean, predictedGapReturn, technicalDrift float64) nextSessionCloseModelResult {
	out := nextSessionCloseModelResult{}
	if len(candles) < 20 || lastClose <= 0 {
		out.Warnings = append(out.Warnings, "insufficient_data_for_close_model")
		out.Uncertain = true
		return out
	}
	atrPct := safeRatio(atr, lastClose)
	ret1 := candleReturn(candles, 1)
	ret3 := candleReturn(candles, 3)
	ret5 := candleReturn(candles, 5)
	ret10 := candleReturn(candles, 10)
	volumeImpulse := candleVolumeImpulse(candles, 20)
	indicatorScore := nextSessionIndicatorScore(snapshot, lastClose)
	bollingerScore := nextSessionBollingerScore(snapshot, lastClose)
	gapContinuation := mathutil.Clamp(predictedGapReturn/(math.Max(atrPct, 0.0001)), -1, 1) * atrPct
	intradayReturn := 0.34*intradayMean +
		0.12*ret1 +
		0.10*ret3 +
		0.06*ret5 +
		0.04*ret10 +
		0.18*indicatorScore*atrPct +
		0.08*mathutil.Clamp(volumeImpulse, -2, 2)*atrPct +
		0.08*bollingerScore*atrPct +
		0.08*gapContinuation +
		0.12*technicalDrift
	intradayReturn = mathutil.Clamp(intradayReturn, -1.65*atrPct, 1.65*atrPct)
	out.IntradayReturn = intradayReturn
	out.Reasons = append(out.Reasons,
		fmt.Sprintf("Kapanış modeli: intraday_ewma=%+.2f%%, tahmini_gap=%+.2f%%, RSI=%.1f, MACD_hist=%.4f, Bollinger_skor=%.2f, ATR=%.2f%%.",
			100*intradayMean, 100*predictedGapReturn, snapshot.RSI14, snapshot.MACDHistogram, bollingerScore, 100*atrPct),
	)
	out.Warnings = append(out.Warnings,
		"close_model_uses_separate_intraday_momentum_volume_volatility_support_resistance_features",
		"index_sector_flow_freefloat_inputs_missing_use_zero_weight_until_available",
	)
	if atrPct >= 0.035 || math.Abs(predictedGapReturn) >= 1.10*atrPct || math.Abs(intradayReturn) >= 1.25*atrPct {
		out.Uncertain = true
		out.Warnings = append(out.Warnings, "close_model_uncertain_large_gap_or_high_volatility")
	}
	return out
}

func candleReturn(candles []ohlcv.Candle, lookback int) float64 {
	if lookback <= 0 || len(candles) <= lookback {
		return 0
	}
	current := candles[len(candles)-1].EffectiveClose()
	base := candles[len(candles)-1-lookback].EffectiveClose()
	if current <= 0 || base <= 0 {
		return 0
	}
	return mathutil.Clamp(current/base-1, -0.25, 0.25)
}

func candleVolumeImpulse(candles []ohlcv.Candle, period int) float64 {
	if period <= 0 || len(candles) < 2 {
		return 0
	}
	lastVolume := candles[len(candles)-1].EffectiveVolume()
	if lastVolume <= 0 {
		return 0
	}
	start := len(candles) - 1 - period
	if start < 0 {
		start = 0
	}
	total := 0.0
	count := 0
	for _, candle := range candles[start : len(candles)-1] {
		volume := candle.EffectiveVolume()
		if volume > 0 {
			total += volume
			count++
		}
	}
	if count == 0 || total <= 0 {
		return 0
	}
	return math.Log(lastVolume / (total / float64(count)))
}

func nextSessionIndicatorScore(snapshot ohlcv.IndicatorSnapshot, lastClose float64) float64 {
	if lastClose <= 0 {
		return 0
	}
	bull, bear := indicatorConfluence(snapshot, lastClose)
	return nextSessionConfluenceScore(bull, bear)
}

func nextSessionBollingerScore(snapshot ohlcv.IndicatorSnapshot, lastClose float64) float64 {
	upper := snapshot.BollingerUpper
	lower := snapshot.BollingerLower
	if lastClose <= 0 || upper <= lower || upper <= 0 || lower <= 0 {
		return 0
	}
	position := (lastClose - lower) / (upper - lower)
	switch {
	case position >= 0.85:
		return -0.70
	case position <= 0.15:
		return 0.70
	default:
		return mathutil.Clamp((0.50-position)*0.8, -0.35, 0.35)
	}
}

func safeRatio(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return num / den
}

func nextSessionVolatilityRegime(snapshot ohlcv.IndicatorSnapshot, lastClose float64) string {
	atrPct := safeRatio(snapshot.ATR14, lastClose) * 100
	bbWidth := snapshot.AdditionalIndicators["Bollinger Band Width"]
	switch {
	case atrPct >= 5.0 || bbWidth >= 15.0:
		return "extreme"
	case atrPct >= 3.0 || bbWidth >= 10.0:
		return "high"
	case atrPct <= 1.0 && (bbWidth == 0 || bbWidth <= 4.0):
		return "low"
	default:
		return "normal"
	}
}

func nextForecastSessionDate(last time.Time, assetType string) string {
	if last.IsZero() {
		return ""
	}
	next := last.AddDate(0, 0, 1)
	if !ohlcv.IsCryptoAssetType(assetType) {
		next = forecastpolicy.NextWeekdaySession(last)
	}
	return next.Format("2006-01-02")
}

const (
	bistEquityPriceStepRule                    = "bist_pay_market_equity_price_step_v2026"
	forecastNearestTickRounding                = "nearest_tick"
	forecastActualNotObserved                  = "actual_session_not_observed"
	forecastActualObserved                     = "actual_session_observed"
	forecastRollingBacktestOnly                = "rolling_backtest_actual_session_not_observed"
	forecastRollingBacktestWindow              = 60
	nextSessionForecastDecisionBacktestWindow  = 20
	nextSessionDecisionMaxCloseMAPEPct         = 2.0
	nextSessionDecisionMinDirectionAccuracyPct = 55.0
	nextSessionPointForecastMinBacktestSamples = 30
	nextSessionPointForecastMinDirectionHitPct = 55.0
	nextSessionPointForecastMaxCloseMAEPct     = 1.25
	nextSessionPointForecastMinConfidence      = 55.0
	nextSessionDirectionTolerancePct           = forecastpolicy.NextSessionDirectionTolerancePct
)

func NextSessionDirectionTolerancePct() float64 {
	return forecastpolicy.NextSessionDirectionTolerancePct
}

func NextSessionDirectionToleranceReturn() float64 {
	return forecastpolicy.NextSessionDirectionToleranceReturn()
}

func NextSessionDirectionFromReturn(ret float64) string {
	return forecastpolicy.DirectionFromReturn(ret)
}

func NextSessionForecastTargetSession(asOf time.Time) time.Time {
	return forecastpolicy.NextWeekdaySession(asOf)
}

func applyTradablePriceStepToNextSessionForecast(f NextSessionForecast, assetType, symbol string) NextSessionForecast {
	if !f.Computed {
		return f
	}
	rawOpen := roundForecastPrice(forecastRawPredictedOpen(f))
	rawClose := roundForecastPrice(forecastRawPredictedClose(f))
	if rawOpen <= 0 || rawClose <= 0 {
		return f
	}
	f.RawPredictedOpen = rawOpen
	f.RawPredictedClose = rawClose
	rawLow := roundForecastPrice(forecastRawExpectedLow(f))
	rawHigh := roundForecastPrice(forecastRawExpectedHigh(f))
	if rawLow > 0 {
		f.RawExpectedLow = rawLow
	}
	if rawHigh > 0 {
		f.RawExpectedHigh = rawHigh
	}
	if !nextSessionForecastUsesBISTPriceStep(assetType, symbol) {
		f.PredictedOpen = rawOpen
		f.PredictedClose = rawClose
		f.ExpectedLow = rawLow
		f.ExpectedHigh = rawHigh
		if f.LastClose > 0 {
			f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
			f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
		}
		f = normalizeNextSessionForecastScenario(f, assetType, symbol)
		f = applyNextSessionForecastDirectionFields(f)
		return f
	}

	openTick := bistEquityTickSize(rawOpen)
	closeTick := bistEquityTickSize(rawClose)
	lowTick := bistEquityTickSize(rawLow)
	highTick := bistEquityTickSize(rawHigh)
	tradableOpen := roundForecastPriceToTick(rawOpen, openTick)
	tradableClose := roundForecastPriceToTick(rawClose, closeTick)
	f.TradablePredictedOpen = tradableOpen
	f.TradablePredictedClose = tradableClose
	f.PredictedOpen = tradableOpen
	f.PredictedClose = tradableClose
	if rawLow > 0 {
		f.TradableExpectedLow = roundForecastPriceToTick(rawLow, lowTick)
		f.ExpectedLow = f.TradableExpectedLow
	}
	if rawHigh > 0 {
		f.TradableExpectedHigh = roundForecastPriceToTick(rawHigh, highTick)
		f.ExpectedHigh = f.TradableExpectedHigh
	}
	f.TickSize = openTick
	f.RoundingMethod = forecastNearestTickRounding
	f.PriceStepRule = bistEquityPriceStepRule
	if f.LastClose > 0 {
		f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
		f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
	}
	f = normalizeNextSessionForecastScenario(f, assetType, symbol)
	f = applyNextSessionForecastDirectionFields(f)
	f.Warnings = appendUniqueAnalysisString(f.Warnings, "bist_price_step_applied_to_tradable_forecast")
	if openTick != closeTick || (rawLow > 0 && openTick != lowTick) || (rawHigh > 0 && openTick != highTick) {
		f.Warnings = appendUniqueAnalysisString(f.Warnings, "forecast_prices_cross_multiple_bist_tick_bands")
	}
	return f
}

func nextSessionForecastUsesBISTPriceStep(assetType, symbol string) bool {
	if ohlcv.IsCryptoAssetType(assetType) || ohlcv.IsCommodityAssetType(assetType) {
		return false
	}
	if strings.TrimSpace(symbol) != "" && ohlcv.IsCommoditySymbol(symbol) {
		return false
	}
	return ohlcv.NormalizeAssetType(assetType) == ohlcv.AssetTypeEquity
}

func attachNextSessionForecastScenario(f NextSessionForecast, candles []ohlcv.Candle) NextSessionForecast {
	if !f.Computed || f.LastClose <= 0 {
		return f
	}
	openCenter := firstPositiveFloat(f.RawPredictedOpen, f.PredictedOpen)
	closeCenter := firstPositiveFloat(f.RawPredictedClose, f.PredictedClose)
	if openCenter <= 0 || closeCenter <= 0 {
		return f
	}
	openReturns, closeReturns := nextSessionForecastReturnSamples(candles, forecastRollingBacktestWindow)
	f.ForecastDistributionSamples = min(len(openReturns), len(closeReturns))
	f.OpenP10, f.OpenP50, f.OpenP90 = nextSessionShiftedQuantilePrices(f.LastClose, openCenter, openReturns, f.RawExpectedLow, f.RawExpectedHigh)
	f.CloseP10, f.CloseP50, f.CloseP90 = nextSessionShiftedQuantilePrices(f.LastClose, closeCenter, closeReturns, f.RawExpectedLow, f.RawExpectedHigh)
	f.UpsideProbabilityPct, f.FlatProbabilityPct, f.DownsideProbabilityPct = nextSessionDirectionProbabilities(f.LastClose, closeCenter, closeReturns)
	f.InvalidationLevel, f.InvalidationReason = nextSessionInvalidationLevel(f)
	return f
}

func nextSessionForecastReturnSamples(candles []ohlcv.Candle, limit int) ([]float64, []float64) {
	if len(candles) < 2 || limit <= 0 {
		return nil, nil
	}
	start := len(candles) - limit
	if start < 1 {
		start = 1
	}
	openReturns := []float64{}
	closeReturns := []float64{}
	for i := start; i < len(candles); i++ {
		prevClose := candles[i-1].EffectiveClose()
		openPrice := candles[i].EffectiveOpen()
		closePrice := candles[i].EffectiveClose()
		if prevClose <= 0 || openPrice <= 0 || closePrice <= 0 {
			continue
		}
		openReturns = append(openReturns, openPrice/prevClose-1)
		closeReturns = append(closeReturns, closePrice/prevClose-1)
	}
	return openReturns, closeReturns
}

func nextSessionShiftedQuantilePrices(lastClose, center float64, returns []float64, fallbackLow, fallbackHigh float64) (float64, float64, float64) {
	if lastClose <= 0 || center <= 0 {
		return 0, 0, 0
	}
	if len(returns) >= 5 {
		sortedReturns := append([]float64{}, returns...)
		sort.Float64s(sortedReturns)
		p10 := lastClose * (1 + percentileSorted(sortedReturns, 0.10))
		p50 := lastClose * (1 + percentileSorted(sortedReturns, 0.50))
		p90 := lastClose * (1 + percentileSorted(sortedReturns, 0.90))
		shift := center - p50
		return orderedForecastQuantiles(p10+shift, center, p90+shift)
	}
	span := lastClose * 0.015
	if fallbackLow > 0 && fallbackHigh > fallbackLow {
		span = math.Max(span, (fallbackHigh-fallbackLow)/2)
	}
	return orderedForecastQuantiles(center-span, center, center+span)
}

func percentileSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	p = mathutil.Clamp(p, 0, 1)
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	weight := pos - float64(lo)
	return sorted[lo]*(1-weight) + sorted[hi]*weight
}

func orderedForecastQuantiles(p10, p50, p90 float64) (float64, float64, float64) {
	p10 = math.Max(p10, 0.01)
	p50 = math.Max(p50, 0.01)
	p90 = math.Max(p90, 0.01)
	if p10 > p50 {
		p10 = p50
	}
	if p90 < p50 {
		p90 = p50
	}
	return roundForecastPrice(p10), roundForecastPrice(p50), roundForecastPrice(p90)
}

func nextSessionDirectionProbabilities(lastClose, center float64, closeReturns []float64) (float64, float64, float64) {
	if lastClose <= 0 || center <= 0 {
		return 0, 0, 0
	}
	neutralTolerance := forecastpolicy.NextSessionDirectionToleranceReturn()
	if len(closeReturns) == 0 {
		change := center/lastClose - 1
		switch {
		case change > neutralTolerance:
			return 55, 25, 20
		case change < -neutralTolerance:
			return 20, 25, 55
		default:
			return 25, 50, 25
		}
	}
	sortedReturns := append([]float64{}, closeReturns...)
	sort.Float64s(sortedReturns)
	shift := center/lastClose - 1 - percentileSorted(sortedReturns, 0.50)
	up, flat, down := 0, 0, 0
	for _, ret := range closeReturns {
		shifted := ret + shift
		switch {
		case shifted > neutralTolerance:
			up++
		case shifted < -neutralTolerance:
			down++
		default:
			flat++
		}
	}
	total := float64(len(closeReturns))
	return roundForecastMetric(100 * float64(up) / total),
		roundForecastMetric(100 * float64(flat) / total),
		roundForecastMetric(100 * float64(down) / total)
}

func nextSessionInvalidationLevel(f NextSessionForecast) (float64, string) {
	if f.LastClose <= 0 {
		return 0, ""
	}
	direction := nextSessionForecastDirectionFromChange(f.CloseChangePct)
	if direction == "neutral" {
		direction = nextSessionForecastDirectionFromBias(f.DirectionBias)
	}
	switch direction {
	case "bullish":
		level := maxPositiveForecastPrice(f.PivotS1, f.ExpectedLow, f.CloseP10)
		if level <= 0 {
			level = f.LastClose * 0.98
		}
		return roundForecastPrice(level), "yukselis_senaryosu_destek_altinda_gecersiz"
	case "bearish":
		level := minPositiveForecastPrice(f.PivotR1, f.ExpectedHigh, f.CloseP90)
		if level <= 0 {
			level = f.LastClose * 1.02
		}
		return roundForecastPrice(level), "dusus_senaryosu_direnc_ustunde_gecersiz"
	default:
		return 0, ""
	}
}

func nextSessionForecastDirectionFromBias(bias string) string {
	text := strings.ToLower(strings.TrimSpace(bias))
	switch {
	case strings.Contains(text, "yüks") || strings.Contains(text, "pozitif"):
		return "bullish"
	case strings.Contains(text, "düş") || strings.Contains(text, "negatif"):
		return "bearish"
	default:
		return "neutral"
	}
}

func maxPositiveForecastPrice(values ...float64) float64 {
	out := 0.0
	for _, value := range values {
		if value > out {
			out = value
		}
	}
	return out
}

func minPositiveForecastPrice(values ...float64) float64 {
	out := 0.0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if out == 0 || value < out {
			out = value
		}
	}
	return out
}

func normalizeNextSessionForecastScenario(f NextSessionForecast, assetType, symbol string) NextSessionForecast {
	if !f.Computed {
		return f
	}
	roundPrice := func(value float64) float64 {
		if value <= 0 {
			return 0
		}
		if nextSessionForecastUsesBISTPriceStep(assetType, symbol) {
			return roundForecastPriceToTick(value, bistEquityTickSize(value))
		}
		return roundForecastPrice(value)
	}
	f.OpenP10 = roundPrice(f.OpenP10)
	f.OpenP90 = roundPrice(f.OpenP90)
	f.OpenP50 = roundPrice(firstPositiveFloat(f.PredictedOpen, f.OpenP50))
	if f.OpenP10 > 0 && f.OpenP50 > 0 && f.OpenP10 > f.OpenP50 {
		f.OpenP10 = f.OpenP50
	}
	if f.OpenP90 > 0 && f.OpenP50 > 0 && f.OpenP90 < f.OpenP50 {
		f.OpenP90 = f.OpenP50
	}
	f.CloseP10 = roundPrice(f.CloseP10)
	f.CloseP90 = roundPrice(f.CloseP90)
	f.CloseP50 = roundPrice(firstPositiveFloat(f.PredictedClose, f.CloseP50))
	if f.CloseP10 > 0 && f.CloseP50 > 0 && f.CloseP10 > f.CloseP50 {
		f.CloseP10 = f.CloseP50
	}
	if f.CloseP90 > 0 && f.CloseP50 > 0 && f.CloseP90 < f.CloseP50 {
		f.CloseP90 = f.CloseP50
	}
	f.UpsideProbabilityPct = roundForecastMetric(f.UpsideProbabilityPct)
	f.FlatProbabilityPct = roundForecastMetric(f.FlatProbabilityPct)
	f.DownsideProbabilityPct = roundForecastMetric(f.DownsideProbabilityPct)
	f.InvalidationLevel = roundPrice(f.InvalidationLevel)
	return f
}

func applyNextSessionForecastDirectionFields(f NextSessionForecast) NextSessionForecast {
	if !f.Computed || f.LastClose <= 0 {
		return f
	}
	if f.PredictedOpen > 0 && f.OpenChangePct == 0 {
		f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
	}
	if f.PredictedClose > 0 && f.CloseChangePct == 0 {
		f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
	}
	if f.PredictedOpen > 0 {
		f.PredictedOpenDirection = nextSessionForecastPriceDirectionTR(f.OpenChangePct)
	}
	if f.PredictedClose > 0 {
		f.PredictedCloseDirection = nextSessionForecastPriceDirectionTR(f.CloseChangePct)
	}
	f.DirectionTolerancePct = nextSessionDirectionTolerancePct
	return f
}

func nextSessionForecastPriceDirectionTR(changePct float64) string {
	return forecastpolicy.TurkishDirectionFromChangePct(changePct)
}

func forecastRawPredictedOpen(f NextSessionForecast) float64 {
	if f.RawPredictedOpen > 0 {
		return f.RawPredictedOpen
	}
	return f.PredictedOpen
}

func forecastRawPredictedClose(f NextSessionForecast) float64 {
	if f.RawPredictedClose > 0 {
		return f.RawPredictedClose
	}
	return f.PredictedClose
}

func forecastRawExpectedLow(f NextSessionForecast) float64 {
	if f.RawExpectedLow > 0 {
		return f.RawExpectedLow
	}
	return f.ExpectedLow
}

func forecastRawExpectedHigh(f NextSessionForecast) float64 {
	if f.RawExpectedHigh > 0 {
		return f.RawExpectedHigh
	}
	return f.ExpectedHigh
}

func bistEquityTickSize(price float64) float64 {
	if price < 20 {
		return 0.01
	}
	if price < 50 {
		return 0.02
	}
	if price < 100 {
		return 0.05
	}
	if price < 250 {
		return 0.10
	}
	if price < 500 {
		return 0.25
	}
	if price < 1000 {
		return 0.50
	}
	if price < 2500 {
		return 1.00
	}
	return 2.50
}

func BISTEquityTickSize(price float64) float64 {
	return bistEquityTickSize(price)
}

func roundForecastPriceToTick(value, tick float64) float64 {
	if value <= 0 || tick <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return roundForecastPrice(value)
	}
	return roundForecastPrice(math.Round(value/tick) * tick)
}

func RoundBISTEquityPriceToTick(value float64) float64 {
	return roundForecastPriceToTick(value, bistEquityTickSize(value))
}

// ApplyBISTEquityTickSizeToTechnicalLevels normalizes user-facing BIST equity
// levels before investor QA, decision support and report rendering build text.
func ApplyBISTEquityTickSizeToTechnicalLevels(result SymbolAnalysis) SymbolAnalysis {
	if !nextSessionForecastUsesBISTPriceStep(result.AssetType, result.Symbol) {
		return result
	}
	if result.Timeframes != nil {
		for key, tf := range result.Timeframes {
			tf.SupportLevels = roundBISTEquityLevels(tf.SupportLevels)
			tf.ResistanceLevels = roundBISTEquityLevels(tf.ResistanceLevels)
			tf.NearestSupport = roundBISTEquityLevelPtr(tf.NearestSupport)
			tf.NearestResistance = roundBISTEquityLevelPtr(tf.NearestResistance)
			tf.TradePlan = roundBISTEquityTradePlan(tf.TradePlan)
			tf.Patterns = roundBISTEquityPatternResults(tf.Patterns)
			tf.PatternCandidates = roundBISTEquityPatternResults(tf.PatternCandidates)
			if tf.NextSessionForecast.Computed {
				tf.NextSessionForecast = applyTradablePriceStepToNextSessionForecast(tf.NextSessionForecast, result.AssetType, result.Symbol)
			}
			result.Timeframes[key] = tf
		}
	}
	if result.NextSessionForecast.Computed {
		result.NextSessionForecast = applyTradablePriceStepToNextSessionForecast(result.NextSessionForecast, result.AssetType, result.Symbol)
	}
	if daily, ok := result.Timeframes["1D"]; ok && daily.NextSessionForecast.Computed {
		result.NextSessionForecast = daily.NextSessionForecast
	}
	return result
}

func roundBISTEquityLevels(levels []ohlcv.SupportResistanceLevel) []ohlcv.SupportResistanceLevel {
	if len(levels) == 0 {
		return levels
	}
	out := append([]ohlcv.SupportResistanceLevel(nil), levels...)
	for i := range out {
		if out[i].Price > 0 {
			out[i].Price = RoundBISTEquityPriceToTick(out[i].Price)
		}
	}
	return out
}

func roundBISTEquityLevelPtr(level *ohlcv.SupportResistanceLevel) *ohlcv.SupportResistanceLevel {
	if level == nil {
		return nil
	}
	out := *level
	if out.Price > 0 {
		out.Price = RoundBISTEquityPriceToTick(out.Price)
	}
	return &out
}

func roundBISTEquityTradePlan(plan ohlcv.TradePlan) ohlcv.TradePlan {
	plan.EntryMin = roundBISTEquityPositivePrice(plan.EntryMin)
	plan.EntryMax = roundBISTEquityPositivePrice(plan.EntryMax)
	plan.StopLoss = roundBISTEquityPositivePrice(plan.StopLoss)
	plan.TakeProfit1 = roundBISTEquityPositivePrice(plan.TakeProfit1)
	plan.TakeProfit2 = roundBISTEquityPositivePrice(plan.TakeProfit2)
	if plan.RiskRewardRatio > 0 {
		if rr := roundedTradePlanRiskReward(plan); rr > 0 {
			plan.RiskRewardRatio = roundForecastMetric(rr)
		}
	}
	return plan
}

func roundBISTEquityPatternResults(patterns []ohlcv.PatternResult) []ohlcv.PatternResult {
	if len(patterns) == 0 {
		return patterns
	}
	out := append([]ohlcv.PatternResult(nil), patterns...)
	for i := range out {
		out[i].EntryMin = roundBISTEquityPositivePrice(out[i].EntryMin)
		out[i].EntryMax = roundBISTEquityPositivePrice(out[i].EntryMax)
		out[i].StopLoss = roundBISTEquityPositivePrice(out[i].StopLoss)
		out[i].Target1 = roundBISTEquityPositivePrice(out[i].Target1)
		out[i].Target2 = roundBISTEquityPositivePrice(out[i].Target2)
		out[i].InvalidationLevel = roundBISTEquityPositivePrice(out[i].InvalidationLevel)
		if out[i].RiskRewardRatio > 0 {
			if rr := roundedPatternRiskReward(out[i]); rr > 0 {
				out[i].RiskRewardRatio = roundForecastMetric(rr)
			}
		}
	}
	return out
}

func roundBISTEquityPositivePrice(value float64) float64 {
	if value <= 0 {
		return value
	}
	return RoundBISTEquityPriceToTick(value)
}

func roundedTradePlanRiskReward(plan ohlcv.TradePlan) float64 {
	entry := roundedEntryReference(plan.EntryMin, plan.EntryMax)
	if entry <= 0 || plan.StopLoss <= 0 || plan.TakeProfit1 <= 0 {
		return 0
	}
	if strings.EqualFold(strings.TrimSpace(plan.Direction), "short") {
		risk := plan.StopLoss - entry
		reward := entry - plan.TakeProfit1
		if risk > 0 && reward > 0 {
			return reward / risk
		}
		return 0
	}
	risk := entry - plan.StopLoss
	reward := plan.TakeProfit1 - entry
	if risk > 0 && reward > 0 {
		return reward / risk
	}
	return 0
}

func roundedPatternRiskReward(pattern ohlcv.PatternResult) float64 {
	entry := roundedEntryReference(pattern.EntryMin, pattern.EntryMax)
	if entry <= 0 || pattern.StopLoss <= 0 || pattern.Target1 <= 0 {
		return 0
	}
	if strings.Contains(strings.ToLower(pattern.Direction), "bear") || strings.Contains(strings.ToLower(pattern.Direction), "düş") || strings.Contains(strings.ToLower(pattern.Direction), "dus") {
		risk := pattern.StopLoss - entry
		reward := entry - pattern.Target1
		if risk > 0 && reward > 0 {
			return reward / risk
		}
		return 0
	}
	risk := entry - pattern.StopLoss
	reward := pattern.Target1 - entry
	if risk > 0 && reward > 0 {
		return reward / risk
	}
	return 0
}

func roundedEntryReference(minValue, maxValue float64) float64 {
	switch {
	case minValue > 0 && maxValue > 0:
		return (minValue + maxValue) / 2
	case minValue > 0:
		return minValue
	case maxValue > 0:
		return maxValue
	default:
		return 0
	}
}

type nextSessionForecastBacktestMetrics struct {
	samples             int
	openMAE             float64
	openMAEPct          float64
	closeMAE            float64
	closeMAEPct         float64
	directionHitRatePct float64
	hit050Pct           float64
	hit100Pct           float64
	hit200Pct           float64
	rows                []NextSessionBacktestRow
}

func attachNextSessionForecastValidation(f NextSessionForecast, candles []ohlcv.Candle, assetType, source string) NextSessionForecast {
	if !f.Computed {
		return f
	}
	if strings.TrimSpace(source) == "" {
		source = "ohlcv_provider"
	}
	f.ValidationSource = source
	f.ValidationStatus = forecastActualNotObserved
	if actual, ok := findForecastActualCandle(candles, f.ForecastFor); ok {
		actualOpen := actual.EffectiveOpen()
		actualClose := actual.EffectiveClose()
		if actualOpen > 0 && actualClose > 0 {
			f.ValidationStatus = forecastActualObserved
			f = attachNextSessionForecastActual(f, actualOpen, actualClose, source, "")
		}
	}
	metrics := nextSessionForecastBacktest(candles, assetType, nextSessionForecastDecisionBacktestWindow)
	if metrics.samples > 0 {
		f.BacktestSamples = metrics.samples
		f.BacktestSource = source
		f.BacktestOpenMAEPct = roundForecastMetric(metrics.openMAEPct)
		f.BacktestCloseMAEPct = roundForecastMetric(metrics.closeMAEPct)
		f.BacktestDirectionHitRatePct = roundForecastMetric(metrics.directionHitRatePct)
		f.BacktestTable = metrics.rows
		f.BacktestMetrics = NextSessionBacktestMetrics{
			Samples:                  metrics.samples,
			OpenMAE:                  roundForecastPrice(metrics.openMAE),
			OpenMAPE:                 roundForecastMetric(metrics.openMAEPct),
			CloseMAE:                 roundForecastPrice(metrics.closeMAE),
			CloseMAPE:                roundForecastMetric(metrics.closeMAEPct),
			DirectionAccuracy:        roundForecastMetric(metrics.directionHitRatePct),
			HitRatioWithin050Pct:     roundForecastMetric(metrics.hit050Pct),
			HitRatioWithin100Pct:     roundForecastMetric(metrics.hit100Pct),
			HitRatioWithin200Pct:     roundForecastMetric(metrics.hit200Pct),
			TradeSignalAllowed:       metrics.closeMAEPct <= nextSessionDecisionMaxCloseMAPEPct && metrics.directionHitRatePct >= nextSessionDecisionMinDirectionAccuracyPct,
			DirectionModelUnreliable: metrics.directionHitRatePct < nextSessionDecisionMinDirectionAccuracyPct,
		}
		f.DirectionModelUnreliable = f.BacktestMetrics.DirectionModelUnreliable
		if f.BacktestMetrics.DirectionModelUnreliable {
			f.Warnings = appendUniqueAnalysisString(f.Warnings, "direction_model_unreliable")
		}
		if metrics.closeMAEPct > nextSessionDecisionMaxCloseMAPEPct {
			f.Status = "model_validation_failed"
			f.Quality = "not_decision_grade"
			f.Confidence = roundForecastMetric(math.Min(f.Confidence, 35))
			f.ConfidenceLabel = nextSessionConfidenceLabel(f.Confidence)
			f.Warnings = appendUniqueAnalysisString(f.Warnings, "close_mape_above_2pct_trade_signal_blocked")
		}
		if f.ValidationStatus == forecastActualNotObserved {
			f.ValidationStatus = forecastRollingBacktestOnly
		}
		f = calibrateNextSessionDecisionInterval(f, metrics, assetType, "")
	}
	f = syncNextSessionDecisionForecast(f, "")
	return f
}

func attachNextSessionForecastActual(f NextSessionForecast, actualOpen, actualClose float64, source, sourcePath string) NextSessionForecast {
	if !f.Computed || actualOpen <= 0 || actualClose <= 0 {
		return f
	}
	f.ActualAvailable = true
	f.ActualSource = strings.TrimSpace(source)
	f.ActualSourcePath = strings.TrimSpace(sourcePath)
	f.ActualOpen = roundForecastPrice(actualOpen)
	f.ActualClose = roundForecastPrice(actualClose)
	if f.PredictedOpen > 0 {
		f.ActualOpenErrorPct = roundForecastMetric(100 * (f.PredictedOpen/actualOpen - 1))
		f.OpenForecastErrorTL = roundForecastPrice(actualOpen - f.PredictedOpen)
		f.OpenAbsErrorPctVsActual = roundForecastMetric(100 * math.Abs(f.PredictedOpen-actualOpen) / actualOpen)
		if f.LastClose > 0 {
			f.OpenAbsErrorPctVsPreviousClose = roundForecastMetric(100 * math.Abs(f.PredictedOpen-actualOpen) / f.LastClose)
		}
		if hit, ok := nextSessionForecastDirectionHit(f.PredictedOpen, actualOpen, f.LastClose); ok {
			f.OpenDirectionHit = &hit
		}
	}
	if f.PredictedClose > 0 {
		f.ActualCloseErrorPct = roundForecastMetric(100 * (f.PredictedClose/actualClose - 1))
		f.CloseForecastErrorTL = roundForecastPrice(actualClose - f.PredictedClose)
		f.CloseAbsErrorPctVsActual = roundForecastMetric(100 * math.Abs(f.PredictedClose-actualClose) / actualClose)
		if f.LastClose > 0 {
			f.CloseAbsErrorPctVsPreviousClose = roundForecastMetric(100 * math.Abs(f.PredictedClose-actualClose) / f.LastClose)
		}
		if hit, ok := nextSessionForecastDirectionHit(f.PredictedClose, actualClose, f.LastClose); ok {
			f.CloseDirectionHit = &hit
		}
	}
	return f
}

func nextSessionForecastDirectionHit(predicted, actual, lastClose float64) (bool, bool) {
	return forecastpolicy.DirectionHit(predicted, actual, lastClose)
}

func nextSessionForecastPriceDirection(price, lastClose float64) (string, bool) {
	return forecastpolicy.PriceDirection(price, lastClose)
}

func nextSessionForecastDirectionForJSON(price, lastClose float64, allowUncertain bool) string {
	direction, ok := nextSessionForecastPriceDirection(price, lastClose)
	if !ok {
		if allowUncertain {
			return "uncertain"
		}
		return "flat"
	}
	return direction
}

func findForecastActualCandle(candles []ohlcv.Candle, forecastFor string) (ohlcv.Candle, bool) {
	forecastFor = strings.TrimSpace(forecastFor)
	if forecastFor == "" {
		return ohlcv.Candle{}, false
	}
	for _, candle := range candles {
		if candle.Time.Format("2006-01-02") == forecastFor {
			return candle, true
		}
	}
	return ohlcv.Candle{}, false
}

func nextSessionForecastBacktest(candles []ohlcv.Candle, assetType string, limit int) nextSessionForecastBacktestMetrics {
	if len(candles) < 30 || limit <= 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	start := len(candles) - limit
	if start < 25 {
		start = 25
	}
	totalOpenAbsErr := 0.0
	totalOpenAbsErrPct := 0.0
	totalCloseAbsErr := 0.0
	totalCloseAbsErrPct := 0.0
	directionHits := 0
	hit050 := 0
	hit100 := 0
	hit200 := 0
	samples := 0
	rows := []NextSessionBacktestRow{}
	for nextIdx := start; nextIdx < len(candles); nextIdx++ {
		prefix := candles[:nextIdx]
		if len(prefix) < 25 {
			continue
		}
		snapshot, err := indicators.Snapshot(prefix)
		if err != nil {
			continue
		}
		bias := trendBias(prefix, snapshot, nil)
		prediction := computeNextSessionForecastModel(prefix, snapshot, bias, assetType, false)
		actualOpen := candles[nextIdx].EffectiveOpen()
		actualClose := candles[nextIdx].EffectiveClose()
		if !prediction.Computed || prediction.PredictedOpen <= 0 || prediction.PredictedClose <= 0 || actualOpen <= 0 || actualClose <= 0 || prediction.LastClose <= 0 {
			continue
		}
		openAbs := math.Abs(prediction.PredictedOpen - actualOpen)
		closeAbs := math.Abs(prediction.PredictedClose - actualClose)
		openPct := 100 * openAbs / actualOpen
		closePct := 100 * closeAbs / actualClose
		totalOpenAbsErr += openAbs
		totalOpenAbsErrPct += openPct
		totalCloseAbsErr += closeAbs
		totalCloseAbsErrPct += closePct
		if closePct <= 0.50 {
			hit050++
		}
		if closePct <= 1.00 {
			hit100++
		}
		if closePct <= 2.00 {
			hit200++
		}
		predictedDirection := nextSessionForecastDirectionForJSON(prediction.PredictedClose, prediction.LastClose, true)
		actualDirection := nextSessionForecastDirectionForJSON(actualClose, prediction.LastClose, false)
		directionCorrect := predictedDirection != "uncertain" && predictedDirection == actualDirection
		if directionCorrect {
			directionHits++
		}
		rows = append(rows, NextSessionBacktestRow{
			Date:               candles[nextIdx].Time.Format("2006-01-02"),
			PreviousClose:      roundForecastPrice(prediction.LastClose),
			ActualOpen:         roundForecastPrice(actualOpen),
			PredictedOpen:      roundForecastPrice(prediction.PredictedOpen),
			OpenAbsError:       roundForecastPrice(openAbs),
			OpenPctError:       roundForecastMetric(openPct),
			ActualClose:        roundForecastPrice(actualClose),
			PredictedClose:     roundForecastPrice(prediction.PredictedClose),
			CloseAbsError:      roundForecastPrice(closeAbs),
			ClosePctError:      roundForecastMetric(closePct),
			ActualDirection:    actualDirection,
			PredictedDirection: predictedDirection,
			DirectionCorrect:   directionCorrect,
		})
		samples++
	}
	if samples == 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	return nextSessionForecastBacktestMetrics{
		samples:             samples,
		openMAE:             totalOpenAbsErr / float64(samples),
		openMAEPct:          totalOpenAbsErrPct / float64(samples),
		closeMAE:            totalCloseAbsErr / float64(samples),
		closeMAEPct:         totalCloseAbsErrPct / float64(samples),
		directionHitRatePct: 100 * float64(directionHits) / float64(samples),
		hit050Pct:           100 * float64(hit050) / float64(samples),
		hit100Pct:           100 * float64(hit100) / float64(samples),
		hit200Pct:           100 * float64(hit200) / float64(samples),
		rows:                rows,
	}
}

func nextSessionForecastBISTBulletinBacktest(records []datasource.DailyBulletinRecord, assetType string, limit int) nextSessionForecastBacktestMetrics {
	return nextSessionForecastBISTBulletinVariantBacktest(records, assetType, limit, true)
}

func nextSessionForecastBISTBulletinSelectedBacktest(records []datasource.DailyBulletinRecord, assetType string, limit int, useOverlay bool) nextSessionForecastBacktestMetrics {
	if !useOverlay {
		return nextSessionForecastBISTBulletinVariantBacktest(records, assetType, limit, false)
	}
	if len(records) < 30 || limit <= 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	start := len(records) - limit
	if start < 25 {
		start = 25
	}
	totalOpenAbsErr := 0.0
	totalOpenAbsErrPct := 0.0
	totalCloseAbsErr := 0.0
	totalCloseAbsErrPct := 0.0
	directionHits := 0
	hit050 := 0
	hit100 := 0
	hit200 := 0
	samples := 0
	rows := []NextSessionBacktestRow{}
	for nextIdx := start; nextIdx < len(records); nextIdx++ {
		actual := records[nextIdx]
		prefixRecords := append([]datasource.DailyBulletinRecord{}, records[:nextIdx]...)
		prefix := bistBulletinRecordsToCandles(prefixRecords)
		if len(prefix) < 25 || actual.Open <= 0 || actual.Close <= 0 {
			continue
		}
		snapshot, err := indicators.Snapshot(prefix)
		if err != nil {
			continue
		}
		bias := trendBias(prefix, snapshot, nil)
		prediction := computeNextSessionForecastModel(prefix, snapshot, bias, assetType, false)
		prediction = applyBISTBulletinRecordsToNextSessionForecast(prediction, prefixRecords, assetType, "", true)
		if !prediction.Computed || prediction.PredictedOpen <= 0 || prediction.PredictedClose <= 0 || prediction.LastClose <= 0 {
			continue
		}
		openAbs := math.Abs(prediction.PredictedOpen - actual.Open)
		closeAbs := math.Abs(prediction.PredictedClose - actual.Close)
		openPct := 100 * openAbs / actual.Open
		closePct := 100 * closeAbs / actual.Close
		totalOpenAbsErr += openAbs
		totalOpenAbsErrPct += openPct
		totalCloseAbsErr += closeAbs
		totalCloseAbsErrPct += closePct
		if closePct <= 0.50 {
			hit050++
		}
		if closePct <= 1.00 {
			hit100++
		}
		if closePct <= 2.00 {
			hit200++
		}
		predictedDirection := nextSessionForecastDirectionForJSON(prediction.PredictedClose, prediction.LastClose, true)
		actualDirection := nextSessionForecastDirectionForJSON(actual.Close, prediction.LastClose, false)
		directionCorrect := predictedDirection != "uncertain" && predictedDirection == actualDirection
		if directionCorrect {
			directionHits++
		}
		rows = append(rows, NextSessionBacktestRow{
			Date:               actual.TradingDate,
			PreviousClose:      roundForecastPrice(prediction.LastClose),
			ActualOpen:         roundForecastPrice(actual.Open),
			PredictedOpen:      roundForecastPrice(prediction.PredictedOpen),
			OpenAbsError:       roundForecastPrice(openAbs),
			OpenPctError:       roundForecastMetric(openPct),
			ActualClose:        roundForecastPrice(actual.Close),
			PredictedClose:     roundForecastPrice(prediction.PredictedClose),
			CloseAbsError:      roundForecastPrice(closeAbs),
			ClosePctError:      roundForecastMetric(closePct),
			ActualDirection:    actualDirection,
			PredictedDirection: predictedDirection,
			DirectionCorrect:   directionCorrect,
		})
		samples++
	}
	if samples == 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	return nextSessionForecastBacktestMetrics{
		samples:             samples,
		openMAE:             totalOpenAbsErr / float64(samples),
		openMAEPct:          totalOpenAbsErrPct / float64(samples),
		closeMAE:            totalCloseAbsErr / float64(samples),
		closeMAEPct:         totalCloseAbsErrPct / float64(samples),
		directionHitRatePct: 100 * float64(directionHits) / float64(samples),
		hit050Pct:           100 * float64(hit050) / float64(samples),
		hit100Pct:           100 * float64(hit100) / float64(samples),
		hit200Pct:           100 * float64(hit200) / float64(samples),
		rows:                rows,
	}
}

func nextSessionForecastBISTBulletinVariantBacktest(records []datasource.DailyBulletinRecord, assetType string, limit int, useOverlay bool) nextSessionForecastBacktestMetrics {
	if len(records) < 30 || limit <= 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	start := len(records) - limit
	if start < 25 {
		start = 25
	}
	totalOpenAbsErr := 0.0
	totalOpenAbsErrPct := 0.0
	totalCloseAbsErr := 0.0
	totalCloseAbsErrPct := 0.0
	directionHits := 0
	hit050 := 0
	hit100 := 0
	hit200 := 0
	samples := 0
	rows := []NextSessionBacktestRow{}
	for nextIdx := start; nextIdx < len(records); nextIdx++ {
		actual := records[nextIdx]
		prefixRecords := append([]datasource.DailyBulletinRecord{}, records[:nextIdx]...)
		prefix := bistBulletinRecordsToCandles(prefixRecords)
		if len(prefix) < 25 || actual.Open <= 0 || actual.Close <= 0 {
			continue
		}
		snapshot, err := indicators.Snapshot(prefix)
		if err != nil {
			continue
		}
		bias := trendBias(prefix, snapshot, nil)
		prediction := computeNextSessionForecastModel(prefix, snapshot, bias, assetType, false)
		if useOverlay {
			prediction = applyBISTBulletinRecordsToNextSessionForecast(prediction, prefixRecords, assetType, "", false)
		}
		if !prediction.Computed || prediction.PredictedOpen <= 0 || prediction.PredictedClose <= 0 || prediction.LastClose <= 0 {
			continue
		}
		openAbs := math.Abs(prediction.PredictedOpen - actual.Open)
		closeAbs := math.Abs(prediction.PredictedClose - actual.Close)
		openPct := 100 * openAbs / actual.Open
		closePct := 100 * closeAbs / actual.Close
		totalOpenAbsErr += openAbs
		totalOpenAbsErrPct += openPct
		totalCloseAbsErr += closeAbs
		totalCloseAbsErrPct += closePct
		if closePct <= 0.50 {
			hit050++
		}
		if closePct <= 1.00 {
			hit100++
		}
		if closePct <= 2.00 {
			hit200++
		}
		predictedDirection := nextSessionForecastDirectionForJSON(prediction.PredictedClose, prediction.LastClose, true)
		actualDirection := nextSessionForecastDirectionForJSON(actual.Close, prediction.LastClose, false)
		directionCorrect := predictedDirection != "uncertain" && predictedDirection == actualDirection
		if directionCorrect {
			directionHits++
		}
		rows = append(rows, NextSessionBacktestRow{
			Date:               actual.TradingDate,
			PreviousClose:      roundForecastPrice(prediction.LastClose),
			ActualOpen:         roundForecastPrice(actual.Open),
			PredictedOpen:      roundForecastPrice(prediction.PredictedOpen),
			OpenAbsError:       roundForecastPrice(openAbs),
			OpenPctError:       roundForecastMetric(openPct),
			ActualClose:        roundForecastPrice(actual.Close),
			PredictedClose:     roundForecastPrice(prediction.PredictedClose),
			CloseAbsError:      roundForecastPrice(closeAbs),
			ClosePctError:      roundForecastMetric(closePct),
			ActualDirection:    actualDirection,
			PredictedDirection: predictedDirection,
			DirectionCorrect:   directionCorrect,
		})
		samples++
	}
	if samples == 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	return nextSessionForecastBacktestMetrics{
		samples:             samples,
		openMAE:             totalOpenAbsErr / float64(samples),
		openMAEPct:          totalOpenAbsErrPct / float64(samples),
		closeMAE:            totalCloseAbsErr / float64(samples),
		closeMAEPct:         totalCloseAbsErrPct / float64(samples),
		directionHitRatePct: 100 * float64(directionHits) / float64(samples),
		hit050Pct:           100 * float64(hit050) / float64(samples),
		hit100Pct:           100 * float64(hit100) / float64(samples),
		hit200Pct:           100 * float64(hit200) / float64(samples),
		rows:                rows,
	}
}

func nextSessionForecastBISTBulletinOpenAnchorBacktest(records []datasource.DailyBulletinRecord, assetType string, limit int) nextSessionForecastBacktestMetrics {
	return nextSessionForecastBISTBulletinOpenAnchorVariantBacktest(records, assetType, limit, true)
}

func nextSessionForecastBISTBulletinOpenAnchorOpenOnlyBacktest(records []datasource.DailyBulletinRecord, assetType string, limit int) nextSessionForecastBacktestMetrics {
	return nextSessionForecastBISTBulletinOpenAnchorVariantBacktest(records, assetType, limit, false)
}

func nextSessionForecastBISTBulletinOpenAnchorVariantBacktest(records []datasource.DailyBulletinRecord, assetType string, limit int, closeFallback bool) nextSessionForecastBacktestMetrics {
	if len(records) < 30 || limit <= 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	start := len(records) - limit
	if start < 25 {
		start = 25
	}
	totalOpenAbsErr := 0.0
	totalOpenAbsErrPct := 0.0
	totalCloseAbsErr := 0.0
	totalCloseAbsErrPct := 0.0
	directionHits := 0
	hit050 := 0
	hit100 := 0
	hit200 := 0
	samples := 0
	rows := []NextSessionBacktestRow{}
	for nextIdx := start; nextIdx < len(records); nextIdx++ {
		actual := records[nextIdx]
		prefixRecords := append([]datasource.DailyBulletinRecord{}, records[:nextIdx]...)
		prefix := bistBulletinRecordsToCandles(prefixRecords)
		if len(prefix) < 25 || actual.Open <= 0 || actual.Close <= 0 {
			continue
		}
		snapshot, err := indicators.Snapshot(prefix)
		if err != nil {
			continue
		}
		bias := trendBias(prefix, snapshot, nil)
		basePrediction := computeNextSessionForecastModel(prefix, snapshot, bias, assetType, false)
		overlayPrediction := applyBISTBulletinRecordsToNextSessionForecast(basePrediction, prefixRecords, assetType, "", false)
		if !basePrediction.Computed || !overlayPrediction.Computed || overlayPrediction.PredictedOpen <= 0 || basePrediction.LastClose <= 0 {
			continue
		}
		predictedOpen := overlayPrediction.PredictedOpen
		predictedClose := predictedOpen
		if !closeFallback {
			if basePrediction.PredictedClose <= 0 {
				continue
			}
			predictedClose = basePrediction.PredictedClose
		}
		openAbs := math.Abs(predictedOpen - actual.Open)
		closeAbs := math.Abs(predictedClose - actual.Close)
		openPct := 100 * openAbs / actual.Open
		closePct := 100 * closeAbs / actual.Close
		totalOpenAbsErr += openAbs
		totalOpenAbsErrPct += openPct
		totalCloseAbsErr += closeAbs
		totalCloseAbsErrPct += closePct
		if closePct <= 0.50 {
			hit050++
		}
		if closePct <= 1.00 {
			hit100++
		}
		if closePct <= 2.00 {
			hit200++
		}
		predictedDirection := nextSessionForecastDirectionForJSON(predictedClose, basePrediction.LastClose, true)
		actualDirection := nextSessionForecastDirectionForJSON(actual.Close, basePrediction.LastClose, false)
		directionCorrect := predictedDirection != "uncertain" && predictedDirection == actualDirection
		if directionCorrect {
			directionHits++
		}
		rows = append(rows, NextSessionBacktestRow{
			Date:               actual.TradingDate,
			PreviousClose:      roundForecastPrice(basePrediction.LastClose),
			ActualOpen:         roundForecastPrice(actual.Open),
			PredictedOpen:      roundForecastPrice(predictedOpen),
			OpenAbsError:       roundForecastPrice(openAbs),
			OpenPctError:       roundForecastMetric(openPct),
			ActualClose:        roundForecastPrice(actual.Close),
			PredictedClose:     roundForecastPrice(predictedClose),
			CloseAbsError:      roundForecastPrice(closeAbs),
			ClosePctError:      roundForecastMetric(closePct),
			ActualDirection:    actualDirection,
			PredictedDirection: predictedDirection,
			DirectionCorrect:   directionCorrect,
		})
		samples++
	}
	if samples == 0 {
		return nextSessionForecastBacktestMetrics{}
	}
	return nextSessionForecastBacktestMetrics{
		samples:             samples,
		openMAE:             totalOpenAbsErr / float64(samples),
		openMAEPct:          totalOpenAbsErrPct / float64(samples),
		closeMAE:            totalCloseAbsErr / float64(samples),
		closeMAEPct:         totalCloseAbsErrPct / float64(samples),
		directionHitRatePct: 100 * float64(directionHits) / float64(samples),
		hit050Pct:           100 * float64(hit050) / float64(samples),
		hit100Pct:           100 * float64(hit100) / float64(samples),
		hit200Pct:           100 * float64(hit200) / float64(samples),
		rows:                rows,
	}
}

func bistBulletinOverlayValidationAllowsUse(records []datasource.DailyBulletinRecord, assetType string) (bool, nextSessionForecastBacktestMetrics, nextSessionForecastBacktestMetrics) {
	baseline := nextSessionForecastBISTBulletinVariantBacktest(records, assetType, forecastRollingBacktestWindow, false)
	overlay := nextSessionForecastBISTBulletinVariantBacktest(records, assetType, forecastRollingBacktestWindow, true)
	if baseline.samples < nextSessionPointForecastMinBacktestSamples || overlay.samples < nextSessionPointForecastMinBacktestSamples {
		return false, baseline, overlay
	}
	if overlay.closeMAEPct > nextSessionDecisionMaxCloseMAPEPct ||
		overlay.directionHitRatePct < nextSessionDecisionMinDirectionAccuracyPct {
		return false, baseline, overlay
	}
	if overlay.directionHitRatePct+0.01 < baseline.directionHitRatePct && overlay.closeMAEPct > baseline.closeMAEPct+0.10 {
		return false, baseline, overlay
	}
	if overlay.closeMAEPct > baseline.closeMAEPct+0.25 {
		return false, baseline, overlay
	}
	return true, baseline, overlay
}

func bistBulletinOpenAnchorValidationAllowsUse(baseline, overlay, openAnchor nextSessionForecastBacktestMetrics) bool {
	if baseline.samples < nextSessionPointForecastMinBacktestSamples ||
		overlay.samples < nextSessionPointForecastMinBacktestSamples ||
		openAnchor.samples < nextSessionPointForecastMinBacktestSamples {
		return false
	}
	openOverlayUseful := overlay.openMAEPct <= math.Min(baseline.openMAEPct+0.10, 1.25)
	closeAnchorBetter := openAnchor.closeMAEPct+0.10 < math.Min(baseline.closeMAEPct, overlay.closeMAEPct)
	directionAnchorUseful := openAnchor.directionHitRatePct >= nextSessionDecisionMinDirectionAccuracyPct &&
		openAnchor.closeMAEPct <= math.Min(baseline.closeMAEPct, overlay.closeMAEPct)+0.10
	return openOverlayUseful && (closeAnchorBetter || directionAnchorUseful)
}

func bistBulletinBacktestPreferred(candidate, incumbent nextSessionForecastBacktestMetrics) bool {
	if candidate.samples < nextSessionPointForecastMinBacktestSamples {
		return false
	}
	if incumbent.samples < nextSessionPointForecastMinBacktestSamples {
		return true
	}
	if candidate.closeMAEPct+0.10 < incumbent.closeMAEPct {
		return true
	}
	if math.Abs(candidate.closeMAEPct-incumbent.closeMAEPct) <= 0.10 &&
		candidate.directionHitRatePct > incumbent.directionHitRatePct+2 {
		return true
	}
	return false
}

func applyBISTBulletinOpenAnchorForecast(base, overlay NextSessionForecast, metrics nextSessionForecastBacktestMetrics, assetType, symbol string, closeFallback bool) NextSessionForecast {
	out := base
	openTarget := firstPositiveFloat(overlay.PredictedOpen, overlay.RawPredictedOpen)
	if openTarget <= 0 {
		return out
	}
	openTarget = roundForecastPrice(openTarget)
	closeTarget := openTarget
	if !closeFallback {
		closeTarget = firstPositiveFloat(base.PredictedClose, base.RawPredictedClose)
		if closeTarget <= 0 {
			closeTarget = firstPositiveFloat(overlay.PredictedClose, overlay.RawPredictedClose)
		}
		if closeTarget <= 0 {
			return out
		}
	}
	closeTarget = roundForecastPrice(closeTarget)
	out.RawPredictedOpen = openTarget
	out.RawPredictedClose = closeTarget
	out.PredictedOpen = openTarget
	out.PredictedClose = closeTarget
	if out.RawExpectedLow <= 0 {
		out.RawExpectedLow = out.ExpectedLow
	}
	if out.RawExpectedHigh <= 0 {
		out.RawExpectedHigh = out.ExpectedHigh
	}
	if out.RawExpectedLow > 0 {
		out.RawExpectedLow = roundForecastPrice(math.Min(out.RawExpectedLow, math.Min(openTarget, closeTarget)))
	}
	if out.RawExpectedHigh > 0 {
		out.RawExpectedHigh = roundForecastPrice(math.Max(out.RawExpectedHigh, math.Max(openTarget, closeTarget)))
	}
	out.ExpectedLow = out.RawExpectedLow
	out.ExpectedHigh = out.RawExpectedHigh
	if out.LastClose > 0 {
		out.OpenChangePct = roundForecastMetric(100 * (out.PredictedOpen/out.LastClose - 1))
		out.CloseChangePct = roundForecastMetric(100 * (out.PredictedClose/out.LastClose - 1))
	}
	switch {
	case out.CloseChangePct >= 0.35:
		out.DirectionBias = "yükseliş"
	case out.CloseChangePct <= -0.35:
		out.DirectionBias = "düşüş"
	default:
		out.DirectionBias = "yatay"
	}
	out.BiasStrength = "validasyon zayıf"
	out.Confidence = roundForecastMetric(math.Min(out.Confidence, 35))
	out.ConfidenceLabel = nextSessionConfidenceLabel(out.Confidence)
	if closeFallback {
		out.Model = withForecastModelOverlay(out.Model, "bist_open_anchor_close_fallback_v1")
		out.BiasReasons = appendUniqueAnalysisString(out.BiasReasons, fmt.Sprintf(
			"BIST açılış katmanı kullanıldı, kapanış overlay'i rolling backtest daha iyi olduğu için açılış çıpasına indirildi: son %d örnekte açılış MAE %.2f%%, kapanış MAE %.2f%%, yön uyumu %.2f%%.",
			metrics.samples,
			metrics.openMAEPct,
			metrics.closeMAEPct,
			metrics.directionHitRatePct,
		))
		out.Warnings = appendUniqueAnalysisString(out.Warnings, "bist_bulletin_close_overlay_validation_failed_open_anchor_used")
	} else {
		out.Model = withForecastModelOverlay(out.Model, "bist_open_anchor_open_only_v1")
		out.BiasReasons = appendUniqueAnalysisString(out.BiasReasons, fmt.Sprintf(
			"BIST açılış katmanı kullanıldı, kapanış baz modelden ayrı hesaplandı: son %d örnekte açılış MAE %.2f%%, kapanış MAE %.2f%%, yön uyumu %.2f%%.",
			metrics.samples,
			metrics.openMAEPct,
			metrics.closeMAEPct,
			metrics.directionHitRatePct,
		))
		out.Warnings = appendUniqueAnalysisString(out.Warnings, "bist_bulletin_open_anchor_used_close_kept_separate")
	}
	out.Warnings = appendUniqueAnalysisString(out.Warnings, "forecast_model_validation_failed_not_decision_grade")
	return out
}

func dampWeakBISTBulletinOverlayForecast(f NextSessionForecast, metrics nextSessionForecastBacktestMetrics) NextSessionForecast {
	if metrics.samples < nextSessionPointForecastMinBacktestSamples || f.LastClose <= 0 {
		return f
	}
	if metrics.directionHitRatePct >= nextSessionPointForecastMinDirectionHitPct && metrics.closeMAEPct <= nextSessionPointForecastMaxCloseMAEPct {
		return f
	}
	factor := 0.50
	rawOpen := forecastRawPredictedOpen(f)
	rawClose := forecastRawPredictedClose(f)
	if rawOpen > 0 {
		f.RawPredictedOpen = roundForecastPrice(f.LastClose + factor*(rawOpen-f.LastClose))
		f.PredictedOpen = f.RawPredictedOpen
	}
	if rawClose > 0 {
		f.RawPredictedClose = roundForecastPrice(f.LastClose + factor*(rawClose-f.LastClose))
		f.PredictedClose = f.RawPredictedClose
	}
	if f.LastClose > 0 {
		f.OpenChangePct = roundForecastMetric(100 * (f.PredictedOpen/f.LastClose - 1))
		f.CloseChangePct = roundForecastMetric(100 * (f.PredictedClose/f.LastClose - 1))
	}
	f.BiasReasons = appendUniqueAnalysisString(f.BiasReasons, fmt.Sprintf(
		"BIST analog/mikro yapı fiyat hareketi sönümlendi: son %d örnekte yön uyumu %.2f%%, kapanış MAE %.2f%%; hareket katsayısı %.2f.",
		metrics.samples,
		metrics.directionHitRatePct,
		metrics.closeMAEPct,
		factor,
	))
	f.Warnings = appendUniqueAnalysisString(f.Warnings, "bist_bulletin_overlay_damped_by_validation")
	return f
}

func forecastDirectionHit(predictedClose, lastClose, actualClose float64) bool {
	hit, ok := forecastpolicy.DirectionHit(predictedClose, actualClose, lastClose)
	return ok && hit
}

func roundForecastPrice(value float64) float64 {
	return math.Round(value*100) / 100
}

func roundForecastMetric(value float64) float64 {
	return math.Round(value*100) / 100
}

func nextSessionConfidenceLabel(confidence float64) string {
	switch {
	case confidence >= 70:
		return "medium"
	case confidence >= 50:
		return "low_to_medium"
	case confidence >= 35:
		return "low"
	default:
		return "very_low"
	}
}

func trendBias(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, pats []ohlcv.PatternResult) string {
	bullish := 0.0
	bearish := 0.0
	lastClose := candles[len(candles)-1].EffectiveClose()

	// ADX filter: reduce weight of trend signals in ranging markets
	adxWeight := 1.0
	if snapshot.ADX14 < 20 {
		adxWeight = 0.5
	} else if snapshot.ADX14 >= 30 {
		adxWeight = 1.3
	}

	// EMA alignment: scored per independent component so partial alignment counts.
	// Each component contributes proportionally; combined max ≈ 3.0 (same as before).
	if snapshot.EMA20 > 0 {
		if lastClose > snapshot.EMA20 {
			bullish += 0.8 * adxWeight
		} else {
			bearish += 0.8 * adxWeight
		}
	}
	if snapshot.EMA20 > 0 && snapshot.EMA50 > 0 {
		if snapshot.EMA20 > snapshot.EMA50 {
			bullish += 1.0 * adxWeight
		} else {
			bearish += 1.0 * adxWeight
		}
	}
	if snapshot.EMA50 > 0 && snapshot.EMA200 > 0 {
		if snapshot.EMA50 > snapshot.EMA200 {
			bullish += 1.2 * adxWeight
		} else {
			bearish += 1.2 * adxWeight
		}
	}

	// MACD histogram direction (MACDHistogram = MACD - MACDSignal, so this covers both)
	if snapshot.MACDHistogram > 0 {
		bullish += 1.5
	} else if snapshot.MACDHistogram < 0 {
		bearish += 1.5
	}

	// RSI momentum (scaled by distance from neutral)
	if snapshot.RSI14 > 55 {
		bullish += mathutil.Clamp((snapshot.RSI14-55)/15, 0, 1)
	} else if snapshot.RSI14 < 45 {
		bearish += mathutil.Clamp((45-snapshot.RSI14)/15, 0, 1)
	}

	// Supertrend direction
	if snapshot.Supertrend > 0 && lastClose > snapshot.Supertrend {
		bullish += 1.5 * adxWeight
	} else if snapshot.Supertrend > 0 && lastClose < snapshot.Supertrend {
		bearish += 1.5 * adxWeight
	}

	// Ichimoku cloud: weight by cloud thickness relative to price (thicker = stronger signal).
	if snapshot.IchimokuCloudTrend != 0 && lastClose > 0 {
		rawThickness := math.Abs(snapshot.IchimokuSenkouA-snapshot.IchimokuSenkouB) / lastClose * 100
		cloudWeight := mathutil.Clamp(0.3+rawThickness*0.24, 0.3, 1.5)
		if snapshot.IchimokuCloudTrend > 0 {
			bullish += cloudWeight
		} else {
			bearish += cloudWeight
		}
	}

	// Chikou confirmation: positive means current close > price 26 bars ago.
	if snapshot.IchimokuChikou > 0 {
		bullish += 0.5
	} else if snapshot.IchimokuChikou < 0 {
		bearish += 0.5
	}

	// Volume money flow
	if snapshot.ChaikinMoneyFlow20 > 0.05 {
		bullish += 0.5
	} else if snapshot.ChaikinMoneyFlow20 < -0.05 {
		bearish += 0.5
	}

	// Gap signal: significant opening gap carries directional momentum.
	if len(candles) >= 2 {
		prevClose := candles[len(candles)-2].EffectiveClose()
		if prevClose > 0 {
			gapPct := (lastClose - prevClose) / prevClose * 100
			if gapPct >= 3 {
				bullish += 0.5
			} else if gapPct <= -3 {
				bearish += 0.5
			}
		}
	}

	// OBV trend: rising OBV confirms accumulation, falling OBV confirms distribution.
	if snapshot.OBVSlope > 0.01 {
		bullish += 0.4
	} else if snapshot.OBVSlope < -0.01 {
		bearish += 0.4
	}

	// Divergence signals (RSI + MACD histogram): leading reversal indicators.
	divs := indicators.DetectDivergences(candles)
	if divs.RSI.Bullish || divs.MACD.Bullish {
		bullish += 0.6
	}
	if divs.RSI.Bearish || divs.MACD.Bearish {
		bearish += 0.6
	}

	// Supertrend crossover: fresh direction change carries extra weight.
	if snapshot.SupertrendPrev > 0 && snapshot.Supertrend > 0 {
		prevAbove := candles[len(candles)-2].EffectiveClose() > snapshot.SupertrendPrev
		currAbove := lastClose > snapshot.Supertrend
		if !prevAbove && currAbove {
			bullish += 0.8 // bullish crossover this bar
		} else if prevAbove && !currAbove {
			bearish += 0.8 // bearish crossover this bar
		}
	}

	// Pattern signals
	for _, pattern := range pats {
		if pattern.Confidence < 0.55 {
			continue
		}
		if pattern.Direction == "bullish" {
			bullish += pattern.Confidence
		} else if pattern.Direction == "bearish" {
			bearish += pattern.Confidence
		}
	}

	if bullish >= bearish+1.5 {
		return "bullish"
	}
	if bearish >= bullish+1.5 {
		return "bearish"
	}
	return "neutral"
}

func scoreTimeframe(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, pats []ohlcv.PatternResult, sr supportresistance.Result, plan ohlcv.TradePlan, bias string) float64 {
	lastClose := candles[len(candles)-1].EffectiveClose()
	score := 0.0

	// --- Trend (25 pts) ---
	trendScore := 0.0
	if lastClose > snapshot.EMA20 {
		trendScore += 5
	}
	if snapshot.EMA20 > snapshot.EMA50 {
		trendScore += 5
	}
	if snapshot.EMA50 > snapshot.EMA200 {
		trendScore += 7
	}
	if snapshot.Supertrend > 0 && lastClose > snapshot.Supertrend {
		trendScore += 4
	}
	if snapshot.IchimokuCloudTrend > 0 {
		trendScore += 4
	}
	score += mathutil.Clamp(trendScore, 0, 25)

	// --- Momentum (20 pts) ---
	momentum := 10.0
	switch {
	case snapshot.RSI14 < 30:
		momentum -= 4
	case snapshot.RSI14 >= 30 && snapshot.RSI14 < 45:
		momentum -= 2
	case snapshot.RSI14 >= 45 && snapshot.RSI14 <= 65:
		momentum += 4
	case snapshot.RSI14 > 70:
		momentum -= 2
	}
	if snapshot.MACDHistogram > 0 {
		momentum += 6
	} else if snapshot.MACDHistogram < 0 {
		momentum -= 6
	}
	score += mathutil.Clamp(momentum, 0, 20)

	// --- Volume (15 pts) ---
	volume := 0.0
	if snapshot.VolumeSMA20 > 0 {
		ratio := candles[len(candles)-1].EffectiveVolume() / snapshot.VolumeSMA20
		switch {
		case ratio >= 1.5:
			volume = 12
		case ratio >= 1.0:
			volume = 8
		case ratio >= 0.75:
			volume = 4
		}
	}
	if snapshot.ChaikinMoneyFlow20 > 0.05 {
		volume += 3
	} else if snapshot.ChaikinMoneyFlow20 < -0.05 {
		volume -= 3
	}
	score += mathutil.Clamp(volume, 0, 15)

	// --- Patterns (20 pts) ---
	patternScore := 0.0
	for _, pattern := range pats {
		if pattern.Direction == "bullish" {
			patternScore += pattern.Confidence * 2
		}
		if pattern.Direction == "bearish" {
			patternScore -= pattern.Confidence * 2.0
		}
	}
	score += mathutil.Clamp(patternScore, 0, 20)

	// --- Support/Resistance position (10 pts) ---
	// Near support in an uptrend = high score (bounce); near resistance in a downtrend = low score.
	// Trend bias is used to contextualise whether the position is favourable.
	position := 5.0
	if sr.NearestSupport != nil && sr.NearestResistance != nil {
		rangeSize := sr.NearestResistance.Price - sr.NearestSupport.Price
		if rangeSize > 0 {
			positionInRange := mathutil.SafeDiv(lastClose-sr.NearestSupport.Price, rangeSize) // 0 = at support, 1 = at resistance
			switch bias {
			case "bullish":
				// Closer to support is better (room to run)
				position = 10 * (1 - positionInRange)
			case "bearish":
				// Closer to resistance is more exposed (room to fall)
				position = 10 * positionInRange
			default:
				// Neutral: midrange scores highest
				position = 10 * (1 - math.Abs(positionInRange-0.5)*2)
			}
		}
	}
	score += mathutil.Clamp(position, 0, 10)

	// --- Risk/Reward (10 pts) ---
	rrScore := mathutil.Clamp(plan.RiskRewardRatio/3, 0, 1) * 10
	if plan.Rejected {
		rrScore *= 0.35
	}
	score += rrScore

	// --- Confluence bonus (up to 5 pts) ---
	bullC, bearC := indicatorConfluence(snapshot, lastClose)
	totalC := bullC + bearC
	if totalC > 0 {
		maxAgree := bullC
		if bearC > bullC {
			maxAgree = bearC
		}
		confluenceRatio := float64(maxAgree) / float64(totalC)
		if confluenceRatio >= 0.75 {
			score += 5 * (confluenceRatio - 0.5) * 2
		}
	}

	// --- Volatility regime adjustment ---
	regime := volatilityRegime(snapshot)
	if regime == "high" && snapshot.ADX14 < 25 {
		// High volatility + weak trend = unreliable signals
		score *= 0.88
	} else if regime == "low" {
		// Consolidation phase — reduce confidence slightly
		score *= 0.95
	}

	return mathutil.Clamp(score, 0, 100)
}

func overallBias(timeframes map[string]TimeframeAnalysis, score float64) string {
	// Timeframe weights: shorter timeframes matter more for trading decisions
	tfWeights := map[string]float64{
		"1D": 3.0, "1W": 2.0, "1M": 1.5,
		"3M": 1.0, "6M": 1.0, "1Y": 0.8, "YTD": 0.8, "ALL": 0.5,
	}
	bullish := 0.0
	bearish := 0.0
	total := 0.0
	for key, tf := range timeframes {
		w := tfWeights[key]
		if w == 0 {
			w = 1.0
		}
		// Additional weight from trend strength: strong ADX = more conviction
		if tf.Indicators.ADX14 >= 25 {
			w *= 1.3
		} else if tf.Indicators.ADX14 < 15 {
			w *= 0.7
		}
		if tf.TrendBias == "bullish" {
			bullish += w
		} else if tf.TrendBias == "bearish" {
			bearish += w
		}
		total += w
	}
	if total == 0 {
		return "neutral"
	}
	bullRatio := bullish / total
	bearRatio := bearish / total
	if score >= 58 && bullRatio >= 0.50 {
		return "bullish"
	}
	if score <= 42 && bearRatio >= 0.50 {
		return "bearish"
	}
	return "neutral"
}

// multiTimeframeAlignment checks whether 1D, 1W and 1M trend biases agree.
// Returns "aligned" (all non-neutral agree), "mixed" (too few non-neutral), or "conflicting" (disagreement).
func multiTimeframeAlignment(timeframes map[string]TimeframeAnalysis) string {
	var biases []string
	for _, key := range []string{"1D", "1W", "1M"} {
		if tf, ok := timeframes[key]; ok && tf.TrendBias != "neutral" {
			biases = append(biases, tf.TrendBias)
		}
	}
	if len(biases) < 2 {
		return "mixed"
	}
	first := biases[0]
	for _, b := range biases[1:] {
		if b != first {
			return "conflicting"
		}
	}
	return "aligned"
}

// applyMTFAlignmentAdjustment nudges the overall score up when timeframes agree
// and down when they conflict, providing a ±5 point adjustment.
func applyMTFAlignmentAdjustment(score float64, alignment string) float64 {
	switch alignment {
	case "aligned":
		return mathutil.Clamp(score+5, 0, 100)
	case "conflicting":
		return mathutil.Clamp(score-5, 0, 100)
	default:
		return score
	}
}

func integratedOverallScore(technicalScore float64, pro professional.Report, assetType string) float64 {
	if ohlcv.IsCryptoAssetType(assetType) || ohlcv.IsCommodityAssetType(assetType) {
		return mathutil.Clamp(technicalScore, 0, 100)
	}
	if !hasProfessionalReport(pro) {
		return mathutil.Clamp(technicalScore, 0, 100)
	}
	fundamental := fundamentalScore(pro)
	score := technicalScore*0.70 + fundamental*0.30
	if pro.TCMBEVDSContext.ScoreEligible {
		score += mathutil.Clamp(pro.TCMBEVDSContext.ScoreAdjustment, -8, 8)
	}
	if capScore, ok := macroHeadwindScoreCap(pro); ok {
		score = math.Min(score, capScore)
	}
	if strings.EqualFold(pro.Peers.ValuationSignal, "premium") {
		if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok && baseReturn < 0 {
			score = math.Min(score, 54)
		}
	}
	if isBankProfessionalReport(pro) && !professional.BankRegulatoryMetricsComplete(pro.SectorFinancials) {
		score = math.Min(score, 59)
	}
	if pro.DataQuality > 0 && pro.DataQuality < 65 {
		score = math.Min(score, 49)
	}
	return mathutil.Clamp(score, 0, 100)
}

func macroHeadwindScoreCap(pro professional.Report) (float64, bool) {
	impact := pro.TCMBEVDSContext.ForecastImpact
	if !impact.Computed || impact.Direction != "negative" || impact.Confidence < 65 {
		return 0, false
	}
	switch {
	case impact.DecisionUse == "blocking_headwind" || impact.Severity == "high" || impact.PressureScore <= -70:
		return 59, true
	case impact.Severity == "moderate" || impact.PressureScore <= -45:
		return 64, true
	default:
		return 0, false
	}
}

func symbolDataDir(root string, assetType string, symbol string) string {
	base := root
	if filepath.Base(root) == "equities" {
		switch {
		case ohlcv.IsCryptoAssetType(assetType):
			base = filepath.Join(filepath.Dir(root), "crypto")
		case ohlcv.IsCommodityAssetType(assetType):
			base = filepath.Join(filepath.Dir(root), "commodities")
		}
	}
	return filepath.Join(base, ohlcv.SymbolPathKey(symbol))
}

func fundamentalScore(pro professional.Report) float64 {
	if isBankProfessionalReport(pro) {
		return bankFundamentalScore(pro)
	}
	score := mathutil.Clamp(pro.DataQuality/100, 0, 1) * 34
	roe := pro.Valuation.Ratios["ROE"]
	switch {
	case roe >= 0.18:
		score += 18
	case roe >= 0.10:
		score += 14
	case roe > 0:
		score += 8
	}
	netDebtEq := pro.Valuation.Ratios["NetDebt_Eq"]
	switch {
	case netDebtEq <= 0:
		score += 12
	case netDebtEq <= 0.5:
		score += 10
	case netDebtEq <= 1:
		score += 5
	}
	if pro.Valuation.Ratios["Net_Margin"] > 0 {
		score += 7
	}
	if pro.Valuation.FreeCashFlowTTM > 0 {
		score += 7
	}
	switch strings.ToLower(pro.Peers.ValuationSignal) {
	case "discount":
		score += 10
	case "neutral":
		score += 6
	case "premium":
		score += 1
	default:
		score += 3
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok {
		switch {
		case baseReturn >= 20:
			score += 12
		case baseReturn >= 8:
			score += 9
		case baseReturn >= 0:
			score += 6
		case baseReturn > -10:
			score += 2
		}
	}
	return mathutil.Clamp(score, 0, 100)
}

func bankFundamentalScore(pro professional.Report) float64 {
	score := mathutil.Clamp(pro.DataQuality/100, 0, 1) * 26
	if pro.DataGovernance.FinanciallyConsistent {
		score += 6
	}
	regulatoryCompleteness := professional.BankRegulatoryMetricCompletenessScore(pro.SectorFinancials) / 100
	score += regulatoryCompleteness * 24
	roe := pro.Valuation.Ratios["ROE"]
	switch {
	case roe >= 0.25:
		score += 14
	case roe >= 0.15:
		score += 10
	case roe > 0:
		score += 5
	}
	roa := pro.Valuation.Ratios["ROA"]
	switch {
	case roa >= 0.025:
		score += 8
	case roa >= 0.012:
		score += 5
	case roa > 0:
		score += 2
	}
	pb := pro.Valuation.Ratios["PB"]
	switch {
	case pb > 0 && pb <= 0.8:
		score += 8
	case pb > 0 && pb <= 1.2:
		score += 5
	case pb > 0 && pb <= 2:
		score += 2
	}
	equityAssets := pro.Valuation.SectorMetrics["EquityToAssets"]
	if equityAssets == 0 {
		equityAssets = sectorFinancialMetricValue(pro.SectorFinancials, "equity_to_assets")
	}
	switch {
	case equityAssets >= 0.12:
		score += 6
	case equityAssets >= 0.08:
		score += 4
	case equityAssets > 0:
		score += 2
	}
	switch strings.ToLower(pro.Peers.ValuationSignal) {
	case "discount":
		score += 8
	case "neutral":
		score += 5
	case "premium":
		score += 1
	default:
		score += 2
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok {
		switch {
		case baseReturn >= 20:
			score += 6
		case baseReturn >= 8:
			score += 4
		case baseReturn >= 0:
			score += 2
		}
	}
	if !professional.BankRegulatoryMetricsComplete(pro.SectorFinancials) {
		score = math.Min(score, 55)
	}
	return mathutil.Clamp(score, 0, 100)
}

func isBankProfessionalReport(pro professional.Report) bool {
	if strings.EqualFold(strings.TrimSpace(pro.SectorFinancials.Profile), "bank") ||
		strings.EqualFold(strings.TrimSpace(pro.Valuation.SectorModel), "bank_equity_model") ||
		strings.EqualFold(strings.TrimSpace(pro.ValueInvesting.SectorModel.Model), "bank_equity_model") {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		pro.Company.Industry,
		pro.SectorFinancials.ProfileLabel,
		pro.ValueInvesting.SectorModel.Label,
	}, " "))
	return strings.Contains(text, "bank") || strings.Contains(text, "banka")
}

func sectorFinancialMetricValue(out professional.SectorFinancialAnalysis, name string) float64 {
	for _, metric := range out.Metrics {
		if strings.EqualFold(strings.TrimSpace(metric.Name), name) {
			return metric.Value
		}
	}
	return 0
}

func hasProfessionalReport(pro professional.Report) bool {
	return pro.DataQuality > 0 ||
		pro.Valuation.MarketCap > 0 ||
		pro.Valuation.Equity > 0 ||
		pro.Company.Sector != "" ||
		len(pro.Scenarios) > 0
}

func scenarioReturn(scenarios []professional.Scenario, name string) (float64, bool) {
	for _, scenario := range scenarios {
		if strings.EqualFold(scenario.Name, name) {
			return scenario.ReturnPct, true
		}
	}
	return 0, false
}

func (e *Engine) investorQAReport(result SymbolAnalysis) investorqa.Report {
	return BuildInvestorQAReport(result)
}

func BuildInvestorQAReport(result SymbolAnalysis) investorqa.Report {
	timeframes := map[string]investorqa.Timeframe{}
	for key, tf := range result.Timeframes {
		timeframes[key] = investorqa.Timeframe{
			Timeframe:         tf.Timeframe,
			LastClose:         tf.LastClose,
			LastVolume:        tf.LastVolume,
			Score:             tf.Score,
			TrendBias:         tf.TrendBias,
			Indicators:        tf.Indicators,
			NearestSupport:    tf.NearestSupport,
			NearestResistance: tf.NearestResistance,
			TradePlan:         tf.TradePlan,
			Liquidity:         tf.Professional.Liquidity,
			Backtest:          tf.Professional.Backtest,
			SignalStats:       tf.Professional.SignalStats,
			TechnicalGate:     tf.Professional.Technical.SignalGate,
			Range52W:          priceRange52W(tf.Candles, tf.LastClose),
		}
	}
	return investorqa.Analyze(investorqa.Input{
		Symbol:            result.Symbol,
		CompanyName:       result.CompanyName,
		Currency:          result.Currency,
		AssetType:         result.AssetType,
		OverallScore:      result.OverallScore,
		OverallBias:       result.OverallBias,
		Professional:      result.Professional,
		Behavioral:        result.Behavioral,
		PriceVerification: investorQAPriceVerification(result.PriceQuality),
		Timeframes:        timeframes,
	})
}

func investorQAPriceVerification(report *pricequality.SymbolReport) investorqa.PriceVerification {
	if report == nil {
		return investorqa.PriceVerification{}
	}
	out := investorqa.PriceVerification{
		Known:                 true,
		Status:                report.Status,
		ReadyForDecision:      report.ReadyForDecision || report.ReadyForVerifiedClose,
		ReadyForVerifiedClose: report.ReadyForVerifiedClose,
		LatestTradingDate:     report.LatestTradingDate,
		BlockingReasons:       append([]string{}, report.BlockingReasons...),
		MissingFields:         append([]string{}, report.MissingFields...),
	}
	if report.SelectedClose != nil {
		out.SelectedClose = report.SelectedClose.Close
		out.SelectedTradingDate = report.SelectedClose.TradingDate
	}
	return out
}

func priceRange52W(candles []ohlcv.Candle, current float64) investorqa.PriceRange {
	if len(candles) == 0 {
		return investorqa.PriceRange{}
	}
	lookback := 252
	if len(candles) < lookback {
		lookback = len(candles)
	}
	start := len(candles) - lookback
	low := math.Inf(1)
	high := 0.0
	for _, candle := range candles[start:] {
		if value := candle.EffectiveLow(); value > 0 && value < low {
			low = value
		}
		if value := candle.EffectiveHigh(); value > high {
			high = value
		}
	}
	if current <= 0 {
		current = candles[len(candles)-1].EffectiveClose()
	}
	if math.IsInf(low, 1) || high <= 0 || current <= 0 {
		return investorqa.PriceRange{}
	}
	position := 50.0
	if high > low {
		position = mathutil.Clamp((current-low)/(high-low)*100, 0, 100)
	}
	label := "52 hafta"
	if lookback < 240 {
		label = fmt.Sprintf("son %d günlük veri", lookback)
	}
	return investorqa.PriceRange{
		Label:       label,
		Low:         low,
		High:        high,
		Current:     current,
		PositionPct: position,
		SampleSize:  lookback,
	}
}

func PatternNames(patterns []ohlcv.PatternResult, limit int) []string {
	names := []string{}
	for _, pattern := range patterns {
		if pattern.Confidence >= 0.5 {
			names = append(names, fmt.Sprintf("%s (%s %.2f)", localize.PatternName(pattern.Name), localize.Direction(pattern.Direction), pattern.Confidence))
		}
		if limit > 0 && len(names) >= limit {
			break
		}
	}
	return names
}

func mathRound(value float64) float64 {
	return math.Round(value*100) / 100
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ = mathRound
