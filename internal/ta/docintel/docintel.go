package docintel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	pdf "github.com/ledongthuc/pdf"
)

type Input struct {
	EquitiesDir string
	Symbol      string
	AsOf        time.Time
	Limit       int
	TextChars   int
}

type Report struct {
	Computed            bool              `json:"computed"`
	Symbol              string            `json:"symbol,omitempty"`
	Root                string            `json:"root,omitempty"`
	TotalFiles          int               `json:"total_files"`
	PDFCount            int               `json:"pdf_count"`
	OfficeCount         int               `json:"office_count"`
	XBRLCount           int               `json:"xbrl_count"`
	ContentAnalyzed     int               `json:"content_analyzed"`
	ContentReadable     int               `json:"content_readable"`
	LatestYear          int               `json:"latest_year,omitempty"`
	CoverageScore       float64           `json:"coverage_score"`
	Summary             string            `json:"summary"`
	LLMAnalysis         LLMReport         `json:"llm_analysis"`
	Categories          []CategorySummary `json:"categories,omitempty"`
	KeyDocuments        []Document        `json:"key_documents,omitempty"`
	MissingKeyDocuments []string          `json:"missing_key_documents,omitempty"`
	Warnings            []string          `json:"warnings,omitempty"`
}

type LLMReport struct {
	Computed           bool     `json:"computed"`
	Status             string   `json:"status"`
	Provider           string   `json:"provider,omitempty"`
	Model              string   `json:"model,omitempty"`
	Endpoint           string   `json:"endpoint,omitempty"`
	PromptVersion      string   `json:"prompt_version,omitempty"`
	DocumentsRequested int      `json:"documents_requested,omitempty"`
	DocumentsAnalyzed  int      `json:"documents_analyzed,omitempty"`
	Warning            string   `json:"warning,omitempty"`
	Errors             []string `json:"errors,omitempty"`
}

type LLMDocumentAnalysis struct {
	Computed         bool     `json:"computed"`
	Status           string   `json:"status"`
	Provider         string   `json:"provider,omitempty"`
	Model            string   `json:"model,omitempty"`
	PromptVersion    string   `json:"prompt_version,omitempty"`
	Purpose          string   `json:"purpose,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	KeyFindings      []string `json:"key_findings,omitempty"`
	ValuationImpact  []string `json:"valuation_impact,omitempty"`
	RiskFlags        []string `json:"risk_flags,omitempty"`
	EvidenceSnippets []string `json:"evidence_snippets,omitempty"`
	SourceTextChars  int      `json:"source_text_chars,omitempty"`
}

type CategorySummary struct {
	Category   string    `json:"category"`
	Label      string    `json:"label"`
	Count      int       `json:"count"`
	LatestYear int       `json:"latest_year,omitempty"`
	LatestDate time.Time `json:"latest_date,omitempty"`
}

type Document struct {
	Category         string               `json:"category"`
	CategoryLabel    string               `json:"category_label"`
	FileName         string               `json:"file_name"`
	Path             string               `json:"path"`
	Extension        string               `json:"extension"`
	Year             int                  `json:"year,omitempty"`
	DisclosureIndex  int                  `json:"disclosure_index,omitempty"`
	PublishedAt      time.Time            `json:"published_at,omitempty"`
	DisclosureTitle  string               `json:"disclosure_title,omitempty"`
	DisclosureType   string               `json:"disclosure_type,omitempty"`
	DisclosureClass  string               `json:"disclosure_class,omitempty"`
	DisclosurePeriod string               `json:"disclosure_period,omitempty"`
	DisclosureYear   int                  `json:"disclosure_year,omitempty"`
	SizeBytes        int64                `json:"size_bytes,omitempty"`
	TextExtracted    bool                 `json:"text_extracted"`
	TextChars        int                  `json:"text_chars,omitempty"`
	Purpose          string               `json:"purpose,omitempty"`
	ContentSummary   string               `json:"content_summary,omitempty"`
	Topics           []string             `json:"topics,omitempty"`
	ReportImpact     string               `json:"report_impact,omitempty"`
	ExtractionSource string               `json:"extraction_source,omitempty"`
	ExtractionNote   string               `json:"extraction_note,omitempty"`
	LLMAnalysis      *LLMDocumentAnalysis `json:"llm_analysis,omitempty"`
	Evidence         []string             `json:"evidence,omitempty"`
	contextText      string               `json:"-"`
	analysisText     string               `json:"-"`
}

type llmConfig struct {
	Configured bool
	Provider   string
	Model      string
	Endpoint   string
	APIKey     string
	DocLimit   int
}

type detailMetadata struct {
	Title           string
	Summary         string
	DisclosureType  string
	DisclosureClass string
	Category        string
	Period          string
	Year            int
	PublishedAt     time.Time
	BodyText        string
}

type detailFile struct {
	Raw []struct {
		Disclosure struct {
			Basic  map[string]any `json:"disclosureBasic"`
			Detail map[string]any `json:"disclosureDetail"`
		} `json:"disclosure"`
		DisclosureBody []string `json:"disclosureBody"`
	} `json:"raw"`
}

func Analyze(input Input) Report {
	symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
	limit := input.Limit
	if limit <= 0 {
		limit = 40
	}
	textChars := input.TextChars
	if textChars <= 0 {
		textChars = 8000
	}
	root := filepath.Join(input.EquitiesDir, symbol, "kap", "attachments")
	out := Report{
		Symbol: symbol,
		Root:   root,
	}
	if symbol == "" || strings.TrimSpace(input.EquitiesDir) == "" {
		out.Summary = "KAP belge kanıtı okunamadı; hisse kodu veya equities dizini boş."
		out.MissingKeyDocuments = append(out.MissingKeyDocuments, "kap_attachments")
		return out
	}
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			out.Summary = "KAP PDF/ek klasörü bulunamadı; resmi belge kanıtı rapora bağlanamadı."
			out.MissingKeyDocuments = append(out.MissingKeyDocuments, "kap_attachments")
			return out
		}
		out.Summary = "KAP PDF/ek klasörü okunamadı: " + err.Error()
		out.Warnings = append(out.Warnings, "kap_attachment_scan_failed")
		return out
	}

	metaCache := map[int]detailMetadata{}
	categoryCounts := map[string]*CategorySummary{}
	docs := []Document{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			out.Warnings = append(out.Warnings, "kap_attachment_walk_error")
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		year, index := parseAttachmentPath(root, path)
		meta, ok := metaCache[index]
		if index > 0 && !ok {
			meta = loadDetailMetadata(input.EquitiesDir, symbol, index)
			metaCache[index] = meta
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		category, evidence := classifyDocument(name, meta)
		doc := Document{
			Category:         category,
			CategoryLabel:    CategoryLabel(category),
			FileName:         name,
			Path:             path,
			Extension:        ext,
			Year:             year,
			DisclosureIndex:  index,
			PublishedAt:      meta.PublishedAt,
			DisclosureTitle:  meta.Title,
			DisclosureType:   meta.DisclosureType,
			DisclosureClass:  meta.DisclosureClass,
			DisclosurePeriod: meta.Period,
			DisclosureYear:   meta.Year,
			SizeBytes:        info.Size(),
			Evidence:         evidence,
			contextText:      meta.BodyText,
		}
		out.TotalFiles++
		if year > out.LatestYear {
			out.LatestYear = year
		}
		switch ext {
		case "pdf":
			out.PDFCount++
		case "xls", "xlsx", "doc", "docx", "ppt", "pptx":
			out.OfficeCount++
		case "xml", "xbrl", "zip":
			out.XBRLCount++
		}
		summary := categoryCounts[category]
		if summary == nil {
			summary = &CategorySummary{Category: category, Label: CategoryLabel(category)}
			categoryCounts[category] = summary
		}
		summary.Count++
		if year > summary.LatestYear {
			summary.LatestYear = year
		}
		if meta.PublishedAt.After(summary.LatestDate) {
			summary.LatestDate = meta.PublishedAt
		}
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		out.Warnings = append(out.Warnings, "kap_attachment_scan_failed")
		out.Summary = "KAP PDF/ek taraması tamamlanamadı: " + err.Error()
		return out
	}
	out.Categories = sortedCategorySummaries(categoryCounts)
	sortDocuments(docs)
	out.KeyDocuments = limitDocuments(docs, limit)
	analyzeDocumentContents(&out, textChars)
	analyzeDocumentsWithLLM(&out, textChars)
	out.MissingKeyDocuments = missingKeyDocuments(categoryCounts)
	out.CoverageScore = coverageScore(categoryCounts)
	out.Computed = out.TotalFiles > 0
	out.Summary = buildSummary(out)
	if len(out.MissingKeyDocuments) > 0 {
		out.Warnings = append(out.Warnings, "kap_key_document_coverage_limited")
	}
	return out
}

func CategoryLabel(category string) string {
	switch category {
	case "financial_report":
		return "Finansal rapor"
	case "activity_report":
		return "Faaliyet raporu"
	case "audit_report":
		return "Bağımsız denetim / BDR"
	case "valuation_report":
		return "Değerleme / halka arz varsayım raporu"
	case "dividend":
		return "Kar payı / temettü"
	case "buyback":
		return "Pay geri alım"
	case "capital_action":
		return "Sermaye / esas sözleşme / ihraç"
	case "general_assembly":
		return "Genel kurul"
	case "fund_usage":
		return "Fon kullanım"
	case "governance":
		return "Yönetişim / politika / sürdürülebilirlik"
	default:
		return "Diğer KAP eki"
	}
}

func parseAttachmentPath(root, path string) (int, int) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0, 0
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 {
		return 0, 0
	}
	year, _ := strconv.Atoi(parts[0])
	index, _ := strconv.Atoi(parts[1])
	return year, index
}

func loadDetailMetadata(equitiesDir, symbol string, index int) detailMetadata {
	if index <= 0 {
		return detailMetadata{}
	}
	path := filepath.Join(equitiesDir, "_kap", "details", symbol, strconv.Itoa(index)+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return detailMetadata{}
	}
	var detail detailFile
	if json.Unmarshal(raw, &detail) != nil || len(detail.Raw) == 0 {
		return detailMetadata{}
	}
	basic := detail.Raw[0].Disclosure.Basic
	return detailMetadata{
		Title:           firstString(basic, "title"),
		Summary:         firstString(basic, "summary"),
		DisclosureType:  firstString(basic, "disclosureType"),
		DisclosureClass: firstString(basic, "disclosureClass"),
		Category:        firstString(basic, "disclosureCategory"),
		Period:          firstString(basic, "period"),
		Year:            firstInt(basic, "year"),
		PublishedAt:     parseKAPTime(firstString(basic, "publishDate")),
		BodyText:        bodyText(detail.Raw[0].DisclosureBody),
	}
}

func analyzeDocumentContents(report *Report, textChars int) {
	for i := range report.KeyDocuments {
		doc := &report.KeyDocuments[i]
		report.ContentAnalyzed++
		text := ""
		if doc.Extension == "pdf" {
			extracted, note := extractPDFText(doc.Path, textChars)
			if extracted != "" {
				text = extracted
				doc.TextExtracted = true
				doc.TextChars = len([]rune(extracted))
				doc.ExtractionSource = "pdf_text"
				report.ContentReadable++
			} else {
				doc.ExtractionNote = note
			}
		}
		if text == "" && strings.TrimSpace(doc.contextText) != "" {
			text = limitRunes(cleanText(doc.contextText), textChars)
			doc.TextExtracted = true
			doc.TextChars = len([]rune(text))
			doc.ExtractionSource = "kap_disclosure_body"
			report.ContentReadable++
			if doc.ExtractionNote != "" {
				doc.ExtractionNote += "; KAP bildirim gövdesi kullanıldı"
			}
		}
		if text == "" {
			doc.Purpose, doc.ContentSummary, doc.ReportImpact = categoryDefaultAnalysis(*doc)
			if doc.ExtractionNote == "" {
				doc.ExtractionNote = "Metin çıkarılamadı; PDF taranmış görüntü olabilir veya desteklenmeyen formatta."
			}
			continue
		}
		doc.analysisText = text
		doc.Purpose = inferPurpose(*doc, text)
		doc.Topics = detectTopics(text)
		doc.ContentSummary = summarizeContent(*doc, text)
		doc.ReportImpact = inferReportImpact(*doc, text)
	}
}

func analyzeDocumentsWithLLM(report *Report, textChars int) {
	cfg := loadLLMConfigFromEnv()
	report.LLMAnalysis = LLMReport{
		Status:        "not_configured",
		Provider:      cfg.Provider,
		Model:         cfg.Model,
		Endpoint:      cfg.Endpoint,
		PromptVersion: "kap_pdf_analysis_v1",
		Warning:       "LLM analizi yapılmadı; HISSEBOT_LLM_PROVIDER, HISSEBOT_LLM_ENDPOINT, HISSEBOT_LLM_MODEL ve HISSEBOT_LLM_API_KEY ayarları yok.",
	}
	if !cfg.Configured {
		return
	}
	if cfg.APIKey == "" {
		report.LLMAnalysis.Status = "missing_api_key"
		report.LLMAnalysis.Warning = "LLM provider/model ayarlı fakat HISSEBOT_LLM_API_KEY yok; sahte LLM sonucu üretilmedi."
		report.Warnings = append(report.Warnings, "llm_missing_api_key")
		return
	}
	if cfg.Model == "" {
		report.LLMAnalysis.Status = "missing_model"
		report.LLMAnalysis.Warning = "HISSEBOT_LLM_MODEL boş; model adı verilmeden LLM analizi yapılmadı."
		report.Warnings = append(report.Warnings, "llm_missing_model")
		return
	}
	if cfg.Endpoint == "" {
		report.LLMAnalysis.Status = "missing_endpoint"
		report.LLMAnalysis.Warning = "LLM endpoint boş; HISSEBOT_LLM_ENDPOINT ayarlanmalı veya HISSEBOT_LLM_PROVIDER=openai kullanılmalı."
		report.Warnings = append(report.Warnings, "llm_missing_endpoint")
		return
	}
	client := &http.Client{Timeout: 45 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	limit := cfg.DocLimit
	if limit <= 0 {
		limit = 3
	}
	for i := range report.KeyDocuments {
		if report.LLMAnalysis.DocumentsRequested >= limit {
			break
		}
		doc := &report.KeyDocuments[i]
		text := strings.TrimSpace(doc.analysisText)
		if text == "" {
			continue
		}
		report.LLMAnalysis.DocumentsRequested++
		analysis, err := callLLMDocumentAnalysis(ctx, client, cfg, *doc, limitRunes(text, textChars))
		if err != nil {
			report.LLMAnalysis.Errors = append(report.LLMAnalysis.Errors, fmt.Sprintf("%s: %v", doc.FileName, err))
			continue
		}
		doc.LLMAnalysis = analysis
		report.LLMAnalysis.DocumentsAnalyzed++
	}
	switch {
	case report.LLMAnalysis.DocumentsAnalyzed > 0 && len(report.LLMAnalysis.Errors) == 0:
		report.LLMAnalysis.Computed = true
		report.LLMAnalysis.Status = "complete"
		report.LLMAnalysis.Warning = ""
	case report.LLMAnalysis.DocumentsAnalyzed > 0:
		report.LLMAnalysis.Computed = true
		report.LLMAnalysis.Status = "partial"
		report.LLMAnalysis.Warning = "LLM bazı belgeleri analiz etti; bazı çağrılar hata verdi."
		report.Warnings = append(report.Warnings, "llm_partial")
	default:
		report.LLMAnalysis.Status = "failed"
		report.LLMAnalysis.Warning = "LLM çağrısı yapılandırıldı fakat hiçbir belge için geçerli analiz dönmedi."
		report.Warnings = append(report.Warnings, "llm_failed")
	}
}

func loadLLMConfigFromEnv() llmConfig {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("HISSEBOT_LLM_PROVIDER")))
	endpoint := strings.TrimSpace(os.Getenv("HISSEBOT_LLM_ENDPOINT"))
	model := strings.TrimSpace(os.Getenv("HISSEBOT_LLM_MODEL"))
	apiKey := strings.TrimSpace(os.Getenv("HISSEBOT_LLM_API_KEY"))
	if provider == "openai" && endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}
	if provider == "" && endpoint != "" {
		provider = "openai_compatible"
	}
	docLimit := 3
	if raw := strings.TrimSpace(os.Getenv("HISSEBOT_LLM_DOC_LIMIT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			docLimit = parsed
		}
	}
	return llmConfig{
		Configured: provider != "" || endpoint != "" || model != "" || apiKey != "",
		Provider:   provider,
		Model:      model,
		Endpoint:   endpoint,
		APIKey:     apiKey,
		DocLimit:   docLimit,
	}
}

func callLLMDocumentAnalysis(ctx context.Context, client *http.Client, cfg llmConfig, doc Document, text string) (*LLMDocumentAnalysis, error) {
	payload := map[string]any{
		"model":       cfg.Model,
		"temperature": 0,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Sen finansal KAP belge analisti gibi çalışırsın. Sadece verilen belge metnine dayan. Bilmediğin veya metinde olmayan şeyi uydurma. Yanıtı yalnızca geçerli JSON olarak ver.",
			},
			{
				"role":    "user",
				"content": buildLLMPrompt(doc, text),
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("llm status %d: %s", res.StatusCode, limitRunes(cleanText(string(body)), 300))
	}
	content, err := chatCompletionContent(body)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Purpose          string   `json:"purpose"`
		Summary          string   `json:"summary"`
		KeyFindings      []string `json:"key_findings"`
		ValuationImpact  []string `json:"valuation_impact"`
		RiskFlags        []string `json:"risk_flags"`
		EvidenceSnippets []string `json:"evidence_snippets"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &parsed); err != nil {
		return nil, fmt.Errorf("llm json parse failed: %w", err)
	}
	return &LLMDocumentAnalysis{
		Computed:         true,
		Status:           "ok",
		Provider:         cfg.Provider,
		Model:            cfg.Model,
		PromptVersion:    "kap_pdf_analysis_v1",
		Purpose:          strings.TrimSpace(parsed.Purpose),
		Summary:          strings.TrimSpace(parsed.Summary),
		KeyFindings:      cleanStringList(parsed.KeyFindings, 8),
		ValuationImpact:  cleanStringList(parsed.ValuationImpact, 8),
		RiskFlags:        cleanStringList(parsed.RiskFlags, 8),
		EvidenceSnippets: cleanStringList(parsed.EvidenceSnippets, 6),
		SourceTextChars:  len([]rune(text)),
	}, nil
}

func buildLLMPrompt(doc Document, text string) string {
	return fmt.Sprintf(`Belge:
- Dosya: %s
- KAP başlığı: %s
- Kategori: %s
- Yayın tarihi: %s

Görev:
1. Belgenin ne amaçla yazıldığını açıkla.
2. Yatırımcı için önemli gerçek bulguları çıkar.
3. İçsel değer, güvenlik marjı, borç, nakit akışı, karlılık, sermaye tahsisi veya risk skorunu nasıl etkileyebileceğini yaz.
4. Sadece metinde olan kanıtı kullan. Uydurma yapma.

JSON şeması:
{
  "purpose": "tek kısa paragraf",
  "summary": "tek kısa paragraf",
  "key_findings": ["madde"],
  "valuation_impact": ["madde"],
  "risk_flags": ["madde"],
  "evidence_snippets": ["belgeden kısa kanıt parçası, 15 kelimeyi geçmesin"]
}

Belge metni:
%s`, doc.FileName, doc.DisclosureTitle, doc.CategoryLabel, doc.PublishedAt.Format("2006-01-02"), text)
}

func chatCompletionContent(body []byte) (string, error) {
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return "", errors.New("llm response has no message content")
	}
	return decoded.Choices[0].Message.Content, nil
}

func extractJSONObject(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	value = strings.TrimSpace(value)
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start >= 0 && end > start {
		return value[start : end+1]
	}
	return value
}

func cleanStringList(values []string, limit int) []string {
	out := []string{}
	for _, value := range values {
		value = cleanText(value)
		if value == "" {
			continue
		}
		out = append(out, value)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func extractPDFText(path string, maxChars int) (string, string) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return "", "PDF açılamadı: " + err.Error()
	}
	defer file.Close()
	stream, err := reader.GetPlainText()
	if err != nil {
		return "", "PDF metni çıkarılamadı: " + err.Error()
	}
	raw, err := io.ReadAll(io.LimitReader(stream, int64(maxChars*4)))
	if err != nil {
		return "", "PDF metni okunamadı: " + err.Error()
	}
	text := limitRunes(cleanText(string(raw)), maxChars)
	if len([]rune(text)) < 40 {
		return "", "PDF metni çok kısa; belge taranmış görüntü olabilir."
	}
	return text, ""
}

func bodyText(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = cleanText(stripHTML(part))
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, " ")
}

func stripHTML(value string) string {
	var b strings.Builder
	inTag := false
	for _, r := range value {
		switch r {
		case '<':
			inTag = true
			b.WriteRune(' ')
		case '>':
			inTag = false
			b.WriteRune(' ')
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func cleanText(value string) string {
	value = html.UnescapeString(value)
	replacer := strings.NewReplacer(
		"\u00a0", " ",
		"\r", " ",
		"\n", " ",
		"\t", " ",
		"  ", " ",
	)
	value = replacer.Replace(value)
	for strings.Contains(value, "  ") {
		value = strings.ReplaceAll(value, "  ", " ")
	}
	return strings.TrimSpace(value)
}

func limitRunes(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func categoryDefaultAnalysis(doc Document) (string, string, string) {
	purpose := inferPurpose(doc, "")
	summary := doc.CategoryLabel + " olarak sınıflandırıldı; okunabilir metin çıkarılamadığı için içerik özeti KAP başlığı ve dosya adından üretildi."
	impact := inferReportImpact(doc, "")
	return purpose, summary, impact
}

func inferPurpose(doc Document, text string) string {
	normalized := normalize(doc.FileName + " " + doc.DisclosureTitle + " " + text)
	switch doc.Category {
	case "financial_report":
		return "Şirketin ilgili dönem finansal tablolarını, bilanço/gelir tablosu/nakit akışı ve dipnotlarını kamuya açıklamak için yayımlanmış resmi KAP finansal raporu."
	case "activity_report":
		return "Yönetim kurulunun faaliyet dönemi performansını, operasyonel gelişmeleri, riskleri ve yönetim değerlendirmesini yatırımcıya açıklamak için hazırlanmış faaliyet raporu."
	case "audit_report":
		if containsAny(normalized, "sinirli olumlu", "sartli gorus", "gorus bildirmekten kacinma", "olumsuz gorus") {
			return "Finansal tabloların denetim sonucunu ve denetçi çekincelerini yatırımcıya açıklamak için yayımlanmış bağımsız denetim raporu."
		}
		return "Finansal tabloların bağımsız denetimden geçtiğini ve denetim görüşünü açıklamak için yayımlanmış denetim/BDR belgesi."
	case "valuation_report":
		return "Halka arz fiyatı veya değerleme varsayımlarının gerçekleşme durumunu ve sapmalarını yatırımcıya açıklamak için hazırlanmış değerlendirme raporu."
	case "dividend":
		return "Kar payı/temettü politikası veya dağıtım kararını yatırımcıya bildirmek için hazırlanmış KAP eki."
	case "buyback":
		return "Şirket paylarının geri alım programı, yetkisi, limitleri veya gerekçesini yatırımcıya açıklamak için yayımlanmış belge."
	case "capital_action":
		if containsAny(normalized, "kayitli sermaye", "sermaye tavani") {
			return "Kayıtlı sermaye tavanı, sermaye maddesi veya esas sözleşme tadilini pay sahiplerine ve düzenleyici otoriteye açıklamak için hazırlanmış belge."
		}
		return "Sermaye işlemi, esas sözleşme tadili, ihraç belgesi veya SPK onay sürecini açıklamak için yayımlanmış resmi KAP eki."
	case "general_assembly":
		return "Genel kurul çağrısı, gündemi, tutanağı, bilgilendirme dokümanı veya hazır bulunanlar listesini pay sahiplerine açıklamak için hazırlanmış belge."
	case "fund_usage":
		return "Halka arz veya ihraçtan sağlanan fonun nerede ve ne amaçla kullanıldığını yatırımcıya açıklamak için hazırlanmış fon kullanım raporu."
	case "governance":
		return "Kurumsal yönetim, komite, politika, beyan veya sürdürülebilirlik uygulamalarını yatırımcıya açıklamak için yayımlanmış belge."
	default:
		return "KAP ekinin amacı dosya adı ve bildirim başlığından sınırlı olarak anlaşılabiliyor; detaylı amaç için metin okunabilirliği gerekli."
	}
}

func detectTopics(text string) []string {
	normalized := normalize(text)
	groups := []struct {
		label    string
		patterns []string
	}{
		{"hasılat / ciro", []string{"hasilat", "ciro", "satis geliri", "satış geliri"}},
		{"net kar / zarar", []string{"net kar", "net donem kari", "net zarar", "donem zarari", "donem kari"}},
		{"nakit akışı", []string{"nakit akis", "isletme faaliyetlerinden", "serbest nakit", "nakit ve nakit"}},
		{"borç / finansman", []string{"borc", "kredi", "finansman", "net borc", "yukumluluk"}},
		{"yatırım / capex", []string{"yatirim", "maddi duran varlik", "capex", "makine", "tesis"}},
		{"sermaye işlemi", []string{"sermaye", "bedelli", "bedelsiz", "kayitli sermaye", "tadil"}},
		{"temettü / kar dağıtımı", []string{"temettu", "kar payi", "kar dagitim"}},
		{"pay geri alım", []string{"geri alim", "pay geri"}},
		{"denetim görüşü", []string{"olumlu gorus", "sinirli olumlu", "sartli gorus", "bagimsiz denetim"}},
		{"kilit denetim konuları", []string{"kilit denetim", "key audit"}},
		{"risk yönetimi", []string{"risk", "kur riski", "faiz riski", "likidite riski"}},
		{"ilişkili taraf", []string{"iliskili taraf", "ilisikli taraf"}},
		{"halka arz varsayımları", []string{"halka arz", "varsayim", "fiyatinin belirlenmesinde"}},
		{"kurumsal yönetim", []string{"kurumsal yonetim", "komite", "politika", "surdurulebilirlik"}},
		{"genel kurul", []string{"genel kurul", "gundem", "hazir bulunan", "tutanak"}},
	}
	topics := []string{}
	for _, group := range groups {
		for _, pattern := range group.patterns {
			if strings.Contains(normalized, normalize(pattern)) {
				topics = append(topics, group.label)
				break
			}
		}
	}
	if len(topics) > 8 {
		return topics[:8]
	}
	return topics
}

func summarizeContent(doc Document, text string) string {
	topics := detectTopics(text)
	if len(topics) == 0 {
		return doc.CategoryLabel + " içeriği okunabildi; belirgin finansal anahtar başlık sınırlı bulundu."
	}
	return doc.CategoryLabel + " içeriğinde öne çıkan başlıklar: " + strings.Join(topics, ", ") + "."
}

func inferReportImpact(doc Document, text string) string {
	normalized := normalize(text)
	switch doc.Category {
	case "financial_report":
		return "Değerleme modelindeki bilanço, gelir tablosu, nakit akışı, borç, özsermaye ve dönem güncelliği için ana resmi kanıt olarak kullanılır."
	case "activity_report":
		return "Moat, büyüme kalitesi, yönetim anlatısı, operasyonel riskler ve gelecek dönem varsayımlarını yorumlamak için nitel kanıt sağlar."
	case "audit_report":
		if containsAny(normalized, "sartli gorus", "olumsuz gorus", "gorus bildirmekten kacinma", "sinirli olumlu") {
			return "Denetçi çekincesi veya olumsuz ifade varsa değerleme güveni düşürülmeli ve raporda kırmızı bayrak olarak gösterilmelidir."
		}
		return "Denetim görüşü finansal verinin güvenilirlik kontrolüne katkı sağlar; denetçi çekincesi yoksa veri güvenini destekler."
	case "valuation_report":
		return "Halka arz/değerleme varsayımlarının gerçekleşmesi, sapması ve fiyat gerekçesi kontrol edilerek içsel değer varsayımlarıyla karşılaştırılır."
	case "dividend":
		return "Sermaye tahsisi, nakit dönüşü ve hissedar getirisi skorunda kullanılır."
	case "buyback":
		return "Yönetimin fiyatı ucuz görüp görmediğine, sermaye tahsisine ve pay başına değer etkisine ilişkin sinyal sağlar."
	case "capital_action":
		return "Pay sulanması, sermaye tavanı, ihraç, bedelli/bedelsiz ve ortaklık yapısı risklerini değerleme modeline bağlar."
	case "general_assembly":
		return "Temettü, yönetim, denetçi seçimi, sermaye işlemleri ve pay sahibi onaylarını yönetişim riski için izler."
	case "fund_usage":
		return "Halka arz/ihraç fonlarının planlanan yatırımlara gidip gitmediğini ve yatırım disiplinini kontrol eder."
	case "governance":
		return "Yönetim kalitesi, komite yapısı, politika disiplini ve sürdürülebilirlik riskleri için yönetişim kanıtı sağlar."
	default:
		return "Doğrudan değerleme girdisi sınırlı; haber/kurumsal olay bağlamı olarak izlenir."
	}
}

func classifyDocument(fileName string, meta detailMetadata) (string, []string) {
	text := normalize(fileName + " " + meta.Title + " " + meta.Summary + " " + meta.DisclosureType + " " + meta.Category)
	evidence := []string{}
	add := func(reason string) {
		evidence = append(evidence, reason)
	}
	switch {
	case containsAny(text, "bagimsiz denetim", "bagimsizlik", "denetim raporu", " bdr", "_bdr", "bdr "):
		add("Dosya/bildirim bağımsız denetim veya BDR ifadesi içeriyor.")
		return "audit_report", evidence
	case containsAny(text, "faaliyet raporu"):
		add("Dosya/bildirim faaliyet raporu ifadesi içeriyor.")
		return "activity_report", evidence
	case strings.EqualFold(meta.DisclosureType, "FR") || strings.EqualFold(meta.DisclosureClass, "FR") || containsAny(text, "finansal rapor", "finansal tablo", "bilanco", "gelir tablosu", "nakit akim"):
		add("KAP bildirimi finansal rapor/finansal tablo sınıfında.")
		return "financial_report", evidence
	case containsAny(text, "halka arz fiyat", "fiyatinin belirlenmesinde", "varsayim", "varsayimlara", "degerleme", "degerlendirme raporu"):
		add("Dosya/bildirim değerleme, halka arz fiyatı veya varsayım raporu ifadesi içeriyor.")
		return "valuation_report", evidence
	case containsAny(text, "geri alim", "geri alın", "pay geri"):
		add("Dosya/bildirim pay geri alım ifadesi içeriyor.")
		return "buyback", evidence
	case containsAny(text, "kar payi", "kar dagitim", "temettu", "temettü"):
		add("Dosya/bildirim kar payı/temettü ifadesi içeriyor.")
		return "dividend", evidence
	case containsAny(text, "genel kurul", "hazir bulunan", "tutanak", "ilan metni"):
		add("Dosya/bildirim genel kurul dokümanı ifadesi içeriyor.")
		return "general_assembly", evidence
	case containsAny(text, "fon kullanim", "fon kullanım"):
		add("Dosya/bildirim fon kullanım raporu ifadesi içeriyor.")
		return "fund_usage", evidence
	case containsAny(text, "sermaye", "esas sozlesme", "esas sözlesme", "tadil", "ihrac belgesi", "ihraç belgesi", "spk onayli"):
		add("Dosya/bildirim sermaye, esas sözleşme, tadil veya ihraç ifadesi içeriyor.")
		return "capital_action", evidence
	case containsAny(text, "kurumsal yonetim", "komite", "surdurulebilirlik", "sorumluluk beyan", "politika", "bagis", "bagış", "ucretlendirme", "ücretlendirme"):
		add("Dosya/bildirim yönetişim, politika veya sürdürülebilirlik ifadesi içeriyor.")
		return "governance", evidence
	default:
		if meta.Title != "" {
			add("KAP başlığı: " + meta.Title)
		} else {
			add("Dosya adı sınıflandırma için kullanıldı.")
		}
		return "other", evidence
	}
}

func sortedCategorySummaries(values map[string]*CategorySummary) []CategorySummary {
	out := make([]CategorySummary, 0, len(values))
	for _, value := range values {
		out = append(out, *value)
	}
	sort.Slice(out, func(i, j int) bool {
		pi := categoryPriority(out[i].Category)
		pj := categoryPriority(out[j].Category)
		if pi != pj {
			return pi < pj
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func sortDocuments(docs []Document) {
	sort.Slice(docs, func(i, j int) bool {
		if !docs[i].PublishedAt.Equal(docs[j].PublishedAt) {
			return docs[i].PublishedAt.After(docs[j].PublishedAt)
		}
		if docs[i].Year != docs[j].Year {
			return docs[i].Year > docs[j].Year
		}
		if categoryPriority(docs[i].Category) != categoryPriority(docs[j].Category) {
			return categoryPriority(docs[i].Category) < categoryPriority(docs[j].Category)
		}
		if docs[i].DisclosureIndex != docs[j].DisclosureIndex {
			return docs[i].DisclosureIndex > docs[j].DisclosureIndex
		}
		return docs[i].FileName < docs[j].FileName
	})
}

func limitDocuments(docs []Document, limit int) []Document {
	if limit <= 0 || len(docs) <= limit {
		return docs
	}
	selected := make([]Document, 0, limit)
	seen := map[string]bool{}
	add := func(doc Document) bool {
		if seen[doc.Path] {
			return false
		}
		seen[doc.Path] = true
		selected = append(selected, doc)
		return len(selected) == limit
	}
	for _, category := range []string{
		"financial_report",
		"activity_report",
		"audit_report",
		"valuation_report",
		"capital_action",
		"dividend",
		"buyback",
		"fund_usage",
		"governance",
		"general_assembly",
	} {
		if doc, ok := bestDocumentForCategory(docs, category); ok {
			if add(doc) {
				return selected
			}
		}
	}
	for _, doc := range docs {
		if add(doc) {
			return selected
		}
	}
	return selected
}

func bestDocumentForCategory(docs []Document, category string) (Document, bool) {
	best := Document{}
	found := false
	bestScore := -1_000_000
	for _, doc := range docs {
		if doc.Category != category {
			continue
		}
		score := documentSelectionScore(doc)
		if !found || score > bestScore || (score == bestScore && doc.PublishedAt.After(best.PublishedAt)) || (score == bestScore && doc.PublishedAt.Equal(best.PublishedAt) && doc.Year > best.Year) {
			best = doc
			bestScore = score
			found = true
		}
	}
	return best, found
}

func documentSelectionScore(doc Document) int {
	text := normalize(doc.FileName + " " + doc.DisclosureTitle + " " + doc.DisclosureType + " " + doc.DisclosureClass)
	score := 0
	if !doc.PublishedAt.IsZero() {
		score += doc.PublishedAt.Year() - 2000
	} else {
		score += doc.Year - 2000
	}
	switch doc.Category {
	case "financial_report":
		if strings.EqualFold(doc.DisclosureType, "FR") || strings.EqualFold(doc.DisclosureClass, "FR") {
			score += 80
		}
		if containsAny(text, "finansal rapor") {
			score += 50
		}
		if containsAny(text, "herhangi bir otoriteye mali tablo") {
			score -= 45
		}
	case "activity_report":
		if containsAny(text, "faaliyet raporu") {
			score += 70
		}
	case "audit_report":
		if containsAny(text, "bdr", "bagimsiz denetim", "denetim raporu") {
			score += 70
		}
	case "valuation_report":
		if containsAny(text, "halka arz fiyat", "varsayim", "degerlendirme raporu") {
			score += 70
		}
	case "capital_action":
		if containsAny(text, "sermaye", "esas sozlesme", "ihrac") {
			score += 40
		}
	}
	return score
}

func missingKeyDocuments(categories map[string]*CategorySummary) []string {
	missing := []string{}
	if categories["financial_report"] == nil && categories["audit_report"] == nil {
		missing = append(missing, "finansal rapor / bağımsız denetim eki")
	}
	if categories["activity_report"] == nil {
		missing = append(missing, "faaliyet raporu eki")
	}
	return missing
}

func coverageScore(categories map[string]*CategorySummary) float64 {
	score := 0.0
	if categories["financial_report"] != nil {
		score += 35
	}
	if categories["activity_report"] != nil {
		score += 25
	}
	if categories["audit_report"] != nil {
		score += 15
	}
	if categories["valuation_report"] != nil {
		score += 8
	}
	if categories["capital_action"] != nil {
		score += 7
	}
	if categories["dividend"] != nil || categories["buyback"] != nil {
		score += 5
	}
	if categories["governance"] != nil || categories["general_assembly"] != nil {
		score += 5
	}
	if score > 100 {
		return 100
	}
	return score
}

func buildSummary(report Report) string {
	if report.TotalFiles == 0 {
		return "KAP PDF/ek bulunamadı; değerleme yalnızca fiyat ve bilanço JSON verisiyle sınırlı kalır."
	}
	parts := []string{
		fmt.Sprintf("%d KAP eki tarandı", report.TotalFiles),
		fmt.Sprintf("%d PDF", report.PDFCount),
		fmt.Sprintf("%d belgenin içeriği analiz edildi", report.ContentAnalyzed),
		fmt.Sprintf("%d belge okunabilir metin verdi", report.ContentReadable),
		llmSummaryPart(report.LLMAnalysis),
		fmt.Sprintf("belge güven skoru %.0f/100", report.CoverageScore),
	}
	found := []string{}
	for _, category := range []string{"financial_report", "activity_report", "audit_report", "valuation_report"} {
		for _, item := range report.Categories {
			if item.Category == category {
				found = append(found, item.Label)
				break
			}
		}
	}
	if len(found) > 0 {
		parts = append(parts, "bulunan ana belgeler: "+strings.Join(found, ", "))
	}
	if len(report.MissingKeyDocuments) > 0 {
		parts = append(parts, "eksik ana belgeler: "+strings.Join(report.MissingKeyDocuments, ", "))
	}
	return strings.Join(parts, "; ") + "."
}

func llmSummaryPart(report LLMReport) string {
	switch report.Status {
	case "complete":
		return fmt.Sprintf("LLM %d belgeyi gerçek çağrıyla analiz etti", report.DocumentsAnalyzed)
	case "partial":
		return fmt.Sprintf("LLM kısmi çalıştı: %d/%d belge", report.DocumentsAnalyzed, report.DocumentsRequested)
	case "missing_api_key", "missing_model", "missing_endpoint", "failed":
		return "LLM analizi yapılmadı: " + report.Status
	default:
		return "LLM analizi yapılmadı: yapılandırılmadı"
	}
}

func categoryPriority(category string) int {
	switch category {
	case "financial_report":
		return 1
	case "activity_report":
		return 2
	case "audit_report":
		return 3
	case "valuation_report":
		return 4
	case "dividend":
		return 5
	case "buyback":
		return 6
	case "capital_action":
		return 7
	case "general_assembly":
		return 8
	case "fund_usage":
		return 9
	case "governance":
		return 10
	default:
		return 99
	}
}

func containsAny(text string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(text, normalize(pattern)) {
			return true
		}
	}
	return false
}

func normalize(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer(
		"ı", "i",
		"İ", "i",
		"ğ", "g",
		"Ğ", "g",
		"ü", "u",
		"Ü", "u",
		"ş", "s",
		"Ş", "s",
		"ö", "o",
		"Ö", "o",
		"ç", "c",
		"Ç", "c",
	)
	return strings.Join(strings.Fields(replacer.Replace(text)), " ")
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
	return ""
}

func firstInt(values map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case int:
			return typed
		case string:
			parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
			return parsed
		}
	}
	return 0
}

func parseKAPTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006.01.02 15:04:05", time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
