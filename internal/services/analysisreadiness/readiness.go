package analysisreadiness

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/domain/kapextract"
	"hissebot/internal/kapfinance"
	"hissebot/internal/services/bistbulletindb"
	"hissebot/internal/services/classification"
	"hissebot/internal/storage"
	"hissebot/internal/ta/corporateactions"

	_ "github.com/mattn/go-sqlite3"
)

const (
	StatusDecisionReady = "decision_ready"
	StatusLimited       = "limited"
	StatusBlocked       = "blocked"

	checkPass    = "pass"
	checkWarn    = "warn"
	checkFail    = "fail"
	severityInfo = "info"
	severityWarn = "warning"
	severityErr  = "error"
)

type Options struct {
	Symbols             []string
	All                 bool
	Limit               int
	Mode                string
	BISTDBPath          string
	EquitiesDir         string
	SectorFile          string
	MinDailyBars        int
	MaxPriceStaleness   time.Duration
	MinFinancialFacts   int
	MinCanonicalFields  int
	MinCanonicalPeriods int
	MinPeers            int
	Now                 func() time.Time
}

type Report struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	Mode          string            `json:"mode"`
	Status        string            `json:"status"`
	Score         float64           `json:"score"`
	Symbols       int               `json:"symbols"`
	DecisionReady int               `json:"decision_ready"`
	Limited       int               `json:"limited"`
	Blocked       int               `json:"blocked"`
	IssueCounts   map[string]int    `json:"issue_counts,omitempty"`
	Results       []SymbolReadiness `json:"results"`
}

type SymbolReadiness struct {
	Symbol             string                  `json:"symbol"`
	Status             string                  `json:"status"`
	Score              float64                 `json:"score"`
	Blockers           []string                `json:"blockers,omitempty"`
	Warnings           []string                `json:"warnings,omitempty"`
	Checks             []Check                 `json:"checks"`
	Price              PriceCoverage           `json:"price"`
	KAPExtraction      KAPExtractionCoverage   `json:"kap_extraction"`
	Financials         FinancialCoverage       `json:"financials"`
	Sector             SectorCoverage          `json:"sector"`
	CorporateActions   CorporateActionCoverage `json:"corporate_actions"`
	Macro              MacroCoverage           `json:"macro"`
	RecommendedActions []string                `json:"recommended_actions,omitempty"`
	Metadata           map[string]any          `json:"metadata,omitempty"`
}

type Check struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Severity    string   `json:"severity"`
	Score       float64  `json:"score"`
	Message     string   `json:"message,omitempty"`
	Details     []string `json:"details,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
}

type PriceCoverage struct {
	Status            string `json:"status"`
	DBPath            string `json:"db_path,omitempty"`
	Rows              int    `json:"rows"`
	AnalysisReadyRows int    `json:"analysis_ready_rows"`
	AdjustedRows      int    `json:"adjusted_rows"`
	FirstTradingDate  string `json:"first_trading_date,omitempty"`
	LatestTradingDate string `json:"latest_trading_date,omitempty"`
	StaleDays         int    `json:"stale_days,omitempty"`
}

type KAPExtractionCoverage struct {
	Status             string    `json:"status"`
	Path               string    `json:"path,omitempty"`
	GeneratedAt        time.Time `json:"generated_at,omitempty"`
	Documents          int       `json:"documents"`
	TextBlocks         int       `json:"text_blocks"`
	FinancialFacts     int       `json:"financial_facts"`
	CorporateEvents    int       `json:"corporate_events"`
	ReviewItems        int       `json:"review_items"`
	ReviewRequiredDocs int       `json:"review_required_documents"`
	Warnings           []string  `json:"warnings,omitempty"`
}

type FinancialCoverage struct {
	Status        string   `json:"status"`
	FactsRead     int      `json:"facts_read"`
	FactsAccepted int      `json:"facts_accepted"`
	FactsRejected int      `json:"facts_rejected"`
	Fields        int      `json:"fields"`
	Periods       int      `json:"periods"`
	Warnings      []string `json:"warnings,omitempty"`
}

type SectorCoverage struct {
	Status     string   `json:"status"`
	Sector     string   `json:"sector,omitempty"`
	Industry   string   `json:"industry,omitempty"`
	PeerGroup  string   `json:"peer_group,omitempty"`
	PeerCount  int      `json:"peer_count"`
	Source     string   `json:"source,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	Evidence   []string `json:"evidence,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type CorporateActionCoverage struct {
	Status                      string   `json:"status"`
	Actions                     int      `json:"actions"`
	VerifiedActions             int      `json:"verified_actions"`
	CandidateActions            int      `json:"candidate_actions"`
	ReviewRequiredActions       int      `json:"review_required_actions"`
	AdjustmentReadyActions      int      `json:"adjustment_ready_actions"`
	MissingEffectiveDateActions int      `json:"missing_effective_date_actions"`
	MissingAdjustmentActions    int      `json:"missing_adjustment_actions"`
	SourceFiles                 []string `json:"source_files,omitempty"`
	Warnings                    []string `json:"warnings,omitempty"`
}

type MacroCoverage struct {
	Status  string   `json:"status"`
	Files   []string `json:"files,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

type priceSnapshot struct {
	rows              int
	analysisReadyRows int
	adjustedRows      int
	firstDate         string
	latestDate        string
}

func Run(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts Options) (Report, error) {
	opts = normalizeOptions(cfg, store, opts)
	now := opts.now()
	symbols, err := resolveSymbols(ctx, store, opts)
	if err != nil {
		return Report{}, err
	}
	sectors, err := loadSectorFile(opts.SectorFile)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		GeneratedAt: now,
		Mode:        opts.Mode,
		Status:      StatusDecisionReady,
		IssueCounts: map[string]int{},
		Results:     make([]SymbolReadiness, 0, len(symbols)),
	}
	for _, symbol := range symbols {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		readiness := evaluateSymbol(ctx, cfg, opts, sectors, symbol)
		report.Results = append(report.Results, readiness)
		report.Symbols++
		switch readiness.Status {
		case StatusBlocked:
			report.Blocked++
			report.Status = StatusBlocked
		case StatusLimited:
			report.Limited++
			if report.Status != StatusBlocked {
				report.Status = StatusLimited
			}
		default:
			report.DecisionReady++
		}
		for _, check := range readiness.Checks {
			if check.Status != checkPass {
				report.IssueCounts[check.Name]++
			}
		}
	}
	if len(report.Results) > 0 {
		total := 0.0
		for _, result := range report.Results {
			total += result.Score
		}
		report.Score = round1(total / float64(len(report.Results)))
	}
	if len(report.IssueCounts) == 0 {
		report.IssueCounts = nil
	}
	return report, nil
}

func normalizeOptions(cfg config.Config, store *storage.EquityStore, opts Options) Options {
	opts.Mode = strings.ToLower(strings.TrimSpace(opts.Mode))
	if opts.Mode == "" {
		opts.Mode = "research"
	}
	if opts.Mode != "production" {
		opts.Mode = "research"
	}
	if strings.TrimSpace(opts.BISTDBPath) == "" {
		opts.BISTDBPath = filepath.Join(cfg.DataDir, "bist", "bist_ohlcv.sqlite")
		if strings.TrimSpace(opts.BISTDBPath) == "" {
			opts.BISTDBPath = bistbulletindb.DefaultDBPath
		}
	}
	if strings.TrimSpace(opts.EquitiesDir) == "" {
		opts.EquitiesDir = cfg.EquitiesDir
		if opts.EquitiesDir == "" && store != nil {
			opts.EquitiesDir = store.Root()
		}
	}
	if strings.TrimSpace(opts.SectorFile) == "" {
		opts.SectorFile = cfg.SectorClassificationsFile
	}
	if opts.MinPeers <= 0 {
		opts.MinPeers = 3
	}
	if opts.MinFinancialFacts <= 0 {
		opts.MinFinancialFacts = 5
	}
	if opts.MinCanonicalPeriods <= 0 {
		opts.MinCanonicalPeriods = 1
	}
	if opts.MinCanonicalFields <= 0 {
		opts.MinCanonicalFields = 4
		if opts.Mode == "production" {
			opts.MinCanonicalFields = 8
		}
	}
	if opts.MinDailyBars <= 0 {
		opts.MinDailyBars = 120
		if opts.Mode == "production" {
			opts.MinDailyBars = 252
		}
	}
	if opts.MaxPriceStaleness <= 0 {
		opts.MaxPriceStaleness = 7 * 24 * time.Hour
		if opts.Mode == "production" {
			opts.MaxPriceStaleness = 3 * 24 * time.Hour
		}
	}
	return opts
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

func resolveSymbols(ctx context.Context, store *storage.EquityStore, opts Options) ([]string, error) {
	seen := map[string]bool{}
	symbols := []string{}
	for _, symbol := range opts.Symbols {
		symbol = storage.NormalizeTicker(symbol)
		if symbol != "" && !seen[symbol] {
			seen[symbol] = true
			symbols = append(symbols, symbol)
		}
	}
	if len(symbols) == 0 || opts.All {
		if store == nil {
			return nil, errors.New("equity store is required for all-symbol readiness")
		}
		equities, err := store.List()
		if err != nil {
			return nil, err
		}
		for _, equity := range equities {
			if err := ctx.Err(); err != nil {
				return symbols, err
			}
			if equity == nil || !domain.IsEquityOrUnknownAssetType(equity.AssetType) {
				continue
			}
			symbol := storage.NormalizeTicker(equity.Ticker)
			if symbol != "" && !seen[symbol] {
				seen[symbol] = true
				symbols = append(symbols, symbol)
			}
		}
	}
	sort.Strings(symbols)
	if opts.Limit > 0 && len(symbols) > opts.Limit {
		symbols = symbols[:opts.Limit]
	}
	if len(symbols) == 0 {
		return nil, errors.New("no symbols selected")
	}
	return symbols, nil
}

func evaluateSymbol(ctx context.Context, cfg config.Config, opts Options, sectors map[string]classification.Entry, symbol string) SymbolReadiness {
	out := SymbolReadiness{
		Symbol: symbol,
		Status: StatusDecisionReady,
		Score:  100,
	}
	out.Price, out.Checks = checkPrice(ctx, opts, symbol, out.Checks)
	extraction, extractionResult, checks := checkKAPExtraction(opts, symbol, out.Checks)
	out.KAPExtraction = extraction
	out.Checks = checks
	out.Financials, out.Checks = checkFinancials(opts, symbol, extractionResult, out.Checks)
	out.Sector, out.Checks = checkSector(opts, sectors, symbol, out.Checks)
	out.CorporateActions, out.Checks = checkCorporateActions(opts, symbol, out.Checks)
	out.Macro, out.Checks = checkMacro(cfg, out.Checks)
	finalizeSymbol(&out)
	return out
}

func checkPrice(ctx context.Context, opts Options, symbol string, checks []Check) (PriceCoverage, []Check) {
	coverage := PriceCoverage{Status: checkFail, DBPath: opts.BISTDBPath}
	snapshot, err := loadPriceSnapshot(ctx, opts.BISTDBPath, symbol)
	if err != nil {
		checks = append(checks, Check{
			Name:        "official_bist_price",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     err.Error(),
			Remediation: fmt.Sprintf("go run ./cmd/hissebot sync bist-bulletin-db && go run ./cmd/hissebot analyze -provider bistdb -symbol %s", symbol),
		})
		return coverage, checks
	}
	coverage.Rows = snapshot.rows
	coverage.AnalysisReadyRows = snapshot.analysisReadyRows
	coverage.AdjustedRows = snapshot.adjustedRows
	coverage.FirstTradingDate = snapshot.firstDate
	coverage.LatestTradingDate = snapshot.latestDate
	latest, _ := time.Parse("2006-01-02", snapshot.latestDate)
	staleDays := 0
	if !latest.IsZero() {
		staleDays = int(dateOnly(opts.now()).Sub(dateOnly(latest)).Hours() / 24)
		if staleDays < 0 {
			staleDays = 0
		}
		coverage.StaleDays = staleDays
	}
	switch {
	case snapshot.rows == 0 || snapshot.analysisReadyRows == 0:
		checks = append(checks, Check{
			Name:        "official_bist_price",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     "official BIST daily OHLCV is missing or not analysis-ready",
			Details:     []string{fmt.Sprintf("rows=%d", snapshot.rows), fmt.Sprintf("analysis_ready_rows=%d", snapshot.analysisReadyRows)},
			Remediation: fmt.Sprintf("go run ./cmd/hissebot sync bist-bulletin-db && go run ./cmd/hissebot analyze -provider bistdb -symbol %s", symbol),
		})
	case snapshot.analysisReadyRows < opts.MinDailyBars:
		coverage.Status = checkWarn
		checks = append(checks, Check{
			Name:        "official_bist_price",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       65,
			Message:     "official BIST price history is shorter than the readiness threshold",
			Details:     []string{fmt.Sprintf("analysis_ready_rows=%d", snapshot.analysisReadyRows), fmt.Sprintf("min_daily_bars=%d", opts.MinDailyBars)},
			Remediation: "BIST DB geçmiş aralığını genişletin veya kısa geçmişli hisselerde karar raporunu limited kabul edin.",
		})
	case !latest.IsZero() && time.Duration(staleDays)*24*time.Hour > opts.MaxPriceStaleness:
		coverage.Status = checkFail
		checks = append(checks, Check{
			Name:        "official_bist_price",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       20,
			Message:     "official BIST close is stale",
			Details:     []string{fmt.Sprintf("latest_trading_date=%s", snapshot.latestDate), fmt.Sprintf("stale_days=%d", staleDays)},
			Remediation: "Resmi BIST bülten sync komutunu son işlem gününe kadar çalıştırın.",
		})
	default:
		coverage.Status = checkPass
		score := 100.0
		if snapshot.adjustedRows == 0 {
			score = 90
		}
		checks = append(checks, Check{
			Name:     "official_bist_price",
			Status:   checkPass,
			Severity: severityInfo,
			Score:    score,
			Details: []string{
				fmt.Sprintf("analysis_ready_rows=%d", snapshot.analysisReadyRows),
				"latest_trading_date=" + snapshot.latestDate,
			},
		})
	}
	return coverage, checks
}

func loadPriceSnapshot(ctx context.Context, dbPath, symbol string) (priceSnapshot, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = bistbulletindb.DefaultDBPath
	}
	if _, err := os.Stat(dbPath); err != nil {
		return priceSnapshot{}, fmt.Errorf("BIST DB missing: %s", dbPath)
	}
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(dbPath)+"?mode=ro&_busy_timeout=10000")
	if err != nil {
		return priceSnapshot{}, err
	}
	defer db.Close()
	var out priceSnapshot
	var first, latest sql.NullString
	var ready, adjusted sql.NullInt64
	err = db.QueryRowContext(ctx, `
SELECT
  COUNT(*),
  COALESCE(SUM(CASE WHEN analysis_ready = 1 THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN is_adjusted = 1 THEN 1 ELSE 0 END), 0),
  MIN(trading_date),
  MAX(trading_date)
FROM daily_ohlcv
WHERE symbol = ?`, symbol).Scan(&out.rows, &ready, &adjusted, &first, &latest)
	if err != nil {
		return priceSnapshot{}, fmt.Errorf("BIST DB query failed for %s: %w", symbol, err)
	}
	out.analysisReadyRows = int(ready.Int64)
	out.adjustedRows = int(adjusted.Int64)
	out.firstDate = first.String
	out.latestDate = latest.String
	return out, nil
}

func checkKAPExtraction(opts Options, symbol string, checks []Check) (KAPExtractionCoverage, *kapextract.ExtractionResult, []Check) {
	path := filepath.Join(opts.EquitiesDir, symbol, "kap", "extraction", "extraction_result.json")
	coverage := KAPExtractionCoverage{Status: checkFail, Path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		checks = append(checks, Check{
			Name:        "kap_extraction",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     "KAP extraction result is missing",
			Details:     []string{path},
			Remediation: fmt.Sprintf("go run ./cmd/hissebot sync kap-document-archive -ticker %s && go run ./cmd/hissebot sync kap-extract -ticker %s -limit 20 -max-chars 200000", symbol, symbol),
		})
		return coverage, nil, checks
	}
	var result kapextract.ExtractionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		checks = append(checks, Check{
			Name:        "kap_extraction",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     "KAP extraction result cannot be parsed",
			Details:     []string{err.Error()},
			Remediation: fmt.Sprintf("go run ./cmd/hissebot sync kap-extract -ticker %s -limit 20 -max-chars 200000", symbol),
		})
		return coverage, nil, checks
	}
	coverage.Status = checkPass
	coverage.GeneratedAt = result.GeneratedAt
	coverage.Documents = len(result.Documents)
	coverage.TextBlocks = len(result.TextBlocks)
	coverage.FinancialFacts = len(result.FinancialFacts)
	coverage.CorporateEvents = len(result.CorporateEvents)
	coverage.ReviewItems = len(result.HumanReviewQueue)
	coverage.Warnings = append([]string{}, result.Warnings...)
	for _, doc := range result.Documents {
		if doc.ReviewRequired {
			coverage.ReviewRequiredDocs++
		}
	}
	switch {
	case len(result.Documents) == 0:
		coverage.Status = checkFail
		checks = append(checks, Check{
			Name:        "kap_extraction",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     "KAP extraction has no processed documents",
			Remediation: fmt.Sprintf("go run ./cmd/hissebot sync kap-extract -ticker %s -limit 20 -max-chars 200000", symbol),
		})
	case len(result.FinancialFacts) < opts.MinFinancialFacts:
		coverage.Status = checkWarn
		checks = append(checks, Check{
			Name:        "kap_extraction",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       60,
			Message:     "KAP extraction has limited financial facts",
			Details:     []string{fmt.Sprintf("financial_facts=%d", len(result.FinancialFacts)), fmt.Sprintf("min_financial_facts=%d", opts.MinFinancialFacts)},
			Remediation: "Daha fazla son dönem KAP dokümanı çıkarın veya OCR gerektiren belgeleri tamamlayın.",
		})
	default:
		score := 100.0
		if len(result.HumanReviewQueue) > len(result.FinancialFacts) {
			score = 80
		}
		checks = append(checks, Check{
			Name:     "kap_extraction",
			Status:   checkPass,
			Severity: severityInfo,
			Score:    score,
			Details: []string{
				fmt.Sprintf("documents=%d", len(result.Documents)),
				fmt.Sprintf("financial_facts=%d", len(result.FinancialFacts)),
				fmt.Sprintf("review_items=%d", len(result.HumanReviewQueue)),
			},
		})
	}
	return coverage, &result, checks
}

func checkFinancials(opts Options, symbol string, extraction *kapextract.ExtractionResult, checks []Check) (FinancialCoverage, []Check) {
	coverage := FinancialCoverage{Status: checkFail}
	if extraction == nil {
		checks = append(checks, Check{
			Name:        "canonical_financials",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     "canonical financials cannot be built because KAP extraction is missing",
			Remediation: fmt.Sprintf("go run ./cmd/hissebot sync kap-extract -ticker %s -limit 20 -max-chars 200000", symbol),
		})
		return coverage, checks
	}
	if extraction.CanonicalFinancials != nil {
		coverage = FinancialCoverage{
			Status:        checkPass,
			FactsRead:     extraction.CanonicalFinancials.FactsRead,
			FactsAccepted: extraction.CanonicalFinancials.FactsAccepted,
			FactsRejected: extraction.CanonicalFinancials.FactsRejected,
			Fields:        extraction.CanonicalFinancials.Fields,
			Periods:       extraction.CanonicalFinancials.Periods,
			Warnings:      append([]string{}, extraction.CanonicalFinancials.Warnings...),
		}
	} else {
		_, summary := kapfinance.BuildBilanco(kapfinance.BuildOptions{
			Ticker:      symbol,
			GeneratedAt: opts.now(),
		}, canonicalFacts(*extraction))
		coverage = FinancialCoverage{
			Status:        checkPass,
			FactsRead:     summary.FactsRead,
			FactsAccepted: summary.FactsAccepted,
			FactsRejected: summary.FactsRejected,
			Fields:        summary.Fields,
			Periods:       summary.Periods,
			Warnings:      append([]string{}, summary.Warnings...),
		}
	}
	switch {
	case coverage.FactsAccepted == 0 || coverage.Fields == 0 || coverage.Periods == 0:
		coverage.Status = checkFail
		checks = append(checks, Check{
			Name:        "canonical_financials",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     "canonical financial statement coverage is not decision-usable",
			Details:     financialDetails(coverage),
			Remediation: "KAP financial fact doğrulamasını artırın; tablo/XBRL kaynaklarını ve period mapping'i tamamlayın.",
		})
	case coverage.Fields < opts.MinCanonicalFields || coverage.Periods < opts.MinCanonicalPeriods:
		coverage.Status = checkWarn
		checks = append(checks, Check{
			Name:        "canonical_financials",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       65,
			Message:     "canonical financial statement coverage is thin",
			Details:     append(financialDetails(coverage), fmt.Sprintf("min_fields=%d", opts.MinCanonicalFields), fmt.Sprintf("min_periods=%d", opts.MinCanonicalPeriods)),
			Remediation: "Son finansal tabloları daha geniş field kapsamıyla canonical bilançoya bağlayın.",
		})
	default:
		score := 100.0
		if len(coverage.Warnings) > 0 {
			score = 85
		}
		checks = append(checks, Check{
			Name:     "canonical_financials",
			Status:   checkPass,
			Severity: severityInfo,
			Score:    score,
			Details:  financialDetails(coverage),
		})
	}
	return coverage, checks
}

func canonicalFacts(result kapextract.ExtractionResult) []kapfinance.Fact {
	out := make([]kapfinance.Fact, 0, len(result.FinancialFacts))
	for _, fact := range result.FinancialFacts {
		out = append(out, kapfinance.Fact{
			ID:                 fact.FactID,
			Ticker:             firstNonEmpty(fact.Ticker, result.Ticker),
			Period:             fact.Period,
			FiscalYear:         fact.FiscalYear,
			DocumentDate:       fact.CreatedAt.Format("2006-01-02"),
			LineItemOriginal:   fact.LineItemOriginal,
			LineItemNormalized: fact.LineItemNormalized,
			StatementType:      string(fact.StatementType),
			Value:              fact.Value,
			Currency:           fact.Currency,
			Unit:               fact.Unit,
			SourceFile:         fact.Source.LocalFilePath,
			SourceDocumentID:   fact.Source.SourceDocumentID,
			SourcePage:         fact.Source.SourcePage,
			SourceTableID:      fact.Source.SourceTableID,
			SourceText:         firstNonEmpty(fact.Source.SourceText, fact.Source.SourceRow),
			Confidence:         firstPositive(fact.ConfidenceScore, fact.Source.ConfidenceScore),
			ReviewRequired:     fact.ReviewRequired,
			ValidationStatus:   string(fact.ValidationStatus),
			CreatedAt:          fact.CreatedAt,
		})
	}
	return out
}

func financialDetails(coverage FinancialCoverage) []string {
	return []string{
		fmt.Sprintf("facts_read=%d", coverage.FactsRead),
		fmt.Sprintf("facts_accepted=%d", coverage.FactsAccepted),
		fmt.Sprintf("fields=%d", coverage.Fields),
		fmt.Sprintf("periods=%d", coverage.Periods),
	}
}

func checkSector(opts Options, sectors map[string]classification.Entry, symbol string, checks []Check) (SectorCoverage, []Check) {
	entry, ok := sectors[symbol]
	coverage := SectorCoverage{Status: checkFail}
	if ok {
		coverage = SectorCoverage{
			Status:     checkPass,
			Sector:     entry.Sector,
			Industry:   entry.Industry,
			PeerGroup:  entry.PeerGroup,
			PeerCount:  len(entry.PeerSymbols),
			Source:     entry.Source,
			Confidence: entry.Confidence,
			Evidence:   append([]string{}, entry.Evidence...),
			Warnings:   append([]string{}, entry.Warnings...),
		}
	}
	switch {
	case !ok || strings.TrimSpace(entry.Sector) == "":
		checks = append(checks, Check{
			Name:        "sector_peer_group",
			Status:      checkFail,
			Severity:    severityErr,
			Score:       0,
			Message:     "sector classification is missing",
			Details:     []string{opts.SectorFile},
			Remediation: "go run ./cmd/hissebot sync kap-sectors && go run ./cmd/hissebot sync sectors",
		})
	case len(entry.PeerSymbols) < opts.MinPeers:
		coverage.Status = checkWarn
		checks = append(checks, Check{
			Name:        "sector_peer_group",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       65,
			Message:     "peer group is too small for stable relative valuation",
			Details:     []string{fmt.Sprintf("peer_count=%d", len(entry.PeerSymbols)), fmt.Sprintf("min_peers=%d", opts.MinPeers)},
			Remediation: "Sektör sınıflandırması ve peer mapping kaynaklarını genişletin.",
		})
	case entry.Confidence > 0 && entry.Confidence < 0.60:
		coverage.Status = checkWarn
		checks = append(checks, Check{
			Name:        "sector_peer_group",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       70,
			Message:     "sector classification confidence is low",
			Details:     []string{fmt.Sprintf("confidence=%.2f", entry.Confidence)},
			Remediation: "KAP sektör veya güvenilir harici sektör kaynağıyla sınıflandırmayı doğrulayın.",
		})
	default:
		checks = append(checks, Check{
			Name:     "sector_peer_group",
			Status:   checkPass,
			Severity: severityInfo,
			Score:    100,
			Details: []string{
				"sector=" + entry.Sector,
				"industry=" + entry.Industry,
				fmt.Sprintf("peer_count=%d", len(entry.PeerSymbols)),
			},
		})
	}
	return coverage, checks
}

func loadSectorFile(path string) (map[string]classification.Entry, error) {
	out := map[string]classification.Entry{}
	if strings.TrimSpace(path) == "" {
		return out, nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var file classification.File
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for symbol, entry := range file.Entries {
		symbol = storage.NormalizeTicker(symbol)
		if symbol != "" {
			out[symbol] = entry
		}
	}
	return out, nil
}

func checkCorporateActions(opts Options, symbol string, checks []Check) (CorporateActionCoverage, []Check) {
	set := corporateactions.Load(opts.EquitiesDir, symbol)
	coverage := CorporateActionCoverage{
		Status:                      set.Status,
		Actions:                     len(set.Actions),
		VerifiedActions:             set.VerifiedActions,
		CandidateActions:            set.CandidateActions,
		ReviewRequiredActions:       set.ReviewRequiredActions,
		AdjustmentReadyActions:      set.AdjustmentReadyActions,
		MissingEffectiveDateActions: set.MissingEffectiveDateActions,
		MissingAdjustmentActions:    set.MissingAdjustmentActions,
		SourceFiles:                 append([]string{}, set.SourceFiles...),
		Warnings:                    append([]string{}, set.Warnings...),
	}
	switch set.Status {
	case "adjustment_ready":
		checks = append(checks, Check{
			Name:     "corporate_actions_adjustment",
			Status:   checkPass,
			Severity: severityInfo,
			Score:    100,
			Details:  []string{fmt.Sprintf("actions=%d", len(set.Actions)), fmt.Sprintf("adjustment_ready=%d", set.AdjustmentReadyActions)},
		})
	case "missing":
		checks = append(checks, Check{
			Name:        "corporate_actions_adjustment",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       70,
			Message:     "corporate action adjustment source is missing",
			Remediation: "Temettü, bedelli/bedelsiz ve bölünme kaynaklarını doğrulanmış kurumsal aksiyon dosyasına bağlayın.",
		})
	default:
		checks = append(checks, Check{
			Name:        "corporate_actions_adjustment",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       60,
			Message:     "corporate actions require review before backtest-grade adjusted pricing",
			Details:     set.Warnings,
			Remediation: "Eksik effective date, oran veya nakit tutarı olan kurumsal aksiyonları doğrulayın.",
		})
	}
	return coverage, checks
}

func checkMacro(cfg config.Config, checks []Check) (MacroCoverage, []Check) {
	files := []string{cfg.TUIKGDPFile, cfg.TUIKInflationFile, cfg.VAPIndexPortfolioFile}
	coverage := MacroCoverage{Status: checkPass}
	for _, path := range files {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			coverage.Files = append(coverage.Files, path)
		} else {
			coverage.Missing = append(coverage.Missing, path)
		}
	}
	if len(coverage.Missing) > 0 {
		coverage.Status = checkWarn
		checks = append(checks, Check{
			Name:        "macro_context",
			Status:      checkWarn,
			Severity:    severityWarn,
			Score:       75,
			Message:     "macro or index-regime context is incomplete",
			Details:     coverage.Missing,
			Remediation: "Makro ve endeks portföy sync adımlarını tamamlayın; eksikse raporu market-regime sınırlı kabul edin.",
		})
		return coverage, checks
	}
	checks = append(checks, Check{
		Name:     "macro_context",
		Status:   checkPass,
		Severity: severityInfo,
		Score:    100,
		Details:  coverage.Files,
	})
	return coverage, checks
}

func finalizeSymbol(out *SymbolReadiness) {
	score := 100.0
	for _, check := range out.Checks {
		switch check.Severity {
		case severityErr:
			score -= 25
			out.Blockers = appendUnique(out.Blockers, check.Name)
			out.RecommendedActions = appendUnique(out.RecommendedActions, check.Remediation)
		case severityWarn:
			score -= 10
			out.Warnings = appendUnique(out.Warnings, check.Name)
			out.RecommendedActions = appendUnique(out.RecommendedActions, check.Remediation)
		default:
			if check.Score > 0 && check.Score < 100 {
				score -= 2
			}
		}
	}
	if score < 0 {
		score = 0
	}
	out.Score = round1(score)
	switch {
	case len(out.Blockers) > 0:
		out.Status = StatusBlocked
	case len(out.Warnings) > 0:
		out.Status = StatusLimited
	default:
		out.Status = StatusDecisionReady
	}
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
