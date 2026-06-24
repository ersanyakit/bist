package validation

import (
	"testing"
	"time"

	"hissebot/internal/domain/macro"
)

func TestValidateMacroSeriesDetectsGap(t *testing.T) {
	report := ValidateMacroSeries(macro.Series{
		ID:        macro.SeriesCPI,
		Frequency: macro.FrequencyMonthly,
		Observations: []macro.Observation{
			{SeriesID: macro.SeriesCPI, Date: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), Value: 1},
			{SeriesID: macro.SeriesCPI, Date: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC), Value: 2},
		},
	})

	if report.Status != "limited" {
		t.Fatalf("status = %s, want limited: %+v", report.Status, report)
	}
}
