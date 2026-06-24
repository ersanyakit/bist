package professional

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hissebot/internal/services/tcmb"
)

func TestBuildTCMBEVDSContextForSymbolComputesMacroFeatures(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(`{
		"fetched_at":"2026-06-20T08:00:00Z",
		"stats":{"data_groups":5,"series":6}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeEVDSMacroSeries(t, root, "bie_bispolfaiz", "TP.BISPOLFAIZ.TUR", "Türkiye politika faizi", "yüzde", []tcmb.EVDSPoint{
		evdsMacroPoint("2026-03", "38.0000000000"),
		evdsMacroPoint("2026-06", "37.0000000000"),
	})
	writeEVDSMacroSeries(t, root, "bie_dkdovizgn", "TP.DK.USD.A", "ABD doları", "TRY", []tcmb.EVDSPoint{
		evdsMacroPoint("2026-03-20", "40.0000000000"),
		evdsMacroPoint("2026-06-19", "46.0000000000"),
		evdsMacroPoint("2026-06-22", "99.0000000000"),
	})
	writeEVDSMacroSeries(t, root, "bie_abres2", "TP.AB.TOPLAM", "Toplam rezerv", "milyon ABD doları", []tcmb.EVDSPoint{
		evdsMacroPoint("2026-03-20", "170,000.0000000000"),
		evdsMacroPoint("2026-06-12", "152,000.0000000000"),
	})
	writeEVDSMacroSeries(t, root, "bie_bekodtufeyeni", "TP.BEKODTUFEYENI.BT20", "(20.00-20.99) 12 Ay Sonrası Yıllık Tüketici Enflasyonu Beklentisi Olasılığı (%)", "yüzde", []tcmb.EVDSPoint{
		evdsMacroPoint("2026-06", "50.0000000000"),
	})
	writeEVDSMacroSeries(t, root, "bie_bekodtufeyeni", "TP.BEKODTUFEYENI.BT30", "(30.00-30.99) 12 Ay Sonrası Yıllık Tüketici Enflasyonu Beklentisi Olasılığı (%)", "yüzde", []tcmb.EVDSPoint{
		evdsMacroPoint("2026-06", "50.0000000000"),
	})
	writeEVDSMacroSeries(t, root, "bie_tufe1yi", "TP.TUFE1YI.T26", "İçecekler", "endeks", []tcmb.EVDSPoint{
		evdsMacroPoint("2025-05", "1,000.0000000000"),
		evdsMacroPoint("2026-05", "1,350.0000000000"),
	})

	report := buildTCMBEVDSContextForSymbol(root, "AEFES", CompanyProfile{
		Sector:   "İMALAT",
		Industry: "GIDA, İÇECEK VE TÜTÜN",
	}, time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC))

	if !report.AnalysisReady || !report.ScoreEligible {
		t.Fatalf("macro context should be ready and score eligible: %+v", report)
	}
	if report.DataQualityScore != 100 {
		t.Fatalf("unexpected quality score: %.2f", report.DataQualityScore)
	}
	if report.ScoreAdjustment >= 0 || report.ScoreAdjustment < -8 {
		t.Fatalf("expected bounded negative macro adjustment, got %.2f", report.ScoreAdjustment)
	}
	usd := tcmbIndicatorByKey(report.Indicators, "usd_try")
	if !usd.Computed || usd.Value != 46 {
		t.Fatalf("future FX point must be excluded: %+v", usd)
	}
	reserves := tcmbIndicatorByKey(report.Indicators, "gross_reserves")
	if !reserves.Computed || reserves.Value != 152000 {
		t.Fatalf("raw reserve value must be parsed: %+v", reserves)
	}
	ppi := tcmbIndicatorByKey(report.Indicators, "sector_ppi")
	if !ppi.Computed || ppi.SeriesCode != "TP.TUFE1YI.T26" || ppi.ChangeYoY != 35 {
		t.Fatalf("sector PPI mapping not applied: %+v", ppi)
	}
}

func TestTCMBMacroContextDisablesScoreForHistoricalAsOf(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte(`{
		"fetched_at":"2026-06-20T08:00:00Z",
		"stats":{"data_groups":3,"series":3}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeEVDSMacroSeries(t, root, "bie_bispolfaiz", "TP.BISPOLFAIZ.TUR", "Türkiye politika faizi", "yüzde", []tcmb.EVDSPoint{evdsMacroPoint("2025-01", "45")})
	writeEVDSMacroSeries(t, root, "bie_dkdovizgn", "TP.DK.USD.A", "ABD doları", "TRY", []tcmb.EVDSPoint{evdsMacroPoint("2025-01-10", "35")})
	writeEVDSMacroSeries(t, root, "bie_abres2", "TP.AB.TOPLAM", "Toplam rezerv", "milyon ABD doları", []tcmb.EVDSPoint{evdsMacroPoint("2025-01-10", "150,000.00")})

	report := buildTCMBEVDSContextForSymbol(root, "ALGYO", CompanyProfile{Industry: "GAYRİMENKUL YATIRIM ORTAKLIKLARI"}, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC))
	if report.PointInTimeSafe || report.ScoreEligible || report.ScoreAdjustment != 0 {
		t.Fatalf("future-fetched archive must not affect historical score: %+v", report)
	}
}

func TestTCMBDocumentEvidenceIsRequiredForForecastDecisionUse(t *testing.T) {
	report := TCMBEVDSContextReport{
		AnalysisReady: true,
		ForecastImpact: TCMBMacroForecastImpact{
			Computed:        true,
			Direction:       "positive",
			Severity:        "moderate",
			Confidence:      85,
			ScoreAdjustment: 4,
			DecisionUse:     "score_and_gate_input",
			Summary:         "Makro rejim fiyat beklentisini destekliyor.",
		},
	}
	applyTCMBDocumentEvidenceToForecastImpact(&report, TCMBContextReport{
		Computed:                  false,
		DocumentCount:             10,
		RequiredCategoriesMissing: []string{"ppk_kararlari"},
		Warnings:                  []string{"tcmb_category_missing:ppk_kararlari"},
	})

	if report.ForecastImpact.DecisionUse != "audit_only" {
		t.Fatalf("missing TCMB documents must limit forecast impact to audit-only, got %+v", report.ForecastImpact)
	}
	if report.ForecastImpact.Confidence > 50 {
		t.Fatalf("missing TCMB documents must cap confidence, got %+v", report.ForecastImpact)
	}
	if !containsString(report.ForecastImpact.Blockers, "tcmb_document_evidence_missing:ppk_kararlari") {
		t.Fatalf("missing document blocker not recorded: %+v", report.ForecastImpact.Blockers)
	}

	applyTCMBDocumentEvidenceToForecastImpact(&report, TCMBContextReport{
		Computed:        true,
		DocumentCount:   481,
		TextIndexPath:   "data/macro/tcmb/text_index.jsonl",
		TextUsableCount: 481,
		Categories: []TCMBCategoryStat{
			{ID: "basin_duyurulari", DocumentCount: 3},
			{ID: "ppk_kararlari", DocumentCount: 163},
		},
	})
	if !report.ForecastImpact.DocumentEvidenceIncluded || report.ForecastImpact.DocumentCount != 481 {
		t.Fatalf("TCMB document evidence not attached: %+v", report.ForecastImpact)
	}
	if len(report.ForecastImpact.DocumentCategories) != 2 || report.ForecastImpact.DocumentCategories[0] != "basin_duyurulari" {
		t.Fatalf("TCMB document categories not sorted/attached: %+v", report.ForecastImpact.DocumentCategories)
	}
}

func TestTCMBMacroForecastConfidenceIsCappedByStaleSeries(t *testing.T) {
	warnings := []string{"tcmb_evds_stale_total_credit"}
	impact := buildTCMBMacroForecastImpact(true, true, true, 100, -1.5, "karışık / nötr", nil, warnings)
	if impact.Confidence != 85 {
		t.Fatalf("stale EVDS series must cap macro forecast confidence below 100: %+v", impact)
	}
	report := TCMBEVDSContextReport{
		Warnings:       warnings,
		ForecastImpact: impact,
	}
	applyTCMBDocumentEvidenceToForecastImpact(&report, TCMBContextReport{
		Computed:        true,
		DocumentCount:   486,
		TextUsableCount: 486,
		Categories:      []TCMBCategoryStat{{ID: "ppk_kararlari", DocumentCount: 1}},
	})
	if report.ForecastImpact.Confidence != 85 {
		t.Fatalf("document evidence must not override stale-series confidence cap: %+v", report.ForecastImpact)
	}
}

func writeEVDSMacroSeries(t *testing.T, root, group, code, name, unit string, points []tcmb.EVDSPoint) {
	t.Helper()
	dir := filepath.Join(root, "series", group)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dataset := tcmb.EVDSSeriesDataset{
		DataGroup: tcmb.EVDSDataGroup{DataGroupCode: group, Unit: unit},
		Series:    tcmb.EVDSSeriesMeta{SeriesCode: code, SeriesName: name},
		Points:    points,
	}
	raw, err := json.Marshal(dataset)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, strings.ReplaceAll(code, ".", "_")+".json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func evdsMacroPoint(date, raw string) tcmb.EVDSPoint {
	return tcmb.EVDSPoint{Date: date, DisplayDate: date, RawValue: &raw}
}

func tcmbIndicatorByKey(indicators []TCMBMacroIndicator, key string) TCMBMacroIndicator {
	for _, indicator := range indicators {
		if indicator.Key == key {
			return indicator
		}
	}
	return TCMBMacroIndicator{}
}
