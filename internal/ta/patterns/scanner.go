package patterns

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/supportresistance"
)

type ScannerInput struct {
	Timeframe         string
	Candles           []ohlcv.Candle
	Indicators        ohlcv.IndicatorSnapshot
	SupportResistance supportresistance.Result
	chartSwings       []swing
}

type ScannerOutput struct {
	Patterns          []ohlcv.PatternResult     `json:"patterns"`
	PatternCandidates []ohlcv.PatternResult     `json:"pattern_candidates,omitempty"`
	PatternScans      []ohlcv.PatternScanResult `json:"pattern_scans"`
	ScannedCount      int                       `json:"scanned_count"`
	MatchedCount      int                       `json:"matched_count"`
	CandidateCount    int                       `json:"candidate_count"`
	DetectorCount     int                       `json:"detector_count"`
}

type PatternDetector interface {
	Name() string
	Detect(context.Context, ScannerInput) ([]ohlcv.PatternResult, error)
}

type DetectorRunner interface {
	Run(context.Context, []PatternDetector, ScannerInput) ([]ohlcv.PatternResult, int, error)
}

type Scanner struct {
	detectors []PatternDetector
	runner    DetectorRunner
}

func NewScanner(detectors ...PatternDetector) *Scanner {
	if len(detectors) == 0 {
		detectors = defaultDetectors()
	}
	return &Scanner{
		detectors: append([]PatternDetector{}, detectors...),
		runner:    concurrentDetectorRunner{},
	}
}

func Scan(ctx context.Context, input ScannerInput) (ScannerOutput, error) {
	return NewScanner().Scan(ctx, input)
}

func (s *Scanner) Scan(ctx context.Context, input ScannerInput) (ScannerOutput, error) {
	if len(input.Candles) == 0 {
		return ScannerOutput{}, fmt.Errorf("scan patterns requires candles: %w", ErrPatternData)
	}
	if input.chartSwings == nil && len(input.Candles) >= 7 {
		input.chartSwings = detectChartSwings(input.Candles, 3)
	}
	runner := s.runner
	if runner == nil {
		runner = concurrentDetectorRunner{}
	}
	all, scanned, err := runner.Run(ctx, s.detectors, input)
	if err != nil {
		return ScannerOutput{}, err
	}
	candidates := uniquePatterns(all)
	patterns, candidates := resolvePatternSignals(input, candidates)
	patternScans := scanPatternCatalog(input, candidates)
	return ScannerOutput{
		Patterns:          patterns,
		PatternCandidates: candidates,
		PatternScans:      patternScans,
		ScannedCount:      scanned,
		MatchedCount:      len(patterns),
		CandidateCount:    len(candidates),
		DetectorCount:     len(s.detectors),
	}, nil
}

type detectorRunResult struct {
	name     string
	patterns []ohlcv.PatternResult
	err      error
}

type concurrentDetectorRunner struct{}

func (concurrentDetectorRunner) Run(ctx context.Context, detectors []PatternDetector, input ScannerInput) ([]ohlcv.PatternResult, int, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan detectorRunResult, len(detectors))
	var wg sync.WaitGroup
	for _, detector := range detectors {
		detector := detector
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runCtx.Err(); err != nil {
				results <- detectorRunResult{name: detector.Name(), err: err}
				return
			}
			patterns, err := detector.Detect(runCtx, input)
			results <- detectorRunResult{name: detector.Name(), patterns: patterns, err: err}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var all []ohlcv.PatternResult
	scanned := 0
	var firstErr error
	for result := range results {
		scanned++
		if result.err != nil {
			if strings.Contains(result.err.Error(), ErrPatternData.Error()) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("detect %s: %w", result.name, result.err)
				cancel()
			}
			continue
		}
		all = append(all, result.patterns...)
	}
	if firstErr != nil {
		return nil, scanned, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, scanned, fmt.Errorf("pattern scan canceled: %w", err)
	}
	return all, scanned, nil
}

func defaultDetectors() []PatternDetector {
	detectors := []PatternDetector{
		legacyCandlestickDetector{},
		legacyChartDetector{},
	}
	detectors = append(detectors, registeredPatternDetectors()...)
	return detectors
}

type legacyCandlestickDetector struct{}

func (legacyCandlestickDetector) Name() string { return "legacy-candlestick-suite" }

func (legacyCandlestickDetector) Detect(_ context.Context, input ScannerInput) ([]ohlcv.PatternResult, error) {
	return DetectCandlestick(input.Candles, input.Indicators.VolumeSMA20)
}

type legacyChartDetector struct{}

func (legacyChartDetector) Name() string { return "legacy-chart-suite" }

func (legacyChartDetector) Detect(_ context.Context, input ScannerInput) ([]ohlcv.PatternResult, error) {
	if len(input.Candles) < 20 {
		return nil, nil
	}
	return DetectChartPatterns(input.Candles, input.Indicators.VolumeSMA20)
}

func RegisteredPatternNames() []string {
	specs := registeredPatternSpecs()
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	sort.Strings(names)
	return names
}

func scanPatternCatalog(input ScannerInput, matchedPatterns []ohlcv.PatternResult) []ohlcv.PatternScanResult {
	matchedByName := map[string]ohlcv.PatternResult{}
	for _, pattern := range matchedPatterns {
		key := strings.ToLower(strings.TrimSpace(pattern.Name))
		if key == "" {
			continue
		}
		matchedByName[key] = pattern
	}
	specs := registeredPatternSpecs()
	out := make([]ohlcv.PatternScanResult, 0, len(specs))
	for _, spec := range specs {
		scan := ohlcv.PatternScanResult{
			Name:      spec.Name,
			Category:  spec.Category,
			Group:     spec.Group,
			Direction: spec.Direction,
			Matched:   false,
			Source:    spec.Template,
			Evidence:  []string{"Bu formasyon mevcut mum yapısında tespit edilmedi."},
		}
		if patternRequiresExternalData(spec.Template) {
			scan.Source = "requires_external_data"
			scan.Evidence = []string{patternExternalDataEvidence(spec.Template)}
		}
		if patternRequiresSpecificRule(spec.Template) {
			scan.Source = "requires_specific_rule"
			scan.Evidence = []string{patternSpecificRuleEvidence()}
		}
		if pattern, ok := matchedByName[strings.ToLower(strings.TrimSpace(spec.Name))]; ok {
			scan.Direction = pattern.Direction
			scan.Matched = true
			scan.Actionable = pattern.Actionable
			scan.Confidence = pattern.Confidence
			scan.CalibratedConfidence = pattern.CalibratedConfidence
			scan.SignalScore = pattern.SignalScore
			scan.Resolution = pattern.Resolution
			scan.ConflictStatus = pattern.ConflictStatus
			scan.RejectionReasons = append([]string{}, pattern.RejectionReasons...)
			scan.Evidence = append([]string{}, pattern.Evidence...)
			out = append(out, scan)
			continue
		}
		out = append(out, scan)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Matched != out[j].Matched {
			return out[i].Matched
		}
		if out[i].Category == out[j].Category {
			if out[i].Group == out[j].Group {
				return out[i].Name < out[j].Name
			}
			return out[i].Group < out[j].Group
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func patternRequiresExternalData(template string) bool {
	return false
}

func patternRequiresSpecificRule(template string) bool {
	return template == "generic" || (!patternRequiresExternalData(template) && !patternTemplateHasScannerRule(template))
}

func patternExternalDataEvidence(template string) string {
	switch template {
	case "market_profile":
		return "Bu formasyon gerçek TPO/volume profile veya seans içi auction verisi gerektirir; tek varlık OHLCV mum verisiyle profesyonel olarak hesaplanmadı."
	case "point_figure":
		return "Bu formasyon gerçek Point & Figure kutu/ters dönüş verisi gerektirir; tek varlık OHLCV mum verisiyle profesyonel olarak hesaplanmadı."
	default:
		return "Bu formasyon ek dış veri gerektirir; mevcut OHLCV girdisiyle profesyonel olarak hesaplanmadı."
	}
}

func patternSpecificRuleEvidence() string {
	return "Bu formasyon için özel ve doğrulanmış bir tarama kuralı tanımlı değil; genel fiyat hareketi proxy'siyle eşleştirilmedi."
}

func uniquePatterns(patterns []ohlcv.PatternResult) []ohlcv.PatternResult {
	bestByName := map[string]ohlcv.PatternResult{}
	for _, pattern := range patterns {
		key := strings.ToLower(strings.TrimSpace(pattern.Name))
		if key == "" {
			continue
		}
		current, ok := bestByName[key]
		if !ok || pattern.Confidence > current.Confidence || (pattern.Confidence == current.Confidence && pattern.EndIndex > current.EndIndex) {
			bestByName[key] = pattern
		}
	}
	out := make([]ohlcv.PatternResult, 0, len(bestByName))
	for _, pattern := range bestByName {
		out = append(out, pattern)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].Name < out[j].Name
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}
