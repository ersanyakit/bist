package professional

import (
	"math"
	"testing"

	"hissebot/internal/kapingest"
	tamacro "hissebot/internal/ta/macro"
)

func TestDisplayableKAPAssetsRejectsFragmentsAndKeepsRealAssets(t *testing.T) {
	value := 7873542000.0
	items := []kapingest.AssetInventoryItem{
		{
			AssetName:  "Bodrum Hillside Otel",
			AssetType:  "hotel",
			Location:   "Bodrum / Muğla",
			City:       "Mugla",
			Confidence: 0.98,
			ParcelInfo: "363 ada 8 parsel",
			History:    []kapingest.AssetHistoryPoint{{ExpertiseValueExclVATTRY: &value}},
		},
		{
			AssetName:  "F1-01",
			AssetType:  "shop",
			Location:   "Etiler Alkent Sitesi",
			Confidence: 0.91,
			History:    []kapingest.AssetHistoryPoint{{ExpertiseValueExclVATTRY: &value}},
		},
		{
			AssetName:  "ISGY-2503024 KONAK (2 ARSA)",
			AssetType:  "land",
			City:       "Izmir",
			ParcelInfo: "3535 ADA 9 PARSEL",
			Confidence: 1,
			History: []kapingest.AssetHistoryPoint{
				{ExpertiseValueExclVATTRY: &value},
				{ExpertiseDate: "2025-03-05"},
			},
		},
		{
			AssetName:     "Maslak Arsası",
			AssetType:     "land",
			Location:      "Sarıyer / İstanbul",
			City:          "Istanbul",
			District:      "Sarıyer",
			AreaM2:        &value,
			OwnershipType: "subsidiary",
			Confidence:    1,
			History: []kapingest.AssetHistoryPoint{
				{Period: "2026-03-31", ExpertiseDate: "2025-12-31", ExpertiseValueExclVATTRY: &value},
			},
		},
		{
			AssetName:  "ARSA DEĞERĠ=",
			AssetType:  "land",
			Confidence: 0.99,
			History:    []kapingest.AssetHistoryPoint{{ExpertiseValueExclVATTRY: &value}},
		},
		{
			AssetName:  "Net defter değeri 2.184.129 TL",
			AssetType:  "hotel",
			Confidence: 0.99,
			History:    []kapingest.AssetHistoryPoint{{ExpertiseValueExclVATTRY: &value}},
		},
		{
			AssetName:  "Alanı",
			AssetType:  "land",
			AreaM2:     &value,
			Confidence: 1,
			History:    []kapingest.AssetHistoryPoint{{ExpertiseValueExclVATTRY: &value}},
		},
		{
			AssetName:  "Gayrimenkulün bütün hukuki ve yasal prosedürlerini tamamladığı varsayılmıştır.",
			AssetType:  "land",
			Confidence: 0.95,
			History:    []kapingest.AssetHistoryPoint{{ExpertiseValueExclVATTRY: &value}},
		},
		{
			AssetName:     "Türkiye Şişe ve Cam Fabrikaları A.Ş.",
			AssetType:     "factory",
			Location:      "İstanbul/TÜRKİYE",
			City:          "Istanbul",
			OwnershipType: "subsidiary",
			Confidence:    0.90,
		},
	}

	got := displayableKAPAssets("ISGY", items, testInflationDataset())
	if len(got) != 4 {
		t.Fatalf("expected only four displayable assets, got %d: %+v", len(got), got)
	}
	names := map[string]bool{}
	for _, item := range got {
		names[item.AssetName] = true
	}
	if !names["Bodrum Hillside Otel"] || !names["F1-01"] || !names["ISGY-2503024 KONAK (2 ARSA)"] || !names["Maslak Arsası"] {
		t.Fatalf("expected real assets to remain, got %+v", got)
	}
	for _, item := range got {
		if item.AssetName == "ISGY-2503024 KONAK (2 ARSA)" && (item.LatestValueTRY == nil || *item.LatestValueTRY != value) {
			t.Fatalf("expected valuation-code asset to keep latest available value, got %+v", item)
		}
	}
}

func TestDisplayableKAPAssetsKeepsALGYOCanonicalLandAssets(t *testing.T) {
	value := 1122930000.0
	items := []kapingest.AssetInventoryItem{
		{
			AssetName:  "Maslak Arsası",
			AssetType:  "land",
			Location:   "Sarıyer / İstanbul",
			City:       "Istanbul",
			District:   "Sarıyer",
			AreaM2:     &value,
			Confidence: 1,
			History: []kapingest.AssetHistoryPoint{
				{
					Period:                   "2026-03-31",
					ExpertiseDate:            "2025-12-31",
					ExpertiseValueExclVATTRY: valuePtr(value),
					ExpertiseValueInclVATTRY: valuePtr(1235223000),
				},
			},
		},
	}

	got := displayableKAPAssets("ALGYO", items, testInflationDataset())
	if len(got) != 1 || got[0].AssetName != "Maslak Arsası" {
		t.Fatalf("expected ALGYO Maslak Arsası to remain displayable, got %+v", got)
	}
}

func TestKAPAssetInventoryIndexesHistoricalValueToLatestYIUFE(t *testing.T) {
	value := 975000.0
	items := []kapingest.AssetInventoryItem{
		{
			AssetName:  "İzmir Konak Akdeniz Hizmet Binası",
			AssetType:  "office",
			Location:   "İzmir",
			City:       "Izmir",
			ParcelInfo: "950 ada 6 parsel",
			Confidence: 1,
			History: []kapingest.AssetHistoryPoint{
				{ExpertiseDate: "2025-03-28", ExpertiseValueExclVATTRY: &value},
			},
		},
	}

	got := displayableKAPAssets("ISCTR", items, testInflationDataset())
	if len(got) != 1 {
		t.Fatalf("expected one displayable asset, got %+v", got)
	}
	asset := got[0]
	if asset.IndexedValueTRY == nil {
		t.Fatalf("expected indexed value, got %+v", asset)
	}
	want := value * 150 / 100
	if math.Abs(*asset.IndexedValueTRY-want) > 0.01 {
		t.Fatalf("indexed value mismatch: got %.2f want %.2f", *asset.IndexedValueTRY, want)
	}
	if asset.IndexedValueBasePeriod != "2025-03" || asset.IndexedValueAsOf != "2026-05" || asset.IndexedValueFactor != 1.5 {
		t.Fatalf("unexpected index metadata: %+v", asset)
	}
	if asset.Confidence > 0.55 {
		t.Fatalf("suspiciously low office valuation must cap confidence, got %+v", asset)
	}
	if !containsString(asset.Warnings, "asset_value_scale_suspicious_low") {
		t.Fatalf("expected scale warning for suspiciously low real-estate value, got %+v", asset.Warnings)
	}
}

func valuePtr(value float64) *float64 {
	return &value
}

func TestDisplayableKAPAssetsFiltersForeignValuationCode(t *testing.T) {
	value := 975000.0
	items := []kapingest.AssetInventoryItem{
		{
			AssetName:  "ISGY-2503024 KONAK (2 ARSA)",
			AssetType:  "land",
			City:       "Izmir",
			ParcelInfo: "3535 ADA 9 PARSEL",
			Confidence: 1,
			History:    []kapingest.AssetHistoryPoint{{ExpertiseDate: "2025-03-05", ExpertiseValueExclVATTRY: &value}},
		},
	}

	got := displayableKAPAssets("ISCTR", items, testInflationDataset())
	if len(got) != 0 {
		t.Fatalf("foreign valuation-code row must not render as direct ISCTR asset: %+v", got)
	}
	if countForeignKAPValuationRows("ISCTR", items) != 1 {
		t.Fatalf("expected filtered foreign valuation row to be counted")
	}
}

func testInflationDataset() tamacro.InflationDataset {
	return tamacro.InflationDataset{
		Source:          "TUIK fixture",
		PreferredSeries: "yi_ufe",
		Series: []tamacro.InflationSeries{
			{
				ID:     "yi_ufe",
				NameTR: "YI-UFE (2003=100)",
				Points: []tamacro.InflationPoint{
					{Period: "2025-03", Value: 100},
					{Period: "2026-05", Value: 150},
				},
			},
		},
	}
}
