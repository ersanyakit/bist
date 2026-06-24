package kapingest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type processResult struct {
	raw            bool
	event          bool
	processed      bool
	skipped        bool
	analysisUsable bool
	reviewRequired bool
	rejected       bool
	errors         int
}

func processPDF(ctx context.Context, file PDFFile, opts Options, checkpoint *Checkpoint, writer *JSONLWriter) processResult {
	now := nowFunc(opts)
	createdAt := now().UTC().Format(time.RFC3339)
	result := processResult{}

	sha, err := SHA256File(file.FilePath)
	if err != nil {
		_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, Stage: "hash", Error: err.Error(), CreatedAt: createdAt})
		result.errors++
		return result
	}
	if !checkpoint.Reserve(sha) {
		result.skipped = true
		return result
	}

	extractor := opts.TextExtractor
	if extractor == nil {
		extractor = PDFTextExtractor{
			Timeout:         opts.ExtractTimeout,
			DisableOCR:      opts.DisableOCR,
			OCRTimeout:      opts.OCRTimeout,
			OCRLanguages:    opts.OCRLanguages,
			EnableVision:    opts.EnableVision,
			VisionCommand:   opts.VisionCommand,
			VisionTimeout:   opts.VisionTimeout,
			VisionMaxPages:  opts.VisionMaxPages,
			VisionRenderDPI: opts.VisionRenderDPI,
		}
	}
	text, method, warnings, err := extractor.ExtractText(ctx, file.FilePath)
	if err != nil {
		if isPermanentPDFTextExtractionFailure(err) {
			return writeRejectedRawDocumentForExtractFailure(writer, file, sha, firstNonEmpty(method, "pdftotext"), err, createdAt)
		}
		checkpoint.Release(sha)
		_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "extract_text", Error: err.Error(), CreatedAt: createdAt})
		result.errors++
		return result
	}
	quality := AssessTextQuality(text)
	docType := ClassifyDocument(text, file.FileName)
	quality = FinalizeTextQualityForDocument(quality, text, file.FileName, docType)
	warnings = append(warnings, quality.Warnings...)
	warnings = dedupeStrings(warnings)
	gate := EvaluateTextQualityGate(quality, warnings, docType)
	warnings = dedupeStrings(append(warnings, QualityGateWarnings(gate)...))
	raw := RawDocument{
		FilePath:            file.FilePath,
		SHA256:              sha,
		Ticker:              file.Ticker,
		FileName:            filepath.Base(file.FilePath),
		ExtractionMethod:    method,
		DocumentTypeGuess:   docType,
		Text:                text,
		TextLength:          quality.TextLength,
		QualityScore:        quality.Score,
		ParseStatus:         gate.Status,
		AnalysisUsable:      gate.AnalysisUsable,
		HumanReviewRequired: gate.HumanReviewRequired,
		QualityGate:         gate,
		Warnings:            warnings,
		CreatedAt:           createdAt,
	}
	raw = RawDocumentAfterAIAdjudication(raw)
	gate = raw.QualityGate
	warnings = raw.Warnings
	if err := writer.WriteRaw(raw); err != nil {
		checkpoint.Release(sha)
		_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "write", Error: err.Error(), CreatedAt: createdAt})
		result.errors++
		return result
	}
	result.raw = true
	result.analysisUsable = gate.AnalysisUsable
	result.reviewRequired = gate.HumanReviewRequired
	result.rejected = gate.Status == ParseStatusRejected

	if opts.LLM && gate.AnalysisUsable {
		client := opts.LLMClient
		if client == nil {
			client = RuleBasedLLMClient{}
		}
		event, err := client.ExtractKAPEvent(ctx, raw)
		if err != nil {
			checkpoint.Release(sha)
			_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "llm_extract", Error: err.Error(), CreatedAt: createdAt})
			result.errors++
			return result
		}
		if event.FilePath == "" {
			event.FilePath = raw.FilePath
		}
		if event.SHA256 == "" {
			event.SHA256 = raw.SHA256
		}
		if event.Ticker == "" {
			event.Ticker = raw.Ticker
		}
		warns, err := ValidateKAPEvent(&event)
		if err != nil {
			checkpoint.Release(sha)
			_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "validate", Error: err.Error(), CreatedAt: createdAt})
			result.errors++
			return result
		}
		for _, warning := range warns {
			event.RiskFlags = append(event.RiskFlags, warning)
		}
		event.RiskFlags = dedupeStrings(event.RiskFlags)
		if err := writer.WriteEvent(event); err != nil {
			checkpoint.Release(sha)
			_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "write", Error: err.Error(), CreatedAt: createdAt})
			result.errors++
			return result
		}
		result.event = true
	}

	processed := ProcessedFile{
		FilePath:            file.FilePath,
		SHA256:              sha,
		Ticker:              file.Ticker,
		DocumentTypeGuess:   docType,
		QualityScore:        quality.Score,
		TextLength:          quality.TextLength,
		ParseStatus:         gate.Status,
		AnalysisUsable:      gate.AnalysisUsable,
		HumanReviewRequired: gate.HumanReviewRequired,
		AIResolved:          gate.AIResolved,
		AIConfidence:        gate.AIConfidence,
		LLMEnabled:          opts.LLM && gate.AnalysisUsable,
		CreatedAt:           createdAt,
	}
	if err := writer.WriteProcessed(processed); err != nil {
		checkpoint.Release(sha)
		_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "write", Error: fmt.Sprintf("processed checkpoint write failed: %v", err), CreatedAt: createdAt})
		result.errors++
		return result
	}
	checkpoint.Mark(processed)
	result.processed = true
	return result
}

func writeRejectedRawDocumentForExtractFailure(writer *JSONLWriter, file PDFFile, sha, method string, extractErr error, createdAt string) processResult {
	gate := QualityGate{
		Status:              ParseStatusRejected,
		AnalysisUsable:      false,
		HumanReviewRequired: true,
		TrustedThreshold:    TrustedTextQualityThreshold,
		RejectedThreshold:   RejectedTextQualityThreshold,
		Reasons:             []string{"text_extraction_failed", "empty_text"},
	}
	warnings := []string{
		"pdf_text_extraction_failed",
		"pdf_structure_invalid_or_zero_pages",
		"parse_rejected_by_quality_gate",
		"analysis_blocked_by_quality_gate",
	}
	if extractErr != nil {
		warnings = append(warnings, extractionWarningForError(extractErr))
	}
	warnings = dedupeStrings(warnings)
	raw := RawDocument{
		FilePath:            file.FilePath,
		SHA256:              sha,
		Ticker:              file.Ticker,
		FileName:            filepath.Base(file.FilePath),
		ExtractionMethod:    firstNonEmpty(method, "pdftotext_failed"),
		DocumentTypeGuess:   ClassifyDocument("", file.FileName),
		Text:                "",
		TextLength:          0,
		QualityScore:        0,
		ParseStatus:         gate.Status,
		AnalysisUsable:      false,
		HumanReviewRequired: true,
		QualityGate:         gate,
		Warnings:            warnings,
		CreatedAt:           createdAt,
	}
	if err := writer.WriteRaw(raw); err != nil {
		_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "write", Error: err.Error(), CreatedAt: createdAt})
		return processResult{errors: 1}
	}
	processed := ProcessedFile{
		FilePath:            file.FilePath,
		SHA256:              sha,
		Ticker:              file.Ticker,
		DocumentTypeGuess:   raw.DocumentTypeGuess,
		QualityScore:        0,
		TextLength:          0,
		ParseStatus:         gate.Status,
		AnalysisUsable:      false,
		HumanReviewRequired: true,
		LLMEnabled:          false,
		CreatedAt:           createdAt,
	}
	if err := writer.WriteProcessed(processed); err != nil {
		_ = writer.WriteError(ExtractionError{FilePath: file.FilePath, SHA256: sha, Stage: "write", Error: fmt.Sprintf("processed checkpoint write failed: %v", err), CreatedAt: createdAt})
		return processResult{raw: true, rejected: true, reviewRequired: true, errors: 1}
	}
	return processResult{raw: true, processed: true, rejected: true, reviewRequired: true}
}

func isPermanentPDFTextExtractionFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "catalog dictionary does not contain a valid") ||
		strings.Contains(msg, "catalog object is wrong type") ||
		strings.Contains(msg, "couldn't read page catalog") ||
		strings.Contains(msg, "last page (0)") ||
		strings.Contains(msg, "wrong page range") ||
		strings.Contains(msg, "document stream is empty")
}

func extractionWarningForError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "last page (0)") || strings.Contains(msg, "wrong page range"):
		return "pdf_zero_pages"
	case strings.Contains(msg, "catalog dictionary") || strings.Contains(msg, "catalog object is wrong type") || strings.Contains(msg, "couldn't read page catalog"):
		return "pdf_catalog_pages_invalid"
	case strings.Contains(msg, "document stream is empty"):
		return "pdf_document_stream_empty"
	default:
		return "pdf_text_extraction_error"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func nowFunc(opts Options) func() time.Time {
	if opts.Now != nil {
		return opts.Now
	}
	return time.Now
}
