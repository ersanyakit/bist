package mock

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"hissebot/internal/datasources"
	"hissebot/internal/domain/disclosures"
	"hissebot/internal/domain/financials"
	"hissebot/internal/domain/macro"
	"hissebot/internal/domain/marketdata"
	"hissebot/internal/domain/stocks"
)

type Provider struct {
	OHLCV            map[string][]marketdata.OHLCV
	Latest           map[string]marketdata.PriceSnapshot
	IndexOHLCV       map[string][]marketdata.OHLCV
	SectorIndexOHLCV map[string][]marketdata.OHLCV
	OrderBooks       map[string]marketdata.OrderBook
	CorporateActions map[string][]marketdata.CorporateAction
	Stocks           map[string]stocks.Stock
	ReferenceData    map[string]stocks.ReferenceData
	Financials       map[string]financials.Statements
	Disclosures      map[string][]disclosures.Disclosure
	MaterialEvents   map[string][]disclosures.MaterialEvent
	MacroSeries      map[macro.SeriesID][]macro.Observation
}

func (p Provider) Info() datasources.ProviderInfo {
	return datasources.ProviderInfo{Name: "mock", Capabilities: []string{"market_data", "financials", "disclosures", "macro", "corporate_actions", "reference_data"}}
}

func (p Provider) GetOHLCV(_ context.Context, symbol string, _ string, from time.Time, to time.Time) ([]marketdata.OHLCV, error) {
	return filterOHLCV(p.OHLCV[strings.ToUpper(symbol)], from, to), nil
}

func (p Provider) GetLatestPrice(_ context.Context, symbol string) (marketdata.PriceSnapshot, error) {
	value, ok := p.Latest[strings.ToUpper(symbol)]
	if !ok {
		return marketdata.PriceSnapshot{}, fmt.Errorf("mock latest price %s: %w", symbol, datasources.ErrNotConfigured)
	}
	return value, nil
}

func (p Provider) GetIndexOHLCV(_ context.Context, indexCode string, _ string, from time.Time, to time.Time) ([]marketdata.OHLCV, error) {
	return filterOHLCV(p.IndexOHLCV[strings.ToUpper(indexCode)], from, to), nil
}

func (p Provider) GetSectorIndexOHLCV(_ context.Context, sectorCode string, _ string, from time.Time, to time.Time) ([]marketdata.OHLCV, error) {
	return filterOHLCV(p.SectorIndexOHLCV[strings.ToUpper(sectorCode)], from, to), nil
}

func (p Provider) GetOrderBook(_ context.Context, symbol string) (marketdata.OrderBook, error) {
	book, ok := p.OrderBooks[strings.ToUpper(symbol)]
	if !ok {
		return marketdata.OrderBook{}, fmt.Errorf("mock order book %s: %w", symbol, datasources.ErrNotConfigured)
	}
	return book, nil
}

func (p Provider) GetCorporateActions(_ context.Context, symbol string, from time.Time, to time.Time) ([]marketdata.CorporateAction, error) {
	actions := p.CorporateActions[strings.ToUpper(symbol)]
	out := make([]marketdata.CorporateAction, 0, len(actions))
	for _, action := range actions {
		if inRange(action.ExDate, from, to) {
			out = append(out, action)
		}
	}
	return out, nil
}

func (p Provider) GetStock(_ context.Context, symbol string) (stocks.Stock, error) {
	stock, ok := p.Stocks[strings.ToUpper(symbol)]
	if !ok {
		return stocks.Stock{}, fmt.Errorf("mock stock %s: %w", symbol, datasources.ErrNotConfigured)
	}
	return stock, nil
}

func (p Provider) GetReferenceData(_ context.Context, symbol string) (stocks.ReferenceData, error) {
	ref, ok := p.ReferenceData[strings.ToUpper(symbol)]
	if !ok {
		stock, err := p.GetStock(context.Background(), symbol)
		if err != nil {
			return stocks.ReferenceData{}, err
		}
		return stocks.ReferenceData{Stock: stock}, nil
	}
	return ref, nil
}

func (p Provider) GetBalanceSheet(_ context.Context, symbol string, period financials.Period) (financials.BalanceSheet, error) {
	statements, err := p.statements(symbol, period)
	return statements.BalanceSheet, err
}

func (p Provider) GetIncomeStatement(_ context.Context, symbol string, period financials.Period) (financials.IncomeStatement, error) {
	statements, err := p.statements(symbol, period)
	return statements.IncomeStatement, err
}

func (p Provider) GetCashFlowStatement(_ context.Context, symbol string, period financials.Period) (financials.CashFlowStatement, error) {
	statements, err := p.statements(symbol, period)
	return statements.CashFlow, err
}

func (p Provider) GetEquityStatement(_ context.Context, symbol string, period financials.Period) (financials.EquityStatement, error) {
	statements, err := p.statements(symbol, period)
	return statements.EquityStatement, err
}

func (p Provider) GetFinancialNotes(_ context.Context, symbol string, period financials.Period) ([]financials.FinancialNote, error) {
	statements, err := p.statements(symbol, period)
	return statements.Notes, err
}

func (p Provider) GetDisclosures(_ context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.Disclosure, error) {
	items := p.Disclosures[strings.ToUpper(symbol)]
	out := make([]disclosures.Disclosure, 0, len(items))
	for _, item := range items {
		if inRange(item.PublishedAt, from, to) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (p Provider) GetMaterialEvents(_ context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.MaterialEvent, error) {
	items := p.MaterialEvents[strings.ToUpper(symbol)]
	out := make([]disclosures.MaterialEvent, 0, len(items))
	for _, item := range items {
		if inRange(item.PublishedAt, from, to) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (p Provider) GetPolicyRate(_ context.Context, from time.Time, to time.Time) ([]macro.Observation, error) {
	return filterMacro(p.MacroSeries[macro.SeriesPolicyRate], from, to), nil
}

func (p Provider) GetFXRate(_ context.Context, pair string, from time.Time, to time.Time) ([]macro.Observation, error) {
	id := macro.SeriesID(strings.ToLower(strings.ReplaceAll(pair, "/", "_")))
	return filterMacro(p.MacroSeries[id], from, to), nil
}

func (p Provider) GetInflation(_ context.Context, series string, from time.Time, to time.Time) ([]macro.Observation, error) {
	return filterMacro(p.MacroSeries[macro.SeriesID(series)], from, to), nil
}

func (p Provider) GetGDPGrowth(_ context.Context, from time.Time, to time.Time) ([]macro.Observation, error) {
	return filterMacro(p.MacroSeries[macro.SeriesGDPGrowth], from, to), nil
}

func (p Provider) GetIndustrialProduction(_ context.Context, from time.Time, to time.Time) ([]macro.Observation, error) {
	return filterMacro(p.MacroSeries[macro.SeriesIndustrialProduction], from, to), nil
}

func (p Provider) statements(symbol string, period financials.Period) (financials.Statements, error) {
	key := StatementsKey(symbol, period)
	value, ok := p.Financials[key]
	if !ok {
		return financials.Statements{}, fmt.Errorf("mock financial statements %s: %w", key, datasources.ErrNotConfigured)
	}
	return value, nil
}

func StatementsKey(symbol string, period financials.Period) string {
	return fmt.Sprintf("%s:%d:%d:%s", strings.ToUpper(symbol), period.Year, period.Quarter, period.Type)
}

func filterOHLCV(items []marketdata.OHLCV, from time.Time, to time.Time) []marketdata.OHLCV {
	out := make([]marketdata.OHLCV, 0, len(items))
	for _, item := range items {
		if inRange(item.Timestamp, from, to) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}

func filterMacro(items []macro.Observation, from time.Time, to time.Time) []macro.Observation {
	out := make([]macro.Observation, 0, len(items))
	for _, item := range items {
		if inRange(item.Date, from, to) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

func inRange(ts time.Time, from time.Time, to time.Time) bool {
	if !from.IsZero() && ts.Before(from) {
		return false
	}
	if !to.IsZero() && ts.After(to) {
		return false
	}
	return true
}
