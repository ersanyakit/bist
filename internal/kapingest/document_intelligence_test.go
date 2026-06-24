package kapingest

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractDocumentIntelligenceGroupsFinancialManagementOwnershipAndEvents(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "ALGYO", "kap", "attachments", "2026", "sample.pdf"),
		SHA256:            "sha-intel-1",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 31.03.2026 Faaliyet Raporu.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: DocumentAnnualReport,
		QualityScore:      0.91,
		Text: `FİNANSAL DURUM TABLOSU
Varlıklar                     12.500.000 TL
Yatırım Amaçlı Gayrimenkuller 8.750.000 TL
Özkaynaklar                   7.250.000 TL

Yönetim Kurulu Başkanı        Ahmet Yılmaz
Genel Müdür                   Ayşe Demir

Ortaklık Yapısı
Pay sahibi                    Sermaye Payı       Pay Oranı
Alarko Holding A.Ş.           1.250.000 TL       %49,00

Kar payı dağıtımı genel kurul onayına sunulacaktır.`,
		TextLength: 420,
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-15T00:00:00Z")
	if len(got.FinancialFacts) < 3 {
		t.Fatalf("expected financial facts, got %+v", got.FinancialFacts)
	}
	if len(got.FinancialTables) == 0 {
		t.Fatalf("expected financial tables")
	}
	if len(got.People) < 2 {
		t.Fatalf("expected people, got %+v", got.People)
	}
	if len(got.OwnershipFacts) == 0 {
		t.Fatalf("expected ownership facts")
	}
	if len(got.CorporateEvents) == 0 {
		t.Fatalf("expected corporate event")
	}
	if got.IndexDoc.FactCount == 0 || got.IndexDoc.FinancialFactCount == 0 || got.IndexDoc.FinancialTableCount == 0 {
		t.Fatalf("index counts not populated: %+v", got.IndexDoc)
	}
	for _, fact := range got.FinancialFacts {
		if fact.Source.Snippet == "" || fact.SourceFile == "" || fact.SHA256 == "" {
			t.Fatalf("financial fact missing source evidence: %+v", fact)
		}
		if fact.Certification.Status == "" {
			t.Fatalf("financial fact missing certification: %+v", fact)
		}
		if !fact.ReviewRequired || fact.Certification.AnalysisUsable {
			t.Fatalf("financial fact without full normalization must stay analysis-unusable: %+v", fact)
		}
		if !containsTestString(fact.Certification.Reasons, "consolidation_scope_missing") || !containsTestString(fact.Certification.Reasons, "audit_status_missing") {
			t.Fatalf("financial fact certification should require institutional normalization fields: %+v", fact.Certification)
		}
	}
}

func TestFinancialLineCatalogRecognizesSPKFormatGuideRows(t *testing.T) {
	rows := []struct {
		line       string
		normalized string
		statement  string
	}{
		{"Peşin Ödenmiş Giderler 125.000 TL", "prepaid_expenses", "balance_sheet"},
		{"Ana Ortaklık Payları 5.539.312 TL", "parent_net_income", "income_statement"},
		{"İşletme Faaliyetlerinden Kaynaklanan Nakit Akışları 12.500.000 TL", "operating_cash_flow", "cash_flow_statement"},
	}
	for _, row := range rows {
		if !financialLineLooksRelevant(row.line) {
			t.Fatalf("expected relevant financial line: %s", row.line)
		}
		item := financialLineItem(row.line, splitAssetCells(row.line))
		got := normalizeFinancialLineItem(item)
		if got != row.normalized {
			t.Fatalf("normalized %q = %q, want %q", row.line, got, row.normalized)
		}
		if statement := inferDocumentStatementType(got); statement != row.statement {
			t.Fatalf("statement %q = %q, want %q", row.line, statement, row.statement)
		}
	}
}

func TestExtractDocumentIntelligenceUsesFinancialSectionContext(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "TEST", "kap", "attachments", "2026", "sample.pdf"),
		SHA256:            "sha-spk-context",
		Ticker:            "TEST",
		FileName:          "TEST 31.03.2026 Finansal Rapor.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: DocumentAnnualReport,
		QualityScore:      0.95,
		Text: `FİNANSAL DURUM TABLOSU
Dönen Varlıklar
Ticari Alacaklar 100.000 TL
Duran Varlıklar
Ticari Alacaklar 250.000 TL
Kısa Vadeli Yükümlülükler
Ticari Borçlar 75.000 TL
Uzun Vadeli Yükümlülükler
Ticari Borçlar 125.000 TL`,
		TextLength: 260,
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-17T12:00:00Z")
	normalized := map[string]bool{}
	for _, fact := range got.FinancialFacts {
		normalized[fact.LineItemNormalized] = true
	}
	for _, want := range []string{
		"short_term_trade_receivables",
		"long_term_trade_receivables",
		"short_term_trade_payables",
		"long_term_trade_payables",
	} {
		if !normalized[want] {
			t.Fatalf("missing contextual financial fact %q in %+v", want, got.FinancialFacts)
		}
	}
}

func TestFinancialLineLooksRelevantRejectsSPKNoteIndexRows(t *testing.T) {
	line := "31   Hasılat                                                             TMS 18, vd."
	if financialLineLooksRelevant(line) {
		t.Fatalf("note index row should not be extracted as revenue fact")
	}
}

func TestFinancialFactsFromLineMarksUnresolvedPriorPeriodColumnReviewRequired(t *testing.T) {
	doc := RawDocument{
		FilePath:         filepath.Join("data", "equities", "TEST", "kap", "attachments", "2026", "sample.pdf"),
		SHA256:           "sha-period-columns",
		Ticker:           "TEST",
		FileName:         "TEST 31.03.2026 Finansal Rapor.pdf",
		ExtractionMethod: "pdftotext",
		QualityScore:     0.95,
	}
	period := "2026/03"
	docDate := "2026-05-01"
	context := documentFinancialContext{
		Currency:           "TRY",
		Unit:               "thousand_try",
		ConsolidationScope: "consolidated",
		AuditStatus:        "audited",
	}
	row := tableRowCandidate{
		TableID: "tbl-period",
		Page:    1,
		Line:    1,
		Raw:     "Ticari Alacaklar 100.000 85.000",
		Cells:   []string{"Ticari Alacaklar", "100.000", "85.000"},
	}
	facts := financialFactsFromTableRow(doc, row, &docDate, &period, "2026-06-17T12:00:00Z", context, "current_assets")
	if len(facts) != 2 {
		t.Fatalf("facts=%+v, want 2", facts)
	}
	if facts[0].ReviewRequired || !facts[0].Certification.AnalysisUsable {
		t.Fatalf("first/current column should be usable: %+v", facts[0])
	}
	if !facts[1].ReviewRequired || facts[1].Certification.AnalysisUsable || !containsTestString(facts[1].Warnings, "period_column_unresolved") {
		t.Fatalf("second/prior column should require period review: %+v", facts[1])
	}
}

func TestReconcileFinancialContradictionsResolvesClearWinner(t *testing.T) {
	period := "2025/12"
	docDate := "2026-02-19"
	resolved := ReconcileFinancialValueDifferences([]ExtractedFinancialFact{
		{
			ID:                 "certified-total-assets",
			Ticker:             "TEST",
			SourceFile:         "certified.pdf",
			SHA256:             "sha-certified",
			DocumentDate:       &docDate,
			Period:             &period,
			StatementType:      "balance_sheet",
			LineItemOriginal:   "Toplam varlıklar",
			LineItemNormalized: "total_assets",
			Value:              1_250_000,
			Currency:           "TRY",
			Unit:               "thousand_try",
			ConsolidationScope: "consolidated",
			AuditStatus:        "audited",
			Source:             DocumentFactSource{Page: 12, TableID: "tbl-bs", Snippet: "Toplam varlıklar 1.250.000"},
			Confidence:         0.95,
			Certification: EvidenceCertification{
				Status:         EvidenceStatusCertified,
				Score:          100,
				AnalysisUsable: true,
			},
			CreatedAt: "2026-06-17T12:00:00Z",
		},
		{
			ID:                 "review-total-assets",
			Ticker:             "TEST",
			SourceFile:         "review.pdf",
			SHA256:             "sha-review",
			DocumentDate:       &docDate,
			Period:             &period,
			StatementType:      "balance_sheet",
			LineItemOriginal:   "Toplam varlıklar",
			LineItemNormalized: "total_assets",
			Value:              950_000,
			Currency:           "TRY",
			Unit:               "thousand_try",
			Source:             DocumentFactSource{Page: 4, Snippet: "Toplam varlıklar 950.000"},
			Confidence:         0.82,
			ReviewRequired:     true,
			Certification: EvidenceCertification{
				Status:              EvidenceStatusReviewRequired,
				Score:               60,
				RequiresHumanReview: true,
			},
			CreatedAt: "2026-06-17T12:00:00Z",
		},
	})

	if len(resolved) != 1 {
		t.Fatalf("expected one resolved reconciliation, got %+v", resolved)
	}
	if resolved[0].SelectedValue != 1_250_000 || resolved[0].SelectedSourceFile != "certified.pdf" {
		t.Fatalf("unexpected selected value: %+v", resolved[0])
	}
}

func TestReconcileFinancialContradictionsResolvesAmbiguousCertifiedValuesDeterministically(t *testing.T) {
	period := "2025/12"
	docDate := "2026-02-19"
	base := ExtractedFinancialFact{
		Ticker:             "TEST",
		DocumentDate:       &docDate,
		Period:             &period,
		StatementType:      "balance_sheet",
		LineItemOriginal:   "Toplam varlıklar",
		LineItemNormalized: "total_assets",
		Currency:           "TRY",
		Unit:               "thousand_try",
		ConsolidationScope: "consolidated",
		AuditStatus:        "audited",
		Source:             DocumentFactSource{Page: 12, TableID: "tbl-bs", Snippet: "Toplam varlıklar"},
		Confidence:         0.95,
		Certification: EvidenceCertification{
			Status:         EvidenceStatusCertified,
			Score:          100,
			AnalysisUsable: true,
		},
		CreatedAt: "2026-06-17T12:00:00Z",
	}
	first := base
	first.ID = "certified-a"
	first.SourceFile = "a.pdf"
	first.SHA256 = "sha-a"
	first.Value = 1_250_000
	second := base
	second.ID = "certified-b"
	second.SourceFile = "b.pdf"
	second.SHA256 = "sha-b"
	second.Value = 1_500_000

	resolved := ReconcileFinancialValueDifferences([]ExtractedFinancialFact{first, second})
	if len(resolved) != 1 {
		t.Fatalf("expected deterministic reconciliation, got %+v", resolved)
	}
	if resolved[0].SelectedValue != 1_250_000 || resolved[0].SelectedSourceFile != "a.pdf" {
		t.Fatalf("unexpected deterministic selected value: %+v", resolved[0])
	}
}

func TestCompanyKnowledgeGraphJSONUsesResolvedReconciliationNamesAndReadsLegacyContradictions(t *testing.T) {
	graph := CompanyKnowledgeGraph{
		Ticker: "TEST",
		Contradictions: []KnowledgeContradiction{{
			Type:     "financial_value_conflict",
			Key:      "2025/12|balance_sheet|total_assets|TRY|thousand_try",
			Severity: "medium",
		}},
		ResolvedContradictions: []KnowledgeContradictionResolution{{
			Type:          "financial_value_conflict",
			Key:           "2025/12|balance_sheet|equity|TRY|thousand_try",
			Status:        "resolved",
			SelectedValue: 100,
			Reason:        "Sertifikalı kaynak seçildi.",
		}},
	}
	raw, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("marshal graph: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, `"contradictions"`) || strings.Contains(text, `"resolved_contradictions"`) || strings.Contains(text, `"manual_review_reconciliations"`) {
		t.Fatalf("new graph JSON should not expose legacy contradiction names: %s", text)
	}
	if !strings.Contains(text, `"resolved_reconciliations"`) {
		t.Fatalf("new graph JSON should expose reconciliation names: %s", text)
	}

	legacy := []byte(`{"ticker":"TEST","contradictions":[{"type":"financial_value_conflict","key":"k","severity":"high"}],"resolved_contradictions":[{"type":"financial_value_conflict","key":"r","status":"resolved","selected_value":1,"reason":"legacy"}]}`)
	var decoded CompanyKnowledgeGraph
	if err := json.Unmarshal(legacy, &decoded); err != nil {
		t.Fatalf("unmarshal legacy graph: %v", err)
	}
	if len(decoded.Contradictions) != 1 || len(decoded.ResolvedContradictions) != 1 {
		t.Fatalf("legacy reconciliation fields were not preserved: %+v", decoded)
	}
}

func TestExtractDocumentIntelligenceRejectsNoisyKAPRows(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "ALGYO", "kap", "attachments", "2026", "noisy.pdf"),
		SHA256:            "sha-intel-noise",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 2025 Faaliyet Raporu.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: DocumentAnnualReport,
		QualityScore:      0.92,
		Text: `Şirketimiz, İstanbul Genel Müdürlük olmak üzere, Adana, İzmir ve Ankara Bölge müdürlükleri ile faaliyet göstermektedir.
Bağımsız Yönetim Kurulu Üyeliği seçimi için Aday Gösterme Komitesi değerlendirme yapmıştır.
Başlangıç Tarihi Bitiş Tarihi Komite Üyeleri
Bafllang›ç Tarihi Bitifl Tarihi Yönetim Kurulu
Denetimden Sorumlu Komite’nin çalışma esasları
Tapu Kadastro Web Portalı’ndan alınan bilgiler
Sermaye Piyasası Kanunu ve ilgili mevzuat Yönetim kurulu, 2026/2030 yılları arasında
Esas Sözleşmesi Yönetim kurulu toplantılarında dikkate alınır
MADDE 11. YÖNETİM KURULU VE GÖREV SÜRESİ
- Yönetim Kurulu Üyeleri Hakkında Bilgiler 5
Gayrimenkul Değerleme Ve Genel Müdür
Kimlik No Yönetim Kurulu
ĠnĢaat Mühendisi Genel Müdür
ARTIBĠR GAYRĠMENKUL DEĞERLEME A.ġ Yönetim Kurulu
Diğer Olarak Yönetim Kurulu
bağımsız üye olarak Hami Özçelik Çataloğlu, Kudret Vurgun ve Mustafa Tansu Uslu’ nun 3 yıl süre ile görev yapmak üzere
Ortaklar Pay Oranı (%) Pay Tutarı
Pay Sahibi Dışındaki Kişilere Özel Talimatlar
Asma Katlı Dükkan                      1.250.000 TL       %49,00
BİRİM KİRA DEĞERİ                      1.250.000 TL       %49,00
- İş Bankası A.Ş. Likit Fon TL 73.400 678 - - 109,11084 73.977 0,0%
- İş Bankası A.Ş. Tahvil TL 11.08.09 543.972 10.249 - - 54,55462 559.130 0,2%
- Garanti Bankası A.Ş. TL 01.12.09 690.243 - 9,00% 04.01.10 - 695.519 0,3%
anonim ortaklıkların sermaye artırımı dolayısıyla ihraç edecekleri hisse senetlerinin kayda alınması
Genel Kurulun onayına sunar.
3- Genel Kurul toplantı tutanağının imzalanması için Toplantı Başkanlığına yetki verilmesi
arttırabilmektedirler. Bu tip “bedelsiz hisse” dağıtımları, pay başına kazanç hesaplamalarında, ihraç edilmiş hisse
Yatırım amaçlı gayrimenkul satışı (***) (19.075.000)
İştirakler ve/veya İş Ortaklıkları Pay Alımı veya Sermaye Artırımı Sebebiyle Oluşan
İştiraklar ve/veya İş Ortaklıkları Pay Alımı veya Sermaye Artırımı Sebebiyle Oluşan Nakit (108.023.200) -
Maddi Duran Varlık Satışı 16.520 -
Sermaye Artırımı Sebebiyle Oluşan Nakit Çıkışları 24 (10.096.130) –
ORTAKLARA ORTAKLARA DAĞITILAN KAR PAYININ BAĞIŞLAR EKLENMİŞ NET DAĞITILABİLİR
Varlık Satışları veya Ayni Sermaye Katkıları
Bu değişiklik ile bir yatırımcı ile iştirak veya iş ortaklığı arasındaki varlık satışları veya ayni sermaye
Birleşmelerin Etkisi (526.035.816)
30 Eylül 2025 tarihi itibarıyla Şirketin iç kaynaklarından sağlanan alımların toplamı 14.539.680 adet paya isabet eden 85.000.000 TL
dönem karının 193.200.000 TL’ lik kısmının ortaklara kâr payı olarak dağıtılmasına,

Toplam Varlıklar                         25.422.099.313 TL
Yatırım Amaçlı Gayrimenkuller            18.500.000.000 TL
Yönetim Kurulu Üyesi Mehmet Ahkemoğlu
Genel Müdür Mustafa Filiz
Alarko Holding A.fi.                     1.250.000 TL       %49,00
Alarko Holding A.Ş.                      1.250.000 TL       %49,00
2025 yılı kar payı dağıtım tablosu kapsamında nakit kar payı dağıtılmasına karar verilmiştir 7.245.000 TL`,
		TextLength: 900,
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-15T00:00:00Z")
	for _, person := range got.People {
		switch person.FullName {
		case "Adana İzmir", "Aday Gösterme", "Başlangıç Tarihi Bitiş Tarihi", "Bafllang›ç Tarihi", "Denetimden Sorumlu", "Tapu Kadastro", "Sermaye Piyasası", "Esas Sözleşmesi", "VE GÖREV SÜRESİ", "Hakkında Bilgiler", "Değerleme Ve", "Kimlik No", "İnşaat Mühendisi", "ARTIBİR GAYRİMENKUL", "Diğer Olarak", "Özçelik Çataloğlu Kudret Vurgun", "Mustafa Tansu Uslu’":
			t.Fatalf("noisy person extracted: %+v", person)
		}
	}
	if !containsPerson(got.People, "Mehmet Ahkemoğlu") || !containsPerson(got.People, "Mustafa Filiz") {
		t.Fatalf("valid people missing: %+v", got.People)
	}
	if !containsPerson(got.People, "Hami Özçelik Çataloğlu") || !containsPerson(got.People, "Kudret Vurgun") || !containsPerson(got.People, "Mustafa Tansu Uslu") {
		t.Fatalf("comma/apostrophe separated people missing or merged incorrectly: %+v", got.People)
	}
	for _, fact := range got.OwnershipFacts {
		switch fact.HolderName {
		case "Ortaklar Pay Oranı Pay Tutarı", "Pay Sahibi Dışındaki Kişilere Özel Talimatlar", "Asma Katlı Dükkan", "BİRİM KİRA DEĞERİ", "İş Bankası A.Ş.", "Garanti Bankası A.Ş.":
			t.Fatalf("noisy ownership fact extracted: %+v", fact)
		}
	}
	if !containsOwner(got.OwnershipFacts, "Alarko Holding A.Ş.") {
		t.Fatalf("valid ownership fact missing: %+v", got.OwnershipFacts)
	}
	for _, event := range got.CorporateEvents {
		if event.EventType == "merger" || event.Title == "Genel Kurulun onayına sunar." || strings.Contains(event.Title, "anonim ortaklıkların sermaye artırımı") || strings.Contains(event.Title, "toplantı tutanağının imzalanması") || strings.Contains(event.Title, "bedelsiz hisse") || strings.Contains(event.Title, "Yatırım amaçlı gayrimenkul satışı") || strings.Contains(event.Title, "İştirakler ve/veya İş Ortaklıkları") || strings.Contains(event.Title, "İştiraklar ve/veya İş Ortaklıkları") || strings.Contains(event.Title, "Maddi Duran Varlık Satışı") || strings.Contains(event.Title, "Sermaye Artırımı Sebebiyle Oluşan Nakit") || strings.Contains(event.Title, "DAĞITILAN KAR PAYININ BAĞIŞLAR") || strings.Contains(event.Title, "Varlık Satışları veya Ayni Sermaye") || strings.Contains(event.Title, "Bu değişiklik ile bir yatırımcı") {
			t.Fatalf("generic corporate event extracted: %+v", event)
		}
	}
	if !containsCorporateEvent(got.CorporateEvents, "dividend") {
		t.Fatalf("valid dividend event missing: %+v", got.CorporateEvents)
	}
	for _, fact := range got.FinancialFacts {
		if fact.LineItemNormalized == "total_assets" && strings.Contains(fact.Source.Snippet, "iç kaynaklarından sağlanan alımlar") {
			t.Fatalf("share buyback narrative extracted as financial fact: %+v", fact)
		}
		if fact.LineItemNormalized == "net_income" && strings.Contains(fact.Source.Snippet, "ortaklara kâr payı") {
			t.Fatalf("dividend narrative extracted as net income fact: %+v", fact)
		}
	}
	if !containsFinancialFact(got.FinancialFacts, "total_assets") || !containsFinancialFact(got.FinancialFacts, "investment_properties") {
		t.Fatalf("valid financial facts missing: %+v", got.FinancialFacts)
	}
}

func TestCriticalOCRAuditTextIsAppendedWhenItAddsCorporateActionCoverage(t *testing.T) {
	base := strings.Repeat("Yönetim Kurulu faaliyet açıklaması. ", 80) + "Sermaye artırımı kararı alınmıştır."
	ocr := "Bedelli sermaye artırımı %50 hak kullanım tarihi 01.07.2026 rüçhan kullanım fiyatı 1,00 TL"

	text, method, warnings := selectOCRCandidate(base, "pdftotext", nil, QualityResult{Score: 0.95, TextLength: len([]rune(base))}, ocr, []string{"ocr_fallback_used"})

	if method != "pdftotext+ocr_audit" {
		t.Fatalf("method = %q", method)
	}
	if !strings.Contains(text, "###OCR_AUDIT_TEXT###") || !strings.Contains(text, "rüçhan kullanım fiyatı") {
		t.Fatalf("ocr audit text not appended: %s", text)
	}
	if !containsTestString(warnings, "ocr_audit_text_appended") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestVisionIsTriedWhenCriticalCorporateActionCoverageIsIncomplete(t *testing.T) {
	extractor := PDFTextExtractor{EnableVision: true, VisionCommand: "vision"}
	text := "Sermaye artırımı kararı alınmıştır."

	if !extractor.shouldTryVision(text, nil) {
		t.Fatalf("critical corporate action with incomplete ratio/date evidence should trigger vision")
	}
}

func TestCorporateCapitalIncreaseEventCertificationRequiresActionFields(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "TEST", "kap", "attachments", "2026", "capital.pdf"),
		SHA256:            "sha-capital-review",
		Ticker:            "TEST",
		FileName:          "Sermaye Artırımı.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: DocumentCapitalIncrease,
		QualityScore:      0.94,
		Text:              "Sermaye artırımı kararı alınmıştır.",
		TextLength:        38,
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-15T00:00:00Z")
	if len(got.CorporateEvents) != 1 {
		t.Fatalf("expected one corporate event, got %+v", got.CorporateEvents)
	}
	event := got.CorporateEvents[0]
	if !event.ReviewRequired || event.Certification.Status == EvidenceStatusCertified {
		t.Fatalf("incomplete capital increase should require review: %+v", event)
	}
	if !containsTestString(event.Certification.Reasons, "corporate_action_effective_date_missing") || !containsTestString(event.Certification.Reasons, "corporate_action_ratio_missing") {
		t.Fatalf("missing expected certification reasons: %+v", event.Certification)
	}
}

func TestCorporateCapitalIncreaseEventCanBeCertifiedWithRatioDateAndSubscriptionPrice(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "TEST", "kap", "attachments", "2026", "capital-certified.pdf"),
		SHA256:            "sha-capital-certified",
		Ticker:            "TEST",
		FileName:          "Bedelli Sermaye Artırımı.pdf",
		ExtractionMethod:  "pdftotext+ocr_audit",
		DocumentTypeGuess: DocumentCapitalIncrease,
		QualityScore:      0.96,
		Text:              "Bedelli sermaye artırımı        %50        hak kullanım tarihi 01.07.2026        rüçhan kullanım fiyatı 1,00 TL",
		TextLength:        118,
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-15T00:00:00Z")
	if len(got.CorporateEvents) != 1 {
		t.Fatalf("expected one corporate event, got %+v", got.CorporateEvents)
	}
	event := got.CorporateEvents[0]
	if event.Certification.Status != EvidenceStatusCertified || event.ReviewRequired {
		t.Fatalf("complete capital increase should be certified: %+v", event)
	}
	if event.Ratio == nil || *event.Ratio != 50 {
		t.Fatalf("ratio = %+v", event.Ratio)
	}
	if event.EffectiveDate == nil || *event.EffectiveDate != "2026-07-01" {
		t.Fatalf("effective date = %+v", event.EffectiveDate)
	}
	if event.SubscriptionPrice == nil || *event.SubscriptionPrice != 1 {
		t.Fatalf("subscription price = %+v", event.SubscriptionPrice)
	}
}

func TestExtractDocumentIntelligenceCertifiesFinancialRowsWhenMetadataComplete(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "TEST", "kap", "attachments", "2026", "certified.pdf"),
		SHA256:            "sha-intel-certified",
		Ticker:            "TEST",
		FileName:          "TEST 31.12.2025 Konsolide Finansal Tablolar Bagimsiz Denetim Raporu.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: DocumentFinancialStatement,
		QualityScore:      0.94,
		Text: `TEST A.Ş.
31.12.2025 hesap dönemine ait konsolide finansal tablolar
Bağımsız denetimden geçmiştir.
Tutarlar aksi belirtilmedikçe bin TL olarak sunulmuştur.

FİNANSAL DURUM TABLOSU
Varlıklar                         12.500.000
Özkaynaklar                        7.250.000`,
		TextLength: 320,
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-15T00:00:00Z")
	if len(got.FinancialFacts) == 0 || len(got.FinancialTables) == 0 {
		t.Fatalf("expected certified financial rows, got %+v", got)
	}
	foundCertifiedFact := false
	for _, fact := range got.FinancialFacts {
		if fact.Certification.Status == EvidenceStatusCertified {
			foundCertifiedFact = true
			if fact.ReviewRequired || !fact.Certification.AnalysisUsable {
				t.Fatalf("certified fact should be decision-usable: %+v", fact)
			}
			if fact.ConsolidationScope != "consolidated" || fact.AuditStatus != "audited" || fact.Unit != "thousand_try" || fact.Currency != "TRY" {
				t.Fatalf("certified fact missing normalized metadata: %+v", fact)
			}
		}
	}
	if !foundCertifiedFact {
		t.Fatalf("expected at least one certified financial fact, got %+v", got.FinancialFacts)
	}
	foundCertifiedTable := false
	for _, table := range got.FinancialTables {
		if table.Certification.Status == EvidenceStatusCertified {
			foundCertifiedTable = true
			if table.ConsolidationScope != "consolidated" || table.AuditStatus != "audited" || table.Unit != "thousand_try" || table.Currency != "TRY" {
				t.Fatalf("certified table missing normalized metadata: %+v", table)
			}
		}
	}
	if !foundCertifiedTable {
		t.Fatalf("expected at least one certified financial table, got %+v", got.FinancialTables)
	}
}

func TestExtractDocumentIntelligenceBlocksRejectedParseFacts(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "ALGYO", "kap", "attachments", "2026", "scan.pdf"),
		SHA256:            "sha-intel-rejected",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 2026 Finansal Rapor.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: DocumentFinancialStatement,
		Text:              "abc",
		TextLength:        3,
		QualityScore:      0.05,
		Warnings:          []string{"low_text_quality_possible_scanned_pdf"},
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-15T00:00:00Z")
	if got.IndexDoc.ParseStatus != ParseStatusRejected || got.IndexDoc.AnalysisUsable {
		t.Fatalf("expected rejected index doc, got %+v", got.IndexDoc)
	}
	if len(got.Facts) != 0 || len(got.FinancialFacts) != 0 || len(got.FinancialTables) != 0 || len(got.People) != 0 || len(got.OwnershipFacts) != 0 || len(got.CorporateEvents) != 0 {
		t.Fatalf("rejected parse produced analysis facts: %+v", got)
	}
}

func TestExtractDocumentIntelligenceAIResolvesCoordinateTable(t *testing.T) {
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "ALGYO", "kap", "attachments", "2026", "scan.pdf"),
		SHA256:            "sha-intel-rescue",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 31.03.2026 Finansal Rapor.pdf",
		ExtractionMethod:  "pdftotext+tsv",
		DocumentTypeGuess: DocumentFinancialStatement,
		Text: `bozuk OCR metni
###COORDINATE_TABLE_TEXT###
Finansal Durum Tablosu	31.03.2026
Nakit ve Nakit Benzerleri	1.500.000 TL
Net dönem karı	250.000 TL
Özkaynaklar	7.250.000 TL`,
		TextLength:   260,
		QualityScore: 0.55,
		Warnings:     []string{"coordinate_tsv_text_appended"},
	}

	got := ExtractDocumentIntelligence(doc, "2026-06-15T00:00:00Z")
	if got.IndexDoc.ParseStatus != ParseStatusAIResolved || !got.IndexDoc.AnalysisUsable || !got.IndexDoc.AIResolved || got.IndexDoc.HumanReviewRequired {
		t.Fatalf("expected AI-resolved index doc, got %+v", got.IndexDoc)
	}
	if !containsRawWarning(got.IndexDoc.Warnings, lowQualityStructuredRescueWarning) {
		t.Fatalf("expected structured rescue warning: %+v", got.IndexDoc.Warnings)
	}
	if !containsRawWarning(got.IndexDoc.Warnings, structuredRescueAIResolvedWarning) {
		t.Fatalf("expected AI-resolved warning: %+v", got.IndexDoc.Warnings)
	}
	if len(got.FinancialFacts) == 0 || len(got.FinancialTables) == 0 {
		t.Fatalf("expected rescued financial facts and table, got %+v", got)
	}
	for _, fact := range got.FinancialFacts {
		if fact.ReviewRequired || !fact.Certification.AnalysisUsable || fact.Certification.Status != EvidenceStatusAIResolved || !containsRawWarning(fact.Warnings, structuredRescueAIResolvedWarning) {
			t.Fatalf("rescued fact must be AI-resolved: %+v", fact)
		}
	}
	for _, table := range got.FinancialTables {
		if table.ReviewRequired || !table.Certification.AnalysisUsable || table.Certification.Status != EvidenceStatusAIResolved || !containsRawWarning(table.Warnings, structuredRescueAIResolvedWarning) {
			t.Fatalf("rescued table must be AI-resolved: %+v", table)
		}
	}
}

func TestExtractDocumentIntelligenceFromRawDocumentsWritesTickerIndex(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, RawDocumentsFile)
	file, err := os.Create(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := RawDocument{
		FilePath:          filepath.Join("data", "equities", "ALGYO", "kap", "attachments", "2026", "sample.pdf"),
		SHA256:            "sha-intel-2",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 31.03.2026 Finansal Rapor.pdf",
		ExtractionMethod:  "pdftotext",
		DocumentTypeGuess: DocumentFinancialStatement,
		QualityScore:      0.88,
		Text:              "Finansal Durum Tablosu\nNakit ve Nakit Benzerleri      1.500.000 TL\nNet dönem karı                 250.000 TL\nYönetim Kurulu Üyesi Mehmet Kaya",
		TextLength:        180,
	}
	if err := json.NewEncoder(file).Encode(doc); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	summary, err := ExtractDocumentIntelligenceFromRawDocuments(context.Background(), DocumentIntelligenceOptions{
		RawDocumentsPath: rawPath,
		OutputDir:        dir,
		Now:              func() time.Time { return time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ExtractDocumentIntelligenceFromRawDocuments error = %v", err)
	}
	if summary.DocumentFacts == 0 || summary.FinancialFacts == 0 || summary.FinancialTables == 0 || summary.DocumentIndexes != 1 || summary.KnowledgeGraphs != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	indexPath := filepath.Join(dir, "by_ticker", "ALGYO", DocumentIndexFile)
	var index DocumentIndex
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("missing document index: %v", err)
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatalf("invalid document index: %v", err)
	}
	if index.Counts.FinancialFacts == 0 || index.Counts.FinancialTables == 0 || len(index.Documents) != 1 {
		t.Fatalf("unexpected index: %+v", index)
	}
	graphPath := filepath.Join(dir, "by_ticker", "ALGYO", CompanyKnowledgeGraphFile)
	var graph CompanyKnowledgeGraph
	graphRaw, err := os.ReadFile(graphPath)
	if err != nil {
		t.Fatalf("missing company knowledge graph: %v", err)
	}
	if err := json.Unmarshal(graphRaw, &graph); err != nil {
		t.Fatalf("invalid company knowledge graph: %v", err)
	}
	if graph.Ticker != "ALGYO" || len(graph.Nodes) == 0 || len(graph.Edges) == 0 {
		t.Fatalf("unexpected company knowledge graph: %+v", graph)
	}
	factsPath := filepath.Join(dir, "by_ticker", "ALGYO", FinancialFactsFile)
	factsFile, err := os.Open(factsPath)
	if err != nil {
		t.Fatalf("missing financial facts: %v", err)
	}
	defer factsFile.Close()
	scanner := bufio.NewScanner(factsFile)
	if !scanner.Scan() {
		t.Fatalf("expected at least one financial fact row")
	}
	var fact ExtractedFinancialFact
	if err := json.Unmarshal(scanner.Bytes(), &fact); err != nil {
		t.Fatalf("invalid financial fact row: %v", err)
	}
	if fact.Source.Snippet == "" || fact.LineItemNormalized == "" {
		t.Fatalf("fact missing indexed fields: %+v", fact)
	}
	if fact.Certification.Status == "" || !fact.ReviewRequired {
		t.Fatalf("fact should carry certification and review gate: %+v", fact)
	}
}

func containsPerson(values []ExtractedPerson, name string) bool {
	for _, value := range values {
		if value.FullName == name {
			return true
		}
	}
	return false
}

func containsOwner(values []OwnershipFact, holder string) bool {
	for _, value := range values {
		if value.HolderName == holder {
			return true
		}
	}
	return false
}

func containsCorporateEvent(values []ExtractedCorporateEvent, eventType string) bool {
	for _, value := range values {
		if value.EventType == eventType {
			return true
		}
	}
	return false
}

func containsFinancialFact(values []ExtractedFinancialFact, normalized string) bool {
	for _, value := range values {
		if value.LineItemNormalized == normalized {
			return true
		}
	}
	return false
}

func containsTestString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
