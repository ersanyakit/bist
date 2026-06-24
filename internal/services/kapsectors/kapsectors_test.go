package kapsectors

import (
	"testing"
	"time"
)

func TestExtractFromHTMLParsesEmbeddedSectorPayload(t *testing.T) {
	html := `<script>self.__next_f.push([1,"15:[\"$\",\"div\",null,{\"sectorTitles\":[{\"mainSector\":\"MALİ KURULUŞLAR\",\"normalSector\":[\"BANKALAR\"]}],\"data\":[{\"title\":\"MALİ KURULUŞLAR\",\"children\":{\"bank\":{\"title\":\"BANKALAR\",\"children\":null,\"content\":[{\"sectorName\":\"BANKALAR\",\"sectorOid\":\"sector-1\",\"sectorNo\":\"008000.001000.\",\"mkkMemberOid\":\"mkk-1\",\"stockCode\":\"GARAN, TGB\",\"title\":\"TÜRKİYE GARANTİ BANKASI A.Ş.\",\"kapTypes\":[\"IGS\",\"YK\"]}]},\"main\":{\"title\":\"MALİ KURULUŞLAR\",\"children\":null,\"content\":[{\"mainSectorName\":\"MALİ KURULUŞLAR\",\"mainSectorOid\":\"main-1\",\"mainSectorNo\":\"008000.\",\"mkkMemberOid\":\"mkk-1\",\"stockCode\":\"GARAN, TGB\",\"title\":\"TÜRKİYE GARANTİ BANKASI A.Ş.\",\"kapTypes\":[\"IGS\",\"YK\"]}]}}}],\"lang\":\"tr\"}]\n"])</script>`

	file, err := ExtractFromHTML([]byte(html), "https://kap.org.tr/tr/Sektorler", time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("ExtractFromHTML() error = %v", err)
	}
	if file.Summary.Symbols != 2 {
		t.Fatalf("symbols = %d, want 2", file.Summary.Symbols)
	}
	entry := file.Entries["GARAN"]
	if entry.MainSector != "MALİ KURULUŞLAR" {
		t.Fatalf("main sector = %q", entry.MainSector)
	}
	if entry.Sector != "BANKALAR" {
		t.Fatalf("sector = %q", entry.Sector)
	}
	if len(entry.AllSectors) != 2 {
		t.Fatalf("all sectors = %d, want 2", len(entry.AllSectors))
	}
	if file.Entries["TGB"].Sector != "BANKALAR" {
		t.Fatalf("split symbol TGB missing: %+v", file.Entries["TGB"])
	}
}

func TestExtractFromHTMLReturnsHelpfulErrorForMissingPayload(t *testing.T) {
	_, err := ExtractFromHTML([]byte("<html></html>"), "x", time.Now())
	if err == nil {
		t.Fatal("ExtractFromHTML() error = nil, want error")
	}
}
