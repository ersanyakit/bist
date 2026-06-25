package core

import (
	"errors"
	"math"
	"sort"
)

const Epsilon = 1e-12

func IsFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func NormalPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}

func NormalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// NormalInvCDF is Peter J. Acklam's rational approximation for the standard normal quantile.
func NormalInvCDF(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}
	a := [...]float64{-3.969683028665376e+01, 2.209460984245205e+02, -2.759285104469687e+02, 1.383577518672690e+02, -3.066479806614716e+01, 2.506628277459239e+00}
	b := [...]float64{-5.447609879822406e+01, 1.615858368580409e+02, -1.556989798598866e+02, 6.680131188771972e+01, -1.328068155288572e+01}
	c := [...]float64{-7.784894002430293e-03, -3.223964580411365e-01, -2.400758277161838e+00, -2.549732539343734e+00, 4.374664141464968e+00, 2.938163982698783e+00}
	d := [...]float64{7.784695709041462e-03, 3.224671290700398e-01, 2.445134137142996e+00, 3.754408661907416e+00}
	plow := 0.02425
	phigh := 1 - plow
	if p < plow {
		q := math.Sqrt(-2 * math.Log(p))
		return (((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) /
			((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	}
	if p > phigh {
		q := math.Sqrt(-2 * math.Log(1-p))
		return -(((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) /
			((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	}
	q := p - 0.5
	r := q * q
	return (((((a[0]*r+a[1])*r+a[2])*r+a[3])*r+a[4])*r + a[5]) * q /
		(((((b[0]*r+b[1])*r+b[2])*r+b[3])*r+b[4])*r + 1)
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func Variance(values []float64, sample bool) float64 {
	if len(values) == 0 || (sample && len(values) < 2) {
		return 0
	}
	mean := Mean(values)
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	denom := float64(len(values))
	if sample {
		denom--
	}
	return sum / denom
}

func StdDev(values []float64, sample bool) float64 {
	return math.Sqrt(Variance(values, sample))
}

func Covariance(a, b []float64, sample bool) float64 {
	n := min(len(a), len(b))
	if n == 0 || (sample && n < 2) {
		return 0
	}
	ma := Mean(a[:n])
	mb := Mean(b[:n])
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += (a[i] - ma) * (b[i] - mb)
	}
	denom := float64(n)
	if sample {
		denom--
	}
	return sum / denom
}

func Correlation(a, b []float64) float64 {
	cov := Covariance(a, b, true)
	sa := StdDev(a[:min(len(a), len(b))], true)
	sb := StdDev(b[:min(len(a), len(b))], true)
	if sa <= Epsilon || sb <= Epsilon {
		return 0
	}
	return cov / (sa * sb)
}

func Returns(prices []float64) []float64 {
	if len(prices) < 2 {
		return nil
	}
	out := make([]float64, 0, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if math.Abs(prices[i-1]) <= Epsilon {
			out = append(out, 0)
			continue
		}
		out = append(out, prices[i]/prices[i-1]-1)
	}
	return out
}

func LogReturns(prices []float64) []float64 {
	if len(prices) < 2 {
		return nil
	}
	out := make([]float64, 0, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i] <= 0 || prices[i-1] <= 0 {
			out = append(out, 0)
			continue
		}
		out = append(out, math.Log(prices[i]/prices[i-1]))
	}
	return out
}

func Quantile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	p = Clamp(p, 0, 1)
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	if len(cp) == 1 {
		return cp[0]
	}
	pos := p * float64(len(cp)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return cp[lo]
	}
	w := pos - float64(lo)
	return cp[lo]*(1-w) + cp[hi]*w
}

func Dot(a, b []float64) float64 {
	n := min(len(a), len(b))
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func MatVec(m [][]float64, v []float64) []float64 {
	out := make([]float64, len(m))
	for i := range m {
		out[i] = Dot(m[i], v)
	}
	return out
}

func MatrixInverse(m [][]float64) ([][]float64, error) {
	n := len(m)
	if n == 0 {
		return nil, errors.New("empty matrix")
	}
	for _, row := range m {
		if len(row) != n {
			return nil, errors.New("matrix must be square")
		}
	}
	aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		aug[i] = make([]float64, 2*n)
		copy(aug[i], m[i])
		aug[i][n+i] = 1
	}
	for col := 0; col < n; col++ {
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(aug[r][col]) > math.Abs(aug[pivot][col]) {
				pivot = r
			}
		}
		if math.Abs(aug[pivot][col]) <= Epsilon {
			return nil, errors.New("matrix is singular")
		}
		aug[col], aug[pivot] = aug[pivot], aug[col]
		scale := aug[col][col]
		for c := 0; c < 2*n; c++ {
			aug[col][c] /= scale
		}
		for r := 0; r < n; r++ {
			if r == col {
				continue
			}
			factor := aug[r][col]
			for c := 0; c < 2*n; c++ {
				aug[r][c] -= factor * aug[col][c]
			}
		}
	}
	inv := make([][]float64, n)
	for i := 0; i < n; i++ {
		inv[i] = append([]float64(nil), aug[i][n:]...)
	}
	return inv, nil
}

func AddRidge(m [][]float64, ridge float64) [][]float64 {
	out := make([][]float64, len(m))
	for i := range m {
		out[i] = append([]float64(nil), m[i]...)
		if i < len(out[i]) {
			out[i][i] += ridge
		}
	}
	return out
}

func NormalizeWeights(w []float64) []float64 {
	out := append([]float64(nil), w...)
	sum := 0.0
	for _, v := range out {
		sum += v
	}
	if math.Abs(sum) <= Epsilon {
		eq := 1 / float64(len(out))
		for i := range out {
			out[i] = eq
		}
		return out
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
