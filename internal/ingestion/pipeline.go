package ingestion

import (
	"context"
	"fmt"
	"time"

	"hissebot/internal/datasources"
	"hissebot/internal/domain/financials"
	"hissebot/internal/domain/macro"
	"hissebot/internal/domain/marketdata"
	"hissebot/internal/normalization"
	"hissebot/internal/repositories"
	"hissebot/internal/validation"
)

type Pipeline struct {
	MarketProvider    datasources.MarketDataProvider
	FinancialProvider datasources.FinancialStatementProvider
	MacroProvider     datasources.MacroDataProvider
	CorporateProvider datasources.CorporateActionProvider
	Prices            repositories.PriceRepository
	Financials        repositories.FinancialStatementRepository
	Macro             repositories.MacroRepository
}

type Result struct {
	Status           string            `json:"status"`
	SavedRecords     int               `json:"saved_records"`
	ValidationReport validation.Report `json:"validation_report"`
	MissingData      []string          `json:"missing_data,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
}

func (p Pipeline) IngestOHLCV(ctx context.Context, symbol string, timeframe marketdata.Timeframe, from time.Time, to time.Time) (Result, error) {
	if p.MarketProvider == nil || p.Prices == nil {
		return Result{Status: "fail", MissingData: []string{"market_provider_or_price_repository"}}, datasources.ErrNotConfigured
	}
	candles, err := p.MarketProvider.GetOHLCV(ctx, symbol, string(timeframe), from, to)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	candles = normalization.NormalizeOHLCV(candles)
	report := validation.ValidateOHLCVSeries(candles)
	if report.Status == "fail" {
		return Result{Status: "fail", ValidationReport: report}, fmt.Errorf("OHLCV validation failed for %s", symbol)
	}
	if err := p.Prices.SaveOHLCV(ctx, candles); err != nil {
		return Result{Status: "fail", ValidationReport: report}, err
	}
	return Result{Status: report.Status, SavedRecords: len(candles), ValidationReport: report}, nil
}

func (p Pipeline) IngestAdjustedOHLCV(ctx context.Context, symbol string, timeframe marketdata.Timeframe, from time.Time, to time.Time) (Result, error) {
	if p.CorporateProvider == nil {
		return Result{Status: "limited", MissingData: []string{"corporate_action_provider"}}, datasources.ErrNotConfigured
	}
	base, err := p.MarketProvider.GetOHLCV(ctx, symbol, string(timeframe), from, to)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	actions, err := p.CorporateProvider.GetCorporateActions(ctx, symbol, time.Time{}, to)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	adjusted := normalization.ApplyCorporateActions(base, actions)
	report := validation.ValidateOHLCVSeries(adjusted)
	if report.Status == "fail" {
		return Result{Status: "fail", ValidationReport: report}, fmt.Errorf("adjusted OHLCV validation failed for %s", symbol)
	}
	if err := p.Prices.SaveAdjustedOHLCV(ctx, adjusted); err != nil {
		return Result{Status: "fail", ValidationReport: report}, err
	}
	return Result{Status: report.Status, SavedRecords: len(adjusted), ValidationReport: report}, nil
}

func (p Pipeline) IngestFinancialStatements(ctx context.Context, symbol string, period financials.Period) (Result, error) {
	if p.FinancialProvider == nil || p.Financials == nil {
		return Result{Status: "fail", MissingData: []string{"financial_provider_or_repository"}}, datasources.ErrNotConfigured
	}
	bs, err := p.FinancialProvider.GetBalanceSheet(ctx, symbol, period)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	is, err := p.FinancialProvider.GetIncomeStatement(ctx, symbol, period)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	cf, err := p.FinancialProvider.GetCashFlowStatement(ctx, symbol, period)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	eq, err := p.FinancialProvider.GetEquityStatement(ctx, symbol, period)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	notes, err := p.FinancialProvider.GetFinancialNotes(ctx, symbol, period)
	if err != nil {
		notes = nil
	}
	statements := financials.Statements{
		Symbol:          normalization.NormalizeSymbol(symbol),
		Period:          period,
		BalanceSheet:    bs,
		IncomeStatement: is,
		CashFlow:        cf,
		EquityStatement: eq,
		Notes:           notes,
		Meta:            bs.Meta,
	}
	report := validation.ValidateFinancialStatements(statements, 1)
	if report.Status == "fail" {
		return Result{Status: "fail", ValidationReport: report}, fmt.Errorf("financial statement validation failed for %s", symbol)
	}
	if err := p.Financials.SaveStatements(ctx, statements); err != nil {
		return Result{Status: "fail", ValidationReport: report}, err
	}
	return Result{Status: report.Status, SavedRecords: 1, ValidationReport: report}, nil
}

func (p Pipeline) IngestGDPGrowth(ctx context.Context, from time.Time, to time.Time) (Result, error) {
	if p.MacroProvider == nil || p.Macro == nil {
		return Result{Status: "fail", MissingData: []string{"macro_provider_or_repository"}}, datasources.ErrNotConfigured
	}
	observations, err := p.MacroProvider.GetGDPGrowth(ctx, from, to)
	if err != nil {
		return Result{Status: "fail"}, err
	}
	series := macro.Series{
		ID:           macro.SeriesGDPGrowth,
		Name:         "GSYH büyümesi",
		Frequency:    macro.FrequencyAnnual,
		Unit:         "pct_yoy",
		Observations: observations,
	}
	report := validation.ValidateMacroSeries(series)
	if report.Status == "fail" {
		return Result{Status: "fail", ValidationReport: report}, fmt.Errorf("macro series validation failed: %s", series.ID)
	}
	if err := p.Macro.SaveSeries(ctx, series); err != nil {
		return Result{Status: "fail", ValidationReport: report}, err
	}
	return Result{Status: report.Status, SavedRecords: len(observations), ValidationReport: report}, nil
}
