package solver

import (
	"math"
	"testing"
)

func TestRootSolvers(t *testing.T) {
	root, err := Bisection(func(x float64) float64 { return x*x - 2 }, 0, 2, Options{})
	if err != nil || math.Abs(root-math.Sqrt2) > 1e-10 {
		t.Fatalf("bisection root=%.12f err=%v", root, err)
	}
	newton, err := Newton(
		func(x float64) float64 { return x*x - 2 },
		func(x float64) float64 { return 2 * x },
		1,
		Options{},
	)
	if err != nil || math.Abs(newton-math.Sqrt2) > 1e-10 {
		t.Fatalf("newton root=%.12f err=%v", newton, err)
	}
}
