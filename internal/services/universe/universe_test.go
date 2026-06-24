package universe

import (
	"context"
	"path/filepath"
	"testing"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

func TestValidateRequiresListedAndDelistedSources(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2}); err != nil {
		t.Fatal(err)
	}
	listed := filepath.Join(dir, "listed.json")
	if err := util.WriteJSON(listed, []Entry{{Ticker: "ALGYO"}}); err != nil {
		t.Fatal(err)
	}
	report, err := Validate(context.Background(), config.Config{
		UniverseFile:         listed,
		DelistedUniverseFile: filepath.Join(dir, "missing-delisted.json"),
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "fail" || report.DelistedSourceAvailable {
		t.Fatalf("expected missing delisted universe to fail, got %+v", report)
	}
}

func TestValidateRejectsEmptyDelistedPlaceholder(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2}); err != nil {
		t.Fatal(err)
	}
	listed := filepath.Join(dir, "listed.json")
	delisted := filepath.Join(dir, "delisted.json")
	if err := util.WriteJSON(listed, Snapshot{Source: "unit_test_listed", Entries: []Entry{{Ticker: "ALGYO"}}}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(delisted, Snapshot{Source: "equity_json_delisted_snapshot_empty", Entries: []Entry{}}); err != nil {
		t.Fatal(err)
	}
	report, err := Validate(context.Background(), config.Config{
		UniverseFile:         listed,
		DelistedUniverseFile: delisted,
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "fail" || !containsWarning(report.Warnings, "delisted_universe_source_empty_or_placeholder") {
		t.Fatalf("expected empty delisted placeholder failure, got %+v", report)
	}
}

func TestValidatePassesWhenTickerCoveredByUniverse(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	if err := store.Save(&domain.Equity{Ticker: "ALGYO", AssetType: 2}); err != nil {
		t.Fatal(err)
	}
	listed := filepath.Join(dir, "listed.json")
	delisted := filepath.Join(dir, "delisted.json")
	if err := util.WriteJSON(listed, []Entry{{Ticker: "ALGYO", ListedAt: nil}}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(delisted, []Entry{{Ticker: "OLDCO", DelistedAt: nil}}); err != nil {
		t.Fatal(err)
	}
	report, err := Validate(context.Background(), config.Config{
		UniverseFile:         listed,
		DelistedUniverseFile: delisted,
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "pass" || len(report.MissingTickers) != 0 {
		t.Fatalf("expected universe validation pass, got %+v", report)
	}
}

func TestExportCurrentUniverseSplitsNonTradingKAPRecords(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(dir, "equities"))
	if err := store.Save(&domain.Equity{Ticker: "LIVE", AssetType: 2, KAPInfo: map[string]any{"payIslemDurumu": "1"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&domain.Equity{Ticker: "OLDCO", AssetType: 2, KAPInfo: map[string]any{"payIslemDurumu": "0"}}); err != nil {
		t.Fatal(err)
	}
	listed := filepath.Join(dir, "listed.json")
	delisted := filepath.Join(dir, "delisted.json")
	report, err := ExportCurrentUniverse(context.Background(), config.Config{
		UniverseFile:         listed,
		DelistedUniverseFile: delisted,
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	if report.ListedCount != 1 || report.DelistedCount != 1 {
		t.Fatalf("export report = %+v", report)
	}
	validation, err := Validate(context.Background(), config.Config{
		UniverseFile:         listed,
		DelistedUniverseFile: delisted,
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Status != "pass" {
		t.Fatalf("validation = %+v", validation)
	}
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if warning == want {
			return true
		}
	}
	return false
}
