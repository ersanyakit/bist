package kapingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerFindsRecursivePDFs(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "ASELS", "kap", "attachments", "2026", "1", "rapor.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfPath, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ASELS", "note.txt"), []byte("txt"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := ScanPDFs(dir, 0)
	if err != nil {
		t.Fatalf("ScanPDFs() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1: %#v", len(files), files)
	}
	if files[0].Ticker != "ASELS" {
		t.Fatalf("ticker = %q, want ASELS", files[0].Ticker)
	}
}

func TestScannerSkipsGeneratedAnalysisPDFs(t *testing.T) {
	dir := t.TempDir()
	kapPath := filepath.Join(dir, "ALGYO", "kap", "attachments", "2026", "1", "kap.pdf")
	analysisPath := filepath.Join(dir, "ALGYO", "analysis", "2026-06-15", "rapor.pdf")
	writeTestScannerFile(t, kapPath, "kap")
	writeTestScannerFile(t, analysisPath, "generated report")

	files, err := ScanPDFs(dir, 0)
	if err != nil {
		t.Fatalf("ScanPDFs() error = %v", err)
	}
	if len(files) != 1 || files[0].FilePath != kapPath {
		t.Fatalf("expected only KAP pdf, got %#v", files)
	}
}

func TestExtractTickerUsesFolderThenFilename(t *testing.T) {
	got := ExtractTicker(filepath.Join("data", "equities"), filepath.Join("data", "equities", "ARZUM", "kap", "attachments", "x.pdf"))
	if got != "ARZUM" {
		t.Fatalf("folder ticker = %q", got)
	}
	got = ExtractTicker(filepath.Join("tmp", "input"), filepath.Join("tmp", "input", "2026", "ARZUM - 31.03.2026 Konsolide.pdf"))
	if got != "ARZUM" {
		t.Fatalf("filename ticker = %q", got)
	}
	got = ExtractTicker(filepath.Join("tmp", "input"), filepath.Join("tmp", "input", "2026", "SPK Onayli Tadil Metni.pdf"))
	if got != "" {
		t.Fatalf("should not infer ticker from generic filename, got %q", got)
	}
}

func writeTestScannerFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
