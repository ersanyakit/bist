package config

import (
	"reflect"
	"testing"
)

func TestParseTimeframesNormalizesValidInput(t *testing.T) {
	got, err := ParseTimeframes("1d, 1W,1M")
	if err != nil {
		t.Fatalf("ParseTimeframes() error = %v", err)
	}

	want := []string{"1D", "1W", "1M"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTimeframes() = %#v, want %#v", got, want)
	}
}

func TestParseTimeframesRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "1D,4H", "daily"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseTimeframes(input); err == nil {
				t.Fatalf("ParseTimeframes(%q) error = nil, want error", input)
			}
		})
	}
}

func TestDefaultConfigContainsSafeOperationalDefaults(t *testing.T) {
	cfg := Default()

	if cfg.Provider == "" {
		t.Fatalf("Default().Provider is empty")
	}
	if cfg.Limit <= 0 {
		t.Fatalf("Default().Limit = %d, want > 0", cfg.Limit)
	}
	if cfg.Workers <= 0 {
		t.Fatalf("Default().Workers = %d, want > 0", cfg.Workers)
	}
}
