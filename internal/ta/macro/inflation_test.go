package macro

import "testing"

func TestAdjustValueByInflationUsesSeriesSourceURL(t *testing.T) {
	dataset := InflationDataset{
		Source:          "TUIK",
		SourceURL:       "https://example.test/dataset",
		PreferredSeries: "yi_ufe",
		Series: []InflationSeries{
			{
				ID:        "yi_ufe",
				NameTR:    "YI-UFE",
				Source:    "TUIK Veri Portali",
				SourceURL: "https://example.test/series/yi-ufe.xls",
				Points: []InflationPoint{
					{Period: "2026-04", Value: 100},
					{Period: "2026-05", Value: 125},
				},
			},
		},
	}

	adjustment, ok := AdjustValueByInflation(dataset, 200, "2026-04")
	if !ok {
		t.Fatal("AdjustValueByInflation returned ok=false")
	}
	if adjustment.SourceURL != "https://example.test/series/yi-ufe.xls" {
		t.Fatalf("SourceURL = %q", adjustment.SourceURL)
	}
	if adjustment.AdjustedValue != 250 {
		t.Fatalf("AdjustedValue = %.2f", adjustment.AdjustedValue)
	}
}
