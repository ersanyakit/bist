package ml

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"hissebot/internal/ta/features"
)

type ArtifactScaffoldOptions struct {
	RegistryPath      string
	ArtifactRoot      string
	FeatureSetVersion string
	Version           string
	TrainWindow       string
	Status            ArtifactStatus
	IncludeNative     bool
}

func BuildArtifactTemplate(spec ModelSpec, opts ArtifactScaffoldOptions) ModelArtifact {
	version := opts.Version
	if version == "" {
		version = "placeholder_v1"
	}
	featureSetVersion := opts.FeatureSetVersion
	if featureSetVersion == "" {
		featureSetVersion = features.DefaultFeatureSetVersion
	}
	status := opts.Status
	if status == "" {
		status = ArtifactDisabled
	}
	root := opts.ArtifactRoot
	if root == "" {
		root = filepath.Join("data", "ml", "artifacts")
	}
	metadata := map[string]any{
		"model_family":             string(spec.Family),
		"description":              spec.Description,
		"placeholder":              true,
		"required_adapter":         spec.Adapter,
		"inference_schema_version": InferenceSchemaVersion,
		"training_schema_version":  TrainingManifestSchemaVersion,
		"adapter_contract":         ContractForSpec(spec),
	}
	if spec.Adapter == "external_cli" {
		metadata["command"] = ""
		metadata["args"] = []string{"--artifact", "{artifact}"}
		metadata["payload_format"] = "contract"
	}
	if spec.Adapter == "onnx" {
		metadata["onnx_runtime"] = "not_linked_in_go_build"
	}
	artifact := ModelArtifact{
		ModelName:         spec.Name,
		Version:           version,
		FeatureSetVersion: featureSetVersion,
		Adapter:           spec.Adapter,
		TrainDate:         time.Now().UTC(),
		TrainWindow:       opts.TrainWindow,
		ArtifactPath:      filepath.Join(root, spec.Name, "model.placeholder"),
		Status:            status,
		Metadata:          metadata,
	}
	artifact.ConfigHash = HashConfig(metadata)
	artifact.GitCommitHash = CurrentGitCommitHash()
	return artifact
}

func ScaffoldAdvancedArtifacts(path string, featureSetVersion string) (ArtifactRegistry, error) {
	opts := ArtifactScaffoldOptions{
		RegistryPath:      path,
		FeatureSetVersion: featureSetVersion,
		Status:            ArtifactDisabled,
	}
	return ScaffoldArtifacts(opts)
}

func ScaffoldArtifacts(opts ArtifactScaffoldOptions) (ArtifactRegistry, error) {
	registryPath := opts.RegistryPath
	if registryPath == "" {
		registryPath = filepath.Join("data", "ml", "advanced_registry_template.json")
	}
	artifactRoot := opts.ArtifactRoot
	if artifactRoot == "" {
		artifactRoot = filepath.Join(filepath.Dir(registryPath), "artifacts")
	}
	opts.ArtifactRoot = artifactRoot

	reg := ArtifactRegistry{Path: registryPath}
	metadataDir := filepath.Join(filepath.Dir(registryPath), "artifact_metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		return reg, fmt.Errorf("ensure artifact metadata dir: %w", err)
	}
	for _, spec := range DefaultModelZoo() {
		if spec.NativeGo && !opts.IncludeNative {
			continue
		}
		artifact := BuildArtifactTemplate(spec, opts)
		reg.Artifacts = append(reg.Artifacts, artifact)
		if err := WriteArtifact(filepath.Join(metadataDir, spec.Name+".artifact.json"), artifact); err != nil {
			return reg, fmt.Errorf("write artifact template for %s: %w", spec.Name, err)
		}
	}
	reg.sort()
	if err := reg.Save(); err != nil {
		return reg, fmt.Errorf("save artifact scaffold registry: %w", err)
	}
	return reg, nil
}
