package professional

import (
	"fmt"
	"math"
	"strings"

	"hissebot/internal/util"
	"hissebot/pkg/mathutil"
)

type SectorFinancialAnalysis struct {
	Applicable       bool                    `json:"applicable"`
	Profile          string                  `json:"profile"`
	ProfileLabel     string                  `json:"profile_label"`
	MainSector       string                  `json:"main_sector,omitempty"`
	Sector           string                  `json:"sector,omitempty"`
	FinancialGroup   string                  `json:"financial_group,omitempty"`
	LatestYear       int                     `json:"latest_year,omitempty"`
	LatestQuarter    string                  `json:"latest_quarter,omitempty"`
	Score            float64                 `json:"score"`
	Summary          string                  `json:"summary"`
	Metrics          []SectorFinancialMetric `json:"metrics,omitempty"`
	Strengths        []string                `json:"strengths,omitempty"`
	Risks            []string                `json:"risks,omitempty"`
	Warnings         []string                `json:"warnings,omitempty"`
	SuppressedMetric []string                `json:"suppressed_metrics,omitempty"`
	Focus            []string                `json:"focus,omitempty"`
	FieldSchema      string                  `json:"field_schema,omitempty"`
	HistoricalRatios *HistoricalRatioSummary `json:"historical_ratios,omitempty"`
}

// HistoricalRatioSummary holds trend analysis for pre-calculated financial ratios
// sourced from bilanco_hesaplari.json.
type HistoricalRatioSummary struct {
	PeriodsAvailable int                  `json:"periods_available"`
	YearsAvailable   []string             `json:"years_available"`
	Ratios           []HistoricalRatioRow `json:"ratios"`
	TrendSignals     []string             `json:"trend_signals,omitempty"`
}

// HistoricalRatioRow represents one financial ratio across time.
type HistoricalRatioRow struct {
	ID           string             `json:"id"`
	Label        string             `json:"label"`
	Unit         string             `json:"unit,omitempty"`
	LatestValue  *float64           `json:"latest_value"`
	Avg3Y        *float64           `json:"avg_3y,omitempty"`
	Avg5Y        *float64           `json:"avg_5y,omitempty"`
	Trend        string             `json:"trend"`
	TrendScore   float64            `json:"trend_score"`
	YearlyValues map[string]float64 `json:"yearly_values,omitempty"`
}

type SectorFinancialMetric struct {
	Name           string   `json:"name"`
	Label          string   `json:"label"`
	Value          float64  `json:"value"`
	Unit           string   `json:"unit,omitempty"`
	Status         string   `json:"status"`
	Interpretation string   `json:"interpretation"`
	SourceFields   []string `json:"source_fields,omitempty"`
}

type sectorFinancialProfile struct {
	ID         string
	Label      string
	Model      string
	Metrics    []string
	Suppressed []string
	Focus      []string
	Warnings   []string
}

type financialFieldSchema struct {
	ID       string
	Fields   map[string][]string
	Warnings []string
}

const (
	fieldTotalAssets       = "total_assets"
	fieldEquity            = "equity"
	fieldPaidCapital       = "paid_capital"
	fieldCash              = "cash"
	fieldShortTermLiab     = "short_term_liabilities"
	fieldCurrentAssets     = "current_assets"
	fieldInventory         = "inventory"
	fieldReceivables       = "receivables"
	fieldRevenue           = "revenue"
	fieldCOGS              = "cogs"
	fieldGrossProfit       = "gross_profit"
	fieldEBIT              = "ebit"
	fieldAmortization      = "amortization"
	fieldNetIncome         = "net_income"
	fieldOperatingCash       = "operating_cash"
	fieldFreeCashFlow        = "free_cash_flow"
	fieldDeferredTaxIncome   = "deferred_tax_income"
	fieldDebt                = "debt"
	fieldLoans             = "loans"
	fieldDeposits          = "deposits"
	fieldTechnicalBalance  = "technical_balance"
	fieldNetInterestIncome = "net_interest_income"
)

func financialFieldSchemaForContext(context string, fin financialFile) financialFieldSchema {
	slug := util.SlugTR(context)
	switch {
	case strings.Contains(slug, "bank"):
		return financialFieldSchema{
			ID: "bank_ufrs_k",
			Fields: map[string][]string{
				fieldTotalAssets:       {"1Z"},
				fieldEquity:            {"2O"},
				fieldPaidCapital:       {"2OA"},
				fieldCash:              {"1A"},
				fieldRevenue:           {"3C"},
				fieldNetInterestIncome: {"3C"},
				fieldNetIncome:         {"3Z"},
				fieldLoans:             {"1AF"},
				fieldDeposits:          {"2A"},
			},
			Warnings: []string{"bank_field_schema_uses_ufrs_k_roles"},
		}
	case strings.Contains(slug, "sigorta"):
		return financialFieldSchema{
			ID: "insurance_ufrs",
			Fields: map[string][]string{
				fieldTotalAssets:      {"1Z"},
				fieldEquity:           {"2O"},
				fieldPaidCapital:      {"2MEB", "2MEA"},
				fieldCash:             {"1A"},
				fieldRevenue:          {"3C"},
				fieldTechnicalBalance: {"3C"},
				fieldNetIncome:        {"3Z", "3NJA", "3NJD"},
			},
			Warnings: []string{"insurance_field_schema_uses_ufrs_roles"},
		}
	default:
		return financialFieldSchema{
			ID: "industrial_xi_29",
			Fields: map[string][]string{
				fieldTotalAssets:   {"1BL"},
				fieldEquity:        {"2N", "2O"},
				fieldPaidCapital:   {"2OA"},
				fieldCash:          {"1AA"},
				fieldDebt:          {"2AA", "2BA", "2BB"},
				fieldCurrentAssets: {"1A"},
				fieldShortTermLiab: {"2A"},
				fieldInventory:     {"1AF"},
				fieldReceivables:   {"1AC"},
				fieldRevenue:       {"3C"},
				fieldCOGS:          {"3CA"},
				fieldGrossProfit:   {"3D"},
				fieldEBIT:          {"3DF"},
				fieldAmortization:  {"4B"},
				fieldNetIncome:         {"3L"},
				fieldOperatingCash:     {"4C"},
				fieldFreeCashFlow:      {"4CB"},
				fieldDeferredTaxIncome: {"3IC"},
			},
		}
	}
}

func schemaFieldValue(fin financialFile, schema financialFieldSchema, role string, p period) (float64, string, bool) {
	for _, code := range schema.Fields[role] {
		if !fieldRoleMatches(fin, code, role, schema.ID) {
			continue
		}
		value, ok := fieldValueOK(fin, code, p)
		if ok {
			return value, code, true
		}
	}
	return 0, "", false
}

func schemaFieldValueOrZero(fin financialFile, schema financialFieldSchema, role string, p period) float64 {
	value, _, _ := schemaFieldValue(fin, schema, role, p)
	return value
}

func schemaFieldCodes(fin financialFile, schema financialFieldSchema, roles ...string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, role := range roles {
		for _, code := range schema.Fields[role] {
			if seen[code] || !fieldRoleMatches(fin, code, role, schema.ID) {
				continue
			}
			seen[code] = true
			out = append(out, code)
		}
	}
	return out
}

func schemaSumFieldValues(fin financialFile, schema financialFieldSchema, role string, p period) (float64, []string, bool) {
	total := 0.0
	codes := []string{}
	for _, code := range schema.Fields[role] {
		if !fieldRoleMatches(fin, code, role, schema.ID) {
			continue
		}
		value, ok := fieldValueOK(fin, code, p)
		if !ok {
			continue
		}
		total += value
		codes = append(codes, code)
	}
	return total, codes, len(codes) > 0
}

func schemaTTM(fin financialFile, schema financialFieldSchema, role string, p period) (float64, string, bool) {
	for _, code := range schema.Fields[role] {
		if !fieldRoleMatches(fin, code, role, schema.ID) {
			continue
		}
		value, ok := ttm(fin, code, p)
		if ok {
			return value, code, true
		}
	}
	return 0, "", false
}

func fieldRoleMatches(fin financialFile, code string, role string, schemaID string) bool {
	field, ok := fin.Data[code]
	if !ok {
		return false
	}
	desc := util.SlugTR(field.DescTR + " " + field.DescEN)
	if desc == "" {
		return true
	}
	switch role {
	case fieldTotalAssets:
		return strings.Contains(desc, "toplamvarlik") || strings.Contains(desc, "aktiftoplami") || strings.Contains(desc, "toplamaktif")
	case fieldEquity:
		return strings.Contains(desc, "ozkaynak") || strings.Contains(desc, "ozsermaye")
	case fieldPaidCapital:
		return strings.Contains(desc, "odenmissermaye") || strings.Contains(desc, "nominalsermaye")
	case fieldNetIncome:
		return strings.Contains(desc, "donemkar") || strings.Contains(desc, "netdonemkar") || strings.Contains(desc, "karzarar")
	case fieldRevenue:
		if strings.Contains(schemaID, "bank") {
			return strings.Contains(desc, "netfaizgeliri") || strings.Contains(desc, "netfaiz")
		}
		if strings.Contains(schemaID, "insurance") {
			return strings.Contains(desc, "teknikbolumdengesi") || strings.Contains(desc, "teknik")
		}
		return strings.Contains(desc, "satis") || strings.Contains(desc, "hasilat") || strings.Contains(desc, "gelir")
	case fieldCash:
		return strings.Contains(desc, "nakit") || strings.Contains(desc, "merkezbankasi")
	case fieldDebt:
		return strings.Contains(desc, "finansalborc") || strings.Contains(desc, "kredi")
	case fieldLoans:
		return strings.Contains(desc, "kredi")
	case fieldDeposits:
		return strings.Contains(desc, "mevduat")
	case fieldTechnicalBalance:
		return strings.Contains(desc, "teknik")
	case fieldNetInterestIncome:
		return strings.Contains(desc, "netfaizgeliri") || strings.Contains(desc, "netfaiz")
	case fieldCurrentAssets:
		return strings.Contains(desc, "donenvarlik")
	case fieldShortTermLiab:
		return strings.Contains(desc, "kisavadeliyukumluluk")
	case fieldInventory:
		return strings.Contains(desc, "stok")
	case fieldReceivables:
		return strings.Contains(desc, "ticarialacak") || strings.Contains(desc, "alacak")
	case fieldCOGS:
		return strings.Contains(desc, "satislarinmaliyeti") || strings.Contains(desc, "maliyet")
	case fieldGrossProfit:
		return strings.Contains(desc, "brutkar")
	case fieldEBIT:
		return strings.Contains(desc, "faaliyetkari")
	case fieldAmortization:
		return strings.Contains(desc, "amortisman")
	case fieldOperatingCash:
		return strings.Contains(desc, "isletmefaaliyetlerindenkaynaklanannakit") || strings.Contains(desc, "isletmefaaliyetlerindenkaynaklanannetnakit")
	case fieldFreeCashFlow:
		return strings.Contains(desc, "serbestnakitakim")
	case fieldDeferredTaxIncome:
		return strings.Contains(desc, "ertelenmisvergigeliri") || strings.Contains(desc, "ertelenmisvergi")
	default:
		return true
	}
}

var sectorFinancialProfiles = map[string]sectorFinancialProfile{
	"bank": {
		ID:         "bank",
		Label:      "Banka",
		Model:      "bank_balance_sheet",
		Metrics:    []string{"roe", "roa", "equity_to_assets", "assets_to_equity", "loan_to_deposit_proxy", "net_interest_income_to_assets", "pb", "pe", "book_per_share"},
		Suppressed: []string{"current_ratio", "quick_ratio", "inventory_turnover", "ev_ebitda", "fcf_yield", "net_debt_to_equity"},
		Focus:      []string{"ozkaynak karliligi", "aktif kalitesi proxy", "mevduat-kredi dengesi", "defter degeri carpanlari"},
		Warnings:   []string{"bank_reports_need_npl_capital_adequacy_and_regulatory_ratios_for_full_view"},
	},
	"insurance": {
		ID:         "insurance",
		Label:      "Sigorta",
		Model:      "insurance_balance_sheet",
		Metrics:    []string{"roe", "roa", "equity_to_assets", "technical_balance_to_assets", "pb", "pe", "book_per_share"},
		Suppressed: []string{"current_ratio", "quick_ratio", "inventory_turnover", "ev_ebitda", "fcf_yield", "net_debt_to_equity"},
		Focus:      []string{"ozkaynak yeterliligi", "teknik sonuc proxy", "defter degeri", "karlilik"},
		Warnings:   []string{"insurance_reports_need_premium_reserve_claim_and_solvency_ratios_for_full_view"},
	},
	"leasing_factoring_finance": {
		ID:         "leasing_factoring_finance",
		Label:      "Finansal kiralama, faktoring ve finansman",
		Model:      "financial_book_value",
		Metrics:    []string{"roe", "roa", "equity_to_assets", "assets_to_equity", "pb", "pe", "book_per_share"},
		Suppressed: []string{"inventory_turnover", "ev_ebitda", "fcf_yield"},
		Focus:      []string{"kaldirac", "ozkaynak karliligi", "defter degeri", "aktif verimliligi"},
		Warnings:   []string{"financial_services_need_asset_quality_funding_cost_and_npl_ratios_for_full_view"},
	},
	"brokerage_asset_management": {
		ID:         "brokerage_asset_management",
		Label:      "Araci kurum / varlik yonetimi",
		Model:      "capital_markets_financial",
		Metrics:    []string{"roe", "roa", "equity_to_assets", "net_margin", "pb", "pe", "book_per_share"},
		Suppressed: []string{"inventory_turnover", "current_ratio", "ev_ebitda"},
		Focus:      []string{"komisyon ve faaliyet karliligi", "ozkaynak karliligi", "sermaye tamponu"},
	},
	"reit_nav": {
		ID:         "reit_nav",
		Label:      "Gayrimenkul yatirim ortakligi",
		Model:      "reit_nav_proxy",
		Metrics:    []string{"pb", "book_per_share", "net_debt_to_equity", "equity_to_assets", "roe", "roa", "current_ratio"},
		Suppressed: []string{"inventory_turnover", "ev_ebitda", "fcf_yield"},
		Focus:      []string{"net aktif deger proxy", "kaldirac", "defter degeri", "portfoy finansmani"},
		Warnings:   []string{"reit_reports_need_appraisal_based_nav_and_portfolio_occupancy_for_full_view"},
	},
	"investment_trust": {
		ID:         "investment_trust",
		Label:      "Yatirim ortakligi / girisim sermayesi",
		Model:      "investment_trust_nav_proxy",
		Metrics:    []string{"pb", "book_per_share", "equity_to_assets", "roe", "roa", "net_debt_to_equity"},
		Suppressed: []string{"inventory_turnover", "current_ratio", "ev_ebitda", "fcf_yield"},
		Focus:      []string{"portfoy defter degeri", "ozkaynak", "kaldirac", "NAV iskontosu proxy"},
		Warnings:   []string{"investment_trust_reports_need_portfolio_holdings_and_nav_for_full_view"},
	},
	"holding_sotp": {
		ID:         "holding_sotp",
		Label:      "Holding ve yatirim sirketi",
		Model:      "holding_sum_of_parts_proxy",
		Metrics:    []string{"pb", "book_per_share", "net_debt_to_equity", "equity_to_assets", "roe", "net_margin"},
		Suppressed: []string{"inventory_turnover", "receivable_turnover", "ev_ebitda"},
		Focus:      []string{"holding iskontosu proxy", "net borc", "ozkaynak kalitesi", "portfoy karliligi"},
		Warnings:   []string{"holding_reports_need_sum_of_parts_subsidiary_valuation_for_full_view"},
	},
	"utility_infrastructure": {
		ID:      "utility_infrastructure",
		Label:   "Elektrik, gaz, su ve altyapi",
		Model:   "regulated_infrastructure",
		Metrics: []string{"ebitda_margin", "net_debt_to_ebitda", "net_debt_to_equity", "equity_to_assets", "fcf_margin", "roe", "roa"},
		Focus:   []string{"borc servis kapasitesi", "FAVOK marji", "yatirim harcamasi baskisi", "nakit akisi"},
	},
	"telecom_media": {
		ID:      "telecom_media",
		Label:   "Telekom, yayin ve medya",
		Model:   "subscription_or_media_operating",
		Metrics: []string{"ebitda_margin", "net_debt_to_ebitda", "net_debt_to_equity", "fcf_margin", "roe", "roa", "asset_turnover"},
		Focus:   []string{"abone/icerik yatirimi proxy", "FAVOK", "nakit donusumu", "kaldirac"},
	},
	"technology": {
		ID:      "technology",
		Label:   "Teknoloji ve bilisim",
		Model:   "technology_operating",
		Metrics: []string{"gross_margin", "ebit_margin", "net_margin", "fcf_conversion", "cash_ratio", "net_debt_to_equity", "roe", "roa"},
		Focus:   []string{"brut marj", "nakit donusumu", "net nakit/borc", "olceklenebilir karlilik"},
	},
	"defense_industrial": {
		ID:      "defense_industrial",
		Label:   "Savunma ve proje bazli sanayi",
		Model:   "project_industrial",
		Metrics: []string{"current_ratio", "quick_ratio", "receivable_turnover", "inventory_turnover", "gross_margin", "ebit_margin", "fcf_income_quality", "deferred_tax_quality", "net_debt_to_equity", "roe"},
		Focus:   []string{"isletme sermayesi", "stok/alacak donusu", "proje nakit donusumu (FCF kalitesi)", "kaldirac", "ertelenmis vergi muhasebe etkisi"},
	},
	"manufacturing_general": {
		ID:      "manufacturing_general",
		Label:   "Imalat sanayi",
		Model:   "industrial_operating",
		Metrics: []string{"current_ratio", "quick_ratio", "gross_margin", "ebit_margin", "net_margin", "inventory_turnover", "receivable_turnover", "fcf_conversion", "net_debt_to_equity", "roe", "roa"},
		Focus:   []string{"marj kalitesi", "isletme sermayesi", "stok/alacak devri", "nakit donusumu"},
	},
	"materials_commodity": {
		ID:      "materials_commodity",
		Label:   "Emtia, maden ve temel malzemeler",
		Model:   "commodity_cyclical",
		Metrics: []string{"ebitda_margin", "gross_margin", "fcf_margin", "net_debt_to_ebitda", "net_debt_to_equity", "current_ratio", "roe", "roa"},
		Focus:   []string{"dongusel marj", "FAVOK", "borc dongusu", "nakit akisi"},
	},
	"oil_gas": {
		ID:      "oil_gas",
		Label:   "Petrol, gaz ve enerji kaynaklari",
		Model:   "energy_resource",
		Metrics: []string{"ebitda_margin", "fcf_margin", "net_debt_to_ebitda", "net_debt_to_equity", "equity_to_assets", "roe", "roa"},
		Focus:   []string{"emtia fiyat hassasiyeti", "nakit akisi", "borc servis kapasitesi"},
	},
	"retail_trade": {
		ID:      "retail_trade",
		Label:   "Perakende ticaret",
		Model:   "retail_working_capital",
		Metrics: []string{"gross_margin", "ebit_margin", "inventory_turnover", "current_ratio", "cash_ratio", "fcf_conversion", "net_debt_to_equity", "roe"},
		Focus:   []string{"brut marj", "stok devri", "magaza/operasyon nakdi", "kisa vadeli likidite"},
	},
	"wholesale_trade": {
		ID:      "wholesale_trade",
		Label:   "Toptan ticaret",
		Model:   "wholesale_working_capital",
		Metrics: []string{"gross_margin", "ebit_margin", "receivable_turnover", "inventory_turnover", "current_ratio", "fcf_conversion", "net_debt_to_equity", "roe"},
		Focus:   []string{"alacak tahsilati", "stok devri", "dusuk marj disiplini", "nakit donusumu"},
	},
	"transport_logistics": {
		ID:      "transport_logistics",
		Label:   "Ulastirma ve depolama",
		Model:   "transport_asset_heavy",
		Metrics: []string{"ebitda_margin", "net_debt_to_ebitda", "net_debt_to_equity", "asset_turnover", "fcf_margin", "roe", "roa"},
		Focus:   []string{"varlik verimliligi", "FAVOK", "filo/altyapi borcu", "nakit akisi"},
	},
	"construction_contracting": {
		ID:      "construction_contracting",
		Label:   "Insaat ve taahhut",
		Model:   "contracting_working_capital",
		Metrics: []string{"current_ratio", "quick_ratio", "receivable_turnover", "gross_margin", "ebit_margin", "fcf_conversion", "net_debt_to_equity", "equity_to_assets"},
		Focus:   []string{"hak edis/alacak tahsilati", "proje marji", "nakit donusumu", "kaldirac"},
	},
	"real_estate_operations": {
		ID:      "real_estate_operations",
		Label:   "Gayrimenkul faaliyetleri",
		Model:   "real_estate_operations",
		Metrics: []string{"pb", "book_per_share", "net_debt_to_equity", "equity_to_assets", "ebitda_margin", "roe", "roa"},
		Focus:   []string{"varlik degeri", "kaldirac", "kira/satis karliligi", "defter degeri"},
	},
	"healthcare_services": {
		ID:      "healthcare_services",
		Label:   "Saglik hizmetleri",
		Model:   "healthcare_services",
		Metrics: []string{"ebitda_margin", "ebit_margin", "net_margin", "current_ratio", "fcf_conversion", "net_debt_to_equity", "roe", "roa"},
		Focus:   []string{"operasyonel marj", "nakit donusumu", "borc", "varlik verimliligi"},
	},
	"consumer_services": {
		ID:      "consumer_services",
		Label:   "Tuketici hizmetleri, spor ve eglence",
		Model:   "consumer_services",
		Metrics: []string{"gross_margin", "ebitda_margin", "ebit_margin", "net_margin", "current_ratio", "fcf_conversion", "net_debt_to_equity", "roe"},
		Focus:   []string{"talep hassasiyeti", "operasyonel marj", "nakit donusumu", "kaldirac"},
	},
	"hospitality": {
		ID:      "hospitality",
		Label:   "Otel, konaklama ve yiyecek icecek",
		Model:   "hospitality_operating",
		Metrics: []string{"ebitda_margin", "gross_margin", "current_ratio", "net_debt_to_ebitda", "net_debt_to_equity", "fcf_margin", "roe"},
		Focus:   []string{"doluluk/talep proxy", "FAVOK", "borc servis kapasitesi", "sezonsallik"},
	},
	"agriculture_primary": {
		ID:      "agriculture_primary",
		Label:   "Tarim, hayvancilik ve balikcilik",
		Model:   "agriculture_primary",
		Metrics: []string{"gross_margin", "inventory_turnover", "current_ratio", "fcf_conversion", "net_debt_to_equity", "roe", "roa"},
		Focus:   []string{"stok/biolojik varlik proxy", "brut marj", "nakit donusumu", "borc"},
	},
	"professional_services": {
		ID:      "professional_services",
		Label:   "Mesleki ve teknik hizmetler",
		Model:   "asset_light_services",
		Metrics: []string{"gross_margin", "ebit_margin", "net_margin", "receivable_turnover", "current_ratio", "fcf_conversion", "roe", "roa"},
		Focus:   []string{"alacak tahsilati", "insan sermayesi marji", "nakit donusumu"},
	},
	"admin_support_services": {
		ID:      "admin_support_services",
		Label:   "Idari destek ve kiralama hizmetleri",
		Model:   "support_services",
		Metrics: []string{"ebitda_margin", "current_ratio", "receivable_turnover", "net_debt_to_equity", "fcf_conversion", "roe", "roa"},
		Focus:   []string{"sozlesme nakdi", "alacak devri", "borc", "operasyonel marj"},
	},
	"information_services": {
		ID:      "information_services",
		Label:   "Bilgi hizmetleri",
		Model:   "information_services",
		Metrics: []string{"gross_margin", "ebit_margin", "net_margin", "cash_ratio", "fcf_conversion", "roe", "roa"},
		Focus:   []string{"olceklenebilir marj", "nakit donusumu", "net nakit/borc"},
	},
}

var kapSectorFinancialProfileBySector = map[string]string{
	"bilgihizmetfaaliyetleri": "information_services",
	"telekomunikasyon":        "telecom_media",
	"yayimcilik":              "telecom_media",
	"elektrikgazvebuhar":      "utility_infrastructure",
	"sporeglenceboszamanlaridegerlendirmehizmetleri":         "consumer_services",
	"sporfaaliyetlerieglenceveoyunfaaliyetleri":              "consumer_services",
	"yaraticisanatlargosterisanatlariveeglencefaaliyetleri":  "consumer_services",
	"insansagligivesosyalhizmetler":                          "healthcare_services",
	"gayrimenkulfaaliyetleri":                                "real_estate_operations",
	"digermadencilikvetasocakciligi":                         "materials_commodity",
	"hampetrolvedogalgazcikartilmasi":                        "oil_gas",
	"komurvelinyitmadenciligi":                               "materials_commodity",
	"metalcevherimadenciligi":                                "materials_commodity",
	"aracikurumlar":                                          "brokerage_asset_management",
	"bankalar":                                               "bank",
	"finansalkiralamavefaktoringsirketleri":                  "leasing_factoring_finance",
	"finansmansirketleri":                                    "leasing_factoring_finance",
	"gayrimenkulyatirimortakliklari":                         "reit_nav",
	"girisimsermayesiyatirimortakliklari":                    "investment_trust",
	"holdinglerveyatirimsirketleri":                          "holding_sotp",
	"menkulkiymetyatirimortakliklari":                        "investment_trust",
	"sigortasirketleri":                                      "insurance",
	"varlikyonetimsirketleri":                                "brokerage_asset_management",
	"hukukvemuhasebefaaliyetleri":                            "professional_services",
	"mimarlikvemuhendislikfaaliyetleriteknikmuayeneveanaliz": "professional_services",
	"reklamcilikvepazararastirmasi":                          "professional_services",
	"konaklama":                                              "hospitality",
	"yiyecekveicecekhizmetleri":                              "hospitality",
	"balikcilikvesuurunleri":                                 "agriculture_primary",
	"tarimvehayvancilikavcilikveilgilihizmetfaaliyetleri":    "agriculture_primary",
	"bilisim":             "technology",
	"savunma":             "defense_industrial",
	"perakendeticaret":    "retail_trade",
	"toptanticaret":       "wholesale_trade",
	"ulastirmavedepolama": "transport_logistics",
	"buroyonetimiburodestegivedigersirketdestekfaaliyetleri":                      "admin_support_services",
	"kiralamaveleasingfaaliyetleri":                                               "admin_support_services",
	"seyahatacentesituroperatoruvedigerrezervasyonhizmetleriileilgilifaaliyetler": "consumer_services",
	"anametalsanayi":                                    "materials_commodity",
	"digerimalatsanayii":                                "manufacturing_general",
	"gid aicecekvetutun":                                "manufacturing_general",
	"gidaicecekvetutun":                                 "manufacturing_general",
	"kagitvekagiturunleribasim":                         "manufacturing_general",
	"kimyailacpetrollastikveplastikurunler":             "materials_commodity",
	"metalesyamakineelektriklicihazlarveulasimaraclari": "manufacturing_general",
	"ormanurunlerivemobilya":                            "manufacturing_general",
	"tasvetopragadayali":                                "materials_commodity",
	"tekstilgiyimesyasivederi":                          "manufacturing_general",
	"insaatvebayindirlikisleri":                         "construction_contracting",
}

func analyzeSectorFinancials(fin financialFile, latest period, profile CompanyProfile, valuation ValuationAnalysis) SectorFinancialAnalysis {
	return analyzeSectorFinancialsWithHistory(fin, latest, profile, valuation, nil)
}

func analyzeSectorFinancialsWithHistory(fin financialFile, latest period, profile CompanyProfile, valuation ValuationAnalysis, history financialRatioHistory) SectorFinancialAnalysis {
	spec := sectorFinancialProfileFor(profile, fin.FinancialGroup)
	schema := financialFieldSchemaForContext(financialContextText(profile, fin.FinancialGroup), fin)
	out := SectorFinancialAnalysis{
		Applicable:       true,
		Profile:          spec.ID,
		ProfileLabel:     spec.Label,
		MainSector:       profile.Sector,
		Sector:           profile.Industry,
		FinancialGroup:   fin.FinancialGroup,
		LatestYear:       latest.Year,
		LatestQuarter:    quarterName(latest.Quarter),
		Focus:            append([]string{}, spec.Focus...),
		SuppressedMetric: append([]string{}, spec.Suppressed...),
		Warnings:         append(append([]string{}, spec.Warnings...), schema.Warnings...),
		FieldSchema:      schema.ID,
	}
	if latest.Year == 0 || latest.Quarter == 0 {
		out.Warnings = append(out.Warnings, "financial_latest_period_missing")
		out.Summary = "Sektor bazli bilanço yorumu icin finansal donem bulunamadi."
		return out
	}
	seen := map[string]bool{}
	for _, metricID := range spec.Metrics {
		if seen[metricID] {
			continue
		}
		seen[metricID] = true
		metric, ok := sectorMetric(metricID, fin, schema, latest, valuation)
		if !ok {
			out.Warnings = append(out.Warnings, "metric_missing_"+metricID)
			continue
		}
		out.Metrics = append(out.Metrics, metric)
		switch metric.Status {
		case "strong":
			out.Strengths = append(out.Strengths, metric.Interpretation)
		case "weak", "critical":
			out.Risks = append(out.Risks, metric.Interpretation)
		}
	}
	out.Score = sectorFinancialScore(out.Metrics)
	if len(history) > 0 {
		out.HistoricalRatios = buildHistoricalRatioSummary(history, latest)
		if out.HistoricalRatios != nil {
			out.Warnings = append(out.Warnings[:0:0], out.Warnings...)
		}
	}
	out.Summary = sectorFinancialSummary(out)
	return out
}

func sectorFinancialNotApplicable(assetType string) SectorFinancialAnalysis {
	summary := "Bu varlik tipi icin sirket bilancosu ve geleneksel finansal oran yorumu uygulanmaz."
	if strings.EqualFold(assetType, "crypto") {
		summary = "Kripto varliklar icin sirket bilancosu ve geleneksel finansal oran yorumu uygulanmaz."
	} else if strings.EqualFold(assetType, "commodity") {
		summary = "Emtia/altin varliklari icin sirket bilancosu ve geleneksel finansal oran yorumu uygulanmaz."
	}
	return SectorFinancialAnalysis{
		Applicable: false,
		Profile:    "not_applicable",
		Summary:    summary,
		Warnings:   []string{"issuer_financial_statement_interpretation_not_applicable_to_" + assetType},
	}
}

func sectorFinancialProfileFor(profile CompanyProfile, financialGroup string) sectorFinancialProfile {
	key := util.SlugTR(profile.Industry)
	if profileID := kapSectorFinancialProfileBySector[key]; profileID != "" {
		return sectorFinancialProfiles[profileID]
	}
	context := financialContextText(profile, financialGroup)
	slug := util.SlugTR(context)
	switch {
	case strings.Contains(slug, "bank"):
		return sectorFinancialProfiles["bank"]
	case strings.Contains(slug, "sigorta"):
		return sectorFinancialProfiles["insurance"]
	case strings.Contains(slug, "gayrimenkulyatirimortak"):
		return sectorFinancialProfiles["reit_nav"]
	case strings.Contains(slug, "holding"):
		return sectorFinancialProfiles["holding_sotp"]
	case strings.Contains(slug, "finansalkiralama") || strings.Contains(slug, "faktoring") || strings.Contains(slug, "finansman"):
		return sectorFinancialProfiles["leasing_factoring_finance"]
	case strings.Contains(slug, "aracikurum") || strings.Contains(slug, "varlikyonetim"):
		return sectorFinancialProfiles["brokerage_asset_management"]
	case strings.Contains(slug, "teknoloji") || strings.Contains(slug, "bilisim"):
		return sectorFinancialProfiles["technology"]
	case strings.Contains(slug, "elektrik") || strings.Contains(slug, "gaz"):
		return sectorFinancialProfiles["utility_infrastructure"]
	case strings.Contains(slug, "imalat"):
		return sectorFinancialProfiles["manufacturing_general"]
	default:
		spec := sectorFinancialProfiles["manufacturing_general"]
		spec.ID = "generic_operating_company"
		spec.Label = "Genel faaliyet sirketi"
		spec.Warnings = append(append([]string{}, spec.Warnings...), "sector_financial_profile_fallback_used")
		return spec
	}
}

func financialContextText(profile CompanyProfile, financialGroup string) string {
	return strings.TrimSpace(strings.Join([]string{profile.Sector, profile.Industry, profile.PeerGroup, financialGroup}, " "))
}

func sectorTextContains(text string, tokens ...string) bool {
	slug := util.SlugTR(text)
	for _, token := range tokens {
		if strings.Contains(slug, util.SlugTR(token)) {
			return true
		}
	}
	return false
}

func sectorMetric(id string, fin financialFile, schema financialFieldSchema, p period, valuation ValuationAnalysis) (SectorFinancialMetric, bool) {
	ratioMetric := func(label string, value float64, unit string, fields []string, status string, interpretation string) (SectorFinancialMetric, bool) {
		if !validMetricValue(value) {
			return SectorFinancialMetric{}, false
		}
		return SectorFinancialMetric{Name: id, Label: label, Value: value, Unit: unit, Status: status, Interpretation: interpretation, SourceFields: fields}, true
	}
	withMarketCap := func(fields []string) []string {
		out := []string{"market_cap"}
		return append(out, fields...)
	}
	switch id {
	case "current_ratio":
		currentAssets, _, okAssets := schemaFieldValue(fin, schema, fieldCurrentAssets, p)
		currentLiabilities, _, okLiabilities := schemaFieldValue(fin, schema, fieldShortTermLiab, p)
		if !okAssets || !okLiabilities {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(currentAssets, currentLiabilities)
		return ratioMetric("Cari oran", value, "x", schemaFieldCodes(fin, schema, fieldCurrentAssets, fieldShortTermLiab), liquidityStatus(value), fmt.Sprintf("Cari oran %.2fx; kisa vadeli yukumluluk karsilama gucu sektor icin izlenir.", value))
	case "quick_ratio":
		currentAssets, _, okAssets := schemaFieldValue(fin, schema, fieldCurrentAssets, p)
		inventory, _, okInventory := schemaFieldValue(fin, schema, fieldInventory, p)
		currentLiabilities, _, okLiabilities := schemaFieldValue(fin, schema, fieldShortTermLiab, p)
		if !okAssets || !okInventory || !okLiabilities {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(currentAssets-inventory, currentLiabilities)
		return ratioMetric("Asit-test oran", value, "x", schemaFieldCodes(fin, schema, fieldCurrentAssets, fieldInventory, fieldShortTermLiab), liquidityStatus(value), fmt.Sprintf("Asit-test %.2fx; stok haric likidite gucu icin okunur.", value))
	case "cash_ratio":
		cash, _, okCash := schemaFieldValue(fin, schema, fieldCash, p)
		currentLiabilities, _, okLiabilities := schemaFieldValue(fin, schema, fieldShortTermLiab, p)
		if !okCash || !okLiabilities {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(cash, currentLiabilities)
		return ratioMetric("Nakit oran", value, "x", schemaFieldCodes(fin, schema, fieldCash, fieldShortTermLiab), cashRatioStatus(value), fmt.Sprintf("Nakit oran %.2fx; kisa vadeli nakit tamponu okunur.", value))
	case "gross_margin":
		grossProfit, _, okGrossProfit := schemaTTM(fin, schema, fieldGrossProfit, p)
		revenue, _, okRevenue := schemaTTM(fin, schema, fieldRevenue, p)
		if !okGrossProfit || !okRevenue {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(grossProfit, revenue)
		return ratioMetric("Brut kar marji", value, "%", schemaFieldCodes(fin, schema, fieldGrossProfit, fieldRevenue), marginStatus(value, 0.25, 0.10), fmt.Sprintf("Brut marj %.1f%%; fiyatlama ve maliyet disiplini icin ana gostergedir.", value*100))
	case "ebit_margin":
		ebit, _, okEBIT := schemaTTM(fin, schema, fieldEBIT, p)
		revenue, _, okRevenue := schemaTTM(fin, schema, fieldRevenue, p)
		if !okEBIT || !okRevenue {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(ebit, revenue)
		return ratioMetric("Faaliyet kar marji", value, "%", schemaFieldCodes(fin, schema, fieldEBIT, fieldRevenue), marginStatus(value, 0.15, 0.05), fmt.Sprintf("Faaliyet marji %.1f%%; operasyonel karlilik kalitesi icin okunur.", value*100))
	case "ebitda_margin":
		ebit, _, okEBIT := schemaTTM(fin, schema, fieldEBIT, p)
		amortization, _, okAmortization := schemaTTM(fin, schema, fieldAmortization, p)
		revenue, _, okRevenue := schemaTTM(fin, schema, fieldRevenue, p)
		if !okEBIT || !okAmortization || !okRevenue {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(ebit+math.Max(amortization, 0), revenue)
		return ratioMetric("FAVOK marji", value, "%", schemaFieldCodes(fin, schema, fieldEBIT, fieldAmortization, fieldRevenue), marginStatus(value, 0.20, 0.08), fmt.Sprintf("FAVOK marji %.1f%%; borc servis kapasitesiyle birlikte okunur.", value*100))
	case "net_margin":
		netIncome, _, okNetIncome := schemaTTM(fin, schema, fieldNetIncome, p)
		revenue, _, okRevenue := schemaTTM(fin, schema, fieldRevenue, p)
		if !okNetIncome || !okRevenue {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(netIncome, revenue)
		return ratioMetric("Net kar marji", value, "%", schemaFieldCodes(fin, schema, fieldNetIncome, fieldRevenue), marginStatus(value, 0.12, 0.03), fmt.Sprintf("Net kar marji %.1f%%; son satir karlilik gucu icin okunur.", value*100))
	case "fcf_margin":
		freeCashFlow, _, okFCF := schemaTTM(fin, schema, fieldFreeCashFlow, p)
		revenue, _, okRevenue := schemaTTM(fin, schema, fieldRevenue, p)
		if !okFCF || !okRevenue {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(freeCashFlow, revenue)
		return ratioMetric("Serbest nakit akimi marji", value, "%", schemaFieldCodes(fin, schema, fieldFreeCashFlow, fieldRevenue), marginStatus(value, 0.08, 0), fmt.Sprintf("FCF marji %.1f%%; buyume ve borc odeme icin nakit yaratimi olarak okunur.", value*100))
	case "fcf_conversion":
		freeCashFlow, _, okFCF := schemaTTM(fin, schema, fieldFreeCashFlow, p)
		netIncome, _, okNetIncome := schemaTTM(fin, schema, fieldNetIncome, p)
		if !okFCF || !okNetIncome {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(freeCashFlow, netIncome)
		return ratioMetric("FCF / net kar", value, "x", schemaFieldCodes(fin, schema, fieldFreeCashFlow, fieldNetIncome), conversionStatus(value), fmt.Sprintf("FCF/net kar %.2fx; muhasebe karinin nakde donusumu icin okunur.", value))
	case "roe":
		netIncome, _, okNetIncome := schemaTTM(fin, schema, fieldNetIncome, p)
		equity, _, okEquity := schemaFieldValue(fin, schema, fieldEquity, p)
		if !okNetIncome || !okEquity {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(netIncome, equity)
		return ratioMetric("ROE", value, "%", schemaFieldCodes(fin, schema, fieldNetIncome, fieldEquity), returnStatus(value, 0.18, 0.08), fmt.Sprintf("ROE %.1f%%; ozkaynak karliligi sektor profiline gore ana kalite sinyalidir.", value*100))
	case "roa":
		netIncome, _, okNetIncome := schemaTTM(fin, schema, fieldNetIncome, p)
		totalAssets, _, okAssets := schemaFieldValue(fin, schema, fieldTotalAssets, p)
		if !okNetIncome || !okAssets {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(netIncome, totalAssets)
		return ratioMetric("ROA", value, "%", schemaFieldCodes(fin, schema, fieldNetIncome, fieldTotalAssets), returnStatus(value, 0.08, 0.03), fmt.Sprintf("ROA %.1f%%; aktiflerin kar uretme gucu icin okunur.", value*100))
	case "equity_to_assets":
		equity, _, okEquity := schemaFieldValue(fin, schema, fieldEquity, p)
		totalAssets, _, okAssets := schemaFieldValue(fin, schema, fieldTotalAssets, p)
		if !okEquity || !okAssets {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(equity, totalAssets)
		return ratioMetric("Ozkaynak / aktif", value, "%", schemaFieldCodes(fin, schema, fieldEquity, fieldTotalAssets), capitalBufferStatus(value), fmt.Sprintf("Ozkaynak/aktif %.1f%%; kaldirac ve sermaye tamponu icin okunur.", value*100))
	case "assets_to_equity":
		totalAssets, _, okAssets := schemaFieldValue(fin, schema, fieldTotalAssets, p)
		equity, _, okEquity := schemaFieldValue(fin, schema, fieldEquity, p)
		if !okAssets || !okEquity {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(totalAssets, equity)
		return ratioMetric("Aktif / ozkaynak", value, "x", schemaFieldCodes(fin, schema, fieldTotalAssets, fieldEquity), leverageMultipleStatus(value), fmt.Sprintf("Aktif/ozkaynak %.2fx; finansal kaldirac seviyesi icin okunur.", value))
	case "net_debt_to_equity":
		debt, _, okDebt := schemaSumFieldValues(fin, schema, fieldDebt, p)
		cash, _, okCash := schemaFieldValue(fin, schema, fieldCash, p)
		equity, _, okEquity := schemaFieldValue(fin, schema, fieldEquity, p)
		if !okDebt || !okCash || !okEquity {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(debt-cash, equity)
		return ratioMetric("Net borc / ozkaynak", value, "x", schemaFieldCodes(fin, schema, fieldDebt, fieldCash, fieldEquity), leverageStatus(value), fmt.Sprintf("Net borc/ozkaynak %.2fx; bilanço riski icin ana kaldirac sinyalidir.", value))
	case "net_debt_to_ebitda":
		debt, _, okDebt := schemaSumFieldValues(fin, schema, fieldDebt, p)
		cash, _, okCash := schemaFieldValue(fin, schema, fieldCash, p)
		ebit, _, okEBIT := schemaTTM(fin, schema, fieldEBIT, p)
		amortization, _, okAmortization := schemaTTM(fin, schema, fieldAmortization, p)
		if !okDebt || !okCash || !okEBIT || !okAmortization {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(debt-cash, ebit+math.Max(amortization, 0))
		return ratioMetric("Net borc / FAVOK", value, "x", schemaFieldCodes(fin, schema, fieldDebt, fieldCash, fieldEBIT, fieldAmortization), debtServiceStatus(value), fmt.Sprintf("Net borc/FAVOK %.2fx; borcun operasyonel nakit yaratimina gore agirligi icin okunur.", value))
	case "asset_turnover":
		revenue, _, okRevenue := schemaTTM(fin, schema, fieldRevenue, p)
		totalAssets, _, okAssets := schemaFieldValue(fin, schema, fieldTotalAssets, p)
		if !okRevenue || !okAssets {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(revenue, totalAssets)
		return ratioMetric("Aktif devir hizi", value, "x", schemaFieldCodes(fin, schema, fieldRevenue, fieldTotalAssets), turnoverStatus(value, 0.80, 0.25), fmt.Sprintf("Aktif devir hizi %.2fx; varliklarin satis uretme verimi icin okunur.", value))
	case "inventory_turnover":
		cogs, _, okCOGS := schemaTTM(fin, schema, fieldCOGS, p)
		inventory, _, okInventory := schemaFieldValue(fin, schema, fieldInventory, p)
		if !okCOGS || !okInventory {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(math.Abs(cogs), inventory)
		return ratioMetric("Stok devir hizi", value, "x", schemaFieldCodes(fin, schema, fieldCOGS, fieldInventory), turnoverStatus(value, 4.0, 1.5), fmt.Sprintf("Stok devir hizi %.2fx; stok baglama ve satis ritmi icin okunur.", value))
	case "receivable_turnover":
		revenue, _, okRevenue := schemaTTM(fin, schema, fieldRevenue, p)
		receivables, _, okReceivables := schemaFieldValue(fin, schema, fieldReceivables, p)
		if !okRevenue || !okReceivables {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(revenue, receivables)
		return ratioMetric("Alacak devir hizi", value, "x", schemaFieldCodes(fin, schema, fieldRevenue, fieldReceivables), turnoverStatus(value, 4.0, 1.5), fmt.Sprintf("Alacak devir hizi %.2fx; tahsilat ve isletme sermayesi kalitesi icin okunur.", value))
	case "deferred_tax_quality":
		deferredTax, _, okDT := schemaTTM(fin, schema, fieldDeferredTaxIncome, p)
		netIncome, _, okNI := schemaTTM(fin, schema, fieldNetIncome, p)
		if !okDT || !okNI || netIncome == 0 {
			return SectorFinancialMetric{}, false
		}
		ratio := mathutil.SafeDiv(deferredTax, netIncome)
		var status string
		switch {
		case ratio < 0:
			status = "strong"
		case ratio < 0.15:
			status = "ok"
		case ratio < 0.30:
			status = "watch"
		default:
			status = "weak"
		}
		return ratioMetric("Ertelenmiş vergi / net kar", ratio, "x",
			schemaFieldCodes(fin, schema, fieldDeferredTaxIncome, fieldNetIncome), status,
			fmt.Sprintf("Ertelenmis vergi gelirinin net kara orani %.2fx; yuksekse raporlanan net kar nakit esigi olmayan muhasebe kalemi iceriyor olabilir.", ratio))
	case "fcf_income_quality":
		freeCashFlow, _, okFCF := schemaTTM(fin, schema, fieldFreeCashFlow, p)
		netIncome, _, okNI := schemaTTM(fin, schema, fieldNetIncome, p)
		if !okFCF || !okNI || netIncome == 0 {
			return SectorFinancialMetric{}, false
		}
		ratio := mathutil.SafeDiv(freeCashFlow, netIncome)
		var status string
		switch {
		case ratio >= 0.80:
			status = "strong"
		case ratio >= 0.30:
			status = "ok"
		case ratio >= 0:
			status = "watch"
		default:
			// FCF negatif, net kar pozitif: gelir kalitesi uyarısı
			status = "critical"
		}
		return ratioMetric("FCF / net kar (gelir kalitesi)", ratio, "x",
			schemaFieldCodes(fin, schema, fieldFreeCashFlow, fieldNetIncome), status,
			fmt.Sprintf("FCF/net kar %.2fx; negatifse muhasebe karinin nakde donusumu gerceklesmemis demektir — buyuk CAPEX veya isletme sermayesi artisi varligini kontrol et.", ratio))
	case "book_per_share":
		equity, _, okEquity := schemaFieldValue(fin, schema, fieldEquity, p)
		paidCapital, _, okPaidCapital := schemaFieldValue(fin, schema, fieldPaidCapital, p)
		if !okEquity || !okPaidCapital {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(equity, paidCapital)
		return ratioMetric("Hisse basina defter degeri", value, "", schemaFieldCodes(fin, schema, fieldEquity, fieldPaidCapital), neutralStatus(value), fmt.Sprintf("Hisse basina defter degeri %.2f; PD/DD ve NAV proxy okumasinin tabanidir.", value))
	case "pb":
		value := valuation.Ratios["PB"]
		return ratioMetric("PD/DD", value, "x", withMarketCap(schemaFieldCodes(fin, schema, fieldEquity, fieldPaidCapital)), valuationStatus(value), fmt.Sprintf("PD/DD %.2fx; sektorun defter degeri carpanina gore okunmalidir.", value))
	case "pe":
		value := valuation.Ratios["PE"]
		return ratioMetric("F/K", value, "x", withMarketCap(schemaFieldCodes(fin, schema, fieldNetIncome, fieldPaidCapital)), valuationStatus(value), fmt.Sprintf("F/K %.2fx; karliligin surdurulebilirligi ile birlikte okunur.", value))
	case "loan_to_deposit_proxy":
		loans, _, okLoans := schemaFieldValue(fin, schema, fieldLoans, p)
		deposits, _, okDeposits := schemaFieldValue(fin, schema, fieldDeposits, p)
		if !okLoans || !okDeposits {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(loans, deposits)
		return ratioMetric("Kredi / mevduat proxy", value, "x", schemaFieldCodes(fin, schema, fieldLoans, fieldDeposits), loanDepositStatus(value), fmt.Sprintf("Kredi/mevduat proxy %.2fx; banka bilançosunda fonlama dengesi icin izlenir.", value))
	case "net_interest_income_to_assets":
		netInterest, _, okNetInterest := schemaTTM(fin, schema, fieldNetInterestIncome, p)
		totalAssets, _, okAssets := schemaFieldValue(fin, schema, fieldTotalAssets, p)
		if !okNetInterest || !okAssets {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(netInterest, totalAssets)
		return ratioMetric("Net faiz geliri / aktif proxy", value, "%", schemaFieldCodes(fin, schema, fieldNetInterestIncome, fieldTotalAssets), marginStatus(value, 0.04, 0.01), fmt.Sprintf("Net faiz geliri/aktif proxy %.1f%%; banka karliligi icin sinirli bir NIM proxy'sidir.", value*100))
	case "technical_balance_to_assets":
		technicalBalance, _, okTechnicalBalance := schemaTTM(fin, schema, fieldTechnicalBalance, p)
		totalAssets, _, okAssets := schemaFieldValue(fin, schema, fieldTotalAssets, p)
		if !okTechnicalBalance || !okAssets {
			return SectorFinancialMetric{}, false
		}
		value := mathutil.SafeDiv(technicalBalance, totalAssets)
		return ratioMetric("Teknik sonuc / aktif proxy", value, "%", schemaFieldCodes(fin, schema, fieldTechnicalBalance, fieldTotalAssets), marginStatus(value, 0.04, 0), fmt.Sprintf("Teknik sonuc/aktif proxy %.1f%%; sigorta karliligi icin sinirli bir teknik sonuc okumasidir.", value*100))
	default:
		return SectorFinancialMetric{}, false
	}
}

func validMetricValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value != 0
}

func sectorFinancialScore(metrics []SectorFinancialMetric) float64 {
	if len(metrics) == 0 {
		return 0
	}
	total := 0.0
	for _, metric := range metrics {
		total += metricScore(metric.Status)
	}
	return math.Round(mathutil.SafeDiv(total, float64(len(metrics))))
}

func metricScore(status string) float64 {
	switch status {
	case "strong":
		return 90
	case "ok":
		return 70
	case "watch":
		return 50
	case "weak":
		return 30
	case "critical":
		return 15
	default:
		return 60
	}
}

func sectorFinancialSummary(out SectorFinancialAnalysis) string {
	if len(out.Metrics) == 0 {
		return fmt.Sprintf("%s profili icin hesaplanabilir sektor metrigi bulunamadi.", out.ProfileLabel)
	}
	summary := fmt.Sprintf("%s profiliyle %d sektor metrigi okundu; skor %.0f/100.", out.ProfileLabel, len(out.Metrics), out.Score)
	if len(out.Risks) > 0 {
		summary += " Ana risk: " + out.Risks[0]
	} else if len(out.Strengths) > 0 {
		summary += " Ana guc: " + out.Strengths[0]
	}
	return summary
}

func liquidityStatus(value float64) string {
	switch {
	case value < 1:
		return "weak"
	case value >= 1.5:
		return "strong"
	default:
		return "ok"
	}
}

func cashRatioStatus(value float64) string {
	switch {
	case value < 0.10:
		return "weak"
	case value >= 0.30:
		return "strong"
	default:
		return "ok"
	}
}

func marginStatus(value, strong, ok float64) string {
	switch {
	case value < 0:
		return "critical"
	case value >= strong:
		return "strong"
	case value >= ok:
		return "ok"
	default:
		return "watch"
	}
}

func conversionStatus(value float64) string {
	switch {
	case value < 0:
		return "critical"
	case value >= 0.80:
		return "strong"
	case value >= 0.40:
		return "ok"
	default:
		return "weak"
	}
}

func returnStatus(value, strong, ok float64) string {
	switch {
	case value < 0:
		return "critical"
	case value >= strong:
		return "strong"
	case value >= ok:
		return "ok"
	default:
		return "watch"
	}
}

func capitalBufferStatus(value float64) string {
	switch {
	case value < 0.10:
		return "critical"
	case value >= 0.35:
		return "strong"
	case value >= 0.20:
		return "ok"
	default:
		return "watch"
	}
}

func leverageMultipleStatus(value float64) string {
	switch {
	case value <= 0:
		return "neutral"
	case value <= 3:
		return "strong"
	case value <= 6:
		return "ok"
	default:
		return "watch"
	}
}

func leverageStatus(value float64) string {
	switch {
	case value < 0:
		return "strong"
	case value <= 0.50:
		return "strong"
	case value <= 1.0:
		return "ok"
	case value <= 2.0:
		return "watch"
	default:
		return "weak"
	}
}

func debtServiceStatus(value float64) string {
	switch {
	case value < 0:
		return "strong"
	case value <= 2:
		return "strong"
	case value <= 4:
		return "ok"
	case value <= 6:
		return "watch"
	default:
		return "weak"
	}
}

func turnoverStatus(value, strong, ok float64) string {
	switch {
	case value >= strong:
		return "strong"
	case value >= ok:
		return "ok"
	default:
		return "watch"
	}
}

func neutralStatus(value float64) string {
	if value <= 0 {
		return "weak"
	}
	return "neutral"
}

func valuationStatus(value float64) string {
	if value <= 0 {
		return "weak"
	}
	return "neutral"
}

func loanDepositStatus(value float64) string {
	switch {
	case value <= 0:
		return "neutral"
	case value <= 0.90:
		return "strong"
	case value <= 1.10:
		return "ok"
	default:
		return "watch"
	}
}

// ratioMeta defines display metadata for each bilanco_hesaplari ratio key.
var ratioMeta = map[string]struct {
	Label string
	Unit  string
}{
	"ROE":             {Label: "Özsermaye Karlılığı (ROE)", Unit: "%"},
	"ROA":             {Label: "Aktif Karlılığı (ROA)", Unit: "%"},
	"NetKarMarji":     {Label: "Net Kar Marjı", Unit: "%"},
	"BrutKarMarji":    {Label: "Brüt Kar Marjı", Unit: "%"},
	"FaaliyetKarMarji": {Label: "Faaliyet Kar Marjı", Unit: "%"},
	"CariOran":        {Label: "Cari Oran", Unit: "x"},
	"AsitTestOran":    {Label: "Asit Test Oranı", Unit: "x"},
	"LikiditeOran":    {Label: "Likidite Oranı", Unit: "x"},
	"NakitOran":       {Label: "Nakit Oranı", Unit: "x"},
	"AlacakDevirHizi": {Label: "Alacak Devir Hızı", Unit: "x"},
	"StokDevirHizi":   {Label: "Stok Devir Hızı", Unit: "x"},
	"VarlikDevirHizi": {Label: "Varlık Devir Hızı", Unit: "x"},
}

// buildHistoricalRatioSummary derives trend summaries from bilanco_hesaplari.json.
// latest provides the reference year/quarter to determine the most recent period.
func buildHistoricalRatioSummary(history financialRatioHistory, latest period) *HistoricalRatioSummary {
	if len(history) == 0 {
		return nil
	}

	// Collect all years present across all ratios.
	yearSet := map[string]struct{}{}
	for _, byYear := range history {
		for y := range byYear {
			yearSet[y] = struct{}{}
		}
	}
	if len(yearSet) == 0 {
		return nil
	}
	years := make([]string, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	// Sort years ascending.
	for i := 0; i < len(years); i++ {
		for j := i + 1; j < len(years); j++ {
			if years[i] > years[j] {
				years[i], years[j] = years[j], years[i]
			}
		}
	}

	latestYearStr := fmt.Sprintf("%d", latest.Year)
	latestQStr := fmt.Sprintf("Q%d", latest.Quarter)

	// Priority order for key ratios in output.
	keyOrder := []string{
		"ROE", "ROA", "NetKarMarji", "BrutKarMarji", "FaaliyetKarMarji",
		"CariOran", "AsitTestOran", "LikiditeOran", "NakitOran",
		"AlacakDevirHizi", "StokDevirHizi", "VarlikDevirHizi",
	}
	// Add any extra keys not in the priority list.
	extra := map[string]struct{}{}
	for k := range history {
		extra[k] = struct{}{}
	}
	ordered := make([]string, 0, len(history))
	for _, k := range keyOrder {
		if _, ok := history[k]; ok {
			ordered = append(ordered, k)
			delete(extra, k)
		}
	}
	for k := range extra {
		ordered = append(ordered, k)
	}

	periodsTotal := 0
	var rows []HistoricalRatioRow
	var signals []string

	for _, ratioID := range ordered {
		byYear, ok := history[ratioID]
		if !ok {
			continue
		}
		meta := ratioMeta[ratioID]
		label := meta.Label
		if label == "" {
			label = ratioID
		}

		// Build yearly summary: use Q4 if available, else last non-nil quarter.
		quarters := []string{"Q4", "Q3", "Q2", "Q1"}
		yearlyVals := map[string]float64{}
		for _, y := range years {
			qMap := byYear[y]
			for _, q := range quarters {
				if v := qMap[q]; v != nil {
					yearlyVals[y] = *v
					break
				}
			}
		}

		// Latest value: exact period first, then fall back to latest available.
		var latestVal *float64
		if qMap, ok2 := byYear[latestYearStr]; ok2 {
			if v := qMap[latestQStr]; v != nil {
				latestVal = v
			} else {
				for _, q := range quarters {
					if v := qMap[q]; v != nil {
						latestVal = v
						break
					}
				}
			}
		}
		if latestVal == nil {
			// Walk backwards through years to find most recent value.
			for i := len(years) - 1; i >= 0; i-- {
				if vv, found := yearlyVals[years[i]]; found {
					copy := vv
					latestVal = &copy
					break
				}
			}
		}

		// 3-year and 5-year averages (using yearly Q4/last values).
		avg := func(n int) *float64 {
			count := 0
			sum := 0.0
			for i := len(years) - 1; i >= 0 && count < n; i-- {
				if v, ok := yearlyVals[years[i]]; ok {
					sum += v
					count++
				}
			}
			if count == 0 {
				return nil
			}
			r := sum / float64(count)
			return &r
		}
		avg3 := avg(3)
		avg5 := avg(5)

		// Trend: compare latest 3 yearly values.
		trend := "insufficient_data"
		trendScore := 50.0
		if latestVal != nil && len(years) >= 2 {
			// Count how many of the last 3 years have increasing values.
			improvements := 0
			deteriorations := 0
			recentCount := 0
			for i := len(years) - 1; i >= 1 && recentCount < 3; i-- {
				cur, hasCur := yearlyVals[years[i]]
				prev, hasPrev := yearlyVals[years[i-1]]
				if !hasCur || !hasPrev {
					continue
				}
				recentCount++
				if cur > prev {
					improvements++
				} else if cur < prev {
					deteriorations++
				}
			}
			if recentCount > 0 {
				switch {
				case improvements > deteriorations && improvements >= 2:
					trend = "improving"
					trendScore = 70 + float64(improvements)*10
				case deteriorations > improvements && deteriorations >= 2:
					trend = "declining"
					trendScore = 30 - float64(deteriorations)*5
				case improvements == deteriorations:
					trend = "stable"
					trendScore = 50
				default:
					trend = "mixed"
					trendScore = 45
				}
				if trendScore > 100 {
					trendScore = 100
				}
				if trendScore < 0 {
					trendScore = 0
				}
			}
		}

		// Only include years with at least one non-nil value.
		filteredYears := map[string]float64{}
		for y, v := range yearlyVals {
			filteredYears[y] = v
		}

		row := HistoricalRatioRow{
			ID:           ratioID,
			Label:        label,
			Unit:         meta.Unit,
			LatestValue:  latestVal,
			Avg3Y:        avg3,
			Avg5Y:        avg5,
			Trend:        trend,
			TrendScore:   trendScore,
			YearlyValues: filteredYears,
		}
		rows = append(rows, row)
		periodsTotal += len(filteredYears)

		// Generate notable signals.
		if latestVal != nil {
			switch ratioID {
			case "ROE":
				if *latestVal < 0 && trend == "declining" {
					signals = append(signals, fmt.Sprintf("ROE negatif (%.1f%%) ve düşüş trendinde; özsermaye karlılığı baskı altında.", *latestVal))
				} else if *latestVal < 0 {
					signals = append(signals, fmt.Sprintf("ROE negatif (%.1f%%); zarar döneminde.", *latestVal))
				} else if *latestVal > 20 && trend == "improving" {
					signals = append(signals, fmt.Sprintf("ROE güçlü (%.1f%%) ve yükseliş trendinde.", *latestVal))
				}
			case "NetKarMarji":
				if *latestVal < 0 {
					signals = append(signals, fmt.Sprintf("Net Kar Marjı negatif (%.1f%%); operasyonel zarar.", *latestVal))
				} else if avg3 != nil && *latestVal < *avg3*0.7 {
					signals = append(signals, fmt.Sprintf("Net Kar Marjı (%.1f%%) 3Y ortalamasının (%.1f%%) belirgin altında; marj sıkışması.", *latestVal, *avg3))
				}
			case "BrutKarMarji":
				if avg3 != nil && *latestVal < *avg3*0.8 {
					signals = append(signals, fmt.Sprintf("Brüt Kar Marjı (%.1f%%) 3Y ortalamasının (%.1f%%) belirgin altında.", *latestVal, *avg3))
				}
			case "CariOran":
				if *latestVal < 1.0 {
					signals = append(signals, fmt.Sprintf("Cari Oran 1.0 altında (%.2f); kısa vadeli likidite riski.", *latestVal))
				}
			}
		}
	}

	if len(rows) == 0 {
		return nil
	}
	return &HistoricalRatioSummary{
		PeriodsAvailable: periodsTotal,
		YearsAvailable:   years,
		Ratios:           rows,
		TrendSignals:     signals,
	}
}
