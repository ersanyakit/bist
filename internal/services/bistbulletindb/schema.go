package bistbulletindb

const schemaSQL = `
CREATE TABLE IF NOT EXISTS bulletin_sources (
    source_key TEXT PRIMARY KEY,
    trading_date TEXT NOT NULL,
    session INTEGER NOT NULL,
    remote_url TEXT NOT NULL,
    source_format TEXT,
    status TEXT NOT NULL CHECK (status IN ('processing','processed','missing','error')),
    content_sha256 TEXT,
    source_bytes INTEGER NOT NULL DEFAULT 0,
    rows_seen INTEGER NOT NULL DEFAULT 0,
    rows_stored INTEGER NOT NULL DEFAULT 0,
    rows_analysis_ready INTEGER NOT NULL DEFAULT 0,
    checked_at TEXT NOT NULL,
    processed_at TEXT,
    error TEXT,
    UNIQUE (trading_date, session)
);

CREATE INDEX IF NOT EXISTS idx_bulletin_sources_status_date
    ON bulletin_sources(status, trading_date);

CREATE TABLE IF NOT EXISTS instruments (
    symbol TEXT PRIMARY KEY,
    instrument_code TEXT NOT NULL,
    company_name TEXT,
    market TEXT,
    first_trade_date TEXT NOT NULL,
    last_trade_date TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS daily_ohlcv (
    symbol TEXT NOT NULL REFERENCES instruments(symbol),
    trading_date TEXT NOT NULL,
    timeframe TEXT NOT NULL DEFAULT '1D' CHECK (timeframe = '1D'),
    open REAL,
    high REAL NOT NULL CHECK (high > 0),
    low REAL NOT NULL CHECK (low > 0),
    close REAL NOT NULL CHECK (close > 0),
    adjusted_close REAL NOT NULL CHECK (adjusted_close > 0),
    volume REAL NOT NULL CHECK (volume >= 0),
    value_traded REAL NOT NULL DEFAULT 0 CHECK (value_traded >= 0),
    trade_count INTEGER NOT NULL DEFAULT 0 CHECK (trade_count >= 0),
    vwap REAL NOT NULL DEFAULT 0 CHECK (vwap >= 0),
    previous_close REAL NOT NULL DEFAULT 0 CHECK (previous_close >= 0),
    is_adjusted INTEGER NOT NULL DEFAULT 0 CHECK (is_adjusted IN (0,1)),
    analysis_ready INTEGER NOT NULL CHECK (analysis_ready IN (0,1)),
    quality_flags TEXT NOT NULL DEFAULT '[]',
    source_key TEXT NOT NULL REFERENCES bulletin_sources(source_key),
    source_format TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (symbol, trading_date)
);

CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_symbol_date
    ON daily_ohlcv(symbol, trading_date DESC);
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_ready_symbol_date
    ON daily_ohlcv(analysis_ready, symbol, trading_date DESC);
CREATE INDEX IF NOT EXISTS idx_daily_ohlcv_source
    ON daily_ohlcv(source_key);

PRAGMA user_version = 1;
`
