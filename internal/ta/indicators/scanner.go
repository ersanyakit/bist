package indicators

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"hissebot/internal/ta/ohlcv"
)

type ScannerInput struct {
	Timeframe  string
	Candles    []ohlcv.Candle
	Snapshot   ohlcv.IndicatorSnapshot
	LastClose  float64
	LastVolume float64
}

type ScannerOutput struct {
	Indicators    []ohlcv.IndicatorResult `json:"indicators"`
	ScannedCount  int                     `json:"scanned_count"`
	ComputedCount int                     `json:"computed_count"`
	SignalCount   int                     `json:"signal_count"`
	DetectorCount int                     `json:"detector_count"`
}

type IndicatorDetector interface {
	Name() string
	Detect(context.Context, ScannerInput) (ohlcv.IndicatorResult, error)
}

type Scanner struct {
	detectors []IndicatorDetector
}

func NewScanner(detectors ...IndicatorDetector) *Scanner {
	if len(detectors) == 0 {
		detectors = registeredIndicatorDetectors()
	}
	return &Scanner{detectors: append([]IndicatorDetector{}, detectors...)}
}

func ScanIndicators(ctx context.Context, input ScannerInput) (ScannerOutput, error) {
	return NewScanner().Scan(ctx, input)
}

func (s *Scanner) Scan(ctx context.Context, input ScannerInput) (ScannerOutput, error) {
	if len(input.Candles) == 0 {
		return ScannerOutput{}, fmt.Errorf("scan indicators requires candles: %w", ErrInsufficientData)
	}
	if input.LastClose == 0 {
		input.LastClose = input.Candles[len(input.Candles)-1].EffectiveClose()
	}
	if input.LastVolume == 0 {
		input.LastVolume = input.Candles[len(input.Candles)-1].EffectiveVolume()
	}
	results := make([]ohlcv.IndicatorResult, 0, len(s.detectors))
	scanned := 0
	computed := 0
	signals := 0
	for _, detector := range s.detectors {
		if err := ctx.Err(); err != nil {
			return ScannerOutput{}, fmt.Errorf("indicator scan canceled: %w", err)
		}
		scanned++
		result, err := detector.Detect(ctx, input)
		if err != nil {
			if strings.Contains(err.Error(), ErrInsufficientData.Error()) {
				continue
			}
			return ScannerOutput{}, fmt.Errorf("detect %s: %w", detector.Name(), err)
		}
		if result.Name == "" {
			continue
		}
		result = reconcileIndicatorResult(input, result)
		if result.Computed {
			computed++
		}
		if result.Computed && result.Confidence >= 0.5 && result.Signal != "neutral" && result.Signal != "info" {
			signals++
		}
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Computed != results[j].Computed {
			return results[i].Computed
		}
		if results[i].Confidence == results[j].Confidence {
			return results[i].Name < results[j].Name
		}
		return results[i].Confidence > results[j].Confidence
	})
	return ScannerOutput{
		Indicators:    results,
		ScannedCount:  scanned,
		ComputedCount: computed,
		SignalCount:   signals,
		DetectorCount: len(s.detectors),
	}, nil
}

func reconcileIndicatorResult(input ScannerInput, result ohlcv.IndicatorResult) ohlcv.IndicatorResult {
	if !result.Computed {
		return result
	}
	name := normalizeIndicatorText(result.Name)
	signal := result.Signal
	confidence := result.Confidence
	evidence := ""
	switch {
	case isMACDIndicator(name):
		signal, confidence, evidence = macdIndicatorSignal(input.Snapshot, confidence)
	case isVolumeParticipationIndicator(name):
		signal, confidence, evidence = volumeParticipationSignal(input.LastVolume, input.Snapshot.VolumeSMA20, confidence)
	case isIchimokuStateIndicator(name):
		signal, confidence, evidence = signedStateSignal(result.Value, confidence, "Ichimoku cloud state is bullish", "Ichimoku cloud state is bearish", "Ichimoku cloud state is neutral")
	default:
		return result
	}
	if signal == result.Signal {
		return result
	}
	result.Evidence = append(result.Evidence, "snapshot consistency override: "+result.Signal+" -> "+signal)
	if evidence != "" {
		result.Evidence = append(result.Evidence, evidence)
	}
	result.Signal = signal
	result.Confidence = confidence
	return result
}
