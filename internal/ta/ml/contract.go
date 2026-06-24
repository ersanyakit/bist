package ml

import (
	"fmt"
	"math"
	"strings"
	"time"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/validation"
)

const (
	InferenceSchemaVersion        = "bist_ml_inference_v1"
	TrainingManifestSchemaVersion = "bist_ml_training_manifest_v1"
)

type RequiredOutputs struct {
	Open               bool      `json:"open"`
	Close              bool      `json:"close"`
	Direction          bool      `json:"direction"`
	Quantiles          []float64 `json:"quantiles,omitempty"`
	PredictionInterval bool      `json:"prediction_interval"`
	Confidence         bool      `json:"confidence"`
}

type InferenceRequest struct {
	SchemaVersion         string                 `json:"schema_version"`
	RequestID             string                 `json:"request_id"`
	GeneratedAt           time.Time              `json:"generated_at"`
	Symbol                string                 `json:"symbol"`
	AsOf                  time.Time              `json:"as_of"`
	Horizon               string                 `json:"horizon"`
	FeatureSetVersion     string                 `json:"feature_set_version"`
	FeatureVector         features.FeatureVector `json:"feature_vector"`
	Artifact              ModelArtifact          `json:"artifact,omitempty"`
	RequiredOutputs       RequiredOutputs        `json:"required_outputs"`
	PointInTimeRequired   bool                   `json:"point_in_time_required"`
	NoInvestmentAdvice    bool                   `json:"no_investment_advice"`
	UnsafeDataPolicy      string                 `json:"unsafe_data_policy,omitempty"`
	DeterministicFallback bool                   `json:"deterministic_fallback"`
	Warnings              []string               `json:"warnings,omitempty"`
}

type InferenceResponse struct {
	SchemaVersion string           `json:"schema_version"`
	RequestID     string           `json:"request_id,omitempty"`
	Prediction    ModelPrediction  `json:"prediction"`
	Warnings      []string         `json:"warnings,omitempty"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
	ReceivedAt    time.Time        `json:"received_at,omitempty"`
	Latency       DurationEnvelope `json:"latency,omitempty"`
}

type DurationEnvelope struct {
	Milliseconds float64 `json:"milliseconds,omitempty"`
}

type LeakageControls struct {
	PointInTimeOnly                bool `json:"point_in_time_only"`
	RequireReleaseTimestamps       bool `json:"require_release_timestamps"`
	AllowUnsafeReleaseTimestamps   bool `json:"allow_unsafe_release_timestamps"`
	PurgeOverlappingEvents         bool `json:"purge_overlapping_events"`
	EmbargoDays                    int  `json:"embargo_days"`
	RejectFutureSourceTimestamps   bool `json:"reject_future_source_timestamps"`
	RejectSurvivorshipBiasedInputs bool `json:"reject_survivorship_biased_inputs"`
}

type TrainingObjective struct {
	Target string  `json:"target"`
	Loss   string  `json:"loss"`
	Weight float64 `json:"weight,omitempty"`
}

type TrainingManifest struct {
	SchemaVersion      string               `json:"schema_version"`
	GeneratedAt        time.Time            `json:"generated_at"`
	ModelName          string               `json:"model_name"`
	ModelFamily        string               `json:"model_family"`
	Adapter            string               `json:"adapter"`
	FeatureSetVersion  string               `json:"feature_set_version"`
	LabelVersion       string               `json:"label_version,omitempty"`
	TrainWindow        validation.DateRange `json:"train_window"`
	ValidationWindow   validation.DateRange `json:"validation_window"`
	TestWindow         validation.DateRange `json:"test_window"`
	Quantiles          []float64            `json:"quantiles,omitempty"`
	Objectives         []TrainingObjective  `json:"objectives,omitempty"`
	LeakageControls    LeakageControls      `json:"leakage_controls"`
	CostModelVersion   string               `json:"cost_model_version,omitempty"`
	OutputArtifact     ModelArtifact        `json:"output_artifact"`
	NoInvestmentAdvice bool                 `json:"no_investment_advice"`
	Metadata           map[string]any       `json:"metadata,omitempty"`
}

type AdapterCapability struct {
	Adapter                    string   `json:"adapter"`
	ModelFamily                string   `json:"model_family"`
	ExecutionMode              string   `json:"execution_mode"`
	RequiresArtifact           bool     `json:"requires_artifact"`
	SupportsTraining           bool     `json:"supports_training"`
	SupportsDirection          bool     `json:"supports_direction"`
	SupportsQuantiles          bool     `json:"supports_quantiles"`
	SupportsPredictionInterval bool     `json:"supports_prediction_interval"`
	ContractSchema             string   `json:"contract_schema"`
	TrainingManifestSchema     string   `json:"training_manifest_schema"`
	Notes                      []string `json:"notes,omitempty"`
}

type AdapterContract struct {
	ModelName              string            `json:"model_name"`
	ModelFamily            string            `json:"model_family"`
	Adapter                string            `json:"adapter"`
	InferenceSchemaVersion string            `json:"inference_schema_version"`
	TrainingSchemaVersion  string            `json:"training_schema_version"`
	Input                  string            `json:"input"`
	Output                 string            `json:"output"`
	Capability             AdapterCapability `json:"capability"`
}

func NewInferenceRequest(fv features.FeatureVector, artifact ModelArtifact) InferenceRequest {
	req := InferenceRequest{
		SchemaVersion:         InferenceSchemaVersion,
		RequestID:             inferenceRequestID(fv, artifact),
		GeneratedAt:           time.Now().UTC(),
		Symbol:                strings.ToUpper(strings.TrimSpace(fv.Symbol)),
		AsOf:                  fv.AsOf.UTC(),
		Horizon:               strings.TrimSpace(fv.Horizon),
		FeatureSetVersion:     fv.FeatureSetVersion,
		FeatureVector:         fv,
		Artifact:              artifact,
		PointInTimeRequired:   true,
		NoInvestmentAdvice:    true,
		UnsafeDataPolicy:      "warn_and_fallback",
		DeterministicFallback: true,
		RequiredOutputs: RequiredOutputs{
			Open:               true,
			Close:              true,
			Direction:          true,
			Quantiles:          []float64{0.05, 0.10, 0.25, 0.50, 0.75, 0.90, 0.95},
			PredictionInterval: true,
			Confidence:         true,
		},
	}
	req.Warnings = req.Validate()
	return req
}

func (r InferenceRequest) Validate() []string {
	warnings := []string{}
	if r.SchemaVersion != InferenceSchemaVersion {
		warnings = append(warnings, "inference_schema_version_mismatch")
	}
	if strings.TrimSpace(r.Symbol) == "" {
		warnings = append(warnings, "symbol_missing")
	}
	if r.AsOf.IsZero() {
		warnings = append(warnings, "as_of_missing")
	}
	if strings.TrimSpace(r.FeatureSetVersion) == "" {
		warnings = append(warnings, "feature_set_version_missing")
	}
	if r.Artifact.FeatureSetVersion != "" && r.FeatureSetVersion != "" && r.Artifact.FeatureSetVersion != r.FeatureSetVersion {
		warnings = append(warnings, "artifact_feature_set_version_mismatch")
	}
	for source, ts := range r.FeatureVector.SourceTimestamps {
		if !ts.IsZero() && !r.AsOf.IsZero() && ts.After(r.AsOf) {
			warnings = append(warnings, "future_source_timestamp:"+source)
		}
	}
	if len(r.FeatureVector.Quality.LeakageFlags) > 0 {
		warnings = append(warnings, "feature_vector_leakage_flags_present")
	}
	if r.FeatureVector.Quality.MissingRatio >= 0.50 {
		warnings = append(warnings, "feature_vector_missing_ratio_high")
	}
	return warnings
}

func ValidateInferenceResponse(resp InferenceResponse, req InferenceRequest) []string {
	warnings := []string{}
	if resp.SchemaVersion != "" && resp.SchemaVersion != InferenceSchemaVersion {
		warnings = append(warnings, "response_schema_version_mismatch")
	}
	if resp.RequestID != "" && req.RequestID != "" && resp.RequestID != req.RequestID {
		warnings = append(warnings, "response_request_id_mismatch")
	}
	pred := resp.Prediction
	if pred.Symbol != "" && req.Symbol != "" && !strings.EqualFold(pred.Symbol, req.Symbol) {
		warnings = append(warnings, "response_symbol_mismatch")
	}
	if pred.FeatureSetVersion != "" && req.FeatureSetVersion != "" && pred.FeatureSetVersion != req.FeatureSetVersion {
		warnings = append(warnings, "response_feature_set_version_mismatch")
	}
	values := []struct {
		name  string
		value float64
	}{
		{"predicted_open", pred.PredictedOpen},
		{"predicted_close", pred.PredictedClose},
		{"expected_return", pred.ExpectedReturn},
		{"direction_prob_up", pred.DirectionProbUp},
		{"direction_prob_down", pred.DirectionProbDown},
		{"direction_prob_flat", pred.DirectionProbFlat},
		{"confidence", pred.Confidence},
		{"calibrated_confidence", pred.CalibratedConfidence},
	}
	for _, value := range values {
		if math.IsNaN(value.value) || math.IsInf(value.value, 0) {
			warnings = append(warnings, "non_finite_response_value:"+value.name)
		}
	}
	return warnings
}

func DefaultLeakageControls(embargoDays int) LeakageControls {
	if embargoDays < 0 {
		embargoDays = 0
	}
	return LeakageControls{
		PointInTimeOnly:                true,
		RequireReleaseTimestamps:       true,
		AllowUnsafeReleaseTimestamps:   false,
		PurgeOverlappingEvents:         true,
		EmbargoDays:                    embargoDays,
		RejectFutureSourceTimestamps:   true,
		RejectSurvivorshipBiasedInputs: true,
	}
}

func CapabilityForSpec(spec ModelSpec) AdapterCapability {
	adapter := strings.TrimSpace(spec.Adapter)
	if adapter == "" {
		adapter = "external_cli"
	}
	capability := AdapterCapability{
		Adapter:                    adapter,
		ModelFamily:                string(spec.Family),
		RequiresArtifact:           !spec.NativeGo,
		SupportsTraining:           false,
		SupportsDirection:          true,
		SupportsQuantiles:          true,
		SupportsPredictionInterval: true,
		ContractSchema:             InferenceSchemaVersion,
		TrainingManifestSchema:     TrainingManifestSchemaVersion,
	}
	switch adapter {
	case "native_go":
		capability.ExecutionMode = "in_process"
		capability.RequiresArtifact = false
		capability.SupportsTraining = true
	case "onnx":
		capability.ExecutionMode = "onnx_runtime_placeholder"
		capability.Notes = append(capability.Notes, "Go ONNX runtime binding is intentionally not linked in this build.")
	default:
		capability.ExecutionMode = "external_process"
		capability.Notes = append(capability.Notes, "Adapter expects a graceful error when command or artifact is unavailable.")
	}
	return capability
}

func ContractForSpec(spec ModelSpec) AdapterContract {
	return AdapterContract{
		ModelName:              spec.Name,
		ModelFamily:            string(spec.Family),
		Adapter:                spec.Adapter,
		InferenceSchemaVersion: InferenceSchemaVersion,
		TrainingSchemaVersion:  TrainingManifestSchemaVersion,
		Input:                  "features.FeatureVector or InferenceRequest JSON, selected by artifact metadata payload_format",
		Output:                 "ml.ModelPrediction or InferenceResponse JSON",
		Capability:             CapabilityForSpec(spec),
	}
}

func NewTrainingManifest(spec ModelSpec, artifact ModelArtifact, split validation.Split, embargoDays int) TrainingManifest {
	if artifact.ModelName == "" {
		artifact.ModelName = spec.Name
	}
	if artifact.Adapter == "" {
		artifact.Adapter = spec.Adapter
	}
	if artifact.FeatureSetVersion == "" {
		artifact.FeatureSetVersion = features.DefaultFeatureSetVersion
	}
	return TrainingManifest{
		SchemaVersion:      TrainingManifestSchemaVersion,
		GeneratedAt:        time.Now().UTC(),
		ModelName:          artifact.ModelName,
		ModelFamily:        string(spec.Family),
		Adapter:            artifact.Adapter,
		FeatureSetVersion:  artifact.FeatureSetVersion,
		LabelVersion:       "bist_labels_v1",
		TrainWindow:        split.Train,
		ValidationWindow:   split.Validation,
		TestWindow:         split.Test,
		Quantiles:          []float64{0.05, 0.10, 0.25, 0.50, 0.75, 0.90, 0.95},
		Objectives:         defaultObjectives(spec),
		LeakageControls:    DefaultLeakageControls(embargoDays),
		CostModelVersion:   "bist_costs_v1",
		OutputArtifact:     artifact,
		NoInvestmentAdvice: true,
		Metadata: map[string]any{
			"contract": ContractForSpec(spec),
		},
	}
}

func defaultObjectives(spec ModelSpec) []TrainingObjective {
	objectives := []TrainingObjective{
		{Target: "close_to_close_return", Loss: "mae", Weight: 1},
		{Target: "direction", Loss: "logloss", Weight: 0.75},
		{Target: "quantiles", Loss: "pinball", Weight: 0.50},
	}
	if spec.Family == FamilyPolicy {
		objectives = append(objectives, TrainingObjective{Target: "economic_value", Loss: "net_pnl_risk_adjusted", Weight: 1})
	}
	return objectives
}

func inferenceRequestID(fv features.FeatureVector, artifact ModelArtifact) string {
	modelName := artifact.ModelName
	if modelName == "" {
		modelName = "model"
	}
	asOf := fv.AsOf.UTC().Format("20060102T150405Z")
	if fv.AsOf.IsZero() {
		asOf = "no_asof"
	}
	return fmt.Sprintf("%s_%s_%s_%s", strings.ToUpper(strings.TrimSpace(fv.Symbol)), asOf, modelName, strings.TrimSpace(artifact.Version))
}
