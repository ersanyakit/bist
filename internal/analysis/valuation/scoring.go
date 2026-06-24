package valuation

import (
	"context"

	"hissebot/internal/domain/financials"
)

type Service struct{}

func (Service) ScoreValuation(_ context.Context, statements financials.Statements, currentPrice float64) (float64, []string, error) {
	if currentPrice <= 0 || statements.BalanceSheet.TotalEquity <= 0 {
		return 35, []string{"fiyat veya özkaynak eksik; değerleme güveni düşük"}, nil
	}
	if statements.IncomeStatement.NetIncome <= 0 {
		return 40, []string{"net kar pozitif değil; F/K temelli değerleme zayıf"}, nil
	}
	return 60, []string{"temel değerleme için pozitif özkaynak ve net kar mevcut"}, nil
}
