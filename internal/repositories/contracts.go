package repositories

import (
	"context"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/domain/disclosures"
	"hissebot/internal/domain/documents"
	"hissebot/internal/domain/financials"
	"hissebot/internal/domain/macro"
	"hissebot/internal/domain/marketdata"
	"hissebot/internal/domain/stocks"
)

type PriceRepository interface {
	SaveOHLCV(ctx context.Context, candles []marketdata.OHLCV) error
	ListOHLCV(ctx context.Context, symbol string, timeframe marketdata.Timeframe, from time.Time, to time.Time) ([]marketdata.OHLCV, error)
	SaveAdjustedOHLCV(ctx context.Context, candles []marketdata.OHLCV) error
}

type EquityRepository interface {
	Load(ticker string) (*domain.Equity, error)
	Save(equity *domain.Equity) error
	List() ([]*domain.Equity, error)
}

type FinancialStatementRepository interface {
	SaveStatements(ctx context.Context, statements financials.Statements) error
	GetStatements(ctx context.Context, symbol string, period financials.Period) (financials.Statements, error)
	SaveRatios(ctx context.Context, ratios financials.RatioSet) error
}

type DisclosureRepository interface {
	SaveDisclosures(ctx context.Context, items []disclosures.Disclosure) error
	ListDisclosures(ctx context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.Disclosure, error)
}

type MacroRepository interface {
	SaveSeries(ctx context.Context, series macro.Series) error
	GetSeries(ctx context.Context, id macro.SeriesID, from time.Time, to time.Time) (macro.Series, error)
}

type StockRepository interface {
	SaveStock(ctx context.Context, stock stocks.Stock) error
	GetStock(ctx context.Context, symbol string) (stocks.Stock, error)
}

type AnalysisRepository interface {
	SaveAnalysisResult(ctx context.Context, result AnalysisResultRecord) error
}

type DocumentRepository interface {
	SaveDocument(ctx context.Context, document documents.DocumentMetadata) error
	FindDocumentBySource(ctx context.Context, sourceSystem documents.SourceSystem, ticker string, disclosureID string, localFilePath string) (documents.DocumentMetadata, bool, error)
	LatestDocumentVersion(ctx context.Context, sourceSystem documents.SourceSystem, ticker string, disclosureID string, originalFilename string) (int, error)
	ListDocuments(ctx context.Context, ticker string) ([]documents.DocumentMetadata, error)
	SaveIngestionJob(ctx context.Context, job documents.IngestionJob) error
	UpdateIngestionJob(ctx context.Context, job documents.IngestionJob) error
	SaveIngestionError(ctx context.Context, item documents.IngestionError) error
}

type AnalysisResultRecord struct {
	Symbol       string         `json:"symbol"`
	AnalysisDate time.Time      `json:"analysis_date"`
	DataVersion  string         `json:"data_version"`
	Payload      map[string]any `json:"payload"`
}
