package ml

import (
	"math"
	"time"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/validation"
)

type ModelPrediction struct {
	Symbol                 string             `json:"symbol"`
	AsOf                   time.Time          `json:"as_of"`
	TargetSession          time.Time          `json:"target_session"`
	ModelName              string             `json:"model_name"`
	ModelVersion           string             `json:"model_version"`
	FeatureSetVersion      string             `json:"feature_set_version"`
	PredictedOpen          float64            `json:"predicted_open"`
	PredictedClose         float64            `json:"predicted_close"`
	ExpectedReturn         float64            `json:"expected_return"`
	Direction              string             `json:"direction"`
	DirectionProbUp        float64            `json:"direction_prob_up"`
	DirectionProbDown      float64            `json:"direction_prob_down"`
	DirectionProbFlat      float64            `json:"direction_prob_flat"`
	Quantiles              map[string]float64 `json:"quantiles,omitempty"`
	Confidence             float64            `json:"confidence"`
	CalibratedConfidence   float64            `json:"calibrated_confidence"`
	PredictionIntervalLow  float64            `json:"prediction_interval_low"`
	PredictionIntervalHigh float64            `json:"prediction_interval_high"`
	Warnings               []string           `json:"warnings,omitempty"`
	Debug                  map[string]any     `json:"debug,omitempty"`
}

type DirectionPrediction struct {
	Direction string  `json:"direction"`
	ProbUp    float64 `json:"prob_up"`
	ProbDown  float64 `json:"prob_down"`
	ProbFlat  float64 `json:"prob_flat"`
}

type DirectionProbabilities struct {
	Up   float64 `json:"up"`
	Down float64 `json:"down"`
	Flat float64 `json:"flat"`
}

type PredictionInterval struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

type ReportValidationSummary struct {
	WalkForwardWindows int                `json:"walk_forward_windows"`
	MAE                float64            `json:"mae"`
	RMSE               float64            `json:"rmse"`
	MAPE               float64            `json:"mape"`
	DirectionAccuracy  float64            `json:"direction_accuracy"`
	BrierScore         float64            `json:"brier_score"`
	Sharpe             float64            `json:"sharpe"`
	MaxDrawdown        float64            `json:"max_drawdown"`
	BaselineComparison map[string]float64 `json:"baseline_comparison,omitempty"`
}

type ReportTradeGate struct {
	Allowed      bool     `json:"allowed"`
	Action       string   `json:"action"`
	Confidence   string   `json:"confidence"`
	Reasons      []string `json:"reasons,omitempty"`
	RiskWarnings []string `json:"risk_warnings,omitempty"`
}

type ReportDataQuality struct {
	MissingRatio float64  `json:"missing_ratio"`
	StaleFields  []string `json:"stale_fields,omitempty"`
	LeakageFlags []string `json:"leakage_flags,omitempty"`
	SourceScore  float64  `json:"source_score"`
}

type ReportFallback struct {
	Used   bool   `json:"used"`
	Reason string `json:"reason,omitempty"`
}

type CalibrationSummary struct {
	Method             string                         `json:"method,omitempty"`
	BrierScore         float64                        `json:"brier_score,omitempty"`
	CalibrationError   float64                        `json:"calibration_error,omitempty"`
	ReliabilityBuckets []validation.ReliabilityBucket `json:"reliability_buckets,omitempty"`
}

type ModelContribution struct {
	ModelName          string   `json:"model_name"`
	ModelVersion       string   `json:"model_version,omitempty"`
	Family             string   `json:"family,omitempty"`
	Weight             float64  `json:"weight"`
	NormalizedWeight   float64  `json:"normalized_weight"`
	ExpectedReturn     float64  `json:"expected_return"`
	PredictedClose     float64  `json:"predicted_close"`
	Direction          string   `json:"direction,omitempty"`
	DirectionProbUp    float64  `json:"direction_prob_up,omitempty"`
	DirectionProbDown  float64  `json:"direction_prob_down,omitempty"`
	DirectionProbFlat  float64  `json:"direction_prob_flat,omitempty"`
	ContributionReason string   `json:"contribution_reason,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

type RegimeSummary struct {
	VolatilityRegime string   `json:"volatility_regime,omitempty"`
	TrendRegime      string   `json:"trend_regime,omitempty"`
	MarketRegime     string   `json:"market_regime,omitempty"`
	RiskMultiplier   float64  `json:"risk_multiplier,omitempty"`
	PositionScale    float64  `json:"position_scale,omitempty"`
	Reasons          []string `json:"reasons,omitempty"`
}

type MetaLabelSummary struct {
	Label       int      `json:"label"`
	Allowed     bool     `json:"allowed"`
	Probability float64  `json:"probability"`
	Method      string   `json:"method,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type ForecastReport struct {
	Enabled                bool                    `json:"enabled"`
	ShadowMode             bool                    `json:"shadow_mode"`
	ModelName              string                  `json:"model_name,omitempty"`
	ModelVersion           string                  `json:"model_version,omitempty"`
	FeatureSetVersion      string                  `json:"feature_set_version,omitempty"`
	PredictedOpen          float64                 `json:"predicted_open,omitempty"`
	PredictedClose         float64                 `json:"predicted_close,omitempty"`
	ExpectedReturn         float64                 `json:"expected_return,omitempty"`
	Direction              string                  `json:"direction,omitempty"`
	DirectionProbabilities DirectionProbabilities  `json:"direction_probabilities"`
	Quantiles              map[string]float64      `json:"quantiles,omitempty"`
	PredictionInterval     PredictionInterval      `json:"prediction_interval"`
	CalibratedConfidence   float64                 `json:"calibrated_confidence,omitempty"`
	Calibration            CalibrationSummary      `json:"calibration,omitempty"`
	Regime                 RegimeSummary           `json:"regime,omitempty"`
	MetaLabel              MetaLabelSummary        `json:"meta_label,omitempty"`
	ModelContributions     []ModelContribution     `json:"model_contributions,omitempty"`
	ValidationSummary      ReportValidationSummary `json:"validation_summary"`
	TradeGate              ReportTradeGate         `json:"trade_gate"`
	DataQuality            ReportDataQuality       `json:"data_quality"`
	Fallback               ReportFallback          `json:"fallback"`
	Warnings               []string                `json:"warnings,omitempty"`
	Debug                  map[string]any          `json:"debug,omitempty"`
}

func SanitizePrediction(p ModelPrediction, fv features.FeatureVector) ModelPrediction {
	if p.Symbol == "" {
		p.Symbol = fv.Symbol
	}
	if p.AsOf.IsZero() {
		p.AsOf = fv.AsOf
	}
	if p.FeatureSetVersion == "" {
		p.FeatureSetVersion = fv.FeatureSetVersion
	}
	if p.Quantiles == nil {
		p.Quantiles = map[string]float64{}
	}
	if p.Debug == nil {
		p.Debug = map[string]any{}
	}
	values := []*float64{
		&p.PredictedOpen, &p.PredictedClose, &p.ExpectedReturn,
		&p.DirectionProbUp, &p.DirectionProbDown, &p.DirectionProbFlat,
		&p.Confidence, &p.CalibratedConfidence, &p.PredictionIntervalLow, &p.PredictionIntervalHigh,
	}
	for _, value := range values {
		if math.IsNaN(*value) || math.IsInf(*value, 0) {
			*value = 0
			p.Warnings = append(p.Warnings, "non_finite_prediction_value_clipped")
		}
	}
	total := p.DirectionProbUp + p.DirectionProbDown + p.DirectionProbFlat
	if total > 0 {
		p.DirectionProbUp /= total
		p.DirectionProbDown /= total
		p.DirectionProbFlat /= total
	} else if p.Direction != "" {
		switch p.Direction {
		case "up":
			p.DirectionProbUp = 0.55
			p.DirectionProbFlat = 0.25
			p.DirectionProbDown = 0.20
		case "down":
			p.DirectionProbDown = 0.55
			p.DirectionProbFlat = 0.25
			p.DirectionProbUp = 0.20
		default:
			p.DirectionProbFlat = 0.50
			p.DirectionProbUp = 0.25
			p.DirectionProbDown = 0.25
		}
	}
	if p.CalibratedConfidence == 0 {
		p.CalibratedConfidence = p.Confidence
	}
	return p
}
