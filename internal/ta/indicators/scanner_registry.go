package indicators

import (
	"context"
	"sort"
	"strings"

	"hissebot/internal/ta/indicators/generated"
	"hissebot/internal/ta/ohlcv"
)

type indicatorSpec struct {
	Name       string
	Category   string
	Group      string
	Template   string
	Confidence float64
}

func registeredIndicatorDetectors() []IndicatorDetector {
	specs := registeredIndicatorSpecs()
	detectors := make([]IndicatorDetector, 0, len(specs))
	for _, spec := range specs {
		detectors = append(detectors, indicatorSpecDetector{spec: spec})
	}
	return detectors
}

func registeredIndicatorSpecs() []indicatorSpec {
	seen := map[string]bool{}
	specs := make([]indicatorSpec, 0, len(generated.Specs))
	for _, spec := range generated.Specs {
		appendUniqueIndicatorSpec(&specs, seen, indicatorSpec{
			Name:       spec.Name,
			Category:   spec.Category,
			Group:      spec.Group,
			Template:   spec.Template,
			Confidence: spec.Confidence,
		})
	}
	sort.SliceStable(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

func appendUniqueIndicatorSpec(specs *[]indicatorSpec, seen map[string]bool, spec indicatorSpec) {
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return
	}
	if spec.Category == "" {
		spec.Category = "indicator"
	}
	if spec.Template == "" {
		spec.Template = "generic"
	}
	if spec.Confidence <= 0 {
		spec.Confidence = 0.55
	}
	key := strings.ToLower(spec.Name)
	if seen[key] {
		return
	}
	seen[key] = true
	*specs = append(*specs, spec)
}

type indicatorSpecDetector struct {
	spec indicatorSpec
}

func (d indicatorSpecDetector) Name() string { return d.spec.Name }

func (d indicatorSpecDetector) Detect(_ context.Context, input ScannerInput) (ohlcv.IndicatorResult, error) {
	return detectIndicatorSpec(input, d.spec)
}

func RegisteredIndicatorNames() []string {
	detectors := registeredIndicatorDetectors()
	names := make([]string, 0, len(detectors))
	for _, detector := range detectors {
		names = append(names, detector.Name())
	}
	sort.Strings(names)
	return names
}
