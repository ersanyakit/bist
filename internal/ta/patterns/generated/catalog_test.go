package generated

import "testing"

func TestGeneratedPatternSpecsAreCompleteAndUnique(t *testing.T) {
	if len(Specs) == 0 {
		t.Fatalf("Specs is empty")
	}

	allowedDirection := map[string]bool{"bullish": true, "bearish": true, "neutral": true}
	seen := map[string]bool{}
	for _, spec := range Specs {
		if spec.Name == "" || spec.Category == "" || spec.Template == "" || spec.Evidence == "" {
			t.Fatalf("incomplete pattern spec: %#v", spec)
		}
		if !allowedDirection[spec.Direction] {
			t.Fatalf("invalid pattern direction: %#v", spec)
		}
		if spec.Confidence < 0 || spec.Confidence > 1 {
			t.Fatalf("pattern confidence out of range: %#v", spec)
		}
		key := spec.Name + "|" + spec.Category
		if seen[key] {
			t.Fatalf("duplicate pattern spec key %s", key)
		}
		seen[key] = true
	}
}

func TestGeneratedPatternDirectionsUseCanonicalOverrides(t *testing.T) {
	tests := map[string]string{
		"LPSY":                      "bearish",
		"Double Bottom Sell Signal": "bearish",
		"Triple Bottom Sell Signal": "bearish",
		"Double Top Buy Signal":     "bullish",
		"Selling Climax":            "bullish",
		"Buying Climax":             "bearish",
		"No Demand Bar":             "bearish",
		"No Supply Bar":             "bullish",
		"Stopping Volume":           "bullish",
		"Morning Star":              "bullish",
		"Evening Star":              "bearish",
		"Three White Soldiers":      "bullish",
		"Three Black Crows":         "bearish",
	}
	for name, want := range tests {
		spec, ok := findSpec(name)
		if !ok {
			t.Fatalf("missing generated spec %q", name)
		}
		if spec.Direction != want {
			t.Fatalf("%s direction = %q, want %q", name, spec.Direction, want)
		}
	}
}

func findSpec(name string) (Spec, bool) {
	for _, spec := range Specs {
		if spec.Name == name {
			return spec, true
		}
	}
	return Spec{}, false
}
