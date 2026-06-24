package kapingest

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"hissebot/internal/kapfinance"
	"hissebot/internal/util"
)

type DocumentIntelligenceOptions struct {
	RawDocumentsPath   string
	OutputDir          string
	KAPSectorsPath     string
	PromptPackPath     string
	SkipKnowledgeGraph bool
	Workers            int
	Now                func() time.Time
}

type DocumentIntelligenceSummary struct {
	Status                  string   `json:"status"`
	RawDocumentsPath        string   `json:"raw_documents_path"`
	OutputDir               string   `json:"output_dir"`
	RawDocuments            int      `json:"raw_documents"`
	AnalysisUsableDocuments int      `json:"analysis_usable_documents"`
	AIResolvedDocuments     int      `json:"ai_resolved_documents,omitempty"`
	ReviewRequiredDocuments int      `json:"review_required_documents"`
	RejectedDocuments       int      `json:"rejected_documents"`
	DocumentFacts           int      `json:"document_facts"`
	FinancialFacts          int      `json:"financial_facts"`
	FinancialTables         int      `json:"financial_tables"`
	People                  int      `json:"people"`
	OwnershipFacts          int      `json:"ownership_facts"`
	CorporateEvents         int      `json:"corporate_events"`
	DocumentIndexes         int      `json:"document_indexes"`
	KnowledgeGraphs         int      `json:"knowledge_graphs"`
	Tickers                 int      `json:"tickers"`
	OutputFiles             []string `json:"output_files,omitempty"`
	Warnings                []string `json:"warnings,omitempty"`
}

var legacyPDFTurkishReplacer = strings.NewReplacer(
	"A.fi.", "A.Ş.",
	"A.fi", "A.Ş",
	"A.ġ.", "A.Ş.",
	"A.ġ", "A.Ş",
	"A.Ģ.", "A.Ş.",
	"A.Ģ", "A.Ş",
	"Ġ", "İ",
	"ġ", "ş",
	"Ģ", "ş",
	"¤", "ğ",
	"⁄", "Ğ",
	"›", "ı",
	"‹", "İ",
	"ﬂ", "ş",
	"ﬁ", "fi",
	"fll", "şl",
	"fli", "şi",
	"flı", "şı",
	"fl", "ş",
	"FL", "Ş",
)

type DocumentIndex struct {
	Ticker      string               `json:"ticker"`
	GeneratedAt string               `json:"generated_at"`
	Sector      CompanySectorContext `json:"sector"`
	Counts      DocumentIndexCounts  `json:"counts"`
	Documents   []IndexedDocument    `json:"documents"`
	Groups      []DocumentGroupIndex `json:"groups"`
	Warnings    []string             `json:"warnings,omitempty"`
}

type DocumentIndexCounts struct {
	Documents          int `json:"documents"`
	AnalysisUsableDocs int `json:"analysis_usable_docs"`
	AIResolvedDocs     int `json:"ai_resolved_docs,omitempty"`
	ReviewRequiredDocs int `json:"review_required_docs"`
	RejectedDocs       int `json:"rejected_docs"`
	DocumentFacts      int `json:"document_facts"`
	FinancialFacts     int `json:"financial_facts"`
	FinancialTables    int `json:"financial_tables"`
	People             int `json:"people"`
	OwnershipFacts     int `json:"ownership_facts"`
	CorporateEvents    int `json:"corporate_events"`
	LowQualityDocs     int `json:"low_quality_docs"`
	ReviewRequired     int `json:"review_required"`
}

type IndexedDocument struct {
	DocumentID          string               `json:"document_id"`
	Ticker              string               `json:"ticker"`
	FilePath            string               `json:"file_path"`
	FileName            string               `json:"file_name"`
	SHA256              string               `json:"sha256"`
	DocumentTypeGuess   string               `json:"document_type_guess"`
	ExtractionMethod    string               `json:"extraction_method"`
	TextLength          int                  `json:"text_length"`
	QualityScore        float64              `json:"quality_score"`
	ParseStatus         string               `json:"parse_status,omitempty"`
	AnalysisUsable      bool                 `json:"analysis_usable"`
	HumanReviewRequired bool                 `json:"human_review_required,omitempty"`
	AIResolved          bool                 `json:"ai_resolved,omitempty"`
	AIConfidence        float64              `json:"ai_confidence,omitempty"`
	QualityGate         QualityGate          `json:"quality_gate,omitempty"`
	DocumentDate        *string              `json:"document_date"`
	Period              *string              `json:"period"`
	FactCount           int                  `json:"fact_count"`
	FinancialFactCount  int                  `json:"financial_fact_count"`
	FinancialTableCount int                  `json:"financial_table_count"`
	PeopleCount         int                  `json:"people_count"`
	OwnershipFactCount  int                  `json:"ownership_fact_count"`
	CorporateEventCount int                  `json:"corporate_event_count"`
	Warnings            []string             `json:"warnings,omitempty"`
	Sector              CompanySectorContext `json:"sector"`
	BusinessModels      []BusinessModelTag   `json:"business_models,omitempty"`
	ExtractionRoute     ExtractionRoute      `json:"extraction_route"`
}

type DocumentGroupIndex struct {
	Group string `json:"group"`
	Count int    `json:"count"`
}

type CompanyKnowledgeGraph struct {
	Ticker                 string                             `json:"ticker"`
	GeneratedAt            string                             `json:"generated_at"`
	Sector                 CompanySectorContext               `json:"sector"`
	Counts                 DocumentIndexCounts                `json:"counts"`
	Nodes                  []KnowledgeGraphNode               `json:"nodes"`
	Edges                  []KnowledgeGraphEdge               `json:"edges"`
	DuplicateMergeSummary  KnowledgeDuplicateMergeSummary     `json:"duplicate_merge_summary"`
	Contradictions         []KnowledgeContradiction           `json:"-"`
	ResolvedContradictions []KnowledgeContradictionResolution `json:"resolved_reconciliations,omitempty"`
	SourceFiles            map[string]string                  `json:"source_files"`
	Warnings               []string                           `json:"warnings,omitempty"`
}

func (g *CompanyKnowledgeGraph) UnmarshalJSON(data []byte) error {
	type graphAlias CompanyKnowledgeGraph
	aux := struct {
		*graphAlias
		LegacyContradictions         []KnowledgeContradiction           `json:"contradictions,omitempty"`
		LegacyResolvedContradictions []KnowledgeContradictionResolution `json:"resolved_contradictions,omitempty"`
	}{
		graphAlias: (*graphAlias)(g),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(g.Contradictions) == 0 && len(aux.LegacyContradictions) > 0 {
		g.Contradictions = aux.LegacyContradictions
	}
	if len(g.ResolvedContradictions) == 0 && len(aux.LegacyResolvedContradictions) > 0 {
		g.ResolvedContradictions = aux.LegacyResolvedContradictions
	}
	return nil
}

type KnowledgeGraphNode struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Label      string              `json:"label"`
	Attributes map[string]any      `json:"attributes,omitempty"`
	Evidence   []KnowledgeEvidence `json:"evidence,omitempty"`
}

type KnowledgeGraphEdge struct {
	ID         string              `json:"id"`
	From       string              `json:"from"`
	To         string              `json:"to"`
	Type       string              `json:"type"`
	Weight     int                 `json:"weight,omitempty"`
	Evidence   []KnowledgeEvidence `json:"evidence,omitempty"`
	Attributes map[string]any      `json:"attributes,omitempty"`
}

type KnowledgeEvidence struct {
	SourceFile string  `json:"source_file,omitempty"`
	SHA256     string  `json:"sha256,omitempty"`
	Period     *string `json:"period,omitempty"`
	Snippet    string  `json:"snippet,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type KnowledgeDuplicateMergeSummary struct {
	Documents          int `json:"documents"`
	DistinctDocuments  int `json:"distinct_documents"`
	DuplicateDocuments int `json:"duplicate_documents"`
	FactRows           int `json:"fact_rows"`
	DistinctFactKeys   int `json:"distinct_fact_keys"`
	DuplicateFactRows  int `json:"duplicate_fact_rows"`
}

type KnowledgeContradiction struct {
	Type     string                       `json:"type"`
	Key      string                       `json:"key"`
	Severity string                       `json:"severity"`
	Values   []KnowledgeContradictionItem `json:"values"`
	Evidence []KnowledgeEvidence          `json:"evidence,omitempty"`
}

type KnowledgeContradictionItem struct {
	Value      float64 `json:"value"`
	Currency   string  `json:"currency,omitempty"`
	Unit       string  `json:"unit,omitempty"`
	SourceFile string  `json:"source_file,omitempty"`
	Period     *string `json:"period,omitempty"`
}

type KnowledgeContradictionResolution struct {
	Type               string                       `json:"type"`
	Key                string                       `json:"key"`
	Status             string                       `json:"status"`
	SelectedValue      float64                      `json:"selected_value"`
	Currency           string                       `json:"currency,omitempty"`
	Unit               string                       `json:"unit,omitempty"`
	SelectedSourceFile string                       `json:"selected_source_file,omitempty"`
	Period             *string                      `json:"period,omitempty"`
	Reason             string                       `json:"reason"`
	Confidence         float64                      `json:"confidence"`
	CompetingValues    []KnowledgeContradictionItem `json:"competing_values,omitempty"`
	Evidence           []KnowledgeEvidence          `json:"evidence,omitempty"`
}

type DocumentFact struct {
	ID                string                `json:"id"`
	Ticker            string                `json:"ticker"`
	SourceFile        string                `json:"source_file"`
	SHA256            string                `json:"sha256"`
	DocumentTypeGuess string                `json:"document_type_guess"`
	DocumentDate      *string               `json:"document_date"`
	Period            *string               `json:"period"`
	Group             string                `json:"group"`
	Kind              string                `json:"kind"`
	Label             string                `json:"label"`
	NormalizedKey     string                `json:"normalized_key"`
	RawValue          string                `json:"raw_value,omitempty"`
	NumericValue      *float64              `json:"numeric_value,omitempty"`
	Currency          string                `json:"currency,omitempty"`
	Unit              string                `json:"unit,omitempty"`
	Source            DocumentFactSource    `json:"source"`
	Confidence        float64               `json:"confidence"`
	ReviewRequired    bool                  `json:"review_required"`
	Warnings          []string              `json:"warnings,omitempty"`
	Certification     EvidenceCertification `json:"certification,omitempty"`
	CreatedAt         string                `json:"created_at"`
}

type DocumentFactSource struct {
	Page        int      `json:"page,omitempty"`
	Line        int      `json:"line,omitempty"`
	TableID     string   `json:"table_id,omitempty"`
	RowIndex    int      `json:"row_index,omitempty"`
	ColumnIndex int      `json:"column_index,omitempty"`
	Cells       []string `json:"cells,omitempty"`
	Snippet     string   `json:"snippet"`
}

const (
	EvidenceStatusCertified      = "certified"
	EvidenceStatusAIResolved     = "ai_resolved"
	EvidenceStatusReviewRequired = "review_required"
	EvidenceStatusRejected       = "rejected"
)

type EvidenceCertification struct {
	Status                   string   `json:"status,omitempty"`
	Score                    int      `json:"score"`
	AnalysisUsable           bool     `json:"analysis_usable"`
	EvidenceComplete         bool     `json:"evidence_complete"`
	NormalizationComplete    bool     `json:"normalization_complete"`
	RequiresHumanReview      bool     `json:"requires_human_review,omitempty"`
	AIResolved               bool     `json:"ai_resolved,omitempty"`
	AIConfidence             float64  `json:"ai_confidence,omitempty"`
	Reasons                  []string `json:"reasons,omitempty"`
	RequiredForCertification []string `json:"required_for_certification,omitempty"`
}

type ExtractedFinancialFact struct {
	ID                 string                `json:"id"`
	Ticker             string                `json:"ticker"`
	SourceFile         string                `json:"source_file"`
	SHA256             string                `json:"sha256"`
	DocumentDate       *string               `json:"document_date"`
	Period             *string               `json:"period"`
	StatementType      string                `json:"statement_type"`
	LineItemOriginal   string                `json:"line_item_original"`
	LineItemNormalized string                `json:"line_item_normalized"`
	Value              float64               `json:"value"`
	Currency           string                `json:"currency"`
	Unit               string                `json:"unit"`
	ConsolidationScope string                `json:"consolidation_scope,omitempty"`
	AuditStatus        string                `json:"audit_status,omitempty"`
	Source             DocumentFactSource    `json:"source"`
	Confidence         float64               `json:"confidence"`
	ReviewRequired     bool                  `json:"review_required"`
	Warnings           []string              `json:"warnings,omitempty"`
	Certification      EvidenceCertification `json:"certification,omitempty"`
	CreatedAt          string                `json:"created_at"`
}

type ExtractedFinancialTable struct {
	ID                 string                `json:"id"`
	Ticker             string                `json:"ticker"`
	SourceFile         string                `json:"source_file"`
	SHA256             string                `json:"sha256"`
	DocumentDate       *string               `json:"document_date"`
	Period             *string               `json:"period"`
	TableType          string                `json:"table_type"`
	Currency           string                `json:"currency,omitempty"`
	Unit               string                `json:"unit,omitempty"`
	ConsolidationScope string                `json:"consolidation_scope,omitempty"`
	AuditStatus        string                `json:"audit_status,omitempty"`
	Rows               []FinancialTableRow   `json:"rows,omitempty"`
	Source             DocumentFactSource    `json:"source"`
	Confidence         float64               `json:"confidence"`
	ReviewRequired     bool                  `json:"review_required"`
	Warnings           []string              `json:"warnings,omitempty"`
	Certification      EvidenceCertification `json:"certification,omitempty"`
	CreatedAt          string                `json:"created_at"`
}

type FinancialTableRow struct {
	RowIndex int      `json:"row_index"`
	Cells    []string `json:"cells"`
	Snippet  string   `json:"snippet"`
}

type ExtractedPerson struct {
	ID             string             `json:"id"`
	Ticker         string             `json:"ticker"`
	SourceFile     string             `json:"source_file"`
	SHA256         string             `json:"sha256"`
	DocumentDate   *string            `json:"document_date"`
	Period         *string            `json:"period"`
	FullName       string             `json:"full_name"`
	NormalizedName string             `json:"normalized_name"`
	Title          string             `json:"title,omitempty"`
	Role           string             `json:"role"`
	Source         DocumentFactSource `json:"source"`
	Confidence     float64            `json:"confidence"`
	ReviewRequired bool               `json:"review_required"`
	CreatedAt      string             `json:"created_at"`
}

type OwnershipFact struct {
	ID             string             `json:"id"`
	Ticker         string             `json:"ticker"`
	SourceFile     string             `json:"source_file"`
	SHA256         string             `json:"sha256"`
	DocumentDate   *string            `json:"document_date"`
	Period         *string            `json:"period"`
	HolderName     string             `json:"holder_name"`
	ShareAmount    *float64           `json:"share_amount,omitempty"`
	ShareRatio     *float64           `json:"share_ratio,omitempty"`
	Source         DocumentFactSource `json:"source"`
	Confidence     float64            `json:"confidence"`
	ReviewRequired bool               `json:"review_required"`
	CreatedAt      string             `json:"created_at"`
}

type ExtractedCorporateEvent struct {
	ID                string                `json:"id"`
	Ticker            string                `json:"ticker"`
	SourceFile        string                `json:"source_file"`
	SHA256            string                `json:"sha256"`
	DocumentDate      *string               `json:"document_date"`
	Period            *string               `json:"period"`
	EventType         string                `json:"event_type"`
	Title             string                `json:"title"`
	Amount            *float64              `json:"amount,omitempty"`
	Ratio             *float64              `json:"ratio,omitempty"`
	EffectiveDate     *string               `json:"effective_date,omitempty"`
	PaymentDate       *string               `json:"payment_date,omitempty"`
	SubscriptionPrice *float64              `json:"subscription_price,omitempty"`
	Currency          string                `json:"currency,omitempty"`
	Source            DocumentFactSource    `json:"source"`
	Confidence        float64               `json:"confidence"`
	ReviewRequired    bool                  `json:"review_required"`
	Warnings          []string              `json:"warnings,omitempty"`
	Certification     EvidenceCertification `json:"certification,omitempty"`
	CreatedAt         string                `json:"created_at"`
}

type documentIntelligenceExtraction struct {
	IndexDoc        IndexedDocument
	Facts           []DocumentFact
	FinancialFacts  []ExtractedFinancialFact
	FinancialTables []ExtractedFinancialTable
	People          []ExtractedPerson
	OwnershipFacts  []OwnershipFact
	CorporateEvents []ExtractedCorporateEvent
}

type documentFinancialContext struct {
	Currency           string
	Unit               string
	ConsolidationScope string
	AuditStatus        string
}

type indexedTextLine struct {
	Page int
	Line int
	Text string
}

type tableRowCandidate struct {
	TableID  string
	Page     int
	Line     int
	RowIndex int
	Cells    []string
	Raw      string
}

type rawDocumentIntelligenceInput struct {
	Index int
	Doc   RawDocument
}

var (
	percentRE       = regexp.MustCompile(`(?:%\s*(\d{1,3}(?:[.,]\d{1,4})?)|(\d{1,3}(?:[.,]\d{1,4})?)\s*%)`)
	emailRE         = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	websiteRE       = regexp.MustCompile(`(?i)\b(?:https?://)?(?:www\.)?[a-z0-9][a-z0-9\-]+(?:\.[a-z0-9\-]+)+\b`)
	phoneRE         = regexp.MustCompile(`(?i)(?:\+90|0)?\s*\(?\d{3}\)?[\s.-]?\d{3}[\s.-]?\d{2}[\s.-]?\d{2}`)
	capitalValueRE  = regexp.MustCompile(`(?i)(?:sermaye|capital)[^0-9]{0,80}(\d{1,3}(?:[.\s]\d{3})+(?:,\d+)?|\d+(?:,\d+)?)`)
	linePrefixRE    = regexp.MustCompile(`^\s*(?:[A-ZÇĞİÖŞÜ]{0,3}\d+[A-ZÇĞİÖŞÜa-zçğıöşü]?|[A-ZÇĞİÖŞÜ]|[ivxlcdm]+|[a-zçğıöşü])[\.)-]\s+`)
	noteIndexLineRE = regexp.MustCompile(`^\s*\d{1,3}\s+`)
)

const maxStructuredFactLineRunes = 1800

func ExtractDocumentIntelligenceFromRawDocuments(ctx context.Context, opts DocumentIntelligenceOptions) (DocumentIntelligenceSummary, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return DocumentIntelligenceSummary{}, errors.New("output dir is required")
	}
	rawPath := strings.TrimSpace(opts.RawDocumentsPath)
	if rawPath == "" {
		rawPath = filepath.Join(opts.OutputDir, RawDocumentsFile)
	}
	summary := DocumentIntelligenceSummary{
		Status:           "ok",
		RawDocumentsPath: filepath.Clean(rawPath),
		OutputDir:        filepath.Clean(opts.OutputDir),
	}
	if _, err := os.Stat(rawPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			summary.Status = "missing_raw_documents"
			summary.Warnings = append(summary.Warnings, "raw_documents_jsonl_missing")
			return summary, nil
		}
		return summary, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return summary, err
	}
	paths := documentIntelligenceGlobalPaths(opts.OutputDir)
	for _, path := range paths {
		_ = os.Remove(path)
	}
	encoders, closers, err := openDocumentIntelligenceEncoders(paths)
	if err != nil {
		return summary, err
	}
	defer closeDocumentIntelligenceFiles(closers)

	rawFile, err := os.Open(rawPath)
	if err != nil {
		return summary, err
	}
	defer rawFile.Close()

	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	createdAt := now().UTC().Format(time.RFC3339)
	sectorStore := LoadSectorContextStore(opts.KAPSectorsPath, opts.PromptPackPath)
	summary.Warnings = append(summary.Warnings, sectorStore.Warnings...)
	inputs := []rawDocumentIntelligenceInput{}
	scanner := bufio.NewScanner(rawFile)
	scanner.Buffer(make([]byte, 256*1024), 128*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			summary.Status = "cancelled"
			return summary, err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var doc RawDocument
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			summary.Warnings = append(summary.Warnings, "raw_document_parse_failed")
			continue
		}
		summary.RawDocuments++
		inputs = append(inputs, rawDocumentIntelligenceInput{Index: len(inputs), Doc: doc})
	}
	if err := scanner.Err(); err != nil {
		return summary, err
	}

	extractions := extractDocumentIntelligenceBatch(ctx, inputs, createdAt, sectorStore, opts.Workers)
	byTicker := map[string][]documentIntelligenceExtraction{}
	for _, extracted := range extractions {
		if err := ctx.Err(); err != nil {
			summary.Status = "cancelled"
			return summary, err
		}
		if extracted.IndexDoc.AnalysisUsable {
			summary.AnalysisUsableDocuments++
		}
		if extracted.IndexDoc.AIResolved {
			summary.AIResolvedDocuments++
		}
		if extracted.IndexDoc.HumanReviewRequired {
			summary.ReviewRequiredDocuments++
		}
		if extracted.IndexDoc.ParseStatus == ParseStatusRejected {
			summary.RejectedDocuments++
		}
		ticker := extracted.IndexDoc.Ticker
		if ticker == "" {
			ticker = "UNKNOWN"
		}
		byTicker[ticker] = append(byTicker[ticker], extracted)
		for _, fact := range extracted.Facts {
			if err := encoders[DocumentFactsFile].Encode(fact); err != nil {
				return summary, err
			}
			summary.DocumentFacts++
		}
		for _, fact := range extracted.FinancialFacts {
			if err := encoders[FinancialFactsFile].Encode(fact); err != nil {
				return summary, err
			}
			summary.FinancialFacts++
		}
		for _, table := range extracted.FinancialTables {
			if err := encoders[FinancialTablesFile].Encode(table); err != nil {
				return summary, err
			}
			summary.FinancialTables++
		}
		for _, person := range extracted.People {
			if err := encoders[PeopleFile].Encode(person); err != nil {
				return summary, err
			}
			summary.People++
		}
		for _, fact := range extracted.OwnershipFacts {
			if err := encoders[OwnershipFactsFile].Encode(fact); err != nil {
				return summary, err
			}
			summary.OwnershipFacts++
		}
		for _, event := range extracted.CorporateEvents {
			if err := encoders[CorporateFactsFile].Encode(event); err != nil {
				return summary, err
			}
			summary.CorporateEvents++
		}
	}
	tickers := make([]string, 0, len(byTicker))
	for ticker := range byTicker {
		tickers = append(tickers, ticker)
	}
	sort.Strings(tickers)
	for _, ticker := range tickers {
		index, facts, financials, tables, people, ownership, corporate := buildDocumentIndex(ticker, byTicker[ticker], createdAt, !opts.SkipKnowledgeGraph)
		graph := CompanyKnowledgeGraph{}
		if opts.SkipKnowledgeGraph {
			graph = skippedCompanyKnowledgeGraph(index, createdAt)
		} else {
			graph = buildCompanyKnowledgeGraph(index, facts, financials, tables, people, ownership, corporate, createdAt)
		}
		tickerDir := filepath.Join(opts.OutputDir, "by_ticker", ticker)
		if err := os.MkdirAll(tickerDir, 0o755); err != nil {
			return summary, err
		}
		if err := writeJSON(filepath.Join(tickerDir, DocumentIndexFile), index); err != nil {
			return summary, err
		}
		if err := writeJSON(filepath.Join(tickerDir, CompanyKnowledgeGraphFile), graph); err != nil {
			return summary, err
		}
		files := map[string]any{
			DocumentFactsFile:   facts,
			FinancialFactsFile:  financials,
			FinancialTablesFile: tables,
			PeopleFile:          people,
			OwnershipFactsFile:  ownership,
			CorporateFactsFile:  corporate,
		}
		for name, rows := range files {
			path := filepath.Join(tickerDir, name)
			if err := writeJSONLRows(path, rows); err != nil {
				return summary, err
			}
			summary.OutputFiles = append(summary.OutputFiles, path)
		}
		summary.OutputFiles = append(summary.OutputFiles, filepath.Join(tickerDir, DocumentIndexFile))
		summary.OutputFiles = append(summary.OutputFiles, filepath.Join(tickerDir, CompanyKnowledgeGraphFile))
		summary.DocumentIndexes++
		if !opts.SkipKnowledgeGraph {
			summary.KnowledgeGraphs++
		}
	}
	summary.Tickers = len(tickers)
	summary.OutputFiles = append(globalDocumentIntelligenceOutputFiles(opts.OutputDir), summary.OutputFiles...)
	if summary.DocumentFacts == 0 {
		summary.Status = "no_document_facts_found"
		summary.Warnings = append(summary.Warnings, "document_facts_empty")
	}
	summary.Warnings = dedupeStrings(summary.Warnings)
	return summary, nil
}

func ExtractDocumentIntelligence(doc RawDocument, createdAt string) documentIntelligenceExtraction {
	return ExtractDocumentIntelligenceWithContext(doc, createdAt, SectorContextStore{Symbols: map[string]CompanySectorContext{}})
}

func extractDocumentIntelligenceBatch(ctx context.Context, inputs []rawDocumentIntelligenceInput, createdAt string, sectorStore SectorContextStore, requestedWorkers int) []documentIntelligenceExtraction {
	if len(inputs) == 0 {
		return nil
	}
	workers := documentIntelligenceWorkerCount(requestedWorkers, len(inputs))
	if workers <= 1 {
		out := make([]documentIntelligenceExtraction, len(inputs))
		for _, input := range inputs {
			if ctx.Err() != nil {
				return out
			}
			out[input.Index] = ExtractDocumentIntelligenceWithContext(input.Doc, createdAt, sectorStore)
		}
		return out
	}
	out := make([]documentIntelligenceExtraction, len(inputs))
	jobs := make(chan rawDocumentIntelligenceInput)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for input := range jobs {
				if ctx.Err() != nil {
					continue
				}
				out[input.Index] = ExtractDocumentIntelligenceWithContext(input.Doc, createdAt, sectorStore)
			}
		}()
	}
	for _, input := range inputs {
		if ctx.Err() != nil {
			break
		}
		jobs <- input
	}
	close(jobs)
	wg.Wait()
	return out
}

func documentIntelligenceWorkerCount(requested, total int) int {
	if total <= 1 {
		return total
	}
	if requested <= 0 {
		requested = runtime.NumCPU()
	}
	if requested < 1 {
		requested = 1
	}
	if requested > total {
		requested = total
	}
	return requested
}

func ExtractDocumentIntelligenceWithContext(doc RawDocument, createdAt string, sectorStore SectorContextStore) documentIntelligenceExtraction {
	doc.Ticker = strings.ToUpper(strings.TrimSpace(doc.Ticker))
	if doc.Ticker == "" {
		doc.Ticker = ExtractTicker("", doc.FilePath)
	}
	doc.DocumentTypeGuess = NormalizeDocumentType(firstNonEmptyAsset(doc.DocumentTypeGuess, ClassifyDocument(doc.Text, doc.FileName)))
	gate := QualityGateForRawDocument(doc)
	doc.ParseStatus = gate.Status
	doc.AnalysisUsable = gate.AnalysisUsable
	doc.HumanReviewRequired = gate.HumanReviewRequired
	doc.AIResolved = gate.AIResolved
	doc.AIConfidence = gate.AIConfidence
	doc.QualityGate = gate
	route := BuildExtractionRoute(doc, sectorStore)
	structuredRescue := (!gate.AnalysisUsable && RawDocumentStructuredRescueUsable(doc)) ||
		(gate.AIResolved && containsRawWarning(doc.Warnings, lowQualityStructuredRescueWarning))
	parseDoc := doc
	effectiveGate := gate
	if structuredRescue {
		parseDoc = RawDocumentForStructuredRescue(doc)
		route = enableStructuredRescueRoute(route, parseDoc)
		effectiveGate = parseDoc.QualityGate
	}
	documentDate := extractDateString(firstNonEmptyAsset(doc.FileName, doc.Text))
	period := extractPeriodString(firstNonEmptyAsset(doc.FileName, doc.Text))
	indexWarnings := append([]string{}, doc.Warnings...)
	if structuredRescue {
		indexWarnings = append([]string{}, parseDoc.Warnings...)
	}
	out := documentIntelligenceExtraction{
		IndexDoc: IndexedDocument{
			DocumentID:          documentIDForRawDocument(doc),
			Ticker:              doc.Ticker,
			FilePath:            doc.FilePath,
			FileName:            doc.FileName,
			SHA256:              doc.SHA256,
			DocumentTypeGuess:   doc.DocumentTypeGuess,
			ExtractionMethod:    doc.ExtractionMethod,
			TextLength:          doc.TextLength,
			QualityScore:        doc.QualityScore,
			ParseStatus:         effectiveGate.Status,
			AnalysisUsable:      effectiveGate.AnalysisUsable,
			HumanReviewRequired: effectiveGate.HumanReviewRequired,
			AIResolved:          effectiveGate.AIResolved,
			AIConfidence:        effectiveGate.AIConfidence,
			QualityGate:         effectiveGate,
			DocumentDate:        documentDate,
			Period:              period,
			Warnings:            indexWarnings,
			Sector:              route.Sector,
			BusinessModels:      route.BusinessModels,
			ExtractionRoute:     route,
		},
	}
	if !effectiveGate.AnalysisUsable {
		out.IndexDoc.Warnings = append(out.IndexDoc.Warnings, QualityGateWarnings(effectiveGate)...)
		out.IndexDoc.Warnings = append(out.IndexDoc.Warnings, "low_quality_document_index_needs_review")
		if structuredRescue {
			out.IndexDoc.Warnings = append(out.IndexDoc.Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning)
		}
		out.IndexDoc.Warnings = dedupeStrings(out.IndexDoc.Warnings)
		if !structuredRescue {
			return out
		}
	}
	financialContext := newDocumentFinancialContext(parseDoc)
	lines := indexedDocumentLines(parseDoc.Text)
	rows := tableRowsFromIndexedLines(lines, parseDoc.SHA256)
	sectionByLine := financialSectionsByIndexedLine(lines)
	out.FinancialTables = financialTablesFromRows(parseDoc, rows, documentDate, period, createdAt, route, financialContext)
	seenFacts := map[string]bool{}
	seenFinancials := map[string]bool{}
	seenPeople := map[string]bool{}
	seenOwnership := map[string]bool{}
	seenCorporate := map[string]bool{}
	for _, row := range rows {
		out.addDocumentFacts(parseDoc, documentDate, period, factsFromTableRow(parseDoc, row, createdAt), seenFacts)
		out.addFinancialFacts(financialFactsFromTableRow(parseDoc, row, documentDate, period, createdAt, financialContext, sectionByLine[indexedLineKey(row.Page, row.Line)]), seenFinancials)
		out.addPeople(peopleFromLine(parseDoc, row.Page, row.Line, row.Raw, row.Cells, documentDate, period, createdAt), seenPeople)
		out.addOwnership(ownershipFactsFromLine(parseDoc, row.Page, row.Line, row.Raw, row.Cells, documentDate, period, createdAt), seenOwnership)
		out.addCorporate(corporateEventsFromLine(parseDoc, row.Page, row.Line, row.Raw, row.Cells, documentDate, period, createdAt), seenCorporate)
	}
	for _, line := range lines {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		if lineTooLongForStructuredFact(line.Text) {
			continue
		}
		cells := splitAssetCells(line.Text)
		out.addDocumentFacts(parseDoc, documentDate, period, factsFromFreeTextLine(parseDoc, line, cells, createdAt), seenFacts)
		out.addFinancialFacts(financialFactsFromLine(parseDoc, line.Page, line.Line, line.Text, cells, documentDate, period, createdAt, financialContext, sectionByLine[indexedLineKey(line.Page, line.Line)]), seenFinancials)
		out.addPeople(peopleFromLine(parseDoc, line.Page, line.Line, line.Text, cells, documentDate, period, createdAt), seenPeople)
		out.addOwnership(ownershipFactsFromLine(parseDoc, line.Page, line.Line, line.Text, cells, documentDate, period, createdAt), seenOwnership)
		out.addCorporate(corporateEventsFromLine(parseDoc, line.Page, line.Line, line.Text, cells, documentDate, period, createdAt), seenCorporate)
	}
	if structuredRescue {
		markStructuredRescueExtraction(&out, parseDoc)
	}
	out.IndexDoc.FactCount = len(out.Facts)
	out.IndexDoc.FinancialFactCount = len(out.FinancialFacts)
	out.IndexDoc.FinancialTableCount = len(out.FinancialTables)
	out.IndexDoc.PeopleCount = len(out.People)
	out.IndexDoc.OwnershipFactCount = len(out.OwnershipFacts)
	out.IndexDoc.CorporateEventCount = len(out.CorporateEvents)
	if effectiveGate.Status != ParseStatusTrusted && effectiveGate.Status != ParseStatusAIResolved {
		out.IndexDoc.Warnings = append(out.IndexDoc.Warnings, "low_quality_document_index_needs_review")
	}
	out.IndexDoc.Warnings = dedupeStrings(out.IndexDoc.Warnings)
	return out
}

func (out *documentIntelligenceExtraction) addDocumentFacts(_ RawDocument, _ *string, _ *string, rows []DocumentFact, seen map[string]bool) {
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out.Facts = append(out.Facts, row)
	}
}

func (out *documentIntelligenceExtraction) addFinancialFacts(rows []ExtractedFinancialFact, seen map[string]bool) {
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out.FinancialFacts = append(out.FinancialFacts, row)
	}
}

func (out *documentIntelligenceExtraction) addPeople(rows []ExtractedPerson, seen map[string]bool) {
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out.People = append(out.People, row)
	}
}

func (out *documentIntelligenceExtraction) addOwnership(rows []OwnershipFact, seen map[string]bool) {
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out.OwnershipFacts = append(out.OwnershipFacts, row)
	}
}

func (out *documentIntelligenceExtraction) addCorporate(rows []ExtractedCorporateEvent, seen map[string]bool) {
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out.CorporateEvents = append(out.CorporateEvents, row)
	}
}

func enableStructuredRescueRoute(route ExtractionRoute, doc RawDocument) ExtractionRoute {
	route.UniversalExtractor = true
	route.FinancialTableParser = documentNeedsFinancialTableParser(route.DocumentType) || structuredRescueHasFinancialRows(doc.Text)
	route.HumanReviewRequired = !doc.AIResolved
	route.AIResolved = doc.AIResolved
	route.AIConfidence = doc.AIConfidence
	if doc.AIResolved {
		route.Warnings = dedupeStrings(append(route.Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
	} else {
		route.Warnings = dedupeStrings(append(route.Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning))
	}
	return route
}

func structuredRescueHasFinancialRows(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if financialLineLooksRelevant(line) || classifyFinancialTableType(line) != "" {
			return true
		}
	}
	return false
}

func markStructuredRescueExtraction(out *documentIntelligenceExtraction, doc RawDocument) {
	if out == nil {
		return
	}
	if doc.AIResolved {
		markAIResolvedStructuredRescueExtraction(out, doc)
		return
	}
	out.IndexDoc.HumanReviewRequired = true
	out.IndexDoc.Warnings = dedupeStrings(append(out.IndexDoc.Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning))
	out.IndexDoc.ExtractionRoute = enableStructuredRescueRoute(out.IndexDoc.ExtractionRoute, doc)
	for i := range out.Facts {
		out.Facts[i].ReviewRequired = true
		if out.Facts[i].Confidence > 0.82 {
			out.Facts[i].Confidence = 0.82
		}
		out.Facts[i].Warnings = dedupeStrings(append(out.Facts[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning))
	}
	for i := range out.FinancialFacts {
		out.FinancialFacts[i].ReviewRequired = true
		if out.FinancialFacts[i].Confidence > 0.82 {
			out.FinancialFacts[i].Confidence = 0.82
		}
		out.FinancialFacts[i].Warnings = dedupeStrings(append(out.FinancialFacts[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning))
		downgradeCertificationForStructuredRescue(&out.FinancialFacts[i].Certification)
	}
	for i := range out.FinancialTables {
		out.FinancialTables[i].ReviewRequired = true
		if out.FinancialTables[i].Confidence > 0.82 {
			out.FinancialTables[i].Confidence = 0.82
		}
		out.FinancialTables[i].Warnings = dedupeStrings(append(out.FinancialTables[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning))
		downgradeCertificationForStructuredRescue(&out.FinancialTables[i].Certification)
	}
	for i := range out.People {
		out.People[i].ReviewRequired = true
		if out.People[i].Confidence > 0.82 {
			out.People[i].Confidence = 0.82
		}
	}
	for i := range out.OwnershipFacts {
		out.OwnershipFacts[i].ReviewRequired = true
		if out.OwnershipFacts[i].Confidence > 0.82 {
			out.OwnershipFacts[i].Confidence = 0.82
		}
	}
	for i := range out.CorporateEvents {
		out.CorporateEvents[i].ReviewRequired = true
		if out.CorporateEvents[i].Confidence > 0.82 {
			out.CorporateEvents[i].Confidence = 0.82
		}
		out.CorporateEvents[i].Warnings = dedupeStrings(append(out.CorporateEvents[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning))
		downgradeCertificationForStructuredRescue(&out.CorporateEvents[i].Certification)
	}
}

func markAIResolvedStructuredRescueExtraction(out *documentIntelligenceExtraction, doc RawDocument) {
	out.IndexDoc.AnalysisUsable = true
	out.IndexDoc.HumanReviewRequired = false
	out.IndexDoc.AIResolved = true
	out.IndexDoc.AIConfidence = doc.AIConfidence
	out.IndexDoc.ParseStatus = ParseStatusAIResolved
	out.IndexDoc.QualityGate = doc.QualityGate
	out.IndexDoc.Warnings = dedupeStrings(append(out.IndexDoc.Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
	out.IndexDoc.ExtractionRoute = enableStructuredRescueRoute(out.IndexDoc.ExtractionRoute, doc)
	for i := range out.Facts {
		out.Facts[i].ReviewRequired = false
		if out.Facts[i].Confidence < doc.AIConfidence {
			out.Facts[i].Confidence = doc.AIConfidence
		}
		if out.Facts[i].Confidence > 0.88 {
			out.Facts[i].Confidence = 0.88
		}
		out.Facts[i].Warnings = removeWarnings(out.Facts[i].Warnings, structuredRescueReviewWarning, "needs_human_review")
		out.Facts[i].Warnings = dedupeStrings(append(out.Facts[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
	}
	for i := range out.FinancialFacts {
		out.FinancialFacts[i].ReviewRequired = false
		if out.FinancialFacts[i].Confidence < doc.AIConfidence {
			out.FinancialFacts[i].Confidence = doc.AIConfidence
		}
		if out.FinancialFacts[i].Confidence > 0.88 {
			out.FinancialFacts[i].Confidence = 0.88
		}
		out.FinancialFacts[i].Warnings = removeWarnings(out.FinancialFacts[i].Warnings, structuredRescueReviewWarning, "needs_human_review")
		out.FinancialFacts[i].Warnings = dedupeStrings(append(out.FinancialFacts[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
		aiResolveCertificationForStructuredRescue(&out.FinancialFacts[i].Certification, doc)
	}
	for i := range out.FinancialTables {
		out.FinancialTables[i].ReviewRequired = false
		if out.FinancialTables[i].Confidence < doc.AIConfidence {
			out.FinancialTables[i].Confidence = doc.AIConfidence
		}
		if out.FinancialTables[i].Confidence > 0.88 {
			out.FinancialTables[i].Confidence = 0.88
		}
		out.FinancialTables[i].Warnings = removeWarnings(out.FinancialTables[i].Warnings, structuredRescueReviewWarning, "needs_human_review")
		out.FinancialTables[i].Warnings = dedupeStrings(append(out.FinancialTables[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
		aiResolveCertificationForStructuredRescue(&out.FinancialTables[i].Certification, doc)
	}
	for i := range out.People {
		out.People[i].ReviewRequired = false
		if out.People[i].Confidence < doc.AIConfidence {
			out.People[i].Confidence = doc.AIConfidence
		}
	}
	for i := range out.OwnershipFacts {
		out.OwnershipFacts[i].ReviewRequired = false
		if out.OwnershipFacts[i].Confidence < doc.AIConfidence {
			out.OwnershipFacts[i].Confidence = doc.AIConfidence
		}
	}
	for i := range out.CorporateEvents {
		out.CorporateEvents[i].ReviewRequired = false
		if out.CorporateEvents[i].Confidence < doc.AIConfidence {
			out.CorporateEvents[i].Confidence = doc.AIConfidence
		}
		if out.CorporateEvents[i].Confidence > 0.88 {
			out.CorporateEvents[i].Confidence = 0.88
		}
		out.CorporateEvents[i].Warnings = removeWarnings(out.CorporateEvents[i].Warnings, structuredRescueReviewWarning, "needs_human_review")
		out.CorporateEvents[i].Warnings = dedupeStrings(append(out.CorporateEvents[i].Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
		aiResolveCertificationForStructuredRescue(&out.CorporateEvents[i].Certification, doc)
	}
}

func aiResolveCertificationForStructuredRescue(cert *EvidenceCertification, doc RawDocument) {
	if cert == nil {
		return
	}
	if !cert.EvidenceComplete {
		cert.Status = EvidenceStatusRejected
		cert.AnalysisUsable = false
		cert.RequiresHumanReview = false
		cert.AIResolved = false
		cert.Reasons = dedupeStrings(append(cert.Reasons, "structured_rescue_evidence_incomplete"))
		return
	}
	cert.Status = EvidenceStatusAIResolved
	cert.AnalysisUsable = true
	cert.RequiresHumanReview = false
	cert.AIResolved = true
	cert.AIConfidence = doc.AIConfidence
	if cert.Score < 80 {
		cert.Score = 80
	}
	cert.Reasons = dedupeStrings(append(cert.Reasons, lowQualityStructuredRescueWarning, aiResolvedByStructuredEvidence))
	cert.RequiredForCertification = nil
}

func removeWarnings(warnings []string, remove ...string) []string {
	if len(warnings) == 0 || len(remove) == 0 {
		return warnings
	}
	blocked := map[string]bool{}
	for _, value := range remove {
		blocked[strings.TrimSpace(value)] = true
	}
	out := warnings[:0]
	for _, warning := range warnings {
		if blocked[strings.TrimSpace(warning)] {
			continue
		}
		out = append(out, warning)
	}
	return out
}

func downgradeCertificationForStructuredRescue(cert *EvidenceCertification) {
	if cert == nil {
		return
	}
	if cert.Status == "" {
		cert.Status = EvidenceStatusReviewRequired
	}
	if cert.Status == EvidenceStatusCertified {
		cert.Status = EvidenceStatusReviewRequired
		if cert.Score > 90 {
			cert.Score = 90
		}
	}
	cert.AnalysisUsable = false
	cert.RequiresHumanReview = false
	cert.Reasons = dedupeStrings(append(cert.Reasons, lowQualityStructuredRescueWarning))
	cert.RequiredForCertification = dedupeStrings(append(cert.RequiredForCertification, "AI vision/OCR reprocess or cleaner text extraction"))
}

func factsFromTableRow(doc RawDocument, row tableRowCandidate, createdAt string) []DocumentFact {
	if len(row.Cells) < 2 || isGenericAssetHeader(row.Raw) {
		return nil
	}
	group, kind, label, normalized := classifyDocumentLine(row.Raw)
	if group == "" {
		return nil
	}
	source := DocumentFactSource{Page: row.Page, Line: row.Line, TableID: row.TableID, RowIndex: row.RowIndex, Cells: row.Cells, Snippet: truncateAssetSnippet(row.Raw, 700)}
	confidence := factConfidence(doc, true, group, row.Raw)
	return []DocumentFact{newDocumentFact(doc, group, kind, label, normalized, "", nil, "", "", source, confidence, createdAt)}
}

func factsFromFreeTextLine(doc RawDocument, line indexedTextLine, cells []string, createdAt string) []DocumentFact {
	group, kind, label, normalized := classifyDocumentLine(line.Text)
	if group == "" {
		return nil
	}
	source := DocumentFactSource{Page: line.Page, Line: line.Line, Cells: cells, Snippet: truncateAssetSnippet(line.Text, 700)}
	confidence := factConfidence(doc, false, group, line.Text)
	return []DocumentFact{newDocumentFact(doc, group, kind, label, normalized, "", nil, "", "", source, confidence, createdAt)}
}

func financialFactsFromTableRow(doc RawDocument, row tableRowCandidate, documentDate, period *string, createdAt string, context documentFinancialContext, sectionHint string) []ExtractedFinancialFact {
	facts := financialFactsFromLine(doc, row.Page, row.Line, row.Raw, row.Cells, documentDate, period, createdAt, context, sectionHint)
	for i := range facts {
		facts[i].Source.TableID = row.TableID
		facts[i].Source.RowIndex = row.RowIndex
		facts[i].Source.Cells = row.Cells
		finalizeFinancialFactCertification(&facts[i])
	}
	return facts
}

func financialTablesFromRows(doc RawDocument, rows []tableRowCandidate, documentDate, period *string, createdAt string, route ExtractionRoute, context documentFinancialContext) []ExtractedFinancialTable {
	if !route.FinancialTableParser {
		return nil
	}
	out := []ExtractedFinancialTable{}
	var current *ExtractedFinancialTable
	flush := func() {
		if current == nil || len(current.Rows) == 0 {
			current = nil
			return
		}
		current.ID = stableFactID("financial_table", doc.SHA256, current.TableType, current.Source.Page, current.Source.Line, len(out))
		current.Source.Snippet = truncateAssetSnippet(joinFinancialTableRows(current.Rows), 900)
		finalizeFinancialTableCertification(current)
		out = append(out, *current)
		current = nil
	}
	for _, row := range rows {
		tableType := classifyFinancialTableType(row.Raw)
		if tableType == "" && current != nil && rowLooksLikeFinancialTableContinuation(row.Raw) {
			tableType = current.TableType
		}
		if tableType == "" {
			flush()
			continue
		}
		if current == nil || current.TableType != tableType || row.Page != current.Source.Page {
			flush()
			confidence := factConfidence(doc, true, "financial", row.Raw)
			current = &ExtractedFinancialTable{
				Ticker:             strings.ToUpper(doc.Ticker),
				SourceFile:         doc.FilePath,
				SHA256:             doc.SHA256,
				DocumentDate:       documentDate,
				Period:             period,
				TableType:          tableType,
				Currency:           context.Currency,
				Unit:               context.Unit,
				ConsolidationScope: context.ConsolidationScope,
				AuditStatus:        context.AuditStatus,
				Source:             DocumentFactSource{Page: row.Page, Line: row.Line, TableID: row.TableID, RowIndex: row.RowIndex, Cells: row.Cells, Snippet: truncateAssetSnippet(row.Raw, 700)},
				Confidence:         confidence,
				ReviewRequired:     confidence < 0.86,
				Warnings:           factWarnings(doc, confidence),
				CreatedAt:          createdAt,
			}
		}
		current.Rows = append(current.Rows, FinancialTableRow{RowIndex: row.RowIndex, Cells: row.Cells, Snippet: truncateAssetSnippet(row.Raw, 700)})
	}
	flush()
	return out
}

func classifyFinancialTableType(line string) string {
	slug := util.SlugTR(line)
	switch {
	case slugContains(slug, "finansal durum tablosu") || slugContains(slug, "varliklar") || slugContains(slug, "yukumlulukler") || slugContains(slug, "ozkaynaklar"):
		return "balance_sheet"
	case slugContains(slug, "kar veya zarar") || slugContains(slug, "gelir tablosu") || slugContains(slug, "hasilat") || slugContains(slug, "net donem kari"):
		return "income_statement"
	case slugContains(slug, "nakit akis") || slugContains(slug, "isletme faaliyetlerinden") || slugContains(slug, "yatirim faaliyetlerinden") || slugContains(slug, "finansman faaliyetlerinden"):
		return "cash_flow_statement"
	case slugContains(slug, "ozkaynak degisim") || slugContains(slug, "gecmis yillar karlari") || slugContains(slug, "odenmis sermaye"):
		return "equity_statement"
	case slugContains(slug, "bolumlere gore raporlama") || slugContains(slug, "segment"):
		return "segment_reporting"
	case slugContains(slug, "iliskili taraf"):
		return "related_party_notes"
	case slugContains(slug, "finansal borc") || slugContains(slug, "krediler") || slugContains(slug, "borclanma"):
		return "debt_notes"
	case slugContains(slug, "taahhut") || slugContains(slug, "kosullu yukumluluk") || slugContains(slug, "kefalet") || slugContains(slug, "teminat"):
		return "commitments_and_contingencies"
	case slugContains(slug, "vergi"):
		return "tax_notes"
	case slugContains(slug, "karsilik"):
		return "provisions"
	case slugContains(slug, "stok"):
		return "inventory_notes"
	case slugContains(slug, "alacak"):
		return "receivables_notes"
	case slugContains(slug, "maddi duran varlik"):
		return "ppe_notes"
	case slugContains(slug, "yatirim amacli gayrimenkul"):
		return "investment_property_notes"
	case slugContains(slug, "maddi olmayan duran varlik"):
		return "intangible_assets_notes"
	case slugContains(slug, "finansal arac") || slugContains(slug, "finansal varlik") || slugContains(slug, "finansal yukumluluk"):
		return "financial_instruments_notes"
	default:
		return ""
	}
}

func rowLooksLikeFinancialTableContinuation(line string) bool {
	if !hasASCIIDigit(line) {
		return false
	}
	return len(extractMoneyAmounts(line)) > 0 && financialLineItem(line, splitAssetCells(line)) != ""
}

func joinFinancialTableRows(rows []FinancialTableRow) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Snippet != "" {
			parts = append(parts, row.Snippet)
		}
	}
	return strings.Join(parts, "\n")
}

func financialFactsFromLine(doc RawDocument, page, lineNumber int, line string, cells []string, documentDate, period *string, createdAt string, context documentFinancialContext, sectionHint string) []ExtractedFinancialFact {
	if !financialLineLooksRelevant(line) {
		return nil
	}
	lineItem := financialLineItem(line, cells)
	if lineItem == "" {
		return nil
	}
	normalized := normalizeFinancialLineItemInSection(lineItem, sectionHint)
	if normalized == "" {
		return nil
	}
	amounts := extractMoneyAmounts(line)
	if len(amounts) == 0 {
		return nil
	}
	out := []ExtractedFinancialFact{}
	for idx, amount := range amounts {
		if amount.Value <= 0 || amount.Value >= 1e15 {
			continue
		}
		source := DocumentFactSource{
			Page:        page,
			Line:        lineNumber,
			ColumnIndex: idx,
			Cells:       cells,
			Snippet:     truncateAssetSnippet(line, 700),
		}
		confidence := factConfidence(doc, len(cells) >= 2, "financial", line)
		fact := ExtractedFinancialFact{
			ID:                 stableFactID("financial", doc.SHA256, normalized, line, idx),
			Ticker:             strings.ToUpper(doc.Ticker),
			SourceFile:         doc.FilePath,
			SHA256:             doc.SHA256,
			DocumentDate:       documentDate,
			Period:             period,
			StatementType:      inferDocumentStatementType(normalized),
			LineItemOriginal:   lineItem,
			LineItemNormalized: normalized,
			Value:              amount.Value,
			Currency:           firstNonEmptyAsset(amount.Currency, inferCurrencyFromDocumentLine(line), context.Currency, "TRY"),
			Unit:               firstNonEmptyAsset(inferUnitFromDocumentLine(line), context.Unit, "unit"),
			ConsolidationScope: context.ConsolidationScope,
			AuditStatus:        context.AuditStatus,
			Source:             source,
			Confidence:         confidence,
			ReviewRequired:     confidence < 0.86,
			Warnings:           factWarnings(doc, confidence),
			CreatedAt:          createdAt,
		}
		if idx > 0 && !financialFactColumnPeriodResolved(line, cells, idx) {
			fact.Warnings = dedupeStrings(append(fact.Warnings, "period_column_unresolved"))
		}
		finalizeFinancialFactCertification(&fact)
		out = append(out, fact)
	}
	return out
}

func financialFactColumnPeriodResolved(line string, cells []string, amountIndex int) bool {
	if amountIndex <= 0 {
		return true
	}
	context := strings.Join(append([]string{line}, cells...), " ")
	return extractPeriodString(context) != nil || extractDateString(context) != nil
}

func peopleFromLine(doc RawDocument, page, lineNumber int, line string, cells []string, documentDate, period *string, createdAt string) []ExtractedPerson {
	line = normalizeLegacyPDFTurkish(line)
	cells = normalizeLegacyCells(cells)
	if !personLineLooksRelevant(line) {
		return nil
	}
	names := candidatePersonNames(personNameCandidateText(line))
	if len(names) == 0 {
		return nil
	}
	source := DocumentFactSource{Page: page, Line: lineNumber, Cells: cells, Snippet: truncateAssetSnippet(line, 700)}
	role := inferDocumentPersonRole(line)
	title := inferDocumentPersonTitle(line)
	confidence := factConfidence(doc, len(cells) >= 2, "management", line)
	out := []ExtractedPerson{}
	for _, name := range names {
		out = append(out, ExtractedPerson{
			ID:             stableFactID("person", doc.SHA256, name, line, 0),
			Ticker:         strings.ToUpper(doc.Ticker),
			SourceFile:     doc.FilePath,
			SHA256:         doc.SHA256,
			DocumentDate:   documentDate,
			Period:         period,
			FullName:       name,
			NormalizedName: strings.ToUpper(util.SlugTR(name)),
			Title:          title,
			Role:           role,
			Source:         source,
			Confidence:     confidence,
			ReviewRequired: confidence < 0.86,
			CreatedAt:      createdAt,
		})
	}
	return out
}

func ownershipFactsFromLine(doc RawDocument, page, lineNumber int, line string, cells []string, documentDate, period *string, createdAt string) []OwnershipFact {
	line = normalizeLegacyPDFTurkish(line)
	cells = normalizeLegacyCells(cells)
	if !ownershipLineLooksRelevant(line) {
		return nil
	}
	holder := ownershipHolderName(line, cells)
	if !ownershipHolderLooksValid(holder, line) {
		return nil
	}
	ratio := extractPercentValue(line)
	amount := largestMoneyAmount(line, "TRY")
	if ratio == nil && amount == nil {
		return nil
	}
	source := DocumentFactSource{Page: page, Line: lineNumber, Cells: cells, Snippet: truncateAssetSnippet(line, 700)}
	confidence := factConfidence(doc, len(cells) >= 2, "ownership", line)
	return []OwnershipFact{{
		ID:             stableFactID("ownership", doc.SHA256, holder, line, 0),
		Ticker:         strings.ToUpper(doc.Ticker),
		SourceFile:     doc.FilePath,
		SHA256:         doc.SHA256,
		DocumentDate:   documentDate,
		Period:         period,
		HolderName:     holder,
		ShareAmount:    amount,
		ShareRatio:     ratio,
		Source:         source,
		Confidence:     confidence,
		ReviewRequired: confidence < 0.86,
		CreatedAt:      createdAt,
	}}
}

func normalizeLegacyCells(cells []string) []string {
	if len(cells) == 0 {
		return cells
	}
	out := make([]string, len(cells))
	for i, cell := range cells {
		out[i] = normalizeLegacyPDFTurkish(cell)
	}
	return out
}

func corporateEventsFromLine(doc RawDocument, page, lineNumber int, line string, cells []string, documentDate, period *string, createdAt string) []ExtractedCorporateEvent {
	eventType := documentCorporateEventType(line)
	if eventType == "" || !corporateEventLineLooksActionable(line, eventType) {
		return nil
	}
	amount := largestMoneyAmount(line, "")
	ratio := extractPercentValue(line)
	effectiveDate := corporateActionDate(line)
	paymentDate := corporatePaymentDate(line, eventType)
	subscriptionPrice := corporateSubscriptionPrice(line)
	source := DocumentFactSource{Page: page, Line: lineNumber, Cells: cells, Snippet: truncateAssetSnippet(line, 700)}
	confidence := factConfidence(doc, len(cells) >= 2, "corporate_event", line)
	event := ExtractedCorporateEvent{
		ID:                stableFactID("corporate_event", doc.SHA256, eventType, line, 0),
		Ticker:            strings.ToUpper(doc.Ticker),
		SourceFile:        doc.FilePath,
		SHA256:            doc.SHA256,
		DocumentDate:      documentDate,
		Period:            period,
		EventType:         eventType,
		Title:             truncateAssetSnippet(line, 160),
		Amount:            amount,
		Ratio:             ratio,
		EffectiveDate:     effectiveDate,
		PaymentDate:       paymentDate,
		SubscriptionPrice: subscriptionPrice,
		Currency:          inferCurrencyFromDocumentLine(line),
		Source:            source,
		Confidence:        confidence,
		ReviewRequired:    confidence < 0.86,
		Warnings:          factWarnings(doc, confidence),
		CreatedAt:         createdAt,
	}
	finalizeCorporateEventCertification(&event)
	return []ExtractedCorporateEvent{event}
}

func corporateActionDate(line string) *string {
	return extractDateString(line)
}

func corporatePaymentDate(line, eventType string) *string {
	if eventType != "dividend" {
		return nil
	}
	return extractDateString(line)
}

func corporateSubscriptionPrice(line string) *float64 {
	slug := util.SlugTR(line)
	if !anySlugContains(slug, []string{"bedelli", "ruchan", "kullanim fiyati", "yeni pay alma"}) {
		return nil
	}
	amounts := extractMoneyAmounts(line)
	if len(amounts) == 0 {
		return nil
	}
	best := 0.0
	for _, amount := range amounts {
		if amount.Value <= 0 || amount.Value > 1000 {
			continue
		}
		if best == 0 || amount.Value < best {
			best = amount.Value
		}
	}
	if best == 0 {
		return nil
	}
	return floatPtr(best)
}

func newDocumentFact(doc RawDocument, group, kind, label, normalized, rawValue string, numeric *float64, currency, unit string, source DocumentFactSource, confidence float64, createdAt string) DocumentFact {
	documentDate := extractDateString(firstNonEmptyAsset(doc.FileName, source.Snippet))
	period := extractPeriodString(firstNonEmptyAsset(doc.FileName, source.Snippet))
	return DocumentFact{
		ID:                stableFactID("document_fact", doc.SHA256, group, normalized, source.Line),
		Ticker:            strings.ToUpper(doc.Ticker),
		SourceFile:        doc.FilePath,
		SHA256:            doc.SHA256,
		DocumentTypeGuess: doc.DocumentTypeGuess,
		DocumentDate:      documentDate,
		Period:            period,
		Group:             group,
		Kind:              kind,
		Label:             label,
		NormalizedKey:     normalized,
		RawValue:          rawValue,
		NumericValue:      numeric,
		Currency:          currency,
		Unit:              unit,
		Source:            source,
		Confidence:        confidence,
		ReviewRequired:    confidence < 0.86,
		Warnings:          factWarnings(doc, confidence),
		CreatedAt:         createdAt,
	}
}

func classifyDocumentLine(line string) (group, kind, label, normalized string) {
	eventType := documentCorporateEventType(line)
	switch {
	case financialLineLooksRelevant(line):
		item := financialLineItem(line, splitAssetCells(line))
		return "financial", "financial_statement_line", item, normalizeFinancialLineItem(item)
	case personLineLooksRelevant(line):
		return "management", "person_or_role", "Yönetim/organizasyon satırı", "management_person_or_role"
	case ownershipLineLooksRelevant(line) && ownershipHolderLooksValid(ownershipHolderName(line, splitAssetCells(line)), line):
		return "ownership", "ownership_or_capital", "Ortaklık/sermaye satırı", "ownership_or_capital"
	case eventType != "" && corporateEventLineLooksActionable(line, eventType):
		return "corporate_event", eventType, "Kurumsal olay satırı", eventType
	case companyInfoLineLooksRelevant(line):
		return "company_profile", "company_profile_line", "Şirket profil satırı", companyInfoKey(line)
	case riskLineLooksRelevant(line):
		return "risk", "risk_disclosure", "Risk/dipnot satırı", "risk_disclosure"
	case assetLineLooksRelevantForIndex(line):
		return "asset", "asset_or_portfolio_line", "Varlık/portföy satırı", "asset_or_portfolio_line"
	default:
		return "", "", "", ""
	}
}

func indexedDocumentLines(text string) []indexedTextLine {
	out := []indexedTextLine{}
	pages := strings.Split(text, "\f")
	for pageIndex, page := range pages {
		lineNo := 0
		for _, line := range strings.Split(page, "\n") {
			lineNo++
			clean := strings.TrimSpace(strings.ReplaceAll(line, "\t", "  "))
			if normalizeAssetLine(clean) == "" {
				continue
			}
			out = append(out, indexedTextLine{Page: pageIndex + 1, Line: lineNo, Text: clean})
		}
	}
	return out
}

func indexedLineKey(page, line int) string {
	return fmt.Sprintf("%d:%d", page, line)
}

func financialSectionsByIndexedLine(lines []indexedTextLine) map[string]string {
	out := map[string]string{}
	section := ""
	for _, line := range lines {
		if next := financialSectionFromLine(line.Text); next != "" {
			section = next
		}
		out[indexedLineKey(line.Page, line.Line)] = section
	}
	return out
}

func financialSectionFromLine(line string) string {
	clean := cleanFinancialLabel(line)
	slug := util.SlugTR(clean)
	switch slug {
	case "donenvarliklar", "toplamdonenvarliklar":
		return "current_assets"
	case "duranvarliklar", "toplamduranvarliklar":
		return "non_current_assets"
	case "kisavadeliyukumlulukler", "toplamkisavadeliyukumlulukler":
		return "current_liabilities"
	case "uzunvadeliyukumlulukler", "toplamuzunvadeliyukumlulukler":
		return "non_current_liabilities"
	case "ozkaynaklar", "anaortakligaaitozkaynaklar", "toplamozkaynaklar", "toplamozkaynakalar":
		return "equity"
	case "karveyazararkismi", "karveyazarartablosu", "karveyazararvedigerkapsamligelirtablosu":
		return "income_statement"
	case "digerkapsamligelir", "digerkapsamligelirler":
		return "income_statement"
	case "ozkaynaklardegisimtablosu":
		return "equity_statement"
	case "nakitakistablosu", "isletmefaaliyetlerindennakitakislari", "yatirimfaaliyetlerindenkaynaklanannakitakislari", "finansmanfaaliyetlerindenkaynaklanannakitakislari":
		return "cash_flow_statement"
	case "finansalaraclardankaynaklananrisklerinniteligiveduzeyi", "krediriskaciklamalari", "finansalaracturleriitibariylemaruzkalinankrediriskleri":
		return "credit_risk_table"
	case "likiditeriskineiliskinaciklamalar", "likiditeriskiaciklamalari":
		return "liquidity_risk_table"
	case "dovizpozisyonutablosu", "dovizpozisyonutablo", "dovizpozisyonutablosuveilgiliduyarlilikanalizi":
		return "fx_position_table"
	case "dovizkuruduyarlilikanalizitablosu":
		return "fx_sensitivity_table"
	case "faizpozisyonutablosu", "faizpozisyonutablo":
		return "interest_rate_position_table"
	case "portfoysinirlamalarinauyumunkontrolu", "ekdipnotportfoysinirlamalarinauyumunkontrolu":
		return "portfolio_limits_table"
	default:
		return ""
	}
}

func tableRowsFromIndexedLines(lines []indexedTextLine, sha string) []tableRowCandidate {
	out := []tableRowCandidate{}
	tableIndexByPage := map[int]int{}
	lastPage := -1
	rowIndex := 0
	for _, line := range lines {
		if lineTooLongForStructuredFact(line.Text) {
			lastPage = -1
			rowIndex = 0
			continue
		}
		cells := splitAssetCells(line.Text)
		if len(cells) < 2 {
			lastPage = -1
			rowIndex = 0
			continue
		}
		if line.Page != lastPage {
			tableIndexByPage[line.Page]++
			rowIndex = 0
			lastPage = line.Page
		}
		rowIndex++
		tableID := fmt.Sprintf("%s_p%d_t%d", shortHash(sha), line.Page, tableIndexByPage[line.Page])
		out = append(out, tableRowCandidate{
			TableID:  tableID,
			Page:     line.Page,
			Line:     line.Line,
			RowIndex: rowIndex,
			Cells:    cells,
			Raw:      line.Text,
		})
	}
	return out
}

func lineTooLongForStructuredFact(line string) bool {
	if len(line) <= maxStructuredFactLineRunes {
		return false
	}
	return len([]rune(line)) > maxStructuredFactLineRunes
}

func hasASCIIDigit(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= '0' && value[i] <= '9' {
			return true
		}
	}
	return false
}

func financialLineLooksRelevant(line string) bool {
	if lineTooLongForStructuredFact(line) {
		return false
	}
	if !hasASCIIDigit(line) {
		return false
	}
	if len(extractMoneyAmounts(line)) == 0 {
		return false
	}
	item := financialLineItem(line, splitAssetCells(line))
	if item == "" || financialLineLooksLikeNarrative(line, item) {
		return false
	}
	return financialLineItemMatchesCanonical(util.SlugTR(item))
}

func financialLineItemTerms() []string {
	terms := []string{
		"toplam varliklar", "varliklar", "donen varliklar", "duran varliklar", "nakit ve nakit benzerleri",
		"finansal yatirimlar", "ticari alacaklar", "stoklar", "yatirim amacli gayrimenkuller",
		"maddi duran varliklar", "maddi olmayan duran varliklar", "toplam kaynaklar", "kaynaklar", "yukumlulukler",
		"kisa vadeli yukumlulukler", "uzun vadeli yukumlulukler", "finansal borclar",
		"ticari borclar", "ozkaynaklar", "odenmis sermaye", "gecmis yillar kar", "donem kari",
		"net donem kari", "hasilat", "satis gelirleri", "brut kar", "faaliyet kari",
		"esas faaliyet kari", "finansman geliri", "finansman gideri", "vergi gideri",
		"nakit akislari", "isletme faaliyetlerinden", "yatirim faaliyetlerinden", "finansman faaliyetlerinden",
		"bankalar", "krediler", "mevduat", "prim uretimi", "teknik bolum", "net aktif deger",
	}
	terms = append(terms, kapfinance.CanonicalFinancialLineTerms()...)
	return dedupeStrings(terms)
}

var financialLineItemTermSlugs = buildFinancialLineItemTermSlugs()

func buildFinancialLineItemTermSlugs() []string {
	terms := financialLineItemTerms()
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		out = append(out, util.SlugTR(term))
	}
	return out
}

func financialLineItemMatchesCanonical(slug string) bool {
	if slug == "" || financialLabelSlugLooksBlocked(slug) {
		return false
	}
	words := strings.Fields(slug)
	if len(words) == 0 || len(words) > 9 {
		return false
	}
	for _, termSlug := range financialLineItemTermSlugs {
		if slug == termSlug {
			return true
		}
		if slugContains(slug, termSlug) && financialTermMayAppearInsideLabel(termSlug, slug) {
			return true
		}
	}
	return false
}

func financialTermMayAppearInsideLabel(termSlug, labelSlug string) bool {
	if termSlug == "" || labelSlug == "" {
		return false
	}
	words := strings.Fields(labelSlug)
	if len(words) > 7 {
		return false
	}
	switch termSlug {
	case "varliklar":
		return labelSlug == "varliklar" || labelSlug == "toplam varliklar"
	case "kaynaklar":
		return labelSlug == "kaynaklar" || labelSlug == "toplam kaynaklar"
	case "yatirim faaliyetlerinden":
		return slugContains(labelSlug, "yatirim faaliyetlerinden nakit")
	case "yatirim":
		return false
	default:
		return true
	}
}

func financialLineLooksLikeNarrative(line, item string) bool {
	lineSlug := util.SlugTR(line)
	itemSlug := util.SlugTR(item)
	lower := strings.ToLower(line)
	if financialLabelSlugLooksBlocked(lineSlug) || financialLabelSlugLooksBlocked(itemSlug) {
		return true
	}
	if financialLineLooksLikeNoteIndex(line, item) {
		return true
	}
	if len(strings.Fields(itemSlug)) > 9 || len([]rune(item)) > 95 {
		return true
	}
	if slugContains(lineSlug, "tarihi itibariyla") && (slugContains(lineSlug, "adet pay") || slugContains(lineSlug, "birim pay") || slugContains(lineSlug, "kaynaklarindan")) {
		return true
	}
	if anySlugContains(lineSlug, []string{"donem karindan", "ortaklara kar payi", "faiz orani", "tahakkuk eden faiz", "her biri 1 kr", "her biri bir kurus", "sermayesi sirasiyla"}) {
		return true
	}
	if strings.Contains(lower, "dönem kar") && (strings.Contains(lower, "kâr pay") || strings.Contains(lower, "kar pay")) && strings.Contains(lower, "dağıt") {
		return true
	}
	if strings.Count(line, ".") >= 2 && len(strings.Fields(line)) > 18 {
		return true
	}
	return false
}

func financialLineLooksLikeNoteIndex(line, item string) bool {
	line = strings.TrimSpace(line)
	if line == "" || util.SlugTR(item) == "" {
		return false
	}
	lineSlug := util.SlugTR(line)
	if !(strings.Contains(lineSlug, "tms") || strings.Contains(lineSlug, "tfrs") || strings.Contains(lineSlug, "yorum")) {
		return false
	}
	if amount := largestMoneyAmount(line, ""); amount != nil && *amount > 1000 {
		return false
	}
	return noteIndexLineRE.MatchString(line)
}

func financialLabelSlugLooksBlocked(slug string) bool {
	for _, blocked := range []string{
		"adet pay", "birim pay", "pay geri alim", "geri alim programi", "kaynaklarindan",
		"ic kaynak", "alimlarin toplami", "pay sahibi", "genel kurul", "vekaletname",
		"teblig", "esas sozlesme", "tapu", "kadastro", "rapor tarihi", "degerleme yontemi",
		"emsal", "kira ekspertiz", "kdv haric", "kdv dahil", "sigorta degeri", "imza",
		"sayfa", "madde", "dipnot", "bagimsiz denetci gorusu",
		"faaliyet raporunda", "iken", "cevaben", "hissedarimizin sorusuna",
		"donem karindan", "ortaklara kar payi", "kar payi olarak", "faiz orani",
		"tahakkuk eden faiz", "her biri 1 kr", "her biri bir kurus", "sermayesi sirasiyla",
		"finanse edildigi", "fonlardan saglanan", "kaynaklardan saglanan",
	} {
		if slugContains(slug, blocked) {
			return true
		}
	}
	return false
}

func financialLineItem(line string, cells []string) string {
	for _, cell := range cells {
		if len(extractMoneyAmounts(cell)) > 0 || isMostlyNumeric(cell) {
			continue
		}
		clean := cleanFinancialLabel(cell)
		if clean != "" && financialLabelLooksUseful(clean) {
			return clean
		}
	}
	clean := cleanFinancialLabel(line)
	if financialLabelLooksUseful(clean) {
		return clean
	}
	return ""
}

func cleanFinancialLabel(value string) string {
	value = normalizeLegacyPDFTurkish(value)
	value = normalizeAssetLine(value)
	value = linePrefixRE.ReplaceAllString(value, "")
	value = amountRE.ReplaceAllString(value, "")
	value = percentRE.ReplaceAllString(value, "")
	value = strings.Trim(value, " :-")
	if len([]rune(value)) > 140 {
		value = truncateAssetSnippet(value, 140)
	}
	return value
}

func financialLabelLooksUseful(value string) bool {
	if value == "" || len([]rune(value)) < 4 || isMostlyNumeric(value) {
		return false
	}
	slug := util.SlugTR(value)
	if slug == "tl" || slug == "try" || slug == "usd" || slug == "eur" || financialLabelSlugLooksBlocked(slug) {
		return false
	}
	return financialLineItemMatchesCanonical(slug)
}

func normalizeFinancialLineItem(value string) string {
	return normalizeFinancialLineItemInSection(value, "")
}

func normalizeFinancialLineItemInSection(value, sectionHint string) string {
	if def, ok := kapfinance.CanonicalLineDefinitionForTextInContext(value, sectionHint); ok && def.Normalized != "" {
		return def.Normalized
	}
	slug := util.SlugTR(value)
	for _, pair := range []struct {
		key   string
		value string
	}{
		{"nakit ve nakit benzerleri", "cash_and_cash_equivalents"},
		{"donen varliklar", "current_assets"},
		{"duran varliklar", "non_current_assets"},
		{"varliklar", "total_assets"},
		{"kisa vadeli yukumlulukler", "current_liabilities"},
		{"uzun vadeli yukumlulukler", "non_current_liabilities"},
		{"yukumlulukler", "total_liabilities"},
		{"ozkaynaklar", "equity"},
		{"odenmis sermaye", "paid_in_capital"},
		{"yatirim amacli gayrimenkuller", "investment_properties"},
		{"hasilat", "revenue"},
		{"satis gelirleri", "revenue"},
		{"brut kar", "gross_profit"},
		{"faaliyet kari", "operating_profit"},
		{"net donem kari", "net_income"},
		{"donem kari", "net_income"},
		{"finansman gideri", "finance_expense"},
		{"finansman geliri", "finance_income"},
		{"nakit akislari", "cash_flow"},
		{"net aktif deger", "net_asset_value"},
		{"finansal borclar", "financial_debt"},
		{"ticari alacaklar", "trade_receivables"},
		{"ticari borclar", "trade_payables"},
		{"stoklar", "inventories"},
		{"bankalar", "banks"},
		{"krediler", "loans"},
		{"mevduat", "deposits"},
	} {
		if slugContains(slug, pair.key) {
			return pair.value
		}
	}
	if slug == "" {
		return ""
	}
	return slug
}

func inferDocumentStatementType(normalized string) string {
	if statementType := kapfinance.StatementTypeForNormalizedLine(normalized); statementType != "" {
		return statementType
	}
	switch normalized {
	case "revenue", "gross_profit", "operating_profit", "net_income", "finance_expense", "finance_income":
		return "income_statement"
	case "cash_flow":
		return "cash_flow_statement"
	case "paid_in_capital":
		return "equity_statement"
	default:
		return "balance_sheet"
	}
}

func inferCurrencyFromDocumentLine(line string) string {
	if currency := detectCurrency(line); currency != "" {
		return currency
	}
	slug := util.SlugTR(line)
	switch {
	case slugContains(slug, "usd") || slugContains(slug, "abd dolari"):
		return "USD"
	case slugContains(slug, "eur") || slugContains(slug, "euro"):
		return "EUR"
	default:
		return ""
	}
}

func inferUnitFromDocumentLine(line string) string {
	slug := util.SlugTR(line)
	switch {
	case slugContains(slug, "bin tl") || slugContains(slug, "bin turk lirasi"):
		return "thousand_try"
	case slugContains(slug, "milyon tl") || slugContains(slug, "milyon turk lirasi"):
		return "million_try"
	case slugContains(slug, "milyon usd") || slugContains(slug, "milyon abd dolari"):
		return "million_usd"
	case slugContains(slug, "bin usd") || slugContains(slug, "bin abd dolari"):
		return "thousand_usd"
	default:
		return ""
	}
}

func newDocumentFinancialContext(doc RawDocument) documentFinancialContext {
	context := financialDocumentContextText(doc)
	slug := util.SlugTR(context)
	return documentFinancialContext{
		Currency:           inferDocumentReportingCurrencyFromContext(context, slug),
		Unit:               inferDocumentReportingUnitFromSlug(slug),
		ConsolidationScope: inferDocumentConsolidationScopeFromSlug(slug),
		AuditStatus:        inferDocumentAuditStatusFromSlug(slug),
	}
}

func inferDocumentReportingCurrency(doc RawDocument) string {
	context := financialDocumentContextText(doc)
	return inferDocumentReportingCurrencyFromContext(context, util.SlugTR(context))
}

func inferDocumentReportingCurrencyFromContext(context, slug string) string {
	if currency := detectCurrency(context); currency != "" {
		return currency
	}
	switch {
	case slugContains(slug, "turk lirasi") || slugContains(slug, "tl"):
		return "TRY"
	case slugContains(slug, "abd dolari") || slugContains(slug, "amerikan dolari"):
		return "USD"
	case slugContains(slug, "euro"):
		return "EUR"
	default:
		return ""
	}
}

func inferDocumentReportingUnit(doc RawDocument) string {
	return inferDocumentReportingUnitFromSlug(util.SlugTR(financialDocumentContextText(doc)))
}

func inferDocumentReportingUnitFromSlug(slug string) string {
	switch {
	case slugContains(slug, "aksi belirtilmedikce bin turk lirasi") || slugContains(slug, "bin turk lirasi") || slugContains(slug, "bin tl") || slugContains(slug, "thousand turkish lira"):
		return "thousand_try"
	case slugContains(slug, "milyon turk lirasi") || slugContains(slug, "milyon tl") || slugContains(slug, "million turkish lira"):
		return "million_try"
	case slugContains(slug, "bin abd dolari") || slugContains(slug, "bin usd") || slugContains(slug, "thousand us dollar"):
		return "thousand_usd"
	case slugContains(slug, "milyon abd dolari") || slugContains(slug, "milyon usd") || slugContains(slug, "million us dollar"):
		return "million_usd"
	default:
		return ""
	}
}

func inferDocumentConsolidationScope(doc RawDocument) string {
	return inferDocumentConsolidationScopeFromSlug(util.SlugTR(financialDocumentContextText(doc)))
}

func inferDocumentConsolidationScopeFromSlug(slug string) string {
	switch {
	case slugContains(slug, "konsolide olmayan") || slugContains(slug, "solo finansal") || slugContains(slug, "bireysel finansal") || slugContains(slug, "konsolidasyona tabi olmayan"):
		return "standalone"
	case slugContains(slug, "konsolide finansal") || slugContains(slug, "konsolide"):
		return "consolidated"
	default:
		return ""
	}
}

func inferDocumentAuditStatus(doc RawDocument) string {
	return inferDocumentAuditStatusFromSlug(util.SlugTR(financialDocumentContextText(doc)))
}

func inferDocumentAuditStatusFromSlug(slug string) string {
	switch {
	case slugContains(slug, "bagimsiz denetimden gecmemis") || slugContains(slug, "denetimden gecmemis") || slugContains(slug, "denetimden gecmemistir"):
		return "unaudited"
	case slugContains(slug, "sinirli denetimden gecmis") || slugContains(slug, "sinirli denetim") || slugContains(slug, "sinirli inceleme"):
		return "limited_review"
	case slugContains(slug, "bagimsiz denetimden gecmis") || slugContains(slug, "denetimden gecmis") || slugContains(slug, "bagimsiz denetci raporu") || slugContains(slug, "bagimsiz denetim raporu"):
		return "audited"
	default:
		return ""
	}
}

func financialDocumentContextText(doc RawDocument) string {
	text := doc.FileName + "\n" + doc.Text
	const maxContextRunes = 12000
	count := 0
	for idx := range text {
		if count == maxContextRunes {
			return text[:idx]
		}
		count++
	}
	return text
}

func personLineLooksRelevant(line string) bool {
	slug := util.SlugTR(line)
	for _, blocked := range []string{
		"genel mudurluk", "bolge mudurluk", "mudurluk olmak uzere", "ticaret sicil mudurlugu",
		"yonetim kurulu uyeleri hakkinda bilgiler", "uyeleri hakkinda bilgiler",
		"yonetim kurulu ve gorev suresi", "gorev suresi", "baslangic tarihi", "bitis tarihi",
		"bafllang", "bitifl", "madde",
	} {
		if slugContains(slug, blocked) {
			return false
		}
	}
	for _, term := range []string{"yonetim kurulu", "genel mudur", "icra kurulu", "denetim komitesi", "kurumsal yonetim komitesi", "riskin erken", "bagimsiz uye", "ust yonetim"} {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func candidatePersonNames(line string) []string {
	words := strings.Fields(line)
	out := []string{}
	window := []string{}
	for _, word := range words {
		hardSeparator := personNameTokenEndsGroup(word)
		clean := cleanPersonNameWord(word)
		if !nameWordLooksLikePersonToken(clean) {
			if len(window) >= 2 {
				candidate := strings.Join(window, " ")
				if personNameLooksValid(candidate) {
					out = append(out, candidate)
				}
			}
			window = nil
			continue
		}
		window = append(window, clean)
		if len(window) > 4 {
			window = window[1:]
		}
		if hardSeparator {
			if len(window) >= 2 {
				candidate := strings.Join(window, " ")
				if personNameLooksValid(candidate) {
					out = append(out, candidate)
				}
			}
			window = nil
		}
	}
	if len(window) >= 2 {
		candidate := strings.Join(window, " ")
		if personNameLooksValid(candidate) {
			out = append(out, candidate)
		}
	}
	return dedupeStrings(out)
}

func personNameTokenEndsGroup(word string) bool {
	word = strings.TrimSpace(word)
	return strings.HasSuffix(word, ",") || strings.HasSuffix(word, ";") || strings.HasSuffix(word, ":")
}

func cleanPersonNameWord(word string) string {
	clean := normalizeLegacyPDFTurkish(strings.TrimSpace(word))
	clean = strings.Trim(clean, ".,;:()[]{}\"“”")
	if idx := strings.IndexAny(clean, "’'`´"); idx > 0 {
		suffix := util.SlugTR(clean[idx+1:])
		switch suffix {
		case "", "a", "e", "ya", "ye", "yi", "yu", "yı", "yü", "da", "de", "dan", "den", "in", "un", "nın", "nin", "nun", "nün", "na", "ne", "nunu", "dir":
			clean = clean[:idx]
		}
	}
	return strings.Trim(clean, ".,;:()[]{}'’`´\"“”")
}

func personNameCandidateText(line string) string {
	clean := normalizeLegacyPDFTurkish(line)
	replacements := []string{
		"Sayın", "Sayin", "Sn.", "Sn",
		"Yönetim Kurulu Başkanı", "Yonetim Kurulu Baskani",
		"Yönetim Kurulu Başkan Yardımcısı", "Yonetim Kurulu Baskan Yardimcisi",
		"Yönetim Kurulu Üyesi", "Yonetim Kurulu Uyesi",
		"Bağımsız Yönetim Kurulu Üyesi", "Bagimsiz Yonetim Kurulu Uyesi",
		"Bağımsız Yönetim Kurulu Üyeliği", "Bagimsiz Yonetim Kurulu Uyeligi",
		"Genel Müdür", "Genel Mudur",
		"İcra Kurulu Başkanı", "Icra Kurulu Baskani",
		"Denetim Komitesi", "Kurumsal Yönetim Komitesi", "Riskin Erken Saptanması Komitesi",
		"Genel Kurul", "Aday Gösterme", "Aday Gosterme",
		"Başkan", "Baskan", "Üye", "Uye", "Üyeliği", "Uyeligi",
		"Evet", "Hayır", "Hayir", "İngilizce", "Ingilizce",
		"Yatırımcı İlişkileri Bölümünün", "Yatirimci Iliskileri Bolumunun",
		"Başkan Vekili", "Baskan Vekili", "Vekili",
		"Bölümünün", "Bolumunun", "Yöneticisi", "Yoneticisi", "Yönetici", "Yonetici",
		"Kimlik No", "T.C.", "TC", "İmza", "Imza",
	}
	for _, item := range replacements {
		clean = strings.ReplaceAll(clean, item, " ")
	}
	return normalizeAssetLine(clean)
}

func normalizeLegacyPDFTurkish(value string) string {
	return legacyPDFTurkishReplacer.Replace(value)
}

func nameWordLooksLikePersonToken(value string) bool {
	value = normalizeLegacyPDFTurkish(value)
	if len([]rune(value)) < 2 || strings.ContainsAny(value, "0123456789/%") {
		return false
	}
	if personNameTokenBlocked(value) {
		return false
	}
	runes := []rune(value)
	return unicode.IsUpper(runes[0])
}

func personNameLooksValid(value string) bool {
	value = strings.TrimSpace(value)
	slug := util.SlugTR(normalizeLegacyPDFTurkish(value))
	for _, blocked := range []string{
		"yonetim kurulu", "genel mudur", "kurumsal yonetim", "denetim komitesi", "riskin erken",
		"faaliyet raporu", "alarko gayrimenkul", "anonim sirket", "genel kurul", "baslangic tarihi",
		"bitis tarihi", "denetimden sorumlu", "tapu kadastro", "web portali", "turk ticaret",
		"ticaret kanunu", "aday gosterme", "ic ticaret", "ticaret bakanligi", "olusturulan komitelerin",
		"pay sahibi", "bagimsiz yonetim", "komite nin", "komitenin", "sorumlu komite",
		"sermaye piyasasi", "esas sozlesme", "yatirim ortakligi", "ayrica denetimden",
		"hakkinda bilgiler", "hakkinda denetci", "degerleme ve", "ust yonetimden",
		"sirketin idaresi", "uyelerinin ucreti", "gorev bolumu", "ust duzey yoneticilere",
		"uyeleri hakkinda bilgiler", "hesap uzmani", "sirketler toplulugu", "kanalizasyon idaresi",
		"bakanlik temsilcisine", "isci bulma kurumu", "azil ve degistirme gerekceleri",
		"finansal tablolara iliskin sorumluluklari", "carrier sanayi", "artibir gayrimenkul degerleme",
		"yoneticilerin yetki", "tasinmaz degerleme", "say yapi",
		"kimlik no", "insaat muhendisi", "durum tablosu", "basvuru sistemi", "baskani imza",
		"azli degistirilmesi veya secimi", "istanbul su", "olanlarin konsolide",
		"idari sorumlulugu", "yer bagimsizligini finans alaninda", "sirkete borclanma yasagi",
		"toplantilarinin sekli", "kar dagitim politikasi", "birinci sutunda", "beyaninin yer bagimsizligini",
		"turkiye cumhuriyeti uyruklu", "tescil edilen hususlar", "seri iv no", "mosalarko ojsc",
		"kote ortaklik yoneticileri dernegi", "yatirimci iliskileri", "gestalt kocluk",
		"sirketimizin turk", "yazmani oy toplama memuru", "vakifbank international",
		"temile yetkili", "sozlesme tadil metni", "the board",
		"ili beyoğlu ilcesi", "ili beyoglu ilcesi", "islem yapma sirkete",
		"turkiye cumhuriyeti", "finans alaninda", "koteder borsaya", "kotoder borsaya",
		"tofas tat gida petkim", "milli emlak", "capital markets", "general assembly",
		"abdi ibrahim ilac", "fenerbahce futbol", "fenerbahce sportif",
		"kapsamli gelir", "finans faktoring", "hava tasimaciligi", "pay sahipleri",
		"iliskiler birimi", "turk lirasi", "yillik olagan toplantisindan", "diger", "olarak",
	} {
		if slugContains(slug, blocked) {
			return false
		}
	}
	for _, token := range []string{"kurulu", "baskani", "başkanı", "mudur", "müdür", "komitesi", "komite", "raporu", "faaliyet", "portal", "kanunu", "baskanligi", "uyeligi", "hakkinda", "bilgiler", "denetci", "degerleme", "gorev", "suresi", "muhendisi", "tablosu", "sistemi", "imza", "kimlik", "ilac", "holding", "anonim", "sirket", "sportif", "futbol"} {
		if slugContains(slug, token) {
			return false
		}
	}
	parts := strings.Fields(value)
	if len(parts) < 2 || len(parts) > 4 || len([]rune(value)) > 70 {
		return false
	}
	if personNameHasTooMuchOCRNoise(value) {
		return false
	}
	if personNameLooksLikeLocation(parts) {
		return false
	}
	for _, part := range parts {
		if personNameTokenBlocked(part) {
			return false
		}
		runes := []rune(strings.Trim(part, ".,;:()[]{}'’"))
		if len(runes) < 2 || len(runes) > 24 || !unicode.IsUpper(runes[0]) {
			return false
		}
	}
	return true
}

func personNameTokenBlocked(value string) bool {
	token := util.SlugTR(normalizeLegacyPDFTurkish(strings.Trim(value, ".,;:()[]{}'’")))
	if token == "" {
		return true
	}
	for _, blocked := range []string{
		"yonetim", "kurulu", "kurul", "genel", "mudur", "mudurluk", "icra", "denetim",
		"komite", "komitesi", "kurumsal", "riskin", "erken", "saptanmasi", "bagimsiz",
		"uye", "uyesi", "uyeligi", "baskan", "baskani", "baskanligi", "yardimcisi",
		"faaliyet", "raporu", "rapor", "tarih", "baslangic", "bitis", "sorumlu",
		"aday", "gosterme", "ticaret", "kanunu", "bakanligi", "web", "portali",
		"tapu", "kadastro", "sirket", "anonim", "alarko", "gayrimenkul", "kap",
		"spk", "mkk", "egks", "tfrs", "tl", "try", "usd", "eur", "adet",
		"olusturulan", "komitelerin", "sayi", "yapi",
		"sermaye", "piyasasi", "esas", "sozlesmesi", "yatirim", "ortakligi", "ayrica",
		"hakkinda", "bilgiler", "denetci", "degerleme", "ve", "ile", "ust", "yonetimden",
		"idare", "islerinin", "sirketin", "idaresi", "uyelerinin", "ucreti", "gorev",
		"bolumu", "duzey", "yoneticilere", "hesap", "uzmani", "sirketler", "toplulugu",
		"kanalizasyon", "idaresi", "iski", "bakanlik", "temsilcisine", "isci", "bulma",
		"kurumu", "yardimciligina", "dalaman", "seka", "muessesinde", "azil",
		"degistirme", "gerekceleri", "finansal", "tablolara", "iliskin", "sorumluluklari",
		"carrier", "sanayi", "artibir", "yoneticilerin", "yetki", "tasinmaz", "tc",
		"gumruk", "say", "sn", "bafllang", "bitifl", "tarihi", "ingilizce", "evet", "hayir",
		"kimlik", "no", "insaat", "muhendisi", "durum", "tablosu", "basvuru",
		"sistemi", "imza", "azli", "secimi", "degistirilmesi", "istanbul", "su",
		"olanlarin", "konsolide", "idari", "sorumlulugu", "bagimsizligini",
		"borclanma", "yasagi", "toplantilarinin", "sekli", "birinci", "sutunda",
		"beyaninin", "uyruklu", "tescil", "edilen", "hususlar", "seri", "mosalarko",
		"ojsc", "kote", "dernegi", "yatirimci", "iliskileri", "gestalt", "kocluk",
		"temsil", "yetkili", "sozlesme", "tadil", "metni", "board",
		"ili", "ilcesi", "evliyacelebi", "sokak", "islem", "yapma", "sirkete",
		"turkiye", "cumhuriyeti", "finans", "alaninda", "koteder", "kotoder", "borsaya",
		"tofas", "tat", "gida", "petkim", "muzesi", "odtu", "arkeoloji", "emlak",
		"capital", "markets", "general", "assembly", "kapsamli", "gelir", "faktoring",
		"tasimaciligi", "mensuplari", "pay", "sahipleri", "birimi", "lirasi",
		"yillik", "olagan", "toplantisindan", "ilac", "holding", "as", "aş",
		"sportif", "futbol", "hizmetler", "kulubu", "kulübü",
	} {
		if token == blocked {
			return true
		}
	}
	return false
}

func personNameHasTooMuchOCRNoise(value string) bool {
	if strings.ContainsAny(value, "¤§{}<>›‹ﬁﬂ") {
		return true
	}
	letters := 0
	noisy := 0
	for _, r := range value {
		if unicode.IsLetter(r) {
			letters++
			continue
		}
		if unicode.IsSpace(r) || r == '\'' || r == '’' || r == '-' || r == '.' {
			continue
		}
		noisy++
	}
	return letters > 0 && noisy*5 > letters
}

func personNameLooksLikeLocation(parts []string) bool {
	if len(parts) < 2 {
		return false
	}
	locationTokens := 0
	for _, part := range parts {
		switch util.SlugTR(part) {
		case "adana", "ankara", "istanbul", "izmir", "eyup", "topcular", "bodrum", "mugla", "antalya", "eskisehir", "bolge":
			locationTokens++
		}
	}
	return locationTokens == len(parts) || locationTokens >= 2
}

func inferDocumentPersonRole(line string) string {
	slug := util.SlugTR(line)
	switch {
	case slugContains(slug, "yonetim kurulu"):
		return "board_of_directors"
	case slugContains(slug, "genel mudur") || slugContains(slug, "icra"):
		return "executive_management"
	case slugContains(slug, "komite"):
		return "committee"
	default:
		return "management"
	}
}

func inferDocumentPersonTitle(line string) string {
	slug := util.SlugTR(line)
	switch {
	case slugContains(slug, "baskan yardimcisi"):
		return "Başkan Yardımcısı"
	case slugContains(slug, "baskan"):
		return "Başkan"
	case slugContains(slug, "genel mudur"):
		return "Genel Müdür"
	case slugContains(slug, "uye"):
		return "Üye"
	default:
		return ""
	}
}

func ownershipHolderName(line string, cells []string) string {
	for _, cell := range cells {
		if len(extractMoneyAmounts(cell)) > 0 || extractPercentValue(cell) != nil {
			continue
		}
		clean := cleanFinancialLabel(cell)
		if clean != "" && !assetNameLooksGeneric(clean) && hasLetters(clean) {
			return clean
		}
	}
	clean := cleanFinancialLabel(line)
	if clean == "" {
		return ""
	}
	return truncateAssetSnippet(clean, 120)
}

func ownershipLineLooksRelevant(line string) bool {
	if !hasASCIIDigit(line) {
		return false
	}
	slug := util.SlugTR(line)
	if ownershipLineLooksLikeNonOwnership(line) {
		return false
	}
	ratio := extractPercentValue(line)
	amount := largestMoneyAmount(line, "TRY")
	for _, blocked := range []string{
		"pay sahibi disindaki", "ozel talimat", "vekaletname", "genel kurul", "oy haklari",
		"imtiyazli paylara iliskin", "paylarin devri", "pay alim satim bildirimi yoktur",
	} {
		if slugContains(slug, blocked) {
			return false
		}
	}
	hasOwnershipTerm := false
	for _, term := range []string{"ortaklik yapisi", "sermaye payi", "pay orani", "pay sahibi", "hissedar", "halka acik kisim"} {
		if slugContains(slug, term) {
			hasOwnershipTerm = true
			break
		}
	}
	if !hasOwnershipTerm {
		cells := splitAssetCells(line)
		holder := ownershipHolderName(line, cells)
		return len(cells) >= 2 && len(cells) <= 6 && ratio != nil && *ratio >= 0.5 && ownershipHolderLooksLikeCorporateOrCategory(holder)
	}
	return ratio != nil || amount != nil || len(extractMoneyAmounts(line)) > 0
}

func ownershipHolderLooksValid(holder, line string) bool {
	holder = strings.TrimSpace(holder)
	if holder == "" || len([]rune(holder)) > 120 {
		return false
	}
	if strings.ContainsAny(holder, "{}§<>›‹ﬁﬂ") {
		return false
	}
	slug := util.SlugTR(holder)
	for _, blocked := range []string{
		"ortaklar", "ortaklik yapisi", "pay sahibi", "pay orani", "pay tutari", "sermaye payi",
		"sermaye", "hissedar", "imtiyaz", "oy haklari", "ozel talimat", "pay sahibi disindaki",
		"toplam", "genel kurul", "kar payi", "vekaletname", "esas sozlesme",
		"varliklar", "gayrimenkuller", "yukumlulukler", "ozkaynaklar", "hasilat", "stoklar",
		"finansal yatirimlar", "ticari alacaklar", "maddi duran",
		"likit fon", "yatirim fonu", "tahvil", "mevduat", "repo", "bono", "vadeli",
	} {
		if slug == util.SlugTR(blocked) || slugContains(slug, blocked) {
			return false
		}
	}
	if !hasLetters(holder) || isMostlyNumeric(holder) {
		return false
	}
	if ownershipLineLooksLikeNonOwnership(line) {
		return false
	}
	if ownershipHolderLooksLikeCorporateOrCategory(holder) {
		return true
	}
	if !ownershipLineHasExplicitTerm(line) {
		return false
	}
	return personNameLooksValid(holder)
}

func ownershipHolderLooksLikeCorporateOrCategory(holder string) bool {
	holder = strings.TrimSpace(holder)
	slug := util.SlugTR(holder)
	if slug == "diger" || slugContains(slug, "halka acik kisim") || slugContains(slug, "dolasimdaki pay") {
		return true
	}
	for _, suffix := range []string{"anonim", "holding", "vakfi", "ltd", "limited", "sirketi"} {
		if strings.Contains(slug, suffix) {
			return true
		}
	}
	if strings.HasSuffix(slug, "as") && len([]rune(slug)) > 4 {
		return true
	}
	return false
}

func ownershipLineHasExplicitTerm(line string) bool {
	slug := util.SlugTR(line)
	for _, term := range []string{"ortaklik yapisi", "sermaye payi", "pay orani", "pay sahibi", "hissedar", "halka acik kisim"} {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func ownershipLineLooksLikeNonOwnership(line string) bool {
	slug := util.SlugTR(line)
	for _, blocked := range []string{
		"birim kira", "birim satis", "satis degeri", "satisa arz", "pazarlik payi",
		"degerleme", "ekspertiz", "gayrimenkul", "dukkan", "arsa", "otel", "fabrika",
		"kredi", "borc ozkaynak", "teminatli", "teminatsiz", "banka kredisi",
		"abd dolari", "piyasa kosullari", "konfor kosullari", "orta iyi", "orta kotu",
		"cok iyi", "kira degeri", "m2", "tl m", "katli", "kat", "kardan ayrilan kisitlanmis yedek",
		"likit fon", "yatirim fonu", "tahvil", "bono", "repo", "mevduat", "vadeli mevduat",
		"faiz oran", "vade tarihi", "vadeye kalan", "finansal varlik", "para ve sermaye piyasasi",
		"portfoy deger tablosu", "ashmore",
	} {
		if slugContains(slug, blocked) {
			return true
		}
	}
	return false
}

func extractPercentValue(line string) *float64 {
	matches := percentRE.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		var raw string
		for i := 1; i < len(match); i++ {
			if strings.TrimSpace(match[i]) != "" {
				raw = match[i]
				break
			}
		}
		if raw == "" {
			continue
		}
		value, ok := parseTurkishNumber(raw)
		if ok && value >= 0 && value <= 100 {
			return floatPtr(value)
		}
	}
	return nil
}

func documentCorporateEventType(line string) string {
	slug := util.SlugTR(line)
	switch {
	case slugContains(slug, "kar payi") || slugContains(slug, "temettu"):
		return "dividend"
	case slugContains(slug, "sermaye artir"):
		return "capital_increase"
	case slugContains(slug, "bedelli") || slugContains(slug, "bedelsiz"):
		return "capital_action"
	case slugContains(slug, "genel kurul"):
		return "general_assembly"
	case slugContains(slug, "pay geri alim"):
		return "share_buyback"
	case slugContains(slug, "birlesme"):
		return "merger"
	case slugContains(slug, "bolunme"):
		return "spin_off"
	case slugContains(slug, "varlik satis") || slugContains(slug, "tasinmaz satis") || slugContains(slug, "gayrimenkul satis"):
		return "asset_sale"
	case slugContains(slug, "yatirim") && (slugContains(slug, "tesvik") || slugContains(slug, "proje")):
		return "investment_project"
	default:
		return ""
	}
}

func corporateEventLineLooksActionable(line, eventType string) bool {
	slug := util.SlugTR(line)
	if corporateEventLineLooksGeneric(slug) {
		return false
	}
	switch eventType {
	case "general_assembly":
		if anySlugContains(slug, []string{
			"genel kurul oncesi", "genel kurul toplantilarinin gundemleri", "genel kurul toplantisina elektronik ortamda katilim",
			"genel kurul toplantisinda ortaya cikabilecek diger konular", "gundeminde yer alan hususlar hakkinda",
			"toplanti tutanaginin imzalanmasi", "toplanti baskanligina yetki", "gundem maddesinin karsisinda",
			"talimat bildirim formu", "asgari 3 hafta", "gundemiyle her gundem", "gorevler genel kurul",
			"genel kurul toplantisi tarihi ile sinirli", "genel kurul gundeminde", "genel kurul a okunmus",
			"suresi dolmus olsa bile", "adi veya fevkalade genel kurul", "yonetim kurulu tarafindan toplantiya cagrilabilir",
			"ortaklar genel kurul toplantisi", "genel kurulun toplantiya cagrilmasi", "yonetim kurulu ve denetci raporlariyla",
			"elektronik ortamda katil", "belirten genel kurul gundem maddesine", "gundemiyle dogrudan",
			"genel kurul toplantisinda kabul edilen ozel denetci talebi", "genel kurul toplantisinda pay sahiplerinin onayina sunulacaktir",
			"roportaj ve basin aciklamalari", "konusu islemler genel kurulda",
		}) {
			return false
		}
		return anySlugContains(slug, []string{"olagan genel kurul toplantisi", "genel kurul toplantisi", "toplanti tutanagi", "toplantiya cagri", "gundem", "hazirun", "toplanti tarihi"})
	case "dividend":
		if anySlugContains(slug, []string{"temettu geliri", "kar payi geliri", "kar payi politikas", "kar dagitim politikas", "temettuler ilgili ozkaynak", "kar payi avansi dusulmus"}) {
			return false
		}
		return anySlugContains(slug, []string{"kar payi dagitim tablosu", "kar payi dagitilmasina", "kar payi dagitimi", "temettu dagitimi", "nakit kar payi", "brut", "net", "odeme tarihi", "dagitilacak"})
	case "capital_increase", "capital_action":
		if anySlugContains(slug, []string{
			"anonim ortakliklarin sermaye artirimi", "hisse basina kazanc hesaplanirken", "bedelsiz hisse ihraci cikarilmis",
			"pay alimi veya sermaye artirimi sebebiyle olusan nakit cikislari", "bedelsiz hisse dagitimlari pay basina kazanc",
			"istirakler ve veya is ortakliklari pay alimi", "pay sahipleri genel olarak", "karari ile sermaye artirimi yapamaz",
			"istirakler veveya is ortakliklari pay alimi", "ortakliklarin sermaye artirimi dolayisiyla",
			"yapilacak sermaye artiriminda ihrac olunacak paylar", "hamiline olmasi esastir", "yonetim kurulu cesitli",
			"sayilir sermaye artirimi yapamaz",
		}) {
			return false
		}
		return anySlugContains(slug, []string{"sermaye artirimi", "bedelli sermaye", "bedelsiz sermaye", "ruchan hakki", "spk basvurusu", "ihraç", "ihrac", "fon kullanim"})
	case "share_buyback":
		return anySlugContains(slug, []string{"pay geri alim programi", "geri alim programi", "geri alima konu", "geri alinacak", "geri alinan pay", "azami fon"})
	case "merger":
		if anySlugContains(slug, []string{"isletme birlesmelerinin etkisi", "birlesmelerin etkisi", "yeniden degerleme deger artis fonundan"}) {
			return false
		}
		return anySlugContains(slug, []string{"birlesme islemi", "devralma yoluyla birlesme", "kolaylastirilmis usulde birlesme"})
	case "spin_off":
		if anySlugContains(slug, []string{"bolunmesiyle bulunur", "bolunmesi suretiyle hesaplanir"}) {
			return false
		}
		return anySlugContains(slug, []string{"kismi bolunme", "bolunme islemi", "bolunme yoluyla", "devredilmesine"})
	case "asset_sale":
		if anySlugContains(slug, []string{
			"gayrimenkule iliskin olarak yapilmis sozlesmelere", "gayrimenkul satis vaadi", "satis kari", "satis zarari",
			"satis gelirleri", "satis amaciyla elde tutulan", "gayrimenkul satisindan elde edilen gelirler",
			"yatirim amacli gayrimenkul satisi", "tasinmaz satisiyla ilgili duzenleme",
		}) && !anySlugContains(slug, []string{"satisina karar", "satilmasina karar"}) {
			return false
		}
		return anySlugContains(slug, []string{"tasinmaz satis", "gayrimenkul satis", "varlik satis", "satisina karar", "satilmasina karar"})
	case "investment_project":
		if anySlugContains(slug, []string{"alinip alinmadigi hakkinda bilgi", "portfoyune alinmadigi", "sermaye piyasasi araclarina yatirim yapmak", "belirli projeleri", "yatirim ortakligi portfoyu"}) {
			return false
		}
		if !anySlugContains(slug, []string{"yatirim projesi", "tesvik belgesi", "kapasite artisi", "yeni yatirim", "proje yatirimi"}) {
			return false
		}
		return len(extractMoneyAmounts(line)) > 0 || anySlugContains(slug, []string{"tesvik belgesi", "kapasite artisi", "baslanmasina", "tamamlanmasina"})
	default:
		return false
	}
}

func corporateEventLineLooksGeneric(slug string) bool {
	for _, blocked := range []string{
		"muhasebe politikasi", "muhasebe tahmin", "dipnot", "tfrs", "finansal tablo dipnot",
		"genel kurulun onayina sunar", "genel kurul tarafindan secilir", "yonetim kurulunun yillik faaliyet raporu",
		"esas sozlesme hukumleri", "turk ticaret kanunu", "sermaye piyasasi mevzuati",
		"sermaye piyasasi kurulu nun", "teblig", "yonetmelik", "mevzuat kapsaminda",
		"maddi duran varlik satisi", "sermaye artirimi sebebiyle olusan nakit cikislari",
		"istiraklar ve veya is ortakliklari pay alimi", "istirakler ve veya is ortakliklari pay alimi",
		"istiraklar veveya is ortakliklari pay alimi", "istirakler veveya is ortakliklari pay alimi",
		"yatirim amacli gayrimenkul satis kari",
		"ortaklara ortaklara dagitilan kar payinin", "dagitilan kar payinin bagislar eklenmis",
		"varlik satislari veya ayni sermaye katkilari", "bu degisiklik ile bir yatirimci ile istirak",
	} {
		if slugContains(slug, blocked) {
			return true
		}
	}
	return false
}

func anySlugContains(slug string, terms []string) bool {
	for _, term := range terms {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func companyInfoLineLooksRelevant(line string) bool {
	slug := util.SlugTR(line)
	if emailRE.MatchString(line) || phoneRE.MatchString(line) || websiteRE.MatchString(line) {
		return true
	}
	for _, term := range []string{"mersis", "ticaret sicil", "vergi dairesi", "vergi no", "merkez adres", "sirket merkezi", "odenmis sermaye", "kayitli sermaye"} {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func companyInfoKey(line string) string {
	slug := util.SlugTR(line)
	switch {
	case emailRE.MatchString(line):
		return "email"
	case phoneRE.MatchString(line):
		return "phone"
	case websiteRE.MatchString(line):
		return "website"
	case slugContains(slug, "mersis"):
		return "mersis_number"
	case slugContains(slug, "ticaret sicil"):
		return "trade_registry_number"
	case slugContains(slug, "vergi"):
		return "tax_information"
	case slugContains(slug, "sermaye"):
		return "capital"
	default:
		return "company_profile"
	}
}

func riskLineLooksRelevant(line string) bool {
	slug := util.SlugTR(line)
	for _, term := range []string{"risk", "belirsizlik", "dava", "hukuki", "ipotek", "rehin", "teminat", "kur riski", "likidite riski", "faiz riski"} {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func assetLineLooksRelevantForIndex(line string) bool {
	slug := util.SlugTR(line)
	for _, term := range assetCueKeywords {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func factConfidence(doc RawDocument, tableLike bool, group string, line string) float64 {
	score := 0.48
	if tableLike {
		score += 0.16
	}
	if doc.QualityScore >= 0.75 {
		score += 0.10
	} else if doc.QualityScore < RejectedTextQualityThreshold {
		score -= 0.18
	}
	if strings.TrimSpace(doc.SHA256) != "" {
		score += 0.05
	}
	if len(extractMoneyAmounts(line)) > 0 {
		score += 0.08
	}
	if group == "financial" && financialLineItem(line, splitAssetCells(line)) != "" {
		score += 0.08
	}
	if group == "management" && len(candidatePersonNames(line)) > 0 {
		score += 0.08
	}
	return clampAsset(score, 0.05, 0.95)
}

func factWarnings(doc RawDocument, confidence float64) []string {
	warnings := []string{}
	analysisUsable := doc.AnalysisUsable
	if doc.QualityGate.Status == "" {
		analysisUsable = doc.QualityScore >= TrustedTextQualityThreshold
	}
	if !analysisUsable || containsRawWarning(doc.Warnings, "low_text_quality_possible_scanned_pdf") {
		warnings = append(warnings, "low_text_quality_possible_scanned_pdf")
	}
	if confidence < 0.70 {
		warnings = append(warnings, "needs_human_review")
	}
	return warnings
}

func finalizeFinancialFactCertification(fact *ExtractedFinancialFact) {
	fact.Warnings = removeCertificationReasonWarnings(fact.Warnings)
	cert := certifyFinancialFact(*fact)
	fact.Certification = cert
	if !evidenceCertificationAnalysisUsable(cert) {
		fact.ReviewRequired = true
		fact.Warnings = dedupeStrings(append(fact.Warnings, cert.Reasons...))
	} else {
		fact.ReviewRequired = false
	}
}

func finalizeFinancialTableCertification(table *ExtractedFinancialTable) {
	table.Warnings = removeCertificationReasonWarnings(table.Warnings)
	cert := certifyFinancialTable(*table)
	table.Certification = cert
	if !evidenceCertificationAnalysisUsable(cert) {
		table.ReviewRequired = true
		table.Warnings = dedupeStrings(append(table.Warnings, cert.Reasons...))
	} else {
		table.ReviewRequired = false
	}
}

func finalizeCorporateEventCertification(event *ExtractedCorporateEvent) {
	event.Warnings = removeCertificationReasonWarnings(event.Warnings)
	cert := certifyCorporateEvent(*event)
	event.Certification = cert
	if !evidenceCertificationAnalysisUsable(cert) {
		event.ReviewRequired = true
		event.Warnings = dedupeStrings(append(event.Warnings, cert.Reasons...))
	} else {
		event.ReviewRequired = false
	}
}

func removeCertificationReasonWarnings(warnings []string) []string {
	out := warnings[:0]
	for _, warning := range warnings {
		if isCertificationReason(warning) {
			continue
		}
		out = append(out, warning)
	}
	return out
}

func evidenceCertificationAnalysisUsable(cert EvidenceCertification) bool {
	return cert.AnalysisUsable && (cert.Status == EvidenceStatusCertified || cert.Status == EvidenceStatusAIResolved)
}

func isCertificationReason(value string) bool {
	switch strings.TrimSpace(value) {
	case "source_file_missing",
		"sha256_missing",
		"source_snippet_missing",
		"source_locator_missing",
		"source_table_id_missing",
		"metric_identity_missing",
		"statement_type_missing",
		"table_identity_missing",
		"table_rows_missing",
		"period_missing",
		"document_date_missing",
		"currency_missing",
		"unit_not_explicit",
		"consolidation_scope_missing",
		"audit_status_missing",
		"event_type_missing",
		"event_title_missing",
		"corporate_action_effective_date_missing",
		"corporate_action_ratio_missing",
		"rights_subscription_price_missing",
		"dividend_amount_missing",
		"confidence_below_certification_threshold",
		"value_out_of_bounds",
		"period_column_unresolved":
		return true
	default:
		return false
	}
}

func certifyFinancialFact(fact ExtractedFinancialFact) EvidenceCertification {
	cert := EvidenceCertification{
		Status:                EvidenceStatusCertified,
		Score:                 100,
		AnalysisUsable:        true,
		EvidenceComplete:      true,
		NormalizationComplete: true,
	}
	deduct := func(points int, reason string, evidence, normalization bool) {
		cert.Score -= points
		cert.Reasons = append(cert.Reasons, reason)
		if evidence {
			cert.EvidenceComplete = false
		}
		if normalization {
			cert.NormalizationComplete = false
		}
	}
	if strings.TrimSpace(fact.SourceFile) == "" {
		deduct(25, "source_file_missing", true, false)
	}
	if strings.TrimSpace(fact.SHA256) == "" {
		deduct(20, "sha256_missing", true, false)
	}
	if strings.TrimSpace(fact.Source.Snippet) == "" {
		deduct(20, "source_snippet_missing", true, false)
	}
	if fact.Source.Page == 0 && fact.Source.Line == 0 && strings.TrimSpace(fact.Source.TableID) == "" {
		deduct(10, "source_locator_missing", true, false)
	}
	if strings.TrimSpace(fact.ID) == "" || strings.TrimSpace(fact.LineItemNormalized) == "" {
		deduct(20, "metric_identity_missing", false, true)
	}
	if strings.TrimSpace(fact.StatementType) == "" {
		deduct(10, "statement_type_missing", false, true)
	}
	if fact.Period == nil || strings.TrimSpace(*fact.Period) == "" {
		deduct(15, "period_missing", false, true)
	}
	if containsRawWarning(fact.Warnings, "period_column_unresolved") ||
		(fact.Source.ColumnIndex > 0 && !financialFactColumnPeriodResolved(fact.Source.Snippet, fact.Source.Cells, fact.Source.ColumnIndex)) {
		deduct(25, "period_column_unresolved", false, true)
	}
	if strings.TrimSpace(fact.Currency) == "" {
		deduct(15, "currency_missing", false, true)
	}
	if strings.TrimSpace(fact.Unit) == "" || fact.Unit == "unit" {
		deduct(10, "unit_not_explicit", false, true)
	}
	if fact.Value <= 0 || fact.Value >= 1e15 {
		deduct(20, "value_out_of_bounds", false, true)
	}
	if fact.Confidence < 0.86 {
		deduct(10, "confidence_below_certification_threshold", false, false)
	}
	if strings.TrimSpace(fact.Source.TableID) == "" {
		deduct(5, "source_table_id_missing", true, false)
	}
	if fact.DocumentDate == nil || strings.TrimSpace(*fact.DocumentDate) == "" {
		deduct(5, "document_date_missing", false, true)
	}
	if strings.TrimSpace(fact.ConsolidationScope) == "" {
		deduct(5, "consolidation_scope_missing", false, true)
	}
	if strings.TrimSpace(fact.AuditStatus) == "" {
		deduct(5, "audit_status_missing", false, true)
	}
	return finalizeEvidenceCertification(cert)
}

func certifyCorporateEvent(event ExtractedCorporateEvent) EvidenceCertification {
	cert := EvidenceCertification{
		Status:                EvidenceStatusCertified,
		Score:                 100,
		AnalysisUsable:        true,
		EvidenceComplete:      true,
		NormalizationComplete: true,
	}
	deduct := func(points int, reason string, evidence, normalization bool) {
		cert.Score -= points
		cert.Reasons = append(cert.Reasons, reason)
		if evidence {
			cert.EvidenceComplete = false
		}
		if normalization {
			cert.NormalizationComplete = false
		}
	}
	if strings.TrimSpace(event.SourceFile) == "" {
		deduct(25, "source_file_missing", true, false)
	}
	if strings.TrimSpace(event.SHA256) == "" {
		deduct(20, "sha256_missing", true, false)
	}
	if strings.TrimSpace(event.Source.Snippet) == "" {
		deduct(20, "source_snippet_missing", true, false)
	}
	if event.Source.Page == 0 && event.Source.Line == 0 && strings.TrimSpace(event.Source.TableID) == "" {
		deduct(10, "source_locator_missing", true, false)
	}
	if strings.TrimSpace(event.EventType) == "" {
		deduct(20, "event_type_missing", false, true)
	}
	if strings.TrimSpace(event.Title) == "" {
		deduct(20, "event_title_missing", false, true)
	}
	if event.Confidence < 0.86 {
		deduct(10, "confidence_below_certification_threshold", false, false)
	}
	switch event.EventType {
	case "capital_increase", "capital_action":
		if event.EffectiveDate == nil || strings.TrimSpace(*event.EffectiveDate) == "" {
			deduct(20, "corporate_action_effective_date_missing", false, true)
		}
		if event.Ratio == nil || *event.Ratio <= 0 {
			deduct(25, "corporate_action_ratio_missing", false, true)
		}
		if corporateEventRequiresSubscriptionPrice(event) && (event.SubscriptionPrice == nil || *event.SubscriptionPrice <= 0) {
			deduct(10, "rights_subscription_price_missing", false, true)
		}
	case "dividend":
		if event.Amount == nil || *event.Amount <= 0 {
			deduct(20, "dividend_amount_missing", false, true)
		}
		if (event.PaymentDate == nil || strings.TrimSpace(*event.PaymentDate) == "") && (event.EffectiveDate == nil || strings.TrimSpace(*event.EffectiveDate) == "") {
			deduct(15, "corporate_action_effective_date_missing", false, true)
		}
	}
	return finalizeEvidenceCertification(cert)
}

func corporateEventRequiresSubscriptionPrice(event ExtractedCorporateEvent) bool {
	slug := util.SlugTR(event.Title + " " + event.Source.Snippet)
	return anySlugContains(slug, []string{"bedelli", "ruchan", "yeni pay alma", "kullanim fiyati"})
}

func certifyFinancialTable(table ExtractedFinancialTable) EvidenceCertification {
	cert := EvidenceCertification{
		Status:                EvidenceStatusCertified,
		Score:                 100,
		AnalysisUsable:        true,
		EvidenceComplete:      true,
		NormalizationComplete: true,
	}
	deduct := func(points int, reason string, evidence, normalization bool) {
		cert.Score -= points
		cert.Reasons = append(cert.Reasons, reason)
		if evidence {
			cert.EvidenceComplete = false
		}
		if normalization {
			cert.NormalizationComplete = false
		}
	}
	if strings.TrimSpace(table.SourceFile) == "" {
		deduct(25, "source_file_missing", true, false)
	}
	if strings.TrimSpace(table.SHA256) == "" {
		deduct(20, "sha256_missing", true, false)
	}
	if strings.TrimSpace(table.Source.Snippet) == "" {
		deduct(20, "source_snippet_missing", true, false)
	}
	if table.Source.Page == 0 && table.Source.Line == 0 && strings.TrimSpace(table.Source.TableID) == "" {
		deduct(10, "source_locator_missing", true, false)
	}
	if strings.TrimSpace(table.ID) == "" || strings.TrimSpace(table.TableType) == "" {
		deduct(20, "table_identity_missing", false, true)
	}
	if len(table.Rows) == 0 {
		deduct(20, "table_rows_missing", true, true)
	}
	if table.Period == nil || strings.TrimSpace(*table.Period) == "" {
		deduct(15, "period_missing", false, true)
	}
	if table.Confidence < 0.86 {
		deduct(10, "confidence_below_certification_threshold", false, false)
	}
	if table.DocumentDate == nil || strings.TrimSpace(*table.DocumentDate) == "" {
		deduct(5, "document_date_missing", false, true)
	}
	if strings.TrimSpace(table.Currency) == "" {
		deduct(10, "currency_missing", false, true)
	}
	if strings.TrimSpace(table.Unit) == "" || table.Unit == "unit" {
		deduct(10, "unit_not_explicit", false, true)
	}
	if strings.TrimSpace(table.ConsolidationScope) == "" {
		deduct(5, "consolidation_scope_missing", false, true)
	}
	if strings.TrimSpace(table.AuditStatus) == "" {
		deduct(5, "audit_status_missing", false, true)
	}
	return finalizeEvidenceCertification(cert)
}

func finalizeEvidenceCertification(cert EvidenceCertification) EvidenceCertification {
	if cert.Score < 0 {
		cert.Score = 0
	}
	cert.Reasons = dedupeStrings(cert.Reasons)
	switch {
	case !cert.EvidenceComplete:
		cert.Status = EvidenceStatusRejected
		cert.AnalysisUsable = false
		cert.RequiresHumanReview = true
	case cert.Score < 100:
		cert.Status = EvidenceStatusReviewRequired
		cert.AnalysisUsable = false
		cert.RequiresHumanReview = true
	default:
		cert.Status = EvidenceStatusCertified
		cert.AnalysisUsable = true
	}
	if cert.Status != EvidenceStatusCertified {
		cert.RequiredForCertification = missingCertificationRequirements(cert.Reasons)
	}
	return cert
}

func missingCertificationRequirements(reasons []string) []string {
	required := []string{}
	for _, reason := range reasons {
		switch reason {
		case "source_file_missing", "sha256_missing", "source_snippet_missing", "source_locator_missing", "source_table_id_missing":
			required = append(required, "source document, sha256, page/line/table and snippet evidence")
		case "metric_identity_missing", "statement_type_missing", "table_identity_missing", "table_rows_missing":
			required = append(required, "normalized statement/table identity")
		case "event_type_missing", "event_title_missing":
			required = append(required, "normalized corporate event identity")
		case "period_missing", "document_date_missing":
			required = append(required, "period and document date normalization")
		case "corporate_action_effective_date_missing":
			required = append(required, "corporate action effective/payment date")
		case "corporate_action_ratio_missing":
			required = append(required, "capital increase or split ratio")
		case "rights_subscription_price_missing":
			required = append(required, "rights issue subscription price")
		case "dividend_amount_missing":
			required = append(required, "dividend amount")
		case "currency_missing", "unit_not_explicit":
			required = append(required, "explicit currency and unit normalization")
		case "consolidation_scope_missing":
			required = append(required, "consolidated_or_standalone scope")
		case "audit_status_missing":
			required = append(required, "audited_or_unaudited status")
		case "confidence_below_certification_threshold":
			required = append(required, "confidence >= 0.86 or human approval")
		}
	}
	return dedupeStrings(required)
}

func documentIDForRawDocument(doc RawDocument) string {
	if doc.SHA256 != "" {
		return "rawdoc_" + shortHash(doc.SHA256)
	}
	return stableFactID("rawdoc", doc.FilePath, doc.FileName, 0)
}

func stableFactID(parts ...any) string {
	h := sha1.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(fmt.Sprint(part)))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:20]
}

func shortHash(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 12 {
		return value[:12]
	}
	if value == "" {
		return stableFactID(time.Now().UnixNano())[:12]
	}
	return value
}

func documentIntelligenceGlobalPaths(outputDir string) map[string]string {
	return map[string]string{
		DocumentFactsFile:   filepath.Join(outputDir, DocumentFactsFile),
		FinancialFactsFile:  filepath.Join(outputDir, FinancialFactsFile),
		FinancialTablesFile: filepath.Join(outputDir, FinancialTablesFile),
		PeopleFile:          filepath.Join(outputDir, PeopleFile),
		OwnershipFactsFile:  filepath.Join(outputDir, OwnershipFactsFile),
		CorporateFactsFile:  filepath.Join(outputDir, CorporateFactsFile),
	}
}

func globalDocumentIntelligenceOutputFiles(outputDir string) []string {
	files := []string{}
	for _, name := range []string{DocumentFactsFile, FinancialFactsFile, FinancialTablesFile, PeopleFile, OwnershipFactsFile, CorporateFactsFile} {
		files = append(files, filepath.Join(outputDir, name))
	}
	return files
}

func openDocumentIntelligenceEncoders(paths map[string]string) (map[string]*json.Encoder, []*os.File, error) {
	encoders := map[string]*json.Encoder{}
	files := []*os.File{}
	for _, name := range []string{DocumentFactsFile, FinancialFactsFile, FinancialTablesFile, PeopleFile, OwnershipFactsFile, CorporateFactsFile} {
		file, err := os.OpenFile(paths[name], os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			closeDocumentIntelligenceFiles(files)
			return nil, nil, err
		}
		files = append(files, file)
		encoders[name] = json.NewEncoder(file)
	}
	return encoders, files, nil
}

func closeDocumentIntelligenceFiles(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

func buildDocumentIndex(ticker string, extracts []documentIntelligenceExtraction, generatedAt string, sortRows bool) (DocumentIndex, []DocumentFact, []ExtractedFinancialFact, []ExtractedFinancialTable, []ExtractedPerson, []OwnershipFact, []ExtractedCorporateEvent) {
	index := DocumentIndex{Ticker: ticker, GeneratedAt: generatedAt}
	facts := []DocumentFact{}
	financials := []ExtractedFinancialFact{}
	tables := []ExtractedFinancialTable{}
	people := []ExtractedPerson{}
	ownership := []OwnershipFact{}
	corporate := []ExtractedCorporateEvent{}
	groupCounts := map[string]int{}
	for _, extracted := range extracts {
		index.Documents = append(index.Documents, extracted.IndexDoc)
		facts = append(facts, extracted.Facts...)
		financials = append(financials, extracted.FinancialFacts...)
		tables = append(tables, extracted.FinancialTables...)
		people = append(people, extracted.People...)
		ownership = append(ownership, extracted.OwnershipFacts...)
		corporate = append(corporate, extracted.CorporateEvents...)
		if extracted.IndexDoc.AnalysisUsable {
			index.Counts.AnalysisUsableDocs++
		}
		if extracted.IndexDoc.AIResolved {
			index.Counts.AIResolvedDocs++
		}
		if extracted.IndexDoc.HumanReviewRequired {
			index.Counts.ReviewRequiredDocs++
		}
		if extracted.IndexDoc.ParseStatus == ParseStatusRejected {
			index.Counts.RejectedDocs++
		}
		if extracted.IndexDoc.ParseStatus != ParseStatusTrusted {
			index.Counts.LowQualityDocs++
		}
		for _, fact := range extracted.Facts {
			groupCounts[fact.Group]++
			if fact.ReviewRequired {
				index.Counts.ReviewRequired++
			}
		}
		for _, fact := range extracted.FinancialFacts {
			if fact.ReviewRequired {
				index.Counts.ReviewRequired++
			}
		}
		for _, table := range extracted.FinancialTables {
			if table.ReviewRequired {
				index.Counts.ReviewRequired++
			}
		}
	}
	if sortRows {
		sort.SliceStable(index.Documents, func(i, j int) bool {
			if index.Documents[i].Period != nil && index.Documents[j].Period != nil && *index.Documents[i].Period != *index.Documents[j].Period {
				return *index.Documents[i].Period > *index.Documents[j].Period
			}
			return index.Documents[i].FilePath < index.Documents[j].FilePath
		})
		sortDocumentFacts(facts)
		sortFinancialFacts(financials)
		sortFinancialTables(tables)
		sortPeopleFacts(people)
		sortOwnershipFacts(ownership)
		sortCorporateFacts(corporate)
	}
	groups := make([]string, 0, len(groupCounts))
	for group := range groupCounts {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	for _, group := range groups {
		index.Groups = append(index.Groups, DocumentGroupIndex{Group: group, Count: groupCounts[group]})
	}
	index.Counts.Documents = len(index.Documents)
	index.Counts.DocumentFacts = len(facts)
	index.Counts.FinancialFacts = len(financials)
	index.Counts.FinancialTables = len(tables)
	index.Counts.People = len(people)
	index.Counts.OwnershipFacts = len(ownership)
	index.Counts.CorporateEvents = len(corporate)
	if index.Counts.DocumentFacts == 0 {
		index.Warnings = append(index.Warnings, "document_facts_empty")
	}
	if len(extracts) > 0 {
		index.Sector = extracts[0].IndexDoc.Sector
	}
	return index, facts, financials, tables, people, ownership, corporate
}

func buildCompanyKnowledgeGraph(index DocumentIndex, facts []DocumentFact, financials []ExtractedFinancialFact, tables []ExtractedFinancialTable, people []ExtractedPerson, ownership []OwnershipFact, corporate []ExtractedCorporateEvent, generatedAt string) CompanyKnowledgeGraph {
	resolvedContradictions := ReconcileFinancialValueDifferences(financials)
	graph := CompanyKnowledgeGraph{
		Ticker:                 index.Ticker,
		GeneratedAt:            generatedAt,
		Sector:                 index.Sector,
		Counts:                 index.Counts,
		SourceFiles:            companyKnowledgeGraphSourceFiles(),
		Warnings:               append([]string{}, index.Warnings...),
		ResolvedContradictions: resolvedContradictions,
		DuplicateMergeSummary: KnowledgeDuplicateMergeSummary{
			Documents:          len(index.Documents),
			DistinctDocuments:  distinctDocumentHashCount(index.Documents),
			DuplicateDocuments: len(index.Documents) - distinctDocumentHashCount(index.Documents),
			FactRows:           len(facts) + len(financials) + len(tables) + len(people) + len(ownership) + len(corporate),
		},
	}
	nodes := map[string]KnowledgeGraphNode{}
	edges := map[string]KnowledgeGraphEdge{}
	addNode := func(node KnowledgeGraphNode) {
		if strings.TrimSpace(node.ID) == "" {
			return
		}
		existing, ok := nodes[node.ID]
		if !ok {
			nodes[node.ID] = node
			return
		}
		if existing.Attributes == nil {
			existing.Attributes = map[string]any{}
		}
		for key, value := range node.Attributes {
			existing.Attributes[key] = value
		}
		existing.Evidence = appendLimitedEvidence(existing.Evidence, node.Evidence...)
		nodes[node.ID] = existing
	}
	addEdge := func(edge KnowledgeGraphEdge) {
		if strings.TrimSpace(edge.From) == "" || strings.TrimSpace(edge.To) == "" || strings.TrimSpace(edge.Type) == "" {
			return
		}
		if edge.ID == "" {
			edge.ID = "edge:" + stableFactID(edge.From, edge.To, edge.Type)
		}
		existing, ok := edges[edge.ID]
		if !ok {
			if edge.Weight == 0 {
				edge.Weight = 1
			}
			edges[edge.ID] = edge
			return
		}
		if edge.Weight == 0 {
			edge.Weight = 1
		}
		existing.Weight += edge.Weight
		if existing.Attributes == nil {
			existing.Attributes = map[string]any{}
		}
		for key, value := range edge.Attributes {
			existing.Attributes[key] = value
		}
		existing.Evidence = appendLimitedEvidence(existing.Evidence, edge.Evidence...)
		edges[edge.ID] = existing
	}

	companyID := knowledgeNodeID("company", index.Ticker)
	addNode(KnowledgeGraphNode{
		ID:    companyID,
		Type:  "company",
		Label: index.Ticker,
		Attributes: map[string]any{
			"ticker": index.Ticker,
			"title":  index.Sector.Title,
		},
	})
	if index.Sector.Sector != "" {
		sectorID := knowledgeNodeID("sector", index.Sector.Sector)
		addNode(KnowledgeGraphNode{
			ID:    sectorID,
			Type:  "sector",
			Label: index.Sector.Sector,
			Attributes: map[string]any{
				"main_sector": index.Sector.MainSector,
				"source":      index.Sector.Source,
				"fallback":    index.Sector.Fallback,
			},
		})
		addEdge(KnowledgeGraphEdge{From: companyID, To: sectorID, Type: "classified_in_sector"})
	}

	docNodeBySHA := map[string]string{}
	docTypeCounts := map[string]int{}
	businessModelCounts := map[string]int{}
	for _, doc := range index.Documents {
		docID := knowledgeNodeID("document", firstNonEmptyAsset(doc.DocumentID, doc.SHA256, doc.FilePath))
		docNodeBySHA[doc.SHA256] = docID
		docType := NormalizeDocumentType(doc.DocumentTypeGuess)
		docTypeCounts[docType]++
		addNode(KnowledgeGraphNode{
			ID:    docID,
			Type:  "document",
			Label: doc.FileName,
			Attributes: map[string]any{
				"file_path":             doc.FilePath,
				"sha256":                doc.SHA256,
				"document_type":         docType,
				"period":                doc.Period,
				"document_date":         doc.DocumentDate,
				"quality_score":         doc.QualityScore,
				"parse_status":          doc.ParseStatus,
				"analysis_usable":       doc.AnalysisUsable,
				"human_review_required": doc.HumanReviewRequired,
				"ai_resolved":           doc.AIResolved,
				"ai_confidence":         doc.AIConfidence,
				"fact_count":            doc.FactCount,
				"financial_fact_count":  doc.FinancialFactCount,
				"financial_table_count": doc.FinancialTableCount,
				"people_count":          doc.PeopleCount,
				"ownership_fact_count":  doc.OwnershipFactCount,
				"corporate_event_count": doc.CorporateEventCount,
				"warnings":              doc.Warnings,
			},
			Evidence: []KnowledgeEvidence{knowledgeEvidenceFromIndexedDocument(doc)},
		})
		addEdge(KnowledgeGraphEdge{From: companyID, To: docID, Type: "has_document", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromIndexedDocument(doc)}})
		docTypeID := knowledgeNodeID("document_type", docType)
		addNode(KnowledgeGraphNode{ID: docTypeID, Type: "document_type", Label: docType})
		addEdge(KnowledgeGraphEdge{From: docID, To: docTypeID, Type: "has_document_type"})
		for _, model := range doc.BusinessModels {
			if strings.TrimSpace(model.Tag) == "" {
				continue
			}
			businessModelCounts[model.Tag]++
			modelID := knowledgeNodeID("business_model", model.Tag)
			addNode(KnowledgeGraphNode{
				ID:    modelID,
				Type:  "business_model",
				Label: model.Tag,
				Attributes: map[string]any{
					"confidence":      model.Confidence,
					"review_required": model.ReviewRequired,
				},
				Evidence: []KnowledgeEvidence{{SourceFile: doc.FilePath, SHA256: doc.SHA256, Period: doc.Period, Snippet: strings.Join(model.Evidence, " | "), Confidence: model.Confidence}},
			})
			addEdge(KnowledgeGraphEdge{From: companyID, To: modelID, Type: "has_business_model", Evidence: []KnowledgeEvidence{{SourceFile: doc.FilePath, SHA256: doc.SHA256, Period: doc.Period, Snippet: strings.Join(model.Evidence, " | "), Confidence: model.Confidence}}})
			addEdge(KnowledgeGraphEdge{From: docID, To: modelID, Type: "supports_business_model"})
			if model.ReviewRequired {
				graph.Warnings = append(graph.Warnings, "business_model_review_required:"+model.Tag)
			}
		}
		if doc.ExtractionRoute.HumanReviewRequired {
			graph.Warnings = append(graph.Warnings, "extraction_route_human_review_required:"+doc.FileName)
		}
	}
	for docType, count := range docTypeCounts {
		docTypeID := knowledgeNodeID("document_type", docType)
		addNode(KnowledgeGraphNode{ID: docTypeID, Type: "document_type", Label: docType, Attributes: map[string]any{"document_count": count}})
		addEdge(KnowledgeGraphEdge{From: companyID, To: docTypeID, Type: "has_document_type", Weight: count})
	}
	for model, count := range businessModelCounts {
		modelID := knowledgeNodeID("business_model", model)
		addNode(KnowledgeGraphNode{ID: modelID, Type: "business_model", Label: model, Attributes: map[string]any{"document_count": count}})
	}
	for _, group := range index.Groups {
		groupID := knowledgeNodeID("fact_group", group.Group)
		addNode(KnowledgeGraphNode{ID: groupID, Type: "fact_group", Label: group.Group, Attributes: map[string]any{"fact_count": group.Count}})
		addEdge(KnowledgeGraphEdge{From: companyID, To: groupID, Type: "has_fact_group", Weight: group.Count})
	}

	distinctFactKeys := map[string]bool{}
	for _, fact := range facts {
		distinctFactKeys[knowledgeFactKey(fact)] = true
		groupID := knowledgeNodeID("fact_group", fact.Group)
		docID := docNodeBySHA[fact.SHA256]
		if docID != "" {
			addEdge(KnowledgeGraphEdge{From: docID, To: groupID, Type: "contains_fact_group", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromDocumentFact(fact)}})
		}
	}
	for _, fact := range financials {
		distinctFactKeys[knowledgeFinancialFactKey(fact)] = true
		label := firstNonEmptyAsset(fact.LineItemNormalized, fact.LineItemOriginal)
		if label == "" {
			continue
		}
		itemID := knowledgeNodeID("financial_line_item", label)
		addNode(KnowledgeGraphNode{
			ID:    itemID,
			Type:  "financial_line_item",
			Label: label,
			Attributes: map[string]any{
				"statement_type": fact.StatementType,
				"currency":       fact.Currency,
				"unit":           fact.Unit,
			},
			Evidence: []KnowledgeEvidence{knowledgeEvidenceFromFinancialFact(fact)},
		})
		docID := docNodeBySHA[fact.SHA256]
		if docID != "" {
			addEdge(KnowledgeGraphEdge{From: docID, To: itemID, Type: "reports_financial_line_item", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromFinancialFact(fact)}})
		}
		addEdge(KnowledgeGraphEdge{From: companyID, To: itemID, Type: "has_financial_line_item", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromFinancialFact(fact)}})
	}
	for _, table := range tables {
		distinctFactKeys[knowledgeFinancialTableKey(table)] = true
		tableID := knowledgeNodeID("financial_table", table.TableType)
		addNode(KnowledgeGraphNode{ID: tableID, Type: "financial_table", Label: table.TableType, Attributes: map[string]any{"table_type": table.TableType}, Evidence: []KnowledgeEvidence{knowledgeEvidenceFromFinancialTable(table)}})
		docID := docNodeBySHA[table.SHA256]
		if docID != "" {
			addEdge(KnowledgeGraphEdge{From: docID, To: tableID, Type: "contains_financial_table", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromFinancialTable(table)}})
		}
		addEdge(KnowledgeGraphEdge{From: companyID, To: tableID, Type: "has_financial_table", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromFinancialTable(table)}})
	}
	for _, person := range people {
		distinctFactKeys[knowledgePersonKey(person)] = true
		name := firstNonEmptyAsset(person.NormalizedName, person.FullName)
		if name == "" {
			continue
		}
		personID := knowledgeNodeID("person", name)
		addNode(KnowledgeGraphNode{
			ID:    personID,
			Type:  "person",
			Label: person.FullName,
			Attributes: map[string]any{
				"role":  person.Role,
				"title": person.Title,
			},
			Evidence: []KnowledgeEvidence{knowledgeEvidenceFromPerson(person)},
		})
		docID := docNodeBySHA[person.SHA256]
		if docID != "" {
			addEdge(KnowledgeGraphEdge{From: docID, To: personID, Type: "mentions_person", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromPerson(person)}})
		}
		addEdge(KnowledgeGraphEdge{From: companyID, To: personID, Type: "has_related_person", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromPerson(person)}})
	}
	for _, item := range ownership {
		distinctFactKeys[knowledgeOwnershipKey(item)] = true
		holder := strings.TrimSpace(item.HolderName)
		if holder == "" {
			continue
		}
		holderID := knowledgeNodeID("shareholder", holder)
		addNode(KnowledgeGraphNode{
			ID:    holderID,
			Type:  "shareholder",
			Label: holder,
			Attributes: map[string]any{
				"share_amount": item.ShareAmount,
				"share_ratio":  item.ShareRatio,
			},
			Evidence: []KnowledgeEvidence{knowledgeEvidenceFromOwnership(item)},
		})
		docID := docNodeBySHA[item.SHA256]
		if docID != "" {
			addEdge(KnowledgeGraphEdge{From: docID, To: holderID, Type: "mentions_shareholder", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromOwnership(item)}})
		}
		addEdge(KnowledgeGraphEdge{From: holderID, To: companyID, Type: "owns_or_controls", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromOwnership(item)}})
	}
	for _, event := range corporate {
		distinctFactKeys[knowledgeCorporateEventKey(event)] = true
		eventType := firstNonEmptyAsset(event.EventType, "corporate_event")
		eventID := knowledgeNodeID("corporate_event_type", eventType)
		addNode(KnowledgeGraphNode{ID: eventID, Type: "corporate_event_type", Label: eventType, Evidence: []KnowledgeEvidence{knowledgeEvidenceFromCorporateEvent(event)}})
		docID := docNodeBySHA[event.SHA256]
		if docID != "" {
			addEdge(KnowledgeGraphEdge{From: docID, To: eventID, Type: "contains_corporate_event", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromCorporateEvent(event)}})
		}
		addEdge(KnowledgeGraphEdge{From: companyID, To: eventID, Type: "has_corporate_event_type", Evidence: []KnowledgeEvidence{knowledgeEvidenceFromCorporateEvent(event)}})
	}
	graph.DuplicateMergeSummary.DistinctFactKeys = len(distinctFactKeys)
	graph.DuplicateMergeSummary.DuplicateFactRows = graph.DuplicateMergeSummary.FactRows - graph.DuplicateMergeSummary.DistinctFactKeys
	graph.Warnings = dedupeStrings(graph.Warnings)
	graph.Nodes = sortedKnowledgeNodes(nodes)
	graph.Edges = sortedKnowledgeEdges(edges)
	return graph
}

func skippedCompanyKnowledgeGraph(index DocumentIndex, generatedAt string) CompanyKnowledgeGraph {
	companyID := knowledgeNodeID("company", index.Ticker)
	graph := CompanyKnowledgeGraph{
		Ticker:      index.Ticker,
		GeneratedAt: generatedAt,
		Sector:      index.Sector,
		Counts:      index.Counts,
		SourceFiles: companyKnowledgeGraphSourceFiles(),
		Warnings:    dedupeStrings(append(append([]string{}, index.Warnings...), "knowledge_graph_skipped")),
		Nodes: []KnowledgeGraphNode{{
			ID:    companyID,
			Type:  "company",
			Label: index.Ticker,
			Attributes: map[string]any{
				"ticker": index.Ticker,
				"title":  index.Sector.Title,
			},
		}},
		DuplicateMergeSummary: KnowledgeDuplicateMergeSummary{
			Documents:          len(index.Documents),
			DistinctDocuments:  distinctDocumentHashCount(index.Documents),
			DuplicateDocuments: len(index.Documents) - distinctDocumentHashCount(index.Documents),
		},
	}
	if index.Sector.Sector != "" {
		sectorID := knowledgeNodeID("sector", index.Sector.Sector)
		graph.Nodes = append(graph.Nodes, KnowledgeGraphNode{
			ID:    sectorID,
			Type:  "sector",
			Label: index.Sector.Sector,
			Attributes: map[string]any{
				"main_sector": index.Sector.MainSector,
				"source":      index.Sector.Source,
				"fallback":    index.Sector.Fallback,
			},
		})
		graph.Edges = append(graph.Edges, KnowledgeGraphEdge{
			ID:   "edge:" + stableFactID(companyID, sectorID, "classified_in_sector"),
			From: companyID,
			To:   sectorID,
			Type: "classified_in_sector",
		})
	}
	return graph
}

func companyKnowledgeGraphSourceFiles() map[string]string {
	return map[string]string{
		DocumentFactsFile:         DocumentFactsFile,
		FinancialFactsFile:        FinancialFactsFile,
		FinancialTablesFile:       FinancialTablesFile,
		PeopleFile:                PeopleFile,
		OwnershipFactsFile:        OwnershipFactsFile,
		CorporateFactsFile:        CorporateFactsFile,
		DocumentIndexFile:         DocumentIndexFile,
		CompanyKnowledgeGraphFile: CompanyKnowledgeGraphFile,
	}
}

func distinctDocumentHashCount(docs []IndexedDocument) int {
	seen := map[string]bool{}
	for _, doc := range docs {
		key := firstNonEmptyAsset(doc.SHA256, doc.FilePath)
		if key != "" {
			seen[key] = true
		}
	}
	return len(seen)
}

func sortedKnowledgeNodes(nodes map[string]KnowledgeGraphNode) []KnowledgeGraphNode {
	out := make([]KnowledgeGraphNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func sortedKnowledgeEdges(edges map[string]KnowledgeGraphEdge) []KnowledgeGraphEdge {
	out := make([]KnowledgeGraphEdge, 0, len(edges))
	for _, edge := range edges {
		out = append(out, edge)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func appendLimitedEvidence(existing []KnowledgeEvidence, values ...KnowledgeEvidence) []KnowledgeEvidence {
	seen := map[string]bool{}
	out := append([]KnowledgeEvidence{}, existing...)
	for _, item := range out {
		seen[knowledgeEvidenceKey(item)] = true
	}
	for _, item := range values {
		if item.SourceFile == "" && item.Snippet == "" && item.SHA256 == "" {
			continue
		}
		key := knowledgeEvidenceKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func knowledgeEvidenceKey(item KnowledgeEvidence) string {
	period := ""
	if item.Period != nil {
		period = *item.Period
	}
	return item.SourceFile + "|" + item.SHA256 + "|" + period + "|" + item.Snippet
}

func knowledgeNodeID(prefix, value string) string {
	key := util.SlugTR(strings.TrimSpace(value))
	if key == "" {
		key = stableFactID(prefix, value)
	}
	return prefix + ":" + key
}

func knowledgeEvidenceFromIndexedDocument(doc IndexedDocument) KnowledgeEvidence {
	return KnowledgeEvidence{SourceFile: doc.FilePath, SHA256: doc.SHA256, Period: doc.Period, Snippet: knowledgeEvidenceSnippet(doc.FileName), Confidence: clampAsset(doc.QualityScore, 0, 1)}
}

func knowledgeEvidenceFromDocumentFact(fact DocumentFact) KnowledgeEvidence {
	return KnowledgeEvidence{SourceFile: fact.SourceFile, SHA256: fact.SHA256, Period: fact.Period, Snippet: knowledgeEvidenceSnippet(fact.Source.Snippet), Confidence: fact.Confidence}
}

func knowledgeEvidenceFromFinancialFact(fact ExtractedFinancialFact) KnowledgeEvidence {
	return KnowledgeEvidence{SourceFile: fact.SourceFile, SHA256: fact.SHA256, Period: fact.Period, Snippet: knowledgeEvidenceSnippet(fact.Source.Snippet), Confidence: fact.Confidence}
}

func knowledgeEvidenceFromFinancialTable(table ExtractedFinancialTable) KnowledgeEvidence {
	return KnowledgeEvidence{SourceFile: table.SourceFile, SHA256: table.SHA256, Period: table.Period, Snippet: knowledgeEvidenceSnippet(table.Source.Snippet), Confidence: table.Confidence}
}

func knowledgeEvidenceFromPerson(person ExtractedPerson) KnowledgeEvidence {
	return KnowledgeEvidence{SourceFile: person.SourceFile, SHA256: person.SHA256, Period: person.Period, Snippet: knowledgeEvidenceSnippet(person.Source.Snippet), Confidence: person.Confidence}
}

func knowledgeEvidenceFromOwnership(fact OwnershipFact) KnowledgeEvidence {
	return KnowledgeEvidence{SourceFile: fact.SourceFile, SHA256: fact.SHA256, Period: fact.Period, Snippet: knowledgeEvidenceSnippet(fact.Source.Snippet), Confidence: fact.Confidence}
}

func knowledgeEvidenceFromCorporateEvent(event ExtractedCorporateEvent) KnowledgeEvidence {
	return KnowledgeEvidence{SourceFile: event.SourceFile, SHA256: event.SHA256, Period: event.Period, Snippet: knowledgeEvidenceSnippet(event.Source.Snippet), Confidence: event.Confidence}
}

func knowledgeEvidenceSnippet(value string) string {
	return truncateAssetSnippet(strings.Join(strings.Fields(value), " "), 220)
}

func knowledgeFactKey(fact DocumentFact) string {
	period := ""
	if fact.Period != nil {
		period = *fact.Period
	}
	return strings.Join([]string{"document", fact.Group, fact.Kind, fact.NormalizedKey, period, fact.RawValue}, "|")
}

func knowledgeFinancialFactKey(fact ExtractedFinancialFact) string {
	period := ""
	if fact.Period != nil {
		period = *fact.Period
	}
	return strings.Join([]string{"financial", fact.StatementType, fact.LineItemNormalized, period, fact.Currency, fact.Unit, fmt.Sprintf("%.4f", fact.Value)}, "|")
}

func knowledgeFinancialTableKey(table ExtractedFinancialTable) string {
	period := ""
	if table.Period != nil {
		period = *table.Period
	}
	return strings.Join([]string{"financial_table", table.TableType, period, table.SHA256}, "|")
}

func knowledgePersonKey(person ExtractedPerson) string {
	period := ""
	if person.Period != nil {
		period = *person.Period
	}
	return strings.Join([]string{"person", person.NormalizedName, person.Role, period}, "|")
}

func knowledgeOwnershipKey(fact OwnershipFact) string {
	period := ""
	if fact.Period != nil {
		period = *fact.Period
	}
	ratio := ""
	if fact.ShareRatio != nil {
		ratio = fmt.Sprintf("%.4f", *fact.ShareRatio)
	}
	return strings.Join([]string{"ownership", fact.HolderName, ratio, period}, "|")
}

func knowledgeCorporateEventKey(event ExtractedCorporateEvent) string {
	period := ""
	if event.Period != nil {
		period = *event.Period
	}
	return strings.Join([]string{"corporate_event", event.EventType, event.Title, period}, "|")
}

func ReconcileFinancialValueDifferences(financials []ExtractedFinancialFact) []KnowledgeContradictionResolution {
	groups := map[string][]knowledgeValueEvidence{}
	for _, fact := range financials {
		if fact.Confidence < 0.78 || fact.LineItemNormalized == "" {
			continue
		}
		if fact.Certification.Status == EvidenceStatusRejected {
			continue
		}
		period := ""
		if fact.Period != nil {
			period = *fact.Period
		}
		key := strings.Join([]string{period, fact.StatementType, fact.LineItemNormalized, fact.Currency, fact.Unit}, "|")
		groups[key] = append(groups[key], knowledgeValueEvidence{value: fact.Value, fact: fact})
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	resolved := []KnowledgeContradictionResolution{}
	for _, key := range keys {
		values := groups[key]
		distinct := map[string][]knowledgeValueEvidence{}
		for _, item := range values {
			distinct[fmt.Sprintf("%.2f", item.value)] = append(distinct[fmt.Sprintf("%.2f", item.value)], item)
		}
		if len(distinct) < 2 {
			continue
		}
		minValue, maxValue := math.Inf(1), math.Inf(-1)
		for _, items := range distinct {
			if len(items) == 0 {
				continue
			}
			value := items[0].value
			if value < minValue {
				minValue = value
			}
			if value > maxValue {
				maxValue = value
			}
		}
		tolerance := math.Max(1, math.Abs(maxValue)*0.005)
		if math.Abs(maxValue-minValue) <= tolerance {
			continue
		}
		candidates := make([]knowledgeContradictionCandidate, 0, len(distinct))
		for _, grouped := range distinct {
			candidates = append(candidates, knowledgeBestContradictionCandidate(grouped))
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Score != candidates[j].Score {
				return candidates[i].Score > candidates[j].Score
			}
			if candidates[i].Fact.SourceFile != candidates[j].Fact.SourceFile {
				return candidates[i].Fact.SourceFile < candidates[j].Fact.SourceFile
			}
			return candidates[i].Value > candidates[j].Value
		})
		items := knowledgeContradictionItems(candidates)
		evidence := knowledgeContradictionEvidence(candidates)
		resolved = append(resolved, resolveKnowledgeContradiction(key, candidates, items, evidence))
	}
	return resolved
}

type knowledgeContradictionCandidate struct {
	Value float64
	Fact  ExtractedFinancialFact
	Score float64
	Count int
}

type knowledgeValueEvidence struct {
	value float64
	fact  ExtractedFinancialFact
}

func knowledgeBestContradictionCandidate(items []knowledgeValueEvidence) knowledgeContradictionCandidate {
	best := knowledgeContradictionCandidate{}
	for i, item := range items {
		score := knowledgeFinancialFactScore(item.fact)
		candidate := knowledgeContradictionCandidate{
			Value: item.value,
			Fact:  item.fact,
			Score: score,
			Count: len(items),
		}
		if i == 0 || knowledgeContradictionCandidateBetter(candidate, best) {
			best = candidate
		}
	}
	best.Score += math.Log1p(float64(best.Count)) * 0.08
	return best
}

func knowledgeContradictionCandidateBetter(candidate, current knowledgeContradictionCandidate) bool {
	if candidate.Score != current.Score {
		return candidate.Score > current.Score
	}
	candidateTime := knowledgeFactTime(candidate.Fact)
	currentTime := knowledgeFactTime(current.Fact)
	if !candidateTime.IsZero() && !currentTime.IsZero() && !candidateTime.Equal(currentTime) {
		return candidateTime.After(currentTime)
	}
	if candidate.Fact.SourceFile != current.Fact.SourceFile {
		return candidate.Fact.SourceFile < current.Fact.SourceFile
	}
	if candidate.Fact.ID != "" && current.Fact.ID != "" {
		return candidate.Fact.ID < current.Fact.ID
	}
	return candidate.Value > current.Value
}

func knowledgeContradictionItems(candidates []knowledgeContradictionCandidate) []KnowledgeContradictionItem {
	items := make([]KnowledgeContradictionItem, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, KnowledgeContradictionItem{
			Value:      candidate.Value,
			Currency:   candidate.Fact.Currency,
			Unit:       candidate.Fact.Unit,
			SourceFile: candidate.Fact.SourceFile,
			Period:     candidate.Fact.Period,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items
}

func knowledgeContradictionEvidence(candidates []knowledgeContradictionCandidate) []KnowledgeEvidence {
	evidence := []KnowledgeEvidence{}
	for _, candidate := range candidates {
		evidence = appendLimitedEvidence(evidence, knowledgeEvidenceFromFinancialFact(candidate.Fact))
	}
	return evidence
}

func resolveKnowledgeContradiction(key string, candidates []knowledgeContradictionCandidate, items []KnowledgeContradictionItem, evidence []KnowledgeEvidence) KnowledgeContradictionResolution {
	if len(candidates) < 2 {
		return KnowledgeContradictionResolution{}
	}
	winner := candidates[0]
	runnerUp := candidates[1]
	status := "resolved_by_source_priority"
	reason := "Kaynak önceliği, belge kanıtı ve güven puanı birlikte değerlendirilerek seçildi."
	if winner.Count > runnerUp.Count {
		status = "resolved_by_consensus"
		reason = "Aynı değeri destekleyen daha fazla kaynak olduğu için seçildi."
	}
	if winner.Fact.Certification.Status == EvidenceStatusCertified && runnerUp.Fact.Certification.Status != EvidenceStatusCertified {
		status = "resolved_by_certification"
		reason = "Sertifikalı ve analize uygun kaynak olduğu için seçildi."
	} else if winner.Fact.Certification.Status == EvidenceStatusAIResolved && runnerUp.Fact.Certification.Status != EvidenceStatusCertified && runnerUp.Fact.Certification.Status != EvidenceStatusAIResolved {
		status = "resolved_by_ai_adjudication"
		reason = "AI tarafından çözülen yapılandırılmış kanıt daha yüksek kaynak güveni verdiği için seçildi."
	} else if winner.Score >= runnerUp.Score+0.35 {
		status = "resolved_by_evidence_score"
		reason = "Daha yüksek kaynak güveni ve belge kanıtı nedeniyle seçildi."
	}
	return KnowledgeContradictionResolution{
		Type:               "financial_value_conflict",
		Key:                key,
		Status:             status,
		SelectedValue:      winner.Value,
		Currency:           winner.Fact.Currency,
		Unit:               winner.Fact.Unit,
		SelectedSourceFile: winner.Fact.SourceFile,
		Period:             winner.Fact.Period,
		Reason:             reason,
		Confidence:         clampKnowledgeScore(winner.Score, 0, 3) / 3,
		CompetingValues:    items,
		Evidence:           evidence,
	}
}

func knowledgeFinancialFactScore(fact ExtractedFinancialFact) float64 {
	score := fact.Confidence
	score += knowledgeFinancialSourcePriority(fact)
	switch fact.Certification.Status {
	case EvidenceStatusCertified:
		score += 1.0
	case EvidenceStatusAIResolved:
		score += 0.75
	case EvidenceStatusReviewRequired:
		score += 0.2
	case EvidenceStatusRejected:
		score -= 1.0
	}
	if fact.Certification.AnalysisUsable {
		score += 0.35
	}
	if !fact.ReviewRequired {
		score += 0.2
	}
	if fact.Source.TableID != "" {
		score += 0.25
	}
	if fact.Source.Page > 0 {
		score += 0.05
	}
	if strings.EqualFold(fact.ConsolidationScope, "consolidated") || strings.TrimSpace(fact.ConsolidationScope) == "" {
		score += 0.1
	}
	if strings.EqualFold(fact.AuditStatus, "audited") {
		score += 0.1
	}
	return score
}

func knowledgeFinancialSourcePriority(fact ExtractedFinancialFact) float64 {
	text := strings.ToLower(strings.Join([]string{fact.SourceFile, fact.StatementType, fact.Source.TableID, fact.Source.Snippet}, " "))
	switch {
	case strings.Contains(text, "finansal") || strings.Contains(text, "financial"):
		return 0.35
	case strings.Contains(text, "faaliyet") || strings.Contains(text, "annual"):
		return 0.25
	case strings.Contains(text, "degerleme") || strings.Contains(text, "değerleme") || strings.Contains(text, "valuation"):
		return 0.10
	default:
		return 0
	}
}

func knowledgeFactTime(fact ExtractedFinancialFact) time.Time {
	for _, value := range []string{stringPtrValueKAP(fact.DocumentDate), fact.CreatedAt} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02", "02.01.2006", "02/01/2006"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

func stringPtrValueKAP(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func clampKnowledgeScore(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func sortDocumentFacts(rows []DocumentFact) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Period != nil && rows[j].Period != nil && *rows[i].Period != *rows[j].Period {
			return *rows[i].Period > *rows[j].Period
		}
		if rows[i].Group != rows[j].Group {
			return rows[i].Group < rows[j].Group
		}
		return rows[i].SourceFile < rows[j].SourceFile
	})
}

func sortFinancialFacts(rows []ExtractedFinancialFact) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Period != nil && rows[j].Period != nil && *rows[i].Period != *rows[j].Period {
			return *rows[i].Period > *rows[j].Period
		}
		if rows[i].LineItemNormalized != rows[j].LineItemNormalized {
			return rows[i].LineItemNormalized < rows[j].LineItemNormalized
		}
		return rows[i].SourceFile < rows[j].SourceFile
	})
}

func sortFinancialTables(rows []ExtractedFinancialTable) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Period != nil && rows[j].Period != nil && *rows[i].Period != *rows[j].Period {
			return *rows[i].Period > *rows[j].Period
		}
		if rows[i].TableType != rows[j].TableType {
			return rows[i].TableType < rows[j].TableType
		}
		return rows[i].SourceFile < rows[j].SourceFile
	})
}

func sortPeopleFacts(rows []ExtractedPerson) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Period != nil && rows[j].Period != nil && *rows[i].Period != *rows[j].Period {
			return *rows[i].Period > *rows[j].Period
		}
		return rows[i].NormalizedName < rows[j].NormalizedName
	})
}

func sortOwnershipFacts(rows []OwnershipFact) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Period != nil && rows[j].Period != nil && *rows[i].Period != *rows[j].Period {
			return *rows[i].Period > *rows[j].Period
		}
		return rows[i].HolderName < rows[j].HolderName
	})
}

func sortCorporateFacts(rows []ExtractedCorporateEvent) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Period != nil && rows[j].Period != nil && *rows[i].Period != *rows[j].Period {
			return *rows[i].Period > *rows[j].Period
		}
		if rows[i].EventType != rows[j].EventType {
			return rows[i].EventType < rows[j].EventType
		}
		return rows[i].SourceFile < rows[j].SourceFile
	})
}

func writeJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeJSONLRows(path string, rows any) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	switch values := rows.(type) {
	case []DocumentFact:
		for _, row := range values {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
	case []ExtractedFinancialFact:
		for _, row := range values {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
	case []ExtractedFinancialTable:
		for _, row := range values {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
	case []ExtractedPerson:
		for _, row := range values {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
	case []OwnershipFact:
		for _, row := range values {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
	case []ExtractedCorporateEvent:
		for _, row := range values {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported jsonl rows type %T", rows)
	}
	return nil
}
