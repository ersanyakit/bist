package ml

import (
	"context"
	"fmt"
	"math"
	"time"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/forecastpolicy"
	"hissebot/internal/ta/labels"
	"hissebot/internal/ta/validation"
)

type TrainingDataset struct {
	Features []features.FeatureVector `json:"features"`
	Targets  []labels.ForecastTarget  `json:"targets"`
}

type ForecastModel interface {
	Name() string
	Version() string
	Predict(ctx context.Context, features features.FeatureVector) (ModelPrediction, error)
}

type TrainableModel interface {
	ForecastModel
	Train(ctx context.Context, dataset TrainingDataset) (ModelArtifact, validation.ValidationReport, error)
}

type DirectionClassifier interface {
	PredictDirection(ctx context.Context, features features.FeatureVector) (DirectionPrediction, error)
}

type QuantileForecaster interface {
	PredictQuantiles(ctx context.Context, features features.FeatureVector, quantiles []float64) (map[string]float64, error)
}

func BaselineModels() []ForecastModel {
	return []ForecastModel{
		PreviousCloseBaseline{},
		EWMAForecast{},
		MovingAverageForecast{},
		ATRRangeForecast{},
		RidgeRegressionForecast{},
		LogisticDirectionClassifier{},
	}
}

type PreviousCloseBaseline struct{}

func (PreviousCloseBaseline) Name() string    { return "previous_close_baseline" }
func (PreviousCloseBaseline) Version() string { return "go_baseline_v1" }

func (m PreviousCloseBaseline) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("previous close predict canceled: %w", err)
	}
	last := fv.Values["last_close"]
	p := basePrediction(m.Name(), m.Version(), fv, last, last, 0)
	p.Direction = "flat"
	p.DirectionProbFlat = 0.50
	p.DirectionProbUp = 0.25
	p.DirectionProbDown = 0.25
	p.Confidence = 0.35
	return SanitizePrediction(p, fv), nil
}

type NaiveOpenBaseline struct{}

func (NaiveOpenBaseline) Name() string    { return "naive_open_baseline" }
func (NaiveOpenBaseline) Version() string { return "go_baseline_v1" }

func (m NaiveOpenBaseline) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("naive open predict canceled: %w", err)
	}
	last := fv.Values["last_close"]
	gap := fv.Values["gap_1d"]
	open := last * (1 + clamp(gap, -0.05, 0.05))
	p := basePrediction(m.Name(), m.Version(), fv, open, last, safeReturn(last, open))
	p.Direction = directionFromReturn(p.ExpectedReturn)
	p.DirectionProbUp, p.DirectionProbDown, p.DirectionProbFlat = probabilitiesFromReturn(p.ExpectedReturn)
	p.Confidence = 0.38
	return SanitizePrediction(p, fv), nil
}

type EWMAForecast struct {
	Alpha float64
}

func (EWMAForecast) Name() string    { return "ewma_forecast" }
func (EWMAForecast) Version() string { return "go_baseline_v1" }

func (m EWMAForecast) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("ewma predict canceled: %w", err)
	}
	alpha := m.Alpha
	if alpha <= 0 || alpha > 1 {
		alpha = 0.35
	}
	ret := alpha*fv.Values["return_1d"] + (1-alpha)*fv.Values["return_5d"]/5
	last := fv.Values["last_close"]
	close := last * (1 + clamp(ret, -0.08, 0.08))
	p := basePrediction(m.Name(), m.Version(), fv, last, close, ret)
	p.Direction = directionFromReturn(ret)
	p.DirectionProbUp, p.DirectionProbDown, p.DirectionProbFlat = probabilitiesFromReturn(ret)
	p.Confidence = 0.45
	p.Quantiles = residualQuantiles(ret, fv.Values["volatility_20d"])
	return SanitizePrediction(p, fv), nil
}

type MovingAverageForecast struct {
	Window int
}

func (MovingAverageForecast) Name() string    { return "moving_average_forecast" }
func (MovingAverageForecast) Version() string { return "go_baseline_v1" }

func (m MovingAverageForecast) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("moving average predict canceled: %w", err)
	}
	last := fv.Values["last_close"]
	maDistance := fv.Values["sma20_distance"]
	ret := clamp(-maDistance*0.10+fv.Values["trend_slope_20d"], -0.06, 0.06)
	close := last * (1 + ret)
	p := basePrediction(m.Name(), m.Version(), fv, last, close, ret)
	p.Direction = directionFromReturn(ret)
	p.DirectionProbUp, p.DirectionProbDown, p.DirectionProbFlat = probabilitiesFromReturn(ret)
	p.Confidence = 0.42
	return SanitizePrediction(p, fv), nil
}

type ATRRangeForecast struct{}

func (ATRRangeForecast) Name() string    { return "atr_range_forecast" }
func (ATRRangeForecast) Version() string { return "go_baseline_v1" }

func (m ATRRangeForecast) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("atr range predict canceled: %w", err)
	}
	last := fv.Values["last_close"]
	atr := fv.Values["atr14"]
	ret := clamp(fv.Values["trend_slope_20d"], -0.04, 0.04)
	close := last * (1 + ret)
	p := basePrediction(m.Name(), m.Version(), fv, last, close, ret)
	p.Direction = directionFromReturn(ret)
	p.DirectionProbUp, p.DirectionProbDown, p.DirectionProbFlat = probabilitiesFromReturn(ret)
	p.Confidence = 0.40
	p.PredictionIntervalLow = math.Max(0, close-atr)
	p.PredictionIntervalHigh = close + atr
	return SanitizePrediction(p, fv), nil
}

type RidgeRegressionForecast struct {
	Weights map[string]float64 `json:"weights,omitempty"`
	Bias    float64            `json:"bias,omitempty"`
	Lambda  float64            `json:"lambda,omitempty"`
}

func (RidgeRegressionForecast) Name() string    { return "ridge_regression_forecast" }
func (RidgeRegressionForecast) Version() string { return "go_linear_v1" }

func (m RidgeRegressionForecast) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("ridge predict canceled: %w", err)
	}
	ret := m.Bias
	if len(m.Weights) == 0 {
		ret = 0.35*fv.Values["return_1d"] + 0.20*fv.Values["return_5d"]/5 + 0.15*fv.Values["trend_slope_20d"] - 0.05*fv.Values["volume_zscore20"]
	} else {
		for name, weight := range m.Weights {
			ret += weight * fv.Values[name]
		}
	}
	ret = clamp(ret, -0.10, 0.10)
	last := fv.Values["last_close"]
	p := basePrediction(m.Name(), m.Version(), fv, last, last*(1+ret), ret)
	p.Direction = directionFromReturn(ret)
	p.DirectionProbUp, p.DirectionProbDown, p.DirectionProbFlat = probabilitiesFromReturn(ret)
	p.Confidence = 0.50
	return SanitizePrediction(p, fv), nil
}

func (m RidgeRegressionForecast) Train(ctx context.Context, dataset TrainingDataset) (ModelArtifact, validation.ValidationReport, error) {
	if err := ctx.Err(); err != nil {
		return ModelArtifact{}, validation.ValidationReport{}, fmt.Errorf("ridge train canceled: %w", err)
	}
	featuresUsed := []string{"return_1d", "return_5d", "trend_slope_20d", "volume_zscore20"}
	weights := map[string]float64{}
	for _, name := range featuresUsed {
		weights[name] = 0
	}
	lr := 0.01
	lambda := m.Lambda
	if lambda <= 0 {
		lambda = 0.01
	}
	for epoch := 0; epoch < 200; epoch++ {
		for i, fv := range dataset.Features {
			if i >= len(dataset.Targets) {
				break
			}
			pred := 0.0
			for _, name := range featuresUsed {
				pred += weights[name] * fv.Values[name]
			}
			err := pred - dataset.Targets[i].LogReturn
			for _, name := range featuresUsed {
				weights[name] -= lr * (err*fv.Values[name] + lambda*weights[name])
			}
		}
	}
	m.Weights = weights
	report := validation.ValidationReport{ModelName: m.Name(), ModelVersion: m.Version(), FeatureSetVersion: features.DefaultFeatureSetVersion, Passed: true}
	artifact := ModelArtifact{ModelName: m.Name(), Version: m.Version(), FeatureSetVersion: features.DefaultFeatureSetVersion, TrainDate: time.Now().UTC(), Status: ArtifactCandidate}
	return artifact, report, nil
}

type LogisticDirectionClassifier struct {
	Weights map[string]float64 `json:"weights,omitempty"`
	Bias    float64            `json:"bias,omitempty"`
}

func (LogisticDirectionClassifier) Name() string    { return "logistic_direction_classifier" }
func (LogisticDirectionClassifier) Version() string { return "go_logistic_v1" }

func (m LogisticDirectionClassifier) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("logistic predict canceled: %w", err)
	}
	dp, err := m.PredictDirection(ctx, fv)
	if err != nil {
		return ModelPrediction{}, err
	}
	last := fv.Values["last_close"]
	ret := fv.Values["return_1d"] * 0.25
	if dp.Direction == "down" {
		ret = -math.Abs(ret)
	} else if dp.Direction == "up" {
		ret = math.Abs(ret)
	} else {
		ret = 0
	}
	p := basePrediction(m.Name(), m.Version(), fv, last, last*(1+ret), ret)
	p.Direction = dp.Direction
	p.DirectionProbUp = dp.ProbUp
	p.DirectionProbDown = dp.ProbDown
	p.DirectionProbFlat = dp.ProbFlat
	p.Confidence = math.Max(dp.ProbUp, math.Max(dp.ProbDown, dp.ProbFlat))
	return SanitizePrediction(p, fv), nil
}

func (m LogisticDirectionClassifier) PredictDirection(ctx context.Context, fv features.FeatureVector) (DirectionPrediction, error) {
	if err := ctx.Err(); err != nil {
		return DirectionPrediction{}, fmt.Errorf("logistic direction canceled: %w", err)
	}
	score := m.Bias
	if len(m.Weights) == 0 {
		score = 3*fv.Values["return_1d"] + 1.5*fv.Values["trend_slope_20d"] + 0.01*(fv.Values["rsi14"]-50) + 0.5*fv.Values["macd_histogram"]
	} else {
		for name, weight := range m.Weights {
			score += weight * fv.Values[name]
		}
	}
	up := sigmoid(score)
	down := sigmoid(-score)
	flat := math.Max(0.05, 1-math.Abs(up-down))
	total := up + down + flat
	dp := DirectionPrediction{ProbUp: up / total, ProbDown: down / total, ProbFlat: flat / total}
	dp.Direction = "flat"
	if dp.ProbUp >= dp.ProbDown && dp.ProbUp >= dp.ProbFlat {
		dp.Direction = "up"
	} else if dp.ProbDown >= dp.ProbUp && dp.ProbDown >= dp.ProbFlat {
		dp.Direction = "down"
	}
	return dp, nil
}

func basePrediction(name, version string, fv features.FeatureVector, open, close, ret float64) ModelPrediction {
	if close <= 0 {
		close = fv.Values["last_close"]
	}
	if open <= 0 {
		open = fv.Values["last_close"]
	}
	vol := math.Max(fv.Values["volatility_20d"], safeDiv(fv.Values["atr14"], fv.Values["last_close"]))
	span := math.Max(fv.Values["atr14"], math.Abs(close-open))
	return ModelPrediction{
		Symbol:                 fv.Symbol,
		AsOf:                   fv.AsOf,
		TargetSession:          forecastpolicy.NextSessionForAssetType(fv.AsOf, fv.Categorical["asset_type"]),
		ModelName:              name,
		ModelVersion:           version,
		FeatureSetVersion:      fv.FeatureSetVersion,
		PredictedOpen:          open,
		PredictedClose:         close,
		ExpectedReturn:         ret,
		Quantiles:              residualQuantiles(ret, vol),
		PredictionIntervalLow:  math.Max(0, close-span),
		PredictionIntervalHigh: close + span,
		Debug:                  map[string]any{"baseline": true},
	}
}

func residualQuantiles(center, vol float64) map[string]float64 {
	if vol <= 0 {
		vol = 0.01
	}
	return map[string]float64{
		"p05": center - 1.65*vol,
		"p10": center - 1.28*vol,
		"p25": center - 0.67*vol,
		"p50": center,
		"p75": center + 0.67*vol,
		"p90": center + 1.28*vol,
		"p95": center + 1.65*vol,
	}
}

func probabilitiesFromReturn(ret float64) (float64, float64, float64) {
	score := clamp(math.Abs(ret)*20, 0, 0.35)
	if ret > forecastpolicy.NextSessionDirectionToleranceReturn() {
		return 0.40 + score, 0.25 - score/2, 0.35 - score/2
	}
	if ret < -forecastpolicy.NextSessionDirectionToleranceReturn() {
		return 0.25 - score/2, 0.40 + score, 0.35 - score/2
	}
	return 0.30, 0.30, 0.40
}

func directionFromReturn(ret float64) string {
	return forecastpolicy.DirectionFromReturn(ret)
}

func sigmoid(x float64) float64 {
	if x > 40 {
		return 1
	}
	if x < -40 {
		return 0
	}
	return 1 / (1 + math.Exp(-x))
}

func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func safeReturn(base, value float64) float64 {
	if base <= 0 {
		return 0
	}
	return value/base - 1
}

func safeDiv(a, b float64) float64 {
	if b == 0 || math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return 0
	}
	return a / b
}
