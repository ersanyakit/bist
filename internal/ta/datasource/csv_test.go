package datasource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCSVProviderSearchAndFetchLimit(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "TEST_1D.csv"), `time,open,high,low,close,volume
2026-06-10,10,12,9,11,1000
2026-06-11,11,13,10,12,1200
`)

	provider := NewCSVProvider(dir)
	instrument, err := provider.SearchSymbol(context.Background(), "test")
	if err != nil {
		t.Fatalf("SearchSymbol() error = %v", err)
	}
	if instrument.Symbol != "TEST" {
		t.Fatalf("SearchSymbol() = %#v, want TEST", instrument)
	}

	candles, err := provider.FetchOHLCV(context.Background(), instrument, "1D", 1)
	if err != nil {
		t.Fatalf("FetchOHLCV() error = %v", err)
	}
	if len(candles) != 1 {
		t.Fatalf("FetchOHLCV() returned %d candles, want 1", len(candles))
	}
	if candles[0].Close != 12 || candles[0].Volume != 1200 {
		t.Fatalf("FetchOHLCV() last candle = %#v, want close 12 volume 1200", candles[0])
	}
}

func TestCSVProviderRejectsInvalidCSV(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "BROKEN_1D.csv"), `time,open,high,low,close
2026-06-10,10,12,9,11
`)

	provider := NewCSVProvider(dir)
	instrument, err := provider.SearchSymbol(context.Background(), "BROKEN")
	if err != nil {
		t.Fatalf("SearchSymbol() error = %v", err)
	}
	if instrument.Symbol != "BROKEN" {
		t.Fatalf("SearchSymbol() = %#v, want BROKEN", instrument)
	}

	_, err = provider.FetchOHLCV(context.Background(), instrument, "1D", 10)
	if !errors.Is(err, ErrInvalidCSV) {
		t.Fatalf("FetchOHLCV() error = %v, want ErrInvalidCSV", err)
	}
}

func writeCSV(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
