package value

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/ta/docintel"
	"hissebot/internal/ta/macro"
	"hissebot/pkg/mathutil"
)

type Input struct {
	EquitiesDir        string
	Symbol             string
	Sector             string
	Industry           string
	FinancialGroup     string
	Currency           string
	CurrentPrice       float64
	PriceHistory       []PriceObservation
	AsOf               time.Time
	AssumptionsFile    string
	MacroGDPFile       string
	SkipLegacyDocIntel bool
}

type PriceObservation struct {
	Time  time.Time
	Close float64
}

type Report struct {
	Computed          bool                    `json:"computed"`
	Symbol            string                  `json:"symbol,omitempty"`
	Currency          string                  `json:"currency,omitempty"`
	AsOf              time.Time               `json:"as_of,omitempty"`
	CurrentPrice      float64                 `json:"current_price"`
	Decision          string                  `json:"decision"`
	DecisionLabel     string                  `json:"decision_label"`
	Summary           string                  `json:"summary"`
	SectorModel       SectorModelReport       `json:"sector_model"`
	IntrinsicValue    IntrinsicValueReport    `json:"intrinsic_value"`
	FairValue         FairValueConclusion     `json:"fair_value_conclusion"`
	MarginOfSafety    MarginOfSafetyReport    `json:"margin_of_safety"`
	OwnerEarnings     OwnerEarningsReport     `json:"owner_earnings"`
	NormalizedFCF     NormalizedFCFReport     `json:"normalized_fcf"`
	CapitalAllocation CapitalAllocationReport `json:"capital_allocation"`
	RetainedEarnings  RetainedEarningsReport  `json:"retained_earnings_test"`
	Moat              MoatReport              `json:"moat"`
	BuffettChecklist  BuffettChecklistReport  `json:"buffett_checklist"`
	DocumentEvidence  docintel.Report         `json:"document_evidence"`
	MacroGDP          macro.GDPContext        `json:"macro_gdp"`
	QualityScore      float64                 `json:"quality_score"`
	ValueScore        float64                 `json:"value_score"`
	DataQuality       float64                 `json:"data_quality"`
	Confidence        float64                 `json:"confidence"`
	Assumptions       Assumptions             `json:"assumptions"`
	Years             []YearMetric            `json:"years,omitempty"`
	Checks            []Check                 `json:"checks,omitempty"`
	Warnings          []string                `json:"warnings,omitempty"`
	Missing           []string                `json:"missing,omitempty"`
}

func (r Report) MarshalJSON() ([]byte, error) {
	if !r.Computed && strings.EqualFold(r.Decision, "NOT_APPLICABLE") {
		type notApplicableReport struct {
			Computed       bool                 `json:"computed"`
			Symbol         string               `json:"symbol,omitempty"`
			Currency       string               `json:"currency,omitempty"`
			AsOf           time.Time            `json:"as_of,omitempty"`
			CurrentPrice   float64              `json:"current_price"`
			Decision       string               `json:"decision"`
			DecisionLabel  string               `json:"decision_label"`
			Summary        string               `json:"summary"`
			IntrinsicValue IntrinsicValueReport `json:"intrinsic_value"`
			FairValue      FairValueConclusion  `json:"fair_value_conclusion"`
			MarginOfSafety MarginOfSafetyReport `json:"margin_of_safety"`
			Checks         []Check              `json:"checks,omitempty"`
			Warnings       []string             `json:"warnings,omitempty"`
		}
		return json.Marshal(notApplicableReport{
			Computed:       r.Computed,
			Symbol:         r.Symbol,
			Currency:       r.Currency,
			AsOf:           r.AsOf,
			CurrentPrice:   r.CurrentPrice,
			Decision:       r.Decision,
			DecisionLabel:  r.DecisionLabel,
			Summary:        r.Summary,
			IntrinsicValue: r.IntrinsicValue,
			FairValue:      r.FairValue,
			MarginOfSafety: r.MarginOfSafety,
			Checks:         r.Checks,
			Warnings:       r.Warnings,
		})
	}
	type reportAlias Report
	return json.Marshal(reportAlias(r))
}

type SectorModelReport struct {
	Model     string   `json:"model"`
	Label     string   `json:"label"`
	Reason    string   `json:"reason"`
	Primary   []string `json:"primary_metrics,omitempty"`
	Secondary []string `json:"secondary_metrics,omitempty"`
}

type IntrinsicValueReport struct {
	Computed   bool     `json:"computed"`
	Bear       float64  `json:"bear"`
	Base       float64  `json:"base"`
	Bull       float64  `json:"bull"`
	Method     string   `json:"method"`
	Drivers    []string `json:"drivers,omitempty"`
	Confidence float64  `json:"confidence"`
}

type MarginOfSafetyReport struct {
	Computed    bool    `json:"computed"`
	BearPct     float64 `json:"bear_pct"`
	BasePct     float64 `json:"base_pct"`
	BullPct     float64 `json:"bull_pct"`
	RequiredPct float64 `json:"required_pct"`
	Label       string  `json:"label"`
}

type FairValueConclusion struct {
	Computed            bool     `json:"computed"`
	Status              string   `json:"status"`
	Label               string   `json:"label"`
	CurrentPrice        float64  `json:"current_price"`
	FairValueBase       float64  `json:"fair_value_base"`
	FairValueBear       float64  `json:"fair_value_bear"`
	FairValueBull       float64  `json:"fair_value_bull"`
	PriceToFairValuePct float64  `json:"price_to_fair_value_pct"`
	UpsideDownsidePct   float64  `json:"upside_downside_pct"`
	MarginOfSafetyPct   float64  `json:"margin_of_safety_pct"`
	RequiredMarginPct   float64  `json:"required_margin_pct"`
	Explanation         string   `json:"explanation"`
	DataInputs          []string `json:"data_inputs,omitempty"`
}

type OwnerEarningsReport struct {
	Applicable          bool         `json:"applicable"`
	TTM                 float64      `json:"ttm"`
	Normalized5Y        float64      `json:"normalized_5y"`
	Normalized10Y       float64      `json:"normalized_10y"`
	OperatingCashTTM    float64      `json:"operating_cash_ttm"`
	CapexTTM            float64      `json:"capex_ttm"`
	MaintenanceCapexTTM float64      `json:"maintenance_capex_ttm"`
	Method              string       `json:"method"`
	PositiveYears       int          `json:"positive_years"`
	TotalYears          int          `json:"total_years"`
	Score               float64      `json:"score"`
	Years               []YearMetric `json:"years,omitempty"`
	Warnings            []string     `json:"warnings,omitempty"`
}

type NormalizedFCFReport struct {
	Applicable        bool     `json:"applicable"`
	TTM               float64  `json:"ttm"`
	Median5Y          float64  `json:"median_5y"`
	Median10Y         float64  `json:"median_10y"`
	Average5Y         float64  `json:"average_5y"`
	Average10Y        float64  `json:"average_10y"`
	PositiveYearRatio float64  `json:"positive_year_ratio"`
	Stability         float64  `json:"stability"`
	TrendCAGR5Y       float64  `json:"trend_cagr_5y"`
	Score             float64  `json:"score"`
	Warnings          []string `json:"warnings,omitempty"`
}

type CapitalAllocationReport struct {
	PaidCapitalLatest                     float64  `json:"paid_capital_latest"`
	PaidCapital5Y                         float64  `json:"paid_capital_5y"`
	PaidCapital10Y                        float64  `json:"paid_capital_10y"`
	Dilution5YPct                         float64  `json:"dilution_5y_pct"`
	Dilution10YPct                        float64  `json:"dilution_10y_pct"`
	RightsIssues10Y                       float64  `json:"rights_issues_10y"`
	Dividends10Y                          float64  `json:"dividends_10y"`
	DividendDataAvailable                 bool     `json:"dividend_data_available"`
	DividendYears                         int      `json:"dividend_years"`
	DividendContinuity5Y                  float64  `json:"dividend_continuity_5y"`
	DividendContinuity10Y                 float64  `json:"dividend_continuity_10y"`
	DividendPayoutRatio10Y                float64  `json:"dividend_payout_ratio_10y"`
	CapitalMovementClassificationRequired bool     `json:"capital_movement_classification_required,omitempty"`
	NetDebtToEquity                       float64  `json:"net_debt_to_equity"`
	NetDebtChange5Y                       float64  `json:"net_debt_change_5y"`
	Score                                 float64  `json:"score"`
	Warnings                              []string `json:"warnings,omitempty"`
}

type RetainedEarningsReport struct {
	Computed           bool     `json:"computed"`
	Ratio              float64  `json:"ratio"`
	MarketCapLatest    float64  `json:"market_cap_latest"`
	MarketCap5Y        float64  `json:"market_cap_5y"`
	MarketCapChange5Y  float64  `json:"market_cap_change_5y"`
	RetainedEarnings5Y float64  `json:"retained_earnings_5y"`
	PriceLatest        float64  `json:"price_latest"`
	Price5Y            float64  `json:"price_5y"`
	Years              int      `json:"years"`
	Method             string   `json:"method"`
	Warnings           []string `json:"warnings,omitempty"`
}

type MoatReport struct {
	Label                   string   `json:"label"`
	AverageROE5Y            float64  `json:"average_roe_5y"`
	AverageROE10Y           float64  `json:"average_roe_10y"`
	AverageROIC5Y           float64  `json:"average_roic_5y"`
	GrossMarginMedian5Y     float64  `json:"gross_margin_median_5y"`
	OperatingMarginMedian5Y float64  `json:"operating_margin_median_5y"`
	NetMarginMedian5Y       float64  `json:"net_margin_median_5y"`
	MarginStability         float64  `json:"margin_stability"`
	RevenueCAGR5Y           float64  `json:"revenue_cagr_5y"`
	Score                   float64  `json:"score"`
	Warnings                []string `json:"warnings,omitempty"`
}

type Assumptions struct {
	Source              string  `json:"source"`
	DiscountRate        float64 `json:"discount_rate"`
	TerminalGrowth      float64 `json:"terminal_growth"`
	OwnerEarningsGrowth float64 `json:"owner_earnings_growth"`
	TaxRate             float64 `json:"tax_rate"`
	RequiredMarginPct   float64 `json:"required_margin_pct"`
}

type YearMetric struct {
	Year             int     `json:"year"`
	Revenue          float64 `json:"revenue"`
	GrossProfit      float64 `json:"gross_profit"`
	OperatingProfit  float64 `json:"operating_profit"`
	NetIncome        float64 `json:"net_income"`
	OperatingCash    float64 `json:"operating_cash"`
	Capex            float64 `json:"capex"`
	FreeCashFlow     float64 `json:"free_cash_flow"`
	MaintenanceCapex float64 `json:"maintenance_capex"`
	OwnerEarnings    float64 `json:"owner_earnings"`
	DividendsPaid    float64 `json:"dividends_paid"`
	RightsIssue      float64 `json:"rights_issue"`
	PaidCapital      float64 `json:"paid_capital"`
	Equity           float64 `json:"equity"`
	TotalAssets      float64 `json:"total_assets"`
	Cash             float64 `json:"cash"`
	Debt             float64 `json:"debt"`
	NetDebt          float64 `json:"net_debt"`
	ROE              float64 `json:"roe"`
	ROIC             float64 `json:"roic"`
	GrossMargin      float64 `json:"gross_margin"`
	OperatingMargin  float64 `json:"operating_margin"`
	NetMargin        float64 `json:"net_margin"`
}

type Check struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Score   float64 `json:"score,omitempty"`
	Message string  `json:"message"`
}

type BuffettChecklistReport struct {
	Computed           bool                          `json:"computed"`
	Status             string                        `json:"status"`
	StatusLabel        string                        `json:"status_label"`
	Score              float64                       `json:"score"`
	CoveragePct        float64                       `json:"coverage_pct"`
	BuyEligible        bool                          `json:"buy_eligible"`
	Summary            string                        `json:"summary"`
	Requirements       []BuffettChecklistRequirement `json:"requirements,omitempty"`
	BlockingIssues     []string                      `json:"blocking_issues,omitempty"`
	MissingData        []string                      `json:"missing_data,omitempty"`
	MethodologyVersion string                        `json:"methodology_version"`
}

type BuffettChecklistRequirement struct {
	ID        string   `json:"id"`
	Pillar    string   `json:"pillar"`
	Label     string   `json:"label"`
	Status    string   `json:"status"`
	Required  bool     `json:"required"`
	Value     string   `json:"value,omitempty"`
	Threshold string   `json:"threshold,omitempty"`
	Evidence  string   `json:"evidence,omitempty"`
	Missing   []string `json:"missing,omitempty"`
}

type financialFile struct {
	Ticker         string                    `json:"ticker"`
	Source         string                    `json:"source,omitempty"`
	Currency       string                    `json:"currency,omitempty"`
	FinancialGroup string                    `json:"financial_group,omitempty"`
	FetchedAt      time.Time                 `json:"fetched_at,omitempty"`
	Data           map[string]financialField `json:"data"`
}

type financialField struct {
	DescTR string                `json:"desc_tr"`
	DescEN string                `json:"desc_en"`
	Years  map[string][]*float64 `json:"years"`
}

type period struct {
	Year    int
	Quarter int
}

func Analyze(input Input) Report {
	report := Report{
		Symbol:       strings.ToUpper(strings.TrimSpace(input.Symbol)),
		Currency:     firstNonEmpty(input.Currency, "TRY"),
		AsOf:         input.AsOf,
		CurrentPrice: input.CurrentPrice,
		Assumptions:  loadAssumptions(input.AssumptionsFile, input.Sector),
	}
	if report.AsOf.IsZero() {
		report.AsOf = time.Now().UTC()
	}
	if input.SkipLegacyDocIntel {
		report.DocumentEvidence = docintel.Report{
			Symbol:  report.Symbol,
			Summary: "KAP PDF belge kanıtı kap-ingest JSONL katmanından okunduğu için legacy PDF taraması atlandı.",
			Warnings: []string{
				"legacy_docintel_skipped_kap_pdf_ingest_available",
			},
		}
	} else {
		report.DocumentEvidence = docintel.Analyze(docintel.Input{
			EquitiesDir: input.EquitiesDir,
			Symbol:      report.Symbol,
			AsOf:        report.AsOf,
			Limit:       40,
		})
	}
	report.MacroGDP = loadMacroGDP(input.EquitiesDir, input.MacroGDPFile)
	fin, ok := loadFinancialsForSymbol(input.EquitiesDir, report.Symbol)
	if !ok {
		report.Decision = "HESAPLANAMADI"
		report.DecisionLabel = "Finansal veri yok"
		report.Missing = append(report.Missing, "financial_statements")
		report.Summary = "İçsel değer hesaplanamadı; bilanço JSON dosyası okunamadı."
		report.BuffettChecklist = buildBuffettChecklist(report)
		return report
	}
	if report.Currency == "" {
		report.Currency = firstNonEmpty(fin.Currency, "TRY")
	}
	latest := latestPeriod(fin)
	if latest.Year == 0 {
		report.Decision = "HESAPLANAMADI"
		report.DecisionLabel = "Dönem yok"
		report.Missing = append(report.Missing, "financial_periods")
		report.Summary = "İçsel değer hesaplanamadı; finansal dönem bulunamadı."
		report.BuffettChecklist = buildBuffettChecklist(report)
		return report
	}
	model := chooseSectorModel(input.Sector, input.Industry, fin.FinancialGroup)
	years := buildYearMetrics(fin, latest, report.Assumptions)
	report.Years = years
	report.SectorModel = model
	report.OwnerEarnings = analyzeOwnerEarnings(fin, latest, years, model)
	report.NormalizedFCF = analyzeNormalizedFCF(fin, latest, years, model)
	report.CapitalAllocation = analyzeCapitalAllocation(fin, latest, years, model)
	report.RetainedEarnings = analyzeRetainedEarningsTest(input, years)
	report.Moat = analyzeMoat(years)
	report.DataQuality = dataQuality(fin, years, model)
	report.QualityScore = qualityScore(report, model)
	report.IntrinsicValue = estimateIntrinsicValue(fin, latest, years, model, report)
	report.MarginOfSafety = marginOfSafety(report.CurrentPrice, report.IntrinsicValue, report.Assumptions.RequiredMarginPct)
	report.FairValue = fairValueConclusion(report)
	report.Confidence = confidenceScore(report)
	report.ValueScore = valueScore(report)
	report.BuffettChecklist = buildBuffettChecklist(report)
	report.Checks = valueChecks(report)
	report.Decision, report.DecisionLabel = decision(report)
	report.Computed = report.IntrinsicValue.Computed
	report.Summary = summary(report)
	report.Warnings = append(report.Warnings, report.OwnerEarnings.Warnings...)
	report.Warnings = append(report.Warnings, report.NormalizedFCF.Warnings...)
	report.Warnings = append(report.Warnings, report.CapitalAllocation.Warnings...)
	report.Warnings = append(report.Warnings, report.Moat.Warnings...)
	report.Warnings = append(report.Warnings, report.DocumentEvidence.Warnings...)
	return report
}

func loadMacroGDP(equitiesDir, path string) macro.GDPContext {
	path = strings.TrimSpace(path)
	if path == "" {
		path = macro.DefaultGDPPathFromEquitiesDir(equitiesDir)
	}
	dataset, ok, err := macro.LoadGDPDataset(path)
	if err != nil {
		return macro.GDPContext{
			Computed:           false,
			Source:             "TÜİK CİP",
			SourceURL:          "https://cip.tuik.gov.tr/",
			DataQualityWarning: err.Error(),
			RequiredCaveats:    []string{"GSYH makro verisi okunamadığı için değerleme bağlamına eklenemedi."},
		}
	}
	if !ok {
		return macro.GDPContext{
			Computed:           false,
			Source:             "TÜİK CİP",
			SourceURL:          "https://cip.tuik.gov.tr/",
			DataQualityWarning: "TÜİK CİP GSYH veri dosyası bulunamadı: " + path,
			RequiredCaveats:    []string{"Önce `go run ./cmd/hissebot sync tuik-gdp` komutu ile GSYH verisi çekilmelidir."},
		}
	}
	return macro.AnalyzeGDP(dataset)
}

func loadFinancials(path string) (financialFile, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return financialFile{}, false
	}
	var fin financialFile
	if json.Unmarshal(raw, &fin) != nil || len(fin.Data) == 0 {
		return financialFile{}, false
	}
	return fin, true
}

func loadFinancialsForSymbol(equitiesDir, symbol string) (financialFile, bool) {
	for _, path := range financialCandidatePaths(equitiesDir, symbol) {
		if fin, ok := loadFinancials(path); ok {
			return fin, true
		}
	}
	return financialFile{}, false
}

func financialCandidatePaths(equitiesDir, symbol string) []string {
	symbolUpper := strings.ToUpper(strings.TrimSpace(symbol))
	symbolLower := strings.ToLower(symbolUpper)
	if symbolUpper == "" {
		return nil
	}
	paths := []string{
		filepath.Join(equitiesDir, symbolUpper, "financials", "bilanco.json"),
		filepath.Join(equitiesDir, symbolUpper, "financials", "kap_bilanco.json"),
	}
	dataDir := filepath.Dir(filepath.Clean(equitiesDir))
	paths = append(paths,
		filepath.Join(dataDir, "processed", "by_ticker", symbolUpper, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", "by_ticker", symbolLower, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolUpper, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolLower, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", symbolLower, "kap_financials", "bilanco.json"),
		filepath.Join("data", "processed", "by_ticker", symbolUpper, "kap_financials", "bilanco.json"),
		filepath.Join("data", "processed", "by_ticker", symbolLower, "kap_financials", "bilanco.json"),
	)
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func latestPeriod(fin financialFile) period {
	latest := period{}
	for _, field := range fin.Data {
		for yearText, values := range field.Years {
			year, err := strconv.Atoi(yearText)
			if err != nil {
				continue
			}
			for i, value := range values {
				if value == nil {
					continue
				}
				q := quarterFromIndex(i)
				if year > latest.Year || (year == latest.Year && q > latest.Quarter) {
					latest = period{Year: year, Quarter: q}
				}
			}
		}
	}
	return latest
}

func buildYearMetrics(fin financialFile, latest period, assumptions Assumptions) []YearMetric {
	years := completedYears(fin, latest, 10)
	out := make([]YearMetric, 0, len(years))
	bankSchema := isBankFinancialSchema(fin)
	for _, year := range years {
		p := period{Year: year, Quarter: 4}
		cfo := fieldValue(fin, "4C", p)
		capex := fieldValue(fin, "4CAI", p)
		depreciation := math.Max(fieldValue(fin, "4B", p), fieldValue(fin, "4CAB", p))
		revenue := firstFieldValue(fin, p, "3C")
		gross := fieldValue(fin, "3D", p)
		operating := fieldValue(fin, "3DF", p)
		netIncome := firstFieldValue(fin, p, "3L", "3Z", "3ZA", "3CN")
		equity := firstFieldValue(fin, p, "2N", "2O")
		totalAssets := firstFieldValue(fin, p, "1BL", "1Z")
		cash := fieldValue(fin, "1AA", p)
		debt := fieldValue(fin, "2AA", p) + fieldValue(fin, "2BA", p) + fieldValue(fin, "2BB", p)
		netDebt := debt - cash
		freeCashFlow := fieldValue(fin, "4CB", p)
		dividendsPaid := math.Abs(fieldValue(fin, "4CBB", p))
		rightsIssue := math.Max(fieldValue(fin, "4CBC", p), 0)
		if bankSchema {
			cfo = 0
			capex = 0
			depreciation = 0
			revenue = firstFieldValue(fin, p, "3CE", "3C")
			gross = fieldValue(fin, "3C", p)
			operating = firstFieldValue(fin, p, "3CH", "3CL", "3CE")
			netIncome = firstFieldValue(fin, p, "3ZA", "3Z", "3CN")
			equity = fieldValue(fin, "2O", p)
			totalAssets = fieldValue(fin, "1Z", p)
			cash = fieldValue(fin, "1A", p)
			debt = 0
			netDebt = 0
			freeCashFlow = 0
			dividendsPaid = 0
			rightsIssue = 0
		}
		maintenance := estimateMaintenanceCapex(cfo, capex, depreciation)
		investedCapital := equity + math.Max(netDebt, 0)
		nopat := operating * (1 - assumptions.TaxRate)
		out = append(out, YearMetric{
			Year:             year,
			Revenue:          revenue,
			GrossProfit:      gross,
			OperatingProfit:  operating,
			NetIncome:        netIncome,
			OperatingCash:    cfo,
			Capex:            capex,
			FreeCashFlow:     freeCashFlow,
			MaintenanceCapex: maintenance,
			OwnerEarnings:    cfo - maintenance,
			DividendsPaid:    dividendsPaid,
			RightsIssue:      rightsIssue,
			PaidCapital:      fieldValue(fin, "2OA", p),
			Equity:           equity,
			TotalAssets:      totalAssets,
			Cash:             cash,
			Debt:             debt,
			NetDebt:          netDebt,
			ROE:              mathutil.SafeDiv(netIncome, equity),
			ROIC:             mathutil.SafeDiv(nopat, investedCapital),
			GrossMargin:      mathutil.SafeDiv(gross, revenue),
			OperatingMargin:  mathutil.SafeDiv(operating, revenue),
			NetMargin:        mathutil.SafeDiv(netIncome, revenue),
		})
	}
	return out
}

func analyzeOwnerEarnings(fin financialFile, latest period, years []YearMetric, model SectorModelReport) OwnerEarningsReport {
	out := OwnerEarningsReport{Applicable: operatingCashFlowModel(model.Model), Method: "operating_cash_minus_estimated_maintenance_capex"}
	cfo, cfoOK := ttm(fin, "4C", latest)
	capex, capexOK := ttm(fin, "4CAI", latest)
	dep, _ := ttm(fin, "4B", latest)
	depAlt, _ := ttm(fin, "4CAB", latest)
	out.OperatingCashTTM = cfo
	out.CapexTTM = capex
	out.MaintenanceCapexTTM = estimateMaintenanceCapex(cfo, capex, math.Max(dep, depAlt))
	if cfoOK && capexOK {
		out.TTM = cfo - out.MaintenanceCapexTTM
	} else {
		out.Warnings = append(out.Warnings, "owner_earnings_ttm_inputs_missing")
	}
	ownerValues := mapYearValues(years, func(y YearMetric) float64 { return y.OwnerEarnings })
	out.Normalized5Y = median(lastN(ownerValues, 5))
	out.Normalized10Y = median(lastN(ownerValues, 10))
	out.Years = years
	for _, value := range ownerValues {
		if value > 0 {
			out.PositiveYears++
		}
	}
	out.TotalYears = len(ownerValues)
	positiveRatio := mathutil.SafeDiv(float64(out.PositiveYears), float64(out.TotalYears))
	stability := stabilityScore(ownerValues)
	out.Score = mathutil.Clamp(positiveRatio*55+stability*30+scorePositive(out.Normalized5Y)*15, 0, 100)
	if !out.Applicable {
		out.Warnings = append(out.Warnings, "owner_earnings_not_primary_for_sector_model")
	}
	return out
}

func analyzeNormalizedFCF(fin financialFile, latest period, years []YearMetric, model SectorModelReport) NormalizedFCFReport {
	out := NormalizedFCFReport{Applicable: operatingCashFlowModel(model.Model) || model.Model == "holding_sum_of_parts_proxy"}
	fcf, ok := ttm(fin, "4CB", latest)
	if ok {
		out.TTM = fcf
	} else {
		out.Warnings = append(out.Warnings, "free_cash_flow_ttm_inputs_missing")
	}
	values := mapYearValues(years, func(y YearMetric) float64 { return y.FreeCashFlow })
	out.Median5Y = median(lastN(values, 5))
	out.Median10Y = median(lastN(values, 10))
	out.Average5Y = mean(lastN(values, 5))
	out.Average10Y = mean(lastN(values, 10))
	positive := 0
	for _, value := range values {
		if value > 0 {
			positive++
		}
	}
	out.PositiveYearRatio = mathutil.SafeDiv(float64(positive), float64(len(values)))
	out.Stability = stabilityScore(values)
	out.TrendCAGR5Y = cagrFromValues(lastN(values, 5))
	out.Score = mathutil.Clamp(out.PositiveYearRatio*45+out.Stability*30+scorePositive(out.Median5Y)*15+scoreCAGR(out.TrendCAGR5Y)*10, 0, 100)
	if !out.Applicable {
		out.Warnings = append(out.Warnings, "fcf_not_primary_for_sector_model")
	}
	return out
}

func analyzeCapitalAllocation(fin financialFile, latest period, years []YearMetric, model SectorModelReport) CapitalAllocationReport {
	out := CapitalAllocationReport{}
	if len(years) == 0 {
		out.Warnings = append(out.Warnings, "capital_allocation_history_missing")
		return out
	}
	latestYear := years[len(years)-1]
	out.PaidCapitalLatest = fieldValue(fin, "2OA", latest)
	if out.PaidCapitalLatest == 0 {
		out.PaidCapitalLatest = latestYear.PaidCapital
	}
	out.PaidCapital5Y = firstPositivePaidCapital(lastN(years, 6))
	out.PaidCapital10Y = firstPositivePaidCapital(years)
	out.Dilution5YPct = pctChange(out.PaidCapital5Y, out.PaidCapitalLatest)
	out.Dilution10YPct = pctChange(out.PaidCapital10Y, out.PaidCapitalLatest)
	for _, y := range years {
		out.RightsIssues10Y += y.RightsIssue
		out.Dividends10Y += y.DividendsPaid
		if y.DividendsPaid > 0 {
			out.DividendYears++
		}
	}
	out.DividendDataAvailable = hasFinancialFields(fin, "4CBB") && !financialCapitalModel(model)
	out.DividendContinuity5Y = dividendContinuity(lastN(years, 5))
	out.DividendContinuity10Y = dividendContinuity(lastN(years, 10))
	totalNetIncome := sumYearValues(years, func(y YearMetric) float64 { return math.Max(y.NetIncome, 0) })
	out.DividendPayoutRatio10Y = mathutil.SafeDiv(out.Dividends10Y, totalNetIncome)
	out.NetDebtToEquity = mathutil.SafeDiv(latestYear.NetDebt, latestYear.Equity)
	if len(years) >= 6 {
		out.NetDebtChange5Y = latestYear.NetDebt - years[len(years)-6].NetDebt
	}
	score := 100.0
	if financialCapitalModel(model) {
		out.NetDebtToEquity = 0
		if out.Dilution5YPct > 10 {
			out.Warnings = append(out.Warnings, "paid_capital_growth_requires_bonus_split_rights_issue_classification")
			out.CapitalMovementClassificationRequired = true
			score = math.Min(score, 80)
		}
	} else if out.Dilution5YPct > 10 {
		score -= mathutil.Clamp((out.Dilution5YPct-10)*1.5, 0, 30)
		out.Warnings = append(out.Warnings, "paid_capital_dilution_above_10pct_5y")
		out.CapitalMovementClassificationRequired = true
	}
	if !out.DividendDataAvailable {
		out.Warnings = append(out.Warnings, "dividend_structured_data_missing_or_not_applicable")
	}
	if !financialCapitalModel(model) && out.RightsIssues10Y > 0 {
		score -= 10
	}
	if !financialCapitalModel(model) && out.NetDebtToEquity > 1 {
		score -= 25
		out.Warnings = append(out.Warnings, "net_debt_to_equity_above_1")
	} else if !financialCapitalModel(model) && out.NetDebtToEquity > 0.5 {
		score -= 12
	}
	if out.DividendPayoutRatio10Y > 0 && out.DividendPayoutRatio10Y <= 0.75 {
		score += 5
	}
	out.Score = mathutil.Clamp(score, 0, 100)
	return out
}

func analyzeRetainedEarningsTest(input Input, years []YearMetric) RetainedEarningsReport {
	out := RetainedEarningsReport{Method: "market_cap_change_5y_divided_by_retained_earnings_proxy"}
	if len(years) < 6 {
		out.Warnings = append(out.Warnings, "retained_earnings_test_requires_6y_financial_history")
		return out
	}
	latestYear := years[len(years)-1]
	baseYear := years[len(years)-6]
	if latestYear.PaidCapital <= 0 || baseYear.PaidCapital <= 0 {
		out.Warnings = append(out.Warnings, "retained_earnings_test_paid_capital_missing")
		return out
	}
	priceLatest := input.CurrentPrice
	if priceLatest <= 0 {
		priceLatest = latestPriceAtOrBefore(input.PriceHistory, input.AsOf)
	}
	targetDate := input.AsOf.AddDate(-5, 0, 0)
	price5Y := latestPriceAtOrBefore(input.PriceHistory, targetDate)
	if priceLatest <= 0 || price5Y <= 0 {
		out.Warnings = append(out.Warnings, "retained_earnings_test_market_price_history_missing")
		return out
	}
	retained := 0.0
	dividendMissing := true
	for _, y := range lastN(years, 5) {
		retainedYear := y.NetIncome + y.DividendsPaid
		if y.DividendsPaid != 0 {
			dividendMissing = false
		}
		retained += math.Max(retainedYear, 0)
	}
	if retained <= 0 {
		out.Warnings = append(out.Warnings, "retained_earnings_test_retained_earnings_non_positive")
		return out
	}
	out.PriceLatest = priceLatest
	out.Price5Y = price5Y
	out.MarketCapLatest = priceLatest * latestYear.PaidCapital
	out.MarketCap5Y = price5Y * baseYear.PaidCapital
	out.MarketCapChange5Y = out.MarketCapLatest - out.MarketCap5Y
	out.RetainedEarnings5Y = retained
	out.Ratio = mathutil.SafeDiv(out.MarketCapChange5Y, retained)
	out.Years = 5
	out.Computed = true
	if dividendMissing {
		out.Warnings = append(out.Warnings, "retained_earnings_test_dividend_history_missing_proxy_uses_net_income")
	}
	return out
}

func latestPriceAtOrBefore(history []PriceObservation, at time.Time) float64 {
	if at.IsZero() || len(history) == 0 {
		return 0
	}
	bestTime := time.Time{}
	best := 0.0
	for _, item := range history {
		if item.Close <= 0 || item.Time.IsZero() || item.Time.After(at) {
			continue
		}
		if bestTime.IsZero() || item.Time.After(bestTime) {
			bestTime = item.Time
			best = item.Close
		}
	}
	return best
}

func dividendContinuity(years []YearMetric) float64 {
	if len(years) == 0 {
		return 0
	}
	positive := 0
	for _, y := range years {
		if y.DividendsPaid > 0 {
			positive++
		}
	}
	return mathutil.SafeDiv(float64(positive), float64(len(years)))
}

func financialCapitalModel(model SectorModelReport) bool {
	switch model.Model {
	case "bank_residual_income", "insurance_book_value", "financial_book_value":
		return true
	default:
		return false
	}
}

func analyzeMoat(years []YearMetric) MoatReport {
	out := MoatReport{}
	if len(years) == 0 {
		out.Label = "veri_yok"
		out.Warnings = append(out.Warnings, "moat_history_missing")
		return out
	}
	last5 := lastN(years, 5)
	last10 := lastN(years, 10)
	out.AverageROE5Y = mean(mapYearValues(last5, func(y YearMetric) float64 { return y.ROE }))
	out.AverageROE10Y = mean(mapYearValues(last10, func(y YearMetric) float64 { return y.ROE }))
	out.AverageROIC5Y = mean(mapYearValues(last5, func(y YearMetric) float64 { return y.ROIC }))
	out.GrossMarginMedian5Y = median(mapYearValues(last5, func(y YearMetric) float64 { return y.GrossMargin }))
	out.OperatingMarginMedian5Y = median(mapYearValues(last5, func(y YearMetric) float64 { return y.OperatingMargin }))
	out.NetMarginMedian5Y = median(mapYearValues(last5, func(y YearMetric) float64 { return y.NetMargin }))
	out.MarginStability = stabilityScore(mapYearValues(last5, func(y YearMetric) float64 { return y.OperatingMargin }))
	out.RevenueCAGR5Y = cagrFromValues(mapYearValues(last5, func(y YearMetric) float64 { return y.Revenue }))
	score := 0.0
	score += thresholdScore(out.AverageROE5Y, []threshold{{0.25, 30}, {0.18, 25}, {0.12, 18}, {0.08, 10}})
	score += thresholdScore(out.AverageROIC5Y, []threshold{{0.20, 25}, {0.14, 20}, {0.10, 14}, {0.06, 8}})
	score += out.MarginStability * 20
	score += thresholdScore(out.RevenueCAGR5Y, []threshold{{0.25, 15}, {0.15, 12}, {0.08, 8}, {0.02, 4}})
	score += thresholdScore(out.OperatingMarginMedian5Y, []threshold{{0.25, 10}, {0.15, 8}, {0.08, 5}, {0.03, 2}})
	out.Score = mathutil.Clamp(score, 0, 100)
	switch {
	case out.Score >= 75:
		out.Label = "güçlü_moat_adayı"
	case out.Score >= 55:
		out.Label = "orta_moat"
	default:
		out.Label = "zayıf_moat"
	}
	return out
}

func estimateIntrinsicValue(fin financialFile, latest period, years []YearMetric, model SectorModelReport, report Report) IntrinsicValueReport {
	shares := fieldValue(fin, "2OA", latest)
	equity := firstFieldValue(fin, latest, "2N", "2O")
	if isBankFinancialSchema(fin) {
		equity = fieldValue(fin, "2O", latest)
	}
	netDebt := latestNetDebt(fin, latest)
	bookPerShare := mathutil.SafeDiv(equity, shares)
	out := IntrinsicValueReport{Method: model.Model}
	if shares <= 0 {
		out.Drivers = append(out.Drivers, "paid_capital_missing")
		return out
	}
	switch model.Model {
	case "bank_residual_income", "insurance_book_value", "financial_book_value", "gyo_nav_proxy":
		roe := report.Moat.AverageROE5Y
		required := math.Max(0.12, report.Assumptions.DiscountRate*0.55)
		pb := mathutil.Clamp(1+mathutil.SafeDiv(roe-required, required), 0.35, 2.25)
		if model.Model == "gyo_nav_proxy" {
			pb = mathutil.Clamp(pb, 0.40, 1.35)
		}
		out.Base = bookPerShare * pb
		out.Bear = bookPerShare * math.Max(pb-0.25, 0.25)
		out.Bull = bookPerShare * (pb + 0.30)
		out.Computed = out.Base > 0
		out.Drivers = []string{"book_value_per_share", "normalized_roe", "sector_book_value_model"}
		out.Confidence = mathutil.Clamp(report.DataQuality*0.35+report.Moat.Score*0.35+report.CapitalAllocation.Score*0.30, 0, 100)
	case "holding_sum_of_parts_proxy":
		out.Base = bookPerShare * 0.75
		out.Bear = bookPerShare * 0.55
		out.Bull = bookPerShare * 0.95
		out.Computed = out.Base > 0
		out.Drivers = []string{"book_value_proxy", "holding_discount", "sum_of_parts_data_missing"}
		out.Confidence = mathutil.Clamp(report.DataQuality*0.30+report.Moat.Score*0.20+35, 0, 70)
	default:
		owner := firstPositive(report.OwnerEarnings.Normalized5Y, report.OwnerEarnings.Normalized10Y, report.OwnerEarnings.TTM)
		if owner <= 0 {
			out = operatingBookValueCrossCheck(bookPerShare, years, report, model, "normalized_owner_earnings_not_positive")
			if out.Computed {
				return out
			}
			out.Drivers = append(out.Drivers, "normalized_owner_earnings_not_positive", "book_value_crosscheck_not_available")
			out.Confidence = mathutil.Clamp(report.DataQuality*0.25+report.Moat.Score*0.20, 0, 45)
			return out
		}
		growth := mathutil.Clamp(report.Moat.RevenueCAGR5Y, -0.03, report.Assumptions.OwnerEarningsGrowth)
		discount := report.Assumptions.DiscountRate
		out.Base = perShareDCF(owner, netDebt, shares, growth, discount, report.Assumptions.TerminalGrowth)
		out.Bear = perShareDCF(owner, netDebt, shares, growth-0.03, discount+0.03, report.Assumptions.TerminalGrowth-0.01)
		out.Bull = perShareDCF(owner, netDebt, shares, growth+0.02, math.Max(discount-0.02, 0.10), report.Assumptions.TerminalGrowth+0.01)
		if out.Base > 0 && bookPerShare > 0 && out.Base < bookPerShare*0.25 && report.OwnerEarnings.TTM > 0 && latestPositiveOperatingProfile(years) {
			fallback := operatingBookValueCrossCheck(bookPerShare, years, report, model, "owner_earnings_dcf_below_book_value_floor")
			if fallback.Computed {
				return fallback
			}
		}
		out.Computed = out.Base > 0
		out.Drivers = []string{"normalized_owner_earnings", "maintenance_capex_estimate", "net_debt", "discounted_cash_flow"}
		out.Confidence = mathutil.Clamp(report.DataQuality*0.30+report.OwnerEarnings.Score*0.25+report.NormalizedFCF.Score*0.20+report.Moat.Score*0.25, 0, 100)
	}
	if out.Base <= 0 {
		if operatingCashFlowModel(model.Model) {
			fallback := operatingBookValueCrossCheck(bookPerShare, years, report, model, "dcf_equity_value_not_positive")
			if fallback.Computed {
				return fallback
			}
		}
		out.Computed = false
		out.Bear = 0
		out.Base = 0
		out.Bull = 0
		out.Drivers = append(out.Drivers, "positive_equity_value_not_available")
	}
	return out
}

func latestPositiveOperatingProfile(years []YearMetric) bool {
	if len(years) == 0 {
		return false
	}
	latest := years[len(years)-1]
	return latest.NetIncome > 0 && latest.OperatingProfit > 0 && (latest.OperatingCash > 0 || latest.FreeCashFlow > 0)
}

func operatingBookValueCrossCheck(bookPerShare float64, years []YearMetric, report Report, model SectorModelReport, reason string) IntrinsicValueReport {
	method := model.Model + "_book_value_crosscheck"
	if model.Model == "owner_earnings_dcf" {
		method = "owner_earnings_dcf_book_value_crosscheck"
	}
	out := IntrinsicValueReport{Method: method}
	if bookPerShare <= 0 {
		out.Drivers = []string{reason, "book_value_per_share_not_available"}
		return out
	}
	qualityPB := 0.55 +
		0.25*(report.QualityScore/100) +
		0.25*(report.Moat.Score/100) +
		0.15*(report.CapitalAllocation.Score/100) +
		0.10*(report.OwnerEarnings.Score/100) +
		0.10*(report.NormalizedFCF.Score/100)
	if len(years) > 0 {
		latest := years[len(years)-1]
		if latest.NetIncome < 0 {
			qualityPB -= 0.10
		}
		if latest.FreeCashFlow < 0 {
			qualityPB -= 0.10
		}
		if latest.ROIC < 0.06 {
			qualityPB -= 0.05
		}
	}
	if !financialCapitalModel(model) && report.CapitalAllocation.Dilution5YPct > 10 {
		qualityPB -= 0.10
	}
	basePB := mathutil.Clamp(qualityPB, 0.35, 1.15)
	bearPB := mathutil.Clamp(basePB-0.20, 0.25, 1.00)
	bullPB := mathutil.Clamp(basePB+0.25, 0.45, 1.35)
	out.Bear = bookPerShare * bearPB
	out.Base = bookPerShare * basePB
	out.Bull = bookPerShare * bullPB
	out.Computed = out.Base > 0
	out.Drivers = []string{reason, "book_value_per_share", "quality_adjusted_pb"}
	if financialCapitalModel(model) {
		out.Drivers = append(out.Drivers, "roe_and_regulatory_capital_proxy")
	} else {
		out.Drivers = append(out.Drivers, "cash_flow_weakness_discount", "dilution_discount")
	}
	out.Confidence = mathutil.Clamp(report.DataQuality*0.30+report.CapitalAllocation.Score*0.25+report.Moat.Score*0.20+report.OwnerEarnings.Score*0.10+report.NormalizedFCF.Score*0.10+5, 0, 72)
	return out
}

func chooseSectorModel(sector, industry, financialGroup string) SectorModelReport {
	text := normalizeText(sector + " " + industry + " " + financialGroup)
	switch {
	case strings.Contains(text, "banka") || strings.Contains(text, "bank"):
		return SectorModelReport{Model: "bank_residual_income", Label: "Banka / özsermaye getirisi modeli", Reason: "Bankalarda nakit akımı yerine özsermaye, ROE, aktif kalitesi ve defter değeri daha anlamlıdır.", Primary: []string{"book_value_per_share", "roe", "capital_adequacy_proxy", "asset_quality_proxy"}, Secondary: []string{"net_income_stability", "loan_deposit_proxy", "paid_capital_event_classification"}}
	case strings.Contains(text, "sigorta") || strings.Contains(text, "insurance"):
		return SectorModelReport{Model: "insurance_book_value", Label: "Sigorta / defter değeri modeli", Reason: "Sigorta şirketlerinde özsermaye, teknik kârlılık ve yatırım geliri nakit akımı modelinden daha anlamlıdır.", Primary: []string{"book_value_per_share", "roe", "equity_growth"}, Secondary: []string{"net_margin", "dividend_policy"}}
	case strings.Contains(text, "gayrimenkul") || strings.Contains(text, "gyo") || strings.Contains(text, "reit"):
		return SectorModelReport{Model: "gyo_nav_proxy", Label: "GYO / net aktif değer proxy modeli", Reason: "GYO değerlemesinde portföy/NAV yaklaşımı daha uygundur; portföy detay verisi yoksa defter değeri proxy kullanılır.", Primary: []string{"book_value_per_share", "net_debt_to_equity", "nav_proxy"}, Secondary: []string{"dividend_policy", "asset_growth"}}
	case strings.Contains(text, "holding"):
		return SectorModelReport{Model: "holding_sum_of_parts_proxy", Label: "Holding / iştirak değeri proxy modeli", Reason: "Holdinglerde ideal model sum-of-parts'tır; iştirak detayı yoksa defter değeri iskontosu proxy kullanılır.", Primary: []string{"book_value_per_share", "holding_discount"}, Secondary: []string{"capital_allocation", "debt_control"}}
	case strings.Contains(text, "finansal kiralama") || strings.Contains(text, "faktoring") || strings.Contains(text, "araci kurum") || strings.Contains(text, "yatirim ortakligi"):
		return SectorModelReport{Model: "financial_book_value", Label: "Finansal şirket / özsermaye modeli", Reason: "Finansal şirketlerde bilanço ve özsermaye getirisi nakit akımından daha anlamlıdır.", Primary: []string{"book_value_per_share", "roe"}, Secondary: []string{"net_income_stability", "paid_capital_dilution"}}
	case (strings.Contains(text, "teknoloji") || strings.Contains(text, "bilisim") || strings.Contains(text, "bilgi hizmet") || strings.Contains(text, "yazilim")) && !strings.Contains(text, "savunma"):
		return SectorModelReport{
			Model:     "technology_growth_quality",
			Label:     "Teknoloji / büyüme kalitesi ve nakit dönüşümü modeli",
			Reason:    "Teknoloji şirketlerinde değer; brüt marj, AR-GE, tekrarlayan gelir, ölçeklenebilir büyüme, net nakit ve FCF dönüşümüyle birlikte ölçülmelidir.",
			Primary:   []string{"gross_margin", "revenue_growth", "normalized_fcf", "net_cash", "research_and_development"},
			Secondary: []string{"recurring_revenue", "customer_concentration", "roic", "peer_multiples"},
		}
	default:
		return SectorModelReport{Model: "owner_earnings_dcf", Label: "Operasyonel şirket / owner earnings modeli", Reason: "Operasyonel şirketlerde sahibine kalan nakit ve normalize serbest nakit akımı ana değer sürücüsüdür.", Primary: []string{"owner_earnings", "normalized_fcf", "roic"}, Secondary: []string{"moat", "capital_allocation", "peer_multiples"}}
	}
}

func operatingCashFlowModel(model string) bool {
	switch model {
	case "owner_earnings_dcf", "technology_growth_quality":
		return true
	default:
		return false
	}
}

func marginOfSafety(price float64, intrinsic IntrinsicValueReport, required float64) MarginOfSafetyReport {
	out := MarginOfSafetyReport{RequiredPct: required}
	if price <= 0 || !intrinsic.Computed || intrinsic.Base <= 0 {
		out.Label = "hesaplanamadı"
		return out
	}
	out.Computed = true
	out.BearPct = 100 * (intrinsic.Bear - price) / intrinsic.Bear
	out.BasePct = 100 * (intrinsic.Base - price) / intrinsic.Base
	out.BullPct = 100 * (intrinsic.Bull - price) / intrinsic.Bull
	switch {
	case out.BasePct >= required:
		out.Label = "güvenlik_marjı_var"
	case out.BasePct >= 0:
		out.Label = "değerine_yakın"
	default:
		out.Label = "güvenlik_marjı_yok"
	}
	return out
}

func decision(report Report) (string, string) {
	if !report.IntrinsicValue.Computed || !report.MarginOfSafety.Computed {
		return "HESAPLANAMADI", "İçsel değer güvenilir hesaplanamadı"
	}
	switch {
	case report.MarginOfSafety.BasePct >= report.Assumptions.RequiredMarginPct && report.QualityScore >= 60 && report.Confidence >= 55:
		return "UCUZ", "İçsel değere göre güvenlik marjı var"
	case report.MarginOfSafety.BasePct >= 0 && report.QualityScore >= 50:
		return "MAKUL", "Fiyat içsel değere yakın fakat güvenlik marjı sınırlı"
	case report.MarginOfSafety.BasePct < -10:
		return "PAHALI", "Fiyat baz içsel değerin üzerinde"
	default:
		return "BEKLE", "Güvenlik marjı yeterli değil"
	}
}

func summary(report Report) string {
	if !report.IntrinsicValue.Computed {
		return "İçsel değer hesaplanamadı; owner earnings, defter değeri veya sektör modeli için yeterli güvenilir veri yok."
	}
	answer := report.FairValue.Label
	if answer == "" {
		answer = report.DecisionLabel
	}
	return "Güncel fiyat " + formatFloat(report.CurrentPrice) + " " + report.Currency +
		", baz içsel değer " + formatFloat(report.IntrinsicValue.Base) + " " + report.Currency +
		", güvenlik marjı " + formatFloat(report.MarginOfSafety.BasePct) + "%. Cevap: " + answer + "."
}

func valueChecks(report Report) []Check {
	checks := []Check{}
	add := func(name, status, message string, score float64) {
		checks = append(checks, Check{Name: name, Status: status, Message: message, Score: score})
	}
	if report.IntrinsicValue.Computed {
		add("intrinsic_value", "pass", "İçsel değer aralığı hesaplandı.", report.IntrinsicValue.Confidence)
	} else {
		add("intrinsic_value", "fail", "İçsel değer güvenilir hesaplanamadı.", report.IntrinsicValue.Confidence)
	}
	if report.FairValue.Computed {
		add("fair_value_conclusion", statusForFairValue(report.FairValue), report.FairValue.Explanation, report.Confidence)
	} else {
		add("fair_value_conclusion", "fail", "Fiyat ile içsel değer farkı hesaplanamadı.", 0)
	}
	if report.OwnerEarnings.Applicable {
		add("owner_earnings", statusForScore(report.OwnerEarnings.Score), "Owner earnings ve bakım capex proxy hesaplandı.", report.OwnerEarnings.Score)
	} else {
		add("owner_earnings", "not_applicable", "Bu sektör modelinde owner earnings ana değerleme girdisi değildir.", report.OwnerEarnings.Score)
	}
	if report.NormalizedFCF.Applicable {
		add("normalized_fcf", statusForScore(report.NormalizedFCF.Score), "5-10 yıllık normalize FCF hesaplandı.", report.NormalizedFCF.Score)
	} else {
		add("normalized_fcf", "not_applicable", "Bu sektör modelinde serbest nakit akımı ana değerleme girdisi değildir.", report.NormalizedFCF.Score)
	}
	capitalMessage := "Temettü, sermaye artırımı, pay sulanması ve borç disiplini ölçüldü."
	if financialCapitalModel(report.SectorModel) {
		capitalMessage = "Temettü ve sermaye hareketleri izlendi; ödenmiş sermaye artışı bedelli/bedelsiz/split/nominal düzeltme olarak ayrıca sınıflandırılmalıdır."
	}
	add("capital_allocation", statusForScore(report.CapitalAllocation.Score), capitalMessage, report.CapitalAllocation.Score)
	add("moat", statusForScore(report.Moat.Score), "ROE/ROIC, marj istikrarı ve büyüme kalitesiyle moat proxy üretildi.", report.Moat.Score)
	add("kap_document_evidence", statusForScore(report.DocumentEvidence.CoverageScore), report.DocumentEvidence.Summary, report.DocumentEvidence.CoverageScore)
	if report.MacroGDP.Computed {
		add("tuik_gdp_macro", statusForScore(report.MacroGDP.Score), report.MacroGDP.Interpretation+" "+report.MacroGDP.EquityImpact, report.MacroGDP.Score)
	} else if report.MacroGDP.DataQualityWarning != "" {
		add("tuik_gdp_macro", "limited", report.MacroGDP.DataQualityWarning, 35)
	}
	if report.BuffettChecklist.Computed {
		add("buffett_value_checklist", report.BuffettChecklist.Status, report.BuffettChecklist.Summary, report.BuffettChecklist.Score)
	}
	return checks
}

func buildBuffettChecklist(report Report) BuffettChecklistReport {
	out := BuffettChecklistReport{
		Computed:           true,
		MethodologyVersion: "buffett_value_checklist_v1",
	}
	add := func(id, pillar, label, status string, required bool, value, threshold, evidence string, missing ...string) {
		item := BuffettChecklistRequirement{
			ID:        id,
			Pillar:    pillar,
			Label:     label,
			Status:    status,
			Required:  required,
			Value:     value,
			Threshold: threshold,
			Evidence:  evidence,
			Missing:   compactStrings(missing),
		}
		out.Requirements = append(out.Requirements, item)
		if required {
			switch status {
			case "fail", "missing":
				out.BlockingIssues = append(out.BlockingIssues, id)
			case "limited":
				out.BlockingIssues = append(out.BlockingIssues, id+"_limited")
			}
		}
		out.MissingData = append(out.MissingData, item.Missing...)
	}
	years := report.Years
	latest := YearMetric{}
	if len(years) > 0 {
		latest = years[len(years)-1]
	}
	hasModel := strings.TrimSpace(report.SectorModel.Model) != ""
	add(
		"business_model_understandable",
		"business",
		"İş modeli ve sektör modeli anlaşılır mı?",
		statusByBool(hasModel, false),
		true,
		firstNonEmpty(report.SectorModel.Label, report.SectorModel.Model),
		"sektör modeli seçilmiş olmalı",
		report.SectorModel.Reason,
		missingIf(!hasModel, "sector_model"),
	)
	historyStatus := "missing"
	switch {
	case len(years) >= 10:
		historyStatus = "pass"
	case len(years) >= 5:
		historyStatus = "limited"
	}
	add(
		"consistent_operating_history",
		"business",
		"Tutarlı faaliyet geçmişi",
		historyStatus,
		true,
		fmt.Sprintf("%d yıl", len(years)),
		"en az 10 yıl güçlü, 5 yıl sınırlı kabul edilir",
		"Gelir, kâr, nakit akışı ve sermaye kalemlerinin tarihsel seri uzunluğu.",
		missingIf(len(years) < 5, "5y_financial_history"),
	)
	prospectStatus := statusForThreshold(report.Moat.RevenueCAGR5Y, 0.08, 0.02)
	add(
		"long_term_prospects",
		"business",
		"Uzun vadeli büyüme zemini",
		prospectStatus,
		true,
		percentText(report.Moat.RevenueCAGR5Y),
		"5Y gelir CAGR >= %8 güçlü, >= %2 sınırlı",
		"Gelir büyümesi moat skoruna bağlı uzun vade proxy'sidir.",
		missingIf(len(years) < 5, "5y_revenue_history"),
	)
	add(
		"capital_allocation_discipline",
		"management",
		"Yönetim sermayeyi rasyonel dağıtıyor mu?",
		statusForScore(report.CapitalAllocation.Score),
		true,
		scoreText(report.CapitalAllocation.Score),
		">= 70/100",
		"Sermaye artışı, temettü sürekliliği, borç disiplini ve sulanma birlikte ölçülür.",
		report.CapitalAllocation.Warnings...,
	)
	dilutionStatus := "pass"
	if report.CapitalAllocation.CapitalMovementClassificationRequired {
		dilutionStatus = "limited"
	}
	if report.CapitalAllocation.Dilution5YPct > 20 {
		dilutionStatus = "fail"
	}
	add(
		"dilution_and_shareholder_alignment",
		"management",
		"Pay sulanması ve hissedar hizalanması",
		dilutionStatus,
		true,
		percentPlainText(report.CapitalAllocation.Dilution5YPct),
		"5Y sulanma <= %10; üzerinde sınıflama gerekir",
		"Ödenmiş sermaye değişimi bedelli/bedelsiz/split ayrımıyla denetlenir.",
		missingIf(report.CapitalAllocation.CapitalMovementClassificationRequired, "capital_movement_classification"),
	)
	retainedStatus := "missing"
	retainedValue := ""
	retainedMissing := []string{"retained_earnings_history", "market_cap_history"}
	if report.RetainedEarnings.Computed {
		retainedStatus = "pass"
		if report.RetainedEarnings.Ratio < 0 {
			retainedStatus = "fail"
		} else if report.RetainedEarnings.Ratio < 1 {
			retainedStatus = "limited"
		}
		retainedValue = ratioText(report.RetainedEarnings.Ratio)
		retainedMissing = report.RetainedEarnings.Warnings
	}
	add(
		"one_dollar_retained_earnings_test",
		"management",
		"1 Dolar dağıtılmamış kâr testi",
		retainedStatus,
		true,
		retainedValue,
		"5Y piyasa değeri artışı / 5Y dağıtılmamış kâr >= 1.0",
		"Yönetimin içeride tuttuğu kârın piyasa değerine dönüşümü ölçülür.",
		retainedMissing...,
	)
	add(
		"roe_threshold",
		"financial",
		"Özsermaye kârlılığı eşiği",
		statusForThreshold(report.Moat.AverageROE5Y, 0.15, 0.08),
		true,
		percentText(report.Moat.AverageROE5Y),
		"5Y ortalama ROE >= %15; tercih >= %20",
		"Buffett finansal sütunu için ana sermaye verimliliği metriği.",
		missingIf(len(years) < 5, "5y_roe_history"),
	)
	add(
		"gross_margin_threshold",
		"financial",
		"Brüt kâr marjı",
		statusForThreshold(report.Moat.GrossMarginMedian5Y, 0.40, 0.25),
		true,
		percentText(report.Moat.GrossMarginMedian5Y),
		"5Y medyan brüt marj >= %40",
		"Fiyatlama gücü ve operasyonel katma değer proxy'si.",
		missingIf(len(years) < 5, "5y_gross_margin_history"),
	)
	add(
		"net_margin_threshold",
		"financial",
		"Net kâr marjı",
		statusForThreshold(report.Moat.NetMarginMedian5Y, 0.20, 0.08),
		true,
		percentText(report.Moat.NetMarginMedian5Y),
		"5Y medyan net marj >= %20",
		"Kârın ortaklara kalabilen kısmını ölçer.",
		missingIf(len(years) < 5, "5y_net_margin_history"),
	)
	ownerStatus := "not_applicable"
	ownerMissing := []string(nil)
	if report.OwnerEarnings.Applicable {
		ownerStatus = statusForThreshold(report.OwnerEarnings.Score, 70, 45)
		if report.OwnerEarnings.TTM <= 0 || report.OwnerEarnings.Normalized5Y <= 0 {
			ownerStatus = weakerStatus(ownerStatus, "limited")
			ownerMissing = append(ownerMissing, "positive_owner_earnings")
		}
		ownerMissing = append(ownerMissing, report.OwnerEarnings.Warnings...)
	}
	add(
		"owner_earnings_quality",
		"financial",
		"Sahibine kalan nakit",
		ownerStatus,
		report.OwnerEarnings.Applicable,
		fmt.Sprintf("TTM %s | 5Y normalize %s | %s", formatFloat(report.OwnerEarnings.TTM), formatFloat(report.OwnerEarnings.Normalized5Y), scoreText(report.OwnerEarnings.Score)),
		"pozitif owner earnings ve skor >= 70/100",
		"Owner earnings = operasyonel nakit akışı - bakım capex tahmini.",
		ownerMissing...,
	)
	capexRatio := mathutil.SafeDiv(math.Abs(latest.Capex), math.Abs(latest.NetIncome))
	capexStatus := "missing"
	capexMissing := []string{"capex_or_net_income_history"}
	if latest.NetIncome != 0 && latest.Capex != 0 {
		capexStatus = "pass"
		capexMissing = nil
		if capexRatio > 0.50 {
			capexStatus = "fail"
		} else if capexRatio > 0.25 {
			capexStatus = "limited"
		}
	}
	add(
		"capex_intensity",
		"financial",
		"Sermaye harcaması yoğunluğu",
		capexStatus,
		report.OwnerEarnings.Applicable,
		ratioText(capexRatio),
		"CapEx / net kâr < 0.25",
		"Yüksek bakım yatırım ihtiyacı sahibine kalan nakdi azaltır.",
		capexMissing...,
	)
	debtStatus := "missing"
	debtMissing := []string{"cash_and_debt_history"}
	if latest.Cash != 0 || latest.Debt != 0 {
		debtStatus = "pass"
		debtMissing = nil
		if latest.Cash < latest.Debt && report.CapitalAllocation.NetDebtToEquity > 0.80 {
			debtStatus = "fail"
		} else if latest.Cash < latest.Debt {
			debtStatus = "limited"
		}
	}
	add(
		"cash_vs_debt",
		"financial",
		"Nakit ve borç dayanıklılığı",
		debtStatus,
		!financialCapitalModel(report.SectorModel),
		fmt.Sprintf("nakit %s | borç %s | net borç/özsermaye %.2f", formatFloat(latest.Cash), formatFloat(latest.Debt), report.CapitalAllocation.NetDebtToEquity),
		"nakit > toplam borç veya net borç/özsermaye <= 0.80",
		"Bilanço şoku ve faiz hassasiyeti kontrolü.",
		debtMissing...,
	)
	add(
		"moat_proxy",
		"financial",
		"Ekonomik hendek proxy'si",
		statusForScore(report.Moat.Score),
		true,
		fmt.Sprintf("%s | %s", report.Moat.Label, scoreText(report.Moat.Score)),
		">= 70/100",
		"ROE/ROIC, marj istikrarı ve büyüme kalitesiyle ölçülür.",
		report.Moat.Warnings...,
	)
	add(
		"intrinsic_value_computed",
		"market",
		"İçsel değer hesaplandı mı?",
		statusByBool(report.IntrinsicValue.Computed, false),
		true,
		formatFloat(report.IntrinsicValue.Base),
		"pozitif baz içsel değer",
		strings.Join(report.IntrinsicValue.Drivers, ", "),
		missingIf(!report.IntrinsicValue.Computed, "intrinsic_value_inputs"),
	)
	marginStatus := "missing"
	if report.MarginOfSafety.Computed {
		marginStatus = "fail"
		if report.MarginOfSafety.BasePct >= report.MarginOfSafety.RequiredPct {
			marginStatus = "pass"
		} else if report.MarginOfSafety.BasePct >= 0 {
			marginStatus = "limited"
		}
	}
	add(
		"margin_of_safety",
		"market",
		"Güvenlik marjı",
		marginStatus,
		true,
		percentPlainText(report.MarginOfSafety.BasePct),
		fmt.Sprintf(">= %.1f%%", report.MarginOfSafety.RequiredPct),
		"Fiyat, muhafazakâr içsel değer altında yeterli iskonto taşımalı.",
		missingIf(!report.MarginOfSafety.Computed, "current_price_or_intrinsic_value"),
	)
	add(
		"valuation_assumptions",
		"market",
		"DCF / değerleme varsayımları açık mı?",
		statusByBool(strings.TrimSpace(report.Assumptions.Source) != "", false),
		true,
		report.Assumptions.Source,
		"iskonto, terminal büyüme ve gerekli marj kaynağı olmalı",
		fmt.Sprintf("iskonto %s, terminal %s, gerekli marj %.1f%%", percentText(report.Assumptions.DiscountRate), percentText(report.Assumptions.TerminalGrowth), report.Assumptions.RequiredMarginPct),
		missingIf(strings.TrimSpace(report.Assumptions.Source) == "", "valuation_assumptions_source"),
	)
	pass, limited, required := 0, 0, 0
	for _, item := range out.Requirements {
		if item.Required {
			required++
			switch item.Status {
			case "pass":
				pass++
			case "limited", "not_applicable":
				limited++
			}
		}
	}
	out.MissingData = uniqueNonEmptyStrings(out.MissingData)
	out.BlockingIssues = uniqueNonEmptyStrings(out.BlockingIssues)
	out.CoveragePct = mathutil.Clamp(100*mathutil.SafeDiv(float64(required-len(out.MissingData)), float64(required)), 0, 100)
	out.Score = mathutil.Clamp(100*mathutil.SafeDiv(float64(pass)+0.5*float64(limited), float64(required)), 0, 100)
	switch {
	case len(out.BlockingIssues) == 0 && out.Score >= 80:
		out.Status = "pass"
		out.StatusLabel = "Buffett filtresi geçti"
	case out.Score >= 55:
		out.Status = "limited"
		out.StatusLabel = "Buffett filtresi sınırlı"
	default:
		out.Status = "fail"
		out.StatusLabel = "Buffett filtresi başarısız"
	}
	out.BuyEligible = out.Status == "pass" && report.MarginOfSafety.Computed && report.MarginOfSafety.BasePct >= report.MarginOfSafety.RequiredPct
	out.Summary = fmt.Sprintf("%s: %d zorunlu kriterin %d tanesi geçti, %d tanesi sınırlı/uygulanmaz, %d engel var; kapsam %.0f/100, skor %.0f/100.",
		out.StatusLabel,
		required,
		pass,
		limited,
		len(out.BlockingIssues),
		out.CoveragePct,
		out.Score,
	)
	if !out.BuyEligible {
		out.Summary += " Bu sonuç tek başına AL kararı üretmez."
	}
	return out
}

func fairValueConclusion(report Report) FairValueConclusion {
	out := FairValueConclusion{
		CurrentPrice:      report.CurrentPrice,
		FairValueBear:     report.IntrinsicValue.Bear,
		FairValueBase:     report.IntrinsicValue.Base,
		FairValueBull:     report.IntrinsicValue.Bull,
		RequiredMarginPct: report.Assumptions.RequiredMarginPct,
	}
	out.DataInputs = []string{
		"son kapanış/fiyat verisi",
		"finansal tablo JSON",
		"sektör modeline göre içsel değer",
	}
	if report.DocumentEvidence.Computed {
		out.DataInputs = append(out.DataInputs, fmt.Sprintf("KAP belge kanıtı: %d ek, skor %.0f/100", report.DocumentEvidence.TotalFiles, report.DocumentEvidence.CoverageScore))
	} else {
		out.DataInputs = append(out.DataInputs, "KAP belge kanıtı: eksik")
	}
	if report.MacroGDP.Computed {
		out.DataInputs = append(out.DataInputs, fmt.Sprintf("TÜİK GSYH: %d, makro skor %.0f/100", report.MacroGDP.LatestYear, report.MacroGDP.Score))
	} else if report.MacroGDP.DataQualityWarning != "" {
		out.DataInputs = append(out.DataInputs, "TÜİK GSYH: "+report.MacroGDP.DataQualityWarning)
	}
	if report.CurrentPrice <= 0 || !report.IntrinsicValue.Computed || report.IntrinsicValue.Base <= 0 || !report.MarginOfSafety.Computed {
		out.Status = "not_computed"
		out.Label = "Hesaplanamadı"
		out.Explanation = "Güncel fiyat veya içsel değer güvenilir olmadığı için fiyat/değer farkı hesaplanamadı."
		return out
	}
	out.Computed = true
	out.PriceToFairValuePct = 100 * (report.CurrentPrice - report.IntrinsicValue.Base) / report.IntrinsicValue.Base
	out.UpsideDownsidePct = 100 * (report.IntrinsicValue.Base - report.CurrentPrice) / report.CurrentPrice
	out.MarginOfSafetyPct = report.MarginOfSafety.BasePct
	switch {
	case report.MarginOfSafety.BasePct >= report.Assumptions.RequiredMarginPct:
		out.Status = "undervalued_with_margin"
		out.Label = "İçsel değerin altında - güvenlik marjı var"
	case report.CurrentPrice < report.IntrinsicValue.Base:
		out.Status = "undervalued_limited_margin"
		out.Label = "İçsel değerin altında fakat güvenlik marjı sınırlı"
	case math.Abs(out.PriceToFairValuePct) <= 10:
		out.Status = "fair_value_band"
		out.Label = "Makul değer bandında"
	default:
		out.Status = "overvalued"
		out.Label = "İçsel değerin üstünde"
	}
	out.Explanation = fmt.Sprintf(
		"Güncel fiyat baz içsel değere göre %.1f%% fark taşıyor; baz senaryoya göre potansiyel yukarı/aşağı alan %.1f%%. Gerekli güvenlik marjı %.1f%%.",
		out.PriceToFairValuePct,
		out.UpsideDownsidePct,
		out.RequiredMarginPct,
	)
	if report.MacroGDP.Computed {
		out.Explanation += " TÜİK GSYH okuması makro talep zemini için " + report.MacroGDP.Regime + " olarak rapora bağlandı."
	}
	return out
}

func statusForFairValue(value FairValueConclusion) string {
	switch value.Status {
	case "undervalued_with_margin", "fair_value_band", "undervalued_limited_margin":
		return "pass"
	case "overvalued":
		return "limited"
	default:
		return "fail"
	}
}

func dataQuality(fin financialFile, years []YearMetric, model SectorModelReport) float64 {
	required := requiredFieldsForModel(fin, model)
	hit := 0
	for _, code := range required {
		if _, ok := fin.Data[code]; ok {
			hit++
		}
	}
	history := mathutil.Clamp(float64(len(years))/10, 0, 1)
	return mathutil.Clamp(100*(0.65*float64(hit)/float64(len(required))+0.35*history), 0, 100)
}

func requiredFieldsForModel(fin financialFile, model SectorModelReport) []string {
	switch model.Model {
	case "bank_residual_income", "insurance_book_value", "financial_book_value", "gyo_nav_proxy":
		if isBankFinancialSchema(fin) {
			return []string{"3Z", "2OA", "2O", "1Z", "3C", "3CE"}
		}
		return []string{"3L", "2OA", "2N", "1BL", "1AA", "2AA"}
	case "holding_sum_of_parts_proxy":
		return []string{"3L", "4CB", "2OA", "2N", "1BL", "1AA", "2AA"}
	default:
		return []string{"3C", "3L", "4C", "4CAI", "4CB", "2OA", "2N", "1BL"}
	}
}

func qualityScore(report Report, model SectorModelReport) float64 {
	switch model.Model {
	case "bank_residual_income", "insurance_book_value", "financial_book_value", "gyo_nav_proxy":
		return weightedScore([]scoreWeight{
			{report.Moat.Score, 0.55},
			{report.CapitalAllocation.Score, 0.35},
			{report.DataQuality, 0.10},
		})
	case "holding_sum_of_parts_proxy":
		return weightedScore([]scoreWeight{
			{report.Moat.Score, 0.40},
			{report.CapitalAllocation.Score, 0.35},
			{report.NormalizedFCF.Score, 0.10},
			{report.DataQuality, 0.15},
		})
	default:
		return weightedScore([]scoreWeight{
			{report.OwnerEarnings.Score, 0.22},
			{report.NormalizedFCF.Score, 0.22},
			{report.CapitalAllocation.Score, 0.22},
			{report.Moat.Score, 0.34},
		})
	}
}

func confidenceScore(report Report) float64 {
	return mathutil.Clamp(report.DataQuality*0.30+report.IntrinsicValue.Confidence*0.35+report.QualityScore*0.25+report.CapitalAllocation.Score*0.10, 0, 100)
}

func valueScore(report Report) float64 {
	marginScore := mathutil.Clamp((report.MarginOfSafety.BasePct+20)/60*100, 0, 100)
	if !report.MarginOfSafety.Computed {
		marginScore = 0
	}
	return mathutil.Clamp(report.QualityScore*0.50+marginScore*0.35+report.Confidence*0.15, 0, 100)
}

func statusForScore(score float64) string {
	switch {
	case score >= 70:
		return "pass"
	case score >= 45:
		return "limited"
	default:
		return "fail"
	}
}

func statusForThreshold(value, pass, limited float64) string {
	switch {
	case value >= pass:
		return "pass"
	case value >= limited:
		return "limited"
	default:
		return "fail"
	}
}

func statusByBool(ok bool, limited bool) string {
	if ok {
		return "pass"
	}
	if limited {
		return "limited"
	}
	return "missing"
}

func weakerStatus(current, candidate string) string {
	rank := map[string]int{
		"pass":           4,
		"not_applicable": 3,
		"limited":        2,
		"missing":        1,
		"fail":           0,
	}
	if rank[candidate] < rank[current] {
		return candidate
	}
	return current
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func missingIf(condition bool, value string) string {
	if condition {
		return value
	}
	return ""
}

func scoreText(score float64) string {
	return fmt.Sprintf("%.0f/100", score)
}

func percentText(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}

func percentPlainText(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func ratioText(value float64) string {
	if value == 0 {
		return "0.00x"
	}
	return fmt.Sprintf("%.2fx", value)
}

func loadAssumptions(path string, sector string) Assumptions {
	out := Assumptions{
		Source:              "default_static",
		DiscountRate:        0.18,
		TerminalGrowth:      0.05,
		OwnerEarningsGrowth: 0.08,
		TaxRate:             0.25,
		RequiredMarginPct:   25,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var payload struct {
		Default struct {
			WACC           float64 `json:"wacc"`
			TerminalGrowth float64 `json:"terminal_growth"`
			FCFGrowth      float64 `json:"fcf_growth"`
			TaxRate        float64 `json:"tax_rate"`
		} `json:"default"`
		Sectors map[string]struct {
			WACC           float64 `json:"wacc"`
			TerminalGrowth float64 `json:"terminal_growth"`
			FCFGrowth      float64 `json:"fcf_growth"`
			TaxRate        float64 `json:"tax_rate"`
		} `json:"sectors"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return out
	}
	record := payload.Default
	for key, candidate := range payload.Sectors {
		if strings.Contains(normalizeText(sector), normalizeText(key)) {
			record = candidate
			break
		}
	}
	out.Source = path
	if record.WACC > 0 {
		out.DiscountRate = record.WACC
	}
	if record.TerminalGrowth > 0 {
		out.TerminalGrowth = record.TerminalGrowth
	}
	if record.FCFGrowth > 0 {
		out.OwnerEarningsGrowth = record.FCFGrowth
	}
	if record.TaxRate > 0 {
		out.TaxRate = record.TaxRate
	}
	if out.DiscountRate <= out.TerminalGrowth {
		out.DiscountRate = out.TerminalGrowth + 0.08
	}
	return out
}

func completedYears(fin financialFile, latest period, limit int) []int {
	set := map[int]bool{}
	for _, field := range fin.Data {
		for yearText, values := range field.Years {
			year, err := strconv.Atoi(yearText)
			if err != nil || year <= 0 {
				continue
			}
			if year == latest.Year && latest.Quarter < 4 {
				continue
			}
			if len(values) > 0 && values[0] != nil {
				set[year] = true
			}
		}
	}
	years := make([]int, 0, len(set))
	for year := range set {
		years = append(years, year)
	}
	sort.Ints(years)
	if limit > 0 && len(years) > limit {
		years = years[len(years)-limit:]
	}
	return years
}

func fieldValue(fin financialFile, code string, p period) float64 {
	value, _ := fieldValueOK(fin, code, p)
	return value
}

func firstFieldValue(fin financialFile, p period, codes ...string) float64 {
	for _, code := range codes {
		if value, ok := fieldValueOK(fin, code, p); ok {
			return value
		}
	}
	return 0
}

func fieldValueOK(fin financialFile, code string, p period) (float64, bool) {
	field, ok := fin.Data[code]
	if !ok || p.Year == 0 || p.Quarter == 0 {
		return 0, false
	}
	values := field.Years[strconv.Itoa(p.Year)]
	idx := indexFromQuarter(p.Quarter)
	if idx < 0 || idx >= len(values) || values[idx] == nil {
		return 0, false
	}
	return *values[idx], true
}

func ttm(fin financialFile, code string, p period) (float64, bool) {
	latestYTD, ok := fieldValueOK(fin, code, p)
	if !ok {
		return 0, false
	}
	if p.Quarter == 4 {
		return latestYTD, true
	}
	prevFY, ok := fieldValueOK(fin, code, period{Year: p.Year - 1, Quarter: 4})
	if !ok {
		return 0, false
	}
	prevSameQuarter, ok := fieldValueOK(fin, code, period{Year: p.Year - 1, Quarter: p.Quarter})
	if !ok {
		return 0, false
	}
	return latestYTD + prevFY - prevSameQuarter, true
}

func latestNetDebt(fin financialFile, latest period) float64 {
	if isBankFinancialSchema(fin) {
		return 0
	}
	return fieldValue(fin, "2AA", latest) + fieldValue(fin, "2BA", latest) + fieldValue(fin, "2BB", latest) - fieldValue(fin, "1AA", latest)
}

func isBankFinancialSchema(fin financialFile) bool {
	text := normalizeText(fin.FinancialGroup)
	if strings.Contains(text, "ufrs_k") {
		return true
	}
	if strings.Contains(text, "bank") && hasFinancialFields(fin, "1Z", "2O", "3Z") {
		return true
	}
	return hasFinancialFields(fin, "1Z", "2O", "3Z", "3CE") && !hasFinancialFields(fin, "1BL", "2N")
}

func hasFinancialFields(fin financialFile, codes ...string) bool {
	for _, code := range codes {
		if _, ok := fin.Data[code]; !ok {
			return false
		}
	}
	return true
}

func estimateMaintenanceCapex(cfo, capex, depreciation float64) float64 {
	absCapex := math.Abs(capex)
	if absCapex == 0 {
		return 0
	}
	if depreciation > 0 {
		return math.Min(absCapex, math.Max(depreciation, absCapex*0.35))
	}
	return absCapex * 0.65
}

func perShareDCF(owner, netDebt, shares, growth, discount, terminalGrowth float64) float64 {
	if owner <= 0 || shares <= 0 {
		return 0
	}
	if discount <= terminalGrowth {
		return 0
	}
	growth = math.Max(growth, -0.05)
	cashFlow := owner
	value := 0.0
	for year := 1; year <= 10; year++ {
		cashFlow *= 1 + growth
		value += cashFlow / math.Pow(1+discount, float64(year))
	}
	terminal := cashFlow * (1 + terminalGrowth) / (discount - terminalGrowth)
	value += terminal / math.Pow(1+discount, 10)
	return mathutil.SafeDiv(value-netDebt, shares)
}

type scoreWeight struct {
	score  float64
	weight float64
}

func weightedScore(values []scoreWeight) float64 {
	total := 0.0
	weight := 0.0
	for _, item := range values {
		total += item.score * item.weight
		weight += item.weight
	}
	return mathutil.Clamp(mathutil.SafeDiv(total, weight), 0, 100)
}

func mapYearValues(years []YearMetric, fn func(YearMetric) float64) []float64 {
	out := make([]float64, 0, len(years))
	for _, year := range years {
		value := fn(year)
		if !math.IsNaN(value) && !math.IsInf(value, 0) {
			out = append(out, value)
		}
	}
	return out
}

func sumYearValues(years []YearMetric, fn func(YearMetric) float64) float64 {
	sum := 0.0
	for _, y := range years {
		sum += fn(y)
	}
	return sum
}

func lastN[T any](values []T, n int) []T {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func mean(values []float64) float64 {
	return mathutil.Mean(values)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func stabilityScore(values []float64) float64 {
	filtered := []float64{}
	for _, value := range values {
		if value != 0 {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) < 2 {
		return 0
	}
	avg := math.Abs(mean(filtered))
	if avg == 0 {
		return 0
	}
	cv := math.Abs(mathutil.StdDev(filtered) / avg)
	return mathutil.Clamp(1-cv, 0, 1)
}

func cagrFromValues(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	first := values[0]
	last := values[len(values)-1]
	if first <= 0 || last <= 0 {
		return 0
	}
	return math.Pow(last/first, 1/float64(len(values)-1)) - 1
}

func scorePositive(value float64) float64 {
	if value > 0 {
		return 1
	}
	return 0
}

func scoreCAGR(value float64) float64 {
	return mathutil.Clamp((value+0.05)/0.20, 0, 1)
}

type threshold struct {
	min   float64
	score float64
}

func thresholdScore(value float64, thresholds []threshold) float64 {
	for _, threshold := range thresholds {
		if value >= threshold.min {
			return threshold.score
		}
	}
	return 0
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositivePaidCapital(years []YearMetric) float64 {
	for _, year := range years {
		if year.PaidCapital > 0 {
			return year.PaidCapital
		}
	}
	return 0
}

func pctChange(oldValue, newValue float64) float64 {
	if oldValue <= 0 {
		return 0
	}
	return 100 * (newValue - oldValue) / oldValue
}

func quarterFromIndex(index int) int {
	switch index {
	case 0:
		return 4
	case 1:
		return 3
	case 2:
		return 2
	case 3:
		return 1
	default:
		return 0
	}
}

func indexFromQuarter(quarter int) int {
	switch quarter {
	case 4:
		return 0
	case 3:
		return 1
	case 2:
		return 2
	case 1:
		return 3
	default:
		return -1
	}
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("ı", "i", "ğ", "g", "ü", "u", "ş", "s", "ö", "o", "ç", "c", "İ", "i")
	return replacer.Replace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}
