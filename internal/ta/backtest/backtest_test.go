package backtest

import (
	"testing"
	"time"

	"hissebot/internal/ta/features"
)

func TestBacktestTransactionCostReducesReturn(t *testing.T) {
	bars := []features.MarketBar{
		{Time: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), Open: 100, High: 101, Low: 99, Close: 100, Volume: 1000},
		{Time: time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC), Open: 100, High: 103, Low: 99, Close: 102, Volume: 1000},
	}
	report := RunCostBacktest("ASELS", "ml_gate", bars, CostsConfig{CommissionBps: 10, SlippageBps: 10, SpreadBps: 10})
	if report.TradeCount != 1 {
		t.Fatalf("trade count=%d", report.TradeCount)
	}
	if !(report.NetReturn < report.GrossReturn) {
		t.Fatalf("cost not applied: gross=%v net=%v", report.GrossReturn, report.NetReturn)
	}
}
