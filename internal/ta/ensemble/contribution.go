package ensemble

import (
	"math"
	"strings"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/ml"
)

func modelWeight(pred ml.ModelPrediction, fv features.FeatureVector, regime ml.RegimeSummary) (float64, string) {
	name := strings.ToLower(pred.ModelName)
	weight := 0.75
	reason := "baseline_default"
	switch {
	case strings.HasPrefix(name, "deterministic_"):
		weight = 1.50
		reason = "deterministic_engine_anchor"
	case strings.Contains(name, "previous_close"):
		weight = 0.35
		reason = "naive_stability_baseline"
	case strings.Contains(name, "ewma"):
		weight = 0.80
		reason = "recent_return_baseline"
	case strings.Contains(name, "moving_average"):
		weight = 0.65
		reason = "mean_reversion_baseline"
	case strings.Contains(name, "atr_range"):
		weight = 0.60
		reason = "range_risk_baseline"
	case strings.Contains(name, "ridge"):
		weight = 0.90
		reason = "linear_feature_model"
	case strings.Contains(name, "logistic"):
		weight = 0.85
		reason = "direction_classifier"
	case !strings.Contains(name, "baseline"):
		weight = 1.15
		reason = "registry_external_or_artifact_model"
	}
	if pred.Confidence > 0 {
		weight *= math.Max(0.5, math.Min(1.25, 0.75+pred.Confidence*0.5))
	}
	if len(pred.Warnings) > 0 {
		weight *= 0.85
		reason += "_warnings_discount"
	}
	if regime.RiskMultiplier > 0 {
		weight *= regime.RiskMultiplier
	}
	if fv.Quality.SourceScore > 0 && fv.Quality.SourceScore < 0.75 {
		weight *= math.Max(0.35, fv.Quality.SourceScore)
		reason += "_data_quality_discount"
	}
	if weight < 0 {
		weight = 0
	}
	return weight, reason
}

func contributionFromPrediction(pred ml.ModelPrediction, weight float64, reason string) ml.ModelContribution {
	return ml.ModelContribution{
		ModelName:          pred.ModelName,
		ModelVersion:       pred.ModelVersion,
		Family:             contributionFamily(pred.ModelName),
		Weight:             weight,
		ExpectedReturn:     pred.ExpectedReturn,
		PredictedClose:     pred.PredictedClose,
		Direction:          pred.Direction,
		DirectionProbUp:    pred.DirectionProbUp,
		DirectionProbDown:  pred.DirectionProbDown,
		DirectionProbFlat:  pred.DirectionProbFlat,
		ContributionReason: reason,
		Warnings:           pred.Warnings,
	}
}

func normalizeContributions(contributions []ml.ModelContribution) []ml.ModelContribution {
	total := 0.0
	for _, contribution := range contributions {
		total += contribution.Weight
	}
	if total <= 0 {
		return contributions
	}
	for i := range contributions {
		contributions[i].NormalizedWeight = contributions[i].Weight / total
	}
	return contributions
}

func contributionFamily(modelName string) string {
	name := strings.ToLower(modelName)
	switch {
	case strings.HasPrefix(name, "deterministic_"):
		return "deterministic"
	case strings.Contains(name, "baseline") || strings.Contains(name, "ewma") || strings.Contains(name, "moving_average") || strings.Contains(name, "atr_range"):
		return "baseline"
	case strings.Contains(name, "ridge") || strings.Contains(name, "logistic"):
		return "go_ml"
	case strings.Contains(name, "lob") || strings.Contains(name, "micro"):
		return "microstructure"
	case strings.Contains(name, "tft") || strings.Contains(name, "transformer") || strings.Contains(name, "tcn") || strings.Contains(name, "lstm"):
		return "sequence"
	default:
		return "external"
	}
}
