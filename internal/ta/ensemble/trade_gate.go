package ensemble

import (
	"fmt"
	"math"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/ml"
	"hissebot/internal/ta/validation"
)

type TradeGateDecision struct {
	Allowed             bool     `json:"allowed"`
	Action              string   `json:"action"`
	SizeHint            float64  `json:"size_hint"`
	Confidence          string   `json:"confidence"`
	Reasons             []string `json:"reasons,omitempty"`
	RiskWarnings        []string `json:"risk_warnings,omitempty"`
	RequiredDataMissing []string `json:"required_data_missing,omitempty"`
	ModelQualitySummary string   `json:"model_quality_summary,omitempty"`
}

type TradeGateConfig struct {
	MinExpectedReturnAfterCost  float64
	MinDirectionProbability     float64
	MaxIntervalWidthATRMultiple float64
	MinSourceScore              float64
	AllowTradeOnLowQualityData  bool
	TransactionCostPct          float64
}

func DefaultTradeGateConfig() TradeGateConfig {
	return TradeGateConfig{
		MinExpectedReturnAfterCost:  0.003,
		MinDirectionProbability:     0.56,
		MaxIntervalWidthATRMultiple: 2.5,
		MinSourceScore:              0.75,
		TransactionCostPct:          0.002,
	}
}

func EvaluateTradeGate(pred ml.ModelPrediction, fv features.FeatureVector, metrics validation.Metrics, cfg TradeGateConfig) TradeGateDecision {
	if cfg.MinDirectionProbability <= 0 {
		cfg = DefaultTradeGateConfig()
	}
	decision := TradeGateDecision{Allowed: true, Action: "hold", SizeHint: 1, Confidence: "medium"}
	if pred.Direction == "up" {
		decision.Action = "buy"
	} else if pred.Direction == "down" {
		decision.Action = "sell"
	} else {
		decision.Action = "hold"
		decision.Allowed = false
		decision.Reasons = append(decision.Reasons, "flat_direction")
	}
	maxProb := math.Max(pred.DirectionProbUp, math.Max(pred.DirectionProbDown, pred.DirectionProbFlat))
	if maxProb < cfg.MinDirectionProbability {
		decision.Allowed = false
		decision.Reasons = append(decision.Reasons, "direction_probability_below_threshold")
	}
	if math.Abs(pred.ExpectedReturn)-cfg.TransactionCostPct < cfg.MinExpectedReturnAfterCost {
		decision.Allowed = false
		decision.Reasons = append(decision.Reasons, "expected_return_not_above_cost")
	}
	atr := fv.Values["atr14"]
	if atr > 0 && pred.PredictionIntervalHigh > pred.PredictionIntervalLow {
		width := pred.PredictionIntervalHigh - pred.PredictionIntervalLow
		if width/atr > cfg.MaxIntervalWidthATRMultiple {
			decision.Allowed = false
			decision.Reasons = append(decision.Reasons, "prediction_interval_too_wide")
		}
	}
	if fv.Quality.SourceScore < cfg.MinSourceScore && !cfg.AllowTradeOnLowQualityData {
		decision.Allowed = false
		decision.Reasons = append(decision.Reasons, "data_source_score_below_threshold")
	}
	if len(fv.Quality.LeakageFlags) > 0 {
		decision.Allowed = false
		decision.Reasons = append(decision.Reasons, "feature_leakage_flags_present")
		decision.RiskWarnings = append(decision.RiskWarnings, fv.Quality.LeakageFlags...)
	}
	if fv.Quality.MissingRatio > 0.40 && !cfg.AllowTradeOnLowQualityData {
		decision.Allowed = false
		decision.Reasons = append(decision.Reasons, "missing_feature_ratio_high")
	}
	if metrics.Samples > 0 {
		if metrics.DirectionAccuracy > 0 && metrics.DirectionAccuracy < 0.53 {
			decision.Allowed = false
			decision.Reasons = append(decision.Reasons, "validation_direction_accuracy_below_threshold")
		}
		if metrics.MAPE > 0.05 {
			decision.Allowed = false
			decision.Reasons = append(decision.Reasons, "validation_mape_above_threshold")
		}
	}
	if fv.Values["kap_risk_flags"] > 0 || fv.Values["material_disclosure_flag"] > 0 {
		decision.RiskWarnings = append(decision.RiskWarnings, "kap_material_event_risk")
		decision.SizeHint = math.Min(decision.SizeHint, 0.5)
	}
	if fv.Values["volatility_20d"] > 0.05 {
		decision.RiskWarnings = append(decision.RiskWarnings, "crisis_volatility_regime")
		decision.SizeHint = math.Min(decision.SizeHint, 0.35)
	}
	if !decision.Allowed {
		decision.Action = "no_trade"
		decision.Confidence = "low"
	} else if maxProb >= 0.68 && math.Abs(pred.ExpectedReturn) > cfg.MinExpectedReturnAfterCost*2 {
		decision.Confidence = "high"
	}
	if decision.ModelQualitySummary == "" {
		decision.ModelQualitySummary = fmt.Sprintf("prob=%.2f expected_return=%.4f source_score=%.2f", maxProb, pred.ExpectedReturn, fv.Quality.SourceScore)
	}
	return decision
}
