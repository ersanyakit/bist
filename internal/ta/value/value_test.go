package value

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAnalyzeOperationalCompanyComputesValueStack(t *testing.T) {
	equitiesDir := writeFinancialFixture(t, "TEST", false)

	report := Analyze(Input{
		EquitiesDir:  equitiesDir,
		Symbol:       "TEST",
		Sector:       "Savunma ve Elektronik",
		Industry:     "Savunma Elektroniği",
		Currency:     "TRY",
		CurrentPrice: 10,
		AsOf:         time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
	})

	if !report.Computed {
		t.Fatalf("expected computed value report, got %+v", report)
	}
	if report.SectorModel.Model != "owner_earnings_dcf" {
		t.Fatalf("sector model = %s", report.SectorModel.Model)
	}
	if report.OwnerEarnings.TTM <= 0 || report.NormalizedFCF.Median5Y <= 0 {
		t.Fatalf("cash flow stack not computed: owner=%+v fcf=%+v", report.OwnerEarnings, report.NormalizedFCF)
	}
	if !report.MarginOfSafety.Computed || report.IntrinsicValue.Base <= report.CurrentPrice {
		t.Fatalf("margin/intrinsic value not useful: intrinsic=%+v margin=%+v", report.IntrinsicValue, report.MarginOfSafety)
	}
	if !report.FairValue.Computed || report.FairValue.Status == "" {
		t.Fatalf("fair value conclusion not computed: %+v", report.FairValue)
	}
	if report.CapitalAllocation.Dilution5YPct != 0 {
		t.Fatalf("dilution = %.2f, want 0", report.CapitalAllocation.Dilution5YPct)
	}
	if report.Moat.Score <= 0 || report.QualityScore <= 0 || report.Confidence <= 0 {
		t.Fatalf("quality stack not scored: moat=%+v quality=%.2f confidence=%.2f", report.Moat, report.QualityScore, report.Confidence)
	}
	if !hasValueCheck(report, "intrinsic_value", "pass") {
		t.Fatalf("missing passed intrinsic value check: %+v", report.Checks)
	}
	if !report.BuffettChecklist.Computed || len(report.BuffettChecklist.Requirements) == 0 {
		t.Fatalf("Buffett checklist missing: %+v", report.BuffettChecklist)
	}
	if !hasValueCheck(report, "buffett_value_checklist", report.BuffettChecklist.Status) {
		t.Fatalf("missing Buffett checklist value check: status=%s checks=%+v", report.BuffettChecklist.Status, report.Checks)
	}
	if !hasBuffettRequirement(report, "one_dollar_retained_earnings_test", "missing") {
		t.Fatalf("Buffett checklist should expose missing one-dollar retained earnings test: %+v", report.BuffettChecklist.Requirements)
	}
}

func TestAnalyzeTechnologyCompanyUsesTechnologyGrowthQualityModel(t *testing.T) {
	equitiesDir := writeFinancialFixture(t, "TECH", false)
	report := Analyze(Input{
		EquitiesDir: equitiesDir, Symbol: "TECH",
		Sector: "TEKNOLOJİ", Industry: "BİLİŞİM",
		Currency: "TRY", CurrentPrice: 10,
		AsOf: time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC),
	})
	if report.SectorModel.Model != "technology_growth_quality" {
		t.Fatalf("technology model = %s", report.SectorModel.Model)
	}
	if !report.OwnerEarnings.Applicable || !report.NormalizedFCF.Applicable {
		t.Fatalf("technology cash-flow stack should be applicable: owner=%+v fcf=%+v", report.OwnerEarnings, report.NormalizedFCF)
	}
}

func TestAnalyzeComputesOneDollarRetainedEarningsTestFromPriceHistory(t *testing.T) {
	equitiesDir := writeFinancialFixture(t, "RETAIN", false)
	asOf := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	report := Analyze(Input{
		EquitiesDir:  equitiesDir,
		Symbol:       "RETAIN",
		Sector:       "Sanayi",
		Industry:     "Savunma",
		Currency:     "TRY",
		CurrentPrice: 20,
		AsOf:         asOf,
		PriceHistory: []PriceObservation{{Time: asOf.AddDate(-5, 0, 0), Close: 5}, {Time: asOf, Close: 20}},
	})

	if !report.RetainedEarnings.Computed || report.RetainedEarnings.Ratio < 1 {
		t.Fatalf("retained earnings test not computed/passable: %+v", report.RetainedEarnings)
	}
	if !hasBuffettRequirement(report, "one_dollar_retained_earnings_test", "pass") {
		t.Fatalf("Buffett checklist should pass one-dollar test with price history: %+v", report.BuffettChecklist.Requirements)
	}
}

func TestAnalyzeCombinesKAPDocumentsAndTUIKGDPWithFairValue(t *testing.T) {
	equitiesDir := writeFinancialFixture(t, "DOCS", false)
	writeKAPDocumentFixture(t, equitiesDir, "DOCS")
	gdpPath := writeGDPFixture(t, filepath.Join(filepath.Dir(equitiesDir), "macro", "tuik_gdp.json"))

	report := Analyze(Input{
		EquitiesDir:  equitiesDir,
		Symbol:       "DOCS",
		Sector:       "Sanayi",
		Industry:     "Gıda",
		Currency:     "TRY",
		CurrentPrice: 10,
		AsOf:         time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
		MacroGDPFile: gdpPath,
	})

	if !report.DocumentEvidence.Computed {
		t.Fatalf("expected KAP document evidence: %+v", report.DocumentEvidence)
	}
	if report.DocumentEvidence.CoverageScore < 60 {
		t.Fatalf("KAP document coverage too weak: %+v", report.DocumentEvidence)
	}
	if !report.MacroGDP.Computed || report.MacroGDP.LatestYear != 2024 {
		t.Fatalf("expected TÜİK GDP context: %+v", report.MacroGDP)
	}
	if !report.FairValue.Computed || !strings.Contains(strings.Join(report.FairValue.DataInputs, " "), "KAP belge kanıtı") {
		t.Fatalf("fair value conclusion did not carry evidence inputs: %+v", report.FairValue)
	}
	if !hasValueCheck(report, "kap_document_evidence", "pass") {
		t.Fatalf("missing KAP document evidence check: %+v", report.Checks)
	}
}

func TestAnalyzeBankUsesBookValueSectorModel(t *testing.T) {
	equitiesDir := writeFinancialFixture(t, "BANK", true)

	report := Analyze(Input{
		EquitiesDir:  equitiesDir,
		Symbol:       "BANK",
		Sector:       "Banka",
		Industry:     "Mevduat Bankası",
		Currency:     "TRY",
		CurrentPrice: 15,
		AsOf:         time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
	})

	if !report.Computed {
		t.Fatalf("expected computed bank value report, got %+v", report)
	}
	if report.SectorModel.Model != "bank_residual_income" {
		t.Fatalf("sector model = %s", report.SectorModel.Model)
	}
	if report.OwnerEarnings.Applicable {
		t.Fatalf("owner earnings should not be primary for banks: %+v", report.OwnerEarnings)
	}
	if report.IntrinsicValue.Method != "bank_residual_income" || report.IntrinsicValue.Base <= 0 {
		t.Fatalf("bank intrinsic value not computed with book model: %+v", report.IntrinsicValue)
	}
	if !hasValueCheck(report, "owner_earnings", "not_applicable") {
		t.Fatalf("bank report should mark owner earnings not applicable: %+v", report.Checks)
	}
}

func TestAnalyzeBankUFRSKSchemaUsesBankLineItems(t *testing.T) {
	equitiesDir := writeBankSchemaFinancialFixture(t, "ISCTR")

	report := Analyze(Input{
		EquitiesDir:  equitiesDir,
		Symbol:       "ISCTR",
		Sector:       "Banka",
		Industry:     "Mevduat Bankası",
		Currency:     "TRY",
		CurrentPrice: 15,
		AsOf:         time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
	})

	if !report.Computed {
		t.Fatalf("expected computed UFRS_K bank value report, got %+v", report)
	}
	if report.SectorModel.Model != "bank_residual_income" {
		t.Fatalf("sector model = %s", report.SectorModel.Model)
	}
	if report.Years[len(report.Years)-1].Equity <= 0 || report.Years[len(report.Years)-1].TotalAssets <= 0 {
		t.Fatalf("bank schema line items not mapped: latest year=%+v", report.Years[len(report.Years)-1])
	}
	if report.IntrinsicValue.Method != "bank_residual_income" || report.IntrinsicValue.Base <= 0 {
		t.Fatalf("bank intrinsic value not computed from UFRS_K schema: %+v", report.IntrinsicValue)
	}
	if report.DataQuality < 70 {
		t.Fatalf("bank data quality too low after UFRS_K mapping: %.2f", report.DataQuality)
	}
}

func writeKAPDocumentFixture(t *testing.T, equitiesDir, symbol string) {
	t.Helper()
	files := []struct {
		index int
		name  string
		title string
		typ   string
		class string
	}{
		{100, "DOCS Finansal Rapor 31.12.2025.pdf", "Finansal Rapor", "FR", "FR"},
		{101, "DOCS 2025 Faaliyet Raporu.pdf", "Faaliyet Raporu", "CA", "ODA"},
		{102, "DOCS BDR.pdf", "Bağımsız Denetim Raporu", "FR", "FR"},
	}
	for _, file := range files {
		path := filepath.Join(equitiesDir, symbol, "kap", "attachments", "2025", strconv.Itoa(file.index), "obj_"+file.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir kap fixture: %v", err)
		}
		if err := os.WriteFile(path, []byte("pdf"), 0o644); err != nil {
			t.Fatalf("write kap fixture: %v", err)
		}
		detailPath := filepath.Join(equitiesDir, "_kap", "details", symbol, strconv.Itoa(file.index)+".json")
		if err := os.MkdirAll(filepath.Dir(detailPath), 0o755); err != nil {
			t.Fatalf("mkdir detail fixture: %v", err)
		}
		raw := `{"raw":[{"disclosure":{"disclosureBasic":{"title":"` + file.title + `","disclosureType":"` + file.typ + `","disclosureClass":"` + file.class + `","publishDate":"2025.03.10 18:01:02","year":2025,"period":"3AB"}}}]}`
		if err := os.WriteFile(detailPath, []byte(raw), 0o644); err != nil {
			t.Fatalf("write detail fixture: %v", err)
		}
	}
}

func writeGDPFixture(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir gdp fixture: %v", err)
	}
	raw := `{
  "source": "TÜİK Coğrafi İstatistik Portalı",
  "source_url": "https://cip.tuik.gov.tr/Home/GetMapData",
  "methodology": "test",
  "fetched_at": "2026-06-13T18:35:03Z",
  "points": [
    {"year": 2024, "gdp_thousand_try": 44587225441, "per_capita_try": 503075, "per_capita_usd": 15325, "implied_population": 88629286},
    {"year": 2023, "gdp_thousand_try": 27091469064, "per_capita_try": 305569, "per_capita_usd": 13007, "implied_population": 88658857}
  ]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write gdp fixture: %v", err)
	}
	return path
}

func TestAnalyzeOperationalCompanyFallsBackToBookCrossCheckWhenDCFIsNegative(t *testing.T) {
	equitiesDir := writeWeakCashFlowFixture(t, "WEAK")

	report := Analyze(Input{
		EquitiesDir:  equitiesDir,
		Symbol:       "WEAK",
		Sector:       "Sanayi",
		Industry:     "Gıda",
		Currency:     "TRY",
		CurrentPrice: 8,
		AsOf:         time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
	})

	if !report.Computed {
		t.Fatalf("expected book cross-check to compute a conservative value report, got %+v", report)
	}
	if report.IntrinsicValue.Method != "owner_earnings_dcf_book_value_crosscheck" {
		t.Fatalf("method = %s, want book value cross-check", report.IntrinsicValue.Method)
	}
	if report.MarginOfSafety.Computed != true {
		t.Fatalf("margin should be computed with cross-check intrinsic value: %+v", report.MarginOfSafety)
	}
	if !hasValueCheck(report, "intrinsic_value", "pass") {
		t.Fatalf("missing intrinsic value pass after cross-check: %+v", report.Checks)
	}
}

func TestAnalyzeOperationalCompanyFallsBackWhenDCFIsImplausiblyBelowBook(t *testing.T) {
	equitiesDir := writeLowOwnerEarningsHighBookFixture(t, "LOWDCF")

	report := Analyze(Input{
		EquitiesDir:  equitiesDir,
		Symbol:       "LOWDCF",
		Sector:       "Sanayi",
		Industry:     "Savunma",
		Currency:     "TRY",
		CurrentPrice: 10,
		AsOf:         time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
	})

	if !report.Computed {
		t.Fatalf("expected computed report: %+v", report)
	}
	if report.IntrinsicValue.Method != "owner_earnings_dcf_book_value_crosscheck" {
		t.Fatalf("method = %s, want book value cross-check; intrinsic=%+v owner=%+v", report.IntrinsicValue.Method, report.IntrinsicValue, report.OwnerEarnings)
	}
	if !containsString(report.IntrinsicValue.Drivers, "owner_earnings_dcf_below_book_value_floor") {
		t.Fatalf("missing book floor driver: %+v", report.IntrinsicValue.Drivers)
	}
	if report.IntrinsicValue.Base < 25 {
		t.Fatalf("fallback intrinsic value too low: %+v", report.IntrinsicValue)
	}
}

func hasValueCheck(report Report, name, status string) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func hasBuffettRequirement(report Report, id, status string) bool {
	for _, item := range report.BuffettChecklist.Requirements {
		if item.ID == id && item.Status == status {
			return true
		}
	}
	return false
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func writeFinancialFixture(t *testing.T, symbol string, bank bool) string {
	t.Helper()
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	financialDir := filepath.Join(equitiesDir, symbol, "financials")
	if err := os.MkdirAll(financialDir, 0o755); err != nil {
		t.Fatalf("mkdir financial dir: %v", err)
	}
	data := map[string]map[string]any{}
	add := func(code string, values map[string]float64) {
		years := map[string][]float64{}
		for year, value := range values {
			years[year] = []float64{value}
		}
		data[code] = map[string]any{"desc_tr": code, "years": years}
	}

	revenue := map[string]float64{}
	grossProfit := map[string]float64{}
	operatingProfit := map[string]float64{}
	netIncome := map[string]float64{}
	operatingCash := map[string]float64{}
	capex := map[string]float64{}
	freeCashFlow := map[string]float64{}
	depreciation := map[string]float64{}
	dividends := map[string]float64{}
	rights := map[string]float64{}
	paidCapital := map[string]float64{}
	equity := map[string]float64{}
	assets := map[string]float64{}
	cash := map[string]float64{}
	debt := map[string]float64{}

	for i := 0; i < 10; i++ {
		year := 2016 + i
		key := strconv.Itoa(year)
		scale := math.Pow(1.08, float64(i))
		rev := 1_000_000_000 * scale
		if bank {
			rev = 700_000_000 * scale
		}
		revenue[key] = rev
		grossProfit[key] = rev * 0.36
		operatingProfit[key] = rev * 0.19
		netIncome[key] = rev * 0.13
		operatingCash[key] = rev * 0.17
		capex[key] = -rev * 0.04
		freeCashFlow[key] = operatingCash[key] + capex[key]
		depreciation[key] = rev * 0.03
		dividends[key] = -netIncome[key] * 0.20
		rights[key] = 0
		paidCapital[key] = 100_000_000
		equity[key] = 700_000_000 + rev*0.35
		assets[key] = equity[key] * 1.8
		cash[key] = rev * 0.08
		debt[key] = rev * 0.12
	}

	add("3C", revenue)
	add("3D", grossProfit)
	add("3DF", operatingProfit)
	add("3L", netIncome)
	add("4C", operatingCash)
	add("4CAI", capex)
	add("4CB", freeCashFlow)
	add("4CAB", depreciation)
	add("4CBB", dividends)
	add("4CBC", rights)
	add("2OA", paidCapital)
	add("2N", equity)
	add("1BL", assets)
	add("1AA", cash)
	add("2AA", debt)
	add("2BA", map[string]float64{})
	add("2BB", map[string]float64{})

	payload := map[string]any{
		"ticker":          symbol,
		"source":          "test_fixture",
		"currency":        "TRY",
		"financial_group": "",
		"data":            data,
	}
	if bank {
		payload["financial_group"] = "bank"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(financialDir, "bilanco.json"), raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return equitiesDir
}

func writeBankSchemaFinancialFixture(t *testing.T, symbol string) string {
	t.Helper()
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	financialDir := filepath.Join(equitiesDir, symbol, "financials")
	if err := os.MkdirAll(financialDir, 0o755); err != nil {
		t.Fatalf("mkdir financial dir: %v", err)
	}
	data := map[string]map[string]any{}
	add := func(code string, values map[string]float64) {
		years := map[string][]float64{}
		for year, value := range values {
			years[year] = []float64{value}
		}
		data[code] = map[string]any{"desc_tr": code, "years": years}
	}

	netInterest := map[string]float64{}
	operatingIncome := map[string]float64{}
	operatingProfit := map[string]float64{}
	netIncome := map[string]float64{}
	groupNetIncome := map[string]float64{}
	paidCapital := map[string]float64{}
	equity := map[string]float64{}
	assets := map[string]float64{}
	cash := map[string]float64{}
	for i := 0; i < 10; i++ {
		year := 2016 + i
		key := strconv.Itoa(year)
		scale := math.Pow(1.10, float64(i))
		operatingIncome[key] = 80_000_000_000 * scale
		netInterest[key] = operatingIncome[key] * 0.38
		operatingProfit[key] = operatingIncome[key] * 0.31
		netIncome[key] = operatingIncome[key] * 0.22
		groupNetIncome[key] = operatingIncome[key] * 0.18
		paidCapital[key] = 25_000_000_000
		equity[key] = 250_000_000_000 * scale
		assets[key] = equity[key] * 10.5
		cash[key] = assets[key] * 0.03
	}

	add("3C", netInterest)
	add("3CE", operatingIncome)
	add("3CH", operatingProfit)
	add("3Z", netIncome)
	add("3ZA", groupNetIncome)
	add("2OA", paidCapital)
	add("2O", equity)
	add("1Z", assets)
	add("1A", cash)

	payload := map[string]any{
		"ticker":          symbol,
		"source":          "test_fixture",
		"currency":        "TRY",
		"financial_group": "UFRS_K",
		"data":            data,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(financialDir, "bilanco.json"), raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return equitiesDir
}

func writeWeakCashFlowFixture(t *testing.T, symbol string) string {
	t.Helper()
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	financialDir := filepath.Join(equitiesDir, symbol, "financials")
	if err := os.MkdirAll(financialDir, 0o755); err != nil {
		t.Fatalf("mkdir financial dir: %v", err)
	}
	data := map[string]map[string]any{}
	add := func(code string, values map[string]float64) {
		years := map[string][]float64{}
		for year, value := range values {
			years[year] = []float64{value}
		}
		data[code] = map[string]any{"desc_tr": code, "years": years}
	}

	revenue := map[string]float64{}
	grossProfit := map[string]float64{}
	operatingProfit := map[string]float64{}
	netIncome := map[string]float64{}
	operatingCash := map[string]float64{}
	capex := map[string]float64{}
	freeCashFlow := map[string]float64{}
	depreciation := map[string]float64{}
	dividends := map[string]float64{}
	rights := map[string]float64{}
	paidCapital := map[string]float64{}
	equity := map[string]float64{}
	assets := map[string]float64{}
	cash := map[string]float64{}
	debt := map[string]float64{}

	for i := 0; i < 6; i++ {
		year := 2020 + i
		key := strconv.Itoa(year)
		rev := 1_000_000_000 * math.Pow(1.05, float64(i))
		revenue[key] = rev
		grossProfit[key] = rev * 0.22
		operatingProfit[key] = rev * 0.07
		netIncome[key] = rev * 0.03
		if i == 5 {
			netIncome[key] = -rev * 0.02
		}
		operatingCash[key] = -rev * 0.03
		capex[key] = -rev * 0.10
		freeCashFlow[key] = operatingCash[key] + capex[key]
		depreciation[key] = rev * 0.02
		dividends[key] = 0
		rights[key] = 0
		paidCapital[key] = 100_000_000
		equity[key] = 1_300_000_000 + rev*0.25
		assets[key] = equity[key] * 1.45
		cash[key] = rev * 0.04
		debt[key] = rev * 0.06
	}

	add("3C", revenue)
	add("3D", grossProfit)
	add("3DF", operatingProfit)
	add("3L", netIncome)
	add("4C", operatingCash)
	add("4CAI", capex)
	add("4CB", freeCashFlow)
	add("4CAB", depreciation)
	add("4CBB", dividends)
	add("4CBC", rights)
	add("2OA", paidCapital)
	add("2N", equity)
	add("1BL", assets)
	add("1AA", cash)
	add("2AA", debt)
	add("2BA", map[string]float64{})
	add("2BB", map[string]float64{})

	payload := map[string]any{
		"ticker":   symbol,
		"source":   "test_fixture",
		"currency": "TRY",
		"data":     data,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(financialDir, "bilanco.json"), raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return equitiesDir
}

func writeLowOwnerEarningsHighBookFixture(t *testing.T, symbol string) string {
	t.Helper()
	root := t.TempDir()
	equitiesDir := filepath.Join(root, "equities")
	financialDir := filepath.Join(equitiesDir, symbol, "financials")
	if err := os.MkdirAll(financialDir, 0o755); err != nil {
		t.Fatalf("mkdir financial dir: %v", err)
	}
	data := map[string]map[string]any{}
	add := func(code string, values map[string]float64) {
		years := map[string][]float64{}
		for year, value := range values {
			years[year] = []float64{value}
		}
		data[code] = map[string]any{"desc_tr": code, "years": years}
	}
	revenue := map[string]float64{}
	grossProfit := map[string]float64{}
	operatingProfit := map[string]float64{}
	netIncome := map[string]float64{}
	operatingCash := map[string]float64{}
	capex := map[string]float64{}
	freeCashFlow := map[string]float64{}
	depreciation := map[string]float64{}
	paidCapital := map[string]float64{}
	equity := map[string]float64{}
	assets := map[string]float64{}
	cash := map[string]float64{}
	debt := map[string]float64{}
	for i := 0; i < 10; i++ {
		year := strconv.Itoa(2016 + i)
		revenue[year] = 1_000_000_000
		grossProfit[year] = 300_000_000
		operatingProfit[year] = 80_000_000
		netIncome[year] = 60_000_000
		operatingCash[year] = 45_000_000
		capex[year] = -10_000_000
		freeCashFlow[year] = 35_000_000
		depreciation[year] = 10_000_000
		paidCapital[year] = 100_000_000
		equity[year] = 10_000_000_000
		assets[year] = 12_000_000_000
		cash[year] = 100_000_000
		debt[year] = 150_000_000
	}
	add("3C", revenue)
	add("3D", grossProfit)
	add("3DF", operatingProfit)
	add("3L", netIncome)
	add("4C", operatingCash)
	add("4CAI", capex)
	add("4CB", freeCashFlow)
	add("4CAB", depreciation)
	add("4CBB", map[string]float64{})
	add("4CBC", map[string]float64{})
	add("2OA", paidCapital)
	add("2N", equity)
	add("1BL", assets)
	add("1AA", cash)
	add("2AA", debt)
	add("2BA", map[string]float64{})
	add("2BB", map[string]float64{})
	payload := map[string]any{"ticker": symbol, "source": "test_fixture", "currency": "TRY", "data": data}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(financialDir, "bilanco.json"), raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return equitiesDir
}
