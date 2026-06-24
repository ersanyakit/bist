package ingestion

import (
	"context"
	"testing"
	"time"

	"hissebot/internal/datasources/mock"
	"hissebot/internal/domain/financials"
	"hissebot/internal/domain/macro"
	"hissebot/internal/domain/marketdata"
	"hissebot/internal/repositories/memory"
)

func TestPipelineIngestsOHLCVWithMockProvider(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	provider := mock.Provider{
		OHLCV: map[string][]marketdata.OHLCV{
			"ASELS": {
				{Symbol: "asels", Timeframe: marketdata.TimeframeDaily, Timestamp: time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC), Open: 100, High: 110, Low: 90, Close: 105, Volume: 1000},
				{Symbol: "ASELS", Timeframe: marketdata.TimeframeDaily, Timestamp: time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), Open: 105, High: 115, Low: 100, Close: 112, Volume: 1200},
			},
		},
	}

	result, err := Pipeline{MarketProvider: provider, Prices: store}.IngestOHLCV(ctx, "ASELS", marketdata.TimeframeDaily, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("IngestOHLCV() error = %v", err)
	}
	if result.SavedRecords != 2 || result.Status != "pass" {
		t.Fatalf("result = %+v", result)
	}
	got, err := store.ListOHLCV(ctx, "ASELS", marketdata.TimeframeDaily, time.Time{}, time.Time{})
	if err != nil || len(got) != 2 {
		t.Fatalf("stored candles len=%d err=%v", len(got), err)
	}
	if got[0].AdjustedClose != got[0].Close {
		t.Fatalf("adjusted close should default to close: %+v", got[0])
	}
}

func TestPipelineIngestsFinancialStatementsWithMockProvider(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	period := financials.Period{Year: 2026, Quarter: 1, Type: financials.PeriodQuarterly}
	statements := financials.Statements{
		Symbol:          "TEST",
		Period:          period,
		BalanceSheet:    financials.BalanceSheet{Symbol: "TEST", Period: period, TotalAssets: 100, TotalLiabilities: 40, TotalEquity: 60},
		IncomeStatement: financials.IncomeStatement{Symbol: "TEST", Period: period, Sales: 100, NetIncome: 10},
		CashFlow:        financials.CashFlowStatement{Symbol: "TEST", Period: period, OperatingCashFlow: 12, FreeCashFlow: 8},
		EquityStatement: financials.EquityStatement{Symbol: "TEST", Period: period, ClosingEquity: 60},
	}
	provider := mock.Provider{Financials: map[string]financials.Statements{mock.StatementsKey("TEST", period): statements}}

	result, err := Pipeline{FinancialProvider: provider, Financials: store}.IngestFinancialStatements(ctx, "TEST", period)
	if err != nil {
		t.Fatalf("IngestFinancialStatements() error = %v", err)
	}
	if result.SavedRecords != 1 || result.Status != "pass" {
		t.Fatalf("result = %+v", result)
	}
	if _, err := store.GetStatements(ctx, "TEST", period); err != nil {
		t.Fatalf("stored statements: %v", err)
	}
}

func TestPipelineIngestsGDPGrowthWithMockProvider(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	provider := mock.Provider{MacroSeries: map[macro.SeriesID][]macro.Observation{
		macro.SeriesGDPGrowth: {
			{SeriesID: macro.SeriesGDPGrowth, Date: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), Value: 5, Unit: "pct_yoy"},
			{SeriesID: macro.SeriesGDPGrowth, Date: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), Value: 4, Unit: "pct_yoy"},
		},
	}}

	result, err := Pipeline{MacroProvider: provider, Macro: store}.IngestGDPGrowth(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("IngestGDPGrowth() error = %v", err)
	}
	if result.SavedRecords != 2 || result.Status != "pass" {
		t.Fatalf("result = %+v", result)
	}
	series, err := store.GetSeries(ctx, macro.SeriesGDPGrowth, time.Time{}, time.Time{})
	if err != nil || len(series.Observations) != 2 {
		t.Fatalf("stored series len=%d err=%v", len(series.Observations), err)
	}
}
