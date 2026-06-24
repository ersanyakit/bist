package ml

import (
	"fmt"
	"strings"
)

type ModelFamily string

const (
	FamilyTabular        ModelFamily = "tabular"
	FamilySequence       ModelFamily = "sequence"
	FamilyMicrostructure ModelFamily = "microstructure"
	FamilyGraph          ModelFamily = "graph"
	FamilyPolicy         ModelFamily = "policy"
	FamilyBaseline       ModelFamily = "baseline"
)

type ModelSpec struct {
	Name        string      `json:"name"`
	Family      ModelFamily `json:"family"`
	Adapter     string      `json:"adapter"`
	NativeGo    bool        `json:"native_go"`
	Description string      `json:"description,omitempty"`
}

func DefaultModelZoo() []ModelSpec {
	return []ModelSpec{
		{Name: "previous_close_baseline", Family: FamilyBaseline, Adapter: "native_go", NativeGo: true},
		{Name: "naive_open_baseline", Family: FamilyBaseline, Adapter: "native_go", NativeGo: true},
		{Name: "ewma_forecast", Family: FamilyBaseline, Adapter: "native_go", NativeGo: true},
		{Name: "moving_average_forecast", Family: FamilyBaseline, Adapter: "native_go", NativeGo: true},
		{Name: "atr_range_forecast", Family: FamilyBaseline, Adapter: "native_go", NativeGo: true},
		{Name: "ridge_regression_forecast", Family: FamilyBaseline, Adapter: "native_go", NativeGo: true},
		{Name: "logistic_direction_classifier", Family: FamilyBaseline, Adapter: "native_go", NativeGo: true},
		{Name: "lightgbm", Family: FamilyTabular, Adapter: "external_cli", Description: "Tabular gradient boosting artifact."},
		{Name: "xgboost", Family: FamilyTabular, Adapter: "external_cli", Description: "Tabular gradient boosting artifact."},
		{Name: "catboost", Family: FamilyTabular, Adapter: "external_cli", Description: "Categorical/tabular boosting artifact."},
		{Name: "tft", Family: FamilySequence, Adapter: "onnx", Description: "Temporal Fusion Transformer inference artifact."},
		{Name: "patchtst", Family: FamilySequence, Adapter: "onnx"},
		{Name: "timesnet", Family: FamilySequence, Adapter: "onnx"},
		{Name: "nbeats", Family: FamilySequence, Adapter: "onnx"},
		{Name: "dlinear", Family: FamilySequence, Adapter: "onnx"},
		{Name: "tcn", Family: FamilySequence, Adapter: "onnx"},
		{Name: "lstm_gru", Family: FamilySequence, Adapter: "onnx"},
		{Name: "itransformer", Family: FamilySequence, Adapter: "onnx"},
		{Name: "deeplob", Family: FamilyMicrostructure, Adapter: "onnx"},
		{Name: "lob_transformer", Family: FamilyMicrostructure, Adapter: "onnx"},
		{Name: "gnn_stock_relation", Family: FamilyGraph, Adapter: "external_cli"},
		{Name: "rl_policy", Family: FamilyPolicy, Adapter: "external_cli"},
	}
}

func LookupModelSpec(name string) (ModelSpec, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, spec := range DefaultModelZoo() {
		if spec.Name == name {
			return spec, true
		}
	}
	return ModelSpec{}, false
}

func AdapterFromArtifact(artifact ModelArtifact) (ForecastModel, error) {
	if spec, ok := LookupModelSpec(artifact.ModelName); ok && !spec.NativeGo {
		return NewAdvancedAdapter(artifact)
	}
	return basicAdapterFromArtifact(artifact)
}

func basicAdapterFromArtifact(artifact ModelArtifact) (ForecastModel, error) {
	adapter := strings.ToLower(strings.TrimSpace(artifact.Adapter))
	if adapter == "" {
		if spec, ok := LookupModelSpec(artifact.ModelName); ok {
			adapter = spec.Adapter
		}
	}
	switch adapter {
	case "json", "json_prediction", "json_prediction_adapter":
		return JSONPredictionAdapter{
			ModelName:    artifact.ModelName,
			ModelVersion: artifact.Version,
			ArtifactPath: artifact.ArtifactPath,
		}, nil
	case "external", "external_cli", "lightgbm_cli", "xgboost_cli", "catboost_cli":
		command := metadataString(artifact.Metadata, "command")
		args := metadataStringSlice(artifact.Metadata, "args")
		return ExternalCLIAdapter{
			ModelName:    artifact.ModelName,
			ModelVersion: artifact.Version,
			ArtifactPath: artifact.ArtifactPath,
			Command:      command,
			Args:         args,
			PayloadFormat: firstNonEmpty(
				metadataString(artifact.Metadata, "payload_format"),
				metadataString(artifact.Metadata, "payload"),
			),
		}, nil
	case "onnx", "onnx_adapter":
		return ONNXAdapter{
			ModelName:    artifact.ModelName,
			ModelVersion: artifact.Version,
			ArtifactPath: artifact.ArtifactPath,
		}, nil
	case "native_go":
		return nativeModelByName(artifact.ModelName)
	default:
		return nil, fmt.Errorf("unsupported artifact adapter %q for model %s", adapter, artifact.ModelName)
	}
}

func nativeModelByName(name string) (ForecastModel, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "previous_close_baseline":
		return PreviousCloseBaseline{}, nil
	case "naive_open_baseline":
		return NaiveOpenBaseline{}, nil
	case "ewma_forecast":
		return EWMAForecast{}, nil
	case "moving_average_forecast":
		return MovingAverageForecast{}, nil
	case "atr_range_forecast":
		return ATRRangeForecast{}, nil
	case "ridge_regression_forecast":
		return RidgeRegressionForecast{}, nil
	case "logistic_direction_classifier":
		return LogisticDirectionClassifier{}, nil
	default:
		return nil, fmt.Errorf("native Go model %q is not registered", name)
	}
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func metadataStringSlice(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return append([]string{}, values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
