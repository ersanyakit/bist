package financials

import (
	"math"
	"path/filepath"
	"testing"

	"hissebot/internal/domain"
)

func TestCalculateEquityFinancialRatiosAreDeterministic(t *testing.T) {
	info := &domain.BilancoInfo{Ticker: "TEST", Data: map[string]domain.BilancoField{
		"1A":  field("Dönen Varlıklar", 400, 300, 200, 100),
		"1AA": field("Nakit ve Nakit Benzerleri", 80, 60, 40, 20),
		"1AC": field("Ticari Alacaklar", 120, 90, 60, 30),
		"1AF": field("Stoklar", 100, 75, 50, 25),
		"1BL": field("Toplam Varlıklar", 1000, 900, 800, 700),
		"2A":  field("Kısa Vadeli Yükümlülükler", 200, 150, 100, 50),
		"2N":  field("Özkaynaklar", 500, 450, 400, 350),
		"3C":  field("Satış Gelirleri", 1000, 800, 600, 400),
		"3CA": field("Satışların Maliyeti", -700, -560, -420, -280),
		"3D":  field("Brüt Kar", 300, 240, 180, 120),
		"3DF": field("Faaliyet Karı", 200, 160, 120, 80),
		"3L":  field("Net Kar", 100, 80, 60, 40),
	}}
	info.Data["1BL"].Years["2025"] = []*float64{ptr(1000), ptr(900), ptr(800), ptr(700)}
	info.Data["2N"].Years["2025"] = []*float64{ptr(500), ptr(450), ptr(400), ptr(350)}

	got := CalculateEquity(info)
	assertQuarter(t, got, "CariOran", "2026", "Q4", 2)
	assertQuarter(t, got, "NakitOran", "2026", "Q4", 0.4)
	assertQuarter(t, got, "AsitTestOran", "2026", "Q4", 1.5)
	assertQuarter(t, got, "LikiditeOran", "2026", "Q4", 1)
	assertQuarter(t, got, "NetKarMarji", "2026", "Q4", 10)
	assertQuarter(t, got, "BrutKarMarji", "2026", "Q4", 30)
	assertQuarter(t, got, "FaaliyetKarMarji", "2026", "Q4", 20)
	assertQuarter(t, got, "ROA", "2026", "Q4", 10)
	assertQuarter(t, got, "ROE", "2026", "Q4", 20)
	assertQuarter(t, got, "StokDevirHizi", "2026", "Q4", 7)
	assertQuarter(t, got, "AlacakDevirHizi", "2026", "Q4", 8.333333333333334)
	assertQuarter(t, got, "VarlikDevirHizi", "2026", "Q4", 1)
}

func TestCalculateEquitySkipsZeroDenominatorRatios(t *testing.T) {
	info := &domain.BilancoInfo{Ticker: "ZERO", Data: map[string]domain.BilancoField{
		"1A":  field("Dönen Varlıklar", 400, 300, 200, 100),
		"1AA": field("Nakit ve Nakit Benzerleri", 80, 60, 40, 20),
		"1AC": field("Ticari Alacaklar", 120, 90, 60, 30),
		"1AF": field("Stoklar", 100, 75, 50, 25),
		"1BL": field("Toplam Varlıklar", 1000, 900, 800, 700),
		"2A":  field("Kısa Vadeli Yükümlülükler", 0, 150, 100, 50),
		"2N":  field("Özkaynaklar", 500, 450, 400, 350),
		"3C":  field("Satış Gelirleri", 0, 800, 600, 400),
		"3CA": field("Satışların Maliyeti", -700, -560, -420, -280),
		"3D":  field("Brüt Kar", 300, 240, 180, 120),
		"3DF": field("Faaliyet Karı", 200, 160, 120, 80),
		"3L":  field("Net Kar", 100, 80, 60, 40),
	}}

	got := CalculateEquity(info)
	assertQuarterMissing(t, got, "CariOran", "2026", "Q4")
	assertQuarterMissing(t, got, "NetKarMarji", "2026", "Q4")
	assertQuarter(t, got, "CariOran", "2026", "Q3", 2)
	assertQuarter(t, got, "NetKarMarji", "2026", "Q3", 10)
}

func TestCalculateEquityUsesTTMIncomeAndAverageBalanceForROE(t *testing.T) {
	info := &domain.BilancoInfo{Ticker: "ROE", Data: map[string]domain.BilancoField{
		"3L": {
			DescTR: "Net Kar",
			Years: map[string][]*float64{
				"2026": {nil, nil, nil, ptr(30)},
				"2025": {ptr(100), nil, nil, ptr(20)},
			},
		},
		"2N": {
			DescTR: "Özkaynaklar",
			Years: map[string][]*float64{
				"2026": {nil, nil, nil, ptr(600)},
				"2025": {nil, nil, nil, ptr(400)},
			},
		},
		"1BL": {
			DescTR: "Toplam Varlıklar",
			Years: map[string][]*float64{
				"2026": {nil, nil, nil, ptr(1200)},
				"2025": {nil, nil, nil, ptr(800)},
			},
		},
	}}

	got := CalculateEquity(info)
	assertQuarter(t, got, "ROE", "2026", "Q1", 22)
	assertQuarter(t, got, "ROA", "2026", "Q1", 11)
}

func TestRatioRejectsInvalidDenominator(t *testing.T) {
	for _, denominator := range []float64{0, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := ratio(10, denominator); !math.IsNaN(got) {
			t.Fatalf("ratio with denominator %.2f = %.8f, want NaN", denominator, got)
		}
	}
}

func TestCalculateEquityWithAuditCarriesFormulaLineage(t *testing.T) {
	info := &domain.BilancoInfo{Ticker: "AUDIT", Data: map[string]domain.BilancoField{
		"1A": field("Dönen Varlıklar", 400, 300, 200, 100),
		"2A": field("Kısa Vadeli Yükümlülükler", 200, 150, 100, 50),
	}}

	got, audit := CalculateEquityWithAudit(info)
	if _, ok := got["CariOran"]; !ok {
		t.Fatalf("expected CariOran calculation, got %#v", got)
	}
	if audit == nil {
		t.Fatal("expected calculation audit")
	}
	metric := audit.Metrics["CariOran"]
	if metric.Formula != "1A/2A" {
		t.Fatalf("CariOran formula = %q, want 1A/2A", metric.Formula)
	}
	if len(metric.InputFields) != 2 || metric.InputFields[0] != "1A" || metric.InputFields[1] != "2A" {
		t.Fatalf("CariOran input fields = %#v", metric.InputFields)
	}
	if audit.BacktestSafe {
		t.Fatalf("expected audit to be unsafe without publish dates, got %+v", audit)
	}
}

func TestGoldenFinancialRatioDataset(t *testing.T) {
	report, err := ValidateGoldenRatios(filepath.Join("..", "..", "..", "data", "seed", "golden_financial_ratios.json"))
	if err != nil {
		t.Fatalf("validate golden ratios: %v", err)
	}
	if report.Status != "pass" {
		t.Fatalf("golden ratio report = %+v", report)
	}
}

func field(desc string, q4, q3, q2, q1 float64) domain.BilancoField {
	return domain.BilancoField{
		DescTR: desc,
		Years: map[string][]*float64{
			"2026": {ptr(q4), ptr(q3), ptr(q2), ptr(q1)},
		},
	}
}

func ptr(value float64) *float64 {
	return &value
}

func assertQuarter(t *testing.T, got map[string]domain.YearQuarter, name, year, quarter string, want float64) {
	t.Helper()
	value, ok := quarterValue(got, name, year, quarter)
	if !ok {
		t.Fatalf("missing %s %s %s", name, year, quarter)
	}
	if math.Abs(value-want) > 1e-9 {
		t.Fatalf("%s %s %s = %.12f, want %.12f", name, year, quarter, value, want)
	}
}

func assertQuarterMissing(t *testing.T, got map[string]domain.YearQuarter, name, year, quarter string) {
	t.Helper()
	if value, ok := quarterValue(got, name, year, quarter); ok {
		t.Fatalf("expected missing %s %s %s, got %.12f", name, year, quarter, value)
	}
}

func quarterValue(got map[string]domain.YearQuarter, name, year, quarter string) (float64, bool) {
	yearValues, ok := got[name][year]
	if !ok {
		return 0, false
	}
	var value *float64
	switch quarter {
	case "Q1":
		value = yearValues.Q1
	case "Q2":
		value = yearValues.Q2
	case "Q3":
		value = yearValues.Q3
	case "Q4":
		value = yearValues.Q4
	}
	if value == nil {
		return 0, false
	}
	return *value, true
}
