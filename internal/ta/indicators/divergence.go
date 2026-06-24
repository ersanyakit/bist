package indicators

import (
	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

// DivergenceSignal holds the result of a divergence check.
type DivergenceSignal struct {
	// Bullish: price made a lower low while the oscillator made a higher low.
	Bullish bool
	// Bearish: price made a higher high while the oscillator made a lower high.
	Bearish bool
}

// RSIDivergence detects classic RSI divergence using the last two confirmed swing
// points in the price and RSI series. A swing is defined as a local extremum
// within a lookback window of `swingLookback` bars on each side.
func RSIDivergence(candles []ohlcv.Candle, rsiPeriod, swingLookback int) DivergenceSignal {
	if len(candles) < rsiPeriod+2*swingLookback+1 {
		return DivergenceSignal{}
	}
	cls := closes(candles)
	rsiSeries := RSISeries(cls, rsiPeriod)

	priceLows, priceHighs := swingExtremes(cls, swingLookback)
	rsiLows, rsiHighs := swingExtremes(rsiSeries, swingLookback)

	bull := bullishDivergence(priceLows, rsiLows)
	bear := bearishDivergence(priceHighs, rsiHighs)
	return DivergenceSignal{Bullish: bull, Bearish: bear}
}

// MACDHistogramDivergence detects divergence between price and the MACD histogram.
func MACDHistogramDivergence(candles []ohlcv.Candle, swingLookback int) DivergenceSignal {
	if len(candles) < 35+2*swingLookback {
		return DivergenceSignal{}
	}
	cls := closes(candles)
	histSeries := macdHistogramSeries(cls, 12, 26, 9)

	priceLows, priceHighs := swingExtremes(cls, swingLookback)
	histLows, histHighs := swingExtremes(histSeries, swingLookback)

	bull := bullishDivergence(priceLows, histLows)
	bear := bearishDivergence(priceHighs, histHighs)
	return DivergenceSignal{Bullish: bull, Bearish: bear}
}

// macdHistogramSeries returns the full MACD histogram series for divergence analysis.
func macdHistogramSeries(values []float64, fast, slow, signal int) []float64 {
	if len(values) < slow+signal-1 {
		return make([]float64, len(values))
	}
	fastEMA := EMASeries(values, fast)
	slowEMA := EMASeries(values, slow)
	if fastEMA == nil || slowEMA == nil {
		return make([]float64, len(values))
	}
	macdLine := make([]float64, len(values))
	for i := slow - 1; i < len(values); i++ {
		macdLine[i] = fastEMA[i] - slowEMA[i]
	}
	sigSeries := EMASeries(macdLine[slow-1:], signal)
	if sigSeries == nil {
		return make([]float64, len(values))
	}
	hist := make([]float64, len(values))
	offset := slow - 1
	for i, s := range sigSeries {
		hist[offset+i] = macdLine[offset+i] - s
	}
	return hist
}

// swingExtreme holds the value and index of a swing point.
type swingExtreme struct {
	idx   int
	value float64
}

// swingExtremes finds swing lows and highs using a `lookback` bar window on each side.
func swingExtremes(series []float64, lookback int) (lows, highs []swingExtreme) {
	for i := lookback; i < len(series)-lookback; i++ {
		isLow := true
		isHigh := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j == i {
				continue
			}
			if series[j] <= series[i] {
				isLow = false
			}
			if series[j] >= series[i] {
				isHigh = false
			}
		}
		if isLow {
			lows = append(lows, swingExtreme{idx: i, value: series[i]})
		}
		if isHigh {
			highs = append(highs, swingExtreme{idx: i, value: series[i]})
		}
	}
	return lows, highs
}

// bullishDivergence: price makes a lower low while the oscillator makes a higher low.
func bullishDivergence(priceLows, oscLows []swingExtreme) bool {
	if len(priceLows) < 2 || len(oscLows) < 2 {
		return false
	}
	pLast := priceLows[len(priceLows)-1]
	pPrev := priceLows[len(priceLows)-2]
	oLast := lastSwingBefore(oscLows, pLast.idx)
	oPrev := lastSwingBefore(oscLows, pPrev.idx+1)
	if oLast == nil || oPrev == nil {
		return false
	}
	priceLowerLow := pLast.value < pPrev.value*(1-mathutil.Epsilon)
	oscHigherLow := oLast.value > oPrev.value*(1+mathutil.Epsilon)
	return priceLowerLow && oscHigherLow
}

// bearishDivergence: price makes a higher high while the oscillator makes a lower high.
func bearishDivergence(priceHighs, oscHighs []swingExtreme) bool {
	if len(priceHighs) < 2 || len(oscHighs) < 2 {
		return false
	}
	pLast := priceHighs[len(priceHighs)-1]
	pPrev := priceHighs[len(priceHighs)-2]
	oLast := lastSwingBefore(oscHighs, pLast.idx)
	oPrev := lastSwingBefore(oscHighs, pPrev.idx+1)
	if oLast == nil || oPrev == nil {
		return false
	}
	priceHigherHigh := pLast.value > pPrev.value*(1+mathutil.Epsilon)
	oscLowerHigh := oLast.value < oPrev.value*(1-mathutil.Epsilon)
	return priceHigherHigh && oscLowerHigh
}

func lastSwingBefore(swings []swingExtreme, maxIdx int) *swingExtreme {
	for i := len(swings) - 1; i >= 0; i-- {
		if swings[i].idx <= maxIdx {
			return &swings[i]
		}
	}
	return nil
}

// DivergenceResult packages RSI and MACD histogram divergence signals together.
type DivergenceResult struct {
	RSI  DivergenceSignal
	MACD DivergenceSignal
}

// DetectDivergences runs both RSI and MACD histogram divergence detection.
func DetectDivergences(candles []ohlcv.Candle) DivergenceResult {
	return DivergenceResult{
		RSI:  RSIDivergence(candles, 14, 3),
		MACD: MACDHistogramDivergence(candles, 3),
	}
}
