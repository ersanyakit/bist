package mkk

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"hissebot/internal/util"
)

func TestCompanyIDAcceptsCommonJSONNumberShapes(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{name: "float64", in: float64(123), want: 123},
		{name: "int64", in: int64(456), want: 456},
		{name: "json number", in: json.Number("789"), want: 789},
		{name: "string", in: " 321 ", want: 321},
		{name: "invalid", in: "abc", want: 0},
		{name: "nil", in: nil, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := companyID(tc.in); got != tc.want {
				t.Fatalf("companyID(%#v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestLoadOverridesNormalizesTickerKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overrides.json")
	if err := util.WriteJSON(path, map[string]string{
		" bist:eupwr ": "Europower Enerji",
		"asels":        "Aselsan",
	}); err != nil {
		t.Fatalf("write overrides: %v", err)
	}

	got, err := loadOverrides(path)
	if err != nil {
		t.Fatalf("loadOverrides() error = %v", err)
	}
	want := map[string]string{
		"EUPWR": "Europower Enerji",
		"ASELS": "Aselsan",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadOverrides() = %#v, want %#v", got, want)
	}
}
