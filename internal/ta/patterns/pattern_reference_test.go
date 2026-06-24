package patterns

import (
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestCandlestickCanonicalFixtures(t *testing.T) {
	tests := []struct {
		name      string
		candles   []ohlcv.Candle
		wantName  string
		direction string
		start     int
		end       int
	}{
		{
			name: "bullish engulfing",
			candles: []ohlcv.Candle{
				patternCandle(0, 10.20, 10.30, 8.80, 9.00, 1200),
				patternCandle(1, 8.90, 10.60, 8.70, 10.40, 1500),
			},
			wantName:  "Bullish Engulfing",
			direction: "bullish",
			start:     0,
			end:       1,
		},
		{
			name: "bearish engulfing",
			candles: []ohlcv.Candle{
				patternCandle(0, 9.00, 10.40, 8.80, 10.20, 1200),
				patternCandle(1, 10.30, 10.50, 8.70, 8.90, 1500),
			},
			wantName:  "Bearish Engulfing",
			direction: "bearish",
			start:     0,
			end:       1,
		},
		{
			name: "doji",
			candles: []ohlcv.Candle{
				patternCandle(0, 10.00, 11.00, 9.00, 10.03, 1500),
			},
			wantName:  "Doji",
			direction: "neutral",
			start:     0,
			end:       0,
		},
	}

	for _, tt := range tests {
		patterns, err := DetectCandlestick(tt.candles, 1000)
		if err != nil {
			t.Fatalf("%s: detect candlestick: %v", tt.name, err)
		}
		got, ok := findPatternByName(patterns, tt.wantName)
		if !ok {
			t.Fatalf("%s: expected %q in results, got %+v", tt.name, tt.wantName, patternNames(patterns))
		}
		if got.Direction != tt.direction || got.StartIndex != tt.start || got.EndIndex != tt.end {
			t.Fatalf("%s: invalid pattern result %+v", tt.name, got)
		}
		if len(got.Evidence) == 0 {
			t.Fatalf("%s: expected evidence for %+v", tt.name, got)
		}
	}
}

func TestCandlestickCanonicalFixturesRejectOppositeEngulfing(t *testing.T) {
	bullish := []ohlcv.Candle{
		patternCandle(0, 10.20, 10.30, 8.80, 9.00, 1200),
		patternCandle(1, 8.90, 10.60, 8.70, 10.40, 1500),
	}
	bearish := []ohlcv.Candle{
		patternCandle(0, 9.00, 10.40, 8.80, 10.20, 1200),
		patternCandle(1, 10.30, 10.50, 8.70, 8.90, 1500),
	}

	bullishPatterns, err := DetectCandlestick(bullish, 1000)
	if err != nil {
		t.Fatalf("bullish fixture: %v", err)
	}
	if _, ok := findPatternByName(bullishPatterns, "Bearish Engulfing"); ok {
		t.Fatalf("bullish engulfing fixture should not match bearish engulfing: %+v", bullishPatterns)
	}

	bearishPatterns, err := DetectCandlestick(bearish, 1000)
	if err != nil {
		t.Fatalf("bearish fixture: %v", err)
	}
	if _, ok := findPatternByName(bearishPatterns, "Bullish Engulfing"); ok {
		t.Fatalf("bearish engulfing fixture should not match bullish engulfing: %+v", bearishPatterns)
	}
}

func TestPatternScanCatalogIsDeterministicForCanonicalFixture(t *testing.T) {
	candles := []ohlcv.Candle{
		patternCandle(0, 10.20, 10.30, 8.80, 9.00, 1200),
		patternCandle(1, 8.90, 10.60, 8.70, 10.40, 1500),
	}
	scans := scanPatternCatalog(ScannerInput{Candles: candles}, []ohlcv.PatternResult{{
		Name:       "Bullish Engulfing",
		Category:   "candlestick",
		Direction:  "bullish",
		Confidence: 0.84,
		StartIndex: 0,
		EndIndex:   1,
		Evidence:   []string{"canonical bullish engulfing fixture"},
	}})
	if len(scans) == 0 {
		t.Fatal("expected full scan catalog")
	}
	if !scans[0].Matched {
		t.Fatalf("matched pattern should be sorted before unmatched scans, first=%+v", scans[0])
	}
	got, ok := findPatternScanByName(scans, "Bullish Engulfing")
	if !ok {
		t.Fatal("expected Bullish Engulfing scan entry")
	}
	if !got.Matched || got.Direction != "bullish" || got.Confidence != 0.84 {
		t.Fatalf("invalid scan entry: %+v", got)
	}
}

func patternCandle(day int, open, high, low, closePrice, volume float64) ohlcv.Candle {
	return ohlcv.Candle{
		Time:   time.Date(2026, 1, 1+day, 0, 0, 0, 0, time.UTC),
		Open:   open,
		High:   high,
		Low:    low,
		Close:  closePrice,
		Volume: volume,
	}
}

func findPatternByName(patterns []ohlcv.PatternResult, name string) (ohlcv.PatternResult, bool) {
	for _, pattern := range patterns {
		if pattern.Name == name {
			return pattern, true
		}
	}
	return ohlcv.PatternResult{}, false
}

func findPatternScanByName(scans []ohlcv.PatternScanResult, name string) (ohlcv.PatternScanResult, bool) {
	for _, scan := range scans {
		if scan.Name == name {
			return scan, true
		}
	}
	return ohlcv.PatternScanResult{}, false
}

func patternNames(patterns []ohlcv.PatternResult) []string {
	out := make([]string, len(patterns))
	for i, pattern := range patterns {
		out[i] = pattern.Name
	}
	return out
}
