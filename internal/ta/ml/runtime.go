package ml

import (
	"path/filepath"
	"strings"
)

type RuntimeModelSelection struct {
	Models       []ForecastModel   `json:"-"`
	Artifact     ModelArtifact     `json:"artifact,omitempty"`
	Selection    ArtifactSelection `json:"selection"`
	Fallback     ReportFallback    `json:"fallback"`
	Warnings     []string          `json:"warnings,omitempty"`
	RegistryPath string            `json:"registry_path,omitempty"`
}

func SelectRuntimeModels(dataDir string, cfg RuntimeConfig, includeBaselines bool) RuntimeModelSelection {
	out := RuntimeModelSelection{}
	if includeBaselines {
		out.Models = append(out.Models, BaselineModels()...)
	}
	registryPath := strings.TrimSpace(cfg.ML.RegistryPath)
	if registryPath == "" {
		if strings.TrimSpace(dataDir) == "" {
			dataDir = "data"
		}
		registryPath = filepath.Join(dataDir, "ml", "registry.json")
	}
	out.RegistryPath = registryPath
	registry, err := LoadRegistry(registryPath)
	if err != nil {
		out.Fallback = ReportFallback{Used: true, Reason: "model_registry_unavailable"}
		out.Warnings = append(out.Warnings, err.Error())
		return out
	}
	out.Selection = registry.Select(cfg.ML.DefaultModel, cfg.ML.FeatureSetVersion, ArtifactProduction, ArtifactShadow)
	if !out.Selection.Found {
		out.Fallback = ReportFallback{Used: true, Reason: "production_model_artifact_missing_using_go_baseline_shadow"}
		out.Warnings = append(out.Warnings, out.Selection.Reasons...)
		return out
	}
	model, err := AdapterFromArtifact(out.Selection.Selected)
	if err != nil {
		out.Fallback = ReportFallback{Used: true, Reason: "artifact_adapter_unavailable_using_go_baseline_shadow"}
		out.Warnings = append(out.Warnings, err.Error())
		return out
	}
	out.Artifact = out.Selection.Selected
	out.Models = append(out.Models, model)
	return out
}
