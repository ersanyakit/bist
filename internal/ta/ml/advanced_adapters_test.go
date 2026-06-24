package ml

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hissebot/internal/ta/features"
)

func TestInferenceRequestValidationCatchesFutureSourceTimestamp(t *testing.T) {
	asOf := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	fv := features.EmptyFeatureVector("ASELS", asOf, "1d", features.DefaultFeatureSetVersion)
	fv.Quality.MissingRatio = 0.10
	fv.Quality.SourceScore = 0.90
	fv.SourceTimestamps["kap"] = asOf.Add(2 * time.Hour)

	req := NewInferenceRequest(fv, ModelArtifact{
		ModelName:         "lightgbm",
		Version:           "test",
		FeatureSetVersion: features.DefaultFeatureSetVersion,
		Adapter:           "external_cli",
	})

	if !req.PointInTimeRequired || !req.NoInvestmentAdvice {
		t.Fatalf("contract flags not set: %+v", req)
	}
	if !hasWarning(req.Warnings, "future_source_timestamp:kap") {
		t.Fatalf("expected future timestamp warning, got %+v", req.Warnings)
	}
}

func TestAdapterFromArtifactReturnsTypedAdvancedAdapterAndGracefulError(t *testing.T) {
	artifact := ModelArtifact{
		ModelName:         "lightgbm",
		Version:           "v1",
		FeatureSetVersion: features.DefaultFeatureSetVersion,
		Adapter:           "external_cli",
		ArtifactPath:      filepath.Join(t.TempDir(), "missing.bin"),
		Status:            ArtifactShadow,
		Metadata: map[string]any{
			"command":        "forecast-model",
			"payload_format": "contract",
		},
	}
	model, err := AdapterFromArtifact(artifact)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}
	if _, ok := model.(LightGBMAdapter); !ok {
		t.Fatalf("expected LightGBMAdapter, got %T", model)
	}
	fv := features.EmptyFeatureVector("ASELS", time.Now().UTC(), "1d", features.DefaultFeatureSetVersion)
	_, err = model.Predict(context.Background(), fv)
	if err == nil || !strings.Contains(err.Error(), "artifact unavailable") {
		t.Fatalf("expected graceful missing artifact error, got %v", err)
	}
}

func TestONNXAdvancedAdapterPlaceholderDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "model.onnx")
	if err := os.WriteFile(artifactPath, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact := ModelArtifact{
		ModelName:         "tft",
		Version:           "v1",
		FeatureSetVersion: features.DefaultFeatureSetVersion,
		Adapter:           "onnx",
		ArtifactPath:      artifactPath,
		Status:            ArtifactShadow,
	}
	model, err := AdapterFromArtifact(artifact)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}
	if _, ok := model.(TFTAdapter); !ok {
		t.Fatalf("expected TFTAdapter, got %T", model)
	}
	fv := features.EmptyFeatureVector("ASELS", time.Now().UTC(), "1d", features.DefaultFeatureSetVersion)
	_, err = model.Predict(context.Background(), fv)
	if err == nil || !strings.Contains(err.Error(), "runtime is not linked") {
		t.Fatalf("expected ONNX placeholder error, got %v", err)
	}
}

func TestScaffoldAdvancedArtifactsWritesDisabledRegistry(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "registry_template.json")
	reg, err := ScaffoldAdvancedArtifacts(registryPath, features.DefaultFeatureSetVersion)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if len(reg.Artifacts) == 0 {
		t.Fatal("expected advanced artifact templates")
	}
	for _, artifact := range reg.Artifacts {
		if artifact.Status != ArtifactDisabled {
			t.Fatalf("artifact should be disabled: %+v", artifact)
		}
		if artifact.Metadata["inference_schema_version"] != InferenceSchemaVersion {
			t.Fatalf("missing contract metadata for %s", artifact.ModelName)
		}
	}
	loaded, err := LoadRegistry(registryPath)
	if err != nil {
		t.Fatalf("load scaffold registry: %v", err)
	}
	health := loaded.Health(features.DefaultFeatureSetVersion)
	if len(health) != len(reg.Artifacts) {
		t.Fatalf("health count mismatch")
	}
	if health[0].Usable {
		t.Fatalf("disabled placeholder must not be usable: %+v", health[0])
	}
	if !hasWarning(health[0].Warnings, "artifact_disabled") {
		t.Fatalf("expected disabled warning, got %+v", health[0].Warnings)
	}
	if _, err := os.Stat(filepath.Join(dir, "artifact_metadata", reg.Artifacts[0].ModelName+".artifact.json")); err != nil {
		t.Fatalf("metadata template missing: %v", err)
	}
}

func TestValidateInferenceResponseFlagsContractMismatch(t *testing.T) {
	asOf := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	fv := features.EmptyFeatureVector("ASELS", asOf, "1d", features.DefaultFeatureSetVersion)
	req := NewInferenceRequest(fv, ModelArtifact{ModelName: "xgboost", Version: "v1", FeatureSetVersion: features.DefaultFeatureSetVersion})
	resp := InferenceResponse{
		SchemaVersion: "old_schema",
		RequestID:     req.RequestID + "_different",
		Prediction: ModelPrediction{
			Symbol:            "THYAO",
			FeatureSetVersion: "other_features",
			PredictedClose:    100,
		},
	}
	warnings := ValidateInferenceResponse(resp, req)
	for _, want := range []string{"response_schema_version_mismatch", "response_request_id_mismatch", "response_symbol_mismatch", "response_feature_set_version_mismatch"} {
		if !hasWarning(warnings, want) {
			t.Fatalf("missing warning %s in %+v", want, warnings)
		}
	}
}

func hasWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if warning == want {
			return true
		}
	}
	return false
}
