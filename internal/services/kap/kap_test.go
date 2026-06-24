package kap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"hissebot/internal/config"
)

func TestSplitStockCodesNormalizesCommaSeparatedTickers(t *testing.T) {
	got := splitStockCodes(" EUPWR , ASELS, BIST:thyao ")
	want := []string{"EUPWR", "ASELS", "THYAO"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitStockCodes() = %#v, want %#v", got, want)
	}
}

func TestSplitStockCodesKeepsEmptyEntriesForCallerSkip(t *testing.T) {
	got := splitStockCodes("EUPWR,,ASELS")
	want := []string{"EUPWR", "", "ASELS"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitStockCodes() = %#v, want %#v", got, want)
	}
}

func TestFetchCompaniesReadsKAPCompanyItemsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		_, _ = w.Write([]byte(`[{
			"kapMemberTitle": "TEST A.Ş.",
			"stockCode": "TEST",
			"kapMemberType": "IGS",
			"kapMemberState": "A"
		}]`))
	}))
	defer server.Close()

	got, err := FetchCompanies(context.Background(), config.Config{}, CompanySyncOptions{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["stockCode"] != "TEST" {
		t.Fatalf("unexpected companies: %#v", got)
	}
}
