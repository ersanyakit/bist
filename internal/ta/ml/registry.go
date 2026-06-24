package ml

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ArtifactRegistry struct {
	Path      string          `json:"path,omitempty"`
	Artifacts []ModelArtifact `json:"artifacts"`
}

type ArtifactHealth struct {
	ModelName         string         `json:"model_name"`
	Version           string         `json:"version"`
	FeatureSetVersion string         `json:"feature_set_version"`
	Status            ArtifactStatus `json:"status"`
	Adapter           string         `json:"adapter,omitempty"`
	ArtifactPath      string         `json:"artifact_path,omitempty"`
	Usable            bool           `json:"usable"`
	Warnings          []string       `json:"warnings,omitempty"`
	TrainDate         time.Time      `json:"train_date,omitempty"`
}

type ArtifactSelection struct {
	Selected ModelArtifact    `json:"selected"`
	Found    bool             `json:"found"`
	Health   []ArtifactHealth `json:"health,omitempty"`
	Reasons  []string         `json:"reasons,omitempty"`
}

func LoadRegistry(path string) (ArtifactRegistry, error) {
	if path == "" {
		path = filepath.Join("data", "ml", "registry.json")
	}
	reg := ArtifactRegistry{Path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return reg, fmt.Errorf("read model registry: %w", err)
	}
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &reg.Artifacts); err != nil {
			return reg, fmt.Errorf("parse model registry: %w", err)
		}
	} else {
		var envelope ArtifactRegistry
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return reg, fmt.Errorf("parse model registry: %w", err)
		}
		reg.Artifacts = envelope.Artifacts
	}
	reg.sort()
	return reg, nil
}

func (r *ArtifactRegistry) Save() error {
	if r.Path == "" {
		r.Path = filepath.Join("data", "ml", "registry.json")
	}
	if err := os.MkdirAll(filepath.Dir(r.Path), 0o755); err != nil {
		return fmt.Errorf("ensure model registry dir: %w", err)
	}
	raw, err := json.MarshalIndent(r.Artifacts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal model registry: %w", err)
	}
	if err := os.WriteFile(r.Path, raw, 0o644); err != nil {
		return fmt.Errorf("write model registry: %w", err)
	}
	return nil
}

func (r *ArtifactRegistry) Register(artifact ModelArtifact) error {
	if artifact.TrainDate.IsZero() {
		artifact.TrainDate = time.Now().UTC()
	}
	replaced := false
	for i, existing := range r.Artifacts {
		if existing.ModelName == artifact.ModelName && existing.Version == artifact.Version && existing.FeatureSetVersion == artifact.FeatureSetVersion {
			r.Artifacts[i] = artifact
			replaced = true
			break
		}
	}
	if !replaced {
		r.Artifacts = append(r.Artifacts, artifact)
	}
	r.sort()
	return r.Save()
}

func (r ArtifactRegistry) Latest(modelName, featureSetVersion string, statuses ...ArtifactStatus) (ModelArtifact, bool) {
	statusOK := map[ArtifactStatus]bool{}
	for _, status := range statuses {
		statusOK[status] = true
	}
	for _, artifact := range r.Artifacts {
		if modelName != "" && artifact.ModelName != modelName {
			continue
		}
		if featureSetVersion != "" && artifact.FeatureSetVersion != featureSetVersion {
			continue
		}
		if len(statusOK) > 0 && !statusOK[artifact.Status] {
			continue
		}
		return artifact, true
	}
	return ModelArtifact{}, false
}

func (r ArtifactRegistry) Select(modelName, featureSetVersion string, statuses ...ArtifactStatus) ArtifactSelection {
	statusOK := map[ArtifactStatus]bool{}
	for _, status := range statuses {
		statusOK[status] = true
	}
	selection := ArtifactSelection{}
	for _, artifact := range r.Artifacts {
		if modelName != "" && artifact.ModelName != modelName {
			continue
		}
		if featureSetVersion != "" && artifact.FeatureSetVersion != featureSetVersion {
			continue
		}
		health := artifactHealth(artifact, featureSetVersion)
		selection.Health = append(selection.Health, health)
		if len(statusOK) > 0 && !statusOK[artifact.Status] {
			selection.Reasons = append(selection.Reasons, artifact.ModelName+":"+string(artifact.Status)+"_status_not_allowed")
			continue
		}
		if !health.Usable {
			selection.Reasons = append(selection.Reasons, artifact.ModelName+":"+strings.Join(health.Warnings, ","))
			continue
		}
		selection.Selected = artifact
		selection.Found = true
		return selection
	}
	if !selection.Found && len(selection.Reasons) == 0 {
		selection.Reasons = append(selection.Reasons, "no_matching_artifact")
	}
	return selection
}

func (r ArtifactRegistry) Health(featureSetVersion string) []ArtifactHealth {
	out := make([]ArtifactHealth, 0, len(r.Artifacts))
	for _, artifact := range r.Artifacts {
		out = append(out, artifactHealth(artifact, featureSetVersion))
	}
	return out
}

func (r *ArtifactRegistry) sort() {
	sort.SliceStable(r.Artifacts, func(i, j int) bool {
		left := r.Artifacts[i]
		right := r.Artifacts[j]
		if left.Status != right.Status {
			return artifactStatusRank(left.Status) < artifactStatusRank(right.Status)
		}
		return left.TrainDate.After(right.TrainDate)
	})
}

func artifactHealth(artifact ModelArtifact, featureSetVersion string) ArtifactHealth {
	warnings := artifact.Validate(featureSetVersion)
	return ArtifactHealth{
		ModelName:         artifact.ModelName,
		Version:           artifact.Version,
		FeatureSetVersion: artifact.FeatureSetVersion,
		Status:            artifact.Status,
		Adapter:           artifact.Adapter,
		ArtifactPath:      artifact.ArtifactPath,
		Usable:            len(warnings) == 0,
		Warnings:          warnings,
		TrainDate:         artifact.TrainDate,
	}
}

func artifactStatusRank(status ArtifactStatus) int {
	switch status {
	case ArtifactProduction:
		return 0
	case ArtifactShadow:
		return 1
	case ArtifactCandidate:
		return 2
	case ArtifactDisabled:
		return 3
	default:
		return 4
	}
}
