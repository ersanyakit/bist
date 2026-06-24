package kapingest

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Checkpoint struct {
	mu        sync.Mutex
	processed map[string]ProcessedFile
}

func NewCheckpoint() *Checkpoint {
	return &Checkpoint{processed: map[string]ProcessedFile{}}
}

func LoadCheckpoint(outputDir string) (*Checkpoint, error) {
	cp := NewCheckpoint()
	path := filepath.Join(outputDir, ProcessedFilesFile)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return cp, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item ProcessedFile
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		if item.SHA256 != "" {
			cp.processed[item.SHA256] = item
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cp, nil
}

func (c *Checkpoint) IsProcessed(sha256 string) bool {
	if c == nil || sha256 == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.processed[sha256]
	return ok
}

func (c *Checkpoint) Reserve(sha256 string) bool {
	if c == nil || sha256 == "" {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.processed[sha256]; ok {
		return false
	}
	c.processed[sha256] = ProcessedFile{SHA256: sha256}
	return true
}

func (c *Checkpoint) Release(sha256 string) {
	if c == nil || sha256 == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.processed[sha256]; ok && item.FilePath == "" {
		delete(c.processed, sha256)
	}
}

func (c *Checkpoint) Mark(item ProcessedFile) {
	if c == nil || item.SHA256 == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processed[item.SHA256] = item
}
