// internal/datasource/mock.go
package datasource

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"time"

	"hissebot/internal/ta/ohlcv"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	if err := ctx.Err(); err != nil {
		return ohlcv.Instrument{}, fmt.Errorf("search symbol canceled: %w", err)
	}
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return ohlcv.Instrument{}, fmt.Errorf("empty symbol: %w", ErrSymbolNotFound)
	}
	assetType := ohlcv.InferAssetTypeFromSymbol(normalized)
	currency := "TRY"
	companyName := normalized + " Anonim Sirketi"
	if assetType == ohlcv.AssetTypeCommodity {
		if instrument, ok := ohlcv.CanonicalCommodityInstrument(normalized); ok {
			return instrument, nil
		}
	} else if assetType == ohlcv.AssetTypeCrypto {
		if pair, quote, ok := ohlcv.CanonicalCryptoPair(normalized); ok {
			normalized = pair
			currency = quote
			companyName = ohlcv.CryptoDisplayName(pair)
		}
	}
	return ohlcv.Instrument{
		Symbol:      normalized,
		Exchange:    "BIST",
		CompanyName: companyName,
		Currency:    currency,
		AssetType:   assetType,
	}, nil
}

func (p *MockProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if err := validateTimeframe(timeframe); err != nil {
		return nil, fmt.Errorf("validate timeframe: %w", err)
	}
	if limit <= 0 {
		limit = 260
	}
	seed := int64(hashString(instrument.Symbol + ":" + timeframe))
	random := rand.New(rand.NewSource(seed))
	days := timeframeDays(timeframe)
	end := time.Now().UTC().Truncate(24 * time.Hour)
	base := 20 + float64(hashString(instrument.Symbol)%9000)/100
	trend := (float64(int(hashString(timeframe)%17)-8) / 1000) + 0.0015
	volatility := 0.012 + float64(hashString(instrument.Symbol+timeframe)%20)/1000
	candles := make([]ohlcv.Candle, 0, limit)
	closePrice := base
	for i := 0; i < limit; i++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("fetch mock ohlcv canceled: %w", err)
		}
		wave := math.Sin(float64(i)/9.0+float64(seed%19)) * volatility * 2
		shock := (random.Float64() - 0.5) * volatility
		open := closePrice
		closePrice = math.Max(0.5, closePrice*(1+trend+wave+shock))
		span := closePrice * (volatility*0.8 + random.Float64()*volatility)
		high := math.Max(open, closePrice) + span
		low := math.Max(0.1, math.Min(open, closePrice)-span)
		volume := 500000 + float64(hashString(instrument.Symbol)%1000000)
		volume *= 1 + 0.35*math.Sin(float64(i)/6.0) + random.Float64()*0.25
		adjustment := 1.0
		if i < limit/3 {
			adjustment = 0.985
		}
		candles = append(candles, ohlcv.Candle{
			Time:           end.AddDate(0, 0, -days*(limit-1-i)),
			Open:           open,
			High:           high,
			Low:            low,
			Close:          closePrice,
			Volume:         volume,
			AdjustedOpen:   open * adjustment,
			AdjustedHigh:   high * adjustment,
			AdjustedLow:    low * adjustment,
			AdjustedClose:  closePrice * adjustment,
			AdjustedVolume: volume / adjustment,
			IsAdjusted:     true,
		})
	}
	return candles, nil
}

func hashString(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func timeframeDays(timeframe string) int {
	switch timeframe {
	case "1D", "YTD", "ALL":
		return 1
	case "1W":
		return 7
	case "1M":
		return 30
	case "3M":
		return 91
	case "6M":
		return 182
	case "1Y":
		return 365
	default:
		return 1
	}
}
