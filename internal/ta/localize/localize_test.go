package localize

import "testing"

func TestSignalLocalizesExternalAndProxyStatuses(t *testing.T) {
	cases := map[string]string{
		"requires_external_data": "Hesaplanmadı: Dış Veri Gerekir",
		"proxy_only":             "Yaklaşık Hesap (Sinyal Değil)",
	}

	for input, want := range cases {
		if got := Signal(input); got != want {
			t.Fatalf("Signal(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSignalFallsBackToInputForUnknownCode(t *testing.T) {
	if got := Signal("custom_status"); got != "custom_status" {
		t.Fatalf("Signal(unknown) = %q, want input", got)
	}
}
