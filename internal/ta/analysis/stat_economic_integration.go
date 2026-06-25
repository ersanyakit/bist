package analysis

import (
	"math"
	"sort"
	"strings"
	"time"

	"hissebot/internal/quant/core"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
)

type StatEconomicAnalysis struct {
	Computed         bool                         `json:"computed"`
	Status           string                       `json:"status,omitempty"`
	SampleCount      int                          `json:"sample_count,omitempty"`
	CompositeScore   float64                      `json:"composite_score,omitempty"`
	DataIntegrity    StatisticalDataIntegrity     `json:"data_integrity"`
	FactorModel      StatisticalFactorModel       `json:"factor_model"`
	Regime           StatisticalRegimeModel       `json:"regime_model"`
	TailStress       StatisticalTailStress        `json:"tail_stress"`
	MacroSensitivity EconomicMacroSensitivity     `json:"macro_sensitivity"`
	FinancialQuality EconomicFinancialQuality     `json:"financial_quality"`
	Liquidity        EconomicLiquidityDiagnostics `json:"liquidity_diagnostics"`
	Validation       StatisticalValidation        `json:"validation"`
	Warnings         []string                     `json:"warnings,omitempty"`
}

type StatisticalDataIntegrity struct {
	Score                  float64  `json:"score"`
	CandleCount            int      `json:"candle_count"`
	DuplicateDates         int      `json:"duplicate_dates"`
	ChronologyViolations   int      `json:"chronology_violations"`
	PriceRuleViolations    int      `json:"price_rule_violations"`
	NonPositivePrices      int      `json:"non_positive_prices"`
	ReturnOutliers         int      `json:"return_outliers"`
	VolumeOutliers         int      `json:"volume_outliers"`
	LargeCalendarGaps      int      `json:"large_calendar_gaps"`
	AdjustedCandleRatioPct float64  `json:"adjusted_candle_ratio_pct"`
	Status                 string   `json:"status"`
	Warnings               []string `json:"warnings,omitempty"`
}

type StatisticalFactorModel struct {
	Score                    float64           `json:"score"`
	MarketBeta60             float64           `json:"market_beta_60,omitempty"`
	MarketAlpha60AnnualPct   float64           `json:"market_alpha_60_annual_pct,omitempty"`
	MarketCorrelation60      float64           `json:"market_correlation_60,omitempty"`
	RelativeStrength60Pct    float64           `json:"relative_strength_60_pct,omitempty"`
	SectorBeta60             float64           `json:"sector_beta_60,omitempty"`
	SectorAlpha60AnnualPct   float64           `json:"sector_alpha_60_annual_pct,omitempty"`
	SectorRelativeStrength60 float64           `json:"sector_relative_strength_60_pct,omitempty"`
	MomentumScore            float64           `json:"momentum_score"`
	QualityProxyScore        float64           `json:"quality_proxy_score"`
	ValueProxyScore          float64           `json:"value_proxy_score"`
	LowVolatilityScore       float64           `json:"low_volatility_score"`
	LiquidityScore           float64           `json:"liquidity_score"`
	Factors                  []StatFactorScore `json:"factors,omitempty"`
}

type StatFactorScore struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	Exposure float64 `json:"exposure,omitempty"`
	Status   string  `json:"status"`
	Evidence string  `json:"evidence,omitempty"`
}

type StatisticalRegimeModel struct {
	Score                      float64 `json:"score"`
	TrendRegime                string  `json:"trend_regime,omitempty"`
	VolatilityRegime           string  `json:"volatility_regime,omitempty"`
	DrawdownRegime             string  `json:"drawdown_regime,omitempty"`
	EWMAVolatilityAnnualPct    float64 `json:"ewma_volatility_annual_pct,omitempty"`
	GARCHVolatilityForecastPct float64 `json:"garch_volatility_forecast_pct,omitempty"`
	VolatilityClusteringScore  float64 `json:"volatility_clustering_score,omitempty"`
	NextPositiveProbabilityPct float64 `json:"next_positive_probability_pct,omitempty"`
	LastReturnState            string  `json:"last_return_state,omitempty"`
	TransitionSampleCount      int     `json:"transition_sample_count,omitempty"`
}

type StatisticalTailStress struct {
	Score                    float64          `json:"score"`
	VaR95Pct                 float64          `json:"var_95_pct,omitempty"`
	CVaR95Pct                float64          `json:"cvar_95_pct,omitempty"`
	ExtremeTailLossPct       float64          `json:"extreme_tail_loss_pct,omitempty"`
	WorstSessionLossPct      float64          `json:"worst_session_loss_pct,omitempty"`
	MaxDrawdownLossPct       float64          `json:"max_drawdown_loss_pct,omitempty"`
	StressScenarios          []StressScenario `json:"stress_scenarios,omitempty"`
	StressWorstCaseReturnPct float64          `json:"stress_worst_case_return_pct,omitempty"`
}

type StressScenario struct {
	Name      string  `json:"name"`
	ReturnPct float64 `json:"return_pct"`
	Price     float64 `json:"price,omitempty"`
	Driver    string  `json:"driver,omitempty"`
}

type EconomicMacroSensitivity struct {
	Score                float64  `json:"score"`
	TCMBReady            bool     `json:"tcmb_ready"`
	PointInTimeSafe      bool     `json:"point_in_time_safe"`
	TCMBRegime           string   `json:"tcmb_regime,omitempty"`
	TCMBImpactDirection  string   `json:"tcmb_impact_direction,omitempty"`
	TCMBImpactSeverity   string   `json:"tcmb_impact_severity,omitempty"`
	GDPScore             float64  `json:"gdp_score,omitempty"`
	RateSensitivity      string   `json:"rate_sensitivity,omitempty"`
	FXSensitivity        string   `json:"fx_sensitivity,omitempty"`
	InflationSensitivity string   `json:"inflation_sensitivity,omitempty"`
	CyclicalSensitivity  string   `json:"cyclical_sensitivity,omitempty"`
	SectorMacroProfile   string   `json:"sector_macro_profile,omitempty"`
	Warnings             []string `json:"warnings,omitempty"`
}

type EconomicFinancialQuality struct {
	Score                    float64  `json:"score"`
	DataGovernanceScore      float64  `json:"data_governance_score"`
	ValueInvestingScore      float64  `json:"value_investing_score,omitempty"`
	SectorFinancialScore     float64  `json:"sector_financial_score,omitempty"`
	MoatScore                float64  `json:"moat_score,omitempty"`
	CapitalAllocationScore   float64  `json:"capital_allocation_score,omitempty"`
	AccrualQualityProxyScore float64  `json:"accrual_quality_proxy_score,omitempty"`
	EarningsPersistenceScore float64  `json:"earnings_persistence_score,omitempty"`
	RestatementRisk          string   `json:"restatement_risk,omitempty"`
	ManipulationRiskProxy    string   `json:"manipulation_risk_proxy,omitempty"`
	AltmanPiotroskiStatus    string   `json:"altman_piotroski_status,omitempty"`
	RedFlags                 []string `json:"red_flags,omitempty"`
	Warnings                 []string `json:"warnings,omitempty"`
}

type EconomicLiquidityDiagnostics struct {
	Score                   float64  `json:"score"`
	AverageValue20TRY       float64  `json:"average_value_20_try,omitempty"`
	MedianValue20TRY        float64  `json:"median_value_20_try,omitempty"`
	AmihudIlliquidity20     float64  `json:"amihud_illiquidity_20,omitempty"`
	CapacityTRYAt10PctADV   float64  `json:"capacity_try_at_10pct_adv,omitempty"`
	DaysToExit1MTRY         float64  `json:"days_to_exit_1m_try,omitempty"`
	VolumeVsAverage20       float64  `json:"volume_vs_average_20,omitempty"`
	MicrostructureAvailable bool     `json:"microstructure_available"`
	Status                  string   `json:"status"`
	Warnings                []string `json:"warnings,omitempty"`
}

type StatisticalValidation struct {
	Score                          float64  `json:"score"`
	BacktestSafe                   bool     `json:"backtest_safe"`
	TechnicalWalkForwardTrades     int      `json:"technical_walk_forward_trades,omitempty"`
	TechnicalOutOfSampleTrades     int      `json:"technical_out_of_sample_trades,omitempty"`
	TechnicalExpectancyPct         float64  `json:"technical_expectancy_pct,omitempty"`
	FundamentalEventBacktestSafe   bool     `json:"fundamental_event_backtest_safe"`
	FundamentalTradableEvents      int      `json:"fundamental_tradable_events,omitempty"`
	FundamentalLookaheadViolations int      `json:"fundamental_lookahead_violations,omitempty"`
	ForecastBacktestSamples        int      `json:"forecast_backtest_samples,omitempty"`
	ForecastDirectionHitRatePct    float64  `json:"forecast_direction_hit_rate_pct,omitempty"`
	ForecastCloseMAEPct            float64  `json:"forecast_close_mae_pct,omitempty"`
	ModelRisk                      string   `json:"model_risk"`
	Warnings                       []string `json:"warnings,omitempty"`
}

func BuildStatEconomicAnalysis(result SymbolAnalysis) StatEconomicAnalysis {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		tf, ok = selectFinTradeBenchTimeframe(result.Timeframes)
	}
	out := StatEconomicAnalysis{Status: "not_computed"}
	if !ok || len(tf.Candles) < 3 {
		out.Warnings = append(out.Warnings, "stat_economic_requires_price_history")
		return out
	}
	closes := quantEffectiveCloses(tf.Candles)
	returns := quantFiniteReturns(core.Returns(closes))
	if len(returns) < 2 {
		out.Warnings = append(out.Warnings, "stat_economic_requires_valid_returns")
		return out
	}
	out.Computed = true
	out.Status = "computed"
	out.SampleCount = len(returns)
	out.DataIntegrity = buildStatDataIntegrity(tf.Candles, returns, result)
	out.FactorModel = buildStatFactorModel(result)
	out.Regime = buildStatRegimeModel(closes, returns, result)
	out.TailStress = buildStatTailStress(closes, returns, result)
	out.MacroSensitivity = buildMacroSensitivity(result)
	out.FinancialQuality = buildEconomicFinancialQuality(result)
	out.Liquidity = buildEconomicLiquidity(result, tf)
	out.Validation = buildStatValidation(result, tf)
	out.CompositeScore = roundStat(weightedStatScore([]statScoreWeight{
		{out.DataIntegrity.Score, 0.18},
		{out.FactorModel.Score, 0.13},
		{out.Regime.Score, 0.12},
		{out.TailStress.Score, 0.12},
		{out.MacroSensitivity.Score, 0.13},
		{out.FinancialQuality.Score, 0.16},
		{out.Liquidity.Score, 0.08},
		{out.Validation.Score, 0.08},
	}))
	out.Warnings = statCompactWarnings(
		out.DataIntegrity.Warnings,
		out.MacroSensitivity.Warnings,
		out.FinancialQuality.Warnings,
		out.Liquidity.Warnings,
		out.Validation.Warnings,
	)
	return out
}

func buildStatDataIntegrity(candles []ohlcv.Candle, returns []float64, result SymbolAnalysis) StatisticalDataIntegrity {
	out := StatisticalDataIntegrity{CandleCount: len(candles), Score: 100}
	seen := map[string]bool{}
	adjusted := 0
	for i, candle := range candles {
		if candle.IsAdjusted {
			adjusted++
		}
		date := candle.Time.Format("2006-01-02")
		if seen[date] {
			out.DuplicateDates++
		}
		seen[date] = true
		if i > 0 {
			if !candle.Time.After(candles[i-1].Time) {
				out.ChronologyViolations++
			}
			if candle.Time.Sub(candles[i-1].Time) > 10*24*time.Hour {
				out.LargeCalendarGaps++
			}
		}
		open, high, low, close := candle.EffectiveOpen(), candle.EffectiveHigh(), candle.EffectiveLow(), candle.EffectiveClose()
		if open <= 0 || high <= 0 || low <= 0 || close <= 0 {
			out.NonPositivePrices++
		}
		if high+1e-9 < math.Max(open, close) || low-1e-9 > math.Min(open, close) || high < low {
			out.PriceRuleViolations++
		}
	}
	out.AdjustedCandleRatioPct = roundStat(100 * safeDiv(float64(adjusted), float64(len(candles))))
	out.ReturnOutliers = countReturnOutliers(returns)
	out.VolumeOutliers = countVolumeOutliers(candles)
	out.Score -= float64(out.DuplicateDates) * 12
	out.Score -= float64(out.ChronologyViolations) * 15
	out.Score -= float64(out.PriceRuleViolations) * 12
	out.Score -= float64(out.NonPositivePrices) * 18
	out.Score -= float64(out.ReturnOutliers) * 3
	out.Score -= float64(out.VolumeOutliers) * 1.5
	out.Score -= float64(out.LargeCalendarGaps) * 2
	if !result.Professional.DataGovernance.BacktestSafe && result.Professional.DataGovernance.AvailabilityStatus != "" {
		out.Score -= 12
		out.Warnings = append(out.Warnings, "financial_data_not_backtest_safe")
	}
	out.Score = roundStat(core.Clamp(out.Score, 0, 100))
	out.Status = scoreStatus(out.Score)
	if out.DuplicateDates > 0 {
		out.Warnings = append(out.Warnings, "duplicate_candle_dates")
	}
	if out.PriceRuleViolations > 0 || out.NonPositivePrices > 0 {
		out.Warnings = append(out.Warnings, "ohlcv_price_rule_violations")
	}
	if out.ReturnOutliers > 0 {
		out.Warnings = append(out.Warnings, "return_outliers_detected")
	}
	return out
}

func buildStatFactorModel(result SymbolAnalysis) StatisticalFactorModel {
	q := result.Quant
	m := result.Professional.Market
	liquidityScore := liquidityScoreFromDaily(result)
	qualityScore := firstPositive(result.Professional.ValueInvesting.QualityScore, result.Professional.SectorFinancials.Score, result.Professional.DataQuality)
	valueScore := firstPositive(result.Professional.ValueInvesting.ValueScore, result.Professional.Valuation.FairValue.Confidence)
	momentumScore := core.Clamp(50+q.Return.Return60DPct*0.9+q.Return.Return20DPct*0.45, 0, 100)
	lowVolScore := statLowVolatilityScore(result.AssetType, q.Risk.AnnualizedVolatilityPct)
	out := StatisticalFactorModel{
		MarketBeta60:             roundStat(m.Beta60),
		MarketAlpha60AnnualPct:   roundStat(m.Alpha60 * 100),
		MarketCorrelation60:      roundStat(m.Correlation60),
		RelativeStrength60Pct:    roundStat(m.RelativeStrength60 * 100),
		SectorBeta60:             roundStat(m.SectorBeta60),
		SectorAlpha60AnnualPct:   roundStat(m.SectorAlpha60 * 100),
		SectorRelativeStrength60: roundStat(m.SectorRelativeStrength60 * 100),
		MomentumScore:            roundStat(momentumScore),
		QualityProxyScore:        roundStat(qualityScore),
		ValueProxyScore:          roundStat(valueScore),
		LowVolatilityScore:       roundStat(lowVolScore),
		LiquidityScore:           roundStat(liquidityScore),
	}
	out.Factors = []StatFactorScore{
		{Name: "market_beta", Score: scoreBeta(m.Beta60), Exposure: roundStat(m.Beta60), Status: factorStatus(result.Professional.Market.BenchmarkAvailable), Evidence: "BIST benchmark beta/alpha/correlation"},
		{Name: "sector_relative_strength", Score: core.Clamp(50+m.SectorRelativeStrength60*700, 0, 100), Exposure: roundStat(m.SectorRelativeStrength60 * 100), Status: factorStatus(m.SectorBenchmarkAvailable), Evidence: "sector benchmark relative return"},
		{Name: "momentum", Score: out.MomentumScore, Exposure: q.Return.Return60DPct, Status: "computed", Evidence: "20/60 day total return"},
		{Name: "quality", Score: out.QualityProxyScore, Status: computedStatus(out.QualityProxyScore > 0), Evidence: "value/sector/professional quality proxy"},
		{Name: "value", Score: out.ValueProxyScore, Status: computedStatus(out.ValueProxyScore > 0), Evidence: "margin of safety and fair value proxy"},
		{Name: "low_volatility", Score: out.LowVolatilityScore, Exposure: q.Risk.AnnualizedVolatilityPct, Status: "computed", Evidence: "annualized realized volatility"},
		{Name: "liquidity", Score: out.LiquidityScore, Status: computedStatus(out.LiquidityScore > 0), Evidence: "ADV, Amihud and capacity proxy"},
	}
	out.Score = roundStat(weightedStatScore([]statScoreWeight{
		{out.MomentumScore, 0.18},
		{out.QualityProxyScore, 0.22},
		{out.ValueProxyScore, 0.16},
		{out.LowVolatilityScore, 0.16},
		{out.LiquidityScore, 0.14},
		{scoreRelativeStrength(m.RelativeStrength60), 0.14},
	}))
	return out
}

func statLowVolatilityScore(assetType string, annualizedVolPct float64) float64 {
	baseline := 18.0
	penaltySlope := 1.5
	if ohlcv.IsCryptoAssetType(assetType) {
		baseline = 65
		penaltySlope = 0.55
	}
	return core.Clamp(100-(annualizedVolPct-baseline)*penaltySlope, 0, 100)
}

func buildStatRegimeModel(closes, returns []float64, result SymbolAnalysis) StatisticalRegimeModel {
	annualizationDays := quantAnnualizationDays(result.AssetType)
	ewmaVol := ewmaVolatility(returns, 0.94) * math.Sqrt(annualizationDays) * 100
	garchVol := garchForecastVolatility(returns) * math.Sqrt(annualizationDays) * 100
	volClustering := squaredReturnAutocorrelation(returns)
	nextProb, lastState, transitions := markovNextPositiveProbability(returns)
	trend := trendRegime(closes)
	drawdown := drawdownRegime(result.Quant.Risk.CurrentDrawdownLossPct)
	volRegime := quantVolatilityRegimeForAsset(result.AssetType, firstPositive(garchVol, ewmaVol, result.Quant.Risk.AnnualizedVolatilityPct))
	score := 65.0
	if trend == "uptrend" {
		score += 12
	} else if trend == "downtrend" {
		score -= 15
	}
	if volRegime == "high" {
		score -= 10
	} else if volRegime == "extreme" {
		score -= 20
	}
	if strings.Contains(drawdown, "deep") {
		score -= 12
	}
	score += (nextProb - 50) * 0.25
	return StatisticalRegimeModel{
		Score:                      roundStat(core.Clamp(score, 0, 100)),
		TrendRegime:                trend,
		VolatilityRegime:           volRegime,
		DrawdownRegime:             drawdown,
		EWMAVolatilityAnnualPct:    roundStat(ewmaVol),
		GARCHVolatilityForecastPct: roundStat(garchVol),
		VolatilityClusteringScore:  roundStat(core.Clamp(volClustering*100, 0, 100)),
		NextPositiveProbabilityPct: roundStat(nextProb),
		LastReturnState:            lastState,
		TransitionSampleCount:      transitions,
	}
}

func buildStatTailStress(closes, returns []float64, result SymbolAnalysis) StatisticalTailStress {
	q := result.Quant
	tailLoss := extremeTailLossPct(returns, 0.05)
	worst := worstSessionLossPct(returns)
	lastClose := closes[len(closes)-1]
	beta := q.Benchmark.Beta60
	if beta == 0 {
		beta = 1
	}
	sectorShock := sectorStressShockPct(result.Professional.Company.Sector, result.Professional.Company.Industry, result.Professional.Company.PeerGroup)
	scenarios := []StressScenario{
		stressScenario("market_down_5pct", -5*beta, lastClose, "benchmark shock scaled by beta"),
		stressScenario("market_down_10pct", -10*beta, lastClose, "benchmark shock scaled by beta"),
		stressScenario("sector_specific_shock", sectorShock, lastClose, "sector macro/liquidity stress proxy"),
		stressScenario("tail_loss_replay", -tailLoss, lastClose, "historical extreme tail loss"),
	}
	worstStress := 0.0
	for _, s := range scenarios {
		if s.ReturnPct < worstStress {
			worstStress = s.ReturnPct
		}
	}
	score := 100.0
	score -= core.Clamp((q.Risk.HistoricalVaR95Pct-2.5)*8, 0, 25)
	score -= core.Clamp((q.Risk.MaxDrawdownLossPct-20)*0.8, 0, 25)
	score -= core.Clamp((tailLoss-5)*5, 0, 25)
	return StatisticalTailStress{
		Score:                    roundStat(core.Clamp(score, 0, 100)),
		VaR95Pct:                 q.Risk.HistoricalVaR95Pct,
		CVaR95Pct:                q.Risk.HistoricalCVaR95Pct,
		ExtremeTailLossPct:       roundStat(tailLoss),
		WorstSessionLossPct:      roundStat(worst),
		MaxDrawdownLossPct:       q.Risk.MaxDrawdownLossPct,
		StressScenarios:          scenarios,
		StressWorstCaseReturnPct: roundStat(worstStress),
	}
}

func buildMacroSensitivity(result SymbolAnalysis) EconomicMacroSensitivity {
	pro := result.Professional
	sectorText := strings.ToLower(strings.Join([]string{pro.Company.Sector, pro.Company.Industry, pro.Company.PeerGroup}, " "))
	out := EconomicMacroSensitivity{
		TCMBReady:            pro.TCMBEVDSContext.AnalysisReady,
		PointInTimeSafe:      pro.TCMBEVDSContext.PointInTimeSafe,
		TCMBRegime:           pro.TCMBEVDSContext.Regime,
		TCMBImpactDirection:  pro.TCMBEVDSContext.ForecastImpact.Direction,
		TCMBImpactSeverity:   pro.TCMBEVDSContext.ForecastImpact.Severity,
		GDPScore:             pro.Market.GDP.Score,
		RateSensitivity:      classifyRateSensitivity(sectorText),
		FXSensitivity:        classifyFXSensitivity(sectorText),
		InflationSensitivity: classifyInflationSensitivity(sectorText),
		CyclicalSensitivity:  classifyCyclicalSensitivity(sectorText),
		SectorMacroProfile:   sectorMacroProfile(sectorText),
		Warnings:             append([]string{}, pro.TCMBEVDSContext.Warnings...),
	}
	score := 60.0
	if out.TCMBReady {
		score += 15
	} else {
		score -= 12
		out.Warnings = append(out.Warnings, "tcmb_evds_not_analysis_ready")
	}
	if out.PointInTimeSafe {
		score += 8
	} else if pro.TCMBEVDSContext.Computed {
		score -= 10
		out.Warnings = append(out.Warnings, "macro_not_point_in_time_safe")
	}
	if pro.TCMBEVDSContext.ForecastImpact.DecisionUse == "blocking_headwind" {
		score -= 18
	}
	if pro.Market.GDP.Score > 0 {
		score = score*0.75 + pro.Market.GDP.Score*0.25
	}
	out.Score = roundStat(core.Clamp(score, 0, 100))
	return out
}

func buildEconomicFinancialQuality(result SymbolAnalysis) EconomicFinancialQuality {
	pro := result.Professional
	govScore := dataGovernanceScore(pro.DataGovernance)
	valueScore := firstPositive(pro.ValueInvesting.QualityScore, pro.ValueInvesting.ValueScore)
	out := EconomicFinancialQuality{
		DataGovernanceScore:      roundStat(govScore),
		ValueInvestingScore:      roundStat(valueScore),
		SectorFinancialScore:     roundStat(pro.SectorFinancials.Score),
		MoatScore:                roundStat(pro.ValueInvesting.Moat.Score),
		CapitalAllocationScore:   roundStat(pro.ValueInvesting.CapitalAllocation.Score),
		AccrualQualityProxyScore: roundStat(accrualQualityProxy(pro)),
		EarningsPersistenceScore: roundStat(earningsPersistenceProxy(pro)),
		RestatementRisk:          restatementRisk(pro.DataGovernance.RestatementCount),
		ManipulationRiskProxy:    manipulationRiskProxy(pro.InvestmentResearch.FinancialQuality.RedFlags, pro.DataGovernance),
		AltmanPiotroskiStatus:    "requires_balance_sheet_fields",
		RedFlags:                 append([]string{}, pro.InvestmentResearch.FinancialQuality.RedFlags...),
		Warnings:                 append([]string{}, pro.DataGovernance.Warnings...),
	}
	out.Warnings = append(out.Warnings, pro.ValueInvesting.Warnings...)
	out.Score = roundStat(weightedStatScore([]statScoreWeight{
		{out.DataGovernanceScore, 0.24},
		{out.ValueInvestingScore, 0.22},
		{out.SectorFinancialScore, 0.18},
		{out.MoatScore, 0.14},
		{out.CapitalAllocationScore, 0.12},
		{out.AccrualQualityProxyScore, 0.05},
		{out.EarningsPersistenceScore, 0.05},
	}))
	if len(out.RedFlags) > 0 {
		out.Score = roundStat(core.Clamp(out.Score-float64(len(out.RedFlags))*4, 0, 100))
	}
	return out
}

func buildEconomicLiquidity(result SymbolAnalysis, tf TimeframeAnalysis) EconomicLiquidityDiagnostics {
	liq := tf.Professional.Liquidity
	score := liquidityScore(liq)
	out := EconomicLiquidityDiagnostics{
		Score:                   roundStat(score),
		AverageValue20TRY:       roundStat(liq.AverageValueTraded20TRY),
		MedianValue20TRY:        roundStat(liq.MedianValueTraded20TRY),
		AmihudIlliquidity20:     liq.AmihudIlliquidity20,
		CapacityTRYAt10PctADV:   roundStat(liq.CapacityTRYAt10PctADV),
		DaysToExit1MTRY:         roundStat(liq.DaysToExit1MTRY),
		VolumeVsAverage20:       roundStat(liq.VolumeVsAverage20),
		MicrostructureAvailable: result.Professional.Market.Microstructure != nil,
		Status:                  scoreStatus(score),
		Warnings:                append([]string{}, liq.Warnings...),
	}
	if liq.AverageValueTraded20TRY > 0 && liq.AverageValueTraded20TRY < 5_000_000 {
		out.Warnings = append(out.Warnings, "low_average_value_traded")
	}
	if !out.MicrostructureAvailable {
		out.Warnings = append(out.Warnings, "live_microstructure_missing")
	}
	return out
}

func buildStatValidation(result SymbolAnalysis, tf TimeframeAnalysis) StatisticalValidation {
	bt := tf.Professional.Backtest
	fbt := result.Professional.FundamentalBacktest
	forecast := result.NextSessionForecast
	score := 50.0
	if bt.BacktestSafe {
		score += 12
	}
	if bt.Trades >= 30 {
		score += 10
	}
	if bt.OutOfSampleTrades >= 10 {
		score += 8
	}
	if bt.Expectancy > 0 {
		score += 8
	}
	if fbt.BacktestSafe {
		score += 8
	}
	if fbt.LookaheadViolations > 0 {
		score -= 20
	}
	if forecast.BacktestMetrics.Samples > 0 {
		score += 6
		if forecast.BacktestMetrics.DirectionAccuracy < 50 {
			score -= 8
		}
		if forecast.BacktestMetrics.CloseMAPE > 2 {
			score -= 8
		}
	}
	out := StatisticalValidation{
		Score:                          roundStat(core.Clamp(score, 0, 100)),
		BacktestSafe:                   bt.BacktestSafe && fbt.BacktestSafe && fbt.LookaheadViolations == 0,
		TechnicalWalkForwardTrades:     bt.Trades,
		TechnicalOutOfSampleTrades:     bt.OutOfSampleTrades,
		TechnicalExpectancyPct:         roundStat(bt.Expectancy * 100),
		FundamentalEventBacktestSafe:   fbt.BacktestSafe,
		FundamentalTradableEvents:      fbt.TradableEvents,
		FundamentalLookaheadViolations: fbt.LookaheadViolations,
		ForecastBacktestSamples:        forecast.BacktestMetrics.Samples,
		ForecastDirectionHitRatePct:    roundStat(forecast.BacktestMetrics.DirectionAccuracy),
		ForecastCloseMAEPct:            roundStat(forecast.BacktestMetrics.CloseMAPE),
	}
	switch {
	case out.Score >= 75 && out.BacktestSafe:
		out.ModelRisk = "controlled"
	case out.Score >= 55:
		out.ModelRisk = "moderate"
	default:
		out.ModelRisk = "high"
	}
	out.Warnings = append(out.Warnings, fbt.Warnings...)
	if bt.LookaheadViolations > 0 {
		out.Warnings = append(out.Warnings, "technical_backtest_lookahead_violations")
	}
	if forecast.DirectionModelUnreliable {
		out.Warnings = append(out.Warnings, "next_session_direction_model_unreliable")
	}
	return out
}

type statScoreWeight struct {
	score  float64
	weight float64
}

func weightedStatScore(items []statScoreWeight) float64 {
	total, weight := 0.0, 0.0
	for _, item := range items {
		if item.score <= 0 || item.weight <= 0 {
			continue
		}
		total += item.score * item.weight
		weight += item.weight
	}
	return safeDiv(total, weight)
}

func countReturnOutliers(returns []float64) int {
	if len(returns) < 10 {
		return 0
	}
	median := core.Quantile(returns, 0.5)
	dev := make([]float64, len(returns))
	for i, r := range returns {
		dev[i] = math.Abs(r - median)
	}
	mad := core.Quantile(dev, 0.5)
	threshold := math.Max(0.12, 6*1.4826*mad)
	count := 0
	for _, r := range returns {
		if math.Abs(r-median) > threshold {
			count++
		}
	}
	return count
}

func countVolumeOutliers(candles []ohlcv.Candle) int {
	if len(candles) < 10 {
		return 0
	}
	vols := make([]float64, 0, len(candles))
	for _, c := range candles {
		if v := c.EffectiveVolume(); v > 0 {
			vols = append(vols, v)
		}
	}
	if len(vols) < 10 {
		return 0
	}
	median := core.Quantile(vols, 0.5)
	if median <= 0 {
		return 0
	}
	count := 0
	for _, v := range vols {
		if v > median*6 {
			count++
		}
	}
	return count
}

func ewmaVolatility(returns []float64, lambda float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	variance := core.Variance(returns, true)
	for _, r := range returns {
		variance = lambda*variance + (1-lambda)*r*r
	}
	return math.Sqrt(math.Max(variance, 0))
}

func garchForecastVolatility(returns []float64) float64 {
	if len(returns) < 20 {
		return ewmaVolatility(returns, 0.94)
	}
	baseVar := core.Variance(returns, true)
	bestLL := math.Inf(1)
	bestAlpha, bestBeta := 0.08, 0.88
	for alpha := 0.04; alpha <= 0.18; alpha += 0.02 {
		for beta := 0.70; beta <= 0.94; beta += 0.03 {
			if alpha+beta >= 0.995 {
				continue
			}
			omega := baseVar * (1 - alpha - beta)
			h := baseVar
			ll := 0.0
			for _, r := range returns {
				h = math.Max(omega+alpha*r*r+beta*h, 1e-12)
				ll += math.Log(h) + r*r/h
			}
			if ll < bestLL {
				bestLL, bestAlpha, bestBeta = ll, alpha, beta
			}
		}
	}
	omega := baseVar * (1 - bestAlpha - bestBeta)
	h := baseVar
	for _, r := range returns {
		h = math.Max(omega+bestAlpha*r*r+bestBeta*h, 1e-12)
	}
	last := returns[len(returns)-1]
	forecast := math.Max(omega+bestAlpha*last*last+bestBeta*h, 1e-12)
	return math.Sqrt(forecast)
}

func squaredReturnAutocorrelation(returns []float64) float64 {
	if len(returns) < 3 {
		return 0
	}
	a := make([]float64, len(returns)-1)
	b := make([]float64, len(returns)-1)
	for i := 1; i < len(returns); i++ {
		a[i-1] = returns[i-1] * returns[i-1]
		b[i-1] = returns[i] * returns[i]
	}
	return math.Max(0, core.Correlation(a, b))
}

func markovNextPositiveProbability(returns []float64) (float64, string, int) {
	if len(returns) < 3 {
		return 50, "unknown", 0
	}
	state := func(v float64) int {
		if v >= 0 {
			return 1
		}
		return 0
	}
	counts := [2][2]int{}
	for i := 1; i < len(returns); i++ {
		counts[state(returns[i-1])][state(returns[i])]++
	}
	last := state(returns[len(returns)-1])
	total := counts[last][0] + counts[last][1]
	label := "negative"
	if last == 1 {
		label = "positive"
	}
	if total == 0 {
		return 50, label, 0
	}
	return 100 * float64(counts[last][1]) / float64(total), label, total
}

func trendRegime(closes []float64) string {
	if len(closes) < 20 {
		return "unknown"
	}
	last := closes[len(closes)-1]
	sma20 := meanLast(closes, 20)
	sma60 := meanLast(closes, 60)
	switch {
	case last > sma20 && (sma60 == 0 || sma20 > sma60):
		return "uptrend"
	case last < sma20 && (sma60 == 0 || sma20 < sma60):
		return "downtrend"
	default:
		return "range"
	}
}

func drawdownRegime(ddLossPct float64) string {
	switch {
	case ddLossPct < 5:
		return "near_high"
	case ddLossPct < 15:
		return "normal_pullback"
	case ddLossPct < 30:
		return "correction"
	default:
		return "deep_drawdown"
	}
}

func extremeTailLossPct(returns []float64, tailPct float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	cp := append([]float64{}, returns...)
	sort.Float64s(cp)
	n := int(math.Ceil(float64(len(cp)) * tailPct))
	if n < 1 {
		n = 1
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += cp[i]
	}
	return -100 * sum / float64(n)
}

func worstSessionLossPct(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	minR := returns[0]
	for _, r := range returns[1:] {
		if r < minR {
			minR = r
		}
	}
	return -100 * minR
}

func stressScenario(name string, returnPct, lastClose float64, driver string) StressScenario {
	return StressScenario{Name: name, ReturnPct: roundStat(returnPct), Price: roundStat(lastClose * (1 + returnPct/100)), Driver: driver}
}

func sectorStressShockPct(parts ...string) float64 {
	text := strings.ToLower(strings.Join(parts, " "))
	switch {
	case strings.Contains(text, "banka"), strings.Contains(text, "finans"):
		return -12
	case strings.Contains(text, "gayrimenkul"), strings.Contains(text, "gyo"):
		return -15
	case strings.Contains(text, "teknoloji"), strings.Contains(text, "bilisim"):
		return -14
	case strings.Contains(text, "holding"):
		return -10
	default:
		return -9
	}
}

func classifyRateSensitivity(text string) string {
	switch {
	case strings.Contains(text, "banka") || strings.Contains(text, "finans") || strings.Contains(text, "gayrimenkul"):
		return "high"
	case strings.Contains(text, "holding") || strings.Contains(text, "sanayi"):
		return "medium"
	default:
		return "medium_low"
	}
}

func classifyFXSensitivity(text string) string {
	switch {
	case strings.Contains(text, "ihrac") || strings.Contains(text, "otomotiv") || strings.Contains(text, "demir") || strings.Contains(text, "celik"):
		return "high"
	case strings.Contains(text, "perakende") || strings.Contains(text, "havac"):
		return "medium"
	default:
		return "unknown_medium"
	}
}

func classifyInflationSensitivity(text string) string {
	switch {
	case strings.Contains(text, "perakende") || strings.Contains(text, "gida") || strings.Contains(text, "gayrimenkul"):
		return "high"
	case strings.Contains(text, "banka") || strings.Contains(text, "holding"):
		return "medium"
	default:
		return "medium_low"
	}
}

func classifyCyclicalSensitivity(text string) string {
	switch {
	case strings.Contains(text, "otomotiv") || strings.Contains(text, "demir") || strings.Contains(text, "celik") || strings.Contains(text, "cimento"):
		return "high"
	case strings.Contains(text, "telekom") || strings.Contains(text, "saglik") || strings.Contains(text, "gida"):
		return "low"
	default:
		return "medium"
	}
}

func sectorMacroProfile(text string) string {
	switch {
	case strings.Contains(text, "banka"):
		return "rate_curve_credit_sensitive"
	case strings.Contains(text, "gayrimenkul"):
		return "rate_inflation_nav_sensitive"
	case strings.Contains(text, "holding"):
		return "portfolio_discount_macro_sensitive"
	case strings.Contains(text, "teknoloji") || strings.Contains(text, "bilisim"):
		return "growth_duration_sensitive"
	default:
		return "general_equity_macro_sensitive"
	}
}

func dataGovernanceScore(gov professional.FinancialDataGovernance) float64 {
	score := 45.0
	if gov.BacktestSafe {
		score += 18
	}
	if gov.ProductionReady {
		score += 15
	}
	if gov.FinanciallyConsistent {
		score += 12
	}
	score += core.Clamp(gov.PublishDateCoverage, 0, 1) * 5
	score += core.Clamp(gov.AvailableAtCoverage, 0, 1) * 5
	score -= float64(gov.ReconciliationFailureCount) * 8
	score -= float64(gov.UnsafeAvailabilityCount) * 4
	score -= float64(gov.RestatementCount) * 5
	if gov.SurvivorshipBiasRisk {
		score -= 12
	}
	return core.Clamp(score, 0, 100)
}

func accrualQualityProxy(pro professional.Report) float64 {
	if pro.Valuation.NetIncomeTTM == 0 && pro.Valuation.OperatingCashTTM == 0 {
		return 0
	}
	if pro.Valuation.NetIncomeTTM <= 0 {
		if pro.Valuation.OperatingCashTTM > 0 {
			return 55
		}
		return 25
	}
	cashCoverage := pro.Valuation.OperatingCashTTM / math.Abs(pro.Valuation.NetIncomeTTM)
	return core.Clamp(45+cashCoverage*35, 0, 100)
}

func earningsPersistenceProxy(pro professional.Report) float64 {
	years := pro.ValueInvesting.Years
	if len(years) == 0 {
		return firstPositive(pro.SectorFinancials.Score, pro.ValueInvesting.QualityScore)
	}
	positive := 0
	for _, year := range years {
		if year.NetIncome > 0 {
			positive++
		}
	}
	return core.Clamp(100*safeDiv(float64(positive), float64(len(years))), 0, 100)
}

func restatementRisk(count int) string {
	switch {
	case count <= 0:
		return "low"
	case count <= 1:
		return "medium"
	default:
		return "high"
	}
}

func manipulationRiskProxy(redFlags []string, gov professional.FinancialDataGovernance) string {
	score := len(redFlags)
	if gov.ReconciliationFailureCount > 0 {
		score += gov.ReconciliationFailureCount
	}
	if gov.RestatementCount > 0 {
		score += gov.RestatementCount
	}
	switch {
	case score == 0:
		return "low"
	case score <= 2:
		return "medium"
	default:
		return "high"
	}
}

func liquidityScoreFromDaily(result SymbolAnalysis) float64 {
	if tf, ok := result.Timeframes["1D"]; ok {
		return liquidityScore(tf.Professional.Liquidity)
	}
	return 0
}

func liquidityScore(liq professional.LiquidityProfile) float64 {
	score := 45.0
	switch {
	case liq.AverageValueTraded20TRY >= 100_000_000:
		score += 30
	case liq.AverageValueTraded20TRY >= 25_000_000:
		score += 22
	case liq.AverageValueTraded20TRY >= 5_000_000:
		score += 12
	case liq.AverageValueTraded20TRY > 0:
		score += 3
	}
	if liq.CapacityTRYAt10PctADV >= 5_000_000 {
		score += 10
	}
	if liq.DaysToExit1MTRY > 0 && liq.DaysToExit1MTRY <= 2 {
		score += 8
	} else if liq.DaysToExit1MTRY > 5 {
		score -= 10
	}
	if liq.AmihudIlliquidity20 > 0 {
		score -= core.Clamp(math.Log10(1+liq.AmihudIlliquidity20*1e9)*2, 0, 12)
	}
	score -= float64(len(liq.Warnings)) * 4
	return core.Clamp(score, 0, 100)
}

func safeDiv(num, den float64) float64 {
	if math.Abs(den) < 1e-12 {
		return 0
	}
	return num / den
}

func roundStat(value float64) float64 {
	if !core.IsFinite(value) {
		return 0
	}
	return math.Round(value*100) / 100
}

func scoreStatus(score float64) string {
	switch {
	case score >= 75:
		return "pass"
	case score >= 55:
		return "limited"
	default:
		return "fail"
	}
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 && core.IsFinite(value) {
			return value
		}
	}
	return 0
}

func factorStatus(available bool) string {
	if available {
		return "computed"
	}
	return "missing_data"
}

func computedStatus(ok bool) string {
	if ok {
		return "computed"
	}
	return "missing_data"
}

func scoreBeta(beta float64) float64 {
	if beta <= 0 {
		return 50
	}
	return core.Clamp(100-math.Abs(beta-1)*35, 0, 100)
}

func scoreRelativeStrength(rs float64) float64 {
	return core.Clamp(50+rs*700, 0, 100)
}

func meanLast(values []float64, n int) float64 {
	if len(values) == 0 {
		return 0
	}
	if n <= 0 || len(values) < n {
		n = len(values)
	}
	return core.Mean(values[len(values)-n:])
}

func statCompactWarnings(groups ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
