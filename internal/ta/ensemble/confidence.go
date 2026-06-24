package ensemble

import (
	"math"

	"hissebot/internal/ta/ml"
)

func CalibrateConfidence(pred ml.ModelPrediction, brier float64) ml.ModelPrediction {
	maxProb := math.Max(pred.DirectionProbUp, math.Max(pred.DirectionProbDown, pred.DirectionProbFlat))
	conf := maxProb
	if brier > 0.25 {
		conf *= 0.75
	}
	if conf < 0.05 {
		conf = 0.05
	}
	if conf > 0.95 {
		conf = 0.95
	}
	pred.Confidence = maxProb
	pred.CalibratedConfidence = conf
	return pred
}
