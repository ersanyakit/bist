package financials

import (
	"context"

	"hissebot/internal/domain"
	"hissebot/internal/storage"
)

type ReconciliationReport struct {
	EquitiesChecked int                                              `json:"equities_checked"`
	PeriodChecks    int                                              `json:"period_checks"`
	Failures        int                                              `json:"failures"`
	DataGaps        int                                              `json:"data_gaps"`
	ByTicker        map[string][]domain.FinancialReconciliationCheck `json:"by_ticker,omitempty"`
	Samples         []ReconciliationSample                           `json:"samples,omitempty"`
}

type ReconciliationSample struct {
	Ticker    string  `json:"ticker"`
	PeriodKey string  `json:"period_key"`
	Check     string  `json:"check"`
	Warning   string  `json:"warning"`
	Expected  float64 `json:"expected,omitempty"`
	Actual    float64 `json:"actual,omitempty"`
	Diff      float64 `json:"difference,omitempty"`
}

func Reconcile(ctx context.Context, store *storage.EquityStore) (ReconciliationReport, error) {
	equities, err := store.List()
	if err != nil {
		return ReconciliationReport{}, err
	}
	report := ReconciliationReport{ByTicker: map[string][]domain.FinancialReconciliationCheck{}}
	for _, equity := range equities {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		default:
		}
		if equity.BilancoInfo == nil || len(equity.BilancoInfo.Data) == 0 {
			continue
		}
		domain.NormalizeBilancoInfo(equity.BilancoInfo, equity.Ticker)
		checks := domain.ValidateFinancialReconciliation(equity.BilancoInfo)
		report.EquitiesChecked++
		report.PeriodChecks += len(checks)
		for _, check := range checks {
			if check.MissingInputs {
				report.DataGaps++
				appendReconciliationSample(&report, reconciliationSample(equity.Ticker, check), false)
				continue
			}
			if !check.Passed {
				report.Failures++
				report.ByTicker[equity.Ticker] = append(report.ByTicker[equity.Ticker], check)
				appendReconciliationSample(&report, reconciliationSample(equity.Ticker, check), true)
			}
		}
	}
	if len(report.ByTicker) == 0 {
		report.ByTicker = nil
	}
	return report, nil
}

func reconciliationSample(ticker string, check domain.FinancialReconciliationCheck) ReconciliationSample {
	return ReconciliationSample{
		Ticker:    ticker,
		PeriodKey: check.PeriodKey,
		Check:     check.Check,
		Warning:   check.Warning,
		Expected:  check.Expected,
		Actual:    check.Actual,
		Diff:      check.Difference,
	}
}

func appendReconciliationSample(report *ReconciliationReport, sample ReconciliationSample, priority bool) {
	const limit = 20
	if priority {
		report.Samples = append([]ReconciliationSample{sample}, report.Samples...)
		if len(report.Samples) > limit {
			report.Samples = report.Samples[:limit]
		}
		return
	}
	if len(report.Samples) < limit {
		report.Samples = append(report.Samples, sample)
	}
}
