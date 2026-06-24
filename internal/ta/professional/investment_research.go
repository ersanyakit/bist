package professional

import (
	"fmt"
	"math"
	"strings"

	"hissebot/internal/ta/value"
	"hissebot/internal/util"
	"hissebot/pkg/mathutil"
)

type InvestmentResearchReview struct {
	Computed              bool                   `json:"computed"`
	Summary               string                 `json:"summary"`
	Readiness             []InvestorReadiness    `json:"investor_readiness,omitempty"`
	InstitutionalMemo     InstitutionalMemo      `json:"institutional_memo"`
	InvestmentStory       InvestmentStory        `json:"investment_story"`
	ValuationBridge       ValuationTransparency  `json:"valuation_bridge"`
	AssetDueDiligence     AssetDueDiligence      `json:"asset_due_diligence"`
	FinancialQuality      FinancialQualityBridge `json:"financial_quality"`
	DecisionFramework     DecisionFramework      `json:"decision_framework"`
	ScoreExplanations     []ScoreExplanation     `json:"score_explanations,omitempty"`
	OpenResearchQuestions []string               `json:"open_research_questions,omitempty"`
	Warnings              []string               `json:"warnings,omitempty"`
}

type InvestorReadiness struct {
	Segment     string  `json:"segment"`
	CoveragePct float64 `json:"coverage_pct"`
	Comment     string  `json:"comment"`
}

type InvestmentStory struct {
	CoreThesis         string   `json:"core_thesis"`
	ValueSource        string   `json:"value_source"`
	MispricingQuestion string   `json:"mispricing_question"`
	KeyEvidence        []string `json:"key_evidence,omitempty"`
	KeyRisks           []string `json:"key_risks,omitempty"`
	Catalysts          []string `json:"catalysts,omitempty"`
}

type ValuationTransparency struct {
	Model              string    `json:"model"`
	Method             string    `json:"method"`
	Formula            string    `json:"formula"`
	PrimaryInputs      []string  `json:"primary_inputs,omitempty"`
	MissingInputs      []string  `json:"missing_inputs,omitempty"`
	Limitations        []string  `json:"limitations,omitempty"`
	NAVBridge          NAVBridge `json:"nav_bridge"`
	CurrentPrice       float64   `json:"current_price"`
	BaseIntrinsicValue float64   `json:"base_intrinsic_value"`
	BearIntrinsicValue float64   `json:"bear_intrinsic_value"`
	BullIntrinsicValue float64   `json:"bull_intrinsic_value"`
	RequiredMarginPct  float64   `json:"required_margin_pct"`
	BuyBelowPrice      float64   `json:"buy_below_price,omitempty"`
	PriceToBasePct     float64   `json:"price_to_base_pct,omitempty"`
	NAVStatus          string    `json:"nav_status"`
}

type NAVBridge struct {
	Status                    string   `json:"status"`
	DataQuality               string   `json:"data_quality"`
	PortfolioValueExclVATTRY  *float64 `json:"portfolio_value_excl_vat_try,omitempty"`
	PortfolioValueInclVATTRY  *float64 `json:"portfolio_value_incl_vat_try,omitempty"`
	PortfolioBookValueTRY     *float64 `json:"portfolio_book_value_try,omitempty"`
	SelectedPortfolioValueTRY float64  `json:"selected_portfolio_value_try,omitempty"`
	NetDebtTRY                float64  `json:"net_debt_try,omitempty"`
	EquityTRY                 float64  `json:"equity_try,omitempty"`
	MarketCapTRY              float64  `json:"market_cap_try,omitempty"`
	PaidCapital               float64  `json:"paid_capital,omitempty"`
	EstimatedNAVTRY           float64  `json:"estimated_nav_try,omitempty"`
	EstimatedNAVPerShare      float64  `json:"estimated_nav_per_share,omitempty"`
	MarketCapToNAVPremiumPct  float64  `json:"market_cap_to_nav_premium_pct,omitempty"`
	RequiredInputsForFullNAV  []string `json:"required_inputs_for_full_nav,omitempty"`
	ReconciliationLimitations []string `json:"reconciliation_limitations,omitempty"`
}

type AssetDueDiligence struct {
	InventoryComputed        bool     `json:"inventory_computed"`
	EventCount               int      `json:"event_count"`
	RawAssetCount            int      `json:"raw_asset_count"`
	DisplayAssetCount        int      `json:"display_asset_count"`
	RentalAssetCount         int      `json:"rental_asset_count"`
	ProjectCount             int      `json:"project_count"`
	PortfolioHistoryCount    int      `json:"portfolio_history_count"`
	PortfolioTotalsAvailable bool     `json:"portfolio_totals_available"`
	ValuationLinkedStatus    string   `json:"valuation_linked_status"`
	Findings                 []string `json:"findings,omitempty"`
	RequiredChecks           []string `json:"required_checks,omitempty"`
}

type FinancialQualityBridge struct {
	Summary       string                   `json:"summary"`
	Metrics       []FinancialQualityMetric `json:"metrics,omitempty"`
	RedFlags      []string                 `json:"red_flags,omitempty"`
	NeedToExplain []string                 `json:"need_to_explain,omitempty"`
}

type FinancialQualityMetric struct {
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit,omitempty"`
	Status  string  `json:"status"`
	Comment string  `json:"comment"`
}

type DecisionFramework struct {
	CurrentDecision string   `json:"current_decision"`
	DecisionBasis   []string `json:"decision_basis,omitempty"`
	BuyConditions   []string `json:"buy_conditions,omitempty"`
	HoldConditions  []string `json:"hold_conditions,omitempty"`
	SellConditions  []string `json:"sell_conditions,omitempty"`
	Invalidation    []string `json:"invalidation,omitempty"`
}

type InstitutionalMemo struct {
	Recommendation            string   `json:"recommendation"`
	WorkflowStatus            string   `json:"workflow_status,omitempty"`
	DirectBuyEligible         bool     `json:"direct_buy_eligible"`
	InvestmentCommitteeReady  bool     `json:"investment_committee_ready"`
	BrokeragePublishableReady bool     `json:"brokerage_publishable_ready"`
	ReadinessScore            float64  `json:"readiness_score"`
	Decision                  string   `json:"decision"`
	PositionSizeSuggestion    string   `json:"position_size_suggestion,omitempty"`
	InvestmentHorizon         string   `json:"investment_horizon,omitempty"`
	ExpectedReturnPct         float64  `json:"expected_return_pct,omitempty"`
	DownsideRiskPct           float64  `json:"downside_risk_pct,omitempty"`
	RiskRewardRatio           float64  `json:"risk_reward_ratio,omitempty"`
	LiquidityConsideration    string   `json:"liquidity_consideration,omitempty"`
	PortfolioFit              string   `json:"portfolio_fit,omitempty"`
	KeyAssumptions            []string `json:"key_assumptions,omitempty"`
	ApprovalConditions        []string `json:"approval_conditions,omitempty"`
	RejectionConditions       []string `json:"rejection_conditions,omitempty"`
	BlockingIssues            []string `json:"blocking_issues,omitempty"`
	RequiredFixes             []string `json:"required_fixes,omitempty"`
	CommitteeQuestions        []string `json:"committee_questions,omitempty"`
	PositiveSignals           []string `json:"positive_signals,omitempty"`
}

type ScoreExplanation struct {
	Score   string `json:"score"`
	Meaning string `json:"meaning"`
	GoodBad string `json:"good_bad"`
	Driver  string `json:"driver"`
}

func buildInvestmentResearchReview(
	lastClose float64,
	profile CompanyProfile,
	valuation ValuationAnalysis,
	sectorFinancials SectorFinancialAnalysis,
	kapPDF KAPPDFIngestSummary,
	assets KAPAssetInventorySummary,
	peers PeerComparison,
	scenarios []Scenario,
	valueReport value.Report,
	coverage CoverageReport,
) InvestmentResearchReview {
	out := InvestmentResearchReview{Computed: true}
	isGYO := isGYOProfile(profile, valuation, valueReport)
	out.Readiness = buildInvestorReadiness(coverage, valueReport, kapPDF, assets, peers, isGYO)
	out.ValuationBridge = buildValuationTransparency(lastClose, valuation, valueReport, assets, isGYO)
	out.AssetDueDiligence = buildAssetDueDiligence(assets, isGYO)
	out.FinancialQuality = buildFinancialQualityBridge(valuation, sectorFinancials, valueReport, isGYO)
	out.DecisionFramework = buildDecisionFramework(lastClose, valueReport, valuation, assets, scenarios, out.ValuationBridge)
	out.InvestmentStory = buildInvestmentStory(profile, valuation, kapPDF, assets, valueReport, out.FinancialQuality, out.DecisionFramework, isGYO)
	out.InstitutionalMemo = buildInstitutionalMemo(out.Readiness, out.ValuationBridge, out.AssetDueDiligence, out.FinancialQuality, out.DecisionFramework, kapPDF, valueReport, isGYO)
	out.ScoreExplanations = buildScoreExplanations(coverage, valueReport)
	out.OpenResearchQuestions = buildOpenResearchQuestions(out.ValuationBridge, out.AssetDueDiligence, out.FinancialQuality, isGYO)
	out.Warnings = investmentResearchWarnings(out)
	out.Summary = investmentResearchSummary(out)
	return out
}

func isGYOProfile(profile CompanyProfile, valuation ValuationAnalysis, valueReport value.Report) bool {
	text := util.SlugTR(profile.Sector + " " + profile.Industry + " " + valuation.SectorModel + " " + valueReport.SectorModel.Model)
	return strings.Contains(text, "gyo") || strings.Contains(text, "gayrimenkul") || strings.Contains(text, "reit")
}

func isBankProfile(profile CompanyProfile, valuation ValuationAnalysis, valueReport value.Report) bool {
	text := util.SlugTR(profile.Sector + " " + profile.Industry + " " + valuation.SectorModel + " " + valueReport.SectorModel.Model + " " + valueReport.SectorModel.Label)
	return strings.Contains(text, "banka") || strings.Contains(text, "bank")
}

func isBankValuationModel(valuation ValuationAnalysis, valueReport value.Report) bool {
	return isBankProfile(CompanyProfile{}, valuation, valueReport)
}

func buildInvestorReadiness(coverage CoverageReport, valueReport value.Report, kapPDF KAPPDFIngestSummary, assets KAPAssetInventorySummary, peers PeerComparison, isGYO bool) []InvestorReadiness {
	base := 25.0
	if kapPDF.Computed {
		base += 12
	}
	if assets.Computed {
		base += 10
	}
	if valueReport.Computed {
		base += 8
	}
	if peers.PeerCount >= 3 {
		base += 5
	}
	base += mathutil.Clamp(coverage.Score, 0, 100) * 0.10
	if isGYO && assets.Computed && assets.PortfolioSummary.TotalRealEstateValueExclVATTRY == nil && assets.PortfolioSummary.TotalRealEstateValueInclVATTRY == nil {
		base -= 12
	}
	if isGYO && assets.Computed && assets.TotalRentalAssets == 0 {
		base -= 5
	}
	if valueReport.Confidence > 0 && valueReport.Confidence < 55 {
		base -= 5
	}
	retail := mathutil.Clamp(base, 20, 75)
	serious := mathutil.Clamp(retail-15, 15, 65)
	institutional := mathutil.Clamp(serious-12, 10, 55)
	seriousComment := "Temel analiz için başlangıç dosyasıdır; gelir, marj, borç, nakit akışı ve sermaye artırımı etkisi ayrıca doğrulanmalıdır."
	institutionalComment := "Kurumsal karar için model validasyonu, manuel KAP/PDF örneklemesi, finansal tablo mutabakatı ve yatırım komitesi notu gerekir."
	if isGYO {
		seriousComment = "Temel analiz için başlangıç dosyasıdır; NAD mutabakatı, kira sürdürülebilirliği ve finansal tablo kalitesi ayrıca doğrulanmalıdır."
		institutionalComment = "Kurumsal karar için model validasyonu, manuel KAP/PDF örneklemesi, portföy değer mutabakatı ve yatırım komitesi notu gerekir."
	}
	return []InvestorReadiness{
		{Segment: "small_investor", CoveragePct: retail, Comment: "Fiyat, teknik görünüm, KAP PDF kapsamı ve temel uyarılar için kullanılabilir; tek başına al/sat kararı değildir."},
		{Segment: "serious_fundamental_investor", CoveragePct: serious, Comment: seriousComment},
		{Segment: "institutional_professional_decision", CoveragePct: institutional, Comment: institutionalComment},
	}
}

func buildValuationTransparency(lastClose float64, valuation ValuationAnalysis, valueReport value.Report, assets KAPAssetInventorySummary, isGYO bool) ValuationTransparency {
	isBank := isBankValuationModel(valuation, valueReport)
	out := ValuationTransparency{
		Model:              firstNonEmpty(valueReport.SectorModel.Label, valuation.SectorModel),
		Method:             firstNonEmpty(valueReport.IntrinsicValue.Method, valuation.SectorModel),
		CurrentPrice:       lastClose,
		BaseIntrinsicValue: valueReport.IntrinsicValue.Base,
		BearIntrinsicValue: valueReport.IntrinsicValue.Bear,
		BullIntrinsicValue: valueReport.IntrinsicValue.Bull,
		RequiredMarginPct:  valueReport.MarginOfSafety.RequiredPct,
	}
	if out.BaseIntrinsicValue <= 0 {
		out.BaseIntrinsicValue = valuation.FairValue.Base
		out.BearIntrinsicValue = valuation.FairValue.Bear
		out.BullIntrinsicValue = valuation.FairValue.Bull
	}
	if out.RequiredMarginPct <= 0 {
		out.RequiredMarginPct = 30
	}
	if out.BaseIntrinsicValue > 0 {
		out.PriceToBasePct = 100 * mathutil.SafeDiv(lastClose-out.BaseIntrinsicValue, out.BaseIntrinsicValue)
		out.BuyBelowPrice = out.BaseIntrinsicValue * (1 - out.RequiredMarginPct/100)
	}
	out.PrimaryInputs = append(out.PrimaryInputs,
		fmt.Sprintf("book_per_share=%.2f", valuation.Ratios["BookPerShare"]),
		fmt.Sprintf("pb=%.2f", valuation.Ratios["PB"]),
		fmt.Sprintf("roe=%.1f%%", valuation.Ratios["ROE"]*100),
	)
	if isBank {
		out.PrimaryInputs = append(out.PrimaryInputs,
			fmt.Sprintf("pe=%.2f", valuation.Ratios["PE"]),
			fmt.Sprintf("roa=%.1f%%", valuation.Ratios["ROA"]*100),
		)
	} else {
		out.PrimaryInputs = append(out.PrimaryInputs, fmt.Sprintf("net_debt_to_equity=%.2f", valuation.Ratios["NetDebt_Eq"]))
	}
	if isGYO {
		out.Formula = "Baz içsel değer = hisse başına defter değeri x kaliteye göre ayarlanmış PD/DD katsayısı; güvenli alım fiyatı = baz içsel değer x (1 - gerekli güvenlik marjı)."
		out.Limitations = append(out.Limitations, "Bu hesap güncel ekspertiz tablosundan tam NAD mutabakatı değildir; defter değeri/NAV proxy olarak kullanılır.")
		if assets.PortfolioSummary.TotalRealEstateValueExclVATTRY != nil || assets.PortfolioSummary.TotalRealEstateValueInclVATTRY != nil {
			out.NAVStatus = "partial_portfolio_total_available_not_full_nav"
		} else {
			out.NAVStatus = "balance_sheet_nav_proxy"
		}
		out.NAVBridge = buildNAVBridge(valuation, assets)
		if out.NAVBridge.Status == "nav_not_reconciled" {
			out.MissingInputs = append(out.MissingInputs, "NAD veya finansal tablo özkaynak tabanı")
		} else if out.NAVBridge.Status == "book_value_nav_proxy" {
			out.Limitations = append(out.Limitations, "Tam ekspertiz NAD yerine güncel finansal tablo özkaynağı/hisse başına defter değeri proxy olarak kullanıldı.")
		}
	} else if isBank {
		out.Formula = "Banka için resmi içsel değer seti özkaynak, hisse başına defter değeri, sürdürülebilir ROE ve güvenlik marjı üzerinden okunur; peer çarpanları sadece kontrol setidir."
		out.Limitations = append(out.Limitations,
			"Klasik FCF, net borç/özsermaye, FD/Satış ve FD/FAVÖK banka değerlemesinde ana girdi değildir.",
			"Tam banka değerlemesi için sermaye yeterlilik rasyosu, çekirdek sermaye, NPL, NIM, LCR ve karşılık giderleri structured veri olarak bağlanmalıdır.",
		)
		out.MissingInputs = append(out.MissingInputs,
			"sermaye yeterlilik rasyosu",
			"çekirdek sermaye / ana sermaye oranı",
			"takipteki kredi oranı ve karşılık kapsamı",
			"net faiz marjı ve net faiz geliri büyümesi",
			"net ücret-komisyon geliri büyümesi",
			"likidite karşılama oranı ve kredi/mevduat spread'i",
		)
		out.NAVStatus = "not_applicable"
		out.NAVBridge = NAVBridge{Status: "not_applicable", DataQuality: "not_applicable"}
	} else {
		out.Formula = "Sektör modeline göre seçilen içsel değer yöntemi ve emsal çarpanları birlikte okunur."
		out.NAVStatus = "not_applicable"
		out.NAVBridge = NAVBridge{Status: "not_applicable", DataQuality: "not_applicable"}
	}
	return out
}

func buildNAVBridge(valuation ValuationAnalysis, assets KAPAssetInventorySummary) NAVBridge {
	out := NAVBridge{
		Status:                    "nav_not_reconciled",
		DataQuality:               "missing_portfolio_total",
		PortfolioValueExclVATTRY:  assets.PortfolioSummary.TotalRealEstateValueExclVATTRY,
		PortfolioValueInclVATTRY:  assets.PortfolioSummary.TotalRealEstateValueInclVATTRY,
		PortfolioBookValueTRY:     assets.PortfolioSummary.TotalBookValueTRY,
		NetDebtTRY:                valuation.NetDebt,
		EquityTRY:                 valuation.Equity,
		MarketCapTRY:              valuation.MarketCap,
		PaidCapital:               valuation.PaidCapital,
		RequiredInputsForFullNAV:  fullNAVRequiredInputs(),
		ReconciliationLimitations: []string{"Bu köprü tam NAD değildir; KAP portföy toplamı ile bilanço borç/özsermaye verisini birleştiren kısmi kontrol noktasıdır."},
	}
	selected := 0.0
	switch {
	case out.PortfolioValueExclVATTRY != nil && *out.PortfolioValueExclVATTRY > 0:
		selected = *out.PortfolioValueExclVATTRY
	case out.PortfolioValueInclVATTRY != nil && *out.PortfolioValueInclVATTRY > 0:
		selected = *out.PortfolioValueInclVATTRY
		out.ReconciliationLimitations = append(out.ReconciliationLimitations, "KDV hariç portföy toplamı bulunamadığı için KDV dahil değer proxy olarak kullanıldı.")
	case out.PortfolioBookValueTRY != nil && *out.PortfolioBookValueTRY > 0:
		selected = *out.PortfolioBookValueTRY
		out.ReconciliationLimitations = append(out.ReconciliationLimitations, "Ekspertiz toplamı bulunamadığı için defter değeri proxy olarak kullanıldı.")
	}
	out.SelectedPortfolioValueTRY = selected
	if selected <= 0 {
		if valuation.Equity > 0 {
			out.SelectedPortfolioValueTRY = valuation.Equity
			out.EstimatedNAVTRY = valuation.Equity
			out.Status = "book_value_nav_proxy"
			out.DataQuality = "financial_statement_equity_proxy"
			out.ReconciliationLimitations = append(out.ReconciliationLimitations, "Güncel ekspertiz toplamı yerine mutabakatlı finansal tablo özkaynağı kullanıldı; net borç özkaynağın içinde olduğu için ikinci kez düşülmedi.")
			if valuation.PaidCapital > 0 {
				out.EstimatedNAVPerShare = mathutil.SafeDiv(out.EstimatedNAVTRY, valuation.PaidCapital)
			}
			if valuation.MarketCap > 0 {
				out.MarketCapToNAVPremiumPct = 100 * mathutil.SafeDiv(valuation.MarketCap-out.EstimatedNAVTRY, out.EstimatedNAVTRY)
			}
		}
		return out
	}
	out.EstimatedNAVTRY = selected - valuation.NetDebt
	out.Status = "partial_nav_proxy"
	out.DataQuality = "portfolio_total_plus_net_debt"
	if valuation.PaidCapital > 0 {
		out.EstimatedNAVPerShare = mathutil.SafeDiv(out.EstimatedNAVTRY, valuation.PaidCapital)
	} else {
		out.ReconciliationLimitations = append(out.ReconciliationLimitations, "Ödenmiş sermaye/hisse adedi bulunamadığı için hisse başına NAD hesaplanamadı.")
	}
	if valuation.MarketCap > 0 && out.EstimatedNAVTRY > 0 {
		out.MarketCapToNAVPremiumPct = 100 * mathutil.SafeDiv(valuation.MarketCap-out.EstimatedNAVTRY, out.EstimatedNAVTRY)
	}
	return out
}

func fullNAVRequiredInputs() []string {
	return []string{
		"varlık bazında güncel KDV hariç ekspertiz değeri",
		"nakit ve nakit benzerleri",
		"kısa/uzun vadeli finansal borç",
		"ertelenmiş vergi etkisi",
		"azınlık payı ve bağlı ortaklık düzeltmeleri",
		"satılmış veya portföyden çıkmış varlık mutabakatı",
		"hisse sayısı/ödenmiş sermaye doğrulaması",
	}
}

func buildAssetDueDiligence(assets KAPAssetInventorySummary, isGYO bool) AssetDueDiligence {
	out := AssetDueDiligence{
		InventoryComputed:        assets.Computed,
		EventCount:               assets.EventCount,
		RawAssetCount:            assets.RawAssetCount,
		DisplayAssetCount:        assets.DisplayAssetCount,
		RentalAssetCount:         assets.TotalRentalAssets,
		ProjectCount:             assets.TotalProjects,
		PortfolioHistoryCount:    assets.PortfolioSummary.HistoryCount,
		PortfolioTotalsAvailable: assets.PortfolioSummary.TotalRealEstateValueExclVATTRY != nil || assets.PortfolioSummary.TotalRealEstateValueInclVATTRY != nil,
	}
	if !assets.Computed {
		out.ValuationLinkedStatus = "asset_inventory_missing"
		out.RequiredChecks = append(out.RequiredChecks, "KAP PDF asset extraction çıktısı üretilmeli.")
		return out
	}
	out.Findings = append(out.Findings,
		fmt.Sprintf("%d asset event işlendi; %d birleşik envanter satırı ve %d rapor satırı üretildi.", assets.EventCount, assets.RawAssetCount, assets.DisplayAssetCount),
		fmt.Sprintf("%d tarihsel portföy toplamı satırı saklandı.", assets.PortfolioSummary.HistoryCount),
	)
	if isGYO && assets.TotalRentalAssets == 0 {
		out.Findings = append(out.Findings, "Kira üreten varlık güvenilir alanlardan eşlenemedi; rapordaki 'Kira: Yok' kira yokluğu kanıtı değil, veri eşleme boşluğudur.")
	}
	if out.PortfolioTotalsAvailable {
		out.ValuationLinkedStatus = "portfolio_totals_available_but_nav_bridge_required"
	} else if isGYO {
		out.ValuationLinkedStatus = "not_linked_to_valuation_portfolio_totals_missing"
	} else {
		out.ValuationLinkedStatus = "asset_inventory_reference_only"
	}
	if isGYO {
		out.RequiredChecks = []string{
			"Her büyük varlık için mülkiyetin devam edip etmediği, satılıp satılmadığı ve ipotek/rehin durumu kontrol edilmeli.",
			"Son ekspertiz tarihi, KDV hariç/değer dahil ayrımı ve portföy ağırlığı varlık bazında mutabık hale getirilmeli.",
			"Kira kontratı, kiracı, döviz bazlı kira, doluluk ve yıllık kira geliri ayrı doğrulanmalı.",
			"Geçmiş KAP'ta olup son bilançoda görünmeyen varlıklar için çıkış/satış olayı aranmalı.",
		}
	} else {
		out.RequiredChecks = []string{
			"KAP olayları faaliyet modeli, finansal tablo ve yönetim açıklamalarıyla aynı dönem için mutabık hale getirilmeli.",
			"Sektöre özgü ana gelir, maliyet, borç ve nakit akışı sürücüleri KAP/faliyet raporu kanıtıyla eşlenmeli.",
		}
	}
	return out
}

func buildFinancialQualityBridge(valuation ValuationAnalysis, sectorFinancials SectorFinancialAnalysis, valueReport value.Report, isGYO bool) FinancialQualityBridge {
	isBank := isBankValuationModel(valuation, valueReport)
	out := FinancialQualityBridge{}
	addMetric := func(name string, value float64, unit string, status string, comment string) {
		out.Metrics = append(out.Metrics, FinancialQualityMetric{Name: name, Value: value, Unit: unit, Status: status, Comment: comment})
	}
	netIncomeComment := "Net kâr pozitif/negatif ayrımı tek başına yeterli değildir; faaliyet kârı, finansman gideri ve tek seferlik kalemler ayrıca ayrılmalıdır."
	roeComment := "Özkaynak kârlılığı sermaye verimliliği ve defter değerinin kalitesi için ana sinyaldir."
	leverageComment := "Finansman riski, faiz hassasiyeti ve bilanço dayanıklılığı için kullanılır."
	pbComment := "PD/DD sektöre göre yorumlanır; özkaynak kalitesi ve sürdürülebilir ROE ile birlikte okunmalıdır."
	needToExplain := []string{
		"Net kârın ne kadarı operasyonel faaliyetlerden, ne kadarı finansman veya tek seferlik kalemlerden geliyor?",
		"Finansman gideri ve borç vade yapısı gelecek dönem kârını nasıl etkiler?",
		"Temettü veya büyüme kapasitesi nakit akışı ve işletme sermayesiyle destekleniyor mu?",
	}
	if isGYO {
		netIncomeComment = "Net kâr pozitif/negatif ayrımı tek başına yeterli değildir; GYO'da yeniden değerleme etkisi ayrıca ayrılmalıdır."
		roeComment = "Özkaynak kârlılığı defter/NAD iskontosu için ana kalite sinyalidir."
		leverageComment = "Finansman riski ve portföy değer düşüşlerine dayanıklılık için kullanılır."
		pbComment = "GYO'da NAD mutabakatı yoksa yalnızca defter değeri proxy olarak okunur."
		needToExplain = []string{
			"Net kârın ne kadarı nakit faaliyetlerden, ne kadarı yeniden değerleme veya tek seferlik kalemlerden geliyor?",
			"Finansman gideri ve borç vade yapısı gelecek dönem kârını nasıl etkiler?",
			"Temettü kapasitesi nakit akışı ve portföy satış/kira gelirleriyle destekleniyor mu?",
		}
	} else if isBank {
		netIncomeComment = "Bankada net kâr net faiz geliri, ücret-komisyon geliri, ticari kâr/zarar, karşılık giderleri ve vergi etkisiyle köprülenmelidir."
		roeComment = "Özkaynak kârlılığı banka değerlemesinde ana sinyaldir; sermaye yeterliliği ve aktif kalitesiyle birlikte okunmalıdır."
		pbComment = "Bankada PD/DD sürdürülebilir ROE, sermaye tamponu ve aktif kalitesiyle birlikte yorumlanır."
		needToExplain = []string{
			"Net kârın ne kadarı net faiz geliri, ücret-komisyon geliri, ticari kâr/zarar ve tek seferlik gelirlerden geliyor?",
			"Karşılık giderleri, takipteki kredi oranı ve karşılık kapsamı kârlılığı nasıl etkiliyor?",
			"Sermaye yeterlilik rasyosu, çekirdek sermaye oranı ve likidite karşılama oranı güvenli mi?",
			"Mevduat maliyeti, kredi/mevduat spread'i ve TL/YP açık pozisyonu kârlılığı nasıl etkiliyor?",
		}
	}
	addMetric("Net kar TTM", valuation.NetIncomeTTM, "TRY", signStatus(valuation.NetIncomeTTM, 0), netIncomeComment)
	if !isBank {
		addMetric("Serbest nakit akımı TTM", valuation.FreeCashFlowTTM, "TRY", signStatus(valuation.FreeCashFlowTTM, 0), "Nakit akışı kalitesi temettü ve borç ödeme kapasitesi için izlenir.")
	}
	if roe, ok := valuation.Ratios["ROE"]; ok {
		addMetric("ROE", roe*100, "%", returnMetricStatus(roe), roeComment)
	}
	if isBank {
		if roa, ok := valuation.Ratios["ROA"]; ok {
			addMetric("ROA", roa*100, "%", returnMetricStatus(roa), "Aktif kârlılığı banka bilançosunda kâr kalitesi ve aktif verimliliği için izlenir.")
		}
	}
	if netDebtToEquity, ok := valuation.Ratios["NetDebt_Eq"]; ok && !valuationRatioSuppressed(valuation, "NetDebt_Eq") {
		addMetric("Net borç/özsermaye", netDebtToEquity, "x", researchLeverageStatus(netDebtToEquity), leverageComment)
	} else if valuationRatioSuppressed(valuation, "NetDebt_Eq") && !isBank {
		out.RedFlags = append(out.RedFlags, "net_debt_to_equity_not_applicable_for_financial_profile")
	}
	if pb, ok := valuation.Ratios["PB"]; ok {
		addMetric("PD/DD", pb, "x", neutralMetricStatus(pb), pbComment)
	}
	if isBank {
		if pe, ok := valuation.Ratios["PE"]; ok {
			addMetric("F/K", pe, "x", neutralMetricStatus(pe), "F/K tek başına yeterli değildir; kârın karşılık ve tek seferlik gelirlerden arındırılmış sürdürülebilirliği kontrol edilmelidir.")
		}
		out.RedFlags = append(out.RedFlags, "bank_specific_regulatory_metrics_missing")
	}
	out.RedFlags = append(out.RedFlags, valuation.Flags...)
	out.RedFlags = append(out.RedFlags, valueReport.Warnings...)
	if sectorFinancials.Applicable {
		out.Summary = sectorFinancials.Summary
		out.RedFlags = append(out.RedFlags, sectorFinancials.Warnings...)
	}
	if out.Summary == "" {
		if isBank {
			out.Summary = "Finansal kalite; kârlılık, özkaynak getirisi, aktif kalitesi, fonlama maliyeti, likidite ve sermaye yeterliliği üzerinden okunur."
		} else {
			out.Summary = "Finansal kalite; kârlılık, nakit akışı, kaldıraç ve özkaynak getirisi üzerinden okunur."
		}
	}
	out.NeedToExplain = needToExplain
	if isGYO {
		out.NeedToExplain = append(out.NeedToExplain,
			"NAD iskontosu/primi güncel ekspertiz değerleri, nakit, borç ve ertelenmiş vergiyle hesaplanmalı.",
			"Kira gelirlerinin sürdürülebilirliği, doluluk ve döviz/TL kontrat kırılımı açıklanmalı.",
		)
	}
	out.RedFlags = uniqueStrings(out.RedFlags)
	return out
}

func valuationRatioSuppressed(valuation ValuationAnalysis, key string) bool {
	for _, suppressed := range valuation.SuppressedRatios {
		if suppressed == key {
			return true
		}
	}
	return false
}

func buildDecisionFramework(lastClose float64, valueReport value.Report, valuation ValuationAnalysis, assets KAPAssetInventorySummary, scenarios []Scenario, bridge ValuationTransparency) DecisionFramework {
	decision := firstNonEmpty(valueReport.DecisionLabel, "Karar hesaplanamadı")
	isGYO := bridge.NAVStatus != "" && bridge.NAVStatus != "not_applicable"
	isBank := isBankValuationModel(valuation, valueReport)
	out := DecisionFramework{CurrentDecision: decision}
	if bridge.BaseIntrinsicValue > 0 {
		out.DecisionBasis = append(out.DecisionBasis, fmt.Sprintf("Son fiyat %.2f, baz içsel değer %.2f; fiyat/baz değer farkı %.1f%%.", lastClose, bridge.BaseIntrinsicValue, bridge.PriceToBasePct))
	}
	if valueReport.MarginOfSafety.Computed {
		out.DecisionBasis = append(out.DecisionBasis, fmt.Sprintf("Güvenlik marjı %.1f%%; gereken marj %.1f%%.", valueReport.MarginOfSafety.BasePct, valueReport.MarginOfSafety.RequiredPct))
	}
	if isBank && (valuation.Ratios["ROE"] < 0 || valuation.NetIncomeTTM < 0) {
		out.DecisionBasis = append(out.DecisionBasis, "Banka finansal kalitesi zayıf: negatif kârlılık/ROE sinyali sermaye yeterliliği ve aktif kalitesiyle birlikte doğrulanmalı.")
	} else if !isBank && (valuation.Ratios["ROE"] < 0 || valuation.NetIncomeTTM < 0 || valuation.FreeCashFlowTTM < 0) {
		out.DecisionBasis = append(out.DecisionBasis, "Finansal kalite zayıf: negatif kârlılık/FCF/ROE sinyali kararın önüne geçiyor.")
	}
	if isGYO && assets.Computed && assets.PortfolioSummary.TotalRealEstateValueExclVATTRY == nil && assets.PortfolioSummary.TotalRealEstateValueInclVATTRY == nil {
		out.DecisionBasis = append(out.DecisionBasis, "KAP envanteri var fakat güncel portföy toplamı/NAD köprüsü güvenilir şekilde bağlanamadı.")
	}
	if bridge.BuyBelowPrice > 0 {
		out.BuyConditions = append(out.BuyConditions, fmt.Sprintf("Fiyat %.2f altına iner veya baz içsel değer yukarı revize olur ve gerekli güvenlik marjı sağlanır.", bridge.BuyBelowPrice))
	}
	if isBank {
		out.BuyConditions = append(out.BuyConditions,
			"Sermaye yeterlilik rasyosu, çekirdek sermaye, NPL, karşılık kapsamı, NIM ve LCR structured veriyle doğrulanır.",
			"Net faiz geliri ve net ücret-komisyon gelirleri sürdürülebilir büyüme gösterir; ticari kâr/zarar ve tek seferlik gelir etkisi ayrıştırılır.",
			"Teknik tarafta baskın trend ve hacim fiyatı destekler; AL/SAT kullanım kapısı kalite kontrolünden geçer.",
		)
		out.HoldConditions = append(out.HoldConditions,
			"Fiyat içsel değer çevresinde kalır fakat güvenlik marjı oluşmaz.",
			"Banka metrikleri kısmen olumlu olsa da NPL/NIM/sermaye yeterliliği ve fonlama maliyeti kanıtı eksik kalır.",
		)
		out.SellConditions = append(out.SellConditions,
			"Ana destek altında kapanış ve değerleme varsayımlarını bozan sermaye/aktif kalitesi gelişmesi birlikte görülür.",
			"ROE düşerken NPL, karşılık gideri, mevduat maliyeti veya kur riski yükselir.",
			"Özkaynak kalitesini bozan bedelli sermaye artışı veya tek seferlik gelir bağımlılığı ortaya çıkar.",
		)
		out.Invalidation = append(out.Invalidation,
			"SYR, çekirdek sermaye, NPL, NIM, LCR ve kredi/mevduat spread'i bağlanamazsa banka analizi ön analiz seviyesinde kalır.",
			"Finansal tablo yayın tarihi veya kaynak güvenliği doğrulanamazsa geriye dönük test ve karar güveni düşürülür.",
		)
	} else if isGYO {
		out.BuyConditions = append(out.BuyConditions,
			"NAD mutabakatı güncel ekspertiz, nakit, borç ve ertelenmiş vergiyle tamamlanır.",
			"Kira gelirleri/doluluk ve nakit akışı kalitesi sürdürülebilir görünür.",
			"Teknik tarafta baskın trend ve hacim fiyatı destekler; işlem planı kalite kapısından geçer.",
		)
		out.HoldConditions = append(out.HoldConditions,
			"Fiyat içsel değer çevresinde kalır fakat güvenlik marjı oluşmaz.",
			"KAP varlık doğrulaması ilerler ama kârlılık/nakit akışı teyidi henüz gelmez.",
		)
		out.SellConditions = append(out.SellConditions,
			"Ana destek altında kapanış ve değerleme varsayımlarını bozan KAP/bilanço gelişmesi birlikte görülür.",
			"NAD/defter değeri primlenirken ROE, nakit akışı ve kira görünümü zayıf kalır.",
			"Büyük varlık satışı, kira kaybı, yüksek borçlanma veya sermaye artırımı mevcut değeri seyreltir.",
		)
		out.Invalidation = append(out.Invalidation,
			"Varlık envanteri ile son bilanço arasında mutabakat kurulamazsa GYO/NAD yorumu sınırlı kabul edilir.",
			"Finansal tablo yayın tarihi veya kaynak güvenliği doğrulanamazsa geriye dönük test ve karar güveni düşürülür.",
		)
	} else {
		out.BuyConditions = append(out.BuyConditions,
			"Operasyonel kârlılık, nakit dönüşümü ve borçluluk aynı anda iyileşir.",
			"Sektör emsallerine göre iskonto kaynaklı, sürdürülebilir ve kanıtlı hale gelir.",
			"Teknik tarafta baskın trend ve hacim fiyatı destekler; işlem planı kalite kapısından geçer.",
		)
		out.HoldConditions = append(out.HoldConditions,
			"Fiyat içsel değer çevresinde kalır fakat güvenlik marjı oluşmaz.",
			"Kârlılık/nakit akışı teyidi henüz gelmez ama bilanço riski kötüleşmez.",
		)
		out.SellConditions = append(out.SellConditions,
			"Ana destek altında kapanış ve değerleme varsayımlarını bozan KAP/bilanço gelişmesi birlikte görülür.",
			"ROE, nakit akışı ve faaliyet marjları zayıf kalırken piyasa çarpanları primli hale gelir.",
			"Yüksek borçlanma, sermaye artırımı veya operasyonel zarar mevcut değeri seyreltir.",
		)
		out.Invalidation = append(out.Invalidation,
			"Finansal tablo yayın tarihi veya kaynak güvenliği doğrulanamazsa geriye dönük test ve karar güveni düşürülür.",
			"Operasyonel kârlılık ve nakit akışı varsayımları bozulursa yatırım tezi geçersizleşir.",
		)
	}
	if len(scenarios) == 3 {
		out.DecisionBasis = append(out.DecisionBasis, "Senaryolar şartlıdır; hedefler kesin fiyat tahmini olarak kullanılmaz.")
	}
	return out
}

func buildInvestmentStory(profile CompanyProfile, valuation ValuationAnalysis, kapPDF KAPPDFIngestSummary, assets KAPAssetInventorySummary, valueReport value.Report, financial FinancialQualityBridge, decision DecisionFramework, isGYO bool) InvestmentStory {
	out := InvestmentStory{}
	isBank := isBankProfile(profile, valuation, valueReport)
	if isBank {
		out.ValueSource = fmt.Sprintf(
			"Değerin ana kaynağı banka özkaynağı, sürdürülebilir ROE, aktif kalitesi ve fonlama gücüdür. Mevcut resmi içsel değer seti %.2f baz değeri, %.2f güncel fiyatı ve %.1f%% güvenlik marjını kullanır; peer çarpanları yalnızca kontrol setidir. Klasik FCF, net borç/özsermaye, FD/Satış ve FD/FAVÖK bankada ana değerleme kanıtı değildir.",
			valueReport.IntrinsicValue.Base,
			valueReport.CurrentPrice,
			valueReport.MarginOfSafety.BasePct,
		)
		out.MispricingQuestion = "Piyasa bankanın sürdürülebilir ROE'sini, aktif kalitesini, sermaye tamponunu ve mevduat maliyetini doğru fiyatlıyor mu?"
		out.CoreThesis = "Karar; içsel değer/güvenlik marjı, sürdürülebilir ROE, sermaye yeterliliği, NPL/NIM/LCR metrikleri ve teknik teyidin birlikte geçmesine bağlıdır."
	} else if isGYO {
		netDebtToEquity := valuation.Ratios["NetDebt_Eq"]
		if netDebtToEquity == 0 {
			netDebtToEquity = valuation.Ratios["NetDebtToEquity"]
		}
		out.ValueSource = fmt.Sprintf(
			"Değerin ana kaynağı GYO portföyüdür: %d birleşik varlık satırı, %d rapor satırı ve %d tarihsel portföy toplamı işlendi. Güncel ekspertiz toplamı/NAD köprüsü tam bağlanmadığı için değerleme defter değeri/NAD proxy seviyesinde kalır; baz içsel değer %.2f, güncel fiyat %.2f, güvenlik marjı %.1f%% ve gereken marj %.1f%%. Kira/doluluk %d varlıkta güvenilir eşleşti; bu sayı düşükse kira potansiyeli kararın ana dayanağı değil, tamamlanması gereken kanıttır. Borçluluk tarafında net borç/özsermaye %.2fx izlenir; portföy değeri, kira üretimi ve borç dengesi aynı anda doğrulanmadan güçlü AL tezi kurulmaz.",
			assets.RawAssetCount,
			assets.DisplayAssetCount,
			assets.PortfolioSummary.HistoryCount,
			valueReport.IntrinsicValue.Base,
			valueReport.CurrentPrice,
			valueReport.MarginOfSafety.BasePct,
			valueReport.MarginOfSafety.RequiredPct,
			assets.TotalRentalAssets,
			netDebtToEquity,
		)
		out.MispricingQuestion = "Piyasa portföy varlık değerini mi, yoksa zayıf kârlılık/nakit akışı ve veri belirsizliğini mi fiyatlıyor?"
		out.CoreThesis = "Şirketin değeri esas olarak GYO portföyünün kalitesi ve bu portföyün güncel NAD'a dönüşme gücünden gelir; mevcut rapor bunu proxy seviyesinde izler, tam NAD raporu değildir."
	} else {
		out.ValueSource = "Faaliyet kârlılığı, nakit akışı, sermaye getirisi ve emsal değerleme."
		out.MispricingQuestion = "Piyasa şirketin sürdürülebilir kazanç gücünü ve bilanço riskini doğru fiyatlıyor mu?"
		out.CoreThesis = "Karar, içsel değer/güvenlik marjı ile finansal kalite ve teknik teyidin birlikte geçmesine bağlıdır."
	}
	out.KeyEvidence = append(out.KeyEvidence,
		fmt.Sprintf("KAP PDF kapsamı: %d benzersiz metin, %d analize uygun, %d analiz dışı (%d review, %d rejected), %d OCR fallback.", kapPDF.TotalDocuments, kapPDF.AnalysisUsableCount, kapPDF.ReviewRequiredCount, kapPDFReviewOnlyCount(kapPDF), kapPDF.RejectedCount, kapPDF.OCRUsedCount),
		fmt.Sprintf("Değerleme: model=%s, baz içsel değer=%.2f, karar=%s.", valueReport.SectorModel.Label, valueReport.IntrinsicValue.Base, valueReport.DecisionLabel),
	)
	if isBank {
		out.KeyEvidence = append(out.KeyEvidence, fmt.Sprintf("KAP varlık envanteri referans bilgi: %d event, %d birleşik satır, %d rapor satırı; ana banka değerleme girdisi değildir.", assets.EventCount, assets.RawAssetCount, assets.DisplayAssetCount))
	} else {
		out.KeyEvidence = append(out.KeyEvidence, fmt.Sprintf("Varlık envanteri: %d event, %d birleşik satır, %d rapor satırı.", assets.EventCount, assets.RawAssetCount, assets.DisplayAssetCount))
	}
	out.KeyRisks = append(out.KeyRisks, financial.RedFlags...)
	out.KeyRisks = append(out.KeyRisks, decision.Invalidation...)
	if isBank {
		out.Catalysts = []string{
			"Sermaye yeterlilik rasyosu, çekirdek sermaye ve LCR verilerinin structured olarak bağlanması.",
			"NPL, karşılık giderleri ve karşılık kapsamındaki eğilimin netleşmesi.",
			"Net faiz marjı, mevduat maliyeti ve net ücret-komisyon gelirlerinde sürdürülebilir iyileşme.",
			"Teknik kırılımın hacim ve kapanışla teyit edilmesi.",
		}
	} else if isGYO {
		out.Catalysts = []string{
			"Güncel ekspertiz/NAD tablosunun güvenilir şekilde açıklanması veya çıkarılması.",
			"Kira gelirleri, doluluk ve döviz bazlı kontrat görünümünde iyileşme.",
			"Borçluluk/finansman gideri riskinin azalması.",
			"Teknik kırılımın hacim ve kapanışla teyit edilmesi.",
		}
	} else {
		out.Catalysts = []string{
			"Operasyonel kârlılığın ve brüt/faaliyet marjının toparlanması.",
			"Finansman gideri ve borçluluk baskısının azalması.",
			"Sermaye artırımı, sulanma ve nakit ihtiyacı riskinin netleşmesi.",
			"Teknik kırılımın hacim ve kapanışla teyit edilmesi.",
		}
	}
	if strings.TrimSpace(profile.PeerGroup) != "" {
		out.KeyEvidence = append(out.KeyEvidence, "Peer grup: "+profile.PeerGroup)
	}
	return out
}

func kapPDFReviewOnlyCount(kapPDF KAPPDFIngestSummary) int {
	reviewOnly := kapPDF.ReviewRequiredCount - kapPDF.RejectedCount
	if reviewOnly < 0 {
		return kapPDF.ReviewRequiredCount
	}
	return reviewOnly
}

func buildInstitutionalMemo(readiness []InvestorReadiness, bridge ValuationTransparency, assets AssetDueDiligence, financial FinancialQualityBridge, decision DecisionFramework, kapPDF KAPPDFIngestSummary, valueReport value.Report, isGYO bool) InstitutionalMemo {
	score := institutionalReadinessScore(readiness)
	memo := InstitutionalMemo{
		Decision:               firstNonEmpty(decision.CurrentDecision, valueReport.DecisionLabel, "Karar hesaplanamadı"),
		ReadinessScore:         score,
		InvestmentHorizon:      "6-18 ay; katalist ve finansal tablo doğrulaması tamamlanmadan daha kısa vadeli işlem planı ayrı yönetilmeli.",
		LiquidityConsideration: "Pozisyon büyüklüğü günlük ortalama hacim ve emir etkisi kontrol edilmeden artırılmamalı.",
		PortfolioFit:           "Tekil hisse fikri olarak izlenebilir; portföy rolü beta, likidite, sektör yoğunluğu ve korelasyon ölçülmeden nihai değildir.",
		PositiveSignals:        []string{},
	}
	memo.ExpectedReturnPct = computeReturnPct(bridge.CurrentPrice, bridge.BaseIntrinsicValue)
	memo.DownsideRiskPct = computeReturnPct(bridge.CurrentPrice, bridge.BearIntrinsicValue)
	memo.RiskRewardRatio = committeeRiskReward(memo.ExpectedReturnPct, memo.DownsideRiskPct)
	if kapPDF.TotalDocuments > 0 {
		memo.PositiveSignals = append(memo.PositiveSignals, fmt.Sprintf("%d KAP PDF metni işlendi; %d belge analiz kanıtı olarak kullanılabilir.", kapPDF.TotalDocuments, kapPDF.AnalysisUsableCount))
	}
	if isGYO && assets.InventoryComputed {
		memo.PositiveSignals = append(memo.PositiveSignals, fmt.Sprintf("%d varlık event'i ve %d raporlanabilir varlık satırı üretildi.", assets.EventCount, assets.DisplayAssetCount))
	}
	if bridge.BaseIntrinsicValue > 0 {
		memo.PositiveSignals = append(memo.PositiveSignals, fmt.Sprintf("Baz içsel değer %.2f, fiyat/baz farkı %.1f%% olarak hesaplandı.", bridge.BaseIntrinsicValue, bridge.PriceToBasePct))
	}

	if !decisionAllowsDirectBuy(memo.Decision) {
		memo.BlockingIssues = append(memo.BlockingIssues, "rapor_karari_dogrudan_al_degildir")
		memo.RequiredFixes = append(memo.RequiredFixes, "Kararı AL'a çevirecek koşullar ayrı ayrı kanıtlanmalı; fiyat, güvenlik marjı ve veri kalitesi aynı anda geçmelidir.")
	}
	if bridge.NAVStatus != "not_applicable" && bridge.NAVBridge.Status != "full_nav_reconciled" {
		memo.BlockingIssues = append(memo.BlockingIssues, "tam_nad_mutabakati_yok")
		memo.RequiredFixes = append(memo.RequiredFixes, "Güncel ekspertiz toplamı, nakit, borç, ertelenmiş vergi, azınlık payı ve hisse sayısıyla hisse başına NAD köprüsü kurulmalı.")
	}
	if isGYO && assets.InventoryComputed && assets.RentalAssetCount == 0 {
		memo.BlockingIssues = append(memo.BlockingIssues, "kira_doluluk_eslesmesi_yok")
		memo.RequiredFixes = append(memo.RequiredFixes, "Her büyük varlık için kiracı, kira tutarı, para birimi, kontrat süresi, doluluk oranı ve yıllık gelir çıkarılmalı.")
	}
	if isGYO && assets.InventoryComputed && assets.ValuationLinkedStatus != "portfolio_totals_available_but_nav_bridge_required" {
		memo.BlockingIssues = append(memo.BlockingIssues, "varlik_envanteri_degerlemeye_tam_bagli_degildir")
		memo.RequiredFixes = append(memo.RequiredFixes, "Varlık listesi güncel portföy toplamı, portföy ağırlığı, mülkiyet durumu ve son bilanço kalemleriyle mutabık hale getirilmeli.")
	}
	if len(financial.RedFlags) > 0 {
		memo.BlockingIssues = append(memo.BlockingIssues, "finansal_kalite_kirmizi_bayraklari_var")
		fix := "Net kâr, faaliyet kârı, finansman gideri, nakit akışı ve tek seferlik kalemler ayrıştırılmalı."
		if isGYO {
			fix = "Net kâr, yeniden değerleme etkisi, faaliyet kârı, finansman gideri, nakit akışı ve vergi etkisi ayrıştırılmalı."
		}
		memo.RequiredFixes = append(memo.RequiredFixes, fix)
	}
	if kapPDF.TotalDocuments > 0 && kapPDF.AnalysisUsableCount == 0 {
		memo.BlockingIssues = append(memo.BlockingIssues, "pdf_parse_kalite_kapisi_tam_gecilmedi")
		memo.RequiredFixes = append(memo.RequiredFixes, "Review/rejected PDF'ler OCR, manuel örnekleme veya KAP servis verisiyle doğrulanmadan finansal kanıt olarak kullanılmamalı.")
	} else if kapPDF.RejectedCount > 0 || kapPDF.ReviewRequiredCount > 0 {
		memo.RequiredFixes = append(memo.RequiredFixes, "Review/rejected PDF'ler kalite raporunda izlenmeli; ana yatırım metrikleri analize uygun veya sertifikalı belgelerden alınmalı.")
	}
	if valueReport.MarginOfSafety.Computed && valueReport.MarginOfSafety.BasePct < valueReport.MarginOfSafety.RequiredPct {
		memo.BlockingIssues = append(memo.BlockingIssues, "gereken_guvenlik_marji_yok")
		memo.RequiredFixes = append(memo.RequiredFixes, "Baz içsel değer yukarı doğrulanmalı veya fiyat gereken güvenlik marjını sağlayacak seviyeye gerilemeli.")
	}
	if valueReport.Confidence > 0 && valueReport.Confidence < 70 {
		memo.BlockingIssues = append(memo.BlockingIssues, "model_guveni_kurumsal_esigin_altinda")
		memo.RequiredFixes = append(memo.RequiredFixes, "Değerleme varsayımları baz/ayı/boğa senaryoları, hassasiyet analizi ve kaynak kanıtlarıyla yeniden valide edilmeli.")
	}
	if valueReport.BuffettChecklist.Computed && valueReport.BuffettChecklist.Status != "pass" {
		memo.BlockingIssues = append(memo.BlockingIssues, "buffett_deger_yatirimi_filtresi_gecmedi")
		if len(valueReport.BuffettChecklist.MissingData) > 0 {
			memo.RequiredFixes = append(memo.RequiredFixes, "Buffett filtresi için eksik veri tamamlanmalı: "+strings.Join(valueReport.BuffettChecklist.MissingData, ", ")+".")
		} else {
			memo.RequiredFixes = append(memo.RequiredFixes, "Buffett filtresinde başarısız/sınırlı kalan kriterler finansal tablo, nakit akışı, sermaye tahsisi ve güvenlik marjı kanıtıyla düzeltilmeli.")
		}
	}

	memo.KeyAssumptions = uniqueStrings(append(append([]string{}, bridge.PrimaryInputs...), bridge.MissingInputs...))
	if len(memo.KeyAssumptions) == 0 {
		memo.KeyAssumptions = []string{
			"Fiyat, finansal tablo ve KAP/PDF kaynakları aynı dönem ve para birimiyle mutabık kabul edilmiştir.",
			"Baz içsel değer mevcut veri kalitesiyle geçici referanstır; tam kanıt kapısı geçmeden hedef fiyat değildir.",
		}
	}
	memo.ApprovalConditions = uniqueStrings(append(append([]string{}, decision.BuyConditions...), []string{
		"Kaynak kanıtı olmayan ana metrik yatırım kararında ana dayanak yapılmamalı.",
		"Beklenen getiri, aşağı yön riskini ve gerekli güvenlik marjını açık şekilde aşmalı.",
	}...))
	memo.RejectionConditions = uniqueStrings(append(append([]string{}, decision.SellConditions...), decision.Invalidation...))
	if len(memo.RejectionConditions) == 0 {
		memo.RejectionConditions = []string{"Kanıt, değerleme veya risk koşulları komite eşiğini karşılamazsa fikir reddedilmeli veya izleme listesinde kalmalı."}
	}

	questions := []string{
		"Negatif kârlılık/FCF varsa bunun operasyonel mi, finansman kaynaklı mı, tek seferlik mi olduğu ayrıştı mı?",
		"AL kararı için fiyat, güvenlik marjı ve teknik teyit aynı anda hangi koşulda geçiyor?",
	}
	if isGYO {
		questions = append([]string{
			"Hisse başına tam NAD kaç TL ve piyasa değeri bu NAD'a göre iskonto mu prim mi taşıyor?",
			"Portföydeki her büyük varlık bugün hâlâ şirkete ait mi, ipotek/rehin veya satış süreci var mı?",
			"Kira gelirleri hangi varlıklardan, hangi para birimiyle ve hangi kontrat süresiyle geliyor?",
		}, questions...)
	} else {
		questions = append([]string{
			"Gelir büyümesi, brüt marj, faaliyet marjı ve nakit dönüşümü sürdürülebilir mi?",
			"Borçluluk, faiz gideri ve sermaye artırımı riski özkaynak getirisini nasıl etkiliyor?",
			"Sektör emsallerine göre iskonto/premium hangi operasyonel farkla açıklanıyor?",
		}, questions...)
	}
	memo.CommitteeQuestions = uniqueStrings(append(questions, bridge.MissingInputs...))
	memo.BlockingIssues = uniqueStrings(memo.BlockingIssues)
	memo.RequiredFixes = uniqueStrings(memo.RequiredFixes)
	memo.PositiveSignals = uniqueStrings(memo.PositiveSignals)
	if len(memo.BlockingIssues) > 0 {
		score -= math.Min(35, float64(len(memo.BlockingIssues))*5)
	}
	memo.ReadinessScore = mathutil.Clamp(score, 0, 100)
	memo.DirectBuyEligible = memo.ReadinessScore >= 80 && len(memo.BlockingIssues) == 0 && decisionAllowsDirectBuy(memo.Decision)
	memo.InvestmentCommitteeReady = memo.ReadinessScore >= 70 && len(memo.BlockingIssues) <= 1
	memo.BrokeragePublishableReady = memo.ReadinessScore >= 75 && len(memo.BlockingIssues) == 0
	switch {
	case memo.DirectBuyEligible:
		memo.WorkflowStatus = "direct_buy_candidate"
	case memo.InvestmentCommitteeReady:
		memo.WorkflowStatus = "committee_review_candidate"
	case len(memo.PositiveSignals) > 0:
		memo.WorkflowStatus = "research_backlog_or_watchlist"
	default:
		memo.WorkflowStatus = "not_investable_from_current_report"
	}
	memo.Recommendation = normalizedCommitteeRecommendation(memo.Decision, memo.DirectBuyEligible, memo.ReadinessScore, memo.BlockingIssues)
	memo.PositionSizeSuggestion = committeePositionSizeSuggestion(memo.Recommendation, memo.ReadinessScore, memo.DirectBuyEligible, memo.InvestmentCommitteeReady, memo.BlockingIssues)
	return memo
}

func computeReturnPct(current, target float64) float64 {
	if current <= 0 || target <= 0 {
		return 0
	}
	return (target/current - 1) * 100
}

func committeeRiskReward(expectedReturnPct, downsideRiskPct float64) float64 {
	if expectedReturnPct <= 0 || downsideRiskPct >= 0 {
		return 0
	}
	return mathutil.Clamp(expectedReturnPct/math.Abs(downsideRiskPct), 0, 99)
}

func committeePositionSizeSuggestion(recommendation string, readiness float64, directBuy, committeeReady bool, blockers []string) string {
	if len(blockers) >= 6 || recommendation == "INSUFFICIENT_DATA" {
		return "0%; yatırım yapılmaz, yalnızca araştırma kuyruğu."
	}
	if recommendation == "REDUCE" || recommendation == "SELL" || recommendation == "AVOID" {
		return "Yeni pozisyon açılmaz; mevcut pozisyon varsa risk azaltma planı hazırlanır."
	}
	if !committeeReady || recommendation == "WATCH" {
		return "0%; takip listesi. Pilot pozisyon ancak manuel doğrulama sonrası değerlendirilmeli."
	}
	if directBuy && readiness >= 85 && (recommendation == "BUY" || recommendation == "STRONG_BUY") {
		return "Portföy risk bütçesine bağlı olarak küçük/orta ağırlık; tek emir öncesi likidite ve ADV kontrolü zorunlu."
	}
	if recommendation == "ACCUMULATE" || recommendation == "HOLD" {
		return "Mevcut pozisyon korunabilir veya kademeli düşük ağırlık düşünülebilir; yeni alım için onay koşulları izlenmeli."
	}
	return "Pozisyon büyüklüğü belirlenmedi; komite onayı ve likidite kontrolü gerekir."
}

func normalizedCommitteeRecommendation(decision string, directBuy bool, readiness float64, blockers []string) string {
	if len(blockers) >= 6 {
		return "INSUFFICIENT_DATA"
	}
	slug := util.SlugTR(decision)
	switch {
	case directBuy && (strings.Contains(slug, "guclu") || strings.Contains(slug, "strong")):
		return "STRONG_BUY"
	case directBuy || slug == "al" || strings.Contains(slug, "alim"):
		return "BUY"
	case strings.Contains(slug, "pahali") || strings.Contains(slug, "uzerinde"):
		return "AVOID"
	case strings.Contains(slug, "biriktir") || strings.Contains(slug, "accumulate"):
		return "ACCUMULATE"
	case strings.Contains(slug, "azalt") || strings.Contains(slug, "reduce"):
		return "REDUCE"
	case strings.Contains(slug, "sat") || strings.Contains(slug, "sell"):
		return "SELL"
	case strings.Contains(slug, "kacin") || strings.Contains(slug, "avoid"):
		return "AVOID"
	case containsString(blockers, "finansal_kalite_kirmizi_bayraklari_var") && containsString(blockers, "gereken_guvenlik_marji_yok"):
		return "AVOID"
	case len(blockers) > 0:
		return "WATCH"
	default:
		return "HOLD"
	}
}

func institutionalReadinessScore(readiness []InvestorReadiness) float64 {
	for _, item := range readiness {
		if item.Segment == "institutional_professional_decision" {
			return item.CoveragePct
		}
	}
	if len(readiness) == 0 {
		return 0
	}
	return readiness[len(readiness)-1].CoveragePct
}

func decisionAllowsDirectBuy(decision string) bool {
	slug := util.SlugTR(decision)
	return slug == "al" || strings.Contains(slug, "guclu al") || strings.Contains(slug, "alim")
}

func buildScoreExplanations(coverage CoverageReport, valueReport value.Report) []ScoreExplanation {
	return []ScoreExplanation{
		{Score: "Rapor güveni", Meaning: "Veri kapsamı, model tutarlılığı ve karar üretilebilirliğini ölçer; fiyat hedefi değildir.", GoodBad: "100 iyiye yakındır, ancak eksik veri varsa yüksek skor yatırım kararı anlamına gelmez.", Driver: fmt.Sprintf("Kapsam skoru %.0f/100, değerleme güveni %.0f/100.", coverage.Score, valueReport.Confidence)},
		{Score: "Değer yatırım kalitesi", Meaning: "İçsel değer, güvenlik marjı, nakit akışı ve sermaye tahsisi kalitesini ölçer.", GoodBad: "Yüksek skor ucuzluk değil; kaliteli veri ve makul değerleme birlikte aranır.", Driver: fmt.Sprintf("Kalite %.0f/100, değer skoru %.0f/100.", valueReport.QualityScore, valueReport.ValueScore)},
		{Score: "Model riski", Meaning: "Seçilen sektör modelinin gerçek iş modelini ne kadar temsil ettiğini anlatır.", GoodBad: "Düşük model riski iyidir; seçilen model gerçek iş modelini temsil etmiyorsa risk artar.", Driver: valueReport.SectorModel.Reason},
	}
}

func buildOpenResearchQuestions(bridge ValuationTransparency, assets AssetDueDiligence, financial FinancialQualityBridge, isGYO bool) []string {
	out := []string{}
	out = append(out, bridge.MissingInputs...)
	out = append(out, assets.RequiredChecks...)
	out = append(out, financial.NeedToExplain...)
	if isGYO {
		out = append(out, "Hisse başına NAD, piyasa değeri/NAD iskontosu ve portföy ağırlıkları açık tabloya dönüştürülmeli.")
	}
	return uniqueStrings(out)
}

func investmentResearchWarnings(review InvestmentResearchReview) []string {
	warnings := []string{}
	if review.ValuationBridge.NAVStatus == "nav_not_reconciled_portfolio_totals_missing" {
		warnings = append(warnings, "investment_research_nav_not_reconciled")
	}
	if review.ValuationBridge.NAVStatus != "" && review.ValuationBridge.NAVStatus != "not_applicable" && review.AssetDueDiligence.RentalAssetCount == 0 && review.AssetDueDiligence.InventoryComputed {
		warnings = append(warnings, "investment_research_rental_mapping_missing")
	}
	if len(review.FinancialQuality.RedFlags) > 0 {
		warnings = append(warnings, "investment_research_financial_quality_flags_present")
	}
	return uniqueStrings(warnings)
}

func investmentResearchSummary(review InvestmentResearchReview) string {
	readiness := ""
	if len(review.Readiness) >= 3 {
		readiness = fmt.Sprintf("yatırımcı yeterlilik tahmini: küçük yatırımcı %.0f/100, ciddi temel analiz %.0f/100, kurumsal %.0f/100",
			review.Readiness[0].CoveragePct,
			review.Readiness[1].CoveragePct,
			review.Readiness[2].CoveragePct,
		)
	}
	parts := []string{readiness, review.InvestmentStory.CoreThesis}
	if review.ValuationBridge.NAVStatus != "" && review.ValuationBridge.NAVStatus != "not_applicable" {
		parts = append(parts, "NAD durumu: "+review.ValuationBridge.NAVStatus)
	}
	return strings.Join(nonEmptyStrings(parts), "; ") + "."
}

func signStatus(value, threshold float64) string {
	if value > threshold {
		return "positive"
	}
	if value < threshold {
		return "negative"
	}
	return "neutral"
}

func returnMetricStatus(value float64) string {
	switch {
	case value >= 0.15:
		return "strong"
	case value >= 0.05:
		return "acceptable"
	case value < 0:
		return "weak"
	default:
		return "low"
	}
}

func researchLeverageStatus(value float64) string {
	switch {
	case math.Abs(value) < 0.01:
		return "neutral"
	case value <= 0.35:
		return "controlled"
	case value <= 1.0:
		return "watch"
	default:
		return "high_risk"
	}
}

func neutralMetricStatus(value float64) string {
	if value <= 0 {
		return "missing"
	}
	return "context_required"
}

func nonEmptyStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
