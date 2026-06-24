package chart

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"hissebot/internal/ta/formations"
	"hissebot/internal/ta/ohlcv"
)

func TestPNGRendererProducesDecodableImage(t *testing.T) {
	candles := make([]ohlcv.Candle, 80)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	price := 40.0
	for i := range candles {
		open := price
		closePrice := open + float64((i%7)-3)*0.18 + 0.08
		high := maxFloat(open, closePrice) + 0.75
		low := minFloat(open, closePrice) - 0.65
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 1000000 + float64(i*25000),
		}
		price = closePrice
	}

	renderer := NewPNGRenderer()
	content, err := renderer.RenderPNG(context.Background(), RenderInput{
		Symbol:    "ADEL",
		Timeframe: "1D",
		Candles:   candles,
		Indicators: ohlcv.IndicatorSnapshot{
			RSI14: 54.3,
			ATR14: 1.8,
		},
		Levels: []ohlcv.SupportResistanceLevel{
			{Type: "support", Price: 39.2, Strength: 0.72, TouchCount: 3},
			{Type: "resistance", Price: 45.6, Strength: 0.65, TouchCount: 2},
		},
		TradePlan: ohlcv.TradePlan{
			Direction:       "long",
			EntryMin:        42.1,
			EntryMax:        42.9,
			TakeProfit1:     45.6,
			TakeProfit2:     48.2,
			StopLoss:        39.7,
			RiskRewardRatio: 2.1,
			Quality:         "medium",
		},
	})
	if err != nil {
		t.Fatalf("render png: %v", err)
	}
	if len(content) < 10000 {
		t.Fatalf("png too small: %d bytes", len(content))
	}
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		t.Fatalf("decode png: %v", err)
	}
}

func TestDecisionRendererProducesDecodableImage(t *testing.T) {
	renderer := NewDecisionRenderer()
	content, err := renderer.RenderPNG(context.Background(), DecisionRenderInput{
		Symbol:         "ADEL",
		CompanyName:    "Adel Kalemcilik",
		AnalysisDate:   "2026-06-07",
		Timeframe:      "1D",
		LastClose:      42.8,
		LastVolume:     1800000,
		OverallScore:   68,
		OverallBias:    "bullish",
		TimeframeScore: 71,
		TrendBias:      "bullish",
		Indicators: ohlcv.IndicatorSnapshot{
			SMA20:              40.2,
			SMA50:              39.4,
			SMA100:             38.1,
			SMA200:             35.7,
			EMA20:              40.8,
			EMA50:              39.9,
			RSI14:              57.4,
			MACD:               0.9,
			MACDSignal:         0.6,
			MACDHistogram:      0.3,
			ADX14:              22,
			VolumeSMA20:        1200000,
			ChaikinMoneyFlow20: 0.12,
			StochasticK:        62,
			StochasticD:        54,
			ATR14:              1.6,
		},
		Patterns: []ohlcv.PatternResult{
			{Name: "Bullish Engulfing", Direction: "bullish", Confidence: 0.68, VolumeConfirmed: true},
		},
		SupportLevels: []ohlcv.SupportResistanceLevel{
			{Type: "support", Price: 40.2, Strength: 0.72, TouchCount: 3},
		},
		ResistanceLevels: []ohlcv.SupportResistanceLevel{
			{Type: "resistance", Price: 46.4, Strength: 0.65, TouchCount: 2},
			{Type: "resistance", Price: 49.8, Strength: 0.58, TouchCount: 1},
		},
		NearestSupport:    &ohlcv.SupportResistanceLevel{Type: "support", Price: 40.2, Strength: 0.72, TouchCount: 3},
		NearestResistance: &ohlcv.SupportResistanceLevel{Type: "resistance", Price: 46.4, Strength: 0.65, TouchCount: 2},
		TradePlan: ohlcv.TradePlan{
			Direction:       "long",
			EntryMin:        42.1,
			EntryMax:        43.2,
			TakeProfit1:     46.4,
			TakeProfit2:     49.8,
			StopLoss:        39.5,
			RiskRewardRatio: 1.9,
			Quality:         "medium",
		},
		Disclaimer: ohlcv.Disclaimer,
	})
	if err != nil {
		t.Fatalf("render decision png: %v", err)
	}
	if len(content) < 10000 {
		t.Fatalf("decision png too small: %d bytes", len(content))
	}
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		t.Fatalf("decode decision png: %v", err)
	}
}

func TestPlaceLevelCalloutAvoidsOverlappingDenseLevels(t *testing.T) {
	layout := chartLayout{
		left:        100,
		top:         100,
		chartWidth:  900,
		priceBottom: 620,
	}
	occupied := []image.Rectangle{}
	for i := 0; i < 12; i++ {
		placement := levelCalloutPlacement{
			Target:     image.Pt(720, 260+i*5),
			PreferredX: 530,
			PreferredY: 260 + i*5,
		}
		rect := placeLevelCallout(layout, placement, occupied)
		if rect.Min.X < layout.left || rect.Max.X > layout.left+layout.chartWidth {
			t.Fatalf("callout outside horizontal plot bounds: %+v", rect)
		}
		if rect.Min.Y < layout.top || rect.Max.Y > layout.priceBottom {
			t.Fatalf("callout outside vertical plot bounds: %+v", rect)
		}
		for _, other := range occupied {
			if expandedRect(rect, levelCalloutGap).Overlaps(expandedRect(other, levelCalloutGap)) {
				t.Fatalf("callout rectangles overlap: new=%+v other=%+v", rect, other)
			}
		}
		occupied = append(occupied, rect)
	}
}

func TestCandleTimeIndexUsesMarketSessionDate(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 16, 21, 0, 0, 0, time.UTC), Open: 1, High: 2, Low: 1, Close: 2},
	}
	index := candleTimeIndex(candles)
	if got, ok := index["2026-06-17"]; !ok || got != 0 {
		t.Fatalf("market-session date index missing, got idx=%d ok=%v map=%+v", got, ok, index)
	}
}

func TestFutureProjectionSlotsReserveSpaceForScenarioPaths(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC), Open: 10, High: 11, Low: 9, Close: 10},
		{Time: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC), Open: 10, High: 11, Low: 9, Close: 10},
		{Time: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), Open: 10, High: 11, Low: 9, Close: 10},
	}
	drawings := formations.DrawingObjects{
		Paths: []formations.PathObject{
			{
				Label: "bullish_breakout",
				Points: []formations.TimePrice{
					{Time: "2026-06-18", Price: 10},
					{Time: "2026-06-28", Price: 12},
				},
			},
		},
	}
	slots := futureProjectionSlots(candles, drawings, "1D")
	if slots < 24 {
		t.Fatalf("projection slots = %d, want at least 24 for readable scenario space", slots)
	}
	index := candleTimeIndex(candles)
	toX := func(index int) int { return index * 10 }
	lastX, ok := drawingTimeToPlotX(index, candles, "2026-06-18", toX, slots, 24*time.Hour)
	if !ok {
		t.Fatal("last candle x not resolved")
	}
	futureX, ok := drawingTimeToPlotX(index, candles, "2026-06-28", toX, slots, 24*time.Hour)
	if !ok {
		t.Fatal("future scenario x not resolved")
	}
	if futureX <= lastX {
		t.Fatalf("future scenario x should be right of last candle: future=%d last=%d", futureX, lastX)
	}
}

func TestGeometryPathDescriptionExplainsScenarioArrow(t *testing.T) {
	path := formations.PathObject{
		Label: "bullish_support_reaction",
		Points: []formations.TimePrice{
			{Time: "2026-06-18", Price: 5675},
			{Time: "2026-06-28", Price: 6000},
		},
	}
	if got := geometryPathDescription(path); !strings.Contains(got, "Mavi ok") {
		t.Fatalf("path description should mention blue arrow, got %q", got)
	}
	if got := geometryPathValue(path); got != "5675 -> 6000" {
		t.Fatalf("path value = %q", got)
	}
}

func TestParallelFillLinesRejectsNonParallelChannel(t *testing.T) {
	if parallelFillLines(0, 100, 100, 120, 0, 100, 80, 160) {
		t.Fatal("non-parallel channel fill should be rejected")
	}
	if !parallelFillLines(0, 100, 100, 120, 5, 105, 80, 100) {
		t.Fatal("parallel channel fill should be allowed")
	}
}

func TestNormalizeLevelsByPriceReclassifiesMisplacedSupport(t *testing.T) {
	levels := normalizeLevelsByPrice([]ohlcv.SupportResistanceLevel{
		{Type: "support", Price: 4438, Strength: 0.55, TouchCount: 3},
		{Type: "resistance", Price: 4274, Strength: 0.45, TouchCount: 1},
	}, 4333)
	if levels[0].Type != "resistance" {
		t.Fatalf("level above price should be resistance: %+v", levels[0])
	}
	if levels[1].Type != "support" {
		t.Fatalf("level below price should be support: %+v", levels[1])
	}
}

func TestNewPriceAxisUsesLogScaleForWidePositiveRange(t *testing.T) {
	input := RenderInput{
		Candles: []ohlcv.Candle{
			{Open: 0.30, High: 4.94, Low: 0.30, Close: 2.74},
			{Open: 0.22, High: 0.24, Low: 0.15, Close: 0.21},
		},
	}
	axis := newPriceAxis(input)
	if !axis.log {
		t.Fatalf("wide positive crypto range should use log scale: %+v", axis)
	}
	if axis.min <= 0 {
		t.Fatalf("log axis min must stay positive: %+v", axis)
	}
}

func TestPriceAxisLabelsArePlacedOnRightSide(t *testing.T) {
	layout := chartLayout{left: 70, chartWidth: 930}
	x := priceAxisLabelX(layout)
	if x <= layout.left+layout.chartWidth {
		t.Fatalf("price axis label x should be right of plot: x=%d right=%d", x, layout.left+layout.chartWidth)
	}
	if x < layout.left {
		t.Fatalf("price axis label must not be on left side: x=%d left=%d", x, layout.left)
	}
}

func TestChartPatternWindowDrawableRequiresValidCandleWindow(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 6, 18, 6, 0, 0, 0, time.UTC), Open: 10, High: 11, Low: 9, Close: 10.4, Volume: 1000},
		{Time: time.Date(2026, 6, 19, 6, 0, 0, 0, time.UTC), Open: 10.4, High: 12, Low: 10, Close: 11.8, Volume: 1300},
	}
	if !chartPatternWindowDrawable(candles, ohlcv.PatternResult{Name: "bullish_engulfing", StartIndex: 0, EndIndex: 1}) {
		t.Fatal("valid pattern window should be drawable")
	}
	if chartPatternWindowDrawable(candles, ohlcv.PatternResult{Name: "broken_pattern", StartIndex: 1, EndIndex: 4}) {
		t.Fatal("invalid pattern window must not be drawable")
	}
	if chartPatternWindowDrawable(candles, ohlcv.PatternResult{Name: "", StartIndex: 0, EndIndex: 1}) {
		t.Fatal("unnamed pattern must not be drawable")
	}
}

func TestFormatPriceKeepsSmallCryptoLevelsReadable(t *testing.T) {
	if got := formatPrice(0.08474); got != "0.0847" {
		t.Fatalf("formatPrice small DOGE level = %q", got)
	}
	if got := formatPrice(0.00321); got != "0.00321" {
		t.Fatalf("formatPrice sub-cent level = %q", got)
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
