package analysisreadiness

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/domain/kapextract"
	"hissebot/internal/services/classification"
	"hissebot/internal/storage"
)

func TestRunDecisionReadyWhenCoreEvidenceExists(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)
	store := storage.NewEquityStore(cfg.EquitiesDir)
	if err := store.Save(&domain.Equity{Ticker: "TEST", AssetType: domain.AssetTypeEquity}); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(cfg.DataDir, "bist", "bist_ohlcv.sqlite")
	writePriceDB(t, dbPath, "TEST", 130, "2026-06-24")
	writeSectorFile(t, cfg.SectorClassificationsFile)
	writeExtraction(t, cfg.EquitiesDir, "TEST")
	writeCorporateActions(t, cfg.EquitiesDir, "TEST")
	writeMacroFiles(t, cfg)

	report, err := Run(context.Background(), cfg, store, Options{
		Symbols: []string{"TEST"},
		Mode:    "research",
		Now:     fixedNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusDecisionReady {
		t.Fatalf("status = %s, want %s; report=%+v", report.Status, StatusDecisionReady, report.Results[0])
	}
	got := report.Results[0]
	if got.Price.AnalysisReadyRows != 130 {
		t.Fatalf("analysis-ready rows = %d, want 130", got.Price.AnalysisReadyRows)
	}
	if got.Financials.Fields < 4 || got.Financials.Periods < 1 {
		t.Fatalf("financial coverage too thin: %+v", got.Financials)
	}
	if got.Sector.PeerCount < 3 {
		t.Fatalf("peer coverage too thin: %+v", got.Sector)
	}
}

func TestRunBlocksWhenPriceAndKAPAreMissing(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)
	store := storage.NewEquityStore(cfg.EquitiesDir)
	if err := store.Save(&domain.Equity{Ticker: "MISS", AssetType: domain.AssetTypeEquity}); err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), cfg, store, Options{
		Symbols: []string{"MISS"},
		Mode:    "production",
		Now:     fixedNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusBlocked {
		t.Fatalf("status = %s, want %s", report.Status, StatusBlocked)
	}
	if report.Blocked != 1 {
		t.Fatalf("blocked = %d, want 1", report.Blocked)
	}
	if !hasString(report.Results[0].Blockers, "official_bist_price") || !hasString(report.Results[0].Blockers, "kap_extraction") {
		t.Fatalf("missing expected blockers: %+v", report.Results[0].Blockers)
	}
}

func testConfig(root string) config.Config {
	dataDir := filepath.Join(root, "data")
	return config.Config{
		DataDir:                   dataDir,
		EquitiesDir:               filepath.Join(dataDir, "equities"),
		SectorClassificationsFile: filepath.Join(dataDir, "seed", "sector_classifications.json"),
		TUIKGDPFile:               filepath.Join(dataDir, "macro", "tuik_gdp.json"),
		TUIKInflationFile:         filepath.Join(dataDir, "macro", "tuik_inflation_indices.json"),
		VAPIndexPortfolioFile:     filepath.Join(dataDir, "macro", "vap", "bist_endeks_portfoy.json"),
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
}

func writePriceDB(t *testing.T, path, symbol string, rows int, latest string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE daily_ohlcv (symbol TEXT, trading_date TEXT, analysis_ready INTEGER, is_adjusted INTEGER)`); err != nil {
		t.Fatal(err)
	}
	latestDate, err := time.Parse("2006-01-02", latest)
	if err != nil {
		t.Fatal(err)
	}
	for i := rows - 1; i >= 0; i-- {
		day := latestDate.AddDate(0, 0, -i).Format("2006-01-02")
		if _, err := db.Exec(`INSERT INTO daily_ohlcv(symbol, trading_date, analysis_ready, is_adjusted) VALUES (?, ?, 1, 1)`, symbol, day); err != nil {
			t.Fatal(err)
		}
	}
}

func writeSectorFile(t *testing.T, path string) {
	t.Helper()
	file := classification.File{
		Entries: map[string]classification.Entry{
			"TEST": {
				Sector:      "Sanayi",
				Industry:    "Test Endustri",
				PeerGroup:   "test_peer_group",
				PeerSymbols: []string{"AAA", "BBB", "CCC"},
				Source:      "test",
				Confidence:  0.95,
			},
		},
	}
	writeJSON(t, path, file)
}

func writeExtraction(t *testing.T, equitiesDir, symbol string) {
	t.Helper()
	createdAt := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	lines := []struct {
		id   string
		line string
		val  float64
		stmt kapextract.StatementType
	}{
		{"assets", "total_assets", 1000, kapextract.StatementBalanceSheet},
		{"equity", "equity", 650, kapextract.StatementBalanceSheet},
		{"cash", "cash", 120, kapextract.StatementBalanceSheet},
		{"revenue", "revenue", 500, kapextract.StatementIncomeStatement},
		{"income", "net_income", 80, kapextract.StatementIncomeStatement},
	}
	result := kapextract.ExtractionResult{
		Ticker:      symbol,
		GeneratedAt: createdAt,
		Documents: []kapextract.DocumentSummary{{
			DocumentID:       "doc1",
			Ticker:           symbol,
			DocumentType:     "pdf",
			LocalFilePath:    "doc1.pdf",
			OriginalFilename: "doc1.pdf",
			ExtractionStatus: "text_ready",
			ProcessedAt:      createdAt,
		}},
		TextBlocks: []kapextract.TextBlock{{BlockID: "block1", DocumentID: "doc1", Text: "financial table", CreatedAt: createdAt}},
	}
	for _, line := range lines {
		result.FinancialFacts = append(result.FinancialFacts, kapextract.FinancialFact{
			FactID:             line.id,
			Ticker:             symbol,
			Period:             "2025-Q4",
			FiscalYear:         2025,
			StatementType:      line.stmt,
			LineItemOriginal:   line.line,
			LineItemNormalized: line.line,
			Value:              line.val,
			Currency:           "TRY",
			Unit:               "TRY",
			Source: kapextract.SourceRef{
				SourceDocumentID: "doc1",
				SourceSystem:     "KAP",
				Ticker:           symbol,
				SourcePage:       1,
				ConfidenceScore:  0.92,
			},
			ConfidenceScore:  0.92,
			ValidationStatus: kapextract.ValidationValid,
			CreatedAt:        createdAt,
		})
	}
	writeJSON(t, filepath.Join(equitiesDir, symbol, "kap", "extraction", "extraction_result.json"), result)
}

func writeCorporateActions(t *testing.T, equitiesDir, symbol string) {
	t.Helper()
	effective := "2026-04-01T00:00:00Z"
	actions := []map[string]any{{
		"id":             "ca1",
		"symbol":         symbol,
		"type":           "dividend",
		"status":         "verified",
		"effective_date": effective,
		"cash_amount":    1.25,
		"currency":       "TRY",
		"source":         "test",
	}}
	writeJSON(t, filepath.Join(equitiesDir, symbol, "corporate_actions.json"), actions)
}

func writeMacroFiles(t *testing.T, cfg config.Config) {
	t.Helper()
	writeJSON(t, cfg.TUIKGDPFile, map[string]any{"ok": true})
	writeJSON(t, cfg.TUIKInflationFile, map[string]any{"ok": true})
	writeJSON(t, cfg.VAPIndexPortfolioFile, map[string]any{"ok": true})
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
