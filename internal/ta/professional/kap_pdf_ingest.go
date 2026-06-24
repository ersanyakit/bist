package professional

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hissebot/internal/kapingest"
	"hissebot/internal/util"
	"hissebot/pkg/mathutil"
)

type KAPPDFIngestSummary struct {
	Computed                    bool                    `json:"computed"`
	Symbol                      string                  `json:"symbol"`
	OutputDir                   string                  `json:"output_dir,omitempty"`
	RawDocumentsPath            string                  `json:"raw_documents_path,omitempty"`
	ProcessedFilesPath          string                  `json:"processed_files_path,omitempty"`
	ErrorsPath                  string                  `json:"errors_path,omitempty"`
	SourcePDFCount              int                     `json:"source_pdf_count,omitempty"`
	SourceUniqueHashes          int                     `json:"source_unique_hash_count,omitempty"`
	DuplicatePDFCount           int                     `json:"duplicate_pdf_count,omitempty"`
	MissingUniqueHashes         int                     `json:"missing_unique_hash_count,omitempty"`
	SourceHashErrors            int                     `json:"source_hash_error_count,omitempty"`
	TotalDocuments              int                     `json:"total_documents"`
	UniqueProcessed             int                     `json:"unique_processed"`
	LowQualityCount             int                     `json:"low_quality_count"`
	AnalysisUsableCount         int                     `json:"analysis_usable_count"`
	ReviewRequiredCount         int                     `json:"review_required_count"`
	RejectedCount               int                     `json:"rejected_count"`
	OCRUsedCount                int                     `json:"ocr_used_count"`
	ErrorCount                  int                     `json:"error_count"`
	AverageQuality              float64                 `json:"average_quality"`
	DecisionRelevantDocuments   int                     `json:"decision_relevant_documents,omitempty"`
	DecisionRelevantUsableCount int                     `json:"decision_relevant_usable_count,omitempty"`
	TypeCounts                  []KAPPDFTypeCount       `json:"type_counts,omitempty"`
	ImportantDocuments          []KAPPDFDocumentSummary `json:"important_documents,omitempty"`
	Documents                   []KAPPDFDocumentSummary `json:"documents,omitempty"`
	Summary                     string                  `json:"summary"`
	Warnings                    []string                `json:"warnings,omitempty"`
}

type KAPPDFTypeCount struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type KAPPDFDocumentSummary struct {
	FileName       string   `json:"file_name"`
	FilePath       string   `json:"file_path"`
	DocumentType   string   `json:"document_type"`
	DocumentLabel  string   `json:"document_label"`
	TextLength     int      `json:"text_length"`
	QualityScore   float64  `json:"quality_score"`
	ParseStatus    string   `json:"parse_status,omitempty"`
	AnalysisUsable bool     `json:"analysis_usable"`
	Warnings       []string `json:"warnings,omitempty"`
	ContentSnippet string   `json:"content_snippet,omitempty"`
}

func analyzeKAPPDFIngest(equitiesDir, symbol string) KAPPDFIngestSummary {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := KAPPDFIngestSummary{Symbol: symbol}
	if symbol == "" {
		out.Summary = "KAP PDF ingest icin sembol bos."
		out.Warnings = []string{"kap_pdf_ingest_symbol_missing"}
		return out
	}
	outputDir, rawPath := findKAPPDFRawDocuments(equitiesDir, symbol)
	if rawPath == "" {
		out.Warnings = []string{"kap_pdf_ingest_output_missing"}
		sourceDir := kapPDFSourceInputDir(equitiesDir, symbol)
		if sourceDir != "" {
			files, err := kapingest.ScanPDFs(sourceDir, 0)
			if err == nil {
				out.SourcePDFCount = len(files)
				if len(files) > 0 {
					out.Summary = fmt.Sprintf("KAP PDF ingest ciktisi bulunamadi; %s icin %d PDF eki var fakat raw_documents.jsonl uretilmemis. Beklenen cikti: %s.", symbol, len(files), strings.Join(expectedKAPPDFRawDocumentPaths(equitiesDir, symbol), " veya "))
					out.Warnings = append(out.Warnings, "kap_pdf_source_files_exist_but_ingest_missing")
					return out
				}
			} else {
				out.Warnings = append(out.Warnings, "kap_pdf_source_scan_failed")
			}
		}
		out.Summary = fmt.Sprintf("KAP PDF ingest ciktisi bulunamadi; %s icin raw_documents.jsonl yok.", symbol)
		return out
	}
	out.OutputDir = outputDir
	out.RawDocumentsPath = rawPath
	out.ProcessedFilesPath = filepath.Join(outputDir, kapingest.ProcessedFilesFile)
	out.ErrorsPath = filepath.Join(outputDir, kapingest.ExtractionErrorsFile)

	docs, warnings := readKAPPDFRawDocuments(rawPath, symbol)
	out.Warnings = append(out.Warnings, warnings...)
	if len(docs) == 0 {
		out.Summary = fmt.Sprintf("%s icin KAP PDF ingest dosyasi var fakat sembole ait belge okunamadi.", symbol)
		out.Warnings = append(out.Warnings, "kap_pdf_ingest_empty_for_symbol")
		return out
	}
	out.Computed = true
	out.TotalDocuments = len(docs)
	out.UniqueProcessed = readKAPPDFProcessedCount(out.ProcessedFilesPath, symbol)
	if out.UniqueProcessed == 0 {
		out.UniqueProcessed = out.TotalDocuments
	}
	out.ErrorCount = readKAPPDFErrorCount(out.ErrorsPath, symbol)
	sourceStats := scanKAPPDFSourceStats(equitiesDir, symbol, docs)
	out.SourcePDFCount = sourceStats.SourcePDFCount
	out.SourceUniqueHashes = sourceStats.SourceUniqueHashes
	out.DuplicatePDFCount = sourceStats.DuplicatePDFCount
	out.MissingUniqueHashes = sourceStats.MissingUniqueHashes
	out.SourceHashErrors = sourceStats.SourceHashErrors

	typeCounts := map[string]int{}
	qualitySum := 0.0
	for i := range docs {
		doc := &docs[i]
		docType := strings.TrimSpace(doc.DocumentTypeGuess)
		if docType == "" {
			docType = kapingest.DocumentUnknown
		}
		reclassified := kapingest.ClassifyDocument(doc.Text, doc.FileName)
		if docType == kapingest.DocumentUnknown && reclassified != kapingest.DocumentUnknown {
			docType = reclassified
			doc.DocumentTypeGuess = reclassified
		}
		quality := kapingest.FinalizeTextQualityForDocument(kapingest.AssessTextQuality(doc.Text), doc.Text, doc.FileName, docType)
		doc.QualityScore = quality.Score
		doc.TextLength = quality.TextLength
		doc.Warnings = kapPDFMergeQualityWarnings(doc.Warnings, quality.Warnings)
		gate := kapingest.EvaluateTextQualityGate(quality, doc.Warnings, docType)
		doc.ParseStatus = gate.Status
		doc.AnalysisUsable = gate.AnalysisUsable
		doc.HumanReviewRequired = gate.HumanReviewRequired
		doc.QualityGate = gate
		doc.Warnings = kapPDFMergeQualityWarnings(doc.Warnings, kapingest.QualityGateWarnings(gate))
		typeCounts[docType]++
		qualitySum += quality.Score
		if gate.Status != kapingest.ParseStatusTrusted {
			out.LowQualityCount++
		}
		if gate.AnalysisUsable {
			out.AnalysisUsableCount++
		}
		if kapPDFDecisionRelevantDocumentType(docType) {
			out.DecisionRelevantDocuments++
			if gate.AnalysisUsable {
				out.DecisionRelevantUsableCount++
			}
		}
		if gate.HumanReviewRequired {
			out.ReviewRequiredCount++
		}
		if gate.Status == kapingest.ParseStatusRejected {
			out.RejectedCount++
		}
		if strings.Contains(strings.ToLower(doc.ExtractionMethod), "ocr") || containsString(doc.Warnings, "ocr_fallback_used") {
			out.OCRUsedCount++
		}
	}
	out.AverageQuality = mathutil.SafeDiv(qualitySum, float64(len(docs)))
	out.TypeCounts = kapPDFTypeCounts(typeCounts)
	out.ImportantDocuments = kapPDFImportantDocuments(docs, 10)
	out.Documents = kapPDFAllDocuments(docs)
	if out.LowQualityCount > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("kap_pdf_low_text_quality_count_%d", out.LowQualityCount))
	}
	if out.ReviewRequiredCount > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("kap_pdf_review_required_count_%d", out.ReviewRequiredCount))
	}
	if out.RejectedCount > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("kap_pdf_parse_rejected_count_%d", out.RejectedCount))
	}
	if out.SourcePDFCount > 0 && out.SourcePDFCount > out.TotalDocuments {
		out.Warnings = append(out.Warnings, fmt.Sprintf("kap_pdf_source_files_exceed_unique_documents_%d", out.SourcePDFCount-out.TotalDocuments))
	}
	if out.MissingUniqueHashes > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("kap_pdf_missing_unique_hashes_%d", out.MissingUniqueHashes))
	}
	if out.SourceHashErrors > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("kap_pdf_source_hash_errors_%d", out.SourceHashErrors))
	}
	out.Summary = kapPDFIngestSummaryText(out)
	return out
}

type kapPDFSourceStats struct {
	SourcePDFCount      int
	SourceUniqueHashes  int
	DuplicatePDFCount   int
	MissingUniqueHashes int
	SourceHashErrors    int
}

func kapPDFMergeQualityWarnings(existing, quality []string) []string {
	out := []string{}
	for _, warning := range existing {
		switch warning {
		case "low_text_quality_possible_scanned_pdf", "short_structured_non_financial_document":
			continue
		default:
			out = append(out, warning)
		}
	}
	out = append(out, quality...)
	return uniqueStrings(out)
}

func recalculateCoverageScore(coverage CoverageReport) CoverageReport {
	total := len(coverage.Available) + len(coverage.Missing)
	coverage.Score = 100 * mathutil.SafeDiv(float64(len(coverage.Available)), float64(total))
	return coverage
}

func findKAPPDFRawDocuments(equitiesDir, symbol string) (string, string) {
	for _, dir := range kapPDFCandidateOutputDirs(equitiesDir, symbol) {
		rawPath := filepath.Join(dir, kapingest.RawDocumentsFile)
		if fileExists(rawPath) {
			return dir, rawPath
		}
	}
	return "", ""
}

func expectedKAPPDFRawDocumentPaths(equitiesDir, symbol string) []string {
	paths := []string{}
	for _, dir := range kapPDFCandidateOutputDirs(equitiesDir, symbol) {
		paths = append(paths, filepath.Join(dir, kapingest.RawDocumentsFile))
		if len(paths) >= 2 {
			break
		}
	}
	return paths
}

func kapPDFCandidateOutputDirs(equitiesDir, symbol string) []string {
	symbol = strings.ToLower(strings.TrimSpace(symbol))
	dirs := []string{}
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" || dir == "." {
			return
		}
		for _, existing := range dirs {
			if existing == dir {
				return
			}
		}
		dirs = append(dirs, dir)
	}
	if equitiesDir != "" {
		dataDir := filepath.Dir(filepath.Clean(equitiesDir))
		add(filepath.Join(dataDir, "processed", symbol))
		add(filepath.Join(dataDir, "processed"))
	}
	add(filepath.Join("data", "processed", symbol))
	add(filepath.Join("data", "processed"))
	return dirs
}

func readKAPPDFRawDocuments(path, symbol string) ([]kapingest.RawDocument, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, []string{"kap_pdf_ingest_raw_documents_open_failed"}
	}
	defer file.Close()

	docs := []kapingest.RawDocument{}
	warnings := []string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var doc kapingest.RawDocument
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			warnings = append(warnings, "kap_pdf_ingest_raw_document_parse_failed")
			continue
		}
		if !kapPDFDocumentMatchesSymbol(doc.Ticker, doc.FilePath, doc.FileName, symbol) {
			continue
		}
		docs = append(docs, doc)
	}
	if err := scanner.Err(); err != nil {
		warnings = append(warnings, "kap_pdf_ingest_raw_documents_scan_failed")
	}
	return docs, uniqueStrings(warnings)
}

func readKAPPDFProcessedCount(path, symbol string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item kapingest.ProcessedFile
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if kapPDFDocumentMatchesSymbol(item.Ticker, item.FilePath, "", symbol) {
			count++
		}
	}
	return count
}

func readKAPPDFErrorCount(path, symbol string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item kapingest.ExtractionError
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			count++
			continue
		}
		if kapPDFDocumentMatchesSymbol("", item.FilePath, "", symbol) {
			count++
		}
	}
	return count
}

func kapPDFDocumentMatchesSymbol(ticker, filePath, fileName, symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(ticker), symbol) {
		return true
	}
	slashPath := strings.ToUpper(filepath.ToSlash(filePath))
	if strings.Contains(slashPath, "/EQUITIES/"+symbol+"/") {
		return true
	}
	return strings.Contains(strings.ToUpper(fileName), symbol)
}

func kapPDFTypeCounts(counts map[string]int) []KAPPDFTypeCount {
	out := make([]KAPPDFTypeCount, 0, len(counts))
	for docType, count := range counts {
		out = append(out, KAPPDFTypeCount{Type: docType, Label: kapPDFDocumentTypeLabel(docType), Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return kapPDFDocumentTypePriority(out[i].Type) < kapPDFDocumentTypePriority(out[j].Type)
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func kapPDFDecisionRelevantDocumentType(docType string) bool {
	switch strings.TrimSpace(docType) {
	case kapingest.DocumentFinancialStatement,
		kapingest.DocumentAnnualReport,
		kapingest.DocumentActivityReport,
		kapingest.DocumentInterimActivityReport,
		kapingest.DocumentIndependentAuditReport,
		kapingest.DocumentCapitalIncrease,
		kapingest.DocumentDividendDistribution:
		return true
	default:
		return false
	}
}

func scanKAPPDFSourceStats(equitiesDir, symbol string, docs []kapingest.RawDocument) kapPDFSourceStats {
	inputDir := kapPDFSourceInputDir(equitiesDir, symbol)
	if inputDir == "" {
		return kapPDFSourceStats{}
	}
	files, err := kapingest.ScanPDFs(inputDir, 0)
	if err != nil {
		return kapPDFSourceStats{SourceHashErrors: 1}
	}
	rawHashes := map[string]bool{}
	for _, doc := range docs {
		if doc.SHA256 != "" {
			rawHashes[doc.SHA256] = true
		}
	}
	seen := map[string]bool{}
	stats := kapPDFSourceStats{SourcePDFCount: len(files)}
	for _, file := range files {
		sha, err := kapingest.SHA256File(file.FilePath)
		if err != nil {
			stats.SourceHashErrors++
			continue
		}
		seen[sha] = true
	}
	stats.SourceUniqueHashes = len(seen)
	if stats.SourcePDFCount > stats.SourceUniqueHashes {
		stats.DuplicatePDFCount = stats.SourcePDFCount - stats.SourceUniqueHashes
	}
	for sha := range seen {
		if !rawHashes[sha] {
			stats.MissingUniqueHashes++
		}
	}
	return stats
}

func kapPDFSourceInputDir(equitiesDir, symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return ""
	}
	equitiesDir = strings.TrimSpace(equitiesDir)
	if equitiesDir == "" {
		equitiesDir = filepath.Join("data", "equities")
	}
	clean := filepath.Clean(equitiesDir)
	if strings.EqualFold(filepath.Base(clean), symbol) {
		return clean
	}
	symbolDir := filepath.Join(clean, symbol)
	if info, err := os.Stat(symbolDir); err == nil && info.IsDir() {
		return symbolDir
	}
	return clean
}

func kapPDFImportantDocuments(docs []kapingest.RawDocument, limit int) []KAPPDFDocumentSummary {
	if limit <= 0 {
		return nil
	}
	candidates := append([]kapingest.RawDocument(nil), docs...)
	sort.Slice(candidates, func(i, j int) bool {
		pi := kapPDFDocumentTypePriority(candidates[i].DocumentTypeGuess)
		pj := kapPDFDocumentTypePriority(candidates[j].DocumentTypeGuess)
		if pi != pj {
			return pi < pj
		}
		if candidates[i].QualityScore != candidates[j].QualityScore {
			return candidates[i].QualityScore > candidates[j].QualityScore
		}
		if candidates[i].TextLength != candidates[j].TextLength {
			return candidates[i].TextLength > candidates[j].TextLength
		}
		return candidates[i].FileName > candidates[j].FileName
	})

	selected := []KAPPDFDocumentSummary{}
	usedSHA := map[string]bool{}
	usedType := map[string]bool{}
	add := func(doc kapingest.RawDocument) {
		if len(selected) >= limit || usedSHA[doc.SHA256] {
			return
		}
		usedSHA[doc.SHA256] = true
		selected = append(selected, KAPPDFDocumentSummary{
			FileName:       doc.FileName,
			FilePath:       doc.FilePath,
			DocumentType:   emptyAsUnknown(doc.DocumentTypeGuess),
			DocumentLabel:  kapPDFDocumentTypeLabel(doc.DocumentTypeGuess),
			TextLength:     doc.TextLength,
			QualityScore:   doc.QualityScore,
			ParseStatus:    doc.ParseStatus,
			AnalysisUsable: doc.AnalysisUsable,
			Warnings:       doc.Warnings,
			ContentSnippet: kapPDFContentSnippet(doc.Text),
		})
	}
	for _, doc := range candidates {
		docType := emptyAsUnknown(doc.DocumentTypeGuess)
		if usedType[docType] {
			continue
		}
		usedType[docType] = true
		add(doc)
	}
	for _, doc := range candidates {
		add(doc)
	}
	return selected
}

func kapPDFAllDocuments(docs []kapingest.RawDocument) []KAPPDFDocumentSummary {
	candidates := append([]kapingest.RawDocument(nil), docs...)
	sort.Slice(candidates, func(i, j int) bool {
		left := filepath.ToSlash(candidates[i].FilePath)
		right := filepath.ToSlash(candidates[j].FilePath)
		if left != right {
			return left > right
		}
		return candidates[i].FileName > candidates[j].FileName
	})
	out := make([]KAPPDFDocumentSummary, 0, len(candidates))
	for _, doc := range candidates {
		out = append(out, kapPDFDocumentSummaryFromRaw(doc))
	}
	return out
}

func kapPDFDocumentSummaryFromRaw(doc kapingest.RawDocument) KAPPDFDocumentSummary {
	return KAPPDFDocumentSummary{
		FileName:       doc.FileName,
		FilePath:       doc.FilePath,
		DocumentType:   emptyAsUnknown(doc.DocumentTypeGuess),
		DocumentLabel:  kapPDFDocumentTypeLabel(doc.DocumentTypeGuess),
		TextLength:     doc.TextLength,
		QualityScore:   doc.QualityScore,
		ParseStatus:    doc.ParseStatus,
		AnalysisUsable: doc.AnalysisUsable,
		Warnings:       doc.Warnings,
		ContentSnippet: kapPDFContentSnippet(doc.Text),
	}
}

func kapPDFIngestSummaryText(summary KAPPDFIngestSummary) string {
	topTypes := []string{}
	for i, item := range summary.TypeCounts {
		if i >= 4 {
			break
		}
		topTypes = append(topTypes, fmt.Sprintf("%s %d", item.Label, item.Count))
	}
	parts := []string{}
	if summary.SourcePDFCount > 0 {
		parts = append(parts, fmt.Sprintf("%d kaynak PDF dosyasi tarandi", summary.SourcePDFCount))
		if summary.SourceUniqueHashes > 0 {
			parts = append(parts, fmt.Sprintf("%d benzersiz PDF hash'i bulundu", summary.SourceUniqueHashes))
		}
		if summary.DuplicatePDFCount > 0 {
			parts = append(parts, fmt.Sprintf("%d duplicate PDF dosyasi ayni hash nedeniyle tekil metne indirildi", summary.DuplicatePDFCount))
		}
	}
	parts = append(parts,
		fmt.Sprintf("%d benzersiz KAP PDF metni rapora dahil edildi", summary.TotalDocuments),
		fmt.Sprintf("%d belge analiz kaniti olarak kullanilabilir", summary.AnalysisUsableCount),
		fmt.Sprintf("ortalama metin kalite skoru %.2f", summary.AverageQuality),
	)
	if len(topTypes) > 0 {
		parts = append(parts, "agirlikli belge tipleri: "+strings.Join(topTypes, ", "))
	}
	if summary.ReviewRequiredCount > 0 {
		reviewOnlyCount := summary.ReviewRequiredCount - summary.RejectedCount
		if reviewOnlyCount < 0 {
			reviewOnlyCount = summary.ReviewRequiredCount
		}
		if summary.RejectedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d belge kalite kapisi nedeniyle analiz disi kaldi: %d review, %d rejected", summary.ReviewRequiredCount, reviewOnlyCount, summary.RejectedCount))
		} else {
			parts = append(parts, fmt.Sprintf("%d belge kalite kapisi nedeniyle AI vision/OCR cozum bekliyor", summary.ReviewRequiredCount))
		}
	} else if summary.RejectedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d belge parse reddedildi ve analiz kaniti sayilmadi", summary.RejectedCount))
	}
	if summary.OCRUsedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d belge OCR fallback ile okundu", summary.OCRUsedCount))
	}
	if summary.ErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("%d ingest hatasi var", summary.ErrorCount))
	}
	return strings.Join(parts, "; ") + "."
}

func kapPDFContentSnippet(text string) string {
	lines := strings.Split(text, "\n")
	keywords := []string{
		"gayrimenkul", "degerleme", "ekspertiz", "net aktif deger", "portfoy",
		"finansal durum", "kar veya zarar", "faaliyet raporu", "kar payi", "sermaye",
	}
	for _, keyword := range keywords {
		for _, line := range lines {
			cleaned := normalizeKAPPDFSnippet(line)
			if len([]rune(cleaned)) < 35 {
				continue
			}
			if strings.Contains(util.SlugTR(cleaned), util.SlugTR(keyword)) {
				return truncateRunes(cleaned, 260)
			}
		}
	}
	for _, line := range lines {
		cleaned := normalizeKAPPDFSnippet(line)
		if len([]rune(cleaned)) >= 35 {
			return truncateRunes(cleaned, 260)
		}
	}
	return ""
}

func normalizeKAPPDFSnippet(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func kapPDFDocumentTypePriority(docType string) int {
	switch emptyAsUnknown(docType) {
	case kapingest.DocumentValuationReport:
		return 1
	case kapingest.DocumentFinancialStatement:
		return 2
	case kapingest.DocumentAnnualReport:
		return 3
	case kapingest.DocumentInterimActivityReport:
		return 4
	case kapingest.DocumentDividend:
		return 5
	case kapingest.DocumentCapitalIncrease:
		return 6
	case kapingest.DocumentCorporateGovernance:
		return 7
	case kapingest.DocumentGeneralAssembly:
		return 8
	case kapingest.DocumentMaterialEvent:
		return 9
	default:
		return 50
	}
}

func kapPDFDocumentTypeLabel(docType string) string {
	switch emptyAsUnknown(docType) {
	case kapingest.DocumentInterimActivityReport:
		return "Ara donem faaliyet raporu"
	case kapingest.DocumentAnnualReport:
		return "Yillik faaliyet raporu"
	case kapingest.DocumentFinancialStatement:
		return "Finansal tablo"
	case kapingest.DocumentValuationReport:
		return "Degerleme raporu"
	case kapingest.DocumentMaterialEvent:
		return "Ozel durum"
	case kapingest.DocumentGeneralAssembly:
		return "Genel kurul"
	case kapingest.DocumentDividend:
		return "Kar payi/temettu"
	case kapingest.DocumentCapitalIncrease:
		return "Sermaye artirimi"
	case kapingest.DocumentShareBuyback:
		return "Pay geri alim"
	case kapingest.DocumentBoardDecision:
		return "Yonetim kurulu karari"
	case kapingest.DocumentAuditReport:
		return "Denetim raporu"
	case kapingest.DocumentCorporateGovernance:
		return "Kurumsal yonetim"
	default:
		return "Bilinmeyen/karma belge"
	}
}

func emptyAsUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return kapingest.DocumentUnknown
	}
	return value
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "..."
}
