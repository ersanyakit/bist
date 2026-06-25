package rates

import (
	"errors"
	"math"
	"sort"

	"hissebot/internal/quant/core"
)

type Compounding string

const (
	Continuous Compounding = "continuous"
	Simple     Compounding = "simple"
	Annual     Compounding = "annual"
	SemiAnnual Compounding = "semiannual"
	Quarterly  Compounding = "quarterly"
	Monthly    Compounding = "monthly"
)

type MarketInstrumentType string

const (
	DepositInstrument MarketInstrumentType = "deposit"
	ZeroInstrument    MarketInstrumentType = "zero"
	SwapInstrument    MarketInstrumentType = "swap"
)

type MarketInstrument struct {
	Type      MarketInstrumentType `json:"type"`
	Maturity  float64              `json:"maturity"`
	Rate      float64              `json:"rate"`
	Frequency int                  `json:"frequency,omitempty"`
}

type DiscountCurve struct {
	Times     []float64 `json:"times"`
	Discounts []float64 `json:"discounts"`
}

func Frequency(comp Compounding) int {
	switch comp {
	case Annual:
		return 1
	case SemiAnnual:
		return 2
	case Quarterly:
		return 4
	case Monthly:
		return 12
	default:
		return 0
	}
}

func DiscountFactorFromRate(rate, maturity float64, comp Compounding) float64 {
	if maturity <= 0 {
		return 1
	}
	switch comp {
	case Continuous:
		return math.Exp(-rate * maturity)
	case Simple:
		return 1 / (1 + rate*maturity)
	default:
		freq := Frequency(comp)
		if freq <= 0 {
			freq = 1
		}
		return math.Pow(1+rate/float64(freq), -float64(freq)*maturity)
	}
}

func ZeroRateFromDiscountFactor(df, maturity float64, comp Compounding) float64 {
	if df <= 0 || maturity <= 0 {
		return 0
	}
	switch comp {
	case Continuous:
		return -math.Log(df) / maturity
	case Simple:
		return (1/df - 1) / maturity
	default:
		freq := Frequency(comp)
		if freq <= 0 {
			freq = 1
		}
		return float64(freq) * (math.Pow(1/df, 1/(float64(freq)*maturity)) - 1)
	}
}

func ForwardRateFromDiscountFactors(dfStart, dfEnd, tStart, tEnd float64, comp Compounding) float64 {
	if dfStart <= 0 || dfEnd <= 0 || tEnd <= tStart {
		return 0
	}
	ratio := dfStart / dfEnd
	dt := tEnd - tStart
	switch comp {
	case Continuous:
		return math.Log(ratio) / dt
	case Simple:
		return (ratio - 1) / dt
	default:
		freq := Frequency(comp)
		if freq <= 0 {
			freq = 1
		}
		return float64(freq) * (math.Pow(ratio, 1/(float64(freq)*dt)) - 1)
	}
}

func NewDiscountCurve(times, discounts []float64) (DiscountCurve, error) {
	if len(times) != len(discounts) || len(times) == 0 {
		return DiscountCurve{}, errors.New("times and discounts must have equal non-zero length")
	}
	type point struct {
		t  float64
		df float64
	}
	points := make([]point, len(times))
	for i := range times {
		if times[i] < 0 || discounts[i] <= 0 {
			return DiscountCurve{}, errors.New("invalid curve point")
		}
		points[i] = point{t: times[i], df: discounts[i]}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].t < points[j].t })
	out := DiscountCurve{Times: make([]float64, len(points)), Discounts: make([]float64, len(points))}
	for i, p := range points {
		out.Times[i] = p.t
		out.Discounts[i] = p.df
	}
	return out, nil
}

func (c DiscountCurve) Discount(t float64) float64 {
	if len(c.Times) == 0 {
		return 0
	}
	if t <= c.Times[0] {
		return c.Discounts[0]
	}
	last := len(c.Times) - 1
	if t >= c.Times[last] {
		z := ZeroRateFromDiscountFactor(c.Discounts[last], c.Times[last], Continuous)
		return DiscountFactorFromRate(z, t, Continuous)
	}
	for i := 1; i < len(c.Times); i++ {
		if t <= c.Times[i] {
			t0, t1 := c.Times[i-1], c.Times[i]
			l0, l1 := math.Log(c.Discounts[i-1]), math.Log(c.Discounts[i])
			w := (t - t0) / (t1 - t0)
			return math.Exp(l0*(1-w) + l1*w)
		}
	}
	return 0
}

func (c DiscountCurve) ZeroRate(t float64, comp Compounding) float64 {
	return ZeroRateFromDiscountFactor(c.Discount(t), t, comp)
}

func (c DiscountCurve) ForwardRate(tStart, tEnd float64, comp Compounding) float64 {
	return ForwardRateFromDiscountFactors(c.Discount(tStart), c.Discount(tEnd), tStart, tEnd, comp)
}

func ParSwapRate(curve DiscountCurve, maturity float64, fixedFrequency int) float64 {
	if maturity <= 0 {
		return 0
	}
	if fixedFrequency <= 0 {
		fixedFrequency = 1
	}
	dt := 1 / float64(fixedFrequency)
	annuity := 0.0
	for t := dt; t <= maturity+1e-9; t += dt {
		annuity += dt * curve.Discount(t)
	}
	if annuity <= 0 {
		return 0
	}
	return (curve.Discount(0) - curve.Discount(maturity)) / annuity
}

func BootstrapDiscountCurve(instruments []MarketInstrument) (DiscountCurve, error) {
	if len(instruments) == 0 {
		return DiscountCurve{}, errors.New("no instruments")
	}
	items := append([]MarketInstrument(nil), instruments...)
	sort.Slice(items, func(i, j int) bool { return items[i].Maturity < items[j].Maturity })
	times := []float64{0}
	dfs := []float64{1}
	curve := DiscountCurve{Times: times, Discounts: dfs}
	for _, inst := range items {
		if inst.Maturity <= 0 {
			return DiscountCurve{}, errors.New("instrument maturity must be positive")
		}
		switch inst.Type {
		case DepositInstrument, ZeroInstrument, "":
			times = append(times, inst.Maturity)
			dfs = append(dfs, DiscountFactorFromRate(inst.Rate, inst.Maturity, Simple))
		case SwapInstrument:
			freq := inst.Frequency
			if freq <= 0 {
				freq = 1
			}
			dt := 1 / float64(freq)
			annuityKnown := 0.0
			for t := dt; t < inst.Maturity-1e-9; t += dt {
				annuityKnown += dt * curve.Discount(t)
			}
			dfN := (1 - inst.Rate*annuityKnown) / (1 + inst.Rate*dt)
			if dfN <= 0 || !core.IsFinite(dfN) {
				return DiscountCurve{}, errors.New("bootstrap produced invalid discount factor")
			}
			times = append(times, inst.Maturity)
			dfs = append(dfs, dfN)
		default:
			return DiscountCurve{}, errors.New("unsupported instrument type")
		}
		var err error
		curve, err = NewDiscountCurve(times, dfs)
		if err != nil {
			return DiscountCurve{}, err
		}
	}
	return curve, nil
}

func ConvexityAdjustment(volatility, maturity, tenor float64) float64 {
	if volatility <= 0 || maturity <= 0 || tenor <= 0 {
		return 0
	}
	return 0.5 * volatility * volatility * maturity * tenor
}

func ForwardToFuturesRate(forwardRate, convexityAdjustment float64) float64 {
	return forwardRate + convexityAdjustment
}

func FuturesToForwardRate(futuresRate, convexityAdjustment float64) float64 {
	return futuresRate - convexityAdjustment
}
