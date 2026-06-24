package labels

import (
	"time"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/forecastpolicy"
)

type ForecastTarget struct {
	Symbol             string             `json:"symbol"`
	AsOf               time.Time          `json:"as_of"`
	Horizon            string             `json:"horizon"`
	OpenReturn         float64            `json:"open_return"`
	CloseReturn        float64            `json:"close_return"`
	LogReturn          float64            `json:"log_return"`
	Direction          string             `json:"direction"`
	DirectionThreshold float64            `json:"direction_threshold"`
	Quantiles          map[string]float64 `json:"quantiles,omitempty"`
	TripleBarrierLabel string             `json:"triple_barrier_label,omitempty"`
	MetaLabel          int                `json:"meta_label"`
	RegimeLabel        string             `json:"regime_label,omitempty"`
}

type Options struct {
	Horizon             string
	DirectionThreshold  float64
	UseATRThreshold     bool
	UseVolThreshold     bool
	ThresholdMultiplier float64
	ProfitTaking        float64
	StopLoss            float64
	MaxHoldingBars      int
}

func DefaultOptions() Options {
	return Options{
		Horizon:             "1d",
		DirectionThreshold:  forecastpolicy.NextSessionDirectionToleranceReturn(),
		ThresholdMultiplier: 0.50,
		ProfitTaking:        0.02,
		StopLoss:            0.015,
		MaxHoldingBars:      5,
	}
}

func BuildTarget(symbol string, bars []features.MarketBar, asOf time.Time, opts Options) (ForecastTarget, bool) {
	opts = withDefaults(opts)
	i := indexAtOrBefore(bars, asOf)
	if i < 0 || i+1 >= len(bars) {
		return ForecastTarget{}, false
	}
	current := bars[i]
	next := bars[i+1]
	threshold := DirectionThreshold(bars[:i+1], opts)
	cc := CloseToCloseReturn(current, next)
	target := ForecastTarget{
		Symbol:             symbol,
		AsOf:               current.Time.UTC(),
		Horizon:            opts.Horizon,
		OpenReturn:         OpenGapReturn(current, next),
		CloseReturn:        CloseIntradayReturn(next),
		LogReturn:          LogCloseToCloseReturn(current, next),
		Direction:          DirectionLabel(cc, threshold),
		DirectionThreshold: threshold,
		Quantiles:          QuantileLabels(bars, i, []float64{0.05, 0.10, 0.25, 0.50, 0.75, 0.90, 0.95}),
		TripleBarrierLabel: TripleBarrierLabel(bars, i, opts),
		MetaLabel:          MetaLabel(cc, threshold),
		RegimeLabel:        RegimeLabel(bars[:i+1]),
	}
	return target, true
}

func withDefaults(opts Options) Options {
	def := DefaultOptions()
	if opts.Horizon == "" {
		opts.Horizon = def.Horizon
	}
	if opts.DirectionThreshold <= 0 {
		opts.DirectionThreshold = def.DirectionThreshold
	}
	if opts.ThresholdMultiplier <= 0 {
		opts.ThresholdMultiplier = def.ThresholdMultiplier
	}
	if opts.ProfitTaking <= 0 {
		opts.ProfitTaking = def.ProfitTaking
	}
	if opts.StopLoss <= 0 {
		opts.StopLoss = def.StopLoss
	}
	if opts.MaxHoldingBars <= 0 {
		opts.MaxHoldingBars = def.MaxHoldingBars
	}
	return opts
}

func indexAtOrBefore(bars []features.MarketBar, asOf time.Time) int {
	if asOf.IsZero() {
		return len(bars) - 1
	}
	cutoff := time.Date(asOf.UTC().Year(), asOf.UTC().Month(), asOf.UTC().Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	out := -1
	for i, bar := range bars {
		if !bar.Time.After(cutoff) {
			out = i
		}
	}
	return out
}
