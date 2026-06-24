package tuik

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/ta/macro"
	"hissebot/pkg/mathutil"
)

const (
	CIPMapDataURL = "https://cip.tuik.gov.tr/Home/GetMapData"

	GDPPerCapitaUSDCode = "UHS-GK095-O0011"
	GDPPerCapitaTRYCode = "UHS-GK096-O0011"
	GDPThousandTRYCode  = "UHS-GK097-O0011"
)

type GDPOptions struct {
	OutputPath string
	Years      int
	Timeout    time.Duration
	BaseURL    string
}

type CIPResponse struct {
	IndicatorNo string   `json:"gostergeNo"`
	NameTR      string   `json:"gosterge_ad"`
	NameEN      string   `json:"gosterge_ad_ing"`
	Period      string   `json:"period"`
	Decimals    string   `json:"ondalikHassasiyet"`
	MetadataURL string   `json:"metaVeriURL"`
	Dates       []string `json:"tarihler"`
	Rows        []CIPRow `json:"veriler"`
}

type CIPRow struct {
	RegionCode string   `json:"duzeyKodu"`
	Values     []string `json:"veri"`
}

type GDPSyncResult struct {
	OutputPath  string           `json:"output_path"`
	Years       int              `json:"years"`
	LatestYear  int              `json:"latest_year"`
	SourceURL   string           `json:"source_url"`
	MetadataURL string           `json:"metadata_url,omitempty"`
	Context     macro.GDPContext `json:"context"`
}

func SyncGDP(ctx context.Context, opts GDPOptions) (GDPSyncResult, error) {
	dataset, err := FetchGDPDataset(ctx, opts)
	if err != nil {
		return GDPSyncResult{}, err
	}
	outputPath := strings.TrimSpace(opts.OutputPath)
	if outputPath == "" {
		outputPath = macro.DefaultTUIKGDPFile
	}
	if err := macro.SaveGDPDataset(outputPath, dataset); err != nil {
		return GDPSyncResult{}, err
	}
	gdpContext := macro.AnalyzeGDP(dataset)
	return GDPSyncResult{
		OutputPath:  outputPath,
		Years:       len(dataset.Points),
		LatestYear:  gdpContext.LatestYear,
		SourceURL:   dataset.SourceURL,
		MetadataURL: dataset.MetadataURL,
		Context:     gdpContext,
	}, nil
}

func FetchGDPDataset(ctx context.Context, opts GDPOptions) (macro.GDPDataset, error) {
	years := opts.Years
	if years <= 0 {
		years = 10
	}
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = CIPMapDataURL
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	perCapUSD, err := fetchCIPIndicator(ctx, client, baseURL, GDPPerCapitaUSDCode, years)
	if err != nil {
		return macro.GDPDataset{}, err
	}
	perCapTRY, err := fetchCIPIndicator(ctx, client, baseURL, GDPPerCapitaTRYCode, years)
	if err != nil {
		return macro.GDPDataset{}, err
	}
	gdpTRY, err := fetchCIPIndicator(ctx, client, baseURL, GDPThousandTRYCode, years)
	if err != nil {
		return macro.GDPDataset{}, err
	}
	points, err := aggregateGDP(perCapUSD, perCapTRY, gdpTRY)
	if err != nil {
		return macro.GDPDataset{}, err
	}
	metadataURL := firstNonEmpty(gdpTRY.MetadataURL, perCapTRY.MetadataURL, perCapUSD.MetadataURL)
	indicators := []macro.GDPIndicatorMeta{
		{Code: perCapUSD.IndicatorNo, NameTR: strings.TrimSpace(perCapUSD.NameTR), NameEN: strings.TrimSpace(perCapUSD.NameEN), Unit: "$"},
		{Code: perCapTRY.IndicatorNo, NameTR: strings.TrimSpace(perCapTRY.NameTR), NameEN: strings.TrimSpace(perCapTRY.NameEN), Unit: "TL"},
		{Code: gdpTRY.IndicatorNo, NameTR: strings.TrimSpace(gdpTRY.NameTR), NameEN: strings.TrimSpace(gdpTRY.NameEN), Unit: "bin TL"},
	}
	return macro.NewDatasetFetchedNow(points, indicators, baseURL, metadataURL), nil
}

func fetchCIPIndicator(ctx context.Context, client *http.Client, baseURL, indicator string, years int) (CIPResponse, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return CIPResponse{}, fmt.Errorf("parse TÜİK CİP URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("kaynak", "ilGostergeleri")
	query.Set("duzey", "3")
	query.Set("gostergeNo", indicator)
	query.Set("kayitSayisi", strconv.Itoa(years))
	query.Set("period", "yillik")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return CIPResponse{}, fmt.Errorf("build TÜİK CİP request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "hissebot/1.0 (+macro-gdp-report)")
	resp, err := client.Do(req)
	if err != nil {
		return CIPResponse{}, fmt.Errorf("fetch TÜİK CİP %s: %w", indicator, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return CIPResponse{}, fmt.Errorf("read TÜİK CİP %s: %w", indicator, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CIPResponse{}, fmt.Errorf("TÜİK CİP %s returned HTTP %d: %s", indicator, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out CIPResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return CIPResponse{}, fmt.Errorf("parse TÜİK CİP %s JSON: %w", indicator, err)
	}
	if len(out.Dates) == 0 || len(out.Rows) == 0 {
		return CIPResponse{}, fmt.Errorf("TÜİK CİP %s returned empty data", indicator)
	}
	return out, nil
}

func aggregateGDP(perCapUSD, perCapTRY, gdpTRY CIPResponse) ([]macro.GDPPoint, error) {
	if len(gdpTRY.Dates) == 0 {
		return nil, fmt.Errorf("GSYH dates are empty")
	}
	usdRows := rowsByRegion(perCapUSD.Rows)
	tryRows := rowsByRegion(perCapTRY.Rows)
	points := make([]macro.GDPPoint, 0, len(gdpTRY.Dates))
	for idx, yearText := range gdpTRY.Dates {
		year, err := strconv.Atoi(strings.TrimSpace(yearText))
		if err != nil {
			return nil, fmt.Errorf("parse GSYH year %q: %w", yearText, err)
		}
		totalGDPThousandTRY := 0.0
		totalPopulation := 0.0
		weightedPerCapUSD := 0.0
		for _, row := range gdpTRY.Rows {
			gdpThousandTRY, ok := parseCIPNumber(valueAt(row.Values, idx))
			if !ok || gdpThousandTRY <= 0 {
				continue
			}
			perCapTRYValue, ok := parseCIPNumber(valueAt(valueAtRow(tryRows, row.RegionCode), idx))
			if !ok || perCapTRYValue <= 0 {
				continue
			}
			perCapUSDValue, ok := parseCIPNumber(valueAt(valueAtRow(usdRows, row.RegionCode), idx))
			if !ok || perCapUSDValue <= 0 {
				continue
			}
			population := mathutil.SafeDiv(gdpThousandTRY*1000, perCapTRYValue)
			if population <= 0 {
				continue
			}
			totalGDPThousandTRY += gdpThousandTRY
			totalPopulation += population
			weightedPerCapUSD += perCapUSDValue * population
		}
		if totalGDPThousandTRY <= 0 || totalPopulation <= 0 {
			continue
		}
		points = append(points, macro.GDPPoint{
			Year:              year,
			GDPThousandTRY:    totalGDPThousandTRY,
			PerCapitaTRY:      mathutil.SafeDiv(totalGDPThousandTRY*1000, totalPopulation),
			PerCapitaUSD:      mathutil.SafeDiv(weightedPerCapUSD, totalPopulation),
			ImpliedPopulation: totalPopulation,
		})
	}
	if len(points) < 2 {
		return nil, fmt.Errorf("TÜİK CİP aggregate produced insufficient GSYH points")
	}
	return points, nil
}

func rowsByRegion(rows []CIPRow) map[string]CIPRow {
	out := make(map[string]CIPRow, len(rows))
	for _, row := range rows {
		out[strings.TrimSpace(row.RegionCode)] = row
	}
	return out
}

func valueAtRow(rows map[string]CIPRow, code string) []string {
	row, ok := rows[strings.TrimSpace(code)]
	if !ok {
		return nil
	}
	return row.Values
}

func valueAt(values []string, idx int) string {
	if idx < 0 || idx >= len(values) {
		return ""
	}
	return values[idx]
}

func parseCIPNumber(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "-9999" || value == "-8888" {
		return 0, false
	}
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\u00a0", "")
	value = strings.ReplaceAll(value, ",", ".")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
