package docintel

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAnalyzeClassifiesKAPAttachments(t *testing.T) {
	clearLLMEnv(t)
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	writeTestFile(t, filepath.Join(equitiesDir, "TEST", "kap", "attachments", "2025", "100", "obj_TEST 31.12.2024 Finansal Rapor.pdf"))
	writeTestFile(t, filepath.Join(equitiesDir, "TEST", "kap", "attachments", "2025", "101", "obj_TEST Faaliyet Raporu.pdf"))
	writeTestFile(t, filepath.Join(equitiesDir, "TEST", "kap", "attachments", "2025", "102", "obj_TEST BDR.pdf"))
	writeTestFile(t, filepath.Join(equitiesDir, "TEST", "kap", "attachments", "2025", "103", "obj_Halka Arz Fiyatının Belirlenmesinde Esas Alınan Varsayımlara İlişkin Değerlendirme Raporu.pdf"))
	writeDetail(t, equitiesDir, "TEST", 100, "Finansal Rapor", "FR", "FR", "2025.03.10 18:01:02")
	writeDetail(t, equitiesDir, "TEST", 101, "Faaliyet Raporu", "CA", "ODA", "2025.03.11 18:01:02")
	writeDetail(t, equitiesDir, "TEST", 102, "Bağımsız Denetim Raporu", "FR", "FR", "2025.03.12 18:01:02")
	writeDetail(t, equitiesDir, "TEST", 103, "Varsayım Değerlendirme Raporu", "ODA", "ODA", "2025.03.13 18:01:02")

	report := Analyze(Input{EquitiesDir: equitiesDir, Symbol: "TEST", Limit: 10})

	if !report.Computed {
		t.Fatalf("expected computed document report: %+v", report)
	}
	if report.TotalFiles != 4 || report.PDFCount != 4 {
		t.Fatalf("unexpected file counts: %+v", report)
	}
	if report.CoverageScore < 80 {
		t.Fatalf("coverage score too low: %.0f", report.CoverageScore)
	}
	for _, category := range []string{"financial_report", "activity_report", "audit_report", "valuation_report"} {
		if !hasCategory(report, category) {
			t.Fatalf("missing category %s in %+v", category, report.Categories)
		}
	}
	if len(report.MissingKeyDocuments) != 0 {
		t.Fatalf("did not expect missing key documents: %+v", report.MissingKeyDocuments)
	}
	if !strings.Contains(report.Summary, "4 KAP eki tarandı") {
		t.Fatalf("summary does not describe scan: %s", report.Summary)
	}
	if report.LLMAnalysis.Status != "not_configured" || report.LLMAnalysis.Computed {
		t.Fatalf("LLM should not be faked without config: %+v", report.LLMAnalysis)
	}
}

func TestAnalyzeReportsMissingKAPAttachmentFolder(t *testing.T) {
	clearLLMEnv(t)
	report := Analyze(Input{EquitiesDir: t.TempDir(), Symbol: "NONE"})
	if report.Computed {
		t.Fatalf("missing folder should not compute: %+v", report)
	}
	if len(report.MissingKeyDocuments) == 0 {
		t.Fatalf("missing folder should be visible: %+v", report)
	}
}

func clearLLMEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HISSEBOT_LLM_PROVIDER", "HISSEBOT_LLM_ENDPOINT", "HISSEBOT_LLM_MODEL", "HISSEBOT_LLM_API_KEY", "HISSEBOT_LLM_DOC_LIMIT"} {
		t.Setenv(key, "")
	}
}

func hasCategory(report Report, category string) bool {
	for _, item := range report.Categories {
		if item.Category == category {
			return true
		}
	}
	return false
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeDetail(t *testing.T, equitiesDir, symbol string, index int, title, typ, class, publishDate string) {
	t.Helper()
	path := filepath.Join(equitiesDir, "_kap", "details", symbol, strconv.Itoa(index)+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir detail: %v", err)
	}
	raw := `{
  "raw": [
    {
      "disclosure": {
        "disclosureBasic": {
          "title": "` + title + `",
          "disclosureType": "` + typ + `",
          "disclosureClass": "` + class + `",
          "publishDate": "` + publishDate + `",
          "year": 2025,
          "period": "3AB"
        }
      }
    }
  ]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write detail: %v", err)
	}
}
