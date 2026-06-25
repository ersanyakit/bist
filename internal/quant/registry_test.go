package quant

import "testing"

func TestSupportedModulesCoversQuantLibStyleSurface(t *testing.T) {
	modules := SupportedModules()
	if len(modules) < 90 {
		t.Fatalf("supported modules too small: %d", len(modules))
	}
	required := []string{
		"Calculate Bond Yield to Maturity",
		"Calculate Option-Adjusted Spread (OAS)",
		"Calibrate Vasicek Short Rate Model",
		"Black-Scholes Option Price",
		"Black76 Swaption Price",
		"SABR Implied Volatility",
		"Minimum Variance Portfolio",
		"Black-Litterman Posterior Returns & Weights",
		"Bond Future Ctd",
	}
	seen := map[string]ModuleSupport{}
	for _, module := range modules {
		if module.Function == "" || module.Function == "not implemented" {
			t.Fatalf("module has no implementation binding: %+v", module)
		}
		seen[module.Name] = module
	}
	for _, name := range required {
		if _, ok := seen[name]; !ok {
			t.Fatalf("missing module %q", name)
		}
	}
}
