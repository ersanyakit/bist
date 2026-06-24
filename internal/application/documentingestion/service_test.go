package documentingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/domain/documents"
	"hissebot/internal/repositories/memory"
)

func TestIngestKAPAttachmentIsIdempotentBySource(t *testing.T) {
	repo := memory.New()
	now := time.Date(2026, 6, 14, 1, 0, 0, 0, time.UTC)
	service := Service{Repository: repo, Now: func() time.Time { return now }}
	path := writeFile(t, filepath.Join(t.TempDir(), "obj_finansal_rapor.pdf"), "first content")

	input := KAPAttachmentInput{
		Ticker:           "test",
		DisclosureID:     "123",
		DisclosureIndex:  123,
		DisclosureDate:   now,
		DisclosureType:   "FR",
		Period:           "3AB",
		FiscalYear:       2026,
		LocalFilePath:    path,
		OriginalFilename: "finansal_rapor.pdf",
	}
	doc, saved, err := service.IngestKAPAttachment(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !saved {
		t.Fatal("first ingest should save")
	}
	if doc.Ticker != "TEST" || doc.DocumentType != documents.DocumentTypePDF || doc.Version != 1 || !doc.IsLatestVersion {
		t.Fatalf("unexpected document: %+v", doc)
	}

	again, saved, err := service.IngestKAPAttachment(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if saved {
		t.Fatal("second ingest of same source/checksum should skip")
	}
	if again.DocumentID != doc.DocumentID {
		t.Fatalf("idempotent ingest returned different document: %s != %s", again.DocumentID, doc.DocumentID)
	}
}

func TestArchiveLocalKAPAttachmentsBuildsDocumentMetadata(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	attachmentPath := filepath.Join(equitiesDir, "TEST", "kap", "attachments", "2026", "123", "obj-1_Finansal Rapor.pdf")
	writeFile(t, attachmentPath, "%PDF-1.7\nfinancial report\n")
	writeDetail(t, equitiesDir, "TEST", 123)

	repo := memory.New()
	now := time.Date(2026, 6, 14, 2, 0, 0, 0, time.UTC)
	service := Service{Repository: repo, Now: func() time.Time { return now }}
	result, err := service.ArchiveLocalKAPAttachments(context.Background(), ArchiveRequest{
		EquitiesDir: equitiesDir,
		Ticker:      "TEST",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.Status != documents.JobStatusSuccess || result.Job.DocumentsSaved != 1 || result.Job.DocumentsScanned != 1 {
		t.Fatalf("unexpected job: %+v errors=%+v", result.Job, result.Errors)
	}
	if len(result.Documents) != 1 {
		t.Fatalf("expected one document, got %d", len(result.Documents))
	}
	doc := result.Documents[0]
	if doc.KAPCompanyID != "mkk-1" || doc.DisclosureID != "disc-123" || doc.DisclosureIndex != 123 {
		t.Fatalf("KAP metadata not mapped: %+v", doc)
	}
	if doc.OriginalFilename != "Finansal Rapor.pdf" {
		t.Fatalf("original filename = %q", doc.OriginalFilename)
	}
	if doc.FileURL != "https://www.kap.org.tr/tr/api/file/download/obj-1" {
		t.Fatalf("file url = %q", doc.FileURL)
	}
	if doc.Language != documents.LanguageTR {
		t.Fatalf("language = %s", doc.Language)
	}
	list, err := repo.ListDocuments(context.Background(), "TEST")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("repository document count = %d", len(list))
	}
}

func TestIngestKAPAttachmentVersionsChangedFileAtSamePath(t *testing.T) {
	repo := memory.New()
	now := time.Date(2026, 6, 14, 3, 0, 0, 0, time.UTC)
	service := Service{Repository: repo, Now: func() time.Time { return now }}
	path := writeFile(t, filepath.Join(t.TempDir(), "obj_finansal_rapor.pdf"), "first content")
	input := KAPAttachmentInput{
		Ticker:           "TEST",
		DisclosureID:     "123",
		DisclosureIndex:  123,
		DisclosureDate:   now,
		DisclosureType:   "FR",
		LocalFilePath:    path,
		OriginalFilename: "finansal_rapor.pdf",
	}
	first, saved, err := service.IngestKAPAttachment(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !saved || first.Version != 1 || !first.IsLatestVersion {
		t.Fatalf("unexpected first document: saved=%v doc=%+v", saved, first)
	}

	if err := os.WriteFile(path, []byte("changed content"), 0o644); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	second, saved, err := service.IngestKAPAttachment(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !saved || second.Version != 2 || !second.IsLatestVersion || second.Checksum == first.Checksum {
		t.Fatalf("unexpected second document: saved=%v doc=%+v first=%+v", saved, second, first)
	}
	docs, err := repo.ListDocuments(context.Background(), "TEST")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("documents = %d, want 2", len(docs))
	}
	latest := 0
	for _, doc := range docs {
		if doc.IsLatestVersion {
			latest++
			if doc.DocumentID != second.DocumentID {
				t.Fatalf("latest document = %s, want %s", doc.DocumentID, second.DocumentID)
			}
		}
	}
	if latest != 1 {
		t.Fatalf("latest document count = %d, want 1", latest)
	}
}

func TestIngestKAPAttachmentRejectsEmptyFile(t *testing.T) {
	repo := memory.New()
	now := time.Date(2026, 6, 14, 4, 0, 0, 0, time.UTC)
	service := Service{Repository: repo, Now: func() time.Time { return now }}
	path := writeFile(t, filepath.Join(t.TempDir(), "empty.pdf"), "")

	_, saved, err := service.IngestKAPAttachment(context.Background(), KAPAttachmentInput{
		Ticker:           "TEST",
		DisclosureID:     "123",
		LocalFilePath:    path,
		OriginalFilename: "empty.pdf",
	})
	if err == nil {
		t.Fatal("expected empty file error")
	}
	if saved {
		t.Fatal("empty file must not be saved")
	}
}

func writeFile(t *testing.T, path string, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeDetail(t *testing.T, equitiesDir, ticker string, disclosureIndex int) {
	t.Helper()
	path := filepath.Join(equitiesDir, "_kap", "details", ticker, "123.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{
  "source": "kap_attachment_detail",
  "raw": [
    {
      "disclosure": {
        "disclosureBasic": {
          "mkkMemberOid": "mkk-1",
          "disclosureId": "disc-123",
          "disclosureIndex": ` + "123" + `,
          "disclosureType": "FR",
          "period": "3AB",
          "year": 2026,
          "publishDate": "2026.06.13 18:01:02"
        }
      },
      "attachments": [
        {"objId": "obj-1", "fileName": "Finansal Rapor.pdf", "fileExtension": "pdf"}
      ]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = disclosureIndex
}
