// internal/patterns/candlestick.go
package patterns

import (
	"fmt"
	"math"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

type candleShape struct {
	open      float64
	high      float64
	low       float64
	close     float64
	body      float64
	rangeSize float64
	upperWick float64
	lowerWick float64
	bullish   bool
	bearish   bool
}

type candleDetector func([]ohlcv.Candle, float64) []ohlcv.PatternResult

func DetectCandlestick(candles []ohlcv.Candle, volumeSMA20 float64) ([]ohlcv.PatternResult, error) {
	if len(candles) == 0 {
		return nil, fmt.Errorf("candlestick detection requires candles: %w", ErrPatternData)
	}
	detectors := []candleDetector{
		detectDoji,
		detectDragonflyDoji,
		detectGravestoneDoji,
		detectLongLeggedDoji,
		detectSpinningTop,
		detectMarubozuBullish,
		detectMarubozuBearish,
		detectHammer,
		detectInvertedHammer,
		detectHangingMan,
		detectShootingStar,
		detectBullishEngulfing,
		detectBearishEngulfing,
		detectBullishHarami,
		detectBearishHarami,
		detectPiercingLine,
		detectDarkCloudCover,
		detectMorningStar,
		detectEveningStar,
		detectThreeWhiteSoldiers,
		detectThreeBlackCrows,
		detectTweezerBottom,
		detectTweezerTop,
		detectBullishKicker,
		detectBearishKicker,
		detectRisingThreeMethods,
		detectFallingThreeMethods,
		detectAdditionalCandlestickPatterns,
	}
	var results []ohlcv.PatternResult
	for _, detector := range detectors {
		results = append(results, detector(candles, volumeSMA20)...)
	}
	return results, nil
}

type extraCandleSpec struct {
	name       string
	direction  string
	bars       int
	confidence float64
	evidence   string
	match      func([]ohlcv.Candle) bool
}

// additionalCandleSpecs is the rule table for detectAdditionalCandlestickPatterns.
// Exposed at package level (rather than function-local) so tests can cross-check its
// directions against the auto-generated pattern catalog in internal/ta/patterns/generated.
var additionalCandleSpecs = []extraCandleSpec{
	{"Rickshaw Man", "neutral", 1, 0.70, "doji body appears near the middle with long balanced shadows", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.body <= s.rangeSize*0.05 && s.upperWick >= s.rangeSize*0.35 && s.lowerWick >= s.rangeSize*0.35
	}},
	{"High Wave Candle", "neutral", 1, 0.68, "small body with unusually long upper and lower shadows", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.body <= s.rangeSize*0.25 && s.upperWick >= s.body*2 && s.lowerWick >= s.body*2
	}},
	{"Belt Hold Bullish", "bullish", 1, 0.76, "bullish candle opens near the low and closes strong", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.bullish && s.lowerWick <= s.rangeSize*0.05 && s.body >= s.rangeSize*0.55
	}},
	{"Belt Hold Bearish", "bearish", 1, 0.76, "bearish candle opens near the high and closes weak", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.bearish && s.upperWick <= s.rangeSize*0.05 && s.body >= s.rangeSize*0.55
	}},
	{"Closing Marubozu Bullish", "bullish", 1, 0.78, "bullish close is near the candle high", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.bullish && s.upperWick <= s.rangeSize*0.03 && s.body >= s.rangeSize*0.55
	}},
	{"Closing Marubozu Bearish", "bearish", 1, 0.78, "bearish close is near the candle low", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.bearish && s.lowerWick <= s.rangeSize*0.03 && s.body >= s.rangeSize*0.55
	}},
	{"Opening Marubozu Bullish", "bullish", 1, 0.78, "bullish open is near the candle low", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.bullish && s.lowerWick <= s.rangeSize*0.03 && s.body >= s.rangeSize*0.55
	}},
	{"Opening Marubozu Bearish", "bearish", 1, 0.78, "bearish open is near the candle high", func(x []ohlcv.Candle) bool {
		s := shape(x[len(x)-1])
		return s.bearish && s.upperWick <= s.rangeSize*0.03 && s.body >= s.rangeSize*0.55
	}},
	{"Long Upper Shadow", "bearish", 1, 0.66, "upper shadow dominates the candle range", func(x []ohlcv.Candle) bool { s := shape(x[len(x)-1]); return s.upperWick >= s.rangeSize*0.55 }},
	{"Long Lower Shadow", "bullish", 1, 0.66, "lower shadow dominates the candle range", func(x []ohlcv.Candle) bool { s := shape(x[len(x)-1]); return s.lowerWick >= s.rangeSize*0.55 }},
	{"Short Line Candle", "neutral", 1, 0.60, "current candle range is small relative to recent range", func(x []ohlcv.Candle) bool { return relativeRange(x) < 0.55 }},
	{"Long Line Candle", "neutral", 1, 0.62, "current candle range is large relative to recent range", func(x []ohlcv.Candle) bool { return relativeRange(x) > 1.65 }},
	{"Matching Low", "bullish", 2, 0.72, "two bearish candles close near the same low", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bearish && near(a.close, b.close, 0.005)
	}},
	{"Matching High", "bearish", 2, 0.72, "two bullish candles close near the same high", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bullish && near(a.close, b.close, 0.005)
	}},
	{"On Neck", "bearish", 2, 0.70, "small bullish candle closes near previous bearish low", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && near(b.close, a.low, 0.01)
	}},
	{"In Neck", "bearish", 2, 0.70, "bullish reaction closes at or just above the prior bearish close, barely penetrating the body", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && b.close >= a.close && near(b.close, a.close, 0.005)
	}},
	{"Thrusting Pattern", "bearish", 2, 0.70, "bullish candle recovers well past the prior close but stays below the prior bearish midpoint", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && b.close > a.close && !near(b.close, a.close, 0.005) && b.close < (a.open+a.close)/2
	}},
	{"Separating Lines Bullish", "bullish", 2, 0.74, "opposite color candles open at nearly the same price and bullish continuation wins", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && near(a.open, b.open, 0.005) && b.close > a.open
	}},
	{"Separating Lines Bearish", "bearish", 2, 0.74, "opposite color candles open at nearly the same price and bearish continuation wins", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bearish && near(a.open, b.open, 0.005) && b.close < a.open
	}},
	{"Homing Pigeon", "bullish", 2, 0.72, "small bearish body nests inside a prior larger bearish body", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bearish && b.body < a.body && b.open < a.open && b.close > a.close
	}},
	{"Kicking Bullish", "bullish", 2, 0.82, "bearish marubozu is followed by a bullish marubozu that gaps clear of its entire range", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && b.low > a.high && b.body >= a.body*0.8
	}},
	{"Kicking Bearish", "bearish", 2, 0.82, "bullish marubozu is followed by a bearish marubozu that gaps clear of its entire range", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bearish && b.high < a.low && b.body >= a.body*0.8
	}},
	{"Counterattack Bullish", "bullish", 2, 0.74, "bearish candle is countered by bullish close near the same level", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && near(a.close, b.close, 0.006)
	}},
	{"Counterattack Bearish", "bearish", 2, 0.74, "bullish candle is countered by bearish close near the same level", func(x []ohlcv.Candle) bool {
		a, b := shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bearish && near(a.close, b.close, 0.006)
	}},
	{"Upside Gap Two Crows", "bearish", 3, 0.76, "two bearish candles appear after an upside gap", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bearish && d.bearish && b.low > a.high && d.close < b.close
	}},
	{"Downside Gap Two Rabbits", "bullish", 3, 0.76, "two bullish candles appear after a downside gap", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && d.bullish && b.high < a.low && d.close > b.close
	}},
	{"Abandoned Baby Bullish", "bullish", 3, 0.86, "doji is isolated below prior and next candle ranges", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.body <= b.rangeSize*0.05 && d.bullish && b.high < a.low && d.low > b.high
	}},
	{"Abandoned Baby Bearish", "bearish", 3, 0.86, "doji is isolated above prior and next candle ranges", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.body <= b.rangeSize*0.05 && d.bearish && b.low > a.high && d.high < b.low
	}},
	{"Tri Star Bullish", "bullish", 3, 0.78, "three doji candles form after weakness", func(x []ohlcv.Candle) bool { return threeDoji(x) && downtrend(x, len(x)-1, 6) }},
	{"Tri Star Bearish", "bearish", 3, 0.78, "three doji candles form after strength", func(x []ohlcv.Candle) bool { return threeDoji(x) && uptrend(x, len(x)-1, 6) }},
	{"Three Inside Up", "bullish", 3, 0.80, "bullish confirmation follows a bullish harami structure", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && b.open >= a.close && b.close <= a.open && d.bullish && d.close > a.open
	}},
	{"Three Inside Down", "bearish", 3, 0.80, "bearish confirmation follows a bearish harami structure", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bearish && b.open <= a.close && b.close >= a.open && d.bearish && d.close < a.open
	}},
	{"Three Outside Up", "bullish", 3, 0.82, "bullish engulfing receives a third candle confirmation", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && b.open <= a.close && b.close >= a.open && d.close > b.close
	}},
	{"Three Outside Down", "bearish", 3, 0.82, "bearish engulfing receives a third candle confirmation", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bearish && b.open >= a.close && b.close <= a.open && d.close < b.close
	}},
	{"Three Stars in the South", "bullish", 3, 0.76, "three bearish candles contract in range near lows", func(x []ohlcv.Candle) bool { return contractingThree(x, false) }},
	{"Three Advancing White Soldiers", "bullish", 3, 0.86, "three advancing bullish candles close progressively higher", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bullish && d.bullish && a.close < b.close && b.close < d.close
	}},
	{"Identical Three Crows", "bearish", 3, 0.84, "three bearish candles close progressively lower with similar opens", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bearish && d.bearish && near(a.open, b.open, 0.015) && near(b.open, d.open, 0.015) && a.close > b.close && b.close > d.close
	}},
	{"Deliberation", "bearish", 3, 0.72, "third bullish candle weakens after two strong advances", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bullish && d.bullish && d.body < b.body*0.55 && b.close > a.close && d.close > b.close
	}},
	{"Advance Block", "bearish", 3, 0.72, "bullish advance loses quality as upper shadows expand", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bullish && d.bullish && a.close < b.close && b.close < d.close && d.upperWick > b.upperWick && b.upperWick > a.upperWick
	}},
	{"Two Crows", "bearish", 3, 0.74, "two bearish candles follow an up candle near highs", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bearish && d.bearish && b.close > a.close && d.close < b.close
	}},
	{"Unique Three River Bottom", "bullish", 3, 0.74, "bearish washout is followed by a smaller low and bullish reaction", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bearish && b.low < a.low && d.bullish && d.close < b.open
	}},
	{"Upside Tasuki Gap", "bullish", 3, 0.76, "bearish third candle partially fills an upside gap without closing it", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bullish && b.low > a.high && d.bearish && d.close > a.high
	}},
	{"Downside Tasuki Gap", "bearish", 3, 0.76, "bullish third candle partially fills a downside gap without closing it", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bearish && b.high < a.low && d.bullish && d.close < a.low
	}},
	{"Side by Side White Lines Bullish", "bullish", 3, 0.74, "two similar bullish candles hold above an upside gap", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bullish && d.bullish && b.low > a.high && near(b.open, d.open, 0.015)
	}},
	{"Side by Side White Lines Bearish", "bearish", 3, 0.74, "two similar bullish candles fail under a downside gap", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && d.bullish && b.high < a.low && near(b.open, d.open, 0.015)
	}},
	{"Mat Hold Bullish", "bullish", 5, 0.80, "strong bullish candle holds through a shallow multi-bar pause and resumes", func(x []ohlcv.Candle) bool { return matHold(x, true) }},
	{"Mat Hold Bearish", "bearish", 5, 0.80, "strong bearish candle holds through a shallow multi-bar pause and resumes", func(x []ohlcv.Candle) bool { return matHold(x, false) }},
	{"Rising Window", "bullish", 2, 0.72, "current low gaps above prior high", func(x []ohlcv.Candle) bool { a, b := shape(x[len(x)-2]), shape(x[len(x)-1]); return b.low > a.high }},
	{"Falling Window", "bearish", 2, 0.72, "current high gaps below prior low", func(x []ohlcv.Candle) bool { a, b := shape(x[len(x)-2]), shape(x[len(x)-1]); return b.high < a.low }},
	{"Upside Gap Three Methods", "bullish", 3, 0.74, "third candle fills the upside gap but closes above the first candle", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bullish && b.bullish && b.low > a.high && d.bearish && d.close > a.close
	}},
	{"Downside Gap Three Methods", "bearish", 3, 0.74, "third candle fills the downside gap but closes below the first candle", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bearish && b.high < a.low && d.bullish && d.close < a.close
	}},
	{"Ladder Bottom", "bullish", 5, 0.76, "four declining bearish candles are followed by a strong bullish reversal", func(x []ohlcv.Candle) bool { return ladderBottom(x) }},
	{"Concealing Baby Swallow", "bullish", 4, 0.78, "bearish sequence compresses and is absorbed near lows", func(x []ohlcv.Candle) bool { return concealingBabySwallow(x) }},
	{"Stick Sandwich", "bullish", 3, 0.74, "two bearish closes bracket a bullish candle at matching close levels", func(x []ohlcv.Candle) bool {
		a, b, d := shape(x[len(x)-3]), shape(x[len(x)-2]), shape(x[len(x)-1])
		return a.bearish && b.bullish && d.bearish && near(a.close, d.close, 0.006)
	}},
}

func detectAdditionalCandlestickPatterns(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	var out []ohlcv.PatternResult
	for _, spec := range additionalCandleSpecs {
		if len(c) < spec.bars || !spec.match(c) {
			continue
		}
		start := len(c) - spec.bars
		end := len(c) - 1
		out = append(out, onePattern(spec.name, spec.direction, start, end, c, v, spec.confidence, spec.evidence)...)
	}
	return out
}

var ErrPatternData = fmt.Errorf("insufficient pattern data")

func detectDoji(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if s.rangeSize > 0 && s.body <= s.rangeSize*0.05 {
		return onePattern("Doji", "neutral", i, i, c, v, 0.72, "body is less than five percent of total candle range")
	}
	return nil
}

func detectDragonflyDoji(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if s.rangeSize > 0 && s.body <= s.rangeSize*0.05 && s.lowerWick >= s.rangeSize*0.6 && s.upperWick <= s.rangeSize*0.1 {
		return onePattern("Dragonfly Doji", "bullish", i, i, c, v, 0.80, "small body with long lower wick and minimal upper wick")
	}
	return nil
}

func detectGravestoneDoji(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if s.rangeSize > 0 && s.body <= s.rangeSize*0.05 && s.upperWick >= s.rangeSize*0.6 && s.lowerWick <= s.rangeSize*0.1 {
		return onePattern("Gravestone Doji", "bearish", i, i, c, v, 0.80, "small body with long upper wick and minimal lower wick")
	}
	return nil
}

func detectLongLeggedDoji(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if s.rangeSize > 0 && s.body <= s.rangeSize*0.05 && s.upperWick >= s.rangeSize*0.25 && s.lowerWick >= s.rangeSize*0.25 {
		return onePattern("Long Legged Doji", "neutral", i, i, c, v, 0.76, "small body with meaningful upper and lower wicks")
	}
	return nil
}

func detectSpinningTop(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if s.rangeSize > 0 && s.body <= s.rangeSize*0.30 && s.upperWick >= s.body*0.8 && s.lowerWick >= s.body*0.8 {
		return onePattern("Spinning Top", "neutral", i, i, c, v, 0.66, "small body with balanced upper and lower wicks")
	}
	return nil
}

func detectMarubozuBullish(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if s.bullish && s.rangeSize > 0 && s.upperWick+s.lowerWick <= s.rangeSize*0.05 {
		return onePattern("Marubozu Bullish", "bullish", i, i, c, v, 0.86, "strong bullish candle with minimal wicks")
	}
	return nil
}

func detectMarubozuBearish(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if s.bearish && s.rangeSize > 0 && s.upperWick+s.lowerWick <= s.rangeSize*0.05 {
		return onePattern("Marubozu Bearish", "bearish", i, i, c, v, 0.86, "strong bearish candle with minimal wicks")
	}
	return nil
}

func detectHammer(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if downtrend(c, i, 5) && s.lowerWick >= s.body*2 && s.upperWick <= math.Max(s.body*0.1, s.rangeSize*0.05) {
		return onePattern("Hammer", "bullish", i, i, c, v, 0.82, "long lower wick after a falling short term trend")
	}
	return nil
}

func detectInvertedHammer(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if downtrend(c, i, 5) && s.upperWick >= s.body*2 && s.lowerWick <= math.Max(s.body*0.1, s.rangeSize*0.05) {
		return onePattern("Inverted Hammer", "bullish", i, i, c, v, 0.78, "long upper wick after a falling short term trend")
	}
	return nil
}

func detectHangingMan(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if uptrend(c, i, 5) && s.lowerWick >= s.body*2 && s.upperWick <= math.Max(s.body*0.1, s.rangeSize*0.05) {
		return onePattern("Hanging Man", "bearish", i, i, c, v, 0.78, "long lower wick after a rising short term trend")
	}
	return nil
}

func detectShootingStar(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	i := len(c) - 1
	s := shape(c[i])
	if uptrend(c, i, 5) && s.upperWick >= s.body*2 && s.lowerWick <= math.Max(s.body*0.1, s.rangeSize*0.05) {
		return onePattern("Shooting Star", "bearish", i, i, c, v, 0.82, "long upper wick after a rising short term trend")
	}
	return nil
}

func detectBullishEngulfing(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev := shape(c[i-1])
	cur := shape(c[i])
	if prev.bearish && cur.bullish && cur.open <= prev.close && cur.close >= prev.open {
		return onePattern("Bullish Engulfing", "bullish", i-1, i, c, v, 0.84, "current bullish body fully engulfs previous bearish body")
	}
	return nil
}

func detectBearishEngulfing(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev := shape(c[i-1])
	cur := shape(c[i])
	if prev.bullish && cur.bearish && cur.open >= prev.close && cur.close <= prev.open {
		return onePattern("Bearish Engulfing", "bearish", i-1, i, c, v, 0.84, "current bearish body fully engulfs previous bullish body")
	}
	return nil
}

func detectBullishHarami(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev := shape(c[i-1])
	cur := shape(c[i])
	if prev.bearish && cur.bullish && prev.body > prev.rangeSize*0.45 && cur.body < prev.body*0.6 && cur.open >= prev.close && cur.close <= prev.open {
		return onePattern("Bullish Harami", "bullish", i-1, i, c, v, 0.72, "small bullish body is contained inside prior long bearish body")
	}
	return nil
}

func detectBearishHarami(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev := shape(c[i-1])
	cur := shape(c[i])
	if prev.bullish && cur.bearish && prev.body > prev.rangeSize*0.45 && cur.body < prev.body*0.6 && cur.open <= prev.close && cur.close >= prev.open {
		return onePattern("Bearish Harami", "bearish", i-1, i, c, v, 0.72, "small bearish body is contained inside prior long bullish body")
	}
	return nil
}

func detectPiercingLine(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev := shape(c[i-1])
	cur := shape(c[i])
	mid := (prev.open + prev.close) / 2
	if downtrend(c, i-1, 4) && prev.bearish && cur.bullish && cur.open < prev.close && cur.close > mid {
		return onePattern("Piercing Line", "bullish", i-1, i, c, v, 0.80, "bullish close pierces above the midpoint of the prior bearish body")
	}
	return nil
}

func detectDarkCloudCover(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev := shape(c[i-1])
	cur := shape(c[i])
	mid := (prev.open + prev.close) / 2
	if uptrend(c, i-1, 4) && prev.bullish && cur.bearish && cur.open > prev.close && cur.close < mid {
		return onePattern("Dark Cloud Cover", "bearish", i-1, i, c, v, 0.80, "bearish close cuts below the midpoint of the prior bullish body")
	}
	return nil
}

func detectMorningStar(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 3 {
		return nil
	}
	i := len(c) - 1
	a := shape(c[i-2])
	b := shape(c[i-1])
	d := shape(c[i])
	if downtrend(c, i-2, 5) && a.bearish && a.body > a.rangeSize*0.45 && b.body <= a.body*0.45 && d.bullish && d.close > (a.open+a.close)/2 {
		return onePattern("Morning Star", "bullish", i-2, i, c, v, 0.86, "three candle bullish reversal with strong close above first body midpoint")
	}
	return nil
}

func detectEveningStar(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 3 {
		return nil
	}
	i := len(c) - 1
	a := shape(c[i-2])
	b := shape(c[i-1])
	d := shape(c[i])
	if uptrend(c, i-2, 5) && a.bullish && a.body > a.rangeSize*0.45 && b.body <= a.body*0.45 && d.bearish && d.close < (a.open+a.close)/2 {
		return onePattern("Evening Star", "bearish", i-2, i, c, v, 0.86, "three candle bearish reversal with strong close below first body midpoint")
	}
	return nil
}

func detectThreeWhiteSoldiers(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 3 {
		return nil
	}
	i := len(c) - 1
	a, b, d := shape(c[i-2]), shape(c[i-1]), shape(c[i])
	if a.bullish && b.bullish && d.bullish && b.open >= a.open && b.open <= a.close && d.open >= b.open && d.open <= b.close && a.close < b.close && b.close < d.close {
		return onePattern("Three White Soldiers", "bullish", i-2, i, c, v, 0.88, "three consecutive strong bullish candles with rising closes")
	}
	return nil
}

func detectThreeBlackCrows(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 3 {
		return nil
	}
	i := len(c) - 1
	a, b, d := shape(c[i-2]), shape(c[i-1]), shape(c[i])
	if a.bearish && b.bearish && d.bearish && b.open <= a.open && b.open >= a.close && d.open <= b.open && d.open >= b.close && a.close > b.close && b.close > d.close {
		return onePattern("Three Black Crows", "bearish", i-2, i, c, v, 0.88, "three consecutive strong bearish candles with falling closes")
	}
	return nil
}

func detectTweezerBottom(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	tol := math.Min(c[i].EffectiveLow(), c[i-1].EffectiveLow()) * 0.005
	if downtrend(c, i-1, 5) && mathutil.AlmostEqualTol(c[i].EffectiveLow(), c[i-1].EffectiveLow(), tol) {
		return onePattern("Tweezer Bottom", "bullish", i-1, i, c, v, 0.76, "two adjacent lows are aligned within half percent tolerance")
	}
	return nil
}

func detectTweezerTop(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	tol := math.Min(c[i].EffectiveHigh(), c[i-1].EffectiveHigh()) * 0.005
	if uptrend(c, i-1, 5) && mathutil.AlmostEqualTol(c[i].EffectiveHigh(), c[i-1].EffectiveHigh(), tol) {
		return onePattern("Tweezer Top", "bearish", i-1, i, c, v, 0.76, "two adjacent highs are aligned within half percent tolerance")
	}
	return nil
}

func detectBullishKicker(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev, cur := shape(c[i-1]), shape(c[i])
	if prev.bearish && cur.bullish && cur.open > prev.open && cur.body > prev.body*0.8 {
		return onePattern("Bullish Kicker", "bullish", i-1, i, c, v, 0.82, "strong bullish reversal opens above prior bearish body")
	}
	return nil
}

func detectBearishKicker(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 2 {
		return nil
	}
	i := len(c) - 1
	prev, cur := shape(c[i-1]), shape(c[i])
	if prev.bullish && cur.bearish && cur.open < prev.open && cur.body > prev.body*0.8 {
		return onePattern("Bearish Kicker", "bearish", i-1, i, c, v, 0.82, "strong bearish reversal opens below prior bullish body")
	}
	return nil
}

func detectRisingThreeMethods(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 5 {
		return nil
	}
	i := len(c) - 1
	first := shape(c[i-4])
	last := shape(c[i])
	if !first.bullish || !last.bullish || last.close <= first.close {
		return nil
	}
	for j := i - 3; j <= i-1; j++ {
		s := shape(c[j])
		if s.close < first.open || s.close > first.close || s.body > first.body*0.7 {
			return nil
		}
	}
	return onePattern("Rising Three Methods", "bullish", i-4, i, c, v, 0.84, "bullish continuation with three contained corrective candles")
}

func detectFallingThreeMethods(c []ohlcv.Candle, v float64) []ohlcv.PatternResult {
	if len(c) < 5 {
		return nil
	}
	i := len(c) - 1
	first := shape(c[i-4])
	last := shape(c[i])
	if !first.bearish || !last.bearish || last.close >= first.close {
		return nil
	}
	for j := i - 3; j <= i-1; j++ {
		s := shape(c[j])
		if s.close > first.open || s.close < first.close || s.body > first.body*0.7 {
			return nil
		}
	}
	return onePattern("Falling Three Methods", "bearish", i-4, i, c, v, 0.84, "bearish continuation with three contained corrective candles")
}

func onePattern(name, direction string, start, end int, candles []ohlcv.Candle, volumeSMA20, baseConfidence float64, evidence string) []ohlcv.PatternResult {
	confirmed := volumeConfirmed(candles[end], volumeSMA20)
	confidence := baseConfidence
	resultEvidence := []string{evidence}
	if confirmed {
		confidence = mathutil.Clamp(confidence+0.06, 0, 1)
		resultEvidence = append(resultEvidence, volumeEvidence(true))
	} else if volumeSMA20 > 0 {
		resultEvidence = append(resultEvidence, volumeEvidence(false))
	}
	result := ohlcv.PatternResult{
		Name:            name,
		Category:        "candlestick",
		Direction:       direction,
		Confidence:      confidence,
		StartIndex:      start,
		EndIndex:        end,
		StartTime:       candles[start].Time,
		EndTime:         candles[end].Time,
		Evidence:        resultEvidence,
		VolumeConfirmed: confirmed,
	}
	return []ohlcv.PatternResult{enrichPatternBacktestMetadata(result, candles)}
}

func shape(c ohlcv.Candle) candleShape {
	open := c.EffectiveOpen()
	closePrice := c.EffectiveClose()
	high := c.EffectiveHigh()
	low := c.EffectiveLow()
	body := math.Abs(closePrice - open)
	upper := high - math.Max(open, closePrice)
	lower := math.Min(open, closePrice) - low
	return candleShape{
		open:      open,
		high:      high,
		low:       low,
		close:     closePrice,
		body:      body,
		rangeSize: math.Max(high-low, mathutil.Epsilon),
		upperWick: math.Max(upper, 0),
		lowerWick: math.Max(lower, 0),
		bullish:   closePrice > open,
		bearish:   closePrice < open,
	}
}

func volumeConfirmed(candle ohlcv.Candle, volumeSMA20 float64) bool {
	if volumeSMA20 <= 0 {
		return false
	}
	return candle.EffectiveVolume() >= volumeSMA20*1.20
}

func volumeEvidence(confirmed bool) string {
	if confirmed {
		return "completion bar volume is at least twenty percent above Volume SMA20"
	}
	return "completion bar volume is below the twenty percent premium over Volume SMA20"
}

func uptrend(candles []ohlcv.Candle, end, period int) bool {
	start := end - period
	if start < 0 {
		return false
	}
	return candles[end].EffectiveClose() > candles[start].EffectiveClose()
}

func downtrend(candles []ohlcv.Candle, end, period int) bool {
	start := end - period
	if start < 0 {
		return false
	}
	return candles[end].EffectiveClose() < candles[start].EffectiveClose()
}

func near(a, b, pct float64) bool {
	base := math.Max(math.Min(math.Abs(a), math.Abs(b)), mathutil.Epsilon)
	return math.Abs(a-b) <= base*pct
}

func relativeRange(c []ohlcv.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	start := len(c) - 20
	if start < 0 {
		start = 0
	}
	total := 0.0
	for _, candle := range c[start:] {
		total += candle.EffectiveHigh() - candle.EffectiveLow()
	}
	avg := mathutil.SafeDiv(total, float64(len(c[start:])))
	current := c[len(c)-1].EffectiveHigh() - c[len(c)-1].EffectiveLow()
	return mathutil.SafeDiv(current, avg)
}

func threeDoji(c []ohlcv.Candle) bool {
	if len(c) < 3 {
		return false
	}
	for i := len(c) - 3; i < len(c); i++ {
		s := shape(c[i])
		if s.body > s.rangeSize*0.08 {
			return false
		}
	}
	return true
}

func contractingThree(c []ohlcv.Candle, bullish bool) bool {
	if len(c) < 3 {
		return false
	}
	a, b, d := shape(c[len(c)-3]), shape(c[len(c)-2]), shape(c[len(c)-1])
	if bullish {
		return a.bullish && b.bullish && d.bullish && a.rangeSize > b.rangeSize && b.rangeSize > d.rangeSize
	}
	return a.bearish && b.bearish && d.bearish && a.rangeSize > b.rangeSize && b.rangeSize > d.rangeSize
}

func matHold(c []ohlcv.Candle, bullish bool) bool {
	if len(c) < 5 {
		return false
	}
	a := shape(c[len(c)-5])
	e := shape(c[len(c)-1])
	if bullish {
		if !a.bullish || !e.bullish || e.close <= a.close {
			return false
		}
		for i := len(c) - 4; i <= len(c)-2; i++ {
			s := shape(c[i])
			if s.low < a.open || s.high > e.close {
				return false
			}
		}
		return true
	}
	if !a.bearish || !e.bearish || e.close >= a.close {
		return false
	}
	for i := len(c) - 4; i <= len(c)-2; i++ {
		s := shape(c[i])
		if s.high > a.open || s.low < e.close {
			return false
		}
	}
	return true
}

func ladderBottom(c []ohlcv.Candle) bool {
	if len(c) < 5 {
		return false
	}
	for i := len(c) - 5; i <= len(c)-2; i++ {
		if !shape(c[i]).bearish {
			return false
		}
		if i > len(c)-5 && c[i].EffectiveClose() >= c[i-1].EffectiveClose() {
			return false
		}
	}
	return shape(c[len(c)-1]).bullish && c[len(c)-1].EffectiveClose() > c[len(c)-2].EffectiveOpen()
}

func concealingBabySwallow(c []ohlcv.Candle) bool {
	if len(c) < 4 {
		return false
	}
	a, b, d, e := shape(c[len(c)-4]), shape(c[len(c)-3]), shape(c[len(c)-2]), shape(c[len(c)-1])
	return a.bearish && b.bearish && d.bearish && e.bearish && b.high < a.high && d.high > b.high && e.close < d.close && e.body < d.body
}
