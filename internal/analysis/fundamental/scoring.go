package fundamental

import (
	"context"

	"hissebot/internal/domain/financials"
)

type Service struct{}

func (Service) ScoreFundamental(_ context.Context, statements financials.Statements) (float64, []string, error) {
	score := 50.0
	notes := []string{}
	if statements.IncomeStatement.NetIncome > 0 {
		score += 15
		notes = append(notes, "net kar pozitif")
	} else {
		score -= 15
		notes = append(notes, "net kar pozitif değil")
	}
	if statements.CashFlow.OperatingCashFlow > 0 {
		score += 15
		notes = append(notes, "operasyonel nakit akışı pozitif")
	} else {
		score -= 15
		notes = append(notes, "operasyonel nakit akışı zayıf")
	}
	if statements.BalanceSheet.TotalEquity > 0 {
		score += 10
		notes = append(notes, "özkaynak pozitif")
	} else {
		score -= 25
		notes = append(notes, "özkaynak negatif veya eksik")
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, notes, nil
}
