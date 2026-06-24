package pricequality

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/util"
)

func TestImportOfficialClosesWritesCanonicalFilesAndEnablesVerifiedClose(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	mkdir(t, filepath.Join(equitiesDir, "AAA"))
	writeFile(t, filepath.Join(equitiesDir, "AAA", "equity.json"), `{"ticker":"AAA"}`)
	csvPath := filepath.Join(root, "official_closes.csv")
	writeFile(t, csvPath, "symbol,trading_date,close,source_timestamp\nAAA,2026-06-18,100.50,2026-06-18T15:10:00Z\n")

	report, err := ImportOfficialCloses(context.Background(), OfficialCloseImportOptions{
		EquitiesDir: equitiesDir,
		InputPath:   csvPath,
		Source:      "bist_official",
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Imported != 1 || report.Skipped != 0 || len(report.Errors) != 0 {
		t.Fatalf("unexpected import report = %+v", report)
	}
	var latest OfficialCloseRecord
	if err := util.ReadJSON(filepath.Join(equitiesDir, "AAA", "price", "official_close.json"), &latest); err != nil {
		t.Fatal(err)
	}
	if latest.Symbol != "AAA" || latest.Close != 100.5 || latest.Source != "bist_official" {
		t.Fatalf("unexpected official close = %+v", latest)
	}
	priceReport, err := InspectSymbol(context.Background(), "AAA", Options{
		EquitiesDir: equitiesDir,
		StaleAfter:  48 * time.Hour,
		Now: func() time.Time {
			return time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if priceReport.Status != StatusReadyForVerifiedClose || !priceReport.ReadyForVerifiedClose {
		t.Fatalf("expected verified close after import, got %+v", priceReport)
	}
}

func TestImportOfficialClosesRejectsMissingSourceTimestamp(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	mkdir(t, filepath.Join(equitiesDir, "AAA"))
	writeFile(t, filepath.Join(equitiesDir, "AAA", "equity.json"), `{"ticker":"AAA"}`)
	csvPath := filepath.Join(root, "official_closes.csv")
	writeFile(t, csvPath, "symbol,trading_date,close\nAAA,2026-06-18,100.50\n")

	report, err := ImportOfficialCloses(context.Background(), OfficialCloseImportOptions{
		EquitiesDir: equitiesDir,
		InputPath:   csvPath,
		Source:      "bist_official",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Imported != 0 || len(report.Errors) != 1 {
		t.Fatalf("expected strict source timestamp error, got %+v", report)
	}
}
