package kap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/domain/financials"
	"hissebot/internal/storage"
)

func TestProviderReadsLocalFinancialStatements(t *testing.T) {
	root := t.TempDir()
	store := storage.NewEquityStore(root)
	writeJSON(t, store.FinancialInfoPath("TEST"), domain.BilancoInfo{
		Ticker:         "TEST",
		Source:         "kap_disclosures",
		Currency:       "TRY",
		FinancialGroup: "XI_29",
		FetchedAt:      time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC),
		Data: map[string]domain.BilancoField{
			"1BL":  field("Toplam Varlıklar", 1000),
			"2A":   field("Kısa Vadeli Yükümlülükler", 250),
			"2B":   field("Uzun Vadeli Yükümlülükler", 150),
			"2N":   field("Özkaynaklar", 600),
			"1AA":  field("Nakit ve Nakit Benzerleri", 90),
			"3C":   field("Satış Gelirleri", 700),
			"3D":   field("Brüt Kar", 210),
			"3DF":  field("Faaliyet Karı", 120),
			"3L":   field("Net Kar", 80),
			"4C":   field("İşletme Faaliyetlerinden Kaynaklanan Net Nakit", 110),
			"4CAK": field("Yatırım Faaliyetlerinden Kaynaklanan Nakit", -40),
			"4CB":  field("Serbest Nakit Akım", 70),
			"4CBE": field("Finansman Faaliyetlerden Kaynaklanan Nakit", -20),
			"4CBL": field("Dönem Sonu Nakit", 90),
		},
	})

	provider := NewWithStore("", store)
	period := financials.Period{Year: 2026, Quarter: 4, Type: financials.PeriodAnnual}
	balance, err := provider.GetBalanceSheet(context.Background(), "TEST", period)
	if err != nil {
		t.Fatal(err)
	}
	if balance.TotalAssets != 1000 || balance.TotalLiabilities != 400 || balance.TotalEquity != 600 {
		t.Fatalf("unexpected balance sheet: %+v", balance)
	}
	income, err := provider.GetIncomeStatement(context.Background(), "TEST", period)
	if err != nil {
		t.Fatal(err)
	}
	if income.Sales != 700 || income.NetIncome != 80 {
		t.Fatalf("unexpected income statement: %+v", income)
	}
	cashFlow, err := provider.GetCashFlowStatement(context.Background(), "TEST", period)
	if err != nil {
		t.Fatal(err)
	}
	if cashFlow.OperatingCashFlow != 110 || cashFlow.FreeCashFlow != 70 {
		t.Fatalf("unexpected cash flow: %+v", cashFlow)
	}
	if provider.Info().RequiresKey {
		t.Fatal("KAP provider must not require API key")
	}
}

func TestProviderFetchesKAPDisclosuresWithoutAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"disclosureBasic": {
				"title": "Finansal Rapor",
				"stockCode": "TEST",
				"disclosureClass": "FR",
				"disclosureType": "FR",
				"disclosureCategory": "FR",
				"publishDate": "12.06.2026 10:00:00",
				"disclosureId": "kap-test-1",
				"year": 2026,
				"donem": 3
			},
			"disclosureDetail": {}
		}]`))
	}))
	defer server.Close()

	provider := Provider{BaseURL: server.URL + "/tr/api/disclosure/list/main"}
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	got, err := provider.GetDisclosures(context.Background(), "TEST", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one disclosure, got %d", len(got))
	}
	if got[0].Symbol != "TEST" || got[0].Title != "Finansal Rapor" {
		t.Fatalf("unexpected disclosure: %+v", got[0])
	}
}

func field(desc string, q4 float64) domain.BilancoField {
	return domain.BilancoField{
		DescTR: desc,
		Years: map[string][]*float64{
			"2026": {floatPtr(q4), nil, nil, nil},
		},
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
