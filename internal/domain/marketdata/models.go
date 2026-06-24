package marketdata

import (
	"encoding/json"
	"time"
)

type Timeframe string

const (
	Timeframe1Min    Timeframe = "1m"
	Timeframe5Min    Timeframe = "5m"
	Timeframe15Min   Timeframe = "15m"
	Timeframe1Hour   Timeframe = "1h"
	TimeframeDaily   Timeframe = "daily"
	TimeframeWeekly  Timeframe = "weekly"
	TimeframeMonthly Timeframe = "monthly"
)

type SourceMeta struct {
	Source      string    `json:"source"`
	SourceURL   string    `json:"source_url,omitempty"`
	License     string    `json:"license,omitempty"`
	AsOf        time.Time `json:"as_of"`
	DataVersion string    `json:"data_version,omitempty"`
	IngestedAt  time.Time `json:"ingested_at"`
}

type OHLCV struct {
	Symbol        string     `json:"symbol"`
	Timeframe     Timeframe  `json:"timeframe"`
	Timestamp     time.Time  `json:"timestamp"`
	Open          float64    `json:"open"`
	High          float64    `json:"high"`
	Low           float64    `json:"low"`
	Close         float64    `json:"close"`
	AdjustedClose float64    `json:"adjusted_close"`
	Volume        float64    `json:"volume"`
	TradeCount    int64      `json:"trade_count,omitempty"`
	VWAP          float64    `json:"vwap,omitempty"`
	Meta          SourceMeta `json:"meta"`
}

type PriceSnapshot struct {
	Symbol    string     `json:"symbol"`
	Last      float64    `json:"last"`
	Bid       float64    `json:"bid,omitempty"`
	Ask       float64    `json:"ask,omitempty"`
	Open      float64    `json:"open,omitempty"`
	High      float64    `json:"high,omitempty"`
	Low       float64    `json:"low,omitempty"`
	Volume    float64    `json:"volume,omitempty"`
	Currency  string     `json:"currency"`
	Timestamp time.Time  `json:"timestamp"`
	Meta      SourceMeta `json:"meta"`
}

type OrderBookLevel struct {
	Price      float64 `json:"price"`
	Quantity   float64 `json:"quantity"`
	OrderCount int64   `json:"order_count,omitempty"`
}

type OrderBook struct {
	Symbol    string           `json:"symbol"`
	Bids      []OrderBookLevel `json:"bids"`
	Asks      []OrderBookLevel `json:"asks"`
	Timestamp time.Time        `json:"timestamp"`
	Meta      SourceMeta       `json:"meta"`
}

type TradeSummary struct {
	Symbol           string     `json:"symbol"`
	Timestamp        time.Time  `json:"timestamp"`
	TradeCount       int64      `json:"trade_count"`
	AverageTradeSize float64    `json:"average_trade_size"`
	ValueTraded      float64    `json:"value_traded"`
	Meta             SourceMeta `json:"meta"`
}

type CorporateActionType string

const (
	ActionCashDividend    CorporateActionType = "cash_dividend"
	ActionStockDividend   CorporateActionType = "stock_dividend"
	ActionBonusIssue      CorporateActionType = "bonus_issue"
	ActionRightsIssue     CorporateActionType = "rights_issue"
	ActionSplit           CorporateActionType = "share_split"
	ActionReverseSplit    CorporateActionType = "reverse_split"
	ActionBuyback         CorporateActionType = "share_buyback"
	ActionCapitalIncrease CorporateActionType = "capital_increase"
	ActionCapitalDecrease CorporateActionType = "capital_decrease"
	ActionMerger          CorporateActionType = "merger"
	ActionSpinOff         CorporateActionType = "spin_off"
)

type CorporateAction struct {
	Symbol      string              `json:"symbol"`
	Type        CorporateActionType `json:"type"`
	ExDate      time.Time           `json:"ex_date"`
	PayDate     time.Time           `json:"pay_date,omitempty"`
	Ratio       float64             `json:"ratio,omitempty"`
	CashAmount  float64             `json:"cash_amount,omitempty"`
	Currency    string              `json:"currency,omitempty"`
	Description string              `json:"description,omitempty"`
	Meta        SourceMeta          `json:"meta"`
}

type IndexConstituent struct {
	IndexCode string     `json:"index_code"`
	Symbol    string     `json:"symbol"`
	Weight    float64    `json:"weight,omitempty"`
	AsOf      time.Time  `json:"as_of"`
	Meta      SourceMeta `json:"meta"`
}

type DataCapability struct {
	Name                  string `json:"name"`
	AvailableWithFreeData bool   `json:"available_with_free_data"`
	RequiresLicense       bool   `json:"requires_license"`
	RequiredFor           string `json:"required_for,omitempty"`
}

type LiveIndexQuote struct {
	Code      string    `json:"code"`
	Last      float64   `json:"last,omitempty"`
	Reference float64   `json:"reference,omitempty"`
	Bid       float64   `json:"bid,omitempty"`
	Ask       float64   `json:"ask,omitempty"`
	Raw       []float64 `json:"raw,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LiveSymbolSnapshot struct {
	Symbol    string         `json:"symbol"`
	Last      float64        `json:"last,omitempty"`
	Reference float64        `json:"reference,omitempty"`
	Bid       float64        `json:"bid,omitempty"`
	Ask       float64        `json:"ask,omitempty"`
	Open      float64        `json:"open,omitempty"`
	High      float64        `json:"high,omitempty"`
	Low       float64        `json:"low,omitempty"`
	Change    float64        `json:"change,omitempty"`
	ChangePct float64        `json:"change_pct,omitempty"`
	Volume    float64        `json:"volume,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type LiveMarketSnapshot struct {
	Source          string                        `json:"source"`
	SourceHost      string                        `json:"source_host,omitempty"`
	UpdatedAt       time.Time                     `json:"updated_at"`
	ActiveUsers     int                           `json:"active_users,omitempty"`
	Indices         map[string]LiveIndexQuote     `json:"indices,omitempty"`
	ViewerCounts    map[string]int                `json:"viewer_counts,omitempty"`
	Symbols         map[string]LiveSymbolSnapshot `json:"symbols,omitempty"`
	RequestCounts   map[string]int                `json:"request_counts,omitempty"`
	RequestMetadata map[string]map[string]any     `json:"request_metadata,omitempty"`
	MessageTypes    map[string]int                `json:"message_types,omitempty"`
	Datasets        map[string][]json.RawMessage  `json:"datasets,omitempty"`
	PrivateData     map[string][]json.RawMessage  `json:"private_data,omitempty"`
	RawSamples      []json.RawMessage             `json:"raw_samples,omitempty"`
	MQTTMessages    int                           `json:"mqtt_messages,omitempty"`
	MQTTTopics      map[string]int                `json:"mqtt_topics,omitempty"`
	MQTTRawSamples  []MQTTPublishSample           `json:"mqtt_raw_samples,omitempty"`
	Warnings        []string                      `json:"warnings,omitempty"`
}

func (s LiveMarketSnapshot) HasData() bool {
	return s.ActiveUsers > 0 || len(s.Indices) > 0 || len(s.ViewerCounts) > 0 || len(s.Symbols) > 0 || len(s.Datasets) > 0 || len(s.PrivateData) > 0 || s.MQTTMessages > 0
}

type MQTTPublishSample struct {
	ReceivedAt       time.Time      `json:"received_at"`
	SourceHost       string         `json:"-"`
	Topic            string         `json:"-"`
	Symbol           string         `json:"symbol,omitempty"`
	InstrumentID     int64          `json:"instrument_id,omitempty"`
	MarketTime       string         `json:"market_time,omitempty"`
	MarketDate       string         `json:"market_date,omitempty"`
	MarketClock      string         `json:"market_clock,omitempty"`
	Last             *float64       `json:"last,omitempty"`
	Duplicate        bool           `json:"-"`
	Retain           bool           `json:"-"`
	QoS              byte           `json:"-"`
	PacketID         uint16         `json:"-"`
	PayloadSize      int            `json:"-"`
	PayloadBase64    string         `json:"-"`
	PayloadHexPrefix string         `json:"-"`
	DecodedPayload   map[string]any `json:"-"`
	DecodedSummary   map[string]any `json:"-"`
}
