package patterns

import (
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestDerivedProfilePatternTemplatesMatchWithOHLCV(t *testing.T) {
	tests := []struct {
		spec    patternSpec
		candles []ohlcv.Candle
	}{
		{spec: patternSpec{Name: "D-Shape Profile", Category: "market_profile", Direction: "neutral", Template: "market_profile", Confidence: 0.8}, candles: externalDataFixtureCandles()},
		{spec: patternSpec{Name: "High Volume Node", Category: "market_profile", Direction: "neutral", Template: "market_profile", Confidence: 0.8}, candles: externalDataFixtureCandles()},
		{spec: patternSpec{Name: "Double Top Buy Signal", Category: "point_and_figure", Direction: "bullish", Template: "point_figure", Confidence: 0.8}, candles: pointFigureFixtureCandles()},
	}
	for _, tt := range tests {
		t.Run(tt.spec.Name, func(t *testing.T) {
			match := matchPatternSpec(ScannerInput{Candles: tt.candles}, tt.spec)
			if !match.ok {
				t.Fatalf("matchPatternSpec(%s) did not match with derived OHLCV data", tt.spec.Template)
			}
		})
	}
}

func TestPatternScanDoesNotMarkProfileTemplatesAsExternalData(t *testing.T) {
	scans := scanPatternCatalog(ScannerInput{Candles: externalDataFixtureCandles()}, nil)

	seenProfile := false
	for _, scan := range scans {
		if scan.Source == "requires_external_data" {
			t.Fatalf("pattern scan still marks template as external data: %#v", scan)
		}
		if scan.Source != "market_profile" && scan.Source != "point_figure" {
			continue
		}
		seenProfile = true
	}
	if !seenProfile {
		t.Fatalf("no derived profile templates found in catalog scan")
	}
}

func externalDataFixtureCandles() []ohlcv.Candle {
	candles := make([]ohlcv.Candle, 60)
	for i := range candles {
		closeValue := 100 + float64(i%10)
		candles[i] = ohlcv.Candle{
			Time:   time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Open:   closeValue - 1,
			High:   closeValue + 2,
			Low:    closeValue - 2,
			Close:  closeValue,
			Volume: 1000 + float64(i*10),
		}
	}
	return candles
}

func pointFigureFixtureCandles() []ohlcv.Candle {
	closes := []float64{100, 101, 102, 103, 101, 99, 100, 102, 104, 105}
	candles := make([]ohlcv.Candle, len(closes))
	for i, closeValue := range closes {
		candles[i] = ohlcv.Candle{
			Time:   time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Open:   closeValue - 0.20,
			High:   closeValue + 0.35,
			Low:    closeValue - 0.35,
			Close:  closeValue,
			Volume: 1000 + float64(i*20),
		}
	}
	return candles
}
