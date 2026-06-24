package datasources

import (
	"context"
	"errors"
	"time"

	"hissebot/internal/domain/disclosures"
	"hissebot/internal/domain/financials"
	"hissebot/internal/domain/macro"
	"hissebot/internal/domain/marketdata"
	"hissebot/internal/domain/stocks"
)

var (
	ErrNotConfigured         = errors.New("data provider is not configured")
	ErrLicensedDataRequired  = errors.New("licensed/professional market data is required")
	ErrUnsupportedCapability = errors.New("provider does not support requested capability")
)

type ProviderInfo struct {
	Name         string   `json:"name"`
	SourceURL    string   `json:"source_url,omitempty"`
	License      string   `json:"license,omitempty"`
	RequiresKey  bool     `json:"requires_key,omitempty"`
	Capabilities []string `json:"capabilities"`
}

type MarketDataProvider interface {
	Info() ProviderInfo
	GetOHLCV(ctx context.Context, symbol string, timeframe string, from time.Time, to time.Time) ([]marketdata.OHLCV, error)
	GetLatestPrice(ctx context.Context, symbol string) (marketdata.PriceSnapshot, error)
	GetIndexOHLCV(ctx context.Context, indexCode string, timeframe string, from time.Time, to time.Time) ([]marketdata.OHLCV, error)
	GetSectorIndexOHLCV(ctx context.Context, sectorCode string, timeframe string, from time.Time, to time.Time) ([]marketdata.OHLCV, error)
	GetOrderBook(ctx context.Context, symbol string) (marketdata.OrderBook, error)
}

type FinancialStatementProvider interface {
	Info() ProviderInfo
	GetBalanceSheet(ctx context.Context, symbol string, period financials.Period) (financials.BalanceSheet, error)
	GetIncomeStatement(ctx context.Context, symbol string, period financials.Period) (financials.IncomeStatement, error)
	GetCashFlowStatement(ctx context.Context, symbol string, period financials.Period) (financials.CashFlowStatement, error)
	GetEquityStatement(ctx context.Context, symbol string, period financials.Period) (financials.EquityStatement, error)
	GetFinancialNotes(ctx context.Context, symbol string, period financials.Period) ([]financials.FinancialNote, error)
}

type DisclosureProvider interface {
	Info() ProviderInfo
	GetDisclosures(ctx context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.Disclosure, error)
	GetMaterialEvents(ctx context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.MaterialEvent, error)
}

type MacroDataProvider interface {
	Info() ProviderInfo
	GetPolicyRate(ctx context.Context, from time.Time, to time.Time) ([]macro.Observation, error)
	GetFXRate(ctx context.Context, pair string, from time.Time, to time.Time) ([]macro.Observation, error)
	GetInflation(ctx context.Context, series string, from time.Time, to time.Time) ([]macro.Observation, error)
	GetGDPGrowth(ctx context.Context, from time.Time, to time.Time) ([]macro.Observation, error)
	GetIndustrialProduction(ctx context.Context, from time.Time, to time.Time) ([]macro.Observation, error)
}

type CorporateActionProvider interface {
	Info() ProviderInfo
	GetCorporateActions(ctx context.Context, symbol string, from time.Time, to time.Time) ([]marketdata.CorporateAction, error)
}

type CompanyReferenceDataProvider interface {
	Info() ProviderInfo
	GetStock(ctx context.Context, symbol string) (stocks.Stock, error)
	GetReferenceData(ctx context.Context, symbol string) (stocks.ReferenceData, error)
}
