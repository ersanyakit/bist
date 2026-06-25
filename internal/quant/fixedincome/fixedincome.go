package fixedincome

import (
	"errors"
	"math"
	"sort"
	"time"

	"hissebot/internal/quant/core"
	"hissebot/internal/quant/rates"
	"hissebot/internal/quant/solver"
)

type Cashflow struct {
	Time   float64 `json:"time"`
	Amount float64 `json:"amount"`
}

type DatedCashflow struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
}

type BondAnalytics struct {
	Price            float64 `json:"price"`
	YieldToMaturity  float64 `json:"yield_to_maturity"`
	MacaulayDuration float64 `json:"macaulay_duration"`
	ModifiedDuration float64 `json:"modified_duration"`
	Convexity        float64 `json:"convexity"`
	PV01             float64 `json:"pv01"`
	DV01             float64 `json:"dv01"`
}

func FixedBondCashflows(face, couponRate, maturity float64, frequency int) []Cashflow {
	if face <= 0 || maturity <= 0 {
		return nil
	}
	if frequency <= 0 {
		frequency = 1
	}
	n := int(math.Round(maturity * float64(frequency)))
	if n <= 0 {
		n = 1
	}
	cfs := make([]Cashflow, n)
	coupon := face * couponRate / float64(frequency)
	for i := 1; i <= n; i++ {
		amount := coupon
		if i == n {
			amount += face
		}
		cfs[i-1] = Cashflow{Time: float64(i) / float64(frequency), Amount: amount}
	}
	return cfs
}

func PresentValue(cashflows []Cashflow, rate float64, comp rates.Compounding) float64 {
	pv := 0.0
	for _, cf := range cashflows {
		pv += cf.Amount * rates.DiscountFactorFromRate(rate, cf.Time, comp)
	}
	return pv
}

func FixedBondPrice(face, couponRate, maturity float64, frequency int, yield float64, comp rates.Compounding) float64 {
	return PresentValue(FixedBondCashflows(face, couponRate, maturity, frequency), yield, comp)
}

func YieldToMaturity(cashflows []Cashflow, marketPrice float64, comp rates.Compounding) (float64, error) {
	if len(cashflows) == 0 || marketPrice <= 0 {
		return 0, errors.New("cashflows and market price are required")
	}
	fn := func(y float64) float64 {
		return PresentValue(cashflows, y, comp) - marketPrice
	}
	return solver.Bisection(fn, -0.95, 5, solver.Options{Tolerance: 1e-12, MaxIterations: 200})
}

func MacaulayDuration(cashflows []Cashflow, yield float64, comp rates.Compounding) float64 {
	price := PresentValue(cashflows, yield, comp)
	if price <= 0 {
		return 0
	}
	sum := 0.0
	for _, cf := range cashflows {
		df := rates.DiscountFactorFromRate(yield, cf.Time, comp)
		sum += cf.Time * cf.Amount * df
	}
	return sum / price
}

func ModifiedDuration(cashflows []Cashflow, yield float64, comp rates.Compounding, frequency int) float64 {
	mac := MacaulayDuration(cashflows, yield, comp)
	if comp == rates.Continuous {
		return mac
	}
	if frequency <= 0 {
		frequency = rates.Frequency(comp)
	}
	if frequency <= 0 {
		frequency = 1
	}
	return mac / (1 + yield/float64(frequency))
}

func Convexity(cashflows []Cashflow, yield float64, comp rates.Compounding, frequency int) float64 {
	price := PresentValue(cashflows, yield, comp)
	if price <= 0 {
		return 0
	}
	if frequency <= 0 {
		frequency = rates.Frequency(comp)
	}
	if frequency <= 0 {
		frequency = 1
	}
	sum := 0.0
	for _, cf := range cashflows {
		df := rates.DiscountFactorFromRate(yield, cf.Time, comp)
		if comp == rates.Continuous {
			sum += cf.Amount * df * cf.Time * cf.Time
		} else {
			n := cf.Time * float64(frequency)
			sum += cf.Amount * df * n * (n + 1) / math.Pow(float64(frequency)*(1+yield/float64(frequency)), 2)
		}
	}
	return sum / price
}

func PV01(cashflows []Cashflow, yield float64, comp rates.Compounding) float64 {
	down := PresentValue(cashflows, yield-0.0001, comp)
	up := PresentValue(cashflows, yield+0.0001, comp)
	return (down - up) / 2
}

func FixedBondAnalytics(face, couponRate, maturity float64, frequency int, marketPrice float64, comp rates.Compounding) (BondAnalytics, error) {
	cfs := FixedBondCashflows(face, couponRate, maturity, frequency)
	ytm, err := YieldToMaturity(cfs, marketPrice, comp)
	if err != nil {
		return BondAnalytics{}, err
	}
	pv01 := PV01(cfs, ytm, comp)
	return BondAnalytics{
		Price:            marketPrice,
		YieldToMaturity:  ytm,
		MacaulayDuration: MacaulayDuration(cfs, ytm, comp),
		ModifiedDuration: ModifiedDuration(cfs, ytm, comp, frequency),
		Convexity:        Convexity(cfs, ytm, comp, frequency),
		PV01:             pv01,
		DV01:             pv01,
	}, nil
}

func InternalRateOfReturn(cashflows []Cashflow) (float64, error) {
	if len(cashflows) < 2 {
		return 0, errors.New("at least two cashflows are required")
	}
	fn := func(r float64) float64 {
		sum := 0.0
		for _, cf := range cashflows {
			sum += cf.Amount / math.Pow(1+r, cf.Time)
		}
		return sum
	}
	return solver.Bisection(fn, -0.9999, 10, solver.Options{Tolerance: 1e-11, MaxIterations: 200})
}

func XIRR(cashflows []DatedCashflow) (float64, error) {
	if len(cashflows) < 2 {
		return 0, errors.New("at least two dated cashflows are required")
	}
	items := append([]DatedCashflow(nil), cashflows...)
	sort.Slice(items, func(i, j int) bool { return items[i].Date.Before(items[j].Date) })
	start := items[0].Date
	cfs := make([]Cashflow, len(items))
	for i, cf := range items {
		cfs[i] = Cashflow{Time: cf.Date.Sub(start).Hours() / 24 / 365, Amount: cf.Amount}
	}
	return InternalRateOfReturn(cfs)
}

func GSpread(bondYield, governmentYield float64) float64 {
	return bondYield - governmentYield
}

func ISpread(bondYield, swapRate float64) float64 {
	return bondYield - swapRate
}

func ZSpread(cashflows []Cashflow, curve rates.DiscountCurve, marketPrice float64) (float64, error) {
	if len(cashflows) == 0 || marketPrice <= 0 {
		return 0, errors.New("cashflows and market price are required")
	}
	fn := func(spread float64) float64 {
		pv := 0.0
		for _, cf := range cashflows {
			pv += cf.Amount * curve.Discount(cf.Time) * math.Exp(-spread*cf.Time)
		}
		return pv - marketPrice
	}
	return solver.Bisection(fn, -0.5, 5, solver.Options{Tolerance: 1e-11, MaxIterations: 200})
}

func OptionAdjustedSpread(cashflows []Cashflow, curve rates.DiscountCurve, marketPrice, embeddedOptionValue float64) (float64, error) {
	return ZSpread(cashflows, curve, marketPrice+embeddedOptionValue)
}

func AssetSwapSpread(couponRate, parSwapRate, price, par float64) float64 {
	if par <= 0 {
		par = 100
	}
	return couponRate - parSwapRate + (par-price)/par
}

func Basis(spot, futures float64) float64 {
	return futures - spot
}

func CostOfCarryRate(spot, forward, maturity float64) float64 {
	if spot <= 0 || forward <= 0 || maturity <= 0 {
		return 0
	}
	return math.Log(forward/spot) / maturity
}

func ImpliedRepoRate(cashPrice, invoicePrice, income, maturity float64) float64 {
	if cashPrice <= 0 || maturity <= 0 {
		return 0
	}
	return (invoicePrice + income - cashPrice) / cashPrice / maturity
}

func ZeroCouponPrice(face, rate, maturity float64, comp rates.Compounding) float64 {
	return face * rates.DiscountFactorFromRate(rate, maturity, comp)
}

func CleanPriceFromDirty(dirtyPrice, accruedInterest float64) float64 {
	return dirtyPrice - accruedInterest
}

func DirtyPriceFromClean(cleanPrice, accruedInterest float64) float64 {
	return cleanPrice + accruedInterest
}

func SpreadDurationApprox(cashflows []Cashflow, curve rates.DiscountCurve, spread float64) float64 {
	base := 0.0
	down := 0.0
	up := 0.0
	for _, cf := range cashflows {
		base += cf.Amount * curve.Discount(cf.Time) * math.Exp(-spread*cf.Time)
		down += cf.Amount * curve.Discount(cf.Time) * math.Exp(-(spread-0.0001)*cf.Time)
		up += cf.Amount * curve.Discount(cf.Time) * math.Exp(-(spread+0.0001)*cf.Time)
	}
	if base <= 0 {
		return 0
	}
	return ((down - up) / 2) / base / 0.0001
}

func ForwardPriceFromSpot(spot, carryRate, maturity float64) float64 {
	if maturity <= 0 {
		return spot
	}
	return spot * math.Exp(carryRate*maturity)
}

func EffectiveAnnualRate(rate float64, comp rates.Compounding) float64 {
	if comp == rates.Continuous {
		return math.Exp(rate) - 1
	}
	if comp == rates.Simple {
		return rate
	}
	freq := rates.Frequency(comp)
	if freq <= 0 {
		freq = 1
	}
	return math.Pow(1+rate/float64(freq), float64(freq)) - 1
}

func SanitizeCashflows(cashflows []Cashflow) []Cashflow {
	out := make([]Cashflow, 0, len(cashflows))
	for _, cf := range cashflows {
		if cf.Time >= 0 && core.IsFinite(cf.Amount) {
			out = append(out, cf)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out
}
