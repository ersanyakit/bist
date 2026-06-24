package backtest

import (
	"sort"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/ta/ohlcv"
)

type FundamentalEventResult struct {
	ExecutionModel                string                     `json:"execution_model"`
	RequireVerifiedPublishDate    bool                       `json:"require_verified_publish_date"`
	Events                        int                        `json:"events"`
	TradableEvents                int                        `json:"tradable_events"`
	VerifiedPublishDateEvents     int                        `json:"verified_publish_date_events"`
	ConservativeAvailableAtEvents int                        `json:"conservative_available_at_events"`
	UnsafeAvailabilityEvents      int                        `json:"unsafe_availability_events"`
	PolicyRejectedEvents          int                        `json:"policy_rejected_events"`
	MissingPublishDateEvents      int                        `json:"missing_publish_date_events"`
	MissingAvailableAtEvents      int                        `json:"missing_available_at_events"`
	NoExecutionBarEvents          int                        `json:"no_execution_bar_events"`
	LookaheadViolations           int                        `json:"lookahead_violations"`
	BacktestSafe                  bool                       `json:"backtest_safe"`
	Warnings                      []string                   `json:"warnings,omitempty"`
	EventsList                    []FundamentalEventInstance `json:"events_list,omitempty"`
}

type FundamentalEventInstance struct {
	PeriodKey          string     `json:"period_key"`
	PeriodEnd          time.Time  `json:"period_end"`
	PublishDate        *time.Time `json:"publish_date,omitempty"`
	AvailableAt        *time.Time `json:"available_at,omitempty"`
	AvailabilitySource string     `json:"availability_source,omitempty"`
	AvailabilityStatus string     `json:"availability_status,omitempty"`
	ExecutionTime      time.Time  `json:"execution_time,omitempty"`
	ExecutionIndex     int        `json:"execution_index,omitempty"`
	BacktestSafe       bool       `json:"backtest_safe"`
	RejectionReason    string     `json:"rejection_reason,omitempty"`
}

type FundamentalEventOptions struct {
	RequireVerifiedPublishDate bool
}

func RunFundamentalEvents(candles []ohlcv.Candle, periods map[string]domain.FinancialPeriod) FundamentalEventResult {
	return RunFundamentalEventsWithOptions(candles, periods, FundamentalEventOptions{})
}

func RunFundamentalEventsWithOptions(candles []ohlcv.Candle, periods map[string]domain.FinancialPeriod, opts FundamentalEventOptions) FundamentalEventResult {
	result := FundamentalEventResult{
		ExecutionModel:             "financial_statement_available_at_execute_next_bar",
		RequireVerifiedPublishDate: opts.RequireVerifiedPublishDate,
		BacktestSafe:               true,
	}
	if len(periods) == 0 {
		result.BacktestSafe = false
		result.Warnings = append(result.Warnings, "financial_periods_missing")
		return result
	}
	keys := make([]string, 0, len(periods))
	for key := range periods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		period := periods[key]
		event := FundamentalEventInstance{
			PeriodKey:          key,
			PeriodEnd:          period.PeriodEnd,
			PublishDate:        period.PublishDate,
			AvailableAt:        domain.EffectiveFinancialAvailableAt(period),
			AvailabilitySource: period.AvailabilitySource,
			AvailabilityStatus: domain.FinancialPeriodAvailabilityStatus(period),
		}
		result.Events++
		switch event.AvailabilityStatus {
		case domain.FinancialAvailabilityVerifiedPublishDate:
			result.VerifiedPublishDateEvents++
		case domain.FinancialAvailabilityConservativeAvailableAt:
			result.ConservativeAvailableAtEvents++
		default:
			result.UnsafeAvailabilityEvents++
		}
		if period.PublishDate == nil {
			result.MissingPublishDateEvents++
		}
		if event.AvailableAt == nil {
			event.RejectionReason = "available_at_missing"
			result.MissingAvailableAtEvents++
			result.BacktestSafe = false
			result.EventsList = append(result.EventsList, event)
			continue
		}
		if opts.RequireVerifiedPublishDate && event.AvailabilityStatus != domain.FinancialAvailabilityVerifiedPublishDate {
			event.RejectionReason = "publish_date_unverified_for_production"
			event.BacktestSafe = true
			result.PolicyRejectedEvents++
			result.EventsList = append(result.EventsList, event)
			continue
		}
		if event.AvailabilityStatus == domain.FinancialAvailabilityUnsafeUnverifiedAvailable {
			event.RejectionReason = "unverified_available_at_source"
			result.BacktestSafe = false
			result.EventsList = append(result.EventsList, event)
			continue
		}
		executionIndex := firstCandleAfter(candles, *event.AvailableAt)
		if executionIndex < 0 {
			event.RejectionReason = "no_execution_bar_after_available_at"
			result.NoExecutionBarEvents++
			result.BacktestSafe = false
			result.EventsList = append(result.EventsList, event)
			continue
		}
		event.ExecutionIndex = executionIndex
		event.ExecutionTime = candles[executionIndex].Time
		event.BacktestSafe = true
		if event.ExecutionTime.Before(*event.AvailableAt) {
			result.LookaheadViolations++
			result.BacktestSafe = false
			event.BacktestSafe = false
			event.RejectionReason = "execution_before_available_at"
		} else {
			result.TradableEvents++
		}
		result.EventsList = append(result.EventsList, event)
	}
	if result.MissingPublishDateEvents > 0 {
		result.Warnings = append(result.Warnings, "fundamental_publish_dates_missing")
	}
	if result.MissingAvailableAtEvents > 0 {
		result.Warnings = append(result.Warnings, "fundamental_available_at_missing")
	}
	if result.UnsafeAvailabilityEvents > result.MissingAvailableAtEvents {
		result.Warnings = append(result.Warnings, "fundamental_unverified_available_at_source")
	}
	if result.PolicyRejectedEvents > 0 {
		result.Warnings = append(result.Warnings, "fundamental_events_rejected_by_verified_publish_date_policy")
	}
	if result.NoExecutionBarEvents > 0 {
		result.Warnings = append(result.Warnings, "fundamental_execution_bars_missing")
	}
	if result.LookaheadViolations > 0 {
		result.Warnings = append(result.Warnings, "fundamental_lookahead_violation")
	}
	return result
}

func firstCandleAfter(candles []ohlcv.Candle, ts time.Time) int {
	for i, candle := range candles {
		if candle.Time.After(ts) {
			return i
		}
	}
	return -1
}
