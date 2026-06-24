package patterns

import (
	"math"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

const patternRuleVersion = "patterns.v2.backtest"

func enrichPatternBacktestMetadata(result ohlcv.PatternResult, candles []ohlcv.Candle) ohlcv.PatternResult {
	result.RuleVersion = patternRuleVersion
	result.SetupCompleteIndex = result.EndIndex
	result.TriggerIndex = result.EndIndex
	result.Trigger = "pattern_completed_on_close"
	if result.StartIndex < 0 || result.EndIndex < result.StartIndex || result.EndIndex >= len(candles) {
		return result
	}
	direction := result.Direction
	if direction != "bullish" && direction != "bearish" {
		result.Tradeable = false
		result.InvalidationLevel = neutralInvalidationLevel(candles[result.StartIndex : result.EndIndex+1])
		return result
	}
	window := candles[result.StartIndex : result.EndIndex+1]
	entry := candles[result.EndIndex].EffectiveClose()
	if entry <= 0 {
		return result
	}
	atr := math.Max(averageRange(window), priceDenominator(entry)*0.01)
	high := highest(window)
	low := lowest(window)
	height := math.Max(high-low, atr)
	buffer := atr * 0.25
	entryPad := atr * 0.10
	result.Tradeable = true
	result.EntryMin = math.Max(mathutil.Epsilon, entry-entryPad)
	result.EntryMax = entry + entryPad
	if direction == "bullish" {
		result.StopLoss = math.Max(mathutil.Epsilon, low-buffer)
		risk := entry - result.StopLoss
		if risk <= 0 {
			result.StopLoss = math.Max(mathutil.Epsilon, entry-atr)
			risk = entry - result.StopLoss
		}
		result.Target1 = entry + risk
		result.Target2 = entry + math.Max(2*risk, height)
		result.InvalidationLevel = result.StopLoss
		result.RiskRewardRatio = mathutil.SafeDiv(result.Target1-entry, risk)
		return result
	}
	result.StopLoss = high + buffer
	risk := result.StopLoss - entry
	if risk <= 0 {
		result.StopLoss = entry + atr
		risk = result.StopLoss - entry
	}
	result.Target1 = math.Max(mathutil.Epsilon, entry-risk)
	result.Target2 = math.Max(mathutil.Epsilon, entry-math.Max(2*risk, height))
	result.InvalidationLevel = result.StopLoss
	result.RiskRewardRatio = mathutil.SafeDiv(entry-result.Target1, risk)
	return result
}

func neutralInvalidationLevel(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	return (highest(candles) + lowest(candles)) / 2
}
