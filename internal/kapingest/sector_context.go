package kapingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hissebot/internal/services/kapsectors"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

const (
	DefaultSectorFallback = "DİĞER"
	defaultKAPSectorsPath = "data/seed/kap_sectors.json"
	defaultPromptPackPath = "data/seed/kap_sector_prompt_pack.json"
)

type SectorContextStore struct {
	Source       string
	SourceURL    string
	Sectors      kapsectors.File
	Symbols      map[string]CompanySectorContext
	PromptPack   SectorPromptPack
	PromptLoaded bool
	Warnings     []string
}

type CompanySectorContext struct {
	Symbol        string                        `json:"symbol"`
	Title         string                        `json:"title,omitempty"`
	MainSector    string                        `json:"main_sector"`
	Sector        string                        `json:"sector"`
	MainSectorNo  string                        `json:"main_sector_no,omitempty"`
	SectorNo      string                        `json:"sector_no,omitempty"`
	MainSectorOID string                        `json:"main_sector_oid,omitempty"`
	SectorOID     string                        `json:"sector_oid,omitempty"`
	MKKMemberOID  string                        `json:"mkk_member_oid,omitempty"`
	KAPTypes      []string                      `json:"kap_types,omitempty"`
	AllSectors    []kapsectors.SectorMembership `json:"all_sectors,omitempty"`
	Source        string                        `json:"source"`
	Fallback      bool                          `json:"fallback"`
	Warnings      []string                      `json:"warnings,omitempty"`
}

type SectorPromptPack struct {
	SectorCount           int                         `json:"sector_count,omitempty"`
	MainSectors           []string                    `json:"main_sectors,omitempty"`
	CommonExtractionFocus []string                    `json:"common_extraction_focus,omitempty"`
	SectorPrompts         map[string]SectorPromptSpec `json:"sector_prompts,omitempty"`
	Raw                   map[string]json.RawMessage  `json:"-"`
}

type SectorPromptSpec struct {
	Prompt          string         `json:"prompt,omitempty"`
	Schema          map[string]any `json:"schema,omitempty"`
	ExtractionFocus []string       `json:"extraction_focus,omitempty"`
	RiskFocus       []string       `json:"risk_focus,omitempty"`
	Raw             map[string]any `json:"raw,omitempty"`
}

type PromptSelection struct {
	SelectedKey string         `json:"selected_key"`
	Source      string         `json:"source"`
	Prompt      string         `json:"prompt"`
	Schema      map[string]any `json:"schema,omitempty"`
	Focus       []string       `json:"focus,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
}

type BusinessModelTag struct {
	Tag            string   `json:"tag"`
	Confidence     float64  `json:"confidence"`
	Evidence       []string `json:"evidence,omitempty"`
	ReviewRequired bool     `json:"review_required,omitempty"`
}

type ExtractionRoute struct {
	DocumentType            string               `json:"document_type"`
	ParseStatus             string               `json:"parse_status,omitempty"`
	AnalysisUsable          bool                 `json:"analysis_usable"`
	Sector                  CompanySectorContext `json:"sector"`
	BusinessModels          []BusinessModelTag   `json:"business_models,omitempty"`
	PromptSelection         PromptSelection      `json:"prompt_selection"`
	UniversalExtractor      bool                 `json:"universal_extractor"`
	SectorSpecificExtractor string               `json:"sector_specific_extractor,omitempty"`
	DocumentTypeExtractor   string               `json:"document_type_extractor,omitempty"`
	BusinessModelExtractors []string             `json:"business_model_extractors,omitempty"`
	FinancialTableParser    bool                 `json:"financial_table_parser"`
	AssetInventoryExtractor bool                 `json:"asset_inventory_extractor"`
	EvidenceValidator       bool                 `json:"evidence_validator"`
	DuplicateMerger         bool                 `json:"duplicate_merger"`
	ContradictionDetector   bool                 `json:"contradiction_detector"`
	KnowledgeGraphWriter    bool                 `json:"knowledge_graph_writer"`
	HumanReviewRequired     bool                 `json:"human_review_required"`
	AIResolved              bool                 `json:"ai_resolved,omitempty"`
	AIConfidence            float64              `json:"ai_confidence,omitempty"`
	QualityGate             QualityGate          `json:"quality_gate,omitempty"`
	Warnings                []string             `json:"warnings,omitempty"`
}

func LoadSectorContextStore(sectorPath, promptPackPath string) SectorContextStore {
	if strings.TrimSpace(sectorPath) == "" {
		sectorPath = defaultKAPSectorsPath
	}
	if strings.TrimSpace(promptPackPath) == "" {
		promptPackPath = defaultPromptPackPath
	}
	store := SectorContextStore{
		Symbols: map[string]CompanySectorContext{},
	}
	if file, err := kapsectors.Load(sectorPath); err == nil {
		store.Sectors = file
		store.Source = file.Source
		store.SourceURL = file.SourceURL
		store.Symbols = buildCompanySectorMap(file)
	} else {
		store.Warnings = append(store.Warnings, "kap_sectors_load_failed: "+err.Error())
	}
	if pack, err := LoadSectorPromptPack(promptPackPath); err == nil {
		store.PromptPack = pack
		store.PromptLoaded = true
	} else if !os.IsNotExist(err) {
		store.Warnings = append(store.Warnings, "kap_sector_prompt_pack_load_failed: "+err.Error())
	} else {
		store.Warnings = append(store.Warnings, "kap_sector_prompt_pack_missing: "+filepath.Clean(promptPackPath))
	}
	return store
}

func LoadSectorPromptPack(path string) (SectorPromptPack, error) {
	var pack SectorPromptPack
	raw, err := os.ReadFile(path)
	if err != nil {
		return pack, err
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return pack, err
	}
	_ = json.Unmarshal(generic["sector_count"], &pack.SectorCount)
	_ = json.Unmarshal(generic["main_sectors"], &pack.MainSectors)
	_ = json.Unmarshal(generic["common_extraction_focus"], &pack.CommonExtractionFocus)
	pack.Raw = generic
	pack.SectorPrompts = map[string]SectorPromptSpec{}
	var prompts map[string]json.RawMessage
	if err := json.Unmarshal(generic["sector_prompts"], &prompts); err == nil {
		keys := make([]string, 0, len(prompts))
		for key := range prompts {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			spec := decodeSectorPromptSpec(prompts[key])
			pack.SectorPrompts[normalizeSectorKey(key)] = spec
		}
	}
	return pack, nil
}

func decodeSectorPromptSpec(raw json.RawMessage) SectorPromptSpec {
	spec := SectorPromptSpec{Raw: map[string]any{}}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		spec.Prompt = text
		return spec
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		spec.Prompt = string(raw)
		return spec
	}
	spec.Raw = obj
	for _, key := range []string{"prompt", "system_prompt", "analysis_prompt", "text"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			spec.Prompt = value
			break
		}
	}
	if schema, ok := obj["schema"].(map[string]any); ok {
		spec.Schema = schema
	}
	spec.ExtractionFocus = stringSliceFromAny(obj["extraction_focus"])
	if len(spec.ExtractionFocus) == 0 {
		spec.ExtractionFocus = stringSliceFromAny(obj["focus"])
	}
	spec.RiskFocus = stringSliceFromAny(obj["risk_focus"])
	return spec
}

func buildCompanySectorMap(file kapsectors.File) map[string]CompanySectorContext {
	out := map[string]CompanySectorContext{}
	for symbol, entry := range file.Entries {
		key := strings.ToUpper(storage.NormalizeTicker(symbol))
		if key == "" {
			key = strings.ToUpper(strings.TrimSpace(symbol))
		}
		if key == "" {
			continue
		}
		out[key] = companySectorContextFromEntry(entry, file.Source)
	}
	return out
}

func companySectorContextFromEntry(entry kapsectors.CompanySector, source string) CompanySectorContext {
	mainSector := strings.TrimSpace(entry.MainSector)
	sector := strings.TrimSpace(entry.Sector)
	warnings := []string{}
	if mainSector == "" {
		mainSector = DefaultSectorFallback
		warnings = append(warnings, "main_sector_missing_fallback")
	}
	if sector == "" {
		sector = mainSector
		if sector == "" {
			sector = DefaultSectorFallback
		}
		warnings = append(warnings, "sector_missing_fallback")
	}
	return CompanySectorContext{
		Symbol:        strings.ToUpper(strings.TrimSpace(entry.Symbol)),
		Title:         entry.Title,
		MainSector:    mainSector,
		Sector:        sector,
		MainSectorNo:  entry.MainSectorNo,
		SectorNo:      entry.SectorNo,
		MainSectorOID: entry.MainSectorOID,
		SectorOID:     entry.SectorOID,
		MKKMemberOID:  entry.MKKMemberOID,
		KAPTypes:      append([]string{}, entry.KAPTypes...),
		AllSectors:    append([]kapsectors.SectorMembership{}, entry.AllSectors...),
		Source:        firstNonEmptyAsset(source, kapsectors.SourceName),
		Warnings:      warnings,
	}
}

func (s SectorContextStore) Lookup(symbol string) CompanySectorContext {
	key := strings.ToUpper(storage.NormalizeTicker(symbol))
	if key == "" {
		key = strings.ToUpper(strings.TrimSpace(symbol))
	}
	if key != "" {
		if entry, ok := s.Symbols[key]; ok {
			return entry
		}
	}
	return fallbackSectorContext(key, "kap_sector_symbol_not_found")
}

func fallbackSectorContext(symbol string, warning string) CompanySectorContext {
	warnings := []string{}
	if warning != "" {
		warnings = append(warnings, warning)
	}
	return CompanySectorContext{
		Symbol:     strings.ToUpper(strings.TrimSpace(symbol)),
		MainSector: DefaultSectorFallback,
		Sector:     DefaultSectorFallback,
		Source:     "fallback",
		Fallback:   true,
		Warnings:   warnings,
	}
}

func (s SectorContextStore) SelectPrompt(company CompanySectorContext) PromptSelection {
	keys := []struct {
		key    string
		source string
	}{
		{company.Sector, "sector"},
		{company.MainSector, "main_sector"},
		{DefaultSectorFallback, "fallback"},
	}
	for _, item := range keys {
		key := normalizeSectorKey(item.key)
		if spec, ok := s.PromptPack.SectorPrompts[key]; ok {
			return promptSelectionFromSpec(item.key, item.source, spec, s.PromptPack.CommonExtractionFocus)
		}
	}
	return PromptSelection{
		SelectedKey: DefaultSectorFallback,
		Source:      "generated_generic",
		Prompt:      GenericUniversalPrompt(company),
		Focus:       append([]string{}, s.PromptPack.CommonExtractionFocus...),
		Warnings:    []string{"sector_prompt_missing_generated_generic"},
	}
}

func promptSelectionFromSpec(key, source string, spec SectorPromptSpec, commonFocus []string) PromptSelection {
	focus := append([]string{}, commonFocus...)
	focus = append(focus, spec.ExtractionFocus...)
	focus = dedupeStrings(focus)
	prompt := strings.TrimSpace(spec.Prompt)
	if prompt == "" {
		prompt = GenericUniversalPrompt(CompanySectorContext{Sector: key, MainSector: key})
	}
	return PromptSelection{
		SelectedKey: strings.TrimSpace(key),
		Source:      source,
		Prompt:      prompt,
		Schema:      spec.Schema,
		Focus:       focus,
	}
}

func GenericUniversalPrompt(company CompanySectorContext) string {
	sector := firstNonEmptyAsset(company.Sector, company.MainSector, DefaultSectorFallback)
	return "KAP PDF yatırım bilgisi çıkarım motorusun. Metinde olmayan bilgiyi uydurma; her çıkarımı kaynak snippet, dönem, para birimi ve confidence ile döndür. Şirket sektörü: " + sector + ". Belirsiz alanları null/review_required olarak işaretle."
}

func BuildExtractionRoute(doc RawDocument, sectorStore SectorContextStore) ExtractionRoute {
	symbol := strings.ToUpper(strings.TrimSpace(doc.Ticker))
	if symbol == "" {
		symbol = ExtractTicker("", doc.FilePath)
	}
	company := sectorStore.Lookup(symbol)
	documentType := NormalizeDocumentType(doc.DocumentTypeGuess)
	if documentType == DocumentOther || documentType == "" {
		documentType = ClassifyDocument(doc.Text, doc.FileName)
	}
	documentType = NormalizeDocumentType(documentType)
	gate := QualityGateForRawDocument(doc)
	prompt := sectorStore.SelectPrompt(company)
	businessModels := []BusinessModelTag{}
	if gate.AnalysisUsable {
		businessModels = ClassifyBusinessModels(company, doc.Text)
	}
	warnings := append([]string{}, company.Warnings...)
	warnings = append(warnings, prompt.Warnings...)
	warnings = append(warnings, QualityGateWarnings(gate)...)
	humanReview := gate.HumanReviewRequired || company.Fallback || documentType == DocumentOther || documentType == DocumentUnknown
	for _, tag := range businessModels {
		if tag.ReviewRequired {
			humanReview = true
			break
		}
	}
	return ExtractionRoute{
		DocumentType:            documentType,
		ParseStatus:             gate.Status,
		AnalysisUsable:          gate.AnalysisUsable,
		Sector:                  company,
		BusinessModels:          businessModels,
		PromptSelection:         prompt,
		UniversalExtractor:      gate.AnalysisUsable,
		SectorSpecificExtractor: extractorKey("sector", company.Sector),
		DocumentTypeExtractor:   extractorKey("document", documentType),
		BusinessModelExtractors: businessModelExtractorKeys(businessModels),
		FinancialTableParser:    gate.AnalysisUsable && documentNeedsFinancialTableParser(documentType),
		AssetInventoryExtractor: gate.AnalysisUsable && routeNeedsAssetExtractor(company, businessModels, doc),
		EvidenceValidator:       true,
		DuplicateMerger:         true,
		ContradictionDetector:   true,
		KnowledgeGraphWriter:    true,
		HumanReviewRequired:     humanReview,
		QualityGate:             gate,
		Warnings:                dedupeStrings(warnings),
	}
}

func businessModelExtractorKeys(tags []BusinessModelTag) []string {
	out := []string{}
	for _, tag := range tags {
		out = append(out, extractorKey("business_model", tag.Tag))
	}
	return dedupeStrings(out)
}

func extractorKey(prefix, value string) string {
	value = strings.Trim(util.SlugTR(value), "-_ ")
	if value == "" {
		value = strings.ToLower(DefaultSectorFallback)
	}
	return prefix + ":" + strings.ReplaceAll(value, " ", "_")
}

func documentNeedsFinancialTableParser(documentType string) bool {
	switch NormalizeDocumentType(documentType) {
	case DocumentFinancialStatement, DocumentAnnualReport, DocumentActivityReport, DocumentInterimActivityReport, DocumentAuditReport, DocumentIndependentAuditReport:
		return true
	default:
		return false
	}
}

func routeNeedsAssetExtractor(company CompanySectorContext, tags []BusinessModelTag, doc RawDocument) bool {
	slug := util.SlugTR(company.Sector + " " + company.MainSector + " " + doc.DocumentTypeGuess + " " + doc.FileName)
	if slugContains(slug, "gayrimenkul") || slugContains(slug, "degerleme") || slugContains(slug, "maddi duran") {
		return true
	}
	for _, tag := range tags {
		switch tag.Tag {
		case "reit", "real_estate_developer", "hotel_accommodation", "leasing_rental", "industrial_manufacturing", "energy_generation", "regulated_utility", "construction_contracting", "infrastructure":
			return true
		}
	}
	return containsAssetCue(doc.Text)
}

func normalizeSectorKey(value string) string {
	return strings.ToUpper(util.SlugTR(strings.TrimSpace(value)))
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := []string{}
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	default:
		return nil
	}
}

func ClassifyBusinessModels(company CompanySectorContext, text string) []BusinessModelTag {
	source := util.SlugTR(company.MainSector + " " + company.Sector + " " + text)
	tags := []BusinessModelTag{}
	add := func(tag string, confidence float64, evidence ...string) {
		if confidence <= 0 {
			return
		}
		tags = append(tags, BusinessModelTag{
			Tag:            tag,
			Confidence:     clampAsset(confidence, 0, 1),
			Evidence:       dedupeStrings(evidence),
			ReviewRequired: confidence < 0.65,
		})
	}
	has := func(values ...string) bool {
		for _, value := range values {
			if slugContains(source, value) {
				return true
			}
		}
		return false
	}
	switch {
	case has("bankalar", "mevduat", "kredi portfoyu"):
		add("bank", 0.92, "KAP sector/content bank signal")
	case has("katilim bank"):
		add("participation_bank", 0.88, "participation banking signal")
	case has("sigorta", "emeklilik"):
		if has("emeklilik", "hayat") {
			add("pension_life_insurance", 0.82, "life/pension insurance signal")
		} else {
			add("insurance", 0.86, "insurance signal")
		}
	case has("araci kurum", "portfoy araciligi", "halka arz"):
		add("brokerage", 0.84, "brokerage signal")
	case has("gayrimenkul yatirim ortakligi", "gyo"):
		add("reit", 0.94, "GYO sector/content signal")
	case has("gayrimenkul faaliyetleri", "proje gelistirme", "arsa"):
		add("real_estate_developer", 0.74, "real estate development signal")
	case has("holding", "yatirim sirket"):
		add("holding", 0.88, "holding/investment company signal")
		if has("yatirim sirket") {
			add("investment_company", 0.78, "investment company signal")
		}
	case has("girisim sermayesi"):
		add("venture_capital_investment_trust", 0.90, "venture capital trust signal")
	case has("menkul kiymet yatirim ortakligi"):
		add("securities_investment_trust", 0.90, "securities investment trust signal")
	case has("finansal kiralama", "faktoring", "finansman sirket"):
		add("leasing_factoring", 0.86, "leasing/factoring signal")
	case has("varlik yonetim"):
		add("asset_management", 0.86, "asset management signal")
	case has("bilisim", "yazilim", "lisans", "abonelik"):
		if has("yazilim") {
			add("software", 0.82, "software signal")
		}
		add("technology_services", 0.74, "technology service signal")
	case has("savunma"):
		add("defense", 0.88, "defense sector signal")
	case has("elektrik gaz ve buhar", "santral", "kurulu guc", "yekdem"):
		add("energy_generation", 0.78, "energy generation signal")
		if has("yenilenebilir", "ges", "res", "hidroelektrik", "jeotermal") {
			add("renewable_energy", 0.82, "renewable energy signal")
		}
		if has("dogal gaz dagitim", "dogalgaz dagitim") {
			add("natural_gas_distribution", 0.86, "natural gas distribution signal")
		}
		add("regulated_utility", 0.68, "regulated utility signal")
	case has("komur", "linyit"):
		add("mining_coal_lignite", 0.86, "coal/lignite mining signal")
	case has("petrol", "dogal gaz cikartilmasi"):
		add("mining_oil_gas", 0.84, "oil/gas extraction signal")
	case has("metal cevheri"):
		add("mining_metal_ore", 0.84, "metal ore mining signal")
	case has("madencilik", "tas ocakciligi"):
		add("mining_other", 0.72, "mining signal")
	case has("insaat", "bayindirlik", "ihale", "hakedi"):
		add("construction_contracting", 0.78, "construction contracting signal")
	case has("toptan ticaret"):
		add("wholesale_trade", 0.78, "wholesale trade signal")
	case has("perakende", "magaza", "sube"):
		add("retail", 0.80, "retail signal")
	case has("ulastirma", "lojistik", "depolama"):
		add("logistics_transportation", 0.80, "transport/logistics signal")
	case has("konaklama", "otel", "oda", "doluluk"):
		add("hotel_accommodation", 0.82, "hotel/accommodation signal")
	case has("yiyecek", "icecek hizmetleri", "restoran"):
		add("restaurant_food_service", 0.80, "restaurant signal")
	case has("telekom"):
		add("telecom", 0.88, "telecommunication signal")
	case has("yayimcilik", "medya"):
		add("media_publishing", 0.76, "media/publishing signal")
	case has("saglik", "hastane", "klinik"):
		add("healthcare", 0.82, "healthcare signal")
	case has("spor"):
		add("sports_club", 0.78, "sports signal")
	case has("tarim", "hayvancilik"):
		add("agriculture", 0.78, "agriculture signal")
	case has("ormancilik", "tomruk"):
		add("forestry", 0.78, "forestry signal")
	case has("balikcilik", "su urunleri"):
		add("fishery_aquaculture", 0.78, "fishery/aquaculture signal")
	}
	if has("gida", "icecek", "tutun") {
		add("food_beverage", 0.78, "food/beverage/tobacco signal")
	}
	if has("tekstil", "giyim", "deri") {
		add("textile_apparel", 0.78, "textile/apparel/leather signal")
	}
	if has("kagit", "ambalaj", "basim") {
		add("paper_packaging", 0.78, "paper/packaging/printing signal")
	}
	if has("kimya", "kimyasal") {
		add("chemicals", 0.78, "chemicals signal")
	}
	if has("ilac", "eczacilik", "farmasotik") {
		add("pharmaceuticals", 0.78, "pharmaceuticals signal")
	}
	if has("petrol", "lastik", "plastik") {
		add("petroleum_plastics", 0.74, "petroleum/rubber/plastics signal")
	}
	if has("cimento", "cam", "seramik", "tas ve topraga dayali", "yapi malzemesi") {
		add("cement_building_materials", 0.78, "cement/building materials signal")
	}
	if has("ana metal", "celik", "aluminyum", "bakir", "demir") {
		add("steel_metal", 0.78, "steel/metal signal")
	}
	if has("makine", "elektrikli cihaz", "metal esya") {
		add("machinery_electrical_equipment", 0.78, "machinery/electrical equipment signal")
	}
	if has("otomotiv", "ulasim araclari", "motorlu arac") {
		add("automotive", 0.78, "automotive signal")
	}
	if has("altyapi", "liman", "terminal", "otoyol", "kopru") {
		add("infrastructure", 0.72, "infrastructure signal")
	}
	if has("havacilik", "havayolu", "ucak", "yolcu") {
		add("aviation", 0.76, "aviation signal")
	}
	if has("depolama", "antrepo", "warehouse") {
		add("warehousing", 0.74, "warehousing signal")
	}
	if has("eglence", "oyun", "yaratici sanat", "gosteri sanat") {
		add("entertainment", 0.74, "entertainment signal")
	}
	if has("hukuk", "muhasebe", "danismanlik", "profesyonel hizmet") {
		add("professional_services", 0.72, "professional services signal")
	}
	if has("mimarlik", "muhendislik", "teknik muayene", "teknik analiz") {
		add("engineering_architecture", 0.78, "engineering/architecture signal")
	}
	if has("bilimsel arastirma", "ar-ge", "arge", "tubitak") {
		add("r_and_d", 0.76, "R&D signal")
	}
	if has("reklam", "pazar arastirmasi", "medya satin alma") {
		add("advertising_market_research", 0.78, "advertising/market research signal")
	}
	if has("kiralama ve leasing", "kiralama faaliyetleri", "varlik kiralama") {
		add("leasing_rental", 0.72, "leasing/rental signal")
	}
	if has("istihdam", "personel yerlestirme", "dis kaynak") {
		add("employment_services", 0.76, "employment services signal")
	}
	if has("seyahat acentesi", "tur operatoru", "rezervasyon") {
		add("travel_agency", 0.78, "travel agency signal")
	}
	if has("guvenlik", "sorusturma") {
		add("security_services", 0.78, "security services signal")
	}
	if has("tesis yonetimi", "bina ve cevre", "temizlik", "bakim hizmet") {
		add("facility_management", 0.78, "facility management signal")
	}
	if has("imalat", "sanayi", "uretim", "kapasite") {
		add("industrial_manufacturing", 0.62, "generic manufacturing signal")
	}
	if len(tags) == 0 {
		add("other", 0.40, "no strong business model signal")
	}
	return filterBusinessModelTagsForCompanySector(company, mergeBusinessModelTags(tags))
}

func mergeBusinessModelTags(tags []BusinessModelTag) []BusinessModelTag {
	byTag := map[string]BusinessModelTag{}
	for _, tag := range tags {
		current, ok := byTag[tag.Tag]
		if !ok || tag.Confidence > current.Confidence {
			current = tag
		}
		current.Evidence = dedupeStrings(append(current.Evidence, tag.Evidence...))
		current.ReviewRequired = current.Confidence < 0.65
		byTag[tag.Tag] = current
	}
	keys := make([]string, 0, len(byTag))
	for key := range byTag {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]BusinessModelTag, 0, len(keys))
	for _, key := range keys {
		out = append(out, byTag[key])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].Tag < out[j].Tag
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}

func filterBusinessModelTagsForCompanySector(company CompanySectorContext, tags []BusinessModelTag) []BusinessModelTag {
	if len(tags) == 0 {
		return tags
	}
	sector := util.SlugTR(strings.Join([]string{company.MainSector, company.Sector}, " "))
	if !strings.Contains(sector, "savunma") {
		return tags
	}
	allowed := map[string]bool{
		"defense":                        true,
		"industrial_manufacturing":       true,
		"machinery_electrical_equipment": true,
		"engineering_architecture":       true,
		"r_and_d":                        true,
		"technology_services":            true,
		"software":                       true,
		"aviation":                       true,
		"infrastructure":                 true,
	}
	filtered := make([]BusinessModelTag, 0, len(tags))
	for _, tag := range tags {
		if allowed[tag.Tag] {
			filtered = append(filtered, tag)
		}
	}
	if len(filtered) == 0 {
		return tags
	}
	return filtered
}
