CREATE TABLE IF NOT EXISTS documents (
    document_id TEXT PRIMARY KEY,
    company_id TEXT,
    ticker TEXT NOT NULL,
    kap_company_id TEXT,
    disclosure_id TEXT NOT NULL,
    disclosure_index BIGINT,
    disclosure_date TIMESTAMPTZ,
    disclosure_type TEXT,
    period TEXT,
    fiscal_year INTEGER,
    document_type TEXT NOT NULL CHECK (document_type IN ('PDF', 'HTML', 'XML', 'XBRL', 'OCR_IMAGE', 'OTHER')),
    file_url TEXT,
    local_file_path TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    checksum_algorithm TEXT NOT NULL DEFAULT 'sha256',
    checksum TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    language TEXT NOT NULL DEFAULT 'mixed' CHECK (language IN ('tr', 'en', 'mixed')),
    source_system TEXT NOT NULL DEFAULT 'KAP',
    version INTEGER NOT NULL CHECK (version > 0),
    is_latest_version BOOLEAN NOT NULL DEFAULT true,
    extraction_status TEXT NOT NULL DEFAULT 'pending' CHECK (extraction_status IN ('pending', 'text_ready', 'needs_ocr', 'rejected', 'review_required')),
    review_required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_system, ticker, disclosure_id, local_file_path, checksum),
    UNIQUE (source_system, ticker, disclosure_id, original_filename, version)
);

CREATE INDEX IF NOT EXISTS idx_documents_ticker_disclosure ON documents(ticker, disclosure_date DESC, disclosure_id);
CREATE INDEX IF NOT EXISTS idx_documents_checksum ON documents(checksum_algorithm, checksum);
CREATE INDEX IF NOT EXISTS idx_documents_latest ON documents(ticker, is_latest_version) WHERE is_latest_version = true;
CREATE INDEX IF NOT EXISTS idx_documents_review ON documents(review_required, extraction_status);

CREATE TABLE IF NOT EXISTS document_pages (
    document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL CHECK (page_number > 0),
    width NUMERIC(18,6),
    height NUMERIC(18,6),
    rotation INTEGER,
    text_extracted BOOLEAN NOT NULL DEFAULT false,
    ocr_required BOOLEAN NOT NULL DEFAULT false,
    checksum TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (document_id, page_number)
);

CREATE TABLE IF NOT EXISTS document_tables (
    table_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
    page_number INTEGER,
    table_index INTEGER NOT NULL,
    bbox JSONB,
    extraction_method TEXT NOT NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_id, page_number, table_index)
);

CREATE TABLE IF NOT EXISTS extracted_text_blocks (
    block_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
    page_number INTEGER,
    block_index INTEGER NOT NULL,
    block_type TEXT NOT NULL,
    text TEXT NOT NULL,
    bbox JSONB,
    extraction_method TEXT NOT NULL,
    confidence_score NUMERIC(8,4) NOT NULL DEFAULT 0 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    review_required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_id, page_number, block_index)
);

CREATE TABLE IF NOT EXISTS extraction_jobs (
    job_id TEXT PRIMARY KEY,
    job_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'success', 'partial', 'failed')),
    ticker TEXT,
    source_system TEXT NOT NULL DEFAULT 'KAP',
    source_root TEXT,
    documents_scanned INTEGER NOT NULL DEFAULT 0,
    documents_saved INTEGER NOT NULL DEFAULT 0,
    documents_skipped INTEGER NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    review_required BOOLEAN NOT NULL DEFAULT false,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_extraction_jobs_status ON extraction_jobs(status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_extraction_jobs_ticker ON extraction_jobs(ticker, started_at DESC);

CREATE TABLE IF NOT EXISTS extraction_errors (
    error_id BIGSERIAL PRIMARY KEY,
    job_id TEXT REFERENCES extraction_jobs(job_id) ON DELETE SET NULL,
    ticker TEXT,
    document_path TEXT,
    stage TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_extraction_errors_job ON extraction_errors(job_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_extraction_errors_ticker ON extraction_errors(ticker, created_at DESC);
