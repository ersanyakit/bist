package tcmb

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/datasources"
	"hissebot/internal/domain/macro"
)

const (
	defaultRatesBaseURL = "https://www.tcmb.gov.tr/kurlar"
	defaultPolicyURL    = "https://www.tcmb.gov.tr/wps/wcm/connect/tr/tcmb%2Btr/main%2Bmenu/temel%2Bfaaliyetler/para%2Bpolitikasi/merkez%2Bbankasi%2Bfaiz%2Boranlari/1%2Bhafta%2Brepo"
)

type Provider struct {
	BaseURL       string
	PolicyRateURL string
	APIKey        string
	Timeout       time.Duration
	HTTPClient    *http.Client
}

func New(baseURL string, apiKey string) Provider {
	return Provider{BaseURL: baseURL, APIKey: apiKey}
}

func (p Provider) Info() datasources.ProviderInfo {
	return datasources.ProviderInfo{
		Name:         "TCMB",
		SourceURL:    firstNonEmpty(p.BaseURL, defaultRatesBaseURL),
		License:      "official public TCMB sources; daily FX and 1-week repo policy table do not require an API key",
		RequiresKey:  false,
		Capabilities: []string{"policy_rate", "fx_rates"},
	}
}

func (p Provider) GetPolicyRate(ctx context.Context, from time.Time, to time.Time) ([]macro.Observation, error) {
	url := firstNonEmpty(p.PolicyRateURL, defaultPolicyURL)
	raw, err := p.get(ctx, url)
	if err != nil {
		return nil, err
	}
	return parsePolicyRateTable(raw, url, from, to)
}

func (p Provider) GetFXRate(ctx context.Context, pair string, from time.Time, to time.Time) ([]macro.Observation, error) {
	base, seriesID, err := normalizeTRYPair(pair)
	if err != nil {
		return nil, err
	}
	dates := rateDates(from, to)
	out := make([]macro.Observation, 0, len(dates))
	for _, date := range dates {
		url := p.rateURL(date)
		raw, err := p.get(ctx, url)
		if err != nil {
			if isMissingTCMBRateFile(err) {
				continue
			}
			return out, err
		}
		point, ok, err := parseRateXML(raw, url, base, seriesID)
		if err != nil {
			return out, err
		}
		if !ok || !inRange(point.Date, from, to) {
			continue
		}
		out = append(out, point)
	}
	return out, nil
}

func (p Provider) GetInflation(context.Context, string, time.Time, time.Time) ([]macro.Observation, error) {
	return nil, datasources.ErrUnsupportedCapability
}

func (p Provider) GetGDPGrowth(context.Context, time.Time, time.Time) ([]macro.Observation, error) {
	return nil, datasources.ErrUnsupportedCapability
}

func (p Provider) GetIndustrialProduction(context.Context, time.Time, time.Time) ([]macro.Observation, error) {
	return nil, datasources.ErrUnsupportedCapability
}

func (p Provider) rateURL(date time.Time) string {
	base := strings.TrimRight(firstNonEmpty(p.BaseURL, defaultRatesBaseURL), "/")
	if date.IsZero() {
		return base + "/today.xml"
	}
	return fmt.Sprintf("%s/%s/%s.xml", base, date.Format("200601"), date.Format("02012006"))
}

func (p Provider) get(ctx context.Context, url string) ([]byte, error) {
	client := p.HTTPClient
	if client == nil {
		timeout := p.Timeout
		if timeout <= 0 {
			timeout = 20 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/xml,text/html,*/*")
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
		return nil, httpStatusError{URL: url, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	return raw, nil
}

type httpStatusError struct {
	URL        string
	StatusCode int
	Body       string
}

func (e httpStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: HTTP %d", e.URL, e.StatusCode)
	}
	if len(e.Body) > 160 {
		e.Body = e.Body[:160]
	}
	return fmt.Sprintf("%s: HTTP %d: %s", e.URL, e.StatusCode, e.Body)
}

func isMissingTCMBRateFile(err error) bool {
	status, ok := err.(httpStatusError)
	return ok && (status.StatusCode == http.StatusNotFound || status.StatusCode == http.StatusForbidden)
}

type tcmbRatesXML struct {
	Date       string            `xml:"Date,attr"`
	DateTR     string            `xml:"Tarih,attr"`
	Currencies []tcmbCurrencyXML `xml:"Currency"`
}

type tcmbCurrencyXML struct {
	Code         string `xml:"CurrencyCode,attr"`
	LegacyCode   string `xml:"Kod,attr"`
	Unit         string `xml:"Unit"`
	ForexBuying  string `xml:"ForexBuying"`
	ForexSelling string `xml:"ForexSelling"`
}

func parseRateXML(raw []byte, sourceURL string, currencyCode string, seriesID macro.SeriesID) (macro.Observation, bool, error) {
	var doc tcmbRatesXML
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return macro.Observation{}, false, err
	}
	date, err := parseTCMBRateDate(firstNonEmpty(doc.Date, doc.DateTR))
	if err != nil {
		return macro.Observation{}, false, err
	}
	currencyCode = strings.ToUpper(currencyCode)
	for _, currency := range doc.Currencies {
		code := strings.ToUpper(firstNonEmpty(currency.Code, currency.LegacyCode))
		if code != currencyCode {
			continue
		}
		buying, hasBuying := parseDecimal(currency.ForexBuying)
		selling, hasSelling := parseDecimal(currency.ForexSelling)
		if !hasBuying && !hasSelling {
			return macro.Observation{}, false, nil
		}
		unit, hasUnit := parseDecimal(currency.Unit)
		if !hasUnit || unit == 0 {
			unit = 1
		}
		value := buying
		if hasBuying && hasSelling {
			value = (buying + selling) / 2
		} else if hasSelling {
			value = selling
		}
		return macro.Observation{
			SeriesID: seriesID,
			Date:     date,
			Value:    value / unit,
			Unit:     "TRY",
			Meta: macro.SourceMeta{
				Source:     "TCMB",
				SourceURL:  sourceURL,
				License:    "official public indicative exchange rate",
				AsOf:       date,
				IngestedAt: time.Now().UTC(),
			},
		}, true, nil
	}
	return macro.Observation{}, false, nil
}

var policyRowPattern = regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})\s+-\s+([0-9]+(?:[.,][0-9]+)?)`)

func parsePolicyRateTable(raw []byte, sourceURL string, from time.Time, to time.Time) ([]macro.Observation, error) {
	matches := policyRowPattern.FindAllStringSubmatch(string(raw), -1)
	out := make([]macro.Observation, 0, len(matches))
	for _, match := range matches {
		date, err := time.ParseInLocation("02.01.2006", match[1], time.Local)
		if err != nil {
			continue
		}
		date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		if !inRange(date, from, to) {
			continue
		}
		value, ok := parseDecimal(match[2])
		if !ok {
			continue
		}
		out = append(out, macro.Observation{
			SeriesID: macro.SeriesPolicyRate,
			Date:     date,
			Value:    value,
			Unit:     "pct",
			Meta: macro.SourceMeta{
				Source:     "TCMB",
				SourceURL:  sourceURL,
				License:    "official public policy rate table",
				AsOf:       date,
				IngestedAt: time.Now().UTC(),
			},
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tcmb policy rate table: no observations parsed from %s", sourceURL)
	}
	return out, nil
}

func parseTCMBRateDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"01/02/2006", "02.01.2006"} {
		if ts, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported TCMB rate date %q", value)
}

func normalizeTRYPair(pair string) (string, macro.SeriesID, error) {
	normalized := strings.ToUpper(strings.NewReplacer("/", "", "-", "", "_", "", " ", "").Replace(pair))
	switch normalized {
	case "USDTRY":
		return "USD", macro.SeriesUSDTRY, nil
	case "EURTRY":
		return "EUR", macro.SeriesEURTRY, nil
	default:
		return "", "", fmt.Errorf("unsupported TCMB FX pair %q; supported pairs: USDTRY, EURTRY", pair)
	}
}

func rateDates(from time.Time, to time.Time) []time.Time {
	if from.IsZero() && to.IsZero() {
		return []time.Time{time.Time{}}
	}
	if from.IsZero() {
		from = to
	}
	if to.IsZero() {
		to = from
	}
	from = dateOnly(from)
	to = dateOnly(to)
	if to.Before(from) {
		from, to = to, from
	}
	out := []time.Time{}
	for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
		out = append(out, date)
	}
	return out
}

func dateOnly(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func parseDecimal(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func inRange(ts time.Time, from time.Time, to time.Time) bool {
	ts = dateOnly(ts)
	if !from.IsZero() && ts.Before(dateOnly(from)) {
		return false
	}
	if !to.IsZero() && ts.After(dateOnly(to)) {
		return false
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
