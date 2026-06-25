package instruments

import (
	"math"
	"testing"

	"hissebot/internal/quant/rates"
)

func TestInstrumentWrappers(t *testing.T) {
	curve, err := rates.NewDiscountCurve([]float64{0, 1, 2}, []float64{1, math.Exp(-0.05), math.Exp(-0.10)})
	if err != nil {
		t.Fatalf("curve: %v", err)
	}
	par := IRSParRate(curve, 2, 1)
	if math.Abs(IRSValue(1_000_000, par, curve, 2, 1, true)) > 1e-5 {
		t.Fatalf("par swap value not near zero")
	}
	if fx := FXForward(10, 0.05, 0.02, 1); math.Abs(fx-10*math.Exp(0.03)) > 1e-12 {
		t.Fatalf("fx forward=%.8f", fx)
	}
	if hz := CDSHazardRate(0.01, 0.40); math.Abs(hz-0.0166666667) > 1e-8 {
		t.Fatalf("hazard=%.10f", hz)
	}
	ctd, err := BondFutureCTD(100, []BondFutureDelivery{
		{Name: "A", CashPrice: 104, ConversionFactor: 1.02},
		{Name: "B", CashPrice: 101, ConversionFactor: 1.01},
	})
	if err != nil || ctd.Name != "B" {
		t.Fatalf("ctd=%+v err=%v", ctd, err)
	}
}
