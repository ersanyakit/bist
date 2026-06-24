package kap

import (
	"net/http"
	"testing"
)

func TestSetKAPRequestHeadersAddsRandomUserAgent(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://www.kap.org.tr/tr/api/file/download/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	setKAPRequestHeaders(req)

	if got := req.Header.Get("User-Agent"); got == "" {
		t.Fatal("expected User-Agent header to be set")
	}
	if got := req.Header.Get("Accept-Language"); got == "" {
		t.Fatal("expected Accept-Language header to be set")
	}
}

func TestSetKAPRequestHeadersKeepsExistingUserAgent(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://www.kap.org.tr/tr/api/file/download/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "custom-agent")

	setKAPRequestHeaders(req)

	if got := req.Header.Get("User-Agent"); got != "custom-agent" {
		t.Fatalf("User-Agent = %q, want custom-agent", got)
	}
}
