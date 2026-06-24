package normalization

import (
	"testing"
	"time"

	"hissebot/internal/domain/marketdata"
)

func TestApplyCorporateActionsAdjustsPreSplitCandles(t *testing.T) {
	candles := []marketdata.OHLCV{
		{
			Symbol:        "asels",
			Timeframe:     marketdata.TimeframeDaily,
			Timestamp:     DateUTC(2026, time.January, 1),
			Open:          100,
			High:          110,
			Low:           90,
			Close:         100,
			AdjustedClose: 100,
			Volume:        1000,
		},
		{
			Symbol:        "asels",
			Timeframe:     marketdata.TimeframeDaily,
			Timestamp:     DateUTC(2026, time.January, 3),
			Open:          60,
			High:          65,
			Low:           55,
			Close:         60,
			AdjustedClose: 60,
			Volume:        2000,
		},
	}
	actions := []marketdata.CorporateAction{{Symbol: "ASELS", Type: marketdata.ActionSplit, ExDate: DateUTC(2026, time.January, 2), Ratio: 2}}

	adjusted := ApplyCorporateActions(candles, actions)
	if adjusted[0].Close != 50 || adjusted[0].Volume != 2000 {
		t.Fatalf("pre-split candle not adjusted: %+v", adjusted[0])
	}
	if adjusted[1].Close != 60 || adjusted[1].Volume != 2000 {
		t.Fatalf("post-split candle should not change: %+v", adjusted[1])
	}
	if adjusted[0].Symbol != "ASELS" {
		t.Fatalf("symbol not normalized: %s", adjusted[0].Symbol)
	}
}
