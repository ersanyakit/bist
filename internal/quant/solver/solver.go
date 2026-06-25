package solver

import (
	"errors"
	"fmt"
	"math"
)

type Options struct {
	Tolerance     float64
	MaxIterations int
}

func withDefaults(opts Options) Options {
	if opts.Tolerance <= 0 {
		opts.Tolerance = 1e-10
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 100
	}
	return opts
}

func Bisection(fn func(float64) float64, lo, hi float64, opts Options) (float64, error) {
	opts = withDefaults(opts)
	if !finite(lo) || !finite(hi) || lo >= hi {
		return 0, errors.New("invalid bracket")
	}
	flo := fn(lo)
	fhi := fn(hi)
	if !finite(flo) || !finite(fhi) {
		return 0, errors.New("function returned non-finite bracket value")
	}
	if math.Abs(flo) <= opts.Tolerance {
		return lo, nil
	}
	if math.Abs(fhi) <= opts.Tolerance {
		return hi, nil
	}
	if flo*fhi > 0 {
		return 0, fmt.Errorf("root is not bracketed: f(lo)=%.12g f(hi)=%.12g", flo, fhi)
	}
	for i := 0; i < opts.MaxIterations; i++ {
		mid := 0.5 * (lo + hi)
		fmid := fn(mid)
		if !finite(fmid) {
			return 0, errors.New("function returned non-finite value")
		}
		if math.Abs(fmid) <= opts.Tolerance || math.Abs(hi-lo) <= opts.Tolerance {
			return mid, nil
		}
		if flo*fmid <= 0 {
			hi = mid
			fhi = fmid
		} else {
			lo = mid
			flo = fmid
		}
		_ = fhi
	}
	return 0.5 * (lo + hi), nil
}

func Newton(fn, derivative func(float64) float64, guess float64, opts Options) (float64, error) {
	opts = withDefaults(opts)
	x := guess
	if !finite(x) {
		return 0, errors.New("invalid initial guess")
	}
	for i := 0; i < opts.MaxIterations; i++ {
		fx := fn(x)
		if math.Abs(fx) <= opts.Tolerance {
			return x, nil
		}
		dfx := derivative(x)
		if !finite(fx) || !finite(dfx) || math.Abs(dfx) <= 1e-14 {
			return 0, errors.New("newton derivative failed")
		}
		next := x - fx/dfx
		if !finite(next) {
			return 0, errors.New("newton produced non-finite iterate")
		}
		if math.Abs(next-x) <= opts.Tolerance {
			return next, nil
		}
		x = next
	}
	return x, nil
}

func Secant(fn func(float64) float64, x0, x1 float64, opts Options) (float64, error) {
	opts = withDefaults(opts)
	f0 := fn(x0)
	f1 := fn(x1)
	for i := 0; i < opts.MaxIterations; i++ {
		denom := f1 - f0
		if math.Abs(denom) <= 1e-14 {
			return 0, errors.New("secant denominator collapsed")
		}
		x2 := x1 - f1*(x1-x0)/denom
		if !finite(x2) {
			return 0, errors.New("secant produced non-finite iterate")
		}
		f2 := fn(x2)
		if math.Abs(f2) <= opts.Tolerance || math.Abs(x2-x1) <= opts.Tolerance {
			return x2, nil
		}
		x0, f0 = x1, f1
		x1, f1 = x2, f2
	}
	return x1, nil
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
