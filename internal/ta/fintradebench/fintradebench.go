package fintradebench

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
)

const Version = "fintradebench_v1"

type Input struct {
	Symbol       string
	AssetType    string
	AsOf         time.Time
	LastClose    float64
	Candles      []ohlcv.Candle
	Indicators   ohlcv.IndicatorSnapshot
	Professional professional.Report
}

type Report struct {
	Computed                bool                    `json:"computed"`
	Version                 string                  `json:"version"`
	SourcePaper             string                  `json:"source_paper"`
	Symbol                  string                  `json:"symbol,omitempty"`
	AsOf                    string                  `json:"as_of,omitempty"`
	Summary                 string                  `json:"summary,omitempty"`
	QuestionTaxonomy        []ReasoningTrack        `json:"question_taxonomy"`
	TradingSignals          []Signal                `json:"trading_signals,omitempty"`
	FundamentalSignals      []Signal                `json:"fundamental_signals,omitempty"`
	SupplementalSignals     []Signal                `json:"supplemental_signals,omitempty"`
	DocumentSignals         []Signal                `json:"document_signals,omitempty"`
	DocumentEvidence        DocumentEvidence        `json:"document_evidence"`
	CalculationValidation   CalculationValidation   `json:"calculation_validation"`
	GoldenIndicatorCoverage GoldenIndicatorCoverage `json:"golden_indicator_coverage"`
	NumericalAudit          NumericalAudit          `json:"numerical_audit"`
	RetrievalContext        RetrievalContext        `json:"retrieval_context"`
	PointInTime             PointInTimeReport       `json:"point_in_time"`
	HybridReadinessScore    float64                 `json:"hybrid_readiness_score"`
	Warnings                []string                `json:"warnings,omitempty"`
	Limitations             []string                `json:"limitations,omitempty"`
}

type Signal struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name"`
	Family        string   `json:"family"`
	Formula       string   `json:"formula,omitempty"`
	Value         float64  `json:"value,omitempty"`
	Unit          string   `json:"unit,omitempty"`
	Available     bool     `json:"available"`
	Direction     string   `json:"direction,omitempty"`
	Strength      float64  `json:"strength,omitempty"`
	Source        string   `json:"source,omitempty"`
	AsOf          string   `json:"as_of,omitempty"`
	MissingReason string   `json:"missing_reason,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

type ReasoningTrack struct {
	Type             string   `json:"type"`
	Description      string   `json:"description"`
	RequiredSignals  []string `json:"required_signals"`
	AvailableSignals []string `json:"available_signals"`
	MissingSignals   []string `json:"missing_signals,omitempty"`
	Coverage         float64  `json:"coverage"`
	Status           string   `json:"status"`
}

type GoldenIndicatorCoverage struct {
	Required  []string `json:"required"`
	Generated []string `json:"generated"`
	Missing   []string `json:"missing,omitempty"`
	Precision float64  `json:"precision"`
	Recall    float64  `json:"recall"`
	F1        float64  `json:"f1"`
}

type NumericalAudit struct {
	Status              string   `json:"status"`
	StructuredSignals   int      `json:"structured_signals"`
	AvailableSignals    int      `json:"available_signals"`
	MissingSignals      int      `json:"missing_signals"`
	Contradictions      []string `json:"contradictions,omitempty"`
	UnsupportedRequired []string `json:"unsupported_required,omitempty"`
}

type RetrievalContext struct {
	Mode         string   `json:"mode"`
	ContextType  string   `json:"context_type"`
	ContextLines []string `json:"context_lines,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

type PointInTimeReport struct {
	Status            string   `json:"status"`
	AsOf              string   `json:"as_of,omitempty"`
	MarketDataThrough string   `json:"market_data_through,omitempty"`
	FundamentalPeriod string   `json:"fundamental_period,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

type DocumentEvidence struct {
	Computed                      bool                   `json:"computed"`
	Status                        string                 `json:"status"`
	Source                        string                 `json:"source,omitempty"`
	Summary                       string                 `json:"summary,omitempty"`
	TotalDocuments                int                    `json:"total_documents"`
	SourcePDFCount                int                    `json:"source_pdf_count,omitempty"`
	AnalysisUsableCount           int                    `json:"analysis_usable_count"`
	DecisionRelevantDocuments     int                    `json:"decision_relevant_documents,omitempty"`
	DecisionRelevantUsableCount   int                    `json:"decision_relevant_usable_count,omitempty"`
	ReviewRequiredCount           int                    `json:"review_required_count"`
	RejectedCount                 int                    `json:"rejected_count"`
	OCRUsedCount                  int                    `json:"ocr_used_count,omitempty"`
	ErrorCount                    int                    `json:"error_count,omitempty"`
	AverageQuality                float64                `json:"average_quality,omitempty"`
	CoverageScore                 float64                `json:"coverage_score"`
	DecisionRelevantCoverageScore float64                `json:"decision_relevant_coverage_score,omitempty"`
	TypeCounts                    []DocumentTypeEvidence `json:"type_counts,omitempty"`
	KeyDocuments                  []DocumentEvidenceItem `json:"key_documents,omitempty"`
	Warnings                      []string               `json:"warnings,omitempty"`
	RequiredActions               []string               `json:"required_actions,omitempty"`
}

type DocumentTypeEvidence struct {
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`
	Count int    `json:"count"`
}

type DocumentEvidenceItem struct {
	FileName       string   `json:"file_name,omitempty"`
	DocumentType   string   `json:"document_type,omitempty"`
	DocumentLabel  string   `json:"document_label,omitempty"`
	TextLength     int      `json:"text_length,omitempty"`
	QualityScore   float64  `json:"quality_score,omitempty"`
	ParseStatus    string   `json:"parse_status,omitempty"`
	AnalysisUsable bool     `json:"analysis_usable"`
	ContentSnippet string   `json:"content_snippet,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

func Analyze(input Input) Report {
	symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
	candles := sortedCandles(input.Candles)
	if input.AsOf.IsZero() && len(candles) > 0 {
		input.AsOf = candles[len(candles)-1].Time
	}
	lastClose := input.LastClose
	if lastClose <= 0 && len(candles) > 0 {
		lastClose = candles[len(candles)-1].EffectiveClose()
	}

	report := Report{
		Computed:    len(candles) > 0 || hasProfessionalFundamentals(input.Professional) || hasDocumentEvidence(input.Professional),
		Version:     Version,
		SourcePaper: "FinTradeBench: A Financial Reasoning Benchmark for LLMs, arXiv:2603.19225v5",
		Symbol:      symbol,
		AsOf:        formatDate(input.AsOf),
		Limitations: []string{
			"benchmark-inspired reasoning layer; not a trading system or portfolio optimizer",
			"llm-as-a-judge calibration from the paper is not executed without a configured external judge",
		},
	}
	if !report.Computed {
		report.Summary = "FinTradeBench layer could not compute because OHLCV, professional fundamentals, and KAP/PDF evidence were unavailable."
		report.Warnings = append(report.Warnings, "fintradebench_inputs_missing")
		return report
	}

	report.TradingSignals = buildTradingSignals(candles, input.Indicators, lastClose)
	report.FundamentalSignals = buildFundamentalSignals(input.Professional, lastClose)
	report.SupplementalSignals = buildSupplementalSignals(input.Professional, lastClose)
	report.DocumentEvidence = buildDocumentEvidence(input.Professional)
	report.DocumentSignals = buildDocumentSignals(report.DocumentEvidence)
	report.CalculationValidation = buildCalculationValidation(input, candles, lastClose, report.TradingSignals, report.FundamentalSignals, report.SupplementalSignals)
	report.PointInTime = buildPointInTime(input, candles)
	report.QuestionTaxonomy = buildQuestionTaxonomy(report.TradingSignals, report.FundamentalSignals, report.DocumentSignals)
	report.GoldenIndicatorCoverage = buildGoldenIndicatorCoverage(report.TradingSignals, report.FundamentalSignals)
	report.NumericalAudit = buildNumericalAudit(report.GoldenIndicatorCoverage, report.CalculationValidation, report.DocumentEvidence, report.TradingSignals, report.FundamentalSignals, report.DocumentSignals)
	report.RetrievalContext = buildRetrievalContext(report)
	report.HybridReadinessScore = hybridReadiness(report.QuestionTaxonomy, report.NumericalAudit)
	report.Summary = summary(report)
	report.Warnings = append(report.Warnings, report.PointInTime.Warnings...)
	report.Warnings = append(report.Warnings, report.RetrievalContext.Warnings...)
	report.Warnings = append(report.Warnings, report.DocumentEvidence.Warnings...)
	report.Warnings = uniqueStrings(report.Warnings)
	return report
}

func buildTradingSignals(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, lastClose float64) []Signal {
	asOf := ""
	if len(candles) > 0 {
		asOf = formatDate(candles[len(candles)-1].Time)
	}
	signals := []Signal{}
	add := func(signal Signal) {
		if signal.Family == "" {
			signal.Family = "trading"
		}
		if signal.AsOf == "" {
			signal.AsOf = asOf
		}
		signals = append(signals, signal)
	}
	add(valueSignal("ma_20", "MA 20", "trading", "sum(P_t-i, i=1..N)/N", snapshot.SMA20, "price", snapshot.SMA20 > 0, maDirection(lastClose, snapshot.SMA20), "indicators.snapshot.sma20"))
	add(valueSignal("ema_20", "EMA 20", "trading", "alpha*P_t + (1-alpha)*EMA_t-1", snapshot.EMA20, "price", snapshot.EMA20 > 0, maDirection(lastClose, snapshot.EMA20), "indicators.snapshot.ema20"))
	add(valueSignal("macd", "MACD", "trading", "EMA_short - EMA_long", snapshot.MACD, "price", snapshot.MACD != 0 || snapshot.MACDSignal != 0 || snapshot.MACDHistogram != 0, macdDirection(snapshot), "indicators.snapshot.macd"))
	add(valueSignal("rsi_14", "RSI 14", "trading", "100 - 100/(1+RS)", snapshot.RSI14, "score", snapshot.RSI14 > 0 && snapshot.RSI14 <= 100, rsiDirection(snapshot.RSI14), "indicators.snapshot.rsi14"))
	add(valueSignal("obv", "On-Balance Volume", "trading", "OBV_t-1 + V_t*sgn(P_t-P_t-1)", snapshot.OBV, "volume", snapshot.OBV != 0 || snapshot.OBVSlope != 0, obvDirection(snapshot.OBVSlope), "indicators.snapshot.obv"))

	oneDay, ok := oneDayReversal(candles)
	add(valueSignal("one_day_reversal", "One-Day Reversal", "trading", "(P_t-P_t-1)/P_t-1", oneDay, "pct", ok, returnDirection(oneDay, 0.01), "ohlcv.daily.close"))
	maxRet, ok := maxDailyReturn(candles, 20)
	add(valueSignal("max_return_20d", "Max Return 20D", "trading", "max((P_t-i-P_t-i-1)/P_t-i-1), i<=20", maxRet, "pct", ok, volatilityShockDirection(maxRet), "ohlcv.daily.close"))
	momentum, ok := cumulativeReturn(candles, 60)
	add(valueSignal("medium_term_momentum_60d", "Medium-Term Momentum 60D", "trading", "product(1+R_t-i)-1", momentum, "pct", ok, returnDirection(momentum, 0.05), "ohlcv.daily.close"))
	meanRev, ok := longTermMeanReversion(candles, snapshot, lastClose)
	add(valueSignal("long_term_mean_reversion", "Long-Term Mean Reversion", "trading", "(Pbar_long-P_t)/P_t", meanRev, "pct", ok, meanReversionDirection(meanRev), "indicators.snapshot.sma200"))
	realizedVol, ok := realizedVolatility(candles, 20)
	add(valueSignal("realized_volatility_20d", "Realized Volatility 20D", "trading", "std(log returns, 20)*sqrt(252)", realizedVol, "pct", ok, volatilityDirection(realizedVol), "ohlcv.daily.close"))
	drawdown, ok := maxDrawdown(candles, 20)
	add(valueSignal("max_drawdown_20d", "Max Drawdown 20D", "trading", "min(P_t/rolling_peak-1), 20D", drawdown, "pct", ok, drawdownDirection(drawdown), "ohlcv.daily.close"))
	if snapshot.VolumeSMA20 > 0 && len(candles) > 0 {
		ratio := candles[len(candles)-1].EffectiveVolume() / snapshot.VolumeSMA20
		add(valueSignal("volume_sma20_ratio", "Volume / SMA20 Volume", "trading", "V_t/SMA20(V)", ratio, "ratio", true, volumeDirection(ratio), "indicators.snapshot.volume_sma20"))
	}
	return signals
}

func buildFundamentalSignals(pro professional.Report, lastClose float64) []Signal {
	v := pro.Valuation
	asOf := formatFundamentalPeriod(v)
	ratios := v.Ratios
	if ratios == nil {
		ratios = map[string]float64{}
	}
	signals := []Signal{}
	add := func(signal Signal) {
		if signal.Family == "" {
			signal.Family = "fundamental"
		}
		if signal.AsOf == "" {
			signal.AsOf = asOf
		}
		signals = append(signals, signal)
	}
	add(valueSignal("cash_flow_assets", "Cash Flow / Assets", "fundamental", "operating cash flow / total assets", safeDiv(v.OperatingCashTTM, v.TotalAssets), "ratio", v.OperatingCashTTM != 0 && v.TotalAssets > 0, qualityRatioDirection(safeDiv(v.OperatingCashTTM, v.TotalAssets), 0.05, 0), "professional.valuation.operating_cash_ttm,total_assets"))
	bookPrice := firstNonZero(inversePositive(ratios["PB"]), safeDiv(v.Equity, v.MarketCap))
	add(valueSignal("book_price", "Book / Price", "fundamental", "book value of equity / market capitalization", bookPrice, "ratio", ratios["PB"] > 0 || (v.Equity > 0 && v.MarketCap > 0), valuationYieldDirection(bookPrice), "professional.valuation.ratios.pb"))
	earningsPrice := firstNonZero(inversePositive(ratios["PE"]), safeDiv(v.NetIncomeTTM, v.MarketCap))
	add(valueSignal("earnings_price", "Earnings / Price", "fundamental", "earnings per share / price per share", earningsPrice, "ratio", ratios["PE"] > 0 || (v.NetIncomeTTM != 0 && v.MarketCap > 0), earningsYieldDirection(earningsPrice), "professional.valuation.ratios.pe"))
	add(missingSignal("forecast_earnings_price", "Forecast Earnings / Price", "fundamental", "expected future EPS / price per share", "analyst EPS forecast is not loaded as a structured point-in-time input"))
	add(valueSignal("sales_assets", "Sales / Assets", "fundamental", "total sales / total assets", safeDiv(v.SalesTTM, v.TotalAssets), "ratio", v.SalesTTM != 0 && v.TotalAssets > 0, qualityRatioDirection(safeDiv(v.SalesTTM, v.TotalAssets), 1.0, 0.25), "professional.valuation.sales_ttm,total_assets"))
	debtAssets := safeDiv(v.TotalDebt, v.TotalAssets)
	add(valueSignal("debt_assets", "Debt / Assets", "fundamental", "total debt / total assets", debtAssets, "ratio", v.DebtDataAvailable && v.TotalAssets > 0, leverageDirection(debtAssets), "professional.valuation.total_debt,total_assets"))
	debtEquity := safeDiv(v.TotalDebt, v.Equity)
	add(valueSignal("debt_equity", "Debt / Equity", "fundamental", "total debt / shareholders equity", debtEquity, "ratio", v.DebtDataAvailable && v.Equity > 0, leverageDirection(debtEquity), "professional.valuation.total_debt,equity"))
	add(dividendYieldSignal(pro, lastClose))
	roa := firstNonZero(ratios["ROA"], safeDiv(v.NetIncomeTTM, v.TotalAssets))
	add(valueSignal("return_assets", "Return on Assets", "fundamental", "net income / total assets", roa, "ratio", ratioAvailable(ratios, "ROA") || (v.NetIncomeTTM != 0 && v.TotalAssets > 0), qualityRatioDirection(roa, 0.08, 0), "professional.valuation.ratios.roa"))
	roe := firstNonZero(ratios["ROE"], safeDiv(v.NetIncomeTTM, v.Equity))
	add(valueSignal("return_equity", "Return on Equity", "fundamental", "net income / shareholders equity", roe, "ratio", ratioAvailable(ratios, "ROE") || (v.NetIncomeTTM != 0 && v.Equity > 0), qualityRatioDirection(roe, 0.18, 0), "professional.valuation.ratios.roe"))
	return signals
}

func buildSupplementalSignals(pro professional.Report, _ float64) []Signal {
	v := pro.Valuation
	ratios := v.Ratios
	if ratios == nil {
		ratios = map[string]float64{}
	}
	asOf := formatFundamentalPeriod(v)
	signals := []Signal{}
	add := func(signal Signal) {
		signal.Family = "supplemental"
		if signal.AsOf == "" {
			signal.AsOf = asOf
		}
		signals = append(signals, signal)
	}
	fcfYield := firstNonZero(ratios["FCF_Yield"], safeDiv(v.FreeCashFlowTTM, v.MarketCap))
	add(valueSignal("free_cash_flow_yield", "Free Cash Flow Yield", "supplemental", "free cash flow / market capitalization", fcfYield, "ratio", ratioAvailable(ratios, "FCF_Yield") || (v.FreeCashFlowTTM != 0 && v.MarketCap > 0), earningsYieldDirection(fcfYield), "professional.valuation.ratios.fcf_yield"))
	bookPerShare := firstNonZero(ratios["BookPerShare"], safeDiv(v.Equity, v.PaidCapital))
	add(valueSignal("book_per_share", "Book Per Share", "supplemental", "shareholders equity / paid capital", bookPerShare, "price", ratioAvailable(ratios, "BookPerShare") || (v.Equity > 0 && v.PaidCapital > 0), valueDirection(bookPerShare), "professional.valuation.ratios.book_per_share"))
	netDebtEquity := firstNonZero(ratios["NetDebt_Eq"], safeDiv(v.NetDebt, v.Equity))
	add(valueSignal("net_debt_equity", "Net Debt / Equity", "supplemental", "net debt / shareholders equity", netDebtEquity, "ratio", ratioAvailable(ratios, "NetDebt_Eq") || (v.NetDebt != 0 && v.Equity > 0), leverageDirection(netDebtEquity), "professional.valuation.ratios.netdebt_eq"))
	if pro.ValueInvesting.CapitalAllocation.DividendDataAvailable || pro.ValueInvesting.CapitalAllocation.Dividends10Y != 0 {
		add(valueSignal("dividend_payout_ratio_10y", "Dividend Payout Ratio 10Y", "supplemental", "10Y dividends / 10Y net income", pro.ValueInvesting.CapitalAllocation.DividendPayoutRatio10Y, "ratio", true, payoutDirection(pro.ValueInvesting.CapitalAllocation.DividendPayoutRatio10Y), "professional.value_investing.capital_allocation"))
	}
	return signals
}

func buildDocumentEvidence(pro professional.Report) DocumentEvidence {
	kap := pro.KAPPDFIngest
	evidence := DocumentEvidence{
		Computed:                      kap.Computed,
		Source:                        "professional.kap_pdf_ingest",
		Summary:                       strings.TrimSpace(kap.Summary),
		TotalDocuments:                kap.TotalDocuments,
		SourcePDFCount:                kap.SourcePDFCount,
		AnalysisUsableCount:           kap.AnalysisUsableCount,
		DecisionRelevantDocuments:     kap.DecisionRelevantDocuments,
		DecisionRelevantUsableCount:   kap.DecisionRelevantUsableCount,
		ReviewRequiredCount:           kap.ReviewRequiredCount,
		RejectedCount:                 kap.RejectedCount,
		OCRUsedCount:                  kap.OCRUsedCount,
		ErrorCount:                    kap.ErrorCount,
		AverageQuality:                round4(kap.AverageQuality),
		CoverageScore:                 round2(percentInt(kap.AnalysisUsableCount, kap.TotalDocuments)),
		DecisionRelevantCoverageScore: round2(percentInt(kap.DecisionRelevantUsableCount, kap.DecisionRelevantDocuments)),
		TypeCounts:                    documentTypeEvidence(kap.TypeCounts),
		KeyDocuments:                  documentEvidenceItems(kap.ImportantDocuments, 12),
		Warnings:                      append([]string(nil), kap.Warnings...),
	}
	evidence.Status = documentEvidenceStatus(evidence)
	evidence.RequiredActions = documentEvidenceActions(evidence)
	return evidence
}

func buildDocumentSignals(evidence DocumentEvidence) []Signal {
	signals := []Signal{}
	add := func(signal Signal) {
		if signal.Family == "" {
			signal.Family = "document"
		}
		signals = append(signals, signal)
	}
	coverage := evidence.CoverageScore / 100
	add(valueSignal("kap_pdf_usable_coverage", "KAP PDF Usable Coverage", "document", "analysis_usable_documents / total_documents", coverage, "ratio", evidence.Computed && evidence.TotalDocuments > 0, documentCoverageDirection(evidence.CoverageScore), evidence.Source))
	if evidence.DecisionRelevantDocuments > 0 {
		decisionCoverage := evidence.DecisionRelevantCoverageScore / 100
		add(valueSignal("kap_pdf_decision_relevant_coverage", "KAP PDF Decision-Relevant Coverage", "document", "decision_relevant_usable_documents / decision_relevant_documents", decisionCoverage, "ratio", evidence.Computed, documentCoverageDirection(evidence.DecisionRelevantCoverageScore), evidence.Source))
	} else {
		add(missingSignal("kap_pdf_decision_relevant_coverage", "KAP PDF Decision-Relevant Coverage", "document", "decision_relevant_usable_documents / decision_relevant_documents", "decision-relevant KAP/PDF document class coverage is unavailable"))
	}
	reviewRatio := safeDiv(float64(evidence.ReviewRequiredCount), float64(evidence.TotalDocuments))
	add(valueSignal("kap_pdf_review_required_ratio", "KAP PDF Review Required Ratio", "document", "review_required_documents / total_documents", reviewRatio, "ratio", evidence.Computed && evidence.TotalDocuments > 0, documentReviewDirection(reviewRatio), evidence.Source))
	rejectedRatio := safeDiv(float64(evidence.RejectedCount), float64(evidence.TotalDocuments))
	add(valueSignal("kap_pdf_rejected_ratio", "KAP PDF Rejected Ratio", "document", "rejected_documents / total_documents", rejectedRatio, "ratio", evidence.Computed && evidence.TotalDocuments > 0, documentRejectedDirection(rejectedRatio), evidence.Source))
	return signals
}

func documentTypeEvidence(counts []professional.KAPPDFTypeCount) []DocumentTypeEvidence {
	out := make([]DocumentTypeEvidence, 0, len(counts))
	for _, count := range counts {
		if count.Count <= 0 {
			continue
		}
		out = append(out, DocumentTypeEvidence{
			Type:  strings.TrimSpace(count.Type),
			Label: strings.TrimSpace(count.Label),
			Count: count.Count,
		})
	}
	return out
}

func documentEvidenceItems(docs []professional.KAPPDFDocumentSummary, limit int) []DocumentEvidenceItem {
	if limit <= 0 || len(docs) == 0 {
		return nil
	}
	if len(docs) > limit {
		docs = docs[:limit]
	}
	out := make([]DocumentEvidenceItem, 0, len(docs))
	for _, doc := range docs {
		out = append(out, DocumentEvidenceItem{
			FileName:       strings.TrimSpace(doc.FileName),
			DocumentType:   strings.TrimSpace(doc.DocumentType),
			DocumentLabel:  strings.TrimSpace(doc.DocumentLabel),
			TextLength:     doc.TextLength,
			QualityScore:   round4(doc.QualityScore),
			ParseStatus:    strings.TrimSpace(doc.ParseStatus),
			AnalysisUsable: doc.AnalysisUsable,
			ContentSnippet: strings.TrimSpace(doc.ContentSnippet),
			Warnings:       append([]string(nil), doc.Warnings...),
		})
	}
	return out
}

func documentEvidenceStatus(evidence DocumentEvidence) string {
	switch {
	case !evidence.Computed:
		return "missing"
	case evidence.TotalDocuments <= 0:
		return "failed"
	case evidence.AnalysisUsableCount <= 0:
		return "failed"
	case evidence.CoverageScore < 60:
		return "limited"
	case evidence.DecisionRelevantDocuments > 0 && evidence.DecisionRelevantCoverageScore < 60:
		return "limited"
	case evidence.RejectedCount > evidence.AnalysisUsableCount:
		return "limited"
	case safeDiv(float64(evidence.ReviewRequiredCount), float64(evidence.TotalDocuments)) > 0.35:
		return "limited"
	case safeDiv(float64(evidence.RejectedCount), float64(evidence.TotalDocuments)) > 0.10:
		return "limited"
	default:
		return "passed"
	}
}

func documentEvidenceActions(evidence DocumentEvidence) []string {
	switch evidence.Status {
	case "missing":
		return []string{"run_kap_pdf_ingest"}
	case "failed":
		return []string{"rerun_kap_pdf_ingest", "review_pdf_text_extraction_errors"}
	case "limited":
		actions := []string{}
		if evidence.CoverageScore < 70 {
			actions = append(actions, "review_low_quality_or_unusable_pdf_extracts")
		}
		if evidence.DecisionRelevantDocuments > 0 && evidence.DecisionRelevantCoverageScore < 70 {
			actions = append(actions, "review_decision_relevant_pdf_extracts")
		}
		if evidence.ReviewRequiredCount > 0 {
			actions = append(actions, "human_review_required_pdf_documents")
		}
		if evidence.RejectedCount > 0 {
			actions = append(actions, "repair_rejected_pdf_extractions")
		}
		return uniqueStrings(actions)
	default:
		return nil
	}
}

func buildQuestionTaxonomy(trading []Signal, fundamentals []Signal, documents []Signal) []ReasoningTrack {
	fRequired := []string{"cash_flow_assets", "book_price", "earnings_price", "sales_assets", "debt_equity", "return_assets", "return_equity"}
	tRequired := []string{"ma_20", "ema_20", "macd", "rsi_14", "obv", "one_day_reversal", "max_return_20d", "medium_term_momentum_60d", "long_term_mean_reversion"}
	dRequired := []string{"kap_pdf_usable_coverage", "kap_pdf_decision_relevant_coverage"}
	allRequired := uniqueStrings(append(append(append([]string{}, fRequired...), tRequired...), dRequired...))
	available := availableSet(append(append(append([]Signal{}, trading...), fundamentals...), documents...))
	return []ReasoningTrack{
		reasoningTrack("F", "fundamentals-focused reasoning over accounting and valuation signals", fRequired, available),
		reasoningTrack("T", "trading-signal-focused reasoning over OHLCV-derived indicators", tRequired, available),
		reasoningTrack("D", "document-evidence reasoning over KAP/PDF filings and extraction quality gates", dRequired, available),
		reasoningTrack("FT", "hybrid reasoning that integrates company fundamentals, KAP/PDF evidence, and market behavior", allRequired, available),
	}
}

func reasoningTrack(kind, description string, required []string, available map[string]bool) ReasoningTrack {
	present := []string{}
	missing := []string{}
	for _, name := range required {
		if available[name] {
			present = append(present, name)
		} else {
			missing = append(missing, name)
		}
	}
	coverage := 0.0
	if len(required) > 0 {
		coverage = float64(len(present)) / float64(len(required))
	}
	status := "fail"
	switch {
	case coverage >= 0.85:
		status = "ready"
	case coverage >= 0.60:
		status = "limited"
	}
	return ReasoningTrack{
		Type:             kind,
		Description:      description,
		RequiredSignals:  append([]string(nil), required...),
		AvailableSignals: present,
		MissingSignals:   missing,
		Coverage:         round4(coverage),
		Status:           status,
	}
}

func buildGoldenIndicatorCoverage(trading []Signal, fundamentals []Signal) GoldenIndicatorCoverage {
	required := uniqueStrings([]string{
		"cash_flow_assets", "book_price", "earnings_price", "forecast_earnings_price", "sales_assets", "debt_assets", "debt_equity", "dividend_yield", "return_assets", "return_equity",
		"ma_20", "ema_20", "macd", "rsi_14", "obv", "one_day_reversal", "max_return_20d", "medium_term_momentum_60d", "long_term_mean_reversion",
	})
	signals := append(append([]Signal{}, trading...), fundamentals...)
	generated := []string{}
	for _, signal := range signals {
		if signal.Available {
			generated = append(generated, signal.Name)
		}
	}
	return coverage(required, generated)
}

func buildNumericalAudit(coverage GoldenIndicatorCoverage, validation CalculationValidation, documents DocumentEvidence, trading []Signal, fundamentals []Signal, documentSignals []Signal) NumericalAudit {
	signals := append(append(append([]Signal{}, trading...), fundamentals...), documentSignals...)
	available := 0
	contradictions := []string{}
	for _, signal := range signals {
		if signal.Available {
			available++
		}
		if signal.Available && (!isFinite(signal.Value) || signal.Unit == "pct" && math.Abs(signal.Value) > 10) {
			contradictions = append(contradictions, signal.Name)
		}
	}
	for _, check := range validation.Checks {
		if check.Status == "failed" {
			contradictions = append(contradictions, "calculation:"+check.Name)
		}
	}
	status := "passed"
	if len(contradictions) > 0 {
		status = "failed"
	} else if coverage.F1 < 0.70 || validation.Status == "limited" || documents.Status == "missing" || documents.Status == "failed" || documents.Status == "limited" {
		status = "limited"
	}
	unsupported := append([]string(nil), coverage.Missing...)
	if documents.Status == "missing" || documents.Status == "failed" {
		unsupported = append(unsupported, "document_evidence")
	}
	return NumericalAudit{
		Status:              status,
		StructuredSignals:   len(signals),
		AvailableSignals:    available,
		MissingSignals:      len(signals) - available,
		Contradictions:      contradictions,
		UnsupportedRequired: uniqueStrings(unsupported),
	}
}

func buildRetrievalContext(report Report) RetrievalContext {
	lines := []string{}
	for _, signal := range append(append(append([]Signal{}, report.TradingSignals...), report.FundamentalSignals...), report.DocumentSignals...) {
		if !signal.Available {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s.%s=%s direction=%s source=%s", signal.Family, signal.Name, formatValue(signal), signal.Direction, signal.Source))
		if len(lines) >= 18 {
			break
		}
	}
	if report.DocumentEvidence.Computed {
		lines = append(lines, fmt.Sprintf("document_evidence.status=%s usable=%d/%d coverage=%.1f/100 decision_relevant=%d/%d source=%s",
			report.DocumentEvidence.Status,
			report.DocumentEvidence.AnalysisUsableCount,
			report.DocumentEvidence.TotalDocuments,
			report.DocumentEvidence.CoverageScore,
			report.DocumentEvidence.DecisionRelevantUsableCount,
			report.DocumentEvidence.DecisionRelevantDocuments,
			report.DocumentEvidence.Source,
		))
		for _, count := range report.DocumentEvidence.TypeCounts {
			lines = append(lines, fmt.Sprintf("document_evidence.type.%s count=%d label=%s", count.Type, count.Count, count.Label))
			if len(lines) >= 24 {
				break
			}
		}
		for _, doc := range report.DocumentEvidence.KeyDocuments {
			lines = append(lines, fmt.Sprintf("document_evidence.key file=%s type=%s usable=%t quality=%.2f status=%s",
				doc.FileName,
				doc.DocumentType,
				doc.AnalysisUsable,
				doc.QualityScore,
				doc.ParseStatus,
			))
			if len(lines) >= 30 {
				break
			}
		}
	}
	ctx := RetrievalContext{
		Mode:         "ideal_rag_precomputed_context",
		ContextType:  "structured_signals_and_kap_pdf_evidence",
		ContextLines: lines,
	}
	if len(lines) == 0 {
		ctx.Warnings = append(ctx.Warnings, "precomputed_context_empty")
	}
	if report.GoldenIndicatorCoverage.F1 < 0.70 {
		ctx.Warnings = append(ctx.Warnings, "golden_indicator_context_coverage_limited")
	}
	if report.DocumentEvidence.Status == "missing" || report.DocumentEvidence.Status == "failed" {
		ctx.Warnings = append(ctx.Warnings, "kap_pdf_document_evidence_missing_or_unusable")
	}
	return ctx
}

func buildPointInTime(input Input, candles []ohlcv.Candle) PointInTimeReport {
	report := PointInTimeReport{Status: "passed", AsOf: formatDate(input.AsOf)}
	if len(candles) > 0 {
		latest := candles[len(candles)-1].Time
		report.MarketDataThrough = formatDate(latest)
		if !input.AsOf.IsZero() && latest.After(endOfDayUTC(input.AsOf)) {
			report.Status = "failed"
			report.Warnings = append(report.Warnings, "market_data_after_as_of")
		}
	}
	report.FundamentalPeriod = formatFundamentalPeriod(input.Professional.Valuation)
	for _, flag := range input.Professional.Valuation.Flags {
		if flag == "financial_publish_date_unverified" {
			report.Status = pointInTimeLimitedStatus(report.Status)
			report.Warnings = append(report.Warnings, "financial_publish_date_unverified")
			break
		}
	}
	if report.MarketDataThrough == "" {
		report.Status = pointInTimeLimitedStatus(report.Status)
		report.Warnings = append(report.Warnings, "market_data_timestamp_missing")
	}
	if report.FundamentalPeriod == "" {
		report.Status = pointInTimeLimitedStatus(report.Status)
		report.Warnings = append(report.Warnings, "fundamental_period_missing")
	}
	return report
}

func pointInTimeLimitedStatus(current string) string {
	if current == "failed" {
		return current
	}
	return "limited"
}

func hybridReadiness(tracks []ReasoningTrack, audit NumericalAudit) float64 {
	score := 0.0
	totalWeight := 0.0
	for _, track := range tracks {
		weight := 1.0
		if track.Type == "FT" {
			weight = 2.0
		}
		score += track.Coverage * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0
	}
	score = score / totalWeight * 100
	if audit.Status == "limited" {
		score *= 0.85
	}
	if audit.Status == "failed" {
		score *= 0.50
	}
	return round2(score)
}

func summary(report Report) string {
	fStatus, tStatus, dStatus, ftStatus := "missing", "missing", "missing", "missing"
	for _, track := range report.QuestionTaxonomy {
		switch track.Type {
		case "F":
			fStatus = track.Status
		case "T":
			tStatus = track.Status
		case "D":
			dStatus = track.Status
		case "FT":
			ftStatus = track.Status
		}
	}
	return fmt.Sprintf("FinTradeBench evidence coverage: F=%s, T=%s, D=%s, FT=%s, GI_F1=%.2f, DOC=%s %.1f/100, hybrid_readiness=%.1f/100.", fStatus, tStatus, dStatus, ftStatus, report.GoldenIndicatorCoverage.F1, report.DocumentEvidence.Status, report.DocumentEvidence.CoverageScore, report.HybridReadinessScore)
}

func valueSignal(name, displayName, family, formula string, value float64, unit string, available bool, direction string, source string) Signal {
	signal := Signal{
		Name:        name,
		DisplayName: displayName,
		Family:      family,
		Formula:     formula,
		Unit:        unit,
		Available:   available && isFinite(value),
		Direction:   direction,
		Source:      source,
	}
	if signal.Available {
		signal.Value = round6(value)
		signal.Strength = signalStrength(value, unit)
		signal.Evidence = []string{fmt.Sprintf("%s=%s", name, formatValue(signal))}
	} else {
		signal.MissingReason = "required source value unavailable"
	}
	return signal
}

func missingSignal(name, displayName, family, formula, reason string) Signal {
	return Signal{
		Name:          name,
		DisplayName:   displayName,
		Family:        family,
		Formula:       formula,
		Available:     false,
		MissingReason: reason,
	}
}

func dividendYieldSignal(pro professional.Report, lastClose float64) Signal {
	capital := pro.ValueInvesting.CapitalAllocation
	if capital.DividendDataAvailable && capital.PaidCapitalLatest > 0 && lastClose > 0 && len(pro.ValueInvesting.Years) > 0 {
		latest := pro.ValueInvesting.Years[len(pro.ValueInvesting.Years)-1]
		dividendPerShare := safeDiv(latest.DividendsPaid, capital.PaidCapitalLatest)
		if dividendPerShare > 0 {
			yield := dividendPerShare / lastClose
			return valueSignal("dividend_yield", "Dividend Yield", "fundamental", "dividends per share / price per share", yield, "ratio", true, payoutDirection(yield), "professional.value_investing.years.dividends_paid")
		}
	}
	return missingSignal("dividend_yield", "Dividend Yield", "fundamental", "dividends per share / price per share", "latest dividends per share is not available as a structured point-in-time value")
}

func coverage(required []string, generated []string) GoldenIndicatorCoverage {
	requiredSet := map[string]bool{}
	for _, name := range required {
		requiredSet[name] = true
	}
	generated = uniqueStrings(generated)
	sort.Strings(generated)
	missing := []string{}
	intersection := 0
	for _, name := range required {
		if contains(generated, name) {
			intersection++
		} else {
			missing = append(missing, name)
		}
	}
	extraGeneratedRequired := 0
	for _, name := range generated {
		if requiredSet[name] {
			extraGeneratedRequired++
		}
	}
	precision := safeDiv(float64(extraGeneratedRequired), float64(len(generated)))
	recall := safeDiv(float64(intersection), float64(len(required)))
	f1 := 0.0
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}
	return GoldenIndicatorCoverage{
		Required:  append([]string(nil), required...),
		Generated: generated,
		Missing:   missing,
		Precision: round4(precision),
		Recall:    round4(recall),
		F1:        round4(f1),
	}
}

func sortedCandles(candles []ohlcv.Candle) []ohlcv.Candle {
	out := append([]ohlcv.Candle(nil), candles...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

func oneDayReversal(candles []ohlcv.Candle) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}
	prev := candles[len(candles)-2].EffectiveClose()
	last := candles[len(candles)-1].EffectiveClose()
	if prev <= 0 {
		return 0, false
	}
	return last/prev - 1, true
}

func maxDailyReturn(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}
	start := maxInt(1, len(candles)-period)
	maxReturn := math.Inf(-1)
	for i := start; i < len(candles); i++ {
		prev := candles[i-1].EffectiveClose()
		curr := candles[i].EffectiveClose()
		if prev <= 0 {
			continue
		}
		maxReturn = math.Max(maxReturn, curr/prev-1)
	}
	if math.IsInf(maxReturn, -1) {
		return 0, false
	}
	return maxReturn, true
}

func cumulativeReturn(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) <= period {
		return 0, false
	}
	prev := candles[len(candles)-1-period].EffectiveClose()
	last := candles[len(candles)-1].EffectiveClose()
	if prev <= 0 {
		return 0, false
	}
	return last/prev - 1, true
}

func longTermMeanReversion(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, lastClose float64) (float64, bool) {
	mean := snapshot.SMA200
	if mean <= 0 && len(candles) >= 200 {
		sum := 0.0
		for _, candle := range candles[len(candles)-200:] {
			sum += candle.EffectiveClose()
		}
		mean = sum / 200
	}
	if mean <= 0 || lastClose <= 0 {
		return 0, false
	}
	return (mean - lastClose) / lastClose, true
}

func realizedVolatility(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) <= period {
		return 0, false
	}
	returns := []float64{}
	for i := len(candles) - period; i < len(candles); i++ {
		prev := candles[i-1].EffectiveClose()
		curr := candles[i].EffectiveClose()
		if prev > 0 && curr > 0 {
			returns = append(returns, math.Log(curr/prev))
		}
	}
	if len(returns) < 2 {
		return 0, false
	}
	return stddev(returns) * math.Sqrt(252), true
}

func maxDrawdown(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}
	start := maxInt(0, len(candles)-period)
	peak := 0.0
	maxDD := 0.0
	for _, candle := range candles[start:] {
		close := candle.EffectiveClose()
		if close <= 0 {
			continue
		}
		if close > peak {
			peak = close
		}
		if peak > 0 {
			dd := close/peak - 1
			if dd < maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD, peak > 0
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, value := range values {
		diff := value - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(values)-1))
}

func hasProfessionalFundamentals(pro professional.Report) bool {
	return pro.Valuation.MarketCap > 0 || pro.Valuation.TotalAssets > 0 || len(pro.Valuation.Ratios) > 0
}

func hasDocumentEvidence(pro professional.Report) bool {
	kap := pro.KAPPDFIngest
	return kap.Computed || kap.TotalDocuments > 0 || kap.SourcePDFCount > 0 || len(kap.ImportantDocuments) > 0
}

func availableSet(signals []Signal) map[string]bool {
	out := map[string]bool{}
	for _, signal := range signals {
		if signal.Available {
			out[signal.Name] = true
		}
	}
	return out
}

func maDirection(lastClose, ma float64) string {
	if lastClose <= 0 || ma <= 0 {
		return ""
	}
	if lastClose > ma {
		return "price_above_average"
	}
	if lastClose < ma {
		return "price_below_average"
	}
	return "neutral"
}

func macdDirection(snapshot ohlcv.IndicatorSnapshot) string {
	switch {
	case snapshot.MACDHistogram > 0 && snapshot.MACD >= snapshot.MACDSignal:
		return "bullish_momentum"
	case snapshot.MACDHistogram < 0 && snapshot.MACD <= snapshot.MACDSignal:
		return "bearish_momentum"
	case snapshot.MACDHistogram > 0:
		return "improving_momentum"
	case snapshot.MACDHistogram < 0:
		return "weakening_momentum"
	default:
		return "neutral"
	}
}

func rsiDirection(rsi float64) string {
	switch {
	case rsi >= 70:
		return "overbought_risk"
	case rsi <= 30 && rsi > 0:
		return "oversold_reversal_potential"
	case rsi > 55:
		return "bullish_momentum"
	case rsi < 45 && rsi > 0:
		return "bearish_momentum"
	default:
		return "neutral"
	}
}

func obvDirection(slope float64) string {
	switch {
	case slope > 0.01:
		return "accumulation"
	case slope < -0.01:
		return "distribution"
	default:
		return "neutral"
	}
}

func returnDirection(value, threshold float64) string {
	switch {
	case value > threshold:
		return "positive_momentum"
	case value < -threshold:
		return "negative_momentum"
	default:
		return "neutral"
	}
}

func volatilityShockDirection(value float64) string {
	switch {
	case value >= 0.05:
		return "positive_volatility_extreme"
	case value <= -0.05:
		return "negative_volatility_extreme"
	default:
		return "normal"
	}
}

func meanReversionDirection(value float64) string {
	switch {
	case value > 0.10:
		return "below_long_mean"
	case value < -0.10:
		return "above_long_mean"
	default:
		return "near_long_mean"
	}
}

func volatilityDirection(value float64) string {
	switch {
	case value >= 0.60:
		return "high_volatility"
	case value >= 0.30:
		return "elevated_volatility"
	default:
		return "normal_volatility"
	}
}

func drawdownDirection(value float64) string {
	switch {
	case value <= -0.20:
		return "deep_drawdown"
	case value <= -0.10:
		return "moderate_drawdown"
	default:
		return "contained_drawdown"
	}
}

func volumeDirection(value float64) string {
	switch {
	case value >= 1.5:
		return "high_volume_confirmation"
	case value <= 0.7:
		return "low_volume"
	default:
		return "normal_volume"
	}
}

func qualityRatioDirection(value, strong, weak float64) string {
	switch {
	case value >= strong:
		return "strong"
	case value <= weak:
		return "weak"
	default:
		return "moderate"
	}
}

func valuationYieldDirection(value float64) string {
	switch {
	case value >= 1:
		return "high_book_yield"
	case value > 0:
		return "positive_book_yield"
	default:
		return "unavailable"
	}
}

func earningsYieldDirection(value float64) string {
	switch {
	case value >= 0.10:
		return "high_yield"
	case value > 0:
		return "positive_yield"
	case value < 0:
		return "negative_yield"
	default:
		return "unavailable"
	}
}

func leverageDirection(value float64) string {
	switch {
	case value < 0:
		return "net_cash"
	case value >= 1:
		return "high_leverage"
	case value > 0:
		return "moderate_leverage"
	default:
		return "low_or_unavailable"
	}
}

func valueDirection(value float64) string {
	if value > 0 {
		return "available"
	}
	return "unavailable"
}

func payoutDirection(value float64) string {
	switch {
	case value <= 0:
		return "unavailable"
	case value <= 0.75:
		return "covered"
	default:
		return "high_payout"
	}
}

func documentCoverageDirection(score float64) string {
	switch {
	case score >= 80:
		return "strong_document_coverage"
	case score >= 60:
		return "usable_document_coverage"
	case score > 0:
		return "limited_document_coverage"
	default:
		return "missing_document_coverage"
	}
}

func documentReviewDirection(ratio float64) string {
	switch {
	case ratio <= 0:
		return "no_review_queue"
	case ratio <= 0.10:
		return "low_review_queue"
	case ratio <= 0.35:
		return "moderate_review_queue"
	default:
		return "high_review_queue"
	}
}

func documentRejectedDirection(ratio float64) string {
	switch {
	case ratio <= 0:
		return "no_rejected_documents"
	case ratio <= 0.05:
		return "low_rejected_documents"
	case ratio <= 0.20:
		return "moderate_rejected_documents"
	default:
		return "high_rejected_documents"
	}
}

func signalStrength(value float64, unit string) float64 {
	switch unit {
	case "pct", "ratio":
		return round4(math.Min(1, math.Abs(value)))
	case "score":
		return round4(math.Min(1, math.Abs(value-50)/50))
	default:
		if value == 0 {
			return 0
		}
		return 1
	}
}

func formatFundamentalPeriod(v professional.ValuationAnalysis) string {
	if v.LatestYear == 0 {
		return ""
	}
	if strings.TrimSpace(v.LatestQuarter) == "" {
		return fmt.Sprintf("%d", v.LatestYear)
	}
	return fmt.Sprintf("%d-%s", v.LatestYear, strings.TrimSpace(v.LatestQuarter))
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func formatValue(signal Signal) string {
	switch signal.Unit {
	case "pct", "ratio":
		return fmt.Sprintf("%.4f", signal.Value)
	case "score":
		return fmt.Sprintf("%.2f", signal.Value)
	default:
		return fmt.Sprintf("%.4f", signal.Value)
	}
}

func safeDiv(num, den float64) float64 {
	if den == 0 || math.IsNaN(num) || math.IsNaN(den) || math.IsInf(num, 0) || math.IsInf(den, 0) {
		return 0
	}
	return num / den
}

func percentInt(num, den int) float64 {
	if den <= 0 {
		return 0
	}
	return 100 * safeDiv(float64(num), float64(den))
}

func inversePositive(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return 1 / value
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func ratioAvailable(ratios map[string]float64, key string) bool {
	_, ok := ratios[key]
	return ok
}

func endOfDayUTC(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func round6(value float64) float64 {
	return math.Round(value*1000000) / 1000000
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
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
