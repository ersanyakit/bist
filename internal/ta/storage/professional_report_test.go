package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/kapingest"
	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/investorqa"
	tamacro "hissebot/internal/ta/macro"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/value"

	"github.com/xuri/excelize/v2"
)

func TestReportTextSimplifiesTechnicalEnglishForInvestors(t *testing.T) {
	input := "Trading edge backtest/OOS Owner earnings TTM normalize FCF Bear case Base case Bull case review rejected duplicate ingest hash peer multiple discount moat sentiment"
	got := reportText(input)

	for _, banned := range []string{
		"Trading edge",
		"backtest",
		"OOS",
		"Owner earnings",
		"TTM",
		"FCF",
		"Bear case",
		"Base case",
		"Bull case",
		"review",
		"rejected",
		"duplicate",
		"ingest",
		"hash",
		"peer multiple discount",
		"moat",
		"sentiment",
	} {
		if strings.Contains(got, banned) {
			t.Fatalf("reportText leaked %q in %q", banned, got)
		}
	}

	for _, want := range []string{
		"İstatistiksel işlem avantajı",
		"geçmiş test",
		"ileri dönem test",
		"Sahibine kalan nakit",
		"son 12 ay",
		"normalize serbest nakit akımı",
		"Kötümser senaryo",
		"Baz senaryo",
		"İyimser senaryo",
		"AI çözüm",
		"reddedilen",
		"tekrarlı kayıt",
		"veri içe aktarma",
		"dosya izi",
		"benzer şirketlere göre düşük çarpan",
		"rekabet gücü",
		"haber/duygu tonu",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("reportText() = %q, want %q", got, want)
		}
	}

	path := "data/processed/by_ticker/ALGYO/asset_inventory.json"
	if got := reportText(path); got != "İç veri kaynağı raporda gizlendi." {
		t.Fatalf("reportText(path) = %q, want redacted investor-safe text", got)
	}
	if got := reportText("bist_official_unprocessed_ohlcv kap_pdf_financial_reading_requires_review"); strings.Contains(got, "bist_official") || strings.Contains(got, "kap_pdf") {
		t.Fatalf("reportText leaked internal data keys: %q", got)
	}
	if got := reportText("Strict kanıt politikası model çıktısını engelledi: kap_pdf_ingest_missing"); !strings.Contains(got, "kap_pdf_ingest_missing") {
		t.Fatalf("strict evidence blocker code should stay auditable, got %q", got)
	}
	if got := reportLabel("bist_official_unprocessed_ohlcv"); got != "BIST resmi günlük bülten OHLCV" {
		t.Fatalf("reportLabel(bist_official_unprocessed_ohlcv) = %q", got)
	}
}

func TestRetailTextKeepsMoneyFlowGenitiveReadable(t *testing.T) {
	got := retailText("411.25 üstü kapanış; para akışının pozitife dönmesi")
	if strings.Contains(got, "ilgisinın") || !strings.Contains(got, "alıcı ilgisinin pozitife dönmesi") {
		t.Fatalf("retailText produced malformed genitive money-flow text: %q", got)
	}
}

func TestHydrateBISTOfficialCoverageRemovesStaleMissingInput(t *testing.T) {
	const key = "bist_official_unprocessed_ohlcv"
	result := analysis.SymbolAnalysis{
		Symbol:    "ASELS",
		AssetType: ohlcv.AssetTypeEquity,
		BISTBulletin: analysis.BISTBulletinContext{
			Computed: true,
			Warnings: []string{"bist_official_bulletin_records_used_for_validation"},
		},
		Professional: professional.Report{
			Coverage: professional.CoverageReport{
				Available: []string{"financial_statements"},
				Missing:   []string{key},
				Score:     50,
				Warnings:  []string{"bist_official_ohlcv_missing"},
			},
		},
	}

	got := normalizeReportCoverageScores(hydrateBISTOfficialCoverage(result))
	if reportTestHasString(got.Professional.Coverage.Missing, key) {
		t.Fatalf("computed BIST official bulletin must not remain missing: %+v", got.Professional.Coverage)
	}
	if !reportTestHasString(got.Professional.Coverage.Available, key) {
		t.Fatalf("computed BIST official bulletin must be available: %+v", got.Professional.Coverage)
	}
	if reportTestHasString(got.Professional.Coverage.Warnings, "bist_official_ohlcv_missing") {
		t.Fatalf("stale BIST missing warning leaked after computed bulletin: %+v", got.Professional.Coverage)
	}
	if !reportTestHasString(got.Professional.Coverage.Warnings, "bist_official_bulletin_records_used_for_validation") {
		t.Fatalf("active BIST validation warning missing: %+v", got.Professional.Coverage)
	}
	if got.Professional.Coverage.Score != 100 || got.Professional.DataQuality != 100 {
		t.Fatalf("coverage score must be recalculated after stale missing input is removed: %+v data_quality=%.1f", got.Professional.Coverage, got.Professional.DataQuality)
	}
}

func TestGDPImpactSummaryShowsStaleObservationWarning(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Professional: professional.Report{
			Market: professional.MarketContext{
				GDP: tamacro.GDPContext{
					Computed:            true,
					LatestYear:          2024,
					ReferenceYear:       2026,
					ObservationLagYears: 2,
					FreshnessStatus:     "stale_annual_actual",
					PerCapitaUSD:        15325,
					PerCapitaTRY:        503075,
					GDPThousandTRY:      44587225441,
					Score:               59,
					Regime:              "büyüme ve dolar bazlı gelir destekleyici",
					DataQualityWarning:  "GSYH dosyası 2026 yılında çekilmiş olsa da son gerçekleşen gözlem 2024.",
				},
			},
		},
	}

	got := gdpImpactSummary(result)
	for _, want := range []string{"referans yıl 2026", "son gerçekleşen gözlem 2024", "2 yıl gözlem gecikmesi", "Uyarı:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("GDP stale context missing %q from %q", want, got)
		}
	}
}

func TestHydrateDerivedReportFieldsLoadsVAPAndForecastForLegacyResult(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	vapDir := filepath.Join(root, "macro", "vap", "fiili_dolasim")
	if err := os.MkdirAll(vapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []any{"Tarih", "ISIN", "ISIN Açıklama", "Borsa Kodu", "İhraççı Üye", "Fiili Dolaşımdaki Pay Adedi", "İhraççı Sermaye", "Fiili Pay/Sermaye Oranı (%)"}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		t.Fatal(err)
	}
	rows := [][]any{
		{"18.06.2026", "X", "ASELSAN", "ASELS", "X", 300.0, 1000.0, 30.0},
		{"19.06.2026", "X", "ASELSAN", "ASELS", "X", 315.0, 1000.0, 31.5},
	}
	for index, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(filepath.Join(vapDir, "Fiili_Dolasim_Raporu_MKK-ASELS.xlsx")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	indexPayload := `{"records":[{"year_month":"2026-05","endeks":"BIST 100","portfoy_deger_mtl":1000},{"year_month":"2026-06","endeks":"BIST 100","portfoy_deger_mtl":1100}]}`
	if err := os.WriteFile(filepath.Join(root, "macro", "vap", "bist_endeks_portfoy.json"), []byte(indexPayload), 0o644); err != nil {
		t.Fatal(err)
	}

	candles := make([]ohlcv.Candle, 0, 25)
	price := 100.0
	start := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 25; i++ {
		open := price * 1.001
		closePrice := open * 1.002
		candles = append(candles, ohlcv.Candle{
			Time: start.AddDate(0, 0, i), Open: open, High: closePrice + 1, Low: open - 1, Close: closePrice, Volume: 1_000_000,
		})
		price = closePrice
	}
	candles[len(candles)-1].Time = time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	result := analysis.SymbolAnalysis{
		Symbol:       "ASELS",
		AssetType:    ohlcv.AssetTypeEquity,
		AnalysisDate: "2026-06-21_12-00-00",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe: "1D", Candles: candles, LastClose: candles[len(candles)-1].Close, TrendBias: "bullish",
				Indicators: ohlcv.IndicatorSnapshot{ATR14: 2.5, RSI14: 61, MACDHistogram: 0.4, ChaikinMoneyFlow20: 0.10, EMA20: price - 2, Supertrend: price - 4},
			},
		},
		Professional: professional.Report{Company: professional.CompanyProfile{Sector: "TEKNOLOJİ", Industry: "SAVUNMA"}},
	}
	hydrated := hydrateDerivedReportFields(equitiesDir, result)
	if !hydrated.Professional.VAPFreeFloat.Computed || hydrated.Professional.VAPFreeFloat.FreeFloatRatioPct != 31.5 {
		t.Fatalf("VAP free float was not hydrated from XLSX: %+v", hydrated.Professional.VAPFreeFloat)
	}
	if !hydrated.Professional.VAPIndexPortfolio.Computed || hydrated.Professional.VAPIndexPortfolio.PortfolioValueMTL != 1100 {
		t.Fatalf("VAP index portfolio was not hydrated: %+v", hydrated.Professional.VAPIndexPortfolio)
	}
	if !hydrated.NextSessionForecast.Computed || hydrated.NextSessionForecast.PredictedOpen <= 0 || hydrated.NextSessionForecast.PredictedClose <= 0 {
		t.Fatalf("next-session forecast was not hydrated: %+v", hydrated.NextSessionForecast)
	}
}

func TestSanitizeUnpublishedPointForecastsSuppressesFutureScenarioPrices(t *testing.T) {
	result := analysis.SymbolAnalysis{
		NextSessionForecast: analysis.NextSessionForecast{
			Computed:                true,
			LastClose:               100,
			PredictedOpen:           101,
			PredictedClose:          102,
			RawPredictedOpen:        100.8,
			RawPredictedClose:       102.2,
			OpenChangePct:           1,
			CloseChangePct:          2,
			ExpectedLow:             98,
			ExpectedHigh:            104,
			CloseP10:                99,
			CloseP50:                102,
			CloseP90:                105,
			PredictedOpenDirection:  "yükseliş",
			PredictedCloseDirection: "yükseliş",
			DirectionTolerancePct:   0.05,
			UpsideProbabilityPct:    55,
			FlatProbabilityPct:      10,
			DownsideProbabilityPct:  35,
			DirectionBias:           "yükseliş",
			BiasStrength:            "orta",
			Confidence:              52,
		},
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				NextSessionForecast: analysis.NextSessionForecast{
					Computed:                true,
					LastClose:               100,
					PredictedOpen:           101,
					PredictedClose:          102,
					ExpectedLow:             98,
					ExpectedHigh:            104,
					PredictedOpenDirection:  "yükseliş",
					PredictedCloseDirection: "yükseliş",
					DirectionBias:           "yükseliş",
					BiasStrength:            "orta",
					UpsideProbabilityPct:    55,
					Confidence:              52,
				},
			},
		},
	}

	got := sanitizeUnpublishedPointForecasts(result)
	if got.NextSessionForecast.PredictedOpen != 0 || got.NextSessionForecast.PredictedClose != 0 ||
		got.NextSessionForecast.RawPredictedOpen != 0 || got.NextSessionForecast.RawPredictedClose != 0 {
		t.Fatalf("blocked future forecast prices should be suppressed: %+v", got.NextSessionForecast)
	}
	if got.NextSessionForecast.ExpectedLow != 0 || got.NextSessionForecast.CloseP50 != 0 ||
		got.NextSessionForecast.ForecastDistributionSamples != 0 {
		t.Fatalf("blocked scenario band/distribution should be suppressed: %+v", got.NextSessionForecast)
	}
	if got.NextSessionForecast.PredictedOpenDirection != "" || got.NextSessionForecast.PredictedCloseDirection != "" ||
		got.NextSessionForecast.DirectionBias != "" || got.NextSessionForecast.BiasStrength != "" ||
		got.NextSessionForecast.UpsideProbabilityPct != 0 || got.NextSessionForecast.DirectionTolerancePct != 0 {
		t.Fatalf("blocked future scenario direction should be suppressed: %+v", got.NextSessionForecast)
	}
	if got.NextSessionForecast.PointForecastPublishable ||
		got.NextSessionForecast.PublishedPredictedOpen != nil ||
		got.NextSessionForecast.PublishedPredictedClose != nil {
		t.Fatalf("non decision-grade forecast must not populate published point fields: %+v", got.NextSessionForecast)
	}
	daily := got.Timeframes["1D"].NextSessionForecast
	if daily.PredictedOpen != 0 || daily.PredictedClose != 0 || daily.PredictedCloseDirection != "" || daily.DirectionBias != "" {
		t.Fatalf("timeframe blocked future scenario forecast should be suppressed: %+v", daily)
	}
}

func TestSanitizeUnpublishedPointForecastsKeepsDecisionGradePointPrices(t *testing.T) {
	forecast := analysis.NextSessionForecast{
		Computed:                    true,
		LastClose:                   100,
		PredictedOpen:               101,
		PredictedClose:              102,
		PredictedOpenDirection:      "yükseliş",
		PredictedCloseDirection:     "yükseliş",
		DirectionBias:               "yükseliş",
		BiasStrength:                "orta",
		Status:                      "mathematically_consistent",
		Quality:                     "technical_model",
		TechnicalDecisionStatus:     "pass",
		BacktestSamples:             40,
		BacktestDirectionHitRatePct: 60,
		BacktestCloseMAEPct:         1.0,
		Confidence:                  62,
	}

	got := sanitizeUnpublishedPointForecasts(analysis.SymbolAnalysis{NextSessionForecast: forecast})
	if got.NextSessionForecast.PredictedOpen != 101 || got.NextSessionForecast.PredictedClose != 102 {
		t.Fatalf("decision-grade forecast should keep published point prices: %+v", got.NextSessionForecast)
	}
	if !got.NextSessionForecast.PointForecastPublishable || got.NextSessionForecast.PublishedPredictedOpen == nil || got.NextSessionForecast.PublishedPredictedClose == nil {
		t.Fatalf("decision-grade forecast should expose published point fields: %+v", got.NextSessionForecast)
	}
	if got.NextSessionForecast.PredictedCloseDirection != "yükseliş" || got.NextSessionForecast.DirectionBias != "yükseliş" {
		t.Fatalf("decision-grade forecast should keep direction fields: %+v", got.NextSessionForecast)
	}
}

func TestReportOutputsDoNotLeakAuditPointForecastWhenActualObserved(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:    "ASELS",
		Currency:  "TL",
		AssetType: ohlcv.AssetTypeEquity,
		NextSessionForecast: analysis.NextSessionForecast{
			Computed:                       true,
			ForecastFor:                    "2026-06-23",
			LastClose:                      394.75,
			PredictedOpen:                  397.50,
			PredictedClose:                 401.50,
			RawPredictedOpen:               397.50,
			RawPredictedClose:              401.50,
			OpenChangePct:                  0.70,
			CloseChangePct:                 1.71,
			ExpectedLow:                    385,
			ExpectedHigh:                   408,
			CloseP10:                       388,
			CloseP50:                       398,
			CloseP90:                       407,
			DirectionBias:                  "yükseliş",
			BiasStrength:                   "validasyon zayıf",
			Confidence:                     35,
			ConfidenceLabel:                "low",
			HistoricalSamples:              20,
			Status:                         "model_validation_failed",
			Quality:                        "not_decision_grade",
			BacktestSamples:                60,
			BacktestCloseMAEPct:            2.73,
			BacktestDirectionHitRatePct:    45,
			ValidationStatus:               "actual_session_observed",
			ActualAvailable:                true,
			ActualOpen:                     392,
			ActualClose:                    390,
			ActualOpenErrorPct:             1.40,
			ActualCloseErrorPct:            2.95,
			OpenForecastErrorTL:            5.50,
			CloseForecastErrorTL:           11.50,
			OpenAbsErrorPctVsActual:        1.40,
			CloseAbsErrorPctVsActual:       2.95,
			PredictedOpenDirection:         "yükseliş",
			PredictedCloseDirection:        "yükseliş",
			PointForecastSuppressionReason: "forecast_not_decision_grade",
		},
		Timeframes: map[string]analysis.TimeframeAnalysis{},
		Professional: professional.Report{
			Company: professional.CompanyProfile{Symbol: "ASELS", Name: "ASELSAN"},
		},
	}
	result.NextSessionForecast = analysis.ApplyNextSessionForecastPublishState(result.NextSessionForecast)

	html := professionalReportHTML(result)
	for _, banned := range []string{
		"397.50 TL",
		"401.50 TL",
		"yükseliş | son kapanışa göre",
		"yükseliş / validasyon zayıf",
		"Denetim amaçlı model açılışı",
		"Denetim amaçlı model kapanışı",
		"Ham model açılışı",
		"Ham model kapanışı",
		"hata: açılış",
	} {
		if strings.Contains(html, banned) {
			t.Fatalf("report HTML leaked audit point forecast %q\n%s", banned, html)
		}
	}
	if !strings.Contains(html, "Resmi: 392.00 TL") ||
		!strings.Contains(html, "Resmi: 390.00 TL") ||
		!strings.Contains(html, "Resmi: aşağı") {
		t.Fatalf("report HTML should show official observed result instead of stale model forecast\n%s", html)
	}

	tr := turkishAnalysis(result)
	forecastTR := tr["sonraki_seans_tahmini"].(map[string]any)
	for _, key := range []string{
		"tahmini_acilis",
		"tahmini_kapanis",
		"ham_tahmini_acilis",
		"ham_tahmini_kapanis",
		"gerceklesen_acilis_hata_yuzde",
		"gerceklesen_kapanis_hata_yuzde",
		"acilis_hata_tl",
		"kapanis_hata_tl",
		"acilis_mutlak_hata_yuzde_gercege_gore",
		"kapanis_mutlak_hata_yuzde_gercege_gore",
		"tahmini_acilis_yonu",
		"tahmini_kapanis_yonu",
		"yon_toleransi_yuzde",
		"yukari_olasilik_yuzde",
		"yatay_olasilik_yuzde",
		"asagi_olasilik_yuzde",
		"yon_beklentisi",
		"yon_gucu",
	} {
		if forecastTR[key] != nil {
			t.Fatalf("Turkish analysis leaked %s: %+v", key, forecastTR)
		}
	}
	official := forecastTR["kesinlesmis_resmi_sonuc"].(map[string]any)
	if official["denetim_tahmini_acilis"] != nil ||
		official["denetim_tahmini_kapanis"] != nil ||
		official["denetim_acilis_hata_tl"] != nil ||
		official["denetim_kapanis_hata_tl"] != nil ||
		official["denetim_acilis_hata_yuzde"] != nil ||
		official["denetim_kapanis_hata_yuzde"] != nil {
		t.Fatalf("Turkish official result leaked audit point forecast: %+v", official)
	}

	rows := nextSessionForecastPDFRows(result)
	for _, row := range rows {
		joined := strings.Join(row, " ")
		if strings.Contains(joined, "397.50") || strings.Contains(joined, "401.50") ||
			strings.Contains(joined, "Denetim amaçlı model") || strings.Contains(joined, "yükseliş / validasyon zayıf") {
			t.Fatalf("PDF forecast rows leaked audit point forecast: %+v", rows)
		}
	}
	manifestForecast := reportManifestNextSessionForecast(result)
	for _, key := range []string{
		"predicted_open_direction",
		"predicted_close_direction",
		"direction_tolerance_pct",
		"upside_probability_pct",
		"flat_probability_pct",
		"downside_probability_pct",
		"direction_bias",
		"bias_strength",
	} {
		if manifestForecast[key] != nil {
			t.Fatalf("manifest leaked %s: %+v", key, manifestForecast)
		}
	}
	if got, ok := manifestForecast["direction"].(string); !ok || !strings.Contains(got, "Resmi: aşağı / aşağı") {
		t.Fatalf("manifest should show official observed direction: %+v", manifestForecast)
	}
}

func TestRetailBacktestLineSuppressesUnsafePriceAdjustment(t *testing.T) {
	got := retailBacktestLine(professional.TimeframeReport{
		Backtest: professional.BacktestResult{
			Trades:            17,
			OutOfSampleTrades: 5,
			AverageReturn:     2.4094,
			BacktestSafe:      true,
		},
		PriceAdjustment: professional.PriceAdjustmentReview{BacktestSafe: false},
	})
	if strings.Contains(got, "240") || strings.Contains(got, "ortalama sonuç") {
		t.Fatalf("unsafe backtest line leaked performance result: %q", got)
	}
	if !strings.Contains(got, "karar girdisi yapılmadı") {
		t.Fatalf("unsafe backtest line = %q, want decision-input warning", got)
	}
}

func TestRejectedTradePlanWordingSeparatesTrendFromDecision(t *testing.T) {
	tf := analysis.TimeframeAnalysis{
		LastClose: 397,
		TrendBias: "bullish",
		Score:     54.1,
		TradePlan: ohlcv.TradePlan{
			Rejected:        true,
			RejectReason:    "risk/reward ratio below 1.5",
			RiskRewardRatio: 0.55,
		},
	}

	line := retailTimeframeLine(tf, "TRY")
	if !strings.Contains(line, "teknik görünüm güçlenen") {
		t.Fatalf("trend context missing from line: %q", line)
	}
	if !strings.Contains(line, "işlem kapısı kapalı; RR 0.55 < 1.50") {
		t.Fatalf("decision gate missing from line: %q", line)
	}
	if strings.Contains(line, "güçlenen yön") || strings.Contains(line, "Aktif plan yok") {
		t.Fatalf("old ambiguous wording leaked: %q", line)
	}

	plan := reportPlanText(tf.TradePlan)
	if !strings.Contains(plan, "Kapalı: RR 0.55 < 1.50") {
		t.Fatalf("plan gate text = %q", plan)
	}
}

func TestCryptoRejectedSpotGateUsesActionableWaitLanguage(t *testing.T) {
	tf := analysis.TimeframeAnalysis{
		Timeframe:         "1D",
		LastClose:         0.08477,
		TrendBias:         "bearish",
		Score:             37.9,
		NearestResistance: &ohlcv.SupportResistanceLevel{Price: 0.0885825},
		Professional: professional.TimeframeReport{Technical: professional.TechnicalEvidence{SignalGate: professional.TechnicalSignalGate{
			Blockers: []string{
				"fiyat yapısı: uygulanabilir giriş/stop/hedef planı yok",
				"trade plan reddi: Spot varlıkta short/marjin planı üretilmez; aktif alım planı yok",
				"risk/ödül 0.00; aktif sinyal için en az 1.50 gerekir",
			},
		}}},
		TradePlan: ohlcv.TradePlan{
			Rejected:     true,
			RejectReason: "Spot varlıkta short/marjin planı üretilmez; aktif alım planı yok",
			Reasoning:    []string{"Düşüş yönlü teknik kanıtlar spot varlık için alım kurulumu oluşturmaz."},
		},
	}

	gate := timeframeGateText(tf)
	if !strings.Contains(gate, "0.0886 üstü kapanış") {
		t.Fatalf("gate should show precise confirmation level, got %q", gate)
	}
	for _, banned := range []string{"short", "marjin"} {
		if strings.Contains(strings.ToLower(gate), banned) {
			t.Fatalf("gate leaked internal risk wording %q in %q", banned, gate)
		}
	}

	line := retailTimeframeLine(tf, "USDT")
	if !strings.Contains(line, "0.0848 USDT kapanış") || !strings.Contains(line, "0.0886 üstü kapanış") {
		t.Fatalf("retail timeframe line lost precise price/action language: %q", line)
	}

	blockers := retailSignalGateBlockers(tf, 3)
	if !strings.Contains(blockers, "aktif alım planı yok") || !strings.Contains(blockers, "risk/getiri henüz ölçülemedi") {
		t.Fatalf("signal gate blockers should be actionable retail text: %q", blockers)
	}
	for _, banned := range []string{"short", "marjin"} {
		if strings.Contains(strings.ToLower(blockers), banned) {
			t.Fatalf("signal gate blockers leaked internal risk wording %q in %q", banned, blockers)
		}
	}
}

func TestSortedTimeframeKeysUsesChronologicalOrder(t *testing.T) {
	keys := sortedTimeframeKeys(map[string]analysis.TimeframeAnalysis{
		"1M": {},
		"1D": {},
		"1W": {},
	})
	got := strings.Join(keys, ",")
	if got != "1D,1W,1M" {
		t.Fatalf("timeframe order = %s", got)
	}
}

func TestDailyContextWindowDoesNotLookLikeDirectBuyPlan(t *testing.T) {
	tf := analysis.TimeframeAnalysis{
		Timeframe:   "YTD",
		LastClose:   15.27,
		CandleCount: 118,
		Candles: []ohlcv.Candle{
			{Time: time.Date(2026, 1, 2, 6, 0, 0, 0, time.UTC), Close: 12},
			{Time: time.Date(2026, 6, 19, 6, 0, 0, 0, time.UTC), Close: 15.27},
		},
		TradePlan: ohlcv.TradePlan{
			Direction:       "long",
			EntryMin:        15.1,
			EntryMax:        15.4,
			StopLoss:        14.8,
			TakeProfit1:     16.4,
			TakeProfit2:     17.3,
			RiskRewardRatio: 2.9,
		},
	}

	gate := timeframeGateText(tf)
	if strings.Contains(gate, "Giriş") || !strings.Contains(gate, "günlük veri penceresi") {
		t.Fatalf("YTD gate should be context-only, got %q", gate)
	}
	comment := timeframePlainComment(tf)
	if !strings.Contains(comment, "kapanış son günlük fiyatla aynı olabilir") {
		t.Fatalf("YTD comment should explain repeated close, got %q", comment)
	}
	if got := timeframeLastBarText(tf); got != "2026-06-19" {
		t.Fatalf("last bar text = %s", got)
	}
	if got := timeframeCandleWindowText(tf); !strings.Contains(got, "yıl içi pencere") {
		t.Fatalf("window text = %q", got)
	}
}

func TestActionableTradePlanRequiresDecisionTimeframeAndPassingGates(t *testing.T) {
	plan := ohlcv.TradePlan{
		Direction:       "long",
		EntryMin:        0.2249,
		EntryMax:        0.2378,
		StopLoss:        0.2245,
		TakeProfit1:     0.2749,
		TakeProfit2:     0.3119,
		RiskRewardRatio: 6.32,
		Quality:         "high",
		ConfidenceScore: 0.78,
	}
	blockedResult := analysis.SymbolAnalysis{
		InstitutionalValidation: analysis.InstitutionalValidation{Status: "limited"},
	}

	ytd := actionableTradePlan("YTD", blockedResult, plan)
	if ytd["action"] != "CONTEXT_ONLY" || ytd["execution_ready"] != false || ytd["context_only"] != true {
		t.Fatalf("YTD plan must be context-only and not executable: %+v", ytd)
	}

	dailyBlocked := actionableTradePlan("1D", blockedResult, plan)
	if dailyBlocked["action"] != "WAIT" || dailyBlocked["execution_ready"] != false || dailyBlocked["blocked_by_report_gate"] != true {
		t.Fatalf("blocked daily plan must not be execution-ready: %+v", dailyBlocked)
	}

	dailyResearchFixture := actionableTradePlan("1D", analysis.SymbolAnalysis{}, plan)
	if dailyResearchFixture["action"] != "BUY_ON_CONFIRMATION" || dailyResearchFixture["execution_ready"] != true {
		t.Fatalf("ungated fixture should preserve raw actionable plan semantics: %+v", dailyResearchFixture)
	}
}

func TestLongTimeframeRejectedPlanUsesContextWording(t *testing.T) {
	tf := analysis.TimeframeAnalysis{
		Timeframe: "1M",
		TrendBias: "bullish",
		TradePlan: ohlcv.TradePlan{
			Rejected:        true,
			RejectReason:    "Risk/getiri oranı 1,5 seviyesinin altında",
			RiskRewardRatio: 0.49,
		},
	}

	gate := timeframeGateText(tf)
	if !strings.Contains(gate, "uzun dönem bağlamı") || !strings.Contains(gate, "RR 0.49 < 1.50") {
		t.Fatalf("monthly gate = %q", gate)
	}
	comment := timeframePlainComment(tf)
	if !strings.Contains(comment, "Uzun dönem bar devam ediyor") || !strings.Contains(comment, "günlük/haftalık teyit gerekir") {
		t.Fatalf("monthly comment = %q", comment)
	}
}

func TestProfessionalReportHTMLIncludesTechnicalAppendix(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:       "TEST",
		AssetType:    ohlcv.AssetTypeEquity,
		CompanyName:  "Test Sirketi",
		AnalysisDate: "2026-06-13",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe:   "1D",
				LastClose:   100,
				LastVolume:  1_250_000,
				CandleCount: 120,
				Indicators: ohlcv.IndicatorSnapshot{
					SMA20:              98,
					SMA50:              96,
					RSI14:              57,
					MACD:               1.2,
					MACDSignal:         0.9,
					MACDHistogram:      0.3,
					ATR14:              4.5,
					ChaikinMoneyFlow20: 0.12,
				},
				IndicatorSignals: []ohlcv.IndicatorResult{
					{Name: "RSI", Category: "momentum", Signal: "bullish", Value: 57, Confidence: 0.71, Computed: true, Source: "snapshot.rsi14", Evidence: []string{"momentum oscillator value evaluated"}},
					{Name: "Funding Rate", Category: "sentiment", Signal: "requires_external_data", Confidence: 0.55, Computed: false, Source: "external_data_required", Evidence: []string{"requires external data not available in current scanner input"}},
				},
				Patterns: []ohlcv.PatternResult{
					{Name: "Bullish Engulfing", Category: "candlestick", Direction: "bullish", Confidence: 0.74, Actionable: true, BacktestReady: true, VolumeConfirmed: true, VolumeConfirmation: "confirmed", ValidationStatus: "statistically_validated", BacktestSampleSize: 42, BacktestWinRate: 0.58, BacktestExpectancyR: 0.41, Evidence: []string{"candlestick geometry matched the named setup"}, EntryMin: 100, EntryMax: 102, StopLoss: 96, Target1: 108, Target2: 114, RiskRewardRatio: 2},
				},
				PatternCandidates: []ohlcv.PatternResult{
					{Name: "Double Top", Category: "chart", Direction: "bearish", Confidence: 0.63, RejectionReasons: []string{"not_current_completed_pattern"}, Evidence: []string{"two adjacent wide candles form a top"}},
					{Name: "Flag", Category: "chart", Direction: "bullish", Confidence: 0.61, RejectionReasons: []string{"calibrated_confidence_below_threshold"}, Evidence: []string{"flag geometry is present but confirmation is weak"}},
				},
				PatternScans: []ohlcv.PatternScanResult{
					{Name: "Bullish Engulfing", Matched: true, Actionable: true},
				},
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{
						Summary: "test technical summary",
						Score:   professional.TechnicalScore{Trend: 12, Momentum: 10, Volume: 8, VolatilityRisk: 7, Pattern: 10, Total: 47},
						SignalCounts: map[string]int{
							"total":     2,
							"computed":  1,
							"not_ready": 1,
						},
						PatternCounts: map[string]int{
							"catalog":   1,
							"matched":   1,
							"confirmed": 1,
						},
					},
				},
				TrendBias: "bullish",
				Score:     64,
			},
		},
		Disclaimer: ohlcv.Disclaimer,
	}
	result.NextSessionForecast = analysis.NextSessionForecast{
		Computed: true, ForecastFor: "2026-06-22", LastClose: 100, PredictedOpen: 101.25, PredictedClose: 102.40,
		OpenChangePct: 1.25, CloseChangePct: 2.40, ExpectedLow: 97, ExpectedHigh: 104, DirectionBias: "yükseliş",
		PredictedOpenDirection: "yükseliş", PredictedCloseDirection: "yükseliş", DirectionTolerancePct: 0.05,
		BiasStrength: "orta", Confidence: 64, HistoricalSamples: 20, Model: "atr_gap_intraday_ewma_v1", BiasReasons: []string{"RSI yükselen momentum"},
	}
	result.Professional.VAPFreeFloat.Computed = true
	result.Professional.VAPFreeFloat.LatestDate = "2026-06-19"
	result.Professional.VAPFreeFloat.FreeFloatRatioPct = 31.5
	result.Professional.VAPFreeFloat.RatioChange20DPP = 0.75
	result.Professional.VAPFreeFloat.LiquidityRisk = "orta"
	result.Professional.VAPFreeFloat.SupplySignal = "artan arz"
	result.Professional.VAPFreeFloat.Summary = "VAP fiili dolaşım özeti"
	result.Professional.VAPIndexPortfolio.Computed = true
	result.Professional.VAPIndexPortfolio.SelectedIndex = "BIST 100"
	result.Professional.VAPIndexPortfolio.LatestMonth = "2026-05"
	result.Professional.VAPIndexPortfolio.PortfolioValueMTL = 1234.5
	result.Professional.VAPIndexPortfolio.Change1MPct = 3.2
	result.Professional.VAPIndexPortfolio.RelativeMomentum = 1.1
	result.Professional.VAPIndexPortfolio.Signal = "pozitif"
	result.Professional.VAPIndexPortfolio.Summary = "VAP endeks portföyü özeti"
	turkish := turkishAnalysis(result)
	forecastTR, ok := turkish["sonraki_seans_tahmini"].(map[string]any)
	if !ok {
		t.Fatalf("Turkish analysis forecast missing: %+v", turkish["sonraki_seans_tahmini"])
	}
	if forecastTR["tahmini_acilis"] != nil || forecastTR["tahmini_kapanis"] != nil {
		t.Fatalf("Turkish analysis must not expose blocked scenario prices as published forecasts: %+v", forecastTR)
	}
	if forecastTR["senaryo_acilis"] != nil || forecastTR["senaryo_kapanis"] != nil ||
		forecastTR["beklenen_en_dusuk"] != nil || forecastTR["kapanis_p50"] != nil {
		t.Fatalf("Turkish analysis must suppress blocked future scenario prices/bands: %+v", forecastTR)
	}
	if forecastTR["gerceklesen_var"] != false || forecastTR["gerceklesen_acilis"] != nil || forecastTR["gerceklesen_kapanis"] != nil {
		t.Fatalf("Turkish analysis must keep unavailable actual prices null: %+v", forecastTR)
	}
	if forecastTR["islem_gorebilir_acilis_degisim_yuzde"] != nil || forecastTR["islem_gorebilir_kapanis_degisim_yuzde"] != nil {
		t.Fatalf("Turkish analysis must not publish unvalidated tradable forecast pct: %+v", forecastTR)
	}
	if forecastTR["tahmini_acilis_yonu"] != nil || forecastTR["tahmini_kapanis_yonu"] != nil {
		t.Fatalf("Turkish analysis must not expose blocked scenario directions as published forecasts: %+v", forecastTR)
	}
	if forecastTR["senaryo_acilis_yonu"] != nil || forecastTR["senaryo_kapanis_yonu"] != nil ||
		forecastTR["yon_beklentisi"] != nil || forecastTR["yon_gucu"] != nil {
		t.Fatalf("Turkish analysis must suppress blocked future scenario direction: %+v", forecastTR)
	}
	if forecastTR["nokta_tahmin_yayinlanabilir"] != false || forecastTR["yayinlanan_tahmini_acilis"] != nil || forecastTR["yayinlanan_tahmini_kapanis"] != nil {
		t.Fatalf("Turkish analysis must suppress point forecast without backtest validation: %+v", forecastTR)
	}
	if forecastTR["nokta_tahmin_yayin_durumu"] != "yayinlanmadi" ||
		!strings.Contains(fmt.Sprint(forecastTR["nokta_tahmin_bastirma_nedeni"]), "Fiyat/yön senaryosu yayınlanmadı") {
		t.Fatalf("Turkish analysis must explain blocked forecast publication: %+v", forecastTR)
	}
	professionalTR, ok := turkish["profesyonel_analiz"].(map[string]any)
	if !ok || professionalTR["vap_fiili_dolasim"] == nil || professionalTR["vap_bist_endeks_portfoyu"] == nil {
		t.Fatalf("Turkish analysis VAP sections missing: %+v", turkish["profesyonel_analiz"])
	}

	html := professionalReportHTML(result)
	for _, want := range []string{
		"Tek Bakış Karar Paneli",
		"Yatırım tavsiyesi değildir",
		"ol-grid",
		"Son Kapanış",
		"Açılış Fiyat Senaryosu",
		"Kapanış Fiyat Senaryosu",
		"Yayınlanmadı",
		"Ertesi Kapanış Yönü",
		"Beklenen kapanış yönü",
		"Ertesi Seans Yönü",
		"Bir Sonraki Seans Senaryo ve Risk Kapısı",
		"Senaryo kullanım durumu",
		"Fiyat/yön senaryosu yayınlanmadı",
		"Açılış fiyat senaryosu",
		"VAP / MKK Piyasa Yapısı",
		"Fiili dolaşım oranı",
		"31.50%",
		"BIST Endeks Portföyü",
		"VAP endeks portföyü özeti",
		"Teknik Ek - Günlük",
		"Hesaplanan İndikatör Sinyalleri",
		"RSI",
		"Aktif Formasyon Sonuçları",
		"Yükseliş Yutan",
		"İşlem Sinyali Olmayan İzleme / Elenen Formasyonlar",
		"Bayrak",
		"Kalibre güven eşiğin altında",
		"Veriye Dayalı Okuma Notları",
		"Günlük MACD histogramı 0.3000",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report HTML does not contain %q\n%s", want, html)
		}
	}
	md := markdownSummary(result)
	for _, want := range []string{
		"Bir Sonraki Seans Beklentisi",
		"Senaryo durumu: Fiyat/yön senaryosu yayınlanmadı",
		"Açılış fiyat senaryosu: Yayınlanmadı",
		"Kapanış fiyat senaryosu: Yayınlanmadı",
		"Kapanış dağılımı P10/P50/P90",
		"VAP / MKK Piyasa Yapısı",
		"Fiili dolaşım oranı: 31.50%",
		"BIST Endeks Portföyü: BIST 100 / 2026-05",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("summary markdown does not contain %q\n%s", want, md)
		}
	}
	fallback := result
	fallback.NextSessionForecast = analysis.NextSessionForecast{}
	daily := fallback.Timeframes["1D"]
	daily.NextSessionForecast = result.NextSessionForecast
	fallback.Timeframes["1D"] = daily
	fallbackTR := turkishAnalysis(fallback)
	fallbackForecastTR, ok := fallbackTR["sonraki_seans_tahmini"].(map[string]any)
	if !ok {
		t.Fatalf("Turkish analysis forecast fallback missing: %+v", fallbackTR["sonraki_seans_tahmini"])
	}
	if fallbackForecastTR["tahmini_acilis"] != nil || fallbackForecastTR["tahmini_kapanis"] != nil {
		t.Fatalf("Turkish analysis fallback must not expose blocked scenario prices as published forecasts: %+v", fallbackForecastTR)
	}
	if fallbackForecastTR["senaryo_acilis"] != nil || fallbackForecastTR["senaryo_kapanis"] != nil {
		t.Fatalf("Turkish analysis fallback must suppress blocked future scenario prices: %+v", fallbackForecastTR)
	}
	if fallbackForecastTR["nokta_tahmin_yayinlanabilir"] != false || fallbackForecastTR["yayinlanan_tahmini_acilis"] != nil || fallbackForecastTR["yayinlanan_tahmini_kapanis"] != nil {
		t.Fatalf("Turkish analysis published forecast missing: %+v", fallbackTR["sonraki_seans_tahmini"])
	}
	if strings.Contains(html, "MACD negatif yazıyorsa") {
		t.Fatalf("static MACD reading note leaked into dynamic report")
	}
	if strings.Contains(html, "Funding Rate") {
		t.Fatalf("non-computed external indicator leaked into computed indicator report")
	}
	if strings.Contains(html, "Son tamamlanan mumda güncel formasyon değil") {
		t.Fatalf("stale pattern candidate leaked into report")
	}
	if strings.Contains(html, "tek_bakis_ozet.png") || strings.Contains(html, "summary-visual") {
		t.Fatalf("HTML should render one-look summary natively, not as an embedded image")
	}
	if strings.Contains(html, "<th>Kapanış</th>") {
		t.Fatalf("timeframe table should label repeated close as Son Kapanış")
	}
	decisionIdx := strings.Index(html, "Karar Özeti")
	verificationIdx := strings.Index(html, "Doğrulama Skoru")
	technicalIdx := strings.Index(html, "Teknik Ek - Günlük")
	if decisionIdx < 0 || verificationIdx < 0 || technicalIdx < 0 {
		t.Fatalf("expected decision, verification and technical sections in report")
	}
	if technicalIdx < verificationIdx || technicalIdx < decisionIdx {
		t.Fatalf("technical appendix should be rendered after decision/verification sections")
	}
}

func TestProfessionalReportHTMLIncludesBuffettChecklist(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:       "BUFF",
		CompanyName:  "Buffett Test A.S.",
		AssetType:    ohlcv.AssetTypeEquity,
		AnalysisDate: "2026-06-22",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {Timeframe: "1D", LastClose: 100, Score: 50},
		},
		Professional: professional.Report{
			DataQuality: 75,
			Coverage:    professional.CoverageReport{Score: 75},
			Company:     professional.CompanyProfile{Symbol: "BUFF", Name: "Buffett Test A.S.", Sector: "Sanayi"},
			ValueInvesting: value.Report{
				Computed:      true,
				CurrentPrice:  100,
				DecisionLabel: "BEKLE",
				Summary:       "Güvenlik marjı ve owner earnings denetimi tamamlanmadan AL kararı yok.",
				SectorModel:   value.SectorModelReport{Model: "owner_earnings_dcf", Label: "Owner earnings DCF", Reason: "Operasyonel şirket modeli."},
				IntrinsicValue: value.IntrinsicValueReport{
					Computed: true,
					Bear:     90,
					Base:     120,
					Bull:     150,
				},
				FairValue: value.FairValueConclusion{
					Computed:            true,
					Label:               "İçsel değerin altında fakat güvenlik marjı sınırlı",
					PriceToFairValuePct: -16.7,
					UpsideDownsidePct:   20,
					RequiredMarginPct:   25,
				},
				MarginOfSafety: value.MarginOfSafetyReport{Computed: true, BasePct: 16.7, RequiredPct: 25},
				OwnerEarnings:  value.OwnerEarningsReport{Applicable: true, TTM: 1000, Normalized5Y: 900, Score: 65},
				NormalizedFCF:  value.NormalizedFCFReport{Applicable: true, TTM: 800, Median5Y: 750, Median10Y: 700, Score: 62},
				CapitalAllocation: value.CapitalAllocationReport{
					Score:           58,
					Dilution5YPct:   12,
					NetDebtToEquity: 0.3,
				},
				Moat:        value.MoatReport{Label: "orta_moat", AverageROE5Y: 0.14, AverageROIC5Y: 0.11, GrossMarginMedian5Y: 0.36, NetMarginMedian5Y: 0.13, MarginStability: 0.70, Score: 57},
				Assumptions: value.Assumptions{DiscountRate: 0.18, TerminalGrowth: 0.05, OwnerEarningsGrowth: 0.08, TaxRate: 0.25},
				BuffettChecklist: value.BuffettChecklistReport{
					Computed:    true,
					Status:      "fail",
					StatusLabel: "Buffett filtresi başarısız",
					Score:       48,
					CoveragePct: 80,
					Summary:     "Buffett filtresi başarısız: 4 engel var.",
					BlockingIssues: []string{
						"one_dollar_retained_earnings_test",
						"margin_of_safety_limited",
					},
					MissingData: []string{"retained_earnings_history", "market_cap_history"},
					Requirements: []value.BuffettChecklistRequirement{
						{ID: "business_model_understandable", Pillar: "business", Label: "İş modeli ve sektör modeli anlaşılır mı?", Status: "pass", Required: true, Value: "Owner earnings DCF"},
						{ID: "one_dollar_retained_earnings_test", Pillar: "management", Label: "1 Dolar dağıtılmamış kâr testi", Status: "missing", Required: true, Threshold: ">= 1.0", Missing: []string{"retained_earnings_history", "market_cap_history"}},
						{ID: "margin_of_safety", Pillar: "market", Label: "Güvenlik marjı", Status: "limited", Required: true, Value: "16.7%", Threshold: ">= 25.0%"},
					},
				},
			},
		},
		Disclaimer: ohlcv.Disclaimer,
	}

	html := professionalReportHTML(result)
	for _, want := range []string{
		"Buffett / Değer Yatırımı Gereksinim Matrisi",
		"Buffett filtresi başarısız",
		"1 Dolar dağıtılmamış kâr testi",
		"dağıtılmamış kâr geçmişi",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report HTML missing %q\n%s", want, html)
		}
	}
}

func TestWriteAnalysisCreatesProfessionalResearchArtifacts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "equities")
	result := analysis.SymbolAnalysis{
		Symbol:       "ARTF",
		CompanyName:  "Artifact Test A.S.",
		AssetType:    ohlcv.AssetTypeEquity,
		AnalysisDate: "2026-06-15",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe:  "1D",
				LastClose:  10,
				LastVolume: 1000000,
				Score:      52,
				TrendBias:  "neutral",
				Indicators: ohlcv.IndicatorSnapshot{RSI14: 55, MACD: 0.4, MACDSignal: 0.2, ATR14: 0.7, ChaikinMoneyFlow20: 0.08},
				TradePlan: ohlcv.TradePlan{
					Direction:       "long",
					EntryMin:        10,
					EntryMax:        10.2,
					TakeProfit1:     11.5,
					TakeProfit2:     12.4,
					StopLoss:        9.4,
					RiskRewardRatio: 2.1,
					Quality:         "good",
					ConfidenceScore: 0.72,
				},
			},
		},
		Professional: professional.Report{
			DataQuality: 70,
			Coverage:    professional.CoverageReport{Score: 72},
			Company:     professional.CompanyProfile{Symbol: "ARTF", Name: "Artifact Test A.S.", Sector: "Sanayi", Industry: "Test"},
			Valuation: professional.ValuationAnalysis{
				MarketCap: 1000,
				NetDebt:   100,
				Equity:    700,
				Ratios:    map[string]float64{"PB": 1.2, "ROE": 0.08},
			},
			Peers: professional.PeerComparison{PeerCount: 3, Sector: "Sanayi", ValuationSignal: "neutral"},
			KAPPDFIngest: professional.KAPPDFIngestSummary{
				Computed:            true,
				TotalDocuments:      3,
				AnalysisUsableCount: 2,
				ReviewRequiredCount: 1,
				AverageQuality:      0.78,
			},
			InvestmentResearch: professional.InvestmentResearchReview{
				Computed: true,
				Summary:  "Artifact test summary.",
				InstitutionalMemo: professional.InstitutionalMemo{
					Recommendation:         "WATCH",
					WorkflowStatus:         "research_backlog_or_watchlist",
					ReadinessScore:         45,
					PositionSizeSuggestion: "0%; takip listesi.",
					InvestmentHorizon:      "6-18 ay",
					ExpectedReturnPct:      10,
					DownsideRiskPct:        -8,
					RiskRewardRatio:        1.25,
					LiquidityConsideration: "ADV kontrolü gerekli.",
					PortfolioFit:           "Sektör yoğunluğu kontrol edilmeli.",
					KeyAssumptions:         []string{"Baz değer kaynaklıdır."},
					ApprovalConditions:     []string{"Kanıt tamamlanır."},
					RejectionConditions:    []string{"Tez bozulur."},
					BlockingIssues:         []string{"tam_nad_mutabakati_yok"},
					RequiredFixes:          []string{"NAD köprüsü tamamlanmalı."},
					PositiveSignals:        []string{"PDF kapsamı var."},
				},
				InvestmentStory: professional.InvestmentStory{
					CoreThesis:         "Test thesis.",
					ValueSource:        "Faaliyet karlılığı.",
					MispricingQuestion: "Piyasa kaliteyi doğru fiyatlıyor mu?",
				},
				ValuationBridge: professional.ValuationTransparency{Model: "test", Formula: "base case", CurrentPrice: 10, BaseIntrinsicValue: 11, NAVStatus: "not_applicable"},
				DecisionFramework: professional.DecisionFramework{
					CurrentDecision: "BEKLE",
					DecisionBasis:   []string{"Veri kalite kontrolü gerekiyor."},
					BuyConditions:   []string{"Kanıt tamamlanır."},
					SellConditions:  []string{"Tez bozulur."},
				},
				FinancialQuality: professional.FinancialQualityBridge{Summary: "Test financial quality."},
			},
			RawKAPData: &professional.KAPRawDataBundle{
				Computed: true,
				Symbol:   "ARTF",
				SourceFiles: professional.KAPRawDataSourceFiles{
					FinancialFactsPath:  "data/processed/by_ticker/ARTF/financial_facts.jsonl",
					FinancialTablesPath: "data/processed/by_ticker/ARTF/financial_tables.jsonl",
				},
				FinancialFacts: []kapingest.ExtractedFinancialFact{
					{
						ID:                 "fact-1",
						StatementType:      "income_statement",
						LineItemOriginal:   "Hasılat",
						LineItemNormalized: "revenue",
						Value:              1234,
						Currency:           "TRY",
						Unit:               "thousand",
						Period:             stringPtrForReportTest("2025/12"),
						SourceFile:         "ARTF_finansal.pdf",
						Source:             kapingest.DocumentFactSource{Page: 12, TableID: "gelir_tablosu", Snippet: "Hasılat 1.234"},
						Confidence:         0.89,
					},
				},
				FinancialTables: []kapingest.ExtractedFinancialTable{
					{
						ID:         "table-1",
						TableType:  "income_statement",
						Period:     stringPtrForReportTest("2025/12"),
						SourceFile: "ARTF_finansal.pdf",
						Source:     kapingest.DocumentFactSource{Page: 12, TableID: "gelir_tablosu", Snippet: "Kar veya zarar tablosu"},
						Confidence: 0.91,
						Rows:       []kapingest.FinancialTableRow{{RowIndex: 1, Cells: []string{"Hasılat", "1.234"}, Snippet: "Hasılat 1.234"}},
					},
				},
			},
		},
		Disclaimer: ohlcv.Disclaimer,
	}

	writeStructuredFinancialsForReportTest(t, root, "ARTF")

	writer := NewReportWriterWithOptions(ReportWriterOptions{RenderPDF: false})
	if err := writer.WriteAnalysis(context.Background(), root, result); err != nil {
		t.Fatalf("WriteAnalysis error = %v", err)
	}
	dir := AnalysisDirForAsset(root, ohlcv.AssetTypeEquity, "ARTF", "2026-06-15")
	for _, name := range []string{
		"investment_committee_memo.json",
		"quality_control_report.json",
		"source_evidence_index.json",
		"kap_pdf_reportable_data_index.json",
		"kap_pdf_financial_analysis.json",
		"valuation_model.json",
		"buffett_value_checklist.json",
		"quant_risk_report.json",
		"stat_economic_report.json",
		"technical_trade_plan.json",
		"tek_bakis_ozet.png",
		"rapor_veri_manifesti.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing artifact %s: %v", name, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "investment_committee_memo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var memo map[string]any
	if err := json.Unmarshal(raw, &memo); err != nil {
		t.Fatalf("invalid memo json: %v", err)
	}
	body := memo["memo"].(map[string]any)
	if body["recommendation"] != "REDDET" {
		t.Fatalf("unexpected recommendation: %+v", body)
	}
	if memo["position_size_suggestion"] == "" || memo["risk_reward_ratio"].(float64) == 0 {
		t.Fatalf("committee memo top-level fields missing: %+v", memo)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "financial_statements_normalized.json"))
	if err != nil {
		t.Fatal(err)
	}
	var financials map[string]any
	if err := json.Unmarshal(raw, &financials); err != nil {
		t.Fatalf("invalid financial statements json: %v", err)
	}
	if len(financials["normalized_metric_rows"].([]any)) == 0 || len(financials["normalized_table_rows"].([]any)) == 0 {
		t.Fatalf("normalized financial rows missing: %+v", financials)
	}
	coverage := financials["statement_coverage"].(map[string]any)
	if coverage["structured_financial_rows"].(float64) == 0 {
		t.Fatalf("structured financial rows missing from coverage: %+v", coverage)
	}
	certificationSummary := coverage["certification_summary"].(map[string]any)
	if certificationSummary["financial_facts_review"].(float64) == 0 {
		t.Fatalf("financial certification summary missing review gate: %+v", certificationSummary)
	}
	metricRow := financials["normalized_metric_rows"].([]any)[0].(map[string]any)
	if _, ok := metricRow["certification"]; !ok {
		t.Fatalf("normalized metric row missing certification: %+v", metricRow)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "kap_pdf_reportable_data_index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reportable map[string]any
	if err := json.Unmarshal(raw, &reportable); err != nil {
		t.Fatalf("invalid reportable pdf data json: %v", err)
	}
	categories := reportable["reportable_categories"].([]any)
	if len(categories) == 0 {
		t.Fatalf("reportable category catalog missing: %+v", reportable)
	}
	foundFinancialFacts := false
	for _, categoryRaw := range categories {
		category := categoryRaw.(map[string]any)
		if category["kind"] == "financial_facts" && category["count"].(float64) == 1 && category["full_export_available"].(bool) {
			foundFinancialFacts = true
		}
	}
	if !foundFinancialFacts {
		t.Fatalf("financial facts not reportable in catalog: %+v", categories)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "technical_trade_plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var technical map[string]any
	if err := json.Unmarshal(raw, &technical); err != nil {
		t.Fatalf("invalid technical trade plan json: %v", err)
	}
	plans := technical["plans"].([]any)
	firstPlan := plans[0].(map[string]any)
	actionable := firstPlan["actionable_plan"].(map[string]any)
	if actionable["action"] != "WAIT" || actionable["execution_ready"] != false || actionable["blocked_by_report_gate"] != true {
		t.Fatalf("actionable technical plan missing: %+v", actionable)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "rapor_veri_manifesti.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("invalid report data manifest json: %v", err)
	}
	if manifest["symbol"] != "ARTF" {
		t.Fatalf("manifest symbol = %v", manifest["symbol"])
	}
	sources := manifest["primary_data_sources"].(map[string]any)
	structured := sources["structured_financial_statements"].(map[string]any)
	if structured["row_count"].(float64) == 0 || structured["latest_period"] != "2025-Q2" {
		t.Fatalf("manifest structured financial source missing: %+v", structured)
	}
	critical := manifest["critical_metric_sources"].(map[string]any)
	if rows := critical["source_evidence_rows"].([]any); len(rows) == 0 {
		t.Fatalf("manifest evidence sample missing: %+v", critical)
	}
}

func TestWriteAnalysisLoadsKAPRawDataForArtifactsWithoutEmbeddingCanonicalJSON(t *testing.T) {
	baseDir := t.TempDir()
	root := filepath.Join(baseDir, "equities")
	processedRoot := filepath.Join(baseDir, "processed", "kapt")
	tickerDir := filepath.Join(processedRoot, "by_ticker", "ARTF")
	if err := EnsureDir(tickerDir); err != nil {
		t.Fatalf("ensure ticker dir: %v", err)
	}
	rawDocumentsPath := filepath.Join(processedRoot, kapingest.RawDocumentsFile)
	processedFilesPath := filepath.Join(processedRoot, kapingest.ProcessedFilesFile)
	financialFactsPath := filepath.Join(tickerDir, kapingest.FinancialFactsFile)
	financialTablesPath := filepath.Join(tickerDir, kapingest.FinancialTablesFile)
	documentIndexPath := filepath.Join(tickerDir, kapingest.DocumentIndexFile)

	period := "2025/12"
	sourceFile := "data/equities/ARTF/kap/attachments/ARTF_finansal.pdf"
	writeJSONLForStorageTest(t, rawDocumentsPath, []kapingest.RawDocument{{
		FilePath:          sourceFile,
		SHA256:            "sha-artf",
		Ticker:            "ARTF",
		FileName:          "ARTF_finansal.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: "financial_statement",
		Text:              "Hasılat 1.234",
		TextLength:        128,
		QualityScore:      0.91,
		AnalysisUsable:    true,
		CreatedAt:         "2026-06-19T12:00:00Z",
	}})
	writeJSONLForStorageTest(t, processedFilesPath, []kapingest.ProcessedFile{{
		FilePath:          sourceFile,
		SHA256:            "sha-artf",
		Ticker:            "ARTF",
		DocumentTypeGuess: "financial_statement",
		QualityScore:      0.91,
		TextLength:        128,
		AnalysisUsable:    true,
		CreatedAt:         "2026-06-19T12:00:00Z",
	}})
	writeJSONForStorageTest(t, documentIndexPath, kapingest.DocumentIndex{
		Ticker:      "ARTF",
		GeneratedAt: "2026-06-19T12:00:00Z",
		Counts: kapingest.DocumentIndexCounts{
			Documents:          1,
			AnalysisUsableDocs: 1,
			FinancialFacts:     1,
			FinancialTables:    1,
		},
		Documents: []kapingest.IndexedDocument{{
			DocumentID:          "doc-1",
			Ticker:              "ARTF",
			FilePath:            sourceFile,
			FileName:            "ARTF_finansal.pdf",
			SHA256:              "sha-artf",
			DocumentTypeGuess:   "financial_statement",
			ExtractionMethod:    "pdftotext",
			TextLength:          128,
			QualityScore:        0.91,
			AnalysisUsable:      true,
			FinancialFactCount:  1,
			FinancialTableCount: 1,
		}},
	})
	writeJSONLForStorageTest(t, financialFactsPath, []kapingest.ExtractedFinancialFact{{
		ID:                 "fact-1",
		Ticker:             "ARTF",
		SourceFile:         sourceFile,
		SHA256:             "sha-artf",
		Period:             &period,
		StatementType:      "income_statement",
		LineItemOriginal:   "Hasılat",
		LineItemNormalized: "revenue",
		Value:              1234,
		Currency:           "TRY",
		Unit:               "thousand_try",
		Source:             kapingest.DocumentFactSource{Page: 12, TableID: "gelir_tablosu", Snippet: "Hasılat 1.234"},
		Confidence:         0.89,
		Certification: kapingest.EvidenceCertification{
			Status:                kapingest.EvidenceStatusCertified,
			Score:                 100,
			AnalysisUsable:        true,
			EvidenceComplete:      true,
			NormalizationComplete: true,
		},
		CreatedAt: "2026-06-19T12:00:00Z",
	}})
	writeJSONLForStorageTest(t, financialTablesPath, []kapingest.ExtractedFinancialTable{{
		ID:         "table-1",
		Ticker:     "ARTF",
		SourceFile: sourceFile,
		SHA256:     "sha-artf",
		Period:     &period,
		TableType:  "income_statement",
		Currency:   "TRY",
		Unit:       "thousand_try",
		Rows:       []kapingest.FinancialTableRow{{RowIndex: 1, Cells: []string{"Hasılat", "1.234"}, Snippet: "Hasılat 1.234"}},
		Source:     kapingest.DocumentFactSource{Page: 12, TableID: "gelir_tablosu", Snippet: "Kar veya zarar tablosu"},
		Confidence: 0.91,
		Certification: kapingest.EvidenceCertification{
			Status:                kapingest.EvidenceStatusCertified,
			Score:                 100,
			AnalysisUsable:        true,
			EvidenceComplete:      true,
			NormalizationComplete: true,
		},
		CreatedAt: "2026-06-19T12:00:00Z",
	}})

	result := analysis.SymbolAnalysis{
		Symbol:       "ARTF",
		CompanyName:  "Artifact Test A.S.",
		AssetType:    ohlcv.AssetTypeEquity,
		AnalysisDate: "2026-06-19",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe:  "1D",
				LastClose:  10,
				LastVolume: 1_000_000,
				Score:      52,
				TrendBias:  "neutral",
				Indicators: ohlcv.IndicatorSnapshot{RSI14: 55, MACD: 0.4, MACDSignal: 0.2, ATR14: 0.7, ChaikinMoneyFlow20: 0.08},
				TradePlan:  ohlcv.TradePlan{Direction: "neutral", Rejected: true, RejectReason: "Nötr eğilim yeterli yön avantajı sağlamıyor"},
			},
		},
		Professional: professional.Report{
			DataQuality: 70,
			Coverage:    professional.CoverageReport{Score: 72},
			Company:     professional.CompanyProfile{Symbol: "ARTF", Name: "Artifact Test A.S.", Sector: "Sanayi", Industry: "Test"},
			Valuation:   professional.ValuationAnalysis{Ratios: map[string]float64{"PB": 1.2, "ROE": 0.08}},
			KAPPDFIngest: professional.KAPPDFIngestSummary{
				Computed:            true,
				Symbol:              "ARTF",
				OutputDir:           processedRoot,
				RawDocumentsPath:    rawDocumentsPath,
				ProcessedFilesPath:  processedFilesPath,
				TotalDocuments:      1,
				UniqueProcessed:     1,
				AnalysisUsableCount: 1,
				AverageQuality:      0.91,
			},
		},
		Disclaimer: ohlcv.Disclaimer,
	}
	if !shouldLoadKAPRawDataForReport(root, result) {
		t.Fatal("expected writer to auto-load raw KAP data for report artifacts")
	}

	writer := NewReportWriterWithOptions(ReportWriterOptions{RenderPDF: false})
	if err := writer.WriteAnalysis(context.Background(), root, result); err != nil {
		t.Fatalf("WriteAnalysis error = %v", err)
	}
	dir := AnalysisDirForAsset(root, ohlcv.AssetTypeEquity, "ARTF", "2026-06-19")
	raw, err := os.ReadFile(filepath.Join(dir, "analysis.json"))
	if err != nil {
		t.Fatal(err)
	}
	var canonical map[string]any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		t.Fatalf("invalid analysis json: %v", err)
	}
	pro := canonical["professional"].(map[string]any)
	if _, ok := pro["raw_kap_data"]; ok {
		t.Fatalf("canonical analysis.json should not embed raw_kap_data unless explicitly requested")
	}

	raw, err = os.ReadFile(filepath.Join(dir, "kap_pdf_financial_analysis.json"))
	if err != nil {
		t.Fatal(err)
	}
	var financialArtifact map[string]any
	if err := json.Unmarshal(raw, &financialArtifact); err != nil {
		t.Fatalf("invalid kap financial artifact: %v", err)
	}
	if financialArtifact["status"] != "ready" {
		t.Fatalf("financial artifact status = %v, want ready: %+v", financialArtifact["status"], financialArtifact)
	}
	if rows := financialArtifact["financial_metric_rows"].([]any); len(rows) == 0 {
		t.Fatalf("financial metric rows missing: %+v", financialArtifact)
	}
	sourceFiles := financialArtifact["source_files"].(map[string]any)
	if sourceFiles["financial_facts"] != financialFactsPath || sourceFiles["financial_tables"] != financialTablesPath {
		t.Fatalf("source files not attached: %+v", sourceFiles)
	}

	raw, err = os.ReadFile(filepath.Join(dir, "kap_pdf_reportable_data_index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reportable map[string]any
	if err := json.Unmarshal(raw, &reportable); err != nil {
		t.Fatalf("invalid reportable artifact: %v", err)
	}
	if reportable["status"] != "ready" {
		t.Fatalf("reportable artifact status = %v, want ready: %+v", reportable["status"], reportable)
	}

	raw, err = os.ReadFile(filepath.Join(dir, "rapor_veri_manifesti.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("invalid report data manifest json: %v", err)
	}
	sources := manifest["primary_data_sources"].(map[string]any)
	kapRaw := sources["kap_raw_data_files"].(map[string]any)
	files := kapRaw["files"].([]any)
	foundFinancialFactsPath := false
	for _, fileRaw := range files {
		file := fileRaw.(map[string]any)
		if file["label"] == "financial_facts" && file["path"] == financialFactsPath {
			foundFinancialFactsPath = true
		}
	}
	if !foundFinancialFactsPath {
		t.Fatalf("manifest missing financial facts path %s: %+v", financialFactsPath, files)
	}
}

func TestMarketOnlyResearchArtifactsSkipEquityFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"source_evidence_index.json",
		"kap_pdf_reportable_data_index.json",
		"kap_pdf_financial_analysis.json",
		"financial_statements_normalized.json",
		"investment_committee_memo.json",
		"quality_control_report.json",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"stale":true}`), 0o644); err != nil {
			t.Fatalf("write stale artifact %s: %v", name, err)
		}
	}
	result := analysis.SymbolAnalysis{
		Symbol:       "XAUUSD",
		AssetType:    ohlcv.AssetTypeCommodity,
		AnalysisDate: "2026-06-17",
		Currency:     "USD",
		Professional: professional.Report{
			Coverage: professional.CoverageReport{
				Score:     100,
				Available: []string{"tradingview_ohlcv_price_volume", "usd_index_dxy_real_yield_macro"},
			},
			DataQuality: 95,
			Company:     professional.CompanyProfile{Sector: "Precious Metals", Industry: "Spot Gold", SectorSource: "asset_type_commodity"},
			CommodityContext: professional.CommodityContextReport{
				Computed: true,
				Macro:    professional.CommodityContextSection{Available: true, Score: 90},
			},
			DataGovernance: professional.FinancialDataGovernance{Source: "tradingview_ohlcv+commodity_context", BacktestSafe: true, ProductionReady: true},
		},
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe: "1D",
				LastClose: 4333,
				Score:     45,
				Professional: professional.TimeframeReport{
					Technical: professional.TechnicalEvidence{Guardrails: []string{"4341 üstü kapanış pozitif teyit sağlar"}},
				},
			},
		},
	}

	if err := writeResearchArtifacts(dir, dir, result); err != nil {
		t.Fatalf("writeResearchArtifacts error = %v", err)
	}
	for _, name := range []string{
		"source_evidence_index.json",
		"kap_pdf_reportable_data_index.json",
		"kap_pdf_financial_analysis.json",
		"financial_statements_normalized.json",
		"investment_committee_memo.json",
		"quality_control_report.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("equity-only artifact %s should be removed, stat err=%v", name, err)
		}
	}
	for _, name := range []string{
		"market_research_context.json",
		"risk_matrix.json",
		"technical_trade_plan.json",
		"data_quality_report.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing market artifact %s: %v", name, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "market_research_context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "KAP") || strings.Contains(string(raw), "financial_statements_normalized") {
		t.Fatalf("market artifact contains equity-only wording: %s", raw)
	}
}

func TestKAPFinancialMetricUnitNormalization(t *testing.T) {
	fact := kapingest.ExtractedFinancialFact{
		Value:    9225378,
		Currency: "TRY",
		Unit:     "thousand_try",
	}
	if got, want := kapMetricSignedValue(fact), 9_225_378_000.0; got != want {
		t.Fatalf("kapMetricSignedValue() = %.0f, want %.0f", got, want)
	}
	text := kapMetricValueText(fact, "TRY")
	if !strings.Contains(text, "9.23 milyar TL") || !strings.Contains(text, "kaynak birim: thousand_try") {
		t.Fatalf("kapMetricValueText() did not expose normalized value and source unit: %q", text)
	}
}

func writeStructuredFinancialsForReportTest(t *testing.T, root, symbol string) {
	t.Helper()
	financialDir := filepath.Join(root, symbol, "financials")
	if err := EnsureDir(filepath.Join(financialDir, "raw")); err != nil {
		t.Fatalf("ensure financial dir: %v", err)
	}
	periodEnd := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)
	publishDate := time.Date(2025, 8, 10, 0, 0, 0, 0, time.UTC)
	fetchedAt := time.Date(2025, 8, 11, 0, 0, 0, 0, time.UTC)
	assets := 2000.0
	revenue := 1234.0
	info := domain.BilancoInfo{
		Ticker:         symbol,
		Source:         "unit_test_structured_financials",
		Currency:       "TRY",
		FinancialGroup: "12-9-6-3",
		FetchedAt:      fetchedAt,
		Periods: map[string]domain.FinancialPeriod{
			"2025-Q2": {
				Key:                "2025-Q2",
				FiscalYear:         2025,
				FiscalQuarter:      2,
				PeriodEnd:          periodEnd,
				PublishDate:        &publishDate,
				AvailableAt:        &publishDate,
				AvailabilitySource: "kap_publish_date",
				Source:             "unit_test_structured_financials",
				FinancialGroup:     "12-9-6-3",
				Currency:           "TRY",
				FetchedAt:          fetchedAt,
				BacktestSafe:       true,
			},
		},
		Lineage: []domain.DataLineageEvent{
			{Stage: "test_import", Source: "financials/raw/2025-12-9-6-3.json", Transform: "bilanco_json", CreatedAt: fetchedAt},
		},
		Data: map[string]domain.BilancoField{
			"1BL":  {DescTR: "Toplam varlıklar", DescEN: "Total assets", Years: map[string][]*float64{"2025": {nil, nil, &assets, nil}}},
			"2ODB": {DescTR: "Toplam kaynaklar", DescEN: "Total liabilities and equity", Years: map[string][]*float64{"2025": {nil, nil, &assets, nil}}},
			"REV":  {DescTR: "Hasılat", DescEN: "Revenue", Years: map[string][]*float64{"2025": {nil, nil, &revenue, nil}}},
		},
	}
	domain.NormalizeBilancoInfo(&info, symbol)
	if err := WriteJSON(filepath.Join(financialDir, "bilanco.json"), info); err != nil {
		t.Fatalf("write test bilanco json: %v", err)
	}
	if err := WriteJSON(filepath.Join(financialDir, "raw", "2025-12-9-6-3.json"), map[string]any{"ticker": symbol, "year": 2025}); err != nil {
		t.Fatalf("write test raw bilanco json: %v", err)
	}
}

func TestReportConfidenceCappedWhenInstitutionalGateFails(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:   "FAIL",
		Currency: "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				LastClose:         10,
				NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 9},
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 11},
			},
		},
		Professional: professional.Report{
			DataQuality: 100,
			DataGovernance: professional.FinancialDataGovernance{
				BacktestSafe:          true,
				ProductionReady:       true,
				FinanciallyConsistent: true,
				LatestPeriod:          "2025-Q4",
			},
			Company: professional.CompanyProfile{
				Sector:                   "Test Sektör",
				ClassificationConfidence: 0.9,
			},
			Peers: professional.PeerComparison{PeerCount: 10},
			Valuation: professional.ValuationAnalysis{
				FairValue: professional.FairValueRange{Confidence: 1, Drivers: []string{"peer_median_pb", "peer_median_ps"}},
			},
			Scenarios: []professional.Scenario{{Name: "bear"}, {Name: "base"}, {Name: "bull"}},
		},
		InstitutionalValidation: analysis.InstitutionalValidation{
			Status:  "fail",
			Score:   83,
			Summary: "Rapor güvenlik ve doğrulama kapısı başarısız.",
		},
	}

	confidence := reportConfidenceFor(result)
	if confidence.Score > 59 {
		t.Fatalf("failed institutional gate must cap report confidence, got %.0f", confidence.Score)
	}
	foundGate := false
	for _, item := range confidence.Items {
		if item.Label == "Rapor güvenlik kapısı" {
			foundGate = true
			if item.Score != 0 || item.Status != "Başarısız" {
				t.Fatalf("failed gate item should score zero and be Başarısız, got %+v", item)
			}
		}
	}
	if !foundGate {
		t.Fatalf("confidence items do not include institutional gate: %+v", confidence.Items)
	}
}

func TestProfessionalReportHTMLSeparatesDataCoverageFromDecisionConfidence(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:       "ASELS",
		Currency:     "TRY",
		AnalysisDate: "2026-06-21",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe:         "1D",
				LastClose:         402.5,
				NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 390},
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 420},
				Professional:      professional.TimeframeReport{},
			},
		},
		Professional: professional.Report{
			Coverage:    professional.CoverageReport{Score: 95, Available: []string{"financial_statements", "bist_official_unprocessed_ohlcv"}},
			DataQuality: 95,
			DataGovernance: professional.FinancialDataGovernance{
				BacktestSafe:          true,
				FinanciallyConsistent: true,
				LatestPeriod:          "2026-Q1",
			},
			Company: professional.CompanyProfile{Sector: "TEKNOLOJİ", ClassificationConfidence: 0.95},
			Peers:   professional.PeerComparison{PeerCount: 4},
			Valuation: professional.ValuationAnalysis{
				FairValue: professional.FairValueRange{Confidence: 0.8, Drivers: []string{"owner_earnings", "peer_check"}},
			},
			Scenarios: []professional.Scenario{{Name: "bear"}, {Name: "base"}, {Name: "bull"}},
		},
		InstitutionalValidation: analysis.InstitutionalValidation{Status: "fail", Score: 40},
	}

	html := professionalReportHTML(result)
	for _, want := range []string{"Veri Kapsamı", "95/100", "Karar / Model Güveni"} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing separated confidence label/value %q:\n%s", want, html)
		}
	}
	if reportDataCoverageScore(result) <= reportConfidenceFor(result).Score {
		t.Fatalf("fixture should keep data coverage separate and higher than decision/model confidence")
	}
	if strings.Contains(html, "Rapor / Veri Güveni") || strings.Contains(html, "Rapor / veri güveni") {
		t.Fatalf("combined report/data confidence label must not be shown:\n%s", html)
	}
}

func TestProfessionalReportHTMLIncludesKAPPDFIngestOnFirstPage(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:       "ALGYO",
		CompanyName:  "Alarko Gayrimenkul Yatirim Ortakligi A.S.",
		AnalysisDate: "2026-06-15",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				LastClose: 100,
				Indicators: ohlcv.IndicatorSnapshot{
					MACDHistogram: 0.2,
					RSI14:         50,
				},
				Score: 55,
			},
		},
		Professional: professional.Report{
			DataQuality: 72,
			KAPPDFIngest: professional.KAPPDFIngestSummary{
				Computed:        true,
				Symbol:          "ALGYO",
				TotalDocuments:  668,
				UniqueProcessed: 668,
				LowQualityCount: 164,
				AverageQuality:  0.74,
				TypeCounts: []professional.KAPPDFTypeCount{
					{Type: "valuation_report", Label: "Degerleme raporu", Count: 224},
					{Type: "financial_statement", Label: "Finansal tablo", Count: 59},
				},
				ImportantDocuments: []professional.KAPPDFDocumentSummary{
					{FileName: "ALGYO_degerleme.pdf", DocumentType: "valuation_report", DocumentLabel: "Degerleme raporu", QualityScore: 0.92, ContentSnippet: "Gayrimenkul degerleme raporu portfoy ve ekspertiz degeri sinyali."},
				},
				Summary: "668 benzersiz KAP PDF metni rapora dahil edildi.",
			},
			KAPAssetInventory: professional.KAPAssetInventorySummary{
				Computed:          true,
				Symbol:            "ALGYO",
				InventoryPath:     "data/processed/by_ticker/ALGYO/asset_inventory.json",
				EventCount:        10627,
				RawAssetCount:     5270,
				DisplayAssetCount: 2,
				PortfolioSummary: professional.KAPAssetPortfolioSummary{
					TotalBookValueTRY: floatPtrForReportTest(257378090),
					HistoryCount:      37,
				},
				ValueIndex: professional.KAPAssetValueIndexSummary{
					Computed:            true,
					SeriesID:            "yi_ufe",
					SeriesLabel:         "YI-UFE (2003=100)",
					Source:              "TUIK fixture",
					LatestPeriod:        "2026-05",
					IndexedAssetCount:   1,
					IndexableAssetCount: 1,
				},
				Assets: []professional.KAPAssetInventoryItem{
					{AssetName: "Bodrum Otel", AssetType: "hotel", LatestValueTRY: floatPtrForReportTest(6956049434), ValueSource: "book_value_try", IndexedValueTRY: floatPtrForReportTest(10434074151), IndexedValueAsOf: "2026-05", IndexedValueBasePeriod: "2025-03", IndexedValueFactor: 1.5, IndexedValueSource: "YI-UFE (2003=100)", HistoryCount: 6, Confidence: 0.83, SourceFile: "ALGYO faaliyet raporu.pdf"},
					{AssetName: "Etiler Alkent Sitesi Dükkanlar", AssetType: "shop", City: "Istanbul", AreaM2: floatPtrForReportTest(568), HistoryCount: 4, Confidence: 0.80, SourceFile: "ALGYO degerleme.pdf"},
				},
				Summary: "10627 asset event ve 5270 birleşik envanter satırı okundu.",
			},
			RawKAPData: &professional.KAPRawDataBundle{
				Computed: true,
				Symbol:   "ALGYO",
				Counts: professional.KAPRawDataCounts{
					RawDocuments:     668,
					DocumentFacts:    280807,
					FinancialFacts:   42364,
					FinancialTables:  29605,
					People:           3723,
					OwnershipFacts:   1339,
					CorporateEvents:  7369,
					KnowledgeGraph:   1,
					ExtractionErrors: 0,
				},
				SourceFiles: professional.KAPRawDataSourceFiles{
					FinancialTablesPath: "data/processed/by_ticker/ALGYO/financial_tables.jsonl",
					KnowledgeGraphPath:  "data/processed/by_ticker/ALGYO/company_knowledge_graph.json",
				},
				FinancialTables: []kapingest.ExtractedFinancialTable{
					{TableType: "balance_sheet", Period: stringPtrForReportTest("2025/12"), SourceFile: "ALGYO finansal rapor.pdf", Source: kapingest.DocumentFactSource{Snippet: "Finansal durum tablosu varlıklar özkaynaklar"}, Confidence: 0.91},
				},
				FinancialFacts: []kapingest.ExtractedFinancialFact{
					{StatementType: "balance_sheet", LineItemNormalized: "toplam varlik", Value: 123456789, Currency: "TRY", Period: stringPtrForReportTest("2025/12"), SourceFile: "ALGYO finansal rapor.pdf", Source: kapingest.DocumentFactSource{Snippet: "Toplam varlıklar 123.456.789"}, Confidence: 0.88},
				},
				People: []kapingest.ExtractedPerson{
					{FullName: "Ayşe Yılmaz", NormalizedName: "ayse yilmaz", Role: "board_of_directors", Title: "Üye", Period: stringPtrForReportTest("2025/12"), Confidence: 0.86},
				},
				OwnershipFacts: []kapingest.OwnershipFact{
					{HolderName: "Alarko Holding A.Ş.", ShareRatio: floatPtrForReportTest(49.0), Period: stringPtrForReportTest("2025/12"), SourceFile: "ALGYO faaliyet raporu.pdf", Source: kapingest.DocumentFactSource{Snippet: "Alarko Holding A.Ş. %49"}},
				},
				CorporateEvents: []kapingest.ExtractedCorporateEvent{
					{EventType: "dividend", Title: "Kar payı dağıtımı", Period: stringPtrForReportTest("2025/12"), SourceFile: "ALGYO genel kurul.pdf", Source: kapingest.DocumentFactSource{Snippet: "Kar payı dağıtımı görüşüldü"}, Confidence: 0.90},
				},
				KnowledgeGraph: &kapingest.CompanyKnowledgeGraph{
					Ticker: "ALGYO",
					Nodes:  []kapingest.KnowledgeGraphNode{{ID: "company:algyo", Type: "company", Label: "ALGYO"}},
					Edges:  []kapingest.KnowledgeGraphEdge{{ID: "edge:1", From: "company:algyo", To: "document:1", Type: "has_document"}},
					Contradictions: []kapingest.KnowledgeContradiction{
						{Type: "financial_value_conflict", Key: "2025/12|balance_sheet|toplam varlik", Severity: "medium"},
					},
				},
			},
			Company: professional.CompanyProfile{Sector: "Gayrimenkul Yatirim Ortakligi"},
			Peers:   professional.PeerComparison{ValuationSignal: "neutral"},
			Valuation: professional.ValuationAnalysis{
				Ratios: map[string]float64{},
			},
		},
		Disclaimer: ohlcv.Disclaimer,
	}

	html := professionalReportHTML(result)
	for _, want := range []string{
		"KAP PDF Raporları",
		"668 belge",
		"Degerleme raporu: 224",
		"Öne Çıkan PDF Kanıtları",
		"KAP PDF rapor kanıtı",
		"KAP Varlık Envanteri",
		"10627 varlık olayı",
		"Endeksli değerleme",
		"YI-UFE (2003=100) | son endeks 2026-05 | 1/1 varlık endekslendi",
		"Yİ-ÜFE güncel değer",
		"10434074151.00 TL | 2026-05 | 2025-03=&gt;x1.5000 | YI-UFE (2003=100)",
		"Bodrum Otel",
		"Tam envanter JSON",
		"İç veri kaynağı raporda gizlendi.",
		"KAP Ham Veri İndeksi",
		"Finansal Tablo Blokları",
		"Ham Finansal Satır Adayları (Denetim)",
		"Yönetim / Kişi Adayları",
		"Ortaklık / Sermaye Adayları",
		"Kurumsal Olay Adayları",
		"Belge İlişki Ağı / Veri Mutabakatı",
		"Ayşe Yılmaz",
		"Alarko Holding A.Ş.",
		"Kar payı dağıtımı",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report HTML does not contain %q\n%s", want, html)
		}
	}
	assetIdx := strings.Index(html, "<h2>KAP Varlık Envanteri</h2>")
	sourceIdx := strings.Index(html, "<h2>Kaynaklar ve PDF Ekleri</h2>")
	if assetIdx < 0 || sourceIdx < 0 || assetIdx > sourceIdx {
		t.Fatalf("asset inventory should render before source appendix: asset=%d source=%d", assetIdx, sourceIdx)
	}
	if strings.Count(html, "<h2>KAP Varlık Envanteri</h2>") != 1 {
		t.Fatalf("asset inventory should render once\n%s", html)
	}
}

func TestKAPRawPeopleRowsRejectsCompanyNamesAndTruncatedPrefixes(t *testing.T) {
	rows := kapRawPeopleRows(professional.KAPRawDataBundle{
		People: []kapingest.ExtractedPerson{
			{FullName: "Abdi İbrahim İlaç", NormalizedName: "ABDIIBRAHIMILAC", Role: "board_of_directors", Title: "Başkan", Confidence: 0.95},
			{FullName: "Acar Sertaç", NormalizedName: "ACARSERTAC", Role: "board_of_directors", Title: "Üye", Confidence: 0.95},
			{FullName: "Acar Sertaç Komsuoğlu", NormalizedName: "ACARSERTACKOMSUOGLU", Role: "board_of_directors", Title: "Üye", Confidence: 0.87},
			{FullName: "Ali Koç", NormalizedName: "ALIKOC", Role: "board_of_directors", Title: "Başkan", Confidence: 0.92},
			{FullName: "Sabahattin Zaim Üniversitesi İşletme", NormalizedName: "SABAHATTINZAIMUNIVERSITESIISLETME", Role: "board_of_directors", Title: "Üye", Confidence: 0.95},
			{FullName: "Aselsan Katar Şubesi", NormalizedName: "ASELSANKATARSUBESI", Role: "board_of_directors", Title: "Başkan", Confidence: 0.95},
			{FullName: "MGEO Sektör", NormalizedName: "MGEOSEKTOR", Role: "executive_management", Title: "Başkan", Confidence: 0.95},
			{FullName: "Silahlı Kuvvetlerini Güçlendirme Vakfı", NormalizedName: "TSKGV", Role: "board_of_directors", Title: "Üye", Confidence: 0.95},
			{FullName: "A.Ş BİTES", NormalizedName: "ASBITES", Role: "executive_management", Title: "Başkan", Confidence: 0.95},
			{FullName: "Murat ASLAN Aselsan Konya", NormalizedName: "MURATASLANASELSANKONYA", Role: "board_of_directors", Title: "Başkan", Confidence: 0.95},
			{FullName: "Limited Şirketi Mikro AR-GE", NormalizedName: "LIMITEDSIRKETIMIKROARGE", Role: "executive_management", Title: "Üye", Confidence: 0.95},
			{FullName: "Tedarik Zinciri Yönetimi Yardımcılığı", NormalizedName: "TEDARIKZINCIRIYONETIMIYARDIMCILIGI", Role: "executive_management", Title: "Üye", Confidence: 0.95},
			{FullName: "Hukuk Müşavirliği", NormalizedName: "HUKUKMUSAVIRLIGI", Role: "executive_management", Title: "Üye", Confidence: 0.95},
			{FullName: "Saudi Arabian Defense", NormalizedName: "SAUDIARABIANDEFENSE", Role: "executive_management", Title: "Üye", Confidence: 0.95},
			{FullName: "And Electronics Engineers IEEE", NormalizedName: "ANDELECTRONICSENGINEERSIEEE", Role: "executive_management", Title: "Üye", Confidence: 0.95},
			{FullName: "Faik EKEN Mayıs", NormalizedName: "FAIKEKENMAYIS", Role: "executive_management", Title: "Üye", Confidence: 0.95},
		},
	}, 20)
	joined := ""
	for _, row := range rows {
		joined += strings.Join(row, " ") + "\n"
	}
	for _, banned := range []string{
		"Abdi İbrahim İlaç",
		"Sabahattin Zaim Üniversitesi İşletme",
		"Aselsan Katar Şubesi",
		"MGEO Sektör",
		"Silahlı Kuvvetlerini Güçlendirme Vakfı",
		"A.Ş BİTES",
		"Murat ASLAN Aselsan Konya",
		"Limited Şirketi Mikro AR-GE",
		"Tedarik Zinciri Yönetimi Yardımcılığı",
		"Hukuk Müşavirliği",
		"Saudi Arabian Defense",
		"And Electronics Engineers IEEE",
		"Faik EKEN Mayıs",
	} {
		if strings.Contains(joined, banned) {
			t.Fatalf("non-person candidate %q must not be rendered as a person:\n%s", banned, joined)
		}
	}
	for _, row := range rows {
		if len(row) > 0 && row[0] == "Acar Sertaç" {
			t.Fatalf("truncated person prefix must not be preferred:\n%s", joined)
		}
	}
	if !strings.Contains(joined, "Acar Sertaç Komsuoğlu") || !strings.Contains(joined, "Ali Koç") {
		t.Fatalf("valid person names missing:\n%s", joined)
	}
}

func TestInvestorHTMLRedactsInternalCommandsAndDataPaths(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:       "FENER",
		AssetType:    ohlcv.AssetTypeEquity,
		CompanyName:  "Fenerbahçe Futbol A.Ş.",
		AnalysisDate: "2026-06-19_14-32-05",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {Timeframe: "1D", LastClose: 10, Score: 42},
		},
		DecisionSupport: &analysis.DecisionSupportReport{
			Summary: "go run ./cmd/hissebot analyze --all --provider tradingview",
			CompletionActions: []analysis.DecisionCompletionStep{
				{
					Priority: 1,
					Area:     "kap_evidence",
					Action:   "İç veri tamamlanmalı",
					Command:  "go run ./cmd/hissebot sync kap-attachments -ticker FENER",
					AcceptanceCriteria: []string{
						"analysis.json ve data/processed/fener/raw_documents.jsonl hazır olmalı",
						"Kaynak kanıtı yatırımcıya açık şekilde özetlenmeli",
					},
				},
			},
			BatchRefresh: analysis.DecisionCompletionStep{
				Priority: 90,
				Area:     "all",
				Action:   "Yenile",
				Command:  "go run ./cmd/hissebot analyze --all --provider tradingview",
			},
		},
		Professional: professional.Report{
			RawKAPData: &professional.KAPRawDataBundle{
				Computed: true,
				Counts:   professional.KAPRawDataCounts{RawDocuments: 1, People: 1},
				SourceFiles: professional.KAPRawDataSourceFiles{
					FinancialTablesPath: "data/processed/fener/financial_tables.jsonl",
					KnowledgeGraphPath:  "data/processed/fener/company_knowledge_graph.json",
				},
				People: []kapingest.ExtractedPerson{
					{FullName: "Ali Koç", NormalizedName: "ALIKOC", Role: "board_of_directors", Confidence: 0.92},
				},
			},
		},
	}
	html := professionalReportHTML(result)
	for _, banned := range []string{"go run", "cmd/hissebot", "analysis.json", ".jsonl", "data/processed", "raw_documents"} {
		if strings.Contains(html, banned) {
			t.Fatalf("investor HTML leaked internal token %q\n%s", banned, html)
		}
	}
	if !strings.Contains(html, "İç veri kaynağı raporda gizlendi") && !strings.Contains(html, "yatırımcı raporunda gösterilmez") {
		t.Fatalf("expected redacted investor wording\n%s", html)
	}
}

func TestProfessionalReportHTMLRendersBankOnlyMetricsAsNotApplicable(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:       "ISCTR",
		CompanyName:  "Türkiye İş Bankası A.Ş.",
		AnalysisDate: "2026-06-19",
		Currency:     "TRY",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {LastClose: 15.47, Score: 50},
		},
		Professional: professional.Report{
			Company: professional.CompanyProfile{Sector: "Banka", Industry: "BANKALAR"},
			Valuation: professional.ValuationAnalysis{
				LatestYear:       2026,
				LatestQuarter:    "Q1",
				SectorModel:      "bank_equity_model",
				MarketCap:        386_750_000_000,
				NetDebt:          -902_210_000_000,
				Equity:           498_064_000_000,
				SuppressedRatios: []string{"PS", "EV_Sales", "EV_EBIT", "EV_EBITDA", "FCF_Yield", "NetDebt_Eq"},
				Ratios: map[string]float64{
					"PE":  3.93,
					"PB":  0.78,
					"ROE": 0.197,
					"ROA": 0.017,
				},
			},
			SectorFinancials: professional.SectorFinancialAnalysis{Profile: "bank", ProfileLabel: "Banka", Summary: "Banka bilanço profili."},
			Peers:            professional.PeerComparison{ValuationSignal: "neutral"},
			KAPAssetInventory: professional.KAPAssetInventorySummary{
				Computed:          true,
				EventCount:        1,
				RawAssetCount:     1,
				DisplayAssetCount: 1,
				ValueIndex: professional.KAPAssetValueIndexSummary{
					Computed:            true,
					SeriesLabel:         "Yİ-ÜFE (2003=100)",
					LatestPeriod:        "2026-05",
					IndexedAssetCount:   1,
					IndexableAssetCount: 1,
				},
				Assets: []professional.KAPAssetInventoryItem{
					{AssetName: "İzmir Konak Hizmet Binası", AssetType: "office", LatestValueTRY: floatPtrForReportTest(1_000_000), IndexedValueTRY: floatPtrForReportTest(1_500_000), IndexedValueAsOf: "2026-05", IndexedValueBasePeriod: "2025-03", IndexedValueFactor: 1.5, IndexedValueSource: "Yİ-ÜFE (2003=100)", Confidence: 1},
				},
			},
		},
		Disclaimer: ohlcv.Disclaimer,
	}

	html := professionalReportHTML(result)
	for _, want := range []string{
		"Net borç</th><td>Uygulanmaz",
		"Banka çarpanları</th><td>F/K 3.93 | PD/DD 0.78 | FD/Satış A.D.",
		"ROE | ROA",
		"19.7% | 1.7%",
		"Kapsam dışı değerleme metrikleri",
		"KAP Varlık Envanteri (Referans)",
		"Yİ-ÜFE güncel değer",
		"1500000.00 TL | 2026-05 | 2025-03=&gt;x1.5000 | Yİ-ÜFE (2003=100)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("bank report HTML does not contain %q\n%s", want, html)
		}
	}
	for _, banned := range []string{
		"-902.21 milyar TL",
		"3.93 | 0.78 | 0.00",
		"ROE | Net borç/özsermaye",
	} {
		if strings.Contains(html, banned) {
			t.Fatalf("bank report HTML contains banned text %q\n%s", banned, html)
		}
	}
}

func TestKAPPDFFinancialMetricSelectionPrefersPrimaryStatementRows(t *testing.T) {
	period := stringPtrForReportTest("2026-02-28")
	raw := &professional.KAPRawDataBundle{
		Computed: true,
		FinancialFacts: []kapingest.ExtractedFinancialFact{
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "TOPLAM VARLIKLAR",
				LineItemNormalized: "total_assets",
				Value:              33_061_674_991,
				Currency:           "TRY",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "TOPLAM VARLIKLAR 33.061.674.991 15.420.161.527"},
				Confidence:         0.8,
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "9. TOPLAM VARLIKLAR (4+8)",
				LineItemNormalized: "total_assets",
				Value:              1_614_271_116,
				Currency:           "TRY",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "9. TOPLAM VARLIKLAR (4+8) 1.614.271.116 1.126.528 30.268.955 -"},
				Confidence:         0.95,
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Nakit ve Nakit Benzerleri",
				LineItemNormalized: "cash_and_cash_equivalents",
				Value:              372_393_660,
				Currency:           "TRY",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Nakit ve Nakit Benzerleri 5 372.393.660 542.251.478"},
				Confidence:         0.8,
			},
			{
				StatementType:      "cash_flow_statement",
				LineItemOriginal:   "C. FİNANSMAN FAALİYETLERİNDEN NAKİT AKIŞLARI",
				LineItemNormalized: "cash_flow",
				Value:              6_542_361_161,
				Currency:           "TRY",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "C. FİNANSMAN FAALİYETLERİNDEN NAKİT AKIŞLARI 6.542.361.161 (1.107.928.861)"},
				Confidence:         0.95,
			},
			{
				StatementType:      "cash_flow_statement",
				LineItemOriginal:   "A. İŞLETME FAALİYETLERİNDEN NAKİT AKIŞLARI",
				LineItemNormalized: "cash_flow",
				Value:              3_027_757_255,
				Currency:           "TRY",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "A. İŞLETME FAALİYETLERİNDEN NAKİT AKIŞLARI (3.027.757.255) 3.205.603.555"},
				Confidence:         0.8,
			},
			{
				StatementType:      "income_statement",
				LineItemOriginal:   "Faaliyet karı (zararı)",
				LineItemNormalized: "operating_profit",
				Value:              4_655_789_174,
				Currency:           "TRY",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Faaliyet karı (zararı) (4.655.789.174) 456.904.200 - (4.198.884.974)"},
				Confidence:         0.8,
			},
		},
	}

	points := kapFinancialComparisonMap(raw)
	totalAssets := latestMetric(points, "total_assets")
	if totalAssets == nil || totalAssets.Fact.Value != 33_061_674_991 {
		t.Fatalf("primary total assets row not selected: %+v", totalAssets)
	}
	cash := latestMetric(points, "cash")
	if cash == nil || cash.Fact.Value != 372_393_660 {
		t.Fatalf("balance-sheet cash row not selected: %+v", cash)
	}
	cfo := latestMetric(points, "operating_cash_flow")
	if cfo == nil || cfo.Fact.Value != 3_027_757_255 || kapMetricSignedValue(cfo.Fact) >= 0 {
		t.Fatalf("operating cash flow row/sign not selected: %+v signed=%v", cfo, kapMetricSignedValue(cfo.Fact))
	}
	operatingProfit := latestMetric(points, "operating_profit")
	if operatingProfit == nil || kapMetricSignedValue(operatingProfit.Fact) >= 0 {
		t.Fatalf("operating loss sign not detected: %+v signed=%v", operatingProfit, kapMetricSignedValue(operatingProfit.Fact))
	}
}

func TestKAPPDFFinancialMetricSelectionUsesResolvedReconciliation(t *testing.T) {
	period := stringPtrForReportTest("2026-02-28")
	raw := &professional.KAPRawDataBundle{
		Computed: true,
		FinancialFacts: []kapingest.ExtractedFinancialFact{
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "TOPLAM VARLIKLAR",
				LineItemNormalized: "total_assets",
				Value:              33_061_674_991,
				Currency:           "TRY",
				Period:             period,
				SourceFile:         "ALGYO finansal rapor.pdf",
				Source:             kapingest.DocumentFactSource{Snippet: "TOPLAM VARLIKLAR 33.061.674.991 15.420.161.527"},
				Confidence:         0.80,
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "TOPLAM VARLIKLAR",
				LineItemNormalized: "total_assets",
				Value:              1_614_271_116,
				Currency:           "TRY",
				Period:             period,
				SourceFile:         "ALGYO faaliyet raporu.pdf",
				Source:             kapingest.DocumentFactSource{Snippet: "TOPLAM VARLIKLAR 1.614.271.116"},
				Confidence:         0.99,
			},
		},
		KnowledgeGraph: &kapingest.CompanyKnowledgeGraph{
			ResolvedContradictions: []kapingest.KnowledgeContradictionResolution{
				{
					Type:               "financial_value_conflict",
					Key:                "2026-02-28|balance_sheet|total_assets|TRY|",
					Status:             "resolved_by_certification",
					SelectedValue:      33_061_674_991,
					Currency:           "TRY",
					SelectedSourceFile: "ALGYO finansal rapor.pdf",
					Period:             period,
					Reason:             "Sertifikalı ve analize uygun kaynak olduğu için seçildi.",
					Confidence:         0.96,
				},
			},
		},
	}

	points := kapFinancialComparisonMap(raw)
	totalAssets := latestMetric(points, "total_assets")
	if totalAssets == nil || totalAssets.Fact.Value != 33_061_674_991 {
		t.Fatalf("resolved total assets value not selected: %+v", totalAssets)
	}
}

func TestOneLookUpsideLinesDedupesSameResistanceLevel(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Currency: "TL",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 411.25},
			},
		},
		InvestorQA: investorqa.Report{
			BuyConditions: []string{
				"411.25 üstü kapanış; alıcı ilgisinin pozitife dönmesi",
				"para akışının pozitife dönmesi",
				"420.00 üstü kapanış ikinci teyit olur",
				"içsel değere göre yeterli güvenlik marjı",
			},
		},
		Professional: professional.Report{
			ValueInvesting: value.Report{
				Computed: false,
				IntrinsicValue: value.IntrinsicValueReport{
					Computed: false,
					Base:     0,
				},
				MarginOfSafety: value.MarginOfSafetyReport{
					Computed:    false,
					RequiredPct: 25,
				},
			},
		},
	}

	lines := oneLookUpsideLines(result)
	joined := strings.Join(lines, "\n")
	if count := strings.Count(joined, "411.25"); count != 1 {
		t.Fatalf("same resistance level must be shown once, count=%d lines=%v", count, lines)
	}
	if !strings.Contains(joined, "420.00") {
		t.Fatalf("distinct upside level should remain: %v", lines)
	}
	if count := strings.Count(joined, "alıcı ilgisinin pozitife dönmesi"); count != 1 {
		t.Fatalf("standalone buyer-interest condition should be deduped, count=%d lines=%v", count, lines)
	}
	if strings.Contains(joined, "ilgisinın") {
		t.Fatalf("upside lines contain malformed Turkish spelling: %v", lines)
	}
	if strings.Contains(joined, "güvenlik marjı") {
		t.Fatalf("upside lines must not claim safety margin when valuation is suppressed: %v", lines)
	}
	dataLines := strings.Join(oneLookDataLines(result), "\n")
	if !strings.Contains(dataLines, "içsel değer") || !strings.Contains(dataLines, "güvenlik marjı") {
		t.Fatalf("suppressed valuation should be shown as missing data, got: %s", dataLines)
	}
}

func TestKAPPDFFinancialReadingRowsRequireSamePeriodForDerivedRatios(t *testing.T) {
	raw := &professional.KAPRawDataBundle{
		Computed: true,
		FinancialFacts: []kapingest.ExtractedFinancialFact{
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Toplam Varlıklar",
				LineItemNormalized: "total_assets",
				Value:              22_390_000_000,
				Currency:           "TRY",
				Period:             stringPtrForReportTest("2019-09-30"),
				Source:             kapingest.DocumentFactSource{Snippet: "Toplam Varlıklar 22.390.000.000"},
				Confidence:         0.90,
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Özkaynaklar",
				LineItemNormalized: "equity",
				Value:              282_740_000_000,
				Currency:           "TRY",
				Period:             stringPtrForReportTest("2026-03-31"),
				Source:             kapingest.DocumentFactSource{Snippet: "Özkaynaklar 282.740.000.000"},
				Confidence:         0.90,
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Dönen Varlıklar",
				LineItemNormalized: "current_assets",
				Value:              10_000_000_000,
				Currency:           "TRY",
				Period:             stringPtrForReportTest("2019-09-30"),
				Source:             kapingest.DocumentFactSource{Snippet: "Dönen Varlıklar 10.000.000.000"},
				Confidence:         0.90,
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Kısa Vadeli Yükümlülükler",
				LineItemNormalized: "current_liabilities",
				Value:              110_000_000_000,
				Currency:           "TRY",
				Period:             stringPtrForReportTest("2026-03-31"),
				Source:             kapingest.DocumentFactSource{Snippet: "Kısa Vadeli Yükümlülükler 110.000.000.000"},
				Confidence:         0.90,
			},
		},
	}

	rows := kapPDFFinancialReadingRows(raw, "TRY", false)
	joined := strings.Join(flattenReportTestRows(rows), "\n")
	for _, banned := range []string{"1262", "Cari oran yaklaşık 0.09"} {
		if strings.Contains(joined, banned) {
			t.Fatalf("period-mixed derived ratio leaked %q in rows: %v", banned, rows)
		}
	}
	for _, want := range []string{"Hesaplanmadı", "Toplam varlıklar=2019-09-30", "Özkaynaklar=2026-03-31", "Dönen varlıklar=2019-09-30", "Kısa vadeli yükümlülükler=2026-03-31"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in period mismatch rows, got %v", want, rows)
		}
	}
}

func TestKAPPDFFinancialReadingRowsCompareIncomeStatementSameSeason(t *testing.T) {
	raw := &professional.KAPRawDataBundle{
		Computed: true,
		FinancialFacts: []kapingest.ExtractedFinancialFact{
			{
				StatementType:      "income_statement",
				LineItemOriginal:   "Hasılat",
				LineItemNormalized: "revenue",
				Value:              34_305_800,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             stringPtrForReportTest("2026-03-31"),
				Source:             kapingest.DocumentFactSource{Snippet: "Hasılat 34.305.800"},
				Confidence:         0.95,
			},
			{
				StatementType:      "income_statement",
				LineItemOriginal:   "Hasılat",
				LineItemNormalized: "revenue",
				Value:              198_000_000,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             stringPtrForReportTest("2025-12-31"),
				Source:             kapingest.DocumentFactSource{Snippet: "Hasılat 198.000.000"},
				Confidence:         0.95,
			},
			{
				StatementType:      "income_statement",
				LineItemOriginal:   "Hasılat",
				LineItemNormalized: "revenue",
				Value:              29_825_146,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             stringPtrForReportTest("2025-03-31"),
				Source:             kapingest.DocumentFactSource{Snippet: "Hasılat 29.825.146"},
				Confidence:         0.95,
			},
		},
	}

	readings := strings.Join(flattenReportTestRows(kapPDFFinancialReadingRows(raw, "TRY", false)), "\n")
	if !strings.Contains(readings, "Hasılat 2025-03-31 döneminden 2026-03-31 dönemine") {
		t.Fatalf("income statement trend should compare same-season periods, got:\n%s", readings)
	}
	if strings.Contains(readings, "Hasılat 2025-12-31 döneminden 2026-03-31 dönemine") {
		t.Fatalf("income statement trend compared cumulative Q1 to annual Q4:\n%s", readings)
	}
}

func TestKAPBusinessModelTextFiltersDefenseSectorNoise(t *testing.T) {
	raw := &professional.KAPRawDataBundle{
		DocumentIndex: &kapingest.DocumentIndex{
			Sector: kapingest.CompanySectorContext{Sector: "SAVUNMA"},
			Documents: []kapingest.IndexedDocument{{
				BusinessModels: []kapingest.BusinessModelTag{
					{Tag: "food_beverage", Confidence: 0.90},
					{Tag: "pharmaceuticals", Confidence: 0.90},
					{Tag: "defense", Confidence: 0.88},
					{Tag: "r_and_d", Confidence: 0.76},
				},
			}},
		},
	}

	text := kapBusinessModelText(raw, 8)
	for _, want := range []string{"defense", "r_and_d"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected defense-compatible business model %q in %q", want, text)
		}
	}
	for _, banned := range []string{"food_beverage", "pharmaceuticals"} {
		if strings.Contains(text, banned) {
			t.Fatalf("incompatible defense business model %q leaked in %q", banned, text)
		}
	}
}

func TestKAPPDFFinancialRowsPreferPrimaryStatementRows(t *testing.T) {
	period := stringPtrForReportTest("2026-03-31")
	raw := &professional.KAPRawDataBundle{
		Computed: true,
		FinancialFacts: []kapingest.ExtractedFinancialFact{
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "9. Toplam varlıklar (4+8)",
				LineItemNormalized: "total_assets",
				Value:              139_446_730,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "9. Toplam varlıklar (4+8) 139.446.730 1.598.912 75.384.563 1.108.956 61.363.606 317.422"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "TOPLAM VARLIKLAR",
				LineItemNormalized: "total_assets",
				Value:              485_018_937,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "TOPLAM VARLIKLAR 485.018.937 474.918.696"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "4. Dönen varlıklar (1+2+3)",
				LineItemNormalized: "current_assets",
				Value:              67_077_437,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "4. Dönen varlıklar (1+2+3) 67.077.437 897.294 39.836.394 433.844 22.095.457 331.012"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Dönen Varlıklar",
				LineItemNormalized: "current_assets",
				Value:              207_675_087,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Dönen Varlıklar 207.675.087 192.552.460"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "ÖZKAYNAKLAR",
				LineItemNormalized: "equity",
				Value:              282_737_905,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "ÖZKAYNAKLAR 282.737.905 277.065.638"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Kısa Vadeli Yükümlülükler",
				LineItemNormalized: "current_liabilities",
				Value:              134_881_888,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Kısa Vadeli Yükümlülükler 134.881.888 138.902.755"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Uzun Vadeli Yükümlülükler",
				LineItemNormalized: "non_current_liabilities",
				Value:              67_399_144,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Uzun Vadeli Yükümlülükler 67.399.144 58.950.303"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Nakit ve Nakit Benzerleri",
				LineItemNormalized: "cash_and_cash_equivalents",
				Value:              31_005_730,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Nakit ve Nakit Benzerleri 3 31.005.730 32.006.758"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Toplam uzun vadeli finansal borçlar",
				LineItemNormalized: "financial_debt",
				Value:              4_319_259,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Toplam uzun vadeli finansal borçlar 4.319.259 5.533.211"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Toplam finansal borçlar",
				LineItemNormalized: "financial_debt",
				Value:              53_089_817,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Toplam finansal borçlar 53.089.817 47.379.427"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "income_statement",
				LineItemOriginal:   "Hasılat",
				LineItemNormalized: "revenue",
				Value:              34_305_800,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "Hasılat 14 34.305.800 29.825.146"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "income_statement",
				LineItemOriginal:   "ESAS FAALİYET KÂRI",
				LineItemNormalized: "operating_profit",
				Value:              9_692_489,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "ESAS FAALİYET KÂRI 8.549.768 9.692.489"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "income_statement",
				LineItemOriginal:   "DÖNEM KÂRI",
				LineItemNormalized: "net_income",
				Value:              5_549_453,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             period,
				Source:             kapingest.DocumentFactSource{Snippet: "DÖNEM KÂRI 5.549.453 2.791.993"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
		},
	}

	metricRows := strings.Join(flattenReportTestRows(kapPDFFinancialMetricRows(raw, "TRY")), "\n")
	for _, want := range []string{"485.02 milyar TL", "207.68 milyar TL", "282.74 milyar TL"} {
		if !strings.Contains(metricRows, want) {
			t.Fatalf("expected primary statement value %q in metric rows:\n%s", want, metricRows)
		}
	}
	for _, banned := range []string{"139.45 milyar TL", "67.08 milyar TL"} {
		if strings.Contains(metricRows, banned) {
			t.Fatalf("formula/breakdown row leaked as primary value %q in metric rows:\n%s", banned, metricRows)
		}
	}

	readings := strings.Join(flattenReportTestRows(kapPDFFinancialReadingRows(raw, "TRY", false)), "\n")
	for _, want := range []string{"Özkaynak / aktif oranı 58.3%", "Cari oran yaklaşık 1.54x", "yaklaşık 22.08 milyar TL"} {
		if !strings.Contains(readings, want) {
			t.Fatalf("expected reconciled financial reading %q in rows:\n%s", want, readings)
		}
	}
	if strings.Contains(readings, "Hesaplanmadı") {
		t.Fatalf("primary statement rows should reconcile without manual-review warning:\n%s", readings)
	}
}

func TestHydrateStructuredFinancialFactsSuppliesPrimaryKAPRows(t *testing.T) {
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	symbol := "ASELS"
	financialDir := filepath.Join(equitiesDir, symbol, "financials")
	if err := EnsureDir(financialDir); err != nil {
		t.Fatalf("ensure financial dir: %v", err)
	}

	periodEnd := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	publishDate := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	fetchedAt := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	period := domain.FinancialPeriod{
		Key:                "2026-Q1",
		FiscalYear:         2026,
		FiscalQuarter:      1,
		PeriodEnd:          periodEnd,
		PublishDate:        &publishDate,
		AvailableAt:        &publishDate,
		AvailabilitySource: "kap_publish_date",
		Source:             "unit_test_structured_financials",
		FinancialGroup:     "12-9-6-3",
		Currency:           "TRY",
		FetchedAt:          fetchedAt,
		BacktestSafe:       true,
	}
	info := domain.BilancoInfo{
		Ticker:         symbol,
		Source:         "unit_test_structured_financials",
		Currency:       "TRY",
		FinancialGroup: "12-9-6-3",
		FetchedAt:      fetchedAt,
		Periods:        map[string]domain.FinancialPeriod{"2026-Q1": period},
		Data: map[string]domain.BilancoField{
			"1A":  testBilancoQ1Field("Dönen Varlıklar", 207_675_087_000),
			"1AA": testBilancoQ1Field("Nakit ve Nakit Benzerleri", 31_005_730_000),
			"1BL": testBilancoQ1Field("TOPLAM VARLIKLAR", 485_018_937_000),
			"2A":  testBilancoQ1Field("Kısa Vadeli Yükümlülükler", 134_881_888_000),
			"2AA": testBilancoQ1Field("Kısa Vadeli Finansal Borçlar", 48_770_558_000),
			"2B":  testBilancoQ1Field("Uzun Vadeli Yükümlülükler", 67_399_144_000),
			"2BA": testBilancoQ1Field("Uzun Vadeli Finansal Borçlar", 4_319_259_000),
			"2N":  testBilancoQ1Field("Özkaynaklar", 282_737_905_000),
			"3C":  testBilancoQ1Field("Satış Gelirleri", 34_305_800_000),
			"3DF": testBilancoQ1Field("FAALİYET KARI (ZARARI)", 8_549_768_000),
			"3L":  testBilancoQ1Field("DÖNEM KARI (ZARARI)", 5_549_453_000),
			"4C":  testBilancoQ1Field("İşletme Faaliyetlerinden Kaynaklanan Net Nakit", 5_863_229_000),
			"4CB": testBilancoQ1Field("Serbest Nakit Akım", -7_716_663_000),
		},
	}
	domain.NormalizeBilancoInfo(&info, symbol)
	if err := WriteJSON(filepath.Join(financialDir, "bilanco.json"), info); err != nil {
		t.Fatalf("write test bilanco: %v", err)
	}

	rawPeriod := stringPtrForReportTest("2026-03-31")
	bundle := professional.KAPRawDataBundle{
		Computed: true,
		KnowledgeGraph: &kapingest.CompanyKnowledgeGraph{ResolvedContradictions: []kapingest.KnowledgeContradictionResolution{
			{
				Type:          "financial_value_conflict",
				Key:           "2026-03-31|balance_sheet|financial_debt|TRY|thousand_try",
				Status:        "resolved",
				SelectedValue: 4_319_259,
			},
		}},
		FinancialFacts: []kapingest.ExtractedFinancialFact{
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "9. Toplam varlıklar (4+8)",
				LineItemNormalized: "total_assets",
				Value:              139_446_730,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             rawPeriod,
				Source:             kapingest.DocumentFactSource{Snippet: "9. Toplam varlıklar (4+8) 139.446.730 1.598.912 75.384.563"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "4. Dönen varlıklar (1+2+3)",
				LineItemNormalized: "current_assets",
				Value:              67_077_437,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             rawPeriod,
				Source:             kapingest.DocumentFactSource{Snippet: "4. Dönen varlıklar (1+2+3) 67.077.437 897.294 39.836.394"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
			{
				StatementType:      "balance_sheet",
				LineItemOriginal:   "Toplam uzun vadeli finansal borçlar",
				LineItemNormalized: "financial_debt",
				Value:              4_319_259,
				Currency:           "TRY",
				Unit:               "thousand_try",
				Period:             rawPeriod,
				Source:             kapingest.DocumentFactSource{Snippet: "Toplam uzun vadeli finansal borçlar 4.319.259 5.533.211"},
				Confidence:         0.95,
				Certification:      kapingest.EvidenceCertification{Status: "certified"},
			},
		},
	}

	hydrated := hydrateStructuredFinancialFacts(equitiesDir, analysis.SymbolAnalysis{Symbol: symbol}, bundle)
	metricRows := strings.Join(flattenReportTestRows(kapPDFFinancialMetricRows(&hydrated, "TRY")), "\n")
	for _, want := range []string{"485.02 milyar TL", "207.68 milyar TL", "282.74 milyar TL"} {
		if !strings.Contains(metricRows, want) {
			t.Fatalf("structured bilanco fact %q missing from metric rows:\n%s", want, metricRows)
		}
	}
	for _, banned := range []string{"139.45 milyar TL", "67.08 milyar TL"} {
		if strings.Contains(metricRows, banned) {
			t.Fatalf("formula/breakdown PDF row leaked after structured fallback %q:\n%s", banned, metricRows)
		}
	}
	readings := strings.Join(flattenReportTestRows(kapPDFFinancialReadingRows(&hydrated, "TRY", false)), "\n")
	for _, want := range []string{"Özkaynak / aktif oranı 58.3%", "Cari oran yaklaşık 1.54x", "yaklaşık 22.08 milyar TL", "Brüt finansal borç 53.09 milyar TL"} {
		if !strings.Contains(readings, want) {
			t.Fatalf("structured bilanco reading %q missing:\n%s", want, readings)
		}
	}
	if strings.Contains(readings, "Hesaplanmadı") {
		t.Fatalf("structured bilanco rows should avoid manual-review financial readings:\n%s", readings)
	}
}

func TestKAPPDFFinancialReadingBlocksIntegrityDetectsSlashRatio(t *testing.T) {
	if !kapPDFFinancialReadingBlocksIntegrity("Sermaye yapısı", "Hesaplanmadı: özkaynak/aktif 2.03x çıktı ve insan incelemesi gerektirir.") {
		t.Fatalf("slash-separated equity/assets warning should block financial integrity")
	}
}

func TestKAPRawOwnershipRowsFiltersNarrativeFalsePositives(t *testing.T) {
	raw := professional.KAPRawDataBundle{
		OwnershipFacts: []kapingest.OwnershipFact{
			{
				HolderName: "Alarko Holding A.Ş.",
				ShareRatio: floatPtrForReportTest(49),
				Period:     stringPtrForReportTest("2025/12"),
				Source:     kapingest.DocumentFactSource{Snippet: "Alarko Holding A.Ş. %49"},
				Confidence: 0.90,
				SourceFile: "faaliyet.pdf",
			},
			{
				HolderName: "teknoloji savunma ve güvenlik şirketidir.",
				ShareRatio: floatPtrForReportTest(50),
				Source:     kapingest.DocumentFactSource{Snippet: "teknoloji savunma ve güvenlik şirketidir."},
				Confidence: 0.95,
			},
			{
				HolderName: "50+ yaş",
				ShareRatio: floatPtrForReportTest(50),
				Source:     kapingest.DocumentFactSource{Snippet: "50+ yaş"},
				Confidence: 0.95,
			},
			{
				HolderName: "bir savunma sanayii şirketine dönüşüm",
				ShareRatio: floatPtrForReportTest(30),
				Source:     kapingest.DocumentFactSource{Snippet: "bir savunma sanayii şirketine dönüşüm"},
				Confidence: 0.95,
			},
		},
	}

	rows := kapRawOwnershipRows(raw, 0)
	if len(rows) != 1 || rows[0][0] != "Alarko Holding A.Ş." {
		t.Fatalf("unexpected ownership rows: %+v", rows)
	}
	summary := kapOwnershipSummaryText(&raw, 10)
	if !strings.Contains(summary, "Alarko Holding A.Ş.") || strings.Contains(summary, "50+ yaş") || strings.Contains(summary, "güvenlik şirketidir") {
		t.Fatalf("unexpected ownership summary: %s", summary)
	}
}

func TestRetailTLPricesUseBISTTickSize(t *testing.T) {
	if got := retailPrice(403.93, "TL"); got != "404.00 TL" {
		t.Fatalf("retailPrice = %s, want 404.00 TL", got)
	}
	levels := actionSignalLevels(investorqa.ActionSignal{
		EntryMin: 382.38,
		EntryMax: 409.01,
		StopLoss: 356.33,
		Target1:  446.22,
	}, "TL")
	for _, want := range []string{"382.50 TL", "409.00 TL", "356.25 TL", "446.25 TL"} {
		if !strings.Contains(levels, want) {
			t.Fatalf("expected %q in rounded action levels: %s", want, levels)
		}
	}
}

func flattenReportTestRows(rows [][]string) []string {
	out := []string{}
	for _, row := range rows {
		out = append(out, row...)
	}
	return out
}

func testBilancoQ1Field(desc string, value float64) domain.BilancoField {
	return domain.BilancoField{DescTR: desc, Years: map[string][]*float64{"2026": {nil, nil, nil, floatPtrForReportTest(value)}}}
}

func reportTestHasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func floatPtrForReportTest(value float64) *float64 {
	return &value
}

func stringPtrForReportTest(value string) *string {
	return &value
}

func writeJSONLForStorageTest[T any](t *testing.T, path string, rows []T) {
	t.Helper()
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("ensure jsonl dir: %v", err)
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("marshal jsonl row: %v", err)
		}
		lines = append(lines, string(raw))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write jsonl %s: %v", path, err)
	}
}

func writeJSONForStorageTest(t *testing.T, path string, value any) {
	t.Helper()
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("ensure json dir: %v", err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write json %s: %v", path, err)
	}
}

func TestCryptoProfessionalReportUsesCryptoDataContext(t *testing.T) {
	result := analysis.SymbolAnalysis{
		Symbol:       "BTCUSDT",
		Exchange:     "BINANCE",
		AssetType:    ohlcv.AssetTypeCrypto,
		CompanyName:  "Bitcoin / USDT",
		AnalysisDate: "2026-06-13",
		Currency:     "USDT",
		Timeframes: map[string]analysis.TimeframeAnalysis{
			"1D": {
				Timeframe:         "1D",
				LastClose:         65000,
				LastVolume:        1000,
				NearestSupport:    &ohlcv.SupportResistanceLevel{Price: 62000},
				NearestResistance: &ohlcv.SupportResistanceLevel{Price: 68000},
				Indicators:        ohlcv.IndicatorSnapshot{RSI14: 45, MACDHistogram: -10, ATR14: 2200},
				TradePlan:         ohlcv.TradePlan{Direction: "neutral", Rejected: true, RejectReason: "Nötr eğilim yeterli yön avantajı sağlamıyor"},
				Professional:      professional.TimeframeReport{Backtest: professional.BacktestResult{LookbackBars: 260, Trades: 12}},
				TrendBias:         "neutral",
				Score:             50,
				IndicatorSignals:  []ohlcv.IndicatorResult{{Name: "MACD", Signal: "bearish", Computed: true, Confidence: 0.7}},
				PatternScans:      []ohlcv.PatternScanResult{{Name: "Test", Matched: false}},
				PatternCandidates: []ohlcv.PatternResult{{Name: "Test", RejectionReasons: []string{"not_current_completed_pattern"}}},
			},
		},
		Professional: professional.Report{
			Coverage: professional.CoverageReport{
				Score:     50,
				Available: []string{"tradingview_ohlcv_price_volume", "technical_indicators"},
				Missing:   []string{"onchain_mvrv_nupl_sopr_realized_cap", "derivatives_funding_open_interest_liquidations"},
			},
			DataQuality: 50,
			Company:     professional.CompanyProfile{Sector: "Crypto Assets", Industry: "Digital Asset"},
			Peers:       professional.PeerComparison{ValuationSignal: "not_applicable"},
			Valuation:   professional.ValuationAnalysis{Ratios: map[string]float64{}, FairValue: professional.FairValueRange{Confidence: 0.25}},
		},
		InstitutionalValidation: analysis.InstitutionalValidation{Status: "limited", Score: 67, Summary: "Kripto veri kapsamı sınırlı."},
		Disclaimer:              ohlcv.Disclaimer,
	}

	html := professionalReportHTML(result)
	for _, want := range []string{
		"Kripto Veri Kapsamı",
		"Geleneksel finansal tablo ve çarpan değerlemesi bu kripto raporuna dahil edilmedi.",
		"blokzincir değerleme verileri",
		"fonlama, açık pozisyon ve tasfiye verileri",
		"Son Kapanış",
		"65000.00 USDT",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("crypto report HTML does not contain %q\n%s", want, html)
		}
	}
	if strings.Contains(html, "Piyasa değeri") || strings.Contains(html, "F/K | PD/DD") {
		t.Fatalf("crypto report should not render equity valuation rows\n%s", html)
	}
	if strings.Contains(html, "KAP") || strings.Contains(html, "BIST") {
		t.Fatalf("crypto report should not mention KAP or BIST\n%s", html)
	}
	if strings.Contains(html, "onchain_mvrv_nupl_sopr_realized_cap") {
		t.Fatalf("crypto report should render retail labels instead of raw data keys\n%s", html)
	}
}
