package financials

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/services/kap"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

var Years = []int{2026, 2025, 2024, 2023, 2022, 2021, 2020, 2019, 2018, 2017, 2016, 2015, 2014, 2013, 2012, 2011, 2010, 2009, 2008}
var FinancialGroups = []string{"XI_29", "UFRS_K", "UFRS"}
var DefaultFetchWorkers = len(Years)

const DefaultFetchRetries = 3
const DefaultTickerDelay = time.Second

type IsYatirimResponse struct {
	Source         string          `json:"source,omitempty"`
	Ticker         string          `json:"ticker,omitempty"`
	Year           int             `json:"year,omitempty"`
	FinancialGroup string          `json:"financial_group,omitempty"`
	FetchedAt      time.Time       `json:"fetched_at,omitempty"`
	Value          []IsYatirimLine `json:"value"`
}

type IsYatirimLine struct {
	ItemCode    string `json:"itemCode"`
	ItemDescTR  string `json:"itemDescTr"`
	ItemDescEng string `json:"itemDescEng"`
	Value1      string `json:"value1"`
	Value2      string `json:"value2"`
	Value3      string `json:"value3"`
	Value4      string `json:"value4"`
}

type FetchOptions struct {
	Force        bool
	ForceHistory bool
	Ticker       string
	Limit        int
	Workers      int
	Retries      int
	TickerDelay  time.Duration
}

type fetchJob struct {
	Ticker    string
	Year      int
	CachePath string
}

type fetchResult struct {
	Ticker   string
	Year     int
	Group    string
	Fetched  bool
	SkipErr  error
	FatalErr error
}

type runJob struct {
	Ticker string
	Year   int
}

type runResult struct {
	Ticker         string
	Year           int
	Source         string
	FinancialGroup string
	FetchedAt      time.Time
	Lines          []IsYatirimLine
	Fetched        bool
	SkipErr        error
	FatalErr       error
}

type runEquityState struct {
	Equity  *domain.Equity
	Info    *domain.BilancoInfo
	Pending int
	Fetched int
}

type runEquityWork struct {
	State *runEquityState
	Jobs  []runJob
}

func Fetch(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts FetchOptions) error {
	equities, err := fetchTargets(store, opts)
	if err != nil {
		return err
	}
	workers := opts.Workers
	if workers < 1 {
		workers = 1
	}
	jobs := make([]fetchJob, 0, len(equities)*len(Years))
	for _, equity := range equities {
		if equity.AssetType != 2 {
			continue
		}
		for _, year := range Years {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			cachePath := periodPath(cfg, store, equity.Ticker, year)
			if !opts.Force {
				if _, err := os.Stat(cachePath); err == nil {
					continue
				}
			}
			jobs = append(jobs, fetchJob{
				Ticker:    equity.Ticker,
				Year:      year,
				CachePath: cachePath,
			})
		}
	}

	if len(jobs) == 0 {
		fmt.Printf("financials: 0 period files fetched\n")
		return nil
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	fmt.Printf("financials: fetching %d period files with %d workers\n", len(jobs), workers)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	jobCh := make(chan fetchJob)
	resultCh := make(chan fetchResult, workers)
	fetched := 0

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				result := fetchOnePeriod(runCtx, cfg, client, job, opts.Retries)
				select {
				case resultCh <- result:
				case <-runCtx.Done():
					return
				}
				if result.FatalErr != nil {
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobCh)
		for _, job := range jobs {
			select {
			case jobCh <- job:
			case <-runCtx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var firstErr error
	for result := range resultCh {
		if result.FatalErr != nil {
			if firstErr == nil {
				firstErr = result.FatalErr
				cancel()
			}
			continue
		}
		if result.SkipErr != nil {
			fmt.Printf("financials: %s %d skipped: %v\n", result.Ticker, result.Year, result.SkipErr)
			continue
		}
		if result.Fetched {
			fetched++
			fmt.Printf("financials: fetched %s %d %s\n", result.Ticker, result.Year, result.Group)
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Printf("financials: %d period files fetched\n", fetched)
	return nil
}

func Run(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts FetchOptions) error {
	equities, err := fetchTargets(store, opts)
	if err != nil {
		return err
	}
	workers := opts.Workers
	if workers < 1 {
		workers = len(Years)
	}
	tickerDelay := opts.TickerDelay
	if tickerDelay < 0 {
		tickerDelay = 0
	}

	works := make([]runEquityWork, 0, len(equities))
	totalJobs := 0
	for _, equity := range equities {
		if equity.AssetType != 2 {
			continue
		}
		info, err := existingBilancoInfo(store, equity, opts.ForceHistory)
		if err != nil {
			return err
		}
		state := &runEquityState{
			Equity: equity,
			Info:   info,
		}
		jobs := make([]runJob, 0, len(Years))
		for _, year := range Years {
			if !shouldFetchRunYear(info, year, opts) {
				continue
			}
			jobs = append(jobs, runJob{Ticker: equity.Ticker, Year: year})
			state.Pending++
		}
		if len(jobs) == 0 {
			continue
		}
		works = append(works, runEquityWork{
			State: state,
			Jobs:  jobs,
		})
		totalJobs += len(jobs)
	}
	if totalJobs == 0 {
		fmt.Printf("financials: 0 equities updated\n")
		return nil
	}
	if workers > len(Years) {
		workers = len(Years)
	}
	fmt.Printf("financials: processing %d equities / %d periods with %d year workers, ticker delay %s\n", len(works), totalJobs, workers, tickerDelay)

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	updated := 0
	fetched := 0
	for index, work := range works {
		fmt.Printf(
			"financials: active %s (%d/%d, %d periods)\n",
			work.State.Equity.Ticker,
			index+1,
			len(works),
			len(work.Jobs),
		)
		workFetched, workUpdated, err := runOneEquityYears(ctx, cfg, store, client, work, workers, opts.Retries)
		if err != nil {
			return err
		}
		fetched += workFetched
		if workUpdated {
			updated++
			fmt.Printf("financials: updated %s (%d periods fetched)\n", work.State.Equity.Ticker, workFetched)
		}
		if index < len(works)-1 && tickerDelay > 0 {
			if err := sleepDuration(ctx, tickerDelay); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Printf("financials: %d periods fetched\n", fetched)
	fmt.Printf("financials: %d equities updated\n", updated)
	return nil
}

func runOneEquityYears(ctx context.Context, cfg config.Config, store *storage.EquityStore, client *http.Client, work runEquityWork, workers int, retries int) (int, bool, error) {
	if len(work.Jobs) == 0 {
		return 0, false, nil
	}
	if workers < 1 {
		workers = len(Years)
	}
	if workers > len(work.Jobs) {
		workers = len(work.Jobs)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan runJob)
	resultCh := make(chan runResult, workers)

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				result := runOneYear(runCtx, cfg, client, job, retries)
				select {
				case resultCh <- result:
				case <-runCtx.Done():
					return
				}
				if result.FatalErr != nil {
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobCh)
		for _, job := range work.Jobs {
			select {
			case jobCh <- job:
			case <-runCtx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	fetched := 0
	var firstErr error
	for result := range resultCh {
		if result.FatalErr != nil {
			if firstErr == nil {
				firstErr = result.FatalErr
				cancel()
			}
			continue
		}
		work.State.Pending--
		if result.SkipErr != nil {
			fmt.Printf("financials: %s %d skipped: %v\n", result.Ticker, result.Year, result.SkipErr)
		} else if result.Fetched {
			mergePeriodIntoInfo(work.State.Info, result.Year, result.Lines, result.Source, result.FinancialGroup, result.FetchedAt)
			work.State.Fetched++
			fetched++
		}
	}
	if firstErr != nil {
		return fetched, false, firstErr
	}
	if err := ctx.Err(); err != nil {
		return fetched, false, err
	}
	if fetched == 0 {
		return fetched, false, nil
	}
	if len(work.State.Info.Data) == 0 {
		return fetched, false, nil
	}
	if err := writeBilancoEquity(store, work.State); err != nil {
		return fetched, false, err
	}
	return fetched, true, nil
}

func fetchOnePeriod(ctx context.Context, cfg config.Config, client *http.Client, job fetchJob, retries int) fetchResult {
	if err := os.MkdirAll(filepath.Dir(job.CachePath), 0o755); err != nil {
		return fetchResult{Ticker: job.Ticker, Year: job.Year, FatalErr: err}
	}

	data, group, err := fetchPeriodWithRetry(ctx, cfg, client, job.Ticker, job.Year, retries)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fetchResult{Ticker: job.Ticker, Year: job.Year, FatalErr: ctxErr}
		}
		return fetchResult{Ticker: job.Ticker, Year: job.Year, SkipErr: err}
	}
	if len(data.Value) == 0 {
		return fetchResult{Ticker: job.Ticker, Year: job.Year}
	}
	data.Source = "isyatirim"
	data.Ticker = job.Ticker
	data.Year = job.Year
	data.FinancialGroup = group
	data.FetchedAt = time.Now().UTC()
	if err := util.WriteJSON(job.CachePath, data); err != nil {
		return fetchResult{Ticker: job.Ticker, Year: job.Year, FatalErr: err}
	}
	return fetchResult{Ticker: job.Ticker, Year: job.Year, Group: group, Fetched: true}
}

func runOneYear(ctx context.Context, cfg config.Config, client *http.Client, job runJob, retries int) runResult {
	data, group, err := fetchPeriodWithRetry(ctx, cfg, client, job.Ticker, job.Year, retries)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return runResult{Ticker: job.Ticker, Year: job.Year, FatalErr: ctxErr}
		}
		return runResult{Ticker: job.Ticker, Year: job.Year, SkipErr: err}
	}
	if len(data.Value) == 0 {
		return runResult{Ticker: job.Ticker, Year: job.Year}
	}
	return runResult{
		Ticker:         job.Ticker,
		Year:           job.Year,
		Source:         "isyatirim",
		FinancialGroup: group,
		FetchedAt:      time.Now().UTC(),
		Lines:          data.Value,
		Fetched:        true,
	}
}

func fetchPeriodWithRetry(ctx context.Context, cfg config.Config, client *http.Client, ticker string, year int, retries int) (IsYatirimResponse, string, error) {
	if retries < 0 {
		retries = 0
	}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		data, group, err := fetchPeriod(ctx, cfg, client, ticker, year)
		if err == nil {
			return data, group, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return IsYatirimResponse{}, "", ctxErr
		}
		lastErr = err
		if attempt == retries {
			break
		}
		fmt.Printf("financials: %s %d retry %d/%d: %v\n", ticker, year, attempt+1, retries, err)
		if err := sleepBeforeRetry(ctx, attempt); err != nil {
			return IsYatirimResponse{}, "", err
		}
	}
	return IsYatirimResponse{}, "", lastErr
}

func sleepBeforeRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt+1) * time.Second
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}
	return sleepDuration(ctx, delay)
}

func sleepDuration(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func writeBilancoEquity(store *storage.EquityStore, state *runEquityState) error {
	domain.NormalizeBilancoInfo(state.Info, state.Equity.Ticker)
	if _, err := kap.ApplyFinancialPublishDatesFromStore(store, state.Equity.Ticker, state.Info); err != nil {
		return err
	}
	domain.MarkFinancialPeriodsAvailableAt(state.Info, time.Now().UTC(), "local_json_fetch_at")
	calculations, audit := CalculateEquityWithAudit(state.Info)
	versionStore, err := upsertStatementVersionStore(store, state.Equity.Ticker, state.Info)
	if err != nil {
		return err
	}
	if err := util.WriteJSON(store.FinancialInfoPath(state.Equity.Ticker), state.Info); err != nil {
		return err
	}
	if len(calculations) > 0 {
		if err := util.WriteJSON(store.FinancialCalculationsPath(state.Equity.Ticker), calculations); err != nil {
			return err
		}
	}
	if len(versionStore.Versions) > 0 {
		if err := util.WriteJSON(store.FinancialStatementVersionsPath(state.Equity.Ticker), versionStore); err != nil {
			return err
		}
	}
	return store.Update(state.Equity.Ticker, func(e *domain.Equity) error {
		e.BilancoInfo = state.Info
		e.BilancoCalculations = calculations
		e.BilancoCalculationAudit = audit
		return nil
	})
}

func upsertStatementVersionStore(store *storage.EquityStore, ticker string, info *domain.BilancoInfo) (domain.FinancialStatementVersionStore, error) {
	var existing domain.FinancialStatementVersionStore
	if err := util.ReadJSON(store.FinancialStatementVersionsPath(ticker), &existing); err != nil && !os.IsNotExist(err) {
		return existing, err
	}
	now := time.Now().UTC()
	incoming := domain.BuildStatementVersions(info, now)
	return domain.UpsertStatementVersions(existing, ticker, incoming, now), nil
}

func existingBilancoInfo(store *storage.EquityStore, equity *domain.Equity, force bool) (*domain.BilancoInfo, error) {
	info := &domain.BilancoInfo{
		Ticker: equity.Ticker,
		Data:   map[string]domain.BilancoField{},
	}
	if force {
		return info, nil
	}
	if equity.BilancoInfo != nil && len(equity.BilancoInfo.Data) > 0 {
		return normalizeBilancoInfo(equity.Ticker, equity.BilancoInfo), nil
	}
	var fromFile domain.BilancoInfo
	if err := util.ReadJSON(store.FinancialInfoPath(equity.Ticker), &fromFile); err == nil {
		return normalizeBilancoInfo(equity.Ticker, &fromFile), nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return info, nil
}

func normalizeBilancoInfo(ticker string, info *domain.BilancoInfo) *domain.BilancoInfo {
	if info == nil {
		return &domain.BilancoInfo{Ticker: ticker, Data: map[string]domain.BilancoField{}}
	}
	domain.NormalizeBilancoInfo(info, ticker)
	return info
}

func bilancoHasYear(info *domain.BilancoInfo, year int) bool {
	if info == nil {
		return false
	}
	yearKey := fmt.Sprint(year)
	for _, field := range info.Data {
		if len(field.Years[yearKey]) > 0 {
			return true
		}
	}
	return false
}

func shouldFetchRunYear(info *domain.BilancoInfo, year int, opts FetchOptions) bool {
	if year >= time.Now().Year() {
		return true
	}
	if !bilancoHasYear(info, year) {
		return true
	}
	if opts.ForceHistory {
		return true
	}
	return false
}

func mergePeriodIntoInfo(info *domain.BilancoInfo, year int, lines []IsYatirimLine, source, group string, fetchedAt time.Time) {
	yearKey := fmt.Sprint(year)
	if source == "" {
		source = "unknown"
	}
	if info.Source == "" {
		info.Source = source
	}
	if info.FinancialGroup == "" {
		info.FinancialGroup = group
	}
	if info.Currency == "" {
		info.Currency = "TRY"
	}
	if !fetchedAt.IsZero() {
		info.FetchedAt = fetchedAt
	}
	for _, line := range lines {
		if line.ItemCode == "" {
			continue
		}
		field := info.Data[line.ItemCode]
		if field.Years == nil {
			field.Years = map[string][]*float64{}
		}
		if field.DescTR == "" {
			field.DescTR = line.ItemDescTR
		}
		if field.DescEN == "" {
			field.DescEN = line.ItemDescEng
		}
		field.Years[yearKey] = []*float64{
			parseNullableFloat(line.Value1),
			parseNullableFloat(line.Value2),
			parseNullableFloat(line.Value3),
			parseNullableFloat(line.Value4),
		}
		info.Data[line.ItemCode] = field
	}
	domain.ApplyBilancoSourceMetadata(info, source, group, "TRY", fetchedAt)
	domain.AppendLineage(info, domain.DataLineageEvent{
		Stage:     "raw_financial_fetch",
		Source:    source,
		Transform: "merge_12_9_6_3_period_values",
		Version:   domain.FinancialMetadataVersion,
		CreatedAt: time.Now().UTC(),
		Notes: []string{
			fmt.Sprintf("ticker=%s", info.Ticker),
			fmt.Sprintf("year=%d", year),
			fmt.Sprintf("financial_group=%s", group),
			"publish_date_source=unavailable",
		},
	})
}

func fetchPeriod(ctx context.Context, cfg config.Config, client *http.Client, ticker string, year int) (IsYatirimResponse, string, error) {
	var lastErr error
	for _, group := range FinancialGroups {
		data, err := fetchPeriodGroup(ctx, cfg, client, ticker, year, group)
		if err != nil {
			lastErr = err
			continue
		}
		if len(data.Value) > 0 {
			return data, group, nil
		}
	}
	if lastErr != nil {
		return IsYatirimResponse{}, "", lastErr
	}
	return IsYatirimResponse{}, "", nil
}

func fetchPeriodGroup(ctx context.Context, cfg config.Config, client *http.Client, ticker string, year int, group string) (IsYatirimResponse, error) {
	url := fmt.Sprintf("https://www.isyatirim.com.tr/_layouts/15/IsYatirim.Website/Common/Data.aspx/MaliTablo?companyCode=%s&exchange=TRY&financialGroup=%s&year1=%d&period1=12&year2=%d&period2=9&year3=%d&period3=6&year4=%d&period4=3&_=1696687680320", ticker, group, year, year, year, year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return IsYatirimResponse{}, err
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", "https://www.isyatirim.com.tr/tr-tr/analiz/hisse/Sayfalar/sirket-karti.aspx?hisse="+ticker)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", "Mozilla/5.0 hissebot-go/1.0")
	if cfg.IsYatirimCookie != "" {
		req.Header.Set("Cookie", cfg.IsYatirimCookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return IsYatirimResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return IsYatirimResponse{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	var data IsYatirimResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return IsYatirimResponse{}, err
	}
	return data, nil
}

func fetchTargets(store *storage.EquityStore, opts FetchOptions) ([]*domain.Equity, error) {
	if opts.Ticker != "" {
		equity, err := store.Load(opts.Ticker)
		if err != nil {
			return nil, err
		}
		if equity.Ticker == "" {
			equity.Ticker = storage.NormalizeTicker(opts.Ticker)
		}
		if equity.AssetType == 0 {
			equity.AssetType = 2
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
		targets = append(targets, equity)
		if opts.Limit > 0 && len(targets) >= opts.Limit {
			break
		}
	}
	return targets, nil
}

func Merge(ctx context.Context, cfg config.Config, store *storage.EquityStore) error {
	equities, err := store.List()
	if err != nil {
		return err
	}
	merged := 0

	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info := &domain.BilancoInfo{
			Ticker: equity.Ticker,
			Data:   map[string]domain.BilancoField{},
		}
		for _, year := range Years {
			var response IsYatirimResponse
			if err := util.ReadJSON(periodPath(cfg, store, equity.Ticker, year), &response); err != nil {
				if err := util.ReadJSON(legacyPeriodPath(cfg, equity.Ticker, year), &response); err != nil {
					continue
				}
			}
			source := response.Source
			if source == "" {
				source = "isyatirim_cache"
			}
			mergePeriodIntoInfo(info, year, response.Value, source, response.FinancialGroup, response.FetchedAt)
		}
		if len(info.Data) == 0 {
			continue
		}
		domain.NormalizeBilancoInfo(info, equity.Ticker)
		if _, err := kap.ApplyFinancialPublishDatesFromStore(store, equity.Ticker, info); err != nil {
			return err
		}
		domain.MarkFinancialPeriodsAvailableAt(info, time.Now().UTC(), "local_json_merge_at")
		if err := util.WriteJSON(store.FinancialInfoPath(equity.Ticker), info); err != nil {
			return err
		}
		versionStore, err := upsertStatementVersionStore(store, equity.Ticker, info)
		if err != nil {
			return err
		}
		if len(versionStore.Versions) > 0 {
			if err := util.WriteJSON(store.FinancialStatementVersionsPath(equity.Ticker), versionStore); err != nil {
				return err
			}
		}
		if err := store.Update(equity.Ticker, func(e *domain.Equity) error {
			e.BilancoInfo = info
			return nil
		}); err != nil {
			return err
		}
		merged++
	}

	fmt.Printf("financials: %d equities merged from cache\n", merged)
	return nil
}

func periodPath(_ config.Config, store *storage.EquityStore, ticker string, year int) string {
	return store.FinancialPeriodPath(ticker, year)
}

func legacyPeriodPath(cfg config.Config, ticker string, year int) string {
	return filepath.Join(cfg.BilancoCacheDir, storage.NormalizeTicker(ticker), fmt.Sprintf("%d-12-9-6-3.json", year))
}
