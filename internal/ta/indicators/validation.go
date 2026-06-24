package indicators

import (
	"fmt"
	"math"
	"reflect"
	"sort"
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
		return fmt.Sprintf("validated %d indicator checks with no issues", r.Checked)
	}
	parts := []string{fmt.Sprintf("validated %d indicator checks: %d error(s), %d warning(s)", r.Checked, errors, warnings)}
	issues := prioritizedValidationIssues(r.Issues)
	limit := minInt(len(issues), 5)
	for i := 0; i < limit; i++ {
		issue := issues[i]
		parts = append(parts, fmt.Sprintf("%s/%s: %s", issue.Scope, issue.Name, issue.Message))
	}
	if len(issues) > limit {
		parts = append(parts, fmt.Sprintf("... %d more issue(s)", len(issues)-limit))
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

func prioritizedValidationIssues(issues []ValidationIssue) []ValidationIssue {
	out := append([]ValidationIssue{}, issues...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity == ValidationSeverityError
		}
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func ValidateIndicatorSystem(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot, output ScannerOutput) ValidationReport {
	var report ValidationReport
	report.Merge(ValidateOHLCVCandles(candles))
	report.Merge(ValidateSnapshot(candles, snapshot))
	report.Merge(ValidateIndicatorCatalog())
	report.Merge(ValidateIndicatorScan(output))
	lastVolume := 0.0
	if len(candles) > 0 {
		lastVolume = candles[len(candles)-1].EffectiveVolume()
	}
	report.Merge(ValidateIndicatorSignalConsistency(ScannerInput{
		Candles:    candles,
		Snapshot:   snapshot,
		LastVolume: lastVolume,
	}, output))
	return report
}

func ValidateOHLCVCandles(candles []ohlcv.Candle) ValidationReport {
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
			if !validFloat(value) {
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

func ValidateSnapshot(candles []ohlcv.Candle, snapshot ohlcv.IndicatorSnapshot) ValidationReport {
	var report ValidationReport
	validateSnapshotFinite(&report, snapshot)
	validateSnapshotInvariants(&report, snapshot)
	if len(candles) > 0 {
		expected, err := Snapshot(candles)
		if err != nil {
			report.addIssue("snapshot", "recompute", ValidationSeverityError, err.Error())
		} else {
			compareSnapshot(&report, snapshot, expected)
		}
	}
	return report
}

func ValidateIndicatorCatalog() ValidationReport {
	var report ValidationReport
	specs := registeredIndicatorSpecs()
	if len(specs) == 0 {
		report.addIssue("catalog", "indicators", ValidationSeverityError, "no registered indicator specs")
		return report
	}
	exact := exactIndicatorFormulaNames()
	seen := map[string]struct{}{}
	for _, spec := range specs {
		key := normalizeIndicatorText(spec.Name)
		exactKey := strings.ToLower(strings.TrimSpace(spec.Name))
		report.Checked++
		if key == "" {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "empty indicator name")
		}
		report.Checked++
		if _, exists := seen[exactKey]; exists {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "duplicate indicator name")
		}
		seen[exactKey] = struct{}{}
		report.Checked++
		if strings.TrimSpace(spec.Category) == "" || strings.TrimSpace(spec.Template) == "" {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "missing category or template")
		}
		report.Checked++
		if spec.Confidence < 0 || spec.Confidence > 1 {
			report.addIssue("catalog", spec.Name, ValidationSeverityError, "confidence is outside [0,1]")
		}
		if spec.Template == "generic" {
			report.Checked++
			report.addIssue("catalog", spec.Name, ValidationSeverityWarning, "generic indicator template has no exact formula and must stay proxy-only")
		}
		if !isExternalOnly(spec) {
			if _, ok := exact[key]; !ok {
				report.Checked++
				report.addIssue("catalog", spec.Name, ValidationSeverityWarning, "no exact OHLCV formula is registered; scanner result must stay proxy-only")
			}
		}
	}
	return report
}

func ValidateIndicatorScan(output ScannerOutput) ValidationReport {
	var report ValidationReport
	report.Checked++
	if output.DetectorCount < 0 || output.ScannedCount < 0 || output.ComputedCount < 0 || output.SignalCount < 0 {
		report.addIssue("scan", "counts", ValidationSeverityError, "negative scanner count")
	}
	report.Checked++
	if output.DetectorCount != 0 && output.ScannedCount != output.DetectorCount {
		report.addIssue("scan", "counts", ValidationSeverityError, "scanned count does not match detector count")
	}
	report.Checked++
	if len(output.Indicators) > output.ScannedCount {
		report.addIssue("scan", "counts", ValidationSeverityError, "result count exceeds scanned count")
	}
	seen := map[string]struct{}{}
	computed, signals := 0, 0
	for _, result := range output.Indicators {
		name := strings.TrimSpace(result.Name)
		report.Checked++
		if name == "" || strings.TrimSpace(result.Category) == "" || strings.TrimSpace(result.Signal) == "" || strings.TrimSpace(result.Source) == "" {
			report.addIssue("scan", name, ValidationSeverityError, "missing required result fields")
		}
		exactKey := strings.ToLower(strings.TrimSpace(result.Name))
		report.Checked++
		if _, exists := seen[exactKey]; exists {
			report.addIssue("scan", result.Name, ValidationSeverityError, "duplicate indicator result")
		}
		seen[exactKey] = struct{}{}
		report.Checked++
		if !validFloat(result.Value) {
			report.addIssue("scan", result.Name, ValidationSeverityError, "value is NaN or infinite")
		}
		report.Checked++
		if result.Confidence < 0 || result.Confidence > 1 {
			report.addIssue("scan", result.Name, ValidationSeverityError, "confidence is outside [0,1]")
		}
		if result.Computed {
			computed++
			report.Checked++
			if result.Signal == "proxy_only" || result.Signal == "requires_external_data" || result.Source == proxySource || result.Source == "unavailable" {
				report.addIssue("scan", result.Name, ValidationSeverityError, "computed result is marked as proxy/external")
			}
			if result.Confidence >= 0.5 && result.Signal != "neutral" && result.Signal != "info" {
				signals++
			}
		} else {
			report.Checked++
			if result.Signal != "proxy_only" && result.Signal != "requires_external_data" && result.Signal != "insufficient_data" && result.Signal != algorithmRequiredSource {
				report.addIssue("scan", result.Name, ValidationSeverityError, "non-computed result has unsupported signal")
			}
			report.Checked++
			if result.Confidence != 0 {
				report.addIssue("scan", result.Name, ValidationSeverityError, "non-computed result has non-zero confidence")
			}
		}
		report.Checked++
		if len(result.Evidence) == 0 {
			report.addIssue("scan", result.Name, ValidationSeverityError, "missing evidence")
		}
	}
	report.Checked++
	if output.ComputedCount != computed {
		report.addIssue("scan", "computed_count", ValidationSeverityError, "computed count does not match results")
	}
	report.Checked++
	if output.SignalCount != signals {
		report.addIssue("scan", "signal_count", ValidationSeverityError, "signal count does not match results")
	}
	return report
}

func ValidateIndicatorSignalConsistency(input ScannerInput, output ScannerOutput) ValidationReport {
	var report ValidationReport
	if input.LastVolume == 0 && len(input.Candles) > 0 {
		input.LastVolume = input.Candles[len(input.Candles)-1].EffectiveVolume()
	}
	for _, result := range output.Indicators {
		if !result.Computed {
			continue
		}
		name := normalizeIndicatorText(result.Name)
		expectedSignal := ""
		switch {
		case isMACDIndicator(name):
			expectedSignal, _, _ = macdIndicatorSignal(input.Snapshot, result.Confidence)
		case isVolumeParticipationIndicator(name):
			expectedSignal, _, _ = volumeParticipationSignal(input.LastVolume, input.Snapshot.VolumeSMA20, result.Confidence)
		case isIchimokuStateIndicator(name):
			expectedSignal, _, _ = signedStateSignal(result.Value, result.Confidence, "", "", "")
		default:
			continue
		}
		report.Checked++
		if expectedSignal != "" && result.Signal != expectedSignal {
			report.addIssue("scan.consistency", result.Name, ValidationSeverityError, fmt.Sprintf("signal %q contradicts snapshot-derived %q", result.Signal, expectedSignal))
		}
	}
	return report
}

func validateSnapshotFinite(report *ValidationReport, snapshot ohlcv.IndicatorSnapshot) {
	value := reflect.ValueOf(snapshot)
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		name := typ.Field(i).Name
		switch field.Kind() {
		case reflect.Float64:
			report.Checked++
			if !validFloat(field.Float()) {
				report.addIssue("snapshot", name, ValidationSeverityError, "value is NaN or infinite")
			}
		case reflect.Map:
			iter := field.MapRange()
			for iter.Next() {
				report.Checked++
				entryName := fmt.Sprintf("%s[%s]", name, iter.Key().String())
				if !validFloat(iter.Value().Float()) {
					report.addIssue("snapshot", entryName, ValidationSeverityError, "value is NaN or infinite")
				}
			}
		}
	}
}

func validateSnapshotInvariants(report *ValidationReport, s ohlcv.IndicatorSnapshot) {
	checkRange(report, "snapshot", "RSI14", s.RSI14, 0, 100)
	checkRange(report, "snapshot", "ADX14", s.ADX14, 0, 100)
	checkRange(report, "snapshot", "MFI14", s.MFI14, 0, 100)
	checkRange(report, "snapshot", "StochRSIK", s.StochRSIK, 0, 100)
	checkRange(report, "snapshot", "StochRSID", s.StochRSID, 0, 100)
	checkRange(report, "snapshot", "StochasticK", s.StochasticK, 0, 100)
	checkRange(report, "snapshot", "StochasticD", s.StochasticD, 0, 100)
	checkRange(report, "snapshot", "WilliamsR14", s.WilliamsR14, -100, 0)
	checkRange(report, "snapshot", "ChaikinMoneyFlow20", s.ChaikinMoneyFlow20, -1, 1)
	checkNonNegative(report, "snapshot", "ATR14", s.ATR14)
	checkNonNegative(report, "snapshot", "VolumeSMA20", s.VolumeSMA20)
	checkOrder(report, "snapshot", "BollingerBands", []float64{s.BollingerUpper, s.BollingerMiddle, s.BollingerLower}, []string{"upper", "middle", "lower"})
	checkOrder(report, "snapshot", "KeltnerChannel", []float64{s.KeltnerUpper, s.KeltnerMiddle, s.KeltnerLower}, []string{"upper", "middle", "lower"})
	checkOrder(report, "snapshot", "DonchianChannel", []float64{s.DonchianUpper, s.DonchianLower}, []string{"upper", "lower"})
	checkApprox(report, "snapshot", "MACDHistogram", s.MACDHistogram, s.MACD-s.MACDSignal)
	checkOrder(report, "snapshot", "PivotResistance", []float64{s.PivotR2, s.PivotR1, s.PivotPoint}, []string{"R2", "R1", "pivot"})
	checkOrder(report, "snapshot", "PivotSupport", []float64{s.PivotPoint, s.PivotS1, s.PivotS2}, []string{"pivot", "S1", "S2"})
	checkAdditionalIndicatorInvariants(report, s.AdditionalIndicators)
	for _, field := range []struct {
		name  string
		value float64
	}{
		{"IchimokuCloudTrend", s.IchimokuCloudTrend},
		{"IchimokuKumoTwist", s.IchimokuKumoTwist},
		{"IchimokuTKCross", s.IchimokuTKCross},
		{"IchimokuPriceCloudBreakout", s.IchimokuPriceCloudBreakout},
	} {
		checkRange(report, "snapshot", field.name, field.value, -1, 1)
		report.Checked++
		if !almostSame(field.value, math.Round(field.value)) {
			report.addIssue("snapshot", field.name, ValidationSeverityError, "state value must be -1, 0 or 1")
		}
	}
	checkFibonacciLevels(report, s.FibonacciLevels)
}

func checkAdditionalIndicatorInvariants(report *ValidationReport, values map[string]float64) {
	for _, item := range []struct {
		name string
		min  float64
		max  float64
	}{
		{"Aroon Up", 0, 100},
		{"Aroon Down", 0, 100},
		{"Connors RSI", 0, 100},
		{"Money Flow Index", 0, 100},
		{"Schaff Trend Cycle", 0, 100},
		{"Stochastic Oscillator K", 0, 100},
		{"Stochastic Oscillator D", 0, 100},
		{"Ultimate Oscillator", 0, 100},
		{"Williams %R", -100, 0},
		{"Stochastic Momentum Index", -100, 100},
		{"Stochastic Momentum Signal", -100, 100},
	} {
		if value, ok := values[item.name]; ok {
			checkRange(report, "snapshot.AdditionalIndicators", item.name, value, item.min, item.max)
		}
	}
}

func compareSnapshot(report *ValidationReport, got, want ohlcv.IndicatorSnapshot) {
	gotValue := reflect.ValueOf(got)
	wantValue := reflect.ValueOf(want)
	typ := gotValue.Type()
	for i := 0; i < gotValue.NumField(); i++ {
		name := typ.Field(i).Name
		gotField := gotValue.Field(i)
		wantField := wantValue.Field(i)
		switch gotField.Kind() {
		case reflect.Float64:
			checkApprox(report, "snapshot.recompute", name, gotField.Float(), wantField.Float())
		case reflect.Map:
			compareFloatMaps(report, name, gotField, wantField)
		}
	}
}

func compareFloatMaps(report *ValidationReport, name string, got, want reflect.Value) {
	report.Checked++
	if got.Len() != want.Len() {
		report.addIssue("snapshot.recompute", name, ValidationSeverityError, "map length differs from recomputed snapshot")
	}
	iter := want.MapRange()
	for iter.Next() {
		key := iter.Key()
		gotValue := got.MapIndex(key)
		entryName := fmt.Sprintf("%s[%s]", name, key.String())
		report.Checked++
		if !gotValue.IsValid() {
			report.addIssue("snapshot.recompute", entryName, ValidationSeverityError, "missing recomputed map key")
			continue
		}
		if !almostSame(gotValue.Float(), iter.Value().Float()) {
			report.addIssue("snapshot.recompute", entryName, ValidationSeverityError, fmt.Sprintf("value %.8f does not match recomputed %.8f", gotValue.Float(), iter.Value().Float()))
		}
	}
}

func checkRange(report *ValidationReport, scope, name string, value, minValue, maxValue float64) {
	report.Checked++
	if value < minValue-1e-9 || value > maxValue+1e-9 {
		report.addIssue(scope, name, ValidationSeverityError, fmt.Sprintf("value %.8f is outside [%.2f, %.2f]", value, minValue, maxValue))
	}
}

func checkNonNegative(report *ValidationReport, scope, name string, value float64) {
	report.Checked++
	if value < -1e-9 {
		report.addIssue(scope, name, ValidationSeverityError, fmt.Sprintf("value %.8f is negative", value))
	}
}

func checkOrder(report *ValidationReport, scope, name string, values []float64, labels []string) {
	for i := 1; i < len(values); i++ {
		report.Checked++
		if values[i-1]+1e-9 < values[i] {
			report.addIssue(scope, name, ValidationSeverityError, fmt.Sprintf("%s %.8f is below %s %.8f", labels[i-1], values[i-1], labels[i], values[i]))
		}
	}
}

func checkApprox(report *ValidationReport, scope, name string, got, want float64) {
	report.Checked++
	if !almostSame(got, want) {
		report.addIssue(scope, name, ValidationSeverityError, fmt.Sprintf("value %.8f does not match expected %.8f", got, want))
	}
}

func checkFibonacciLevels(report *ValidationReport, levels map[string]float64) {
	order := []string{"0.236", "0.382", "0.500", "0.618", "0.786"}
	for _, key := range order {
		report.Checked++
		if _, ok := levels[key]; !ok {
			report.addIssue("snapshot", "FibonacciLevels", ValidationSeverityError, "missing level "+key)
		}
	}
	for i := 1; i < len(order); i++ {
		prev, okPrev := levels[order[i-1]]
		next, okNext := levels[order[i]]
		report.Checked++
		if okPrev && okNext && prev+1e-9 < next {
			report.addIssue("snapshot", "FibonacciLevels", ValidationSeverityError, "retracement levels are not monotonic")
		}
	}
}

func exactIndicatorFormulaNames() map[string]struct{} {
	sample := []ohlcv.Candle{{Open: 1, High: 1, Low: 1, Close: 1, Volume: 1}}
	snapshot, _ := Snapshot(sample)
	input := ScannerInput{Candles: sample, Snapshot: snapshot, LastClose: 1, LastVolume: 1}
	specs := registeredIndicatorSpecs()
	out := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		value := valueForIndicator(input, spec)
		if !value.computed || value.source == "" || value.source == proxySource {
			continue
		}
		out[normalizeIndicatorText(spec.Name)] = struct{}{}
	}
	return out
}

func validFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func almostSame(got, want float64) bool {
	tol := 1e-8 * math.Max(1, math.Max(math.Abs(got), math.Abs(want)))
	return math.Abs(got-want) <= tol
}
