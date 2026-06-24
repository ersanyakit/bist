package generated

import "testing"

func TestGeneratedIndicatorSpecsAreCompleteAndUnique(t *testing.T) {
	if len(Specs) == 0 {
		t.Fatalf("Specs is empty")
	}

	seen := map[string]bool{}
	for _, spec := range Specs {
		if spec.Name == "" || spec.Category == "" || spec.Template == "" {
			t.Fatalf("incomplete indicator spec: %#v", spec)
		}
		if spec.Confidence < 0 || spec.Confidence > 1 {
			t.Fatalf("indicator confidence out of range: %#v", spec)
		}
		key := spec.Name + "|" + spec.Category
		if seen[key] {
			t.Fatalf("duplicate indicator spec key %s", key)
		}
		seen[key] = true
	}
}
