package domain

import (
	"encoding/json"
	"time"
)

type Equity struct {
	Ticker                  string                     `json:"ticker"`
	Name                    string                     `json:"name,omitempty"`
	Pair                    string                     `json:"pair,omitempty"`
	AssetType               int                        `json:"asset_type,omitempty"`
	Score                   float64                    `json:"score,omitempty"`
	Data                    map[string]any             `json:"data,omitempty"`
	OHLCV                   *OHLCV                     `json:"ohlcv,omitempty"`
	ChartData               map[string]ChartDataRef    `json:"chart_data,omitempty"`
	KAPInfo                 map[string]any             `json:"kap_info,omitempty"`
	MKKID                   int64                      `json:"mkk_id,omitempty"`
	CompanyInfo             map[string]any             `json:"company_info,omitempty"`
	BilancoInfo             *BilancoInfo               `json:"bilanco_info,omitempty"`
	BilancoCalculations     map[string]YearQuarter     `json:"bilanco_calculations,omitempty"`
	BilancoCalculationAudit *FinancialCalculationAudit `json:"bilanco_calculation_audit,omitempty"`
	Comments                []Comment                  `json:"comments,omitempty"`
	External                map[string]any             `json:"external,omitempty"`
	UpdatedAt               time.Time                  `json:"updated_at"`
	RawTradingViewByFeed    map[string]json.RawMessage `json:"raw_tradingview_by_feed,omitempty"`
}

type OHLCV struct {
	Source      string         `json:"source"`
	Symbol      string         `json:"symbol,omitempty"`
	Exchange    string         `json:"exchange,omitempty"`
	Currency    string         `json:"currency,omitempty"`
	Open        *float64       `json:"open,omitempty"`
	High        *float64       `json:"high,omitempty"`
	Low         *float64       `json:"low,omitempty"`
	Close       *float64       `json:"close,omitempty"`
	Volume      *float64       `json:"volume,omitempty"`
	Change      *float64       `json:"change,omitempty"`
	ChangeAbs   *float64       `json:"change_abs,omitempty"`
	ValueTraded *float64       `json:"value_traded,omitempty"`
	Time        *int64         `json:"time,omitempty"`
	FetchedAt   time.Time      `json:"fetched_at"`
	Raw         map[string]any `json:"raw,omitempty"`
}

type ChartDataRef struct {
	Source    string    `json:"source"`
	Interval  string    `json:"interval"`
	Path      string    `json:"path"`
	Bars      int       `json:"bars"`
	FirstTime *int64    `json:"first_time,omitempty"`
	LastTime  *int64    `json:"last_time,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

type Comment struct {
	Target    string         `json:"target,omitempty"`
	Title     string         `json:"title,omitempty"`
	Tooltip   string         `json:"tooltip,omitempty"`
	Text      string         `json:"text,omitempty"`
	Username  string         `json:"username,omitempty"`
	User      map[string]any `json:"user,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type BilancoInfo struct {
	Ticker         string                     `json:"ticker"`
	Source         string                     `json:"source,omitempty"`
	Currency       string                     `json:"currency,omitempty"`
	FinancialGroup string                     `json:"financial_group,omitempty"`
	FetchedAt      time.Time                  `json:"fetched_at,omitempty"`
	Periods        map[string]FinancialPeriod `json:"periods,omitempty"`
	Lineage        []DataLineageEvent         `json:"lineage,omitempty"`
	Quality        FinancialDataQuality       `json:"quality,omitempty"`
	Data           map[string]BilancoField    `json:"data"`
}

type BilancoField struct {
	DescTR string                `json:"desc_tr,omitempty"`
	DescEN string                `json:"desc_en,omitempty"`
	Years  map[string][]*float64 `json:"years"`
}

type YearQuarter map[string]QuarterValues

type QuarterValues struct {
	Q1 *float64 `json:"Q1"`
	Q2 *float64 `json:"Q2"`
	Q3 *float64 `json:"Q3"`
	Q4 *float64 `json:"Q4"`
}
