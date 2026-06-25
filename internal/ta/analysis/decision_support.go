package analysis

import (
	"fmt"
	"math"
	"strings"

	"hissebot/internal/services/pricequality"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
)

type DecisionSupportReport struct {
	SchemaVersion     int                       `json:"schema_version"`
	Scope             string                    `json:"scope"`
	Status            string                    `json:"status"`
	Summary           string                    `json:"summary"`
	Classification    DecisionClassification    `json:"classification"`
	Institutional     InstitutionalDecision     `json:"institutional_decision"`
	Retail            RetailDecision            `json:"retail_decision"`
	UseCaseMatrix     []DecisionUseCase         `json:"use_case_matrix"`
	ActionGates       []DecisionActionGate      `json:"action_gates"`
	RequiredMinimums  []DecisionRequirement     `json:"required_minimums"`
	MissingInputs     []DecisionMissingInput    `json:"missing_inputs,omitempty"`
	CompletionActions []DecisionCompletionStep  `json:"completion_actions"`
	BatchRefresh      DecisionCompletionStep    `json:"batch_refresh"`
	ProductScope      []DecisionProductScopeRow `json:"product_scope"`
}

// InstitutionalDecision is the primary answer for a large investor. Missing
// inputs can lower confidence or produce WAIT/REJECT, but they do not turn the
// report into a research/radar product.
type InstitutionalDecision struct {
	Audience           string   `json:"audience"`
	Decision           string   `json:"decision"`
	PositionAction     string   `json:"position_action"`
	CanOpenPosition    bool     `json:"can_open_position"`
	Status             string   `json:"status"`
	Score              float64  `json:"score"`
	Confidence         float64  `json:"confidence"`
	OneLineAnswer      string   `json:"one_line_answer"`
	DecisionReasons    []string `json:"decision_reasons,omitempty"`
	BlockingReasons    []string `json:"blocking_reasons,omitempty"`
	ApprovalConditions []string `json:"approval_conditions,omitempty"`
}

// RetailDecision gives a position-aware, direct AL/SAT answer. New-position
// and existing-position actions are separate because a single AL/SAT label is
// ambiguous when the report does not know whether the reader already owns the
// security.
type RetailDecision struct {
	Audience               string   `json:"audience"`
	Signal                 string   `json:"signal"`
	NewPositionAction      string   `json:"new_position_action"`
	ExistingPositionAction string   `json:"existing_position_action"`
	Actionable             bool     `json:"actionable"`
	Status                 string   `json:"status"`
	Confidence             float64  `json:"confidence"`
	TimeHorizon            string   `json:"time_horizon"`
	OneLineAnswer          string   `json:"one_line_answer"`
	EntryMin               float64  `json:"entry_min,omitempty"`
	EntryMax               float64  `json:"entry_max,omitempty"`
	StopLoss               float64  `json:"stop_loss,omitempty"`
	Target1                float64  `json:"target_1,omitempty"`
	Target2                float64  `json:"target_2,omitempty"`
	Trigger                string   `json:"trigger,omitempty"`
	Invalidation           string   `json:"invalidation,omitempty"`
	Reasons                []string `json:"reasons,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

type DecisionUseCase struct {
	UseCase string `json:"use_case"`
	Allowed bool   `json:"allowed"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

type DecisionActionGate struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Blocking  bool     `json:"blocking"`
	Required  string   `json:"required"`
	Current   string   `json:"current"`
	Reason    string   `json:"reason"`
	Evidence  []string `json:"evidence,omitempty"`
	NextSteps []string `json:"next_steps,omitempty"`
}

type DecisionRequirement struct {
	Area     string `json:"area"`
	Required string `json:"required"`
	Current  string `json:"current"`
	Status   string `json:"status"`
	Why      string `json:"why"`
	NextStep string `json:"next_step,omitempty"`
}

type DecisionMissingInput struct {
	Key           string `json:"key"`
	Category      string `json:"category"`
	Priority      string `json:"priority"`
	WhyItMatters  string `json:"why_it_matters"`
	HowToComplete string `json:"how_to_complete"`
}

type DecisionCompletionStep struct {
	Priority           int      `json:"priority"`
	Area               string   `json:"area"`
	Action             string   `json:"action"`
	Command            string   `json:"command,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

type DecisionProductScopeRow struct {
	Product string `json:"product"`
	Status  string `json:"status"`
	Notes   string `json:"notes"`
}

func BuildDecisionSupport(result SymbolAnalysis) *DecisionSupportReport {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return nil
	}
	classification := result.DecisionClassification
	if classification.SchemaVersion == 0 {
		classification = ClassifyDecision(result)
		result.DecisionClassification = classification
		result = ApplyDecisionClassification(result)
	}
	daily, hasDaily := result.Timeframes["1D"]
	evidencePass := statusPass(result.Professional.EvidencePolicy.Status)
	macroOK := macroDecisionOK(result)
	transactionOK := statusPass(result.InvestorQA.InstitutionalViews.FinancialTransactionUse.Status)

	report := &DecisionSupportReport{
		SchemaVersion:    2,
		Scope:            "equity_decision_support_standard",
		Classification:   classification,
		RequiredMinimums: decisionSupportMinimums(result, daily, hasDaily),
		ProductScope:     decisionProductScope(),
	}
	report.Institutional = buildInstitutionalDecision(result)
	retailSignalOK := classification.Classes.RetailDirect.Qualified
	report.Retail = buildRetailDecision(result, daily, hasDaily, retailSignalOK)
	if report.Retail.Actionable || report.Retail.Status == "buy" {
		impact := result.Professional.TCMBEVDSContext.ForecastImpact
		if impact.Computed && impact.DecisionUse == "blocking_headwind" {
			report.Retail.Warnings = appendUniqueAnalysisString(report.Retail.Warnings, "makro ters rüzgar aktif: "+strings.Join(impact.Drivers, "; ")+"; sinyali dikkatli uygula")
		} else if !macroOK {
			report.Retail.Warnings = appendUniqueAnalysisString(report.Retail.Warnings, "makro bağlam tamamlanmadı (TCMB/TÜİK/EVDS veya canlı snapshot eksik); sinyal teknik ve fiyat kalitesine dayalı")
		}
		if !evidencePass {
			report.Retail.Warnings = appendUniqueAnalysisString(report.Retail.Warnings, "kanıt politikası kapısı geçmedi; sinyali sınırlı portföy riskiyle uygula")
		}
		if !transactionOK {
			report.Retail.Warnings = appendUniqueAnalysisString(report.Retail.Warnings, "finansal işlem kapısı geçmedi; kurumsal kullanım için ek doğrulama gerekiyor")
		}
	}
	report.ActionGates = decisionActionGates(result, daily, hasDaily)
	report.UseCaseMatrix = []DecisionUseCase{
		useCase("Büyük yatırımcı karar raporu", classification.Classes.LargeInvestor.Qualified, classification.Classes.LargeInvestor.Summary),
		useCase("Küçük yatırımcı doğrudan AL/SAT raporu", classification.Classes.RetailDirect.Qualified, classification.Classes.RetailDirect.Summary),
		useCase("Teknik durum özeti", hasDaily, "Günlük teknik veri varsa trend, momentum, destek/direnç ve teyit şartları özetlenebilir."),
		useCase("Finans uzmanı ön değerlendirme", result.Professional.DataQuality > 0 || result.InvestorQA.Computed, "Finansal veri, kalite kapıları ve yatırımcı soru-cevap katmanı ön inceleme için kullanılabilir."),
		useCase("Risk komitesi ön notu", len(result.InstitutionalValidation.Checks) > 0, "Kurumsal doğrulama kontrolleri üretildiği için komite ön notu hazırlanabilir."),
		useCase("Destek/direnç ve teyit şartı takibi", hasSupportOrResistance(daily) || hasTradePlan(daily), "Fiyat seviyeleri, geçersiz kılma ve teyit şartları izlenebilir."),
		useCase("AL/SAT sinyali", classification.Classes.RetailDirect.Qualified, classification.Classes.RetailDirect.Summary),
		useCase("Production trading / otomatik emir", classification.Classes.AutomaticOrder.Qualified, classification.Classes.AutomaticOrder.Summary),
		useCase("Kurumsal portföy pozisyonu", classification.Classes.InstitutionalPortfolio.Qualified, classification.Classes.InstitutionalPortfolio.Summary),
		{UseCase: "Tek başına yatırım tavsiyesi", Allowed: false, Status: "fail", Reason: "Rapor karar destek çıktısıdır; tek başına yatırım tavsiyesi olarak kullanılamaz."},
	}
	report.MissingInputs = decisionMissingInputs(result)
	report.CompletionActions = decisionCompletionActions(result)
	report.BatchRefresh = DecisionCompletionStep{
		Priority: 90,
		Area:     "all_equities_refresh",
		Action:   "Kapı ve kalite güncellemelerinden sonra tüm hisse evreni için raporları aynı standartla yeniden üret.",
		Command:  "go run ./cmd/hissebot analyze --all --provider tradingview",
		AcceptanceCriteria: []string{
			"Her hisse analiz klasöründe decision_support_standard.json üretilir.",
			"analysis.json içindeki decision_support.use_case_matrix AL/SAT ve production trading kararlarını kapılara bağlar.",
			"quality_control_report.json eksik veri ve kabul kriterlerini aynı kapılarla gösterir.",
		},
	}

	switch {
	case classification.Classes.AutomaticOrder.Qualified:
		report.Status = "decision_ready"
		report.Summary = "Büyük yatırımcı ve küçük yatırımcı kararları üretildi; tüm canlı kullanım kapıları da geçiyor."
	case classification.Classes.LargeInvestor.Qualified || classification.Classes.RetailDirect.Qualified:
		report.Status = "decision_ready_with_execution_limits"
		report.Summary = "Büyük yatırımcı ve küçük yatırımcı kararları üretildi; canlı emir için geçmeyen execution kapıları ayrıca gösteriliyor."
	case len(result.Timeframes) > 0:
		report.Status = "decision_issued_with_limitations"
		report.Summary = "Merkezi motor büyük yatırımcı ve küçük yatırımcı kararlarını üretti; geçmeyen kapılar nedeniyle pozisyon/AL-SAT onayı verilmedi."
	default:
		report.Status = "decision_unavailable"
		report.Summary = "Karar üretmek için günlük fiyat ve temel doğrulama çıktıları yok."
	}
	return report
}

func buildInstitutionalDecision(result SymbolAnalysis) InstitutionalDecision {
	views := result.InvestorQA.InstitutionalViews
	portfolio := views.Portfolio
	class := result.DecisionClassification.Classes.LargeInvestor
	out := InstitutionalDecision{
		Audience:           "large_investor",
		Decision:           "BEKLE",
		PositionAction:     "YENI_POZISYON_ACMA",
		Status:             "wait",
		Score:              portfolio.Score,
		Confidence:         portfolio.Confidence,
		DecisionReasons:    compactStrings(portfolio.Passes, 5),
		BlockingReasons:    compactStrings(portfolio.Blockers, 8),
		ApprovalConditions: compactStrings(portfolio.RequiredActions, 8),
	}
	out.BlockingReasons = compactStrings(append(out.BlockingReasons, class.FailedGateLabels...), 10)
	if out.Confidence <= 0 {
		out.Confidence = result.InvestorQA.Confidence
	}
	switch {
	case class.Qualified:
		out.Decision = "ONAYLA"
		out.PositionAction = "POZISYON_AC"
		out.CanOpenPosition = true
		out.Status = "approved"
	default:
		out.Decision = "REDDET"
		out.PositionAction = "POZISYON_ACMA"
		out.Status = "rejected"
	}
	if strings.EqualFold(result.InvestorQA.Decision, "RED") {
		out.Decision = "REDDET"
		out.PositionAction = "POZISYON_ACMA_RISKI_AZALT"
		out.CanOpenPosition = false
		out.Status = "rejected"
	}
	out.OneLineAnswer = fmt.Sprintf("%s: %s. Kurumsal skor %.0f/100, kurumsal karar güveni %.0f/100.", out.Decision, institutionalPositionActionTR(out.PositionAction), out.Score, out.Confidence)
	return out
}

func buildRetailDecision(result SymbolAnalysis, daily TimeframeAnalysis, hasDaily, liveBuyAllowed bool) RetailDecision {
	out := RetailDecision{
		Audience:               "small_investor",
		Signal:                 "BEKLE",
		NewPositionAction:      "BEKLE",
		ExistingPositionAction: "TUT_RISKI_IZLE",
		Status:                 "wait",
		Confidence:             result.InvestorQA.Confidence,
		TimeHorizon:            "kısa/orta vade",
		Reasons:                compactStrings(retailDecisionReasons(result, daily, hasDaily), 6),
	}
	var buy, sell *investorAction
	for i := range result.InvestorQA.ActionMatrix {
		item := result.InvestorQA.ActionMatrix[i]
		action := strings.ToUpper(strings.TrimSpace(item.Action))
		candidate := &investorAction{
			Action: item.Action, Current: item.CurrentSignal, Status: item.Status,
			Confidence: item.Confidence, TimeHorizon: item.TimeHorizon,
			EntryMin: item.EntryMin, EntryMax: item.EntryMax, StopLoss: item.StopLoss,
			Target1: item.Target1, Target2: item.Target2, Trigger: item.Trigger,
			Invalidation: item.Invalidation, Blockers: item.Blockers,
		}
		if strings.Contains(action, "SAT") || strings.Contains(action, "RISK") {
			sell = candidate
		} else if strings.Contains(action, "AL") {
			buy = candidate
		}
	}
	switch {
	case sell != nil && sell.Current && liveBuyAllowed:
		out.Signal = "SAT"
		out.NewPositionAction = "ALMA"
		out.ExistingPositionAction = "SAT_RISKI_AZALT"
		out.Actionable = statusPass(sell.Status)
		out.Status = "sell"
		applyRetailAction(&out, sell)
	case buy != nil && buy.Current && statusPass(buy.Status) && liveBuyAllowed:
		out.Signal = "AL"
		out.NewPositionAction = "AL"
		out.ExistingPositionAction = "TUT_POZISYONU_ARTIR"
		out.Actionable = true
		out.Status = "buy"
		applyRetailAction(&out, buy)
	case strings.EqualFold(result.InvestorQA.Decision, "RED"):
		out.Signal = "SAT"
		out.NewPositionAction = "ALMA"
		out.ExistingPositionAction = "SAT_RISKI_AZALT"
		out.Status = "sell"
		if sell != nil {
			applyRetailAction(&out, sell)
		}
	case buy != nil && buy.Current:
		out.Signal = "ALIM_ADAYI"
		out.NewPositionAction = "BEKLE"
		out.ExistingPositionAction = "TUT_RISKI_IZLE"
		out.Status = "conditional_buy"
		applyRetailAction(&out, buy)
		out.Warnings = append(out.Warnings, "koşullu alım adayı: tam AL için teknik kanıt kapısı veya fiyat kalite kapısı geçmeli")
	default:
		out.Signal = "BEKLE"
		out.NewPositionAction = "BEKLE"
		out.ExistingPositionAction = "TUT_RISKI_IZLE"
	}
	if !hasDaily {
		out.Signal = "KARAR_YOK"
		out.NewPositionAction = "ISLEM_YAPMA"
		out.ExistingPositionAction = "RISK_LIMITINI_KORU"
		out.Actionable = false
		out.Status = "unavailable"
		out.Warnings = append(out.Warnings, "günlük fiyat ve teknik analiz yok")
	}
	if hasDaily && !liveBuyAllowed {
		out.Signal = "BEKLE"
		out.NewPositionAction = "BEKLE"
		out.ExistingPositionAction = "TUT_RISKI_IZLE"
		out.Actionable = false
		out.Status = "central_gate_blocked"
		if score := result.DecisionClassification.Classes.RetailDirect.Score; score > 0 {
			out.Confidence = math.Min(out.Confidence, score)
		}
		out.Warnings = append(out.Warnings, "merkezi sınıflandırma doğrudan AL/SAT kullanımını onaylamadı: "+strings.Join(result.DecisionClassification.Classes.RetailDirect.FailedGateLabels, ", "))
	}
	if !decisionPriceOK(result) {
		out.Actionable = false
		out.Warnings = append(out.Warnings, "güncel karar fiyatı kaynaklarla mutabık değil; karar yeni fiyatla yenilenmeli")
	}
	if buy != nil && !statusPass(buy.Status) {
		out.Warnings = append(out.Warnings, compactStrings(buy.Blockers, 4)...)
	}
	out.Warnings = compactStrings(out.Warnings, 8)
	out.OneLineAnswer = fmt.Sprintf("Yeni pozisyon: %s. Elde varsa: %s. Ana sinyal: %s; sinyal güveni %.0f/100.", retailActionTR(out.NewPositionAction), retailActionTR(out.ExistingPositionAction), out.Signal, out.Confidence)
	return out
}

type investorAction struct {
	Action, Status, TimeHorizon, Trigger, Invalidation string
	Current                                            bool
	Confidence, EntryMin, EntryMax, StopLoss           float64
	Target1, Target2                                   float64
	Blockers                                           []string
}

func applyRetailAction(out *RetailDecision, action *investorAction) {
	if out == nil || action == nil {
		return
	}
	if action.Confidence > 0 {
		out.Confidence = action.Confidence
	}
	if strings.TrimSpace(action.TimeHorizon) != "" {
		out.TimeHorizon = action.TimeHorizon
	}
	out.EntryMin = action.EntryMin
	out.EntryMax = action.EntryMax
	out.StopLoss = action.StopLoss
	out.Target1 = action.Target1
	out.Target2 = action.Target2
	out.Trigger = action.Trigger
	out.Invalidation = action.Invalidation
}

func retailDecisionReasons(result SymbolAnalysis, daily TimeframeAnalysis, hasDaily bool) []string {
	reasons := []string{}
	if strings.TrimSpace(result.InvestorQA.DecisionLabel) != "" {
		reasons = append(reasons, result.InvestorQA.DecisionLabel)
	}
	if hasDaily {
		reasons = append(reasons, fmt.Sprintf("günlük teknik skor %.0f/100, yön %s", daily.Score, emptyDecisionText(daily.TrendBias, "belirsiz")))
	}
	if strings.TrimSpace(result.InvestorQA.TopOpportunity) != "" {
		reasons = append(reasons, "fırsat: "+result.InvestorQA.TopOpportunity)
	}
	if strings.TrimSpace(result.InvestorQA.TopRisk) != "" {
		reasons = append(reasons, "risk: "+result.InvestorQA.TopRisk)
	}
	return reasons
}

func institutionalPositionActionTR(action string) string {
	switch action {
	case "POZISYON_AC":
		return "pozisyon açılabilir"
	case "SINIRLI_POZISYON_TASLAGI":
		return "yalnızca limitli pozisyon taslağı komiteye sunulur"
	case "POZISYON_ACMA_RISKI_AZALT":
		return "yeni pozisyon açılmaz, mevcut risk azaltılır"
	default:
		return "yeni pozisyon açılmaz"
	}
}

func retailActionTR(action string) string {
	switch action {
	case "AL":
		return "AL"
	case "ALMA":
		return "ALMA"
	case "SAT_RISKI_AZALT":
		return "SAT / RİSKİ AZALT"
	case "TUT_POZISYONU_ARTIR":
		return "TUT / uygun girişte artır"
	case "TUT_RISKI_IZLE":
		return "TUT / riski izle"
	case "ISLEM_YAPMA":
		return "İŞLEM YAPMA"
	case "RISK_LIMITINI_KORU":
		return "RİSK LİMİTİNİ KORU"
	default:
		return action
	}
}

func useCase(name string, allowed bool, reason string) DecisionUseCase {
	status := "fail"
	if allowed {
		status = "pass"
	}
	return DecisionUseCase{UseCase: name, Allowed: allowed, Status: status, Reason: reason}
}

func decisionActionGates(result SymbolAnalysis, daily TimeframeAnalysis, hasDaily bool) []DecisionActionGate {
	gates := []DecisionActionGate{}
	bankCoreOK := !bankCoreMetricsMissingForDecision(result)
	add := func(name, status, required, current, reason string, evidence, next []string) {
		gates = append(gates, DecisionActionGate{
			Name:      name,
			Status:    normalizeGateStatus(status),
			Blocking:  !statusPass(status),
			Required:  required,
			Current:   current,
			Reason:    reason,
			Evidence:  compactStrings(evidence, 10),
			NextSteps: compactStrings(next, 8),
		})
	}
	add("report_security_validation", result.InstitutionalValidation.Status, "pass", fmt.Sprintf("%s %.0f/100", emptyDecisionText(result.InstitutionalValidation.Status, "none"), result.InstitutionalValidation.Score), result.InstitutionalValidation.Summary, institutionalFailedDetails(result), nil)
	add("strict_evidence_policy", result.Professional.EvidencePolicy.Status, "pass", strings.Join(result.Professional.EvidencePolicy.BlockingIssues, ", "), "Kanıt politikası geçmeden hedef fiyat, yatırım kararı ve işlem kullanımı baskılanır.", result.Professional.EvidencePolicy.Notes, result.Professional.EvidencePolicy.RequiredEvidence)
	decisionPriceStatus, decisionPriceCurrent, decisionPriceReason, decisionPriceEvidence, decisionPriceNext := decisionPriceGate(result)
	add("decision_price", decisionPriceStatus, "güncel analiz kapanışı + kaynak mutabakatı", decisionPriceCurrent, decisionPriceReason, decisionPriceEvidence, decisionPriceNext)
	priceStatus, priceCurrent, priceReason, priceEvidence, priceNext := priceCloseDecisionGate(result)
	add("verified_price_close", priceStatus, "official final close + kaynak mutabakatı + taze veri", priceCurrent, "Bu kapı yalnız production/otomatik emir içindir. "+priceReason, priceEvidence, priceNext)
	add("macro_regime_confirmation", macroDecisionGateStatus(result), "TCMB EVDS analysis_ready + point-in-time güvenli + ağır negatif makro ters rüzgar yok", macroDecisionCurrent(result), "Fiyat beklentisi yalnız teknik skorla değil, faiz/enflasyon/kur/rezerv/kredi rejimiyle koşullandırılır.", macroDecisionEvidence(result), macroDecisionNextSteps(result))
	add("market_microstructure_gate", marketMicrostructureGateStatus(result), "live quote + order book + KDM2 derinlik + AKD + takas", marketMicrostructureCurrent(result), "Fiyat tahmini ve AL/SAT kararı son likidite, aracı kurum dağılımı ve takas görünümü olmadan tamamlanmış sayılmaz.", marketMicrostructureEvidence(result), marketMicrostructureNextSteps(result))
	add("automatic_order_liquidity_gate", automaticOrderLiquidityGateStatus(result), "microstructure complete + top-of-book + spread <= 50 bps", automaticOrderLiquidityCurrent(result), "Otomatik emir için karar sinyalinden ayrı olarak anlık spread ve defter yürütme riski geçmelidir.", automaticOrderLiquidityEvidence(result), automaticOrderLiquidityNextSteps(result))
	if hasDaily {
		gate := daily.Professional.Technical.SignalGate
		add("daily_technical_signal_gate", gate.Status, "pass", fmt.Sprintf("%s %.1f/100 actionable=%t", gate.Label, gate.Score, gate.Actionable), "Günlük aktif işlem sinyali için plan, risk/ödül, hacim, indikatör, formasyon, backtest ve fiyat düzeltme kapıları birlikte geçmelidir.", gate.Evidence, gate.Blockers)
		bt := daily.Professional.Backtest
		btStatus := "pass"
		if !bt.BacktestSafe || bt.LookaheadViolations > 0 || bt.Trades < 30 || bt.OutOfSampleTrades < 10 || bt.Expectancy <= 0 || bt.OutOfSampleReturn <= 0 {
			btStatus = "fail"
		}
		add("walk_forward_backtest", btStatus, "30+ işlem, 10+ OOS işlem, pozitif expectancy ve pozitif OOS getiri", fmt.Sprintf("trades=%d oos=%d expectancy=%.2f%% oos=%.2f%%", bt.Trades, bt.OutOfSampleTrades, bt.Expectancy*100, bt.OutOfSampleReturn*100), "Canlı işlem sinyali için geçmiş performans hipotez değil, ölçülü edge göstermelidir.", nil, []string{"Daha uzun günlük OHLCV geçmişi bağla.", "Walk-forward ve işlem maliyeti sonrası sonuçları yenile."})
		stats := daily.Professional.SignalStats
		statsStatus := "pass"
		if stats.InsufficientData || stats.SampleSize < 30 || stats.WinRate < 0.52 || stats.AverageForwardReturn <= 0 {
			statsStatus = "limited"
		}
		add("signal_success_rate", statsStatus, "30+ benzer rejim, >=52% win rate, pozitif ileri getiri", fmt.Sprintf("n=%d win=%.1f%% forward=%.2f%%", stats.SampleSize, stats.WinRate*100, stats.AverageForwardReturn*100), "Mevcut rejimin geçmişte çalışıp çalışmadığı ayrı ölçülmelidir.", nil, []string{"Daha uzun fiyat geçmişi ve rejim kırılımı ekle."})
	} else {
		add("daily_technical_signal_gate", "fail", "1D teknik analiz", "missing", "Günlük teknik analiz olmadan AL/SAT veya risk kapısı kurulamaz.", nil, []string{"TradingView günlük OHLCV verisini yenile."})
	}
	add("next_session_forecast_model", nextSessionForecastDecisionGateStatus(result), "decision_grade forecast; resmi gerçekleşen yalnız doğrulama kanıtıdır", nextSessionForecastDecisionCurrent(result), "Sonraki seans tahmin modeli geriye dönük doğrulama, teknik karar kapısı ve güven eşiğini geçmeden AL/SAT veya emir girdisi olamaz.", nextSessionForecastDecisionEvidence(result), nextSessionForecastDecisionNextSteps(result))
	add("quant_risk_gate", quantDecisionGateStatus(result), quantDecisionRequirement(result), quantDecisionCurrent(result), "Nicel risk/getiri katmanı, teknik ve temel kararın portföy riskiyle taşınabilir olup olmadığını ölçer.", quantDecisionEvidence(result), quantDecisionNextSteps(result))
	add("stat_economic_consistency_gate", statEconomicGateStatus(result), "composite >= 58 + data/model risk fail değil", statEconomicCurrent(result), "İstatistiksel ve ekonomik tutarlılık kapısı veri kalitesi, faktör, rejim, stres, makro, finansal kalite, likidite ve validasyonu birlikte ölçer.", statEconomicEvidence(result), statEconomicNextSteps(result))
	add("advanced_analysis_production_gate", advancedDecisionGateStatus(result), "faz 0-10 computed + production readiness limited/fail değil", advancedDecisionCurrent(result), "Roadmap fazları veri güvenliği, faktör, makro, finansal kalite, değerleme, event-study, monitoring, likidite ve production orkestrasyonunu tek kapıda denetler.", advancedDecisionEvidence(result), advancedDecisionNextSteps(result))
	valueStatus := "fail"
	if result.Professional.ValueInvesting.Computed {
		valueStatus = "pass"
		if result.Professional.ValueInvesting.Confidence < 65 {
			valueStatus = "limited"
		}
	}
	valueEvidence := append([]string{}, result.Professional.ValueInvesting.Warnings...)
	valueNext := append([]string{}, result.Professional.InvestmentResearch.ValuationBridge.MissingInputs...)
	if !bankCoreOK {
		valueStatus = "limited"
		valueEvidence = append(valueEvidence, "bank_regulatory_metrics_missing")
		valueNext = append(valueNext, "Banka için SYR, çekirdek sermaye, NPL, karşılık kapsamı, NIM, LCR, kredi/mevduat, mevduat maliyeti ve kredi/mevduat spread'i verilerini bağla.")
	}
	add("value_investing", valueStatus, "içsel değer, güvenlik marjı, sektör modeli ve >=65/100 güven", decisionValueCurrent(result, bankCoreOK), "Hisse raporu için teknik sinyal tek başına yeterli değildir; bilanço, sektör metrikleri ve değerleme kanıtı gerekir.", valueEvidence, valueNext)
	portfolioStatus := result.InvestorQA.InstitutionalViews.Portfolio.Status
	portfolioEvidence := append([]string{}, result.InvestorQA.InstitutionalViews.Portfolio.Passes...)
	portfolioNext := append([]string{}, result.InvestorQA.InstitutionalViews.Portfolio.RequiredActions...)
	if !bankCoreOK {
		portfolioStatus = "limited"
		portfolioEvidence = append(portfolioEvidence, "bank_regulatory_metrics_missing")
		portfolioNext = append(portfolioNext, "Kurumsal portföy kararı için banka çekirdek metriklerini tamamla.")
	}
	add("portfolio_gate", portfolioStatus, "pass", fmt.Sprintf("%s %.1f/100", portfolioStatus, result.InvestorQA.InstitutionalViews.Portfolio.Score), result.InvestorQA.InstitutionalViews.Portfolio.Takeaway, portfolioEvidence, portfolioNext)
	add("trading_edge_gate", result.InvestorQA.InstitutionalViews.TradingEdge.TransactionUseStatus, "pass", fmt.Sprintf("%s %.1f/100", result.InvestorQA.InstitutionalViews.TradingEdge.TransactionUseStatus, result.InvestorQA.InstitutionalViews.TradingEdge.Score), result.InvestorQA.InstitutionalViews.TradingEdge.TransactionUseAnswer, result.InvestorQA.InstitutionalViews.TradingEdge.Blockers, result.InvestorQA.InstitutionalViews.TradingEdge.RequiredActions)
	prodStatus := "fail"
	if result.Professional.DataGovernance.ProductionReady {
		prodStatus = "pass"
	}
	add("financial_data_production_ready", prodStatus, "production_ready=true ve backtest_safe=true", fmt.Sprintf("production_ready=%t backtest_safe=%t current_decision_safe=%t status=%s", result.Professional.DataGovernance.ProductionReady, result.Professional.DataGovernance.BacktestSafe, financialStatementDecisionSafe(result), result.Professional.DataGovernance.AvailabilityStatus), "Production trading için tüm tarihsel publish-date/available-at zinciri doğrulanmalıdır; güncel karar güvenliği ayrı değerlendirilir.", result.Professional.DataGovernance.Warnings, []string{"Production modu için tarihsel publish-date zincirini doğrula."})
	return gates
}

func decisionSupportMinimums(result SymbolAnalysis, daily TimeframeAnalysis, hasDaily bool) []DecisionRequirement {
	reqs := []DecisionRequirement{}
	add := func(area, required, current string, pass bool, why, next string) {
		status := "fail"
		if pass {
			status = "pass"
		}
		reqs = append(reqs, DecisionRequirement{Area: area, Required: required, Current: current, Status: status, Why: why, NextStep: next})
	}
	add("historical_backtest", "100+ toplam işlem hedefi; canlı sinyal için minimum 30 işlem ve 10 OOS", dailyBacktestCurrent(daily, hasDaily), hasDaily && daily.Professional.Backtest.Trades >= 30 && daily.Professional.Backtest.OutOfSampleTrades >= 10, "Örnek sayısı düşükse edge kanıtı değil hipotezdir.", "Daha uzun OHLCV geçmişi bağla ve walk-forward testini yenile.")
	priceCurrent := "missing"
	if result.PriceQuality != nil {
		priceCurrent = fmt.Sprintf("status=%s official=%t conflict=%t stale=%t", result.PriceQuality.Status, result.PriceQuality.ReadyForVerifiedClose, result.PriceQuality.Conflict, result.PriceQuality.Stale)
	}
	add("verified_price_close", "resmi final kapanış, source_timestamp, fetched_at ve kaynak mutabakatı", priceCurrent, verifiedPriceCloseOK(result), "Kapanış fiyatı verified değilse teknik seviyeler ve AL/SAT çıktısı kesin fiyat kanıtı gibi kullanılamaz.", "Resmi/lisanslı kapanış dosyasını içe aktar ve price-quality kapısını yenile.")
	add("technical_trade_plan", "giriş, stop, hedef, risk/ödül ve geçersiz kılma şartı", dailyPlanCurrent(daily, hasDaily), hasDaily && !daily.TradePlan.Rejected && daily.TradePlan.EntryMin > 0 && daily.TradePlan.StopLoss > 0 && daily.TradePlan.TakeProfit1 > 0 && daily.TradePlan.RiskRewardRatio >= 1.5, "Seviye listesi tek başına işlem planı değildir.", "technical_trade_plan.json içindeki reddedilen plan nedenlerini kapat.")
	add("next_session_forecast_model", "sonraki seans model doğrulaması decision-grade", nextSessionForecastDecisionCurrent(result), nextSessionForecastDecisionOK(result), "Kısa vadeli fiyat/yön tahmini karar girdisi olacaksa model validasyonu, teknik karar kapısı ve güven eşiği birlikte geçmelidir.", "Aylık/rolling forecast audit sonucunu iyileştir veya bu modeli karar katmanından çıkar.")
	add("quant_risk", "VaR/CVaR, volatilite, drawdown, Sharpe/Sortino ve benchmark beta", quantDecisionCurrent(result), quantDecisionOK(result), "Quant katmanı teknik/fundamental görüşün portföy riskiyle uyumlu olup olmadığını sayısallaştırır.", "Daha uzun OHLCV geçmişi, benchmark verisi, resmi kapanış ve finansal/KAP kanıtlarını yenile.")
	add("statistical_economic_consistency", "veri bütünlüğü, faktör, rejim, stres, makro, finansal kalite, likidite ve validasyon >= limited", statEconomicCurrent(result), statEconomicOK(result), "Karar tutarlılığı tek modelden değil, birbirini doğrulayan istatistiksel/ekonomik katmanlardan gelmelidir.", "Eksik makro, benchmark, mikro yapı ve finansal veri uyarılarını kapat; walk-forward ve event-study örnek sayısını artır.")
	add("advanced_analysis_phase_coverage", "Faz 0-10 üretildi; production gate fail değil", advancedDecisionCurrent(result), advancedDecisionOK(result), "Hisse kararının doğru yapılması için tüm istatistiksel/ekonomik/quant/fundamental fazlar tek audit trail altında birleşmelidir.", "Eksik fazları production_readiness_report.json ve human_review_queue.json üzerinden kapat.")
	add("financial_statements", "normalize bilanço, güncel dönem publish-date/available-at ve reconciliation", result.Professional.DataGovernance.AvailabilityStatus, financialStatementDecisionSafe(result), "Hisse değerlemesi için finansal tablo ve güncel dönem zaman güvenliği gerekir.", "Güncel dönem publish-date/available-at eksikse financials metadata ve reconcile adımlarını çalıştır.")
	add("valuation", "içsel değer, güvenlik marjı ve sektör modeli", fmt.Sprintf("computed=%t confidence=%.1f", result.Professional.ValueInvesting.Computed, result.Professional.ValueInvesting.Confidence), result.Professional.ValueInvesting.Computed && result.Professional.ValueInvesting.Confidence >= 65 && !bankCoreMetricsMissingForDecision(result), "Kurumsal karar için teknik skorun finansal tezle desteklenmesi gerekir.", "valuation_assumptions ve sektör model eksiklerini tamamla.")
	if isBankDecisionResult(result) {
		missing := professional.MissingBankRegulatoryMetricNames(result.Professional.SectorFinancials)
		current := "complete"
		if len(missing) > 0 {
			current = "missing=" + strings.Join(missing, ",")
		}
		add("bank_core_metrics", "sertifikalı SYR, CET1, NPL, karşılık kapsamı, NIM, LCR, kredi/mevduat, mevduat maliyeti ve kredi/mevduat spread'i", current, len(missing) == 0 && professional.BankRegulatoryMetricsComplete(result.Professional.SectorFinancials), "Banka için hedef fiyat, güvenlik marjı ve AL/SAT kararı sanayi metrikleriyle üretilemez.", "KAP PDF/XBRL çıkarımından banka ana rasyolarını kaynak belge, sayfa, tablo ve sertifika bilgisiyle bağla.")
	}
	add("macro_market_context", "benchmark göreli güç, BIST canlı snapshot, TÜİK GSYH, TCMB PDF/HTML text index ve EVDS seri arşivi", macroContextCurrent(result), macroContextOK(result), "Piyasa rejimi, faiz/enflasyon kararları ve likidite hisse kararını doğrudan etkiler.", "sync market-ws, sync tuik-gdp, sync tuik-inflation, sync tcmb, sync tcmb-extract ve sync tcmb-evds adımlarını çalıştır.")
	add("market_microstructure", "live quote, order book, KDM2 derinlik, AKD ve takas", marketMicrostructureCurrent(result), marketMicrostructureDecisionOK(result), "Son emir defteri, derinlik, aracı kurum dağılımı ve saklama/takas görünümü fiyat tahmininin kısa vadeli uygulanabilirliğini belirler.", "sync market-ws -symbols {SYMBOL} ve sync market-quality adımlarını çalıştır.")
	add("news_sentiment", "KAP, haber veya yorum/sentiment kaynak kapsamı", sentimentCurrent(result), sentimentCoverageOK(result), "Haber/sentiment yoksa kısa vadeli karar eksik kalır.", "sync news ve sync kap-disclosures adımlarını çalıştır.")
	add("kap_pdf_evidence", "KAP PDF/ek arşivi, OCR kalite kapısı ve kaynak indexi", fmt.Sprintf("computed=%t usable=%d/%d", result.Professional.KAPPDFIngest.Computed, result.Professional.KAPPDFIngest.AnalysisUsableCount, result.Professional.KAPPDFIngest.TotalDocuments), result.Professional.KAPPDFIngest.Computed && result.Professional.KAPPDFIngest.AnalysisUsableCount > 0, "Ham veri, kaynak ve sayfa kanıtı olmadan rapor kurumsal görünse de denetlenebilir değildir.", "sync kap-attachments, kap-document-archive ve kap-extract adımlarını çalıştır.")
	add("peer_universe", "3+ güvenilir sektör peer", fmt.Sprintf("peer_count=%d classification=%.2f", result.Professional.Peers.PeerCount, result.Professional.Company.ClassificationConfidence), result.Professional.Peers.PeerCount >= 3 && result.Professional.Company.ClassificationConfidence >= 0.80, "Peer ve sektör sınıflaması değerleme çarpanlarını belirler.", "sync kap-sectors ve sync sectors adımlarını çalıştır.")
	return reqs
}

func quantDecisionOK(result SymbolAnalysis) bool {
	return result.Quant.Computed &&
		result.Quant.Decision.Score >= quantDecisionScoreLimit(result.AssetType) &&
		result.Quant.Decision.RiskScore >= quantDecisionRiskScoreLimit(result.AssetType) &&
		result.Quant.Risk.HistoricalVaR95Pct < quantDecisionVaRLimit(result.AssetType)
}

func quantDecisionGateStatus(result SymbolAnalysis) string {
	if !result.Quant.Computed {
		return "fail"
	}
	if result.Quant.Decision.RiskScore < quantDecisionFailRiskScoreLimit(result.AssetType) || result.Quant.Risk.HistoricalVaR95Pct >= quantDecisionFailVaRLimit(result.AssetType) {
		return "fail"
	}
	if !quantDecisionOK(result) {
		return "limited"
	}
	return "pass"
}

func quantDecisionRequirement(result SymbolAnalysis) string {
	return fmt.Sprintf("quant computed + score >= %.0f + risk_score >= %.0f + VaR95 < %.1f%%", quantDecisionScoreLimit(result.AssetType), quantDecisionRiskScoreLimit(result.AssetType), quantDecisionVaRLimit(result.AssetType))
}

func quantDecisionScoreLimit(assetType string) float64 {
	if ohlcv.IsCryptoAssetType(assetType) {
		return 55
	}
	return 58
}

func quantDecisionRiskScoreLimit(assetType string) float64 {
	if ohlcv.IsCryptoAssetType(assetType) {
		return 45
	}
	return 50
}

func quantDecisionVaRLimit(assetType string) float64 {
	if ohlcv.IsCryptoAssetType(assetType) {
		return 8
	}
	return 5
}

func quantDecisionFailRiskScoreLimit(assetType string) float64 {
	if ohlcv.IsCryptoAssetType(assetType) {
		return 32
	}
	return 40
}

func quantDecisionFailVaRLimit(assetType string) float64 {
	if ohlcv.IsCryptoAssetType(assetType) {
		return 11
	}
	return 7
}

func quantDecisionCurrent(result SymbolAnalysis) string {
	if !result.Quant.Computed {
		return "missing"
	}
	q := result.Quant
	return fmt.Sprintf(
		"score=%.1f risk=%.1f label=%s var95=%.2f%% cvar95=%.2f%% vol=%.2f%% mdd=%.2f%% beta=%.2f",
		q.Decision.Score,
		q.Decision.RiskScore,
		emptyDecisionText(q.Decision.Label, "n/a"),
		q.Risk.HistoricalVaR95Pct,
		q.Risk.HistoricalCVaR95Pct,
		q.Risk.AnnualizedVolatilityPct,
		q.Risk.MaxDrawdownLossPct,
		q.Benchmark.Beta60,
	)
}

func quantDecisionEvidence(result SymbolAnalysis) []string {
	if !result.Quant.Computed {
		return result.Quant.Warnings
	}
	evidence := []string{result.Quant.Decision.Summary}
	evidence = append(evidence, result.Quant.Decision.Passes...)
	evidence = append(evidence, result.Quant.Decision.Warnings...)
	evidence = append(evidence, result.Quant.Decision.Blockers...)
	return evidence
}

func quantDecisionNextSteps(result SymbolAnalysis) []string {
	if !result.Quant.Computed {
		return []string{"Günlük OHLCV geçmişini en az 60-252 bar olacak şekilde yenile."}
	}
	steps := []string{}
	if result.Quant.Decision.Score < quantDecisionScoreLimit(result.AssetType) {
		steps = append(steps, "Quant toplam skoru düşük; momentum, benchmark göreli güç, veri kalitesi ve risk bütçesi birlikte yeniden değerlendirilmeli.")
	}
	if result.Quant.Decision.RiskScore < quantDecisionRiskScoreLimit(result.AssetType) {
		steps = append(steps, "Pozisyon boyutunu VaR/CVaR limitine göre küçült veya trade planı riskini düşür.")
	}
	if result.Quant.Risk.HistoricalVaR95Pct >= quantDecisionVaRLimit(result.AssetType) {
		steps = append(steps, "1 günlük VaR yüksek; stop mesafesi ve sermaye riski birlikte tekrar hesaplanmalı.")
	}
	if !result.Quant.Benchmark.Available {
		steps = append(steps, "Benchmark OHLCV verisini yenileyerek beta/alpha ve göreli güç katmanını tamamla.")
	}
	if len(steps) == 0 {
		steps = append(steps, "Quant risk metriklerini periyodik güncelle ve portföy limitiyle izle.")
	}
	return steps
}

func statEconomicOK(result SymbolAnalysis) bool {
	return result.StatEconomic.Computed &&
		result.StatEconomic.CompositeScore >= 58 &&
		result.StatEconomic.DataIntegrity.Score >= 55 &&
		result.StatEconomic.Validation.ModelRisk != "high"
}

func statEconomicGateStatus(result SymbolAnalysis) string {
	if !result.StatEconomic.Computed {
		return "fail"
	}
	if result.StatEconomic.DataIntegrity.Score < 45 || result.StatEconomic.Validation.ModelRisk == "high" || result.StatEconomic.CompositeScore < 45 {
		return "fail"
	}
	if !statEconomicOK(result) {
		return "limited"
	}
	return "pass"
}

func statEconomicCurrent(result SymbolAnalysis) string {
	if !result.StatEconomic.Computed {
		return "missing"
	}
	s := result.StatEconomic
	return fmt.Sprintf(
		"composite=%.1f data=%.1f factor=%.1f regime=%.1f stress=%.1f macro=%.1f financial=%.1f liquidity=%.1f validation=%.1f model_risk=%s",
		s.CompositeScore,
		s.DataIntegrity.Score,
		s.FactorModel.Score,
		s.Regime.Score,
		s.TailStress.Score,
		s.MacroSensitivity.Score,
		s.FinancialQuality.Score,
		s.Liquidity.Score,
		s.Validation.Score,
		emptyDecisionText(s.Validation.ModelRisk, "unknown"),
	)
}

func statEconomicEvidence(result SymbolAnalysis) []string {
	if !result.StatEconomic.Computed {
		return result.StatEconomic.Warnings
	}
	s := result.StatEconomic
	evidence := []string{
		fmt.Sprintf("regime=%s/%s drawdown=%s", s.Regime.TrendRegime, s.Regime.VolatilityRegime, s.Regime.DrawdownRegime),
		fmt.Sprintf("tail var95=%.2f%% cvar95=%.2f%% stress=%.2f%%", s.TailStress.VaR95Pct, s.TailStress.CVaR95Pct, s.TailStress.StressWorstCaseReturnPct),
		fmt.Sprintf("macro profile=%s rate=%s fx=%s inflation=%s", s.MacroSensitivity.SectorMacroProfile, s.MacroSensitivity.RateSensitivity, s.MacroSensitivity.FXSensitivity, s.MacroSensitivity.InflationSensitivity),
	}
	evidence = append(evidence, compactStrings(s.Warnings, 6)...)
	return evidence
}

func statEconomicNextSteps(result SymbolAnalysis) []string {
	if !result.StatEconomic.Computed {
		return []string{"Günlük OHLCV, benchmark, finansal tablo ve makro veri setlerini yenile."}
	}
	steps := []string{}
	s := result.StatEconomic
	if s.DataIntegrity.Score < 70 {
		steps = append(steps, "OHLCV veri bütünlüğünü, bölünme/temettü düzeltmelerini ve outlier mumları denetle.")
	}
	if s.MacroSensitivity.Score < 60 {
		steps = append(steps, "TCMB EVDS, TÜİK ve piyasa makro serilerini point-in-time güvenli şekilde tamamla.")
	}
	if s.FinancialQuality.Score < 60 {
		steps = append(steps, "Finansal kalite için bilanço oran geçmişi, nakit akımı ve restatement kontrollerini tamamla.")
	}
	if s.Validation.Score < 60 {
		steps = append(steps, "Walk-forward, event-study ve forecast audit örnek sayısını artır.")
	}
	if s.Liquidity.Score < 60 {
		steps = append(steps, "Likidite, spread ve market impact verilerini güncelle; pozisyon kapasitesini küçült.")
	}
	if len(steps) == 0 {
		steps = append(steps, "Stat/economic skorlarını her veri güncellemesinde yeniden hesapla.")
	}
	return steps
}

func advancedDecisionOK(result SymbolAnalysis) bool {
	return result.Advanced.Computed &&
		result.Advanced.CompositeScore >= 58 &&
		result.Advanced.Production.Status != "fail"
}

func advancedDecisionGateStatus(result SymbolAnalysis) string {
	if !result.Advanced.Computed {
		return "fail"
	}
	if result.Advanced.Production.Status == "fail" || result.Advanced.CompositeScore < 45 {
		return "fail"
	}
	if !advancedDecisionOK(result) {
		return "limited"
	}
	return "pass"
}

func advancedDecisionCurrent(result SymbolAnalysis) string {
	if !result.Advanced.Computed {
		return "missing"
	}
	a := result.Advanced
	return fmt.Sprintf(
		"composite=%.1f production=%s impact=%s data=%.1f factor=%.1f vol=%.1f macro=%.1f financial=%.1f valuation=%.1f event=%.1f monitoring=%.1f liquidity=%.1f",
		a.CompositeScore,
		emptyDecisionText(a.Production.Status, "unknown"),
		emptyDecisionText(a.DecisionImpact, "unknown"),
		a.DataQuality.Score,
		a.FactorModel.Score,
		a.Volatility.Score,
		a.Macro.Score,
		a.FinancialQuality.Score,
		a.Valuation.Score,
		a.EventStudy.Score,
		a.ModelMonitoring.Score,
		a.LiquidityPortfolio.Score,
	)
}

func advancedDecisionEvidence(result SymbolAnalysis) []string {
	if !result.Advanced.Computed {
		return result.Advanced.Warnings
	}
	a := result.Advanced
	evidence := []string{
		fmt.Sprintf("production gates data=%s validation=%s risk=%s valuation=%s execution=%s", a.Production.DataGate, a.Production.ValidationGate, a.Production.RiskGate, a.Production.ValuationGate, a.Production.ExecutionGate),
		fmt.Sprintf("report_hash=%s", a.Production.ReportHash),
	}
	for _, phase := range a.Phases {
		if phase.Blocking || phase.Status == "limited" || phase.Status == "missing" {
			evidence = append(evidence, fmt.Sprintf("%s=%s %.1f", phase.Phase, phase.Status, phase.Score))
		}
	}
	evidence = append(evidence, compactStrings(a.Warnings, 8)...)
	return evidence
}

func advancedDecisionNextSteps(result SymbolAnalysis) []string {
	if !result.Advanced.Computed {
		return []string{"Günlük OHLCV, finansal veri, benchmark ve makro veri setleriyle advanced_analysis üretimini tamamla."}
	}
	steps := []string{}
	for _, phase := range result.Advanced.Phases {
		if phase.Status == "pass" {
			continue
		}
		if len(phase.Missing) > 0 {
			steps = append(steps, fmt.Sprintf("%s eksikleri: %s", phase.Phase, strings.Join(phase.Missing, ",")))
		} else {
			steps = append(steps, phase.Phase+" limited/fail kapısını kapat")
		}
	}
	if len(steps) == 0 {
		steps = append(steps, "Advanced faz raporlarını periyodik yeniden üret ve production_readiness_report.json hash'ini izle.")
	}
	return compactStrings(steps, 8)
}

func financialStatementDecisionSafe(result SymbolAnalysis) bool {
	if len(rawKAPFinancialIntegrityIssues(result)) > 0 {
		return false
	}
	gov := result.Professional.DataGovernance
	if !gov.FinanciallyConsistent || gov.LatestPeriod == "" {
		return false
	}
	if len(gov.InvalidChronologyPeriods) > 0 || containsDecisionWarning(gov.Warnings, "financial_period_chronology_invalid") {
		return false
	}
	if gov.BacktestSafe {
		return true
	}
	asOf := gov.AsOf
	hasLatestTiming := false
	if gov.LatestAvailableAt != nil {
		hasLatestTiming = true
		if !asOf.IsZero() && gov.LatestAvailableAt.After(asOf) {
			return false
		}
	}
	if gov.LatestPublishDate != nil {
		hasLatestTiming = true
		if !asOf.IsZero() && gov.LatestPublishDate.After(asOf) {
			return false
		}
	}
	return hasLatestTiming
}

func verifiedPriceCloseOK(result SymbolAnalysis) bool {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return true
	}
	return result.PriceQuality != nil && result.PriceQuality.ReadyForVerifiedClose
}

func decisionPriceOK(result SymbolAnalysis) bool {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return true
	}
	return result.PriceQuality != nil && (result.PriceQuality.ReadyForDecision || result.PriceQuality.ReadyForVerifiedClose)
}

func decisionPriceGate(result SymbolAnalysis) (string, string, string, []string, []string) {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return "pass", "not_applicable_for_asset_type", "BIST karar fiyatı kapısı bu varlık tipine uygulanmaz.", nil, nil
	}
	if result.PriceQuality == nil {
		return "fail", "missing", "Güncel karar fiyatı üretilemedi.", nil, []string{"Günlük fiyat verisini yenile."}
	}
	current := fmt.Sprintf("status=%s latest=%s decision_ready=%t conflict=%t stale=%t", result.PriceQuality.Status, result.PriceQuality.LatestTradingDate, result.PriceQuality.ReadyForDecision, result.PriceQuality.Conflict, result.PriceQuality.Stale)
	evidence := append([]string{}, result.PriceQuality.BlockingReasons...)
	for _, candidate := range result.PriceQuality.Candidates {
		evidence = append(evidence, fmt.Sprintf("%s:%s %.4f %s", candidate.Source, candidate.SourceType, candidate.Close, candidate.TradingDate))
	}
	if result.PriceQuality.ReadyForDecision {
		return "pass", current, "Güncel analiz fiyatı taze ve aynı işlem günündeki kaynaklarla mutabık.", evidence, nil
	}
	return "fail", current, "Güncel karar fiyatı stale veya aynı işlem günündeki kaynaklarla çelişkili.", evidence, []string{"Günlük OHLCV ve piyasa snapshot verisini yenile."}
}

func priceCloseDecisionGate(result SymbolAnalysis) (string, string, string, []string, []string) {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return "pass", "not_applicable_for_asset_type", "Kripto/emtia için BIST resmi kapanış kapısı uygulanmaz.", nil, nil
	}
	if result.PriceQuality == nil {
		return "fail", "missing", "Fiyat kalite raporu yok; kapanış verified sayılmaz.", nil, []string{
			"go run ./cmd/hissebot sync price-quality",
			"Resmi kapanış kaynağı için `sync official-close-import` çalıştır.",
		}
	}
	current := fmt.Sprintf("status=%s latest=%s ready=%t conflict=%t stale=%t", result.PriceQuality.Status, result.PriceQuality.LatestTradingDate, result.PriceQuality.ReadyForVerifiedClose, result.PriceQuality.Conflict, result.PriceQuality.Stale)
	evidence := append([]string{}, result.PriceQuality.BlockingReasons...)
	for _, candidate := range result.PriceQuality.Candidates {
		evidence = append(evidence, fmt.Sprintf("%s:%s %.4f %s", candidate.Source, candidate.SourceType, candidate.Close, candidate.TradingDate))
	}
	next := []string{
		"go run ./cmd/hissebot sync official-close-import -file data/import/official_closes.csv -source bist_official",
		"go run ./cmd/hissebot sync price-quality",
	}
	if result.PriceQuality.ReadyForVerifiedClose {
		return "pass", current, "Resmi final kapanış taze ve kaynaklarla mutabık.", evidence, nil
	}
	if result.PriceQuality.Status == pricequality.StatusProvisionalLastPrice {
		return "limited", current, "Yatırımcı kararı üretilebilir; otomatik emir için ayrıca resmi final kapanış gerekir.", evidence, next
	}
	return "fail", current, "Kapanış fiyatı verified değil; eksik, stale veya kaynaklar arası çelişki var.", evidence, next
}

func decisionValueCurrent(result SymbolAnalysis, bankCoreOK bool) string {
	current := fmt.Sprintf("computed=%t confidence=%.1f", result.Professional.ValueInvesting.Computed, result.Professional.ValueInvesting.Confidence)
	if !bankCoreOK {
		current += " bank_core_metrics=missing"
	}
	return current
}

func bankCoreMetricsMissingForDecision(result SymbolAnalysis) bool {
	pro := result.Professional
	if containsDecisionWarning(pro.Valuation.Flags, "bank_sector_requires_regulatory_capital_and_asset_quality_model") {
		return true
	}
	if containsDecisionWarning(pro.Valuation.Flags, "bank_book_value_reconciliation_missing") || containsDecisionWarning(pro.Valuation.Flags, "bank_book_value_reconciliation_failed") {
		return true
	}
	if !isBankDecisionResult(result) {
		return false
	}
	return !professional.BankRegulatoryMetricsComplete(pro.SectorFinancials)
}

func isBankDecisionResult(result SymbolAnalysis) bool {
	pro := result.Professional
	if strings.EqualFold(strings.TrimSpace(pro.SectorFinancials.Profile), "bank") ||
		strings.EqualFold(strings.TrimSpace(pro.Valuation.SectorModel), "bank_equity_model") ||
		strings.EqualFold(strings.TrimSpace(pro.ValueInvesting.SectorModel.Model), "bank_equity_model") {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		pro.Company.Industry,
		pro.SectorFinancials.ProfileLabel,
		pro.ValueInvesting.SectorModel.Label,
	}, " "))
	return strings.Contains(text, "bank") || strings.Contains(text, "banka")
}

func containsDecisionWarning(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func decisionMissingInputs(result SymbolAnalysis) []DecisionMissingInput {
	items := []DecisionMissingInput{}
	seen := map[string]bool{}
	add := func(key, category, priority, why, how string) {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		items = append(items, DecisionMissingInput{Key: key, Category: category, Priority: priority, WhyItMatters: why, HowToComplete: how})
	}
	for _, key := range result.Professional.Coverage.Missing {
		if key == "order_book_depth" || key == "kdm2_depth" {
			// These inputs belong to automatic execution sizing. Their absence
			// must not turn an investor decision report into a missing-data report.
			continue
		}
		category, priority, why, how := missingInputMeta(key, result.Symbol)
		add(key, category, priority, why, how)
	}
	if !decisionPriceOK(result) {
		add("decision_price_reconciliation", "price_quality", "critical", "Güncel ve mutabık karar fiyatı olmadan AL/SAT seviyesi hesaplanamaz.", "Günlük OHLCV ve piyasa snapshot verisini yenileyip aynı işlem gününü mutabık hale getir.")
		if result.PriceQuality != nil && (result.PriceQuality.Conflict || result.PriceQuality.Stale) {
			add("price_source_reconciliation", "price_quality", "critical", "Aynı işlem günündeki OHLCV ve piyasa kaynakları çelişirse analiz yanlış son fiyat üzerinden üretilebilir.", "Güncel chart/OHLCV cache'ini ve piyasa snapshot'ını yenile.")
		}
	}
	for _, key := range result.Professional.EvidencePolicy.BlockingIssues {
		if !evidenceBlockingIssueIsMissingInput(key) {
			continue
		}
		category, priority, why, how := blockingIssueMeta(key, result.Symbol)
		add(key, category, priority, why, how)
	}
	if !nextSessionForecastDecisionOK(result) {
		add("next_session_forecast_model_validation", "forecast_model", "critical", "Sonraki seans modeli doğrulama/teknik karar kapısını geçmiyorsa AL/SAT ve emir kararına giremez.", "forecast-audit aralığını çalıştır; yön/MAE/decision-grade eşikleri geçmeden modeli karar kapısından çıkar.")
	}
	if isBankDecisionResult(result) {
		for _, name := range professional.MissingBankRegulatoryMetricNames(result.Professional.SectorFinancials) {
			add("bank_metric_"+name, "bank_core_metrics", "critical", "Banka değerlemesi için SYR/CET1/NPL/NIM/LCR/kredi-mevduat, mevduat maliyeti ve kredi/mevduat spread'i sertifikalı kaynakla gelmelidir.", "KAP PDF/XBRL çıkarımında metrik için source_document_id, sayfa, tablo/satır, orijinal değer, normalize değer ve kap_certified kanıtı üret.")
		}
	}
	return items
}

func evidenceBlockingIssueIsMissingInput(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "missing") ||
		strings.Contains(key, "kap_pdf_ingest") ||
		strings.Contains(key, "financial_statements") ||
		strings.Contains(key, "intrinsic_value")
}

func decisionCompletionActions(result SymbolAnalysis) []DecisionCompletionStep {
	symbol := strings.ToUpper(strings.TrimSpace(result.Symbol))
	if symbol == "" {
		symbol = "{SYMBOL}"
	}
	actions := []DecisionCompletionStep{}
	if !decisionPriceOK(result) {
		actions = append(actions, DecisionCompletionStep{Priority: 1, Area: "price_quality", Action: "Güncel OHLCV ve piyasa fiyatını aynı işlem günü için mutabık hale getir.", Command: "go run ./cmd/hissebot sync charts -ticker " + symbol + " -intervals D -bars 0 && go run ./cmd/hissebot sync price-quality", AcceptanceCriteria: []string{"price_quality.ready_for_decision=true olur."}})
	}
	if containsDecisionWarning(result.Professional.Coverage.Missing, "financial_statements") || !result.Professional.DataGovernance.FinanciallyConsistent {
		actions = append(actions, DecisionCompletionStep{Priority: 2, Area: "financial_statements", Action: "Finansal tablo ve mutabakat zincirini yenile.", Command: fmt.Sprintf("go run ./cmd/hissebot financials run -ticker %s -force-history && go run ./cmd/hissebot financials reconcile", symbol), AcceptanceCriteria: []string{"Finansal tablolar dolu ve reconciliation_failure_count=0 olur."}})
	}
	if !result.Professional.KAPPDFIngest.Computed || result.Professional.KAPPDFIngest.AnalysisUsableCount == 0 {
		actions = append(actions, DecisionCompletionStep{Priority: 3, Area: "kap_evidence", Action: "Mevcut KAP eklerini çıkarım ve kaynak indeksine bağla.", Command: fmt.Sprintf("go run ./cmd/kap-ingest --input data/equities/%s --output data/processed/%s --workers 4 --llm=false --vision=false --ocr=true", symbol, strings.ToLower(symbol)), AcceptanceCriteria: []string{"KAP PDF kalite kapısı analize uygun belge üretir."}})
	}
	if result.Professional.Company.ClassificationConfidence < 0.80 || result.Professional.Peers.PeerCount < 3 {
		actions = append(actions, DecisionCompletionStep{Priority: 4, Area: "sector_peer_universe", Action: "Sektör sınıflaması ve peer evrenini mevcut KAP verisinden yenile.", Command: "go run ./cmd/hissebot sync sectors", AcceptanceCriteria: []string{"Sınıflama güveni >=0.80 ve peer_count >=3 olur."}})
	}
	if !nextSessionForecastDecisionOK(result) {
		actions = append(actions, DecisionCompletionStep{Priority: 5, Area: "forecast_model", Action: "Sonraki seans forecast modelini decision-grade eşiğe getir veya karar katmanından çıkar.", Command: fmt.Sprintf("go run ./cmd/hissebot forecast-audit -symbol %s -from 2026-05-22 -to 2026-06-22 -data ./data -out ./data/equities -limit 0", symbol), AcceptanceCriteria: []string{"next_session_forecast_model kapısı pass olur veya AL/SAT/otomatik emir sınıfları kapalı kalır."}})
	}
	if len(actions) > 0 {
		actions = append(actions, DecisionCompletionStep{Priority: 9, Area: "analysis_refresh", Action: "Güncellenen girdilerle karar raporunu yeniden üret.", Command: fmt.Sprintf("go run ./cmd/hissebot analyze --symbol %s --provider tradingview --mode decision", symbol), AcceptanceCriteria: []string{"Büyük yatırımcı kararı ve küçük yatırımcı AL/SAT kararı yeniden üretilir."}})
	}
	return actions
}

func decisionProductScope() []DecisionProductScopeRow {
	return []DecisionProductScopeRow{
		{Product: "Büyük yatırımcı karar raporu", Status: "in_scope", Notes: "ONAYLA/BEKLE/REDDET kararı; pozisyon, likidite, değerleme, makro ve risk gerekçeleriyle üretilir."},
		{Product: "Küçük yatırımcı AL/SAT raporu", Status: "in_scope", Notes: "Yeni ve mevcut pozisyon için AL/ALMA/BEKLE/TUT/AZALT/SAT aksiyonları ayrı verilir."},
		{Product: "VİOP / kaldıraçlı hisse pozisyonu", Status: "limited", Notes: "Ayrı teminat, kaldıraç, likidasyon ve kontrat vade riski eklenmeden production işlem için kullanılmaz."},
		{Product: "ETF/fon veya sepet", Status: "out_of_scope", Notes: "Fon portföy kompozisyonu ve yönetim ücreti katmanı ayrı bağlanmalıdır."},
		{Product: "Otomatik emir", Status: "separate_gate", Notes: "Yatırımcı kararından ayrıdır; resmi kapanış, anlık spread, defter ve execution kontrolleri ayrıca geçmelidir."},
	}
}

func hasSupportOrResistance(tf TimeframeAnalysis) bool {
	return tf.NearestSupport != nil || tf.NearestResistance != nil || len(tf.SupportLevels) > 0 || len(tf.ResistanceLevels) > 0
}

func hasTradePlan(tf TimeframeAnalysis) bool {
	return tf.TradePlan.EntryMin > 0 || tf.TradePlan.StopLoss > 0 || tf.TradePlan.TakeProfit1 > 0 || tf.TradePlan.RejectReason != "" || len(tf.TradePlan.Reasoning) > 0
}

func dailyBacktestCurrent(tf TimeframeAnalysis, ok bool) string {
	if !ok {
		return "1D missing"
	}
	bt := tf.Professional.Backtest
	return fmt.Sprintf("trades=%d oos=%d expectancy=%.2f%% oos_return=%.2f%%", bt.Trades, bt.OutOfSampleTrades, bt.Expectancy*100, bt.OutOfSampleReturn*100)
}

func dailyPlanCurrent(tf TimeframeAnalysis, ok bool) string {
	if !ok {
		return "1D missing"
	}
	plan := tf.TradePlan
	if plan.Rejected {
		return "rejected: " + plan.RejectReason
	}
	return fmt.Sprintf("%s entry=%.2f-%.2f stop=%.2f target=%.2f rr=%.2f", plan.Direction, plan.EntryMin, plan.EntryMax, plan.StopLoss, plan.TakeProfit1, plan.RiskRewardRatio)
}

func sentimentCurrent(result SymbolAnalysis) string {
	coverage := result.Behavioral.SourceCoverage
	return fmt.Sprintf("kap=%d news=%d comments=%d analyzed=%d", coverage.KAPDisclosureCount, coverage.NewsItemCount, coverage.CommentCount, coverage.AnalyzedTextCount)
}

func macroContextCurrent(result SymbolAnalysis) string {
	evdsMatched, evdsExtra := macroEVDSDisplayFileCounts(result.Professional.TCMBEVDSContext)
	return fmt.Sprintf(
		"coverage=%.1f benchmark=%t live=%t gdp=%t tcmb_docs=%t tcmb_text=%d/%d evds_ready=%t evds_full_archive=%t evds_files=%d/%d evds_extra=%d",
		result.Professional.Coverage.Score,
		result.Professional.Market.BenchmarkAvailable,
		result.Professional.Market.LiveSnapshot != nil,
		result.Professional.Market.GDP.Computed,
		result.Professional.TCMBContext.Computed,
		result.Professional.TCMBContext.TextUsableCount,
		result.Professional.TCMBContext.TextDocumentCount,
		result.Professional.TCMBEVDSContext.AnalysisReady,
		result.Professional.TCMBEVDSContext.Computed,
		evdsMatched,
		result.Professional.TCMBEVDSContext.CatalogSeriesCount,
		evdsExtra,
	)
}

func macroEVDSDisplayFileCounts(ctx professional.TCMBEVDSContextReport) (matched int, extra int) {
	matched = ctx.CatalogMatchedSeriesFileCount
	if matched == 0 {
		matched = ctx.SeriesFileCount
		if ctx.CatalogSeriesCount > 0 && matched > ctx.CatalogSeriesCount {
			matched = ctx.CatalogSeriesCount
		}
	}
	extra = ctx.ExtraSeriesFileCount
	if extra == 0 && ctx.CatalogMatchedSeriesFileCount == 0 && ctx.CatalogSeriesCount > 0 && ctx.SeriesFileCount > ctx.CatalogSeriesCount {
		extra = ctx.SeriesFileCount - ctx.CatalogSeriesCount
	}
	return matched, extra
}

func macroContextOK(result SymbolAnalysis) bool {
	return result.Professional.Market.BenchmarkAvailable &&
		result.Professional.Market.LiveSnapshot != nil &&
		result.Professional.Market.GDP.Computed &&
		result.Professional.TCMBContext.Computed &&
		result.Professional.TCMBEVDSContext.AnalysisReady
}

func macroDecisionOK(result SymbolAnalysis) bool {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return true
	}
	if !macroContextOK(result) {
		return false
	}
	impact := result.Professional.TCMBEVDSContext.ForecastImpact
	if !impact.Computed || impact.DecisionUse == "not_usable" || impact.DecisionUse == "audit_only" {
		return false
	}
	if impact.Confidence > 0 && impact.Confidence < 65 {
		return false
	}
	if macroContextHasQualityWarnings(result.Professional.TCMBEVDSContext.Warnings) {
		return false
	}
	if impact.DecisionUse == "blocking_headwind" {
		return false
	}
	return !(impact.Direction == "negative" && impact.Confidence >= 65 && math.Abs(impact.PressureScore) >= 55)
}

func macroContextHasQualityWarnings(warnings []string) bool {
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if strings.HasPrefix(warning, "tcmb_evds_stale_") ||
			warning == "tcmb_evds_series_extra_files" ||
			warning == "tcmb_evds_catalog_series_missing_files" ||
			warning == "tcmb_evds_series_partial" {
			return true
		}
	}
	return false
}

func macroDecisionGateStatus(result SymbolAnalysis) string {
	if macroDecisionOK(result) {
		return "pass"
	}
	if macroContextOK(result) && result.Professional.TCMBEVDSContext.ForecastImpact.Computed {
		return "limited"
	}
	return "fail"
}

func macroDecisionCurrent(result SymbolAnalysis) string {
	impact := result.Professional.TCMBEVDSContext.ForecastImpact
	if !impact.Computed {
		return macroContextCurrent(result) + " impact=not_usable"
	}
	return fmt.Sprintf(
		"%s impact=%s severity=%s confidence=%.0f/100 pressure=%.0f adjustment=%+.2f use=%s",
		macroContextCurrent(result),
		impact.Direction,
		impact.Severity,
		impact.Confidence,
		impact.PressureScore,
		impact.ScoreAdjustment,
		impact.DecisionUse,
	)
}

func macroDecisionEvidence(result SymbolAnalysis) []string {
	impact := result.Professional.TCMBEVDSContext.ForecastImpact
	evidence := []string{}
	if impact.Summary != "" {
		evidence = append(evidence, impact.Summary)
	}
	evidence = append(evidence, impact.Drivers...)
	evidence = append(evidence, impact.Blockers...)
	for _, warning := range result.Professional.TCMBEVDSContext.Warnings {
		evidence = append(evidence, "uyarı: "+macroWarningLabel(warning))
	}
	return evidence
}

func macroWarningLabel(warning string) string {
	switch strings.TrimSpace(warning) {
	case "tcmb_evds_series_extra_files":
		return "EVDS klasöründe katalog dışı seri dosyası var"
	case "tcmb_evds_catalog_series_missing_files":
		return "EVDS katalog serilerinden eksik dosya var"
	case "tcmb_evds_series_partial":
		return "EVDS seri arşivi kısmi"
	default:
		if strings.HasPrefix(warning, "tcmb_evds_stale_") {
			return "EVDS serisi güncelliğini yitirmiş: " + strings.TrimPrefix(warning, "tcmb_evds_stale_")
		}
		return warning
	}
}

func macroDecisionNextSteps(result SymbolAnalysis) []string {
	if macroDecisionOK(result) {
		return nil
	}
	impact := result.Professional.TCMBEVDSContext.ForecastImpact
	if impact.DecisionUse == "blocking_headwind" {
		return []string{
			"Yeni AL sinyali için makro ters rüzgar hafifleyene veya fiyat/hacim tarafında daha güçlü teyit oluşana kadar kapıyı kapalı tut.",
			"Makro katkıların hangi seriden geldiğini TCMB EVDS indicator/drivers alanından denetle.",
		}
	}
	return []string{
		"sync macro-data ile TCMB EVDS, TCMB PDF/HTML text index, TÜİK ve piyasa snapshot verilerini tamamla.",
		"Makro sinyal point-in-time güvenli değilse backtest/karar skoruna uygulama.",
	}
}

func marketMicrostructureDecisionOK(result SymbolAnalysis) bool {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return false
	}
	return micro.Liquidity.DecisionUsable && micro.Liquidity.MicrostructureComplete
}

func marketMicrostructureOrderOK(result SymbolAnalysis) bool {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return false
	}
	return marketMicrostructureDecisionOK(result) && micro.Liquidity.AutomaticOrderReady
}

func marketMicrostructureGateStatus(result SymbolAnalysis) string {
	if marketMicrostructureDecisionOK(result) {
		return "pass"
	}
	if micro := result.Professional.Market.Microstructure; micro != nil && micro.Computed {
		return "limited"
	}
	return "fail"
}

func marketMicrostructureCurrent(result SymbolAnalysis) string {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return "missing"
	}
	return fmt.Sprintf(
		"status=%s score=%.0f/100 quote=%t order_book=%t kdm2=%t akd=%t takas=%t equilibrium=%t spread=%.1fbps auto=%t updated=%s",
		emptyDecisionText(micro.Status, "missing"),
		micro.Score,
		micro.Quote.Available,
		micro.OrderBook.Available,
		micro.Depth.Available,
		micro.BrokerageDistribution.Available,
		micro.Custody.Available,
		micro.Equilibrium.Available,
		micro.OrderBook.SpreadBps,
		micro.Liquidity.AutomaticOrderReady,
		marketMicrostructureUpdatedAt(micro),
	)
}

func marketMicrostructureEvidence(result SymbolAnalysis) []string {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return nil
	}
	evidence := []string{}
	if micro.Quote.Available {
		evidence = append(evidence, fmt.Sprintf("live quote last=%.2f bid=%.2f ask=%.2f volume=%.0f", micro.Quote.Last, micro.Quote.Bid, micro.Quote.Ask, micro.Quote.Volume))
	}
	if micro.OrderBook.Available {
		evidence = append(evidence, fmt.Sprintf("order book bid_levels=%d ask_levels=%d spread=%.1fbps top5_imbalance=%.2f", micro.OrderBook.BidLevels, micro.OrderBook.AskLevels, micro.OrderBook.SpreadBps, micro.OrderBook.ImbalanceTop5))
	}
	if micro.Depth.Available {
		evidence = append(evidence, fmt.Sprintf("KDM2 depth levels=%d net_buy_sell=%.1f", micro.Depth.Levels, micro.Depth.NetBuySellPct))
	}
	if micro.BrokerageDistribution.Available {
		evidence = append(evidence, fmt.Sprintf("AKD results=%d net_buy=%.0f net_sell=%.0f", micro.BrokerageDistribution.ResultCount, micro.BrokerageDistribution.NetBuyTotal, micro.BrokerageDistribution.NetSellTotal))
	}
	if micro.Custody.Available {
		evidence = append(evidence, fmt.Sprintf("takas results=%d date=%s top10_share=%.2f foreign=%.2f institutional=%.2f", micro.Custody.ResultCount, micro.Custody.Date, micro.Custody.Top10Share, micro.Custody.ForeignShare, micro.Custody.InstitutionalShare))
	}
	if micro.Equilibrium.Available {
		evidence = append(evidence, fmt.Sprintf("equilibrium price=%.2f matched=%.0f imbalance=%.2f", micro.Equilibrium.Price, micro.Equilibrium.MatchedLots, micro.Equilibrium.Imbalance))
	}
	for _, warning := range micro.Warnings {
		evidence = append(evidence, "uyarı: "+warning)
	}
	return evidence
}

func marketMicrostructureNextSteps(result SymbolAnalysis) []string {
	if marketMicrostructureDecisionOK(result) {
		return nil
	}
	symbol := strings.ToUpper(strings.TrimSpace(result.Symbol))
	if symbol == "" {
		symbol = "{SYMBOL}"
	}
	steps := []string{
		fmt.Sprintf("go run ./cmd/hissebot sync market-ws -symbols %s -duration 3m", symbol),
		"go run ./cmd/hissebot sync market-quality",
	}
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return steps
	}
	if !micro.Quote.Available {
		steps = append(steps, "market_ws/live_symbol_snapshot.json dosyasını üret.")
	}
	if !micro.OrderBook.Available {
		steps = append(steps, "market_ws/order_book.json dosyasını üret.")
	}
	if !micro.Depth.Available {
		steps = append(steps, "market_ws/kdm2_data.json dosyasını üret.")
	}
	if !micro.BrokerageDistribution.Available {
		steps = append(steps, "market_ws/akd_data.json dosyasını üret.")
	}
	if !micro.Custody.Available {
		steps = append(steps, "market_ws/custodian_data.json dosyasını üret.")
	}
	return steps
}

func automaticOrderLiquidityGateStatus(result SymbolAnalysis) string {
	if marketMicrostructureOrderOK(result) {
		return "pass"
	}
	if marketMicrostructureDecisionOK(result) {
		return "limited"
	}
	return "fail"
}

func automaticOrderLiquidityCurrent(result SymbolAnalysis) string {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return "missing"
	}
	return fmt.Sprintf(
		"top_of_book=%t spread=%.1fbps automatic_order_ready=%t blockers=%s",
		micro.Liquidity.TopOfBookAvailable,
		micro.Liquidity.SpreadBps,
		micro.Liquidity.AutomaticOrderReady,
		strings.Join(micro.Liquidity.AutomaticOrderBlockers, ","),
	)
}

func automaticOrderLiquidityEvidence(result SymbolAnalysis) []string {
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return nil
	}
	evidence := []string{
		fmt.Sprintf("microstructure_complete=%t decision_usable=%t", micro.Liquidity.MicrostructureComplete, micro.Liquidity.DecisionUsable),
	}
	if micro.OrderBook.Available {
		evidence = append(evidence, fmt.Sprintf("best_bid=%.2f best_ask=%.2f spread=%.1fbps", micro.OrderBook.BestBid, micro.OrderBook.BestAsk, micro.OrderBook.SpreadBps))
	}
	evidence = append(evidence, micro.Liquidity.AutomaticOrderBlockers...)
	return evidence
}

func automaticOrderLiquidityNextSteps(result SymbolAnalysis) []string {
	if marketMicrostructureOrderOK(result) {
		return nil
	}
	if !marketMicrostructureDecisionOK(result) {
		return marketMicrostructureNextSteps(result)
	}
	micro := result.Professional.Market.Microstructure
	if micro == nil {
		return nil
	}
	steps := []string{}
	for _, blocker := range micro.Liquidity.AutomaticOrderBlockers {
		switch {
		case strings.HasPrefix(blocker, "spread_too_wide_"):
			steps = append(steps, "Spread daralana kadar otomatik emir kapısını kapalı tut veya emir dilimleme/slippage limitini sıkılaştır.")
		default:
			steps = append(steps, blocker+" engelini kapat.")
		}
	}
	return steps
}

func marketMicrostructureUpdatedAt(micro *professional.MarketMicrostructureContext) string {
	if micro == nil || micro.UpdatedAt.IsZero() {
		return "unknown"
	}
	return micro.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
}

func sentimentCoverageOK(result SymbolAnalysis) bool {
	coverage := result.Behavioral.SourceCoverage
	return coverage.KAPDisclosureCount > 0 || coverage.NewsItemCount > 0 || coverage.AnalyzedTextCount > 0
}

func institutionalFailedDetails(result SymbolAnalysis) []string {
	out := []string{}
	for _, check := range result.InstitutionalValidation.Checks {
		if statusPass(check.Status) {
			continue
		}
		out = append(out, check.Name+": "+check.Message)
	}
	return out
}

func missingInputMeta(key, symbol string) (category, priority, why, how string) {
	switch key {
	case "financial_statements":
		return "financials", "critical", "Bilanço olmadan içsel değer, kalite ve sektör finansalı güvenilir hesaplanamaz.", fmt.Sprintf("go run ./cmd/hissebot financials run -ticker %s -force-history", symbol)
	case "sector_financial_interpretation":
		return "financials", "high", "Sektöre özel bilanço yorumu yoksa oranlar yanlış ağırlıklandırılabilir.", "Finansal tablo ve sektör sınıflamasını tamamla."
	case "valuation":
		return "valuation", "critical", "Değerleme olmadan kurumsal portföy kararı hedef/getiri/risk bağı kuramaz.", "Finansalları, peer evrenini ve valuation assumptions dosyasını tamamla."
	case "peer_comparison_min_3":
		return "peer_universe", "high", "Üçten az peer ile çarpan medyanı ve iskonto/premium yorumu zayıftır.", "go run ./cmd/hissebot sync sectors"
	case "benchmark_relative_strength":
		return "market_context", "high", "BIST benchmark'a göre göreli güç yoksa piyasa beta/alpha ayrımı eksik kalır.", "TradingView benchmark verisini yenile."
	case "sector_benchmark_relative_strength":
		return "market_context", "high", "Sektör endeksine göre göreli güç yoksa hisseye özel alfa ile sektör etkisi ayrışmaz.", "TradingView sektör benchmark chart verisini yenile."
	case "bist_live_websocket_snapshot":
		return "liquidity", "medium", "Canlı likidite ve piyasa durumu olmadan emir üretimi güvenli değildir.", "go run ./cmd/hissebot sync market-ws"
	case "bist_market_microstructure":
		return "liquidity", "high", "Fiyat tahmini ve AL/SAT kararı son quote, emir defteri, derinlik, AKD ve takas görünümü olmadan eksik kalır.", fmt.Sprintf("go run ./cmd/hissebot sync market-ws -symbols %s -duration 3m && go run ./cmd/hissebot sync market-quality", symbol)
	case "order_book_depth":
		return "liquidity", "high", "Top-of-book ve kademe defteri olmadan spread, slippage ve emir yürütme riski ölçülemez.", fmt.Sprintf("go run ./cmd/hissebot sync market-ws -symbols %s -duration 3m", symbol)
	case "kdm2_depth":
		return "liquidity", "high", "KDM2 derinlik olmadan alıcı/satıcı yoğunluğu ve kısa vadeli fiyat baskısı eksik kalır.", fmt.Sprintf("go run ./cmd/hissebot sync market-ws -symbols %s -duration 3m", symbol)
	case "brokerage_distribution_akd":
		return "liquidity", "high", "AKD olmadan aracı kurum bazlı net alım/satım davranışı fiyat tahminine dahil edilemez.", fmt.Sprintf("go run ./cmd/hissebot sync market-ws -symbols %s -duration 3m", symbol)
	case "custody_takas":
		return "liquidity", "high", "Takas/saklama dağılımı olmadan pozisyon sahipliği ve yoğunlaşma resmi eksik kalır.", fmt.Sprintf("go run ./cmd/hissebot sync market-ws -symbols %s -duration 3m", symbol)
	case "tuik_gdp_macro_context":
		return "macro", "medium", "Makro rejim ve büyüme etkisi özellikle orta vadeli hisse kararı için gerekir.", "go run ./cmd/hissebot sync tuik-gdp"
	case "tuik_inflation_indices":
		return "macro", "medium", "Tarihi KAP ekspertiz değerlerini bugünkü TL'ye taşımak için TÜİK Yİ-ÜFE endeksi gerekir.", "go run ./cmd/hissebot sync tuik-inflation"
	case "tcmb_macro_context":
		return "macro", "high", "PPK, faiz kararları, basın duyuruları ve enflasyon raporu text index olmadan faiz/enflasyon rejimi kanıtı eksik kalır.", "go run ./cmd/hissebot sync tcmb && go run ./cmd/hissebot sync tcmb-extract"
	case "tcmb_evds_series_context":
		return "macro", "high", "TCMB EVDS seri arşivi olmadan faiz, kur, rezerv, kredi ve parasal büyüklük rejimi tarihsel olarak doğrulanamaz.", "go run ./cmd/hissebot sync tcmb-evds -workers 1 -delay 250ms"
	case "recent_kap_news_disclosures":
		return "news_sentiment", "high", "Son KAP/haber akışı olmadan kısa vadeli karar eksik kalır.", "go run ./cmd/hissebot sync kap-disclosures -from 2009-06-29 -disclosure-types all"
	case "kap_pdf_ingest":
		return "kap_evidence", "high", "PDF kaynak kanıtı yoksa rapor denetlenebilirlik kaybeder.", fmt.Sprintf("go run ./cmd/hissebot sync kap-attachments -ticker %s && go run ./cmd/kap-ingest --input data/equities/%s --output data/processed/%s --workers 4 --llm=false --vision=false --ocr=true && go run ./cmd/kap-ingest --index-only --raw-documents data/processed/%s/raw_documents.jsonl --output data/processed", symbol, symbol, strings.ToLower(symbol), strings.ToLower(symbol))
	case "kap_asset_inventory":
		return "kap_evidence", "medium", "Varlık ağırlıklı şirketlerde portföy/NAD köprüsü için envanter gerekir.", fmt.Sprintf("go run ./cmd/hissebot sync kap-extract -ticker %s", symbol)
	default:
		return "data_quality", "medium", "Bu eksik kapı rapor güvenini düşürür.", "İlgili veri kaynağını bağla ve analizi yeniden üret."
	}
}

func blockingIssueMeta(key, symbol string) (category, priority, why, how string) {
	switch {
	case strings.Contains(key, "coverage_score"):
		return "data_coverage", "critical", "Kapsam skoru düşükse rapor karar desteği dışında kullanılmamalıdır.", "Coverage missing listesindeki veri kaynaklarını tamamla."
	case strings.Contains(key, "financial_statements") || strings.Contains(key, "intrinsic_value"):
		return "financials", "critical", "Finansal tablo/içsel değer yoksa hisse yatırım tezi eksiktir.", fmt.Sprintf("go run ./cmd/hissebot financials run -ticker %s -force-history", symbol)
	case strings.Contains(key, "kap_pdf"):
		return "kap_evidence", "high", "KAP PDF kanıtı eksik veya kalite eşiği altında.", fmt.Sprintf("go run ./cmd/hissebot sync kap-attachments -ticker %s && go run ./cmd/kap-ingest --input data/equities/%s --output data/processed/%s --workers 4 --llm=false --vision=false --ocr=true && go run ./cmd/kap-ingest --index-only --raw-documents data/processed/%s/raw_documents.jsonl --output data/processed", symbol, symbol, strings.ToLower(symbol), strings.ToLower(symbol))
	case strings.Contains(key, "classification") || strings.Contains(key, "peer"):
		return "peer_universe", "high", "Sektör/peer güveni düşükse değerleme çarpanları kurumsal eşik altında kalır.", "go run ./cmd/hissebot sync kap-sectors && go run ./cmd/hissebot sync sectors"
	case strings.Contains(key, "backtest") || strings.Contains(key, "available_at") || strings.Contains(key, "publish_date"):
		return "backtest_safety", "critical", "Zaman güvenliği olmadan backtest ve üretim kararı hatalı olabilir.", "go run ./cmd/hissebot financials metadata && go run ./cmd/hissebot financials reconcile"
	default:
		return "strict_evidence_policy", "high", "Sıkı kanıt politikası bu konuyu karar öncesi blocker sayıyor.", "Blocking issue kapatılana kadar hedef/öneri üretimini kullanma."
	}
}

func compactStrings(values []string, limit int) []string {
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen := false
		for _, existing := range out {
			if existing == value {
				seen = true
				break
			}
		}
		if seen {
			continue
		}
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			return out
		}
	}
	return out
}

func normalizeGateStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "pass", "passed", "ok", "ready":
		return "pass"
	case "limited", "warning", "watch":
		return "limited"
	default:
		return "fail"
	}
}

func statusPass(status string) bool {
	return normalizeGateStatus(status) == "pass"
}

func statusLimited(status string) bool {
	return normalizeGateStatus(status) == "limited"
}

func emptyDecisionText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
