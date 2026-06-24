// internal/datasource/tradingview.go
package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"

	"github.com/gorilla/websocket"
)

const tradingViewEndpoint = "wss://data.tradingview.com/socket.io/websocket"

type TradingViewProvider struct {
	endpoint string
	exchange string
	timeout  time.Duration
}

func NewTradingViewProvider() *TradingViewProvider {
	return &TradingViewProvider{
		endpoint: tradingViewEndpoint,
		exchange: "BIST",
		timeout:  30 * time.Second,
	}
}

func (p *TradingViewProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	if err := ctx.Err(); err != nil {
		return ohlcv.Instrument{}, fmt.Errorf("tradingview sembol aramasi iptal edildi: %w", err)
	}
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return ohlcv.Instrument{}, fmt.Errorf("bos sembol: %w", ErrSymbolNotFound)
	}
	return tradingViewInstrument(normalized, p.exchange), nil
}

func (p *TradingViewProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	resolution, err := tradingViewResolution(timeframe)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 260
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	headers := http.Header{}
	headers.Set("Origin", "https://www.tradingview.com")
	headers.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, p.endpoint, headers)
	if err != nil {
		return nil, fmt.Errorf("tradingview baglantisi acilamadi: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	chartSession := randomSession("cs")
	seriesSymbolID := "sds_sym_1"
	seriesID := "sds_1"
	seriesCommandID := "s1"
	tvSymbol := tradingViewSymbol(instrument)
	resolvePayload, err := tradingViewResolvePayload(tvSymbol)
	if err != nil {
		return nil, err
	}

	if err := receiveTradingViewHello(ctx, conn); err != nil {
		return nil, err
	}

	commands := []tradingViewCommand{
		{"set_auth_token", []any{"unauthorized_user_token"}},
		{"chart_create_session", []any{chartSession, ""}},
		{"resolve_symbol", []any{chartSession, seriesSymbolID, resolvePayload}},
		{"create_series", []any{chartSession, seriesID, seriesCommandID, seriesSymbolID, resolution, limit, ""}},
	}
	for _, command := range commands {
		if err := sendTradingViewCommand(conn, command); err != nil {
			return nil, err
		}
	}

	var candles []ohlcv.Candle
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("tradingview ohlcv beklenirken zaman asimi: %w", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(p.timeout))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("tradingview mesaji okunamadi: %w", err)
		}

		frames, err := decodeTradingViewFrames(string(raw))
		if err != nil {
			return nil, err
		}
		for _, frame := range frames {
			if strings.HasPrefix(frame, "~h~") {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(encodeTradingViewFrame(frame))); err != nil {
					return nil, fmt.Errorf("tradingview kalp atisi yanitlanamadi: %w", err)
				}
				continue
			}

			message, err := parseTradingViewMessage(frame)
			if err != nil {
				continue
			}
			switch message.Method {
			case "timescale_update", "du":
				parsed, err := candlesFromTimescaleUpdate(message, seriesID)
				if err != nil {
					return nil, err
				}
				if len(parsed) > 0 {
					candles = parsed
				}
			case "series_completed":
				if len(candles) == 0 {
					return nil, fmt.Errorf("tradingview %s icin mum verisi dondurmedi: %w", tvSymbol, ErrSymbolNotFound)
				}
				return trimCandles(candles, limit), nil
			case "critical_error", "protocol_error", "symbol_error", "series_error":
				return nil, fmt.Errorf("tradingview protokol hatasi: %s", frame)
			}
		}
	}
}

type tradingViewCommand struct {
	method string
	params []any
}

type tradingViewMessage struct {
	Method string            `json:"m"`
	Params []json.RawMessage `json:"p"`
}

type tradingViewBar struct {
	Values []float64 `json:"v"`
}

func tradingViewSymbol(instrument ohlcv.Instrument) string {
	symbol := ohlcv.NormalizeSymbol(instrument.Symbol)
	exchange := strings.ToUpper(strings.TrimSpace(instrument.Exchange))
	if exchange == "" {
		exchange = "BIST"
	}
	if strings.Contains(symbol, ":") {
		return symbol
	}
	return exchange + ":" + symbol
}

func tradingViewInstrument(symbol string, defaultExchange string) ohlcv.Instrument {
	if instrument, ok := ohlcv.CanonicalCommodityInstrument(symbol); ok {
		return instrument
	}
	exchange, rawSymbol := ohlcv.SplitExchangeSymbol(symbol)
	if exchange == "" {
		exchange = strings.ToUpper(strings.TrimSpace(defaultExchange))
	}
	if exchange == "" {
		exchange = "BIST"
	}
	if instrument, ok := ohlcv.CanonicalCommodityInstrument(rawSymbol); ok {
		if exchange != "" && exchange != "BIST" {
			instrument.Exchange = exchange
		}
		return instrument
	}
	if pair, quote, ok := ohlcv.CanonicalCryptoPair(rawSymbol); ok && (exchange != "BIST" || ohlcv.InferAssetTypeFromSymbol(symbol) == ohlcv.AssetTypeCrypto) {
		if exchange == "" || exchange == "BIST" {
			exchange = "BINANCE"
		}
		return ohlcv.Instrument{
			Symbol:      pair,
			Exchange:    exchange,
			CompanyName: ohlcv.CryptoDisplayName(pair),
			Currency:    quote,
			AssetType:   ohlcv.AssetTypeCrypto,
		}
	}
	if pair, quote, ok := ohlcv.CanonicalCryptoPair(symbol); ok {
		return ohlcv.Instrument{
			Symbol:      pair,
			Exchange:    "BINANCE",
			CompanyName: ohlcv.CryptoDisplayName(pair),
			Currency:    quote,
			AssetType:   ohlcv.AssetTypeCrypto,
		}
	}
	return ohlcv.Instrument{
		Symbol:      rawSymbol,
		Exchange:    exchange,
		CompanyName: rawSymbol,
		Currency:    "TRY",
		AssetType:   ohlcv.AssetTypeEquity,
	}
}

func tradingViewResolvePayload(symbol string) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"adjustment": "splits",
		"symbol":     symbol,
	})
	if err != nil {
		return "", fmt.Errorf("tradingview sembol cozumu hazirlanamadi: %w", err)
	}
	return "=" + string(payload), nil
}

func tradingViewResolution(timeframe string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1D":
		return "D", nil
	case "1W":
		return "W", nil
	case "1M":
		return "M", nil
	case "3M":
		return "3M", nil
	case "6M":
		return "6M", nil
	case "1Y":
		return "12M", nil
	case "YTD", "ALL":
		return "D", nil
	default:
		return "", fmt.Errorf("zaman dilimi %q desteklenmiyor: %w", timeframe, ErrTimeframe)
	}
}

func receiveTradingViewHello(ctx context.Context, conn *websocket.Conn) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("tradingview ilk mesaji okunamadi: %w", err)
	}
	frames, err := decodeTradingViewFrames(string(raw))
	if err != nil {
		return err
	}
	for _, frame := range frames {
		if strings.HasPrefix(frame, "~h~") {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(encodeTradingViewFrame(frame))); err != nil {
				return fmt.Errorf("tradingview kalp atisi yanitlanamadi: %w", err)
			}
		}
	}
	return nil
}

func randomSession(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteByte('_')
	for i := 0; i < 12; i++ {
		b.WriteByte(letters[random.Intn(len(letters))])
	}
	return b.String()
}

func sendTradingViewCommand(conn *websocket.Conn, command tradingViewCommand) error {
	payload, err := json.Marshal(map[string]any{"m": command.method, "p": command.params})
	if err != nil {
		return fmt.Errorf("tradingview komutu hazirlanamadi: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(encodeTradingViewFrame(string(payload)))); err != nil {
		return fmt.Errorf("tradingview komutu gonderilemedi: %w", err)
	}
	return nil
}

func encodeTradingViewFrame(payload string) string {
	return fmt.Sprintf("~m~%d~m~%s", len(payload), payload)
}

func decodeTradingViewFrames(raw string) ([]string, error) {
	if !strings.Contains(raw, "~m~") {
		return []string{raw}, nil
	}
	var frames []string
	for len(raw) > 0 {
		if !strings.HasPrefix(raw, "~m~") {
			next := strings.Index(raw, "~m~")
			if next < 0 {
				break
			}
			raw = raw[next:]
		}
		raw = strings.TrimPrefix(raw, "~m~")
		separator := strings.Index(raw, "~m~")
		if separator < 0 {
			return nil, fmt.Errorf("tradingview cercevesi bozuk: uzunluk ayraci yok")
		}
		length, err := strconv.Atoi(raw[:separator])
		if err != nil {
			return nil, fmt.Errorf("tradingview cercevesi bozuk: %w", err)
		}
		raw = raw[separator+3:]
		if length > len(raw) {
			return nil, fmt.Errorf("tradingview cercevesi bozuk: mesaj eksik")
		}
		frames = append(frames, raw[:length])
		raw = raw[length:]
	}
	return frames, nil
}

func parseTradingViewMessage(frame string) (tradingViewMessage, error) {
	var message tradingViewMessage
	if err := json.Unmarshal([]byte(frame), &message); err != nil {
		return tradingViewMessage{}, err
	}
	return message, nil
}

func candlesFromTimescaleUpdate(message tradingViewMessage, seriesID string) ([]ohlcv.Candle, error) {
	if len(message.Params) < 2 {
		return nil, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(message.Params[1], &payload); err != nil {
		return nil, fmt.Errorf("tradingview mum verisi cozumlenemedi: %w", err)
	}
	seriesRaw, ok := payload[seriesID]
	if !ok {
		for _, candidate := range payload {
			seriesRaw = candidate
			ok = true
			break
		}
	}
	if !ok {
		return nil, nil
	}
	candles, err := candlesFromSeriesRaw(seriesRaw)
	if err != nil {
		return nil, err
	}
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Time.Before(candles[j].Time)
	})
	return candles, nil
}

func candlesFromSeriesRaw(raw json.RawMessage) ([]ohlcv.Candle, error) {
	var series map[string]json.RawMessage
	if err := json.Unmarshal(raw, &series); err != nil {
		return nil, fmt.Errorf("tradingview seri verisi cozumlenemedi: %w", err)
	}
	barsRaw, ok := series["s"]
	if !ok {
		return nil, nil
	}
	var bars []tradingViewBar
	if err := json.Unmarshal(barsRaw, &bars); err != nil {
		return nil, nil
	}
	return candlesFromBars(bars), nil
}

func candlesFromBars(bars []tradingViewBar) []ohlcv.Candle {
	candles := make([]ohlcv.Candle, 0, len(bars))
	for _, bar := range bars {
		if len(bar.Values) < 5 {
			continue
		}
		volume := 0.0
		if len(bar.Values) >= 6 {
			volume = bar.Values[5]
		}
		candles = append(candles, ohlcv.Candle{
			Time:   time.Unix(int64(bar.Values[0]), 0).UTC(),
			Open:   bar.Values[1],
			High:   bar.Values[2],
			Low:    bar.Values[3],
			Close:  bar.Values[4],
			Volume: volume,
		})
	}
	return candles
}

func trimCandles(candles []ohlcv.Candle, limit int) []ohlcv.Candle {
	if limit > 0 && len(candles) > limit {
		return candles[len(candles)-limit:]
	}
	return candles
}
