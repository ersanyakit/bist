package risk

import (
	"context"

	"hissebot/internal/analysis"
)

type Service struct{}

func (Service) ScoreRisk(_ context.Context, input analysis.Input) (float64, []string, error) {
	score := 75.0
	notes := []string{}
	if len(input.MissingData) > 0 {
		score -= float64(len(input.MissingData)) * 8
		notes = append(notes, "eksik veri confidence seviyesini düşürdü")
	}
	if input.Financials.BalanceSheet.TotalEquity < 0 {
		score -= 30
		notes = append(notes, "negatif özkaynak riski")
	}
	if score < 0 {
		score = 0
	}
	return score, notes, nil
}
