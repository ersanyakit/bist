// internal/risk/engine.go
package risk

import (
	"fmt"
	"math"

	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/supportresistance"
	"hissebot/pkg/mathutil"
)

type Input struct {
	LastPrice  float64
	ATR        float64
	Bias       string
	AllowShort bool
	Patterns   []ohlcv.PatternResult
	Levels     supportresistance.Result
	Indicators ohlcv.IndicatorSnapshot
}

const (
	defaultATRPct      = 0.03
	minATRPct          = 0.002
	maxATRPct          = 0.06
	maxEntryDistance   = 0.05
	maxLevelDistance   = 0.18
	maxTargetDistance  = 0.40
	maxStopDistance    = 0.15
	entryATRMultiplier = 0.35
	stopATRMultiplier  = 1.40
	stopLevelBufferATR = 0.35
)

func BuildTradePlan(input Input) (ohlcv.TradePlan, error) {
	if input.LastPrice <= 0 {
		return ohlcv.TradePlan{}, fmt.Errorf("risk engine requires positive last price: %w", ErrInvalidRiskInput)
	}
	atr := normalizedATR(input.LastPrice, input.ATR)
	direction := chooseDirection(input)
	if direction == "short" && !input.AllowShort {
		return ohlcv.TradePlan{
			Direction:       "neutral",
			Quality:         "low",
			ConfidenceScore: 0.20,
			Rejected:        true,
			RejectReason:    "short selling is not supported for this instrument",
			Reasoning:       []string{"Bearish evidence does not create a spot long setup."},
		}, nil
	}
	if direction == "neutral" {
		return ohlcv.TradePlan{
			Direction:       "neutral",
			Quality:         "low",
			ConfidenceScore: 0.25,
			Rejected:        true,
			RejectReason:    "neutral trend bias does not provide enough directional edge",
			Reasoning:       []string{"Trend, momentum and pattern evidence are mixed."},
		}, nil
	}
	entryMin := input.LastPrice - atr*entryATRMultiplier
	entryMax := input.LastPrice + atr*entryATRMultiplier
	stop := 0.0
	tp1 := 0.0
	tp2 := 0.0
	reasoning := []string{}
	if direction == "long" {
		if relevantSupport(input.Levels.NearestSupport, input.LastPrice) {
			entryMin = math.Max(input.LastPrice*(1-maxEntryDistance), math.Min(entryMin, input.Levels.NearestSupport.Price+atr*0.15))
			stop = math.Max(entryMin-stopATRMultiplier*atr, input.Levels.NearestSupport.Price-stopLevelBufferATR*atr)
			reasoning = append(reasoning, "Nearest support is used as long risk reference.")
		} else {
			if input.Levels.NearestSupport != nil {
				reasoning = append(reasoning, "Nearest support is ignored because it is too far from the last price.")
			}
			stop = entryMin - stopATRMultiplier*atr
		}
		if relevantResistance(input.Levels.NearestResistance, input.LastPrice) && pctDistance(input.LastPrice, input.Levels.NearestResistance.Price) <= maxTargetDistance {
			resistanceTarget := input.Levels.NearestResistance.Price
			if resistanceTarget > entryMax+atr*0.25 {
				tp1 = math.Min(resistanceTarget, entryMax+2*atr)
				tp2 = math.Max(tp1+atr, math.Min(resistanceTarget+2*atr, entryMax+4*atr))
				reasoning = append(reasoning, "Nearest resistance is used as first upside objective.")
			} else {
				tp1 = entryMax + 2*atr
				tp2 = entryMax + 4*atr
				reasoning = append(reasoning, "Nearest resistance is too close to entry, so ATR based upside objectives are used.")
			}
		} else {
			if input.Levels.NearestResistance != nil {
				reasoning = append(reasoning, "Nearest resistance is ignored because it is too far from the last price.")
			}
			tp1 = entryMax + 2*atr
			tp2 = entryMax + 4*atr
		}
	} else {
		if relevantResistance(input.Levels.NearestResistance, input.LastPrice) {
			entryMax = math.Min(input.LastPrice*(1+maxEntryDistance), math.Max(entryMax, input.Levels.NearestResistance.Price-atr*0.15))
			stop = math.Min(entryMax+stopATRMultiplier*atr, input.Levels.NearestResistance.Price+stopLevelBufferATR*atr)
			reasoning = append(reasoning, "Nearest resistance is used as short risk reference.")
		} else {
			if input.Levels.NearestResistance != nil {
				reasoning = append(reasoning, "Nearest resistance is ignored because it is too far from the last price.")
			}
			stop = entryMax + stopATRMultiplier*atr
		}
		if relevantSupport(input.Levels.NearestSupport, input.LastPrice) && pctDistance(input.LastPrice, input.Levels.NearestSupport.Price) <= maxTargetDistance {
			supportTarget := input.Levels.NearestSupport.Price
			if supportTarget < entryMin-atr*0.25 {
				tp1 = math.Max(supportTarget, entryMin-2*atr)
				tp2 = math.Min(tp1-atr, math.Max(supportTarget-2*atr, entryMin-4*atr))
				reasoning = append(reasoning, "Nearest support is used as first downside objective.")
			} else {
				tp1 = entryMin - 2*atr
				tp2 = entryMin - 4*atr
				reasoning = append(reasoning, "Nearest support is too close to entry, so ATR based downside objectives are used.")
			}
		} else {
			if input.Levels.NearestSupport != nil {
				reasoning = append(reasoning, "Nearest support is ignored because it is too far from the last price.")
			}
			tp1 = entryMin - 2*atr
			tp2 = entryMin - 4*atr
		}
	}
	entryMin, entryMax, stop, tp1, tp2, reasoning = sanitizePlanLevels(input.LastPrice, atr, direction, entryMin, entryMax, stop, tp1, tp2, reasoning)

	riskValue := math.Abs((entryMin+entryMax)/2 - stop)
	rewardValue := math.Abs(tp1 - (entryMin+entryMax)/2)
	rr := mathutil.SafeDiv(rewardValue, riskValue)
	confidence := confidence(input, rr)
	quality := "medium"
	if confidence >= 0.72 && rr >= 2 {
		quality = "high"
	}
	rejected := false
	rejectReason := ""
	if rr < 1.5 {
		rejected = true
		quality = "low"
		rejectReason = "risk reward ratio is below 1.5"
	}
	if invalidPlan(input.LastPrice, direction, entryMin, entryMax, stop, tp1, tp2) {
		rejected = true
		quality = "low"
		rejectReason = "trade plan levels failed sanity checks"
		reasoning = append(reasoning, "Trade plan is rejected because generated levels are not internally consistent.")
	}
	if len(reasoning) == 0 {
		reasoning = append(reasoning, "ATR based entry, stop and targets are used because nearby levels are not available.")
	}
	return ohlcv.TradePlan{
		Direction:       direction,
		EntryMin:        entryMin,
		EntryMax:        entryMax,
		TakeProfit1:     tp1,
		TakeProfit2:     tp2,
		StopLoss:        stop,
		RiskRewardRatio: rr,
		Quality:         quality,
		ConfidenceScore: confidence,
		Rejected:        rejected,
		RejectReason:    rejectReason,
		Reasoning:       reasoning,
	}, nil
}

var ErrInvalidRiskInput = fmt.Errorf("invalid risk input")

func normalizedATR(lastPrice, atr float64) float64 {
	if atr <= 0 || math.IsNaN(atr) || math.IsInf(atr, 0) {
		atr = lastPrice * defaultATRPct
	}
	minATR := lastPrice * minATRPct
	maxATR := lastPrice * maxATRPct
	return mathutil.Clamp(atr, minATR, maxATR)
}

func relevantSupport(level *ohlcv.SupportResistanceLevel, lastPrice float64) bool {
	return level != nil && finitePositive(level.Price) && level.Price < lastPrice && pctDistance(lastPrice, level.Price) <= maxLevelDistance
}

func relevantResistance(level *ohlcv.SupportResistanceLevel, lastPrice float64) bool {
	return level != nil && finitePositive(level.Price) && level.Price > lastPrice && pctDistance(lastPrice, level.Price) <= maxLevelDistance
}

func sanitizePlanLevels(lastPrice, atr float64, direction string, entryMin, entryMax, stop, tp1, tp2 float64, reasoning []string) (float64, float64, float64, float64, float64, []string) {
	minEntry := lastPrice * (1 - maxEntryDistance)
	maxEntry := lastPrice * (1 + maxEntryDistance)
	entryMin = mathutil.Clamp(entryMin, minEntry, maxEntry)
	entryMax = mathutil.Clamp(entryMax, minEntry, maxEntry)
	if entryMin > entryMax {
		entryMin, entryMax = entryMax, entryMin
	}
	entryMid := (entryMin + entryMax) / 2
	if direction == "long" {
		fallbackStop := entryMin - stopATRMultiplier*atr
		minStop := entryMid * (1 - maxStopDistance)
		maxStop := entryMin - math.Max(atr*0.25, lastPrice*0.002)
		if !finitePositive(stop) || stop >= entryMin || stop < minStop {
			stop = mathutil.Clamp(fallbackStop, minStop, maxStop)
			reasoning = append(reasoning, "Stop loss is normalized to a bounded ATR distance from entry.")
		}
		if tp1 <= entryMax || !finitePositive(tp1) || pctDistance(lastPrice, tp1) > maxTargetDistance {
			tp1 = entryMax + 2*atr
			reasoning = append(reasoning, "First target is normalized to an ATR based objective.")
		}
		if tp2 <= tp1 || !finitePositive(tp2) || pctDistance(lastPrice, tp2) > maxTargetDistance {
			tp2 = tp1 + 2*atr
			reasoning = append(reasoning, "Second target is normalized to an ATR based objective.")
		}
		return entryMin, entryMax, stop, tp1, tp2, reasoning
	}
	fallbackStop := entryMax + stopATRMultiplier*atr
	maxStop := entryMid * (1 + maxStopDistance)
	minStop := entryMax + math.Max(atr*0.25, lastPrice*0.002)
	if !finitePositive(stop) || stop <= entryMax || stop > maxStop {
		stop = mathutil.Clamp(fallbackStop, minStop, maxStop)
		reasoning = append(reasoning, "Stop loss is normalized to a bounded ATR distance from entry.")
	}
	if tp1 >= entryMin || !finitePositive(tp1) || pctDistance(lastPrice, tp1) > maxTargetDistance {
		tp1 = entryMin - 2*atr
		reasoning = append(reasoning, "First target is normalized to an ATR based objective.")
	}
	if tp2 >= tp1 || !finitePositive(tp2) || pctDistance(lastPrice, tp2) > maxTargetDistance {
		tp2 = tp1 - 2*atr
		reasoning = append(reasoning, "Second target is normalized to an ATR based objective.")
	}
	if tp1 <= 0 {
		tp1 = lastPrice * (1 - maxStopDistance)
		reasoning = append(reasoning, "First target is floored to a positive price level.")
	}
	if tp2 <= 0 {
		tp2 = math.Max(lastPrice*(1-maxTargetDistance), tp1-2*atr)
		reasoning = append(reasoning, "Second target is floored to a positive price level.")
	}
	return entryMin, entryMax, stop, tp1, tp2, reasoning
}

func invalidPlan(lastPrice float64, direction string, entryMin, entryMax, stop, tp1, tp2 float64) bool {
	for _, value := range []float64{entryMin, entryMax, stop, tp1, tp2} {
		if !finitePositive(value) {
			return true
		}
	}
	if entryMin > entryMax || pctDistance(lastPrice, entryMin) > maxEntryDistance+0.001 || pctDistance(lastPrice, entryMax) > maxEntryDistance+0.001 {
		return true
	}
	entryMid := (entryMin + entryMax) / 2
	if direction == "long" {
		return stop >= entryMin || tp1 <= entryMax || tp2 <= tp1 || mathutil.SafeDiv(entryMid-stop, entryMid) > maxStopDistance+0.001
	}
	return stop <= entryMax || tp1 >= entryMin || tp2 >= tp1 || mathutil.SafeDiv(stop-entryMid, entryMid) > maxStopDistance+0.001
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func pctDistance(a, b float64) float64 {
	return math.Abs(a-b) / math.Max(math.Abs(a), mathutil.Epsilon)
}

func chooseDirection(input Input) string {
	if input.Bias == "bullish" {
		return "long"
	}
	if input.Bias == "bearish" {
		return "short"
	}
	bullish, bearish := 0, 0
	for _, pattern := range input.Patterns {
		if pattern.Direction == "bullish" && pattern.Confidence >= 0.6 {
			bullish++
		}
		if pattern.Direction == "bearish" && pattern.Confidence >= 0.6 {
			bearish++
		}
	}
	if bullish > bearish+1 {
		return "long"
	}
	if bearish > bullish+1 {
		return "short"
	}
	return "neutral"
}

func confidence(input Input, rr float64) float64 {
	score := 0.35
	if input.Bias == "bullish" || input.Bias == "bearish" {
		score += 0.18
	}
	score += mathutil.Clamp(rr/4, 0, 0.25)
	confirmed := 0.0
	total := 0.0
	for _, pattern := range input.Patterns {
		total++
		if pattern.VolumeConfirmed {
			confirmed++
		}
	}
	score += mathutil.SafeDiv(confirmed, total) * 0.22
	return mathutil.Clamp(score, 0, 1)
}
