package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"hissebot/internal/services/bistbulletindb"
	"hissebot/internal/ta/ohlcv"

	_ "github.com/mattn/go-sqlite3"
)

// BISTBulletinDBProvider reads official BIST THB bulletin OHLCV rows from the
// local SQLite index produced by `sync bist-bulletin-db`.
type BISTBulletinDBProvider struct {
	dbPath string
}

func NewBISTBulletinDBProvider(dbPath string) *BISTBulletinDBProvider {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = bistbulletindb.DefaultDBPath
	}
	return &BISTBulletinDBProvider{dbPath: dbPath}
}

func (p *BISTBulletinDBProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return ohlcv.Instrument{}, fmt.Errorf("empty symbol: %w", ErrSymbolNotFound)
	}
	db, err := p.open()
	if err != nil {
		return ohlcv.Instrument{}, err
	}
	defer db.Close()

	var companyName string
	err = db.QueryRowContext(ctx, `
SELECT COALESCE(NULLIF(company_name, ''), symbol)
FROM instruments
WHERE symbol = ?`, normalized).Scan(&companyName)
	if err == sql.ErrNoRows {
		return ohlcv.Instrument{}, fmt.Errorf("bist bulletin db: no instrument for %s: %w", normalized, ErrSymbolNotFound)
	}
	if err != nil {
		return ohlcv.Instrument{}, fmt.Errorf("bist bulletin db search %s: %w", normalized, err)
	}
	return ohlcv.Instrument{
		Symbol:      normalized,
		Exchange:    "BIST",
		CompanyName: companyName,
		Currency:    "TRY",
		AssetType:   ohlcv.AssetTypeEquity,
	}, nil
}

func (p *BISTBulletinDBProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if ohlcv.IsCryptoAssetType(instrument.AssetType) || ohlcv.IsCommodityAssetType(instrument.AssetType) {
		return nil, fmt.Errorf("bist bulletin db only supports equity: %w", ErrSymbolNotFound)
	}
	records, err := p.FetchDailyBulletinRecords(ctx, instrument.Symbol, 0)
	if err != nil {
		return nil, err
	}
	daily := make([]ohlcv.Candle, 0, len(records))
	for _, r := range records {
		t, err := time.Parse("2006-01-02", r.TradingDate)
		if err != nil {
			continue
		}
		open := r.Open
		if open <= 0 {
			open = r.PreviousClose
		}
		if open <= 0 {
			open = r.Close
		}
		daily = append(daily, ohlcv.Candle{
			Time:           t,
			Open:           open,
			High:           r.High,
			Low:            r.Low,
			Close:          r.Close,
			Volume:         r.Volume,
			AdjustedOpen:   open,
			AdjustedHigh:   r.High,
			AdjustedLow:    r.Low,
			AdjustedClose:  r.Close,
			AdjustedVolume: r.Volume,
			IsAdjusted:     false,
		})
	}
	candles := aggregateBulletinCandles(daily, timeframe)
	if limit > 0 && len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("bist bulletin db: empty result for %s %s: %w", instrument.Symbol, timeframe, ErrSymbolNotFound)
	}
	return candles, nil
}

func (p *BISTBulletinDBProvider) FetchDailyBulletinRecords(ctx context.Context, symbol string, limit int) ([]DailyBulletinRecord, error) {
	return p.FetchDailyBulletinRecordsRange(ctx, symbol, "", "", limit)
}

func (p *BISTBulletinDBProvider) FetchDailyBulletinRecordsRange(ctx context.Context, symbol, fromDate, toDate string, limit int) ([]DailyBulletinRecord, error) {
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return nil, fmt.Errorf("empty symbol: %w", ErrSymbolNotFound)
	}
	db, err := p.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	where := []string{"d.symbol = ?", "d.analysis_ready = 1"}
	args := []any{normalized}
	if strings.TrimSpace(fromDate) != "" {
		where = append(where, "d.trading_date >= ?")
		args = append(args, fromDate)
	}
	if strings.TrimSpace(toDate) != "" {
		where = append(where, "d.trading_date <= ?")
		args = append(args, toDate)
	}
	baseQuery := `
SELECT
  d.symbol,
  d.trading_date,
  COALESCE(i.instrument_code, ''),
  COALESCE(NULLIF(i.company_name, ''), d.symbol),
  COALESCE(i.market, ''),
  COALESCE(d.open, 0),
  d.high,
  d.low,
  d.close,
  d.previous_close,
  d.volume,
  d.value_traded,
  d.trade_count,
  d.vwap,
  d.source_format,
  d.source_key
FROM daily_ohlcv d
LEFT JOIN instruments i ON i.symbol = d.symbol
WHERE ` + strings.Join(where, " AND ") + `
`
	query := baseQuery + `ORDER BY d.trading_date ASC`
	if limit > 0 {
		query = `
SELECT * FROM (` + baseQuery + `
ORDER BY d.trading_date DESC
LIMIT ?)
ORDER BY trading_date ASC`
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bist bulletin db records %s: %w", normalized, err)
	}
	defer rows.Close()

	var out []DailyBulletinRecord
	for rows.Next() {
		var r DailyBulletinRecord
		var sourceKey string
		var tradeCount float64
		if err := rows.Scan(
			&r.Symbol,
			&r.TradingDate,
			&r.InstrumentCode,
			&r.InstrumentName,
			&r.Market,
			&r.Open,
			&r.High,
			&r.Low,
			&r.Close,
			&r.PreviousClose,
			&r.Volume,
			&r.ValueTraded,
			&tradeCount,
			&r.VWAP,
			&r.SourceFormat,
			&sourceKey,
		); err != nil {
			return nil, err
		}
		r.TradeCount = tradeCount
		r.SourcePath = p.sourceRef(sourceKey)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("bist bulletin db: no official daily records for %s: %w", normalized, ErrSymbolNotFound)
	}
	return out, nil
}

func (p *BISTBulletinDBProvider) open() (*sql.DB, error) {
	dbPath := p.dbPath
	if strings.TrimSpace(dbPath) == "" {
		dbPath = bistbulletindb.DefaultDBPath
	}
	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=ro&_busy_timeout=10000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func (p *BISTBulletinDBProvider) sourceRef(sourceKey string) string {
	if strings.TrimSpace(sourceKey) == "" {
		return p.dbPath
	}
	return p.dbPath + "#" + sourceKey
}
