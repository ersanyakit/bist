package enterprise

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/audit"
	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/services/financials"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

func TestCheckReadinessFailsResearchOnlyConfiguration(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2}); err != nil {
		t.Fatal(err)
	}
	report := CheckReadiness(context.Background(), config.Config{
		EndpointToken: "secret",
		DataDir:       dir,
		EquitiesDir:   store.Root(),
	}, store)
	if report.Status != "fail" {
		t.Fatalf("status = %q, want fail: %+v", report.Status, report)
	}
	if !hasCheck(report, "financial_json_version_store", "fail") {
		t.Fatalf("expected financial_json_version_store failure, got %+v", report.Checks)
	}
}

func TestCheckReadinessPassesRequiredGovernanceInputs(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	info := reconciliationReadyBilanco("ALGYO")
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2, BilancoInfo: info}); err != nil {
		t.Fatal(err)
	}
	versionStore := domain.UpsertStatementVersions(domain.FinancialStatementVersionStore{}, "ALGYO", domain.BuildStatementVersions(info, fixedNow()), fixedNow())
	if err := util.WriteJSON(store.FinancialStatementVersionsPath("ALGYO"), versionStore); err != nil {
		t.Fatal(err)
	}
	listed := filepath.Join(dir, "listed.json")
	delisted := filepath.Join(dir, "delisted.json")
	assumptions := filepath.Join(dir, "valuation_assumptions.json")
	golden := filepath.Join(dir, "golden_financial_ratios.json")
	auditLog := filepath.Join(dir, "audit", "events.jsonl")
	if err := util.WriteJSON(listed, []map[string]string{{"ticker": "ALGYO"}}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(delisted, []map[string]string{{"ticker": "OLDCO"}}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(assumptions, map[string]any{
		"metadata": map[string]string{"approval_status": "unit_test_approved"},
		"default": map[string]float64{
			"risk_free_rate":      0.30,
			"beta":                1.00,
			"equity_risk_premium": 0.08,
			"wacc":                0.14,
			"terminal_growth":     0.03,
			"fcf_growth":          0.05,
		},
		"sectors": map[string]map[string]float64{
			"banka":       {"wacc": 0.14},
			"sigorta":     {"wacc": 0.14},
			"gayrimenkul": {"wacc": 0.14},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(golden, unitGoldenSuite()); err != nil {
		t.Fatal(err)
	}
	if _, err := audit.Append(auditLog, audit.Event{Action: "unit_test", Entity: "readiness"}); err != nil {
		t.Fatal(err)
	}
	report := CheckReadiness(context.Background(), config.Config{
		EndpointToken:             "secret",
		UniverseFile:              listed,
		DelistedUniverseFile:      delisted,
		ValuationAssumptionsFile:  assumptions,
		GoldenFinancialRatiosFile: golden,
		AuditLogPath:              auditLog,
	}, store)
	if report.Status != "pass" {
		t.Fatalf("status = %q, want pass: %+v", report.Status, report)
	}
}

func TestFinancialPublishDateCoveragePassesConservativeAvailableAtControl(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	info := reconciliationReadyBilanco("ALGYO")
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2, BilancoInfo: info}); err != nil {
		t.Fatal(err)
	}
	versionStore := domain.UpsertStatementVersions(domain.FinancialStatementVersionStore{}, "ALGYO", domain.BuildStatementVersions(info, fixedNow()), fixedNow())
	availableAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	for i := range versionStore.Versions {
		versionStore.Versions[i].PublishDate = nil
		versionStore.Versions[i].AvailableAt = &availableAt
		versionStore.Versions[i].FetchedAt = availableAt
		versionStore.Versions[i].AvailabilitySource = "fetched_at"
	}
	if err := util.WriteJSON(store.FinancialStatementVersionsPath("ALGYO"), versionStore); err != nil {
		t.Fatal(err)
	}

	coverage := checkFinancialPublishDateCoverage(context.Background(), store)
	if coverage.Status != "pass" {
		t.Fatalf("coverage status = %q, want pass: %+v", coverage.Status, coverage)
	}
	lookahead := checkLookaheadBiasGuard(context.Background(), store)
	if lookahead.Status != "pass" {
		t.Fatalf("lookahead status = %q, want pass: %+v", lookahead.Status, lookahead)
	}
}

func TestFinancialPublishDateCoverageFailsUntrustedAvailableAtControl(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	info := reconciliationReadyBilanco("ALGYO")
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2, BilancoInfo: info}); err != nil {
		t.Fatal(err)
	}
	versionStore := domain.UpsertStatementVersions(domain.FinancialStatementVersionStore{}, "ALGYO", domain.BuildStatementVersions(info, fixedNow()), fixedNow())
	availableAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	for i := range versionStore.Versions {
		versionStore.Versions[i].PublishDate = nil
		versionStore.Versions[i].AvailableAt = &availableAt
		versionStore.Versions[i].AvailabilitySource = "vendor_supplied_date"
	}
	if err := util.WriteJSON(store.FinancialStatementVersionsPath("ALGYO"), versionStore); err != nil {
		t.Fatal(err)
	}

	coverage := checkFinancialPublishDateCoverage(context.Background(), store)
	if coverage.Status != "fail" {
		t.Fatalf("coverage status = %q, want fail: %+v", coverage.Status, coverage)
	}
	lookahead := checkLookaheadBiasGuard(context.Background(), store)
	if lookahead.Status != "fail" {
		t.Fatalf("lookahead status = %q, want fail: %+v", lookahead.Status, lookahead)
	}
}

func TestProductionDataPolicyPassesVerifiedPublishDateSubset(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	info := reconciliationReadyBilanco("ALGYO")
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2, BilancoInfo: info}); err != nil {
		t.Fatal(err)
	}
	versionStore := domain.UpsertStatementVersions(domain.FinancialStatementVersionStore{}, "ALGYO", domain.BuildStatementVersions(info, fixedNow()), fixedNow())
	if err := util.WriteJSON(store.FinancialStatementVersionsPath("ALGYO"), versionStore); err != nil {
		t.Fatal(err)
	}

	check := checkProductionDataPolicy(context.Background(), store)
	if check.Status != "pass" {
		t.Fatalf("production policy status = %q, want pass: %+v", check.Status, check)
	}
}

func TestProductionDataPolicyFailsWithoutVerifiedPublishDates(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	info := reconciliationReadyBilanco("ALGYO")
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2, BilancoInfo: info}); err != nil {
		t.Fatal(err)
	}
	versionStore := domain.UpsertStatementVersions(domain.FinancialStatementVersionStore{}, "ALGYO", domain.BuildStatementVersions(info, fixedNow()), fixedNow())
	availableAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	for i := range versionStore.Versions {
		versionStore.Versions[i].PublishDate = nil
		versionStore.Versions[i].AvailableAt = &availableAt
		versionStore.Versions[i].FetchedAt = availableAt
		versionStore.Versions[i].AvailabilitySource = "fetched_at"
	}
	if err := util.WriteJSON(store.FinancialStatementVersionsPath("ALGYO"), versionStore); err != nil {
		t.Fatal(err)
	}

	check := checkProductionDataPolicy(context.Background(), store)
	if check.Status != "fail" {
		t.Fatalf("production policy status = %q, want fail: %+v", check.Status, check)
	}
}

func hasCheck(report Report, name string, status string) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func reconciliationReadyBilanco(ticker string) *domain.BilancoInfo {
	assets := 100.0
	currentLiabilities := 30.0
	nonCurrentLiabilities := 20.0
	equity := 50.0
	info := &domain.BilancoInfo{
		Ticker: ticker,
		Data: map[string]domain.BilancoField{
			"1BL": {Years: map[string][]*float64{"2026": {&assets}}},
			"2A":  {Years: map[string][]*float64{"2026": {&currentLiabilities}}},
			"2B":  {Years: map[string][]*float64{"2026": {&nonCurrentLiabilities}}},
			"2N":  {Years: map[string][]*float64{"2026": {&equity}}},
		},
	}
	domain.NormalizeBilancoInfo(info, ticker)
	publishDate := time.Date(2026, 4, 30, 18, 0, 0, 0, time.UTC)
	for key, period := range info.Periods {
		period.PublishDate = &publishDate
		period.ReportDate = &publishDate
		period.BacktestSafe = true
		period.Warnings = nil
		info.Periods[key] = period
	}
	info.Quality = domain.ValidateFinancialDataQuality(info, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	return info
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
}

func unitGoldenSuite() financials.GoldenRatioSuite {
	return financials.GoldenRatioSuite{
		Version: financials.GoldenFinancialRatiosVersion,
		Cases: []financials.GoldenRatioCase{
			{
				Name: "current_ratio_unit",
				Input: domain.BilancoInfo{Ticker: "GOLDEN", Data: map[string]domain.BilancoField{
					"1A": {Years: map[string][]*float64{"2026": {floatPtr(200)}}},
					"2A": {Years: map[string][]*float64{"2026": {floatPtr(100)}}},
				}},
				Expected: map[string]domain.YearQuarter{
					"CariOran": {
						"2026": {Q4: floatPtr(2)},
					},
				},
			},
		},
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
