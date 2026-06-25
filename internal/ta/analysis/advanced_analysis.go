package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	"hissebot/internal/quant/core"
	"hissebot/internal/ta/professional"
)

type AdvancedAnalysis struct {
	Computed           bool                         `json:"computed"`
	Status             string                       `json:"status,omitempty"`
	SchemaVersion      int                          `json:"schema_version"`
	CompositeScore     float64                      `json:"composite_score,omitempty"`
	ProductionReady    bool                         `json:"production_ready"`
	DecisionImpact     string                       `json:"decision_impact,omitempty"`
	DataQuality        AdvancedDataQuality          `json:"data_quality"`
	PriceReconcile     AdvancedPriceReconciliation  `json:"price_reconciliation"`
	CorporateAction    AdvancedCorporateActionAudit `json:"corporate_action_audit"`
	PointInTime        AdvancedPointInTimeLineage   `json:"point_in_time_lineage"`
	FactorModel        AdvancedFactorModel          `json:"factor_model"`
	Volatility         AdvancedVolatilityRegime     `json:"volatility_regime"`
	Macro              AdvancedMacroModel           `json:"macro_model"`
	FinancialQuality   AdvancedFinancialQuality     `json:"financial_quality"`
	Valuation          AdvancedValuationEnsemble    `json:"valuation_ensemble"`
	EventStudy         AdvancedEventStudy           `json:"event_study"`
	ModelMonitoring    AdvancedModelMonitoring      `json:"model_monitoring"`
	LiquidityPortfolio AdvancedLiquidityPortfolio   `json:"liquidity_portfolio"`
	Production         AdvancedProductionReadiness  `json:"production_readiness"`
	Phases             []AdvancedPhaseStatus        `json:"phases,omitempty"`
	Warnings           []string                     `json:"warnings,omitempty"`
}

type AdvancedPhaseStatus struct {
	Phase       string   `json:"phase"`
	Status      string   `json:"status"`
	Score       float64  `json:"score,omitempty"`
	Computed    bool     `json:"computed"`
	Blocking    bool     `json:"blocking"`
	Missing     []string `json:"missing,omitempty"`
	NextSteps   []string `json:"next_steps,omitempty"`
	Description string   `json:"description,omitempty"`
}

type AdvancedDataQuality struct {
	Computed                    bool     `json:"computed"`
	Status                      string   `json:"status"`
	Score                       float64  `json:"score"`
	CandleCount                 int      `json:"candle_count"`
	DuplicateDates              int      `json:"duplicate_dates"`
	ChronologyViolations        int      `json:"chronology_violations"`
	PriceRuleViolations         int      `json:"price_rule_violations"`
	LargeCalendarGaps           int      `json:"large_calendar_gaps"`
	OfficialCloseVerified       bool     `json:"official_close_verified"`
	FinancialBacktestSafe       bool     `json:"financial_backtest_safe"`
	CorporateActionBacktestSafe bool     `json:"corporate_action_backtest_safe"`
	SurvivorshipBiasRisk        bool     `json:"survivorship_bias_risk"`
	MissingInputs               []string `json:"missing_inputs,omitempty"`
	Warnings                    []string `json:"warnings,omitempty"`
}

type AdvancedPriceReconciliation struct {
	Computed              bool                     `json:"computed"`
	Status                string                   `json:"status"`
	Score                 float64                  `json:"score"`
	SelectedClose         float64                  `json:"selected_close,omitempty"`
	AnalysisClose         float64                  `json:"analysis_close,omitempty"`
	DeltaPct              float64                  `json:"delta_pct,omitempty"`
	ReadyForDecision      bool                     `json:"ready_for_decision"`
	ReadyForVerifiedClose bool                     `json:"ready_for_verified_close"`
	Conflict              bool                     `json:"conflict"`
	Stale                 bool                     `json:"stale"`
	Candidates            []AdvancedCloseCandidate `json:"candidates,omitempty"`
	MissingInputs         []string                 `json:"missing_inputs,omitempty"`
	Warnings              []string                 `json:"warnings,omitempty"`
}

type AdvancedCloseCandidate struct {
	Source      string  `json:"source"`
	SourceType  string  `json:"source_type,omitempty"`
	Close       float64 `json:"close,omitempty"`
	TradingDate string  `json:"trading_date,omitempty"`
	Official    bool    `json:"official,omitempty"`
	Final       bool    `json:"final,omitempty"`
	Stale       bool    `json:"stale,omitempty"`
}

type AdvancedCorporateActionAudit struct {
	Computed               bool     `json:"computed"`
	Status                 string   `json:"status"`
	Score                  float64  `json:"score"`
	ActionCount            int      `json:"action_count"`
	VerifiedActions        int      `json:"verified_actions"`
	CandidateActions       int      `json:"candidate_actions"`
	ReviewRequiredActions  int      `json:"review_required_actions"`
	AdjustmentReadyActions int      `json:"adjustment_ready_actions"`
	MissingEffectiveDate   int      `json:"missing_effective_date_actions"`
	MissingAdjustment      int      `json:"missing_adjustment_actions"`
	PotentialSplitGapBars  int      `json:"potential_split_gap_bars"`
	BacktestSafe           bool     `json:"backtest_safe"`
	SourceFiles            []string `json:"source_files,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

type AdvancedPointInTimeLineage struct {
	Computed                  bool     `json:"computed"`
	Status                    string   `json:"status"`
	Score                     float64  `json:"score"`
	DataMode                  string   `json:"data_mode,omitempty"`
	AsOf                      string   `json:"as_of,omitempty"`
	LatestPeriod              string   `json:"latest_period,omitempty"`
	LatestPublishDate         string   `json:"latest_publish_date,omitempty"`
	LatestAvailableAt         string   `json:"latest_available_at,omitempty"`
	PublishDateCoveragePct    float64  `json:"publish_date_coverage_pct,omitempty"`
	AvailableAtCoveragePct    float64  `json:"available_at_coverage_pct,omitempty"`
	LineageEvents             int      `json:"lineage_events,omitempty"`
	StatementVersionCount     int      `json:"statement_version_count,omitempty"`
	RestatementCount          int      `json:"restatement_count,omitempty"`
	BacktestSafe              bool     `json:"backtest_safe"`
	ProductionReady           bool     `json:"production_ready"`
	FinanciallyConsistent     bool     `json:"financially_consistent"`
	MissingPublishPeriods     []string `json:"missing_publish_periods,omitempty"`
	MissingAvailableAtPeriods []string `json:"missing_available_at_periods,omitempty"`
	UnsafeBacktestPeriods     []string `json:"unsafe_backtest_periods,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
}

type AdvancedFactorModel struct {
	Computed                    bool                  `json:"computed"`
	Status                      string                `json:"status"`
	Score                       float64               `json:"score"`
	BenchmarkAvailable          bool                  `json:"benchmark_available"`
	SectorBenchmarkAvailable    bool                  `json:"sector_benchmark_available"`
	MarketBeta60                float64               `json:"market_beta_60,omitempty"`
	MarketAlpha60AnnualPct      float64               `json:"market_alpha_60_annual_pct,omitempty"`
	MarketCorrelation60         float64               `json:"market_correlation_60,omitempty"`
	SectorBeta60                float64               `json:"sector_beta_60,omitempty"`
	SectorAlpha60AnnualPct      float64               `json:"sector_alpha_60_annual_pct,omitempty"`
	RelativeStrength20Pct       float64               `json:"relative_strength_20_pct,omitempty"`
	RelativeStrength60Pct       float64               `json:"relative_strength_60_pct,omitempty"`
	SectorRelativeStrength60Pct float64               `json:"sector_relative_strength_60_pct,omitempty"`
	ActiveReturn20Pct           float64               `json:"active_return_20_pct,omitempty"`
	ActiveReturn60Pct           float64               `json:"active_return_60_pct,omitempty"`
	FactorExposures             []AdvancedFactorScore `json:"factor_exposures,omitempty"`
	MissingInputs               []string              `json:"missing_inputs,omitempty"`
	Warnings                    []string              `json:"warnings,omitempty"`
}

type AdvancedFactorScore struct {
	Name       string  `json:"name"`
	Exposure   float64 `json:"exposure,omitempty"`
	Score      float64 `json:"score,omitempty"`
	Status     string  `json:"status"`
	Evidence   string  `json:"evidence,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type AdvancedVolatilityRegime struct {
	Computed                   bool                       `json:"computed"`
	Status                     string                     `json:"status"`
	Score                      float64                    `json:"score"`
	Regime                     string                     `json:"regime,omitempty"`
	TrendRegime                string                     `json:"trend_regime,omitempty"`
	EWMAVolatilityAnnualPct    float64                    `json:"ewma_volatility_annual_pct,omitempty"`
	GARCHVolatilityForecastPct float64                    `json:"garch_volatility_forecast_pct,omitempty"`
	CornishFisherVaR95Pct      float64                    `json:"cornish_fisher_var_95_pct,omitempty"`
	EVTTailLossPct             float64                    `json:"evt_tail_loss_pct,omitempty"`
	VolatilityClusteringScore  float64                    `json:"volatility_clustering_score,omitempty"`
	StressScenarios            []StressScenario           `json:"stress_scenarios,omitempty"`
	BootstrapScenarios         []AdvancedStressSimulation `json:"bootstrap_scenarios,omitempty"`
	Warnings                   []string                   `json:"warnings,omitempty"`
}

type AdvancedStressSimulation struct {
	Name      string  `json:"name"`
	ReturnPct float64 `json:"return_pct"`
	Price     float64 `json:"price,omitempty"`
}

type AdvancedMacroModel struct {
	Computed                bool                    `json:"computed"`
	Status                  string                  `json:"status"`
	Score                   float64                 `json:"score"`
	PointInTimeSafe         bool                    `json:"point_in_time_safe"`
	SectorMacroProfile      string                  `json:"sector_macro_profile,omitempty"`
	RateSensitivity         string                  `json:"rate_sensitivity,omitempty"`
	FXSensitivity           string                  `json:"fx_sensitivity,omitempty"`
	InflationSensitivity    string                  `json:"inflation_sensitivity,omitempty"`
	CyclicalSensitivity     string                  `json:"cyclical_sensitivity,omitempty"`
	TCMBRegime              string                  `json:"tcmb_regime,omitempty"`
	ForecastImpactDirection string                  `json:"forecast_impact_direction,omitempty"`
	ForecastImpactSeverity  string                  `json:"forecast_impact_severity,omitempty"`
	MacroStressScenarios    []AdvancedMacroScenario `json:"macro_stress_scenarios,omitempty"`
	MissingInputs           []string                `json:"missing_inputs,omitempty"`
	Warnings                []string                `json:"warnings,omitempty"`
}

type AdvancedMacroScenario struct {
	Name           string  `json:"name"`
	Shock          string  `json:"shock"`
	ExpectedImpact string  `json:"expected_impact"`
	ReturnPct      float64 `json:"return_pct,omitempty"`
}

type AdvancedFinancialQuality struct {
	Computed                 bool                          `json:"computed"`
	Status                   string                        `json:"status"`
	Score                    float64                       `json:"score"`
	PiotroskiProxyScore      float64                       `json:"piotroski_proxy_score,omitempty"`
	BeneishRiskProxy         string                        `json:"beneish_risk_proxy,omitempty"`
	AltmanRiskProxy          string                        `json:"altman_risk_proxy,omitempty"`
	DuPontROE                float64                       `json:"dupont_roe,omitempty"`
	AccrualQualityProxyScore float64                       `json:"accrual_quality_proxy_score,omitempty"`
	EarningsPersistenceScore float64                       `json:"earnings_persistence_score,omitempty"`
	DebtSustainabilityScore  float64                       `json:"debt_sustainability_score,omitempty"`
	SectorSpecificMetrics    []AdvancedSectorQualityMetric `json:"sector_specific_metrics,omitempty"`
	RedFlags                 []string                      `json:"red_flags,omitempty"`
	MissingInputs            []string                      `json:"missing_inputs,omitempty"`
	Warnings                 []string                      `json:"warnings,omitempty"`
}

type AdvancedSectorQualityMetric struct {
	Name     string  `json:"name"`
	Value    float64 `json:"value,omitempty"`
	Status   string  `json:"status"`
	Evidence string  `json:"evidence,omitempty"`
}

type AdvancedValuationEnsemble struct {
	Computed          bool                           `json:"computed"`
	Status            string                         `json:"status"`
	Score             float64                        `json:"score"`
	CurrentPrice      float64                        `json:"current_price,omitempty"`
	BearFairValue     float64                        `json:"bear_fair_value,omitempty"`
	BaseFairValue     float64                        `json:"base_fair_value,omitempty"`
	BullFairValue     float64                        `json:"bull_fair_value,omitempty"`
	ExpectedUpsidePct float64                        `json:"expected_upside_pct,omitempty"`
	MarginOfSafetyPct float64                        `json:"margin_of_safety_pct,omitempty"`
	ModelReliability  float64                        `json:"model_reliability,omitempty"`
	Models            []AdvancedValuationModel       `json:"models,omitempty"`
	Sensitivity       []AdvancedValuationSensitivity `json:"sensitivity,omitempty"`
	MissingInputs     []string                       `json:"missing_inputs,omitempty"`
	Warnings          []string                       `json:"warnings,omitempty"`
}

type AdvancedValuationModel struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	FairValue  float64 `json:"fair_value,omitempty"`
	Weight     float64 `json:"weight,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Evidence   string  `json:"evidence,omitempty"`
}

type AdvancedValuationSensitivity struct {
	Name      string  `json:"name"`
	Variable  string  `json:"variable"`
	Change    string  `json:"change"`
	FairValue float64 `json:"fair_value,omitempty"`
}

type AdvancedEventStudy struct {
	Computed               bool                 `json:"computed"`
	Status                 string               `json:"status"`
	Score                  float64              `json:"score"`
	EventCount             int                  `json:"event_count"`
	TradableEventCount     int                  `json:"tradable_event_count"`
	AbnormalReturnProxyPct float64              `json:"abnormal_return_proxy_pct,omitempty"`
	VolumeShiftProxyPct    float64              `json:"volume_shift_proxy_pct,omitempty"`
	MaterialityScore       float64              `json:"materiality_score,omitempty"`
	LatestEvents           []AdvancedEventScore `json:"latest_events,omitempty"`
	MissingInputs          []string             `json:"missing_inputs,omitempty"`
	Warnings               []string             `json:"warnings,omitempty"`
}

type AdvancedEventScore struct {
	Type             string  `json:"type,omitempty"`
	Title            string  `json:"title,omitempty"`
	Period           string  `json:"period,omitempty"`
	MaterialityScore float64 `json:"materiality_score,omitempty"`
	ExpectedImpact   string  `json:"expected_impact,omitempty"`
}

type AdvancedModelMonitoring struct {
	Computed                    bool     `json:"computed"`
	Status                      string   `json:"status"`
	Score                       float64  `json:"score"`
	ForecastBacktestSamples     int      `json:"forecast_backtest_samples,omitempty"`
	ForecastDirectionHitRatePct float64  `json:"forecast_direction_hit_rate_pct,omitempty"`
	ForecastCloseMAEPct         float64  `json:"forecast_close_mae_pct,omitempty"`
	WalkForwardTrades           int      `json:"walk_forward_trades,omitempty"`
	OutOfSampleTrades           int      `json:"out_of_sample_trades,omitempty"`
	ModelRisk                   string   `json:"model_risk,omitempty"`
	ChampionModel               string   `json:"champion_model,omitempty"`
	ChallengerModel             string   `json:"challenger_model,omitempty"`
	DriftStatus                 string   `json:"drift_status,omitempty"`
	MissingInputs               []string `json:"missing_inputs,omitempty"`
	Warnings                    []string `json:"warnings,omitempty"`
}

type AdvancedLiquidityPortfolio struct {
	Computed                  bool     `json:"computed"`
	Status                    string   `json:"status"`
	Score                     float64  `json:"score"`
	ADV20TRY                  float64  `json:"adv_20_try,omitempty"`
	MedianValue20TRY          float64  `json:"median_value_20_try,omitempty"`
	SpreadBps                 float64  `json:"spread_bps,omitempty"`
	OrderBookImbalanceTop5    float64  `json:"order_book_imbalance_top5,omitempty"`
	SlippageEstimateBps       float64  `json:"slippage_estimate_bps,omitempty"`
	MarketImpactFor1MTRYBps   float64  `json:"market_impact_for_1m_try_bps,omitempty"`
	CapacityTRYAt10PctADV     float64  `json:"capacity_try_at_10pct_adv,omitempty"`
	DaysToExit1MTRY           float64  `json:"days_to_exit_1m_try,omitempty"`
	PortfolioVaR1DayPer100K   float64  `json:"portfolio_var_1d_per_100k,omitempty"`
	RecommendedMaxPositionTRY float64  `json:"recommended_max_position_try,omitempty"`
	MissingInputs             []string `json:"missing_inputs,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
}

type AdvancedProductionReadiness struct {
	Computed             bool     `json:"computed"`
	Status               string   `json:"status"`
	Score                float64  `json:"score"`
	Decision             string   `json:"decision"`
	ReportHash           string   `json:"report_hash,omitempty"`
	DataGate             string   `json:"data_gate"`
	ValidationGate       string   `json:"validation_gate"`
	RiskGate             string   `json:"risk_gate"`
	ValuationGate        string   `json:"valuation_gate"`
	ExecutionGate        string   `json:"execution_gate"`
	HumanReviewRequired  bool     `json:"human_review_required"`
	HumanReviewReasons   []string `json:"human_review_reasons,omitempty"`
	ModelRegistryVersion string   `json:"model_registry_version,omitempty"`
	AuditTrail           []string `json:"audit_trail,omitempty"`
}

func BuildAdvancedAnalysis(result SymbolAnalysis) AdvancedAnalysis {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		tf, ok = selectFinTradeBenchTimeframe(result.Timeframes)
	}
	out := AdvancedAnalysis{
		SchemaVersion: 1,
		Status:        "not_computed",
	}
	if !ok || len(tf.Candles) < 3 {
		out.Warnings = append(out.Warnings, "advanced_analysis_requires_daily_price_history")
		out.Phases = advancedPhaseStatuses(out)
		return out
	}
	closes := quantEffectiveCloses(tf.Candles)
	returns := quantFiniteReturns(core.Returns(closes))
	if len(closes) < 3 || len(returns) < 2 {
		out.Warnings = append(out.Warnings, "advanced_analysis_requires_valid_returns")
		out.Phases = advancedPhaseStatuses(out)
		return out
	}
	out.Computed = true
	out.Status = "computed"
	out.DataQuality = buildAdvancedDataQuality(result, tf, returns)
	out.PriceReconcile = buildAdvancedPriceReconciliation(result, tf)
	out.CorporateAction = buildAdvancedCorporateActionAudit(result, tf)
	out.PointInTime = buildAdvancedPointInTimeLineage(result)
	out.FactorModel = buildAdvancedFactorModel(result)
	out.Volatility = buildAdvancedVolatilityRegime(result, closes, returns)
	out.Macro = buildAdvancedMacroModel(result)
	out.FinancialQuality = buildAdvancedFinancialQuality(result)
	out.Valuation = buildAdvancedValuationEnsemble(result, tf.LastClose)
	out.EventStudy = buildAdvancedEventStudy(result, returns)
	out.ModelMonitoring = buildAdvancedModelMonitoring(result, tf)
	out.LiquidityPortfolio = buildAdvancedLiquidityPortfolio(result, tf)
	out.Production = buildAdvancedProductionReadiness(result, out)
	out.ProductionReady = out.Production.Status == "pass"
	out.CompositeScore = roundStat(weightedStatScore([]statScoreWeight{
		{out.DataQuality.Score, 0.12},
		{out.PointInTime.Score, 0.10},
		{out.FactorModel.Score, 0.10},
		{out.Volatility.Score, 0.10},
		{out.Macro.Score, 0.10},
		{out.FinancialQuality.Score, 0.12},
		{out.Valuation.Score, 0.12},
		{out.EventStudy.Score, 0.06},
		{out.ModelMonitoring.Score, 0.09},
		{out.LiquidityPortfolio.Score, 0.09},
		{out.Production.Score, 0.10},
	}))
	out.DecisionImpact = advancedDecisionImpact(out)
	out.Warnings = statCompactWarnings(
		out.DataQuality.Warnings,
		out.PriceReconcile.Warnings,
		out.CorporateAction.Warnings,
		out.PointInTime.Warnings,
		out.FactorModel.Warnings,
		out.Volatility.Warnings,
		out.Macro.Warnings,
		out.FinancialQuality.Warnings,
		out.Valuation.Warnings,
		out.EventStudy.Warnings,
		out.ModelMonitoring.Warnings,
		out.LiquidityPortfolio.Warnings,
	)
	out.Phases = advancedPhaseStatuses(out)
	return out
}

func buildAdvancedDataQuality(result SymbolAnalysis, tf TimeframeAnalysis, returns []float64) AdvancedDataQuality {
	stat := buildStatDataIntegrity(tf.Candles, returns, result)
	out := AdvancedDataQuality{
		Computed:                    true,
		Status:                      stat.Status,
		Score:                       stat.Score,
		CandleCount:                 stat.CandleCount,
		DuplicateDates:              stat.DuplicateDates,
		ChronologyViolations:        stat.ChronologyViolations,
		PriceRuleViolations:         stat.PriceRuleViolations,
		LargeCalendarGaps:           stat.LargeCalendarGaps,
		OfficialCloseVerified:       result.PriceQuality != nil && result.PriceQuality.ReadyForVerifiedClose,
		FinancialBacktestSafe:       result.Professional.DataGovernance.BacktestSafe,
		CorporateActionBacktestSafe: tf.Professional.PriceAdjustment.BacktestSafe,
		SurvivorshipBiasRisk:        result.Professional.DataGovernance.SurvivorshipBiasRisk,
		Warnings:                    append([]string{}, stat.Warnings...),
	}
	if result.PriceQuality == nil {
		out.MissingInputs = append(out.MissingInputs, "price_quality_report")
	} else if !result.PriceQuality.ReadyForVerifiedClose {
		out.Warnings = append(out.Warnings, "official_close_not_verified")
	}
	if !out.FinancialBacktestSafe {
		out.Warnings = append(out.Warnings, "financial_point_in_time_not_backtest_safe")
	}
	if !out.CorporateActionBacktestSafe {
		out.Warnings = append(out.Warnings, "corporate_action_adjustment_not_backtest_safe")
	}
	if out.SurvivorshipBiasRisk {
		out.Warnings = append(out.Warnings, "survivorship_bias_risk")
	}
	out.Score = roundStat(core.Clamp(out.Score-missingPenalty(out.MissingInputs)-float64(len(out.Warnings))*1.5, 0, 100))
	out.Status = scoreStatus(out.Score)
	return out
}

func buildAdvancedPriceReconciliation(result SymbolAnalysis, tf TimeframeAnalysis) AdvancedPriceReconciliation {
	out := AdvancedPriceReconciliation{Status: "missing", AnalysisClose: roundStat(tf.LastClose)}
	if result.PriceQuality == nil {
		out.MissingInputs = []string{"price_quality_report"}
		out.Warnings = []string{"price_quality_report_missing"}
		return out
	}
	pq := result.PriceQuality
	out.Computed = true
	out.ReadyForDecision = pq.ReadyForDecision
	out.ReadyForVerifiedClose = pq.ReadyForVerifiedClose
	out.Conflict = pq.Conflict
	out.Stale = pq.Stale
	for _, candidate := range pq.Candidates {
		out.Candidates = append(out.Candidates, AdvancedCloseCandidate{
			Source:      candidate.Source,
			SourceType:  candidate.SourceType,
			Close:       roundStat(candidate.Close),
			TradingDate: candidate.TradingDate,
			Official:    candidate.Official,
			Final:       candidate.Final,
			Stale:       candidate.Stale,
		})
	}
	if pq.SelectedClose != nil {
		out.SelectedClose = roundStat(pq.SelectedClose.Close)
		out.DeltaPct = roundStat(safeDiv(tf.LastClose-pq.SelectedClose.Close, pq.SelectedClose.Close) * 100)
	}
	out.Warnings = append(out.Warnings, pq.BlockingReasons...)
	out.MissingInputs = append(out.MissingInputs, pq.MissingFields...)
	score := 35.0
	if pq.ReadyForDecision {
		score += 25
	}
	if pq.ReadyForVerifiedClose {
		score += 30
	}
	if pq.Conflict {
		score -= 25
	}
	if pq.Stale {
		score -= 15
	}
	score -= missingPenalty(out.MissingInputs)
	out.Score = roundStat(core.Clamp(score, 0, 100))
	out.Status = scoreStatus(out.Score)
	return out
}

func buildAdvancedCorporateActionAudit(result SymbolAnalysis, tf TimeframeAnalysis) AdvancedCorporateActionAudit {
	ca := result.Professional.CorporateActions
	pa := tf.Professional.PriceAdjustment
	out := AdvancedCorporateActionAudit{
		Computed:               true,
		ActionCount:            len(ca.Actions),
		VerifiedActions:        ca.VerifiedActions,
		CandidateActions:       ca.CandidateActions,
		ReviewRequiredActions:  ca.ReviewRequiredActions,
		AdjustmentReadyActions: ca.AdjustmentReadyActions,
		MissingEffectiveDate:   ca.MissingEffectiveDateActions,
		MissingAdjustment:      ca.MissingAdjustmentActions,
		PotentialSplitGapBars:  pa.PotentialSplitGapBars,
		BacktestSafe:           pa.BacktestSafe,
		SourceFiles:            append([]string{}, ca.SourceFiles...),
		Warnings:               append([]string{}, ca.Warnings...),
	}
	out.Warnings = append(out.Warnings, pa.Warnings...)
	score := 70.0
	if pa.BacktestSafe {
		score += 20
	}
	score -= float64(out.ReviewRequiredActions) * 8
	score -= float64(out.MissingEffectiveDate) * 8
	score -= float64(out.MissingAdjustment) * 8
	score -= float64(out.PotentialSplitGapBars) * 4
	if out.ActionCount == 0 {
		score -= 5
		out.Warnings = append(out.Warnings, "corporate_action_source_empty_or_no_events")
	}
	out.Score = roundStat(core.Clamp(score, 0, 100))
	out.Status = scoreStatus(out.Score)
	return out
}

func buildAdvancedPointInTimeLineage(result SymbolAnalysis) AdvancedPointInTimeLineage {
	gov := result.Professional.DataGovernance
	out := AdvancedPointInTimeLineage{
		Computed:                  strings.TrimSpace(gov.AvailabilityStatus) != "" || gov.LineageEvents > 0 || gov.StatementVersionCount > 0,
		DataMode:                  gov.DataMode,
		LatestPeriod:              gov.LatestPeriod,
		PublishDateCoveragePct:    roundStat(gov.PublishDateCoverage * 100),
		AvailableAtCoveragePct:    roundStat(gov.AvailableAtCoverage * 100),
		LineageEvents:             gov.LineageEvents,
		StatementVersionCount:     gov.StatementVersionCount,
		RestatementCount:          gov.RestatementCount,
		BacktestSafe:              gov.BacktestSafe,
		ProductionReady:           gov.ProductionReady,
		FinanciallyConsistent:     gov.FinanciallyConsistent,
		MissingPublishPeriods:     append([]string{}, gov.MissingPublishPeriods...),
		MissingAvailableAtPeriods: append([]string{}, gov.MissingAvailableAtPeriods...),
		UnsafeBacktestPeriods:     append([]string{}, gov.UnsafeBacktestPeriods...),
		Warnings:                  append([]string{}, gov.Warnings...),
	}
	if !gov.AsOf.IsZero() {
		out.AsOf = gov.AsOf.Format("2006-01-02")
	}
	if gov.LatestPublishDate != nil {
		out.LatestPublishDate = gov.LatestPublishDate.Format("2006-01-02")
	}
	if gov.LatestAvailableAt != nil {
		out.LatestAvailableAt = gov.LatestAvailableAt.Format("2006-01-02")
	}
	score := 25.0
	if gov.BacktestSafe {
		score += 25
	}
	if gov.ProductionReady {
		score += 20
	}
	if gov.FinanciallyConsistent {
		score += 15
	}
	score += core.Clamp(gov.PublishDateCoverage*10, 0, 10)
	score += core.Clamp(gov.AvailableAtCoverage*10, 0, 10)
	score -= float64(gov.UnsafeAvailabilityCount) * 4
	score -= float64(gov.RestatementCount) * 2
	score -= float64(len(gov.InvalidChronologyPeriods)) * 5
	out.Score = roundStat(core.Clamp(score, 0, 100))
	out.Status = scoreStatus(out.Score)
	if !out.Computed {
		out.Status = "missing"
		out.Warnings = append(out.Warnings, "financial_lineage_not_available")
	}
	return out
}

func buildAdvancedFactorModel(result SymbolAnalysis) AdvancedFactorModel {
	stat := result.StatEconomic.FactorModel
	m := result.Professional.Market
	out := AdvancedFactorModel{
		Computed:                    result.Quant.Computed,
		Score:                       stat.Score,
		BenchmarkAvailable:          m.BenchmarkAvailable,
		SectorBenchmarkAvailable:    m.SectorBenchmarkAvailable,
		MarketBeta60:                stat.MarketBeta60,
		MarketAlpha60AnnualPct:      stat.MarketAlpha60AnnualPct,
		MarketCorrelation60:         stat.MarketCorrelation60,
		SectorBeta60:                stat.SectorBeta60,
		SectorAlpha60AnnualPct:      stat.SectorAlpha60AnnualPct,
		RelativeStrength20Pct:       roundStat(m.RelativeStrength20 * 100),
		RelativeStrength60Pct:       stat.RelativeStrength60Pct,
		SectorRelativeStrength60Pct: stat.SectorRelativeStrength60,
		ActiveReturn20Pct:           roundStat((m.StockReturn20 - m.BenchmarkReturn20) * 100),
		ActiveReturn60Pct:           roundStat((m.StockReturn60 - m.BenchmarkReturn60) * 100),
	}
	for _, factor := range stat.Factors {
		out.FactorExposures = append(out.FactorExposures, AdvancedFactorScore{
			Name:       factor.Name,
			Exposure:   factor.Exposure,
			Score:      roundStat(factor.Score),
			Status:     factor.Status,
			Evidence:   factor.Evidence,
			Confidence: factorConfidence(factor.Status, factor.Score),
		})
	}
	if !m.BenchmarkAvailable {
		out.MissingInputs = append(out.MissingInputs, "benchmark_ohlcv")
	}
	if !m.SectorBenchmarkAvailable {
		out.MissingInputs = append(out.MissingInputs, "sector_benchmark_ohlcv")
	}
	if !out.Computed {
		out.Warnings = append(out.Warnings, "factor_model_requires_quant_returns")
	}
	out.Score = roundStat(core.Clamp(out.Score-missingPenalty(out.MissingInputs), 0, 100))
	out.Status = scoreStatus(out.Score)
	if !out.Computed {
		out.Status = "missing"
	}
	return out
}

func buildAdvancedVolatilityRegime(result SymbolAnalysis, closes, returns []float64) AdvancedVolatilityRegime {
	regime := result.StatEconomic.Regime
	tail := result.StatEconomic.TailStress
	lastClose := closes[len(closes)-1]
	out := AdvancedVolatilityRegime{
		Computed:                   true,
		Score:                      weightedStatScore([]statScoreWeight{{regime.Score, 0.55}, {tail.Score, 0.45}}),
		Regime:                     regime.VolatilityRegime,
		TrendRegime:                regime.TrendRegime,
		EWMAVolatilityAnnualPct:    regime.EWMAVolatilityAnnualPct,
		GARCHVolatilityForecastPct: regime.GARCHVolatilityForecastPct,
		CornishFisherVaR95Pct:      roundStat(cornishFisherVaRPct(returns, 0.95)),
		EVTTailLossPct:             roundStat(evtTailLossPct(returns)),
		VolatilityClusteringScore:  regime.VolatilityClusteringScore,
		StressScenarios:            append([]StressScenario{}, tail.StressScenarios...),
	}
	for _, pct := range []float64{-out.CornishFisherVaR95Pct, -out.EVTTailLossPct, tail.StressWorstCaseReturnPct} {
		name := "bootstrap_proxy"
		if pct == tail.StressWorstCaseReturnPct {
			name = "stress_worst_case"
		}
		out.BootstrapScenarios = append(out.BootstrapScenarios, AdvancedStressSimulation{
			Name:      name,
			ReturnPct: roundStat(pct),
			Price:     roundStat(lastClose * (1 + pct/100)),
		})
	}
	if out.Regime == "high" || out.Regime == "extreme" {
		out.Warnings = append(out.Warnings, "high_volatility_regime_position_size_limit")
	}
	out.Score = roundStat(core.Clamp(out.Score, 0, 100))
	out.Status = scoreStatus(out.Score)
	return out
}

func buildAdvancedMacroModel(result SymbolAnalysis) AdvancedMacroModel {
	macro := result.StatEconomic.MacroSensitivity
	out := AdvancedMacroModel{
		Computed:                macro.Score > 0,
		Score:                   macro.Score,
		PointInTimeSafe:         macro.PointInTimeSafe,
		SectorMacroProfile:      macro.SectorMacroProfile,
		RateSensitivity:         macro.RateSensitivity,
		FXSensitivity:           macro.FXSensitivity,
		InflationSensitivity:    macro.InflationSensitivity,
		CyclicalSensitivity:     macro.CyclicalSensitivity,
		TCMBRegime:              macro.TCMBRegime,
		ForecastImpactDirection: macro.TCMBImpactDirection,
		ForecastImpactSeverity:  macro.TCMBImpactSeverity,
		Warnings:                append([]string{}, macro.Warnings...),
	}
	if !macro.TCMBReady {
		out.MissingInputs = append(out.MissingInputs, "tcmb_evds")
	}
	if !macro.PointInTimeSafe {
		out.MissingInputs = append(out.MissingInputs, "macro_available_at_lineage")
	}
	out.MacroStressScenarios = []AdvancedMacroScenario{
		{Name: "policy_rate_up", Shock: "+500bp politika faizi", ExpectedImpact: macroStressDirection(macro.RateSensitivity, "rate"), ReturnPct: macroStressReturn(macro.RateSensitivity, -6)},
		{Name: "try_depreciation", Shock: "TRY değer kaybı", ExpectedImpact: macroStressDirection(macro.FXSensitivity, "fx"), ReturnPct: macroStressReturn(macro.FXSensitivity, -5)},
		{Name: "inflation_surprise", Shock: "enflasyon sürprizi", ExpectedImpact: macroStressDirection(macro.InflationSensitivity, "inflation"), ReturnPct: macroStressReturn(macro.InflationSensitivity, -4)},
	}
	out.Score = roundStat(core.Clamp(out.Score-missingPenalty(out.MissingInputs), 0, 100))
	out.Status = scoreStatus(out.Score)
	if !out.Computed {
		out.Status = "missing"
	}
	return out
}

func buildAdvancedFinancialQuality(result SymbolAnalysis) AdvancedFinancialQuality {
	fq := result.StatEconomic.FinancialQuality
	valuation := result.Professional.Valuation
	out := AdvancedFinancialQuality{
		Computed:                 fq.Score > 0,
		Score:                    fq.Score,
		PiotroskiProxyScore:      roundStat(piotroskiProxy(result)),
		BeneishRiskProxy:         fq.ManipulationRiskProxy,
		AltmanRiskProxy:          altmanRiskProxy(valuation),
		DuPontROE:                roundStat(ratioValue(valuation.Ratios, "ROE") * 100),
		AccrualQualityProxyScore: fq.AccrualQualityProxyScore,
		EarningsPersistenceScore: fq.EarningsPersistenceScore,
		DebtSustainabilityScore:  roundStat(debtSustainabilityScore(valuation)),
		RedFlags:                 append([]string{}, fq.RedFlags...),
		Warnings:                 append([]string{}, fq.Warnings...),
	}
	for _, metric := range result.Professional.SectorFinancials.Metrics {
		out.SectorSpecificMetrics = append(out.SectorSpecificMetrics, AdvancedSectorQualityMetric{
			Name:     metric.Name,
			Value:    roundStat(metric.Value),
			Status:   metric.Status,
			Evidence: strings.Join(metric.SourceFields, ","),
		})
	}
	if valuation.Equity == 0 {
		out.MissingInputs = append(out.MissingInputs, "equity")
	}
	if valuation.NetIncomeTTM == 0 {
		out.MissingInputs = append(out.MissingInputs, "net_income_ttm")
	}
	if valuation.OperatingCashTTM == 0 && valuation.FreeCashFlowTTM == 0 {
		out.MissingInputs = append(out.MissingInputs, "cash_flow_ttm")
	}
	out.Score = roundStat(core.Clamp(weightedStatScore([]statScoreWeight{
		{fq.Score, 0.45},
		{out.PiotroskiProxyScore, 0.18},
		{out.DebtSustainabilityScore, 0.16},
		{out.AccrualQualityProxyScore, 0.11},
		{out.EarningsPersistenceScore, 0.10},
	})-missingPenalty(out.MissingInputs), 0, 100))
	if len(out.RedFlags) > 0 {
		out.Score = roundStat(core.Clamp(out.Score-float64(len(out.RedFlags))*4, 0, 100))
	}
	out.Status = scoreStatus(out.Score)
	if !out.Computed {
		out.Status = "missing"
	}
	return out
}

func buildAdvancedValuationEnsemble(result SymbolAnalysis, lastClose float64) AdvancedValuationEnsemble {
	val := result.Professional.Valuation
	bridge := result.Professional.InvestmentResearch.ValuationBridge
	current := firstPositive(lastClose, bridge.CurrentPrice, result.Quant.Return.LastClose)
	out := AdvancedValuationEnsemble{
		Computed:         result.Professional.ValueInvesting.Computed || val.FairValue.Base > 0 || bridge.BaseIntrinsicValue > 0,
		CurrentPrice:     roundStat(current),
		BearFairValue:    roundStat(firstPositive(val.FairValue.Bear, bridge.BearIntrinsicValue)),
		BaseFairValue:    roundStat(firstPositive(val.FairValue.Base, bridge.BaseIntrinsicValue)),
		BullFairValue:    roundStat(firstPositive(val.FairValue.Bull, bridge.BullIntrinsicValue)),
		ModelReliability: roundStat(firstPositive(result.Professional.ValueInvesting.Confidence, val.FairValue.Confidence)),
		MissingInputs:    append([]string{}, bridge.MissingInputs...),
		Warnings:         append([]string{}, result.Professional.ValueInvesting.Warnings...),
	}
	if current > 0 && out.BaseFairValue > 0 {
		out.ExpectedUpsidePct = roundStat((out.BaseFairValue/current - 1) * 100)
		out.MarginOfSafetyPct = roundStat((out.BaseFairValue - current) / out.BaseFairValue * 100)
	}
	if val.DCF.Computed {
		out.Models = append(out.Models, AdvancedValuationModel{Name: "dcf", Status: "computed", FairValue: roundStat(val.DCF.FairValuePerShare), Weight: 0.30, Confidence: out.ModelReliability, Evidence: val.DCF.AssumptionSource})
		for _, s := range val.DCF.Sensitivity {
			out.Sensitivity = append(out.Sensitivity, AdvancedValuationSensitivity{Name: s.Name, Variable: "wacc_terminal_growth", Change: fmt.Sprintf("wacc=%.2f terminal=%.2f", s.WACC, s.TerminalGrowth), FairValue: roundStat(s.FairValuePerShare)})
		}
	} else {
		out.Models = append(out.Models, AdvancedValuationModel{Name: "dcf", Status: "missing", Evidence: "free_cash_flow_and_assumptions_required"})
	}
	out.Models = append(out.Models,
		AdvancedValuationModel{Name: "fair_value_range", Status: computedStatus(out.BaseFairValue > 0), FairValue: out.BaseFairValue, Weight: 0.25, Confidence: val.FairValue.Confidence, Evidence: strings.Join(val.FairValue.Drivers, "; ")},
		AdvancedValuationModel{Name: "peer_multiples", Status: computedStatus(result.Professional.Peers.PeerCount >= 3), FairValue: out.BaseFairValue, Weight: 0.18, Confidence: peerConfidence(result.Professional.Peers.PeerCount), Evidence: result.Professional.Peers.ValuationSignal},
		AdvancedValuationModel{Name: "nav_or_sum_of_parts", Status: bridge.NAVStatus, FairValue: roundStat(bridge.NAVBridge.EstimatedNAVPerShare), Weight: 0.15, Confidence: navConfidence(bridge.NAVStatus), Evidence: bridge.NAVBridge.Status},
		AdvancedValuationModel{Name: "residual_income_proxy", Status: computedStatus(val.Equity > 0 && val.NetIncomeTTM != 0), FairValue: roundStat(residualIncomeProxy(val)), Weight: 0.12, Confidence: out.ModelReliability * 0.75, Evidence: "equity + roe proxy"},
	)
	if !out.Computed {
		out.MissingInputs = append(out.MissingInputs, "valuation_model_inputs")
	}
	score := out.ModelReliability
	if out.ExpectedUpsidePct > 20 {
		score += 8
	} else if out.ExpectedUpsidePct < -10 {
		score -= 12
	}
	score -= missingPenalty(out.MissingInputs)
	out.Score = roundStat(core.Clamp(score, 0, 100))
	out.Status = scoreStatus(out.Score)
	if !out.Computed {
		out.Status = "missing"
	}
	return out
}

func buildAdvancedEventStudy(result SymbolAnalysis, returns []float64) AdvancedEventStudy {
	out := AdvancedEventStudy{Score: 45}
	if result.Professional.RawKAPData == nil {
		out.Status = "missing"
		out.MissingInputs = []string{"kap_raw_data"}
		out.Warnings = []string{"kap_event_study_requires_raw_kap_data"}
		return out
	}
	raw := result.Professional.RawKAPData
	out.Computed = raw.Computed
	out.EventCount = len(raw.CorporateEvents) + len(raw.KAPEvents)
	out.TradableEventCount = result.Professional.FundamentalBacktest.TradableEvents
	out.MaterialityScore = roundStat(eventMaterialityScore(result))
	out.AbnormalReturnProxyPct = roundStat(result.Professional.Market.StockReturn20*100 - result.Professional.Market.BenchmarkReturn20*100)
	out.VolumeShiftProxyPct = roundStat(volumeShiftProxy(result))
	for _, event := range raw.CorporateEvents {
		if len(out.LatestEvents) >= 10 {
			break
		}
		out.LatestEvents = append(out.LatestEvents, AdvancedEventScore{
			Type:             event.EventType,
			Title:            event.Title,
			Period:           stringPtrValue(event.Period),
			MaterialityScore: eventMaterialityFromText(event.EventType + " " + event.Title),
			ExpectedImpact:   eventImpactFromText(event.EventType + " " + event.Title),
		})
	}
	score := 45.0 + core.Clamp(float64(out.EventCount), 0, 20) + core.Clamp(float64(out.TradableEventCount), 0, 20)
	score += out.MaterialityScore * 0.15
	if result.Professional.FundamentalBacktest.BacktestSafe {
		score += 10
	}
	if result.Professional.FundamentalBacktest.LookaheadViolations > 0 {
		score -= 25
		out.Warnings = append(out.Warnings, "event_backtest_lookahead_violations")
	}
	if len(returns) < 60 {
		out.Warnings = append(out.Warnings, "event_study_price_window_short")
	}
	out.Score = roundStat(core.Clamp(score, 0, 100))
	out.Status = scoreStatus(out.Score)
	if out.EventCount == 0 {
		out.Status = "missing"
		out.MissingInputs = append(out.MissingInputs, "kap_events")
	}
	return out
}

func buildAdvancedModelMonitoring(result SymbolAnalysis, tf TimeframeAnalysis) AdvancedModelMonitoring {
	bt := tf.Professional.Backtest
	forecast := result.NextSessionForecast
	out := AdvancedModelMonitoring{
		Computed:                    bt.Trades > 0 || forecast.BacktestMetrics.Samples > 0,
		ForecastBacktestSamples:     forecast.BacktestMetrics.Samples,
		ForecastDirectionHitRatePct: roundStat(forecast.BacktestMetrics.DirectionAccuracy),
		ForecastCloseMAEPct:         roundStat(forecast.BacktestMetrics.CloseMAPE),
		WalkForwardTrades:           bt.Trades,
		OutOfSampleTrades:           bt.OutOfSampleTrades,
		ModelRisk:                   result.StatEconomic.Validation.ModelRisk,
		ChampionModel:               "technical_quant_stat_economic_ensemble_v1",
		ChallengerModel:             "not_registered",
		DriftStatus:                 driftStatus(result),
		Warnings:                    append([]string{}, result.StatEconomic.Validation.Warnings...),
	}
	if forecast.BacktestMetrics.Samples == 0 {
		out.MissingInputs = append(out.MissingInputs, "forecast_backtest_history")
	}
	if bt.Trades < 30 {
		out.MissingInputs = append(out.MissingInputs, "walk_forward_trade_sample")
	}
	score := result.StatEconomic.Validation.Score
	if out.DriftStatus == "drift_watch" {
		score -= 10
	}
	score -= missingPenalty(out.MissingInputs)
	out.Score = roundStat(core.Clamp(score, 0, 100))
	out.Status = scoreStatus(out.Score)
	if !out.Computed {
		out.Status = "missing"
	}
	return out
}

func buildAdvancedLiquidityPortfolio(result SymbolAnalysis, tf TimeframeAnalysis) AdvancedLiquidityPortfolio {
	liq := result.StatEconomic.Liquidity
	out := AdvancedLiquidityPortfolio{
		Computed:                  liq.Score > 0,
		Score:                     liq.Score,
		ADV20TRY:                  liq.AverageValue20TRY,
		MedianValue20TRY:          liq.MedianValue20TRY,
		CapacityTRYAt10PctADV:     liq.CapacityTRYAt10PctADV,
		DaysToExit1MTRY:           liq.DaysToExit1MTRY,
		PortfolioVaR1DayPer100K:   result.Quant.Risk.RiskBudgetOneDayVaRPer100K,
		RecommendedMaxPositionTRY: roundStat(recommendedMaxPositionTRY(result, liq.CapacityTRYAt10PctADV)),
		Warnings:                  append([]string{}, liq.Warnings...),
	}
	if micro := result.Professional.Market.Microstructure; micro != nil {
		out.SpreadBps = roundStat(micro.OrderBook.SpreadBps)
		out.OrderBookImbalanceTop5 = roundStat(micro.OrderBook.ImbalanceTop5)
	}
	out.SlippageEstimateBps = roundStat(slippageEstimateBps(out.SpreadBps, liq.AmihudIlliquidity20, liq.AverageValue20TRY))
	out.MarketImpactFor1MTRYBps = roundStat(marketImpactBps(1_000_000, liq.AverageValue20TRY, out.SpreadBps))
	if result.Professional.Market.Microstructure == nil {
		out.MissingInputs = append(out.MissingInputs, "market_microstructure")
	}
	if tf.Professional.Liquidity.AverageValueTraded20TRY <= 0 {
		out.MissingInputs = append(out.MissingInputs, "average_value_traded_20")
	}
	out.Score = roundStat(core.Clamp(out.Score-missingPenalty(out.MissingInputs), 0, 100))
	if out.MarketImpactFor1MTRYBps > 100 {
		out.Score = roundStat(core.Clamp(out.Score-12, 0, 100))
		out.Warnings = append(out.Warnings, "market_impact_high_for_1m_try")
	}
	out.Status = scoreStatus(out.Score)
	if !out.Computed {
		out.Status = "missing"
	}
	return out
}

func buildAdvancedProductionReadiness(result SymbolAnalysis, advanced AdvancedAnalysis) AdvancedProductionReadiness {
	out := AdvancedProductionReadiness{
		Computed:             true,
		DataGate:             passLimitedFail(advanced.DataQuality.Score, 70, 50),
		ValidationGate:       passLimitedFail(advanced.ModelMonitoring.Score, 70, 50),
		RiskGate:             passLimitedFail(weightedStatScore([]statScoreWeight{{advanced.Volatility.Score, 0.5}, {result.Quant.Decision.RiskScore, 0.5}}), 65, 45),
		ValuationGate:        passLimitedFail(advanced.Valuation.Score, 65, 45),
		ExecutionGate:        passLimitedFail(advanced.LiquidityPortfolio.Score, 65, 45),
		ModelRegistryVersion: "advanced_equity_analysis_v1",
		ReportHash:           advancedReportHash(result),
	}
	out.AuditTrail = []string{
		"data:" + out.DataGate,
		"validation:" + out.ValidationGate,
		"risk:" + out.RiskGate,
		"valuation:" + out.ValuationGate,
		"execution:" + out.ExecutionGate,
	}
	gates := []string{out.DataGate, out.ValidationGate, out.RiskGate, out.ValuationGate, out.ExecutionGate}
	failCount, limitedCount := 0, 0
	for _, gate := range gates {
		if gate == "fail" {
			failCount++
		} else if gate == "limited" {
			limitedCount++
		}
	}
	switch {
	case failCount > 0:
		out.Status = "fail"
		out.Decision = "research_only"
	case limitedCount > 0:
		out.Status = "limited"
		out.Decision = "decision_with_limits"
	default:
		out.Status = "pass"
		out.Decision = "production_ready"
	}
	out.Score = roundStat(weightedStatScore([]statScoreWeight{
		{advanced.DataQuality.Score, 0.20},
		{advanced.ModelMonitoring.Score, 0.18},
		{advanced.Volatility.Score, 0.16},
		{advanced.Valuation.Score, 0.18},
		{advanced.LiquidityPortfolio.Score, 0.16},
		{advanced.PointInTime.Score, 0.12},
	}))
	if out.Status != "pass" {
		out.HumanReviewRequired = true
		if out.DataGate != "pass" {
			out.HumanReviewReasons = append(out.HumanReviewReasons, "data_gate_"+out.DataGate)
		}
		if out.ValidationGate != "pass" {
			out.HumanReviewReasons = append(out.HumanReviewReasons, "validation_gate_"+out.ValidationGate)
		}
		if out.RiskGate != "pass" {
			out.HumanReviewReasons = append(out.HumanReviewReasons, "risk_gate_"+out.RiskGate)
		}
		if out.ValuationGate != "pass" {
			out.HumanReviewReasons = append(out.HumanReviewReasons, "valuation_gate_"+out.ValuationGate)
		}
		if out.ExecutionGate != "pass" {
			out.HumanReviewReasons = append(out.HumanReviewReasons, "execution_gate_"+out.ExecutionGate)
		}
	}
	return out
}

func advancedPhaseStatuses(a AdvancedAnalysis) []AdvancedPhaseStatus {
	return []AdvancedPhaseStatus{
		advancedPhase("0_contract", a.Computed, a.Production.Score, a.Production.Status, nil, "JSON kontrati, artifact ve karar kapisi kapsami"),
		advancedPhase("1_data_point_in_time", a.DataQuality.Computed && a.PointInTime.Computed, weightedStatScore([]statScoreWeight{{a.DataQuality.Score, 0.5}, {a.PointInTime.Score, 0.5}}), worstStatus(a.DataQuality.Status, a.PointInTime.Status), append(a.DataQuality.MissingInputs, a.PointInTime.MissingPublishPeriods...), "veri guvenilirligi ve point-in-time zincir"),
		advancedPhase("2_bist_factor_model", a.FactorModel.Computed, a.FactorModel.Score, a.FactorModel.Status, a.FactorModel.MissingInputs, "piyasa/sektor/stil faktorleri"),
		advancedPhase("3_volatility_tail_risk", a.Volatility.Computed, a.Volatility.Score, a.Volatility.Status, nil, "rejim, GARCH/EWMA ve kuyruk riski"),
		advancedPhase("4_macro_sensitivity", a.Macro.Computed, a.Macro.Score, a.Macro.Status, a.Macro.MissingInputs, "TCMB/TUIK/makro duyarlilik"),
		advancedPhase("5_financial_quality", a.FinancialQuality.Computed, a.FinancialQuality.Score, a.FinancialQuality.Status, a.FinancialQuality.MissingInputs, "finansal kalite ve muhasebe riski"),
		advancedPhase("6_valuation_ensemble", a.Valuation.Computed, a.Valuation.Score, a.Valuation.Status, a.Valuation.MissingInputs, "DCF/peer/NAV/residual income ensemble"),
		advancedPhase("7_event_study", a.EventStudy.Computed, a.EventStudy.Score, a.EventStudy.Status, a.EventStudy.MissingInputs, "KAP event study ve haber etki modeli"),
		advancedPhase("8_model_monitoring", a.ModelMonitoring.Computed, a.ModelMonitoring.Score, a.ModelMonitoring.Status, a.ModelMonitoring.MissingInputs, "walk-forward, forecast audit ve drift"),
		advancedPhase("9_liquidity_portfolio", a.LiquidityPortfolio.Computed, a.LiquidityPortfolio.Score, a.LiquidityPortfolio.Status, a.LiquidityPortfolio.MissingInputs, "likidite, market impact ve portfoy kapasitesi"),
		advancedPhase("10_production_orchestration", a.Production.Computed, a.Production.Score, a.Production.Status, a.Production.HumanReviewReasons, "gate hiyerarsisi, audit trail ve human review"),
	}
}

func advancedPhase(phase string, computed bool, score float64, status string, missing []string, description string) AdvancedPhaseStatus {
	if status == "" {
		status = "missing"
	}
	next := []string{}
	if status != "pass" && status != "good" && status != "strong" {
		next = append(next, "eksik veri ve limited/fail kapilarini kapat")
	}
	return AdvancedPhaseStatus{
		Phase:       phase,
		Status:      normalizeAdvancedStatus(status),
		Score:       roundStat(score),
		Computed:    computed,
		Blocking:    normalizeAdvancedStatus(status) == "fail",
		Missing:     compactStrings(missing, 12),
		NextSteps:   next,
		Description: description,
	}
}

func normalizeAdvancedStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass", "good", "strong", "ready", "computed":
		return "pass"
	case "limited", "moderate", "watch", "warning", "fair":
		return "limited"
	case "missing", "not_computed", "":
		return "missing"
	default:
		if strings.Contains(status, "fail") || strings.Contains(status, "weak") || strings.Contains(status, "poor") {
			return "fail"
		}
		return status
	}
}

func worstStatus(values ...string) string {
	priority := map[string]int{"pass": 0, "limited": 1, "missing": 2, "fail": 3}
	worst := "pass"
	for _, value := range values {
		v := normalizeAdvancedStatus(value)
		if priority[v] > priority[worst] {
			worst = v
		}
	}
	return worst
}

func passLimitedFail(score, pass, limited float64) string {
	switch {
	case score >= pass:
		return "pass"
	case score >= limited:
		return "limited"
	default:
		return "fail"
	}
}

func missingPenalty(values []string) float64 {
	return math.Min(float64(len(values))*4, 24)
}

func factorConfidence(status string, score float64) float64 {
	if normalizeAdvancedStatus(status) != "pass" {
		return 35
	}
	return roundStat(core.Clamp(score, 30, 95))
}

func cornishFisherVaRPct(returns []float64, confidence float64) float64 {
	if len(returns) < 4 {
		return 0
	}
	mean := core.Mean(returns)
	std := core.StdDev(returns, true)
	if std <= 0 {
		return 0
	}
	skew := quantSkewness(returns)
	kurt := quantExcessKurtosis(returns)
	z := 1.6448536269514722
	if confidence >= 0.99 {
		z = 2.3263478740408408
	}
	zcf := z + (z*z-1)*skew/6 + (z*z*z-3*z)*kurt/24 - (2*z*z*z-5*z)*skew*skew/36
	return math.Max(0, -(mean-std*zcf)*100)
}

func evtTailLossPct(returns []float64) float64 {
	if len(returns) < 20 {
		return extremeTailLossPct(returns, 0.05)
	}
	losses := make([]float64, 0, len(returns))
	for _, r := range returns {
		if r < 0 {
			losses = append(losses, -r)
		}
	}
	if len(losses) == 0 {
		return 0
	}
	sort.Float64s(losses)
	thresholdIndex := int(float64(len(losses)) * 0.8)
	if thresholdIndex >= len(losses) {
		thresholdIndex = len(losses) - 1
	}
	threshold := losses[thresholdIndex]
	excess := []float64{}
	for _, loss := range losses {
		if loss >= threshold {
			excess = append(excess, loss-threshold)
		}
	}
	return roundStat((threshold + core.Mean(excess)*1.5) * 100)
}

func macroStressDirection(sensitivity, kind string) string {
	text := strings.ToLower(sensitivity)
	switch {
	case strings.Contains(text, "positive"), strings.Contains(text, "benefit"):
		return kind + "_tailwind"
	case strings.Contains(text, "high"), strings.Contains(text, "negative"), strings.Contains(text, "sensitive"):
		return kind + "_headwind"
	default:
		return kind + "_mixed"
	}
}

func macroStressReturn(sensitivity string, base float64) float64 {
	text := strings.ToLower(sensitivity)
	switch {
	case strings.Contains(text, "low"):
		return roundStat(base * 0.35)
	case strings.Contains(text, "high"), strings.Contains(text, "sensitive"):
		return roundStat(base * 1.25)
	case strings.Contains(text, "positive"), strings.Contains(text, "benefit"):
		return roundStat(math.Abs(base) * 0.5)
	default:
		return roundStat(base * 0.75)
	}
}

func piotroskiProxy(result SymbolAnalysis) float64 {
	val := result.Professional.Valuation
	score := 0.0
	checks := 0.0
	add := func(pass bool) {
		checks++
		if pass {
			score++
		}
	}
	add(val.NetIncomeTTM > 0)
	add(val.OperatingCashTTM > 0 || val.FreeCashFlowTTM > 0)
	add(val.OperatingCashTTM > val.NetIncomeTTM && val.NetIncomeTTM > 0)
	add(ratioValue(val.Ratios, "ROE") > 0.08)
	add(ratioValue(val.Ratios, "ROA") > 0.04)
	add(val.NetDebt <= val.Equity)
	add(ratioValue(val.Ratios, "gross_margin") > 0)
	add(result.Professional.ValueInvesting.CapitalAllocation.Score > 55)
	add(result.Professional.SectorFinancials.Score >= 60)
	return safeDiv(score, checks) * 100
}

func ratioValue(ratios map[string]float64, keys ...string) float64 {
	for _, key := range keys {
		for existing, value := range ratios {
			if strings.EqualFold(existing, key) {
				return value
			}
		}
	}
	return 0
}

func altmanRiskProxy(val professional.ValuationAnalysis) string {
	if val.TotalAssets <= 0 || val.Equity <= 0 {
		return "requires_full_balance_sheet"
	}
	debtRatio := safeDiv(val.TotalDebt, val.TotalAssets)
	roe := ratioValue(val.Ratios, "ROE", "roe")
	switch {
	case debtRatio < 0.35 && roe > 0.10:
		return "low_proxy"
	case debtRatio < 0.65 && roe > 0:
		return "moderate_proxy"
	default:
		return "high_proxy"
	}
}

func peerConfidence(peerCount int) float64 {
	switch {
	case peerCount >= 5:
		return 80
	case peerCount >= 3:
		return 65
	case peerCount > 0:
		return 40
	default:
		return 0
	}
}

func navConfidence(status string) float64 {
	switch normalizeAdvancedStatus(status) {
	case "pass":
		return 75
	case "limited":
		return 45
	default:
		return 0
	}
}

func debtSustainabilityScore(val professional.ValuationAnalysis) float64 {
	score := 55.0
	if val.DebtDataAvailable {
		score += 10
	}
	debtToEquity := safeDiv(val.TotalDebt, val.Equity)
	netDebtToEBITDA := safeDiv(val.NetDebt, val.EBITDATTM)
	switch {
	case debtToEquity > 2:
		score -= 25
	case debtToEquity > 1:
		score -= 12
	case debtToEquity > 0:
		score += 8
	}
	switch {
	case netDebtToEBITDA > 4:
		score -= 20
	case netDebtToEBITDA > 2.5:
		score -= 10
	case netDebtToEBITDA > 0:
		score += 8
	}
	if val.NetDebt <= 0 && val.Equity > 0 {
		score += 10
	}
	return core.Clamp(score, 0, 100)
}

func residualIncomeProxy(val professional.ValuationAnalysis) float64 {
	if val.Equity <= 0 || val.PaidCapital <= 0 {
		return 0
	}
	roe := ratioValue(val.Ratios, "ROE", "roe")
	if roe == 0 && val.NetIncomeTTM != 0 {
		roe = safeDiv(val.NetIncomeTTM, val.Equity)
	}
	requiredReturn := 0.22
	residual := val.Equity + (roe-requiredReturn)*val.Equity/requiredReturn
	return math.Max(0, residual/val.PaidCapital)
}

func eventMaterialityScore(result SymbolAnalysis) float64 {
	score := 0.0
	for _, flag := range result.Professional.Disclosure.RiskFlags {
		score = math.Max(score, eventMaterialityFromText(flag))
	}
	if result.Professional.NewsSentiment.Computed {
		score = math.Max(score, math.Abs(result.Professional.NewsSentiment.Score-50))
	}
	if result.Professional.FundamentalBacktest.TradableEvents > 0 {
		score = math.Max(score, 55)
	}
	return roundStat(core.Clamp(score, 0, 100))
}

func eventMaterialityFromText(text string) float64 {
	slug := strings.ToLower(text)
	score := 35.0
	for _, token := range []string{"ihale", "sozlesme", "contract", "temettu", "dividend", "sermaye", "bedelli", "bedelsiz", "buyback", "geri alim", "dava", "ceza", "regulatory"} {
		if strings.Contains(slug, token) {
			score += 12
		}
	}
	return roundStat(core.Clamp(score, 0, 100))
}

func eventImpactFromText(text string) string {
	slug := strings.ToLower(text)
	switch {
	case strings.Contains(slug, "temettu"), strings.Contains(slug, "dividend"), strings.Contains(slug, "geri alim"), strings.Contains(slug, "buyback"), strings.Contains(slug, "ihale"), strings.Contains(slug, "sozlesme"):
		return "potential_positive_or_cashflow_relevant"
	case strings.Contains(slug, "ceza"), strings.Contains(slug, "dava"), strings.Contains(slug, "risk"):
		return "potential_negative_or_risk_relevant"
	default:
		return "requires_manual_classification"
	}
}

func volumeShiftProxy(result SymbolAnalysis) float64 {
	tf, ok := result.Timeframes["1D"]
	if !ok || len(tf.Candles) < 25 {
		return 0
	}
	last := tf.Candles[len(tf.Candles)-1].EffectiveVolume()
	sum := 0.0
	for _, c := range tf.Candles[len(tf.Candles)-21 : len(tf.Candles)-1] {
		sum += c.EffectiveVolume()
	}
	avg := sum / 20
	return safeDiv(last-avg, avg) * 100
}

func driftStatus(result SymbolAnalysis) string {
	if result.StatEconomic.Validation.ModelRisk == "high" {
		return "drift_watch"
	}
	if result.NextSessionForecast.DirectionModelUnreliable {
		return "drift_watch"
	}
	if result.StatEconomic.DataIntegrity.ReturnOutliers > 0 {
		return "data_drift_watch"
	}
	return "stable"
}

func recommendedMaxPositionTRY(result SymbolAnalysis, capacity float64) float64 {
	candidates := []float64{}
	if capacity > 0 {
		candidates = append(candidates, capacity)
	}
	if result.Quant.Risk.RiskBudgetOneDayVaRPer100K > 0 {
		candidates = append(candidates, 100000*safeDiv(2500, result.Quant.Risk.RiskBudgetOneDayVaRPer100K))
	}
	min := 0.0
	for _, value := range candidates {
		if value <= 0 {
			continue
		}
		if min == 0 || value < min {
			min = value
		}
	}
	return min
}

func slippageEstimateBps(spreadBps, amihud, adv float64) float64 {
	base := spreadBps / 2
	if base <= 0 {
		base = 25
	}
	if adv > 0 {
		base += math.Min(75, safeDiv(1_000_000, adv)*35)
	}
	if amihud > 0 {
		base += math.Min(50, amihud*1e8)
	}
	return math.Max(0, base)
}

func marketImpactBps(orderTRY, advTRY, spreadBps float64) float64 {
	if advTRY <= 0 {
		return 0
	}
	return math.Max(0, spreadBps/2+math.Sqrt(orderTRY/advTRY)*65)
}

func advancedDecisionImpact(a AdvancedAnalysis) string {
	if a.Production.Status == "pass" {
		return "production_ready"
	}
	if a.Production.Status == "limited" {
		return "decision_allowed_with_limits"
	}
	return "research_only_or_human_review"
}

func advancedReportHash(result SymbolAnalysis) string {
	raw := strings.Join([]string{
		result.Symbol,
		result.AnalysisDate,
		fmt.Sprintf("%.4f", result.OverallScore),
		result.Quant.Status,
		result.StatEconomic.Status,
		result.Professional.DataGovernance.AvailabilityStatus,
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
