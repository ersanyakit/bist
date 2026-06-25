package rates

import (
	"math"
	"testing"
)

func TestRatesCurveAndBootstrap(t *testing.T) {
	df := DiscountFactorFromRate(0.05, 2, Continuous)
	if math.Abs(df-math.Exp(-0.10)) > 1e-12 {
		t.Fatalf("df mismatch")
	}
	z := ZeroRateFromDiscountFactor(df, 2, Continuous)
	if math.Abs(z-0.05) > 1e-12 {
		t.Fatalf("zero rate mismatch: %.12f", z)
	}
	curve, err := BootstrapDiscountCurve([]MarketInstrument{
		{Type: DepositInstrument, Maturity: 1, Rate: 0.05},
		{Type: SwapInstrument, Maturity: 2, Rate: 0.055, Frequency: 1},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if curve.Discount(2) <= 0 || curve.Discount(2) >= curve.Discount(1) {
		t.Fatalf("invalid bootstrapped curve: %+v", curve)
	}
	if par := ParSwapRate(curve, 2, 1); math.Abs(par-0.055) > 1e-10 {
		t.Fatalf("par swap=%.12f", par)
	}
}

func TestCalibrateVasicek(t *testing.T) {
	series := []float64{0.0300, 0.0305, 0.0310, 0.0312, 0.0311, 0.0314, 0.0317, 0.0320}
	params, err := CalibrateVasicek(series, 1.0/252.0)
	if err != nil {
		t.Fatalf("CalibrateVasicek: %v", err)
	}
	if params.MeanReversion <= 0 || params.Volatility < 0 {
		t.Fatalf("invalid params: %+v", params)
	}
	if price := VasicekZeroCouponPrice(series[len(series)-1], 2, params); price <= 0 || price >= 1.5 {
		t.Fatalf("invalid vasicek zc price: %.6f", price)
	}
}
