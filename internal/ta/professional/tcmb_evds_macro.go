package professional

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/services/tcmb"
	"hissebot/pkg/mathutil"
)

type TCMBMacroExposure struct {
	Profile                 string  `json:"profile,omitempty"`
	InterestRateSensitivity float64 `json:"interest_rate_sensitivity,omitempty"`
	FXSensitivity           float64 `json:"fx_sensitivity,omitempty"`
	CreditSensitivity       float64 `json:"credit_sensitivity,omitempty"`
	InputCostSensitivity    float64 `json:"input_cost_sensitivity,omitempty"`
	SectorPPISeriesCode     string  `json:"sector_ppi_series_code,omitempty"`
}

type TCMBMacroIndicator struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	SeriesCode   string  `json:"series_code"`
	Computed     bool    `json:"computed"`
	LatestDate   string  `json:"latest_date,omitempty"`
	Value        float64 `json:"value,omitempty"`
	Unit         string  `json:"unit,omitempty"`
	ChangeMode   string  `json:"change_mode,omitempty"`
	Change1M     float64 `json:"change_1m,omitempty"`
	Change3M     float64 `json:"change_3m,omitempty"`
	ChangeYoY    float64 `json:"change_yoy,omitempty"`
	AgeDays      int     `json:"age_days,omitempty"`
	Stale        bool    `json:"stale"`
	Contribution float64 `json:"score_contribution,omitempty"`
	Summary      string  `json:"summary,omitempty"`
}

type TCMBMacroContribution struct {
	Factor      string  `json:"factor"`
	Value       float64 `json:"value"`
	Explanation string  `json:"explanation"`
}

type tcmbMacroAnalysis struct {
	Ready           bool
	PointInTimeSafe bool
	ScoreEligible   bool
	QualityScore    float64
	Adjustment      float64
	Regime          string
	Exposure        TCMBMacroExposure
	Indicators      []TCMBMacroIndicator
	Contributions   []TCMBMacroContribution
	ForecastImpact  TCMBMacroForecastImpact
	Warnings        []string
}

type evdsObservation struct {
	Date  time.Time
	Value float64
}

func buildTCMBEVDSContextForSymbol(evdsDir, symbol string, profile CompanyProfile, asOf time.Time) TCMBEVDSContextReport {
	report := buildTCMBEVDSContext(evdsDir)
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	analysis := analyzeTCMBMacro(evdsDir, symbol, profile, asOf, report.LatestFetchAt)
	report.AsOf = asOf
	report.AnalysisReady = analysis.Ready
	report.DataQualityScore = analysis.QualityScore
	report.PointInTimeSafe = analysis.PointInTimeSafe
	report.ScoreEligible = analysis.ScoreEligible
	report.ScoreAdjustment = analysis.Adjustment
	report.Regime = analysis.Regime
	report.Exposure = analysis.Exposure
	report.Indicators = analysis.Indicators
	report.Contributions = analysis.Contributions
	report.ForecastImpact = analysis.ForecastImpact
	report.Warnings = uniqueTCMBStrings(append(report.Warnings, analysis.Warnings...))
	report.DataQualityScore = tcmbMacroQualityCap(report.DataQualityScore, report.Warnings)
	report.ForecastImpact.Confidence = tcmbForecastConfidenceCap(report.ForecastImpact.Confidence, report.Warnings)
	if analysis.Ready {
		report.Summary = fmt.Sprintf(
			"TCMB EVDS makro bağlamı hazır: kalite %.0f/100, rejim %s, hisse skoru düzeltmesi %+.2f puan, fiyat beklenti etkisi %s.",
			report.DataQualityScore,
			analysis.Regime,
			analysis.Adjustment,
			analysis.ForecastImpact.Label,
		)
	}
	return report
}

func analyzeTCMBMacro(evdsDir, symbol string, profile CompanyProfile, asOf, latestFetchAt time.Time) tcmbMacroAnalysis {
	exposure := tcmbExposureForCompany(symbol, profile)
	indicators := []TCMBMacroIndicator{
		loadTCMBIndicator(evdsDir, "policy_rate", "TCMB politika faizi", "bie_bispolfaiz", "TP.BISPOLFAIZ.TUR", asOf, "percentage_point", 120),
		loadTCMBIndicator(evdsDir, "usd_try", "ABD doları döviz alış", "bie_dkdovizgn", "TP.DK.USD.A", asOf, "percent", 10),
		loadTCMBIndicator(evdsDir, "gross_reserves", "TCMB toplam rezervleri", "bie_abres2", "TP.AB.TOPLAM", asOf, "percent", 24),
		loadTCMBIndicator(evdsDir, "total_credit", "Toplam kredi hacmi", "bie_kredi", "TP.KREDI.L001", asOf, "percent", 60),
	}
	indicators = append(indicators, loadInflationExpectationIndicators(evdsDir, asOf)...)
	if exposure.SectorPPISeriesCode != "" {
		indicators = append(indicators, loadTCMBIndicator(
			evdsDir,
			"sector_ppi",
			"Şirket sektörü üretici fiyat endeksi",
			"bie_tufe1yi",
			exposure.SectorPPISeriesCode,
			asOf,
			"percent",
			90,
		))
	}

	coreKeys := map[string]bool{
		"policy_rate":               true,
		"usd_try":                   true,
		"gross_reserves":            true,
		"inflation_expectation_12m": true,
	}
	coreComputed := 0
	quality := 0.0
	for _, indicator := range indicators {
		if !coreKeys[indicator.Key] || !indicator.Computed {
			continue
		}
		coreComputed++
		quality += 20
		if !indicator.Stale {
			quality += 5
		}
	}
	quality = mathutil.Clamp(quality, 0, 100)
	ready := coreComputed >= 3 && quality >= 60
	pointInTimeSafe := latestFetchAt.IsZero() || !asOf.Before(latestFetchAt.AddDate(0, 0, -7))
	scoreEligible := ready && pointInTimeSafe
	contributions, indicators := scoreTCMBMacroIndicators(indicators, exposure)
	adjustment := 0.0
	for _, contribution := range contributions {
		adjustment += contribution.Value
	}
	if scoreEligible {
		adjustment = mathutil.Clamp(adjustment*(quality/100), -8, 8)
	} else {
		adjustment = 0
	}
	regime := "karışık / nötr"
	switch {
	case adjustment <= -2:
		regime = "sıkı ve hisse açısından baskılayıcı"
	case adjustment >= 2:
		regime = "gevşeyen ve hisse açısından destekleyici"
	}
	warnings := []string{}
	for _, indicator := range indicators {
		if indicator.Computed && indicator.Stale {
			warnings = append(warnings, "tcmb_evds_stale_"+indicator.Key)
		}
	}
	if !ready {
		warnings = append(warnings, "tcmb_evds_macro_features_insufficient")
	}
	if !pointInTimeSafe {
		warnings = append(warnings, "tcmb_evds_not_point_in_time_safe_for_as_of")
	}
	forecastImpact := buildTCMBMacroForecastImpact(ready, pointInTimeSafe, scoreEligible, quality, adjustment, regime, contributions, warnings)
	return tcmbMacroAnalysis{
		Ready:           ready,
		PointInTimeSafe: pointInTimeSafe,
		ScoreEligible:   scoreEligible,
		QualityScore:    quality,
		Adjustment:      adjustment,
		Regime:          regime,
		Exposure:        exposure,
		Indicators:      indicators,
		Contributions:   contributions,
		ForecastImpact:  forecastImpact,
		Warnings:        warnings,
	}
}

func buildTCMBMacroForecastImpact(ready, pointInTimeSafe, scoreEligible bool, quality, adjustment float64, regime string, contributions []TCMBMacroContribution, warnings []string) TCMBMacroForecastImpact {
	impact := TCMBMacroForecastImpact{
		Computed:        ready,
		Confidence:      0,
		Horizon:         "1-3 ay makro koşullandırma",
		DecisionUse:     "not_usable",
		ScoreAdjustment: 0,
		Blockers:        []string{},
	}
	if !ready {
		impact.Direction = "unsupported"
		impact.Label = "makro veri yetersiz"
		impact.Severity = "unknown"
		impact.Summary = "TCMB EVDS çekirdek göstergeleri hazır olmadığı için fiyat beklentisine makro etki yazılmadı."
		impact.Blockers = append(impact.Blockers, "tcmb_evds_macro_features_insufficient")
		return impact
	}

	impact.Confidence = tcmbForecastConfidenceCap(mathutil.Clamp(quality, 0, 100), warnings)
	if !pointInTimeSafe {
		impact.Direction = "unsupported"
		impact.Label = "denetim amaçlı"
		impact.Severity = "unknown"
		impact.DecisionUse = "audit_only"
		impact.Confidence = math.Min(impact.Confidence, 30)
		impact.Summary = "EVDS arşivi analiz tarihinden sonra çekildiği için bu makro sinyal geçmiş tahmin/backtest kararına uygulanmadı."
		impact.Blockers = append(impact.Blockers, "tcmb_evds_not_point_in_time_safe_for_as_of")
		return impact
	}

	if !scoreEligible {
		impact.Direction = "unsupported"
		impact.Label = "skora uygun değil"
		impact.Severity = "unknown"
		impact.DecisionUse = "audit_only"
		impact.Summary = "Makro veri hazır olsa da skor/karar etkisi için güvenlik kapısı geçmedi."
		impact.Blockers = append(impact.Blockers, warnings...)
		return impact
	}

	pressure := mathutil.Clamp(adjustment/8*100, -100, 100)
	impact.PressureScore = pressure
	impact.ScoreAdjustment = adjustment
	impact.Direction = tcmbForecastDirection(pressure)
	impact.Label = tcmbForecastLabel(impact.Direction)
	impact.Severity = tcmbForecastSeverity(pressure)
	impact.Drivers = tcmbForecastDrivers(contributions, 4)
	impact.DecisionUse = tcmbForecastDecisionUse(impact.Direction, impact.Severity, pressure)
	impact.Summary = tcmbForecastSummary(impact.Direction, impact.Severity, regime, adjustment)
	if impact.DecisionUse == "blocking_headwind" {
		impact.Blockers = append(impact.Blockers, "macro_severe_headwind")
	}
	return impact
}

func applyTCMBDocumentEvidenceToForecastImpact(report *TCMBEVDSContextReport, docs TCMBContextReport) {
	if report == nil {
		return
	}
	impact := report.ForecastImpact
	impact.DocumentCount = docs.DocumentCount
	impact.DocumentTextIndexPath = docs.TextIndexPath
	impact.DocumentTextUsableCount = docs.TextUsableCount
	impact.DocumentCategories = tcmbDocumentCategoryIDs(docs.Categories)
	if docs.Computed && docs.TextUsableCount > 0 {
		impact.DocumentEvidenceIncluded = true
		if impact.Computed {
			impact.Confidence = mathutil.Clamp(impact.Confidence+3, 0, 100)
			impact.Confidence = tcmbForecastConfidenceCap(impact.Confidence, report.Warnings)
			impact.Drivers = appendUniqueTCMBStrings(impact.Drivers, fmt.Sprintf(
				"tcmb_documents +0.00: %d TCMB dokümanı ve %d okunabilir PDF/HTML text kaydı makro fiyat etkisi için kanıt kapsamına dahil edildi.",
				docs.DocumentCount,
				docs.TextUsableCount,
			))
			if impact.Summary != "" && !strings.Contains(impact.Summary, "TCMB doküman") {
				impact.Summary += " TCMB PDF/HTML text index kapsamı kanıt kapısı olarak dahil edildi."
			}
		}
		report.ForecastImpact = impact
		return
	}

	impact.DocumentEvidenceIncluded = false
	missing := append([]string{}, docs.RequiredCategoriesMissing...)
	for _, item := range docs.RequiredTextCategoriesMissing {
		missing = appendUniqueTCMBStrings(missing, "text:"+item)
	}
	if len(missing) == 0 {
		missing = append(missing, "tcmb_document_text_index_incomplete")
	}
	for _, item := range missing {
		impact.Blockers = appendUniqueTCMBStrings(impact.Blockers, "tcmb_document_evidence_missing:"+item)
	}
	if impact.Computed {
		impact.Confidence = math.Min(impact.Confidence, 50)
		if impact.DecisionUse != "not_usable" {
			impact.DecisionUse = "audit_only"
		}
		if impact.Summary != "" {
			impact.Summary += " TCMB PDF/HTML text kanıtı eksik olduğu için karar/skor etkisi denetim amaçlı sınırlandı."
		}
	} else if impact.DecisionUse == "" {
		impact.DecisionUse = "not_usable"
	}
	report.Warnings = uniqueTCMBStrings(append(report.Warnings, docs.Warnings...))
	report.ForecastImpact = impact
}

func tcmbMacroQualityCap(score float64, warnings []string) float64 {
	return math.Min(score, tcmbForecastConfidenceCap(score, warnings))
}

func tcmbForecastConfidenceCap(confidence float64, warnings []string) float64 {
	capValue := 100.0
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		switch {
		case strings.Contains(warning, "tcmb_evds_catalog_series_missing_files"):
			capValue = math.Min(capValue, 70)
		case strings.Contains(warning, "tcmb_evds_series_partial"):
			capValue = math.Min(capValue, 70)
		case strings.Contains(warning, "tcmb_evds_stale_"):
			capValue = math.Min(capValue, 85)
		case strings.Contains(warning, "tcmb_evds_series_extra_files"):
			capValue = math.Min(capValue, 95)
		}
	}
	return mathutil.Clamp(math.Min(confidence, capValue), 0, 100)
}

func tcmbDocumentCategoryIDs(categories []TCMBCategoryStat) []string {
	out := make([]string, 0, len(categories))
	for _, category := range categories {
		if strings.TrimSpace(category.ID) != "" {
			out = append(out, category.ID)
		}
	}
	sort.Strings(out)
	return out
}

func tcmbForecastDirection(pressure float64) string {
	switch {
	case pressure >= 25:
		return "positive"
	case pressure <= -25:
		return "negative"
	default:
		return "neutral"
	}
}

func tcmbForecastLabel(direction string) string {
	switch direction {
	case "positive":
		return "pozitif makro rüzgar"
	case "negative":
		return "negatif makro rüzgar"
	case "neutral":
		return "nötr makro rüzgar"
	default:
		return "makro etki yok"
	}
}

func tcmbForecastSeverity(pressure float64) string {
	absPressure := math.Abs(pressure)
	switch {
	case absPressure >= 70:
		return "high"
	case absPressure >= 35:
		return "moderate"
	default:
		return "low"
	}
}

func tcmbForecastDecisionUse(direction, severity string, pressure float64) string {
	if direction == "negative" && (severity == "high" || pressure <= -55) {
		return "blocking_headwind"
	}
	if severity == "low" {
		return "context_only"
	}
	return "score_and_gate_input"
}

func tcmbForecastSummary(direction, severity, regime string, adjustment float64) string {
	switch direction {
	case "positive":
		return fmt.Sprintf("Makro rejim fiyat beklentisini destekliyor; şiddet %s, skor etkisi %+.2f puan. Rejim: %s.", severity, adjustment, regime)
	case "negative":
		return fmt.Sprintf("Makro rejim fiyat beklentisini baskılıyor; şiddet %s, skor etkisi %+.2f puan. Rejim: %s.", severity, adjustment, regime)
	default:
		return fmt.Sprintf("Makro rejim belirgin yön üretmiyor; şiddet %s, skor etkisi %+.2f puan. Rejim: %s.", severity, adjustment, regime)
	}
}

func tcmbForecastDrivers(contributions []TCMBMacroContribution, limit int) []string {
	if limit <= 0 || len(contributions) == 0 {
		return nil
	}
	sorted := append([]TCMBMacroContribution{}, contributions...)
	sort.Slice(sorted, func(i, j int) bool {
		return math.Abs(sorted[i].Value) > math.Abs(sorted[j].Value)
	})
	drivers := []string{}
	for _, item := range sorted {
		if len(drivers) >= limit {
			break
		}
		drivers = append(drivers, fmt.Sprintf("%s %+.2f: %s", item.Factor, item.Value, item.Explanation))
	}
	return drivers
}

func appendUniqueTCMBStrings(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return items
		}
	}
	return append(items, value)
}

func loadTCMBIndicator(evdsDir, key, name, dataGroup, seriesCode string, asOf time.Time, changeMode string, staleAfterDays int) TCMBMacroIndicator {
	indicator := TCMBMacroIndicator{
		Key:        key,
		Name:       name,
		SeriesCode: seriesCode,
		ChangeMode: changeMode,
	}
	path := filepath.Join(evdsDir, "series", dataGroup, strings.ReplaceAll(seriesCode, ".", "_")+".json")
	dataset, err := loadEVDSSeriesDataset(path)
	if err != nil {
		indicator.Summary = err.Error()
		return indicator
	}
	observations := validEVDSObservations(dataset, asOf)
	if len(observations) == 0 {
		indicator.Summary = "Analiz tarihi itibarıyla kullanılabilir gözlem yok."
		return indicator
	}
	latest := observations[len(observations)-1]
	indicator.Computed = true
	indicator.LatestDate = latest.Date.Format("2006-01-02")
	indicator.Value = latest.Value
	indicator.Unit = firstNonEmptyString(dataset.DataGroup.Unit, dataset.DataGroup.UnitENG)
	indicator.AgeDays = calendarAgeDays(asOf, latest.Date)
	indicator.Stale = staleAfterDays > 0 && indicator.AgeDays > staleAfterDays
	indicator.Change1M = evdsChangeAt(observations, latest, latest.Date.AddDate(0, -1, 0), changeMode)
	indicator.Change3M = evdsChangeAt(observations, latest, latest.Date.AddDate(0, -3, 0), changeMode)
	indicator.ChangeYoY = evdsChangeAt(observations, latest, latest.Date.AddDate(-1, 0, 0), changeMode)
	indicator.Summary = fmt.Sprintf("%s tarihinde %.4f; 3 aylık değişim %+.2f.", indicator.LatestDate, indicator.Value, indicator.Change3M)
	return indicator
}

func loadEVDSSeriesDataset(path string) (tcmb.EVDSSeriesDataset, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return tcmb.EVDSSeriesDataset{}, fmt.Errorf("EVDS seri dosyası okunamadı: %s", path)
	}
	var dataset tcmb.EVDSSeriesDataset
	if err := json.Unmarshal(raw, &dataset); err != nil {
		return tcmb.EVDSSeriesDataset{}, fmt.Errorf("EVDS seri dosyası ayrıştırılamadı: %s", path)
	}
	return dataset, nil
}

func validEVDSObservations(dataset tcmb.EVDSSeriesDataset, asOf time.Time) []evdsObservation {
	cutoff := endOfUTCDay(asOf)
	observations := make([]evdsObservation, 0, len(dataset.Points))
	for _, point := range dataset.Points {
		date, ok := evdsPointDate(point)
		if !ok || date.After(cutoff) {
			continue
		}
		value, ok := evdsPointNumericValue(point)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		observations = append(observations, evdsObservation{Date: date, Value: value})
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].Date.Before(observations[j].Date) })
	return observations
}

func evdsPointNumericValue(point tcmb.EVDSPoint) (float64, bool) {
	if point.Value != nil {
		return *point.Value, true
	}
	if point.RawValue == nil || strings.TrimSpace(*point.RawValue) == "" {
		return 0, false
	}
	parsed, err := tcmb.ParseEVDSNumber(*point.RawValue)
	return parsed, err == nil
}

func evdsPointDate(point tcmb.EVDSPoint) (time.Time, bool) {
	for _, value := range []string{point.Date, point.DisplayDate} {
		value = strings.TrimSpace(value)
		for _, layout := range []string{"2006-01-02", "02-01-2006", "02.01.2006", "2006-01", "01-2006", "2006"} {
			if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
				return parsed, true
			}
		}
	}
	if point.UnixTime != nil && *point.UnixTime > 0 {
		return time.Unix(*point.UnixTime, 0).UTC(), true
	}
	return time.Time{}, false
}

func evdsChangeAt(observations []evdsObservation, latest evdsObservation, target time.Time, mode string) float64 {
	previous, ok := observationAtOrBefore(observations, target)
	if !ok {
		return 0
	}
	if mode == "percentage_point" {
		return latest.Value - previous.Value
	}
	if previous.Value == 0 {
		return 0
	}
	return (latest.Value - previous.Value) / previous.Value * 100
}

func observationAtOrBefore(observations []evdsObservation, target time.Time) (evdsObservation, bool) {
	index := sort.Search(len(observations), func(i int) bool { return observations[i].Date.After(target) })
	if index == 0 {
		return evdsObservation{}, false
	}
	return observations[index-1], true
}

var expectationRangePattern = regexp.MustCompile(`\((>=)?([0-9]+(?:\.[0-9]+)?)(?:-([0-9]+(?:\.[0-9]+)?))?\)`)

type expectationAggregate struct {
	Probability float64
	Weighted    float64
	WeightedSq  float64
}

func loadInflationExpectationIndicators(evdsDir string, asOf time.Time) []TCMBMacroIndicator {
	paths, _ := filepath.Glob(filepath.Join(evdsDir, "series", "bie_bekodtufeyeni", "*.json"))
	byHorizon := map[int]map[time.Time]*expectationAggregate{12: {}, 24: {}}
	for _, path := range paths {
		dataset, err := loadEVDSSeriesDataset(path)
		if err != nil {
			continue
		}
		horizon, midpoint, ok := expectationSeriesBin(dataset.Series.SeriesName)
		if !ok {
			continue
		}
		for _, observation := range validEVDSObservations(dataset, asOf) {
			period := time.Date(observation.Date.Year(), observation.Date.Month(), 1, 0, 0, 0, 0, time.UTC)
			aggregate := byHorizon[horizon][period]
			if aggregate == nil {
				aggregate = &expectationAggregate{}
				byHorizon[horizon][period] = aggregate
			}
			aggregate.Probability += observation.Value
			aggregate.Weighted += observation.Value * midpoint
			aggregate.WeightedSq += observation.Value * midpoint * midpoint
		}
	}
	indicators := []TCMBMacroIndicator{}
	for _, horizon := range []int{12, 24} {
		observations := []evdsObservation{}
		dispersionByDate := map[time.Time]float64{}
		for date, aggregate := range byHorizon[horizon] {
			if aggregate.Probability < 80 || aggregate.Probability > 120 {
				continue
			}
			mean := aggregate.Weighted / aggregate.Probability
			variance := math.Max(aggregate.WeightedSq/aggregate.Probability-mean*mean, 0)
			observations = append(observations, evdsObservation{Date: date, Value: mean})
			dispersionByDate[date] = math.Sqrt(variance)
		}
		sort.Slice(observations, func(i, j int) bool { return observations[i].Date.Before(observations[j].Date) })
		indicator := TCMBMacroIndicator{
			Key:        fmt.Sprintf("inflation_expectation_%dm", horizon),
			Name:       fmt.Sprintf("%d ay sonrası enflasyon beklentisi", horizon),
			SeriesCode: fmt.Sprintf("TP.BEKODTUFEYENI.BT%d-*", map[int]int{12: 2, 24: 62}[horizon]),
			Unit:       "yüzde",
			ChangeMode: "percentage_point",
		}
		if len(observations) == 0 {
			indicator.Summary = "Beklenti dağılımı hesaplanamadı."
			indicators = append(indicators, indicator)
			continue
		}
		latest := observations[len(observations)-1]
		indicator.Computed = true
		indicator.LatestDate = latest.Date.Format("2006-01")
		indicator.Value = latest.Value
		indicator.AgeDays = calendarAgeDays(asOf, latest.Date)
		indicator.Stale = indicator.AgeDays > 60
		indicator.Change1M = evdsChangeAt(observations, latest, latest.Date.AddDate(0, -1, 0), "percentage_point")
		indicator.Change3M = evdsChangeAt(observations, latest, latest.Date.AddDate(0, -3, 0), "percentage_point")
		indicator.ChangeYoY = evdsChangeAt(observations, latest, latest.Date.AddDate(-1, 0, 0), "percentage_point")
		indicator.Summary = fmt.Sprintf(
			"Yaklaşık ortalama %.2f%%, dağılım sapması %.2f puan, 3 aylık değişim %+.2f puan.",
			latest.Value,
			dispersionByDate[latest.Date],
			indicator.Change3M,
		)
		indicators = append(indicators, indicator)
	}
	return indicators
}

func expectationSeriesBin(name string) (int, float64, bool) {
	lower := strings.ToLower(name)
	horizon := 0
	switch {
	case strings.Contains(lower, "12 ay"):
		horizon = 12
	case strings.Contains(lower, "24 ay"):
		horizon = 24
	default:
		return 0, 0, false
	}
	match := expectationRangePattern.FindStringSubmatch(name)
	if len(match) != 4 {
		return 0, 0, false
	}
	lowerBound, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return 0, 0, false
	}
	upperBound := lowerBound + 1
	if match[3] != "" {
		upperBound, err = strconv.ParseFloat(match[3], 64)
		if err != nil {
			return 0, 0, false
		}
	}
	return horizon, (lowerBound + upperBound) / 2, true
}

func scoreTCMBMacroIndicators(indicators []TCMBMacroIndicator, exposure TCMBMacroExposure) ([]TCMBMacroContribution, []TCMBMacroIndicator) {
	contributions := []TCMBMacroContribution{}
	for i := range indicators {
		indicator := &indicators[i]
		if !indicator.Computed || indicator.Stale {
			continue
		}
		value := 0.0
		explanation := ""
		switch indicator.Key {
		case "policy_rate":
			levelPressure := mathutil.Clamp((indicator.Value-25)/10, 0, 2.5)
			momentum := mathutil.Clamp(-indicator.Change3M/2, -1.5, 1.5)
			value = (momentum - levelPressure) * exposure.InterestRateSensitivity
			explanation = "Faiz seviyesi ve üç aylık yön, şirketin faiz hassasiyetiyle ağırlıklandırıldı."
		case "usd_try":
			value = mathutil.Clamp(indicator.Change3M/10, -2, 2) * exposure.FXSensitivity
			explanation = "Kur değişimi, sektörün döviz gelir/girdi hassasiyetiyle ağırlıklandırıldı."
		case "gross_reserves":
			value = mathutil.Clamp(indicator.Change3M/10, -1.5, 1.5) * 0.75
			explanation = "Rezerv eğilimi tüm BIST hisseleri için sınırlı sistemik risk katkısıdır."
		case "total_credit":
			value = mathutil.Clamp(indicator.Change3M/10, -1.25, 1.25) * exposure.CreditSensitivity
			explanation = "Kredi büyümesi, sektörün finansman ve iç talep hassasiyetiyle ağırlıklandırıldı."
		case "inflation_expectation_12m":
			levelPressure := mathutil.Clamp((indicator.Value-20)/10, 0, 1.5)
			trendPressure := mathutil.Clamp(indicator.Change3M/3, -1, 1)
			value = -(levelPressure + trendPressure) * 0.65
			explanation = "Yüksek veya yükselen 12 aylık enflasyon beklentisi değerleme baskısı olarak ele alındı."
		case "sector_ppi":
			costPressure := mathutil.Clamp((indicator.ChangeYoY-20)/15, -1, 2)
			value = -costPressure * exposure.InputCostSensitivity
			explanation = "Sektörel üretici fiyatı, şirketin girdi maliyeti hassasiyetiyle ağırlıklandırıldı."
		}
		value = mathutil.Clamp(value, -3, 3)
		if math.Abs(value) < 0.01 {
			continue
		}
		indicator.Contribution = value
		contributions = append(contributions, TCMBMacroContribution{Factor: indicator.Key, Value: value, Explanation: explanation})
	}
	return contributions, indicators
}

func tcmbExposureForCompany(symbol string, profile CompanyProfile) TCMBMacroExposure {
	classification := normalizeTCMBText(strings.Join([]string{profile.Sector, profile.Industry, profile.PeerGroup}, " "))
	exposure := TCMBMacroExposure{
		Profile:                 firstNonEmptyString(profile.Industry, profile.Sector, "genel BIST şirketi"),
		InterestRateSensitivity: 0.35,
		InputCostSensitivity:    0.45,
	}
	switch {
	case strings.Contains(classification, "banka"):
		exposure.InterestRateSensitivity = 0.65
		exposure.CreditSensitivity = 0.9
		exposure.InputCostSensitivity = 0
	case strings.Contains(classification, "gayrimenkul yatirim ortakligi"):
		exposure.InterestRateSensitivity = 1
		exposure.CreditSensitivity = 0.65
		exposure.InputCostSensitivity = 0.25
	case strings.Contains(classification, "elektrik gaz"):
		exposure.InterestRateSensitivity = 0.65
		exposure.FXSensitivity = -0.3
		exposure.InputCostSensitivity = 0.7
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T120"
	case strings.Contains(classification, "savunma"):
		exposure.InterestRateSensitivity = 0.2
		exposure.FXSensitivity = 0.5
		exposure.InputCostSensitivity = 0.45
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T86"
	case strings.Contains(classification, "gida") || strings.Contains(classification, "icecek"):
		exposure.InterestRateSensitivity = 0.35
		exposure.FXSensitivity = -0.15
		exposure.InputCostSensitivity = 0.8
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T26"
	case strings.Contains(classification, "kimya") || strings.Contains(classification, "plastik"):
		exposure.FXSensitivity = -0.35
		exposure.InputCostSensitivity = 0.8
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T52"
	case strings.Contains(classification, "tekstil"):
		exposure.FXSensitivity = 0.2
		exposure.InputCostSensitivity = 0.7
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T30"
	case strings.Contains(classification, "ana metal"):
		exposure.FXSensitivity = 0.2
		exposure.InputCostSensitivity = 0.75
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T73"
	case strings.Contains(classification, "metal esya") || strings.Contains(classification, "makine") || strings.Contains(classification, "elektrikli cihaz"):
		exposure.FXSensitivity = -0.2
		exposure.InputCostSensitivity = 0.7
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T93"
	case strings.Contains(classification, "teknoloji") || strings.Contains(classification, "bilisim"):
		exposure.FXSensitivity = 0.25
		exposure.InputCostSensitivity = 0.45
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T86"
	}
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "AEFES":
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T26"
	case "ALCAR":
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T93"
	case "ASELS":
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T86"
	case "AKSEN", "AKSUE", "AKFYE", "ALFAS":
		exposure.SectorPPISeriesCode = "TP.TUFE1YI.T120"
	}
	return exposure
}

func normalizeTCMBText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.NewReplacer(
		"ı", "i", "ç", "c", "ğ", "g", "ö", "o", "ş", "s", "ü", "u",
	).Replace(value)
}

func calendarAgeDays(asOf, observation time.Time) int {
	age := endOfUTCDay(asOf).Sub(observation.UTC())
	if age <= 0 {
		return 0
	}
	return int(age.Hours() / 24)
}

func endOfUTCDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 23, 59, 59, 0, time.UTC)
}

func uniqueTCMBStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
