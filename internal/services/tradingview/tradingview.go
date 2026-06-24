package tradingview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type RequestSeed struct {
	Key    string         `json:"key"`
	Params map[string]any `json:"params"`
	Asset  int            `json:"asset"`
	Path   string         `json:"path"`
}

type scannerResponse struct {
	Data []scannerItem `json:"data"`
}

type scannerItem struct {
	S string `json:"s"`
	D []any  `json:"d"`
}

type ScannerFeedFile struct {
	Source    string      `json:"source"`
	Key       string      `json:"key"`
	Asset     int         `json:"asset"`
	Path      string      `json:"path"`
	Ticker    string      `json:"ticker"`
	Symbol    string      `json:"symbol"`
	Columns   []string    `json:"columns"`
	Values    []any       `json:"values"`
	FetchedAt time.Time   `json:"fetched_at"`
	Raw       scannerItem `json:"raw"`
}

var ohlcvColumns = []string{
	"name",
	"description",
	"exchange",
	"currency",
	"open",
	"high",
	"low",
	"close",
	"volume",
	"change",
	"change_abs",
	"Value.Traded",
	"time",
}

func Sync(ctx context.Context, cfg config.Config, store *storage.EquityStore, requestFile string, fetch bool, parse bool) error {
	var requests []RequestSeed
	if err := util.ReadJSON(requestFile, &requests); err != nil {
		return err
	}

	if fetch {
		if err := Fetch(ctx, cfg, store, requests); err != nil {
			return err
		}
	}
	if parse {
		return Parse(ctx, cfg, store, requests)
	}
	return nil
}

func SyncOHLCV(ctx context.Context, cfg config.Config, store *storage.EquityStore) error {
	request := RequestSeed{
		Key:   "ohlcv",
		Asset: 2,
		Path:  "turkey",
		Params: map[string]any{
			"columns":               stringSliceToAny(ohlcvColumns),
			"ignore_unknown_fields": false,
			"options": map[string]any{
				"lang": "tr",
			},
			"range": []any{0, 2000},
			"sort": map[string]any{
				"sortBy":     "name",
				"sortOrder":  "asc",
				"nullsFirst": false,
			},
			"preset": "all_stocks",
		},
	}
	if err := Fetch(ctx, cfg, store, []RequestSeed{request}); err != nil {
		return err
	}
	return ParseOHLCV(ctx, cfg, store, request)
}

func Fetch(ctx context.Context, cfg config.Config, store *storage.EquityStore, requests []RequestSeed) error {
	client := &http.Client{Timeout: cfg.HTTPTimeout}

	for _, seed := range requests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		body, err := json.Marshal(seed.Params)
		if err != nil {
			return fmt.Errorf("%s params: %w", seed.Key, err)
		}
		url := "https://scanner.tradingview.com/" + seed.Path + "/scan"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "hissebot-go/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("%s fetch: %w", seed.Key, err)
		}
		data, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("%s fetch: status %d: %s", seed.Key, resp.StatusCode, string(data))
		}

		var response scannerResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return err
		}
		columns := columnsFromParams(seed.Params)
		written := 0
		fetchedAt := time.Now().UTC()
		for _, item := range response.Data {
			ticker := tickerFromItem(seed.Asset, item)
			if ticker == "" {
				continue
			}
			feed := ScannerFeedFile{
				Source:    "tradingview",
				Key:       storage.NormalizeDataKey(seed.Key),
				Asset:     seed.Asset,
				Path:      seed.Path,
				Ticker:    ticker,
				Symbol:    item.S,
				Columns:   columns,
				Values:    item.D,
				FetchedAt: fetchedAt,
				Raw:       item,
			}
			if err := util.WriteJSON(store.TradingViewFeedPath(ticker, seed.Key), feed); err != nil {
				return err
			}
			written++
		}
		fmt.Printf("tradingview: fetched %s (%d ticker files)\n", seed.Key, written)
	}
	return nil
}

func ParseOHLCV(ctx context.Context, cfg config.Config, store *storage.EquityStore, seed RequestSeed) error {
	feeds, err := readScannerFeeds(ctx, cfg, store, seed)
	if err != nil {
		return err
	}

	updated := 0
	fetchedAt := time.Now().UTC()
	for _, feed := range feeds {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ticker := storage.NormalizeTicker(feed.Ticker)
		if ticker == "" {
			continue
		}
		columns := feed.Columns
		if len(columns) == 0 {
			columns = ohlcvColumns
		}
		values := valuesByColumn(columns, feed.Values)
		ohlcv := &domain.OHLCV{
			Source:      "tradingview",
			Symbol:      feed.Symbol,
			Exchange:    stringFromAny(values["exchange"]),
			Currency:    stringFromAny(values["currency"]),
			Open:        floatFromAny(values["open"]),
			High:        floatFromAny(values["high"]),
			Low:         floatFromAny(values["low"]),
			Close:       floatFromAny(values["close"]),
			Volume:      floatFromAny(values["volume"]),
			Change:      floatFromAny(values["change"]),
			ChangeAbs:   floatFromAny(values["change_abs"]),
			ValueTraded: floatFromAny(values["Value.Traded"]),
			Time:        intFromAny(values["time"]),
			FetchedAt:   fetchedAt,
			Raw:         values,
		}
		if err := util.WriteJSON(store.OHLCVPath(ticker), ohlcv); err != nil {
			return err
		}
		err := store.Update(ticker, func(e *domain.Equity) error {
			e.AssetType = 2
			if e.Name == "" {
				e.Name = firstNonEmptyString(stringFromAny(values["description"]), stringFromAny(values["name"]))
			}
			e.OHLCV = ohlcv
			return nil
		})
		if err != nil {
			return err
		}
		updated++
	}

	fmt.Printf("tradingview ohlcv: %d equity folders updated\n", updated)
	return nil
}

func Parse(ctx context.Context, cfg config.Config, store *storage.EquityStore, requests []RequestSeed) error {
	merged := map[string]map[string]any{}
	assetType := map[string]int{}

	for _, seed := range requests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		columns := columnsFromParams(seed.Params)
		if len(columns) == 0 {
			return fmt.Errorf("%s: columns empty", seed.Key)
		}

		feeds, err := readScannerFeeds(ctx, cfg, store, seed)
		if err != nil {
			return err
		}

		for _, feed := range feeds {
			ticker := storage.NormalizeTicker(feed.Ticker)
			if ticker == "" {
				continue
			}
			if merged[ticker] == nil {
				merged[ticker] = map[string]any{}
			}
			assetType[ticker] = seed.Asset

			values := feed.Values
			if len(feed.Columns) > 0 {
				columns = feed.Columns
			}
			for i, value := range values {
				if value == nil || i >= len(columns) {
					continue
				}
				setColumnValue(merged[ticker], columns[i], value)
			}
		}
	}

	for ticker, data := range merged {
		targetTicker := ticker
		if assetType[ticker] == 2 {
			if name, ok := data["name"].(string); ok && strings.TrimSpace(name) != "" {
				targetTicker = storage.NormalizeTicker(name)
			}
		}
		if targetTicker == "" {
			targetTicker = ticker
		}
		err := store.Update(targetTicker, func(e *domain.Equity) error {
			e.AssetType = assetType[ticker]
			e.Data = data
			e.Score = score(data, assetType[ticker])
			return nil
		})
		if err != nil {
			return err
		}
	}

	fmt.Printf("tradingview: %d equity json files updated\n", len(merged))
	return nil
}

func readScannerFeeds(ctx context.Context, cfg config.Config, store *storage.EquityStore, seed RequestSeed) ([]ScannerFeedFile, error) {
	paths, err := store.TradingViewFeedFiles(seed.Key)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return readLegacyScannerFeeds(ctx, cfg, seed)
	}
	feeds := make([]ScannerFeedFile, 0, len(paths))
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var feed ScannerFeedFile
		if err := util.ReadJSON(path, &feed); err != nil {
			return nil, err
		}
		if feed.Key != "" && storage.NormalizeDataKey(feed.Key) != storage.NormalizeDataKey(seed.Key) {
			continue
		}
		if feed.Ticker == "" {
			feed.Ticker = storage.NormalizeTicker(strings.TrimPrefix(feed.Symbol, "BIST:"))
		}
		feeds = append(feeds, feed)
	}
	return feeds, nil
}

func readLegacyScannerFeeds(ctx context.Context, cfg config.Config, seed RequestSeed) ([]ScannerFeedFile, error) {
	var response scannerResponse
	cachePath := filepath.Join(cfg.TradingViewCacheDir, seed.Key+".json")
	if err := util.ReadJSON(cachePath, &response); err != nil {
		return nil, err
	}
	columns := columnsFromParams(seed.Params)
	feeds := make([]ScannerFeedFile, 0, len(response.Data))
	for _, item := range response.Data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		ticker := tickerFromItem(seed.Asset, item)
		if ticker == "" {
			continue
		}
		feeds = append(feeds, ScannerFeedFile{
			Source:    "tradingview",
			Key:       storage.NormalizeDataKey(seed.Key),
			Asset:     seed.Asset,
			Path:      seed.Path,
			Ticker:    ticker,
			Symbol:    item.S,
			Columns:   columns,
			Values:    item.D,
			FetchedAt: time.Now().UTC(),
			Raw:       item,
		})
	}
	return feeds, nil
}

func valuesByColumn(columns []string, values []any) map[string]any {
	out := map[string]any{}
	for i, column := range columns {
		if i >= len(values) {
			break
		}
		out[column] = values[i]
	}
	return out
}

func columnsFromParams(params map[string]any) []string {
	raw, ok := params["columns"].([]any)
	if !ok {
		return nil
	}
	columns := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			columns = append(columns, value)
		}
	}
	return columns
}

func tickerFromItem(asset int, item scannerItem) string {
	if asset == 2 {
		return storage.NormalizeTicker(strings.TrimPrefix(item.S, "BIST:"))
	}
	if len(item.D) == 0 || item.D[0] == nil {
		return ""
	}
	return storage.NormalizeTicker(fmt.Sprint(item.D[0]))
}

func setColumnValue(target map[string]any, column string, value any) {
	prefix, suffix, hasSuffix := strings.Cut(column, "|")
	parts := strings.Split(prefix, ".")
	if hasSuffix {
		parts = append(parts, suffix)
	}
	if len(parts) == 0 {
		return
	}

	current := target
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}

	finalValue := value
	if len(parts) > 1 && !strings.Contains(column, "|") {
		finalValue = map[string]any{"value": value}
	}
	current[parts[len(parts)-1]] = finalValue
}

func score(_ map[string]any, assetType int) float64 {
	if assetType == 2 {
		return 0
	}
	return 0
}

func stringSliceToAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func floatFromAny(value any) *float64 {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		return &v
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case json.Number:
		f, err := v.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil
		}
		return &f
	case string:
		v = strings.TrimSpace(strings.ReplaceAll(v, ",", "."))
		if v == "" {
			return nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil
		}
		return &f
	default:
		return nil
	}
}

func intFromAny(value any) *int64 {
	switch v := value.(type) {
	case float64:
		i := int64(v)
		return &i
	case int:
		i := int64(v)
		return &i
	case int64:
		return &v
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return nil
		}
		return &i
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return nil
		}
		return &i
	default:
		return nil
	}
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func writeRawJSON(path string, data []byte) error {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return os.WriteFile(path, append(data, '\n'), 0o644)
	}
	return util.WriteJSON(path, json.RawMessage(pretty.Bytes()))
}
