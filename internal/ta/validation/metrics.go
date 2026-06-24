package validation

import "math"

type Metrics struct {
	MAE               float64            `json:"mae"`
	RMSE              float64            `json:"rmse"`
	MAPE              float64            `json:"mape"`
	DirectionAccuracy float64            `json:"direction_accuracy"`
	BalancedAccuracy  float64            `json:"balanced_accuracy"`
	F1                float64            `json:"f1"`
	BrierScore        float64            `json:"brier_score"`
	CalibrationError  float64            `json:"calibration_error"`
	PinballLoss       map[string]float64 `json:"pinball_loss,omitempty"`
	Coverage          float64            `json:"coverage"`
	IntervalWidth     float64            `json:"interval_width"`
	Sharpe            float64            `json:"sharpe"`
	Sortino           float64            `json:"sortino"`
	MaxDrawdown       float64            `json:"max_drawdown"`
	Turnover          float64            `json:"turnover"`
	NetPnL            float64            `json:"net_pnl"`
	GrossPnL          float64            `json:"gross_pnl"`
	HitRatio          float64            `json:"hit_ratio"`
	AvgHoldingPeriod  float64            `json:"avg_holding_period"`
	Samples           int                `json:"samples,omitempty"`
}

func RegressionMetrics(actual, predicted []float64) Metrics {
	n := minLen(len(actual), len(predicted))
	m := Metrics{PinballLoss: map[string]float64{}, Samples: n}
	if n == 0 {
		return m
	}
	absSum := 0.0
	sqSum := 0.0
	apeSum := 0.0
	apeN := 0
	hit := 0
	for i := 0; i < n; i++ {
		err := predicted[i] - actual[i]
		absSum += math.Abs(err)
		sqSum += err * err
		if actual[i] != 0 {
			apeSum += math.Abs(err / actual[i])
			apeN++
		}
		if direction(actual[i]) == direction(predicted[i]) {
			hit++
		}
	}
	m.MAE = absSum / float64(n)
	m.RMSE = math.Sqrt(sqSum / float64(n))
	if apeN > 0 {
		m.MAPE = apeSum / float64(apeN)
	}
	m.DirectionAccuracy = float64(hit) / float64(n)
	m.HitRatio = m.DirectionAccuracy
	return m
}

func DirectionMetrics(actual, predicted []string) Metrics {
	n := minLen(len(actual), len(predicted))
	m := Metrics{Samples: n}
	if n == 0 {
		return m
	}
	labels := []string{"up", "down", "flat"}
	correct := 0
	recallSum := 0.0
	f1Sum := 0.0
	used := 0
	for _, label := range labels {
		tp, fp, fn := 0, 0, 0
		for i := 0; i < n; i++ {
			if actual[i] == predicted[i] {
				correct++
			}
			if predicted[i] == label && actual[i] == label {
				tp++
			} else if predicted[i] == label && actual[i] != label {
				fp++
			} else if predicted[i] != label && actual[i] == label {
				fn++
			}
		}
		if tp+fn > 0 {
			recallSum += float64(tp) / float64(tp+fn)
			precision := 0.0
			if tp+fp > 0 {
				precision = float64(tp) / float64(tp+fp)
			}
			recall := float64(tp) / float64(tp+fn)
			if precision+recall > 0 {
				f1Sum += 2 * precision * recall / (precision + recall)
			}
			used++
		}
	}
	m.DirectionAccuracy = float64(correct) / float64(n*len(labels))
	if used > 0 {
		m.BalancedAccuracy = recallSum / float64(used)
		m.F1 = f1Sum / float64(used)
	}
	return m
}

func EconomicMetrics(returns []float64) Metrics {
	m := Metrics{Samples: len(returns)}
	if len(returns) == 0 {
		return m
	}
	sum := 0.0
	downside := []float64{}
	hit := 0
	equity := 0.0
	peak := 0.0
	maxDD := 0.0
	for _, r := range returns {
		sum += r
		if r > 0 {
			hit++
		}
		if r < 0 {
			downside = append(downside, r)
		}
		equity += r
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}
	mean := sum / float64(len(returns))
	std := stddev(returns)
	if std > 0 {
		m.Sharpe = mean / std * math.Sqrt(252)
	}
	downStd := stddev(downside)
	if downStd > 0 {
		m.Sortino = mean / downStd * math.Sqrt(252)
	}
	m.NetPnL = sum
	m.GrossPnL = sum
	m.MaxDrawdown = maxDD
	m.HitRatio = float64(hit) / float64(len(returns))
	return m
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(values)-1))
}

func direction(v float64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}
