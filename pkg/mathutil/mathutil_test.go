package mathutil

import (
	"math"
	"testing"
)

func closeTo(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

func TestSafeDiv(t *testing.T) {
	if got := SafeDiv(10, 2); got != 5 {
		t.Fatalf("SafeDiv(10, 2) = %v, want 5", got)
	}
	if got := SafeDiv(10, 0); got != 0 {
		t.Fatalf("SafeDiv(10, 0) = %v, want 0", got)
	}
}

func TestClamp(t *testing.T) {
	cases := []struct {
		name     string
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{name: "inside", value: 5, min: 1, max: 10, expected: 5},
		{name: "low", value: -2, min: 1, max: 10, expected: 1},
		{name: "high", value: 15, min: 1, max: 10, expected: 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Clamp(tc.value, tc.min, tc.max); got != tc.expected {
				t.Fatalf("Clamp() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestMeanStdDevMinMax(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}

	if got := Mean(values); !closeTo(got, 5) {
		t.Fatalf("Mean() = %v, want 5", got)
	}
	if got := StdDev(values); !closeTo(got, 2) {
		t.Fatalf("StdDev() = %v, want 2", got)
	}
	if got := Min(values); got != 2 {
		t.Fatalf("Min() = %v, want 2", got)
	}
	if got := Max(values); got != 9 {
		t.Fatalf("Max() = %v, want 9", got)
	}
}

func TestMathutilEmptyInputsAreNeutral(t *testing.T) {
	var values []float64

	if got := Mean(values); got != 0 {
		t.Fatalf("Mean(nil) = %v, want 0", got)
	}
	if got := StdDev(values); got != 0 {
		t.Fatalf("StdDev(nil) = %v, want 0", got)
	}
	if got := Min(values); got != 0 {
		t.Fatalf("Min(nil) = %v, want 0", got)
	}
	if got := Max(values); got != 0 {
		t.Fatalf("Max(nil) = %v, want 0", got)
	}
}
