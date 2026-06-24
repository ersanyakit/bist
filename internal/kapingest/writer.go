package kapingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type JSONLWriter struct {
	outputDir string
	dryRun    bool
	mu        sync.Mutex
	files     map[string]*os.File
}

func NewJSONLWriter(outputDir string, dryRun bool) (*JSONLWriter, error) {
	w := &JSONLWriter{
		outputDir: outputDir,
		dryRun:    dryRun,
		files:     map[string]*os.File{},
	}
	if dryRun {
		return w, nil
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *JSONLWriter) WriteRaw(doc RawDocument) error {
	return w.writeJSONL(RawDocumentsFile, doc)
}

func (w *JSONLWriter) WriteEvent(event KAPEvent) error {
	return w.writeJSONL(KAPEventsFile, event)
}

func (w *JSONLWriter) WriteError(item ExtractionError) error {
	return w.writeJSONL(ExtractionErrorsFile, item)
}

func (w *JSONLWriter) WriteProcessed(item ProcessedFile) error {
	return w.writeJSONL(ProcessedFilesFile, item)
}

func (w *JSONLWriter) OutputFiles(llm bool) []string {
	files := []string{
		filepath.Join(w.outputDir, RawDocumentsFile),
		filepath.Join(w.outputDir, ExtractionErrorsFile),
		filepath.Join(w.outputDir, ProcessedFilesFile),
	}
	if llm {
		files = append(files, filepath.Join(w.outputDir, KAPEventsFile))
	}
	return files
}

func (w *JSONLWriter) EnsureFiles(llm bool) error {
	if w.dryRun {
		return nil
	}
	for _, path := range w.OutputFiles(llm) {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (w *JSONLWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var firstErr error
	for name, file := range w.files {
		if err := file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(w.files, name)
	}
	return firstErr
}

func (w *JSONLWriter) writeJSONL(name string, value any) error {
	if w.dryRun {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	file, err := w.file(name)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	return encoder.Encode(value)
}

func (w *JSONLWriter) file(name string) (*os.File, error) {
	if file := w.files[name]; file != nil {
		return file, nil
	}
	path := filepath.Join(w.outputDir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w.files[name] = file
	return file, nil
}
