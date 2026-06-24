package labels

import (
	"fmt"
	"math"
	"sort"

	"hissebot/internal/ta/features"
)

func QuantileLabels(bars []features.MarketBar, index int, quantiles []float64) map[string]float64 {
	out := map[string]float64{}
	if index < 0 || index+1 >= len(bars) {
		return out
	}
	forward := make([]float64, 0, len(bars)-index-1)
	base := bars[index].Close
	if base <= 0 {
		return out
	}
	for i := index + 1; i < len(bars); i++ {
		forward = append(forward, bars[i].Close/base-1)
	}
	sort.Float64s(forward)
	for _, q := range quantiles {
		key := fmt.Sprintf("p%02.0f", q*100)
		out[key] = quantile(forward, q)
	}
	return out
}

func quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		return values[0]
	}
	if q >= 1 {
		return values[len(values)-1]
	}
	pos := q * float64(len(values)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return values[lo]
	}
	w := pos - float64(lo)
	return values[lo]*(1-w) + values[hi]*w
}
