package stocks

import "time"

type SourceMeta struct {
	Source      string    `json:"source"`
	SourceURL   string    `json:"source_url,omitempty"`
	DataVersion string    `json:"data_version,omitempty"`
	AsOf        time.Time `json:"as_of"`
	IngestedAt  time.Time `json:"ingested_at"`
}

type Stock struct {
	Ticker            string     `json:"ticker"`
	ISIN              string     `json:"isin,omitempty"`
	CompanyName       string     `json:"company_name"`
	Market            string     `json:"market"`
	Sector            string     `json:"sector,omitempty"`
	Industry          string     `json:"industry,omitempty"`
	ListingDate       time.Time  `json:"listing_date,omitempty"`
	FreeFloatRatio    float64    `json:"free_float_ratio,omitempty"`
	SharesOutstanding float64    `json:"shares_outstanding,omitempty"`
	PaidInCapital     float64    `json:"paid_in_capital,omitempty"`
	Meta              SourceMeta `json:"meta"`
}

type OwnershipStake struct {
	Symbol    string     `json:"symbol"`
	OwnerName string     `json:"owner_name"`
	SharePct  float64    `json:"share_pct"`
	VotingPct float64    `json:"voting_pct,omitempty"`
	OwnerType string     `json:"owner_type,omitempty"`
	Meta      SourceMeta `json:"meta"`
}

type Subsidiary struct {
	Symbol   string     `json:"symbol"`
	Name     string     `json:"name"`
	SharePct float64    `json:"share_pct,omitempty"`
	Country  string     `json:"country,omitempty"`
	Activity string     `json:"activity,omitempty"`
	Meta     SourceMeta `json:"meta"`
}

type SegmentBreakdown struct {
	Symbol     string     `json:"symbol"`
	PeriodYear int        `json:"period_year"`
	Segment    string     `json:"segment"`
	Revenue    float64    `json:"revenue,omitempty"`
	Profit     float64    `json:"profit,omitempty"`
	Geography  string     `json:"geography,omitempty"`
	Currency   string     `json:"currency"`
	Meta       SourceMeta `json:"meta"`
}

type FXPosition struct {
	Symbol     string     `json:"symbol"`
	PeriodYear int        `json:"period_year"`
	LongFX     float64    `json:"long_fx,omitempty"`
	ShortFX    float64    `json:"short_fx,omitempty"`
	NetFX      float64    `json:"net_fx,omitempty"`
	Currency   string     `json:"currency"`
	Meta       SourceMeta `json:"meta"`
}

type DebtMaturityBucket struct {
	Symbol     string     `json:"symbol"`
	PeriodYear int        `json:"period_year"`
	Bucket     string     `json:"bucket"`
	Amount     float64    `json:"amount"`
	Currency   string     `json:"currency"`
	Meta       SourceMeta `json:"meta"`
}

type WorkingCapitalBreakdown struct {
	Symbol           string     `json:"symbol"`
	PeriodYear       int        `json:"period_year"`
	TradeReceivables float64    `json:"trade_receivables,omitempty"`
	Inventory        float64    `json:"inventory,omitempty"`
	TradePayables    float64    `json:"trade_payables,omitempty"`
	Currency         string     `json:"currency"`
	Meta             SourceMeta `json:"meta"`
}

type ReferenceData struct {
	Stock          Stock                    `json:"stock"`
	Ownership      []OwnershipStake         `json:"ownership,omitempty"`
	Subsidiaries   []Subsidiary             `json:"subsidiaries,omitempty"`
	Segments       []SegmentBreakdown       `json:"segments,omitempty"`
	FXPosition     *FXPosition              `json:"fx_position,omitempty"`
	DebtMaturity   []DebtMaturityBucket     `json:"debt_maturity,omitempty"`
	WorkingCapital *WorkingCapitalBreakdown `json:"working_capital,omitempty"`
}
