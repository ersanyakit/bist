package portfolio

import (
	"math"
	"testing"
)

func TestPortfolioOptimizersAndRisk(t *testing.T) {
	mu := []float64{0.08, 0.12, 0.05}
	cov := [][]float64{
		{0.04, 0.006, 0.004},
		{0.006, 0.09, 0.008},
		{0.004, 0.008, 0.025},
	}
	minVar, err := MinimumVariancePortfolio(cov)
	if err != nil {
		t.Fatalf("min var: %v", err)
	}
	if math.Abs(sum(minVar)-1) > 1e-10 {
		t.Fatalf("min var weights do not sum to 1: %+v", minVar)
	}
	maxSharpe, err := MaximumSharpeRatioPortfolio(mu, cov, 0.02, true)
	if err != nil {
		t.Fatalf("max sharpe: %v", err)
	}
	if math.Abs(sum(maxSharpe)-1) > 1e-10 {
		t.Fatalf("max sharpe weights do not sum to 1: %+v", maxSharpe)
	}
	frontier, err := EfficientFrontier(mu, cov, 5)
	if err != nil || len(frontier) != 5 {
		t.Fatalf("frontier len=%d err=%v", len(frontier), err)
	}
	rp, err := RiskParityPortfolio(cov, 300)
	if err != nil || math.Abs(sum(rp)-1) > 1e-8 {
		t.Fatalf("risk parity=%+v err=%v", rp, err)
	}
	returns := []float64{-0.03, -0.02, -0.01, 0.01, 0.02}
	if v := HistoricalVaR(returns, 0.95); v <= 0 {
		t.Fatalf("VaR=%.8f", v)
	}
	report := ComprehensiveRiskAnalysis(maxSharpe, mu, cov, returns, 0.95, 0.02)
	if report.Volatility <= 0 || len(report.RiskContrib) != 3 {
		t.Fatalf("bad risk report: %+v", report)
	}
}

func sum(values []float64) float64 {
	out := 0.0
	for _, v := range values {
		out += v
	}
	return out
}
