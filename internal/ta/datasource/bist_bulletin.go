package datasource

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"hissebot/internal/ta/ohlcv"
)

// BISTBulletinProvider reads daily OHLCV data from BIST official bulletin files.
//
// Supported file types:
//   - CSV (2015-11 → present):  thb{YYYYMMDD}1.csv, semicolon-delimited, 2 header rows
//   - XLS (2000 → 2015-10):     tbult_s1.xls / g1{YYMMDD}.xls, multi-sheet Excel (BIFF8)
//   - XLSX:                      modern Excel bulletins with the same THB columns as CSV
//
// Directory layout: {root}/bulten_verileri/{year}/{month}/{YYYYMMDD}_s1/extracted/{file}
type BISTBulletinProvider struct {
	root     string
	initOnce sync.Once
	files    []bistBulletinFile // sorted by date ascending
	initErr  error

	cacheMu sync.Mutex
	cache   map[string][]ohlcv.Candle // symbol → full daily series (immutable once set)

	recordCacheMu sync.Mutex
	recordCache   map[string][]DailyBulletinRecord // symbol → full daily bulletin records (immutable once set)
}

type bistBulletinFile struct {
	path   string
	date   time.Time
	format string
}

// bistColSchema holds column indices resolved from the CSV/XLS header row.
type bistColSchema struct {
	ticker int
	open   int // -1 when absent (old XLS format pre-2005)
	low    int
	high   int
	close_ int
	volume int // TOPLAM ISLEM ADEDI (shares)
}

type bistRecordColSchema struct {
	ticker                   int
	instrumentName           int
	marketGroup              int
	market                   int
	instrumentGroup          int
	instrumentType           int
	tradingMethod            int
	previousClose            int
	open                     int
	openingSessionPrice      int
	low                      int
	high                     int
	close_                   int
	closingSessionPrice      int
	changePct                int
	remainingBid             int
	remainingAsk             int
	vwap                     int
	valueTraded              int
	volume                   int
	tradeCount               int
	referencePrice           int
	openingSessionValue      int
	openingSessionVolume     int
	openingSessionTradeCount int
	closingSessionValue      int
	closingSessionVolume     int
	closingSessionTradeCount int
}

const bistBulletinSubdir = "bulten_verileri"

// oldTLCutoff is when Turkey redenominated: 1 new TL = 1,000,000 old TL.
var oldTLCutoff = time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC)

// NewBISTBulletinProvider returns a provider that reads from bistUnprocessedDir
// (typically data/bist/unprocessed).
func NewBISTBulletinProvider(bistUnprocessedDir string) *BISTBulletinProvider {
	return &BISTBulletinProvider{
		root:        bistUnprocessedDir,
		cache:       make(map[string][]ohlcv.Candle),
		recordCache: make(map[string][]DailyBulletinRecord),
	}
}

func (p *BISTBulletinProvider) SearchSymbol(ctx context.Context, symbol string) (ohlcv.Instrument, error) {
	if err := ctx.Err(); err != nil {
		return ohlcv.Instrument{}, fmt.Errorf("bist bulletin search canceled: %w", err)
	}
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return ohlcv.Instrument{}, fmt.Errorf("empty symbol: %w", ErrSymbolNotFound)
	}
	return ohlcv.Instrument{
		Symbol:      normalized,
		Exchange:    "BIST",
		CompanyName: normalized,
		Currency:    "TRY",
		AssetType:   ohlcv.AssetTypeEquity,
	}, nil
}

func (p *BISTBulletinProvider) FetchOHLCV(ctx context.Context, instrument ohlcv.Instrument, timeframe string, limit int) ([]ohlcv.Candle, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("bist bulletin fetch canceled: %w", err)
	}
	if ohlcv.IsCryptoAssetType(instrument.AssetType) || ohlcv.IsCommodityAssetType(instrument.AssetType) {
		return nil, fmt.Errorf("bist bulletin only supports equity: %w", ErrSymbolNotFound)
	}

	p.initOnce.Do(func() { p.initErr = p.scanFiles() })
	if p.initErr != nil {
		return nil, fmt.Errorf("bist bulletin index: %w", p.initErr)
	}
	if len(p.files) == 0 {
		return nil, fmt.Errorf("bist bulletin: no data files under %s: %w", p.root, ErrSymbolNotFound)
	}

	symbol := ohlcv.NormalizeSymbol(instrument.Symbol)
	daily, err := p.fetchDailyCandles(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if len(daily) == 0 {
		return nil, fmt.Errorf("bist bulletin: no data for %s: %w", symbol, ErrSymbolNotFound)
	}

	candles := aggregateBulletinCandles(daily, timeframe)
	if limit > 0 && len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("bist bulletin: empty result for %s %s: %w", symbol, timeframe, ErrSymbolNotFound)
	}
	return candles, nil
}

func (p *BISTBulletinProvider) FetchDailyBulletinRecords(ctx context.Context, symbol string, limit int) ([]DailyBulletinRecord, error) {
	return p.FetchDailyBulletinRecordsRange(ctx, symbol, "", "", limit)
}

func (p *BISTBulletinProvider) FetchDailyBulletinRecordsRange(ctx context.Context, symbol, fromDate, toDate string, limit int) ([]DailyBulletinRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("bist bulletin records fetch canceled: %w", err)
	}
	p.initOnce.Do(func() { p.initErr = p.scanFiles() })
	if p.initErr != nil {
		return nil, fmt.Errorf("bist bulletin index: %w", p.initErr)
	}
	if len(p.files) == 0 {
		return nil, fmt.Errorf("bist bulletin: no data files under %s: %w", p.root, ErrSymbolNotFound)
	}
	normalized := ohlcv.NormalizeSymbol(symbol)
	if normalized == "" {
		return nil, fmt.Errorf("empty symbol: %w", ErrSymbolNotFound)
	}
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	if limit <= 0 && fromDate == "" && toDate == "" {
		return p.fetchAllDailyBulletinRecords(ctx, normalized)
	}

	capacity := 0
	if limit > 0 {
		capacity = limit
	}
	records := make([]DailyBulletinRecord, 0, capacity)
	for i := len(p.files) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fileDate := p.files[i].date.Format("2006-01-02")
		if toDate != "" && fileDate > toDate {
			continue
		}
		if fromDate != "" && fileDate < fromDate {
			break
		}
		record, ok := bistExtractDailyRecord(p.files[i], normalized)
		if !ok {
			continue
		}
		records = append(records, record)
		if limit > 0 && len(records) >= limit {
			break
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("bist bulletin: no official daily records for %s: %w", normalized, ErrSymbolNotFound)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].TradingDate < records[j].TradingDate })
	return records, nil
}

func (p *BISTBulletinProvider) fetchAllDailyBulletinRecords(ctx context.Context, symbol string) ([]DailyBulletinRecord, error) {
	p.recordCacheMu.Lock()
	if cached, ok := p.recordCache[symbol]; ok {
		p.recordCacheMu.Unlock()
		return append([]DailyBulletinRecord{}, cached...), nil
	}
	p.recordCacheMu.Unlock()

	records := make([]DailyBulletinRecord, 0, 512)
	for _, f := range p.files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, ok := bistExtractDailyRecord(f, symbol)
		if !ok {
			continue
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("bist bulletin: no official daily records for %s: %w", symbol, ErrSymbolNotFound)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].TradingDate < records[j].TradingDate })

	p.recordCacheMu.Lock()
	p.recordCache[symbol] = append([]DailyBulletinRecord{}, records...)
	p.recordCacheMu.Unlock()
	return records, nil
}

// scanFiles indexes all bulletin files (CSV + XLS + XLSX) sorted by date ascending.
func (p *BISTBulletinProvider) scanFiles() error {
	root := filepath.Join(p.root, bistBulletinSubdir)
	var files []bistBulletinFile
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		ext := strings.ToLower(filepath.Ext(base))

		switch ext {
		case ".csv":
			// Expect thb{YYYYMMDD}1.csv
			dateStr := bistDateFromTHBName(base)
			if dateStr == "" {
				return nil
			}
			t, err2 := time.Parse("20060102", dateStr)
			if err2 != nil {
				return nil
			}
			files = append(files, bistBulletinFile{path: path, date: t, format: "csv"})

		case ".xls":
			// Derive date from parent directory name {YYYYMMDD}_s1
			dateStr := bistDateFromDirPath(path)
			if dateStr == "" {
				return nil
			}
			t, err2 := time.Parse("20060102", dateStr)
			if err2 != nil {
				return nil
			}
			files = append(files, bistBulletinFile{path: path, date: t, format: "xls"})

		case ".xlsx":
			dateStr := bistDateFromDirPath(path)
			if dateStr == "" {
				dateStr = bistDateFromTHBName(base)
			}
			if dateStr == "" {
				return nil
			}
			t, err2 := time.Parse("20060102", dateStr)
			if err2 != nil {
				return nil
			}
			files = append(files, bistBulletinFile{path: path, date: t, format: "xlsx"})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk bist bulletin dir: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].date.Before(files[j].date) })
	p.files = files
	return nil
}

// bistDateFromTHBName extracts YYYYMMDD from thb202606191.csv/xlsx.
func bistDateFromTHBName(base string) string {
	s := strings.TrimPrefix(strings.ToLower(base), "thb")
	s = strings.TrimSuffix(s, filepath.Ext(s))
	if len(s) > 8 {
		s = s[:8]
	}
	if len(s) != 8 {
		return ""
	}
	return s
}

// bistDateFromDirPath extracts YYYYMMDD from …/{YYYYMMDD}_s1/extracted/file.xls
func bistDateFromDirPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, p := range parts {
		if len(p) >= 8 && (strings.Contains(p, "_s") || len(p) == 8) {
			candidate := p
			if idx := strings.Index(candidate, "_"); idx >= 8 {
				candidate = candidate[:idx]
			}
			if len(candidate) == 8 {
				if _, err := time.Parse("20060102", candidate); err == nil {
					return candidate
				}
			}
		}
	}
	return ""
}

// fetchDailyCandles returns the full sorted daily candle series for a symbol, cached.
func (p *BISTBulletinProvider) fetchDailyCandles(ctx context.Context, symbol string) ([]ohlcv.Candle, error) {
	p.cacheMu.Lock()
	if c, ok := p.cache[symbol]; ok {
		p.cacheMu.Unlock()
		return c, nil
	}
	p.cacheMu.Unlock()

	const workers = 16
	type result struct {
		candle ohlcv.Candle
		ok     bool
	}

	jobs := make(chan bistBulletinFile, len(p.files))
	results := make(chan result, len(p.files))

	for i := 0; i < workers; i++ {
		go func() {
			for f := range jobs {
				if ctx.Err() != nil {
					results <- result{}
					continue
				}
				c, ok := bistExtractCandle(f, symbol)
				results <- result{candle: c, ok: ok}
			}
		}()
	}
	for _, f := range p.files {
		jobs <- f
	}
	close(jobs)

	candles := make([]ohlcv.Candle, 0, 512)
	for range p.files {
		r := <-results
		if r.ok {
			candles = append(candles, r.candle)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sort.Slice(candles, func(i, j int) bool { return candles[i].Time.Before(candles[j].Time) })

	p.cacheMu.Lock()
	p.cache[symbol] = candles
	p.cacheMu.Unlock()
	return candles, nil
}

// bistExtractCandle dispatches to CSV or XLS reader.
func bistExtractCandle(f bistBulletinFile, symbol string) (ohlcv.Candle, bool) {
	switch f.format {
	case "xls":
		return bistExtractXLS(f, symbol)
	case "xlsx":
		return bistExtractXLSX(f, symbol)
	default:
		return bistExtractCSV(f, symbol)
	}
}

func bistExtractDailyRecord(f bistBulletinFile, symbol string) (DailyBulletinRecord, bool) {
	switch f.format {
	case "xls":
		return bistExtractXLSRecord(f, symbol)
	case "xlsx":
		return bistExtractXLSXRecord(f, symbol)
	default:
		return bistExtractCSVRecord(f, symbol)
	}
}

// ── CSV reader ────────────────────────────────────────────────────────────────

func bistExtractCSV(f bistBulletinFile, symbol string) (ohlcv.Candle, bool) {
	fh, err := os.Open(f.path)
	if err != nil {
		return ohlcv.Candle{}, false
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	scanner.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)

	// Row 1: Turkish column headers
	if !scanner.Scan() {
		return ohlcv.Candle{}, false
	}
	schema, ok := bistCSVSchema(scanner.Text())
	if !ok {
		return ohlcv.Candle{}, false
	}

	// Row 2: English headers — skip
	if !scanner.Scan() {
		return ohlcv.Candle{}, false
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !bistCSVLineMayContainSymbol(line, symbol) {
			continue
		}
		fields := strings.Split(line, ";")
		if len(fields) <= schema.volume {
			continue
		}
		rawTicker := strings.TrimSpace(fields[schema.ticker])
		if !bistTickerMatch(rawTicker, symbol) {
			continue
		}

		open := parseFloatField(fields, schema.open)
		low := parseFloatField(fields, schema.low)
		high := parseFloatField(fields, schema.high)
		closeP := parseFloatField(fields, schema.close_)
		vol := parseFloatField(fields, schema.volume)

		return bistCandle(f.date, open, low, high, closeP, vol)
	}
	return ohlcv.Candle{}, false
}

func bistCSVSchema(headerLine string) (bistColSchema, bool) {
	cols := strings.Split(headerLine, ";")
	idx := make(map[string]int, len(cols))
	for i, c := range cols {
		key := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(c, "  ", " ")))
		idx[key] = i
	}
	find := func(names ...string) int {
		for _, n := range names {
			if i, ok := idx[n]; ok {
				return i
			}
		}
		return -1
	}
	tickerIdx := find("ISLEM KODU", "ISLEM  KODU", "İŞLEM KODU")
	openIdx := find("ACILIS FIYATI", "AÇILIŞ FİYATI")
	lowIdx := find("EN DUSUK FIYAT", "EN DÜŞÜK FİYAT")
	highIdx := find("EN YUKSEK FIYAT", "EN YÜKSEK FİYAT")
	closeIdx := find("KAPANIS FIYATI", "KAPANIŞ FİYATI")
	volIdx := find("TOPLAM ISLEM ADEDI", "TOPLAM İŞLEM ADEDI")
	if tickerIdx < 0 || openIdx < 0 || lowIdx < 0 || highIdx < 0 || closeIdx < 0 || volIdx < 0 {
		return bistColSchema{}, false
	}
	return bistColSchema{ticker: tickerIdx, open: openIdx, low: lowIdx, high: highIdx, close_: closeIdx, volume: volIdx}, true
}

func bistExtractCSVRecord(f bistBulletinFile, symbol string) (DailyBulletinRecord, bool) {
	fh, err := os.Open(f.path)
	if err != nil {
		return DailyBulletinRecord{}, false
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	if !scanner.Scan() {
		return DailyBulletinRecord{}, false
	}
	schema, ok := bistCSVRecordSchema(scanner.Text())
	if !ok {
		return DailyBulletinRecord{}, false
	}
	if !scanner.Scan() {
		return DailyBulletinRecord{}, false
	}

	maxCol := bistRecordSchemaMax(schema)
	for scanner.Scan() {
		line := scanner.Text()
		if !bistCSVLineMayContainSymbol(line, symbol) {
			continue
		}
		fields := strings.Split(line, ";")
		if len(fields) <= maxCol {
			continue
		}
		rawTicker := strings.TrimSpace(fields[schema.ticker])
		if !bistTickerMatch(rawTicker, symbol) {
			continue
		}
		closeP := parseFloatField(fields, schema.close_)
		if closeP <= 0 {
			return DailyBulletinRecord{}, false
		}
		openP := parseFloatField(fields, schema.open)
		lowP := parseFloatField(fields, schema.low)
		highP := parseFloatField(fields, schema.high)
		if openP <= 0 {
			openP = closeP
		}
		if lowP <= 0 {
			lowP = math.Min(openP, closeP)
		}
		if highP <= 0 {
			highP = math.Max(openP, closeP)
		}
		record := DailyBulletinRecord{
			Symbol:                   symbol,
			TradingDate:              f.date.Format("2006-01-02"),
			InstrumentCode:           rawTicker,
			InstrumentName:           stringField(fields, schema.instrumentName),
			MarketGroup:              stringField(fields, schema.marketGroup),
			Market:                   stringField(fields, schema.market),
			InstrumentGroup:          stringField(fields, schema.instrumentGroup),
			InstrumentType:           stringField(fields, schema.instrumentType),
			TradingMethod:            stringField(fields, schema.tradingMethod),
			PreviousClose:            parseFloatField(fields, schema.previousClose),
			Open:                     openP,
			OpeningSessionPrice:      parseFloatField(fields, schema.openingSessionPrice),
			Low:                      lowP,
			High:                     highP,
			Close:                    closeP,
			ClosingSessionPrice:      parseFloatField(fields, schema.closingSessionPrice),
			ChangePct:                parseFloatField(fields, schema.changePct),
			RemainingBid:             parseFloatField(fields, schema.remainingBid),
			RemainingAsk:             parseFloatField(fields, schema.remainingAsk),
			VWAP:                     parseFloatField(fields, schema.vwap),
			ValueTraded:              parseFloatField(fields, schema.valueTraded),
			Volume:                   parseFloatField(fields, schema.volume),
			TradeCount:               parseFloatField(fields, schema.tradeCount),
			ReferencePrice:           parseFloatField(fields, schema.referencePrice),
			OpeningSessionValue:      parseFloatField(fields, schema.openingSessionValue),
			OpeningSessionVolume:     parseFloatField(fields, schema.openingSessionVolume),
			OpeningSessionTradeCount: parseFloatField(fields, schema.openingSessionTradeCount),
			ClosingSessionValue:      parseFloatField(fields, schema.closingSessionValue),
			ClosingSessionVolume:     parseFloatField(fields, schema.closingSessionVolume),
			ClosingSessionTradeCount: parseFloatField(fields, schema.closingSessionTradeCount),
			SourceFormat:             "csv",
			SourcePath:               f.path,
		}
		return record, true
	}
	return DailyBulletinRecord{}, false
}

// ── XLSX reader ───────────────────────────────────────────────────────────────

func bistExtractXLSX(f bistBulletinFile, symbol string) (ohlcv.Candle, bool) {
	record, ok := bistExtractXLSXRecord(f, symbol)
	if !ok {
		return ohlcv.Candle{}, false
	}
	return bistCandle(f.date, record.Open, record.Low, record.High, record.Close, record.Volume)
}

func bistExtractXLSXRecord(f bistBulletinFile, symbol string) (DailyBulletinRecord, bool) {
	book, err := excelize.OpenFile(f.path)
	if err != nil {
		return DailyBulletinRecord{}, false
	}
	defer book.Close()

	for _, sheet := range book.GetSheetList() {
		rows, err := book.GetRows(sheet)
		if err != nil || len(rows) == 0 {
			continue
		}
		record, ok := bistExtractXLSXRecordFromRows(rows, f, symbol)
		if ok {
			return record, true
		}
	}
	return DailyBulletinRecord{}, false
}

func bistExtractXLSXRecordFromRows(rows [][]string, f bistBulletinFile, symbol string) (DailyBulletinRecord, bool) {
	headerRow := -1
	var schema bistRecordColSchema
	for ri := 0; ri < len(rows) && ri <= 20; ri++ {
		s, ok := bistCSVRecordSchema(strings.Join(rows[ri], ";"))
		if ok {
			headerRow = ri
			schema = s
			break
		}
	}
	if headerRow < 0 {
		return DailyBulletinRecord{}, false
	}

	for ri := headerRow + 1; ri < len(rows); ri++ {
		row := rows[ri]
		if bistBulletinEnglishHeaderRow(row) {
			continue
		}
		rawTicker := strings.TrimSpace(stringField(row, schema.ticker))
		if !bistTickerMatch(rawTicker, symbol) {
			continue
		}
		closeP := parseFloatField(row, schema.close_)
		if closeP <= 0 {
			return DailyBulletinRecord{}, false
		}
		openP := parseFloatField(row, schema.open)
		lowP := parseFloatField(row, schema.low)
		highP := parseFloatField(row, schema.high)
		if openP <= 0 {
			openP = closeP
		}
		if lowP <= 0 {
			lowP = math.Min(openP, closeP)
		}
		if highP <= 0 {
			highP = math.Max(openP, closeP)
		}
		if f.date.Before(oldTLCutoff) && closeP > 500 {
			divisor := 1_000_000.0
			openP /= divisor
			lowP /= divisor
			highP /= divisor
			closeP /= divisor
		}
		return DailyBulletinRecord{
			Symbol:                   symbol,
			TradingDate:              f.date.Format("2006-01-02"),
			InstrumentCode:           rawTicker,
			InstrumentName:           stringField(row, schema.instrumentName),
			MarketGroup:              stringField(row, schema.marketGroup),
			Market:                   stringField(row, schema.market),
			InstrumentGroup:          stringField(row, schema.instrumentGroup),
			InstrumentType:           stringField(row, schema.instrumentType),
			TradingMethod:            stringField(row, schema.tradingMethod),
			PreviousClose:            parseFloatField(row, schema.previousClose),
			Open:                     openP,
			OpeningSessionPrice:      parseFloatField(row, schema.openingSessionPrice),
			Low:                      lowP,
			High:                     highP,
			Close:                    closeP,
			ClosingSessionPrice:      parseFloatField(row, schema.closingSessionPrice),
			ChangePct:                parseFloatField(row, schema.changePct),
			RemainingBid:             parseFloatField(row, schema.remainingBid),
			RemainingAsk:             parseFloatField(row, schema.remainingAsk),
			VWAP:                     parseFloatField(row, schema.vwap),
			ValueTraded:              parseFloatField(row, schema.valueTraded),
			Volume:                   parseFloatField(row, schema.volume),
			TradeCount:               parseFloatField(row, schema.tradeCount),
			ReferencePrice:           parseFloatField(row, schema.referencePrice),
			OpeningSessionValue:      parseFloatField(row, schema.openingSessionValue),
			OpeningSessionVolume:     parseFloatField(row, schema.openingSessionVolume),
			OpeningSessionTradeCount: parseFloatField(row, schema.openingSessionTradeCount),
			ClosingSessionValue:      parseFloatField(row, schema.closingSessionValue),
			ClosingSessionVolume:     parseFloatField(row, schema.closingSessionVolume),
			ClosingSessionTradeCount: parseFloatField(row, schema.closingSessionTradeCount),
			SourceFormat:             "xlsx",
			SourcePath:               f.path,
		}, true
	}
	return DailyBulletinRecord{}, false
}

func bistCSVRecordSchema(headerLine string) (bistRecordColSchema, bool) {
	cols := strings.Split(headerLine, ";")
	idx := make(map[string]int, len(cols))
	for i, c := range cols {
		key := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(c, "  ", " ")))
		idx[key] = i
	}
	find := func(names ...string) int {
		for _, n := range names {
			if i, ok := idx[n]; ok {
				return i
			}
		}
		return -1
	}
	schema := bistRecordColSchema{
		ticker:                   find("ISLEM KODU", "ISLEM  KODU", "İŞLEM KODU"),
		instrumentName:           find("BULTEN ADI", "BÜLTEN ADI", "INSTRUMENT NAME"),
		marketGroup:              find("PAZAR GRUBU", "MARKET SUB SEGMENT"),
		market:                   find("PAZAR", "MARKET SEGMENT", "PIYASA", "PİYASA"),
		instrumentGroup:          find("ENSTRUMAN GRUBU", "ENSTRÜMAN GRUBU", "INSTRUMENT GROUP"),
		instrumentType:           find("ENSTRUMAN TIPI", "ENSTRÜMAN TİPİ", "INSTRUMENT TYPE"),
		tradingMethod:            find("ISLEM YONTEMI", "İŞLEM YÖNTEMİ", "TRADING METHOD"),
		previousClose:            find("ONCEKI KAPANIS FIYATI", "ÖNCEKİ KAPANIŞ FİYATI", "PREVIOUS LAST PRICE"),
		open:                     find("ACILIS FIYATI", "AÇILIŞ FİYATI", "OPENING PRICE"),
		openingSessionPrice:      find("ACILIS SEANSI FIYATI", "AÇILIŞ SEANSI FİYATI", "OPENING SESSION PRICE"),
		low:                      find("EN DUSUK FIYAT", "EN DÜŞÜK FİYAT", "LOWEST PRICE"),
		high:                     find("EN YUKSEK FIYAT", "EN YÜKSEK FİYAT", "HIGHEST PRICE"),
		close_:                   find("KAPANIS FIYATI", "KAPANIŞ FİYATI", "CLOSING PRICE"),
		closingSessionPrice:      find("KAPANIS SEANSI FIYATI", "KAPANIŞ SEANSI FİYATI", "CLOSING SESSION PRICE"),
		changePct:                find("DEGISIM (%)", "DEĞİŞİM (%)", "CHANGE TO PREVIOUS CLOSING (%)"),
		remainingBid:             find("BEKLEYEN EN IYI ALIS", "BEKLEYEN EN İYİ ALIŞ", "REMAINING BID"),
		remainingAsk:             find("BEKLEYEN EN IYI SATIS", "BEKLEYEN EN İYİ SATIŞ", "REMAINING ASK"),
		vwap:                     find("A.O.F", "A.O.F.", "AOF", "VWAP"),
		valueTraded:              find("TOPLAM ISLEM HACMI", "TOPLAM İŞLEM HACMİ", "TOPLAM ISLEM TUTARI", "TOTAL TRADED VALUE"),
		volume:                   find("TOPLAM ISLEM ADEDI", "TOPLAM İŞLEM ADEDİ", "TOTAL TRADED VOLUME"),
		tradeCount:               find("TOPLAM SOZLESME SAYISI", "TOPLAM SÖZLEŞME SAYISI", "TOTAL NUMBER OF CONTRACTS"),
		referencePrice:           find("REFERANS FIYAT", "REFERANS FİYAT", "REFERENCE PRICE"),
		openingSessionValue:      find("ACILIS SEANSI ISLEM HACMI", "AÇILIŞ SEANSI İŞLEM HACMİ", "TRADED VALUE AT OPENING SESSION"),
		openingSessionVolume:     find("ACILIS SEANSI ISLEM MIKTARI", "AÇILIŞ SEANSI İŞLEM MİKTARI", "TRADED VOLUME AT OPENING SESSION"),
		openingSessionTradeCount: find("ACILIS SEANSI SOZLESME SAYISI", "AÇILIŞ SEANSI SÖZLEŞME SAYISI", "NUMBER OF CONTRACTS AT OPENING SESSION"),
		closingSessionValue:      find("KAPANIS SEANSI ISLEM HACMI", "KAPANIŞ SEANSI İŞLEM HACMİ", "TRADED VALUE AT CLOSING SESSION"),
		closingSessionVolume:     find("KAPANIS SEANSI ISLEM MIKTARI", "KAPANIŞ SEANSI İŞLEM MİKTARI", "TRADED VOLUME AT CLOSING SESSION"),
		closingSessionTradeCount: find("KAPANIS SEANSI SOZLESME SAYISI", "KAPANIŞ SEANSI SÖZLEŞME SAYISI", "NUMBER OF CONTRACTS AT CLOSING SESSION"),
	}
	if schema.ticker < 0 || schema.open < 0 || schema.low < 0 || schema.high < 0 || schema.close_ < 0 || schema.volume < 0 {
		return bistRecordColSchema{}, false
	}
	return schema, true
}

func bistRecordSchemaMax(schema bistRecordColSchema) int {
	maximum := -1
	for _, value := range []int{
		schema.ticker, schema.instrumentName, schema.marketGroup, schema.market, schema.instrumentGroup, schema.instrumentType,
		schema.tradingMethod, schema.previousClose, schema.open, schema.openingSessionPrice, schema.low, schema.high,
		schema.close_, schema.closingSessionPrice, schema.changePct, schema.remainingBid, schema.remainingAsk, schema.vwap,
		schema.valueTraded, schema.volume, schema.tradeCount, schema.referencePrice, schema.openingSessionValue,
		schema.openingSessionVolume, schema.openingSessionTradeCount, schema.closingSessionValue, schema.closingSessionVolume,
		schema.closingSessionTradeCount,
	} {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

// ── XLS reader ────────────────────────────────────────────────────────────────

func bistExtractXLS(f bistBulletinFile, symbol string) (ohlcv.Candle, bool) {
	book, err := xls.Open(f.path, "utf-8")
	if err != nil {
		return ohlcv.Candle{}, false
	}
	for si := 0; si < book.NumSheets(); si++ {
		sheet := book.GetSheet(si)
		if sheet == nil {
			continue
		}
		if c, ok := bistSearchXLSSheet(sheet, symbol, f.date); ok {
			return c, true
		}
	}
	return ohlcv.Candle{}, false
}

func bistExtractXLSRecord(f bistBulletinFile, symbol string) (DailyBulletinRecord, bool) {
	candle, ok := bistExtractXLS(f, symbol)
	if !ok {
		return DailyBulletinRecord{}, false
	}
	return DailyBulletinRecord{
		Symbol:       symbol,
		TradingDate:  f.date.Format("2006-01-02"),
		Open:         candle.Open,
		Low:          candle.Low,
		High:         candle.High,
		Close:        candle.Close,
		Volume:       candle.Volume,
		SourceFormat: "xls",
		SourcePath:   f.path,
	}, true
}

func bistSearchXLSSheet(sheet *xls.WorkSheet, symbol string, date time.Time) (ohlcv.Candle, bool) {
	maxRow := int(sheet.MaxRow)
	if maxRow < 12 {
		return ohlcv.Candle{}, false
	}

	// Locate header row (contains "KODU" or "AÇILIŞ") — usually row 11.
	headerRow := -1
	var schema bistColSchema
	for ri := 0; ri <= maxRow && ri <= 15; ri++ {
		row := xlsSafeRow(sheet, ri)
		if row == nil {
			continue
		}
		cols := xlsRowStrings(row)
		s, ok := bistXLSSchema(cols)
		if ok {
			headerRow = ri
			schema = s
			break
		}
	}
	if headerRow < 0 {
		return ohlcv.Candle{}, false
	}

	for ri := headerRow + 1; ri <= maxRow; ri++ {
		row := xlsSafeRow(sheet, ri)
		if row == nil {
			continue
		}
		cols := xlsRowStrings(row)
		if len(cols) <= schema.ticker {
			continue
		}
		rawTicker := strings.TrimSpace(cols[schema.ticker])
		if !bistTickerMatch(rawTicker, symbol) {
			continue
		}
		var openP float64
		if schema.open >= 0 && schema.open < len(cols) {
			openP, _ = strconv.ParseFloat(cols[schema.open], 64)
		}
		lowP, _ := strconv.ParseFloat(safeIdx(cols, schema.low), 64)
		highP, _ := strconv.ParseFloat(safeIdx(cols, schema.high), 64)
		closeP, _ := strconv.ParseFloat(safeIdx(cols, schema.close_), 64)
		vol, _ := strconv.ParseFloat(safeIdx(cols, schema.volume), 64)

		// Pre-2005 redenomination: 1 new TL = 1,000,000 old TL.
		if date.Before(oldTLCutoff) && closeP > 500 {
			divisor := 1_000_000.0
			openP /= divisor
			lowP /= divisor
			highP /= divisor
			closeP /= divisor
		}
		return bistCandle(date, openP, lowP, highP, closeP, vol)
	}
	return ohlcv.Candle{}, false
}

// bistXLSSchema identifies column positions from an XLS header row.
// Handles two historical layouts:
//   - Pre-2010 ("HİSSE SENEDİNİN KODU"): no AÇILIŞ column
//   - 2010-2015  ("PAYIN KODU"):          has AÇILIŞ column
func bistXLSSchema(cols []string) (bistColSchema, bool) {
	idx := make(map[string]int, len(cols))
	for i, c := range cols {
		key := xlsNormKey(c)
		idx[key] = i
	}
	find := func(keys ...string) int {
		for _, k := range keys {
			if i, ok := idx[k]; ok {
				return i
			}
		}
		return -1
	}

	tickerIdx := find("PAYINKODU", "HISSESENEDININKODU")
	lowIdx := find("ENDUSUKFIYAT")
	highIdx := find("ENYUKSEKFIYAT")
	closeIdx := find("KAPANIS", "KAPANISFIYATI")
	volIdx := find("TOPLAMISLEMADEDI")

	if tickerIdx < 0 || lowIdx < 0 || highIdx < 0 || closeIdx < 0 || volIdx < 0 {
		return bistColSchema{}, false
	}
	openIdx := find("ACILISFIYATI")
	return bistColSchema{
		ticker: tickerIdx,
		open:   openIdx,
		low:    lowIdx,
		high:   highIdx,
		close_: closeIdx,
		volume: volIdx,
	}, true
}

// xlsNormKey strips accents, spaces, newlines and uppercases for matching.
func xlsNormKey(s string) string {
	s = xlsDecodeCP1254(s)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ToUpper(s)
	// Normalize common Turkish characters to ASCII for robust matching.
	replacer := strings.NewReplacer(
		"İ", "I", "Ğ", "G", "Ş", "S", "Ç", "C", "Ü", "U", "Ö", "O",
	)
	return replacer.Replace(s)
}

func xlsDecodeCP1254(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	decoded, err := charmap.Windows1254.NewDecoder().Bytes([]byte(s))
	if err != nil {
		return s
	}
	return string(decoded)
}

func xlsSafeRow(sheet *xls.WorkSheet, index int) (row *xls.Row) {
	defer func() {
		if recover() != nil {
			row = nil
		}
	}()
	return sheet.Row(index)
}

func xlsRowStrings(row *xls.Row) []string {
	if row == nil {
		return nil
	}
	last := int(row.LastCol())
	out := make([]string, last)
	for i := range out {
		out[i] = xlsDecodeCP1254(strings.TrimSpace(row.Col(i)))
	}
	return out
}

// ── shared helpers ────────────────────────────────────────────────────────────

func bistTickerMatch(rawTicker, symbol string) bool {
	// Equity suffix is ".E"; ignore other suffixes (warrants, rights, etc.)
	base := rawTicker
	suffix := ""
	if dot := strings.LastIndex(rawTicker, "."); dot > 0 {
		base = rawTicker[:dot]
		suffix = rawTicker[dot+1:]
	}
	if !strings.EqualFold(base, symbol) {
		return false
	}
	// Accept .E or no suffix (bare ticker without dot).
	return suffix == "" || strings.EqualFold(suffix, "E")
}

func bistCSVLineMayContainSymbol(line, symbol string) bool {
	return strings.Contains(line, ";"+symbol+".") ||
		strings.Contains(line, ";"+symbol+";") ||
		strings.HasPrefix(line, symbol+".") ||
		strings.HasPrefix(line, symbol+";")
}

func bistBulletinEnglishHeaderRow(row []string) bool {
	if len(row) == 0 {
		return false
	}
	first := strings.ToUpper(strings.TrimSpace(row[0]))
	if first == "INSTRUMENT SERIES CODE" {
		return true
	}
	for _, cell := range row {
		key := strings.ToUpper(strings.TrimSpace(cell))
		if key == "OPENING PRICE" || key == "CLOSING PRICE" {
			return true
		}
	}
	return false
}

func bistCandle(date time.Time, open, low, high, close_, vol float64) (ohlcv.Candle, bool) {
	if close_ <= 0 {
		return ohlcv.Candle{}, false
	}
	if open <= 0 {
		open = close_ // old XLS format: use close as open
	}
	if low <= 0 {
		low = math.Min(open, close_)
	}
	if high <= 0 {
		high = math.Max(open, close_)
	}
	t := date.UTC()
	return ohlcv.Candle{
		Time:           t,
		Open:           open,
		High:           high,
		Low:            low,
		Close:          close_,
		Volume:         vol,
		AdjustedOpen:   open,
		AdjustedHigh:   high,
		AdjustedLow:    low,
		AdjustedClose:  close_,
		AdjustedVolume: vol,
		IsAdjusted:     false,
	}, true
}

func parseFloatField(fields []string, idx int) float64 {
	if idx < 0 || idx >= len(fields) {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(fields[idx], ",", ".")), 64)
	return v
}

func stringField(fields []string, idx int) string {
	if idx < 0 || idx >= len(fields) {
		return ""
	}
	return strings.TrimSpace(fields[idx])
}

func safeIdx(cols []string, idx int) string {
	if idx < 0 || idx >= len(cols) {
		return "0"
	}
	return cols[idx]
}

// ── timeframe aggregation ─────────────────────────────────────────────────────

func aggregateBulletinCandles(daily []ohlcv.Candle, timeframe string) []ohlcv.Candle {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1D", "YTD", "ALL", "3M", "6M", "1Y":
		return daily
	case "1W":
		return aggregateToWeekly(daily)
	case "1M":
		return aggregateToMonthly(daily)
	default:
		return daily
	}
}

func aggregateToWeekly(daily []ohlcv.Candle) []ohlcv.Candle {
	if len(daily) == 0 {
		return nil
	}
	var weeks []ohlcv.Candle
	var cur *ohlcv.Candle
	yr0, wk0 := daily[0].Time.ISOWeek()
	for i := range daily {
		c := &daily[i]
		yr, wk := c.Time.ISOWeek()
		if cur == nil || yr != yr0 || wk != wk0 {
			if cur != nil {
				weeks = append(weeks, *cur)
			}
			cp := *c
			cur = &cp
			yr0, wk0 = yr, wk
		} else {
			if c.High > cur.High {
				cur.High = c.High
			}
			if c.Low < cur.Low {
				cur.Low = c.Low
			}
			cur.Close = c.Close
			cur.Volume += c.Volume
		}
	}
	if cur != nil {
		weeks = append(weeks, *cur)
	}
	return weeks
}

func aggregateToMonthly(daily []ohlcv.Candle) []ohlcv.Candle {
	if len(daily) == 0 {
		return nil
	}
	var months []ohlcv.Candle
	var cur *ohlcv.Candle
	yr0, mo0 := daily[0].Time.Year(), daily[0].Time.Month()
	for i := range daily {
		c := &daily[i]
		yr, mo := c.Time.Year(), c.Time.Month()
		if cur == nil || yr != yr0 || mo != mo0 {
			if cur != nil {
				months = append(months, *cur)
			}
			cp := *c
			cur = &cp
			yr0, mo0 = yr, mo
		} else {
			if c.High > cur.High {
				cur.High = c.High
			}
			if c.Low < cur.Low {
				cur.Low = c.Low
			}
			cur.Close = c.Close
			cur.Volume += c.Volume
		}
	}
	if cur != nil {
		months = append(months, *cur)
	}
	return months
}
