package ml

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/validation"
)

func TestRegistrySelectsUsableShadowArtifact(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "prediction.json")
	if err := os.WriteFile(artifactPath, []byte(`{"predicted_close":101,"direction":"up","direction_prob_up":0.7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := ArtifactRegistry{Path: filepath.Join(dir, "registry.json")}
	err := reg.Register(ModelArtifact{
		ModelName:         "ensemble_v1",
		Version:           "test",
		FeatureSetVersion: features.DefaultFeatureSetVersion,
		Adapter:           "json_prediction",
		ArtifactPath:      artifactPath,
		Status:            ArtifactShadow,
		TrainDate:         time.Now().UTC(),
		ValidationMetrics: validation.Metrics{Samples: 10, DirectionAccuracy: 0.60},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	loaded, err := LoadRegistry(reg.Path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	selection := loaded.Select("ensemble_v1", features.DefaultFeatureSetVersion, ArtifactProduction, ArtifactShadow)
	if !selection.Found {
		t.Fatalf("artifact not selected: %+v", selection)
	}
	model, err := AdapterFromArtifact(selection.Selected)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}
	fv := features.EmptyFeatureVector("ASELS", time.Now().UTC(), "1d", features.DefaultFeatureSetVersion)
	fv.Values["last_close"] = 100
	pred, err := model.Predict(context.Background(), fv)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}
	if pred.PredictedClose != 101 || pred.ModelName != "ensemble_v1" {
		t.Fatalf("prediction=%+v", pred)
	}
}

func TestDefaultModelZooIncludesAdvancedHooks(t *testing.T) {
	wants := []string{"lightgbm", "xgboost", "catboost", "tft", "itransformer", "deeplob", "gnn_stock_relation", "rl_policy"}
	for _, want := range wants {
		if _, ok := LookupModelSpec(want); !ok {
			t.Fatalf("model zoo missing %s", want)
		}
	}
}

func TestArtifactValidationReportsMissingFile(t *testing.T) {
	artifact := ModelArtifact{
		ModelName:         "ensemble_v1",
		Version:           "missing",
		FeatureSetVersion: features.DefaultFeatureSetVersion,
		ArtifactPath:      filepath.Join(t.TempDir(), "missing.json"),
		Status:            ArtifactProduction,
	}
	if err := artifact.Usable(features.DefaultFeatureSetVersion); err == nil {
		t.Fatal("expected unusable artifact")
	}
}
