package matriksformations

import (
	"context"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/storage"
	"hissebot/internal/util"

	xhtml "golang.org/x/net/html"
)

const (
	DefaultURL         = "https://www.matriksdata.com/formation_iframe/Default.aspx?GUID=D459F378901F9F16B03D5CD47DFB77F214993DB0D25FFB7EEB6D870CB23B5EDA44C96DE8038F3DB6D05DCF44E46B9F12B63923AD"
	DefaultMarketValue = "4"
	DefaultMarketLabel = "Bist Tüm"
)

var (
	rowIDPattern          = regexp.MustCompile(`^LV_formations_([A-Za-z0-9_]+?)(?:Label|TD)?_([0-9]+)$`)
	graphIDPattern        = regexp.MustCompile(`(?:^|[?&])ID=([0-9]+)`)
	errMatriksNoFormState = errors.New("matriks formation page form state not found")
	errMatriksAccess      = errors.New("matriks formation page access expired or denied")
)

type Options struct {
	URL                     string
	OutputPath              string
	EquitiesDir             string
	MarketValue             string
	Sets                    []string
	PeriodType              string
	FormationType           string
	Size                    string
	Status                  string
	MinStrength             string
	MaxLineErr              string
	MaxPages                int
	Timeout                 time.Duration
	PageDelay               time.Duration
	Retries                 int
	UserAgent               string
	WritePerTicker          bool
	CreateMissingTickerDirs bool
}

type Snapshot struct {
	SourceURL   string            `json:"source_url"`
	FetchedAt   time.Time         `json:"fetched_at"`
	MarketValue string            `json:"market_value"`
	MarketLabel string            `json:"market_label"`
	Filters     map[string]string `json:"filters"`
	Stats       SnapshotStats     `json:"stats"`
	Records     []FormationRecord `json:"records"`
}

type SnapshotStats struct {
	Pages           int            `json:"pages"`
	Records         int            `json:"records"`
	Tickers         int            `json:"tickers"`
	Active          int            `json:"active"`
	Completed       int            `json:"completed"`
	BySet           map[string]int `json:"by_set,omitempty"`
	ByTimeframe     map[string]int `json:"by_timeframe,omitempty"`
	ByFormation     map[string]int `json:"by_formation,omitempty"`
	TickerFiles     int            `json:"ticker_files,omitempty"`
	SkippedTickers  int            `json:"skipped_tickers,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
	SourceDeadlocks int            `json:"source_deadlocks,omitempty"`
}

type FormationRecord struct {
	Symbol                 string     `json:"symbol"`
	Ticker                 string     `json:"ticker"`
	Set                    string     `json:"set"`
	MarketValue            string     `json:"market_value"`
	MarketLabel            string     `json:"market_label"`
	PeriodLabel            string     `json:"period_label"`
	Timeframe              string     `json:"timeframe"`
	FormationType          string     `json:"formation_type"`
	CanonicalPatternName   string     `json:"canonical_pattern_name,omitempty"`
	Direction              string     `json:"direction"`
	DirectionLabel         string     `json:"direction_label"`
	Status                 string     `json:"status"`
	PriceDifferencePercent *float64   `json:"price_difference_percent,omitempty"`
	MaxPossibleGainPercent *float64   `json:"max_possible_gain_percent,omitempty"`
	Strength               *float64   `json:"strength,omitempty"`
	MaxLineErr             *float64   `json:"max_line_err,omitempty"`
	ConfirmationDate       string     `json:"confirmation_date,omitempty"`
	ConfirmationAt         *time.Time `json:"confirmation_at,omitempty"`
	UpdateDate             string     `json:"update_date,omitempty"`
	UpdatedAt              *time.Time `json:"updated_at,omitempty"`
	GraphID                string     `json:"graph_id,omitempty"`
	GraphURL               string     `json:"graph_url,omitempty"`
	SourceURL              string     `json:"source_url"`
	SourcePage             int        `json:"source_page"`
	SourceRow              int        `json:"source_row"`
	FetchedAt              time.Time  `json:"fetched_at"`
}

type TickerSnapshot struct {
	Symbol    string            `json:"symbol"`
	SourceURL string            `json:"source_url"`
	FetchedAt time.Time         `json:"fetched_at"`
	Stats     SnapshotStats     `json:"stats"`
	Records   []FormationRecord `json:"records"`
}

type Result struct {
	SourceURL      string        `json:"source_url"`
	OutputPath     string        `json:"output_path"`
	MarketValue    string        `json:"market_value"`
	MarketLabel    string        `json:"market_label"`
	Sets           []string      `json:"sets"`
	FetchedAt      time.Time     `json:"fetched_at"`
	Pages          int           `json:"pages"`
	Records        int           `json:"records"`
	Tickers        int           `json:"tickers"`
	TickerFiles    int           `json:"ticker_files"`
	SkippedTickers int           `json:"skipped_tickers"`
	Warnings       []string      `json:"warnings,omitempty"`
	Snapshot       *Snapshot     `json:"snapshot,omitempty"`
	Duration       time.Duration `json:"duration"`
}

type client struct {
	http       *http.Client
	url        string
	userAgent  string
	retries    int
	pageDelay  time.Duration
	deadlocks  int
	lastRefURL string
}

type rowBuilder struct {
	record FormationRecord
}

func Sync(ctx context.Context, cfg config.Config, opts Options) (Result, error) {
	started := time.Now()
	opts = normalizeOptions(cfg, opts)
	c, err := newClient(opts)
	if err != nil {
		return Result{}, err
	}
	fetchedAt := time.Now().UTC()
	all := []FormationRecord{}
	pages := 0
	warnings := []string{}

	for _, set := range opts.Sets {
		html, err := c.get(ctx)
		if err != nil {
			return Result{}, fmt.Errorf("matriks initial fetch %s: %w", set, err)
		}
		form, err := parseFormValues(html)
		if err != nil {
			return Result{}, err
		}
		applyFilterForm(form, opts, set)
		target := "IB_refresh"
		if set == "completed" {
			target = "RBL_formStatus$1"
		}
		html, err = c.post(ctx, form, target, "")
		if err != nil {
			return Result{}, fmt.Errorf("matriks apply filter %s: %w", set, err)
		}

		seenPages := map[string]bool{}
		for page := 1; ; page++ {
			records, err := parseFormationRecords(html, set, opts.MarketValue, DefaultMarketLabel, opts.URL, page, fetchedAt)
			if err != nil {
				return Result{}, fmt.Errorf("parse matriks %s page %d: %w", set, page, err)
			}
			fingerprint := pageFingerprint(records)
			if seenPages[fingerprint] {
				warnings = append(warnings, fmt.Sprintf("%s scan stopped at page %d because Matriks returned a repeated page", set, page))
				break
			}
			seenPages[fingerprint] = true
			all = append(all, records...)
			pages++
			if opts.MaxPages > 0 && page >= opts.MaxPages {
				warnings = append(warnings, fmt.Sprintf("%s scan stopped at max_pages=%d", set, opts.MaxPages))
				break
			}
			if !hasNextPage(html) {
				break
			}
			nextForm, err := parseFormValues(html)
			if err != nil {
				return Result{}, err
			}
			nextForm.Set("DP_formations$ctl02$ctl00", "Sonraki")
			html, err = c.postValues(ctx, nextForm)
			if err != nil {
				return Result{}, fmt.Errorf("matriks next page %s page %d: %w", set, page+1, err)
			}
			if err := sleepContext(ctx, opts.PageDelay); err != nil {
				return Result{}, err
			}
		}
	}

	all = dedupeRecords(all)
	stats := buildStats(all, pages)
	stats.SourceDeadlocks = c.deadlocks
	if c.deadlocks > 0 {
		warnings = append(warnings, fmt.Sprintf("matriks source returned %d transient ASP.NET deadlock page(s); retry succeeded", c.deadlocks))
	}
	stats.Warnings = append(stats.Warnings, warnings...)
	snapshot := Snapshot{
		SourceURL:   opts.URL,
		FetchedAt:   fetchedAt,
		MarketValue: opts.MarketValue,
		MarketLabel: DefaultMarketLabel,
		Filters: map[string]string{
			"period_type":    opts.PeriodType,
			"formation_type": opts.FormationType,
			"size":           opts.Size,
			"status":         opts.Status,
			"min_strength":   opts.MinStrength,
			"max_line_err":   opts.MaxLineErr,
		},
		Stats:   stats,
		Records: all,
	}
	if opts.OutputPath != "" {
		if err := util.WriteJSON(opts.OutputPath, snapshot); err != nil {
			return Result{}, fmt.Errorf("write matriks snapshot: %w", err)
		}
	}
	tickerFiles := 0
	skippedTickers := 0
	if opts.WritePerTicker && opts.EquitiesDir != "" {
		tickerFiles, skippedTickers, err = writeTickerSnapshots(opts.EquitiesDir, opts.URL, fetchedAt, all, opts.CreateMissingTickerDirs)
		if err != nil {
			return Result{}, err
		}
		snapshot.Stats.TickerFiles = tickerFiles
		snapshot.Stats.SkippedTickers = skippedTickers
		if opts.OutputPath != "" {
			if err := util.WriteJSON(opts.OutputPath, snapshot); err != nil {
				return Result{}, fmt.Errorf("rewrite matriks snapshot stats: %w", err)
			}
		}
	}

	return Result{
		SourceURL:      opts.URL,
		OutputPath:     opts.OutputPath,
		MarketValue:    opts.MarketValue,
		MarketLabel:    DefaultMarketLabel,
		Sets:           append([]string{}, opts.Sets...),
		FetchedAt:      fetchedAt,
		Pages:          pages,
		Records:        len(all),
		Tickers:        stats.Tickers,
		TickerFiles:    tickerFiles,
		SkippedTickers: skippedTickers,
		Warnings:       warnings,
		Snapshot:       &snapshot,
		Duration:       time.Since(started),
	}, nil
}

func LoadTickerSnapshot(equitiesDir, symbol string) (*TickerSnapshot, error) {
	ticker := storage.NormalizeTicker(symbol)
	if ticker == "" || strings.TrimSpace(equitiesDir) == "" {
		return nil, nil
	}
	path := TickerPath(equitiesDir, ticker)
	var snapshot TickerSnapshot
	if err := util.ReadJSON(path, &snapshot); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &snapshot, nil
}

func TickerPath(equitiesDir, ticker string) string {
	return filepath.Join(equitiesDir, storage.NormalizeTicker(ticker), "matriks_formations.json")
}

func normalizeOptions(cfg config.Config, opts Options) Options {
	if opts.URL == "" {
		opts.URL = cfg.MatriksFormationsURL
	}
	if opts.URL == "" {
		opts.URL = DefaultURL
	}
	if opts.OutputPath == "" {
		opts.OutputPath = cfg.MatriksFormationsFile
	}
	if opts.EquitiesDir == "" {
		opts.EquitiesDir = cfg.EquitiesDir
	}
	if opts.MarketValue == "" {
		opts.MarketValue = DefaultMarketValue
	}
	opts.Sets = normalizeSets(opts.Sets)
	if opts.PeriodType == "" {
		opts.PeriodType = "-1"
	}
	if opts.FormationType == "" {
		opts.FormationType = "-1"
	}
	if opts.Size == "" {
		opts.Size = "-1"
	}
	if opts.Status == "" {
		opts.Status = "-1"
	}
	if opts.MinStrength == "" {
		opts.MinStrength = "0"
	}
	if opts.MaxLineErr == "" {
		opts.MaxLineErr = "1"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = cfg.HTTPTimeout
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 45 * time.Second
	}
	if opts.PageDelay == 0 {
		opts.PageDelay = 250 * time.Millisecond
	}
	if opts.Retries <= 0 {
		opts.Retries = 3
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "Mozilla/5.0 (compatible; hissebot/1.0; +https://localhost)"
	}
	return opts
}

func normalizeSets(values []string) []string {
	if len(values) == 0 {
		return []string{"active", "completed"}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			switch part {
			case "active", "aktif", "1":
				part = "active"
			case "completed", "tamamlanmis", "tamamlanmış", "0":
				part = "completed"
			default:
				continue
			}
			if !seen[part] {
				seen[part] = true
				out = append(out, part)
			}
		}
	}
	if len(out) == 0 {
		return []string{"active", "completed"}
	}
	return out
}

func newClient(opts Options) (*client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &client{
		http: &http.Client{
			Jar:     jar,
			Timeout: opts.Timeout,
		},
		url:        opts.URL,
		userAgent:  opts.UserAgent,
		retries:    opts.Retries,
		pageDelay:  opts.PageDelay,
		lastRefURL: opts.URL,
	}, nil
}

func (c *client) get(ctx context.Context) (string, error) {
	return c.do(ctx, http.MethodGet, nil)
}

func (c *client) post(ctx context.Context, values url.Values, eventTarget, eventArgument string) (string, error) {
	next := cloneValues(values)
	next.Set("__EVENTTARGET", eventTarget)
	next.Set("__EVENTARGUMENT", eventArgument)
	return c.postValues(ctx, next)
}

func (c *client) postValues(ctx context.Context, values url.Values) (string, error) {
	values.Set("__LASTFOCUS", "")
	if _, ok := values["__EVENTTARGET"]; !ok {
		values.Set("__EVENTTARGET", "")
	}
	if _, ok := values["__EVENTARGUMENT"]; !ok {
		values.Set("__EVENTARGUMENT", "")
	}
	return c.do(ctx, http.MethodPost, values)
}

func (c *client) do(ctx context.Context, method string, values url.Values) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, time.Duration(attempt)*750*time.Millisecond); err != nil {
				return "", err
			}
		}
		var body io.Reader
		if values != nil {
			body = strings.NewReader(values.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, c.url, body)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")
		req.Header.Set("Referer", c.lastRefURL)
		if values != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		html := string(data)
		if resp.StatusCode >= 500 || isTransientMatriksHTML(html) {
			if isTransientMatriksHTML(html) {
				c.deadlocks++
			}
			lastErr = fmt.Errorf("matriks transient response: status=%d", resp.StatusCode)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("matriks response status %d", resp.StatusCode)
		}
		c.lastRefURL = c.url
		return html, nil
	}
	return "", lastErr
}

func isTransientMatriksHTML(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "deadlocked on lock") ||
		strings.Contains(value, "httphandledexception") ||
		strings.Contains(value, "httpunhandledexception") ||
		strings.Contains(value, "customerrors mode")
}

func isAccessDeniedHTML(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "erişmek istediğiniz sayfaya erişim yetkiniz") ||
		strings.Contains(value, "erismek istediginiz sayfaya erisim yetkiniz") ||
		strings.Contains(value, "yetkiniz zaman aşımına uğradı") ||
		strings.Contains(value, "yetkiniz zaman asimina ugradi")
}

func applyFilterForm(values url.Values, opts Options, set string) {
	values.Set("RBL_formStatus", formStatusValue(set))
	values.Set("DDL_index", opts.MarketValue)
	values.Set("DDL_symbols", "-1")
	values.Set("DDL_periodType", opts.PeriodType)
	values.Set("DDL_FormationType", opts.FormationType)
	values.Set("DDL_Size", opts.Size)
	values.Set("DDL_Status", opts.Status)
	values.Set("TXT_MinStrength", opts.MinStrength)
	values.Set("TXT_MaxLineErr", opts.MaxLineErr)
}

func formStatusValue(set string) string {
	if set == "completed" {
		return "0"
	}
	return "1"
}

func parseFormValues(html string) (url.Values, error) {
	doc, err := xhtml.Parse(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch strings.ToLower(n.Data) {
			case "input":
				parseInputValue(values, n)
			case "select":
				parseSelectValue(values, n)
			case "textarea":
				if name := attr(n, "name"); name != "" {
					values.Set(name, nodeText(n))
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if values.Get("__VIEWSTATE") == "" || values.Get("__EVENTVALIDATION") == "" {
		if isAccessDeniedHTML(html) {
			return nil, errMatriksAccess
		}
		return nil, errMatriksNoFormState
	}
	return values, nil
}

func parseInputValue(values url.Values, n *xhtml.Node) {
	name := attr(n, "name")
	if name == "" {
		return
	}
	inputType := strings.ToLower(attr(n, "type"))
	switch inputType {
	case "submit", "image", "button":
		return
	case "radio", "checkbox":
		if !hasAttr(n, "checked") {
			return
		}
	}
	values.Set(name, attr(n, "value"))
}

func parseSelectValue(values url.Values, n *xhtml.Node) {
	name := attr(n, "name")
	if name == "" {
		return
	}
	selected := ""
	first := ""
	var walkOptions func(*xhtml.Node)
	walkOptions = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "option") {
			value := attr(node, "value")
			if first == "" {
				first = value
			}
			if hasAttr(node, "selected") {
				selected = value
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walkOptions(child)
		}
	}
	walkOptions(n)
	if selected == "" {
		selected = first
	}
	values.Set(name, selected)
}

func hasNextPage(html string) bool {
	doc, err := xhtml.Parse(strings.NewReader(html))
	if err != nil {
		return strings.Contains(html, `name="DP_formations$ctl02$ctl00"`) && !strings.Contains(html, `name="DP_formations$ctl02$ctl00" disabled`)
	}
	found := false
	disabled := false
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && strings.EqualFold(n.Data, "input") && attr(n, "name") == "DP_formations$ctl02$ctl00" {
			found = true
			disabled = hasAttr(n, "disabled") || strings.Contains(attr(n, "class"), "aspNetDisabled")
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return found && !disabled
}

func parseFormationRecords(html, set, marketValue, marketLabel, sourceURL string, page int, fetchedAt time.Time) ([]FormationRecord, error) {
	doc, err := xhtml.Parse(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	rows := map[int]*rowBuilder{}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			id := attr(n, "id")
			if matches := rowIDPattern.FindStringSubmatch(id); len(matches) == 3 {
				idx, _ := strconv.Atoi(matches[2])
				builder := rows[idx]
				if builder == nil {
					builder = &rowBuilder{}
					rows[idx] = builder
				}
				fillRecordField(&builder.record, matches[1], n)
			}
			if strings.EqualFold(n.Data, "a") {
				href := attr(n, "href")
				if strings.Contains(href, "Graph.aspx") {
					if idx, ok := descendantFormationIndex(n); ok {
						builder := rows[idx]
						if builder == nil {
							builder = &rowBuilder{}
							rows[idx] = builder
						}
						builder.record.GraphURL = absoluteMatriksURL(sourceURL, href)
						if id := graphID(href); id != "" {
							builder.record.GraphID = id
						}
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	indexes := make([]int, 0, len(rows))
	for idx := range rows {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	out := make([]FormationRecord, 0, len(indexes))
	for _, idx := range indexes {
		record := rows[idx].record
		record.Symbol = strings.TrimSpace(record.Symbol)
		if record.Symbol == "" {
			continue
		}
		record.Ticker = storage.NormalizeTicker(record.Symbol)
		record.Set = set
		record.MarketValue = marketValue
		record.MarketLabel = marketLabel
		record.Timeframe = periodToTimeframe(record.PeriodLabel)
		record.Direction = directionCode(record.DirectionLabel)
		record.CanonicalPatternName = canonicalPatternName(record.FormationType)
		record.SourceURL = sourceURL
		record.SourcePage = page
		record.SourceRow = idx
		record.FetchedAt = fetchedAt
		record.ConfirmationAt = parseMatriksDate(record.ConfirmationDate)
		record.UpdatedAt = parseMatriksDate(record.UpdateDate)
		out = append(out, record)
	}
	return out, nil
}

func fillRecordField(record *FormationRecord, field string, n *xhtml.Node) {
	text := cleanText(nodeText(n))
	switch field {
	case "Sembol":
		record.Symbol = text
	case "PeriodType":
		record.PeriodLabel = text
	case "formationType":
		record.FormationType = text
	case "directionArrow":
		record.DirectionLabel = cleanText(attr(n, "alt"))
	case "status":
		record.Status = text
	case "priceDifferencePercent":
		record.PriceDifferencePercent = parsePercent(text)
	case "maxPossibleGain":
		record.MaxPossibleGainPercent = parsePercent(text)
	case "confirmationDate":
		record.ConfirmationDate = text
	case "strength":
		record.Strength = parseFloatPtr(text)
	case "maxLineErr":
		record.MaxLineErr = parseFloatPtr(text)
	case "updateDate":
		record.UpdateDate = text
	}
}

func descendantFormationIndex(n *xhtml.Node) (int, bool) {
	var found int
	ok := false
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if ok {
			return
		}
		if node.Type == xhtml.ElementNode {
			id := attr(node, "id")
			if strings.HasPrefix(id, "LV_formations_formationTypeLabel_") {
				parts := strings.Split(id, "_")
				if len(parts) > 0 {
					if idx, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
						found = idx
						ok = true
						return
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return found, ok
}

func graphID(href string) string {
	if matches := graphIDPattern.FindStringSubmatch(href); len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func absoluteMatriksURL(baseURL, href string) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	return base.ResolveReference(parsed).String()
}

func periodToTimeframe(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "günlük", "gunluk", "daily":
		return "1D"
	case "saatlik", "hourly":
		return "1H"
	default:
		return value
	}
}

func directionCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, "artış") || strings.Contains(value, "artis") || strings.Contains(value, "up"):
		return "bullish"
	case strings.Contains(value, "düşüş") || strings.Contains(value, "dusus") || strings.Contains(value, "down"):
		return "bearish"
	default:
		return "neutral"
	}
}

func canonicalPatternName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "head shoulder tops":
		return "Head and Shoulders"
	case "head shoulder bottoms":
		return "Inverse Head and Shoulders"
	case "triple tops":
		return "Triple Top"
	case "triple bottoms":
		return "Triple Bottom"
	case "double tops":
		return "Double Top"
	case "double bottoms":
		return "Double Bottom"
	case "broadening formation asc.":
		return "Broadening Top"
	case "broadening formation desc.":
		return "Broadening Bottom"
	case "triangles asc.":
		return "Ascending Triangle"
	case "triangles desc.":
		return "Descending Triangle"
	case "triangles sym.":
		return "Symmetrical Triangle"
	case "wedges asc.", "wedges rising":
		return "Rising Wedge"
	case "wedges desc.", "wedges falling":
		return "Falling Wedge"
	case "flag/pennant rising":
		return "Bullish Flag/Pennant"
	case "flag/pennant falling":
		return "Bearish Flag/Pennant"
	default:
		return strings.TrimSpace(value)
	}
}

func parsePercent(value string) *float64 {
	value = strings.ReplaceAll(value, "%", "")
	return parseFloatPtr(value)
}

func parseFloatPtr(value string) *float64 {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ",", ".")
	value = strings.Trim(value, "%")
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseMatriksDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	loc := time.FixedZone("TRT", 3*60*60)
	for _, layout := range []string{"02/01/2006 15:04:05", "02/01/2006 15:04", "02/01/2006"} {
		parsed, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func buildStats(records []FormationRecord, pages int) SnapshotStats {
	bySet := map[string]int{}
	byTimeframe := map[string]int{}
	byFormation := map[string]int{}
	tickers := map[string]bool{}
	for _, record := range records {
		bySet[record.Set]++
		byTimeframe[record.Timeframe]++
		byFormation[record.FormationType]++
		if record.Ticker != "" {
			tickers[record.Ticker] = true
		}
	}
	return SnapshotStats{
		Pages:       pages,
		Records:     len(records),
		Tickers:     len(tickers),
		Active:      bySet["active"],
		Completed:   bySet["completed"],
		BySet:       bySet,
		ByTimeframe: byTimeframe,
		ByFormation: byFormation,
	}
}

func dedupeRecords(records []FormationRecord) []FormationRecord {
	seen := map[string]bool{}
	out := make([]FormationRecord, 0, len(records))
	for _, record := range records {
		key := strings.Join([]string{
			record.Set,
			record.Symbol,
			record.Timeframe,
			record.FormationType,
			record.Status,
			record.ConfirmationDate,
			record.GraphID,
		}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, record)
	}
	return out
}

func pageFingerprint(records []FormationRecord) string {
	if len(records) == 0 {
		return "empty"
	}
	parts := make([]string, 0, len(records))
	for _, record := range records {
		parts = append(parts, strings.Join([]string{
			record.Symbol,
			record.Timeframe,
			record.FormationType,
			record.Status,
			record.ConfirmationDate,
			record.UpdateDate,
			record.GraphID,
		}, "\x00"))
	}
	return strings.Join(parts, "\x01")
}

func writeTickerSnapshots(equitiesDir, sourceURL string, fetchedAt time.Time, records []FormationRecord, createMissingDirs bool) (int, int, error) {
	grouped := map[string][]FormationRecord{}
	for _, record := range records {
		ticker := storage.NormalizeTicker(record.Ticker)
		if ticker == "" {
			continue
		}
		grouped[ticker] = append(grouped[ticker], record)
	}
	tickers := make([]string, 0, len(grouped))
	for ticker := range grouped {
		tickers = append(tickers, ticker)
	}
	sort.Strings(tickers)
	written := 0
	skipped := 0
	for _, ticker := range tickers {
		tickerDir := filepath.Join(equitiesDir, ticker)
		if !createMissingDirs {
			if _, err := os.Stat(filepath.Join(tickerDir, "equity.json")); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					skipped++
					continue
				}
				return written, skipped, err
			}
		}
		stats := buildStats(grouped[ticker], 0)
		snapshot := TickerSnapshot{
			Symbol:    ticker,
			SourceURL: sourceURL,
			FetchedAt: fetchedAt,
			Stats:     stats,
			Records:   grouped[ticker],
		}
		if err := util.WriteJSON(TickerPath(equitiesDir, ticker), snapshot); err != nil {
			return written, skipped, err
		}
		written++
	}
	return written, skipped, nil
}

func cloneValues(values url.Values) url.Values {
	out := url.Values{}
	for key, items := range values {
		out[key] = append([]string{}, items...)
	}
	return out
}

func attr(n *xhtml.Node, name string) string {
	for _, item := range n.Attr {
		if strings.EqualFold(item.Key, name) {
			return stdhtml.UnescapeString(item.Val)
		}
	}
	return ""
}

func hasAttr(n *xhtml.Node, name string) bool {
	for _, item := range n.Attr {
		if strings.EqualFold(item.Key, name) {
			return true
		}
	}
	return false
}

func nodeText(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return stdhtml.UnescapeString(b.String())
}

func cleanText(value string) string {
	value = stdhtml.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.Join(strings.Fields(value), " ")
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
