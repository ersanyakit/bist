package financials

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/services/kap"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

type ratioSpec struct {
	Name    string
	Fields  []string
	Formula string
	Calc    func([]float64) float64
	CalcAt  func(*domain.BilancoInfo, int, int) (float64, bool)
}

var ratioSpecs = []ratioSpec{
	{Name: "CariOran", Fields: []string{"1A", "2A"}, Formula: "1A/2A", Calc: func(v []float64) float64 { return ratio(v[0], v[1]) }},
	{Name: "NakitOran", Fields: []string{"1AA", "2A"}, Formula: "1AA/2A", Calc: func(v []float64) float64 { return ratio(v[0], v[1]) }},
	{Name: "AsitTestOran", Fields: []string{"1A", "1AF", "2A"}, Formula: "(1A-1AF)/2A", Calc: func(v []float64) float64 { return ratio(v[0]-v[1], v[2]) }},
	{Name: "LikiditeOran", Fields: []string{"1AA", "1AC", "2A"}, Formula: "(1AA+1AC)/2A", Calc: func(v []float64) float64 { return ratio(v[0]+v[1], v[2]) }},
	{Name: "NetKarMarji", Fields: []string{"3L", "3C"}, Formula: "3L/3C*100", Calc: func(v []float64) float64 { return ratio(v[0], v[1]) * 100 }},
	{Name: "BrutKarMarji", Fields: []string{"3D", "3C"}, Formula: "3D/3C*100", Calc: func(v []float64) float64 { return ratio(v[0], v[1]) * 100 }},
	{Name: "FaaliyetKarMarji", Fields: []string{"3DF", "3C"}, Formula: "3DF/3C*100", Calc: func(v []float64) float64 { return ratio(v[0], v[1]) * 100 }},
	{Name: "ROA", Fields: []string{"3L", "1BL"}, Formula: "TTM(3L)/AVG(1BL,current,prior_year_same_quarter)*100", CalcAt: calcROA},
	{Name: "ROE", Fields: []string{"3L", "2N"}, Formula: "TTM(3L)/AVG(2N,current,prior_year_same_quarter)*100", CalcAt: calcROE},
	{Name: "StokDevirHizi", Fields: []string{"3CA", "1AF"}, Formula: "abs(3CA)/1AF", Calc: func(v []float64) float64 { return ratio(math.Abs(v[0]), v[1]) }},
	{Name: "AlacakDevirHizi", Fields: []string{"3C", "1AC"}, Formula: "3C/1AC", Calc: func(v []float64) float64 { return ratio(v[0], v[1]) }},
	{Name: "VarlikDevirHizi", Fields: []string{"3C", "1BL"}, Formula: "3C/1BL", Calc: func(v []float64) float64 { return ratio(v[0], v[1]) }},
}

func ratio(numerator, denominator float64) float64 {
	if denominator == 0 || math.IsNaN(denominator) || math.IsInf(denominator, 0) {
		return math.NaN()
	}
	return numerator / denominator
}

func Calculate(ctx context.Context, store *storage.EquityStore) error {
	equities, err := store.List()
	if err != nil {
		return err
	}
	updated := 0

	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if equity.BilancoInfo == nil || len(equity.BilancoInfo.Data) == 0 {
			continue
		}
		domain.NormalizeBilancoInfo(equity.BilancoInfo, equity.Ticker)
		if _, err := kap.ApplyFinancialPublishDatesFromStore(store, equity.Ticker, equity.BilancoInfo); err != nil {
			return err
		}
		domain.MarkFinancialPeriodsAvailableAt(equity.BilancoInfo, time.Now().UTC(), "local_json_calculation_at")
		calculations, audit := CalculateEquityWithAudit(equity.BilancoInfo)
		versionStore, err := upsertStatementVersionStore(store, equity.Ticker, equity.BilancoInfo)
		if err != nil {
			return err
		}
		if len(calculations) > 0 {
			if err := util.WriteJSON(store.FinancialCalculationsPath(equity.Ticker), calculations); err != nil {
				return err
			}
		} else if len(versionStore.Versions) == 0 {
			continue
		}
		if err := util.WriteJSON(store.FinancialInfoPath(equity.Ticker), equity.BilancoInfo); err != nil {
			return err
		}
		if len(versionStore.Versions) > 0 {
			if err := util.WriteJSON(store.FinancialStatementVersionsPath(equity.Ticker), versionStore); err != nil {
				return err
			}
		}
		if err := store.Update(equity.Ticker, func(e *domain.Equity) error {
			e.BilancoInfo = equity.BilancoInfo
			e.BilancoCalculations = calculations
			e.BilancoCalculationAudit = audit
			return nil
		}); err != nil {
			return err
		}
		updated++
	}

	fmt.Printf("financials: %d equity calculation blocks updated\n", updated)
	return nil
}

func CalculateEquity(info *domain.BilancoInfo) map[string]domain.YearQuarter {
	calculations, _ := CalculateEquityWithAudit(info)
	return calculations
}

func CalculateEquityWithAudit(info *domain.BilancoInfo) (map[string]domain.YearQuarter, *domain.FinancialCalculationAudit) {
	result := map[string]domain.YearQuarter{}
	if info == nil {
		return result, &domain.FinancialCalculationAudit{
			Version:      domain.FinancialCalculationVersion,
			CreatedAt:    time.Now().UTC(),
			BacktestSafe: false,
			Warnings:     []string{"financial_statements_missing"},
		}
	}
	domain.NormalizeBilancoInfo(info, "")
	quality := domain.ValidateFinancialDataQuality(info, time.Time{})
	audit := &domain.FinancialCalculationAudit{
		Version:      domain.FinancialCalculationVersion,
		CreatedAt:    time.Now().UTC(),
		BacktestSafe: quality.BacktestSafe,
		InputQuality: quality,
		Lineage:      append([]domain.DataLineageEvent{}, info.Lineage...),
		Metrics:      map[string]domain.FinancialMetricAudit{},
	}
	for _, spec := range ratioSpecs {
		years := yearsForSpec(info, spec)
		if len(years) == 0 {
			continue
		}
		specResult := domain.YearQuarter{}
		for _, year := range years {
			yearInt, _ := strconv.Atoi(year)
			quarters := domain.QuarterValues{}
			for index := 0; index < 4; index++ {
				value, ok := calculateSpecValue(info, spec, year, yearInt, index)
				if !ok {
					audit.Warnings = append(audit.Warnings, fmt.Sprintf("%s_%s_Q%d_inputs_missing_or_invalid", spec.Name, year, domain.FiscalQuarterFromIndex(index)))
					continue
				}
				if math.IsNaN(value) || math.IsInf(value, 0) {
					continue
				}
				setQuarter(&quarters, index, value)
			}
			specResult[year] = quarters
		}
		result[spec.Name] = specResult
		audit.Metrics[spec.Name] = domain.FinancialMetricAudit{
			Formula:     spec.Formula,
			InputFields: append([]string{}, spec.Fields...),
		}
	}
	if !audit.BacktestSafe {
		audit.Warnings = append(audit.Warnings, "financial_calculations_not_backtest_safe_without_verified_publish_dates")
	}
	return result, audit
}

func calculateSpecValue(info *domain.BilancoInfo, spec ratioSpec, year string, yearInt int, index int) (float64, bool) {
	if spec.CalcAt != nil {
		return spec.CalcAt(info, yearInt, domain.FiscalQuarterFromIndex(index))
	}
	values, ok := valuesForQuarter(info, spec.Fields, year, index)
	if !ok {
		return 0, false
	}
	return spec.Calc(values), true
}

func yearsForSpec(info *domain.BilancoInfo, spec ratioSpec) []string {
	if info == nil || len(spec.Fields) == 0 {
		return nil
	}
	first, ok := info.Data[spec.Fields[0]]
	if !ok {
		return nil
	}
	years := make([]string, 0, len(first.Years))
	for year := range first.Years {
		hasAll := true
		for _, field := range spec.Fields[1:] {
			if _, ok := info.Data[field].Years[year]; !ok {
				hasAll = false
				break
			}
		}
		if hasAll {
			years = append(years, year)
		}
	}
	sortYearsDesc(years)
	return years
}

func valuesForQuarter(info *domain.BilancoInfo, fields []string, year string, index int) ([]float64, bool) {
	values := make([]float64, 0, len(fields))
	for _, fieldName := range fields {
		field, ok := info.Data[fieldName]
		if !ok {
			return nil, false
		}
		periods := field.Years[year]
		if len(periods) <= index || periods[index] == nil {
			return nil, false
		}
		values = append(values, *periods[index])
	}
	return values, true
}

func calcROA(info *domain.BilancoInfo, year int, quarter int) (float64, bool) {
	return calcReturnOnAverageBalance(info, "1BL", year, quarter)
}

func calcROE(info *domain.BilancoInfo, year int, quarter int) (float64, bool) {
	return calcReturnOnAverageBalance(info, "2N", year, quarter)
}

func calcReturnOnAverageBalance(info *domain.BilancoInfo, balanceCode string, year int, quarter int) (float64, bool) {
	netIncomeTTM, ok := ttmFlow(info, "3L", year, quarter)
	if !ok {
		return 0, false
	}
	currentBalance, ok := fieldValueAt(info, balanceCode, year, quarter)
	if !ok {
		return 0, false
	}
	priorBalance, ok := fieldValueAt(info, balanceCode, year-1, quarter)
	if !ok {
		return 0, false
	}
	averageBalance := (currentBalance + priorBalance) / 2
	value := ratio(netIncomeTTM, averageBalance) * 100
	return value, !(math.IsNaN(value) || math.IsInf(value, 0))
}

func ttmFlow(info *domain.BilancoInfo, code string, year int, quarter int) (float64, bool) {
	current, ok := fieldValueAt(info, code, year, quarter)
	if !ok {
		return 0, false
	}
	if quarter == 4 {
		return current, true
	}
	prevFY, ok := fieldValueAt(info, code, year-1, 4)
	if !ok {
		return 0, false
	}
	prevSameQuarter, ok := fieldValueAt(info, code, year-1, quarter)
	if !ok {
		return 0, false
	}
	return current + prevFY - prevSameQuarter, true
}

func fieldValueAt(info *domain.BilancoInfo, code string, year int, quarter int) (float64, bool) {
	if info == nil || year <= 0 || quarter < 1 || quarter > 4 {
		return 0, false
	}
	field, ok := info.Data[code]
	if !ok {
		return 0, false
	}
	values := field.Years[strconv.Itoa(year)]
	index := indexFromFiscalQuarter(quarter)
	if index < 0 || index >= len(values) || values[index] == nil {
		return 0, false
	}
	return *values[index], true
}

func indexFromFiscalQuarter(quarter int) int {
	switch quarter {
	case 4:
		return 0
	case 3:
		return 1
	case 2:
		return 2
	case 1:
		return 3
	default:
		return -1
	}
}

func setQuarter(q *domain.QuarterValues, index int, value float64) {
	v := value
	switch index {
	case 0:
		q.Q4 = &v
	case 1:
		q.Q3 = &v
	case 2:
		q.Q2 = &v
	case 3:
		q.Q1 = &v
	}
}

func parseNullableFloat(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = strings.ReplaceAll(value, ",", ".")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil
	}
	return &parsed
}

func sortYearsDesc(years []string) {
	for i := 0; i < len(years); i++ {
		for j := i + 1; j < len(years); j++ {
			if years[j] > years[i] {
				years[i], years[j] = years[j], years[i]
			}
		}
	}
}
