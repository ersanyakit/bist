package kapingest

import (
	"os"
	"path/filepath"
	"testing"

	"hissebot/internal/services/kapsectors"
)

func TestSectorContextLookupFallbackAndPromptSelection(t *testing.T) {
	store := SectorContextStore{
		Symbols: map[string]CompanySectorContext{
			"ALGYO": companySectorContextFromEntry(kapsectors.CompanySector{
				Symbol:     "ALGYO",
				Title:      "Alarko GYO",
				MainSector: "MALİ KURULUŞLAR",
				Sector:     "GAYRİMENKUL YATIRIM ORTAKLIKLARI",
				KAPTypes:   []string{"IGS"},
				AllSectors: []kapsectors.SectorMembership{{Level: "normal", MainSector: "MALİ KURULUŞLAR", Sector: "GAYRİMENKUL YATIRIM ORTAKLIKLARI"}},
			}, kapsectors.SourceName),
		},
		PromptPack: SectorPromptPack{SectorPrompts: map[string]SectorPromptSpec{
			normalizeSectorKey("GAYRİMENKUL YATIRIM ORTAKLIKLARI"): {Prompt: "GYO prompt", Schema: map[string]any{"asset_inventory": true}},
			normalizeSectorKey(DefaultSectorFallback):              {Prompt: "Fallback prompt"},
		}},
		PromptLoaded: true,
	}
	company := store.Lookup("algyo")
	if company.Sector != "GAYRİMENKUL YATIRIM ORTAKLIKLARI" || len(company.KAPTypes) != 1 || len(company.AllSectors) != 1 {
		t.Fatalf("unexpected company sector: %+v", company)
	}
	selected := store.SelectPrompt(company)
	if selected.Prompt != "GYO prompt" || selected.Schema["asset_inventory"] != true {
		t.Fatalf("unexpected prompt selection: %+v", selected)
	}
	fallback := store.Lookup("XXXX")
	if !fallback.Fallback || fallback.Sector != DefaultSectorFallback {
		t.Fatalf("expected DİĞER fallback, got %+v", fallback)
	}
	if got := store.SelectPrompt(fallback); got.Prompt != "Fallback prompt" {
		t.Fatalf("expected fallback prompt, got %+v", got)
	}
}

func TestLoadSectorPromptPackSupportsStringAndObjectPrompts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kap_sector_prompt_pack.json")
	if err := os.WriteFile(path, []byte(`{
		"sector_count": 2,
		"main_sectors": ["MALİ KURULUŞLAR"],
		"common_extraction_focus": ["borç", "nakit"],
		"sector_prompts": {
			"BANKALAR": "Banka prompt",
			"DİĞER": {"prompt": "Generic prompt", "schema": {"review_required": true}, "extraction_focus": ["faaliyet konusu"]}
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := LoadSectorPromptPack(path)
	if err != nil {
		t.Fatalf("LoadSectorPromptPack error = %v", err)
	}
	if pack.SectorPrompts[normalizeSectorKey("BANKALAR")].Prompt != "Banka prompt" {
		t.Fatalf("string prompt not decoded: %+v", pack.SectorPrompts)
	}
	other := pack.SectorPrompts[normalizeSectorKey(DefaultSectorFallback)]
	if other.Prompt != "Generic prompt" || other.Schema["review_required"] != true || len(other.ExtractionFocus) != 1 {
		t.Fatalf("object prompt not decoded: %+v", other)
	}
}

func TestBuildExtractionRouteUsesSectorBusinessModelAndDocumentType(t *testing.T) {
	store := SectorContextStore{
		Symbols: map[string]CompanySectorContext{
			"ALGYO": {
				Symbol:     "ALGYO",
				MainSector: "MALİ KURULUŞLAR",
				Sector:     "GAYRİMENKUL YATIRIM ORTAKLIKLARI",
				Source:     kapsectors.SourceName,
			},
		},
		PromptPack: SectorPromptPack{SectorPrompts: map[string]SectorPromptSpec{
			normalizeSectorKey("GAYRİMENKUL YATIRIM ORTAKLIKLARI"): {Prompt: "GYO prompt"},
		}},
	}
	doc := RawDocument{
		Ticker:            "ALGYO",
		FileName:          "ALGYO Finansal Rapor.pdf",
		DocumentTypeGuess: DocumentFinancialStatement,
		Text:              "Gayrimenkul yatırım ortaklığı finansal durum tablosu yatırım amaçlı gayrimenkuller Hasılat 100 Özkaynaklar 50",
		TextLength:        120,
		QualityScore:      0.92,
	}
	route := BuildExtractionRoute(doc, store)
	if route.DocumentType != DocumentFinancialStatement || !route.FinancialTableParser || !route.AssetInventoryExtractor {
		t.Fatalf("unexpected route: %+v", route)
	}
	if len(route.BusinessModels) == 0 || route.BusinessModels[0].Tag != "reit" {
		t.Fatalf("expected reit business model, got %+v", route.BusinessModels)
	}
	if route.PromptSelection.Prompt != "GYO prompt" {
		t.Fatalf("prompt not selected: %+v", route.PromptSelection)
	}
}

func TestClassifyBusinessModelsFiltersIncompatibleDefenseTags(t *testing.T) {
	company := CompanySectorContext{
		Symbol:     "ASELS",
		MainSector: "İMALAT",
		Sector:     "SAVUNMA",
	}
	text := "Savunma sistemleri ar-ge mühendislik gıda ilaç tekstil güvenlik faaliyetleri"
	tags := ClassifyBusinessModels(company, text)
	seen := map[string]bool{}
	for _, tag := range tags {
		seen[tag.Tag] = true
	}
	for _, want := range []string{"defense", "r_and_d", "engineering_architecture"} {
		if !seen[want] {
			t.Fatalf("expected defense-compatible tag %q in %+v", want, tags)
		}
	}
	for _, banned := range []string{"food_beverage", "pharmaceuticals", "textile_apparel", "security_services"} {
		if seen[banned] {
			t.Fatalf("incompatible defense business model %q leaked in %+v", banned, tags)
		}
	}
}
