package analysis

import (
	"math"
	"strconv"
	"strings"

	"hissebot/internal/quant/core"
	cryptoquant "hissebot/internal/quant/crypto"
	equityquant "hissebot/internal/quant/equity"
	"hissebot/internal/quant/portfolio"
	"hissebot/internal/ta/ohlcv"
)

const (
	quantTradingDaysPerYear = 252.0
	quantCryptoDaysPerYear  = 365.0
	quantConfidenceLevel    = 0.95
)

type QuantAnalysis struct {
	Computed          bool                  `json:"computed"`
	Status            string                `json:"status,omitempty"`
	Method            string                `json:"method,omitempty"`
	SourceTimeframe   string                `json:"source_timeframe,omitempty"`
	MarketClock       string                `json:"market_clock,omitempty"`
	AnnualizationDays float64               `json:"annualization_days,omitempty"`
	SampleStart       string                `json:"sample_start,omitempty"`
	SampleEnd         string                `json:"sample_end,omitempty"`
	SampleCount       int                   `json:"sample_count,omitempty"`
	Return            QuantReturnMetrics    `json:"return_metrics"`
	Risk              QuantRiskMetrics      `json:"risk_metrics"`
	Benchmark         QuantBenchmarkMetrics `json:"benchmark_metrics"`
	EquityProfile     *equityquant.Report   `json:"equity_profile,omitempty"`
	CryptoProfile     *cryptoquant.Report   `json:"crypto_profile,omitempty"`
	Decision          QuantDecision         `json:"decision"`
	Modules           []QuantModuleCoverage `json:"modules,omitempty"`
	Warnings          []string              `json:"warnings,omitempty"`
}

type QuantReturnMetrics struct {
	LastClose               float64 `json:"last_close,omitempty"`
	Return1DPct             float64 `json:"return_1d_pct,omitempty"`
	Return5DPct             float64 `json:"return_5d_pct,omitempty"`
	Return20DPct            float64 `json:"return_20d_pct,omitempty"`
	Return60DPct            float64 `json:"return_60d_pct,omitempty"`
	Return120DPct           float64 `json:"return_120d_pct,omitempty"`
	Return252DPct           float64 `json:"return_252d_pct,omitempty"`
	MeanDailyReturnPct      float64 `json:"mean_daily_return_pct,omitempty"`
	AnnualizedReturnPct     float64 `json:"annualized_return_pct,omitempty"`
	CAGRPct                 float64 `json:"cagr_pct,omitempty"`
	PositiveSessionRatioPct float64 `json:"positive_session_ratio_pct,omitempty"`
}

type QuantRiskMetrics struct {
	DailyVolatilityPct         float64 `json:"daily_volatility_pct,omitempty"`
	AnnualizedVolatilityPct    float64 `json:"annualized_volatility_pct,omitempty"`
	DownsideVolatilityPct      float64 `json:"downside_volatility_pct,omitempty"`
	HistoricalVaR95Pct         float64 `json:"historical_var_95_pct,omitempty"`
	HistoricalCVaR95Pct        float64 `json:"historical_cvar_95_pct,omitempty"`
	ParametricVaR95Pct         float64 `json:"parametric_var_95_pct,omitempty"`
	MaxDrawdownLossPct         float64 `json:"max_drawdown_loss_pct,omitempty"`
	CurrentDrawdownLossPct     float64 `json:"current_drawdown_loss_pct,omitempty"`
	SharpeRatio                float64 `json:"sharpe_ratio,omitempty"`
	SortinoRatio               float64 `json:"sortino_ratio,omitempty"`
	CalmarRatio                float64 `json:"calmar_ratio,omitempty"`
	Skewness                   float64 `json:"skewness,omitempty"`
	ExcessKurtosis             float64 `json:"excess_kurtosis,omitempty"`
	VolatilityRegime           string  `json:"volatility_regime,omitempty"`
	RiskBudgetOneDayVaRPer100K float64 `json:"risk_budget_one_day_var_per_100k,omitempty"`
}

type QuantBenchmarkMetrics struct {
	Symbol                string  `json:"symbol,omitempty"`
	Available             bool    `json:"available"`
	Error                 string  `json:"error,omitempty"`
	RelativeStrength20Pct float64 `json:"relative_strength_20_pct,omitempty"`
	RelativeStrength60Pct float64 `json:"relative_strength_60_pct,omitempty"`
	Beta60                float64 `json:"beta_60,omitempty"`
	Alpha60AnnualPct      float64 `json:"alpha_60_annual_pct,omitempty"`
	Correlation60         float64 `json:"correlation_60,omitempty"`
}

type QuantDecision struct {
	Score       float64  `json:"score,omitempty"`
	RiskScore   float64  `json:"risk_score,omitempty"`
	ReturnScore float64  `json:"return_score,omitempty"`
	Label       string   `json:"label,omitempty"`
	Suitability string   `json:"suitability,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Passes      []string `json:"passes,omitempty"`
	Blockers    []string `json:"blockers,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type QuantModuleCoverage struct {
	Name     string `json:"name"`
	Package  string `json:"package"`
	Status   string `json:"status"`
	Evidence string `json:"evidence,omitempty"`
}

func BuildQuantAnalysis(result SymbolAnalysis) QuantAnalysis {
	tf, ok := result.Timeframes["1D"]
	source := "1D"
	if !ok {
		tf, ok = selectFinTradeBenchTimeframe(result.Timeframes)
		source = tf.Timeframe
	}
	annualizationDays := quantAnnualizationDays(result.AssetType)
	out := QuantAnalysis{
		Status:            "not_computed",
		Method:            "equity_crypto_quant_risk_features",
		SourceTimeframe:   source,
		MarketClock:       quantMarketClock(result.AssetType),
		AnnualizationDays: annualizationDays,
		Benchmark: QuantBenchmarkMetrics{
			Symbol: strings.TrimSpace(result.Professional.Market.BenchmarkSymbol),
			Error:  strings.TrimSpace(result.Professional.Market.BenchmarkError),
		},
		Modules: quantModuleCoverage(false, result.Professional.Market.BenchmarkAvailable, result.AssetType),
	}
	if !ok || len(tf.Candles) < 3 {
		out.Warnings = append(out.Warnings, "quant_requires_at_least_3_candles")
		return out
	}
	closes := quantEffectiveCloses(tf.Candles)
	returns := quantFiniteReturns(core.Returns(closes))
	if len(closes) < 3 || len(returns) < 2 {
		out.Warnings = append(out.Warnings, "quant_requires_valid_close_returns")
		return out
	}
	out.Computed = true
	out.Status = "computed"
	out.Modules = quantModuleCoverage(true, result.Professional.Market.BenchmarkAvailable, result.AssetType)
	out.SampleCount = len(returns)
	out.SampleStart = tf.Candles[len(tf.Candles)-len(returns)-1].Time.Format("2006-01-02")
	out.SampleEnd = tf.Candles[len(tf.Candles)-1].Time.Format("2006-01-02")
	out.Return = quantReturnMetrics(closes, returns, annualizationDays)
	out.Risk = quantRiskMetrics(closes, returns, annualizationDays, result.AssetType)
	out.Benchmark = quantBenchmarkMetrics(result)
	out.EquityProfile, out.CryptoProfile = quantAssetProfiles(result, out)
	out.Decision = quantDecision(result.AssetType, out.Return, out.Risk, out.Benchmark)
	out.Warnings = append(out.Warnings, out.Decision.Warnings...)
	return out
}

func quantEffectiveCloses(candles []ohlcv.Candle) []float64 {
	out := make([]float64, 0, len(candles))
	for _, candle := range candles {
		close := candle.EffectiveClose()
		if close > 0 && core.IsFinite(close) {
			out = append(out, close)
		}
	}
	return out
}

func quantFiniteReturns(values []float64) []float64 {
	out := make([]float64, 0, len(values))
	for _, value := range values {
		if core.IsFinite(value) && value > -0.95 && value < 10 {
			out = append(out, value)
		}
	}
	return out
}

func quantReturnMetrics(closes, returns []float64, annualizationDays float64) QuantReturnMetrics {
	last := closes[len(closes)-1]
	positive := 0
	for _, r := range returns {
		if r > 0 {
			positive++
		}
	}
	return QuantReturnMetrics{
		LastClose:               roundQuant(last),
		Return1DPct:             roundQuant(quantWindowReturnPct(closes, 1)),
		Return5DPct:             roundQuant(quantWindowReturnPct(closes, 5)),
		Return20DPct:            roundQuant(quantWindowReturnPct(closes, 20)),
		Return60DPct:            roundQuant(quantWindowReturnPct(closes, 60)),
		Return120DPct:           roundQuant(quantWindowReturnPct(closes, 120)),
		Return252DPct:           roundQuant(quantWindowReturnPct(closes, 252)),
		MeanDailyReturnPct:      roundQuant(core.Mean(returns) * 100),
		AnnualizedReturnPct:     roundQuant(core.Mean(returns) * annualizationDays * 100),
		CAGRPct:                 roundQuant(quantCAGRPct(closes, annualizationDays)),
		PositiveSessionRatioPct: roundQuant(100 * float64(positive) / float64(len(returns))),
	}
}

func quantRiskMetrics(closes, returns []float64, annualizationDays float64, assetType string) QuantRiskMetrics {
	mean := core.Mean(returns)
	dailyVol := core.StdDev(returns, true)
	downside := make([]float64, 0, len(returns))
	for _, r := range returns {
		if r < 0 {
			downside = append(downside, r)
		}
	}
	sharpe, sortino, maxDD := portfolio.PerformanceRatios(returns, 0)
	paramVaR := portfolio.ParametricVaR(mean, dailyVol, quantConfidenceLevel)
	maxDrawdownLoss := -maxDD * 100
	currentDrawdownLoss := quantCurrentDrawdownLossPct(closes)
	annualReturn := mean * annualizationDays
	calmar := 0.0
	if maxDrawdownLoss > 0 {
		calmar = annualReturn / (maxDrawdownLoss / 100)
	}
	variance := core.Variance(returns, true)
	expected := []float64{mean}
	covariance := [][]float64{{variance}}
	riskReport := portfolio.ComprehensiveRiskAnalysis([]float64{1}, expected, covariance, returns, quantConfidenceLevel, 0)
	annualizedVolPct := dailyVol * math.Sqrt(annualizationDays) * 100
	return QuantRiskMetrics{
		DailyVolatilityPct:         roundQuant(dailyVol * 100),
		AnnualizedVolatilityPct:    roundQuant(annualizedVolPct),
		DownsideVolatilityPct:      roundQuant(core.StdDev(downside, true) * math.Sqrt(annualizationDays) * 100),
		HistoricalVaR95Pct:         roundQuant(riskReport.VaR * 100),
		HistoricalCVaR95Pct:        roundQuant(riskReport.CVaR * 100),
		ParametricVaR95Pct:         roundQuant(paramVaR * 100),
		MaxDrawdownLossPct:         roundQuant(maxDrawdownLoss),
		CurrentDrawdownLossPct:     roundQuant(currentDrawdownLoss),
		SharpeRatio:                roundQuant(sharpe),
		SortinoRatio:               roundQuant(sortino),
		CalmarRatio:                roundQuant(calmar),
		Skewness:                   roundQuant(quantSkewness(returns)),
		ExcessKurtosis:             roundQuant(quantExcessKurtosis(returns)),
		VolatilityRegime:           quantVolatilityRegimeForAsset(assetType, annualizedVolPct),
		RiskBudgetOneDayVaRPer100K: roundQuant(riskReport.VaR * 100000),
	}
}

func quantBenchmarkMetrics(result SymbolAnalysis) QuantBenchmarkMetrics {
	market := result.Professional.Market
	return QuantBenchmarkMetrics{
		Symbol:                strings.TrimSpace(market.BenchmarkSymbol),
		Available:             market.BenchmarkAvailable,
		Error:                 strings.TrimSpace(market.BenchmarkError),
		RelativeStrength20Pct: roundQuant(market.RelativeStrength20 * 100),
		RelativeStrength60Pct: roundQuant(market.RelativeStrength60 * 100),
		Beta60:                roundQuant(market.Beta60),
		Alpha60AnnualPct:      roundQuant(market.Alpha60 * 100),
		Correlation60:         roundQuant(market.Correlation60),
	}
}

func quantDecision(assetType string, ret QuantReturnMetrics, risk QuantRiskMetrics, benchmark QuantBenchmarkMetrics) QuantDecision {
	volBase := 22.0
	varBase := 2.5
	drawdownBase := 20.0
	varWarn := 5.0
	drawdownWarn := 35.0
	if ohlcv.IsCryptoAssetType(assetType) {
		volBase = 65
		varBase = 6
		drawdownBase = 42
		varWarn = 8
		drawdownWarn = 55
	}
	riskScore := 100.0
	riskScore -= core.Clamp((risk.AnnualizedVolatilityPct-volBase)*1.15, 0, 32)
	riskScore -= core.Clamp((risk.HistoricalVaR95Pct-varBase)*8, 0, 26)
	riskScore -= core.Clamp((risk.MaxDrawdownLossPct-drawdownBase)*0.85, 0, 24)
	if benchmark.Available && benchmark.Beta60 > 1.25 {
		riskScore -= core.Clamp((benchmark.Beta60-1.25)*24, 0, 10)
	}
	riskScore = core.Clamp(riskScore, 0, 100)

	returnScore := 48.0 + ret.AnnualizedReturnPct*0.9 + ret.Return60DPct*0.35
	if benchmark.Available {
		returnScore += benchmark.RelativeStrength60Pct * 0.35
		returnScore += benchmark.Alpha60AnnualPct * 0.20
	}
	returnScore = core.Clamp(returnScore, 0, 100)

	benchmarkScore := 50.0
	if benchmark.Available {
		benchmarkScore = 50 + benchmark.RelativeStrength60Pct*0.7 + benchmark.Alpha60AnnualPct*0.45
		if benchmark.Beta60 > 1.2 && benchmark.RelativeStrength60Pct < 0 {
			benchmarkScore -= 10
		}
	}
	benchmarkScore = core.Clamp(benchmarkScore, 0, 100)

	score := core.Clamp(riskScore*0.55+returnScore*0.30+benchmarkScore*0.15, 0, 100)
	decision := QuantDecision{
		Score:       roundQuant(score),
		RiskScore:   roundQuant(riskScore),
		ReturnScore: roundQuant(returnScore),
	}
	acceptableScore := quantDecisionScoreLimit(assetType)
	acceptableRiskScore := quantDecisionRiskScoreLimit(assetType)
	failRiskScore := quantDecisionFailRiskScoreLimit(assetType)
	strongRiskScore := 65.0
	if ohlcv.IsCryptoAssetType(assetType) {
		strongRiskScore = 60
	}
	switch {
	case score >= 75 && riskScore >= strongRiskScore:
		decision.Label = "quant_strong"
		decision.Suitability = "portfoye_uygun"
		decision.Passes = append(decision.Passes, "quant risk/getiri profili güçlü")
	case score >= acceptableScore && riskScore >= acceptableRiskScore:
		decision.Label = "quant_acceptable"
		decision.Suitability = "sinirli_pozisyon_uygun"
		decision.Passes = append(decision.Passes, "quant profil izlenebilir seviyede")
	case riskScore < failRiskScore:
		decision.Label = "quant_high_risk"
		decision.Suitability = "risk_limiti_gerekli"
		decision.Blockers = append(decision.Blockers, "quant risk skoru düşük")
	default:
		decision.Label = "quant_watchlist"
		decision.Suitability = "izleme_listesi"
		decision.Warnings = append(decision.Warnings, "quant skor karar için sınırlı")
	}
	if risk.HistoricalVaR95Pct >= varWarn {
		decision.Warnings = append(decision.Warnings, "1 günlük VaR yüksek")
	}
	if risk.MaxDrawdownLossPct >= drawdownWarn {
		decision.Warnings = append(decision.Warnings, "tarihsel maksimum düşüş yüksek")
	}
	if benchmark.Available && benchmark.RelativeStrength60Pct < -5 {
		decision.Warnings = append(decision.Warnings, "benchmark'a göre 60 günlük göreli güç zayıf")
	}
	decision.Summary = quantDecisionSummary(decision, ret, risk, benchmark)
	return decision
}

func quantDecisionSummary(decision QuantDecision, ret QuantReturnMetrics, risk QuantRiskMetrics, benchmark QuantBenchmarkMetrics) string {
	parts := []string{
		"Quant skor " + formatQuant(decision.Score) + "/100",
		"yıllık vol " + formatQuant(risk.AnnualizedVolatilityPct) + "%",
		"VaR95 " + formatQuant(risk.HistoricalVaR95Pct) + "%",
		"60g getiri " + formatQuant(ret.Return60DPct) + "%",
	}
	if benchmark.Available {
		parts = append(parts, "beta "+formatQuant(benchmark.Beta60), "RS60 "+formatQuant(benchmark.RelativeStrength60Pct)+"%")
	}
	return strings.Join(parts, "; ") + "."
}

func quantModuleCoverage(computed bool, benchmarkAvailable bool, assetType string) []QuantModuleCoverage {
	status := "not_computed"
	if computed {
		status = "computed"
	}
	benchmarkStatus := "missing_benchmark"
	if benchmarkAvailable {
		benchmarkStatus = "computed"
	}
	equityStatus := "not_applicable"
	cryptoStatus := "not_applicable"
	if ohlcv.NormalizeAssetType(assetType) == ohlcv.AssetTypeEquity {
		equityStatus = status
	}
	if ohlcv.IsCryptoAssetType(assetType) {
		cryptoStatus = status
	}
	return []QuantModuleCoverage{
		{Name: "statistics", Package: "internal/quant/core", Status: status, Evidence: "returns, volatility, quantiles"},
		{Name: "portfolio_risk", Package: "internal/quant/portfolio", Status: status, Evidence: "VaR, CVaR, Sharpe, Sortino, drawdown"},
		{Name: "benchmark_factor", Package: "internal/ta/professional + internal/quant/core", Status: benchmarkStatus, Evidence: "beta, alpha, correlation, relative strength"},
		{Name: "equity_profile", Package: "internal/quant/equity", Status: equityStatus, Evidence: "verified close, benchmark, financial/KAP evidence, equity risk budget"},
		{Name: "crypto_profile", Package: "internal/quant/crypto", Status: cryptoStatus, Evidence: "24/7 annualization, crypto context, leverage/market-structure risk budget"},
	}
}

func quantAssetProfiles(result SymbolAnalysis, q QuantAnalysis) (*equityquant.Report, *cryptoquant.Report) {
	if ohlcv.NormalizeAssetType(result.AssetType) == ohlcv.AssetTypeEquity {
		profile := equityquant.Evaluate(equityquant.Input{
			VerifiedCloseAvailable:   result.PriceQuality != nil && result.PriceQuality.ReadyForVerifiedClose,
			BenchmarkAvailable:       q.Benchmark.Available,
			FinancialEvidencePresent: result.Professional.ValueInvesting.Computed || result.Professional.DataGovernance.ProductionReady,
			KAPPDFEvidencePresent:    result.Professional.KAPPDFIngest.Computed && result.Professional.KAPPDFIngest.AnalysisUsableCount > 0,
			Beta60:                   q.Benchmark.Beta60,
			Alpha60AnnualPct:         q.Benchmark.Alpha60AnnualPct,
			RelativeStrength60Pct:    q.Benchmark.RelativeStrength60Pct,
			Return60Pct:              q.Return.Return60DPct,
			AnnualizedVolatilityPct:  q.Risk.AnnualizedVolatilityPct,
			HistoricalVaR95Pct:       q.Risk.HistoricalVaR95Pct,
			MaxDrawdownLossPct:       q.Risk.MaxDrawdownLossPct,
		})
		return &profile, nil
	}
	if ohlcv.IsCryptoAssetType(result.AssetType) {
		ctx := result.Professional.CryptoContext
		profile := cryptoquant.Evaluate(cryptoquant.Input{
			OnChainAvailable:        ctx.OnChain.Available,
			DerivativesAvailable:    ctx.Derivatives.Available,
			ExchangeFlowAvailable:   ctx.ExchangeFlow.Available,
			NewsSentimentAvailable:  ctx.NewsSentiment.Available,
			ContextCoverageScore:    result.Professional.Coverage.Score,
			BenchmarkAvailable:      q.Benchmark.Available,
			Beta60:                  q.Benchmark.Beta60,
			RelativeStrength60Pct:   q.Benchmark.RelativeStrength60Pct,
			Return60Pct:             q.Return.Return60DPct,
			AnnualizedVolatilityPct: q.Risk.AnnualizedVolatilityPct,
			HistoricalVaR95Pct:      q.Risk.HistoricalVaR95Pct,
			MaxDrawdownLossPct:      q.Risk.MaxDrawdownLossPct,
		})
		return nil, &profile
	}
	return nil, nil
}

func applyQuantAdjustment(score float64, quant QuantAnalysis) float64 {
	if !quant.Computed || quant.Decision.Score <= 0 {
		return score
	}
	adjustment := (quant.Decision.Score - 50) * 0.08
	if quant.Decision.RiskScore < 40 {
		adjustment -= (40 - quant.Decision.RiskScore) * 0.08
	}
	return core.Clamp(score+adjustment, 0, 100)
}

func quantWindowReturnPct(closes []float64, window int) float64 {
	if window <= 0 || len(closes) <= window {
		return 0
	}
	base := closes[len(closes)-1-window]
	last := closes[len(closes)-1]
	if base <= 0 {
		return 0
	}
	return (last/base - 1) * 100
}

func quantCAGRPct(closes []float64, annualizationDays float64) float64 {
	if len(closes) < 2 || closes[0] <= 0 {
		return 0
	}
	years := float64(len(closes)-1) / annualizationDays
	if years <= 0 {
		return 0
	}
	return (math.Pow(closes[len(closes)-1]/closes[0], 1/years) - 1) * 100
}

func quantCurrentDrawdownLossPct(closes []float64) float64 {
	if len(closes) == 0 {
		return 0
	}
	peak := closes[0]
	for _, close := range closes {
		if close > peak {
			peak = close
		}
	}
	if peak <= 0 {
		return 0
	}
	return math.Max(0, (1-closes[len(closes)-1]/peak)*100)
}

func quantSkewness(values []float64) float64 {
	if len(values) < 3 {
		return 0
	}
	mean := core.Mean(values)
	sd := core.StdDev(values, true)
	if sd <= core.Epsilon {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		z := (value - mean) / sd
		sum += z * z * z
	}
	return sum / float64(len(values))
}

func quantExcessKurtosis(values []float64) float64 {
	if len(values) < 4 {
		return 0
	}
	mean := core.Mean(values)
	sd := core.StdDev(values, true)
	if sd <= core.Epsilon {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		z := (value - mean) / sd
		sum += z * z * z * z
	}
	return sum/float64(len(values)) - 3
}

func quantVolatilityRegime(annualizedVolPct float64) string {
	switch {
	case annualizedVolPct <= 18:
		return "low"
	case annualizedVolPct <= 32:
		return "normal"
	case annualizedVolPct <= 50:
		return "high"
	default:
		return "extreme"
	}
}

func quantVolatilityRegimeForAsset(assetType string, annualizedVolPct float64) string {
	if ohlcv.IsCryptoAssetType(assetType) {
		switch {
		case annualizedVolPct <= 45:
			return "low"
		case annualizedVolPct <= 85:
			return "normal"
		case annualizedVolPct <= 125:
			return "high"
		default:
			return "extreme"
		}
	}
	return quantVolatilityRegime(annualizedVolPct)
}

func quantAnnualizationDays(assetType string) float64 {
	if ohlcv.IsCryptoAssetType(assetType) {
		return quantCryptoDaysPerYear
	}
	return quantTradingDaysPerYear
}

func quantMarketClock(assetType string) string {
	if ohlcv.IsCryptoAssetType(assetType) {
		return "24_7"
	}
	return "exchange_sessions"
}

func roundQuant(value float64) float64 {
	if !core.IsFinite(value) {
		return 0
	}
	return math.Round(value*100) / 100
}

func formatQuant(value float64) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimRight(strconv.FormatFloat(roundQuant(value), 'f', 2, 64), "0"), "."))
}
