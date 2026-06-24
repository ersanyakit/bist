package documentingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

type FileDigest struct {
	Algorithm string `json:"algorithm"`
	Hex       string `json:"hex"`
	SizeBytes int64  `json:"size_bytes"`
}

func SHA256File(path string) (FileDigest, error) {
	file, err := os.Open(path)
	if err != nil {
		return FileDigest{}, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return FileDigest{}, err
	}
	return FileDigest{
		Algorithm: "sha256",
		Hex:       hex.EncodeToString(hash.Sum(nil)),
		SizeBytes: size,
	}, nil
}
