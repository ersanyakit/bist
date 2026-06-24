// pkg/mathutil/mathutil.go
package mathutil

import "math"

const Epsilon = 1e-9

func AlmostEqual(a, b float64) bool {
	return math.Abs(a-b) <= Epsilon*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func AlmostEqualTol(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= math.Max(tolerance, Epsilon)
}

func GreaterThan(a, b float64) bool {
	return a-b > Epsilon
}

func GreaterOrEqual(a, b float64) bool {
	return GreaterThan(a, b) || AlmostEqual(a, b)
}

func LessThan(a, b float64) bool {
	return b-a > Epsilon
}

func LessOrEqual(a, b float64) bool {
	return LessThan(a, b) || AlmostEqual(a, b)
}

func SafeDiv(numerator, denominator float64) float64 {
	if AlmostEqual(denominator, 0) {
		return 0
	}
	return numerator / denominator
}

func Clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func StdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := Mean(values)
	sum := 0.0
	for _, value := range values {
		diff := value - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)))
}

func Max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func Min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
	}
	return minValue
}
