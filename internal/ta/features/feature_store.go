package features

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"
)

const DefaultFeatureSetVersion = "bist_ml_features_v1"

type MarketBar struct {
	Symbol   string    `json:"symbol"`
	Time     time.Time `json:"time"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
	VWAP     *float64  `json:"vwap,omitempty"`
	Source   string    `json:"source"`
	Adjusted bool      `json:"adjusted"`
}

type CorporateAction struct {
	Symbol           string    `json:"symbol"`
	EffectiveDate    time.Time `json:"effective_date"`
	AnnouncementDate time.Time `json:"announcement_date"`
	Type             string    `json:"type"`
	Ratio            float64   `json:"ratio"`
	CashAmount       float64   `json:"cash_amount"`
	Source           string    `json:"source"`
}

type FeatureVector struct {
	Symbol            string               `json:"symbol"`
	AsOf              time.Time            `json:"as_of"`
	Horizon           string               `json:"horizon"`
	FeatureSetVersion string               `json:"feature_set_version"`
	Values            map[string]float64   `json:"values"`
	Categorical       map[string]string    `json:"categorical,omitempty"`
	SourceTimestamps  map[string]time.Time `json:"source_timestamps,omitempty"`
	Quality           FeatureQuality       `json:"quality"`
}

type FeatureQuality struct {
	MissingRatio  float64  `json:"missing_ratio"`
	StaleFields   []string `json:"stale_fields,omitempty"`
	SuspectFields []string `json:"suspect_fields,omitempty"`
	LeakageFlags  []string `json:"leakage_flags,omitempty"`
	SourceScore   float64  `json:"source_score"`
	IsTradable    bool     `json:"is_tradable"`
}

type StoreOptions struct {
	Root              string
	FeatureSetVersion string
	PreferAdjusted    bool
	MaxStaleness      time.Duration
}

type Store struct {
	root              string
	featureSetVersion string
	preferAdjusted    bool
	maxStaleness      time.Duration
}

func NewStore(opts StoreOptions) *Store {
	version := strings.TrimSpace(opts.FeatureSetVersion)
	if version == "" {
		version = DefaultFeatureSetVersion
	}
	maxStaleness := opts.MaxStaleness
	if maxStaleness <= 0 {
		maxStaleness = 96 * time.Hour
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = filepath.Join("data", "equities")
	}
	return &Store{
		root:              root,
		featureSetVersion: version,
		preferAdjusted:    opts.PreferAdjusted,
		maxStaleness:      maxStaleness,
	}
}

func (s *Store) Build(ctx context.Context, symbol string, asOf time.Time, horizon string) (FeatureVector, []MarketBar, error) {
	if err := ctx.Err(); err != nil {
		return FeatureVector{}, nil, fmt.Errorf("build feature vector canceled: %w", err)
	}
	bars, warnings, err := s.LoadMarketBars(ctx, symbol, asOf, "D")
	if err != nil {
		fv := EmptyFeatureVector(symbol, asOf, horizon, s.featureSetVersion)
		fv.Quality.MissingRatio = 1
		fv.Quality.SourceScore = 0
		fv.Quality.IsTradable = false
		fv.Quality.SuspectFields = append(fv.Quality.SuspectFields, err.Error())
		return fv, nil, nil
	}
	fv := BuildFromBars(symbol, asOf, horizon, s.featureSetVersion, bars)
	fv.Quality.SuspectFields = append(fv.Quality.SuspectFields, warnings...)
	if err := GuardFeatureVector(fv); err != nil {
		fv.Quality.LeakageFlags = append(fv.Quality.LeakageFlags, err.Error())
		fv.Quality.IsTradable = false
	}
	return fv, bars, nil
}

func (s *Store) LoadMarketBars(ctx context.Context, symbol string, asOf time.Time, interval string) ([]MarketBar, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("load market bars canceled: %w", err)
	}
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, nil, errors.New("symbol is required")
	}
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	interval = strings.TrimSpace(interval)
	if interval == "" {
		interval = "D"
	}
	candidates := []struct {
		interval string
		adjusted bool
	}{}
	if s.preferAdjusted {
		candidates = append(candidates, struct {
			interval string
			adjusted bool
		}{interval + "_adjusted", true})
	}
	candidates = append(candidates, struct {
		interval string
		adjusted bool
	}{interval, false})

	var readErrs []string
	for _, candidate := range candidates {
		path := filepath.Join(s.root, ohlcv.SymbolPathKey(symbol), "charts", candidate.interval+".json")
		bars, err := readChartFile(path, symbol, candidate.adjusted)
		if err != nil {
			readErrs = append(readErrs, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		cutoff := endOfAsOf(asOf)
		out := make([]MarketBar, 0, len(bars))
		for _, bar := range bars {
			if !bar.Time.After(cutoff) {
				out = append(out, bar)
			}
		}
		if len(out) == 0 {
			readErrs = append(readErrs, fmt.Sprintf("%s: no point-in-time bars before %s", path, cutoff.Format(time.RFC3339)))
			continue
		}
		warnings := []string{}
		if !candidate.adjusted && s.preferAdjusted {
			warnings = append(warnings, "adjusted_chart_missing_using_raw_ohlcv")
		}
		latest := out[len(out)-1].Time
		if cutoff.Sub(latest) > s.maxStaleness {
			warnings = append(warnings, "ohlcv_stale_"+latest.Format("2006-01-02"))
		}
		return out, warnings, nil
	}
	return nil, nil, fmt.Errorf("daily chart unavailable: %s", strings.Join(readErrs, "; "))
}

func EmptyFeatureVector(symbol string, asOf time.Time, horizon string, version string) FeatureVector {
	if version == "" {
		version = DefaultFeatureSetVersion
	}
	if horizon == "" {
		horizon = "1d"
	}
	return FeatureVector{
		Symbol:            strings.ToUpper(strings.TrimSpace(symbol)),
		AsOf:              asOf.UTC(),
		Horizon:           horizon,
		FeatureSetVersion: version,
		Values:            map[string]float64{},
		Categorical:       map[string]string{},
		SourceTimestamps:  map[string]time.Time{},
		Quality: FeatureQuality{
			MissingRatio: 1,
			SourceScore:  0,
			IsTradable:   false,
		},
	}
}

func BuildFromCandles(symbol string, asOf time.Time, horizon, version string, candles []ohlcv.Candle) FeatureVector {
	bars := make([]MarketBar, 0, len(candles))
	for _, c := range candles {
		if c.Time.IsZero() {
			continue
		}
		bars = append(bars, MarketBar{
			Symbol:   symbol,
			Time:     c.Time.UTC(),
			Open:     c.EffectiveOpen(),
			High:     c.EffectiveHigh(),
			Low:      c.EffectiveLow(),
			Close:    c.EffectiveClose(),
			Volume:   c.EffectiveVolume(),
			Source:   "analysis_timeframe_candles",
			Adjusted: c.IsAdjusted,
		})
	}
	sort.SliceStable(bars, func(i, j int) bool { return bars[i].Time.Before(bars[j].Time) })
	cutoff := endOfAsOf(asOf)
	filtered := bars[:0]
	for _, bar := range bars {
		if !bar.Time.After(cutoff) {
			filtered = append(filtered, bar)
		}
	}
	return BuildFromBars(symbol, asOf, horizon, version, filtered)
}

type chartFile struct {
	Source   string        `json:"source"`
	Ticker   string        `json:"ticker"`
	Symbol   string        `json:"symbol"`
	Interval string        `json:"interval"`
	Candles  []chartCandle `json:"candles"`
}

type chartCandle struct {
	Time   int64    `json:"time"`
	Open   *float64 `json:"open,omitempty"`
	High   *float64 `json:"high,omitempty"`
	Low    *float64 `json:"low,omitempty"`
	Close  *float64 `json:"close,omitempty"`
	Volume *float64 `json:"volume,omitempty"`
}

func readChartFile(path string, symbol string, adjusted bool) ([]MarketBar, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file chartFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse chart json: %w", err)
	}
	out := make([]MarketBar, 0, len(file.Candles))
	source := strings.TrimSpace(file.Source)
	if source == "" {
		source = path
	}
	for _, item := range file.Candles {
		if item.Time <= 0 || item.Open == nil || item.High == nil || item.Low == nil || item.Close == nil {
			continue
		}
		volume := 0.0
		if item.Volume != nil {
			volume = *item.Volume
		}
		bar := MarketBar{
			Symbol:   symbol,
			Time:     time.Unix(item.Time, 0).UTC(),
			Open:     *item.Open,
			High:     *item.High,
			Low:      *item.Low,
			Close:    *item.Close,
			Volume:   volume,
			Source:   source,
			Adjusted: adjusted,
		}
		if validBar(bar) {
			out = append(out, bar)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return dedupeBars(out), nil
}

func validBar(bar MarketBar) bool {
	values := []float64{bar.Open, bar.High, bar.Low, bar.Close, bar.Volume}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return bar.Open > 0 && bar.High > 0 && bar.Low > 0 && bar.Close > 0 && bar.High >= bar.Low
}

func dedupeBars(bars []MarketBar) []MarketBar {
	if len(bars) < 2 {
		return bars
	}
	out := make([]MarketBar, 0, len(bars))
	for _, bar := range bars {
		if len(out) > 0 && sameSession(out[len(out)-1].Time, bar.Time) {
			out[len(out)-1] = bar
			continue
		}
		out = append(out, bar)
	}
	return out
}

func sameSession(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

func endOfAsOf(asOf time.Time) time.Time {
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	y, m, d := asOf.UTC().Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
}
