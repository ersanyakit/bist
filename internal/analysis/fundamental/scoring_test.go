package fundamental

import (
	"context"
	"testing"

	"hissebot/internal/domain/financials"
)

func TestScoreFundamental(t *testing.T) {
	tests := []struct {
		name       string
		statements financials.Statements
		want       float64
		wantNotes  int
	}{
		{
			name: "profitable cash generative positive equity",
			statements: financials.Statements{
				IncomeStatement: financials.IncomeStatement{NetIncome: 100},
				CashFlow:        financials.CashFlowStatement{OperatingCashFlow: 80},
				BalanceSheet:    financials.BalanceSheet{TotalEquity: 500},
			},
			want:      90,
			wantNotes: 3,
		},
		{
			name: "loss negative cash and negative equity floors above zero",
			statements: financials.Statements{
				IncomeStatement: financials.IncomeStatement{NetIncome: -10},
				CashFlow:        financials.CashFlowStatement{OperatingCashFlow: -5},
				BalanceSheet:    financials.BalanceSheet{TotalEquity: -1},
			},
			want:      0,
			wantNotes: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, notes, err := Service{}.ScoreFundamental(context.Background(), tt.statements)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("score = %.2f, want %.2f", got, tt.want)
			}
			if len(notes) != tt.wantNotes {
				t.Fatalf("notes = %d, want %d: %+v", len(notes), tt.wantNotes, notes)
			}
		})
	}
}
