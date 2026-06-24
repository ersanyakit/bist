package documentingestion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSHA256File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(path, []byte("kap document"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Algorithm != "sha256" {
		t.Fatalf("algorithm = %s", got.Algorithm)
	}
	if got.Hex != "9e8db5482f09edc2ce783297a1f24977549dd9f96b3b04b9c345ce1553a9d356" {
		t.Fatalf("unexpected checksum: %s", got.Hex)
	}
	if got.SizeBytes != int64(len("kap document")) {
		t.Fatalf("size = %d", got.SizeBytes)
	}
}
