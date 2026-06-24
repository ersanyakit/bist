package fintradebench

import (
	"fmt"
	"math"

	"hissebot/internal/ta/ohlcv"
)

type CalculationValidation struct {
	Status   string             `json:"status"`
	Score    float64            `json:"score"`
	Checked  int                `json:"checked"`
	Passed   int                `json:"passed"`
	Failed   int                `json:"failed"`
	Skipped  int                `json:"skipped"`
	Checks   []CalculationCheck `json:"checks,omitempty"`
	Warnings []string           `json:"warnings,omitempty"`
}

type CalculationCheck struct {
	Name      string  `json:"name"`
	Family    string  `json:"family"`
	Formula   string  `json:"formula,omitempty"`
	Source    string  `json:"source,omitempty"`
	Expected  float64 `json:"expected,omitempty"`
	Actual    float64 `json:"actual,omitempty"`
	Delta     float64 `json:"delta,omitempty"`
	Tolerance float64 `json:"tolerance,omitempty"`
	Status    string  `json:"status"`
	Message   string  `json:"message,omitempty"`
}

type validationValue struct {
	value float64
	ok    bool
}

func buildCalculationValidation(input Input, candles []ohlcv.Candle, lastClose float64, trading []Signal, fundamentals []Signal, supplemental []Signal) CalculationValidation {
	signals := signalMap(append(append(append([]Signal{}, trading...), fundamentals...), supplemental...))
	closes := effectiveCloses(candles)
	volumes := effectiveVolumes(candles)
	out := CalculationValidation{Status: "passed"}

	add := func(name, family string, result validationValue) {
		out.Checks = append(out.Checks, calculationCheck(name, family, result.value, result.ok, signals))
	}

	add("ma_20", "trading", validationResult(validationSMA(closes, 20)))
	add("ema_20", "trading", validationResult(validationEMA(closes, 20)))
	add("macd", "trading", validationResult(validationMACD(closes, 12, 26, 9)))
	add("rsi_14", "trading", validationResult(validationRSI(closes, 14)))
	add("obv", "trading", validationResult(validationOBV(candles)))
	add("one_day_reversal", "trading", validationResult(validationOneDayReversal(candles)))
	add("max_return_20d", "trading", validationResult(validationMaxDailyReturn(candles, 20)))
	add("medium_term_momentum_60d", "trading", validationResult(validationCumulativeReturn(candles, 60)))
	add("long_term_mean_reversion", "trading", validationResult(validationLongTermMeanReversion(closes, lastClose, 200)))
	add("realized_volatility_20d", "trading", validationResult(validationRealizedVolatility(candles, 20)))
	add("max_drawdown_20d", "trading", validationResult(validationMaxDrawdown(candles, 20)))
	add("volume_sma20_ratio", "trading", validationResult(validationVolumeSMA20Ratio(volumes)))

	v := input.Professional.Valuation
	add("cash_flow_assets", "fundamental", validationResult(safeValidationDiv(v.OperatingCashTTM, v.TotalAssets, v.OperatingCashTTM != 0 && v.TotalAssets > 0)))
	add("book_price", "fundamental", validationResult(safeValidationDiv(v.Equity, v.MarketCap, v.Equity > 0 && v.MarketCap > 0)))
	add("earnings_price", "fundamental", validationResult(safeValidationDiv(v.NetIncomeTTM, v.MarketCap, v.NetIncomeTTM != 0 && v.MarketCap > 0)))
	add("sales_assets", "fundamental", validationResult(safeValidationDiv(v.SalesTTM, v.TotalAssets, v.SalesTTM != 0 && v.TotalAssets > 0)))
	add("debt_assets", "fundamental", validationResult(safeValidationDiv(v.TotalDebt, v.TotalAssets, v.DebtDataAvailable && v.TotalAssets > 0)))
	add("debt_equity", "fundamental", validationResult(safeValidationDiv(v.TotalDebt, v.Equity, v.DebtDataAvailable && v.Equity > 0)))
	add("dividend_yield", "fundamental", validationResult(validationDividendYield(input, lastClose)))
	add("return_assets", "fundamental", validationResult(safeValidationDiv(v.NetIncomeTTM, v.TotalAssets, v.NetIncomeTTM != 0 && v.TotalAssets > 0)))
	add("return_equity", "fundamental", validationResult(safeValidationDiv(v.NetIncomeTTM, v.Equity, v.NetIncomeTTM != 0 && v.Equity > 0)))

	add("free_cash_flow_yield", "supplemental", validationResult(safeValidationDiv(v.FreeCashFlowTTM, v.MarketCap, v.FreeCashFlowTTM != 0 && v.MarketCap > 0)))
	add("book_per_share", "supplemental", validationResult(safeValidationDiv(v.Equity, v.PaidCapital, v.Equity > 0 && v.PaidCapital > 0)))
	add("net_debt_equity", "supplemental", validationResult(safeValidationDiv(v.NetDebt, v.Equity, v.NetDebt != 0 && v.Equity > 0)))

	for _, check := range out.Checks {
		switch check.Status {
		case "passed":
			out.Passed++
			out.Checked++
		case "failed":
			out.Failed++
			out.Checked++
		case "skipped":
			out.Skipped++
		}
	}
	switch {
	case out.Failed > 0:
		out.Status = "failed"
		out.Warnings = append(out.Warnings, fmt.Sprintf("calculation_validation_failed:%d", out.Failed))
	case out.Checked == 0:
		out.Status = "limited"
		out.Warnings = append(out.Warnings, "calculation_validation_no_checks")
	default:
		out.Status = "passed"
	}
	if out.Checked > 0 {
		out.Score = round2(100 * float64(out.Passed) / float64(out.Checked))
	}
	return out
}

func validationResult(value float64, ok bool) validationValue {
	return validationValue{value: value, ok: ok}
}

func calculationCheck(name, family string, expected float64, ok bool, signals map[string]Signal) CalculationCheck {
	signal, exists := signals[name]
	check := CalculationCheck{
		Name:    name,
		Family:  family,
		Formula: signal.Formula,
		Source:  signal.Source,
		Status:  "skipped",
	}
	if !ok || !isFinite(expected) {
		check.Message = "source inputs unavailable for independent recalculation"
		if exists && signal.Available {
			check.Actual = signal.Value
		}
		return check
	}
	check.Expected = round6(expected)
	if !exists || !signal.Available {
		check.Status = "failed"
		check.Message = "source inputs support calculation but reported signal is unavailable"
		return check
	}
	check.Actual = signal.Value
	check.Delta = math.Abs(check.Actual - check.Expected)
	check.Tolerance = calculationTolerance(check.Expected)
	if check.Delta <= check.Tolerance {
		check.Status = "passed"
		return check
	}
	check.Status = "failed"
	check.Message = "reported signal differs from independent formula recalculation"
	return check
}

func signalMap(signals []Signal) map[string]Signal {
	out := map[string]Signal{}
	for _, signal := range signals {
		out[signal.Name] = signal
	}
	return out
}

func calculationTolerance(expected float64) float64 {
	return math.Max(1e-5, math.Abs(expected)*1e-6)
}

func effectiveCloses(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, candle := range candles {
		out[i] = candle.EffectiveClose()
	}
	return out
}

func effectiveVolumes(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, candle := range candles {
		out[i] = candle.EffectiveVolume()
	}
	return out
}

func validationSMA(values []float64, period int) (float64, bool) {
	if period <= 0 || len(values) < period {
		return 0, false
	}
	sum := 0.0
	for _, value := range values[len(values)-period:] {
		sum += value
	}
	return sum / float64(period), true
}

func validationEMA(values []float64, period int) (float64, bool) {
	series := validationEMASeries(values, period)
	if len(series) == 0 {
		return 0, false
	}
	return series[len(series)-1], true
}

func validationEMASeries(values []float64, period int) []float64 {
	if len(values) < period || period <= 0 {
		return nil
	}
	out := make([]float64, len(values))
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
		out[i] = sum / float64(i+1)
	}
	out[period-1] = sum / float64(period)
	alpha := 2.0 / float64(period+1)
	for i := period; i < len(values); i++ {
		out[i] = values[i]*alpha + out[i-1]*(1-alpha)
	}
	return out
}

func validationMACD(values []float64, fast int, slow int, signal int) (float64, bool) {
	if len(values) < slow+signal-1 {
		return 0, false
	}
	fastEMA := validationEMASeries(values, fast)
	slowEMA := validationEMASeries(values, slow)
	if len(fastEMA) == 0 || len(slowEMA) == 0 {
		return 0, false
	}
	line := make([]float64, len(values))
	for i := slow - 1; i < len(values); i++ {
		line[i] = fastEMA[i] - slowEMA[i]
	}
	signalSeries := validationEMASeries(line[slow-1:], signal)
	if len(signalSeries) == 0 {
		return 0, false
	}
	return line[len(values)-1], true
}

func validationRSI(values []float64, period int) (float64, bool) {
	if len(values) < 2 || period <= 0 {
		return 0, false
	}
	if len(values) <= period {
		return 50, true
	}
	sumGain := 0.0
	sumLoss := 0.0
	for i := 1; i <= period; i++ {
		change := values[i] - values[i-1]
		if change > 0 {
			sumGain += change
		} else {
			sumLoss -= change
		}
	}
	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)
	rsi := rsiFromAverages(avgGain, avgLoss)
	for i := period + 1; i < len(values); i++ {
		change := values[i] - values[i-1]
		currentGain := 0.0
		currentLoss := 0.0
		if change > 0 {
			currentGain = change
		} else {
			currentLoss = -change
		}
		avgGain = (avgGain*float64(period-1) + currentGain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + currentLoss) / float64(period)
		rsi = rsiFromAverages(avgGain, avgLoss)
	}
	return rsi, true
}

func rsiFromAverages(avgGain float64, avgLoss float64) float64 {
	switch {
	case avgLoss == 0 && avgGain == 0:
		return 50
	case avgLoss == 0:
		return 100
	case avgGain == 0:
		return 0
	default:
		return 100 - 100/(1+avgGain/avgLoss)
	}
}

func validationOBV(candles []ohlcv.Candle) (float64, bool) {
	if len(candles) == 0 {
		return 0, false
	}
	obv := 0.0
	for i := 1; i < len(candles); i++ {
		switch {
		case candles[i].EffectiveClose() > candles[i-1].EffectiveClose():
			obv += candles[i].EffectiveVolume()
		case candles[i].EffectiveClose() < candles[i-1].EffectiveClose():
			obv -= candles[i].EffectiveVolume()
		}
	}
	return obv, true
}

func validationOneDayReversal(candles []ohlcv.Candle) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}
	return safeValidationDiv(candles[len(candles)-1].EffectiveClose(), candles[len(candles)-2].EffectiveClose(), candles[len(candles)-2].EffectiveClose() > 0, -1)
}

func validationMaxDailyReturn(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}
	start := maxInt(1, len(candles)-period)
	best := math.Inf(-1)
	for i := start; i < len(candles); i++ {
		value, ok := safeValidationDiv(candles[i].EffectiveClose(), candles[i-1].EffectiveClose(), candles[i-1].EffectiveClose() > 0, -1)
		if ok && value > best {
			best = value
		}
	}
	if math.IsInf(best, -1) {
		return 0, false
	}
	return best, true
}

func validationCumulativeReturn(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) <= period {
		return 0, false
	}
	return safeValidationDiv(candles[len(candles)-1].EffectiveClose(), candles[len(candles)-1-period].EffectiveClose(), candles[len(candles)-1-period].EffectiveClose() > 0, -1)
}

func validationLongTermMeanReversion(values []float64, lastClose float64, period int) (float64, bool) {
	mean, ok := validationSMA(values, period)
	if !ok || lastClose <= 0 {
		return 0, false
	}
	return (mean - lastClose) / lastClose, true
}

func validationRealizedVolatility(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) <= period {
		return 0, false
	}
	values := make([]float64, 0, period)
	for i := len(candles) - period; i < len(candles); i++ {
		prev := candles[i-1].EffectiveClose()
		curr := candles[i].EffectiveClose()
		if prev > 0 && curr > 0 {
			values = append(values, math.Log(curr/prev))
		}
	}
	if len(values) < 2 {
		return 0, false
	}
	return validationStddev(values) * math.Sqrt(252), true
}

func validationMaxDrawdown(candles []ohlcv.Candle, period int) (float64, bool) {
	if len(candles) < 2 {
		return 0, false
	}
	start := maxInt(0, len(candles)-period)
	peak := 0.0
	maxDD := 0.0
	for _, candle := range candles[start:] {
		close := candle.EffectiveClose()
		if close <= 0 {
			continue
		}
		if close > peak {
			peak = close
		}
		if peak > 0 {
			maxDD = math.Min(maxDD, close/peak-1)
		}
	}
	return maxDD, peak > 0
}

func validationVolumeSMA20Ratio(volumes []float64) (float64, bool) {
	volumeSMA, ok := validationSMA(volumes, 20)
	if !ok || volumeSMA <= 0 || len(volumes) == 0 {
		return 0, false
	}
	return volumes[len(volumes)-1] / volumeSMA, true
}

func validationDividendYield(input Input, lastClose float64) (float64, bool) {
	capital := input.Professional.ValueInvesting.CapitalAllocation
	years := input.Professional.ValueInvesting.Years
	if !capital.DividendDataAvailable || capital.PaidCapitalLatest <= 0 || lastClose <= 0 || len(years) == 0 {
		return 0, false
	}
	dividendPerShare := years[len(years)-1].DividendsPaid / capital.PaidCapitalLatest
	if dividendPerShare <= 0 {
		return 0, false
	}
	return dividendPerShare / lastClose, true
}

func safeValidationDiv(num float64, den float64, ok bool, offsets ...float64) (float64, bool) {
	if !ok || den == 0 || math.IsNaN(num) || math.IsNaN(den) || math.IsInf(num, 0) || math.IsInf(den, 0) {
		return 0, false
	}
	out := num / den
	for _, offset := range offsets {
		out += offset
	}
	return out, true
}

func validationStddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	return math.Sqrt(variance / float64(len(values)-1))
}
