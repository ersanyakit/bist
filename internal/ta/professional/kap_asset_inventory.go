package professional

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"hissebot/internal/kapingest"
	tamacro "hissebot/internal/ta/macro"
	"hissebot/internal/util"
)

var (
	kapAssetDisplayUnitCodeRE      = regexp.MustCompile(`(?i)^[A-Z]{1,4}\d{0,4}[-/]\d{1,4}[A-Z]?$`)
	kapAssetDisplayValuationCodeRE = regexp.MustCompile(`(?i)^[A-Z]{2,6}-\d{6,}(?:\s+.+)?$`)
)

type KAPAssetInventorySummary struct {
	Computed          bool                      `json:"computed"`
	Symbol            string                    `json:"symbol"`
	InventoryPath     string                    `json:"inventory_path,omitempty"`
	EventCount        int                       `json:"event_count"`
	RawAssetCount     int                       `json:"raw_asset_count"`
	DisplayAssetCount int                       `json:"display_asset_count"`
	PortfolioSummary  KAPAssetPortfolioSummary  `json:"portfolio_summary"`
	ValueIndex        KAPAssetValueIndexSummary `json:"value_index,omitempty"`
	TotalRentalAssets int                       `json:"total_rental_assets"`
	TotalProjects     int                       `json:"total_projects"`
	Assets            []KAPAssetInventoryItem   `json:"assets,omitempty"`
	Summary           string                    `json:"summary"`
	Warnings          []string                  `json:"warnings,omitempty"`
}

type KAPAssetPortfolioSummary struct {
	TotalRealEstateValueExclVATTRY *float64 `json:"total_real_estate_value_excl_vat_try,omitempty"`
	TotalRealEstateValueInclVATTRY *float64 `json:"total_real_estate_value_incl_vat_try,omitempty"`
	TotalBookValueTRY              *float64 `json:"total_book_value_try,omitempty"`
	HistoryCount                   int      `json:"history_count"`
}

type KAPAssetValueIndexSummary struct {
	Computed            bool   `json:"computed"`
	SeriesID            string `json:"series_id,omitempty"`
	SeriesLabel         string `json:"series_label,omitempty"`
	Source              string `json:"source,omitempty"`
	SourceURL           string `json:"source_url,omitempty"`
	LatestPeriod        string `json:"latest_period,omitempty"`
	IndexedAssetCount   int    `json:"indexed_asset_count,omitempty"`
	IndexableAssetCount int    `json:"indexable_asset_count,omitempty"`
	DataQualityWarning  string `json:"data_quality_warning,omitempty"`
}

type KAPAssetInventoryItem struct {
	AssetName              string   `json:"asset_name"`
	AssetType              string   `json:"asset_type"`
	Location               string   `json:"location,omitempty"`
	City                   string   `json:"city,omitempty"`
	District               string   `json:"district,omitempty"`
	AreaM2                 *float64 `json:"area_m2,omitempty"`
	ParcelInfo             string   `json:"parcel_info,omitempty"`
	LatestPeriod           string   `json:"latest_period,omitempty"`
	ExpertiseDate          string   `json:"expertise_date,omitempty"`
	LatestValueTRY         *float64 `json:"latest_value_try,omitempty"`
	LatestExpertiseExclVAT *float64 `json:"latest_expertise_value_excl_vat_try,omitempty"`
	LatestExpertiseInclVAT *float64 `json:"latest_expertise_value_incl_vat_try,omitempty"`
	LatestBookValueTRY     *float64 `json:"latest_book_value_try,omitempty"`
	ValueSource            string   `json:"value_source,omitempty"`
	IndexedValueTRY        *float64 `json:"indexed_value_try,omitempty"`
	IndexedValueAsOf       string   `json:"indexed_value_as_of,omitempty"`
	IndexedValueBasePeriod string   `json:"indexed_value_base_period,omitempty"`
	IndexedValueFactor     float64  `json:"indexed_value_factor,omitempty"`
	IndexedValueSource     string   `json:"indexed_value_source,omitempty"`
	IndexedValueWarning    string   `json:"indexed_value_warning,omitempty"`
	MonthlyRentTRY         *float64 `json:"monthly_rent_try,omitempty"`
	AnnualRentTRY          *float64 `json:"annual_rent_try,omitempty"`
	AnnualRentUSD          *float64 `json:"annual_rent_usd,omitempty"`
	HistoryCount           int      `json:"history_count"`
	Confidence             float64  `json:"confidence"`
	SourceFile             string   `json:"source_file,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

func analyzeKAPAssetInventory(equitiesDir, symbol string) KAPAssetInventorySummary {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := KAPAssetInventorySummary{Symbol: symbol}
	if symbol == "" {
		out.Summary = "KAP varlik envanteri icin sembol bos."
		out.Warnings = []string{"kap_asset_inventory_symbol_missing"}
		return out
	}
	path := findKAPAssetInventoryPath(equitiesDir, symbol)
	if path == "" {
		out.Summary = fmt.Sprintf("KAP varlik envanteri bulunamadi; %s icin asset_inventory.json yok.", symbol)
		out.Warnings = []string{"kap_asset_inventory_missing"}
		return out
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		out.Summary = fmt.Sprintf("KAP varlik envanteri okunamadi: %s.", path)
		out.Warnings = []string{"kap_asset_inventory_read_failed"}
		return out
	}
	var inventory kapingest.AssetInventory
	if err := json.Unmarshal(raw, &inventory); err != nil {
		out.Summary = fmt.Sprintf("KAP varlik envanteri JSON parse edilemedi: %s.", path)
		out.Warnings = []string{"kap_asset_inventory_parse_failed"}
		return out
	}
	out.Computed = true
	out.InventoryPath = path
	out.EventCount = inventory.EventCount
	out.RawAssetCount = inventory.AssetCount
	out.PortfolioSummary = KAPAssetPortfolioSummary{
		TotalRealEstateValueExclVATTRY: inventory.PortfolioSummary.TotalRealEstateValueExclVATTRY,
		TotalRealEstateValueInclVATTRY: inventory.PortfolioSummary.TotalRealEstateValueInclVATTRY,
		TotalBookValueTRY:              inventory.PortfolioSummary.TotalBookValueTRY,
		HistoryCount:                   len(inventory.PortfolioSummary.History),
	}
	inflationDataset, inflationSummary, inflationWarning := loadKAPAssetInflationIndex(equitiesDir)
	out.ValueIndex = inflationSummary
	out.Assets = displayableKAPAssets(symbol, inventory.Assets, inflationDataset)
	out.DisplayAssetCount = len(out.Assets)
	for _, asset := range out.Assets {
		if asset.MonthlyRentTRY != nil || asset.AnnualRentTRY != nil || asset.AnnualRentUSD != nil {
			out.TotalRentalAssets++
		}
		if asset.AssetType == "project" {
			out.TotalProjects++
		}
		if asset.LatestValueTRY != nil {
			out.ValueIndex.IndexableAssetCount++
		}
		if asset.IndexedValueTRY != nil {
			out.ValueIndex.IndexedAssetCount++
		}
		out.Warnings = append(out.Warnings, asset.Warnings...)
	}
	out.Warnings = append(out.Warnings, inventory.Warnings...)
	if countForeignKAPValuationRows(symbol, inventory.Assets) > 0 {
		out.Warnings = append(out.Warnings, "asset_inventory_cross_ticker_valuation_rows_filtered")
	}
	if inflationWarning != "" && out.ValueIndex.IndexableAssetCount > 0 {
		out.Warnings = append(out.Warnings, inflationWarning)
	}
	if out.PortfolioSummary.TotalRealEstateValueExclVATTRY == nil && out.PortfolioSummary.TotalRealEstateValueInclVATTRY == nil {
		out.Warnings = append(out.Warnings, "asset_inventory_portfolio_vat_totals_missing")
	}
	if out.DisplayAssetCount < out.RawAssetCount {
		out.Warnings = append(out.Warnings, "asset_inventory_strict_display_filter_applied")
	}
	if out.EventCount > out.DisplayAssetCount && out.DisplayAssetCount > 0 {
		out.Warnings = append(out.Warnings, "asset_inventory_full_json_contains_more_events_than_html_summary")
	}
	if out.DisplayAssetCount == 0 {
		out.Warnings = append(out.Warnings, "asset_inventory_display_assets_empty")
	}
	out.Warnings = uniqueStrings(out.Warnings)
	out.Summary = kapAssetInventorySummaryText(out)
	return out
}

func findKAPAssetInventoryPath(equitiesDir, symbol string) string {
	for _, path := range kapAssetInventoryCandidatePaths(equitiesDir, symbol) {
		if fileExists(path) {
			return path
		}
	}
	return ""
}

func kapAssetInventoryCandidatePaths(equitiesDir, symbol string) []string {
	symbolUpper := strings.ToUpper(strings.TrimSpace(symbol))
	symbolLower := strings.ToLower(symbolUpper)
	paths := []string{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || path == "." {
			return
		}
		for _, existing := range paths {
			if existing == path {
				return
			}
		}
		paths = append(paths, path)
	}
	if equitiesDir != "" {
		dataDir := filepath.Dir(filepath.Clean(equitiesDir))
		add(filepath.Join(dataDir, "processed", "by_ticker", symbolUpper, kapingest.AssetInventoryFile))
		add(filepath.Join(dataDir, "processed", "by_ticker", symbolLower, kapingest.AssetInventoryFile))
		add(filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolUpper, kapingest.AssetInventoryFile))
		add(filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolLower, kapingest.AssetInventoryFile))
		add(filepath.Join(dataDir, "processed", symbolLower, kapingest.AssetInventoryFile))
	}
	add(filepath.Join("data", "processed", "by_ticker", symbolUpper, kapingest.AssetInventoryFile))
	add(filepath.Join("data", "processed", "by_ticker", symbolLower, kapingest.AssetInventoryFile))
	add(filepath.Join("data", "processed", symbolLower, "by_ticker", symbolUpper, kapingest.AssetInventoryFile))
	add(filepath.Join("data", "processed", symbolLower, "by_ticker", symbolLower, kapingest.AssetInventoryFile))
	add(filepath.Join("data", "processed", symbolLower, kapingest.AssetInventoryFile))
	return paths
}

func displayableKAPAssets(symbol string, items []kapingest.AssetInventoryItem, inflationDataset tamacro.InflationDataset) []KAPAssetInventoryItem {
	byKey := map[string]KAPAssetInventoryItem{}
	for _, item := range items {
		if !isDisplayableKAPAsset(symbol, item) {
			continue
		}
		summary := kapAssetInventoryItemSummary(item, inflationDataset)
		key := kapAssetInventoryItemKey(summary)
		if key == "" {
			continue
		}
		if existing, ok := byKey[key]; ok {
			byKey[key] = betterKAPAssetSummary(existing, summary)
			continue
		}
		byKey[key] = summary
	}
	out := make([]KAPAssetInventoryItem, 0, len(byKey))
	for _, item := range byKey {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		vi := kapAssetSortValue(out[i])
		vj := kapAssetSortValue(out[j])
		if vi != vj {
			return vi > vj
		}
		if out[i].LatestPeriod != out[j].LatestPeriod {
			return out[i].LatestPeriod > out[j].LatestPeriod
		}
		if out[i].ExpertiseDate != out[j].ExpertiseDate {
			return out[i].ExpertiseDate > out[j].ExpertiseDate
		}
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].AssetName < out[j].AssetName
	})
	return out
}

func isDisplayableKAPAsset(symbol string, item kapingest.AssetInventoryItem) bool {
	name := strings.TrimSpace(item.AssetName)
	if name == "" || item.Confidence < 0.60 || item.AssetType == "" || item.AssetType == "other" {
		return false
	}
	if kapAssetForeignValuationCode(symbol, name) {
		return false
	}
	if item.AssetType == "subsidiary" || item.AssetType == "financial_asset" {
		return false
	}
	if item.OwnershipType == "subsidiary" && !kapAssetNameHasDisplayIdentity(name) {
		return false
	}
	if kapAssetNameLooksUnsafe(name) {
		return false
	}
	latest := latestKAPAssetHistory(item)
	hasValue := latest.ExpertiseValueExclVATTRY != nil || latest.ExpertiseValueInclVATTRY != nil || latest.BookValueTRY != nil
	hasPhysicalAnchor := item.Location != "" || item.City != "" || item.District != "" || item.ParcelInfo != "" || item.AreaM2 != nil
	hasEvidence := hasPhysicalAnchor || hasValue || latest.MonthlyRentTRY != nil || latest.AnnualRentTRY != nil || latest.AnnualMinRentUSD != nil
	if !hasEvidence {
		return false
	}
	if kapAssetNameHasDisplayIdentity(name) {
		return true
	}
	return kapAssetNameLooksConcise(name) && hasPhysicalAnchor && hasValue && item.Confidence >= 0.80
}

func kapAssetNameLooksUnsafe(name string) bool {
	trimmed := strings.TrimSpace(name)
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "'") || strings.HasPrefix(trimmed, "\"") ||
		strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "•") || strings.Contains(trimmed, "=") ||
		strings.Contains(trimmed, "%") || strings.Contains(trimmed, ",") {
		return true
	}
	if len([]rune(trimmed)) > 90 && !kapAssetNameHasDisplayIdentity(trimmed) {
		return true
	}
	slug := util.SlugTR(trimmed)
	exactBlocked := []string{
		"arsa", "arazi", "otel", "fabrika", "dukkan", "magaza", "gayrimenkul", "proje",
		"arsa alani", "arsa yuz olcumu", "alani", "vasfi", "birim degeri", "satilik",
		"davasinda", "bulundurularak", "ust hakki odemesi", "ust hakki uzatimi odemesi",
	}
	for _, item := range exactBlocked {
		if slug == util.SlugTR(item) {
			return true
		}
	}
	blocked := []string{
		"toplam", "kdv haric", "kdv dahil", "degerleme yontemi", "gider", "gelir",
		"maliyet", "risk maliyeti", "arsa degeri", "bina insa", "dolarini asarsa",
		"beyan edilen", "aittir", "ettigi", "kisi ile", "nufusa sahip", "talebiniz",
		"karsisinda", "pazar degeri", "proje riski", "finansman maliyeti",
		"net defter degeri", "maddi duran varlik", "gelistirilmis arsa", "bugunku ortalama",
		"arsa birim degeri", "emlak vergisi", "rayic degeri", "degerlenen parsel", "vasfi",
		"degerlenen parseller", "mevcutta yapi bulunmamakta", "yapi bulunmamakta",
		"hukuki ve yasal prosedur", "prosedurlerini tamamladigi", "varsayilmistir",
		"varilmistir", "kamulastirma", "mahkemece", "mimari proje", "onayli mimari",
		"tapu mudurlugu", "ilce belediyesi", "incelenmistir", "proje gelistirme hesaplari",
		"konutun net alani", "ticari arsa degerleri", "belediyeden alinan",
		"satis bedeli istenilmektedir", "benzer ozelliklere sahip", "edilebilmesidir",
		"yakin konumda", "emsal teskil", "is bu rapora konu", "rapora konu olan tasinmaz",
		"degerlemesi yapilan tasinmaz", "nitelikli ana gayrimenkul", "no lu gayrimenkullerdir",
		"bulunup bulunmadigi", "hangi amacla kullanildigi", "butun parseller", "mevcut agac",
		"uygulama yapilacak", "arsa paylarinin", "villalarin arsa paylari",
		"kira satis bedellerine", "beyanlar temin", "arsaya", "dukkan icin", "telefon", "karara baglamak",
		"lejant", "imar plani", "imarli", "emsal", "parselin birim", "birim satis",
		"arsa satislari", "bolgede yapilan arastirmalara", "yorumlanarak", "ozel spor alani",
		"egitim tesis alani", "paftasindadir", "sıniri", "siniri", "irtifak",
		"genel kurul", "vekaletname", "sirketimiz", "adına kayıtlı", "adina kayitli",
		"musterinin talebi", "müşterinin talebi", "bu rapor", "portfoyune", "portföyüne",
		"gayrimenkuller sunlardir", "gayrimenkuller şunlardır", "kullanim alanli",
		"kullanım alanlı", "yakın konumlu", "yakin konumlu", "pazarlik payi",
		"pazarlık payı", "satilik", "satılık", "satis bedeli", "satış bedeli",
		"yapilasma kosulu", "yapılaşma koşulu", "belirtilen", "belirtilmistir",
		"belirtilmiştir", "karsilastirilabilir", "karşılaştırılabilir", "varsayim",
		"varsayım", "ruhsati", "ruhsatı", "izin belgesi", "hissesine dusen",
		"hissesine düşen", "tasınmaz", "tasinmaz", "normal kat", "bodrum kat",
		"zemin kat", "civarinda", "civarında", "minimum parsel", "duvarlar",
		"görülmüştür", "gorulmustur", "telefon",
		"kira", "aylik", "aylık", "birim degeri", "birim değeri",
		"portfoyumuzdeki", "portföyümüzdeki", "arsalar ve araziler",
		"arazi teslimlerinde", "gayrimenkule dayali", "gayrimenkule dayalı",
		"ndeki", "üzerine", "uzerine", "koyumuz", "köyümüz", "yeniden",
		"odali", "odalı", "katlar", "alinmistir", "alınmıştır",
	}
	for _, item := range blocked {
		if strings.Contains(slug, util.SlugTR(item)) {
			return true
		}
	}
	return false
}

func kapAssetNameHasDisplayIdentity(name string) bool {
	trimmed := strings.TrimSpace(name)
	if kapAssetDisplayUnitCodeRE.MatchString(trimmed) || kapAssetDisplayValuationCodeRE.MatchString(trimmed) {
		return true
	}
	if len([]rune(trimmed)) > 70 {
		return false
	}
	first := []rune(trimmed)[0]
	if first >= 'a' && first <= 'z' {
		return false
	}
	slug := util.SlugTR(trimmed)
	identityTerms := []string{
		"hillside beach club", "alarko dim", "alarko is merkezi",
		"karakoy is merkezi", "sishane is merkezi", "cankaya is merkezi",
		"buyukcekmece alkent", "buyukcekmece eskice", "buyukcekmece arsalar",
		"maslak arsasi", "sariyer maslak", "bodrum hillside", "bodrum otel",
		"fethiye kalemya", "etiler alkent", "eyup topcular", "topcular fabrika",
		"fabrika binasi", "fabrika bina", "kargir iki evli arsa",
		"bahceli kargir fabrika", "mosalarko", "ojsc",
	}
	for _, term := range identityTerms {
		if strings.Contains(slug, util.SlugTR(term)) {
			return true
		}
	}
	return false
}

func kapAssetNameLooksConcise(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || len([]rune(trimmed)) > 55 {
		return false
	}
	if strings.ContainsAny(trimmed, ".,;:()") {
		return false
	}
	words := strings.Fields(trimmed)
	if len(words) == 0 || len(words) > 6 {
		return false
	}
	first := []rune(trimmed)[0]
	if first >= 'a' && first <= 'z' {
		return false
	}
	slug := util.SlugTR(trimmed)
	genericShort := []string{
		"no lu parsel", "arsadir", "arsa uzerinde", "otel tesisi", "ana otel binasinin",
		"konut arsasina", "ve arsasi", "arsa dahil degeri", "olcumune sahip arsa",
	}
	for _, item := range genericShort {
		if strings.Contains(slug, util.SlugTR(item)) {
			return false
		}
	}
	return true
}

func kapAssetInventoryItemSummary(item kapingest.AssetInventoryItem, inflationDataset tamacro.InflationDataset) KAPAssetInventoryItem {
	latest := latestKAPAssetHistory(item)
	value, source := latestKAPAssetValue(latest)
	if value == nil {
		value, source = latestKAPAssetValueFromHistory(item.History)
	}
	excl, incl, book := latestKAPAssetValuesByType(item.History)
	out := KAPAssetInventoryItem{
		AssetName:              item.AssetName,
		AssetType:              item.AssetType,
		Location:               cleanKAPAssetLocation(item.Location),
		City:                   item.City,
		District:               item.District,
		AreaM2:                 item.AreaM2,
		ParcelInfo:             item.ParcelInfo,
		LatestPeriod:           latest.Period,
		ExpertiseDate:          latest.ExpertiseDate,
		LatestValueTRY:         value,
		LatestExpertiseExclVAT: excl,
		LatestExpertiseInclVAT: incl,
		LatestBookValueTRY:     book,
		ValueSource:            source,
		MonthlyRentTRY:         latest.MonthlyRentTRY,
		AnnualRentTRY:          latest.AnnualRentTRY,
		AnnualRentUSD:          latest.AnnualMinRentUSD,
		HistoryCount:           len(item.History),
		Confidence:             item.Confidence,
		SourceFile:             latest.SourceFile,
	}
	applyKAPAssetInflationAdjustment(&out, inflationDataset)
	applyKAPAssetSanityChecks(&out)
	return out
}

func applyKAPAssetSanityChecks(asset *KAPAssetInventoryItem) {
	if asset == nil {
		return
	}
	if kapAssetValueScaleSuspicious(*asset) {
		asset.Warnings = append(asset.Warnings, "asset_value_scale_suspicious_low")
		if asset.Confidence > 0.55 {
			asset.Confidence = 0.55
		}
	}
	if kapAssetNameNeedsOwnershipReview(asset.AssetName) {
		asset.Warnings = append(asset.Warnings, "asset_valuation_report_code_requires_ownership_review")
		if asset.Confidence > 0.55 {
			asset.Confidence = 0.55
		}
	}
	asset.Warnings = uniqueStrings(asset.Warnings)
}

func kapAssetValueScaleSuspicious(asset KAPAssetInventoryItem) bool {
	if asset.LatestValueTRY == nil || *asset.LatestValueTRY <= 0 || *asset.LatestValueTRY >= 2_000_000 {
		return false
	}
	if asset.AreaM2 != nil && *asset.AreaM2 > 0 && *asset.AreaM2 < 20 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(asset.AssetType)) {
	case "office", "building", "factory", "hotel", "shop", "land", "project":
		return asset.Location != "" || asset.City != "" || asset.District != "" || asset.ParcelInfo != "" || asset.AreaM2 != nil
	default:
		return false
	}
}

func kapAssetNameNeedsOwnershipReview(name string) bool {
	return kapAssetDisplayValuationCodeRE.MatchString(strings.TrimSpace(name))
}

func countForeignKAPValuationRows(symbol string, items []kapingest.AssetInventoryItem) int {
	count := 0
	for _, item := range items {
		if kapAssetForeignValuationCode(symbol, item.AssetName) {
			count++
		}
	}
	return count
}

func kapAssetForeignValuationCode(symbol, name string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	name = strings.ToUpper(strings.TrimSpace(name))
	if symbol == "" || name == "" || !kapAssetDisplayValuationCodeRE.MatchString(name) {
		return false
	}
	prefix := name
	if idx := strings.Index(prefix, "-"); idx >= 0 {
		prefix = prefix[:idx]
	}
	return prefix != "" && prefix != symbol
}

func latestKAPAssetHistory(item kapingest.AssetInventoryItem) kapingest.AssetHistoryPoint {
	if len(item.History) == 0 {
		return kapingest.AssetHistoryPoint{}
	}
	latest := item.History[0]
	for _, candidate := range item.History[1:] {
		if candidate.Period > latest.Period || (candidate.Period == latest.Period && candidate.ExpertiseDate > latest.ExpertiseDate) {
			latest = candidate
		}
	}
	return latest
}

func latestKAPAssetValue(history kapingest.AssetHistoryPoint) (*float64, string) {
	switch {
	case history.ExpertiseValueExclVATTRY != nil:
		return history.ExpertiseValueExclVATTRY, "expertise_excl_vat_try"
	case history.ExpertiseValueInclVATTRY != nil:
		return history.ExpertiseValueInclVATTRY, "expertise_incl_vat_try"
	case history.BookValueTRY != nil:
		return history.BookValueTRY, "book_value_try"
	default:
		return nil, ""
	}
}

func latestKAPAssetValueFromHistory(history []kapingest.AssetHistoryPoint) (*float64, string) {
	var best kapingest.AssetHistoryPoint
	found := false
	for _, candidate := range history {
		value, _ := latestKAPAssetValue(candidate)
		if value == nil {
			continue
		}
		if !found || candidate.Period > best.Period || (candidate.Period == best.Period && candidate.ExpertiseDate > best.ExpertiseDate) {
			best = candidate
			found = true
		}
	}
	if !found {
		return nil, ""
	}
	return latestKAPAssetValue(best)
}

func latestKAPAssetValuesByType(history []kapingest.AssetHistoryPoint) (*float64, *float64, *float64) {
	var exclPoint, inclPoint, bookPoint kapingest.AssetHistoryPoint
	var hasExcl, hasIncl, hasBook bool
	for _, candidate := range history {
		if candidate.ExpertiseValueExclVATTRY != nil && (!hasExcl || kapAssetHistoryAfter(candidate, exclPoint)) {
			exclPoint = candidate
			hasExcl = true
		}
		if candidate.ExpertiseValueInclVATTRY != nil && (!hasIncl || kapAssetHistoryAfter(candidate, inclPoint)) {
			inclPoint = candidate
			hasIncl = true
		}
		if candidate.BookValueTRY != nil && (!hasBook || kapAssetHistoryAfter(candidate, bookPoint)) {
			bookPoint = candidate
			hasBook = true
		}
	}
	var excl, incl, book *float64
	if hasExcl {
		excl = exclPoint.ExpertiseValueExclVATTRY
	}
	if hasIncl {
		incl = inclPoint.ExpertiseValueInclVATTRY
	}
	if hasBook {
		book = bookPoint.BookValueTRY
	}
	return excl, incl, book
}

func kapAssetHistoryAfter(a, b kapingest.AssetHistoryPoint) bool {
	if a.Period != b.Period {
		return a.Period > b.Period
	}
	if a.ExpertiseDate != b.ExpertiseDate {
		return a.ExpertiseDate > b.ExpertiseDate
	}
	return a.SourceFile > b.SourceFile
}

func kapAssetInventoryItemKey(item KAPAssetInventoryItem) string {
	name := util.SlugTR(item.AssetName)
	if name == "" {
		return ""
	}
	if len(name) > 90 {
		name = name[:90]
	}
	keyParts := []string{item.AssetType, name}
	if item.City != "" {
		keyParts = append(keyParts, util.SlugTR(item.City))
	}
	if item.ParcelInfo != "" {
		keyParts = append(keyParts, util.SlugTR(item.ParcelInfo))
	}
	return strings.Join(keyParts, "|")
}

func betterKAPAssetSummary(a, b KAPAssetInventoryItem) KAPAssetInventoryItem {
	av := kapAssetSortValue(a)
	bv := kapAssetSortValue(b)
	if bv > av {
		return b
	}
	if bv == av && b.HistoryCount > a.HistoryCount {
		return b
	}
	if bv == av && b.Confidence > a.Confidence {
		return b
	}
	return a
}

func loadKAPAssetInflationIndex(equitiesDir string) (tamacro.InflationDataset, KAPAssetValueIndexSummary, string) {
	path := tamacro.DefaultInflationPathFromEquitiesDir(equitiesDir)
	dataset, ok, err := tamacro.LoadInflationDataset(path)
	if err != nil {
		return tamacro.InflationDataset{}, KAPAssetValueIndexSummary{
			DataQualityWarning: "TÜİK enflasyon endeksi okunamadı: " + err.Error(),
		}, "asset_inventory_inflation_index_read_failed"
	}
	if !ok {
		return tamacro.InflationDataset{}, KAPAssetValueIndexSummary{
			DataQualityWarning: "TÜİK/Yİ-ÜFE aylık endeks dosyası bulunamadı; tarihi ekspertiz değerleri güncel TL'ye taşınmadı. Önce `go run ./cmd/hissebot sync tuik-inflation` çalıştırılmalıdır.",
		}, "asset_inventory_inflation_index_missing"
	}
	series, seriesOK := tamacro.PreferredInflationSeries(dataset)
	if !seriesOK {
		return dataset, KAPAssetValueIndexSummary{
			Source:             dataset.Source,
			SourceURL:          dataset.SourceURL,
			DataQualityWarning: "TÜİK/Yİ-ÜFE dosyasında kullanılabilir aylık endeks noktası yok.",
		}, "asset_inventory_inflation_index_empty"
	}
	return dataset, KAPAssetValueIndexSummary{
		Computed:     true,
		SeriesID:     series.ID,
		SeriesLabel:  tamacro.InflationSeriesLabel(dataset),
		Source:       firstNonEmptyString(series.Source, dataset.Source),
		SourceURL:    firstNonEmptyString(dataset.SourceURL, dataset.MetadataURL),
		LatestPeriod: tamacro.InflationLatestPeriod(dataset),
	}, ""
}

func applyKAPAssetInflationAdjustment(asset *KAPAssetInventoryItem, dataset tamacro.InflationDataset) {
	if asset == nil || asset.LatestValueTRY == nil {
		return
	}
	basePeriod := kapAssetInflationBasePeriod(*asset)
	if basePeriod == "" {
		asset.IndexedValueWarning = "asset_value_base_period_missing"
		return
	}
	adjustment, ok := tamacro.AdjustValueByInflation(dataset, *asset.LatestValueTRY, basePeriod)
	if !ok {
		asset.IndexedValueWarning = "asset_value_inflation_index_missing_for_period"
		return
	}
	asset.IndexedValueTRY = &adjustment.AdjustedValue
	asset.IndexedValueAsOf = adjustment.ToPeriod
	asset.IndexedValueBasePeriod = adjustment.FromPeriod
	asset.IndexedValueFactor = adjustment.Factor
	asset.IndexedValueSource = adjustment.SeriesName
}

func kapAssetInflationBasePeriod(asset KAPAssetInventoryItem) string {
	if period := monthPeriodFromDate(asset.ExpertiseDate); period != "" {
		return period
	}
	if period := monthPeriodFromPeriod(asset.LatestPeriod); period != "" {
		return period
	}
	return ""
}

func monthPeriodFromDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if ts, err := time.Parse("2006-01-02", value); err == nil {
		return ts.Format("2006-01")
	}
	if len(value) >= len("2006-01") {
		return monthPeriodFromPeriod(value[:len("2006-01")])
	}
	return ""
}

func monthPeriodFromPeriod(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "/", "-"))
	if len(value) >= len("2006-01") {
		value = value[:len("2006-01")]
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 || len(parts[0]) != 4 {
		return ""
	}
	month := parts[1]
	if len(month) == 1 {
		month = "0" + month
	}
	if len(month) != 2 || month < "01" || month > "12" {
		return ""
	}
	return parts[0] + "-" + month
}

func kapAssetSortValue(asset KAPAssetInventoryItem) float64 {
	if asset.IndexedValueTRY != nil {
		return *asset.IndexedValueTRY
	}
	return ptrFloatValue(asset.LatestValueTRY)
}

func cleanKAPAssetLocation(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 180 {
		value = truncateRunes(value, 180)
	}
	return value
}

func kapAssetInventorySummaryText(summary KAPAssetInventorySummary) string {
	if !summary.Computed {
		return summary.Summary
	}
	parts := []string{
		fmt.Sprintf("%d asset event ve %d birlesik envanter satiri okundu", summary.EventCount, summary.RawAssetCount),
		fmt.Sprintf("raporda %d denetlenebilir varlik satiri gosteriliyor", summary.DisplayAssetCount),
	}
	if summary.PortfolioSummary.TotalRealEstateValueExclVATTRY == nil && summary.PortfolioSummary.TotalRealEstateValueInclVATTRY == nil {
		parts = append(parts, "KDV haric/dahil portfoy toplami guvenilir tablo baglaminda bulunamadigi icin rapor degerlemesine yazilmadi")
	}
	return strings.Join(parts, "; ") + "."
}

func ptrFloatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
