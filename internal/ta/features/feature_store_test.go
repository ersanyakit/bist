package features

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFeatureBuilderUsesOnlyAsOfBars(t *testing.T) {
	root := t.TempDir()
	chartDir := filepath.Join(root, "ASELS", "charts")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"source":"test","ticker":"ASELS","interval":"D","candles":[
		{"time":1765756800,"open":10,"high":11,"low":9,"close":10,"volume":1000},
		{"time":1765843200,"open":10,"high":12,"low":9,"close":11,"volume":1100},
		{"time":1765929600,"open":99,"high":100,"low":98,"close":99,"volume":1200}
	]}`
	if err := os.WriteFile(filepath.Join(chartDir, "D.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(StoreOptions{Root: root, FeatureSetVersion: DefaultFeatureSetVersion, PreferAdjusted: true})
	asOf, _ := time.Parse("2006-01-02", "2025-12-16")
	fv, bars, err := store.Build(context.Background(), "ASELS", asOf, "1d")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(bars) != 2 {
		t.Fatalf("bars=%d, want 2", len(bars))
	}
	if got := fv.Values["last_close"]; got != 11 {
		t.Fatalf("last_close=%v, want point-in-time close 11", got)
	}
	if !contains(fv.Quality.SuspectFields, "adjusted_chart_missing_using_raw_ohlcv") {
		t.Fatalf("raw fallback warning missing: %+v", fv.Quality)
	}
}

func TestLeakageGuardDetectsFutureTimestamp(t *testing.T) {
	asOf, _ := time.Parse("2006-01-02", "2026-06-19")
	fv := EmptyFeatureVector("ASELS", asOf, "1d", DefaultFeatureSetVersion)
	fv.SourceTimestamps["kap"] = asOf.AddDate(0, 0, 1)
	err := GuardFeatureVector(fv)
	if err == nil || !strings.Contains(err.Error(), "future_source_timestamp:kap") {
		t.Fatalf("expected leakage error, got %v", err)
	}
}

func TestBISTTickRounding(t *testing.T) {
	if got := RoundToTick(411.37, BISTTickSize(411.37)); fmt.Sprintf("%.2f", got) != "411.25" {
		t.Fatalf("rounded=%v", got)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
