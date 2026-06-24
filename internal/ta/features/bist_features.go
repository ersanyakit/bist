package features

import "math"

func addBISTFeatures(fv *FeatureVector) {
	for _, key := range []string{
		"free_float_ratio", "free_float_market_cap", "float_change", "index_membership",
		"index_portfolio_weight", "official_bulletin_open", "official_bulletin_close",
		"official_bulletin_vwap", "official_bulletin_volume",
		"bist_volume_relative_strength", "market_wide_regime_proxy",
	} {
		if _, ok := fv.Values[key]; !ok {
			fv.Values[key] = 0
		}
	}
}

func BISTTickSize(price float64) float64 {
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0.01
	}
	switch {
	case price < 20:
		return 0.01
	case price < 50:
		return 0.02
	case price < 100:
		return 0.05
	case price < 250:
		return 0.10
	case price < 500:
		return 0.25
	case price < 1000:
		return 0.50
	default:
		return 1.00
	}
}

func RoundToTick(price, tick float64) float64 {
	if tick <= 0 {
		tick = BISTTickSize(price)
	}
	if price <= 0 {
		return 0
	}
	return math.Round(price/tick) * tick
}
