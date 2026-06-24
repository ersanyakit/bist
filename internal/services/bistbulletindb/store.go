package bistbulletindb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if path == "" {
		path = DefaultDBPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=10000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.initialize(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("initialize BIST bulletin SQLite schema: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SourceStates(ctx context.Context) (map[string]SourceState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source_key, status, checked_at FROM bulletin_sources`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SourceState{}
	for rows.Next() {
		var state SourceState
		var checked string
		if err := rows.Scan(&state.SourceKey, &state.Status, &checked); err != nil {
			return nil, err
		}
		state.CheckedAt, _ = time.Parse(time.RFC3339Nano, checked)
		out[state.SourceKey] = state
	}
	return out, rows.Err()
}

func (s *Store) SaveProcessedSource(ctx context.Context, source SourceResult, records []DailyRecord) error {
	now := source.CheckedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
INSERT INTO bulletin_sources (
    source_key, trading_date, session, remote_url, source_format, status,
    content_sha256, source_bytes, rows_seen, rows_stored, rows_analysis_ready,
    checked_at, processed_at, error
) VALUES (?, ?, ?, ?, ?, 'processing', ?, ?, ?, 0, 0, ?, NULL, NULL)
ON CONFLICT(source_key) DO UPDATE SET
    remote_url=excluded.remote_url,
    source_format=excluded.source_format,
    status='processing',
    content_sha256=excluded.content_sha256,
    source_bytes=excluded.source_bytes,
    rows_seen=excluded.rows_seen,
    checked_at=excluded.checked_at,
    processed_at=NULL,
    error=NULL`,
		source.SourceKey, source.TradingDate.Format("2006-01-02"), source.Session,
		source.RemoteURL, source.SourceFormat, source.ContentSHA256, source.SourceBytes,
		source.RowsSeen, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM daily_ohlcv WHERE source_key = ?`, source.SourceKey); err != nil {
		return err
	}
	instrumentStmt, err := tx.PrepareContext(ctx, `
INSERT INTO instruments (
    symbol, instrument_code, company_name, market, first_trade_date, last_trade_date, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(symbol) DO UPDATE SET
    instrument_code=excluded.instrument_code,
    company_name=CASE WHEN excluded.company_name <> '' THEN excluded.company_name ELSE instruments.company_name END,
    market=CASE WHEN excluded.market <> '' THEN excluded.market ELSE instruments.market END,
    first_trade_date=MIN(instruments.first_trade_date, excluded.first_trade_date),
    last_trade_date=MAX(instruments.last_trade_date, excluded.last_trade_date),
    updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer instrumentStmt.Close()
	priceStmt, err := tx.PrepareContext(ctx, `
INSERT INTO daily_ohlcv (
    symbol, trading_date, timeframe, open, high, low, close, adjusted_close,
    volume, value_traded, trade_count, vwap, previous_close, is_adjusted,
    analysis_ready, quality_flags, source_key, source_format, data_version,
    created_at, updated_at
) VALUES (?, ?, '1D', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(symbol, trading_date) DO UPDATE SET
    open=excluded.open,
    high=excluded.high,
    low=excluded.low,
    close=excluded.close,
    adjusted_close=excluded.adjusted_close,
    volume=excluded.volume,
    value_traded=excluded.value_traded,
    trade_count=excluded.trade_count,
    vwap=excluded.vwap,
    previous_close=excluded.previous_close,
    is_adjusted=excluded.is_adjusted,
    analysis_ready=excluded.analysis_ready,
    quality_flags=excluded.quality_flags,
    source_key=excluded.source_key,
    source_format=excluded.source_format,
    data_version=excluded.data_version,
    updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer priceStmt.Close()
	stored := 0
	ready := 0
	for _, record := range records {
		date := record.TradingDate.Format("2006-01-02")
		if _, err := instrumentStmt.ExecContext(ctx,
			record.Symbol, record.InstrumentCode, record.CompanyName, record.Market,
			date, date, now.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("upsert instrument %s: %w", record.Symbol, err)
		}
		flags, err := json.Marshal(record.QualityFlags)
		if err != nil {
			return err
		}
		var open any
		if record.Open != nil {
			open = *record.Open
		}
		analysisReady := 0
		if record.AnalysisReady {
			analysisReady = 1
			ready++
		}
		if _, err := priceStmt.ExecContext(ctx,
			record.Symbol, date, open, record.High, record.Low, record.Close, record.Close,
			record.Volume, record.ValueTraded, record.TradeCount, record.VWAP, record.PreviousClose,
			analysisReady, string(flags), source.SourceKey, record.SourceFormat, SourceVersion,
			now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("upsert OHLCV %s %s: %w", record.Symbol, date, err)
		}
		stored++
	}
	_, err = tx.ExecContext(ctx, `
UPDATE bulletin_sources SET
    status='processed', rows_stored=?, rows_analysis_ready=?, processed_at=?, error=NULL
WHERE source_key=?`, stored, ready, now.Format(time.RFC3339Nano), source.SourceKey)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkSource(ctx context.Context, source SourceResult, status string) error {
	checked := source.CheckedAt.UTC()
	if checked.IsZero() {
		checked = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO bulletin_sources (
    source_key, trading_date, session, remote_url, source_format, status,
    content_sha256, source_bytes, rows_seen, rows_stored, rows_analysis_ready,
    checked_at, processed_at, error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, NULL, ?)
ON CONFLICT(source_key) DO UPDATE SET
    remote_url=excluded.remote_url,
    source_format=excluded.source_format,
    status=excluded.status,
    content_sha256=excluded.content_sha256,
    source_bytes=excluded.source_bytes,
    rows_seen=excluded.rows_seen,
    checked_at=excluded.checked_at,
    processed_at=NULL,
    error=excluded.error`,
		source.SourceKey, source.TradingDate.Format("2006-01-02"), source.Session,
		source.RemoteURL, source.SourceFormat, status, source.ContentSHA256,
		source.SourceBytes, source.RowsSeen, checked.Format(time.RFC3339Nano), source.Error,
	)
	return err
}

func (s *Store) Counts(ctx context.Context) (sources int, candles int, symbols int, err error) {
	for _, item := range []struct {
		query string
		out   *int
	}{
		{`SELECT COUNT(*) FROM bulletin_sources WHERE status='processed'`, &sources},
		{`SELECT COUNT(*) FROM daily_ohlcv`, &candles},
		{`SELECT COUNT(*) FROM instruments`, &symbols},
	} {
		if err = s.db.QueryRowContext(ctx, item.query).Scan(item.out); err != nil {
			return 0, 0, 0, err
		}
	}
	return sources, candles, symbols, nil
}
