package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEquityJSONRoundTripPreservesFinancialFields(t *testing.T) {
	closeValue := 123.45
	q4 := 42.0
	updatedAt := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	equity := Equity{
		Ticker:    "EUPWR",
		Name:      "Europower Enerji",
		AssetType: AssetTypeEquity,
		OHLCV: &OHLCV{
			Source:    "tradingview",
			Close:     &closeValue,
			FetchedAt: updatedAt,
		},
		BilancoCalculations: map[string]YearQuarter{
			"NetKar": {
				"2025": QuarterValues{Q4: &q4},
			},
		},
		UpdatedAt: updatedAt,
	}

	data, err := json.Marshal(equity)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Equity
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.Ticker != "EUPWR" || got.AssetType != AssetTypeEquity {
		t.Fatalf("round trip equity identity = %#v", got)
	}
	if got.OHLCV == nil || got.OHLCV.Close == nil || *got.OHLCV.Close != closeValue {
		t.Fatalf("round trip OHLCV close = %#v, want %v", got.OHLCV, closeValue)
	}
	if got.BilancoCalculations["NetKar"]["2025"].Q4 == nil || *got.BilancoCalculations["NetKar"]["2025"].Q4 != q4 {
		t.Fatalf("round trip financial quarter = %#v, want %v", got.BilancoCalculations, q4)
	}
}

func TestAssetTypePolicyMapsCodesAndKinds(t *testing.T) {
	if !IsEquityAssetType(AssetTypeEquity) {
		t.Fatal("AssetTypeEquity should be recognized as equity")
	}
	if !IsEquityOrUnknownAssetType(AssetTypeUnknown) {
		t.Fatal("AssetTypeUnknown should be accepted for legacy equity universe records")
	}
	if NormalizeAssetKind("digital-asset") != AssetKindCrypto {
		t.Fatal("digital-asset should normalize to crypto")
	}
	if AssetCodeFromKind(AssetKindCrypto) != AssetTypeCrypto {
		t.Fatal("crypto kind should map to crypto code")
	}
	if NormalizeAssetKind("gold") != AssetKindCommodity {
		t.Fatal("gold should normalize to commodity")
	}
	if AssetCodeFromKind(AssetKindCommodity) != AssetTypeCommodity {
		t.Fatal("commodity kind should map to commodity code")
	}
	if AssetKindFromCode(AssetTypeEquity) != AssetKindEquity {
		t.Fatal("equity code should map to equity kind")
	}
}
