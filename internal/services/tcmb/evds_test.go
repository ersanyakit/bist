package tcmb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReportFrequencyFromDataGroup(t *testing.T) {
	cases := map[float64]string{
		1:  "1",
		2:  "2",
		5:  "3",
		7:  "4",
		9:  "5",
		13: "6",
		17: "7",
		21: "8",
	}
	for input, want := range cases {
		if got := ReportFrequencyFromDataGroup(input); got != want {
			t.Fatalf("frequency %.0f: got %q want %q", input, got, want)
		}
	}
}

func TestPointsFromEVDSItems(t *testing.T) {
	raw := []byte(`[
		{"Tarih":"02-01-2024","TP_DK_USD_A_YTL":"29.438200","UNIXTIME":{"$numberLong":"1704142800"}},
		{"Tarih":"04-01-2024","TP_DK_USD_A_YTL":"1,506.2500000000","UNIXTIME":{"$numberLong":"1704315600"}},
		{"Tarih":"03-01-2024","TP_DK_USD_A_YTL":null,"UNIXTIME":{"$numberLong":"1704229200"}}
	]`)
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	points := pointsFromEVDSItems("TP.DK.USD.A.YTL", items)
	if len(points) != 3 {
		t.Fatalf("got %d points", len(points))
	}
	if points[0].Date != "2024-01-02" || points[0].Value == nil || *points[0].Value != 29.4382 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if points[1].Value == nil || *points[1].Value != 1506.25 {
		t.Fatalf("thousands separator value not parsed: %+v", points[1])
	}
	if points[2].Value != nil {
		t.Fatalf("null value should stay nil: %+v", points[2])
	}
}

func TestParseEVDSNumberSupportsTurkishAndInternationalFormats(t *testing.T) {
	cases := map[string]float64{
		"1,506.2500000000":  1506.25,
		"15,184,522,492.00": 15184522492,
		"1.506,25":          1506.25,
		"1506,25":           1506.25,
		"1506.25":           1506.25,
	}
	for input, want := range cases {
		got, err := ParseEVDSNumber(input)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if got != want {
			t.Fatalf("%q: got %f want %f", input, got, want)
		}
	}
}

func TestNormalizeEVDSDisplayDate(t *testing.T) {
	cases := map[string]string{
		"02-01-2024": "2024-01-02",
		"01-2024":    "2024-01",
		"2026-05":    "2026-05",
		"MAYIS 2026": "2026-05",
		"2026":       "2026",
	}
	for input, want := range cases {
		if got := normalizeEVDSDisplayDate(input); got != want {
			t.Fatalf("%q: got %q want %q", input, got, want)
		}
	}
}

func TestRepairEVDSArchiveUsesRawValuesAndNormalizesDates(t *testing.T) {
	root := t.TempDir()
	seriesDir := filepath.Join(root, "series", "bie_test")
	if err := os.MkdirAll(seriesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(seriesDir, "TP_TEST.json")
	raw := []byte(`{
		"series": {"SERIE_CODE":"TP.TEST"},
		"points": [
			{"display_date":"2026-05","value":null,"raw_value":"133,187.5000000000"},
			{"display_date":"2026-06","value":null}
		]
	}`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RepairEVDSArchive(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesUpdated != 1 || result.ValuesRepaired != 1 || result.DatesRepaired != 2 {
		t.Fatalf("unexpected repair result: %+v", result)
	}
	repairedRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var dataset EVDSSeriesDataset
	if err := json.Unmarshal(repairedRaw, &dataset); err != nil {
		t.Fatal(err)
	}
	if dataset.Points[0].Value == nil || *dataset.Points[0].Value != 133187.5 || dataset.Points[0].Date != "2026-05" {
		t.Fatalf("first point not repaired: %+v", dataset.Points[0])
	}
	if dataset.Points[1].Value != nil {
		t.Fatalf("real null must remain nil: %+v", dataset.Points[1])
	}
}
