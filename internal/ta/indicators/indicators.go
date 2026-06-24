// internal/indicators/indicators.go
package indicators

import (
	"fmt"
	"math"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

func Snapshot(candles []ohlcv.Candle) (ohlcv.IndicatorSnapshot, error) {
	if len(candles) == 0 {
		return ohlcv.IndicatorSnapshot{}, fmt.Errorf("indicator snapshot requires candles: %w", ErrInsufficientData)
	}
	closes := closes(candles)
	volumes := volumes(candles)
	typicals := typicalPrices(candles)

	macdLine, signalLine, hist := MACD(closes, 12, 26, 9)
	upper, middle, lower := BollingerBands(closes, 20, 2)
	keltnerUpper, keltnerMiddle, keltnerLower := Keltner(candles, 20, 10, 2)
	stochK, stochD := StochasticOscillator(candles, 14, 3)
	stochRSIK, stochRSID := StochasticRSI(closes, 14, 14, 3, 3)
	tenkan, kijun, senkouA, senkouB, chikou, cloudTrend, kumoTwist, tkCross, priceCloudBreakout := Ichimoku(candles, 9, 26, 52)
	donchianUpper, donchianLower := Donchian(candles, 20)
	pivot, r1, r2, s1, s2 := PivotPoints(candles)
	additional := AdditionalIndicators(candles)
	supertrendCurrent, supertrendPrev := SupertrendLastTwo(candles, 10, 3)

	return ohlcv.IndicatorSnapshot{
		SMA5:                       SMA(closes, 5),
		SMA10:                      SMA(closes, 10),
		SMA20:                      SMA(closes, 20),
		SMA50:                      SMA(closes, 50),
		SMA100:                     SMA(closes, 100),
		SMA200:                     SMA(closes, 200),
		EMA5:                       EMA(closes, 5),
		EMA10:                      EMA(closes, 10),
		EMA20:                      EMA(closes, 20),
		EMA50:                      EMA(closes, 50),
		EMA100:                     EMA(closes, 100),
		EMA200:                     EMA(closes, 200),
		RSI14:                      RSI(closes, 14),
		ATR14:                      ATR(candles, 14),
		MACD:                       macdLine,
		MACDSignal:                 signalLine,
		MACDHistogram:              hist,
		BollingerUpper:             upper,
		BollingerMiddle:            middle,
		BollingerLower:             lower,
		ADX14:                      ADX(candles, 14),
		VWAP:                       AnchoredVWAP(candles, maxInt(0, len(candles)-20)),
		OBV:                        OBV(candles),
		OBVSlope:                   OBVSlope(candles, 20),
		MFI14:                      MFI(candles, 14),
		StochRSIK:                  stochRSIK,
		StochRSID:                  stochRSID,
		StochasticK:                stochK,
		StochasticD:                stochD,
		CCI20:                      CCI(typicals, 20),
		WilliamsR14:                WilliamsR(candles, 14),
		ROC12:                      ROC(closes, 12),
		Supertrend:                 supertrendCurrent,
		SupertrendPrev:             supertrendPrev,
		DonchianUpper:              donchianUpper,
		DonchianLower:              donchianLower,
		KeltnerUpper:               keltnerUpper,
		KeltnerMiddle:              keltnerMiddle,
		KeltnerLower:               keltnerLower,
		VolumeSMA20:                SMA(volumes, 20),
		ChaikinMoneyFlow20:         ChaikinMoneyFlow(candles, 20),
		AccumulationDistribution:   AccumulationDistribution(candles),
		IchimokuTenkan:             tenkan,
		IchimokuKijun:              kijun,
		IchimokuSenkouA:            senkouA,
		IchimokuSenkouB:            senkouB,
		IchimokuChikou:             chikou,
		IchimokuCloudTrend:         cloudTrend,
		IchimokuKumoTwist:          kumoTwist,
		IchimokuTKCross:            tkCross,
		IchimokuPriceCloudBreakout: priceCloudBreakout,
		PivotPoint:                 pivot,
		PivotR1:                    r1,
		PivotR2:                    r2,
		PivotS1:                    s1,
		PivotS2:                    s2,
		FibonacciLevels:            FibonacciLevels(candles),
		AdditionalIndicators:       additional,
		SupportTools:               SupportTools(candles),
		MarketStructure:            MarketStructure(candles),
		RelativeStrength:           RelativeStrengthMetrics(candles),
	}, nil
}

var ErrInsufficientData = fmt.Errorf("insufficient data")

func SMA(values []float64, period int) float64 {
	if period <= 0 || len(values) == 0 {
		return 0
	}
	if len(values) < period {
		return 0
	}
	return mathutil.Mean(values[len(values)-period:])
}

func EMA(values []float64, period int) float64 {
	series := EMASeries(values, period)
	if len(series) == 0 {
		return 0
	}
	return series[len(series)-1]
}

func EMASeries(values []float64, period int) []float64 {
	if len(values) == 0 || period <= 0 {
		return nil
	}
	if len(values) < period {
		return nil
	}
	out := make([]float64, len(values))

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
		out[i] = sum / float64(i+1)
	}
	sma := sum / float64(period)
	out[period-1] = sma

	alpha := 2.0 / float64(period+1)
	for i := period; i < len(values); i++ {
		out[i] = values[i]*alpha + out[i-1]*(1-alpha)
	}
	return out
}

func RSI(values []float64, period int) float64 {
	series := RSISeries(values, period)
	if len(series) == 0 {
		return 50
	}
	return series[len(series)-1]
}

func RSISeries(values []float64, period int) []float64 {
	if len(values) < 2 || period <= 0 {
		return nil
	}
	out := make([]float64, len(values))
	out[0] = 50

	if len(values) <= period {
		for i := 1; i < len(values); i++ {
			out[i] = 50
		}
		return out
	}

	sumGain := 0.0
	sumLoss := 0.0
	for i := 1; i <= period; i++ {
		change := values[i] - values[i-1]
		if change > 0 {
			sumGain += change
		} else {
			sumLoss -= change
		}
		out[i] = 50
	}

	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)

	if avgLoss == 0 && avgGain == 0 {
		out[period] = 50
	} else if avgLoss == 0 {
		out[period] = 100
	} else if avgGain == 0 {
		out[period] = 0
	} else {
		rs := avgGain / avgLoss
		out[period] = 100 - 100/(1+rs)
	}

	for i := period + 1; i < len(values); i++ {
		change := values[i] - values[i-1]
		currentGain := 0.0
		currentLoss := 0.0
		if change > 0 {
			currentGain = change
		} else {
			currentLoss = -change
		}

		avgGain = (avgGain*float64(period-1) + currentGain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + currentLoss) / float64(period)

		if avgLoss == 0 && avgGain == 0 {
			out[i] = 50
		} else if avgLoss == 0 {
			out[i] = 100
		} else if avgGain == 0 {
			out[i] = 0
		} else {
			rs := avgGain / avgLoss
			out[i] = 100 - 100/(1+rs)
		}
	}
	return out
}

func ATR(candles []ohlcv.Candle, period int) float64 {
	series := ATRSeries(candles, period)
	if len(series) == 0 {
		return 0
	}
	return series[len(series)-1]
}

func ATRSeries(candles []ohlcv.Candle, period int) []float64 {
	if len(candles) == 0 || period <= 0 {
		return nil
	}
	tr := make([]float64, len(candles))
	for i, candle := range candles {
		high := candle.EffectiveHigh()
		low := candle.EffectiveLow()
		if i == 0 {
			tr[i] = high - low
		} else {
			prevClose := candles[i-1].EffectiveClose()
			tr[i] = math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		}
	}
	out := make([]float64, len(tr))
	if len(tr) <= period {
		for i := range tr {
			out[i] = mathutil.Mean(tr[:i+1])
		}
		return out
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
		out[i] = sum / float64(i+1)
	}

	for i := period; i < len(tr); i++ {
		out[i] = (out[i-1]*float64(period-1) + tr[i]) / float64(period)
	}
	return out
}

func MACD(values []float64, fast, slow, signal int) (float64, float64, float64) {
	if len(values) == 0 || len(values) < slow+signal-1 {
		return 0, 0, 0
	}
	fastEMA := EMASeries(values, fast)
	slowEMA := EMASeries(values, slow)
	line := make([]float64, len(values))
	for i := slow - 1; i < len(values); i++ {
		line[i] = fastEMA[i] - slowEMA[i]
	}
	signalSeries := EMASeries(line[slow-1:], signal)
	last := len(values) - 1
	macdLine := line[last]
	sig := signalSeries[len(signalSeries)-1]
	return macdLine, sig, macdLine - sig
}

func BollingerBands(values []float64, period int, multiplier float64) (float64, float64, float64) {
	if len(values) < period {
		return 0, 0, 0
	}
	window := values[len(values)-period:]
	middle := mathutil.Mean(window)
	width := populationStdDev(window) * multiplier
	return middle + width, middle, middle - width
}

// populationStdDev computes the rolling population standard deviation used by Bollinger Bands.
func populationStdDev(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	mean := mathutil.Mean(values)
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(n))
}

func ADX(candles []ohlcv.Candle, period int) float64 {
	// Wilder's ADX requires 2*period candles: first period for TR/DM smoothing, second for DX→ADX.
	if period <= 0 || len(candles) < 2*period {
		return 0
	}
	plusDM := make([]float64, len(candles))
	minusDM := make([]float64, len(candles))
	tr := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		upMove := candles[i].EffectiveHigh() - candles[i-1].EffectiveHigh()
		downMove := candles[i-1].EffectiveLow() - candles[i].EffectiveLow()
		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
		prevClose := candles[i-1].EffectiveClose()
		tr[i] = math.Max(candles[i].EffectiveHigh()-candles[i].EffectiveLow(), math.Max(math.Abs(candles[i].EffectiveHigh()-prevClose), math.Abs(candles[i].EffectiveLow()-prevClose)))
	}
	smoothedPlusDM := 0.0
	smoothedMinusDM := 0.0
	smoothedTR := 0.0
	for i := 1; i <= period; i++ {
		smoothedPlusDM += plusDM[i]
		smoothedMinusDM += minusDM[i]
		smoothedTR += tr[i]
	}
	dxValues := []float64{dxFromDirectionalMovement(smoothedPlusDM, smoothedMinusDM, smoothedTR)}
	for i := period + 1; i < len(candles); i++ {
		smoothedPlusDM = smoothedPlusDM - smoothedPlusDM/float64(period) + plusDM[i]
		smoothedMinusDM = smoothedMinusDM - smoothedMinusDM/float64(period) + minusDM[i]
		smoothedTR = smoothedTR - smoothedTR/float64(period) + tr[i]
		dxValues = append(dxValues, dxFromDirectionalMovement(smoothedPlusDM, smoothedMinusDM, smoothedTR))
	}
	if len(dxValues) == 0 {
		return 0
	}
	if len(dxValues) < period {
		return mathutil.Clamp(mathutil.Mean(dxValues), 0, 100)
	}
	adx := mathutil.Mean(dxValues[:period])
	for i := period; i < len(dxValues); i++ {
		adx = (adx*float64(period-1) + dxValues[i]) / float64(period)
	}
	return mathutil.Clamp(adx, 0, 100)
}

func dxFromDirectionalMovement(plusDM, minusDM, tr float64) float64 {
	if mathutil.AlmostEqual(tr, 0) {
		return 0
	}
	plusDI := 100 * mathutil.SafeDiv(plusDM, tr)
	minusDI := 100 * mathutil.SafeDiv(minusDM, tr)
	return 100 * mathutil.SafeDiv(math.Abs(plusDI-minusDI), plusDI+minusDI)
}

func VWAP(candles []ohlcv.Candle) float64 {
	sumPV := 0.0
	sumVolume := 0.0
	for _, candle := range candles {
		typical := (candle.EffectiveHigh() + candle.EffectiveLow() + candle.EffectiveClose()) / 3
		volume := candle.EffectiveVolume()
		sumPV += typical * volume
		sumVolume += volume
	}
	return mathutil.SafeDiv(sumPV, sumVolume)
}

func OBV(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	obv := 0.0
	for i := 1; i < len(candles); i++ {
		if candles[i].EffectiveClose() > candles[i-1].EffectiveClose() {
			obv += candles[i].EffectiveVolume()
		} else if candles[i].EffectiveClose() < candles[i-1].EffectiveClose() {
			obv -= candles[i].EffectiveVolume()
		}
	}
	return obv
}

func MFI(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 || period <= 0 {
		return 50
	}
	start := len(candles) - period
	if start < 1 {
		start = 1
	}
	positive := 0.0
	negative := 0.0
	for i := start; i < len(candles); i++ {
		current := (candles[i].EffectiveHigh() + candles[i].EffectiveLow() + candles[i].EffectiveClose()) / 3
		previous := (candles[i-1].EffectiveHigh() + candles[i-1].EffectiveLow() + candles[i-1].EffectiveClose()) / 3
		flow := current * candles[i].EffectiveVolume()
		if current > previous {
			positive += flow
		} else if current < previous {
			negative += flow
		}
	}
	if mathutil.AlmostEqual(positive, 0) && mathutil.AlmostEqual(negative, 0) {
		return 50
	}
	if mathutil.AlmostEqual(negative, 0) {
		return 100
	}
	if mathutil.AlmostEqual(positive, 0) {
		return 0
	}
	return 100 - 100/(1+positive/negative)
}

func StochasticRSI(values []float64, rsiPeriod, stochPeriod, kPeriod, dPeriod int) (float64, float64) {
	rsi := RSISeries(values, rsiPeriod)
	if len(rsi) == 0 {
		return 50, 50
	}
	kSeries := make([]float64, len(rsi))
	for i := range rsi {
		start := i - stochPeriod + 1
		if start < 0 {
			start = 0
		}
		window := rsi[start : i+1]
		lowest := mathutil.Min(window)
		highest := mathutil.Max(window)
		if mathutil.AlmostEqual(highest, lowest) {
			kSeries[i] = 50
		} else {
			kSeries[i] = 100 * (rsi[i] - lowest) / (highest - lowest)
		}
	}
	smoothedK := make([]float64, len(kSeries))
	for i := range kSeries {
		smoothedK[i] = partialSMA(kSeries[:i+1], kPeriod)
	}
	smoothedD := make([]float64, len(smoothedK))
	for i := range smoothedK {
		smoothedD[i] = partialSMA(smoothedK[:i+1], dPeriod)
	}
	return smoothedK[len(smoothedK)-1], smoothedD[len(smoothedD)-1]
}

func StochasticOscillator(candles []ohlcv.Candle, kPeriod, dPeriod int) (float64, float64) {
	if len(candles) == 0 {
		return 50, 50
	}
	rawK := make([]float64, len(candles))
	for i := range candles {
		start := i - kPeriod + 1
		if start < 0 {
			start = 0
		}
		window := candles[start : i+1]
		highest := window[0].EffectiveHigh()
		lowest := window[0].EffectiveLow()
		for _, candle := range window[1:] {
			highest = math.Max(highest, candle.EffectiveHigh())
			lowest = math.Min(lowest, candle.EffectiveLow())
		}
		if mathutil.AlmostEqual(highest, lowest) {
			rawK[i] = 50
		} else {
			rawK[i] = 100 * (candles[i].EffectiveClose() - lowest) / (highest - lowest)
		}
	}

	smoothK := make([]float64, len(rawK))
	for i := range rawK {
		smoothK[i] = partialSMA(rawK[:i+1], 3) // standard 3-period smoothing for Slow %K
	}

	smoothedD := make([]float64, len(smoothK))
	for i := range smoothK {
		smoothedD[i] = partialSMA(smoothK[:i+1], dPeriod)
	}

	return smoothK[len(smoothK)-1], smoothedD[len(smoothedD)-1]
}

func partialSMA(values []float64, period int) float64 {
	if period <= 0 || len(values) == 0 {
		return 0
	}
	n := len(values)
	if n > period {
		n = period
	}
	return mathutil.Mean(values[len(values)-n:])
}

func CCI(typicals []float64, period int) float64 {
	if len(typicals) == 0 {
		return 0
	}
	start := len(typicals) - period
	if start < 0 {
		start = 0
	}
	window := typicals[start:]
	mean := mathutil.Mean(window)
	meanDeviation := 0.0
	for _, value := range window {
		meanDeviation += math.Abs(value - mean)
	}
	meanDeviation = mathutil.SafeDiv(meanDeviation, float64(len(window)))
	return mathutil.SafeDiv(typicals[len(typicals)-1]-mean, 0.015*meanDeviation)
}

func WilliamsR(candles []ohlcv.Candle, period int) float64 {
	if len(candles) == 0 {
		return -50
	}
	start := len(candles) - period
	if start < 0 {
		start = 0
	}
	window := candles[start:]
	highest := window[0].EffectiveHigh()
	lowest := window[0].EffectiveLow()
	for _, candle := range window[1:] {
		highest = math.Max(highest, candle.EffectiveHigh())
		lowest = math.Min(lowest, candle.EffectiveLow())
	}
	closePrice := candles[len(candles)-1].EffectiveClose()
	if mathutil.AlmostEqual(highest, lowest) {
		return -50
	}
	return -100 * mathutil.SafeDiv(highest-closePrice, highest-lowest)
}

func ROC(values []float64, period int) float64 {
	if len(values) <= period || period <= 0 {
		return 0
	}
	old := values[len(values)-1-period]
	return 100 * mathutil.SafeDiv(values[len(values)-1]-old, math.Max(math.Abs(old), mathutil.Epsilon))
}

func HighsLows(candles []ohlcv.Candle, period int) float64 {
	if len(candles) == 0 || period <= 0 {
		return 0
	}
	recentHighs := lastValues(highs(candles), period)
	recentLows := lastValues(lows(candles), period)
	if len(recentHighs) == 0 || len(recentLows) == 0 {
		return 0
	}
	midpoint := (mathutil.Max(recentHighs) + mathutil.Min(recentLows)) / 2
	return candles[len(candles)-1].EffectiveClose() - midpoint
}

func Supertrend(candles []ohlcv.Candle, period int, multiplier float64) float64 {
	cur, _ := SupertrendLastTwo(candles, period, multiplier)
	return cur
}

// SupertrendLastTwo returns the Supertrend values for the last two bars (current, previous).
// prev is 0 when there is insufficient history. Use prev to detect crossovers.
func SupertrendLastTwo(candles []ohlcv.Candle, period int, multiplier float64) (current, prev float64) {
	if len(candles) == 0 {
		return 0, 0
	}
	atr := ATRSeries(candles, period)
	finalLower := make([]float64, len(candles))
	finalUpper := make([]float64, len(candles))
	direction := 1
	results := make([]float64, len(candles))
	for i, candle := range candles {
		hl2 := (candle.EffectiveHigh() + candle.EffectiveLow()) / 2
		basicUpper := hl2 + multiplier*atr[i]
		basicLower := hl2 - multiplier*atr[i]
		if i == 0 {
			finalLower[i] = basicLower
			finalUpper[i] = basicUpper
			results[i] = finalLower[i]
			continue
		}
		prevClose := candles[i-1].EffectiveClose()
		if prevClose >= finalLower[i-1] {
			finalLower[i] = math.Max(basicLower, finalLower[i-1])
		} else {
			finalLower[i] = basicLower
		}
		if prevClose <= finalUpper[i-1] {
			finalUpper[i] = math.Min(basicUpper, finalUpper[i-1])
		} else {
			finalUpper[i] = basicUpper
		}
		closePrice := candle.EffectiveClose()
		if direction == -1 && closePrice > finalUpper[i-1] {
			direction = 1
		} else if direction == 1 && closePrice < finalLower[i-1] {
			direction = -1
		}
		if direction > 0 {
			results[i] = finalLower[i]
		} else {
			results[i] = finalUpper[i]
		}
	}
	n := len(results)
	current = results[n-1]
	if n >= 2 {
		prev = results[n-2]
	}
	return current, prev
}

// OBVSlope returns the linear regression slope of OBV over the last `period` candles,
// normalised by the mean OBV magnitude so it is comparable across instruments.
func OBVSlope(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 || period <= 0 {
		return 0
	}
	// Build OBV series.
	obvSeries := make([]float64, len(candles))
	running := 0.0
	for i := 1; i < len(candles); i++ {
		if candles[i].EffectiveClose() > candles[i-1].EffectiveClose() {
			running += candles[i].EffectiveVolume()
		} else if candles[i].EffectiveClose() < candles[i-1].EffectiveClose() {
			running -= candles[i].EffectiveVolume()
		}
		obvSeries[i] = running
	}
	window := lastValues(obvSeries, period)
	if len(window) < 2 {
		return 0
	}
	slope, _ := linearRegression(window, len(window))
	// Normalise by mean absolute OBV so slope is a unit-less direction indicator.
	mean := 0.0
	for _, v := range window {
		mean += math.Abs(v)
	}
	mean /= float64(len(window))
	return mathutil.SafeDiv(slope, math.Max(math.Abs(mean), mathutil.Epsilon))
}

func Donchian(candles []ohlcv.Candle, period int) (float64, float64) {
	if len(candles) == 0 {
		return 0, 0
	}
	start := len(candles) - period
	if start < 0 {
		start = 0
	}
	window := candles[start:]
	upper := window[0].EffectiveHigh()
	lower := window[0].EffectiveLow()
	for _, candle := range window[1:] {
		upper = math.Max(upper, candle.EffectiveHigh())
		lower = math.Min(lower, candle.EffectiveLow())
	}
	return upper, lower
}

func Keltner(candles []ohlcv.Candle, emaPeriod, atrPeriod int, multiplier float64) (float64, float64, float64) {
	if len(candles) == 0 {
		return 0, 0, 0
	}
	middle := EMA(closes(candles), emaPeriod)
	atr := ATR(candles, atrPeriod)
	return middle + multiplier*atr, middle, middle - multiplier*atr
}

func ChaikinMoneyFlow(candles []ohlcv.Candle, period int) float64 {
	if len(candles) == 0 {
		return 0
	}
	start := len(candles) - period
	if start < 0 {
		start = 0
	}
	mfv := 0.0
	volume := 0.0
	for _, candle := range candles[start:] {
		highLow := candle.EffectiveHigh() - candle.EffectiveLow()
		multiplier := mathutil.SafeDiv((candle.EffectiveClose()-candle.EffectiveLow())-(candle.EffectiveHigh()-candle.EffectiveClose()), highLow)
		mfv += multiplier * candle.EffectiveVolume()
		volume += candle.EffectiveVolume()
	}
	return mathutil.SafeDiv(mfv, volume)
}

func AccumulationDistribution(candles []ohlcv.Candle) float64 {
	value := 0.0
	for _, candle := range candles {
		multiplier := mathutil.SafeDiv((candle.EffectiveClose()-candle.EffectiveLow())-(candle.EffectiveHigh()-candle.EffectiveClose()), candle.EffectiveHigh()-candle.EffectiveLow())
		value += multiplier * candle.EffectiveVolume()
	}
	return value
}

func Ichimoku(candles []ohlcv.Candle, tenkanPeriod, kijunPeriod, senkouBPeriod int) (float64, float64, float64, float64, float64, float64, float64, float64, float64) {
	tenkanHigh, tenkanLow := highLowWindow(candles, tenkanPeriod)
	kijunHigh, kijunLow := highLowWindow(candles, kijunPeriod)
	senkouHigh, senkouLow := highLowWindow(candles, senkouBPeriod)
	tenkan := (tenkanHigh + tenkanLow) / 2
	kijun := (kijunHigh + kijunLow) / 2
	senkouA := (tenkan + kijun) / 2
	senkouB := (senkouHigh + senkouLow) / 2
	lastClose := 0.0
	if len(candles) > 0 {
		lastClose = candles[len(candles)-1].EffectiveClose()
	}
	// Chikou: current close vs. price kijunPeriod bars ago — positive means chikou above past price.
	chikou := 0.0
	if len(candles) > kijunPeriod {
		pastClose := candles[len(candles)-1-kijunPeriod].EffectiveClose()
		chikou = lastClose - pastClose
	}
	cloudTop := math.Max(senkouA, senkouB)
	cloudBottom := math.Min(senkouA, senkouB)
	cloudTrend := sign(senkouA - senkouB)
	kumoTwist := 0.0
	if len(candles) > 1 {
		prevCloudTrend := ichimokuCloudTrend(candles[:len(candles)-1], tenkanPeriod, kijunPeriod, senkouBPeriod)
		if prevCloudTrend != 0 && cloudTrend != 0 && prevCloudTrend != cloudTrend {
			kumoTwist = cloudTrend
		}
	}
	tkCross := sign(tenkan - kijun)
	breakout := 0.0
	if lastClose > cloudTop {
		breakout = 1
	} else if lastClose < cloudBottom {
		breakout = -1
	}
	return tenkan, kijun, senkouA, senkouB, chikou, cloudTrend, kumoTwist, tkCross, breakout
}

func ichimokuCloudTrend(candles []ohlcv.Candle, tenkanPeriod, kijunPeriod, senkouBPeriod int) float64 {
	tenkanHigh, tenkanLow := highLowWindow(candles, tenkanPeriod)
	kijunHigh, kijunLow := highLowWindow(candles, kijunPeriod)
	senkouHigh, senkouLow := highLowWindow(candles, senkouBPeriod)
	tenkan := (tenkanHigh + tenkanLow) / 2
	kijun := (kijunHigh + kijunLow) / 2
	senkouA := (tenkan + kijun) / 2
	senkouB := (senkouHigh + senkouLow) / 2
	return sign(senkouA - senkouB)
}

func PivotPoints(candles []ohlcv.Candle) (float64, float64, float64, float64, float64) {
	c, ok := pivotSourceCandle(candles)
	if !ok {
		return 0, 0, 0, 0, 0
	}
	pivot := (c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 3
	r1 := 2*pivot - c.EffectiveLow()
	s1 := 2*pivot - c.EffectiveHigh()
	r2 := pivot + (c.EffectiveHigh() - c.EffectiveLow())
	s2 := pivot - (c.EffectiveHigh() - c.EffectiveLow())
	return pivot, r1, r2, s1, s2
}

func pivotSourceCandle(candles []ohlcv.Candle) (ohlcv.Candle, bool) {
	if len(candles) == 0 {
		return ohlcv.Candle{}, false
	}
	if len(candles) > 1 {
		return candles[len(candles)-2], true
	}
	return candles[len(candles)-1], true
}

func FibonacciLevels(candles []ohlcv.Candle) map[string]float64 {
	levels := map[string]float64{"0.236": 0, "0.382": 0, "0.500": 0, "0.618": 0, "0.786": 0}
	if len(candles) == 0 {
		return levels
	}
	highest, lowest := highLowWindow(candles, minInt(120, len(candles)))
	diff := highest - lowest
	levels["0.236"] = highest - diff*0.236
	levels["0.382"] = highest - diff*0.382
	levels["0.500"] = highest - diff*0.5
	levels["0.618"] = highest - diff*0.618
	levels["0.786"] = highest - diff*0.786
	return levels
}

func AdditionalIndicators(candles []ohlcv.Candle) map[string]float64 {
	cls := closes(candles)
	vol := volumes(candles)
	result := map[string]float64{}
	result["Weighted Moving Average"] = WMA(cls, 20)
	result["Hull Moving Average"] = HMA(cls, 20)
	result["Arnaud Legoux Moving Average"] = ALMA(cls, 20, 0.85, 6)
	result["Kaufman Adaptive Moving Average"] = KAMA(cls, 10, 2, 30)
	result["TEMA"] = TEMA(cls, 20)
	result["DEMA"] = DEMA(cls, 20)
	result["TRIX"] = TRIX(cls, 15)
	result["Parabolic SAR"] = ParabolicSAR(candles)
	result["ZigZag"] = ZigZag(candles, 0.05)
	result["Linear Regression Line"] = LinearRegressionLine(cls, 20)
	result["Linear Regression Slope"] = LinearRegressionSlope(cls, 20)
	result["Moving Average Ribbon"] = MovingAverageRibbon(cls)
	result["Moving Average Envelope Upper"] = EMA(cls, 20) * 1.025
	result["Moving Average Envelope Lower"] = EMA(cls, 20) * 0.975
	aroonUp, aroonDown := Aroon(candles, 25)
	result["Aroon Up"] = aroonUp
	result["Aroon Down"] = aroonDown
	result["Aroon Oscillator"] = aroonUp - aroonDown
	vortexPlus, vortexMinus := Vortex(candles, 14)
	result["Vortex Indicator Plus"] = vortexPlus
	result["Vortex Indicator Minus"] = vortexMinus
	priceUpper, priceLower := Donchian(candles, 20)
	result["Price Channel Upper"] = priceUpper
	result["Price Channel Lower"] = priceLower
	cksLong, cksShort := ChandeKrollStop(candles, 10, 9, 1.5)
	result["Chande Kroll Stop Long"] = cksLong
	result["Chande Kroll Stop Short"] = cksShort
	result["Guppy Multiple Moving Average Short"] = (EMA(cls, 3) + EMA(cls, 5) + EMA(cls, 8) + EMA(cls, 10) + EMA(cls, 12) + EMA(cls, 15)) / 6
	result["Guppy Multiple Moving Average Long"] = (EMA(cls, 30) + EMA(cls, 35) + EMA(cls, 40) + EMA(cls, 45) + EMA(cls, 50) + EMA(cls, 60)) / 6
	result["Momentum"] = Momentum(cls, 10)
	result["Awesome Oscillator"] = AwesomeOscillator(candles)
	result["Ultimate Oscillator"] = UltimateOscillator(candles)
	result["True Strength Index"] = TrueStrengthIndex(cls, 25, 13)
	result["Chande Momentum Oscillator"] = ChandeMomentumOscillator(cls, 14)
	result["Know Sure Thing"] = KnowSureThing(cls)
	result["Relative Vigor Index"] = RelativeVigorIndex(candles, 10)
	bull, bear := ElderRay(candles, 13)
	result["Elder Ray Bull Power"] = bull
	result["Elder Ray Bear Power"] = bear
	result["Fisher Transform"] = FisherTransform(candles, 10)
	result["Schaff Trend Cycle"] = SchaffTrendCycle(cls)
	result["Connors RSI"] = ConnorsRSI(cls)
	result["Detrended Price Oscillator"] = DPO(cls, 20)
	result["Percentage Price Oscillator"] = PPO(cls, 12, 26)
	result["Commodity Channel Index"] = CCI(typicalPrices(candles), 20)
	result["Rate of Change"] = ROC(cls, 12)
	result["Highs/Lows(14)"] = HighsLows(candles, 14)
	result["Williams %R"] = WilliamsR(candles, 14)
	stochK, stochD := StochasticOscillator(candles, 14, 3)
	result["Stochastic Oscillator K"] = stochK
	result["Stochastic Oscillator D"] = stochD
	smi, smiSignal := StochasticMomentumIndex(candles, 14, 3)
	result["Stochastic Momentum Index"] = smi
	result["Stochastic Momentum Signal"] = smiSignal
	result["Ease of Movement"] = EaseOfMovement(candles, 14)
	result["Force Index"] = ForceIndex(candles, 13)
	result["Negative Volume Index"] = NegativeVolumeIndex(candles)
	result["Positive Volume Index"] = PositiveVolumeIndex(candles)
	result["Volume Price Trend"] = VolumePriceTrend(candles)
	result["Volume Weighted Moving Average"] = VWMA(candles, 20)
	result["Volume Oscillator"] = 100 * mathutil.SafeDiv(EMA(vol, 5)-EMA(vol, 20), EMA(vol, 20))
	klinger, klingerSignal := KlingerOscillator(candles)
	result["Klinger Oscillator"] = klinger
	result["Klinger Oscillator Signal"] = klingerSignal
	result["On Balance Volume"] = OBV(candles)
	result["Accumulation Distribution Line"] = AccumulationDistribution(candles)
	result["Chaikin Oscillator"] = ChaikinOscillator(candles)
	result["Chaikin Money Flow"] = ChaikinMoneyFlow(candles, 20)
	result["Money Flow Index"] = MFI(candles, 14)
	vp := VolumeProfile(candles, 24)
	result["Volume Profile Point of Control"] = vp["Point of Control"]
	result["Volume Profile Value Area High"] = vp["Value Area High"]
	result["Volume Profile Value Area Low"] = vp["Value Area Low"]
	result["Anchored VWAP"] = AnchoredVWAP(candles, maxInt(0, len(candles)-60))
	result["Long Term VWAP"] = VWAP(candles)
	result["Historical Volatility"] = HistoricalVolatility(cls, 20)
	result["Standard Deviation"] = mathutil.StdDev(lastValues(cls, 20))
	result["Average True Range Percent"] = 100 * mathutil.SafeDiv(ATR(candles, 14), absDenominator(last(cls)))
	result["Normalized ATR"] = mathutil.SafeDiv(ATR(candles, 14), absDenominator(SMA(cls, 20)))
	result["Chaikin Volatility"] = ChaikinVolatility(candles, 10, 10)
	result["Mass Index"] = MassIndex(candles, 25)
	result["Ulcer Index"] = UlcerIndex(cls, 14)
	result["Relative Volatility Index"] = RelativeVolatilityIndex(cls, 14)
	result["Bollinger Band Width"] = BollingerBandWidth(cls, 20, 2)
	result["Bollinger %B"] = BollingerPercentB(cls, 20, 2)
	result["Keltner Channel Upper"], result["Keltner Channel Middle"], result["Keltner Channel Lower"] = Keltner(candles, 20, 10, 2)
	result["Donchian Channel Upper"], result["Donchian Channel Lower"] = Donchian(candles, 20)
	result["Hilbert Transform Trendline"] = ALMA(cls, 20, 0.5, 3)
	result["Hilbert Transform Dominant Cycle Period"] = DominantCyclePeriod(cls)
	result["Hilbert Transform Dominant Cycle Phase"] = DominantCyclePhase(cls)
	result["Sine Wave"] = math.Sin(DominantCyclePhase(cls))
	return result
}

func SupportTools(candles []ohlcv.Candle) map[string]float64 {
	classicLevels := ClassicPivotLevels(candles)
	camarilla := CamarillaPivots(candles)
	fib := FibonacciPivots(candles)
	woodieLevels := WoodiePivotLevels(candles)
	demarkLevels := DeMarkPivotLevels(candles)
	extensions := FibonacciExtensions(candles)
	profile := VolumeProfile(candles, 24)
	result := map[string]float64{
		"Camarilla Pivot Points H3":  camarilla["H3"],
		"Camarilla Pivot Points L3":  camarilla["L3"],
		"Woodie Pivot Points":        woodieLevels["P"],
		"Fibonacci Pivot Points R1":  fib["R1"],
		"Fibonacci Pivot Points S1":  fib["S1"],
		"DeMark Pivot Points":        demarkLevels["P"],
		"Fibonacci Extension 1.272":  extensions["1.272"],
		"Fibonacci Projection 1.618": extensions["1.618"],
		"Fibonacci Fan 0.618":        extensions["fan_0.618"],
		"Fibonacci Time Zones":       float64(nextFibWindow(len(candles))),
		"Gann Fan 1x1":               LinearRegressionLine(closes(candles), 45),
		"Gann Square": func() float64 {
			sr := math.Sqrt(math.Max(last(closes(candles)), 0))
			nearest := math.Round(sr*2) / 2
			return nearest * nearest
		}(),
		"Murrey Math Lines":               MurreyMathLine(candles),
		"Market Profile Point of Control": profile["Point of Control"],
		"Volume Profile Visible Range":    profile["Visible Range"],
		"Point of Control":                profile["Point of Control"],
		"Value Area High":                 profile["Value Area High"],
		"Value Area Low":                  profile["Value Area Low"],
		"Anchored Volume Profile":         AnchoredVWAP(candles, maxInt(0, len(candles)-100)),
		"Classic Pivot Points":            classicLevels["P"],
	}
	addPivotLevels(result, "Classic Pivot Points", classicLevels)
	addPivotLevels(result, "Fibonacci Pivot Points", fib)
	addPivotLevels(result, "Camarilla Pivot Points", camarilla)
	addPivotLevels(result, "Woodie Pivot Points", woodieLevels)
	addPivotLevels(result, "DeMark Pivot Points", demarkLevels)
	return result
}

func addPivotLevels(result map[string]float64, prefix string, levels map[string]float64) {
	for _, key := range []string{"S3", "S2", "S1", "P", "R1", "R2", "R3"} {
		if value, ok := levels[key]; ok {
			label := key
			if key == "P" {
				label = "Pivot"
			}
			result[prefix+" "+label] = value
		}
	}
}

func MarketStructure(candles []ohlcv.Candle) map[string]float64 {
	swings := recentSwingValues(candles, 80)
	highTrend := 0.0
	lowTrend := 0.0
	if len(swings.highs) >= 2 && swings.highs[len(swings.highs)-1] > swings.highs[len(swings.highs)-2] {
		highTrend = 1
	} else if len(swings.highs) >= 2 {
		highTrend = -1
	}
	if len(swings.lows) >= 2 && swings.lows[len(swings.lows)-1] > swings.lows[len(swings.lows)-2] {
		lowTrend = 1
	} else if len(swings.lows) >= 2 {
		lowTrend = -1
	}
	lastClose := last(closes(candles))
	prevHigh := priorWindowHigh(candles, 20)
	prevLow := priorWindowLow(candles, 20)
	bullishBOS := boolScore(prevHigh > 0 && lastClose > prevHigh)
	bearishBOS := boolScore(prevLow > 0 && lastClose < prevLow)
	fvg := FairValueGapScore(candles)
	return map[string]float64{
		"Higher High Higher Low Detection": boolScore(highTrend > 0 && lowTrend > 0),
		"Lower High Lower Low Detection":   boolScore(highTrend < 0 && lowTrend < 0),
		"Break of Structure":               math.Max(bullishBOS, bearishBOS),
		"Bullish Break of Structure":       bullishBOS,
		"Bearish Break of Structure":       bearishBOS,
		"Change of Character":              boolScore(highTrend != 0 && lowTrend != 0 && highTrend != lowTrend),
		"Liquidity Sweep":                  LiquiditySweepScore(candles),
		"Fair Value Gap":                   fvg,
		"Order Block":                      OrderBlockScore(candles),
		"Breaker Block":                    boolScore(fvg > 0.5 && LiquiditySweepScore(candles) > 0.5),
		"Mitigation Block":                 mathutil.Clamp(OrderBlockScore(candles)*0.7, 0, 1),
		"Supply Zone":                      prevHigh,
		"Demand Zone":                      prevLow,
		"Premium Discount Zone":            PremiumDiscount(candles),
		"Imbalance Detection":              fvg,
	}
}

func priorWindowHigh(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 || period <= 0 {
		return 0
	}
	start := maxInt(0, len(candles)-1-period)
	high := candles[start].EffectiveHigh()
	for _, candle := range candles[start : len(candles)-1] {
		high = math.Max(high, candle.EffectiveHigh())
	}
	return high
}

func priorWindowLow(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 || period <= 0 {
		return 0
	}
	start := maxInt(0, len(candles)-1-period)
	low := candles[start].EffectiveLow()
	for _, candle := range candles[start : len(candles)-1] {
		low = math.Min(low, candle.EffectiveLow())
	}
	return low
}

func RelativeStrengthMetrics(candles []ohlcv.Candle) map[string]float64 {
	cls := closes(candles)
	return map[string]float64{
		"Rolling Return 20":       ROC(cls, 20),
		"Rolling Return 60":       ROC(cls, 60),
		"Rolling Sharpe Ratio":    RollingSharpe(cls, 60),
		"Rolling Sortino Ratio":   RollingSortino(cls, 60),
		"Rolling Max Drawdown":    RollingMaxDrawdown(cls, 120),
		"Return Volatility 20":    returnVolatility(cls, 20),
		"Return Volatility 60":    returnVolatility(cls, 60),
		"Downside Volatility 60":  downsideVolatility(cls, 60),
		"Ulcer Index":             UlcerIndex(cls, 120),
		"Risk Adjusted Return 60": mathutil.SafeDiv(ROC(cls, 60)/100, returnVolatility(cls, 60)),
	}
}

func WMA(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) == 0 {
		return 0
	}
	sum := 0.0
	weight := 0.0
	for i, value := range window {
		w := float64(i + 1)
		sum += value * w
		weight += w
	}
	return mathutil.SafeDiv(sum, weight)
}

func HMA(values []float64, period int) float64 {
	if len(values) == 0 || period <= 0 {
		return 0
	}
	half := maxInt(1, period/2)
	sqrtPeriod := maxInt(1, int(math.Sqrt(float64(period))))
	diff := make([]float64, len(values))
	for i := range values {
		diff[i] = 2*WMA(values[:i+1], half) - WMA(values[:i+1], period)
	}
	return WMA(diff, sqrtPeriod)
}

func ALMA(values []float64, period int, offset, sigma float64) float64 {
	window := lastValues(values, period)
	if len(window) == 0 {
		return 0
	}
	m := offset * float64(len(window)-1)
	s := float64(len(window)) / sigma
	sum := 0.0
	weight := 0.0
	for i, value := range window {
		w := math.Exp(-math.Pow(float64(i)-m, 2) / (2 * s * s))
		sum += value * w
		weight += w
	}
	return mathutil.SafeDiv(sum, weight)
}

func KAMA(values []float64, period, fast, slow int) float64 {
	if len(values) == 0 {
		return 0
	}
	kama := values[0]
	fastSC := 2.0 / float64(fast+1)
	slowSC := 2.0 / float64(slow+1)
	for i := 1; i < len(values); i++ {
		start := i - period
		if start < 0 {
			start = 0
		}
		change := math.Abs(values[i] - values[start])
		volatility := 0.0
		for j := start + 1; j <= i; j++ {
			volatility += math.Abs(values[j] - values[j-1])
		}
		er := mathutil.SafeDiv(change, volatility)
		sc := math.Pow(er*(fastSC-slowSC)+slowSC, 2)
		kama += sc * (values[i] - kama)
	}
	return kama
}

func DEMA(values []float64, period int) float64 {
	ema1 := EMASeries(values, period)
	if len(ema1) < period {
		return 0
	}
	ema2 := EMASeries(ema1[period-1:], period)
	if len(ema2) == 0 {
		return 0
	}
	return 2*ema1[len(ema1)-1] - ema2[len(ema2)-1]
}

func TEMA(values []float64, period int) float64 {
	ema1 := EMASeries(values, period)
	if len(ema1) < period {
		return 0
	}
	ema2 := EMASeries(ema1[period-1:], period)
	if len(ema2) < period {
		return 0
	}
	ema3 := EMASeries(ema2[period-1:], period)
	if len(ema3) == 0 {
		return 0
	}
	return 3*ema1[len(ema1)-1] - 3*ema2[len(ema2)-1] + ema3[len(ema3)-1]
}

func TRIX(values []float64, period int) float64 {
	ema1 := EMASeries(values, period)
	if len(ema1) < period {
		return 0
	}
	ema2 := EMASeries(ema1[period-1:], period)
	if len(ema2) < period {
		return 0
	}
	ema3 := EMASeries(ema2[period-1:], period)
	if len(ema3) < 2 {
		return 0
	}
	return 100 * mathutil.SafeDiv(ema3[len(ema3)-1]-ema3[len(ema3)-2], absDenominator(ema3[len(ema3)-2]))
}

func ParabolicSAR(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	sar := candles[0].EffectiveLow()
	ep := candles[0].EffectiveHigh()
	af := 0.02
	up := true
	for i := 1; i < len(candles); i++ {
		sar = sar + af*(ep-sar)
		if up {
			// SAR must not be placed above the prior two bars' lows (Wilder rule)
			if sar > candles[i-1].EffectiveLow() {
				sar = candles[i-1].EffectiveLow()
			}
			if i >= 2 && sar > candles[i-2].EffectiveLow() {
				sar = candles[i-2].EffectiveLow()
			}
			if candles[i].EffectiveLow() < sar {
				up = false
				sar = ep
				ep = candles[i].EffectiveLow()
				af = 0.02
			} else {
				if candles[i].EffectiveHigh() > ep {
					ep = candles[i].EffectiveHigh()
					af = math.Min(af+0.02, 0.20)
				}
			}
		} else {
			// SAR must not be placed below the prior two bars' highs (Wilder rule)
			if sar < candles[i-1].EffectiveHigh() {
				sar = candles[i-1].EffectiveHigh()
			}
			if i >= 2 && sar < candles[i-2].EffectiveHigh() {
				sar = candles[i-2].EffectiveHigh()
			}
			if candles[i].EffectiveHigh() > sar {
				up = true
				sar = ep
				ep = candles[i].EffectiveHigh()
				af = 0.02
			} else {
				if candles[i].EffectiveLow() < ep {
					ep = candles[i].EffectiveLow()
					af = math.Min(af+0.02, 0.20)
				}
			}
		}
	}
	return sar
}

func ZigZag(candles []ohlcv.Candle, threshold float64) float64 {
	if len(candles) == 0 {
		return 0
	}
	pivot := candles[0].EffectiveClose()
	for _, candle := range candles[1:] {
		closePrice := candle.EffectiveClose()
		change := mathutil.SafeDiv(closePrice-pivot, absDenominator(pivot))
		if math.Abs(change) >= threshold {
			pivot = closePrice
		}
	}
	return pivot
}

func LinearRegressionLine(values []float64, period int) float64 {
	slope, intercept := linearRegression(values, period)
	window := lastValues(values, period)
	if len(window) == 0 {
		return 0
	}
	return intercept + slope*float64(len(window)-1)
}

func LinearRegressionSlope(values []float64, period int) float64 {
	slope, _ := linearRegression(values, period)
	return slope
}

func linearRegression(values []float64, period int) (float64, float64) {
	window := lastValues(values, period)
	n := float64(len(window))
	if n == 0 {
		return 0, 0
	}
	sumX, sumY, sumXY, sumXX := 0.0, 0.0, 0.0, 0.0
	for i, y := range window {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	slope := mathutil.SafeDiv(n*sumXY-sumX*sumY, n*sumXX-sumX*sumX)
	intercept := mathutil.SafeDiv(sumY-slope*sumX, n)
	return slope, intercept
}

func MovingAverageRibbon(values []float64) float64 {
	short := (EMA(values, 5) + EMA(values, 8) + EMA(values, 13)) / 3
	long := (EMA(values, 21) + EMA(values, 34) + EMA(values, 55)) / 3
	return mathutil.SafeDiv(short-long, absDenominator(long)) * 100
}

func Aroon(candles []ohlcv.Candle, period int) (float64, float64) {
	if len(candles) < 2 || period <= 0 {
		return 0, 0
	}
	p := period
	if len(candles) < p+1 {
		p = len(candles) - 1
	}
	windowHighs := lastValues(highs(candles), p+1)
	windowLows := lastValues(lows(candles), p+1)

	highIdx, lowIdx := 0, 0
	for i := 0; i < len(windowHighs); i++ {
		if windowHighs[i] >= windowHighs[highIdx] {
			highIdx = i
		}
		if windowLows[i] <= windowLows[lowIdx] {
			lowIdx = i
		}
	}

	daysSinceHigh := len(windowHighs) - 1 - highIdx
	daysSinceLow := len(windowLows) - 1 - lowIdx

	aroonUp := 100.0 * float64(p-daysSinceHigh) / float64(p)
	aroonDown := 100.0 * float64(p-daysSinceLow) / float64(p)

	return aroonUp, aroonDown
}

func Vortex(candles []ohlcv.Candle, period int) (float64, float64) {
	if len(candles) < 2 {
		return 0, 0
	}
	start := maxInt(1, len(candles)-period)
	plus, minus, tr := 0.0, 0.0, 0.0
	for i := start; i < len(candles); i++ {
		plus += math.Abs(candles[i].EffectiveHigh() - candles[i-1].EffectiveLow())
		minus += math.Abs(candles[i].EffectiveLow() - candles[i-1].EffectiveHigh())
		prevClose := candles[i-1].EffectiveClose()
		tr += math.Max(candles[i].EffectiveHigh()-candles[i].EffectiveLow(), math.Max(math.Abs(candles[i].EffectiveHigh()-prevClose), math.Abs(candles[i].EffectiveLow()-prevClose)))
	}
	return mathutil.SafeDiv(plus, tr), mathutil.SafeDiv(minus, tr)
}

func ChandeKrollStop(candles []ohlcv.Candle, period, stopPeriod int, multiplier float64) (float64, float64) {
	n := len(candles)
	if n < period {
		return 0, 0
	}
	atrSeries := ATRSeries(candles, period)
	// Step 1: firstStopShort = Highest(High, period) - multiplier*ATR
	//         firstStopLong  = Lowest(Low,  period) + multiplier*ATR
	firstShort := make([]float64, n)
	firstLong := make([]float64, n)
	for i := period - 1; i < n; i++ {
		hh := candles[i-period+1].EffectiveHigh()
		ll := candles[i-period+1].EffectiveLow()
		for j := i - period + 2; j <= i; j++ {
			if candles[j].EffectiveHigh() > hh {
				hh = candles[j].EffectiveHigh()
			}
			if candles[j].EffectiveLow() < ll {
				ll = candles[j].EffectiveLow()
			}
		}
		firstShort[i] = hh - multiplier*atrSeries[i]
		firstLong[i] = ll + multiplier*atrSeries[i]
	}
	// Step 2: stopShort (resistance) = Highest(firstShort, stopPeriod)
	//         stopLong  (support)    = Lowest(firstLong,  stopPeriod)
	validShort := firstShort[period-1:]
	validLong := firstLong[period-1:]
	stopLong := mathutil.Min(lastValues(validLong, stopPeriod))
	stopShort := mathutil.Max(lastValues(validShort, stopPeriod))
	return stopLong, stopShort
}

func Momentum(values []float64, period int) float64 {
	if len(values) <= period {
		return 0
	}
	return values[len(values)-1] - values[len(values)-1-period]
}

func AwesomeOscillator(candles []ohlcv.Candle) float64 {
	median := make([]float64, len(candles))
	for i, candle := range candles {
		median[i] = (candle.EffectiveHigh() + candle.EffectiveLow()) / 2
	}
	return SMA(median, 5) - SMA(median, 34)
}

func UltimateOscillator(candles []ohlcv.Candle) float64 {
	return ultimate(candles, 7, 14, 28)
}

func ultimate(candles []ohlcv.Candle, p1, p2, p3 int) float64 {
	bp := make([]float64, len(candles))
	tr := make([]float64, len(candles))
	for i := range candles {
		prevClose := candles[i].EffectiveClose()
		if i > 0 {
			prevClose = candles[i-1].EffectiveClose()
		}
		bp[i] = candles[i].EffectiveClose() - math.Min(candles[i].EffectiveLow(), prevClose)
		tr[i] = math.Max(candles[i].EffectiveHigh(), prevClose) - math.Min(candles[i].EffectiveLow(), prevClose)
	}
	avg1 := mathutil.SafeDiv(sum(lastValues(bp, p1)), sum(lastValues(tr, p1)))
	avg2 := mathutil.SafeDiv(sum(lastValues(bp, p2)), sum(lastValues(tr, p2)))
	avg3 := mathutil.SafeDiv(sum(lastValues(bp, p3)), sum(lastValues(tr, p3)))
	return 100 * mathutil.SafeDiv(4*avg1+2*avg2+avg3, 7)
}

func TrueStrengthIndex(values []float64, longPeriod, shortPeriod int) float64 {
	if len(values) < 2 {
		return 0
	}
	momentum := make([]float64, len(values)-1)
	absMomentum := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		momentum[i-1] = values[i] - values[i-1]
		absMomentum[i-1] = math.Abs(momentum[i-1])
	}
	ema1 := EMASeries(momentum, longPeriod)
	ema1Abs := EMASeries(absMomentum, longPeriod)
	if len(ema1) < longPeriod {
		return 0
	}
	double := EMASeries(ema1[longPeriod-1:], shortPeriod)
	doubleAbs := EMASeries(ema1Abs[longPeriod-1:], shortPeriod)
	if len(double) == 0 {
		return 0
	}
	return 100 * mathutil.SafeDiv(double[len(double)-1], doubleAbs[len(doubleAbs)-1])
}

func ChandeMomentumOscillator(values []float64, period int) float64 {
	if len(values) < 2 {
		return 0
	}
	start := maxInt(1, len(values)-period)
	up, down := 0.0, 0.0
	for i := start; i < len(values); i++ {
		change := values[i] - values[i-1]
		if change >= 0 {
			up += change
		} else {
			down -= change
		}
	}
	return 100 * mathutil.SafeDiv(up-down, up+down)
}

func KnowSureThing(values []float64) float64 {
	if len(values) < 50 {
		return 0
	}
	// Compact ROC series — no leading zeros that would corrupt SMA averaging.
	rocSeries := func(v []float64, period int) []float64 {
		out := make([]float64, 0, len(v)-period)
		for i := period; i < len(v); i++ {
			old := v[i-period]
			out = append(out, 100*mathutil.SafeDiv(v[i]-old, absDenominator(old)))
		}
		return out
	}
	rcma1 := SMA(rocSeries(values, 10), 10)
	rcma2 := SMA(rocSeries(values, 15), 13)
	rcma3 := SMA(rocSeries(values, 20), 15)
	rcma4 := SMA(rocSeries(values, 30), 20)
	return rcma1 + 2*rcma2 + 3*rcma3 + 4*rcma4
}

func RelativeVigorIndex(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 4 || period <= 0 {
		return 0
	}
	n := len(candles)
	val := make([]float64, n)
	ran := make([]float64, n)
	for i := 3; i < n; i++ {
		a := candles[i].EffectiveClose() - candles[i].EffectiveOpen()
		b := candles[i-1].EffectiveClose() - candles[i-1].EffectiveOpen()
		c := candles[i-2].EffectiveClose() - candles[i-2].EffectiveOpen()
		d := candles[i-3].EffectiveClose() - candles[i-3].EffectiveOpen()
		val[i] = (a + 2*b + 2*c + d) / 6.0

		e := candles[i].EffectiveHigh() - candles[i].EffectiveLow()
		f := candles[i-1].EffectiveHigh() - candles[i-1].EffectiveLow()
		g := candles[i-2].EffectiveHigh() - candles[i-2].EffectiveLow()
		h := candles[i-3].EffectiveHigh() - candles[i-3].EffectiveLow()
		ran[i] = (e + 2*f + 2*g + h) / 6.0
	}

	rviRatio := make([]float64, n)
	for i := 3; i < n; i++ {
		if ran[i] == 0 {
			rviRatio[i] = 0
		} else {
			rviRatio[i] = val[i] / ran[i]
		}
	}

	return SMA(rviRatio, period)
}

func ElderRay(candles []ohlcv.Candle, period int) (float64, float64) {
	if len(candles) == 0 {
		return 0, 0
	}
	ema := EMA(closes(candles), period)
	lastCandle := candles[len(candles)-1]
	return lastCandle.EffectiveHigh() - ema, lastCandle.EffectiveLow() - ema
}

func FisherTransform(candles []ohlcv.Candle, period int) float64 {
	if len(candles) == 0 {
		return 0
	}
	high, low := highLowWindow(candles, period)
	value := 2*mathutil.SafeDiv(candles[len(candles)-1].EffectiveClose()-low, high-low) - 1
	value = mathutil.Clamp(value, -0.999, 0.999)
	return 0.5 * math.Log((1+value)/(1-value))
}

func SchaffTrendCycle(values []float64) float64 {
	if len(values) == 0 {
		return 50
	}
	macdLine := macdLineSeries(values, 23, 50)
	firstK := stochasticPercentSeries(macdLine, 10, 50)
	firstD := EMASeries(firstK, 3)
	secondK := stochasticPercentSeries(firstD, 10, 50)
	stc := EMA(secondK, 3)
	return mathutil.Clamp(stc, 0, 100)
}

func macdLineSeries(values []float64, fast, slow int) []float64 {
	if len(values) == 0 || slow > len(values) {
		return nil
	}
	fastEMA := EMASeries(values, fast)
	slowEMA := EMASeries(values, slow)
	out := make([]float64, len(values))
	for i := slow - 1; i < len(values); i++ {
		out[i] = fastEMA[i] - slowEMA[i]
	}
	return out
}

func stochasticPercentSeries(values []float64, period int, neutral float64) []float64 {
	out := make([]float64, len(values))
	for i := range values {
		start := i - period + 1
		if start < 0 {
			start = 0
		}
		window := values[start : i+1]
		low := mathutil.Min(window)
		high := mathutil.Max(window)
		if mathutil.AlmostEqual(high, low) {
			out[i] = neutral
			continue
		}
		out[i] = mathutil.Clamp(100*mathutil.SafeDiv(values[i]-low, high-low), 0, 100)
	}
	return out
}

func ConnorsRSI(values []float64) float64 {
	return (RSI(values, 3) + RSI(streakSeries(values), 2) + percentileRank(values, 100)) / 3
}

func DPO(values []float64, period int) float64 {
	shift := period/2 + 1
	// Need at least period+shift values: shift bars ago we must still have a full period window.
	if len(values) < period+shift {
		return 0
	}
	// Historical close at the shifted index.
	idx := len(values) - 1 - shift
	historicalClose := values[idx]
	// SMA of the same period ending at the shifted index.
	historicalSMA := SMA(values[:idx+1], period)
	return historicalClose - historicalSMA
}

func PPO(values []float64, fast, slow int) float64 {
	slowEMA := EMA(values, slow)
	return 100 * mathutil.SafeDiv(EMA(values, fast)-slowEMA, absDenominator(slowEMA))
}

func StochasticMomentumIndex(candles []ohlcv.Candle, period, smooth int) (float64, float64) {
	if len(candles) == 0 {
		return 0, 0
	}
	values := make([]float64, len(candles))
	for i := range candles {
		start := maxInt(0, i-period+1)
		highest := mathutil.Max(highs(candles[start : i+1]))
		lowest := mathutil.Min(lows(candles[start : i+1]))
		mid := (highest + lowest) / 2
		denominator := (highest - lowest) / 2
		if mathutil.AlmostEqual(denominator, 0) {
			values[i] = 0
		} else {
			values[i] = 100 * (candles[i].EffectiveClose() - mid) / denominator
		}
	}
	return EMA(values, smooth), EMA(EMASeries(values, smooth), smooth)
}

func EaseOfMovement(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 {
		return 0
	}
	values := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		midMove := ((candles[i].EffectiveHigh()+candles[i].EffectiveLow())/2 - (candles[i-1].EffectiveHigh()+candles[i-1].EffectiveLow())/2)
		boxRatio := mathutil.SafeDiv(candles[i].EffectiveVolume(), candles[i].EffectiveHigh()-candles[i].EffectiveLow())
		values = append(values, mathutil.SafeDiv(midMove, boxRatio))
	}
	return SMA(values, period)
}

func ForceIndex(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 {
		return 0
	}
	values := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		values[i-1] = (candles[i].EffectiveClose() - candles[i-1].EffectiveClose()) * candles[i].EffectiveVolume()
	}
	return EMA(values, period)
}

func NegativeVolumeIndex(candles []ohlcv.Candle) float64 {
	return volumeIndex(candles, false)
}

func PositiveVolumeIndex(candles []ohlcv.Candle) float64 {
	return volumeIndex(candles, true)
}

func volumeIndex(candles []ohlcv.Candle, positive bool) float64 {
	if len(candles) == 0 {
		return 1000
	}
	value := 1000.0
	for i := 1; i < len(candles); i++ {
		volumeUp := candles[i].EffectiveVolume() > candles[i-1].EffectiveVolume()
		if volumeUp == positive {
			value += value * mathutil.SafeDiv(candles[i].EffectiveClose()-candles[i-1].EffectiveClose(), absDenominator(candles[i-1].EffectiveClose()))
		}
	}
	return value
}

func VolumePriceTrend(candles []ohlcv.Candle) float64 {
	value := 0.0
	for i := 1; i < len(candles); i++ {
		value += candles[i].EffectiveVolume() * mathutil.SafeDiv(candles[i].EffectiveClose()-candles[i-1].EffectiveClose(), absDenominator(candles[i-1].EffectiveClose()))
	}
	return value
}

func VWMA(candles []ohlcv.Candle, period int) float64 {
	start := maxInt(0, len(candles)-period)
	pv, volume := 0.0, 0.0
	for _, candle := range candles[start:] {
		pv += candle.EffectiveClose() * candle.EffectiveVolume()
		volume += candle.EffectiveVolume()
	}
	return mathutil.SafeDiv(pv, volume)
}

func KlingerOscillator(candles []ohlcv.Candle) (float64, float64) {
	if len(candles) < 2 {
		return 0, 0
	}
	force := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		trend := sign((candles[i].EffectiveHigh() + candles[i].EffectiveLow() + candles[i].EffectiveClose()) - (candles[i-1].EffectiveHigh() + candles[i-1].EffectiveLow() + candles[i-1].EffectiveClose()))
		force[i] = candles[i].EffectiveVolume() * trend * math.Abs(candles[i].EffectiveHigh()-candles[i].EffectiveLow())
	}
	ema34 := EMASeries(force, 34)
	ema55 := EMASeries(force, 55)
	if ema34 == nil || ema55 == nil {
		return 0, 0
	}
	oscSeries := make([]float64, len(candles))
	for i := range candles {
		oscSeries[i] = ema34[i] - ema55[i]
	}
	signalSeries := EMASeries(oscSeries, 13)
	if signalSeries == nil {
		return oscSeries[len(oscSeries)-1], 0
	}
	return oscSeries[len(oscSeries)-1], signalSeries[len(signalSeries)-1]
}

func ChaikinOscillator(candles []ohlcv.Candle) float64 {
	ad := make([]float64, len(candles))
	running := 0.0
	for i, candle := range candles {
		running += mathutil.SafeDiv((candle.EffectiveClose()-candle.EffectiveLow())-(candle.EffectiveHigh()-candle.EffectiveClose()), candle.EffectiveHigh()-candle.EffectiveLow()) * candle.EffectiveVolume()
		ad[i] = running
	}
	return EMA(ad, 3) - EMA(ad, 10)
}

func VolumeProfile(candles []ohlcv.Candle, bins int) map[string]float64 {
	result := map[string]float64{"Point of Control": 0, "Value Area High": 0, "Value Area Low": 0, "Visible Range": 0}
	if len(candles) == 0 || bins <= 0 {
		return result
	}
	high := mathutil.Max(highs(candles))
	low := mathutil.Min(lows(candles))
	width := mathutil.SafeDiv(high-low, float64(bins))
	if mathutil.AlmostEqual(width, 0) {
		result["Point of Control"] = last(closes(candles))
		result["Value Area High"] = high
		result["Value Area Low"] = low
		return result
	}
	buckets := make([]float64, bins)
	for _, candle := range candles {
		idx := int(mathutil.Clamp(math.Floor((candle.EffectiveClose()-low)/width), 0, float64(bins-1)))
		buckets[idx] += candle.EffectiveVolume()
	}
	pocIdx := 0
	for i := range buckets {
		if buckets[i] > buckets[pocIdx] {
			pocIdx = i
		}
	}
	result["Point of Control"] = low + (float64(pocIdx)+0.5)*width
	result["Value Area High"] = low + (float64(minInt(bins-1, pocIdx+int(float64(bins)*0.15)))+0.5)*width
	result["Value Area Low"] = low + (float64(maxInt(0, pocIdx-int(float64(bins)*0.15)))+0.5)*width
	result["Visible Range"] = high - low
	return result
}

func AnchoredVWAP(candles []ohlcv.Candle, anchor int) float64 {
	if len(candles) == 0 {
		return 0
	}
	if anchor < 0 || anchor >= len(candles) {
		anchor = 0
	}
	return VWAP(candles[anchor:])
}

func HistoricalVolatility(values []float64, period int) float64 {
	window := lastValues(values, period+1)
	if len(window) < 2 {
		return 0
	}
	logRets := make([]float64, len(window)-1)
	for i := 1; i < len(window); i++ {
		if window[i-1] > 0 && window[i] > 0 {
			logRets[i-1] = math.Log(window[i] / window[i-1])
		}
	}
	return mathutil.StdDev(logRets) * math.Sqrt(252) * 100
}

func ChaikinVolatility(candles []ohlcv.Candle, emaPeriod, rocPeriod int) float64 {
	ranges := make([]float64, len(candles))
	for i, candle := range candles {
		ranges[i] = candle.EffectiveHigh() - candle.EffectiveLow()
	}
	ema := EMASeries(ranges, emaPeriod)
	return ROC(ema, rocPeriod)
}

func MassIndex(candles []ohlcv.Candle, period int) float64 {
	ranges := make([]float64, len(candles))
	for i, candle := range candles {
		ranges[i] = candle.EffectiveHigh() - candle.EffectiveLow()
	}
	ema1 := EMASeries(ranges, 9)
	if len(ema1) < 9 {
		return 0
	}
	validEma1 := ema1[8:] // start from first valid EMA value (index = period-1)
	ema2 := EMASeries(validEma1, 9)
	ratio := make([]float64, len(ema2))
	for i := range ema2 {
		ratio[i] = mathutil.SafeDiv(validEma1[i], ema2[i])
	}
	return sum(lastValues(ratio, period))
}

func UlcerIndex(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) == 0 {
		return 0
	}
	peak := window[0]
	sumSquares := 0.0
	for _, value := range window {
		peak = math.Max(peak, value)
		drawdown := 100 * mathutil.SafeDiv(value-peak, absDenominator(peak))
		sumSquares += drawdown * drawdown
	}
	return math.Sqrt(sumSquares / float64(len(window)))
}

func RelativeVolatilityIndex(values []float64, period int) float64 {
	n := len(values)
	if n < period+1 {
		return 50
	}
	// Dorsey RVI: rolling StdDev of closes, split by direction, smoothed with EMA
	upSeries := make([]float64, n-period)
	downSeries := make([]float64, n-period)
	for i := period; i < n; i++ {
		sd := mathutil.StdDev(values[i-period : i])
		if values[i] > values[i-1] {
			upSeries[i-period] = sd
		} else {
			downSeries[i-period] = sd
		}
	}
	emaUp := EMASeries(upSeries, period)
	emaDown := EMASeries(downSeries, period)
	if len(emaUp) == 0 || len(emaDown) == 0 {
		return 50
	}
	u := emaUp[len(emaUp)-1]
	d := emaDown[len(emaDown)-1]
	return 100 * mathutil.SafeDiv(u, u+d)
}

func BollingerBandWidth(values []float64, period int, multiplier float64) float64 {
	upper, middle, lower := BollingerBands(values, period, multiplier)
	return 100 * mathutil.SafeDiv(upper-lower, absDenominator(middle))
}

func BollingerPercentB(values []float64, period int, multiplier float64) float64 {
	upper, _, lower := BollingerBands(values, period, multiplier)
	if mathutil.AlmostEqual(upper, lower) {
		return 0.5
	}
	return (last(values) - lower) / (upper - lower)
}

func CamarillaPivots(candles []ohlcv.Candle) map[string]float64 {
	levels := CamarillaPivotLevels(candles)
	out := map[string]float64{"H3": levels["R3"], "L3": levels["S3"]}
	for key, value := range levels {
		out[key] = value
	}
	return out
}

func WoodiePivots(candles []ohlcv.Candle) float64 {
	return WoodiePivotLevels(candles)["P"]
}

func FibonacciPivots(candles []ohlcv.Candle) map[string]float64 {
	c, ok := pivotSourceCandle(candles)
	out := map[string]float64{"S3": 0, "S2": 0, "S1": 0, "P": 0, "R1": 0, "R2": 0, "R3": 0}
	if !ok {
		return out
	}
	p, _, _, _, _ := PivotPoints(candles)
	r := c.EffectiveHigh() - c.EffectiveLow()
	out["R1"] = p + 0.382*r
	out["R2"] = p + 0.618*r
	out["R3"] = p + r
	out["P"] = p
	out["S1"] = p - 0.382*r
	out["S2"] = p - 0.618*r
	out["S3"] = p - r
	return out
}

func DeMarkPivot(candles []ohlcv.Candle) float64 {
	return DeMarkPivotLevels(candles)["P"]
}

func ClassicPivotLevels(candles []ohlcv.Candle) map[string]float64 {
	c, ok := pivotSourceCandle(candles)
	out := map[string]float64{"S3": 0, "S2": 0, "S1": 0, "P": 0, "R1": 0, "R2": 0, "R3": 0}
	if !ok {
		return out
	}
	p, r1, r2, s1, s2 := PivotPoints(candles)
	out["P"] = p
	out["R1"] = r1
	out["R2"] = r2
	out["R3"] = c.EffectiveHigh() + 2*(p-c.EffectiveLow())
	out["S1"] = s1
	out["S2"] = s2
	out["S3"] = c.EffectiveLow() - 2*(c.EffectiveHigh()-p)
	return out
}

func CamarillaPivotLevels(candles []ohlcv.Candle) map[string]float64 {
	c, ok := pivotSourceCandle(candles)
	out := map[string]float64{"S3": 0, "S2": 0, "S1": 0, "P": 0, "R1": 0, "R2": 0, "R3": 0}
	if !ok {
		return out
	}
	r := c.EffectiveHigh() - c.EffectiveLow()
	out["P"] = (c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 3
	out["R1"] = c.EffectiveClose() + r*1.1/12
	out["R2"] = c.EffectiveClose() + r*1.1/6
	out["R3"] = c.EffectiveClose() + r*1.1/4
	out["S1"] = c.EffectiveClose() - r*1.1/12
	out["S2"] = c.EffectiveClose() - r*1.1/6
	out["S3"] = c.EffectiveClose() - r*1.1/4
	return out
}

func WoodiePivotLevels(candles []ohlcv.Candle) map[string]float64 {
	c, ok := pivotSourceCandle(candles)
	out := map[string]float64{"S3": 0, "S2": 0, "S1": 0, "P": 0, "R1": 0, "R2": 0, "R3": 0}
	if !ok {
		return out
	}
	r := c.EffectiveHigh() - c.EffectiveLow()
	p := (c.EffectiveHigh() + c.EffectiveLow() + 2*c.EffectiveClose()) / 4
	out["P"] = p
	out["R1"] = 2*p - c.EffectiveLow()
	out["R2"] = p + r
	out["R3"] = c.EffectiveHigh() + 2*(p-c.EffectiveLow())
	out["S1"] = 2*p - c.EffectiveHigh()
	out["S2"] = p - r
	out["S3"] = c.EffectiveLow() - 2*(c.EffectiveHigh()-p)
	return out
}

func DeMarkPivotLevels(candles []ohlcv.Candle) map[string]float64 {
	c, ok := pivotSourceCandle(candles)
	out := map[string]float64{"S1": 0, "P": 0, "R1": 0}
	if !ok {
		return out
	}
	x := c.EffectiveHigh() + c.EffectiveLow() + 2*c.EffectiveClose()
	if c.EffectiveClose() < c.EffectiveOpen() {
		x = c.EffectiveHigh() + 2*c.EffectiveLow() + c.EffectiveClose()
	} else if c.EffectiveClose() > c.EffectiveOpen() {
		x = 2*c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()
	}
	out["P"] = x / 4
	out["R1"] = x/2 - c.EffectiveLow()
	out["S1"] = x/2 - c.EffectiveHigh()
	return out
}

func FibonacciExtensions(candles []ohlcv.Candle) map[string]float64 {
	out := map[string]float64{"1.272": 0, "1.618": 0, "fan_0.618": 0}
	if len(candles) == 0 {
		return out
	}
	high := mathutil.Max(highs(candles))
	low := mathutil.Min(lows(candles))
	diff := high - low
	out["1.272"] = high + diff*0.272
	out["1.618"] = high + diff*0.618
	out["fan_0.618"] = low + diff*0.618
	return out
}

func MurreyMathLine(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	high := mathutil.Max(highs(candles))
	low := mathutil.Min(lows(candles))
	step := mathutil.SafeDiv(high-low, 8)
	return low + step*4
}

func FairValueGapScore(candles []ohlcv.Candle) float64 {
	if len(candles) < 3 {
		return 0
	}
	i := len(candles) - 1
	if candles[i].EffectiveLow() > candles[i-2].EffectiveHigh() || candles[i].EffectiveHigh() < candles[i-2].EffectiveLow() {
		return 1
	}
	return 0
}

func LiquiditySweepScore(candles []ohlcv.Candle) float64 {
	if len(candles) < 21 {
		return 0
	}
	lastCandle := candles[len(candles)-1]
	prevHigh := mathutil.Max(highs(candles[len(candles)-21 : len(candles)-1]))
	prevLow := mathutil.Min(lows(candles[len(candles)-21 : len(candles)-1]))
	if lastCandle.EffectiveHigh() > prevHigh && lastCandle.EffectiveClose() < prevHigh {
		return 1
	}
	if lastCandle.EffectiveLow() < prevLow && lastCandle.EffectiveClose() > prevLow {
		return 1
	}
	return 0
}

func OrderBlockScore(candles []ohlcv.Candle) float64 {
	if len(candles) < 5 {
		return 0
	}
	lastMove := math.Abs(candles[len(candles)-1].EffectiveClose() - candles[len(candles)-5].EffectiveOpen())
	atr := ATR(candles, 14)
	return mathutil.Clamp(mathutil.SafeDiv(lastMove, 3*atr), 0, 1)
}

func PremiumDiscount(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	high := mathutil.Max(highs(candles))
	low := mathutil.Min(lows(candles))
	return mathutil.SafeDiv(last(closes(candles))-low, high-low)
}

func DominantCyclePeriod(values []float64) float64 {
	n := len(values)
	if n < 40 {
		return 20
	}
	// Autocorrelation periodicity: find lag 10-40 with highest positive correlation
	window := lastValues(values, minInt(n, 100))
	m := len(window)
	mean := mathutil.Mean(window)
	variance := 0.0
	for _, v := range window {
		d := v - mean
		variance += d * d
	}
	if variance == 0 {
		return 20
	}
	bestPeriod := 20
	bestCorr := -math.MaxFloat64
	for lag := 10; lag <= 40; lag++ {
		if lag >= m {
			break
		}
		corr := 0.0
		for i := 0; i < m-lag; i++ {
			corr += (window[i] - mean) * (window[i+lag] - mean)
		}
		corr /= variance
		if corr > bestCorr {
			bestCorr = corr
			bestPeriod = lag
		}
	}
	return float64(bestPeriod)
}

func DominantCyclePhase(values []float64) float64 {
	period := DominantCyclePeriod(values)
	return 2 * math.Pi * mathutil.SafeDiv(float64(len(values)%int(math.Max(period, 1))), period)
}

func RollingSharpe(values []float64, period int) float64 {
	rets := lastValues(returns(values), period)
	return mathutil.SafeDiv(mathutil.Mean(rets), mathutil.StdDev(rets)) * math.Sqrt(252)
}

func RollingSortino(values []float64, period int) float64 {
	rets := lastValues(returns(values), period)
	downside := []float64{}
	for _, r := range rets {
		if r < 0 {
			downside = append(downside, r)
		}
	}
	return mathutil.SafeDiv(mathutil.Mean(rets), mathutil.StdDev(downside)) * math.Sqrt(252)
}

func returnVolatility(values []float64, period int) float64 {
	return mathutil.StdDev(lastValues(returns(values), period)) * math.Sqrt(252)
}

func downsideVolatility(values []float64, period int) float64 {
	rets := lastValues(returns(values), period)
	downside := []float64{}
	for _, r := range rets {
		if r < 0 {
			downside = append(downside, r)
		}
	}
	return mathutil.StdDev(downside) * math.Sqrt(252)
}

func RollingMaxDrawdown(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) == 0 {
		return 0
	}
	peak := window[0]
	maxDD := 0.0
	for _, value := range window {
		peak = math.Max(peak, value)
		dd := mathutil.SafeDiv(value-peak, absDenominator(peak))
		maxDD = math.Min(maxDD, dd)
	}
	return maxDD
}

type swingValues struct {
	highs []float64
	lows  []float64
}

func recentSwingValues(candles []ohlcv.Candle, lookback int) swingValues {
	start := maxInt(0, len(candles)-lookback)
	out := swingValues{}
	for i := start + 3; i < len(candles)-3; i++ {
		high, low := candles[i].EffectiveHigh(), candles[i].EffectiveLow()
		isHigh, isLow := true, true
		for j := i - 3; j <= i+3; j++ {
			if j == i {
				continue
			}
			if candles[j].EffectiveHigh() >= high {
				isHigh = false
			}
			if candles[j].EffectiveLow() <= low {
				isLow = false
			}
		}
		if isHigh {
			out.highs = append(out.highs, high)
		}
		if isLow {
			out.lows = append(out.lows, low)
		}
	}
	return out
}

func streakSeries(values []float64) []float64 {
	out := make([]float64, len(values))
	streak := 0.0
	for i := 1; i < len(values); i++ {
		if values[i] > values[i-1] {
			if streak < 0 {
				streak = 0
			}
			streak++
		} else if values[i] < values[i-1] {
			if streak > 0 {
				streak = 0
			}
			streak--
		} else {
			streak = 0
		}
		out[i] = streak
	}
	return out
}

func percentileRank(values []float64, period int) float64 {
	if len(values) < 2 {
		return 50
	}
	window := lastValues(values, period)
	current := values[len(values)-1] - values[len(values)-2]
	count := 0
	for i := 1; i < len(window); i++ {
		if window[i]-window[i-1] <= current {
			count++
		}
	}
	return 100 * mathutil.SafeDiv(float64(count), float64(maxInt(1, len(window)-1)))
}

func returns(values []float64) []float64 {
	if len(values) < 2 {
		return nil
	}
	out := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		out[i-1] = mathutil.SafeDiv(values[i]-values[i-1], absDenominator(values[i-1]))
	}
	return out
}

func lastValues(values []float64, period int) []float64 {
	if period <= 0 || len(values) == 0 {
		return nil
	}
	start := len(values) - period
	if start < 0 {
		start = 0
	}
	return values[start:]
}

func last(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func sum(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

func sign(value float64) float64 {
	if value > 0 {
		return 1
	}
	if value < 0 {
		return -1
	}
	return 0
}

func boolScore(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func nextFibWindow(n int) int {
	a, b := 1, 1
	for b < n {
		a, b = b, a+b
	}
	return b
}

func closes(candles []ohlcv.Candle) []float64 {
	values := make([]float64, len(candles))
	for i, candle := range candles {
		values[i] = candle.EffectiveClose()
	}
	return values
}

func highs(candles []ohlcv.Candle) []float64 {
	values := make([]float64, len(candles))
	for i, candle := range candles {
		values[i] = candle.EffectiveHigh()
	}
	return values
}

func lows(candles []ohlcv.Candle) []float64 {
	values := make([]float64, len(candles))
	for i, candle := range candles {
		values[i] = candle.EffectiveLow()
	}
	return values
}

func volumes(candles []ohlcv.Candle) []float64 {
	values := make([]float64, len(candles))
	for i, candle := range candles {
		values[i] = candle.EffectiveVolume()
	}
	return values
}

func typicalPrices(candles []ohlcv.Candle) []float64 {
	values := make([]float64, len(candles))
	for i, candle := range candles {
		values[i] = (candle.EffectiveHigh() + candle.EffectiveLow() + candle.EffectiveClose()) / 3
	}
	return values
}

func highLowWindow(candles []ohlcv.Candle, period int) (float64, float64) {
	if len(candles) == 0 {
		return 0, 0
	}
	start := len(candles) - period
	if start < 0 {
		start = 0
	}
	window := candles[start:]
	highest := window[0].EffectiveHigh()
	lowest := window[0].EffectiveLow()
	for _, candle := range window[1:] {
		highest = math.Max(highest, candle.EffectiveHigh())
		lowest = math.Min(lowest, candle.EffectiveLow())
	}
	return highest, lowest
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
