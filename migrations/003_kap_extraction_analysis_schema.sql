CREATE TABLE IF NOT EXISTS companies (
    company_id TEXT PRIMARY KEY,
    ticker TEXT NOT NULL UNIQUE,
    company_name TEXT,
    legal_name TEXT,
    kap_company_id TEXT,
    sector TEXT,
    industry TEXT,
    sub_industry TEXT,
    foundation_date DATE,
    headquarters TEXT,
    address TEXT,
    phone TEXT,
    website TEXT,
    email TEXT,
    trade_registry_number TEXT,
    mersis_number TEXT,
    tax_office TEXT,
    tax_number TEXT,
    paid_in_capital NUMERIC(24,6),
    registered_capital_ceiling NUMERIC(24,6),
    free_float_rate NUMERIC(12,6),
    listing_date DATE,
    market TEXT,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT false,
    last_updated_source TEXT,
    last_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS company_identifiers (
    company_id TEXT NOT NULL REFERENCES companies(company_id) ON DELETE CASCADE,
    identifier_type TEXT NOT NULL,
    identifier_value TEXT NOT NULL,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (identifier_type, identifier_value)
);

CREATE TABLE IF NOT EXISTS kap_disclosures (
    disclosure_id TEXT PRIMARY KEY,
    ticker TEXT NOT NULL,
    disclosure_index BIGINT,
    title TEXT,
    disclosure_class TEXT,
    disclosure_type TEXT,
    disclosure_category TEXT,
    period TEXT,
    fiscal_year INTEGER,
    publish_date TIMESTAMPTZ,
    source_url TEXT,
    raw JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_kap_disclosures_ticker_date ON kap_disclosures(ticker, publish_date DESC);
CREATE INDEX IF NOT EXISTS idx_kap_disclosures_index ON kap_disclosures(disclosure_index);

CREATE TABLE IF NOT EXISTS normalized_financial_items (
    item_code TEXT PRIMARY KEY,
    normalized_name TEXT NOT NULL UNIQUE,
    statement_type TEXT NOT NULL,
    synonyms TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS financial_facts (
    fact_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    period TEXT,
    fiscal_year INTEGER,
    statement_type TEXT NOT NULL CHECK (statement_type IN ('balance_sheet', 'income_statement', 'cash_flow_statement', 'equity_statement', 'note')),
    line_item_original TEXT NOT NULL,
    line_item_normalized TEXT NOT NULL,
    value NUMERIC(28,6) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'TRY',
    unit TEXT NOT NULL,
    is_consolidated BOOLEAN NOT NULL DEFAULT false,
    accounting_standard TEXT NOT NULL DEFAULT 'UNKNOWN',
    is_audited BOOLEAN,
    source_document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
    source_page INTEGER,
    source_table_id TEXT,
    source_row TEXT,
    source_column TEXT,
    source_text TEXT,
    confidence_score NUMERIC(8,4) NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 1),
    validation_status TEXT NOT NULL CHECK (validation_status IN ('valid', 'invalid', 'warning', 'unknown')),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_financial_facts_ticker_period ON financial_facts(ticker, fiscal_year DESC, period);
CREATE INDEX IF NOT EXISTS idx_financial_facts_source ON financial_facts(source_document_id, source_page);
CREATE INDEX IF NOT EXISTS idx_financial_facts_review ON financial_facts(review_required, validation_status);

CREATE TABLE IF NOT EXISTS people (
    person_id TEXT PRIMARY KEY,
    full_name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    education TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_people_normalized_name ON people(normalized_name);

CREATE TABLE IF NOT EXISTS company_people_roles (
    company_id TEXT,
    ticker TEXT NOT NULL,
    person_id TEXT NOT NULL REFERENCES people(person_id) ON DELETE CASCADE,
    title TEXT,
    role TEXT,
    committee_role TEXT,
    start_date DATE,
    end_date DATE,
    is_independent_board_member BOOLEAN,
    related_party_status TEXT,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    source_page INTEGER,
    source_text TEXT,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ticker, person_id, role, title, source_document_id)
);

CREATE TABLE IF NOT EXISTS shareholders (
    shareholder_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    name TEXT NOT NULL,
    share_amount NUMERIC(28,6),
    share_ratio NUMERIC(12,6),
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS subsidiaries (
    subsidiary_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    name TEXT NOT NULL,
    ownership_ratio NUMERIC(12,6),
    country TEXT,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS associates (
    associate_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    name TEXT NOT NULL,
    ownership_ratio NUMERIC(12,6),
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS corporate_events (
    event_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    event_date DATE,
    event_type TEXT NOT NULL,
    event_title TEXT NOT NULL,
    description TEXT NOT NULL,
    affected_assets TEXT[] NOT NULL DEFAULT '{}',
    affected_financial_items TEXT[] NOT NULL DEFAULT '{}',
    amount NUMERIC(28,6),
    currency TEXT,
    counterparty TEXT,
    location TEXT,
    source_document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
    source_page INTEGER,
    source_text TEXT,
    confidence_score NUMERIC(8,4) NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_corporate_events_ticker_type ON corporate_events(ticker, event_type, event_date DESC);

CREATE TABLE IF NOT EXISTS tracked_assets (
    asset_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    asset_name TEXT NOT NULL,
    asset_type TEXT NOT NULL,
    location TEXT,
    acquisition_date DATE,
    acquisition_cost NUMERIC(28,6),
    currency TEXT,
    current_book_value NUMERIC(28,6),
    fair_value NUMERIC(28,6),
    valuation_date DATE,
    status TEXT NOT NULL CHECK (status IN ('active', 'sold', 'transferred', 'impaired', 'pledged', 'unknown', 'likely_active', 'likely_sold')),
    confidence_score NUMERIC(8,4) NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS asset_events (
    asset_id TEXT NOT NULL REFERENCES tracked_assets(asset_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL REFERENCES corporate_events(event_id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL DEFAULT 'mentioned',
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, event_id, relation_type)
);

CREATE TABLE IF NOT EXISTS related_party_transactions (
    transaction_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    counterparty TEXT,
    description TEXT NOT NULL,
    amount NUMERIC(28,6),
    currency TEXT,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pledges_mortgages_guarantees (
    pmg_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    pmg_type TEXT NOT NULL,
    description TEXT NOT NULL,
    amount NUMERIC(28,6),
    currency TEXT,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS legal_cases (
    legal_case_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    description TEXT NOT NULL,
    amount NUMERIC(28,6),
    currency TEXT,
    status TEXT,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_reports (
    audit_report_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    period TEXT,
    fiscal_year INTEGER,
    auditor TEXT,
    opinion TEXT,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS evidence_chains (
    evidence_chain_id TEXT PRIMARY KEY,
    subject TEXT NOT NULL,
    company_id TEXT,
    ticker TEXT NOT NULL,
    initial_event JSONB NOT NULL,
    yearly_follow_up JSONB NOT NULL DEFAULT '[]'::jsonb,
    current_status TEXT NOT NULL,
    final_confidence_score NUMERIC(8,4) NOT NULL CHECK (final_confidence_score >= 0 AND final_confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS validation_results (
    validation_id TEXT PRIMARY KEY,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('valid', 'invalid', 'warning', 'unknown')),
    severity TEXT NOT NULL,
    code TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS confidence_scores (
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    confidence_score NUMERIC(8,4) NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 1),
    method TEXT NOT NULL,
    signals JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (subject_type, subject_id, method)
);

CREATE TABLE IF NOT EXISTS human_review_queue (
    review_id TEXT PRIMARY KEY,
    ticker TEXT NOT NULL,
    subject_type TEXT NOT NULL,
    subject_id TEXT,
    reason TEXT NOT NULL,
    source_document_id TEXT REFERENCES documents(document_id) ON DELETE SET NULL,
    source_page INTEGER,
    source_text TEXT,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'approved', 'rejected', 'resolved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_human_review_queue_status ON human_review_queue(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_human_review_queue_ticker ON human_review_queue(ticker, status, created_at DESC);
