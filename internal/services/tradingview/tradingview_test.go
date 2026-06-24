package tradingview

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

func TestParseOHLCVMapsColumnsToEquityOHLCV(t *testing.T) {
	store := storage.NewEquityStore(t.TempDir())
	feed := ScannerFeedFile{
		Source:  "tradingview",
		Key:     "ohlcv",
		Asset:   2,
		Ticker:  "EUPWR",
		Symbol:  "BIST:EUPWR",
		Columns: ohlcvColumns,
		Values: []any{
			"EUPWR", "Europower Enerji", "BIST", "TRY",
			100.0, 110.0, 95.0, 105.0, 123456.0, 1.5, 1.55, 12962900.0, float64(1710000000),
		},
	}
	if err := util.WriteJSON(store.TradingViewFeedPath("EUPWR", "ohlcv"), feed); err != nil {
		t.Fatalf("write feed: %v", err)
	}

	err := ParseOHLCV(context.Background(), config.Config{}, store, RequestSeed{Key: "ohlcv"})
	if err != nil {
		t.Fatalf("ParseOHLCV() error = %v", err)
	}

	equity, err := store.Load("EUPWR")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if equity.Name != "Europower Enerji" || equity.AssetType != 2 {
		t.Fatalf("equity metadata = %#v", equity)
	}
	if equity.OHLCV == nil || equity.OHLCV.Close == nil || *equity.OHLCV.Close != 105 {
		t.Fatalf("OHLCV close = %#v, want 105", equity.OHLCV)
	}
	if equity.OHLCV.Time == nil || *equity.OHLCV.Time != 1710000000 {
		t.Fatalf("OHLCV time = %#v, want 1710000000", equity.OHLCV.Time)
	}
}

func TestScannerColumnHelpers(t *testing.T) {
	params := map[string]any{"columns": []any{"name", "RSI.1D", "Recommend.All|1D", 42}}
	if got := columnsFromParams(params); !reflect.DeepEqual(got, []string{"name", "RSI.1D", "Recommend.All|1D"}) {
		t.Fatalf("columnsFromParams() = %#v", got)
	}

	values := valuesByColumn([]string{"a", "b"}, []any{1, 2, 3})
	if !reflect.DeepEqual(values, map[string]any{"a": 1, "b": 2}) {
		t.Fatalf("valuesByColumn() = %#v", values)
	}

	target := map[string]any{}
	setColumnValue(target, "close", 100.0)
	setColumnValue(target, "RSI.1D", 55.0)
	setColumnValue(target, "Recommend.All|1D", 0.25)
	if target["close"] != 100.0 {
		t.Fatalf("flat column value = %#v", target)
	}
	rsi := target["RSI"].(map[string]any)["1D"].(map[string]any)["value"]
	if rsi != 55.0 {
		t.Fatalf("nested RSI value = %#v", target)
	}
	recommend := target["Recommend"].(map[string]any)["All"].(map[string]any)["1D"]
	if recommend != 0.25 {
		t.Fatalf("suffixed recommend value = %#v", target)
	}
}

func TestScannerTypeConversions(t *testing.T) {
	if got := tickerFromItem(2, scannerItem{S: "BIST:eupwr"}); got != "EUPWR" {
		t.Fatalf("tickerFromItem(asset=2) = %q, want EUPWR", got)
	}
	if got := tickerFromItem(1, scannerItem{D: []any{" asels "}}); got != "ASELS" {
		t.Fatalf("tickerFromItem(asset=1) = %q, want ASELS", got)
	}
	if got := *floatFromAny("123,45"); got != 123.45 {
		t.Fatalf("floatFromAny() = %v, want 123.45", got)
	}
	if got := floatFromAny("NaN"); got != nil {
		t.Fatalf("floatFromAny(NaN) = %#v, want nil", got)
	}
	if got := *intFromAny(json.Number("1710000000")); got != 1710000000 {
		t.Fatalf("intFromAny() = %v, want 1710000000", got)
	}
	if got := stringFromAny("  TRY "); got != "TRY" {
		t.Fatalf("stringFromAny() = %q, want TRY", got)
	}
}

func TestChartHelpers(t *testing.T) {
	if got := ParseIntervals("1d,D, 60, 1M,60"); !reflect.DeepEqual(got, []string{"D", "60", "M"}) {
		t.Fatalf("ParseIntervals() = %#v", got)
	}
	if got := chartSymbol("eupwr", "D"); got != "BIST_DLY:EUPWR" {
		t.Fatalf("chartSymbol(D) = %q", got)
	}
	if got := chartSymbol("eupwr", "60"); got != "BIST:EUPWR" {
		t.Fatalf("chartSymbol(60) = %q", got)
	}
	if got := chartResolution("D"); got != "1D" {
		t.Fatalf("chartResolution(D) = %q", got)
	}
	if got := normalizeTransport("wss"); got != "socket" {
		t.Fatalf("normalizeTransport(wss) = %q", got)
	}
	if got := chartLookbackSeconds("D", 2); got != int64(2*4*24*60*60) {
		t.Fatalf("chartLookbackSeconds() = %d", got)
	}
}

func TestMergeChartCandlesSortsAndOverwritesDuplicateTimes(t *testing.T) {
	closeOld := 20.0
	closeNew := 22.0
	closeFirst := 10.0
	existing := []ChartCandle{{Time: 2, Close: &closeOld}}
	incoming := []ChartCandle{{Time: 1, Close: &closeFirst}, {Time: 2, Close: &closeNew}}

	got := mergeChartCandles(existing, incoming)
	if len(got) != 2 || got[0].Time != 1 || got[1].Time != 2 {
		t.Fatalf("mergeChartCandles() order = %#v", got)
	}
	if got[1].Close == nil || *got[1].Close != closeNew {
		t.Fatalf("mergeChartCandles() duplicate close = %#v, want %v", got[1].Close, closeNew)
	}
}

func TestTradingViewSocketPayloadParsing(t *testing.T) {
	first := `{"m":"one","p":[]}`
	second := `{"m":"two","p":[1]}`
	if got := tvPayloads(tvPackRaw(first) + tvPackRaw(second)); !reflect.DeepEqual(got, []string{first, second}) {
		t.Fatalf("tvPayloads() = %#v", got)
	}

	msg, err := parseTVMessage(second)
	if err != nil {
		t.Fatalf("parseTVMessage() error = %v", err)
	}
	if msg.Method != "two" || len(msg.Params) != 1 {
		t.Fatalf("parseTVMessage() = %#v", msg)
	}
}

func TestCandlesFromTimescaleParsesSeriesValues(t *testing.T) {
	update := map[string]any{
		"s1": map[string]any{
			"status": "ok",
			"node":   "node-1",
			"s": []map[string]any{
				{"i": 0, "v": []any{float64(10), 1.0, 2.0, 0.5, 1.5, 100.0}},
			},
		},
	}
	raw, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal update: %v", err)
	}

	candles, meta, ok := candlesFromTimescale([]json.RawMessage{json.RawMessage(`"session"`), raw}, "s1")
	if !ok {
		t.Fatalf("candlesFromTimescale() ok = false")
	}
	if len(candles) != 1 || candles[0].Time != 10 || candles[0].Close == nil || *candles[0].Close != 1.5 {
		t.Fatalf("candlesFromTimescale() candles = %#v", candles)
	}
	if meta["status"] != "ok" || meta["node"] != "node-1" {
		t.Fatalf("candlesFromTimescale() meta = %#v", meta)
	}
}

func TestChartRefUsesFirstAndLastCandleTimes(t *testing.T) {
	fetchedAt := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	chart := ChartFile{FetchedAt: fetchedAt, Candles: []ChartCandle{{Time: 1}, {Time: 3}}}
	ref := chartRef(filepath.Join("charts", "D.json"), "D", chart)
	if ref.Bars != 2 || ref.FirstTime == nil || *ref.FirstTime != 1 || ref.LastTime == nil || *ref.LastTime != 3 {
		t.Fatalf("chartRef() = %#v", ref)
	}
	if !ref.FetchedAt.Equal(fetchedAt) {
		t.Fatalf("chartRef() fetched_at = %v, want %v", ref.FetchedAt, fetchedAt)
	}
}
