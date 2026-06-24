package ml

import (
	"context"
	"testing"
	"time"

	"hissebot/internal/ta/features"
)

func TestBaselineModelsPredictWithoutPanic(t *testing.T) {
	fv := features.EmptyFeatureVector("ASELS", time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC), "1d", features.DefaultFeatureSetVersion)
	fv.Values["last_close"] = 100
	fv.Values["last_open"] = 99
	fv.Values["return_1d"] = 0.01
	fv.Values["return_5d"] = 0.03
	fv.Values["trend_slope_20d"] = 0.002
	fv.Values["atr14"] = 2
	fv.Values["volatility_20d"] = 0.015
	fv.Values["rsi14"] = 55
	models := []ForecastModel{
		PreviousCloseBaseline{},
		NaiveOpenBaseline{},
		EWMAForecast{},
		MovingAverageForecast{},
		ATRRangeForecast{},
		RidgeRegressionForecast{},
		LogisticDirectionClassifier{},
	}
	for _, model := range models {
		pred, err := model.Predict(context.Background(), fv)
		if err != nil {
			t.Fatalf("%s predict: %v", model.Name(), err)
		}
		if pred.PredictedClose <= 0 || pred.ModelName == "" {
			t.Fatalf("%s invalid prediction: %+v", model.Name(), pred)
		}
	}
}

func TestBaselinePredictionTargetsNextWeekdaySession(t *testing.T) {
	fv := features.EmptyFeatureVector("ASELS", time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC), "1d", features.DefaultFeatureSetVersion)
	fv.Values["last_close"] = 100
	pred, err := PreviousCloseBaseline{}.Predict(context.Background(), fv)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}
	if got, want := pred.TargetSession.Format("2006-01-02"), "2026-06-22"; got != want {
		t.Fatalf("target session=%s, want %s", got, want)
	}

	fv.Categorical["asset_type"] = "crypto"
	pred, err = PreviousCloseBaseline{}.Predict(context.Background(), fv)
	if err != nil {
		t.Fatalf("crypto predict: %v", err)
	}
	if got, want := pred.TargetSession.Format("2006-01-02"), "2026-06-20"; got != want {
		t.Fatalf("crypto target session=%s, want %s", got, want)
	}
}

func TestDirectionFromReturnUsesSharedNeutralTolerance(t *testing.T) {
	if got := directionFromReturn(0.0004); got != "flat" {
		t.Fatalf("sub-tolerance direction=%s, want flat", got)
	}
	if got := directionFromReturn(0.0006); got != "up" {
		t.Fatalf("above-tolerance direction=%s, want up", got)
	}
}
