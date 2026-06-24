package validation

import (
	"time"

	"hissebot/internal/domain/macro"
)

func ValidateMacroSeries(series macro.Series) Report {
	report := NewReport()
	if series.ID == "" {
		report.Add(SeverityError, "series_id_missing", "id", "makro seri id boş olamaz")
	}
	if len(series.Observations) == 0 {
		report.Add(SeverityError, "macro_observations_missing", "observations", "makro seri gözlemi yok")
		return report
	}
	for i, observation := range series.Observations {
		if observation.Date.IsZero() {
			report.Add(SeverityError, "macro_date_missing", "date", "makro gözlem tarihi boş olamaz")
		}
		if i > 0 && !observation.Date.After(series.Observations[i-1].Date) {
			report.Add(SeverityWarning, "macro_series_not_increasing", "date", "makro seri tarihleri artan sırada değil")
		}
		if i > 0 && hasLargeGap(series.Frequency, series.Observations[i-1].Date, observation.Date) {
			report.Add(SeverityWarning, "macro_series_gap", "date", "makro zaman serisinde beklenenden büyük boşluk var")
		}
	}
	return report
}

func hasLargeGap(freq macro.Frequency, prev time.Time, current time.Time) bool {
	days := current.Sub(prev).Hours() / 24
	switch freq {
	case macro.FrequencyDaily:
		return days > 5
	case macro.FrequencyMonthly:
		return days > 45
	case macro.FrequencyQuarterly:
		return days > 120
	case macro.FrequencyAnnual:
		return days > 400
	default:
		return false
	}
}
