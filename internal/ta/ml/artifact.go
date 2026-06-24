package ml

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"hissebot/internal/ta/validation"
)

type ArtifactStatus string

const (
	ArtifactCandidate  ArtifactStatus = "candidate"
	ArtifactShadow     ArtifactStatus = "shadow"
	ArtifactProduction ArtifactStatus = "production"
	ArtifactDisabled   ArtifactStatus = "disabled"
)

type ModelArtifact struct {
	ModelName         string             `json:"model_name"`
	Version           string             `json:"version"`
	FeatureSetVersion string             `json:"feature_set_version"`
	Adapter           string             `json:"adapter,omitempty"`
	TrainDate         time.Time          `json:"train_date"`
	TrainWindow       string             `json:"train_window,omitempty"`
	ValidationMetrics validation.Metrics `json:"validation_metrics"`
	ArtifactPath      string             `json:"artifact_path"`
	ConfigHash        string             `json:"config_hash,omitempty"`
	GitCommitHash     string             `json:"git_commit_hash,omitempty"`
	Status            ArtifactStatus     `json:"status"`
	Metadata          map[string]any     `json:"metadata,omitempty"`
}

type RuntimeConfig struct {
	ML struct {
		Enabled                 bool   `json:"enabled"`
		ShadowMode              bool   `json:"shadow_mode"`
		FeatureSetVersion       string `json:"feature_set_version"`
		DefaultModel            string `json:"default_model"`
		RegistryPath            string `json:"registry_path,omitempty"`
		ShadowLogPath           string `json:"shadow_log_path,omitempty"`
		FallbackToDeterministic bool   `json:"fallback_to_deterministic"`
	} `json:"ml"`
	Validation struct {
		TrainYears           int     `json:"train_years"`
		ValidationMonths     int     `json:"validation_months"`
		TestMonths           int     `json:"test_months"`
		EmbargoDays          int     `json:"embargo_days"`
		MinDirectionAccuracy float64 `json:"min_direction_accuracy"`
		MaxMAPE              float64 `json:"max_mape"`
		MinSharpe            float64 `json:"min_sharpe"`
		MaxDrawdown          float64 `json:"max_drawdown"`
	} `json:"validation"`
	TradeGate struct {
		MinExpectedReturnAfterCost  float64 `json:"min_expected_return_after_cost"`
		MinDirectionProbability     float64 `json:"min_direction_probability"`
		MaxIntervalWidthATRMultiple float64 `json:"max_interval_width_atr_multiple"`
		MinSourceScore              float64 `json:"min_source_score"`
		AllowTradeOnLowQualityData  bool    `json:"allow_trade_on_low_quality_data"`
	} `json:"trade_gate"`
	Costs struct {
		CommissionBps            float64 `json:"commission_bps"`
		SlippageBps              float64 `json:"slippage_bps"`
		SpreadBps                float64 `json:"spread_bps"`
		VolumeParticipationLimit float64 `json:"volume_participation_limit"`
	} `json:"costs"`
	BIST struct {
		TickSizeDefault     float64 `json:"tick_size_default"`
		UseOfficialBulletin bool    `json:"use_official_bulletin"`
		UseAdjustedPrices   bool    `json:"use_adjusted_prices"`
	} `json:"bist"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	var cfg RuntimeConfig
	cfg.ML.Enabled = true
	cfg.ML.ShadowMode = true
	cfg.ML.FeatureSetVersion = "bist_ml_features_v1"
	cfg.ML.DefaultModel = "ensemble_v1"
	cfg.ML.RegistryPath = filepath.Join("data", "ml", "registry.json")
	cfg.ML.ShadowLogPath = filepath.Join("data", "ml", "shadow", "shadow_log.jsonl")
	cfg.ML.FallbackToDeterministic = true
	cfg.Validation.TrainYears = 5
	cfg.Validation.ValidationMonths = 3
	cfg.Validation.TestMonths = 3
	cfg.Validation.EmbargoDays = 1
	cfg.Validation.MinDirectionAccuracy = 0.53
	cfg.Validation.MaxMAPE = 0.05
	cfg.Validation.MinSharpe = 0.5
	cfg.Validation.MaxDrawdown = 0.15
	cfg.TradeGate.MinExpectedReturnAfterCost = 0.003
	cfg.TradeGate.MinDirectionProbability = 0.56
	cfg.TradeGate.MaxIntervalWidthATRMultiple = 2.5
	cfg.TradeGate.MinSourceScore = 0.75
	cfg.TradeGate.AllowTradeOnLowQualityData = false
	cfg.Costs.CommissionBps = 5
	cfg.Costs.SlippageBps = 10
	cfg.Costs.SpreadBps = 5
	cfg.Costs.VolumeParticipationLimit = 0.05
	cfg.BIST.TickSizeDefault = 0.25
	cfg.BIST.UseOfficialBulletin = true
	cfg.BIST.UseAdjustedPrices = true
	return cfg
}

func LoadRuntimeConfig(path string) RuntimeConfig {
	cfg := DefaultRuntimeConfig()
	if path == "" {
		path = filepath.Join("config", "forecast_ml.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return DefaultRuntimeConfig()
	}
	return cfg
}

func (a ModelArtifact) Validate(featureSetVersion string) []string {
	warnings := []string{}
	if strings.TrimSpace(a.ModelName) == "" {
		warnings = append(warnings, "model_name_missing")
	}
	if strings.TrimSpace(a.Version) == "" {
		warnings = append(warnings, "model_version_missing")
	}
	if strings.TrimSpace(a.FeatureSetVersion) == "" {
		warnings = append(warnings, "feature_set_version_missing")
	} else if featureSetVersion != "" && a.FeatureSetVersion != featureSetVersion {
		warnings = append(warnings, "feature_set_version_mismatch")
	}
	if a.Status == ArtifactDisabled {
		warnings = append(warnings, "artifact_disabled")
	}
	if strings.TrimSpace(a.ArtifactPath) == "" {
		warnings = append(warnings, "artifact_path_missing")
	} else if _, err := os.Stat(a.ArtifactPath); err != nil {
		warnings = append(warnings, "artifact_file_unavailable:"+err.Error())
	}
	if a.ValidationMetrics.Samples > 0 {
		if a.ValidationMetrics.DirectionAccuracy > 0 && a.ValidationMetrics.DirectionAccuracy < 0.53 {
			warnings = append(warnings, "direction_accuracy_below_research_gate")
		}
		if a.ValidationMetrics.MAPE > 0.05 {
			warnings = append(warnings, "mape_above_research_gate")
		}
	}
	return warnings
}

func (a ModelArtifact) Usable(featureSetVersion string) error {
	warnings := a.Validate(featureSetVersion)
	if len(warnings) == 0 {
		return nil
	}
	return fmt.Errorf("artifact not usable: %s", strings.Join(warnings, ","))
}

func HashConfig(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:])
}

func CurrentGitCommitHash() string {
	out, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func WriteArtifact(path string, artifact ModelArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure artifact dir: %w", err)
	}
	raw, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}
	return nil
}
