package patterns

import (
	"fmt"
	"math"
	"strings"

	"hissebot/internal/ta/ohlcv"
)

const (
	ValidationSeverityError   = "error"
	ValidationSeverityWarning = "warning"
)

type ValidationIssue struct {
	Scope    string `json:"scope"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ValidationReport struct {
	Checked int               `json:"checked"`
	Issues  []ValidationIssue `json:"issues"`
}

func (r ValidationReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == ValidationSeverityError {
			return true
		}
	}
	return false
}

func (r ValidationReport) Err() error {
	if !r.HasErrors() {
		return nil
	}
	return fmt.Errorf("%s", r.Summary())
}

func (r ValidationReport) Summary() string {
	errors, warnings := 0, 0
	for _, issue := range r.Issues {
		switch issue.Severity {
		case ValidationSeverityError:
			errors++
		case ValidationSeverityWarning:
			warnings++
		}
	}
	if len(r.Issues) == 0 {
		return fmt.Sprintf("validated %d pattern checks with no issues", r.Checked)
	}
	parts := []string{fmt.Sprintf("validated %d pattern checks: %d error(s), %d warning(s)", r.Checked, errors, warnings)}
	limit := minInt(len(r.Issues), 5)
	for i := 0; i < limit; i++ {
		issue := r.Issues[i]
		parts = append(parts, fmt.Sprintf("%s/%s: %s", issue.Scope, issue.Name, issue.Message))
	}
	if len(r.Issues) > limit {
		parts = append(parts, fmt.Sprintf("... %d more issue(s)", len(r.Issues)-limit))
	}
	return strings.Join(parts, "; ")
}

func (r *ValidationReport) Merge(other ValidationReport) {
	r.Checked += other.Checked
	r.Issues = append(r.Issues, other.Issues...)
}

func (r *ValidationReport) addIssue(scope, name, severity, message string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Scope:    scope,
		Name:     name,
		Severity: severity,
		Message:  message,
	})
}

func ValidatePatternSystem(input ScannerInput, output ScannerOutput) ValidationReport {
	var report ValidationReport
	report.Merge(validatePatternCandles(input.Candles))
	report.Merge(ValidatePatternCatalog())
	report.Merge(ValidatePatternScan(input, output))
	return report
}

func ValidatePatternCatalog() ValidationReport {
	var report ValidationReport
	specs := registeredPatternSpecs()
	if len(specs) == 0 {
		report.addIssue("catalog", "patterns", ValidationSeverityError, "no registered pattern specs")
		return report
	}
	seen := map[string]struct{}{}
	for _, spec := range specs {
		key := normalizePatternText(spec.Name)
		report.Checked++
		if key == "" {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "empty pattern name")
		}
		report.Checked++
		if _, exists := seen[key]; exists {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "duplicate pattern name")
		}
		seen[key] = struct{}{}
		report.Checked++
		if strings.TrimSpace(spec.Category) == "" || strings.TrimSpace(spec.Template) == "" || strings.TrimSpace(string(spec.Rule)) == "" {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "missing category, template or rule")
		}
		report.Checked++
		if !validPatternDirection(spec.Direction) {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "unsupported direction "+spec.Direction)
		}
		report.Checked++
		if spec.Confidence < 0 || spec.Confidence > 1 {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "confidence is outside [0,1]")
		}
		report.Checked++
		if strings.TrimSpace(spec.Evidence) == "" {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "missing evidence")
		}
		if patternRequiresExternalData(spec.Template) {
			report.Checked++
			if patternTemplateHasScannerRule(spec.Template) {
				report.addIssue("catalog", spec.Name, ValidationSeverityWarning, "external-only template also has a scanner rule")
			}
			continue
		}
		report.Checked++
		if spec.Template == "generic" || patternRequiresSpecificRule(spec.Template) {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "pattern has no explicit scanner rule")
		}
		rule, ok := registeredPatternRule(spec.Rule)
		report.Checked++
		if !ok {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "bound scanner rule is not registered")
			continue
		}
		report.Checked++
		if rule.id != spec.Rule || rule.match == nil {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "bound scanner rule is malformed")
		}
	}
	for id, rule := range patternRuleRegistry {
		report.Checked++
		if id == "" || rule.id != id || rule.match == nil {
			report.addIssue("registry", string(id), ValidationSeverityError, "registered pattern rule is malformed")
		}
	}
	return report
}

func ValidatePatternScan(input ScannerInput, output ScannerOutput) ValidationReport {
	var report ValidationReport
	report.Checked++
	if output.DetectorCount < 0 || output.ScannedCount < 0 || output.MatchedCount < 0 {
		report.addIssue("scan", "counts", ValidationSeverityError, "negative scanner count")
	}
	report.Checked++
	if output.DetectorCount != 0 && output.ScannedCount != output.DetectorCount {
		report.addIssue("scan", "counts", ValidationSeverityError, "scanned count does not match detector count")
	}
	report.Checked++
	if output.MatchedCount != len(output.Patterns) {
		report.addIssue("scan", "matched_count", ValidationSeverityError, "matched count does not match pattern results")
	}
	report.Checked++
	if output.CandidateCount != 0 && output.CandidateCount != len(output.PatternCandidates) {
		report.addIssue("scan", "candidate_count", ValidationSeverityError, "candidate count does not match pattern candidates")
	}
	specs := registeredPatternSpecs()
	specByName := map[string]patternSpec{}
	for _, spec := range specs {
		specByName[normalizePatternText(spec.Name)] = spec
	}
	report.Checked++
	if len(output.PatternScans) < len(specs) {
		report.addIssue("scan", "pattern_scans", ValidationSeverityError, "scan catalog is missing registered patterns")
	}
	validatePatternResults(&report, input, output.Patterns, specByName)
	if len(output.PatternCandidates) > 0 {
		validatePatternResults(&report, input, output.PatternCandidates, specByName)
	}
	validatePatternScans(&report, output.PatternScans, output.Patterns, output.PatternCandidates, specByName)
	return report
}

func validatePatternResults(report *ValidationReport, input ScannerInput, results []ohlcv.PatternResult, specByName map[string]patternSpec) {
	seen := map[string]struct{}{}
	for _, result := range results {
		key := normalizePatternText(result.Name)
		report.Checked++
		if key == "" || strings.TrimSpace(result.Category) == "" || strings.TrimSpace(result.Direction) == "" {
			report.addIssue("result", result.Name, ValidationSeverityError, "missing required result fields")
		}
		report.Checked++
		if _, exists := seen[key]; exists {
			report.addIssue("result", result.Name, ValidationSeverityError, "duplicate pattern result")
		}
		seen[key] = struct{}{}
		report.Checked++
		if !validPatternDirection(result.Direction) {
			report.addIssue("result", result.Name, ValidationSeverityError, "unsupported direction "+result.Direction)
		}
		report.Checked++
		if result.Confidence < 0 || result.Confidence > 1 || !validPatternFloat(result.Confidence) {
			report.addIssue("result", result.Name, ValidationSeverityError, "confidence is outside [0,1]")
		}
		report.Checked++
		if len(result.Evidence) == 0 {
			report.addIssue("result", result.Name, ValidationSeverityError, "missing evidence")
		}
		report.Checked++
		if len(input.Candles) > 0 && (result.StartIndex < 0 || result.EndIndex < result.StartIndex || result.EndIndex >= len(input.Candles)) {
			report.addIssue("result", result.Name, ValidationSeverityError, "invalid candle index window")
		}
		if len(input.Candles) > 0 && result.StartIndex >= 0 && result.EndIndex >= result.StartIndex && result.EndIndex < len(input.Candles) {
			if !result.StartTime.IsZero() && !input.Candles[result.StartIndex].Time.IsZero() && !result.StartTime.Equal(input.Candles[result.StartIndex].Time) {
				report.Checked++
				report.addIssue("result", result.Name, ValidationSeverityWarning, "start time does not match start index candle")
			}
			if !result.EndTime.IsZero() && !input.Candles[result.EndIndex].Time.IsZero() && !result.EndTime.Equal(input.Candles[result.EndIndex].Time) {
				report.Checked++
				report.addIssue("result", result.Name, ValidationSeverityWarning, "end time does not match end index candle")
			}
		}
		if !result.StartTime.IsZero() && !result.EndTime.IsZero() {
			report.Checked++
			if result.StartTime.After(result.EndTime) {
				report.addIssue("result", result.Name, ValidationSeverityError, "start time is after end time")
			}
		}
		if spec, ok := specByName[key]; ok {
			report.Checked++
			if patternRequiresExternalData(spec.Template) {
				report.addIssue("result", result.Name, ValidationSeverityError, "external-data pattern matched with OHLCV-only scanner")
			}
		}
		validatePatternBacktestMetadata(report, result)
		validatePatternDecisionMetadata(report, result)
	}
}

func validatePatternDecisionMetadata(report *ValidationReport, result ohlcv.PatternResult) {
	report.Checked++
	if result.RawConfidence < 0 || result.RawConfidence > 1 || !validPatternFloat(result.RawConfidence) {
		report.addIssue("result", result.Name, ValidationSeverityError, "raw confidence is outside [0,1]")
	}
	report.Checked++
	if result.CalibratedConfidence < 0 || result.CalibratedConfidence > 1 || !validPatternFloat(result.CalibratedConfidence) {
		report.addIssue("result", result.Name, ValidationSeverityError, "calibrated confidence is outside [0,1]")
	}
	report.Checked++
	if result.SignalScore < 0 || result.SignalScore > 1 || !validPatternFloat(result.SignalScore) {
		report.addIssue("result", result.Name, ValidationSeverityError, "signal score is outside [0,1]")
	}
	report.Checked++
	if strings.TrimSpace(result.SignalGroup) == "" {
		report.addIssue("result", result.Name, ValidationSeverityError, "missing signal group")
	}
	if result.Actionable {
		report.Checked++
		if result.Direction != "bullish" && result.Direction != "bearish" {
			report.addIssue("result", result.Name, ValidationSeverityError, "actionable pattern must be directional")
		}
		report.Checked++
		if len(result.RejectionReasons) > 0 {
			report.addIssue("result", result.Name, ValidationSeverityError, "actionable pattern has rejection reasons")
		}
		report.Checked++
		if !result.BacktestReady || !result.Tradeable {
			report.addIssue("result", result.Name, ValidationSeverityError, "actionable pattern is not backtest ready")
		}
		report.Checked++
		if result.Confidence < minActionableConfidence {
			report.addIssue("result", result.Name, ValidationSeverityError, "actionable pattern is below calibrated confidence threshold")
		}
	}
}

func validatePatternBacktestMetadata(report *ValidationReport, result ohlcv.PatternResult) {
	report.Checked++
	if strings.TrimSpace(result.RuleVersion) == "" {
		report.addIssue("result", result.Name, ValidationSeverityError, "missing rule version")
	}
	report.Checked++
	if result.SetupCompleteIndex != result.EndIndex || result.TriggerIndex != result.EndIndex {
		report.addIssue("result", result.Name, ValidationSeverityError, "setup/trigger indexes must align with completed candle")
	}
	report.Checked++
	if strings.TrimSpace(result.Trigger) == "" {
		report.addIssue("result", result.Name, ValidationSeverityError, "missing trigger description")
	}
	if result.Direction != "bullish" && result.Direction != "bearish" {
		return
	}
	report.Checked++
	if !result.Tradeable {
		report.addIssue("result", result.Name, ValidationSeverityError, "directional pattern is not marked tradeable")
	}
	for _, field := range []struct {
		name  string
		value float64
	}{
		{"EntryMin", result.EntryMin},
		{"EntryMax", result.EntryMax},
		{"StopLoss", result.StopLoss},
		{"Target1", result.Target1},
		{"Target2", result.Target2},
		{"InvalidationLevel", result.InvalidationLevel},
		{"RiskRewardRatio", result.RiskRewardRatio},
	} {
		report.Checked++
		if !validPatternFloat(field.value) || field.value <= 0 {
			report.addIssue("result", result.Name, ValidationSeverityError, "invalid backtest field "+field.name)
		}
	}
	report.Checked++
	if result.EntryMin > result.EntryMax {
		report.addIssue("result", result.Name, ValidationSeverityError, "entry range is inverted")
	}
	entry := (result.EntryMin + result.EntryMax) / 2
	if result.Direction == "bullish" {
		report.Checked++
		if result.StopLoss >= entry || result.Target1 <= entry || result.Target2 <= result.Target1 || result.InvalidationLevel != result.StopLoss {
			report.addIssue("result", result.Name, ValidationSeverityError, "bullish trade metadata is not directionally coherent")
		}
		return
	}
	report.Checked++
	if result.StopLoss <= entry || result.Target1 >= entry || result.Target2 >= result.Target1 || result.InvalidationLevel != result.StopLoss {
		report.addIssue("result", result.Name, ValidationSeverityError, "bearish trade metadata is not directionally coherent")
	}
}

func validatePatternScans(report *ValidationReport, scans []ohlcv.PatternScanResult, results []ohlcv.PatternResult, candidates []ohlcv.PatternResult, specByName map[string]patternSpec) {
	resultNames := map[string]struct{}{}
	for _, result := range results {
		resultNames[normalizePatternText(result.Name)] = struct{}{}
	}
	candidateNames := map[string]struct{}{}
	for _, candidate := range candidates {
		candidateNames[normalizePatternText(candidate.Name)] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, scan := range scans {
		key := normalizePatternText(scan.Name)
		report.Checked++
		if key == "" || strings.TrimSpace(scan.Category) == "" || strings.TrimSpace(scan.Direction) == "" || strings.TrimSpace(scan.Source) == "" {
			report.addIssue("scan", scan.Name, ValidationSeverityError, "missing required scan fields")
		}
		report.Checked++
		if _, exists := seen[key]; exists {
			report.addIssue("scan", scan.Name, ValidationSeverityError, "duplicate pattern scan")
		}
		seen[key] = struct{}{}
		report.Checked++
		if !validPatternDirection(scan.Direction) {
			report.addIssue("scan", scan.Name, ValidationSeverityError, "unsupported direction "+scan.Direction)
		}
		report.Checked++
		if scan.Confidence < 0 || scan.Confidence > 1 || !validPatternFloat(scan.Confidence) {
			report.addIssue("scan", scan.Name, ValidationSeverityError, "confidence is outside [0,1]")
		}
		report.Checked++
		if len(scan.Evidence) == 0 {
			report.addIssue("scan", scan.Name, ValidationSeverityError, "missing evidence")
		}
		if scan.Matched {
			report.Checked++
			if _, ok := candidateNames[key]; !ok {
				if _, resultOK := resultNames[key]; !resultOK {
					report.addIssue("scan", scan.Name, ValidationSeverityError, "matched scan has no matching pattern candidate")
				}
			}
			if scan.Actionable {
				report.Checked++
				if _, ok := resultNames[key]; !ok {
					report.addIssue("scan", scan.Name, ValidationSeverityError, "actionable scan has no matching pattern result")
				}
			}
		} else {
			report.Checked++
			if scan.Confidence != 0 {
				report.addIssue("scan", scan.Name, ValidationSeverityError, "unmatched scan has non-zero confidence")
			}
		}
		if spec, ok := specByName[key]; ok {
			report.Checked++
			if patternRequiresExternalData(spec.Template) && scan.Source != "requires_external_data" {
				report.addIssue("scan", scan.Name, ValidationSeverityError, "external-data pattern is not marked as requiring external data")
			}
			report.Checked++
			if !patternRequiresExternalData(spec.Template) && scan.Source == "requires_specific_rule" {
				report.addIssue("scan", scan.Name, ValidationSeverityError, "pattern scan is missing a specific rule")
			}
		}
	}
	for key, spec := range specByName {
		report.Checked++
		if _, ok := seen[key]; !ok {
			report.addIssue("scan", spec.Name, ValidationSeverityError, "registered pattern is missing from scan output")
		}
	}
}

func validatePatternCandles(candles []ohlcv.Candle) ValidationReport {
	var report ValidationReport
	if len(candles) == 0 {
		report.addIssue("ohlcv", "candles", ValidationSeverityError, "empty candle series")
		return report
	}
	for i, candle := range candles {
		name := fmt.Sprintf("candle[%d]", i)
		open, high, low, closePrice, volume := candle.EffectiveOpen(), candle.EffectiveHigh(), candle.EffectiveLow(), candle.EffectiveClose(), candle.EffectiveVolume()
		for field, value := range map[string]float64{"open": open, "high": high, "low": low, "close": closePrice, "volume": volume} {
			report.Checked++
			if !validPatternFloat(value) {
				report.addIssue("ohlcv", name+"."+field, ValidationSeverityError, "value is NaN or infinite")
			}
		}
		report.Checked++
		if high < low {
			report.addIssue("ohlcv", name, ValidationSeverityError, "high is below low")
		}
		report.Checked++
		if high < math.Max(open, closePrice) || low > math.Min(open, closePrice) {
			report.addIssue("ohlcv", name, ValidationSeverityError, "open/close is outside high-low range")
		}
		report.Checked++
		if volume < 0 {
			report.addIssue("ohlcv", name+".volume", ValidationSeverityError, "volume is negative")
		}
		if i > 0 && !candle.Time.IsZero() && !candles[i-1].Time.IsZero() && !candle.Time.After(candles[i-1].Time) {
			report.Checked++
			report.addIssue("ohlcv", name+".time", ValidationSeverityWarning, "timestamp is not strictly increasing")
		}
	}
	return report
}

func validPatternDirection(direction string) bool {
	switch strings.TrimSpace(direction) {
	case "bullish", "bearish", "neutral":
		return true
	default:
		return false
	}
}

func validPatternFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
