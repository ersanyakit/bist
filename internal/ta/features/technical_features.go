package features

import "math"

func addTechnicalFeatures(fv *FeatureVector, bars []MarketBar) {
	if len(bars) == 0 {
		return
	}
	last := bars[len(bars)-1]
	setFinite(fv.Values, "last_close", last.Close)
	setFinite(fv.Values, "last_open", last.Open)
	setFinite(fv.Values, "last_high", last.High)
	setFinite(fv.Values, "last_low", last.Low)
	setFinite(fv.Values, "last_volume", last.Volume)

	for _, n := range []int{1, 2, 3, 5, 10, 20} {
		setFinite(fv.Values, "return_"+itoa(n)+"d", pctReturn(bars, n))
	}
	setFinite(fv.Values, "log_return_1d", logReturn(bars, 1))
	setFinite(fv.Values, "log_return_5d", logReturn(bars, 5))
	setFinite(fv.Values, "log_return_20d", logReturn(bars, 20))
	setFinite(fv.Values, "volatility_5d", rollingStd(logReturns(bars), 5))
	setFinite(fv.Values, "volatility_20d", rollingStd(logReturns(bars), 20))
	atr := atr(bars, 14)
	setFinite(fv.Values, "atr14", atr)
	setFinite(fv.Values, "realized_volatility_20d", rollingStd(logReturns(bars), 20)*math.Sqrt(252))
	setFinite(fv.Values, "rsi14", rsi(bars, 14))
	macd, signal, hist := macd(bars)
	setFinite(fv.Values, "macd", macd)
	setFinite(fv.Values, "macd_signal", signal)
	setFinite(fv.Values, "macd_histogram", hist)
	mid := smaClose(bars, 20)
	std := rollingStd(closes(bars), 20)
	if std > 0 {
		setFinite(fv.Values, "bollinger_zscore", (last.Close-mid)/std)
	}
	ema20 := emaClose(bars, 20)
	if last.Close > 0 {
		setFinite(fv.Values, "ema20_distance", ema20/last.Close-1)
		setFinite(fv.Values, "sma20_distance", mid/last.Close-1)
	}
	setFinite(fv.Values, "supertrend_direction", supertrendDirection(bars, atr))
	setFinite(fv.Values, "cmf20", cmf(bars, 20))
	stochK, stochD := stochastic(bars, 14, 3)
	setFinite(fv.Values, "stochastic_k", stochK)
	setFinite(fv.Values, "stochastic_d", stochD)
	if len(bars) >= 2 && bars[len(bars)-2].Volume > 0 {
		setFinite(fv.Values, "volume_change_1d", last.Volume/bars[len(bars)-2].Volume-1)
	}
	setFinite(fv.Values, "volume_zscore20", zscore(volumes(bars), 20))
	rng := last.High - last.Low
	if rng > 0 {
		setFinite(fv.Values, "candle_body_ratio", math.Abs(last.Close-last.Open)/rng)
		setFinite(fv.Values, "upper_wick_ratio", (last.High-math.Max(last.Open, last.Close))/rng)
		setFinite(fv.Values, "lower_wick_ratio", (math.Min(last.Open, last.Close)-last.Low)/rng)
	}
	if len(bars) >= 2 && bars[len(bars)-2].Close > 0 {
		prevClose := bars[len(bars)-2].Close
		setFinite(fv.Values, "gap_1d", last.Open/prevClose-1)
		setFinite(fv.Values, "previous_close_distance", last.Close/prevClose-1)
	}
	support, resistance := supportResistance(bars, 20)
	if last.Close > 0 {
		setFinite(fv.Values, "support_distance_pct", last.Close/support-1)
		setFinite(fv.Values, "resistance_distance_pct", resistance/last.Close-1)
	}
	setFinite(fv.Values, "trend_slope_20d", trendSlope(bars, 20))
	if last.VWAP != nil && *last.VWAP > 0 {
		setFinite(fv.Values, "vwap_distance", last.Close/(*last.VWAP)-1)
	}
	setFinite(fv.Values, "intraday_range", safeDiv(last.High-last.Low, last.Close))
}

func pctReturn(bars []MarketBar, n int) float64 {
	if len(bars) <= n {
		return 0
	}
	prev := bars[len(bars)-1-n].Close
	if prev <= 0 {
		return 0
	}
	return bars[len(bars)-1].Close/prev - 1
}

func logReturn(bars []MarketBar, n int) float64 {
	r := pctReturn(bars, n)
	if r <= -1 {
		return 0
	}
	return math.Log1p(r)
}

func logReturns(bars []MarketBar) []float64 {
	out := make([]float64, len(bars))
	for i := 1; i < len(bars); i++ {
		if bars[i-1].Close > 0 && bars[i].Close > 0 {
			out[i] = math.Log(bars[i].Close / bars[i-1].Close)
		}
	}
	return out
}

func closes(bars []MarketBar) []float64 {
	out := make([]float64, len(bars))
	for i, bar := range bars {
		out[i] = bar.Close
	}
	return out
}

func volumes(bars []MarketBar) []float64 {
	out := make([]float64, len(bars))
	for i, bar := range bars {
		out[i] = bar.Volume
	}
	return out
}

func rollingStd(values []float64, period int) float64 {
	if period <= 1 || len(values) < period {
		return 0
	}
	window := values[len(values)-period:]
	mean := 0.0
	for _, value := range window {
		mean += value
	}
	mean /= float64(len(window))
	variance := 0.0
	for _, value := range window {
		d := value - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(window)-1))
}

func smaClose(bars []MarketBar, period int) float64 {
	return sma(closes(bars), period)
}

func sma(values []float64, period int) float64 {
	if period <= 0 || len(values) < period {
		return 0
	}
	sum := 0.0
	for _, value := range values[len(values)-period:] {
		sum += value
	}
	return sum / float64(period)
}

func emaClose(bars []MarketBar, period int) float64 {
	return ema(closes(bars), period)
}

func ema(values []float64, period int) float64 {
	if period <= 0 || len(values) == 0 {
		return 0
	}
	alpha := 2 / float64(period+1)
	out := values[0]
	for _, value := range values[1:] {
		out = alpha*value + (1-alpha)*out
	}
	return out
}

func atr(bars []MarketBar, period int) float64 {
	if len(bars) < 2 {
		return 0
	}
	tr := make([]float64, 0, len(bars)-1)
	for i := 1; i < len(bars); i++ {
		h := bars[i].High
		l := bars[i].Low
		pc := bars[i-1].Close
		tr = append(tr, math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc))))
	}
	return sma(tr, minInt(period, len(tr)))
}

func rsi(bars []MarketBar, period int) float64 {
	if len(bars) <= period {
		return 50
	}
	gain := 0.0
	loss := 0.0
	start := len(bars) - period
	for i := start; i < len(bars); i++ {
		d := bars[i].Close - bars[i-1].Close
		if d >= 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	if loss == 0 && gain == 0 {
		return 50
	}
	if loss == 0 {
		return 100
	}
	rs := gain / loss
	return 100 - 100/(1+rs)
}

func macd(bars []MarketBar) (float64, float64, float64) {
	values := closes(bars)
	if len(values) == 0 {
		return 0, 0, 0
	}
	lineSeries := make([]float64, len(values))
	for i := range values {
		prefix := values[:i+1]
		lineSeries[i] = ema(prefix, 12) - ema(prefix, 26)
	}
	line := lineSeries[len(lineSeries)-1]
	signal := ema(lineSeries, 9)
	return line, signal, line - signal
}

func supertrendDirection(bars []MarketBar, atrValue float64) float64 {
	if len(bars) == 0 || atrValue <= 0 {
		return 0
	}
	last := bars[len(bars)-1]
	mid := (last.High + last.Low) / 2
	if last.Close > mid+atrValue {
		return 1
	}
	if last.Close < mid-atrValue {
		return -1
	}
	return 0
}

func cmf(bars []MarketBar, period int) float64 {
	if len(bars) < period {
		return 0
	}
	sumMFV := 0.0
	sumVol := 0.0
	for _, bar := range bars[len(bars)-period:] {
		rng := bar.High - bar.Low
		if rng <= 0 {
			continue
		}
		mfm := ((bar.Close - bar.Low) - (bar.High - bar.Close)) / rng
		sumMFV += mfm * bar.Volume
		sumVol += bar.Volume
	}
	return safeDiv(sumMFV, sumVol)
}

func stochastic(bars []MarketBar, period int, smooth int) (float64, float64) {
	if len(bars) < period {
		return 50, 50
	}
	kSeries := []float64{}
	for i := period - 1; i < len(bars); i++ {
		window := bars[i-period+1 : i+1]
		lo := window[0].Low
		hi := window[0].High
		for _, bar := range window[1:] {
			lo = math.Min(lo, bar.Low)
			hi = math.Max(hi, bar.High)
		}
		kSeries = append(kSeries, 100*safeDiv(bars[i].Close-lo, hi-lo))
	}
	k := kSeries[len(kSeries)-1]
	d := sma(kSeries, minInt(smooth, len(kSeries)))
	return k, d
}

func zscore(values []float64, period int) float64 {
	if len(values) < period || period <= 1 {
		return 0
	}
	window := values[len(values)-period:]
	mean := sma(values, period)
	std := rollingStd(window, len(window))
	if std <= 0 {
		return 0
	}
	return (window[len(window)-1] - mean) / std
}

func supportResistance(bars []MarketBar, period int) (float64, float64) {
	if len(bars) == 0 {
		return 0, 0
	}
	start := len(bars) - period
	if start < 0 {
		start = 0
	}
	lo := bars[start].Low
	hi := bars[start].High
	for _, bar := range bars[start:] {
		lo = math.Min(lo, bar.Low)
		hi = math.Max(hi, bar.High)
	}
	return lo, hi
}

func trendSlope(bars []MarketBar, period int) float64 {
	if len(bars) < 2 {
		return 0
	}
	start := len(bars) - period
	if start < 0 {
		start = 0
	}
	window := bars[start:]
	n := float64(len(window))
	sumX, sumY, sumXY, sumXX := 0.0, 0.0, 0.0, 0.0
	for i, bar := range window {
		x := float64(i)
		y := math.Log(math.Max(bar.Close, 1e-9))
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	den := n*sumXX - sumX*sumX
	if den == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / den
}

func safeDiv(a, b float64) float64 {
	if b == 0 || math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return 0
	}
	return a / b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa(n int) string {
	switch n {
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 5:
		return "5"
	case 10:
		return "10"
	case 20:
		return "20"
	default:
		return "x"
	}
}
