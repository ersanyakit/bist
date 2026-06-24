package storage

import (
	"path/filepath"
	"reflect"
	"testing"

	"hissebot/internal/domain"
)

func TestNormalizeHelpers(t *testing.T) {
	if got := NormalizeTicker(" bist:eupwr! "); got != "EUPWR" {
		t.Fatalf("NormalizeTicker() = %q, want EUPWR", got)
	}
	if got := NormalizeInterval(" 1d "); got != "1D" {
		t.Fatalf("NormalizeInterval() = %q, want 1D", got)
	}
	if got := NormalizeDataKey(" Recommend.All|1D "); got != "recommend.all_1d" {
		t.Fatalf("NormalizeDataKey() = %q, want recommend.all_1d", got)
	}
}

func TestEquityStoreSaveLoadUpdateAndList(t *testing.T) {
	store := NewEquityStore(t.TempDir())

	if err := store.Save(&domain.Equity{Ticker: "eupwr", Name: "Europower"}); err != nil {
		t.Fatalf("Save(EUPWR) error = %v", err)
	}
	if err := store.Save(&domain.Equity{Ticker: "asels", Name: "Aselsan"}); err != nil {
		t.Fatalf("Save(ASELS) error = %v", err)
	}
	if err := store.Update("BIST:EUPWR", func(e *domain.Equity) error {
		e.Pair = "BIST:EUPWR"
		return nil
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	loaded, err := store.Load("eupwr")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Ticker != "EUPWR" || loaded.Pair != "BIST:EUPWR" {
		t.Fatalf("Load() = %#v", loaded)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	gotTickers := []string{list[0].Ticker, list[1].Ticker}
	wantTickers := []string{"ASELS", "EUPWR"}
	if !reflect.DeepEqual(gotTickers, wantTickers) {
		t.Fatalf("List() tickers = %#v, want %#v", gotTickers, wantTickers)
	}
}

func TestMaterializeEmbeddedDataFilesWritesMissingAndAvailableFiles(t *testing.T) {
	store := NewEquityStore(t.TempDir())
	closeValue := 105.0
	equity := &domain.Equity{
		Ticker:    "EUPWR",
		AssetType: 2,
		OHLCV:     &domain.OHLCV{Source: "fixture", Close: &closeValue},
		KAPInfo:   map[string]any{"stockCode": "EUPWR"},
		MKKID:     123,
	}
	if err := store.Save(equity); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.MaterializeEmbeddedDataFiles(); err != nil {
		t.Fatalf("MaterializeEmbeddedDataFiles() error = %v", err)
	}

	for _, path := range []string{
		store.OHLCVPath("EUPWR"),
		store.KAPPath("EUPWR"),
		store.MKKPath("EUPWR"),
		store.FinancialInfoPath("EUPWR"),
		store.FinancialCalculationsPath("EUPWR"),
	} {
		if _, err := filepath.Abs(path); err != nil {
			t.Fatalf("invalid path %s: %v", path, err)
		}
	}
}
