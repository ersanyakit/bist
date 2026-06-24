package classification

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/services/kapsectors"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

func TestSyncBuildsSectorClassificationsAndPeerGroups(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:     dir,
		SeedDir:     filepath.Join(dir, "seed"),
		EquitiesDir: filepath.Join(dir, "equities"),
	}
	store := storage.NewEquityStore(cfg.EquitiesDir)

	mustSave(t, store, &domain.Equity{
		Ticker: "ASELS",
		Name:   "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.",
		KAPInfo: map[string]any{
			"financialType":  "SIR",
			"kapMemberTitle": "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.",
		},
	})
	mustSave(t, store, &domain.Equity{
		Ticker: "SDTTR",
		Name:   "SDT UZAY VE SAVUNMA TEKNOLOJILERI A.S.",
		KAPInfo: map[string]any{
			"financialType":  "SIR",
			"kapMemberTitle": "SDT UZAY VE SAVUNMA TEKNOLOJILERI A.S.",
		},
	})
	mustSave(t, store, &domain.Equity{
		Ticker: "EKGYO",
		Name:   "EMLAK KONUT GAYRIMENKUL YATIRIM ORTAKLIGI A.S.",
		KAPInfo: map[string]any{
			"financialType":  "GYO",
			"kapMemberTitle": "EMLAK KONUT GAYRIMENKUL YATIRIM ORTAKLIGI A.S.",
		},
	})

	outPath := filepath.Join(cfg.SeedDir, "sector_classifications.json")
	if err := util.WriteJSON(outPath, File{Entries: map[string]Entry{
		"ASELS": {
			Sector:     "Savunma ve Elektronik",
			Industry:   "Savunma Elektroniği",
			PeerGroup:  "bist_savunma_elektronigi",
			Source:     "manual_peer_universe",
			Confidence: 0.85,
		},
	}}); err != nil {
		t.Fatalf("write existing classification: %v", err)
	}

	result, err := Sync(context.Background(), cfg, store, Options{OutputPath: outPath, PreserveExisting: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Entries != 3 {
		t.Fatalf("entries = %d, want 3", result.Entries)
	}

	var got File
	if err := util.ReadJSON(outPath, &got); err != nil {
		t.Fatalf("read output: %v", err)
	}
	if got.Entries["EKGYO"].Sector != "Gayrimenkul Yatırım Ortaklığı" {
		t.Fatalf("EKGYO sector = %q", got.Entries["EKGYO"].Sector)
	}
	if got.Entries["EKGYO"].Confidence < 0.90 {
		t.Fatalf("EKGYO confidence = %.2f, want official high confidence", got.Entries["EKGYO"].Confidence)
	}
	if got.Entries["ASELS"].Source != "manual_peer_universe" {
		t.Fatalf("ASELS source = %q, want preserved manual source", got.Entries["ASELS"].Source)
	}
	if got.Entries["ASELS"].PeerGroup != "bist_savunma_elektronigi" {
		t.Fatalf("ASELS peer_group = %q", got.Entries["ASELS"].PeerGroup)
	}
}

func TestMKKActivityTextClassifiesBeforeTitleFallback(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:     dir,
		SeedDir:     filepath.Join(dir, "seed"),
		EquitiesDir: filepath.Join(dir, "equities"),
	}
	store := storage.NewEquityStore(cfg.EquitiesDir)
	mustSave(t, store, &domain.Equity{
		Ticker: "SOFT",
		Name:   "ORNEK TICARET A.S.",
		KAPInfo: map[string]any{
			"financialType":  "SIR",
			"kapMemberTitle": "ORNEK TICARET A.S.",
		},
	})
	if err := util.WriteJSON(store.MKKCompanyInfoPath("SOFT"), map[string]any{
		"faaliyetAlani": "Yazılım, bilişim ve teknoloji çözümleri geliştirmek",
	}); err != nil {
		t.Fatalf("write mkk info: %v", err)
	}

	outPath := filepath.Join(cfg.SeedDir, "sector_classifications.json")
	if _, err := Sync(context.Background(), cfg, store, Options{OutputPath: outPath}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	var got File
	if err := util.ReadJSON(outPath, &got); err != nil {
		t.Fatalf("read output: %v", err)
	}
	entry := got.Entries["SOFT"]
	if entry.Sector != "Teknoloji ve Elektronik" {
		t.Fatalf("sector = %q, want technology", entry.Sector)
	}
	if entry.Source != "mkk_activity_text" {
		t.Fatalf("source = %q, want mkk_activity_text", entry.Source)
	}
	if entry.Confidence < 0.75 {
		t.Fatalf("confidence = %.2f, want activity confidence", entry.Confidence)
	}
}

func TestExternalSourceOverridesLowConfidenceKeywordClassification(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:     dir,
		SeedDir:     filepath.Join(dir, "seed"),
		EquitiesDir: filepath.Join(dir, "equities"),
	}
	store := storage.NewEquityStore(cfg.EquitiesDir)
	mustSave(t, store, &domain.Equity{
		Ticker: "ASELS",
		Name:   "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.",
		KAPInfo: map[string]any{
			"financialType":  "SIR",
			"kapMemberTitle": "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.",
		},
	})
	sourcePath := filepath.Join(dir, "tv_sectors.csv")
	if err := os.WriteFile(sourcePath, []byte("symbol,sector,industry,source,confidence\nASELS,Elektronik Teknoloji,Uzay ve Savunma,tradingview_sector_industry,0.88\n"), 0o644); err != nil {
		t.Fatalf("write source csv: %v", err)
	}

	outPath := filepath.Join(cfg.SeedDir, "sector_classifications.json")
	result, err := Sync(context.Background(), cfg, store, Options{OutputPath: outPath, SourceFiles: []string{sourcePath}})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.ExternalClassified != 1 {
		t.Fatalf("ExternalClassified = %d, want 1", result.ExternalClassified)
	}
	var got File
	if err := util.ReadJSON(outPath, &got); err != nil {
		t.Fatalf("read output: %v", err)
	}
	entry := got.Entries["ASELS"]
	if entry.Industry != "Uzay ve Savunma" {
		t.Fatalf("industry = %q, want external industry", entry.Industry)
	}
	if entry.Confidence != 0.88 {
		t.Fatalf("confidence = %.2f, want 0.88", entry.Confidence)
	}
}

func TestKAPSectorSourceOverridesExternalClassification(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:     dir,
		SeedDir:     filepath.Join(dir, "seed"),
		EquitiesDir: filepath.Join(dir, "equities"),
	}
	store := storage.NewEquityStore(cfg.EquitiesDir)
	mustSave(t, store, &domain.Equity{
		Ticker: "ASELS",
		Name:   "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.",
		KAPInfo: map[string]any{
			"financialType":  "SIR",
			"kapMemberTitle": "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.",
		},
	})

	kapPath := filepath.Join(cfg.SeedDir, "kap_sectors.json")
	if err := util.WriteJSON(kapPath, kapsectors.File{
		Source:    kapsectors.SourceName,
		SourceURL: "https://kap.org.tr/tr/Sektorler",
		Entries: map[string]kapsectors.CompanySector{
			"ASELS": {
				Symbol:     "ASELS",
				Title:      "ASELSAN ELEKTRONİK SANAYİ VE TİCARET A.Ş.",
				MainSector: "TEKNOLOJİ",
				Sector:     "SAVUNMA",
				SectorNo:   "011000.002000.",
			},
		},
	}); err != nil {
		t.Fatalf("write kap sectors: %v", err)
	}
	sourcePath := filepath.Join(dir, "external.csv")
	if err := os.WriteFile(sourcePath, []byte("symbol,sector,industry,source,confidence\nASELS,Yanlis Sektor,Yanlis Endustri,external_high_confidence,1.0\n"), 0o644); err != nil {
		t.Fatalf("write source csv: %v", err)
	}

	outPath := filepath.Join(cfg.SeedDir, "sector_classifications.json")
	result, err := Sync(context.Background(), cfg, store, Options{
		OutputPath:       outPath,
		SourceFiles:      []string{sourcePath},
		KAPSectorsFile:   kapPath,
		UseTradingView:   false,
		PreserveExisting: true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.KAPSectorClassified != 1 {
		t.Fatalf("KAPSectorClassified = %d, want 1", result.KAPSectorClassified)
	}
	var got File
	if err := util.ReadJSON(outPath, &got); err != nil {
		t.Fatalf("read output: %v", err)
	}
	entry := got.Entries["ASELS"]
	if entry.Source != kapsectors.SourceName {
		t.Fatalf("source = %q, want %q", entry.Source, kapsectors.SourceName)
	}
	if entry.Sector != "TEKNOLOJİ" || entry.Industry != "SAVUNMA" {
		t.Fatalf("entry = %+v, want KAP sector/industry", entry)
	}
}

func TestTradingViewTranslations(t *testing.T) {
	if got := tradingViewSectorTR("Electronic Technology"); got != "Elektronik Teknoloji" {
		t.Fatalf("sector translation = %q", got)
	}
	if got := tradingViewIndustryTR("Aerospace & Defense"); got != "Uzay ve Savunma" {
		t.Fatalf("industry translation = %q", got)
	}
}

func mustSave(t *testing.T, store *storage.EquityStore, equity *domain.Equity) {
	t.Helper()
	if err := store.Save(equity); err != nil {
		t.Fatalf("save %s: %v", equity.Ticker, err)
	}
	if len(equity.KAPInfo) > 0 {
		if err := util.WriteJSON(store.KAPPath(equity.Ticker), equity.KAPInfo); err != nil {
			t.Fatalf("write kap %s: %v", equity.Ticker, err)
		}
	}
}
