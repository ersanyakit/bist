package indicators

import (
	"context"
	"math"
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestIndicatorValidatorsAcceptSnapshotAndScan(t *testing.T) {
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
	report := ValidateIndicatorSystem(candles, snapshot, output)
	if err := report.Err(); err != nil {
		t.Fatalf("indicator validation failed: %v", err)
	}
	if !hasValidationWarning(report) {
		t.Fatal("expected catalog warnings for proxy/external-only indicator formulas")
	}
}

func TestIndicatorValidatorRejectsCorruptSnapshot(t *testing.T) {
	candles := indicatorTestCandles(80)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	snapshot.RSI14 = 150
	report := ValidateSnapshot(candles, snapshot)
	if !report.HasErrors() {
		t.Fatalf("expected corrupt snapshot to fail validation: %s", report.Summary())
	}
}

func TestSignedSeriesNumericHelpersUseWindowValues(t *testing.T) {
	candles := signedIndicatorCandles(40)
	gotK, gotD := StochasticOscillator(candles, 14, 3)
	wantK, wantD := referenceStochastic(candles, 14, 3)
	assertClose(t, "signed stochastic %K", gotK, wantK, 1e-9)
	assertClose(t, "signed stochastic %D", gotD, wantD, 1e-9)

	gotWilliams := WilliamsR(candles, 14)
	if gotWilliams < -100 || gotWilliams > 0 {
		t.Fatalf("signed Williams %%R outside range: %.8f", gotWilliams)
	}
	upper, lower := Donchian(candles, 20)
	if upper < lower {
		t.Fatalf("signed Donchian upper/lower inverted: upper=%.8f lower=%.8f", upper, lower)
	}
	if got := ROC([]float64{-100, -90}, 1); math.Abs(got-10) > 1e-9 {
		t.Fatalf("signed ROC = %.8f, want 10", got)
	}
	if width := BollingerBandWidth([]float64{-105, -103, -101, -100, -98, -97, -95}, 5, 2); width < 0 {
		t.Fatalf("signed Bollinger bandwidth should be non-negative, got %.8f", width)
	}
}

func hasValidationWarning(report ValidationReport) bool {
	for _, issue := range report.Issues {
		if issue.Severity == ValidationSeverityWarning {
			return true
		}
	}
	return false
}

func signedIndicatorCandles(count int) []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	price := -120.0
	for i := range candles {
		change := 0.45 + math.Sin(float64(i)*0.37)*0.35
		open := price
		closePrice := price + change
		high := math.Max(open, closePrice) + 0.6
		low := math.Min(open, closePrice) - 0.5
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 1000 + float64(i*11),
		}
		price = closePrice
	}
	return candles
}
