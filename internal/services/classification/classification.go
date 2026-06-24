package classification

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/services/kapsectors"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type Options struct {
	OutputPath        string
	SourceFiles       []string
	KAPSectorsFile    string
	DisableKAPSectors bool
	UseTradingView    bool
	MaxPeers          int
	PreserveExisting  bool
}

type Result struct {
	OutputPath            string `json:"output_path"`
	Equities              int    `json:"equities"`
	Entries               int    `json:"entries"`
	OfficialClassified    int    `json:"official_classified"`
	ActivityClassified    int    `json:"activity_classified"`
	KeywordClassified     int    `json:"keyword_classified"`
	ExternalClassified    int    `json:"external_classified"`
	KAPSectorClassified   int    `json:"kap_sector_classified"`
	TradingViewClassified int    `json:"tradingview_classified"`
	LowConfidence         int    `json:"low_confidence"`
	PeerGroups            int    `json:"peer_groups"`
}

type File struct {
	Source      string           `json:"source"`
	Version     string           `json:"version"`
	GeneratedAt time.Time        `json:"generated_at"`
	Summary     Summary          `json:"summary"`
	Entries     map[string]Entry `json:"entries"`
}

type Summary struct {
	Entries               int `json:"entries"`
	OfficialClassified    int `json:"official_classified"`
	ActivityClassified    int `json:"activity_classified"`
	KeywordClassified     int `json:"keyword_classified"`
	ExternalClassified    int `json:"external_classified"`
	KAPSectorClassified   int `json:"kap_sector_classified"`
	TradingViewClassified int `json:"tradingview_classified"`
	LowConfidence         int `json:"low_confidence"`
	PeerGroups            int `json:"peer_groups"`
}

type tradingViewScanResponse struct {
	TotalCount int                    `json:"totalCount"`
	Data       []tradingViewScanEntry `json:"data"`
}

type tradingViewScanEntry struct {
	S string `json:"s"`
	D []any  `json:"d"`
}

type Entry struct {
	Sector      string   `json:"sector"`
	Industry    string   `json:"industry"`
	PeerGroup   string   `json:"peer_group"`
	PeerSymbols []string `json:"peer_symbols"`
	Source      string   `json:"source"`
	Confidence  float64  `json:"confidence"`
	Evidence    []string `json:"evidence,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Activity    string   `json:"activity_text,omitempty"`
}

type classificationCandidate struct {
	entry      Entry
	sourceKind string
}

type rule struct {
	sector   string
	industry string
	keywords []string
}

var industryRules = []rule{
	{sector: "Gayrimenkul Yatırım Ortaklığı", industry: "Gayrimenkul Yatırım Ortaklığı", keywords: []string{"gayrimenkul yatirim ortakligi", "gayrimenkul yatırım ortaklığı", "gyo"}},
	{sector: "Banka", industry: "Bankacılık", keywords: []string{"bank", "banka"}},
	{sector: "Sigorta ve Emeklilik", industry: "Sigorta", keywords: []string{"sigorta", "emeklilik"}},
	{sector: "Finansal Hizmetler", industry: "Faktoring", keywords: []string{"faktoring"}},
	{sector: "Finansal Hizmetler", industry: "Finansal Kiralama", keywords: []string{"finansal kiralama", "leasing"}},
	{sector: "Finansal Hizmetler", industry: "Varlık Kiralama", keywords: []string{"varlik kiralama", "varlık kiralama"}},
	{sector: "Finansal Hizmetler", industry: "Aracı Kurum ve Yatırım Hizmetleri", keywords: []string{"araci kurum", "aracı kurum", "menkul deger", "menkul değer", "yatirim menkul", "yatırım menkul", "portfoy", "portföy"}},
	{sector: "Holding ve Yatırım", industry: "Holding", keywords: []string{"holding"}},
	{sector: "Savunma ve Elektronik", industry: "Savunma Teknolojileri", keywords: []string{"savunma", "askeri", "guvenlik teknolojileri", "güvenlik teknolojileri"}},
	{sector: "Teknoloji ve Elektronik", industry: "Yazılım ve Bilişim", keywords: []string{"yazilim", "yazılım", "bilisim", "bilişim", "teknoloji", "teknolojik"}},
	{sector: "Teknoloji ve Elektronik", industry: "Elektronik ve Haberleşme", keywords: []string{"elektronik", "haberlesme", "haberleşme", "telekom", "iletisim", "iletişim"}},
	{sector: "Enerji", industry: "Elektrik ve Yenilenebilir Enerji", keywords: []string{"enerji", "elektrik", "gunes", "güneş", "ruzgar", "rüzgar", "jeotermal", "hidroelektrik"}},
	{sector: "Petrol, Gaz ve Kimya", industry: "Petrol ve Gaz", keywords: []string{"petrol", "dogalgaz", "doğalgaz", "akaryakit", "akaryakıt", "gaz"}},
	{sector: "Petrol, Gaz ve Kimya", industry: "Kimya", keywords: []string{"kimya", "boya", "plastik", "petrokimya"}},
	{sector: "Gıda ve İçecek", industry: "Gıda Üretimi", keywords: []string{"gida", "gıda", "icecek", "içecek", "sut", "süt", "et ve sut", "tarim", "tarım", "un", "yem"}},
	{sector: "Perakende ve Ticaret", industry: "Perakende", keywords: []string{"perakende", "magaza", "mağaza", "market", "ticaret", "toptan"}},
	{sector: "Otomotiv ve Yan Sanayi", industry: "Otomotiv", keywords: []string{"otomotiv", "motorlu arac", "motorlu araç", "oto", "traktor", "traktör"}},
	{sector: "Ulaştırma ve Lojistik", industry: "Havacılık", keywords: []string{"hava yollari", "hava yolları", "havacilik", "havacılık"}},
	{sector: "Ulaştırma ve Lojistik", industry: "Lojistik ve Taşımacılık", keywords: []string{"lojistik", "tasimacilik", "taşımacılık", "liman", "denizcilik"}},
	{sector: "Demir Çelik ve Metal", industry: "Demir Çelik", keywords: []string{"demir", "celik", "çelik", "metal", "aluminyum", "alüminyum", "dokum", "döküm"}},
	{sector: "Çimento ve İnşaat Malzemeleri", industry: "Çimento", keywords: []string{"cimento", "çimento", "beton"}},
	{sector: "Çimento ve İnşaat Malzemeleri", industry: "Yapı Malzemeleri", keywords: []string{"insaat malzem", "inşaat malzem", "seramik", "karo", "yapi", "yapı"}},
	{sector: "İnşaat ve Taahhüt", industry: "İnşaat ve Taahhüt", keywords: []string{"insaat", "inşaat", "taahhut", "taahhüt"}},
	{sector: "Cam ve Ambalaj", industry: "Cam", keywords: []string{"cam"}},
	{sector: "Cam ve Ambalaj", industry: "Ambalaj", keywords: []string{"ambalaj", "kagit", "kağıt", "karton"}},
	{sector: "Tekstil ve Hazır Giyim", industry: "Tekstil", keywords: []string{"tekstil", "dokuma", "konfeksiyon", "hazir giyim", "hazır giyim"}},
	{sector: "Sağlık ve İlaç", industry: "İlaç ve Sağlık", keywords: []string{"ilac", "ilaç", "saglik", "sağlık", "hastane", "tibbi", "tıbbi"}},
	{sector: "Turizm ve Eğlence", industry: "Turizm ve Otelcilik", keywords: []string{"turizm", "otel", "otelcilik", "eglence", "eğlence"}},
	{sector: "Makine ve Endüstriyel Üretim", industry: "Makine", keywords: []string{"makina", "makine", "endustri", "endüstri", "imalat"}},
	{sector: "Mobilya ve Dayanıklı Tüketim", industry: "Mobilya", keywords: []string{"mobilya", "yatak"}},
	{sector: "Madencilik", industry: "Madencilik", keywords: []string{"maden", "madencilik", "altin", "altın", "komur", "kömür"}},
}

var tradingViewSectorTranslations = map[string]string{
	"Finance":                "Finans",
	"Electronic Technology":  "Elektronik Teknoloji",
	"Non-Energy Minerals":    "Enerji Dışı Mineraller",
	"Consumer Non-Durables":  "Dayanıklı Olmayan Tüketici Ürünleri",
	"Utilities":              "Elektrik, Su ve Gaz Hizmetleri",
	"Producer Manufacturing": "Üretici İmalatı",
	"Energy Minerals":        "Enerji Mineralleri",
	"Process Industries":     "İşlenebilen Endüstriler",
	"Consumer Durables":      "Dayanıklı Tüketim Malları",
	"Transportation":         "Taşımacılık",
	"Industrial Services":    "Endüstriyel Hizmetler",
	"Retail Trade":           "Perakende Satış",
	"Communications":         "İletişim",
	"Consumer Services":      "Tüketici Hizmetleri",
	"Technology Services":    "Teknoloji Hizmetleri",
	"Distribution Services":  "Dağıtım Servisleri",
	"Health Technology":      "Sağlık Teknolojisi",
	"Health Services":        "Sağlık Hizmetleri",
	"Miscellaneous":          "Çeşitli Hizmetler",
	"Commercial Services":    "Ticari Hizmetler",
}

var tradingViewIndustryTranslations = map[string]string{
	"Aerospace & Defense":                "Uzay ve Savunma",
	"Major Banks":                        "Majör Bankalar",
	"Regional Banks":                     "Bölgesel Bankalar",
	"Investment Banks/Brokers":           "Yatırım Bankaları ve Aracı Kurumlar",
	"Investment Managers":                "Yatırım Yöneticileri",
	"Finance/Rental/Leasing":             "Finansal Kiralama ve Leasing",
	"Real Estate Investment Trusts":      "Gayrimenkul Yatırım Ortaklıkları",
	"Real Estate Development":            "Gayrimenkul Geliştirme",
	"Steel":                              "Çelik",
	"Construction Materials":             "İnşaat Malzemeleri",
	"Motor Vehicles":                     "Motorlu Taşıtlar",
	"Airlines":                           "Havayolları",
	"Electrical Products":                "Elektrikli Ürünler",
	"Alternative Power Generation":       "Alternatif Enerji Üretimi",
	"Electric Utilities":                 "Elektrik Hizmetleri",
	"Gas Distributors":                   "Gaz Distribütörleri",
	"Packaged Software":                  "Paket Yazılım",
	"Food Retail":                        "Gıda Perakende",
	"Food: Specialty/Candy":              "Gıda: Özel/Şeker",
	"Food: Major Diversified":            "Gıda: Başlıca Çeşitlendirilmiş",
	"Beverages: Non-Alcoholic":           "İçecekler: Alkolsüz",
	"Restaurants":                        "Restoranlar",
	"Textiles":                           "Tekstil",
	"Chemicals: Specialty":               "Kimyasallar: Özel",
	"Precious Metals":                    "Değerli Metaller",
	"Oil Refining/Marketing":             "Petrol Rafineri ve Pazarlama",
	"Engineering & Construction":         "Mühendislik ve İnşaat",
	"Trucks/Construction/Farm Machinery": "Kamyon, İnşaat ve Tarım Makineleri",
	"Wireless Telecommunications":        "Kablosuz Telekomünikasyon",
	"Major Telecommunications":           "Başlıca Telekomünikasyon",
	"Multi-Line Insurance":               "Çok Hatlı Sigorta",
}

func Sync(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts Options) (Result, error) {
	opts = normalizeOptions(cfg, opts)
	existing, err := loadExisting(opts.OutputPath)
	if err != nil {
		return Result{}, err
	}
	external, err := loadExternalSources(opts.SourceFiles)
	if err != nil {
		return Result{}, err
	}
	kapSectorSource, err := loadKAPSectorSource(opts.KAPSectorsFile, !opts.DisableKAPSectors)
	if err != nil {
		return Result{}, err
	}
	tradingView, err := loadTradingViewSource(ctx, cfg, opts.UseTradingView)
	if err != nil {
		return Result{}, err
	}
	equities, err := store.List()
	if err != nil {
		return Result{}, err
	}

	entries := map[string]Entry{}
	grouped := map[string][]string{}
	sectorGrouped := map[string][]string{}
	result := Result{OutputPath: opts.OutputPath, Equities: len(equities)}
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		if equity == nil {
			continue
		}
		symbol := storage.NormalizeTicker(equity.Ticker)
		if symbol == "" {
			continue
		}
		candidate := classifyEquity(store, equity)
		entry := candidate.entry
		if sourced, ok := external[symbol]; ok && sourced.Confidence >= entry.Confidence {
			entry = sourced
			candidate.sourceKind = "external"
		}
		if sourced, ok := tradingView[symbol]; ok && sourced.Confidence >= entry.Confidence {
			entry = sourced
			candidate.sourceKind = "tradingview"
		}
		if sourced, ok := kapSectorSource[symbol]; ok {
			entry = sourced
			candidate.sourceKind = "kap_sector"
		}
		if opts.PreserveExisting && entry.Source != kapsectors.SourceName {
			if preserved, ok := preserveExisting(existing.Entries[symbol], entry); ok {
				entry = preserved
				candidate.sourceKind = sourceKind(entry.Source)
			}
		}
		if entry.PeerGroup == "" {
			entry.PeerGroup = peerGroup(entry.Sector, entry.Industry)
		}
		if entry.Confidence < 0.60 {
			entry.Warnings = appendUnique(entry.Warnings, "low_classification_confidence")
			result.LowConfidence++
		}
		entries[symbol] = entry
		grouped[entry.PeerGroup] = append(grouped[entry.PeerGroup], symbol)
		sectorGrouped[entry.Sector] = append(sectorGrouped[entry.Sector], symbol)
		switch candidate.sourceKind {
		case "official":
			result.OfficialClassified++
		case "activity":
			result.ActivityClassified++
		case "keyword":
			result.KeywordClassified++
		case "external":
			result.ExternalClassified++
		case "kap_sector":
			result.KAPSectorClassified++
		case "tradingview":
			result.TradingViewClassified++
		}
	}

	for group, symbols := range grouped {
		sort.Strings(symbols)
		grouped[group] = symbols
	}
	for sector, symbols := range sectorGrouped {
		sort.Strings(symbols)
		sectorGrouped[sector] = symbols
	}
	for symbol, entry := range entries {
		existingPeers := []string{}
		if previous := existing.Entries[symbol]; shouldReuseExistingPeers(previous, entry) {
			existingPeers = previous.PeerSymbols
		}
		peers := peersFor(symbol, grouped[entry.PeerGroup], sectorGrouped[entry.Sector], existingPeers, 3, opts.MaxPeers)
		entry.PeerSymbols = peers
		entries[symbol] = entry
	}

	out := File{
		Source:      "kap_mkk_classification_sync",
		Version:     time.Now().UTC().Format("2006-01-02"),
		GeneratedAt: time.Now().UTC(),
		Entries:     entries,
	}
	out.Summary = Summary{
		Entries:               len(entries),
		OfficialClassified:    result.OfficialClassified,
		ActivityClassified:    result.ActivityClassified,
		KeywordClassified:     result.KeywordClassified,
		ExternalClassified:    result.ExternalClassified,
		KAPSectorClassified:   result.KAPSectorClassified,
		TradingViewClassified: result.TradingViewClassified,
		LowConfidence:         result.LowConfidence,
		PeerGroups:            len(grouped),
	}
	if err := util.WriteJSON(opts.OutputPath, out); err != nil {
		return result, err
	}
	result.Entries = len(entries)
	result.PeerGroups = len(grouped)
	return result, nil
}

func normalizeOptions(cfg config.Config, opts Options) Options {
	if strings.TrimSpace(opts.OutputPath) == "" {
		opts.OutputPath = filepath.Join(cfg.SeedDir, "sector_classifications.json")
	}
	if strings.TrimSpace(opts.KAPSectorsFile) == "" {
		opts.KAPSectorsFile = cfg.KAPSectorsFile
	}
	if opts.MaxPeers <= 0 {
		opts.MaxPeers = 30
	}
	return opts
}

func loadKAPSectorSource(path string, enabled bool) (map[string]Entry, error) {
	if !enabled {
		return map[string]Entry{}, nil
	}
	file, err := kapsectors.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]Entry{}
	for symbol, sector := range file.Entries {
		symbol = storage.NormalizeTicker(symbol)
		if symbol == "" {
			continue
		}
		entry := completeEntry(Entry{
			Sector:     firstNonEmpty(sector.MainSector, sector.Sector),
			Industry:   firstNonEmpty(sector.Sector, sector.MainSector),
			PeerGroup:  peerGroup(firstNonEmpty(sector.MainSector, sector.Sector), firstNonEmpty(sector.Sector, sector.MainSector)),
			Source:     kapsectors.SourceName,
			Confidence: 0.97,
			Evidence: []string{
				"kap.sectors.url=" + file.SourceURL,
				"kap.sectors.main_sector=" + sector.MainSector,
				"kap.sectors.sector=" + sector.Sector,
				"kap.sectors.sector_no=" + sector.SectorNo,
			},
		})
		out[symbol] = entry
	}
	return out, nil
}

func loadExisting(path string) (File, error) {
	out := File{Entries: map[string]Entry{}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("%s: %w", path, err)
	}
	if out.Entries == nil {
		out.Entries = map[string]Entry{}
	}
	normalized := map[string]Entry{}
	for symbol, entry := range out.Entries {
		symbol = storage.NormalizeTicker(symbol)
		if symbol != "" {
			normalized[symbol] = entry
		}
	}
	out.Entries = normalized
	return out, nil
}

func loadExternalSources(paths []string) (map[string]Entry, error) {
	out := map[string]Entry{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		entries, err := loadExternalSource(path)
		if err != nil {
			return nil, err
		}
		for symbol, entry := range entries {
			out[symbol] = entry
		}
	}
	return out, nil
}

func loadExternalSource(path string) (map[string]Entry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".csv") {
		return loadExternalCSV(path, raw)
	}
	if entries, ok := loadExternalJSON(raw); ok {
		return entries, nil
	}
	return nil, fmt.Errorf("%s: unsupported sector source format", path)
}

func loadTradingViewSource(ctx context.Context, cfg config.Config, enabled bool) (map[string]Entry, error) {
	if !enabled {
		return map[string]Entry{}, nil
	}
	body, err := json.Marshal(map[string]any{
		"columns":               []string{"name", "description", "sector", "industry"},
		"ignore_unknown_fields": false,
		"options":               map[string]any{"lang": "tr"},
		"range":                 []int{0, 2000},
		"sort": map[string]any{
			"sortBy":     "name",
			"sortOrder":  "asc",
			"nullsFirst": false,
		},
		"preset": "all_stocks",
	})
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	if client.Timeout <= 0 {
		client.Timeout = 45 * time.Second
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://scanner.tradingview.com/turkey/scan", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "hissebot-go/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tradingview sector scan: %w", err)
	}
	data, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("tradingview sector scan: status %d: %s", resp.StatusCode, string(data))
	}
	var parsed tradingViewScanResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("tradingview sector scan decode: %w", err)
	}
	out := map[string]Entry{}
	for _, row := range parsed.Data {
		symbol := storage.NormalizeTicker(strings.TrimPrefix(row.S, "BIST:"))
		if symbol == "" && len(row.D) > 0 {
			symbol = storage.NormalizeTicker(stringValue(row.D[0]))
		}
		if symbol == "" || len(row.D) < 4 {
			continue
		}
		sectorEN := stringValue(row.D[2])
		industryEN := stringValue(row.D[3])
		if sectorEN == "" && industryEN == "" {
			continue
		}
		sector := tradingViewSectorTR(sectorEN)
		industry := tradingViewIndustryTR(industryEN)
		entry := completeEntry(Entry{
			Sector:     firstNonEmpty(sector, sectorEN),
			Industry:   firstNonEmpty(industry, industryEN, sector, sectorEN),
			Source:     "tradingview_sector_industry",
			Confidence: 0.88,
			Evidence: []string{
				"tradingview_scanner.sector=" + sectorEN,
				"tradingview_scanner.industry=" + industryEN,
			},
		})
		out[symbol] = entry
	}
	return out, nil
}

func loadExternalJSON(raw []byte) (map[string]Entry, bool) {
	var file File
	if json.Unmarshal(raw, &file) == nil && len(file.Entries) > 0 {
		return normalizeExternalEntryMap(file.Entries), true
	}
	var entries map[string]Entry
	if json.Unmarshal(raw, &entries) == nil && len(entries) > 0 {
		return normalizeExternalEntryMap(entries), true
	}
	var genericMap map[string]map[string]any
	if json.Unmarshal(raw, &genericMap) == nil && len(genericMap) > 0 {
		out := map[string]Entry{}
		for symbol, row := range genericMap {
			row["symbol"] = symbol
			if parsed, ok := externalRowEntry(row); ok {
				out[storage.NormalizeTicker(symbol)] = parsed
			}
		}
		if len(out) > 0 {
			return out, true
		}
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) == nil && len(rows) > 0 {
		out := map[string]Entry{}
		for _, row := range rows {
			symbol := externalRowSymbol(row)
			if symbol == "" {
				continue
			}
			if parsed, ok := externalRowEntry(row); ok {
				out[symbol] = parsed
			}
		}
		if len(out) > 0 {
			return out, true
		}
	}
	return nil, false
}

func loadExternalCSV(path string, raw []byte) (map[string]Entry, error) {
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(records) < 2 {
		return map[string]Entry{}, nil
	}
	headers := records[0]
	out := map[string]Entry{}
	for _, record := range records[1:] {
		row := map[string]any{}
		for i, header := range headers {
			if i < len(record) {
				row[header] = record[i]
			}
		}
		symbol := externalRowSymbol(row)
		if symbol == "" {
			continue
		}
		entry, ok := externalRowEntry(row)
		if ok {
			out[symbol] = entry
		}
	}
	return out, nil
}

func normalizeExternalEntryMap(entries map[string]Entry) map[string]Entry {
	out := map[string]Entry{}
	for symbol, entry := range entries {
		symbol = storage.NormalizeTicker(symbol)
		if symbol == "" {
			continue
		}
		entry = completeExternalEntry(entry)
		out[symbol] = entry
	}
	return out
}

func externalRowEntry(row map[string]any) (Entry, bool) {
	sector := firstNonEmpty(
		rowValue(row, "sector"),
		rowValue(row, "sektor"),
		rowValue(row, "ana_sektor"),
		rowValue(row, "main_sector"),
	)
	industry := firstNonEmpty(
		rowValue(row, "industry"),
		rowValue(row, "alt_sektor"),
		rowValue(row, "sub_sector"),
		rowValue(row, "faaliyet_alani"),
		rowValue(row, "faaliyet"),
		rowValue(row, "activity"),
		rowValue(row, "business"),
	)
	if sector == "" && industry == "" {
		return Entry{}, false
	}
	if sector == "" {
		if inferred, ok := classifyByKeywords(industry); ok {
			sector = inferred.Sector
			if inferred.Industry != "" && industry == "" {
				industry = inferred.Industry
			}
		}
	}
	source := firstNonEmpty(rowValue(row, "source"), rowValue(row, "provider"), "external_sector_source")
	confidence := floatValue(rowValue(row, "confidence"))
	entry := Entry{
		Sector:     sector,
		Industry:   firstNonEmpty(industry, sector),
		PeerGroup:  firstNonEmpty(rowValue(row, "peer_group"), rowValue(row, "peerGroup")),
		Source:     source,
		Confidence: confidence,
		Activity:   firstNonEmpty(rowValue(row, "activity_text"), rowValue(row, "faaliyet_metni")),
		Evidence:   []string{"external_sector_source_file"},
	}
	return completeExternalEntry(entry), true
}

func completeExternalEntry(entry Entry) Entry {
	entry = completeEntry(entry)
	if entry.Confidence <= 0 {
		entry.Confidence = 0.88
	}
	if entry.Source == "" || entry.Source == "classification_sync" {
		entry.Source = "external_sector_source"
	}
	return entry
}

func externalRowSymbol(row map[string]any) string {
	return storage.NormalizeTicker(firstNonEmpty(
		rowValue(row, "symbol"),
		rowValue(row, "ticker"),
		rowValue(row, "code"),
		rowValue(row, "stock_code"),
		rowValue(row, "stockCode"),
		rowValue(row, "hisse"),
		rowValue(row, "kod"),
	))
}

func rowValue(row map[string]any, key string) string {
	target := normalizeKey(key)
	for rowKey, value := range row {
		if normalizeKey(rowKey) == target {
			return stringValue(value)
		}
	}
	return ""
}

func floatValue(value string) float64 {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if value == "" {
		return 0
	}
	var out float64
	if _, err := fmt.Sscanf(value, "%f", &out); err != nil {
		return 0
	}
	return out
}

func classifyEquity(store *storage.EquityStore, equity *domain.Equity) classificationCandidate {
	symbol := storage.NormalizeTicker(equity.Ticker)
	name := firstNonEmpty(equity.Name, symbol)
	kap := readMap(store.KAPPath(symbol))
	if len(kap) == 0 && len(equity.KAPInfo) > 0 {
		kap = equity.KAPInfo
	}
	mkk := readMap(store.MKKCompanyInfoPath(symbol))
	if len(mkk) == 0 && len(equity.CompanyInfo) > 0 {
		mkk = equity.CompanyInfo
	}
	if title := stringValue(kap["kapMemberTitle"]); title != "" {
		name = firstNonEmpty(name, title)
	}

	if entry, ok := officialFromKAP(kap); ok {
		entry.Evidence = append(entry.Evidence, "kap.financialType="+stringValue(kap["financialType"]))
		return classificationCandidate{entry: completeEntry(entry), sourceKind: "official"}
	}

	activityText := extractActivityText(mkk)
	if activityText != "" {
		if entry, ok := classifyByKeywords(activityText); ok {
			entry.Source = "mkk_activity_text"
			entry.Confidence = 0.78
			entry.Activity = truncateText(activityText, 500)
			entry.Evidence = append(entry.Evidence, "mkk_company_info.activity_text")
			return classificationCandidate{entry: completeEntry(entry), sourceKind: "activity"}
		}
	}

	text := strings.Join([]string{name, stringValue(kap["kapMemberTitle"]), stringValue(kap["stockCode"])}, " ")
	if entry, ok := classifyByKeywords(text); ok {
		entry.Source = "kap_title_keyword"
		entry.Confidence = titleKeywordConfidence(entry)
		entry.Evidence = append(entry.Evidence, "kap_title_or_company_name_keyword")
		if entry.Confidence < 0.80 {
			entry.Warnings = appendUnique(entry.Warnings, "medium_classification_confidence")
		}
		return classificationCandidate{entry: completeEntry(entry), sourceKind: "keyword"}
	}

	return classificationCandidate{entry: completeEntry(Entry{
		Sector:     "BIST Genel",
		Industry:   "Sınıflandırılamadı",
		Source:     "classification_fallback",
		Confidence: 0.25,
		Warnings:   []string{"kap_mkk_activity_or_sector_missing"},
	}), sourceKind: "fallback"}
}

func officialFromKAP(kap map[string]any) (Entry, bool) {
	financialType := normalizeKey(stringValue(kap["financialType"]))
	switch financialType {
	case "gyo":
		return Entry{Sector: "Gayrimenkul Yatırım Ortaklığı", Industry: "Gayrimenkul Yatırım Ortaklığı", Source: "kap_financial_type", Confidence: 0.92}, true
	case "bnk", "banka":
		return Entry{Sector: "Banka", Industry: "Bankacılık", Source: "kap_financial_type", Confidence: 0.92}, true
	case "sgr", "sigorta":
		return Entry{Sector: "Sigorta ve Emeklilik", Industry: "Sigorta", Source: "kap_financial_type", Confidence: 0.92}, true
	}
	return Entry{}, false
}

func classifyByKeywords(text string) (Entry, bool) {
	key := normalizeKey(text)
	if key == "" {
		return Entry{}, false
	}
	for _, rule := range industryRules {
		for _, keyword := range rule.keywords {
			if strings.Contains(key, normalizeKey(keyword)) {
				return Entry{Sector: rule.sector, Industry: rule.industry}, true
			}
		}
	}
	return Entry{}, false
}

func titleKeywordConfidence(entry Entry) float64 {
	switch entry.Industry {
	case "Bankacılık",
		"Sigorta",
		"Faktoring",
		"Finansal Kiralama",
		"Varlık Kiralama",
		"Aracı Kurum ve Yatırım Hizmetleri",
		"Gayrimenkul Yatırım Ortaklığı":
		return 0.82
	case "Holding":
		return 0.80
	default:
		return 0.62
	}
}

func completeEntry(entry Entry) Entry {
	if entry.Sector == "" {
		entry.Sector = "BIST Genel"
	}
	if entry.Industry == "" {
		entry.Industry = entry.Sector
	}
	if entry.PeerGroup == "" {
		entry.PeerGroup = peerGroup(entry.Sector, entry.Industry)
	}
	if entry.Source == "" {
		entry.Source = "classification_sync"
	}
	return entry
}

func preserveExisting(existing Entry, generated Entry) (Entry, bool) {
	if existing.Sector == "" && existing.Industry == "" {
		return Entry{}, false
	}
	if existing.Source == generated.Source && existing.PeerGroup == generated.PeerGroup {
		return generated, false
	}
	if existing.Confidence < generated.Confidence {
		return generated, false
	}
	existing = completeEntry(existing)
	existing.Evidence = appendUnique(existing.Evidence, "preserved_existing_classification")
	if existing.Activity == "" {
		existing.Activity = generated.Activity
	}
	return existing, true
}

func peersFor(symbol string, group []string, secondary []string, existing []string, minPeers int, maxPeers int) []string {
	seen := map[string]bool{symbol: true}
	out := []string{}
	for _, peer := range existing {
		peer = storage.NormalizeTicker(peer)
		if peer == "" || seen[peer] {
			continue
		}
		seen[peer] = true
		out = append(out, peer)
		if len(out) >= maxPeers {
			return out
		}
	}
	for _, peer := range group {
		peer = storage.NormalizeTicker(peer)
		if peer == "" || seen[peer] {
			continue
		}
		seen[peer] = true
		out = append(out, peer)
		if len(out) >= maxPeers {
			return out
		}
	}
	if len(out) >= minPeers {
		return out
	}
	for _, peer := range secondary {
		peer = storage.NormalizeTicker(peer)
		if peer == "" || seen[peer] {
			continue
		}
		seen[peer] = true
		out = append(out, peer)
		if len(out) >= maxPeers || len(out) >= minPeers {
			return out
		}
	}
	return out
}

func shouldReuseExistingPeers(previous Entry, current Entry) bool {
	if len(previous.PeerSymbols) == 0 {
		return false
	}
	if previous.Source != current.Source || previous.PeerGroup != current.PeerGroup {
		return false
	}
	return strings.Contains(previous.Source, "manual")
}

func readMap(path string) map[string]any {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	if out["available"] == false {
		return nil
	}
	return out
}

func extractActivityText(payload map[string]any) string {
	values := []string{}
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				walk(joinPath(prefix, key), child)
			}
		case []any:
			for _, child := range v {
				walk(prefix, child)
			}
		case string:
			if relevantActivityKey(prefix) && strings.TrimSpace(v) != "" {
				values = append(values, strings.TrimSpace(v))
			}
		}
	}
	walk("", payload)
	return strings.Join(uniqueStrings(values), " ")
}

func relevantActivityKey(key string) bool {
	key = normalizeKey(key)
	for _, token := range []string{"faaliyet", "activity", "business", "sector", "industry", "nace", "purpose", "konu", "alan"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return false
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func peerGroup(sector, industry string) string {
	base := industry
	if strings.TrimSpace(base) == "" {
		base = sector
	}
	base = util.SlugTR(base)
	if base == "" {
		base = "bist_genel"
	}
	return "bist_" + base
}

func normalizeKey(value string) string {
	return util.SlugTR(value)
}

func tradingViewSectorTR(value string) string {
	if translated := tradingViewSectorTranslations[strings.TrimSpace(value)]; translated != "" {
		return translated
	}
	return strings.TrimSpace(value)
}

func tradingViewIndustryTR(value string) string {
	if translated := tradingViewIndustryTranslations[strings.TrimSpace(value)]; translated != "" {
		return translated
	}
	return strings.TrimSpace(value)
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		return fmt.Sprintf("%g", v)
	case float32:
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func appendUnique(values []string, next string) []string {
	if next == "" {
		return values
	}
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit]))
}

func sourceKind(source string) string {
	switch {
	case strings.Contains(source, kapsectors.SourceName):
		return "kap_sector"
	case strings.Contains(source, "kap_financial_type"):
		return "official"
	case strings.Contains(source, "mkk_activity"):
		return "activity"
	case strings.Contains(source, "tradingview"):
		return "tradingview"
	case strings.Contains(source, "keyword"), strings.Contains(source, "manual"):
		return "keyword"
	default:
		return "fallback"
	}
}
