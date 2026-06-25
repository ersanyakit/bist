package crypto

import "math"

const AnnualizationDays = 365.0

type Input struct {
	OnChainAvailable        bool
	DerivativesAvailable    bool
	ExchangeFlowAvailable   bool
	NewsSentimentAvailable  bool
	ContextCoverageScore    float64
	BenchmarkAvailable      bool
	Beta60                  float64
	RelativeStrength60Pct   float64
	Return60Pct             float64
	AnnualizedVolatilityPct float64
	HistoricalVaR95Pct      float64
	MaxDrawdownLossPct      float64
}

type Report struct {
	Computed               bool     `json:"computed"`
	Status                 string   `json:"status,omitempty"`
	MarketClock            string   `json:"market_clock"`
	AnnualizationDays      float64  `json:"annualization_days"`
	Score                  float64  `json:"score,omitempty"`
	RiskBudgetMultiplier   float64  `json:"risk_budget_multiplier,omitempty"`
	PositionSizeMultiplier float64  `json:"position_size_multiplier,omitempty"`
	LeverageRiskLabel      string   `json:"leverage_risk_label,omitempty"`
	MarketStructureLabel   string   `json:"market_structure_label,omitempty"`
	RequiredData           []string `json:"required_data,omitempty"`
	AvailableData          []string `json:"available_data,omitempty"`
	MissingData            []string `json:"missing_data,omitempty"`
	DecisionUse            []string `json:"decision_use,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

func Evaluate(input Input) Report {
	out := Report{
		Computed:          true,
		MarketClock:       "24_7",
		AnnualizationDays: AnnualizationDays,
		RequiredData: []string{
			"spot_ohlcv",
			"btc_or_market_benchmark",
			"onchain_context",
			"derivatives_funding_open_interest_liquidations",
			"exchange_flow_reserve_netflow",
			"crypto_news_sentiment",
		},
		AvailableData: []string{"spot_ohlcv"},
		DecisionUse: []string{
			"risk_gate",
			"position_size",
			"leverage_veto",
			"forecast_feature_quality",
		},
	}
	score := 100.0
	addData := func(name string, ok bool, penalty float64) {
		if ok {
			out.AvailableData = append(out.AvailableData, name)
			return
		}
		out.MissingData = append(out.MissingData, name)
		score -= penalty
	}
	addData("btc_or_market_benchmark", input.BenchmarkAvailable, 10)
	addData("onchain_context", input.OnChainAvailable, 10)
	addData("derivatives_funding_open_interest_liquidations", input.DerivativesAvailable, 16)
	addData("exchange_flow_reserve_netflow", input.ExchangeFlowAvailable, 10)
	addData("crypto_news_sentiment", input.NewsSentimentAvailable, 6)

	if input.ContextCoverageScore > 0 {
		score += clamp((input.ContextCoverageScore-50)*0.10, -8, 8)
	}
	score -= clamp((input.AnnualizedVolatilityPct-85)*0.20, 0, 20)
	score -= clamp((input.HistoricalVaR95Pct-7)*5.0, 0, 24)
	score -= clamp((input.MaxDrawdownLossPct-45)*0.35, 0, 18)
	if input.BenchmarkAvailable && input.Beta60 > 1.35 && input.RelativeStrength60Pct < 0 {
		score -= clamp((input.Beta60-1.35)*12, 0, 8)
	}
	if input.RelativeStrength60Pct < -12 {
		score -= 6
	}
	if input.Return60Pct < -25 {
		score -= 6
	}
	out.Score = round2(clamp(score, 0, 100))
	out.LeverageRiskLabel = leverageRiskLabel(input.HistoricalVaR95Pct, input.AnnualizedVolatilityPct, input.DerivativesAvailable)
	out.MarketStructureLabel = marketStructureLabel(input.OnChainAvailable, input.DerivativesAvailable, input.ExchangeFlowAvailable)
	out.RiskBudgetMultiplier = riskBudgetMultiplier(out.Score)
	out.PositionSizeMultiplier = out.RiskBudgetMultiplier
	out.Status = status(out.Score, len(out.MissingData))
	if input.HistoricalVaR95Pct >= 8 {
		out.Warnings = append(out.Warnings, "crypto_var95_high")
	}
	if input.AnnualizedVolatilityPct >= 120 {
		out.Warnings = append(out.Warnings, "crypto_extreme_volatility")
	}
	if !input.DerivativesAvailable {
		out.Warnings = append(out.Warnings, "crypto_derivatives_context_missing")
	}
	return out
}

func leverageRiskLabel(var95, annualVol float64, derivativesAvailable bool) string {
	if !derivativesAvailable {
		return "derivatives_missing"
	}
	switch {
	case var95 >= 10 || annualVol >= 130:
		return "extreme"
	case var95 >= 7 || annualVol >= 90:
		return "high"
	case var95 >= 4:
		return "normal"
	default:
		return "low"
	}
}

func marketStructureLabel(onchain, derivatives, flow bool) string {
	available := 0
	for _, ok := range []bool{onchain, derivatives, flow} {
		if ok {
			available++
		}
	}
	switch available {
	case 3:
		return "complete"
	case 2:
		return "usable"
	case 1:
		return "thin"
	default:
		return "missing"
	}
}

func status(score float64, missing int) string {
	switch {
	case score >= 72 && missing <= 1:
		return "pass"
	case score >= 50:
		return "limited"
	default:
		return "fail"
	}
}

func riskBudgetMultiplier(score float64) float64 {
	switch {
	case score >= 82:
		return 0.75
	case score >= 65:
		return 0.55
	case score >= 50:
		return 0.35
	default:
		return 0.15
	}
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}

func round2(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*100) / 100
}
