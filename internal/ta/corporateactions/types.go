package corporateactions

import "time"

const (
	TypeDividend         = "dividend"
	TypeBonusIssue       = "bonus_issue"
	TypeRightsIssue      = "rights_issue"
	TypeCapitalIncrease  = "capital_increase"
	TypeCapitalReduction = "capital_reduction"
	TypeSplit            = "split"
	TypeMerger           = "merger"
	TypeSpinOff          = "spin_off"

	StatusVerified  = "verified"
	StatusCandidate = "candidate"
	StatusReview    = "review_required"
)

type Action struct {
	ID                string     `json:"id"`
	Symbol            string     `json:"symbol"`
	Type              string     `json:"type"`
	Status            string     `json:"status"`
	AnnouncementDate  *time.Time `json:"announcement_date,omitempty"`
	EffectiveDate     *time.Time `json:"effective_date,omitempty"`
	Source            string     `json:"source"`
	SourceDataset     string     `json:"source_dataset,omitempty"`
	SourceFile        string     `json:"source_file,omitempty"`
	SourceURL         string     `json:"source_url,omitempty"`
	Title             string     `json:"title,omitempty"`
	Ratio             *float64   `json:"ratio,omitempty"`
	CashAmount        *float64   `json:"cash_amount,omitempty"`
	SubscriptionPrice *float64   `json:"subscription_price,omitempty"`
	Currency          string     `json:"currency,omitempty"`
	AdjustmentFactor  *float64   `json:"adjustment_factor,omitempty"`
	Confidence        float64    `json:"confidence,omitempty"`
	ReviewRequired    bool       `json:"review_required,omitempty"`
	Warnings          []string   `json:"warnings,omitempty"`
}

type ActionSet struct {
	Symbol                      string   `json:"symbol"`
	Status                      string   `json:"status"`
	Actions                     []Action `json:"actions,omitempty"`
	VerifiedActions             int      `json:"verified_actions"`
	CandidateActions            int      `json:"candidate_actions"`
	ReviewRequiredActions       int      `json:"review_required_actions"`
	AdjustmentReadyActions      int      `json:"adjustment_ready_actions"`
	MissingEffectiveDateActions int      `json:"missing_effective_date_actions"`
	MissingAdjustmentActions    int      `json:"missing_adjustment_actions"`
	SourceFiles                 []string `json:"source_files,omitempty"`
	Warnings                    []string `json:"warnings,omitempty"`
}

type AdjustmentReport struct {
	Symbol                    string   `json:"symbol,omitempty"`
	Timeframe                 string   `json:"timeframe,omitempty"`
	PriceSeries               string   `json:"price_series"`
	ActionsConsidered         int      `json:"actions_considered"`
	AppliedActions            int      `json:"applied_actions"`
	SkippedActions            int      `json:"skipped_actions"`
	AdjustedCandles           int      `json:"adjusted_candles"`
	UnadjustedCandles         int      `json:"unadjusted_candles"`
	PotentialCorporateGapBars int      `json:"potential_corporate_gap_bars"`
	BacktestSafe              bool     `json:"backtest_safe"`
	AppliedActionIDs          []string `json:"applied_action_ids,omitempty"`
	SkippedActionIDs          []string `json:"skipped_action_ids,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
}
