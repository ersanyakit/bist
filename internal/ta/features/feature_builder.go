package features

import (
	"math"
	"sort"
	"strings"
	"time"
)

var defaultFeatureNames = []string{
	"last_close", "last_open", "last_high", "last_low", "last_volume",
	"return_1d", "return_2d", "return_3d", "return_5d", "return_10d", "return_20d",
	"log_return_1d", "log_return_5d", "log_return_20d",
	"volatility_5d", "volatility_20d", "atr14", "realized_volatility_20d",
	"rsi14", "macd", "macd_signal", "macd_histogram", "bollinger_zscore",
	"ema20_distance", "sma20_distance", "supertrend_direction", "cmf20",
	"stochastic_k", "stochastic_d", "volume_change_1d", "volume_zscore20",
	"candle_body_ratio", "upper_wick_ratio", "lower_wick_ratio", "gap_1d",
	"previous_close_distance", "support_distance_pct", "resistance_distance_pct",
	"trend_slope_20d", "vwap_distance",
	"kap_event_count_1d", "kap_event_count_3d", "kap_event_count_7d", "kap_event_count_30d",
	"material_disclosure_flag", "earnings_event_flag", "dividend_or_capital_increase_flag",
	"sentiment_score", "sentiment_confidence", "news_volume_zscore", "kap_risk_flags",
	"free_float_ratio", "free_float_market_cap", "float_change", "index_membership",
	"bist_volume_relative_strength", "market_wide_regime_proxy",
	"usdtry_change", "interest_rate_level", "interest_rate_change", "inflation_proxy",
	"bist100_return", "sector_index_return", "market_volatility_proxy",
	"order_imbalance", "spread", "quote_depth", "intraday_range",
	"volume_profile", "first_hour_return", "last_hour_return",
	"financial_quality_score", "buffett_checklist_score",
}

func BuildFromBars(symbol string, asOf time.Time, horizon string, version string, bars []MarketBar) FeatureVector {
	fv := EmptyFeatureVector(symbol, asOf, horizon, version)
	fv.Quality.MissingRatio = 0
	fv.Quality.SourceScore = 1
	fv.Quality.IsTradable = false
	if len(bars) == 0 {
		fv.Quality.MissingRatio = 1
		fv.Quality.SourceScore = 0
		fv.Quality.SuspectFields = append(fv.Quality.SuspectFields, "no_market_bars")
		return fv
	}
	sort.SliceStable(bars, func(i, j int) bool { return bars[i].Time.Before(bars[j].Time) })
	latest := bars[len(bars)-1]
	fv.Symbol = strings.ToUpper(strings.TrimSpace(symbol))
	fv.AsOf = asOf.UTC()
	fv.SourceTimestamps["ohlcv"] = latest.Time.UTC()
	addTechnicalFeatures(&fv, bars)
	addFundamentalFeatures(&fv)
	addBISTFeatures(&fv)
	addMacroFeatures(&fv)
	addNewsFeatures(&fv)
	addMKKFeatures(&fv)
	finalizeQuality(&fv)
	return fv
}

func finalizeQuality(fv *FeatureVector) {
	missing := 0
	total := 0
	for _, name := range defaultFeatureNames {
		total++
		value, ok := fv.Values[name]
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			missing++
			fv.Values[name] = 0
		}
	}
	if total > 0 {
		fv.Quality.MissingRatio = float64(missing) / float64(total)
	}
	fv.Quality.SourceScore = clamp01(1 - fv.Quality.MissingRatio)
	if fv.Values["last_close"] > 0 && fv.Values["last_volume"] > 0 && len(fv.Quality.LeakageFlags) == 0 && fv.Quality.SourceScore >= 0.55 {
		fv.Quality.IsTradable = true
	}
	if fv.Quality.MissingRatio > 0.35 {
		fv.Quality.SuspectFields = append(fv.Quality.SuspectFields, "high_missing_feature_ratio")
	}
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func setFinite(values map[string]float64, key string, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		values[key] = 0
		return
	}
	values[key] = value
}
