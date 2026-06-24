package labels

import "hissebot/internal/ta/features"

func RegimeLabel(bars []features.MarketBar) string {
	if len(bars) < 20 {
		return "normal_vol_choppy"
	}
	vol := realizedVolatility(bars, 20)
	volRegime := "normal_vol"
	switch {
	case vol > 0.05:
		volRegime = "crisis"
	case vol > 0.03:
		volRegime = "high_vol"
	case vol < 0.01:
		volRegime = "low_vol"
	}
	trend := "choppy"
	first := bars[len(bars)-20].Close
	last := bars[len(bars)-1].Close
	if first > 0 {
		r := last/first - 1
		if r > 0.04 {
			trend = "trend_market_up"
		} else if r < -0.04 {
			trend = "trend_market_down"
		} else {
			trend = "mean_reversion_market_sideways"
		}
	}
	return volRegime + "_" + trend
}
