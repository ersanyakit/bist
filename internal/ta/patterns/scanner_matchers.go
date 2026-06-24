package patterns

import (
	"fmt"
	"math"
	"strings"

	"hissebot/internal/ta/indicators"
	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

type generatedMatch struct {
	ok         bool
	start      int
	end        int
	direction  string
	confidence float64
	evidence   string
}

type generatedPatternWindow struct {
	start int
	end   int
}

type generatedVolumePolicy int

const (
	generatedVolumeIgnored generatedVolumePolicy = iota
	generatedVolumeOptional
	generatedVolumeRequired
	generatedVolumeIntrinsic
)

type generatedVolumeAssessment struct {
	confirmed bool
	evidence  string
}

func detectPatternSpec(input ScannerInput, spec patternSpec) ([]ohlcv.PatternResult, error) {
	if len(input.Candles) == 0 {
		return nil, fmt.Errorf("generated detector requires candles: %w", ErrPatternData)
	}
	spec = applyPatternRuleOverrides(spec)
	spec = bindPatternRule(spec)
	match := matchPatternSpec(input, spec)
	if !match.ok {
		return nil, nil
	}
	result, err := buildGeneratedPatternResult(input, spec, match)
	if err != nil {
		return nil, fmt.Errorf("build generated pattern result %q: %w", spec.Name, err)
	}
	return []ohlcv.PatternResult{result}, nil
}

func buildGeneratedPatternResult(input ScannerInput, spec patternSpec, match generatedMatch) (ohlcv.PatternResult, error) {
	window, err := normalizeGeneratedMatchWindow(input.Candles, match)
	if err != nil {
		return ohlcv.PatternResult{}, err
	}
	policy := generatedVolumePolicyForSpec(spec)
	volume := assessGeneratedVolume(input, window, policy)
	confidence := scoreGeneratedConfidence(spec, match, policy, volume)
	evidence := generatedPatternEvidence(spec, match, policy, volume)
	result := ohlcv.PatternResult{
		Name:            spec.Name,
		Category:        spec.Category,
		Direction:       generatedPatternDirection(spec, match),
		Confidence:      confidence,
		StartIndex:      window.start,
		EndIndex:        window.end,
		StartTime:       input.Candles[window.start].Time,
		EndTime:         input.Candles[window.end].Time,
		Evidence:        evidence,
		VolumeConfirmed: volume.confirmed,
	}
	return enrichPatternBacktestMetadata(result, input.Candles), nil
}

func normalizeGeneratedMatchWindow(candles []ohlcv.Candle, match generatedMatch) (generatedPatternWindow, error) {
	if len(candles) == 0 {
		return generatedPatternWindow{}, fmt.Errorf("cannot build pattern window without candles: %w", ErrPatternData)
	}
	start := match.start
	end := match.end
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = len(candles) - 1
	}
	if start >= len(candles) {
		return generatedPatternWindow{}, fmt.Errorf("pattern start index %d is outside candle range %d", match.start, len(candles))
	}
	if end >= len(candles) {
		end = len(candles) - 1
	}
	if end < start {
		return generatedPatternWindow{}, fmt.Errorf("pattern end index %d is before start index %d", match.end, match.start)
	}
	return generatedPatternWindow{start: start, end: end}, nil
}

func generatedPatternDirection(spec patternSpec, match generatedMatch) string {
	direction := match.direction
	if direction == "" {
		direction = spec.Direction
	}
	return direction
}

func scoreGeneratedConfidence(spec patternSpec, match generatedMatch, policy generatedVolumePolicy, volume generatedVolumeAssessment) float64 {
	confidence := match.confidence
	if confidence <= 0 {
		confidence = spec.Confidence
	}
	switch policy {
	case generatedVolumeOptional:
		if volume.confirmed {
			confidence += 0.04
		}
	case generatedVolumeRequired:
		if volume.confirmed {
			confidence += 0.06
		} else {
			confidence *= 0.80
		}
	}
	return mathutil.Clamp(confidence, 0, 1)
}

func generatedPatternEvidence(spec patternSpec, match generatedMatch, policy generatedVolumePolicy, volume generatedVolumeAssessment) []string {
	evidence := match.evidence
	if evidence == "" {
		evidence = spec.Evidence
	}
	out := []string{evidence}
	if policy != generatedVolumeIgnored && volume.evidence != "" {
		out = append(out, volume.evidence)
	}
	return out
}

func assessGeneratedVolume(input ScannerInput, window generatedPatternWindow, policy generatedVolumePolicy) generatedVolumeAssessment {
	switch policy {
	case generatedVolumeIgnored:
		return generatedVolumeAssessment{}
	case generatedVolumeIntrinsic:
		return generatedVolumeAssessment{confirmed: true, evidence: "volume behavior is part of the matched setup"}
	default:
		confirmed := volumeConfirmed(input.Candles[window.end], volumeSMAAt(input.Candles, window.end, 20))
		return generatedVolumeAssessment{confirmed: confirmed, evidence: volumeEvidence(confirmed)}
	}
}

func generatedVolumePolicyForSpec(spec patternSpec) generatedVolumePolicy {
	rule := patternRuleForSpec(spec)
	if spec.Category == "volume" || rule == ruleVolume {
		return generatedVolumeIntrinsic
	}
	switch rule {
	case ruleBuyingClimax, ruleSellingClimax, ruleSOS, ruleSOW:
		return generatedVolumeRequired
	case ruleBreakout, ruleBreakdown, ruleFlagBull, ruleFlagBear, rulePennantBull, rulePennantBear, ruleSpring, ruleUpthrust, ruleLPS, ruleLPSY:
		return generatedVolumeOptional
	}
	switch spec.Category {
	case "classic_chart", "price_action", "wyckoff", "gap", "trend_channel":
		return generatedVolumeOptional
	default:
		return generatedVolumeIgnored
	}
}

func patternRuleForSpec(spec patternSpec) patternRuleID {
	if spec.Rule != "" {
		return spec.Rule
	}
	return patternRuleID(spec.Template)
}

func matchPatternSpec(input ScannerInput, spec patternSpec) generatedMatch {
	spec = applyPatternRuleOverrides(spec)
	spec = bindPatternRule(spec)
	rule, ok := registeredPatternRule(spec.Rule)
	if !ok {
		return generatedMatch{}
	}
	return matchPatternSpecExhaustive(input, spec, rule)
}

func matchPatternRuleOnce(input ScannerInput, spec patternSpec, rule patternRule) generatedMatch {
	return rule.match(newPatternMatchContext(input), spec)
}

func matchPatternSpecExhaustive(input ScannerInput, spec patternSpec, rule patternRule) generatedMatch {
	candles := input.Candles
	best := normalizeGeneratedWindowMatch(candles, 0, matchPatternRuleOnce(input, spec, rule))
	if len(candles) == 0 || patternRequiresExternalData(spec.Template) {
		return best
	}
	for _, window := range exhaustivePatternScanWindows(len(candles), spec.Rule) {
		if window.start == 0 && window.end == len(candles)-1 {
			continue
		}
		windowInput := patternWindowInput(input, window, spec.Rule)
		match := matchPatternRuleOnce(windowInput, spec, rule)
		match = normalizeGeneratedWindowMatch(windowInput.Candles, window.start, match)
		if betterGeneratedMatch(spec, match, best) {
			best = match
		}
	}
	return best
}

type patternScanWindow struct {
	start int
	end   int
}

func exhaustivePatternScanWindows(total int, rule patternRuleID) []patternScanWindow {
	if total <= 0 {
		return nil
	}
	sizes := patternScanWindowSizes(total, rule)
	seen := map[patternScanWindow]struct{}{}
	windows := make([]patternScanWindow, 0, total*(len(sizes)+1))
	add := func(start, end int) {
		if start < 0 || end < start || end >= total {
			return
		}
		window := patternScanWindow{start: start, end: end}
		if _, ok := seen[window]; ok {
			return
		}
		seen[window] = struct{}{}
		windows = append(windows, window)
	}
	for end := 0; end < total; end++ {
		add(0, end)
		available := end + 1
		for _, size := range sizes {
			if size <= available {
				add(end-size+1, end)
			}
		}
	}
	return windows
}

func patternScanWindowSizes(total int, rule patternRuleID) []int {
	base := []int{1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377}
	switch rule {
	case ruleCandlestick:
		base = []int{1, 2, 3, 5, 8, 13}
	case ruleGap, ruleGapUp, ruleGapDown, ruleFVG:
		base = []int{2, 3, 5, 8, 13, 21}
	case ruleIndicator, ruleMeanReversion, ruleFibonacci:
		base = []int{20, 30, 50, 80, 120, 160, 220, 260, 320}
	case ruleMarketProfile:
		base = []int{12, 20, 30, 40, 60, 80, 120, 160, 240}
	case rulePointFigure:
		base = []int{15, 30, 50, 80, 120, 200, 320}
	case ruleElliottImpulse, ruleElliottABC, ruleElliottZigzag, ruleElliottFlat, ruleElliottExpandedFlat, ruleElliottRunningFlat, ruleElliottDiagonal,
		ruleABCD, ruleHarmonicGeneric, ruleHarmonicGartley, ruleHarmonicBat, ruleHarmonicButterfly, ruleHarmonicCrab, ruleHarmonicDeepCrab, ruleHarmonicShark, ruleHarmonicCypher, ruleThreeDrives, ruleFiveZero:
		base = []int{34, 55, 89, 144, 233, 377}
	}
	out := make([]int, 0, len(base)+1)
	seen := map[int]struct{}{}
	for _, size := range base {
		if size <= 0 || size > total {
			continue
		}
		if _, ok := seen[size]; ok {
			continue
		}
		seen[size] = struct{}{}
		out = append(out, size)
	}
	if _, ok := seen[total]; !ok {
		out = append(out, total)
	}
	return out
}

func patternWindowInput(input ScannerInput, window patternScanWindow, rule patternRuleID) ScannerInput {
	out := input
	out.Candles = input.Candles[window.start : window.end+1]
	out.chartSwings = nil
	if patternRuleNeedsWindowIndicators(rule) {
		if snapshot, err := indicators.Snapshot(out.Candles); err == nil {
			out.Indicators = snapshot
		}
	}
	return out
}

func patternRuleNeedsWindowIndicators(rule patternRuleID) bool {
	switch rule {
	case ruleIndicator, ruleMeanReversion, ruleFibonacci:
		return true
	default:
		return false
	}
}

func normalizeGeneratedWindowMatch(windowCandles []ohlcv.Candle, globalStart int, match generatedMatch) generatedMatch {
	if !match.ok {
		return generatedMatch{}
	}
	window, err := normalizeGeneratedMatchWindow(windowCandles, match)
	if err != nil {
		return generatedMatch{}
	}
	match.start = globalStart + window.start
	match.end = globalStart + window.end
	return match
}

func betterGeneratedMatch(spec patternSpec, candidate, current generatedMatch) bool {
	if !candidate.ok {
		return false
	}
	if !current.ok {
		return true
	}
	candidateConfidence := effectiveGeneratedMatchConfidence(spec, candidate)
	currentConfidence := effectiveGeneratedMatchConfidence(spec, current)
	if candidate.end != current.end && math.Abs(candidateConfidence-currentConfidence) <= 0.12 {
		return candidate.end > current.end
	}
	if math.Abs(candidateConfidence-currentConfidence) > 1e-9 {
		return candidateConfidence > currentConfidence
	}
	if candidate.end != current.end {
		return candidate.end > current.end
	}
	return candidate.start > current.start
}

func effectiveGeneratedMatchConfidence(spec patternSpec, match generatedMatch) float64 {
	if match.confidence > 0 {
		return match.confidence
	}
	return spec.Confidence
}

func volumeSMAAt(candles []ohlcv.Candle, end, period int) float64 {
	if len(candles) == 0 || end < 0 || end >= len(candles) || period <= 0 {
		return 0
	}
	start := end - period + 1
	if start < 0 {
		start = 0
	}
	sum := 0.0
	for _, candle := range candles[start : end+1] {
		sum += candle.EffectiveVolume()
	}
	return mathutil.SafeDiv(sum, float64(end-start+1))
}

func adaptMatch(fn func([]ohlcv.Candle, []swing) (bool, int, int), c []ohlcv.Candle, s []swing, direction string, confidence float64, evidence string) generatedMatch {
	ok, start, end := fn(c, s)
	return generatedMatch{ok: ok, start: start, end: end, direction: direction, confidence: confidence, evidence: evidence}
}

func matchCandlestickAlias(spec patternSpec, c []ohlcv.Candle) generatedMatch {
	if len(c) == 0 {
		return generatedMatch{}
	}
	name := normalizePatternText(spec.Name)
	i := len(c) - 1
	cur := shape(c[i])
	one := func(ok bool, evidence string) generatedMatch {
		return generatedMatch{ok: ok, start: i, end: i, direction: spec.Direction, confidence: spec.Confidence, evidence: evidence}
	}
	two := func(ok bool, evidence string) generatedMatch {
		return generatedMatch{ok: ok, start: maxInt(0, i-1), end: i, direction: spec.Direction, confidence: spec.Confidence, evidence: evidence}
	}
	three := func(ok bool, evidence string) generatedMatch {
		return generatedMatch{ok: ok, start: maxInt(0, i-2), end: i, direction: spec.Direction, confidence: spec.Confidence, evidence: evidence}
	}
	if strings.Contains(name, "four price doji") {
		return one(cur.rangeSize <= mathutil.Epsilon || cur.body <= cur.rangeSize*0.01, "open high low and close are nearly identical")
	}
	if strings.Contains(name, "dragonfly doji") {
		return one(cur.body <= cur.rangeSize*0.05 && cur.lowerWick >= cur.rangeSize*0.55 && cur.upperWick <= cur.rangeSize*0.12, "dragonfly doji candle shape matched")
	}
	if strings.Contains(name, "gravestone doji") {
		return one(cur.body <= cur.rangeSize*0.05 && cur.upperWick >= cur.rangeSize*0.55 && cur.lowerWick <= cur.rangeSize*0.12, "gravestone doji candle shape matched")
	}
	if strings.Contains(name, "doji") {
		return one(cur.body <= cur.rangeSize*0.08, "doji-like small candle body matched")
	}
	if strings.Contains(name, "marubozu") {
		ok := cur.upperWick+cur.lowerWick <= cur.rangeSize*0.08 && cur.body >= cur.rangeSize*0.75
		if strings.Contains(name, "bullish") {
			ok = ok && cur.bullish
		}
		if strings.Contains(name, "bearish") {
			ok = ok && cur.bearish
		}
		return one(ok, "marubozu body and wick proportions matched")
	}
	if strings.Contains(name, "pin bar") || strings.Contains(name, "hammer") || strings.Contains(name, "hanging man") || strings.Contains(name, "shooting star") {
		longLower := cur.lowerWick >= cur.body*2 && cur.upperWick <= math.Max(cur.body*0.35, cur.rangeSize*0.08)
		longUpper := cur.upperWick >= cur.body*2 && cur.lowerWick <= math.Max(cur.body*0.35, cur.rangeSize*0.08)
		ok := false
		switch {
		case strings.Contains(name, "shooting star"):
			ok = longUpper && uptrend(c, i, 5)
		case strings.Contains(name, "inverted hammer"):
			ok = longUpper && downtrend(c, i, 5)
		case strings.Contains(name, "hanging man"):
			ok = longLower && uptrend(c, i, 5)
		case strings.Contains(name, "hammer"):
			ok = longLower && downtrend(c, i, 5)
		case strings.Contains(name, "bullish"):
			ok = longLower
		case strings.Contains(name, "bearish"):
			ok = longUpper
		default:
			ok = longLower || longUpper
		}
		return one(ok, "dominant wick reversal candle matched")
	}
	if strings.Contains(name, "belt hold") {
		ok := cur.body >= cur.rangeSize*0.55
		if strings.Contains(name, "bullish") {
			ok = ok && cur.bullish && cur.lowerWick <= cur.rangeSize*0.08
		}
		if strings.Contains(name, "bearish") {
			ok = ok && cur.bearish && cur.upperWick <= cur.rangeSize*0.08
		}
		return one(ok, "belt-hold body and open location matched")
	}
	if strings.Contains(name, "high wave") || strings.Contains(name, "spinning") {
		return one(cur.body <= cur.rangeSize*0.30 && cur.upperWick >= cur.body && cur.lowerWick >= cur.body, "small body with balanced shadows matched")
	}
	if strings.Contains(name, "long upper") {
		return one(cur.upperWick >= cur.rangeSize*0.55, "long upper shadow matched")
	}
	if strings.Contains(name, "long lower") {
		return one(cur.lowerWick >= cur.rangeSize*0.55, "long lower shadow matched")
	}
	if strings.Contains(name, "short body") || strings.Contains(name, "short line") {
		return one(cur.body <= cur.rangeSize*0.25, "short body candle matched")
	}
	if strings.Contains(name, "long body") || strings.Contains(name, "long line") {
		return one(cur.body >= cur.rangeSize*0.65, "long body candle matched")
	}
	if len(c) < 2 {
		return generatedMatch{}
	}
	prev := shape(c[i-1])
	if strings.Contains(name, "engulfing") || strings.Contains(name, "outside bar") {
		bull := prev.bearish && cur.bullish && cur.open <= prev.close && cur.close >= prev.open
		bear := prev.bullish && cur.bearish && cur.open >= prev.close && cur.close <= prev.open
		return two(directionOK(spec.Direction, bull, bear), "engulfing or outside-bar structure matched")
	}
	if strings.Contains(name, "harami") || strings.Contains(name, "inside bar") {
		inside := cur.high <= prev.high && cur.low >= prev.low
		if strings.Contains(name, "cross") {
			inside = inside && cur.body <= cur.rangeSize*0.08
		}
		bull := inside && prev.bearish
		bear := inside && prev.bullish
		return two(directionOK(spec.Direction, bull, bear), "harami or inside-bar containment matched")
	}
	if strings.Contains(name, "piercing") {
		return two(prev.bearish && cur.bullish && cur.close > (prev.open+prev.close)/2, "piercing-line recovery matched")
	}
	if strings.Contains(name, "dark cloud") {
		return two(prev.bullish && cur.bearish && cur.close < (prev.open+prev.close)/2, "dark-cloud close below midpoint matched")
	}
	if strings.Contains(name, "tweezer top") || strings.Contains(name, "matching high") {
		return two(near(prev.high, cur.high, 0.005), "matching adjacent highs matched")
	}
	if strings.Contains(name, "tweezer bottom") || strings.Contains(name, "matching low") {
		return two(near(prev.low, cur.low, 0.005), "matching adjacent lows matched")
	}
	if strings.Contains(name, "counterattack") || strings.Contains(name, "meeting line") {
		return two(near(prev.close, cur.close, 0.006), "counterattack or meeting-line close alignment matched")
	}
	if strings.Contains(name, "kicking") || strings.Contains(name, "kicker") {
		bull := prev.bearish && cur.bullish && cur.open > prev.open && cur.body >= prev.body*0.7
		bear := prev.bullish && cur.bearish && cur.open < prev.open && cur.body >= prev.body*0.7
		return two(directionOK(spec.Direction, bull, bear), "forceful opposite-color kicking structure matched")
	}
	if strings.Contains(name, "separating") {
		bull := prev.bearish && cur.bullish && near(prev.open, cur.open, 0.01)
		bear := prev.bullish && cur.bearish && near(prev.open, cur.open, 0.01)
		return two(directionOK(spec.Direction, bull, bear), "same-open separating lines matched")
	}
	if strings.Contains(name, "neck") || strings.Contains(name, "thrusting") {
		return two(prev.bearish && cur.bullish && cur.close > prev.close && cur.close < (prev.open+prev.close)/2, "neck or thrusting recovery matched")
	}
	if strings.Contains(name, "homing pigeon") {
		return two(prev.bearish && cur.bearish && cur.body < prev.body && cur.open < prev.open && cur.close > prev.close, "small bearish body nested inside a larger bearish body")
	}
	if strings.Contains(name, "descending hawk") {
		return two(prev.bullish && cur.bullish && cur.body < prev.body && cur.open > prev.open && cur.close < prev.close, "small bullish body nested inside a larger bullish body")
	}
	if strings.Contains(name, "gapping doji") {
		return two(cur.body <= cur.rangeSize*0.08 && (cur.low > prev.high || cur.high < prev.low), "gapping doji matched")
	}
	if len(c) < 3 {
		return generatedMatch{}
	}
	a := shape(c[i-2])
	if strings.Contains(name, "morning") {
		mid := shape(c[i-1])
		return three(downtrend(c, i-2, 5) && a.bearish && a.body > a.rangeSize*0.40 && mid.body <= a.body*0.50 && cur.bullish && cur.close > (a.open+a.close)/2, "morning-star style reversal matched")
	}
	if strings.Contains(name, "evening") {
		mid := shape(c[i-1])
		return three(uptrend(c, i-2, 5) && a.bullish && a.body > a.rangeSize*0.40 && mid.body <= a.body*0.50 && cur.bearish && cur.close < (a.open+a.close)/2, "evening-star style reversal matched")
	}
	if strings.Contains(name, "abandoned baby") {
		mid := shape(c[i-1])
		gappedDown := mid.body <= mid.rangeSize*0.08 && mid.high < a.low && cur.low > mid.high
		gappedUp := mid.body <= mid.rangeSize*0.08 && mid.low > a.high && cur.high < mid.low
		return three(directionOK(spec.Direction, gappedDown, gappedUp), "abandoned baby gap isolation matched")
	}
	if strings.Contains(name, "three white") || strings.Contains(name, "advancing white") {
		b := shape(c[i-1])
		return three(a.bullish && b.bullish && cur.bullish && a.close < b.close && b.close < cur.close, "three advancing bullish candles matched")
	}
	if strings.Contains(name, "three black") || strings.Contains(name, "three crows") {
		b := shape(c[i-1])
		return three(a.bearish && b.bearish && cur.bearish && a.close > b.close && b.close > cur.close, "three declining bearish candles matched")
	}
	if strings.Contains(name, "three inside") {
		inside := c[i-1].EffectiveHigh() <= c[i-2].EffectiveHigh() && c[i-1].EffectiveLow() >= c[i-2].EffectiveLow()
		bull := inside && cur.bullish && cur.close > a.open
		bear := inside && cur.bearish && cur.close < a.open
		return three(directionOK(spec.Direction, bull, bear), "three-inside confirmation matched")
	}
	if strings.Contains(name, "three outside") {
		bull := a.bearish && shape(c[i-1]).bullish && c[i-1].EffectiveClose() >= a.open && cur.close > c[i-1].EffectiveClose()
		bear := a.bullish && shape(c[i-1]).bearish && c[i-1].EffectiveClose() <= a.open && cur.close < c[i-1].EffectiveClose()
		return three(directionOK(spec.Direction, bull, bear), "three-outside confirmation matched")
	}
	if strings.Contains(name, "tri star") || strings.Contains(name, "three stars") {
		return three(threeDoji(c), "three doji candles matched")
	}
	if strings.Contains(name, "two crows") {
		b := shape(c[i-1])
		return three(a.bullish && b.bearish && cur.bearish, "two bearish candles after bullish candle matched")
	}
	if strings.Contains(name, "advance block") || strings.Contains(name, "deliberation") || strings.Contains(name, "stalled") {
		b := shape(c[i-1])
		return three(a.bullish && b.bullish && cur.bullish && cur.body < b.body*0.7, "weakening bullish three-candle advance matched")
	}
	if strings.Contains(name, "stick sandwich") {
		return three(a.bearish && shape(c[i-1]).bullish && cur.bearish && near(a.close, cur.close, 0.006), "stick sandwich matching closes matched")
	}
	if strings.Contains(name, "unique three river") || strings.Contains(name, "three river") {
		return three(a.bearish && shape(c[i-1]).bearish && c[i-1].EffectiveLow() < a.low && cur.bullish, "three river bottom structure matched")
	}
	if strings.Contains(name, "rising three") || strings.Contains(name, "falling three") || strings.Contains(name, "mat hold") {
		if strings.Contains(name, "falling") {
			return adaptMatch(func(c []ohlcv.Candle, _ []swing) (bool, int, int) {
				return matHold(c, false), maxInt(0, len(c)-5), len(c) - 1
			}, c, nil, "bearish", spec.Confidence, "falling method or mat-hold structure matched")
		}
		return adaptMatch(func(c []ohlcv.Candle, _ []swing) (bool, int, int) {
			return matHold(c, true), maxInt(0, len(c)-5), len(c) - 1
		}, c, nil, "bullish", spec.Confidence, "rising method or mat-hold structure matched")
	}
	if strings.Contains(name, "window") || strings.Contains(name, "gap") || strings.Contains(name, "breakaway") || strings.Contains(name, "gapping play") {
		return matchGap(c, spec.Direction, spec.Confidence, "multi-candle gap/window structure matched")
	}
	if strings.Contains(name, "ladder bottom") {
		return adaptMatch(func(c []ohlcv.Candle, _ []swing) (bool, int, int) {
			return ladderBottom(c), maxInt(0, len(c)-5), len(c) - 1
		}, c, nil, "bullish", spec.Confidence, "ladder bottom structure matched")
	}
	if strings.Contains(name, "concealing baby") {
		return adaptMatch(func(c []ohlcv.Candle, _ []swing) (bool, int, int) {
			return concealingBabySwallow(c), maxInt(0, len(c)-4), len(c) - 1
		}, c, nil, "bullish", spec.Confidence, "concealing baby swallow structure matched")
	}
	return generatedMatch{}
}

func directionOK(direction string, bullish, bearish bool) bool {
	switch direction {
	case "bullish":
		return bullish
	case "bearish":
		return bearish
	default:
		return bullish || bearish
	}
}

func matchGap(c []ohlcv.Candle, direction string, confidence float64, evidence string) generatedMatch {
	if len(c) < 2 {
		return generatedMatch{}
	}
	up := c[len(c)-1].EffectiveLow() > c[len(c)-2].EffectiveHigh()
	down := c[len(c)-1].EffectiveHigh() < c[len(c)-2].EffectiveLow()
	return generatedMatch{ok: directionOK(direction, up, down), start: len(c) - 2, end: len(c) - 1, direction: direction, confidence: confidence, evidence: evidence}
}

func matchGapDirectional(c []ohlcv.Candle, up bool, confidence float64, evidence string) generatedMatch {
	direction := "bullish"
	if !up {
		direction = "bearish"
	}
	m := matchGap(c, direction, confidence, evidence)
	m.direction = direction
	return m
}

func matchCompression(c []ohlcv.Candle, direction string, confidence float64) generatedMatch {
	if len(c) < 40 {
		return generatedMatch{}
	}
	early := rangeWidth(c[len(c)-40 : len(c)-20])
	late := rangeWidth(c[len(c)-20:])
	return generatedMatch{ok: late < early*0.75, start: len(c) - 40, end: len(c) - 1, direction: direction, confidence: confidence, evidence: "recent range contracted versus prior range"}
}

func matchExpansion(c []ohlcv.Candle, direction string, confidence float64) generatedMatch {
	if len(c) < 40 {
		return generatedMatch{}
	}
	early := rangeWidth(c[len(c)-40 : len(c)-20])
	late := rangeWidth(c[len(c)-20:])
	return generatedMatch{ok: late > early*1.25, start: len(c) - 40, end: len(c) - 1, direction: direction, confidence: confidence, evidence: "recent range expanded versus prior range"}
}

func matchVolumePattern(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 20 {
		return generatedMatch{}
	}
	name := normalizePatternText(spec.Name)
	last := c[len(c)-1]
	lastShape := shape(last)
	avgVol := averageVolume(c[len(c)-20:])
	relVol := mathutil.SafeDiv(last.EffectiveVolume(), avgVol)
	rangeRel := relativeRange(c)
	closeLocation := mathutil.SafeDiv(last.EffectiveClose()-last.EffectiveLow(), math.Max(last.EffectiveHigh()-last.EffectiveLow(), mathutil.Epsilon))
	ok := relVol > 1.25
	evidence := "volume expanded above recent average"
	direction := spec.Direction
	switch {
	case strings.Contains(name, "no demand"):
		ok = relVol < 0.70 && rangeRel < 1.10 && lastShape.bullish && trend(c, len(c)-1, 10) > 0
		direction = "bearish"
		evidence = "low-volume up bar after a recent advance matched no-demand behavior"
	case strings.Contains(name, "no supply"):
		ok = relVol < 0.70 && rangeRel < 1.10 && lastShape.bearish && trend(c, len(c)-1, 10) < 0
		direction = "bullish"
		evidence = "low-volume down bar after a recent decline matched no-supply behavior"
	case strings.Contains(name, "dry"):
		ok = relVol < 0.70
		evidence = "volume dried up below recent average"
	case strings.Contains(name, "absorption"):
		ok = relVol > 1.35 && rangeRel < 0.90
		evidence = "high volume produced a narrow spread, matching absorption behavior"
	case strings.Contains(name, "effort"):
		ok = relVol > 1.35 && (rangeRel < 0.85 || matchVolumeDivergence(c))
		evidence = "large effort in volume produced limited price result"
	case strings.Contains(name, "accumulation volume"):
		ok = relVol > 1.15 && closeLocation >= 0.55 && trend(c, len(c)-1, 20) <= 0.04
		direction = "bullish"
		evidence = "volume expanded while price held the upper part of a base"
	case strings.Contains(name, "distribution volume"):
		ok = relVol > 1.15 && closeLocation <= 0.45 && trend(c, len(c)-1, 20) >= -0.04
		direction = "bearish"
		evidence = "volume expanded while price closed weak in a distribution range"
	case strings.Contains(name, "buying climax"):
		ok = relVol > 1.45 && rangeRel > 1.25 && lastShape.bullish && closeLocation >= 0.55 && trend(c, len(c)-1, 15) > 0
		direction = "bearish"
		evidence = "wide high-volume up bar after an advance matched buying climax behavior"
	case strings.Contains(name, "selling climax"):
		ok = relVol > 1.45 && rangeRel > 1.25 && lastShape.bearish && closeLocation <= 0.45 && trend(c, len(c)-1, 15) < 0
		direction = "bullish"
		evidence = "wide high-volume down bar after a decline matched selling climax behavior"
	case strings.Contains(name, "stopping"):
		ok = relVol > 1.35 && rangeRel > 1.10 && lastShape.bearish && closeLocation > 0.35 && trend(c, len(c)-1, 15) < 0
		direction = "bullish"
		evidence = "high-volume down bar closing off the low after weakness matched stopping volume"
	case strings.Contains(name, "climax") || strings.Contains(name, "wide spread"):
		ok = relVol > 1.35 && rangeRel > 1.20
		evidence = "wide high-volume bar matched"
	}
	if strings.Contains(name, "narrow spread") {
		ok = rangeRel < 0.65
		evidence = "narrow spread bar matched"
	}
	if strings.Contains(name, "divergence") {
		ok = matchVolumeDivergence(c)
		evidence = "price and volume diverged over recent window"
	}
	if direction == "neutral" {
		if last.EffectiveClose() > last.EffectiveOpen() {
			direction = "bullish"
		}
		if last.EffectiveClose() < last.EffectiveOpen() {
			direction = "bearish"
		}
	}
	return generatedMatch{ok: ok, start: len(c) - 20, end: len(c) - 1, direction: direction, confidence: spec.Confidence, evidence: evidence}
}

func matchVolumeDivergence(c []ohlcv.Candle) bool {
	if len(c) < 30 {
		return false
	}
	priceSlope := trend(c, len(c)-1, 20)
	volumes := make([]float64, 20)
	for i, candle := range c[len(c)-20:] {
		volumes[i] = candle.EffectiveVolume()
	}
	volSlope := mathutil.SafeDiv(volumes[len(volumes)-1]-volumes[0], math.Max(volumes[0], mathutil.Epsilon))
	return (priceSlope > 0.03 && volSlope < -0.15) || (priceSlope < -0.03 && volSlope > 0.15)
}

type marketProfileStats struct {
	high               float64
	low                float64
	width              float64
	poc                float64
	valueAreaHigh      float64
	valueAreaLow       float64
	visibleRange       float64
	volumeBins         []float64
	tpoBins            []int
	pocIdx             int
	highVolumeNodeIdx  int
	lowVolumeNodeIdx   int
	upperVolume        float64
	lowerVolume        float64
	totalVolume        float64
	initialBalanceHigh float64
	initialBalanceLow  float64
}

func matchMarketProfilePattern(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 12 {
		return generatedMatch{}
	}
	profile := buildMarketProfileStats(c, 24)
	if profile.width <= 0 || profile.totalVolume <= 0 {
		return generatedMatch{}
	}
	name := normalizePatternText(spec.Name)
	lastCandle := c[len(c)-1]
	prevClose := c[maxInt(0, len(c)-2)].EffectiveClose()
	lastClose := lastCandle.EffectiveClose()
	direction := spec.Direction
	ok := false
	evidence := "OHLCV-derived TPO/volume profile matched the named auction structure"
	switch {
	case strings.Contains(name, "value area breakout"):
		bull := prevClose <= profile.valueAreaHigh && lastClose > profile.valueAreaHigh
		bear := prevClose >= profile.valueAreaLow && lastClose < profile.valueAreaLow
		ok = directionOK(direction, bull, bear)
		direction = resolvedDirection(direction, bull, bear)
		evidence = "close broke away from the derived value area"
	case strings.Contains(name, "value area rejection"):
		bull := lastCandle.EffectiveLow() < profile.valueAreaLow && lastClose > profile.valueAreaLow
		bear := lastCandle.EffectiveHigh() > profile.valueAreaHigh && lastClose < profile.valueAreaHigh
		ok = directionOK(direction, bull, bear)
		direction = resolvedDirection(direction, bull, bear)
		evidence = "price rejected outside the derived value area and returned inside"
	case strings.Contains(name, "initial balance breakout"):
		bull := lastClose > profile.initialBalanceHigh
		bear := lastClose < profile.initialBalanceLow
		ok = directionOK(direction, bull, bear)
		direction = resolvedDirection(direction, bull, bear)
		evidence = "close broke the derived initial-balance range"
	case strings.Contains(name, "initial balance failure"):
		bull := lastCandle.EffectiveLow() < profile.initialBalanceLow && lastClose > profile.initialBalanceLow
		bear := lastCandle.EffectiveHigh() > profile.initialBalanceHigh && lastClose < profile.initialBalanceHigh
		ok = directionOK(direction, bull, bear)
		direction = resolvedDirection(direction, bull, bear)
		evidence = "initial-balance break failed back inside the range"
	case strings.Contains(name, "d shape"):
		ok = profileBalanced(profile)
		direction = "neutral"
		evidence = "balanced D-shaped volume distribution is visible"
	case strings.Contains(name, "p shape"):
		ok = profile.upperVolume > profile.lowerVolume*1.25 && lastClose >= profile.poc
		direction = "bullish"
		evidence = "upper-half volume bulge forms a P-shaped profile"
	case strings.Contains(name, "b shape"):
		ok = profile.lowerVolume > profile.upperVolume*1.25 && lastClose <= profile.poc
		direction = "bearish"
		evidence = "lower-half volume bulge forms a b-shaped profile"
	case strings.Contains(name, "double distribution"):
		ok = profileDoubleDistribution(profile)
		direction = "neutral"
		evidence = "two high-volume distributions separated by a low-volume valley are visible"
	case strings.Contains(name, "single print"):
		ok = profileHasSinglePrint(profile)
		direction = "neutral"
		evidence = "single-print TPO area is visible in the derived profile"
	case strings.Contains(name, "poor high"):
		ok = profilePoorHigh(c, profile)
		direction = "bearish"
		evidence = "poor high without clear excess is visible"
	case strings.Contains(name, "poor low"):
		ok = profilePoorLow(c, profile)
		direction = "bullish"
		evidence = "poor low without clear excess is visible"
	case strings.Contains(name, "excess high"):
		ok = profileExcessHigh(c, profile)
		direction = "bearish"
		evidence = "excess high tail and rejection are visible"
	case strings.Contains(name, "excess low"):
		ok = profileExcessLow(c, profile)
		direction = "bullish"
		evidence = "excess low tail and rejection are visible"
	case strings.Contains(name, "naked poc"), strings.Contains(name, "virgin point of control"):
		ok = profileNakedPOC(c, profile)
		direction = "neutral"
		evidence = "prior high-volume POC remains effectively untested and nearby"
	case strings.Contains(name, "high volume node"), strings.Contains(name, "volume node"):
		ok = profileNearBin(lastClose, profile, profile.highVolumeNodeIdx, 1.5)
		evidence = "price is interacting with a high-volume node"
	case strings.Contains(name, "low volume node"):
		ok = profileNearBin(lastClose, profile, profile.lowVolumeNodeIdx, 1.5)
		evidence = "price is interacting with a low-volume node"
	default:
		ok = profileBalanced(profile) || profileDoubleDistribution(profile) || near(lastClose, profile.poc, 0.015)
		evidence = "derived auction profile structure is active"
	}
	if !ok {
		return generatedMatch{}
	}
	return generatedMatch{ok: true, start: 0, end: len(c) - 1, direction: direction, confidence: spec.Confidence, evidence: evidence}
}

func buildMarketProfileStats(c []ohlcv.Candle, bins int) marketProfileStats {
	high := highest(c)
	low := lowest(c)
	width := mathutil.SafeDiv(high-low, float64(bins))
	profile := marketProfileStats{
		high:              high,
		low:               low,
		width:             width,
		visibleRange:      high - low,
		volumeBins:        make([]float64, bins),
		tpoBins:           make([]int, bins),
		lowVolumeNodeIdx:  -1,
		highVolumeNodeIdx: -1,
	}
	if len(c) == 0 || bins <= 0 || width <= 0 {
		return profile
	}
	for _, candle := range c {
		start := profilePriceBin(profile, candle.EffectiveLow())
		end := profilePriceBin(profile, candle.EffectiveHigh())
		if end < start {
			start, end = end, start
		}
		covered := float64(end - start + 1)
		share := mathutil.SafeDiv(candle.EffectiveVolume(), covered)
		for i := start; i <= end; i++ {
			profile.volumeBins[i] += share
			profile.tpoBins[i]++
			profile.totalVolume += share
		}
	}
	profile.pocIdx = maxVolumeBin(profile.volumeBins)
	profile.poc = profileBinPrice(profile, profile.pocIdx)
	valIdx, vahIdx := profileValueArea(profile)
	profile.valueAreaLow = profileBinPrice(profile, valIdx)
	profile.valueAreaHigh = profileBinPrice(profile, vahIdx)
	mid := bins / 2
	for i, volume := range profile.volumeBins {
		if i >= mid {
			profile.upperVolume += volume
		} else {
			profile.lowerVolume += volume
		}
		if profile.highVolumeNodeIdx < 0 || volume > profile.volumeBins[profile.highVolumeNodeIdx] {
			profile.highVolumeNodeIdx = i
		}
		if volume > 0 && (profile.lowVolumeNodeIdx < 0 || volume < profile.volumeBins[profile.lowVolumeNodeIdx]) {
			profile.lowVolumeNodeIdx = i
		}
	}
	ibCount := maxInt(2, len(c)/5)
	profile.initialBalanceHigh = c[0].EffectiveHigh()
	profile.initialBalanceLow = c[0].EffectiveLow()
	for _, candle := range c[:ibCount] {
		profile.initialBalanceHigh = math.Max(profile.initialBalanceHigh, candle.EffectiveHigh())
		profile.initialBalanceLow = math.Min(profile.initialBalanceLow, candle.EffectiveLow())
	}
	return profile
}

func profilePriceBin(profile marketProfileStats, price float64) int {
	if profile.width <= 0 || len(profile.volumeBins) == 0 {
		return 0
	}
	return int(mathutil.Clamp(math.Floor((price-profile.low)/profile.width), 0, float64(len(profile.volumeBins)-1)))
}

func profileBinPrice(profile marketProfileStats, idx int) float64 {
	if len(profile.volumeBins) == 0 {
		return 0
	}
	idx = int(mathutil.Clamp(float64(idx), 0, float64(len(profile.volumeBins)-1)))
	return profile.low + (float64(idx)+0.5)*profile.width
}

func maxVolumeBin(values []float64) int {
	best := 0
	for i := range values {
		if values[i] > values[best] {
			best = i
		}
	}
	return best
}

func profileValueArea(profile marketProfileStats) (int, int) {
	lowIdx, highIdx := profile.pocIdx, profile.pocIdx
	sum := profile.volumeBins[profile.pocIdx]
	target := profile.totalVolume * 0.70
	for sum < target && (lowIdx > 0 || highIdx < len(profile.volumeBins)-1) {
		left := -1.0
		right := -1.0
		if lowIdx > 0 {
			left = profile.volumeBins[lowIdx-1]
		}
		if highIdx < len(profile.volumeBins)-1 {
			right = profile.volumeBins[highIdx+1]
		}
		if right >= left {
			highIdx++
			sum += profile.volumeBins[highIdx]
		} else {
			lowIdx--
			sum += profile.volumeBins[lowIdx]
		}
	}
	return lowIdx, highIdx
}

func profileBalanced(profile marketProfileStats) bool {
	mid := (profile.high + profile.low) / 2
	centeredPOC := math.Abs(profile.poc-mid) <= profile.visibleRange*0.15
	balance := mathutil.SafeDiv(math.Abs(profile.upperVolume-profile.lowerVolume), math.Max(profile.upperVolume+profile.lowerVolume, mathutil.Epsilon))
	return centeredPOC && balance <= 0.25
}

func profileDoubleDistribution(profile marketProfileStats) bool {
	if len(profile.volumeBins) < 8 {
		return false
	}
	first, second := -1, -1
	for i, volume := range profile.volumeBins {
		if first < 0 || volume > profile.volumeBins[first] {
			second = first
			first = i
			continue
		}
		if second < 0 || volume > profile.volumeBins[second] {
			second = i
		}
	}
	if first < 0 || second < 0 || absInt(first-second) < 4 {
		return false
	}
	lo, hi := minInt(first, second), maxInt(first, second)
	valley := profile.volumeBins[lo+1]
	for i := lo + 1; i < hi; i++ {
		valley = math.Min(valley, profile.volumeBins[i])
	}
	return valley < math.Min(profile.volumeBins[first], profile.volumeBins[second])*0.45
}

func profileHasSinglePrint(profile marketProfileStats) bool {
	for i, tpo := range profile.tpoBins {
		if tpo == 1 && i > 0 && i < len(profile.tpoBins)-1 {
			return true
		}
	}
	return false
}

func profilePoorHigh(c []ohlcv.Candle, profile marketProfileStats) bool {
	tolerance := math.Max(profile.width*1.5, priceDenominator(profile.high)*0.003)
	touches := 0
	for _, candle := range c {
		if math.Abs(candle.EffectiveHigh()-profile.high) <= tolerance {
			touches++
		}
	}
	return touches >= 2 && !profileExcessHigh(c, profile)
}

func profilePoorLow(c []ohlcv.Candle, profile marketProfileStats) bool {
	tolerance := math.Max(profile.width*1.5, priceDenominator(profile.low)*0.003)
	touches := 0
	for _, candle := range c {
		if math.Abs(candle.EffectiveLow()-profile.low) <= tolerance {
			touches++
		}
	}
	return touches >= 2 && !profileExcessLow(c, profile)
}

func profileExcessHigh(c []ohlcv.Candle, profile marketProfileStats) bool {
	for _, candle := range c {
		if math.Abs(candle.EffectiveHigh()-profile.high) > profile.width*1.5 {
			continue
		}
		s := shape(candle)
		if s.upperWick >= s.rangeSize*0.45 && candle.EffectiveClose() < candle.EffectiveHigh()-s.rangeSize*0.35 {
			return true
		}
	}
	return false
}

func profileExcessLow(c []ohlcv.Candle, profile marketProfileStats) bool {
	for _, candle := range c {
		if math.Abs(candle.EffectiveLow()-profile.low) > profile.width*1.5 {
			continue
		}
		s := shape(candle)
		if s.lowerWick >= s.rangeSize*0.45 && candle.EffectiveClose() > candle.EffectiveLow()+s.rangeSize*0.35 {
			return true
		}
	}
	return false
}

func profileNakedPOC(c []ohlcv.Candle, profile marketProfileStats) bool {
	if len(c) < 20 {
		return false
	}
	poc := profile.poc
	last := c[len(c)-1].EffectiveClose()
	if !near(last, poc, 0.02) {
		return false
	}
	start := maxInt(0, len(c)-12)
	for _, candle := range c[start : len(c)-1] {
		if candle.EffectiveLow() <= poc && candle.EffectiveHigh() >= poc {
			return false
		}
	}
	return true
}

func profileNearBin(price float64, profile marketProfileStats, idx int, widthMultiplier float64) bool {
	if idx < 0 || idx >= len(profile.volumeBins) {
		return false
	}
	return math.Abs(price-profileBinPrice(profile, idx)) <= profile.width*widthMultiplier
}

type pointFigureColumn struct {
	direction int
	high      int
	low       int
	start     int
	end       int
}

type pointFigureChart struct {
	columns  []pointFigureColumn
	boxSize  float64
	origin   float64
	reversal int
}

func matchPointFigurePattern(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 10 {
		return generatedMatch{}
	}
	chart := buildPointFigureChart(c, 3)
	if len(chart.columns) < 2 || chart.boxSize <= 0 {
		return generatedMatch{}
	}
	name := normalizePatternText(spec.Name)
	bullDouble := pnfDoubleTopBuy(chart)
	bearDouble := pnfDoubleBottomSell(chart)
	bullTriple := pnfTripleTopBuy(chart)
	bearTriple := pnfTripleBottomSell(chart)
	bullCatapult := pnfCatapult(chart, true)
	bearCatapult := pnfCatapult(chart, false)
	direction := spec.Direction
	ok := false
	evidence := "OHLCV-derived Point & Figure columns matched the named setup"
	switch {
	case strings.Contains(name, "bullish catapult"):
		ok, direction, evidence = bullCatapult, "bullish", "bullish P&F catapult breakout is visible"
	case strings.Contains(name, "bearish catapult"):
		ok, direction, evidence = bearCatapult, "bearish", "bearish P&F catapult breakdown is visible"
	case strings.Contains(name, "double top buy"):
		ok, direction, evidence = bullDouble, "bullish", "P&F double-top buy signal is visible"
	case strings.Contains(name, "double bottom sell"):
		ok, direction, evidence = bearDouble, "bearish", "P&F double-bottom sell signal is visible"
	case strings.Contains(name, "triple top buy"), strings.Contains(name, "ascending triple top"), strings.Contains(name, "spread triple top"):
		ok, direction, evidence = bullTriple, "bullish", "P&F triple-top buy breakout is visible"
	case strings.Contains(name, "triple bottom sell"), strings.Contains(name, "descending triple bottom"), strings.Contains(name, "spread triple bottom"):
		ok, direction, evidence = bearTriple, "bearish", "P&F triple-bottom sell breakdown is visible"
	case strings.Contains(name, "high pole"):
		ok, direction, evidence = pnfHighPole(chart), "bearish", "P&F high-pole reversal is visible"
	case strings.Contains(name, "low pole"):
		ok, direction, evidence = pnfLowPole(chart), "bullish", "P&F low-pole reversal is visible"
	case strings.Contains(name, "triangle"):
		ok, direction, evidence = pnfTriangle(chart), "neutral", "P&F triangle congestion is visible"
	case strings.Contains(name, "fulcrum"):
		ok, evidence = pnfFulcrum(chart), "P&F fulcrum congestion and breakout are visible"
	default:
		ok = directionOK(direction, bullDouble || bullTriple || bullCatapult, bearDouble || bearTriple || bearCatapult)
		direction = resolvedDirection(direction, bullDouble || bullTriple || bullCatapult, bearDouble || bearTriple || bearCatapult)
	}
	if !ok {
		return generatedMatch{}
	}
	start := chart.columns[maxInt(0, len(chart.columns)-6)].start
	return generatedMatch{ok: true, start: start, end: len(c) - 1, direction: direction, confidence: spec.Confidence, evidence: evidence}
}

func buildPointFigureChart(c []ohlcv.Candle, reversal int) pointFigureChart {
	high := highest(c)
	low := lowest(c)
	boxSize := math.Max((high-low)/40, averageRange(c)*0.5)
	boxSize = math.Max(boxSize, priceDenominator(c[len(c)-1].EffectiveClose())*0.005)
	if boxSize <= 0 {
		return pointFigureChart{}
	}
	chart := pointFigureChart{boxSize: boxSize, origin: low, reversal: reversal}
	lastBox := pnfBox(chart, c[0].EffectiveClose())
	for i := 1; i < len(c); i++ {
		box := pnfBox(chart, c[i].EffectiveClose())
		if len(chart.columns) == 0 {
			move := box - lastBox
			if absInt(move) < 1 {
				continue
			}
			dir := 1
			if move < 0 {
				dir = -1
			}
			chart.columns = append(chart.columns, pointFigureColumn{
				direction: dir,
				high:      maxInt(lastBox, box),
				low:       minInt(lastBox, box),
				start:     0,
				end:       i,
			})
			lastBox = box
			continue
		}
		current := &chart.columns[len(chart.columns)-1]
		if current.direction > 0 {
			if box > current.high {
				current.high = box
				current.end = i
			} else if current.high-box >= reversal {
				chart.columns = append(chart.columns, pointFigureColumn{direction: -1, high: current.high - 1, low: box, start: i, end: i})
			}
		} else {
			if box < current.low {
				current.low = box
				current.end = i
			} else if box-current.low >= reversal {
				chart.columns = append(chart.columns, pointFigureColumn{direction: 1, high: box, low: current.low + 1, start: i, end: i})
			}
		}
		lastBox = box
	}
	return chart
}

func pnfBox(chart pointFigureChart, price float64) int {
	return int(math.Round((price - chart.origin) / chart.boxSize))
}

func pnfDoubleTopBuy(chart pointFigureChart) bool {
	last := chart.columns[len(chart.columns)-1]
	prev, ok := previousPNFColumn(chart, len(chart.columns)-2, 1)
	return ok && last.direction > 0 && last.high > prev.high
}

func pnfDoubleBottomSell(chart pointFigureChart) bool {
	last := chart.columns[len(chart.columns)-1]
	prev, ok := previousPNFColumn(chart, len(chart.columns)-2, -1)
	return ok && last.direction < 0 && last.low < prev.low
}

func pnfTripleTopBuy(chart pointFigureChart) bool {
	last := chart.columns[len(chart.columns)-1]
	aIdx := pnfPreviousColumnIndex(chart, len(chart.columns)-2, 1)
	bIdx := pnfPreviousColumnIndex(chart, aIdx-1, 1)
	if aIdx < 0 || bIdx < 0 {
		return false
	}
	a, b := chart.columns[aIdx], chart.columns[bIdx]
	return last.direction > 0 && last.high > maxInt(a.high, b.high) && absInt(a.high-b.high) <= 2
}

func pnfTripleBottomSell(chart pointFigureChart) bool {
	last := chart.columns[len(chart.columns)-1]
	aIdx := pnfPreviousColumnIndex(chart, len(chart.columns)-2, -1)
	bIdx := pnfPreviousColumnIndex(chart, aIdx-1, -1)
	if aIdx < 0 || bIdx < 0 {
		return false
	}
	a, b := chart.columns[aIdx], chart.columns[bIdx]
	return last.direction < 0 && last.low < minInt(a.low, b.low) && absInt(a.low-b.low) <= 2
}

func pnfCatapult(chart pointFigureChart, bullish bool) bool {
	if bullish {
		if !pnfDoubleTopBuy(chart) {
			return false
		}
		prevIdx := pnfPreviousColumnIndex(chart, len(chart.columns)-2, 1)
		if prevIdx < 0 {
			return false
		}
		sub := pointFigureChart{columns: chart.columns[:prevIdx+1]}
		return len(sub.columns) >= 3 && pnfDoubleTopBuy(sub)
	}
	if !pnfDoubleBottomSell(chart) {
		return false
	}
	prevIdx := pnfPreviousColumnIndex(chart, len(chart.columns)-2, -1)
	if prevIdx < 0 {
		return false
	}
	sub := pointFigureChart{columns: chart.columns[:prevIdx+1]}
	return len(sub.columns) >= 3 && pnfDoubleBottomSell(sub)
}

func pnfHighPole(chart pointFigureChart) bool {
	if len(chart.columns) < 2 {
		return false
	}
	last := chart.columns[len(chart.columns)-1]
	prev := chart.columns[len(chart.columns)-2]
	height := prev.high - prev.low + 1
	retrace := prev.high - last.low
	return prev.direction > 0 && last.direction < 0 && height >= 6 && retrace*2 >= height
}

func pnfLowPole(chart pointFigureChart) bool {
	if len(chart.columns) < 2 {
		return false
	}
	last := chart.columns[len(chart.columns)-1]
	prev := chart.columns[len(chart.columns)-2]
	height := prev.high - prev.low + 1
	retrace := last.high - prev.low
	return prev.direction < 0 && last.direction > 0 && height >= 6 && retrace*2 >= height
}

func pnfTriangle(chart pointFigureChart) bool {
	if len(chart.columns) < 5 {
		return false
	}
	recent := chart.columns[len(chart.columns)-5:]
	xHighs := []int{}
	oLows := []int{}
	for _, col := range recent {
		if col.direction > 0 {
			xHighs = append(xHighs, col.high)
		} else {
			oLows = append(oLows, col.low)
		}
	}
	return len(xHighs) >= 2 && len(oLows) >= 2 && xHighs[len(xHighs)-1] <= xHighs[0] && oLows[len(oLows)-1] >= oLows[0]
}

func pnfFulcrum(chart pointFigureChart) bool {
	if len(chart.columns) < 7 {
		return false
	}
	recent := chart.columns[len(chart.columns)-7:]
	high, low := recent[0].high, recent[0].low
	for _, col := range recent[:len(recent)-1] {
		high = maxInt(high, col.high)
		low = minInt(low, col.low)
	}
	last := recent[len(recent)-1]
	congested := high-low <= 8
	breakout := last.high > high || last.low < low
	return congested && breakout
}

func previousPNFColumn(chart pointFigureChart, start, direction int) (pointFigureColumn, bool) {
	idx := pnfPreviousColumnIndex(chart, start, direction)
	if idx < 0 {
		return pointFigureColumn{}, false
	}
	return chart.columns[idx], true
}

func pnfPreviousColumnIndex(chart pointFigureChart, start, direction int) int {
	for i := start; i >= 0; i-- {
		if chart.columns[i].direction == direction {
			return i
		}
	}
	return -1
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func matchWyckoffComposite(c []ohlcv.Candle, s []swing, spec patternSpec) generatedMatch {
	bull, startBull, endBull := matchWyckoff(true)(c, s)
	bear, startBear, endBear := matchWyckoff(false)(c, s)
	start, end := startBull, endBull
	if bear && !bull {
		start, end = startBear, endBear
	}
	return directionalMatch(bull, bear, start, end, spec, "Wyckoff composite-operator range behavior matched")
}

func matchStructureBreak(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 21 {
		return generatedMatch{}
	}
	bull := breakout(c, true)
	bear := breakout(c, false)
	return directionalMatch(bull, bear, maxInt(0, len(c)-30), len(c)-1, spec, "close broke the latest market structure boundary")
}

func matchLiquiditySweep(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 21 {
		return generatedMatch{}
	}
	bull := sweep(c, false)
	bear := sweep(c, true)
	return directionalMatch(bull, bear, maxInt(0, len(c)-20), len(c)-1, spec, "liquidity sweep and reclaim/rejection matched")
}

func matchBreakRetest(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 35 {
		return generatedMatch{}
	}
	baseStart := len(c) - 35
	baseEnd := len(c) - 10
	base := c[baseStart:baseEnd]
	recent := c[baseEnd:]
	levelHigh := highest(base)
	levelLow := lowest(base)
	last := c[len(c)-1]
	brokeUp, brokeDown := false, false
	for _, candle := range recent[:len(recent)-1] {
		brokeUp = brokeUp || candle.EffectiveClose() > levelHigh
		brokeDown = brokeDown || candle.EffectiveClose() < levelLow
	}
	highBand := priceBand(levelHigh, 0.012)
	lowBand := priceBand(levelLow, 0.012)
	bull := brokeUp && last.EffectiveLow() <= levelHigh+highBand && last.EffectiveClose() > levelHigh
	bear := brokeDown && last.EffectiveHigh() >= levelLow-lowBand && last.EffectiveClose() < levelLow
	return directionalMatch(bull, bear, baseStart, len(c)-1, spec, "structure break was retested and held")
}

func matchPullback(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 40 {
		return generatedMatch{}
	}
	closes := closesFromCandles(c)
	sma20 := simpleMovingAverageSeries(closes, 20)
	lastIndex := len(c) - 1
	level := sma20[lastIndex]
	last := c[lastIndex]
	band := priceBand(level, 0.018)
	bull := trend(c, lastIndex, 40) > 0.05 && last.EffectiveLow() <= level+band && last.EffectiveClose() > level && !breakout(c, false)
	bear := trend(c, lastIndex, 40) < -0.05 && last.EffectiveHigh() >= level-band && last.EffectiveClose() < level && !breakout(c, true)
	return directionalMatch(bull, bear, maxInt(0, len(c)-40), lastIndex, spec, "trend pullback held near SMA20")
}

func matchFakey(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 3 {
		return generatedMatch{}
	}
	mother := shape(c[len(c)-3])
	inside := shape(c[len(c)-2])
	last := shape(c[len(c)-1])
	insideOK := inside.high <= mother.high && inside.low >= mother.low
	bull := insideOK && last.low < mother.low && last.close > mother.low
	bear := insideOK && last.high > mother.high && last.close < mother.high
	return directionalMatch(bull, bear, len(c)-3, len(c)-1, spec, "inside-bar false break returned inside the mother bar")
}

func matchMotherBar(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 3 {
		return generatedMatch{}
	}
	mother := shape(c[len(c)-3])
	firstInside := shape(c[len(c)-2])
	secondInside := shape(c[len(c)-1])
	ok := firstInside.high <= mother.high && firstInside.low >= mother.low && secondInside.high <= mother.high && secondInside.low >= mother.low
	return generatedMatch{ok: ok, start: len(c) - 3, end: len(c) - 1, direction: spec.Direction, confidence: spec.Confidence, evidence: "two latest candles remain inside the mother-bar range"}
}

func matchQuasimodo(c []ohlcv.Candle, s []swing, spec patternSpec) generatedMatch {
	if len(c) < 35 {
		return generatedMatch{}
	}
	base := c[len(c)-35 : len(c)-10]
	recent := c[len(c)-10:]
	baseHigh := highest(base)
	baseLow := lowest(base)
	sweptHigh, sweptLow := false, false
	for _, candle := range recent[:len(recent)-1] {
		sweptHigh = sweptHigh || candle.EffectiveHigh() > baseHigh
		sweptLow = sweptLow || candle.EffectiveLow() < baseLow
	}
	lastClose := c[len(c)-1].EffectiveClose()
	bear := sweptHigh && lastClose < baseLow
	bull := sweptLow && lastClose > baseHigh
	if !(bull || bear) {
		points := lastSwings(s, "", 5)
		if len(points) == 5 && alternatingSwings(points) {
			bull = swingKinds(points, "low", "high", "low", "high", "low") &&
				points[2].price < points[0].price &&
				points[3].price > points[1].price &&
				points[4].price > points[2].price
			bear = swingKinds(points, "high", "low", "high", "low", "high") &&
				points[2].price > points[0].price &&
				points[3].price < points[1].price &&
				points[4].price < points[2].price
		}
	}
	return directionalMatch(bull, bear, maxInt(0, len(c)-35), len(c)-1, spec, "Quasimodo over-and-under swing failure matched")
}

func matchRangeZone(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 20 {
		return generatedMatch{}
	}
	window := c[maxInt(0, len(c)-60):]
	low := lowest(window)
	high := highest(window)
	width := high - low
	if width <= mathutil.Epsilon {
		return generatedMatch{}
	}
	position := mathutil.SafeDiv(c[len(c)-1].EffectiveClose()-low, width)
	switch patternRuleForSpec(spec) {
	case rulePremiumZone:
		return directionalMatch(false, position >= 0.70, len(c)-len(window), len(c)-1, spec, "close is in the premium portion of the recent range")
	case ruleDiscountZone:
		return directionalMatch(position <= 0.30, false, len(c)-len(window), len(c)-1, spec, "close is in the discount portion of the recent range")
	default:
		ok := position >= 0.45 && position <= 0.55
		return generatedMatch{ok: ok, start: len(c) - len(window), end: len(c) - 1, direction: "neutral", confidence: spec.Confidence, evidence: "close is near the equilibrium midpoint of the recent range"}
	}
}

func matchMeanReversion(input ScannerInput, spec patternSpec) generatedMatch {
	c := input.Candles
	if len(c) < 30 {
		return generatedMatch{}
	}
	last := c[len(c)-1].EffectiveClose()
	prev := c[len(c)-2].EffectiveClose()
	snap := input.Indicators
	bull, bear := false, false
	if snap.BollingerUpper > snap.BollingerLower && snap.BollingerMiddle != 0 {
		bull = prev < snap.BollingerLower && last > snap.BollingerLower
		bear = prev > snap.BollingerUpper && last < snap.BollingerUpper
	} else if snap.SMA20 != 0 {
		band := priceBand(snap.SMA20, 0.05)
		bull = prev < snap.SMA20-band && last > prev
		bear = prev > snap.SMA20+band && last < prev
	}
	return directionalMatch(bull, bear, maxInt(0, len(c)-30), len(c)-1, spec, "price stretched away from the mean and started reverting")
}

func matchDeadCatBounce(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 45 {
		return generatedMatch{}
	}
	crash := trend(c, len(c)-11, 30) < -0.12
	bounce := trend(c, len(c)-1, 10) > 0.03
	stillWeak := trend(c, len(c)-1, 45) < -0.08
	last := shape(c[len(c)-1])
	bear := crash && bounce && stillWeak && (last.bearish || c[len(c)-1].EffectiveClose() < simpleMovingAverageSeries(closesFromCandles(c), 20)[len(c)-1])
	return directionalMatch(false, bear, len(c)-45, len(c)-1, spec, "weak rebound after a sharp decline failed below mean resistance")
}

func matchPocketPivot(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 15 {
		return generatedMatch{}
	}
	last := shape(c[len(c)-1])
	maxDownVolume := 0.0
	for _, candle := range c[len(c)-11 : len(c)-1] {
		if candle.EffectiveClose() < candle.EffectiveOpen() {
			maxDownVolume = math.Max(maxDownVolume, candle.EffectiveVolume())
		}
	}
	if maxDownVolume <= 0 {
		maxDownVolume = averageVolume(c[len(c)-11 : len(c)-1])
	}
	bull := last.bullish && c[len(c)-1].EffectiveVolume() > maxDownVolume && c[len(c)-1].EffectiveClose() > highest(c[len(c)-11:len(c)-1])
	return directionalMatch(bull, false, len(c)-11, len(c)-1, spec, "up candle volume exceeded recent down-volume and broke a short base")
}

func matchTrendExhaustion(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 25 {
		return generatedMatch{}
	}
	last := shape(c[len(c)-1])
	avgVol := averageVolume(c[len(c)-20:])
	relVol := mathutil.SafeDiv(c[len(c)-1].EffectiveVolume(), avgVol)
	volumeOK := avgVol <= 0 || relVol >= 1.15
	rangeOK := relativeRange(c) >= 1.15
	bear := trend(c, len(c)-1, 25) > 0.08 && last.upperWick >= math.Max(last.body*1.8, last.rangeSize*0.35) && rangeOK && volumeOK
	bull := trend(c, len(c)-1, 25) < -0.08 && last.lowerWick >= math.Max(last.body*1.8, last.rangeSize*0.35) && rangeOK && volumeOK
	return directionalMatch(bull, bear, maxInt(0, len(c)-25), len(c)-1, spec, "extended trend printed an exhaustion wick with expanded range/volume")
}

func matchStairStep(c []ohlcv.Candle, s []swing, spec patternSpec) generatedMatch {
	if len(c) < 30 {
		return generatedMatch{}
	}
	hs := lastSwings(s, "high", 3)
	ls := lastSwings(s, "low", 3)
	bull, bear := false, false
	if len(hs) == 3 && len(ls) == 3 {
		bull = hs[0].price < hs[1].price && hs[1].price < hs[2].price && ls[0].price < ls[1].price && ls[1].price < ls[2].price
		bear = hs[0].price > hs[1].price && hs[1].price > hs[2].price && ls[0].price > ls[1].price && ls[1].price > ls[2].price
	}
	return directionalMatch(bull, bear, maxInt(0, len(c)-40), len(c)-1, spec, "successive swing highs and lows form a stair-step trend")
}

func matchSecondaryTest(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 30 {
		return generatedMatch{}
	}
	prev := c[len(c)-30 : len(c)-1]
	last := c[len(c)-1]
	avgVol := averageVolume(prev)
	relVol := mathutil.SafeDiv(last.EffectiveVolume(), avgVol)
	closeLocation := mathutil.SafeDiv(last.EffectiveClose()-last.EffectiveLow(), math.Max(last.EffectiveHigh()-last.EffectiveLow(), mathutil.Epsilon))
	lowRetest := near(last.EffectiveLow(), lowest(prev), 0.02) && relVol <= 1.10 && closeLocation > 0.45
	highRetest := near(last.EffectiveHigh(), highest(prev), 0.02) && relVol <= 1.10 && closeLocation < 0.55
	return directionalMatch(lowRetest, highRetest, len(c)-30, len(c)-1, spec, "secondary test retested a prior range extreme on controlled volume")
}

func matchTrendlineBreak(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 35 {
		return generatedMatch{}
	}
	bull := trend(c, len(c)-2, 30) < -0.03 && breakout(c, true)
	bear := trend(c, len(c)-2, 30) > 0.03 && breakout(c, false)
	if !(bull || bear) {
		bull = breakout(c, true)
		bear = breakout(c, false)
	}
	return directionalMatch(bull, bear, maxInt(0, len(c)-35), len(c)-1, spec, "close broke the prevailing trendline/structure boundary")
}

func matchRoundedReversal(c []ohlcv.Candle, s []swing, spec patternSpec) generatedMatch {
	bull, startBull, endBull := matchRounding(false)(c, s)
	bear, startBear, endBear := matchRounding(true)(c, s)
	start, end := startBull, endBull
	if bear && !bull {
		start, end = startBear, endBear
	}
	return directionalMatch(bull, bear, start, end, spec, "rounded reversal arc matched")
}

func matchWedgeEither(c []ohlcv.Candle, s []swing, spec patternSpec) generatedMatch {
	bear, startBear, endBear := matchWedge(true)(c, s)
	bull, startBull, endBull := matchWedge(false)(c, s)
	start, end := startBull, endBull
	if bear && !bull {
		start, end = startBear, endBear
	}
	return directionalMatch(bull, bear, start, end, spec, "rising or falling wedge compression matched")
}

func matchAMD(c []ohlcv.Candle, spec patternSpec) generatedMatch {
	if len(c) < 45 {
		return generatedMatch{}
	}
	start := len(c) - 45
	base := c[start : len(c)-15]
	recent := c[len(c)-15:]
	baseHigh := highest(base)
	baseLow := lowest(base)
	rangeOK := rangeWidth(base) <= 0.12
	sweptHigh, sweptLow := false, false
	for _, candle := range recent[:len(recent)-1] {
		sweptHigh = sweptHigh || candle.EffectiveHigh() > baseHigh
		sweptLow = sweptLow || candle.EffectiveLow() < baseLow
	}
	lastClose := c[len(c)-1].EffectiveClose()
	bull := rangeOK && sweptLow && lastClose > baseHigh
	bear := rangeOK && sweptHigh && lastClose < baseLow
	return directionalMatch(bull, bear, start, len(c)-1, spec, "range accumulation was swept and followed by displacement")
}

func directionalMatch(bullish, bearish bool, start, end int, spec patternSpec, evidence string) generatedMatch {
	if !directionOK(spec.Direction, bullish, bearish) {
		return generatedMatch{}
	}
	direction := resolvedDirection(spec.Direction, bullish, bearish)
	if evidence == "" {
		evidence = spec.Evidence
	}
	return generatedMatch{ok: true, start: start, end: end, direction: direction, confidence: spec.Confidence, evidence: evidence}
}

func resolvedDirection(requested string, bullish, bearish bool) string {
	if requested == "bullish" && bullish {
		return "bullish"
	}
	if requested == "bearish" && bearish {
		return "bearish"
	}
	if bullish && !bearish {
		return "bullish"
	}
	if bearish && !bullish {
		return "bearish"
	}
	if bullish {
		return "bullish"
	}
	if bearish {
		return "bearish"
	}
	if requested != "" {
		return requested
	}
	return "neutral"
}

func priceBand(level, pct float64) float64 {
	return math.Max(math.Abs(level)*pct, mathutil.Epsilon)
}

func matchTrend(c []ohlcv.Candle, up bool, confidence float64, evidence string) generatedMatch {
	if len(c) < 30 {
		return generatedMatch{}
	}
	slope := trend(c, len(c)-1, 30)
	if up {
		return generatedMatch{ok: slope > 0.04, start: len(c) - 30, end: len(c) - 1, direction: "bullish", confidence: confidence, evidence: evidence}
	}
	return generatedMatch{ok: slope < -0.04, start: len(c) - 30, end: len(c) - 1, direction: "bearish", confidence: confidence, evidence: evidence}
}

func matchSideways(c []ohlcv.Candle, confidence float64) generatedMatch {
	if len(c) < 40 {
		return generatedMatch{}
	}
	return generatedMatch{ok: math.Abs(trend(c, len(c)-1, 40)) < 0.02, start: len(c) - 40, end: len(c) - 1, direction: "neutral", confidence: confidence, evidence: "recent trend slope is flat"}
}

func matchIndicatorPattern(input ScannerInput, spec patternSpec) generatedMatch {
	c := input.Candles
	if len(c) < 30 {
		return generatedMatch{}
	}
	name := normalizePatternText(spec.Name)
	last := c[len(c)-1].EffectiveClose()
	snap := input.Indicators
	ok := false
	direction := spec.Direction
	evidence := "indicator condition matched"
	closes := closesFromCandles(c)
	if strings.Contains(name, "rsi") {
		rsi := indicators.RSISeries(closes, 14)
		ok = false
		if strings.Contains(name, "bullish") || strings.Contains(name, "positive") || strings.Contains(name, "bottom") {
			direction = "bullish"
		}
		if strings.Contains(name, "bearish") || strings.Contains(name, "negative") || strings.Contains(name, "top") {
			direction = "bearish"
		}
		if strings.Contains(name, "divergence") {
			if strings.Contains(name, "hidden") {
				ok = matchHiddenOscillatorDivergence(closes, rsi, direction)
			} else {
				ok = matchOscillatorDivergence(closes, rsi, direction)
			}
			evidence = "RSI price/oscillator divergence matched on recent swing points"
		} else if strings.Contains(name, "overbought") || strings.Contains(name, "top") {
			ok = snap.RSI14 >= 70
			evidence = "RSI is in overbought territory"
		} else if strings.Contains(name, "oversold") || strings.Contains(name, "bottom") {
			ok = snap.RSI14 <= 30
			evidence = "RSI is in oversold territory"
		} else {
			ok = directionOK(direction, snap.RSI14 > 55, snap.RSI14 < 45)
			evidence = "RSI momentum condition matched"
		}
	}
	if strings.Contains(name, "macd") {
		macdLine, signalLine := macdSeries(closes)
		macdBull := len(macdLine) > 0 && (snap.MACD > snap.MACDSignal || snap.MACDHistogram > 0)
		macdBear := len(macdLine) > 0 && (snap.MACD < snap.MACDSignal || snap.MACDHistogram < 0)
		ok = directionOK(direction, macdBull, macdBear)
		if strings.Contains(name, "divergence") {
			if strings.Contains(name, "hidden") {
				ok = matchHiddenOscillatorDivergence(closes, macdLine, direction)
			} else {
				ok = matchOscillatorDivergence(closes, macdLine, direction)
			}
			evidence = "MACD price/oscillator divergence matched on recent swing points"
		} else if strings.Contains(name, "crossover") || strings.Contains(name, "cross over") {
			ok = directionOK(direction, crossedAbove(macdLine, signalLine), crossedBelow(macdLine, signalLine))
			evidence = "MACD crossed its signal line on the latest bar"
		} else if strings.Contains(name, "zero") {
			ok = directionOK(direction, crossedLevel(macdLine, 0, true), crossedLevel(macdLine, 0, false))
			evidence = "MACD crossed the zero line on the latest bar"
		} else {
			evidence = "MACD momentum condition matched"
		}
	}
	if strings.Contains(name, "golden cross") {
		ok = movingAverageCrossed(closes, 50, 200, true)
		direction = "bullish"
		evidence = "SMA50 crossed above SMA200 on the latest bar"
	}
	if strings.Contains(name, "death cross") {
		ok = movingAverageCrossed(closes, 50, 200, false)
		direction = "bearish"
		evidence = "SMA50 crossed below SMA200 on the latest bar"
	}
	if movingAveragePatternName(name) {
		ok, direction, evidence = matchMovingAverageIndicator(name, direction, last, snap, closes)
	}
	if strings.Contains(name, "bollinger") || strings.Contains(name, "m top") || strings.Contains(name, "w bottom") || strings.Contains(name, "band") {
		width := mathutil.SafeDiv(snap.BollingerUpper-snap.BollingerLower, priceDenominator(snap.BollingerMiddle))
		if strings.Contains(name, "squeeze") {
			ok = width < 0.08
			evidence = "Bollinger band width is compressed"
		} else if strings.Contains(name, "fakeout") {
			prev := c[len(c)-2].EffectiveClose()
			bull := prev < snap.BollingerLower && last > snap.BollingerLower
			bear := prev > snap.BollingerUpper && last < snap.BollingerUpper
			ok = directionOK(direction, bull, bear)
			direction = resolvedDirection(direction, bull, bear)
			evidence = "Bollinger fakeout returned back inside the band"
		} else if strings.Contains(name, "expansion") || strings.Contains(name, "breakout") {
			ok = width > 0.12 || last > snap.BollingerUpper || last < snap.BollingerLower
			evidence = "Bollinger expansion or breakout matched"
		} else {
			ok = last > snap.BollingerUpper || last < snap.BollingerLower || math.Abs(last-snap.BollingerMiddle) < width*snap.BollingerMiddle
			evidence = "Bollinger band interaction matched"
		}
	}
	if strings.Contains(name, "ichimoku") || strings.Contains(name, "kumo") || strings.Contains(name, "tenkan") || strings.Contains(name, "kijun") || strings.Contains(name, "chikou") || strings.Contains(name, "cloud") || strings.Contains(name, "tk cross") {
		ok, direction, evidence = matchIchimokuIndicator(name, direction, snap)
	}
	return generatedMatch{ok: ok, start: len(c) - 30, end: len(c) - 1, direction: direction, confidence: spec.Confidence, evidence: evidence}
}

func movingAveragePatternName(name string) bool {
	return strings.Contains(name, "moving average") ||
		strings.Contains(name, "price ma") ||
		strings.HasPrefix(name, "ma ") ||
		strings.Contains(name, " ma ") ||
		strings.Contains(name, "ma ribbon")
}

func matchMovingAverageIndicator(name, direction string, last float64, snap ohlcv.IndicatorSnapshot, closes []float64) (bool, string, string) {
	sma20Series := simpleMovingAverageSeries(closes, 20)
	sma50Series := simpleMovingAverageSeries(closes, 50)
	sma20 := latestMovingAverage(snap.SMA20, sma20Series)
	sma50 := latestMovingAverage(snap.SMA50, sma50Series)
	bullOrdered, bearOrdered := movingAverageOrder(snap, sma20, sma50)
	spread := movingAverageSpread(sma20, sma50, snap.SMA100, snap.SMA200)
	evidence := "moving-average condition matched"
	switch {
	case strings.Contains(name, "compression") || strings.Contains(name, "squeeze"):
		return spread > 0 && spread <= 0.035, direction, "moving averages compressed into a tight ribbon"
	case strings.Contains(name, "expansion") || strings.Contains(name, "fan"):
		ok := spread >= 0.055 && directionOK(direction, bullOrdered, bearOrdered)
		return ok, resolvedDirection(direction, bullOrdered, bearOrdered), "moving averages expanded into a directional ribbon"
	case strings.Contains(name, "ribbon"):
		ok := directionOK(direction, bullOrdered, bearOrdered)
		return ok, resolvedDirection(direction, bullOrdered, bearOrdered), "moving-average ribbon ordering matched"
	case strings.Contains(name, "crossover") || strings.Contains(name, "cross"):
		bull := crossedAbove(sma20Series, sma50Series)
		bear := crossedBelow(sma20Series, sma50Series)
		return directionOK(direction, bull, bear), resolvedDirection(direction, bull, bear), "SMA20 crossed SMA50 on the latest bar"
	case strings.Contains(name, "breakout"):
		bull := sma20 != 0 && last > sma20 && crossedAbove(closes, sma20Series)
		return directionOK("bullish", bull, false), "bullish", "price crossed above SMA20 on the latest bar"
	case strings.Contains(name, "breakdown"):
		bear := sma20 != 0 && last < sma20 && crossedBelow(closes, sma20Series)
		return directionOK("bearish", false, bear), "bearish", "price crossed below SMA20 on the latest bar"
	case strings.Contains(name, "pullback"):
		bull := sma20 != 0 && last >= sma20 && bullOrdered
		bear := sma20 != 0 && last <= sma20 && bearOrdered
		return directionOK(direction, bull, bear), resolvedDirection(direction, bull, bear), "price held a moving-average pullback"
	default:
		bull := sma20 != 0 && sma50 != 0 && last > sma20 && sma20 > sma50
		bear := sma20 != 0 && sma50 != 0 && last < sma20 && sma20 < sma50
		return directionOK(direction, bull, bear), resolvedDirection(direction, bull, bear), evidence
	}
}

func latestMovingAverage(snapshotValue float64, series []float64) float64 {
	if snapshotValue != 0 {
		return snapshotValue
	}
	if len(series) == 0 {
		return 0
	}
	return series[len(series)-1]
}

func movingAverageOrder(snap ohlcv.IndicatorSnapshot, sma20, sma50 float64) (bool, bool) {
	if sma20 == 0 || sma50 == 0 {
		return false, false
	}
	bull := sma20 > sma50
	bear := sma20 < sma50
	if snap.SMA100 != 0 {
		bull = bull && sma50 > snap.SMA100
		bear = bear && sma50 < snap.SMA100
	}
	if snap.SMA200 != 0 {
		anchor := snap.SMA100
		if anchor == 0 {
			anchor = sma50
		}
		bull = bull && anchor > snap.SMA200
		bear = bear && anchor < snap.SMA200
	}
	return bull, bear
}

func movingAverageSpread(values ...float64) float64 {
	filtered := make([]float64, 0, len(values))
	for _, value := range values {
		if value != 0 {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) < 2 {
		return 0
	}
	minimum := filtered[0]
	maximum := filtered[0]
	totalAbs := 0.0
	for _, value := range filtered {
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
		totalAbs += math.Abs(value)
	}
	denom := mathutil.SafeDiv(totalAbs, float64(len(filtered)))
	return mathutil.SafeDiv(maximum-minimum, math.Max(denom, mathutil.Epsilon))
}

func matchIchimokuIndicator(name, direction string, snap ohlcv.IndicatorSnapshot) (bool, string, string) {
	switch {
	case strings.Contains(name, "twist"):
		bull := snap.IchimokuKumoTwist > 0
		bear := snap.IchimokuKumoTwist < 0
		return directionOK(direction, bull, bear), resolvedDirection(direction, bull, bear), "Ichimoku Kumo twist matched"
	case strings.Contains(name, "tk cross") || strings.Contains(name, "tenkan kijun") || strings.Contains(name, "cross"):
		bull := snap.IchimokuTKCross > 0
		bear := snap.IchimokuTKCross < 0
		return directionOK(direction, bull, bear), resolvedDirection(direction, bull, bear), "Ichimoku Tenkan/Kijun cross matched"
	case strings.Contains(name, "breakout") || strings.Contains(name, "breakdown"):
		bull := snap.IchimokuPriceCloudBreakout > 0
		bear := snap.IchimokuPriceCloudBreakout < 0
		return directionOK(direction, bull, bear), resolvedDirection(direction, bull, bear), "price broke the Ichimoku cloud"
	case strings.Contains(name, "support"):
		bull := snap.IchimokuCloudTrend > 0
		return directionOK("bullish", bull, false), "bullish", "Ichimoku cloud support matched"
	case strings.Contains(name, "resistance"):
		bear := snap.IchimokuCloudTrend < 0
		return directionOK("bearish", false, bear), "bearish", "Ichimoku cloud resistance matched"
	default:
		bull := snap.IchimokuCloudTrend > 0 || snap.IchimokuPriceCloudBreakout > 0
		bear := snap.IchimokuCloudTrend < 0 || snap.IchimokuPriceCloudBreakout < 0
		return directionOK(direction, bull, bear), resolvedDirection(direction, bull, bear), "Ichimoku cloud trend condition matched"
	}
}

func matchOscillatorDivergence(price, osc []float64, direction string) bool {
	price, osc = alignedSeries(price, osc)
	if len(price) < 20 {
		return false
	}
	if direction == "bullish" {
		return oscillatorDiverged(price, osc, "low", false)
	}
	if direction == "bearish" {
		return oscillatorDiverged(price, osc, "high", false)
	}
	return oscillatorDiverged(price, osc, "low", false) || oscillatorDiverged(price, osc, "high", false)
}

func matchHiddenOscillatorDivergence(price, osc []float64, direction string) bool {
	price, osc = alignedSeries(price, osc)
	if len(price) < 20 {
		return false
	}
	if direction == "bullish" {
		return oscillatorDiverged(price, osc, "low", true)
	}
	if direction == "bearish" {
		return oscillatorDiverged(price, osc, "high", true)
	}
	return oscillatorDiverged(price, osc, "low", true) || oscillatorDiverged(price, osc, "high", true)
}

func oscillatorDiverged(price, osc []float64, pivotKind string, hidden bool) bool {
	pivots := lastPivotIndexes(price, pivotKind, 2, 80, 2)
	if len(pivots) < 2 {
		return false
	}
	first := pivots[0]
	second := pivots[1]
	switch pivotKind {
	case "low":
		if hidden {
			return price[second] > price[first] && osc[second] < osc[first]
		}
		return price[second] < price[first] && osc[second] > osc[first]
	case "high":
		if hidden {
			return price[second] < price[first] && osc[second] > osc[first]
		}
		return price[second] > price[first] && osc[second] < osc[first]
	default:
		return false
	}
}

func alignedSeries(a, b []float64) ([]float64, []float64) {
	if len(a) == len(b) {
		return a, b
	}
	n := minInt(len(a), len(b))
	if n <= 0 {
		return nil, nil
	}
	return a[len(a)-n:], b[len(b)-n:]
}

func lastPivotIndexes(values []float64, kind string, wing int, lookback int, count int) []int {
	if len(values) < wing*2+1 || count <= 0 {
		return nil
	}
	start := maxInt(wing, len(values)-lookback)
	end := len(values) - wing
	out := []int{}
	for i := end - 1; i >= start && len(out) < count; i-- {
		pivot := true
		for j := i - wing; j <= i+wing; j++ {
			if j == i {
				continue
			}
			if kind == "low" && values[i] > values[j] {
				pivot = false
				break
			}
			if kind == "high" && values[i] < values[j] {
				pivot = false
				break
			}
		}
		if pivot {
			out = append([]int{i}, out...)
		}
	}
	return out
}

func macdSeries(values []float64) ([]float64, []float64) {
	if len(values) == 0 {
		return nil, nil
	}
	fast := indicators.EMASeries(values, 12)
	slow := indicators.EMASeries(values, 26)
	line := make([]float64, len(values))
	for i := range values {
		line[i] = fast[i] - slow[i]
	}
	return line, indicators.EMASeries(line, 9)
}

func movingAverageCrossed(values []float64, fast, slow int, bullish bool) bool {
	if len(values) < slow+1 || fast <= 0 || slow <= 0 {
		return false
	}
	fastSeries := simpleMovingAverageSeries(values, fast)
	slowSeries := simpleMovingAverageSeries(values, slow)
	if bullish {
		return crossedAbove(fastSeries, slowSeries)
	}
	return crossedBelow(fastSeries, slowSeries)
}

func crossedAbove(left, right []float64) bool {
	left, right = alignedSeries(left, right)
	if len(left) < 2 {
		return false
	}
	last := len(left) - 1
	return left[last-1] <= right[last-1] && left[last] > right[last]
}

func crossedBelow(left, right []float64) bool {
	left, right = alignedSeries(left, right)
	if len(left) < 2 {
		return false
	}
	last := len(left) - 1
	return left[last-1] >= right[last-1] && left[last] < right[last]
}

func crossedLevel(values []float64, level float64, bullish bool) bool {
	if len(values) < 2 {
		return false
	}
	last := len(values) - 1
	if bullish {
		return values[last-1] <= level && values[last] > level
	}
	return values[last-1] >= level && values[last] < level
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
		window := i + 1
		if window > period {
			window = period
		}
		out[i] = mathutil.SafeDiv(sum, float64(window))
	}
	return out
}

func matchFibonacci(input ScannerInput, spec patternSpec) generatedMatch {
	c := input.Candles
	if len(c) < 20 {
		return generatedMatch{}
	}
	levels := input.Indicators.FibonacciLevels
	if len(levels) == 0 {
		return generatedMatch{}
	}
	last := c[len(c)-1].EffectiveClose()
	for _, level := range levels {
		if near(last, level, 0.015) {
			return generatedMatch{ok: true, start: len(c) - 20, end: len(c) - 1, direction: spec.Direction, confidence: spec.Confidence, evidence: "price is near a Fibonacci level"}
		}
	}
	return generatedMatch{}
}

func closesFromCandles(c []ohlcv.Candle) []float64 {
	out := make([]float64, len(c))
	for i, candle := range c {
		out[i] = candle.EffectiveClose()
	}
	return out
}

func normalizePatternText(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "=", " ")
	for strings.Contains(value, "  ") {
		value = strings.ReplaceAll(value, "  ", " ")
	}
	return strings.TrimSpace(value)
}
