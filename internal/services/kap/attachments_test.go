package kap

import (
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
)

func TestSyncAttachmentsDownloadsMissingFileAndSkipsExisting(t *testing.T) {
	pdf := []byte("%PDF-1.7\nfake kap attachment\n")
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("missing User-Agent header for %s", r.URL.Path)
		}
		requests[r.URL.Path]++
		switch r.URL.Path {
		case "/tr/api/notification/attachment-detail/123":
			_, _ = w.Write([]byte(`[{
				"disclosure": {
					"disclosureBasic": {"stockCode": "TEST", "disclosureIndex": 123, "attachmentCount": 1},
					"disclosureDetail": {}
				},
				"disclosureBody": ["body"],
				"attachments": [{"objId": "obj-1", "fileName": "rapor.pdf", "fileExtension": "pdf"}]
			}]`))
		case "/tr/api/file/download/obj-1":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(wrapJavaByteArray(pdf))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(root, "equities"))
	if err := writeJSON(store.KAPDisclosuresPath("TEST"), []FinancialDisclosure{{
		Ticker:          "TEST",
		Title:           "Finansal Rapor",
		DisclosureIndex: 123,
		AttachmentCount: 1,
		PublishDate:     timePtr(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)),
	}}); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "kap")
	opts := AttachmentSyncOptions{
		OutputDir:    out,
		BaseURL:      server.URL,
		Ticker:       "TEST",
		Limit:        10,
		Delay:        time.Nanosecond,
		MinFreeBytes: 1,
	}
	result, err := SyncAttachments(context.Background(), config.Config{DataDir: root, HTTPTimeout: time.Second}, store, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesDownloaded != 1 || result.FilesSkipped != 0 || result.DetailsFetched != 1 {
		t.Fatalf("unexpected first result: %+v", result)
	}
	files, err := filepath.Glob(filepath.Join(store.Root(), "TEST", "kap", "attachments", "2026", "123", "*_rapor.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one downloaded file, got %d", len(files))
	}
	got, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(pdf) {
		t.Fatalf("downloaded file was not unwrapped, got %q", string(got))
	}
	legacyFiles, err := filepath.Glob(filepath.Join(out, "attachments", "TEST", "2026", "123", "*_rapor.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyFiles) != 0 {
		t.Fatalf("expected no legacy KAP attachment copy, got %d", len(legacyFiles))
	}

	result, err = SyncAttachments(context.Background(), config.Config{DataDir: root, HTTPTimeout: time.Second}, store, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesDownloaded != 0 || result.FilesSkipped != 1 || result.DetailsCached != 1 {
		t.Fatalf("unexpected second result: %+v", result)
	}
	if requests["/tr/api/file/download/obj-1"] != 1 {
		t.Fatalf("file endpoint should be called once, got %d", requests["/tr/api/file/download/obj-1"])
	}
}

func TestSyncAttachmentsSkipsDuplicateObjIDInSamePass(t *testing.T) {
	pdf := []byte("%PDF-1.7\nsame object\n")
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		switch r.URL.Path {
		case "/tr/api/notification/attachment-detail/123", "/tr/api/notification/attachment-detail/124":
			index := "123"
			if r.URL.Path == "/tr/api/notification/attachment-detail/124" {
				index = "124"
			}
			_, _ = w.Write([]byte(`[{
				"disclosure": {
					"disclosureBasic": {"stockCode": "TEST", "disclosureIndex": ` + index + `, "attachmentCount": 1},
					"disclosureDetail": {}
				},
				"disclosureBody": ["body"],
				"attachments": [{"objId": "obj-same", "fileName": "rapor.pdf", "fileExtension": "pdf"}]
			}]`))
		case "/tr/api/file/download/obj-same":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(wrapJavaByteArray(pdf))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(root, "equities"))
	if err := writeJSON(store.KAPDisclosuresPath("TEST"), []FinancialDisclosure{
		{
			Ticker:          "TEST",
			Title:           "Finansal Rapor 1",
			DisclosureIndex: 123,
			AttachmentCount: 1,
			PublishDate:     timePtr(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)),
		},
		{
			Ticker:          "TEST",
			Title:           "Finansal Rapor 2",
			DisclosureIndex: 124,
			AttachmentCount: 1,
			PublishDate:     timePtr(time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)),
		},
	}); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "kap")
	result, err := SyncAttachments(context.Background(), config.Config{DataDir: root, HTTPTimeout: time.Second}, store, AttachmentSyncOptions{
		OutputDir:    out,
		BaseURL:      server.URL,
		Ticker:       "TEST",
		Limit:        10,
		Delay:        time.Nanosecond,
		MinFreeBytes: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesDownloaded != 1 || result.FilesSkipped != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requests["/tr/api/file/download/obj-same"] != 1 {
		t.Fatalf("file endpoint should be called once, got %d", requests["/tr/api/file/download/obj-same"])
	}
	files, err := filepath.Glob(filepath.Join(store.Root(), "TEST", "kap", "attachments", "2026", "123", "*_rapor.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one canonical equity attachment file, got %d", len(files))
	}
	duplicateFiles, err := filepath.Glob(filepath.Join(store.Root(), "TEST", "kap", "attachments", "2026", "124", "*_rapor.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(duplicateFiles) != 0 {
		t.Fatalf("expected duplicate objID disclosure to be skipped, got %d files", len(duplicateFiles))
	}
	legacyFiles, err := filepath.Glob(filepath.Join(out, "attachments", "TEST", "2026", "*", "*_rapor.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyFiles) != 0 {
		t.Fatalf("expected no legacy KAP attachment copies, got %d", len(legacyFiles))
	}
	objects, err := filepath.Glob(filepath.Join(out, "objects", "ob", "obj-same.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 0 {
		t.Fatalf("expected no object cache copy, got %d", len(objects))
	}
}

func TestSyncAttachmentsReusesDetailCacheAcrossTickers(t *testing.T) {
	pdf := []byte("%PDF-1.7\nglobal detail cache\n")
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		switch r.URL.Path {
		case "/tr/api/notification/attachment-detail/123":
			_, _ = w.Write([]byte(`[{
				"disclosure": {
					"disclosureBasic": {"stockCode": "TEST", "disclosureIndex": 123, "attachmentCount": 1},
					"disclosureDetail": {}
				},
				"disclosureBody": ["body"],
				"attachments": [{"objId": "obj-detail-cache", "fileName": "rapor.pdf", "fileExtension": "pdf"}]
			}]`))
		case "/tr/api/file/download/obj-detail-cache":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(wrapJavaByteArray(pdf))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	store := storage.NewEquityStore(filepath.Join(root, "equities"))
	for _, ticker := range []string{"TEST", "TEST2"} {
		if err := store.Save(&domain.Equity{Ticker: ticker, AssetType: 2}); err != nil {
			t.Fatal(err)
		}
		if err := writeJSON(store.KAPDisclosuresPath(ticker), []FinancialDisclosure{{
			Ticker:          ticker,
			Title:           "Finansal Rapor",
			DisclosureIndex: 123,
			AttachmentCount: 1,
			PublishDate:     timePtr(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)),
		}}); err != nil {
			t.Fatal(err)
		}
	}

	out := filepath.Join(root, "kap")
	result, err := SyncAttachments(context.Background(), config.Config{DataDir: root, HTTPTimeout: time.Second}, store, AttachmentSyncOptions{
		OutputDir:    out,
		BaseURL:      server.URL,
		Delay:        time.Nanosecond,
		MinFreeBytes: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.DetailsFetched != 1 || result.DetailsCached != 1 {
		t.Fatalf("unexpected detail counts: %+v", result)
	}
	if result.FilesDownloaded != 1 || result.FilesSkipped != 1 {
		t.Fatalf("unexpected file counts: %+v", result)
	}
	if requests["/tr/api/notification/attachment-detail/123"] != 1 {
		t.Fatalf("detail endpoint should be called once, got %d", requests["/tr/api/notification/attachment-detail/123"])
	}
	if requests["/tr/api/file/download/obj-detail-cache"] != 1 {
		t.Fatalf("file endpoint should be called once, got %d", requests["/tr/api/file/download/obj-detail-cache"])
	}
}

func TestUnwrapKAPFileSupportsRawBytes(t *testing.T) {
	raw := []byte("%PDF-raw")
	got, err := unwrapKAPFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("got %q", string(got))
	}
}

func TestIsTransientAttachmentError(t *testing.T) {
	if !isTransientAttachmentError(errors.New("net/http: HTTP/1.x transport connection broken: unexpected EOF")) {
		t.Fatal("unexpected EOF should be transient")
	}
	if isTransientAttachmentError(errors.New("empty attachment objId")) {
		t.Fatal("validation errors should not be transient")
	}
}

func wrapJavaByteArray(body []byte) []byte {
	header := []byte{
		0xac, 0xed, 0x00, 0x05,
		0x75, 0x72, 0x00, 0x02,
		'[', 'B',
		0xac, 0xf3, 0x17, 0xf8, 0x06, 0x08, 0x54, 0xe0,
		0x02, 0x00, 0x00,
		0x78, 0x70,
		0x00, 0x00, 0x00, 0x00,
	}
	binary.BigEndian.PutUint32(header[23:27], uint32(len(body)))
	return append(header, body...)
}

func timePtr(value time.Time) *time.Time {
	return &value
}
