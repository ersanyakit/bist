package professional

import "hissebot/internal/kapingest"

type KAPRawDataBundle struct {
	Computed         bool                                `json:"computed"`
	Symbol           string                              `json:"symbol,omitempty"`
	SourceFiles      KAPRawDataSourceFiles               `json:"source_files"`
	Counts           KAPRawDataCounts                    `json:"counts"`
	RawDocuments     []kapingest.RawDocument             `json:"raw_documents,omitempty"`
	KAPEvents        []kapingest.KAPEvent                `json:"kap_events,omitempty"`
	AssetEvents      []kapingest.AssetEvent              `json:"asset_events,omitempty"`
	AssetInventory   *kapingest.AssetInventory           `json:"asset_inventory,omitempty"`
	DocumentIndex    *kapingest.DocumentIndex            `json:"document_index,omitempty"`
	KnowledgeGraph   *kapingest.CompanyKnowledgeGraph    `json:"company_knowledge_graph,omitempty"`
	DocumentFacts    []kapingest.DocumentFact            `json:"document_facts,omitempty"`
	FinancialFacts   []kapingest.ExtractedFinancialFact  `json:"financial_facts,omitempty"`
	FinancialTables  []kapingest.ExtractedFinancialTable `json:"financial_tables,omitempty"`
	People           []kapingest.ExtractedPerson         `json:"people,omitempty"`
	OwnershipFacts   []kapingest.OwnershipFact           `json:"ownership_facts,omitempty"`
	CorporateEvents  []kapingest.ExtractedCorporateEvent `json:"corporate_events,omitempty"`
	ExtractionErrors []kapingest.ExtractionError         `json:"extraction_errors,omitempty"`
	ProcessedFiles   []kapingest.ProcessedFile           `json:"processed_files,omitempty"`
	Warnings         []string                            `json:"warnings,omitempty"`
}

type KAPRawDataSourceFiles struct {
	RawDocumentsPath     string `json:"raw_documents_path,omitempty"`
	KAPEventsPath        string `json:"kap_events_path,omitempty"`
	AssetEventsPath      string `json:"asset_events_path,omitempty"`
	AssetInventoryPath   string `json:"asset_inventory_path,omitempty"`
	DocumentIndexPath    string `json:"document_index_path,omitempty"`
	KnowledgeGraphPath   string `json:"knowledge_graph_path,omitempty"`
	DocumentFactsPath    string `json:"document_facts_path,omitempty"`
	FinancialFactsPath   string `json:"financial_facts_path,omitempty"`
	FinancialTablesPath  string `json:"financial_tables_path,omitempty"`
	PeoplePath           string `json:"people_path,omitempty"`
	OwnershipFactsPath   string `json:"ownership_facts_path,omitempty"`
	CorporateEventsPath  string `json:"corporate_events_path,omitempty"`
	ExtractionErrorsPath string `json:"extraction_errors_path,omitempty"`
	ProcessedFilesPath   string `json:"processed_files_path,omitempty"`
}

type KAPRawDataCounts struct {
	RawDocuments     int `json:"raw_documents"`
	KAPEvents        int `json:"kap_events"`
	AssetEvents      int `json:"asset_events"`
	InventoryItems   int `json:"inventory_items"`
	KnowledgeGraph   int `json:"knowledge_graph"`
	DocumentFacts    int `json:"document_facts"`
	FinancialFacts   int `json:"financial_facts"`
	FinancialTables  int `json:"financial_tables"`
	People           int `json:"people"`
	OwnershipFacts   int `json:"ownership_facts"`
	CorporateEvents  int `json:"corporate_events"`
	ExtractionErrors int `json:"extraction_errors"`
	ProcessedFiles   int `json:"processed_files"`
}
