// internal/datasource/csv.go
package datasource

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"
)

var (
	ErrSymbolNotFound = errors.New("symbol not found")
	ErrInvalidCSV     = errors.New("invalid csv")
	ErrTimeframe      = errors.New("unsupported timeframe")
)

type CSVProvider struct {
	dataDir string
}

func NewCSVProvider(dataDir string) *CSVProvider {
	return &CSVProvider{dataDir: dataDir}
}

func (p *CSVProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	if err := ctx.Err(); err != nil {
		return ohlcv.Instrument{}, fmt.Errorf("search csv symbol canceled: %w", err)
	}
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return ohlcv.Instrument{}, fmt.Errorf("empty symbol: %w", ErrSymbolNotFound)
	}
	assetType := ohlcv.InferAssetTypeFromSymbol(normalized)
	currency := "TRY"
	companyName := normalized
	exchange := "BIST"
	if assetType == ohlcv.AssetTypeCommodity {
		if instrument, ok := ohlcv.CanonicalCommodityInstrument(normalized); ok {
			normalized = instrument.Symbol
			exchange = instrument.Exchange
			currency = instrument.Currency
			companyName = instrument.CompanyName
		}
	} else if assetType == ohlcv.AssetTypeCrypto {
		if pair, quote, ok := ohlcv.CanonicalCryptoPair(normalized); ok {
			normalized = pair
			currency = quote
			companyName = ohlcv.CryptoDisplayName(pair)
		}
	}
	return ohlcv.Instrument{Symbol: normalized, Exchange: exchange, CompanyName: companyName, Currency: currency, AssetType: assetType}, nil
}

func (p *CSVProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if err := validateTimeframe(timeframe); err != nil {
		return nil, fmt.Errorf("validate timeframe: %w", err)
	}
	path := filepath.Join(p.dataDir, fmt.Sprintf("%s_%s.csv", instrument.Symbol, timeframe))
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	index := map[string]int{}
	for i, name := range header {
		index[strings.ToLower(strings.TrimSpace(name))] = i
	}
	required := []string{"time", "open", "high", "low", "close", "volume"}
	for _, name := range required {
		if _, ok := index[name]; !ok {
			return nil, fmt.Errorf("missing csv column %s: %w", name, ErrInvalidCSV)
		}
	}

	var candles []ohlcv.Candle
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("read csv canceled: %w", err)
		}
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read csv row: %w", err)
		}
		candle, err := parseCandle(record, index)
		if err != nil {
			return nil, fmt.Errorf("parse csv candle: %w", err)
		}
		candles = append(candles, candle)
	}
	if limit > 0 && len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	return candles, nil
}

func validateTimeframe(timeframe string) error {
	switch timeframe {
	case "1D", "1W", "1M", "3M", "6M", "1Y", "YTD", "ALL":
		return nil
	default:
		return fmt.Errorf("timeframe %q: %w", timeframe, ErrTimeframe)
	}
}

func parseCandle(record []string, index map[string]int) (ohlcv.Candle, error) {
	t, err := parseTime(value(record, index, "time"))
	if err != nil {
		return ohlcv.Candle{}, fmt.Errorf("parse time: %w", err)
	}
	open, err := parseFloat(value(record, index, "open"))
	if err != nil {
		return ohlcv.Candle{}, fmt.Errorf("parse open: %w", err)
	}
	high, err := parseFloat(value(record, index, "high"))
	if err != nil {
		return ohlcv.Candle{}, fmt.Errorf("parse high: %w", err)
	}
	low, err := parseFloat(value(record, index, "low"))
	if err != nil {
		return ohlcv.Candle{}, fmt.Errorf("parse low: %w", err)
	}
	closePrice, err := parseFloat(value(record, index, "close"))
	if err != nil {
		return ohlcv.Candle{}, fmt.Errorf("parse close: %w", err)
	}
	volume, err := parseFloat(value(record, index, "volume"))
	if err != nil {
		return ohlcv.Candle{}, fmt.Errorf("parse volume: %w", err)
	}
	adjustedOpen := optionalFloat(record, index, "adjusted_open", open)
	adjustedHigh := optionalFloat(record, index, "adjusted_high", high)
	adjustedLow := optionalFloat(record, index, "adjusted_low", low)
	adjustedClose := optionalFloat(record, index, "adjusted_close", closePrice)
	adjustedVolume := optionalFloat(record, index, "adjusted_volume", volume)
	return ohlcv.Candle{
		Time:           t,
		Open:           open,
		High:           high,
		Low:            low,
		Close:          closePrice,
		Volume:         volume,
		AdjustedOpen:   adjustedOpen,
		AdjustedHigh:   adjustedHigh,
		AdjustedLow:    adjustedLow,
		AdjustedClose:  adjustedClose,
		AdjustedVolume: adjustedVolume,
		IsAdjusted:     true,
	}, nil
}

func value(record []string, index map[string]int, name string) string {
	i, ok := index[name]
	if !ok || i < 0 || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

func parseFloat(raw string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("parse float %q: %w", raw, err)
	}
	return f, nil
}

func optionalFloat(record []string, index map[string]int, name string, fallback float64) float64 {
	raw := value(record, index, name)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseTime(raw string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q: %w", raw, ErrInvalidCSV)
}
