package analysis

import (
	"fmt"
	"math"
	"strings"

	"hissebot/internal/ta/localize"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
)

type InstitutionalValidation struct {
	Status  string                         `json:"status"`
	Score   float64                        `json:"score"`
	Mode    string                         `json:"mode"`
	Summary string                         `json:"summary"`
	Checks  []InstitutionalValidationCheck `json:"checks"`
}

type InstitutionalValidationCheck struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Details  []string `json:"details,omitempty"`
}

func ValidateInstitutionalReadiness(result SymbolAnalysis) InstitutionalValidation {
	report := InstitutionalValidation{Status: "pass", Mode: "decision_validation"}
	report.add(validateDailyWalkForward(result))
	report.add(validateSignalSuccessStats(result))
	report.add(validateSentimentSourcePolicy(result))
	if ohlcv.IsCryptoAssetType(result.AssetType) {
		report.add(validateCryptoDataStack(result))
	} else if ohlcv.IsCommodityAssetType(result.AssetType) {
		report.add(validateCommodityDataStack(result))
	} else {
		report.add(validatePeerUniverse(result))
		report.add(validateValueInvesting(result))
	}
	report.add(validateInvestorQA(result))
	report.add(validateInstitutionalPersonaViews(result))
	report.add(validateExplainability(result))
	report.add(validateVisualTextPolicy(result))

	failures := 0
	limited := 0
	for _, check := range report.Checks {
		switch check.Status {
		case "fail":
			failures++
		case "limited":
			limited++
		}
	}
	if failures > 0 {
		report.Status = "fail"
	} else if limited > 0 {
		report.Status = "limited"
	}
	if len(report.Checks) > 0 {
		report.Score = math.Round(100 * float64(len(report.Checks)-failures) / float64(len(report.Checks)))
		if failures > 0 {
			report.Score = math.Min(report.Score, 59)
		} else if limited > 0 {
			report.Score = math.Min(report.Score, 84)
		}
	}
	report.Summary = institutionalSummary(report)
	return report
}

func (report *InstitutionalValidation) add(check InstitutionalValidationCheck) {
	if check.Status == "" {
		check.Status = "pass"
	}
	if check.Severity == "" {
		check.Severity = "critical"
	}
	report.Checks = append(report.Checks, check)
}

func validateDailyWalkForward(result SymbolAnalysis) InstitutionalValidationCheck {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		return failCheck("walk_forward_backtest", "critical", "Günlük ana sinyal yok; işlem kararı kurumsal validasyondan geçemez.")
	}
	bt := tf.Professional.Backtest
	details := []string{
		fmt.Sprintf("execution_model=%s", bt.ExecutionModel),
		fmt.Sprintf("selected_strategy=%s", bt.Strategy),
		fmt.Sprintf("lookback_bars=%d", bt.LookbackBars),
		fmt.Sprintf("trades=%d", bt.Trades),
		fmt.Sprintf("out_of_sample_trades=%d", bt.OutOfSampleTrades),
		fmt.Sprintf("win_rate=%.1f%%", bt.WinRate*100),
		fmt.Sprintf("expectancy=%.2f%%", bt.Expectancy*100),
		fmt.Sprintf("out_of_sample_average_return=%.2f%%", bt.OutOfSampleReturn*100),
		fmt.Sprintf("lookahead_violations=%d", bt.LookaheadViolations),
	}
	for _, candidate := range bt.CandidateStrategies {
		details = append(details, fmt.Sprintf("candidate=%s status=%s trades=%d oos=%d expectancy=%.2f%% oos_return=%.2f%%", candidate.Strategy, candidate.Status, candidate.Trades, candidate.OutOfSampleTrades, candidate.Expectancy*100, candidate.OutOfSampleReturn*100))
	}
	switch {
	case !bt.BacktestSafe || bt.LookaheadViolations > 0:
		return InstitutionalValidationCheck{Name: "walk_forward_backtest", Status: "fail", Severity: "critical", Message: "Backtest zaman güvenliği ihlali var.", Details: details}
	case bt.LookbackBars < 200:
		return InstitutionalValidationCheck{Name: "walk_forward_backtest", Status: "fail", Severity: "critical", Message: "Walk-forward için günlük geçmiş veri yetersiz.", Details: details}
	case bt.Trades < 30 || bt.OutOfSampleTrades < 10:
		return InstitutionalValidationCheck{Name: "walk_forward_backtest", Status: "limited", Severity: "critical", Message: "Backtest güvenli fakat minimum 30 işlem ve 10 ileri dönem işlem eşiği sağlanmadı.", Details: details}
	case bt.Expectancy <= 0 || bt.OutOfSampleReturn <= 0:
		return InstitutionalValidationCheck{Name: "walk_forward_backtest", Status: "fail", Severity: "critical", Message: "Günlük stratejinin beklenen getirisi veya out-of-sample performansı zayıf.", Details: details}
	default:
		return InstitutionalValidationCheck{Name: "walk_forward_backtest", Status: "pass", Severity: "critical", Message: "Günlük sinyal geçmiş veride zaman güvenli ve out-of-sample örnek içeriyor.", Details: details}
	}
}

func validateSignalSuccessStats(result SymbolAnalysis) InstitutionalValidationCheck {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		return failCheck("signal_success_rate", "critical", "Günlük rejim istatistiği yok.")
	}
	stats := tf.Professional.SignalStats
	details := []string{
		fmt.Sprintf("current_regime=%s", emptyText(stats.CurrentRegime, "unknown")),
		fmt.Sprintf("sample_size=%d", stats.SampleSize),
		fmt.Sprintf("forward_bars=%d", stats.ForwardBars),
		fmt.Sprintf("win_rate=%.1f%%", stats.WinRate*100),
		fmt.Sprintf("average_forward_return=%.2f%%", stats.AverageForwardReturn*100),
		fmt.Sprintf("probability_score=%.1f", stats.ProbabilityScore),
	}
	switch {
	case stats.InsufficientData || stats.SampleSize < 15:
		return InstitutionalValidationCheck{Name: "signal_success_rate", Status: "limited", Severity: "critical", Message: "Benzer rejim örneği karar için sınırlı; sinyal kesin alım sebebi sayılamaz.", Details: details}
	case stats.WinRate < 0.50 || stats.AverageForwardReturn <= 0:
		return InstitutionalValidationCheck{Name: "signal_success_rate", Status: "fail", Severity: "critical", Message: "Benzer geçmiş rejimde ileri getiri istatistiği zayıf.", Details: details}
	default:
		return InstitutionalValidationCheck{Name: "signal_success_rate", Status: "pass", Severity: "critical", Message: "Mevcut rejimin geçmiş ileri getiri istatistiği ölçüldü ve rapora bağlandı.", Details: details}
	}
}

func validateSentimentSourcePolicy(result SymbolAnalysis) InstitutionalValidationCheck {
	coverage := result.Behavioral.SourceCoverage
	gate := result.Behavioral.Contrarian.QualityGate
	details := []string{
		fmt.Sprintf("news_items=%d", coverage.NewsItemCount),
		fmt.Sprintf("comments=%d", coverage.CommentCount),
		fmt.Sprintf("recent_texts=%d", coverage.RecentTextCount),
		fmt.Sprintf("analyzed_texts=%d", coverage.AnalyzedTextCount),
		fmt.Sprintf("can_affect_buy_signal=%t", gate.CanAffectBuySignal),
		fmt.Sprintf("contrarian_gate=%s", emptyText(gate.Status, "none")),
	}
	if !ohlcv.IsCryptoAssetType(result.AssetType) && !ohlcv.IsCommodityAssetType(result.AssetType) {
		details = append(details, fmt.Sprintf("kap_disclosures=%d", coverage.KAPDisclosureCount))
	}
	if coverage.KAPDisclosureCount == 0 && coverage.NewsItemCount == 0 && coverage.AnalyzedTextCount == 0 {
		return InstitutionalValidationCheck{Name: "sentiment_source_policy", Status: "limited", Severity: "critical", Message: "Haber/sentiment verisi yok; bu katman işlem kararını etkileyemez.", Details: details}
	}
	if gate.Status == "fail" && gate.CanAffectBuySignal {
		return InstitutionalValidationCheck{Name: "sentiment_source_policy", Status: "fail", Severity: "critical", Message: "Sentiment kalite kapısı fail iken karar katmanına etki verilmiş.", Details: details}
	}
	if !coverage.HasCommentData {
		details = append(details, "policy=external comments missing; sentiment read-only, buy signal blocked")
	}
	return InstitutionalValidationCheck{Name: "sentiment_source_policy", Status: "pass", Severity: "critical", Message: "Sentiment kaynakları güven politikasına bağlı; sınırlı kaynaklar alım sinyalini tek başına etkileyemez.", Details: details}
}

func validatePeerUniverse(result SymbolAnalysis) InstitutionalValidationCheck {
	company := result.Professional.Company
	peers := result.Professional.Peers
	details := []string{
		"sector=" + emptyText(company.Sector, "unknown"),
		"industry=" + emptyText(company.Industry, "unknown"),
		"source=" + emptyText(company.SectorSource, "unknown"),
		fmt.Sprintf("classification_confidence=%.2f", company.ClassificationConfidence),
		fmt.Sprintf("peer_count=%d", peers.PeerCount),
		fmt.Sprintf("sector_financial_score=%.1f", result.Professional.SectorFinancials.Score),
	}
	if len(peers.Warnings) > 0 {
		details = append(details, "peer_warnings="+strings.Join(peers.Warnings, ","))
	}
	if isBankAnalysisResult(result) {
		missing := professional.MissingBankRegulatoryMetricNames(result.Professional.SectorFinancials)
		if len(missing) > 0 {
			details = append(details, "missing_bank_metrics="+strings.Join(missing, ","))
		}
	}
	switch {
	case company.Sector == "" || company.Sector == "BIST Genel":
		return InstitutionalValidationCheck{Name: "peer_universe", Status: "fail", Severity: "critical", Message: "Sektör sınıflaması güvenilir değil.", Details: details}
	case company.ClassificationConfidence > 0 && company.ClassificationConfidence < 0.80:
		return InstitutionalValidationCheck{Name: "peer_universe", Status: "limited", Severity: "critical", Message: "Sektör sınıflaması var ancak güven skoru kurumsal eşik altında.", Details: details}
	case peers.PeerCount < 3:
		return InstitutionalValidationCheck{Name: "peer_universe", Status: "limited", Severity: "critical", Message: "Peer evreni en az 3 şirket eşiğinin altında.", Details: details}
	case isBankAnalysisResult(result) && bankCoreMetricsMissing(result):
		return InstitutionalValidationCheck{Name: "peer_universe", Status: "fail", Severity: "critical", Message: "Banka peer/değerleme yorumu için çekirdek banka metrikleri tamamlanmadı.", Details: details}
	case isBankAnalysisResult(result) && result.Professional.SectorFinancials.Score > 0 && result.Professional.SectorFinancials.Score < 80:
		return InstitutionalValidationCheck{Name: "peer_universe", Status: "limited", Severity: "critical", Message: "Banka sektör profili kısmi; peer evreni geçse bile banka metrikleri/evren doğrulaması sınırlı.", Details: details}
	case isBankAnalysisResult(result) && len(peers.Warnings) > 0:
		return InstitutionalValidationCheck{Name: "peer_universe", Status: "limited", Severity: "critical", Message: "Banka peer medyanları uç değer filtresiyle sınırlı güvene indirildi.", Details: details}
	default:
		return InstitutionalValidationCheck{Name: "peer_universe", Status: "pass", Severity: "critical", Message: "Sektör ve peer evreni veri kaynaklı sınıflamayla rapora bağlandı.", Details: details}
	}
}

func validateCryptoDataStack(result SymbolAnalysis) InstitutionalValidationCheck {
	coverage := result.Professional.Coverage
	details := []string{
		fmt.Sprintf("asset_type=%s", emptyText(result.AssetType, "unknown")),
		fmt.Sprintf("coverage_score=%.1f", coverage.Score),
		"available=" + strings.Join(coverage.Available, ","),
	}
	if len(coverage.Missing) > 0 {
		details = append(details, "missing="+strings.Join(coverage.Missing, ","))
	}
	if coverage.Score <= 0 {
		return InstitutionalValidationCheck{Name: "crypto_data_stack", Status: "fail", Severity: "critical", Message: "Kripto veri kapsamı yok; rapor üretim kararında kullanılamaz.", Details: details}
	}
	for _, item := range coverage.Missing {
		switch item {
		case "onchain_mvrv_nupl_sopr_realized_cap", "derivatives_funding_open_interest_liquidations", "exchange_flow_reserve_netflow":
			return InstitutionalValidationCheck{Name: "crypto_data_stack", Status: "limited", Severity: "critical", Message: "Kripto için teknik fiyat verisi var; on-chain, derivatives veya exchange-flow kaynakları bağlı olmadığı için kurumsal rapor sınırlı.", Details: details}
		}
	}
	return InstitutionalValidationCheck{Name: "crypto_data_stack", Status: "pass", Severity: "critical", Message: "Kripto fiyat, on-chain/derivatives ve veri kapsamı rapora bağlı.", Details: details}
}

func validateCommodityDataStack(result SymbolAnalysis) InstitutionalValidationCheck {
	coverage := result.Professional.Coverage
	details := []string{
		fmt.Sprintf("asset_type=%s", emptyText(result.AssetType, "unknown")),
		fmt.Sprintf("coverage_score=%.1f", coverage.Score),
		"available=" + strings.Join(coverage.Available, ","),
	}
	if len(coverage.Missing) > 0 {
		details = append(details, "missing="+strings.Join(coverage.Missing, ","))
	}
	if coverage.Score <= 0 {
		return InstitutionalValidationCheck{Name: "commodity_data_stack", Status: "fail", Severity: "critical", Message: "Altın/emtia veri kapsamı yok; rapor üretim kararında kullanılamaz.", Details: details}
	}
	for _, item := range coverage.Missing {
		switch item {
		case "usd_index_dxy_real_yield_macro", "futures_cot_open_interest_positioning", "gold_etf_physical_flow", "central_bank_geopolitical_news":
			return InstitutionalValidationCheck{Name: "commodity_data_stack", Status: "limited", Severity: "critical", Message: "TradingView fiyat grafiği var; DXY/reel faiz, vadeli pozisyon, fon akışı veya merkez bankası/jeopolitik kaynakları bağlı olmadığı için altın raporu sınırlı.", Details: details}
		}
	}
	return InstitutionalValidationCheck{Name: "commodity_data_stack", Status: "pass", Severity: "critical", Message: "Altın/emtia fiyat, makro, pozisyon, akış ve merkez bankası/jeopolitik verileri rapora bağlı.", Details: details}
}

func validateValueInvesting(result SymbolAnalysis) InstitutionalValidationCheck {
	valueReport := result.Professional.ValueInvesting
	details := []string{
		fmt.Sprintf("computed=%t", valueReport.Computed),
		fmt.Sprintf("decision=%s", emptyText(valueReport.Decision, "none")),
		fmt.Sprintf("current_price=%.2f", valueReport.CurrentPrice),
		fmt.Sprintf("intrinsic_base=%.2f", valueReport.IntrinsicValue.Base),
		fmt.Sprintf("margin_of_safety=%.1f%%", valueReport.MarginOfSafety.BasePct),
		fmt.Sprintf("quality_score=%.1f", valueReport.QualityScore),
		fmt.Sprintf("confidence=%.1f", valueReport.Confidence),
		fmt.Sprintf("sector_model=%s", emptyText(valueReport.SectorModel.Model, "none")),
	}
	switch {
	case !valueReport.Computed:
		return InstitutionalValidationCheck{Name: "value_investing", Status: "fail", Severity: "critical", Message: "İçsel değer ve güvenlik marjı hesaplanamadı.", Details: details}
	case bankCoreMetricsMissing(result):
		details = append(details, "missing="+strings.Join(professional.MissingBankRegulatoryMetricNames(result.Professional.SectorFinancials), ","))
		return InstitutionalValidationCheck{Name: "value_investing", Status: "fail", Severity: "critical", Message: "Banka ana metrikleri eksik; içsel değer, hedef fiyat ve AL/SAT kararı kapalı kalmalı.", Details: details}
	case valueReport.Confidence < 45:
		return InstitutionalValidationCheck{Name: "value_investing", Status: "fail", Severity: "critical", Message: "İçsel değer hesabının güveni kurumsal karar için düşük.", Details: details}
	case valueReport.Confidence < 65:
		return InstitutionalValidationCheck{Name: "value_investing", Status: "limited", Severity: "critical", Message: "İçsel değer ve güvenlik marjı hesaplandı fakat güven seviyesi sınırlı.", Details: details}
	default:
		return InstitutionalValidationCheck{Name: "value_investing", Status: "pass", Severity: "critical", Message: "İçsel değer, güvenlik marjı ve değer yatırım kalite skoru rapora bağlandı.", Details: details}
	}
}

func validateInvestorQA(result SymbolAnalysis) InstitutionalValidationCheck {
	qa := result.InvestorQA
	details := []string{
		fmt.Sprintf("computed=%t", qa.Computed),
		fmt.Sprintf("decision=%s", emptyText(qa.Decision, "none")),
		fmt.Sprintf("score=%.1f", qa.Score),
		fmt.Sprintf("confidence=%.1f", qa.Confidence),
		fmt.Sprintf("questions=%d", len(qa.Questions)),
		fmt.Sprintf("quality=%.1f", qa.Quality.Score),
		fmt.Sprintf("liquidity=%.1f", qa.Liquidity.Score),
		fmt.Sprintf("model_risk=%.1f", qa.ModelRisk.Score),
	}
	switch {
	case !qa.Computed:
		return InstitutionalValidationCheck{Name: "investor_questions", Status: "fail", Severity: "critical", Message: "Yatırımcı soru-cevap katmanı üretilmedi.", Details: details}
	case len(qa.Questions) < 12:
		return InstitutionalValidationCheck{Name: "investor_questions", Status: "fail", Severity: "critical", Message: "Rapor yatırımcının temel sorularının yeterli kısmını cevaplamıyor.", Details: details}
	case qa.Confidence < 45:
		return InstitutionalValidationCheck{Name: "investor_questions", Status: "fail", Severity: "critical", Message: "Soru-cevap raporunun güven skoru düşük.", Details: details}
	case bankCoreMetricsMissing(result):
		details = append(details, "missing="+strings.Join(professional.MissingBankRegulatoryMetricNames(result.Professional.SectorFinancials), ","))
		return InstitutionalValidationCheck{Name: "investor_questions", Status: "limited", Severity: "critical", Message: "Yatırımcı soru-cevap katmanı var fakat banka ana metrikleri tamamlanmadan karar güveni sınırlı.", Details: details}
	case qa.Confidence < 65 || qa.ModelRisk.Score < 55:
		return InstitutionalValidationCheck{Name: "investor_questions", Status: "limited", Severity: "critical", Message: "Yatırımcı soru-cevap katmanı var fakat model riski/güven sınırlı.", Details: details}
	default:
		return InstitutionalValidationCheck{Name: "investor_questions", Status: "pass", Severity: "critical", Message: "Rapor yatırımcının ana sorularını veriyle cevaplıyor.", Details: details}
	}
}

func validateInstitutionalPersonaViews(result SymbolAnalysis) InstitutionalValidationCheck {
	views := result.InvestorQA.InstitutionalViews
	marketOnly := ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType)
	details := []string{
		fmt.Sprintf("computed=%t", views.Computed),
		fmt.Sprintf("overall_status=%s", emptyText(views.OverallStatus, "none")),
		fmt.Sprintf("overall_quality_status=%s", emptyText(views.OverallQualityStatus, "none")),
		fmt.Sprintf("elite_candidate=%s/%.1f", emptyText(views.EliteCandidate.Status, "none"), views.EliteCandidate.Score),
		fmt.Sprintf("portfolio=%s/%.1f", emptyText(views.Portfolio.Status, "none"), views.Portfolio.Score),
		fmt.Sprintf("portfolio_quality=%s/%.1f", emptyText(views.Portfolio.ReportQualityStatus, "none"), views.Portfolio.ReportQualityScore),
		fmt.Sprintf("trading_edge=%s/%.1f", emptyText(views.TradingEdge.Status, "none"), views.TradingEdge.Score),
		fmt.Sprintf("trading_edge_quality=%s/%.1f", emptyText(views.TradingEdge.ReportQualityStatus, "none"), views.TradingEdge.ReportQualityScore),
	}
	if !marketOnly {
		details = append(details,
			fmt.Sprintf("value_investing=%s/%.1f", emptyText(views.ValueInvesting.Status, "none"), views.ValueInvesting.Score),
			fmt.Sprintf("value_investing_quality=%s/%.1f", emptyText(views.ValueInvesting.ReportQualityStatus, "none"), views.ValueInvesting.ReportQualityScore),
		)
	}
	switch {
	case !views.Computed:
		return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "fail", Severity: "critical", Message: "Rapor kalite ve aksiyon kapıları üretilmedi.", Details: details}
	case marketOnly:
		if views.Portfolio.Name == "" || views.TradingEdge.Name == "" {
			return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "fail", Severity: "critical", Message: "Rapor kalite ve aksiyon kapısı görünümü eksik.", Details: details}
		}
		if views.OverallQualityStatus == "fail" {
			return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "fail", Severity: "critical", Message: "Portföy ve trading edge kalite kapıları geçmeden rapor başarılı sayılamaz.", Details: details}
		}
		if views.OverallQualityStatus == "limited" {
			return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "limited", Severity: "critical", Message: "Rapor kalite kapılarını üretiyor; fakat en az bir kapı için kanıt kalitesi sınırlı.", Details: details}
		}
		if views.OverallStatus == "fail" {
			return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "limited", Severity: "critical", Message: "Rapor kalite kapıları üretildi; fakat aksiyon kapıları fail olduğu için yatırım/trading uygunluğu yok.", Details: details}
		}
		return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "pass", Severity: "critical", Message: "Kurumsal portföy ve trading edge kalite kapıları geçti; yatırım/trading uygunluğu veriye göre ayrıca değerlendirilir.", Details: details}
	case views.ValueInvesting.Name == "" || views.Portfolio.Name == "" || views.TradingEdge.Name == "":
		return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "fail", Severity: "critical", Message: "Rapor kalite ve aksiyon kapısı görünümü eksik.", Details: details}
	case views.OverallQualityStatus == "fail":
		return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "fail", Severity: "critical", Message: "Üç kalite kapısının rapor kalitesi geçmeden rapor başarılı sayılamaz.", Details: details}
	case views.OverallQualityStatus == "limited":
		return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "limited", Severity: "critical", Message: "Rapor kalite kapılarını üretiyor; fakat en az bir kapı için kanıt kalitesi sınırlı.", Details: details}
	case views.OverallStatus == "fail":
		return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "limited", Severity: "critical", Message: "Rapor kalite kapıları üretildi; fakat değer/portföy/trading aksiyon kapıları fail olduğu için karar uygunluğu yok.", Details: details}
	default:
		return InstitutionalValidationCheck{Name: "institutional_data_gates", Status: "pass", Severity: "critical", Message: "Değer yatırım, kurumsal portföy ve trading edge kalite kapılarının rapor kalitesi geçti; yatırım/trading uygunluğu veriye göre ayrıca değerlendirilir.", Details: details}
	}
}

func isBankAnalysisResult(result SymbolAnalysis) bool {
	text := strings.ToLower(strings.Join([]string{
		result.Professional.Company.Sector,
		result.Professional.Company.Industry,
		result.Professional.Valuation.SectorModel,
		result.Professional.SectorFinancials.Profile,
		result.Professional.SectorFinancials.ProfileLabel,
		result.Professional.ValueInvesting.SectorModel.Model,
	}, " "))
	return strings.Contains(text, "bank") || strings.Contains(text, "banka")
}

func bankCoreMetricsMissing(result SymbolAnalysis) bool {
	for _, flag := range result.Professional.Valuation.Flags {
		if flag == "bank_sector_requires_regulatory_capital_and_asset_quality_model" {
			return true
		}
		if flag == "bank_book_value_reconciliation_missing" || flag == "bank_book_value_reconciliation_failed" {
			return true
		}
	}
	if !isBankAnalysisResult(result) {
		return false
	}
	return !professional.BankRegulatoryMetricsComplete(result.Professional.SectorFinancials)
}

func validateExplainability(result SymbolAnalysis) InstitutionalValidationCheck {
	details := []string{}
	missing := []string{}
	for key, tf := range result.Timeframes {
		plan := tf.TradePlan
		if !activeExplainedPlan(plan) {
			missing = append(missing, key)
		}
		details = append(details, fmt.Sprintf("%s direction=%s rejected=%t reason=%s reasoning=%d", key, emptyText(plan.Direction, "none"), plan.Rejected, emptyText(localize.Reason(plan.RejectReason), "none"), len(plan.Reasoning)))
	}
	if result.Professional.DataQuality <= 0 {
		missing = append(missing, "professional.data_quality")
	}
	coverageLimited := (ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType)) && result.Professional.Coverage.Score < 70
	if !ohlcv.IsCryptoAssetType(result.AssetType) && !ohlcv.IsCommodityAssetType(result.AssetType) && result.Professional.Coverage.Score < 85 {
		missing = append(missing, "professional.coverage")
	}
	if len(missing) > 0 {
		details = append(details, "missing="+strings.Join(missing, ","))
		return InstitutionalValidationCheck{Name: "explainability_audit", Status: "fail", Severity: "critical", Message: "Karar nedenleri ve veri kapsamı tam açıklanmıyor.", Details: details}
	}
	if coverageLimited {
		details = append(details, fmt.Sprintf("market_data_coverage_limited=%.1f", result.Professional.Coverage.Score))
		return InstitutionalValidationCheck{Name: "explainability_audit", Status: "limited", Severity: "critical", Message: "Karar teknik olarak açıklanıyor; ek piyasa veri kapsamı sınırlı olduğu için kesin işlem kararı değildir.", Details: details}
	}
	return InstitutionalValidationCheck{Name: "explainability_audit", Status: "pass", Severity: "critical", Message: "Karar, ret nedeni, işlem planı ve finansal kapsam açıklanabilir durumda.", Details: details}
}

func activeExplainedPlan(plan ohlcv.TradePlan) bool {
	if plan.Direction == "" {
		return plan.RejectReason != "" || len(plan.Reasoning) > 0
	}
	if plan.Rejected {
		return plan.RejectReason != "" || len(plan.Reasoning) > 0
	}
	if plan.Direction == "neutral" {
		return true
	}
	return plan.EntryMax > 0 &&
		plan.StopLoss > 0 &&
		plan.TakeProfit1 > 0 &&
		(plan.RejectReason != "" || len(plan.Reasoning) > 0 || plan.ConfidenceScore > 0)
}

func validateVisualTextPolicy(result SymbolAnalysis) InstitutionalValidationCheck {
	texts := []string{
		result.Symbol,
		result.CompanyName,
		result.Disclaimer,
		result.Professional.Company.Sector,
		result.Professional.Company.Industry,
		result.Behavioral.Sentiment.PlainLanguage,
		result.Behavioral.Contrarian.PlainLanguage,
	}
	for _, tf := range result.Timeframes {
		texts = append(texts, tf.TradePlan.RejectReason)
		texts = append(texts, tf.TradePlan.Reasoning...)
		for _, pattern := range tf.Patterns {
			texts = append(texts, pattern.Name)
			texts = append(texts, pattern.Evidence...)
		}
	}
	bad := suspiciousVisualText(texts)
	details := []string{
		"policy=reject mojibake, markdown pipes, duplicated MA tokens and known broken chart phrases",
		fmt.Sprintf("checked_texts=%d", len(texts)),
	}
	if len(bad) > 0 {
		details = append(details, "bad_text="+strings.Join(firstValidationStrings(bad, 8), " | "))
		return InstitutionalValidationCheck{Name: "visual_text_quality", Status: "fail", Severity: "critical", Message: "Grafik/PDF kullanıcı metinlerinde bozuk veya profesyonel olmayan ifade yakalandı.", Details: details}
	}
	return InstitutionalValidationCheck{Name: "visual_text_quality", Status: "pass", Severity: "critical", Message: "Rapor metni ham markdown/bozuk phrase kapısından geçti.", Details: details}
}

func suspiciousVisualText(texts []string) []string {
	bad := []string{}
	for _, text := range texts {
		lower := strings.ToLower(strings.TrimSpace(text))
		if lower == "" {
			continue
		}
		switch {
		case strings.Contains(lower, "spe hiez"):
			bad = append(bad, text)
		case strings.Contains(lower, "de ir alm"):
			bad = append(bad, text)
		case strings.Contains(lower, "plai ylan"):
			bad = append(bad, text)
		case strings.Contains(lower, "ma20/ma20"):
			bad = append(bad, text)
		case strings.Contains(lower, "|") && strings.Contains(lower, "---"):
			bad = append(bad, text)
		case strings.Contains(lower, "undefined") || strings.Contains(lower, "<nil>") || strings.Contains(lower, "%!"):
			bad = append(bad, text)
		}
	}
	return bad
}

func failCheck(name, severity, message string) InstitutionalValidationCheck {
	return InstitutionalValidationCheck{Name: name, Status: "fail", Severity: severity, Message: message}
}

func emptyText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstValidationStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	out := append([]string{}, values[:limit]...)
	out = append(out, fmt.Sprintf("...+%d", len(values)-limit))
	return out
}

func institutionalSummary(report InstitutionalValidation) string {
	switch report.Status {
	case "pass":
		return "Rapor güvenlik ve doğrulama kapısı geçti; çıktı yatırım tavsiyesi değildir ve ölçülen veri/model koşullarıyla sınırlıdır."
	case "limited":
		return "Rapor güvenlik ve doğrulama kapısı sınırlı geçti; eksik/sınırlı katmanlar işlem kararını tek başına etkileyemez."
	default:
		return "Rapor güvenlik ve doğrulama kapısı başarısız; çıktı ana karar veya production trading aracı olarak kullanılmamalıdır."
	}
}
