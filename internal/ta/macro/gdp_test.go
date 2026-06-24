package macro

import "testing"

func TestAnalyzeGDPComputesMacroImpact(t *testing.T) {
	ctx := AnalyzeGDP(GDPDataset{
		Source:    "test",
		SourceURL: "https://cip.tuik.gov.tr/",
		Points: []GDPPoint{
			{Year: 2024, GDPThousandTRY: 1000, PerCapitaTRY: 500, PerCapitaUSD: 110},
			{Year: 2023, GDPThousandTRY: 700, PerCapitaTRY: 350, PerCapitaUSD: 100},
		},
	})

	if !ctx.Computed {
		t.Fatalf("expected computed context: %+v", ctx)
	}
	if ctx.LatestYear != 2024 || ctx.PreviousYear != 2023 {
		t.Fatalf("years mismatch: %+v", ctx)
	}
	if ctx.PerCapitaUSDYoY != 10 {
		t.Fatalf("usd yoy = %.2f", ctx.PerCapitaUSDYoY)
	}
	if ctx.Score <= 50 {
		t.Fatalf("expected supportive score, got %.2f", ctx.Score)
	}
	if ctx.EquityImpact == "" || ctx.Interpretation == "" {
		t.Fatalf("missing narrative: %+v", ctx)
	}
}

func TestAnalyzeGDPRequiresTwoYears(t *testing.T) {
	ctx := AnalyzeGDP(GDPDataset{Points: []GDPPoint{{Year: 2024, GDPThousandTRY: 1000, PerCapitaTRY: 500, PerCapitaUSD: 100}}})
	if ctx.Computed {
		t.Fatalf("expected not computed: %+v", ctx)
	}
	if ctx.DataQualityWarning == "" {
		t.Fatalf("expected warning: %+v", ctx)
	}
}

func TestAnalyzeGDPSeparatesFetchYearFromObservationYear(t *testing.T) {
	ctx := AnalyzeGDP(GDPDataset{
		FetchedAt: "2026-06-18T12:15:30Z",
		Points: []GDPPoint{
			{Year: 2024, GDPThousandTRY: 1000, PerCapitaTRY: 500, PerCapitaUSD: 110},
			{Year: 2023, GDPThousandTRY: 700, PerCapitaTRY: 350, PerCapitaUSD: 100},
		},
	})
	if ctx.ReferenceYear != 2026 || ctx.LatestYear != 2024 || ctx.ObservationLagYears != 2 {
		t.Fatalf("fetch/observation years were conflated: %+v", ctx)
	}
	if ctx.FreshnessStatus != "stale_annual_actual" || ctx.DataQualityWarning == "" {
		t.Fatalf("stale actual GDP should be explicit: %+v", ctx)
	}
	if ctx.Score > 59 {
		t.Fatalf("stale GDP score must be capped, got %.1f", ctx.Score)
	}
}
