package extraction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"hissebot/internal/confidence"
	"hissebot/internal/domain/documents"
	"hissebot/internal/domain/kapextract"
	"hissebot/internal/repositories"

	pdf "github.com/ledongthuc/pdf"
	htmlparser "golang.org/x/net/html"
)

type Processor struct {
	Repository          repositories.DocumentRepository
	EquitiesDir         string
	MaxCharsPerDocument int
	Now                 func() time.Time
}

type Options struct {
	Ticker           string
	Limit            int
	IncludeNonLatest bool
}

type documentExtraction struct {
	Summary  kapextract.DocumentSummary
	Pages    []kapextract.DocumentPage
	Blocks   []kapextract.TextBlock
	Tables   []kapextract.DocumentTable
	Facts    []kapextract.FinancialFact
	People   []kapextract.PersonExtraction
	Events   []kapextract.CorporateEvent
	Assets   []kapextract.TrackedAsset
	Chains   []kapextract.EvidenceChain
	Reviews  []kapextract.ReviewItem
	Warnings []string
	Status   documents.ExtractionStatus
}

func (p Processor) ProcessBatch(ctx context.Context, opts Options) (kapextract.BatchExtractionResult, error) {
	if p.Repository == nil {
		return kapextract.BatchExtractionResult{}, errors.New("document repository is required")
	}
	if strings.TrimSpace(p.EquitiesDir) == "" {
		return kapextract.BatchExtractionResult{}, errors.New("equities dir is required")
	}
	now := p.now()
	docs, err := p.Repository.ListDocuments(ctx, documents.NormalizeTicker(opts.Ticker))
	if err != nil {
		return kapextract.BatchExtractionResult{}, err
	}
	byTicker := map[string][]documents.DocumentMetadata{}
	for _, doc := range docs {
		if !opts.IncludeNonLatest && !doc.IsLatestVersion {
			continue
		}
		if doc.Ticker == "" {
			continue
		}
		byTicker[doc.Ticker] = append(byTicker[doc.Ticker], doc)
	}
	tickers := make([]string, 0, len(byTicker))
	for ticker := range byTicker {
		tickers = append(tickers, ticker)
	}
	sort.Strings(tickers)
	batch := kapextract.BatchExtractionResult{GeneratedAt: now}
	for _, ticker := range tickers {
		if err := ctx.Err(); err != nil {
			return batch, err
		}
		result, err := p.processTicker(ctx, ticker, byTicker[ticker], opts)
		if err != nil {
			return batch, err
		}
		batch.Tickers++
		batch.DocumentsScanned += len(byTicker[ticker])
		batch.DocumentsProcessed += len(result.Documents)
		batch.FinancialFacts += len(result.FinancialFacts)
		batch.ReviewItems += len(result.HumanReviewQueue)
		if result.OutputPath != "" {
			batch.OutputPaths = append(batch.OutputPaths, result.OutputPath)
		}
		batch.Results = append(batch.Results, result)
	}
	return batch, nil
}

func (p Processor) processTicker(ctx context.Context, ticker string, docs []documents.DocumentMetadata, opts Options) (kapextract.ExtractionResult, error) {
	now := p.now()
	sort.Slice(docs, func(i, j int) bool {
		if !docs[i].DisclosureDate.Equal(docs[j].DisclosureDate) {
			return docs[i].DisclosureDate.After(docs[j].DisclosureDate)
		}
		return docs[i].OriginalFilename < docs[j].OriginalFilename
	})
	result := kapextract.ExtractionResult{
		Ticker:          ticker,
		GeneratedAt:     now,
		CompanyInfoCard: p.companyInfoCard(ticker, now),
	}
	processed := 0
	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if opts.Limit > 0 && processed >= opts.Limit {
			break
		}
		extracted := p.processDocument(doc)
		processed++
		result.Documents = append(result.Documents, extracted.Summary)
		result.Pages = append(result.Pages, extracted.Pages...)
		result.TextBlocks = append(result.TextBlocks, extracted.Blocks...)
		result.Tables = append(result.Tables, extracted.Tables...)
		result.FinancialFacts = append(result.FinancialFacts, extracted.Facts...)
		result.People = append(result.People, extracted.People...)
		result.CorporateEvents = append(result.CorporateEvents, extracted.Events...)
		result.TrackedAssets = append(result.TrackedAssets, extracted.Assets...)
		result.EvidenceChains = append(result.EvidenceChains, extracted.Chains...)
		result.HumanReviewQueue = append(result.HumanReviewQueue, extracted.Reviews...)
		result.Warnings = append(result.Warnings, extracted.Warnings...)
		doc.ExtractionStatus = extracted.Status
		doc.ReviewRequired = doc.ReviewRequired || extracted.Status == documents.ExtractionStatusNeedsOCR || extracted.Status == documents.ExtractionStatusReviewRequired
		doc.UpdatedAt = now
		if err := p.Repository.SaveDocument(ctx, doc); err != nil {
			return result, err
		}
	}
	result.FundamentalAnalysis = buildFundamentalAnalysis(result)
	path, err := p.writeResult(result)
	if err != nil {
		return result, err
	}
	result.OutputPath = path
	return result, nil
}

func (p Processor) processDocument(doc documents.DocumentMetadata) documentExtraction {
	now := p.now()
	out := documentExtraction{
		Summary: kapextract.DocumentSummary{
			DocumentID:       doc.DocumentID,
			Ticker:           doc.Ticker,
			DocumentType:     string(doc.DocumentType),
			LocalFilePath:    doc.LocalFilePath,
			OriginalFilename: doc.OriginalFilename,
			Checksum:         doc.Checksum,
			ProcessedAt:      now,
		},
		Status: documents.ExtractionStatusPending,
	}
	text, method, warning, err := p.extractText(doc)
	if err != nil {
		out.Status = documents.ExtractionStatusReviewRequired
		out.Summary.ExtractionStatus = string(out.Status)
		out.Summary.ReviewRequired = true
		out.Reviews = append(out.Reviews, reviewForDocument(doc, "document_text_extraction_failed", err.Error(), 0.1, now))
		return out
	}
	if warning != "" {
		out.Warnings = append(out.Warnings, warning)
	}
	if method == "ocr_required" {
		out.Status = documents.ExtractionStatusNeedsOCR
		out.Pages = append(out.Pages, kapextract.DocumentPage{
			DocumentID: doc.DocumentID, PageNumber: 1, OCRRequired: true, CreatedAt: now,
		})
		out.Reviews = append(out.Reviews, reviewForDocument(doc, "ocr_required", "Belge taranmış görüntü veya OCR gerektiren formatta.", 0.05, now))
		out.Summary.ExtractionStatus = string(out.Status)
		out.Summary.ReviewRequired = true
		return out
	}
	text = cleanText(limitRunes(text, p.maxChars()))
	if len([]rune(text)) < 20 {
		out.Status = documents.ExtractionStatusReviewRequired
		out.Pages = append(out.Pages, kapextract.DocumentPage{
			DocumentID: doc.DocumentID, PageNumber: 1, TextExtracted: false, OCRRequired: doc.DocumentType == documents.DocumentTypePDF, CreatedAt: now,
		})
		out.Reviews = append(out.Reviews, reviewForDocument(doc, "text_too_short", "Belgeden güvenilir metin çıkarılamadı.", 0.1, now))
		out.Summary.ExtractionStatus = string(out.Status)
		out.Summary.ReviewRequired = true
		return out
	}
	out.Status = documents.ExtractionStatusTextReady
	out.Pages = append(out.Pages, kapextract.DocumentPage{
		DocumentID:    doc.DocumentID,
		PageNumber:    1,
		TextExtracted: true,
		Checksum:      hashString(text),
		CreatedAt:     now,
	})
	out.Blocks = splitTextBlocks(doc, text, method, now)
	out.Tables = detectTables(doc, text, method, now)
	out.Facts = extractFinancialFacts(doc, out.Blocks, out.Tables, now)
	out.People = extractPeople(doc, out.Blocks, now)
	out.Events = extractEvents(doc, out.Blocks, now)
	out.Assets, out.Chains = trackAssets(doc, out.Events, now)
	for _, fact := range out.Facts {
		if fact.ReviewRequired {
			out.Reviews = append(out.Reviews, kapextract.ReviewItem{
				ReviewID:        stableID("review", fact.FactID, "financial_fact"),
				Ticker:          doc.Ticker,
				SubjectType:     "financial_fact",
				SubjectID:       fact.FactID,
				Reason:          "Finansal fact tablo/satır/sütun veya validation eksikliği nedeniyle insan denetimi gerektiriyor.",
				Source:          fact.Source,
				ConfidenceScore: fact.ConfidenceScore,
				CreatedAt:       now,
			})
		}
	}
	for _, event := range out.Events {
		if event.ReviewRequired {
			out.Reviews = append(out.Reviews, kapextract.ReviewItem{
				ReviewID:        stableID("review", event.EventID, "corporate_event"),
				Ticker:          doc.Ticker,
				SubjectType:     "corporate_event",
				SubjectID:       event.EventID,
				Reason:          "Olay metinden aday olarak çıkarıldı; tutar, karşı taraf veya varlık eşleşmesi kesin değil.",
				Source:          event.Source,
				ConfidenceScore: event.ConfidenceScore,
				CreatedAt:       now,
			})
		}
	}
	out.Summary.ExtractionStatus = string(out.Status)
	out.Summary.ReviewRequired = len(out.Reviews) > 0
	return out
}

func (p Processor) extractText(doc documents.DocumentMetadata) (string, string, string, error) {
	switch doc.DocumentType {
	case documents.DocumentTypePDF:
		text, note := extractPDFText(doc.LocalFilePath, p.maxChars())
		if text == "" {
			return "", "ocr_required", note, nil
		}
		return text, "pdf_text", note, nil
	case documents.DocumentTypeHTML:
		raw, err := os.ReadFile(doc.LocalFilePath)
		if err != nil {
			return "", "", "", err
		}
		return htmlText(raw), "html_text", "", nil
	case documents.DocumentTypeXML, documents.DocumentTypeXBRL:
		raw, err := os.ReadFile(doc.LocalFilePath)
		if err != nil {
			return "", "", "", err
		}
		return xmlText(raw), "xml_text", "", nil
	case documents.DocumentTypeOCRImage:
		return "", "ocr_required", "OCR adapter yapılandırılmadan görüntü belgeden metin çıkarılmadı.", nil
	default:
		return "", "", "", fmt.Errorf("unsupported document type: %s", doc.DocumentType)
	}
}

func (p Processor) maxChars() int {
	if p.MaxCharsPerDocument > 0 {
		return p.MaxCharsPerDocument
	}
	return 200000
}

func (p Processor) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func (p Processor) writeResult(result kapextract.ExtractionResult) (string, error) {
	if result.Ticker == "" {
		return "", nil
	}
	path := filepath.Join(p.EquitiesDir, result.Ticker, "kap", "extraction", "extraction_result.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmpPath, path)
}

func (p Processor) companyInfoCard(ticker string, now time.Time) kapextract.CompanyInfoCard {
	card := kapextract.CompanyInfoCard{
		Ticker:            ticker,
		LastUpdatedAt:     now,
		ConfidenceScore:   0.25,
		ReviewRequired:    true,
		LastUpdatedSource: "kap_extraction",
	}
	path := filepath.Join(p.EquitiesDir, ticker, "kap.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return card
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return card
	}
	card.CompanyName = firstString(data, "kapMemberTitle")
	card.LegalName = card.CompanyName
	card.KAPCompanyID = firstString(data, "kapMemberOid")
	card.Headquarters = firstString(data, "cityName")
	card.TradeRegistryNumber = firstString(data, "tradeRegNo")
	card.TaxOffice = firstString(data, "taxOffice")
	card.TaxNumber = firstString(data, "taxNo")
	card.PaidInCapital = firstFloat(data, "paidCapital")
	card.RegisteredCapitalCeiling = firstFloat(data, "kayitliSermayeTavani")
	card.FoundationDate = firstString(data, "tradeRegDate")
	card.IndependentAuditor = firstString(data, "relatedMemberTitle")
	card.LastUpdatedSource = path
	card.ConfidenceScore = 0.82
	card.ReviewRequired = false
	return card
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
	text := cleanText(string(raw))
	if len([]rune(text)) < 40 {
		return "", "PDF metni çok kısa; belge taranmış görüntü olabilir."
	}
	return text, ""
}

func htmlText(raw []byte) string {
	node, err := htmlparser.Parse(bytes.NewReader(raw))
	if err != nil {
		return cleanText(stripTags(string(raw)))
	}
	var parts []string
	var walk func(*htmlparser.Node)
	walk = func(n *htmlparser.Node) {
		if n == nil {
			return
		}
		if n.Type == htmlparser.TextNode {
			text := cleanText(n.Data)
			if text != "" {
				parts = append(parts, text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return cleanText(strings.Join(parts, " "))
}

func xmlText(raw []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	parts := []string{}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return cleanText(string(raw))
		}
		if chars, ok := token.(xml.CharData); ok {
			text := cleanText(string(chars))
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return cleanText(strings.Join(parts, " "))
}

func splitTextBlocks(doc documents.DocumentMetadata, text string, method string, now time.Time) []kapextract.TextBlock {
	paragraphs := splitParagraphs(text)
	blocks := make([]kapextract.TextBlock, 0, len(paragraphs))
	score := confidence.Score(confidence.EvidenceSignals{PDFText: method == "pdf_text", StructuredSource: method == "xml_text" || method == "html_text", HasDocumentID: true, HasPage: true})
	for i, paragraph := range paragraphs {
		if paragraph == "" {
			continue
		}
		blocks = append(blocks, kapextract.TextBlock{
			BlockID:          stableID("block", doc.DocumentID, strconv.Itoa(i), paragraph),
			DocumentID:       doc.DocumentID,
			PageNumber:       1,
			BlockIndex:       i + 1,
			BlockType:        "paragraph",
			Text:             paragraph,
			ExtractionMethod: method,
			ConfidenceScore:  score,
			ReviewRequired:   confidence.ReviewRequired(score),
			CreatedAt:        now,
		})
	}
	return blocks
}

func splitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	out := []string{}
	var current strings.Builder
	flush := func() {
		value := cleanText(current.String())
		current.Reset()
		if value == "" {
			return
		}
		if len([]rune(value)) > 1600 {
			runes := []rune(value)
			for len(runes) > 0 {
				take := 1600
				if len(runes) < take {
					take = len(runes)
				}
				out = append(out, string(runes[:take]))
				runes = runes[take:]
			}
			return
		}
		out = append(out, value)
	}
	for _, line := range lines {
		line = cleanText(line)
		if line == "" {
			if current.Len() >= 120 {
				flush()
			}
			continue
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(line)
		if current.Len() >= 1000 || strings.HasSuffix(line, ".") || strings.HasSuffix(line, ":") {
			flush()
		}
	}
	flush()
	if len(out) == 0 && strings.TrimSpace(text) != "" {
		out = append(out, cleanText(text))
	}
	return out
}

func detectTables(doc documents.DocumentMetadata, text string, method string, now time.Time) []kapextract.DocumentTable {
	lines := strings.Split(text, "\n")
	tables := []kapextract.DocumentTable{}
	current := [][]string{}
	flush := func() {
		if len(current) < 2 {
			current = nil
			return
		}
		index := len(tables) + 1
		score := confidence.Score(confidence.EvidenceSignals{HasDocumentID: true, HasPage: true, HasTable: true, HasRowColumn: true, PDFText: method == "pdf_text", StructuredSource: method == "html_text" || method == "xml_text", ValidationWarning: true})
		tables = append(tables, kapextract.DocumentTable{
			TableID:          stableID("table", doc.DocumentID, strconv.Itoa(index), fmt.Sprint(current)),
			DocumentID:       doc.DocumentID,
			PageNumber:       1,
			TableIndex:       index,
			Rows:             current,
			ExtractionMethod: method + "_heuristic_table",
			ConfidenceScore:  score,
			ReviewRequired:   true,
			CreatedAt:        now,
		})
		current = nil
	}
	for _, line := range lines {
		fields := splitTableLine(line)
		if len(fields) >= 3 {
			current = append(current, fields)
			continue
		}
		flush()
	}
	flush()
	return tables
}

func splitTableLine(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if strings.Contains(line, "\t") {
		return cleanFields(strings.Split(line, "\t"))
	}
	re := regexp.MustCompile(`\s{2,}`)
	fields := cleanFields(re.Split(line, -1))
	if len(fields) >= 3 {
		return fields
	}
	return nil
}

func cleanFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = cleanText(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

var financialSynonyms = map[string]string{
	"dönen varlıklar":                        "Dönen varlıklar",
	"nakit ve nakit benzerleri":              "Nakit ve nakit benzerleri",
	"finansal yatırımlar":                    "Finansal yatırımlar",
	"ticari alacaklar":                       "Ticari alacaklar",
	"diğer alacaklar":                        "Diğer alacaklar",
	"stoklar":                                "Stoklar",
	"duran varlıklar":                        "Duran varlıklar",
	"maddi duran varlıklar":                  "Maddi duran varlıklar",
	"maddi olmayan duran varlıklar":          "Maddi olmayan duran varlıklar",
	"kullanım hakkı varlıkları":              "Kullanım hakkı varlıkları",
	"yatırım amaçlı gayrimenkuller":          "Yatırım amaçlı gayrimenkuller",
	"toplam varlıklar":                       "Toplam varlıklar",
	"kısa vadeli yükümlülükler":              "Kısa vadeli yükümlülükler",
	"uzun vadeli yükümlülükler":              "Uzun vadeli yükümlülükler",
	"finansal borçlar":                       "Finansal borçlar",
	"kiralama yükümlülükleri":                "Kiralama yükümlülükleri",
	"ticari borçlar":                         "Ticari borçlar",
	"özkaynaklar":                            "Özkaynaklar",
	"ana ortaklığa ait özkaynaklar":          "Ana ortaklığa ait özkaynaklar",
	"ödenmiş sermaye":                        "Ödenmiş sermaye",
	"hasılat":                                "Hasılat",
	"satışların maliyeti":                    "Satışların maliyeti",
	"brüt kar":                               "Brüt kar",
	"genel yönetim giderleri":                "Genel yönetim giderleri",
	"pazarlama giderleri":                    "Pazarlama giderleri",
	"araştırma geliştirme giderleri":         "Araştırma geliştirme giderleri",
	"esas faaliyet karı":                     "Esas faaliyet karı",
	"finansman gelirleri":                    "Finansman gelirleri",
	"finansman giderleri":                    "Finansman giderleri",
	"vergi öncesi kar":                       "Vergi öncesi kar",
	"net dönem karı":                         "Net dönem karı",
	"amortisman ve itfa giderleri":           "Amortisman ve itfa giderleri",
	"esas faaliyetlerden nakit akışı":        "Esas faaliyetlerden nakit akışı",
	"yatırım faaliyetlerinden nakit akışı":   "Yatırım faaliyetlerinden nakit akışı",
	"finansman faaliyetlerinden nakit akışı": "Finansman faaliyetlerinden nakit akışı",
	"serbest nakit akışı":                    "Serbest nakit akışı",
	"capex":                                  "CAPEX",
	"favök":                                  "FAVÖK",
}

var numberPattern = regexp.MustCompile(`-?\d{1,3}(?:\.\d{3})+(?:,\d+)?|-?\d+(?:,\d+)?`)

func extractFinancialFacts(doc documents.DocumentMetadata, blocks []kapextract.TextBlock, tables []kapextract.DocumentTable, now time.Time) []kapextract.FinancialFact {
	facts := []kapextract.FinancialFact{}
	for _, block := range blocks {
		lines := splitFactLines(block.Text)
		for _, line := range lines {
			normalized, original, ok := financialItemForLine(line)
			if !ok {
				continue
			}
			value, ok := parseAmount(line)
			if !ok {
				continue
			}
			source := sourceFromBlock(doc, block, line, 0.45)
			score := confidence.Score(confidence.EvidenceSignals{HasDocumentID: true, HasPage: true, PDFText: block.ExtractionMethod == "pdf_text", StructuredSource: block.ExtractionMethod == "xml_text", HasRowColumn: true, ValidationWarning: true, Inferred: true})
			fact := kapextract.FinancialFact{
				FactID:             stableID("fact", doc.DocumentID, normalized, line, fmt.Sprintf("%.4f", value)),
				CompanyID:          doc.CompanyID,
				Ticker:             doc.Ticker,
				Period:             doc.Period,
				FiscalYear:         doc.FiscalYear,
				StatementType:      inferStatementType(normalized),
				LineItemOriginal:   original,
				LineItemNormalized: normalized,
				Value:              value,
				Currency:           inferCurrency(line),
				Unit:               inferUnit(line),
				AccountingStandard: "UNKNOWN",
				Source:             source,
				ConfidenceScore:    score,
				ValidationStatus:   kapextract.ValidationUnknown,
				ReviewRequired:     true,
				CreatedAt:          now,
			}
			if fact.Validate() == nil {
				facts = append(facts, fact)
			}
		}
	}
	_ = tables
	return dedupeFacts(facts)
}

func splitFactLines(text string) []string {
	text = strings.ReplaceAll(text, "  ", "\n")
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = cleanText(part)
		if len([]rune(part)) >= 8 {
			out = append(out, part)
		}
	}
	return out
}

func financialItemForLine(line string) (string, string, bool) {
	lower := strings.ToLower(line)
	for synonym, normalized := range financialSynonyms {
		if strings.Contains(lower, synonym) {
			return normalized, synonym, true
		}
	}
	return "", "", false
}

func parseAmount(line string) (float64, bool) {
	matches := numberPattern.FindAllString(line, -1)
	if len(matches) == 0 {
		return 0, false
	}
	candidates := make([]float64, 0, len(matches))
	for _, match := range matches {
		value, ok := parseTRNumber(match)
		if !ok {
			continue
		}
		if value >= 1900 && value <= 2100 && len(matches) > 1 {
			continue
		}
		candidates = append(candidates, value)
	}
	if len(candidates) == 0 {
		return 0, false
	}
	return candidates[len(candidates)-1], true
}

func parseTRNumber(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	value = strings.ReplaceAll(value, ".", "")
	value = strings.ReplaceAll(value, ",", ".")
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func inferCurrency(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "usd") || strings.Contains(lower, "abd doları") || strings.Contains(lower, "dolar"):
		return "USD"
	case strings.Contains(lower, "eur") || strings.Contains(lower, "euro"):
		return "EUR"
	default:
		return "TRY"
	}
}

func inferUnit(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "milyon"):
		return "million_try"
	case strings.Contains(lower, "bin"):
		return "thousand_try"
	default:
		return "TRY"
	}
}

func inferStatementType(item string) kapextract.StatementType {
	lower := strings.ToLower(item)
	switch {
	case strings.Contains(lower, "hasılat"), strings.Contains(lower, "kar"), strings.Contains(lower, "gider"), strings.Contains(lower, "gelir"), strings.Contains(lower, "favök"):
		return kapextract.StatementIncomeStatement
	case strings.Contains(lower, "nakit akışı"), strings.Contains(lower, "capex"):
		return kapextract.StatementCashFlow
	case strings.Contains(lower, "ödenmiş sermaye"), strings.Contains(lower, "geçmiş yıllar"):
		return kapextract.StatementEquityStatement
	default:
		return kapextract.StatementBalanceSheet
	}
}

func dedupeFacts(facts []kapextract.FinancialFact) []kapextract.FinancialFact {
	seen := map[string]struct{}{}
	out := make([]kapextract.FinancialFact, 0, len(facts))
	for _, fact := range facts {
		if _, ok := seen[fact.FactID]; ok {
			continue
		}
		seen[fact.FactID] = struct{}{}
		out = append(out, fact)
	}
	return out
}

func extractPeople(doc documents.DocumentMetadata, blocks []kapextract.TextBlock, now time.Time) []kapextract.PersonExtraction {
	people := []kapextract.PersonExtraction{}
	for _, block := range blocks {
		lower := strings.ToLower(block.Text)
		if !hasPersonRoleContext(lower) {
			continue
		}
		name := candidateName(block.Text)
		if name == "" {
			continue
		}
		score := confidence.Score(confidence.EvidenceSignals{HasDocumentID: true, HasPage: true, PDFText: block.ExtractionMethod == "pdf_text", Inferred: true})
		people = append(people, kapextract.PersonExtraction{
			PersonID:        stableID("person", doc.DocumentID, name, block.Text),
			FullName:        name,
			NormalizedName:  normalizeName(name),
			Title:           inferPersonTitle(block.Text),
			Role:            inferPersonRole(block.Text),
			Source:          sourceFromBlock(doc, block, block.Text, score),
			ConfidenceScore: score,
			ReviewRequired:  true,
			CreatedAt:       now,
		})
	}
	return people
}

func hasPersonRoleContext(lower string) bool {
	for _, phrase := range []string{
		"yönetim kurulu başkanı",
		"yönetim kurulu üyesi",
		"bağımsız yönetim kurulu",
		"genel müdür",
		"icra kurulu",
		"denetim komitesi",
		"riskin erken saptanması komitesi",
		"kurumsal yönetim komitesi",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func candidateName(text string) string {
	fields := strings.Fields(text)
	window := []string{}
	for _, field := range fields {
		cleaned := strings.Trim(field, ".,;:()[]")
		if len([]rune(cleaned)) < 2 {
			window = nil
			continue
		}
		runes := []rune(cleaned)
		if unicode.IsUpper(runes[0]) {
			window = append(window, cleaned)
			if len(window) >= 2 && len(window) <= 4 {
				candidate := strings.Join(window, " ")
				if !looksLikeInstitution(candidate) {
					return candidate
				}
			}
			if len(window) > 4 {
				window = window[1:]
			}
		} else {
			window = nil
		}
	}
	return ""
}

func looksLikeInstitution(value string) bool {
	upper := strings.ToUpper(value)
	for _, token := range []string{"KURULU", "KOMİTESİ", "ŞİRKET", "ANONİM", "GENEL", "OLAĞAN"} {
		if strings.Contains(upper, token) {
			return true
		}
	}
	for _, token := range []string{"TÜRK", "SİLAHLI", "KUVVET", "ASELSAN", "ELEKTRONİK", "TOPLANTI", "BAŞKAN", "YÖNETİM", "KURUMSAL"} {
		if strings.Contains(upper, token) {
			return true
		}
	}
	return false
}

func normalizeName(value string) string {
	return strings.ToUpper(cleanText(value))
}

func inferPersonTitle(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "başkan"):
		return "Başkan"
	case strings.Contains(lower, "genel müdür"):
		return "Genel Müdür"
	case strings.Contains(lower, "üye"):
		return "Üye"
	default:
		return ""
	}
}

func inferPersonRole(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "yönetim kurulu") {
		return "board_of_directors"
	}
	if strings.Contains(lower, "genel müdür") {
		return "executive_management"
	}
	if strings.Contains(lower, "komite") {
		return "committee"
	}
	return "unknown"
}

func extractEvents(doc documents.DocumentMetadata, blocks []kapextract.TextBlock, now time.Time) []kapextract.CorporateEvent {
	events := []kapextract.CorporateEvent{}
	for _, block := range blocks {
		eventType, ok := eventTypeForText(block.Text)
		if !ok {
			continue
		}
		amount, hasAmount := parseAmount(block.Text)
		var amountPtr *float64
		if hasAmount {
			amountPtr = &amount
		}
		score := confidence.Score(confidence.EvidenceSignals{HasDocumentID: true, HasPage: true, PDFText: block.ExtractionMethod == "pdf_text", Inferred: true})
		events = append(events, kapextract.CorporateEvent{
			EventID:         stableID("event", doc.DocumentID, eventType, block.Text),
			CompanyID:       doc.CompanyID,
			Ticker:          doc.Ticker,
			EventType:       eventType,
			EventTitle:      limitRunes(block.Text, 90),
			Description:     block.Text,
			Amount:          amountPtr,
			Currency:        inferCurrency(block.Text),
			Source:          sourceFromBlock(doc, block, block.Text, score),
			ConfidenceScore: score,
			ReviewRequired:  true,
			CreatedAt:       now,
		})
	}
	return events
}

func eventTypeForText(text string) (string, bool) {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "arsa") && (strings.Contains(lower, "alım") || strings.Contains(lower, "satın")):
		return "land_purchase", true
	case hasAssetSaleContext(lower):
		return "asset_sale", true
	case strings.Contains(lower, "yatırım amaçlı gayrimenkul"):
		return "investment_property_transfer", true
	case strings.Contains(lower, "ipotek"):
		return "mortgage", true
	case strings.Contains(lower, "rehin") || strings.Contains(lower, "teminat"):
		return "pledge", true
	case strings.Contains(lower, "dava") || strings.Contains(lower, "hukuki"):
		return "lawsuit", true
	case strings.Contains(lower, "birleşme"):
		return "merger", true
	case strings.Contains(lower, "bölünme"):
		return "spin_off", true
	case strings.Contains(lower, "sermaye artır"):
		return "capital_increase", true
	case strings.Contains(lower, "kar payı") || strings.Contains(lower, "temettü"):
		return "dividend", true
	case strings.Contains(lower, "yönetim kurulu") && strings.Contains(lower, "seçil"):
		return "management_change", true
	default:
		return "", false
	}
}

func hasAssetSaleContext(lower string) bool {
	if !(strings.Contains(lower, "satış") || strings.Contains(lower, "satıldı") || strings.Contains(lower, "devredildi")) {
		return false
	}
	for _, assetWord := range []string{"varlık", "taşınmaz", "arsa", "arazi", "bina", "gayrimenkul", "iştirak", "bağlı ortaklık", "tesis"} {
		if strings.Contains(lower, assetWord) {
			return true
		}
	}
	return false
}

func trackAssets(doc documents.DocumentMetadata, events []kapextract.CorporateEvent, now time.Time) ([]kapextract.TrackedAsset, []kapextract.EvidenceChain) {
	assets := []kapextract.TrackedAsset{}
	chains := []kapextract.EvidenceChain{}
	for _, event := range events {
		assetType := ""
		switch event.EventType {
		case "land_purchase":
			assetType = "land"
		case "asset_sale":
			assetType = "other"
		case "investment_property_transfer":
			assetType = "investment_property"
		case "mortgage", "pledge":
			assetType = "other"
		}
		if assetType == "" {
			continue
		}
		status := "unknown"
		if event.EventType == "land_purchase" {
			status = "likely_active"
		}
		if event.EventType == "asset_sale" {
			status = "likely_sold"
		}
		assetID := stableID("asset", doc.DocumentID, event.EventID, assetType)
		asset := kapextract.TrackedAsset{
			AssetID:         assetID,
			CompanyID:       doc.CompanyID,
			Ticker:          doc.Ticker,
			AssetName:       event.EventTitle,
			AssetType:       assetType,
			Status:          status,
			RelatedEvents:   []string{event.EventID},
			SourceChain:     []string{event.Source.SourceDocumentID},
			ConfidenceScore: event.ConfidenceScore,
			ReviewRequired:  true,
			CreatedAt:       now,
		}
		if event.Amount != nil {
			asset.AcquisitionCost = event.Amount
			asset.Currency = event.Currency
		}
		assets = append(assets, asset)
		chains = append(chains, kapextract.EvidenceChain{
			EvidenceChainID:      stableID("evidence_chain", assetID),
			Subject:              asset.AssetName,
			CompanyID:            doc.CompanyID,
			Ticker:               doc.Ticker,
			InitialEvent:         event.Source,
			CurrentStatus:        status,
			FinalConfidenceScore: event.ConfidenceScore,
			ReviewRequired:       true,
			CreatedAt:            now,
		})
	}
	return assets, chains
}

func buildFundamentalAnalysis(result kapextract.ExtractionResult) kapextract.FundamentalAnalysis {
	analysis := kapextract.FundamentalAnalysis{
		Ticker:          result.Ticker,
		FinancialRatios: map[string]any{},
		Metadata: map[string]any{
			"documents":       len(result.Documents),
			"text_blocks":     len(result.TextBlocks),
			"financial_facts": len(result.FinancialFacts),
			"review_items":    len(result.HumanReviewQueue),
		},
	}
	for _, fact := range result.FinancialFacts {
		analysis.Sources = append(analysis.Sources, fact.Source)
	}
	if len(analysis.Sources) > 10 {
		analysis.Sources = analysis.Sources[:10]
	}
	switch {
	case len(result.FinancialFacts) == 0:
		analysis.Summary = "KAP belgelerinden kaynaklı metin çıkarıldı; deterministik finansal fact üretilemedi."
		analysis.Risks = append(analysis.Risks, "Normalize finansal veri için tablo/XBRL çıkarımı veya insan onayı gerekiyor.")
		analysis.ConfidenceScore = 0.25
		analysis.ReviewRequired = true
	default:
		analysis.Summary = "KAP belgelerinden kaynaklı finansal fact adayları üretildi; kesin temel analiz için validation ve insan denetimi gerekiyor."
		analysis.Strengths = append(analysis.Strengths, "Kaynak belge, sayfa ve metin referansı korunuyor.")
		analysis.Risks = append(analysis.Risks, "PDF metninden çıkarılan aday değerler tablo satır/sütun doğrulaması olmadan kesin finansal veri sayılmaz.")
		total := 0.0
		for _, fact := range result.FinancialFacts {
			total += fact.ConfidenceScore
			if fact.ReviewRequired {
				analysis.ReviewRequired = true
			}
		}
		analysis.ConfidenceScore = total / float64(len(result.FinancialFacts))
	}
	if len(result.HumanReviewQueue) > 0 {
		analysis.ReviewRequired = true
		analysis.RedFlags = append(analysis.RedFlags, map[string]any{
			"code":  "human_review_required",
			"count": len(result.HumanReviewQueue),
		})
	}
	for _, asset := range result.TrackedAssets {
		analysis.AssetQualityFindings = append(analysis.AssetQualityFindings, map[string]any{
			"asset_id":         asset.AssetID,
			"asset_type":       asset.AssetType,
			"status":           asset.Status,
			"review_required":  asset.ReviewRequired,
			"confidence_score": asset.ConfidenceScore,
		})
	}
	for _, person := range result.People {
		analysis.ManagementFindings = append(analysis.ManagementFindings, map[string]any{
			"person_id":        person.PersonID,
			"full_name":        person.FullName,
			"role":             person.Role,
			"review_required":  person.ReviewRequired,
			"confidence_score": person.ConfidenceScore,
		})
	}
	return analysis
}

func sourceFromBlock(doc documents.DocumentMetadata, block kapextract.TextBlock, text string, score float64) kapextract.SourceRef {
	return kapextract.SourceRef{
		SourceDocumentID: doc.DocumentID,
		SourceSystem:     string(doc.SourceSystem),
		Ticker:           doc.Ticker,
		LocalFilePath:    doc.LocalFilePath,
		SourcePage:       block.PageNumber,
		SourceRow:        text,
		SourceText:       text,
		ConfidenceScore:  score,
	}
}

func reviewForDocument(doc documents.DocumentMetadata, reason string, message string, score float64, now time.Time) kapextract.ReviewItem {
	source := kapextract.SourceRef{
		SourceDocumentID: doc.DocumentID,
		SourceSystem:     string(doc.SourceSystem),
		Ticker:           doc.Ticker,
		LocalFilePath:    doc.LocalFilePath,
		ConfidenceScore:  score,
	}
	return kapextract.ReviewItem{
		ReviewID:        stableID("review", doc.DocumentID, reason, message),
		Ticker:          doc.Ticker,
		SubjectType:     "document",
		SubjectID:       doc.DocumentID,
		Reason:          reason + ": " + message,
		Source:          source,
		ConfidenceScore: score,
		CreatedAt:       now,
	}
}

func firstString(values map[string]any, key string) string {
	if value, ok := values[key]; ok && value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func firstFloat(values map[string]any, key string) float64 {
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		parsed, _ := parseTRNumber(typed)
		return parsed
	default:
		return 0
	}
}

func cleanText(value string) string {
	value = stdhtml.UnescapeString(value)
	replacer := strings.NewReplacer("\u00a0", " ", "\r", "\n", "\t", " ")
	value = replacer.Replace(value)
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = strings.Join(strings.Fields(lines[i]), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func stripTags(value string) string {
	var out strings.Builder
	inTag := false
	for _, r := range value {
		switch r {
		case '<':
			inTag = true
			out.WriteRune(' ')
		case '>':
			inTag = false
			out.WriteRune(' ')
		default:
			if !inTag {
				out.WriteRune(r)
			}
		}
	}
	return out.String()
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

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func stableID(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "_" + hex.EncodeToString(sum[:])[:24]
}
