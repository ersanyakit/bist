package professional

import (
	"fmt"
	"strings"

	"hissebot/internal/ta/value"
)

const (
	strictCoverageThreshold          = 80.0
	strictValueDataQualityThreshold  = 70.0
	strictValueConfidenceThreshold   = 70.0
	strictClassificationThreshold    = 0.80
	strictKAPPDFMinUsableDocuments   = 20
	strictKAPPDFUsableRatioThreshold = 0.60
)

// ApplyStrictEvidencePolicy keeps factual source data in the report, but blocks
// valuation targets and recommendations unless the evidence chain is strong.
func ApplyStrictEvidencePolicy(report *Report) {
	if report == nil {
		return
	}
	policy := evaluateStrictEvidencePolicy(*report)
	report.EvidencePolicy = policy
	policyWarning := "strict_evidence_policy_active"
	if policy.Mode == "decision" {
		policyWarning = "decision_evidence_policy_active"
	}
	report.Coverage.Warnings = uniqueStrings(append(report.Coverage.Warnings, policyWarning))

	if !policy.ValuationTargetsAllowed {
		if isMarketOnlyStrictPolicyReport(*report) {
			markMarketOnlyValuationNotApplicable(report)
		} else {
			suppressValuationOutputs(report, policy)
		}
	}
	if !policy.ScenarioTargetsAllowed {
		report.Scenarios = nil
	}
	if !policy.RecommendationAllowed {
		suppressRecommendationOutputs(report, policy)
	}

	report.InvestmentResearch.Warnings = uniqueStrings(append(report.InvestmentResearch.Warnings, policy.BlockingIssues...))
	report.Coverage.Warnings = uniqueStrings(report.Coverage.Warnings)
	report.EvidencePolicy = policy
}

func evaluateStrictEvidencePolicy(report Report) EvidencePolicyReport {
	policy := EvidencePolicyReport{
		Mode:                 firstNonEmpty(report.DataGovernance.DataMode, "research"),
		Status:               "pass",
		Strict:               true,
		FactualDataAllowed:   len(report.Coverage.Available) > 0,
		TechnicalPlanAllowed: true,
		RequiredEvidence:     strictRequiredEvidence(),
		Notes: []string{
			"Kaynaklı ham ve normalize veri görünür kalabilir; model hedefi ve yatırım kararı için tüm kanıt kapıları geçmelidir.",
			"Kapı geçmezse rapor boşluğu varsayımla doldurmak yerine hangi verinin bağlanması gerektiğini açık yazmalıdır.",
		},
	}

	block := func(issue string) {
		if strings.TrimSpace(issue) == "" {
			return
		}
		policy.BlockingIssues = append(policy.BlockingIssues, issue)
	}

	marketOnlyAsset := isMarketOnlyStrictPolicyReport(report)
	if !marketOnlyAsset {
		if containsString(report.Coverage.Missing, "financial_statements") || !report.ValueInvesting.Computed || !report.ValueInvesting.IntrinsicValue.Computed {
			block("financial_statements_or_intrinsic_value_missing")
		}
	}
	if report.Coverage.Score < strictCoverageThreshold {
		block(fmt.Sprintf("coverage_score_below_%02.0f", strictCoverageThreshold))
	}
	if !marketOnlyAsset && report.ValueInvesting.Computed && report.ValueInvesting.DataQuality < strictValueDataQualityThreshold {
		block(fmt.Sprintf("value_data_quality_below_%02.0f", strictValueDataQualityThreshold))
	}
	if !marketOnlyAsset && report.ValueInvesting.Computed && report.ValueInvesting.Confidence < strictValueConfidenceThreshold {
		block(fmt.Sprintf("valuation_confidence_below_%02.0f", strictValueConfidenceThreshold))
	}
	if !report.DataGovernance.FinanciallyConsistent {
		block("financial_statements_not_reconciled")
	}
	if report.DataGovernance.ReconciliationFailureCount > 0 {
		block("financial_reconciliation_failures_present")
	}
	if !strictFinancialTimingEvidenceSafe(report.DataGovernance) {
		block("financial_available_at_or_publish_date_not_safe")
	}
	if report.DataGovernance.LineageEvents == 0 && strings.TrimSpace(report.DataGovernance.Source) == "" {
		block("financial_source_lineage_missing")
	}
	if !marketOnlyAsset {
		if !report.KAPPDFIngest.Computed || report.KAPPDFIngest.TotalDocuments == 0 {
			block("kap_pdf_ingest_missing")
		} else {
			if report.KAPPDFIngest.AnalysisUsableCount == 0 {
				block("kap_pdf_no_analysis_usable_documents")
			}
			if strictKAPPDFUsableEvidenceInsufficient(report.KAPPDFIngest) {
				block("kap_pdf_analysis_usable_ratio_below_60")
			}
		}
	}
	if !marketOnlyAsset {
		if report.Company.ClassificationConfidence <= 0 {
			block("sector_classification_missing")
		} else if report.Company.ClassificationConfidence < strictClassificationThreshold {
			block("sector_classification_confidence_below_strict_threshold")
		}
		if report.Peers.PeerCount < 3 {
			block("peer_sample_below_3")
		}
		if strictGYORequiresNAVBridge(report) {
			block("reit_nav_bridge_missing")
		}
		if strictBankCoreMetricsMissing(report) {
			block("bank_regulatory_metrics_missing")
		}
	}

	policy.BlockingIssues = uniqueStrings(policy.BlockingIssues)
	if policy.Mode == "decision" {
		return decisionEvidencePolicy(policy, marketOnlyAsset)
	}
	if marketOnlyAsset {
		policy.RequiredEvidence = marketOnlyRequiredEvidence(report)
		policy.ValuationTargetsAllowed = false
		policy.ScenarioTargetsAllowed = true
		policy.RecommendationAllowed = len(policy.BlockingIssues) == 0
		policy.SuppressedOutputs = append(policy.SuppressedOutputs, "intrinsic_value_target", "margin_of_safety")
		if !policy.RecommendationAllowed {
			policy.Status = "blocked"
			policy.SuppressedOutputs = append(policy.SuppressedOutputs,
				"investment_recommendation",
				"buy_sell_decision",
			)
		}
		policy.SuppressedOutputs = uniqueStrings(policy.SuppressedOutputs)
		return policy
	}
	policy.RequiredEvidence = strictRequiredEvidenceForIssues(policy.BlockingIssues)
	policy.ValuationTargetsAllowed = len(policy.BlockingIssues) == 0
	policy.ScenarioTargetsAllowed = policy.ValuationTargetsAllowed
	policy.RecommendationAllowed = len(policy.BlockingIssues) == 0
	if !policy.ValuationTargetsAllowed {
		policy.Status = "blocked"
		policy.SuppressedOutputs = append(policy.SuppressedOutputs,
			"fair_value_range",
			"intrinsic_value_target",
			"margin_of_safety",
			"scenario_price_targets",
		)
	}
	if !policy.RecommendationAllowed {
		policy.Status = "blocked"
		policy.SuppressedOutputs = append(policy.SuppressedOutputs,
			"investment_recommendation",
			"buy_sell_decision",
		)
	}
	policy.SuppressedOutputs = uniqueStrings(policy.SuppressedOutputs)
	return policy
}

// decisionEvidencePolicy separates a missing core input from a model
// limitation. A limitation must reduce confidence and remain visible, but it
// must not erase an otherwise computable valuation or turn the product into a
// data-acquisition report.
func decisionEvidencePolicy(policy EvidencePolicyReport, marketOnlyAsset bool) EvidencePolicyReport {
	blocking := []string{}
	limitations := []string{}
	for _, issue := range policy.BlockingIssues {
		if coreDecisionEvidenceIssue(issue) {
			blocking = append(blocking, issue)
		} else {
			limitations = append(limitations, issue)
		}
	}
	policy.BlockingIssues = uniqueStrings(blocking)
	policy.Limitations = uniqueStrings(limitations)
	policy.Strict = false
	policy.Status = "pass"
	if len(policy.BlockingIssues) > 0 {
		policy.Status = "blocked"
	}
	policy.ValuationTargetsAllowed = !marketOnlyAsset && len(policy.BlockingIssues) == 0
	policy.ScenarioTargetsAllowed = marketOnlyAsset || policy.ValuationTargetsAllowed
	policy.RecommendationAllowed = len(policy.BlockingIssues) == 0
	policy.RequiredEvidence = strictRequiredEvidenceForIssues(policy.BlockingIssues)
	policy.SuppressedOutputs = nil
	if marketOnlyAsset {
		policy.ValuationTargetsAllowed = false
		policy.SuppressedOutputs = append(policy.SuppressedOutputs, "intrinsic_value_target", "margin_of_safety")
	}
	if !policy.ValuationTargetsAllowed && !marketOnlyAsset {
		policy.SuppressedOutputs = append(policy.SuppressedOutputs, "fair_value_range", "intrinsic_value_target", "margin_of_safety", "scenario_price_targets")
	}
	if !policy.RecommendationAllowed {
		policy.SuppressedOutputs = append(policy.SuppressedOutputs, "investment_recommendation", "buy_sell_decision")
	}
	policy.SuppressedOutputs = uniqueStrings(policy.SuppressedOutputs)
	policy.Notes = []string{
		"Mevcut ve kaynaklı veriler doğrudan karar üretiminde kullanılır.",
		"Model sınırlamaları güveni düşürür ve raporda görünür kalır; yalnız çekirdek kanıt yokluğu karar/ hedef çıktısını bastırır.",
	}
	return policy
}

func coreDecisionEvidenceIssue(issue string) bool {
	issue = strings.ToLower(strings.TrimSpace(issue))
	switch {
	case strings.HasPrefix(issue, "coverage_score_below_"),
		issue == "financial_statements_or_intrinsic_value_missing",
		issue == "financial_statements_not_reconciled",
		issue == "financial_reconciliation_failures_present",
		issue == "financial_available_at_or_publish_date_not_safe",
		issue == "financial_source_lineage_missing",
		issue == "kap_pdf_ingest_missing",
		issue == "kap_pdf_no_analysis_usable_documents",
		issue == "sector_classification_missing",
		issue == "peer_sample_below_3":
		return true
	default:
		return false
	}
}

func strictFinancialTimingEvidenceSafe(gov FinancialDataGovernance) bool {
	if gov.BacktestSafe {
		return true
	}
	if gov.LatestAvailableAt != nil {
		if gov.AsOf.IsZero() || !gov.LatestAvailableAt.After(gov.AsOf) {
			return true
		}
	}
	if gov.LatestPublishDate != nil {
		if gov.AsOf.IsZero() || !gov.LatestPublishDate.After(gov.AsOf) {
			return true
		}
	}
	return false
}

func strictGYORequiresNAVBridge(report Report) bool {
	bridge := report.InvestmentResearch.ValuationBridge
	if bridge.NAVStatus == "" || bridge.NAVStatus == "not_applicable" {
		return false
	}
	if bridge.NAVBridge.Status == "partial_nav_proxy" || bridge.NAVBridge.Status == "full_nav_reconciled" {
		return false
	}
	return true
}

func strictBankCoreMetricsMissing(report Report) bool {
	text := strings.ToLower(strings.Join([]string{
		report.Company.Sector,
		report.Company.Industry,
		report.Valuation.SectorModel,
		report.SectorFinancials.Profile,
		report.SectorFinancials.ProfileLabel,
		report.ValueInvesting.SectorModel.Model,
	}, " "))
	if !strings.Contains(text, "bank") && !strings.Contains(text, "banka") {
		return false
	}
	if bankRegulatoryMetricsComplete(report.SectorFinancials) {
		return containsString(report.Valuation.Flags, "bank_book_value_reconciliation_missing") ||
			containsString(report.Valuation.Flags, "bank_book_value_reconciliation_failed")
	}
	if containsString(report.Valuation.Flags, bankRegulatoryValuationFlag) {
		return true
	}
	return true
}

func strictKAPPDFUsableEvidenceInsufficient(kap KAPPDFIngestSummary) bool {
	total := kap.TotalDocuments
	usable := kap.AnalysisUsableCount
	if kap.DecisionRelevantDocuments > 0 {
		total = kap.DecisionRelevantDocuments
		usable = kap.DecisionRelevantUsableCount
	}
	if total <= 0 || usable <= 0 {
		return true
	}
	if usable >= strictKAPPDFMinUsableDocuments {
		return false
	}
	usableRatio := float64(usable) / float64(total)
	return usableRatio < strictKAPPDFUsableRatioThreshold
}

func isMarketOnlyStrictPolicyReport(report Report) bool {
	return report.Company.SectorSource == "asset_type_crypto" || report.Company.SectorSource == "asset_type_commodity"
}

func markMarketOnlyValuationNotApplicable(report *Report) {
	if report == nil {
		return
	}
	v := report.ValueInvesting
	v.Computed = false
	v.Decision = "NOT_APPLICABLE"
	v.DecisionLabel = "Uygulanmaz"
	if v.Symbol == "" {
		v.Symbol = report.Company.Symbol
	}
	if v.Currency == "" {
		v.Currency = report.DataGovernance.Currency
	}
	if v.CurrentPrice <= 0 {
		v.CurrentPrice = report.Valuation.FairValue.Base
	}
	v.Summary = "Geleneksel şirket finansalları temelli içsel değer ve güvenlik marjı çerçevesi bu varlık sınıfına uygulanmaz."
	v.IntrinsicValue = value.IntrinsicValueReport{
		Computed: false,
		Method:   "not_applicable_market_only_asset",
		Drivers:  []string{"issuer_intrinsic_value_not_applicable_to_market_only_asset"},
	}
	requiredPct := v.MarginOfSafety.RequiredPct
	if requiredPct <= 0 {
		requiredPct = v.Assumptions.RequiredMarginPct
	}
	v.MarginOfSafety = value.MarginOfSafetyReport{
		Computed:    false,
		RequiredPct: requiredPct,
		Label:       "uygulanmaz",
	}
	v.FairValue = value.FairValueConclusion{
		Computed:     false,
		Status:       "not_applicable",
		Label:        "Uygulanmaz",
		CurrentPrice: v.CurrentPrice,
		Explanation:  "Spot kripto/emtia için şirket içsel değer hedefi hesaplanmaz; raporda fiyat, teknik yapı, likidite, makro, pozisyon ve akış verileri kullanılır.",
	}
	v.Checks = []value.Check{
		{
			Name:    "market_only_value_framework",
			Status:  "not_applicable",
			Message: "Şirket finansalı temelli değer yatırım çerçevesi market-only varlık için uygulanmaz.",
		},
	}
	v.Warnings = uniqueStrings(append(v.Warnings, "issuer_value_framework_not_applicable_to_market_only_asset"))
	report.ValueInvesting = v
	report.Valuation.Flags = uniqueStrings(report.Valuation.Flags)
}

func suppressValuationOutputs(report *Report, policy EvidencePolicyReport) {
	reason := strictEvidenceSummary(policy)
	report.Valuation.FairValue = FairValueRange{
		Drivers:    []string{"strict_evidence_policy_suppressed", reason},
		Confidence: 0,
	}
	report.Valuation.Flags = uniqueStrings(append(report.Valuation.Flags, "strict_evidence_policy_suppressed_valuation_targets"))
	if report.Valuation.DCF.Computed || len(report.Valuation.DCF.Warnings) > 0 {
		report.Valuation.DCF.Computed = false
		report.Valuation.DCF.FairValuePerShare = 0
		report.Valuation.DCF.EnterpriseValue = 0
		report.Valuation.DCF.EquityValue = 0
		report.Valuation.DCF.Warnings = uniqueStrings(append(report.Valuation.DCF.Warnings, "strict_evidence_policy_suppressed_dcf"))
	}

	required := strictRequiredEvidenceForIssues(policy.BlockingIssues)
	bridge := report.InvestmentResearch.ValuationBridge
	bridge.BaseIntrinsicValue = 0
	bridge.BearIntrinsicValue = 0
	bridge.BullIntrinsicValue = 0
	bridge.BuyBelowPrice = 0
	bridge.PriceToBasePct = 0
	bridge.Method = "suppressed_by_strict_evidence_policy"
	bridge.Formula = "Kanıt kapısı geçmediği için hedef fiyat, içsel değer ve güvenlik marjı hesaplanmaz."
	bridge.MissingInputs = uniqueStrings(append(bridge.MissingInputs, required...))
	bridge.Limitations = uniqueStrings(append(bridge.Limitations, reason))
	if bridge.NAVBridge.Status != "" && bridge.NAVBridge.Status != "not_applicable" {
		bridge.NAVBridge.Status = "suppressed_by_strict_evidence_policy"
		bridge.NAVBridge.DataQuality = "insufficient_for_nav_conclusion"
		bridge.NAVBridge.EstimatedNAVTRY = 0
		bridge.NAVBridge.EstimatedNAVPerShare = 0
		bridge.NAVBridge.MarketCapToNAVPremiumPct = 0
		bridge.NAVBridge.ReconciliationLimitations = uniqueStrings(append(bridge.NAVBridge.ReconciliationLimitations, reason))
	}
	report.InvestmentResearch.ValuationBridge = bridge

	v := report.ValueInvesting
	v.Computed = false
	v.Decision = "INSUFFICIENT_DATA"
	v.DecisionLabel = "Kanıt kapısı geçmedi"
	v.Summary = "İçsel değer, hedef fiyat ve güvenlik marjı bastırıldı; raporda eksik kanıt tamamlanmadan yatırım kararı üretilemez."
	v.IntrinsicValue = value.IntrinsicValueReport{
		Computed: false,
		Method:   "suppressed_by_strict_evidence_policy",
		Drivers:  []string{reason},
	}
	requiredPct := v.MarginOfSafety.RequiredPct
	if requiredPct <= 0 {
		requiredPct = v.Assumptions.RequiredMarginPct
	}
	v.MarginOfSafety = value.MarginOfSafetyReport{
		Computed:    false,
		RequiredPct: requiredPct,
		Label:       "kanıt_yetersiz",
	}
	v.FairValue = value.FairValueConclusion{
		Computed:    false,
		Status:      "insufficient_data",
		Label:       "Kanıt kapısı geçmedi",
		Explanation: reason,
	}
	v.Checks = []value.Check{
		{
			Name:    "strict_evidence_policy",
			Status:  "fail",
			Score:   0,
			Message: reason,
		},
	}
	v.Warnings = uniqueStrings(append(v.Warnings, "strict_evidence_policy_suppressed_value_investing"))
	report.ValueInvesting = v
}

func suppressRecommendationOutputs(report *Report, policy EvidencePolicyReport) {
	reason := strictEvidenceSummary(policy)
	required := strictRequiredEvidenceForIssues(policy.BlockingIssues)

	framework := report.InvestmentResearch.DecisionFramework
	framework.CurrentDecision = "INSUFFICIENT_DATA"
	framework.DecisionBasis = []string{reason}
	framework.BuyConditions = required
	framework.HoldConditions = []string{"Eksik kanıt tamamlanana kadar yatırım kararı değil; finansal kalite, bilanço riski ve KAP/PDF kanıt izleme dosyası olarak kullanılmalı."}
	framework.SellConditions = []string{"Sat/risk azalt kararı da yalnızca kaynaklı olumsuz KAP, finansal tablo veya fiyat verisiyle değerlendirilebilir."}
	framework.Invalidation = uniqueStrings(append(policy.BlockingIssues, "strict_evidence_policy_failed"))
	report.InvestmentResearch.DecisionFramework = framework

	story := report.InvestmentResearch.InvestmentStory
	story.CoreThesis = "Kanıt kapısı hedef fiyat ve AL/SAT kararını engelledi; araştırma özeti finansal kalite, bilanço riski, nakit akışı, sermaye hareketleri ve KAP/PDF kanıt durumu ile sınırlıdır."
	if strings.TrimSpace(story.ValueSource) == "" || strings.Contains(story.ValueSource, "not_established") {
		story.ValueSource = "Faaliyet kârlılığı, nakit akışı, borçluluk, sermaye sulanması ve emsal değerleme; hedef fiyat olarak kullanılmaz."
	}
	story.MispricingQuestion = "Piyasa zayıf kârlılık/nakit akışı, borçluluk, sermaye artırımı ve kaynak kalitesi risklerini yeterince fiyatlıyor mu?"
	story.KeyEvidence = strictEvidenceSnapshot(*report)
	story.KeyRisks = uniqueStrings(append(story.KeyRisks, policy.BlockingIssues...))
	if len(story.Catalysts) == 0 {
		story.Catalysts = []string{
			"Finansal tablo mutabakatının ve dönemsel KAP/PDF kanıt zincirinin güçlenmesi.",
			"Faaliyet kârlılığı, nakit akışı ve finansman gideri baskısında iyileşme.",
			"Sermaye artırımı, borçlanma veya sulanma risklerinin netleşmesi.",
		}
	}
	report.InvestmentResearch.InvestmentStory = story

	memo := report.InvestmentResearch.InstitutionalMemo
	memo.Recommendation = "INSUFFICIENT_DATA"
	memo.Decision = "INSUFFICIENT_DATA"
	memo.WorkflowStatus = "blocked_by_strict_evidence_policy"
	memo.DirectBuyEligible = false
	memo.InvestmentCommitteeReady = false
	memo.BrokeragePublishableReady = false
	memo.PositionSizeSuggestion = "0%; strict evidence policy gecmedi."
	memo.ExpectedReturnPct = 0
	memo.DownsideRiskPct = 0
	memo.RiskRewardRatio = 0
	memo.LiquidityConsideration = "Likidite degerlendirmesi ancak kaynak kanit kapisi gecildikten sonra anlamlidir."
	memo.PortfolioFit = "Portfoye eklenemez; once kanit, mutabakat ve veri kalitesi eksikleri kapatilmalidir."
	if memo.ReadinessScore > 25 {
		memo.ReadinessScore = 25
	}
	memo.BlockingIssues = uniqueStrings(append(append(memo.BlockingIssues, "strict_evidence_policy_failed"), policy.BlockingIssues...))
	memo.RequiredFixes = uniqueStrings(append(memo.RequiredFixes, required...))
	memo.KeyAssumptions = uniqueStrings(append(memo.KeyAssumptions, required...))
	memo.ApprovalConditions = []string{"Strict evidence policy gecmeli; ana metrikler kaynak dokuman, sayfa/tablo/alinti ve confidence kaydi ile dogrulanmali."}
	memo.RejectionConditions = uniqueStrings(append(memo.RejectionConditions, "strict_evidence_policy_failed"))
	memo.CommitteeQuestions = uniqueStrings(append(memo.CommitteeQuestions,
		"Her ana metrik için kaynak doküman, sayfa/tablo/alinti ve confidence kaydı var mı?",
		"Değerleme varsayımları gerçek kaynaklarla ve dönem/para birimi/birim mutabakatıyla doğrulandı mı?",
	))
	memo.PositiveSignals = evidenceOnlyPositiveSignals(*report)
	report.InvestmentResearch.InstitutionalMemo = memo

	report.InvestmentResearch.OpenResearchQuestions = uniqueStrings(append(report.InvestmentResearch.OpenResearchQuestions, required...))
	report.InvestmentResearch.Summary = "Kanıt kapısı AL/SAT ve hedef fiyatı bastırdı; rapor finansal kalite, nakit akışı, borçluluk, sermaye hareketleri ve KAP/PDF kanıt kalitesi için araştırma dosyasıdır."
}

func suppressValuationDependentMemoFields(report *Report, policy EvidencePolicyReport) {
	reason := strictEvidenceSummary(policy)
	memo := report.InvestmentResearch.InstitutionalMemo
	memo.ExpectedReturnPct = 0
	memo.DownsideRiskPct = 0
	memo.RiskRewardRatio = 0
	memo.KeyAssumptions = uniqueStrings(append(memo.KeyAssumptions, "hedef_fiyat_ve_getiri_risk_sayilari_strict_policy_ile_bastirildi"))
	memo.RequiredFixes = uniqueStrings(append(memo.RequiredFixes, "Değerleme güveni 70/100 üstüne çıkmadan hedef fiyat, beklenen getiri ve risk/ödül yatırım komitesi girdisi yapılmamalı."))
	memo.ApprovalConditions = uniqueStrings(append(memo.ApprovalConditions, "Hedef fiyat kullanılacaksa önce değerleme güveni ve kaynak kanıt kapısı geçmelidir."))
	if memo.PositionSizeSuggestion == "" || strings.Contains(strings.ToUpper(memo.Recommendation), "BUY") || strings.Contains(strings.ToUpper(memo.Recommendation), "ACCUMULATE") {
		memo.PositionSizeSuggestion = "0%; hedef fiyat bastırıldığı için yeni pozisyon ancak manuel değerleme doğrulaması sonrası değerlendirilmeli."
	}
	report.InvestmentResearch.InstitutionalMemo = memo

	framework := report.InvestmentResearch.DecisionFramework
	framework.DecisionBasis = uniqueStrings(append(framework.DecisionBasis, reason))
	framework.Invalidation = uniqueStrings(append(framework.Invalidation, policy.BlockingIssues...))
	report.InvestmentResearch.DecisionFramework = framework
}

func strictRequiredEvidence() []string {
	return []string{
		"her ana metrik için kaynak doküman id, dosya adı, sayfa/tablo veya alıntı",
		"finansal tablolar için dönem, para birimi, birim, konsolide/solo ve denetim bilgisi",
		"KAP PDF ingest çıktısında analize uygun belge ve kapanmış review/rejected kuyruğu",
		"normalize finansal tabloda mutabakat hatası olmaması",
		"finansal veri için publish_date/available_at veya geriye dönük bakışı bozmayacak kaynak zamanı",
		"sektör modeli ve değerleme varsayımları için açık kaynak ve sınırlama tablosu",
		"peer seçimi için aynı sektör/iş modeli gerekçesi ve yeterli örneklem",
	}
}

func marketOnlyRequiredEvidence(report Report) []string {
	required := []string{
		"TradingView OHLCV fiyat/hacim verisi ve teknik gösterge pencereleri",
		"destek/direnç, trend çizgisi ve kırılım/iptal seviyelerinin güncel fiyatla tutarlı sınıflandırması",
		"walk-forward backtest ve örneklem büyüklüğü",
		"likidite, işlem kapasitesi ve volatilite ölçümü",
	}
	switch report.Company.SectorSource {
	case "asset_type_commodity":
		required = append(required,
			"DXY / USD gücü ve ABD reel faiz teyidi",
			"COMEX/COT vadeli pozisyon ve open interest teyidi",
			"altın ETF/fiziki akış, merkez bankası ve jeopolitik haber akışı",
		)
	case "asset_type_crypto":
		required = append(required,
			"on-chain, derivatives/funding/open interest, exchange-flow ve haber/sentiment teyidi",
			"spot borsa likiditesi ve volatilite rejimi",
		)
	default:
		required = append(required, "tamamlayıcı piyasa ve haber/sentiment kaynakları")
	}
	for _, missing := range report.Coverage.Missing {
		required = append(required, "geliştirilecek veri kaynağı: "+missing)
	}
	return uniqueStrings(required)
}

func strictRequiredEvidenceForIssues(blockingIssues []string) []string {
	if len(blockingIssues) == 0 {
		return strictRequiredEvidence()
	}
	required := []string{}
	add := func(items ...string) {
		required = append(required, items...)
	}
	for _, issue := range blockingIssues {
		switch {
		case issue == "financial_statements_or_intrinsic_value_missing":
			add(
				"finansal tablolar ve içsel değer girdileri eksiksiz yüklenmeli",
				"ana değerleme girdileri kaynak dosya, dönem, para birimi ve birim bilgisiyle bağlanmalı",
			)
		case strings.HasPrefix(issue, "coverage_score_below_"):
			add("kapsam skoru strict eşik üstüne çıkacak şekilde eksik veri kaynakları tamamlanmalı")
		case strings.HasPrefix(issue, "value_data_quality_below_"):
			add("değerleme veri kalitesi 70/100 üstüne çıkmalı; düşük kaliteli girdiler manuel doğrulanmalı")
		case strings.HasPrefix(issue, "valuation_confidence_below_"):
			add(
				"değerleme güveni 70/100 üstüne çıkmalı",
				"değerleme varsayımları, hassasiyet tablosu ve model sınırlamaları kaynaklı olarak doğrulanmalı",
			)
		case issue == "financial_statements_not_reconciled":
			add("normalize finansal tabloda bilanço mutabakatı geçmeli")
		case issue == "financial_reconciliation_failures_present":
			add("finansal mutabakat hataları kapatılmalı veya insan incelemesiyle açıklanmalı")
		case issue == "financial_available_at_or_publish_date_not_safe":
			add("finansal veri için publish_date/available_at veya geriye dönük bakışı bozmayacak kaynak zamanı doğrulanmalı")
		case issue == "financial_source_lineage_missing":
			add("finansal veri için kaynak lineage kaydı, import aşaması ve dönüşüm versiyonu yazılmalı")
		case issue == "kap_pdf_ingest_missing":
			add("KAP PDF ingest çalışmalı ve kaynak doküman indeksi üretilmeli")
		case issue == "kap_pdf_no_analysis_usable_documents":
			add("KAP PDF kalite kapısında en az bir analize uygun belge bulunmalı")
		case issue == "kap_pdf_analysis_usable_ratio_below_60":
			add("KAP PDF review/rejected kuyruğu kapatılarak analize uygun belge oranı en az %60 olmalı")
		case issue == "sector_classification_missing":
			add("sektör, iş modeli ve peer group sınıflandırması kaynaklı olarak üretilmeli")
		case issue == "sector_classification_confidence_below_strict_threshold":
			add("sektör ve iş modeli sınıflandırması güveni strict eşik üstüne çıkarılmalı")
		case issue == "peer_sample_below_3":
			add("peer seçimi için aynı sektör/iş modeli gerekçesiyle en az üç karşılaştırılabilir şirket sağlanmalı")
		case issue == "reit_nav_bridge_missing":
			add("GYO için güncel ekspertiz/portföy toplamı, net borç, nakit, ertelenmiş vergi, azınlık payı ve hisse sayısıyla NAD köprüsü kurulmalı")
		case issue == "bank_regulatory_metrics_missing":
			add(
				"bankalar için SYR, CET1/çekirdek sermaye, NPL/takipteki kredi, karşılık kapsamı, NIM/net faiz marjı, LCR, kredi/mevduat, mevduat maliyeti ve kredi/mevduat spread'i structured veri olarak bağlanmalı",
				"banka özkaynak ve HBDD doğrudan konsolide finansal tablodan mutabakatlı alınmalı",
				"bu metrikler tamamlanmadan içsel değer, hedef fiyat ve AL/SAT kararı üretilmemeli",
			)
		default:
			add("strict evidence policy bloklayıcısı kapatılmalı: " + issue)
		}
	}
	return uniqueStrings(required)
}

func strictEvidenceSummary(policy EvidencePolicyReport) string {
	if len(policy.BlockingIssues) == 0 {
		return "Strict kanıt politikası geçti."
	}
	return "Strict kanıt politikası model çıktısını engelledi: " + strings.Join(policy.BlockingIssues, ", ")
}

func strictEvidenceSnapshot(report Report) []string {
	out := []string{}
	if report.DataGovernance.LatestPeriod != "" {
		out = append(out, "Finansal son dönem: "+report.DataGovernance.LatestPeriod)
	}
	if report.DataGovernance.Source != "" {
		out = append(out, "Finansal kaynak: "+report.DataGovernance.Source)
	}
	if report.KAPPDFIngest.TotalDocuments > 0 {
		out = append(out, fmt.Sprintf("KAP PDF kapsamı: %d belge, %d analize uygun, %d analiz dışı (%d review, %d rejected).", report.KAPPDFIngest.TotalDocuments, report.KAPPDFIngest.AnalysisUsableCount, report.KAPPDFIngest.ReviewRequiredCount, kapPDFReviewOnlyCount(report.KAPPDFIngest), report.KAPPDFIngest.RejectedCount))
	}
	if report.KAPAssetInventory.Computed {
		out = append(out, fmt.Sprintf("Varlık envanteri: %d event, %d rapor satırı.", report.KAPAssetInventory.EventCount, report.KAPAssetInventory.DisplayAssetCount))
	}
	if len(out) == 0 {
		out = append(out, "Kanıt kapısı için yeterli kaynak özeti yok.")
	}
	return out
}

func evidenceOnlyPositiveSignals(report Report) []string {
	out := []string{}
	if report.KAPPDFIngest.TotalDocuments > 0 {
		out = append(out, fmt.Sprintf("%d KAP PDF belgesi indekslendi; %d belge analize uygun işaretlendi.", report.KAPPDFIngest.TotalDocuments, report.KAPPDFIngest.AnalysisUsableCount))
	}
	if report.KAPAssetInventory.Computed {
		out = append(out, fmt.Sprintf("%d varlık event'i işlendi.", report.KAPAssetInventory.EventCount))
	}
	if report.DataGovernance.LatestPeriod != "" {
		out = append(out, "Finansal son dönem kaydı var: "+report.DataGovernance.LatestPeriod)
	}
	return uniqueStrings(out)
}
