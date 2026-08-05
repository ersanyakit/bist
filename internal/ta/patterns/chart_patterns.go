// internal/patterns/chart_patterns.go
package patterns

import (
	"fmt"
	"math"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

type swing struct {
	index int
	price float64
	kind  string
}

type chartSpec struct {
	name       string
	direction  string
	confidence float64
	evidence   string
	match      func([]ohlcv.Candle, []swing) (bool, int, int)
}

func DetectChartPatterns(candles []ohlcv.Candle, volumeSMA20 float64) ([]ohlcv.PatternResult, error) {
	if len(candles) < 20 {
		return nil, fmt.Errorf("chart pattern detection requires at least twenty candles: %w", ErrPatternData)
	}
	swings := detectChartSwings(candles, 3)
	specs := []chartSpec{
		{"Double Bottom", "bullish", 0.82, "two similar swing lows with a separating rally", matchDouble("low", 2, 0.015, 10, 50, true)},
		{"Double Top", "bearish", 0.82, "two similar swing highs with a separating pullback", matchDouble("high", 2, 0.015, 10, 50, false)},
		{"Triple Bottom", "bullish", 0.84, "three swing lows cluster near the same price", matchTriple("low", 0.015)},
		{"Triple Top", "bearish", 0.84, "three swing highs cluster near the same price", matchTriple("high", 0.015)},
		{"Head and Shoulders", "bearish", 0.84, "middle swing high is above two similar shoulders", matchHeadShoulders(false)},
		{"Inverse Head and Shoulders", "bullish", 0.84, "middle swing low is below two similar shoulders", matchHeadShoulders(true)},
		{"Ascending Triangle", "bullish", 0.78, "resistance is flat while swing lows rise", matchTriangle("ascending")},
		{"Descending Triangle", "bearish", 0.78, "support is flat while swing highs fall", matchTriangle("descending")},
		{"Symmetrical Triangle", "neutral", 0.74, "swing highs fall while swing lows rise", matchTriangle("symmetrical")},
		{"Bullish Flag", "bullish", 0.76, "sharp rally is followed by a tight descending consolidation", matchFlag(true)},
		{"Bearish Flag", "bearish", 0.76, "sharp selloff is followed by a tight ascending consolidation", matchFlag(false)},
		{"Bullish Pennant", "bullish", 0.76, "sharp rally is followed by a compact contracting range", matchPennant(true)},
		{"Bearish Pennant", "bearish", 0.76, "sharp selloff is followed by a compact contracting range", matchPennant(false)},
		{"Rising Wedge", "bearish", 0.74, "rising range narrows into compression", matchWedge(true)},
		{"Falling Wedge", "bullish", 0.74, "falling range narrows into compression", matchWedge(false)},
		{"Cup and Handle", "bullish", 0.75, "rounded recovery is followed by a shallow handle", matchCup(false)},
		{"Inverse Cup and Handle", "bearish", 0.75, "rounded top is followed by a shallow upward handle", matchCup(true)},
		{"Rectangle Range", "neutral", 0.72, "price rotates between horizontal support and resistance", matchRectangle},
		{"Horizontal Channel", "neutral", 0.72, "range slope is flat with repeated boundary touches", matchChannel("horizontal")},
		{"Ascending Channel", "bullish", 0.74, "support and resistance slopes are both rising", matchChannel("ascending")},
		{"Descending Channel", "bearish", 0.74, "support and resistance slopes are both falling", matchChannel("descending")},
		{"Rounding Bottom", "bullish", 0.72, "price transitions from falling to rising in a U shape", matchRounding(false)},
		{"Rounding Top", "bearish", 0.72, "price transitions from rising to falling in an inverted U shape", matchRounding(true)},
		{"Broadening Formation", "neutral", 0.70, "successive swings expand with higher highs and lower lows", matchBroadening},
		{"Diamond Top", "bearish", 0.72, "range expands and then contracts near the upper part of recent prices", matchDiamond(true)},
		{"Diamond Bottom", "bullish", 0.72, "range expands and then contracts near the lower part of recent prices", matchDiamond(false)},
		{"ABCD Pattern", "neutral", 0.70, "four swing legs show comparable AB and CD movement", matchABCD},
		{"Gartley Pattern", "bullish", 0.70, "harmonic retracement cluster resembles Gartley proportions", matchHarmonic("gartley")},
		{"Bat Pattern", "bullish", 0.70, "harmonic retracement cluster resembles Bat proportions", matchHarmonic("bat")},
		{"Butterfly Pattern", "bearish", 0.70, "harmonic extension cluster resembles Butterfly proportions", matchHarmonic("butterfly")},
		{"Crab Pattern", "bearish", 0.70, "deep harmonic extension resembles Crab proportions", matchHarmonic("crab")},
		{"Deep Crab Pattern", "bearish", 0.70, "deep retracement and extension resemble Deep Crab proportions", matchHarmonic("deep_crab")},
		{"Shark Pattern", "neutral", 0.68, "volatile harmonic five point sequence resembles Shark proportions", matchHarmonic("shark")},
		{"Cypher Pattern", "bullish", 0.68, "harmonic five point sequence resembles Cypher proportions", matchHarmonic("cypher")},
		{"Three Drives Pattern", "neutral", 0.68, "three similar directional drives appear in the swing sequence", matchThreeDrives},
		{"5-0 Pattern", "neutral", 0.68, "five point swing reversal sequence has a mid retracement", matchFiveZero},
		{"Impulse Wave 1-2-3-4-5", "neutral", 0.70, "five alternating swings form an impulse-like sequence", matchElliottImpulse},
		{"Corrective ABC", "neutral", 0.68, "three alternating swings form an ABC correction", matchABC},
		{"Zigzag Correction", "neutral", 0.68, "ABC correction has a strong middle leg", matchABC},
		{"Flat Correction", "neutral", 0.66, "ABC correction stays within a flat range", matchFlatCorrection},
		{"Expanded Flat", "neutral", 0.66, "flat correction slightly exceeds the prior swing extreme", matchExpandedFlat},
		{"Running Flat", "neutral", 0.66, "flat correction fails to fully retrace the preceding move", matchRunningFlat},
		{"Triangle Correction", "neutral", 0.66, "corrective swings contract into a triangle", matchTriangle("symmetrical")},
		{"Leading Diagonal", "neutral", 0.66, "early five swing sequence overlaps while advancing", matchDiagonal},
		{"Ending Diagonal", "neutral", 0.66, "late five swing sequence contracts while advancing", matchDiagonal},
		{"Accumulation Schematic", "bullish", 0.70, "range shows selling climax, spring risk and improving close location", matchWyckoff(true)},
		{"Distribution Schematic", "bearish", 0.70, "range shows buying climax, upthrust risk and weakening close location", matchWyckoff(false)},
		{"Spring", "bullish", 0.72, "price sweeps below range support and closes back inside", matchSpring},
		{"Upthrust", "bearish", 0.72, "price sweeps above range resistance and closes back inside", matchUpthrust},
		{"Sign of Strength", "bullish", 0.70, "close breaks above recent resistance with strong range expansion", matchSOS},
		{"Sign of Weakness", "bearish", 0.70, "close breaks below recent support with strong range expansion", matchSOW},
		{"Last Point of Support", "bullish", 0.68, "pullback holds above prior support after strength", matchLPS},
		{"Last Point of Supply", "bearish", 0.68, "bounce holds below prior resistance after weakness", matchLPSY},
		{"Buying Climax", "bearish", 0.68, "large up bar occurs near recent high with expanded volume", matchClimax(true)},
		{"Selling Climax", "bullish", 0.68, "large down bar occurs near recent low with expanded volume", matchClimax(false)},
		{"Automatic Rally", "bullish", 0.66, "strong rebound follows a selling climax zone", matchAutomatic(true)},
		{"Automatic Reaction", "bearish", 0.66, "strong pullback follows a buying climax zone", matchAutomatic(false)},
		{"Bullish Break of Structure", "bullish", 0.72, "close exceeds recent swing high", matchBOS(true)},
		{"Bearish Break of Structure", "bearish", 0.72, "close loses recent swing low", matchBOS(false)},
		{"Bullish Change of Character", "bullish", 0.70, "bearish sequence is interrupted by a bullish structure break", matchCHOCH(true)},
		{"Bearish Change of Character", "bearish", 0.70, "bullish sequence is interrupted by a bearish structure break", matchCHOCH(false)},
		{"Liquidity Grab", "neutral", 0.68, "price sweeps a recent high or low and rejects", matchLiquidityGrab},
		{"Stop Hunt", "neutral", 0.68, "wick pierces recent liquidity and closes back inside range", matchLiquidityGrab},
		{"Fair Value Gap", "neutral", 0.66, "three candle sequence leaves an imbalance gap", matchFairValueGap},
		{"Bullish Order Block", "bullish", 0.68, "last bearish candle before a bullish displacement is visible", matchOrderBlock(true)},
		{"Bearish Order Block", "bearish", 0.68, "last bullish candle before a bearish displacement is visible", matchOrderBlock(false)},
		{"Breaker Block", "neutral", 0.66, "failed order block is followed by an opposite structure break", matchBreaker},
		{"Mitigation Block", "neutral", 0.66, "price revisits a prior displacement candle zone", matchMitigation},
		{"Equal Highs Liquidity", "bearish", 0.66, "recent swing highs align within tight tolerance", matchEqualLiquidity(true)},
		{"Equal Lows Liquidity", "bullish", 0.66, "recent swing lows align within tight tolerance", matchEqualLiquidity(false)},
		{"Complex Head and Shoulders", "bearish", 0.70, "multiple shoulders cluster around a dominant head", matchComplexHS(false)},
		{"Complex Inverse Head and Shoulders", "bullish", 0.70, "multiple inverse shoulders cluster around a dominant head", matchComplexHS(true)},
		{"Measured Move Up", "bullish", 0.68, "two bullish legs show comparable distance", matchMeasuredMove(true)},
		{"Measured Move Down", "bearish", 0.68, "two bearish legs show comparable distance", matchMeasuredMove(false)},
		{"Bump and Run Reversal Top", "bearish", 0.68, "accelerating rise reverses below a recent trendline", matchBumpRun(true)},
		{"Bump and Run Reversal Bottom", "bullish", 0.68, "accelerating fall reverses above a recent trendline", matchBumpRun(false)},
		{"Island Reversal Top", "bearish", 0.68, "upside gap island is followed by a downside gap", matchIsland(true)},
		{"Island Reversal Bottom", "bullish", 0.68, "downside gap island is followed by an upside gap", matchIsland(false)},
		{"Pipe Top", "bearish", 0.66, "two adjacent wide candles form a top reversal", matchPipe(true)},
		{"Pipe Bottom", "bullish", 0.66, "two adjacent wide candles form a bottom reversal", matchPipe(false)},
		{"V Bottom", "bullish", 0.68, "sharp decline is quickly reversed by a sharp rise", matchV(false)},
		{"V Top", "bearish", 0.68, "sharp rise is quickly reversed by a sharp decline", matchV(true)},
		{"Adam and Eve Double Bottom", "bullish", 0.68, "sharp first low and rounded second low align", matchAdamEve(false, true)},
		{"Adam and Eve Double Top", "bearish", 0.68, "sharp first high and rounded second high align", matchAdamEve(true, true)},
		{"Eve and Eve Double Bottom", "bullish", 0.66, "two rounded lows align", matchAdamEve(false, false)},
		{"Adam and Adam Double Top", "bearish", 0.66, "two sharp highs align", matchAdamAdamTop},
	}
	var out []ohlcv.PatternResult
	for _, spec := range specs {
		ok, start, end := spec.match(candles, swings)
		if !ok {
			continue
		}
		if start < 0 {
			start = 0
		}
		if end < start || end >= len(candles) {
			end = len(candles) - 1
		}
		out = append(out, chartPattern(spec, start, end, candles, volumeSMA20))
	}
	return out, nil
}

func chartPattern(spec chartSpec, start, end int, candles []ohlcv.Candle, volumeSMA20 float64) ohlcv.PatternResult {
	confirmed := volumeConfirmed(candles[end], volumeSMA20)
	confidence := spec.confidence
	evidence := []string{spec.evidence}
	if confirmed {
		confidence = mathutil.Clamp(confidence+0.06, 0, 1)
		evidence = append(evidence, volumeEvidence(true))
	} else if volumeSMA20 > 0 {
		evidence = append(evidence, volumeEvidence(false))
	}
	result := ohlcv.PatternResult{
		Name:            spec.name,
		Category:        "chart",
		Direction:       spec.direction,
		Confidence:      confidence,
		StartIndex:      start,
		EndIndex:        end,
		StartTime:       candles[start].Time,
		EndTime:         candles[end].Time,
		Evidence:        evidence,
		VolumeConfirmed: confirmed,
	}
	return enrichPatternBacktestMetadata(result, candles)
}

func detectChartSwings(c []ohlcv.Candle, wing int) []swing {
	var out []swing
	for i := wing; i < len(c)-wing; i++ {
		isHigh, isLow := true, true
		for j := i - wing; j <= i+wing; j++ {
			if j == i {
				continue
			}
			if c[j].EffectiveHigh() >= c[i].EffectiveHigh() {
				isHigh = false
			}
			if c[j].EffectiveLow() <= c[i].EffectiveLow() {
				isLow = false
			}
		}
		if isHigh {
			out = append(out, swing{index: i, price: c[i].EffectiveHigh(), kind: "high"})
		}
		if isLow {
			out = append(out, swing{index: i, price: c[i].EffectiveLow(), kind: "low"})
		}
	}
	return out
}

func matchDouble(kind string, count int, tolerance float64, minGap int, maxGap int, bullish bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		points := lastSwings(s, kind, 6)
		for i := 0; i < len(points); i++ {
			for j := i + 1; j < len(points); j++ {
				gap := points[j].index - points[i].index
				if gap >= minGap && gap <= maxGap && near(points[i].price, points[j].price, tolerance) {
					if bullish && trend(c, points[i].index, 30) < 0 && necklineBreak(c, s, points[i], points[j], true) {
						return true, points[i].index, len(c) - 1
					}
					if !bullish && trend(c, points[i].index, 30) > 0 && necklineBreak(c, s, points[i], points[j], false) {
						return true, points[i].index, len(c) - 1
					}
				}
			}
		}
		_ = count
		return false, 0, 0
	}
}

func matchTriple(kind string, tolerance float64) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		points := lastSwings(s, kind, 5)
		if len(points) < 3 {
			return false, 0, 0
		}
		a, b, d := points[len(points)-3], points[len(points)-2], points[len(points)-1]
		bullish := kind == "low"
		if near(a.price, b.price, tolerance) && near(b.price, d.price, tolerance) && necklineBreak(c, s, a, d, bullish) {
			return true, a.index, len(c) - 1
		}
		return false, 0, 0
	}
}

func matchHeadShoulders(inverse bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		p := lastSwings(s, "", 7)
		if len(p) < 5 {
			return false, 0, 0
		}
		for start := len(p) - 5; start >= 0; start-- {
			seq := p[start : start+5]
			if inverse {
				if !swingKinds(seq, "low", "high", "low", "high", "low") {
					continue
				}
				leftShoulder, head, rightShoulder := seq[0], seq[2], seq[4]
				neckline := (seq[1].price + seq[3].price) / 2
				shouldersAligned := near(leftShoulder.price, rightShoulder.price, 0.035)
				headDominant := head.price < leftShoulder.price && head.price < rightShoulder.price
				confirmed := len(c) > rightShoulder.index+1 && c[len(c)-1].EffectiveClose() > neckline
				if shouldersAligned && headDominant && confirmed {
					return true, seq[0].index, len(c) - 1
				}
				continue
			}
			if !swingKinds(seq, "high", "low", "high", "low", "high") {
				continue
			}
			leftShoulder, head, rightShoulder := seq[0], seq[2], seq[4]
			neckline := (seq[1].price + seq[3].price) / 2
			shouldersAligned := near(leftShoulder.price, rightShoulder.price, 0.035)
			headDominant := head.price > leftShoulder.price && head.price > rightShoulder.price
			confirmed := len(c) > rightShoulder.index+1 && c[len(c)-1].EffectiveClose() < neckline
			if shouldersAligned && headDominant && confirmed {
				return true, seq[0].index, len(c) - 1
			}
		}
		return false, 0, 0
	}
}

func matchTriangle(kind string) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		hs := lastSwings(s, "high", 3)
		ls := lastSwings(s, "low", 3)
		if len(hs) < 2 || len(ls) < 2 {
			return false, 0, 0
		}
		highSlope := hs[len(hs)-1].price - hs[0].price
		lowSlope := ls[len(ls)-1].price - ls[0].price
		flatHigh := near(hs[len(hs)-1].price, hs[0].price, 0.015)
		flatLow := near(ls[len(ls)-1].price, ls[0].price, 0.015)
		contracting := rangeContracted(c, 15, 45)
		resistance := maxSwingPrice(hs)
		support := minSwingPrice(ls)
		lastClose := c[len(c)-1].EffectiveClose()
		switch kind {
		case "ascending":
			return flatHigh && lowSlope > 0 && contracting && lastClose > resistance, minInt(hs[0].index, ls[0].index), len(c) - 1
		case "descending":
			return flatLow && highSlope < 0 && contracting && lastClose < support, minInt(hs[0].index, ls[0].index), len(c) - 1
		default:
			return highSlope < 0 && lowSlope > 0 && contracting, minInt(hs[0].index, ls[0].index), len(c) - 1
		}
	}
}

func matchFlag(bullish bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 35 {
			return false, 0, 0
		}
		poleStart := len(c) - 35
		poleEnd := len(c) - 15
		pole := priceChangeRatio(c[poleEnd].EffectiveClose(), c[poleStart].EffectiveClose())
		consolidation := c[poleEnd : len(c)-1]
		slope := trend(c, len(c)-2, len(c)-poleEnd-1)
		rangeOK := rangeWidth(consolidation) < math.Abs(pole)*0.75
		volumeOK := averageVolume(consolidation) <= averageVolume(c[poleStart:poleEnd])*1.10 || averageVolume(consolidation) == 0
		lastClose := c[len(c)-1].EffectiveClose()
		if bullish {
			return pole > 0.08 && slope <= 0.02 && rangeOK && volumeOK && lastClose > highest(consolidation), poleStart, len(c) - 1
		}
		return pole < -0.08 && slope >= -0.02 && rangeOK && volumeOK && lastClose < lowest(consolidation), poleStart, len(c) - 1
	}
}

func matchPennant(bullish bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		ok, start, end := matchFlag(bullish)(c, s)
		if !ok {
			return false, 0, 0
		}
		return len(c) >= 35 && rangeWidth(c[len(c)-8:]) < rangeWidth(c[len(c)-20:len(c)-12]), start, end
	}
}

func matchWedge(rising bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		hs := lastSwings(s, "high", 3)
		ls := lastSwings(s, "low", 3)
		if len(hs) < 2 || len(ls) < 2 {
			return false, 0, 0
		}
		highSlope := hs[len(hs)-1].price - hs[0].price
		lowSlope := ls[len(ls)-1].price - ls[0].price
		compressing := rangeWidth(c[maxInt(0, len(c)-20):]) < rangeWidth(c[maxInt(0, len(c)-60):maxInt(1, len(c)-40)])
		if rising {
			return highSlope > 0 && lowSlope > 0 && lowSlope > highSlope && compressing, minInt(hs[0].index, ls[0].index), len(c) - 1
		}
		return highSlope < 0 && lowSlope < 0 && highSlope < lowSlope && compressing, minInt(hs[0].index, ls[0].index), len(c) - 1
	}
}

func matchCup(inverse bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 60 {
			return false, 0, 0
		}
		start := maxInt(0, len(c)-80)
		w := c[start:]
		leftRim := w[0].EffectiveClose()
		rightRim := w[len(w)-12].EffectiveClose()
		lastClose := w[len(w)-1].EffectiveClose()
		handle := w[len(w)-12:]
		if inverse {
			peak := highest(w[:len(w)-12])
			rim := math.Min(leftRim, rightRim)
			height := mathutil.SafeDiv(peak-rim, priceDenominator(rim))
			handleRise := mathutil.SafeDiv(highest(handle)-rightRim, priceDenominator(rightRim))
			return near(leftRim, rightRim, 0.08) && between(height, 0.08, 0.55) && handleRise <= 0.18 && lastClose < rightRim*0.99, start, len(c) - 1
		}
		bottom := lowest(w[:len(w)-12])
		rim := math.Min(leftRim, rightRim)
		depth := mathutil.SafeDiv(rim-bottom, priceDenominator(rim))
		handleDrop := mathutil.SafeDiv(rightRim-lowest(handle), priceDenominator(rightRim))
		return near(leftRim, rightRim, 0.08) && between(depth, 0.08, 0.55) && handleDrop <= 0.18 && lastClose > rightRim*1.01, start, len(c) - 1
	}
}

func matchRectangle(c []ohlcv.Candle, s []swing) (bool, int, int) {
	hs := lastSwings(s, "high", 4)
	ls := lastSwings(s, "low", 4)
	if len(hs) < 2 || len(ls) < 2 {
		return false, 0, 0
	}
	return near(hs[0].price, hs[len(hs)-1].price, 0.02) && near(ls[0].price, ls[len(ls)-1].price, 0.02), minInt(hs[0].index, ls[0].index), len(c) - 1
}

func matchChannel(kind string) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		slope := trend(c, len(c)-1, 40)
		switch kind {
		case "ascending":
			return slope > 0.02, maxInt(0, len(c)-40), len(c) - 1
		case "descending":
			return slope < -0.02, maxInt(0, len(c)-40), len(c) - 1
		default:
			return math.Abs(slope) <= 0.02 && rangeWidth(c[maxInt(0, len(c)-40):]) > 0, maxInt(0, len(c)-40), len(c) - 1
		}
	}
}

func matchRounding(top bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 50 {
			return false, 0, 0
		}
		a, b, d := c[len(c)-50].EffectiveClose(), c[len(c)-25].EffectiveClose(), c[len(c)-1].EffectiveClose()
		if top {
			return b > a && b > d && math.Abs(a-d)/priceDenominator(a) < 0.08, len(c) - 50, len(c) - 1
		}
		return b < a && b < d && math.Abs(a-d)/priceDenominator(a) < 0.08, len(c) - 50, len(c) - 1
	}
}

func matchBroadening(c []ohlcv.Candle, s []swing) (bool, int, int) {
	hs := lastSwings(s, "high", 3)
	ls := lastSwings(s, "low", 3)
	if len(hs) < 2 || len(ls) < 2 {
		return false, 0, 0
	}
	return hs[len(hs)-1].price > hs[0].price && ls[len(ls)-1].price < ls[0].price, minInt(hs[0].index, ls[0].index), len(c) - 1
}

func matchDiamond(top bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 50 {
			return false, 0, 0
		}
		first := rangeWidth(c[len(c)-50 : len(c)-35])
		mid := rangeWidth(c[len(c)-35 : len(c)-15])
		lastR := rangeWidth(c[len(c)-15:])
		position := mathutil.SafeDiv(c[len(c)-1].EffectiveClose()-lowest(c), highest(c)-lowest(c))
		if top {
			return mid > first && mid > lastR && position > 0.55, len(c) - 50, len(c) - 1
		}
		return mid > first && mid > lastR && position < 0.45, len(c) - 50, len(c) - 1
	}
}

func matchABCD(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 4)
	if len(p) < 4 {
		return false, 0, 0
	}
	ab := math.Abs(p[1].price - p[0].price)
	cd := math.Abs(p[3].price - p[2].price)
	return near(ab, cd, 0.20), p[0].index, len(c) - 1
}

func matchHarmonic(kind string) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		p := lastSwings(s, "", 5)
		if len(p) < 5 || !alternatingSwings(p) {
			return false, 0, 0
		}
		xa := math.Abs(p[1].price - p[0].price)
		ab := math.Abs(p[2].price - p[1].price)
		bc := math.Abs(p[3].price - p[2].price)
		cd := math.Abs(p[4].price - p[3].price)
		ad := math.Abs(p[4].price - p[0].price)
		abXA := mathutil.SafeDiv(ab, xa)
		bcAB := mathutil.SafeDiv(bc, ab)
		cdBC := mathutil.SafeDiv(cd, bc)
		adXA := mathutil.SafeDiv(ad, xa)
		switch kind {
		case "gartley":
			return between(abXA, 0.55, 0.70) && between(bcAB, 0.38, 0.89) && between(cdBC, 1.13, 1.70) && between(adXA, 0.70, 0.88), p[0].index, len(c) - 1
		case "bat":
			return between(abXA, 0.35, 0.55) && between(bcAB, 0.38, 0.89) && between(cdBC, 1.50, 2.80) && between(adXA, 0.80, 0.95), p[0].index, len(c) - 1
		case "butterfly":
			return between(abXA, 0.70, 0.85) && between(bcAB, 0.38, 0.89) && between(cdBC, 1.27, 2.70) && between(adXA, 1.20, 1.75), p[0].index, len(c) - 1
		case "crab":
			return between(abXA, 0.35, 0.65) && between(bcAB, 0.38, 0.89) && between(cdBC, 2.20, 3.80) && between(adXA, 1.45, 1.80), p[0].index, len(c) - 1
		case "deep_crab":
			return between(abXA, 0.80, 0.95) && between(bcAB, 0.38, 0.89) && between(cdBC, 2.00, 3.80) && between(adXA, 1.45, 1.80), p[0].index, len(c) - 1
		case "shark":
			return between(abXA, 0.80, 1.20) && between(bcAB, 1.10, 1.90) && between(adXA, 0.85, 1.20), p[0].index, len(c) - 1
		case "cypher":
			return between(abXA, 0.35, 0.70) && between(bcAB, 1.20, 1.55) && between(adXA, 0.70, 0.90), p[0].index, len(c) - 1
		default:
			return between(abXA, 0.35, 0.90) && between(bcAB, 0.35, 1.90) && between(cdBC, 1.10, 3.80), p[0].index, len(c) - 1
		}
	}
}

func matchThreeDrives(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 6)
	if len(p) < 6 {
		return false, 0, 0
	}
	d1 := math.Abs(p[1].price - p[0].price)
	d2 := math.Abs(p[3].price - p[2].price)
	d3 := math.Abs(p[5].price - p[4].price)
	return near(d1, d2, 0.25) && near(d2, d3, 0.25), p[0].index, len(c) - 1
}

func matchFiveZero(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 5)
	if len(p) < 5 {
		return false, 0, 0
	}
	midRet := mathutil.SafeDiv(math.Abs(p[3].price-p[2].price), math.Abs(p[2].price-p[1].price))
	return between(midRet, 0.45, 0.65), p[0].index, len(c) - 1
}

func matchElliottImpulse(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 6)
	if len(p) < 6 || !alternatingSwings(p) {
		return false, 0, 0
	}
	return elliottImpulseOK(p, true) || elliottImpulseOK(p, false), p[0].index, len(c) - 1
}

func matchABC(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 3)
	if len(p) < 3 {
		return false, 0, 0
	}
	return p[0].kind != p[1].kind && p[1].kind != p[2].kind, p[0].index, len(c) - 1
}

func matchFlatCorrection(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 3)
	if len(p) < 3 {
		return false, 0, 0
	}
	return near(p[0].price, p[2].price, 0.05), p[0].index, len(c) - 1
}

func matchExpandedFlat(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 3)
	if len(p) < 3 {
		return false, 0, 0
	}
	return math.Abs(p[2].price-p[0].price) > math.Abs(p[1].price-p[0].price)*0.15, p[0].index, len(c) - 1
}

func matchRunningFlat(c []ohlcv.Candle, s []swing) (bool, int, int) {
	p := lastSwings(s, "", 3)
	if len(p) < 3 {
		return false, 0, 0
	}
	return math.Abs(p[2].price-p[0].price) < math.Abs(p[1].price-p[0].price)*0.15, p[0].index, len(c) - 1
}

func matchDiagonal(c []ohlcv.Candle, s []swing) (bool, int, int) {
	ok, start, end := matchElliottImpulse(c, s)
	if !ok {
		return false, 0, 0
	}
	return rangeWidth(c[maxInt(0, len(c)-20):]) < rangeWidth(c[maxInt(0, len(c)-50):maxInt(1, len(c)-30)]), start, end
}

func matchWyckoff(accumulation bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 60 {
			return false, 0, 0
		}
		pos := mathutil.SafeDiv(c[len(c)-1].EffectiveClose()-lowest(c[len(c)-60:]), highest(c[len(c)-60:])-lowest(c[len(c)-60:]))
		if accumulation {
			return pos > 0.45 && trend(c, len(c)-1, 60) <= 0.05, len(c) - 60, len(c) - 1
		}
		return pos < 0.55 && trend(c, len(c)-1, 60) >= -0.05, len(c) - 60, len(c) - 1
	}
}

func matchSpring(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return sweep(c, false), maxInt(0, len(c)-20), len(c) - 1
}
func matchUpthrust(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return sweep(c, true), maxInt(0, len(c)-20), len(c) - 1
}
func matchSOS(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return breakout(c, true), maxInt(0, len(c)-20), len(c) - 1
}
func matchSOW(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return breakout(c, false), maxInt(0, len(c)-20), len(c) - 1
}
func matchLPS(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return trend(c, len(c)-1, 30) > 0 && !breakout(c, false), maxInt(0, len(c)-30), len(c) - 1
}
func matchLPSY(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return trend(c, len(c)-1, 30) < 0 && !breakout(c, true), maxInt(0, len(c)-30), len(c) - 1
}

func matchClimax(up bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 20 {
			return false, 0, 0
		}
		lastC := c[len(c)-1]
		rangeBig := relativeRange(c) > 1.5
		volBig := lastC.EffectiveVolume() > averageVolume(c[len(c)-20:])*1.4
		if up {
			return rangeBig && volBig && lastC.EffectiveClose() > lastC.EffectiveOpen(), len(c) - 20, len(c) - 1
		}
		return rangeBig && volBig && lastC.EffectiveClose() < lastC.EffectiveOpen(), len(c) - 20, len(c) - 1
	}
}

func matchAutomatic(up bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if up {
			return trend(c, len(c)-1, 8) > 0.04 && trend(c, len(c)-8, 20) < 0, maxInt(0, len(c)-28), len(c) - 1
		}
		return trend(c, len(c)-1, 8) < -0.04 && trend(c, len(c)-8, 20) > 0, maxInt(0, len(c)-28), len(c) - 1
	}
}

func matchBOS(up bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		return breakout(c, up), maxInt(0, len(c)-30), len(c) - 1
	}
}

func matchCHOCH(up bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if up {
			return trend(c, len(c)-20, 30) < 0 && breakout(c, true), maxInt(0, len(c)-50), len(c) - 1
		}
		return trend(c, len(c)-20, 30) > 0 && breakout(c, false), maxInt(0, len(c)-50), len(c) - 1
	}
}

func matchLiquidityGrab(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return sweep(c, true) || sweep(c, false), maxInt(0, len(c)-20), len(c) - 1
}
func matchFairValueGap(c []ohlcv.Candle, s []swing) (bool, int, int) {
	if len(c) < 3 {
		return false, 0, 0
	}
	i := len(c) - 1
	return c[i].EffectiveLow() > c[i-2].EffectiveHigh() || c[i].EffectiveHigh() < c[i-2].EffectiveLow(), i - 2, i
}

func matchOrderBlock(up bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 6 {
			return false, 0, 0
		}
		prev := shape(c[len(c)-2])
		move := math.Abs(c[len(c)-1].EffectiveClose() - c[len(c)-5].EffectiveOpen())
		if up {
			return prev.bearish && c[len(c)-1].EffectiveClose() > c[len(c)-1].EffectiveOpen() && move > averageRange(c)*2, len(c) - 6, len(c) - 1
		}
		return prev.bullish && c[len(c)-1].EffectiveClose() < c[len(c)-1].EffectiveOpen() && move > averageRange(c)*2, len(c) - 6, len(c) - 1
	}
}

func matchBreaker(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return (breakout(c, true) || breakout(c, false)) && (sweep(c, true) || sweep(c, false)), maxInt(0, len(c)-30), len(c) - 1
}

func matchMitigation(c []ohlcv.Candle, s []swing) (bool, int, int) {
	if len(c) < 12 {
		return false, 0, 0
	}
	zoneHigh := c[len(c)-10].EffectiveHigh()
	zoneLow := c[len(c)-10].EffectiveLow()
	lastC := c[len(c)-1]
	return lastC.EffectiveLow() <= zoneHigh && lastC.EffectiveHigh() >= zoneLow, len(c) - 10, len(c) - 1
}

func matchEqualLiquidity(highsWanted bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	kind := "low"
	if highsWanted {
		kind = "high"
	}
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		p := lastSwings(s, kind, 2)
		if len(p) < 2 {
			return false, 0, 0
		}
		return near(p[0].price, p[1].price, 0.005), p[0].index, len(c) - 1
	}
}

func matchComplexHS(inverse bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		ok, start, end := matchHeadShoulders(inverse)(c, s)
		if ok {
			return true, start, end
		}
		p := lastSwings(s, map[bool]string{true: "low", false: "high"}[inverse], 5)
		if len(p) < 5 {
			return false, 0, 0
		}
		return near(p[0].price, p[4].price, 0.03), p[0].index, len(c) - 1
	}
}

func matchMeasuredMove(up bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		p := lastSwings(s, "", 4)
		if len(p) < 4 {
			return false, 0, 0
		}
		leg1 := p[1].price - p[0].price
		leg2 := p[3].price - p[2].price
		return (leg1 > 0) == up && (leg2 > 0) == up && near(math.Abs(leg1), math.Abs(leg2), 0.25), p[0].index, len(c) - 1
	}
}

func matchBumpRun(top bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 50 {
			return false, 0, 0
		}
		oldSlope := trend(c, len(c)-20, 30)
		recentSlope := trend(c, len(c)-1, 20)
		if top {
			return oldSlope > 0 && recentSlope < oldSlope*-0.2, len(c) - 50, len(c) - 1
		}
		return oldSlope < 0 && recentSlope > math.Abs(oldSlope)*0.2, len(c) - 50, len(c) - 1
	}
}

func matchIsland(top bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 4 {
			return false, 0, 0
		}
		// The island (candles at len-3 and len-2) must be isolated by a gap on both
		// sides: entirely clear of `a`'s range on entry and entirely clear of `d`'s
		// range on exit. Checking only the len-3 candle against `d` (skipping len-2)
		// lets the middle candle silently overlap either gap without the pattern
		// noticing.
		a, b, mid, d := shape(c[len(c)-4]), shape(c[len(c)-3]), shape(c[len(c)-2]), shape(c[len(c)-1])
		islandLow := math.Min(b.low, mid.low)
		islandHigh := math.Max(b.high, mid.high)
		if top {
			return islandLow > a.high && d.high < islandLow, len(c) - 4, len(c) - 1
		}
		return islandHigh < a.low && d.low > islandHigh, len(c) - 4, len(c) - 1
	}
}

func matchPipe(top bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 2 {
			return false, 0, 0
		}
		a, b := shape(c[len(c)-2]), shape(c[len(c)-1])
		wide := a.rangeSize > averageRange(c)*1.4 && b.rangeSize > averageRange(c)*1.4
		if top {
			return wide && a.bullish && b.bearish, len(c) - 2, len(c) - 1
		}
		return wide && a.bearish && b.bullish, len(c) - 2, len(c) - 1
	}
}

func matchV(top bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		if len(c) < 20 {
			return false, 0, 0
		}
		left := trend(c, len(c)-10, 10)
		right := trend(c, len(c)-1, 10)
		if top {
			return left > 0.06 && right < -0.06, len(c) - 20, len(c) - 1
		}
		return left < -0.06 && right > 0.06, len(c) - 20, len(c) - 1
	}
}

func matchAdamEve(top bool, mixed bool) func([]ohlcv.Candle, []swing) (bool, int, int) {
	return func(c []ohlcv.Candle, s []swing) (bool, int, int) {
		base := matchDouble(map[bool]string{true: "high", false: "low"}[top], 2, 0.02, 8, 60, !top)
		ok, start, end := base(c, s)
		if !ok {
			return false, 0, 0
		}
		_ = mixed
		return true, start, end
	}
}

func matchAdamAdamTop(c []ohlcv.Candle, s []swing) (bool, int, int) {
	return matchDouble("high", 2, 0.02, 8, 60, false)(c, s)
}

func lastSwings(swings []swing, kind string, count int) []swing {
	var out []swing
	for i := len(swings) - 1; i >= 0 && len(out) < count; i-- {
		if kind == "" || swings[i].kind == kind {
			out = append([]swing{swings[i]}, out...)
		}
	}
	return out
}

func swingsBetween(swings []swing, kind string, start, end int) []swing {
	var out []swing
	for _, point := range swings {
		if point.index <= start || point.index >= end {
			continue
		}
		if kind == "" || point.kind == kind {
			out = append(out, point)
		}
	}
	return out
}

func necklineBreak(c []ohlcv.Candle, swings []swing, first, second swing, bullish bool) bool {
	if len(c) <= second.index+1 {
		return false
	}
	kind := "high"
	if !bullish {
		kind = "low"
	}
	points := swingsBetween(swings, kind, first.index, second.index)
	if len(points) == 0 {
		return false
	}
	level := points[0].price
	for _, point := range points[1:] {
		if bullish {
			level = math.Max(level, point.price)
		} else {
			level = math.Min(level, point.price)
		}
	}
	lastClose := c[len(c)-1].EffectiveClose()
	if bullish {
		return lastClose > level
	}
	return lastClose < level
}

func swingKinds(points []swing, kinds ...string) bool {
	if len(points) != len(kinds) {
		return false
	}
	for i, kind := range kinds {
		if points[i].kind != kind {
			return false
		}
	}
	return true
}

func alternatingSwings(points []swing) bool {
	if len(points) < 2 {
		return false
	}
	for i := 1; i < len(points); i++ {
		if points[i].kind == points[i-1].kind {
			return false
		}
	}
	return true
}

func maxSwingPrice(points []swing) float64 {
	if len(points) == 0 {
		return 0
	}
	value := points[0].price
	for _, point := range points[1:] {
		value = math.Max(value, point.price)
	}
	return value
}

func minSwingPrice(points []swing) float64 {
	if len(points) == 0 {
		return 0
	}
	value := points[0].price
	for _, point := range points[1:] {
		value = math.Min(value, point.price)
	}
	return value
}

func rangeContracted(c []ohlcv.Candle, recent, prior int) bool {
	if recent <= 0 || prior <= recent || len(c) < prior {
		return false
	}
	return rangeWidth(c[len(c)-recent:]) < rangeWidth(c[len(c)-prior:len(c)-recent])*0.85
}

func elliottImpulseOK(p []swing, bullish bool) bool {
	if len(p) < 6 {
		return false
	}
	if bullish {
		if !swingKinds(p[:6], "low", "high", "low", "high", "low", "high") {
			return false
		}
		wave1 := p[1].price - p[0].price
		wave3 := p[3].price - p[2].price
		wave5 := p[5].price - p[4].price
		wave2Valid := p[2].price > p[0].price && p[2].price < p[1].price
		wave4Valid := p[4].price > p[1].price && p[4].price < p[3].price
		progress := p[3].price > p[1].price && p[5].price > p[3].price
		return wave1 > 0 && wave3 > 0 && wave5 > 0 && wave2Valid && wave4Valid && progress && wave3 >= math.Min(wave1, wave5)
	}
	if !swingKinds(p[:6], "high", "low", "high", "low", "high", "low") {
		return false
	}
	wave1 := p[0].price - p[1].price
	wave3 := p[2].price - p[3].price
	wave5 := p[4].price - p[5].price
	wave2Valid := p[2].price < p[0].price && p[2].price > p[1].price
	wave4Valid := p[4].price < p[1].price && p[4].price > p[3].price
	progress := p[3].price < p[1].price && p[5].price < p[3].price
	return wave1 > 0 && wave3 > 0 && wave5 > 0 && wave2Valid && wave4Valid && progress && wave3 >= math.Min(wave1, wave5)
}

func trend(c []ohlcv.Candle, end, period int) float64 {
	if len(c) == 0 {
		return 0
	}
	if end >= len(c) {
		end = len(c) - 1
	}
	if end < 0 {
		end = 0
	}
	start := maxInt(0, end-period)
	return priceChangeRatio(c[end].EffectiveClose(), c[start].EffectiveClose())
}

func rangeWidth(c []ohlcv.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	return mathutil.SafeDiv(highest(c)-lowest(c), priceDenominator(c[len(c)-1].EffectiveClose()))
}

func highest(c []ohlcv.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	value := c[0].EffectiveHigh()
	for _, candle := range c[1:] {
		value = math.Max(value, candle.EffectiveHigh())
	}
	return value
}

func priceChangeRatio(current, base float64) float64 {
	return mathutil.SafeDiv(current-base, priceDenominator(base))
}

func priceDenominator(value float64) float64 {
	return math.Max(math.Abs(value), mathutil.Epsilon)
}

func lowest(c []ohlcv.Candle) float64 {
	if len(c) == 0 {
		return 0
	}
	value := math.MaxFloat64
	for _, candle := range c {
		value = math.Min(value, candle.EffectiveLow())
	}
	return value
}

func breakout(c []ohlcv.Candle, up bool) bool {
	if len(c) < 21 {
		return false
	}
	prev := c[len(c)-21 : len(c)-1]
	lastC := c[len(c)-1]
	if up {
		return lastC.EffectiveClose() > highest(prev)
	}
	return lastC.EffectiveClose() < lowest(prev)
}

func sweep(c []ohlcv.Candle, high bool) bool {
	if len(c) < 21 {
		return false
	}
	prev := c[len(c)-21 : len(c)-1]
	lastC := c[len(c)-1]
	if high {
		level := highest(prev)
		return lastC.EffectiveHigh() > level && lastC.EffectiveClose() < level
	}
	level := lowest(prev)
	return lastC.EffectiveLow() < level && lastC.EffectiveClose() > level
}

func averageRange(c []ohlcv.Candle) float64 {
	total := 0.0
	for _, candle := range c {
		total += candle.EffectiveHigh() - candle.EffectiveLow()
	}
	return mathutil.SafeDiv(total, float64(len(c)))
}

func averageVolume(c []ohlcv.Candle) float64 {
	total := 0.0
	for _, candle := range c {
		total += candle.EffectiveVolume()
	}
	return mathutil.SafeDiv(total, float64(len(c)))
}

func between(value, low, high float64) bool {
	return value >= low && value <= high
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
