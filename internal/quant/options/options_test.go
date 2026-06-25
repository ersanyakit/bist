package options

import (
	"math"
	"testing"
)

func TestBlackScholesBlack76BachelierAndBinomial(t *testing.T) {
	price := BlackScholesPrice(100, 100, 0.05, 0, 0.20, 1, Call)
	if math.Abs(price-10.45058357) > 1e-6 {
		t.Fatalf("bs price=%.8f", price)
	}
	greeks := BlackScholesGreeks(100, 100, 0.05, 0, 0.20, 1, Call)
	if math.Abs(greeks.Delta-0.63683065) > 1e-6 || math.Abs(greeks.Gamma-0.01876202) > 1e-6 {
		t.Fatalf("bs greeks=%+v", greeks)
	}
	iv, err := BlackScholesImpliedVolatility(price, 100, 100, 0.05, 0, 1, Call)
	if err != nil || math.Abs(iv-0.20) > 1e-7 {
		t.Fatalf("bs iv=%.10f err=%v", iv, err)
	}
	black76 := Black76Price(100, 100, 0.05, 0.20, 1, Call)
	if math.Abs(black76-7.57708215) > 1e-6 {
		t.Fatalf("black76 price=%.8f", black76)
	}
	bach := BachelierPrice(100, 100, 1, 10, 1, Call)
	if math.Abs(bach-3.98942280) > 1e-6 {
		t.Fatalf("bachelier price=%.8f", bach)
	}
	binomial := BinomialEuropean(100, 100, 0.05, 0, 0.20, 1, Call, 1000)
	if math.Abs(binomial-price) > 0.02 {
		t.Fatalf("binomial %.8f vs bs %.8f", binomial, price)
	}
	if d := DigitalCashOrNothing(100, 100, 0.05, 0, 0.2, 1, 1, Call); d <= 0 || d >= 1 {
		t.Fatalf("digital call invalid %.8f", d)
	}
	if g := KirkSpreadOptionGreeks(110, 100, 5, 1, 0.2, 0.25, 0.5, 1, Call); g.Delta <= 0 || g.Vega <= 0 {
		t.Fatalf("kirk greeks invalid %+v", g)
	}
}
