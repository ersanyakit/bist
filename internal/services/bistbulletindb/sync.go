package bistbulletindb

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

// Sync imports BIST bulletin files from rawRoot into the SQLite store at dbPath.
// Files already processed (by source_key) are skipped unless Force=true.
func Sync(ctx context.Context, opts Options) (Report, error) {
	if opts.DBPath == "" {
		opts.DBPath = DefaultDBPath
	}
	if opts.RawRoot == "" {
		opts.RawRoot = DefaultRawRoot
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	store, err := OpenStore(opts.DBPath)
	if err != nil {
		return Report{}, fmt.Errorf("open bulletin store: %w", err)
	}
	defer store.Close()

	report := Report{
		GeneratedAtUTC: opts.Now().UTC(),
		DBPath:         opts.DBPath,
		RawRoot:        opts.RawRoot,
		FromYear:       opts.FromYear,
		ToYear:         opts.ToYear,
		Session:        opts.Session,
	}

	states, err := store.SourceStates(ctx)
	if err != nil {
		return report, fmt.Errorf("load source states: %w", err)
	}

	files, err := scanBulletinFiles(opts.RawRoot, opts.Session, opts.FromYear, opts.ToYear)
	if err != nil {
		return report, fmt.Errorf("scan bulletin files: %w", err)
	}
	report.LocalSourcesFound = len(files)

	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		sk := SourceKey(f.date, f.session)
		if state, ok := states[sk]; ok && state.Status == SourceStatusOK && !opts.Force {
			report.LocalSourcesSkipped++
			continue
		}

		src, records, err := parseBulletinFile(f, opts.Now())
		if err != nil {
			report.SourcesFailed++
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", sk, err))
			if markErr := store.MarkSource(ctx, src, SourceStatusErr); markErr != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("%s mark error: %v", sk, markErr))
			}
			continue
		}

		if err := store.SaveProcessedSource(ctx, src, records); err != nil {
			report.SourcesFailed++
			report.Errors = append(report.Errors, fmt.Sprintf("%s: store: %v", sk, err))
			continue
		}
		report.LocalSourcesImported++
		report.RowsSeen += src.RowsSeen
		report.RowsStored += src.RowsStored
		report.RowsAnalysisReady += src.RowsAnalysisReady
	}

	sources, candles, symbols, err := store.Counts(ctx)
	if err == nil {
		report.DatabaseSources = sources
		report.DatabaseCandles = candles
		report.DatabaseSymbols = symbols
	}
	return report, ctx.Err()
}

// ── file scanning ─────────────────────────────────────────────────────────────

type bulletinFileEntry struct {
	path    string
	date    time.Time
	session int
	format  string
}

func scanBulletinFiles(rawRoot string, sessionFilter, fromYear, toYear int) ([]bulletinFileEntry, error) {
	root := rawRoot
	if !strings.HasSuffix(filepath.ToSlash(rawRoot), "bulten_verileri") {
		root = filepath.Join(rawRoot, "bulten_verileri")
	}
	var files []bulletinFileEntry
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		ext := strings.ToLower(filepath.Ext(base))
		switch ext {
		case ".csv":
			d, sess, ok := bulletinDateFromCSV(path, base)
			if !ok || (sessionFilter > 0 && sess != sessionFilter) {
				return nil
			}
			if (fromYear > 0 && d.Year() < fromYear) || (toYear > 0 && d.Year() > toYear) {
				return nil
			}
			files = append(files, bulletinFileEntry{path: path, date: d, session: sess, format: "csv"})
		case ".xls":
			d, sess, ok := bulletinDateFromXLS(path)
			if !ok || (sessionFilter > 0 && sess != sessionFilter) {
				return nil
			}
			if (fromYear > 0 && d.Year() < fromYear) || (toYear > 0 && d.Year() > toYear) {
				return nil
			}
			files = append(files, bulletinFileEntry{path: path, date: d, session: sess, format: "xls"})
		case ".xlsx":
			d, sess, ok := bulletinDateFromXLS(path)
			if !ok {
				d, sess, ok = bulletinDateFromTHBFile(path, base)
			}
			if !ok || (sessionFilter > 0 && sess != sessionFilter) {
				return nil
			}
			if (fromYear > 0 && d.Year() < fromYear) || (toYear > 0 && d.Year() > toYear) {
				return nil
			}
			files = append(files, bulletinFileEntry{path: path, date: d, session: sess, format: "xlsx"})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].date.Before(files[j].date) })
	return files, nil
}

// bulletinDateFromCSV extracts (date, session) from thb{YYYYMMDD}{N}.csv
func bulletinDateFromCSV(path, base string) (time.Time, int, bool) {
	return bulletinDateFromTHBFile(path, base)
}

func bulletinDateFromTHBFile(_ string, base string) (time.Time, int, bool) {
	lower := strings.ToLower(base)
	if !strings.HasPrefix(lower, "thb") {
		return time.Time{}, 0, false
	}
	s := lower[3:]
	s = strings.TrimSuffix(s, filepath.Ext(s))
	if len(s) < 8 {
		return time.Time{}, 0, false
	}
	t, err := time.Parse("20060102", s[:8])
	if err != nil {
		return time.Time{}, 0, false
	}
	session := 1
	if suffix := s[8:]; suffix != "" {
		if n, e := strconv.Atoi(suffix); e == nil {
			session = n
		}
	}
	return t, session, true
}

// bulletinDateFromXLS extracts (date, session) from …/{YYYYMMDD}_s{N}/extracted/file.xls
func bulletinDateFromXLS(path string) (time.Time, int, bool) {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, p := range parts {
		idx := strings.Index(p, "_s")
		if idx < 8 {
			continue
		}
		dateStr := p[:idx]
		sesStr := p[idx+2:]
		if len(dateStr) != 8 {
			continue
		}
		t, err := time.Parse("20060102", dateStr)
		if err != nil {
			continue
		}
		session := 1
		if n, e := strconv.Atoi(sesStr); e == nil && n > 0 {
			session = n
		}
		return t, session, true
	}
	return time.Time{}, 0, false
}

// ── file parsing ──────────────────────────────────────────────────────────────

var oldTLCutoffDB = time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC)

func parseBulletinFile(f bulletinFileEntry, now time.Time) (SourceResult, []DailyRecord, error) {
	content, err := os.ReadFile(f.path)
	if err != nil {
		return SourceResult{SourceKey: SourceKey(f.date, f.session), TradingDate: f.date, Session: f.session, Error: err.Error(), CheckedAt: now}, nil, err
	}
	sum := sha256.Sum256(content)
	hexsum := hex.EncodeToString(sum[:])

	src := SourceResult{
		SourceKey:     SourceKey(f.date, f.session),
		TradingDate:   f.date,
		Session:       f.session,
		ContentSHA256: hexsum,
		SourceBytes:   int64(len(content)),
		CheckedAt:     now,
	}

	var records []DailyRecord
	var seen int
	switch f.format {
	case "xls":
		src.SourceFormat = "xls"
		records, seen, err = parseXLSAllRows(f.path, f.date)
	case "xlsx":
		src.SourceFormat = "xlsx"
		records, seen, err = parseXLSXAllRows(f.path, f.date)
	default:
		src.SourceFormat = "csv"
		records, seen, err = parseCSVAllRows(f.path, f.date)
	}
	if err != nil {
		src.Error = err.Error()
		return src, nil, err
	}

	src.RowsSeen = seen
	src.RowsStored = len(records)
	for _, r := range records {
		if r.AnalysisReady {
			src.RowsAnalysisReady++
		}
	}
	return src, records, nil
}

// ── CSV parsing (all rows) ────────────────────────────────────────────────────

type csvFullSchema struct {
	ticker         int
	instrumentCode int
	companyName    int
	open           int
	high           int
	low            int
	close_         int
	previousClose  int
	volume         int
	valueTraded    int
	tradeCount     int
	vwap           int
	market         int
}

func parseCSVAllRows(path string, date time.Time) ([]DailyRecord, int, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	if !scanner.Scan() {
		return nil, 0, fmt.Errorf("empty file or missing header row 1")
	}
	schema, ok := buildCSVFullSchema(scanner.Text())
	if !ok {
		return nil, 0, fmt.Errorf("unrecognized CSV header")
	}
	// Skip second header row (English)
	scanner.Scan()

	maxCol := maxInt(schema.ticker, schema.close_, schema.volume, schema.high, schema.low)

	var records []DailyRecord
	seen := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		seen++
		fields := strings.Split(line, ";")
		if len(fields) <= maxCol {
			continue
		}
		rawTicker := strings.TrimSpace(fields[schema.ticker])
		symbol, ok := extractEquitySymbol(rawTicker)
		if !ok {
			continue
		}

		closeP := parseFloatBulletin(safeField(fields, schema.close_))
		if closeP <= 0 {
			continue
		}
		openP := parseFloatBulletin(safeField(fields, schema.open))
		highP := parseFloatBulletin(safeField(fields, schema.high))
		lowP := parseFloatBulletin(safeField(fields, schema.low))
		vol := parseFloatBulletin(safeField(fields, schema.volume))

		if openP <= 0 {
			openP = closeP
		}
		if lowP <= 0 {
			lowP = math.Min(openP, closeP)
		}
		if highP <= 0 {
			highP = math.Max(openP, closeP)
		}

		rec := DailyRecord{
			Symbol:       symbol,
			TradingDate:  date.UTC(),
			High:         highP,
			Low:          lowP,
			Close:        closeP,
			Volume:       vol,
			SourceFormat: "csv",
		}
		v := openP
		rec.Open = &v
		if schema.instrumentCode >= 0 {
			rec.InstrumentCode = strings.TrimSpace(safeField(fields, schema.instrumentCode))
		}
		if schema.companyName >= 0 {
			rec.CompanyName = strings.TrimSpace(safeField(fields, schema.companyName))
		}
		if schema.previousClose >= 0 {
			rec.PreviousClose = parseFloatBulletin(safeField(fields, schema.previousClose))
		}
		if schema.valueTraded >= 0 {
			rec.ValueTraded = parseFloatBulletin(safeField(fields, schema.valueTraded))
		}
		if schema.tradeCount >= 0 {
			rec.TradeCount = int64(parseFloatBulletin(safeField(fields, schema.tradeCount)))
		}
		if schema.vwap >= 0 {
			rec.VWAP = parseFloatBulletin(safeField(fields, schema.vwap))
		}
		if schema.market >= 0 {
			rec.Market = strings.TrimSpace(safeField(fields, schema.market))
		}

		if date.Before(oldTLCutoffDB) && rec.Close > 500 {
			divisor := 1_000_000.0
			rec.Close /= divisor
			rec.High /= divisor
			rec.Low /= divisor
			rec.PreviousClose /= divisor
			rec.VWAP /= divisor
			if rec.Open != nil {
				vv := *rec.Open / divisor
				rec.Open = &vv
			}
			rec.QualityFlags = append(rec.QualityFlags, "old_tl_adjusted")
		}

		rec.AnalysisReady = rec.High > 0 && rec.Low > 0 && rec.Close > 0 &&
			rec.High >= rec.Close && rec.Close >= rec.Low
		records = append(records, rec)
	}
	return records, seen, scanner.Err()
}

// ── XLSX parsing (all rows) ───────────────────────────────────────────────────

func parseXLSXAllRows(path string, date time.Time) ([]DailyRecord, int, error) {
	book, err := excelize.OpenFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open xlsx %s: %w", path, err)
	}
	defer book.Close()

	var records []DailyRecord
	seen := 0
	foundHeader := false
	for _, sheet := range book.GetSheetList() {
		rows, err := book.GetRows(sheet)
		if err != nil || len(rows) == 0 {
			continue
		}
		headerRow := -1
		var schema csvFullSchema
		for ri := 0; ri < len(rows) && ri <= 20; ri++ {
			s, ok := buildCSVFullSchema(strings.Join(rows[ri], ";"))
			if ok {
				headerRow = ri
				schema = s
				foundHeader = true
				break
			}
		}
		if headerRow < 0 {
			continue
		}
		dataStart := headerRow + 1
		if dataStart < len(rows) && bulletinEnglishHeaderRow(rows[dataStart]) {
			dataStart++
		}
		recs, s := parseXLSXCSVRows(rows[dataStart:], schema, date)
		records = append(records, recs...)
		seen += s
	}
	if !foundHeader {
		return nil, 0, fmt.Errorf("unrecognized XLSX header")
	}
	return records, seen, nil
}

func parseXLSXCSVRows(rows [][]string, schema csvFullSchema, date time.Time) ([]DailyRecord, int) {
	maxCol := maxInt(schema.ticker, schema.close_, schema.volume, schema.high, schema.low)
	var records []DailyRecord
	seen := 0
	for _, fields := range rows {
		if len(fields) == 0 || len(fields) <= maxCol {
			continue
		}
		rawTicker := strings.TrimSpace(safeField(fields, schema.ticker))
		if rawTicker == "" {
			continue
		}
		seen++
		symbol, ok := extractEquitySymbol(rawTicker)
		if !ok {
			continue
		}

		closeP := parseFloatBulletin(safeField(fields, schema.close_))
		if closeP <= 0 {
			continue
		}
		openP := parseFloatBulletin(safeField(fields, schema.open))
		highP := parseFloatBulletin(safeField(fields, schema.high))
		lowP := parseFloatBulletin(safeField(fields, schema.low))
		vol := parseFloatBulletin(safeField(fields, schema.volume))

		if openP <= 0 {
			openP = closeP
		}
		if lowP <= 0 {
			lowP = math.Min(openP, closeP)
		}
		if highP <= 0 {
			highP = math.Max(openP, closeP)
		}

		rec := DailyRecord{
			Symbol:       symbol,
			TradingDate:  date.UTC(),
			High:         highP,
			Low:          lowP,
			Close:        closeP,
			Volume:       vol,
			SourceFormat: "xlsx",
		}
		v := openP
		rec.Open = &v
		if schema.instrumentCode >= 0 {
			rec.InstrumentCode = strings.TrimSpace(safeField(fields, schema.instrumentCode))
		}
		if schema.companyName >= 0 {
			rec.CompanyName = strings.TrimSpace(safeField(fields, schema.companyName))
		}
		if schema.previousClose >= 0 {
			rec.PreviousClose = parseFloatBulletin(safeField(fields, schema.previousClose))
		}
		if schema.valueTraded >= 0 {
			rec.ValueTraded = parseFloatBulletin(safeField(fields, schema.valueTraded))
		}
		if schema.tradeCount >= 0 {
			rec.TradeCount = int64(parseFloatBulletin(safeField(fields, schema.tradeCount)))
		}
		if schema.vwap >= 0 {
			rec.VWAP = parseFloatBulletin(safeField(fields, schema.vwap))
		}
		if schema.market >= 0 {
			rec.Market = strings.TrimSpace(safeField(fields, schema.market))
		}

		if date.Before(oldTLCutoffDB) && rec.Close > 500 {
			divisor := 1_000_000.0
			rec.Close /= divisor
			rec.High /= divisor
			rec.Low /= divisor
			rec.PreviousClose /= divisor
			rec.VWAP /= divisor
			if rec.Open != nil {
				vv := *rec.Open / divisor
				rec.Open = &vv
			}
			rec.QualityFlags = append(rec.QualityFlags, "old_tl_adjusted")
		}

		rec.AnalysisReady = rec.High > 0 && rec.Low > 0 && rec.Close > 0 &&
			rec.High >= rec.Close && rec.Close >= rec.Low
		records = append(records, rec)
	}
	return records, seen
}

func buildCSVFullSchema(headerLine string) (csvFullSchema, bool) {
	cols := strings.Split(headerLine, ";")
	idx := make(map[string]int, len(cols))
	for i, c := range cols {
		key := normHeaderCSV(c)
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
	ticker := find("ISLEM KODU", "ISLEM  KODU", "İŞLEM KODU")
	open_ := find("ACILIS FIYATI", "AÇILIŞ FİYATI")
	high := find("EN YUKSEK FIYAT", "EN YÜKSEK FİYAT")
	low := find("EN DUSUK FIYAT", "EN DÜŞÜK FİYAT")
	close_ := find("KAPANIS FIYATI", "KAPANIŞ FİYATI")
	vol := find("TOPLAM ISLEM ADEDI", "TOPLAM İŞLEM ADEDI")
	if ticker < 0 || high < 0 || low < 0 || close_ < 0 || vol < 0 {
		return csvFullSchema{}, false
	}
	return csvFullSchema{
		ticker:         ticker,
		instrumentCode: find("ENSTRUMAN KODU", "ENSTRÜMAN KODU", "ISLEM KODU", "ISLEM  KODU", "İŞLEM KODU"),
		companyName:    find("SIRKET UNVANI", "ŞİRKET ÜNVANI", "ŞIRKET ÜNVANI", "ŞIRKET UNVANI", "BULTEN ADI", "BÜLTEN ADI", "INSTRUMENT NAME"),
		open:           open_,
		high:           high,
		low:            low,
		close_:         close_,
		previousClose:  find("ONCEKI KAPANIS", "ÖNCEKİ KAPANIŞ", "ONCEKI KAPANIS FIYATI", "ÖNCEKİ KAPANIŞ FİYATI"),
		volume:         vol,
		valueTraded:    find("TOPLAM ISLEM TUTARI", "TOPLAM İŞLEM TUTARI", "TOPLAM ISLEM HACMI", "TOPLAM İŞLEM HACMİ", "TOTAL TRADED VALUE"),
		tradeCount:     find("ISLEM SAYISI", "İŞLEM SAYISI", "TOPLAM SOZLESME SAYISI", "TOPLAM SÖZLEŞME SAYISI", "TOTAL NUMBER OF CONTRACTS"),
		vwap:           find("AGIRLIKH ORTALAMA FIYAT", "AĞIRLIKLI ORTALAMA FİYAT", "ORTALAMA FIYAT", "ORTALAMA FİYAT", "A.O.F", "A.O.F.", "AOF", "VWAP"),
		market:         find("PIYASA", "PİYASA", "PAZAR", "MARKET SEGMENT"),
	}, true
}

func normHeaderCSV(s string) string {
	return strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(s, "  ", " ")))
}

// ── XLS parsing (all rows) ────────────────────────────────────────────────────

type xlsFullSchema struct {
	ticker         int
	instrumentCode int
	companyName    int
	open           int
	high           int
	low            int
	close_         int
	previousClose  int
	volume         int
	valueTraded    int
	tradeCount     int
	vwap           int
	market         int
}

func parseXLSAllRows(path string, date time.Time) ([]DailyRecord, int, error) {
	book, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, 0, fmt.Errorf("open xls %s: %w", path, err)
	}
	var records []DailyRecord
	seen := 0
	for si := 0; si < book.NumSheets(); si++ {
		sheet := book.GetSheet(si)
		if sheet == nil {
			continue
		}
		recs, s := parseXLSSheet(sheet, date)
		records = append(records, recs...)
		seen += s
	}
	return records, seen, nil
}

func parseXLSSheet(sheet *xls.WorkSheet, date time.Time) ([]DailyRecord, int) {
	maxRow := int(sheet.MaxRow)
	if maxRow < 2 {
		return nil, 0
	}

	headerRow := -1
	var schema xlsFullSchema
	for ri := 0; ri <= maxRow && ri <= 20; ri++ {
		row := xlsSafeRowDB(sheet, ri)
		if row == nil {
			continue
		}
		s, ok := buildXLSFullSchema(xlsRowStringsDB(row))
		if ok {
			headerRow = ri
			schema = s
			break
		}
	}
	if headerRow < 0 {
		return nil, 0
	}

	var records []DailyRecord
	seen := 0
	for ri := headerRow + 1; ri <= maxRow; ri++ {
		row := xlsSafeRowDB(sheet, ri)
		if row == nil {
			continue
		}
		cols := xlsRowStringsDB(row)
		if len(cols) <= schema.ticker {
			continue
		}
		rawTicker := strings.TrimSpace(cols[schema.ticker])
		if rawTicker == "" {
			continue
		}
		seen++
		symbol, ok := extractEquitySymbol(rawTicker)
		if !ok {
			continue
		}

		closeP := xlsParseFloat(safeXLSCol(cols, schema.close_))
		if closeP <= 0 {
			continue
		}
		openP := 0.0
		if schema.open >= 0 {
			openP = xlsParseFloat(safeXLSCol(cols, schema.open))
		}
		highP := xlsParseFloat(safeXLSCol(cols, schema.high))
		lowP := xlsParseFloat(safeXLSCol(cols, schema.low))
		vol := xlsParseFloat(safeXLSCol(cols, schema.volume))

		if openP <= 0 {
			openP = closeP
		}
		if lowP <= 0 {
			lowP = math.Min(openP, closeP)
		}
		if highP <= 0 {
			highP = math.Max(openP, closeP)
		}

		rec := DailyRecord{
			Symbol:       symbol,
			TradingDate:  date.UTC(),
			High:         highP,
			Low:          lowP,
			Close:        closeP,
			Volume:       vol,
			SourceFormat: "xls",
		}
		v := openP
		rec.Open = &v
		if schema.instrumentCode >= 0 {
			rec.InstrumentCode = strings.TrimSpace(safeXLSCol(cols, schema.instrumentCode))
		}
		if schema.companyName >= 0 {
			rec.CompanyName = strings.TrimSpace(safeXLSCol(cols, schema.companyName))
		}
		if schema.previousClose >= 0 {
			rec.PreviousClose = xlsParseFloat(safeXLSCol(cols, schema.previousClose))
		}
		if schema.valueTraded >= 0 {
			rec.ValueTraded = xlsParseFloat(safeXLSCol(cols, schema.valueTraded))
		}
		if schema.tradeCount >= 0 {
			rec.TradeCount = int64(xlsParseFloat(safeXLSCol(cols, schema.tradeCount)))
		}
		if schema.vwap >= 0 {
			rec.VWAP = xlsParseFloat(safeXLSCol(cols, schema.vwap))
		}
		if schema.market >= 0 {
			rec.Market = strings.TrimSpace(safeXLSCol(cols, schema.market))
		}

		if date.Before(oldTLCutoffDB) && rec.Close > 500 {
			divisor := 1_000_000.0
			rec.Close /= divisor
			rec.High /= divisor
			rec.Low /= divisor
			rec.PreviousClose /= divisor
			rec.VWAP /= divisor
			if rec.Open != nil {
				vv := *rec.Open / divisor
				rec.Open = &vv
			}
			rec.QualityFlags = append(rec.QualityFlags, "old_tl_adjusted")
		}

		rec.AnalysisReady = rec.High > 0 && rec.Low > 0 && rec.Close > 0 &&
			rec.High >= rec.Close && rec.Close >= rec.Low
		records = append(records, rec)
	}
	return records, seen
}

func buildXLSFullSchema(cols []string) (xlsFullSchema, bool) {
	idx := make(map[string]int, len(cols))
	for i, c := range cols {
		key := xlsNormKeyDB(c)
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
	ticker := find("PAYINKODU", "HISSESENEDININKODU", "ENSTRUMANKODU")
	high := find("ENYUKSEKFIYAT")
	low := find("ENDUSUKFIYAT")
	close_ := find("KAPANIS", "KAPANISFIYATI")
	vol := find("TOPLAMISLEMADEDI")
	if ticker < 0 || high < 0 || low < 0 || close_ < 0 || vol < 0 {
		return xlsFullSchema{}, false
	}
	return xlsFullSchema{
		ticker:         ticker,
		instrumentCode: find("ENSTRUMANKODU"),
		companyName:    find("SIRKETUNVANI", "SIRKETADI"),
		open:           find("ACILISFIYATI"),
		high:           high,
		low:            low,
		close_:         close_,
		previousClose:  find("ONCEKIKAPANIS", "ONCEKIKAPANISFIYATI"),
		volume:         vol,
		valueTraded:    find("TOPLAMISLEMTUTARI"),
		tradeCount:     find("ISLEMSAYISI"),
		vwap:           find("AGIRLIKLIORTALAMAFIYAT", "ORTALAMAFIYAT"),
		market:         find("PIYASA"),
	}, true
}

// ── shared helpers ────────────────────────────────────────────────────────────

func extractEquitySymbol(rawTicker string) (string, bool) {
	base := rawTicker
	suffix := ""
	if dot := strings.LastIndex(rawTicker, "."); dot > 0 {
		base = rawTicker[:dot]
		suffix = strings.ToUpper(rawTicker[dot+1:])
	}
	if suffix != "" && suffix != "E" {
		return "", false
	}
	symbol := strings.ToUpper(strings.TrimSpace(base))
	if symbol == "" {
		return "", false
	}
	return symbol, true
}

func parseFloatBulletin(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func safeField(fields []string, idx int) string {
	if idx < 0 || idx >= len(fields) {
		return ""
	}
	return fields[idx]
}

func safeXLSCol(cols []string, idx int) string {
	if idx < 0 || idx >= len(cols) {
		return "0"
	}
	return cols[idx]
}

func xlsParseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func xlsNormKeyDB(s string) string {
	s = xlsDecodeCP1254DB(s)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ToUpper(s)
	r := strings.NewReplacer(
		"İ", "I", "Ğ", "G", "Ş", "S", "Ç", "C", "Ü", "U", "Ö", "O",
	)
	return r.Replace(s)
}

func xlsDecodeCP1254DB(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	decoded, err := charmap.Windows1254.NewDecoder().Bytes([]byte(s))
	if err != nil {
		return s
	}
	return string(decoded)
}

func xlsSafeRowDB(sheet *xls.WorkSheet, index int) (row *xls.Row) {
	defer func() {
		if recover() != nil {
			row = nil
		}
	}()
	return sheet.Row(index)
}

func xlsRowStringsDB(row *xls.Row) []string {
	if row == nil {
		return nil
	}
	last := int(row.LastCol())
	out := make([]string, last)
	for i := range out {
		out[i] = xlsDecodeCP1254DB(strings.TrimSpace(row.Col(i)))
	}
	return out
}

func bulletinEnglishHeaderRow(row []string) bool {
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

func maxInt(vals ...int) int {
	m := 0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}
