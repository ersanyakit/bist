package indicators

import "testing"

func TestBullishDivergenceRequiresRecentPriceSwing(t *testing.T) {
	priceLows := []swingExtreme{{idx: 10, value: 100}, {idx: 40, value: 90}} // lower low
	oscLows := []swingExtreme{{idx: 10, value: 30}, {idx: 40, value: 40}}    // higher low
	const maxAge = 34

	if !bullishDivergence(priceLows, oscLows, 41, maxAge) {
		t.Fatal("expected bullish divergence when the price swing is fresh (right at series end)")
	}
	// 60 bars have passed since the price swing with no newer swing printed since —
	// the divergence must no longer be reported as active.
	if bullishDivergence(priceLows, oscLows, 101, maxAge) {
		t.Fatal("expected bullish divergence to expire once the price swing goes stale")
	}
}

func TestBearishDivergenceRequiresRecentPriceSwing(t *testing.T) {
	priceHighs := []swingExtreme{{idx: 10, value: 100}, {idx: 40, value: 110}} // higher high
	oscHighs := []swingExtreme{{idx: 10, value: 70}, {idx: 40, value: 60}}     // lower high
	const maxAge = 34

	if !bearishDivergence(priceHighs, oscHighs, 41, maxAge) {
		t.Fatal("expected bearish divergence when the price swing is fresh (right at series end)")
	}
	if bearishDivergence(priceHighs, oscHighs, 101, maxAge) {
		t.Fatal("expected bearish divergence to expire once the price swing goes stale")
	}
}

func TestDivergenceRegularDefinitionIsNotSwapped(t *testing.T) {
	// Regular bullish divergence: price lower-low + oscillator higher-low. Confirm the
	// inverse (price higher-low, i.e. no new low) does NOT trigger it.
	priceLows := []swingExtreme{{idx: 10, value: 90}, {idx: 20, value: 95}} // higher low, not lower low
	oscLows := []swingExtreme{{idx: 10, value: 30}, {idx: 20, value: 40}}
	if bullishDivergence(priceLows, oscLows, 21, 100) {
		t.Fatal("bullishDivergence should require price to make a LOWER low, not a higher one")
	}

	// Regular bearish divergence: price higher-high + oscillator lower-high. Confirm
	// the inverse (price lower-high) does NOT trigger it.
	priceHighs := []swingExtreme{{idx: 10, value: 100}, {idx: 20, value: 95}} // lower high, not higher high
	oscHighs := []swingExtreme{{idx: 10, value: 70}, {idx: 20, value: 60}}
	if bearishDivergence(priceHighs, oscHighs, 21, 100) {
		t.Fatal("bearishDivergence should require price to make a HIGHER high, not a lower one")
	}
}

// TestDivergenceProxyMatchesSwingBasedDetection guards against divergenceProxy
// regressing to its old naive two-fixed-point comparison, which could report a
// divergence with no genuine local swing extremum in either series (a classic source
// of false signals on noisy data). It must agree with the real swing-based detector.
func TestDivergenceProxyMatchesSwingBasedDetection(t *testing.T) {
	candles := indicatorTestCandles(80)
	got := divergenceProxy(ScannerInput{Candles: candles})

	divs := DetectDivergences(candles)
	bullish := divs.RSI.Bullish || divs.MACD.Bullish
	bearish := divs.RSI.Bearish || divs.MACD.Bearish
	want := 0.0
	switch {
	case bullish && !bearish:
		want = 1
	case bearish && !bullish:
		want = -1
	}
	if got != want {
		t.Fatalf("divergenceProxy() = %v, want %v (bullish=%v bearish=%v)", got, want, bullish, bearish)
	}
}

func TestDivergenceToleranceHandlesNegativeOscillatorValues(t *testing.T) {
	// MACD-histogram-style oscillator values are routinely negative. -3 is a genuinely
	// higher low than -5 (closer to zero); a naive `value*(1+Epsilon)` tolerance
	// tightens rather than loosens the comparison for negative values.
	priceLows := []swingExtreme{{idx: 10, value: 100}, {idx: 20, value: 90}}
	oscLows := []swingExtreme{{idx: 10, value: -5}, {idx: 20, value: -3}}
	if !bullishDivergence(priceLows, oscLows, 21, 100) {
		t.Fatal("expected bullish divergence: -3 is a higher low than -5")
	}

	priceHighs := []swingExtreme{{idx: 10, value: 100}, {idx: 20, value: 110}}
	oscHighs := []swingExtreme{{idx: 10, value: -3}, {idx: 20, value: -5}}
	if !bearishDivergence(priceHighs, oscHighs, 21, 100) {
		t.Fatal("expected bearish divergence: -5 is a lower high than -3")
	}
}
