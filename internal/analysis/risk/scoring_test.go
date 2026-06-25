package risk

import (
	"context"
	"testing"

	"hissebot/internal/analysis"
	"hissebot/internal/domain/financials"
)

func TestScoreRisk(t *testing.T) {
	tests := []struct {
		name  string
		input analysis.Input
		want  float64
	}{
		{name: "clean input starts at baseline", input: analysis.Input{}, want: 75},
		{
			name: "missing data and negative equity reduce score",
			input: analysis.Input{
				MissingData: []string{"price", "financials"},
				Financials: financials.Statements{
					BalanceSheet: financials.BalanceSheet{TotalEquity: -10},
				},
			},
			want: 29,
		},
		{
			name: "many gaps floor at zero",
			input: analysis.Input{
				MissingData: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				Financials: financials.Statements{
					BalanceSheet: financials.BalanceSheet{TotalEquity: -1},
				},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Service{}.ScoreRisk(context.Background(), tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("score = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}
