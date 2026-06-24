package labels

import (
	"math"

	"hissebot/internal/ta/features"
)

func OpenGapReturn(current, next features.MarketBar) float64 {
	if current.Close <= 0 {
		return 0
	}
	return next.Open/current.Close - 1
}

func CloseIntradayReturn(next features.MarketBar) float64 {
	if next.Open <= 0 {
		return 0
	}
	return next.Close/next.Open - 1
}

func CloseToCloseReturn(current, next features.MarketBar) float64 {
	if current.Close <= 0 {
		return 0
	}
	return next.Close/current.Close - 1
}

func LogCloseToCloseReturn(current, next features.MarketBar) float64 {
	if current.Close <= 0 || next.Close <= 0 {
		return 0
	}
	return math.Log(next.Close / current.Close)
}
