package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendBuildsHashChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	first, err := Append(path, Event{Action: "import", Entity: "kap_disclosures", EntityID: "A"})
	if err != nil {
		t.Fatalf("append first: %v", err)
	}
	second, err := Append(path, Event{Action: "import", Entity: "kap_disclosures", EntityID: "B"})
	if err != nil {
		t.Fatalf("append second: %v", err)
	}
	if first.EventHash == "" || second.EventHash == "" {
		t.Fatalf("missing event hash: %+v %+v", first, second)
	}
	if second.PrevHash != first.EventHash {
		t.Fatalf("prev hash = %q, want %q", second.PrevHash, first.EventHash)
	}
	report, err := Verify(path)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if report.Status != "pass" || report.Events != 2 || report.LastHash != second.EventHash {
		t.Fatalf("unexpected verify report: %+v", report)
	}
}

func TestVerifyRejectsTamperedHashChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if _, err := Append(path, Event{Action: "import", Entity: "kap_disclosures", EntityID: "A"}); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if _, err := Append(path, Event{Action: "import", Entity: "kap_disclosures", EntityID: "B"}); err != nil {
		t.Fatalf("append second: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	raw[0] = '{'
	raw[20] = 'X'
	if err := os.WriteFile(path, raw, 0o640); err != nil {
		t.Fatalf("tamper audit: %v", err)
	}
	report, err := Verify(path)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if report.Status != "fail" || len(report.Errors) == 0 {
		t.Fatalf("expected verify failure, got %+v", report)
	}
}
