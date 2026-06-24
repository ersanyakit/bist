package patterns

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hissebot/internal/ta/indicators"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/supportresistance"
)

func TestRegisteredPatternCatalogCoversRequestedFamilies(t *testing.T) {
	names := RegisteredPatternNames()
	if len(names) < 590 {
		t.Fatalf("registered pattern catalog is too small: got %d", len(names))
	}
	for _, expected := range []string{
		"Head and Shoulders",
		"Bullish RSI Divergence",
		"Fair Value Gap",
		"Fibonacci Spiral",
		"Accumulation Schematic Type 1",
		"Double Top Buy Signal",
		"P-Shape Profile",
	} {
		if !containsName(names, expected) {
			t.Fatalf("missing expected pattern %q", expected)
		}
	}
}

func TestScannerInputOutputContract(t *testing.T) {
	candles := trendingCandles(90)
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	output, err := Scan(context.Background(), ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Indicators: snapshot,
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if output.DetectorCount < len(RegisteredPatternNames())+2 {
		t.Fatalf("detector count does not include generated and legacy detectors: %d", output.DetectorCount)
	}
	if output.ScannedCount != output.DetectorCount {
		t.Fatalf("scanned count mismatch: scanned=%d detectors=%d", output.ScannedCount, output.DetectorCount)
	}
	if len(output.Patterns) == 0 {
		t.Fatal("expected at least one actionable pattern")
	}
	if len(output.PatternCandidates) == 0 {
		t.Fatal("expected raw pattern candidates")
	}
	if len(output.PatternScans) < len(RegisteredPatternNames()) {
		t.Fatalf("expected full pattern scan catalog, got %d scans for %d registered patterns", len(output.PatternScans), len(RegisteredPatternNames()))
	}
	matchedScans := 0
	unmatchedScans := 0
	for _, scan := range output.PatternScans {
		if scan.Name == "" || scan.Category == "" || scan.Source == "" {
			t.Fatalf("invalid pattern scan output: %+v", scan)
		}
		if scan.Matched {
			matchedScans++
			continue
		}
		unmatchedScans++
	}
	if matchedScans == 0 || unmatchedScans == 0 {
		t.Fatalf("expected matched and unmatched pattern scans, matched=%d unmatched=%d", matchedScans, unmatchedScans)
	}
	for _, pattern := range output.Patterns {
		if pattern.Name == "" || pattern.Category == "" || pattern.EndIndex < pattern.StartIndex {
			t.Fatalf("invalid pattern output: %+v", pattern)
		}
		if !pattern.Actionable || len(pattern.RejectionReasons) > 0 {
			t.Fatalf("pattern result should be actionable and unrejected: %+v", pattern)
		}
	}
	assertNoActionablePatternDirectionConflicts(t, output.Patterns)
}

func TestALGYOFixtureScansAndValidatesPatternsAndIndicators(t *testing.T) {
	candles := loadALGYOChartFixture(t, "D")
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	levels, err := supportresistance.Analyze(candles, snapshot.ATR14, 200)
	if err != nil {
		t.Fatalf("support/resistance: %v", err)
	}
	patternInput := ScannerInput{
		Timeframe:         "1D",
		Candles:           candles,
		Indicators:        snapshot,
		SupportResistance: levels,
	}
	patternOutput, err := Scan(context.Background(), patternInput)
	if err != nil {
		t.Fatalf("scan patterns: %v", err)
	}
	if patternOutput.DetectorCount < len(RegisteredPatternNames())+2 {
		t.Fatalf("detector count does not cover registered catalog: detectors=%d registered=%d", patternOutput.DetectorCount, len(RegisteredPatternNames()))
	}
	if len(patternOutput.Patterns) == 0 {
		t.Fatal("ALGYO daily fixture produced no actionable patterns")
	}
	if len(patternOutput.PatternCandidates) == 0 {
		t.Fatal("ALGYO daily fixture produced no pattern candidates")
	}
	if len(patternOutput.PatternScans) < len(RegisteredPatternNames()) {
		t.Fatalf("missing pattern scan rows: scans=%d registered=%d", len(patternOutput.PatternScans), len(RegisteredPatternNames()))
	}
	for _, scan := range patternOutput.PatternScans {
		if scan.Source == "requires_external_data" || scan.Source == "requires_specific_rule" {
			t.Fatalf("pattern %q is not fully backed by an OHLCV scanner rule: source=%s evidence=%v", scan.Name, scan.Source, scan.Evidence)
		}
	}
	if report := ValidatePatternSystem(patternInput, patternOutput); report.HasErrors() {
		t.Fatalf("pattern validation failed: %s", report.Summary())
	}
	assertNoActionablePatternDirectionConflicts(t, patternOutput.Patterns)

	indicatorInput := indicators.ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Snapshot:   snapshot,
		LastClose:  candles[len(candles)-1].EffectiveClose(),
		LastVolume: candles[len(candles)-1].EffectiveVolume(),
	}
	indicatorOutput, err := indicators.ScanIndicators(context.Background(), indicatorInput)
	if err != nil {
		t.Fatalf("scan indicators: %v", err)
	}
	if indicatorOutput.ComputedCount == 0 {
		t.Fatal("ALGYO daily fixture produced no computed indicators")
	}
	if report := indicators.ValidateIndicatorSystem(candles, snapshot, indicatorOutput); report.HasErrors() {
		t.Fatalf("indicator validation failed: %s", report.Summary())
	}
}

func assertNoActionablePatternDirectionConflicts(t *testing.T, patterns []ohlcv.PatternResult) {
	t.Helper()
	byGroup := map[string]string{}
	globalDirection := ""
	for _, pattern := range patterns {
		if !pattern.Actionable {
			t.Fatalf("non-actionable pattern leaked into actionable output: %+v", pattern)
		}
		if pattern.Direction != "bullish" && pattern.Direction != "bearish" {
			t.Fatalf("non-directional pattern leaked into actionable output: %+v", pattern)
		}
		if globalDirection != "" && globalDirection != pattern.Direction {
			t.Fatalf("actionable output contains both %s and %s signals", globalDirection, pattern.Direction)
		}
		globalDirection = pattern.Direction
		key := pattern.SignalGroup
		if key == "" {
			key = pattern.Category + ":" + pattern.Name
		}
		if existing, ok := byGroup[key]; ok && existing != pattern.Direction {
			t.Fatalf("actionable direction conflict in group %s: %s vs %s", key, existing, pattern.Direction)
		}
		byGroup[key] = pattern.Direction
	}
}

func TestScannerRunsDetectorsConcurrently(t *testing.T) {
	started := make(chan string, 4)
	release := make(chan struct{})
	var active int32
	var maxActive int32
	scanner := NewScanner(
		blockingPatternDetector{name: "a", started: started, release: release, active: &active, maxActive: &maxActive},
		blockingPatternDetector{name: "b", started: started, release: release, active: &active, maxActive: &maxActive},
		blockingPatternDetector{name: "c", started: started, release: release, active: &active, maxActive: &maxActive},
		blockingPatternDetector{name: "d", started: started, release: release, active: &active, maxActive: &maxActive},
	)
	done := make(chan struct {
		output ScannerOutput
		err    error
	}, 1)
	go func() {
		output, err := scanner.Scan(context.Background(), ScannerInput{Candles: flatCandles(25, 100)})
		done <- struct {
			output ScannerOutput
			err    error
		}{output: output, err: err}
	}()

	for i := 0; i < 4; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("detectors did not all start before release; scanner is not running them concurrently")
		}
	}
	if got := atomic.LoadInt32(&maxActive); got < 4 {
		close(release)
		t.Fatalf("expected all detectors to overlap, max active=%d", got)
	}
	close(release)

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("scan: %v", result.err)
		}
		if result.output.ScannedCount != 4 || result.output.DetectorCount != 4 {
			t.Fatalf("unexpected scanner counts: %+v", result.output)
		}
	case <-time.After(time.Second):
		t.Fatal("concurrent scan did not complete after release")
	}
}

func TestBullishRSIDivergenceRequiresSwingDivergence(t *testing.T) {
	candles := trendingCandles(120)
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	match := matchIndicatorPattern(ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Indicators: snapshot,
	}, patternSpec{
		Name:       "Bullish RSI Divergence",
		Category:   "indicator",
		Direction:  "bullish",
		Confidence: 0.58,
	})
	if match.ok {
		t.Fatalf("monotonic RSI strength should not be reported as bullish RSI divergence: %+v", match)
	}
}

func TestGoldenCrossRequiresLatestCross(t *testing.T) {
	candles := trendingCandles(240)
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.SMA50 <= snapshot.SMA200 {
		t.Fatalf("test setup should have SMA50 above SMA200, got sma50=%.2f sma200=%.2f", snapshot.SMA50, snapshot.SMA200)
	}
	match := matchIndicatorPattern(ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Indicators: snapshot,
	}, patternSpec{
		Name:       "Golden Cross",
		Category:   "indicator",
		Direction:  "bullish",
		Confidence: 0.58,
	})
	if match.ok {
		t.Fatalf("existing SMA50>SMA200 state should not be reported as a fresh golden cross: %+v", match)
	}
}

func TestOscillatorDivergenceUsesSwingPivots(t *testing.T) {
	price := make([]float64, 30)
	osc := make([]float64, 30)
	for i := range price {
		price[i] = 100 + float64(i)*0.1
		osc[i] = 55
	}
	price[8], price[9], price[10], price[11], price[12] = 94, 92, 90, 92, 94
	price[23], price[24], price[25], price[26], price[27] = 93, 90, 88, 90, 93
	osc[8], osc[9], osc[10], osc[11], osc[12] = 42, 34, 30, 34, 42
	osc[23], osc[24], osc[25], osc[26], osc[27] = 48, 44, 40, 44, 48
	if !matchOscillatorDivergence(price, osc, "bullish") {
		t.Fatal("expected lower price low and higher oscillator low to match bullish divergence")
	}
	if matchOscillatorDivergence(price, osc, "bearish") {
		t.Fatal("bullish swing divergence should not match bearish divergence")
	}
}

func TestGeneratedPatternResultBuilderKeepsSingleBarWindow(t *testing.T) {
	candles := flatCandles(5, 100)
	result, err := buildGeneratedPatternResult(ScannerInput{Candles: candles}, patternSpec{
		Name:       "Synthetic Indicator Pattern",
		Category:   "indicator",
		Direction:  "neutral",
		Confidence: 0.55,
		Evidence:   "synthetic indicator condition matched",
	}, generatedMatch{
		ok:         true,
		start:      2,
		end:        2,
		confidence: 0.55,
	})
	if err != nil {
		t.Fatalf("build result: %v", err)
	}
	if result.StartIndex != 2 || result.EndIndex != 2 {
		t.Fatalf("single-bar match window should remain intact, got start=%d end=%d", result.StartIndex, result.EndIndex)
	}
	if result.Confidence != 0.55 {
		t.Fatalf("indicator confidence should not be volume-adjusted, got %.2f", result.Confidence)
	}
	if len(result.Evidence) != 1 {
		t.Fatalf("volume evidence should be omitted for volume-ignored specs, got %+v", result.Evidence)
	}
}

func loadALGYOChartFixture(t *testing.T, interval string) []ohlcv.Candle {
	t.Helper()
	root := repoRoot(t)
	path := filepath.Join(root, "data", "equities", "ALGYO", "charts", strings.ToUpper(interval)+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ALGYO chart fixture %s: %v", path, err)
	}
	var chart struct {
		Candles []struct {
			Time   int64   `json:"time"`
			Open   float64 `json:"open"`
			High   float64 `json:"high"`
			Low    float64 `json:"low"`
			Close  float64 `json:"close"`
			Volume float64 `json:"volume"`
		} `json:"candles"`
	}
	if err := json.Unmarshal(raw, &chart); err != nil {
		t.Fatalf("parse ALGYO chart fixture %s: %v", path, err)
	}
	if len(chart.Candles) < 260 {
		t.Fatalf("ALGYO chart fixture has too few candles: %d", len(chart.Candles))
	}
	candles := make([]ohlcv.Candle, 0, len(chart.Candles))
	for i, c := range chart.Candles {
		if c.Time <= 0 || c.Open <= 0 || c.High <= 0 || c.Low <= 0 || c.Close <= 0 {
			t.Fatalf("invalid ALGYO candle[%d]: %+v", i, c)
		}
		candles = append(candles, ohlcv.Candle{
			Time:   time.Unix(c.Time, 0).UTC(),
			Open:   c.Open,
			High:   c.High,
			Low:    c.Low,
			Close:  c.Close,
			Volume: c.Volume,
		})
	}
	return candles
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found while resolving repository root")
		}
		dir = parent
	}
}

func TestGeneratedPatternVolumePoliciesAreExplicit(t *testing.T) {
	candles := flatCandles(25, 100)
	input := ScannerInput{Candles: candles}
	result, err := buildGeneratedPatternResult(input, patternSpec{
		Name:       "Synthetic Double Top",
		Category:   "classic_chart",
		Direction:  "bearish",
		Template:   "double_top",
		Confidence: 0.70,
		Evidence:   "synthetic structure matched",
	}, generatedMatch{
		ok:         true,
		start:      5,
		end:        20,
		confidence: 0.70,
	})
	if err != nil {
		t.Fatalf("build classic chart result: %v", err)
	}
	if result.Confidence != 0.70 {
		t.Fatalf("optional volume should not globally penalize confidence, got %.2f", result.Confidence)
	}
	if len(result.Evidence) != 2 {
		t.Fatalf("optional volume specs should include volume context, got %+v", result.Evidence)
	}

	result, err = buildGeneratedPatternResult(input, patternSpec{
		Name:       "Synthetic Volume Pattern",
		Category:   "volume",
		Direction:  "bullish",
		Template:   "volume",
		Confidence: 0.58,
		Evidence:   "synthetic volume behavior matched",
	}, generatedMatch{
		ok:         true,
		start:      5,
		end:        20,
		confidence: 0.58,
	})
	if err != nil {
		t.Fatalf("build volume result: %v", err)
	}
	if !result.VolumeConfirmed {
		t.Fatalf("intrinsic volume specs should report volume-confirmed when the matcher succeeds")
	}
	if len(result.Evidence) != 2 || result.Evidence[1] != "volume behavior is part of the matched setup" {
		t.Fatalf("unexpected intrinsic volume evidence: %+v", result.Evidence)
	}
}

func TestGenericPatternDoesNotMatchPlainMovement(t *testing.T) {
	candles := trendingCandles(80)
	match := matchPatternSpec(ScannerInput{Candles: candles}, patternSpec{
		Name:       "Navarro 200 Pattern",
		Category:   "other",
		Direction:  "neutral",
		Template:   "generic",
		Confidence: 0.62,
	})
	if match.ok {
		t.Fatalf("unsupported generic pattern should not match plain price movement: %+v", match)
	}
}

func TestRegisteredPatternSpecsHaveExplicitScannerRules(t *testing.T) {
	var missing []string
	for _, spec := range registeredPatternSpecs() {
		if patternRequiresExternalData(spec.Template) {
			continue
		}
		if spec.Template == "generic" || !patternTemplateHasScannerRule(spec.Template) {
			missing = append(missing, spec.Name+"("+spec.Template+")")
		}
	}
	if len(missing) > 0 {
		limit := minInt(len(missing), 20)
		t.Fatalf("patterns without explicit scanner rules: %v", missing[:limit])
	}
}

func TestGeneratedPatternScannerFindsHistoricalWindow(t *testing.T) {
	candles := []ohlcv.Candle{
		patternCandle(0, 10.00, 10.20, 9.70, 10.10, 1000),
		patternCandle(1, 10.10, 10.35, 9.95, 10.25, 1000),
		patternCandle(2, 10.35, 10.50, 10.20, 10.40, 1000),
		patternCandle(3, 10.45, 10.60, 10.40, 10.55, 1000),
		patternCandle(4, 10.65, 10.72, 9.55, 9.70, 1000),
		patternCandle(5, 9.70, 9.90, 9.55, 9.80, 1000),
		patternCandle(6, 9.80, 9.95, 9.65, 9.88, 1000),
		patternCandle(7, 9.88, 10.00, 9.75, 9.92, 1000),
	}
	match := matchPatternSpec(ScannerInput{Candles: candles}, patternSpec{
		Name:       "Bearish Engulfing",
		Category:   "candlestick",
		Direction:  "bearish",
		Template:   "candlestick",
		Confidence: 0.66,
	})
	if !match.ok {
		t.Fatal("expected historical bearish engulfing window to be detected")
	}
	if match.start != 3 || match.end != 4 {
		t.Fatalf("historical match should be mapped to global indexes 3-4, got %d-%d", match.start, match.end)
	}
	result, err := buildGeneratedPatternResult(ScannerInput{Candles: candles}, patternSpec{
		Name:       "Bearish Engulfing",
		Category:   "candlestick",
		Direction:  "bearish",
		Template:   "candlestick",
		Confidence: 0.66,
	}, match)
	if err != nil {
		t.Fatalf("build generated result: %v", err)
	}
	if !result.Tradeable || result.RuleVersion == "" || result.TriggerIndex != result.EndIndex {
		t.Fatalf("missing backtest metadata: %+v", result)
	}
	if result.StopLoss <= result.EntryMax || result.Target1 >= result.EntryMin || result.InvalidationLevel != result.StopLoss {
		t.Fatalf("bearish backtest metadata is incoherent: %+v", result)
	}
}

func TestCandlestickAliasWickSemantics(t *testing.T) {
	candles := []ohlcv.Candle{
		patternCandle(0, 10.00, 10.20, 9.80, 10.00, 1000),
		patternCandle(1, 9.80, 9.90, 9.40, 9.50, 1000),
		patternCandle(2, 9.40, 9.50, 9.10, 9.20, 1000),
		patternCandle(3, 9.10, 9.20, 8.80, 8.90, 1000),
		patternCandle(4, 8.80, 8.90, 8.50, 8.60, 1000),
		patternCandle(5, 8.70, 9.60, 8.68, 8.75, 1000),
	}
	invertedHammer := matchPatternSpec(ScannerInput{Candles: candles}, patternSpec{
		Name:       "Inverted Hammer",
		Category:   "candlestick",
		Direction:  "bullish",
		Template:   "candlestick",
		Confidence: 0.66,
	})
	if !invertedHammer.ok {
		t.Fatalf("expected long upper wick after decline to match inverted hammer")
	}
	hammer := matchPatternSpec(ScannerInput{Candles: candles}, patternSpec{
		Name:       "Hammer",
		Category:   "candlestick",
		Direction:  "bullish",
		Template:   "candlestick",
		Confidence: 0.66,
	})
	if hammer.ok {
		t.Fatalf("inverted hammer fixture should not match hammer: %+v", hammer)
	}
}

func TestCandlestickAliasNestedBodyColors(t *testing.T) {
	homing := []ohlcv.Candle{
		patternCandle(0, 10.00, 10.20, 8.80, 9.00, 1000),
		patternCandle(1, 9.80, 9.90, 9.10, 9.20, 1000),
	}
	if match := matchPatternSpec(ScannerInput{Candles: homing}, patternSpec{
		Name:       "Homing Pigeon",
		Category:   "candlestick",
		Direction:  "bullish",
		Template:   "candlestick",
		Confidence: 0.66,
	}); !match.ok {
		t.Fatalf("expected bearish body nested in bearish body to match homing pigeon")
	}
	if match := matchPatternSpec(ScannerInput{Candles: homing}, patternSpec{
		Name:       "Descending Hawk",
		Category:   "candlestick",
		Direction:  "bearish",
		Template:   "candlestick",
		Confidence: 0.66,
	}); match.ok {
		t.Fatalf("homing pigeon fixture should not match descending hawk: %+v", match)
	}

	descendingHawk := []ohlcv.Candle{
		patternCandle(0, 9.00, 10.20, 8.80, 10.00, 1000),
		patternCandle(1, 9.20, 9.90, 9.10, 9.80, 1000),
	}
	if match := matchPatternSpec(ScannerInput{Candles: descendingHawk}, patternSpec{
		Name:       "Descending Hawk",
		Category:   "candlestick",
		Direction:  "bearish",
		Template:   "candlestick",
		Confidence: 0.66,
	}); !match.ok {
		t.Fatalf("expected bullish body nested in bullish body to match descending hawk")
	}
}

func TestVolumePatternsUseDirectionalVSARules(t *testing.T) {
	noDemand := volumeTrendFixture(true)
	match := matchVolumePattern(noDemand, patternSpec{Name: "No Demand Bar", Category: "volume", Direction: "bearish", Confidence: 0.58})
	if !match.ok || match.direction != "bearish" {
		t.Fatalf("expected no-demand low-volume up bar to be bearish, got %+v", match)
	}

	noSupply := volumeTrendFixture(false)
	match = matchVolumePattern(noSupply, patternSpec{Name: "No Supply Bar", Category: "volume", Direction: "bullish", Confidence: 0.58})
	if !match.ok || match.direction != "bullish" {
		t.Fatalf("expected no-supply low-volume down bar to be bullish, got %+v", match)
	}
}

func TestVolumeAbsorptionRequiresNarrowResult(t *testing.T) {
	narrow := flatCandles(20, 100)
	narrow[len(narrow)-1] = patternCandle(len(narrow)-1, 100.0, 100.25, 99.85, 100.05, 2200)
	match := matchVolumePattern(narrow, patternSpec{Name: "Absorption Volume", Category: "volume", Direction: "neutral", Confidence: 0.58})
	if !match.ok {
		t.Fatalf("expected high-volume narrow-spread candle to match absorption: %+v", match)
	}

	wide := flatCandles(20, 100)
	wide[len(wide)-1] = patternCandle(len(wide)-1, 100.0, 103.0, 98.0, 102.5, 2200)
	match = matchVolumePattern(wide, patternSpec{Name: "Absorption Volume", Category: "volume", Direction: "neutral", Confidence: 0.58})
	if match.ok {
		t.Fatalf("wide high-volume candle should not match absorption: %+v", match)
	}
}

func TestMovingAverageRulesUseExplicitSemantics(t *testing.T) {
	compression := matchIndicatorPattern(ScannerInput{
		Candles: flatCandles(60, 100),
		Indicators: ohlcv.IndicatorSnapshot{
			SMA20:  100.0,
			SMA50:  101.0,
			SMA100: 100.5,
			SMA200: 99.8,
		},
	}, patternSpec{Name: "MA Ribbon Compression", Category: "indicator", Direction: "neutral", Confidence: 0.58})
	if !compression.ok {
		t.Fatalf("MA Ribbon Compression should use the MA short-name rule: %+v", compression)
	}

	noCross := matchIndicatorPattern(ScannerInput{
		Candles: flatCandles(80, 111),
		Indicators: ohlcv.IndicatorSnapshot{
			SMA20: 110,
			SMA50: 100,
		},
	}, patternSpec{Name: "Moving Average Crossover", Category: "indicator", Direction: "bullish", Confidence: 0.58})
	if noCross.ok {
		t.Fatalf("moving-average crossover should require a latest-bar cross, not just bullish MA state: %+v", noCross)
	}
}

func TestIchimokuRulesDoNotShareKumoTwistSignal(t *testing.T) {
	candles := flatCandles(40, 100)
	tkCross := matchIndicatorPattern(ScannerInput{
		Candles: candles,
		Indicators: ohlcv.IndicatorSnapshot{
			IchimokuKumoTwist: 1,
		},
	}, patternSpec{Name: "Tenkan Kijun Bullish Cross", Category: "indicator", Direction: "bullish", Confidence: 0.58})
	if tkCross.ok {
		t.Fatalf("Kumo twist alone should not match a Tenkan/Kijun cross: %+v", tkCross)
	}

	tkCross = matchIndicatorPattern(ScannerInput{
		Candles: candles,
		Indicators: ohlcv.IndicatorSnapshot{
			IchimokuTKCross: 1,
		},
	}, patternSpec{Name: "Tenkan Kijun Bullish Cross", Category: "indicator", Direction: "bullish", Confidence: 0.58})
	if !tkCross.ok {
		t.Fatalf("Tenkan/Kijun cross should match TKCross signal: %+v", tkCross)
	}
}

func TestChartNumericHelpersHandleSignedSeries(t *testing.T) {
	candles := []ohlcv.Candle{
		patternCandle(0, -10, -5, -12, -9, 1000),
		patternCandle(1, -9, -3, -11, -4, 1000),
	}
	if got := highest(candles); got != -3 {
		t.Fatalf("highest should use actual candle highs for signed data, got %.2f", got)
	}
	if got := rangeWidth(candles); got <= 0 {
		t.Fatalf("rangeWidth should remain positive with signed close values, got %.4f", got)
	}
	if got := trend(candles, 1, 1); got <= 0 {
		t.Fatalf("trend should preserve direction with signed base values, got %.4f", got)
	}
}

func TestLegacyPatternVolumeIsOptionalContext(t *testing.T) {
	candles := flatCandles(5, 100)
	chart := chartPattern(chartSpec{name: "Synthetic Chart", direction: "neutral", confidence: 0.72, evidence: "synthetic chart"}, 2, 2, candles, 0)
	if chart.StartIndex != 2 || chart.EndIndex != 2 {
		t.Fatalf("chart pattern should preserve single-bar window, got %+v", chart)
	}
	if chart.Confidence != 0.72 {
		t.Fatalf("missing optional volume should not penalize chart confidence, got %.2f", chart.Confidence)
	}

	candle := onePattern("Synthetic Candle", "neutral", 2, 2, candles, 0, 0.66, "synthetic candle")[0]
	if candle.Confidence != 0.66 {
		t.Fatalf("missing optional volume should not penalize candlestick confidence, got %.2f", candle.Confidence)
	}
}

func TestDoubleTopRequiresNecklineBreak(t *testing.T) {
	candles := flatCandles(70, 100)
	for i := range candles {
		price := 100 + float64(i)*0.2
		candles[i].Open = price - 0.1
		candles[i].High = price + 1
		candles[i].Low = price - 1
		candles[i].Close = price
	}
	swings := []swing{
		{index: 35, price: 120, kind: "high"},
		{index: 43, price: 105, kind: "low"},
		{index: 55, price: 120.5, kind: "high"},
	}
	candles[69].Close = 110
	ok, _, _ := matchDouble("high", 2, 0.01, 8, 70, false)(candles, swings)
	if ok {
		t.Fatal("double top should not match before close breaks neckline")
	}
	candles[69].Close = 103
	ok, _, _ = matchDouble("high", 2, 0.01, 8, 70, false)(candles, swings)
	if !ok {
		t.Fatal("double top should match after close breaks neckline")
	}
}

func containsName(names []string, expected string) bool {
	for _, name := range names {
		if name == expected {
			return true
		}
	}
	return false
}

func trendingCandles(count int) []ohlcv.Candle {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]ohlcv.Candle, count)
	price := 100.0
	for i := 0; i < count; i++ {
		open := price
		closePrice := price + 0.8 + float64(i%5)*0.08
		high := closePrice + 1.4
		low := open - 0.6
		candles[i] = ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: 1000 + float64(i*20),
		}
		price = closePrice
	}
	return candles
}

func volumeTrendFixture(up bool) []ohlcv.Candle {
	candles := flatCandles(20, 100)
	price := 100.0
	for i := range candles {
		open := price
		closePrice := price + 0.4
		if !up {
			closePrice = price - 0.4
		}
		high := mathMax(open, closePrice) + 0.25
		low := mathMin(open, closePrice) - 0.25
		candles[i] = patternCandle(i, open, high, low, closePrice, 1000)
		price = closePrice
	}
	lastOpen := price
	lastClose := price + 0.15
	if !up {
		lastClose = price - 0.15
	}
	candles[len(candles)-1] = patternCandle(len(candles)-1, lastOpen, mathMax(lastOpen, lastClose)+0.10, mathMin(lastOpen, lastClose)-0.10, lastClose, 450)
	return candles
}

func flatCandles(count int, price float64) []ohlcv.Candle {
	candles := make([]ohlcv.Candle, count)
	for i := range candles {
		candles[i] = patternCandle(i, price, price+1, price-1, price, 1000)
	}
	return candles
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

type blockingPatternDetector struct {
	name      string
	started   chan<- string
	release   <-chan struct{}
	active    *int32
	maxActive *int32
}

func (d blockingPatternDetector) Name() string { return d.name }

func (d blockingPatternDetector) Detect(ctx context.Context, _ ScannerInput) ([]ohlcv.PatternResult, error) {
	current := atomic.AddInt32(d.active, 1)
	for {
		maximum := atomic.LoadInt32(d.maxActive)
		if current <= maximum || atomic.CompareAndSwapInt32(d.maxActive, maximum, current) {
			break
		}
	}
	defer atomic.AddInt32(d.active, -1)
	select {
	case d.started <- d.name:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case <-d.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
