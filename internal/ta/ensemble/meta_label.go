package ensemble

import (
	"fmt"
	"math"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/ml"
)

type MetaLabelConfig struct {
	MinProbability      float64
	TransactionCostPct  float64
	MaxIntervalATRWidth float64
}

func DefaultMetaLabelConfig() MetaLabelConfig {
	return MetaLabelConfig{
		MinProbability:      0.55,
		TransactionCostPct:  0.002,
		MaxIntervalATRWidth: 2.5,
	}
}

func EvaluateMetaLabel(pred ml.ModelPrediction, fv features.FeatureVector, regime ml.RegimeSummary, cfg MetaLabelConfig) ml.MetaLabelSummary {
	if cfg.MinProbability <= 0 {
		cfg = DefaultMetaLabelConfig()
	}
	prob := 0.50
	reasons := []string{}
	maxDirProb := math.Max(pred.DirectionProbUp, math.Max(pred.DirectionProbDown, pred.DirectionProbFlat))
	prob += (maxDirProb - 0.50) * 0.55
	prob += math.Min(0.15, math.Max(0, math.Abs(pred.ExpectedReturn)-cfg.TransactionCostPct)*8)
	prob += (fv.Quality.SourceScore - 0.75) * 0.20
	prob -= fv.Quality.MissingRatio * 0.20
	if len(fv.Quality.LeakageFlags) > 0 {
		prob -= 0.35
		reasons = append(reasons, "leakage_flags_present")
	}
	atr := fv.Values["atr14"]
	if atr > 0 && pred.PredictionIntervalHigh > pred.PredictionIntervalLow {
		widthATR := (pred.PredictionIntervalHigh - pred.PredictionIntervalLow) / atr
		if widthATR > cfg.MaxIntervalATRWidth {
			prob -= 0.18
			reasons = append(reasons, "prediction_interval_wide")
		}
	}
	if regime.VolatilityRegime == "crisis" {
		prob -= 0.20
		reasons = append(reasons, "crisis_regime")
	} else if regime.VolatilityRegime == "high_vol" {
		prob -= 0.08
		reasons = append(reasons, "high_vol_regime")
	}
	if fv.Values["material_disclosure_flag"] > 0 || fv.Values["kap_risk_flags"] > 0 {
		prob -= 0.15
		reasons = append(reasons, "kap_event_risk")
	}
	if pred.Direction == "up" && regime.TrendRegime == "trend_down" {
		prob -= 0.10
		reasons = append(reasons, "signal_against_downtrend")
	}
	if pred.Direction == "down" && regime.TrendRegime == "trend_up" {
		prob -= 0.10
		reasons = append(reasons, "signal_against_uptrend")
	}
	prob = math.Max(0.01, math.Min(0.99, prob))
	label := 0
	allowed := prob >= cfg.MinProbability
	if allowed {
		label = 1
	}
	reason := "meta_label_pass"
	if !allowed {
		reason = fmt.Sprintf("meta_label_probability_below_threshold:%.2f<%.2f", prob, cfg.MinProbability)
	}
	return ml.MetaLabelSummary{
		Label:       label,
		Allowed:     allowed,
		Probability: prob,
		Method:      "rule_meta_label_v1",
		Reason:      reason,
		Warnings:    reasons,
	}
}

func ApplyMetaLabelGate(decision TradeGateDecision, meta ml.MetaLabelSummary) TradeGateDecision {
	if meta.Allowed {
		if meta.Probability < 0.62 {
			decision.Confidence = "medium"
		}
		return decision
	}
	decision.Allowed = false
	decision.Action = "no_trade"
	decision.Confidence = "low"
	decision.Reasons = append(decision.Reasons, "meta_label_filtered_trade")
	if meta.Reason != "" {
		decision.Reasons = append(decision.Reasons, meta.Reason)
	}
	decision.RiskWarnings = append(decision.RiskWarnings, meta.Warnings...)
	return decision
}
