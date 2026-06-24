package tcmb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hissebot/internal/domain/macro"
)

func TestProviderGetFXRateUsesPublicTCMBXMLWithoutKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/kurlar/202606/12062026.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Tarih_Date Tarih="12.06.2026" Date="06/12/2026">
  <Currency Kod="USD" CurrencyCode="USD">
    <Unit>1</Unit>
    <ForexBuying>46.0857</ForexBuying>
    <ForexSelling>46.1688</ForexSelling>
  </Currency>
</Tarih_Date>`))
	}))
	defer server.Close()

	provider := Provider{BaseURL: server.URL + "/kurlar"}
	from := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	got, err := provider.GetFXRate(context.Background(), "USD/TRY", from, from)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one observation, got %d", len(got))
	}
	if got[0].SeriesID != macro.SeriesUSDTRY {
		t.Fatalf("series = %s", got[0].SeriesID)
	}
	if got[0].Value < 46.12 || got[0].Value > 46.13 {
		t.Fatalf("unexpected midpoint rate %.4f", got[0].Value)
	}
	if provider.Info().RequiresKey {
		t.Fatal("TCMB public FX provider must not require API key")
	}
}

func TestProviderGetPolicyRateParsesOfficialRepoTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html><body>
Tarih Borç Alma Borç Verme
12.12.2025 - 38.00
23.01.2026 - 37.00
</body></html>`))
	}))
	defer server.Close()

	provider := Provider{PolicyRateURL: server.URL}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	got, err := provider.GetPolicyRate(context.Background(), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one observation, got %d", len(got))
	}
	if got[0].SeriesID != macro.SeriesPolicyRate || got[0].Value != 37 {
		t.Fatalf("unexpected policy observation: %+v", got[0])
	}
}
