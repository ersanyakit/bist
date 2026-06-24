package backtest

import (
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestRunTrendMomentumExecutesAfterSignalBar(t *testing.T) {
	candles := trendThenBreakCandles(140)
	result := RunTrendMomentum(candles, Config{CommissionBps: 5, SlippageBps: 10})
	if result.LookaheadViolations != 0 {
		t.Fatalf("lookahead violations = %d", result.LookaheadViolations)
	}
	if result.Trades == 0 {
		t.Fatalf("expected at least one trade, got %+v", result)
	}
	for _, trade := range result.TradesList {
		if trade.EntryIndex <= 0 {
			t.Fatalf("invalid entry index: %+v", trade)
		}
		if !trade.EntryTime.After(trade.EntrySignalAt) {
			t.Fatalf("entry must execute after signal availability: %+v", trade)
		}
		if !trade.ExitTime.After(trade.ExitSignalAt) {
			t.Fatalf("exit must execute after signal availability: %+v", trade)
		}
	}
}

func TestRunDowntrendMeanReversionExecutesAfterSignalBar(t *testing.T) {
	candles := downtrendBounceCandles(160)
	result := RunDowntrendMeanReversion(candles, Config{CommissionBps: 5, SlippageBps: 10})
	if result.LookaheadViolations != 0 {
		t.Fatalf("lookahead violations = %d", result.LookaheadViolations)
	}
	if result.Trades == 0 {
		t.Fatalf("expected at least one trade, got %+v", result)
	}
	for _, trade := range result.TradesList {
		if !trade.EntryTime.After(trade.EntrySignalAt) {
			t.Fatalf("entry must execute after signal availability: %+v", trade)
		}
		if !trade.ExitTime.After(trade.ExitSignalAt) {
			t.Fatalf("exit must execute after signal availability: %+v", trade)
		}
		if trade.HoldingBars < 20 {
			t.Fatalf("mean reversion trade exited before fixed holding window: %+v", trade)
		}
	}
}

func trendThenBreakCandles(count int) []ohlcv.Candle {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	price := 100.0
	for i := 0; i < count; i++ {
		if i < 100 {
			price += 0.8
		} else {
			price -= 1.4
		}
		open := price - 0.2
		closePrice := price
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   closePrice + 1,
			Low:    open - 1,
			Close:  closePrice,
			Volume: 100000,
		}
	}
	return candles
}

func downtrendBounceCandles(count int) []ohlcv.Candle {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	price := 120.0
	for i := 0; i < count; i++ {
		phase := i % 40
		if phase < 24 {
			price -= 1.2
		} else {
			price += 1.9
		}
		open := price - 0.25
		closePrice := price
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   closePrice + 1,
			Low:    open - 1,
			Close:  closePrice,
			Volume: 100000,
		}
	}
	return candles
}
