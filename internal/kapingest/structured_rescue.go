package kapingest

import (
	"sort"
	"strings"

	"hissebot/internal/util"
)

const (
	lowQualityStructuredRescueWarning = "low_quality_structured_rescue"
	structuredRescueReviewWarning     = "structured_rescue_extracted_review_required"
	structuredRescueAIResolvedWarning = "structured_rescue_ai_resolved"
	aiResolvedByStructuredEvidence    = "ai_resolved_by_structured_evidence"
)

type StructuredRescueAssessment struct {
	Usable        bool
	AIResolved    bool
	Confidence    float64
	EvidenceScore int
	UsefulLines   int
	TableLikeRows int
	NumericRows   int
	FinancialRows int
	AssetRows     int
	OwnershipRows int
	CorporateRows int
	Reasons       []string
}

var structuredRescueMarkers = []string{
	"###COORDINATE_TABLE_TEXT###",
	"###OCR_AUDIT_TEXT###",
}

func RawDocumentStructuredRescueUsable(doc RawDocument) bool {
	return AssessStructuredRescue(doc).Usable
}

func RawDocumentStructuredAIResolved(doc RawDocument) bool {
	return AssessStructuredRescue(doc).AIResolved
}

func RawDocumentStructuredExtractionUsable(doc RawDocument) bool {
	return RawDocumentAnalysisUsable(doc) || RawDocumentStructuredRescueUsable(doc)
}

func RawDocumentForStructuredRescue(doc RawDocument) RawDocument {
	assessment := AssessStructuredRescue(doc)
	doc.Text = StructuredRescueText(doc.Text)
	doc.AIResolved = assessment.AIResolved
	doc.AIConfidence = assessment.Confidence
	if assessment.AIResolved {
		doc.AnalysisUsable = true
		doc.HumanReviewRequired = false
		doc.Warnings = dedupeStrings(append(doc.Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
	} else {
		doc.AnalysisUsable = false
		doc.HumanReviewRequired = true
		doc.Warnings = dedupeStrings(append(doc.Warnings, lowQualityStructuredRescueWarning, structuredRescueReviewWarning))
	}
	gate := QualityGateForRawDocument(doc)
	if assessment.AIResolved {
		gate.Status = ParseStatusAIResolved
		gate.AnalysisUsable = true
		gate.HumanReviewRequired = false
		gate.AIResolved = true
		gate.AIConfidence = assessment.Confidence
		gate.Reasons = dedupeStrings(append(gate.Reasons, lowQualityStructuredRescueWarning, aiResolvedByStructuredEvidence))
	} else {
		if strings.TrimSpace(gate.Status) == "" {
			gate.Status = ParseStatusReviewRequired
		}
		gate.AnalysisUsable = false
		gate.HumanReviewRequired = true
		gate.Reasons = dedupeStrings(append(gate.Reasons, lowQualityStructuredRescueWarning))
	}
	doc.ParseStatus = gate.Status
	doc.QualityGate = gate
	return doc
}

func RawDocumentAfterAIAdjudication(doc RawDocument) RawDocument {
	assessment := AssessStructuredRescue(doc)
	if !assessment.AIResolved {
		return doc
	}
	doc.AIResolved = true
	doc.AIConfidence = assessment.Confidence
	doc.AnalysisUsable = true
	doc.HumanReviewRequired = false
	doc.Warnings = dedupeStrings(append(doc.Warnings, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence))
	gate := QualityGateForRawDocument(doc)
	gate.Status = ParseStatusAIResolved
	gate.AnalysisUsable = true
	gate.HumanReviewRequired = false
	gate.AIResolved = true
	gate.AIConfidence = assessment.Confidence
	gate.Reasons = dedupeStrings(append(gate.Reasons, lowQualityStructuredRescueWarning, aiResolvedByStructuredEvidence))
	doc.ParseStatus = gate.Status
	doc.QualityGate = gate
	return doc
}

func StructuredRescueText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	sections := markerSections(text, structuredRescueMarkers)
	if len(sections) == 0 {
		return structuredRescueRelevantLines(text)
	}
	return structuredRescueRelevantLines(strings.Join(sections, "\n"))
}

func rawDocumentHasStructuredRescueSignal(doc RawDocument) bool {
	method := strings.ToLower(strings.TrimSpace(doc.ExtractionMethod))
	if strings.Contains(method, "tsv") || strings.Contains(method, "ocr") {
		return true
	}
	for _, marker := range structuredRescueMarkers {
		if strings.Contains(doc.Text, marker) {
			return true
		}
	}
	for _, warning := range doc.Warnings {
		warning = strings.TrimSpace(warning)
		switch warning {
		case "coordinate_tsv_text_appended",
			"ocr_audit_text_appended",
			"pdftotext_low_quality_ocr_used",
			"pdftotext_failed_ocr_used",
			"ocr_fallback_used",
			lowQualityStructuredRescueWarning:
			return true
		}
		if strings.HasPrefix(warning, "ocr_timeout_partial") || strings.HasPrefix(warning, "coordinate_tsv_") {
			return true
		}
	}
	return false
}

func markerSections(text string, markers []string) []string {
	type hit struct {
		pos    int
		marker string
	}
	hits := []hit{}
	for _, marker := range markers {
		start := 0
		for {
			idx := strings.Index(text[start:], marker)
			if idx < 0 {
				break
			}
			hits = append(hits, hit{pos: start + idx, marker: marker})
			start += idx + len(marker)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].pos < hits[j].pos
	})
	sections := []string{}
	for i, current := range hits {
		start := current.pos + len(current.marker)
		end := len(text)
		if i+1 < len(hits) {
			end = hits[i+1].pos
		}
		section := strings.TrimSpace(text[start:end])
		if section != "" {
			sections = append(sections, section)
		}
	}
	return sections
}

func structuredRescueRelevantLines(text string) string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "###") {
			continue
		}
		if structuredRescueLineUseful(line) {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func structuredRescueEvidenceScore(text string) int {
	return assessStructuredRescueText(text, 0, false).EvidenceScore
}

func AssessStructuredRescue(doc RawDocument) StructuredRescueAssessment {
	if strings.TrimSpace(doc.Text) == "" {
		return StructuredRescueAssessment{Reasons: []string{"empty_text"}}
	}
	if RawDocumentAnalysisUsable(doc) && !rawDocumentAlreadyAIResolved(doc) {
		return StructuredRescueAssessment{Reasons: []string{"already_analysis_usable"}}
	}
	if !rawDocumentHasStructuredRescueSignal(doc) {
		return StructuredRescueAssessment{Reasons: []string{"structured_rescue_signal_missing"}}
	}
	assessment := assessStructuredRescueText(StructuredRescueText(doc.Text), doc.QualityScore, strings.TrimSpace(doc.SHA256) != "")
	if !assessment.Usable {
		assessment.Reasons = dedupeStrings(append(assessment.Reasons, "structured_evidence_too_weak"))
	}
	return assessment
}

func rawDocumentAlreadyAIResolved(doc RawDocument) bool {
	return doc.AIResolved || doc.QualityGate.AIResolved || strings.EqualFold(strings.TrimSpace(doc.ParseStatus), ParseStatusAIResolved)
}

func assessStructuredRescueText(text string, qualityScore float64, hasSHA bool) StructuredRescueAssessment {
	assessment := StructuredRescueAssessment{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !structuredRescueLineUseful(line) {
			continue
		}
		assessment.UsefulLines++
		cells := splitAssetCells(line)
		if len(cells) >= 2 {
			assessment.TableLikeRows++
		}
		if hasASCIIDigit(line) || len(extractMoneyAmounts(line)) > 0 || extractPercentValue(line) != nil {
			assessment.NumericRows++
		}
		slug := util.SlugTR(line)
		if financialLineLooksRelevant(line) || classifyFinancialTableType(line) != "" {
			assessment.FinancialRows++
		}
		if assetLineLooksRelevantForIndex(line) || containsAssetCue(line) {
			assessment.AssetRows++
		}
		if ownershipLineLooksRelevant(line) {
			assessment.OwnershipRows++
		}
		if documentCorporateEventType(line) != "" ||
			slugContains(slug, "sermaye") || slugContains(slug, "kar payi") {
			assessment.CorporateRows++
		}
	}
	assessment.EvidenceScore = assessment.UsefulLines
	if assessment.FinancialRows > 0 {
		assessment.EvidenceScore++
	}
	if assessment.AssetRows > 0 {
		assessment.EvidenceScore++
	}
	if assessment.OwnershipRows > 0 || assessment.CorporateRows > 0 {
		assessment.EvidenceScore++
	}
	assessment.Usable = assessment.EvidenceScore >= 2
	assessment.Confidence = structuredRescueAIConfidence(assessment, qualityScore, hasSHA)
	assessment.AIResolved = structuredRescueAIResolvable(assessment)
	assessment.Reasons = structuredRescueAssessmentReasons(assessment)
	return assessment
}

func structuredRescueAIConfidence(assessment StructuredRescueAssessment, qualityScore float64, hasSHA bool) float64 {
	score := 0.46
	score += float64(minInt(assessment.UsefulLines, 8)) * 0.035
	score += float64(minInt(assessment.TableLikeRows, 6)) * 0.025
	score += float64(minInt(assessment.NumericRows, 6)) * 0.030
	if assessment.FinancialRows > 0 {
		score += 0.08
	}
	if assessment.AssetRows > 0 {
		score += 0.08
	}
	if assessment.OwnershipRows > 0 {
		score += 0.05
	}
	if assessment.CorporateRows > 0 {
		score += 0.05
	}
	if qualityScore >= 0.50 {
		score += 0.04
	}
	if hasSHA {
		score += 0.03
	}
	return clampAsset(score, 0, 0.92)
}

func structuredRescueAIResolvable(assessment StructuredRescueAssessment) bool {
	if !assessment.Usable || assessment.TableLikeRows == 0 || assessment.NumericRows == 0 {
		return false
	}
	if assessment.FinancialRows > 0 && assessment.TableLikeRows >= 2 && assessment.NumericRows >= 2 && assessment.Confidence >= 0.76 {
		return true
	}
	if assessment.AssetRows > 0 && assessment.TableLikeRows >= 1 && assessment.NumericRows >= 1 && assessment.Confidence >= 0.68 {
		return true
	}
	if assessment.OwnershipRows > 0 && assessment.TableLikeRows >= 1 && assessment.NumericRows >= 1 && assessment.Confidence >= 0.70 {
		return true
	}
	if assessment.CorporateRows > 0 && assessment.TableLikeRows >= 1 && assessment.NumericRows >= 1 && assessment.Confidence >= 0.72 {
		return true
	}
	return false
}

func structuredRescueAssessmentReasons(assessment StructuredRescueAssessment) []string {
	reasons := []string{}
	if assessment.Usable {
		reasons = append(reasons, "structured_evidence_detected")
	}
	if assessment.AIResolved {
		reasons = append(reasons, aiResolvedByStructuredEvidence)
	}
	if assessment.TableLikeRows > 0 {
		reasons = append(reasons, "table_like_rows_detected")
	}
	if assessment.NumericRows > 0 {
		reasons = append(reasons, "numeric_rows_detected")
	}
	return dedupeStrings(reasons)
}

func structuredRescueLineUseful(line string) bool {
	line = normalizeAssetLine(line)
	if line == "" || lineTooLongForStructuredFact(line) {
		return false
	}
	cells := splitAssetCells(line)
	tableLike := len(cells) >= 2
	hasNumeric := hasASCIIDigit(line) || len(extractMoneyAmounts(line)) > 0 || extractPercentValue(line) != nil
	if classifyFinancialTableType(line) != "" && (tableLike || hasNumeric) {
		return true
	}
	if !hasNumeric {
		return false
	}
	if financialLineLooksRelevant(line) ||
		assetLineLooksRelevantForIndex(line) ||
		ownershipLineLooksRelevant(line) ||
		documentCorporateEventType(line) != "" ||
		corporateEventLineLooksActionable(line, documentCorporateEventType(line)) {
		return true
	}
	slug := util.SlugTR(line)
	return tableLike && anySlugContains(slug, []string{
		"toplam varlik", "toplam kaynak", "ozkaynak", "hasilat", "net donem kari",
		"portfoy degeri", "kdv haric", "kdv dahil", "ekspertiz degeri",
		"ada", "parsel", "sermaye", "pay orani", "temettu", "kar payi",
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
