package formations

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/indicators"
	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

const (
	statusActive   = "active"
	statusBroken   = "broken"
	statusRetested = "retested"
)

var candleDateLocation = time.FixedZone("TRT", 3*60*60)

func Analyze(candles []ohlcv.Candle, opts Options) (Result, error) {
	if len(candles) < 20 {
		return Result{}, fmt.Errorf("formation analysis requires at least twenty candles: %w", indicators.ErrInsufficientData)
	}
	opts = normalizeOptions(opts, len(candles))
	closes := closeValues(candles)
	ema20 := indicators.EMASeries(closes, 20)
	ema50 := indicators.EMASeries(closes, 50)
	atrSeries := indicators.ATRSeries(candles, 14)
	atr := lastValue(atrSeries)
	current := candles[len(candles)-1].EffectiveClose()
	analysisDate := opts.AnalysisDate
	if analysisDate == "" {
		analysisDate = candleDate(candles[len(candles)-1])
	}

	swings := DetectSwings(candles, opts.PivotLookback)
	supports, resistances := buildLevels(candles, swings, atr, opts)
	trendlines := buildTrendlines(candles, swings, atr, opts)
	patterns := detectPatterns(candles, swings, supports, resistances, trendlines, atr, opts)
	breakout := detectBreakout(candles, supports, resistances, atr)
	trend := detectTrend(candles, ema50, atr)
	ma := movingAverageSummary(current, ema20, ema50, atr)
	scenarios := buildScenarios(candles, current, supports, resistances, trendlines, patterns, ma)
	drawings := buildDrawingObjects(candles, supports, resistances, trendlines, patterns, scenarios, opts)
	summary := buildSummary(current, supports, resistances, patterns, breakout, ma)

	return Result{
		Symbol:            opts.Symbol,
		Timeframe:         opts.Timeframe,
		CurrentPrice:      round2(current),
		AnalysisDate:      analysisDate,
		Trend:             trend,
		MovingAverages:    ma,
		SupportResistance: SupportResistanceSummary{Supports: supports, Resistances: resistances},
		Trendlines:        exportTrendlines(trendlines),
		Patterns:          patterns,
		BreakoutAnalysis:  breakout,
		Scenarios:         scenarios,
		DrawingObjects:    drawings,
		Summary:           summary,
	}, nil
}

func normalizeOptions(opts Options, candleCount int) Options {
	if opts.Timeframe == "" {
		opts.Timeframe = "1D"
	}
	profile := timeframeGeometryProfile(opts.Timeframe, candleCount)
	if opts.PivotLookback <= 0 {
		opts.PivotLookback = profile.PivotLookback
	}
	if opts.LevelLookback <= 0 || opts.LevelLookback > candleCount {
		opts.LevelLookback = profile.LevelLookback
	}
	if opts.MaxLevels <= 0 {
		opts.MaxLevels = 5
	}
	if opts.ConsolidationWindow <= 0 {
		opts.ConsolidationWindow = 20
	}
	if opts.TrendlineSwingLimit <= 0 {
		opts.TrendlineSwingLimit = profile.TrendlineSwingLimit
	}
	if opts.MinTrendlineSpanBars <= 0 {
		opts.MinTrendlineSpanBars = profile.MinTrendlineSpanBars
	}
	return opts
}

type geometryProfile struct {
	PivotLookback        int
	LevelLookback        int
	TrendlineSwingLimit  int
	MinTrendlineSpanBars int
}

func timeframeGeometryProfile(timeframe string, candleCount int) geometryProfile {
	if candleCount < 1 {
		candleCount = 1
	}
	tf := strings.ToUpper(strings.TrimSpace(timeframe))
	full := candleCount
	switch tf {
	case "1H", "60", "30", "15", "5":
		return geometryProfile{
			PivotLookback:        8,
			LevelLookback:        minInt(candleCount, 260),
			TrendlineSwingLimit:  minInt(24, maxInt(10, candleCount/6)),
			MinTrendlineSpanBars: maxInt(10, candleCount/28),
		}
	case "1W", "W":
		return geometryProfile{
			PivotLookback:        4,
			LevelLookback:        full,
			TrendlineSwingLimit:  minInt(26, maxInt(10, candleCount/5)),
			MinTrendlineSpanBars: maxInt(6, candleCount/18),
		}
	case "1M", "M", "3M", "6M", "1Y", "YTD", "ALL":
		return geometryProfile{
			PivotLookback:        4,
			LevelLookback:        full,
			TrendlineSwingLimit:  minInt(22, maxInt(8, candleCount/5)),
			MinTrendlineSpanBars: maxInt(4, candleCount/24),
		}
	default:
		return geometryProfile{
			PivotLookback:        5,
			LevelLookback:        full,
			TrendlineSwingLimit:  minInt(30, maxInt(12, candleCount/5)),
			MinTrendlineSpanBars: maxInt(8, candleCount/16),
		}
	}
}

func DetectSwings(candles []ohlcv.Candle, lookback int) []SwingPoint {
	if lookback <= 0 {
		lookback = 5
	}
	if len(candles) < lookback*2+1 {
		return nil
	}
	out := make([]SwingPoint, 0)
	for i := lookback; i < len(candles)-lookback; i++ {
		high := candles[i].EffectiveHigh()
		low := candles[i].EffectiveLow()
		isHigh := true
		isLow := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j == i {
				continue
			}
			// Tie-break on equal high/low: an earlier bar (j < i) tying the candidate
			// disqualifies it (the earlier occurrence is the pivot), but a later bar
			// (j > i) merely tying does not — otherwise a flat-top/flat-bottom plateau
			// disqualifies every bar in it and the swing is silently dropped entirely.
			if j < i {
				if candles[j].EffectiveHigh() >= high {
					isHigh = false
				}
				if candles[j].EffectiveLow() <= low {
					isLow = false
				}
			} else {
				if candles[j].EffectiveHigh() > high {
					isHigh = false
				}
				if candles[j].EffectiveLow() < low {
					isLow = false
				}
			}
			if !isHigh && !isLow {
				break
			}
		}
		if isHigh {
			out = append(out, SwingPoint{Index: i, Time: candleDate(candles[i]), Price: high, Kind: "high", Volume: candles[i].EffectiveVolume()})
		}
		if isLow {
			out = append(out, SwingPoint{Index: i, Time: candleDate(candles[i]), Price: low, Kind: "low", Volume: candles[i].EffectiveVolume()})
		}
	}
	return out
}

func buildLevels(candles []ohlcv.Candle, swings []SwingPoint, atr float64, opts Options) ([]Level, []Level) {
	start := maxInt(0, len(candles)-opts.LevelLookback)
	var supportPoints, resistancePoints []SwingPoint
	for _, swing := range swings {
		if swing.Index < start {
			continue
		}
		if swing.Kind == "low" {
			supportPoints = append(supportPoints, swing)
		}
		if swing.Kind == "high" {
			resistancePoints = append(resistancePoints, swing)
		}
	}
	// clusterLevels intentionally does not truncate to MaxLevels itself: truncating a
	// same-kind (swing-low/swing-high) list before normalizeHorizontalLevelsByPrice
	// reclassifies each cluster as support or resistance relative to the current price
	// would let same-kind candidates crowd out slots before we know which final bucket
	// they belong to (e.g. a stale swing-high cluster now below price, which should
	// count as support). limitLevels below applies the real cap after classification.
	supports := clusterLevels(candles, supportPoints, "horizontal_support", atr)
	resistances := clusterLevels(candles, resistancePoints, "horizontal_resistance", atr)
	supports, resistances = normalizeHorizontalLevelsByPrice(supports, resistances, candles[len(candles)-1].EffectiveClose())
	sort.SliceStable(supports, func(i, j int) bool {
		if supports[i].Strength == supports[j].Strength {
			return supports[i].Price > supports[j].Price
		}
		return supports[i].Strength > supports[j].Strength
	})
	sort.SliceStable(resistances, func(i, j int) bool {
		if resistances[i].Strength == resistances[j].Strength {
			return resistances[i].Price < resistances[j].Price
		}
		return resistances[i].Strength > resistances[j].Strength
	})
	supports = limitLevels(supports, opts.MaxLevels)
	resistances = limitLevels(resistances, opts.MaxLevels)
	return supports, resistances
}

func normalizeHorizontalLevelsByPrice(supports, resistances []Level, current float64) ([]Level, []Level) {
	normalizedSupports := make([]Level, 0, len(supports)+len(resistances))
	normalizedResistances := make([]Level, 0, len(supports)+len(resistances))
	for _, level := range append(append([]Level{}, supports...), resistances...) {
		if level.Price <= 0 {
			continue
		}
		if level.Price <= current {
			level.Type = "horizontal_support"
			normalizedSupports = append(normalizedSupports, level)
			continue
		}
		level.Type = "horizontal_resistance"
		normalizedResistances = append(normalizedResistances, level)
	}
	return dedupeLevels(normalizedSupports), dedupeLevels(normalizedResistances)
}

func dedupeLevels(levels []Level) []Level {
	if len(levels) <= 1 {
		return levels
	}
	sort.SliceStable(levels, func(i, j int) bool {
		if levels[i].Price == levels[j].Price {
			return levels[i].Strength > levels[j].Strength
		}
		return levels[i].Price < levels[j].Price
	})
	out := make([]Level, 0, len(levels))
	for _, level := range levels {
		if len(out) == 0 || math.Abs(out[len(out)-1].Price-level.Price) > math.Max(level.Price*0.001, 0.000001) {
			out = append(out, level)
			continue
		}
		if level.Strength > out[len(out)-1].Strength {
			out[len(out)-1] = level
		}
	}
	return out
}

func limitLevels(levels []Level, limit int) []Level {
	if limit > 0 && len(levels) > limit {
		return levels[:limit]
	}
	return levels
}

type levelCluster struct {
	points []SwingPoint
	price  float64
	seed   float64
}

func clusterLevels(candles []ohlcv.Candle, points []SwingPoint, kind string, atr float64) []Level {
	sort.SliceStable(points, func(i, j int) bool { return points[i].Price < points[j].Price })
	avgVolume := averageVolume(candles)
	clusters := make([]levelCluster, 0)
	for _, point := range points {
		tol := levelTolerance(point.Price, atr)
		merged := false
		for i := range clusters {
			// Membership is tested against the cluster's original seed price, not its
			// continuously-updated weighted mean. Comparing against a drifting mean lets
			// a monotonic run of points (e.g. a sloped sequence of higher lows) chain
			// together transitively, each one within tolerance of the last update but
			// the cluster as a whole spanning far more than `tol` end to end — which
			// then gets reported as one flat level. Anchoring to the seed bounds the
			// cluster's total width to at most 2*tol.
			if math.Abs(point.Price-clusters[i].seed) <= tol {
				clusters[i].points = append(clusters[i].points, point)
				clusters[i].price = weightedLevelPrice(clusters[i].points)
				merged = true
				break
			}
		}
		if !merged {
			clusters = append(clusters, levelCluster{points: []SwingPoint{point}, price: point.Price, seed: point.Price})
		}
	}
	levels := make([]Level, 0, len(clusters))
	for _, cluster := range clusters {
		if len(cluster.points) == 0 {
			continue
		}
		last := cluster.points[0]
		volSum := 0.0
		for _, point := range cluster.points {
			if point.Index > last.Index {
				last = point
			}
			volSum += point.Volume
		}
		touchScore := mathutil.Clamp(float64(len(cluster.points))/5, 0, 1)
		recencyScore := 1 - mathutil.Clamp(float64(len(candles)-1-last.Index)/float64(maxInt(len(candles), 1)), 0, 1)
		volumeScore := mathutil.Clamp(mathutil.SafeDiv(volSum/float64(len(cluster.points)), avgVolume)/2, 0, 1)
		strength := mathutil.Clamp(touchScore*0.45+recencyScore*0.30+volumeScore*0.25, 0, 1)
		levels = append(levels, Level{
			Price:         round2(cluster.price),
			TouchCount:    len(cluster.points),
			Strength:      round4(strength),
			LastTouchDate: last.Time,
			Type:          kind,
		})
	}
	sort.SliceStable(levels, func(i, j int) bool {
		if levels[i].Strength == levels[j].Strength {
			return levels[i].Price < levels[j].Price
		}
		return levels[i].Strength > levels[j].Strength
	})
	return levels
}

func weightedLevelPrice(points []SwingPoint) float64 {
	sum := 0.0
	for _, point := range points {
		sum += point.Price
	}
	return mathutil.SafeDiv(sum, float64(len(points)))
}

func levelTolerance(price, atr float64) float64 {
	return math.Max(atr*0.65, price*0.012)
}

func buildTrendlines(candles []ohlcv.Candle, swings []SwingPoint, atr float64, opts Options) []trendLineCandidate {
	highs := filterSwings(swings, "high")
	lows := filterSwings(swings, "low")
	candidates := make([]trendLineCandidate, 0)
	candidates = append(candidates, trendlineCandidates(candles, lows, "support_trendline", atr, opts)...)
	candidates = append(candidates, trendlineCandidates(candles, highs, "resistance_trendline", atr, opts)...)
	reactionCandidates := recentCounterTrendlines(candles, atr, opts)
	candidates = append(candidates, reactionCandidates...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].line.Strength == candidates[j].line.Strength {
			return trendlineSpan(candidates[i]) > trendlineSpan(candidates[j])
		}
		return candidates[i].line.Strength > candidates[j].line.Strength
	})
	if len(candidates) > 24 {
		candidates = candidates[:24]
	}
	for _, reaction := range reactionCandidates {
		if !containsSimilarTrendline(candidates, reaction) {
			candidates = append(candidates, reaction)
			break
		}
	}
	assignTrendlineNames(candidates)
	return candidates
}

func trendlineCandidates(candles []ohlcv.Candle, points []SwingPoint, kind string, atr float64, opts Options) []trendLineCandidate {
	if len(points) < 2 {
		return nil
	}
	startLimit := maxInt(0, len(points)-opts.TrendlineSwingLimit)
	points = points[startLimit:]
	out := make([]trendLineCandidate, 0)
	current := candles[len(candles)-1].EffectiveClose()
	lengthTarget := math.Max(float64(opts.MinTrendlineSpanBars*4), float64(len(candles))*0.42)
	recencyTarget := math.Max(float64(opts.MinTrendlineSpanBars*2), float64(len(candles))*0.26)
	for i := 0; i < len(points)-1; i++ {
		for j := i + 1; j < len(points); j++ {
			span := points[j].Index - points[i].Index
			if span < opts.MinTrendlineSpanBars {
				continue
			}
			slope := mathutil.SafeDiv(points[j].Price-points[i].Price, float64(span))
			touches, weightedTouches, violations := trendlineTouchStats(candles, points[i].Index, points[i].Price, slope, kind, atr)
			if touches < 2 {
				continue
			}
			lengthScore := mathutil.Clamp(float64(span)/lengthTarget, 0, 1)
			touchScore := mathutil.Clamp(weightedTouches/6, 0, 1)
			violationScore := 1 - mathutil.Clamp(float64(violations)/math.Max(4, float64(touches)+3), 0, 1)
			recencyScore := 1 - mathutil.Clamp(float64(len(candles)-1-points[j].Index)/recencyTarget, 0, 1)
			endIdx := len(candles) - 1
			endPrice := lineValue(points[i].Index, points[i].Price, slope, endIdx)
			proximityScore := 1.0
			if current > 0 && endPrice > 0 {
				proximityScore = 1 - mathutil.Clamp(math.Abs(endPrice-current)/(current*0.85), 0, 1)
			}
			strength := mathutil.Clamp(touchScore*0.34+lengthScore*0.32+violationScore*0.22+recencyScore*0.07+proximityScore*0.05, 0, 1)
			status := trendlineStatus(candles, endPrice, kind, atr)
			out = append(out, trendLineCandidate{
				line: TrendlineResult{
					Type:       kind,
					Slope:      round6(slope),
					TouchCount: touches,
					Strength:   round4(strength),
					Start:      TimePrice{Time: points[i].Time, Price: round2(points[i].Price)},
					End:        TimePrice{Time: candleDate(candles[endIdx]), Price: round2(endPrice)},
					Status:     status,
				},
				startIdx:   points[i].Index,
				endIdx:     endIdx,
				startPrice: points[i].Price,
				endPrice:   endPrice,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].line.Strength == out[j].line.Strength {
			return trendlineSpan(out[i]) > trendlineSpan(out[j])
		}
		return out[i].line.Strength > out[j].line.Strength
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func recentCounterTrendlines(candles []ohlcv.Candle, atr float64, opts Options) []trendLineCandidate {
	if len(candles) < 12 {
		return nil
	}
	window := minInt(len(candles), maxInt(opts.MinTrendlineSpanBars*5, len(candles)/4))
	start := len(candles) - window
	lookback := maxInt(2, opts.PivotLookback/2)
	recent := DetectSwings(candles[start:], lookback)
	for i := range recent {
		recent[i].Index += start
	}
	highs := filterSwings(recent, "high")
	if len(highs) < 2 {
		return nil
	}
	minSpan := maxInt(3, opts.MinTrendlineSpanBars/3)
	maxSpan := maxInt(minSpan+1, opts.MinTrendlineSpanBars*5)
	current := candles[len(candles)-1].EffectiveClose()
	out := make([]trendLineCandidate, 0)
	for i := 0; i < len(highs)-1; i++ {
		for j := i + 1; j < len(highs); j++ {
			span := highs[j].Index - highs[i].Index
			if span < minSpan || span > maxSpan {
				continue
			}
			if highs[j].Price >= highs[i].Price*0.995 {
				continue
			}
			slope := mathutil.SafeDiv(highs[j].Price-highs[i].Price, float64(span))
			if slope >= 0 {
				continue
			}
			endIdx := len(candles) - 1
			endPrice := lineValue(highs[i].Index, highs[i].Price, slope, endIdx)
			if endPrice <= 0 {
				continue
			}
			touches, weightedTouches, violations := trendlineTouchStats(candles, highs[i].Index, highs[i].Price, slope, "resistance_trendline", atr)
			if touches < 2 {
				continue
			}
			proximity := 1.0
			if current > 0 {
				proximity = 1 - mathutil.Clamp(math.Abs(endPrice-current)/(current*0.45), 0, 1)
			}
			recency := 1 - mathutil.Clamp(float64(len(candles)-1-highs[j].Index)/float64(window), 0, 1)
			touchScore := mathutil.Clamp(weightedTouches/5, 0, 1)
			violationScore := 1 - mathutil.Clamp(float64(violations)/math.Max(4, float64(touches)+3), 0, 1)
			spanScore := mathutil.Clamp(float64(span)/float64(maxSpan), 0, 1)
			strength := mathutil.Clamp(touchScore*0.28+violationScore*0.24+proximity*0.24+recency*0.14+spanScore*0.10, 0, 1)
			out = append(out, trendLineCandidate{
				line: TrendlineResult{
					Type:       "resistance_trendline",
					Slope:      round6(slope),
					TouchCount: touches,
					Strength:   round4(strength),
					Start:      TimePrice{Time: highs[i].Time, Price: round2(highs[i].Price)},
					End:        TimePrice{Time: candleDate(candles[endIdx]), Price: round2(endPrice)},
					Status:     trendlineStatus(candles, endPrice, "resistance_trendline", atr),
				},
				startIdx:   highs[i].Index,
				endIdx:     endIdx,
				startPrice: highs[i].Price,
				endPrice:   endPrice,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].line.Strength == out[j].line.Strength {
			return out[i].startIdx > out[j].startIdx
		}
		return out[i].line.Strength > out[j].line.Strength
	})
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func containsSimilarTrendline(candidates []trendLineCandidate, target trendLineCandidate) bool {
	for _, candidate := range candidates {
		if similarTrendline(candidate, target) {
			return true
		}
	}
	return false
}

// trendlineTouchStats returns the accurate touch count (touches, matching the number of
// touch markers collectTouchPoints would draw), a volume-weighted touch score
// (weightedTouches, which counts a volume-confirmed touch as worth more toward
// strength without inflating the reported count), and the violation count.
func trendlineTouchStats(candles []ohlcv.Candle, startIdx int, startPrice, slope float64, kind string, atr float64) (touches int, weightedTouches float64, violations int) {
	tol := trendlineStatsTolerance(startPrice, atr)
	avgVol := averageVolume(candles)
	for i := startIdx; i < len(candles); i++ {
		expected := lineValue(startIdx, startPrice, slope, i)
		if expected <= 0 {
			continue
		}
		switch kind {
		case "support_trendline":
			if math.Abs(candles[i].EffectiveLow()-expected) <= tol {
				touches++
				weightedTouches++
				if avgVol > 0 && candles[i].EffectiveVolume() > avgVol*1.5 {
					weightedTouches++ // volume-confirmed touch weighs double toward strength only
				}
			}
			if candles[i].EffectiveClose() < expected-tol {
				violations++
			}
		case "resistance_trendline":
			if math.Abs(candles[i].EffectiveHigh()-expected) <= tol {
				touches++
				weightedTouches++
				if avgVol > 0 && candles[i].EffectiveVolume() > avgVol*1.5 {
					weightedTouches++
				}
			}
			if candles[i].EffectiveClose() > expected+tol {
				violations++
			}
		}
	}
	return touches, weightedTouches, violations
}

func collectTouchPoints(candles []ohlcv.Candle, startIdx int, startPrice, slope float64, kind string, atr float64) []TimePrice {
	var points []TimePrice
	minGap := maxInt(3, len(candles)/85)
	maxPoints := maxInt(8, minInt(18, len(candles)/18))
	lastTouchIdx := -minGap - 1
	for i := startIdx; i < len(candles); i++ {
		expected := lineValue(startIdx, startPrice, slope, i)
		if expected <= 0 {
			continue
		}
		if i-lastTouchIdx < minGap {
			continue
		}
		tol := trendlineMarkerTolerance(expected, atr)
		touched := false
		switch kind {
		case "support_trendline":
			if math.Abs(candles[i].EffectiveLow()-expected) <= tol {
				touched = true
			}
		case "resistance_trendline":
			if math.Abs(candles[i].EffectiveHigh()-expected) <= tol {
				touched = true
			}
		}
		if !touched {
			continue
		}
		points = append(points, TimePrice{Time: candleDate(candles[i]), Price: round2(expected)})
		lastTouchIdx = i
		if len(points) >= maxPoints {
			break
		}
	}
	return points
}

func trendlineTouchTolerance(price, atr float64) float64 {
	if price <= 0 {
		return 0
	}
	tol := math.Max(price*0.003, 0.000001)
	if atr > 0 {
		tol = math.Max(atr*0.18, price*0.0025)
	}
	return math.Min(tol, price*0.006)
}

func trendlineStatsTolerance(price, atr float64) float64 {
	if price <= 0 {
		return 0
	}
	return math.Max(atr*0.45, price*0.008)
}

func trendlineMarkerTolerance(price, atr float64) float64 {
	if price <= 0 {
		return 0
	}
	return math.Min(trendlineTouchTolerance(price, atr)*0.75, price*0.0045)
}

func trendlineStatus(candles []ohlcv.Candle, currentLinePrice float64, kind string, atr float64) string {
	last := candles[len(candles)-1]
	tol := math.Max(atr*0.4, currentLinePrice*0.006)
	if kind == "support_trendline" && last.EffectiveClose() < currentLinePrice-tol {
		if recentRetest(candles, currentLinePrice, tol, false) {
			return statusRetested
		}
		return statusBroken
	}
	if kind == "resistance_trendline" && last.EffectiveClose() > currentLinePrice+tol {
		if recentRetest(candles, currentLinePrice, tol, true) {
			return statusRetested
		}
		return statusBroken
	}
	return statusActive
}

func recentRetest(candles []ohlcv.Candle, level, tol float64, fromAbove bool) bool {
	start := maxInt(0, len(candles)-5)
	for i := start; i < len(candles); i++ {
		if fromAbove && candles[i].EffectiveLow() <= level+tol && candles[i].EffectiveClose() >= level-tol {
			return true
		}
		if !fromAbove && candles[i].EffectiveHigh() >= level-tol && candles[i].EffectiveClose() <= level+tol {
			return true
		}
	}
	return false
}

func assignTrendlineNames(candidates []trendLineCandidate) {
	supportN := 0
	resistanceN := 0
	for i := range candidates {
		switch candidates[i].line.Type {
		case "support_trendline":
			supportN++
			candidates[i].line.Name = fmt.Sprintf("support_trendline_%d", supportN)
		case "resistance_trendline":
			resistanceN++
			candidates[i].line.Name = fmt.Sprintf("resistance_trendline_%d", resistanceN)
		}
	}
}

func detectPatterns(candles []ohlcv.Candle, swings []SwingPoint, supports, resistances []Level, trendlines []trendLineCandidate, atr float64, opts Options) []PatternResult {
	patterns := make([]PatternResult, 0)
	if channel, ok := detectChannel(candles, supports, resistances, trendlines, atr); ok {
		patterns = append(patterns, channel)
	}
	if triangle, ok := detectTriangle(candles, supports, resistances, trendlines, atr); ok {
		patterns = append(patterns, triangle)
	}
	if wedge, ok := detectWedge(candles, trendlines, atr); ok {
		patterns = append(patterns, wedge)
	}
	if flag, ok := detectFlagOrPennant(candles, trendlines, atr); ok {
		patterns = append(patterns, flag)
	}
	if consolidation, ok := detectConsolidation(candles, supports, resistances, atr, opts.ConsolidationWindow); ok {
		patterns = append(patterns, consolidation)
	}
	sort.SliceStable(patterns, func(i, j int) bool {
		return patterns[i].Confidence > patterns[j].Confidence
	})
	return patterns
}

func detectChannel(candles []ohlcv.Candle, supports, resistances []Level, trendlines []trendLineCandidate, atr float64) (PatternResult, bool) {
	lastIdx := len(candles) - 1
	// Prefer a true sloped channel only when both boundaries have ideal 3+ touches.
	for _, lowLine := range trendlines {
		if lowLine.line.Type != "support_trendline" || lowLine.line.TouchCount < 3 {
			continue
		}
		for _, highLine := range trendlines {
			if highLine.line.Type != "resistance_trendline" || highLine.line.TouchCount < 3 {
				continue
			}
			if !parallelSlopes(lowLine.line.Slope, highLine.line.Slope) {
				continue
			}
			kind := "horizontal_channel"
			category := "channel"
			if lowLine.line.Slope > slopeFlatThreshold(candles) && highLine.line.Slope > slopeFlatThreshold(candles) {
				kind = "ascending_channel"
			}
			if lowLine.line.Slope < -slopeFlatThreshold(candles) && highLine.line.Slope < -slopeFlatThreshold(candles) {
				kind = "descending_channel"
			}
			upperNow := highLine.line.End.Price
			lowerNow := lowLine.line.End.Price
			if upperNow <= lowerNow {
				continue
			}
			status := containmentStatus(candles[lastIdx].EffectiveClose(), lowerNow, upperNow, atr)
			confidence := mathutil.Clamp(0.52+float64(lowLine.line.TouchCount+highLine.line.TouchCount)*0.055, 0, 0.86)
			return patternFromLines(kind, category, confidence, status, candles, highLine, lowLine, lowerNow, upperNow), true
		}
	}
	// If sloped trendline quality is insufficient, a horizontal channel can still be valid from clustered OHLCV levels.
	if len(supports) == 0 || len(resistances) == 0 {
		return PatternResult{}, false
	}
	var support *Level
	var resistance *Level
	current := candles[lastIdx].EffectiveClose()
	for i := range supports {
		if supports[i].TouchCount >= 3 && supports[i].Price < current+atr*2 {
			support = &supports[i]
			break
		}
	}
	for i := range resistances {
		if resistances[i].TouchCount >= 3 && resistances[i].Price > current-atr*2 {
			resistance = &resistances[i]
			break
		}
	}
	if support == nil || resistance == nil || resistance.Price <= support.Price {
		return PatternResult{}, false
	}
	status := containmentStatus(current, support.Price, resistance.Price, atr)
	confidence := mathutil.Clamp(0.46+float64(support.TouchCount+resistance.TouchCount)*0.045+(support.Strength+resistance.Strength)*0.12, 0, 0.84)
	start := maxInt(0, len(candles)-120)
	return PatternResult{
		Name:           "horizontal_channel",
		Category:       "channel",
		Confidence:     round4(confidence),
		Status:         status,
		StartDate:      candleDate(candles[start]),
		EndDate:        candleDate(candles[lastIdx]),
		UpperLine:      horizontalLine(candles[start], candles[lastIdx], resistance.Price),
		LowerLine:      horizontalLine(candles[start], candles[lastIdx], support.Price),
		MainSupport:    round2(support.Price),
		MainResistance: round2(resistance.Price),
		BreakoutLevel:  round2(resistance.Price),
		BreakdownLevel: round2(support.Price),
		Targets:        nextTargets(resistances, resistance.Price, true),
		InvalidLevel:   round2(support.Price),
	}, true
}

func detectTriangle(candles []ohlcv.Candle, supports, resistances []Level, trendlines []trendLineCandidate, atr float64) (PatternResult, bool) {
	upper, lower, name, ok := bestTriangleTrendlinePair(candles, trendlines)
	if !ok {
		return PatternResult{}, false
	}
	startDistance := lineValue(upper.startIdx, upper.startPrice, upper.line.Slope, maxInt(upper.startIdx, lower.startIdx)) -
		lineValue(lower.startIdx, lower.startPrice, lower.line.Slope, maxInt(upper.startIdx, lower.startIdx))
	endDistance := upper.line.End.Price - lower.line.End.Price
	if startDistance <= 0 || endDistance <= 0 || endDistance > startDistance*0.82 {
		return PatternResult{}, false
	}
	status := patternBreakStatus(candles, lower.line.End.Price, upper.line.End.Price, atr)
	// Measured-move height uses the triangle's width at its widest (start) point,
	// evaluated at a common bar index (startDistance, computed above) rather than each
	// trendline's own unrelated start bar — the two candidate trendlines are selected
	// independently and their raw Start.Price fields are not a valid vertical
	// cross-section of the pattern.
	height := startDistance
	targets := []float64{round2(upper.line.End.Price + height*0.5), round2(upper.line.End.Price + height)}
	confidence := mathutil.Clamp(0.44+float64(upper.line.TouchCount+lower.line.TouchCount)*0.055+(1-mathutil.SafeDiv(endDistance, startDistance))*0.25, 0, 0.88)
	result := patternFromLines(name, "triangle", confidence, status, candles, *upper, *lower, lower.line.End.Price, upper.line.End.Price)
	result.Targets = targets
	result.InvalidLevel = round2(lower.line.End.Price)
	if len(supports) > 0 {
		result.MainSupport = supports[0].Price
	}
	if len(resistances) > 0 {
		result.MainResistance = resistances[0].Price
	}
	return result, true
}

func detectWedge(candles []ohlcv.Candle, trendlines []trendLineCandidate, atr float64) (PatternResult, bool) {
	upper, lower, ok := bestWedgeTrendlinePair(candles, trendlines)
	if !ok {
		return PatternResult{}, false
	}
	if upper.line.Slope*lower.line.Slope <= 0 {
		return PatternResult{}, false
	}
	// upper/lower are independently selected trendline candidates, each anchored at its
	// own start bar — evaluate both lines at a shared index (like the triangle path
	// does) rather than subtracting their unrelated Start.Price fields directly.
	wedgeStartIdx := maxInt(upper.startIdx, lower.startIdx)
	startDistance := lineValue(upper.startIdx, upper.startPrice, upper.line.Slope, wedgeStartIdx) -
		lineValue(lower.startIdx, lower.startPrice, lower.line.Slope, wedgeStartIdx)
	endDistance := upper.line.End.Price - lower.line.End.Price
	if startDistance <= 0 || endDistance <= 0 || endDistance > startDistance*0.78 {
		return PatternResult{}, false
	}
	name := "falling_wedge"
	if upper.line.Slope > 0 && lower.line.Slope > 0 {
		name = "rising_wedge"
	}
	status := patternBreakStatus(candles, lower.line.End.Price, upper.line.End.Price, atr)
	confidence := mathutil.Clamp(0.42+float64(upper.line.TouchCount+lower.line.TouchCount)*0.045+(1-mathutil.SafeDiv(endDistance, startDistance))*0.22, 0, 0.80)
	return patternFromLines(name, "wedge", confidence, status, candles, *upper, *lower, lower.line.End.Price, upper.line.End.Price), true
}

func bestTriangleTrendlinePair(candles []ohlcv.Candle, trendlines []trendLineCandidate) (*trendLineCandidate, *trendLineCandidate, string, bool) {
	flat := slopeFlatThreshold(candles)
	bestScore := -1.0
	var bestUpper trendLineCandidate
	var bestLower trendLineCandidate
	bestName := ""
	for i := range trendlines {
		upper := trendlines[i]
		if upper.line.Type != "resistance_trendline" {
			continue
		}
		for j := range trendlines {
			lower := trendlines[j]
			if lower.line.Type != "support_trendline" {
				continue
			}
			name := trianglePatternName(upper.line.Slope, lower.line.Slope, flat)
			if name == "" {
				continue
			}
			startIdx := maxInt(upper.startIdx, lower.startIdx)
			startDistance := lineValue(upper.startIdx, upper.startPrice, upper.line.Slope, startIdx) -
				lineValue(lower.startIdx, lower.startPrice, lower.line.Slope, startIdx)
			endDistance := upper.line.End.Price - lower.line.End.Price
			if startDistance <= 0 || endDistance <= 0 || endDistance > startDistance*0.82 {
				continue
			}
			convergenceScore := 1 - mathutil.SafeDiv(endDistance, startDistance)
			touchScore := mathutil.Clamp(float64(upper.line.TouchCount+lower.line.TouchCount)/10, 0, 1)
			strengthScore := (upper.line.Strength + lower.line.Strength) / 2
			spanScore := mathutil.Clamp(float64(minInt(trendlineSpan(upper), trendlineSpan(lower)))/math.Max(1, float64(len(candles))), 0, 1)
			score := convergenceScore*0.42 + strengthScore*0.28 + touchScore*0.18 + spanScore*0.12
			if score > bestScore {
				bestScore = score
				bestUpper = upper
				bestLower = lower
				bestName = name
			}
		}
	}
	if bestScore < 0 {
		return nil, nil, "", false
	}
	return &bestUpper, &bestLower, bestName, true
}

func trianglePatternName(upperSlope, lowerSlope, flat float64) string {
	switch {
	case upperSlope < -flat && lowerSlope > flat:
		return "symmetrical_triangle"
	case math.Abs(upperSlope) <= flat && lowerSlope > flat:
		return "ascending_triangle"
	case upperSlope < -flat && math.Abs(lowerSlope) <= flat:
		return "descending_triangle"
	default:
		return ""
	}
}

func bestWedgeTrendlinePair(candles []ohlcv.Candle, trendlines []trendLineCandidate) (*trendLineCandidate, *trendLineCandidate, bool) {
	bestScore := -1.0
	var bestUpper trendLineCandidate
	var bestLower trendLineCandidate
	for i := range trendlines {
		upper := trendlines[i]
		if upper.line.Type != "resistance_trendline" {
			continue
		}
		for j := range trendlines {
			lower := trendlines[j]
			if lower.line.Type != "support_trendline" || upper.line.Slope*lower.line.Slope <= 0 {
				continue
			}
			startIdx := maxInt(upper.startIdx, lower.startIdx)
			startDistance := lineValue(upper.startIdx, upper.startPrice, upper.line.Slope, startIdx) -
				lineValue(lower.startIdx, lower.startPrice, lower.line.Slope, startIdx)
			endDistance := upper.line.End.Price - lower.line.End.Price
			if startDistance <= 0 || endDistance <= 0 || endDistance > startDistance*0.78 {
				continue
			}
			convergenceScore := 1 - mathutil.SafeDiv(endDistance, startDistance)
			touchScore := mathutil.Clamp(float64(upper.line.TouchCount+lower.line.TouchCount)/10, 0, 1)
			strengthScore := (upper.line.Strength + lower.line.Strength) / 2
			spanScore := mathutil.Clamp(float64(minInt(trendlineSpan(upper), trendlineSpan(lower)))/math.Max(1, float64(len(candles))), 0, 1)
			score := convergenceScore*0.42 + strengthScore*0.28 + touchScore*0.18 + spanScore*0.12
			if score > bestScore {
				bestScore = score
				bestUpper = upper
				bestLower = lower
			}
		}
	}
	if bestScore < 0 {
		return nil, nil, false
	}
	return &bestUpper, &bestLower, true
}

func detectFlagOrPennant(candles []ohlcv.Candle, trendlines []trendLineCandidate, atr float64) (PatternResult, bool) {
	if len(candles) < 45 || atr <= 0 {
		return PatternResult{}, false
	}
	impulseStart := len(candles) - 35
	impulseEnd := len(candles) - 16
	if impulseStart < 0 {
		return PatternResult{}, false
	}
	impulse := candles[impulseEnd].EffectiveClose() - candles[impulseStart].EffectiveClose()
	if math.Abs(impulse) < atr*3 {
		return PatternResult{}, false
	}
	recentStart := len(candles) - 15
	rangeHigh, rangeLow := highLow(candles[recentStart:])
	if rangeHigh-rangeLow > math.Abs(impulse)*0.55 {
		return PatternResult{}, false
	}
	upper := bestTrendlineByType(trendlines, "resistance_trendline")
	lower := bestTrendlineByType(trendlines, "support_trendline")
	if upper == nil || lower == nil {
		return PatternResult{}, false
	}
	name := "bullish_flag"
	if impulse < 0 {
		name = "bearish_flag"
	}
	if upper.line.Slope*lower.line.Slope < 0 {
		if impulse > 0 {
			name = "bullish_pennant"
		} else {
			name = "bearish_pennant"
		}
	}
	status := patternBreakStatus(candles, lower.line.End.Price, upper.line.End.Price, atr)
	confidence := mathutil.Clamp(0.42+math.Abs(impulse)/(atr*20), 0, 0.76)
	return patternFromLines(name, "flag", confidence, status, candles, *upper, *lower, lower.line.End.Price, upper.line.End.Price), true
}

func detectConsolidation(candles []ohlcv.Candle, supports, resistances []Level, atr float64, window int) (PatternResult, bool) {
	if len(candles) < window*2 || window < 10 {
		return PatternResult{}, false
	}
	recent := candles[len(candles)-window:]
	prior := candles[len(candles)-window*2 : len(candles)-window]
	recentRange := averageRange(recent)
	priorRange := averageRange(prior)
	recentHigh, recentLow := highLow(recent)
	priorHigh, priorLow := highLow(prior)
	recentPctRange := mathutil.SafeDiv(recentHigh-recentLow, recentLow)
	priorPctRange := mathutil.SafeDiv(priorHigh-priorLow, priorLow)
	if !(recentRange < priorRange*0.82 || recentPctRange < priorPctRange*0.75 || recentRange < atr*0.85) {
		return PatternResult{}, false
	}
	start := len(candles) - window
	upper := recentHigh
	lower := recentLow
	if len(resistances) > 0 && math.Abs(resistances[0].Price-recentHigh) <= levelTolerance(recentHigh, atr)*2 {
		upper = resistances[0].Price
	}
	if len(supports) > 0 && math.Abs(supports[0].Price-recentLow) <= levelTolerance(recentLow, atr)*2 {
		lower = supports[0].Price
	}
	confidence := mathutil.Clamp(0.45+(1-mathutil.SafeDiv(recentRange, priorRange))*0.35, 0, 0.78)
	return PatternResult{
		Name:           "consolidation",
		Category:       "consolidation",
		Confidence:     round4(confidence),
		Status:         "forming",
		StartDate:      candleDate(candles[start]),
		EndDate:        candleDate(candles[len(candles)-1]),
		UpperLine:      horizontalLine(candles[start], candles[len(candles)-1], upper),
		LowerLine:      horizontalLine(candles[start], candles[len(candles)-1], lower),
		MainSupport:    round2(lower),
		MainResistance: round2(upper),
		BreakoutLevel:  round2(upper),
		BreakdownLevel: round2(lower),
		Targets:        []float64{round2(upper), round2(lower)},
		InvalidLevel:   round2(lower),
	}, true
}

func detectBreakout(candles []ohlcv.Candle, supports, resistances []Level, atr float64) BreakoutAnalysis {
	if len(candles) < 2 {
		return BreakoutAnalysis{Status: "none"}
	}
	last := len(candles) - 1
	current := candles[last].EffectiveClose()
	prev := candles[last-1].EffectiveClose()
	volSMA := averageRecentVolume(candles, 20)
	tol := math.Max(atr*0.35, current*0.006)
	for _, resistance := range resistances {
		if prev <= resistance.Price+tol && current > resistance.Price+tol {
			return BreakoutAnalysis{
				Status:             fakeBreakoutStatus(candles, resistance.Price, true, tol),
				Level:              round2(resistance.Price),
				CloseConfirmation:  true,
				VolumeConfirmation: candles[last].EffectiveVolume() > volSMA,
				RetestDetected:     recentRetest(candles, resistance.Price, tol, true),
			}
		}
	}
	for _, support := range supports {
		if prev >= support.Price-tol && current < support.Price-tol {
			return BreakoutAnalysis{
				Status:             fakeBreakoutStatus(candles, support.Price, false, tol),
				Level:              round2(support.Price),
				CloseConfirmation:  true,
				VolumeConfirmation: candles[last].EffectiveVolume() > volSMA,
				RetestDetected:     recentRetest(candles, support.Price, tol, false),
			}
		}
	}
	if fake := detectRecentFake(candles, supports, resistances, tol); fake.Status != "" {
		return fake
	}
	return BreakoutAnalysis{Status: "none", CloseConfirmation: false, VolumeConfirmation: false, RetestDetected: false}
}

func fakeBreakoutStatus(candles []ohlcv.Candle, level float64, bullish bool, tol float64) string {
	current := candles[len(candles)-1].EffectiveClose()
	if bullish && current > level+tol {
		return "bullish_breakout"
	}
	if !bullish && current < level-tol {
		return "bearish_breakdown"
	}
	return "none"
}

func detectRecentFake(candles []ohlcv.Candle, supports, resistances []Level, tol float64) BreakoutAnalysis {
	last := len(candles) - 1
	start := maxInt(0, last-5)
	current := candles[last].EffectiveClose()
	volSMA := averageRecentVolume(candles, 20)
	for _, resistance := range resistances {
		for i := start; i <= last; i++ {
			if candles[i].EffectiveHigh() > resistance.Price+tol && candles[i].EffectiveClose() <= resistance.Price+tol && current < resistance.Price+tol {
				return BreakoutAnalysis{Status: "fake_breakout", Level: round2(resistance.Price), CloseConfirmation: false, VolumeConfirmation: candles[i].EffectiveVolume() > volSMA, RetestDetected: true}
			}
		}
	}
	for _, support := range supports {
		for i := start; i <= last; i++ {
			if candles[i].EffectiveLow() < support.Price-tol && candles[i].EffectiveClose() >= support.Price-tol && current > support.Price-tol {
				return BreakoutAnalysis{Status: "fake_breakdown", Level: round2(support.Price), CloseConfirmation: false, VolumeConfirmation: candles[i].EffectiveVolume() > volSMA, RetestDetected: true}
			}
		}
	}
	return BreakoutAnalysis{}
}

func detectTrend(candles []ohlcv.Candle, ema50 []float64, atr float64) TrendSummary {
	primarySlope := regressionSlope(candles, minInt(100, len(candles)))
	secondarySlope := regressionSlope(candles, minInt(30, len(candles)))
	threshold := math.Max(atr*0.04, candles[len(candles)-1].EffectiveClose()*0.0007)
	primary := classifySlope(primarySlope, threshold)
	secondary := classifySlope(secondarySlope, threshold)
	if len(ema50) > 0 {
		last := candles[len(candles)-1].EffectiveClose()
		if last > ema50[len(ema50)-1] && primary == "downtrend" {
			primary = "sideways"
		}
		if last < ema50[len(ema50)-1] && primary == "uptrend" {
			primary = "sideways"
		}
	}
	confidence := mathutil.Clamp(math.Abs(primarySlope)/(threshold*4), 0, 1)
	return TrendSummary{Primary: primary, Secondary: secondary, Confidence: round4(confidence)}
}

func movingAverageSummary(current float64, ema20, ema50 []float64, atr float64) MovingAverages {
	ema20Current := lastValue(ema20)
	ema50Current := lastValue(ema50)
	signal := "neutral"
	if current > ema20Current && ema20Current > ema50Current {
		signal = "bullish"
	}
	if current < ema20Current && ema20Current < ema50Current {
		signal = "bearish"
	}
	return MovingAverages{
		EMA20:  MASummary{Current: round2(ema20Current), Position: maPosition(current, ema20Current), Slope: maSlope(ema20, atr)},
		EMA50:  MASummary{Current: round2(ema50Current), Position: maPosition(current, ema50Current), Slope: maSlope(ema50, atr)},
		Signal: signal,
	}
}

func buildScenarios(candles []ohlcv.Candle, current float64, supports, resistances []Level, trendlines []trendLineCandidate, patterns []PatternResult, ma MovingAverages) []Scenario {
	lastTime := candleDate(candles[len(candles)-1])
	mainSupport := nearestSupport(supports, current)
	mainResistance := nearestResistance(resistances, current)
	var nextResistance float64
	if mainResistance != nil {
		nextResistance = mainResistance.Price
	}
	var supportPrice float64
	if mainSupport != nil {
		supportPrice = mainSupport.Price
	}
	if supportPrice == 0 {
		supportPrice = current * 0.97
	}
	targets := nextResistanceLevels(resistances, current, 2)
	if len(targets) == 0 && nextResistance > 0 {
		targets = []float64{nextResistance}
	}
	score := 0.5
	if ma.Signal == "bullish" {
		score += 0.12
	}
	if ma.Signal == "bearish" {
		score -= 0.10
	}
	if mainSupport != nil && mathutil.SafeDiv(math.Abs(current-mainSupport.Price), current) < 0.03 {
		score += 0.07
	}
	scenarios := make([]Scenario, 0, 3)
	if mainSupport != nil {
		bullishPath := []TimePrice{
			{Time: lastTime, Price: round2(current)},
			futurePoint(candles, 5, maxFloat(current, supportPrice)),
			futurePoint(candles, 10, firstOr(targets, current*1.04)),
		}
		scenarios = append(scenarios, Scenario{
			Name:             "bullish_support_reaction",
			Condition:        "Ana destek üzerinde tutunma ve EMA20 üzerine kapanış",
			ProbabilityScore: round4(mathutil.Clamp(score, 0, 1)),
			TargetLevels:     targets,
			InvalidLevel:     round2(supportPrice),
			PathPoints:       bullishPath,
		})
	}
	if len(targets) > 0 || mainResistance != nil {
		scenarios = append(scenarios, Scenario{
			Name:             "bullish_breakout",
			Condition:        "Üst direnç hacimli kırılırsa bir sonraki direnç hedeflenebilir",
			ProbabilityScore: round4(mathutil.Clamp(score-0.08, 0, 1)),
			TargetLevels:     nextResistanceLevels(resistances, firstOr(targets, current), 2),
			InvalidLevel:     round2(firstOr(targets, supportPrice)),
			PathPoints: []TimePrice{
				{Time: lastTime, Price: round2(current)},
				futurePoint(candles, 8, firstOr(targets, current*1.04)),
				futurePoint(candles, 16, firstOr(nextResistanceLevels(resistances, firstOr(targets, current), 1), current*1.08)),
			},
		})
	}
	if mainSupport != nil {
		bearTargets := lowerSupportLevels(supports, current, 2)
		if len(bearTargets) == 0 {
			bearTargets = []float64{round2(supportPrice * 0.96)}
		}
		scenarios = append(scenarios, Scenario{
			Name:             "bearish_support_loss",
			Condition:        "Ana destek altında kapanış",
			ProbabilityScore: round4(mathutil.Clamp(1-score, 0, 1)),
			TargetLevels:     bearTargets,
			InvalidLevel:     round2(supportPrice),
			PathPoints: []TimePrice{
				{Time: lastTime, Price: round2(current)},
				futurePoint(candles, 5, supportPrice),
				futurePoint(candles, 12, firstOr(bearTargets, supportPrice*0.96)),
			},
		})
	}
	return scenarios
}

func buildDrawingObjects(candles []ohlcv.Candle, supports, resistances []Level, trendlines []trendLineCandidate, patterns []PatternResult, scenarios []Scenario, opts Options) DrawingObjects {
	lines := make([]LineObject, 0)
	labels := make([]LabelObject, 0)
	fills := make([]FillBand, 0)
	var touchPoints []TimePrice
	lastTime := candleDate(candles[len(candles)-1])
	for i, support := range supports {
		if i >= 3 {
			break
		}
		id := fmt.Sprintf("support_%d", i+1)
		width := 2
		label := "Support"
		if i == 0 {
			id = "main_support"
			width = 4
			label = "Main Support"
		}
		lines = append(lines, LineObject{ID: id, Type: "horizontal", Color: "yellow", Width: width, Style: "solid", Price: support.Price, Label: label})
		labels = append(labels, LabelObject{Text: fmt.Sprintf("Support %.2f", support.Price), Time: lastTime, Price: support.Price})
	}
	for i, resistance := range resistances {
		if i >= 3 {
			break
		}
		lines = append(lines, LineObject{ID: fmt.Sprintf("resistance_%d", i+1), Type: "horizontal", Color: "red", Width: 2, Style: "dashed", Price: resistance.Price, Label: "Resistance"})
		labels = append(labels, LabelObject{Text: fmt.Sprintf("Resistance %.2f", resistance.Price), Time: lastTime, Price: resistance.Price})
	}
	selectedTrendlines := selectDrawingTrendlines(candles, trendlines, opts)
	for _, drawing := range selectedTrendlines {
		trendline := drawing.candidate
		lines = append(lines, LineObject{
			ID:         trendline.line.Name,
			Type:       "trendline",
			Color:      drawing.color,
			Width:      drawing.width,
			Style:      drawing.style,
			StartTime:  trendline.line.Start.Time,
			StartPrice: trendline.line.Start.Price,
			EndTime:    trendline.line.End.Time,
			EndPrice:   trendline.line.End.Price,
			Label:      drawing.label,
		})
	}
	// Collect touch points from the best support and resistance trendlines
	atrVal := 0.0
	if len(candles) > 0 {
		atrSeries := indicators.ATRSeries(candles, 14)
		if len(atrSeries) > 0 {
			atrVal = atrSeries[len(atrSeries)-1]
		}
	}
	for _, td := range selectedTrendlines {
		pts := collectTouchPoints(candles, td.candidate.startIdx, td.candidate.startPrice, td.candidate.line.Slope, td.candidate.line.Type, atrVal)
		touchPoints = appendUniqueTouchPoints(touchPoints, pts)
	}

	for _, pattern := range patterns {
		if pattern.UpperLine.Start.Time == "" && pattern.LowerLine.Start.Time == "" {
			continue
		}
		upperColor, lowerColor, lineStyle, lineWidth := patternLineStyle(pattern)
		if pattern.UpperLine.Start.Time != "" {
			lines = append(lines, LineObject{
				ID:         pattern.Name + "_upper",
				Type:       "trendline",
				Color:      upperColor,
				Width:      lineWidth,
				Style:      lineStyle,
				StartTime:  pattern.UpperLine.Start.Time,
				StartPrice: pattern.UpperLine.Start.Price,
				EndTime:    pattern.UpperLine.End.Time,
				EndPrice:   pattern.UpperLine.End.Price,
				Label:      pattern.Name + " upper",
			})
		}
		if pattern.LowerLine.Start.Time != "" {
			lines = append(lines, LineObject{
				ID:         pattern.Name + "_lower",
				Type:       "trendline",
				Color:      lowerColor,
				Width:      lineWidth,
				Style:      lineStyle,
				StartTime:  pattern.LowerLine.Start.Time,
				StartPrice: pattern.LowerLine.Start.Price,
				EndTime:    pattern.LowerLine.End.Time,
				EndPrice:   pattern.LowerLine.End.Price,
				Label:      pattern.Name + " lower",
			})
		}
		// Add shaded fill band for channel patterns
		if pattern.Category == "channel" && pattern.UpperLine.Start.Time != "" && pattern.LowerLine.Start.Time != "" {
			fillColor, fillOpacity := channelFillStyle(pattern)
			fills = append(fills, FillBand{
				ID:              pattern.Name + "_fill",
				Color:           fillColor,
				Opacity:         fillOpacity,
				UpperStartTime:  pattern.UpperLine.Start.Time,
				UpperStartPrice: pattern.UpperLine.Start.Price,
				UpperEndTime:    pattern.UpperLine.End.Time,
				UpperEndPrice:   pattern.UpperLine.End.Price,
				LowerStartTime:  pattern.LowerLine.Start.Time,
				LowerStartPrice: pattern.LowerLine.Start.Price,
				LowerEndTime:    pattern.LowerLine.End.Time,
				LowerEndPrice:   pattern.LowerLine.End.Price,
			})
		}
	}
	paths := make([]PathObject, 0, len(scenarios))
	if drawScenarioPathsForTimeframe(opts.Timeframe) {
		for _, scenario := range scenarios {
			color := "cyan"
			if strings.Contains(scenario.Name, "bearish") {
				color = "red"
			}
			paths = append(paths, PathObject{ID: scenario.Name + "_path", Type: "scenario_path", Color: color, Width: 3, Style: "solid", Points: scenario.PathPoints, Label: scenario.Name})
		}
	}
	return DrawingObjects{Lines: dedupeLines(lines), Paths: paths, Labels: labels, Fills: fills, TouchPoints: touchPoints}
}

func drawScenarioPathsForTimeframe(timeframe string) bool {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1M", "M", "3M", "6M", "1Y", "YTD", "ALL":
		return false
	default:
		return true
	}
}

func channelFillStyle(p PatternResult) (color string, opacity uint8) {
	switch p.Name {
	case "ascending_channel":
		return "#2196F3", 28 // blue, very transparent
	case "descending_channel":
		return "#FF5722", 28 // orange-red
	default:
		return "#9E9E9E", 22 // gray
	}
}

func trendlineDrawingColor(candidate trendLineCandidate) string {
	if candidate.line.Type == "resistance_trendline" {
		return "red"
	}
	if candidate.line.Type == "support_trendline" {
		return "yellow"
	}
	return "yellow"
}

func appendUniqueTouchPoints(existing []TimePrice, incoming []TimePrice) []TimePrice {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, point := range existing {
		seen[touchPointKey(point)] = struct{}{}
	}
	for _, point := range incoming {
		key := touchPointKey(point)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		existing = append(existing, point)
	}
	return existing
}

func touchPointKey(point TimePrice) string {
	return fmt.Sprintf("%s:%.2f", point.Time, point.Price)
}

type drawingTrendline struct {
	candidate trendLineCandidate
	color     string
	width     int
	style     string
	label     string
}

func selectDrawingTrendlines(candles []ohlcv.Candle, trendlines []trendLineCandidate, opts Options) []drawingTrendline {
	supports := filterTrendlineCandidates(trendlines, "support_trendline")
	resistances := filterTrendlineCandidates(trendlines, "resistance_trendline")
	selected := make([]drawingTrendline, 0, 4)
	current := candles[len(candles)-1].EffectiveClose()

	if support, ok := bestMainTrendline(candles, supports, opts); ok {
		label, color := trendlineDisplay(support, current, "main_support")
		selected = appendDrawingTrendline(selected, drawingTrendline{
			candidate: support,
			color:     color,
			width:     4,
			style:     "solid",
			label:     label,
		})
		if resistance, ok := bestChannelMate(candles, support, resistances, opts); ok {
			label, color := trendlineDisplay(resistance, current, "main_resistance")
			selected = appendDrawingTrendline(selected, drawingTrendline{
				candidate: resistance,
				color:     color,
				width:     4,
				style:     "solid",
				label:     label,
			})
		} else if resistance, ok := parallelEnvelopeTrendline(candles, support, "upper"); ok {
			label, color := trendlineDisplay(resistance, current, "main_resistance")
			selected = appendDrawingTrendline(selected, drawingTrendline{
				candidate: resistance,
				color:     color,
				width:     4,
				style:     "solid",
				label:     label,
			})
		}
	}
	if len(selected) == 0 {
		if resistance, ok := bestMainTrendline(candles, resistances, opts); ok {
			label, color := trendlineDisplay(resistance, current, "main_resistance")
			selected = appendDrawingTrendline(selected, drawingTrendline{
				candidate: resistance,
				color:     color,
				width:     4,
				style:     "solid",
				label:     label,
			})
		}
	}
	if reaction, ok := bestRecentReactionTrendline(candles, resistances, opts); ok {
		label, color := trendlineDisplay(reaction, current, "reaction_resistance")
		selected = appendDrawingTrendline(selected, drawingTrendline{
			candidate: reaction,
			color:     color,
			width:     3,
			style:     "solid",
			label:     label,
		})
		if reactionSupport, ok := bestChannelMate(candles, reaction, supports, opts); ok {
			label, color := trendlineDisplay(reactionSupport, current, "reaction_support")
			selected = appendDrawingTrendline(selected, drawingTrendline{
				candidate: reactionSupport,
				color:     color,
				width:     3,
				style:     "solid",
				label:     label,
			})
		} else if reactionSupport, ok := parallelEnvelopeTrendline(candles, reaction, "lower"); ok {
			label, color := trendlineDisplay(reactionSupport, current, "reaction_support")
			selected = appendDrawingTrendline(selected, drawingTrendline{
				candidate: reactionSupport,
				color:     color,
				width:     3,
				style:     "solid",
				label:     label,
			})
		}
	}
	if len(selected) < 4 {
		for _, candidate := range trendlines {
			label, color := trendlineDisplay(candidate, current, "fallback")
			selected = appendDrawingTrendline(selected, drawingTrendline{
				candidate: candidate,
				color:     color,
				width:     3,
				style:     "solid",
				label:     label,
			})
			if len(selected) >= 4 {
				break
			}
		}
	}
	return selected
}

func trendlineDisplay(candidate trendLineCandidate, current float64, role string) (label, color string) {
	above := current > 0 && candidate.endPrice > current+trendlineDisplayBuffer(current)
	below := current > 0 && candidate.endPrice < current-trendlineDisplayBuffer(current)
	switch role {
	case "main_support":
		if above {
			return "Ana trend geri kazanım direnci", "red"
		}
		return "Uzun vadeli yükselen destek", "yellow"
	case "main_resistance":
		if below {
			return "Aşılan ana trend çizgisi", "yellow"
		}
		return "Ana trend direnci", "red"
	case "reaction_resistance":
		if below {
			return "Aşılan kısa düşen trend", "yellow"
		}
		return "Kısa düşen trend direnci", "red"
	case "reaction_support":
		if above {
			return "Kısa kanal geri kazanım direnci", "red"
		}
		return "Kısa kanal alt desteği", "yellow"
	}
	if candidate.line.Type == "support_trendline" {
		if above {
			return "Kaybedilen trend geri kazanım direnci", "red"
		}
		if below {
			return "Trend destek çizgisi", "yellow"
		}
		return "Test edilen trend desteği", "yellow"
	}
	if candidate.line.Type == "resistance_trendline" {
		if below {
			return "Aşılan trend çizgisi", "yellow"
		}
		return "Trend direnci", "red"
	}
	return "Trend çizgisi", trendlineDrawingColor(candidate)
}

func trendlineDisplayBuffer(current float64) float64 {
	return math.Max(math.Abs(current)*0.001, 0.01)
}

func parallelEnvelopeTrendline(candles []ohlcv.Candle, anchor trendLineCandidate, side string) (trendLineCandidate, bool) {
	if len(candles) == 0 || anchor.endIdx <= anchor.startIdx {
		return trendLineCandidate{}, false
	}
	startIdx := anchor.startIdx
	endIdx := len(candles) - 1
	slope := mathutil.SafeDiv(anchor.endPrice-anchor.startPrice, float64(anchor.endIdx-anchor.startIdx))
	if slope == 0 {
		slope = anchor.line.Slope
	}
	residuals := make([]float64, 0, endIdx-startIdx+1)
	for i := startIdx; i <= endIdx; i++ {
		base := lineValue(anchor.startIdx, anchor.startPrice, slope, i)
		switch side {
		case "upper":
			if delta := candles[i].EffectiveHigh() - base; delta > 0 {
				residuals = append(residuals, delta)
			}
		case "lower":
			if delta := base - candles[i].EffectiveLow(); delta > 0 {
				residuals = append(residuals, delta)
			}
		}
	}
	if len(residuals) == 0 {
		return trendLineCandidate{}, false
	}
	offset := envelopeOffset(residuals)
	current := candles[endIdx].EffectiveClose()
	startBase := lineValue(anchor.startIdx, anchor.startPrice, slope, startIdx)
	endBase := lineValue(anchor.startIdx, anchor.startPrice, slope, endIdx)
	if current > 0 && offset/current > 0.65 {
		return trendLineCandidate{}, false
	}
	typ := "resistance_trendline"
	startPrice := startBase + offset
	endPrice := endBase + offset
	touchKind := "resistance_trendline"
	name := "parallel_channel_resistance"
	if side == "lower" {
		typ = "support_trendline"
		startPrice = startBase - offset
		endPrice = endBase - offset
		touchKind = "support_trendline"
		name = "parallel_channel_support"
	}
	if startPrice <= 0 || endPrice <= 0 {
		return trendLineCandidate{}, false
	}
	touches, weightedTouches, violations := trendlineTouchStats(candles, startIdx, startPrice, slope, touchKind, 0)
	if touches < 2 {
		return trendLineCandidate{}, false
	}
	if violations > maxInt(3, touches) {
		return trendLineCandidate{}, false
	}
	span := endIdx - startIdx
	touchScore := mathutil.Clamp(weightedTouches/8, 0, 1)
	spanScore := mathutil.Clamp(float64(span)/math.Max(1, float64(len(candles))*0.7), 0, 1)
	violationScore := 1 - mathutil.Clamp(float64(violations)/math.Max(4, float64(touches)+3), 0, 1)
	strength := mathutil.Clamp(anchor.line.Strength*0.45+touchScore*0.25+spanScore*0.18+violationScore*0.12, 0, 1)
	return trendLineCandidate{
		line: TrendlineResult{
			Name:       name,
			Type:       typ,
			Slope:      round6(slope),
			TouchCount: touches,
			Strength:   round4(strength),
			Start:      TimePrice{Time: candleDate(candles[startIdx]), Price: round2(startPrice)},
			End:        TimePrice{Time: candleDate(candles[endIdx]), Price: round2(endPrice)},
			Status:     trendlineStatus(candles, endPrice, typ, 0),
		},
		startIdx:   startIdx,
		endIdx:     endIdx,
		startPrice: startPrice,
		endPrice:   endPrice,
	}, true
}

func envelopeOffset(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	// Use 70th percentile: captures most price action without extreme spike bias
	idx := int(math.Ceil(float64(len(values))*0.70)) - 1
	idx = maxInt(0, minInt(idx, len(values)-1))
	return values[idx]
}

func filterTrendlineCandidates(candidates []trendLineCandidate, typ string) []trendLineCandidate {
	out := make([]trendLineCandidate, 0)
	for _, candidate := range candidates {
		if candidate.line.Type == typ {
			out = append(out, candidate)
		}
	}
	return out
}

func bestMainTrendline(candles []ohlcv.Candle, candidates []trendLineCandidate, opts Options) (trendLineCandidate, bool) {
	if len(candidates) == 0 {
		return trendLineCandidate{}, false
	}
	minSpan := maxInt(opts.MinTrendlineSpanBars*2, len(candles)/3)
	if candidate, ok := bestCurrentRelevantTrendline(candles, candidates, opts); ok {
		return candidate, true
	}
	bestScore := -1.0
	var best trendLineCandidate
	for _, candidate := range candidates {
		span := trendlineSpan(candidate)
		if span < minSpan {
			continue
		}
		score := mainTrendlineScore(candles, candidate)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	if bestScore >= 0 {
		return best, true
	}
	for _, candidate := range candidates {
		score := mainTrendlineScore(candles, candidate)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best, bestScore >= 0
}

func bestCurrentRelevantTrendline(candles []ohlcv.Candle, candidates []trendLineCandidate, opts Options) (trendLineCandidate, bool) {
	current := candles[len(candles)-1].EffectiveClose()
	if current <= 0 {
		return trendLineCandidate{}, false
	}
	minSpan := maxInt(opts.MinTrendlineSpanBars, len(candles)/6)
	bestScore := -1.0
	var best trendLineCandidate
	for _, candidate := range candidates {
		if trendlineSpan(candidate) < minSpan || !currentRelevantTrendline(candidate, current) {
			continue
		}
		proximity := 1 - mathutil.Clamp(math.Abs(candidate.endPrice-current)/(current*0.25), 0, 1)
		spanScore := mathutil.Clamp(float64(trendlineSpan(candidate))/math.Max(1, float64(len(candles))*0.55), 0, 1)
		touchScore := mathutil.Clamp(float64(candidate.line.TouchCount)/8, 0, 1)
		score := candidate.line.Strength*0.42 + proximity*0.34 + spanScore*0.16 + touchScore*0.08
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best, bestScore >= 0
}

func currentRelevantTrendline(candidate trendLineCandidate, current float64) bool {
	if current <= 0 || candidate.endPrice <= 0 {
		return false
	}
	switch candidate.line.Type {
	case "support_trendline":
		return candidate.endPrice >= current*0.88 && candidate.endPrice <= current*1.04
	case "resistance_trendline":
		return candidate.endPrice >= current*1.04 && candidate.endPrice <= current*2.25
	default:
		return math.Abs(candidate.endPrice-current) <= current*0.35
	}
}

func bestChannelMate(candles []ohlcv.Candle, anchor trendLineCandidate, candidates []trendLineCandidate, opts Options) (trendLineCandidate, bool) {
	if len(candidates) == 0 {
		return trendLineCandidate{}, false
	}
	minSpan := maxInt(opts.MinTrendlineSpanBars, len(candles)/5)
	bestScore := -1.0
	var best trendLineCandidate
	for _, candidate := range candidates {
		if trendlineSpan(candidate) < minSpan {
			continue
		}
		if !parallelSlopes(anchor.line.Slope, candidate.line.Slope) {
			continue
		}
		if anchor.line.Type == "support_trendline" && candidate.endPrice <= anchor.endPrice {
			continue
		}
		if anchor.line.Type == "resistance_trendline" && candidate.endPrice >= anchor.endPrice {
			continue
		}
		slopeDelta := math.Abs(anchor.line.Slope - candidate.line.Slope)
		slopeBase := math.Max(math.Max(math.Abs(anchor.line.Slope), math.Abs(candidate.line.Slope)), 0.000001)
		parallelScore := 1 - mathutil.Clamp(slopeDelta/slopeBase, 0, 1)
		score := mainTrendlineScore(candles, candidate)*0.78 + parallelScore*0.22
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best, bestScore >= 0
}

func bestRecentReactionTrendline(candles []ohlcv.Candle, candidates []trendLineCandidate, opts Options) (trendLineCandidate, bool) {
	bestScore := -1.0
	var best trendLineCandidate
	current := candles[len(candles)-1].EffectiveClose()
	minSpan := maxInt(3, opts.MinTrendlineSpanBars/3)
	maxSpan := maxInt(opts.MinTrendlineSpanBars*5, len(candles)/2)
	recencyWindow := maxInt(opts.MinTrendlineSpanBars*4, len(candles)/4)
	for _, candidate := range candidates {
		span := trendlineSpan(candidate)
		if candidate.line.Slope >= 0 || span < minSpan || span > maxSpan {
			continue
		}
		recency := len(candles) - 1 - candidate.startIdx
		if recency > recencyWindow {
			continue
		}
		proximity := 1.0
		if current > 0 && candidate.endPrice > 0 {
			proximity = 1 - mathutil.Clamp(math.Abs(candidate.endPrice-current)/(current*0.35), 0, 1)
		}
		score := candidate.line.Strength*0.55 + proximity*0.25 + mathutil.Clamp(float64(span)/float64(maxSpan), 0, 1)*0.20
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best, bestScore >= 0
}

func mainTrendlineScore(candles []ohlcv.Candle, candidate trendLineCandidate) float64 {
	spanScore := mathutil.Clamp(float64(trendlineSpan(candidate))/math.Max(1, float64(len(candles))), 0, 1)
	touchScore := mathutil.Clamp(float64(candidate.line.TouchCount)/8, 0, 1)
	statusScore := 1.0
	if candidate.line.Status == statusBroken {
		statusScore = 0.55
	}
	return candidate.line.Strength*0.50 + spanScore*0.32 + touchScore*0.12 + statusScore*0.06
}

func appendDrawingTrendline(selected []drawingTrendline, item drawingTrendline) []drawingTrendline {
	for _, existing := range selected {
		if similarTrendline(existing.candidate, item.candidate) {
			return selected
		}
	}
	return append(selected, item)
}

func buildSummary(current float64, supports, resistances []Level, patterns []PatternResult, breakout BreakoutAnalysis, ma MovingAverages) Summary {
	mainSupport := nearestSupport(supports, current)
	mainResistance := nearestResistance(resistances, current)
	comment := "OHLCV verisine göre ana formasyon güveni düşük; destek/direnç ve EMA konumu izlenmeli."
	if len(patterns) > 0 {
		comment = fmt.Sprintf("%s tespit edildi; durum %s, confidence %.2f.", patterns[0].Name, patterns[0].Status, patterns[0].Confidence)
	}
	bullish := "EMA20 üzerine kapanış ve en yakın direncin hacimli kırılımı beklenir."
	if mainResistance != nil {
		bullish = fmt.Sprintf("%.2f direnci hacimli kırılırsa bir sonraki direnç hedeflenebilir.", mainResistance.Price)
	}
	bearish := "Ana destek altında kapanış formasyonu zayıflatır."
	if mainSupport != nil {
		bearish = fmt.Sprintf("%.2f ana desteği altında kapanış gelirse destek kaybı senaryosu çalışır.", mainSupport.Price)
	}
	if breakout.Status != "none" {
		comment += " Kırılım durumu: " + breakout.Status + "."
	}
	if ma.Signal == "bearish" {
		comment += " EMA20/EMA50 görünümü zayıf."
	}
	return Summary{
		ShortComment:     comment,
		BullishCondition: bullish,
		BearishCondition: bearish,
		RiskNote:         "Bu analiz yatırım tavsiyesi değildir.",
	}
}

func patternFromLines(name, category string, confidence float64, status string, candles []ohlcv.Candle, upper, lower trendLineCandidate, support, resistance float64) PatternResult {
	startIdx := minInt(upper.startIdx, lower.startIdx)
	endIdx := len(candles) - 1
	height := math.Abs(resistance - support)
	return PatternResult{
		Name:       name,
		Category:   category,
		Confidence: round4(confidence),
		Status:     status,
		StartDate:  candleDate(candles[startIdx]),
		EndDate:    candleDate(candles[endIdx]),
		UpperLine: PatternLine{
			Start: TimePrice{Time: upper.line.Start.Time, Price: upper.line.Start.Price},
			End:   TimePrice{Time: upper.line.End.Time, Price: upper.line.End.Price},
		},
		LowerLine: PatternLine{
			Start: TimePrice{Time: lower.line.Start.Time, Price: lower.line.Start.Price},
			End:   TimePrice{Time: lower.line.End.Time, Price: lower.line.End.Price},
		},
		MainSupport:    round2(support),
		MainResistance: round2(resistance),
		BreakoutLevel:  round2(resistance),
		BreakdownLevel: round2(support),
		Targets:        []float64{round2(resistance + height*0.5), round2(resistance + height)},
		InvalidLevel:   round2(support),
	}
}

func horizontalLine(start, end ohlcv.Candle, price float64) PatternLine {
	return PatternLine{
		Start: TimePrice{Time: candleDate(start), Price: round2(price)},
		End:   TimePrice{Time: candleDate(end), Price: round2(price)},
	}
}

func patternBreakStatus(candles []ohlcv.Candle, lower, upper, atr float64) string {
	current := candles[len(candles)-1].EffectiveClose()
	tol := math.Max(atr*0.35, current*0.006)
	if current > upper+tol {
		if recentRetest(candles, upper, tol, true) {
			return "retest"
		}
		return "breakout"
	}
	if current < lower-tol {
		if recentRetest(candles, lower, tol, false) {
			return "retest"
		}
		return "breakdown"
	}
	if fake := detectRecentFake(candles, []Level{{Price: lower}}, []Level{{Price: upper}}, tol); fake.Status == "fake_breakout" || fake.Status == "fake_breakdown" {
		return fake.Status
	}
	return "forming"
}

func containmentStatus(current, lower, upper, atr float64) string {
	tol := math.Max(atr*0.35, current*0.006)
	if current > upper+tol {
		return "breakout"
	}
	if current < lower-tol {
		return "breakdown"
	}
	return "forming"
}

func exportTrendlines(candidates []trendLineCandidate) []TrendlineResult {
	out := make([]TrendlineResult, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.line)
	}
	return out
}

func bestTrendlineByType(candidates []trendLineCandidate, typ string) *trendLineCandidate {
	for i := range candidates {
		if candidates[i].line.Type == typ {
			return &candidates[i]
		}
	}
	return nil
}

func trendlineSpan(candidate trendLineCandidate) int {
	return maxInt(0, candidate.endIdx-candidate.startIdx)
}

func similarTrendline(a, b trendLineCandidate) bool {
	if a.line.Type != b.line.Type {
		return false
	}
	if math.Abs(float64(a.startIdx-b.startIdx)) > 8 {
		return false
	}
	if math.Abs(a.line.Slope-b.line.Slope) > math.Max(math.Abs(a.line.Slope), math.Abs(b.line.Slope))*0.18 {
		return false
	}
	return math.Abs(a.endPrice-b.endPrice) <= math.Max(a.endPrice, b.endPrice)*0.025
}

func filterSwings(swings []SwingPoint, kind string) []SwingPoint {
	out := make([]SwingPoint, 0)
	for _, swing := range swings {
		if swing.Kind == kind {
			out = append(out, swing)
		}
	}
	return out
}

func parallelSlopes(a, b float64) bool {
	denom := math.Max(math.Max(math.Abs(a), math.Abs(b)), 1e-9)
	return math.Abs(a-b)/denom <= 0.15
}

func slopeFlatThreshold(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	last := candles[len(candles)-1].EffectiveClose()
	// Threshold: classify as ascending/descending if total move > 1.5% over the period.
	// slope unit = price/bar, so normalize by bar count.
	return last * 0.015 / float64(len(candles))
}

// patternLineStyle returns colors and line style for pattern boundary lines.
func patternLineStyle(p PatternResult) (upperColor, lowerColor, style string, width int) {
	switch p.Category {
	case "channel":
		switch p.Name {
		case "ascending_channel":
			return "#2196F3", "#2196F3", "solid", 3 // blue
		case "descending_channel":
			return "#FF5722", "#FF5722", "solid", 3 // orange-red
		default: // horizontal_channel
			return "#9E9E9E", "#9E9E9E", "dashed", 2 // gray
		}
	case "triangle":
		return "#FF9800", "#FF9800", "solid", 2 // orange
	case "wedge":
		return "#9C27B0", "#9C27B0", "solid", 2 // purple
	default:
		return "yellow", "yellow", "solid", 3
	}
}

func lineValue(startIdx int, startPrice, slope float64, idx int) float64 {
	return startPrice + slope*float64(idx-startIdx)
}

func closeValues(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, candle := range candles {
		out[i] = candle.EffectiveClose()
	}
	return out
}

func highLow(candles []ohlcv.Candle) (float64, float64) {
	if len(candles) == 0 {
		return 0, 0
	}
	high := candles[0].EffectiveHigh()
	low := candles[0].EffectiveLow()
	for _, candle := range candles[1:] {
		if candle.EffectiveHigh() > high {
			high = candle.EffectiveHigh()
		}
		if candle.EffectiveLow() < low {
			low = candle.EffectiveLow()
		}
	}
	return high, low
}

func averageRange(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	sum := 0.0
	for _, candle := range candles {
		sum += candle.EffectiveHigh() - candle.EffectiveLow()
	}
	return sum / float64(len(candles))
}

func averageVolume(candles []ohlcv.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	sum := 0.0
	for _, candle := range candles {
		sum += candle.EffectiveVolume()
	}
	return sum / float64(len(candles))
}

func averageRecentVolume(candles []ohlcv.Candle, period int) float64 {
	start := maxInt(0, len(candles)-period)
	return averageVolume(candles[start:])
}

func regressionSlope(candles []ohlcv.Candle, period int) float64 {
	if len(candles) == 0 {
		return 0
	}
	start := maxInt(0, len(candles)-period)
	values := closeValues(candles[start:])
	if len(values) < 2 {
		return 0
	}
	n := len(values)
	meanX := float64(n-1) / 2
	meanY := mathutil.Mean(values)
	num := 0.0
	den := 0.0
	for i, value := range values {
		x := float64(i)
		num += (x - meanX) * (value - meanY)
		den += (x - meanX) * (x - meanX)
	}
	return mathutil.SafeDiv(num, den)
}

func classifySlope(slope, threshold float64) string {
	if slope > threshold {
		return "uptrend"
	}
	if slope < -threshold {
		return "downtrend"
	}
	return "sideways"
}

func maPosition(price, ma float64) string {
	if price >= ma {
		return "price_above"
	}
	return "price_below"
}

func maSlope(values []float64, atr float64) string {
	if len(values) < 6 {
		return "flat"
	}
	diff := values[len(values)-1] - values[len(values)-6]
	threshold := math.Max(atr*0.08, math.Abs(values[len(values)-1])*0.001)
	if diff > threshold {
		return "rising"
	}
	if diff < -threshold {
		return "falling"
	}
	return "flat"
}

func nearestSupport(levels []Level, price float64) *Level {
	var selected *Level
	best := math.MaxFloat64
	for i := range levels {
		if levels[i].Price > price {
			continue
		}
		distance := price - levels[i].Price
		if distance < best {
			best = distance
			selected = &levels[i]
		}
	}
	if selected == nil && len(levels) > 0 {
		return &levels[0]
	}
	return selected
}

func nearestResistance(levels []Level, price float64) *Level {
	var selected *Level
	best := math.MaxFloat64
	for i := range levels {
		if levels[i].Price < price {
			continue
		}
		distance := levels[i].Price - price
		if distance < best {
			best = distance
			selected = &levels[i]
		}
	}
	if selected == nil && len(levels) > 0 {
		return &levels[0]
	}
	return selected
}

func nextTargets(levels []Level, from float64, above bool) []float64 {
	if above {
		return nextResistanceLevels(levels, from, 2)
	}
	return lowerSupportLevels(levels, from, 2)
}

func nextResistanceLevels(levels []Level, from float64, limit int) []float64 {
	vals := make([]float64, 0)
	for _, level := range levels {
		if level.Price > from {
			vals = append(vals, level.Price)
		}
	}
	sort.Float64s(vals)
	return limitFloats(vals, limit)
}

func lowerSupportLevels(levels []Level, from float64, limit int) []float64 {
	vals := make([]float64, 0)
	for _, level := range levels {
		if level.Price < from {
			vals = append(vals, level.Price)
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(vals)))
	return limitFloats(vals, limit)
}

func limitFloats(values []float64, limit int) []float64 {
	if len(values) > limit {
		values = values[:limit]
	}
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = round2(value)
	}
	return out
}

func futurePoint(candles []ohlcv.Candle, bars int, price float64) TimePrice {
	last := candles[len(candles)-1]
	step := time.Hour * 24
	if len(candles) >= 2 {
		diff := last.Time.Sub(candles[len(candles)-2].Time)
		if diff > 0 {
			step = diff
		}
	}
	return TimePrice{Time: candleDateFromTime(last.Time.Add(step * time.Duration(bars))), Price: round2(price)}
}

func firstOr(values []float64, fallback float64) float64 {
	if len(values) == 0 {
		return round2(fallback)
	}
	return values[0]
}

func lastValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func candleDate(candle ohlcv.Candle) string {
	if candle.Time.IsZero() {
		return ""
	}
	return candleDateFromTime(candle.Time)
}

func candleDateFromTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(candleDateLocation).Format("2006-01-02")
}

func dedupeLines(lines []LineObject) []LineObject {
	seen := map[string]bool{}
	out := make([]LineObject, 0, len(lines))
	for _, line := range lines {
		if line.ID != "" && seen[line.ID] {
			continue
		}
		if containsSimilarLine(out, line) {
			continue
		}
		if line.ID != "" {
			seen[line.ID] = true
		}
		out = append(out, line)
	}
	return out
}

func containsSimilarLine(lines []LineObject, target LineObject) bool {
	for _, line := range lines {
		if similarLineObject(line, target) {
			return true
		}
	}
	return false
}

func similarLineObject(a, b LineObject) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case "horizontal":
		return math.Abs(a.Price-b.Price) <= math.Max(maxFloat(a.Price, b.Price)*0.001, 0.01)
	case "trendline":
		if a.StartTime != b.StartTime || a.EndTime != b.EndTime {
			return false
		}
		return closeLinePrice(a.StartPrice, b.StartPrice) && closeLinePrice(a.EndPrice, b.EndPrice)
	default:
		return false
	}
}

func closeLinePrice(a, b float64) bool {
	return math.Abs(a-b) <= math.Max(maxFloat(a, b)*0.0025, 0.01)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func round6(value float64) float64 {
	return math.Round(value*1000000) / 1000000
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

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
