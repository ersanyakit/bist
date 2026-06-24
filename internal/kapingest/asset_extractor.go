package kapingest

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/util"
)

var assetDocumentTypes = map[string]bool{
	DocumentInterimActivityReport: true,
	DocumentAnnualReport:          true,
	DocumentFinancialStatement:    true,
	DocumentValuationReport:       true,
	DocumentMaterialEvent:         true,
	DocumentUnknown:               true,
}

var assetHeadingKeywords = []string{
	"gayrimenkuller",
	"gayrimenkul projeleri",
	"gayrimenkule dayali haklar",
	"portfoyde yer alan varliklar",
	"portfoy tablosu",
	"kiraya verilenler",
	"ekspertiz degeri",
	"kdv haric",
	"kdv dahil",
	"aylik kira bedeli",
	"kira ekspertiz degeri",
	"sigorta degeri",
	"para ve sermaye piyasasi araclari",
	"istirakler",
}

var assetCueKeywords = []string{
	"gayrimenkul", "portfoy", "ekspertiz", "kdv haric", "kdv dahil", "kira",
	"arsa", "arazi", "otel", "is merkezi", "ofis", "dukkan", "magaza", "fabrika",
	"kullanim hakki", "ust hakki", "proje", "insaat", "ada", "parsel", "m2", "metrekare",
	"istirak", "bagli ortaklik", "para ve sermaye piyasasi",
}

var cityNamesTR = []string{
	"adana", "adiyaman", "afyonkarahisar", "agri", "amasya", "ankara", "antalya", "artvin",
	"aydin", "balikesir", "bilecik", "bingol", "bitlis", "bolu", "burdur", "bursa",
	"canakkale", "cankiri", "corum", "denizli", "diyarbakir", "edirne", "elazig", "erzincan",
	"erzurum", "eskisehir", "gaziantep", "giresun", "gumushane", "hakkari", "hatay", "isparta",
	"mersin", "istanbul", "izmir", "kars", "kastamonu", "kayseri", "kirklareli", "kirsehir",
	"kocaeli", "konya", "kutahya", "malatya", "manisa", "kahramanmaras", "mardin", "mugla",
	"mus", "nevsehir", "nigde", "ordu", "rize", "sakarya", "samsun", "siirt", "sinop",
	"sivas", "tekirdag", "tokat", "trabzon", "tunceli", "sanliurfa", "usak", "van", "yozgat",
	"zonguldak", "aksaray", "bayburt", "karaman", "kirikkale", "batman", "sirnak", "bartin",
	"ardahan", "igdir", "yalova", "karabuk", "kilis", "osmaniye", "duzce",
}

var (
	twoSpaceSplitRE        = regexp.MustCompile(`\s{2,}`)
	dateRE                 = regexp.MustCompile(`\b(\d{1,2})[./-](\d{1,2})[./-](\d{4})\b`)
	periodRE               = regexp.MustCompile(`(?i)\b(20\d{2})\s*[/.-]?\s*(Q[1-4]|[1-4]\.? ceyrek|[1-4]\.? çeyrek|03|06|09|12)\b`)
	areaRE                 = regexp.MustCompile(`(?i)(\d{1,3}(?:[.\s]\d{3})*(?:,\d+)?|\d+(?:,\d+)?)\s*(?:m2|m²|m\^2|metrekare)`)
	footnoteLineRE         = regexp.MustCompile(`(?i)^\(?\d+\)?\s*[-–]`)
	trailingParenNumberRE  = regexp.MustCompile(`^\d+\)?$`)
	trailingPercentValueRE = regexp.MustCompile(`\s+\d+(?:,\d+)?$`)
	assetUnitCodeRE        = regexp.MustCompile(`(?i)^[A-Z]{1,4}\d{0,4}[-/]\d{1,4}[A-Z]?$`)
	assetValuationCodeRE   = regexp.MustCompile(`(?i)^[A-Z]{2,6}-\d{6,}(?:\s+.+)?$`)
	moneyCurrencyRE        = regexp.MustCompile(`(?i)(TRY|TL|USD|EUR|₺|\$|€)`)
	parcelREs              = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b\d+\s*ada\s*,?\s*[\d/]+(?:\s*(?:,|ve)\s*[\d/]+)*\s*(?:numaral[ıi]|no\.?\s*lu|nolu)?\s*parsel(?:ler(?:de|i)?|de|deki|i)?\b`),
		regexp.MustCompile(`(?i)\b\d+\s*ada\s+[\w/-]+\s*parsel\w*\b`),
		regexp.MustCompile(`(?i)\b\d+\s*ada\s*,?\s*\d+\s*(?:no\.?lu\s*)?parsel\w*\b`),
		regexp.MustCompile(`(?i)\bpafta\s*[:=]?\s*[\w/-]+\s+ada\s*[:=]?\s*\d+\s+parsel\s*[:=]?\s*[\w/-]+`),
		regexp.MustCompile(`(?i)\b\d+\s*pafta\s+\d+\s*ada\s+\d+\s*(?:no\.?lu\s*)?parsel\w*\b`),
		regexp.MustCompile(`(?i)\bada\s*[/:-]\s*parsel\s*[:=]?\s*\d+\s*/\s*[\w/-]+`),
		regexp.MustCompile(`(?i)\bada\s*[:=]?\s*\d+[^,\n;]{0,40}\bparsel\s*[:=]?\s*[\w/-]+`),
		regexp.MustCompile(`(?i)\bparsel\s*[:=]?\s*[\w/-]+[^,\n;]{0,40}\bada\s*[:=]?\s*\d+`),
	}
	amountRE           = regexp.MustCompile(`(?i)(?:TRY|TL|USD|EUR|₺|\$|€)\s*-?(?:\d{1,3}(?:[.\s]\d{3})+(?:,\d+)?|\d{1,3}(?:,\d{3})+(?:\.\d+)?|\d+(?:[,.]\d+)?)|-?(?:\d{1,3}(?:[.\s]\d{3})+(?:,\d+)?|\d{1,3}(?:,\d{3})+(?:\.\d+)?|\d+(?:[,.]\d+)?)\s*(?:TRY|TL|USD|EUR|₺|\$|€)|-?(?:\d{1,3}(?:[.\s]\d{3})+(?:,\d+)?|\d{1,3}(?:,\d{3})+(?:\.\d+)?)`)
	tableAmountTokenRE = regexp.MustCompile(`^-?\d{1,3}(?:\.\d{3})+(?:,\d+)?$|^-?\d{1,3}(?:,\d{3})+(?:\.\d+)?$|^-?\d+(?:,\d+)?$`)
)

type assetLine struct {
	Page int
	Text string
}

func ExtractAssetEvents(doc RawDocument) []AssetEvent {
	if strings.TrimSpace(doc.Text) == "" {
		return nil
	}
	if !RawDocumentStructuredExtractionUsable(doc) {
		return nil
	}
	if !RawDocumentAnalysisUsable(doc) || (doc.AIResolved && containsRawWarning(doc.Warnings, lowQualityStructuredRescueWarning)) {
		doc = RawDocumentForStructuredRescue(doc)
	}
	if !assetDocumentTypes[doc.DocumentTypeGuess] && !containsAssetCue(doc.Text) {
		return nil
	}
	lines := documentAssetLines(doc.Text)
	events := []AssetEvent{}
	seen := map[string]bool{}
	section := ""
	sectionTTL := 0
	for i, line := range lines {
		clean := normalizeAssetLine(line.Text)
		if clean == "" {
			continue
		}
		if isAssetHeading(clean) {
			section = clean
			sectionTTL = 80
			continue
		}
		context := assetContext(lines, i, section)
		if !isAssetCandidateLine(clean, context, sectionTTL > 0) {
			if sectionTTL > 0 {
				sectionTTL--
			}
			continue
		}
		event, ok := parseAssetEvent(doc, line.Text, context, line.Page)
		if !ok || event.Confidence < 0.30 {
			if sectionTTL > 0 {
				sectionTTL--
			}
			continue
		}
		key := assetEventKey(event)
		if !seen[key] {
			seen[key] = true
			events = append(events, event)
		}
		if sectionTTL > 0 {
			sectionTTL--
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Confidence == events[j].Confidence {
			return events[i].AssetName < events[j].AssetName
		}
		return events[i].Confidence > events[j].Confidence
	})
	return events
}

func ExtractPortfolioSummaryAndNotes(doc RawDocument) ([]PortfolioSummarySnapshot, []ValuationNote) {
	if strings.TrimSpace(doc.Text) == "" {
		return nil, nil
	}
	if !RawDocumentStructuredExtractionUsable(doc) {
		return nil, nil
	}
	if !RawDocumentAnalysisUsable(doc) || (doc.AIResolved && containsRawWarning(doc.Warnings, lowQualityStructuredRescueWarning)) {
		doc = RawDocumentForStructuredRescue(doc)
	}
	lines := documentAssetLines(doc.Text)
	snapshots := []PortfolioSummarySnapshot{}
	notes := []ValuationNote{}
	seenSummary := map[string]bool{}
	seenNote := map[string]bool{}
	for i, line := range lines {
		clean := normalizeAssetLine(line.Text)
		if clean == "" {
			continue
		}
		context := assetContext(lines, i, "")
		if isPortfolioSummaryLine(clean, context) {
			snapshot, ok := parsePortfolioSummarySnapshot(doc, clean, context)
			if ok {
				key := snapshot.SourceFile + "|" + snapshot.Snippet
				if !seenSummary[key] {
					seenSummary[key] = true
					snapshots = append(snapshots, snapshot)
				}
			}
			continue
		}
		if isValuationMethodLine(clean) {
			note := ValuationNote{
				Ticker:       strings.ToUpper(strings.TrimSpace(doc.Ticker)),
				SourceFile:   doc.FilePath,
				SHA256:       doc.SHA256,
				DocumentDate: extractDateString(firstNonEmptyAsset(doc.FileName, context)),
				Period:       extractPeriodString(firstNonEmptyAsset(doc.FileName, context)),
				NoteType:     "valuation_method",
				Snippet:      truncateAssetSnippet(context, 500),
				SourceReferences: []AssetSourceReference{
					{Page: intPtr(line.Page), Snippet: truncateAssetSnippet(context, 500)},
				},
				Confidence: 0.70,
			}
			if note.Ticker == "" {
				note.Ticker = ExtractTicker("", doc.FilePath)
			}
			key := note.SourceFile + "|" + note.Snippet
			if !seenNote[key] {
				seenNote[key] = true
				notes = append(notes, note)
			}
		}
	}
	return snapshots, notes
}

func parsePortfolioSummarySnapshot(doc RawDocument, line, context string) (PortfolioSummarySnapshot, bool) {
	var excl *float64
	var incl *float64
	if allowsRealEstateVATPortfolioTotal(line, context) {
		excl = tableAmountByHeader(context, []string{"kdv haric", "kdv hariç"}, "TRY")
		incl = tableAmountByHeader(context, []string{"kdv dahil"}, "TRY")
	}
	book := tableAmountByHeader(context, []string{"portfoy degeri", "portföy değeri", "toplam deger", "toplam değer", "defter degeri", "defter değeri"}, "TRY")
	if excl == nil && allowsRealEstateVATPortfolioTotal(line, context) {
		excl = amountNearKeywords(context, []string{"kdv haric", "kdv hariç"}, "TRY")
	}
	if incl == nil && allowsRealEstateVATPortfolioTotal(line, context) {
		incl = amountNearKeywords(context, []string{"kdv dahil"}, "TRY")
	}
	amounts := extractMoneyAmounts(line)
	if excl == nil && incl == nil && book == nil && len(amounts) > 0 {
		best := 0.0
		for _, amount := range amounts {
			if amount.Currency == "" || amount.Currency == "TRY" {
				if amount.Value > best {
					best = amount.Value
				}
			}
		}
		if best > 0 {
			book = floatPtr(best)
		}
	}
	if excl == nil && incl == nil && book == nil {
		return PortfolioSummarySnapshot{}, false
	}
	period := ""
	if parsed := extractPeriodString(firstNonEmptyAsset(doc.FileName, context)); parsed != nil {
		period = *parsed
	}
	documentDate := ""
	if parsed := extractDateString(firstNonEmptyAsset(doc.FileName, context)); parsed != nil {
		documentDate = *parsed
	}
	return PortfolioSummarySnapshot{
		Period:                         period,
		DocumentDate:                   documentDate,
		TotalRealEstateValueExclVATTRY: excl,
		TotalRealEstateValueInclVATTRY: incl,
		TotalBookValueTRY:              book,
		SourceFile:                     doc.FilePath,
		Snippet:                        truncateAssetSnippet(context, 500),
	}, true
}

func allowsRealEstateVATPortfolioTotal(line, context string) bool {
	slug := util.SlugTR(line + " " + context)
	if !(slugContains(slug, "kdv haric") || slugContains(slug, "kdv dahil")) {
		return false
	}
	return slugContains(slug, "portfoyde yer alan varliklar") ||
		slugContains(slug, "gayrimenkuller toplam") ||
		slugContains(slug, "toplam gayrimenkul") ||
		slugContains(slug, "gayrimenkul projeleri")
}

func documentAssetLines(text string) []assetLine {
	out := []assetLine{}
	pages := strings.Split(text, "\f")
	for pageIndex, page := range pages {
		for _, line := range strings.Split(page, "\n") {
			out = append(out, assetLine{Page: pageIndex + 1, Text: line})
		}
	}
	return out
}

func assetContext(lines []assetLine, index int, section string) string {
	start := index - 2
	if start < 0 {
		start = 0
	}
	end := index + 3
	if end > len(lines) {
		end = len(lines)
	}
	parts := []string{}
	if section != "" {
		parts = append(parts, section)
	}
	for i := start; i < end; i++ {
		parts = append(parts, strings.TrimSpace(lines[i].Text))
	}
	return strings.Join(parts, "\n")
}

func isAssetCandidateLine(line, context string, inSection bool) bool {
	slugLine := util.SlugTR(line)
	slugContext := util.SlugTR(context)
	if len([]rune(line)) > 650 {
		return false
	}
	if isNonAssetLine(line) || isMostlyNumeric(line) || isGenericAssetHeader(line) || isAssetExplanationLine(line) {
		return false
	}
	hasCue := false
	for _, keyword := range assetCueKeywords {
		if strings.Contains(slugContext, util.SlugTR(keyword)) {
			hasCue = true
			break
		}
	}
	if !hasCue && !inSection {
		return false
	}
	hasSpecificAssetWord := slugContains(slugLine, "arsa") ||
		slugContains(slugLine, "otel") ||
		slugContains(slugLine, "is merkezi") ||
		slugContains(slugLine, "dukkan") ||
		slugContains(slugLine, "fabrika") ||
		slugContains(slugLine, "proje") ||
		slugContains(slugLine, "ust hakki") ||
		slugContains(slugLine, "kullanim hakki") ||
		slugContains(slugLine, "istirak") ||
		slugContains(slugLine, "para ve sermaye")
	hasMoney := len(extractMoneyAmounts(line)) > 0
	hasArea := areaRE.MatchString(line)
	hasParcel := extractParcelInfo(line) != ""
	hasCity := cityInText(line) != ""
	hasRent := slugContains(slugLine, "kira") || slugContains(slugContext, "kira")
	hasPhysicalAnchor := hasArea || hasParcel || hasCity
	if hasSpecificAssetWord && hasStrongLineAssetAnchor(line) && (hasMoney || hasPhysicalAnchor || hasRent || inSection) {
		return true
	}
	if inSection && hasPhysicalAnchor && (hasMoney || hasRent) && hasLetters(line) {
		return true
	}
	return false
}

func parseAssetEvent(doc RawDocument, line, context string, page int) (AssetEvent, bool) {
	lineAssetName := extractAssetName(line, line)
	assetName := lineAssetName
	if assetName == "" {
		assetName = extractAssetName(line, context)
		if assetName != "" && !strings.Contains(util.SlugTR(line), util.SlugTR(assetName)) && !assetNameHasPortfolioIdentity(line) {
			return AssetEvent{}, false
		}
	}
	lineAssetType := inferAssetType(line)
	assetType := lineAssetType
	if assetType == "other" {
		assetType = inferAssetType(context)
	}
	location, city, district := extractLocation(line)
	if location == "" && city == "" && district == "" {
		location, city, district = extractLocation(context)
	}
	location, city, district = sanitizeAssetLocationFields(assetName, location, city, district)
	area := extractAreaM2(line)
	if area == nil {
		area = extractAreaM2(context)
	}
	parcel := extractParcelInfo(line)
	if parcel == "" {
		parcel = extractParcelInfo(context)
	}
	ownership := inferOwnershipType(context)
	status := inferAssetStatus(context)
	expertiseDate := extractDateString(context)
	documentDate := extractDateString(firstNonEmptyAsset(doc.FileName, context))
	period := extractPeriodString(firstNonEmptyAsset(doc.FileName, context))
	rental := extractRentalInfo(context)
	lineExcl, lineIncl, lineBook, lineValueFlags := extractAssetValues(line)
	excl, incl, book, valueFlags := extractAssetValues(context)
	if lineExcl != nil {
		excl = lineExcl
	}
	if lineIncl != nil {
		incl = lineIncl
	}
	if lineBook != nil {
		book = lineBook
	}
	valueFlags = dedupeStrings(append(valueFlags, lineValueFlags...))
	confidence := assetConfidence(doc, assetName, assetType, location, area, parcel, excl, incl, book, rental)
	if strings.TrimSpace(assetName) == "" || assetNameLooksGeneric(assetName) || !assetNameHasPortfolioIdentity(assetName) {
		return AssetEvent{}, false
	}
	evidenceCount := assetEvidenceCount(assetName, assetType, location, city, district, area, parcel, expertiseDate, excl, incl, book, rental)
	if evidenceCount < 3 {
		return AssetEvent{}, false
	}
	lineEvidenceCount := assetLineEvidenceCount(line, lineAssetName)
	if lineEvidenceCount < 2 && !hasStrongLineAssetAnchor(line) {
		return AssetEvent{}, false
	}
	if assetLineHasMultipleKnownAssets(line) {
		return AssetEvent{}, false
	}
	riskFlags := valueFlags
	if evidenceCount == 3 {
		riskFlags = append(riskFlags, "minimum_asset_evidence")
	}
	rescue := containsRawWarning(doc.Warnings, lowQualityStructuredRescueWarning) || RawDocumentStructuredRescueUsable(doc)
	aiResolved := doc.AIResolved || containsRawWarning(doc.Warnings, structuredRescueAIResolvedWarning) || containsRawWarning(doc.Warnings, aiResolvedByStructuredEvidence)
	if !RawDocumentAnalysisUsable(doc) || containsRawWarning(doc.Warnings, "low_text_quality_possible_scanned_pdf") || rescue {
		riskFlags = append(riskFlags, "low_text_quality_possible_scanned_pdf")
		if rescue {
			if aiResolved {
				riskFlags = append(riskFlags, lowQualityStructuredRescueWarning, structuredRescueAIResolvedWarning, aiResolvedByStructuredEvidence)
				if confidence > 0.82 {
					confidence = 0.82
				}
			} else if confidence > 0.72 {
				confidence = 0.72
			}
			if !aiResolved {
				riskFlags = append(riskFlags, lowQualityStructuredRescueWarning, structuredRescueReviewWarning)
			}
		} else if confidence > 0.45 {
			confidence = 0.45
		}
	}
	if assetType == "other" {
		riskFlags = append(riskFlags, "asset_type_uncertain")
	}
	if location == "" && parcel == "" {
		riskFlags = append(riskFlags, "location_or_parcel_missing")
	}
	event := AssetEvent{
		Ticker:                   strings.ToUpper(strings.TrimSpace(doc.Ticker)),
		CompanyName:              extractCompanyName(doc.Text),
		SourceFile:               doc.FilePath,
		SHA256:                   doc.SHA256,
		DocumentDate:             documentDate,
		Period:                   period,
		AssetName:                assetName,
		AssetType:                assetType,
		Location:                 location,
		City:                     city,
		District:                 district,
		AreaM2:                   area,
		ParcelInfo:               parcel,
		OwnershipType:            ownership,
		Status:                   status,
		ExpertiseDate:            expertiseDate,
		ExpertiseValueExclVATTRY: excl,
		ExpertiseValueInclVATTRY: incl,
		BookValueTRY:             book,
		RentalInfo:               rental,
		IncomeRelevance:          incomeRelevance(rental, assetType, book, excl, incl),
		RiskFlags:                dedupeStrings(riskFlags),
		OpportunityFlags:         opportunityFlags(assetType, rental, status),
		SourceReferences: []AssetSourceReference{
			{Page: intPtr(page), Snippet: truncateAssetSnippet(context, 500)},
		},
		Confidence: clampAsset(confidence, 0, 1),
	}
	if event.Ticker == "" {
		event.Ticker = ExtractTicker("", doc.FilePath)
	}
	return event, true
}

func extractAssetName(line, context string) string {
	cells := splitAssetCells(line)
	for _, cell := range cells {
		clean := cleanAssetNameCell(cell)
		if prefix := assetNameBeforeDescription(clean); prefix != "" {
			clean = prefix
		}
		clean = trimAssetNameDescriptionMarkers(clean)
		if isLikelyAssetName(clean) {
			return clean
		}
	}
	clean := cleanAssetNameCell(line)
	if prefix := assetNameBeforeDescription(clean); prefix != "" {
		clean = prefix
	}
	clean = trimAssetNameDescriptionMarkers(clean)
	if isLikelyAssetName(clean) {
		return clean
	}
	for _, cell := range splitAssetCells(context) {
		clean = cleanAssetNameCell(cell)
		if prefix := assetNameBeforeDescription(clean); prefix != "" {
			clean = prefix
		}
		clean = trimAssetNameDescriptionMarkers(clean)
		if isLikelyAssetName(clean) {
			return clean
		}
	}
	return ""
}

func trimAssetNameDescriptionMarkers(value string) string {
	clean := strings.TrimSpace(value)
	lower := strings.ToLower(clean)
	for _, marker := range []string{
		" kdv ", " ekspertiz ", " aylik ", " yıllık ", " yillik ", " kira ",
		" ada ", " parsel ", " toplam ", " toplamda ", " bugünkü ", " bugunku ",
		" proje değeri", " proje degeri", " gider ", " gelir ", " olup", " olan ",
		" yer alan ", " üzerinde ", " uzerinde ", " için", " icin", " bedelle",
		" tarihli", " raporuna", " doğrultusunda", " dogrultusunda",
	} {
		idx := strings.Index(lower, marker)
		if idx > 8 {
			return strings.TrimSpace(clean[:idx])
		}
	}
	runes := []rune(clean)
	for idx, r := range runes {
		if r >= '0' && r <= '9' && idx > 6 {
			tail := strings.TrimSpace(string(runes[idx:]))
			if trailingParenNumberRE.MatchString(tail) {
				continue
			}
			prefix := strings.TrimSpace(string(runes[:idx]))
			if inferAssetType(prefix) != "other" && !assetNameLooksGeneric(prefix) {
				return prefix
			}
			break
		}
	}
	return clean
}

func assetNameBeforeDescription(value string) string {
	for _, sep := range []string{":", " - "} {
		idx := strings.Index(value, sep)
		if idx <= 3 {
			continue
		}
		prefix := strings.TrimSpace(value[:idx])
		if len([]rune(prefix)) > 4 && len([]rune(prefix)) <= 90 && inferAssetType(prefix) != "other" && !assetNameLooksGeneric(prefix) {
			return prefix
		}
	}
	return ""
}

func splitAssetCells(line string) []string {
	raw := twoSpaceSplitRE.Split(strings.ReplaceAll(line, "\t", "  "), -1)
	if len(raw) <= 1 {
		raw = strings.FieldsFunc(line, func(r rune) bool { return r == '|' || r == ';' })
	}
	out := []string{}
	for _, cell := range raw {
		cell = strings.TrimSpace(cell)
		if cell != "" {
			out = append(out, cell)
		}
	}
	return out
}

func cleanAssetNameCell(value string) string {
	value = normalizeAssetLine(value)
	replacer := strings.NewReplacer(
		"Gayrimenkulün Adı", "", "Gayrimenkulun Adi", "", "Varlık Adı", "",
		"Varlik Adi", "", "Proje Adı", "", "Proje Adi", "", "Ticaret Unvanı", "",
		"Ticaret Unvani", "", "Açıklama", "", "Aciklama", "",
	)
	value = replacer.Replace(value)
	value = strings.Trim(value, " :-")
	if len([]rune(value)) > 140 {
		value = truncateAssetSnippet(value, 140)
	}
	return value
}

func isLikelyAssetName(value string) bool {
	if value == "" || len([]rune(value)) < 4 || !hasLetters(value) || isMostlyNumeric(value) {
		return false
	}
	trimmed := strings.TrimSpace(value)
	if strings.Contains(trimmed, "=") || strings.Contains(trimmed, "%") {
		return false
	}
	if trimmed != "" {
		first := []rune(trimmed)[0]
		if first >= '0' && first <= '9' {
			return false
		}
		if first == '*' || first == '•' || first == '-' || first == '–' {
			return false
		}
	}
	if assetNameLooksGeneric(value) {
		return false
	}
	if assetNameLooksLocationOnly(value) {
		return false
	}
	slug := util.SlugTR(value)
	for _, blocked := range []string{
		"toplam", "sayfa", "dipnot", "tablo", "tutar", "deger", "kdv", "ekspertiz",
		"aylik kira", "yillik kira", "sigorta degeri", "portfoy toplam",
	} {
		blockedSlug := util.SlugTR(blocked)
		if slug == blockedSlug || strings.HasPrefix(slug, blockedSlug) {
			return false
		}
	}
	return true
}

func assetNameLooksGeneric(value string) bool {
	slug := util.SlugTR(value)
	generic := map[string]bool{
		util.SlugTR("gayrimenkuller"): true, util.SlugTR("gayrimenkul projeleri"): true, util.SlugTR("kiraya verilenler"): true,
		util.SlugTR("ekspertiz degeri"): true, util.SlugTR("kdv haric"): true, util.SlugTR("kdv dahil"): true, util.SlugTR("varliklar"): true,
		util.SlugTR("portfoyde yer alan varliklar"): true, util.SlugTR("para ve sermaye piyasasi araclari"): true,
		util.SlugTR("arsa"): true, util.SlugTR("arazi"): true, util.SlugTR("otel"): true, util.SlugTR("fabrika"): true,
		util.SlugTR("dukkan"): true, util.SlugTR("magaza"): true, util.SlugTR("gayrimenkul"): true, util.SlugTR("proje"): true,
		util.SlugTR("fabrika binasi"): true,
		util.SlugTR("arsa alani"):     true, util.SlugTR("arsa yuz olcumu"): true, util.SlugTR("alani"): true, util.SlugTR("vasfi"): true,
		util.SlugTR("birim degeri"): true, util.SlugTR("ust hakki odemesi"): true, util.SlugTR("ust hakki uzatimi odemesi"): true,
		util.SlugTR("satilik"): true, util.SlugTR("davasinda"): true, util.SlugTR("bulundurularak"): true,
		util.SlugTR("arsalar ve araziler"): true, util.SlugTR("arsa ve araziler"): true, util.SlugTR("arsasi"): true,
		util.SlugTR("tapu kaydi"): true, util.SlugTR("tapu kaydı"): true,
	}
	if generic[slug] {
		return true
	}
	genericContains := []string{
		"toplam gelir", "toplam gider", "toplam deger", "toplam portfoy", "net bugunku",
		"bugunku degeri", "yuvarlatilmis", "emsal karsilastirma", "gelir indirgeme",
		"satis degeri", "arsa degeri", "arsa alani", "arsa yuz olcumu", "proje degeri", "projenin tamamlanmasi",
		"nakit karsiligi", "benzer gayrimenkuller", "degerleme konusu", "ulasildi",
		"belirlenmistir", "varsayimina", "olmasi varsayimina",
		"dipnot", "not ", "aciklama", "kdv haric", "kdv dahil", "tl kdv",
		"lokasyon", "ekspertiz tarihi", "ada parsel", "alan m", "metrekare", "vasfi",
		"aittir", "kiralama islemi", "sirketimiz tarafindan", "topluluk",
		"yatirim yaparak", "arsa payi",
		"calismalara ivsc", "kdv dahil edilmemistir", "otel diger gelir",
		"yenileme gideri", "genel gider", "toplam maliyet", "yaklasik toplam",
		"yatak fiyati", "mal sahibi tarafindan", "kiraya verilmesi durumunda",
		"tamamlanmasi durumunda", "toplam bugunku deger", "giderler",
		"gider", "gelirler", "fiyat", "projenin tamamlanmis", "tamamlanmis",
		"toplami", "bulunan toplam", "mevkiinde bulunan", "alana sahip",
		"insaat alani", "kapali alandan", "kaydedilmistir", "gerceklesen kiralama",
		"uzerinde toplam", "toplamda", "yapilasmaya", "yapilasma sart",
		"gayrimenkuller gayrimenkul projeleri", "talebiniz dogrultusunda",
		"tasınmazlarin toplam", "tasinmazlarin toplam", "pazar degeri ve pazar",
		"yerinde yapilan incelemede", "projeksiyon ust hakki",
		"net defter degeri", "maddi duran varlik", "gelistirilmis arsa", "bugunku ortalama",
		"arsa birim degeri", "emlak vergisi", "rayic degeri", "degerlenen parsel",
		"degerlenen parseller", "mevcutta yapi bulunmamakta", "yapi bulunmamakta",
		"hukuki ve yasal prosedur", "prosedurlerini tamamladigi", "varsayilmistir",
		"varilmistir", "kamulastirma", "mahkemece", "mimari proje", "onayli mimari",
		"tapu mudurlugu", "ilce belediyesi", "incelenmistir", "proje gelistirme hesaplari",
		"proje riski", "finansman maliyeti", "konutun net alani", "ticari arsa degerleri",
		"belediyeden alinan", "satis bedeli istenilmektedir", "benzer ozelliklere sahip",
		"edilebilmesidir", "yakin konumda", "emsal teskil", "is bu rapora konu",
		"rapora konu olan tasinmaz", "degerlemesi yapilan tasinmaz", "nitelikli ana gayrimenkul",
		"no lu gayrimenkullerdir", "bulunup bulunmadigi", "hangi amacla kullanildigi",
		"butun parseller", "mevcut agac", "uygulama yapilacak", "arsa paylarinin",
		"villalarin arsa paylari", "kira satis bedellerine", "beyanlar temin",
		"arsaya", "dukkan icin", "telefon", "karara baglamak", "genel kurul", "vekaletname",
		"lejant", "imar plani", "imarli", "emsal", "parselin birim",
		"birim satis", "arsa satislari", "bolgede yapilan arastirmalara", "yorumlanarak", "ozel spor alani", "egitim tesis alani", "paftasindadir",
		"sıniri", "siniri", "irtifak",
		"kullanilan bu yontem", "kullanilan bu degerleme", "arsa birim satis degeri",
		"hasilat paylasimi yontemine gore", "yontemine gore", "rayic degerleri dikkate",
		"kira aylik", "ust hakki birim degeri", "gayrimenkulun acik adresi",
		"kap yilsonu", "degerleme raporu", "karsilastirilabilir gayrimenkuller",
		"diger varsayimlar", "varsayimlar", "musterinin talebinin kapsami",
		"musterinin talebinin", "portfoyumuzdeki", "portfoyumuzde",
		"kullanim alanli", "ofis katindan olusan", "dukkandan olusan",
		"ise sunlardir", "buyuklugundeki", "uzerine insaa", "uzerine insa",
		"civarinda", "satis tarihi itibari", "deger kaybi",
		"gayrimenkul projelerine", "gayrimenkuller uzerine ipotek",
		"sinirli ayni haklar", "rapora konu tasinmaz",
		"konu tasinmaz", "konu tasinmazlarin", "bulundugu bolgede",
		"kira bedelleri incelenmis", "yapilan oldugu belirtilen",
		"pazarlanan ile arsanin", "gorusmede", "coordinate table text",
		"alarko gayrimenkul yatirim ortakligi", "olagan genel kurul",
		"vekaletname", "beni temsile", "oy vermeye",
		"adet 3 katli", "adres", "akfen gyo", "alarko gyo",
		"nakit akisi", "tarafindan beyan edilen", "istirak tutari",
		"gercege uygun deger", "finansal varlik deger", "ozkaynak",
		"borsa istanbul", "hisse senedi", "tutari",
		"konu tasinmaz", "konu taşınmaz", "soz konusu gayrimenkul", "söz konusu gayrimenkul",
		"tapudaki vasiflari", "tapudaki vasıfları", "belediyesi imar mudurlugu",
		"belediyesi imar müdürlüğü", "cadde uzerinde yer alan", "cadde üzerinde yer alan",
		"bulundugu cadde", "bulunduğu cadde", "uygun arazi arastirmalarimiz",
		"uygun arazi araştırmalarımız", "fizibilite calismalarimiz", "fizibilite çalışmalarımız",
		"elde ettigimi", "elde ettiğimi", "gunumuz piyasa kosullarinda", "günümüz piyasa koşullarında",
		"karsisinda", "karşısında", "caddesi uzerinde", "caddesi üzerinde",
		"tomtom", "green beach resort", "bodrum kati", "bodrum katı",
		"buyukcekmece deki alkent", "büyükçekmece deki alkent", "en prestijli bolum",
		"en prestijli bölüm", "eyup te 13503", "eyüp te 13503", "istanbul eyup te 13503",
		"istanbul eyüp te 13503",
	}
	for _, item := range genericContains {
		if slugContains(slug, item) {
			return true
		}
	}
	genericPrefixes := []string{"bulunan", "mevkiinde", "mahallesi", "caddesi", "sokagi", "yaklasik", "projenin"}
	genericPrefixes = append(genericPrefixes, "otelin", "tesisin", "alanli", "depolu", "katli", "olarak", "oldugu", "onunde")
	genericPrefixes = append(genericPrefixes, "mahkemece", "belediyeden", "bu degerleme raporu", "is bu rapora", "sirketimiz")
	genericPrefixes = append(genericPrefixes, "parsel", "ada", "tasinmazin", "tasinmazlar", "degerlenen", "edilebilmesidir")
	genericPrefixes = append(genericPrefixes, "algy-", "algyo-", "agmyo", "agyo", "nedeniyle")
	genericPrefixes = append(genericPrefixes, "adres", "adet", "eski", "dmh", "tl hesaplanmasi")
	for _, prefix := range genericPrefixes {
		if strings.HasPrefix(slug, util.SlugTR(prefix)) {
			return true
		}
	}
	if strings.HasPrefix(strings.TrimSpace(value), "'") || strings.HasPrefix(strings.TrimSpace(value), "\"") {
		return true
	}
	if len([]rune(value)) > 90 && !assetNameHasPortfolioIdentity(value) {
		return true
	}
	return false
}

func assetNameHasPortfolioIdentity(value string) bool {
	trimmed := strings.TrimSpace(value)
	if assetUnitCodeRE.MatchString(trimmed) || assetValuationCodeRE.MatchString(trimmed) {
		return true
	}
	slug := util.SlugTR(trimmed)
	identityTerms := []string{
		"alkent", "hillside", "beach club", "alarko", "karakoy", "sishane", "cankaya",
		"buyukcekmece", "maslak", "bodrum", "fethiye", "etiler", "eyup", "topcular",
		"kalemya", "otel", "is merkezi", "dukkanlar", "magaza", "fabrika", "tesis",
		"arsa", "arazi", "parsel", "villa", "ojsc", "anonim", "limited",
	}
	for _, term := range identityTerms {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func assetLineHasMultipleKnownAssets(value string) bool {
	slug := util.SlugTR(value)
	knownAssets := []string{
		"etiler alkent",
		"buyukcekmece alkent",
		"eyup topcular",
		"ankara cankaya",
		"karakoy is merkezi",
		"sishane is merkezi",
		"fethiye hillside",
		"bodrum hillside",
		"bodrum otel",
		"maslak arsasi",
		"buyukcekmece eskice",
		"alarko dim is merkezi",
	}
	count := 0
	for _, item := range knownAssets {
		if slugContains(slug, item) {
			count++
		}
	}
	return count >= 2
}

func assetNameLooksLocationOnly(value string) bool {
	if len([]rune(value)) > 60 || cityInText(value) == "" {
		return false
	}
	if inferAssetType(value) != "other" || assetUnitCodeRE.MatchString(strings.TrimSpace(value)) {
		return false
	}
	slug := util.SlugTR(value)
	return slugContains(slug, "ilce") || slugContains(slug, "mahalle") || strings.Contains(value, "/") || strings.Contains(value, ",")
}

func assetEvidenceCount(assetName, assetType, location, city, district string, area *float64, parcel string, expertiseDate *string, excl, incl, book *float64, rental AssetRentalInfo) int {
	count := 0
	if strings.TrimSpace(assetName) != "" && !assetNameLooksGeneric(assetName) {
		count++
	}
	if location != "" || city != "" || district != "" {
		count++
	}
	if expertiseDate != nil {
		count++
	}
	if excl != nil || incl != nil || book != nil {
		count++
	}
	if area != nil || parcel != "" {
		count++
	}
	if assetType != "" && assetType != "other" {
		count++
	}
	if rental.IsRented != nil || rental.MonthlyRentTRY != nil || rental.AnnualRentTRY != nil || rental.AnnualMinRentUSD != nil {
		count++
	}
	return count
}

func assetLineEvidenceCount(line, assetName string) int {
	location, city, district := extractLocation(line)
	area := extractAreaM2(line)
	parcel := extractParcelInfo(line)
	expertiseDate := extractDateString(line)
	excl, incl, book, _ := extractAssetValues(line)
	rental := extractRentalInfo(line)
	assetType := inferAssetType(line)
	if strings.TrimSpace(assetName) == "" {
		assetName = extractAssetName(line, line)
	}
	return assetEvidenceCount(assetName, assetType, location, city, district, area, parcel, expertiseDate, excl, incl, book, rental)
}

func hasStrongLineAssetAnchor(line string) bool {
	if isAssetExplanationLine(line) || isNonAssetLine(line) {
		return false
	}
	if cityInText(line) != "" || extractParcelInfo(line) != "" {
		return true
	}
	if areaRE.MatchString(line) && inferAssetType(line) != "other" {
		return true
	}
	name := extractAssetName(line, line)
	return name != "" && inferAssetType(line) != "other" && !assetNameLooksGeneric(name)
}

func inferAssetType(text string) string {
	slug := util.SlugTR(text)
	switch {
	case slugContains(slug, "para ve sermaye piyasasi") || slugContains(slug, "menkul kiymet") || slugContains(slug, "tahvil") || slugContains(slug, "fon"):
		return "financial_asset"
	case slugContains(slug, "istirak") || slugContains(slug, "bagli ortaklik"):
		return "subsidiary"
	case slugContains(slug, "ust hakki") || slugContains(slug, "kullanim hakki") || slugContains(slug, "intifa"):
		return "usage_right"
	case slugContains(slug, "otel"):
		return "hotel"
	case slugContains(slug, "is merkezi") || slugContains(slug, "ofis") || slugContains(slug, "buro"):
		return "office"
	case slugContains(slug, "dukkan") || slugContains(slug, "magaza"):
		return "shop"
	case slugContains(slug, "fabrika") || slugContains(slug, "tesis"):
		return "factory"
	case slugContains(slug, "proje") || slugContains(slug, "insaat") || slugContains(slug, "devam eden"):
		return "project"
	case slugContains(slug, "arsa") || slugContains(slug, "arazi") || slugContains(slug, "tarla"):
		return "land"
	default:
		return "other"
	}
}

func inferOwnershipType(text string) string {
	slug := util.SlugTR(text)
	switch {
	case slugContains(slug, "ust hakki") || slugContains(slug, "kullanim hakki") || slugContains(slug, "intifa"):
		return "usage_right"
	case slugContains(slug, "kiralandi") || slugContains(slug, "kiralanan") || slugContains(slug, "kiraya verilen"):
		return "leased"
	case slugContains(slug, "istirak") || slugContains(slug, "bagli ortaklik"):
		return "subsidiary"
	case slugContains(slug, "mulkiyet") || slugContains(slug, "tapu") || slugContains(slug, "sahip"):
		return "owned"
	default:
		return "unknown"
	}
}

func inferAssetStatus(text string) string {
	slug := util.SlugTR(text)
	switch {
	case slugContains(slug, "devam eden") || slugContains(slug, "insaat") || slugContains(slug, "yapimi devam"):
		return "under_construction"
	case slugContains(slug, "renovasyon") || slugContains(slug, "tadilat"):
		return "renovation"
	case slugContains(slug, "kiraya verilen") || slugContains(slug, "kiraci") || slugContains(slug, "kira"):
		return "rented"
	case slugContains(slug, "satisa konu") || slugContains(slug, "satis amacli"):
		return "for_sale"
	case slugContains(slug, "bos") || slugContains(slug, "vacant"):
		return "vacant"
	case slugContains(slug, "faal") || slugContains(slug, "isletme") || slugContains(slug, "operasyon"):
		return "operating"
	default:
		return "unknown"
	}
}

func extractLocation(text string) (string, string, string) {
	city, district := cityInText(text), ""
	if city != "" {
		district = districtNearCity(text, city)
	}
	location := ""
	for _, cell := range splitAssetCells(text) {
		slug := util.SlugTR(cell)
		if city != "" && slugContains(slug, city) {
			location = normalizeAssetLine(cell)
			break
		}
		if slugContains(slug, "mah") || slugContains(slug, "cad") || slugContains(slug, "sok") || slugContains(slug, "ilce") || slugContains(slug, "mevkii") {
			location = normalizeAssetLine(cell)
			break
		}
	}
	return location, city, district
}

func sanitizeAssetLocationFields(assetName, location, city, district string) (string, string, string) {
	location = cleanAssetLocationText(location)
	district = cleanAssetDistrictText(district)
	city = strings.TrimSpace(city)
	expectedCity := expectedCityForAssetText(assetName + " " + location)
	if expectedCity != "" {
		if city == "" || !strings.EqualFold(city, expectedCity) {
			city = expectedCity
		}
		if locationContradictsExpectedCity(location, expectedCity) {
			location = ""
		}
	}
	if city != "" && district != "" && strings.EqualFold(city, district) {
		district = ""
	}
	if district != "" && assetLocationTokenLooksNoisy(district) {
		district = ""
	}
	if location != "" && assetLocationTokenLooksNoisy(location) {
		location = ""
	}
	return location, city, district
}

func cleanAssetLocationText(value string) string {
	value = normalizeAssetLine(value)
	if value == "" {
		return ""
	}
	value = stripAssetTableHeaderPrefix(value)
	if assetLocationTokenLooksNoisy(value) {
		return ""
	}
	if len([]rune(value)) > 180 {
		return truncateAssetSnippet(value, 180)
	}
	return value
}

func stripAssetTableHeaderPrefix(value string) string {
	slug := util.SlugTR(value)
	if !(slugContains(slug, "kdv haric") || slugContains(slug, "kdv dahil") || slugContains(slug, "ekspertiz")) {
		return value
	}
	for _, marker := range []string{
		"İstanbul İli", "Istanbul Ili", "Eyüp / İstanbul", "Eyup / Istanbul",
		"Eyüpsultan", "Eyupsultan", "Topçular Mahallesi", "Topcular Mahallesi",
		"Bodrum / Muğla", "Bodrum / Mugla", "Fethiye", "Beşiktaş / İstanbul", "Besiktas / Istanbul",
		"Ankara Çankaya", "Ankara Cankaya", "Sarıyer / İstanbul", "Sariyer / Istanbul",
	} {
		if idx := strings.Index(value, marker); idx > 0 {
			return strings.TrimSpace(value[idx:])
		}
	}
	return value
}

func cleanAssetDistrictText(value string) string {
	value = cleanLocationToken(value)
	if assetLocationTokenLooksNoisy(value) {
		return ""
	}
	return value
}

func assetLocationTokenLooksNoisy(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	trimmed := strings.TrimLeft(value, "- ")
	if trimmed != "" {
		first := []rune(trimmed)[0]
		if first >= '0' && first <= '9' {
			return true
		}
	}
	slug := util.SlugTR(value)
	blocked := []string{
		"kdv haric", "kdv dahil", "alis tarihi", "alis maliyeti", "ekspertiz degeri",
		"portfoy degeri", "sigorta degeri", "kira bedeli", "kiraci", "baslangic",
		"ortalaması", "ortalamasi", "emsal karsilastirma",
		"asansorlu", "havalandirma", "dogalgaz", "isitmali", "isıtmalı", "chiller",
		"klima", "brut", "net", "tek blok", "kat halinde", "kapasiteli", "parsel",
		"no lu", "nolu", "m2", "metrekare",
	}
	for _, item := range blocked {
		if slugContains(slug, item) {
			return true
		}
	}
	if areaRE.MatchString(value) && len(strings.Fields(value)) <= 4 {
		return true
	}
	return false
}

func locationContradictsExpectedCity(location, expectedCity string) bool {
	if strings.TrimSpace(location) == "" || strings.TrimSpace(expectedCity) == "" {
		return false
	}
	found := cityInText(location)
	return found != "" && !strings.EqualFold(found, expectedCity)
}

func expectedCityForAssetText(text string) string {
	slug := util.SlugTR(text)
	switch {
	case slugContains(slug, "eyup") || slugContains(slug, "eyupsultan") || slugContains(slug, "topcular") ||
		slugContains(slug, "etiler") || slugContains(slug, "besiktas") || slugContains(slug, "karakoy") ||
		slugContains(slug, "sishane") || slugContains(slug, "maslak") || slugContains(slug, "sariyer") ||
		slugContains(slug, "buyukcekmece"):
		return "Istanbul"
	case slugContains(slug, "bodrum") || slugContains(slug, "mugla") || slugContains(slug, "fethiye"):
		return "Mugla"
	case slugContains(slug, "ankara") || slugContains(slug, "cankaya"):
		return "Ankara"
	default:
		return ""
	}
}

func cityInText(text string) string {
	words := slugWordSet(text)
	for _, city := range cityNamesTR {
		if words[util.SlugTR(city)] {
			return cityDisplay(city)
		}
	}
	return ""
}

func districtNearCity(text, city string) string {
	parts := regexp.MustCompile(`\s*/\s*|\s+-\s+|,\s*`).Split(text, -1)
	citySlug := util.SlugTR(city)
	for i, part := range parts {
		if slugContains(util.SlugTR(part), citySlug) {
			if i+1 < len(parts) {
				next := cleanLocationToken(strings.TrimSpace(parts[i+1]))
				if hasLetters(next) && len([]rune(next)) <= 40 && !assetLocationTokenLooksNoisy(next) {
					return next
				}
			}
			if i > 0 {
				prev := cleanLocationToken(strings.TrimSpace(parts[i-1]))
				if hasLetters(prev) && len([]rune(prev)) <= 40 && !assetLocationTokenLooksNoisy(prev) {
					return prev
				}
			}
		}
	}
	return ""
}

func cleanLocationToken(value string) string {
	value = normalizeAssetLine(value)
	value = regexp.MustCompile(`(?i)\b(il|ili|ilce|ilcesi|mahallesi|mah\.?)\b`).ReplaceAllString(value, "")
	return strings.Trim(value, " :-")
}

func cityDisplay(slug string) string {
	for _, city := range []string{"istanbul", "izmir", "ankara"} {
		if slug == city {
			return strings.Title(city)
		}
	}
	return strings.Title(slug)
}

func extractAreaM2(text string) *float64 {
	matches := areaRE.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value, ok := parseTurkishNumber(match[1])
		if ok && value > 0 && value < 100000000 {
			return floatPtr(value)
		}
	}
	return nil
}

func extractParcelInfo(text string) string {
	for _, re := range parcelREs {
		if match := re.FindString(text); match != "" {
			return normalizeParcelInfo(match)
		}
	}
	return ""
}

func normalizeParcelInfo(value string) string {
	value = normalizeAssetLine(value)
	value = strings.Trim(value, " .,:;-")
	value = regexp.MustCompile(`(?i)\s+ile\s+kay[ıi]tl[ıi].*$`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`(?i)\s+konumlu.*$`).ReplaceAllString(value, "")
	replacer := strings.NewReplacer(
		"parsellerde", "parseller",
		"Parsellerde", "Parseller",
		"parselleri", "parseller",
		"Parselleri", "Parseller",
		"parseldeki", "parsel",
		"Parseldeki", "Parsel",
		"parselde", "parsel",
		"Parselde", "Parsel",
		"parseli", "parsel",
		"Parseli", "Parsel",
		"parseldedir", "parsel",
		"Parseldedir", "Parsel",
	)
	value = replacer.Replace(value)
	value = regexp.MustCompile(`(?i)\bno\s*\.?\s*lu\b`).ReplaceAllString(value, "nolu")
	value = strings.Join(strings.Fields(value), " ")
	return strings.Trim(value, " .,:;-")
}

func extractRentalInfo(text string) AssetRentalInfo {
	slug := util.SlugTR(text)
	out := AssetRentalInfo{}
	if slugContains(slug, "kira") || slugContains(slug, "kiraci") || slugContains(slug, "kiraya") {
		out.IsRented = boolPtr(true)
	}
	if value := tableAmountByHeader(text, []string{"aylik kira", "aylık kira", "aylik kira bedeli", "aylık kira bedeli"}, "TRY"); value != nil {
		out.MonthlyRentTRY = value
	} else if value := amountNearKeywords(text, []string{"aylik kira", "aylık kira", "aylik kira bedeli", "aylık kira bedeli"}, "TRY"); value != nil {
		out.MonthlyRentTRY = value
	}
	if value := tableAmountByHeader(text, []string{"yillik kira", "yıllık kira"}, "TRY"); value != nil {
		out.AnnualRentTRY = value
	} else if value := amountNearKeywords(text, []string{"yillik kira", "yıllık kira"}, "TRY"); value != nil {
		out.AnnualRentTRY = value
	}
	if value := tableAmountByHeader(text, []string{"asgari kira", "minimum kira", "yillik asgari kira", "yıllık asgari kira"}, "USD"); value != nil {
		out.AnnualMinRentUSD = value
	} else if value := amountNearKeywords(text, []string{"asgari kira", "minimum kira", "yillik kira", "yıllık kira"}, "USD"); value != nil {
		out.AnnualMinRentUSD = value
	}
	if slugContains(slug, "ciro") || strings.Contains(text, "%") {
		if slugContains(slug, "kira") {
			out.VariableRentTerms = truncateAssetSnippet(text, 220)
		}
	}
	out.Tenant = extractTenant(text)
	return out
}

func extractTenant(text string) string {
	re := regexp.MustCompile(`(?i)kirac[ıi]\s*[:=-]?\s*([A-ZÇĞİÖŞÜa-zçğıöşü0-9 .&/-]{3,80})`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return strings.Trim(match[1], " .;-")
	}
	return ""
}

func extractAssetValues(text string) (*float64, *float64, *float64, []string) {
	flags := []string{}
	excl := tableAmountByHeader(text, []string{"kdv haric", "kdv hariç"}, "TRY")
	if excl == nil {
		if sameLineExcl, sameLineIncl := sameLineVATAmounts(text); sameLineExcl != nil || sameLineIncl != nil {
			return sameLineExcl, sameLineIncl, nil, flags
		}
	}
	if excl == nil {
		excl = amountNearKeywords(text, []string{"kdv haric", "kdv hariç"}, "TRY")
	}
	incl := tableAmountByHeader(text, []string{"kdv dahil"}, "TRY")
	if incl == nil {
		incl = amountNearKeywords(text, []string{"kdv dahil"}, "TRY")
	}
	book := tableAmountByHeader(text, []string{"defter degeri", "defter değeri", "kayitli deger", "kayıtlı değer", "rayic deger", "rayiç değer", "rayic degerleri", "rayiç değerleri", "gercege uygun degeri", "gerçeğe uygun değeri", "ekspertiz degeri", "ekspertiz değeri", "sigorta degeri", "sigorta değeri"}, "TRY")
	if book == nil {
		book = amountNearKeywords(text, []string{"defter degeri", "defter değeri", "kayitli deger", "kayıtlı değer", "rayic deger", "rayiç değer", "rayic degerleri", "rayiç değerleri", "gercege uygun degeri", "gerçeğe uygun değeri", "ekspertiz degeri", "ekspertiz değeri", "sigorta degeri", "sigorta değeri"}, "TRY")
	}
	if book != nil && excl == nil && incl == nil && containsExpertiseCue(text) {
		flags = append(flags, "vat_status_unknown_for_expertise_value")
	}
	if excl != nil && incl != nil && *excl > *incl {
		flags = append(flags, "vat_values_need_review")
	}
	return excl, incl, book, flags
}

func sameLineVATAmounts(text string) (*float64, *float64) {
	slug := util.SlugTR(text)
	if !(slugContains(slug, "kdv haric") && slugContains(slug, "kdv dahil")) {
		return nil, nil
	}
	if strings.Contains(text, "\n") && assetLineHasMultipleKnownAssets(text) {
		return nil, nil
	}
	amountText := text
	if dateMatches := dateRE.FindAllStringIndex(text, -1); len(dateMatches) > 0 {
		amountText = text[dateMatches[len(dateMatches)-1][1]:]
	}
	values := extractTableAmountValues(amountText)
	if len(values) < 2 {
		return nil, nil
	}
	excl, incl := values[0], values[1]
	if excl > incl {
		return nil, nil
	}
	return floatPtr(excl), floatPtr(incl)
}

func extractTableAmountValues(text string) []float64 {
	clean := strings.NewReplacer("(", " ", ")", " ", "[", " ", "]", " ", "|", " ", ";", " ").Replace(text)
	values := []float64{}
	for _, field := range strings.Fields(clean) {
		token := strings.Trim(field, ".,:;")
		if !tableAmountTokenRE.MatchString(token) {
			continue
		}
		value, ok := parseTurkishNumber(token)
		if !ok || value < 100000 {
			continue
		}
		values = append(values, value)
	}
	return values
}

func amountNearKeywords(text string, keywords []string, currency string) *float64 {
	slug, indexMap := slugWithIndex(text)
	if len(indexMap) == 0 {
		return nil
	}
	for _, keyword := range keywords {
		k := util.SlugTR(keyword)
		idx := strings.Index(slug, k)
		if idx < 0 {
			continue
		}
		startSlug := idx - 30
		if startSlug < 0 {
			startSlug = 0
		}
		endSlug := idx + len(k) + 80
		if endSlug >= len(indexMap) {
			endSlug = len(indexMap) - 1
		}
		runes := []rune(text)
		start := indexMap[startSlug]
		end := indexMap[endSlug] + 1
		if start < 0 || start > len(runes) {
			start = 0
		}
		if end < start || end > len(runes) {
			end = len(runes)
		}
		if value := largestMoneyAmount(string(runes[start:end]), currency); value != nil {
			return value
		}
	}
	return nil
}

func slugWithIndex(text string) (string, []int) {
	replacer := strings.NewReplacer(
		"İ", "I", "I", "i", "ı", "i", "Ö", "O", "ö", "o", "Ü", "U", "ü", "u",
		"Ğ", "G", "ğ", "g", "Ş", "S", "ş", "s", "Ç", "C", "ç", "c",
	)
	runes := []rune(strings.ToLower(replacer.Replace(text)))
	var b strings.Builder
	indexMap := []int{}
	for idx, r := range runes {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			indexMap = append(indexMap, idx)
		}
	}
	return b.String(), indexMap
}

func tableAmountByHeader(text string, keywords []string, currency string) *float64 {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !lineHasAnyKeyword(line, keywords) {
			continue
		}
		headers := splitAssetCells(line)
		if len(headers) < 2 {
			continue
		}
		headerIndex := -1
		for idx, header := range headers {
			if lineHasAnyKeyword(header, keywords) {
				headerIndex = idx
				break
			}
		}
		if headerIndex < 0 {
			continue
		}
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			row := splitAssetCells(lines[j])
			if len(row) <= headerIndex || isGenericAssetHeader(lines[j]) {
				continue
			}
			if value := largestMoneyAmount(row[headerIndex], currency); value != nil {
				return value
			}
		}
	}
	return nil
}

func lineHasAnyKeyword(line string, keywords []string) bool {
	slug := util.SlugTR(line)
	for _, keyword := range keywords {
		if slugContains(slug, keyword) {
			return true
		}
	}
	return false
}

func largestMoneyAmount(text, currency string) *float64 {
	amounts := extractMoneyAmounts(text)
	best := 0.0
	found := false
	for _, amount := range amounts {
		if currency != "" && amount.Currency != "" && amount.Currency != currency {
			continue
		}
		if currency != "" && amount.Currency == "" && !contextImpliesCurrency(text, currency) {
			continue
		}
		if amount.Value > best {
			best = amount.Value
			found = true
		}
	}
	if !found {
		return nil
	}
	return floatPtr(best)
}

type parsedAmount struct {
	Value    float64
	Currency string
}

func extractMoneyAmounts(text string) []parsedAmount {
	matches := amountRE.FindAllStringIndex(text, -1)
	out := []parsedAmount{}
	for _, indexes := range matches {
		if len(indexes) != 2 {
			continue
		}
		match := text[indexes[0]:indexes[1]]
		if indexes[1] < len(text) && text[indexes[1]] == '%' {
			match = trailingPercentValueRE.ReplaceAllString(match, "")
		}
		if strings.TrimSpace(match) == "" || strings.Contains(match, "%") {
			continue
		}
		currency := detectCurrency(match)
		valueText := moneyCurrencyRE.ReplaceAllString(match, "")
		value, ok := parseTurkishNumber(valueText)
		if !ok || value <= 0 {
			continue
		}
		out = append(out, parsedAmount{Value: value, Currency: currency})
	}
	return out
}

func detectCurrency(text string) string {
	upper := strings.ToUpper(text)
	switch {
	case strings.Contains(upper, "USD") || strings.Contains(upper, "$"):
		return "USD"
	case strings.Contains(upper, "EUR") || strings.Contains(upper, "€"):
		return "EUR"
	case strings.Contains(upper, "TRY") || strings.Contains(upper, "TL") || strings.Contains(upper, "₺"):
		return "TRY"
	default:
		return ""
	}
}

func contextImpliesCurrency(text, currency string) bool {
	slug := util.SlugTR(text)
	switch currency {
	case "TRY":
		return strings.Contains(slug, "tl") || strings.Contains(slug, "try") || strings.Contains(slug, "turk lirasi") ||
			slugContains(slug, "kdv haric") || slugContains(slug, "kdv dahil") ||
			slugContains(slug, "gercege uygun degeri") || slugContains(slug, "rayic deger") ||
			slugContains(slug, "ekspertiz degeri") || slugContains(slug, "portfoy degeri")
	case "USD":
		return strings.Contains(slug, "usd") || strings.Contains(slug, "abd dolari") || strings.Contains(text, "$")
	default:
		return false
	}
}

func parseTurkishNumber(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	value = moneyCurrencyRE.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, "\u00a0", "")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.Trim(value, ".,;:()")
	if value == "" {
		return 0, false
	}
	lastComma := strings.LastIndex(value, ",")
	lastDot := strings.LastIndex(value, ".")
	switch {
	case lastComma >= 0 && lastDot >= 0 && lastDot > lastComma:
		value = strings.ReplaceAll(value, ",", "")
	case lastComma >= 0 && lastDot >= 0 && lastComma > lastDot:
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	case lastComma >= 0:
		value = normalizeKAPSingleSeparatorNumber(value, ',')
	case lastDot >= 0:
		value = normalizeKAPSingleSeparatorNumber(value, '.')
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func normalizeKAPSingleSeparatorNumber(value string, separator rune) string {
	separatorText := string(separator)
	parts := strings.Split(value, separatorText)
	if len(parts) == 2 {
		left, right := parts[0], parts[1]
		if len(right) == 3 && len(left) >= 1 && len(left) <= 3 {
			return left + right
		}
		if separator == ',' {
			return left + "." + right
		}
		return value
	}
	thousands := len(parts) > 2
	for _, part := range parts[1:] {
		if len(part) != 3 {
			thousands = false
			break
		}
	}
	if thousands {
		return strings.Join(parts, "")
	}
	decimal := parts[len(parts)-1]
	integer := strings.Join(parts[:len(parts)-1], "")
	return integer + "." + decimal
}

func extractDateString(text string) *string {
	match := dateRE.FindStringSubmatch(text)
	if len(match) != 4 {
		return nil
	}
	day, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	year, _ := strconv.Atoi(match[3])
	if year < 1990 || month < 1 || month > 12 || day < 1 || day > 31 {
		return nil
	}
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	return &value
}

func extractPeriodString(text string) *string {
	match := periodRE.FindStringSubmatch(text)
	if len(match) < 3 {
		return extractDateString(text)
	}
	period := strings.ToUpper(strings.ReplaceAll(match[2], ".", ""))
	switch period {
	case "03":
		period = "Q1"
	case "06":
		period = "Q2"
	case "09":
		period = "Q3"
	case "12":
		period = "Q4"
	default:
		if strings.Contains(util.SlugTR(period), "1") {
			period = "Q1"
		} else if strings.Contains(util.SlugTR(period), "2") {
			period = "Q2"
		} else if strings.Contains(util.SlugTR(period), "3") {
			period = "Q3"
		} else if strings.Contains(util.SlugTR(period), "4") {
			period = "Q4"
		}
	}
	value := match[1] + "-" + period
	return &value
}

func extractCompanyName(text string) string {
	lines := strings.Split(firstRunes(text, 6000), "\n")
	for _, line := range lines {
		clean := normalizeAssetLine(line)
		slug := util.SlugTR(clean)
		if slugContains(slug, "anonim sirket") || slugContains(slug, "yatirim ortakligi") {
			return truncateAssetSnippet(clean, 180)
		}
	}
	return ""
}

func assetConfidence(doc RawDocument, assetName, assetType, location string, area *float64, parcel string, excl, incl, book *float64, rental AssetRentalInfo) float64 {
	score := 0.20
	if doc.DocumentTypeGuess == DocumentValuationReport || doc.DocumentTypeGuess == DocumentAnnualReport || doc.DocumentTypeGuess == DocumentInterimActivityReport {
		score += 0.10
	}
	if assetName != "" {
		score += 0.18
	}
	if assetType != "other" {
		score += 0.12
	}
	if location != "" {
		score += 0.10
	}
	if area != nil {
		score += 0.08
	}
	if parcel != "" {
		score += 0.10
	}
	if excl != nil || incl != nil || book != nil {
		score += 0.14
	}
	if rental.IsRented != nil || rental.MonthlyRentTRY != nil || rental.AnnualMinRentUSD != nil {
		score += 0.08
	}
	score += doc.QualityScore * 0.10
	return clampAsset(score, 0, 1)
}

func incomeRelevance(rental AssetRentalInfo, assetType string, book, excl, incl *float64) string {
	if rental.IsRented != nil && *rental.IsRented {
		return "high"
	}
	if rental.MonthlyRentTRY != nil || rental.AnnualRentTRY != nil || rental.AnnualMinRentUSD != nil {
		return "high"
	}
	if assetType == "financial_asset" || book != nil || excl != nil || incl != nil {
		return "medium"
	}
	return "unknown"
}

func opportunityFlags(assetType string, rental AssetRentalInfo, status string) []string {
	flags := []string{}
	if rental.AnnualMinRentUSD != nil {
		flags = append(flags, "fx_linked_rental_income")
	}
	if rental.IsRented != nil && *rental.IsRented {
		flags = append(flags, "rental_income_asset")
	}
	if status == "under_construction" || assetType == "project" {
		flags = append(flags, "development_project")
	}
	return flags
}

func isAssetHeading(line string) bool {
	slug := util.SlugTR(line)
	if len([]rune(line)) > 160 {
		return false
	}
	if areaRE.MatchString(line) || len(extractMoneyAmounts(line)) > 0 || extractParcelInfo(line) != "" {
		return false
	}
	for _, keyword := range assetHeadingKeywords {
		if strings.Contains(slug, util.SlugTR(keyword)) {
			return true
		}
	}
	return false
}

func containsAssetCue(text string) bool {
	slug := util.SlugTR(firstRunes(text, 40000))
	for _, keyword := range assetCueKeywords {
		if strings.Contains(slug, util.SlugTR(keyword)) {
			return true
		}
	}
	return false
}

func normalizeAssetLine(line string) string {
	line = strings.ReplaceAll(line, "\u00a0", " ")
	line = strings.TrimSpace(line)
	return strings.Join(strings.Fields(line), " ")
}

func isMostlyNumeric(value string) bool {
	digits := 0
	letters := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits++
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || strings.ContainsRune("ÇĞİÖŞÜçğıöşü", r) {
			letters++
		}
	}
	return digits > 0 && letters == 0
}

func hasLetters(value string) bool {
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || strings.ContainsRune("ÇĞİÖŞÜçğıöşü", r) {
			return true
		}
	}
	return false
}

func isNonAssetLine(line string) bool {
	clean := normalizeAssetLine(line)
	if clean == "" {
		return true
	}
	slug := util.SlugTR(clean)
	if isTotalLikeLine(clean) || isValuationMethodLine(clean) {
		return true
	}
	if isGenericAssetHeader(clean) {
		return true
	}
	if slugContains(slug, "dipnot") || footnoteLineRE.MatchString(clean) {
		return true
	}
	if (slugContains(slug, "kdv haric") || slugContains(slug, "kdv dahil")) && len(extractMoneyAmounts(clean)) == 0 {
		return true
	}
	if len([]rune(clean)) > 220 && !areaRE.MatchString(clean) && extractParcelInfo(clean) == "" && cityInText(clean) == "" {
		return true
	}
	return false
}

func isAssetExplanationLine(line string) bool {
	slug := util.SlugTR(line)
	if slug == "" {
		return true
	}
	explanationPhrases := []string{
		"degerleme yontemi",
		"degerleme calismasi",
		"degerleme konusu",
		"deger takdirinde",
		"emsal karsilastirma",
		"gelir indirgeme",
		"indirgenmis nakit",
		"maliyet yaklasimi",
		"net bugunku deger",
		"piyasa degerinin",
		"kdv dahil edilmemistir",
		"calismalara ivsc",
		"yaklasik toplam bugunku deger",
		"projenin tamamlanmasi durumundaki",
		"projenin tamamlanmasi durumunda",
		"mal sahibi tarafindan isletmesi",
		"kiraya verilmesi durumunda",
		"ort yatak fiyati",
		"otel toplam gider",
		"otel giderler",
		"otel gideri",
		"otel gelirler",
		"diger gelirler",
		"toplam giderler",
		"genel giderler",
		"yenileme gideri",
		"proje toplam maliyeti",
		"bina toplam insaat alani",
		"talebiniz dogrultusunda",
		"pazar degeri ve pazar",
		"yerinde yapilan incelemede",
		"projeksiyon ust hakki",
		"tasinmazlarin toplam",
	}
	for _, phrase := range explanationPhrases {
		if slugContains(slug, phrase) {
			return true
		}
	}
	lineSlug := util.SlugTR(line)
	if (slugContains(lineSlug, "kdv haric") || slugContains(lineSlug, "kdv dahil")) && !slugContains(lineSlug, "arsa") && !slugContains(lineSlug, "otel") && !slugContains(lineSlug, "dukkan") && !slugContains(lineSlug, "is merkezi") {
		return true
	}
	if slugContains(lineSlug, "toplam") && (slugContains(lineSlug, "gider") || slugContains(lineSlug, "gelir") || slugContains(lineSlug, "maliyet")) {
		return true
	}
	return false
}

func isTotalLikeLine(line string) bool {
	slug := util.SlugTR(line)
	if slug == "" {
		return false
	}
	if slugContains(slug, "ara toplam") || slugContains(slug, "genel toplam") || strings.HasPrefix(slug, util.SlugTR("toplam")) {
		return true
	}
	return false
}

func isPortfolioSummaryLine(line, context string) bool {
	if !isTotalLikeLine(line) {
		return false
	}
	if isFinancialStatementTotalLine(line) {
		return false
	}
	lineSlug := util.SlugTR(line)
	slug := util.SlugTR(line + " " + context)
	totalPhrases := []string{
		"toplam portfoy", "toplam portföy", "portfoy toplam", "portföy toplam",
		"gayrimenkuller toplam", "gayrimenkul toplam", "toplam gayrimenkul",
	}
	for _, phrase := range totalPhrases {
		if slugContains(slug, phrase) {
			return true
		}
	}
	if strings.HasPrefix(lineSlug, util.SlugTR("toplam")) &&
		(slugContains(lineSlug, "portfoy") || slugContains(lineSlug, "gayrimenkuller") || slugContains(lineSlug, "gayrimenkul projeleri")) &&
		(slugContains(slug, "kdv haric") || slugContains(slug, "kdv dahil") || slugContains(slug, "ekspertiz")) {
		return true
	}
	if strings.HasPrefix(lineSlug, util.SlugTR("toplam")) &&
		slugContains(slug, "portfoyde yer alan varliklar") &&
		(slugContains(slug, "kdv haric") || slugContains(slug, "kdv dahil") || slugContains(slug, "ekspertiz degeri")) {
		return true
	}
	if strings.HasPrefix(lineSlug, util.SlugTR("toplam")) &&
		isGenericAssetHeader(context) &&
		(slugContains(slug, "kdv haric") || slugContains(slug, "kdv dahil") || slugContains(slug, "ekspertiz degeri")) {
		return true
	}
	return false
}

func isFinancialStatementTotalLine(line string) bool {
	slug := util.SlugTR(line)
	financialTotals := []string{
		"toplam varliklar", "toplam kaynaklar", "toplam yukumluluk", "toplam kapsamli",
		"toplam gelir", "toplam gider", "toplam vergi", "toplam borc", "toplam alacak",
		"toplam ozkaynak", "toplam satis", "toplam nakit", "toplam kredi",
	}
	for _, item := range financialTotals {
		if slugContains(slug, item) {
			return true
		}
	}
	return false
}

func isValuationMethodLine(line string) bool {
	slug := util.SlugTR(line)
	methods := []string{
		"degerleme yontemi", "emsal karsilastirma", "gelir indirgeme", "maliyet yaklasimi",
		"net bugunku deger", "indirgenmis nakit akimi", "piyasa yaklasimi",
		"degerleme calismasinda", "degerleme konusu", "deger takdirinde",
		"nakit karsiligi pesin satis degeri belirlenmistir",
	}
	for _, method := range methods {
		if slugContains(slug, method) {
			return true
		}
	}
	return false
}

func isGenericAssetHeader(line string) bool {
	slug := util.SlugTR(line)
	headers := []string{"varlik adi", "lokasyon", "il ilce", "ekspertiz tarihi", "kdv haric", "kdv dahil", "metrekare", "kira bedeli"}
	matches := 0
	for _, header := range headers {
		if slugContains(slug, header) {
			matches++
		}
	}
	return matches >= 3
}

func containsExpertiseCue(text string) bool {
	slug := util.SlugTR(text)
	return slugContains(slug, "ekspertiz") || slugContains(slug, "degerleme")
}

func containsRawWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if warning == want {
			return true
		}
	}
	return false
}

func assetEventKey(event AssetEvent) string {
	parts := []string{
		event.SHA256,
		util.SlugTR(event.AssetName),
		util.SlugTR(event.Location),
		util.SlugTR(event.ParcelInfo),
		event.AssetType,
		filepath.Base(event.SourceFile),
	}
	return strings.Join(parts, "|")
}

func firstNonEmptyAsset(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func slugContains(slug string, phrase string) bool {
	if slug == "" || phrase == "" {
		return false
	}
	return strings.Contains(slug, util.SlugTR(phrase))
}

func slugWordSet(text string) map[string]bool {
	normalized := turkishAssetASCII(strings.ToLower(text))
	var b strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	words := map[string]bool{}
	for _, word := range strings.Fields(b.String()) {
		if word != "" {
			words[word] = true
		}
	}
	return words
}

func turkishAssetASCII(value string) string {
	replacer := strings.NewReplacer(
		"İ", "I", "I", "i", "ı", "i", "Ö", "O", "ö", "o", "Ü", "U", "ü", "u",
		"Ğ", "G", "ğ", "g", "Ş", "S", "ş", "s", "Ç", "C", "ç", "c",
	)
	return replacer.Replace(value)
}

func boolPtr(value bool) *bool        { return &value }
func floatPtr(value float64) *float64 { return &value }
func intPtr(value int) *int           { return &value }

func clampAsset(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func truncateAssetSnippet(value string, limit int) string {
	value = normalizeAssetLine(value)
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "..."
}
