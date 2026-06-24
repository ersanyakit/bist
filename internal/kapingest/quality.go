package kapingest

import (
	"math"
	"strings"
	"unicode"

	"hissebot/internal/util"
)

const (
	ParseStatusTrusted        = "trusted"
	ParseStatusAIResolved     = "ai_resolved"
	ParseStatusReviewRequired = "review_required"
	ParseStatusRejected       = "rejected"

	RejectedTextQualityThreshold = 0.35
	TrustedTextQualityThreshold  = 0.62
)

var financialQualityKeywords = []string{
	"faaliyet raporu",
	"finansal durum tablosu",
	"kar veya zarar",
	"nakit akis",
	"ozkaynaklar",
	"hasilat",
	"net donem kari",
	"varliklar",
	"yukumlulukler",
	"gayrimenkul",
	"degerleme",
	"kar payi",
	"sermaye artirimi",
}

var kapQualityKeywords = []string{
	"kamuyu aydinlatma platformu",
	"kap",
	"yonetim kurulu",
	"genel kurul",
	"bagimsiz denetim",
	"dipnot",
	"portfoy",
}

func AssessTextQuality(text string) QualityResult {
	length := len([]rune(text))
	result := QualityResult{TextLength: length}
	if strings.TrimSpace(text) == "" {
		result.Score = 0
		result.Warnings = append(result.Warnings, "empty_text")
		result.Warnings = append(result.Warnings, "low_text_quality_possible_scanned_pdf")
		return result
	}
	digits, broken, printable := 0, 0, 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			digits++
		}
		if r == '�' || (unicode.IsControl(r) && r != '\n' && r != '\t') {
			broken++
		}
		if !unicode.IsSpace(r) {
			printable++
		}
	}
	result.NumericDensity = safeRatio(float64(digits), float64(maxInt(length, 1)))
	result.BrokenRatio = safeRatio(float64(broken), float64(maxInt(length, 1)))

	lengthScore := clamp01(float64(length) / 2500.0)
	if length > 500 {
		lengthScore = math.Max(lengthScore, 0.45)
	}
	if length > 5000 {
		lengthScore = 1
	}
	numericScore := 0.0
	switch {
	case result.NumericDensity >= 0.02 && result.NumericDensity <= 0.30:
		numericScore = 1
	case result.NumericDensity > 0:
		numericScore = 0.45
	}
	financialScore := keywordScore(text, financialQualityKeywords)
	kapScore := keywordScore(text, kapQualityKeywords)
	printableScore := clamp01(float64(printable) / float64(maxInt(length, 1)))

	score := 0.34*lengthScore + 0.16*numericScore + 0.30*financialScore + 0.12*kapScore + 0.08*printableScore
	score -= math.Min(0.35, result.BrokenRatio*4)
	result.Score = clamp01(score)
	if result.Score < RejectedTextQualityThreshold {
		result.Warnings = append(result.Warnings, "low_text_quality_possible_scanned_pdf")
	}
	return result
}

func FinalizeTextQualityForDocument(result QualityResult, text, fileName, docType string) QualityResult {
	if result.TextLength == 0 || result.Score >= RejectedTextQualityThreshold || result.BrokenRatio >= 0.02 {
		return result
	}
	if isShortStructuredNonFinancialDocument(text, fileName, docType) {
		result.Score = math.Max(result.Score, 0.45)
		result.Warnings = removeQualityWarning(result.Warnings, "low_text_quality_possible_scanned_pdf")
		result.Warnings = append(result.Warnings, "short_structured_non_financial_document")
	}
	return result
}

func EvaluateTextQualityGate(result QualityResult, warnings []string, docType string) QualityGate {
	reasons := []string{}
	if result.TextLength == 0 {
		reasons = append(reasons, "empty_text")
	}
	if result.BrokenRatio >= 0.02 {
		reasons = append(reasons, "broken_text_ratio_high")
	}
	if result.Score < RejectedTextQualityThreshold || containsRawWarning(warnings, "low_text_quality_possible_scanned_pdf") {
		reasons = append(reasons, "text_quality_below_rejected_threshold")
	}
	if warningHasPrefix(warnings, "ocr_fallback_failed") {
		reasons = append(reasons, "ocr_fallback_failed")
	}
	if warningHasPrefix(warnings, "ocr_timeout_partial") {
		reasons = append(reasons, "ocr_timeout_partial")
	}
	if containsRawWarning(warnings, "ocr_audit_required_but_disabled") {
		reasons = append(reasons, "ocr_audit_required_but_disabled")
	}
	if containsRawWarning(warnings, "invalid_utf8_replaced") || containsRawWarning(warnings, "ocr_invalid_utf8_replaced") {
		reasons = append(reasons, "invalid_utf8_replaced")
	}

	gate := QualityGate{
		TrustedThreshold:  TrustedTextQualityThreshold,
		RejectedThreshold: RejectedTextQualityThreshold,
	}
	switch {
	case len(reasons) > 0:
		gate.Status = ParseStatusRejected
		gate.AnalysisUsable = false
		gate.HumanReviewRequired = true
		gate.Reasons = dedupeStrings(reasons)
	case result.Score < TrustedTextQualityThreshold:
		gate.Status = ParseStatusReviewRequired
		gate.AnalysisUsable = false
		gate.HumanReviewRequired = true
		gate.Reasons = []string{"text_quality_below_trusted_threshold"}
		if containsRawWarning(warnings, "short_structured_non_financial_document") {
			gate.Reasons = append(gate.Reasons, "short_structured_non_financial_document")
		}
	case documentTypeNeedsStrictParse(docType) && result.NumericDensity == 0:
		gate.Status = ParseStatusReviewRequired
		gate.AnalysisUsable = false
		gate.HumanReviewRequired = true
		gate.Reasons = []string{"financial_document_without_numeric_content"}
	default:
		gate.Status = ParseStatusTrusted
		gate.AnalysisUsable = true
		gate.HumanReviewRequired = false
	}
	gate.Reasons = dedupeStrings(gate.Reasons)
	return gate
}

func QualityGateForRawDocument(doc RawDocument) QualityGate {
	if strings.TrimSpace(doc.QualityGate.Status) != "" {
		gate := doc.QualityGate
		if gate.TrustedThreshold == 0 {
			gate.TrustedThreshold = TrustedTextQualityThreshold
		}
		if gate.RejectedThreshold == 0 {
			gate.RejectedThreshold = RejectedTextQualityThreshold
		}
		return gate
	}
	quality := QualityResult{
		Score:      doc.QualityScore,
		TextLength: doc.TextLength,
	}
	if strings.TrimSpace(doc.Text) != "" {
		assessed := AssessTextQuality(doc.Text)
		assessed = FinalizeTextQualityForDocument(assessed, doc.Text, doc.FileName, doc.DocumentTypeGuess)
		if quality.TextLength == 0 {
			quality.TextLength = assessed.TextLength
		}
		quality.NumericDensity = assessed.NumericDensity
		quality.BrokenRatio = assessed.BrokenRatio
		if quality.Score <= 0 {
			quality.Score = assessed.Score
		}
	}
	return EvaluateTextQualityGate(quality, doc.Warnings, doc.DocumentTypeGuess)
}

func RawDocumentAnalysisUsable(doc RawDocument) bool {
	return QualityGateForRawDocument(doc).AnalysisUsable
}

func RawDocumentNeedsHumanReview(doc RawDocument) bool {
	return QualityGateForRawDocument(doc).HumanReviewRequired
}

func QualityGateWarnings(gate QualityGate) []string {
	warnings := []string{}
	if gate.Status == ParseStatusTrusted || gate.Status == ParseStatusAIResolved {
		return warnings
	}
	if gate.Status == ParseStatusRejected {
		warnings = append(warnings, "parse_rejected_by_quality_gate")
	} else {
		warnings = append(warnings, "parse_review_required_by_quality_gate")
	}
	if !gate.AnalysisUsable {
		warnings = append(warnings, "analysis_blocked_by_quality_gate")
	}
	return warnings
}

func documentTypeNeedsStrictParse(docType string) bool {
	switch NormalizeDocumentType(docType) {
	case DocumentFinancialStatement, DocumentAnnualReport, DocumentActivityReport, DocumentInterimActivityReport, DocumentIndependentAuditReport, DocumentValuationReport:
		return true
	default:
		return false
	}
}

func warningHasPrefix(warnings []string, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	for _, warning := range warnings {
		if strings.HasPrefix(strings.TrimSpace(warning), prefix) {
			return true
		}
	}
	return false
}

func isShortStructuredNonFinancialDocument(text, fileName, docType string) bool {
	if len([]rune(text)) > 1200 {
		return false
	}
	switch docType {
	case DocumentCorporateGovernance, DocumentBoardDecision, DocumentGeneralAssembly, DocumentCapitalIncrease, DocumentMaterialEvent:
	default:
		return false
	}
	slug := util.SlugTR(fileName + " " + text)
	signals := []string{
		"bagimsizuye", "bagimsizaday", "bagimsizyonetimkurulu",
		"kayitlisermayetavani", "tadilmetni", "spkizni",
		"genelkurul", "yonetimkurulu",
	}
	for _, signal := range signals {
		if strings.Contains(slug, util.SlugTR(signal)) {
			return true
		}
	}
	return false
}

func removeQualityWarning(warnings []string, warning string) []string {
	out := warnings[:0]
	for _, item := range warnings {
		if item == warning {
			continue
		}
		out = append(out, item)
	}
	return out
}

func keywordScore(text string, keywords []string) float64 {
	slug := util.SlugTR(text)
	if slug == "" {
		return 0
	}
	count := 0
	for _, keyword := range keywords {
		if strings.Contains(slug, util.SlugTR(keyword)) {
			count++
		}
	}
	return clamp01(float64(count) / 5.0)
}

func safeRatio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
