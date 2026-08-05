package main

import "testing"

func TestPatternGeneratorCategoryDirectionAndTemplateInference(t *testing.T) {
	if got := canonicalCategory("japon_mum_candlestick_formasyonlari"); got != "candlestick" {
		t.Fatalf("canonicalCategory() = %q, want candlestick", got)
	}
	if got := inferDirection("bullish breakout"); got != "bullish" {
		t.Fatalf("inferDirection(bullish) = %q", got)
	}
	if got := inferDirection("bearish breakdown"); got != "bearish" {
		t.Fatalf("inferDirection(bearish) = %q", got)
	}
	if got := inferTemplate("classic_chart", "double top"); got != "double_top" {
		t.Fatalf("inferTemplate(double top) = %q", got)
	}
	if got := inferTemplate("candlestick", "doji"); got != "candlestick" {
		t.Fatalf("inferTemplate(candlestick) = %q", got)
	}
	if got := inferTemplate("other", "bos"); got != "bos" {
		t.Fatalf("inferTemplate(bos) = %q", got)
	}
	if got := inferTemplate("other", "discount zone"); got != "discount_zone" {
		t.Fatalf("inferTemplate(discount zone) = %q", got)
	}
	if got := inferTemplate("wyckoff", "composite operator"); got != "wyckoff_composite" {
		t.Fatalf("inferTemplate(composite operator) = %q", got)
	}
	if got := evidenceFor("market_profile"); got != "OHLCV-derived TPO and volume profile matched the named auction structure" {
		t.Fatalf("evidenceFor(market_profile) = %q", got)
	}
	if got := evidenceFor("point_figure"); got != "OHLCV-derived Point & Figure box/reversal structure matched the named setup" {
		t.Fatalf("evidenceFor(point_figure) = %q", got)
	}
}

func TestPatternGeneratorInferBuildsCompleteSpec(t *testing.T) {
	spec := infer(rawSpec{Name: "Bullish Engulfing", Category: "japon_mum_candlestick_formasyonlari", Group: "engulfing"})
	if spec.Name != "Bullish Engulfing" || spec.Category != "candlestick" || spec.Direction != "bullish" {
		t.Fatalf("infer() = %#v", spec)
	}
	if spec.Template == "" || spec.Evidence == "" || spec.Confidence <= 0 {
		t.Fatalf("infer() incomplete spec = %#v", spec)
	}
}

func TestPatternGeneratorDirectionOverrides(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"LPSY", "bearish"},
		{"Double Bottom Sell Signal", "bearish"},
		{"Triple Bottom Sell Signal", "bearish"},
		{"Double Top Buy Signal", "bullish"},
		{"Selling Climax", "bullish"},
		{"Buying Climax", "bearish"},
		{"No Demand Bar", "bearish"},
		{"No Supply Bar", "bullish"},
		{"Stopping Volume", "bullish"},
		{"Buy Side Liquidity Sweep", "bearish"},
		{"Sell Side Liquidity Sweep", "bullish"},
		{"Morning Star", "bullish"},
		{"Evening Star", "bearish"},
		{"Three White Soldiers", "bullish"},
		{"Three Black Crows", "bearish"},
	}
	for _, tt := range tests {
		spec := infer(rawSpec{Name: tt.name, Category: "diger_bilinen_formasyonlar"})
		if spec.Direction != tt.want {
			t.Fatalf("%s direction = %q, want %q (spec=%#v)", tt.name, spec.Direction, tt.want, spec)
		}
	}
}

// TestPatternGeneratorCandlestickDirectionOverrides pins the direction of classic
// candlestick patterns whose names previously fell through inferDirection's substring
// heuristics to "neutral" — either because no keyword matched at all (e.g. "Three
// Inside Down" has no "top"/"bottom"/"bullish"/"bearish" token) or because the name is a
// composite that breaks a substring match (e.g. "Evening Doji Star" does not contain the
// substring "evening star"). Each of these has a single, unambiguous classic direction
// and must not silently regress back to "neutral", since a neutral pattern is excluded
// from directional conflict resolution and actionable output.
func TestPatternGeneratorCandlestickDirectionOverrides(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Three Inside Down", "bearish"},
		{"Three Outside Down", "bearish"},
		{"Concealing Baby Swallow", "bullish"},
		{"Thrusting Line", "bearish"},
		{"Matching High", "bearish"},
		{"In-Neck Line", "bearish"},
		{"On-Neck Line", "bearish"},
		{"Evening Doji Star", "bearish"},
		{"Morning Doji Star", "bullish"},
		{"Upside Tasuki Gap", "bullish"},
		{"Downside Tasuki Gap", "bearish"},
		{"High Price Gapping Play", "bullish"},
		{"Low Price Gapping Play", "bearish"},
		{"Breaking High Signal", "bullish"},
		{"Breaking Low Signal", "bearish"},
		{"Rising Mat Hold", "bullish"},
		{"Falling Mat Hold", "bearish"},
		// Their directional siblings must remain correct too (regression guard).
		{"Three Inside Up", "bullish"},
		{"Three Outside Up", "bullish"},
		{"Matching Low", "bullish"},
	}
	for _, tt := range tests {
		spec := infer(rawSpec{Name: tt.name, Category: "japon_mum_candlestick_formasyonlari"})
		if spec.Direction != tt.want {
			t.Fatalf("%s direction = %q, want %q (spec=%#v)", tt.name, spec.Direction, tt.want, spec)
		}
	}
}
