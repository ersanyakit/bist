package kapingest

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractAssetEventsFromGYOPortfolioText(t *testing.T) {
	doc := RawDocument{
		FilePath:          "data/equities/ALGYO/kap/attachments/2026/1/test.pdf",
		SHA256:            "sha-asset-1",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 31.12.2025 GYO Portfoy Tablosu.pdf",
		DocumentTypeGuess: DocumentValuationReport,
		QualityScore:      0.95,
		Text: `ALARKO GAYRİMENKUL YATIRIM ORTAKLIĞI ANONİM ŞİRKETİ
Portföyde Yer Alan Varlıklar
Varlık Adı        Lokasyon        Alan m²        Ada/Parsel        Ekspertiz Tarihi        Ekspertiz Değeri KDV Hariç TL        Ekspertiz Değeri KDV Dahil TL        Aylık Kira Bedeli TL        Yıllık Asgari Kira USD        Kiracı
Hillside Beach Club Otel        Muğla / Fethiye        118.000 m²        123 ada 45 parsel        31.12.2025        4.500.000.000 TL        5.400.000.000 TL        12.500.000 TL        3.200.000 USD        Hillside A.Ş.
TOPLAM        4.500.000.000 TL        5.400.000.000 TL
`,
	}
	events := ExtractAssetEvents(doc)
	if len(events) != 1 {
		t.Fatalf("expected one asset event and no TOPLAM asset, got %d: %+v", len(events), events)
	}
	event := events[0]
	if event.Ticker != "ALGYO" || event.AssetType != "hotel" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
	if event.City != "Mugla" && event.City != "Muğla" {
		t.Fatalf("expected city from row, got %+v", event)
	}
	if event.AreaM2 == nil || *event.AreaM2 != 118000 {
		t.Fatalf("expected area 118000, got %+v", event.AreaM2)
	}
	if event.ParcelInfo == "" {
		t.Fatalf("expected parcel info: %+v", event)
	}
	if event.ExpertiseValueExclVATTRY == nil || *event.ExpertiseValueExclVATTRY != 4500000000 {
		t.Fatalf("expected KDV excl value, got %+v", event.ExpertiseValueExclVATTRY)
	}
	if event.ExpertiseValueInclVATTRY == nil || *event.ExpertiseValueInclVATTRY != 5400000000 {
		t.Fatalf("expected KDV incl value, got %+v", event.ExpertiseValueInclVATTRY)
	}
	if event.RentalInfo.MonthlyRentTRY == nil || *event.RentalInfo.MonthlyRentTRY != 12500000 {
		t.Fatalf("expected monthly rent, got %+v", event.RentalInfo)
	}
	if event.RentalInfo.AnnualMinRentUSD == nil || *event.RentalInfo.AnnualMinRentUSD != 3200000 {
		t.Fatalf("expected annual min USD rent, got %+v", event.RentalInfo)
	}
	if event.Confidence < 0.75 {
		t.Fatalf("expected high confidence, got %.2f: %+v", event.Confidence, event)
	}
	snapshots, notes := ExtractPortfolioSummaryAndNotes(doc)
	if len(notes) != 0 {
		t.Fatalf("did not expect valuation notes, got %+v", notes)
	}
	if len(snapshots) == 0 {
		t.Fatalf("expected portfolio summary snapshot")
	}
}

func TestKAPMoneyParserSupportsTurkishAndInternationalSeparators(t *testing.T) {
	cases := map[string]float64{
		"975.000":       975000,
		"1.234.567,89":  1234567.89,
		"1,234,567.89":  1234567.89,
		"12.500.000 TL": 12500000,
		"3,200,000 USD": 3200000,
		"1,25":          1.25,
		"1500000":       1500000,
	}
	for input, want := range cases {
		got, ok := parseTurkishNumber(input)
		if !ok {
			t.Fatalf("%q did not parse", input)
		}
		if got != want {
			t.Fatalf("%q parsed as %.4f, want %.4f", input, got, want)
		}
	}
}

func TestExtractMoneyAmountsDoesNotTruncateInternationalThousands(t *testing.T) {
	amounts := extractMoneyAmounts("Ekspertiz değeri TRY 1,234,567.89 ve aylık kira 12.500,75 TL olarak açıklanmıştır.")
	if len(amounts) != 2 {
		t.Fatalf("expected two amounts, got %+v", amounts)
	}
	if amounts[0].Currency != "TRY" || amounts[0].Value != 1234567.89 {
		t.Fatalf("international amount parsed incorrectly: %+v", amounts[0])
	}
	if amounts[1].Currency != "TRY" || amounts[1].Value != 12500.75 {
		t.Fatalf("Turkish amount parsed incorrectly: %+v", amounts[1])
	}
}

func TestExtractAssetEventsAIResolvesCoordinateTable(t *testing.T) {
	doc := RawDocument{
		FilePath:          "data/equities/ALGYO/kap/attachments/2026/1/scan.pdf",
		SHA256:            "sha-asset-rescue",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 31.12.2025 GYO Portfoy Tablosu.pdf",
		ExtractionMethod:  "pdftotext+tsv",
		DocumentTypeGuess: DocumentValuationReport,
		QualityScore:      0.55,
		TextLength:        220,
		Warnings:          []string{"coordinate_tsv_text_appended"},
		Text: `bozuk OCR metni
###COORDINATE_TABLE_TEXT###
Hillside Beach Club Otel	Muğla / Fethiye	118.000 m2	123 ada 45 parsel	31.12.2025	4.500.000.000 TL`,
	}
	events := ExtractAssetEvents(doc)
	if len(events) != 1 {
		t.Fatalf("expected rescued asset event, got %d: %+v", len(events), events)
	}
	if !containsRawWarning(events[0].RiskFlags, lowQualityStructuredRescueWarning) {
		t.Fatalf("expected structured rescue risk flag: %+v", events[0])
	}
	if !containsRawWarning(events[0].RiskFlags, structuredRescueAIResolvedWarning) {
		t.Fatalf("expected AI-resolved risk flag: %+v", events[0])
	}
	if events[0].Confidence > 0.82 {
		t.Fatalf("AI-resolved asset confidence should be capped, got %+v", events[0])
	}
}

func TestExtractAssetEventsRejectsTotalsAndExplanationRows(t *testing.T) {
	doc := RawDocument{
		FilePath:          "data/equities/ALGYO/kap/attachments/2025/1/noise.pdf",
		SHA256:            "sha-noise",
		Ticker:            "ALGYO",
		FileName:          "ALGYO degerleme.pdf",
		DocumentTypeGuess: DocumentValuationReport,
		QualityScore:      0.95,
		Text: `Portföyde Yer Alan Varlıklar
Varlık Adı        Lokasyon        Alan m²        Ekspertiz Tarihi        Ekspertiz Değeri KDV Hariç TL
TOPLAM        4.500.000.000 TL        5.400.000.000 TL
OTEL GİDERLER ( TL)        307.569.555        548.836.698
Ort Yatak Fiyatı (EUR) (KDV hariç) 240-260 275-300 Yaklaşık Toplam Bugünkü Değer (TL) 2.594.755.000
* Çalışmalara IVSC kapsamında vergi ve KDV dahil edilmemiştir. Esaslı Yatırım 278.324.189
Projenin Tamamlanmış Değeri TL 3.633.210.000
`,
	}
	events := ExtractAssetEvents(doc)
	if len(events) != 0 {
		t.Fatalf("expected noisy total/explanation rows to be rejected, got %+v", events)
	}
	snapshots, _ := ExtractPortfolioSummaryAndNotes(doc)
	if len(snapshots) == 0 {
		t.Fatalf("expected TOPLAM row to be kept in portfolio summary")
	}
}

func TestExtractAssetEventsRejectsNarrativeAndTableFragments(t *testing.T) {
	doc := RawDocument{
		FilePath:          "data/equities/ALGYO/kap/attachments/2025/1/fragments.pdf",
		SHA256:            "sha-fragments",
		Ticker:            "ALGYO",
		FileName:          "ALGYO degerleme.pdf",
		DocumentTypeGuess: DocumentValuationReport,
		QualityScore:      0.95,
		Text: `Portföyde Yer Alan Varlıklar
Varlık Adı        Lokasyon        Alan m²        Ada/Parsel        Ekspertiz Tarihi        Ekspertiz Değeri KDV Hariç TL
ARSA DEĞERĠ=        10.000 m²        1.000.000 TL
Arsa Alanı        606.462,64 m²        1.000.000 TL
Gayrimenkulün bütün hukuki ve yasal prosedürlerini tamamladığı varsayılmıştır.
Net defter değeri 2.184.129 TL olan maddi duran varlıklar, Bodrum Otel alımı sırasında edinilmiş
Kullanılan bu yöntem 384 ada, 11 parselin arsa birim satış değeri 2.717-TL/m2+KDV olarak hesaplanmıştır. KDV HARİÇ TOPLAM DEĞER= 51.528.000 TL
HASILAT PAYLAŞIMI YÖNTEMİNE GÖRE 1 ADET ARSANIN DEĞERİ Toplam Satış Hasılatı 104.562.102 TL Hasılat Payı Oranı 35%
Müşterinin Talebinin Kapsamı ve Varsa Getirilen Sınırlamalar
Portföyümüzdeki arsalar
Ankara Çankaya İş Merkezi (Kira-Aylık)
Etiler Alkent Sitesi – Dükkanlar 28 Aralık 2017 28.240.000 Büyükçekmece Alkent 2000 – Dükkanlar 28 Aralık 2017 9.740.000 Eyüp Topçular – Fabrika 28 Aralık 2017 64.910.000
Bodrum Hillside Otel        Muğla / Bodrum        41.830 m²        363 ada 8 parsel        31.12.2025        Ekspertiz Değeri KDV Hariç 7.873.542.000 TL
`,
	}
	events := ExtractAssetEvents(doc)
	if len(events) != 1 {
		t.Fatalf("expected only the real asset row, got %d: %+v", len(events), events)
	}
	if events[0].AssetName != "Bodrum Hillside Otel" {
		t.Fatalf("unexpected asset event: %+v", events[0])
	}
}

func TestExtractAssetEventsBlocksRejectedParse(t *testing.T) {
	doc := RawDocument{
		FilePath:          "data/equities/ALGYO/kap/attachments/2026/1/scan.pdf",
		SHA256:            "sha-asset-rejected",
		Ticker:            "ALGYO",
		FileName:          "ALGYO 31.12.2025 GYO Portfoy Tablosu.pdf",
		DocumentTypeGuess: DocumentValuationReport,
		QualityScore:      0.05,
		TextLength:        80,
		Warnings:          []string{"low_text_quality_possible_scanned_pdf"},
		Text: `Portföyde Yer Alan Varlıklar
Hillside Beach Club Otel        Muğla / Fethiye        118.000 m²        4.500.000.000 TL`,
	}

	if events := ExtractAssetEvents(doc); len(events) != 0 {
		t.Fatalf("rejected parse produced asset events: %+v", events)
	}
	if snapshots, notes := ExtractPortfolioSummaryAndNotes(doc); len(snapshots) != 0 || len(notes) != 0 {
		t.Fatalf("rejected parse produced portfolio evidence: snapshots=%+v notes=%+v", snapshots, notes)
	}
}

func TestExtractMoneyAmountsDoesNotAttachPercentColumn(t *testing.T) {
	amounts := extractMoneyAmounts("TOPLAM PORTFÖY DEĞERİ 257.378.090 100,0%")
	if len(amounts) == 0 {
		t.Fatalf("expected amount")
	}
	if amounts[0].Value != 257378090 {
		t.Fatalf("expected amount without percent column, got %+v", amounts[0])
	}
}

func TestExtractAssetsFromRawDocumentsWritesEventsAndInventory(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, RawDocumentsFile)
	docs := []RawDocument{
		{
			FilePath:          "data/equities/ALGYO/kap/attachments/2025/1/a.pdf",
			SHA256:            "sha-a",
			Ticker:            "ALGYO",
			FileName:          "ALGYO 31.12.2024 portfoy.pdf",
			DocumentTypeGuess: DocumentValuationReport,
			QualityScore:      0.9,
			Text: `Portföyde Yer Alan Varlıklar
Hillside Beach Club Otel        Muğla / Fethiye        118.000 m²        123 ada 45 parsel        31.12.2024        Ekspertiz Değeri KDV Hariç 4.000.000.000 TL        Aylık Kira Bedeli 10.000.000 TL`,
		},
		{
			FilePath:          "data/equities/ALGYO/kap/attachments/2026/1/b.pdf",
			SHA256:            "sha-b",
			Ticker:            "ALGYO",
			FileName:          "ALGYO 31.12.2025 portfoy.pdf",
			DocumentTypeGuess: DocumentValuationReport,
			QualityScore:      0.9,
			Text: `Portföyde Yer Alan Varlıklar
Hillside Beach Club Otel        Muğla / Fethiye        118.000 m²        123 ada 45 parsel        31.12.2025        Ekspertiz Değeri KDV Hariç 4.500.000.000 TL        Aylık Kira Bedeli 12.500.000 TL
TOPLAM PORTFÖY DEĞERİ KDV HARİÇ 4.500.000.000 TL`,
		},
	}
	writeRawDocumentsForAssetTest(t, rawPath, docs)

	summary, err := ExtractAssetsFromRawDocuments(context.Background(), AssetExtractionOptions{
		RawDocumentsPath: rawPath,
		OutputDir:        dir,
		Now: func() time.Time {
			return time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("ExtractAssetsFromRawDocuments error = %v", err)
	}
	if summary.AssetEvents != 2 || summary.AssetInventories != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if count := countJSONLLines(t, filepath.Join(dir, AssetEventsFile)); count != 2 {
		t.Fatalf("global asset_events lines = %d", count)
	}
	byTickerEvents := filepath.Join(dir, "by_ticker", "ALGYO", AssetEventsFile)
	if count := countJSONLLines(t, byTickerEvents); count != 2 {
		t.Fatalf("by ticker asset_events lines = %d", count)
	}
	inventoryPath := filepath.Join(dir, "by_ticker", "ALGYO", AssetInventoryFile)
	raw, err := os.ReadFile(inventoryPath)
	if err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	var inventory AssetInventory
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatalf("parse inventory: %v", err)
	}
	if inventory.AssetCount != 1 || len(inventory.Assets) != 1 {
		t.Fatalf("expected duplicate events to merge into one asset: %+v", inventory)
	}
	if len(inventory.Assets[0].History) != 2 {
		t.Fatalf("expected two history points: %+v", inventory.Assets[0])
	}
	if inventory.GYOSummary.TotalRealEstateValueExclVATTRY == nil || *inventory.GYOSummary.TotalRealEstateValueExclVATTRY != 4500000000 {
		t.Fatalf("expected latest excl VAT total, got %+v", inventory.GYOSummary)
	}
	if inventory.PortfolioSummary.TotalRealEstateValueExclVATTRY == nil || *inventory.PortfolioSummary.TotalRealEstateValueExclVATTRY != 4500000000 {
		t.Fatalf("expected portfolio summary total, got %+v", inventory.PortfolioSummary)
	}
}

func TestBuildAssetInventoryFiltersNarrativeRows(t *testing.T) {
	area := 265.0
	inventory := BuildAssetInventory("ALGYO", []AssetEvent{
		{
			Ticker:                   "ALGYO",
			AssetName:                "Bodrum Hillside Otel",
			AssetType:                "hotel",
			Location:                 "Muğla / Bodrum",
			AreaM2:                   floatPtr(41830),
			ExpertiseValueExclVATTRY: floatPtr(7873542000),
			Confidence:               0.91,
		},
		{
			Ticker:                   "ALGYO",
			AssetName:                "F1-01",
			AssetType:                "shop",
			Location:                 "Beşiktaş / İstanbul, brüt 265 m², Etiler Alkent Sitesi'nde dükkan",
			AreaM2:                   &area,
			ExpertiseValueExclVATTRY: floatPtr(1500000),
			Confidence:               0.93,
		},
		{
			Ticker:                   "ALGYO",
			AssetName:                "Alarko Turistik da kirası bu dükkanlardan Fethiye alınmaktadır.",
			AssetType:                "shop",
			ExpertiseValueExclVATTRY: floatPtr(1500000),
			Confidence:               0.86,
		},
		{
			Ticker:                   "ALGYO",
			AssetName:                "FABRİKA BİNASI 1",
			AssetType:                "factory",
			ExpertiseValueExclVATTRY: floatPtr(4000),
			Confidence:               0.83,
		},
		{
			Ticker:     "ALGYO",
			AssetName:  "F1-02",
			AssetType:  "shop",
			Location:   "İstanbul İli, Beşiktaş İlçesi, Etiler Alkent Sitesi bünyesinde yer alan alışveriş",
			AreaM2:     floatPtr(100000),
			Confidence: 0.93,
		},
	}, nil, nil, "2026-06-15T00:00:00Z")
	if inventory.AssetCount != 2 {
		t.Fatalf("expected only two reliable inventory assets, got %d: %+v", inventory.AssetCount, inventory.Assets)
	}
	names := map[string]bool{}
	for _, asset := range inventory.Assets {
		names[asset.AssetName] = true
	}
	if !names["Bodrum Hillside Otel"] || !names["F1-01"] {
		t.Fatalf("expected Bodrum Hillside Otel and F1-01, got %+v", names)
	}
}

func TestBuildAssetInventoryExpandsBankOwnedPropertySentence(t *testing.T) {
	snippet := "b.5. Banka mülkiyetinde olan İzmir İli, Konak İlçesi, Akdeniz Mahallesi, 950 ada 6 Parsel numarası ile kayıtlı olan İzmir Hizmet Binası'nın KDV hariç 975.000 TL bedel üzerinden, İzmir İli, Konak İlçesi, Umurbey Mahallesi, 3535 ada 8 ve 9 numaralı parsellerde kayıtlı olan arsaların KDV hariç 945.000 TL bedel üzerinden ve İstanbul İli, Ataşehir İlçesi, İçerenköy Mahallesi, 3219 ada 165 numaralı parselde kayıtlı iki bloktan oluşan taşınmazların KDV hariç 335.000 TL bedel üzerinden olmak üzere değerleri açıklanmıştır."
	date := "2025-03-28"
	inventory := BuildAssetInventory("ISCTR", []AssetEvent{{
		Ticker:                   "ISCTR",
		CompanyName:              "Türkiye İş Bankası Anonim Şirketi",
		AssetName:                "numaralı parsellerde kayıtlı olan arsaların",
		AssetType:                "land",
		Location:                 "İstanbul İli, Ataşehir İlçesi, İçerenköy",
		City:                     "Istanbul",
		District:                 "Ataşehir İlçesi",
		ParcelInfo:               "950 ada 6 Parsel",
		OwnershipType:            "owned",
		ExpertiseDate:            &date,
		ExpertiseValueExclVATTRY: floatPtr(945000),
		SourceReferences:         []AssetSourceReference{{Page: intPtr(129), Snippet: snippet}},
		Confidence:               0.94,
	}}, nil, nil, "2026-06-15T00:00:00Z")

	byName := map[string]AssetInventoryItem{}
	for _, asset := range inventory.Assets {
		byName[asset.AssetName] = asset
	}
	for _, name := range []string{
		"İzmir Konak Akdeniz Hizmet Binası",
		"İzmir Konak Umurbey Arsaları",
		"İstanbul Ataşehir İçerenköy Binaları",
	} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("missing expanded bank-owned asset %q in %+v", name, inventory.Assets)
		}
	}
	if got := byName["İzmir Konak Akdeniz Hizmet Binası"].History[0].ExpertiseValueExclVATTRY; got == nil || *got != 975000 {
		t.Fatalf("unexpected Izmir building value: %.0f asset=%+v", derefFloat(got), byName["İzmir Konak Akdeniz Hizmet Binası"])
	}
	if got := byName["İzmir Konak Umurbey Arsaları"].History[0].ExpertiseValueExclVATTRY; got == nil || *got != 945000 {
		t.Fatalf("unexpected Umurbey land value: %.0f asset=%+v", derefFloat(got), byName["İzmir Konak Umurbey Arsaları"])
	}
	if got := byName["İstanbul Ataşehir İçerenköy Binaları"].History[0].ExpertiseValueExclVATTRY; got == nil || *got != 335000 {
		t.Fatalf("unexpected Atasehir building value: %.0f asset=%+v", derefFloat(got), byName["İstanbul Ataşehir İçerenköy Binaları"])
	}
}

func TestBuildAssetInventoryKeepsValuationCodeAndRejectsFinancialCompanyFragments(t *testing.T) {
	inventory := BuildAssetInventory("ISCTR", []AssetEvent{
		{
			Ticker:                   "ISCTR",
			AssetName:                "ISGY-2503024 KONAK (2 ARSA)",
			AssetType:                "land",
			Location:                 ": İZMİR",
			City:                     "Izmir",
			ParcelInfo:               "3535 ADA 9 PARSEL",
			ExpertiseValueExclVATTRY: floatPtr(10194),
			SourceReferences:         []AssetSourceReference{{Page: intPtr(46), Snippet: "(TL) (KDV Hariç) ISGY-2503024 KONAK (2 ARSA) 7 3535 ADA 9 PARSEL İli : İZMİR"}},
			Confidence:               1,
		},
		{
			Ticker:           "ISCTR",
			AssetName:        "ISGY-2503024 KONAK (2 ARSA)",
			AssetType:        "land",
			ParcelInfo:       "ADA 8 VE 9 PARSELLER",
			SourceReferences: []AssetSourceReference{{Page: intPtr(40), Snippet: "ISGY-2503024 KONAK (2 ARSA) 40 İNDİRGENMİŞ NAKİT AKIMLARI -3535 ADA 8 VE 9 PARSELLER"}},
			Confidence:       0.79,
		},
		{
			Ticker:           "ISCTR",
			AssetName:        "Türkiye Şişe ve Cam Fabrikaları A.Ş.",
			AssetType:        "factory",
			Location:         "İstanbul/TÜRKİYE",
			City:             "Istanbul",
			District:         "TÜRKİYE 67",
			SourceReferences: []AssetSourceReference{{Page: intPtr(145), Snippet: "Türkiye Şişe ve Cam Fabrikaları A.Ş. gerçeğe uygun değer notu"}},
			Confidence:       0.90,
		},
	}, nil, nil, "2026-06-15T00:00:00Z")
	if inventory.AssetCount != 1 {
		t.Fatalf("expected only valuation-code asset, got %d: %+v", inventory.AssetCount, inventory.Assets)
	}
	if inventory.Assets[0].AssetName != "ISGY-2503024 KONAK (2 ARSA)" {
		t.Fatalf("unexpected inventory asset: %+v", inventory.Assets[0])
	}
}

func TestBuildAssetInventoryKeepsDistinctUnitAreasAndAggregateAssets(t *testing.T) {
	inventory := BuildAssetInventory("ALGYO", []AssetEvent{
		{
			Ticker:                   "ALGYO",
			AssetName:                "F2-07",
			AssetType:                "shop",
			Location:                 "Beşiktaş / İstanbul, brüt 34 m², Etiler Alkent Sitesi'nde dükkan",
			City:                     "Istanbul",
			AreaM2:                   floatPtr(34),
			ExpertiseValueExclVATTRY: floatPtr(1500000),
			Confidence:               0.93,
		},
		{
			Ticker:                   "ALGYO",
			AssetName:                "F2-07",
			AssetType:                "shop",
			Location:                 "Beşiktaş / İstanbul, brüt 67 m², Etiler Alkent Sitesi'nde dükkan",
			City:                     "Istanbul",
			AreaM2:                   floatPtr(67),
			ExpertiseValueExclVATTRY: floatPtr(2500000),
			Confidence:               0.93,
		},
		{
			Ticker:       "ALGYO",
			AssetName:    "Büyükçekmece Alkent 2000 – Dükkanlar",
			AssetType:    "shop",
			BookValueTRY: floatPtr(306000000),
			SourceReferences: []AssetSourceReference{{
				Snippet: "Etiler Alkent Sitesi – Dükkanlar Büyükçekmece Alkent 2000 – Dükkanlar Ankara Çankaya İş Merkezi",
			}},
			Confidence: 0.93,
		},
	}, nil, nil, "2026-06-15T00:00:00Z")
	if inventory.AssetCount != 3 {
		t.Fatalf("expected three distinct reliable assets, got %d: %+v", inventory.AssetCount, inventory.Assets)
	}
	unitAreas := map[float64]bool{}
	hasAggregateShop := false
	for _, asset := range inventory.Assets {
		if asset.AssetName == "F2-07" && asset.AreaM2 != nil {
			unitAreas[*asset.AreaM2] = true
		}
		if asset.AssetName == "Büyükçekmece Alkent 2000 10 Adet Dükkan" {
			hasAggregateShop = true
		}
	}
	if !unitAreas[34] || !unitAreas[67] {
		t.Fatalf("expected F2-07 rows with 34 and 67 m2 to stay separate, got %+v", inventory.Assets)
	}
	if !hasAggregateShop {
		t.Fatalf("expected Büyükçekmece Alkent aggregate shop to be retained: %+v", inventory.Assets)
	}
}

func TestNormalizeInventoryAssetNameKeepsEskiceParcelVariants(t *testing.T) {
	if got := normalizeInventoryAssetName("Büyükçekmece Eskice Köyü Arsası -a (2)"); got != "Büyükçekmece Eskice Köyü Arsası - A" {
		t.Fatalf("unexpected -A normalization: %q", got)
	}
	if got := normalizeInventoryAssetName("Büyükçekmece Eskice Köyü Arsası -b (3)"); got != "Büyükçekmece Eskice Köyü Arsası - B" {
		t.Fatalf("unexpected -B normalization: %q", got)
	}
}

func TestLocationSanitizerCleansCrossColumnLocationBleed(t *testing.T) {
	location, city, district := sanitizeAssetLocationFields(
		"Eyüp - Topçular Sanayii Tesisi",
		"KDV Hariç KDV Dahil Ankara",
		"Ankara",
		"KDV Hariç KDV Dahil",
	)
	if location != "" {
		t.Fatalf("expected noisy cross-column location removed, got %q", location)
	}
	if city != "Istanbul" {
		t.Fatalf("expected Topçular city Istanbul, got %q", city)
	}
	if district != "" {
		t.Fatalf("expected noisy district removed, got %q", district)
	}
	if parcel := extractParcelInfo("Eyüp / İstanbul, 15.675 m2, Topçular Mahallesinde 247 Ada, 56 nolu parselde konumlu tesis."); parcel == "" {
		t.Fatalf("expected parcel info from 247 Ada, 56 nolu parsel")
	}
	if parcel := extractParcelInfo("İzmir İli, Konak İlçesi, Umurbey Mahallesi, 3535 ada 8 ve 9 numaralı parsellerde kayıtlı olan arsalar"); parcel == "" || parcel != "3535 ada 8 ve 9 numaralı parseller" {
		t.Fatalf("expected multi-parcel info, got %q", parcel)
	}
	_, _, noisyDistrict := sanitizeAssetLocationFields(
		"İstanbul Karaköy İş Merkezi",
		"Karaköy / İstanbul, brüt 1.730 m², tek blok halinde, asansörlü",
		"Istanbul",
		"asansörlü",
	)
	if noisyDistrict != "" {
		t.Fatalf("expected descriptor district removed, got %q", noisyDistrict)
	}
}

func TestBuildAssetInventoryEnrichesValuesFromActivityReportSegments(t *testing.T) {
	fairValueSnippet := "31 Aralık 2025 tarihi itibarıyla yatırım amaçlı gayrimenkullerin rayiç değerleri aşağıdaki gibidir: Gayrimenkul Adı Gerçeğe Uygun Değeri (TL) Hillside Beach Club Tatil Köyü 11.037.033.271 Bodrum Otel 8.664.062.315 Büyükçekmece Arsası 1.393.208.121 Maslak Arsası 1.235.674.553"
	vatSnippet := "KDV Hariç KDV Dahil Karaköy / İstanbul, brüt 1.730 m², tek blok halinde, asansörlü, - İstanbul Karaköy İş Merkezi klima ısıtmalı, 1/2'si 1997 31.12.2025 224.270.000 269.124.000 yılında satın alınmıştır."
	inventory := BuildAssetInventory("ALGYO", []AssetEvent{
		{
			Ticker:           "ALGYO",
			AssetName:        "Bodrum Otel",
			AssetType:        "hotel",
			City:             "Mugla",
			SourceReferences: []AssetSourceReference{{Page: intPtr(33), Snippet: fairValueSnippet}},
			Confidence:       0.93,
		},
		{
			Ticker:           "ALGYO",
			AssetName:        "İstanbul Karaköy İş Merkezi",
			AssetType:        "office",
			City:             "Istanbul",
			SourceReferences: []AssetSourceReference{{Page: intPtr(8), Snippet: vatSnippet}},
			Confidence:       0.93,
		},
	}, nil, nil, "2026-06-15T00:00:00Z")
	byName := map[string]AssetInventoryItem{}
	for _, asset := range inventory.Assets {
		byName[asset.AssetName] = asset
	}
	bodrum := byName["Bodrum Hillside Otel"]
	if bodrum.AssetName == "" || bodrum.History[0].BookValueTRY == nil || *bodrum.History[0].BookValueTRY != 8664062315 {
		t.Fatalf("expected Bodrum fair value from segment, got %+v", bodrum)
	}
	karakoy := byName["İstanbul Karaköy İş Merkezi"]
	if karakoy.AssetName == "" || karakoy.History[0].ExpertiseValueExclVATTRY == nil || *karakoy.History[0].ExpertiseValueExclVATTRY != 224270000 {
		t.Fatalf("expected Karakoy KDV excl value from segment, got excl=%.0f incl=%.0f asset=%+v", derefFloat(karakoy.History[0].ExpertiseValueExclVATTRY), derefFloat(karakoy.History[0].ExpertiseValueInclVATTRY), karakoy)
	}
	if karakoy.History[0].ExpertiseValueInclVATTRY == nil || *karakoy.History[0].ExpertiseValueInclVATTRY != 269124000 {
		t.Fatalf("expected Karakoy KDV incl value from segment, got %+v", karakoy)
	}
	if karakoy.District == "asansörlü" {
		t.Fatalf("descriptor leaked as district: %+v", karakoy)
	}
}

func TestBuildAssetInventoryMergesTopcularCrossColumnFragments(t *testing.T) {
	inventory := BuildAssetInventory("ALGYO", []AssetEvent{
		{
			Ticker:     "ALGYO",
			AssetName:  "Eyüp Topçular Fabrika",
			AssetType:  "factory",
			City:       "Istanbul",
			AreaM2:     floatPtr(15675),
			Confidence: 0.93,
		},
		{
			Ticker:           "ALGYO",
			AssetName:        "Eyüp - Topçular Sanayii Tesisi",
			AssetType:        "factory",
			Location:         "KDV Hariç KDV Dahil Ankara",
			City:             "Ankara",
			District:         "Ankara fib. 01.02.2011 1 yıl",
			ParcelInfo:       "247 Ada, 56 nolu parsel",
			Confidence:       0.85,
			RentalInfo:       AssetRentalInfo{IsRented: boolPtr(true)},
			SourceReferences: []AssetSourceReference{{Snippet: "KDV Hariç KDV Dahil - Ankara Çankaya İş Merkezi - Eyüp - Topçular Sanayii Tesisi"}},
		},
		{
			Ticker:           "ALGYO",
			AssetName:        "Eyüp Topçular Kargir Fabrika ve Arsası",
			AssetType:        "factory",
			Location:         "Topçular Mahallesinde konumlu, ve Arsası",
			City:             "Istanbul",
			ParcelInfo:       "247 Ada, 56 nolu parsel",
			Confidence:       0.82,
			SourceReferences: []AssetSourceReference{{Snippet: "Eyüp Topçular Kargir Fabrika ve Arsası 247 Ada, 56 nolu parsel"}},
		},
	}, nil, nil, "2026-06-15T00:00:00Z")
	if inventory.AssetCount != 1 {
		t.Fatalf("expected Topçular fragments to merge, got %+v", inventory.Assets)
	}
	asset := inventory.Assets[0]
	if asset.City != "Istanbul" {
		t.Fatalf("expected Istanbul city after normalization, got %+v", asset)
	}
	if asset.District != "Eyüp" || asset.Location != "Eyüp / İstanbul" {
		t.Fatalf("expected noisy location fields removed, got %+v", asset)
	}
	if asset.ParcelInfo == "" {
		t.Fatalf("expected parcel info retained, got %+v", asset)
	}
}

func TestBuildAssetInventoryPortfolioSummaryUsesSourceYearFallback(t *testing.T) {
	inventory := BuildAssetInventory("ALGYO", nil, []PortfolioSummarySnapshot{
		{
			Period:            "2009-12-31",
			TotalBookValueTRY: floatPtr(253330447),
			SourceFile:        "data/equities/ALGYO/kap/attachments/2010/old.pdf",
			Snippet:           "TOPLAM PORTFÖY DEĞERİ 253.330.447",
		},
		{
			TotalBookValueTRY: floatPtr(1234567890),
			SourceFile:        "data/equities/ALGYO/kap/attachments/2026/new.pdf",
			Snippet:           "TOPLAM PORTFÖY DEĞERİ 1.234.567.890",
		},
	}, nil, "2026-06-15T00:00:00Z")
	if inventory.PortfolioSummary.TotalBookValueTRY == nil || *inventory.PortfolioSummary.TotalBookValueTRY != 1234567890 {
		t.Fatalf("expected latest source-year total, got %+v", inventory.PortfolioSummary)
	}
}

func derefFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func writeRawDocumentsForAssetTest(t *testing.T, path string, docs []RawDocument) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create raw docs: %v", err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			t.Fatalf("write raw doc: %v", err)
		}
	}
}

func countJSONLLines(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl %s: %v", path, err)
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan jsonl: %v", err)
	}
	return count
}
