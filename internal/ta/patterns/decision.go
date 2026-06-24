package patterns

import (
	"math"
	"sort"
	"strings"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

const (
	minActionableConfidence = 0.60
	minActionableRR         = 0.95
	maxActionablePatterns   = 12
	calibrationHorizonBars  = 10
	minCalibrationSamples   = 3
)

type patternPerformanceProfile struct {
	samples     int
	wins        int
	expectancyR float64
}

func resolvePatternSignals(input ScannerInput, raw []ohlcv.PatternResult) ([]ohlcv.PatternResult, []ohlcv.PatternResult) {
	if len(raw) == 0 {
		return nil, nil
	}
	profiles := buildPatternPerformanceProfiles(input.Candles, raw)
	context := directionalMarketContext(input)
	candidates := make([]ohlcv.PatternResult, len(raw))
	for i, candidate := range raw {
		candidates[i] = scorePatternCandidate(input, candidate, profiles, context)
	}
	resolveDirectionalConflicts(candidates)
	actionables := consolidateActionablePatterns(candidates)
	sortPatternResults(candidates)
	return actionables, candidates
}

func scorePatternCandidate(input ScannerInput, pattern ohlcv.PatternResult, profiles map[string]patternPerformanceProfile, context float64) ohlcv.PatternResult {
	pattern.RawConfidence = pattern.Confidence
	pattern.SignalGroup = patternSignalGroup(pattern)
	pattern.Resolution = "candidate"
	pattern.ConflictStatus = "none"
	pattern.Actionable = false
	pattern.BacktestReady = patternBacktestReady(pattern)

	profile, source := selectPatternPerformanceProfile(pattern, profiles)
	pattern.BacktestSampleSize = profile.samples
	pattern.BacktestWinRate = mathutil.SafeDiv(float64(profile.wins), float64(profile.samples))
	pattern.BacktestExpectancyR = profile.expectancyR
	pattern.CalibrationSource = source

	reliability := categoryReliability(pattern.Category)
	if profile.samples >= minCalibrationSamples {
		hitRate := pattern.BacktestWinRate
		expectancyComponent := mathutil.Clamp(0.50+profile.expectancyR*0.12, 0.30, 0.82)
		reliability = mathutil.Clamp(0.60*hitRate+0.40*expectancyComponent, 0.25, 0.85)
	}
	alignment := directionalAlignment(pattern.Direction, context)
	recency := patternRecencyScore(input.Candles, pattern)
	volumeBonus := 0.0
	if pattern.VolumeConfirmed {
		volumeBonus = 0.04
	}
	rrBonus := mathutil.Clamp((pattern.RiskRewardRatio-minActionableRR)/3.0, 0, 0.06)
	calibrated := pattern.RawConfidence*0.50 + reliability*0.28 + alignment*0.12 + recency*0.06 + volumeBonus + rrBonus
	if pattern.Direction == "neutral" {
		calibrated *= 0.92
	}
	pattern.CalibratedConfidence = mathutil.Clamp(calibrated, 0, 1)
	pattern.Confidence = pattern.CalibratedConfidence
	pattern.SignalScore = mathutil.Clamp(pattern.CalibratedConfidence*0.82+reliability*0.12+recency*0.06, 0, 1)

	last := len(input.Candles) - 1
	if last < 0 || pattern.EndIndex != last {
		pattern.RejectionReasons = appendPatternReason(pattern.RejectionReasons, "not_current_completed_pattern")
	}
	if pattern.Direction != "bullish" && pattern.Direction != "bearish" {
		pattern.RejectionReasons = appendPatternReason(pattern.RejectionReasons, "neutral_pattern_context_only")
	}
	if !pattern.Tradeable {
		pattern.RejectionReasons = appendPatternReason(pattern.RejectionReasons, "not_directional_trade_setup")
	}
	if !pattern.BacktestReady {
		pattern.RejectionReasons = appendPatternReason(pattern.RejectionReasons, "backtest_metadata_not_ready")
	}
	if pattern.CalibratedConfidence < minActionableConfidence {
		pattern.RejectionReasons = appendPatternReason(pattern.RejectionReasons, "calibrated_confidence_below_threshold")
	}
	return pattern
}

func buildPatternPerformanceProfiles(candles []ohlcv.Candle, candidates []ohlcv.PatternResult) map[string]patternPerformanceProfile {
	profiles := map[string]patternPerformanceProfile{}
	if len(candles) < calibrationHorizonBars+2 {
		return profiles
	}
	for _, pattern := range candidates {
		if pattern.Direction != "bullish" && pattern.Direction != "bearish" {
			continue
		}
		if pattern.EndIndex < 0 || pattern.EndIndex+calibrationHorizonBars >= len(candles) {
			continue
		}
		entry := candles[pattern.EndIndex].EffectiveClose()
		exit := candles[pattern.EndIndex+calibrationHorizonBars].EffectiveClose()
		atr := historicalPatternATR(candles, pattern.EndIndex)
		if entry <= 0 || exit <= 0 || atr <= 0 {
			continue
		}
		moveR := (exit - entry) / atr
		if pattern.Direction == "bearish" {
			moveR = (entry - exit) / atr
		}
		win := moveR >= 0.50
		for _, key := range []string{patternPerformanceKey(pattern), patternCategoryPerformanceKey(pattern)} {
			profile := profiles[key]
			profile.samples++
			if win {
				profile.wins++
			}
			profile.expectancyR += mathutil.Clamp(moveR, -2.5, 3.5)
			profiles[key] = profile
		}
	}
	for key, profile := range profiles {
		profile.expectancyR = mathutil.SafeDiv(profile.expectancyR, float64(profile.samples))
		profiles[key] = profile
	}
	return profiles
}

func selectPatternPerformanceProfile(pattern ohlcv.PatternResult, profiles map[string]patternPerformanceProfile) (patternPerformanceProfile, string) {
	if profile := profiles[patternPerformanceKey(pattern)]; profile.samples >= minCalibrationSamples {
		return profile, "pattern_group_walk_forward"
	}
	if profile := profiles[patternCategoryPerformanceKey(pattern)]; profile.samples >= minCalibrationSamples {
		return profile, "category_direction_walk_forward"
	}
	return patternPerformanceProfile{
		samples:     0,
		wins:        0,
		expectancyR: categoryReliability(pattern.Category) - 0.50,
	}, "category_prior"
}

func resolveDirectionalConflicts(candidates []ohlcv.PatternResult) {
	bullish := 0.0
	bearish := 0.0
	for i := range candidates {
		if !candidateCanEnterConflict(candidates[i]) {
			continue
		}
		if candidates[i].Direction == "bullish" {
			bullish += candidates[i].SignalScore
		}
		if candidates[i].Direction == "bearish" {
			bearish += candidates[i].SignalScore
		}
	}
	if bullish == 0 || bearish == 0 {
		for i := range candidates {
			if candidateCanEnterConflict(candidates[i]) {
				candidates[i].ConflictStatus = "no_conflict"
			}
		}
		return
	}
	diff := math.Abs(bullish - bearish)
	if diff < 0.18 || mathutil.SafeDiv(math.Max(bullish, bearish), math.Min(bullish, bearish)) < 1.18 {
		for i := range candidates {
			if candidateCanEnterConflict(candidates[i]) {
				candidates[i].ConflictStatus = "unresolved"
				candidates[i].RejectionReasons = appendPatternReason(candidates[i].RejectionReasons, "directional_conflict_unresolved")
			}
		}
		return
	}
	winner := "bullish"
	if bearish > bullish {
		winner = "bearish"
	}
	for i := range candidates {
		if !candidateCanEnterConflict(candidates[i]) {
			continue
		}
		if candidates[i].Direction == winner {
			candidates[i].ConflictStatus = "winner"
			continue
		}
		candidates[i].ConflictStatus = "loser"
		candidates[i].RejectionReasons = appendPatternReason(candidates[i].RejectionReasons, "directional_conflict_loser")
	}
}

func consolidateActionablePatterns(candidates []ohlcv.PatternResult) []ohlcv.PatternResult {
	groupMembers := map[string][]int{}
	for i := range candidates {
		if len(candidates[i].RejectionReasons) > 0 {
			continue
		}
		groupMembers[candidates[i].SignalGroup] = append(groupMembers[candidates[i].SignalGroup], i)
	}
	selected := make([]int, 0, len(groupMembers))
	for _, members := range groupMembers {
		best := members[0]
		for _, idx := range members[1:] {
			if betterActionableCandidate(candidates[idx], candidates[best]) {
				best = idx
			}
		}
		for _, idx := range members {
			if idx == best {
				continue
			}
			candidates[idx].RejectionReasons = appendPatternReason(candidates[idx].RejectionReasons, "lower_priority_alias_or_duplicate")
			candidates[best].ConsolidatedFrom = append(candidates[best].ConsolidatedFrom, candidates[idx].Name)
		}
		selected = append(selected, best)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return betterActionableCandidate(candidates[selected[i]], candidates[selected[j]])
	})
	if len(selected) > maxActionablePatterns {
		for _, idx := range selected[maxActionablePatterns:] {
			candidates[idx].RejectionReasons = appendPatternReason(candidates[idx].RejectionReasons, "actionable_signal_limit_exceeded")
		}
		selected = selected[:maxActionablePatterns]
	}
	actionables := make([]ohlcv.PatternResult, 0, len(selected))
	for _, idx := range selected {
		candidates[idx].Actionable = true
		candidates[idx].Resolution = "actionable_consolidated_signal"
		if candidates[idx].ConflictStatus == "none" {
			candidates[idx].ConflictStatus = "no_conflict"
		}
		sort.Strings(candidates[idx].ConsolidatedFrom)
		actionables = append(actionables, candidates[idx])
	}
	sortPatternResults(actionables)
	return actionables
}

func candidateCanEnterConflict(pattern ohlcv.PatternResult) bool {
	if pattern.Direction != "bullish" && pattern.Direction != "bearish" {
		return false
	}
	if hasPatternReason(pattern.RejectionReasons, "not_current_completed_pattern") ||
		hasPatternReason(pattern.RejectionReasons, "backtest_metadata_not_ready") ||
		hasPatternReason(pattern.RejectionReasons, "calibrated_confidence_below_threshold") {
		return false
	}
	return true
}

func betterActionableCandidate(candidate, current ohlcv.PatternResult) bool {
	if math.Abs(candidate.SignalScore-current.SignalScore) > 1e-9 {
		return candidate.SignalScore > current.SignalScore
	}
	if math.Abs(candidate.CalibratedConfidence-current.CalibratedConfidence) > 1e-9 {
		return candidate.CalibratedConfidence > current.CalibratedConfidence
	}
	if candidate.EndIndex != current.EndIndex {
		return candidate.EndIndex > current.EndIndex
	}
	return candidate.Name < current.Name
}

func sortPatternResults(patterns []ohlcv.PatternResult) {
	sort.SliceStable(patterns, func(i, j int) bool {
		if patterns[i].Actionable != patterns[j].Actionable {
			return patterns[i].Actionable
		}
		if math.Abs(patterns[i].SignalScore-patterns[j].SignalScore) > 1e-9 {
			return patterns[i].SignalScore > patterns[j].SignalScore
		}
		if patterns[i].EndIndex != patterns[j].EndIndex {
			return patterns[i].EndIndex > patterns[j].EndIndex
		}
		return patterns[i].Name < patterns[j].Name
	})
}

func patternBacktestReady(pattern ohlcv.PatternResult) bool {
	if pattern.Direction != "bullish" && pattern.Direction != "bearish" {
		return false
	}
	if !pattern.Tradeable || pattern.RiskRewardRatio < minActionableRR {
		return false
	}
	if pattern.EntryMin <= 0 || pattern.EntryMax <= 0 || pattern.StopLoss <= 0 || pattern.Target1 <= 0 || pattern.Target2 <= 0 || pattern.InvalidationLevel <= 0 {
		return false
	}
	if pattern.EntryMin > pattern.EntryMax {
		return false
	}
	entry := (pattern.EntryMin + pattern.EntryMax) / 2
	if pattern.Direction == "bullish" {
		return pattern.StopLoss < entry && pattern.Target1 > entry && pattern.Target2 > pattern.Target1 && pattern.InvalidationLevel == pattern.StopLoss
	}
	return pattern.StopLoss > entry && pattern.Target1 < entry && pattern.Target2 < pattern.Target1 && pattern.InvalidationLevel == pattern.StopLoss
}

func directionalMarketContext(input ScannerInput) float64 {
	candles := input.Candles
	if len(candles) == 0 {
		return 0
	}
	last := candles[len(candles)-1].EffectiveClose()
	snapshot := input.Indicators
	score := 0.0
	addPriceScore := func(level float64, weight float64) {
		if level <= 0 {
			return
		}
		if last > level {
			score += weight
		} else if last < level {
			score -= weight
		}
	}
	addPriceScore(snapshot.EMA20, 0.18)
	addPriceScore(snapshot.SMA50, 0.18)
	addPriceScore(snapshot.SMA200, 0.16)
	if snapshot.SMA50 > 0 && snapshot.SMA200 > 0 {
		if snapshot.SMA50 > snapshot.SMA200 {
			score += 0.14
		} else if snapshot.SMA50 < snapshot.SMA200 {
			score -= 0.14
		}
	}
	if snapshot.MACDHistogram > 0 {
		score += 0.12
	} else if snapshot.MACDHistogram < 0 {
		score -= 0.12
	}
	if snapshot.RSI14 > 55 {
		score += 0.10
	} else if snapshot.RSI14 < 45 {
		score -= 0.10
	}
	return mathutil.Clamp(score, -1, 1)
}

func directionalAlignment(direction string, context float64) float64 {
	switch direction {
	case "bullish":
		return mathutil.Clamp(0.50+context*0.35, 0.10, 0.90)
	case "bearish":
		return mathutil.Clamp(0.50-context*0.35, 0.10, 0.90)
	default:
		return 0.45
	}
}

func patternRecencyScore(candles []ohlcv.Candle, pattern ohlcv.PatternResult) float64 {
	if len(candles) == 0 || pattern.EndIndex < 0 {
		return 0
	}
	age := len(candles) - 1 - pattern.EndIndex
	if age <= 0 {
		return 1
	}
	return mathutil.Clamp(1.0-float64(age)*0.08, 0.05, 1)
}

func categoryReliability(category string) float64 {
	switch strings.TrimSpace(category) {
	case "candlestick":
		return 0.58
	case "chart", "classic_chart", "trend_channel":
		return 0.55
	case "price_action", "volume", "wyckoff":
		return 0.53
	case "market_profile", "point_and_figure", "indicator":
		return 0.51
	case "harmonic", "elliott_wave", "fibonacci":
		return 0.48
	default:
		return 0.46
	}
}

func patternPerformanceKey(pattern ohlcv.PatternResult) string {
	return pattern.SignalGroup + "|" + pattern.Direction
}

func patternCategoryPerformanceKey(pattern ohlcv.PatternResult) string {
	return strings.TrimSpace(pattern.Category) + "|" + pattern.Direction
}

func patternSignalGroup(pattern ohlcv.PatternResult) string {
	name := normalizePatternText(pattern.Name)
	family := semanticPatternFamily(name)
	return strings.Join([]string{strings.TrimSpace(pattern.Category), pattern.Direction, family}, "|")
}

func semanticPatternFamily(name string) string {
	replacer := strings.NewReplacer(
		"bullish ", "",
		"bearish ", "",
		"neutral ", "",
		"ascending ", "",
		"descending ", "",
		"inverse ", "",
		"inverted ", "",
	)
	base := strings.TrimSpace(replacer.Replace(name))
	switch {
	case strings.Contains(base, "engulfing") || strings.Contains(base, "outside bar") || strings.Contains(base, "piercing") || strings.Contains(base, "dark cloud"):
		return "two_candle_reversal"
	case strings.Contains(base, "harami") || strings.Contains(base, "inside bar"):
		return "inside_bar"
	case strings.Contains(base, "doji"):
		return "doji"
	case strings.Contains(base, "marubozu") || strings.Contains(base, "belt hold") || strings.Contains(base, "long body"):
		return "strong_body"
	case strings.Contains(base, "pin bar") || strings.Contains(base, "hammer") || strings.Contains(base, "shooting star") || strings.Contains(base, "hanging man"):
		return "dominant_wick_reversal"
	case strings.Contains(base, "wedge"):
		return "wedge"
	case strings.Contains(base, "channel") || strings.Contains(base, "pitchfork"):
		return "channel"
	case strings.Contains(base, "triangle") || strings.Contains(base, "coil"):
		return "triangle"
	case strings.Contains(base, "rectangle") || strings.Contains(base, "range") || strings.Contains(base, "box"):
		return "range"
	case strings.Contains(base, "order block"):
		return "order_block"
	case strings.Contains(base, "fair value gap") || strings.Contains(base, "fvg") || strings.Contains(base, "imbalance"):
		return "imbalance"
	case strings.Contains(base, "liquidity") || strings.Contains(base, "sweep") || strings.Contains(base, "stop hunt") || strings.Contains(base, "turtle soup"):
		return "liquidity_sweep"
	case strings.Contains(base, "higher high") || strings.Contains(base, "higher low") || strings.Contains(base, "uptrend") || strings.Contains(base, "lower high") || strings.Contains(base, "lower low") || strings.Contains(base, "downtrend"):
		return "trend_structure"
	case strings.Contains(base, "lps") || strings.Contains(base, "last point"):
		return "wyckoff_last_point"
	case strings.Contains(base, "spring") || strings.Contains(base, "upthrust") || strings.Contains(base, "utad"):
		return "wyckoff_spring_upthrust"
	case strings.Contains(base, "volume"):
		return "volume"
	case strings.Contains(base, "fibonacci") || strings.Contains(base, "retracement") || strings.Contains(base, "extension"):
		return "fibonacci"
	default:
		return strings.Join(strings.Fields(base), "_")
	}
}

func historicalPatternATR(candles []ohlcv.Candle, end int) float64 {
	if len(candles) == 0 || end < 0 {
		return 0
	}
	start := end - 13
	if start < 0 {
		start = 0
	}
	return averageRange(candles[start : end+1])
}

func appendPatternReason(reasons []string, reason string) []string {
	if reason == "" || hasPatternReason(reasons, reason) {
		return reasons
	}
	return append(reasons, reason)
}

func hasPatternReason(reasons []string, reason string) bool {
	for _, item := range reasons {
		if item == reason {
			return true
		}
	}
	return false
}
