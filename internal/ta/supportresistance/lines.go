// internal/supportresistance/lines.go
package supportresistance

import (
	"fmt"
	"math"
	"sort"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

type Result struct {
	SupportLevels     []ohlcv.SupportResistanceLevel `json:"support_levels"`
	ResistanceLevels  []ohlcv.SupportResistanceLevel `json:"resistance_levels"`
	NearestSupport    *ohlcv.SupportResistanceLevel  `json:"nearest_support,omitempty"`
	NearestResistance *ohlcv.SupportResistanceLevel  `json:"nearest_resistance,omitempty"`
}

type swingPoint struct {
	index  int
	price  float64
	kind   string
	volume float64
}

func Analyze(candles []ohlcv.Candle, atr float64, lookback int) (Result, error) {
	if len(candles) == 0 {
		return Result{}, fmt.Errorf("support resistance requires candles: %w", ErrNoCandles)
	}
	if lookback <= 0 || lookback > len(candles) {
		lookback = minInt(200, len(candles))
	}
	start := len(candles) - lookback
	points := detectSwings(candles[start:], start)
	supports := cluster(points, "support", candles, atr)
	resistances := cluster(points, "resistance", candles, atr)

	lastClose := candles[len(candles)-1].EffectiveClose()
	supports, resistances = reclassifyByCurrentPrice(supports, resistances, lastClose)
	sortLevels(supports, false)
	sortLevels(resistances, true)

	result := Result{
		SupportLevels:    supports,
		ResistanceLevels: resistances,
	}
	result.NearestSupport = nearestLevel(supports, lastClose, false)
	result.NearestResistance = nearestLevel(resistances, lastClose, true)
	return result, nil
}

var ErrNoCandles = fmt.Errorf("no candles")

func detectSwings(candles []ohlcv.Candle, offset int) []swingPoint {
	var points []swingPoint
	left := 3
	right := 3
	for i := left; i < len(candles)-right; i++ {
		high := candles[i].EffectiveHigh()
		low := candles[i].EffectiveLow()
		isHigh := true
		isLow := true
		for j := i - left; j <= i+right; j++ {
			if j == i {
				continue
			}
			if mathutil.LessOrEqual(high, candles[j].EffectiveHigh()) {
				isHigh = false
			}
			if mathutil.GreaterOrEqual(low, candles[j].EffectiveLow()) {
				isLow = false
			}
		}
		if isHigh {
			points = append(points, swingPoint{index: offset + i, price: high, kind: "resistance", volume: candles[i].EffectiveVolume()})
		}
		if isLow {
			points = append(points, swingPoint{index: offset + i, price: low, kind: "support", volume: candles[i].EffectiveVolume()})
		}
	}
	return points
}

func cluster(points []swingPoint, kind string, candles []ohlcv.Candle, atr float64) []ohlcv.SupportResistanceLevel {
	var levels []ohlcv.SupportResistanceLevel
	avgVolume := averageVolume(candles)
	for _, point := range points {
		if point.kind != kind {
			continue
		}
		tolerance := math.Max(atr*0.5, point.price*0.01)
		merged := false
		for i := range levels {
			if mathutil.AlmostEqualTol(levels[i].Price, point.price, tolerance) {
				oldTouches := levels[i].TouchCount
				levels[i].Price = (levels[i].Price*float64(oldTouches) + point.price) / float64(oldTouches+1)
				levels[i].TouchCount++
				levels[i].LastTouchedAt = candles[point.index].Time
				levels[i].Strength = strengthScore(levels[i].TouchCount, point.index, len(candles), point.volume, avgVolume)
				merged = true
				break
			}
		}
		if !merged {
			levels = append(levels, ohlcv.SupportResistanceLevel{
				Type:          kind,
				Price:         point.price,
				Strength:      strengthScore(1, point.index, len(candles), point.volume, avgVolume),
				TouchCount:    1,
				LastTouchedAt: candles[point.index].Time,
			})
		}
	}
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Strength > levels[j].Strength
	})
	if len(levels) > 8 {
		levels = levels[:8]
	}
	return levels
}

func strengthScore(touches, index, total int, volume, avgVolume float64) float64 {
	touchScore := mathutil.Clamp(float64(touches)/5, 0, 1)
	recencyScore := 1 - mathutil.Clamp(float64(total-1-index)/float64(maxInt(total, 1)), 0, 1)
	volumeScore := mathutil.Clamp(mathutil.SafeDiv(volume, avgVolume)/2, 0, 1)
	score := touchScore*0.45 + recencyScore*0.30 + volumeScore*0.25
	return mathutil.Clamp(score, 0, 1)
}

func nearestLevel(levels []ohlcv.SupportResistanceLevel, price float64, above bool) *ohlcv.SupportResistanceLevel {
	var selected *ohlcv.SupportResistanceLevel
	bestDistance := math.MaxFloat64
	for i := range levels {
		level := levels[i]
		if above && mathutil.LessOrEqual(level.Price, price) {
			continue
		}
		if !above && mathutil.GreaterOrEqual(level.Price, price) {
			continue
		}
		distance := math.Abs(level.Price - price)
		if distance < bestDistance {
			bestDistance = distance
			selected = &levels[i]
		}
	}
	return selected
}

func reclassifyByCurrentPrice(supports, resistances []ohlcv.SupportResistanceLevel, lastClose float64) ([]ohlcv.SupportResistanceLevel, []ohlcv.SupportResistanceLevel) {
	reclassifiedSupports := make([]ohlcv.SupportResistanceLevel, 0, len(supports)+len(resistances))
	reclassifiedResistances := make([]ohlcv.SupportResistanceLevel, 0, len(supports)+len(resistances))
	allLevels := append([]ohlcv.SupportResistanceLevel{}, supports...)
	allLevels = append(allLevels, resistances...)
	for _, level := range allLevels {
		if level.Price <= 0 {
			continue
		}
		switch {
		case level.Price < lastClose:
			level.Type = "support"
			reclassifiedSupports = append(reclassifiedSupports, level)
		case level.Price > lastClose:
			level.Type = "resistance"
			reclassifiedResistances = append(reclassifiedResistances, level)
		}
	}
	return strongestLevels(reclassifiedSupports, 8), strongestLevels(reclassifiedResistances, 8)
}

func strongestLevels(levels []ohlcv.SupportResistanceLevel, limit int) []ohlcv.SupportResistanceLevel {
	if limit <= 0 || len(levels) <= limit {
		return levels
	}
	sort.SliceStable(levels, func(i, j int) bool {
		if levels[i].Strength == levels[j].Strength {
			return levels[i].Price < levels[j].Price
		}
		return levels[i].Strength > levels[j].Strength
	})
	return append([]ohlcv.SupportResistanceLevel{}, levels[:limit]...)
}

func sortLevels(levels []ohlcv.SupportResistanceLevel, ascending bool) {
	sort.Slice(levels, func(i, j int) bool {
		if ascending {
			return levels[i].Price < levels[j].Price
		}
		return levels[i].Price > levels[j].Price
	})
}

func averageVolume(candles []ohlcv.Candle) float64 {
	sum := 0.0
	for _, candle := range candles {
		sum += candle.EffectiveVolume()
	}
	return mathutil.SafeDiv(sum, float64(len(candles)))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
