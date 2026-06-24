package tradingview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
	"hissebot/internal/wsclient"
)

const tradingViewChartURL = "wss://data.tradingview.com/socket.io/websocket"
const socketChartChunkBars = 100

var DefaultChartIntervals = []string{"5", "30", "60", "180", "D", "M"}

type ChartSyncOptions struct {
	Ticker        string
	Intervals     []string
	Bars          int
	Limit         int
	OnlyWithOHLCV bool
	Transport     string
}

type ChartFile struct {
	Source        string         `json:"source"`
	Ticker        string         `json:"ticker"`
	Symbol        string         `json:"symbol"`
	Interval      string         `json:"interval"`
	Transport     string         `json:"transport,omitempty"`
	BarsRequested int            `json:"bars_requested"`
	FetchedAt     time.Time      `json:"fetched_at"`
	Candles       []ChartCandle  `json:"candles"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type ChartCandle struct {
	Time   int64    `json:"time"`
	Open   *float64 `json:"open,omitempty"`
	High   *float64 `json:"high,omitempty"`
	Low    *float64 `json:"low,omitempty"`
	Close  *float64 `json:"close,omitempty"`
	Volume *float64 `json:"volume,omitempty"`
}

func SyncCharts(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts ChartSyncOptions) error {
	if len(opts.Intervals) == 0 {
		opts.Intervals = DefaultChartIntervals
	}

	equities, err := chartTargets(store, opts)
	if err != nil {
		return err
	}
	if opts.Limit > 0 && len(equities) > opts.Limit {
		equities = equities[:opts.Limit]
	}

	total := 0
	for _, equity := range equities {
		for _, interval := range opts.Intervals {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			interval = normalizeInterval(interval)
			if interval == "" {
				continue
			}
			chart, err := fetchChart(ctx, cfg, equity.Ticker, interval, opts.Bars, opts.Transport)
			if err != nil {
				fmt.Printf("tradingview chart: %s %s skipped: %v\n", equity.Ticker, interval, err)
				continue
			}
			if len(chart.Candles) == 0 {
				continue
			}
			path := store.ChartPath(equity.Ticker, interval)
			if err := util.WriteJSON(path, chart); err != nil {
				return err
			}
			ref := chartRef(path, interval, chart)
			if err := store.Update(equity.Ticker, func(e *domain.Equity) error {
				if e.ChartData == nil {
					e.ChartData = map[string]domain.ChartDataRef{}
				}
				e.ChartData[interval] = ref
				return nil
			}); err != nil {
				return err
			}
			total++
		}
	}

	fmt.Printf("tradingview charts: %d chart files updated for %d equities\n", total, len(equities))
	return nil
}

func chartTargets(store *storage.EquityStore, opts ChartSyncOptions) ([]*domain.Equity, error) {
	if opts.Ticker != "" {
		equity, err := store.Load(opts.Ticker)
		if err != nil {
			return nil, err
		}
		if equity.Ticker == "" {
			equity.Ticker = storage.NormalizeTicker(opts.Ticker)
		}
		return []*domain.Equity{equity}, nil
	}

	equities, err := store.List()
	if err != nil {
		return nil, err
	}
	targets := make([]*domain.Equity, 0, len(equities))
	for _, equity := range equities {
		if equity.AssetType != 2 {
			continue
		}
		if opts.OnlyWithOHLCV && equity.OHLCV == nil {
			continue
		}
		targets = append(targets, equity)
	}
	return targets, nil
}

func fetchChart(ctx context.Context, cfg config.Config, ticker string, interval string, bars int, transport string) (ChartFile, error) {
	transport = normalizeTransport(transport)
	if transport == "" {
		transport = normalizeTransport(cfg.TradingViewChartTransport)
	}
	switch transport {
	case "http":
		return fetchHTTPChart(ctx, cfg, ticker, interval, bars)
	case "socket":
		return fetchSocketChart(ctx, ticker, interval, bars)
	default:
		var httpErr error
		if strings.TrimSpace(cfg.TradingViewHistoryURL) != "" {
			chart, err := fetchHTTPChart(ctx, cfg, ticker, interval, bars)
			if err == nil {
				return chart, nil
			}
			httpErr = err
		}
		chart, err := fetchSocketChart(ctx, ticker, interval, bars)
		if err == nil {
			if httpErr != nil {
				if chart.Meta == nil {
					chart.Meta = map[string]any{}
				}
				chart.Meta["http_fallback_error"] = httpErr.Error()
			}
			return chart, nil
		}
		if httpErr != nil {
			return ChartFile{}, fmt.Errorf("http failed: %v; socket failed: %w", httpErr, err)
		}
		return ChartFile{}, err
	}
}

func fetchHTTPChart(ctx context.Context, cfg config.Config, ticker string, interval string, bars int) (ChartFile, error) {
	if strings.TrimSpace(cfg.TradingViewHistoryURL) == "" {
		return ChartFile{}, errors.New("HISSEBOT_TV_HISTORY_URL is required for HTTP chart transport")
	}
	endpoint, err := url.Parse(cfg.TradingViewHistoryURL)
	if err != nil {
		return ChartFile{}, err
	}
	symbol := chartSymbol(ticker, interval)
	to := time.Now().UTC().Unix()
	from := int64(0)
	if bars > 0 {
		from = to - chartLookbackSeconds(interval, bars)
	}
	query := endpoint.Query()
	query.Set("symbol", symbol)
	query.Set("resolution", chartResolution(interval))
	query.Set("from", strconv.FormatInt(from, 10))
	query.Set("to", strconv.FormatInt(to, 10))
	if bars > 0 {
		query.Set("countback", strconv.Itoa(bars))
	}
	query.Set("adjustment", "splits")
	query.Set("session", "regular")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ChartFile{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://www.tradingview.com/")
	req.Header.Set("User-Agent", "hissebot-go/1.0")

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return ChartFile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ChartFile{}, fmt.Errorf("http history status %d", resp.StatusCode)
	}

	var history struct {
		Status string    `json:"s"`
		Times  []int64   `json:"t"`
		Open   []float64 `json:"o"`
		High   []float64 `json:"h"`
		Low    []float64 `json:"l"`
		Close  []float64 `json:"c"`
		Volume []float64 `json:"v"`
		Error  string    `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return ChartFile{}, err
	}
	if strings.EqualFold(history.Status, "no_data") {
		return ChartFile{}, errors.New("http history returned no_data")
	}
	if history.Status != "" && !strings.EqualFold(history.Status, "ok") {
		if history.Error != "" {
			return ChartFile{}, fmt.Errorf("http history %s: %s", history.Status, history.Error)
		}
		return ChartFile{}, fmt.Errorf("http history status %s", history.Status)
	}

	count := minInt(len(history.Times), len(history.Open), len(history.High), len(history.Low), len(history.Close))
	candles := make([]ChartCandle, 0, count)
	for i := 0; i < count; i++ {
		open := history.Open[i]
		high := history.High[i]
		low := history.Low[i]
		closeValue := history.Close[i]
		candle := ChartCandle{
			Time:  history.Times[i],
			Open:  &open,
			High:  &high,
			Low:   &low,
			Close: &closeValue,
		}
		if i < len(history.Volume) {
			volume := history.Volume[i]
			candle.Volume = &volume
		}
		candles = append(candles, candle)
	}
	if len(candles) == 0 {
		return ChartFile{}, errors.New("http history returned empty candles")
	}

	return ChartFile{
		Source:        "tradingview",
		Ticker:        storage.NormalizeTicker(ticker),
		Symbol:        symbol,
		Interval:      interval,
		Transport:     "http",
		BarsRequested: bars,
		FetchedAt:     time.Now().UTC(),
		Candles:       candles,
		Meta: map[string]any{
			"endpoint": endpoint.Scheme + "://" + endpoint.Host + endpoint.Path,
			"from":     from,
			"to":       to,
			"status":   history.Status,
			"all_time": bars <= 0,
		},
	}, nil
}

func fetchSocketChart(ctx context.Context, ticker string, interval string, bars int) (ChartFile, error) {
	session := "cs_" + randomHex(6)
	seriesID := "s1"
	symbolID := "symbol_1"
	symbol := chartSymbol(ticker, interval)
	fetchedAt := time.Now().UTC()
	initialBars := socketChartChunkBars
	if bars > 0 {
		initialBars = minInt(bars, socketChartChunkBars)
	}
	if initialBars <= 0 {
		initialBars = socketChartChunkBars
	}

	conn, err := wsclient.DialHeaders(ctx, tradingViewChartURL, map[string]string{
		"Origin": "https://data.tradingview.com",
	})
	if err != nil {
		return ChartFile{}, err
	}
	defer conn.Close()

	messages := []string{
		tvMessage("set_auth_token", []any{"unauthorized_user_token"}),
		tvMessage("chart_create_session", []any{session, ""}),
		tvMessage("switch_timezone", []any{session, "exchange"}),
		tvMessage("resolve_symbol", []any{
			session,
			symbolID,
			"=" + mustJSON(map[string]any{
				"symbol":     symbol,
				"adjustment": "splits",
				"session":    "regular",
			}),
		}),
		tvMessage("create_series", []any{session, seriesID, seriesID, symbolID, chartResolution(interval), initialBars}),
	}
	for _, msg := range messages {
		if err := conn.WriteText(msg); err != nil {
			return ChartFile{}, err
		}
	}

	timeoutBars := bars
	if timeoutBars <= 0 {
		timeoutBars = 5000
	}
	readTimeout := 20*time.Second + time.Duration((timeoutBars/socketChartChunkBars)+1)*2*time.Second
	if readTimeout > 3*time.Minute {
		readTimeout = 3 * time.Minute
	}
	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	var bestCandles []ChartCandle
	var bestMeta map[string]any
	pendingMore := false
	requestedBars := initialBars
	unchangedUpdates := 0
	for {
		messages, err := readTVMessages(readCtx, conn)
		if err != nil {
			if len(bestCandles) > 0 {
				return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
			}
			return ChartFile{}, err
		}
		for _, msg := range messages {
			switch msg.Method {
			case "timescale_update":
				candles, meta, ok := candlesFromTimescale(msg.Params, seriesID)
				if ok {
					pendingMore = false
					if updateBestChart(&bestCandles, &bestMeta, candles, meta) {
						unchangedUpdates = 0
					} else {
						unchangedUpdates++
					}
					if shouldReturnChart(bars, bestCandles, requestedBars, unchangedUpdates) {
						return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
					}
					if !pendingMore {
						next := nextSocketChunkSize(bars, len(bestCandles))
						if next <= 0 {
							return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
						}
						if err := conn.WriteText(tvMessage("request_more_data", []any{session, seriesID, next})); err != nil {
							return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
						}
						pendingMore = true
						requestedBars += next
					}
				}
			case "du":
				candles, meta, ok := candlesFromTimescale(msg.Params, seriesID)
				if ok {
					pendingMore = false
					meta["partial"] = true
					if updateBestChart(&bestCandles, &bestMeta, candles, meta) {
						unchangedUpdates = 0
					} else {
						unchangedUpdates++
					}
					if shouldReturnChart(bars, bestCandles, requestedBars, unchangedUpdates) {
						return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
					}
					if !pendingMore {
						next := nextSocketChunkSize(bars, len(bestCandles))
						if next <= 0 {
							return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
						}
						if err := conn.WriteText(tvMessage("request_more_data", []any{session, seriesID, next})); err != nil {
							return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
						}
						pendingMore = true
						requestedBars += next
					}
				}
			case "series_error", "symbol_error", "critical_error", "protocol_error":
				if msg.Method == "critical_error" && strings.Contains(string(msg.RawParams), "request_more_data") && len(bestCandles) > 0 {
					return socketChartFile(ticker, symbol, interval, bars, fetchedAt, bestCandles, bestMeta), nil
				}
				return ChartFile{}, fmt.Errorf("tradingview %s: %s", msg.Method, string(msg.RawParams))
			}
		}
	}
}

func socketChartFile(ticker string, symbol string, interval string, bars int, fetchedAt time.Time, candles []ChartCandle, meta map[string]any) ChartFile {
	if meta == nil {
		meta = map[string]any{}
	}
	meta["all_time"] = bars <= 0
	meta["bars_available"] = len(candles)
	if bars > 0 && len(candles) < bars {
		meta["partial"] = true
	}
	return ChartFile{
		Source:        "tradingview",
		Ticker:        storage.NormalizeTicker(ticker),
		Symbol:        symbol,
		Interval:      interval,
		Transport:     "socket",
		BarsRequested: bars,
		FetchedAt:     fetchedAt,
		Candles:       candles,
		Meta:          meta,
	}
}

func updateBestChart(bestCandles *[]ChartCandle, bestMeta *map[string]any, candles []ChartCandle, meta map[string]any) bool {
	merged := mergeChartCandles(*bestCandles, candles)
	if len(merged) <= len(*bestCandles) {
		return false
	}
	*bestCandles = merged
	*bestMeta = meta
	return true
}

func mergeChartCandles(existing []ChartCandle, incoming []ChartCandle) []ChartCandle {
	if len(existing) == 0 {
		out := append([]ChartCandle(nil), incoming...)
		sort.Slice(out, func(i, j int) bool {
			return out[i].Time < out[j].Time
		})
		return out
	}
	if len(incoming) == 0 {
		return existing
	}
	byTime := make(map[int64]ChartCandle, len(existing)+len(incoming))
	for _, candle := range existing {
		byTime[candle.Time] = candle
	}
	for _, candle := range incoming {
		byTime[candle.Time] = candle
	}
	out := make([]ChartCandle, 0, len(byTime))
	for _, candle := range byTime {
		out = append(out, candle)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Time < out[j].Time
	})
	return out
}

func shouldReturnChart(bars int, candles []ChartCandle, requestedBars int, unchangedUpdates int) bool {
	if len(candles) == 0 {
		return false
	}
	if bars <= 0 {
		return unchangedUpdates >= 2
	}
	if len(candles) >= bars {
		return true
	}
	if requestedBars >= bars && unchangedUpdates > 0 {
		return true
	}
	return unchangedUpdates >= 2
}

func nextSocketChunkSize(bars int, currentCandles int) int {
	if bars <= 0 {
		return socketChartChunkBars
	}
	return minInt(socketChartChunkBars, bars-currentCandles)
}

func waitForSymbolResolved(ctx context.Context, conn *wsclient.Conn, session string, symbolID string) error {
	for {
		messages, err := readTVMessages(ctx, conn)
		if err != nil {
			return err
		}
		for _, msg := range messages {
			switch msg.Method {
			case "symbol_resolved":
				if symbolResolved(msg.Params, session, symbolID) {
					return nil
				}
			case "symbol_error", "critical_error", "protocol_error":
				return fmt.Errorf("tradingview %s: %s", msg.Method, string(msg.RawParams))
			}
		}
	}
}

func readTVMessages(ctx context.Context, conn *wsclient.Conn) ([]tvIncomingMessage, error) {
	raw, err := conn.ReadText(ctx)
	if err != nil {
		return nil, err
	}
	messages := []tvIncomingMessage{}
	for _, payload := range tvPayloads(raw) {
		if os.Getenv("HISSEBOT_TV_DEBUG") == "1" {
			fmt.Printf("tv debug: %s\n", payload)
		}
		if strings.HasPrefix(payload, "~h~") {
			continue
		}
		if payload == "m~~h~" {
			_ = conn.WriteText(tvPackRaw(payload))
			continue
		}
		msg, err := parseTVMessage(payload)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func symbolResolved(params []json.RawMessage, session string, symbolID string) bool {
	if len(params) < 2 {
		return false
	}
	var gotSession string
	var gotSymbol string
	_ = json.Unmarshal(params[0], &gotSession)
	_ = json.Unmarshal(params[1], &gotSymbol)
	return gotSession == session && gotSymbol == symbolID
}

type tvIncomingMessage struct {
	Method    string            `json:"m"`
	Params    []json.RawMessage `json:"p"`
	RawParams json.RawMessage   `json:"-"`
}

func parseTVMessage(payload string) (tvIncomingMessage, error) {
	var raw struct {
		Method string            `json:"m"`
		Params []json.RawMessage `json:"p"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return tvIncomingMessage{}, err
	}
	rawParams, _ := json.Marshal(raw.Params)
	return tvIncomingMessage{Method: raw.Method, Params: raw.Params, RawParams: rawParams}, nil
}

func candlesFromTimescale(params []json.RawMessage, seriesID string) ([]ChartCandle, map[string]any, bool) {
	if len(params) < 2 {
		return nil, nil, false
	}
	var update map[string]struct {
		Status string `json:"status"`
		Node   string `json:"node"`
		Series []struct {
			Index int   `json:"i"`
			Value []any `json:"v"`
		} `json:"s"`
	}
	if err := json.Unmarshal(params[1], &update); err != nil {
		return nil, nil, false
	}
	series, ok := update[seriesID]
	if !ok || len(series.Series) == 0 {
		return nil, nil, false
	}
	candles := make([]ChartCandle, 0, len(series.Series))
	for _, point := range series.Series {
		candle, ok := candleFromValues(point.Value)
		if ok {
			candles = append(candles, candle)
		}
	}
	meta := map[string]any{
		"status": series.Status,
		"node":   series.Node,
	}
	return candles, meta, len(candles) > 0
}

func candleFromValues(values []any) (ChartCandle, bool) {
	if len(values) < 5 {
		return ChartCandle{}, false
	}
	ts := intFromAny(values[0])
	if ts == nil {
		return ChartCandle{}, false
	}
	candle := ChartCandle{
		Time:  *ts,
		Open:  floatFromAny(values[1]),
		High:  floatFromAny(values[2]),
		Low:   floatFromAny(values[3]),
		Close: floatFromAny(values[4]),
	}
	if len(values) > 5 {
		candle.Volume = floatFromAny(values[5])
	}
	return candle, true
}

func chartRef(path string, interval string, chart ChartFile) domain.ChartDataRef {
	var first *int64
	var last *int64
	if len(chart.Candles) > 0 {
		f := chart.Candles[0].Time
		l := chart.Candles[len(chart.Candles)-1].Time
		first = &f
		last = &l
	}
	return domain.ChartDataRef{
		Source:    "tradingview",
		Interval:  interval,
		Path:      path,
		Bars:      len(chart.Candles),
		FirstTime: first,
		LastTime:  last,
		FetchedAt: chart.FetchedAt,
	}
}

func tvMessage(method string, params []any) string {
	body, _ := json.Marshal(map[string]any{"m": method, "p": params})
	return tvPackRaw(string(body))
}

func tvPackRaw(payload string) string {
	return fmt.Sprintf("~m~%d~m~%s", len([]byte(payload)), payload)
}

func tvPayloads(raw string) []string {
	if !strings.HasPrefix(raw, "~m~") {
		return []string{raw}
	}
	var payloads []string
	for strings.HasPrefix(raw, "~m~") {
		raw = strings.TrimPrefix(raw, "~m~")
		idx := strings.Index(raw, "~m~")
		if idx < 0 {
			break
		}
		length, err := strconv.Atoi(raw[:idx])
		if err != nil {
			break
		}
		raw = raw[idx+3:]
		if length < 0 || length > len(raw) {
			break
		}
		payloads = append(payloads, raw[:length])
		raw = raw[length:]
	}
	return payloads
}

func mustJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

func randomHex(size int) string {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(bytes)
}

func normalizeInterval(interval string) string {
	interval = strings.TrimSpace(strings.ToUpper(interval))
	switch interval {
	case "1D":
		return "D"
	case "1W":
		return "W"
	case "1M":
		return "M"
	default:
		return interval
	}
}

func chartResolution(interval string) string {
	switch normalizeInterval(interval) {
	case "D":
		return "1D"
	case "W":
		return "1W"
	case "M":
		return "1M"
	default:
		return normalizeInterval(interval)
	}
}

func chartLookbackSeconds(interval string, bars int) int64 {
	if bars <= 0 {
		bars = 5000
	}
	var seconds int64
	switch normalizeInterval(interval) {
	case "D":
		seconds = 24 * 60 * 60
	case "W":
		seconds = 7 * 24 * 60 * 60
	case "M":
		seconds = 31 * 24 * 60 * 60
	default:
		minutes, err := strconv.Atoi(normalizeInterval(interval))
		if err != nil || minutes <= 0 {
			minutes = 60
		}
		seconds = int64(minutes) * 60
	}
	return seconds * int64(bars) * 4
}

func normalizeTransport(transport string) string {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "http", "https":
		return "http"
	case "ws", "wss", "websocket", "socket":
		return "socket"
	case "", "auto":
		return "auto"
	default:
		return "auto"
	}
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func chartSymbol(ticker string, interval string) string {
	ticker = storage.NormalizeTicker(ticker)
	switch normalizeInterval(interval) {
	case "D", "W", "M":
		return "BIST_DLY:" + ticker
	default:
		return "BIST:" + ticker
	}
}

func ParseIntervals(input string) []string {
	if strings.TrimSpace(input) == "" {
		return DefaultChartIntervals
	}
	parts := strings.Split(input, ",")
	intervals := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		interval := normalizeInterval(part)
		if interval == "" || seen[interval] {
			continue
		}
		seen[interval] = true
		intervals = append(intervals, interval)
	}
	return intervals
}
