package analysis

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"hissebot/internal/kapingest"
	"hissebot/internal/ta/investorqa"
	"hissebot/pkg/mathutil"
)

const valuationModelDivergenceLimitPct = 50.0

// DecisionClassification is the single source of truth for every report
// class. Renderers explain these results but may not promote a failed class.
type DecisionClassification struct {
	SchemaVersion         int                            `json:"schema_version"`
	Status                string                         `json:"status"`
	Summary               string                         `json:"summary"`
	Gates                 []ClassificationGate           `json:"gates"`
	Classes               DecisionClasses                `json:"classes"`
	ValuationConsistency  ValuationConsistencyAssessment `json:"valuation_consistency"`
	SectorModelAlignment  SectorModelAssessment          `json:"sector_model_alignment"`
	EffectiveModelRisk    float64                        `json:"effective_model_risk_score"`
	ConsistencyViolations []string                       `json:"consistency_violations,omitempty"`
}

type ClassificationGate struct {
	Key         string   `json:"key"`
	Label       string   `json:"label,omitempty"`
	Explanation string   `json:"explanation,omitempty"`
	Status      string   `json:"status"`
	Passed      bool     `json:"passed"`
	Score       float64  `json:"score,omitempty"`
	Reason      string   `json:"reason"`
	Evidence    []string `json:"evidence,omitempty"`
}

type DecisionClasses struct {
	LargeInvestor          DecisionClassResult `json:"large_investor"`
	RetailDirect           DecisionClassResult `json:"retail_direct_al_sat"`
	ValueInvesting         DecisionClassResult `json:"value_investing"`
	TradingEdge            DecisionClassResult `json:"trading_edge"`
	InstitutionalPortfolio DecisionClassResult `json:"institutional_portfolio"`
	AutomaticOrder         DecisionClassResult `json:"automatic_order"`
	ResearchReport         DecisionClassResult `json:"research_report"`
}

type DecisionClassResult struct {
	Key                    string   `json:"key"`
	Status                 string   `json:"status"`
	Qualified              bool     `json:"qualified"`
	Decision               string   `json:"decision"`
	Score                  float64  `json:"score"`
	Summary                string   `json:"summary"`
	RequiredGates          []string `json:"required_gates"`
	RequiredGateLabels     []string `json:"required_gate_labels,omitempty"`
	FailedGates            []string `json:"failed_gates,omitempty"`
	FailedGateLabels       []string `json:"failed_gate_labels,omitempty"`
	FailedGateExplanations []string `json:"failed_gate_explanations,omitempty"`
}

type ValuationModelValue struct {
	Model string  `json:"model"`
	Value float64 `json:"value"`
}

type ValuationConsistencyAssessment struct {
	Computed          bool                  `json:"computed"`
	Publishable       bool                  `json:"publishable"`
	Status            string                `json:"status"`
	ThresholdPct      float64               `json:"threshold_pct"`
	MaxDivergencePct  float64               `json:"max_divergence_pct"`
	MinimumModelValue float64               `json:"minimum_model_value,omitempty"`
	MaximumModelValue float64               `json:"maximum_model_value,omitempty"`
	Models            []ValuationModelValue `json:"models,omitempty"`
	Reason            string                `json:"reason"`
}

type SectorModelAssessment struct {
	Status                 string   `json:"status"`
	Passed                 bool     `json:"passed"`
	Sector                 string   `json:"sector"`
	Industry               string   `json:"industry"`
	FinancialProfile       string   `json:"financial_profile"`
	ExpectedValuationModel []string `json:"expected_valuation_models,omitempty"`
	ActualValuationModel   string   `json:"actual_valuation_model"`
	RequiredMetrics        []string `json:"required_metrics,omitempty"`
	AvailableMetrics       []string `json:"available_metrics,omitempty"`
	MetricCoveragePct      float64  `json:"metric_coverage_pct"`
	Reason                 string   `json:"reason"`
}

func ClassifyDecision(result SymbolAnalysis) DecisionClassification {
	valuation := evaluateValuationConsistency(result)
	sectorModel := evaluateSectorModelAlignment(result)
	effectiveModelRisk := result.InvestorQA.ModelRisk.Score
	if valuation.Computed && !valuation.Publishable {
		effectiveModelRisk = math.Min(effectiveModelRisk, 49)
	}
	if !sectorModel.Passed {
		effectiveModelRisk = math.Min(effectiveModelRisk, 59)
	}

	daily, hasDaily := result.Timeframes["1D"]
	technicalPass := hasDaily && statusPass(daily.Professional.Technical.SignalGate.Status) && daily.Professional.Technical.SignalGate.Actionable
	activePlan := hasDaily && activeDecisionTradePlan(daily)
	bt := daily.Professional.Backtest
	backtestPass := hasDaily && bt.BacktestSafe && bt.LookaheadViolations == 0 && bt.Trades >= 30 && bt.OutOfSampleTrades >= 10 && bt.Expectancy > 0 && bt.OutOfSampleReturn > 0
	views := result.InvestorQA.InstitutionalViews
	rawKAPIssues := rawKAPFinancialIntegrityIssues(result)
	financialIntegrity := financialStatementDecisionSafe(result) &&
		result.Professional.DataGovernance.ReconciliationFailureCount == 0 &&
		result.Professional.ValueInvesting.Computed &&
		len(rawKAPIssues) == 0
	evidencePass := statusPass(result.Professional.EvidencePolicy.Status)
	portfolioPass := statusPass(views.Portfolio.Status) && statusPass(views.Portfolio.ReportQualityStatus)
	tradingPass := statusPass(views.TradingEdge.Status) && statusPass(views.TradingEdge.TransactionUseStatus) && backtestPass
	valuePass := statusPass(views.ValueInvesting.Status) && result.Professional.ValueInvesting.Computed && valuation.Publishable
	macroPass := macroDecisionOK(result)
	modelRiskPass := effectiveModelRisk >= 65
	nextSessionForecastPass := nextSessionForecastDecisionOK(result)
	microDecisionPass, microOrderPass := false, false
	if micro := result.Professional.Market.Microstructure; micro != nil {
		microDecisionPass = micro.Liquidity.DecisionUsable
		microOrderPass = micro.Liquidity.AutomaticOrderReady
	}

	gates := []ClassificationGate{
		classificationGate("evidence_policy", evidencePass, result.Professional.Coverage.Score, "Karar kanıt politikası", result.Professional.EvidencePolicy.BlockingIssues),
		classificationGate("decision_price", decisionPriceOK(result), priceQualityScore(result), "Güncel ve mutabık yatırımcı karar fiyatı", priceQualityEvidence(result)),
		classificationGate("verified_final_close", verifiedPriceCloseOK(result), priceQualityScore(result), "Resmi/final kapanış; yalnız otomatik emir için zorunlu", priceQualityEvidence(result)),
		classificationGate("financial_integrity", financialIntegrity, result.Professional.DataQuality, "Finansal tablo mutabakatı ve kullanılabilir değerleme girdisi", financialIntegrityEvidence(result)),
		{Key: "valuation_consistency", Status: valuation.Status, Passed: valuation.Publishable, Score: valuationConsistencyScore(valuation), Reason: valuation.Reason, Evidence: valuationModelEvidence(valuation)},
		{Key: "sector_model_alignment", Status: sectorModel.Status, Passed: sectorModel.Passed, Score: sectorModel.MetricCoveragePct, Reason: sectorModel.Reason, Evidence: sectorModelEvidence(sectorModel)},
		classificationGate("value_thesis", valuePass, views.ValueInvesting.Score, "Değer yatırım aksiyon kapısı", append(append([]string{}, views.ValueInvesting.Blockers...), views.ValueInvesting.RequiredActions...)),
		classificationGate("institutional_portfolio", portfolioPass, views.Portfolio.Score, "Kurumsal portföy aksiyon ve rapor kalite kapısı", append(append([]string{}, views.Portfolio.Blockers...), views.Portfolio.RequiredActions...)),
		classificationGate("technical_signal", technicalPass, dailyTechnicalScore(daily, hasDaily), "Aksiyonlanabilir günlük teknik sinyal", technicalGateEvidence(daily, hasDaily)),
		classificationGate("active_trade_plan", activePlan, tradePlanScore(daily, hasDaily), "Aktif giriş, stop ve hedef planı", tradePlanEvidence(daily, hasDaily)),
		classificationGate("trading_edge", tradingPass, views.TradingEdge.Score, "Walk-forward/OOS ve maliyet sonrası trading edge", append(append([]string{}, views.TradingEdge.Blockers...), views.TradingEdge.RequiredActions...)),
		classificationGate("next_session_forecast_model", nextSessionForecastPass, nextSessionForecastDecisionScore(result), "Sonraki seans tahmini karar kalitesinde olmalı", nextSessionForecastDecisionEvidence(result)),
		classificationGate("macro_regime", macroPass, macroDecisionScore(result), "Makro rejim yeni pozisyonu bloke etmemeli", macroClassificationEvidence(result)),
		classificationGate("model_risk", modelRiskPass, effectiveModelRisk, "Model güven skoru ve kritik model tutarlılığı", result.InvestorQA.ModelRisk.PrimaryLimitations),
		classificationGate("market_microstructure", microDecisionPass, microstructureScore(result), "AKD/takas/derinlik verilerinin karar ve pozisyon boyutu kullanımı", microstructureClassificationEvidence(result)),
		classificationGate("automatic_execution", microOrderPass, microstructureScore(result), "Anlık spread, defter ve execution kapısı", automaticExecutionEvidence(result)),
	}
	for i := range gates {
		enrichClassificationGate(&gates[i])
	}
	gateMap := make(map[string]ClassificationGate, len(gates)+1)
	for _, gate := range gates {
		gateMap[gate.Key] = gate
	}

	classes := DecisionClasses{}
	classes.ValueInvesting = classifyReportClass("value_investing", []string{"evidence_policy", "financial_integrity", "sector_model_alignment", "valuation_consistency", "value_thesis", "model_risk"}, gateMap, views.ValueInvesting.Score, "YAYIMLA", "YAYIMLAMA")
	classes.InstitutionalPortfolio = classifyReportClass("institutional_portfolio", []string{"evidence_policy", "financial_integrity", "sector_model_alignment", "valuation_consistency", "institutional_portfolio", "macro_regime", "decision_price", "model_risk"}, gateMap, views.Portfolio.Score, "PORTFOYE_AL", "PORTFOYE_ALMA")
	classes.LargeInvestor = classifyReportClass("large_investor", classes.InstitutionalPortfolio.RequiredGates, gateMap, views.Portfolio.Score, "ONAYLA", "REDDET")
	classes.TradingEdge = classifyReportClass("trading_edge", []string{"decision_price", "technical_signal", "active_trade_plan", "trading_edge", "next_session_forecast_model", "model_risk"}, gateMap, views.TradingEdge.Score, "ISLEM_SINYALI_HAZIR", "ISLEM_ACMA")
	classes.RetailDirect = classifyReportClass("retail_direct_al_sat", []string{"evidence_policy", "decision_price", "financial_integrity", "sector_model_alignment", "valuation_consistency", "technical_signal", "active_trade_plan", "trading_edge", "next_session_forecast_model", "macro_regime", "model_risk"}, gateMap, result.InvestorQA.Score, "AL_SAT_SINYALI_HAZIR", "BEKLE")
	autoRequired := append([]string{}, classes.RetailDirect.RequiredGates...)
	autoRequired = append(autoRequired, "verified_final_close", "automatic_execution")
	classes.AutomaticOrder = classifyReportClass("automatic_order", autoRequired, gateMap, math.Min(classes.RetailDirect.Score, microstructureScore(result)), "EMIR_ACIK", "EMIR_KAPALI")
	researchGate := classificationGate("research_material", result.Professional.DataQuality > 0 || result.Professional.Coverage.Score > 0 || len(result.Timeframes) > 0, result.Professional.Coverage.Score, "Kaynaklı araştırma malzemesi üretilebilir", nil)
	enrichClassificationGate(&researchGate)
	gates = append(gates, researchGate)
	gateMap[researchGate.Key] = researchGate
	classes.ResearchReport = classifyReportClass("research_report", []string{"research_material"}, gateMap, result.Professional.Coverage.Score, "YAYIMLA", "YAYIMLAMA")

	out := DecisionClassification{
		SchemaVersion:        1,
		Gates:                gates,
		Classes:              classes,
		ValuationConsistency: valuation,
		SectorModelAlignment: sectorModel,
		EffectiveModelRisk:   effectiveModelRisk,
	}
	if classes.LargeInvestor.Qualified || classes.RetailDirect.Qualified {
		out.Status = "decision_ready"
		out.Summary = "Merkezi kapılar yatırımcı karar sınıflarından en az birini onayladı."
	} else {
		out.Status = "decision_issued_not_qualified"
		out.Summary = "Merkezi motor karar üretti; büyük yatırımcı pozisyonu ve küçük yatırımcı doğrudan AL/SAT kullanımı onaylanmadı."
	}
	out.ConsistencyViolations = classificationConsistencyViolations(out)
	return out
}

func ApplyDecisionClassification(result SymbolAnalysis) SymbolAnalysis {
	c := result.DecisionClassification
	if c.SchemaVersion == 0 {
		c = ClassifyDecision(result)
		result.DecisionClassification = c
	}
	if c.EffectiveModelRisk < result.InvestorQA.ModelRisk.Score || result.InvestorQA.ModelRisk.Score == 0 {
		result.InvestorQA.ModelRisk.Score = c.EffectiveModelRisk
		result.InvestorQA.ModelRisk.Status = classificationStatusFromScore(c.EffectiveModelRisk)
	}
	if c.ValuationConsistency.Computed && !c.ValuationConsistency.Publishable {
		limitation := fmt.Sprintf("valuation_model_divergence_%.1fpct_above_%.1fpct", c.ValuationConsistency.MaxDivergencePct, c.ValuationConsistency.ThresholdPct)
		result.InvestorQA.ModelRisk.PrimaryLimitations = appendUniqueAnalysisString(result.InvestorQA.ModelRisk.PrimaryLimitations, limitation)
		result.Professional.ValueInvesting.Warnings = appendUniqueAnalysisString(result.Professional.ValueInvesting.Warnings, limitation)
		result.Professional.ValueInvesting.Confidence = math.Min(result.Professional.ValueInvesting.Confidence, 49)
		result.Professional.ValueInvesting.FairValue.Status = "not_publishable_model_divergence"
		result.Professional.ValueInvesting.FairValue.Label = "Değerleme modelleri mutabık değil; yayımlanamaz"
		result.Professional.EvidencePolicy.ValuationTargetsAllowed = false
		result.Professional.EvidencePolicy.ScenarioTargetsAllowed = false
		result.Professional.EvidencePolicy.RecommendationAllowed = false
	}
	alignPersonaView(&result.InvestorQA.InstitutionalViews.ValueInvesting, c.Classes.ValueInvesting, "KULLANIR", "KULLANMAZ")
	alignPersonaView(&result.InvestorQA.InstitutionalViews.Portfolio, c.Classes.InstitutionalPortfolio, "PORTFOY_ON_ELEME", "PORTFOY_KULLANMAZ")
	alignPersonaView(&result.InvestorQA.InstitutionalViews.TradingEdge, c.Classes.TradingEdge, "TRADING_ADAYI", "TRADING_KULLANMAZ")
	views := &result.InvestorQA.InstitutionalViews
	views.OverallStatus = combineCentralClassStatus(c.Classes.ValueInvesting, c.Classes.InstitutionalPortfolio, c.Classes.TradingEdge)
	views.EliteCandidate.Status = views.OverallStatus
	views.EliteCandidate.Computed = true
	views.EliteCandidate.FailedPasses = uniqueSortedStringsAnalysis(append(append(append([]string{}, c.Classes.ValueInvesting.FailedGateLabels...), c.Classes.InstitutionalPortfolio.FailedGateLabels...), c.Classes.TradingEdge.FailedGateLabels...))
	views.FinancialTransactionUse.Status = c.Classes.RetailDirect.Status
	views.FinancialTransactionUse.Answer = centralClassAnswer(c.Classes.RetailDirect)
	views.FinancialTransactionUse.Summary = c.Classes.RetailDirect.Summary
	if !c.Classes.RetailDirect.Qualified {
		result.InvestorQA.Decision = "BEKLE"
		result.InvestorQA.DecisionLabel = "Merkezi karar kapıları doğrudan AL/SAT kullanımını onaylamadı; BEKLE"
	}
	memo := &result.Professional.InvestmentResearch.InstitutionalMemo
	memo.InvestmentCommitteeReady = c.Classes.LargeInvestor.Qualified
	memo.DirectBuyEligible = c.Classes.LargeInvestor.Qualified && c.Classes.ValueInvesting.Qualified
	memo.BrokeragePublishableReady = c.Classes.ValueInvesting.Qualified
	memo.WorkflowStatus = c.Classes.LargeInvestor.Status
	memo.Decision = c.Classes.LargeInvestor.Decision
	memo.Recommendation = c.Classes.LargeInvestor.Decision
	if !c.Classes.LargeInvestor.Qualified {
		memo.BlockingIssues = uniqueSortedStringsAnalysis(append(memo.BlockingIssues, c.Classes.LargeInvestor.FailedGateLabels...))
	}
	if !c.Classes.ValueInvesting.Qualified {
		result.Professional.InvestmentResearch.DecisionFramework.CurrentDecision = "YAYIMLANAMAZ / " + c.Classes.ValueInvesting.Decision
	}
	return result
}

func alignPersonaView(view *investorqa.PersonaView, class DecisionClassResult, passDecision, failDecision string) {
	view.Status = class.Status
	view.TransactionUseStatus = class.Status
	view.Score = math.Min(view.Score, class.Score)
	if class.Qualified {
		view.Decision = passDecision
		view.TransactionUseAnswer = centralClassAnswer(class)
		return
	}
	view.Decision = failDecision
	view.DecisionLabel = class.Decision
	view.TransactionUseAnswer = centralClassAnswer(class)
	view.Blockers = uniqueSortedStringsAnalysis(append(view.Blockers, class.FailedGateLabels...))
}

func classificationGate(key string, passed bool, score float64, reason string, evidence []string) ClassificationGate {
	status := "fail"
	if passed {
		status = "pass"
	}
	return ClassificationGate{
		Key: key, Status: status, Passed: passed,
		Score: mathutil.Clamp(score, 0, 100), Reason: reason,
		Evidence: compactStrings(evidence, 8),
	}
}

func classifyReportClass(key string, required []string, gates map[string]ClassificationGate, score float64, passDecision, failDecision string) DecisionClassResult {
	out := DecisionClassResult{
		Key: key, RequiredGates: append([]string{}, required...),
		Score: mathutil.Clamp(score, 0, 100), Decision: passDecision,
	}
	out.RequiredGateLabels = ClassificationGateLabels(out.RequiredGates)
	for _, gateKey := range required {
		gate, ok := gates[gateKey]
		if !ok || !gate.Passed {
			out.FailedGates = append(out.FailedGates, gateKey)
		}
	}
	out.FailedGates = uniqueSortedStringsAnalysis(out.FailedGates)
	out.FailedGateLabels = ClassificationGateLabels(out.FailedGates)
	out.FailedGateExplanations = ClassificationGateExplanations(out.FailedGates)
	out.Qualified = len(out.FailedGates) == 0
	if out.Qualified {
		out.Status = "pass"
		out.Summary = "Gerekli merkezi kapıların tamamı geçti."
	} else {
		out.Status = "fail"
		out.Decision = failDecision
		out.Summary = "Geçmeyen kontroller: " + strings.Join(out.FailedGateExplanations, "; ") + "."
	}
	return out
}

func enrichClassificationGate(gate *ClassificationGate) {
	if gate == nil {
		return
	}
	gate.Label = ClassificationGateLabel(gate.Key)
	gate.Explanation = ClassificationGateExplanation(gate.Key)
}

func ClassificationGateLabels(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, ClassificationGateLabel(key))
	}
	return out
}

func ClassificationGateExplanations(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, ClassificationGateLabel(key)+": "+ClassificationGateExplanation(key))
	}
	return out
}

func ClassificationGateLabel(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "evidence_policy":
		return "Kanıt politikası"
	case "decision_price":
		return "Güncel karar fiyatı"
	case "verified_final_close":
		return "Resmi kapanış doğrulaması"
	case "financial_integrity":
		return "Finansal veri mutabakatı"
	case "valuation_consistency":
		return "Değerleme tutarlılığı"
	case "sector_model_alignment":
		return "Sektör ve değerleme modeli uyumu"
	case "value_thesis":
		return "Değer yatırım tezi"
	case "institutional_portfolio":
		return "Kurumsal portföy uygunluğu"
	case "technical_signal":
		return "Günlük teknik sinyal"
	case "active_trade_plan":
		return "Aktif işlem planı"
	case "trading_edge":
		return "İstatistiksel işlem avantajı"
	case "next_session_forecast_model":
		return "Sonraki seans tahmin modeli"
	case "macro_regime":
		return "Makro ortam"
	case "model_risk":
		return "Model güveni"
	case "market_microstructure":
		return "Piyasa mikroyapısı"
	case "automatic_execution":
		return "Otomatik emir altyapısı"
	case "research_material":
		return "Araştırma malzemesi"
	default:
		if strings.TrimSpace(key) == "" {
			return "Bilinmeyen kontrol"
		}
		return strings.ReplaceAll(strings.TrimSpace(key), "_", " ")
	}
}

func ClassificationGateExplanation(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "evidence_policy":
		return "Hedef fiyat, değerleme ve karar cümlesi için yeterli kaynak ve kanıt izlenebilirliği aranır."
	case "decision_price":
		return "Raporda kullanılan son fiyat taze ve kaynaklarla uyumlu olmalıdır."
	case "verified_final_close":
		return "Son kapanış fiyatı resmi veya lisanslı kaynakla nihai kapanış olarak doğrulanmalıdır; özellikle otomatik emir için zorunludur."
	case "financial_integrity":
		return "Bilanço, gelir tablosu, nakit akışı ve hesaplanan finansal oranlar birbiriyle çelişmemelidir."
	case "valuation_consistency":
		return "Farklı değerleme modelleri birbirinden fazla ayrışmamalı; ayrışma yüksekse hedef/değerleme yayımlanmaz."
	case "sector_model_alignment":
		return "Şirketin sektörü için doğru değerleme modeli ve zorunlu sektör metrikleri kullanılmalıdır."
	case "value_thesis":
		return "İçsel değer, kalite, finansal güç ve güvenlik marjı birlikte yatırım tezi oluşturmalıdır."
	case "institutional_portfolio":
		return "Kurumsal portföye uygunluk için veri kalitesi, risk, likidite ve portföy gerekçesi yeterli olmalıdır."
	case "technical_signal":
		return "Günlük trend, hacim, momentum, formasyon ve backtest birlikte aktif AL/SAT sinyali üretmelidir."
	case "active_trade_plan":
		return "Giriş aralığı, stop-loss, hedef fiyat ve risk/getiri oranı aynı anda hazır olmalıdır."
	case "trading_edge":
		return "Geçmiş testlerde yeterli örnek, OOS sonuç ve maliyet sonrası pozitif beklenti görülmelidir."
	case "next_session_forecast_model":
		return "Sonraki seans fiyat/yön modeli geriye dönük doğrulamayı ve teknik karar kapısını geçmeden AL/SAT veya emir girdisi olamaz."
	case "macro_regime":
		return "Faiz, enflasyon, kur, rezerv ve kredi koşulları yeni pozisyonu sert biçimde terslememelidir."
	case "model_risk":
		return "Model skoru, veri kalitesi ve tutarlılık seviyesi karar için kabul edilebilir güven aralığında olmalıdır."
	case "market_microstructure":
		return "AKD, takas, derinlik, emir defteri ve likidite verileri pozisyon boyutu için yeterli olmalıdır."
	case "automatic_execution":
		return "Anlık spread, emir defteri ve likidite koşulları otomatik emir göndermeye uygun olmalıdır."
	case "research_material":
		return "Rapor kaynaklı araştırma dokümanı olarak kullanılabilecek en az temel veri ve analiz üretmelidir."
	default:
		return "Bu kontrol için açıklama tanımı henüz eklenmedi."
	}
}

func evaluateValuationConsistency(result SymbolAnalysis) ValuationConsistencyAssessment {
	out := ValuationConsistencyAssessment{
		ThresholdPct: valuationModelDivergenceLimitPct,
		Status:       "limited",
		Publishable:  true,
	}
	add := func(model string, value float64) {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return
		}
		for _, existing := range out.Models {
			if math.Abs(existing.Value-value)/math.Max(existing.Value, value) < 0.01 {
				return
			}
		}
		out.Models = append(out.Models, ValuationModelValue{Model: model, Value: value})
	}
	add("sector_intrinsic_value", result.Professional.ValueInvesting.IntrinsicValue.Base)
	add("peer_fair_value", result.Professional.Valuation.FairValue.Base)
	if result.Professional.Valuation.DCF.Computed {
		add("dcf", result.Professional.Valuation.DCF.FairValuePerShare)
	}
	nav := result.Professional.InvestmentResearch.ValuationBridge.NAVBridge
	if nav.Status == "full_nav_reconciled" || nav.Status == "book_value_nav_proxy" {
		add("nav_or_book_value_proxy", nav.EstimatedNAVPerShare)
	}
	sort.Slice(out.Models, func(i, j int) bool { return out.Models[i].Value < out.Models[j].Value })
	if len(out.Models) == 0 {
		out.Publishable = false
		out.Status = "fail"
		out.Reason = "Yayımlanabilir değerleme modeli değeri yok."
		return out
	}
	out.Computed = true
	out.MinimumModelValue = out.Models[0].Value
	out.MaximumModelValue = out.Models[len(out.Models)-1].Value
	if len(out.Models) == 1 {
		out.Status = "limited"
		out.Reason = "Tek değerleme modeli var; model ayrışması çapraz kontrol edilemedi."
		return out
	}
	out.MaxDivergencePct = mathutil.SafeDiv(out.MaximumModelValue-out.MinimumModelValue, out.MinimumModelValue) * 100
	if out.MaxDivergencePct > out.ThresholdPct {
		out.Publishable = false
		out.Status = "fail"
		out.Reason = fmt.Sprintf("Model değerleri arasındaki azami ayrışma %.1f%% ((en yüksek-en düşük)/en düşük); %.1f%% yayın eşiği aşıldı. Bu oran fiyat/baz değer farkı değildir.", out.MaxDivergencePct, out.ThresholdPct)
		return out
	}
	out.Status = "pass"
	out.Reason = fmt.Sprintf("Model değerleri arasındaki azami ayrışma %.1f%% ((en yüksek-en düşük)/en düşük); yayın eşiği içinde.", out.MaxDivergencePct)
	return out
}

func evaluateSectorModelAlignment(result SymbolAnalysis) SectorModelAssessment {
	profile := strings.TrimSpace(result.Professional.SectorFinancials.Profile)
	actual := strings.TrimSpace(result.Professional.ValueInvesting.SectorModel.Model)
	out := SectorModelAssessment{
		Status: "fail", Sector: result.Professional.Company.Sector,
		Industry:         result.Professional.Company.Industry,
		FinancialProfile: profile, ActualValuationModel: actual,
	}
	out.ExpectedValuationModel, out.RequiredMetrics = expectedSectorModelsAndMetrics(profile)
	for _, metric := range result.Professional.SectorFinancials.Metrics {
		out.AvailableMetrics = append(out.AvailableMetrics, metric.Name)
	}
	out.AvailableMetrics = uniqueSortedStringsAnalysis(out.AvailableMetrics)
	matched := 0
	for _, metric := range out.RequiredMetrics {
		if containsExactString(out.AvailableMetrics, metric) {
			matched++
		}
	}
	if len(out.RequiredMetrics) == 0 {
		out.MetricCoveragePct = 100
	} else {
		out.MetricCoveragePct = 100 * float64(matched) / float64(len(out.RequiredMetrics))
	}
	modelOK := len(out.ExpectedValuationModel) == 0 || containsExactString(out.ExpectedValuationModel, actual)
	classificationOK := result.Professional.Company.ClassificationConfidence >= 0.80
	profileOK := profile != "" && profile != "generic_operating_company" && classificationOK
	metricsOK := len(out.RequiredMetrics) == 0 || out.MetricCoveragePct >= 60
	out.Passed = profileOK && modelOK && metricsOK
	if out.Passed {
		out.Status = "pass"
		out.Reason = fmt.Sprintf("%s sektör profili, %s değerleme modeli ve %.0f%% zorunlu metrik kapsamı uyumlu.", profile, actual, out.MetricCoveragePct)
		return out
	}
	parts := []string{}
	if !profileOK {
		parts = append(parts, "güvenilir sektör finansal profili yok")
	}
	if !modelOK {
		parts = append(parts, fmt.Sprintf("değerleme modeli %s; beklenen %s", emptyDecisionText(actual, "yok"), strings.Join(out.ExpectedValuationModel, "/")))
	}
	if !metricsOK {
		parts = append(parts, fmt.Sprintf("zorunlu sektör metriği kapsamı %.0f%%", out.MetricCoveragePct))
	}
	out.Reason = strings.Join(parts, "; ")
	return out
}

func expectedSectorModelsAndMetrics(profile string) ([]string, []string) {
	switch profile {
	case "technology", "information_services":
		return []string{"technology_growth_quality"}, []string{"gross_margin", "ebit_margin", "fcf_conversion", "cash_ratio", "net_debt_to_equity"}
	case "defense_industrial":
		return []string{"owner_earnings_dcf"}, []string{"current_ratio", "receivable_turnover", "inventory_turnover", "gross_margin", "ebit_margin", "fcf_conversion"}
	case "bank":
		return []string{"bank_residual_income"}, []string{"capital_adequacy_ratio", "npl_ratio", "nim", "lcr"}
	case "insurance":
		return []string{"insurance_book_value"}, nil
	case "reit_nav":
		return []string{"gyo_nav_proxy"}, []string{"pb", "book_per_share", "net_debt_to_equity"}
	case "holding_sotp":
		return []string{"holding_sum_of_parts_proxy"}, []string{"pb", "book_per_share", "net_debt_to_equity"}
	case "leasing_factoring_finance", "brokerage_asset_management", "investment_trust":
		return []string{"financial_book_value", "holding_sum_of_parts_proxy"}, nil
	case "":
		return nil, nil
	default:
		return []string{"owner_earnings_dcf"}, nil
	}
}

func activeDecisionTradePlan(daily TimeframeAnalysis) bool {
	plan := daily.TradePlan
	if plan.Rejected || strings.TrimSpace(plan.Direction) == "" || strings.EqualFold(plan.Direction, "neutral") {
		return false
	}
	return plan.EntryMin > 0 && plan.EntryMax > 0 && plan.StopLoss > 0 && plan.TakeProfit1 > 0
}

func classificationStatusFromScore(score float64) string {
	if score >= 70 {
		return "pass"
	}
	if score >= 50 {
		return "limited"
	}
	return "fail"
}

func valuationConsistencyScore(value ValuationConsistencyAssessment) float64 {
	if !value.Computed {
		return 0
	}
	return mathutil.Clamp(100-value.MaxDivergencePct, 0, 100)
}

func priceQualityScore(result SymbolAnalysis) float64 {
	if result.PriceQuality == nil {
		return 0
	}
	if result.PriceQuality.ReadyForVerifiedClose {
		return 100
	}
	if result.PriceQuality.ReadyForDecision {
		return 80
	}
	return 0
}

func priceQualityEvidence(result SymbolAnalysis) []string {
	if result.PriceQuality == nil {
		return []string{"price_quality_missing"}
	}
	return append([]string{}, result.PriceQuality.BlockingReasons...)
}

func financialIntegrityEvidence(result SymbolAnalysis) []string {
	gov := result.Professional.DataGovernance
	out := []string{
		fmt.Sprintf("financially_consistent=%t", gov.FinanciallyConsistent),
		fmt.Sprintf("reconciliation_failures=%d", gov.ReconciliationFailureCount),
		fmt.Sprintf("latest_period=%s", gov.LatestPeriod),
		fmt.Sprintf("value_computed=%t", result.Professional.ValueInvesting.Computed),
	}
	out = append(out, rawKAPFinancialIntegrityIssues(result)...)
	return out
}

func rawKAPFinancialIntegrityIssues(result SymbolAnalysis) []string {
	raw := result.Professional.RawKAPData
	if raw == nil || !raw.Computed || len(raw.FinancialFacts) == 0 {
		return nil
	}
	periods := map[string]map[string]bool{}
	values := map[string]map[string]float64{}
	for _, fact := range raw.FinancialFacts {
		if rawKAPFinancialFactRejected(fact) {
			continue
		}
		key, ok := rawKAPFinancialMetricKey(fact)
		if !ok {
			continue
		}
		period := rawKAPFinancialFactPeriod(fact)
		if period == "" {
			continue
		}
		if periods[key] == nil {
			periods[key] = map[string]bool{}
			values[key] = map[string]float64{}
		}
		periods[key][period] = true
		if _, exists := values[key][period]; !exists || math.Abs(fact.Value) > math.Abs(values[key][period]) {
			values[key][period] = fact.Value
		}
	}
	target := rawKAPFinancialTargetPeriod(periods)
	if target == "" {
		return nil
	}
	issues := []string{}
	for _, key := range []string{"total_assets", "current_assets", "equity", "current_liabilities", "revenue", "net_income"} {
		if len(periods[key]) == 0 {
			issues = append(issues, "kap_pdf_missing_metric_"+key)
			continue
		}
		if !periods[key][target] {
			issues = append(issues, fmt.Sprintf("kap_pdf_current_period_missing_%s_target_%s_latest_%s", key, target, rawKAPLatestPeriod(periods[key])))
		}
	}
	if totalAssets := values["total_assets"][target]; totalAssets > 0 {
		if equity := values["equity"][target]; equity > 0 && equity/totalAssets > 1.05 {
			issues = append(issues, fmt.Sprintf("kap_pdf_invalid_equity_assets_ratio_target_%s_ratio_%.2fx", target, equity/totalAssets))
		}
	}
	return uniqueSortedStringsAnalysis(issues)
}

func rawKAPFinancialFactRejected(fact kapingest.ExtractedFinancialFact) bool {
	return fact.ReviewRequired ||
		strings.EqualFold(fact.Certification.Status, "rejected") ||
		(fact.Confidence > 0 && fact.Confidence < 0.65)
}

func rawKAPFinancialMetricKey(fact kapingest.ExtractedFinancialFact) (string, bool) {
	statement := normalizeRawKAPFinancialText(fact.StatementType)
	line := normalizeRawKAPFinancialText(fact.LineItemNormalized + " " + fact.LineItemOriginal)
	isBalance := strings.Contains(statement, "balance") || strings.Contains(statement, "bilan")
	isIncome := strings.Contains(statement, "income") || strings.Contains(statement, "gelir")
	switch {
	case isBalance && rawKAPContainsAny(line, "total assets", "toplam varlik", "toplam aktif"):
		return "total_assets", true
	case isBalance && rawKAPContainsAny(line, "current assets", "donen varlik"):
		return "current_assets", true
	case isBalance && rawKAPContainsAny(line, "equity", "ozkaynak"):
		return "equity", true
	case isBalance && rawKAPContainsAny(line, "current liabilities", "kisa vadeli yukumluluk"):
		return "current_liabilities", true
	case isIncome && rawKAPContainsAny(line, "revenue", "hasilat", "satis gelirleri"):
		return "revenue", true
	case isIncome && rawKAPContainsAny(line, "net income", "net donem kari", "donem kari zarari"):
		return "net_income", true
	default:
		return "", false
	}
}

func rawKAPFinancialFactPeriod(fact kapingest.ExtractedFinancialFact) string {
	if fact.Period != nil && strings.TrimSpace(*fact.Period) != "" {
		return strings.TrimSpace(*fact.Period)
	}
	if fact.DocumentDate != nil {
		return strings.TrimSpace(*fact.DocumentDate)
	}
	return ""
}

func rawKAPFinancialTargetPeriod(periods map[string]map[string]bool) string {
	candidates := []string{}
	for _, key := range []string{"equity", "revenue", "net_income", "current_liabilities", "current_assets"} {
		for period := range periods[key] {
			if strings.TrimSpace(period) != "" {
				candidates = append(candidates, period)
			}
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[len(candidates)-1]
}

func rawKAPLatestPeriod(periods map[string]bool) string {
	items := make([]string, 0, len(periods))
	for period := range periods {
		if strings.TrimSpace(period) != "" {
			items = append(items, period)
		}
	}
	sort.Strings(items)
	if len(items) == 0 {
		return "none"
	}
	return items[len(items)-1]
}

func normalizeRawKAPFinancialText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"ı", "i", "İ", "i", "ğ", "g", "ü", "u", "ş", "s", "ö", "o", "ç", "c",
		"_", " ", "-", " ", "’", " ", "'", " ", ".", " ", ",", " ", ";", " ",
		":", " ", "(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
	)
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func rawKAPContainsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, normalizeRawKAPFinancialText(term)) {
			return true
		}
	}
	return false
}

func valuationModelEvidence(value ValuationConsistencyAssessment) []string {
	out := []string{}
	for _, model := range value.Models {
		out = append(out, fmt.Sprintf("%s=%.4f", model.Model, model.Value))
	}
	return out
}

func sectorModelEvidence(value SectorModelAssessment) []string {
	return []string{
		"profile=" + value.FinancialProfile,
		"actual_model=" + value.ActualValuationModel,
		"expected_models=" + strings.Join(value.ExpectedValuationModel, ","),
		fmt.Sprintf("metric_coverage=%.1f", value.MetricCoveragePct),
	}
}

func dailyTechnicalScore(daily TimeframeAnalysis, ok bool) float64 {
	if !ok {
		return 0
	}
	if daily.Professional.Technical.SignalGate.Score > 0 {
		return daily.Professional.Technical.SignalGate.Score
	}
	return daily.Score
}

func technicalGateEvidence(daily TimeframeAnalysis, ok bool) []string {
	if !ok {
		return []string{"daily_timeframe_missing"}
	}
	gate := daily.Professional.Technical.SignalGate
	return append(append([]string{}, gate.Passes...), gate.Blockers...)
}

func tradePlanScore(daily TimeframeAnalysis, ok bool) float64 {
	if ok && activeDecisionTradePlan(daily) {
		return 100
	}
	return 0
}

func tradePlanEvidence(daily TimeframeAnalysis, ok bool) []string {
	if !ok {
		return []string{"daily_timeframe_missing"}
	}
	plan := daily.TradePlan
	return []string{fmt.Sprintf(
		"direction=%s rejected=%t entry=%.4f-%.4f stop=%.4f target=%.4f reason=%s",
		plan.Direction, plan.Rejected, plan.EntryMin, plan.EntryMax,
		plan.StopLoss, plan.TakeProfit1, plan.RejectReason,
	)}
}

func nextSessionForecastDecisionOK(result SymbolAnalysis) bool {
	forecast, ok := decisionNextSessionForecast(result)
	if !ok {
		return true
	}
	if forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		return false
	}
	forecast = ApplyNextSessionForecastPublishState(forecast)
	return forecast.PointForecastPublishable
}

func nextSessionForecastDecisionGateStatus(result SymbolAnalysis) string {
	forecast, ok := decisionNextSessionForecast(result)
	if !ok {
		return "pass"
	}
	if nextSessionForecastDecisionOK(result) {
		return "pass"
	}
	forecast = ApplyNextSessionForecastPublishState(forecast)
	if strings.EqualFold(forecast.Status, "model_validation_failed") ||
		strings.EqualFold(forecast.Status, "technical_decision_context_failed") ||
		strings.EqualFold(forecast.Quality, "not_decision_grade") ||
		(forecast.TechnicalDecisionStatus != "" && !strings.EqualFold(forecast.TechnicalDecisionStatus, "pass")) {
		return "fail"
	}
	if strings.EqualFold(forecast.Quality, "provisional") ||
		(forecast.Confidence > 0 && forecast.Confidence < nextSessionPointForecastMinConfidence) {
		return "limited"
	}
	return "fail"
}

func nextSessionForecastDecisionScore(result SymbolAnalysis) float64 {
	forecast, ok := decisionNextSessionForecast(result)
	if !ok {
		return 100
	}
	score := forecast.Confidence
	if score <= 0 {
		score = 50
	}
	if forecast.BacktestSamples >= 20 {
		score = math.Min(score, forecast.BacktestDirectionHitRatePct)
		if forecast.BacktestCloseMAEPct > 0 {
			score = math.Min(score, mathutil.Clamp(100-forecast.BacktestCloseMAEPct*20, 0, 100))
		}
	}
	if !nextSessionForecastDecisionOK(result) {
		score = math.Min(score, 49)
	}
	return mathutil.Clamp(score, 0, 100)
}

func nextSessionForecastDecisionCurrent(result SymbolAnalysis) string {
	forecast, ok := decisionNextSessionForecast(result)
	if !ok {
		return "not_computed_not_required"
	}
	forecast = ApplyNextSessionForecastPublishState(forecast)
	return fmt.Sprintf(
		"status=%s quality=%s point=%s confidence=%.0f/100 technical=%s backtest_n=%d direction_hit=%.2f%% close_mae=%.2f%% actual_available=%t",
		emptyDecisionText(forecast.Status, "unknown"),
		emptyDecisionText(forecast.Quality, "unknown"),
		emptyDecisionText(forecast.PointForecastStatus, "unknown"),
		forecast.Confidence,
		emptyDecisionText(forecast.TechnicalDecisionStatus, "unknown"),
		forecast.BacktestSamples,
		forecast.BacktestDirectionHitRatePct,
		forecast.BacktestCloseMAEPct,
		forecast.ActualAvailable,
	)
}

func nextSessionForecastDecisionEvidence(result SymbolAnalysis) []string {
	forecast, ok := decisionNextSessionForecast(result)
	if !ok {
		return []string{"next_session_forecast_not_computed"}
	}
	forecast = ApplyNextSessionForecastPublishState(forecast)
	evidence := []string{
		fmt.Sprintf("model=%s", forecast.Model),
		fmt.Sprintf("forecast_for=%s last_close=%.4f published_open=%s published_close=%s", forecast.ForecastFor, forecast.LastClose, nextSessionPublishedDecisionValue(forecast.PublishedPredictedOpen), nextSessionPublishedDecisionValue(forecast.PublishedPredictedClose)),
		fmt.Sprintf("point_forecast_publishable=%t status=%s reason=%s", forecast.PointForecastPublishable, forecast.PointForecastStatus, forecast.PointForecastSuppressionReason),
	}
	if forecast.ActualAvailable {
		evidence = append(evidence, fmt.Sprintf("official_actual open=%.4f close=%.4f source=%s", forecast.ActualOpen, forecast.ActualClose, forecast.ActualSourcePath))
	}
	evidence = append(evidence, forecast.Warnings...)
	evidence = append(evidence, compactStrings(forecast.BiasReasons, 4)...)
	return evidence
}

func nextSessionPublishedDecisionValue(value *float64) string {
	if value == nil || *value <= 0 {
		return "not_published"
	}
	return fmt.Sprintf("%.4f", *value)
}

func nextSessionForecastDecisionNextSteps(result SymbolAnalysis) []string {
	if nextSessionForecastDecisionOK(result) {
		return nil
	}
	symbol := strings.ToUpper(strings.TrimSpace(result.Symbol))
	if symbol == "" {
		symbol = "{SYMBOL}"
	}
	return []string{
		fmt.Sprintf("go run ./cmd/hissebot forecast-audit -symbol %s -from 2026-05-22 -to 2026-06-22 -data ./data -out ./data/equities -limit 0", symbol),
		"Model decision-grade değilse AL/SAT ve otomatik emir kapısını kapalı tut.",
		"Teknik karar kapısı, backtest yön uyumu ve kapanış MAE eşiği geçmeden forecast'i karar girdisi yapma.",
	}
}

func decisionNextSessionForecast(result SymbolAnalysis) (NextSessionForecast, bool) {
	forecast := result.NextSessionForecast
	if forecast.Computed {
		return forecast, true
	}
	if daily, ok := result.Timeframes["1D"]; ok && daily.NextSessionForecast.Computed {
		return daily.NextSessionForecast, true
	}
	return NextSessionForecast{}, false
}

func macroDecisionScore(result SymbolAnalysis) float64 {
	if result.Professional.TCMBEVDSContext.ForecastImpact.Computed {
		return result.Professional.TCMBEVDSContext.ForecastImpact.Confidence
	}
	return result.Professional.Market.GDP.Score
}

func macroClassificationEvidence(result SymbolAnalysis) []string {
	impact := result.Professional.TCMBEVDSContext.ForecastImpact
	return append([]string{fmt.Sprintf("decision_use=%s severity=%s", impact.DecisionUse, impact.Severity)}, impact.Blockers...)
}

func microstructureScore(result SymbolAnalysis) float64 {
	if result.Professional.Market.Microstructure == nil {
		return 0
	}
	return result.Professional.Market.Microstructure.Score
}

func microstructureClassificationEvidence(result SymbolAnalysis) []string {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return []string{"market_microstructure_missing"}
	}
	return []string{fmt.Sprintf(
		"quote=%t order_book=%t depth=%t akd=%t custody=%t equilibrium=%t",
		micro.Quote.Available, micro.OrderBook.Available, micro.Depth.Available,
		micro.BrokerageDistribution.Available, micro.Custody.Available, micro.Equilibrium.Available,
	)}
}

func automaticExecutionEvidence(result SymbolAnalysis) []string {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return []string{"market_microstructure_missing"}
	}
	return append([]string{}, micro.Liquidity.AutomaticOrderBlockers...)
}

func centralClassAnswer(class DecisionClassResult) string {
	if class.Qualified {
		return "Merkezi karar kapılarının tamamı geçti: " + class.Decision
	}
	return class.Decision + "; geçmeyen kontroller: " + strings.Join(class.FailedGateLabels, ", ")
}

func classificationConsistencyViolations(c DecisionClassification) []string {
	issues := []string{}
	if c.Classes.LargeInvestor.Qualified && !c.Classes.InstitutionalPortfolio.Qualified {
		issues = append(issues, "large_investor_pass_without_institutional_portfolio")
	}
	if c.Classes.RetailDirect.Qualified &&
		(!classificationGatePassed(c.Gates, "technical_signal") || !classificationGatePassed(c.Gates, "active_trade_plan")) {
		issues = append(issues, "retail_direct_pass_without_technical_signal_and_trade_plan")
	}
	if c.Classes.ValueInvesting.Qualified && !c.ValuationConsistency.Publishable {
		issues = append(issues, "value_investing_pass_with_unpublishable_valuation")
	}
	if c.Classes.AutomaticOrder.Qualified && !c.Classes.RetailDirect.Qualified {
		issues = append(issues, "automatic_order_pass_without_retail_direct_class")
	}
	return issues
}

func classificationGatePassed(gates []ClassificationGate, key string) bool {
	for _, gate := range gates {
		if gate.Key == key {
			return gate.Passed
		}
	}
	return false
}

func combineCentralClassStatus(classes ...DecisionClassResult) string {
	for _, class := range classes {
		if !class.Qualified {
			return "fail"
		}
	}
	return "pass"
}

func containsExactString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func uniqueSortedStringsAnalysis(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
