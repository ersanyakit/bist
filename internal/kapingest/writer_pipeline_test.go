package kapingest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriterWritesValidJSONL(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewJSONLWriter(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteRaw(RawDocument{FilePath: "x.pdf", SHA256: "abc", CreatedAt: "2026-06-15T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(dir, RawDocumentsFile))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one jsonl line")
	}
	var doc RawDocument
	if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
		t.Fatalf("invalid jsonl: %v", err)
	}
	if doc.SHA256 != "abc" {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestPipelineContinuesAfterOneFileError(t *testing.T) {
	input := t.TempDir()
	output := t.TempDir()
	good := filepath.Join(input, "ASELS", "kap", "attachments", "2026", "1", "good.pdf")
	bad := filepath.Join(input, "ASELS", "kap", "attachments", "2026", "1", "bad.pdf")
	writeTestFile(t, good, "good")
	writeTestFile(t, bad, "bad")

	summary, err := Run(context.Background(), Options{
		InputDir:      input,
		OutputDir:     output,
		Workers:       2,
		Resume:        true,
		TextExtractor: fakeExtractor{},
		Now: func() time.Time {
			return time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Status != "partial" || summary.Errors != 1 || summary.RawDocuments != 1 || summary.ProcessedFiles != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(output, RawDocumentsFile)); err != nil {
		t.Fatalf("raw output missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, ExtractionErrorsFile)); err != nil {
		t.Fatalf("error output missing: %v", err)
	}
}

func TestPipelineRecordsPermanentPDFExtractionFailureAsRejectedRawDocument(t *testing.T) {
	input := t.TempDir()
	output := t.TempDir()
	bad := filepath.Join(input, "ASELS", "kap", "attachments", "2026", "1", "bad.pdf")
	writeTestFile(t, bad, "%PDF-1.6")

	summary, err := Run(context.Background(), Options{
		InputDir:      input,
		OutputDir:     output,
		Workers:       1,
		Resume:        true,
		TextExtractor: permanentPDFErrorExtractor{},
		Now: func() time.Time {
			return time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Status != "ok" || summary.Errors != 0 || summary.RawDocuments != 1 || summary.Rejected != 1 || summary.ProcessedFiles != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	file, err := os.Open(filepath.Join(output, RawDocumentsFile))
	if err != nil {
		t.Fatalf("raw output missing: %v", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one rejected raw document")
	}
	var doc RawDocument
	if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
		t.Fatalf("decode raw document: %v", err)
	}
	if doc.ParseStatus != ParseStatusRejected || doc.AnalysisUsable || !doc.HumanReviewRequired {
		t.Fatalf("unexpected rejected raw document: %+v", doc)
	}
	if !containsRawWarning(doc.Warnings, "pdf_structure_invalid_or_zero_pages") {
		t.Fatalf("expected invalid PDF warning, got %+v", doc.Warnings)
	}
}

func TestPermanentPDFExtractionFailureRecognizesBrokenCatalog(t *testing.T) {
	err := errors.New("pdftotext failed: exit status 1: Syntax Error: Catalog object is wrong type (stream)\nSyntax Error: Couldn't read page catalog")
	if !isPermanentPDFTextExtractionFailure(err) {
		t.Fatalf("expected broken page catalog to be permanent PDF extraction failure")
	}
	if got := extractionWarningForError(err); got != "pdf_catalog_pages_invalid" {
		t.Fatalf("warning = %q, want pdf_catalog_pages_invalid", got)
	}
}

type fakeExtractor struct{}

func (fakeExtractor) ExtractText(_ context.Context, filePath string) (string, string, []string, error) {
	if strings.Contains(filepath.Base(filePath), "bad") {
		return "", "fake", nil, errors.New("fake extract failure")
	}
	return "Finansal Durum Tablosu\nKar veya Zarar\nHasılat 100\nÖzkaynaklar 50", "fake", nil, nil
}

type permanentPDFErrorExtractor struct{}

func (permanentPDFErrorExtractor) ExtractText(context.Context, string) (string, string, []string, error) {
	return "", "pdftotext", nil, errors.New("pdftotext failed: Syntax Error: Catalog dictionary does not contain a valid \"Pages\" entry; Wrong page range given: the first page (1) can not be after the last page (0)")
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
