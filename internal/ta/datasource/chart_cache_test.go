package datasource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChartCacheProviderReadsCachedWeeklyCandles(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "FENER", "charts")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatalf("mkdir chart dir: %v", err)
	}
	body := `{
  "source": "tradingview",
  "ticker": "FENER",
  "symbol": "BIST_DLY:FENER",
  "interval": "W",
  "candles": [
    {"time": 1716163200, "open": 1.0, "high": 1.4, "low": 0.9, "close": 1.2, "volume": 1000},
    {"time": 1716768000, "open": 1.2, "high": 1.5, "low": 1.1, "close": 1.4, "volume": 1200}
  ]
}`
	if err := os.WriteFile(filepath.Join(chartDir, "W.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write chart cache: %v", err)
	}

	provider := NewChartCacheProvider(dir)
	instrument, err := provider.SearchSymbol(context.Background(), "fener")
	if err != nil {
		t.Fatalf("search symbol: %v", err)
	}
	candles, err := provider.FetchOHLCV(context.Background(), instrument, "1W", 1)
	if err != nil {
		t.Fatalf("fetch ohlcv: %v", err)
	}
	if len(candles) != 1 {
		t.Fatalf("candles len = %d, want 1", len(candles))
	}
	if candles[0].Close != 1.4 || candles[0].AdjustedClose != 1.4 || !candles[0].IsAdjusted {
		t.Fatalf("unexpected candle: %+v", candles[0])
	}
}

func TestChartCacheProviderReadsBenchmarkFromMarketCache(t *testing.T) {
	dir := t.TempDir()
	equitiesDir := filepath.Join(dir, "data", "equities")
	chartDir := filepath.Join(dir, "data", "market", "benchmarks", "XU100", "charts")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatalf("mkdir benchmark chart dir: %v", err)
	}
	body := `{
  "source": "tradingview",
  "ticker": "XU100",
  "symbol": "BIST:XU100",
  "interval": "D",
  "candles": [
    {"time": 1716163200, "open": 100.0, "high": 110.0, "low": 95.0, "close": 105.0, "volume": 1000}
  ]
}`
	if err := os.WriteFile(filepath.Join(chartDir, "D.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write benchmark chart cache: %v", err)
	}

	provider := NewChartCacheProvider(equitiesDir)
	instrument, err := provider.SearchSymbol(context.Background(), "xu100")
	if err != nil {
		t.Fatalf("search symbol: %v", err)
	}
	candles, err := provider.FetchOHLCV(context.Background(), instrument, "1D", 0)
	if err != nil {
		t.Fatalf("fetch benchmark ohlcv: %v", err)
	}
	if len(candles) != 1 || candles[0].Close != 105 {
		t.Fatalf("unexpected benchmark candles: %+v", candles)
	}
}

func TestChartCacheProviderRejectsGappedDailyCache(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "ASELS", "charts")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatalf("mkdir chart dir: %v", err)
	}
	body := `{
  "source": "tradingview",
  "ticker": "ASELS",
  "symbol": "BIST_DLY:ASELS",
  "interval": "D",
  "candles": [
    {"time": 1767830400, "open": 1.0, "high": 1.4, "low": 0.9, "close": 1.2, "volume": 1000},
    {"time": 1780876800, "open": 1.2, "high": 1.5, "low": 1.1, "close": 1.4, "volume": 1200}
  ]
}`
	if err := os.WriteFile(filepath.Join(chartDir, "D.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write chart cache: %v", err)
	}

	provider := NewChartCacheProvider(dir)
	instrument, err := provider.SearchSymbol(context.Background(), "asels")
	if err != nil {
		t.Fatalf("search symbol: %v", err)
	}
	_, err = provider.FetchOHLCV(context.Background(), instrument, "1D", 0)
	if err == nil {
		t.Fatalf("expected gapped daily cache to fail")
	}
	if !errors.Is(err, ErrSymbolNotFound) || !strings.Contains(err.Error(), "invalid continuity") {
		t.Fatalf("expected fallback-compatible invalid continuity error, got %v", err)
	}
}
