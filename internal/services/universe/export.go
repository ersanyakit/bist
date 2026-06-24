package universe

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type ExportReport struct {
	ListedFile    string `json:"listed_file"`
	DelistedFile  string `json:"delisted_file"`
	ListedCount   int    `json:"listed_count"`
	DelistedCount int    `json:"delisted_count"`
	Source        string `json:"source"`
}

func ExportCurrentUniverse(ctx context.Context, cfg config.Config, store *storage.EquityStore) (ExportReport, error) {
	equities, err := store.List()
	if err != nil {
		return ExportReport{}, err
	}
	listedEntries := make([]Entry, 0, len(equities))
	delistedEntries := make([]Entry, 0)
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return ExportReport{}, ctx.Err()
		default:
		}
		if equity.AssetType != 0 && equity.AssetType != 2 {
			continue
		}
		ticker := storage.NormalizeTicker(equity.Ticker)
		if ticker == "" {
			continue
		}
		entry := Entry{
			Ticker:    ticker,
			Exchange:  "BIST",
			AssetType: equity.AssetType,
			Source:    "equity_json_current_snapshot",
		}
		if isNonTradingKAP(equity.KAPInfo) {
			entry.Source = "kap_pay_islem_durumu_0_non_trading"
			delistedEntries = append(delistedEntries, entry)
			continue
		}
		if kapStatus(equity.KAPInfo, "payIslemDurumu") == "1" {
			entry.Source = "kap_pay_islem_durumu_1_current_trading"
		}
		listedEntries = append(listedEntries, entry)
	}
	sortEntries(listedEntries)
	sortEntries(delistedEntries)
	now := time.Now().UTC()
	listed := Snapshot{
		Source:    "kap_pay_islem_durumu_current_snapshot",
		FetchedAt: now,
		Entries:   listedEntries,
	}
	delisted := Snapshot{
		Source:    "kap_pay_islem_durumu_non_trading_snapshot",
		FetchedAt: now,
		Entries:   delistedEntries,
	}
	if err := util.WriteJSON(cfg.UniverseFile, listed); err != nil {
		return ExportReport{}, err
	}
	if err := util.WriteJSON(cfg.DelistedUniverseFile, delisted); err != nil {
		return ExportReport{}, err
	}
	return ExportReport{
		ListedFile:    cfg.UniverseFile,
		DelistedFile:  cfg.DelistedUniverseFile,
		ListedCount:   len(listed.Entries),
		DelistedCount: len(delisted.Entries),
		Source:        listed.Source,
	}, nil
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Ticker < entries[j].Ticker
	})
}

func isNonTradingKAP(info map[string]any) bool {
	if kapStatus(info, "payIslemDurumu") == "0" {
		return true
	}
	value, ok := info["isAllStateInactive"].(bool)
	return ok && value
}

func kapStatus(info map[string]any, key string) string {
	if len(info) == 0 {
		return ""
	}
	value, ok := info[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(toString(typed))
	}
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}
