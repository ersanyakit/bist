package tcmb

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractDocumentTextsIndexesHTMLDocuments(t *testing.T) {
	root := t.TempDir()
	docDir := filepath.Join(root, "ppk_kararlari")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	htmlPath := filepath.Join(docDir, "ppk.html")
	rawHTML := `<html><head><style>.x{}</style><script>bad()</script></head><body><h1>Para Politikası Kurulu</h1><p>` +
		strings.Repeat("Enflasyon görünümü ve finansal koşullar yakından izlenmektedir. ", 8) +
		`</p></body></html>`
	if err := os.WriteFile(htmlPath, []byte(rawHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := ManifestEntry{
		Category:    "ppk_kararlari",
		URL:         "https://example.com/ppk.html",
		LocalPath:   htmlPath,
		ContentType: "text/html; charset=utf-8",
		FetchedAt:   time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
	}
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ManifestFile), append(rawManifest, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ExtractDocumentTexts(context.Background(), DocumentTextExtractOptions{OutputDir: root})
	if err != nil {
		t.Fatalf("ExtractDocumentTexts() error = %v", err)
	}
	if result.Documents != 1 || result.Extracted != 1 || result.Errors != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	entry := readSingleTCMBTextIndexEntry(t, result.TextIndexPath)
	if entry.Status != "ok" || entry.TextLength < minUsableTCMBTextLength {
		t.Fatalf("unexpected index entry: %+v", entry)
	}
	text, err := os.ReadFile(entry.TextPath)
	if err != nil {
		t.Fatalf("read text: %v", err)
	}
	got := string(text)
	if !strings.Contains(got, "Para Politikası Kurulu") || strings.Contains(got, "bad()") || strings.Contains(got, ".x{}") {
		t.Fatalf("HTML text not normalized correctly: %q", got)
	}
}

func TestStripHTMLTextRemovesMarkupAndKeepsText(t *testing.T) {
	got := StripHTMLText(`<div>Başlık&nbsp;A</div><script>x()</script><p>İkinci<br>satır</p>`)
	if strings.Contains(got, "<") || strings.Contains(got, "x()") {
		t.Fatalf("markup/script leaked: %q", got)
	}
	if !strings.Contains(got, "Başlık A") || !strings.Contains(got, "İkinci") || !strings.Contains(got, "satır") {
		t.Fatalf("expected text missing: %q", got)
	}
}

func readSingleTCMBTextIndexEntry(t *testing.T, path string) DocumentTextIndexEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open text index: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatalf("empty text index")
	}
	var entry DocumentTextIndexEntry
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal text index: %v", err)
	}
	if scanner.Scan() {
		t.Fatalf("expected one text index row")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan text index: %v", err)
	}
	return entry
}
