package domain

import (
	"testing"
	"time"
)

func TestFinancialDataQualityRequiresPublishDates(t *testing.T) {
	value := 10.0
	info := &BilancoInfo{
		Ticker:   "TEST",
		Source:   "isyatirim",
		Currency: "TRY",
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}

	NormalizeBilancoInfo(info, "TEST")
	quality := ValidateFinancialDataQuality(info, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if quality.BacktestSafe {
		t.Fatalf("expected missing publish date to be unsafe, got %+v", quality)
	}
	if quality.PublishDateCoverage != 0 {
		t.Fatalf("publish coverage = %.2f, want 0", quality.PublishDateCoverage)
	}
	if len(quality.MissingPublishPeriods) != 1 || quality.MissingPublishPeriods[0] != "2026-Q1" {
		t.Fatalf("missing publish periods = %#v, want 2026-Q1", quality.MissingPublishPeriods)
	}
	if len(quality.MissingAvailableAtPeriods) != 1 || quality.MissingAvailableAtPeriods[0] != "2026-Q1" {
		t.Fatalf("missing available-at periods = %#v, want 2026-Q1", quality.MissingAvailableAtPeriods)
	}
}

func TestFinancialDataQualityAllowsConservativeAvailableAtWithoutPublishDate(t *testing.T) {
	value := 10.0
	availableAt := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	info := &BilancoInfo{
		Ticker:   "TEST",
		Source:   "legacy_import",
		Currency: "TRY",
		Periods: map[string]FinancialPeriod{
			"2026-Q1": {
				Key:                "2026-Q1",
				FiscalYear:         2026,
				FiscalQuarter:      1,
				PeriodEnd:          FiscalPeriodEnd(2026, 1),
				AvailableAt:        &availableAt,
				AvailabilitySource: "local_json_import_at",
			},
		},
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}

	NormalizeBilancoInfo(info, "TEST")
	quality := ValidateFinancialDataQuality(info, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if !quality.BacktestSafe {
		t.Fatalf("expected available-at fallback to be backtest-safe, got %+v", quality)
	}
	if quality.PublishDateCoverage != 0 {
		t.Fatalf("publish coverage = %.2f, want 0", quality.PublishDateCoverage)
	}
	if quality.AvailableAtCoverage != 1 {
		t.Fatalf("available-at coverage = %.2f, want 1", quality.AvailableAtCoverage)
	}
}

func TestFinancialDataQualityRejectsUnverifiedAvailableAtSource(t *testing.T) {
	value := 10.0
	availableAt := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	info := &BilancoInfo{
		Ticker:   "TEST",
		Source:   "legacy_import",
		Currency: "TRY",
		Periods: map[string]FinancialPeriod{
			"2026-Q1": {
				Key:                "2026-Q1",
				FiscalYear:         2026,
				FiscalQuarter:      1,
				PeriodEnd:          FiscalPeriodEnd(2026, 1),
				AvailableAt:        &availableAt,
				AvailabilitySource: "vendor_timestamp",
			},
		},
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}

	NormalizeBilancoInfo(info, "TEST")
	quality := ValidateFinancialDataQuality(info, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if quality.BacktestSafe {
		t.Fatalf("expected unverified available_at source to be unsafe, got %+v", quality)
	}
	if quality.AvailableAtCoverage != 1 {
		t.Fatalf("available-at coverage = %.2f, want 1", quality.AvailableAtCoverage)
	}
	if len(quality.MissingAvailableAtPeriods) != 0 {
		t.Fatalf("missing available-at periods = %#v, want none", quality.MissingAvailableAtPeriods)
	}
	if len(quality.UnsafeBacktestPeriods) != 1 || quality.UnsafeBacktestPeriods[0] != "2026-Q1" {
		t.Fatalf("unsafe periods = %#v, want 2026-Q1", quality.UnsafeBacktestPeriods)
	}
	if info.Periods["2026-Q1"].BacktestSafe {
		t.Fatalf("expected period backtest_safe=false for unverified source")
	}
}

func TestApplyBilancoSourceMetadataPrefersEarlierFetchedAtAvailability(t *testing.T) {
	value := 10.0
	localAvailableAt := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	fetchedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	info := &BilancoInfo{
		Ticker: "TEST",
		Periods: map[string]FinancialPeriod{
			"2026-Q1": {
				Key:                "2026-Q1",
				FiscalYear:         2026,
				FiscalQuarter:      1,
				PeriodEnd:          FiscalPeriodEnd(2026, 1),
				AvailableAt:        &localAvailableAt,
				AvailabilitySource: "local_json_calculation_at",
			},
		},
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}

	ApplyBilancoSourceMetadata(info, "isyatirim", "XI_29", "TRY", fetchedAt)
	got := info.Periods["2026-Q1"]
	if got.AvailableAt == nil || !got.AvailableAt.Equal(fetchedAt) {
		t.Fatalf("available_at = %v, want fetched_at %v", got.AvailableAt, fetchedAt)
	}
	if got.AvailabilitySource != "fetched_at" {
		t.Fatalf("availability source = %q", got.AvailabilitySource)
	}
}

func TestFinancialDataQualityAcceptsKnownPublishDatesBeforeAsOf(t *testing.T) {
	value := 10.0
	publishDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	info := &BilancoInfo{
		Ticker:   "TEST",
		Source:   "isyatirim",
		Currency: "TRY",
		Periods: map[string]FinancialPeriod{
			"2026-Q1": {
				Key:           "2026-Q1",
				FiscalYear:    2026,
				FiscalQuarter: 1,
				PeriodEnd:     FiscalPeriodEnd(2026, 1),
				PublishDate:   &publishDate,
			},
		},
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}

	NormalizeBilancoInfo(info, "TEST")
	quality := ValidateFinancialDataQuality(info, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if !quality.BacktestSafe {
		t.Fatalf("expected known publish date before as-of to be safe, got %+v", quality)
	}
	if quality.PublishDateCoverage != 1 {
		t.Fatalf("publish coverage = %.2f, want 1", quality.PublishDateCoverage)
	}
}

func TestFinancialDataQualityRejectsPublishDateBeforePeriodEnd(t *testing.T) {
	value := 10.0
	publishDate := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)
	info := &BilancoInfo{
		Ticker:   "TEST",
		Source:   "isyatirim",
		Currency: "TRY",
		Periods: map[string]FinancialPeriod{
			"2026-Q1": {
				Key:           "2026-Q1",
				FiscalYear:    2026,
				FiscalQuarter: 1,
				PeriodEnd:     FiscalPeriodEnd(2026, 1),
				PublishDate:   &publishDate,
			},
		},
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}

	NormalizeBilancoInfo(info, "TEST")
	quality := ValidateFinancialDataQuality(info, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if quality.BacktestSafe {
		t.Fatalf("publish date before period end must be unsafe, got %+v", quality)
	}
	if len(quality.InvalidChronologyPeriods) != 1 || quality.InvalidChronologyPeriods[0] != "2026-Q1" {
		t.Fatalf("invalid chronology periods = %#v, want 2026-Q1", quality.InvalidChronologyPeriods)
	}
	if !containsString(quality.Warnings, "financial_period_chronology_invalid") || !containsString(quality.Warnings, "publish_date_before_period_end") {
		t.Fatalf("expected chronology warnings, got %+v", quality.Warnings)
	}
	if info.Periods["2026-Q1"].BacktestSafe {
		t.Fatalf("expected normalized period backtest_safe=false")
	}
}

func TestValidateFinancialReconciliationChecksBalanceSheetEquation(t *testing.T) {
	assets := 1000.0
	currentLiabilities := 250.0
	nonCurrentLiabilities := 350.0
	equity := 400.0
	info := &BilancoInfo{
		Ticker: "TEST",
		Data: map[string]BilancoField{
			"1BL": {Years: map[string][]*float64{"2026": {nil, nil, nil, &assets}}},
			"2A":  {Years: map[string][]*float64{"2026": {nil, nil, nil, &currentLiabilities}}},
			"2B":  {Years: map[string][]*float64{"2026": {nil, nil, nil, &nonCurrentLiabilities}}},
			"2N":  {Years: map[string][]*float64{"2026": {nil, nil, nil, &equity}}},
		},
	}
	NormalizeBilancoInfo(info, "TEST")

	checks := ValidateFinancialReconciliation(info)
	if len(checks) != 1 || !checks[0].Passed {
		t.Fatalf("expected reconciliation to pass, got %+v", checks)
	}
}

func TestValidateFinancialReconciliationFlagsBrokenBalanceSheetEquation(t *testing.T) {
	assets := 1200.0
	currentLiabilities := 250.0
	nonCurrentLiabilities := 350.0
	equity := 400.0
	info := &BilancoInfo{
		Ticker: "TEST",
		Data: map[string]BilancoField{
			"1BL": {Years: map[string][]*float64{"2026": {nil, nil, nil, &assets}}},
			"2A":  {Years: map[string][]*float64{"2026": {nil, nil, nil, &currentLiabilities}}},
			"2B":  {Years: map[string][]*float64{"2026": {nil, nil, nil, &nonCurrentLiabilities}}},
			"2N":  {Years: map[string][]*float64{"2026": {nil, nil, nil, &equity}}},
		},
	}
	NormalizeBilancoInfo(info, "TEST")

	checks := ValidateFinancialReconciliation(info)
	if len(checks) != 1 || checks[0].Passed || checks[0].Warning != "balance_sheet_reconciliation_failed" {
		t.Fatalf("expected reconciliation failure, got %+v", checks)
	}
	if info.Quality.FinanciallyConsistent {
		t.Fatalf("expected quality to be financially inconsistent, got %+v", info.Quality)
	}
}

func TestValidateFinancialReconciliationPrefersAuthoritativeTotalSources(t *testing.T) {
	assets := 1000.0
	totalSources := 1000.0
	currentLiabilities := 700.0
	nonCurrentLiabilities := 200.0
	equity := 300.0
	info := &BilancoInfo{
		Ticker: "TEST",
		Data: map[string]BilancoField{
			"1BL":  {Years: map[string][]*float64{"2026": {nil, nil, nil, &assets}}},
			"2ODB": {Years: map[string][]*float64{"2026": {nil, nil, nil, &totalSources}}},
			"2A":   {Years: map[string][]*float64{"2026": {nil, nil, nil, &currentLiabilities}}},
			"2B":   {Years: map[string][]*float64{"2026": {nil, nil, nil, &nonCurrentLiabilities}}},
			"2N":   {Years: map[string][]*float64{"2026": {nil, nil, nil, &equity}}},
		},
	}
	NormalizeBilancoInfo(info, "TEST")

	checks := ValidateFinancialReconciliation(info)
	if len(checks) != 1 || !checks[0].Passed || checks[0].Check != "assets_equals_total_sources" {
		t.Fatalf("expected authoritative total-source reconciliation to pass, got %+v", checks)
	}
}

func TestValidateFinancialReconciliationSupportsFinancialSectorTotalCodes(t *testing.T) {
	assets := 100.0
	totalLiabilities := 100.0
	info := &BilancoInfo{
		Ticker: "BANK",
		Data: map[string]BilancoField{
			"1Z": {Years: map[string][]*float64{"2026": {&assets}}},
			"2Z": {Years: map[string][]*float64{"2026": {&totalLiabilities}}},
		},
	}
	NormalizeBilancoInfo(info, "BANK")
	checks := ValidateFinancialReconciliation(info)
	if len(checks) != 1 || !checks[0].Passed || checks[0].Check != "assets_equals_total_liabilities" {
		t.Fatalf("expected financial-sector total reconciliation to pass, got %+v", checks)
	}
}

func TestValidateFinancialReconciliationSupportsFactoringSectorTotalCodes(t *testing.T) {
	assets := 100.0
	totalLiabilities := 100.0
	info := &BilancoInfo{
		Ticker: "FACTORING",
		Data: map[string]BilancoField{
			"A1AK": {Years: map[string][]*float64{"2026": {&assets}}},
			"A2OF": {Years: map[string][]*float64{"2026": {&totalLiabilities}}},
		},
	}
	NormalizeBilancoInfo(info, "FACTORING")
	checks := ValidateFinancialReconciliation(info)
	if len(checks) != 1 || !checks[0].Passed || checks[0].InputFields[0] != "A1AK" {
		t.Fatalf("expected factoring-sector total reconciliation to pass, got %+v", checks)
	}
}

func TestValidateFinancialReconciliationSkipsPeriodsWithoutBalanceSheetAnchors(t *testing.T) {
	revenue := 100.0
	info := &BilancoInfo{
		Ticker: "INCOMEONLY",
		Data: map[string]BilancoField{
			"3C": {Years: map[string][]*float64{"2026": {nil, nil, nil, &revenue}}},
		},
	}
	NormalizeBilancoInfo(info, "INCOMEONLY")
	checks := ValidateFinancialReconciliation(info)
	if len(checks) != 0 {
		t.Fatalf("expected no reconciliation checks for income-only period, got %+v", checks)
	}
}
