package professional

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hissebot/internal/kapingest"
)

func TestAnalyzeKAPPDFIngestLoadsSymbolProcessedOutput(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "data", "equities")
	outputDir := filepath.Join(root, "data", "processed", "algyo")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}
	rawDocs := []kapingest.RawDocument{
		{
			FilePath:          filepath.Join(equitiesDir, "ALGYO", "kap", "attachments", "degerleme.pdf"),
			SHA256:            "sha-1",
			Ticker:            "ALGYO",
			FileName:          "ALGYO_degerleme.pdf",
			DocumentTypeGuess: kapingest.DocumentValuationReport,
			Text:              strings.Repeat("Gayrimenkul degerleme raporu portfoy ekspertiz degeri 4.500.000.000 TL net aktif deger 3.200.000.000 TL portfoy toplam degeri 5.400.000.000 TL. ", 80),
			TextLength:        5000,
			QualityScore:      0.92,
		},
		{
			FilePath:          filepath.Join(equitiesDir, "ALGYO", "kap", "attachments", "finansal.pdf"),
			SHA256:            "sha-2",
			Ticker:            "ALGYO",
			FileName:          "ALGYO_finansal.pdf",
			DocumentTypeGuess: kapingest.DocumentFinancialStatement,
			Text:              strings.Repeat("Finansal durum tablosu ve kar veya zarar tablosu varliklar 12.500.000 TL yukumlulukler 5.250.000 TL ozkaynaklar 7.250.000 TL hasilat 1.100.000 TL. ", 60),
			TextLength:        3200,
			QualityScore:      0.81,
		},
		{
			FilePath:          filepath.Join(equitiesDir, "BIMAS", "kap", "attachments", "bimas.pdf"),
			SHA256:            "sha-3",
			Ticker:            "BIMAS",
			FileName:          "BIMAS.pdf",
			DocumentTypeGuess: kapingest.DocumentAnnualReport,
			Text:              "Baska sembol.",
			TextLength:        1000,
			QualityScore:      0.7,
		},
		{
			FilePath:          filepath.Join(equitiesDir, "ALGYO", "kap", "attachments", "scan.pdf"),
			SHA256:            "sha-4",
			Ticker:            "ALGYO",
			FileName:          "ALGYO_scan.pdf",
			DocumentTypeGuess: kapingest.DocumentUnknown,
			Text:              "",
			TextLength:        0,
			QualityScore:      0.1,
			Warnings:          []string{"low_text_quality_possible_scanned_pdf"},
		},
	}
	writeJSONL(t, filepath.Join(outputDir, kapingest.RawDocumentsFile), rawDocs)
	writeJSONL(t, filepath.Join(outputDir, kapingest.ProcessedFilesFile), []kapingest.ProcessedFile{
		{FilePath: rawDocs[0].FilePath, SHA256: "sha-1", Ticker: "ALGYO"},
		{FilePath: rawDocs[1].FilePath, SHA256: "sha-2", Ticker: "ALGYO"},
		{FilePath: rawDocs[2].FilePath, SHA256: "sha-3", Ticker: "BIMAS"},
		{FilePath: rawDocs[3].FilePath, SHA256: "sha-4", Ticker: "ALGYO"},
	})
	writeJSONL(t, filepath.Join(outputDir, kapingest.ExtractionErrorsFile), []kapingest.ExtractionError{
		{FilePath: rawDocs[3].FilePath, SHA256: "sha-4", Stage: "extract_text", Error: "test"},
	})

	got := analyzeKAPPDFIngest(equitiesDir, "ALGYO")
	if !got.Computed {
		t.Fatalf("expected computed ingest summary: %+v", got)
	}
	if got.TotalDocuments != 3 || got.UniqueProcessed != 3 || got.ErrorCount != 1 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if got.LowQualityCount != 1 {
		t.Fatalf("expected low quality count 1, got %+v", got)
	}
	if got.AnalysisUsableCount != 2 || got.ReviewRequiredCount != 1 || got.RejectedCount != 1 {
		t.Fatalf("unexpected quality gate counts: %+v", got)
	}
	if len(got.TypeCounts) == 0 || got.TypeCounts[0].Type != kapingest.DocumentValuationReport {
		t.Fatalf("expected valuation report to be prioritized in type counts: %+v", got.TypeCounts)
	}
	if len(got.ImportantDocuments) == 0 || !strings.Contains(got.ImportantDocuments[0].ContentSnippet, "Gayrimenkul") {
		t.Fatalf("expected important document snippet, got %+v", got.ImportantDocuments)
	}
	if !strings.Contains(got.Summary, "3 benzersiz KAP PDF") {
		t.Fatalf("summary does not include document count: %s", got.Summary)
	}
}

func TestAnalyzeKAPPDFIngestExplainsMissingRawDocumentsWhenSourcePDFsExist(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "data", "equities")
	pdfPath := filepath.Join(equitiesDir, "ASELS", "kap", "attachments", "2026", "1", "asels.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0o755); err != nil {
		t.Fatalf("mkdir source pdf dir: %v", err)
	}
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	got := analyzeKAPPDFIngest(equitiesDir, "ASELS")
	if got.Computed {
		t.Fatalf("missing raw documents should not be computed: %+v", got)
	}
	if got.SourcePDFCount != 1 {
		t.Fatalf("source pdf count = %d, want 1: %+v", got.SourcePDFCount, got)
	}
	if !strings.Contains(got.Summary, "1 PDF eki var fakat raw_documents.jsonl uretilmemis") {
		t.Fatalf("summary should explain source PDFs exist without ingest output: %s", got.Summary)
	}
	if !containsString(got.Warnings, "kap_pdf_source_files_exist_but_ingest_missing") {
		t.Fatalf("expected missing ingest warning, got %+v", got.Warnings)
	}
}

func writeJSONL[T any](t *testing.T, path string, rows []T) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jsonl %s: %v", path, err)
	}
	defer file.Close()
	for _, row := range rows {
		b, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("marshal jsonl row: %v", err)
		}
		if _, err := file.Write(append(b, '\n')); err != nil {
			t.Fatalf("write jsonl row: %v", err)
		}
	}
}
