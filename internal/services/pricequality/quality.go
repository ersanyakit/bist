package pricequality

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/storage"
	"hissebot/internal/util"
)

const (
	StatusReadyForVerifiedClose = "ready_for_verified_close"
	StatusProvisionalLastPrice  = "provisional_last_price_only"
	StatusStaleOrConflicting    = "stale_or_conflicting"
	StatusMissingPriceData      = "missing_price_data"

	DefaultConflictToleranceBps = 10.0
)

type Options struct {
	EquitiesDir          string
	OutputPath           string
	SampleLimit          int
	StaleAfter           time.Duration
	ConflictToleranceBps float64
	Now                  func() time.Time
}

type Report struct {
	GeneratedAtUTC   time.Time             `json:"generated_at_utc"`
	EquitiesDir      string                `json:"equities_dir"`
	TotalSymbols     int                   `json:"total_symbols"`
	CandidateSources SourceCoverage        `json:"candidate_sources"`
	Reconciliation   ReconciliationSummary `json:"reconciliation"`
	FinalClose       FinalCloseSummary     `json:"final_close"`
	QualityGates     QualityGates          `json:"quality_gates"`
	MissingCounts    map[string]int        `json:"missing_counts,omitempty"`
	MissingSamples   map[string][]string   `json:"missing_samples,omitempty"`
	Symbols          []SymbolReport        `json:"symbols"`
	AcquisitionPlan  []AcquisitionPlanItem `json:"acquisition_plan,omitempty"`
	Warnings         []string              `json:"warnings,omitempty"`
}

type SourceCoverage struct {
	OHLCVSnapshots            int `json:"ohlcv_snapshots"`
	TradingViewRawOHLCV       int `json:"tradingview_raw_ohlcv"`
	DailyCharts               int `json:"daily_charts"`
	AdjustedDailyCharts       int `json:"adjusted_daily_charts"`
	MarketWSSnapshots         int `json:"market_ws_snapshots"`
	OfficialFinalCloseSources int `json:"official_final_close_sources"`
}

type ReconciliationSummary struct {
	ComparableSymbols int `json:"comparable_symbols"`
	MatchingSymbols   int `json:"matching_symbols"`
	ConflictSymbols   int `json:"conflict_symbols"`
	StaleSymbols      int `json:"stale_symbols"`
}

type FinalCloseSummary struct {
	ReadySymbols       int `json:"ready_symbols"`
	ProvisionalSymbols int `json:"provisional_symbols"`
	BlockedSymbols     int `json:"blocked_symbols"`
	MissingSymbols     int `json:"missing_symbols"`
}

type QualityGates struct {
	FinalCloseSourceAvailable bool     `json:"final_close_source_available"`
	DailyChartCoverageBroad   bool     `json:"daily_chart_coverage_broad"`
	StalePriceBlocked         bool     `json:"stale_price_blocked"`
	PriceConflictsBlocked     bool     `json:"price_conflicts_blocked"`
	OfficialCloseRequired     bool     `json:"official_close_required"`
	BlockingReasons           []string `json:"blocking_reasons,omitempty"`
}

type SymbolReport struct {
	Symbol                string           `json:"symbol"`
	Status                string           `json:"status"`
	ReadyForDecision      bool             `json:"ready_for_decision"`
	ReadyForVerifiedClose bool             `json:"ready_for_verified_close"`
	LatestTradingDate     string           `json:"latest_trading_date,omitempty"`
	SelectedClose         *CloseCandidate  `json:"selected_close,omitempty"`
	Candidates            []CloseCandidate `json:"candidates,omitempty"`
	Conflict              bool             `json:"conflict,omitempty"`
	ConflictBps           float64          `json:"conflict_bps,omitempty"`
	Stale                 bool             `json:"stale,omitempty"`
	MissingFields         []string         `json:"missing_fields,omitempty"`
	BlockingReasons       []string         `json:"blocking_reasons,omitempty"`
}

type CloseCandidate struct {
	Source      string     `json:"source"`
	SourceType  string     `json:"source_type"`
	Close       float64    `json:"close"`
	Timestamp   *time.Time `json:"timestamp,omitempty"`
	TradingDate string     `json:"trading_date,omitempty"`
	FetchedAt   *time.Time `json:"fetched_at,omitempty"`
	Stale       bool       `json:"stale,omitempty"`
	Final       bool       `json:"final,omitempty"`
	Official    bool       `json:"official,omitempty"`
	Path        string     `json:"path,omitempty"`
	Field       string     `json:"field,omitempty"`
}

type AcquisitionPlanItem struct {
	Priority   int      `json:"priority"`
	Key        string   `json:"key"`
	Action     string   `json:"action"`
	Command    string   `json:"command,omitempty"`
	Acceptance []string `json:"acceptance,omitempty"`
}

type chartFile struct {
	Source    string        `json:"source"`
	Ticker    string        `json:"ticker"`
	Symbol    string        `json:"symbol"`
	Interval  string        `json:"interval"`
	Transport string        `json:"transport"`
	FetchedAt time.Time     `json:"fetched_at"`
	Candles   []chartCandle `json:"candles"`
}

type chartCandle struct {
	Time  any     `json:"time"`
	Close float64 `json:"close"`
}

type ohlcvFile struct {
	Source    string    `json:"source"`
	Symbol    string    `json:"symbol"`
	Close     float64   `json:"close"`
	Time      int64     `json:"time"`
	FetchedAt time.Time `json:"fetched_at"`
}

type rawOHLCVFile struct {
	Source    string    `json:"source"`
	Symbol    string    `json:"symbol"`
	Columns   []string  `json:"columns"`
	Values    []any     `json:"values"`
	FetchedAt time.Time `json:"fetched_at"`
}

type officialCloseFile struct {
	Source          string    `json:"source"`
	Symbol          string    `json:"symbol"`
	TradingDate     string    `json:"trading_date"`
	Close           float64   `json:"close"`
	IsFinalClose    bool      `json:"is_final_close"`
	SourceTimestamp time.Time `json:"source_timestamp"`
	FetchedAt       time.Time `json:"fetched_at"`
}

func Build(ctx context.Context, opts Options) (Report, error) {
	if strings.TrimSpace(opts.EquitiesDir) == "" {
		return Report{}, errors.New("equities dir is required")
	}
	if opts.SampleLimit <= 0 {
		opts.SampleLimit = 50
	}
	if opts.StaleAfter <= 0 {
		// 120h (5 gün): BIST hafta sonu kapalıdır; cuma kapanış verisi pazartesi
		// sabahı ~60 saat olur. 36h eşiği gereksiz stale=true tetikler.
		opts.StaleAfter = 120 * time.Hour
	}
	if opts.ConflictToleranceBps <= 0 {
		opts.ConflictToleranceBps = DefaultConflictToleranceBps
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	report := Report{
		GeneratedAtUTC: now().UTC(),
		EquitiesDir:    filepath.Clean(opts.EquitiesDir),
		MissingCounts:  map[string]int{},
		MissingSamples: map[string][]string{},
		QualityGates: QualityGates{
			StalePriceBlocked:     true,
			PriceConflictsBlocked: true,
			OfficialCloseRequired: true,
		},
	}
	symbols, err := equitySymbols(opts.EquitiesDir)
	if err != nil {
		return report, err
	}
	report.TotalSymbols = len(symbols)
	for _, symbol := range symbols {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		symbolReport := buildSymbolReport(symbol, opts, now())
		report.Symbols = append(report.Symbols, symbolReport)
		applySymbolReport(&report, symbolReport, opts.SampleLimit)
	}
	sort.Slice(report.Symbols, func(i, j int) bool {
		return report.Symbols[i].Symbol < report.Symbols[j].Symbol
	})
	finalizeQualityGates(&report)
	report.AcquisitionPlan = defaultAcquisitionPlan()
	if strings.TrimSpace(opts.OutputPath) != "" {
		if err := util.WriteJSON(opts.OutputPath, report); err != nil {
			return report, err
		}
	}
	return report, nil
}

func InspectSymbol(ctx context.Context, symbol string, opts Options) (SymbolReport, error) {
	if err := ctx.Err(); err != nil {
		return SymbolReport{}, err
	}
	if strings.TrimSpace(opts.EquitiesDir) == "" {
		return SymbolReport{}, errors.New("equities dir is required")
	}
	if opts.StaleAfter <= 0 {
		// 120h (5 gün): BIST hafta sonu kapalıdır; cuma kapanış verisi pazartesi
		// sabahı ~60 saat olur. 36h eşiği gereksiz stale=true tetikler.
		opts.StaleAfter = 120 * time.Hour
	}
	if opts.ConflictToleranceBps <= 0 {
		opts.ConflictToleranceBps = DefaultConflictToleranceBps
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	normalized := storage.NormalizeTicker(symbol)
	if normalized == "" {
		return SymbolReport{}, errors.New("symbol is required")
	}
	return buildSymbolReport(normalized, opts, now()), nil
}

func equitySymbols(equitiesDir string) ([]string, error) {
	entries, err := os.ReadDir(equitiesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []string{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(equitiesDir, entry.Name(), "equity.json")); err != nil {
			continue
		}
		symbol := storage.NormalizeTicker(entry.Name())
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		out = append(out, symbol)
	}
	sort.Strings(out)
	return out, nil
}

func buildSymbolReport(symbol string, opts Options, now time.Time) SymbolReport {
	base := filepath.Join(opts.EquitiesDir, symbol)
	candidates := collectCandidates(base, symbol, now, opts.StaleAfter)
	sortCandidates(candidates)
	report := SymbolReport{Symbol: symbol}
	return finalizeSymbolReport(report, base, candidates, opts.ConflictToleranceBps)
}

func ReconcileWithAnalysisClose(report SymbolReport, close float64, candleTime time.Time, fetchedAt time.Time, source string, toleranceBps float64) SymbolReport {
	if close <= 0 || strings.TrimSpace(report.Symbol) == "" {
		return report
	}
	if toleranceBps <= 0 {
		toleranceBps = DefaultConflictToleranceBps
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "analysis_provider"
	}
	candidate := CloseCandidate{
		Source:      source,
		SourceType:  "analysis_ohlcv",
		Close:       close,
		TradingDate: tradingDate(candleTime),
		Field:       "timeframes.1D.candles[-1].close",
	}
	if !candleTime.IsZero() {
		ts := candleTime.UTC()
		candidate.Timestamp = &ts
	}
	if !fetchedAt.IsZero() {
		ts := fetchedAt.UTC()
		candidate.FetchedAt = &ts
	}
	candidates := append([]CloseCandidate{}, report.Candidates...)
	candidates = append(candidates, candidate)
	candidates = dedupeCandidates(candidates)
	sortCandidates(candidates)
	return finalizeSymbolReport(SymbolReport{
		Symbol:        report.Symbol,
		MissingFields: append([]string{}, report.MissingFields...),
	}, "", candidates, toleranceBps)
}

func ReconcileWithOfficialFinalClose(report SymbolReport, close float64, tradingDate string, sourceTimestamp time.Time, fetchedAt time.Time, source string, path string, toleranceBps float64) SymbolReport {
	if close <= 0 || strings.TrimSpace(report.Symbol) == "" {
		return report
	}
	if toleranceBps <= 0 {
		toleranceBps = DefaultConflictToleranceBps
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "official_close"
	}
	candidate := CloseCandidate{
		Source:      source,
		SourceType:  "official_final_close",
		Close:       close,
		TradingDate: strings.TrimSpace(tradingDate),
		Final:       true,
		Official:    true,
		Field:       "close",
	}
	if strings.TrimSpace(path) != "" {
		candidate.Path = filepath.Clean(path)
	}
	if !sourceTimestamp.IsZero() {
		ts := sourceTimestamp.UTC()
		candidate.Timestamp = &ts
		if candidate.TradingDate == "" {
			candidate.TradingDate = tradingDateFromTime(ts)
		}
	}
	if !fetchedAt.IsZero() {
		ts := fetchedAt.UTC()
		candidate.FetchedAt = &ts
	}
	candidates := append([]CloseCandidate{}, report.Candidates...)
	candidates = append(candidates, candidate)
	candidates = dedupeCandidates(candidates)
	sortCandidates(candidates)
	return finalizeSymbolReport(SymbolReport{
		Symbol:        report.Symbol,
		MissingFields: removeString(report.MissingFields, "official_final_close"),
	}, "", candidates, toleranceBps)
}

func finalizeSymbolReport(report SymbolReport, base string, candidates []CloseCandidate, conflictToleranceBps float64) SymbolReport {
	report.Candidates = candidates
	if strings.TrimSpace(base) != "" {
		missing := missingFields(base, candidates)
		report.MissingFields = append(report.MissingFields, missing...)
	}
	if len(candidates) == 0 {
		report.Status = StatusMissingPriceData
		report.BlockingReasons = append(report.BlockingReasons, "no_local_price_candidate")
		return report
	}
	report.SelectedClose = selectClose(candidates)
	report.LatestTradingDate = selectedTradingDate(report.SelectedClose, candidates)
	// Old cache files are useful lineage, but they must not invalidate a newer
	// decision price. Staleness is therefore evaluated on the selected/latest
	// candidate; conflicts are already restricted to the latest trading date.
	report.Stale = report.SelectedClose == nil || report.SelectedClose.Stale
	report.Conflict, report.ConflictBps = conflictOnTradingDate(candidates, report.LatestTradingDate, conflictToleranceBps)
	if report.SelectedClose == nil || !report.SelectedClose.Official || !report.SelectedClose.Final {
		report.BlockingReasons = append(report.BlockingReasons, "official_final_close_missing")
	}
	if report.Stale {
		report.BlockingReasons = append(report.BlockingReasons, "stale_price_source_present")
	}
	if report.Conflict {
		report.BlockingReasons = append(report.BlockingReasons, "price_sources_conflict")
	}
	report.ReadyForDecision = report.SelectedClose != nil && !report.Stale && !report.Conflict
	switch {
	case report.ReadyForDecision && report.SelectedClose.Official && report.SelectedClose.Final:
		report.Status = StatusReadyForVerifiedClose
		report.ReadyForVerifiedClose = true
	case !report.ReadyForDecision:
		report.Status = StatusStaleOrConflicting
	default:
		report.Status = StatusProvisionalLastPrice
	}
	return report
}

func collectCandidates(base string, symbol string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	out := []CloseCandidate{}
	out = append(out, readOHLCVCandidate(filepath.Join(base, "ohlcv.json"), symbol, "ohlcv_snapshot", now, staleAfter)...)
	out = append(out, readRawOHLCVCandidate(filepath.Join(base, "tradingview", "ohlcv.json"), symbol, "tradingview_raw_ohlcv", now, staleAfter)...)
	out = append(out, readChartCandidate(filepath.Join(base, "charts", "D.json"), symbol, "daily_chart", now, staleAfter)...)
	out = append(out, readChartCandidate(filepath.Join(base, "charts", "D_adjusted.json"), symbol, "adjusted_daily_chart", now, staleAfter)...)
	out = append(out, readOfficialCloseCandidates(base, symbol, now, staleAfter)...)
	out = append(out, readMarketWSCandidates(base, symbol, now, staleAfter)...)
	return dedupeCandidates(out)
}

func readOHLCVCandidate(path string, symbol string, sourceType string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	var data ohlcvFile
	if err := util.ReadJSON(path, &data); err != nil {
		return nil
	}
	if data.Close <= 0 {
		return nil
	}
	ts := timeFromUnix(data.Time)
	candidate := CloseCandidate{
		Source:      firstNonEmpty(data.Source, "tradingview"),
		SourceType:  sourceType,
		Close:       data.Close,
		Timestamp:   timePtr(ts),
		TradingDate: tradingDate(ts),
		Path:        filepath.Clean(path),
		Field:       "close",
	}
	if !data.FetchedAt.IsZero() {
		candidate.FetchedAt = timePtr(data.FetchedAt.UTC())
	}
	candidate.Stale = isStale(candidate, now, staleAfter)
	return []CloseCandidate{candidate}
}

func readRawOHLCVCandidate(path string, symbol string, sourceType string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	var data rawOHLCVFile
	if err := util.ReadJSON(path, &data); err != nil {
		return nil
	}
	closeIndex := columnIndex(data.Columns, "close")
	timeIndex := columnIndex(data.Columns, "time")
	if closeIndex < 0 || closeIndex >= len(data.Values) {
		return nil
	}
	closeValue, ok := numberValue(data.Values[closeIndex])
	if !ok || closeValue <= 0 {
		return nil
	}
	var ts time.Time
	if timeIndex >= 0 && timeIndex < len(data.Values) {
		if rawTime, ok := numberValue(data.Values[timeIndex]); ok {
			ts = timeFromUnix(int64(rawTime))
		}
	}
	candidate := CloseCandidate{
		Source:     firstNonEmpty(data.Source, "tradingview"),
		SourceType: sourceType,
		Close:      closeValue,
		Path:       filepath.Clean(path),
		Field:      "close",
	}
	if !ts.IsZero() {
		candidate.Timestamp = timePtr(ts)
		candidate.TradingDate = tradingDate(ts)
	}
	if !data.FetchedAt.IsZero() {
		candidate.FetchedAt = timePtr(data.FetchedAt.UTC())
	}
	candidate.Stale = isStale(candidate, now, staleAfter)
	return []CloseCandidate{candidate}
}

func readChartCandidate(path string, symbol string, sourceType string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	var data chartFile
	if err := util.ReadJSON(path, &data); err != nil {
		return nil
	}
	if len(data.Candles) == 0 {
		return nil
	}
	last := data.Candles[len(data.Candles)-1]
	if last.Close <= 0 {
		return nil
	}
	ts, _ := parseTimeAny(last.Time)
	candidate := CloseCandidate{
		Source:      firstNonEmpty(data.Source, "tradingview"),
		SourceType:  sourceType,
		Close:       last.Close,
		Timestamp:   timePtr(ts),
		TradingDate: tradingDate(ts),
		Path:        filepath.Clean(path),
		Field:       "candles[-1].close",
	}
	if !data.FetchedAt.IsZero() {
		candidate.FetchedAt = timePtr(data.FetchedAt.UTC())
	}
	candidate.Stale = isStale(candidate, now, staleAfter)
	return []CloseCandidate{candidate}
}

func readOfficialCloseCandidates(base string, symbol string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	paths := []string{
		filepath.Join(base, "price", "official_close.json"),
		filepath.Join(base, "price", "official_closes.json"),
		filepath.Join(base, "prices", "official_close.json"),
		filepath.Join(base, "official_close.json"),
	}
	var out []CloseCandidate
	for _, path := range paths {
		bytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, parseOfficialClosePayload(path, bytes, symbol, now, staleAfter)...)
	}
	return out
}

func parseOfficialClosePayload(path string, bytes []byte, symbol string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	var single officialCloseFile
	if err := json.Unmarshal(bytes, &single); err == nil && single.Close > 0 {
		return officialCloseToCandidates(path, []officialCloseFile{single}, symbol, now, staleAfter)
	}
	var many []officialCloseFile
	if err := json.Unmarshal(bytes, &many); err == nil {
		return officialCloseToCandidates(path, many, symbol, now, staleAfter)
	}
	return nil
}

func officialCloseToCandidates(path string, records []officialCloseFile, symbol string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	out := []CloseCandidate{}
	for _, record := range records {
		if record.Close <= 0 {
			continue
		}
		recordSymbol := storage.NormalizeTicker(firstNonEmpty(record.Symbol, symbol))
		if recordSymbol != "" && recordSymbol != symbol {
			continue
		}
		candidate := CloseCandidate{
			Source:      firstNonEmpty(record.Source, "official_close"),
			SourceType:  "official_final_close",
			Close:       record.Close,
			TradingDate: record.TradingDate,
			Path:        filepath.Clean(path),
			Field:       "close",
			Final:       record.IsFinalClose,
			Official:    true,
		}
		if !record.SourceTimestamp.IsZero() {
			ts := record.SourceTimestamp.UTC()
			candidate.Timestamp = &ts
			if candidate.TradingDate == "" {
				candidate.TradingDate = tradingDate(ts)
			}
		}
		if !record.FetchedAt.IsZero() {
			fetchedAt := record.FetchedAt.UTC()
			candidate.FetchedAt = &fetchedAt
		}
		candidate.Stale = isStale(candidate, now, staleAfter)
		out = append(out, candidate)
	}
	return out
}

func readMarketWSCandidates(base string, symbol string, now time.Time, staleAfter time.Duration) []CloseCandidate {
	marketWSDir := filepath.Join(base, "market_ws")
	paths := []string{
		filepath.Join(marketWSDir, "symbols_summary_data.json"),
		filepath.Join(marketWSDir, "live_symbol_snapshot.json"),
		filepath.Join(marketWSDir, "snapshot.json"),
		filepath.Join(marketWSDir, "btum_snapshot_data.json"),
		filepath.Join(marketWSDir, "equilibrium_data.json"),
	}
	var out []CloseCandidate
	for _, path := range paths {
		bytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload any
		if err := json.Unmarshal(bytes, &payload); err != nil {
			continue
		}
		walkMarketPayload(payload, symbol, filepath.Clean(path), nil, false, &out)
	}
	for i := range out {
		out[i].Stale = isStale(out[i], now, staleAfter)
	}
	return out
}

func walkMarketPayload(value any, symbol string, path string, inheritedUpdatedAt *time.Time, forceSymbol bool, out *[]CloseCandidate) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			walkMarketPayload(item, symbol, path, inheritedUpdatedAt, forceSymbol, out)
		}
	case map[string]any:
		updatedAt := inheritedUpdatedAt
		if parsed, ok := parseTimeAny(firstPresent(typed, "updated_at", "fetched_at", "received_at", "time")); ok {
			updatedAt = timePtr(parsed)
		}
		matches := forceSymbol || marketRecordMatchesSymbol(typed, symbol)
		if data, ok := typed["data"].(map[string]any); ok {
			if !matches && marketRecordMatchesSymbol(data, symbol) {
				matches = true
			}
			if matches {
				appendMarketCandidate(data, symbol, path, updatedAt, out)
			}
		}
		if matches {
			appendMarketCandidate(typed, symbol, path, updatedAt, out)
		}
		for key, child := range typed {
			childForce := strings.EqualFold(storage.NormalizeTicker(key), symbol)
			walkMarketPayload(child, symbol, path, updatedAt, childForce, out)
		}
	}
}

func appendMarketCandidate(record map[string]any, symbol string, path string, updatedAt *time.Time, out *[]CloseCandidate) {
	for _, field := range []string{"official_close", "close", "last", "last_price", "price", "previous_close_alt", "equilibrium_price_or_last_lot"} {
		value, ok := numberFromMap(record, field)
		if !ok || value <= 0 {
			continue
		}
		sourceType := "market_ws"
		if field == "previous_close_alt" {
			sourceType = "market_ws_previous_close"
		}
		if field == "official_close" {
			sourceType = "market_ws_official_close_candidate"
		}
		candidate := CloseCandidate{
			Source:     "market_ws",
			SourceType: sourceType,
			Close:      value,
			Path:       path,
			Field:      field,
		}
		if updatedAt != nil {
			ts := updatedAt.UTC()
			candidate.Timestamp = &ts
			candidate.FetchedAt = &ts
			candidate.TradingDate = tradingDate(ts)
		}
		*out = append(*out, candidate)
		return
	}
}

func marketRecordMatchesSymbol(record map[string]any, symbol string) bool {
	for _, key := range []string{"symbol", "code", "ticker", "name"} {
		if raw, ok := record[key]; ok {
			if strings.EqualFold(storage.NormalizeTicker(fmt.Sprint(raw)), symbol) {
				return true
			}
		}
	}
	return false
}

func missingFields(base string, candidates []CloseCandidate) []string {
	have := map[string]bool{}
	for _, candidate := range candidates {
		have[candidate.SourceType] = true
	}
	missing := []string{}
	if !have["ohlcv_snapshot"] && !fileExists(filepath.Join(base, "ohlcv.json")) {
		missing = append(missing, "ohlcv_snapshot")
	}
	if !have["daily_chart"] && !fileExists(filepath.Join(base, "charts", "D.json")) {
		missing = append(missing, "daily_chart")
	}
	if !have["adjusted_daily_chart"] && !fileExists(filepath.Join(base, "charts", "D_adjusted.json")) {
		missing = append(missing, "adjusted_daily_chart")
	}
	if !hasMarketWSCandidate(candidates) {
		missing = append(missing, "market_ws_close_candidate")
	}
	if !have["official_final_close"] {
		missing = append(missing, "official_final_close")
	}
	return missing
}

func hasMarketWSCandidate(candidates []CloseCandidate) bool {
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate.SourceType, "market_ws") {
			return true
		}
	}
	return false
}

func selectClose(candidates []CloseCandidate) *CloseCandidate {
	if len(candidates) == 0 {
		return nil
	}
	for _, candidate := range candidates {
		if candidate.Official && candidate.Final && !candidate.Stale {
			copy := candidate
			return &copy
		}
	}
	for _, candidate := range candidates {
		if !candidate.Stale {
			copy := candidate
			return &copy
		}
	}
	copy := candidates[0]
	return &copy
}

func sortCandidates(candidates []CloseCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		left := candidateTime(candidates[i])
		right := candidateTime(candidates[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		if candidates[i].Official != candidates[j].Official {
			return candidates[i].Official
		}
		if candidates[i].Final != candidates[j].Final {
			return candidates[i].Final
		}
		return candidates[i].SourceType < candidates[j].SourceType
	})
}

func candidateTime(candidate CloseCandidate) time.Time {
	if candidate.Timestamp != nil {
		return candidate.Timestamp.UTC()
	}
	if candidate.FetchedAt != nil {
		return candidate.FetchedAt.UTC()
	}
	return time.Time{}
}

func latestTradingDate(candidates []CloseCandidate) string {
	for _, candidate := range candidates {
		if candidate.TradingDate != "" {
			return candidate.TradingDate
		}
	}
	return ""
}

func selectedTradingDate(selected *CloseCandidate, candidates []CloseCandidate) string {
	if selected != nil && selected.TradingDate != "" {
		return selected.TradingDate
	}
	return latestTradingDate(candidates)
}

func hasStale(candidates []CloseCandidate) bool {
	for _, candidate := range candidates {
		if candidate.Stale {
			return true
		}
	}
	return false
}

func conflictOnLatestTradingDate(candidates []CloseCandidate, toleranceBps float64) (bool, float64) {
	date := latestTradingDate(candidates)
	return conflictOnTradingDate(candidates, date, toleranceBps)
}

func conflictOnTradingDate(candidates []CloseCandidate, date string, toleranceBps float64) (bool, float64) {
	if date == "" {
		return false, 0
	}
	values := []float64{}
	for _, candidate := range candidates {
		if candidate.TradingDate == date && candidate.Close > 0 {
			values = append(values, candidate.Close)
		}
	}
	if len(values) < 2 {
		return false, 0
	}
	sort.Float64s(values)
	minValue := values[0]
	maxValue := values[len(values)-1]
	if minValue <= 0 {
		return false, 0
	}
	bps := ((maxValue - minValue) / minValue) * 10_000
	return bps > toleranceBps, bps
}

func tradingDateFromTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func removeString(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == target {
			continue
		}
		out = append(out, value)
	}
	return out
}

func applySymbolReport(report *Report, symbolReport SymbolReport, sampleLimit int) {
	sourceTypes := map[string]bool{}
	for _, candidate := range symbolReport.Candidates {
		sourceTypes[candidate.SourceType] = true
	}
	if sourceTypes["ohlcv_snapshot"] {
		report.CandidateSources.OHLCVSnapshots++
	}
	if sourceTypes["tradingview_raw_ohlcv"] {
		report.CandidateSources.TradingViewRawOHLCV++
	}
	if sourceTypes["daily_chart"] {
		report.CandidateSources.DailyCharts++
	}
	if sourceTypes["adjusted_daily_chart"] {
		report.CandidateSources.AdjustedDailyCharts++
	}
	for sourceType := range sourceTypes {
		if strings.HasPrefix(sourceType, "market_ws") {
			report.CandidateSources.MarketWSSnapshots++
			break
		}
	}
	if sourceTypes["official_final_close"] {
		report.CandidateSources.OfficialFinalCloseSources++
	}
	if len(symbolReport.Candidates) >= 2 {
		report.Reconciliation.ComparableSymbols++
		if symbolReport.Conflict {
			report.Reconciliation.ConflictSymbols++
		} else {
			report.Reconciliation.MatchingSymbols++
		}
	}
	if symbolReport.Stale {
		report.Reconciliation.StaleSymbols++
		addMissing(report, "stale_price", symbolReport.Symbol, sampleLimit)
	}
	if symbolReport.Conflict {
		addMissing(report, "price_conflict", symbolReport.Symbol, sampleLimit)
	}
	for _, field := range symbolReport.MissingFields {
		addMissing(report, "missing_"+field, symbolReport.Symbol, sampleLimit)
	}
	switch symbolReport.Status {
	case StatusReadyForVerifiedClose:
		report.FinalClose.ReadySymbols++
	case StatusProvisionalLastPrice:
		report.FinalClose.ProvisionalSymbols++
		addMissing(report, "not_ready_for_verified_close", symbolReport.Symbol, sampleLimit)
	case StatusMissingPriceData:
		report.FinalClose.MissingSymbols++
		report.FinalClose.BlockedSymbols++
		addMissing(report, "missing_price_data", symbolReport.Symbol, sampleLimit)
		addMissing(report, "not_ready_for_verified_close", symbolReport.Symbol, sampleLimit)
	case StatusStaleOrConflicting:
		report.FinalClose.BlockedSymbols++
		addMissing(report, "not_ready_for_verified_close", symbolReport.Symbol, sampleLimit)
	default:
		addMissing(report, "unknown_price_status", symbolReport.Symbol, sampleLimit)
	}
}

func addMissing(report *Report, key string, symbol string, sampleLimit int) {
	report.MissingCounts[key]++
	if sampleLimit <= 0 {
		return
	}
	samples := report.MissingSamples[key]
	if len(samples) >= sampleLimit {
		return
	}
	report.MissingSamples[key] = append(samples, symbol)
}

func finalizeQualityGates(report *Report) {
	report.QualityGates.FinalCloseSourceAvailable = report.CandidateSources.OfficialFinalCloseSources > 0
	report.QualityGates.DailyChartCoverageBroad = report.TotalSymbols > 0 && float64(report.CandidateSources.DailyCharts)/float64(report.TotalSymbols) >= 0.95
	if !report.QualityGates.FinalCloseSourceAvailable {
		report.QualityGates.BlockingReasons = append(report.QualityGates.BlockingReasons, "official_final_close_source_missing")
	}
	if !report.QualityGates.DailyChartCoverageBroad {
		report.QualityGates.BlockingReasons = append(report.QualityGates.BlockingReasons, "daily_chart_coverage_below_95_pct")
	}
	if report.Reconciliation.ConflictSymbols > 0 {
		report.QualityGates.BlockingReasons = append(report.QualityGates.BlockingReasons, "price_source_conflicts_present")
	}
	if report.Reconciliation.StaleSymbols > 0 {
		report.QualityGates.BlockingReasons = append(report.QualityGates.BlockingReasons, "stale_price_sources_present")
	}
	sort.Strings(report.QualityGates.BlockingReasons)
}

func defaultAcquisitionPlan() []AcquisitionPlanItem {
	return []AcquisitionPlanItem{
		{
			Priority: 1,
			Key:      "official_final_close_feed",
			Action:   "BIST/MKK veya lisansli veri saglayicidan seans sonu resmi kapanis, seans tarihi, yayin zamani ve kaynak hash bilgisini kaydet.",
			Acceptance: []string{
				"Her sembol icin official_final_close adayi uretilir.",
				"close, trading_date, source_timestamp, fetched_at ve source alanlari doludur.",
				"Resmi kapanis yoksa sistem son fiyatla verified close uretmez.",
			},
		},
		{
			Priority: 2,
			Key:      "daily_chart_refresh",
			Action:   "Gunluk chart ve OHLCV cache kapsamlarini tum izleme evreni icin tazele.",
			Command:  "go run ./cmd/hissebot sync charts -intervals D -missing-only=false -skip-existing=false && go run ./cmd/hissebot sync ohlcv",
			Acceptance: []string{
				"daily_chart coverage >= 95%.",
				"ohlcv_snapshot coverage >= 95%.",
				"Stale fiyat sayisi sifira iner veya sadece resmi tatil/kapali seans olarak isaretlenir.",
			},
		},
		{
			Priority: 3,
			Key:      "price_reconciliation_gate",
			Action:   "OHLCV, chart, market-ws ve resmi kapanis kaynaklarini toleransli mutabakata bagla; conflict varsa analiz ve sinyal kapisini kapat.",
			Command:  "go run ./cmd/hissebot sync price-quality",
			Acceptance: []string{
				"price_conflict sayisi sifirdir veya manuel inceleme listesine dusmustur.",
				"not_ready_for_verified_close sembolleri analiz karar motorunda AL/SAT uretmez.",
			},
		},
		{
			Priority: 4,
			Key:      "corporate_action_normalization",
			Action:   "Bedelli, bedelsiz, split ve temettu etkilerini raw/adjusted fiyat ayrimiyla birlikte versiyonla.",
			Command:  "go run ./cmd/hissebot sync corporate-actions -intervals D,W,M",
			Acceptance: []string{
				"Raw kapanis ve adjusted kapanis alanlari ayri tutulur.",
				"Backtest target serisi sadece zamaninda bilinen aksiyonlarla uretilir.",
			},
		},
		{
			Priority: 5,
			Key:      "prediction_target_dataset",
			Action:   "Tahmin modeli icin hedef degiskeni future close degil, tarih/saat kilitli next-session close etiketi olarak uret; lookahead testini zorunlu yap.",
			Acceptance: []string{
				"available_at <= signal_time kurali gecmeden egitim/test kaydi uretilmez.",
				"Gelecek kapanis icin raporda 'kesin' ifade kullanilmaz; sadece olasilik/guven araligi verilir.",
			},
		},
	}
}

func dedupeCandidates(candidates []CloseCandidate) []CloseCandidate {
	out := []CloseCandidate{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.Close <= 0 {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%.8f", candidate.SourceType, candidate.Path, candidate.Field, candidate.TradingDate, candidate.Close)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	return out
}

func isStale(candidate CloseCandidate, now time.Time, staleAfter time.Duration) bool {
	if staleAfter <= 0 {
		return false
	}
	ref := candidateTime(candidate)
	if ref.IsZero() {
		return true
	}
	if now.Before(ref) {
		return false
	}
	return now.Sub(ref) > staleAfter
}

func timeFromUnix(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}

func tradingDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return value.UTC().Format("2006-01-02")
	}
	return value.In(loc).Format("2006-01-02")
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func columnIndex(columns []string, target string) int {
	for i, column := range columns {
		if strings.EqualFold(column, target) {
			return i
		}
	}
	return -1
}

func firstPresent(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func numberFromMap(values map[string]any, key string) (float64, bool) {
	value, ok := values[key]
	if !ok {
		return 0, false
	}
	return numberValue(value)
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		normalized := strings.ReplaceAll(strings.TrimSpace(typed), ",", ".")
		parsed, err := strconv.ParseFloat(normalized, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func parseTimeAny(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), !typed.IsZero()
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02",
		} {
			parsed, err := time.Parse(layout, trimmed)
			if err == nil {
				return parsed.UTC(), true
			}
		}
	case float64:
		return timeFromUnix(int64(typed)), typed > 0
	case int64:
		return timeFromUnix(typed), typed > 0
	case int:
		return timeFromUnix(int64(typed)), typed > 0
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return timeFromUnix(parsed), parsed > 0
		}
	}
	return time.Time{}, false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
