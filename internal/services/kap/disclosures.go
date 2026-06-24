package kap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type FinancialDisclosure struct {
	ID                 string         `json:"id,omitempty"`
	Ticker             string         `json:"ticker,omitempty"`
	Title              string         `json:"title,omitempty"`
	Subject            string         `json:"subject,omitempty"`
	Summary            string         `json:"summary,omitempty"`
	DisclosureClass    string         `json:"disclosure_class,omitempty"`
	DisclosureType     string         `json:"disclosure_type,omitempty"`
	DisclosureCategory string         `json:"disclosure_category,omitempty"`
	DisclosureIndex    int            `json:"disclosure_index,omitempty"`
	AttachmentCount    int            `json:"attachment_count,omitempty"`
	PeriodKey          string         `json:"period_key,omitempty"`
	FiscalYear         int            `json:"fiscal_year,omitempty"`
	FiscalQuarter      int            `json:"fiscal_quarter,omitempty"`
	PublishDate        *time.Time     `json:"publish_date,omitempty"`
	Source             string         `json:"source,omitempty"`
	URL                string         `json:"url,omitempty"`
	ReceivedAt         time.Time      `json:"received_at,omitempty"`
	Raw                map[string]any `json:"raw,omitempty"`
}

type financialDisclosureFile struct {
	Source      string                `json:"source,omitempty"`
	FetchedAt   time.Time             `json:"fetched_at,omitempty"`
	Disclosures []FinancialDisclosure `json:"disclosures,omitempty"`
	Value       []FinancialDisclosure `json:"value,omitempty"`
}

type FinancialDisclosureSyncOptions struct {
	URL             string
	FromDate        time.Time
	ToDate          time.Time
	ChunkDays       int
	MemberTypes     []string
	DisclosureTypes []string
}

const defaultFinancialDisclosureURL = "https://www.kap.org.tr/tr/api/disclosure/list/main"
const AllDisclosureTypes = "all"

func ImportFinancialDisclosures(ctx context.Context, store *storage.EquityStore, file string) error {
	disclosures, err := LoadFinancialDisclosures(file)
	if err != nil {
		return err
	}
	updated, resolved, err := ImportFinancialDisclosureList(ctx, store, disclosures)
	if err != nil {
		return err
	}
	fmt.Printf("kap disclosures: %d tickers updated, %d financial periods resolved from %s\n", updated, resolved, file)
	return nil
}

func SyncFinancialDisclosures(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts FinancialDisclosureSyncOptions) error {
	disclosures, err := FetchFinancialDisclosures(ctx, cfg, opts)
	if err != nil {
		return err
	}
	updated, resolved, err := ImportFinancialDisclosureList(ctx, store, disclosures)
	if err != nil {
		return err
	}
	fmt.Printf("kap disclosures: %d tickers updated, %d financial periods resolved from live feed\n", updated, resolved)
	return nil
}

func FetchFinancialDisclosures(ctx context.Context, cfg config.Config, opts FinancialDisclosureSyncOptions) ([]FinancialDisclosure, error) {
	url := strings.TrimSpace(firstNonEmpty(opts.URL, cfg.KAPDisclosuresURL, defaultFinancialDisclosureURL))
	if url == "" {
		return nil, errors.New("KAP disclosures URL is required")
	}
	if isKAPDisclosureListEndpoint(url) {
		return fetchKAPDisclosureList(ctx, cfg, url, opts)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setKAPRequestHeaders(req)
	req.Header.Set("Accept", "application/json")
	if cfg.KAPDisclosuresToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.KAPDisclosuresToken)
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kap disclosures: unexpected HTTP status %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseFinancialDisclosures(raw)
}

func fetchKAPDisclosureList(ctx context.Context, cfg config.Config, url string, opts FinancialDisclosureSyncOptions) ([]FinancialDisclosure, error) {
	from, to := normalizeDisclosureDateRange(opts.FromDate, opts.ToDate)
	chunkDays := opts.ChunkDays
	if chunkDays <= 0 || chunkDays > 90 {
		chunkDays = 90
	}
	memberTypes := opts.MemberTypes
	if len(memberTypes) == 0 {
		memberTypes = []string{"IGS"}
	}
	disclosureTypes := opts.DisclosureTypes
	if len(disclosureTypes) == 0 {
		disclosureTypes = []string{"FR"}
	}
	allDisclosureTypes := wantsAllDisclosureTypes(disclosureTypes)
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	out := []FinancialDisclosure{}
	for start := from; !start.After(to); start = start.AddDate(0, 0, chunkDays) {
		end := start.AddDate(0, 0, chunkDays-1)
		if end.After(to) {
			end = to
		}
		chunk, err := fetchKAPDisclosureListChunk(ctx, client, cfg, url, start, end, memberTypes, disclosureTypes, allDisclosureTypes)
		if err != nil {
			return out, err
		}
		out = append(out, chunk...)
	}
	return normalizeFinancialDisclosures(out), nil
}

func fetchKAPDisclosureListChunk(ctx context.Context, client *http.Client, cfg config.Config, url string, from time.Time, to time.Time, memberTypes []string, disclosureTypes []string, allDisclosureTypes bool) ([]FinancialDisclosure, error) {
	payload := map[string]any{
		"fromDate":    formatKAPDate(from),
		"toDate":      formatKAPDate(to),
		"memberTypes": memberTypes,
	}
	if !allDisclosureTypes {
		payload["disclosureTypes"] = disclosureTypes
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setKAPRequestHeaders(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if cfg.KAPDisclosuresToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.KAPDisclosuresToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kap disclosures %s-%s: unexpected HTTP status %s: %s", formatKAPDate(from), formatKAPDate(to), resp.Status, strings.TrimSpace(string(raw)))
	}
	disclosures, err := parseFinancialDisclosures(raw)
	if err != nil {
		return nil, fmt.Errorf("kap disclosures %s-%s: %w", formatKAPDate(from), formatKAPDate(to), err)
	}
	return disclosures, nil
}

func wantsAllDisclosureTypes(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case AllDisclosureTypes, "*", "tum", "tüm":
			return true
		}
	}
	return false
}

func normalizeDisclosureDateRange(from time.Time, to time.Time) (time.Time, time.Time) {
	now := time.Now().UTC()
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = time.Date(to.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	}
	from = dateOnly(from)
	to = dateOnly(to)
	if to.Before(from) {
		from, to = to, from
	}
	return from, to
}

func dateOnly(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func formatKAPDate(value time.Time) string {
	return dateOnly(value).Format("02.01.2006")
}

func isKAPDisclosureListEndpoint(url string) bool {
	return strings.Contains(url, "/api/disclosure/list/")
}

func ImportFinancialDisclosureList(ctx context.Context, store *storage.EquityStore, disclosures []FinancialDisclosure) (int, int, error) {
	knownTickers, aliases, titles, err := knownDisclosureTickers(ctx, store)
	if err != nil {
		return 0, 0, err
	}
	byTicker := map[string][]FinancialDisclosure{}
	for _, disclosure := range disclosures {
		for _, ticker := range resolveDisclosureTickers(disclosure, knownTickers, aliases, titles) {
			disclosure := disclosure
			disclosure.Ticker = ticker
			byTicker[ticker] = append(byTicker[ticker], disclosure)
		}
	}
	updated := 0
	resolved := 0
	for ticker, tickerDisclosures := range byTicker {
		select {
		case <-ctx.Done():
			return updated, resolved, ctx.Err()
		default:
		}
		existing, err := LoadFinancialDisclosures(store.KAPDisclosuresPath(ticker))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return updated, resolved, err
		}
		tickerDisclosures = mergeFinancialDisclosures(existing, tickerDisclosures)
		if err := util.WriteJSON(store.KAPDisclosuresPath(ticker), tickerDisclosures); err != nil {
			return updated, resolved, err
		}
		var info domain.BilancoInfo
		if err := util.ReadJSON(store.FinancialInfoPath(ticker), &info); err == nil && len(info.Data) > 0 {
			domain.NormalizeBilancoInfo(&info, ticker)
			resolved += ApplyFinancialPublishDates(&info, tickerDisclosures)
			if err := util.WriteJSON(store.FinancialInfoPath(ticker), &info); err != nil {
				return updated, resolved, err
			}
			if err := refreshStatementVersionMetadata(store, ticker, &info); err != nil {
				return updated, resolved, err
			}
			if err := store.Update(ticker, func(e *domain.Equity) error {
				e.BilancoInfo = &info
				return nil
			}); err != nil {
				return updated, resolved, err
			}
		} else if err != nil && !os.IsNotExist(err) {
			return updated, resolved, err
		}
		updated++
	}
	return updated, resolved, nil
}

func knownDisclosureTickers(ctx context.Context, store *storage.EquityStore) (map[string]bool, map[string][]string, map[string]string, error) {
	equities, err := store.List()
	if err != nil {
		return nil, nil, nil, err
	}
	known := map[string]bool{}
	aliases := map[string][]string{}
	titles := map[string]string{}
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		default:
		}
		ticker := storage.NormalizeTicker(equity.Ticker)
		if ticker == "" {
			continue
		}
		known[ticker] = true
		addDisclosureAlias(aliases, ticker, ticker)
		if equity.KAPInfo != nil {
			titles[ticker] = firstNonEmpty(titles[ticker], firstString(equity.KAPInfo, "kapMemberTitle", "companyTitle"))
			for _, alias := range splitStockCodes(firstString(equity.KAPInfo, "stockCode")) {
				addDisclosureAlias(aliases, alias, ticker)
			}
		}
		var kapInfo map[string]any
		if err := util.ReadJSON(store.KAPPath(ticker), &kapInfo); err == nil {
			titles[ticker] = firstNonEmpty(titles[ticker], firstString(kapInfo, "kapMemberTitle", "companyTitle"))
			for _, alias := range splitStockCodes(firstString(kapInfo, "stockCode")) {
				addDisclosureAlias(aliases, alias, ticker)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil, err
		}
	}
	return known, aliases, titles, nil
}

func addDisclosureAlias(aliases map[string][]string, alias string, ticker string) {
	alias = storage.NormalizeTicker(alias)
	ticker = storage.NormalizeTicker(ticker)
	if alias == "" || ticker == "" {
		return
	}
	for _, existing := range aliases[alias] {
		if existing == ticker {
			return
		}
	}
	aliases[alias] = append(aliases[alias], ticker)
}

func resolveDisclosureTickers(disclosure FinancialDisclosure, known map[string]bool, aliases map[string][]string, titles map[string]string) []string {
	candidates := disclosureTickerCandidates(disclosure)
	disclosureTitle := disclosureCompanyTitle(disclosure)
	resolved := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if known[candidate] && !seen[candidate] && compatibleDisclosureTitle(disclosureTitle, titles[candidate]) {
			resolved = append(resolved, candidate)
			seen[candidate] = true
		}
		for _, ticker := range aliases[candidate] {
			if !seen[ticker] && compatibleDisclosureTitle(disclosureTitle, titles[ticker]) {
				resolved = append(resolved, ticker)
				seen[ticker] = true
			}
		}
	}
	sort.Strings(resolved)
	return resolved
}

func disclosureCompanyTitle(disclosure FinancialDisclosure) string {
	if disclosure.Raw == nil {
		return ""
	}
	if title := firstString(disclosure.Raw, "companyTitle", "kapMemberTitle"); title != "" {
		return title
	}
	for _, key := range []string{"disclosureBasic", "disclosureDetail"} {
		nested, ok := disclosure.Raw[key].(map[string]any)
		if !ok {
			continue
		}
		if title := firstString(nested, "companyTitle", "kapMemberTitle"); title != "" {
			return title
		}
	}
	return ""
}

func compatibleDisclosureTitle(disclosureTitle string, tickerTitle string) bool {
	disclosureTitle = normalizeCompanyTitle(disclosureTitle)
	tickerTitle = normalizeCompanyTitle(tickerTitle)
	if disclosureTitle == "" || tickerTitle == "" {
		return true
	}
	return disclosureTitle == tickerTitle ||
		strings.Contains(disclosureTitle, tickerTitle) ||
		strings.Contains(tickerTitle, disclosureTitle)
}

func normalizeCompanyTitle(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"ı", "i",
		"ğ", "g",
		"ü", "u",
		"ş", "s",
		"ö", "o",
		"ç", "c",
		".", "",
		",", "",
		"-", "",
		" ", "",
	)
	return replacer.Replace(value)
}

func disclosureTickerCandidates(disclosure FinancialDisclosure) []string {
	values := []string{disclosure.Ticker}
	if disclosure.Raw != nil {
		values = append(values, firstString(disclosure.Raw, "stockCode", "symbol", "ticker"))
	}
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, ticker := range splitStockCodes(value) {
			if ticker == "" || seen[ticker] {
				continue
			}
			out = append(out, ticker)
			seen[ticker] = true
		}
	}
	return out
}

func RefreshFinancialPublishDateMetadata(ctx context.Context, store *storage.EquityStore) (int, int, error) {
	equities, err := store.List()
	if err != nil {
		return 0, 0, err
	}
	updated := 0
	resolved := 0
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return updated, resolved, ctx.Err()
		default:
		}
		if equity.BilancoInfo == nil || len(equity.BilancoInfo.Data) == 0 {
			continue
		}
		domain.NormalizeBilancoInfo(equity.BilancoInfo, equity.Ticker)
		count, err := ApplyFinancialPublishDatesFromStore(store, equity.Ticker, equity.BilancoInfo)
		if err != nil {
			return updated, resolved, err
		}
		if err := util.WriteJSON(store.FinancialInfoPath(equity.Ticker), equity.BilancoInfo); err != nil {
			return updated, resolved, err
		}
		if err := refreshStatementVersionMetadata(store, equity.Ticker, equity.BilancoInfo); err != nil {
			return updated, resolved, err
		}
		if err := store.Update(equity.Ticker, func(e *domain.Equity) error {
			e.BilancoInfo = equity.BilancoInfo
			return nil
		}); err != nil {
			return updated, resolved, err
		}
		updated++
		resolved += count
	}
	return updated, resolved, nil
}

func refreshStatementVersionMetadata(store *storage.EquityStore, ticker string, info *domain.BilancoInfo) error {
	var existing domain.FinancialStatementVersionStore
	if err := util.ReadJSON(store.FinancialStatementVersionsPath(ticker), &existing); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	now := time.Now().UTC()
	incoming := domain.BuildStatementVersions(info, now)
	versionStore := domain.UpsertStatementVersions(existing, ticker, incoming, now)
	if len(versionStore.Versions) == 0 {
		return nil
	}
	return util.WriteJSON(store.FinancialStatementVersionsPath(ticker), versionStore)
}

func ApplyFinancialPublishDatesFromStore(store *storage.EquityStore, ticker string, info *domain.BilancoInfo) (int, error) {
	disclosures, err := LoadFinancialDisclosures(store.KAPDisclosuresPath(ticker))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return ApplyFinancialPublishDates(info, disclosures), nil
}

func LoadFinancialDisclosures(path string) ([]FinancialDisclosure, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseFinancialDisclosures(raw)
}

func parseFinancialDisclosures(raw []byte) ([]FinancialDisclosure, error) {
	var generic []map[string]any
	if err := json.Unmarshal(raw, &generic); err == nil {
		out := make([]FinancialDisclosure, 0, len(generic))
		for _, item := range generic {
			out = append(out, disclosureFromMap(item))
		}
		return normalizeFinancialDisclosures(out), nil
	}
	var list []FinancialDisclosure
	if err := json.Unmarshal(raw, &list); err == nil {
		return normalizeFinancialDisclosures(list), nil
	}
	var file financialDisclosureFile
	if err := json.Unmarshal(raw, &file); err == nil {
		if len(file.Disclosures) > 0 {
			return normalizeFinancialDisclosures(file.Disclosures), nil
		}
		if len(file.Value) > 0 {
			return normalizeFinancialDisclosures(file.Value), nil
		}
	}
	return nil, errors.New("unsupported KAP financial disclosures JSON shape")
}

func ApplyFinancialPublishDates(info *domain.BilancoInfo, disclosures []FinancialDisclosure) int {
	if info == nil {
		return 0
	}
	domain.NormalizeBilancoInfo(info, info.Ticker)
	byPeriod := map[string]FinancialDisclosure{}
	for _, disclosure := range normalizeFinancialDisclosures(disclosures) {
		if disclosure.PublishDate == nil || disclosure.PeriodKey == "" || !isFinancialDisclosure(disclosure) {
			continue
		}
		existing, ok := byPeriod[disclosure.PeriodKey]
		if !ok || existing.PublishDate == nil || disclosure.PublishDate.Before(*existing.PublishDate) {
			byPeriod[disclosure.PeriodKey] = disclosure
		}
	}
	resolved := 0
	for key, period := range info.Periods {
		disclosure, ok := byPeriod[key]
		if !ok || disclosure.PublishDate == nil {
			continue
		}
		period.PublishDate = disclosure.PublishDate
		period.AvailableAt = disclosure.PublishDate
		period.AvailabilitySource = "kap_publish_date"
		period.ReportDate = disclosure.PublishDate
		period.BacktestSafe = true
		period.Source = firstNonEmpty(disclosure.Source, period.Source, info.Source)
		period.Warnings = removeString(period.Warnings, "publish_date_missing")
		period.Warnings = removeString(period.Warnings, "available_at_missing")
		if looksLikeRestatement(disclosure) && !containsString(period.Warnings, "kap_restatement_disclosure_seen") {
			period.Warnings = append(period.Warnings, "kap_restatement_disclosure_seen")
		}
		info.Periods[key] = period
		resolved++
	}
	info.Quality = domain.ValidateFinancialDataQuality(info, time.Time{})
	if resolved > 0 {
		domain.AppendLineage(info, domain.DataLineageEvent{
			Stage:     "kap_publish_date_resolution",
			Source:    "kap_disclosures",
			Transform: "match_financial_disclosure_periods",
			Version:   domain.FinancialMetadataVersion,
			CreatedAt: time.Now().UTC(),
		})
	}
	return resolved
}

func normalizeFinancialDisclosures(in []FinancialDisclosure) []FinancialDisclosure {
	out := make([]FinancialDisclosure, 0, len(in))
	for _, disclosure := range in {
		disclosure.Ticker = storage.NormalizeTicker(disclosure.Ticker)
		if disclosure.Source == "" {
			disclosure.Source = "kap_disclosures"
		}
		if disclosure.ReceivedAt.IsZero() {
			disclosure.ReceivedAt = time.Now().UTC()
		}
		if disclosure.PeriodKey == "" {
			disclosure.PeriodKey = domain.FinancialPeriodKey(disclosure.FiscalYear, disclosure.FiscalQuarter)
		}
		if disclosure.PeriodKey == "" {
			year, quarter := inferPeriodFromText(disclosure.Title + " " + disclosure.Subject + " " + disclosure.Summary)
			disclosure.FiscalYear = year
			disclosure.FiscalQuarter = quarter
			disclosure.PeriodKey = domain.FinancialPeriodKey(year, quarter)
		}
		if disclosure.ID == "" {
			disclosure.ID = disclosureID(disclosure)
		}
		out = append(out, disclosure)
	}
	return out
}

func disclosureFromMap(item map[string]any) FinancialDisclosure {
	source := flattenDisclosureItem(item)
	title := firstString(source, "title", "subject", "header", "baslik", "bildirimBasligi")
	subject := firstString(source, "subject", "type", "konu", "bildirimTipi")
	summary := firstString(source, "summary", "description", "ozet", "aciklama")
	year := firstInt(source, "fiscal_year", "year", "donem_yili", "periodYear")
	quarter := firstInt(source, "fiscal_quarter", "quarter")
	period := firstString(source, "period")
	term := firstInt(source, "donem", "period", "periodMonth")
	if quarter == 0 {
		quarter = quarterFromDisclosurePeriod(period, term)
	}
	publishDate := firstTime(source, "publish_date", "publishDate", "disclosureDate", "date", "bildirim_tarihi", "bildirimTarihi")
	return FinancialDisclosure{
		ID:                 firstString(source, "id", "disclosureId", "bildirimId"),
		Ticker:             firstString(source, "ticker", "stockCode", "symbol"),
		Title:              title,
		Subject:            subject,
		Summary:            summary,
		DisclosureClass:    firstString(source, "disclosure_class", "disclosureClass"),
		DisclosureType:     firstString(source, "disclosure_type", "disclosureType"),
		DisclosureCategory: firstString(source, "disclosure_category", "disclosureCategory"),
		DisclosureIndex:    firstInt(source, "disclosure_index", "disclosureIndex"),
		AttachmentCount:    firstInt(source, "attachment_count", "attachmentCount"),
		PeriodKey:          firstString(source, "period_key", "periodKey"),
		FiscalYear:         year,
		FiscalQuarter:      normalizeQuarter(quarter, title+" "+subject+" "+summary),
		PublishDate:        publishDate,
		Source:             firstString(source, "source"),
		URL:                firstString(source, "url", "link", "disclosureUrl", "bildirimUrl"),
		Raw:                item,
	}
}

func flattenDisclosureItem(item map[string]any) map[string]any {
	flat := map[string]any{}
	for key, value := range item {
		if key == "disclosureBasic" || key == "disclosureDetail" {
			continue
		}
		flat[key] = value
	}
	for _, key := range []string{"disclosureBasic", "disclosureDetail"} {
		nested, ok := item[key].(map[string]any)
		if !ok {
			continue
		}
		for nestedKey, value := range nested {
			if _, exists := flat[nestedKey]; !exists {
				flat[nestedKey] = value
			}
		}
	}
	return flat
}

func mergeFinancialDisclosures(existing []FinancialDisclosure, incoming []FinancialDisclosure) []FinancialDisclosure {
	merged := map[string]FinancialDisclosure{}
	for _, disclosure := range normalizeFinancialDisclosures(existing) {
		merged[disclosureIdentity(disclosure)] = disclosure
	}
	for _, disclosure := range normalizeFinancialDisclosures(incoming) {
		key := disclosureIdentity(disclosure)
		if current, ok := merged[key]; ok {
			merged[key] = preferFinancialDisclosure(current, disclosure)
			continue
		}
		merged[key] = disclosure
	}
	out := make([]FinancialDisclosure, 0, len(merged))
	for _, disclosure := range merged {
		out = append(out, disclosure)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		if left.Ticker != right.Ticker {
			return left.Ticker < right.Ticker
		}
		if left.PublishDate != nil && right.PublishDate != nil && !left.PublishDate.Equal(*right.PublishDate) {
			return left.PublishDate.Before(*right.PublishDate)
		}
		if left.PeriodKey != right.PeriodKey {
			return left.PeriodKey < right.PeriodKey
		}
		return left.ID < right.ID
	})
	return out
}

func preferFinancialDisclosure(current FinancialDisclosure, incoming FinancialDisclosure) FinancialDisclosure {
	if current.PublishDate == nil && incoming.PublishDate != nil {
		return incoming
	}
	if current.Title == "" && incoming.Title != "" {
		current.Title = incoming.Title
	}
	if current.Subject == "" && incoming.Subject != "" {
		current.Subject = incoming.Subject
	}
	if current.Summary == "" && incoming.Summary != "" {
		current.Summary = incoming.Summary
	}
	if current.URL == "" && incoming.URL != "" {
		current.URL = incoming.URL
	}
	if current.DisclosureIndex == 0 && incoming.DisclosureIndex != 0 {
		current.DisclosureIndex = incoming.DisclosureIndex
	}
	if current.AttachmentCount == 0 && incoming.AttachmentCount != 0 {
		current.AttachmentCount = incoming.AttachmentCount
	}
	if current.Raw == nil && incoming.Raw != nil {
		current.Raw = incoming.Raw
	}
	return current
}

func disclosureIdentity(disclosure FinancialDisclosure) string {
	if disclosure.ID != "" {
		return disclosure.ID
	}
	return disclosureID(disclosure)
}

func disclosureID(disclosure FinancialDisclosure) string {
	publish := ""
	if disclosure.PublishDate != nil {
		publish = disclosure.PublishDate.UTC().Format(time.RFC3339)
	}
	raw := strings.Join([]string{
		storage.NormalizeTicker(disclosure.Ticker),
		disclosure.PeriodKey,
		publish,
		strings.TrimSpace(disclosure.Title),
		strings.TrimSpace(disclosure.Subject),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:24]
}

func isFinancialDisclosure(disclosure FinancialDisclosure) bool {
	if strings.EqualFold(strings.TrimSpace(disclosure.DisclosureClass), "FR") || strings.EqualFold(strings.TrimSpace(disclosure.DisclosureType), "FR") {
		return true
	}
	text := strings.ToLower(disclosure.Title + " " + disclosure.Subject + " " + disclosure.Summary)
	return strings.Contains(text, "finansal") ||
		strings.Contains(text, "mali tablo") ||
		strings.Contains(text, "financial") ||
		strings.Contains(text, "balance") ||
		strings.Contains(text, "bilan")
}

func looksLikeRestatement(disclosure FinancialDisclosure) bool {
	text := strings.ToLower(disclosure.Title + " " + disclosure.Subject + " " + disclosure.Summary)
	return strings.Contains(text, "düzelt") ||
		strings.Contains(text, "duzelt") ||
		strings.Contains(text, "revize") ||
		strings.Contains(text, "restatement") ||
		strings.Contains(text, "revised")
}

func inferPeriodFromText(text string) (int, int) {
	reYear := regexp.MustCompile(`20[0-9]{2}`)
	year := 0
	if match := reYear.FindString(text); match != "" {
		year, _ = strconv.Atoi(match)
	}
	return year, normalizeQuarter(0, text)
}

func normalizeQuarter(value int, text string) int {
	if value >= 1 && value <= 4 {
		return value
	}
	if quarter := quarterFromMonthPeriod(value); quarter != 0 {
		return quarter
	}
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "q1") || strings.Contains(t, "1. çeyrek") || strings.Contains(t, "3 ayl"):
		return 1
	case strings.Contains(t, "q2") || strings.Contains(t, "2. çeyrek") || strings.Contains(t, "6 ayl"):
		return 2
	case strings.Contains(t, "q3") || strings.Contains(t, "3. çeyrek") || strings.Contains(t, "9 ayl"):
		return 3
	case strings.Contains(t, "q4") || strings.Contains(t, "4. çeyrek") || strings.Contains(t, "12 ayl") || strings.Contains(t, "yıllık") || strings.Contains(t, "yillik"):
		return 4
	default:
		return 0
	}
}

func quarterFromDisclosurePeriod(period string, term int) int {
	period = strings.ToUpper(strings.TrimSpace(period))
	switch period {
	case "3AB":
		if term >= 1 && term <= 4 {
			return term
		}
	case "6AB":
		if term == 1 {
			return 2
		}
		if term == 2 {
			return 4
		}
	case "9AB":
		return 3
	case "12AB", "YB":
		return 4
	}
	return quarterFromMonthPeriod(term)
}

func quarterFromMonthPeriod(value int) int {
	switch value {
	case 3:
		return 1
	case 6:
		return 2
	case 9:
		return 3
	case 12:
		return 4
	default:
		return 0
	}
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstInt(item map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := item[key].(type) {
		case float64:
			return int(value)
		case int:
			return value
		case string:
			parsed, _ := strconv.Atoi(strings.TrimSpace(value))
			if parsed != 0 {
				return parsed
			}
		}
	}
	return 0
}

func firstTime(item map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if parsed, ok := parseKAPTime(value); ok {
			return &parsed
		}
	}
	return nil
}

func parseKAPTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return time.Time{}, false
		}
		if parsed, err := time.Parse(time.RFC3339, text); err == nil {
			return parsed.UTC(), true
		}
		location := kapTimeLocation()
		localLayouts := []string{
			"2006-01-02 15:04:05",
			"2006-01-02",
			"02.01.2006 15:04:05",
			"02.01.2006",
		}
		for _, layout := range localLayouts {
			if parsed, err := time.ParseInLocation(layout, text, location); err == nil {
				return parsed.UTC(), true
			}
		}
	case time.Time:
		return typed.UTC(), true
	}
	return time.Time{}, false
}

func kapTimeLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return time.FixedZone("Europe/Istanbul", 3*60*60)
	}
	return location
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
