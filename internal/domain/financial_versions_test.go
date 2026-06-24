package domain

import (
	"testing"
	"time"
)

func TestUpsertStatementVersionsCreatesRestatementRevision(t *testing.T) {
	value1 := 10.0
	value2 := 12.0
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	info := &BilancoInfo{
		Ticker: "TEST",
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value1}}},
		},
	}
	store := UpsertStatementVersions(FinancialStatementVersionStore{}, "TEST", BuildStatementVersions(info, now), now)
	if len(store.Versions) != 1 {
		t.Fatalf("versions = %d, want 1", len(store.Versions))
	}

	info.Data["3L"] = BilancoField{Years: map[string][]*float64{"2026": {nil, nil, nil, &value2}}}
	store = UpsertStatementVersions(store, "TEST", BuildStatementVersions(info, now.Add(time.Hour)), now.Add(time.Hour))
	if len(store.Versions) != 2 {
		t.Fatalf("versions = %d, want 2", len(store.Versions))
	}
	latestID := store.Latest["2026-Q1"]
	latest, ok := findStatementVersion(store.Versions, latestID)
	if !ok {
		t.Fatalf("missing latest version %q", latestID)
	}
	if !latest.IsRestatement || latest.Revision != 2 || latest.RestatesVersionID == "" {
		t.Fatalf("latest restatement metadata = %+v", latest)
	}
}

func TestUpsertStatementVersionsUpdatesAvailabilityMetadataWithoutRestatement(t *testing.T) {
	value := 10.0
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	info := &BilancoInfo{
		Ticker: "TEST",
		Data: map[string]BilancoField{
			"3L": {Years: map[string][]*float64{"2026": {nil, nil, nil, &value}}},
		},
	}
	store := UpsertStatementVersions(FinancialStatementVersionStore{}, "TEST", BuildStatementVersions(info, now), now)
	availableAt := now.Add(time.Hour)
	MarkFinancialPeriodsAvailableAt(info, availableAt, "unit_test_available_at")
	store = UpsertStatementVersions(store, "TEST", BuildStatementVersions(info, availableAt), availableAt)

	if len(store.Versions) != 1 {
		t.Fatalf("versions = %d, want 1 metadata-only update", len(store.Versions))
	}
	latest, ok := findStatementVersion(store.Versions, store.Latest["2026-Q1"])
	if !ok {
		t.Fatal("latest version missing")
	}
	if latest.AvailableAt == nil || !latest.AvailableAt.Equal(availableAt) {
		t.Fatalf("available_at = %v, want %v", latest.AvailableAt, availableAt)
	}
	if latest.AvailabilitySource != "unit_test_available_at" {
		t.Fatalf("availability source = %q", latest.AvailabilitySource)
	}
}
