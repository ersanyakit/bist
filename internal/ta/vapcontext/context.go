package vapcontext

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type FreeFloatObservation struct {
	Date     time.Time `json:"-"`
	DateText string    `json:"date"`
	Shares   float64   `json:"free_float_shares"`
	Capital  float64   `json:"issuer_capital"`
	RatioPct float64   `json:"free_float_ratio_pct"`
}

type FreeFloatReport struct {
	Computed          bool                   `json:"computed"`
	SourcePath        string                 `json:"source_path,omitempty"`
	AsOf              time.Time              `json:"as_of,omitempty"`
	LatestDate        string                 `json:"latest_date,omitempty"`
	Observations      int                    `json:"observations"`
	FreeFloatShares   float64                `json:"free_float_shares,omitempty"`
	IssuerCapital     float64                `json:"issuer_capital,omitempty"`
	FreeFloatRatioPct float64                `json:"free_float_ratio_pct,omitempty"`
	RatioChange1DPP   float64                `json:"ratio_change_1d_pp,omitempty"`
	RatioChange20DPP  float64                `json:"ratio_change_20d_pp,omitempty"`
	RatioChange60DPP  float64                `json:"ratio_change_60d_pp,omitempty"`
	SharesChange20Pct float64                `json:"shares_change_20d_pct,omitempty"`
	LiquidityRisk     string                 `json:"liquidity_risk,omitempty"`
	SupplySignal      string                 `json:"supply_signal,omitempty"`
	PointInTimeSafe   bool                   `json:"point_in_time_safe"`
	Summary           string                 `json:"summary"`
	Warnings          []string               `json:"warnings,omitempty"`
	History           []FreeFloatObservation `json:"-"`
}

type IndexPortfolioObservation struct {
	Month    time.Time `json:"-"`
	MonthKey string    `json:"year_month"`
	Index    string    `json:"index"`
	ValueMTL float64   `json:"portfolio_value_mtl"`
}

type IndexPortfolioReport struct {
	Computed          bool                                   `json:"computed"`
	SourcePath        string                                 `json:"source_path,omitempty"`
	AsOf              time.Time                              `json:"as_of,omitempty"`
	SelectedIndex     string                                 `json:"selected_index,omitempty"`
	LatestMonth       string                                 `json:"latest_month,omitempty"`
	PortfolioValueMTL float64                                `json:"portfolio_value_mtl,omitempty"`
	Change1MPct       float64                                `json:"change_1m_pct,omitempty"`
	Change3MPct       float64                                `json:"change_3m_pct,omitempty"`
	Change12MPct      float64                                `json:"change_12m_pct,omitempty"`
	BIST100ValueMTL   float64                                `json:"bist100_value_mtl,omitempty"`
	BIST100Change1M   float64                                `json:"bist100_change_1m_pct,omitempty"`
	RelativeMomentum  float64                                `json:"relative_momentum_1m_pct,omitempty"`
	Signal            string                                 `json:"signal,omitempty"`
	PointInTimeSafe   bool                                   `json:"point_in_time_safe"`
	Summary           string                                 `json:"summary"`
	Warnings          []string                               `json:"warnings,omitempty"`
	History           map[string][]IndexPortfolioObservation `json:"-"`
}

type indexPortfolioFile struct {
	UpdatedAt time.Time `json:"updated_at"`
	Source    string    `json:"source"`
	Records   []struct {
		YearMonth string  `json:"year_month"`
		Index     string  `json:"endeks"`
		ValueMTL  float64 `json:"portfoy_deger_mtl"`
	} `json:"records"`
}

func LoadFreeFloat(vapDir, symbol string, asOf time.Time) FreeFloatReport {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	path := filepath.Join(vapDir, "fiili_dolasim", "Fiili_Dolasim_Raporu_MKK-"+symbol+".xlsx")
	report := FreeFloatReport{SourcePath: path, AsOf: asOf}
	if symbol == "" {
		report.Summary = "VAP fiili dolaşım için şirket kodu boş."
		report.Warnings = []string{"vap_free_float_symbol_missing"}
		return report
	}
	observations, err := readFreeFloatWorkbook(path, asOf)
	if err != nil {
		report.Summary = "VAP fiili dolaşım XLSX okunamadı: " + err.Error()
		report.Warnings = []string{"vap_free_float_xlsx_unavailable"}
		return report
	}
	if len(observations) == 0 {
		report.Summary = "VAP fiili dolaşım XLSX içinde analiz tarihine uygun kayıt yok."
		report.Warnings = []string{"vap_free_float_observations_missing"}
		return report
	}
	latest := observations[len(observations)-1]
	report.Computed = true
	report.History = observations
	report.Observations = len(observations)
	report.LatestDate = latest.DateText
	report.FreeFloatShares = latest.Shares
	report.IssuerCapital = latest.Capital
	report.FreeFloatRatioPct = latest.RatioPct
	report.RatioChange1DPP = freeFloatDelta(observations, 1, func(o FreeFloatObservation) float64 { return o.RatioPct })
	report.RatioChange20DPP = freeFloatDelta(observations, 20, func(o FreeFloatObservation) float64 { return o.RatioPct })
	report.RatioChange60DPP = freeFloatDelta(observations, 60, func(o FreeFloatObservation) float64 { return o.RatioPct })
	previousShares := freeFloatLagValue(observations, 20, func(o FreeFloatObservation) float64 { return o.Shares })
	if previousShares > 0 {
		report.SharesChange20Pct = 100 * (latest.Shares/previousShares - 1)
	}
	report.LiquidityRisk = freeFloatLiquidityRisk(latest.RatioPct)
	report.SupplySignal = freeFloatSupplySignal(report.RatioChange20DPP)
	report.PointInTimeSafe = asOf.IsZero() || !latest.Date.After(dayOnly(asOf))
	if !report.PointInTimeSafe {
		report.Warnings = append(report.Warnings, "vap_free_float_not_point_in_time_safe")
	}
	report.Summary = fmt.Sprintf(
		"VAP fiili dolaşım: %s itibarıyla oran %%%.2f, 20 gözlem değişimi %+.2f puan; likidite riski %s, arz sinyali %s.",
		report.LatestDate, report.FreeFloatRatioPct, report.RatioChange20DPP, report.LiquidityRisk, report.SupplySignal,
	)
	return report
}

func LoadIndexPortfolio(path, sector, industry string, asOf time.Time) IndexPortfolioReport {
	report := IndexPortfolioReport{SourcePath: path, AsOf: asOf, SelectedIndex: SelectIndex(sector, industry)}
	raw, err := os.ReadFile(path)
	if err != nil {
		report.Summary = "VAP BIST endeks portföy dosyası okunamadı: " + err.Error()
		report.Warnings = []string{"vap_index_portfolio_unavailable"}
		return report
	}
	var file indexPortfolioFile
	if err := json.Unmarshal(raw, &file); err != nil {
		report.Summary = "VAP BIST endeks portföy dosyası parse edilemedi: " + err.Error()
		report.Warnings = []string{"vap_index_portfolio_parse_failed"}
		return report
	}
	history := make(map[string][]IndexPortfolioObservation)
	cutoff := monthOnly(asOf)
	for _, record := range file.Records {
		month, parseErr := time.Parse("2006-01", strings.TrimSpace(record.YearMonth))
		if parseErr != nil || record.ValueMTL <= 0 {
			continue
		}
		if !cutoff.IsZero() && month.After(cutoff) {
			continue
		}
		index := normalizeIndex(record.Index)
		history[index] = append(history[index], IndexPortfolioObservation{
			Month: month, MonthKey: month.Format("2006-01"), Index: index, ValueMTL: record.ValueMTL,
		})
	}
	for index := range history {
		sort.Slice(history[index], func(i, j int) bool { return history[index][i].Month.Before(history[index][j].Month) })
	}
	report.History = history
	selected := history[normalizeIndex(report.SelectedIndex)]
	if len(selected) == 0 && normalizeIndex(report.SelectedIndex) != "BIST 100" {
		report.Warnings = append(report.Warnings, "vap_sector_index_missing_fallback_bist100")
		report.SelectedIndex = "BIST 100"
		selected = history["BIST 100"]
	}
	if len(selected) == 0 {
		report.Summary = "VAP BIST endeks portföy dosyasında uygun seri yok."
		report.Warnings = append(report.Warnings, "vap_index_portfolio_series_missing")
		return report
	}
	latest := selected[len(selected)-1]
	report.Computed = true
	report.LatestMonth = latest.MonthKey
	report.PortfolioValueMTL = latest.ValueMTL
	report.Change1MPct = portfolioChange(selected, 1)
	report.Change3MPct = portfolioChange(selected, 3)
	report.Change12MPct = portfolioChange(selected, 12)
	bist100 := history["BIST 100"]
	if len(bist100) > 0 {
		report.BIST100ValueMTL = bist100[len(bist100)-1].ValueMTL
		report.BIST100Change1M = portfolioChange(bist100, 1)
		report.RelativeMomentum = report.Change1MPct - report.BIST100Change1M
	}
	report.Signal = portfolioSignal(report.Change1MPct, report.RelativeMomentum)
	report.PointInTimeSafe = asOf.IsZero() || !latest.Month.After(monthOnly(asOf))
	if !report.PointInTimeSafe {
		report.Warnings = append(report.Warnings, "vap_index_portfolio_not_point_in_time_safe")
	}
	report.Summary = fmt.Sprintf(
		"VAP %s portföy değeri %s döneminde %.2f milyon TL; aylık değişim %+.2f%%, BIST 100'e göre fark %+.2f puan; sinyal %s.",
		report.SelectedIndex, report.LatestMonth, report.PortfolioValueMTL, report.Change1MPct, report.RelativeMomentum, report.Signal,
	)
	return report
}

func SelectIndex(sector, industry string) string {
	text := strings.ToLower(strings.Join([]string{sector, industry}, " "))
	switch {
	case containsAny(text, "banka", "bankac", "bank"):
		return "BIST BANKA"
	case containsAny(text, "teknoloji", "yazılım", "bilisim", "bilişim", "software"):
		return "BIST TEKNOLOJİ"
	case containsAny(text, "mali", "finans", "sigorta", "holding", "gayrimenkul", "gyo"):
		return "BIST MALİ"
	case containsAny(text, "sanayi", "üretim", "uretim", "metal", "kimya", "otomotiv", "tekstil", "gıda", "gida"):
		return "BIST SINAİ"
	case containsAny(text, "hizmet", "ulaştırma", "ulastirma", "turizm", "ticaret", "perakende"):
		return "BIST HİZMETLER"
	default:
		return "BIST 100"
	}
}

func readFreeFloatWorkbook(path string, asOf time.Time) ([]FreeFloatObservation, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("workbook has no sheet")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	cutoff := dayOnly(asOf)
	byDate := map[string]FreeFloatObservation{}
	for _, row := range rows {
		if len(row) < 8 {
			continue
		}
		date, parseErr := time.Parse("02.01.2006", strings.TrimSpace(row[0]))
		if parseErr != nil || (!cutoff.IsZero() && date.After(cutoff)) {
			continue
		}
		observation := FreeFloatObservation{
			Date: date, DateText: date.Format("2006-01-02"),
			Shares: parseNumber(row[5]), Capital: parseNumber(row[6]), RatioPct: parseNumber(row[7]),
		}
		if observation.Capital <= 0 || observation.RatioPct < 0 {
			continue
		}
		byDate[observation.DateText] = observation
	}
	out := make([]FreeFloatObservation, 0, len(byDate))
	for _, observation := range byDate {
		out = append(out, observation)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

func parseNumber(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.Contains(value, ",") && strings.Contains(value, ".") {
		if strings.LastIndex(value, ",") > strings.LastIndex(value, ".") {
			value = strings.ReplaceAll(value, ".", "")
			value = strings.ReplaceAll(value, ",", ".")
		} else {
			value = strings.ReplaceAll(value, ",", "")
		}
	} else if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		if len(parts[len(parts)-1]) <= 2 {
			value = strings.ReplaceAll(value, ",", ".")
		} else {
			value = strings.ReplaceAll(value, ",", "")
		}
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func freeFloatDelta(items []FreeFloatObservation, lag int, value func(FreeFloatObservation) float64) float64 {
	if len(items) == 0 {
		return 0
	}
	return value(items[len(items)-1]) - freeFloatLagValue(items, lag, value)
}

func freeFloatLagValue(items []FreeFloatObservation, lag int, value func(FreeFloatObservation) float64) float64 {
	index := len(items) - 1 - lag
	if index < 0 {
		index = 0
	}
	return value(items[index])
}

func portfolioChange(items []IndexPortfolioObservation, lag int) float64 {
	if len(items) < 2 {
		return 0
	}
	index := len(items) - 1 - lag
	if index < 0 {
		index = 0
	}
	base := items[index].ValueMTL
	if base <= 0 {
		return 0
	}
	// Keep serialized percentage metrics stable. Decimal input such as 60/50 can
	// otherwise surface as 19.999999999999996 and make exact downstream gates
	// needlessly brittle.
	return math.Round(100*(items[len(items)-1].ValueMTL/base-1)*1e9) / 1e9
}

func freeFloatLiquidityRisk(ratio float64) string {
	switch {
	case ratio <= 0:
		return "bilinmiyor"
	case ratio < 10:
		return "çok yüksek"
	case ratio < 20:
		return "yüksek"
	case ratio < 35:
		return "orta"
	default:
		return "düşük"
	}
}

func freeFloatSupplySignal(delta20 float64) string {
	switch {
	case delta20 > 1:
		return "dolaşımdaki arz belirgin artıyor"
	case delta20 > 0.1:
		return "dolaşımdaki arz sınırlı artıyor"
	case delta20 < -1:
		return "dolaşımdaki arz belirgin azalıyor"
	case delta20 < -0.1:
		return "dolaşımdaki arz sınırlı azalıyor"
	default:
		return "durağan"
	}
}

func portfolioSignal(change, relative float64) string {
	switch {
	case change > 0 && relative > 0:
		return "sektör portföy büyümesi piyasanın üzerinde"
	case change > 0:
		return "portföy büyüyor ancak göreli momentum zayıf"
	case change < 0 && relative < 0:
		return "sektör portföy daralması piyasanın altında"
	case change < 0:
		return "portföy daralıyor ancak göreli direnç var"
	default:
		return "nötr"
	}
}

func normalizeIndex(value string) string {
	return strings.Join(strings.Fields(strings.ToUpper(strings.TrimSpace(value))), " ")
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func dayOnly(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func monthOnly(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func finite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
