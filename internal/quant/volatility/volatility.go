package volatility

import (
	"errors"
	"math"

	"hissebot/internal/quant/core"
	"hissebot/internal/quant/options"
)

type SurfacePoint struct {
	Maturity float64 `json:"maturity"`
	Strike   float64 `json:"strike"`
	Vol      float64 `json:"vol"`
}

type FlatSurface struct {
	Vol float64 `json:"vol"`
}

type TermStructure struct {
	Maturities []float64 `json:"maturities"`
	Vols       []float64 `json:"vols"`
}

type GridSurface struct {
	Maturities []float64   `json:"maturities"`
	Strikes    []float64   `json:"strikes"`
	Vols       [][]float64 `json:"vols"`
}

type SABRParams struct {
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
	Rho   float64 `json:"rho"`
	Nu    float64 `json:"nu"`
}

type CalibrationResult struct {
	Params SABRParams `json:"params"`
	RMSE   float64    `json:"rmse"`
}

func (s FlatSurface) Volatility(_, _ float64) float64 {
	return math.Max(s.Vol, 0)
}

func (ts TermStructure) Volatility(maturity float64) float64 {
	return linearInterp(ts.Maturities, ts.Vols, maturity)
}

func (g GridSurface) Volatility(maturity, strike float64) float64 {
	if len(g.Maturities) == 0 || len(g.Strikes) == 0 || len(g.Vols) != len(g.Maturities) {
		return 0
	}
	i0, i1, tw := bracket(g.Maturities, maturity)
	j0, j1, sw := bracket(g.Strikes, strike)
	v00 := gridValue(g, i0, j0)
	v01 := gridValue(g, i0, j1)
	v10 := gridValue(g, i1, j0)
	v11 := gridValue(g, i1, j1)
	v0 := v00*(1-sw) + v01*sw
	v1 := v10*(1-sw) + v11*sw
	return v0*(1-tw) + v1*tw
}

func TotalVariance(volatility, maturity float64) float64 {
	if volatility <= 0 || maturity <= 0 {
		return 0
	}
	return volatility * volatility * maturity
}

func BuildSurfaceFromPoints(points []SurfacePoint) (GridSurface, error) {
	if len(points) == 0 {
		return GridSurface{}, errors.New("no surface points")
	}
	maturities := uniqueSorted(points, func(p SurfacePoint) float64 { return p.Maturity })
	strikes := uniqueSorted(points, func(p SurfacePoint) float64 { return p.Strike })
	grid := make([][]float64, len(maturities))
	for i := range grid {
		grid[i] = make([]float64, len(strikes))
	}
	counts := make([][]int, len(maturities))
	for i := range counts {
		counts[i] = make([]int, len(strikes))
	}
	for _, p := range points {
		i := indexOf(maturities, p.Maturity)
		j := indexOf(strikes, p.Strike)
		grid[i][j] += p.Vol
		counts[i][j]++
	}
	for i := range grid {
		for j := range grid[i] {
			if counts[i][j] > 0 {
				grid[i][j] /= float64(counts[i][j])
			} else {
				grid[i][j] = nearestPointVol(points, maturities[i], strikes[j])
			}
		}
	}
	return GridSurface{Maturities: maturities, Strikes: strikes, Vols: grid}, nil
}

func SABRImpliedVolatility(forward, strike, maturity float64, p SABRParams) float64 {
	if forward <= 0 || strike <= 0 || maturity <= 0 || p.Alpha <= 0 {
		return 0
	}
	beta := core.Clamp(p.Beta, 0, 1)
	rho := core.Clamp(p.Rho, -0.999, 0.999)
	nu := math.Max(p.Nu, 0)
	if math.Abs(forward-strike) < 1e-12 {
		fb := math.Pow(forward, 1-beta)
		term1 := (math.Pow(1-beta, 2) / 24) * (p.Alpha * p.Alpha / math.Pow(forward, 2-2*beta))
		term2 := 0.25 * rho * beta * nu * p.Alpha / math.Pow(forward, 1-beta)
		term3 := (2 - 3*rho*rho) * nu * nu / 24
		return p.Alpha / fb * (1 + (term1+term2+term3)*maturity)
	}
	fk := forward * strike
	oneMinusBeta := 1 - beta
	logFK := math.Log(forward / strike)
	powFK := math.Pow(fk, 0.5*oneMinusBeta)
	z := (nu / p.Alpha) * powFK * logFK
	xz := math.Log((math.Sqrt(1-2*rho*z+z*z) + z - rho) / (1 - rho))
	denom := powFK * (1 + oneMinusBeta*oneMinusBeta*logFK*logFK/24 + math.Pow(oneMinusBeta, 4)*math.Pow(logFK, 4)/1920)
	term1 := math.Pow(oneMinusBeta, 2) * p.Alpha * p.Alpha / (24 * math.Pow(fk, oneMinusBeta))
	term2 := rho * beta * nu * p.Alpha / (4 * powFK)
	term3 := (2 - 3*rho*rho) * nu * nu / 24
	if math.Abs(xz) <= 1e-12 {
		return p.Alpha / denom * (1 + (term1+term2+term3)*maturity)
	}
	return p.Alpha / denom * z / xz * (1 + (term1+term2+term3)*maturity)
}

func SABRNormalVolatility(forward, strike, maturity float64, p SABRParams) float64 {
	return options.VolatilityLognormalToNormal(forward, strike, maturity, SABRImpliedVolatility(forward, strike, maturity, p))
}

func SABRSmile(forward, maturity float64, strikes []float64, p SABRParams) []SurfacePoint {
	out := make([]SurfacePoint, len(strikes))
	for i, strike := range strikes {
		out[i] = SurfacePoint{Maturity: maturity, Strike: strike, Vol: SABRImpliedVolatility(forward, strike, maturity, p)}
	}
	return out
}

func CalibrateSABR(forward, maturity float64, strikes, marketVols []float64, beta float64) CalibrationResult {
	n := min(len(strikes), len(marketVols))
	best := CalibrationResult{RMSE: math.Inf(1)}
	if n == 0 {
		return best
	}
	for alpha := 0.01; alpha <= 2.0001; alpha += 0.01 {
		for nu := 0.05; nu <= 3.0001; nu += 0.05 {
			for rho := -0.90; rho <= 0.9001; rho += 0.05 {
				p := SABRParams{Alpha: alpha, Beta: beta, Rho: rho, Nu: nu}
				errSq := 0.0
				for i := 0; i < n; i++ {
					diff := SABRImpliedVolatility(forward, strikes[i], maturity, p) - marketVols[i]
					errSq += diff * diff
				}
				rmse := math.Sqrt(errSq / float64(n))
				if rmse < best.RMSE {
					best = CalibrationResult{Params: p, RMSE: rmse}
				}
			}
		}
	}
	return best
}

func SABRProbabilityDensity(forward, maturity, strike float64, p SABRParams, bump float64) float64 {
	if bump <= 0 {
		bump = math.Max(strike*0.001, 1e-4)
	}
	call := func(k float64) float64 {
		vol := SABRImpliedVolatility(forward, k, maturity, p)
		return options.Black76Price(forward, k, 0, vol, maturity, options.Call)
	}
	return math.Max((call(strike-bump)-2*call(strike)+call(strike+bump))/(bump*bump), 0)
}

func SABRSmileDynamics(forward, maturity float64, strikes []float64, p SABRParams, forwardShock float64) []SurfacePoint {
	return SABRSmile(forward+forwardShock, maturity, strikes, p)
}

func ConstantLocalVolatility(vol float64) func(float64, float64) float64 {
	return func(_, _ float64) float64 { return math.Max(vol, 0) }
}

func ImpliedToLocalVolatility(surface GridSurface, forward, maturity, strike float64) float64 {
	if maturity <= 0 || strike <= 0 || forward <= 0 {
		return 0
	}
	dt := math.Max(maturity*0.01, 1e-4)
	dk := math.Max(strike*0.01, 1e-4)
	call := func(t, k float64) float64 {
		t = math.Max(t, 1e-6)
		vol := surface.Volatility(t, k)
		return options.Black76Price(forward, k, 0, vol, t, options.Call)
	}
	ct := (call(maturity+dt, strike) - call(math.Max(maturity-dt, 1e-6), strike)) / (2 * dt)
	ckk := (call(maturity, strike-dk) - 2*call(maturity, strike) + call(maturity, strike+dk)) / (dk * dk)
	denom := 0.5 * strike * strike * ckk
	if denom <= 0 || ct <= 0 {
		return 0
	}
	return math.Sqrt(ct / denom)
}

func linearInterp(xs, ys []float64, x float64) float64 {
	n := min(len(xs), len(ys))
	if n == 0 {
		return 0
	}
	if x <= xs[0] {
		return ys[0]
	}
	if x >= xs[n-1] {
		return ys[n-1]
	}
	for i := 1; i < n; i++ {
		if x <= xs[i] {
			w := (x - xs[i-1]) / (xs[i] - xs[i-1])
			return ys[i-1]*(1-w) + ys[i]*w
		}
	}
	return ys[n-1]
}

func bracket(xs []float64, x float64) (int, int, float64) {
	if len(xs) == 1 || x <= xs[0] {
		return 0, 0, 0
	}
	last := len(xs) - 1
	if x >= xs[last] {
		return last, last, 0
	}
	for i := 1; i < len(xs); i++ {
		if x <= xs[i] {
			return i - 1, i, (x - xs[i-1]) / (xs[i] - xs[i-1])
		}
	}
	return last, last, 0
}

func gridValue(g GridSurface, i, j int) float64 {
	if i < 0 || i >= len(g.Vols) || j < 0 || j >= len(g.Vols[i]) {
		return 0
	}
	return g.Vols[i][j]
}

func uniqueSorted(points []SurfacePoint, pick func(SurfacePoint) float64) []float64 {
	m := map[float64]bool{}
	for _, p := range points {
		m[pick(p)] = true
	}
	out := make([]float64, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func indexOf(values []float64, target float64) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return 0
}

func nearestPointVol(points []SurfacePoint, maturity, strike float64) float64 {
	best := points[0]
	bestDist := math.Inf(1)
	for _, p := range points {
		d := math.Abs(p.Maturity-maturity) + math.Abs(p.Strike-strike)/math.Max(strike, 1)
		if d < bestDist {
			best = p
			bestDist = d
		}
	}
	return best.Vol
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
