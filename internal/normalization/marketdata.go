package normalization

import (
	"math"
	"sort"
	"strings"
	"time"

	"hissebot/internal/domain/marketdata"
)

func NormalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func NormalizeOHLCV(candles []marketdata.OHLCV) []marketdata.OHLCV {
	out := make([]marketdata.OHLCV, 0, len(candles))
	for _, candle := range candles {
		candle.Symbol = NormalizeSymbol(candle.Symbol)
		candle.Timestamp = candle.Timestamp.UTC()
		if candle.AdjustedClose == 0 && candle.Close > 0 {
			candle.AdjustedClose = candle.Close
		}
		candle.Open = round6(candle.Open)
		candle.High = round6(candle.High)
		candle.Low = round6(candle.Low)
		candle.Close = round6(candle.Close)
		candle.AdjustedClose = round6(candle.AdjustedClose)
		out = append(out, candle)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}

func ApplyCorporateActions(candles []marketdata.OHLCV, actions []marketdata.CorporateAction) []marketdata.OHLCV {
	out := NormalizeOHLCV(candles)
	sort.Slice(actions, func(i, j int) bool { return actions[i].ExDate.Before(actions[j].ExDate) })
	for actionIndex := range actions {
		action := actions[actionIndex]
		if action.Ratio <= 0 {
			continue
		}
		switch action.Type {
		case marketdata.ActionSplit, marketdata.ActionBonusIssue, marketdata.ActionStockDividend:
			for i := range out {
				if out[i].Timestamp.Before(action.ExDate) {
					applySplitFactor(&out[i], action.Ratio)
				}
			}
		case marketdata.ActionReverseSplit:
			for i := range out {
				if out[i].Timestamp.Before(action.ExDate) {
					applySplitFactor(&out[i], 1/action.Ratio)
				}
			}
		}
	}
	return out
}

func applySplitFactor(candle *marketdata.OHLCV, ratio float64) {
	if ratio <= 0 {
		return
	}
	candle.Open = round6(candle.Open / ratio)
	candle.High = round6(candle.High / ratio)
	candle.Low = round6(candle.Low / ratio)
	candle.Close = round6(candle.Close / ratio)
	candle.AdjustedClose = round6(candle.AdjustedClose / ratio)
	candle.Volume = round6(candle.Volume * ratio)
}

func round6(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func DateUTC(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
