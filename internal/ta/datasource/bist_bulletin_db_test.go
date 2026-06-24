package datasource

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/services/bistbulletindb"
	"hissebot/internal/ta/ohlcv"
)

func TestBISTBulletinDBProviderReadsOfficialRowsSortedAndLimited(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bist_ohlcv.sqlite")
	store, err := bistbulletindb.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	days := []struct {
		date  string
		close float64
	}{
		{"2026-06-18", 4.67},
		{"2026-06-19", 4.21},
		{"2026-06-22", 4.24},
	}
	for _, day := range days {
		tradingDate, err := time.Parse("2006-01-02", day.date)
		if err != nil {
			t.Fatal(err)
		}
		open := day.close - 0.01
		source := bistbulletindb.SourceResult{
			SourceKey:         bistbulletindb.SourceKey(tradingDate, 1),
			TradingDate:       tradingDate,
			Session:           1,
			SourceFormat:      "csv",
			RowsSeen:          1,
			RowsStored:        1,
			RowsAnalysisReady: 1,
			CheckedAt:         tradingDate,
		}
		record := bistbulletindb.DailyRecord{
			Symbol:         "ALGYO",
			InstrumentCode: "ALGYO.E",
			CompanyName:    "ALARKO GMYO",
			TradingDate:    tradingDate,
			Open:           &open,
			High:           day.close + 0.10,
			Low:            day.close - 0.10,
			Close:          day.close,
			PreviousClose:  day.close - 0.03,
			Volume:         1000,
			ValueTraded:    4240,
			TradeCount:     42,
			VWAP:           day.close,
			Market:         "Z",
			SourceFormat:   "csv",
			AnalysisReady:  true,
		}
		if err := store.SaveProcessedSource(context.Background(), source, []bistbulletindb.DailyRecord{record}); err != nil {
			t.Fatalf("save source %s: %v", day.date, err)
		}
	}

	provider := NewBISTBulletinDBProvider(dbPath)
	instrument, err := provider.SearchSymbol(context.Background(), "algyo")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if instrument.CompanyName != "ALARKO GMYO" || instrument.AssetType != ohlcv.AssetTypeEquity {
		t.Fatalf("unexpected instrument: %+v", instrument)
	}

	records, err := provider.FetchDailyBulletinRecords(context.Background(), "ALGYO", 2)
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records len = %d, want 2", len(records))
	}
	if records[0].TradingDate != "2026-06-19" || records[1].TradingDate != "2026-06-22" {
		t.Fatalf("records not latest-limited ascending: %+v", records)
	}
	if records[1].Close != 4.24 || records[1].SourcePath != dbPath+"#bist_thb:2026-06-22:s1" {
		t.Fatalf("latest record mismatch: %+v", records[1])
	}

	candles, err := provider.FetchOHLCV(context.Background(), instrument, "1D", 2)
	if err != nil {
		t.Fatalf("ohlcv: %v", err)
	}
	if len(candles) != 2 || candles[1].Close != 4.24 {
		t.Fatalf("candles mismatch: %+v", candles)
	}
}
