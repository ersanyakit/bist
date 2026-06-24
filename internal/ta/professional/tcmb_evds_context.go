package professional

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TCMBEVDSContextReport struct {
	Computed                      bool                    `json:"computed"`
	AnalysisReady                 bool                    `json:"analysis_ready"`
	CatalogPath                   string                  `json:"catalog_path,omitempty"`
	SeriesDir                     string                  `json:"series_dir,omitempty"`
	CatalogSeriesCount            int                     `json:"catalog_series_count"`
	SeriesFileCount               int                     `json:"series_file_count"`
	CatalogMatchedSeriesFileCount int                     `json:"catalog_matched_series_file_count,omitempty"`
	CatalogMissingSeriesFileCount int                     `json:"catalog_missing_series_file_count,omitempty"`
	ExtraSeriesFileCount          int                     `json:"extra_series_file_count,omitempty"`
	DataGroupCount                int                     `json:"data_group_count"`
	LatestFetchAt                 time.Time               `json:"latest_fetch_at,omitempty"`
	AsOf                          time.Time               `json:"as_of,omitempty"`
	DataQualityScore              float64                 `json:"data_quality_score,omitempty"`
	PointInTimeSafe               bool                    `json:"point_in_time_safe"`
	ScoreEligible                 bool                    `json:"score_eligible"`
	ScoreAdjustment               float64                 `json:"score_adjustment,omitempty"`
	Regime                        string                  `json:"regime,omitempty"`
	Exposure                      TCMBMacroExposure       `json:"exposure,omitempty"`
	Indicators                    []TCMBMacroIndicator    `json:"indicators,omitempty"`
	Contributions                 []TCMBMacroContribution `json:"contributions,omitempty"`
	ForecastImpact                TCMBMacroForecastImpact `json:"forecast_impact,omitempty"`
	Summary                       string                  `json:"summary"`
	Warnings                      []string                `json:"warnings,omitempty"`
}

type TCMBMacroForecastImpact struct {
	Computed                 bool     `json:"computed"`
	Direction                string   `json:"direction,omitempty"`
	Label                    string   `json:"label,omitempty"`
	Severity                 string   `json:"severity,omitempty"`
	Confidence               float64  `json:"confidence,omitempty"`
	PressureScore            float64  `json:"pressure_score,omitempty"`
	ScoreAdjustment          float64  `json:"score_adjustment,omitempty"`
	Horizon                  string   `json:"horizon,omitempty"`
	DecisionUse              string   `json:"decision_use,omitempty"`
	DocumentEvidenceIncluded bool     `json:"document_evidence_included"`
	DocumentCount            int      `json:"document_count,omitempty"`
	DocumentTextIndexPath    string   `json:"document_text_index_path,omitempty"`
	DocumentTextUsableCount  int      `json:"document_text_usable_count,omitempty"`
	DocumentCategories       []string `json:"document_categories,omitempty"`
	Summary                  string   `json:"summary,omitempty"`
	Drivers                  []string `json:"drivers,omitempty"`
	Blockers                 []string `json:"blockers,omitempty"`
}

type tcmbEVDSCatalogFile struct {
	FetchedAt string `json:"fetched_at"`
	Stats     struct {
		DataGroups int `json:"data_groups"`
		Series     int `json:"series"`
	} `json:"stats"`
	Series []tcmbEVDSCatalogSeries `json:"series"`
}

type tcmbEVDSCatalogSeries struct {
	SeriesCode    string `json:"SERIE_CODE"`
	DataGroupCode string `json:"DATAGROUP_CODE"`
}

type evdsCatalogSeriesFileStats struct {
	HasCatalogSeries bool
	Matched          int
	Missing          int
	Extra            int
}

func buildTCMBEVDSContext(evdsDir string) TCMBEVDSContextReport {
	evdsDir = strings.TrimSpace(evdsDir)
	if evdsDir == "" {
		return TCMBEVDSContextReport{
			Summary:  "TCMB EVDS dizini yapılandırılmamış.",
			Warnings: []string{"tcmb_evds_dir_not_configured"},
		}
	}

	catalogPath := filepath.Join(evdsDir, "catalog.json")
	seriesDir := filepath.Join(evdsDir, "series")
	raw, err := os.ReadFile(catalogPath)
	if os.IsNotExist(err) {
		return TCMBEVDSContextReport{
			CatalogPath: catalogPath,
			SeriesDir:   seriesDir,
			Summary:     fmt.Sprintf("TCMB EVDS katalog bulunamadı: %s.", catalogPath),
			Warnings:    []string{"tcmb_evds_catalog_missing"},
		}
	}
	if err != nil {
		return TCMBEVDSContextReport{
			CatalogPath: catalogPath,
			SeriesDir:   seriesDir,
			Summary:     "TCMB EVDS katalog okunamadı: " + err.Error(),
			Warnings:    []string{"tcmb_evds_catalog_read_error"},
		}
	}

	var catalog tcmbEVDSCatalogFile
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return TCMBEVDSContextReport{
			CatalogPath: catalogPath,
			SeriesDir:   seriesDir,
			Summary:     "TCMB EVDS katalog parse edilemedi: " + err.Error(),
			Warnings:    []string{"tcmb_evds_catalog_parse_error"},
		}
	}

	seriesFiles, latestMod, walkErr := countEVDSJSONFiles(seriesDir)
	catalogFileStats := inspectEVDSCatalogSeriesFiles(catalog, seriesDir, seriesFiles)
	warnings := []string{}
	if walkErr != nil {
		warnings = append(warnings, "tcmb_evds_series_scan_error")
	}
	if catalog.Stats.Series == 0 {
		warnings = append(warnings, "tcmb_evds_catalog_series_empty")
	}
	if seriesFiles == 0 {
		warnings = append(warnings, "tcmb_evds_series_missing")
	}
	if catalogFileStats.HasCatalogSeries && catalogFileStats.Missing > 0 {
		warnings = append(warnings, "tcmb_evds_catalog_series_missing_files")
	} else if catalog.Stats.Series > 0 && seriesFiles > 0 && seriesFiles < catalog.Stats.Series {
		warnings = append(warnings, "tcmb_evds_series_partial")
	}
	if catalogFileStats.HasCatalogSeries && catalogFileStats.Extra > 0 {
		warnings = append(warnings, "tcmb_evds_series_extra_files")
	}

	latestFetchAt := parseRFC3339Time(catalog.FetchedAt)
	if latestFetchAt.IsZero() {
		latestFetchAt = latestMod
	}
	computed := catalog.Stats.Series > 0 && seriesFiles >= catalog.Stats.Series
	matchedFiles := seriesFiles
	if catalogFileStats.HasCatalogSeries {
		matchedFiles = catalogFileStats.Matched
		computed = catalog.Stats.Series > 0 && catalogFileStats.Missing == 0 && catalogFileStats.Matched >= catalog.Stats.Series
	}
	summary := fmt.Sprintf("TCMB EVDS: katalogda %d seri, yerelde %d seri dosyası; katalogla eşleşen %d.", catalog.Stats.Series, seriesFiles, matchedFiles)
	if computed {
		summary = fmt.Sprintf("TCMB EVDS: katalogdaki %d seri dosyası eşleşti.", matchedFiles)
		if catalogFileStats.Extra > 0 {
			summary += fmt.Sprintf(" Yerelde katalog dışı %d ek seri dosyası var; bunlar güven skorunu sınırlar.", catalogFileStats.Extra)
		}
	}
	if walkErr != nil {
		summary += " Seri klasörü tarama uyarısı: " + walkErr.Error()
	}

	return TCMBEVDSContextReport{
		Computed:                      computed,
		CatalogPath:                   catalogPath,
		SeriesDir:                     seriesDir,
		CatalogSeriesCount:            catalog.Stats.Series,
		SeriesFileCount:               seriesFiles,
		CatalogMatchedSeriesFileCount: matchedFiles,
		CatalogMissingSeriesFileCount: catalogFileStats.Missing,
		ExtraSeriesFileCount:          catalogFileStats.Extra,
		DataGroupCount:                catalog.Stats.DataGroups,
		LatestFetchAt:                 latestFetchAt,
		Summary:                       summary,
		Warnings:                      warnings,
	}
}

func inspectEVDSCatalogSeriesFiles(catalog tcmbEVDSCatalogFile, seriesDir string, totalFiles int) evdsCatalogSeriesFileStats {
	expected := map[string]struct{}{}
	for _, item := range catalog.Series {
		group := strings.ToLower(strings.TrimSpace(item.DataGroupCode))
		code := strings.TrimSpace(item.SeriesCode)
		if group == "" || code == "" {
			continue
		}
		fileName := strings.ReplaceAll(code, ".", "_") + ".json"
		expected[filepath.Clean(filepath.Join(seriesDir, group, fileName))] = struct{}{}
	}
	if len(expected) == 0 {
		return evdsCatalogSeriesFileStats{}
	}
	stats := evdsCatalogSeriesFileStats{HasCatalogSeries: true}
	for path := range expected {
		if _, err := os.Stat(path); err == nil {
			stats.Matched++
		} else {
			stats.Missing++
		}
	}
	if totalFiles > stats.Matched {
		stats.Extra = totalFiles - stats.Matched
	}
	return stats
}

func countEVDSJSONFiles(seriesDir string) (int, time.Time, error) {
	count := 0
	var latest time.Time
	err := filepath.WalkDir(seriesDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".json" {
			return nil
		}
		count++
		if info, err := entry.Info(); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, time.Time{}, nil
	}
	return count, latest, err
}

func parseRFC3339Time(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func defaultTCMBEVDSDirFromEquitiesDir(equitiesDir string) string {
	if strings.TrimSpace(equitiesDir) == "" {
		return filepath.Join("data", "macro", "tcmb_evds")
	}
	return filepath.Join(filepath.Dir(filepath.Clean(equitiesDir)), "macro", "tcmb_evds")
}
