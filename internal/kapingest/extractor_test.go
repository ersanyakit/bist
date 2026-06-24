package kapingest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const extractorCommandTestTimeout = 30 * time.Second

func TestPDFTextExtractorUsesOCRWhenPDFTextIsLowQuality(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binaries are Unix-only")
	}
	binDir := t.TempDir()
	writeFakeBinary(t, filepath.Join(binDir, "pdftotext"), `#!/bin/sh
printf "abc"
`)
	writeFakeBinary(t, filepath.Join(binDir, "pdftoppm"), `#!/bin/sh
prefix=""
for arg in "$@"; do
  prefix="$arg"
done
printf "fake image" > "$prefix-1.png"
`)
	writeFakeBinary(t, filepath.Join(binDir, "tesseract"), `#!/bin/sh
if [ "$1" = "--list-langs" ]; then
  printf "List of available languages\neng\n"
  exit 0
fi
printf "Finansal Durum Tablosu\nKar veya Zarar\nHasılat 12345\nGayrimenkul Değerleme Raporu\nÖzkaynaklar 50\n"
`)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	text, method, warnings, err := PDFTextExtractor{
		Timeout:    extractorCommandTestTimeout,
		OCRTimeout: extractorCommandTestTimeout,
	}.ExtractText(context.Background(), filepath.Join(t.TempDir(), "scan.pdf"))
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if !strings.Contains(method, "ocr") {
		t.Fatalf("method = %q, want OCR-backed extraction; warnings=%v text=%q", method, warnings, text)
	}
	if !strings.Contains(text, "Finansal Durum Tablosu") {
		t.Fatalf("OCR text not used: %q", text)
	}
	if !containsRawWarning(warnings, "ocr_fallback_used") {
		t.Fatalf("expected OCR warning, got %v", warnings)
	}
	if !containsRawWarning(warnings, "ocr_turkish_language_missing") {
		t.Fatalf("expected missing Turkish OCR language warning, got %v", warnings)
	}
}

func TestCoordinateRowsFromTSVBuildsCellSeparatedAssetRows(t *testing.T) {
	tsv := strings.Join([]string{
		"level\tpage_num\tpar_num\tblock_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext",
		"5\t1\t0\t0\t0\t0\t10\t100\t38\t10\t100\tBodrum",
		"5\t1\t0\t0\t0\t1\t53\t100\t42\t10\t100\tHillside",
		"5\t1\t0\t0\t0\t2\t101\t100\t22\t10\t100\tOtel",
		"5\t1\t0\t0\t0\t3\t220\t100\t34\t10\t100\tMuğla",
		"5\t1\t0\t0\t0\t4\t258\t100\t5\t10\t100\t/",
		"5\t1\t0\t0\t0\t5\t267\t100\t42\t10\t100\tBodrum",
		"5\t1\t0\t0\t0\t6\t390\t100\t34\t10\t100\t41.830",
		"5\t1\t0\t0\t0\t7\t428\t100\t10\t10\t100\tm²",
		"5\t1\t0\t0\t0\t8\t510\t100\t86\t10\t100\t7.873.542.000",
		"5\t1\t0\t0\t0\t9\t600\t100\t12\t10\t100\tTL",
	}, "\n")
	got := coordinateRowsFromTSV(tsv)
	want := "Bodrum Hillside Otel\tMuğla / Bodrum\t41.830 m²\t7.873.542.000 TL"
	if got != want {
		t.Fatalf("coordinateRowsFromTSV() = %q, want %q", got, want)
	}
}

func TestPDFTextExtractorAppendsCoordinateTSVText(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binaries are Unix-only")
	}
	binDir := t.TempDir()
	writeFakeBinary(t, filepath.Join(binDir, "pdftotext"), `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "-tsv" ]; then
    printf "level\tpage_num\tpar_num\tblock_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n"
    printf "5\t1\t0\t0\t0\t0\t10\t100\t38\t10\t100\tBodrum\n"
    printf "5\t1\t0\t0\t0\t1\t53\t100\t42\t10\t100\tHillside\n"
    printf "5\t1\t0\t0\t0\t2\t101\t100\t22\t10\t100\tOtel\n"
    printf "5\t1\t0\t0\t0\t3\t220\t100\t34\t10\t100\tMuğla\n"
    printf "5\t1\t0\t0\t0\t4\t258\t100\t5\t10\t100\t/\n"
    printf "5\t1\t0\t0\t0\t5\t267\t100\t42\t10\t100\tBodrum\n"
    exit 0
  fi
done
printf "Finansal Durum Tablosu\nKar veya Zarar\nHasılat 12345\nGayrimenkul Değerleme Raporu\nÖzkaynaklar 50\n"
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	text, method, warnings, err := PDFTextExtractor{Timeout: extractorCommandTestTimeout}.ExtractText(context.Background(), filepath.Join(t.TempDir(), "native.pdf"))
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if method != "pdftotext+tsv" {
		t.Fatalf("method = %q, warnings=%v", method, warnings)
	}
	if !strings.Contains(text, "###COORDINATE_TABLE_TEXT###") || !strings.Contains(text, "Bodrum Hillside Otel") {
		t.Fatalf("coordinate text missing: %q", text)
	}
}

func TestPDFTextExtractorUsesVisionWhenOCRRemainsLowQuality(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binaries are Unix-only")
	}
	binDir := t.TempDir()
	writeFakeBinary(t, filepath.Join(binDir, "pdftotext"), `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "-tsv" ]; then
    printf "level\tpage_num\tleft\ttop\twidth\theight\ttext\n"
    exit 0
  fi
done
printf "abc"
`)
	writeFakeBinary(t, filepath.Join(binDir, "pdftoppm"), `#!/bin/sh
prefix=""
for arg in "$@"; do
  prefix="$arg"
done
printf "fake image" > "$prefix-1.png"
`)
	writeFakeBinary(t, filepath.Join(binDir, "tesseract"), `#!/bin/sh
if [ "$1" = "--list-langs" ]; then
  printf "List of available languages\neng\n"
  exit 0
fi
printf "xx"
`)
	visionCmd := filepath.Join(binDir, "vision_extract")
	writeFakeBinary(t, visionCmd, `#!/bin/sh
test -n "$KAP_VISION_PDF" || exit 2
test -d "$KAP_VISION_IMAGE_DIR" || exit 3
printf "Sayfa 1\nFinansal Durum Tablosu\nHasılat\t12345\tTL\nÖzkaynaklar\t50000\tTL\nBağımsız denetimden geçmiştir\nKonsolide finansal tablolar\n"
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	text, method, warnings, err := PDFTextExtractor{
		Timeout:        extractorCommandTestTimeout,
		OCRTimeout:     extractorCommandTestTimeout,
		EnableVision:   true,
		VisionCommand:  visionCmd,
		VisionTimeout:  extractorCommandTestTimeout,
		VisionMaxPages: 2,
	}.ExtractText(context.Background(), filepath.Join(t.TempDir(), "scan.pdf"))
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if !strings.Contains(method, "visionlm") {
		t.Fatalf("method = %q, want VisionLM-backed extraction; warnings=%v text=%q", method, warnings, text)
	}
	if !strings.Contains(text, "###VISIONLM_TEXT###") || !strings.Contains(text, "Hasılat") {
		t.Fatalf("vision text not merged: %q", text)
	}
	if !containsRawWarning(warnings, "visionlm_fallback_used") {
		t.Fatalf("expected VisionLM fallback warning, got %v", warnings)
	}
}

func writeFakeBinary(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake binary %s: %v", path, err)
	}
}
