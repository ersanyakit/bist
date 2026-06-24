package corporateactions

import (
	"math"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func Apply(candles []ohlcv.Candle, actions ActionSet, timeframe string) ([]ohlcv.Candle, AdjustmentReport) {
	report := AdjustmentReport{
		Symbol:       actions.Symbol,
		Timeframe:    timeframe,
		PriceSeries:  "raw",
		BacktestSafe: true,
	}
	if len(candles) == 0 {
		report.BacktestSafe = false
		report.Warnings = append(report.Warnings, "ohlcv_missing")
		return candles, report
	}
	out := append([]ohlcv.Candle(nil), candles...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	alreadyAdjusted := allCandlesAdjusted(out)
	report.ActionsConsidered = len(actions.Actions)
	if alreadyAdjusted {
		report.PriceSeries = "provider_adjusted"
		report.AdjustedCandles = len(out)
		report.BacktestSafe = potentialGapCount(out) == 0
		report.PotentialCorporateGapBars = potentialGapCount(out)
		if !report.BacktestSafe {
			report.Warnings = append(report.Warnings, "provider_adjusted_series_still_has_large_gap")
		}
		return out, report
	}
	if len(actions.Actions) == 0 {
		report.BacktestSafe = false
		report.UnadjustedCandles = len(out)
		report.PotentialCorporateGapBars = potentialGapCount(out)
		report.Warnings = append(report.Warnings, "corporate_action_source_missing")
		if report.PotentialCorporateGapBars > 0 {
			report.Warnings = append(report.Warnings, "potential_split_or_corporate_action_gap_detected")
		}
		return out, report
	}

	appliedAny := false
	futureActions := 0
	blockingSkippedActions := 0
	lastCandleTime := out[len(out)-1].Time
	for _, action := range actions.Actions {
		if action.EffectiveDate != nil && action.EffectiveDate.After(lastCandleTime) {
			futureActions++
			continue
		}
		factor, ok := actionAdjustmentFactor(out, action)
		if !ok {
			report.SkippedActions++
			report.SkippedActionIDs = append(report.SkippedActionIDs, action.ID)
			if skippedActionBlocksBacktest(action) {
				blockingSkippedActions++
			}
			continue
		}
		applyFactorBefore(out, *action.EffectiveDate, factor)
		report.AppliedActions++
		report.AppliedActionIDs = append(report.AppliedActionIDs, action.ID)
		appliedAny = true
	}
	if appliedAny {
		for i := range out {
			if !out[i].IsAdjusted {
				out[i].AdjustedOpen = out[i].Open
				out[i].AdjustedHigh = out[i].High
				out[i].AdjustedLow = out[i].Low
				out[i].AdjustedClose = out[i].Close
				out[i].AdjustedVolume = out[i].Volume
				out[i].IsAdjusted = true
			}
		}
		report.PriceSeries = "corporate_action_adjusted"
		report.AdjustedCandles = len(out)
	} else {
		report.UnadjustedCandles = len(out)
	}
	report.PotentialCorporateGapBars = potentialGapCount(out)
	switch {
	case report.AppliedActions > 0:
		report.BacktestSafe = report.PotentialCorporateGapBars == 0 && blockingSkippedActions == 0
	case report.SkippedActions == 0 && futureActions > 0 && futureActions == report.ActionsConsidered:
		report.BacktestSafe = report.PotentialCorporateGapBars == 0
		report.Warnings = append(report.Warnings, "future_corporate_actions_not_applied_to_historical_series")
	case report.SkippedActions > 0 && blockingSkippedActions == 0:
		report.BacktestSafe = report.PotentialCorporateGapBars == 0
	default:
		report.BacktestSafe = false
		report.Warnings = append(report.Warnings, "no_adjustment_ready_corporate_actions")
	}
	if report.SkippedActions > 0 {
		report.Warnings = append(report.Warnings, "corporate_actions_skipped_missing_effective_date_or_factor")
		if blockingSkippedActions > 0 {
			report.BacktestSafe = false
			report.Warnings = append(report.Warnings, "verified_price_affecting_corporate_actions_incomplete")
		}
	}
	if report.PotentialCorporateGapBars > 0 {
		report.BacktestSafe = false
		report.Warnings = append(report.Warnings, "potential_split_or_corporate_action_gap_detected")
	}
	if actions.Status == "events_only" || actions.Status == "candidate_adjustments_review_required" {
		report.Warnings = append(report.Warnings, "corporate_action_source_requires_review")
	}
	return out, report
}

func skippedActionBlocksBacktest(action Action) bool {
	if action.Status != StatusVerified {
		return false
	}
	switch action.Type {
	case TypeDividend, TypeBonusIssue, TypeRightsIssue, TypeSplit:
		return true
	default:
		return false
	}
}

func actionAdjustmentFactor(candles []ohlcv.Candle, action Action) (float64, bool) {
	if action.EffectiveDate == nil {
		return 0, false
	}
	if action.AdjustmentFactor != nil && finitePositive(*action.AdjustmentFactor) {
		return *action.AdjustmentFactor, true
	}
	switch action.Type {
	case TypeBonusIssue, TypeSplit:
		if action.Ratio == nil || *action.Ratio <= 0 {
			return 0, false
		}
		return 1 / (1 + *action.Ratio), true
	case TypeDividend:
		if action.CashAmount == nil || *action.CashAmount <= 0 {
			return 0, false
		}
		prevClose := closeBefore(candles, *action.EffectiveDate)
		if prevClose <= *action.CashAmount || prevClose <= 0 {
			return 0, false
		}
		return (prevClose - *action.CashAmount) / prevClose, true
	case TypeRightsIssue:
		if action.Ratio == nil || action.SubscriptionPrice == nil || *action.Ratio <= 0 || *action.SubscriptionPrice <= 0 {
			return 0, false
		}
		prevClose := closeBefore(candles, *action.EffectiveDate)
		if prevClose <= 0 {
			return 0, false
		}
		ratio := *action.Ratio
		subscriptionCost := *action.SubscriptionPrice * ratio
		return (prevClose + subscriptionCost) / (prevClose * (1 + ratio)), true
	default:
		return 0, false
	}
}

func applyFactorBefore(candles []ohlcv.Candle, effective time.Time, factor float64) {
	if !finitePositive(factor) {
		return
	}
	for i := range candles {
		if !candles[i].Time.Before(effective) {
			continue
		}
		baseOpen := effectiveBase(candles[i].AdjustedOpen, candles[i].Open)
		baseHigh := effectiveBase(candles[i].AdjustedHigh, candles[i].High)
		baseLow := effectiveBase(candles[i].AdjustedLow, candles[i].Low)
		baseClose := effectiveBase(candles[i].AdjustedClose, candles[i].Close)
		baseVolume := effectiveBase(candles[i].AdjustedVolume, candles[i].Volume)
		candles[i].AdjustedOpen = baseOpen * factor
		candles[i].AdjustedHigh = baseHigh * factor
		candles[i].AdjustedLow = baseLow * factor
		candles[i].AdjustedClose = baseClose * factor
		if factor > 0 {
			candles[i].AdjustedVolume = baseVolume / factor
		} else {
			candles[i].AdjustedVolume = baseVolume
		}
		candles[i].IsAdjusted = true
	}
}

func allCandlesAdjusted(candles []ohlcv.Candle) bool {
	if len(candles) == 0 {
		return false
	}
	for _, candle := range candles {
		if !candle.IsAdjusted {
			return false
		}
	}
	return true
}

func potentialGapCount(candles []ohlcv.Candle) int {
	count := 0
	for i := 1; i < len(candles); i++ {
		prevClose := candles[i-1].EffectiveClose()
		open := candles[i].EffectiveOpen()
		if prevClose <= 0 || open <= 0 {
			continue
		}
		gap := math.Abs(open/prevClose - 1)
		if gap >= 0.35 {
			count++
		}
	}
	return count
}

func closeBefore(candles []ohlcv.Candle, effective time.Time) float64 {
	for i := len(candles) - 1; i >= 0; i-- {
		if candles[i].Time.Before(effective) {
			return candles[i].EffectiveClose()
		}
	}
	return 0
}

func effectiveBase(adjusted, raw float64) float64 {
	if adjusted > 0 {
		return adjusted
	}
	return raw
}

func finitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func SummaryWarnings(report AdjustmentReport) []string {
	out := append([]string(nil), report.Warnings...)
	if strings.TrimSpace(report.PriceSeries) == "raw" {
		out = append(out, "raw_price_series_used")
	}
	return out
}
