package bistbulletindb

import "time"

const (
	DefaultDBPath   = "data/bist/bist_ohlcv.sqlite"
	DefaultRawRoot  = "data/bist/unprocessed/bulten_verileri"
	SourceVersion   = "bist_thb_v1"
	SourceStatusOK  = "processed"
	SourceStatus404 = "missing"
	SourceStatusErr = "error"
)

type DailyRecord struct {
	Symbol         string
	InstrumentCode string
	CompanyName    string
	TradingDate    time.Time
	Open           *float64
	High           float64
	Low            float64
	Close          float64
	PreviousClose  float64
	Volume         float64
	ValueTraded    float64
	TradeCount     int64
	VWAP           float64
	Market         string
	SourceFormat   string
	AnalysisReady  bool
	QualityFlags   []string
}

type SourceState struct {
	SourceKey string
	Status    string
	CheckedAt time.Time
}

type SourceResult struct {
	SourceKey         string
	TradingDate       time.Time
	Session           int
	RemoteURL         string
	SourceFormat      string
	ContentSHA256     string
	SourceBytes       int64
	RowsSeen          int
	RowsStored        int
	RowsAnalysisReady int
	CheckedAt         time.Time
	Error             string
}

type Options struct {
	DBPath            string
	RawRoot           string
	BaseURL           string
	FromYear          int
	ToYear            int
	Session           int
	ImportLocal       bool
	Download          bool
	PruneRaw          bool
	Force             bool
	Workers           int
	RequestDelay      time.Duration
	Timeout           time.Duration
	RetryMissingAfter time.Duration
	Now               func() time.Time
}

type Report struct {
	GeneratedAtUTC       time.Time `json:"generated_at_utc"`
	DBPath               string    `json:"db_path"`
	RawRoot              string    `json:"raw_root"`
	FromYear             int       `json:"from_year"`
	ToYear               int       `json:"to_year"`
	Session              int       `json:"session"`
	LocalSourcesFound    int       `json:"local_sources_found"`
	LocalSourcesImported int       `json:"local_sources_imported"`
	LocalSourcesSkipped  int       `json:"local_sources_skipped"`
	LocalSourcesPruned   int       `json:"local_sources_pruned"`
	RemoteCandidates     int       `json:"remote_candidates"`
	RemoteSkipped        int       `json:"remote_skipped"`
	RemoteDownloaded     int       `json:"remote_downloaded"`
	RemoteMissing        int       `json:"remote_missing"`
	SourcesFailed        int       `json:"sources_failed"`
	RowsSeen             int       `json:"rows_seen"`
	RowsStored           int       `json:"rows_stored"`
	RowsAnalysisReady    int       `json:"rows_analysis_ready"`
	DatabaseSources      int       `json:"database_sources"`
	DatabaseCandles      int       `json:"database_candles"`
	DatabaseSymbols      int       `json:"database_symbols"`
	Errors               []string  `json:"errors,omitempty"`
}

func SourceKey(day time.Time, session int) string {
	return "bist_thb:" + day.Format("2006-01-02") + ":s" + itoa(session)
}

func itoa(value int) string {
	if value == 2 {
		return "2"
	}
	return "1"
}
