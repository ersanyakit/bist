package ensemble

import (
	"math"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/ml"
)

func DetectRegime(fv features.FeatureVector) ml.RegimeSummary {
	vol := fv.Values["volatility_20d"]
	atrPct := 0.0
	if fv.Values["last_close"] > 0 {
		atrPct = fv.Values["atr14"] / fv.Values["last_close"]
	}
	effectiveVol := math.Max(vol, atrPct)
	regime := ml.RegimeSummary{
		VolatilityRegime: "normal_vol",
		TrendRegime:      "choppy",
		MarketRegime:     "market_sideways",
		RiskMultiplier:   1,
		PositionScale:    1,
	}
	switch {
	case effectiveVol > 0.05:
		regime.VolatilityRegime = "crisis"
		regime.RiskMultiplier = 0.45
		regime.PositionScale = 0.35
		regime.Reasons = append(regime.Reasons, "volatility_crisis")
	case effectiveVol > 0.03:
		regime.VolatilityRegime = "high_vol"
		regime.RiskMultiplier = 0.70
		regime.PositionScale = 0.65
		regime.Reasons = append(regime.Reasons, "high_volatility")
	case effectiveVol < 0.01:
		regime.VolatilityRegime = "low_vol"
		regime.RiskMultiplier = 0.85
		regime.PositionScale = 0.85
		regime.Reasons = append(regime.Reasons, "low_volatility")
	}
	slope := fv.Values["trend_slope_20d"]
	switch {
	case slope > 0.002:
		regime.TrendRegime = "trend_up"
		regime.Reasons = append(regime.Reasons, "positive_trend_slope")
	case slope < -0.002:
		regime.TrendRegime = "trend_down"
		regime.Reasons = append(regime.Reasons, "negative_trend_slope")
	case math.Abs(fv.Values["rsi14"]-50) > 18:
		regime.TrendRegime = "mean_reversion"
		regime.Reasons = append(regime.Reasons, "stretched_rsi_mean_reversion_risk")
	}
	marketReturn := fv.Values["bist100_return"]
	if marketReturn == 0 {
		marketReturn = fv.Values["return_20d"]
	}
	switch {
	case marketReturn > 0.025:
		regime.MarketRegime = "market_up"
	case marketReturn < -0.025:
		regime.MarketRegime = "market_down"
		regime.RiskMultiplier *= 0.85
		regime.PositionScale *= 0.85
	default:
		regime.MarketRegime = "market_sideways"
	}
	if regime.PositionScale <= 0 {
		regime.PositionScale = regime.RiskMultiplier
	}
	return regime
}

func RegimeWeightMultiplier(fv features.FeatureVector) float64 {
	regime := DetectRegime(fv)
	if regime.RiskMultiplier <= 0 {
		return 1
	}
	return regime.RiskMultiplier
}
