package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"hissebot/internal/ta/features"
)

type ExternalCLIAdapter struct {
	ModelName     string
	ModelVersion  string
	ArtifactPath  string
	Command       string
	Args          []string
	PayloadFormat string
}

func (a ExternalCLIAdapter) Name() string {
	if a.ModelName == "" {
		return "external_cli_adapter"
	}
	return a.ModelName
}

func (a ExternalCLIAdapter) Version() string {
	if a.ModelVersion == "" {
		return "external_v1"
	}
	return a.ModelVersion
}

func (a ExternalCLIAdapter) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if a.ArtifactPath == "" {
		return ModelPrediction{}, fmt.Errorf("%s artifact path missing", a.Name())
	}
	if _, err := os.Stat(a.ArtifactPath); err != nil {
		return ModelPrediction{}, fmt.Errorf("%s artifact unavailable: %w", a.Name(), err)
	}
	if strings.TrimSpace(a.Command) == "" {
		return ModelPrediction{}, fmt.Errorf("%s command missing", a.Name())
	}
	payloadValue := any(fv)
	if strings.EqualFold(strings.TrimSpace(a.PayloadFormat), "contract") {
		payloadValue = NewInferenceRequest(fv, ModelArtifact{
			ModelName:         a.Name(),
			Version:           a.Version(),
			FeatureSetVersion: fv.FeatureSetVersion,
			Adapter:           "external_cli",
			ArtifactPath:      a.ArtifactPath,
			Status:            ArtifactShadow,
		})
	}
	payload, err := json.Marshal(payloadValue)
	if err != nil {
		return ModelPrediction{}, fmt.Errorf("marshal external model payload: %w", err)
	}
	cmd := exec.CommandContext(ctx, a.Command, a.Args...)
	for i, arg := range cmd.Args {
		cmd.Args[i] = strings.ReplaceAll(arg, "{artifact}", a.ArtifactPath)
	}
	cmd.Env = append(os.Environ(),
		"FORECAST_MODEL_ARTIFACT="+a.ArtifactPath,
		"FORECAST_MODEL_NAME="+a.Name(),
		"FORECAST_MODEL_VERSION="+a.Version(),
		"FORECAST_MODEL_CONTRACT_SCHEMA="+InferenceSchemaVersion,
		"FORECAST_MODEL_PAYLOAD_FORMAT="+strings.TrimSpace(a.PayloadFormat),
	)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return ModelPrediction{}, fmt.Errorf("external model command failed: %w: %s", err, detail)
		}
		return ModelPrediction{}, fmt.Errorf("external model command failed: %w", err)
	}
	var pred ModelPrediction
	if err := json.Unmarshal(out, &pred); err != nil {
		return ModelPrediction{}, fmt.Errorf("parse external model prediction: %w", err)
	}
	if pred.ModelName == "" {
		pred.ModelName = a.Name()
	}
	if pred.ModelVersion == "" {
		pred.ModelVersion = a.Version()
	}
	return SanitizePrediction(pred, fv), nil
}

type ONNXAdapter struct {
	ModelName    string
	ModelVersion string
	ArtifactPath string
}

func (a ONNXAdapter) Name() string {
	if a.ModelName == "" {
		return "onnx_adapter"
	}
	return a.ModelName
}

func (a ONNXAdapter) Version() string {
	if a.ModelVersion == "" {
		return "onnx_placeholder_v1"
	}
	return a.ModelVersion
}

func (a ONNXAdapter) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("onnx prediction canceled: %w", err)
	}
	if a.ArtifactPath == "" {
		return ModelPrediction{}, fmt.Errorf("%s artifact path missing", a.Name())
	}
	if _, err := os.Stat(a.ArtifactPath); err != nil {
		return ModelPrediction{}, fmt.Errorf("%s artifact unavailable: %w", a.Name(), err)
	}
	return ModelPrediction{}, fmt.Errorf("%s runtime is not linked in Go build", a.Name())
}

type JSONPredictionAdapter struct {
	ModelName    string
	ModelVersion string
	ArtifactPath string
}

func (a JSONPredictionAdapter) Name() string {
	if a.ModelName == "" {
		return "json_prediction_adapter"
	}
	return a.ModelName
}

func (a JSONPredictionAdapter) Version() string {
	if a.ModelVersion == "" {
		return "json_prediction_v1"
	}
	return a.ModelVersion
}

func (a JSONPredictionAdapter) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if err := ctx.Err(); err != nil {
		return ModelPrediction{}, fmt.Errorf("json prediction canceled: %w", err)
	}
	raw, err := os.ReadFile(a.ArtifactPath)
	if err != nil {
		return ModelPrediction{}, fmt.Errorf("%s artifact unavailable: %w", a.Name(), err)
	}
	var pred ModelPrediction
	if err := json.Unmarshal(raw, &pred); err != nil {
		return ModelPrediction{}, fmt.Errorf("parse json prediction artifact: %w", err)
	}
	if pred.ModelName == "" {
		pred.ModelName = a.Name()
	}
	if pred.ModelVersion == "" {
		pred.ModelVersion = a.Version()
	}
	return SanitizePrediction(pred, fv), nil
}
