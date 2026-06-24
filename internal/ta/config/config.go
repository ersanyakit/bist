// internal/ta/config/config.go
package config

import (
	"errors"
	"fmt"
	"strings"
)

const (
	MinWorkers         = 5
	DefaultCandleLimit = 260
)

var (
	ErrInputRequired    = errors.New("symbol or excel input is required")
	ErrInvalidProvider  = errors.New("invalid provider")
	ErrInvalidTimeframe = errors.New("invalid timeframe")
	ErrNoSymbols        = errors.New("no symbols found")
)

type Config struct {
	Symbol                   string
	ExcelPath                string
	Provider                 string
	DataDir                  string
	OutputDir                string
	TimeframesCSV            string
	Timeframes               []string
	Workers                  int
	Limit                    int
	DataMode                 string
	Benchmark                string
	ValuationAssumptionsFile string
	MacroGDPFile             string
	Portfolio                float64
	RiskPct                  float64
	PeerLimit                int
}

func Default() Config {
	timeframes := []string{"1D", "1W", "1M", "3M", "6M", "1Y", "YTD", "ALL"}
	return Config{
		Provider:                 "tradingview",
		DataDir:                  "./data",
		OutputDir:                "./data/equities",
		TimeframesCSV:            strings.Join(timeframes, ","),
		Timeframes:               timeframes,
		Workers:                  MinWorkers,
		Limit:                    DefaultCandleLimit,
		DataMode:                 "research",
		Benchmark:                "XU100",
		ValuationAssumptionsFile: "./data/seed/valuation_assumptions.json",
		MacroGDPFile:             "./data/macro/tuik_gdp.json",
		Portfolio:                100000,
		RiskPct:                  1,
		PeerLimit:                20,
	}
}

func ParseTimeframes(value string) ([]string, error) {
	allowed := map[string]bool{"1D": true, "1W": true, "1M": true, "3M": true, "6M": true, "1Y": true, "YTD": true, "ALL": true}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		tf := strings.ToUpper(strings.TrimSpace(part))
		if tf == "" {
			continue
		}
		if !allowed[tf] {
			return nil, fmt.Errorf("timeframe %q is not supported: %w", tf, ErrInvalidTimeframe)
		}
		if !seen[tf] {
			result = append(result, tf)
			seen[tf] = true
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty timeframe list: %w", ErrInvalidTimeframe)
	}
	return result, nil
}
