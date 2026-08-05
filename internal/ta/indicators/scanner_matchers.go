package indicators

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/scanmatch"
	"hissebot/pkg/mathutil"
)

type indicatorValue struct {
	value    float64
	source   string
	computed bool
}

const proxySource = "ohlcv_proxy"
const insufficientDataSource = "insufficient_data"
const algorithmRequiredSource = "algorithm_required"

func detectIndicatorSpec(input ScannerInput, spec indicatorSpec) (ohlcv.IndicatorResult, error) {
	if len(input.Candles) == 0 {
		return ohlcv.IndicatorResult{}, fmt.Errorf("indicator detector requires candles: %w", ErrInsufficientData)
	}
	value := valueForIndicator(input, spec)
	if !value.computed {
		if isAlgorithmRequired(spec) {
			return ohlcv.IndicatorResult{
				Name:       spec.Name,
				Category:   spec.Category,
				Group:      spec.Group,
				Signal:     algorithmRequiredSource,
				Confidence: 0,
				Computed:   false,
				Source:     algorithmRequiredSource,
				Evidence:   []string{algorithmRequiredEvidence(spec)},
			}, nil
		}
		return ohlcv.IndicatorResult{
			Name:       spec.Name,
			Category:   spec.Category,
			Group:      spec.Group,
			Signal:     "requires_external_data",
			Confidence: 0,
			Computed:   false,
			Source:     "unavailable",
			Evidence:   []string{externalDataEvidence(spec)},
		}, nil
	}
	if minBars := minimumBarsForIndicator(input, spec, value); minBars > len(input.Candles) {
		return ohlcv.IndicatorResult{
			Name:       spec.Name,
			Category:   spec.Category,
			Group:      spec.Group,
			Signal:     "insufficient_data",
			Value:      0,
			Confidence: 0,
			Computed:   false,
			Source:     insufficientDataSource,
			Evidence:   []string{fmt.Sprintf("requires at least %d completed candles for an exact %s calculation; got %d", minBars, spec.Name, len(input.Candles))},
		}, nil
	}
	if value.source == proxySource {
		return ohlcv.IndicatorResult{
			Name:       spec.Name,
			Category:   spec.Category,
			Group:      spec.Group,
			Signal:     "proxy_only",
			Value:      value.value,
			Confidence: 0,
			Computed:   false,
			Source:     value.source,
			Evidence:   []string{"exact indicator formula is not implemented; OHLCV proxy value is kept for audit only"},
		}, nil
	}
	signal, confidence, evidence := signalForIndicator(input, spec, value.value)
	return ohlcv.IndicatorResult{
		Name:       spec.Name,
		Category:   spec.Category,
		Group:      spec.Group,
		Signal:     signal,
		Value:      value.value,
		Confidence: mathutil.Clamp(confidence, 0, 1),
		Computed:   true,
		Source:     value.source,
		Evidence:   []string{evidence},
	}, nil
}

func minimumBarsForIndicator(input ScannerInput, spec indicatorSpec, value indicatorValue) int {
	if value.source == "" || value.source == proxySource || isExternalOnly(spec) {
		return 0
	}
	name := normalizeIndicatorText(spec.Name)
	source := strings.ToLower(strings.TrimSpace(value.source))
	switch source {
	case "snapshot.sma20", "snapshot.ema20", "snapshot.bollinger_middle", "snapshot.bollinger_width", "snapshot.bollinger_percent_b", "snapshot.volume_sma20", "snapshot.chaikin_money_flow20", "snapshot.cci20", "snapshot.donchian_upper", "snapshot.keltner_middle":
		return 20
	case "snapshot.sma50", "snapshot.ema50":
		return 50
	case "snapshot.sma100", "snapshot.ema100":
		return 100
	case "snapshot.sma200", "snapshot.ema200":
		return 200
	case "snapshot.rsi14", "snapshot.atr14", "snapshot.mfi14", "snapshot.williams_r14":
		return 15
	case "snapshot.macd", "snapshot.macd_signal", "snapshot.macd_histogram":
		return 35
	case "snapshot.adx14":
		return 28
	case "snapshot.stochastic_k", "snapshot.stochastic_d":
		return 16
	case "snapshot.stoch_rsi_k", "snapshot.stoch_rsi_d":
		return 31
	case "snapshot.roc12":
		return 13
	case "snapshot.supertrend":
		return 11
	case "snapshot.ichimoku_cloud_trend", "snapshot.ichimoku_tenkan", "snapshot.ichimoku_kijun", "snapshot.ichimoku_senkou_a", "snapshot.ichimoku_senkou_b", "snapshot.ichimoku_chikou", "snapshot.ichimoku_kumo_twist", "snapshot.ichimoku_tk_cross":
		return 52
	case "snapshot.pivot_point":
		return 2
	case "snapshot.fibonacci_levels":
		return 20
	case "snapshot.additional_indicators":
		return additionalIndicatorMinimumBars(name)
	case "snapshot.support_tools", "snapshot.market_structure", "snapshot.relative_strength":
		return 20
	case "snapshot.vwap", "snapshot.obv", "snapshot.accumulation_distribution", "candle.volume":
		return 1
	}
	if strings.Contains(name, "sma200") || strings.Contains(name, "ema200") || strings.Contains(name, "200") {
		return 200
	}
	if strings.Contains(name, "sma100") || strings.Contains(name, "ema100") || strings.Contains(name, "100") {
		return 100
	}
	if strings.Contains(name, "sma50") || strings.Contains(name, "ema50") {
		return 50
	}
	if strings.Contains(name, "macd") {
		return 35
	}
	if strings.Contains(name, "ichimoku") || strings.Contains(name, "senkou") || strings.Contains(name, "kumo") {
		return 52
	}
	if strings.Contains(name, "rsi") || strings.Contains(name, "mfi") || strings.Contains(name, "williams") {
		return 15
	}
	if strings.Contains(name, "stoch") {
		return 16
	}
	if strings.Contains(name, "bollinger") || strings.Contains(name, "donchian") || strings.Contains(name, "keltner") || strings.Contains(name, "cci") {
		return 20
	}
	_ = input
	return 0
}

func additionalIndicatorMinimumBars(name string) int {
	switch {
	case strings.Contains(name, "hull moving average"):
		return 23
	case strings.Contains(name, "kaufman adaptive"):
		return 30
	case strings.Contains(name, "tema"):
		return 60
	case strings.Contains(name, "ultimate oscillator"):
		return 29
	case strings.Contains(name, "aroon"):
		return 25
	case strings.Contains(name, "connors rsi"):
		return 100
	case strings.Contains(name, "stochastic"), strings.Contains(name, "schaff"), strings.Contains(name, "money flow"):
		return 20
	case strings.Contains(name, "moving average") || strings.Contains(name, "average"):
		return 20
	default:
		return 20
	}
}

func valueForIndicator(input ScannerInput, spec indicatorSpec) indicatorValue {
	name := normalizeIndicatorText(spec.Name)
	if isExternalOnly(spec) {
		return indicatorValue{}
	}
	if isAlgorithmRequired(spec) {
		return indicatorValue{}
	}
	known := knownIndicatorValues(input)
	if value, ok := known[name]; ok {
		return value
	}
	// No exact snapshot/derived value resolves this name, but its template has a
	// reasonable OHLCV-derived approximation (see computeProxyValue) — surface that
	// instead of silently mislabeling a computable indicator as needing external data.
	// detectIndicatorSpec reports proxy-sourced values as Signal:"proxy_only" with
	// Computed:false, so this never turns into a scored trading signal.
	if len(input.Candles) > 0 {
		return computeProxyValue(input, spec)
	}
	return indicatorValue{}
}

func knownIndicatorValues(input ScannerInput) map[string]indicatorValue {
	s := input.Snapshot
	cls := closes(input.Candles)
	vol := volumes(input.Candles)
	lastClose := input.LastClose
	if lastClose == 0 {
		lastClose = last(cls)
	}
	m := map[string]indicatorValue{}
	add := func(name string, value float64, source string) {
		m[normalizeIndicatorText(name)] = indicatorValue{value: value, source: source, computed: true}
	}
	addAliases := func(value float64, source string, names ...string) {
		for _, name := range names {
			add(name, value, source)
		}
	}
	plusDI, minusDI, dx := directionalMovementValues(input.Candles, 14)
	plusDM, minusDM := directionalMovementRaw(input.Candles, 14)
	adxr := averageDirectionalMovementRating(input.Candles, 14)
	apo := EMA(cls, 12) - EMA(cls, 26)
	avgPrice := averagePrice(input.Candles)
	bop := balanceOfPower(input.Candles)
	typPrice := typicalPrice(input.Candles)
	medPrice := medianPrice(input.Candles)
	weightedClose := weightedClosePrice(input.Candles)
	ohlc := ohlc4(input.Candles)
	latestRange := latestTrueRange(input.Candles)
	add("Simple Moving Average", s.SMA20, "snapshot.sma20")
	add("SMA", s.SMA20, "snapshot.sma20")
	add("MA", s.SMA20, "snapshot.sma20")
	add("SMA20", s.SMA20, "snapshot.sma20")
	add("SMA50", s.SMA50, "snapshot.sma50")
	add("SMA100", s.SMA100, "snapshot.sma100")
	add("SMA200", s.SMA200, "snapshot.sma200")
	add("Exponential Moving Average", s.EMA20, "snapshot.ema20")
	add("EMA", s.EMA20, "snapshot.ema20")
	add("EMA20", s.EMA20, "snapshot.ema20")
	add("EMA50", s.EMA50, "snapshot.ema50")
	add("EMA100", s.EMA100, "snapshot.ema100")
	add("EMA200", s.EMA200, "snapshot.ema200")
	add("Relative Strength Index", s.RSI14, "snapshot.rsi14")
	add("RSI", s.RSI14, "snapshot.rsi14")
	add("Average True Range", s.ATR14, "snapshot.atr14")
	add("ATR", s.ATR14, "snapshot.atr14")
	add("MACD", s.MACD, "snapshot.macd")
	add("Moving Average Convergence Divergence", s.MACD, "snapshot.macd")
	add("MACD Signal Line", s.MACDSignal, "snapshot.macd_signal")
	add("MACD Histogram", s.MACDHistogram, "snapshot.macd_histogram")
	addAliases(apo, "snapshot.apo", "APO", "Absolute Price Oscillator")
	add("Bollinger Bandwidth", 100*mathutil.SafeDiv(s.BollingerUpper-s.BollingerLower, absDenominator(s.BollingerMiddle)), "snapshot.bollinger_width")
	bollingerPercentB := BollingerPercentB(closes(input.Candles), 20, 2)
	add("Bollinger %B", bollingerPercentB, "snapshot.bollinger_percent_b")
	add("Bollinger Bands %B", bollingerPercentB, "snapshot.bollinger_percent_b")
	add("Bollinger Bandwidth Oscillator", 100*mathutil.SafeDiv(s.BollingerUpper-s.BollingerLower, absDenominator(s.BollingerMiddle)), "snapshot.bollinger_width")
	add("ADX", s.ADX14, "snapshot.adx14")
	add("Average Directional Index", s.ADX14, "snapshot.adx14")
	addAliases(plusDI, "snapshot.directional_movement.plus_di", "+DI", "PLUS_DI", "Plus DI", "Plus Directional Indicator")
	addAliases(minusDI, "snapshot.directional_movement.minus_di", "-DI", "MINUS_DI", "Minus DI", "Minus Directional Indicator")
	addAliases(plusDI, "snapshot.directional_movement.plus_di", "Positive Directional Indicator")
	addAliases(minusDI, "snapshot.directional_movement.minus_di", "Negative Directional Indicator")
	addAliases(plusDM, "snapshot.directional_movement.plus_dm", "PLUS_DM")
	addAliases(minusDM, "snapshot.directional_movement.minus_dm", "MINUS_DM")
	addAliases(dx, "snapshot.directional_movement.dx", "DX", "Directional Movement Index")
	addAliases(adxr, "snapshot.directional_movement.adxr", "ADXR")
	addAliases(plusDI-minusDI, "snapshot.directional_movement.oscillator", "DMI", "DMI Oscillator")
	add("VWAP", s.VWAP, "snapshot.vwap")
	add("Volume Weighted Average Price", s.VWAP, "snapshot.vwap")
	add("OBV", s.OBV, "snapshot.obv")
	add("On Balance Volume", s.OBV, "snapshot.obv")
	add("Money Flow Index", s.MFI14, "snapshot.mfi14")
	add("MFI", s.MFI14, "snapshot.mfi14")
	add("Stochastic Oscillator", s.StochasticK, "snapshot.stochastic_k")
	add("Stochastic %K", s.StochasticK, "snapshot.stochastic_k")
	add("Stochastic %D", s.StochasticD, "snapshot.stochastic_d")
	add("Stochastic RSI", s.StochRSIK, "snapshot.stoch_rsi_k")
	add("StochRSI", s.StochRSIK, "snapshot.stoch_rsi_k")
	add("Commodity Channel Index", s.CCI20, "snapshot.cci20")
	add("CCI", s.CCI20, "snapshot.cci20")
	add("Williams Percent Range", s.WilliamsR14, "snapshot.williams_r14")
	add("Williams %R", s.WilliamsR14, "snapshot.williams_r14")
	add("Rate of Change", s.ROC12, "snapshot.roc12")
	add("ROC", s.ROC12, "snapshot.roc12")
	add("Supertrend", s.Supertrend, "snapshot.supertrend")
	addAliases(averageRange(input.Candles, 14), "snapshot.average_daily_range", "ADR", "Average Daily Range")
	addAliases(100*mathutil.SafeDiv(s.ATR14, absDenominator(lastClose)), "snapshot.atr_percent", "ATR Percent", "Average True Range Percent", "NATR", "Normalized ATR", "Normalized Average True Range")
	addAliases(bop, "snapshot.balance_of_power", "BOP", "Balance of Power")
	addAliases(avgPrice, "snapshot.price_transform.average_price", "AVGPRICE", "Average Price")
	addAliases(medPrice, "snapshot.price_transform.median_price", "MEDPRICE", "Median Price")
	addAliases(typPrice, "snapshot.price_transform.typical_price", "TYPPRICE", "Typical Price")
	addAliases(weightedClose, "snapshot.price_transform.weighted_close", "WCLPRICE", "Weighted Close Price")
	addAliases(ohlc, "snapshot.price_transform.ohlc4", "OHLC4")
	addAliases((lastOpen(input.Candles)+lastClose)/2, "snapshot.price_transform.oc2", "OC2")
	addAliases(hl2(input.Candles), "snapshot.price_transform.hl2", "HL2")
	addAliases(hlc3(input.Candles), "snapshot.price_transform.hlc3", "HLC3")
	addAliases(midpoint(cls, 14), "snapshot.price_transform.midpoint", "MIDPOINT")
	addAliases(midprice(input.Candles, 14), "snapshot.price_transform.midprice", "MIDPRICE")
	addAliases(math.Log(absDenominator(lastClose)), "snapshot.price_transform.log_price", "Log Price")
	addAliases(100*mathutil.SafeDiv(lastClose-firstClose(input.Candles), absDenominator(firstClose(input.Candles))), "snapshot.price_transform.percentage_price", "Percentage Price")
	add("Volume Moving Average", s.VolumeSMA20, "snapshot.volume_sma20")
	add("Volume", input.LastVolume, "candle.volume")
	add("Chaikin Money Flow", s.ChaikinMoneyFlow20, "snapshot.chaikin_money_flow20")
	add("CMF", s.ChaikinMoneyFlow20, "snapshot.chaikin_money_flow20")
	add("Accumulation Distribution Line", s.AccumulationDistribution, "snapshot.accumulation_distribution")
	add("ADL", s.AccumulationDistribution, "snapshot.accumulation_distribution")
	add("AD", s.AccumulationDistribution, "snapshot.accumulation_distribution")
	add("Ichimoku Kinko Hyo", s.IchimokuCloudTrend, "snapshot.ichimoku_cloud_trend")
	add("Ichimoku Cloud", s.IchimokuCloudTrend, "snapshot.ichimoku_cloud_trend")
	add("Tenkan-sen", s.IchimokuTenkan, "snapshot.ichimoku_tenkan")
	add("Kijun-sen", s.IchimokuKijun, "snapshot.ichimoku_kijun")
	add("Senkou Span A", s.IchimokuSenkouA, "snapshot.ichimoku_senkou_a")
	add("Senkou Span B", s.IchimokuSenkouB, "snapshot.ichimoku_senkou_b")
	add("Chikou Span", s.IchimokuChikou, "snapshot.ichimoku_chikou")
	add("Kumo Cloud", s.IchimokuCloudTrend, "snapshot.ichimoku_cloud_trend")
	add("Kumo Twist", s.IchimokuKumoTwist, "snapshot.ichimoku_kumo_twist")
	add("TK Cross", s.IchimokuTKCross, "snapshot.ichimoku_tk_cross")
	add("Pivot Points", s.PivotPoint, "snapshot.pivot_point")
	add("Classic Pivot Points", s.PivotPoint, "snapshot.pivot_point")
	addAliases(boolScore(movingAverageCrossed(input.Candles, 50, 200, true)), "snapshot.derived.cross.golden", "Golden Cross")
	addAliases(boolScore(movingAverageCrossed(input.Candles, 50, 200, false)), "snapshot.derived.cross.death", "Death Cross")
	for name, value := range s.FibonacciLevels {
		add("Fibonacci "+name, value, "snapshot.fibonacci_levels")
	}
	for name, value := range s.AdditionalIndicators {
		add(name, value, "snapshot.additional_indicators")
	}
	addAdditionalIndicatorAliases(addAliases, s.AdditionalIndicators)
	for name, value := range s.SupportTools {
		add(name, value, "snapshot.support_tools")
	}
	addSupportToolAliases(addAliases, s.SupportTools, s.FibonacciLevels)
	for name, value := range s.MarketStructure {
		add(name, value, "snapshot.market_structure")
	}
	addMarketStructureAliases(addAliases, s.MarketStructure)
	for name, value := range s.RelativeStrength {
		add(name, value, "snapshot.relative_strength")
	}
	addAliases(CumulativeReturn(cls), "snapshot.relative_strength.cumulative_return", "Cumulative Return", "Cumulative Return Price")
	addAliases(maxDrawdownPercent(cls), "snapshot.relative_strength.drawdown", "Drawdown", "Average Drawdown", "Maximum Drawdown")
	addAliases(choppinessIndex(input.Candles, 14), "snapshot.choppiness_index", "CHOP", "Choppiness Index")
	addDerivedStatAliases(addAliases, input.Candles, cls, vol)
	addDerivedMomentumAliases(addAliases, input.Candles, cls)
	addDerivedVolatilityAliases(addAliases, input.Candles, cls)
	addDerivedVolumeAliases(addAliases, input.Candles, vol)
	addDerivedLevelAliases(addAliases, input.Candles, cls)
	addAliases(latestRange, "snapshot.true_range", "TRANGE", "True Range")
	return m
}

func addAdditionalIndicatorAliases(addAliases func(float64, string, ...string), values map[string]float64) {
	alias := func(sourceName string, aliases ...string) {
		if value, ok := values[sourceName]; ok {
			addAliases(value, "snapshot.additional_indicators", aliases...)
		}
	}
	alias("Arnaud Legoux Moving Average", "ALMA")
	alias("Awesome Oscillator", "AO", "Elliott Wave Oscillator", "EWO")
	alias("Chande Momentum Oscillator", "CMO")
	alias("Connors RSI", "CRSI")
	alias("Detrended Price Oscillator", "DPO")
	alias("DEMA", "Double Exponential Moving Average")
	alias("Ease of Movement", "EOM")
	alias("Elder Ray Bull Power", "Bull Power")
	alias("Elder Ray Bull Power", "Elder Ray Index")
	alias("Elder Ray Bear Power", "Bear Power")
	alias("Elder Force Index", "Elder Force Index")
	alias("Force Index", "Force Index")
	alias("Hull Moving Average", "HMA")
	alias("Kaufman Adaptive Moving Average", "Adaptive Moving Average", "KAMA")
	alias("Klinger Oscillator", "KVO", "Klinger Volume Oscillator")
	alias("Klinger Oscillator Signal", "Klinger Signal")
	alias("Know Sure Thing", "KST")
	alias("Negative Volume Index", "NVI")
	alias("Positive Volume Index", "PVI")
	alias("Parabolic SAR", "PSAR")
	alias("Parabolic SAR", "SAR", "SAREXT", "SAR Trailing Stop")
	alias("Percentage Price Oscillator", "PPO")
	alias("Percentage Volume Oscillator", "PVO")
	alias("PVO Signal Line", "PVO Signal")
	alias("PVO Histogram", "PVO Hist")
	alias("Price Volume Trend", "PVT")
	alias("Schaff Trend Cycle", "STC")
	alias("Standard Deviation", "STDDEV")
	alias("TEMA", "Triple Exponential Moving Average")
	alias("TRIX", "Trix Indicator")
	alias("True Strength Index", "TSI")
	alias("Ultimate Oscillator", "ULTOSC")
	alias("Ultimate Oscillator", "UO")
	alias("Volume Price Trend", "VPT")
	alias("Volume Oscillator", "PVO", "Percentage Volume Oscillator")
	alias("Volume Weighted Moving Average", "VWMA")
	alias("Weighted Moving Average", "WMA")
	alias("Williams %R", "WILLR")
	alias("Aroon Oscillator", "AROONOSC")
	alias("Aroon Up", "AROON", "Aroon Indicator")
	alias("Vortex Indicator Plus", "VI+", "Vortex Indicator")
	alias("Vortex Indicator Minus", "VI-")
	alias("Guppy Multiple Moving Average Short", "GMMA", "Guppy Multiple Moving Average")
	alias("Linear Regression Line", "LINEARREG", "Linear Regression", "Linear Regression Trendline", "Linear Regression Moving Average", "Least Squares Moving Average", "LSMA", "TSF")
	alias("Linear Regression Slope", "LINEARREG_SLOPE", "Moving Average Slope")
	alias("Moving Average Envelope Upper", "Moving Average Envelope", "Moving Average Channel", "Percentage Envelope")
	alias("ZigZag", "Zig Zag")
	alias("Price Channel Upper", "Price Channel", "High Low Bands")
	alias("Chande Kroll Stop Long", "Chande Kroll Stop")
	alias("Momentum", "MOM")
	alias("Percentage Price Oscillator", "Price Oscillator")
	alias("Relative Vigor Index", "RVI")
	alias("Commodity Channel Index", "Woodies CCI")
	alias("Fisher Transform", "Ehlers Fisher Transform")
	alias("Stochastic Oscillator K", "Fast Stochastic", "Slow Stochastic", "STOCH", "STOCHF")
	alias("Stochastic Oscillator D", "STOCH Signal")
	alias("Stochastic Momentum Index", "SMI")
	alias("Stochastic Momentum Signal", "SMI Signal")
	alias("Chaikin Oscillator", "ADOSC", "Accumulation Distribution Oscillator")
	alias("On Balance Volume", "OBV")
	alias("Accumulation Distribution Line", "Accumulation Distribution Detector")
	alias("Chaikin Money Flow", "Twiggs Money Flow")
	alias("Volume Profile Point of Control", "VPOC", "Volume Point of Control", "TPO POC", "Time Price Opportunity POC", "dPOC")
	alias("Volume Profile Value Area High", "VAH")
	alias("Volume Profile Value Area Low", "VAL")
	alias("Anchored VWAP", "Anchored VWAP Levels")
	alias("Standard Deviation", "STDEV")
	alias("Historical Volatility", "HV")
	alias("Ulcer Index", "Ulcer Index")
	alias("Relative Volatility Index", "RVI Volatility")
	alias("Donchian Channel Upper", "Donchian Channel")
	alias("Keltner Channel Middle", "Keltner Channel")
	alias("Hilbert Transform Dominant Cycle Period", "HT_DCPERIOD", "Dominant Cycle", "Cycle Period")
	alias("Hilbert Transform Dominant Cycle Phase", "HT_DCPHASE", "Cycle Phase")
	alias("Hilbert Transform Trendline", "HT_TRENDLINE", "Ehlers Instantaneous Trendline")
	alias("Sine Wave", "HT_SINE", "SineWave Indicator", "Hilbert Transform SineWave")
	alias("Mass Index", "Mass Index")
	alias("Historical Volatility", "Historical Volatility", "Realized Volatility")
	alias("Chaikin Volatility", "Chaikin Volatility")
	alias("Relative Volatility Index", "Relative Volatility Index")
	alias("Bollinger Band Width", "Bollinger Bandwidth")
}

func addSupportToolAliases(addAliases func(float64, string, ...string), support map[string]float64, fib map[string]float64) {
	alias := func(sourceName string, aliases ...string) {
		if value, ok := support[sourceName]; ok {
			addAliases(value, "snapshot.support_tools", aliases...)
		}
	}
	alias("Camarilla Pivot Points H3", "Camarilla Pivot Points")
	alias("Classic Pivot Points", "CPR", "Central Pivot Range")
	alias("DeMark Pivot Points", "DeMark Pivot Points")
	alias("Fibonacci Extension 1.272", "Fibonacci Extension", "Auto Fib Extension", "Fibonacci Expansion")
	alias("Fibonacci Projection 1.618", "Fibonacci Projection")
	alias("Fibonacci Fan 0.618", "Fibonacci Fan")
	alias("Fibonacci Pivot Points R1", "Fibonacci Pivot Points")
	alias("Gann Fan 1x1", "Gann Fan", "Gann Line", "Gann Angles")
	alias("Gann Square", "Gann Square", "Gann Square of Nine")
	alias("Market Profile Point of Control", "Developing POC", "Fixed Range Volume Profile")
	alias("Murrey Math Lines", "Murrey Math Lines")
	alias("Point of Control", "POC", "POC Cluster")
	alias("Value Area High", "Developing VAH")
	alias("Value Area Low", "Developing VAL")
	alias("Volume Profile Visible Range", "Fixed Range Volume Profile", "Volume Profile")
	alias("Support Resistance Level", "Support Resistance Levels", "Auto Support Resistance", "Dynamic Support Resistance")
	alias("Previous Swing High", "Swing High Low")
	alias("Previous Swing Low", "Fractal Support Resistance")
	alias("Previous High", "Previous Day High", "Previous Week High", "Previous Month High")
	alias("Previous Low", "Previous Day Low", "Previous Week Low", "Previous Month Low")
	alias("VWAP Upper Band", "VWAP Bands")
	alias("VWAP Lower Band", "VWAP Band Lower")
	if value, ok := fib["0.618"]; ok {
		addAliases(value, "snapshot.fibonacci_levels", "Fibonacci Retracement", "Auto Fib Retracement", "Auto Fibonacci", "Fibonacci Confluence")
	}
}

func addMarketStructureAliases(addAliases func(float64, string, ...string), values map[string]float64) {
	alias := func(sourceName string, aliases ...string) {
		if value, ok := values[sourceName]; ok {
			addAliases(value, "snapshot.market_structure", aliases...)
		}
	}
	alias("Break of Structure", "BOS Indicator", "Break of Structure Indicator", "Market Structure Shift Indicator", "MSS Indicator")
	alias("Bullish Break of Structure", "Bullish Break of Structure Indicator", "Bullish BOS Indicator")
	alias("Bearish Break of Structure", "Bearish Break of Structure Indicator", "Bearish BOS Indicator")
	alias("Change of Character", "CHoCH Indicator", "Change of Character Indicator")
	alias("Fair Value Gap", "FVG Indicator", "Fair Value Gap Indicator", "Fair Value Gap Zones", "IFVG", "Inversion Fair Value Gap")
	alias("Liquidity Sweep", "Liquidity Sweep Detector", "Liquidity Sweep Indicator", "Buy Side Liquidity Indicator", "Sell Side Liquidity Indicator", "Liquidity Pool Indicator", "Liquidity Void Detector")
	alias("Order Block", "Order Block Indicator", "Order Block Zones", "Breaker Block Indicator", "Mitigation Block Indicator", "AMD Indicator")
	alias("Premium Discount Zone", "Premium Discount Zone", "Premium Discount Zone Indicator", "Optimal Trade Entry Indicator", "OTE Indicator", "Power of Three Indicator", "Balanced Price Range Indicator", "BPR Indicator")
	alias("Supply Zone", "Supply Zone", "Supply Demand Zones")
	alias("Demand Zone", "Demand Zone")
	alias("Higher High Higher Low Detection", "Gann Swing Chart")
	alias("Imbalance Detection", "Volume Imbalance Indicator")
}

func addDerivedStatAliases(addAliases func(float64, string, ...string), candles []ohlcv.Candle, closes, volumes []float64) {
	window := lastValues(closes, 60)
	ret := returns(closes)
	retWindow := lastValues(ret, 120)
	addAliases(mathutil.Mean(window), "snapshot.derived.stat.mean", "Mean")
	addAliases(median(window), "snapshot.derived.stat.median", "Median")
	addAliases(mode(window), "snapshot.derived.stat.mode", "Mode")
	addAliases(math.Pow(mathutil.StdDev(window), 2), "snapshot.derived.stat.variance", "Variance")
	addAliases(standardError(window), "snapshot.derived.stat.standard_error", "Standard Error")
	addAliases(skewness(window), "snapshot.derived.stat.skewness", "Skewness")
	addAliases(kurtosis(window), "snapshot.derived.stat.kurtosis", "Kurtosis")
	addAliases(shannonEntropy(retWindow, 10), "snapshot.derived.stat.entropy", "Entropy Indicator", "Shannon Entropy")
	addAliases(zScore(last(closes), window), "snapshot.derived.stat.zscore", "Z-Score", "Rolling Z-Score")
	addAliases(percentileRank(closes, 100), "snapshot.derived.stat.percentile_rank", "Percent Rank", "Percentile Rank")
	addAliases(lastReturnPercent(closes), "snapshot.derived.stat.simple_return", "Daily Return", "Simple Return", "Rolling Returns")
	addAliases(lastLogReturnPercent(closes), "snapshot.derived.stat.log_return", "Daily Log Return", "Log Return")
	addAliases(returnVolatility(closes, 20), "snapshot.derived.stat.rolling_volatility", "Rolling Volatility")
	addAliases(RollingSharpe(closes, 60), "snapshot.derived.stat.sharpe", "Sharpe Ratio")
	addAliases(RollingSortino(closes, 60), "snapshot.derived.stat.sortino", "Sortino Ratio")
	addAliases(calmarRatio(closes, 252), "snapshot.derived.stat.calmar", "Calmar Ratio")
	addAliases(historicalVaR(retWindow, 0.95), "snapshot.derived.stat.var", "VaR", "Value at Risk")
	addAliases(historicalCVaR(retWindow, 0.95), "snapshot.derived.stat.cvar", "CVaR", "Conditional Value at Risk")
	addAliases(winRate(retWindow), "snapshot.derived.stat.win_rate", "Win Rate")
	addAliases(profitFactor(retWindow), "snapshot.derived.stat.profit_factor", "Profit Factor")
	addAliases(expectancy(retWindow), "snapshot.derived.stat.expectancy", "Expectancy")
	addAliases(recoveryFactor(closes), "snapshot.derived.stat.recovery_factor", "Recovery Factor")
	addAliases(kellyCriterion(retWindow), "snapshot.derived.stat.kelly", "Kelly Criterion")
	addAliases(linearRegressionIntercept(closes, 20), "snapshot.derived.stat.linear_regression_intercept", "LINEARREG_INTERCEPT", "Linear Regression Intercept")
	addAliases(linearRegressionRSquared(closes, 20), "snapshot.derived.stat.linear_regression_r2", "Linear Regression R-Squared")
	addAliases(linearRegressionAngle(closes, 20), "snapshot.derived.stat.linear_regression_angle", "LINEARREG_ANGLE")
	addAliases(rollingCorrelation(closes, volumes, 60), "snapshot.derived.stat.rolling_correlation", "Rolling Correlation", "Correlation Coefficient", "CORREL")
	addAliases(hurstExponent(closes, 100), "snapshot.derived.stat.hurst", "Hurst Exponent")
	addAliases(fractalDimensionIndex(closes, 100), "snapshot.derived.stat.fdi", "FDI", "Fractal Dimension Index")
	addAliases(equityCurve(ret), "snapshot.derived.risk.equity_curve", "Equity Curve")
	addAliases(SMA(equityCurveSeries(ret), 20), "snapshot.derived.risk.equity_curve_ma", "Equity Curve Moving Average")
	_ = candles
}

func addDerivedMomentumAliases(addAliases func(float64, string, ...string), candles []ohlcv.Candle, closes []float64) {
	addAliases(centerOfGravity(closes, 10), "snapshot.derived.momentum.cog", "COG", "Center of Gravity", "Center of Gravity Oscillator")
	addAliases(acceleratorOscillator(candles), "snapshot.derived.momentum.accelerator", "AC", "Accelerator Oscillator")
	addAliases(coppockCurve(closes), "snapshot.derived.momentum.coppock", "Coppock Curve")
	addAliases(deMarker(candles, 14), "snapshot.derived.momentum.demarker", "DeMarker")
	addAliases(decisionPointPMO(closes), "snapshot.derived.momentum.pmo", "PMO", "DecisionPoint Price Momentum Oscillator")
	addAliases(inverseFisherRSI(closes, 14), "snapshot.derived.momentum.inverse_fisher", "Inverse Fisher Transform")
	addAliases(prettyGoodOscillator(candles, closes, 14), "snapshot.derived.momentum.pgo", "PGO", "Pretty Good Oscillator")
	addAliases(psychologicalLine(closes, 12), "snapshot.derived.momentum.psychological_line", "PSY", "Psychological Line")
	addAliases(relativeMomentumIndex(closes, 14, 5), "snapshot.derived.momentum.rmi", "RMI", "Relative Momentum Index")
	addAliases(trendTriggerFactor(candles, 15), "snapshot.derived.momentum.ttf", "TTF", "Trend Trigger Factor")
	addAliases(verticalHorizontalFilter(closes, 28), "snapshot.derived.trend.vhf", "VHF", "Vertical Horizontal Filter")
	addAliases(ravi(closes, 7, 65), "snapshot.derived.trend.ravi", "RAVI", "Range Action Verification Index")
	addAliases(trendIntensityIndex(closes, 30), "snapshot.derived.trend.tii", "TII", "Trend Intensity Index")
	addAliases(randomWalkIndex(candles, 14), "snapshot.derived.trend.rwi", "RWI", "Random Walk Index")
	addAliases(kaufmanEfficiencyRatio(closes, 10), "snapshot.derived.stat.kaufman_efficiency", "Kaufman Efficiency Ratio")
	addAliases(halfLifeMeanReversion(closes, 60), "snapshot.derived.stat.half_life", "Half Life of Mean Reversion")
	addAliases(qstick(candles, 8), "snapshot.derived.trend.qstick", "QStick")
	addAliases(rma(closes, 20), "snapshot.derived.trend.rma", "RMA", "Running Moving Average", "SMMA", "Smoothed Moving Average", "Wilders Moving Average")
	addAliases(triangularMovingAverage(closes, 20), "snapshot.derived.trend.trima", "TRIMA", "Triangular Moving Average")
	addAliases(tilsonT3(closes, 20, 0.7), "snapshot.derived.trend.t3", "T3", "T3 Moving Average", "Tilson T3")
	addAliases(zeroLagEMA(closes, 20), "snapshot.derived.trend.zlema", "ZLEMA", "Zero Lag Exponential Moving Average")
	addAliases(mcGinleyDynamic(closes, 14), "snapshot.derived.trend.mcginley", "McGinley Dynamic")
	addAliases(vidya(closes, 14), "snapshot.derived.trend.vidya", "VIDYA", "Variable Index Dynamic Average")
	addAliases(kalmanFilter(closes), "snapshot.derived.trend.kalman", "Kalman Filter", "Kalman Trend")
	jaw, teeth, lips := alligatorLines(candles)
	addAliases(jaw, "snapshot.derived.bill_williams.jaw", "Jaw")
	addAliases(teeth, "snapshot.derived.bill_williams.teeth", "Teeth")
	addAliases(lips, "snapshot.derived.bill_williams.lips", "Lips")
	addAliases(lips-teeth, "snapshot.derived.bill_williams.alligator", "Alligator", "Balance Line")
	addAliases(math.Abs(jaw-teeth)-math.Abs(teeth-lips), "snapshot.derived.bill_williams.gator", "Gator Oscillator")
	addAliases(lastFractal(candles), "snapshot.derived.bill_williams.fractals", "Fractals", "Fractal Chaos Oscillator")
	addAliases(tdSetupCount(closes), "snapshot.derived.demark.td_setup", "TD Setup", "TD Sequential", "DeMark Sequential")
	addAliases(chandelierExit(candles, 22, 3), "snapshot.derived.stop.chandelier", "Chandelier Exit")
	addAliases(darvasBoxTop(candles, 20), "snapshot.derived.channel.darvas", "Darvas Box")
	addAliases(100*mathutil.SafeDiv(ROC(closes, 12), 100), "snapshot.derived.momentum.rocp", "ROCP")
	addAliases(1+mathutil.SafeDiv(ROC(closes, 12), 100), "snapshot.derived.momentum.rocr", "ROCR")
	addAliases(100+ROC(closes, 12), "snapshot.derived.momentum.rocr100", "ROCR100")
}

func addDerivedVolatilityAliases(addAliases func(float64, string, ...string), candles []ohlcv.Candle, closes []float64) {
	_, _, keltnerLower := Keltner(candles, 20, 10, 2)
	addAliases(parkinsonVolatility(candles, 20), "snapshot.derived.volatility.parkinson", "Parkinson Volatility")
	addAliases(garmanKlassVolatility(candles, 20), "snapshot.derived.volatility.garman_klass", "Garman Klass Volatility")
	addAliases(rogersSatchellVolatility(candles, 20), "snapshot.derived.volatility.rogers_satchell", "Rogers Satchell Volatility")
	addAliases(yangZhangVolatility(candles, 20), "snapshot.derived.volatility.yang_zhang", "Yang Zhang Volatility")
	addAliases(math.Pow(mathutil.StdDev(lastValues(returns(closes), 20)), 2), "snapshot.derived.volatility.variance", "Variance")
	addAliases(mathutil.SafeDiv(ATR(candles, 14), absDenominator(ATR(candles, 100))), "snapshot.derived.volatility.ratio", "Volatility Ratio")
	addAliases(100*mathutil.SafeDiv(mathutil.StdDev(lastValues(closes, 20)), absDenominator(SMA(closes, 20))), "snapshot.derived.volatility.relative", "Range Percent")
	addAliases(BollingerBandWidth(closes, 20, 2), "snapshot.derived.volatility.bollinger_squeeze", "Bollinger Band Squeeze")
	addAliases(ttmSqueeze(candles), "snapshot.derived.volatility.ttm_squeeze", "TTM Squeeze", "Squeeze Momentum Indicator")
	addAliases(volatilityStop(candles, 14, 3), "snapshot.derived.volatility.stop", "Volatility Stop", "VStop", "ATR Trailing Stop")
	addAliases(kaseDevStop(candles, 14), "snapshot.derived.volatility.kase", "Kase DevStop", "Deviation Stop")
	addAliases(last(closes)+ATR(candles, 14), "snapshot.derived.channel.atr", "ATR Channel")
	addAliases(SMA(closes, 20)+2*ATR(candles, 14), "snapshot.derived.channel.starc", "STARC Bands")
	addAliases(LinearRegressionLine(closes, 20)+standardError(lastValues(closes, 20)), "snapshot.derived.channel.standard_error", "Standard Error Bands")
	addAliases(Supertrend(candles, 10, 3), "snapshot.derived.channel.supertrend", "Supertrend Bands", "SuperTrend AI")
	addAliases(priorWindowHigh(candles, 20), "snapshot.derived.stop.highest_high", "Highest High Stop")
	addAliases(priorWindowLow(candles, 20), "snapshot.derived.stop.lowest_low", "Lowest Low Stop", "Donchian Stop", "N-Bar Stop", "Swing High Low Stop")
	addAliases(keltnerLower, "snapshot.derived.stop.keltner", "Keltner Stop")
	addAliases(SMA(closes, 20)-2*ATR(candles, 14), "snapshot.derived.stop.moving_average", "Moving Average Stop", "Bollinger Stop")
	addAliases(LinearRegressionLine(closes, 20), "snapshot.derived.channel.regression", "Linear Regression Channel", "Regression Trend Channel", "Raff Regression Channel", "Trend Channel")
	addAliases(quantile(lastValues(closes, 60), 0.75), "snapshot.derived.channel.quantile", "Quantile Bands")
	addAliases(PivotPointValue(candles), "snapshot.derived.channel.pivot", "Pivot Bands")
}

func addDerivedVolumeAliases(addAliases func(float64, string, ...string), candles []ohlcv.Candle, volumes []float64) {
	lastVolume := last(volumes)
	volumeMA := SMA(volumes, 20)
	addAliases(mathutil.SafeDiv(lastVolume, volumeMA), "snapshot.derived.volume.relative", "Relative Volume", "RVOL", "Normalized Volume")
	addAliases(ROC(volumes, 20), "snapshot.derived.volume.roc", "Volume Rate of Change", "VROC")
	addAliases(netVolume(candles), "snapshot.derived.volume.net", "Net Volume", "Up Down Volume", "Delta Volume")
	addAliases(marketFacilitationIndex(candles), "snapshot.derived.volume.bw_mfi", "BW MFI", "Market Facilitation Index", "MFI Bill Williams")
	addAliases(intradayIntensityIndex(candles, 21), "snapshot.derived.volume.intraday_intensity", "Intraday Intensity Index")
	addAliases(moneyFlowVolume(candles), "snapshot.derived.volume.money_flow_volume", "Money Flow Volume")
	addAliases(rollingVWAP(candles, 20), "snapshot.derived.volume.rolling_vwap", "Rolling VWAP", "Session VWAP")
	addAliases(sum(volumes), "snapshot.derived.volume.cumulative_volume", "Cumulative Volume Index")
	addAliases(volumeZoneOscillator(candles, 14), "snapshot.derived.volume.vzo", "VZO", "Volume Zone Oscillator")
	addAliases(mathutil.Max(lastValues(highs(candles), 60)), "snapshot.derived.volume.high_volume_node", "HVN", "High Volume Node")
	addAliases(mathutil.Min(lastValues(lows(candles), 60)), "snapshot.derived.volume.low_volume_node", "LVN", "Low Volume Node")
	addAliases(VolumeProfile(candles, 24)["Point of Control"], "snapshot.derived.volume.vpoc", "VPOC Level", "NPOC", "Naked POC", "Naked Point of Control", "Virgin POC", "Virgin Point of Control", "Volume POC")
	addAliases(VolumeProfile(candles, 24)["Value Area High"], "snapshot.derived.volume.value_area", "Value Area")
	addAliases(VolumeProfile(candles, 24)["Visible Range"], "snapshot.derived.volume.vpvr", "VPVR", "Visible Range Volume Profile", "Volume Profile Levels")
}

func addDerivedLevelAliases(addAliases func(float64, string, ...string), candles []ohlcv.Candle, closes []float64) {
	if len(candles) == 0 {
		return
	}
	lastCandle := candles[len(candles)-1]
	prev := previousCandle(candles)
	addAliases(lastCandle.EffectiveHigh(), "snapshot.derived.level.daily_high", "Daily High Low", "Session High Low")
	addAliases(lastCandle.EffectiveOpen(), "snapshot.derived.level.daily_open", "Daily Open", "Midnight Open")
	addAliases(prev.EffectiveHigh(), "snapshot.derived.level.previous_day_high", "Previous Day High")
	addAliases(prev.EffectiveLow(), "snapshot.derived.level.previous_day_low", "Previous Day Low")
	addAliases((prev.EffectiveHigh()+prev.EffectiveLow())/2, "snapshot.derived.level.previous_day_mid", "Previous Day High Low")
	addAliases(windowOpen(candles, 5), "snapshot.derived.level.weekly_open", "Weekly Open")
	previousWeekHigh := priorWindowHigh(candles, 5)
	previousWeekLow := priorWindowLow(candles, 5)
	addAliases(previousWeekHigh, "snapshot.derived.level.previous_week_high", "Previous Week High")
	addAliases(previousWeekLow, "snapshot.derived.level.previous_week_low", "Previous Week Low")
	addAliases((previousWeekHigh+previousWeekLow)/2, "snapshot.derived.level.previous_week_mid", "Previous Week High Low", "Weekly High Low")
	addAliases(windowOpen(candles, 21), "snapshot.derived.level.monthly_open", "Monthly Open")
	previousMonthHigh := priorWindowHigh(candles, 21)
	previousMonthLow := priorWindowLow(candles, 21)
	addAliases(previousMonthHigh, "snapshot.derived.level.previous_month_high", "Previous Month High")
	addAliases(previousMonthLow, "snapshot.derived.level.previous_month_low", "Previous Month Low")
	addAliases((previousMonthHigh+previousMonthLow)/2, "snapshot.derived.level.previous_month_mid", "Previous Month High Low", "Monthly High Low")
	addAliases(windowOpen(candles, 63), "snapshot.derived.level.quarterly_open", "Quarterly Open")
	addAliases(windowOpen(candles, 252), "snapshot.derived.level.yearly_open", "Yearly Open")
	addAliases(nearestRoundNumber(last(closes)), "snapshot.derived.level.round_number", "Round Numbers", "Psychological Levels")
	addAliases(priorWindowHigh(candles, 20), "snapshot.derived.level.resistance", "Support Resistance Levels", "Auto Support Resistance", "Dynamic Support Resistance", "Swing High Low", "Fractal Support Resistance")
	addAliases(priorWindowLow(candles, 20), "snapshot.derived.level.support", "Supply Demand Zones", "Liquidity Zones", "Order Block Zones")
	addAliases(VWAP(candles)+ATR(candles, 14), "snapshot.derived.level.vwap_bands", "VWAP Bands")
	addAliases(100*mathutil.SafeDiv(last(closes)-SMA(closes, 20), absDenominator(SMA(closes, 20))), "snapshot.derived.price.ma_distance", "Price Distance from MA", "Moving Average Deviation")
	addAliases(candleBodyPercent(candles), "snapshot.derived.candle.body", "Candle Body Size Indicator")
	addAliases(wickRatio(candles), "snapshot.derived.candle.wick_ratio", "Wick Ratio Indicator")
	addAliases(gapPercent(candles), "snapshot.derived.candle.gap", "Gap Indicator")
	addAliases(heikinAshiClose(candles), "snapshot.derived.heikin_ashi.close", "Heikin Ashi", "Heikin Ashi Candles")
	addAliases(heikinAshiTrend(candles), "snapshot.derived.heikin_ashi.trend", "Heikin Ashi Trend", "Heikin Ashi Oscillator", "Heikin Ashi Smoothed", "Heikin Ashi Moving Average")
	addAliases(RSI(heikinAshiCloses(candles), 14), "snapshot.derived.heikin_ashi.rsi", "Heikin Ashi RSI")
	addAliases(EMA(heikinAshiCloses(candles), 12)-EMA(heikinAshiCloses(candles), 26), "snapshot.derived.heikin_ashi.macd", "Heikin Ashi MACD")
}

func directionalMovementValues(candles []ohlcv.Candle, period int) (float64, float64, float64) {
	if len(candles) <= period || period <= 0 {
		return 0, 0, 0
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
	for i := period + 1; i < len(candles); i++ {
		smoothedPlusDM = smoothedPlusDM - smoothedPlusDM/float64(period) + plusDM[i]
		smoothedMinusDM = smoothedMinusDM - smoothedMinusDM/float64(period) + minusDM[i]
		smoothedTR = smoothedTR - smoothedTR/float64(period) + tr[i]
	}
	plusDI := 100 * mathutil.SafeDiv(smoothedPlusDM, smoothedTR)
	minusDI := 100 * mathutil.SafeDiv(smoothedMinusDM, smoothedTR)
	dx := 100 * mathutil.SafeDiv(math.Abs(plusDI-minusDI), plusDI+minusDI)
	return plusDI, minusDI, dx
}

func directionalMovementRaw(candles []ohlcv.Candle, period int) (float64, float64) {
	if len(candles) <= period || period <= 0 {
		return 0, 0
	}
	plusDM := 0.0
	minusDM := 0.0
	start := maxInt(1, len(candles)-period)
	for i := start; i < len(candles); i++ {
		upMove := candles[i].EffectiveHigh() - candles[i-1].EffectiveHigh()
		downMove := candles[i-1].EffectiveLow() - candles[i].EffectiveLow()
		if upMove > downMove && upMove > 0 {
			plusDM += upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM += downMove
		}
	}
	return plusDM, minusDM
}

func averageDirectionalMovementRating(candles []ohlcv.Candle, period int) float64 {
	if len(candles) <= period*2 {
		return ADX(candles, period)
	}
	current := ADX(candles, period)
	prior := ADX(candles[:len(candles)-period], period)
	return (current + prior) / 2
}

func averageRange(candles []ohlcv.Candle, period int) float64 {
	start := maxInt(0, len(candles)-period)
	values := make([]float64, 0, len(candles)-start)
	for _, candle := range candles[start:] {
		values = append(values, candle.EffectiveHigh()-candle.EffectiveLow())
	}
	return mathutil.Mean(values)
}

func balanceOfPower(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return mathutil.SafeDiv(c.EffectiveClose()-c.EffectiveOpen(), c.EffectiveHigh()-c.EffectiveLow())
}

func averagePrice(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveOpen() + c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 4
}

func hl2(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveHigh() + c.EffectiveLow()) / 2
}

func hlc3(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 3
}

func cumulativeReturnPercent(values []float64) float64 {
	return CumulativeReturn(values)
}

func maxDrawdownPercent(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	peak := values[0]
	maxDrawdown := 0.0
	for _, value := range values {
		peak = math.Max(peak, value)
		drawdown := 100 * mathutil.SafeDiv(value-peak, absDenominator(peak))
		maxDrawdown = math.Min(maxDrawdown, drawdown)
	}
	return maxDrawdown
}

func choppinessIndex(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 || period <= 1 {
		return 0
	}
	start := maxInt(1, len(candles)-period+1)
	trSum := 0.0
	high := candles[start].EffectiveHigh()
	low := candles[start].EffectiveLow()
	for i := start; i < len(candles); i++ {
		prevClose := candles[i-1].EffectiveClose()
		trSum += math.Max(candles[i].EffectiveHigh()-candles[i].EffectiveLow(), math.Max(math.Abs(candles[i].EffectiveHigh()-prevClose), math.Abs(candles[i].EffectiveLow()-prevClose)))
		high = math.Max(high, candles[i].EffectiveHigh())
		low = math.Min(low, candles[i].EffectiveLow())
	}
	if high <= low || trSum <= 0 {
		return 0
	}
	return 100 * mathutil.SafeDiv(math.Log10(trSum/(high-low)), math.Log10(float64(len(candles)-start)))
}

func firstClose(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	return candles[0].EffectiveClose()
}

func lastOpen(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	return candles[len(candles)-1].EffectiveOpen()
}

func typicalPrice(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 3
}

func medianPrice(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveHigh() + c.EffectiveLow()) / 2
}

func weightedClosePrice(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveHigh() + c.EffectiveLow() + 2*c.EffectiveClose()) / 4
}

func ohlc4(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveOpen() + c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 4
}

func latestTrueRange(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	i := len(candles) - 1
	if i == 0 {
		return candles[i].EffectiveHigh() - candles[i].EffectiveLow()
	}
	prevClose := candles[i-1].EffectiveClose()
	return math.Max(candles[i].EffectiveHigh()-candles[i].EffectiveLow(), math.Max(math.Abs(candles[i].EffectiveHigh()-prevClose), math.Abs(candles[i].EffectiveLow()-prevClose)))
}

func previousCandle(candles []ohlcv.Candle) ohlcv.Candle {
	if len(candles) >= 2 {
		return candles[len(candles)-2]
	}
	if len(candles) == 1 {
		return candles[0]
	}
	return ohlcv.Candle{}
}

func windowOpen(candles []ohlcv.Candle, period int) float64 {
	if len(candles) == 0 {
		return 0
	}
	start := len(candles) - period
	if start < 0 {
		start = 0
	}
	return candles[start].EffectiveOpen()
}

func nearestRoundNumber(value float64) float64 {
	if value == 0 {
		return 0
	}
	magnitude := math.Pow(10, math.Floor(math.Log10(math.Abs(value))))
	step := magnitude / 10
	if step <= 0 {
		step = 1
	}
	return math.Round(value/step) * step
}

func candleBodyPercent(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return 100 * mathutil.SafeDiv(math.Abs(c.EffectiveClose()-c.EffectiveOpen()), absDenominator(c.EffectiveHigh()-c.EffectiveLow()))
}

func wickRatio(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	bodyTop := math.Max(c.EffectiveOpen(), c.EffectiveClose())
	bodyBottom := math.Min(c.EffectiveOpen(), c.EffectiveClose())
	upper := c.EffectiveHigh() - bodyTop
	lower := bodyBottom - c.EffectiveLow()
	return mathutil.SafeDiv(upper, lower)
}

func gapPercent(candles []ohlcv.Candle) float64 {
	if len(candles) < 2 {
		return 0
	}
	prevClose := candles[len(candles)-2].EffectiveClose()
	return 100 * mathutil.SafeDiv(candles[len(candles)-1].EffectiveOpen()-prevClose, absDenominator(prevClose))
}

func heikinAshiClose(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return (c.EffectiveOpen() + c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 4
}

func heikinAshiTrend(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	haOpen := candles[0].EffectiveOpen()
	haClose := heikinAshiClose(candles[:1])
	for i := 1; i < len(candles); i++ {
		haOpen = (haOpen + haClose) / 2
		c := candles[i]
		haClose = (c.EffectiveOpen() + c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 4
	}
	return haClose - haOpen
}

func heikinAshiCloses(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, candle := range candles {
		out[i] = (candle.EffectiveOpen() + candle.EffectiveHigh() + candle.EffectiveLow() + candle.EffectiveClose()) / 4
	}
	return out
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

func mode(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	counts := map[int]int{}
	bestKey := 0
	bestCount := 0
	for _, value := range values {
		key := int(math.Round(value * 100))
		counts[key]++
		if counts[key] > bestCount {
			bestCount = counts[key]
			bestKey = key
		}
	}
	return float64(bestKey) / 100
}

func standardError(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return mathutil.SafeDiv(mathutil.StdDev(values), math.Sqrt(float64(len(values))))
}

func skewness(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := mathutil.Mean(values)
	std := mathutil.StdDev(values)
	if mathutil.AlmostEqual(std, 0) {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += math.Pow((value-mean)/std, 3)
	}
	return total / float64(len(values))
}

func kurtosis(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := mathutil.Mean(values)
	std := mathutil.StdDev(values)
	if mathutil.AlmostEqual(std, 0) {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += math.Pow((value-mean)/std, 4)
	}
	return total/float64(len(values)) - 3
}

func shannonEntropy(values []float64, bins int) float64 {
	if len(values) == 0 || bins <= 0 {
		return 0
	}
	low := mathutil.Min(values)
	high := mathutil.Max(values)
	if mathutil.AlmostEqual(high, low) {
		return 0
	}
	counts := make([]float64, bins)
	width := (high - low) / float64(bins)
	for _, value := range values {
		idx := int(math.Floor((value - low) / width))
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		counts[idx]++
	}
	entropy := 0.0
	for _, count := range counts {
		if count == 0 {
			continue
		}
		p := count / float64(len(values))
		entropy -= p * math.Log2(p)
	}
	return entropy
}

func midpoint(values []float64, period int) float64 {
	window := lastValues(values, period)
	return (mathutil.Max(window) + mathutil.Min(window)) / 2
}

func midprice(candles []ohlcv.Candle, period int) float64 {
	return (mathutil.Max(lastValues(highs(candles), period)) + mathutil.Min(lastValues(lows(candles), period))) / 2
}

func zScore(value float64, window []float64) float64 {
	std := mathutil.StdDev(window)
	return mathutil.SafeDiv(value-mathutil.Mean(window), std)
}

func lastReturnPercent(values []float64) float64 {
	return DailyReturn(values)
}

func lastLogReturnPercent(values []float64) float64 {
	return DailyLogReturn(values)
}

func historicalVaR(rets []float64, confidence float64) float64 {
	if len(rets) == 0 {
		return 0
	}
	cp := append([]float64(nil), rets...)
	sort.Float64s(cp)
	idx := int(math.Floor((1 - confidence) * float64(len(cp))))
	idx = maxInt(0, minInt(len(cp)-1, idx))
	return cp[idx]
}

func historicalCVaR(rets []float64, confidence float64) float64 {
	if len(rets) == 0 {
		return 0
	}
	threshold := historicalVaR(rets, confidence)
	total := 0.0
	count := 0
	for _, ret := range rets {
		if ret <= threshold {
			total += ret
			count++
		}
	}
	return mathutil.SafeDiv(total, float64(count))
}

func winRate(rets []float64) float64 {
	if len(rets) == 0 {
		return 0
	}
	wins := 0
	for _, ret := range rets {
		if ret > 0 {
			wins++
		}
	}
	return 100 * mathutil.SafeDiv(float64(wins), float64(len(rets)))
}

func profitFactor(rets []float64) float64 {
	gains := 0.0
	losses := 0.0
	for _, ret := range rets {
		if ret > 0 {
			gains += ret
		} else if ret < 0 {
			losses += math.Abs(ret)
		}
	}
	return mathutil.SafeDiv(gains, losses)
}

func expectancy(rets []float64) float64 {
	return mathutil.Mean(rets)
}

func recoveryFactor(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	totalReturn := mathutil.SafeDiv(values[len(values)-1]-values[0], absDenominator(values[0]))
	return mathutil.SafeDiv(totalReturn, math.Abs(maxDrawdownPercent(values)/100))
}

func calmarRatio(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) < 2 {
		return 0
	}
	annualReturn := math.Pow(mathutil.SafeDiv(window[len(window)-1], absDenominator(window[0])), mathutil.SafeDiv(252, float64(len(window)-1))) - 1
	return mathutil.SafeDiv(annualReturn, math.Abs(maxDrawdownPercent(window)/100))
}

func kellyCriterion(rets []float64) float64 {
	wins := []float64{}
	losses := []float64{}
	for _, ret := range rets {
		if ret > 0 {
			wins = append(wins, ret)
		} else if ret < 0 {
			losses = append(losses, math.Abs(ret))
		}
	}
	winPct := mathutil.SafeDiv(float64(len(wins)), float64(len(rets)))
	avgWin := mathutil.Mean(wins)
	avgLoss := mathutil.Mean(losses)
	if avgWin <= 0 || avgLoss <= 0 {
		return 0
	}
	return winPct - mathutil.SafeDiv(1-winPct, avgWin/avgLoss)
}

func linearRegressionIntercept(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) == 0 {
		return 0
	}
	slope := linearRegressionSlopeWindow(window)
	xMean := float64(len(window)-1) / 2
	return mathutil.Mean(window) - slope*xMean
}

func linearRegressionRSquared(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) < 2 {
		return 0
	}
	slope := linearRegressionSlopeWindow(window)
	intercept := linearRegressionIntercept(window, len(window))
	mean := mathutil.Mean(window)
	ssTot := 0.0
	ssRes := 0.0
	for i, value := range window {
		fit := intercept + slope*float64(i)
		ssTot += math.Pow(value-mean, 2)
		ssRes += math.Pow(value-fit, 2)
	}
	return 1 - mathutil.SafeDiv(ssRes, ssTot)
}

func linearRegressionAngle(values []float64, period int) float64 {
	return math.Atan(linearRegressionSlopeWindow(lastValues(values, period))) * 180 / math.Pi
}

func linearRegressionSlopeWindow(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	xMean := float64(len(values)-1) / 2
	yMean := mathutil.Mean(values)
	num := 0.0
	den := 0.0
	for i, value := range values {
		x := float64(i)
		num += (x - xMean) * (value - yMean)
		den += math.Pow(x-xMean, 2)
	}
	return mathutil.SafeDiv(num, den)
}

func rollingCorrelation(a, b []float64, period int) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := minInt(len(a), len(b))
	if n > period {
		a = a[n-period : n]
		b = b[n-period : n]
	} else {
		a = a[:n]
		b = b[:n]
	}
	meanA := mathutil.Mean(a)
	meanB := mathutil.Mean(b)
	num := 0.0
	denA := 0.0
	denB := 0.0
	for i := range a {
		da := a[i] - meanA
		db := b[i] - meanB
		num += da * db
		denA += da * da
		denB += db * db
	}
	return mathutil.SafeDiv(num, math.Sqrt(denA*denB))
}

func hurstExponent(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) < 20 {
		return 0.5
	}
	lags := []float64{2, 4, 8, 16}
	xs := []float64{}
	ys := []float64{}
	for _, lag := range lags {
		l := int(lag)
		diffs := []float64{}
		for i := l; i < len(window); i++ {
			diffs = append(diffs, window[i]-window[i-l])
		}
		std := mathutil.StdDev(diffs)
		if std > 0 {
			xs = append(xs, math.Log(lag))
			ys = append(ys, math.Log(std))
		}
	}
	if len(xs) < 2 {
		return 0.5
	}
	return linearRegressionSlopeWindowXY(xs, ys)
}

func fractalDimensionIndex(values []float64, period int) float64 {
	hurst := hurstExponent(values, period)
	return mathutil.Clamp(2-hurst, 1, 2)
}

func linearRegressionSlopeWindowXY(xs, ys []float64) float64 {
	if len(xs) != len(ys) || len(xs) < 2 {
		return 0
	}
	xMean := mathutil.Mean(xs)
	yMean := mathutil.Mean(ys)
	num := 0.0
	den := 0.0
	for i := range xs {
		num += (xs[i] - xMean) * (ys[i] - yMean)
		den += math.Pow(xs[i]-xMean, 2)
	}
	return mathutil.SafeDiv(num, den)
}

func equityCurve(rets []float64) float64 {
	curve := equityCurveSeries(rets)
	return last(curve)
}

func equityCurveSeries(rets []float64) []float64 {
	out := make([]float64, len(rets))
	equity := 1.0
	for i, ret := range rets {
		equity *= 1 + ret
		out[i] = equity
	}
	return out
}

func centerOfGravity(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) == 0 {
		return 0
	}
	num := 0.0
	den := 0.0
	for i, value := range window {
		weight := float64(i + 1)
		num += weight * value
		den += value
	}
	return -mathutil.SafeDiv(num, den)
}

func awesomeOscillatorSeries(candles []ohlcv.Candle) []float64 {
	medianValues := make([]float64, len(candles))
	for i, candle := range candles {
		medianValues[i] = (candle.EffectiveHigh() + candle.EffectiveLow()) / 2
	}
	out := make([]float64, len(candles))
	for i := range candles {
		out[i] = SMA(medianValues[:i+1], 5) - SMA(medianValues[:i+1], 34)
	}
	return out
}

func acceleratorOscillator(candles []ohlcv.Candle) float64 {
	ao := awesomeOscillatorSeries(candles)
	return last(ao) - SMA(ao, 5)
}

func coppockCurve(values []float64) float64 {
	roc11 := rollingROC(values, 11)
	roc14 := rollingROC(values, 14)
	combined := make([]float64, minInt(len(roc11), len(roc14)))
	for i := range combined {
		combined[i] = roc11[len(roc11)-len(combined)+i] + roc14[len(roc14)-len(combined)+i]
	}
	return WMA(combined, 10)
}

func rollingROC(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	for i := range values {
		if i < period || mathutil.AlmostEqual(values[i-period], 0) {
			continue
		}
		out[i] = 100 * mathutil.SafeDiv(values[i]-values[i-period], absDenominator(values[i-period]))
	}
	return out
}

func deMarker(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < 2 {
		return 50
	}
	demax := []float64{}
	demin := []float64{}
	for i := 1; i < len(candles); i++ {
		demax = append(demax, math.Max(candles[i].EffectiveHigh()-candles[i-1].EffectiveHigh(), 0))
		demin = append(demin, math.Max(candles[i-1].EffectiveLow()-candles[i].EffectiveLow(), 0))
	}
	maxMA := SMA(demax, period)
	minMA := SMA(demin, period)
	return 100 * mathutil.SafeDiv(maxMA, maxMA+minMA)
}

func decisionPointPMO(values []float64) float64 {
	roc1 := rollingROC(values, 1)
	scaled := make([]float64, len(roc1))
	for i, value := range roc1 {
		scaled[i] = value * 10
	}
	return EMA(EMASeries(scaled, 35), 20)
}

func inverseFisherRSI(values []float64, period int) float64 {
	rsi := RSI(values, period)
	x := 0.1 * (rsi - 50)
	exp := math.Exp(2 * x)
	return mathutil.SafeDiv(exp-1, exp+1)
}

func prettyGoodOscillator(candles []ohlcv.Candle, values []float64, period int) float64 {
	return mathutil.SafeDiv(last(values)-SMA(values, period), ATR(candles, period))
}

func psychologicalLine(values []float64, period int) float64 {
	if len(values) < 2 {
		return 50
	}
	start := maxInt(1, len(values)-period)
	up := 0
	total := 0
	for i := start; i < len(values); i++ {
		if values[i] > values[i-1] {
			up++
		}
		total++
	}
	return 100 * mathutil.SafeDiv(float64(up), float64(total))
}

func relativeMomentumIndex(values []float64, period, momentum int) float64 {
	if len(values) <= momentum {
		return 50
	}
	gains := []float64{}
	losses := []float64{}
	for i := momentum; i < len(values); i++ {
		diff := values[i] - values[i-momentum]
		if diff > 0 {
			gains = append(gains, diff)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -diff)
		}
	}
	avgGain := SMA(gains, period)
	avgLoss := SMA(losses, period)
	return 100 * mathutil.SafeDiv(avgGain, avgGain+avgLoss)
}

func trendTriggerFactor(candles []ohlcv.Candle, period int) float64 {
	if len(candles) < period*2 {
		return 0
	}
	highsNow := highs(candles[len(candles)-period:])
	lowsNow := lows(candles[len(candles)-period:])
	highsPrev := highs(candles[len(candles)-period*2 : len(candles)-period])
	lowsPrev := lows(candles[len(candles)-period*2 : len(candles)-period])
	buyPower := mathutil.Max(highsNow) - mathutil.Min(lowsPrev)
	sellPower := mathutil.Max(highsPrev) - mathutil.Min(lowsNow)
	return 100 * mathutil.SafeDiv(buyPower-sellPower, 0.5*(buyPower+sellPower))
}

func verticalHorizontalFilter(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) < 2 {
		return 0
	}
	vertical := mathutil.Max(window) - mathutil.Min(window)
	horizontal := 0.0
	for i := 1; i < len(window); i++ {
		horizontal += math.Abs(window[i] - window[i-1])
	}
	return mathutil.SafeDiv(vertical, horizontal)
}

func ravi(values []float64, fast, slow int) float64 {
	slowMA := SMA(values, slow)
	return 100 * mathutil.SafeDiv(math.Abs(SMA(values, fast)-slowMA), absDenominator(slowMA))
}

func trendIntensityIndex(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) == 0 {
		return 50
	}
	ma := SMA(values, period)
	up := 0.0
	down := 0.0
	for _, value := range window {
		if value > ma {
			up += value - ma
		} else {
			down += ma - value
		}
	}
	return 100 * mathutil.SafeDiv(up, up+down)
}

func randomWalkIndex(candles []ohlcv.Candle, period int) float64 {
	if len(candles) <= period {
		return 0
	}
	atr := ATR(candles, period)
	if atr <= 0 {
		return 0
	}
	highsValues := highs(candles)
	lowsValues := lows(candles)
	best := 0.0
	for n := 2; n <= period; n++ {
		if len(candles) <= n {
			continue
		}
		rwiHigh := mathutil.SafeDiv(highsValues[len(highsValues)-1]-lowsValues[len(lowsValues)-1-n], atr*math.Sqrt(float64(n)))
		rwiLow := mathutil.SafeDiv(highsValues[len(highsValues)-1-n]-lowsValues[len(lowsValues)-1], atr*math.Sqrt(float64(n)))
		best = math.Max(best, math.Max(rwiHigh, rwiLow))
	}
	return best
}

func kaufmanEfficiencyRatio(values []float64, period int) float64 {
	if len(values) <= period {
		return 0
	}
	change := math.Abs(values[len(values)-1] - values[len(values)-1-period])
	volatility := 0.0
	for i := len(values) - period; i < len(values); i++ {
		volatility += math.Abs(values[i] - values[i-1])
	}
	return mathutil.SafeDiv(change, volatility)
}

func halfLifeMeanReversion(values []float64, period int) float64 {
	window := lastValues(values, period)
	if len(window) < 3 {
		return 0
	}
	y := []float64{}
	x := []float64{}
	for i := 1; i < len(window); i++ {
		y = append(y, window[i]-window[i-1])
		x = append(x, window[i-1]-mathutil.Mean(window))
	}
	beta := linearRegressionSlopeWindowXY(x, y)
	if beta >= 0 {
		return 0
	}
	return -math.Log(2) / beta
}

func qstick(candles []ohlcv.Candle, period int) float64 {
	values := make([]float64, len(candles))
	for i, candle := range candles {
		values[i] = candle.EffectiveClose() - candle.EffectiveOpen()
	}
	return SMA(values, period)
}

func rma(values []float64, period int) float64 {
	if len(values) == 0 || period <= 0 {
		return 0
	}
	alpha := 1 / float64(period)
	out := values[0]
	for _, value := range values[1:] {
		out = alpha*value + (1-alpha)*out
	}
	return out
}

func triangularMovingAverage(values []float64, period int) float64 {
	return SMA(simpleMovingAverageSeries(values, maxInt(1, (period+1)/2)), maxInt(1, (period+1)/2))
}

func tilsonT3(values []float64, period int, factor float64) float64 {
	e1 := EMASeries(values, period)
	e2 := EMASeries(e1, period)
	e3 := EMASeries(e2, period)
	e4 := EMASeries(e3, period)
	e5 := EMASeries(e4, period)
	e6 := EMASeries(e5, period)
	c1 := -factor * factor * factor
	c2 := 3*factor*factor + 3*factor*factor*factor
	c3 := -6*factor*factor - 3*factor - 3*factor*factor*factor
	c4 := 1 + 3*factor + 3*factor*factor + factor*factor*factor
	return c1*last(e6) + c2*last(e5) + c3*last(e4) + c4*last(e3)
}

func zeroLagEMA(values []float64, period int) float64 {
	if len(values) == 0 {
		return 0
	}
	lag := (period - 1) / 2
	adjusted := make([]float64, len(values))
	for i := range values {
		if i >= lag {
			adjusted[i] = values[i] + (values[i] - values[i-lag])
		} else {
			adjusted[i] = values[i]
		}
	}
	return EMA(adjusted, period)
}

func mcGinleyDynamic(values []float64, period int) float64 {
	if len(values) == 0 {
		return 0
	}
	md := values[0]
	for _, value := range values[1:] {
		ratio := mathutil.SafeDiv(value, absDenominator(md))
		md += mathutil.SafeDiv(value-md, float64(period)*math.Pow(ratio, 4))
	}
	return md
}

func vidya(values []float64, period int) float64 {
	if len(values) == 0 {
		return 0
	}
	alpha := 2 / float64(period+1)
	out := values[0]
	for i := 1; i < len(values); i++ {
		window := values[:i+1]
		cmo := math.Abs(ChandeMomentumOscillator(window, period)) / 100
		out = out + alpha*cmo*(values[i]-out)
	}
	return out
}

func kalmanFilter(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	q := 0.00001
	r := 0.001
	x := values[0]
	p := 1.0
	for _, value := range values[1:] {
		p += q
		k := mathutil.SafeDiv(p, p+r)
		x += k * (value - x)
		p *= 1 - k
	}
	return x
}

func alligatorLines(candles []ohlcv.Candle) (float64, float64, float64) {
	medianValues := make([]float64, len(candles))
	for i, candle := range candles {
		medianValues[i] = (candle.EffectiveHigh() + candle.EffectiveLow()) / 2
	}
	return shiftedRMA(medianValues, 13, 8), shiftedRMA(medianValues, 8, 5), shiftedRMA(medianValues, 5, 3)
}

func shiftedRMA(values []float64, period, shift int) float64 {
	if len(values) == 0 {
		return 0
	}
	end := len(values) - shift
	if end <= 0 {
		end = len(values)
	}
	return rma(values[:end], period)
}

func lastFractal(candles []ohlcv.Candle) float64 {
	if len(candles) < 5 {
		return 0
	}
	i := len(candles) - 3
	high := candles[i].EffectiveHigh()
	low := candles[i].EffectiveLow()
	isHigh := true
	isLow := true
	for j := i - 2; j <= i+2; j++ {
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
		return 1
	}
	if isLow {
		return -1
	}
	return 0
}

func tdSetupCount(values []float64) float64 {
	if len(values) < 5 {
		return 0
	}
	count := 0
	direction := 0
	for i := 4; i < len(values); i++ {
		currentDirection := 0
		if values[i] > values[i-4] {
			currentDirection = 1
		} else if values[i] < values[i-4] {
			currentDirection = -1
		}
		if currentDirection == 0 {
			count = 0
			direction = 0
			continue
		}
		if currentDirection != direction {
			count = 1
			direction = currentDirection
		} else {
			count++
		}
		if count > 9 {
			count = 9
		}
	}
	return float64(direction * count)
}

func chandelierExit(candles []ohlcv.Candle, period int, multiplier float64) float64 {
	if len(candles) == 0 {
		return 0
	}
	return mathutil.Max(lastValues(highs(candles), period)) - multiplier*ATR(candles, period)
}

func darvasBoxTop(candles []ohlcv.Candle, period int) float64 {
	return mathutil.Max(lastValues(highs(candles), period))
}

func quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	pos := q * float64(len(cp)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return cp[lower]
	}
	weight := pos - float64(lower)
	return cp[lower]*(1-weight) + cp[upper]*weight
}

func PivotPointValue(candles []ohlcv.Candle) float64 {
	pivot, _, _, _, _ := PivotPoints(candles)
	return pivot
}

func parkinsonVolatility(candles []ohlcv.Candle, period int) float64 {
	window := lastValues(candleRangeLogSquares(candles), period)
	return math.Sqrt(mathutil.SafeDiv(sum(window), 4*math.Log(2)*float64(maxInt(1, len(window))))) * math.Sqrt(252) * 100
}

func candleRangeLogSquares(candles []ohlcv.Candle) []float64 {
	out := make([]float64, 0, len(candles))
	for _, candle := range candles {
		if candle.EffectiveHigh() <= 0 || candle.EffectiveLow() <= 0 {
			out = append(out, 0)
			continue
		}
		out = append(out, math.Pow(math.Log(candle.EffectiveHigh()/candle.EffectiveLow()), 2))
	}
	return out
}

func garmanKlassVolatility(candles []ohlcv.Candle, period int) float64 {
	values := []float64{}
	for _, candle := range lastValuesCandles(candles, period) {
		if candle.EffectiveOpen() <= 0 || candle.EffectiveClose() <= 0 || candle.EffectiveHigh() <= 0 || candle.EffectiveLow() <= 0 {
			continue
		}
		hl := math.Log(candle.EffectiveHigh() / candle.EffectiveLow())
		co := math.Log(candle.EffectiveClose() / candle.EffectiveOpen())
		values = append(values, 0.5*hl*hl-(2*math.Log(2)-1)*co*co)
	}
	return math.Sqrt(math.Max(mathutil.Mean(values), 0)) * math.Sqrt(252) * 100
}

func rogersSatchellVolatility(candles []ohlcv.Candle, period int) float64 {
	values := []float64{}
	for _, candle := range lastValuesCandles(candles, period) {
		o := candle.EffectiveOpen()
		h := candle.EffectiveHigh()
		l := candle.EffectiveLow()
		c := candle.EffectiveClose()
		if o <= 0 || h <= 0 || l <= 0 || c <= 0 {
			continue
		}
		values = append(values, math.Log(h/c)*math.Log(h/o)+math.Log(l/c)*math.Log(l/o))
	}
	return math.Sqrt(math.Max(mathutil.Mean(values), 0)) * math.Sqrt(252) * 100
}

func yangZhangVolatility(candles []ohlcv.Candle, period int) float64 {
	window := lastValuesCandles(candles, period+1)
	if len(window) < 2 {
		return 0
	}
	openClose := []float64{}
	closeOpen := []float64{}
	rs := []float64{}
	for i := 1; i < len(window); i++ {
		prevClose := window[i-1].EffectiveClose()
		o := window[i].EffectiveOpen()
		h := window[i].EffectiveHigh()
		l := window[i].EffectiveLow()
		c := window[i].EffectiveClose()
		if prevClose <= 0 || o <= 0 || h <= 0 || l <= 0 || c <= 0 {
			continue
		}
		closeOpen = append(closeOpen, math.Log(o/prevClose))
		openClose = append(openClose, math.Log(c/o))
		rs = append(rs, math.Log(h/c)*math.Log(h/o)+math.Log(l/c)*math.Log(l/o))
	}
	n := float64(len(openClose))
	k := mathutil.SafeDiv(0.34, 1.34+mathutil.SafeDiv(n+1, n-1))
	variance := math.Pow(mathutil.StdDev(closeOpen), 2) + k*math.Pow(mathutil.StdDev(openClose), 2) + (1-k)*mathutil.Mean(rs)
	return math.Sqrt(math.Max(variance, 0)) * math.Sqrt(252) * 100
}

func ttmSqueeze(candles []ohlcv.Candle) float64 {
	closes := closes(candles)
	bbUpper, _, bbLower := BollingerBands(closes, 20, 2)
	kUpper, _, kLower := Keltner(candles, 20, 10, 1.5)
	if bbUpper < kUpper && bbLower > kLower {
		return 1
	}
	return 0
}

func volatilityStop(candles []ohlcv.Candle, period int, multiplier float64) float64 {
	if len(candles) == 0 {
		return 0
	}
	return candles[len(candles)-1].EffectiveClose() - multiplier*ATR(candles, period)
}

func kaseDevStop(candles []ohlcv.Candle, period int) float64 {
	if len(candles) == 0 {
		return 0
	}
	return candles[len(candles)-1].EffectiveClose() - 2.2*mathutil.StdDev(lastValues(trueRangeSeries(candles), period))
}

func trueRangeSeries(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i := range candles {
		if i == 0 {
			out[i] = candles[i].EffectiveHigh() - candles[i].EffectiveLow()
			continue
		}
		prevClose := candles[i-1].EffectiveClose()
		out[i] = math.Max(candles[i].EffectiveHigh()-candles[i].EffectiveLow(), math.Max(math.Abs(candles[i].EffectiveHigh()-prevClose), math.Abs(candles[i].EffectiveLow()-prevClose)))
	}
	return out
}

func lastValuesCandles(candles []ohlcv.Candle, period int) []ohlcv.Candle {
	if period <= 0 || len(candles) == 0 {
		return nil
	}
	start := len(candles) - period
	if start < 0 {
		start = 0
	}
	return candles[start:]
}

func netVolume(candles []ohlcv.Candle) float64 {
	total := 0.0
	for _, candle := range lastValuesCandles(candles, 20) {
		if candle.EffectiveClose() >= candle.EffectiveOpen() {
			total += candle.EffectiveVolume()
		} else {
			total -= candle.EffectiveVolume()
		}
	}
	return total
}

func marketFacilitationIndex(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return mathutil.SafeDiv(c.EffectiveHigh()-c.EffectiveLow(), c.EffectiveVolume())
}

func intradayIntensityIndex(candles []ohlcv.Candle, period int) float64 {
	num := 0.0
	den := 0.0
	for _, candle := range lastValuesCandles(candles, period) {
		rangeValue := candle.EffectiveHigh() - candle.EffectiveLow()
		num += mathutil.SafeDiv(2*candle.EffectiveClose()-candle.EffectiveHigh()-candle.EffectiveLow(), rangeValue) * candle.EffectiveVolume()
		den += candle.EffectiveVolume()
	}
	return 100 * mathutil.SafeDiv(num, den)
}

func moneyFlowVolume(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	c := candles[len(candles)-1]
	return ((c.EffectiveHigh() + c.EffectiveLow() + c.EffectiveClose()) / 3) * c.EffectiveVolume()
}

func rollingVWAP(candles []ohlcv.Candle, period int) float64 {
	return VWAP(lastValuesCandles(candles, period))
}

func volumeZoneOscillator(candles []ohlcv.Candle, period int) float64 {
	signed := make([]float64, len(candles))
	absolute := make([]float64, len(candles))
	for i, candle := range candles {
		absolute[i] = candle.EffectiveVolume()
		if i == 0 || candle.EffectiveClose() >= candles[i-1].EffectiveClose() {
			signed[i] = candle.EffectiveVolume()
		} else {
			signed[i] = -candle.EffectiveVolume()
		}
	}
	return 100 * mathutil.SafeDiv(EMA(signed, period), EMA(absolute, period))
}

func computeProxyValue(input ScannerInput, spec indicatorSpec) indicatorValue {
	c := input.Candles
	cls := closes(c)
	name := normalizeIndicatorText(spec.Name)
	source := proxySource
	switch spec.Template {
	case "moving_average":
		return indicatorValue{value: EMA(cls, 20), source: source, computed: true}
	case "trend":
		return indicatorValue{value: trendStrength(c), source: source, computed: true}
	case "momentum":
		return indicatorValue{value: ROC(cls, 12), source: source, computed: true}
	case "volume":
		return indicatorValue{value: mathutil.SafeDiv(input.LastVolume, SMA(volumes(c), 20)), source: source, computed: true}
	case "volatility":
		return indicatorValue{value: mathutil.SafeDiv(ATR(c, 14), absDenominator(input.LastClose)) * 100, source: source, computed: true}
	case "support_resistance", "channel", "fibonacci", "gann":
		return indicatorValue{value: nearestLevelProxy(input), source: source, computed: true}
	case "market_structure", "smart_money", "wyckoff", "pattern_recognition":
		return indicatorValue{value: structureProxy(input), source: source, computed: true}
	case "cycle":
		return indicatorValue{value: DominantCyclePeriod(cls), source: source, computed: true}
	case "statistical", "risk_performance":
		return indicatorValue{value: statisticalProxy(name, cls), source: source, computed: true}
	case "price_transform":
		return indicatorValue{value: priceTransformProxy(name, c), source: source, computed: true}
	case "stop":
		return indicatorValue{value: stopProxy(name, input), source: source, computed: true}
	case "divergence":
		return indicatorValue{value: divergenceProxy(input), source: source, computed: true}
	case "elliott":
		return indicatorValue{value: ZigZag(c, 0.05), source: source, computed: true}
	case "renko_kagi_pnf", "heikin_ashi", "session_time":
		return indicatorValue{value: chartTransformProxy(name, c), source: source, computed: true}
	}
	if strings.Contains(name, "moving average") || strings.HasSuffix(name, " ma") {
		return indicatorValue{value: EMA(cls, 20), source: source, computed: true}
	}
	if strings.Contains(name, "oscillator") || strings.Contains(name, "momentum") {
		return indicatorValue{value: ROC(cls, 12), source: source, computed: true}
	}
	if strings.Contains(name, "volume") {
		return indicatorValue{value: mathutil.SafeDiv(input.LastVolume, SMA(volumes(c), 20)), source: source, computed: true}
	}
	if strings.Contains(name, "volatility") || strings.Contains(name, "range") {
		return indicatorValue{value: mathutil.SafeDiv(ATR(c, 14), absDenominator(input.LastClose)) * 100, source: source, computed: true}
	}
	return indicatorValue{value: ROC(cls, 20), source: source, computed: true}
}

func signalForIndicator(input ScannerInput, spec indicatorSpec, value float64) (string, float64, string) {
	name := normalizeIndicatorText(spec.Name)
	lastClose := input.LastClose
	s := input.Snapshot
	confidence := spec.Confidence
	if confidence <= 0 {
		confidence = 0.55
	}
	if isMACDIndicator(name) {
		return macdIndicatorSignal(s, confidence)
	}
	if isChikouSpanIndicator(name) {
		return signedStateSignal(value, confidence, "Chikou confirms current close is above the comparable past close", "Chikou warns current close is below the comparable past close", "Chikou is neutral against the comparable past close")
	}
	if isIchimokuStateIndicator(name) {
		return signedStateSignal(value, confidence, "Ichimoku cloud state is bullish", "Ichimoku cloud state is bearish", "Ichimoku cloud state is neutral")
	}
	if isBillWilliamsMFIIndicator(name) {
		return "info", confidence * 0.7, "Bill Williams Market Facilitation Index is not directional without volume/spread color-state confirmation"
	}
	if isVolumeParticipationIndicator(name) {
		return volumeParticipationSignal(input.LastVolume, s.VolumeSMA20, confidence)
	}
	if isVolumeWeightedPriceLevel(name) {
		return priceLevelSignal(lastClose, value, confidence, "price is above volume-weighted reference level", "price is below volume-weighted reference level")
	}
	if isMoneyFlowIndicator(name) {
		return moneyFlowSignal(value, confidence)
	}
	if isCumulativeVolumeIndicator(name) {
		return "info", confidence * 0.7, "cumulative volume indicator needs slope confirmation"
	}
	if strings.Contains(name, "cross") || strings.Contains(name, "golden") || strings.Contains(name, "death") {
		if strings.Contains(name, "death") || strings.Contains(name, "bearish") {
			return signalBool(movingAverageCrossed(input.Candles, 50, 200, false), "bearish", "neutral", confidence, "moving averages crossed on the latest bar")
		}
		return signalBool(movingAverageCrossed(input.Candles, 50, 200, true), "bullish", "neutral", confidence, "moving averages crossed on the latest bar")
	}
	if name == "adx" || name == "average directional index" {
		return adxSignal(value, confidence)
	}
	if strings.Contains(name, "williams") {
		return williamsRSignal(value, confidence, "Williams %R oscillator value evaluated")
	}
	if isCCIIndicator(name) {
		return centeredThresholdSignal(value, confidence, 100, "CCI oscillator value evaluated")
	}
	if strings.Contains(name, "bollinger") && strings.Contains(name, "percent b") {
		return percentBSignal(value, confidence, "Bollinger %B value evaluated")
	}
	if name == "moving average ribbon" {
		return signedStateSignal(value, confidence, "moving average ribbon is bullish", "moving average ribbon is bearish", "moving average ribbon is neutral")
	}
	if strings.Contains(name, "time zone") {
		return "info", confidence * 0.7, "time projection indicator is not a price level"
	}
	switch spec.Template {
	case "moving_average", "trend":
		if !isPriceLevelTrendIndicator(name) {
			return "info", confidence * 0.7, "trend indicator is not price-scaled; directional price comparison was skipped"
		}
		if lastClose > value {
			return "bullish", confidence, "price is above trend indicator value"
		}
		if lastClose < value {
			return "bearish", confidence, "price is below trend indicator value"
		}
	case "momentum":
		if isBoundedOscillator(name) {
			return oscillatorSignal(value, confidence, "bounded momentum oscillator value evaluated")
		}
		return centeredMomentumSignal(value, confidence, "centered momentum indicator value evaluated")
	case "volume":
		return "info", confidence * 0.7, "volume-derived indicator needs specific confirmation"
	case "volatility":
		if value > 4 || strings.Contains(name, "squeeze") && BollingerBandWidth(closes(input.Candles), 20, 2) < 8 {
			return "high_volatility", confidence, "volatility condition is elevated or compressed setup is active"
		}
		return "neutral", confidence * 0.65, "volatility condition is not extreme"
	case "support_resistance", "channel", "fibonacci", "gann":
		if value == 0 {
			return "neutral", confidence * 0.5, "no nearby level was derived"
		}
		if math.Abs(lastClose-value)/absDenominator(lastClose) <= 0.02 {
			return "level_nearby", confidence, "price is near derived support/resistance level"
		}
		return "neutral", confidence * 0.6, "derived level is not near current price"
	case "market_structure", "smart_money", "wyckoff", "pattern_recognition":
		if value > 0.65 {
			return "bullish", confidence, "market-structure proxy is active"
		}
		if value < -0.65 {
			return "bearish", confidence, "market-structure proxy is active"
		}
		return "neutral", confidence * 0.6, "market-structure proxy is not directional"
	case "divergence":
		if value > 0 {
			return "bullish", confidence, "bullish divergence proxy matched"
		}
		if value < 0 {
			return "bearish", confidence, "bearish divergence proxy matched"
		}
		return "neutral", confidence * 0.5, "no divergence proxy matched"
	case "risk_performance", "statistical", "cycle", "price_transform", "renko_kagi_pnf", "heikin_ashi", "session_time":
		return "info", confidence * 0.75, "indicator value was computed for informational use"
	}
	if strings.Contains(name, "rsi") || strings.Contains(name, "stochastic") || strings.Contains(name, "mfi") {
		return oscillatorSignal(value, confidence, "oscillator value evaluated")
	}
	if value > 0 {
		return "bullish", confidence * 0.8, "positive indicator value"
	}
	if value < 0 {
		return "bearish", confidence * 0.8, "negative indicator value"
	}
	return "neutral", confidence * 0.5, "indicator value is neutral"
}

func absDenominator(value float64) float64 {
	return math.Max(math.Abs(value), mathutil.Epsilon)
}

func isMACDIndicator(name string) bool {
	return name == "macd" ||
		name == "macd histogram" ||
		name == "macd signal line" ||
		strings.Contains(name, "moving average convergence divergence")
}

func isCCIIndicator(name string) bool {
	if name == "commodity channel index" || strings.Contains(name, "commodity channel") {
		return true
	}
	normalized := strings.NewReplacer("-", " ", "_", " ", "/", " ", ".", " ").Replace(name)
	for _, token := range strings.Fields(normalized) {
		if token == "cci" {
			return true
		}
	}
	return false
}

func macdIndicatorSignal(snapshot ohlcv.IndicatorSnapshot, confidence float64) (string, float64, string) {
	switch {
	case snapshot.MACDHistogram > 0 && snapshot.MACD > snapshot.MACDSignal:
		return "bullish", confidence, "MACD line is above signal and histogram is positive"
	case snapshot.MACDHistogram < 0 && snapshot.MACD < snapshot.MACDSignal:
		return "bearish", confidence, "MACD line is below signal and histogram is negative"
	case snapshot.MACDHistogram > 0:
		return "bullish", confidence * 0.8, "MACD histogram is positive"
	case snapshot.MACDHistogram < 0:
		return "bearish", confidence * 0.8, "MACD histogram is negative"
	default:
		return "neutral", confidence * 0.55, "MACD line and histogram are neutral"
	}
}

func isIchimokuStateIndicator(name string) bool {
	switch name {
	case "ichimoku kinko hyo", "ichimoku cloud", "kumo cloud", "kumo twist", "tk cross":
		return true
	default:
		return false
	}
}

func isChikouSpanIndicator(name string) bool {
	return name == "chikou span"
}

func isBillWilliamsMFIIndicator(name string) bool {
	return name == "bw mfi" ||
		name == "mfi bill williams" ||
		strings.Contains(name, "market facilitation index")
}

func signedStateSignal(value, confidence float64, positiveEvidence, negativeEvidence, neutralEvidence string) (string, float64, string) {
	if value > 0 {
		return "bullish", confidence, positiveEvidence
	}
	if value < 0 {
		return "bearish", confidence, negativeEvidence
	}
	return "neutral", confidence * 0.55, neutralEvidence
}

func isVolumeParticipationIndicator(name string) bool {
	switch name {
	case "volume", "volume moving average":
		return true
	default:
		return false
	}
}

func volumeParticipationSignal(lastVolume, averageVolume, confidence float64) (string, float64, string) {
	if lastVolume <= 0 || averageVolume <= 0 {
		return "neutral", confidence * 0.5, "volume average comparison is not available"
	}
	ratio := lastVolume / averageVolume
	switch {
	case ratio >= 1.2:
		return "bullish", confidence, "volume expanded above recent average"
	case ratio <= 0.8:
		return "bearish", confidence * 0.85, "volume contracted below recent average"
	default:
		return "neutral", confidence * 0.65, "volume is near recent average"
	}
}

func isVolumeWeightedPriceLevel(name string) bool {
	return strings.Contains(name, "vwap") ||
		strings.Contains(name, "volume weighted average price") ||
		strings.Contains(name, "point of control")
}

func priceLevelSignal(lastClose, level, confidence float64, aboveEvidence, belowEvidence string) (string, float64, string) {
	if lastClose <= 0 || level <= 0 {
		return "neutral", confidence * 0.5, "price level comparison is not available"
	}
	if lastClose > level {
		return "bullish", confidence, aboveEvidence
	}
	if lastClose < level {
		return "bearish", confidence, belowEvidence
	}
	return "neutral", confidence * 0.55, "price is on the reference level"
}

func isMoneyFlowIndicator(name string) bool {
	return name == "chaikin money flow" || name == "cmf"
}

func moneyFlowSignal(value, confidence float64) (string, float64, string) {
	switch {
	case value >= 0.05:
		return "bullish", confidence, "Chaikin money flow is positive"
	case value <= -0.05:
		return "bearish", confidence, "Chaikin money flow is negative"
	default:
		return "neutral", confidence * 0.6, "Chaikin money flow is near neutral"
	}
}

func isCumulativeVolumeIndicator(name string) bool {
	return name == "obv" ||
		name == "on balance volume" ||
		name == "adl" ||
		strings.Contains(name, "accumulation distribution") ||
		strings.Contains(name, "price volume trend") ||
		strings.Contains(name, "volume price trend") ||
		strings.Contains(name, "positive volume index") ||
		strings.Contains(name, "negative volume index")
}

func oscillatorSignal(value, confidence float64, evidence string) (string, float64, string) {
	if value >= 70 {
		return "overbought", confidence, evidence
	}
	if value <= 30 {
		return "oversold", confidence, evidence
	}
	if value > 55 {
		return "bullish", confidence * 0.8, evidence
	}
	if value < 45 {
		return "bearish", confidence * 0.8, evidence
	}
	return "neutral", confidence * 0.55, evidence
}

func williamsRSignal(value, confidence float64, evidence string) (string, float64, string) {
	if value >= -20 {
		return "overbought", confidence, evidence
	}
	if value <= -80 {
		return "oversold", confidence, evidence
	}
	if value > -45 {
		return "bullish", confidence * 0.8, evidence
	}
	if value < -55 {
		return "bearish", confidence * 0.8, evidence
	}
	return "neutral", confidence * 0.55, evidence
}

func percentBSignal(value, confidence float64, evidence string) (string, float64, string) {
	if value >= 0.8 {
		return "overbought", confidence, evidence
	}
	if value <= 0.2 {
		return "oversold", confidence, evidence
	}
	if value > 0.55 {
		return "bullish", confidence * 0.8, evidence
	}
	if value < 0.45 {
		return "bearish", confidence * 0.8, evidence
	}
	return "neutral", confidence * 0.55, evidence
}

func centeredThresholdSignal(value, confidence, threshold float64, evidence string) (string, float64, string) {
	if value >= threshold {
		return "overbought", confidence, evidence
	}
	if value <= -threshold {
		return "oversold", confidence, evidence
	}
	return centeredMomentumSignal(value, confidence, evidence)
}

func centeredMomentumSignal(value, confidence float64, evidence string) (string, float64, string) {
	if value > 0 {
		return "bullish", confidence * 0.8, evidence
	}
	if value < 0 {
		return "bearish", confidence * 0.8, evidence
	}
	return "neutral", confidence * 0.55, evidence
}

func isBoundedOscillator(name string) bool {
	for _, token := range []string{"rsi", "relative strength index", "stochastic", "mfi", "money flow", "ultimate oscillator"} {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func signalBool(ok bool, yes, no string, confidence float64, evidence string) (string, float64, string) {
	if ok {
		return yes, confidence, evidence
	}
	return no, confidence * 0.55, evidence
}

func adxSignal(value, confidence float64) (string, float64, string) {
	switch {
	case value >= 25:
		return "strong_trend", confidence, "ADX indicates trend strength, not direction"
	case value >= 20:
		return "emerging_trend", confidence * 0.85, "ADX indicates emerging trend strength, not direction"
	default:
		return "weak_trend", confidence * 0.65, "ADX indicates weak trend strength, not direction"
	}
}

func isPriceLevelTrendIndicator(name string) bool {
	if strings.Contains(name, "moving average") || strings.Contains(name, "sma") || strings.Contains(name, "ema") || strings.Contains(name, "wma") || strings.Contains(name, "hma") {
		return true
	}
	for _, token := range []string{"vwap", "supertrend", "parabolic", "sar", "donchian", "keltner", "ichimoku", "tenkan", "kijun", "senkou", "pivot"} {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func movingAverageCrossed(candles []ohlcv.Candle, fast, slow int, bullish bool) bool {
	if len(candles) < slow+1 || fast <= 0 || slow <= 0 {
		return false
	}
	cls := closes(candles)
	fastSeries := simpleMovingAverageSeries(cls, fast)
	slowSeries := simpleMovingAverageSeries(cls, slow)
	last := len(cls) - 1
	prev := last - 1
	if bullish {
		return fastSeries[prev] <= slowSeries[prev] && fastSeries[last] > slowSeries[last]
	}
	return fastSeries[prev] >= slowSeries[prev] && fastSeries[last] < slowSeries[last]
}

func simpleMovingAverageSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 0 {
		return out
	}
	sum := 0.0
	for i, value := range values {
		sum += value
		if i >= period {
			sum -= values[i-period]
		}
		count := i + 1
		if count > period {
			count = period
		}
		out[i] = mathutil.SafeDiv(sum, float64(count))
	}
	return out
}

func isExternalOnly(spec indicatorSpec) bool {
	switch spec.Template {
	case "breadth", "sentiment", "order_flow", "options", "crypto_onchain", "market_profile_external":
		return true
	}
	name := normalizeIndicatorText(spec.Name)
	switch name {
	case "alpha", "beta", "rolling beta", "treynor ratio", "information ratio", "tracking error", "relative strength comparative", "relative strength ratio", "relative rotation graph", "rrg", "jdk rs momentum", "jdk rs ratio", "mansfield rs", "mansfield relative strength":
		return true
	case "bid ask delta", "cumulative volume delta", "cvd", "volume imbalance indicator", "buy sell volume":
		return true
	case "iv", "implied volatility", "iv rank", "iv percentile", "vix", "volatility index", "volatility smile", "volatility skew", "volatility cone", "news volatility indicator":
		return true
	}
	if strings.Contains(name, "session") || strings.Contains(name, "killzone") || strings.Contains(name, "london range") || strings.Contains(name, "new york range") || strings.Contains(name, "asian range") || strings.Contains(name, "asia ") || strings.Contains(name, "opening range") || name == "orb" || strings.Contains(name, "trading hours") {
		return true
	}
	if strings.Contains(name, "pitchfork") || strings.Contains(name, "gann time") || strings.Contains(name, "price time") {
		return true
	}
	return false
}

func isAlgorithmRequired(spec indicatorSpec) bool {
	name := normalizeIndicatorText(spec.Name)
	template := strings.ToLower(strings.TrimSpace(spec.Template))
	category := strings.ToLower(strings.TrimSpace(spec.Category))
	switch template {
	case "pattern_recognition", "divergence", "renko_kagi_pnf", "wyckoff", "elliott", "demark", "gann":
		return true
	}
	switch category {
	case "pattern_recognition", "renko_kagi_pnf", "wyckoff", "elliott", "demark":
		return true
	}
	if strings.Contains(name, "divergence") ||
		strings.Contains(name, "scanner") ||
		strings.Contains(name, "detector") ||
		strings.Contains(name, "harmonic") ||
		strings.Contains(name, "elliott") ||
		strings.Contains(name, "wyckoff") ||
		strings.Contains(name, "renko") ||
		strings.Contains(name, "kagi") ||
		strings.Contains(name, "point and figure") ||
		strings.Contains(name, "p and f") ||
		strings.Contains(name, "line break") ||
		strings.Contains(name, "three line break") ||
		strings.Contains(name, "range bars") ||
		strings.Contains(name, "tick bars") ||
		strings.Contains(name, "volume bars") ||
		strings.Contains(name, "dollar bars") {
		return true
	}
	switch name {
	case "avwap", "anchored vwap":
		return true
	case "macdext", "macdfix":
		return true
	case "bollinger bands", "bbands", "donchian channel", "keltner channel":
		return true
	case "cloud support resistance", "kijun bounce", "kumo breakout":
		return true
	case "daily high low", "session high low", "daily open", "midnight open",
		"previous day high", "previous day low", "previous day high low",
		"weekly open", "previous week high", "previous week low", "previous week high low", "weekly high low",
		"monthly open", "previous month high", "previous month low", "previous month high low", "monthly high low",
		"quarterly open", "yearly open":
		return true
	case "support resistance levels", "auto support resistance", "dynamic support resistance",
		"swing high low", "fractal support resistance", "supply demand zones",
		"liquidity zones", "order block zones":
		return true
	case "mama", "fama", "mesa adaptive moving average", "jma", "jurik moving average", "frama", "fractal adaptive moving average", "following adaptive moving average", "mavp":
		return true
	case "ehlers mesa stochastic", "ehlers roofing filter", "ehlers super smoother", "cyber cycle", "decycler oscillator", "ht phasor", "hilbert transform phasor components", "ht trendmode", "hilbert transform trend mode", "hurst cycle channel":
		return true
	case "augmented dickey fuller signal", "polarized fractal efficiency":
		return true
	case "asi", "accumulation swing index", "csi", "commodity selection index", "hpi", "herrick payoff index", "tvi", "trade volume index", "vfi", "volume flow indicator", "vsa", "volume spread analysis", "demand index":
		return true
	case "dynamic momentum index", "ergodic oscillator", "laguerre rsi", "swing index", "twiggs momentum", "twiggs volatility":
		return true
	case "elder triple screen", "elder impulse system", "coral trend indicator", "trend magic", "stepma", "gaussian moving average", "gaussian channel", "fractal chaos bands", "fractal stop":
		return true
	case "equal highs equal lows indicator", "judas swing indicator", "silver bullet zone indicator":
		return true
	case "ichimoku price target", "ichimoku time theory", "ichimoku wave":
		return true
	case "fibonacci arc", "fibonacci circle", "fibonacci spiral", "fibonacci wedge", "fibonacci speed resistance fan", "fibonacci channel", "fibonacci cluster", "fibonacci bollinger bands":
		return true
	case "gann box", "gann grid", "gann levels", "gann wheel", "gann hilo activator":
		return true
	case "prime number bands", "prime number oscillator", "projection bands", "projection oscillator", "rainbow charts", "rainbow moving average", "rainbow oscillator":
		return true
	case "risk reward ratio":
		return true
	}
	return false
}

func externalDataEvidence(spec indicatorSpec) string {
	switch spec.Template {
	case "breadth":
		return "requires market-wide breadth data, not available in single-symbol OHLCV"
	case "sentiment":
		return "requires sentiment/options/positioning data, not available in single-symbol OHLCV"
	case "order_flow":
		return "requires order book, bid/ask or footprint data, not available in OHLCV"
	case "options":
		return "requires options chain/greeks data, not available in OHLCV"
	case "crypto_onchain":
		return "requires exchange/on-chain crypto data, not available in OHLCV"
	case "market_profile_external":
		return "requires intraday TPO/session profile data; OHLCV proxy was not sufficient"
	default:
		return "requires external data not available in current scanner input"
	}
}

func algorithmRequiredEvidence(spec indicatorSpec) string {
	template := strings.TrimSpace(spec.Template)
	if template == "" {
		template = "indicator"
	}
	return fmt.Sprintf("%s requires a dedicated %s algorithm; scanner refuses OHLCV proxy values until that algorithm is implemented and validated", spec.Name, template)
}

func trendStrength(c []ohlcv.Candle) float64 {
	if len(c) < 30 {
		return 0
	}
	cls := closes(c)
	base := cls[maxInt(0, len(cls)-30)]
	return mathutil.SafeDiv(last(cls)-base, absDenominator(base)) * 100
}

func nearestLevelProxy(input ScannerInput) float64 {
	lastClose := input.LastClose
	best := 0.0
	bestDist := math.MaxFloat64
	consider := func(value float64) {
		if value <= 0 {
			return
		}
		dist := math.Abs(lastClose - value)
		if dist < bestDist {
			bestDist = dist
			best = value
		}
	}
	s := input.Snapshot
	for _, value := range []float64{s.PivotPoint, s.PivotR1, s.PivotR2, s.PivotS1, s.PivotS2, s.BollingerUpper, s.BollingerLower, s.DonchianUpper, s.DonchianLower, s.KeltnerUpper, s.KeltnerLower, s.VWAP} {
		consider(value)
	}
	for _, value := range s.FibonacciLevels {
		consider(value)
	}
	for _, value := range s.SupportTools {
		consider(value)
	}
	return best
}

func structureProxy(input ScannerInput) float64 {
	m := input.Snapshot.MarketStructure
	score := 0.0
	score += m["Higher High Higher Low Detection"]
	score -= m["Lower High Lower Low Detection"]
	score += m["Bullish Break of Structure"]
	score -= m["Bearish Break of Structure"]
	score += m["Change of Character"] * 0.25
	score += m["Liquidity Sweep"] * 0.2
	score += m["Fair Value Gap"] * 0.2
	score += m["Order Block"] * 0.2
	return mathutil.Clamp(score, -1, 1)
}

func statisticalProxy(name string, values []float64) float64 {
	rets := returns(values)
	switch {
	case strings.Contains(name, "standard deviation"), strings.Contains(name, "stdev"):
		return mathutil.StdDev(lastValues(values, 20))
	case strings.Contains(name, "variance"):
		std := mathutil.StdDev(lastValues(values, 20))
		return std * std
	case strings.Contains(name, "mean"):
		return mathutil.Mean(lastValues(values, 20))
	case strings.Contains(name, "z score"):
		window := lastValues(values, 20)
		return mathutil.SafeDiv(last(values)-mathutil.Mean(window), mathutil.StdDev(window))
	case strings.Contains(name, "sharpe"):
		return RollingSharpe(values, 60)
	case strings.Contains(name, "sortino"):
		return RollingSortino(values, 60)
	case strings.Contains(name, "drawdown"):
		return RollingMaxDrawdown(values, 120)
	case strings.Contains(name, "return"):
		return ROC(values, 20)
	case strings.Contains(name, "volatility"):
		return mathutil.StdDev(lastValues(rets, 20)) * math.Sqrt(252)
	default:
		return ROC(values, 20)
	}
}

func priceTransformProxy(name string, c []ohlcv.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	lastCandle := c[len(c)-1]
	switch {
	case strings.Contains(name, "average price"), strings.Contains(name, "ohlc4"):
		return (lastCandle.EffectiveOpen() + lastCandle.EffectiveHigh() + lastCandle.EffectiveLow() + lastCandle.EffectiveClose()) / 4
	case strings.Contains(name, "median"), strings.Contains(name, "hl2"):
		return (lastCandle.EffectiveHigh() + lastCandle.EffectiveLow()) / 2
	case strings.Contains(name, "typical"), strings.Contains(name, "hlc3"):
		return (lastCandle.EffectiveHigh() + lastCandle.EffectiveLow() + lastCandle.EffectiveClose()) / 3
	case strings.Contains(name, "weighted close"):
		return (lastCandle.EffectiveHigh() + lastCandle.EffectiveLow() + 2*lastCandle.EffectiveClose()) / 4
	case strings.Contains(name, "log"):
		return math.Log(math.Max(lastCandle.EffectiveClose(), mathutil.Epsilon))
	default:
		return lastCandle.EffectiveClose()
	}
}

func stopProxy(name string, input ScannerInput) float64 {
	if strings.Contains(name, "parabolic") || strings.Contains(name, "sar") {
		return ParabolicSAR(input.Candles)
	}
	if strings.Contains(name, "chandelier") {
		return input.LastClose - ATR(input.Candles, 14)*3
	}
	if strings.Contains(name, "donchian") {
		_, lower := Donchian(input.Candles, 20)
		return lower
	}
	return input.LastClose - ATR(input.Candles, 14)*2
}

// divergenceProxy reuses the real swing-based divergence detector (DetectDivergences)
// rather than comparing two arbitrary fixed-offset points, which produces false
// divergence signals on noisy data (a genuine local extremum isn't required, so any two
// points 20 bars apart moving in opposite directions from price/RSI would "count").
func divergenceProxy(input ScannerInput) float64 {
	if len(input.Candles) < 30 {
		return 0
	}
	divs := DetectDivergences(input.Candles)
	bullish := divs.RSI.Bullish || divs.MACD.Bullish
	bearish := divs.RSI.Bearish || divs.MACD.Bearish
	switch {
	case bullish && !bearish:
		return 1
	case bearish && !bullish:
		return -1
	default:
		return 0
	}
}

func chartTransformProxy(name string, c []ohlcv.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	if strings.Contains(name, "renko") || strings.Contains(name, "point and figure") || strings.Contains(name, "kagi") {
		return ZigZag(c, 0.04)
	}
	if strings.Contains(name, "heikin") {
		lastCandle := c[len(c)-1]
		return (lastCandle.EffectiveOpen() + lastCandle.EffectiveHigh() + lastCandle.EffectiveLow() + lastCandle.EffectiveClose()) / 4
	}
	return c[len(c)-1].EffectiveClose()
}

func normalizeIndicatorText(value string) string {
	return scanmatch.NormalizeText(value,
		scanmatch.Replacement{Old: "%", New: " percent "},
		scanmatch.Replacement{Old: "+", New: " plus "},
		scanmatch.Replacement{Old: "&", New: " and "},
	)
}
