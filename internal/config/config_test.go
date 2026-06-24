package config

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadReadsEnvironmentOverrides(t *testing.T) {
	t.Setenv("HISSEBOT_DATA_DIR", "/tmp/hissebot-data")
	t.Setenv("HISSEBOT_ENDPOINT_URL", "http://127.0.0.1:9000/analyze")
	t.Setenv("HISSEBOT_HTTP_TIMEOUT", "2s")
	t.Setenv("HISSEBOT_COMMAND_TIMEOUT", "15s")
	t.Setenv("HISSEBOT_TV_CHART_TRANSPORT", "http")
	t.Setenv("HISSEBOT_GOLDEN_FINANCIAL_RATIOS_FILE", "/tmp/golden-ratios.json")

	cfg := Load()
	value := reflect.ValueOf(cfg)

	if got := stringField(t, value, "DataDir"); got != "/tmp/hissebot-data" {
		t.Fatalf("DataDir = %q, want env override", got)
	}
	if got := stringField(t, value, "EndpointURL"); got != "http://127.0.0.1:9000/analyze" {
		t.Fatalf("EndpointURL = %q, want env override", got)
	}
	if got := durationField(t, value, "HTTPTimeout"); got != 2*time.Second {
		t.Fatalf("HTTPTimeout = %v, want 2s", got)
	}
	if got := durationField(t, value, "CommandTimeout"); got != 15*time.Second {
		t.Fatalf("CommandTimeout = %v, want 15s", got)
	}
	if got := stringField(t, value, "TradingViewChartTransport"); got != "http" {
		t.Fatalf("TradingViewChartTransport = %q, want http", got)
	}
	if got := stringField(t, value, "GoldenFinancialRatiosFile"); got != "/tmp/golden-ratios.json" {
		t.Fatalf("GoldenFinancialRatiosFile = %q, want env override", got)
	}
}

func TestLoadDoesNotUseHardCodedEndpointToken(t *testing.T) {
	t.Setenv("HISSEBOT_ENDPOINT_TOKEN", "")
	cfg := Load()
	if cfg.EndpointToken != "" {
		t.Fatalf("EndpointToken = %q, want empty without env secret", cfg.EndpointToken)
	}
	issues := ValidateSecurity(Config{EndpointURL: "http://127.0.0.1:9000/endpoint"})
	if len(issues) == 0 || issues[0].Code != "endpoint_token_missing" {
		t.Fatalf("expected missing token security issue, got %+v", issues)
	}
}

func TestValidateSecurityRejectsLegacyDefaultEndpointToken(t *testing.T) {
	issues := ValidateSecurity(Config{EndpointToken: "HISSEYORUMCOINVESTINGBOT_TOKEN_AUTH"})
	if len(issues) == 0 || issues[0].Code != "endpoint_token_uses_default_secret" {
		t.Fatalf("expected default token security issue, got %+v", issues)
	}
}

func stringField(t *testing.T, value reflect.Value, name string) string {
	t.Helper()
	field := reflect.Indirect(value).FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("missing string field %s", name)
	}
	return field.String()
}

func durationField(t *testing.T, value reflect.Value, name string) time.Duration {
	t.Helper()
	field := reflect.Indirect(value).FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("missing duration field %s", name)
	}
	got, ok := field.Interface().(time.Duration)
	if !ok {
		t.Fatalf("field %s has type %T, want time.Duration", name, field.Interface())
	}
	return got
}
