package validation

import (
	"testing"

	"hissebot/internal/domain/financials"
)

func TestValidateFinancialStatementsRejectsBrokenBalanceSheet(t *testing.T) {
	report := ValidateFinancialStatements(financials.Statements{
		Symbol: "TEST",
		BalanceSheet: financials.BalanceSheet{
			TotalAssets:      100,
			TotalLiabilities: 80,
			TotalEquity:      10,
		},
	}, 0.01)

	if report.Status != "fail" {
		t.Fatalf("status = %s, want fail: %+v", report.Status, report)
	}
}

func TestValidateFinancialStatementsWarnsProfitWithoutCash(t *testing.T) {
	report := ValidateFinancialStatements(financials.Statements{
		Symbol: "TEST",
		BalanceSheet: financials.BalanceSheet{
			TotalAssets:      100,
			TotalLiabilities: 40,
			TotalEquity:      60,
		},
		IncomeStatement: financials.IncomeStatement{NetIncome: 10},
		CashFlow:        financials.CashFlowStatement{OperatingCashFlow: -5},
	}, 0.01)

	if report.Status != "limited" {
		t.Fatalf("status = %s, want limited: %+v", report.Status, report)
	}
}
