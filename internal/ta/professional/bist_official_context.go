package professional

import (
	"fmt"
	"math"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"
)

type BISTOfficialContext struct {
	Computed           bool      `json:"computed"`
	Source             string    `json:"source,omitempty"`
	AsOf               time.Time `json:"as_of,omitempty"`
	LatestDate         string    `json:"latest_date,omitempty"`
	Candles            int       `json:"candles"`
	LastOpen           float64   `json:"last_open,omitempty"`
	LastHigh           float64   `json:"last_high,omitempty"`
	LastLow            float64   `json:"last_low,omitempty"`
	LastClose          float64   `json:"last_close,omitempty"`
	LastVolume         float64   `json:"last_volume,omitempty"`
	AnalysisClose      float64   `json:"analysis_close,omitempty"`
	CloseDifferenceBps float64   `json:"close_difference_bps,omitempty"`
	PriceConfirmed     bool      `json:"price_confirmed"`
	PointInTimeSafe    bool      `json:"point_in_time_safe"`
	Summary            string    `json:"summary"`
	Warnings           []string  `json:"warnings,omitempty"`
}

func buildBISTOfficialContext(official []ohlcv.Candle, sourceErr string, analysis []ohlcv.Candle, asOf time.Time) BISTOfficialContext {
	report := BISTOfficialContext{
		Source: "Borsa İstanbul resmi günlük bülten CSV/XLS (data/bist/unprocessed)",
		AsOf:   asOf,
	}
	if len(official) == 0 {
		report.Summary = "BIST resmi bülten OHLCV serisi bulunamadı."
		report.Warnings = []string{"bist_official_ohlcv_missing"}
		if strings.TrimSpace(sourceErr) != "" {
			report.Summary += " " + sourceErr
			report.Warnings = append(report.Warnings, "bist_official_ohlcv_read_error")
		}
		return report
	}
	latest := official[len(official)-1]
	report.Computed = true
	report.Candles = len(official)
	report.LatestDate = latest.Time.Format("2006-01-02")
	report.LastOpen = latest.EffectiveOpen()
	report.LastHigh = latest.EffectiveHigh()
	report.LastLow = latest.EffectiveLow()
	report.LastClose = latest.EffectiveClose()
	report.LastVolume = latest.EffectiveVolume()
	report.PointInTimeSafe = asOf.IsZero() || !latest.Time.After(asOf)
	if len(analysis) > 0 {
		analysisLatest := analysis[len(analysis)-1]
		report.AnalysisClose = analysisLatest.EffectiveClose()
		if report.AnalysisClose > 0 {
			report.CloseDifferenceBps = 10000 * (report.LastClose/report.AnalysisClose - 1)
			report.PriceConfirmed = sameTradingDay(latest.Time, analysisLatest.Time) && math.Abs(report.CloseDifferenceBps) <= 15
		}
	}
	if !report.PointInTimeSafe {
		report.Warnings = append(report.Warnings, "bist_official_not_point_in_time_safe")
	}
	if !report.PriceConfirmed {
		report.Warnings = append(report.Warnings, "bist_official_analysis_close_not_confirmed")
	}
	report.Summary = fmt.Sprintf(
		"BIST resmi bülten: %s tarihli %d mum; kapanış %.4f TL, analiz kaynağı farkı %+.1f baz puan, fiyat teyidi %t.",
		report.LatestDate, report.Candles, report.LastClose, report.CloseDifferenceBps, report.PriceConfirmed,
	)
	return report
}

func sameTradingDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
