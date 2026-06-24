package validation

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type Issue struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Field    string   `json:"field,omitempty"`
	Message  string   `json:"message"`
}

type Report struct {
	Status string  `json:"status"`
	Score  float64 `json:"score"`
	Issues []Issue `json:"issues,omitempty"`
}

func NewReport() Report {
	return Report{Status: "pass", Score: 100}
}

func (r *Report) Add(severity Severity, code string, field string, message string) {
	r.Issues = append(r.Issues, Issue{Severity: severity, Code: code, Field: field, Message: message})
	r.recompute()
}

func (r *Report) Merge(other Report) {
	r.Issues = append(r.Issues, other.Issues...)
	r.recompute()
}

func (r *Report) recompute() {
	status := "pass"
	score := 100.0
	for _, issue := range r.Issues {
		switch issue.Severity {
		case SeverityCritical:
			status = "fail"
			score -= 35
		case SeverityError:
			if status != "fail" {
				status = "fail"
			}
			score -= 25
		case SeverityWarning:
			if status == "pass" {
				status = "limited"
			}
			score -= 10
		default:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	r.Status = status
	r.Score = score
}
