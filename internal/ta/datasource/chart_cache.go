package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"
)

type ChartCacheProvider struct {
	equitiesDir string
}

type chartCacheFile struct {
	Source   string             `json:"source"`
	Ticker   string             `json:"ticker"`
	Symbol   string             `json:"symbol"`
	Interval string             `json:"interval"`
	Candles  []chartCacheCandle `json:"candles"`
}

type chartCacheCandle struct {
	Time   int64    `json:"time"`
	Open   *float64 `json:"open,omitempty"`
	High   *float64 `json:"high,omitempty"`
	Low    *float64 `json:"low,omitempty"`
	Close  *float64 `json:"close,omitempty"`
	Volume *float64 `json:"volume,omitempty"`
}

func NewChartCacheProvider(equitiesDir string) *ChartCacheProvider {
	return &ChartCacheProvider{equitiesDir: equitiesDir}
}

func (p *ChartCacheProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	if err := ctx.Err(); err != nil {
		return ohlcv.Instrument{}, fmt.Errorf("search chart cache canceled: %w", err)
	}
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return ohlcv.Instrument{}, fmt.Errorf("empty symbol: %w", ErrSymbolNotFound)
	}
	if pair, quote, ok := ohlcv.CanonicalCryptoPair(normalized); ok {
		return ohlcv.Instrument{Symbol: pair, Exchange: "BINANCE", CompanyName: ohlcv.CryptoDisplayName(pair), Currency: quote, AssetType: ohlcv.AssetTypeCrypto}, nil
	}
	if instrument, ok := ohlcv.CanonicalCommodityInstrument(normalized); ok {
		return instrument, nil
	}
	return ohlcv.Instrument{Symbol: normalized, Exchange: "BIST", CompanyName: normalized, Currency: "TRY", AssetType: ohlcv.AssetTypeEquity}, nil
}

func (p *ChartCacheProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("read chart cache canceled: %w", err)
	}
	interval, err := chartCacheInterval(timeframe)
	if err != nil {
		return nil, err
	}
	path, raw, err := p.readChartCache(instrument, interval)
	if err != nil {
		return nil, err
	}
	var file chartCacheFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse chart cache %s: %w", path, err)
	}
	candles := make([]ohlcv.Candle, 0, len(file.Candles))
	for _, item := range file.Candles {
		candle, ok := chartCacheOHLCVCandle(item)
		if ok {
			candles = append(candles, candle)
		}
	}
	sort.SliceStable(candles, func(i, j int) bool {
		return candles[i].Time.Before(candles[j].Time)
	})
	if limit > 0 && len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("chart cache %s has no usable candles: %w", path, ErrSymbolNotFound)
	}
	if err := validateChartCacheContinuity(candles, timeframe); err != nil {
		return nil, fmt.Errorf("chart cache %s invalid continuity: %w: %w", path, err, ErrSymbolNotFound)
	}
	return candles, nil
}

func (p *ChartCacheProvider) readChartCache(instrument ohlcv.Instrument, interval string) (string, []byte, error) {
	paths := p.chartCachePaths(instrument, interval)
	var errs []string
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err == nil {
			return path, raw, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", path, err))
	}
	return "", nil, fmt.Errorf("read chart cache candidates failed: %s: %w", strings.Join(errs, "; "), ErrSymbolNotFound)
}

func (p *ChartCacheProvider) chartCachePaths(instrument ohlcv.Instrument, interval string) []string {
	assetRoot := chartCacheAssetRoot(p.equitiesDir, instrument.AssetType)
	symbolKey := ohlcv.SymbolPathKey(instrument.Symbol)
	paths := []string{filepath.Join(assetRoot, symbolKey, "charts", interval+".json")}
	if instrument.AssetType == ohlcv.AssetTypeEquity && chartCacheBenchmarkSymbol(instrument.Symbol) {
		paths = append(paths, filepath.Join(filepath.Dir(p.equitiesDir), "market", "benchmarks", symbolKey, "charts", interval+".json"))
	}
	return uniquePaths(paths)
}

func chartCacheAssetRoot(root string, assetType string) string {
	if filepath.Base(root) == "equities" {
		switch {
		case ohlcv.IsCryptoAssetType(assetType):
			return filepath.Join(filepath.Dir(root), "crypto")
		case ohlcv.IsCommodityAssetType(assetType):
			return filepath.Join(filepath.Dir(root), "commodities")
		}
	}
	return root
}

func chartCacheBenchmarkSymbol(symbol string) bool {
	symbol = ohlcv.NormalizeSymbol(symbol)
	prefixes := []string{"XU", "XT", "XB", "XK", "XM", "XS", "XY", "XH", "XF", "XG", "XE", "XI"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(symbol, prefix) {
			return true
		}
	}
	return false
}

func uniquePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func validateChartCacheContinuity(candles []ohlcv.Candle, timeframe string) error {
	if len(candles) < 2 {
		return nil
	}
	// Ordering/duplicate-timestamp checks must run for every timeframe, independent of
	// whether a gap tolerance is configured below — see the matching comment on
	// validateTimeframeCandleContinuity in internal/ta/analysis/engine.go.
	previous := candles[0].Time
	for i := 1; i < len(candles); i++ {
		current := candles[i].Time
		if previous.IsZero() || current.IsZero() {
			return fmt.Errorf("%s candle %d has zero timestamp", timeframe, i)
		}
		if !current.After(previous) {
			return fmt.Errorf("%s candles are not strictly increasing at %s then %s", timeframe, previous.Format("2006-01-02"), current.Format("2006-01-02"))
		}
		previous = current
	}
	maxGap := chartCacheMaxAllowedGap(timeframe)
	if maxGap <= 0 {
		return nil
	}
	previous = candles[0].Time
	for i := 1; i < len(candles); i++ {
		current := candles[i].Time
		if gap := current.Sub(previous); gap > maxGap {
			return fmt.Errorf("%s candle temporal gap %s exceeds %s between %s and %s", timeframe, gap, maxGap, previous.Format("2006-01-02"), current.Format("2006-01-02"))
		}
		previous = current
	}
	return nil
}

func chartCacheMaxAllowedGap(timeframe string) time.Duration {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1D", "D":
		return 14 * 24 * time.Hour
	case "1W", "W":
		return 28 * 24 * time.Hour
	case "1M", "M":
		return 75 * 24 * time.Hour
	default:
		return 0
	}
}

func chartCacheInterval(timeframe string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1D":
		return "D", nil
	case "1W":
		return "W", nil
	case "1M":
		return "M", nil
	default:
		return "", fmt.Errorf("chart cache timeframe %q unavailable: %w", timeframe, ErrTimeframe)
	}
}

func chartCacheOHLCVCandle(item chartCacheCandle) (ohlcv.Candle, bool) {
	if item.Time <= 0 || item.Open == nil || item.High == nil || item.Low == nil || item.Close == nil {
		return ohlcv.Candle{}, false
	}
	volume := 0.0
	if item.Volume != nil {
		volume = *item.Volume
	}
	t := time.Unix(item.Time, 0).UTC()
	return ohlcv.Candle{
		Time:           t,
		Open:           *item.Open,
		High:           *item.High,
		Low:            *item.Low,
		Close:          *item.Close,
		Volume:         volume,
		AdjustedOpen:   *item.Open,
		AdjustedHigh:   *item.High,
		AdjustedLow:    *item.Low,
		AdjustedClose:  *item.Close,
		AdjustedVolume: volume,
		IsAdjusted:     true,
	}, true
}
