package documents

import (
	"errors"
	"strings"
	"time"
)

type DocumentType string

const (
	DocumentTypePDF      DocumentType = "PDF"
	DocumentTypeHTML     DocumentType = "HTML"
	DocumentTypeXML      DocumentType = "XML"
	DocumentTypeXBRL     DocumentType = "XBRL"
	DocumentTypeOCRImage DocumentType = "OCR_IMAGE"
	DocumentTypeOther    DocumentType = "OTHER"
)

type SourceSystem string

const (
	SourceSystemKAP SourceSystem = "KAP"
)

type Language string

const (
	LanguageTR    Language = "tr"
	LanguageEN    Language = "en"
	LanguageMixed Language = "mixed"
)

type ExtractionStatus string

const (
	ExtractionStatusPending        ExtractionStatus = "pending"
	ExtractionStatusTextReady      ExtractionStatus = "text_ready"
	ExtractionStatusNeedsOCR       ExtractionStatus = "needs_ocr"
	ExtractionStatusRejected       ExtractionStatus = "rejected"
	ExtractionStatusReviewRequired ExtractionStatus = "review_required"
)

type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusSuccess JobStatus = "success"
	JobStatusPartial JobStatus = "partial"
	JobStatusFailed  JobStatus = "failed"
)

type DocumentMetadata struct {
	DocumentID        string           `json:"document_id"`
	CompanyID         string           `json:"company_id,omitempty"`
	Ticker            string           `json:"ticker"`
	KAPCompanyID      string           `json:"kap_company_id,omitempty"`
	DisclosureID      string           `json:"disclosure_id"`
	DisclosureIndex   int              `json:"disclosure_index,omitempty"`
	DisclosureDate    time.Time        `json:"disclosure_date"`
	DisclosureType    string           `json:"disclosure_type,omitempty"`
	Period            string           `json:"period,omitempty"`
	FiscalYear        int              `json:"fiscal_year,omitempty"`
	DocumentType      DocumentType     `json:"document_type"`
	FileURL           string           `json:"file_url,omitempty"`
	LocalFilePath     string           `json:"local_file_path"`
	OriginalFilename  string           `json:"original_filename"`
	Checksum          string           `json:"checksum"`
	ChecksumAlgorithm string           `json:"checksum_algorithm"`
	SizeBytes         int64            `json:"size_bytes"`
	Language          Language         `json:"language"`
	SourceSystem      SourceSystem     `json:"source_system"`
	Version           int              `json:"version"`
	IsLatestVersion   bool             `json:"is_latest_version"`
	ExtractionStatus  ExtractionStatus `json:"extraction_status"`
	ReviewRequired    bool             `json:"review_required"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type IngestionJob struct {
	JobID            string     `json:"job_id"`
	JobType          string     `json:"job_type"`
	Status           JobStatus  `json:"status"`
	Ticker           string     `json:"ticker,omitempty"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	DocumentsScanned int        `json:"documents_scanned"`
	DocumentsSaved   int        `json:"documents_saved"`
	DocumentsSkipped int        `json:"documents_skipped"`
	Errors           int        `json:"errors"`
	LastError        string     `json:"last_error,omitempty"`
	ReviewRequired   bool       `json:"review_required"`
	SourceSystem     string     `json:"source_system"`
	SourceRoot       string     `json:"source_root,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type IngestionError struct {
	JobID        string    `json:"job_id"`
	Ticker       string    `json:"ticker,omitempty"`
	DocumentPath string    `json:"document_path,omitempty"`
	Stage        string    `json:"stage"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

func (d DocumentMetadata) Validate() error {
	if strings.TrimSpace(d.DocumentID) == "" {
		return errors.New("document_id is required")
	}
	if strings.TrimSpace(d.Ticker) == "" {
		return errors.New("ticker is required")
	}
	if strings.TrimSpace(d.DisclosureID) == "" && d.DisclosureIndex == 0 {
		return errors.New("disclosure_id or disclosure_index is required")
	}
	if strings.TrimSpace(d.LocalFilePath) == "" {
		return errors.New("local_file_path is required")
	}
	if strings.TrimSpace(d.OriginalFilename) == "" {
		return errors.New("original_filename is required")
	}
	if strings.TrimSpace(d.Checksum) == "" || d.ChecksumAlgorithm != "sha256" {
		return errors.New("sha256 checksum is required")
	}
	if d.SourceSystem == "" {
		return errors.New("source_system is required")
	}
	if d.DocumentType == "" {
		return errors.New("document_type is required")
	}
	if d.Version <= 0 {
		return errors.New("version must be positive")
	}
	return nil
}

func NormalizeTicker(ticker string) string {
	return strings.ToUpper(strings.TrimSpace(ticker))
}
