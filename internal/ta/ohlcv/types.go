// internal/ohlcv/types.go
package ohlcv

import (
	"encoding/json"
	"strings"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/ta/localize"
)

const Disclaimer = "Bu analiz yatırım tavsiyesi değildir."

const (
	AssetTypeEquity    = domain.AssetKindEquity
	AssetTypeCrypto    = domain.AssetKindCrypto
	AssetTypeCommodity = domain.AssetKindCommodity
)

type Candle struct {
	Time           time.Time `json:"time"`
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	Close          float64   `json:"close"`
	Volume         float64   `json:"volume"`
	AdjustedOpen   float64   `json:"adjusted_open"`
	AdjustedHigh   float64   `json:"adjusted_high"`
	AdjustedLow    float64   `json:"adjusted_low"`
	AdjustedClose  float64   `json:"adjusted_close"`
	AdjustedVolume float64   `json:"adjusted_volume"`
	IsAdjusted     bool      `json:"is_adjusted"`
}

func (c Candle) EffectiveOpen() float64 {
	if c.IsAdjusted && c.AdjustedOpen > 0 {
		return c.AdjustedOpen
	}
	return c.Open
}

func (c Candle) EffectiveHigh() float64 {
	if c.IsAdjusted && c.AdjustedHigh > 0 {
		return c.AdjustedHigh
	}
	return c.High
}

func (c Candle) EffectiveLow() float64 {
	if c.IsAdjusted && c.AdjustedLow > 0 {
		return c.AdjustedLow
	}
	return c.Low
}

func (c Candle) EffectiveClose() float64 {
	if c.IsAdjusted && c.AdjustedClose > 0 {
		return c.AdjustedClose
	}
	return c.Close
}

func (c Candle) EffectiveVolume() float64 {
	if c.IsAdjusted && c.AdjustedVolume > 0 {
		return c.AdjustedVolume
	}
	return c.Volume
}

type Instrument struct {
	Symbol      string `json:"symbol"`
	Exchange    string `json:"exchange"`
	CompanyName string `json:"company_name"`
	Currency    string `json:"currency"`
	AssetType   string `json:"asset_type"`
}

type IndicatorSnapshot struct {
	SMA5                       float64            `json:"sma5"`
	SMA10                      float64            `json:"sma10"`
	SMA20                      float64            `json:"sma20"`
	SMA50                      float64            `json:"sma50"`
	SMA100                     float64            `json:"sma100"`
	SMA200                     float64            `json:"sma200"`
	EMA5                       float64            `json:"ema5"`
	EMA10                      float64            `json:"ema10"`
	EMA20                      float64            `json:"ema20"`
	EMA50                      float64            `json:"ema50"`
	EMA100                     float64            `json:"ema100"`
	EMA200                     float64            `json:"ema200"`
	RSI14                      float64            `json:"rsi14"`
	ATR14                      float64            `json:"atr14"`
	MACD                       float64            `json:"macd"`
	MACDSignal                 float64            `json:"macd_signal"`
	MACDHistogram              float64            `json:"macd_histogram"`
	BollingerUpper             float64            `json:"bollinger_upper"`
	BollingerMiddle            float64            `json:"bollinger_middle"`
	BollingerLower             float64            `json:"bollinger_lower"`
	ADX14                      float64            `json:"adx14"`
	VWAP                       float64            `json:"vwap"`
	OBV                        float64            `json:"obv"`
	MFI14                      float64            `json:"mfi14"`
	StochRSIK                  float64            `json:"stoch_rsi_k"`
	StochRSID                  float64            `json:"stoch_rsi_d"`
	StochasticK                float64            `json:"stochastic_k"`
	StochasticD                float64            `json:"stochastic_d"`
	CCI20                      float64            `json:"cci20"`
	WilliamsR14                float64            `json:"williams_r14"`
	ROC12                      float64            `json:"roc12"`
	Supertrend                 float64            `json:"supertrend"`
	SupertrendPrev             float64            `json:"supertrend_prev"`
	OBVSlope                   float64            `json:"obv_slope"`
	DonchianUpper              float64            `json:"donchian_upper"`
	DonchianLower              float64            `json:"donchian_lower"`
	KeltnerUpper               float64            `json:"keltner_upper"`
	KeltnerMiddle              float64            `json:"keltner_middle"`
	KeltnerLower               float64            `json:"keltner_lower"`
	VolumeSMA20                float64            `json:"volume_sma20"`
	ChaikinMoneyFlow20         float64            `json:"chaikin_money_flow20"`
	AccumulationDistribution   float64            `json:"accumulation_distribution"`
	IchimokuTenkan             float64            `json:"ichimoku_tenkan"`
	IchimokuKijun              float64            `json:"ichimoku_kijun"`
	IchimokuSenkouA            float64            `json:"ichimoku_senkou_a"`
	IchimokuSenkouB            float64            `json:"ichimoku_senkou_b"`
	IchimokuChikou             float64            `json:"ichimoku_chikou"`
	IchimokuCloudTrend         float64            `json:"ichimoku_cloud_trend"`
	IchimokuKumoTwist          float64            `json:"ichimoku_kumo_twist"`
	IchimokuTKCross            float64            `json:"ichimoku_tk_cross"`
	IchimokuPriceCloudBreakout float64            `json:"ichimoku_price_cloud_breakout"`
	PivotPoint                 float64            `json:"pivot_point"`
	PivotR1                    float64            `json:"pivot_r1"`
	PivotR2                    float64            `json:"pivot_r2"`
	PivotS1                    float64            `json:"pivot_s1"`
	PivotS2                    float64            `json:"pivot_s2"`
	FibonacciLevels            map[string]float64 `json:"fibonacci_levels"`
	AdditionalIndicators       map[string]float64 `json:"additional_indicators"`
	SupportTools               map[string]float64 `json:"support_tools"`
	MarketStructure            map[string]float64 `json:"market_structure"`
	RelativeStrength           map[string]float64 `json:"relative_strength"`
}

type PatternResult struct {
	Name                 string    `json:"name"`
	Category             string    `json:"category"`
	Direction            string    `json:"direction"`
	Confidence           float64   `json:"confidence"`
	RawConfidence        float64   `json:"raw_confidence,omitempty"`
	CalibratedConfidence float64   `json:"calibrated_confidence,omitempty"`
	SignalScore          float64   `json:"signal_score,omitempty"`
	SignalGroup          string    `json:"signal_group,omitempty"`
	Actionable           bool      `json:"actionable"`
	Resolution           string    `json:"resolution,omitempty"`
	ConflictStatus       string    `json:"conflict_status,omitempty"`
	ConsolidatedFrom     []string  `json:"consolidated_from,omitempty"`
	RejectionReasons     []string  `json:"rejection_reasons,omitempty"`
	VolumeConfirmation   string    `json:"volume_confirmation,omitempty"`
	ConfirmingIndicators []string  `json:"confirming_indicators,omitempty"`
	InvalidatingSignals  []string  `json:"invalidating_signals,omitempty"`
	TradeValue           string    `json:"trade_value,omitempty"`
	ValidationStatus     string    `json:"validation_status,omitempty"`
	ValidationMethod     string    `json:"validation_method,omitempty"`
	ValidationPValue     float64   `json:"validation_p_value,omitempty"`
	ValidationCILow      float64   `json:"validation_ci_low,omitempty"`
	ValidationCIHigh     float64   `json:"validation_ci_high,omitempty"`
	BacktestReady        bool      `json:"backtest_ready"`
	BacktestSampleSize   int       `json:"backtest_sample_size,omitempty"`
	BacktestWinRate      float64   `json:"backtest_win_rate,omitempty"`
	BacktestExpectancyR  float64   `json:"backtest_expectancy_r,omitempty"`
	CalibrationSource    string    `json:"calibration_source,omitempty"`
	StartIndex           int       `json:"start_index"`
	EndIndex             int       `json:"end_index"`
	StartTime            time.Time `json:"start_time"`
	EndTime              time.Time `json:"end_time"`
	Evidence             []string  `json:"evidence"`
	VolumeConfirmed      bool      `json:"volume_confirmed"`
	RuleVersion          string    `json:"rule_version"`
	SetupCompleteIndex   int       `json:"setup_complete_index"`
	TriggerIndex         int       `json:"trigger_index"`
	Trigger              string    `json:"trigger"`
	Tradeable            bool      `json:"tradeable"`
	EntryMin             float64   `json:"entry_min"`
	EntryMax             float64   `json:"entry_max"`
	StopLoss             float64   `json:"stop_loss"`
	Target1              float64   `json:"target1"`
	Target2              float64   `json:"target2"`
	InvalidationLevel    float64   `json:"invalidation_level"`
	RiskRewardRatio      float64   `json:"risk_reward_ratio"`
}

type PatternScanResult struct {
	Name                 string   `json:"name"`
	Category             string   `json:"category"`
	Group                string   `json:"group"`
	Direction            string   `json:"direction"`
	Matched              bool     `json:"matched"`
	Actionable           bool     `json:"actionable"`
	Confidence           float64  `json:"confidence"`
	CalibratedConfidence float64  `json:"calibrated_confidence,omitempty"`
	SignalScore          float64  `json:"signal_score,omitempty"`
	Resolution           string   `json:"resolution,omitempty"`
	ConflictStatus       string   `json:"conflict_status,omitempty"`
	RejectionReasons     []string `json:"rejection_reasons,omitempty"`
	VolumeConfirmation   string   `json:"volume_confirmation,omitempty"`
	ConfirmingIndicators []string `json:"confirming_indicators,omitempty"`
	InvalidatingSignals  []string `json:"invalidating_signals,omitempty"`
	TradeValue           string   `json:"trade_value,omitempty"`
	ValidationStatus     string   `json:"validation_status,omitempty"`
	ValidationMethod     string   `json:"validation_method,omitempty"`
	ValidationPValue     float64  `json:"validation_p_value,omitempty"`
	ValidationCILow      float64  `json:"validation_ci_low,omitempty"`
	ValidationCIHigh     float64  `json:"validation_ci_high,omitempty"`
	Source               string   `json:"source"`
	Evidence             []string `json:"evidence"`
}

type IndicatorResult struct {
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Group      string   `json:"group"`
	Signal     string   `json:"signal"`
	Value      float64  `json:"value"`
	Confidence float64  `json:"confidence"`
	Computed   bool     `json:"computed"`
	Source     string   `json:"source"`
	Evidence   []string `json:"evidence"`
}

type SupportResistanceLevel struct {
	Type          string    `json:"type"`
	Price         float64   `json:"price"`
	Strength      float64   `json:"strength"`
	TouchCount    int       `json:"touch_count"`
	LastTouchedAt time.Time `json:"last_touched_at"`
}

type TradePlan struct {
	Direction       string   `json:"direction"`
	EntryMin        float64  `json:"entry_min"`
	EntryMax        float64  `json:"entry_max"`
	TakeProfit1     float64  `json:"take_profit1"`
	TakeProfit2     float64  `json:"take_profit2"`
	StopLoss        float64  `json:"stop_loss"`
	RiskRewardRatio float64  `json:"risk_reward_ratio"`
	Quality         string   `json:"quality"`
	ConfidenceScore float64  `json:"confidence_score"`
	Rejected        bool     `json:"rejected"`
	RejectReason    string   `json:"reject_reason"`
	Reasoning       []string `json:"reasoning"`
}

func (p TradePlan) MarshalJSON() ([]byte, error) {
	type tradePlanAlias TradePlan
	out := tradePlanAlias(p)
	out.RejectReason = localize.Reason(out.RejectReason)
	if len(p.Reasoning) > 0 {
		out.Reasoning = append([]string(nil), p.Reasoning...)
		for i, reason := range out.Reasoning {
			out.Reasoning[i] = localize.Reason(reason)
		}
	}
	return json.Marshal(out)
}

func NormalizeSymbol(value string) string {
	symbol := strings.ToUpper(strings.TrimSpace(value))
	symbol = strings.TrimPrefix(symbol, "BIST:")
	symbol = strings.TrimSuffix(symbol, ".IS")
	symbol = strings.TrimSpace(symbol)
	return symbol
}

func NormalizeAssetType(value string) string {
	return domain.NormalizeAssetKind(value)
}

func IsCryptoAssetType(value string) bool {
	return domain.IsCryptoAssetKind(value)
}

func IsCommodityAssetType(value string) bool {
	return domain.IsCommodityAssetKind(value)
}

func InferAssetTypeFromSymbol(value string) string {
	symbol := NormalizeSymbol(value)
	if symbol == "" {
		return AssetTypeEquity
	}
	if IsCommoditySymbol(symbol) {
		return AssetTypeCommodity
	}
	exchange, pair := SplitExchangeSymbol(symbol)
	if _, _, ok := CanonicalCryptoPair(pair); ok {
		if exchange == "" || isKnownCryptoExchange(exchange) || strings.Contains(pair, "USD") || strings.Contains(pair, "BTC") {
			return AssetTypeCrypto
		}
	}
	if _, _, ok := CanonicalCryptoPair(symbol); ok {
		return AssetTypeCrypto
	}
	return AssetTypeEquity
}

func SplitExchangeSymbol(value string) (string, string) {
	symbol := NormalizeSymbol(value)
	parts := strings.SplitN(symbol, ":", 2)
	if len(parts) != 2 {
		return "", symbol
	}
	return strings.ToUpper(strings.TrimSpace(parts[0])), strings.ToUpper(strings.TrimSpace(parts[1]))
}

func CanonicalCryptoPair(value string) (string, string, bool) {
	_, pair := SplitExchangeSymbol(value)
	pair = strings.NewReplacer("/", "", "-", "", "_", "", " ", "").Replace(pair)
	pair = strings.ToUpper(strings.TrimSpace(pair))
	if pair == "" {
		return "", "", false
	}
	if pair == "XBT" {
		pair = "BTC"
	}
	if cryptoBaseNames[pair] != "" {
		return pair + "USDT", "USDT", true
	}
	for _, quote := range cryptoQuoteCurrencies {
		if strings.HasSuffix(pair, quote) && len(pair) > len(quote) {
			base := strings.TrimSuffix(pair, quote)
			if base == "XBT" {
				base = "BTC"
			}
			if cryptoBaseNames[base] != "" || isLikelyCryptoBase(base) {
				return base + quote, quote, true
			}
		}
	}
	return "", "", false
}

func CryptoDisplayName(pair string) string {
	pair, quote, ok := CanonicalCryptoPair(pair)
	if !ok {
		return NormalizeSymbol(pair)
	}
	base := strings.TrimSuffix(pair, quote)
	name := cryptoBaseNames[base]
	if name == "" {
		name = base
	}
	return name + " / " + quote
}

type commoditySpec struct {
	Symbol   string
	Currency string
	Name     string
	Exchange string
}

func CanonicalCommodityInstrument(value string) (Instrument, bool) {
	exchange, rawSymbol := SplitExchangeSymbol(value)
	key := commodityAliasKey(rawSymbol)
	spec, ok := commodityAliases[key]
	if !ok {
		return Instrument{}, false
	}
	if exchange == "" {
		exchange = spec.Exchange
	}
	return Instrument{
		Symbol:      spec.Symbol,
		Exchange:    exchange,
		CompanyName: spec.Name,
		Currency:    spec.Currency,
		AssetType:   AssetTypeCommodity,
	}, true
}

func IsCommoditySymbol(value string) bool {
	_, ok := CanonicalCommodityInstrument(value)
	return ok
}

func CommodityDisplayName(value string) string {
	if instrument, ok := CanonicalCommodityInstrument(value); ok {
		return instrument.CompanyName
	}
	return NormalizeSymbol(value)
}

func SymbolPathKey(value string) string {
	symbol := NormalizeSymbol(value)
	if symbol == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range symbol {
		ok := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

var cryptoQuoteCurrencies = []string{"USDT", "USDC", "USD", "TRY", "EUR", "BTC", "ETH"}

var cryptoBaseNames = map[string]string{
	"BTC":   "Bitcoin",
	"ETH":   "Ethereum",
	"BNB":   "BNB",
	"SOL":   "Solana",
	"XRP":   "XRP",
	"ADA":   "Cardano",
	"DOGE":  "Dogecoin",
	"AVAX":  "Avalanche",
	"DOT":   "Polkadot",
	"LINK":  "Chainlink",
	"MATIC": "Polygon",
	"TRX":   "TRON",
	"LTC":   "Litecoin",
	"BCH":   "Bitcoin Cash",
	"ATOM":  "Cosmos",
	"NEAR":  "NEAR",
	"APT":   "Aptos",
	"ARB":   "Arbitrum",
	"OP":    "Optimism",
	"FIL":   "Filecoin",
	"UNI":   "Uniswap",
	"AAVE":  "Aave",
}

var commodityAliases = map[string]commoditySpec{
	"XAU":    {Symbol: "XAUUSD", Currency: "USD", Name: "Gold Spot / USD", Exchange: "OANDA"},
	"GOLD":   {Symbol: "XAUUSD", Currency: "USD", Name: "Gold Spot / USD", Exchange: "OANDA"},
	"XAUUSD": {Symbol: "XAUUSD", Currency: "USD", Name: "Gold Spot / USD", Exchange: "OANDA"},
	"XAUTRY": {Symbol: "XAUTRY", Currency: "TRY", Name: "Gold Spot / TRY", Exchange: "FX_IDC"},
	"GC1!":   {Symbol: "GC1!", Currency: "USD", Name: "Gold Futures Continuous", Exchange: "COMEX"},
	"XAG":    {Symbol: "XAGUSD", Currency: "USD", Name: "Silver Spot / USD", Exchange: "OANDA"},
	"SILVER": {Symbol: "XAGUSD", Currency: "USD", Name: "Silver Spot / USD", Exchange: "OANDA"},
	"XAGUSD": {Symbol: "XAGUSD", Currency: "USD", Name: "Silver Spot / USD", Exchange: "OANDA"},
}

func commodityAliasKey(value string) string {
	key := strings.NewReplacer("/", "", "-", "", "_", "", " ", "").Replace(value)
	return strings.ToUpper(strings.TrimSpace(key))
}

func isKnownCommodityBase(base string) bool {
	switch strings.ToUpper(strings.TrimSpace(base)) {
	case "XAU", "XAG", "XPT", "XPD", "GC", "SI", "GOLD", "SILVER":
		return true
	default:
		return false
	}
}

func isKnownCryptoExchange(exchange string) bool {
	switch strings.ToUpper(strings.TrimSpace(exchange)) {
	case "BINANCE", "COINBASE", "BITSTAMP", "KRAKEN", "BYBIT", "OKX", "KUCOIN", "BITFINEX", "CRYPTO", "GEMINI":
		return true
	default:
		return false
	}
}

func isLikelyCryptoBase(base string) bool {
	if isKnownCommodityBase(base) {
		return false
	}
	if len(base) < 2 || len(base) > 8 {
		return false
	}
	hasLetter := false
	for _, r := range base {
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return hasLetter
}
