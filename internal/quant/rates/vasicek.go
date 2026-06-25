package rates

import (
	"errors"
	"math"

	"hissebot/internal/quant/core"
)

type VasicekParams struct {
	MeanReversion float64 `json:"mean_reversion"`
	LongRunMean   float64 `json:"long_run_mean"`
	Volatility    float64 `json:"volatility"`
	TimeStep      float64 `json:"time_step"`
}

func CalibrateVasicek(ratesSeries []float64, dt float64) (VasicekParams, error) {
	if len(ratesSeries) < 3 {
		return VasicekParams{}, errors.New("at least three rate observations are required")
	}
	if dt <= 0 {
		dt = 1.0 / 252.0
	}
	x := ratesSeries[:len(ratesSeries)-1]
	y := ratesSeries[1:]
	meanX := core.Mean(x)
	meanY := core.Mean(y)
	varXX := 0.0
	covXY := 0.0
	for i := range x {
		dx := x[i] - meanX
		varXX += dx * dx
		covXY += dx * (y[i] - meanY)
	}
	if math.Abs(varXX) <= core.Epsilon {
		return VasicekParams{}, errors.New("rate series variance is zero")
	}
	phi := covXY / varXX
	phi = core.Clamp(phi, 1e-8, 0.999999)
	c := meanY - phi*meanX
	a := -math.Log(phi) / dt
	b := c / (1 - phi)
	residuals := make([]float64, len(x))
	for i := range x {
		residuals[i] = y[i] - c - phi*x[i]
	}
	resVar := core.Variance(residuals, true)
	sigma := math.Sqrt(math.Max(resVar*2*a/(1-phi*phi), 0))
	return VasicekParams{MeanReversion: a, LongRunMean: b, Volatility: sigma, TimeStep: dt}, nil
}

func VasicekZeroCouponPrice(shortRate, maturity float64, p VasicekParams) float64 {
	if maturity <= 0 {
		return 1
	}
	a := p.MeanReversion
	if a <= 0 {
		return math.Exp(-shortRate * maturity)
	}
	b := p.LongRunMean
	sigma := p.Volatility
	bt := (1 - math.Exp(-a*maturity)) / a
	at := math.Exp((b-sigma*sigma/(2*a*a))*(bt-maturity) - sigma*sigma*bt*bt/(4*a))
	return at * math.Exp(-bt*shortRate)
}
