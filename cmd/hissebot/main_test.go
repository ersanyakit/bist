package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	appconfig "hissebot/internal/config"
	taconfig "hissebot/internal/ta/config"
)

func TestWorkersOrMinEnforcesConfiguredMinimum(t *testing.T) {
	if got := workersOrMin(0); got != taconfig.MinWorkers {
		t.Fatalf("workersOrMin(0) = %d, want %d", got, taconfig.MinWorkers)
	}
	if got := workersOrMin(taconfig.MinWorkers + 2); got != taconfig.MinWorkers+2 {
		t.Fatalf("workersOrMin(high) = %d", got)
	}
}

func TestBuildAnalyzeProviderRejectsUnknownProvider(t *testing.T) {
	_, err := buildAnalyzeProvider(taconfig.Config{Provider: "unknown"})
	if !errors.Is(err, taconfig.ErrInvalidProvider) {
		t.Fatalf("buildAnalyzeProvider() error = %v, want ErrInvalidProvider", err)
	}
}

func TestBuildAnalyzeProviderAcceptsSupportedProviders(t *testing.T) {
	for _, provider := range []string{"mock", "csv", "tradingview", "tv", "bistdb", "bist-bulletin-db", "official"} {
		t.Run(provider, func(t *testing.T) {
			got, err := buildAnalyzeProvider(taconfig.Config{Provider: provider})
			if err != nil {
				t.Fatalf("buildAnalyzeProvider(%q) error = %v", provider, err)
			}
			if got == nil {
				t.Fatalf("buildAnalyzeProvider(%q) = nil", provider)
			}
		})
	}
}

func TestBISTBulletinDBPathAcceptsDataRootOrExplicitDB(t *testing.T) {
	if got, want := bistBulletinDBPath("data"), filepath.Join("data", "bist", "bist_ohlcv.sqlite"); got != want {
		t.Fatalf("bistBulletinDBPath(data) = %q, want %q", got, want)
	}
	if got, want := bistBulletinDBPath(filepath.Join("tmp", "official.sqlite")), filepath.Join("tmp", "official.sqlite"); got != want {
		t.Fatalf("bistBulletinDBPath(explicit) = %q, want %q", got, want)
	}
}

func TestAPIServerLoadKAPSectorsSyncsWhenFileMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(testKAPSectorsHTML))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "data", "seed", "kap_sectors.json")
	api := apiServer{cfg: appconfig.Config{
		KAPSectorsFile: path,
		KAPSectorsURL:  server.URL,
		HTTPTimeout:    time.Second,
	}}
	file, err := api.loadKAPSectors()
	if err != nil {
		t.Fatalf("loadKAPSectors() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("synced file stat error = %v", err)
	}
	entry, ok := file.Entries["GARAN"]
	if !ok {
		t.Fatalf("GARAN entry missing: %+v", file.Entries)
	}
	if entry.MainSector != "MALI KURULUSLAR" || entry.Sector != "BANKALAR" {
		t.Fatalf("entry sectors = %q/%q", entry.MainSector, entry.Sector)
	}
}

const testKAPSectorsHTML = `<script>self.__next_f.push([1,"15:[\"$\",\"div\",null,{\"sectorTitles\":[{\"mainSector\":\"MALI KURULUSLAR\",\"normalSector\":[\"BANKALAR\"]}],\"data\":[{\"title\":\"MALI KURULUSLAR\",\"children\":{\"bank\":{\"title\":\"BANKALAR\",\"children\":null,\"content\":[{\"sectorName\":\"BANKALAR\",\"sectorOid\":\"sector-1\",\"sectorNo\":\"008000.001000.\",\"mkkMemberOid\":\"mkk-1\",\"stockCode\":\"GARAN\",\"title\":\"GARANTI BANKASI A.S.\",\"kapTypes\":[\"IGS\"]}]},\"main\":{\"title\":\"MALI KURULUSLAR\",\"children\":null,\"content\":[{\"mainSectorName\":\"MALI KURULUSLAR\",\"mainSectorOid\":\"main-1\",\"mainSectorNo\":\"008000.\",\"mkkMemberOid\":\"mkk-1\",\"stockCode\":\"GARAN\",\"title\":\"GARANTI BANKASI A.S.\",\"kapTypes\":[\"IGS\"]}]}}}],\"lang\":\"tr\"}]\n"])</script>`
