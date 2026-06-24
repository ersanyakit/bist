package financials

import "time"

type PeriodType string

const (
	PeriodQuarterly PeriodType = "quarterly"
	PeriodAnnual    PeriodType = "annual"
)

type FinancialGroup string

const (
	GroupIndustrial FinancialGroup = "industrial"
	GroupBank       FinancialGroup = "bank"
	GroupInsurance  FinancialGroup = "insurance"
	GroupREIT       FinancialGroup = "reit"
	GroupHolding    FinancialGroup = "holding"
)

type Period struct {
	Year        int        `json:"year"`
	Quarter     int        `json:"quarter,omitempty"`
	Type        PeriodType `json:"type"`
	EndDate     time.Time  `json:"end_date"`
	PublishedAt time.Time  `json:"published_at,omitempty"`
}

type SourceMeta struct {
	Source        string    `json:"source"`
	SourceURL     string    `json:"source_url,omitempty"`
	Currency      string    `json:"currency"`
	Consolidation string    `json:"consolidation,omitempty"`
	Audited       bool      `json:"audited,omitempty"`
	DataVersion   string    `json:"data_version,omitempty"`
	AsOf          time.Time `json:"as_of"`
	IngestedAt    time.Time `json:"ingested_at"`
}

type StatementLine struct {
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type BalanceSheet struct {
	Symbol             string                   `json:"symbol"`
	Period             Period                   `json:"period"`
	TotalAssets        float64                  `json:"total_assets"`
	TotalLiabilities   float64                  `json:"total_liabilities"`
	TotalEquity        float64                  `json:"total_equity"`
	CashAndEquivalents float64                  `json:"cash_and_equivalents,omitempty"`
	Inventory          float64                  `json:"inventory,omitempty"`
	TradeReceivables   float64                  `json:"trade_receivables,omitempty"`
	ShortTermDebt      float64                  `json:"short_term_debt,omitempty"`
	LongTermDebt       float64                  `json:"long_term_debt,omitempty"`
	Lines              map[string]StatementLine `json:"lines,omitempty"`
	Meta               SourceMeta               `json:"meta"`
}

type IncomeStatement struct {
	Symbol      string                   `json:"symbol"`
	Period      Period                   `json:"period"`
	Sales       float64                  `json:"sales"`
	GrossProfit float64                  `json:"gross_profit,omitempty"`
	EBIT        float64                  `json:"ebit,omitempty"`
	EBITDA      float64                  `json:"ebitda,omitempty"`
	NetIncome   float64                  `json:"net_income"`
	EPS         float64                  `json:"eps,omitempty"`
	Lines       map[string]StatementLine `json:"lines,omitempty"`
	Meta        SourceMeta               `json:"meta"`
}

type CashFlowStatement struct {
	Symbol            string                   `json:"symbol"`
	Period            Period                   `json:"period"`
	OperatingCashFlow float64                  `json:"operating_cash_flow"`
	InvestingCashFlow float64                  `json:"investing_cash_flow,omitempty"`
	FinancingCashFlow float64                  `json:"financing_cash_flow,omitempty"`
	Capex             float64                  `json:"capex,omitempty"`
	FreeCashFlow      float64                  `json:"free_cash_flow,omitempty"`
	EndCash           float64                  `json:"end_cash,omitempty"`
	Lines             map[string]StatementLine `json:"lines,omitempty"`
	Meta              SourceMeta               `json:"meta"`
}

type EquityStatement struct {
	Symbol             string                   `json:"symbol"`
	Period             Period                   `json:"period"`
	OpeningEquity      float64                  `json:"opening_equity,omitempty"`
	ClosingEquity      float64                  `json:"closing_equity"`
	PaidInCapital      float64                  `json:"paid_in_capital,omitempty"`
	Dividends          float64                  `json:"dividends,omitempty"`
	ShareIssueProceeds float64                  `json:"share_issue_proceeds,omitempty"`
	Lines              map[string]StatementLine `json:"lines,omitempty"`
	Meta               SourceMeta               `json:"meta"`
}

type FinancialNote struct {
	Symbol    string     `json:"symbol"`
	Period    Period     `json:"period"`
	Topic     string     `json:"topic"`
	Text      string     `json:"text"`
	SourceURL string     `json:"source_url,omitempty"`
	Meta      SourceMeta `json:"meta"`
}

type AnnualReport struct {
	Symbol    string     `json:"symbol"`
	Period    Period     `json:"period"`
	SourceURL string     `json:"source_url"`
	Meta      SourceMeta `json:"meta"`
}

type AuditReport struct {
	Symbol    string     `json:"symbol"`
	Period    Period     `json:"period"`
	Opinion   string     `json:"opinion"`
	Auditor   string     `json:"auditor,omitempty"`
	SourceURL string     `json:"source_url,omitempty"`
	Meta      SourceMeta `json:"meta"`
}

type Statements struct {
	Symbol          string            `json:"symbol"`
	Period          Period            `json:"period"`
	Group           FinancialGroup    `json:"financial_group"`
	BalanceSheet    BalanceSheet      `json:"balance_sheet"`
	IncomeStatement IncomeStatement   `json:"income_statement"`
	CashFlow        CashFlowStatement `json:"cash_flow_statement"`
	EquityStatement EquityStatement   `json:"equity_statement"`
	Notes           []FinancialNote   `json:"financial_notes,omitempty"`
	AnnualReport    *AnnualReport     `json:"annual_report,omitempty"`
	AuditReport     *AuditReport      `json:"audit_report,omitempty"`
	Meta            SourceMeta        `json:"meta"`
}

type RatioSet struct {
	Symbol          string     `json:"symbol"`
	Period          Period     `json:"period"`
	MarketCap       float64    `json:"market_cap,omitempty"`
	EnterpriseValue float64    `json:"enterprise_value,omitempty"`
	NetDebt         float64    `json:"net_debt,omitempty"`
	PE              float64    `json:"pe,omitempty"`
	PB              float64    `json:"pb,omitempty"`
	PS              float64    `json:"ps,omitempty"`
	EVEBITDA        float64    `json:"ev_ebitda,omitempty"`
	EVEBIT          float64    `json:"ev_ebit,omitempty"`
	EVSales         float64    `json:"ev_sales,omitempty"`
	PFCF            float64    `json:"p_fcf,omitempty"`
	DividendYield   float64    `json:"dividend_yield,omitempty"`
	EarningsYield   float64    `json:"earnings_yield,omitempty"`
	FCFYield        float64    `json:"fcf_yield,omitempty"`
	ROE             float64    `json:"roe,omitempty"`
	ROIC            float64    `json:"roic,omitempty"`
	NetDebtEquity   float64    `json:"net_debt_equity,omitempty"`
	Meta            SourceMeta `json:"meta"`
}
