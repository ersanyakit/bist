package kap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/domain"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

func TestApplyFinancialPublishDatesMatchesFinancialDisclosurePeriod(t *testing.T) {
	value := 10.0
	info := &domain.BilancoInfo{
		Ticker: "TEST",
		Data: map[string]domain.BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}
	domain.NormalizeBilancoInfo(info, "TEST")
	publishDate := time.Date(2026, 5, 9, 18, 30, 0, 0, time.UTC)

	resolved := ApplyFinancialPublishDates(info, []FinancialDisclosure{{
		Title:         "2026 3 Aylık Finansal Rapor",
		FiscalYear:    2026,
		FiscalQuarter: 1,
		PublishDate:   &publishDate,
		Source:        "kap",
	}})

	if resolved != 1 {
		t.Fatalf("resolved = %d, want 1", resolved)
	}
	period := info.Periods["2026-Q1"]
	if period.PublishDate == nil || !period.PublishDate.Equal(publishDate) {
		t.Fatalf("publish date = %v, want %v", period.PublishDate, publishDate)
	}
	if !info.Quality.BacktestSafe {
		t.Fatalf("expected quality to be backtest safe, got %+v", info.Quality)
	}
}

func TestApplyFinancialPublishDatesIgnoresNonFinancialDisclosure(t *testing.T) {
	value := 10.0
	info := &domain.BilancoInfo{
		Ticker: "TEST",
		Data: map[string]domain.BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}
	domain.NormalizeBilancoInfo(info, "TEST")
	publishDate := time.Date(2026, 5, 9, 18, 30, 0, 0, time.UTC)

	resolved := ApplyFinancialPublishDates(info, []FinancialDisclosure{{
		Title:         "Genel Kurul Toplantısı",
		FiscalYear:    2026,
		FiscalQuarter: 1,
		PublishDate:   &publishDate,
		Source:        "kap",
	}})

	if resolved != 0 {
		t.Fatalf("resolved = %d, want 0", resolved)
	}
	if info.Quality.BacktestSafe {
		t.Fatalf("expected non-financial disclosure to leave data unsafe")
	}
}

func TestParseFinancialDisclosuresHandlesNestedKAPResponse(t *testing.T) {
	raw := []byte(`[
		{
			"disclosureBasic": {
				"title": "Faaliyet Raporu (Konsolide)",
				"stockCode": "AEFES",
				"disclosureClass": "FR",
				"disclosureType": "FR",
				"publishDate": "30.03.2010 22:51:16",
				"disclosureId": "abc",
				"year": 2009,
				"donem": 4,
				"period": "3AB"
			}
		}
	]`)

	disclosures, err := parseFinancialDisclosures(raw)
	if err != nil {
		t.Fatalf("parse disclosures: %v", err)
	}
	if len(disclosures) != 1 {
		t.Fatalf("disclosures = %+v", disclosures)
	}
	got := disclosures[0]
	if got.Ticker != "AEFES" || got.PeriodKey != "2009-Q4" || got.DisclosureClass != "FR" {
		t.Fatalf("unexpected disclosure = %+v", got)
	}
	if got.PublishDate == nil {
		t.Fatalf("publish date is nil")
	}
}

func TestFetchFinancialDisclosuresAllTypesOmitsDisclosureTypeFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, exists := payload["disclosureTypes"]; exists {
			t.Fatalf("disclosureTypes must be omitted for all-category KAP fetch, got %#v", payload["disclosureTypes"])
		}
		_, _ = w.Write([]byte(`[{
			"disclosureBasic": {
				"title": "Özel Durum Açıklaması",
				"stockCode": "TEST",
				"disclosureClass": "ODA",
				"disclosureType": "ODA",
				"disclosureCategory": "ODA",
				"publishDate": "01.06.2026 10:00:00",
				"disclosureId": "kap-all-1"
			},
			"disclosureDetail": {}
		}]`))
	}))
	defer server.Close()

	got, err := FetchFinancialDisclosures(context.Background(), config.Config{HTTPTimeout: time.Second}, FinancialDisclosureSyncOptions{
		URL:             server.URL + "/tr/api/disclosure/list/main",
		FromDate:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		ToDate:          time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		DisclosureTypes: []string{AllDisclosureTypes},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DisclosureType != "ODA" {
		t.Fatalf("unexpected disclosures: %#v", got)
	}
}

func TestApplyFinancialPublishDatesAcceptsFinancialReportClass(t *testing.T) {
	value := 10.0
	info := &domain.BilancoInfo{
		Ticker: "TEST",
		Data: map[string]domain.BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}
	domain.NormalizeBilancoInfo(info, "TEST")
	publishDate := time.Date(2026, 5, 9, 18, 30, 0, 0, time.UTC)

	resolved := ApplyFinancialPublishDates(info, []FinancialDisclosure{{
		Title:           "Faaliyet Raporu",
		DisclosureClass: "FR",
		FiscalYear:      2026,
		FiscalQuarter:   1,
		PublishDate:     &publishDate,
		Source:          "kap",
	}})

	if resolved != 1 {
		t.Fatalf("resolved = %d, want 1", resolved)
	}
}

func TestImportFinancialDisclosureListWritesTickerDisclosureFile(t *testing.T) {
	store := storage.NewEquityStore(t.TempDir())
	if err := store.Save(&domain.Equity{Ticker: "TEST", AssetType: 2}); err != nil {
		t.Fatal(err)
	}
	publishDate := time.Date(2026, 5, 9, 18, 30, 0, 0, time.UTC)
	updated, _, err := ImportFinancialDisclosureList(context.Background(), store, []FinancialDisclosure{{
		Ticker:        "TEST",
		Title:         "2026 3 Aylık Finansal Rapor",
		FiscalYear:    2026,
		FiscalQuarter: 1,
		PublishDate:   &publishDate,
		Source:        "kap",
	}})
	if err != nil {
		t.Fatalf("import disclosures: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}
	disclosures, err := LoadFinancialDisclosures(store.KAPDisclosuresPath("TEST"))
	if err != nil {
		t.Fatalf("load disclosures: %v", err)
	}
	if len(disclosures) != 1 || disclosures[0].Ticker != "TEST" {
		t.Fatalf("disclosures = %+v", disclosures)
	}
}

func TestImportFinancialDisclosureListResolvesMultiCodeKAPDisclosureToKnownTicker(t *testing.T) {
	store := storage.NewEquityStore(t.TempDir())
	if err := store.Save(&domain.Equity{Ticker: "A1CAP", AssetType: 2}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(store.KAPPath("A1CAP"), map[string]any{"stockCode": "A1CAP, ACP"}); err != nil {
		t.Fatal(err)
	}
	publishDate := time.Date(2026, 5, 9, 18, 30, 0, 0, time.UTC)
	updated, _, err := ImportFinancialDisclosureList(context.Background(), store, []FinancialDisclosure{{
		Ticker:          "A1CAP, ACP",
		DisclosureClass: "FR",
		FiscalYear:      2026,
		FiscalQuarter:   1,
		PublishDate:     &publishDate,
		Source:          "kap",
	}})
	if err != nil {
		t.Fatalf("import disclosures: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}
	if _, err := LoadFinancialDisclosures(store.KAPDisclosuresPath("A1CAP")); err != nil {
		t.Fatalf("load canonical disclosures: %v", err)
	}
	if _, err := LoadFinancialDisclosures(store.KAPDisclosuresPath("A1CAP_ACP")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected synthetic ticker disclosure file err=%v", err)
	}
}

func TestImportFinancialDisclosureListRejectsAliasWhenCompanyTitleDiffers(t *testing.T) {
	store := storage.NewEquityStore(t.TempDir())
	if err := store.Save(&domain.Equity{Ticker: "A1CAP", AssetType: 2}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(store.KAPPath("A1CAP"), map[string]any{
		"stockCode":      "A1CAP, ACP",
		"kapMemberTitle": "A1 CAPITAL YATIRIM MENKUL DEĞERLER A.Ş.",
	}); err != nil {
		t.Fatal(err)
	}
	publishDate := time.Date(2010, 3, 3, 9, 9, 1, 0, time.UTC)
	updated, _, err := ImportFinancialDisclosureList(context.Background(), store, []FinancialDisclosure{{
		Ticker:          "A1CAP, ACP",
		DisclosureClass: "FR",
		FiscalYear:      2009,
		FiscalQuarter:   4,
		PublishDate:     &publishDate,
		Source:          "kap",
		Raw: map[string]any{
			"disclosureBasic": map[string]any{
				"stockCode":    "A1CAP, ACP",
				"companyTitle": "CREDIT AGRICOLE CHEUVREUX MENKUL DEĞERLER A.Ş.",
			},
		},
	}})
	if err != nil {
		t.Fatalf("import disclosures: %v", err)
	}
	if updated != 0 {
		t.Fatalf("updated = %d, want 0", updated)
	}
	if _, err := LoadFinancialDisclosures(store.KAPDisclosuresPath("A1CAP")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected canonical disclosure file err=%v", err)
	}
}

func TestImportFinancialDisclosureListRefreshesStatementVersionMetadata(t *testing.T) {
	store := storage.NewEquityStore(t.TempDir())
	value := 10.0
	info := &domain.BilancoInfo{
		Ticker: "TEST",
		Data: map[string]domain.BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}
	domain.NormalizeBilancoInfo(info, "TEST")
	if err := store.Save(&domain.Equity{Ticker: "TEST", AssetType: 2, BilancoInfo: info}); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSON(store.FinancialInfoPath("TEST"), info); err != nil {
		t.Fatal(err)
	}
	versionStore := domain.UpsertStatementVersions(domain.FinancialStatementVersionStore{}, "TEST", domain.BuildStatementVersions(info, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err := util.WriteJSON(store.FinancialStatementVersionsPath("TEST"), versionStore); err != nil {
		t.Fatal(err)
	}
	publishDate := time.Date(2026, 5, 9, 18, 30, 0, 0, time.UTC)

	_, resolved, err := ImportFinancialDisclosureList(context.Background(), store, []FinancialDisclosure{{
		Ticker:          "TEST",
		DisclosureClass: "FR",
		FiscalYear:      2026,
		FiscalQuarter:   1,
		PublishDate:     &publishDate,
		Source:          "kap",
	}})
	if err != nil {
		t.Fatalf("import disclosures: %v", err)
	}
	if resolved != 1 {
		t.Fatalf("resolved = %d, want 1", resolved)
	}
	var refreshed domain.FinancialStatementVersionStore
	if err := util.ReadJSON(store.FinancialStatementVersionsPath("TEST"), &refreshed); err != nil {
		t.Fatal(err)
	}
	if len(refreshed.Versions) != 1 || refreshed.Versions[0].PublishDate == nil || !refreshed.Versions[0].PublishDate.Equal(publishDate) {
		t.Fatalf("refreshed versions = %+v", refreshed.Versions)
	}
}
