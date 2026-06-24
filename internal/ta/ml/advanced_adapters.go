package ml

import (
	"context"
	"fmt"
	"strings"

	"hissebot/internal/ta/features"
)

type AdvancedAdapter struct {
	Spec     ModelSpec
	Artifact ModelArtifact
	Delegate ForecastModel
}

type LightGBMAdapter struct{ AdvancedAdapter }
type XGBoostAdapter struct{ AdvancedAdapter }
type CatBoostAdapter struct{ AdvancedAdapter }
type TFTAdapter struct{ AdvancedAdapter }
type PatchTSTAdapter struct{ AdvancedAdapter }
type TimesNetAdapter struct{ AdvancedAdapter }
type NBEATSAdapter struct{ AdvancedAdapter }
type DLinearAdapter struct{ AdvancedAdapter }
type TCNAdapter struct{ AdvancedAdapter }
type LSTMGRUAdapter struct{ AdvancedAdapter }
type ITransformerAdapter struct{ AdvancedAdapter }
type DeepLOBAdapter struct{ AdvancedAdapter }
type LOBTransformerAdapter struct{ AdvancedAdapter }
type GNNRelationAdapter struct{ AdvancedAdapter }
type RLPolicyAdapter struct{ AdvancedAdapter }

func NewAdvancedAdapter(artifact ModelArtifact) (ForecastModel, error) {
	spec, ok := LookupModelSpec(artifact.ModelName)
	if !ok {
		spec = ModelSpec{
			Name:        strings.ToLower(strings.TrimSpace(artifact.ModelName)),
			Family:      FamilyTabular,
			Adapter:     strings.ToLower(strings.TrimSpace(artifact.Adapter)),
			Description: "User supplied external model artifact.",
		}
	}
	if spec.NativeGo {
		return nativeModelByName(spec.Name)
	}
	if artifact.Adapter == "" {
		artifact.Adapter = spec.Adapter
	}
	delegate, err := basicAdapterFromArtifact(artifact)
	if err != nil {
		return nil, err
	}
	base := AdvancedAdapter{
		Spec:     spec,
		Artifact: artifact,
		Delegate: delegate,
	}
	return typedAdvancedAdapter(spec.Name, base), nil
}

func (a AdvancedAdapter) Name() string {
	if a.Artifact.ModelName != "" {
		return a.Artifact.ModelName
	}
	if a.Spec.Name != "" {
		return a.Spec.Name
	}
	if a.Delegate != nil {
		return a.Delegate.Name()
	}
	return "advanced_adapter"
}

func (a AdvancedAdapter) Version() string {
	if a.Artifact.Version != "" {
		return a.Artifact.Version
	}
	if a.Delegate != nil {
		return a.Delegate.Version()
	}
	return "advanced_adapter_v1"
}

func (a AdvancedAdapter) Predict(ctx context.Context, fv features.FeatureVector) (ModelPrediction, error) {
	if a.Delegate == nil {
		return ModelPrediction{}, fmt.Errorf("%s delegate adapter missing", a.Name())
	}
	req := NewInferenceRequest(fv, a.Artifact)
	pred, err := a.Delegate.Predict(ctx, fv)
	if err != nil {
		return ModelPrediction{}, fmt.Errorf("%s advanced adapter predict: %w", a.Name(), err)
	}
	pred = SanitizePrediction(pred, fv)
	if pred.ModelName == "" {
		pred.ModelName = a.Name()
	}
	if pred.ModelVersion == "" {
		pred.ModelVersion = a.Version()
	}
	if pred.Debug == nil {
		pred.Debug = map[string]any{}
	}
	pred.Debug["adapter"] = a.Artifact.Adapter
	pred.Debug["adapter_family"] = string(a.Spec.Family)
	pred.Debug["adapter_contract_schema"] = InferenceSchemaVersion
	pred.Debug["point_in_time_required"] = req.PointInTimeRequired
	pred.Warnings = append(pred.Warnings, req.Warnings...)
	return SanitizePrediction(pred, fv), nil
}

func (a AdvancedAdapter) Contract() AdapterContract {
	spec := a.Spec
	if spec.Name == "" {
		spec.Name = a.Name()
		spec.Adapter = a.Artifact.Adapter
		spec.Family = FamilyTabular
	}
	return ContractForSpec(spec)
}

func typedAdvancedAdapter(name string, base AdvancedAdapter) ForecastModel {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "lightgbm":
		return LightGBMAdapter{AdvancedAdapter: base}
	case "xgboost":
		return XGBoostAdapter{AdvancedAdapter: base}
	case "catboost":
		return CatBoostAdapter{AdvancedAdapter: base}
	case "tft":
		return TFTAdapter{AdvancedAdapter: base}
	case "patchtst":
		return PatchTSTAdapter{AdvancedAdapter: base}
	case "timesnet":
		return TimesNetAdapter{AdvancedAdapter: base}
	case "nbeats":
		return NBEATSAdapter{AdvancedAdapter: base}
	case "dlinear":
		return DLinearAdapter{AdvancedAdapter: base}
	case "tcn":
		return TCNAdapter{AdvancedAdapter: base}
	case "lstm_gru":
		return LSTMGRUAdapter{AdvancedAdapter: base}
	case "itransformer":
		return ITransformerAdapter{AdvancedAdapter: base}
	case "deeplob":
		return DeepLOBAdapter{AdvancedAdapter: base}
	case "lob_transformer":
		return LOBTransformerAdapter{AdvancedAdapter: base}
	case "gnn_stock_relation":
		return GNNRelationAdapter{AdvancedAdapter: base}
	case "rl_policy":
		return RLPolicyAdapter{AdvancedAdapter: base}
	default:
		return base
	}
}

func IsAdvancedModel(name string) bool {
	spec, ok := LookupModelSpec(name)
	return ok && !spec.NativeGo
}
