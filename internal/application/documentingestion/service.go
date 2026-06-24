package documentingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/domain/documents"
	"hissebot/internal/repositories"
)

type Service struct {
	Repository repositories.DocumentRepository
	Now        func() time.Time
}

type ArchiveRequest struct {
	EquitiesDir string
	Ticker      string
	Limit       int
}

type KAPAttachmentInput struct {
	CompanyID        string
	Ticker           string
	KAPCompanyID     string
	DisclosureID     string
	DisclosureIndex  int
	DisclosureDate   time.Time
	DisclosureType   string
	Period           string
	FiscalYear       int
	FileURL          string
	LocalFilePath    string
	OriginalFilename string
	Language         documents.Language
}

type ArchiveResult struct {
	Job       documents.IngestionJob       `json:"job"`
	Documents []documents.DocumentMetadata `json:"documents,omitempty"`
	Errors    []documents.IngestionError   `json:"errors,omitempty"`
}

type detailFile struct {
	Raw []struct {
		Disclosure struct {
			Basic map[string]any `json:"disclosureBasic"`
		} `json:"disclosure"`
		Attachments []struct {
			ObjID         string `json:"objId"`
			FileName      string `json:"fileName"`
			FileExtension string `json:"fileExtension"`
		} `json:"attachments"`
	} `json:"raw"`
}

func (s Service) ArchiveLocalKAPAttachments(ctx context.Context, req ArchiveRequest) (ArchiveResult, error) {
	if s.Repository == nil {
		return ArchiveResult{}, errors.New("document repository is required")
	}
	if strings.TrimSpace(req.EquitiesDir) == "" {
		return ArchiveResult{}, errors.New("equities dir is required")
	}
	now := s.now()
	job := documents.IngestionJob{
		JobID:        newJobID("kap_document_archive", req.Ticker, now),
		JobType:      "kap_document_archive",
		Status:       documents.JobStatusRunning,
		Ticker:       documents.NormalizeTicker(req.Ticker),
		StartedAt:    now,
		SourceSystem: string(documents.SourceSystemKAP),
		SourceRoot:   req.EquitiesDir,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.Repository.SaveIngestionJob(ctx, job); err != nil {
		return ArchiveResult{}, err
	}
	result := ArchiveResult{Job: job}
	files, err := localAttachmentFiles(req.EquitiesDir, req.Ticker)
	if err != nil {
		job.Status = documents.JobStatusFailed
		job.LastError = err.Error()
		job.Errors++
		finished := s.now()
		job.FinishedAt = &finished
		job.UpdatedAt = finished
		_ = s.Repository.UpdateIngestionJob(ctx, job)
		return result, err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if req.Limit > 0 && job.DocumentsSaved >= req.Limit {
			break
		}
		job.DocumentsScanned++
		input, err := s.inputFromLocalPath(req.EquitiesDir, file)
		if err == nil {
			var saved bool
			var document documents.DocumentMetadata
			document, saved, err = s.IngestKAPAttachment(ctx, input)
			if err == nil {
				if saved {
					job.DocumentsSaved++
					result.Documents = append(result.Documents, document)
				} else {
					job.DocumentsSkipped++
				}
			}
		}
		if err != nil {
			job.Errors++
			job.ReviewRequired = true
			job.LastError = err.Error()
			item := documents.IngestionError{
				JobID:        job.JobID,
				Ticker:       tickerFromAttachmentPath(req.EquitiesDir, file),
				DocumentPath: file,
				Stage:        "document_archive",
				Message:      err.Error(),
				CreatedAt:    s.now(),
			}
			result.Errors = append(result.Errors, item)
			_ = s.Repository.SaveIngestionError(ctx, item)
		}
	}
	finished := s.now()
	job.FinishedAt = &finished
	job.UpdatedAt = finished
	switch {
	case job.Errors > 0 && job.DocumentsSaved == 0:
		job.Status = documents.JobStatusFailed
	case job.Errors > 0:
		job.Status = documents.JobStatusPartial
	default:
		job.Status = documents.JobStatusSuccess
	}
	if err := s.Repository.UpdateIngestionJob(ctx, job); err != nil {
		return result, err
	}
	result.Job = job
	return result, nil
}

func (s Service) IngestKAPAttachment(ctx context.Context, input KAPAttachmentInput) (documents.DocumentMetadata, bool, error) {
	if s.Repository == nil {
		return documents.DocumentMetadata{}, false, errors.New("document repository is required")
	}
	input.Ticker = documents.NormalizeTicker(input.Ticker)
	if input.Ticker == "" {
		return documents.DocumentMetadata{}, false, errors.New("ticker is required")
	}
	if input.LocalFilePath == "" {
		return documents.DocumentMetadata{}, false, errors.New("local file path is required")
	}
	digest, err := SHA256File(input.LocalFilePath)
	if err != nil {
		return documents.DocumentMetadata{}, false, fmt.Errorf("checksum %s: %w", input.LocalFilePath, err)
	}
	if digest.SizeBytes <= 0 {
		return documents.DocumentMetadata{}, false, fmt.Errorf("empty document file: %s", input.LocalFilePath)
	}
	if input.DisclosureID == "" && input.DisclosureIndex > 0 {
		input.DisclosureID = strconv.Itoa(input.DisclosureIndex)
	}
	if existing, ok, err := s.Repository.FindDocumentBySource(ctx, documents.SourceSystemKAP, input.Ticker, input.DisclosureID, input.LocalFilePath); err != nil {
		return documents.DocumentMetadata{}, false, err
	} else if ok && existing.Checksum == digest.Hex {
		return existing, false, nil
	}
	if input.OriginalFilename == "" {
		input.OriginalFilename = originalNameFromPath(input.LocalFilePath)
	}
	latest, err := s.Repository.LatestDocumentVersion(ctx, documents.SourceSystemKAP, input.Ticker, input.DisclosureID, input.OriginalFilename)
	if err != nil {
		return documents.DocumentMetadata{}, false, err
	}
	now := s.now()
	doc := documents.DocumentMetadata{
		DocumentID:        newDocumentID(input.Ticker, input.DisclosureID, input.LocalFilePath, digest.Hex),
		CompanyID:         strings.TrimSpace(input.CompanyID),
		Ticker:            input.Ticker,
		KAPCompanyID:      strings.TrimSpace(input.KAPCompanyID),
		DisclosureID:      input.DisclosureID,
		DisclosureIndex:   input.DisclosureIndex,
		DisclosureDate:    input.DisclosureDate,
		DisclosureType:    strings.TrimSpace(input.DisclosureType),
		Period:            strings.TrimSpace(input.Period),
		FiscalYear:        input.FiscalYear,
		DocumentType:      classifyDocumentType(input.OriginalFilename),
		FileURL:           strings.TrimSpace(input.FileURL),
		LocalFilePath:     input.LocalFilePath,
		OriginalFilename:  input.OriginalFilename,
		Checksum:          digest.Hex,
		ChecksumAlgorithm: digest.Algorithm,
		SizeBytes:         digest.SizeBytes,
		Language:          input.Language,
		SourceSystem:      documents.SourceSystemKAP,
		Version:           latest + 1,
		IsLatestVersion:   true,
		ExtractionStatus:  documents.ExtractionStatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if doc.Language == "" {
		doc.Language = inferLanguage(input.OriginalFilename)
	}
	if doc.DocumentType == documents.DocumentTypeOCRImage {
		doc.ExtractionStatus = documents.ExtractionStatusNeedsOCR
		doc.ReviewRequired = true
	}
	if err := doc.Validate(); err != nil {
		return documents.DocumentMetadata{}, false, err
	}
	if err := s.Repository.SaveDocument(ctx, doc); err != nil {
		return documents.DocumentMetadata{}, false, err
	}
	return doc, true, nil
}

func (s Service) inputFromLocalPath(equitiesDir string, path string) (KAPAttachmentInput, error) {
	ticker := tickerFromAttachmentPath(equitiesDir, path)
	if ticker == "" {
		return KAPAttachmentInput{}, fmt.Errorf("cannot derive ticker from %s", path)
	}
	disclosureIndex := disclosureIndexFromAttachmentPath(equitiesDir, path)
	if disclosureIndex == 0 {
		return KAPAttachmentInput{}, fmt.Errorf("cannot derive disclosure index from %s", path)
	}
	meta := loadDetailMetadata(equitiesDir, ticker, disclosureIndex)
	objID, original := splitObjectFilename(filepath.Base(path))
	if original == "" {
		original = filepath.Base(path)
	}
	fileURL := ""
	if objID != "" {
		fileURL = "https://www.kap.org.tr/tr/api/file/download/" + objID
	}
	return KAPAttachmentInput{
		CompanyID:        ticker,
		Ticker:           ticker,
		KAPCompanyID:     stringFromMap(meta, "mkkMemberOid"),
		DisclosureID:     firstNonEmpty(stringFromMap(meta, "disclosureId"), strconv.Itoa(disclosureIndex)),
		DisclosureIndex:  disclosureIndex,
		DisclosureDate:   parseKAPTime(firstNonEmpty(stringFromMap(meta, "publishDate"), stringFromMap(meta, "publish_date"))),
		DisclosureType:   firstNonEmpty(stringFromMap(meta, "disclosureType"), stringFromMap(meta, "disclosure_type")),
		Period:           firstNonEmpty(stringFromMap(meta, "period"), stringFromMap(meta, "period_key")),
		FiscalYear:       intFromMap(meta, "year", "fiscal_year"),
		FileURL:          fileURL,
		LocalFilePath:    path,
		OriginalFilename: original,
	}, nil
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func localAttachmentFiles(equitiesDir, ticker string) ([]string, error) {
	roots := []string{}
	if ticker = documents.NormalizeTicker(ticker); ticker != "" {
		roots = append(roots, filepath.Join(equitiesDir, ticker, "kap", "attachments"))
	} else {
		entries, err := os.ReadDir(equitiesDir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), "_") {
				roots = append(roots, filepath.Join(equitiesDir, entry.Name(), "kap", "attachments"))
			}
		}
	}
	out := []string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			out = append(out, path)
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return out, nil
}

func loadDetailMetadata(equitiesDir, ticker string, disclosureIndex int) map[string]any {
	path := filepath.Join(equitiesDir, "_kap", "details", ticker, strconv.Itoa(disclosureIndex)+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var detail detailFile
	if json.Unmarshal(raw, &detail) != nil || len(detail.Raw) == 0 {
		return map[string]any{}
	}
	return detail.Raw[0].Disclosure.Basic
}

func tickerFromAttachmentPath(equitiesDir string, path string) string {
	rel, err := filepath.Rel(equitiesDir, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 5 || parts[1] != "kap" || parts[2] != "attachments" {
		return ""
	}
	return documents.NormalizeTicker(parts[0])
}

func disclosureIndexFromAttachmentPath(equitiesDir string, path string) int {
	rel, err := filepath.Rel(equitiesDir, path)
	if err != nil {
		return 0
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 5 {
		return 0
	}
	index, _ := strconv.Atoi(parts[4])
	return index
}

func splitObjectFilename(name string) (string, string) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) == 2 && len(parts[0]) >= 3 && !strings.ContainsAny(parts[0], " /\\") {
		return parts[0], parts[1]
	}
	return "", name
}

func originalNameFromPath(path string) string {
	_, original := splitObjectFilename(filepath.Base(path))
	return original
}

func classifyDocumentType(name string) documents.DocumentType {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "pdf":
		return documents.DocumentTypePDF
	case "html", "htm":
		return documents.DocumentTypeHTML
	case "xbrl":
		return documents.DocumentTypeXBRL
	case "xml":
		return documents.DocumentTypeXML
	case "jpg", "jpeg", "png", "tif", "tiff":
		return documents.DocumentTypeOCRImage
	default:
		return documents.DocumentTypeOther
	}
}

func inferLanguage(name string) documents.Language {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "annual") || strings.Contains(lower, "financial") || strings.Contains(lower, "ordinary general"):
		return documents.LanguageEN
	case strings.Contains(lower, "bilgilendirme") || strings.Contains(lower, "finansal") || strings.Contains(lower, "genel kurul"):
		return documents.LanguageTR
	default:
		return documents.LanguageMixed
	}
}

func stringFromMap(values map[string]any, key string) string {
	if value, ok := values[key]; ok && value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func intFromMap(values map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case int:
			return typed
		case string:
			parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
			return parsed
		}
	}
	return 0
}

func parseKAPTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006.01.02 15:04:05", "02.01.2006 15:04:05", "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func newDocumentID(ticker, disclosureID, path, checksum string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{documents.NormalizeTicker(ticker), disclosureID, path, checksum}, "\x00")))
	return "kap_doc_" + hex.EncodeToString(sum[:])[:32]
}

func newJobID(jobType, ticker string, ts time.Time) string {
	sum := sha256.Sum256([]byte(jobType + "\x00" + documents.NormalizeTicker(ticker) + "\x00" + ts.Format(time.RFC3339Nano)))
	return "job_" + hex.EncodeToString(sum[:])[:24]
}
