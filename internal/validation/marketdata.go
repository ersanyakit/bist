package validation

import (
	"fmt"

	"hissebot/internal/domain/marketdata"
)

func ValidateOHLCV(candle marketdata.OHLCV) Report {
	report := NewReport()
	if candle.Symbol == "" {
		report.Add(SeverityError, "symbol_missing", "symbol", "symbol boş olamaz")
	}
	if candle.Timestamp.IsZero() {
		report.Add(SeverityError, "timestamp_missing", "timestamp", "timestamp boş olamaz")
	}
	if candle.Open < 0 || candle.High < 0 || candle.Low < 0 || candle.Close < 0 {
		report.Add(SeverityError, "negative_price", "ohlc", "open/high/low/close negatif olamaz")
	}
	if candle.High < candle.Open || candle.High < candle.Close || candle.High < candle.Low {
		report.Add(SeverityError, "invalid_high", "high", "high open/close/low değerlerinden küçük olamaz")
	}
	if candle.Low > candle.Open || candle.Low > candle.Close || candle.Low > candle.High {
		report.Add(SeverityError, "invalid_low", "low", "low open/close/high değerlerinden büyük olamaz")
	}
	if candle.Volume < 0 {
		report.Add(SeverityError, "negative_volume", "volume", "volume negatif olamaz")
	}
	if candle.AdjustedClose <= 0 && candle.Close > 0 {
		report.Add(SeverityWarning, "adjusted_close_missing", "adjusted_close", "adjusted_close eksik; kurumsal olay düzeltmesi teyitsiz")
	}
	return report
}

func ValidateOHLCVSeries(candles []marketdata.OHLCV) Report {
	report := NewReport()
	seen := map[string]bool{}
	for _, candle := range candles {
		report.Merge(ValidateOHLCV(candle))
		key := fmt.Sprintf("%s:%s:%s", candle.Symbol, candle.Timeframe, candle.Timestamp.UTC().Format("2006-01-02T15:04:05Z"))
		if seen[key] {
			report.Add(SeverityError, "duplicate_ohlcv", "timestamp", "aynı symbol + timeframe + timestamp için duplicate OHLCV var")
		}
		seen[key] = true
	}
	return report
}
