package kapingest

import (
	"context"
	"time"
)

const (
	DefaultWorkers         = 4
	DefaultExtractTimeout  = 60 * time.Second
	DefaultOCRTimeout      = 10 * time.Minute
	DefaultVisionTimeout   = 10 * time.Minute
	DefaultVisionMaxPages  = 20
	DefaultVisionRenderDPI = 180

	RawDocumentsFile          = "raw_documents.jsonl"
	KAPEventsFile             = "kap_events.jsonl"
	AssetEventsFile           = "asset_events.jsonl"
	AssetInventoryFile        = "asset_inventory.json"
	DocumentFactsFile         = "document_facts.jsonl"
	DocumentIndexFile         = "document_index.json"
	FinancialFactsFile        = "financial_facts.jsonl"
	FinancialTablesFile       = "financial_tables.jsonl"
	CompanyKnowledgeGraphFile = "company_knowledge_graph.json"
	PeopleFile                = "people.jsonl"
	OwnershipFactsFile        = "ownership_facts.jsonl"
	CorporateFactsFile        = "corporate_events.jsonl"
	ExtractionErrorsFile      = "extraction_errors.jsonl"
	ProcessedFilesFile        = "processed_files.jsonl"
)

type PDFFile struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	Ticker   string `json:"ticker"`
}

type RawDocument struct {
	FilePath            string      `json:"file_path"`
	SHA256              string      `json:"sha256"`
	Ticker              string      `json:"ticker"`
	FileName            string      `json:"file_name"`
	ExtractionMethod    string      `json:"extraction_method"`
	DocumentTypeGuess   string      `json:"document_type_guess"`
	Text                string      `json:"text"`
	TextLength          int         `json:"text_length"`
	QualityScore        float64     `json:"quality_score"`
	ParseStatus         string      `json:"parse_status,omitempty"`
	AnalysisUsable      bool        `json:"analysis_usable"`
	HumanReviewRequired bool        `json:"human_review_required,omitempty"`
	AIResolved          bool        `json:"ai_resolved,omitempty"`
	AIConfidence        float64     `json:"ai_confidence,omitempty"`
	QualityGate         QualityGate `json:"quality_gate,omitempty"`
	Warnings            []string    `json:"warnings"`
	CreatedAt           string      `json:"created_at"`
}

type QualityGate struct {
	Status              string   `json:"status,omitempty"`
	AnalysisUsable      bool     `json:"analysis_usable"`
	HumanReviewRequired bool     `json:"human_review_required,omitempty"`
	AIResolved          bool     `json:"ai_resolved,omitempty"`
	AIConfidence        float64  `json:"ai_confidence,omitempty"`
	TrustedThreshold    float64  `json:"trusted_threshold,omitempty"`
	RejectedThreshold   float64  `json:"rejected_threshold,omitempty"`
	Reasons             []string `json:"reasons,omitempty"`
}

type KAPEvent struct {
	Ticker              string            `json:"ticker"`
	CompanyName         string            `json:"company_name"`
	FilePath            string            `json:"file_path"`
	SHA256              string            `json:"sha256"`
	DocumentDate        *string           `json:"document_date"`
	Period              *string           `json:"period"`
	DocumentClass       string            `json:"document_class"`
	FinancialProfile    string            `json:"financial_profile"`
	EventCategory       string            `json:"event_category"`
	Summary             string            `json:"summary"`
	ImportantPoints     []string          `json:"important_points"`
	FinancialHighlights map[string]any    `json:"financial_highlights"`
	PortfolioHighlights map[string]any    `json:"portfolio_highlights"`
	PositivePoints      []string          `json:"positive_points"`
	NegativePoints      []string          `json:"negative_points"`
	RiskFlags           []string          `json:"risk_flags"`
	OpportunityFlags    []string          `json:"opportunity_flags"`
	Impact              Impact            `json:"impact"`
	SourceReferences    []SourceReference `json:"source_references"`
}

type Impact struct {
	Fundamental    string  `json:"fundamental"`
	ShortTermPrice string  `json:"short_term_price"`
	LongTerm       string  `json:"long_term"`
	Confidence     float64 `json:"confidence"`
	Reason         string  `json:"reason"`
}

type SourceReference struct {
	Page    *int   `json:"page"`
	Field   string `json:"field"`
	Snippet string `json:"snippet"`
}

type AssetEvent struct {
	Ticker                   string                 `json:"ticker"`
	CompanyName              string                 `json:"company_name"`
	SourceFile               string                 `json:"source_file"`
	SHA256                   string                 `json:"sha256"`
	DocumentDate             *string                `json:"document_date"`
	Period                   *string                `json:"period"`
	AssetName                string                 `json:"asset_name"`
	AssetType                string                 `json:"asset_type"`
	Location                 string                 `json:"location"`
	City                     string                 `json:"city"`
	District                 string                 `json:"district"`
	AreaM2                   *float64               `json:"area_m2"`
	ParcelInfo               string                 `json:"parcel_info"`
	OwnershipType            string                 `json:"ownership_type"`
	Status                   string                 `json:"status"`
	ExpertiseDate            *string                `json:"expertise_date"`
	ExpertiseValueExclVATTRY *float64               `json:"expertise_value_excl_vat_try"`
	ExpertiseValueInclVATTRY *float64               `json:"expertise_value_incl_vat_try"`
	BookValueTRY             *float64               `json:"book_value_try"`
	RentalInfo               AssetRentalInfo        `json:"rental_info"`
	IncomeRelevance          string                 `json:"income_relevance"`
	RiskFlags                []string               `json:"risk_flags"`
	OpportunityFlags         []string               `json:"opportunity_flags"`
	SourceReferences         []AssetSourceReference `json:"source_references"`
	Confidence               float64                `json:"confidence"`
}

type AssetRentalInfo struct {
	IsRented          *bool    `json:"is_rented"`
	MonthlyRentTRY    *float64 `json:"monthly_rent_try"`
	AnnualRentTRY     *float64 `json:"annual_rent_try"`
	AnnualMinRentUSD  *float64 `json:"annual_min_rent_usd"`
	VariableRentTerms string   `json:"variable_rent_terms"`
	Tenant            string   `json:"tenant"`
}

type AssetSourceReference struct {
	Page    *int   `json:"page"`
	Snippet string `json:"snippet"`
}

type AssetInventory struct {
	Ticker           string               `json:"ticker"`
	CompanyName      string               `json:"company_name"`
	GeneratedAt      string               `json:"generated_at"`
	EventCount       int                  `json:"event_count"`
	AssetCount       int                  `json:"asset_count"`
	Assets           []AssetInventoryItem `json:"assets"`
	PortfolioSummary PortfolioSummary     `json:"portfolio_summary"`
	ValuationNotes   []ValuationNote      `json:"valuation_notes,omitempty"`
	GYOSummary       GYOAssetSummary      `json:"gyo_summary"`
	Warnings         []string             `json:"warnings,omitempty"`
}

type AssetInventoryItem struct {
	AssetName        string                 `json:"asset_name"`
	AssetType        string                 `json:"asset_type"`
	Location         string                 `json:"location"`
	City             string                 `json:"city"`
	District         string                 `json:"district"`
	AreaM2           *float64               `json:"area_m2"`
	ParcelInfo       string                 `json:"parcel_info"`
	OwnershipType    string                 `json:"ownership_type"`
	Status           string                 `json:"status"`
	RentalInfo       AssetRentalInfo        `json:"rental_info"`
	IncomeRelevance  string                 `json:"income_relevance"`
	RiskFlags        []string               `json:"risk_flags,omitempty"`
	OpportunityFlags []string               `json:"opportunity_flags,omitempty"`
	SourceReferences []AssetSourceReference `json:"source_references,omitempty"`
	Confidence       float64                `json:"confidence"`
	History          []AssetHistoryPoint    `json:"history"`
}

type AssetHistoryPoint struct {
	Period                   string   `json:"period"`
	ExpertiseDate            string   `json:"expertise_date"`
	ExpertiseValueExclVATTRY *float64 `json:"expertise_value_excl_vat_try"`
	ExpertiseValueInclVATTRY *float64 `json:"expertise_value_incl_vat_try"`
	BookValueTRY             *float64 `json:"book_value_try,omitempty"`
	MonthlyRentTRY           *float64 `json:"monthly_rent_try"`
	AnnualRentTRY            *float64 `json:"annual_rent_try,omitempty"`
	AnnualMinRentUSD         *float64 `json:"annual_min_rent_usd"`
	SourceFile               string   `json:"source_file"`
}

type GYOAssetSummary struct {
	Ticker                         string             `json:"ticker"`
	TotalRealEstateValueExclVATTRY *float64           `json:"total_real_estate_value_excl_vat_try"`
	TotalRealEstateValueInclVATTRY *float64           `json:"total_real_estate_value_incl_vat_try"`
	TotalRentalAssets              int                `json:"total_rental_assets"`
	TotalProjects                  int                `json:"total_projects"`
	LargestAssets                  []AssetSummaryItem `json:"largest_assets"`
	RentalIncomeAssets             []AssetSummaryItem `json:"rental_income_assets"`
	FXLinkedRentalAssets           []AssetSummaryItem `json:"fx_linked_rental_assets"`
	UnderConstructionProjects      []AssetSummaryItem `json:"under_construction_projects"`
	Warnings                       []string           `json:"warnings"`
}

type AssetSummaryItem struct {
	AssetName string   `json:"asset_name"`
	AssetType string   `json:"asset_type"`
	Location  string   `json:"location"`
	ValueTRY  *float64 `json:"value_try,omitempty"`
	RentTRY   *float64 `json:"rent_try,omitempty"`
	RentUSD   *float64 `json:"rent_usd,omitempty"`
}

type PortfolioSummary struct {
	Ticker                         string                     `json:"ticker"`
	TotalRealEstateValueExclVATTRY *float64                   `json:"total_real_estate_value_excl_vat_try"`
	TotalRealEstateValueInclVATTRY *float64                   `json:"total_real_estate_value_incl_vat_try"`
	TotalBookValueTRY              *float64                   `json:"total_book_value_try,omitempty"`
	History                        []PortfolioSummarySnapshot `json:"history,omitempty"`
	SourceReferences               []AssetSourceReference     `json:"source_references,omitempty"`
	Warnings                       []string                   `json:"warnings,omitempty"`
}

type PortfolioSummarySnapshot struct {
	Period                         string   `json:"period"`
	DocumentDate                   string   `json:"document_date"`
	TotalRealEstateValueExclVATTRY *float64 `json:"total_real_estate_value_excl_vat_try"`
	TotalRealEstateValueInclVATTRY *float64 `json:"total_real_estate_value_incl_vat_try"`
	TotalBookValueTRY              *float64 `json:"total_book_value_try,omitempty"`
	SourceFile                     string   `json:"source_file"`
	Snippet                        string   `json:"snippet"`
}

type ValuationNote struct {
	Ticker           string                 `json:"ticker"`
	SourceFile       string                 `json:"source_file"`
	SHA256           string                 `json:"sha256"`
	DocumentDate     *string                `json:"document_date"`
	Period           *string                `json:"period"`
	NoteType         string                 `json:"note_type"`
	Snippet          string                 `json:"snippet"`
	SourceReferences []AssetSourceReference `json:"source_references,omitempty"`
	Confidence       float64                `json:"confidence"`
}

type ExtractionError struct {
	FilePath  string `json:"file_path"`
	SHA256    string `json:"sha256"`
	Stage     string `json:"stage"`
	Error     string `json:"error"`
	CreatedAt string `json:"created_at"`
	RawReply  string `json:"raw_reply,omitempty"`
}

type ProcessedFile struct {
	FilePath            string  `json:"file_path"`
	SHA256              string  `json:"sha256"`
	Ticker              string  `json:"ticker"`
	DocumentTypeGuess   string  `json:"document_type_guess"`
	QualityScore        float64 `json:"quality_score"`
	TextLength          int     `json:"text_length"`
	ParseStatus         string  `json:"parse_status,omitempty"`
	AnalysisUsable      bool    `json:"analysis_usable"`
	HumanReviewRequired bool    `json:"human_review_required,omitempty"`
	AIResolved          bool    `json:"ai_resolved,omitempty"`
	AIConfidence        float64 `json:"ai_confidence,omitempty"`
	LLMEnabled          bool    `json:"llm_enabled"`
	CreatedAt           string  `json:"created_at"`
}

type Options struct {
	InputDir        string
	OutputDir       string
	Workers         int
	Limit           int
	Resume          bool
	LLM             bool
	DryRun          bool
	ExtractTimeout  time.Duration
	DisableOCR      bool
	OCRTimeout      time.Duration
	OCRLanguages    string
	EnableVision    bool
	VisionCommand   string
	VisionTimeout   time.Duration
	VisionMaxPages  int
	VisionRenderDPI int
	KAPSectorsPath  string
	PromptPackPath  string
	TextExtractor   TextExtractor
	LLMClient       LLMClient
	Now             func() time.Time
}

type Summary struct {
	Status              string    `json:"status"`
	InputDir            string    `json:"input_dir"`
	OutputDir           string    `json:"output_dir"`
	Workers             int       `json:"workers"`
	Limit               int       `json:"limit"`
	Resume              bool      `json:"resume"`
	LLM                 bool      `json:"llm"`
	DryRun              bool      `json:"dry_run"`
	Scanned             int       `json:"scanned"`
	Planned             int       `json:"planned"`
	Skipped             int       `json:"skipped"`
	RawDocuments        int       `json:"raw_documents"`
	AnalysisUsable      int       `json:"analysis_usable_documents"`
	ReviewRequired      int       `json:"review_required_documents"`
	Rejected            int       `json:"rejected_documents"`
	KAPEvents           int       `json:"kap_events"`
	AssetEvents         int       `json:"asset_events"`
	AssetInventories    int       `json:"asset_inventories"`
	DocumentFacts       int       `json:"document_facts"`
	FinancialFacts      int       `json:"financial_facts"`
	FinancialTables     int       `json:"financial_tables"`
	People              int       `json:"people"`
	OwnershipFacts      int       `json:"ownership_facts"`
	CorporateEvents     int       `json:"corporate_events"`
	DocumentIndexes     int       `json:"document_indexes"`
	KnowledgeGraphs     int       `json:"knowledge_graphs"`
	CanonicalFinancials int       `json:"canonical_financials"`
	Errors              int       `json:"errors"`
	ProcessedFiles      int       `json:"processed_files"`
	SampleFiles         []PDFFile `json:"sample_files,omitempty"`
	OutputFiles         []string  `json:"output_files,omitempty"`
	StartedAt           string    `json:"started_at"`
	FinishedAt          string    `json:"finished_at"`
}

type QualityResult struct {
	Score          float64
	TextLength     int
	NumericDensity float64
	BrokenRatio    float64
	Warnings       []string
}

type TextExtractor interface {
	ExtractText(ctx context.Context, filePath string) (text string, method string, warnings []string, err error)
}
