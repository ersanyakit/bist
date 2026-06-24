package validation

import (
	"testing"
	"time"

	"hissebot/internal/domain/marketdata"
)

func TestValidateOHLCVRejectsInvalidHighLow(t *testing.T) {
	report := ValidateOHLCV(marketdata.OHLCV{
		Symbol:        "ASELS",
		Timeframe:     marketdata.TimeframeDaily,
		Timestamp:     time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
		Open:          100,
		High:          95,
		Low:           90,
		Close:         98,
		AdjustedClose: 98,
		Volume:        1000,
	})

	if report.Status != "fail" {
		t.Fatalf("status = %s, want fail: %+v", report.Status, report)
	}
}

func TestValidateOHLCVSeriesDetectsDuplicate(t *testing.T) {
	ts := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	candle := marketdata.OHLCV{
		Symbol:        "ASELS",
		Timeframe:     marketdata.TimeframeDaily,
		Timestamp:     ts,
		Open:          100,
		High:          110,
		Low:           95,
		Close:         105,
		AdjustedClose: 105,
		Volume:        1000,
	}

	report := ValidateOHLCVSeries([]marketdata.OHLCV{candle, candle})
	if report.Status != "fail" {
		t.Fatalf("status = %s, want fail: %+v", report.Status, report)
	}
}
