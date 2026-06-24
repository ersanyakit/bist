package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type WalkForwardConfig struct {
	From             time.Time `json:"from"`
	To               time.Time `json:"to"`
	TrainYears       int       `json:"train_years"`
	ValidationMonths int       `json:"validation_months"`
	TestMonths       int       `json:"test_months"`
	EmbargoDays      int       `json:"embargo_days"`
	Expanding        bool      `json:"expanding"`
}

type ValidationWindowResult struct {
	Window         int       `json:"window"`
	TrainFrom      time.Time `json:"train_from"`
	TrainTo        time.Time `json:"train_to"`
	ValidationFrom time.Time `json:"validation_from"`
	ValidationTo   time.Time `json:"validation_to"`
	TestFrom       time.Time `json:"test_from"`
	TestTo         time.Time `json:"test_to"`
	Metrics        Metrics   `json:"metrics"`
	Passed         bool      `json:"passed"`
	FailReasons    []string  `json:"fail_reasons,omitempty"`
}

type ValidationReport struct {
	ModelName         string                   `json:"model_name"`
	ModelVersion      string                   `json:"model_version"`
	FeatureSetVersion string                   `json:"feature_set_version"`
	Windows           []ValidationWindowResult `json:"windows"`
	AggregateMetrics  Metrics                  `json:"aggregate_metrics"`
	BaselineMetrics   map[string]Metrics       `json:"baseline_metrics,omitempty"`
	Passed            bool                     `json:"passed"`
	FailReasons       []string                 `json:"fail_reasons,omitempty"`
}

func GenerateWindows(cfg WalkForwardConfig) []ValidationWindowResult {
	if cfg.TrainYears <= 0 {
		cfg.TrainYears = 5
	}
	if cfg.ValidationMonths <= 0 {
		cfg.ValidationMonths = 3
	}
	if cfg.TestMonths <= 0 {
		cfg.TestMonths = 3
	}
	if cfg.From.IsZero() || cfg.To.IsZero() || !cfg.From.Before(cfg.To) {
		return nil
	}
	out := []ValidationWindowResult{}
	trainFrom := cfg.From
	trainTo := trainFrom.AddDate(cfg.TrainYears, 0, -1)
	idx := 1
	for trainTo.Before(cfg.To) {
		valFrom := trainTo.AddDate(0, 0, cfg.EmbargoDays+1)
		valTo := valFrom.AddDate(0, cfg.ValidationMonths, -1)
		testFrom := valTo.AddDate(0, 0, cfg.EmbargoDays+1)
		testTo := testFrom.AddDate(0, cfg.TestMonths, -1)
		if testFrom.After(cfg.To) {
			break
		}
		if testTo.After(cfg.To) {
			testTo = cfg.To
		}
		out = append(out, ValidationWindowResult{
			Window:         idx,
			TrainFrom:      trainFrom,
			TrainTo:        trainTo,
			ValidationFrom: valFrom,
			ValidationTo:   valTo,
			TestFrom:       testFrom,
			TestTo:         testTo,
			Passed:         true,
		})
		idx++
		if cfg.Expanding {
			trainTo = trainTo.AddDate(0, cfg.TestMonths, 0)
		} else {
			trainFrom = trainFrom.AddDate(0, cfg.TestMonths, 0)
			trainTo = trainFrom.AddDate(cfg.TrainYears, 0, -1)
		}
	}
	return out
}

func ExportReport(path string, report ValidationReport) error {
	if path == "" {
		return fmt.Errorf("validation report path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure validation report dir: %w", err)
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal validation report: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write validation report: %w", err)
	}
	return nil
}
