package filedocuments

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"hissebot/internal/domain/documents"
)

type Store struct {
	root      string
	mu        sync.Mutex
	documents map[string]documents.DocumentMetadata
	jobs      map[string]documents.IngestionJob
}

func New(root string) (*Store, error) {
	store := &Store{
		root:      root,
		documents: map[string]documents.DocumentMetadata{},
		jobs:      map[string]documents.IngestionJob{},
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) SaveDocument(_ context.Context, document documents.DocumentMetadata) error {
	if err := document.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, existing := range s.documents {
		if sameVersionGroup(existing, document) && existing.DocumentID != document.DocumentID && existing.IsLatestVersion {
			existing.IsLatestVersion = false
			existing.UpdatedAt = document.UpdatedAt
			s.documents[id] = existing
		}
	}
	s.documents[document.DocumentID] = document
	return s.writeDocumentsLocked()
}

func (s *Store) FindDocumentBySource(_ context.Context, sourceSystem documents.SourceSystem, ticker string, disclosureID string, localFilePath string) (documents.DocumentMetadata, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticker = documents.NormalizeTicker(ticker)
	var latest documents.DocumentMetadata
	found := false
	for _, document := range s.documents {
		if document.SourceSystem == sourceSystem && document.Ticker == ticker && document.DisclosureID == disclosureID && document.LocalFilePath == localFilePath {
			if !found || document.Version > latest.Version || (document.Version == latest.Version && document.UpdatedAt.After(latest.UpdatedAt)) {
				latest = document
				found = true
			}
		}
	}
	return latest, found, nil
}

func (s *Store) LatestDocumentVersion(_ context.Context, sourceSystem documents.SourceSystem, ticker string, disclosureID string, originalFilename string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticker = documents.NormalizeTicker(ticker)
	latest := 0
	for _, document := range s.documents {
		if document.SourceSystem == sourceSystem && document.Ticker == ticker && document.DisclosureID == disclosureID && strings.EqualFold(document.OriginalFilename, originalFilename) && document.Version > latest {
			latest = document.Version
		}
	}
	return latest, nil
}

func (s *Store) ListDocuments(_ context.Context, ticker string) ([]documents.DocumentMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticker = documents.NormalizeTicker(ticker)
	out := make([]documents.DocumentMetadata, 0, len(s.documents))
	for _, document := range s.documents {
		if ticker == "" || document.Ticker == ticker {
			out = append(out, document)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ticker != out[j].Ticker {
			return out[i].Ticker < out[j].Ticker
		}
		if !out[i].DisclosureDate.Equal(out[j].DisclosureDate) {
			return out[i].DisclosureDate.Before(out[j].DisclosureDate)
		}
		return out[i].OriginalFilename < out[j].OriginalFilename
	})
	return out, nil
}

func (s *Store) SaveIngestionJob(_ context.Context, job documents.IngestionJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.JobID] = job
	return s.writeJobsLocked()
}

func (s *Store) UpdateIngestionJob(_ context.Context, job documents.IngestionJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.JobID] = job
	return s.writeJobsLocked()
}

func (s *Store) SaveIngestionError(_ context.Context, item documents.IngestionError) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(s.root, "extraction_errors.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = file.Write(append(raw, '\n'))
	return err
}

func (s *Store) load() error {
	if err := loadJSONArray(filepath.Join(s.root, "document_registry.json"), &s.documents); err != nil {
		return err
	}
	if err := loadJSONArray(filepath.Join(s.root, "extraction_jobs.json"), &s.jobs); err != nil {
		return err
	}
	return nil
}

func loadJSONArray[T any](path string, target *map[string]T) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	items := []T{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}
	next := map[string]T{}
	for _, item := range items {
		key := registryKey(item)
		if key != "" {
			next[key] = item
		}
	}
	*target = next
	return nil
}

func registryKey(item any) string {
	switch typed := item.(type) {
	case documents.DocumentMetadata:
		return typed.DocumentID
	case documents.IngestionJob:
		return typed.JobID
	default:
		return ""
	}
}

func (s *Store) writeDocumentsLocked() error {
	items := make([]documents.DocumentMetadata, 0, len(s.documents))
	for _, item := range s.documents {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DocumentID < items[j].DocumentID })
	return writeJSONAtomic(filepath.Join(s.root, "document_registry.json"), items)
}

func (s *Store) writeJobsLocked() error {
	items := make([]documents.IngestionJob, 0, len(s.jobs))
	for _, item := range s.jobs {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartedAt.Before(items[j].StartedAt) })
	return writeJSONAtomic(filepath.Join(s.root, "extraction_jobs.json"), items)
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	writer := bufio.NewWriter(tmp)
	if _, err := writer.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := writer.Flush(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func sameVersionGroup(a, b documents.DocumentMetadata) bool {
	return a.SourceSystem == b.SourceSystem &&
		a.Ticker == b.Ticker &&
		a.DisclosureID == b.DisclosureID &&
		strings.EqualFold(a.OriginalFilename, b.OriginalFilename)
}
