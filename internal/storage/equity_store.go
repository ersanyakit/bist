package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"hissebot/internal/domain"
)

var safeTickerPattern = regexp.MustCompile(`[^A-Z0-9._-]+`)
var safeDataKeyPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type EquityStore struct {
	root string
	mu   sync.Mutex
}

func NewEquityStore(root string) *EquityStore {
	return &EquityStore{root: root}
}

func (s *EquityStore) Root() string {
	return s.root
}

func (s *EquityStore) Load(ticker string) (*domain.Equity, error) {
	ticker = NormalizeTicker(ticker)
	if ticker == "" {
		return nil, errors.New("empty ticker")
	}

	path := s.Path(ticker)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		bytes, err = os.ReadFile(s.LegacyPath(ticker))
		if errors.Is(err, os.ErrNotExist) {
			return &domain.Equity{
				Ticker:    ticker,
				UpdatedAt: time.Now().UTC(),
			}, nil
		}
	}
	if err != nil {
		return nil, err
	}

	var equity domain.Equity
	if err := json.Unmarshal(bytes, &equity); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if equity.Ticker == "" {
		equity.Ticker = ticker
	}
	return &equity, nil
}

func (s *EquityStore) Save(equity *domain.Equity) error {
	if equity == nil {
		return errors.New("nil equity")
	}
	equity.Ticker = NormalizeTicker(equity.Ticker)
	if equity.Ticker == "" {
		return errors.New("empty ticker")
	}
	equity.UpdatedAt = time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	tickerDir := s.TickerDir(equity.Ticker)
	if err := os.MkdirAll(tickerDir, 0o755); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(equity, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')

	target := s.Path(equity.Ticker)
	temp, err := os.CreateTemp(tickerDir, "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(bytes); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, target)
}

func (s *EquityStore) Update(ticker string, fn func(*domain.Equity) error) error {
	equity, err := s.Load(ticker)
	if err != nil {
		return err
	}
	if err := fn(equity); err != nil {
		return err
	}
	return s.Save(equity)
}

func (s *EquityStore) List() ([]*domain.Equity, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]*domain.Equity, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		var ticker string
		if entry.IsDir() {
			if _, err := os.Stat(filepath.Join(s.root, entry.Name(), "equity.json")); err != nil {
				continue
			}
			ticker = entry.Name()
		} else {
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			ticker = strings.TrimSuffix(entry.Name(), ".json")
		}
		ticker = NormalizeTicker(ticker)
		if ticker == "" || seen[ticker] {
			continue
		}
		seen[ticker] = true
		equity, err := s.Load(ticker)
		if err != nil {
			return nil, err
		}
		out = append(out, equity)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Ticker < out[j].Ticker
	})
	return out, nil
}

func (s *EquityStore) Path(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "equity.json")
}

func (s *EquityStore) LegacyPath(ticker string) string {
	return filepath.Join(s.root, NormalizeTicker(ticker)+".json")
}

func (s *EquityStore) TickerDir(ticker string) string {
	return filepath.Join(s.root, NormalizeTicker(ticker))
}

func (s *EquityStore) ChartPath(ticker string, interval string) string {
	return filepath.Join(s.TickerDir(ticker), "charts", NormalizeInterval(interval)+".json")
}

func (s *EquityStore) OHLCVPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "ohlcv.json")
}

func (s *EquityStore) KAPPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "kap.json")
}

func (s *EquityStore) KAPDisclosuresPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "kap_disclosures.json")
}

func (s *EquityStore) MKKPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "mkk.json")
}

func (s *EquityStore) MKKCompanyInfoPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "mkk_company_info.json")
}

func (s *EquityStore) TradingViewFeedPath(ticker string, key string) string {
	return filepath.Join(s.TickerDir(ticker), "tradingview", NormalizeDataKey(key)+".json")
}

func (s *EquityStore) TradingViewFeedFiles(key string) ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	key = NormalizeDataKey(key)
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(s.root, entry.Name(), "tradingview", key+".json")
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func (s *EquityStore) FinancialPeriodPath(ticker string, year int) string {
	return filepath.Join(s.TickerDir(ticker), "financials", "raw", fmt.Sprintf("%d-12-9-6-3.json", year))
}

func (s *EquityStore) FinancialInfoPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "financials", "bilanco.json")
}

func (s *EquityStore) FinancialCalculationsPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "financials", "bilanco_hesaplari.json")
}

func (s *EquityStore) FinancialStatementVersionsPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "financials", "statement_versions.json")
}

func (s *EquityStore) CommentsPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "comments.json")
}

func (s *EquityStore) NewsSentimentPath(ticker string) string {
	return filepath.Join(s.TickerDir(ticker), "news_sentiment.json")
}

func (s *EquityStore) UnmatchedCommentsPath(target string) string {
	name := NormalizeDataKey(target)
	if name == "" {
		name = "unknown"
	}
	return filepath.Join(s.root, "_unmatched", "comments", name+".json")
}

func (s *EquityStore) MigrateToDirectories() (int, int, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}

	equitiesMoved := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		ticker := strings.TrimSuffix(entry.Name(), ".json")
		equity, err := s.Load(ticker)
		if err != nil {
			return equitiesMoved, 0, err
		}
		if err := s.Save(equity); err != nil {
			return equitiesMoved, 0, err
		}
		if err := os.Remove(s.LegacyPath(ticker)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return equitiesMoved, 0, err
		}
		equitiesMoved++
	}

	chartsMoved, err := s.migrateLegacyCharts(filepath.Join(filepath.Dir(s.root), "charts"))
	if err != nil {
		return equitiesMoved, chartsMoved, err
	}
	if err := s.MigratePerEquityFiles(filepath.Join(filepath.Dir(s.root), "cache"), filepath.Join(filepath.Dir(s.root), "comments")); err != nil {
		return equitiesMoved, chartsMoved, err
	}
	if err := s.MaterializeEmbeddedDataFiles(); err != nil {
		return equitiesMoved, chartsMoved, err
	}
	if err := s.MigrateFinancialFileNames(); err != nil {
		return equitiesMoved, chartsMoved, err
	}
	if err := s.RefreshChartRefs(); err != nil {
		return equitiesMoved, chartsMoved, err
	}
	return equitiesMoved, chartsMoved, nil
}

func (s *EquityStore) MigratePerEquityFiles(cacheRoot string, commentsRoot string) error {
	if err := s.migrateFinancialCache(filepath.Join(cacheRoot, "bilanco")); err != nil {
		return err
	}
	if err := s.migrateLegacyComments(commentsRoot); err != nil {
		return err
	}
	return s.migrateTradingViewScannerCache(filepath.Join(cacheRoot, "tradingview"))
}

func (s *EquityStore) MaterializeEmbeddedDataFiles() error {
	equities, err := s.List()
	if err != nil {
		return err
	}
	for _, equity := range equities {
		if equity.OHLCV != nil {
			if err := writeJSONFile(s.OHLCVPath(equity.Ticker), equity.OHLCV); err != nil {
				return err
			}
		}
		if len(equity.KAPInfo) > 0 {
			if err := writeJSONFile(s.KAPPath(equity.Ticker), equity.KAPInfo); err != nil {
				return err
			}
		} else {
			if err := writeJSONFile(s.KAPPath(equity.Ticker), missingDataFile("kap", equity.Ticker)); err != nil {
				return err
			}
		}
		if equity.MKKID != 0 {
			mkk := map[string]any{
				"source":    "mkk",
				"ticker":    equity.Ticker,
				"mkk_id":    equity.MKKID,
				"name":      equity.Name,
				"available": equity.MKKID > 0,
			}
			if err := writeJSONFile(s.MKKPath(equity.Ticker), mkk); err != nil {
				return err
			}
		} else {
			if err := writeJSONFile(s.MKKPath(equity.Ticker), missingDataFile("mkk", equity.Ticker)); err != nil {
				return err
			}
		}
		if len(equity.CompanyInfo) > 0 {
			if err := writeJSONFile(s.MKKCompanyInfoPath(equity.Ticker), equity.CompanyInfo); err != nil {
				return err
			}
		}
		if len(equity.Comments) > 0 {
			if err := writeJSONFile(s.CommentsPath(equity.Ticker), equity.Comments); err != nil {
				return err
			}
		}
		if equity.BilancoInfo != nil && len(equity.BilancoInfo.Data) > 0 {
			if err := writeJSONFile(s.FinancialInfoPath(equity.Ticker), equity.BilancoInfo); err != nil {
				return err
			}
		} else {
			if err := writeJSONFile(s.FinancialInfoPath(equity.Ticker), missingDataFile("bilanco", equity.Ticker)); err != nil {
				return err
			}
		}
		if len(equity.BilancoCalculations) > 0 {
			if err := writeJSONFile(s.FinancialCalculationsPath(equity.Ticker), equity.BilancoCalculations); err != nil {
				return err
			}
		} else {
			if err := writeJSONFile(s.FinancialCalculationsPath(equity.Ticker), missingDataFile("bilanco_hesaplari", equity.Ticker)); err != nil {
				return err
			}
		}
		if len(equity.RawTradingViewByFeed) > 0 {
			for key, raw := range equity.RawTradingViewByFeed {
				if len(raw) == 0 {
					continue
				}
				path := s.TradingViewFeedPath(equity.Ticker, key)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func missingDataFile(source string, ticker string) map[string]any {
	return map[string]any{
		"source":     source,
		"ticker":     ticker,
		"available":  false,
		"updated_at": time.Now().UTC(),
	}
}

func (s *EquityStore) MigrateFinancialFileNames() error {
	equities, err := s.List()
	if err != nil {
		return err
	}
	for _, equity := range equities {
		oldInfo := filepath.Join(s.TickerDir(equity.Ticker), "financials", "bilanco_info.json")
		if _, err := os.Stat(oldInfo); err == nil {
			if err := moveFile(oldInfo, s.FinancialInfoPath(equity.Ticker)); err != nil {
				return err
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}

		oldCalculations := filepath.Join(s.TickerDir(equity.Ticker), "financials", "calculations.json")
		if _, err := os.Stat(oldCalculations); err == nil {
			if err := moveFile(oldCalculations, s.FinancialCalculationsPath(equity.Ticker)); err != nil {
				return err
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *EquityStore) RefreshChartRefs() error {
	equities, err := s.List()
	if err != nil {
		return err
	}
	for _, equity := range equities {
		chartsDir := filepath.Join(s.TickerDir(equity.Ticker), "charts")
		entries, err := os.ReadDir(chartsDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		refs := map[string]domain.ChartDataRef{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			interval := NormalizeInterval(strings.TrimSuffix(entry.Name(), ".json"))
			if interval == "" {
				continue
			}
			ref, err := chartRefFromFile(s.ChartPath(equity.Ticker, interval), interval)
			if err != nil {
				return err
			}
			refs[interval] = ref
		}
		if len(refs) == 0 {
			continue
		}
		if equity.ChartData == nil {
			equity.ChartData = map[string]domain.ChartDataRef{}
		}
		for interval, ref := range refs {
			equity.ChartData[interval] = ref
		}
		if err := s.Save(equity); err != nil {
			return err
		}
	}
	return nil
}

func chartRefFromFile(path string, interval string) (domain.ChartDataRef, error) {
	var chart struct {
		Source    string    `json:"source"`
		Interval  string    `json:"interval"`
		FetchedAt time.Time `json:"fetched_at"`
		Candles   []struct {
			Time int64 `json:"time"`
		} `json:"candles"`
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return domain.ChartDataRef{}, err
	}
	if err := json.Unmarshal(bytes, &chart); err != nil {
		return domain.ChartDataRef{}, err
	}
	if chart.Source == "" {
		chart.Source = "tradingview"
	}
	if chart.Interval != "" {
		interval = NormalizeInterval(chart.Interval)
	}
	var first *int64
	var last *int64
	if len(chart.Candles) > 0 {
		f := chart.Candles[0].Time
		l := chart.Candles[len(chart.Candles)-1].Time
		first = &f
		last = &l
	}
	return domain.ChartDataRef{
		Source:    chart.Source,
		Interval:  interval,
		Path:      path,
		Bars:      len(chart.Candles),
		FirstTime: first,
		LastTime:  last,
		FetchedAt: chart.FetchedAt,
	}, nil
}

func (s *EquityStore) migrateLegacyCharts(chartsRoot string) (int, error) {
	entries, err := os.ReadDir(chartsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	moved := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ticker := NormalizeTicker(entry.Name())
		if ticker == "" {
			continue
		}
		files, err := os.ReadDir(filepath.Join(chartsRoot, entry.Name()))
		if err != nil {
			return moved, err
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			interval := strings.TrimSuffix(file.Name(), ".json")
			source := filepath.Join(chartsRoot, entry.Name(), file.Name())
			target := s.ChartPath(ticker, interval)
			if err := moveFile(source, target); err != nil {
				return moved, err
			}
			moved++
		}
		_ = os.Remove(filepath.Join(chartsRoot, entry.Name()))
	}
	_ = os.Remove(chartsRoot)
	return moved, nil
}

func (s *EquityStore) migrateFinancialCache(root string) error {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ticker := NormalizeTicker(entry.Name())
		if ticker == "" {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, entry.Name()))
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			source := filepath.Join(root, entry.Name(), file.Name())
			target := filepath.Join(s.TickerDir(ticker), "financials", "raw", file.Name())
			if err := moveFile(source, target); err != nil {
				return err
			}
		}
		_ = os.Remove(filepath.Join(root, entry.Name()))
	}
	_ = os.Remove(root)
	return nil
}

func (s *EquityStore) migrateLegacyComments(root string) error {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		target := strings.TrimSuffix(entry.Name(), ".json")
		source := filepath.Join(root, entry.Name())
		equityTicker, ok, err := s.findTickerByPair(target)
		if err != nil {
			return err
		}
		var destination string
		if ok {
			destination = s.CommentsPath(equityTicker)
		} else {
			destination = s.UnmatchedCommentsPath(target)
		}
		if err := moveFile(source, destination); err != nil {
			return err
		}
	}
	_ = os.Remove(root)
	return nil
}

func (s *EquityStore) migrateTradingViewScannerCache(root string) error {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		source := filepath.Join(root, entry.Name())
		if NormalizeDataKey(key) == "ohlcv" {
			if err := s.splitLegacyOHLCVCache(source, key); err != nil {
				return err
			}
			if err := os.Remove(source); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		target := filepath.Join(s.root, "_market", "tradingview", NormalizeDataKey(key)+".json")
		if err := moveFile(source, target); err != nil {
			return err
		}
	}
	_ = os.Remove(root)
	return nil
}

func (s *EquityStore) splitLegacyOHLCVCache(source string, key string) error {
	var response struct {
		Data []struct {
			S string `json:"s"`
			D []any  `json:"d"`
		} `json:"data"`
	}
	bytes, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil
	}
	for _, item := range response.Data {
		ticker := NormalizeTicker(strings.TrimPrefix(item.S, "BIST:"))
		if ticker == "" {
			continue
		}
		record := map[string]any{
			"source":      "tradingview",
			"key":         NormalizeDataKey(key),
			"symbol":      item.S,
			"values":      item.D,
			"migrated_at": time.Now().UTC(),
		}
		if err := writeJSONFile(s.TradingViewFeedPath(ticker, key), record); err != nil {
			return err
		}
	}
	return nil
}

func (s *EquityStore) findTickerByPair(pair string) (string, bool, error) {
	equities, err := s.List()
	if err != nil {
		return "", false, err
	}
	for _, equity := range equities {
		if equity.Pair == pair {
			return equity.Ticker, true, nil
		}
	}
	return "", false, nil
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return os.WriteFile(path, bytes, 0o644)
}

func moveFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Chmod(0o644); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(source)
}

func NormalizeTicker(ticker string) string {
	ticker = strings.TrimSpace(strings.ToUpper(ticker))
	ticker = strings.TrimPrefix(ticker, "BIST:")
	ticker = safeTickerPattern.ReplaceAllString(ticker, "_")
	return strings.Trim(ticker, "_")
}

func NormalizeInterval(interval string) string {
	interval = strings.TrimSpace(strings.ToUpper(interval))
	interval = safeTickerPattern.ReplaceAllString(interval, "_")
	return strings.Trim(interval, "_")
}

func NormalizeDataKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = safeDataKeyPattern.ReplaceAllString(key, "_")
	return strings.Trim(key, "_")
}
