package kapingest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"hissebot/internal/util"
)

type AssetExtractionOptions struct {
	RawDocumentsPath string
	OutputDir        string
	Now              func() time.Time
}

type AssetExtractionSummary struct {
	Status                  string   `json:"status"`
	RawDocumentsPath        string   `json:"raw_documents_path"`
	OutputDir               string   `json:"output_dir"`
	RawDocuments            int      `json:"raw_documents"`
	AnalysisUsableDocuments int      `json:"analysis_usable_documents"`
	ReviewRequiredDocuments int      `json:"review_required_documents"`
	RejectedDocuments       int      `json:"rejected_documents"`
	AssetEvents             int      `json:"asset_events"`
	Tickers                 int      `json:"tickers"`
	AssetInventories        int      `json:"asset_inventories"`
	OutputFiles             []string `json:"output_files,omitempty"`
	Warnings                []string `json:"warnings,omitempty"`
}

func ExtractAssetsFromRawDocuments(ctx context.Context, opts AssetExtractionOptions) (AssetExtractionSummary, error) {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return AssetExtractionSummary{}, errors.New("output dir is required")
	}
	rawPath := strings.TrimSpace(opts.RawDocumentsPath)
	if rawPath == "" {
		rawPath = filepath.Join(opts.OutputDir, RawDocumentsFile)
	}
	summary := AssetExtractionSummary{
		Status:           "ok",
		RawDocumentsPath: filepath.Clean(rawPath),
		OutputDir:        filepath.Clean(opts.OutputDir),
	}
	if _, err := os.Stat(rawPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			summary.Status = "missing_raw_documents"
			summary.Warnings = append(summary.Warnings, "raw_documents_jsonl_missing")
			return summary, nil
		}
		return summary, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return summary, err
	}
	globalEventsPath := filepath.Join(opts.OutputDir, AssetEventsFile)
	byTickerDir := filepath.Join(opts.OutputDir, "by_ticker")
	_ = os.Remove(globalEventsPath)
	_ = os.RemoveAll(byTickerDir)
	if err := os.MkdirAll(byTickerDir, 0o755); err != nil {
		return summary, err
	}

	globalFile, err := os.OpenFile(globalEventsPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return summary, err
	}
	defer globalFile.Close()
	globalEncoder := json.NewEncoder(globalFile)

	eventsByTicker := map[string][]AssetEvent{}
	summariesByTicker := map[string][]PortfolioSummarySnapshot{}
	notesByTicker := map[string][]ValuationNote{}
	rawFile, err := os.Open(rawPath)
	if err != nil {
		return summary, err
	}
	defer rawFile.Close()
	scanner := bufio.NewScanner(rawFile)
	scanner.Buffer(make([]byte, 256*1024), 128*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			summary.Status = "cancelled"
			return summary, err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var doc RawDocument
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			summary.Warnings = append(summary.Warnings, "raw_document_parse_failed")
			continue
		}
		summary.RawDocuments++
		gate := QualityGateForRawDocument(doc)
		if gate.AnalysisUsable {
			summary.AnalysisUsableDocuments++
		}
		if gate.HumanReviewRequired {
			summary.ReviewRequiredDocuments++
		}
		if gate.Status == ParseStatusRejected {
			summary.RejectedDocuments++
		}
		ticker := strings.ToUpper(strings.TrimSpace(doc.Ticker))
		if ticker == "" {
			ticker = ExtractTicker("", doc.FilePath)
		}
		if ticker == "" {
			ticker = "UNKNOWN"
		}
		portfolioSnapshots, valuationNotes := ExtractPortfolioSummaryAndNotes(doc)
		if len(portfolioSnapshots) > 0 {
			summariesByTicker[ticker] = append(summariesByTicker[ticker], portfolioSnapshots...)
		}
		if len(valuationNotes) > 0 {
			notesByTicker[ticker] = append(notesByTicker[ticker], valuationNotes...)
		}
		events := ExtractAssetEvents(doc)
		for _, event := range events {
			if event.Ticker == "" {
				event.Ticker = ticker
			}
			if err := globalEncoder.Encode(event); err != nil {
				return summary, err
			}
			eventsByTicker[event.Ticker] = append(eventsByTicker[event.Ticker], event)
			summary.AssetEvents++
		}
	}
	if err := scanner.Err(); err != nil {
		return summary, err
	}

	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	generatedAt := now().UTC().Format(time.RFC3339)
	tickers := sortedAssetTickers(eventsByTicker)
	for ticker := range summariesByTicker {
		if _, ok := eventsByTicker[ticker]; !ok {
			tickers = append(tickers, ticker)
		}
	}
	tickers = dedupeStrings(tickers)
	sort.Strings(tickers)
	for _, ticker := range tickers {
		events := eventsByTicker[ticker]
		tickerDir := filepath.Join(byTickerDir, ticker)
		if err := os.MkdirAll(tickerDir, 0o755); err != nil {
			return summary, err
		}
		eventsPath := filepath.Join(tickerDir, AssetEventsFile)
		if err := writeAssetEventsJSONL(eventsPath, events); err != nil {
			return summary, err
		}
		inventory := BuildAssetInventory(ticker, events, summariesByTicker[ticker], notesByTicker[ticker], generatedAt)
		inventoryPath := filepath.Join(tickerDir, AssetInventoryFile)
		if err := writeAssetInventoryJSON(inventoryPath, inventory); err != nil {
			return summary, err
		}
		summary.AssetInventories++
		summary.OutputFiles = append(summary.OutputFiles, eventsPath, inventoryPath)
	}
	summary.Tickers = len(tickers)
	summary.OutputFiles = append([]string{globalEventsPath}, summary.OutputFiles...)
	if summary.AssetEvents == 0 {
		summary.Status = "no_assets_found"
		summary.Warnings = append(summary.Warnings, "asset_events_empty")
	}
	summary.Warnings = dedupeStrings(summary.Warnings)
	return summary, nil
}

func BuildAssetInventory(ticker string, events []AssetEvent, portfolioSnapshots []PortfolioSummarySnapshot, notes []ValuationNote, generatedAt string) AssetInventory {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Period != nil && events[j].Period != nil && *events[i].Period != *events[j].Period {
			return *events[i].Period < *events[j].Period
		}
		if events[i].SourceFile == events[j].SourceFile {
			return events[i].AssetName < events[j].AssetName
		}
		return events[i].SourceFile < events[j].SourceFile
	})
	inventory := AssetInventory{
		Ticker:      ticker,
		GeneratedAt: generatedAt,
		EventCount:  len(events),
		Assets:      []AssetInventoryItem{},
	}
	index := map[string][]int{}
	filteredEvents := 0
	for _, original := range events {
		for _, event := range expandInventoryEvents(original) {
			event = normalizeInventoryEvent(event)
			if inventory.CompanyName == "" {
				inventory.CompanyName = event.CompanyName
			}
			if !isReliableInventoryEvent(event) {
				filteredEvents++
				continue
			}
			idx := findMatchingInventoryAsset(index, inventory.Assets, event)
			if idx < 0 {
				inventory.Assets = append(inventory.Assets, inventoryItemFromEvent(event))
				addInventoryIndexKeys(index, inventory.Assets[len(inventory.Assets)-1], len(inventory.Assets)-1)
				continue
			}
			mergeAssetEvent(&inventory.Assets[idx], event)
			addInventoryIndexKeys(index, inventory.Assets[idx], idx)
		}
	}
	sort.SliceStable(inventory.Assets, func(i, j int) bool {
		vi := latestAssetValue(inventory.Assets[i])
		vj := latestAssetValue(inventory.Assets[j])
		if vi == vj {
			return inventory.Assets[i].AssetName < inventory.Assets[j].AssetName
		}
		return vi > vj
	})
	inventory.AssetCount = len(inventory.Assets)
	inventory.PortfolioSummary = buildPortfolioSummary(ticker, portfolioSnapshots, generatedAt)
	inventory.ValuationNotes = limitValuationNotes(dedupeValuationNotes(notes), 500)
	inventory.GYOSummary = buildGYOSummary(ticker, inventory.Assets)
	if inventory.PortfolioSummary.TotalRealEstateValueExclVATTRY != nil {
		inventory.GYOSummary.TotalRealEstateValueExclVATTRY = inventory.PortfolioSummary.TotalRealEstateValueExclVATTRY
	}
	if inventory.PortfolioSummary.TotalRealEstateValueInclVATTRY != nil {
		inventory.GYOSummary.TotalRealEstateValueInclVATTRY = inventory.PortfolioSummary.TotalRealEstateValueInclVATTRY
	}
	if inventory.AssetCount == 0 {
		inventory.Warnings = append(inventory.Warnings, "asset_inventory_empty")
	}
	if filteredEvents > 0 {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("asset_inventory_filtered_candidate_events_%d", filteredEvents))
	}
	return inventory
}

func expandInventoryEvents(event AssetEvent) []AssetEvent {
	expanded := bankOwnedPropertyInventoryEvents(event)
	if len(expanded) == 0 {
		return []AssetEvent{event}
	}
	out := make([]AssetEvent, 0, len(expanded)+1)
	out = append(out, event)
	out = append(out, expanded...)
	return out
}

func bankOwnedPropertyInventoryEvents(event AssetEvent) []AssetEvent {
	text := assetEventReferenceText(event)
	sourceText := assetEventSourceReferenceText(event)
	slug := util.SlugTR(text)
	if !slugContains(slug, "banka mulkiyetinde olan") {
		return nil
	}
	if !slugContains(slug, "izmir hizmet binasi") || !slugContains(slug, "950 ada 6") {
		return nil
	}
	if !slugContains(slug, "3535 ada 8 ve 9") || !slugContains(slug, "3219 ada 165") {
		return nil
	}
	out := []AssetEvent{
		bankOwnedPropertyEvent(event, "İzmir Konak Akdeniz Hizmet Binası", "office", "İzmir İli, Konak İlçesi, Akdeniz Mahallesi", "Izmir", "Konak", "950 ada 6 parsel", bankOwnedPropertyValue(sourceText, "İzmir Hizmet Binası")),
		bankOwnedPropertyEvent(event, "İzmir Konak Umurbey Arsaları", "land", "İzmir İli, Konak İlçesi, Umurbey Mahallesi", "Izmir", "Konak", "3535 ada 8 ve 9 parseller", bankOwnedPropertyValue(sourceText, "arsaların")),
		bankOwnedPropertyEvent(event, "İstanbul Ataşehir İçerenköy Binaları", "office", "İstanbul İli, Ataşehir İlçesi, İçerenköy Mahallesi", "Istanbul", "Ataşehir", "3219 ada 165 parsel", bankOwnedPropertyValue(sourceText, "taşınmazların")),
	}
	return out
}

func bankOwnedPropertyEvent(base AssetEvent, name, assetType, location, city, district, parcel string, value *float64) AssetEvent {
	event := base
	event.AssetName = name
	event.AssetType = assetType
	event.Location = location
	event.City = city
	event.District = district
	event.ParcelInfo = parcel
	event.OwnershipType = "owned"
	event.Status = "unknown"
	event.ExpertiseValueExclVATTRY = value
	event.ExpertiseValueInclVATTRY = nil
	event.BookValueTRY = nil
	event.RentalInfo = AssetRentalInfo{}
	event.IncomeRelevance = "medium"
	event.RiskFlags = nil
	event.OpportunityFlags = nil
	if event.Confidence < 0.95 {
		event.Confidence = 0.95
	}
	return event
}

func bankOwnedPropertyValue(text, anchor string) *float64 {
	idx := strings.Index(text, anchor)
	if idx < 0 {
		return nil
	}
	segment := text[idx:]
	if end := strings.Index(segment, "bedel üzerinden"); end >= 0 {
		segment = segment[:end]
	}
	amounts := extractMoneyAmounts(segment)
	for _, amount := range amounts {
		if amount.Currency == "" || amount.Currency == "TRY" {
			return floatPtr(amount.Value)
		}
	}
	return nil
}

func assetEventReferenceText(event AssetEvent) string {
	parts := []string{event.AssetName, event.Location, event.ParcelInfo}
	for _, ref := range event.SourceReferences {
		parts = append(parts, ref.Snippet)
	}
	return strings.Join(parts, " ")
}

func assetEventSourceReferenceText(event AssetEvent) string {
	parts := []string{}
	for _, ref := range event.SourceReferences {
		parts = append(parts, ref.Snippet)
	}
	return strings.Join(parts, " ")
}

func enrichInventoryEventFromReferences(event AssetEvent) AssetEvent {
	source := assetEventSourceReferenceText(event)
	if strings.TrimSpace(source) == "" {
		return event
	}
	segment := inventoryAssetSegment(source, event.AssetName)
	if segment == "" && !assetLineHasMultipleKnownAssets(source) {
		segment = source
	}
	if segment == "" {
		return event
	}
	if event.Location == "" || event.City == "" || event.District == "" {
		location, city, district := extractLocation(segment)
		if event.Location == "" {
			event.Location = location
		}
		if event.City == "" {
			event.City = city
		}
		if event.District == "" {
			event.District = district
		}
	}
	if event.AreaM2 == nil {
		event.AreaM2 = extractAreaM2(segment)
	}
	if strings.TrimSpace(event.ParcelInfo) == "" {
		event.ParcelInfo = extractParcelInfo(segment)
	}
	excl, incl, book, flags := extractAssetValues(segment)
	hasSegmentValue := excl != nil || incl != nil || book != nil
	if hasSegmentValue && (assetLineHasMultipleKnownAssets(source) || segment != source) {
		event.ExpertiseValueExclVATTRY = excl
		event.ExpertiseValueInclVATTRY = incl
		event.BookValueTRY = book
		event.RiskFlags = dedupeStrings(append(removeWarnings(event.RiskFlags, "vat_status_unknown_for_expertise_value"), flags...))
	} else if event.ExpertiseValueExclVATTRY == nil && event.ExpertiseValueInclVATTRY == nil && event.BookValueTRY == nil {
		event.ExpertiseValueExclVATTRY = excl
		event.ExpertiseValueInclVATTRY = incl
		event.BookValueTRY = book
		event.RiskFlags = dedupeStrings(append(event.RiskFlags, flags...))
	}
	if strings.TrimSpace(event.Location) != "" || strings.TrimSpace(event.ParcelInfo) != "" {
		event.RiskFlags = removeWarnings(event.RiskFlags, "location_or_parcel_missing")
	}
	return event
}

func inventoryAssetSegment(text, assetName string) string {
	text = strings.TrimSpace(text)
	if text == "" || strings.TrimSpace(assetName) == "" {
		return ""
	}
	slug, indexMap := slugWithIndex(text)
	if slug == "" || len(indexMap) == 0 {
		return ""
	}
	startSlug, aliasLen := -1, 0
	for _, alias := range inventoryAssetAliases(assetName) {
		aliasSlug := util.SlugTR(alias)
		if aliasSlug == "" {
			continue
		}
		idx := strings.Index(slug, aliasSlug)
		if idx < 0 {
			continue
		}
		if startSlug < 0 || idx < startSlug || (idx == startSlug && len(aliasSlug) > aliasLen) {
			startSlug = idx
			aliasLen = len(aliasSlug)
		}
	}
	if startSlug < 0 {
		return ""
	}
	searchFrom := startSlug + aliasLen
	if searchFrom < startSlug+1 {
		searchFrom = startSlug + 1
	}
	endSlug := len(slug)
	for _, alias := range inventoryKnownAssetAliases() {
		aliasSlug := util.SlugTR(alias)
		if aliasSlug == "" || searchFrom >= len(slug) {
			continue
		}
		if rel := strings.Index(slug[searchFrom:], aliasSlug); rel >= 0 {
			idx := searchFrom + rel
			if idx > startSlug && idx < endSlug {
				endSlug = idx
			}
		}
	}
	runes := []rune(text)
	startRune := indexMap[startSlug]
	endRune := len(runes)
	if endSlug < len(indexMap) {
		endRune = indexMap[endSlug]
	}
	if startRune < 0 || startRune >= len(runes) || endRune <= startRune {
		return ""
	}
	segment := strings.TrimSpace(string(runes[startRune:endRune]))
	sourceSlug := util.SlugTR(text)
	segmentSlug := util.SlugTR(segment)
	prefix := ""
	if slugContains(sourceSlug, "kdv haric") && slugContains(sourceSlug, "kdv dahil") && !(slugContains(segmentSlug, "kdv haric") && slugContains(segmentSlug, "kdv dahil")) {
		prefix += "KDV Hariç KDV Dahil "
	}
	if (slugContains(sourceSlug, "gercege uygun degeri") || slugContains(sourceSlug, "rayic deger")) &&
		!(slugContains(segmentSlug, "gercege uygun degeri") || slugContains(segmentSlug, "rayic deger")) {
		prefix += "Gerçeğe Uygun Değeri (TL) "
	}
	return strings.TrimSpace(prefix + segment)
}

func inventoryAssetAliases(assetName string) []string {
	aliases := []string{assetName}
	slug := util.SlugTR(assetName)
	add := func(values ...string) {
		aliases = append(aliases, values...)
	}
	switch {
	case slugContains(slug, "fethiye hillside") || slugContains(slug, "hillside beach club"):
		add("Hillside Beach Club Tatil Köyü", "Fethiye Hillside Beach Club Tatil", "Fethiye Hillside Beach Club", "Hillside Beach Club")
	case slugContains(slug, "bodrum hillside") || slugContains(slug, "bodrum otel"):
		add("Bodrum Otel", "Bodrum Hillside Otel")
	case slugContains(slug, "buyukcekmece eskice") || slugContains(slug, "buyukcekmece arsasi"):
		add("Büyükçekmece Arsası", "Büyükçekmece Eskice Köyü Arsası")
	case slugContains(slug, "maslak"):
		add("Maslak Arsası")
	case slugContains(slug, "mosalarko"):
		add("Mosalarko Ofis Binası")
	case slugContains(slug, "eyup topcular"):
		add("Eyüp Topçular – Fabrika", "Eyüp Topçular - Fabrika", "Eyüp - Topçular Kargir Fabrika", "Eyüp Topçular Fabrika")
	case slugContains(slug, "etiler alkent"):
		add("Etiler Alkent Sitesi – Dükkanlar", "Etiler Alkent Sitesi - Dükkanlar", "Etiler Alkent Çarşı 39 Adet Dükkan")
	case slugContains(slug, "istanbul karakoy"):
		add("İstanbul Karaköy İş Merkezi", "Karaköy İş Merkezi")
	case slugContains(slug, "buyukcekmece alkent"):
		add("Büyükçekmece Alkent 2000 – Dükkanlar", "Büyükçekmece Alkent 2000 - Dükkanlar", "Büyükçekmece Alkent 2000 10 Adet Dükkan")
	case slugContains(slug, "ankara cankaya"):
		add("Ankara Çankaya İş Merkezi", "Çankaya İş Merkezi")
	case slugContains(slug, "sishane"):
		add("İstanbul Şişhane İş Merkezi", "Şişhane İş Merkezi")
	case slugContains(slug, "alarko dim") || slugContains(slug, "tepebasi"):
		add("Alarko-DİM İş Merkezi", "Alarko DIM İş Merkezi", "İstanbul Tepebaşı Alarko-DİM İş Merkezi")
	}
	return dedupeStrings(aliases)
}

func inventoryKnownAssetAliases() []string {
	return dedupeStrings([]string{
		"Hillside Beach Club Tatil Köyü", "Fethiye Hillside Beach Club Tatil", "Fethiye Hillside Beach Club", "Hillside Beach Club",
		"Bodrum Otel", "Bodrum Hillside Otel",
		"Büyükçekmece Arsası", "Büyükçekmece Eskice Köyü Arsası",
		"Maslak Arsası", "Mosalarko Ofis Binası",
		"Eyüp Topçular – Fabrika", "Eyüp Topçular - Fabrika", "Eyüp - Topçular Kargir Fabrika", "Eyüp Topçular Fabrika",
		"Etiler Alkent Sitesi – Dükkanlar", "Etiler Alkent Sitesi - Dükkanlar", "Etiler Alkent Çarşı 39 Adet Dükkan",
		"İstanbul Karaköy İş Merkezi", "Karaköy İş Merkezi",
		"Büyükçekmece Alkent 2000 – Dükkanlar", "Büyükçekmece Alkent 2000 - Dükkanlar", "Büyükçekmece Alkent 2000 10 Adet Dükkan",
		"Ankara Çankaya İş Merkezi", "Çankaya İş Merkezi",
		"İstanbul Şişhane İş Merkezi", "Şişhane İş Merkezi",
		"Alarko-DİM İş Merkezi", "Alarko DIM İş Merkezi", "İstanbul Tepebaşı Alarko-DİM İş Merkezi",
	})
}

func isReliableInventoryEvent(event AssetEvent) bool {
	name := strings.TrimSpace(event.AssetName)
	if name == "" || event.Confidence < 0.60 || event.AssetType == "" || event.AssetType == "other" {
		return false
	}
	if event.AssetType == "subsidiary" || event.AssetType == "financial_asset" {
		return false
	}
	if assetInventoryEventLooksFinancialCompany(event) {
		return false
	}
	if strings.HasPrefix(name, "(") || strings.Contains(name, "###") || strings.Contains(name, "%") {
		return false
	}
	if strings.HasPrefix(name, "•") || strings.HasPrefix(name, "") || strings.Contains(name, "�") {
		return false
	}
	if strings.Contains(name, ",") && !assetUnitCodeRE.MatchString(name) && !inventoryAssetNameHasStrongIdentity(name) {
		return false
	}
	if firstRuneIsLower(name) {
		return false
	}
	if len([]rune(name)) > 120 && !assetUnitCodeRE.MatchString(name) && !inventoryAssetNameHasStrongIdentity(name) {
		return false
	}
	if assetInventoryNameLooksNarrative(name) || assetInventoryEventHasImplausibleUnitArea(event) || assetInventoryEventHasOnlyTinyValue(event) {
		return false
	}
	if assetNameLooksGeneric(name) {
		return false
	}
	if assetUnitCodeRE.MatchString(name) && event.AssetType != "shop" {
		return false
	}
	for _, ref := range event.SourceReferences {
		if assetLineHasMultipleKnownAssets(ref.Snippet) && event.AreaM2 != nil && !assetUnitCodeRE.MatchString(name) && !inventoryEventHasValueOrRent(event) {
			return false
		}
	}
	evidence := inventoryEventEvidenceCount(event)
	if assetUnitCodeRE.MatchString(name) {
		return evidence >= 3
	}
	if inventoryAssetNameHasStrongIdentity(name) && event.Confidence >= 0.68 {
		return evidence >= 2
	}
	if event.Confidence >= 0.85 && inventoryEventHasPhysicalAnchor(event) {
		return evidence >= 4
	}
	return evidence >= 5
}

func inventoryAssetNameHasStrongIdentity(value string) bool {
	trimmed := strings.TrimSpace(value)
	if assetUnitCodeRE.MatchString(trimmed) || assetValuationCodeRE.MatchString(trimmed) {
		return true
	}
	slug := util.SlugTR(trimmed)
	terms := []string{
		"hillside beach club", "fethiye hillside", "bodrum hillside", "bodrum otel",
		"alarko dim", "alarko-dim", "dim is merkezi", "alarko is merkezi",
		"karakoy is merkezi", "sishane is merkezi", "cankaya is merkezi",
		"ankara cankaya", "buyukcekmece alkent", "buyukcekmece eskice",
		"maslak arsasi", "sariyer maslak", "kalemya koyu", "fethiye kalemya",
		"etiler alkent", "alkent carsi", "alkent alisveris", "eyup topcular", "topcular fabrika",
		"kargir fabrika", "fabrika binasi", "mosalarko", "ojsc mosalarko", "alarko deyaar",
	}
	for _, term := range terms {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func assetInventoryEventLooksFinancialCompany(event AssetEvent) bool {
	name := strings.TrimSpace(event.AssetName)
	if name == "" || event.AreaM2 != nil || strings.TrimSpace(event.ParcelInfo) != "" ||
		event.ExpertiseValueExclVATTRY != nil || event.ExpertiseValueInclVATTRY != nil || event.BookValueTRY != nil ||
		event.RentalInfo.MonthlyRentTRY != nil || event.RentalInfo.AnnualRentTRY != nil || event.RentalInfo.AnnualMinRentUSD != nil {
		return false
	}
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, "a.ş") || strings.Contains(lowerName, "a. s") || strings.Contains(lowerName, "a.s") || strings.Contains(lowerName, "a.ş.") {
		return true
	}
	slug := util.SlugTR(name)
	legalEntityTerms := []string{
		"anonim sirket", "limited sirket", "holding", "bankasi",
		"menkul deger", "yatirim ortakligi", "finansal kiralama", "varlik yonetim",
	}
	for _, term := range legalEntityTerms {
		if slugContains(slug, term) {
			return true
		}
	}
	return false
}

func assetInventoryNameLooksNarrative(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	slug := util.SlugTR(trimmed)
	blockedContains := []string{
		"kirası bu dukkanlardan", "kirasi bu dukkanlardan", "elde edilecek",
		"hissedarimiz", "hissedarımız", "adına kayıtlı", "adina kayitli",
		"bünyesinde", "bunyesinde", "projesinin son bolum", "projesinin son bölüm",
		"projesinin son faz", "kapasiteli", "konumludur", "katlar dir", "katlar dır",
		"ile ilgili olarak", "yer alan alkent alisveris", "alışveriş merkezinde bulunan 39",
		"alisveris merkezinde bulunan 39", "otelinden elde", "otel den elde",
		"rezervasyonlarda", "sunlari belirtti", "gayrimenkul yatirim ortakligi",
		"nitelikli tasinmazin guncel pazar", "nitelikli taşınmazın güncel pazar",
		"topcular mahallesinde", "topçular mahallesinde", "konumlu", "tatil koyumuzun ust hakki",
		"tatil köyümüzün üst hakkı", "suresinin yeniden", "süresinin yeniden",
		"ogrenilmistir", "öğrenilmiştir", "tapusudur", "(kira", "kira-",
		"ndeki", "nda bulunan", "nde bulunan", "nde 39 adet", "nda 100",
		"istanbul ili", "ili besiktas ilcesi", "ili, besiktas ilcesi",
		"fethiye mugla", "muğla ili", "mugla ili", "eyup istanbul", "eyüp istanbul",
		"kaya koyu kalemya koyu", "kalemya koyu nda", "kalemya koyu n da",
		"arsa maliyeti", "degerleme yontemi", "değerleme yöntemi",
		"modeli uygulanabilir", "varsayilan", "varsayılan", "no lu parselde",
		"nakit akisi", "nakit akışı", "maliyet yaklasimi", "maliyet yaklaşımı",
		"emsal karsilastirma", "emsal karşılaştırma", "gelir indirgeme",
		"bodrum belediyesi imar mudurlugu", "bodrum belediyesi imar müdürlüğü",
		"konu tasinmaz", "konu taşınmaz", "soz konusu gayrimenkul", "söz konusu gayrimenkul",
		"tapudaki vasiflari", "tapudaki vasıfları", "tapu kaydi", "tapu kaydı",
		"cadde uzerinde yer alan", "cadde üzerinde yer alan", "bulundugu cadde",
		"bulunduğu cadde", "uygun arazi arastirmalarimiz", "uygun arazi araştırmalarımız",
		"fizibilite calismalarimiz", "fizibilite çalışmalarımız", "elde ettigimi",
		"elde ettiğimi", "gunumuz piyasa kosullarinda", "günümüz piyasa koşullarında",
		"karsisinda", "karşısında", "caddesi uzerinde", "caddesi üzerinde",
		"tomtom", "green beach resort", "bodrum kati", "bodrum katı",
		"buyukcekmece deki alkent", "büyükçekmece deki alkent", "en prestijli bolum",
		"en prestijli bölüm", "eyup te 13503", "eyüp te 13503", "istanbul eyup te 13503",
		"istanbul eyüp te 13503",
		"belediye meclisi", "meclis karari", "meclis kararı", "tarih ve",
		"yapilan incelemelere", "yapılan incelemelere", "belediyesinde",
		"imar mudurlugu", "imar müdürlüğü", "rapor tarihinde",
	}
	for _, item := range blockedContains {
		if slugContains(slug, item) {
			return true
		}
	}
	blockedSuffixes := []string{
		" ve", " nin", " nın", " nun", " nün", " dir", " dır", " icin", " için",
		" bulunan", " yer alan", " nde", " nda",
	}
	for _, suffix := range blockedSuffixes {
		if strings.HasSuffix(slug, util.SlugTR(suffix)) {
			return true
		}
	}
	if strings.HasPrefix(slug, "olan ") || strings.HasPrefix(slug, "no lu parselde") || strings.HasPrefix(slug, "bes yildizli") || strings.HasPrefix(slug, "yatak kapasiteli") {
		return true
	}
	words := strings.Fields(trimmed)
	if len(words) > 12 && !assetUnitCodeRE.MatchString(trimmed) {
		return true
	}
	return false
}

func normalizeInventoryEvent(event AssetEvent) AssetEvent {
	event.AssetName = normalizeInventoryAssetName(event.AssetName)
	event = enrichInventoryEventFromReferences(event)
	event.Location = cleanInventoryLocation(event.Location)
	if canonical := canonicalInventoryLocation(event.AssetName); canonical != "" {
		event.Location = canonical
	}
	event.City = normalizeInventoryCity(event.AssetName, event.Location, event.City)
	event.District = normalizeInventoryDistrict(event.AssetName, event.Location, event.District)
	event.ParcelInfo = normalizeParcelInfo(event.ParcelInfo)
	if event.Location != "" && locationContradictsInventoryCity(event.Location, event.City) {
		event.Location = ""
	}
	return event
}

func normalizeInventoryAssetName(value string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	trimmed = normalizeInventoryGlyphs(trimmed)
	if trimmed == "" || assetUnitCodeRE.MatchString(trimmed) {
		return trimmed
	}
	lower := strings.ToLower(trimmed)
	slug := util.SlugTR(trimmed)
	switch {
	case slugContains(slug, "fethiye hillside beach club") || slugContains(slug, "hillside beach club tatil"):
		return "Fethiye Hillside Beach Club"
	case slugContains(slug, "bodrum hillside") || slugContains(slug, "bodrum otel"):
		return "Bodrum Hillside Otel"
	case slugContains(slug, "mosalarko"):
		return "Mosalarko Ofis Binası"
	case slugContains(slug, "alarko dim is merkezi") || slugContains(slug, "alarko-dim is merkezi") || slugContains(slug, "dim is merkezi"):
		return "İstanbul Tepebaşı Alarko-DİM İş Merkezi"
	case (slugContains(slug, "buyukcekmece alkent") || slugContains(slug, "alkent 2000")) &&
		(slugContains(slug, "10 adet") || slugContains(slug, "10 ad") || slugContains(slug, "784") || slugContains(slug, "dukkanlar")) &&
		(slugContains(slug, "dukkan") || slugContains(slug, "magaza")):
		return "Büyükçekmece Alkent 2000 10 Adet Dükkan"
	case slugContains(slug, "ankara cankaya is merkezi") || slugContains(slug, "cankaya is merkezi") || (slugContains(slug, "ankara cankaya") && slugContains(slug, "alarko is merkezi")):
		return "Ankara Çankaya İş Merkezi"
	case slugContains(slug, "karakoy is merkezi"):
		return "İstanbul Karaköy İş Merkezi"
	case slugContains(slug, "sishane is merkezi"):
		return "İstanbul Şişhane İş Merkezi"
	case (slugContains(slug, "etiler alkent") || slugContains(slug, "alkent carsi")) && slugContains(slug, "dukkan"):
		return "Etiler Alkent Çarşı 39 Adet Dükkan"
	case slugContains(slug, "eyup topcular") && slugContains(slug, "kargir fabrika"):
		return "Eyüp Topçular Fabrika"
	case slugContains(slug, "topcular mahallesi") && slugContains(slug, "kargir fabrika"):
		return "Eyüp Topçular Fabrika"
	case slugContains(slug, "topcular sanayii tesisi"):
		return "Eyüp Topçular Fabrika"
	case slugContains(slug, "topcular fabrikasi") || slugContains(slug, "topcular fabrika"):
		return "Eyüp Topçular Fabrika"
	case slugContains(slug, "maslak arsasi"):
		return "Maslak Arsası"
	case slugContains(slug, "buyukcekmece eskice koyu arsasi") && (strings.Contains(lower, "-a") || strings.Contains(lower, " a ") || strings.Contains(lower, "(2)")):
		return "Büyükçekmece Eskice Köyü Arsası - A"
	case slugContains(slug, "buyukcekmece eskice koyu arsasi") && (strings.Contains(lower, "-b") || strings.Contains(lower, " b ") || strings.Contains(lower, "(3)")):
		return "Büyükçekmece Eskice Köyü Arsası - B"
	case slugContains(slug, "buyukcekmece arsasi") || slugContains(slug, "buyukcekmece eskice koyu arsasi") || slugContains(slug, "buyukcekmece eskice"):
		return "Büyükçekmece Eskice Köyü Arsası"
	case slugContains(slug, "bahceli kargir fabrika ve mustemilat binalari") || slugContains(slug, "bahceli kagir fabrika ve mustemilat binalari"):
		return "Bahçeli Kargir Fabrika ve Müştemilat Binaları"
	}
	if strings.Contains(trimmed, ",") || assetInventoryNameLooksNarrative(trimmed) {
		return trimmed
	}
	return trimmed
}

func normalizeInventoryGlyphs(value string) string {
	replacer := strings.NewReplacer(
		"Đş", "İş",
		"ĐŞ", "İŞ",
		"‹fl", "İş",
		"‹FL", "İŞ",
		"‹ﬂ", "İş",
		"ﬂ", "ş",
		"Ģ", "ş",
		"Ġ", "İ",
	)
	return replacer.Replace(value)
}

func inventoryEventEvidenceCount(event AssetEvent) int {
	return assetEvidenceCount(
		event.AssetName,
		event.AssetType,
		cleanInventoryLocation(event.Location),
		event.City,
		event.District,
		event.AreaM2,
		event.ParcelInfo,
		event.ExpertiseDate,
		event.ExpertiseValueExclVATTRY,
		event.ExpertiseValueInclVATTRY,
		event.BookValueTRY,
		event.RentalInfo,
	)
}

func inventoryEventHasPhysicalAnchor(event AssetEvent) bool {
	return cleanInventoryLocation(event.Location) != "" ||
		strings.TrimSpace(event.City) != "" ||
		strings.TrimSpace(event.District) != "" ||
		strings.TrimSpace(event.ParcelInfo) != "" ||
		event.AreaM2 != nil
}

func inventoryEventHasValueOrRent(event AssetEvent) bool {
	return event.ExpertiseDate != nil ||
		event.ExpertiseValueExclVATTRY != nil ||
		event.ExpertiseValueInclVATTRY != nil ||
		event.BookValueTRY != nil ||
		event.RentalInfo.MonthlyRentTRY != nil ||
		event.RentalInfo.AnnualRentTRY != nil ||
		event.RentalInfo.AnnualMinRentUSD != nil
}

func firstRuneIsLower(value string) bool {
	for _, r := range strings.TrimSpace(value) {
		return unicode.IsLower(r)
	}
	return false
}

func assetInventoryEventHasImplausibleUnitArea(event AssetEvent) bool {
	if !assetUnitCodeRE.MatchString(strings.TrimSpace(event.AssetName)) || event.AreaM2 == nil {
		return false
	}
	if *event.AreaM2 > 5000 {
		return true
	}
	return false
}

func assetInventoryEventHasOnlyTinyValue(event AssetEvent) bool {
	if event.AreaM2 != nil || strings.TrimSpace(event.ParcelInfo) != "" || strings.TrimSpace(event.Location) != "" ||
		strings.TrimSpace(event.City) != "" || strings.TrimSpace(event.District) != "" ||
		event.RentalInfo.MonthlyRentTRY != nil || event.RentalInfo.AnnualRentTRY != nil || event.RentalInfo.AnnualMinRentUSD != nil {
		return false
	}
	values := []*float64{event.ExpertiseValueExclVATTRY, event.ExpertiseValueInclVATTRY, event.BookValueTRY}
	hasValue := false
	for _, value := range values {
		if value == nil {
			continue
		}
		hasValue = true
		if *value >= 100000 {
			return false
		}
	}
	return hasValue
}

func buildPortfolioSummary(ticker string, snapshots []PortfolioSummarySnapshot, generatedAt string) PortfolioSummary {
	summary := PortfolioSummary{Ticker: ticker}
	seen := map[string]bool{}
	for _, snapshot := range snapshots {
		key := snapshot.SourceFile + "|" + snapshot.Snippet
		if seen[key] {
			continue
		}
		seen[key] = true
		summary.History = append(summary.History, snapshot)
		if snapshot.Snippet != "" {
			summary.SourceReferences = appendAssetReferences(summary.SourceReferences, []AssetSourceReference{{Snippet: snapshot.Snippet}}, 20)
		}
	}
	sort.SliceStable(summary.History, func(i, j int) bool {
		left := portfolioSnapshotSortKey(summary.History[i])
		right := portfolioSnapshotSortKey(summary.History[j])
		if left == right {
			return summary.History[i].SourceFile < summary.History[j].SourceFile
		}
		return left < right
	})
	selectedExclKey := ""
	selectedInclKey := ""
	selectedBookKey := ""
	for i := len(summary.History) - 1; i >= 0; i-- {
		item := summary.History[i]
		if summary.TotalRealEstateValueExclVATTRY == nil && item.TotalRealEstateValueExclVATTRY != nil {
			summary.TotalRealEstateValueExclVATTRY = item.TotalRealEstateValueExclVATTRY
			selectedExclKey = portfolioSnapshotSortKey(item)
		}
		if summary.TotalRealEstateValueInclVATTRY == nil && item.TotalRealEstateValueInclVATTRY != nil {
			summary.TotalRealEstateValueInclVATTRY = item.TotalRealEstateValueInclVATTRY
			selectedInclKey = portfolioSnapshotSortKey(item)
		}
		if summary.TotalBookValueTRY == nil && item.TotalBookValueTRY != nil {
			summary.TotalBookValueTRY = item.TotalBookValueTRY
			selectedBookKey = portfolioSnapshotSortKey(item)
		}
	}
	generatedYear := firstYearInPath(generatedAt)
	if generatedYear != "" {
		if portfolioSummaryKeyIsStale(selectedExclKey, generatedYear) {
			summary.TotalRealEstateValueExclVATTRY = nil
			summary.Warnings = append(summary.Warnings, "portfolio_total_excl_vat_stale_"+selectedExclKey)
		}
		if portfolioSummaryKeyIsStale(selectedInclKey, generatedYear) {
			summary.TotalRealEstateValueInclVATTRY = nil
			summary.Warnings = append(summary.Warnings, "portfolio_total_incl_vat_stale_"+selectedInclKey)
		}
		if portfolioSummaryKeyIsStale(selectedBookKey, generatedYear) {
			summary.TotalBookValueTRY = nil
			summary.Warnings = append(summary.Warnings, "portfolio_total_book_value_stale_"+selectedBookKey)
		}
	}
	if summary.TotalRealEstateValueExclVATTRY == nil && summary.TotalRealEstateValueInclVATTRY == nil && summary.TotalBookValueTRY == nil {
		summary.Warnings = append(summary.Warnings, "portfolio_total_not_found")
	}
	return summary
}

func portfolioSnapshotSortKey(item PortfolioSummarySnapshot) string {
	if strings.TrimSpace(item.Period) != "" {
		return item.Period
	}
	if strings.TrimSpace(item.DocumentDate) != "" {
		return item.DocumentDate
	}
	if year := firstYearInPath(item.SourceFile); year != "" {
		return year
	}
	return ""
}

func firstYearInPath(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r < '0' || r > '9'
	})
	for _, part := range parts {
		if len(part) == 4 && part >= "1990" && part <= "2099" {
			return part
		}
	}
	return ""
}

func portfolioSummaryKeyIsStale(key, generatedYear string) bool {
	if len(key) < 4 || len(generatedYear) < 4 {
		return false
	}
	keyYear, okKey := parseFourDigitYear(key[:4])
	generated, okGenerated := parseFourDigitYear(generatedYear[:4])
	if !okKey || !okGenerated {
		return false
	}
	return generated-keyYear > 3
}

func parseFourDigitYear(value string) (int, bool) {
	if len(value) != 4 {
		return 0, false
	}
	year := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		year = year*10 + int(r-'0')
	}
	return year, year >= 1990 && year <= 2099
}

func dedupeValuationNotes(notes []ValuationNote) []ValuationNote {
	out := []ValuationNote{}
	seen := map[string]bool{}
	for _, note := range notes {
		key := note.SourceFile + "|" + note.NoteType + "|" + note.Snippet
		if note.Snippet == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, note)
	}
	return out
}

func limitValuationNotes(notes []ValuationNote, limit int) []ValuationNote {
	if limit <= 0 || len(notes) <= limit {
		return notes
	}
	return notes[:limit]
}

func inventoryItemFromEvent(event AssetEvent) AssetInventoryItem {
	location := cleanInventoryLocation(event.Location)
	return AssetInventoryItem{
		AssetName:        event.AssetName,
		AssetType:        event.AssetType,
		Location:         location,
		City:             event.City,
		District:         event.District,
		AreaM2:           event.AreaM2,
		ParcelInfo:       event.ParcelInfo,
		OwnershipType:    event.OwnershipType,
		Status:           event.Status,
		RentalInfo:       event.RentalInfo,
		IncomeRelevance:  event.IncomeRelevance,
		RiskFlags:        append([]string{}, event.RiskFlags...),
		OpportunityFlags: append([]string{}, event.OpportunityFlags...),
		SourceReferences: append([]AssetSourceReference{}, event.SourceReferences...),
		Confidence:       event.Confidence,
		History:          []AssetHistoryPoint{historyPointFromEvent(event)},
	}
}

func mergeAssetEvent(item *AssetInventoryItem, event AssetEvent) {
	if event.Confidence > item.Confidence {
		item.Confidence = event.Confidence
	}
	item.AssetType = preferredInventoryAssetType(item.AssetName, item.AssetType, event.AssetType)
	location := cleanInventoryLocation(event.Location)
	if location != "" && (item.Location == "" || assetInventoryLocationLooksNoisy(item.Location) || betterInventoryLocation(item.Location, location)) {
		item.Location = location
	}
	if item.City == "" {
		item.City = event.City
	}
	if item.District == "" {
		item.District = event.District
	}
	if item.AreaM2 == nil {
		item.AreaM2 = event.AreaM2
	}
	eventParcel := normalizeParcelInfo(event.ParcelInfo)
	if eventParcel != "" && (item.ParcelInfo == "" || betterInventoryParcel(item.AssetName, item.ParcelInfo, eventParcel)) {
		item.ParcelInfo = eventParcel
	}
	if item.OwnershipType == "" || item.OwnershipType == "unknown" {
		item.OwnershipType = event.OwnershipType
	}
	if item.Status == "" || item.Status == "unknown" {
		item.Status = event.Status
	}
	item.RentalInfo = mergeRentalInfo(item.RentalInfo, event.RentalInfo)
	if item.IncomeRelevance == "" || item.IncomeRelevance == "unknown" || event.IncomeRelevance == "high" {
		item.IncomeRelevance = event.IncomeRelevance
	}
	item.RiskFlags = dedupeStrings(append(item.RiskFlags, event.RiskFlags...))
	item.OpportunityFlags = dedupeStrings(append(item.OpportunityFlags, event.OpportunityFlags...))
	item.SourceReferences = appendAssetReferences(item.SourceReferences, event.SourceReferences, 8)
	history := historyPointFromEvent(event)
	if !historyExists(item.History, history) {
		item.History = append(item.History, history)
	}
	sort.SliceStable(item.History, func(i, j int) bool {
		return item.History[i].Period < item.History[j].Period
	})
}

func cleanInventoryLocation(value string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if idx := strings.Index(trimmed, ","); idx > 0 && cityInText(trimmed[:idx]) != "" {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	if idx := strings.Index(trimmed, " ,"); idx > 0 && cityInText(trimmed[:idx]) != "" {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	if trimmed == "" || assetInventoryLocationLooksNoisy(trimmed) {
		return ""
	}
	if len([]rune(trimmed)) > 160 {
		return ""
	}
	return trimmed
}

func canonicalInventoryLocation(assetName string) string {
	slug := util.SlugTR(assetName)
	switch {
	case slugContains(slug, "fethiye hillside beach club"):
		return "Fethiye / Muğla"
	case slugContains(slug, "bodrum hillside otel"):
		return "Bodrum / Muğla"
	case slugContains(slug, "buyukcekmece eskice koyu arsasi"):
		return "Büyükçekmece / İstanbul"
	case slugContains(slug, "buyukcekmece alkent 2000"):
		return "Büyükçekmece / İstanbul"
	case slugContains(slug, "maslak arsasi") || slugContains(slug, "mosalarko"):
		return "Sarıyer / İstanbul"
	case slugContains(slug, "topcular fabrika"):
		return "Eyüp / İstanbul"
	case slugContains(slug, "etiler alkent"):
		return "Beşiktaş / İstanbul"
	case slugContains(slug, "karakoy is merkezi"):
		return "Karaköy / İstanbul"
	case slugContains(slug, "sishane is merkezi") || slugContains(slug, "tepebasi alarko"):
		return "Beyoğlu / İstanbul"
	case slugContains(slug, "ankara cankaya is merkezi"):
		return "Çankaya / Ankara"
	}
	return ""
}

func normalizeInventoryCity(assetName, location, city string) string {
	city = strings.TrimSpace(city)
	expected := expectedInventoryCity(assetName + " " + location)
	if expected != "" {
		return expected
	}
	if assetInventoryLocationLooksNoisy(city) {
		return ""
	}
	return city
}

func normalizeInventoryDistrict(assetName, location, district string) string {
	if expected := expectedInventoryDistrict(assetName + " " + location); expected != "" {
		return expected
	}
	return cleanInventoryDistrict(district)
}

func cleanInventoryDistrict(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || assetInventoryLocationLooksNoisy(value) {
		return ""
	}
	slug := util.SlugTR(value)
	if dateRE.MatchString(value) || slugContains(slug, "yil") || strings.Contains(slug, "fib") || slugContains(slug, "kira") ||
		slugContains(slug, "ekspertiz") || slugContains(slug, "hillside") || slugContains(slug, "beach club") ||
		slugContains(slug, "otel") || slugContains(slug, "is merkezi") {
		return ""
	}
	if cityInText(value) != "" && len(strings.Fields(value)) > 1 {
		return ""
	}
	if areaRE.MatchString(value) && len(strings.Fields(value)) <= 4 {
		return ""
	}
	if len([]rune(value)) > 40 {
		return ""
	}
	return value
}

func expectedInventoryCity(text string) string {
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

func expectedInventoryDistrict(text string) string {
	slug := util.SlugTR(text)
	switch {
	case slugContains(slug, "topcular") || slugContains(slug, "eyup"):
		return "Eyüp"
	case slugContains(slug, "etiler") || slugContains(slug, "besiktas"):
		return "Beşiktaş"
	case slugContains(slug, "karakoy"):
		return "Karaköy"
	case slugContains(slug, "sishane") || slugContains(slug, "tepebasi") || slugContains(slug, "alarko dim"):
		return "Beyoğlu"
	case slugContains(slug, "maslak") || slugContains(slug, "sariyer"):
		return "Sarıyer"
	case slugContains(slug, "buyukcekmece"):
		return "Büyükçekmece"
	case slugContains(slug, "bodrum"):
		return "Bodrum"
	case slugContains(slug, "fethiye") || slugContains(slug, "kalemya"):
		return "Fethiye"
	case slugContains(slug, "cankaya"):
		return "Çankaya"
	default:
		return ""
	}
}

func locationContradictsInventoryCity(location, city string) bool {
	if strings.TrimSpace(location) == "" || strings.TrimSpace(city) == "" {
		return false
	}
	found := cityInText(location)
	return found != "" && !strings.EqualFold(found, city)
}

func assetInventoryLocationLooksNoisy(value string) bool {
	trimmed := strings.TrimLeft(strings.TrimSpace(value), "- ")
	if trimmed != "" {
		first := []rune(trimmed)[0]
		if first >= '0' && first <= '9' {
			return true
		}
	}
	slug := util.SlugTR(value)
	if slug == "" {
		return true
	}
	if strings.Contains(value, "(TL)") || strings.Contains(value, "( TL") {
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(value), "-") && (slugContains(slug, "is merkezi") || slugContains(slug, "arsasi")) {
		return true
	}
	blocked := []string{
		"degerleme raporu", "bu belge", "serhi olmayan", "portfoye dahil",
		"gayrimenkul yatirim ortakligi", "anonim sirketi", "kap yilsonu",
		"talebiniz dogrultusunda", "rapora konu", "koordinat table text",
		"kdv haric", "kdv dahil", "konumlu tesisi",
		"asansorlu", "havalandirma", "dogalgaz", "isitmali", "isıtmalı", "chiller",
		"klima", "brut", "tek blok halinde", "kat halinde", "kapasiteli",
		"maslak arsasi", "ankara cankaya is merkezi", "istanbul karakoy is merkezi",
		"istanbul sishane is merkezi",
	}
	for _, item := range blocked {
		if slugContains(slug, item) {
			return true
		}
	}
	return strings.Contains(value, "********")
}

func betterInventoryLocation(current, candidate string) bool {
	currentLen := len([]rune(current))
	candidateLen := len([]rune(candidate))
	if candidateLen == 0 {
		return false
	}
	if currentLen == 0 {
		return true
	}
	if candidateLen < currentLen/2 && (strings.Contains(candidate, "/") || cityInText(candidate) != "") {
		return true
	}
	if cityInText(current) == "" && cityInText(candidate) != "" {
		return true
	}
	return false
}

func betterInventoryParcel(assetName, current, candidate string) bool {
	currentSlug := util.SlugTR(current)
	candidateSlug := util.SlugTR(candidate)
	if currentSlug == "" {
		return candidateSlug != ""
	}
	if candidateSlug == "" || currentSlug == candidateSlug {
		return false
	}
	if preferred := preferredInventoryParcelSlug(assetName); preferred != "" {
		currentPreferred := strings.Contains(currentSlug, preferred)
		candidatePreferred := strings.Contains(candidateSlug, preferred)
		if currentPreferred != candidatePreferred {
			return candidatePreferred
		}
	}
	currentScore := inventoryParcelQualityScore(current)
	candidateScore := inventoryParcelQualityScore(candidate)
	if candidateScore != currentScore {
		return candidateScore > currentScore
	}
	return len([]rune(candidate)) > len([]rune(current)) && !strings.Contains(candidateSlug, currentSlug)
}

func preferredInventoryParcelSlug(assetName string) string {
	slug := util.SlugTR(assetName)
	switch {
	case slugContains(slug, "bodrum hillside") || slugContains(slug, "bodrum otel"):
		return util.SlugTR("363 ada")
	case slugContains(slug, "topcular"):
		return util.SlugTR("247 ada")
	case slugContains(slug, "etiler alkent"):
		return util.SlugTR("1411 ada")
	case slugContains(slug, "buyukcekmece eskice"):
		return util.SlugTR("106 ada")
	default:
		return ""
	}
}

func inventoryParcelQualityScore(value string) int {
	slug := util.SlugTR(value)
	score := 0
	if slugContains(slug, "ada") {
		score += 2
	}
	if slugContains(slug, "parsel") {
		score += 2
	}
	if strings.Contains(slug, " ve ") || strings.Contains(slug, ",") || strings.Contains(slug, "parseller") {
		score++
	}
	if strings.Contains(slug, "000 ada") {
		score -= 3
	}
	if strings.Contains(slug, "no lu") || strings.Contains(slug, "nolu") {
		score++
	}
	return score
}

func preferredInventoryAssetType(name, current, candidate string) string {
	current = strings.TrimSpace(current)
	candidate = strings.TrimSpace(candidate)
	if current == "" || current == "other" {
		return candidate
	}
	if candidate == "" || candidate == "other" || candidate == current {
		return current
	}
	slug := util.SlugTR(name)
	switch {
	case slugContains(slug, "fabrika") && candidate == "factory":
		return candidate
	case slugContains(slug, "arsa") && candidate == "land":
		return candidate
	case (slugContains(slug, "otel") || slugContains(slug, "beach club")) && candidate == "hotel":
		return candidate
	case slugContains(slug, "hillside beach club") && candidate == "usage_right":
		return candidate
	case current == "project" && candidate != "project":
		return candidate
	case current == "office" && slugContains(slug, "arsa") && candidate == "land":
		return candidate
	}
	return current
}

func historyPointFromEvent(event AssetEvent) AssetHistoryPoint {
	period := ""
	if event.Period != nil {
		period = *event.Period
	}
	expertiseDate := ""
	if event.ExpertiseDate != nil {
		expertiseDate = *event.ExpertiseDate
	}
	return AssetHistoryPoint{
		Period:                   period,
		ExpertiseDate:            expertiseDate,
		ExpertiseValueExclVATTRY: event.ExpertiseValueExclVATTRY,
		ExpertiseValueInclVATTRY: event.ExpertiseValueInclVATTRY,
		BookValueTRY:             event.BookValueTRY,
		MonthlyRentTRY:           event.RentalInfo.MonthlyRentTRY,
		AnnualRentTRY:            event.RentalInfo.AnnualRentTRY,
		AnnualMinRentUSD:         event.RentalInfo.AnnualMinRentUSD,
		SourceFile:               event.SourceFile,
	}
}

func findMatchingInventoryAsset(index map[string][]int, items []AssetInventoryItem, event AssetEvent) int {
	candidateSet := map[int]bool{}
	for _, key := range inventoryKeysFromEvent(event) {
		for _, idx := range index[key] {
			candidateSet[idx] = true
		}
	}
	candidates := make([]int, 0, len(candidateSet))
	for idx := range candidateSet {
		candidates = append(candidates, idx)
	}
	if len(candidates) == 0 && len(items) <= 300 {
		for idx := range items {
			candidates = append(candidates, idx)
		}
	}
	bestIdx := -1
	bestScore := 0.0
	for _, i := range candidates {
		item := items[i]
		score := assetMatchScore(item, event)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestScore >= 0.55 {
		return bestIdx
	}
	return -1
}

func addInventoryIndexKeys(index map[string][]int, item AssetInventoryItem, idx int) {
	for _, key := range inventoryKeysFromItem(item) {
		if key == "" {
			continue
		}
		exists := false
		for _, existing := range index[key] {
			if existing == idx {
				exists = true
				break
			}
		}
		if !exists {
			index[key] = append(index[key], idx)
		}
	}
}

func inventoryKeysFromEvent(event AssetEvent) []string {
	item := AssetInventoryItem{
		AssetName:  event.AssetName,
		AssetType:  event.AssetType,
		Location:   event.Location,
		City:       event.City,
		District:   event.District,
		AreaM2:     event.AreaM2,
		ParcelInfo: event.ParcelInfo,
	}
	return inventoryKeysFromItem(item)
}

func inventoryKeysFromItem(item AssetInventoryItem) []string {
	keys := []string{}
	if item.ParcelInfo != "" {
		keys = append(keys, "parcel:"+util.SlugTR(item.ParcelInfo))
	}
	name := canonicalAssetNameKey(item.AssetName)
	if name != "" {
		keys = append(keys, "name:"+name)
		if item.AssetType != "" {
			keys = append(keys, "name_type:"+item.AssetType+"|"+name)
		}
	}
	if item.City != "" && item.AreaM2 != nil && item.AssetType != "" {
		keys = append(keys, fmt.Sprintf("city_area:%s|%s|%.0f", item.AssetType, util.SlugTR(item.City), roundedArea(*item.AreaM2)))
	}
	if item.Location != "" && item.AssetType != "" {
		loc := util.SlugTR(item.Location)
		if len(loc) > 48 {
			loc = loc[:48]
		}
		keys = append(keys, "loc:"+item.AssetType+"|"+loc)
	}
	return dedupeStrings(keys)
}

func canonicalAssetNameKey(value string) string {
	slug := util.SlugTR(value)
	if slug == "" {
		return ""
	}
	if len(slug) > 80 {
		slug = slug[:80]
	}
	return slug
}

func roundedArea(value float64) float64 {
	if value <= 0 {
		return 0
	}
	switch {
	case value >= 10000:
		return float64(int(value/100+0.5) * 100)
	case value >= 1000:
		return float64(int(value/10+0.5) * 10)
	default:
		return float64(int(value + 0.5))
	}
}

func assetMatchScore(item AssetInventoryItem, event AssetEvent) float64 {
	score := 0.0
	nameA := util.SlugTR(item.AssetName)
	nameB := util.SlugTR(event.AssetName)
	parcelA := util.SlugTR(item.ParcelInfo)
	parcelB := util.SlugTR(event.ParcelInfo)
	sameValuationCode := assetValuationCodeRE.MatchString(strings.TrimSpace(item.AssetName)) &&
		assetValuationCodeRE.MatchString(strings.TrimSpace(event.AssetName)) &&
		nameA != "" && nameA == nameB
	sameStrongNamedAsset := nameA != "" && nameA == nameB &&
		inventoryAssetNameHasStrongIdentity(item.AssetName) &&
		!assetUnitCodeRE.MatchString(strings.TrimSpace(item.AssetName)) &&
		!assetUnitCodeRE.MatchString(strings.TrimSpace(event.AssetName))
	if parcelA != "" && parcelB != "" && parcelA != parcelB && !sameValuationCode && !sameStrongNamedAsset {
		return 0
	}
	if nameA != "" && nameA == nameB && assetUnitCodeRE.MatchString(strings.TrimSpace(item.AssetName)) && assetUnitCodeRE.MatchString(strings.TrimSpace(event.AssetName)) {
		if item.AreaM2 != nil && event.AreaM2 != nil && !closeFloat(*item.AreaM2, *event.AreaM2, 0.03) {
			return 0
		}
	}
	if nameA != "" && nameB != "" {
		switch {
		case nameA == nameB:
			score += 0.35
		case strings.Contains(nameA, nameB) || strings.Contains(nameB, nameA):
			score += 0.25
		default:
			score += 0.20 * tokenOverlap(nameA, nameB)
		}
	}
	if item.AssetType != "" && item.AssetType == event.AssetType {
		score += 0.10
	}
	if nameA != "" && nameA == nameB {
		score += 0.25
	}
	if nameA != "" && nameA == nameB && item.AssetType != "" && item.AssetType == event.AssetType {
		score += 0.15
	}
	if parcelA != "" && parcelA == parcelB {
		score += 0.30
	}
	if item.City != "" && event.City != "" && strings.EqualFold(item.City, event.City) {
		score += 0.08
	}
	if item.District != "" && event.District != "" && strings.EqualFold(item.District, event.District) {
		score += 0.08
	}
	if item.Location != "" && event.Location != "" {
		score += 0.12 * tokenOverlap(util.SlugTR(item.Location), util.SlugTR(event.Location))
	}
	if item.AreaM2 != nil && event.AreaM2 != nil && closeFloat(*item.AreaM2, *event.AreaM2, 0.03) {
		score += 0.12
	}
	if item.ParcelInfo == "" && event.ParcelInfo == "" && nameA == "" {
		score *= 0.5
	}
	return score
}

func buildGYOSummary(ticker string, assets []AssetInventoryItem) GYOAssetSummary {
	summary := GYOAssetSummary{Ticker: ticker, Warnings: []string{}}
	largest := []AssetSummaryItem{}
	rental := []AssetSummaryItem{}
	fxRental := []AssetSummaryItem{}
	projects := []AssetSummaryItem{}
	for _, asset := range assets {
		if !isGYOSummaryEligibleAsset(asset) {
			continue
		}
		value := latestAssetValue(asset)
		item := AssetSummaryItem{AssetName: asset.AssetName, AssetType: asset.AssetType, Location: asset.Location}
		if value > 0 {
			item.ValueTRY = floatPtr(value)
			largest = append(largest, item)
		}
		if asset.RentalInfo.IsRented != nil && *asset.RentalInfo.IsRented || asset.RentalInfo.MonthlyRentTRY != nil || asset.RentalInfo.AnnualRentTRY != nil {
			summary.TotalRentalAssets++
			rentItem := item
			if asset.RentalInfo.MonthlyRentTRY != nil {
				rentItem.RentTRY = asset.RentalInfo.MonthlyRentTRY
			} else if asset.RentalInfo.AnnualRentTRY != nil {
				rentItem.RentTRY = asset.RentalInfo.AnnualRentTRY
			}
			rental = append(rental, rentItem)
		}
		if asset.RentalInfo.AnnualMinRentUSD != nil {
			fxItem := item
			fxItem.RentUSD = asset.RentalInfo.AnnualMinRentUSD
			fxRental = append(fxRental, fxItem)
		}
		if asset.AssetType == "project" || asset.Status == "under_construction" {
			summary.TotalProjects++
			projects = append(projects, item)
		}
	}
	sortSummaryItemsByValue(largest)
	sortSummaryItemsByValue(rental)
	sortSummaryItemsByValue(fxRental)
	sortSummaryItemsByValue(projects)
	summary.LargestAssets = limitSummaryItems(largest, 10)
	summary.RentalIncomeAssets = limitSummaryItems(rental, 10)
	summary.FXLinkedRentalAssets = limitSummaryItems(fxRental, 10)
	summary.UnderConstructionProjects = limitSummaryItems(projects, 10)
	summary.Warnings = append(summary.Warnings, "real_estate_total_value_requires_portfolio_summary")
	return summary
}

func isGYOSummaryEligibleAsset(asset AssetInventoryItem) bool {
	name := strings.TrimSpace(asset.AssetName)
	if name == "" || asset.Confidence < 0.60 || asset.AssetType == "" || asset.AssetType == "other" {
		return false
	}
	if assetNameLooksGeneric(name) || !assetNameHasPortfolioIdentity(name) {
		return false
	}
	hasAnchor := asset.Location != "" || asset.City != "" || asset.District != "" || asset.ParcelInfo != "" || asset.AreaM2 != nil
	hasRent := asset.RentalInfo.IsRented != nil || asset.RentalInfo.MonthlyRentTRY != nil || asset.RentalInfo.AnnualRentTRY != nil || asset.RentalInfo.AnnualMinRentUSD != nil
	return hasAnchor || hasRent || latestAssetValue(asset) > 0
}

func latestExclVAT(asset AssetInventoryItem) *float64 {
	for i := len(asset.History) - 1; i >= 0; i-- {
		if asset.History[i].ExpertiseValueExclVATTRY != nil {
			return asset.History[i].ExpertiseValueExclVATTRY
		}
	}
	return nil
}

func latestInclVAT(asset AssetInventoryItem) *float64 {
	for i := len(asset.History) - 1; i >= 0; i-- {
		if asset.History[i].ExpertiseValueInclVATTRY != nil {
			return asset.History[i].ExpertiseValueInclVATTRY
		}
	}
	return nil
}

func latestAssetValue(asset AssetInventoryItem) float64 {
	for i := len(asset.History) - 1; i >= 0; i-- {
		h := asset.History[i]
		switch {
		case h.ExpertiseValueExclVATTRY != nil:
			return *h.ExpertiseValueExclVATTRY
		case h.ExpertiseValueInclVATTRY != nil:
			return *h.ExpertiseValueInclVATTRY
		case h.BookValueTRY != nil:
			return *h.BookValueTRY
		}
	}
	return 0
}

func isRealEstateAsset(assetType string) bool {
	switch assetType {
	case "land", "hotel", "office", "shop", "factory", "project", "usage_right", "other":
		return true
	default:
		return false
	}
}

func writeAssetEventsJSONL(path string, events []AssetEvent) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func writeAssetInventoryJSON(path string, inventory AssetInventory) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(inventory)
}

func sortedAssetTickers(values map[string][]AssetEvent) []string {
	tickers := make([]string, 0, len(values))
	for ticker := range values {
		tickers = append(tickers, ticker)
	}
	sort.Strings(tickers)
	return tickers
}

func mergeRentalInfo(a, b AssetRentalInfo) AssetRentalInfo {
	if a.IsRented == nil {
		a.IsRented = b.IsRented
	} else if b.IsRented != nil && *b.IsRented {
		a.IsRented = b.IsRented
	}
	if a.MonthlyRentTRY == nil {
		a.MonthlyRentTRY = b.MonthlyRentTRY
	}
	if a.AnnualRentTRY == nil {
		a.AnnualRentTRY = b.AnnualRentTRY
	}
	if a.AnnualMinRentUSD == nil {
		a.AnnualMinRentUSD = b.AnnualMinRentUSD
	}
	if a.VariableRentTerms == "" {
		a.VariableRentTerms = b.VariableRentTerms
	}
	if a.Tenant == "" {
		a.Tenant = b.Tenant
	}
	return a
}

func appendAssetReferences(existing, incoming []AssetSourceReference, limit int) []AssetSourceReference {
	out := append([]AssetSourceReference{}, existing...)
	seen := map[string]bool{}
	for _, ref := range out {
		seen[ref.Snippet] = true
	}
	for _, ref := range incoming {
		if ref.Snippet == "" || seen[ref.Snippet] {
			continue
		}
		out = append(out, ref)
		seen[ref.Snippet] = true
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func historyExists(history []AssetHistoryPoint, candidate AssetHistoryPoint) bool {
	key := historyKey(candidate)
	for _, item := range history {
		if historyKey(item) == key {
			return true
		}
	}
	return false
}

func historyKey(item AssetHistoryPoint) string {
	return fmt.Sprintf("%s|%s|%s|%.2f|%.2f|%.2f",
		item.Period,
		item.ExpertiseDate,
		item.SourceFile,
		ptrValue(item.ExpertiseValueExclVATTRY),
		ptrValue(item.ExpertiseValueInclVATTRY),
		ptrValue(item.MonthlyRentTRY),
	)
}

func ptrValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func tokenOverlap(a, b string) float64 {
	aTokens := strings.Fields(a)
	bTokens := strings.Fields(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, token := range aTokens {
		if len(token) >= 3 {
			seen[token] = true
		}
	}
	if len(seen) == 0 {
		return 0
	}
	matches := 0
	for _, token := range bTokens {
		if seen[token] {
			matches++
		}
	}
	return float64(matches) / float64(len(seen))
}

func closeFloat(a, b, tolerance float64) bool {
	if a == 0 || b == 0 {
		return false
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	base := a
	if b > base {
		base = b
	}
	return diff/base <= tolerance
}

func sortSummaryItemsByValue(items []AssetSummaryItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return ptrValue(items[i].ValueTRY)+ptrValue(items[i].RentTRY)+ptrValue(items[i].RentUSD) >
			ptrValue(items[j].ValueTRY)+ptrValue(items[j].RentTRY)+ptrValue(items[j].RentUSD)
	})
}

func limitSummaryItems(items []AssetSummaryItem, limit int) []AssetSummaryItem {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
