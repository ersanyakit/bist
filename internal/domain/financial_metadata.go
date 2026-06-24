package domain

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	FinancialMetadataVersion    = "financial-metadata-v1"
	FinancialCalculationVersion = "financial-ratios-v1"

	FinancialAvailabilityVerifiedPublishDate       = "verified_publish_date"
	FinancialAvailabilityConservativeAvailableAt   = "conservative_available_at"
	FinancialAvailabilityUnsafeUnverifiedAvailable = "unsafe_unverified_available_at"
)

type FinancialPeriod struct {
	Key                string     `json:"key"`
	FiscalYear         int        `json:"fiscal_year"`
	FiscalQuarter      int        `json:"fiscal_quarter"`
	PeriodEnd          time.Time  `json:"period_end"`
	ReportDate         *time.Time `json:"report_date,omitempty"`
	PublishDate        *time.Time `json:"publish_date,omitempty"`
	AvailableAt        *time.Time `json:"available_at,omitempty"`
	AvailabilitySource string     `json:"availability_source,omitempty"`
	Source             string     `json:"source,omitempty"`
	SourceDocumentID   string     `json:"source_document_id,omitempty"`
	FinancialGroup     string     `json:"financial_group,omitempty"`
	Currency           string     `json:"currency,omitempty"`
	FetchedAt          time.Time  `json:"fetched_at,omitempty"`
	BacktestSafe       bool       `json:"backtest_safe"`
	Warnings           []string   `json:"warnings,omitempty"`
}

type DataLineageEvent struct {
	Stage     string    `json:"stage"`
	Source    string    `json:"source,omitempty"`
	Transform string    `json:"transform,omitempty"`
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Notes     []string  `json:"notes,omitempty"`
}

type FinancialDataQuality struct {
	BacktestSafe              bool                           `json:"backtest_safe"`
	FinanciallyConsistent     bool                           `json:"financially_consistent"`
	PublishDateCoverage       float64                        `json:"publish_date_coverage"`
	AvailableAtCoverage       float64                        `json:"available_at_coverage"`
	PeriodCount               int                            `json:"period_count"`
	ReconciliationChecks      []FinancialReconciliationCheck `json:"reconciliation_checks,omitempty"`
	MissingPublishPeriods     []string                       `json:"missing_publish_periods,omitempty"`
	MissingAvailableAtPeriods []string                       `json:"missing_available_at_periods,omitempty"`
	UnsafeBacktestPeriods     []string                       `json:"unsafe_backtest_periods,omitempty"`
	InvalidChronologyPeriods  []string                       `json:"invalid_chronology_periods,omitempty"`
	Warnings                  []string                       `json:"warnings,omitempty"`
	ValidatedAt               time.Time                      `json:"validated_at,omitempty"`
	MetadataVersion           string                         `json:"metadata_version,omitempty"`
}

type FinancialReconciliationCheck struct {
	PeriodKey     string   `json:"period_key"`
	Check         string   `json:"check"`
	Expected      float64  `json:"expected"`
	Actual        float64  `json:"actual"`
	Difference    float64  `json:"difference"`
	Tolerance     float64  `json:"tolerance"`
	Passed        bool     `json:"passed"`
	MissingInputs bool     `json:"missing_inputs,omitempty"`
	InputFields   []string `json:"input_fields,omitempty"`
	Warning       string   `json:"warning,omitempty"`
}

type FinancialCalculationAudit struct {
	Version      string                          `json:"version"`
	CreatedAt    time.Time                       `json:"created_at"`
	BacktestSafe bool                            `json:"backtest_safe"`
	InputQuality FinancialDataQuality            `json:"input_quality"`
	Lineage      []DataLineageEvent              `json:"lineage,omitempty"`
	Metrics      map[string]FinancialMetricAudit `json:"metrics,omitempty"`
	Warnings     []string                        `json:"warnings,omitempty"`
}

type FinancialMetricAudit struct {
	Formula     string   `json:"formula"`
	InputFields []string `json:"input_fields"`
}

func FinancialPeriodKey(year, quarter int) string {
	if year <= 0 || quarter < 1 || quarter > 4 {
		return ""
	}
	return fmt.Sprintf("%04d-Q%d", year, quarter)
}

func FiscalQuarterFromIndex(index int) int {
	switch index {
	case 0:
		return 4
	case 1:
		return 3
	case 2:
		return 2
	case 3:
		return 1
	default:
		return 0
	}
}

func FiscalPeriodEnd(year, quarter int) time.Time {
	switch quarter {
	case 1:
		return time.Date(year, 3, 31, 0, 0, 0, 0, time.UTC)
	case 2:
		return time.Date(year, 6, 30, 0, 0, 0, 0, time.UTC)
	case 3:
		return time.Date(year, 9, 30, 0, 0, 0, 0, time.UTC)
	case 4:
		return time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}
	}
}

func FinancialPeriodAvailabilityStatus(period FinancialPeriod) string {
	return financialAvailabilityStatus(period.PublishDate, EffectiveFinancialAvailableAt(period), period.AvailabilitySource)
}

func FinancialPeriodChronologyWarnings(period FinancialPeriod) []string {
	periodEnd := period.PeriodEnd
	if periodEnd.IsZero() {
		periodEnd = FiscalPeriodEnd(period.FiscalYear, period.FiscalQuarter)
	}
	if periodEnd.IsZero() {
		return nil
	}
	warnings := []string{}
	if period.ReportDate != nil && !period.ReportDate.IsZero() && period.ReportDate.Before(periodEnd) {
		warnings = append(warnings, "report_date_before_period_end")
	}
	if period.PublishDate != nil && !period.PublishDate.IsZero() && period.PublishDate.Before(periodEnd) {
		warnings = append(warnings, "publish_date_before_period_end")
	}
	if availableAt := EffectiveFinancialAvailableAt(period); availableAt != nil && !availableAt.IsZero() && availableAt.Before(periodEnd) {
		warnings = append(warnings, "available_at_before_period_end")
	}
	return warnings
}

func FinancialPeriodChronologySafe(period FinancialPeriod) bool {
	return len(FinancialPeriodChronologyWarnings(period)) == 0
}

func FinancialStatementVersionAvailabilityStatus(version FinancialStatementVersion) string {
	return financialAvailabilityStatus(version.PublishDate, version.AvailableAt, version.AvailabilitySource)
}

func financialAvailabilityStatus(publishDate *time.Time, availableAt *time.Time, source string) string {
	if publishDate != nil && !publishDate.IsZero() {
		return FinancialAvailabilityVerifiedPublishDate
	}
	if availableAt == nil || availableAt.IsZero() {
		return FinancialAvailabilityUnsafeUnverifiedAvailable
	}
	source = strings.TrimSpace(strings.ToLower(source))
	if !IsConservativeFinancialAvailabilitySource(source) {
		return FinancialAvailabilityUnsafeUnverifiedAvailable
	}
	return FinancialAvailabilityConservativeAvailableAt
}

func IsConservativeFinancialAvailabilitySource(source string) bool {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case "fetched_at",
		"local_json_available_at",
		"local_json_calculation_at",
		"local_json_fetch_at",
		"local_json_import_at",
		"local_json_merge_at",
		"provided_available_at":
		return true
	default:
		return false
	}
}

func NewFinancialPeriod(year, quarter int, source, group, currency string, fetchedAt time.Time) FinancialPeriod {
	key := FinancialPeriodKey(year, quarter)
	warnings := []string{"publish_date_missing"}
	return FinancialPeriod{
		Key:            key,
		FiscalYear:     year,
		FiscalQuarter:  quarter,
		PeriodEnd:      FiscalPeriodEnd(year, quarter),
		Source:         source,
		FinancialGroup: group,
		Currency:       currency,
		FetchedAt:      fetchedAt,
		BacktestSafe:   false,
		Warnings:       warnings,
	}
}

func NormalizeBilancoInfo(info *BilancoInfo, ticker string) {
	if info == nil {
		return
	}
	if info.Ticker == "" {
		info.Ticker = ticker
	}
	if info.Data == nil {
		info.Data = map[string]BilancoField{}
	}
	if info.Periods == nil {
		info.Periods = map[string]FinancialPeriod{}
	}
	for code, field := range info.Data {
		if field.Years == nil {
			field.Years = map[string][]*float64{}
			info.Data[code] = field
		}
	}
	EnsureBilancoMetadata(info)
}

func EnsureBilancoMetadata(info *BilancoInfo) FinancialDataQuality {
	if info == nil {
		return FinancialDataQuality{}
	}
	if info.Periods == nil {
		info.Periods = map[string]FinancialPeriod{}
	}
	seen := map[string]FinancialPeriod{}
	for _, field := range info.Data {
		for yearText, values := range field.Years {
			year := 0
			_, _ = fmt.Sscanf(yearText, "%d", &year)
			if year <= 0 {
				continue
			}
			for index, value := range values {
				if value == nil {
					continue
				}
				quarter := FiscalQuarterFromIndex(index)
				key := FinancialPeriodKey(year, quarter)
				if key == "" {
					continue
				}
				period, ok := info.Periods[key]
				if !ok {
					period = NewFinancialPeriod(year, quarter, info.Source, info.FinancialGroup, info.Currency, info.FetchedAt)
				}
				period.Key = key
				period.FiscalYear = year
				period.FiscalQuarter = quarter
				if period.PeriodEnd.IsZero() {
					period.PeriodEnd = FiscalPeriodEnd(year, quarter)
				}
				if period.Source == "" {
					period.Source = info.Source
				}
				if period.FinancialGroup == "" {
					period.FinancialGroup = info.FinancialGroup
				}
				if period.Currency == "" {
					period.Currency = info.Currency
				}
				if period.FetchedAt.IsZero() {
					period.FetchedAt = info.FetchedAt
				}
				period = ensureFinancialPeriodAvailability(period)
				seen[key] = period
			}
		}
	}
	for key, period := range seen {
		info.Periods[key] = period
	}
	info.Quality = ValidateFinancialDataQuality(info, time.Time{})
	return info.Quality
}

func ApplyBilancoSourceMetadata(info *BilancoInfo, source, group, currency string, fetchedAt time.Time) {
	if info == nil {
		return
	}
	if source != "" {
		info.Source = source
	}
	if group != "" {
		info.FinancialGroup = group
	}
	if currency != "" {
		info.Currency = currency
	}
	if !fetchedAt.IsZero() {
		info.FetchedAt = fetchedAt
	}
	EnsureBilancoMetadata(info)
	for key, period := range info.Periods {
		if source != "" {
			period.Source = source
		}
		if group != "" {
			period.FinancialGroup = group
		}
		if currency != "" {
			period.Currency = currency
		}
		if !fetchedAt.IsZero() {
			period.FetchedAt = fetchedAt
		}
		if shouldUseFetchedAtAvailability(period) {
			at := period.FetchedAt.UTC()
			period.AvailableAt = &at
			period.AvailabilitySource = "fetched_at"
		}
		period = ensureFinancialPeriodAvailability(period)
		info.Periods[key] = period
	}
	info.Quality = ValidateFinancialDataQuality(info, time.Time{})
}

func MarkFinancialPeriodsAvailableAt(info *BilancoInfo, availableAt time.Time, source string) int {
	if info == nil || availableAt.IsZero() {
		return 0
	}
	NormalizeBilancoInfo(info, info.Ticker)
	source = firstNonEmpty(source, "local_json_available_at")
	availableAt = availableAt.UTC()
	updated := 0
	for key, period := range info.Periods {
		if period.AvailableAt == nil {
			at := availableAt
			period.AvailableAt = &at
			period.AvailabilitySource = source
			period.BacktestSafe = FinancialPeriodAvailabilityStatus(period) != FinancialAvailabilityUnsafeUnverifiedAvailable
			period.Warnings = removeString(period.Warnings, "available_at_missing")
			if period.PublishDate == nil && !containsString(period.Warnings, "publish_date_missing") {
				period.Warnings = append(period.Warnings, "publish_date_missing")
			}
			info.Periods[key] = period
			updated++
		}
	}
	info.Quality = ValidateFinancialDataQuality(info, time.Time{})
	return updated
}

func ValidateFinancialDataQuality(info *BilancoInfo, asOf time.Time) FinancialDataQuality {
	if info == nil {
		return FinancialDataQuality{MetadataVersion: FinancialMetadataVersion}
	}
	keys := make([]string, 0, len(info.Periods))
	for key := range info.Periods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	missingPublish := []string{}
	missingAvailable := []string{}
	unsafe := []string{}
	invalidChronology := []string{}
	published := 0
	available := 0
	for _, key := range keys {
		period := info.Periods[key]
		chronologyWarnings := FinancialPeriodChronologyWarnings(period)
		if len(chronologyWarnings) > 0 && !containsString(unsafe, key) {
			unsafe = append(unsafe, key)
		}
		if len(chronologyWarnings) > 0 && !containsString(invalidChronology, key) {
			invalidChronology = append(invalidChronology, key)
		}
		if period.PublishDate == nil {
			missingPublish = append(missingPublish, key)
		} else {
			published++
		}
		availableAt := EffectiveFinancialAvailableAt(period)
		if availableAt == nil {
			missingAvailable = append(missingAvailable, key)
			unsafe = append(unsafe, key)
			continue
		}
		available++
		if FinancialPeriodAvailabilityStatus(period) == FinancialAvailabilityUnsafeUnverifiedAvailable {
			if !containsString(unsafe, key) {
				unsafe = append(unsafe, key)
			}
			continue
		}
		if !asOf.IsZero() && availableAt.After(asOf) {
			if !containsString(unsafe, key) {
				unsafe = append(unsafe, key)
			}
		}
	}
	warnings := []string{}
	if len(missingPublish) > 0 {
		warnings = append(warnings, "financial_publish_dates_missing")
	}
	if len(missingAvailable) > 0 {
		warnings = append(warnings, "financial_available_at_missing")
	}
	if len(unsafe) > 0 {
		warnings = append(warnings, "financial_data_not_backtest_safe")
	}
	for _, key := range invalidChronology {
		for _, warning := range FinancialPeriodChronologyWarnings(info.Periods[key]) {
			if !containsString(warnings, warning) {
				warnings = append(warnings, warning)
			}
		}
	}
	if len(invalidChronology) > 0 && !containsString(warnings, "financial_period_chronology_invalid") {
		warnings = append(warnings, "financial_period_chronology_invalid")
	}
	reconciliationChecks := ValidateFinancialReconciliation(info)
	financiallyConsistent := len(reconciliationChecks) > 0
	hardReconciliationChecks := 0
	for _, check := range reconciliationChecks {
		if !check.MissingInputs {
			hardReconciliationChecks++
		}
		if !check.Passed && !check.MissingInputs {
			financiallyConsistent = false
			if check.Warning != "" && !containsString(warnings, check.Warning) {
				warnings = append(warnings, check.Warning)
			}
		} else if check.MissingInputs && check.Warning != "" && !containsString(warnings, check.Warning) {
			warnings = append(warnings, check.Warning)
		}
	}
	if len(reconciliationChecks) == 0 || hardReconciliationChecks == 0 {
		financiallyConsistent = false
		warnings = append(warnings, "balance_sheet_reconciliation_inputs_missing")
	}
	coverage := 0.0
	availableCoverage := 0.0
	if len(keys) > 0 {
		coverage = float64(published) / float64(len(keys))
		availableCoverage = float64(available) / float64(len(keys))
	}
	return FinancialDataQuality{
		BacktestSafe:              len(keys) > 0 && len(unsafe) == 0,
		FinanciallyConsistent:     financiallyConsistent,
		PublishDateCoverage:       coverage,
		AvailableAtCoverage:       availableCoverage,
		PeriodCount:               len(keys),
		ReconciliationChecks:      reconciliationChecks,
		MissingPublishPeriods:     missingPublish,
		MissingAvailableAtPeriods: missingAvailable,
		UnsafeBacktestPeriods:     unsafe,
		InvalidChronologyPeriods:  invalidChronology,
		Warnings:                  warnings,
		ValidatedAt:               time.Now().UTC(),
		MetadataVersion:           FinancialMetadataVersion,
	}
}

func ValidateFinancialReconciliation(info *BilancoInfo) []FinancialReconciliationCheck {
	if info == nil {
		return nil
	}
	keys := make([]string, 0, len(info.Periods))
	for key := range info.Periods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	checks := make([]FinancialReconciliationCheck, 0, len(keys))
	for _, key := range keys {
		period := info.Periods[key]
		if !hasAnyReconciliationAnchor(info, period.FiscalYear, period.FiscalQuarter) {
			continue
		}
		assets, okAssets := bilancoValue(info, "1BL", period.FiscalYear, period.FiscalQuarter)
		totalSources, okTotalSources := bilancoValue(info, "2ODB", period.FiscalYear, period.FiscalQuarter)
		if okAssets && okTotalSources {
			checks = append(checks, totalReconciliationCheck(key, "assets_equals_total_sources", []string{"1BL", "2ODB"}, assets, totalSources))
			continue
		}
		financialAssets, okFinancialAssets := bilancoValue(info, "1Z", period.FiscalYear, period.FiscalQuarter)
		totalLiabilities, okTotalLiabilities := bilancoValue(info, "2Z", period.FiscalYear, period.FiscalQuarter)
		if okFinancialAssets && okTotalLiabilities {
			checks = append(checks, totalReconciliationCheck(key, "assets_equals_total_liabilities", []string{"1Z", "2Z"}, financialAssets, totalLiabilities))
			continue
		}
		factoringAssets, okFactoringAssets := bilancoValue(info, "A1AK", period.FiscalYear, period.FiscalQuarter)
		factoringLiabilities, okFactoringLiabilities := bilancoValue(info, "A2OF", period.FiscalYear, period.FiscalQuarter)
		if okFactoringAssets && okFactoringLiabilities {
			checks = append(checks, totalReconciliationCheck(key, "assets_equals_total_liabilities", []string{"A1AK", "A2OF"}, factoringAssets, factoringLiabilities))
			continue
		}
		currentLiabilities, okCurrent := bilancoValue(info, "2A", period.FiscalYear, period.FiscalQuarter)
		nonCurrentLiabilities, okNonCurrent := bilancoValue(info, "2B", period.FiscalYear, period.FiscalQuarter)
		equity, okEquity := bilancoValue(info, "2N", period.FiscalYear, period.FiscalQuarter)
		check := FinancialReconciliationCheck{
			PeriodKey:   key,
			Check:       "assets_equals_liabilities_plus_equity",
			InputFields: []string{"1BL", "2A", "2B", "2N"},
		}
		if !okAssets || !okCurrent || !okNonCurrent || !okEquity {
			check.Warning = "balance_sheet_reconciliation_inputs_missing"
			check.MissingInputs = true
			checks = append(checks, check)
			continue
		}
		expected := currentLiabilities + nonCurrentLiabilities + equity
		diff := assets - expected
		tolerance := math.Max(math.Abs(assets)*0.01, 1)
		check.Actual = assets
		check.Expected = expected
		check.Difference = diff
		check.Tolerance = tolerance
		check.Passed = math.Abs(diff) <= tolerance
		if !check.Passed {
			check.Warning = "balance_sheet_reconciliation_failed"
		}
		checks = append(checks, check)
	}
	return checks
}

func hasAnyReconciliationAnchor(info *BilancoInfo, year int, quarter int) bool {
	for _, code := range []string{"1BL", "2ODB", "1Z", "2Z", "A1AK", "A2OF", "2A", "2B", "2N"} {
		if _, ok := bilancoValue(info, code, year, quarter); ok {
			return true
		}
	}
	return false
}

func totalReconciliationCheck(periodKey string, name string, inputFields []string, assets float64, totalSources float64) FinancialReconciliationCheck {
	check := FinancialReconciliationCheck{
		PeriodKey:   periodKey,
		Check:       name,
		InputFields: inputFields,
	}
	diff := assets - totalSources
	tolerance := math.Max(math.Abs(assets)*0.01, 1)
	check.Actual = assets
	check.Expected = totalSources
	check.Difference = diff
	check.Tolerance = tolerance
	check.Passed = math.Abs(diff) <= tolerance
	if !check.Passed {
		check.Warning = "total_sources_reconciliation_failed"
	}
	return check
}

func bilancoValue(info *BilancoInfo, code string, year int, quarter int) (float64, bool) {
	if info == nil || year <= 0 || quarter < 1 || quarter > 4 {
		return 0, false
	}
	field, ok := info.Data[code]
	if !ok {
		return 0, false
	}
	values := field.Years[strconv.Itoa(year)]
	index := indexFromFiscalQuarter(quarter)
	if index < 0 || index >= len(values) || values[index] == nil {
		return 0, false
	}
	return *values[index], true
}

func indexFromFiscalQuarter(quarter int) int {
	switch quarter {
	case 4:
		return 0
	case 3:
		return 1
	case 2:
		return 2
	case 1:
		return 3
	default:
		return -1
	}
}

func AppendLineage(info *BilancoInfo, event DataLineageEvent) {
	if info == nil {
		return
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.Version == "" {
		event.Version = FinancialMetadataVersion
	}
	info.Lineage = append(info.Lineage, event)
}

func EffectiveFinancialAvailableAt(period FinancialPeriod) *time.Time {
	if period.AvailableAt != nil {
		return period.AvailableAt
	}
	if period.PublishDate != nil {
		return period.PublishDate
	}
	if !period.FetchedAt.IsZero() {
		at := period.FetchedAt.UTC()
		return &at
	}
	return nil
}

func shouldUseFetchedAtAvailability(period FinancialPeriod) bool {
	if period.PublishDate != nil || period.FetchedAt.IsZero() {
		return false
	}
	if period.AvailableAt == nil {
		return true
	}
	if !period.FetchedAt.Before(*period.AvailableAt) {
		return false
	}
	switch period.AvailabilitySource {
	case "local_json_calculation_at", "local_json_fetch_at", "local_json_merge_at", "local_json_available_at", "provided_available_at":
		return true
	default:
		return false
	}
}

func ensureFinancialPeriodAvailability(period FinancialPeriod) FinancialPeriod {
	if period.PublishDate == nil {
		if !containsString(period.Warnings, "publish_date_missing") {
			period.Warnings = append(period.Warnings, "publish_date_missing")
		}
	} else {
		period.Warnings = removeString(period.Warnings, "publish_date_missing")
		if period.AvailableAt == nil {
			at := period.PublishDate.UTC()
			period.AvailableAt = &at
			period.AvailabilitySource = "kap_publish_date"
		}
	}
	if period.AvailableAt == nil && !period.FetchedAt.IsZero() {
		at := period.FetchedAt.UTC()
		period.AvailableAt = &at
		period.AvailabilitySource = firstNonEmpty(period.AvailabilitySource, "fetched_at")
	}
	if period.AvailableAt == nil {
		if !containsString(period.Warnings, "available_at_missing") {
			period.Warnings = append(period.Warnings, "available_at_missing")
		}
		period.BacktestSafe = false
		return period
	}
	period.AvailabilitySource = firstNonEmpty(period.AvailabilitySource, "provided_available_at")
	period.Warnings = removeString(period.Warnings, "available_at_missing")
	for _, warning := range FinancialPeriodChronologyWarnings(period) {
		if !containsString(period.Warnings, warning) {
			period.Warnings = append(period.Warnings, warning)
		}
	}
	if !FinancialPeriodChronologySafe(period) {
		if !containsString(period.Warnings, "financial_period_chronology_invalid") {
			period.Warnings = append(period.Warnings, "financial_period_chronology_invalid")
		}
		period.BacktestSafe = false
		return period
	}
	period.Warnings = removeString(period.Warnings, "financial_period_chronology_invalid")
	period.BacktestSafe = FinancialPeriodAvailabilityStatus(period) != FinancialAvailabilityUnsafeUnverifiedAvailable
	return period
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
