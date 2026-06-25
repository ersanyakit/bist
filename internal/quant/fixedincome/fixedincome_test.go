package fixedincome

import (
	"math"
	"testing"
	"time"

	"hissebot/internal/quant/rates"
)

func TestBondAnalyticsIRRAndSpreads(t *testing.T) {
	cfs := FixedBondCashflows(100, 0.05, 5, 1)
	price := PresentValue(cfs, 0.05, rates.Annual)
	if math.Abs(price-100) > 1e-9 {
		t.Fatalf("par price=%.12f", price)
	}
	ytm, err := YieldToMaturity(cfs, price, rates.Annual)
	if err != nil {
		t.Fatalf("ytm: %v", err)
	}
	if math.Abs(ytm-0.05) > 1e-9 {
		t.Fatalf("ytm=%.12f", ytm)
	}
	if dur := MacaulayDuration(cfs, ytm, rates.Annual); dur <= 4 || dur >= 5 {
		t.Fatalf("unexpected duration %.6f", dur)
	}
	irr, err := InternalRateOfReturn([]Cashflow{{Time: 0, Amount: -100}, {Time: 1, Amount: 110}})
	if err != nil || math.Abs(irr-0.10) > 1e-9 {
		t.Fatalf("irr=%.12f err=%v", irr, err)
	}
	xirr, err := XIRR([]DatedCashflow{
		{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: -100},
		{Date: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), Amount: 110},
	})
	if err != nil || math.Abs(xirr-0.10) > 1e-9 {
		t.Fatalf("xirr=%.12f err=%v", xirr, err)
	}
	curve, _ := rates.NewDiscountCurve([]float64{0, 1, 5}, []float64{1, math.Exp(-0.03), math.Exp(-0.15)})
	market := PresentValue(cfs, 0.04, rates.Continuous)
	z, err := ZSpread(cfs, curve, market)
	if err != nil || math.Abs(z-0.01) > 1e-6 {
		t.Fatalf("zspread=%.12f err=%v", z, err)
	}
}
