package professional

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"hissebot/internal/kapingest"
	"hissebot/internal/util"
)

const bankRegulatoryValuationFlag = "bank_sector_requires_regulatory_capital_and_asset_quality_model"

type bankRegulatoryMetricSpec struct {
	Name          string
	Label         string
	Aliases       []string
	RejectAliases []string
	RequireAny    []string
	MaxValue      float64
	Status        func(float64) string
	Interpret     func(float64) string
}

type bankRegulatoryCandidate struct {
	Metric      SectorFinancialMetric
	Complete    bool
	DateScore   int
	SourceScore float64
}

var bankRegulatoryMetricSpecs = []bankRegulatoryMetricSpec{
	{
		Name:    "capital_adequacy_ratio",
		Label:   "Sermaye yeterlilik rasyosu",
		Aliases: []string{"sermaye yeterlilik", "sermaye yeterliligi", "sermaye yeterliligi rasyosu", "capital adequacy", "capital adequacy ratio", "car ratio"},
		RequireAny: []string{
			"oran", "rasy", "ratio",
		},
		MaxValue: 0.50,
		Status:   bankCapitalAdequacyStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Sermaye yeterlilik rasyosu %.1f%%; banka sermaye tamponu icin ana duzenleyici metriktir.", value*100)
		},
	},
	{
		Name:    "cet1_ratio",
		Label:   "CET1 / cekirdek sermaye orani",
		Aliases: []string{"cekirdek sermaye", "çekirdek sermaye", "cet1", "common equity tier 1", "common equity tier1", "core tier 1", "core tier1"},
		RequireAny: []string{
			"oran", "rasy", "ratio", "cet1",
		},
		MaxValue: 0.50,
		Status:   bankCET1Status,
		Interpret: func(value float64) string {
			return fmt.Sprintf("CET1/cekirdek sermaye %.1f%%; zarar emme kapasitesi icin izlenir.", value*100)
		},
	},
	{
		Name:    "npl_ratio",
		Label:   "Takipteki krediler orani",
		Aliases: []string{"takipteki krediler orani", "takipteki kredi orani", "takip oran", "npl ratio", "non performing loan ratio", "non-performing loan ratio"},
		RequireAny: []string{
			"oran", "ratio",
		},
		MaxValue: 0.50,
		Status:   bankNPLStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Takipteki krediler orani %.1f%%; aktif kalitesi icin ana risk gostergesidir.", value*100)
		},
	},
	{
		Name:    "provision_coverage_ratio",
		Label:   "Karsilik kapsam orani",
		Aliases: []string{"karsilik kapsam", "karşılık kapsam", "karsilik karsilama", "karşılık karşılama", "npl coverage", "coverage ratio", "takipteki krediler karsilik orani", "takipteki krediler karşılık oranı"},
		RequireAny: []string{
			"kapsam", "karsilama", "karşılama", "coverage", "oran", "ratio",
		},
		MaxValue: 2.00,
		Status:   bankProvisionCoverageStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Karsilik kapsam orani %.1f%%; sorunlu kredi tamponu icin okunur.", value*100)
		},
	},
	{
		Name:    "net_interest_margin",
		Label:   "Net faiz marji",
		Aliases: []string{"net faiz marj", "net faiz marji", "net faiz marjı", "net interest margin"},
		RequireAny: []string{
			"marj", "margin",
		},
		MaxValue: 0.25,
		Status:   bankNIMStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Net faiz marji %.1f%%; fonlama maliyeti ve aktif getirisi arasindaki ana karlilik gostergesidir.", value*100)
		},
	},
	{
		Name:          "liquidity_coverage_ratio",
		Label:         "Likidite karsilama orani",
		Aliases:       []string{"likidite karsilama orani", "likidite karşılama oranı", "liquidity coverage ratio"},
		RejectAliases: []string{"asgari", "minimum", "zorunlu", "requirement", "gereken oran"},
		RequireAny: []string{
			"oran", "ratio",
		},
		MaxValue: 5.00,
		Status:   bankLCRStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Likidite karsilama orani %.1f%%; kisa vadeli likidite tamponu icin izlenir.", value*100)
		},
	},
	{
		Name:     "loan_to_deposit_ratio",
		Label:    "Kredi / mevduat orani",
		Aliases:  []string{"kredi mevduat", "krediler mevduat", "kredi/mevduat", "krediler/mevduat", "loan to deposit", "loan/deposit"},
		MaxValue: 2.50,
		Status:   loanDepositStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Kredi/mevduat orani %.1f%%; fonlama dengesi icin izlenir.", value*100)
		},
	},
	{
		Name:     "deposit_cost",
		Label:    "Mevduat maliyeti",
		Aliases:  []string{"mevduat maliyeti", "tl mevduat maliyeti", "deposit cost", "cost of deposits", "average cost of deposits"},
		MaxValue: 0.80,
		Status:   bankDepositCostStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Mevduat maliyeti %.1f%%; fonlama maliyeti ve marj baskisi icin izlenir.", value*100)
		},
	},
	{
		Name:     "loan_deposit_spread",
		Label:    "Kredi / mevduat spread'i",
		Aliases:  []string{"kredi mevduat spread", "kredi-mevduat spread", "kredi mevduat makasi", "kredi-mevduat makasi", "loan deposit spread", "loan-deposit spread", "credit deposit spread", "net faiz spread"},
		MaxValue: 0.80,
		Status:   bankLoanDepositSpreadStatus,
		Interpret: func(value float64) string {
			return fmt.Sprintf("Kredi/mevduat spread'i %.1f%%; kredi getirisi ile fonlama maliyeti arasindaki ana karlilik farkidir.", value*100)
		},
	},
}

var bankRegulatoryRequiredMetricNames = []string{
	"capital_adequacy_ratio",
	"cet1_ratio",
	"npl_ratio",
	"provision_coverage_ratio",
	"net_interest_margin",
	"liquidity_coverage_ratio",
	"loan_to_deposit_ratio",
}

func BankRegulatoryRequiredMetricNames() []string {
	return append([]string{}, bankRegulatoryRequiredMetricNames...)
}

func BankRegulatoryMetricsComplete(out SectorFinancialAnalysis) bool {
	return bankRegulatoryMetricsComplete(out)
}

func MissingBankRegulatoryMetricNames(out SectorFinancialAnalysis) []string {
	return missingBankRegulatoryMetricNames(out)
}

func BankRegulatoryMetricCompletenessScore(out SectorFinancialAnalysis) float64 {
	required := bankRegulatoryRequiredMetricNames
	if len(required) == 0 {
		return 0
	}
	present := map[string]SectorFinancialMetric{}
	for _, metric := range out.Metrics {
		present[metric.Name] = metric
	}
	complete := 0
	for _, name := range required {
		if metric, ok := present[name]; ok && bankRegulatoryMetricEvidenceComplete(metric) {
			complete++
		}
	}
	return 100 * float64(complete) / float64(len(required))
}

func loadKAPBankRegulatorySources(equitiesDir, symbol string, processedRoots ...string) ([]kapingest.ExtractedFinancialFact, []kapingest.ExtractedFinancialTable, []string) {
	warnings := []string{}
	facts := []kapingest.ExtractedFinancialFact{}
	tables := []kapingest.ExtractedFinancialTable{}
	canonicalSymbol := bankRegulatoryCanonicalSymbol(symbol)
	factsPath := firstExistingPath(kapProcessedPathCandidates(equitiesDir, symbol, kapingest.FinancialFactsFile, processedRoots...))
	if factsPath != "" {
		rows, err := readProfessionalJSONLFile[kapingest.ExtractedFinancialFact](factsPath, func(fact kapingest.ExtractedFinancialFact) bool {
			return strings.EqualFold(strings.TrimSpace(fact.Ticker), canonicalSymbol) || strings.Contains(strings.ToUpper(fact.SourceFile), canonicalSymbol)
		})
		if err != nil {
			warnings = append(warnings, "kap_bank_regulatory_financial_facts_read_failed")
		} else {
			facts = rows
		}
	}
	tablesPath := firstExistingPath(kapProcessedPathCandidates(equitiesDir, symbol, kapingest.FinancialTablesFile, processedRoots...))
	if tablesPath != "" {
		rows, err := readProfessionalJSONLFile[kapingest.ExtractedFinancialTable](tablesPath, func(table kapingest.ExtractedFinancialTable) bool {
			return strings.EqualFold(strings.TrimSpace(table.Ticker), canonicalSymbol) || strings.Contains(strings.ToUpper(table.SourceFile), canonicalSymbol)
		})
		if err != nil {
			warnings = append(warnings, "kap_bank_regulatory_financial_tables_read_failed")
		} else {
			tables = rows
		}
	}
	if len(facts) == 0 && len(tables) == 0 {
		warnings = append(warnings, "kap_bank_regulatory_sources_missing")
	}
	return facts, tables, warnings
}

func kapProcessedPathCandidates(equitiesDir, symbol, fileName string, processedRoots ...string) []string {
	symbol = bankRegulatoryCanonicalSymbol(symbol)
	lower := strings.ToLower(symbol)
	roots := []string{}
	for _, root := range processedRoots {
		root = strings.TrimSpace(root)
		if root != "" {
			roots = append(roots, root)
		}
	}
	if trimmed := strings.TrimSpace(equitiesDir); trimmed != "" {
		dataRoot := filepath.Dir(trimmed)
		roots = append(roots,
			filepath.Join(dataRoot, "processed"),
			filepath.Join(dataRoot, "processed", lower),
		)
	}
	roots = append(roots,
		filepath.Join("data", "processed"),
		filepath.Join("data", "processed", lower),
	)
	candidates := []string{}
	for _, root := range roots {
		candidates = append(candidates,
			filepath.Join(root, "by_ticker", symbol, fileName),
			filepath.Join(root, fileName),
		)
	}
	return uniqueStrings(candidates)
}

func bankRegulatoryCanonicalSymbol(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if strings.Contains(symbol, ".") {
		symbol = strings.Split(symbol, ".")[0]
	}
	return symbol
}

func firstExistingPath(paths []string) string {
	for _, path := range paths {
		if fileExists(path) {
			return path
		}
	}
	return ""
}

func readProfessionalJSONLFile[T any](path string, keep func(T) bool) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rows := []T{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 128*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row T
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return rows, err
		}
		if keep == nil || keep(row) {
			rows = append(rows, row)
		}
	}
	if err := scanner.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}

func enrichBankRegulatoryMetricsFromKAP(out SectorFinancialAnalysis, facts []kapingest.ExtractedFinancialFact, tables []kapingest.ExtractedFinancialTable) SectorFinancialAnalysis {
	if out.Profile != "bank" {
		return out
	}
	candidates := map[string]bankRegulatoryCandidate{}
	for _, fact := range facts {
		matchText := strings.TrimSpace(fact.LineItemOriginal + " " + fact.LineItemNormalized)
		for _, spec := range bankRegulatoryMetricSpecs {
			if !bankRegulatorySpecMatches(spec, matchText) {
				continue
			}
			value, ok := bankRatioFromFact(spec, fact)
			if !ok || !bankRegulatoryValuePlausible(spec, value) {
				continue
			}
			candidate := bankRegulatoryCandidate{
				Metric:      bankRegulatoryMetricFromSource(spec, value, bankFinancialFactSourceFields(fact)),
				Complete:    bankFinancialFactComplete(fact),
				DateScore:   bankFinancialFactDateScore(fact.Period, fact.DocumentDate, fact.SourceFile),
				SourceScore: bankFinancialFactSourceScore(fact.Confidence, fact.ReviewRequired, fact.Certification.Status),
			}
			if bankRegulatoryCandidateBetter(candidate, candidates[spec.Name]) {
				candidates[spec.Name] = candidate
			}
		}
	}
	for _, table := range tables {
		for _, row := range table.Rows {
			rowText := ""
			if len(row.Cells) > 0 {
				rowText = row.Cells[0]
			}
			for _, spec := range bankRegulatoryMetricSpecs {
				if !bankRegulatorySpecMatches(spec, rowText) {
					continue
				}
				value, ok := bankRatioFromTableRow(row.Cells)
				if !ok || !bankRegulatoryValuePlausible(spec, value) {
					continue
				}
				candidate := bankRegulatoryCandidate{
					Metric:      bankRegulatoryMetricFromSource(spec, value, bankFinancialTableSourceFields(table, row)),
					Complete:    bankFinancialTableComplete(table),
					DateScore:   bankFinancialFactDateScore(table.Period, table.DocumentDate, table.SourceFile),
					SourceScore: bankFinancialFactSourceScore(table.Confidence, table.ReviewRequired, table.Certification.Status),
				}
				if bankRegulatoryCandidateBetter(candidate, candidates[spec.Name]) {
					candidates[spec.Name] = candidate
				}
			}
		}
	}
	if len(candidates) == 0 {
		out.Warnings = uniqueStrings(append(out.Warnings, "bank_regulatory_metrics_not_found_in_kap_structured_sources"))
		return out
	}
	seenMetrics := map[string]bool{}
	for _, metric := range out.Metrics {
		seenMetrics[metric.Name] = true
	}
	for _, spec := range bankRegulatoryMetricSpecs {
		candidate, ok := candidates[spec.Name]
		if !ok || seenMetrics[spec.Name] {
			continue
		}
		out.Metrics = append(out.Metrics, candidate.Metric)
		switch candidate.Metric.Status {
		case "strong":
			out.Strengths = append(out.Strengths, candidate.Metric.Interpretation)
		case "weak", "critical":
			out.Risks = append(out.Risks, candidate.Metric.Interpretation)
		}
	}
	if bankRegulatoryMetricsComplete(out) {
		out.Warnings = removeStringValue(out.Warnings, "bank_reports_need_npl_capital_adequacy_and_regulatory_ratios_for_full_view")
		out.Strengths = append(out.Strengths, "Banka ana duzenleyici rasyolari structured KAP verisiyle tamamlandi.")
	} else {
		out.Warnings = append(out.Warnings, "bank_regulatory_metrics_missing_"+strings.Join(missingBankRegulatoryMetricNames(out), "_"))
	}
	out.Warnings = uniqueStrings(out.Warnings)
	out.Strengths = uniqueStrings(out.Strengths)
	out.Risks = uniqueStrings(out.Risks)
	out.Score = sectorFinancialScore(out.Metrics)
	out.Summary = sectorFinancialSummary(out)
	return out
}

func updateBankRegulatoryValuationFlag(valuation *ValuationAnalysis, sectorFinancials SectorFinancialAnalysis) {
	if valuation == nil || sectorFinancials.Profile != "bank" {
		return
	}
	if bankRegulatoryMetricsComplete(sectorFinancials) {
		valuation.Flags = removeStringValue(valuation.Flags, bankRegulatoryValuationFlag)
		valuation.Flags = uniqueStrings(append(valuation.Flags, "bank_regulatory_metrics_structured"))
		return
	}
	if !containsString(valuation.Flags, bankRegulatoryValuationFlag) {
		valuation.Flags = append(valuation.Flags, bankRegulatoryValuationFlag)
	}
	valuation.Flags = uniqueStrings(valuation.Flags)
}

func bankRegulatoryMetricFromSource(spec bankRegulatoryMetricSpec, value float64, sourceFields []string) SectorFinancialMetric {
	return SectorFinancialMetric{
		Name:           spec.Name,
		Label:          spec.Label,
		Value:          value,
		Unit:           "%",
		Status:         spec.Status(value),
		Interpretation: spec.Interpret(value),
		SourceFields:   sourceFields,
	}
}

func bankRegulatoryValuePlausible(spec bankRegulatoryMetricSpec, value float64) bool {
	if value <= 0 {
		return false
	}
	if spec.MaxValue > 0 && value > spec.MaxValue {
		return false
	}
	return true
}

func bankRegulatoryCandidateBetter(candidate, current bankRegulatoryCandidate) bool {
	if current.Metric.Name == "" {
		return true
	}
	if candidate.Complete != current.Complete {
		return candidate.Complete
	}
	if candidate.DateScore != current.DateScore {
		return candidate.DateScore > current.DateScore
	}
	if candidate.SourceScore != current.SourceScore {
		return candidate.SourceScore > current.SourceScore
	}
	return math.Abs(candidate.Metric.Value) > math.Abs(current.Metric.Value)
}

func bankRegulatorySpecMatches(spec bankRegulatoryMetricSpec, text string) bool {
	normalized := util.SlugTR(text)
	if normalized == "" {
		return false
	}
	for _, reject := range spec.RejectAliases {
		if strings.Contains(normalized, util.SlugTR(reject)) {
			return false
		}
	}
	if len(spec.RequireAny) > 0 {
		required := false
		for _, term := range spec.RequireAny {
			if strings.Contains(normalized, util.SlugTR(term)) {
				required = true
				break
			}
		}
		if !required {
			return false
		}
	}
	for _, alias := range spec.Aliases {
		if strings.Contains(normalized, util.SlugTR(alias)) {
			return true
		}
	}
	return false
}

func bankFinancialFactText(fact kapingest.ExtractedFinancialFact) string {
	return strings.Join([]string{
		fact.LineItemOriginal,
		fact.LineItemNormalized,
		fact.StatementType,
		fact.Source.Snippet,
		strings.Join(fact.Source.Cells, " "),
	}, " ")
}

func bankRatioFromFact(spec bankRegulatoryMetricSpec, fact kapingest.ExtractedFinancialFact) (float64, bool) {
	text := bankFinancialFactText(fact)
	if value, ok := bankRatioFromTextNearAliases(text, spec.Aliases); ok {
		return value, true
	}
	if !strings.Contains(util.SlugTR(fact.Unit), "percent") && !strings.Contains(fact.Unit, "%") {
		return 0, false
	}
	return normalizeBankRatioValue(fact.Value)
}

func bankRatioFromTableRow(cells []string) (float64, bool) {
	if len(cells) < 2 {
		return 0, false
	}
	percentScale := strings.Contains(strings.Join(cells, " "), "%")
	for _, cell := range cells[1:] {
		if value, ok := firstBankRatioNumber(cell, percentScale); ok {
			return value, true
		}
	}
	return 0, false
}

func bankRatioFromTextNearAliases(text string, aliases []string) (float64, bool) {
	matches := percentValueRegexp.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return 0, false
	}
	aliasIdx := bankAliasTextIndex(text, aliases)
	chosen := matches[0]
	if aliasIdx >= 0 {
		for _, match := range matches {
			if match[0] >= aliasIdx {
				chosen = match
				break
			}
		}
	}
	valueText := ""
	if chosen[2] >= 0 && chosen[3] >= 0 {
		valueText = text[chosen[2]:chosen[3]]
	} else if chosen[4] >= 0 && chosen[5] >= 0 {
		valueText = text[chosen[4]:chosen[5]]
	}
	return normalizeBankRatioText(valueText, true)
}

var (
	percentValueRegexp = regexp.MustCompile(`%\s*([0-9]+(?:[.,][0-9]+)?)|([0-9]+(?:[.,][0-9]+)?)\s*%`)
	numberValueRegexp  = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)?`)
	yearRegexp         = regexp.MustCompile(`(?:^|[^\d])(20\d{2}|19\d{2})(?:[^\d]|$)`)
)

func firstBankRatioNumber(text string, percentScale bool) (float64, bool) {
	if value, ok := bankRatioFromTextNearAliases(text, nil); ok {
		return value, true
	}
	match := numberValueRegexp.FindString(text)
	if match == "" {
		return 0, false
	}
	return normalizeBankRatioText(match, percentScale)
}

func normalizeBankRatioText(valueText string, percentScale bool) (float64, bool) {
	valueText = strings.TrimSpace(strings.ReplaceAll(valueText, ",", "."))
	if valueText == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil {
		return 0, false
	}
	if percentScale {
		return validateBankRatioValue(value / 100)
	}
	return normalizeBankRatioValue(value)
}

func normalizeBankRatioValue(value float64) (float64, bool) {
	if value > 1.5 {
		value = value / 100
	}
	return validateBankRatioValue(value)
}

func validateBankRatioValue(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0, false
	}
	if value <= 0 || value > 5 {
		return 0, false
	}
	return value, true
}

func bankAliasTextIndex(text string, aliases []string) int {
	if len(aliases) == 0 {
		return -1
	}
	lower := strings.ToLower(text)
	best := -1
	for _, alias := range aliases {
		idx := strings.Index(lower, strings.ToLower(alias))
		if idx >= 0 && (best == -1 || idx < best) {
			best = idx
		}
	}
	return best
}

func bankFinancialFactSourceFields(fact kapingest.ExtractedFinancialFact) []string {
	out := []string{
		"kap_financial_fact:" + strings.TrimSpace(fact.SourceFile),
		fmt.Sprintf("kap_page:%d", fact.Source.Page),
		fmt.Sprintf("kap_confidence:%.2f", fact.Confidence),
	}
	if fact.Period != nil {
		out = append(out, "kap_period:"+strings.TrimSpace(*fact.Period))
	}
	if fact.LineItemOriginal != "" {
		out = append(out, "kap_line_item:"+strings.TrimSpace(fact.LineItemOriginal))
	}
	return append(out, bankEvidenceQualityField(fact.ReviewRequired, fact.Certification.Status, fact.Certification.AnalysisUsable))
}

func bankFinancialTableSourceFields(table kapingest.ExtractedFinancialTable, row kapingest.FinancialTableRow) []string {
	out := []string{
		"kap_financial_table:" + strings.TrimSpace(table.SourceFile),
		fmt.Sprintf("kap_page:%d", table.Source.Page),
		fmt.Sprintf("kap_row:%d", row.RowIndex),
		fmt.Sprintf("kap_confidence:%.2f", table.Confidence),
	}
	if table.Period != nil {
		out = append(out, "kap_period:"+strings.TrimSpace(*table.Period))
	}
	if len(row.Cells) > 0 {
		out = append(out, "kap_line_item:"+strings.TrimSpace(row.Cells[0]))
	}
	return append(out, bankEvidenceQualityField(table.ReviewRequired, table.Certification.Status, table.Certification.AnalysisUsable))
}

func bankEvidenceQualityField(reviewRequired bool, status string, analysisUsable bool) string {
	switch {
	case analysisUsable && strings.EqualFold(status, kapingest.EvidenceStatusCertified):
		return "kap_certified"
	case strings.EqualFold(status, kapingest.EvidenceStatusRejected):
		return "kap_rejected"
	case reviewRequired:
		return "kap_review_required"
	default:
		return "kap_uncertified"
	}
}

func bankFinancialFactComplete(fact kapingest.ExtractedFinancialFact) bool {
	return !fact.ReviewRequired &&
		fact.Confidence >= 0.86 &&
		fact.Period != nil &&
		fact.DocumentDate != nil &&
		fact.Certification.AnalysisUsable &&
		strings.EqualFold(fact.Certification.Status, kapingest.EvidenceStatusCertified)
}

func bankFinancialTableComplete(table kapingest.ExtractedFinancialTable) bool {
	return !table.ReviewRequired &&
		table.Confidence >= 0.86 &&
		table.Period != nil &&
		table.DocumentDate != nil &&
		table.Certification.AnalysisUsable &&
		strings.EqualFold(table.Certification.Status, kapingest.EvidenceStatusCertified)
}

func bankFinancialFactSourceScore(confidence float64, reviewRequired bool, status string) float64 {
	score := confidence
	if strings.EqualFold(status, kapingest.EvidenceStatusCertified) {
		score += 1
	}
	if strings.EqualFold(status, kapingest.EvidenceStatusRejected) {
		score -= 1
	}
	if reviewRequired {
		score -= 0.5
	}
	return score
}

func bankFinancialFactDateScore(periodValue, documentDate *string, sourceFile string) int {
	for _, ptr := range []*string{periodValue, documentDate} {
		if ptr == nil {
			continue
		}
		score := dateScoreFromText(*ptr)
		if score > 0 {
			return score
		}
	}
	return dateScoreFromText(sourceFile)
}

func dateScoreFromText(text string) int {
	matches := yearRegexp.FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0
	}
	year, _ := strconv.Atoi(matches[1])
	month := 12
	day := 31
	if len(text) >= 10 && text[4] == '-' && text[7] == '-' {
		if parsedMonth, err := strconv.Atoi(text[5:7]); err == nil && parsedMonth >= 1 && parsedMonth <= 12 {
			month = parsedMonth
		}
		if parsedDay, err := strconv.Atoi(text[8:10]); err == nil && parsedDay >= 1 && parsedDay <= 31 {
			day = parsedDay
		}
	}
	return year*10000 + month*100 + day
}

func bankRegulatoryMetricsComplete(out SectorFinancialAnalysis) bool {
	present := map[string]SectorFinancialMetric{}
	for _, metric := range out.Metrics {
		present[metric.Name] = metric
	}
	for _, name := range bankRegulatoryRequiredMetricNames {
		metric, ok := present[name]
		if !ok || !bankRegulatoryMetricEvidenceComplete(metric) {
			return false
		}
	}
	return true
}

func bankRegulatoryMetricEvidenceComplete(metric SectorFinancialMetric) bool {
	if metric.Value <= 0 {
		return false
	}
	for _, source := range metric.SourceFields {
		if source == "kap_certified" {
			return true
		}
	}
	return false
}

func missingBankRegulatoryMetricNames(out SectorFinancialAnalysis) []string {
	present := map[string]SectorFinancialMetric{}
	for _, metric := range out.Metrics {
		present[metric.Name] = metric
	}
	missing := []string{}
	for _, name := range bankRegulatoryRequiredMetricNames {
		metric, ok := present[name]
		if !ok || !bankRegulatoryMetricEvidenceComplete(metric) {
			missing = append(missing, name)
		}
	}
	return missing
}

func bankCapitalAdequacyStatus(value float64) string {
	switch {
	case value >= 0.16:
		return "strong"
	case value >= 0.12:
		return "ok"
	case value >= 0.08:
		return "watch"
	default:
		return "weak"
	}
}

func bankCET1Status(value float64) string {
	switch {
	case value >= 0.12:
		return "strong"
	case value >= 0.09:
		return "ok"
	case value >= 0.07:
		return "watch"
	default:
		return "weak"
	}
}

func bankNPLStatus(value float64) string {
	switch {
	case value <= 0.03:
		return "strong"
	case value <= 0.06:
		return "ok"
	case value <= 0.10:
		return "watch"
	default:
		return "weak"
	}
}

func bankProvisionCoverageStatus(value float64) string {
	switch {
	case value >= 0.75:
		return "strong"
	case value >= 0.50:
		return "ok"
	case value >= 0.30:
		return "watch"
	default:
		return "weak"
	}
}

func bankNIMStatus(value float64) string {
	return marginStatus(value, 0.04, 0.02)
}

func bankLCRStatus(value float64) string {
	switch {
	case value >= 1.50:
		return "strong"
	case value >= 1.00:
		return "ok"
	case value >= 0.80:
		return "watch"
	default:
		return "weak"
	}
}

func bankDepositCostStatus(value float64) string {
	switch {
	case value <= 0.20:
		return "strong"
	case value <= 0.35:
		return "ok"
	case value <= 0.50:
		return "watch"
	default:
		return "weak"
	}
}

func bankLoanDepositSpreadStatus(value float64) string {
	switch {
	case value >= 0.04:
		return "strong"
	case value >= 0.02:
		return "ok"
	case value > 0:
		return "watch"
	default:
		return "weak"
	}
}
