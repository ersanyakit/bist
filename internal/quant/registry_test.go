package quant

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSupportedModulesAreEquityCryptoFocused(t *testing.T) {
	modules := SupportedModules()
	if len(modules) < 8 || len(modules) > 16 {
		t.Fatalf("unexpected supported module count: %d", len(modules))
	}
	required := []string{
		"Return and momentum features",
		"Realized volatility",
		"VaR and CVaR",
		"Drawdown and risk-adjusted return",
		"Benchmark beta and relative strength",
		"Equity quant profile",
		"Crypto quant profile",
		"Stress scenarios",
		"Liquidity and data integrity",
	}
	seen := map[string]ModuleSupport{}
	for _, module := range modules {
		if module.Function == "" || module.Package == "" || module.AssetScope == "" || module.DecisionUse == "" {
			t.Fatalf("module has incomplete binding: %+v", module)
		}
		if strings.Contains(strings.ToLower(module.Name), "bond") ||
			strings.Contains(strings.ToLower(module.Name), "swap") ||
			strings.Contains(strings.ToLower(module.Name), "black-scholes") ||
			strings.Contains(strings.ToLower(module.Name), "sabr") {
			t.Fatalf("non equity/crypto module leaked into active registry: %+v", module)
		}
		seen[module.Name] = module
	}
	for _, name := range required {
		if _, ok := seen[name]; !ok {
			t.Fatalf("missing module %q", name)
		}
	}
	encoded, err := json.Marshal(modules[0])
	if err != nil {
		t.Fatalf("marshal module: %v", err)
	}
	if strings.Contains(string(encoded), "tier") {
		t.Fatalf("tier should not be exposed in module registry JSON: %s", encoded)
	}
}
