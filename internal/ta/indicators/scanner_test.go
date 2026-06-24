package indicators

import (
	"context"
	"strings"
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestRegisteredIndicatorCatalogCoversRequestedFamilies(t *testing.T) {
	names := RegisteredIndicatorNames()
	if len(names) < 890 {
		t.Fatalf("registered indicator catalog is too small: got %d", len(names))
	}
	for _, expected := range []string{
		"Simple Moving Average",
		"RSI Divergence",
		"Funding Rate",
		"TD Sequential",
		"Footprint Chart",
		"Fibonacci Spiral",
		"Point of Control",
	} {
		if !containsIndicatorName(names, expected) {
			t.Fatalf("missing expected indicator %q", expected)
		}
	}
}

func TestIndicatorScannerInputOutputContract(t *testing.T) {
	candles := indicatorTestCandles(120)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	output, err := ScanIndicators(context.Background(), ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
	})
	if err != nil {
		t.Fatalf("scan indicators: %v", err)
	}
	if output.DetectorCount != len(RegisteredIndicatorNames()) {
		t.Fatalf("detector count mismatch: got %d names=%d", output.DetectorCount, len(RegisteredIndicatorNames()))
	}
	if output.ScannedCount != output.DetectorCount {
		t.Fatalf("scanned count mismatch: scanned=%d detectors=%d", output.ScannedCount, output.DetectorCount)
	}
	if output.ComputedCount == 0 {
		t.Fatal("expected computed indicators")
	}
	if output.SignalCount == 0 {
		t.Fatal("expected active indicator signals")
	}
	if !hasIndicatorSignal(output.Indicators, "Funding Rate", "requires_external_data") {
		t.Fatal("expected Funding Rate to be marked as external-data dependent")
	}
	if !hasComputedIndicator(output.Indicators, "Simple Moving Average") {
		t.Fatal("expected Simple Moving Average to be computed")
	}
}

func TestRSIFlatSeriesIsNeutral(t *testing.T) {
	values := make([]float64, 40)
	for i := range values {
		values[i] = 100
	}
	if got := RSI(values, 14); got != 50 {
		t.Fatalf("flat RSI should be neutral, got %.2f", got)
	}
}

func TestPivotPointsUsePreviousCandle(t *testing.T) {
	candles := []ohlcv.Candle{
		{High: 12, Low: 8, Close: 10},
		{High: 120, Low: 80, Close: 100},
	}
	pivot, r1, r2, s1, s2 := PivotPoints(candles)
	if pivot != 10 || r1 != 12 || r2 != 14 || s1 != 8 || s2 != 6 {
		t.Fatalf("pivot should use previous completed candle, got pivot=%.2f r1=%.2f r2=%.2f s1=%.2f s2=%.2f", pivot, r1, r2, s1, s2)
	}
}

func TestDerivedLevelAliasesUseCompletedPriorWindow(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, 25)
	for i := 0; i < len(candles)-1; i++ {
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   95,
			High:   100,
			Low:    90,
			Close:  96,
			Volume: 1000,
		}
	}
	candles[len(candles)-1] = ohlcv.Candle{
		Time:   start.AddDate(0, 0, len(candles)-1),
		Open:   101,
		High:   106,
		Low:    98,
		Close:  105,
		Volume: 2000,
	}

	values := knownIndicatorValues(ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		LastClose: candles[len(candles)-1].EffectiveClose(),
	})

	for _, name := range []string{"Previous Week High", "Previous Month High", "Highest High Stop", "Support Resistance Levels"} {
		got := values[normalizeIndicatorText(name)].value
		if got != 100 {
			t.Fatalf("%s should use completed prior window, got %.8f", name, got)
		}
	}
	for _, name := range []string{"Previous Week Low", "Previous Month Low", "Lowest Low Stop", "Supply Demand Zones"} {
		got := values[normalizeIndicatorText(name)].value
		if got != 90 {
			t.Fatalf("%s should use completed prior window, got %.8f", name, got)
		}
	}
}

func TestIndicatorScannerDoesNotPromoteProxyValues(t *testing.T) {
	candles := indicatorTestCandles(240)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	output, err := ScanIndicators(context.Background(), ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
	})
	if err != nil {
		t.Fatalf("scan indicators: %v", err)
	}
	for _, name := range []string{"MAMA", "FAMA"} {
		result, ok := findIndicator(output.Indicators, name)
		if !ok {
			t.Fatalf("expected indicator %q in scan output", name)
		}
		if result.Computed || result.Signal != algorithmRequiredSource || result.Source != algorithmRequiredSource {
			t.Fatalf("expected %s to be algorithm-required, got %+v", name, result)
		}
	}
}

func TestIndicatorScannerMarksExactFormulaInsufficientData(t *testing.T) {
	candles := indicatorTestCandles(80)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	result, err := detectIndicatorSpec(ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
	}, indicatorSpec{
		Name:       "SMA200",
		Category:   "trend",
		Group:      "trend_indikatorleri",
		Template:   "moving_average",
		Confidence: 0.62,
	})
	if err != nil {
		t.Fatalf("detect indicator: %v", err)
	}
	if result.Computed || result.Signal != "insufficient_data" || result.Source != insufficientDataSource || result.Confidence != 0 {
		t.Fatalf("SMA200 with 80 candles should be insufficient-data, got %+v", result)
	}
}

func TestADXSignalIsTrendStrengthNotDirection(t *testing.T) {
	candles := indicatorTestCandles(240)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	output, err := ScanIndicators(context.Background(), ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
	})
	if err != nil {
		t.Fatalf("scan indicators: %v", err)
	}
	result, ok := findIndicator(output.Indicators, "ADX")
	if !ok {
		t.Fatal("expected ADX in scan output")
	}
	if result.Signal == "bullish" || result.Signal == "bearish" {
		t.Fatalf("ADX should report trend strength, not direction: %+v", result)
	}
}

func TestChikouSpanUsesSignedPastCloseDifference(t *testing.T) {
	candles := indicatorTestCandles(80)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshot.IchimokuChikou = -12
	result, err := detectIndicatorSpec(ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
		LastClose: candles[len(candles)-1].EffectiveClose(),
	}, indicatorSpec{Name: "Chikou Span", Category: "trend", Group: "trend_indikatorleri", Template: "trend", Confidence: 0.62})
	if err != nil {
		t.Fatalf("detect Chikou: %v", err)
	}
	if result.Signal != "bearish" {
		t.Fatalf("negative Chikou should be bearish, got %+v", result)
	}
	if len(result.Evidence) == 0 || strings.Contains(result.Evidence[0], "price is below trend indicator value") {
		t.Fatalf("Chikou must not be explained as a price-level comparison: %+v", result.Evidence)
	}

	snapshot.IchimokuChikou = 8
	result, err = detectIndicatorSpec(ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
		LastClose: candles[len(candles)-1].EffectiveClose(),
	}, indicatorSpec{Name: "Chikou Span", Category: "trend", Group: "trend_indikatorleri", Template: "trend", Confidence: 0.62})
	if err != nil {
		t.Fatalf("detect Chikou positive: %v", err)
	}
	if result.Signal != "bullish" {
		t.Fatalf("positive Chikou should be bullish, got %+v", result)
	}
}

func TestBillWilliamsMFIDoesNotUseWilliamsROscillatorRules(t *testing.T) {
	candles := indicatorTestCandles(80)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	result, err := detectIndicatorSpec(ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
		LastClose: candles[len(candles)-1].EffectiveClose(),
	}, indicatorSpec{Name: "MFI Bill Williams", Category: "volume", Group: "hacim_volume_indikatorleri", Template: "volume", Confidence: 0.62})
	if err != nil {
		t.Fatalf("detect BW MFI: %v", err)
	}
	if result.Signal != "info" {
		t.Fatalf("BW MFI should be informational without color-state confirmation, got %+v", result)
	}
	if len(result.Evidence) == 0 || strings.Contains(result.Evidence[0], "Williams %R") {
		t.Fatalf("BW MFI must not reuse Williams %%R evidence: %+v", result.Evidence)
	}
}

func TestIndicatorScannerReconcilesSnapshotContradictions(t *testing.T) {
	candles := indicatorTestCandles(240)
	lastVolume := candles[len(candles)-1].EffectiveVolume()
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshot.MACD = -2
	snapshot.MACDSignal = -1
	snapshot.MACDHistogram = -1
	snapshot.VolumeSMA20 = lastVolume * 2
	snapshot.IchimokuCloudTrend = -1
	output, err := ScanIndicators(context.Background(), ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Snapshot:   snapshot,
		LastVolume: lastVolume,
	})
	if err != nil {
		t.Fatalf("scan indicators: %v", err)
	}
	for _, item := range []string{"MACD", "Volume", "Ichimoku Cloud"} {
		result, ok := findIndicator(output.Indicators, item)
		if !ok {
			t.Fatalf("expected %s in scan output", item)
		}
		if result.Signal == "bullish" {
			t.Fatalf("%s should not be bullish against bearish snapshot: %+v", item, result)
		}
		if result.Signal != "bearish" {
			t.Fatalf("%s signal = %s, want bearish: %+v", item, result.Signal, result)
		}
	}
	validation := ValidateIndicatorSignalConsistency(ScannerInput{
		Candles:    candles,
		Snapshot:   snapshot,
		LastVolume: lastVolume,
	}, output)
	if validation.HasErrors() {
		t.Fatalf("reconciled indicators should pass consistency validation: %s", validation.Summary())
	}
}

func containsIndicatorName(names []string, expected string) bool {
	for _, name := range names {
		if name == expected {
			return true
		}
	}
	return false
}

func hasIndicatorSignal(results []ohlcv.IndicatorResult, name, signal string) bool {
	for _, result := range results {
		if result.Name == name && result.Signal == signal {
			return true
		}
	}
	return false
}

func hasComputedIndicator(results []ohlcv.IndicatorResult, name string) bool {
	for _, result := range results {
		if result.Name == name && result.Computed {
			return true
		}
	}
	return false
}

func findIndicator(results []ohlcv.IndicatorResult, name string) (ohlcv.IndicatorResult, bool) {
	for _, result := range results {
		if result.Name == name {
			return result, true
		}
	}
	return ohlcv.IndicatorResult{}, false
}

func indicatorTestCandles(count int) []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	price := 100.0
	for i := 0; i < count; i++ {
		open := price
		closePrice := price + 0.45 + float64(i%7)*0.05
		high := closePrice + 1.2
		low := open - 0.7
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 1000 + float64(i*15),
		}
		price = closePrice
	}
	return candles
}
