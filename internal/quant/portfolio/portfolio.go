package portfolio

import (
	"errors"
	"math"
	"sort"

	"hissebot/internal/quant/core"
)

type Point struct {
	Return     float64   `json:"return"`
	Volatility float64   `json:"volatility"`
	Weights    []float64 `json:"weights"`
}

type RiskReport struct {
	ExpectedReturn float64   `json:"expected_return"`
	Volatility     float64   `json:"volatility"`
	Variance       float64   `json:"variance"`
	VaR            float64   `json:"var"`
	CVaR           float64   `json:"cvar"`
	Sharpe         float64   `json:"sharpe"`
	Sortino        float64   `json:"sortino"`
	MaxDrawdown    float64   `json:"max_drawdown"`
	RiskContrib    []float64 `json:"risk_contribution"`
}

func CovarianceMatrix(returns [][]float64) [][]float64 {
	n := len(returns)
	out := make([][]float64, n)
	for i := range out {
		out[i] = make([]float64, n)
		for j := range out[i] {
			out[i][j] = core.Covariance(returns[i], returns[j], true)
		}
	}
	return out
}

func MeanReturns(returns [][]float64) []float64 {
	out := make([]float64, len(returns))
	for i := range returns {
		out[i] = core.Mean(returns[i])
	}
	return out
}

func PortfolioReturn(weights, expectedReturns []float64) float64 {
	return core.Dot(weights, expectedReturns)
}

func PortfolioVariance(weights []float64, covariance [][]float64) float64 {
	cv := core.MatVec(covariance, weights)
	return core.Dot(weights, cv)
}

func PortfolioVolatility(weights []float64, covariance [][]float64) float64 {
	return math.Sqrt(math.Max(PortfolioVariance(weights, covariance), 0))
}

func MinimumVariancePortfolio(covariance [][]float64) ([]float64, error) {
	n := len(covariance)
	if n == 0 {
		return nil, errors.New("empty covariance")
	}
	inv, err := inverseStable(covariance)
	if err != nil {
		return nil, err
	}
	ones := make([]float64, n)
	for i := range ones {
		ones[i] = 1
	}
	raw := core.MatVec(inv, ones)
	return core.NormalizeWeights(raw), nil
}

func MaximumSharpeRatioPortfolio(expectedReturns []float64, covariance [][]float64, riskFreeRate float64, longOnly bool) ([]float64, error) {
	n := len(expectedReturns)
	if n == 0 || len(covariance) != n {
		return nil, errors.New("dimension mismatch")
	}
	inv, err := inverseStable(covariance)
	if err != nil {
		return nil, err
	}
	excess := make([]float64, n)
	for i := range excess {
		excess[i] = expectedReturns[i] - riskFreeRate
	}
	raw := core.MatVec(inv, excess)
	if longOnly {
		for i := range raw {
			if raw[i] < 0 {
				raw[i] = 0
			}
		}
	}
	return core.NormalizeWeights(raw), nil
}

func TargetReturnPortfolio(expectedReturns []float64, covariance [][]float64, targetReturn float64) ([]float64, error) {
	n := len(expectedReturns)
	if n == 0 || len(covariance) != n {
		return nil, errors.New("dimension mismatch")
	}
	inv, err := inverseStable(covariance)
	if err != nil {
		return nil, err
	}
	ones := make([]float64, n)
	for i := range ones {
		ones[i] = 1
	}
	invOnes := core.MatVec(inv, ones)
	invMu := core.MatVec(inv, expectedReturns)
	a := core.Dot(ones, invOnes)
	b := core.Dot(ones, invMu)
	c := core.Dot(expectedReturns, invMu)
	d := a*c - b*b
	if math.Abs(d) <= core.Epsilon {
		return nil, errors.New("singular frontier")
	}
	lambda := (c - b*targetReturn) / d
	gamma := (a*targetReturn - b) / d
	w := make([]float64, n)
	for i := range w {
		w[i] = lambda*invOnes[i] + gamma*invMu[i]
	}
	return w, nil
}

func EfficientFrontier(expectedReturns []float64, covariance [][]float64, points int) ([]Point, error) {
	if points <= 1 {
		points = 20
	}
	minRet, maxRet := expectedReturns[0], expectedReturns[0]
	for _, r := range expectedReturns[1:] {
		if r < minRet {
			minRet = r
		}
		if r > maxRet {
			maxRet = r
		}
	}
	out := make([]Point, 0, points)
	for i := 0; i < points; i++ {
		target := minRet + (maxRet-minRet)*float64(i)/float64(points-1)
		w, err := TargetReturnPortfolio(expectedReturns, covariance, target)
		if err != nil {
			return nil, err
		}
		out = append(out, Point{Return: PortfolioReturn(w, expectedReturns), Volatility: PortfolioVolatility(w, covariance), Weights: w})
	}
	return out, nil
}

func BlackLittermanEquilibriumReturns(riskAversion float64, covariance [][]float64, marketWeights []float64) []float64 {
	return scale(core.MatVec(covariance, marketWeights), riskAversion)
}

func BlackLittermanPosteriorReturns(prior []float64, covariance [][]float64, pMatrix [][]float64, views []float64, omega [][]float64, tau float64) ([]float64, []float64, error) {
	n := len(prior)
	if n == 0 || tau <= 0 {
		return nil, nil, errors.New("invalid input")
	}
	tauSigma := scaleMatrix(covariance, tau)
	invTauSigma, err := inverseStable(tauSigma)
	if err != nil {
		return nil, nil, err
	}
	invOmega, err := inverseStable(omega)
	if err != nil {
		return nil, nil, err
	}
	pt := transpose(pMatrix)
	middle := matAdd(invTauSigma, matMul(matMul(pt, invOmega), pMatrix))
	invMiddle, err := inverseStable(middle)
	if err != nil {
		return nil, nil, err
	}
	right := vecAdd(core.MatVec(invTauSigma, prior), core.MatVec(matMul(pt, invOmega), views))
	posterior := core.MatVec(invMiddle, right)
	weights, err := MaximumSharpeRatioPortfolio(posterior, covariance, 0, true)
	return posterior, weights, err
}

func RiskParityPortfolio(covariance [][]float64, iterations int) ([]float64, error) {
	n := len(covariance)
	if n == 0 {
		return nil, errors.New("empty covariance")
	}
	if iterations <= 0 {
		iterations = 1000
	}
	w := make([]float64, n)
	for i := range w {
		w[i] = 1 / float64(n)
	}
	for iter := 0; iter < iterations; iter++ {
		rc := RiskContribution(w, covariance)
		total := 0.0
		for _, v := range rc {
			total += v
		}
		target := total / float64(n)
		for i := range w {
			if rc[i] > 0 {
				w[i] *= math.Sqrt(target / rc[i])
			}
			w[i] = math.Max(w[i], 1e-10)
		}
		w = core.NormalizeWeights(w)
	}
	return w, nil
}

func InverseVolatilityWeights(volatilities []float64) []float64 {
	w := make([]float64, len(volatilities))
	for i, vol := range volatilities {
		if vol > 0 {
			w[i] = 1 / vol
		}
	}
	return core.NormalizeWeights(w)
}

func HistoricalVaR(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	confidence = core.Clamp(confidence, 0, 1)
	return -core.Quantile(returns, 1-confidence)
}

func HistoricalCVaR(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	threshold := core.Quantile(returns, 1-core.Clamp(confidence, 0, 1))
	sum := 0.0
	count := 0
	for _, r := range returns {
		if r <= threshold {
			sum += r
			count++
		}
	}
	if count == 0 {
		return -threshold
	}
	return -sum / float64(count)
}

func ParametricVaR(mean, volatility, confidence float64) float64 {
	z := core.NormalInvCDF(1 - confidence)
	return -(mean + z*volatility)
}

func RiskContribution(weights []float64, covariance [][]float64) []float64 {
	marginal := core.MatVec(covariance, weights)
	variance := core.Dot(weights, marginal)
	out := make([]float64, len(weights))
	if variance <= core.Epsilon {
		return out
	}
	for i := range weights {
		out[i] = weights[i] * marginal[i] / variance
	}
	return out
}

func IncrementalVaR(weights []float64, covariance [][]float64, confidence float64) []float64 {
	vol := PortfolioVolatility(weights, covariance)
	if vol <= core.Epsilon {
		return make([]float64, len(weights))
	}
	z := math.Abs(core.NormalInvCDF(1 - confidence))
	marginal := core.MatVec(covariance, weights)
	out := make([]float64, len(weights))
	for i := range weights {
		out[i] = z * marginal[i] / vol
	}
	return out
}

func PerformanceRatios(returns []float64, riskFreeRatePerPeriod float64) (sharpe, sortino, maxDrawdown float64) {
	if len(returns) == 0 {
		return 0, 0, 0
	}
	excess := make([]float64, len(returns))
	downside := []float64{}
	equity := 1.0
	peak := 1.0
	for i, r := range returns {
		excess[i] = r - riskFreeRatePerPeriod
		if excess[i] < 0 {
			downside = append(downside, excess[i])
		}
		equity *= 1 + r
		if equity > peak {
			peak = equity
		}
		dd := equity/peak - 1
		if dd < maxDrawdown {
			maxDrawdown = dd
		}
	}
	if sd := core.StdDev(excess, true); sd > 0 {
		sharpe = core.Mean(excess) / sd * math.Sqrt(252)
	}
	if dsd := core.StdDev(downside, true); dsd > 0 {
		sortino = core.Mean(excess) / dsd * math.Sqrt(252)
	}
	return sharpe, sortino, maxDrawdown
}

func ComprehensiveRiskAnalysis(weights, expectedReturns []float64, covariance [][]float64, historicalReturns []float64, confidence float64, riskFreeRate float64) RiskReport {
	vol := PortfolioVolatility(weights, covariance)
	variance := vol * vol
	sharpe := 0.0
	if vol > 0 {
		sharpe = (PortfolioReturn(weights, expectedReturns) - riskFreeRate) / vol
	}
	sortino, maxDD := 0.0, 0.0
	if len(historicalReturns) > 0 {
		_, sortino, maxDD = PerformanceRatios(historicalReturns, riskFreeRate/252)
	}
	return RiskReport{
		ExpectedReturn: PortfolioReturn(weights, expectedReturns),
		Volatility:     vol,
		Variance:       variance,
		VaR:            HistoricalVaR(historicalReturns, confidence),
		CVaR:           HistoricalCVaR(historicalReturns, confidence),
		Sharpe:         sharpe,
		Sortino:        sortino,
		MaxDrawdown:    maxDD,
		RiskContrib:    RiskContribution(weights, covariance),
	}
}

func PortfolioReturns(assetReturns [][]float64, weights []float64) []float64 {
	n := 0
	for _, r := range assetReturns {
		if len(r) > n {
			n = len(r)
		}
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		for a, series := range assetReturns {
			if a < len(weights) && i < len(series) {
				out[i] += weights[a] * series[i]
			}
		}
	}
	return out
}

func inverseStable(m [][]float64) ([][]float64, error) {
	if inv, err := core.MatrixInverse(m); err == nil {
		return inv, nil
	}
	return core.MatrixInverse(core.AddRidge(m, 1e-8))
}

func scale(values []float64, k float64) []float64 {
	out := make([]float64, len(values))
	for i := range values {
		out[i] = values[i] * k
	}
	return out
}

func scaleMatrix(m [][]float64, k float64) [][]float64 {
	out := make([][]float64, len(m))
	for i := range m {
		out[i] = make([]float64, len(m[i]))
		for j := range m[i] {
			out[i][j] = m[i][j] * k
		}
	}
	return out
}

func transpose(m [][]float64) [][]float64 {
	if len(m) == 0 {
		return nil
	}
	out := make([][]float64, len(m[0]))
	for i := range out {
		out[i] = make([]float64, len(m))
		for j := range m {
			out[i][j] = m[j][i]
		}
	}
	return out
}

func matMul(a, b [][]float64) [][]float64 {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make([][]float64, len(a))
	bt := transpose(b)
	for i := range a {
		out[i] = make([]float64, len(bt))
		for j := range bt {
			out[i][j] = core.Dot(a[i], bt[j])
		}
	}
	return out
}

func matAdd(a, b [][]float64) [][]float64 {
	out := make([][]float64, len(a))
	for i := range a {
		out[i] = make([]float64, len(a[i]))
		for j := range a[i] {
			if i < len(b) && j < len(b[i]) {
				out[i][j] = a[i][j] + b[i][j]
			} else {
				out[i][j] = a[i][j]
			}
		}
	}
	return out
}

func vecAdd(a, b []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i]
		if i < len(b) {
			out[i] += b[i]
		}
	}
	return out
}

func SortedWeights(weights []float64) []float64 {
	out := append([]float64(nil), weights...)
	sort.Float64s(out)
	return out
}
