package kapfinance

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/util"
)

const (
	DefaultSource         = "kap_extracted_financial_facts"
	DefaultFinancialGroup = "kap_pdf_extracted"
	DefaultCurrency       = "TRY"
)

type Fact struct {
	ID                          string
	Ticker                      string
	Period                      string
	FiscalYear                  int
	FiscalQuarter               int
	DocumentDate                string
	LineItemOriginal            string
	LineItemNormalized          string
	StatementType               string
	Value                       float64
	Currency                    string
	Unit                        string
	SourceFile                  string
	SourceDocumentID            string
	SourcePage                  int
	SourceTableID               string
	SourceText                  string
	Confidence                  float64
	ReviewRequired              bool
	ValidationStatus            string
	CertificationStatus         string
	CertificationAnalysisUsable bool
	CertificationScore          int
	ConsolidationScope          string
	AuditStatus                 string
	CreatedAt                   time.Time
}

type BuildOptions struct {
	Ticker         string
	Source         string
	FinancialGroup string
	GeneratedAt    time.Time
}

type BuildSummary struct {
	Ticker        string   `json:"ticker"`
	FactsRead     int      `json:"facts_read"`
	FactsAccepted int      `json:"facts_accepted"`
	FactsRejected int      `json:"facts_rejected"`
	Fields        int      `json:"fields"`
	Periods       int      `json:"periods"`
	Warnings      []string `json:"warnings,omitempty"`
}

type LineDefinition struct {
	Code          string
	DescTR        string
	DescEN        string
	Normalized    string
	StatementType string
	Aliases       []string
}

type canonicalLine = LineDefinition

type selectedFact struct {
	fact  Fact
	line  canonicalLine
	year  int
	q     int
	value float64
	score float64
}

var (
	yearQuarterRE    = regexp.MustCompile(`(?i)\b((?:19|20)\d{2})\s*[-_/ ]?\s*Q([1-4])\b`)
	yearMonthRE      = regexp.MustCompile(`\b((?:19|20)\d{2})[./_-](0?[369]|1[02])\b`)
	monthYearRE      = regexp.MustCompile(`\b(0?[369]|1[02])[./_-]((?:19|20)\d{2})\b`)
	isoDateRE        = regexp.MustCompile(`\b((?:19|20)\d{2})-(\d{1,2})-(\d{1,2})\b`)
	turkishDateRE    = regexp.MustCompile(`\b(\d{1,2})[./](\d{1,2})[./]((?:19|20)\d{2})\b`)
	rawKAPPeriodRE   = regexp.MustCompile(`(?i)\b(3|6|9|12)\s*[A-ZÇĞİÖŞÜ]*\b`)
	plainYearRE      = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	nonCanonicalUnit = map[string]struct{}{
		"":     {},
		"try":  {},
		"tl":   {},
		"unit": {},
	}
)

func BuildBilanco(opts BuildOptions, facts []Fact) (*domain.BilancoInfo, BuildSummary) {
	ticker := strings.ToUpper(strings.TrimSpace(opts.Ticker))
	if ticker == "" && len(facts) > 0 {
		ticker = strings.ToUpper(strings.TrimSpace(facts[0].Ticker))
	}
	if opts.GeneratedAt.IsZero() {
		opts.GeneratedAt = time.Now().UTC()
	}
	source := firstNonEmpty(opts.Source, DefaultSource)
	group := firstNonEmpty(opts.FinancialGroup, DefaultFinancialGroup)
	summary := BuildSummary{Ticker: ticker, FactsRead: len(facts)}
	if ticker == "" {
		summary.Warnings = append(summary.Warnings, "kap_canonical_ticker_missing")
		summary.FactsRejected = len(facts)
		return nil, summary
	}

	selected := map[string]selectedFact{}
	reviewFactsAccepted := false
	for _, fact := range facts {
		line, ok := CanonicalLineForFact(fact)
		if !ok {
			summary.FactsRejected++
			continue
		}
		year, quarter, ok := ResolvePeriod(fact)
		if !ok {
			summary.FactsRejected++
			continue
		}
		value, ok := CanonicalValue(fact)
		if !ok {
			summary.FactsRejected++
			continue
		}
		if !usableFact(fact) {
			summary.FactsRejected++
			continue
		}
		score := factRank(fact)
		key := fmt.Sprintf("%s|%04d-Q%d", line.Code, year, quarter)
		candidate := selectedFact{fact: fact, line: line, year: year, q: quarter, value: value, score: score}
		current, exists := selected[key]
		if !exists || betterFact(candidate, current) {
			selected[key] = candidate
		}
		if fact.ReviewRequired || strings.EqualFold(fact.CertificationStatus, "review_required") || strings.EqualFold(fact.ValidationStatus, "unknown") {
			reviewFactsAccepted = true
		}
	}
	summary.FactsAccepted = len(selected)
	summary.FactsRejected += len(facts) - summary.FactsRejected - summary.FactsAccepted
	if len(selected) == 0 {
		summary.Warnings = append(summary.Warnings, "kap_canonical_no_usable_financial_facts")
		return nil, summary
	}
	if reviewFactsAccepted {
		summary.Warnings = append(summary.Warnings, "kap_canonical_contains_review_required_or_unvalidated_facts")
	}

	info := &domain.BilancoInfo{
		Ticker:         ticker,
		Source:         source,
		Currency:       DefaultCurrency,
		FinancialGroup: group,
		FetchedAt:      opts.GeneratedAt.UTC(),
		Periods:        map[string]domain.FinancialPeriod{},
		Data:           map[string]domain.BilancoField{},
	}
	keys := make([]string, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		item := selected[key]
		field := info.Data[item.line.Code]
		field.DescTR = item.line.DescTR
		field.DescEN = item.line.DescEN
		if field.Years == nil {
			field.Years = map[string][]*float64{}
		}
		yearText := strconv.Itoa(item.year)
		values := field.Years[yearText]
		for len(values) < 4 {
			values = append(values, nil)
		}
		idx := indexFromQuarter(item.q)
		if idx >= 0 {
			value := item.value
			values[idx] = &value
			field.Years[yearText] = values
			info.Data[item.line.Code] = field
		}
		periodKey := domain.FinancialPeriodKey(item.year, item.q)
		period := domain.NewFinancialPeriod(item.year, item.q, source, group, DefaultCurrency, opts.GeneratedAt.UTC())
		period.PeriodEnd = domain.FiscalPeriodEnd(item.year, item.q)
		reportDate := period.PeriodEnd
		period.ReportDate = &reportDate
		period.SourceDocumentID = strings.TrimSpace(item.fact.SourceDocumentID)
		if docDate, ok := parseDate(item.fact.DocumentDate); ok {
			at := docDate.UTC()
			period.AvailableAt = &at
			period.AvailabilitySource = "provided_available_at"
			period.BacktestSafe = true
		} else if !item.fact.CreatedAt.IsZero() {
			at := item.fact.CreatedAt.UTC()
			period.AvailableAt = &at
			period.AvailabilitySource = "provided_available_at"
			period.BacktestSafe = true
		}
		if period.PublishDate == nil {
			period.Warnings = append(period.Warnings, "publish_date_missing")
		}
		if item.fact.SourceFile != "" {
			period.Warnings = append(period.Warnings, "source_file="+item.fact.SourceFile)
		}
		info.Periods[periodKey] = period
	}
	domain.NormalizeBilancoInfo(info, ticker)
	domain.AppendLineage(info, domain.DataLineageEvent{
		Stage:     "kap_financial_canonicalization",
		Source:    source,
		Transform: "kap_financial_facts_to_bilanco_json",
		Version:   "kap-finance-canonical-v1",
		CreatedAt: opts.GeneratedAt.UTC(),
		Notes:     []string{"KAP PDF/XML/HTML extraction facts converted into fallback financial statement schema; does not overwrite official bilanco.json."},
	})
	info.Quality = domain.ValidateFinancialDataQuality(info, time.Time{})
	summary.Fields = len(info.Data)
	summary.Periods = len(info.Periods)
	return info, summary
}

func CanonicalLineForFact(fact Fact) (canonicalLine, bool) {
	candidates := []string{fact.LineItemNormalized, fact.LineItemOriginal, fact.SourceText}
	for _, value := range candidates {
		if line, ok := canonicalLineForText(value); ok {
			if line.Code != "" {
				return line, true
			}
		}
	}
	return canonicalLine{}, false
}

func CanonicalValue(fact Fact) (float64, bool) {
	if fact.Value <= 0 || fact.Value >= 1e15 || math.IsNaN(fact.Value) || math.IsInf(fact.Value, 0) {
		return 0, false
	}
	currency := strings.ToUpper(strings.TrimSpace(fact.Currency))
	if currency != "" && currency != "TRY" && currency != "TL" {
		return 0, false
	}
	unit := strings.ToLower(strings.TrimSpace(fact.Unit))
	switch {
	case strings.Contains(unit, "million") || strings.Contains(unit, "milyon"):
		return fact.Value * 1_000_000, true
	case strings.Contains(unit, "thousand") || strings.Contains(unit, "bin"):
		return fact.Value * 1_000, true
	default:
		if _, ok := nonCanonicalUnit[unit]; ok {
			return fact.Value, true
		}
		return fact.Value, true
	}
}

func ResolvePeriod(fact Fact) (int, int, bool) {
	if fact.FiscalYear > 0 && fact.FiscalQuarter >= 1 && fact.FiscalQuarter <= 4 {
		return fact.FiscalYear, fact.FiscalQuarter, true
	}
	for _, value := range []string{fact.Period, fact.DocumentDate, fact.SourceText} {
		if year, quarter, ok := parsePeriod(value, fact.FiscalYear); ok {
			return year, quarter, true
		}
	}
	return 0, 0, false
}

func canonicalLineForText(value string) (canonicalLine, bool) {
	return canonicalLineForTextCatalog(value)
}

func usableFact(fact Fact) bool {
	cert := strings.ToLower(strings.TrimSpace(fact.CertificationStatus))
	if cert == "rejected" {
		return false
	}
	if cert == "certified" {
		return fact.CertificationAnalysisUsable || fact.CertificationScore >= 80
	}
	if cert == "ai_resolved" {
		return fact.CertificationAnalysisUsable || fact.CertificationScore >= 75
	}
	validation := strings.ToLower(strings.TrimSpace(fact.ValidationStatus))
	if validation == "invalid" {
		return false
	}
	return fact.Confidence >= 0.45
}

func factRank(fact Fact) float64 {
	score := fact.Confidence
	cert := strings.ToLower(strings.TrimSpace(fact.CertificationStatus))
	switch cert {
	case "certified":
		score += 1.0
	case "ai_resolved":
		score += 0.75
	case "review_required":
		score += 0.25
	}
	switch strings.ToLower(strings.TrimSpace(fact.ValidationStatus)) {
	case "valid":
		score += 0.5
	case "warning":
		score += 0.2
	case "unknown":
		score += 0.05
	}
	if fact.SourceTableID != "" {
		score += 0.25
	}
	if fact.SourcePage > 0 {
		score += 0.05
	}
	if !fact.ReviewRequired {
		score += 0.2
	}
	if strings.EqualFold(fact.ConsolidationScope, "consolidated") || fact.ConsolidationScope == "" {
		score += 0.1
	}
	if strings.EqualFold(fact.AuditStatus, "audited") {
		score += 0.1
	}
	return score
}

func betterFact(candidate, current selectedFact) bool {
	if candidate.score != current.score {
		return candidate.score > current.score
	}
	if !candidate.fact.CreatedAt.IsZero() && !current.fact.CreatedAt.IsZero() && !candidate.fact.CreatedAt.Equal(current.fact.CreatedAt) {
		return candidate.fact.CreatedAt.After(current.fact.CreatedAt)
	}
	if candidate.fact.ID != "" && current.fact.ID != "" {
		return candidate.fact.ID < current.fact.ID
	}
	return candidate.value > current.value
}

func parsePeriod(value string, fallbackYear int) (int, int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, 0, false
	}
	if match := yearQuarterRE.FindStringSubmatch(value); len(match) >= 3 {
		year, _ := strconv.Atoi(match[1])
		quarter, _ := strconv.Atoi(match[2])
		if validPeriod(year, quarter) {
			return year, quarter, true
		}
	}
	if match := yearMonthRE.FindStringSubmatch(value); len(match) >= 3 {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		quarter := quarterFromMonth(month)
		if validPeriod(year, quarter) {
			return year, quarter, true
		}
	}
	if match := monthYearRE.FindStringSubmatch(value); len(match) >= 3 {
		month, _ := strconv.Atoi(match[1])
		year, _ := strconv.Atoi(match[2])
		quarter := quarterFromMonth(month)
		if validPeriod(year, quarter) {
			return year, quarter, true
		}
	}
	for _, re := range []*regexp.Regexp{isoDateRE, turkishDateRE} {
		if match := re.FindStringSubmatch(value); len(match) >= 4 {
			year, month := 0, 0
			if re == isoDateRE {
				year, _ = strconv.Atoi(match[1])
				month, _ = strconv.Atoi(match[2])
			} else {
				month, _ = strconv.Atoi(match[2])
				year, _ = strconv.Atoi(match[3])
			}
			quarter := quarterFromMonth(month)
			if validPeriod(year, quarter) {
				return year, quarter, true
			}
		}
	}
	if fallbackYear > 0 {
		if match := rawKAPPeriodRE.FindStringSubmatch(value); len(match) >= 2 {
			month, _ := strconv.Atoi(match[1])
			quarter := quarterFromMonth(month)
			if validPeriod(fallbackYear, quarter) {
				return fallbackYear, quarter, true
			}
		}
	}
	if fallbackYear == 0 {
		if match := plainYearRE.FindString(value); match != "" {
			fallbackYear, _ = strconv.Atoi(match)
		}
	}
	if fallbackYear > 0 && strings.Contains(strings.ToUpper(value), "Q") {
		for q := 1; q <= 4; q++ {
			if strings.Contains(strings.ToUpper(value), fmt.Sprintf("Q%d", q)) {
				return fallbackYear, q, true
			}
		}
	}
	return 0, 0, false
}

func parseDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "02.01.2006", "02/01/2006", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	if year, month, ok := parseYearMonth(value); ok {
		return domain.FiscalPeriodEnd(year, quarterFromMonth(month)), true
	}
	return time.Time{}, false
}

func parseYearMonth(value string) (int, int, bool) {
	if match := yearMonthRE.FindStringSubmatch(value); len(match) >= 3 {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		return year, month, true
	}
	if match := monthYearRE.FindStringSubmatch(value); len(match) >= 3 {
		month, _ := strconv.Atoi(match[1])
		year, _ := strconv.Atoi(match[2])
		return year, month, true
	}
	return 0, 0, false
}

func quarterFromMonth(month int) int {
	switch month {
	case 1, 2, 3:
		return 1
	case 4, 5, 6:
		return 2
	case 7, 8, 9:
		return 3
	case 10, 11, 12:
		return 4
	default:
		return 0
	}
}

func indexFromQuarter(quarter int) int {
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

func validPeriod(year, quarter int) bool {
	return year >= 1990 && year <= time.Now().UTC().Year()+1 && quarter >= 1 && quarter <= 4
}

func slugContains(slug, needle string) bool {
	return strings.Contains(slug, util.SlugTR(needle))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
