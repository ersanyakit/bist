package macro

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultTUIKInflationFile = "data/macro/tuik_inflation_indices.json"

type InflationDataset struct {
	Source          string            `json:"source"`
	SourceURL       string            `json:"source_url,omitempty"`
	MetadataURL     string            `json:"metadata_url,omitempty"`
	Methodology     string            `json:"methodology,omitempty"`
	FetchedAt       string            `json:"fetched_at,omitempty"`
	PreferredSeries string            `json:"preferred_series,omitempty"`
	Series          []InflationSeries `json:"series"`
}

type InflationSeries struct {
	ID          string           `json:"id"`
	NameTR      string           `json:"name_tr,omitempty"`
	Unit        string           `json:"unit,omitempty"`
	Base        string           `json:"base,omitempty"`
	Points      []InflationPoint `json:"points"`
	Source      string           `json:"source,omitempty"`
	SourceURL   string           `json:"source_url,omitempty"`
	MetadataURL string           `json:"metadata_url,omitempty"`
	Warning     string           `json:"warning,omitempty"`
}

type InflationPoint struct {
	Period string  `json:"period"`
	Value  float64 `json:"value"`
}

type IndexAdjustment struct {
	SeriesID      string
	SeriesName    string
	Source        string
	SourceURL     string
	FromPeriod    string
	ToPeriod      string
	FromIndex     float64
	ToIndex       float64
	Factor        float64
	OriginalValue float64
	AdjustedValue float64
}

func LoadInflationDataset(path string) (InflationDataset, bool, error) {
	if strings.TrimSpace(path) == "" {
		return InflationDataset{}, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InflationDataset{}, false, nil
		}
		return InflationDataset{}, false, fmt.Errorf("read inflation dataset: %w", err)
	}
	var dataset InflationDataset
	if err := json.Unmarshal(raw, &dataset); err != nil {
		return InflationDataset{}, false, fmt.Errorf("parse inflation dataset: %w", err)
	}
	return dataset, true, nil
}

func SaveInflationDataset(path string, dataset InflationDataset) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("inflation dataset path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create macro dir: %w", err)
	}
	raw, err := json.MarshalIndent(dataset, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal inflation dataset: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write inflation dataset: %w", err)
	}
	return nil
}

func DefaultInflationPathFromEquitiesDir(equitiesDir string) string {
	if strings.TrimSpace(equitiesDir) == "" {
		return DefaultTUIKInflationFile
	}
	return filepath.Join(filepath.Dir(equitiesDir), "macro", "tuik_inflation_indices.json")
}

func AdjustValueByInflation(dataset InflationDataset, value float64, fromPeriod string) (IndexAdjustment, bool) {
	if value <= 0 || strings.TrimSpace(fromPeriod) == "" {
		return IndexAdjustment{}, false
	}
	series, ok := PreferredInflationSeries(dataset)
	if !ok {
		return IndexAdjustment{}, false
	}
	points := validInflationPoints(series.Points)
	if len(points) == 0 {
		return IndexAdjustment{}, false
	}
	byPeriod := make(map[string]float64, len(points))
	for _, point := range points {
		byPeriod[point.Period] = point.Value
	}
	basePeriod := normalizeInflationPeriod(fromPeriod)
	baseIndex, ok := byPeriod[basePeriod]
	if !ok || baseIndex <= 0 {
		return IndexAdjustment{}, false
	}
	latest := points[len(points)-1]
	if latest.Value <= 0 || latest.Period < basePeriod {
		return IndexAdjustment{}, false
	}
	factor := latest.Value / baseIndex
	if factor <= 0 {
		return IndexAdjustment{}, false
	}
	source := firstNonEmptyInflation(series.Source, dataset.Source)
	sourceURL := firstNonEmptyInflation(series.SourceURL, dataset.SourceURL, series.MetadataURL, dataset.MetadataURL)
	return IndexAdjustment{
		SeriesID:      series.ID,
		SeriesName:    firstNonEmptyInflation(series.NameTR, strings.ToUpper(series.ID)),
		Source:        source,
		SourceURL:     sourceURL,
		FromPeriod:    basePeriod,
		ToPeriod:      latest.Period,
		FromIndex:     baseIndex,
		ToIndex:       latest.Value,
		Factor:        factor,
		OriginalValue: value,
		AdjustedValue: value * factor,
	}, true
}

func PreferredInflationSeries(dataset InflationDataset) (InflationSeries, bool) {
	preferred := strings.TrimSpace(dataset.PreferredSeries)
	if preferred != "" {
		for _, series := range dataset.Series {
			if strings.EqualFold(strings.TrimSpace(series.ID), preferred) {
				return series, true
			}
		}
	}
	for _, series := range dataset.Series {
		if len(validInflationPoints(series.Points)) > 0 {
			return series, true
		}
	}
	return InflationSeries{}, false
}

func InflationLatestPeriod(dataset InflationDataset) string {
	series, ok := PreferredInflationSeries(dataset)
	if !ok {
		return ""
	}
	points := validInflationPoints(series.Points)
	if len(points) == 0 {
		return ""
	}
	return points[len(points)-1].Period
}

func InflationSeriesLabel(dataset InflationDataset) string {
	series, ok := PreferredInflationSeries(dataset)
	if !ok {
		return ""
	}
	return firstNonEmptyInflation(series.NameTR, strings.ToUpper(series.ID))
}

func validInflationPoints(points []InflationPoint) []InflationPoint {
	out := make([]InflationPoint, 0, len(points))
	for _, point := range points {
		period := normalizeInflationPeriod(point.Period)
		if period == "" || point.Value <= 0 {
			continue
		}
		point.Period = period
		out = append(out, point)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Period < out[j].Period })
	deduped := out[:0]
	for _, point := range out {
		if len(deduped) > 0 && deduped[len(deduped)-1].Period == point.Period {
			deduped[len(deduped)-1] = point
			continue
		}
		deduped = append(deduped, point)
	}
	return deduped
}

func normalizeInflationPeriod(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "/", "-")
	if len(value) >= len("2006-01") {
		value = value[:len("2006-01")]
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 || len(parts[0]) != 4 {
		return ""
	}
	month := parts[1]
	if len(month) == 1 {
		month = "0" + month
	}
	if len(month) != 2 || month < "01" || month > "12" {
		return ""
	}
	return parts[0] + "-" + month
}

func firstNonEmptyInflation(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
