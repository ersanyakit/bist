package patterns

import (
	"context"
	"testing"

	"hissebot/internal/ta/indicators"
	"hissebot/internal/ta/ohlcv"
)

func TestPatternValidatorsAcceptCatalogAndScan(t *testing.T) {
	candles := trendingCandles(160)
	snapshot, err := indicators.Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	input := ScannerInput{
		Timeframe:  "1D",
		Candles:    candles,
		Indicators: snapshot,
	}
	output, err := Scan(context.Background(), input)
	if err != nil {
		t.Fatalf("scan patterns: %v", err)
	}
	report := ValidatePatternSystem(input, output)
	if err := report.Err(); err != nil {
		t.Fatalf("pattern validation failed: %v", err)
	}
}

func TestPatternValidatorRejectsMalformedOutput(t *testing.T) {
	candles := trendingCandles(30)
	report := ValidatePatternScan(ScannerInput{Candles: candles}, ScannerOutput{
		DetectorCount: 1,
		ScannedCount:  1,
		MatchedCount:  1,
		Patterns: []ohlcv.PatternResult{{
			Name:       "Malformed Pattern",
			Category:   "classic_chart",
			Direction:  "sideways",
			Confidence: 1.4,
			StartIndex: 10,
			EndIndex:   8,
		}},
		PatternScans: []ohlcv.PatternScanResult{{
			Name:       "Malformed Pattern",
			Category:   "classic_chart",
			Direction:  "sideways",
			Matched:    true,
			Confidence: 1.4,
			Source:     "generic",
		}},
	})
	if !report.HasErrors() {
		t.Fatalf("expected malformed pattern output to fail validation: %s", report.Summary())
	}
}
