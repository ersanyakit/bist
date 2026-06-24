CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS stocks (
    ticker TEXT PRIMARY KEY,
    isin TEXT UNIQUE,
    company_name TEXT NOT NULL,
    market TEXT NOT NULL DEFAULT 'BIST',
    sector TEXT,
    industry TEXT,
    listing_date DATE,
    free_float_ratio NUMERIC(10,6),
    shares_outstanding NUMERIC(24,6),
    paid_in_capital NUMERIC(24,6),
    source TEXT NOT NULL,
    source_url TEXT,
    data_version TEXT NOT NULL DEFAULT 'v1',
    as_of_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stock_prices (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    timeframe TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    open NUMERIC(24,8) NOT NULL CHECK (open >= 0),
    high NUMERIC(24,8) NOT NULL CHECK (high >= 0),
    low NUMERIC(24,8) NOT NULL CHECK (low >= 0),
    close NUMERIC(24,8) NOT NULL CHECK (close >= 0),
    volume NUMERIC(24,4) NOT NULL CHECK (volume >= 0),
    trade_count BIGINT,
    vwap NUMERIC(24,8),
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (high >= open AND high >= close AND high >= low),
    CHECK (low <= open AND low <= close AND low <= high),
    PRIMARY KEY (ticker, timeframe, ts, data_version)
);

CREATE INDEX IF NOT EXISTS idx_stock_prices_symbol_time ON stock_prices(ticker, timeframe, ts DESC);

CREATE TABLE IF NOT EXISTS adjusted_stock_prices (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    timeframe TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    open NUMERIC(24,8) NOT NULL,
    high NUMERIC(24,8) NOT NULL,
    low NUMERIC(24,8) NOT NULL,
    close NUMERIC(24,8) NOT NULL,
    adjusted_close NUMERIC(24,8) NOT NULL,
    volume NUMERIC(24,4) NOT NULL,
    adjustment_version TEXT NOT NULL,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, timeframe, ts, adjustment_version)
);

CREATE INDEX IF NOT EXISTS idx_adjusted_stock_prices_symbol_time ON adjusted_stock_prices(ticker, timeframe, ts DESC);

CREATE TABLE IF NOT EXISTS indexes (
    index_code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL DEFAULT 'v1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS index_prices (
    index_code TEXT NOT NULL REFERENCES indexes(index_code),
    timeframe TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    open NUMERIC(24,8) NOT NULL,
    high NUMERIC(24,8) NOT NULL,
    low NUMERIC(24,8) NOT NULL,
    close NUMERIC(24,8) NOT NULL,
    volume NUMERIC(24,4),
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (index_code, timeframe, ts, data_version)
);

CREATE TABLE IF NOT EXISTS sector_indexes (
    sector_code TEXT PRIMARY KEY,
    index_code TEXT REFERENCES indexes(index_code),
    sector_name TEXT NOT NULL,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL DEFAULT 'v1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sector_index_prices (
    sector_code TEXT NOT NULL REFERENCES sector_indexes(sector_code),
    timeframe TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    open NUMERIC(24,8) NOT NULL,
    high NUMERIC(24,8) NOT NULL,
    low NUMERIC(24,8) NOT NULL,
    close NUMERIC(24,8) NOT NULL,
    volume NUMERIC(24,4),
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (sector_code, timeframe, ts, data_version)
);

CREATE TABLE IF NOT EXISTS balance_sheets (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    period_year INTEGER NOT NULL,
    period_quarter INTEGER NOT NULL DEFAULT 0,
    consolidated BOOLEAN NOT NULL DEFAULT true,
    total_assets NUMERIC(28,4),
    total_liabilities NUMERIC(28,4),
    total_equity NUMERIC(28,4),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL,
    source_url TEXT,
    data_version TEXT NOT NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, period_year, period_quarter, consolidated, data_version)
);

CREATE INDEX IF NOT EXISTS idx_balance_sheets_period ON balance_sheets(ticker, period_year DESC, period_quarter DESC);

CREATE TABLE IF NOT EXISTS income_statements (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    period_year INTEGER NOT NULL,
    period_quarter INTEGER NOT NULL DEFAULT 0,
    consolidated BOOLEAN NOT NULL DEFAULT true,
    sales NUMERIC(28,4),
    ebit NUMERIC(28,4),
    ebitda NUMERIC(28,4),
    net_income NUMERIC(28,4),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, period_year, period_quarter, consolidated, data_version)
);

CREATE TABLE IF NOT EXISTS cash_flow_statements (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    period_year INTEGER NOT NULL,
    period_quarter INTEGER NOT NULL DEFAULT 0,
    consolidated BOOLEAN NOT NULL DEFAULT true,
    operating_cash_flow NUMERIC(28,4),
    capex NUMERIC(28,4),
    free_cash_flow NUMERIC(28,4),
    end_cash NUMERIC(28,4),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, period_year, period_quarter, consolidated, data_version)
);

CREATE TABLE IF NOT EXISTS equity_statements (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    period_year INTEGER NOT NULL,
    period_quarter INTEGER NOT NULL DEFAULT 0,
    consolidated BOOLEAN NOT NULL DEFAULT true,
    closing_equity NUMERIC(28,4),
    paid_in_capital NUMERIC(28,4),
    dividends NUMERIC(28,4),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, period_year, period_quarter, consolidated, data_version)
);

CREATE TABLE IF NOT EXISTS financial_notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    period_year INTEGER NOT NULL,
    period_quarter INTEGER NOT NULL DEFAULT 0,
    topic TEXT NOT NULL,
    body TEXT NOT NULL,
    source TEXT NOT NULL,
    source_url TEXT,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS financial_ratios (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    period_year INTEGER NOT NULL,
    period_quarter INTEGER NOT NULL DEFAULT 0,
    pe NUMERIC(24,8),
    pb NUMERIC(24,8),
    ps NUMERIC(24,8),
    ev_ebitda NUMERIC(24,8),
    ev_ebit NUMERIC(24,8),
    ev_sales NUMERIC(24,8),
    p_fcf NUMERIC(24,8),
    dividend_yield NUMERIC(24,8),
    roe NUMERIC(24,8),
    roic NUMERIC(24,8),
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, period_year, period_quarter, data_version)
);

CREATE TABLE IF NOT EXISTS corporate_actions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    action_type TEXT NOT NULL,
    ex_date DATE NOT NULL,
    pay_date DATE,
    ratio NUMERIC(24,8),
    cash_amount NUMERIC(24,8),
    currency TEXT,
    source TEXT NOT NULL,
    source_url TEXT,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ticker, action_type, ex_date, data_version)
);

CREATE INDEX IF NOT EXISTS idx_corporate_actions_symbol_date ON corporate_actions(ticker, ex_date DESC);

CREATE TABLE IF NOT EXISTS dividends (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    ex_date DATE NOT NULL,
    pay_date DATE,
    gross_amount NUMERIC(24,8) NOT NULL,
    net_amount NUMERIC(24,8),
    currency TEXT NOT NULL DEFAULT 'TRY',
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ticker, ex_date, data_version)
);

CREATE TABLE IF NOT EXISTS disclosures (
    disclosure_id TEXT PRIMARY KEY,
    ticker TEXT REFERENCES stocks(ticker),
    title TEXT NOT NULL,
    category TEXT,
    disclosure_type TEXT,
    published_at TIMESTAMPTZ NOT NULL,
    url TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_disclosures_symbol_time ON disclosures(ticker, published_at DESC);

CREATE TABLE IF NOT EXISTS macro_series (
    series_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    frequency TEXT NOT NULL,
    unit TEXT NOT NULL,
    source TEXT NOT NULL,
    source_url TEXT,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS macro_observations (
    series_id TEXT NOT NULL REFERENCES macro_series(series_id),
    obs_date DATE NOT NULL,
    value NUMERIC(28,8) NOT NULL,
    unit TEXT NOT NULL,
    revised BOOLEAN NOT NULL DEFAULT false,
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (series_id, obs_date, data_version)
);

CREATE INDEX IF NOT EXISTS idx_macro_observations_series_date ON macro_observations(series_id, obs_date DESC);

CREATE TABLE IF NOT EXISTS analysis_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    analysis_date DATE NOT NULL,
    data_quality_score NUMERIC(8,4),
    technical_score NUMERIC(8,4),
    fundamental_score NUMERIC(8,4),
    valuation_score NUMERIC(8,4),
    risk_score NUMERIC(8,4),
    macro_sensitivity_score NUMERIC(8,4),
    overall_score NUMERIC(8,4),
    investment_view TEXT,
    confidence_level NUMERIC(8,4),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL DEFAULT 'hissebot',
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ticker, analysis_date, data_version)
);

CREATE TABLE IF NOT EXISTS technical_indicators (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    timeframe TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    indicator_name TEXT NOT NULL,
    value NUMERIC(28,8),
    signal TEXT,
    confidence NUMERIC(8,4),
    source TEXT NOT NULL,
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, timeframe, ts, indicator_name, data_version)
);

CREATE TABLE IF NOT EXISTS valuation_results (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    analysis_date DATE NOT NULL,
    method TEXT NOT NULL,
    fair_value_base NUMERIC(24,8),
    fair_value_bear NUMERIC(24,8),
    fair_value_bull NUMERIC(24,8),
    margin_of_safety NUMERIC(12,8),
    confidence NUMERIC(8,4),
    assumptions JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL DEFAULT 'hissebot',
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, analysis_date, method, data_version)
);

CREATE TABLE IF NOT EXISTS risk_scores (
    ticker TEXT NOT NULL REFERENCES stocks(ticker),
    analysis_date DATE NOT NULL,
    risk_type TEXT NOT NULL,
    score NUMERIC(8,4) NOT NULL,
    explanation TEXT,
    source TEXT NOT NULL DEFAULT 'hissebot',
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, analysis_date, risk_type, data_version)
);

CREATE TABLE IF NOT EXISTS data_quality_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    status TEXT NOT NULL,
    score NUMERIC(8,4) NOT NULL,
    issues JSONB NOT NULL DEFAULT '[]'::jsonb,
    source TEXT NOT NULL DEFAULT 'hissebot',
    data_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_data_quality_entity ON data_quality_reports(entity_type, entity_id, created_at DESC);
