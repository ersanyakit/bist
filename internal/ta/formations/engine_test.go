package formations

import (
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestDetectSwingsUsesConfiguredLookback(t *testing.T) {
	candles := []ohlcv.Candle{
		testCandle(0, 10, 11, 9, 10),
		testCandle(1, 10, 12, 9.5, 11),
		testCandle(2, 11, 15, 10, 14),
		testCandle(3, 14, 13, 8, 9),
		testCandle(4, 9, 12, 9, 10),
		testCandle(5, 10, 11, 9.5, 10.5),
	}
	swings := DetectSwings(candles, 2)
	if len(swings) != 2 {
		t.Fatalf("DetectSwings() len = %d, want 2: %+v", len(swings), swings)
	}
	if swings[0].Kind != "high" || swings[0].Index != 2 {
		t.Fatalf("first swing = %+v, want high at index 2", swings[0])
	}
	if swings[1].Kind != "low" || swings[1].Index != 3 {
		t.Fatalf("second swing = %+v, want low at index 3", swings[1])
	}
}

// TestBestWedgeTrendlinePairEvaluatesLinesAtSharedIndex constructs a resistance and a
// support trendline that each start at a different bar. Compared at their own,
// unrelated start bars (the previous, buggy behavior: upper.Start.Price -
// lower.Start.Price = 200 - 180 = +20, which would have been wrongly accepted as a
// valid 20-unit-wide wedge), the pair looks fine. But resistance actually declines
// under support by the time support's own line begins (evaluated at a shared index:
// resistance is 160 there, support is 180 — support above resistance, a geometrically
// invalid pair). The fix must evaluate both lines at a common bar and reject this pair.
func TestBestWedgeTrendlinePairEvaluatesLinesAtSharedIndex(t *testing.T) {
	candles := make([]ohlcv.Candle, 150)
	for i := range candles {
		candles[i] = testCandle(i, 100, 105, 95, 100)
	}

	upper := trendLineCandidate{
		line: TrendlineResult{
			Type:       "resistance_trendline",
			Slope:      -0.5,
			TouchCount: 3,
			Strength:   0.5,
			Start:      TimePrice{Price: 200},
			End:        TimePrice{Price: 130},
		},
		startIdx:   0,
		endIdx:     120,
		startPrice: 200,
		endPrice:   130,
	}
	lower := trendLineCandidate{
		line: TrendlineResult{
			Type:       "support_trendline",
			Slope:      0.05,
			TouchCount: 3,
			Strength:   0.5,
			Start:      TimePrice{Price: 180},
			End:        TimePrice{Price: 125},
		},
		startIdx:   80,
		endIdx:     120,
		startPrice: 180,
		endPrice:   125,
	}

	if _, _, ok := bestWedgeTrendlinePair(candles, []trendLineCandidate{upper, lower}); ok {
		t.Fatal("bestWedgeTrendlinePair() accepted a pair where support is above resistance at their shared comparison bar")
	}
}

// TestDetectTriangleTargetsUseAlignedHeight mirrors the wedge case for the triangle
// measured-move target: detectTriangle used to overwrite the (correctly aligned) height
// from patternFromLines with math.Abs(upper.Start.Price - lower.Start.Price), taken at
// each line's own unrelated start bar. Here the two candidates start 80 bars apart, so
// that raw subtraction (200 - 90 = 110) would produce a wildly inflated target vs. the
// aligned width at the shared start bar (160 - 90 = 70).
func TestDetectTriangleTargetsUseAlignedHeight(t *testing.T) {
	candles := make([]ohlcv.Candle, 150)
	for i := range candles {
		candles[i] = testCandle(i, 100, 105, 95, 100)
	}

	upper := trendLineCandidate{
		line: TrendlineResult{
			Type:       "resistance_trendline",
			Slope:      -0.5,
			TouchCount: 4,
			Strength:   0.6,
			Start:      TimePrice{Price: 200},
			End:        TimePrice{Price: 130},
		},
		startIdx:   0,
		endIdx:     120,
		startPrice: 200,
		endPrice:   130,
	}
	lower := trendLineCandidate{
		line: TrendlineResult{
			Type:       "support_trendline",
			Slope:      0,
			TouchCount: 4,
			Strength:   0.6,
			Start:      TimePrice{Price: 90},
			End:        TimePrice{Price: 100},
		},
		startIdx:   80,
		endIdx:     120,
		startPrice: 90,
		endPrice:   100,
	}

	result, ok := detectTriangle(candles, nil, nil, []trendLineCandidate{upper, lower}, 2)
	if !ok {
		t.Fatal("detectTriangle() did not accept a valid descending-triangle pair")
	}
	// Aligned width at the shared start bar (idx 80): upper = 200 + (-0.5)*80 = 160,
	// lower = 90 (its own start). height = 160 - 90 = 70.
	wantTarget := round2(130 + 70.0)
	if len(result.Targets) != 2 || result.Targets[1] != wantTarget {
		t.Fatalf("Targets = %v, want second target %.2f (aligned height 70, not the unaligned Start.Price gap of 110)", result.Targets, wantTarget)
	}
}

// TestClusterLevelsBoundsClusterWidthToSeedTolerance guards against unbounded
// transitive drift: membership must be tested against each cluster's original seed
// price, not its continuously-updated running mean. With atr=10 (levelTolerance
// dominated by the atr*0.65=6.5 term, independent of price) and points stepped by 3
// (100,103,106,109,112,115,118,121), comparing to a drifting mean chains all 8 points
// spanning a run of higher lows into 2 clusters each spanning 9 (well past the 6.5
// tolerance) — comparing to the fixed seed instead correctly splits them into 3
// clusters, each spanning at most 6.
func TestClusterLevelsBoundsClusterWidthToSeedTolerance(t *testing.T) {
	points := make([]SwingPoint, 8)
	for i := range points {
		points[i] = SwingPoint{Index: i, Price: 100 + float64(i)*3, Kind: "low"}
	}

	levels := clusterLevels(nil, points, "horizontal_support", 10)

	if len(levels) != 3 {
		t.Fatalf("clusterLevels() produced %d levels, want 3 (got %+v) — unbounded drift would collapse this into 2", len(levels), levels)
	}
	total := 0
	for _, level := range levels {
		if level.TouchCount > 3 {
			t.Fatalf("level %+v has TouchCount %d, want <=3 — a cluster spanning more than the tolerance chained together via drift", level, level.TouchCount)
		}
		total += level.TouchCount
	}
	if total != len(points) {
		t.Fatalf("levels account for %d touches, want %d (all points)", total, len(points))
	}
}

// TestTrendlineTouchStatsDoesNotDoubleCountVolumeConfirmedTouches guards against
// volume-confirmed touches inflating the reported touch count itself (it should only
// inflate the separate weighted score used for strength).
func TestTrendlineTouchStatsDoesNotDoubleCountVolumeConfirmedTouches(t *testing.T) {
	// Flat support line at price 100. Bars 2 and 4 touch the line (low=100); bar 2 does
	// so on unusually high volume (5000, > 1.5x the ~2200 average across all 5 bars).
	candles := []ohlcv.Candle{
		testCandle(0, 105, 106, 104, 105),
		testCandle(1, 105, 106, 104, 105),
		testCandle(2, 105, 106, 100, 105),
		testCandle(3, 105, 106, 104, 105),
		testCandle(4, 105, 106, 100, 105),
	}
	candles[2].Volume = 5000 // volume-confirmed touch

	touches, weightedTouches, _ := trendlineTouchStats(candles, 0, 100, 0, "support_trendline", 0)

	if touches != 2 {
		t.Fatalf("touches = %d, want 2 (bars 2 and 4 touch the low=100 line; touch count must not double-count the volume-confirmed one)", touches)
	}
	if weightedTouches != 3 {
		t.Fatalf("weightedTouches = %.1f, want 3 (2 plain touches + 1 extra for the volume-confirmed touch)", weightedTouches)
	}
}

// TestBuildLevelsTruncatesAfterReclassificationNotBefore guards against MaxLevels being
// applied to the same-kind (swing-high) source list before
// normalizeHorizontalLevelsByPrice buckets each cluster as support/resistance relative
// to current price. Four swing-high points (60, 70, 200, 210) with current price 65:
// after reclassification, 60 becomes support and {70, 200, 210} all remain resistance
// (3 valid candidates) — with MaxLevels=2, the final resistances list should be filled
// to 2 from those 3. Truncating the swing-high list to 2 *before* reclassification (the
// previous behavior) would keep only the two lowest-priced points {60, 70} in that
// intermediate list, permanently discarding 200 and 210, and leave only 1 real
// resistance ({70}) once 60 gets reclassified into support.
func TestBuildLevelsTruncatesAfterReclassificationNotBefore(t *testing.T) {
	candles := make([]ohlcv.Candle, 60)
	for i := range candles {
		candles[i] = testCandle(i, 65, 66, 64, 65)
	}
	swings := []SwingPoint{
		{Index: 10, Price: 60, Kind: "high"},
		{Index: 20, Price: 70, Kind: "high"},
		{Index: 30, Price: 200, Kind: "high"},
		{Index: 40, Price: 210, Kind: "high"},
	}
	opts := Options{LevelLookback: 60, MaxLevels: 2}

	_, resistances := buildLevels(candles, swings, 0, opts)

	if len(resistances) != 2 {
		t.Fatalf("resistances = %+v (len %d), want 2 — truncation before reclassification would drop valid resistance candidates", resistances, len(resistances))
	}
}

// TestDetectSwingsBreaksTiesByFirstOccurrence guards against a flat-top plateau (two
// adjacent bars with an identical high, common on coarse tick sizes) disqualifying
// every bar in it: each bar in the tie used to see the other as ">=", so neither
// qualified as a swing high and the real local top was silently dropped. The fix picks
// the first (earlier) occurrence as the pivot.
func TestDetectSwingsBreaksTiesByFirstOccurrence(t *testing.T) {
	candles := []ohlcv.Candle{
		testCandle(0, 10, 10, 8, 9),
		testCandle(1, 10, 10, 8, 9),
		testCandle(2, 15, 20, 8, 15), // plateau start
		testCandle(3, 15, 20, 8, 15), // ties index 2's high
		testCandle(4, 10, 10, 8, 9),
		testCandle(5, 10, 10, 8, 9),
		testCandle(6, 10, 10, 8, 9),
	}

	swings := DetectSwings(candles, 2)

	foundAt2, foundAt3 := false, false
	for _, s := range swings {
		if s.Kind != "high" {
			continue
		}
		if s.Index == 2 {
			foundAt2 = true
		}
		if s.Index == 3 {
			foundAt3 = true
		}
	}
	if !foundAt2 {
		t.Fatalf("expected a swing high at the first occurrence of the tied plateau (index 2), got %+v", swings)
	}
	if foundAt3 {
		t.Fatalf("did not expect a swing high at the second (tied) occurrence (index 3), got %+v", swings)
	}
}

func TestAnalyzeDetectsHorizontalChannelAndFakeBreakdown(t *testing.T) {
	candles := horizontalChannelCandles()
	result, err := Analyze(candles, Options{Symbol: "TEST", Timeframe: "1D", PivotLookback: 2, LevelLookback: 80})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(result.SupportResistance.Supports) == 0 {
		t.Fatal("expected support levels")
	}
	if len(result.SupportResistance.Resistances) == 0 {
		t.Fatal("expected resistance levels")
	}
	if !hasPattern(result.Patterns, "horizontal_channel") {
		t.Fatalf("expected horizontal_channel pattern, got %+v", result.Patterns)
	}
	if result.BreakoutAnalysis.Status != "fake_breakdown" {
		t.Fatalf("breakout status = %q, want fake_breakdown", result.BreakoutAnalysis.Status)
	}
	if len(result.DrawingObjects.Lines) == 0 || len(result.DrawingObjects.Paths) == 0 {
		t.Fatalf("expected drawable lines and paths, got %+v", result.DrawingObjects)
	}
}

func TestAnalyzeDetectsSymmetricalTriangle(t *testing.T) {
	candles := symmetricalTriangleCandles()
	result, err := Analyze(candles, Options{Symbol: "TRI", Timeframe: "1D", PivotLookback: 2, LevelLookback: 90})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	pattern := findPattern(result.Patterns, "symmetrical_triangle")
	if pattern == nil {
		t.Fatalf("expected symmetrical_triangle, got %+v", result.Patterns)
	}
	if pattern.Confidence <= 0 || pattern.Confidence > 1 {
		t.Fatalf("confidence = %.4f, want in (0,1]", pattern.Confidence)
	}
	if len(result.Trendlines) == 0 {
		t.Fatal("expected trendlines")
	}
}

func TestMovingAverageSignalIsBearishWhenPriceBelowEMAs(t *testing.T) {
	candles := make([]ohlcv.Candle, 70)
	price := 100.0
	for i := range candles {
		price -= 0.5
		candles[i] = testCandle(i, price+0.2, price+1, price-1, price)
	}
	result, err := Analyze(candles, Options{Symbol: "EMA", Timeframe: "1D", PivotLookback: 2})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.MovingAverages.EMA20.Position != "price_below" {
		t.Fatalf("EMA20 position = %q, want price_below", result.MovingAverages.EMA20.Position)
	}
	if result.MovingAverages.Signal != "bearish" {
		t.Fatalf("MA signal = %q, want bearish", result.MovingAverages.Signal)
	}
}

func TestCandleDateUsesMarketSessionDate(t *testing.T) {
	candle := ohlcv.Candle{Time: time.Date(2026, 6, 16, 21, 0, 0, 0, time.UTC)}
	if got := candleDate(candle); got != "2026-06-17" {
		t.Fatalf("candleDate() = %s, want 2026-06-17", got)
	}
}

func TestCollectTouchPointsSpacesDenseTouches(t *testing.T) {
	candles := make([]ohlcv.Candle, 120)
	start := time.Date(2026, 1, 1, 21, 0, 0, 0, time.UTC)
	for i := range candles {
		price := 100 + float64(i)*0.2
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   price,
			High:   price + 0.3,
			Low:    price,
			Close:  price + 0.1,
			Volume: 1000,
		}
	}
	points := collectTouchPoints(candles, 0, 100, 0.2, "support_trendline", 10)
	if len(points) == 0 {
		t.Fatal("expected touch points")
	}
	if len(points) >= 30 {
		t.Fatalf("touch points too dense: %d", len(points))
	}
	if points[0].Time != "2026-01-02" {
		t.Fatalf("first touch date = %s, want market-session date 2026-01-02", points[0].Time)
	}
}

func TestDedupeLinesRemovesSameTrendlineDifferentID(t *testing.T) {
	lines := []LineObject{
		{ID: "ascending_channel_lower", Type: "trendline", StartTime: "2026-01-02", StartPrice: 100, EndTime: "2026-02-01", EndPrice: 110},
		{ID: "support_trendline_1", Type: "trendline", StartTime: "2026-01-02", StartPrice: 100.1, EndTime: "2026-02-01", EndPrice: 110.1},
		{ID: "resistance_trendline_1", Type: "trendline", StartTime: "2026-01-02", StartPrice: 130, EndTime: "2026-02-01", EndPrice: 140},
	}
	got := dedupeLines(lines)
	if len(got) != 2 {
		t.Fatalf("dedupeLines() len = %d, want 2: %+v", len(got), got)
	}
}

func TestNormalizeHorizontalLevelsReclassifiesBrokenSupport(t *testing.T) {
	supports, resistances := normalizeHorizontalLevelsByPrice([]Level{
		{Price: 0.66, TouchCount: 1, Strength: 0.25, Type: "horizontal_support"},
	}, []Level{
		{Price: 0.19, TouchCount: 2, Strength: 0.50, Type: "horizontal_resistance"},
	}, 0.21)
	if len(supports) != 1 || supports[0].Price != 0.19 || supports[0].Type != "horizontal_support" {
		t.Fatalf("support below price not normalized: supports=%+v", supports)
	}
	if len(resistances) != 1 || resistances[0].Price != 0.66 || resistances[0].Type != "horizontal_resistance" {
		t.Fatalf("broken support above price should become resistance: resistances=%+v", resistances)
	}
}

func TestDrawScenarioPathsSkipsLongTimeframes(t *testing.T) {
	if drawScenarioPathsForTimeframe("1M") {
		t.Fatal("monthly chart should not draw forward scenario paths")
	}
	if !drawScenarioPathsForTimeframe("1D") {
		t.Fatal("daily chart should keep scenario paths")
	}
}

func TestBuildScenariosSkipsSupportCasesWithoutSupport(t *testing.T) {
	candles := []ohlcv.Candle{
		testCandle(0, 0.24, 0.26, 0.23, 0.24),
		testCandle(1, 0.21, 0.22, 0.20, 0.21),
	}
	scenarios := buildScenarios(candles, 0.21, nil, []Level{{Price: 0.66, Type: "horizontal_resistance"}}, nil, nil, MovingAverages{Signal: "bearish"})
	if len(scenarios) != 1 || scenarios[0].Name != "bullish_breakout" {
		t.Fatalf("desteksiz yapida sadece breakout senaryosu kalmali: %+v", scenarios)
	}
}

func TestTrendlineDisplayReclassifiesBrokenSupportAsResistance(t *testing.T) {
	candidate := trendLineCandidate{
		line:     TrendlineResult{Type: "support_trendline"},
		endPrice: 4438,
	}
	label, color := trendlineDisplay(candidate, 4333, "main_support")
	if color != "red" || label != "Ana trend geri kazanım direnci" {
		t.Fatalf("trendlineDisplay() = %q/%q, want recovery resistance/red", label, color)
	}
}

func horizontalChannelCandles() []ohlcv.Candle {
	closes := []float64{
		100, 104, 108, 111, 113, 110, 106, 102, 99,
		103, 108, 112, 114, 111, 107, 103, 98,
		102, 107, 111, 113, 110, 106, 102, 99,
		103, 108, 112, 114, 111, 107, 103, 98,
		102, 107, 111, 113, 110, 106, 102, 99,
		103, 108, 112, 114, 111, 107, 103, 96, 101,
	}
	out := make([]ohlcv.Candle, len(closes))
	for i, close := range closes {
		high := close + 1.4
		low := close - 1.4
		if close >= 113 {
			high = 116
		}
		if close <= 99 {
			low = 96
		}
		if i == len(closes)-2 {
			low = 93
			close = 96
		}
		if i == len(closes)-1 {
			low = 98
			close = 101
		}
		out[i] = testCandle(i, close, high, low, close)
		out[i].Volume = 1000
		if i == len(closes)-2 {
			out[i].Volume = 3000
		}
	}
	return out
}

func symmetricalTriangleCandles() []ohlcv.Candle {
	out := make([]ohlcv.Candle, 72)
	for i := range out {
		upper := 121 - float64(i)*0.23
		lower := 79 + float64(i)*0.23
		ratio := 0.42 + float64((i*7)%17)/100
		close := lower + (upper-lower)*ratio
		high := close + 0.65 + float64(i%3)*0.08
		low := close - 0.65 - float64(i%4)*0.05
		if high >= upper-0.8 {
			high = upper - 0.8
		}
		if low <= lower+0.8 {
			low = lower + 0.8
		}
		out[i] = testCandle(i, close, high, low, close)
	}
	highs := map[int]float64{12: 120, 26: 115, 40: 111, 54: 108}
	lows := map[int]float64{18: 80, 32: 86, 46: 91, 60: 95}
	for idx, value := range highs {
		out[idx].High = value
		out[idx].Close = value - 3
		out[idx].Open = value - 4
	}
	for idx, value := range lows {
		out[idx].Low = value
		out[idx].Close = value + 3
		out[idx].Open = value + 4
	}
	return out
}

func testCandle(day int, open, high, low, close float64) ohlcv.Candle {
	return ohlcv.Candle{
		Time:   time.Date(2026, 1, 1+day, 0, 0, 0, 0, time.UTC),
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: 1000,
	}
}

func hasPattern(patterns []PatternResult, name string) bool {
	return findPattern(patterns, name) != nil
}

func findPattern(patterns []PatternResult, name string) *PatternResult {
	for i := range patterns {
		if patterns[i].Name == name {
			return &patterns[i]
		}
	}
	return nil
}
