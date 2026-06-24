package kapingest

import (
	"context"
	"strings"
)

type LLMClient interface {
	ExtractKAPEvent(ctx context.Context, doc RawDocument) (KAPEvent, error)
}

const PromptTemplate = `Sen KAP PDF bilgi çıkarım motorusun.

Sana bir BIST şirketine ait KAP PDF’den çıkarılmış ham metin verilecek.

Görevin:
Bu metinden yatırım analizi için kullanılabilecek yapılandırılmış JSON çıkar.

Kurallar:
- Metinde olmayan bilgiyi uydurma.
- Emin değilsen null yaz.
- Her önemli çıkarım için kaynak cümle veya kısa snippet ver.
- PDF faaliyet raporuysa faaliyet raporu olarak sınıflandır.
- Finansal tablo içeriyorsa ana mali göstergeleri çıkar.
- GYO ise gayrimenkul portföyü, ekspertiz değeri, kira geliri, net aktif değer, borçluluk ve değerleme kârı gibi alanları özellikle çıkar.
- Banka, sigorta, holding, sanayi şirketi ayrımını yap.
- Kısa vadeli ve uzun vadeli hisse etkisini kesin tahmin gibi değil, şartlı yorum olarak yaz.
- Çıktı sadece JSON olsun.
- JSON dışında açıklama yazma.

JSON şeması:

{
  "ticker": "",
  "company_name": "",
  "file_path": "",
  "sha256": "",
  "document_date": null,
  "period": null,
  "document_class": "",
  "financial_profile": "",
  "event_category": "",
  "summary": "",
  "important_points": [],
  "financial_highlights": {},
  "portfolio_highlights": {},
  "positive_points": [],
  "negative_points": [],
  "risk_flags": [],
  "opportunity_flags": [],
  "impact": {
    "fundamental": "positive / negative / neutral / mixed / uncertain",
    "short_term_price": "positive / negative / neutral / uncertain",
    "long_term": "positive / negative / neutral / mixed / uncertain",
    "confidence": 0,
    "reason": ""
  },
  "source_references": [
    {
      "page": null,
      "field": "",
      "snippet": ""
    }
  ]
}

HAM METİN:
{{TEXT}}`

func BuildPrompt(doc RawDocument, maxChars int) string {
	text := doc.Text
	if maxChars > 0 {
		text = firstRunes(text, maxChars)
	}
	return strings.Replace(PromptTemplate, "{{TEXT}}", text, 1)
}

type RuleBasedLLMClient struct{}

func (RuleBasedLLMClient) ExtractKAPEvent(_ context.Context, doc RawDocument) (KAPEvent, error) {
	summary := firstMeaningfulLine(doc.Text)
	if summary == "" {
		summary = doc.DocumentTypeGuess
	}
	if summary == "" {
		summary = DocumentUnknown
	}
	event := KAPEvent{
		Ticker:              doc.Ticker,
		FilePath:            doc.FilePath,
		SHA256:              doc.SHA256,
		DocumentClass:       emptyAsUnknown(doc.DocumentTypeGuess),
		FinancialProfile:    inferFinancialProfile(doc.Text),
		EventCategory:       emptyAsUnknown(doc.DocumentTypeGuess),
		Summary:             firstRunes(summary, 500),
		ImportantPoints:     importantSnippets(doc.Text, 5),
		FinancialHighlights: map[string]any{},
		PortfolioHighlights: map[string]any{},
		PositivePoints:      []string{},
		NegativePoints:      []string{},
		RiskFlags:           lowQualityRiskFlags(doc),
		OpportunityFlags:    []string{},
		Impact: Impact{
			Fundamental:    "uncertain",
			ShortTermPrice: "uncertain",
			LongTerm:       "uncertain",
			Confidence:     0.25,
			Reason:         "Gerçek LLM istemcisi bağlı değil; bu kayıt deterministik kural tabanlı MVP çıkarımıdır.",
		},
		SourceReferences: []SourceReference{
			{Field: "summary", Snippet: firstRunes(summary, 300)},
		},
	}
	return event, nil
}

func firstMeaningfulLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if len([]rune(line)) >= 20 {
			return line
		}
	}
	return ""
}

func inferFinancialProfile(text string) string {
	slug := turkishASCII(strings.ToUpper(firstRunes(text, 30000)))
	switch {
	case strings.Contains(slug, "BANKA") || strings.Contains(slug, "NET FAIZ"):
		return "bank"
	case strings.Contains(slug, "SIGORTA") || strings.Contains(slug, "TEKNIK BOLUM"):
		return "insurance"
	case strings.Contains(slug, "GAYRIMENKUL YATIRIM ORTAKLIGI") || strings.Contains(slug, "PORTFOY TABLOSU"):
		return "gyo"
	case strings.Contains(slug, "HOLDING"):
		return "holding"
	case strings.Contains(slug, "FINANSAL DURUM TABLOSU") || strings.Contains(slug, "HASILAT"):
		return "industrial_or_service"
	default:
		return "unknown"
	}
}

func importantSnippets(text string, limit int) []string {
	keywords := []string{"finansal durum", "kar veya zarar", "nakit akis", "ozkaynak", "hasilat", "net donem", "degerleme", "sermaye", "kar payi"}
	out := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if len([]rune(trimmed)) < 20 {
			continue
		}
		slug := turkishASCII(strings.ToLower(trimmed))
		for _, keyword := range keywords {
			if strings.Contains(slug, turkishASCII(keyword)) && !seen[trimmed] {
				seen[trimmed] = true
				out = append(out, firstRunes(trimmed, 300))
				break
			}
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func lowQualityRiskFlags(doc RawDocument) []string {
	gate := QualityGateForRawDocument(doc)
	if gate.Status == ParseStatusTrusted {
		return nil
	}
	return QualityGateWarnings(gate)
}

func emptyAsUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return DocumentUnknown
	}
	return value
}
