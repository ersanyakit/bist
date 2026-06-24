package chart

import (
	"testing"

	"hissebot/internal/ta/ohlcv"
)

func TestIndicatorCountsSeparateComputedExternalProxyAndActive(t *testing.T) {
	computed, external, proxy, active := indicatorCounts([]ohlcv.IndicatorResult{
		{Name: "RSI", Signal: "bullish", Confidence: 0.7, Computed: true},
		{Name: "MACD", Signal: "neutral", Confidence: 0.7, Computed: true},
		{Name: "Funding Rate", Signal: "requires_external_data", Confidence: 0.7, Computed: false},
		{Name: "ADXR", Signal: "proxy_only", Confidence: 0.7, Computed: false},
	})
	if computed != 2 || external != 1 || proxy != 1 || active != 1 {
		t.Fatalf("counts computed=%d external=%d proxy=%d active=%d", computed, external, proxy, active)
	}
}

func TestSignalTextLabelsExternalAndProxyDistinctly(t *testing.T) {
	if got := signalText("requires_external_data"); got != "Dış Veri Gerekir" {
		t.Fatalf("external label = %q", got)
	}
	if got := signalText("proxy_only"); got != "Sinyal Değil" {
		t.Fatalf("proxy label = %q", got)
	}
}
