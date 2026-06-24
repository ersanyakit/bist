package kapingest

import (
	"path/filepath"
	"testing"
)

func TestSHA256FileStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.pdf")
	writeTestFile(t, path, "same content")
	left, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	right, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if left == "" || left != right {
		t.Fatalf("hash mismatch left=%q right=%q", left, right)
	}
}

func TestCheckpointSkipsProcessedHash(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewJSONLWriter(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	item := ProcessedFile{SHA256: "abc", FilePath: "x.pdf"}
	if err := writer.WriteProcessed(item); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	cp, err := LoadCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cp.IsProcessed("abc") {
		t.Fatalf("expected checkpoint to contain hash")
	}
	if cp.IsProcessed("missing") {
		t.Fatalf("unexpected processed hash")
	}
}
