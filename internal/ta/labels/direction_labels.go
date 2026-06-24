package labels

import (
	"math"

	"hissebot/internal/ta/features"
)

func DirectionLabel(ret, threshold float64) string {
	if threshold < 0 {
		threshold = math.Abs(threshold)
	}
	switch {
	case ret > threshold:
		return "up"
	case ret < -threshold:
		return "down"
	default:
		return "flat"
	}
}

func DirectionThreshold(bars []features.MarketBar, opts Options) float64 {
	base := opts.DirectionThreshold
	if len(bars) == 0 {
		return base
	}
	lastClose := bars[len(bars)-1].Close
	if lastClose <= 0 {
		return base
	}
	if opts.UseATRThreshold {
		atr := averageTrueRange(bars, 14)
		if atr > 0 {
			return math.Max(base, opts.ThresholdMultiplier*atr/lastClose)
		}
	}
	if opts.UseVolThreshold {
		vol := realizedVolatility(bars, 20)
		if vol > 0 {
			return math.Max(base, opts.ThresholdMultiplier*vol)
		}
	}
	return base
}

func averageTrueRange(bars []features.MarketBar, period int) float64 {
	if len(bars) < 2 {
		return 0
	}
	start := len(bars) - period
	if start < 1 {
		start = 1
	}
	sum := 0.0
	n := 0
	for i := start; i < len(bars); i++ {
		tr := math.Max(bars[i].High-bars[i].Low, math.Max(math.Abs(bars[i].High-bars[i-1].Close), math.Abs(bars[i].Low-bars[i-1].Close)))
		sum += tr
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func realizedVolatility(bars []features.MarketBar, period int) float64 {
	if len(bars) < 2 {
		return 0
	}
	start := len(bars) - period
	if start < 1 {
		start = 1
	}
	returns := []float64{}
	for i := start; i < len(bars); i++ {
		if bars[i-1].Close > 0 && bars[i].Close > 0 {
			returns = append(returns, math.Log(bars[i].Close/bars[i-1].Close))
		}
	}
	if len(returns) < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(returns)-1))
}
