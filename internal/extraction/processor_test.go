package extraction

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/domain/documents"
	"hissebot/internal/repositories/memory"
)

func TestProcessorExtractsSourceBackedCandidates(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	docPath := writeFixture(t, filepath.Join(equitiesDir, "TEST", "kap", "attachments", "2026", "123", "obj-1_rapor.html"), `
		<html><body>
		<p>Hasılat 1.000.000 TL olarak gerçekleşmiştir.</p>
		<p>Yönetim Kurulu Başkanı Ali Veli seçilmiştir.</p>
		<p>Şirket 2001 yılında Ankara arsa satın alımı yapmıştır.</p>
		</body></html>
	`)
	writeFixture(t, filepath.Join(equitiesDir, "TEST", "kap.json"), `{
		"kapMemberTitle": "TEST A.Ş.",
		"kapMemberOid": "kap-1",
		"cityName": "ISTANBUL",
		"paidCapital": 1000000
	}`)
	repo := memory.New()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	doc := documents.DocumentMetadata{
		DocumentID:        "kap_doc_test",
		CompanyID:         "TEST",
		Ticker:            "TEST",
		DisclosureID:      "123",
		DisclosureIndex:   123,
		DisclosureDate:    now,
		DocumentType:      documents.DocumentTypeHTML,
		LocalFilePath:     docPath,
		OriginalFilename:  "rapor.html",
		Checksum:          "checksum",
		ChecksumAlgorithm: "sha256",
		SizeBytes:         100,
		Language:          documents.LanguageTR,
		SourceSystem:      documents.SourceSystemKAP,
		Version:           1,
		IsLatestVersion:   true,
		ExtractionStatus:  documents.ExtractionStatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repo.SaveDocument(context.Background(), doc); err != nil {
		t.Fatal(err)
	}

	processor := Processor{
		Repository:          repo,
		EquitiesDir:         equitiesDir,
		MaxCharsPerDocument: 5000,
		Now:                 func() time.Time { return now },
	}
	batch, err := processor.ProcessBatch(context.Background(), Options{Ticker: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	if batch.DocumentsProcessed != 1 || batch.FinancialFacts == 0 || batch.ReviewItems == 0 {
		t.Fatalf("unexpected batch: %+v", batch)
	}
	if len(batch.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(batch.Results))
	}
	result := batch.Results[0]
	if result.CompanyInfoCard.CompanyName != "TEST A.Ş." || result.CompanyInfoCard.ReviewRequired {
		t.Fatalf("company info card not populated: %+v", result.CompanyInfoCard)
	}
	if len(result.FinancialFacts) == 0 || result.FinancialFacts[0].Source.SourceDocumentID != doc.DocumentID {
		t.Fatalf("financial fact source missing: %+v", result.FinancialFacts)
	}
	if len(result.CorporateEvents) == 0 || len(result.TrackedAssets) == 0 {
		t.Fatalf("expected event and tracked asset candidates: events=%+v assets=%+v", result.CorporateEvents, result.TrackedAssets)
	}
	if _, err := os.Stat(filepath.Join(equitiesDir, "TEST", "kap", "extraction", "extraction_result.json")); err != nil {
		t.Fatalf("expected extraction result JSON: %v", err)
	}
	docs, err := repo.ListDocuments(context.Background(), "TEST")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].ExtractionStatus != documents.ExtractionStatusTextReady {
		t.Fatalf("document status not updated: %+v", docs)
	}
}

func TestProcessorRoutesOCRImageToReview(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	docPath := writeFixture(t, filepath.Join(equitiesDir, "TEST", "kap", "attachments", "2026", "124", "obj-2_scan.png"), "png")
	repo := memory.New()
	now := time.Date(2026, 6, 14, 13, 0, 0, 0, time.UTC)
	doc := documents.DocumentMetadata{
		DocumentID:        "kap_doc_ocr",
		Ticker:            "TEST",
		DisclosureID:      "124",
		DisclosureDate:    now,
		DocumentType:      documents.DocumentTypeOCRImage,
		LocalFilePath:     docPath,
		OriginalFilename:  "scan.png",
		Checksum:          "checksum",
		ChecksumAlgorithm: "sha256",
		SizeBytes:         3,
		Language:          documents.LanguageTR,
		SourceSystem:      documents.SourceSystemKAP,
		Version:           1,
		IsLatestVersion:   true,
		ExtractionStatus:  documents.ExtractionStatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repo.SaveDocument(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	processor := Processor{Repository: repo, EquitiesDir: equitiesDir, Now: func() time.Time { return now }}
	batch, err := processor.ProcessBatch(context.Background(), Options{Ticker: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	if batch.ReviewItems != 1 || len(batch.Results) != 1 || len(batch.Results[0].FinancialFacts) != 0 {
		t.Fatalf("unexpected OCR batch: %+v", batch)
	}
	docs, err := repo.ListDocuments(context.Background(), "TEST")
	if err != nil {
		t.Fatal(err)
	}
	if docs[0].ExtractionStatus != documents.ExtractionStatusNeedsOCR || !docs[0].ReviewRequired {
		t.Fatalf("OCR document status not set: %+v", docs[0])
	}
}

func writeFixture(t *testing.T, path string, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
