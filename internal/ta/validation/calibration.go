package validation

import "math"

type ReliabilityBucket struct {
	MinProbability float64 `json:"min_probability"`
	MaxProbability float64 `json:"max_probability"`
	Samples        int     `json:"samples"`
	AverageProb    float64 `json:"average_probability"`
	ObservedRate   float64 `json:"observed_rate"`
}

func ClipProbability(p float64) float64 {
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return 0.5
	}
	if p < 0.01 {
		return 0.01
	}
	if p > 0.99 {
		return 0.99
	}
	return p
}

func BrierScore(probs []float64, outcomes []bool) float64 {
	n := minLen(len(probs), len(outcomes))
	if n == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		y := 0.0
		if outcomes[i] {
			y = 1
		}
		d := ClipProbability(probs[i]) - y
		sum += d * d
	}
	return sum / float64(n)
}

func ReliabilityBuckets(probs []float64, outcomes []bool, buckets int) []ReliabilityBucket {
	if buckets <= 0 {
		buckets = 10
	}
	out := make([]ReliabilityBucket, buckets)
	for i := range out {
		out[i].MinProbability = float64(i) / float64(buckets)
		out[i].MaxProbability = float64(i+1) / float64(buckets)
	}
	n := minLen(len(probs), len(outcomes))
	for i := 0; i < n; i++ {
		p := ClipProbability(probs[i])
		idx := int(math.Min(float64(buckets-1), math.Floor(p*float64(buckets))))
		out[idx].Samples++
		out[idx].AverageProb += p
		if outcomes[i] {
			out[idx].ObservedRate++
		}
	}
	for i := range out {
		if out[i].Samples > 0 {
			out[i].AverageProb /= float64(out[i].Samples)
			out[i].ObservedRate /= float64(out[i].Samples)
		}
	}
	return out
}
