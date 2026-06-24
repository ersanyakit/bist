package corporateactions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsMarketWSAndKAPCorporateEvents(t *testing.T) {
	dir := t.TempDir()
	equities := filepath.Join(dir, "data", "equities")
	marketWS := filepath.Join(equities, "TEST", "market_ws")
	processed := filepath.Join(dir, "data", "processed", "by_ticker", "TEST")
	if err := os.MkdirAll(marketWS, 0o755); err != nil {
		t.Fatalf("mkdir market ws: %v", err)
	}
	if err := os.MkdirAll(processed, 0o755); err != nil {
		t.Fatalf("mkdir processed: %v", err)
	}
	writeFile(t, filepath.Join(marketWS, "capital_increases_data.json"), `{
		"results": [
			{"symbol":"TEST","title":"%100 bedelsiz sermaye artırımı","effective_date":"2026-01-03","bedelsiz_oran":100}
		]
	}`)
	writeFile(t, filepath.Join(processed, "corporate_events.jsonl"), `{"id":"kap-1","ticker":"TEST","event_type":"capital_increase","title":"sermaye artırımı dolayısıyla ihraç belgesi","document_date":"2026-01-01","confidence":0.63,"review_required":true}`+"\n")

	set := Load(equities, "TEST")
	if len(set.Actions) != 2 {
		t.Fatalf("actions = %+v", set.Actions)
	}
	if set.Status != "adjustment_ready" {
		t.Fatalf("status = %q, warnings=%v", set.Status, set.Warnings)
	}
	if set.AdjustmentReadyActions != 1 || set.ReviewRequiredActions != 1 {
		t.Fatalf("counts = %+v", set)
	}
	if set.Actions[0].Type != TypeCapitalIncrease && set.Actions[1].Type != TypeCapitalIncrease && set.Actions[0].Type != TypeBonusIssue && set.Actions[1].Type != TypeBonusIssue {
		t.Fatalf("capital action not detected: %+v", set.Actions)
	}
}

func TestLoadReadsKAPDisclosuresAndFiltersNonIssuerEvents(t *testing.T) {
	dir := t.TempDir()
	equities := filepath.Join(dir, "data", "equities")
	root := filepath.Join(equities, "TEST")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir equities: %v", err)
	}
	writeFile(t, filepath.Join(root, "kap_disclosures.json"), `[
		{"id":"kap-bonus","ticker":"TEST","title":"Sermaye Artırımı - Azaltımı İşlemlerine İlişkin Bildirim","summary":"%100 Bedelsiz Pay Alma Hakkı Kullanım Tarihi","disclosure_type":"CA","disclosure_category":"STT","disclosure_index":123,"attachment_count":1,"publish_date":"2026-02-03T10:00:00Z"},
		{"id":"kap-dividend","ticker":"TEST","title":"Kar Payı Dağıtım İşlemlerine İlişkin Bildirim","summary":"Nakit Kar Payı Ödeme Tarihi pay başına brüt 1,25 TL","disclosure_type":"CA","disclosure_category":"STT","disclosure_index":124,"publish_date":"2026-03-04T10:00:00Z"},
		{"id":"kap-none","ticker":"TEST","title":"Kar Payı Dağıtım İşlemlerine İlişkin Bildirim","summary":"Kar Payı Dağıtımı Yapılmamasına İlişkin Genel Kurul Kararı","disclosure_index":125,"publish_date":"2026-04-05T10:00:00Z"},
		{"id":"kap-subsidiary","ticker":"TEST","title":"Özel Durum Açıklaması (Genel)","summary":"Bağlı Ortaklık Sermaye Artırımına Katılım","disclosure_index":126,"publish_date":"2026-05-06T10:00:00Z"}
	]`)

	set := Load(equities, "TEST")
	if len(set.Actions) != 2 {
		t.Fatalf("actions = %+v", set.Actions)
	}
	if set.AdjustmentReadyActions != 2 || set.Status != "candidate_adjustments_review_required" {
		t.Fatalf("set = %+v", set)
	}
	var gotBonus, gotDividend bool
	for _, action := range set.Actions {
		if action.Source != "kap_disclosures" || action.SourceURL == "" || action.EffectiveDate == nil {
			t.Fatalf("invalid KAP action source/date: %+v", action)
		}
		switch action.Type {
		case TypeBonusIssue:
			gotBonus = true
			if action.Ratio == nil || *action.Ratio != 1 {
				t.Fatalf("bonus ratio = %+v", action.Ratio)
			}
		case TypeDividend:
			gotDividend = true
			if action.CashAmount == nil || *action.CashAmount != 1.25 {
				t.Fatalf("dividend cash = %+v", action.CashAmount)
			}
		}
	}
	if !gotBonus || !gotDividend {
		t.Fatalf("missing expected actions: %+v", set.Actions)
	}
}

func TestLoadMissingSourceReportsMissing(t *testing.T) {
	dir := t.TempDir()
	equities := filepath.Join(dir, "data", "equities")
	if err := os.MkdirAll(filepath.Join(equities, "TEST"), 0o755); err != nil {
		t.Fatalf("mkdir equities: %v", err)
	}
	set := Load(equities, "TEST")
	if set.Status != "missing" || len(set.Actions) != 0 {
		t.Fatalf("set = %+v", set)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
