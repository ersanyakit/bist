package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type FeatureDefinition struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Group       string `json:"group"`
	Description string `json:"description,omitempty"`
	PointInTime bool   `json:"point_in_time"`
}

type FeatureSchema struct {
	Version  string              `json:"version"`
	Features []FeatureDefinition `json:"features"`
}

func DefaultSchema(version string) FeatureSchema {
	if version == "" {
		version = DefaultFeatureSetVersion
	}
	defs := make([]FeatureDefinition, 0, len(defaultFeatureNames))
	for _, name := range defaultFeatureNames {
		defs = append(defs, FeatureDefinition{Name: name, Type: "float64", Group: featureGroup(name), PointInTime: true})
	}
	return FeatureSchema{Version: version, Features: defs}
}

func ExportSchema(path string, schema FeatureSchema) error {
	if path == "" {
		return fmt.Errorf("schema export path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure schema dir: %w", err)
	}
	raw, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal feature schema: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write feature schema: %w", err)
	}
	return nil
}

func featureGroup(name string) string {
	switch {
	case hasPrefixAny(name, "return_", "log_return_", "volatility_", "atr", "rsi", "macd", "bollinger", "ema", "sma", "supertrend", "cmf", "stochastic", "volume_", "candle_", "gap_", "support_", "resistance_", "trend_", "vwap", "intraday", "order_", "spread", "quote_", "first_", "last_"):
		return "technical"
	case hasPrefixAny(name, "revenue", "net_income", "ebitda", "gross_", "operating_", "roe", "roa", "debt", "net_debt", "current_", "quick_", "fcf", "pe", "pb", "ev_", "financial_", "buffett"):
		return "fundamental"
	case hasPrefixAny(name, "kap_", "material_", "earnings_", "dividend_", "sentiment", "news_"):
		return "kap_news"
	case hasPrefixAny(name, "free_float", "float_", "index_", "official_", "bist_"):
		return "bist_mkk"
	case hasPrefixAny(name, "usdtry", "interest_", "inflation", "sector_", "market_"):
		return "macro"
	default:
		return "other"
	}
}

func hasPrefixAny(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
