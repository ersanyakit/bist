package financials

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"hissebot/internal/domain"
)

const GoldenFinancialRatiosVersion = "golden-financial-ratios-v1"

type GoldenRatioSuite struct {
	Version   string            `json:"version"`
	Source    string            `json:"source,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
	Cases     []GoldenRatioCase `json:"cases"`
}

type GoldenRatioCase struct {
	Name      string                        `json:"name"`
	Input     domain.BilancoInfo            `json:"input"`
	Expected  map[string]domain.YearQuarter `json:"expected"`
	Tolerance float64                       `json:"tolerance,omitempty"`
}

type GoldenRatioReport struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Cases     int       `json:"cases"`
	Failures  int       `json:"failures"`
	CheckedAt time.Time `json:"checked_at"`
	Errors    []string  `json:"errors,omitempty"`
}

func ValidateGoldenRatios(path string) (GoldenRatioReport, error) {
	report := GoldenRatioReport{Status: "pass", CheckedAt: time.Now().UTC()}
	raw, err := os.ReadFile(path)
	if err != nil {
		return report, err
	}
	var suite GoldenRatioSuite
	if err := json.Unmarshal(raw, &suite); err != nil {
		return report, err
	}
	report.Version = suite.Version
	if suite.Version != GoldenFinancialRatiosVersion {
		report.Errors = append(report.Errors, "golden_ratio_suite_version_invalid")
	}
	if len(suite.Cases) == 0 {
		report.Errors = append(report.Errors, "golden_ratio_cases_missing")
	}
	for _, tc := range suite.Cases {
		report.Cases++
		tolerance := tc.Tolerance
		if tolerance <= 0 {
			tolerance = 1e-9
		}
		if tc.Name == "" {
			report.Errors = append(report.Errors, "golden_ratio_case_name_missing")
		}
		got := CalculateEquity(&tc.Input)
		failures := compareGoldenRatios(tc.Name, got, tc.Expected, tolerance)
		report.Failures += len(failures)
		report.Errors = append(report.Errors, failures...)
	}
	if len(report.Errors) > 0 || report.Failures > 0 {
		report.Status = "fail"
	}
	return report, nil
}

func compareGoldenRatios(name string, got map[string]domain.YearQuarter, expected map[string]domain.YearQuarter, tolerance float64) []string {
	errors := []string{}
	metrics := make([]string, 0, len(expected))
	for metric := range expected {
		metrics = append(metrics, metric)
	}
	sort.Strings(metrics)
	for _, metric := range metrics {
		for year, quarters := range expected[metric] {
			for _, quarter := range []string{"Q1", "Q2", "Q3", "Q4"} {
				want, ok := goldenQuarterValue(quarters, quarter)
				if !ok {
					continue
				}
				gotValue, ok := goldenResultQuarterValue(got, metric, year, quarter)
				if !ok {
					errors = append(errors, fmt.Sprintf("%s:%s:%s:%s_missing", name, metric, year, quarter))
					continue
				}
				if math.Abs(gotValue-want) > tolerance {
					errors = append(errors, fmt.Sprintf("%s:%s:%s:%s_mismatch_got_%.12f_want_%.12f", name, metric, year, quarter, gotValue, want))
				}
			}
		}
	}
	return errors
}

func goldenResultQuarterValue(got map[string]domain.YearQuarter, metric string, year string, quarter string) (float64, bool) {
	yearValues, ok := got[metric][year]
	if !ok {
		return 0, false
	}
	return goldenQuarterValue(yearValues, quarter)
}

func goldenQuarterValue(values domain.QuarterValues, quarter string) (float64, bool) {
	var value *float64
	switch quarter {
	case "Q1":
		value = values.Q1
	case "Q2":
		value = values.Q2
	case "Q3":
		value = values.Q3
	case "Q4":
		value = values.Q4
	}
	if value == nil {
		return 0, false
	}
	return *value, true
}
