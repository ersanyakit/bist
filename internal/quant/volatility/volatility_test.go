package volatility

import (
	"math"
	"testing"
)

func TestVolatilitySurfaceAndSABR(t *testing.T) {
	surface, err := BuildSurfaceFromPoints([]SurfacePoint{
		{Maturity: 1, Strike: 90, Vol: 0.24},
		{Maturity: 1, Strike: 110, Vol: 0.22},
		{Maturity: 2, Strike: 90, Vol: 0.26},
		{Maturity: 2, Strike: 110, Vol: 0.23},
	})
	if err != nil {
		t.Fatalf("surface: %v", err)
	}
	if v := surface.Volatility(1.5, 100); math.Abs(v-0.2375) > 1e-10 {
		t.Fatalf("interp vol=%.12f", v)
	}
	params := SABRParams{Alpha: 0.30, Beta: 0.50, Rho: -0.20, Nu: 0.60}
	vol := SABRImpliedVolatility(100, 100, 1, params)
	if vol <= 0 {
		t.Fatalf("sabr vol=%.8f", vol)
	}
	cal := CalibrateSABR(100, 1, []float64{90, 100, 110}, []float64{
		SABRImpliedVolatility(100, 90, 1, params),
		SABRImpliedVolatility(100, 100, 1, params),
		SABRImpliedVolatility(100, 110, 1, params),
	}, 0.50)
	if cal.RMSE > 0.02 {
		t.Fatalf("calibration rmse=%.8f params=%+v", cal.RMSE, cal.Params)
	}
	if lv := ImpliedToLocalVolatility(surface, 100, 1.5, 100); lv <= 0 {
		t.Fatalf("local vol=%.8f", lv)
	}
}
