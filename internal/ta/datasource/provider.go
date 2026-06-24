// internal/datasource/provider.go
package datasource

import (
	"context"

	"hissebot/internal/ta/ohlcv"
)

type MarketDataProvider interface {
	SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error)
	FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error)
}

type BulletinRecordProvider interface {
	FetchDailyBulletinRecords(ctx context.Context, symbol string, limit int) ([]DailyBulletinRecord, error)
}

type BulletinRecordRangeProvider interface {
	FetchDailyBulletinRecordsRange(ctx context.Context, symbol, fromDate, toDate string, limit int) ([]DailyBulletinRecord, error)
}

type DailyBulletinRecord struct {
	Symbol                   string  `json:"symbol"`
	TradingDate              string  `json:"trading_date"`
	InstrumentCode           string  `json:"instrument_code,omitempty"`
	InstrumentName           string  `json:"instrument_name,omitempty"`
	MarketGroup              string  `json:"market_group,omitempty"`
	Market                   string  `json:"market,omitempty"`
	InstrumentGroup          string  `json:"instrument_group,omitempty"`
	InstrumentType           string  `json:"instrument_type,omitempty"`
	TradingMethod            string  `json:"trading_method,omitempty"`
	PreviousClose            float64 `json:"previous_close,omitempty"`
	Open                     float64 `json:"open,omitempty"`
	OpeningSessionPrice      float64 `json:"opening_session_price,omitempty"`
	Low                      float64 `json:"low,omitempty"`
	High                     float64 `json:"high,omitempty"`
	Close                    float64 `json:"close,omitempty"`
	ClosingSessionPrice      float64 `json:"closing_session_price,omitempty"`
	ChangePct                float64 `json:"change_pct,omitempty"`
	RemainingBid             float64 `json:"remaining_bid,omitempty"`
	RemainingAsk             float64 `json:"remaining_ask,omitempty"`
	VWAP                     float64 `json:"vwap,omitempty"`
	ValueTraded              float64 `json:"value_traded,omitempty"`
	Volume                   float64 `json:"volume,omitempty"`
	TradeCount               float64 `json:"trade_count,omitempty"`
	ReferencePrice           float64 `json:"reference_price,omitempty"`
	OpeningSessionValue      float64 `json:"opening_session_value,omitempty"`
	OpeningSessionVolume     float64 `json:"opening_session_volume,omitempty"`
	OpeningSessionTradeCount float64 `json:"opening_session_trade_count,omitempty"`
	ClosingSessionValue      float64 `json:"closing_session_value,omitempty"`
	ClosingSessionVolume     float64 `json:"closing_session_volume,omitempty"`
	ClosingSessionTradeCount float64 `json:"closing_session_trade_count,omitempty"`
	SourceFormat             string  `json:"source_format,omitempty"`
	SourcePath               string  `json:"source_path,omitempty"`
}
