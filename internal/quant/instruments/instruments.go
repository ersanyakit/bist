package instruments

import (
	"errors"
	"math"

	"hissebot/internal/quant/fixedincome"
	"hissebot/internal/quant/options"
	"hissebot/internal/quant/rates"
)

type BondFutureDelivery struct {
	Name             string  `json:"name"`
	CashPrice        float64 `json:"cash_price"`
	ConversionFactor float64 `json:"conversion_factor"`
	AccruedInterest  float64 `json:"accrued_interest,omitempty"`
}

type CTDResult struct {
	Name      string  `json:"name"`
	NetBasis  float64 `json:"net_basis"`
	CashPrice float64 `json:"cash_price"`
}

func BondFixedCashflows(face, couponRate, maturity float64, frequency int) []fixedincome.Cashflow {
	return fixedincome.FixedBondCashflows(face, couponRate, maturity, frequency)
}

func BondFixedPrice(face, couponRate, maturity float64, frequency int, yield float64, comp rates.Compounding) float64 {
	return fixedincome.FixedBondPrice(face, couponRate, maturity, frequency, yield, comp)
}

func BondFixedYield(face, couponRate, maturity float64, frequency int, price float64, comp rates.Compounding) (float64, error) {
	return fixedincome.YieldToMaturity(fixedincome.FixedBondCashflows(face, couponRate, maturity, frequency), price, comp)
}

func BondFixedAnalytics(face, couponRate, maturity float64, frequency int, price float64, comp rates.Compounding) (fixedincome.BondAnalytics, error) {
	return fixedincome.FixedBondAnalytics(face, couponRate, maturity, frequency, price, comp)
}

func ZeroCouponPrice(face, rate, maturity float64, comp rates.Compounding) float64 {
	return fixedincome.ZeroCouponPrice(face, rate, maturity, comp)
}

func InflationLinkedBondPrice(realCashflows []fixedincome.Cashflow, realYield, indexRatio float64, comp rates.Compounding) float64 {
	return indexRatio * fixedincome.PresentValue(realCashflows, realYield, comp)
}

func IRSParRate(curve rates.DiscountCurve, maturity float64, fixedFrequency int) float64 {
	return rates.ParSwapRate(curve, maturity, fixedFrequency)
}

func IRSValue(notional, fixedRate float64, curve rates.DiscountCurve, maturity float64, fixedFrequency int, receiveFixed bool) float64 {
	if fixedFrequency <= 0 {
		fixedFrequency = 1
	}
	dt := 1 / float64(fixedFrequency)
	fixedPV := 0.0
	for t := dt; t <= maturity+1e-9; t += dt {
		fixedPV += notional * fixedRate * dt * curve.Discount(t)
	}
	floatPV := notional * (curve.Discount(0) - curve.Discount(maturity))
	if receiveFixed {
		return fixedPV - floatPV
	}
	return floatPV - fixedPV
}

func IRSDV01(notional float64, curve rates.DiscountCurve, maturity float64, fixedFrequency int) float64 {
	rate := IRSParRate(curve, maturity, fixedFrequency)
	return math.Abs(IRSValue(notional, rate+0.0001, curve, maturity, fixedFrequency, true) - IRSValue(notional, rate, curve, maturity, fixedFrequency, true))
}

func FRAValue(notional, forwardRate, contractRate, accrual, discountFactor float64, receiveFixed bool) float64 {
	value := notional * (forwardRate - contractRate) * accrual * discountFactor / (1 + forwardRate*accrual)
	if receiveFixed {
		return -value
	}
	return value
}

func FRABreakEven(forwardRate float64) float64 {
	return forwardRate
}

func DepositValue(notional, depositRate, marketRate, maturity float64) float64 {
	payoff := notional * (1 + depositRate*maturity)
	return payoff * rates.DiscountFactorFromRate(marketRate, maturity, rates.Simple)
}

func TBillValue(face, discountYield, maturity float64) float64 {
	return face * (1 - discountYield*maturity)
}

func RepoValue(cashAmount, repoRate, maturity, collateralIncome float64) float64 {
	return cashAmount*(1+repoRate*maturity) - collateralIncome
}

func OISParRate(curve rates.DiscountCurve, maturity float64, paymentFrequency int) float64 {
	return rates.ParSwapRate(curve, maturity, paymentFrequency)
}

func OISValue(notional, fixedRate float64, curve rates.DiscountCurve, maturity float64, paymentFrequency int, receiveFixed bool) float64 {
	return IRSValue(notional, fixedRate, curve, maturity, paymentFrequency, receiveFixed)
}

func OISBuildCurve(instruments []rates.MarketInstrument) (rates.DiscountCurve, error) {
	return rates.BootstrapDiscountCurve(instruments)
}

func VarianceSwapValue(notional, realizedVariance, varianceStrike, discountFactor float64) float64 {
	return notional * discountFactor * (realizedVariance - varianceStrike)
}

func VolatilitySwapValue(notional, realizedVolatility, volatilityStrike, discountFactor float64) float64 {
	return notional * discountFactor * (realizedVolatility - volatilityStrike)
}

func CommodityFuture(spot, riskFreeRate, storageCostRate, convenienceYield, maturity float64) float64 {
	return spot * math.Exp((riskFreeRate+storageCostRate-convenienceYield)*maturity)
}

func FXForward(spot, domesticRate, foreignRate, maturity float64) float64 {
	return spot * math.Exp((domesticRate-foreignRate)*maturity)
}

func FXGarmanKohlhagen(spot, strike, domesticRate, foreignRate, volatility, maturity float64, optionType options.OptionType) float64 {
	return options.BlackScholesPrice(spot, strike, domesticRate, foreignRate, volatility, maturity, optionType)
}

func CDSHazardRate(spread, recoveryRate float64) float64 {
	lgd := 1 - recoveryRate
	if lgd <= 0 {
		return 0
	}
	return spread / lgd
}

func SurvivalProbability(hazardRate, maturity float64) float64 {
	if hazardRate < 0 || maturity < 0 {
		return 0
	}
	return math.Exp(-hazardRate * maturity)
}

func CDSValue(notional, spread, marketSpread, recoveryRate, maturity, discountRate float64) float64 {
	hazard := CDSHazardRate(marketSpread, recoveryRate)
	annuity := 0.0
	dt := 0.25
	for t := dt; t <= maturity+1e-9; t += dt {
		annuity += dt * math.Exp(-(discountRate+hazard)*t)
	}
	return notional * (marketSpread - spread) * annuity
}

func STIRFuturePrice(rate float64) float64 {
	return 100 * (1 - rate)
}

func STIRFutureRate(price float64) float64 {
	return 1 - price/100
}

func BondFutureCTD(futuresPrice float64, deliveries []BondFutureDelivery) (CTDResult, error) {
	if len(deliveries) == 0 {
		return CTDResult{}, errors.New("no delivery candidates")
	}
	best := CTDResult{NetBasis: math.Inf(1)}
	for _, d := range deliveries {
		if d.ConversionFactor <= 0 {
			continue
		}
		cash := d.CashPrice + d.AccruedInterest
		netBasis := cash - futuresPrice*d.ConversionFactor
		if netBasis < best.NetBasis {
			best = CTDResult{Name: d.Name, NetBasis: netBasis, CashPrice: cash}
		}
	}
	if math.IsInf(best.NetBasis, 1) {
		return CTDResult{}, errors.New("no valid delivery candidates")
	}
	return best, nil
}

func ImpliedRepoRate(cashPrice, futuresPrice, conversionFactor, income, maturity float64) float64 {
	return fixedincome.ImpliedRepoRate(cashPrice, futuresPrice*conversionFactor, income, maturity)
}

func CostOfCarry(spot, forward, maturity float64) float64 {
	return fixedincome.CostOfCarryRate(spot, forward, maturity)
}
