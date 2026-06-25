package storage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/ohlcv"
)

func TestTurkishIndicatorSignalsExposeComputationStatus(t *testing.T) {
	input := []ohlcv.IndicatorResult{
		{Name: "RSI", Category: "momentum", Signal: "neutral", Value: 50, Confidence: 0.62, Computed: true, Source: "snapshot.rsi14"},
		{Name: "Funding Rate", Category: "sentiment", Signal: "requires_external_data", Confidence: 0.55, Computed: false, Source: "external_data_required"},
		{Name: "ADXR", Category: "momentum", Signal: "proxy_only", Confidence: 0.55, Computed: false, Source: "ohlcv_proxy"},
	}
	got := turkishIndicatorSignals(input)
	if got[0]["hesap_durumu"] != "hesaplandi" {
		t.Fatalf("computed status = %+v", got[0])
	}
	if got[1]["hesap_durumu"] != "hesaplanmadi_dis_veri_gerekli" {
		t.Fatalf("external status = %+v", got[1])
	}
	if got[2]["hesap_durumu"] != "hesaplanmadi_yaklasik_sinyal_degil" {
		t.Fatalf("proxy status = %+v", got[2])
	}
	if got[1]["kaynak"] != "dis_veri_gerekli" {
		t.Fatalf("external source = %+v", got[1])
	}
	if got[2]["kaynak"] != "yaklasik_hesap_sinyal_degil" {
		t.Fatalf("proxy source = %+v", got[2])
	}
	for _, row := range got {
		if row["durum_aciklama"] == "" {
			t.Fatalf("missing status explanation: %+v", row)
		}
	}
}

func TestIndicatorSignalNamesExcludeExternalAndProxyRows(t *testing.T) {
	input := []ohlcv.IndicatorResult{
		{Name: "RSI", Signal: "bullish", Confidence: 0.7, Computed: true},
		{Name: "Funding Rate", Signal: "requires_external_data", Confidence: 0.8, Computed: false},
		{Name: "ADXR", Signal: "proxy_only", Confidence: 0.8, Computed: false},
		{Name: "Bollinger", Signal: "neutral", Confidence: 0.8, Computed: true},
	}
	got := indicatorSignalNames(input, 10)
	if len(got) != 1 {
		t.Fatalf("expected only one active computed indicator, got %+v", got)
	}
	if got[0] == "" || got[0][:3] != "RSI" {
		t.Fatalf("unexpected active indicator name: %+v", got)
	}
}

func TestAnalysisDirForCryptoUsesCryptoSibling(t *testing.T) {
	root := filepath.Join("data", "equities")
	got := AnalysisDirForAsset(root, ohlcv.AssetTypeCrypto, "BTCUSDT", "2026-06-13")
	want := filepath.Join("data", "crypto", "BTCUSDT", "analysis", "2026-06-13")
	if got != want {
		t.Fatalf("AnalysisDirForAsset() = %s, want %s", got, want)
	}
}

func TestHydratedAnalysisJSONIncludesMLForecastAdditively(t *testing.T) {
	candles := make([]ohlcv.Candle, 0, 35)
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 35; i++ {
		price := 100 + float64(i)*0.5
		candles = append(candles, ohlcv.Candle{
			Time:   start.AddDate(0, 0, i),
			Open:   price - 0.2,
			High:   price + 1,
			Low:    price - 1,
			Close:  price,
			Volume: 1000 + float64(i),
		})
	}
	result := analysis.SymbolAnalysis{
		Symbol:       "ASELS",
		AssetType:    ohlcv.AssetTypeEquity,
		AnalysisDate: "2026-06-04",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe:   "1D",
				Candles:     candles,
				CandleCount: len(candles),
				LastClose:   candles[len(candles)-1].Close,
				LastVolume:  candles[len(candles)-1].Volume,
			},
		},
		Disclaimer: ohlcv.Disclaimer,
	}
	hydrated := hydrateDerivedReportFields(context.Background(), filepath.Join(t.TempDir(), "equities"), result)
	raw, err := json.Marshal(hydrated)
	if err != nil {
		t.Fatalf("marshal hydrated: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal hydrated: %v", err)
	}
	if decoded["symbol"] != "ASELS" {
		t.Fatalf("symbol broken: %+v", decoded["symbol"])
	}
	if _, ok := decoded["next_session_forecast"]; !ok {
		t.Fatalf("next_session_forecast missing")
	}
	mlForecast, ok := decoded["ml_forecast"].(map[string]any)
	if !ok {
		t.Fatalf("ml_forecast missing: %+v", decoded["ml_forecast"])
	}
	if mlForecast["enabled"] != true {
		t.Fatalf("ml_forecast not enabled by default: %+v", mlForecast)
	}
}

func TestDeterministicInputRequiresPublishablePointForecast(t *testing.T) {
	blocked := analysis.NextSessionForecast{
		Computed:                    true,
		ForecastFor:                 "2026-06-22",
		LastClose:                   100,
		PredictedOpen:               101,
		PredictedClose:              102,
		BacktestSamples:             10,
		BacktestCloseMAEPct:         0.8,
		BacktestDirectionHitRatePct: 60,
		Confidence:                  70,
	}
	blockedInput := deterministicInputFromForecast(blocked)
	if blockedInput.PredictedOpen != 0 || blockedInput.PredictedClose != 0 || blockedInput.ExpectedReturn != 0 {
		t.Fatalf("blocked forecast leaked deterministic ML anchor: %+v", blockedInput)
	}

	publishable := blocked
	publishable.BacktestSamples = 30
	publishable.BacktestCloseMAEPct = 0.8
	publishable.BacktestDirectionHitRatePct = 70
	publishable.Confidence = 70
	publishableInput := deterministicInputFromForecast(publishable)
	if publishableInput.PredictedOpen != 101 || publishableInput.PredictedClose != 102 {
		t.Fatalf("publishable forecast did not feed deterministic ML anchor: %+v", publishableInput)
	}
	if publishableInput.Direction != "up" {
		t.Fatalf("publishable deterministic direction uses shared policy, got %+v", publishableInput)
	}
}
