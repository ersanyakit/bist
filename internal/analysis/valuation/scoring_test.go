package valuation

import (
	"context"
	"testing"

	"hissebot/internal/domain/financials"
)

func TestScoreValuation(t *testing.T) {
	tests := []struct {
		name         string
		statements   financials.Statements
		currentPrice float64
		want         float64
	}{
		{name: "missing price", statements: financials.Statements{BalanceSheet: financials.BalanceSheet{TotalEquity: 100}, IncomeStatement: financials.IncomeStatement{NetIncome: 20}}, currentPrice: 0, want: 35},
		{name: "missing equity", statements: financials.Statements{BalanceSheet: financials.BalanceSheet{TotalEquity: 0}, IncomeStatement: financials.IncomeStatement{NetIncome: 20}}, currentPrice: 10, want: 35},
		{name: "loss making company", statements: financials.Statements{BalanceSheet: financials.BalanceSheet{TotalEquity: 100}, IncomeStatement: financials.IncomeStatement{NetIncome: -1}}, currentPrice: 10, want: 40},
		{name: "positive equity and earnings", statements: financials.Statements{BalanceSheet: financials.BalanceSheet{TotalEquity: 100}, IncomeStatement: financials.IncomeStatement{NetIncome: 20}}, currentPrice: 10, want: 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Service{}.ScoreValuation(context.Background(), tt.statements, tt.currentPrice)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("score = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}
