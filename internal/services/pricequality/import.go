package pricequality

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type OfficialCloseRecord struct {
	Source          string    `json:"source"`
	Symbol          string    `json:"symbol"`
	TradingDate     string    `json:"trading_date"`
	Close           float64   `json:"close"`
	IsFinalClose    bool      `json:"is_final_close"`
	SourceTimestamp time.Time `json:"source_timestamp"`
	FetchedAt       time.Time `json:"fetched_at"`
}

type OfficialCloseImportOptions struct {
	EquitiesDir string
	InputPath   string
	Source      string
	DryRun      bool
	Now         func() time.Time
}

type OfficialCloseImportReport struct {
	GeneratedAtUTC time.Time                          `json:"generated_at_utc"`
	InputPath      string                             `json:"input_path"`
	EquitiesDir    string                             `json:"equities_dir"`
	Source         string                             `json:"source"`
	DryRun         bool                               `json:"dry_run,omitempty"`
	RecordsRead    int                                `json:"records_read"`
	Imported       int                                `json:"imported"`
	Skipped        int                                `json:"skipped"`
	Errors         []OfficialCloseImportIssue         `json:"errors,omitempty"`
	Symbols        []OfficialCloseImportSymbolSummary `json:"symbols,omitempty"`
}

type OfficialCloseImportIssue struct {
	Row    int    `json:"row,omitempty"`
	Symbol string `json:"symbol,omitempty"`
	Error  string `json:"error"`
}

type OfficialCloseImportSymbolSummary struct {
	Symbol      string  `json:"symbol"`
	Records     int     `json:"records"`
	LatestDate  string  `json:"latest_date"`
	LatestClose float64 `json:"latest_close"`
	Path        string  `json:"path,omitempty"`
}

func ImportOfficialCloses(ctx context.Context, opts OfficialCloseImportOptions) (OfficialCloseImportReport, error) {
	if strings.TrimSpace(opts.EquitiesDir) == "" {
		return OfficialCloseImportReport{}, errors.New("equities dir is required")
	}
	if strings.TrimSpace(opts.InputPath) == "" {
		return OfficialCloseImportReport{}, errors.New("input file is required")
	}
	if strings.TrimSpace(opts.Source) == "" {
		return OfficialCloseImportReport{}, errors.New("source is required")
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	report := OfficialCloseImportReport{
		GeneratedAtUTC: now().UTC(),
		InputPath:      filepath.Clean(opts.InputPath),
		EquitiesDir:    filepath.Clean(opts.EquitiesDir),
		Source:         strings.TrimSpace(opts.Source),
		DryRun:         opts.DryRun,
	}
	records, issues, err := readOfficialCloseInput(opts.InputPath, report.Source, now())
	if err != nil {
		return report, err
	}
	report.Errors = append(report.Errors, issues...)
	report.RecordsRead = len(records) + len(issues)
	grouped := map[string][]OfficialCloseRecord{}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		symbol := storage.NormalizeTicker(record.Symbol)
		if symbol == "" {
			report.Skipped++
			report.Errors = append(report.Errors, OfficialCloseImportIssue{Error: "symbol is required"})
			continue
		}
		if _, err := os.Stat(filepath.Join(opts.EquitiesDir, symbol, "equity.json")); err != nil {
			report.Skipped++
			report.Errors = append(report.Errors, OfficialCloseImportIssue{Symbol: symbol, Error: "equity.json not found"})
			continue
		}
		record.Symbol = symbol
		grouped[symbol] = append(grouped[symbol], record)
	}
	for symbol, symbolRecords := range grouped {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		summary, err := importOfficialCloseSymbol(opts, symbol, symbolRecords)
		if err != nil {
			report.Skipped += len(symbolRecords)
			report.Errors = append(report.Errors, OfficialCloseImportIssue{Symbol: symbol, Error: err.Error()})
			continue
		}
		report.Imported += len(symbolRecords)
		report.Symbols = append(report.Symbols, summary)
	}
	sort.Slice(report.Symbols, func(i, j int) bool {
		return report.Symbols[i].Symbol < report.Symbols[j].Symbol
	})
	return report, nil
}

func readOfficialCloseInput(path string, defaultSource string, now time.Time) ([]OfficialCloseRecord, []OfficialCloseImportIssue, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return readOfficialCloseJSON(path, defaultSource, now)
	case ".csv", ".txt":
		return readOfficialCloseCSV(path, defaultSource, now)
	default:
		return nil, nil, fmt.Errorf("unsupported official close input %q; expected .csv or .json", path)
	}
}

func readOfficialCloseJSON(path string, defaultSource string, now time.Time) ([]OfficialCloseRecord, []OfficialCloseImportIssue, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var records []OfficialCloseRecord
	if err := json.Unmarshal(bytes, &records); err != nil {
		var single OfficialCloseRecord
		if singleErr := json.Unmarshal(bytes, &single); singleErr != nil {
			return nil, nil, err
		}
		records = []OfficialCloseRecord{single}
	}
	out := []OfficialCloseRecord{}
	issues := []OfficialCloseImportIssue{}
	for i, record := range records {
		normalized, err := normalizeOfficialCloseRecord(record, defaultSource, now)
		if err != nil {
			issues = append(issues, OfficialCloseImportIssue{Row: i + 1, Symbol: record.Symbol, Error: err.Error()})
			continue
		}
		out = append(out, normalized)
	}
	return out, issues, nil
}

func readOfficialCloseCSV(path string, defaultSource string, now time.Time) ([]OfficialCloseRecord, []OfficialCloseImportIssue, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, nil, err
	}
	index := csvHeaderIndex(header)
	out := []OfficialCloseRecord{}
	issues := []OfficialCloseImportIssue{}
	row := 1
	for {
		row++
		values, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			issues = append(issues, OfficialCloseImportIssue{Row: row, Error: err.Error()})
			continue
		}
		record, err := officialCloseRecordFromCSV(index, values, defaultSource, now)
		if err != nil {
			issues = append(issues, OfficialCloseImportIssue{Row: row, Error: err.Error()})
			continue
		}
		out = append(out, record)
	}
	return out, issues, nil
}

func csvHeaderIndex(header []string) map[string]int {
	out := map[string]int{}
	for i, raw := range header {
		key := normalizeHeader(raw)
		if key != "" {
			out[key] = i
		}
	}
	return out
}

func officialCloseRecordFromCSV(index map[string]int, values []string, defaultSource string, now time.Time) (OfficialCloseRecord, error) {
	get := func(keys ...string) string {
		for _, key := range keys {
			i, ok := index[normalizeHeader(key)]
			if ok && i >= 0 && i < len(values) {
				return strings.TrimSpace(values[i])
			}
		}
		return ""
	}
	closeValue, err := parseCloseValue(get("close", "kapanis", "kapanış", "kapanis_fiyati", "kapanış_fiyatı"))
	if err != nil {
		return OfficialCloseRecord{}, err
	}
	sourceTimestamp, err := parseRequiredTime(get("source_timestamp", "published_at", "yayin_zamani", "yayın_zamanı", "available_at"))
	if err != nil {
		return OfficialCloseRecord{}, fmt.Errorf("source_timestamp: %w", err)
	}
	fetchedAt := now.UTC()
	if value := get("fetched_at", "imported_at"); value != "" {
		parsed, err := parseRequiredTime(value)
		if err != nil {
			return OfficialCloseRecord{}, fmt.Errorf("fetched_at: %w", err)
		}
		fetchedAt = parsed
	}
	record := OfficialCloseRecord{
		Source:          firstNonEmpty(get("source", "kaynak"), defaultSource),
		Symbol:          get("symbol", "ticker", "kod", "code"),
		TradingDate:     get("trading_date", "date", "tarih", "seans_tarihi"),
		Close:           closeValue,
		IsFinalClose:    parseBoolDefault(get("is_final_close", "final", "resmi", "kesin"), true),
		SourceTimestamp: sourceTimestamp,
		FetchedAt:       fetchedAt,
	}
	return normalizeOfficialCloseRecord(record, defaultSource, now)
}

func normalizeOfficialCloseRecord(record OfficialCloseRecord, defaultSource string, now time.Time) (OfficialCloseRecord, error) {
	record.Symbol = storage.NormalizeTicker(record.Symbol)
	record.Source = strings.TrimSpace(firstNonEmpty(record.Source, defaultSource))
	record.TradingDate = normalizeTradingDate(record.TradingDate)
	if record.Symbol == "" {
		return OfficialCloseRecord{}, errors.New("symbol is required")
	}
	if record.Source == "" {
		return OfficialCloseRecord{}, errors.New("source is required")
	}
	if record.TradingDate == "" {
		return OfficialCloseRecord{}, errors.New("trading_date is required")
	}
	if record.Close <= 0 {
		return OfficialCloseRecord{}, errors.New("close must be positive")
	}
	if record.SourceTimestamp.IsZero() {
		return OfficialCloseRecord{}, errors.New("source_timestamp is required")
	}
	record.SourceTimestamp = record.SourceTimestamp.UTC()
	if record.FetchedAt.IsZero() {
		record.FetchedAt = now.UTC()
	} else {
		record.FetchedAt = record.FetchedAt.UTC()
	}
	return record, nil
}

func importOfficialCloseSymbol(opts OfficialCloseImportOptions, symbol string, records []OfficialCloseRecord) (OfficialCloseImportSymbolSummary, error) {
	dir := filepath.Join(opts.EquitiesDir, symbol, "price")
	historyPath := filepath.Join(dir, "official_closes.json")
	latestPath := filepath.Join(dir, "official_close.json")
	history := []OfficialCloseRecord{}
	if err := util.ReadJSON(historyPath, &history); err != nil && !errors.Is(err, os.ErrNotExist) {
		return OfficialCloseImportSymbolSummary{}, err
	}
	byKey := map[string]OfficialCloseRecord{}
	for _, record := range history {
		normalized, err := normalizeOfficialCloseRecord(record, opts.Source, time.Now())
		if err != nil {
			continue
		}
		byKey[officialCloseKey(normalized)] = normalized
	}
	for _, record := range records {
		byKey[officialCloseKey(record)] = record
	}
	history = history[:0]
	for _, record := range byKey {
		history = append(history, record)
	}
	sort.Slice(history, func(i, j int) bool {
		if history[i].TradingDate != history[j].TradingDate {
			return history[i].TradingDate < history[j].TradingDate
		}
		return history[i].SourceTimestamp.Before(history[j].SourceTimestamp)
	})
	latest := history[len(history)-1]
	summary := OfficialCloseImportSymbolSummary{
		Symbol:      symbol,
		Records:     len(records),
		LatestDate:  latest.TradingDate,
		LatestClose: latest.Close,
		Path:        filepath.Clean(latestPath),
	}
	if opts.DryRun {
		return summary, nil
	}
	if err := util.WriteJSON(historyPath, history); err != nil {
		return OfficialCloseImportSymbolSummary{}, err
	}
	if err := util.WriteJSON(latestPath, latest); err != nil {
		return OfficialCloseImportSymbolSummary{}, err
	}
	return summary, nil
}

func officialCloseKey(record OfficialCloseRecord) string {
	return strings.Join([]string{record.Symbol, record.TradingDate, record.Source}, "|")
}

func parseCloseValue(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("close is required")
	}
	value = strings.ReplaceAll(value, " ", "")
	if strings.Contains(value, ",") && strings.Contains(value, ".") {
		value = strings.ReplaceAll(value, ".", "")
	}
	value = strings.ReplaceAll(value, ",", ".")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, errors.New("close must be positive")
	}
	return parsed, nil
}

func parseRequiredTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("value is required")
	}
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		loc = time.FixedZone("TRT", 3*60*60)
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		parsed, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q", value)
}

func normalizeTradingDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{"2006-01-02", "02.01.2006", time.RFC3339, time.RFC3339Nano} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return value
}

func parseBoolDefault(value string, fallback bool) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "evet", "resmi", "kesin", "final":
		return true
	case "0", "false", "no", "hayir", "hayır":
		return false
	default:
		return fallback
	}
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"ı", "i",
		"İ", "i",
		"ğ", "g",
		"Ğ", "g",
		"ü", "u",
		"Ü", "u",
		"ş", "s",
		"Ş", "s",
		"ö", "o",
		"Ö", "o",
		"ç", "c",
		"Ç", "c",
		" ", "_",
		"-", "_",
	)
	value = replacer.Replace(value)
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return strings.Trim(value, "_")
}
