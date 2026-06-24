package ops

import (
	"os"
	"time"

	"hissebot/internal/config"
)

type HealthReport struct {
	Status         string                 `json:"status"`
	CheckedAt      time.Time              `json:"checked_at"`
	Checks         []HealthCheck          `json:"checks"`
	SecurityIssues []config.SecurityIssue `json:"security_issues,omitempty"`
}

type HealthCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func CheckReadiness(cfg config.Config) HealthReport {
	report := HealthReport{
		Status:    "ready",
		CheckedAt: time.Now().UTC(),
	}
	report.Checks = append(report.Checks, checkDir("data_dir", cfg.DataDir))
	report.Checks = append(report.Checks, checkDir("equities_dir", cfg.EquitiesDir))
	report.SecurityIssues = config.ValidateSecurity(cfg)
	if len(report.SecurityIssues) > 0 {
		report.Status = "degraded"
	}
	for _, check := range report.Checks {
		if check.Status != "ok" {
			report.Status = "not_ready"
			break
		}
	}
	return report
}

func checkDir(name string, path string) HealthCheck {
	if path == "" {
		return HealthCheck{Name: name, Status: "fail", Message: "path is empty"}
	}
	info, err := os.Stat(path)
	if err != nil {
		return HealthCheck{Name: name, Status: "fail", Message: err.Error()}
	}
	if !info.IsDir() {
		return HealthCheck{Name: name, Status: "fail", Message: "path is not a directory"}
	}
	return HealthCheck{Name: name, Status: "ok"}
}
