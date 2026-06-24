package main

import (
	"reflect"
	"testing"
)

func TestSplitItemsTrimsAndDropsEmptyParts(t *testing.T) {
	got := splitItems(" RSI | MACD || Bollinger Bands ")
	want := []string{"RSI", "MACD", "Bollinger Bands"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitItems() = %#v, want %#v", got, want)
	}
}

func TestInferTemplateClassifiesCoreIndicatorFamilies(t *testing.T) {
	cases := map[string]string{
		"EMA":                     "moving_average",
		"Relative Strength Index": "momentum",
		"Average True Range":      "volatility",
		"Volume Profile":          "support_resistance",
	}

	for name, want := range cases {
		category := "trend"
		if name == "Relative Strength Index" {
			category = "momentum"
		}
		if name == "Average True Range" {
			category = "volatility"
		}
		if name == "Volume Profile" {
			category = "market_profile"
		}
		if got := inferTemplate(category, name); got != want {
			t.Fatalf("inferTemplate(%q, %q) = %q, want %q", category, name, got, want)
		}
	}
}

func TestConfidenceForExternalCategoriesIsZero(t *testing.T) {
	if got := confidenceFor("sentiment"); got != 0 {
		t.Fatalf("confidenceFor(sentiment) = %v, want 0", got)
	}
	if got := confidenceFor("trend"); got <= 0 {
		t.Fatalf("confidenceFor(trend) = %v, want > 0", got)
	}
}
