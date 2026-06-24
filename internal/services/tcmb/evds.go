package tcmb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const DefaultEVDSBaseURL = "https://evds3.tcmb.gov.tr/igmevdsms-dis"

type EVDSOptions struct {
	OutputDir    string
	BaseURL      string
	Timeout      time.Duration
	CatalogOnly  bool
	Values       bool
	DataGroups   []string
	Series       []string
	From         time.Time
	To           time.Time
	Limit        int
	Workers      int
	Delay        time.Duration
	ChunkDays    int
	Force        bool
	SkipExisting bool
}

type EVDSSyncResult struct {
	OutputDir     string   `json:"output_dir"`
	CatalogPath   string   `json:"catalog_path"`
	SeriesDir     string   `json:"series_dir"`
	FailuresPath  string   `json:"failures_path,omitempty"`
	SourceURL     string   `json:"source_url"`
	Categories    int      `json:"categories"`
	DataGroups    int      `json:"data_groups"`
	Series        int      `json:"series"`
	ValuesFetched int      `json:"values_fetched"`
	ValuesSkipped int      `json:"values_skipped"`
	Failures      int      `json:"failures"`
	Status        string   `json:"status"`
	Warnings      []string `json:"warnings,omitempty"`
}

type EVDSRepairResult struct {
	OutputDir          string `json:"output_dir"`
	FilesScanned       int    `json:"files_scanned"`
	FilesUpdated       int    `json:"files_updated"`
	PointsScanned      int    `json:"points_scanned"`
	ValuesRepaired     int    `json:"values_repaired"`
	DatesRepaired      int    `json:"dates_repaired"`
	ValueParseFailures int    `json:"value_parse_failures"`
}

type EVDSCatalog struct {
	Source     string           `json:"source"`
	SourceURL  string           `json:"source_url"`
	BaseURL    string           `json:"base_url"`
	FetchedAt  string           `json:"fetched_at"`
	Stats      EVDSCatalogStats `json:"stats"`
	Categories []EVDSCategory   `json:"categories"`
	Series     []EVDSSeriesMeta `json:"series"`
}

type EVDSCatalogStats struct {
	Categories int `json:"categories"`
	DataGroups int `json:"data_groups"`
	Series     int `json:"series"`
}

type EVDSCategory struct {
	CategoryID    int             `json:"CATEGORY_ID"`
	TopicTitleENG string          `json:"TOPIC_TITLE_ENG"`
	TopicTitleTR  string          `json:"TOPIC_TITLE_TR"`
	Level         int             `json:"SEVIYE"`
	ParentID      int             `json:"UST_CATEGORY_ID"`
	DataGroups    []EVDSDataGroup `json:"DATAGROUPS"`
}

type EVDSDataGroup struct {
	ScreenOrder       int      `json:"screenOrder"`
	DataGroupCode     string   `json:"DATAGROUP_CODE"`
	CategoryID        int      `json:"CATEGORY_ID"`
	DataGroupType     string   `json:"DATAGROUP_TYPE"`
	DataGroupTypeENG  string   `json:"DATAGROUP_TYPE_ENG"`
	Status            int      `json:"STATUS"`
	Frequency         float64  `json:"FREQUENCY"`
	FrequencyStr      string   `json:"FREQUENCY_STR"`
	DataSource        string   `json:"DATASOURCE"`
	DataSourceENG     string   `json:"DATASOURCE_ENG"`
	MetadataLink      string   `json:"METADATA_LINK"`
	MetadataLinkENG   string   `json:"METADATA_LINK_ENG"`
	RevisionPolicy    string   `json:"REV_POL_LINK"`
	RevisionPolicyENG string   `json:"REV_POL_LINK_ENG"`
	ApplicationChange string   `json:"APP_CHA_LINK"`
	AppChangeENG      string   `json:"APP_CHA_LINK_ENG"`
	Yearable          int      `json:"YEARABLE"`
	SixMonthable      int      `json:"6MONTHABLE"`
	Quarterable       int      `json:"QUARTERABLE"`
	Monthable         int      `json:"MONTHABLE"`
	TwoWeekable       int      `json:"2WEEKABLE"`
	Weekable          int      `json:"WEEKABLE"`
	Workdayable       int      `json:"WORKDAYABLE"`
	Dayable           int      `json:"DAYABLE"`
	Unit              string   `json:"BIRIMI"`
	UnitENG           string   `json:"BIRIMI_EN"`
	LastUpdated       string   `json:"LAST_UPDATED"`
	Series            []string `json:"series_codes,omitempty"`
}

type EVDSSeriesMeta struct {
	SeriesCode          string `json:"SERIE_CODE"`
	DataGroupCode       string `json:"DATAGROUP_CODE"`
	SeriesName          string `json:"SERIE_NAME"`
	SeriesNameENG       string `json:"SERIE_NAME_ENG"`
	FrequencyStr        string `json:"FREQUENCY_STR"`
	DefaultAggMethodStr string `json:"DEFAULT_AGG_METHOD_STR"`
	DefaultAggMethod    string `json:"DEFAULT_AGG_METHOD"`
	ScreenOrder         int    `json:"SCREEN_ORDER"`
	Avgable             int    `json:"AVGABLE"`
	Firstable           int    `json:"FIRSTABLE"`
	Lastable            int    `json:"LASTABLE"`
	Maxable             int    `json:"MAXABLE"`
	Minable             int    `json:"MINABLE"`
	Sumable             int    `json:"SUMABLE"`
	Level               int    `json:"SEVIYE"`
	ParentSeriesCode    string `json:"UST_SERIE_CODE"`
}

type EVDSSeriesDataset struct {
	Source              string            `json:"source"`
	SourceURL           string            `json:"source_url"`
	BaseURL             string            `json:"base_url"`
	FetchedAt           string            `json:"fetched_at"`
	DataGroup           EVDSDataGroup     `json:"data_group"`
	Series              EVDSSeriesMeta    `json:"series"`
	ReportFrequency     string            `json:"report_frequency"`
	AggregationType     string            `json:"aggregation_type"`
	Formula             string            `json:"formula"`
	StartDate           string            `json:"start_date"`
	EndDate             string            `json:"end_date"`
	Bounds              EVDSBounds        `json:"bounds"`
	Chunks              []EVDSChunkResult `json:"chunks"`
	SeriesNames         map[string]string `json:"series_names,omitempty"`
	FrequencyConversion map[string]bool   `json:"frequency_conversion,omitempty"`
	Points              []EVDSPoint       `json:"points"`
}

type EVDSPoint struct {
	Date        string   `json:"date,omitempty"`
	DisplayDate string   `json:"display_date"`
	Value       *float64 `json:"value"`
	RawValue    *string  `json:"raw_value,omitempty"`
	UnixTime    *int64   `json:"unix_time,omitempty"`
}

type EVDSBounds struct {
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	MaxStartDate string `json:"maxStartDate"`
	MinEndDate   string `json:"minEndDate"`
	Frequency    string `json:"frequency"`
}

type EVDSChunkResult struct {
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	TotalCount int    `json:"total_count"`
}

type evdsValuesResponse struct {
	TotalCount          int                          `json:"totalCount"`
	Items               []map[string]json.RawMessage `json:"items"`
	SeriesNames         map[string]string            `json:"seriesNames"`
	FrequencyConversion map[string]bool              `json:"frequencyConversion"`
}

type evdsFailure struct {
	SeriesCode    string `json:"series_code"`
	DataGroupCode string `json:"data_group_code"`
	Error         string `json:"error"`
	At            string `json:"at"`
}

func SyncEVDS(ctx context.Context, opts EVDSOptions) (EVDSSyncResult, error) {
	opts = normalizeOptions(opts)
	client := &http.Client{Timeout: opts.Timeout}
	catalog, dataGroupsByCode, err := fetchCatalog(ctx, client, opts)
	if err != nil {
		return EVDSSyncResult{}, err
	}
	if err := saveJSON(filepath.Join(opts.OutputDir, "catalog.json"), catalog); err != nil {
		return EVDSSyncResult{}, err
	}

	result := EVDSSyncResult{
		OutputDir:   opts.OutputDir,
		CatalogPath: filepath.Join(opts.OutputDir, "catalog.json"),
		SeriesDir:   filepath.Join(opts.OutputDir, "series"),
		SourceURL:   strings.TrimRight(opts.BaseURL, "/") + "/categories/withDatagroups/type=json",
		Categories:  catalog.Stats.Categories,
		DataGroups:  catalog.Stats.DataGroups,
		Series:      catalog.Stats.Series,
		Status:      "ok",
	}
	if opts.CatalogOnly || !opts.Values {
		return result, nil
	}

	selected := selectSeries(catalog.Series, opts.Series, opts.Limit)
	failures := syncEVDSValues(ctx, client, opts, selected, dataGroupsByCode, &result)
	if len(failures) > 0 {
		result.Status = "partial"
		result.Failures = len(failures)
		result.FailuresPath = filepath.Join(opts.OutputDir, "failures.jsonl")
		if err := writeFailures(result.FailuresPath, failures); err != nil {
			return result, err
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d EVDS serisi indirilemedi; failures.jsonl tekrar deneme için saklandı", len(failures)))
	}
	return result, nil
}

func normalizeOptions(opts EVDSOptions) EVDSOptions {
	if strings.TrimSpace(opts.OutputDir) == "" {
		opts.OutputDir = filepath.Join("data", "macro", "tcmb_evds")
	}
	if strings.TrimSpace(opts.BaseURL) == "" {
		opts.BaseURL = DefaultEVDSBaseURL
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	if opts.CatalogOnly {
		opts.Values = false
	}
	if !opts.Values && !opts.CatalogOnly {
		opts.Values = true
	}
	if opts.SkipExisting && opts.Force {
		opts.SkipExisting = false
	}
	return opts
}

func fetchCatalog(ctx context.Context, client *http.Client, opts EVDSOptions) (EVDSCatalog, map[string]EVDSDataGroup, error) {
	var categories []EVDSCategory
	if err := doJSON(ctx, client, http.MethodGet, opts.BaseURL+"/categories/withDatagroups/type=json", nil, &categories); err != nil {
		return EVDSCatalog{}, nil, fmt.Errorf("fetch TCMB EVDS categories: %w", err)
	}
	dataGroupFilter := stringSet(opts.DataGroups)
	dataGroups := flattenDataGroups(categories, dataGroupFilter)
	if len(dataGroupFilter) > 0 {
		categories = filterCategoriesByDataGroups(categories, dataGroupFilter)
	}
	dataGroupsByCode := make(map[string]EVDSDataGroup, len(dataGroups))
	for _, dg := range dataGroups {
		dataGroupsByCode[dg.DataGroupCode] = dg
	}

	allSeries := make([]EVDSSeriesMeta, 0, len(dataGroups)*8)
	for i := range dataGroups {
		dg := dataGroups[i]
		var series []EVDSSeriesMeta
		endpoint := opts.BaseURL + "/serieList/fe/type=json&code=" + url.QueryEscape(dg.DataGroupCode)
		if err := doJSON(ctx, client, http.MethodGet, endpoint, nil, &series); err != nil {
			return EVDSCatalog{}, nil, fmt.Errorf("fetch TCMB EVDS series for %s: %w", dg.DataGroupCode, err)
		}
		codes := make([]string, 0, len(series))
		for _, item := range series {
			if strings.TrimSpace(item.SeriesCode) == "" {
				continue
			}
			if item.DataGroupCode == "" {
				item.DataGroupCode = dg.DataGroupCode
			}
			codes = append(codes, item.SeriesCode)
			allSeries = append(allSeries, item)
		}
		sort.Strings(codes)
		dataGroups[i].Series = codes
		dataGroupsByCode[dg.DataGroupCode] = dataGroups[i]
	}
	applyDataGroupSeries(categories, dataGroupsByCode)
	sort.Slice(allSeries, func(i, j int) bool {
		if allSeries[i].DataGroupCode == allSeries[j].DataGroupCode {
			return allSeries[i].ScreenOrder < allSeries[j].ScreenOrder
		}
		return allSeries[i].DataGroupCode < allSeries[j].DataGroupCode
	})

	catalog := EVDSCatalog{
		Source:    "TCMB EVDS3",
		SourceURL: opts.BaseURL + "/categories/withDatagroups/type=json",
		BaseURL:   opts.BaseURL,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Stats: EVDSCatalogStats{
			Categories: len(categories),
			DataGroups: len(dataGroups),
			Series:     len(allSeries),
		},
		Categories: categories,
		Series:     allSeries,
	}
	return catalog, dataGroupsByCode, nil
}

func flattenDataGroups(categories []EVDSCategory, filter map[string]bool) []EVDSDataGroup {
	out := []EVDSDataGroup{}
	seen := map[string]bool{}
	for _, category := range categories {
		for _, dg := range category.DataGroups {
			code := strings.TrimSpace(dg.DataGroupCode)
			if code == "" || seen[code] {
				continue
			}
			if len(filter) > 0 && !filter[strings.ToLower(code)] {
				continue
			}
			seen[code] = true
			out = append(out, dg)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CategoryID == out[j].CategoryID {
			return out[i].ScreenOrder < out[j].ScreenOrder
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out
}

func filterCategoriesByDataGroups(categories []EVDSCategory, filter map[string]bool) []EVDSCategory {
	out := make([]EVDSCategory, 0, len(categories))
	for _, category := range categories {
		dataGroups := make([]EVDSDataGroup, 0, len(category.DataGroups))
		for _, dg := range category.DataGroups {
			if filter[strings.ToLower(strings.TrimSpace(dg.DataGroupCode))] {
				dataGroups = append(dataGroups, dg)
			}
		}
		if len(dataGroups) == 0 {
			continue
		}
		category.DataGroups = dataGroups
		out = append(out, category)
	}
	return out
}

func applyDataGroupSeries(categories []EVDSCategory, byCode map[string]EVDSDataGroup) {
	for i := range categories {
		for j := range categories[i].DataGroups {
			code := categories[i].DataGroups[j].DataGroupCode
			if updated, ok := byCode[code]; ok {
				categories[i].DataGroups[j] = updated
			}
		}
	}
}

func selectSeries(series []EVDSSeriesMeta, filters []string, limit int) []EVDSSeriesMeta {
	filter := stringSet(filters)
	out := make([]EVDSSeriesMeta, 0, len(series))
	for _, item := range series {
		if len(filter) > 0 && !filter[strings.ToLower(item.SeriesCode)] {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func syncEVDSValues(ctx context.Context, client *http.Client, opts EVDSOptions, series []EVDSSeriesMeta, dataGroups map[string]EVDSDataGroup, result *EVDSSyncResult) []evdsFailure {
	type job struct {
		series EVDSSeriesMeta
	}
	jobs := make(chan job)
	var mu sync.Mutex
	failures := []evdsFailure{}
	workerCount := opts.Workers
	if workerCount > len(series) && len(series) > 0 {
		workerCount = len(series)
	}
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				dg := dataGroups[item.series.DataGroupCode]
				path := seriesOutputPath(opts.OutputDir, item.series)
				if opts.SkipExisting && fileHasContent(path) {
					mu.Lock()
					result.ValuesSkipped++
					mu.Unlock()
					continue
				}
				if err := fetchAndSaveSeries(ctx, client, opts, dg, item.series, path); err != nil {
					mu.Lock()
					failures = append(failures, evdsFailure{
						SeriesCode:    item.series.SeriesCode,
						DataGroupCode: item.series.DataGroupCode,
						Error:         err.Error(),
						At:            time.Now().UTC().Format(time.RFC3339),
					})
					mu.Unlock()
				} else {
					mu.Lock()
					result.ValuesFetched++
					mu.Unlock()
				}
				if opts.Delay > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(opts.Delay):
					}
				}
			}
		}()
	}
	for _, item := range series {
		select {
		case <-ctx.Done():
			break
		case jobs <- job{series: item}:
		}
	}
	close(jobs)
	wg.Wait()
	return failures
}

func fetchAndSaveSeries(ctx context.Context, client *http.Client, opts EVDSOptions, dg EVDSDataGroup, meta EVDSSeriesMeta, path string) error {
	reportFrequency := ReportFrequencyFromDataGroup(dg.Frequency)
	if reportFrequency == "" {
		reportFrequency = "1"
	}
	bounds, err := fetchBounds(ctx, client, opts.BaseURL, reportFrequency, meta.SeriesCode, meta.DataGroupCode)
	if err != nil {
		return err
	}
	start, end, err := chooseDateRange(bounds, opts.From, opts.To)
	if err != nil {
		return err
	}
	chunks := buildDateRanges(start, end, opts.ChunkDays)
	aggregation := strings.TrimSpace(meta.DefaultAggMethod)
	if aggregation == "" {
		aggregation = "avg"
	}
	dataset := EVDSSeriesDataset{
		Source:          "TCMB EVDS3",
		SourceURL:       opts.BaseURL + "/fe",
		BaseURL:         opts.BaseURL,
		FetchedAt:       time.Now().UTC().Format(time.RFC3339),
		DataGroup:       dg,
		Series:          meta,
		ReportFrequency: reportFrequency,
		AggregationType: aggregation,
		Formula:         "0",
		StartDate:       start.Format("2006-01-02"),
		EndDate:         end.Format("2006-01-02"),
		Bounds:          bounds,
		SeriesNames:     map[string]string{},
	}
	seen := map[string]bool{}
	for _, chunk := range chunks {
		response, err := fetchValues(ctx, client, opts.BaseURL, evdsValueRequest{
			Type:             "json",
			Series:           meta.SeriesCode,
			AggregationTypes: aggregation,
			Formulas:         "0",
			StartDate:        formatEVDSDate(chunk.start),
			EndDate:          formatEVDSDate(chunk.end),
			Frequency:        reportFrequency,
			DecimalSeparator: ".",
			Decimal:          "10",
			DateFormat:       "0",
			Lang:             "tr",
			Direction:        "0",
			Order:            "0",
			CustomFormulas:   []any{},
			GroupSeparator:   true,
			IsDashboard:      false,
		})
		if err != nil {
			return err
		}
		dataset.Chunks = append(dataset.Chunks, EVDSChunkResult{
			StartDate:  chunk.start.Format("2006-01-02"),
			EndDate:    chunk.end.Format("2006-01-02"),
			TotalCount: response.TotalCount,
		})
		for key, value := range response.SeriesNames {
			dataset.SeriesNames[key] = value
		}
		if len(response.FrequencyConversion) > 0 {
			if dataset.FrequencyConversion == nil {
				dataset.FrequencyConversion = map[string]bool{}
			}
			for key, value := range response.FrequencyConversion {
				dataset.FrequencyConversion[key] = value
			}
		}
		points := pointsFromEVDSItems(meta.SeriesCode, response.Items)
		for _, point := range points {
			key := firstNonEmpty(point.Date, point.DisplayDate)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			dataset.Points = append(dataset.Points, point)
		}
	}
	sort.SliceStable(dataset.Points, func(i, j int) bool {
		if dataset.Points[i].Date != "" && dataset.Points[j].Date != "" {
			return dataset.Points[i].Date < dataset.Points[j].Date
		}
		if dataset.Points[i].UnixTime != nil && dataset.Points[j].UnixTime != nil {
			return *dataset.Points[i].UnixTime < *dataset.Points[j].UnixTime
		}
		return dataset.Points[i].DisplayDate < dataset.Points[j].DisplayDate
	})
	return saveJSON(path, dataset)
}

type evdsValueRequest struct {
	Type             string `json:"type"`
	Series           string `json:"series"`
	AggregationTypes string `json:"aggregationTypes"`
	Formulas         string `json:"formulas"`
	StartDate        string `json:"startDate"`
	EndDate          string `json:"endDate"`
	Frequency        string `json:"frequency"`
	DecimalSeparator string `json:"decimalSeperator"`
	Decimal          string `json:"decimal"`
	DateFormat       string `json:"dateFormat"`
	Lang             string `json:"lang"`
	Direction        string `json:"yon"`
	Order            string `json:"sira"`
	CustomFormulas   []any  `json:"ozelFormuller"`
	GroupSeparator   bool   `json:"groupSeperator"`
	IsDashboard      bool   `json:"isRaporSayfasi"`
}

func fetchBounds(ctx context.Context, client *http.Client, baseURL, frequency, seriesCode, dataGroupCode string) (EVDSBounds, error) {
	body := map[string]any{
		"frequency":  frequency,
		"series":     []string{seriesCode},
		"datagroups": []string{dataGroupCode},
	}
	var bounds EVDSBounds
	if err := doJSON(ctx, client, http.MethodPost, baseURL+"/serieList/baslangicBitis", body, &bounds); err != nil {
		return EVDSBounds{}, fmt.Errorf("fetch TCMB EVDS bounds for %s: %w", seriesCode, err)
	}
	return bounds, nil
}

func fetchValues(ctx context.Context, client *http.Client, baseURL string, request evdsValueRequest) (evdsValuesResponse, error) {
	var response evdsValuesResponse
	if err := doJSON(ctx, client, http.MethodPost, baseURL+"/fe", request, &response); err != nil {
		return evdsValuesResponse{}, fmt.Errorf("fetch TCMB EVDS values for %s: %w", request.Series, err)
	}
	return response, nil
}

func doJSON(ctx context.Context, client *http.Client, method, requestURL string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("User-Agent", "hissebot/1.0 (+tcmb-evds-sync)")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte("<!DOCTYPE html")) || bytes.HasPrefix(trimmed, []byte("<html")) {
		return fmt.Errorf("unexpected HTML response from %s", requestURL)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("parse JSON from %s: %w", requestURL, err)
	}
	return nil
}

func chooseDateRange(bounds EVDSBounds, from, to time.Time) (time.Time, time.Time, error) {
	start, err := parseEVDSAPIDate(firstNonEmpty(bounds.MaxStartDate, bounds.StartDate))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse EVDS start date: %w", err)
	}
	end, err := parseEVDSAPIDate(firstNonEmpty(bounds.MinEndDate, bounds.EndDate))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse EVDS end date: %w", err)
	}
	if !from.IsZero() && from.After(start) {
		start = dateOnly(from)
	}
	if !to.IsZero() && to.Before(end) {
		end = dateOnly(to)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("EVDS date range empty: %s - %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	return start, end, nil
}

type dateRange struct {
	start time.Time
	end   time.Time
}

func buildDateRanges(start, end time.Time, chunkDays int) []dateRange {
	if chunkDays <= 0 {
		return []dateRange{{start: start, end: end}}
	}
	out := []dateRange{}
	current := start
	for !current.After(end) {
		chunkEnd := current.AddDate(0, 0, chunkDays-1)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		out = append(out, dateRange{start: current, end: chunkEnd})
		current = chunkEnd.AddDate(0, 0, 1)
	}
	return out
}

func parseEVDSAPIDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"02-01-2006", "2006-01-02", "02.01.2006"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return dateOnly(parsed), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid EVDS date %q", value)
}

func formatEVDSDate(value time.Time) string {
	return dateOnly(value).Format("02-01-2006")
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func pointsFromEVDSItems(seriesCode string, items []map[string]json.RawMessage) []EVDSPoint {
	out := make([]EVDSPoint, 0, len(items))
	responseKey := EVDSResponseKey(seriesCode)
	for _, item := range items {
		displayDate := rawString(item["Tarih"])
		if strings.TrimSpace(displayDate) == "" {
			continue
		}
		rawValue, ok := item[responseKey]
		if !ok {
			for key, value := range item {
				if key != "Tarih" && key != "UNIXTIME" {
					rawValue = value
					ok = true
					break
				}
			}
		}
		value, rawStringValue := parseEVDSValue(rawValue, ok)
		unixTime := parseEVDSUnixTime(item["UNIXTIME"])
		out = append(out, EVDSPoint{
			Date:        normalizeEVDSDisplayDate(displayDate),
			DisplayDate: displayDate,
			Value:       value,
			RawValue:    rawStringValue,
			UnixTime:    unixTime,
		})
	}
	return out
}

func EVDSResponseKey(seriesCode string) string {
	return strings.ReplaceAll(seriesCode, ".", "_")
}

func parseEVDSValue(raw json.RawMessage, ok bool) (*float64, *string) {
	if !ok || len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return nil, nil
		}
		parsed, err := ParseEVDSNumber(asString)
		if err != nil {
			return nil, &asString
		}
		return &parsed, &asString
	}
	var asFloat float64
	if err := json.Unmarshal(raw, &asFloat); err == nil {
		asString = strconv.FormatFloat(asFloat, 'f', -1, 64)
		return &asFloat, &asString
	}
	return nil, nil
}

func ParseEVDSNumber(value string) (float64, error) {
	normalized := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r), r == '\u00a0':
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(value))
	if normalized == "" {
		return 0, errors.New("empty EVDS number")
	}

	lastComma := strings.LastIndex(normalized, ",")
	lastDot := strings.LastIndex(normalized, ".")
	switch {
	case lastComma >= 0 && lastDot >= 0 && lastDot > lastComma:
		normalized = strings.ReplaceAll(normalized, ",", "")
	case lastComma >= 0 && lastDot >= 0 && lastComma > lastDot:
		normalized = strings.ReplaceAll(normalized, ".", "")
		normalized = strings.ReplaceAll(normalized, ",", ".")
	case lastComma >= 0:
		normalized = normalizeSingleSeparatorNumber(normalized, ',')
	case lastDot >= 0:
		normalized = normalizeSingleSeparatorNumber(normalized, '.')
	}
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, fmt.Errorf("parse EVDS number %q: %w", value, err)
	}
	return parsed, nil
}

func normalizeSingleSeparatorNumber(value string, separator rune) string {
	separatorText := string(separator)
	if strings.Count(value, separatorText) == 1 {
		if separator == ',' {
			return strings.ReplaceAll(value, separatorText, ".")
		}
		return value
	}
	parts := strings.Split(value, separatorText)
	thousands := len(parts) > 2
	for _, part := range parts[1:] {
		if len(part) != 3 {
			thousands = false
			break
		}
	}
	if thousands {
		return strings.Join(parts, "")
	}
	decimal := parts[len(parts)-1]
	integer := strings.Join(parts[:len(parts)-1], "")
	return integer + "." + decimal
}

func parseEVDSUnixTime(raw json.RawMessage) *int64 {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var object map[string]string
	if err := json.Unmarshal(raw, &object); err == nil {
		if value := strings.TrimSpace(object["$numberLong"]); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				return &parsed
			}
		}
	}
	var numeric int64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return &numeric
	}
	return nil
}

func normalizeEVDSDisplayDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{"02-01-2006", "2006-01-02", "01-2006", "2006-01", "2006", "02.01.2006"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			switch layout {
			case "01-2006", "2006-01":
				return parsed.Format("2006-01")
			case "2006":
				return parsed.Format("2006")
			default:
				return parsed.Format("2006-01-02")
			}
		}
	}
	if normalized := normalizeMonthNamePeriod(value); normalized != "" {
		return normalized
	}
	return ""
}

func RepairEVDSArchive(outputDir string) (EVDSRepairResult, error) {
	result := EVDSRepairResult{OutputDir: outputDir}
	seriesDir := filepath.Join(strings.TrimSpace(outputDir), "series")
	err := filepath.WalkDir(seriesDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		result.FilesScanned++
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read EVDS series %s: %w", path, err)
		}
		var dataset EVDSSeriesDataset
		if err := json.Unmarshal(raw, &dataset); err != nil {
			return fmt.Errorf("parse EVDS series %s: %w", path, err)
		}
		changed := false
		for i := range dataset.Points {
			point := &dataset.Points[i]
			result.PointsScanned++
			if point.RawValue != nil && strings.TrimSpace(*point.RawValue) != "" {
				parsed, err := ParseEVDSNumber(*point.RawValue)
				if err != nil {
					result.ValueParseFailures++
				} else if point.Value == nil || *point.Value != parsed {
					point.Value = &parsed
					result.ValuesRepaired++
					changed = true
				}
			}
			if strings.TrimSpace(point.Date) == "" {
				if normalized := normalizeEVDSDisplayDate(point.DisplayDate); normalized != "" {
					point.Date = normalized
					result.DatesRepaired++
					changed = true
				}
			}
		}
		if !changed {
			return nil
		}
		encoded, err := json.MarshalIndent(dataset, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal repaired EVDS series %s: %w", path, err)
		}
		if err := writeEVDSFileAtomically(path, append(encoded, '\n')); err != nil {
			return err
		}
		result.FilesUpdated++
		return nil
	})
	if os.IsNotExist(err) {
		return result, nil
	}
	return result, err
}

func writeEVDSFileAtomically(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".evds-repair-*.json")
	if err != nil {
		return fmt.Errorf("create EVDS repair temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write EVDS repair temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close EVDS repair temp file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace repaired EVDS series %s: %w", path, err)
	}
	return nil
}

func normalizeMonthNamePeriod(value string) string {
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(value)))
	if len(parts) != 2 {
		return ""
	}
	month := map[string]string{
		"OCAK": "01", "JANUARY": "01",
		"ŞUBAT": "02", "SUBAT": "02", "FEBRUARY": "02",
		"MART": "03", "MARCH": "03",
		"NİSAN": "04", "NISAN": "04", "APRIL": "04",
		"MAYIS": "05", "MAY": "05",
		"HAZİRAN": "06", "HAZIRAN": "06", "JUNE": "06",
		"TEMMUZ": "07", "JULY": "07",
		"AĞUSTOS": "08", "AGUSTOS": "08", "AUGUST": "08",
		"EYLÜL": "09", "EYLUL": "09", "SEPTEMBER": "09",
		"EKİM": "10", "EKIM": "10", "OCTOBER": "10",
		"KASIM": "11", "NOVEMBER": "11",
		"ARALIK": "12", "DECEMBER": "12",
	}
	if month[parts[0]] == "" || len(parts[1]) != 4 {
		return ""
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return ""
	}
	return parts[1] + "-" + month[parts[0]]
}

func rawString(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return strings.TrimSpace(value)
	}
	return ""
}

func ReportFrequencyFromDataGroup(frequency float64) string {
	switch int(frequency) {
	case 1:
		return "1"
	case 2:
		return "2"
	case 5:
		return "3"
	case 7:
		return "4"
	case 9:
		return "5"
	case 13:
		return "6"
	case 17:
		return "7"
	case 21:
		return "8"
	default:
		return ""
	}
}

func seriesOutputPath(outputDir string, series EVDSSeriesMeta) string {
	return filepath.Join(outputDir, "series", safePathPart(series.DataGroupCode), safePathPart(series.SeriesCode)+".json")
}

func safePathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '_' || r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func saveJSON(path string, value any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func writeFailures(path string, failures []evdsFailure) error {
	if len(failures) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, failure := range failures {
		if err := encoder.Encode(failure); err != nil {
			return err
		}
	}
	return nil
}

func fileHasContent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
