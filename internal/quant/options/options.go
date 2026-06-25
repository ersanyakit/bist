package options

import (
	"errors"
	"math"

	"hissebot/internal/quant/core"
	"hissebot/internal/quant/solver"
)

type OptionType string

const (
	Call OptionType = "call"
	Put  OptionType = "put"
)

type Greeks struct {
	Delta float64 `json:"delta"`
	Gamma float64 `json:"gamma"`
	Vega  float64 `json:"vega"`
	Theta float64 `json:"theta"`
	Rho   float64 `json:"rho"`
}

type FullGreeks struct {
	Greeks
	Vanna float64 `json:"vanna"`
	Volga float64 `json:"volga"`
	Charm float64 `json:"charm"`
	Vomma float64 `json:"vomma"`
}

func BlackScholesPrice(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType) float64 {
	if maturity <= 0 || volatility <= 0 {
		return intrinsic(spot, strike, optionType)
	}
	if spot <= 0 || strike <= 0 {
		return 0
	}
	d1, d2 := blackScholesD1D2(spot, strike, rate, dividendYield, volatility, maturity)
	dfR := math.Exp(-rate * maturity)
	dfQ := math.Exp(-dividendYield * maturity)
	if optionType == Put {
		return strike*dfR*core.NormalCDF(-d2) - spot*dfQ*core.NormalCDF(-d1)
	}
	return spot*dfQ*core.NormalCDF(d1) - strike*dfR*core.NormalCDF(d2)
}

func BlackScholesGreeks(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType) Greeks {
	if spot <= 0 || strike <= 0 || volatility <= 0 || maturity <= 0 {
		return Greeks{}
	}
	d1, d2 := blackScholesD1D2(spot, strike, rate, dividendYield, volatility, maturity)
	dfR := math.Exp(-rate * maturity)
	dfQ := math.Exp(-dividendYield * maturity)
	sqrtT := math.Sqrt(maturity)
	sign := 1.0
	if optionType == Put {
		sign = -1
	}
	delta := sign * dfQ * core.NormalCDF(sign*d1)
	gamma := dfQ * core.NormalPDF(d1) / (spot * volatility * sqrtT)
	vega := spot * dfQ * core.NormalPDF(d1) * sqrtT
	theta := -spot*dfQ*core.NormalPDF(d1)*volatility/(2*sqrtT) -
		sign*rate*strike*dfR*core.NormalCDF(sign*d2) +
		sign*dividendYield*spot*dfQ*core.NormalCDF(sign*d1)
	rho := sign * strike * maturity * dfR * core.NormalCDF(sign*d2)
	return Greeks{Delta: delta, Gamma: gamma, Vega: vega, Theta: theta, Rho: rho}
}

func BlackScholesFullGreeks(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType) FullGreeks {
	base := BlackScholesGreeks(spot, strike, rate, dividendYield, volatility, maturity, optionType)
	if spot <= 0 || strike <= 0 || volatility <= 0 || maturity <= 0 {
		return FullGreeks{Greeks: base}
	}
	d1, d2 := blackScholesD1D2(spot, strike, rate, dividendYield, volatility, maturity)
	vanna := -math.Exp(-dividendYield*maturity) * core.NormalPDF(d1) * d2 / volatility
	volga := base.Vega * d1 * d2 / volatility
	return FullGreeks{Greeks: base, Vanna: vanna, Volga: volga, Vomma: volga, Charm: charm(spot, strike, rate, dividendYield, volatility, maturity, optionType)}
}

func BlackScholesImpliedVolatility(price, spot, strike, rate, dividendYield, maturity float64, optionType OptionType) (float64, error) {
	return impliedVol(price, 1e-6, 5, func(vol float64) float64 {
		return BlackScholesPrice(spot, strike, rate, dividendYield, vol, maturity, optionType)
	})
}

func Black76Price(forward, strike, rate, volatility, maturity float64, optionType OptionType) float64 {
	if maturity <= 0 || volatility <= 0 {
		return math.Exp(-rate*maturity) * intrinsic(forward, strike, optionType)
	}
	if forward <= 0 || strike <= 0 {
		return 0
	}
	df := math.Exp(-rate * maturity)
	sigT := volatility * math.Sqrt(maturity)
	d1 := (math.Log(forward/strike) + 0.5*volatility*volatility*maturity) / sigT
	d2 := d1 - sigT
	if optionType == Put {
		return df * (strike*core.NormalCDF(-d2) - forward*core.NormalCDF(-d1))
	}
	return df * (forward*core.NormalCDF(d1) - strike*core.NormalCDF(d2))
}

func Black76Greeks(forward, strike, rate, volatility, maturity float64, optionType OptionType) Greeks {
	if forward <= 0 || strike <= 0 || volatility <= 0 || maturity <= 0 {
		return Greeks{}
	}
	df := math.Exp(-rate * maturity)
	sqrtT := math.Sqrt(maturity)
	sigT := volatility * sqrtT
	d1 := (math.Log(forward/strike) + 0.5*volatility*volatility*maturity) / sigT
	sign := 1.0
	if optionType == Put {
		sign = -1
	}
	price := Black76Price(forward, strike, rate, volatility, maturity, optionType)
	return Greeks{
		Delta: sign * df * core.NormalCDF(sign*d1),
		Gamma: df * core.NormalPDF(d1) / (forward * volatility * sqrtT),
		Vega:  df * forward * core.NormalPDF(d1) * sqrtT,
		Theta: -forward*df*core.NormalPDF(d1)*volatility/(2*sqrtT) + rate*price,
		Rho:   -maturity * price,
	}
}

func Black76FullGreeks(forward, strike, rate, volatility, maturity float64, optionType OptionType) FullGreeks {
	base := Black76Greeks(forward, strike, rate, volatility, maturity, optionType)
	if forward <= 0 || strike <= 0 || volatility <= 0 || maturity <= 0 {
		return FullGreeks{Greeks: base}
	}
	sqrtT := math.Sqrt(maturity)
	sigT := volatility * sqrtT
	d1 := (math.Log(forward/strike) + 0.5*volatility*volatility*maturity) / sigT
	d2 := d1 - sigT
	volga := base.Vega * d1 * d2 / volatility
	vanna := -math.Exp(-rate*maturity) * core.NormalPDF(d1) * d2 / volatility
	return FullGreeks{Greeks: base, Vanna: vanna, Volga: volga, Vomma: volga}
}

func Black76ImpliedVolatility(price, forward, strike, rate, maturity float64, optionType OptionType) (float64, error) {
	return impliedVol(price, 1e-6, 5, func(vol float64) float64 {
		return Black76Price(forward, strike, rate, vol, maturity, optionType)
	})
}

func Black76CapletPrice(forwardRate, strikeRate, discountFactor, accrual, volatility, maturity float64) float64 {
	return accrual * discountFactor * blackForwardUndiscounted(forwardRate, strikeRate, volatility, maturity, Call)
}

func Black76FloorletPrice(forwardRate, strikeRate, discountFactor, accrual, volatility, maturity float64) float64 {
	return accrual * discountFactor * blackForwardUndiscounted(forwardRate, strikeRate, volatility, maturity, Put)
}

func Black76SwaptionPrice(forwardSwapRate, strikeRate, annuity, volatility, maturity float64, optionType OptionType) float64 {
	return annuity * blackForwardUndiscounted(forwardSwapRate, strikeRate, volatility, maturity, optionType)
}

func BachelierPrice(forward, strike, discountFactor, normalVolatility, maturity float64, optionType OptionType) float64 {
	if maturity <= 0 || normalVolatility <= 0 {
		return discountFactor * intrinsic(forward, strike, optionType)
	}
	std := normalVolatility * math.Sqrt(maturity)
	d := (forward - strike) / std
	if optionType == Put {
		return discountFactor * ((strike-forward)*core.NormalCDF(-d) + std*core.NormalPDF(d))
	}
	return discountFactor * ((forward-strike)*core.NormalCDF(d) + std*core.NormalPDF(d))
}

func BachelierGreeks(forward, strike, discountFactor, normalVolatility, maturity float64, optionType OptionType) Greeks {
	if maturity <= 0 || normalVolatility <= 0 {
		return Greeks{}
	}
	std := normalVolatility * math.Sqrt(maturity)
	d := (forward - strike) / std
	sign := 1.0
	if optionType == Put {
		sign = -1
	}
	return Greeks{
		Delta: sign * discountFactor * core.NormalCDF(sign*d),
		Gamma: discountFactor * core.NormalPDF(d) / std,
		Vega:  discountFactor * math.Sqrt(maturity) * core.NormalPDF(d),
		Theta: -discountFactor * normalVolatility * core.NormalPDF(d) / (2 * math.Sqrt(maturity)),
	}
}

func BachelierFullGreeks(forward, strike, discountFactor, normalVolatility, maturity float64, optionType OptionType) FullGreeks {
	return FullGreeks{Greeks: BachelierGreeks(forward, strike, discountFactor, normalVolatility, maturity, optionType)}
}

func BachelierImpliedVolatility(price, forward, strike, discountFactor, maturity float64, optionType OptionType) (float64, error) {
	return impliedVol(price, 1e-8, math.Max(math.Abs(forward-strike)+10*price/math.Max(discountFactor, 1e-12), 1), func(vol float64) float64 {
		return BachelierPrice(forward, strike, discountFactor, vol, maturity, optionType)
	})
}

func VolatilityLognormalToNormal(forward, strike, maturity, lognormalVol float64) float64 {
	if maturity <= 0 || lognormalVol <= 0 {
		return 0
	}
	avg := 0.5 * (forward + strike)
	if avg <= 0 {
		avg = math.Max(forward, strike)
	}
	return avg * lognormalVol
}

func VolatilityNormalToLognormal(forward, strike, maturity, normalVol float64) float64 {
	avg := 0.5 * (forward + strike)
	if avg <= 0 {
		return 0
	}
	return normalVol / avg
}

func ShiftedLognormalPrice(forward, strike, shift, discountFactor, volatility, maturity float64, optionType OptionType) float64 {
	return discountFactor * blackForwardUndiscounted(forward+shift, strike+shift, volatility, maturity, optionType)
}

func DigitalCashOrNothing(spot, strike, rate, dividendYield, volatility, maturity, cashPayoff float64, optionType OptionType) float64 {
	if spot <= 0 || strike <= 0 || volatility <= 0 || maturity <= 0 {
		if intrinsic(spot, strike, optionType) > 0 {
			return cashPayoff * math.Exp(-rate*maturity)
		}
		return 0
	}
	_, d2 := blackScholesD1D2(spot, strike, rate, dividendYield, volatility, maturity)
	df := math.Exp(-rate * maturity)
	if optionType == Put {
		return cashPayoff * df * core.NormalCDF(-d2)
	}
	return cashPayoff * df * core.NormalCDF(d2)
}

func AssetOrNothing(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType) float64 {
	if spot <= 0 || strike <= 0 || volatility <= 0 || maturity <= 0 {
		if intrinsic(spot, strike, optionType) > 0 {
			return spot * math.Exp(-dividendYield*maturity)
		}
		return 0
	}
	d1, _ := blackScholesD1D2(spot, strike, rate, dividendYield, volatility, maturity)
	dfQ := math.Exp(-dividendYield * maturity)
	if optionType == Put {
		return spot * dfQ * core.NormalCDF(-d1)
	}
	return spot * dfQ * core.NormalCDF(d1)
}

func MargrabeExchangeOption(s1, s2, q1, q2, vol1, vol2, correlation, maturity float64) float64 {
	if s1 <= 0 || s2 <= 0 || maturity <= 0 {
		return 0
	}
	sigma := math.Sqrt(math.Max(vol1*vol1+vol2*vol2-2*correlation*vol1*vol2, 0))
	if sigma <= 0 {
		return math.Max(s1*math.Exp(-q1*maturity)-s2*math.Exp(-q2*maturity), 0)
	}
	sigT := sigma * math.Sqrt(maturity)
	d1 := (math.Log(s1/s2) + (q2-q1+0.5*sigma*sigma)*maturity) / sigT
	d2 := d1 - sigT
	return s1*math.Exp(-q1*maturity)*core.NormalCDF(d1) - s2*math.Exp(-q2*maturity)*core.NormalCDF(d2)
}

func KirkSpreadOption(forward1, forward2, strike, discountFactor, vol1, vol2, correlation, maturity float64, optionType OptionType) float64 {
	denom := forward2 + strike
	if forward1 <= 0 || denom <= 0 {
		return 0
	}
	effVol := math.Sqrt(math.Max(vol1*vol1+math.Pow(forward2/denom*vol2, 2)-2*correlation*vol1*vol2*forward2/denom, 0))
	return discountFactor * blackForwardUndiscounted(forward1, denom, effVol, maturity, optionType)
}

func KirkSpreadOptionGreeks(forward1, forward2, strike, discountFactor, vol1, vol2, correlation, maturity float64, optionType OptionType) Greeks {
	price := func(f1, v1 float64) float64 {
		return KirkSpreadOption(f1, forward2, strike, discountFactor, v1, vol2, correlation, maturity, optionType)
	}
	hF := math.Max(math.Abs(forward1)*1e-4, 1e-4)
	hV := math.Max(math.Abs(vol1)*1e-4, 1e-5)
	up := price(forward1+hF, vol1)
	mid := price(forward1, vol1)
	down := price(forward1-hF, vol1)
	vegaUp := price(forward1, vol1+hV)
	vegaDown := price(forward1, math.Max(vol1-hV, 1e-8))
	return Greeks{
		Delta: (up - down) / (2 * hF),
		Gamma: (up - 2*mid + down) / (hF * hF),
		Vega:  (vegaUp - vegaDown) / (2 * hV),
	}
}

func BasketOptionLevy(forwards, weights []float64, strike, discountFactor float64, volatilities []float64, correlation [][]float64, maturity float64, optionType OptionType) float64 {
	n := min(len(forwards), min(len(weights), len(volatilities)))
	if n == 0 {
		return 0
	}
	mean := 0.0
	for i := 0; i < n; i++ {
		mean += weights[i] * forwards[i]
	}
	variance := 0.0
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			rho := 0.0
			if i == j {
				rho = 1
			} else if i < len(correlation) && j < len(correlation[i]) {
				rho = correlation[i][j]
			}
			variance += weights[i] * weights[j] * forwards[i] * forwards[j] * volatilities[i] * volatilities[j] * rho
		}
	}
	if mean <= 0 || variance <= 0 {
		return discountFactor * intrinsic(mean, strike, optionType)
	}
	effVol := math.Sqrt(variance) / mean
	return discountFactor * blackForwardUndiscounted(mean, strike, effVol, maturity, optionType)
}

type BinomialConfig struct {
	Steps         int
	American      bool
	BermudanSteps map[int]bool
	BarrierType   string
	Barrier       float64
	Rebate        float64
	DividendYield float64
}

func BinomialTreeOption(spot, strike, rate, volatility, maturity float64, optionType OptionType, cfg BinomialConfig) float64 {
	if cfg.Steps <= 0 {
		cfg.Steps = 200
	}
	if spot <= 0 || strike <= 0 || volatility <= 0 || maturity <= 0 {
		return intrinsic(spot, strike, optionType)
	}
	n := cfg.Steps
	dt := maturity / float64(n)
	u := math.Exp(volatility * math.Sqrt(dt))
	d := 1 / u
	disc := math.Exp(-rate * dt)
	p := (math.Exp((rate-cfg.DividendYield)*dt) - d) / (u - d)
	p = core.Clamp(p, 0, 1)
	values := make([]float64, n+1)
	for i := 0; i <= n; i++ {
		s := spot * math.Pow(u, float64(i)) * math.Pow(d, float64(n-i))
		values[i] = terminalPayoffWithBarrier(s, strike, optionType, cfg)
	}
	for step := n - 1; step >= 0; step-- {
		for i := 0; i <= step; i++ {
			s := spot * math.Pow(u, float64(i)) * math.Pow(d, float64(step-i))
			continuation := disc * (p*values[i+1] + (1-p)*values[i])
			if barrierHit(s, cfg) {
				values[i] = cfg.Rebate
				continue
			}
			canExercise := cfg.American || cfg.BermudanSteps[step]
			if canExercise {
				values[i] = math.Max(continuation, intrinsic(s, strike, optionType))
			} else {
				values[i] = continuation
			}
		}
	}
	return values[0]
}

func BinomialEuropean(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType, steps int) float64 {
	return BinomialTreeOption(spot, strike, rate, volatility, maturity, optionType, BinomialConfig{Steps: steps, DividendYield: dividendYield})
}

func BinomialAmerican(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType, steps int) float64 {
	return BinomialTreeOption(spot, strike, rate, volatility, maturity, optionType, BinomialConfig{Steps: steps, American: true, DividendYield: dividendYield})
}

func BinomialBermudan(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType, steps int, exerciseSteps []int) float64 {
	m := map[int]bool{}
	for _, step := range exerciseSteps {
		m[step] = true
	}
	return BinomialTreeOption(spot, strike, rate, volatility, maturity, optionType, BinomialConfig{Steps: steps, BermudanSteps: m, DividendYield: dividendYield})
}

func BinomialBarrier(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType, steps int, barrierType string, barrier, rebate float64) float64 {
	return BinomialTreeOption(spot, strike, rate, volatility, maturity, optionType, BinomialConfig{Steps: steps, BarrierType: barrierType, Barrier: barrier, Rebate: rebate, DividendYield: dividendYield})
}

func blackScholesD1D2(spot, strike, rate, dividendYield, volatility, maturity float64) (float64, float64) {
	sigT := volatility * math.Sqrt(maturity)
	d1 := (math.Log(spot/strike) + (rate-dividendYield+0.5*volatility*volatility)*maturity) / sigT
	return d1, d1 - sigT
}

func blackForwardUndiscounted(forward, strike, volatility, maturity float64, optionType OptionType) float64 {
	if maturity <= 0 || volatility <= 0 {
		return intrinsic(forward, strike, optionType)
	}
	if forward <= 0 || strike <= 0 {
		return 0
	}
	sigT := volatility * math.Sqrt(maturity)
	d1 := (math.Log(forward/strike) + 0.5*volatility*volatility*maturity) / sigT
	d2 := d1 - sigT
	if optionType == Put {
		return strike*core.NormalCDF(-d2) - forward*core.NormalCDF(-d1)
	}
	return forward*core.NormalCDF(d1) - strike*core.NormalCDF(d2)
}

func impliedVol(targetPrice, lo, hi float64, priceFn func(float64) float64) (float64, error) {
	if targetPrice <= 0 {
		return 0, errors.New("target price must be positive")
	}
	fn := func(v float64) float64 { return priceFn(v) - targetPrice }
	if fn(lo) > 0 {
		return lo, nil
	}
	for fn(hi) < 0 && hi < 20 {
		hi *= 2
	}
	return solver.Bisection(fn, lo, hi, solver.Options{Tolerance: 1e-10, MaxIterations: 200})
}

func intrinsic(underlying, strike float64, optionType OptionType) float64 {
	if optionType == Put {
		return math.Max(strike-underlying, 0)
	}
	return math.Max(underlying-strike, 0)
}

func charm(spot, strike, rate, dividendYield, volatility, maturity float64, optionType OptionType) float64 {
	d1, d2 := blackScholesD1D2(spot, strike, rate, dividendYield, volatility, maturity)
	dfQ := math.Exp(-dividendYield * maturity)
	sign := 1.0
	if optionType == Put {
		sign = -1
	}
	return dividendYield*sign*dfQ*core.NormalCDF(sign*d1) -
		dfQ*core.NormalPDF(d1)*(2*(rate-dividendYield)*maturity-d2*volatility*math.Sqrt(maturity))/(2*maturity*volatility*math.Sqrt(maturity))
}

func terminalPayoffWithBarrier(spot, strike float64, optionType OptionType, cfg BinomialConfig) float64 {
	if barrierHit(spot, cfg) {
		return cfg.Rebate
	}
	return intrinsic(spot, strike, optionType)
}

func barrierHit(spot float64, cfg BinomialConfig) bool {
	if cfg.Barrier <= 0 || cfg.BarrierType == "" {
		return false
	}
	switch cfg.BarrierType {
	case "up-and-out", "up_out":
		return spot >= cfg.Barrier
	case "down-and-out", "down_out":
		return spot <= cfg.Barrier
	default:
		return false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
