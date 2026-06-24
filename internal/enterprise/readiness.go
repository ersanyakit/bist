package enterprise

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"

	"hissebot/internal/audit"
	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/services/financials"
	"hissebot/internal/services/universe"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type Report struct {
	Status string  `json:"status"`
	Mode   string  `json:"mode,omitempty"`
	Score  float64 `json:"score"`
	Checks []Check `json:"checks"`
}

type Check struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Severity string   `json:"severity"`
	Message  string   `json:"message,omitempty"`
	Details  []string `json:"details,omitempty"`
}

type ReadinessOptions struct {
	Mode string
}

func CheckReadiness(ctx context.Context, cfg config.Config, store *storage.EquityStore) Report {
	return CheckReadinessWithOptions(ctx, cfg, store, ReadinessOptions{Mode: "research"})
}

func CheckReadinessWithOptions(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts ReadinessOptions) Report {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "research"
	}
	report := Report{Status: "pass", Mode: mode}
	report.add(checkSecurity(cfg))
	report.add(checkFinancialJSONVersionStore(ctx, store))
	report.add(checkFinancialPublishDateCoverage(ctx, store))
	report.add(checkLookaheadBiasGuard(ctx, store))
	if mode == "production" {
		report.add(checkProductionDataPolicy(ctx, store))
	}
	report.add(checkValuationAssumptions(cfg))
	report.add(checkGoldenFinancialRatios(cfg))
	report.add(checkUniverse(ctx, cfg, store))
	report.add(checkFinancialReconciliation(ctx, store))
	report.add(checkAuditLedger(cfg))
	failures := 0
	for _, check := range report.Checks {
		if check.Status == "fail" {
			failures++
			report.Status = "fail"
		}
	}
	if len(report.Checks) > 0 {
		report.Score = 100 * float64(len(report.Checks)-failures) / float64(len(report.Checks))
	}
	return report
}

func (r *Report) add(check Check) {
	if check.Status == "" {
		check.Status = "pass"
	}
	if check.Severity == "" {
		check.Severity = "info"
	}
	r.Checks = append(r.Checks, check)
}

func checkSecurity(cfg config.Config) Check {
	issues := config.ValidateSecurity(cfg)
	if len(issues) == 0 {
		return Check{Name: "security", Status: "pass", Severity: "critical"}
	}
	details := make([]string, 0, len(issues))
	for _, issue := range issues {
		details = append(details, issue.Code+": "+issue.Message)
	}
	return Check{
		Name:     "security",
		Status:   "fail",
		Severity: "critical",
		Message:  "security configuration is not enterprise-ready",
		Details:  details,
	}
}

func checkFinancialJSONVersionStore(ctx context.Context, store *storage.EquityStore) Check {
	equities, err := store.List()
	if err != nil {
		return Check{Name: "financial_json_version_store", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	checked := 0
	versionCount := 0
	restatementCount := 0
	missing := []string{}
	invalid := []string{}
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return Check{Name: "financial_json_version_store", Status: "fail", Severity: "critical", Message: ctx.Err().Error()}
		default:
		}
		if equity.BilancoInfo == nil || len(equity.BilancoInfo.Data) == 0 {
			continue
		}
		checked++
		var versions domain.FinancialStatementVersionStore
		if err := util.ReadJSON(store.FinancialStatementVersionsPath(equity.Ticker), &versions); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, equity.Ticker)
				continue
			}
			return Check{Name: "financial_json_version_store", Status: "fail", Severity: "critical", Message: err.Error()}
		}
		if versions.SchemaVersion != domain.FinancialStatementStoreVersion || len(versions.Versions) == 0 || len(versions.Latest) == 0 {
			invalid = append(invalid, equity.Ticker)
			continue
		}
		for _, version := range versions.Versions {
			if version.ID == "" || version.PeriodKey == "" || version.FactDigest == "" {
				invalid = append(invalid, equity.Ticker)
				break
			}
			if version.IsRestatement {
				restatementCount++
			}
			versionCount++
		}
	}
	if checked == 0 {
		return Check{Name: "financial_json_version_store", Status: "fail", Severity: "critical", Message: "no JSON financial statements were found"}
	}
	details := []string{
		"financial_equities_checked=" + strconv.Itoa(checked),
		"statement_versions=" + strconv.Itoa(versionCount),
		"restatements=" + strconv.Itoa(restatementCount),
	}
	if len(missing) > 0 {
		details = append(details, "missing_statement_version_files="+strings.Join(firstStrings(missing, 25), ","))
	}
	if len(invalid) > 0 {
		details = append(details, "invalid_statement_version_files="+strings.Join(firstStrings(invalid, 25), ","))
	}
	if len(missing) > 0 || len(invalid) > 0 || versionCount == 0 {
		return Check{Name: "financial_json_version_store", Status: "fail", Severity: "critical", Message: "JSON statement version store coverage is incomplete", Details: details}
	}
	return Check{Name: "financial_json_version_store", Status: "pass", Severity: "critical", Details: details}
}

func checkValuationAssumptions(cfg config.Config) Check {
	if strings.TrimSpace(cfg.ValuationAssumptionsFile) == "" {
		return Check{Name: "valuation_assumptions", Status: "fail", Severity: "critical", Message: "valuation assumption file is not configured"}
	}
	if _, err := os.Stat(cfg.ValuationAssumptionsFile); err != nil {
		return Check{Name: "valuation_assumptions", Status: "fail", Severity: "critical", Message: "valuation assumption file is missing: " + err.Error()}
	}
	raw, err := os.ReadFile(cfg.ValuationAssumptionsFile)
	if err != nil {
		return Check{Name: "valuation_assumptions", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	var payload valuationAssumptionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Check{Name: "valuation_assumptions", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	missing := []string{}
	for _, key := range []string{"risk_free_rate", "beta", "equity_risk_premium", "wacc", "terminal_growth", "fcf_growth"} {
		if _, ok := payload.Default[key]; !ok {
			missing = append(missing, "default."+key)
		}
	}
	for _, sector := range []string{"banka", "sigorta", "gayrimenkul"} {
		if len(payload.Sectors[sector]) == 0 {
			missing = append(missing, "sectors."+sector)
		}
	}
	details := []string{}
	if payload.Metadata.ApprovalStatus != "" {
		details = append(details, "approval_status="+payload.Metadata.ApprovalStatus)
	}
	if len(missing) > 0 {
		return Check{Name: "valuation_assumptions", Status: "fail", Severity: "critical", Message: "valuation assumption store is incomplete", Details: missing}
	}
	return Check{Name: "valuation_assumptions", Status: "pass", Severity: "critical", Details: details}
}

type valuationAssumptionPayload struct {
	Metadata struct {
		ApprovalStatus string `json:"approval_status"`
	} `json:"metadata"`
	Default map[string]float64            `json:"default"`
	Sectors map[string]map[string]float64 `json:"sectors"`
}

func checkGoldenFinancialRatios(cfg config.Config) Check {
	if strings.TrimSpace(cfg.GoldenFinancialRatiosFile) == "" {
		return Check{Name: "golden_financial_ratios", Status: "fail", Severity: "critical", Message: "golden financial ratios file is not configured"}
	}
	report, err := financials.ValidateGoldenRatios(cfg.GoldenFinancialRatiosFile)
	if err != nil {
		return Check{Name: "golden_financial_ratios", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	details := []string{
		"cases=" + strconv.Itoa(report.Cases),
		"failures=" + strconv.Itoa(report.Failures),
		"version=" + report.Version,
	}
	if report.Status != "pass" {
		details = append(details, firstStrings(report.Errors, 10)...)
		return Check{Name: "golden_financial_ratios", Status: "fail", Severity: "critical", Message: "financial formula golden dataset validation failed", Details: details}
	}
	return Check{Name: "golden_financial_ratios", Status: "pass", Severity: "critical", Details: details}
}

func checkUniverse(ctx context.Context, cfg config.Config, store *storage.EquityStore) Check {
	report, err := universe.Validate(ctx, cfg, store)
	if err != nil {
		return Check{Name: "survivorship_universe", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	if report.Status != "pass" {
		details := append([]string{}, report.Warnings...)
		if len(report.MissingTickers) > 0 {
			details = append(details, "missing_tickers="+strings.Join(report.MissingTickers, ","))
		}
		return Check{Name: "survivorship_universe", Status: "fail", Severity: "critical", Message: "listed/delisted universe coverage is incomplete", Details: details}
	}
	return Check{Name: "survivorship_universe", Status: "pass", Severity: "critical"}
}

func checkFinancialReconciliation(ctx context.Context, store *storage.EquityStore) Check {
	report, err := financials.Reconcile(ctx, store)
	if err != nil {
		return Check{Name: "financial_reconciliation", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	if report.PeriodChecks == 0 {
		return Check{Name: "financial_reconciliation", Status: "fail", Severity: "critical", Message: "no financial reconciliation checks were produced"}
	}
	if report.Failures > 0 {
		details := []string{
			"hard_failures=" + strconv.Itoa(report.Failures),
			"data_gaps=" + strconv.Itoa(report.DataGaps),
		}
		for _, sample := range report.Samples {
			details = append(details, sample.Ticker+" "+sample.PeriodKey+" "+sample.Warning)
		}
		return Check{Name: "financial_reconciliation", Status: "fail", Severity: "critical", Message: "financial reconciliation failures detected", Details: details}
	}
	details := []string{
		"equities_checked=" + strconv.Itoa(report.EquitiesChecked),
		"period_checks=" + strconv.Itoa(report.PeriodChecks),
		"data_gaps=" + strconv.Itoa(report.DataGaps),
	}
	if report.DataGaps > 0 {
		for _, sample := range report.Samples {
			details = append(details, sample.Ticker+" "+sample.PeriodKey+" "+sample.Warning)
		}
		return Check{Name: "financial_reconciliation", Status: "fail", Severity: "critical", Message: "financial reconciliation has unresolved input gaps", Details: details}
	}
	return Check{Name: "financial_reconciliation", Status: "pass", Severity: "critical", Details: details}
}

func checkFinancialPublishDateCoverage(ctx context.Context, store *storage.EquityStore) Check {
	stats, err := financialStatementVersionStats(ctx, store)
	if err != nil {
		return Check{Name: "financial_publish_date_coverage", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	details := []string{
		"statement_versions=" + strconv.Itoa(stats.totalVersions),
		"publish_dates=" + strconv.Itoa(stats.publishDates),
		"missing_publish_dates=" + strconv.Itoa(stats.totalVersions-stats.publishDates),
		"verified_publish_date_coverage=" + percent(stats.publishDates, stats.totalVersions),
		"conservative_available_at_without_publish_date=" + strconv.Itoa(stats.conservativeAvailableAt),
		"unsafe_unverified_available_at=" + strconv.Itoa(stats.unsafeUnverified),
		"available_at=" + strconv.Itoa(stats.availableAt),
	}
	details = append(details, countDetails("missing_publish_date_sources", stats.missingPublishBySource, 12)...)
	if stats.totalVersions == 0 {
		return Check{Name: "financial_publish_date_coverage", Status: "fail", Severity: "critical", Message: "no statement versions available", Details: details}
	}
	if stats.unsafeUnverified > 0 {
		details = append(details, firstStrings(stats.unsafeUnverifiedSamples, 20)...)
		return Check{Name: "financial_publish_date_coverage", Status: "fail", Severity: "critical", Message: "publish-date coverage has untrusted available_at fallbacks", Details: details}
	}
	if stats.publishDates != stats.totalVersions {
		details = append(details, firstStrings(stats.missingSamples, 20)...)
		return Check{Name: "financial_publish_date_coverage", Status: "pass", Severity: "critical", Message: "publish-date coverage is incomplete; all missing periods are constrained by conservative available_at controls", Details: details}
	}
	return Check{Name: "financial_publish_date_coverage", Status: "pass", Severity: "critical", Details: details}
}

func checkLookaheadBiasGuard(ctx context.Context, store *storage.EquityStore) Check {
	stats, err := financialStatementVersionStats(ctx, store)
	if err != nil {
		return Check{Name: "lookahead_bias_guard", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	details := []string{
		"execution_policy=financial_statement_available_at_execute_next_bar",
		"statement_versions=" + strconv.Itoa(stats.totalVersions),
		"publish_dates=" + strconv.Itoa(stats.publishDates),
		"conservative_available_at_without_publish_date=" + strconv.Itoa(stats.conservativeAvailableAt),
		"unsafe_unverified_available_at=" + strconv.Itoa(stats.unsafeUnverified),
		"available_at=" + strconv.Itoa(stats.availableAt),
		"missing_available_at=" + strconv.Itoa(stats.totalVersions-stats.availableAt),
	}
	if stats.totalVersions == 0 || stats.availableAt != stats.totalVersions || stats.unsafeUnverified > 0 {
		details = append(details, firstStrings(stats.missingAvailableSamples, 20)...)
		details = append(details, firstStrings(stats.unsafeUnverifiedSamples, 20)...)
		return Check{Name: "lookahead_bias_guard", Status: "fail", Severity: "critical", Message: "fundamental signals cannot be backtest-safe until every statement version has an available_at timestamp", Details: details}
	}
	return Check{Name: "lookahead_bias_guard", Status: "pass", Severity: "critical", Details: details}
}

func checkProductionDataPolicy(ctx context.Context, store *storage.EquityStore) Check {
	stats, err := financialStatementVersionStats(ctx, store)
	if err != nil {
		return Check{Name: "production_data_policy", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	quarantined := stats.totalVersions - stats.publishDates
	details := []string{
		"policy=verified_publish_date_only_for_trading",
		"production_eligible_statement_versions=" + strconv.Itoa(stats.publishDates),
		"production_quarantined_statement_versions=" + strconv.Itoa(quarantined),
		"verified_publish_date_coverage=" + percent(stats.publishDates, stats.totalVersions),
		"unsafe_unverified_available_at=" + strconv.Itoa(stats.unsafeUnverified),
		"required_runtime_behavior=reject_unverified_fundamental_events",
	}
	details = append(details, countDetails("quarantine_sources", stats.missingPublishBySource, 12)...)
	if stats.totalVersions == 0 {
		return Check{Name: "production_data_policy", Status: "fail", Severity: "critical", Message: "no statement versions available for production", Details: details}
	}
	if stats.publishDates == 0 {
		return Check{Name: "production_data_policy", Status: "fail", Severity: "critical", Message: "no verified publish-date statement versions are eligible for production", Details: details}
	}
	if stats.unsafeUnverified > 0 {
		details = append(details, firstStrings(stats.unsafeUnverifiedSamples, 20)...)
		return Check{Name: "production_data_policy", Status: "fail", Severity: "critical", Message: "production cannot quarantine unsafe available_at sources", Details: details}
	}
	return Check{Name: "production_data_policy", Status: "pass", Severity: "critical", Message: "production trading is limited to verified publish-date financial events; unverified periods are quarantined", Details: details}
}

type statementVersionStats struct {
	totalVersions           int
	publishDates            int
	conservativeAvailableAt int
	unsafeUnverified        int
	availableAt             int
	missingSamples          []string
	missingAvailableSamples []string
	unsafeUnverifiedSamples []string
	missingPublishBySource  map[string]int
}

func financialStatementVersionStats(ctx context.Context, store *storage.EquityStore) (statementVersionStats, error) {
	equities, err := store.List()
	if err != nil {
		return statementVersionStats{}, err
	}
	stats := statementVersionStats{missingPublishBySource: map[string]int{}}
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}
		if equity.BilancoInfo == nil || len(equity.BilancoInfo.Data) == 0 {
			continue
		}
		var versions domain.FinancialStatementVersionStore
		if err := util.ReadJSON(store.FinancialStatementVersionsPath(equity.Ticker), &versions); err != nil {
			continue
		}
		for _, version := range versions.Versions {
			stats.totalVersions++
			if version.PublishDate != nil {
				stats.publishDates++
			} else {
				source := strings.TrimSpace(version.AvailabilitySource)
				if source == "" {
					source = "unknown"
				}
				stats.missingPublishBySource[source]++
				switch domain.FinancialStatementVersionAvailabilityStatus(version) {
				case domain.FinancialAvailabilityConservativeAvailableAt:
					stats.conservativeAvailableAt++
				default:
					stats.unsafeUnverified++
					if len(stats.unsafeUnverifiedSamples) < 25 {
						stats.unsafeUnverifiedSamples = append(stats.unsafeUnverifiedSamples, equity.Ticker+" "+version.PeriodKey+" source="+source)
					}
				}
			}
			if version.AvailableAt != nil {
				stats.availableAt++
			} else if len(stats.missingAvailableSamples) < 25 {
				stats.missingAvailableSamples = append(stats.missingAvailableSamples, equity.Ticker+" "+version.PeriodKey)
			}
			if version.PublishDate == nil && len(stats.missingSamples) < 25 {
				stats.missingSamples = append(stats.missingSamples, equity.Ticker+" "+version.PeriodKey)
			}
		}
	}
	return stats, nil
}

func percent(numerator int, denominator int) string {
	if denominator == 0 {
		return "0.00%"
	}
	return strconv.FormatFloat(100*float64(numerator)/float64(denominator), 'f', 2, 64) + "%"
}

func countDetails(prefix string, counts map[string]int, limit int) []string {
	if len(counts) == 0 || limit <= 0 {
		return nil
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for i, key := range keys {
		if i >= limit {
			out = append(out, prefix+"_more="+strconv.Itoa(len(keys)-limit))
			break
		}
		out = append(out, prefix+"."+key+"="+strconv.Itoa(counts[key]))
	}
	return out
}

func checkAuditLedger(cfg config.Config) Check {
	report, err := audit.Verify(cfg.AuditLogPath)
	if err != nil {
		return Check{Name: "audit_ledger", Status: "fail", Severity: "critical", Message: err.Error()}
	}
	details := []string{
		"events=" + strconv.Itoa(report.Events),
	}
	if report.LastHash != "" {
		details = append(details, "last_hash="+report.LastHash)
	}
	if report.Status != "pass" {
		details = append(details, firstStrings(report.Errors, 10)...)
		return Check{Name: "audit_ledger", Status: "fail", Severity: "critical", Message: "audit log hash-chain verification failed", Details: details}
	}
	return Check{Name: "audit_ledger", Status: "pass", Severity: "critical", Details: details}
}

func firstStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	out := append([]string{}, values[:limit]...)
	out = append(out, "...+"+strconv.Itoa(len(values)-limit)+" more")
	return out
}
