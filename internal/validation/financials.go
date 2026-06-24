package validation

import (
	"math"

	"hissebot/internal/domain/financials"
)

func ValidateFinancialStatements(statements financials.Statements, tolerance float64) Report {
	report := NewReport()
	if tolerance <= 0 {
		tolerance = 1
	}
	bs := statements.BalanceSheet
	if statements.Symbol == "" {
		report.Add(SeverityError, "symbol_missing", "symbol", "finansal tablo sembolü boş olamaz")
	}
	if bs.TotalAssets != 0 || bs.TotalLiabilities != 0 || bs.TotalEquity != 0 {
		diff := math.Abs(bs.TotalAssets - (bs.TotalLiabilities + bs.TotalEquity))
		if diff > tolerance {
			report.Add(SeverityCritical, "balance_sheet_not_balanced", "balance_sheet", "assets = liabilities + equity eşitliği bozuk")
		}
	}
	if bs.TotalEquity < 0 {
		report.Add(SeverityCritical, "negative_equity", "total_equity", "negatif özkaynak kırmızı bayraktır")
	}
	cf := statements.CashFlow
	if statements.IncomeStatement.NetIncome > 0 && cf.OperatingCashFlow < 0 {
		report.Add(SeverityWarning, "profit_without_cash_flow", "operating_cash_flow", "net kar pozitif ama operasyonel nakit akışı negatif")
	}
	if cf.FreeCashFlow == 0 && cf.OperatingCashFlow != 0 && cf.Capex != 0 {
		report.Add(SeverityInfo, "fcf_can_be_derived", "free_cash_flow", "serbest nakit akımı CFO ve capex üzerinden türetilebilir")
	}
	return report
}
