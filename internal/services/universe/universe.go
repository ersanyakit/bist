package universe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/repositories"
)

var invalidTickerChars = regexp.MustCompile(`[^A-Z0-9._-]+`)

type Entry struct {
	Ticker     string     `json:"ticker"`
	Exchange   string     `json:"exchange,omitempty"`
	AssetType  int        `json:"asset_type,omitempty"`
	ListedAt   *time.Time `json:"listed_at,omitempty"`
	DelistedAt *time.Time `json:"delisted_at,omitempty"`
	Source     string     `json:"source,omitempty"`
}

type Snapshot struct {
	Source    string    `json:"source,omitempty"`
	FetchedAt time.Time `json:"fetched_at,omitempty"`
	Entries   []Entry   `json:"entries"`
}

type ValidationReport struct {
	Status                  string   `json:"status"`
	ListedSourceAvailable   bool     `json:"listed_source_available"`
	DelistedSourceAvailable bool     `json:"delisted_source_available"`
	ListedCount             int      `json:"listed_count"`
	DelistedCount           int      `json:"delisted_count"`
	EquitiesChecked         int      `json:"equities_checked"`
	MissingTickers          []string `json:"missing_tickers,omitempty"`
	Warnings                []string `json:"warnings,omitempty"`
}

func Validate(ctx context.Context, cfg config.Config, store repositories.EquityRepository) (ValidationReport, error) {
	report := ValidationReport{Status: "pass"}
	listed, listedOK, err := loadOptional(cfg.UniverseFile)
	if err != nil {
		return report, err
	}
	delisted, delistedOK, err := loadOptional(cfg.DelistedUniverseFile)
	if err != nil {
		return report, err
	}
	report.ListedSourceAvailable = listedOK
	report.DelistedSourceAvailable = delistedOK
	report.ListedCount = len(listed.Entries)
	report.DelistedCount = len(delisted.Entries)
	if !listedOK || len(listed.Entries) == 0 {
		report.Status = "fail"
		report.Warnings = append(report.Warnings, "listed_universe_source_missing")
	}
	if !delistedOK {
		report.Status = "fail"
		report.Warnings = append(report.Warnings, "delisted_universe_source_missing")
	}
	if delistedOK && (len(delisted.Entries) == 0 || strings.Contains(strings.ToLower(delisted.Source), "empty")) {
		report.Status = "fail"
		report.Warnings = append(report.Warnings, "delisted_universe_source_empty_or_placeholder")
	}
	equities, err := store.List()
	if err != nil {
		return report, err
	}
	known := map[string]bool{}
	for _, entry := range append(listed.Entries, delisted.Entries...) {
		ticker := normalizeTicker(entry.Ticker)
		if ticker != "" {
			known[ticker] = true
		}
	}
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		default:
		}
		if equity.AssetType != 0 && equity.AssetType != 2 {
			continue
		}
		ticker := normalizeTicker(equity.Ticker)
		if ticker == "" {
			continue
		}
		report.EquitiesChecked++
		if !known[ticker] {
			report.MissingTickers = append(report.MissingTickers, ticker)
		}
	}
	sort.Strings(report.MissingTickers)
	if len(report.MissingTickers) > 0 {
		report.Status = "fail"
		report.Warnings = append(report.Warnings, "equities_missing_from_survivorship_universe")
	}
	return report, nil
}

func Load(path string) (Snapshot, error) {
	if strings.TrimSpace(path) == "" {
		return Snapshot{}, errors.New("universe path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	return parse(raw)
}

func loadOptional(path string) (Snapshot, bool, error) {
	snapshot, err := Load(path)
	if errors.Is(err, os.ErrNotExist) || strings.TrimSpace(path) == "" {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	return snapshot, true, nil
}

func parse(raw []byte) (Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err == nil && (len(snapshot.Entries) > 0 || snapshot.Source != "" || !snapshot.FetchedAt.IsZero()) {
		normalizeEntries(snapshot.Entries)
		return snapshot, nil
	}
	var entries []Entry
	if err := json.Unmarshal(raw, &entries); err == nil {
		normalizeEntries(entries)
		return Snapshot{Entries: entries}, nil
	}
	var generic []map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return Snapshot{}, err
	}
	entries = make([]Entry, 0, len(generic))
	for _, item := range generic {
		entries = append(entries, entryFromMap(item))
	}
	normalizeEntries(entries)
	return Snapshot{Entries: entries}, nil
}

func normalizeEntries(entries []Entry) {
	for i := range entries {
		entries[i].Ticker = normalizeTicker(entries[i].Ticker)
	}
}

func normalizeTicker(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = invalidTickerChars.ReplaceAllString(value, "")
	return value
}

func entryFromMap(item map[string]any) Entry {
	return Entry{
		Ticker:     firstString(item, "ticker", "symbol", "code", "stockCode"),
		Exchange:   firstString(item, "exchange", "market"),
		AssetType:  firstInt(item, "asset_type", "assetType"),
		ListedAt:   firstTime(item, "listed_at", "listedAt", "listing_date", "listingDate"),
		DelistedAt: firstTime(item, "delisted_at", "delistedAt", "delisting_date", "delistingDate"),
		Source:     firstString(item, "source"),
	}
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case string:
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func firstInt(item map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := item[key].(type) {
		case float64:
			return int(value)
		case int:
			return value
		}
	}
	return 0
}

func firstTime(item map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value, ok := item[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		if ts, ok := parseTime(value); ok {
			return &ts
		}
	}
	return nil
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02", "02.01.2006"} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts.UTC(), true
		}
	}
	return time.Time{}, false
}
