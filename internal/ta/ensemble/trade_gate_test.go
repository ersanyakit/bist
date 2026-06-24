package ensemble

import (
	"context"
	"testing"
	"time"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/ml"
	"hissebot/internal/ta/validation"
)

func TestTradeGateBlocksLowQualityData(t *testing.T) {
	fv := features.EmptyFeatureVector("ASELS", time.Now().UTC(), "1d", features.DefaultFeatureSetVersion)
	fv.Quality.SourceScore = 0.1
	fv.Quality.MissingRatio = 0.9
	pred := ml.ModelPrediction{Direction: "up", ExpectedReturn: 0.05, DirectionProbUp: 0.8, PredictedClose: 105, PredictionIntervalLow: 100, PredictionIntervalHigh: 106}
	decision := EvaluateTradeGate(pred, fv, validation.Metrics{}, DefaultTradeGateConfig())
	if decision.Allowed || decision.Action != "no_trade" {
		t.Fatalf("expected no_trade, got %+v", decision)
	}
}

func TestEnsembleFallsBackWhenNoPredictions(t *testing.T) {
	fv := features.EmptyFeatureVector("ASELS", time.Now().UTC(), "1d", features.DefaultFeatureSetVersion)
	fv.Values["last_close"] = 100
	result := RunShadow(nilContext{}, fv, DeterministicInput{PredictedOpen: 101, PredictedClose: 102, Direction: "up", Confidence: 60}, []ml.ForecastModel{}, ml.DefaultRuntimeConfig())
	if result.Prediction.PredictedClose <= 0 {
		t.Fatalf("prediction missing: %+v", result)
	}
}

func TestRunShadowProducesRegimeMetaLabelAndContributions(t *testing.T) {
	fv := features.EmptyFeatureVector("ASELS", time.Now().UTC(), "1d", features.DefaultFeatureSetVersion)
	fv.Values["last_close"] = 100
	fv.Values["last_open"] = 99
	fv.Values["return_1d"] = 0.015
	fv.Values["return_5d"] = 0.03
	fv.Values["return_20d"] = 0.08
	fv.Values["trend_slope_20d"] = 0.004
	fv.Values["atr14"] = 1.5
	fv.Values["volatility_20d"] = 0.012
	fv.Values["rsi14"] = 58
	fv.Quality.SourceScore = 0.95
	fv.Quality.MissingRatio = 0.05
	fv.Quality.IsTradable = true
	result := RunShadow(context.Background(), fv, DeterministicInput{PredictedOpen: 101, PredictedClose: 103, Direction: "up", Confidence: 70}, ml.BaselineModels(), ml.DefaultRuntimeConfig())
	if len(result.Contributions) == 0 {
		t.Fatalf("contributions missing: %+v", result)
	}
	if result.Regime.VolatilityRegime == "" || result.MetaLabel.Method == "" {
		t.Fatalf("phase 3 summaries missing: regime=%+v meta=%+v", result.Regime, result.MetaLabel)
	}
	total := 0.0
	for _, contribution := range result.Contributions {
		total += contribution.NormalizedWeight
	}
	if total < 0.99 || total > 1.01 {
		t.Fatalf("normalized contribution total=%v", total)
	}
}

func TestRunShadowOmitsEmptyDeterministicContributionAndTargetsWeekday(t *testing.T) {
	fv := features.EmptyFeatureVector("ASELS", time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC), "1d", features.DefaultFeatureSetVersion)
	fv.Values["last_close"] = 100
	fv.Values["last_open"] = 100
	fv.Quality.SourceScore = 0.95
	fv.Quality.MissingRatio = 0.05
	fv.Quality.IsTradable = true

	result := RunShadow(context.Background(), fv, DeterministicInput{}, ml.BaselineModels(), ml.DefaultRuntimeConfig())
	for _, contribution := range result.Contributions {
		if contribution.Family == "deterministic" || contribution.ModelName == "deterministic_" {
			t.Fatalf("empty deterministic input should not produce contribution: %+v", result.Contributions)
		}
	}
	if got, want := result.Prediction.TargetSession.Format("2006-01-02"), "2026-06-22"; got != want {
		t.Fatalf("target session=%s, want %s", got, want)
	}
}

func TestMetaLabelBlocksCrisisLowQualityTrade(t *testing.T) {
	fv := features.EmptyFeatureVector("ASELS", time.Now().UTC(), "1d", features.DefaultFeatureSetVersion)
	fv.Values["last_close"] = 100
	fv.Values["atr14"] = 8
	fv.Values["volatility_20d"] = 0.08
	fv.Values["material_disclosure_flag"] = 1
	fv.Quality.SourceScore = 0.4
	fv.Quality.MissingRatio = 0.45
	regime := DetectRegime(fv)
	pred := ml.ModelPrediction{Direction: "up", ExpectedReturn: 0.004, DirectionProbUp: 0.58, PredictionIntervalLow: 80, PredictionIntervalHigh: 120}
	meta := EvaluateMetaLabel(pred, fv, regime, DefaultMetaLabelConfig())
	if meta.Allowed || meta.Label != 0 {
		t.Fatalf("expected meta-label block, got %+v", meta)
	}
	decision := ApplyMetaLabelGate(TradeGateDecision{Allowed: true, Action: "buy", Confidence: "high"}, meta)
	if decision.Allowed || decision.Action != "no_trade" {
		t.Fatalf("meta gate did not block: %+v", decision)
	}
}

type nilContext struct{}

func (nilContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (nilContext) Done() <-chan struct{}       { return nil }
func (nilContext) Err() error                  { return nil }
func (nilContext) Value(key any) any           { return nil }
