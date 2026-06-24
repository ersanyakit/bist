package kap

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/datasources"
	"hissebot/internal/domain"
	"hissebot/internal/domain/disclosures"
	"hissebot/internal/domain/financials"
	servicekap "hissebot/internal/services/kap"
	"hissebot/internal/storage"
)

type Provider struct {
	BaseURL     string
	Token       string
	EquitiesDir string
	Store       *storage.EquityStore
	Timeout     time.Duration
}

func New(baseURL string, token string) Provider {
	return Provider{BaseURL: baseURL, Token: token}
}

func NewWithStore(baseURL string, store *storage.EquityStore) Provider {
	return Provider{BaseURL: baseURL, Store: store}
}

func (p Provider) Info() datasources.ProviderInfo {
	return datasources.ProviderInfo{
		Name:         "KAP",
		SourceURL:    firstNonEmpty(p.BaseURL, "https://www.kap.org.tr"),
		License:      "official public disclosure source; disclosure list endpoint does not require an API key; respect KAP terms and rate limits",
		RequiresKey:  false,
		Capabilities: []string{"financial_statements", "disclosures", "annual_reports", "audit_reports", "corporate_actions"},
	}
}

func (p Provider) GetBalanceSheet(ctx context.Context, symbol string, period financials.Period) (financials.BalanceSheet, error) {
	info, err := p.loadBilancoInfo(ctx, symbol)
	if err != nil {
		return financials.BalanceSheet{}, err
	}
	period = normalizePeriod(period)
	meta := sourceMeta(info, period)
	currentLiabilities, _ := valueAt(info, "2A", period)
	nonCurrentLiabilities, _ := valueAt(info, "2B", period)
	totalLiabilities := currentLiabilities + nonCurrentLiabilities
	if totalLiabilities == 0 {
		totalSources, okSources := valueAt(info, "2ODB", period)
		equity, okEquity := valueAt(info, "2N", period)
		if okSources && okEquity {
			totalLiabilities = totalSources - equity
		}
	}
	return financials.BalanceSheet{
		Symbol:             storage.NormalizeTicker(symbol),
		Period:             period,
		TotalAssets:        firstValue(info, period, "1BL", "1Z", "A1AK"),
		TotalLiabilities:   totalLiabilities,
		TotalEquity:        firstValue(info, period, "2N", "2O"),
		CashAndEquivalents: firstValue(info, period, "1AA"),
		Inventory:          firstValue(info, period, "1AF"),
		TradeReceivables:   firstValue(info, period, "1AC"),
		ShortTermDebt:      firstValue(info, period, "2AA", "2AAG"),
		LongTermDebt:       firstValue(info, period, "2BA"),
		Lines:              statementLines(info, period, "1", "2"),
		Meta:               meta,
	}, nil
}

func (p Provider) GetIncomeStatement(ctx context.Context, symbol string, period financials.Period) (financials.IncomeStatement, error) {
	info, err := p.loadBilancoInfo(ctx, symbol)
	if err != nil {
		return financials.IncomeStatement{}, err
	}
	period = normalizePeriod(period)
	return financials.IncomeStatement{
		Symbol:      storage.NormalizeTicker(symbol),
		Period:      period,
		Sales:       firstValue(info, period, "3C"),
		GrossProfit: firstValue(info, period, "3D"),
		EBIT:        firstValue(info, period, "3DF"),
		NetIncome:   firstValue(info, period, "3L"),
		EPS:         firstValue(info, period, "3ZF", "3ZG"),
		Lines:       statementLines(info, period, "3"),
		Meta:        sourceMeta(info, period),
	}, nil
}

func (p Provider) GetCashFlowStatement(ctx context.Context, symbol string, period financials.Period) (financials.CashFlowStatement, error) {
	info, err := p.loadBilancoInfo(ctx, symbol)
	if err != nil {
		return financials.CashFlowStatement{}, err
	}
	period = normalizePeriod(period)
	return financials.CashFlowStatement{
		Symbol:            storage.NormalizeTicker(symbol),
		Period:            period,
		OperatingCashFlow: firstValue(info, period, "4C"),
		InvestingCashFlow: firstValue(info, period, "4CAK"),
		FinancingCashFlow: firstValue(info, period, "4CBE"),
		Capex:             math.Abs(firstValue(info, period, "4CAJ")),
		FreeCashFlow:      firstValue(info, period, "4CB"),
		EndCash:           firstValue(info, period, "4CBL"),
		Lines:             statementLines(info, period, "4"),
		Meta:              sourceMeta(info, period),
	}, nil
}

func (p Provider) GetEquityStatement(ctx context.Context, symbol string, period financials.Period) (financials.EquityStatement, error) {
	info, err := p.loadBilancoInfo(ctx, symbol)
	if err != nil {
		return financials.EquityStatement{}, err
	}
	period = normalizePeriod(period)
	return financials.EquityStatement{
		Symbol:             storage.NormalizeTicker(symbol),
		Period:             period,
		ClosingEquity:      firstValue(info, period, "2N", "2O"),
		PaidInCapital:      firstValue(info, period, "2OA"),
		Dividends:          math.Abs(firstValue(info, period, "4CBB")),
		ShareIssueProceeds: firstValue(info, period, "4CBC"),
		Lines:              statementLines(info, period, "2O", "4CB"),
		Meta:               sourceMeta(info, period),
	}, nil
}

func (p Provider) GetFinancialNotes(ctx context.Context, symbol string, period financials.Period) ([]financials.FinancialNote, error) {
	items, err := p.GetDisclosures(ctx, symbol, periodStart(period), periodEnd(period))
	if err != nil {
		return nil, err
	}
	notes := make([]financials.FinancialNote, 0, len(items))
	for _, item := range items {
		notes = append(notes, financials.FinancialNote{
			Symbol:    item.Symbol,
			Period:    normalizePeriod(period),
			Topic:     firstNonEmpty(item.Category, item.DisclosureType, item.Title),
			Text:      strings.TrimSpace(strings.Join([]string{item.Title, item.Summary, item.Body}, "\n")),
			SourceURL: item.URL,
			Meta: financials.SourceMeta{
				Source:      item.Meta.Source,
				SourceURL:   item.Meta.SourceURL,
				Currency:    "TRY",
				DataVersion: item.Meta.DataVersion,
				AsOf:        item.PublishedAt,
				IngestedAt:  time.Now().UTC(),
			},
		})
	}
	return notes, nil
}

func (p Provider) GetDisclosures(ctx context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.Disclosure, error) {
	local, localErr := p.loadLocalDisclosures(symbol, from, to)
	if localErr == nil && len(local) > 0 {
		return local, nil
	}
	live, liveErr := p.fetchLiveDisclosures(ctx, symbol, from, to)
	if liveErr == nil {
		return live, nil
	}
	if localErr != nil {
		return nil, localErr
	}
	return nil, liveErr
}

func (p Provider) GetMaterialEvents(ctx context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.MaterialEvent, error) {
	items, err := p.GetDisclosures(ctx, symbol, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]disclosures.MaterialEvent, 0, len(items))
	for _, item := range items {
		out = append(out, disclosures.MaterialEvent{
			ID:           item.ID,
			Symbol:       item.Symbol,
			EventType:    firstNonEmpty(item.Category, item.DisclosureType, "kap_disclosure"),
			Title:        item.Title,
			Impact:       classifyDisclosureImpact(item),
			PublishedAt:  item.PublishedAt,
			DisclosureID: item.ID,
			Meta:         item.Meta,
		})
	}
	return out, nil
}

func (p Provider) loadBilancoInfo(ctx context.Context, symbol string) (*domain.BilancoInfo, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	path := p.financialInfoPath(symbol)
	if path == "" {
		return nil, datasources.ErrNotConfigured
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info domain.BilancoInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}
	if len(info.Data) == 0 {
		return nil, fmt.Errorf("kap financial statements %s: no statement data in %s", storage.NormalizeTicker(symbol), path)
	}
	domain.NormalizeBilancoInfo(&info, storage.NormalizeTicker(symbol))
	if p.Store != nil {
		_, _ = servicekap.ApplyFinancialPublishDatesFromStore(p.Store, symbol, &info)
	}
	return &info, nil
}

func (p Provider) financialInfoPath(symbol string) string {
	symbol = storage.NormalizeTicker(symbol)
	if symbol == "" {
		return ""
	}
	if p.Store != nil {
		return p.Store.FinancialInfoPath(symbol)
	}
	if p.EquitiesDir != "" {
		return filepath.Join(p.EquitiesDir, symbol, "financials", "bilanco.json")
	}
	return ""
}

func (p Provider) disclosurePath(symbol string) string {
	symbol = storage.NormalizeTicker(symbol)
	if symbol == "" {
		return ""
	}
	if p.Store != nil {
		return p.Store.KAPDisclosuresPath(symbol)
	}
	if p.EquitiesDir != "" {
		return filepath.Join(p.EquitiesDir, symbol, "kap_disclosures.json")
	}
	return ""
}

func (p Provider) loadLocalDisclosures(symbol string, from time.Time, to time.Time) ([]disclosures.Disclosure, error) {
	path := p.disclosurePath(symbol)
	if path == "" {
		return nil, datasources.ErrNotConfigured
	}
	items, err := servicekap.LoadFinancialDisclosures(path)
	if err != nil {
		return nil, err
	}
	return mapDisclosures(symbol, items, from, to), nil
}

func (p Provider) fetchLiveDisclosures(ctx context.Context, symbol string, from time.Time, to time.Time) ([]disclosures.Disclosure, error) {
	opts := servicekap.FinancialDisclosureSyncOptions{
		URL:             disclosureListURL(p.BaseURL),
		FromDate:        from,
		ToDate:          to,
		ChunkDays:       90,
		MemberTypes:     []string{"IGS"},
		DisclosureTypes: []string{"FR", "ODA"},
	}
	cfg := minimalConfig(p.Timeout, p.Token)
	items, err := servicekap.FetchFinancialDisclosures(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}
	return mapDisclosures(symbol, items, from, to), nil
}

func mapDisclosures(symbol string, items []servicekap.FinancialDisclosure, from time.Time, to time.Time) []disclosures.Disclosure {
	symbol = storage.NormalizeTicker(symbol)
	out := make([]disclosures.Disclosure, 0, len(items))
	for _, item := range items {
		itemTicker := storage.NormalizeTicker(item.Ticker)
		if symbol != "" && itemTicker != "" && itemTicker != symbol {
			continue
		}
		publishedAt := time.Time{}
		if item.PublishDate != nil {
			publishedAt = item.PublishDate.UTC()
		}
		if !publishedAt.IsZero() && !inRange(publishedAt, from, to) {
			continue
		}
		if itemTicker == "" {
			itemTicker = symbol
		}
		out = append(out, disclosures.Disclosure{
			ID:             item.ID,
			Symbol:         itemTicker,
			Title:          item.Title,
			Summary:        item.Summary,
			Category:       item.DisclosureCategory,
			DisclosureType: item.DisclosureType,
			PublishedAt:    publishedAt,
			URL:            item.URL,
			Meta: disclosures.SourceMeta{
				Source:      firstNonEmpty(item.Source, "KAP"),
				SourceURL:   item.URL,
				DataVersion: item.ReceivedAt.Format(time.RFC3339Nano),
				AsOf:        publishedAt,
				IngestedAt:  time.Now().UTC(),
			},
		})
	}
	return out
}

func disclosureListURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || baseURL == "https://www.kap.org.tr" || baseURL == "https://kap.org.tr" {
		return "https://www.kap.org.tr/tr/api/disclosure/list/main"
	}
	if strings.Contains(baseURL, "/api/disclosure/list/") {
		return baseURL
	}
	return baseURL + "/tr/api/disclosure/list/main"
}

func minimalConfig(timeout time.Duration, token string) config.Config {
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return config.Config{KAPDisclosuresToken: token, HTTPTimeout: timeout}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizePeriod(period financials.Period) financials.Period {
	if period.Year == 0 && !period.EndDate.IsZero() {
		period.Year = period.EndDate.Year()
	}
	if period.Quarter == 0 {
		if period.Type == financials.PeriodAnnual {
			period.Quarter = 4
		} else if !period.EndDate.IsZero() {
			period.Quarter = int((period.EndDate.Month()-1)/3) + 1
		}
	}
	if period.Type == "" {
		if period.Quarter == 4 {
			period.Type = financials.PeriodAnnual
		} else {
			period.Type = financials.PeriodQuarterly
		}
	}
	if period.EndDate.IsZero() && period.Year > 0 && period.Quarter > 0 {
		period.EndDate = domain.FiscalPeriodEnd(period.Year, period.Quarter)
	}
	return period
}

func periodStart(period financials.Period) time.Time {
	period = normalizePeriod(period)
	if period.Year == 0 {
		return time.Time{}
	}
	if period.Quarter == 0 {
		return time.Date(period.Year, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	startMonth := time.Month((period.Quarter-1)*3 + 1)
	return time.Date(period.Year, startMonth, 1, 0, 0, 0, 0, time.UTC)
}

func periodEnd(period financials.Period) time.Time {
	period = normalizePeriod(period)
	return period.EndDate
}

func sourceMeta(info *domain.BilancoInfo, period financials.Period) financials.SourceMeta {
	period = normalizePeriod(period)
	source := firstNonEmpty(info.Source, "kap")
	currency := firstNonEmpty(info.Currency, "TRY")
	asOf := period.EndDate
	if fp, ok := info.Periods[domain.FinancialPeriodKey(period.Year, period.Quarter)]; ok {
		source = firstNonEmpty(fp.Source, source)
		currency = firstNonEmpty(fp.Currency, currency)
		if fp.PublishDate != nil {
			asOf = *fp.PublishDate
		} else if fp.AvailableAt != nil {
			asOf = *fp.AvailableAt
		}
	}
	return financials.SourceMeta{
		Source:        source,
		SourceURL:     "https://www.kap.org.tr",
		Currency:      currency,
		Consolidation: info.FinancialGroup,
		DataVersion:   domain.FinancialMetadataVersion,
		AsOf:          asOf,
		IngestedAt:    time.Now().UTC(),
	}
}

func statementLines(info *domain.BilancoInfo, period financials.Period, prefixes ...string) map[string]financials.StatementLine {
	out := map[string]financials.StatementLine{}
	for code, field := range info.Data {
		if !hasPrefix(code, prefixes...) {
			continue
		}
		value, ok := valueAt(info, code, period)
		if !ok {
			continue
		}
		out[code] = financials.StatementLine{
			Code:  code,
			Name:  firstNonEmpty(field.DescTR, field.DescEN, code),
			Value: value,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func hasPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func firstValue(info *domain.BilancoInfo, period financials.Period, codes ...string) float64 {
	for _, code := range codes {
		if value, ok := valueAt(info, code, period); ok {
			return value
		}
	}
	return 0
}

func valueAt(info *domain.BilancoInfo, code string, period financials.Period) (float64, bool) {
	period = normalizePeriod(period)
	if info == nil || period.Year <= 0 || period.Quarter < 1 || period.Quarter > 4 {
		return 0, false
	}
	field, ok := info.Data[code]
	if !ok {
		return 0, false
	}
	values := field.Years[strconv.Itoa(period.Year)]
	index := indexFromQuarter(period.Quarter)
	if index < 0 || index >= len(values) || values[index] == nil {
		return 0, false
	}
	return *values[index], true
}

func indexFromQuarter(quarter int) int {
	switch quarter {
	case 4:
		return 0
	case 3:
		return 1
	case 2:
		return 2
	case 1:
		return 3
	default:
		return -1
	}
}

func inRange(ts time.Time, from time.Time, to time.Time) bool {
	if ts.IsZero() {
		return true
	}
	if !from.IsZero() && ts.Before(from) {
		return false
	}
	if !to.IsZero() && ts.After(to) {
		return false
	}
	return true
}

func classifyDisclosureImpact(item disclosures.Disclosure) string {
	text := strings.ToLower(item.Title + " " + item.Summary)
	switch {
	case strings.Contains(text, "finansal rapor"), strings.Contains(text, "faaliyet raporu"):
		return "financial_reporting"
	case strings.Contains(text, "temettü"), strings.Contains(text, "kar pay"):
		return "capital_return"
	case strings.Contains(text, "sermaye"):
		return "capital_action"
	default:
		return "informational"
	}
}
