package indicators

import (
	"math"
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

func TestCoreIndicatorsAgainstReferenceFormulas(t *testing.T) {
	candles := referenceCandles()
	closeValues := referenceCloses(candles)

	assertClose(t, "RSI14", RSI(closeValues, 14), referenceRSI(closeValues, 14), 1e-9)
	assertClose(t, "ATR14", ATR(candles, 14), referenceATR(candles, 14), 1e-9)
	assertClose(t, "MFI14", MFI(candles, 14), referenceMFI(candles, 14), 1e-9)

	gotMACD, gotSignal, gotHist := MACD(closeValues, 12, 26, 9)
	wantMACD, wantSignal, wantHist := referenceMACD(closeValues, 12, 26, 9)
	assertClose(t, "MACD", gotMACD, wantMACD, 1e-9)
	assertClose(t, "MACD signal", gotSignal, wantSignal, 1e-9)
	assertClose(t, "MACD histogram", gotHist, wantHist, 1e-9)

	gotUpper, gotMiddle, gotLower := BollingerBands(closeValues, 20, 2)
	wantUpper, wantMiddle, wantLower := referenceBollinger(closeValues, 20, 2)
	assertClose(t, "Bollinger upper", gotUpper, wantUpper, 1e-9)
	assertClose(t, "Bollinger middle", gotMiddle, wantMiddle, 1e-9)
	assertClose(t, "Bollinger lower", gotLower, wantLower, 1e-9)

	gotK, gotD := StochasticOscillator(candles, 14, 3)
	wantK, wantD := referenceStochastic(candles, 14, 3)
	assertClose(t, "Stochastic %K", gotK, wantK, 1e-9)
	assertClose(t, "Stochastic %D", gotD, wantD, 1e-9)

	gotRSIK, gotRSID := StochasticRSI(closeValues, 14, 14, 3, 3)
	wantRSIK, wantRSID := referenceStochasticRSI(closeValues, 14, 14, 3, 3)
	assertClose(t, "StochRSI %K", gotRSIK, wantRSIK, 1e-9)
	assertClose(t, "StochRSI %D", gotRSID, wantRSID, 1e-9)
}

func TestBollingerBandsUsePopulationStdDev(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	gotUpper, gotMiddle, gotLower := BollingerBands(values, 5, 2)

	wantMiddle := 3.0
	wantWidth := 2 * math.Sqrt(2)
	assertClose(t, "Bollinger population upper", gotUpper, wantMiddle+wantWidth, 1e-12)
	assertClose(t, "Bollinger population middle", gotMiddle, wantMiddle, 1e-12)
	assertClose(t, "Bollinger population lower", gotLower, wantMiddle-wantWidth, 1e-12)
}

func TestADXAcceptsMinimumCatalogBars(t *testing.T) {
	candles := directionalTrendCandles(28)
	if got := ADX(candles, 14); got <= 0 {
		t.Fatalf("ADX should be computable at the catalog minimum of 28 bars, got %.8f", got)
	}
}

func TestStochasticOscillatorUsesPartialSMADuringWarmup(t *testing.T) {
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Open: 4, High: 10, Low: 0, Close: 5, Volume: 1000},
		{Time: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Open: 5, High: 12, Low: 0, Close: 9, Volume: 1000},
		{Time: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Open: 9, High: 14, Low: 0, Close: 12, Volume: 1000},
	}

	gotK, gotD := StochasticOscillator(candles, 14, 3)
	wantK, wantD := referenceStochastic(candles, 14, 3)
	assertClose(t, "warmup stochastic %K", gotK, wantK, 1e-9)
	assertClose(t, "warmup stochastic %D", gotD, wantD, 1e-9)
}

func TestIchimokuKumoTwistDetectsCloudDirectionChange(t *testing.T) {
	candles := kumoTwistCandles()
	_, _, _, _, _, prevCloudTrend, prevKumoTwist, _, _ := Ichimoku(candles[:len(candles)-1], 9, 26, 52)
	if prevCloudTrend >= 0 {
		t.Fatalf("previous cloud trend should be bearish, got %.8f", prevCloudTrend)
	}
	if prevKumoTwist != 0 {
		t.Fatalf("previous bar should not already report a twist, got %.8f", prevKumoTwist)
	}

	_, _, _, _, _, cloudTrend, kumoTwist, _, _ := Ichimoku(candles, 9, 26, 52)
	if cloudTrend <= 0 {
		t.Fatalf("current cloud trend should be bullish, got %.8f", cloudTrend)
	}
	if kumoTwist != cloudTrend {
		t.Fatalf("kumo twist = %.8f, want new cloud direction %.8f", kumoTwist, cloudTrend)
	}
}

func TestMarketStructureBreakOfStructureUsesPriorWindow(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, 25)
	for i := 0; i < len(candles)-1; i++ {
		base := 90.0 + float64(i)*0.2
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   base,
			High:   100,
			Low:    85,
			Close:  base + 0.5,
			Volume: 1000 + float64(i),
		}
	}
	candles[len(candles)-1] = ohlcv.Candle{
		Time:   start.AddDate(0, 0, len(candles)-1),
		Open:   99,
		High:   106,
		Low:    98,
		Close:  105,
		Volume: 2000,
	}

	got := MarketStructure(candles)
	if got["Break of Structure"] != 1 {
		t.Fatalf("break of structure should compare close with prior-window high, got %.8f", got["Break of Structure"])
	}
	if got["Bullish Break of Structure"] != 1 || got["Bearish Break of Structure"] != 0 {
		t.Fatalf("bullish break direction not preserved: %+v", got)
	}
	if got["Supply Zone"] != 100 {
		t.Fatalf("supply zone should come from prior window, got %.8f", got["Supply Zone"])
	}

	candles[len(candles)-1] = ohlcv.Candle{
		Time:   start.AddDate(0, 0, len(candles)-1),
		Open:   86,
		High:   87,
		Low:    80,
		Close:  82,
		Volume: 2000,
	}
	got = MarketStructure(candles)
	if got["Break of Structure"] != 1 {
		t.Fatalf("bearish break of structure should be detected, got %.8f", got["Break of Structure"])
	}
	if got["Bullish Break of Structure"] != 0 || got["Bearish Break of Structure"] != 1 {
		t.Fatalf("bearish break direction not preserved: %+v", got)
	}
	if got["Demand Zone"] != 85 {
		t.Fatalf("demand zone should come from prior window, got %.8f", got["Demand Zone"])
	}
}

func TestDegenerateOscillatorWindowsAreNeutral(t *testing.T) {
	candles := flatReferenceCandles(40, 100)
	closeValues := referenceCloses(candles)

	if got := MFI(candles, 14); got != 50 {
		t.Fatalf("flat MFI should be neutral 50, got %.8f", got)
	}
	if got := WilliamsR(candles, 14); got != -50 {
		t.Fatalf("flat Williams %%R should be neutral -50, got %.8f", got)
	}
	if got := BollingerPercentB(closeValues, 20, 2); got != 0.5 {
		t.Fatalf("flat Bollinger %%B should be neutral 0.5, got %.8f", got)
	}
	k, d := StochasticOscillator(candles, 14, 3)
	if k != 50 || d != 50 {
		t.Fatalf("flat stochastic should be neutral 50/50, got %.8f/%.8f", k, d)
	}
	rsiK, rsiD := StochasticRSI(closeValues, 14, 14, 3, 3)
	if rsiK != 50 || rsiD != 50 {
		t.Fatalf("flat StochRSI should be neutral 50/50, got %.8f/%.8f", rsiK, rsiD)
	}
}

func TestAdditionalBoundedIndicatorsStayInPhysicalRanges(t *testing.T) {
	candles := referenceCandles()
	additional := AdditionalIndicators(candles)
	for _, tt := range []struct {
		name string
		min  float64
		max  float64
	}{
		{name: "Schaff Trend Cycle", min: 0, max: 100},
		{name: "Connors RSI", min: 0, max: 100},
		{name: "Ultimate Oscillator", min: 0, max: 100},
		{name: "Williams %R", min: -100, max: 0},
		{name: "Stochastic Momentum Index", min: -100, max: 100},
		{name: "Stochastic Momentum Signal", min: -100, max: 100},
	} {
		got := additional[tt.name]
		if got < tt.min || got > tt.max {
			t.Fatalf("%s = %.8f, want within [%.2f, %.2f]", tt.name, got, tt.min, tt.max)
		}
	}
	if got := additional["ZigZag"]; got <= 0 {
		t.Fatalf("ZigZag should expose the latest pivot price without direction sign, got %.8f", got)
	}
}

func TestIndicatorSignalsUseIndicatorSpecificRanges(t *testing.T) {
	input := ScannerInput{LastClose: 100, Snapshot: ohlcv.IndicatorSnapshot{VolumeSMA20: 1000}}
	tests := []struct {
		name     string
		template string
		value    float64
		want     string
	}{
		{name: "Relative Strength Index", template: "momentum", value: 50, want: "neutral"},
		{name: "Relative Strength Index", template: "momentum", value: 60, want: "bullish"},
		{name: "Williams %R", template: "momentum", value: -10, want: "overbought"},
		{name: "Williams %R", template: "momentum", value: -90, want: "oversold"},
		{name: "Bollinger %B", template: "momentum", value: 0.50, want: "neutral"},
		{name: "Bollinger %B", template: "momentum", value: 0.90, want: "overbought"},
		{name: "Commodity Channel Index", template: "momentum", value: -150, want: "oversold"},
		{name: "Momentum", template: "momentum", value: -2, want: "bearish"},
	}
	for _, tt := range tests {
		got, _, _ := signalForIndicator(input, indicatorSpec{Name: tt.name, Template: tt.template, Confidence: 0.8}, tt.value)
		if got != tt.want {
			t.Fatalf("%s signal for %.4f = %s, want %s", tt.name, tt.value, got, tt.want)
		}
	}
}

func TestIndicatorSignalsDoNotContradictSnapshotValues(t *testing.T) {
	input := ScannerInput{
		LastClose:  6.16,
		LastVolume: 11_523_218,
		Snapshot: ohlcv.IndicatorSnapshot{
			MACD:               -0.2786156965852937,
			MACDSignal:         -0.20139662934520536,
			MACDHistogram:      -0.07721906724008837,
			VolumeSMA20:        21_468_181.15,
			IchimokuCloudTrend: -1,
		},
	}
	tests := []struct {
		name     string
		template string
		value    float64
		want     string
	}{
		{name: "Moving Average Convergence Divergence", template: "trend", value: input.Snapshot.MACD, want: "bearish"},
		{name: "MACD Histogram", template: "trend", value: input.Snapshot.MACDHistogram, want: "bearish"},
		{name: "Volume Moving Average", template: "volume", value: input.Snapshot.VolumeSMA20, want: "bearish"},
		{name: "Volume", template: "volume", value: input.LastVolume, want: "bearish"},
		{name: "Anchored VWAP", template: "volume", value: 6.969835016151953, want: "bearish"},
		{name: "Ichimoku Kinko Hyo", template: "trend", value: input.Snapshot.IchimokuCloudTrend, want: "bearish"},
		{name: "Value Area High", template: "volume", value: 6.4344270833333335, want: "info"},
		{name: "Moving Average Ribbon", template: "moving_average", value: -6.053642360658832, want: "bearish"},
		{name: "Fibonacci Time Zones", template: "support_resistance", value: 377, want: "info"},
	}
	for _, tt := range tests {
		got, _, evidence := signalForIndicator(input, indicatorSpec{Name: tt.name, Template: tt.template, Confidence: 0.8}, tt.value)
		if got != tt.want {
			t.Fatalf("%s signal for %.4f = %s (%s), want %s", tt.name, tt.value, got, evidence, tt.want)
		}
	}
}

func assertClose(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("%s returned invalid value %.8f", name, got)
	}
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %.12f, want %.12f", name, got, want)
	}
}

func referenceCandles() []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, 90)
	price := 100.0
	for i := range candles {
		change := math.Sin(float64(i)*0.41)*1.2 + float64(i%5-2)*0.25 + 0.35
		open := price
		closePrice := price + change
		high := math.Max(open, closePrice) + 1.1 + float64(i%3)*0.15
		low := math.Min(open, closePrice) - 0.9 - float64(i%4)*0.12
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 1000 + float64((i*37)%700) + float64(i)*5,
		}
		price = closePrice
	}
	return candles
}

func directionalTrendCandles(count int) []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	for i := range candles {
		open := 100 + float64(i)*2
		closePrice := open + 1
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   closePrice + 0.5,
			Low:    open - 0.5,
			Close:  closePrice,
			Volume: 1000 + float64(i),
		}
	}
	return candles
}

func kumoTwistCandles() []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, 60)
	for i := range candles {
		low := 90.0
		high := 100.0
		closePrice := 95.0
		if i >= 8 && i <= 32 {
			low = 20
			high = 80
			closePrice = 50
		}
		if i == 7 {
			low = 0
			high = 200
			closePrice = 100
		}
		if i == len(candles)-1 {
			low = 290
			high = 300
			closePrice = 295
		}
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   closePrice,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 1000,
		}
	}
	return candles
}

func flatReferenceCandles(count int, price float64) []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	for i := range candles {
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   price,
			High:   price,
			Low:    price,
			Close:  price,
			Volume: 1000,
		}
	}
	return candles
}

func referenceCloses(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, candle := range candles {
		out[i] = candle.EffectiveClose()
	}
	return out
}

// referenceRSI: naive Wilder RSI — seed with SMA of first `period` gains/losses, then smooth.
func referenceRSI(values []float64, period int) float64 {
	if len(values) < period+1 {
		return 50
	}
	gains := make([]float64, len(values)-1)
	losses := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		d := values[i] - values[i-1]
		if d > 0 {
			gains[i-1] = d
		} else {
			losses[i-1] = -d
		}
	}
	avgGain := mathutil.Mean(gains[:period])
	avgLoss := mathutil.Mean(losses[:period])
	for i := period; i < len(gains); i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
	}
	if avgGain == 0 && avgLoss == 0 {
		return 50
	}
	if avgLoss == 0 {
		return 100
	}
	if avgGain == 0 {
		return 0
	}
	return 100 - 100/(1+avgGain/avgLoss)
}

func referenceRSISeries(values []float64, period int) []float64 {
	return RSISeries(values, period)
}

func referenceRSIValue(gain, loss float64) float64 {
	if gain == 0 && loss == 0 {
		return 50
	}
	if loss == 0 {
		return 100
	}
	if gain == 0 {
		return 0
	}
	rs := gain / loss
	return 100 - 100/(1+rs)
}

// referenceATR: naive Wilder ATR — seed with SMA(tr[0..period-1]), then smooth.
func referenceATR(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	tr := make([]float64, len(candles))
	tr[0] = candles[0].EffectiveHigh() - candles[0].EffectiveLow()
	for i := 1; i < len(candles); i++ {
		h := candles[i].EffectiveHigh()
		l := candles[i].EffectiveLow()
		prevC := candles[i-1].EffectiveClose()
		tr[i] = math.Max(h-l, math.Max(math.Abs(h-prevC), math.Abs(l-prevC)))
	}
	atr := mathutil.Mean(tr[0:period])
	for i := period; i < len(tr); i++ {
		atr = (atr*float64(period-1) + tr[i]) / float64(period)
	}
	return atr
}

// referenceMFI: naive Money Flow Index over the last `period` bars.
func referenceMFI(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 50
	}
	window := candles[len(candles)-period-1:]
	var posFlow, negFlow float64
	prevTP := (window[0].EffectiveHigh() + window[0].EffectiveLow() + window[0].EffectiveClose()) / 3
	for i := 1; i <= period; i++ {
		tp := (window[i].EffectiveHigh() + window[i].EffectiveLow() + window[i].EffectiveClose()) / 3
		mf := tp * window[i].Volume
		if tp > prevTP {
			posFlow += mf
		} else if tp < prevTP {
			negFlow += mf
		}
		prevTP = tp
	}
	if posFlow+negFlow == 0 {
		return 50
	}
	return 100 * posFlow / (posFlow + negFlow)
}

// referenceMACD: independent naive MACD using SMA-seeded EMA (matching EMASeries) and clean signal start.
func referenceMACD(values []float64, fast, slow, signal int) (float64, float64, float64) {
	if len(values) == 0 || slow > len(values) {
		return 0, 0, 0
	}
	smaEMA := func(v []float64, p int) []float64 {
		out := make([]float64, len(v))
		if len(v) == 0 || p <= 0 {
			return out
		}
		k := 2.0 / float64(p+1)
		if len(v) <= p {
			var s float64
			for i, val := range v {
				s += val
				out[i] = s / float64(i+1)
			}
			return out
		}
		var s float64
		for i := 0; i < p; i++ {
			s += v[i]
			out[i] = s / float64(i+1)
		}
		for i := p; i < len(v); i++ {
			out[i] = v[i]*k + out[i-1]*(1-k)
		}
		return out
	}
	fastEMA := smaEMA(values, fast)
	slowEMA := smaEMA(values, slow)
	line := make([]float64, len(values))
	for i := slow - 1; i < len(values); i++ {
		line[i] = fastEMA[i] - slowEMA[i]
	}
	signalLine := smaEMA(line[slow-1:], signal)
	last := len(values) - 1
	macdLine := line[last]
	sig := signalLine[len(signalLine)-1]
	return macdLine, sig, macdLine - sig
}

func referenceEMA(values []float64, period int) []float64 {
	return EMASeries(values, period)
}

// referenceBollinger: SMA + population std dev (N denominator) over last `period` bars.
func referenceBollinger(values []float64, period int, multiplier float64) (float64, float64, float64) {
	if len(values) < period {
		return 0, 0, 0
	}
	window := values[len(values)-period:]
	var sum float64
	for _, v := range window {
		sum += v
	}
	mean := sum / float64(period)
	var variance float64
	for _, v := range window {
		d := v - mean
		variance += d * d
	}
	stddev := math.Sqrt(variance / float64(period))
	return mean + multiplier*stddev, mean, mean - multiplier*stddev
}

// referenceStochastic: naive slow stochastic — growing window raw %K, 3-period SMA → slow %K, dPeriod SMA → %D.
func referenceStochastic(candles []ohlcv.Candle, kPeriod, dPeriod int) (float64, float64) {
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
		hi, lo := window[0].EffectiveHigh(), window[0].EffectiveLow()
		for _, bar := range window[1:] {
			if bar.EffectiveHigh() > hi {
				hi = bar.EffectiveHigh()
			}
			if bar.EffectiveLow() < lo {
				lo = bar.EffectiveLow()
			}
		}
		c := candles[i].EffectiveClose()
		if math.Abs(hi-lo) < 1e-12 {
			rawK[i] = 50
		} else {
			rawK[i] = 100 * (c - lo) / (hi - lo)
		}
	}
	smaSeries := func(vals []float64, period int) []float64 {
		out := make([]float64, len(vals))
		for i := range vals {
			n := i + 1
			if n > period {
				n = period
			}
			var sum float64
			for j := i - n + 1; j <= i; j++ {
				sum += vals[j]
			}
			out[i] = sum / float64(n)
		}
		return out
	}
	slowK := smaSeries(rawK, 3)
	slowD := smaSeries(slowK, dPeriod)
	return slowK[len(slowK)-1], slowD[len(slowD)-1]
}

// referenceStochasticRSI: naive stochastic applied to RSI series with growing window + SMA smoothing.
func referenceStochasticRSI(values []float64, rsiPeriod, stochPeriod, kPeriod, dPeriod int) (float64, float64) {
	rsiSeries := RSISeries(values, rsiPeriod)
	if len(rsiSeries) == 0 {
		return 50, 50
	}
	rawK := make([]float64, len(rsiSeries))
	for i := range rsiSeries {
		start := i - stochPeriod + 1
		if start < 0 {
			start = 0
		}
		window := rsiSeries[start : i+1]
		lo, hi := window[0], window[0]
		for _, v := range window[1:] {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		if math.Abs(hi-lo) < 1e-12 {
			rawK[i] = 50
		} else {
			rawK[i] = 100 * (rsiSeries[i] - lo) / (hi - lo)
		}
	}
	smaSeries := func(vals []float64, period int) []float64 {
		out := make([]float64, len(vals))
		for i := range vals {
			n := i + 1
			if n > period {
				n = period
			}
			var sum float64
			for j := i - n + 1; j <= i; j++ {
				sum += vals[j]
			}
			out[i] = sum / float64(n)
		}
		return out
	}
	smoothK := smaSeries(rawK, kPeriod)
	smoothD := smaSeries(smoothK, dPeriod)
	return smoothK[len(smoothK)-1], smoothD[len(smoothD)-1]
}

func referenceSMA(values []float64, period int) float64 {
	return SMA(values, period)
}

func referenceMean(values []float64) float64 {
	return mathutil.Mean(values)
}

func referenceStdDev(values []float64) float64 {
	return mathutil.StdDev(values)
}

func referenceTypical(candle ohlcv.Candle) float64 {
	return (candle.EffectiveHigh() + candle.EffectiveLow() + candle.EffectiveClose()) / 3
}
