package kapextract

import (
	"errors"
	"strings"
	"time"
)

type SourceRef struct {
	SourceDocumentID string  `json:"source_document_id"`
	SourceSystem     string  `json:"source_system"`
	Ticker           string  `json:"ticker"`
	LocalFilePath    string  `json:"local_file_path,omitempty"`
	SourcePage       int     `json:"source_page,omitempty"`
	SourceTableID    string  `json:"source_table_id,omitempty"`
	SourceRow        string  `json:"source_row,omitempty"`
	SourceColumn     string  `json:"source_column,omitempty"`
	SourceText       string  `json:"source_text,omitempty"`
	BBox             *BBox   `json:"bbox,omitempty"`
	ConfidenceScore  float64 `json:"confidence_score"`
}

type BBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type DocumentPage struct {
	DocumentID    string    `json:"document_id"`
	PageNumber    int       `json:"page_number"`
	TextExtracted bool      `json:"text_extracted"`
	OCRRequired   bool      `json:"ocr_required"`
	Checksum      string    `json:"checksum,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type TextBlock struct {
	BlockID          string    `json:"block_id"`
	DocumentID       string    `json:"document_id"`
	PageNumber       int       `json:"page_number"`
	BlockIndex       int       `json:"block_index"`
	BlockType        string    `json:"block_type"`
	Text             string    `json:"text"`
	ExtractionMethod string    `json:"extraction_method"`
	ConfidenceScore  float64   `json:"confidence_score"`
	ReviewRequired   bool      `json:"review_required"`
	CreatedAt        time.Time `json:"created_at"`
}

type DocumentTable struct {
	TableID          string     `json:"table_id"`
	DocumentID       string     `json:"document_id"`
	PageNumber       int        `json:"page_number"`
	TableIndex       int        `json:"table_index"`
	Rows             [][]string `json:"rows,omitempty"`
	ExtractionMethod string     `json:"extraction_method"`
	ConfidenceScore  float64    `json:"confidence_score"`
	ReviewRequired   bool       `json:"review_required"`
	CreatedAt        time.Time  `json:"created_at"`
}

type StatementType string

const (
	StatementBalanceSheet    StatementType = "balance_sheet"
	StatementIncomeStatement StatementType = "income_statement"
	StatementCashFlow        StatementType = "cash_flow_statement"
	StatementEquityStatement StatementType = "equity_statement"
	StatementNote            StatementType = "note"
)

type ValidationStatus string

const (
	ValidationValid   ValidationStatus = "valid"
	ValidationInvalid ValidationStatus = "invalid"
	ValidationWarning ValidationStatus = "warning"
	ValidationUnknown ValidationStatus = "unknown"
)

type FinancialFact struct {
	FactID             string           `json:"fact_id"`
	CompanyID          string           `json:"company_id,omitempty"`
	Ticker             string           `json:"ticker"`
	Period             string           `json:"period,omitempty"`
	FiscalYear         int              `json:"fiscal_year,omitempty"`
	StatementType      StatementType    `json:"statement_type"`
	LineItemOriginal   string           `json:"line_item_original"`
	LineItemNormalized string           `json:"line_item_normalized"`
	Value              float64          `json:"value"`
	Currency           string           `json:"currency"`
	Unit               string           `json:"unit"`
	IsConsolidated     bool             `json:"is_consolidated"`
	AccountingStandard string           `json:"accounting_standard"`
	IsAudited          *bool            `json:"is_audited,omitempty"`
	Source             SourceRef        `json:"source"`
	ConfidenceScore    float64          `json:"confidence_score"`
	ValidationStatus   ValidationStatus `json:"validation_status"`
	ReviewRequired     bool             `json:"review_required"`
	CreatedAt          time.Time        `json:"created_at"`
}

func (f FinancialFact) Validate() error {
	if strings.TrimSpace(f.FactID) == "" {
		return errors.New("fact_id is required")
	}
	if strings.TrimSpace(f.Ticker) == "" {
		return errors.New("ticker is required")
	}
	if strings.TrimSpace(f.LineItemOriginal) == "" || strings.TrimSpace(f.LineItemNormalized) == "" {
		return errors.New("line item is required")
	}
	if strings.TrimSpace(f.Source.SourceDocumentID) == "" {
		return errors.New("source_document_id is required")
	}
	if f.ConfidenceScore < 0 || f.ConfidenceScore > 1 {
		return errors.New("confidence_score must be between 0 and 1")
	}
	return nil
}

type CompanyInfoCard struct {
	Ticker                   string             `json:"ticker"`
	CompanyName              string             `json:"company_name,omitempty"`
	LegalName                string             `json:"legal_name,omitempty"`
	KAPCompanyID             string             `json:"kap_company_id,omitempty"`
	Sector                   string             `json:"sector,omitempty"`
	Industry                 string             `json:"industry,omitempty"`
	SubIndustry              string             `json:"sub_industry,omitempty"`
	FoundationDate           string             `json:"foundation_date,omitempty"`
	Headquarters             string             `json:"headquarters,omitempty"`
	Address                  string             `json:"address,omitempty"`
	Phone                    string             `json:"phone,omitempty"`
	Website                  string             `json:"website,omitempty"`
	Email                    string             `json:"email,omitempty"`
	TradeRegistryNumber      string             `json:"trade_registry_number,omitempty"`
	MersisNumber             string             `json:"mersis_number,omitempty"`
	TaxOffice                string             `json:"tax_office,omitempty"`
	TaxNumber                string             `json:"tax_number,omitempty"`
	PaidInCapital            float64            `json:"paid_in_capital,omitempty"`
	RegisteredCapitalCeiling float64            `json:"registered_capital_ceiling,omitempty"`
	FreeFloatRate            float64            `json:"free_float_rate,omitempty"`
	ListingDate              string             `json:"listing_date,omitempty"`
	Market                   string             `json:"market,omitempty"`
	IndexMemberships         []string           `json:"index_memberships,omitempty"`
	MainShareholders         []OwnershipRecord  `json:"main_shareholders,omitempty"`
	Subsidiaries             []OwnershipRecord  `json:"subsidiaries,omitempty"`
	Associates               []OwnershipRecord  `json:"associates,omitempty"`
	JointVentures            []OwnershipRecord  `json:"joint_ventures,omitempty"`
	BoardOfDirectors         []PersonExtraction `json:"board_of_directors,omitempty"`
	ExecutiveManagement      []PersonExtraction `json:"executive_management,omitempty"`
	IndependentAuditor       string             `json:"independent_auditor,omitempty"`
	InvestorRelationsContact string             `json:"investor_relations_contact,omitempty"`
	LastUpdatedSource        string             `json:"last_updated_source,omitempty"`
	LastUpdatedAt            time.Time          `json:"last_updated_at"`
	ConfidenceScore          float64            `json:"confidence_score"`
	ReviewRequired           bool               `json:"review_required"`
}

type OwnershipRecord struct {
	Name             string  `json:"name"`
	ShareAmount      float64 `json:"share_amount,omitempty"`
	ShareRatio       float64 `json:"share_ratio,omitempty"`
	Country          string  `json:"country,omitempty"`
	SourceDocumentID string  `json:"source_document_id,omitempty"`
	ConfidenceScore  float64 `json:"confidence_score"`
	ReviewRequired   bool    `json:"review_required,omitempty"`
}

type PersonExtraction struct {
	PersonID                 string    `json:"person_id"`
	FullName                 string    `json:"full_name"`
	NormalizedName           string    `json:"normalized_name"`
	Title                    string    `json:"title,omitempty"`
	Role                     string    `json:"role,omitempty"`
	CommitteeRole            string    `json:"committee_role,omitempty"`
	StartDate                string    `json:"start_date,omitempty"`
	EndDate                  string    `json:"end_date,omitempty"`
	IsIndependentBoardMember *bool     `json:"is_independent_board_member,omitempty"`
	Education                string    `json:"education,omitempty"`
	PreviousPositions        []string  `json:"previous_positions,omitempty"`
	ShareholdingAmount       *float64  `json:"shareholding_amount,omitempty"`
	ShareholdingRatio        *float64  `json:"shareholding_ratio,omitempty"`
	RelatedPartyStatus       string    `json:"related_party_status,omitempty"`
	Source                   SourceRef `json:"source"`
	ConfidenceScore          float64   `json:"confidence_score"`
	ReviewRequired           bool      `json:"review_required"`
	CreatedAt                time.Time `json:"created_at"`
}

type CorporateEvent struct {
	EventID                string    `json:"event_id"`
	CompanyID              string    `json:"company_id,omitempty"`
	Ticker                 string    `json:"ticker"`
	EventDate              string    `json:"event_date,omitempty"`
	EventType              string    `json:"event_type"`
	EventTitle             string    `json:"event_title"`
	Description            string    `json:"description"`
	AffectedAssets         []string  `json:"affected_assets,omitempty"`
	AffectedFinancialItems []string  `json:"affected_financial_items,omitempty"`
	Amount                 *float64  `json:"amount,omitempty"`
	Currency               string    `json:"currency,omitempty"`
	Counterparty           string    `json:"counterparty,omitempty"`
	Location               string    `json:"location,omitempty"`
	Source                 SourceRef `json:"source"`
	ConfidenceScore        float64   `json:"confidence_score"`
	ReviewRequired         bool      `json:"review_required"`
	CreatedAt              time.Time `json:"created_at"`
}

type TrackedAsset struct {
	AssetID          string    `json:"asset_id"`
	CompanyID        string    `json:"company_id,omitempty"`
	Ticker           string    `json:"ticker"`
	AssetName        string    `json:"asset_name"`
	AssetType        string    `json:"asset_type"`
	Location         string    `json:"location,omitempty"`
	AcquisitionDate  string    `json:"acquisition_date,omitempty"`
	AcquisitionCost  *float64  `json:"acquisition_cost,omitempty"`
	Currency         string    `json:"currency,omitempty"`
	CurrentBookValue *float64  `json:"current_book_value,omitempty"`
	FairValue        *float64  `json:"fair_value,omitempty"`
	ValuationDate    string    `json:"valuation_date,omitempty"`
	Status           string    `json:"status"`
	RelatedEvents    []string  `json:"related_events,omitempty"`
	SourceChain      []string  `json:"source_chain,omitempty"`
	ConfidenceScore  float64   `json:"confidence_score"`
	ReviewRequired   bool      `json:"review_required"`
	CreatedAt        time.Time `json:"created_at"`
}

type EvidenceChain struct {
	EvidenceChainID      string             `json:"evidence_chain_id"`
	Subject              string             `json:"subject"`
	CompanyID            string             `json:"company_id,omitempty"`
	Ticker               string             `json:"ticker"`
	InitialEvent         SourceRef          `json:"initial_event"`
	YearlyFollowUp       []EvidenceFollowUp `json:"yearly_follow_up,omitempty"`
	CurrentStatus        string             `json:"current_status"`
	FinalConfidenceScore float64            `json:"final_confidence_score"`
	ReviewRequired       bool               `json:"review_required"`
	CreatedAt            time.Time          `json:"created_at"`
}

type EvidenceFollowUp struct {
	Year            int       `json:"year"`
	Evidence        string    `json:"evidence"`
	Status          string    `json:"status"`
	Source          SourceRef `json:"source"`
	ConfidenceScore float64   `json:"confidence_score"`
}

type ReviewItem struct {
	ReviewID        string    `json:"review_id"`
	Ticker          string    `json:"ticker"`
	SubjectType     string    `json:"subject_type"`
	SubjectID       string    `json:"subject_id,omitempty"`
	Reason          string    `json:"reason"`
	Source          SourceRef `json:"source"`
	ConfidenceScore float64   `json:"confidence_score"`
	CreatedAt       time.Time `json:"created_at"`
}

type FundamentalAnalysis struct {
	Ticker               string           `json:"ticker"`
	Period               string           `json:"period,omitempty"`
	Summary              string           `json:"summary"`
	Strengths            []string         `json:"strengths,omitempty"`
	Weaknesses           []string         `json:"weaknesses,omitempty"`
	Opportunities        []string         `json:"opportunities,omitempty"`
	Risks                []string         `json:"risks,omitempty"`
	FinancialRatios      map[string]any   `json:"financial_ratios,omitempty"`
	AssetQualityFindings []map[string]any `json:"asset_quality_findings,omitempty"`
	ManagementFindings   []map[string]any `json:"management_findings,omitempty"`
	RedFlags             []map[string]any `json:"red_flags,omitempty"`
	ConfidenceScore      float64          `json:"confidence_score"`
	Sources              []SourceRef      `json:"sources,omitempty"`
	ReviewRequired       bool             `json:"review_required"`
	Metadata             map[string]any   `json:"metadata,omitempty"`
}

type DocumentSummary struct {
	DocumentID       string    `json:"document_id"`
	Ticker           string    `json:"ticker"`
	DocumentType     string    `json:"document_type"`
	LocalFilePath    string    `json:"local_file_path"`
	OriginalFilename string    `json:"original_filename"`
	Checksum         string    `json:"checksum"`
	ExtractionStatus string    `json:"extraction_status"`
	ReviewRequired   bool      `json:"review_required"`
	ProcessedAt      time.Time `json:"processed_at"`
}

type ExtractionResult struct {
	Ticker              string                      `json:"ticker"`
	GeneratedAt         time.Time                   `json:"generated_at"`
	Documents           []DocumentSummary           `json:"documents,omitempty"`
	Pages               []DocumentPage              `json:"document_pages,omitempty"`
	TextBlocks          []TextBlock                 `json:"text_blocks,omitempty"`
	Tables              []DocumentTable             `json:"document_tables,omitempty"`
	FinancialFacts      []FinancialFact             `json:"financial_facts,omitempty"`
	CompanyInfoCard     CompanyInfoCard             `json:"company_info_card,omitempty"`
	People              []PersonExtraction          `json:"people,omitempty"`
	CorporateEvents     []CorporateEvent            `json:"corporate_events,omitempty"`
	TrackedAssets       []TrackedAsset              `json:"tracked_assets,omitempty"`
	EvidenceChains      []EvidenceChain             `json:"evidence_chains,omitempty"`
	HumanReviewQueue    []ReviewItem                `json:"human_review_queue,omitempty"`
	FundamentalAnalysis FundamentalAnalysis         `json:"fundamental_analysis"`
	CanonicalFinancials *CanonicalFinancialsSummary `json:"canonical_financials,omitempty"`
	Warnings            []string                    `json:"warnings,omitempty"`
	OutputPath          string                      `json:"output_path,omitempty"`
}

type CanonicalFinancialsSummary struct {
	Path          string   `json:"path,omitempty"`
	FactsRead     int      `json:"facts_read"`
	FactsAccepted int      `json:"facts_accepted"`
	FactsRejected int      `json:"facts_rejected"`
	Fields        int      `json:"fields"`
	Periods       int      `json:"periods"`
	Warnings      []string `json:"warnings,omitempty"`
}

type BatchExtractionResult struct {
	GeneratedAt             time.Time          `json:"generated_at"`
	Tickers                 int                `json:"tickers"`
	DocumentsScanned        int                `json:"documents_scanned"`
	DocumentsProcessed      int                `json:"documents_processed"`
	FinancialFacts          int                `json:"financial_facts"`
	CanonicalFinancials     int                `json:"canonical_financials"`
	ReviewItems             int                `json:"review_items"`
	OutputPaths             []string           `json:"output_paths,omitempty"`
	CanonicalFinancialPaths []string           `json:"canonical_financial_paths,omitempty"`
	Results                 []ExtractionResult `json:"results,omitempty"`
}
