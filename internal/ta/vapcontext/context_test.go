package vapcontext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestLoadFreeFloatAnalyzesWorkbook(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "fiili_dolasim")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Fiili_Dolasim_Raporu_MKK-ASELS.xlsx")
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []any{"Tarih", "ISIN", "ISIN Açıklama", "Borsa Kodu", "İhraççı Üye", "Fiili Dolaşımdaki Pay Adedi", "İhraççı Sermaye", "Fiili Pay/Sermaye Oranı (%)"}
	if err := f.SetSheetRow(sheet, "A3", &headers); err != nil {
		t.Fatal(err)
	}
	for index, row := range [][]any{
		{"01.06.2026", "X", "ASELSAN", "ASELS", "X", 300.0, 1000.0, 30.0},
		{"02.06.2026", "X", "ASELSAN", "ASELS", "X", 310.0, 1000.0, 31.0},
		{"03.06.2026", "X", "ASELSAN", "ASELS", "X", 320.0, 1000.0, 32.0},
	} {
		cell, _ := excelize.CoordinatesToCellName(1, index+4)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()

	report := LoadFreeFloat(root, "ASELS", time.Date(2026, 6, 2, 18, 0, 0, 0, time.UTC))
	if !report.Computed || report.Observations != 2 || report.LatestDate != "2026-06-02" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.FreeFloatRatioPct != 31 || report.RatioChange1DPP != 1 {
		t.Fatalf("unexpected values: %+v", report)
	}
}

func TestLoadIndexPortfolioUsesSectorAndAsOf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bist_endeks_portfoy.json")
	payload := map[string]any{"records": []map[string]any{
		{"year_month": "2026-03", "endeks": "BIST 100", "portfoy_deger_mtl": 100.0},
		{"year_month": "2026-04", "endeks": "BIST 100", "portfoy_deger_mtl": 105.0},
		{"year_month": "2026-03", "endeks": "BIST BANKA", "portfoy_deger_mtl": 50.0},
		{"year_month": "2026-04", "endeks": "BIST BANKA", "portfoy_deger_mtl": 60.0},
		{"year_month": "2026-05", "endeks": "BIST BANKA", "portfoy_deger_mtl": 80.0},
	}}
	raw, _ := json.Marshal(payload)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	report := LoadIndexPortfolio(path, "Bankacılık", "Banka", time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC))
	if !report.Computed || report.SelectedIndex != "BIST BANKA" || report.LatestMonth != "2026-04" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Change1MPct != 20 {
		t.Fatalf("change = %v, want 20", report.Change1MPct)
	}
}
