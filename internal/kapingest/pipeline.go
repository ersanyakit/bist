package kapingest

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"
)

func Run(ctx context.Context, opts Options) (Summary, error) {
	if opts.InputDir == "" {
		return Summary{}, errors.New("input dir is required")
	}
	if opts.OutputDir == "" {
		return Summary{}, errors.New("output dir is required")
	}
	if opts.Workers <= 0 {
		opts.Workers = DefaultWorkers
	}
	if opts.ExtractTimeout <= 0 {
		opts.ExtractTimeout = DefaultExtractTimeout
	}
	if opts.OCRTimeout <= 0 {
		opts.OCRTimeout = DefaultOCRTimeout
	}
	if opts.VisionTimeout <= 0 {
		opts.VisionTimeout = DefaultVisionTimeout
	}
	if opts.VisionMaxPages <= 0 {
		opts.VisionMaxPages = DefaultVisionMaxPages
	}
	if opts.VisionRenderDPI <= 0 {
		opts.VisionRenderDPI = DefaultVisionRenderDPI
	}
	now := nowFunc(opts)
	started := now().UTC()
	summary := Summary{
		Status:    "ok",
		InputDir:  filepath.Clean(opts.InputDir),
		OutputDir: filepath.Clean(opts.OutputDir),
		Workers:   opts.Workers,
		Limit:     opts.Limit,
		Resume:    opts.Resume,
		LLM:       opts.LLM,
		DryRun:    opts.DryRun,
		StartedAt: started.Format(time.RFC3339),
	}

	files, err := ScanPDFs(opts.InputDir, opts.Limit)
	if err != nil {
		return summary, err
	}
	summary.Scanned = len(files)
	summary.Planned = len(files)
	if len(files) > 0 {
		sample := files
		if len(sample) > 10 {
			sample = sample[:10]
		}
		summary.SampleFiles = append([]PDFFile{}, sample...)
	}
	if opts.DryRun {
		summary.Status = "dry_run"
		summary.FinishedAt = now().UTC().Format(time.RFC3339)
		return summary, nil
	}

	checkpoint := NewCheckpoint()
	if opts.Resume {
		var err error
		checkpoint, err = LoadCheckpoint(opts.OutputDir)
		if err != nil {
			return summary, err
		}
	}
	writer, err := NewJSONLWriter(opts.OutputDir, false)
	if err != nil {
		return summary, err
	}
	defer writer.Close()
	if err := writer.EnsureFiles(opts.LLM); err != nil {
		return summary, err
	}
	summary.OutputFiles = writer.OutputFiles(opts.LLM)

	jobs := make(chan PDFFile)
	results := make(chan processResult)
	workerCount := opts.Workers
	if workerCount > len(files) && len(files) > 0 {
		workerCount = len(files)
	}
	if workerCount <= 0 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				if ctx.Err() != nil {
					results <- processResult{errors: 1}
					continue
				}
				results <- processPDF(ctx, file, opts, checkpoint, writer)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, file := range files {
			if ctx.Err() != nil {
				return
			}
			jobs <- file
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.skipped {
			summary.Skipped++
		}
		if result.raw {
			summary.RawDocuments++
		}
		if result.analysisUsable {
			summary.AnalysisUsable++
		}
		if result.reviewRequired {
			summary.ReviewRequired++
		}
		if result.rejected {
			summary.Rejected++
		}
		if result.event {
			summary.KAPEvents++
		}
		if result.processed {
			summary.ProcessedFiles++
		}
		summary.Errors += result.errors
	}
	if err := ctx.Err(); err != nil {
		summary.Status = "cancelled"
		summary.FinishedAt = now().UTC().Format(time.RFC3339)
		return summary, err
	}
	if summary.Errors > 0 {
		summary.Status = "partial"
	}
	if err := writer.Close(); err != nil {
		summary.Status = "partial"
		summary.Errors++
		return summary, err
	}
	assetSummary, err := ExtractAssetsFromRawDocuments(ctx, AssetExtractionOptions{
		RawDocumentsPath: filepath.Join(opts.OutputDir, RawDocumentsFile),
		OutputDir:        opts.OutputDir,
		Now:              opts.Now,
	})
	if err != nil {
		summary.Status = "partial"
		summary.Errors++
		return summary, err
	}
	summary.AssetEvents = assetSummary.AssetEvents
	summary.AssetInventories = assetSummary.AssetInventories
	summary.OutputFiles = append(summary.OutputFiles, assetSummary.OutputFiles...)
	indexSummary, err := ExtractDocumentIntelligenceFromRawDocuments(ctx, DocumentIntelligenceOptions{
		RawDocumentsPath: filepath.Join(opts.OutputDir, RawDocumentsFile),
		OutputDir:        opts.OutputDir,
		KAPSectorsPath:   opts.KAPSectorsPath,
		PromptPackPath:   opts.PromptPackPath,
		Workers:          opts.Workers,
		Now:              opts.Now,
	})
	if err != nil {
		summary.Status = "partial"
		summary.Errors++
		return summary, err
	}
	summary.DocumentFacts = indexSummary.DocumentFacts
	summary.FinancialFacts = indexSummary.FinancialFacts
	summary.FinancialTables = indexSummary.FinancialTables
	summary.People = indexSummary.People
	summary.OwnershipFacts = indexSummary.OwnershipFacts
	summary.CorporateEvents = indexSummary.CorporateEvents
	summary.DocumentIndexes = indexSummary.DocumentIndexes
	summary.KnowledgeGraphs = indexSummary.KnowledgeGraphs
	summary.OutputFiles = append(summary.OutputFiles, indexSummary.OutputFiles...)
	canonicalSummary, err := BuildCanonicalFinancialsFromOutput(opts.OutputDir, opts.Now)
	if err != nil {
		summary.Status = "partial"
		summary.Errors++
	} else {
		summary.CanonicalFinancials = canonicalSummary.FilesWritten
		summary.OutputFiles = append(summary.OutputFiles, canonicalSummary.OutputFiles...)
	}
	summary.FinishedAt = now().UTC().Format(time.RFC3339)
	return summary, nil
}
