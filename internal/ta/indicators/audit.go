package indicators

import (
	"sort"
	"time"

	"hissebot/internal/ta/ohlcv"
)

type CatalogAuditEntry struct {
	Name                 string   `json:"name"`
	Category             string   `json:"category"`
	Group                string   `json:"group"`
	Template             string   `json:"template"`
	Status               string   `json:"status"`
	RobustnessScore      int      `json:"robustness_score"`
	DetectionEligible    bool     `json:"detection_eligible"`
	ExactOHLCVFormula    bool     `json:"exact_ohlcv_formula"`
	ExternalDataRequired bool     `json:"external_data_required"`
	AlgorithmRequired    bool     `json:"algorithm_required"`
	ProxyOnly            bool     `json:"proxy_only"`
	MinimumBars          int      `json:"minimum_bars"`
	Notes                []string `json:"notes,omitempty"`
}

type CatalogAuditReport struct {
	GeneratedAtUTC             time.Time           `json:"generated_at_utc"`
	Total                      int                 `json:"total"`
	ExactOHLCVFormulaCount     int                 `json:"exact_ohlcv_formula_count"`
	ExternalDataRequiredCount  int                 `json:"external_data_required_count"`
	AlgorithmRequiredCount     int                 `json:"algorithm_required_count"`
	ProxyOnlyCount             int                 `json:"proxy_only_count"`
	DetectionEligiblePercent   float64             `json:"detection_eligible_percent"`
	ExternalDataRequiredPct    float64             `json:"external_data_required_percent"`
	AlgorithmRequiredPct       float64             `json:"algorithm_required_percent"`
	ProxyOnlyPercent           float64             `json:"proxy_only_percent"`
	HandledCoveragePercent     float64             `json:"handled_coverage_percent"`
	OverallRobustnessScore     int                 `json:"overall_robustness_score"`
	NoHallucinationSafetyScore int                 `json:"no_hallucination_safety_score"`
	Entries                    []CatalogAuditEntry `json:"entries"`
}

func AuditIndicatorCatalog() CatalogAuditReport {
	specs := registeredIndicatorSpecs()
	candles := indicatorAuditCandles(260)
	snapshot, _ := Snapshot(candles)
	input := ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Snapshot:   snapshot,
		LastClose:  candles[len(candles)-1].EffectiveClose(),
		LastVolume: candles[len(candles)-1].EffectiveVolume(),
	}
	exact := exactIndicatorFormulaNames()
	report := CatalogAuditReport{
		GeneratedAtUTC: time.Now().UTC(),
		Total:          len(specs),
		Entries:        make([]CatalogAuditEntry, 0, len(specs)),
	}
	scoreSum := 0
	for _, spec := range specs {
		entry := auditIndicatorSpec(input, exact, spec)
		if entry.ExactOHLCVFormula {
			report.ExactOHLCVFormulaCount++
		}
		if entry.ExternalDataRequired {
			report.ExternalDataRequiredCount++
		}
		if entry.AlgorithmRequired {
			report.AlgorithmRequiredCount++
		}
		if entry.ProxyOnly {
			report.ProxyOnlyCount++
		}
		scoreSum += entry.RobustnessScore
		report.Entries = append(report.Entries, entry)
	}
	if report.Total > 0 {
		report.DetectionEligiblePercent = pct(report.ExactOHLCVFormulaCount, report.Total)
		report.ExternalDataRequiredPct = pct(report.ExternalDataRequiredCount, report.Total)
		report.AlgorithmRequiredPct = pct(report.AlgorithmRequiredCount, report.Total)
		report.ProxyOnlyPercent = pct(report.ProxyOnlyCount, report.Total)
		report.HandledCoveragePercent = pct(report.ExactOHLCVFormulaCount+report.ExternalDataRequiredCount+report.AlgorithmRequiredCount, report.Total)
		report.OverallRobustnessScore = roundedDiv(scoreSum, report.Total)
	}
	// This score measures whether the engine refuses unsupported indicators instead of fabricating values.
	if report.ProxyOnlyCount == 0 {
		report.NoHallucinationSafetyScore = 100
	} else {
		report.NoHallucinationSafetyScore = 92
	}
	sort.SliceStable(report.Entries, func(i, j int) bool {
		if report.Entries[i].Status != report.Entries[j].Status {
			return report.Entries[i].Status < report.Entries[j].Status
		}
		return report.Entries[i].Name < report.Entries[j].Name
	})
	return report
}

func auditIndicatorSpec(input ScannerInput, exact map[string]struct{}, spec indicatorSpec) CatalogAuditEntry {
	value := valueForIndicator(input, spec)
	minBars := minimumBarsForIndicator(input, spec, value)
	key := normalizeIndicatorText(spec.Name)
	_, exactFormula := exact[key]
	entry := CatalogAuditEntry{
		Name:              spec.Name,
		Category:          spec.Category,
		Group:             spec.Group,
		Template:          spec.Template,
		ExactOHLCVFormula: exactFormula,
		MinimumBars:       minBars,
	}
	switch {
	case exactFormula:
		entry.Status = "exact_ohlcv_formula"
		entry.DetectionEligible = true
		entry.RobustnessScore = 100
		if minBars > 0 {
			entry.Notes = []string{"exact formula plus minimum completed-candle gate"}
		} else {
			entry.Notes = []string{"exact formula available from OHLCV/snapshot"}
		}
	case isExternalOnly(spec):
		entry.Status = "external_data_required"
		entry.ExternalDataRequired = true
		entry.RobustnessScore = 80
		entry.Notes = []string{externalDataEvidence(spec)}
	case isAlgorithmRequired(spec):
		entry.Status = algorithmRequiredSource
		entry.AlgorithmRequired = true
		entry.RobustnessScore = 70
		entry.Notes = []string{algorithmRequiredEvidence(spec)}
	default:
		entry.Status = "proxy_only"
		entry.ProxyOnly = true
		entry.RobustnessScore = 0
		entry.Notes = []string{"exact formula is not registered; scanner must keep this non-computed"}
	}
	return entry
}

func indicatorAuditCandles(count int) []ohlcv.Candle {
	if count < 1 {
		count = 1
	}
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]ohlcv.Candle, count)
	for i := range out {
		closePrice := 100 + float64(i)*0.21 + float64((i%11)-5)*0.18
		open := closePrice - 0.12 + float64(i%3)*0.04
		high := maxFloat(open, closePrice) + 0.75 + float64(i%5)*0.03
		low := minFloat(open, closePrice) - 0.70 - float64(i%7)*0.02
		out[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 100000 + float64((i%17)*2300),
		}
	}
	return out
}

func pct(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func roundedDiv(sum, count int) int {
	if count <= 0 {
		return 0
	}
	return int(float64(sum)/float64(count) + 0.5)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
