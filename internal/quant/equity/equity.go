package equity

import "math"

const AnnualizationDays = 252.0

type Input struct {
	VerifiedCloseAvailable   bool
	BenchmarkAvailable       bool
	FinancialEvidencePresent bool
	KAPPDFEvidencePresent    bool
	Beta60                   float64
	Alpha60AnnualPct         float64
	RelativeStrength60Pct    float64
	Return60Pct              float64
	AnnualizedVolatilityPct  float64
	HistoricalVaR95Pct       float64
	MaxDrawdownLossPct       float64
}

type Report struct {
	Computed               bool     `json:"computed"`
	Status                 string   `json:"status,omitempty"`
	MarketClock            string   `json:"market_clock"`
	AnnualizationDays      float64  `json:"annualization_days"`
	Score                  float64  `json:"score,omitempty"`
	RiskBudgetMultiplier   float64  `json:"risk_budget_multiplier,omitempty"`
	PositionSizeMultiplier float64  `json:"position_size_multiplier,omitempty"`
	BetaRiskLabel          string   `json:"beta_risk_label,omitempty"`
	RelativeStrengthLabel  string   `json:"relative_strength_label,omitempty"`
	RequiredData           []string `json:"required_data,omitempty"`
	AvailableData          []string `json:"available_data,omitempty"`
	MissingData            []string `json:"missing_data,omitempty"`
	DecisionUse            []string `json:"decision_use,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

func Evaluate(input Input) Report {
	out := Report{
		Computed:          true,
		MarketClock:       "exchange_sessions",
		AnnualizationDays: AnnualizationDays,
		RequiredData: []string{
			"verified_official_close",
			"benchmark_beta_relative_strength",
			"financial_or_kap_pdf_evidence",
		},
		DecisionUse: []string{
			"risk_gate",
			"position_size",
			"technical_signal_veto",
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
	addData("verified_official_close", input.VerifiedCloseAvailable, 18)
	addData("benchmark_beta_relative_strength", input.BenchmarkAvailable, 12)
	addData("financial_or_kap_pdf_evidence", input.FinancialEvidencePresent || input.KAPPDFEvidencePresent, 16)

	score -= clamp((input.AnnualizedVolatilityPct-32)*0.50, 0, 20)
	score -= clamp((input.HistoricalVaR95Pct-4.5)*6.0, 0, 22)
	score -= clamp((input.MaxDrawdownLossPct-25)*0.45, 0, 18)
	if input.BenchmarkAvailable && input.Beta60 > 1.25 && input.RelativeStrength60Pct < 0 {
		score -= clamp((input.Beta60-1.25)*16, 0, 10)
	}
	if input.RelativeStrength60Pct < -8 {
		score -= 6
	}
	if input.Return60Pct < -12 {
		score -= 5
	}
	out.Score = round2(clamp(score, 0, 100))
	out.BetaRiskLabel = betaRiskLabel(input.Beta60, input.BenchmarkAvailable)
	out.RelativeStrengthLabel = relativeStrengthLabel(input.RelativeStrength60Pct, input.BenchmarkAvailable)
	out.RiskBudgetMultiplier = riskBudgetMultiplier(out.Score)
	out.PositionSizeMultiplier = out.RiskBudgetMultiplier
	out.Status = status(out.Score, len(out.MissingData))
	if input.HistoricalVaR95Pct >= 5 {
		out.Warnings = append(out.Warnings, "equity_var95_high")
	}
	if input.MaxDrawdownLossPct >= 35 {
		out.Warnings = append(out.Warnings, "equity_drawdown_high")
	}
	if !input.VerifiedCloseAvailable {
		out.Warnings = append(out.Warnings, "verified_close_missing")
	}
	return out
}

func betaRiskLabel(beta float64, available bool) string {
	if !available {
		return "benchmark_missing"
	}
	switch {
	case beta <= 0:
		return "beta_unavailable"
	case beta < 0.80:
		return "defensive"
	case beta <= 1.20:
		return "market_like"
	default:
		return "high_beta"
	}
}

func relativeStrengthLabel(value float64, available bool) string {
	if !available {
		return "benchmark_missing"
	}
	switch {
	case value >= 5:
		return "outperforming"
	case value <= -5:
		return "underperforming"
	default:
		return "neutral"
	}
}

func status(score float64, missing int) string {
	switch {
	case score >= 70 && missing == 0:
		return "pass"
	case score >= 50:
		return "limited"
	default:
		return "fail"
	}
}

func riskBudgetMultiplier(score float64) float64 {
	switch {
	case score >= 80:
		return 1.00
	case score >= 65:
		return 0.75
	case score >= 50:
		return 0.50
	default:
		return 0.25
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
