package professional

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hissebot/internal/domain"
)

func TestLoadFinancialsForSymbolFallsBackToKAPBilanco(t *testing.T) {
	equitiesDir := filepath.Join(t.TempDir(), "equities")
	writeEligibleBilancoForTest(t, filepath.Join(equitiesDir, "TEST", "financials", "kap_bilanco.json"), "kap_extraction_result")
	fin, ok := loadFinancialsForSymbol(equitiesDir, "TEST")
	if !ok {
		t.Fatalf("expected kap_bilanco fallback")
	}
	if fin.Source != "kap_extraction_result" {
		t.Fatalf("source=%q, want kap_extraction_result", fin.Source)
	}
}

func TestLoadFinancialsForSymbolFallsBackToProcessedKAPBilanco(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "data", "equities")
	processedPath := filepath.Join(root, "data", "processed", "by_ticker", "TEST", "kap_financials", "bilanco.json")
	writeEligibleBilancoForTest(t, processedPath, "kapingest_document_intelligence")
	fin, ok := loadFinancialsForSymbol(equitiesDir, "TEST")
	if !ok {
		t.Fatalf("expected processed kap bilanco fallback")
	}
	if fin.Source != "kapingest_document_intelligence" {
		t.Fatalf("source=%q, want kapingest_document_intelligence", fin.Source)
	}
}

func TestLoadFinancialsForSymbolRejectsInconsistentProcessedKAPBilanco(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "data", "equities")
	processedPath := filepath.Join(root, "data", "processed", "by_ticker", "TEST", "kap_financials", "bilanco.json")
	writeBilancoForTest(t, processedPath, "kapingest_document_intelligence", domain.FinancialDataQuality{
		FinanciallyConsistent: false,
		PeriodCount:           1,
	})

	if fin, ok := loadFinancialsForSymbol(equitiesDir, "TEST"); ok {
		t.Fatalf("inconsistent processed KAP PDF fallback must not become main financials: %+v", fin)
	}
}

func writeEligibleBilancoForTest(t *testing.T, path, source string) {
	t.Helper()
	writeBilancoForTest(t, path, source, domain.FinancialDataQuality{
		BacktestSafe:          true,
		FinanciallyConsistent: true,
		PublishDateCoverage:   1,
		AvailableAtCoverage:   1,
		PeriodCount:           1,
	})
}

func writeBilancoForTest(t *testing.T, path, source string, quality domain.FinancialDataQuality) {
	t.Helper()
	value := 1000.0
	info := domain.BilancoInfo{
		Ticker:   "TEST",
		Source:   source,
		Currency: "TRY",
		Quality:  quality,
		Data: map[string]domain.BilancoField{
			"1BL": {DescTR: "TOPLAM VARLIKLAR", Years: map[string][]*float64{"2025": {&value, nil, nil, nil}}},
		},
	}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
