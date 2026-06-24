package datasource

import (
	"context"
	"fmt"

	"hissebot/internal/ta/ohlcv"
)

type FallbackProvider struct {
	primary  MarketDataProvider
	fallback MarketDataProvider
}

func NewFallbackProvider(primary MarketDataProvider, fallback MarketDataProvider) *FallbackProvider {
	return &FallbackProvider{primary: primary, fallback: fallback}
}

func (p *FallbackProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	if p == nil || p.primary == nil {
		if p != nil && p.fallback != nil {
			return p.fallback.SearchSymbol(ctx, symbol)
		}
		return ohlcv.Instrument{}, fmt.Errorf("market data provider missing: %w", ErrSymbolNotFound)
	}
	instrument, err := p.primary.SearchSymbol(ctx, symbol)
	if err == nil {
		return instrument, nil
	}
	if p.fallback == nil {
		return ohlcv.Instrument{}, err
	}
	fallbackInstrument, fallbackErr := p.fallback.SearchSymbol(ctx, symbol)
	if fallbackErr != nil {
		return ohlcv.Instrument{}, fmt.Errorf("primary search failed: %v; fallback search failed: %w", err, fallbackErr)
	}
	return fallbackInstrument, nil
}

func (p *FallbackProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if p == nil || p.primary == nil {
		if p != nil && p.fallback != nil {
			return p.fallback.FetchOHLCV(ctx, instrument, timeframe, limit)
		}
		return nil, fmt.Errorf("market data provider missing: %w", ErrSymbolNotFound)
	}
	candles, err := p.primary.FetchOHLCV(ctx, instrument, timeframe, limit)
	if err == nil {
		return candles, nil
	}
	if p.fallback == nil {
		return nil, err
	}
	fallbackCandles, fallbackErr := p.fallback.FetchOHLCV(ctx, instrument, timeframe, limit)
	if fallbackErr != nil {
		return nil, fmt.Errorf("primary fetch failed: %v; fallback fetch failed: %w", err, fallbackErr)
	}
	return fallbackCandles, nil
}

func (p *FallbackProvider) FetchDailyBulletinRecords(ctx context.Context, symbol string, limit int) ([]DailyBulletinRecord, error) {
	if p == nil {
		return nil, fmt.Errorf("market data provider missing: %w", ErrSymbolNotFound)
	}
	if provider, ok := p.primary.(BulletinRecordProvider); ok {
		records, err := provider.FetchDailyBulletinRecords(ctx, symbol, limit)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if p.fallback == nil {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("primary bulletin records empty for %s: %w", symbol, ErrSymbolNotFound)
		}
	}
	if provider, ok := p.fallback.(BulletinRecordProvider); ok {
		return provider.FetchDailyBulletinRecords(ctx, symbol, limit)
	}
	return nil, fmt.Errorf("bulletin record provider missing: %w", ErrSymbolNotFound)
}

func (p *FallbackProvider) FetchDailyBulletinRecordsRange(ctx context.Context, symbol, fromDate, toDate string, limit int) ([]DailyBulletinRecord, error) {
	if p == nil {
		return nil, fmt.Errorf("market data provider missing: %w", ErrSymbolNotFound)
	}
	if provider, ok := p.primary.(BulletinRecordRangeProvider); ok {
		records, err := provider.FetchDailyBulletinRecordsRange(ctx, symbol, fromDate, toDate, limit)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if p.fallback == nil {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("primary bulletin range records empty for %s: %w", symbol, ErrSymbolNotFound)
		}
	}
	if provider, ok := p.fallback.(BulletinRecordRangeProvider); ok {
		return provider.FetchDailyBulletinRecordsRange(ctx, symbol, fromDate, toDate, limit)
	}
	return p.FetchDailyBulletinRecords(ctx, symbol, limit)
}
