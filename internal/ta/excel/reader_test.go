package excel

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestReadCompaniesAcceptsTurkishHeaderWorkbook(t *testing.T) {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	sheet := file.GetSheetName(file.GetActiveSheetIndex())
	rows := [][]any{
		{"Sirketler Listesi"},
		{},
		{"Kod", "Şirket Ünvanı", "Şehir", "Bağımsız Denetim Kuruluşu"},
		{"A"},
		{"GARAN, TGB", "TÜRKİYE GARANTİ BANKASI A.Ş.", "İSTANBUL", ""},
		{"ADEL", "ADEL KALEMCİLİK TİCARET VE SANAYİ A.Ş.", "KOCAELİ", ""},
	}
	for idx, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, idx+1)
		if err != nil {
			t.Fatalf("cell coordinate: %v", err)
		}
		if err := file.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatalf("set row %d: %v", idx+1, err)
		}
	}

	path := filepath.Join(t.TempDir(), "companies.xlsx")
	if err := file.SaveAs(path); err != nil {
		t.Fatalf("save workbook: %v", err)
	}

	companies, err := ReadCompanies(context.Background(), path)
	if err != nil {
		t.Fatalf("read companies: %v", err)
	}

	want := []struct {
		symbol string
		name   string
	}{
		{"GARAN", "TÜRKİYE GARANTİ BANKASI A.Ş."},
		{"TGB", "TÜRKİYE GARANTİ BANKASI A.Ş."},
		{"ADEL", "ADEL KALEMCİLİK TİCARET VE SANAYİ A.Ş."},
	}
	if len(companies) != len(want) {
		t.Fatalf("company count = %d, want %d", len(companies), len(want))
	}
	for idx, expected := range want {
		if companies[idx].Symbol != expected.symbol {
			t.Fatalf("companies[%d].Symbol = %q, want %q", idx, companies[idx].Symbol, expected.symbol)
		}
		if companies[idx].CompanyName != expected.name {
			t.Fatalf("companies[%d].CompanyName = %q, want %q", idx, companies[idx].CompanyName, expected.name)
		}
		if companies[idx].Currency != "TRY" {
			t.Fatalf("companies[%d].Currency = %q, want TRY", idx, companies[idx].Currency)
		}
	}
}
