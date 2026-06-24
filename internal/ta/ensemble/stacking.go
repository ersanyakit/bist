package ensemble

import (
	"math"

	"hissebot/internal/ta/ml"
)

func WeightedAverage(predictions []ml.ModelPrediction, weights []float64) ml.ModelPrediction {
	if len(predictions) == 0 {
		return ml.ModelPrediction{}
	}
	if len(weights) != len(predictions) {
		weights = make([]float64, len(predictions))
		for i := range weights {
			weights[i] = 1
		}
	}
	out := predictions[0]
	total := 0.0
	out.PredictedOpen = 0
	out.PredictedClose = 0
	out.ExpectedReturn = 0
	out.DirectionProbUp = 0
	out.DirectionProbDown = 0
	out.DirectionProbFlat = 0
	out.PredictionIntervalLow = math.Inf(1)
	out.PredictionIntervalHigh = 0
	out.Quantiles = map[string]float64{}
	for i, pred := range predictions {
		w := weights[i]
		if w <= 0 {
			continue
		}
		total += w
		out.PredictedOpen += pred.PredictedOpen * w
		out.PredictedClose += pred.PredictedClose * w
		out.ExpectedReturn += pred.ExpectedReturn * w
		out.DirectionProbUp += pred.DirectionProbUp * w
		out.DirectionProbDown += pred.DirectionProbDown * w
		out.DirectionProbFlat += pred.DirectionProbFlat * w
		if pred.PredictionIntervalLow > 0 && pred.PredictionIntervalLow < out.PredictionIntervalLow {
			out.PredictionIntervalLow = pred.PredictionIntervalLow
		}
		if pred.PredictionIntervalHigh > out.PredictionIntervalHigh {
			out.PredictionIntervalHigh = pred.PredictionIntervalHigh
		}
		for key, value := range pred.Quantiles {
			out.Quantiles[key] += value * w
		}
	}
	if total == 0 {
		return predictions[0]
	}
	out.PredictedOpen /= total
	out.PredictedClose /= total
	out.ExpectedReturn /= total
	out.DirectionProbUp /= total
	out.DirectionProbDown /= total
	out.DirectionProbFlat /= total
	for key := range out.Quantiles {
		out.Quantiles[key] /= total
	}
	out.Direction = "flat"
	if out.DirectionProbUp >= out.DirectionProbDown && out.DirectionProbUp >= out.DirectionProbFlat {
		out.Direction = "up"
	} else if out.DirectionProbDown >= out.DirectionProbUp && out.DirectionProbDown >= out.DirectionProbFlat {
		out.Direction = "down"
	}
	if math.IsInf(out.PredictionIntervalLow, 1) {
		out.PredictionIntervalLow = 0
	}
	out.ModelName = "ensemble_v1"
	out.ModelVersion = "go_shadow_v1"
	if out.Debug == nil {
		out.Debug = map[string]any{}
	}
	out.Debug["ensemble_members"] = len(predictions)
	return out
}
