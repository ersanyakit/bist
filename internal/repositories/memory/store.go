package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"hissebot/internal/domain/disclosures"
	"hissebot/internal/domain/documents"
	"hissebot/internal/domain/financials"
	"hissebot/internal/domain/macro"
	"hissebot/internal/domain/marketdata"
	"hissebot/internal/domain/stocks"
	"hissebot/internal/repositories"
)

type Store struct {
	mu          sync.RWMutex
	ohlcv       map[string][]marketdata.OHLCV
	adjusted    map[string][]marketdata.OHLCV
	statements  map[string]financials.Statements
	ratios      map[string]financials.RatioSet
	disclosures map[string][]disclosures.Disclosure
	macroSeries map[macro.SeriesID]macro.Series
	stocks      map[string]stocks.Stock
	analysis    []repositories.AnalysisResultRecord
	documents   map[string]documents.DocumentMetadata
	jobs        map[string]documents.IngestionJob
	errors      []documents.IngestionError
}

func New() *Store {
	return &Store{
		ohlcv:       map[string][]marketdata.OHLCV{},
		adjusted:    map[string][]marketdata.OHLCV{},
		statements:  map[string]financials.Statements{},
		ratios:      map[string]financials.RatioSet{},
		disclosures: map[string][]disclosures.Disclosure{},
		macroSeries: map[macro.SeriesID]macro.Series{},
		stocks:      map[string]stocks.Stock{},
		documents:   map[string]documents.DocumentMetadata{},
		jobs:        map[string]documents.IngestionJob{},
	}
}

func (s *Store) SaveOHLCV(_ context.Context, candles []marketdata.OHLCV) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, candle := range candles {
		key := priceKey(candle.Symbol, candle.Timeframe)
		s.ohlcv[key] = upsertCandle(s.ohlcv[key], candle)
	}
	return nil
}

func (s *Store) ListOHLCV(_ context.Context, symbol string, timeframe marketdata.Timeframe, from time.Time, to time.Time) ([]marketdata.OHLCV, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return filterCandles(s.ohlcv[priceKey(symbol, timeframe)], from, to), nil
}

func (s *Store) SaveAdjustedOHLCV(_ context.Context, candles []marketdata.OHLCV) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, candle := range candles {
		key := priceKey(candle.Symbol, candle.Timeframe)
		s.adjusted[key] = upsertCandle(s.adjusted[key], candle)
	}
	return nil
}

func (s *Store) SaveStatements(_ context.Context, statements financials.Statements) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statements[statementKey(statements.Symbol, statements.Period)] = statements
	return nil
}

func (s *Store) GetStatements(_ context.Context, symbol string, period financials.Period) (financials.Statements, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.statements[statementKey(symbol, period)]
	if !ok {
		return financials.Statements{}, fmt.Errorf("statements %s: not found", statementKey(symbol, period))
	}
	return value, nil
}

func (s *Store) SaveRatios(_ context.Context, ratios financials.RatioSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ratios[statementKey(ratios.Symbol, ratios.Period)] = ratios
	return nil
}

func (s *Store) SaveDisclosures(_ context.Context, items []disclosures.Disclosure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range items {
		key := strings.ToUpper(item.Symbol)
		s.disclosures[key] = append(s.disclosures[key], item)
	}
	return nil
}

func (s *Store) ListDisclosures(_ context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.Disclosure, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.disclosures[strings.ToUpper(symbol)]
	out := make([]disclosures.Disclosure, 0, len(items))
	for _, item := range items {
		if inRange(item.PublishedAt, from, to) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.Before(out[j].PublishedAt) })
	return out, nil
}

func (s *Store) SaveSeries(_ context.Context, series macro.Series) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.macroSeries[series.ID] = series
	return nil
}

func (s *Store) GetSeries(_ context.Context, id macro.SeriesID, from time.Time, to time.Time) (macro.Series, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	series, ok := s.macroSeries[id]
	if !ok {
		return macro.Series{}, fmt.Errorf("macro series %s: not found", id)
	}
	series.Observations = filterObservations(series.Observations, from, to)
	return series, nil
}

func (s *Store) SaveStock(_ context.Context, stock stocks.Stock) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stocks[strings.ToUpper(stock.Ticker)] = stock
	return nil
}

func (s *Store) GetStock(_ context.Context, symbol string) (stocks.Stock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stock, ok := s.stocks[strings.ToUpper(symbol)]
	if !ok {
		return stocks.Stock{}, fmt.Errorf("stock %s: not found", symbol)
	}
	return stock, nil
}

func (s *Store) SaveAnalysisResult(_ context.Context, result repositories.AnalysisResultRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.analysis = append(s.analysis, result)
	return nil
}

func (s *Store) SaveDocument(_ context.Context, document documents.DocumentMetadata) error {
	if err := document.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, existing := range s.documents {
		if sameDocumentVersionGroup(existing, document) && existing.DocumentID != document.DocumentID && existing.IsLatestVersion {
			existing.IsLatestVersion = false
			existing.UpdatedAt = document.UpdatedAt
			s.documents[id] = existing
		}
	}
	s.documents[document.DocumentID] = document
	return nil
}

func (s *Store) FindDocumentBySource(_ context.Context, sourceSystem documents.SourceSystem, ticker string, disclosureID string, localFilePath string) (documents.DocumentMetadata, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ticker = documents.NormalizeTicker(ticker)
	var latest documents.DocumentMetadata
	found := false
	for _, document := range s.documents {
		if document.SourceSystem == sourceSystem && document.Ticker == ticker && document.DisclosureID == disclosureID && document.LocalFilePath == localFilePath {
			if !found || document.Version > latest.Version || (document.Version == latest.Version && document.UpdatedAt.After(latest.UpdatedAt)) {
				latest = document
				found = true
			}
		}
	}
	return latest, found, nil
}

func (s *Store) LatestDocumentVersion(_ context.Context, sourceSystem documents.SourceSystem, ticker string, disclosureID string, originalFilename string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ticker = documents.NormalizeTicker(ticker)
	latest := 0
	for _, document := range s.documents {
		if document.SourceSystem == sourceSystem && document.Ticker == ticker && document.DisclosureID == disclosureID && strings.EqualFold(document.OriginalFilename, originalFilename) {
			if document.Version > latest {
				latest = document.Version
			}
		}
	}
	return latest, nil
}

func (s *Store) ListDocuments(_ context.Context, ticker string) ([]documents.DocumentMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ticker = documents.NormalizeTicker(ticker)
	out := make([]documents.DocumentMetadata, 0, len(s.documents))
	for _, document := range s.documents {
		if ticker == "" || document.Ticker == ticker {
			out = append(out, document)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ticker != out[j].Ticker {
			return out[i].Ticker < out[j].Ticker
		}
		if !out[i].DisclosureDate.Equal(out[j].DisclosureDate) {
			return out[i].DisclosureDate.Before(out[j].DisclosureDate)
		}
		return out[i].OriginalFilename < out[j].OriginalFilename
	})
	return out, nil
}

func (s *Store) SaveIngestionJob(_ context.Context, job documents.IngestionJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.JobID] = job
	return nil
}

func (s *Store) UpdateIngestionJob(_ context.Context, job documents.IngestionJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.JobID] = job
	return nil
}

func (s *Store) SaveIngestionError(_ context.Context, item documents.IngestionError) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, item)
	return nil
}

func sameDocumentVersionGroup(a, b documents.DocumentMetadata) bool {
	return a.SourceSystem == b.SourceSystem &&
		a.Ticker == b.Ticker &&
		a.DisclosureID == b.DisclosureID &&
		strings.EqualFold(a.OriginalFilename, b.OriginalFilename)
}

func priceKey(symbol string, timeframe marketdata.Timeframe) string {
	return strings.ToUpper(symbol) + ":" + string(timeframe)
}

func statementKey(symbol string, period financials.Period) string {
	return fmt.Sprintf("%s:%d:%d:%s", strings.ToUpper(symbol), period.Year, period.Quarter, period.Type)
}

func upsertCandle(items []marketdata.OHLCV, candle marketdata.OHLCV) []marketdata.OHLCV {
	for i, item := range items {
		if item.Timestamp.Equal(candle.Timestamp) {
			items[i] = candle
			return items
		}
	}
	items = append(items, candle)
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp.Before(items[j].Timestamp) })
	return items
}

func filterCandles(items []marketdata.OHLCV, from time.Time, to time.Time) []marketdata.OHLCV {
	out := make([]marketdata.OHLCV, 0, len(items))
	for _, item := range items {
		if inRange(item.Timestamp, from, to) {
			out = append(out, item)
		}
	}
	return out
}

func filterObservations(items []macro.Observation, from time.Time, to time.Time) []macro.Observation {
	out := make([]macro.Observation, 0, len(items))
	for _, item := range items {
		if inRange(item.Date, from, to) {
			out = append(out, item)
		}
	}
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
