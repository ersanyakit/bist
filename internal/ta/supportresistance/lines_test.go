package supportresistance

import (
	"math"
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestAnalyzeFindsNearestLevelsOnCorrectSideOfPrice(t *testing.T) {
	candles := supportResistanceFixture()
	result, err := Analyze(candles, 2, 120)
	if err != nil {
		t.Fatalf("analyze support resistance: %v", err)
	}
	lastClose := candles[len(candles)-1].EffectiveClose()
	if len(result.SupportLevels) == 0 {
		t.Fatal("expected support levels")
	}
	if len(result.ResistanceLevels) == 0 {
		t.Fatal("expected resistance levels")
	}
	if result.NearestSupport == nil {
		t.Fatal("expected nearest support")
	}
	if result.NearestResistance == nil {
		t.Fatal("expected nearest resistance")
	}
	if result.NearestSupport.Price >= lastClose {
		t.Fatalf("nearest support should be below price %.2f, got %.2f", lastClose, result.NearestSupport.Price)
	}
	if result.NearestResistance.Price <= lastClose {
		t.Fatalf("nearest resistance should be above price %.2f, got %.2f", lastClose, result.NearestResistance.Price)
	}
	for _, level := range result.SupportLevels {
		if level.Type != "support" || level.Price <= 0 || level.Strength < 0 || level.Strength > 1 || level.TouchCount <= 0 {
			t.Fatalf("invalid support level: %+v", level)
		}
		if level.Price >= lastClose {
			t.Fatalf("support should be below price %.2f, got %+v", lastClose, level)
		}
	}
	for _, level := range result.ResistanceLevels {
		if level.Type != "resistance" || level.Price <= 0 || level.Strength < 0 || level.Strength > 1 || level.TouchCount <= 0 {
			t.Fatalf("invalid resistance level: %+v", level)
		}
		if level.Price <= lastClose {
			t.Fatalf("resistance should be above price %.2f, got %+v", lastClose, level)
		}
	}
}

func TestAnalyzeRejectsEmptyCandles(t *testing.T) {
	if _, err := Analyze(nil, 1, 20); err == nil {
		t.Fatal("expected empty candles error")
	}
}

func TestNearestLevelDoesNotReturnWrongSide(t *testing.T) {
	levels := []ohlcv.SupportResistanceLevel{
		{Type: "support", Price: 90},
		{Type: "support", Price: 95},
		{Type: "support", Price: 105},
	}
	support := nearestLevel(levels, 100, false)
	if support == nil || support.Price != 95 {
		t.Fatalf("nearest support = %+v, want 95", support)
	}
	resistance := nearestLevel(levels, 100, true)
	if resistance == nil || resistance.Price != 105 {
		t.Fatalf("nearest resistance = %+v, want 105", resistance)
	}
	if got := nearestLevel([]ohlcv.SupportResistanceLevel{{Price: 80}}, 100, true); got != nil {
		t.Fatalf("expected no resistance above price, got %+v", got)
	}
}

func TestReclassifyByCurrentPriceMovesBrokenLevelsToCorrectSide(t *testing.T) {
	supports, resistances := reclassifyByCurrentPrice(
		[]ohlcv.SupportResistanceLevel{{Type: "support", Price: 110, Strength: 0.8, TouchCount: 2}},
		[]ohlcv.SupportResistanceLevel{{Type: "resistance", Price: 90, Strength: 0.7, TouchCount: 3}},
		100,
	)
	if len(supports) != 1 || supports[0].Type != "support" || supports[0].Price != 90 {
		t.Fatalf("broken resistance should become support below price: %+v", supports)
	}
	if len(resistances) != 1 || resistances[0].Type != "resistance" || resistances[0].Price != 110 {
		t.Fatalf("broken support should become resistance above price: %+v", resistances)
	}
}

func supportResistanceFixture() []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	closes := make([]float64, 60)
	for i := range closes {
		closes[i] = 100 + 8*math.Sin(float64(i)*math.Pi/6)
	}
	closes[len(closes)-1] = 100
	candles := make([]ohlcv.Candle, len(closes))
	prev := closes[0]
	for i, closePrice := range closes {
		open := prev
		high := closePrice + 1
		low := closePrice - 1
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 1000 + float64(i*10),
		}
		prev = closePrice
	}
	return candles
}
