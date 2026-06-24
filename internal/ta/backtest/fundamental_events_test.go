package backtest

import (
	"testing"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/ta/ohlcv"
)

func TestRunFundamentalEventsExecutesOnlyAfterPublishDate(t *testing.T) {
	publishDate := time.Date(2026, 5, 10, 18, 30, 0, 0, time.UTC)
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), Open: 10, Close: 10},
		{Time: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), Open: 11, Close: 11},
	}
	periods := map[string]domain.FinancialPeriod{
		"2026-Q1": {
			Key:         "2026-Q1",
			PeriodEnd:   domain.FiscalPeriodEnd(2026, 1),
			PublishDate: &publishDate,
		},
	}

	result := RunFundamentalEvents(candles, periods)
	if !result.BacktestSafe || result.TradableEvents != 1 || result.LookaheadViolations != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.EventsList[0].ExecutionTime.After(publishDate) {
		t.Fatalf("execution time must be after publish date: %+v", result.EventsList[0])
	}
}

func TestRunFundamentalEventsRejectsMissingPublishDate(t *testing.T) {
	candles := []ohlcv.Candle{{Time: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), Open: 11, Close: 11}}
	periods := map[string]domain.FinancialPeriod{
		"2026-Q1": {
			Key:       "2026-Q1",
			PeriodEnd: domain.FiscalPeriodEnd(2026, 1),
		},
	}

	result := RunFundamentalEvents(candles, periods)
	if result.BacktestSafe || result.MissingPublishDateEvents != 1 {
		t.Fatalf("expected missing publish date to be unsafe, got %+v", result)
	}
	if result.MissingAvailableAtEvents != 1 {
		t.Fatalf("missing available-at events = %d, want 1", result.MissingAvailableAtEvents)
	}
}

func TestRunFundamentalEventsUsesAvailableAtWhenPublishDateIsMissing(t *testing.T) {
	availableAt := time.Date(2026, 5, 10, 18, 30, 0, 0, time.UTC)
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), Open: 10, Close: 10},
		{Time: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), Open: 11, Close: 11},
	}
	periods := map[string]domain.FinancialPeriod{
		"2026-Q1": {
			Key:                "2026-Q1",
			PeriodEnd:          domain.FiscalPeriodEnd(2026, 1),
			AvailableAt:        &availableAt,
			AvailabilitySource: "local_json_import_at",
		},
	}

	result := RunFundamentalEvents(candles, periods)
	if !result.BacktestSafe || result.TradableEvents != 1 || result.LookaheadViolations != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.MissingPublishDateEvents != 1 || result.MissingAvailableAtEvents != 0 {
		t.Fatalf("unexpected missing metadata counters: %+v", result)
	}
	if !result.EventsList[0].ExecutionTime.After(availableAt) {
		t.Fatalf("execution time must be after available_at: %+v", result.EventsList[0])
	}
}

func TestRunFundamentalEventsProductionPolicyRejectsConservativeAvailableAt(t *testing.T) {
	availableAt := time.Date(2026, 5, 10, 18, 30, 0, 0, time.UTC)
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), Open: 11, Close: 11},
	}
	periods := map[string]domain.FinancialPeriod{
		"2026-Q1": {
			Key:                "2026-Q1",
			PeriodEnd:          domain.FiscalPeriodEnd(2026, 1),
			AvailableAt:        &availableAt,
			AvailabilitySource: "fetched_at",
		},
	}

	result := RunFundamentalEventsWithOptions(candles, periods, FundamentalEventOptions{RequireVerifiedPublishDate: true})
	if !result.BacktestSafe || result.TradableEvents != 0 || result.PolicyRejectedEvents != 1 {
		t.Fatalf("expected production policy rejection without unsafe backtest, got %+v", result)
	}
	if got := result.EventsList[0].RejectionReason; got != "publish_date_unverified_for_production" {
		t.Fatalf("rejection reason = %q", got)
	}
}

func TestRunFundamentalEventsRejectsUntrustedAvailableAtSource(t *testing.T) {
	availableAt := time.Date(2026, 5, 10, 18, 30, 0, 0, time.UTC)
	candles := []ohlcv.Candle{
		{Time: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), Open: 11, Close: 11},
	}
	periods := map[string]domain.FinancialPeriod{
		"2026-Q1": {
			Key:                "2026-Q1",
			PeriodEnd:          domain.FiscalPeriodEnd(2026, 1),
			AvailableAt:        &availableAt,
			AvailabilitySource: "unknown_vendor_date",
		},
	}

	result := RunFundamentalEvents(candles, periods)
	if result.BacktestSafe || result.TradableEvents != 0 || result.UnsafeAvailabilityEvents != 1 {
		t.Fatalf("expected unsafe untrusted available-at source, got %+v", result)
	}
	if got := result.EventsList[0].RejectionReason; got != "unverified_available_at_source" {
		t.Fatalf("rejection reason = %q", got)
	}
}
