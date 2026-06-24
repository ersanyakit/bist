package kapingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/domain"
)

func TestBuildCanonicalFinancialsFromOutput(t *testing.T) {
	dir := t.TempDir()
	tickerDir := filepath.Join(dir, "by_ticker", "TEST")
	if err := os.MkdirAll(tickerDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	period := "2025/12"
	documentDate := "2026-02-19"
	fact := ExtractedFinancialFact{
		ID:                 "fact-revenue",
		Ticker:             "TEST",
		SourceFile:         "TEST finansal rapor.pdf",
		SHA256:             "abc123",
		DocumentDate:       &documentDate,
		Period:             &period,
		StatementType:      "income_statement",
		LineItemOriginal:   "Hasılat",
		LineItemNormalized: "revenue",
		Value:              3_400,
		Currency:           "TRY",
		Unit:               "thousand_try",
		Source:             DocumentFactSource{Page: 8, TableID: "tbl-income", Snippet: "Hasılat 3.400"},
		Confidence:         0.93,
		Certification: EvidenceCertification{
			Status:         EvidenceStatusCertified,
			Score:          100,
			AnalysisUsable: true,
		},
		CreatedAt: time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	competingFact := fact
	competingFact.ID = "fact-revenue-competing"
	competingFact.SourceFile = "TEST faaliyet raporu.pdf"
	competingFact.SHA256 = "def456"
	competingFact.Value = 3_900
	competingFact.Source = DocumentFactSource{Page: 19, TableID: "tbl-summary", Snippet: "Hasılat 3.900"}
	competingFact.Confidence = 0.99
	competingFact.ReviewRequired = true
	competingFact.Certification = EvidenceCertification{
		Status:         EvidenceStatusReviewRequired,
		Score:          65,
		AnalysisUsable: true,
	}
	raw, err := json.Marshal(fact)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	competingRaw, err := json.Marshal(competingFact)
	if err != nil {
		t.Fatalf("marshal competing fact: %v", err)
	}
	payload := append(append(append([]byte{}, raw...), '\n'), competingRaw...)
	payload = append(payload, '\n')
	if err := os.WriteFile(filepath.Join(tickerDir, FinancialFactsFile), payload, 0o644); err != nil {
		t.Fatalf("write facts: %v", err)
	}
	summary, err := BuildCanonicalFinancialsFromOutput(dir, func() time.Time {
		return time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("BuildCanonicalFinancialsFromOutput: %v", err)
	}
	if summary.FilesWritten != 1 || summary.FactsRead != 2 || summary.FactsAccepted != 1 || summary.FactsRejected != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Warnings) == 0 || summary.Warnings[0] != "kap_canonical_reconciled_competing_values_1:TEST" {
		t.Fatalf("expected reconciliation warning, got %+v", summary.Warnings)
	}
	var info domain.BilancoInfo
	out := filepath.Join(tickerDir, CanonicalFinancialsDir, CanonicalBilancoFile)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	value := info.Data["3C"].Years["2025"][0]
	if value == nil || *value != 3_400_000 {
		t.Fatalf("unexpected Q4 revenue: %v", value)
	}
}
