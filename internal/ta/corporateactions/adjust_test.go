package corporateactions

import (
	"testing"
	"time"

	"hissebot/internal/ta/ohlcv"
)

func TestApplyBonusIssueAdjustsHistoricalOHLCV(t *testing.T) {
	effective := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	ratio := 1.0
	candles := []ohlcv.Candle{
		testCandle("2026-01-01", 100, 100, 1000),
		testCandle("2026-01-02", 100, 100, 1000),
		testCandle("2026-01-03", 50, 50, 2000),
	}

	adjusted, report := Apply(candles, ActionSet{
		Symbol: "TEST",
		Status: "adjustment_ready",
		Actions: []Action{{
			ID:            "bonus-1",
			Symbol:        "TEST",
			Type:          TypeBonusIssue,
			Status:        StatusVerified,
			EffectiveDate: &effective,
			Ratio:         &ratio,
		}},
	}, "1D")

	if !report.BacktestSafe || report.AppliedActions != 1 || report.PriceSeries != "corporate_action_adjusted" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if adjusted[0].AdjustedClose != 50 || adjusted[1].AdjustedClose != 50 || adjusted[2].AdjustedClose != 50 {
		t.Fatalf("adjusted closes = %.2f %.2f %.2f", adjusted[0].AdjustedClose, adjusted[1].AdjustedClose, adjusted[2].AdjustedClose)
	}
	if adjusted[0].AdjustedVolume != 2000 {
		t.Fatalf("adjusted volume = %.2f", adjusted[0].AdjustedVolume)
	}
}

func TestApplyDividendUsesPerShareCashAmount(t *testing.T) {
	effective := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	cash := 2.0
	candles := []ohlcv.Candle{
		testCandle("2026-01-01", 100, 100, 1000),
		testCandle("2026-01-02", 98, 98, 1000),
	}

	adjusted, report := Apply(candles, ActionSet{
		Symbol: "TEST",
		Status: "adjustment_ready",
		Actions: []Action{{
			ID:            "div-1",
			Symbol:        "TEST",
			Type:          TypeDividend,
			Status:        StatusVerified,
			EffectiveDate: &effective,
			CashAmount:    &cash,
		}},
	}, "1D")

	if !report.BacktestSafe || report.AppliedActions != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if adjusted[0].AdjustedClose != 98 {
		t.Fatalf("adjusted close = %.2f, want 98", adjusted[0].AdjustedClose)
	}
}

func TestApplyRejectsRawSeriesWhenCorporateActionSourceMissing(t *testing.T) {
	candles := []ohlcv.Candle{testCandle("2026-01-01", 100, 100, 1000)}
	_, report := Apply(candles, ActionSet{Symbol: "TEST", Status: "missing"}, "1D")
	if report.BacktestSafe {
		t.Fatalf("raw series without corporate action source should be unsafe: %+v", report)
	}
	if report.UnadjustedCandles != 1 || report.PriceSeries != "raw" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestApplyKeepsHistoricalSeriesSafeWhenActionsAreFutureDated(t *testing.T) {
	effective := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	cash := 2.0
	candles := []ohlcv.Candle{
		testCandle("2026-01-01", 100, 101, 1000),
		testCandle("2026-01-02", 101, 102, 1000),
	}

	adjusted, report := Apply(candles, ActionSet{
		Symbol: "TEST",
		Status: "adjustment_ready",
		Actions: []Action{{
			ID:            "future-div-1",
			Symbol:        "TEST",
			Type:          TypeDividend,
			Status:        StatusVerified,
			EffectiveDate: &effective,
			CashAmount:    &cash,
		}},
	}, "1D")

	if !report.BacktestSafe || report.AppliedActions != 0 || report.SkippedActions != 0 {
		t.Fatalf("future action should not make historical backtest unsafe: %+v", report)
	}
	if report.PriceSeries != "raw" || report.UnadjustedCandles != len(candles) {
		t.Fatalf("unexpected report: %+v", report)
	}
	if adjusted[0].AdjustedClose != 0 || adjusted[0].IsAdjusted {
		t.Fatalf("future action should not alter historical candle: %+v", adjusted[0])
	}
}

func TestApplyDoesNotBlockBacktestForReviewOnlyKAPEventsWithoutFactors(t *testing.T) {
	effective := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	ratio := 1.0
	candles := []ohlcv.Candle{
		testCandle("2026-01-01", 100, 100, 1000),
		testCandle("2026-01-02", 100, 100, 1000),
		testCandle("2026-01-03", 50, 50, 2000),
	}

	_, report := Apply(candles, ActionSet{
		Symbol: "TEST",
		Status: "candidate_adjustments_review_required",
		Actions: []Action{
			{
				ID:            "bonus-1",
				Symbol:        "TEST",
				Type:          TypeBonusIssue,
				Status:        StatusVerified,
				EffectiveDate: &effective,
				Ratio:         &ratio,
			},
			{
				ID:     "kap-review-1",
				Symbol: "TEST",
				Type:   TypeCapitalIncrease,
				Status: StatusReview,
				Title:  "Sermaye artırımı hakkında KAP metinsel olay adayı",
			},
		},
	}, "1D")

	if !report.BacktestSafe {
		t.Fatalf("review-only event without adjustment payload should not block backtest: %+v", report)
	}
	if report.AppliedActions != 1 || report.SkippedActions != 1 {
		t.Fatalf("unexpected action counts: %+v", report)
	}
}

func TestApplyBlocksBacktestForVerifiedPriceActionMissingFactor(t *testing.T) {
	effective := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	candles := []ohlcv.Candle{
		testCandle("2026-01-01", 100, 100, 1000),
		testCandle("2026-01-02", 99, 99, 1000),
	}

	_, report := Apply(candles, ActionSet{
		Symbol: "TEST",
		Status: "candidate_adjustments_review_required",
		Actions: []Action{{
			ID:            "verified-dividend-missing-cash",
			Symbol:        "TEST",
			Type:          TypeDividend,
			Status:        StatusVerified,
			EffectiveDate: &effective,
		}},
	}, "1D")

	if report.BacktestSafe {
		t.Fatalf("verified price-affecting action without factor should block backtest: %+v", report)
	}
	if !containsString(report.Warnings, "verified_price_affecting_corporate_actions_incomplete") {
		t.Fatalf("missing verified incomplete warning: %+v", report.Warnings)
	}
}

func testCandle(date string, open, close, volume float64) ohlcv.Candle {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return ohlcv.Candle{
		Time:   t,
		Open:   open,
		High:   maxFloat(open, close),
		Low:    minFloat(open, close),
		Close:  close,
		Volume: volume,
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
