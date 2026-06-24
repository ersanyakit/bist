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
