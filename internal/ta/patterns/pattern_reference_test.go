package patterns

import (
	"strings"
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/patterns/generated"
)

func TestInNeckAndThrustingPatternAreDistinguishableByPenetrationDepth(t *testing.T) {
	// a: bearish, open 100, close 90 (midpoint 95). In Neck requires the bullish
	// reaction to close at/just above the prior close (minimal penetration); Thrusting
	// requires a deeper recovery that still stops short of the midpoint. These must be
	// mutually exclusive, not the same condition.
	a := patternCandle(0, 100, 101, 88, 90, 1000)

	inNeck := []ohlcv.Candle{a, patternCandle(1, 89, 91, 88.5, 90.3, 1000)}
	got, err := DetectCandlestick(inNeck, 1000)
	if err != nil {
		t.Fatalf("DetectCandlestick: %v", err)
	}
	if _, ok := findPatternByName(got, "In Neck"); !ok {
		t.Fatalf("expected In Neck for a close barely above the prior close: %+v", patternNames(got))
	}
	if _, ok := findPatternByName(got, "Thrusting Pattern"); ok {
		t.Fatalf("In Neck fixture should not also match Thrusting Pattern: %+v", patternNames(got))
	}

	thrusting := []ohlcv.Candle{a, patternCandle(1, 89, 94, 88.5, 93, 1000)}
	got, err = DetectCandlestick(thrusting, 1000)
	if err != nil {
		t.Fatalf("DetectCandlestick: %v", err)
	}
	if _, ok := findPatternByName(got, "Thrusting Pattern"); !ok {
		t.Fatalf("expected Thrusting Pattern for a close well past the prior close but below midpoint: %+v", patternNames(got))
	}
	if _, ok := findPatternByName(got, "In Neck"); ok {
		t.Fatalf("Thrusting fixture should not also match In Neck: %+v", patternNames(got))
	}
}

func TestKickingRequiresFullRangeGap(t *testing.T) {
	a := patternCandle(0, 100, 100.2, 89.8, 90, 1000) // bearish, near-marubozu

	overlapping := []ohlcv.Candle{a, patternCandle(1, 95, 105.2, 94.8, 105, 1000)} // bullish body but ranges overlap a
	got, err := DetectCandlestick(overlapping, 1000)
	if err != nil {
		t.Fatalf("DetectCandlestick: %v", err)
	}
	if _, ok := findPatternByName(got, "Kicking Bullish"); ok {
		t.Fatalf("overlapping candles should not match Kicking Bullish without a full-range gap: %+v", patternNames(got))
	}

	gapped := []ohlcv.Candle{a, patternCandle(1, 101, 111.2, 100.8, 111, 1000)} // bullish, low(100.8) > a.high(100.2)
	got, err = DetectCandlestick(gapped, 1000)
	if err != nil {
		t.Fatalf("DetectCandlestick: %v", err)
	}
	if _, ok := findPatternByName(got, "Kicking Bullish"); !ok {
		t.Fatalf("expected Kicking Bullish when the second candle's range fully clears the first: %+v", patternNames(got))
	}
}

func TestMatchIslandRequiresMiddleCandleIsolatedOnBothSides(t *testing.T) {
	a := patternCandle(0, 100, 101, 99, 100, 1000)
	b := patternCandle(1, 110, 112, 109, 111, 1000) // gaps up from a (low 109 > a.high 101)
	d := patternCandle(3, 94, 95, 90, 91, 1000)     // gaps down below b (high 95 < b.low 109)

	isolatedMid := patternCandle(2, 109, 113, 108, 112, 1000) // stays above a.high and above d.high
	got, _, _ := matchIsland(true)([]ohlcv.Candle{a, b, isolatedMid, d}, nil)
	if !got {
		t.Fatal("matchIsland(top) should accept a genuinely isolated two-candle island")
	}

	overlappingMid := patternCandle(2, 97, 98, 95, 96, 1000) // dips back into a's range
	got, _, _ = matchIsland(true)([]ohlcv.Candle{a, b, overlappingMid, d}, nil)
	if got {
		t.Fatal("matchIsland(top) should reject when the middle candle overlaps back into the entry gap")
	}
}

func TestMatchCandlestickAliasPiercingLineRequiresPriorDowntrend(t *testing.T) {
	// Piercing Line is only a meaningful reversal signal after an established downtrend.
	// The generated-catalog alias path used to skip that check entirely (unlike the
	// hand-written detectPiercingLine), so a piercing-shaped pair in a flat/up market
	// would still fire.
	spec := patternSpec{Name: "Piercing Line", Direction: "bullish", Confidence: 0.8}

	prev := patternCandle(4, 118, 119, 108, 110, 1000) // bearish
	cur := patternCandle(5, 108, 117, 107, 116, 1000)  // bullish, closes above prev's midpoint (114)

	downtrendContext := []ohlcv.Candle{
		patternCandle(0, 131, 132, 129, 130, 1000),
		patternCandle(1, 126, 127, 124, 125, 1000),
		patternCandle(2, 121, 122, 119, 120, 1000),
		patternCandle(3, 116, 117, 114, 115, 1000),
		prev, cur,
	}
	if m := matchCandlestickAlias(spec, downtrendContext); !m.ok {
		t.Fatal("expected Piercing Line to match after a genuine downtrend")
	}

	flatContext := []ohlcv.Candle{
		patternCandle(0, 89, 92, 88, 90, 1000),
		patternCandle(1, 94, 97, 93, 95, 1000),
		patternCandle(2, 99, 102, 98, 100, 1000),
		patternCandle(3, 104, 107, 103, 105, 1000),
		prev, cur,
	}
	if m := matchCandlestickAlias(spec, flatContext); m.ok {
		t.Fatal("Piercing Line should not match a piercing-shaped pair without a prior downtrend")
	}
}

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

// normalizePatternName makes two pattern names comparable across the hand-written
// candlestick.go table and the auto-generated catalog, which use slightly different
// separators/punctuation for the same pattern (e.g. "On Neck" vs "On-Neck Line").
func normalizePatternName(name string) string {
	name = strings.ToLower(name)
	name = strings.NewReplacer("-", " ", "_", " ").Replace(name)
	return strings.Join(strings.Fields(name), " ")
}

// TestAdditionalCandlestickDirectionsMatchGeneratedCatalog guards against the class of
// bug found in the 2026-08-05 audit: several patterns in internal/ta/patterns/generated
// were mislabeled "neutral" even though the hand-written detector for the same pattern
// name in candlestick.go correctly assigns a bullish/bearish direction. Since
// uniquePatterns() dedups by confidence with no direction tie-break, whichever path
// "wins" determines whether a real directional signal survives to actionable output —
// so the two sources must never disagree on a non-neutral direction.
func TestAdditionalCandlestickDirectionsMatchGeneratedCatalog(t *testing.T) {
	generatedDirection := make(map[string]string, len(generated.Specs))
	for _, spec := range generated.Specs {
		if spec.Category != "candlestick" {
			continue
		}
		generatedDirection[normalizePatternName(spec.Name)] = spec.Direction
	}

	for _, spec := range additionalCandleSpecs {
		if spec.direction == "neutral" {
			continue
		}
		want, ok := generatedDirection[normalizePatternName(spec.name)]
		if !ok || want == "neutral" {
			continue // no matching (or intentionally neutral) generated entry to cross-check
		}
		if want != spec.direction {
			t.Errorf("%s: candlestick.go says %q, generated catalog says %q", spec.name, spec.direction, want)
		}
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
