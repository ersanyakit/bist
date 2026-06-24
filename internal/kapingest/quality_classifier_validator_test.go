package kapingest

import "testing"

func TestTextQualityLowTextScore(t *testing.T) {
	got := AssessTextQuality("abc")
	if got.Score >= 0.35 {
		t.Fatalf("score = %.4f, want low", got.Score)
	}
	if len(got.Warnings) == 0 || got.Warnings[0] != "low_text_quality_possible_scanned_pdf" {
		t.Fatalf("warnings = %#v", got.Warnings)
	}
}

func TestTextQualityShortStructuredGovernanceDocumentIsNotScannedWarning(t *testing.T) {
	text := "BAĞIMSIZ ÜYE ADAY LİSTESİ\nYönetim Kurulu bağımsız üye adayı: Ayşe Yılmaz"
	docType := ClassifyDocument(text, "BAĞIMSIZ ÜYE ADAY LİSTESİ.pdf")
	if docType != DocumentCorporateGovernance {
		t.Fatalf("class = %q, want corporate_governance", docType)
	}
	got := FinalizeTextQualityForDocument(AssessTextQuality(text), text, "BAĞIMSIZ ÜYE ADAY LİSTESİ.pdf", docType)
	if got.Score < 0.35 {
		t.Fatalf("score = %.4f, want non-low after structured document adjustment", got.Score)
	}
	if containsRawWarning(got.Warnings, "low_text_quality_possible_scanned_pdf") {
		t.Fatalf("unexpected scanned warning: %#v", got.Warnings)
	}
	if !containsRawWarning(got.Warnings, "short_structured_non_financial_document") {
		t.Fatalf("expected short structured warning: %#v", got.Warnings)
	}
}

func TestQualityGateRejectsLowQualityTextForAnalysis(t *testing.T) {
	quality := AssessTextQuality("abc")
	gate := EvaluateTextQualityGate(quality, quality.Warnings, DocumentFinancialStatement)
	if gate.Status != ParseStatusRejected || gate.AnalysisUsable || !gate.HumanReviewRequired {
		t.Fatalf("unexpected quality gate: %+v", gate)
	}
	if !containsRawWarning(QualityGateWarnings(gate), "analysis_blocked_by_quality_gate") {
		t.Fatalf("expected analysis-block warning for rejected parse")
	}
}

func TestQualityGateRequiresReviewBelowTrustedThreshold(t *testing.T) {
	quality := QualityResult{Score: 0.50, TextLength: 900, NumericDensity: 0.04}
	gate := EvaluateTextQualityGate(quality, nil, DocumentFinancialStatement)
	if gate.Status != ParseStatusReviewRequired || gate.AnalysisUsable || !gate.HumanReviewRequired {
		t.Fatalf("unexpected quality gate: %+v", gate)
	}
}

func TestStructuredRescueDetectsCoordinateTableEvidence(t *testing.T) {
	doc := RawDocument{
		FilePath:          "data/equities/TEST/kap/attachments/2026/scan.pdf",
		SHA256:            "sha-rescue-quality",
		Ticker:            "TEST",
		FileName:          "TEST Finansal Rapor.pdf",
		ExtractionMethod:  "pdftotext+tsv",
		DocumentTypeGuess: DocumentFinancialStatement,
		QualityScore:      0.55,
		TextLength:        200,
		Warnings:          []string{"coordinate_tsv_text_appended"},
		Text:              "bozuk metin\n###COORDINATE_TABLE_TEXT###\nFinansal Durum Tablosu\t31.03.2026\nNakit ve Nakit Benzerleri\t1.500.000 TL\nNet dönem karı\t250.000 TL",
	}
	if !RawDocumentStructuredRescueUsable(doc) {
		t.Fatalf("coordinate table evidence should be structured-rescue usable")
	}
	if !RawDocumentStructuredAIResolved(doc) {
		t.Fatalf("coordinate table evidence should be AI-resolved")
	}
	rescued := RawDocumentForStructuredRescue(doc)
	if !containsRawWarning(rescued.Warnings, lowQualityStructuredRescueWarning) || !rescued.AnalysisUsable || !rescued.AIResolved || rescued.HumanReviewRequired {
		t.Fatalf("unexpected rescued doc state: %+v", rescued)
	}
}

func TestClassifierRecognizesActivityReport(t *testing.T) {
	text := "ARA DÖNEM FAALİYET RAPORU 01.01.2026 - 31.03.2026 Yönetim Kurulu"
	if got := ClassifyDocument(text, "rapor.pdf"); got != DocumentInterimActivityReport {
		t.Fatalf("class = %q", got)
	}
}

func TestClassifierRecognizesFinancialStatement(t *testing.T) {
	text := "Finansal Durum Tablosu\nKar veya Zarar Tablosu\nVarlıklar Özkaynaklar Hasılat"
	if got := ClassifyDocument(text, "finansal.pdf"); got != DocumentFinancialStatement {
		t.Fatalf("class = %q", got)
	}
}

func TestClassifierRecognizesRegisteredCapitalCeilingAmendment(t *testing.T) {
	text := "Kayıtlı sermaye tavanı artışı SPK izni ve tadil metni"
	if got := ClassifyDocument(text, "KAYITLI SERMAYE TAVANI ARTIŞI SPK İZNİ VE TADİL METNİ.pdf"); got != DocumentCapitalIncrease {
		t.Fatalf("class = %q", got)
	}
}

func TestKAPEventValidatorEnums(t *testing.T) {
	event := KAPEvent{
		FilePath:         "x.pdf",
		SHA256:           "abc",
		DocumentClass:    "",
		FinancialProfile: "",
		Impact: Impact{
			Fundamental:    "bullish",
			ShortTermPrice: "uncertain",
			LongTerm:       "uncertain",
			Confidence:     0.5,
		},
	}
	if _, err := ValidateKAPEvent(&event); err == nil {
		t.Fatalf("expected invalid impact enum")
	}
	event.Impact.Fundamental = "uncertain"
	warnings, err := ValidateKAPEvent(&event)
	if err != nil {
		t.Fatalf("ValidateKAPEvent() error = %v", err)
	}
	if event.DocumentClass != DocumentUnknown || event.FinancialProfile != "unknown" {
		t.Fatalf("normalized event = %+v", event)
	}
	if len(warnings) == 0 || warnings[0] != "ticker_missing" {
		t.Fatalf("warnings = %#v", warnings)
	}
}
