package ops

import (
	"testing"

	"hissebot/internal/config"
)

func TestCheckReadinessReadyForOfflineJSONMode(t *testing.T) {
	dir := t.TempDir()
	report := CheckReadiness(config.Config{
		DataDir:     dir,
		EquitiesDir: dir,
	})
	if report.Status != "ready" {
		t.Fatalf("status = %q, want ready", report.Status)
	}
	if len(report.SecurityIssues) != 0 {
		t.Fatalf("expected no security issues, got %+v", report.SecurityIssues)
	}
}

func TestCheckReadinessReportsSecurityDegradedForExposedEndpoint(t *testing.T) {
	dir := t.TempDir()
	report := CheckReadiness(config.Config{
		DataDir:     dir,
		EquitiesDir: dir,
		EndpointURL: "http://127.0.0.1:9000/analyze",
	})
	if report.Status != "degraded" {
		t.Fatalf("status = %q, want degraded for missing token", report.Status)
	}
	if len(report.SecurityIssues) == 0 {
		t.Fatalf("expected security issues, got %+v", report)
	}
}

func TestCheckReadinessFailsMissingDirectory(t *testing.T) {
	dir := t.TempDir()
	report := CheckReadiness(config.Config{
		DataDir:       dir,
		EquitiesDir:   dir + "/missing",
		EndpointToken: "secret",
	})
	if report.Status != "not_ready" {
		t.Fatalf("status = %q, want not_ready", report.Status)
	}
}
