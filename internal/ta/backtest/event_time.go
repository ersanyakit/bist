package backtest

import (
	"math"
	"time"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

type Config struct {
	CommissionBps      float64
	SlippageBps        float64
	ExecutionDelayBars int
}

type Signal struct {
	BarIndex    int       `json:"bar_index"`
	AvailableAt time.Time `json:"available_at"`
	Direction   string    `json:"direction"`
	Reason      string    `json:"reason"`
}

type Trade struct {
	EntrySignalAt time.Time `json:"entry_signal_at"`
	EntryTime     time.Time `json:"entry_time"`
	ExitSignalAt  time.Time `json:"exit_signal_at"`
	ExitTime      time.Time `json:"exit_time"`
	EntryIndex    int       `json:"entry_index"`
	ExitIndex     int       `json:"exit_index"`
	EntryPrice    float64   `json:"entry_price"`
	ExitPrice     float64   `json:"exit_price"`
	Return        float64   `json:"return"`
	HoldingBars   int       `json:"holding_bars"`
}

type Result struct {
	Strategy            string   `json:"strategy"`
	ExecutionModel      string   `json:"execution_model"`
	LookbackBars        int      `json:"lookback_bars"`
	Trades              int      `json:"trades"`
	WinRate             float64  `json:"win_rate"`
	AverageReturn       float64  `json:"average_return"`
	MedianReturn        float64  `json:"median_return"`
	ProfitFactor        float64  `json:"profit_factor"`
	MaxDrawdown         float64  `json:"max_drawdown"`
	CAGR                float64  `json:"cagr"`
	Volatility          float64  `json:"volatility"`
	Sharpe              float64  `json:"sharpe"`
	Sortino             float64  `json:"sortino"`
	Exposure            float64  `json:"exposure"`
	InSampleTrades      int      `json:"in_sample_trades"`
	OutOfSampleTrades   int      `json:"out_of_sample_trades"`
	OutOfSampleReturn   float64  `json:"out_of_sample_average_return"`
	Expectancy          float64  `json:"expectancy"`
	AvgHoldingBars      float64  `json:"avg_holding_bars"`
	CurrentInMarket     bool     `json:"current_in_market"`
	CommissionBps       float64  `json:"commission_bps"`
	SlippageBps         float64  `json:"slippage_bps"`
	LookaheadViolations int      `json:"lookahead_violations"`
	Warnings            []string `json:"warnings,omitempty"`
	TradesList          []Trade  `json:"trades_list,omitempty"`
}

func NormalizeConfig(cfg Config) Config {
	if cfg.CommissionBps < 0 {
		cfg.CommissionBps = 0
	}
	if cfg.SlippageBps < 0 {
		cfg.SlippageBps = 0
	}
	if cfg.ExecutionDelayBars <= 0 {
		cfg.ExecutionDelayBars = 1
	}
	return cfg
}

func RunTrendMomentum(candles []ohlcv.Candle, cfg Config) Result {
	cfg = NormalizeConfig(cfg)
	result := Result{
		Strategy:       "SMA50 trend + EMA12/26 momentum + SMA20 exit",
		ExecutionModel: "signal_at_close_execute_next_open",
		LookbackBars:   len(candles),
		CommissionBps:  cfg.CommissionBps,
		SlippageBps:    cfg.SlippageBps,
	}
	if len(candles) < 80 {
		result.Warnings = append(result.Warnings, "insufficient_bars")
		return result
	}
	closes := closeSeries(candles)
	sma20 := smaSeries(closes, 20)
	sma50 := smaSeries(closes, 50)
	ema12 := emaSeries(closes, 12)
	ema26 := emaSeries(closes, 26)
	inTrade := false
	var pendingEntry *Signal
	var pendingExit *Signal
	entryPrice := 0.0
	entryIndex := 0
	entrySignalAt := time.Time{}
	tradeReturns := []float64{}
	holdBars := []float64{}
	equity := 1.0
	peak := 1.0
	maxDD := 0.0
	grossGain := 0.0
	grossLoss := 0.0
	for i := 51; i < len(candles); i++ {
		if pendingEntry != nil && !inTrade && i >= pendingEntry.BarIndex+cfg.ExecutionDelayBars {
			if pendingEntry.AvailableAt.After(candles[i].Time) {
				result.LookaheadViolations++
			}
			entryPrice = applyBuyCosts(candles[i].EffectiveOpen(), cfg)
			entryIndex = i
			entrySignalAt = pendingEntry.AvailableAt
			inTrade = entryPrice > 0
			pendingEntry = nil
		}
		if pendingExit != nil && inTrade && i >= pendingExit.BarIndex+cfg.ExecutionDelayBars {
			if pendingExit.AvailableAt.After(candles[i].Time) {
				result.LookaheadViolations++
			}
			exitPrice := applySellCosts(candles[i].EffectiveOpen(), cfg)
			ret := mathutil.SafeDiv(exitPrice-entryPrice, entryPrice)
			trade := Trade{
				EntrySignalAt: entrySignalAt,
				EntryTime:     candles[entryIndex].Time,
				ExitSignalAt:  pendingExit.AvailableAt,
				ExitTime:      candles[i].Time,
				EntryIndex:    entryIndex,
				ExitIndex:     i,
				EntryPrice:    entryPrice,
				ExitPrice:     exitPrice,
				Return:        ret,
				HoldingBars:   i - entryIndex,
			}
			result.TradesList = append(result.TradesList, trade)
			tradeReturns = append(tradeReturns, ret)
			holdBars = append(holdBars, float64(trade.HoldingBars))
			equity *= 1 + ret
			peak = math.Max(peak, equity)
			maxDD = math.Min(maxDD, mathutil.SafeDiv(equity-peak, peak))
			if ret >= 0 {
				grossGain += ret
			} else {
				grossLoss += math.Abs(ret)
			}
			inTrade = false
			pendingExit = nil
		}
		momentum := ema12[i] - ema26[i]
		enter := closes[i] > sma50[i] && sma20[i] > sma50[i] && momentum > 0
		exit := closes[i] < sma20[i] || momentum < 0
		if !inTrade && pendingEntry == nil && enter {
			signal := Signal{BarIndex: i, AvailableAt: candles[i].Time, Direction: "long", Reason: "trend_momentum_entry"}
			pendingEntry = &signal
			continue
		}
		if inTrade && pendingExit == nil && exit {
			signal := Signal{BarIndex: i, AvailableAt: candles[i].Time, Direction: "flat", Reason: "trend_momentum_exit"}
			pendingExit = &signal
		}
	}
	result.CurrentInMarket = inTrade || pendingEntry != nil
	wins := 0
	for _, ret := range tradeReturns {
		if ret > 0 {
			wins++
		}
	}
	result.Trades = len(tradeReturns)
	result.WinRate = mathutil.SafeDiv(float64(wins), float64(len(tradeReturns)))
	result.AverageReturn = mathutil.Mean(tradeReturns)
	result.MedianReturn = median(tradeReturns)
	result.ProfitFactor = mathutil.SafeDiv(grossGain, grossLoss)
	result.MaxDrawdown = maxDD
	result.Expectancy = result.AverageReturn
	result.AvgHoldingBars = mathutil.Mean(holdBars)
	result.Exposure = exposureFromTrades(result.TradesList, len(candles))
	result.Volatility = annualizedVolatility(tradeReturns, holdBars)
	result.Sharpe = mathutil.SafeDiv(result.AverageReturn, tradeReturnStdDev(tradeReturns)) * math.Sqrt(mathutil.SafeDiv(252, math.Max(result.AvgHoldingBars, 1)))
	result.Sortino = sortino(tradeReturns, result.AvgHoldingBars)
	result.CAGR = cagrFromTrades(result.TradesList)
	result.InSampleTrades, result.OutOfSampleTrades, result.OutOfSampleReturn = outOfSampleStats(result.TradesList, len(candles))
	return result
}

func RunDowntrendMeanReversion(candles []ohlcv.Candle, cfg Config) Result {
	cfg = NormalizeConfig(cfg)
	const holdBars = 20
	result := Result{
		Strategy:       "Downtrend negative momentum + 20 bar mean reversion",
		ExecutionModel: "signal_at_close_execute_next_open",
		LookbackBars:   len(candles),
		CommissionBps:  cfg.CommissionBps,
		SlippageBps:    cfg.SlippageBps,
	}
	if len(candles) < 90 {
		result.Warnings = append(result.Warnings, "insufficient_bars")
		return result
	}
	closes := closeSeries(candles)
	sma20 := smaSeries(closes, 20)
	sma50 := smaSeries(closes, 50)
	ema12 := emaSeries(closes, 12)
	ema26 := emaSeries(closes, 26)
	inTrade := false
	var pendingEntry *Signal
	var pendingExit *Signal
	entryPrice := 0.0
	entryIndex := 0
	entrySignalAt := time.Time{}
	tradeReturns := []float64{}
	holdSamples := []float64{}
	equity := 1.0
	peak := 1.0
	maxDD := 0.0
	grossGain := 0.0
	grossLoss := 0.0
	for i := 51; i < len(candles); i++ {
		if pendingEntry != nil && !inTrade && i >= pendingEntry.BarIndex+cfg.ExecutionDelayBars {
			if pendingEntry.AvailableAt.After(candles[i].Time) {
				result.LookaheadViolations++
			}
			entryPrice = applyBuyCosts(candles[i].EffectiveOpen(), cfg)
			entryIndex = i
			entrySignalAt = pendingEntry.AvailableAt
			inTrade = entryPrice > 0
			pendingEntry = nil
		}
		if pendingExit != nil && inTrade && i >= pendingExit.BarIndex+cfg.ExecutionDelayBars {
			if pendingExit.AvailableAt.After(candles[i].Time) {
				result.LookaheadViolations++
			}
			exitPrice := applySellCosts(candles[i].EffectiveOpen(), cfg)
			ret := mathutil.SafeDiv(exitPrice-entryPrice, entryPrice)
			trade := Trade{
				EntrySignalAt: entrySignalAt,
				EntryTime:     candles[entryIndex].Time,
				ExitSignalAt:  pendingExit.AvailableAt,
				ExitTime:      candles[i].Time,
				EntryIndex:    entryIndex,
				ExitIndex:     i,
				EntryPrice:    entryPrice,
				ExitPrice:     exitPrice,
				Return:        ret,
				HoldingBars:   i - entryIndex,
			}
			result.TradesList = append(result.TradesList, trade)
			tradeReturns = append(tradeReturns, ret)
			holdSamples = append(holdSamples, float64(trade.HoldingBars))
			equity *= 1 + ret
			peak = math.Max(peak, equity)
			maxDD = math.Min(maxDD, mathutil.SafeDiv(equity-peak, peak))
			if ret >= 0 {
				grossGain += ret
			} else {
				grossLoss += math.Abs(ret)
			}
			inTrade = false
			pendingExit = nil
		}
		momentum := ema12[i] - ema26[i]
		downtrendNegativeMomentum := closes[i] < sma50[i] && sma20[i] < sma50[i] && momentum < 0
		extendedBelowShortTrend := sma20[i] > 0 && closes[i] < sma20[i]*0.98
		enter := downtrendNegativeMomentum && extendedBelowShortTrend
		exit := inTrade && i-entryIndex >= holdBars
		if !inTrade && pendingEntry == nil && enter {
			signal := Signal{BarIndex: i, AvailableAt: candles[i].Time, Direction: "long", Reason: "downtrend_negative_momentum_mean_reversion_entry"}
			pendingEntry = &signal
			continue
		}
		if exit && pendingExit == nil {
			signal := Signal{BarIndex: i, AvailableAt: candles[i].Time, Direction: "flat", Reason: "fixed_20_bar_mean_reversion_exit"}
			pendingExit = &signal
		}
	}
	result.CurrentInMarket = inTrade || pendingEntry != nil
	wins := 0
	for _, ret := range tradeReturns {
		if ret > 0 {
			wins++
		}
	}
	result.Trades = len(tradeReturns)
	result.WinRate = mathutil.SafeDiv(float64(wins), float64(len(tradeReturns)))
	result.AverageReturn = mathutil.Mean(tradeReturns)
	result.MedianReturn = median(tradeReturns)
	result.ProfitFactor = mathutil.SafeDiv(grossGain, grossLoss)
	result.MaxDrawdown = maxDD
	result.Expectancy = result.AverageReturn
	result.AvgHoldingBars = mathutil.Mean(holdSamples)
	result.Exposure = exposureFromTrades(result.TradesList, len(candles))
	result.Volatility = annualizedVolatility(tradeReturns, holdSamples)
	result.Sharpe = mathutil.SafeDiv(result.AverageReturn, tradeReturnStdDev(tradeReturns)) * math.Sqrt(mathutil.SafeDiv(252, math.Max(result.AvgHoldingBars, 1)))
	result.Sortino = sortino(tradeReturns, result.AvgHoldingBars)
	result.CAGR = cagrFromTrades(result.TradesList)
	result.InSampleTrades, result.OutOfSampleTrades, result.OutOfSampleReturn = outOfSampleStats(result.TradesList, len(candles))
	return result
}

func outOfSampleStats(trades []Trade, bars int) (int, int, float64) {
	if bars <= 0 {
		return 0, 0, 0
	}
	split := int(float64(bars) * 0.7)
	oosReturns := []float64{}
	inSample := 0
	for _, trade := range trades {
		if trade.EntryIndex < split {
			inSample++
			continue
		}
		oosReturns = append(oosReturns, trade.Return)
	}
	return inSample, len(oosReturns), mathutil.Mean(oosReturns)
}

func exposureFromTrades(trades []Trade, bars int) float64 {
	if bars <= 0 {
		return 0
	}
	held := 0
	for _, trade := range trades {
		if trade.HoldingBars > 0 {
			held += trade.HoldingBars
		}
	}
	return mathutil.SafeDiv(float64(held), float64(bars))
}

func annualizedVolatility(returns []float64, holdBars []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	avgHold := math.Max(mathutil.Mean(holdBars), 1)
	return tradeReturnStdDev(returns) * math.Sqrt(252/avgHold)
}

func tradeReturnStdDev(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	mean := mathutil.Mean(returns)
	sum := 0.0
	for _, ret := range returns {
		diff := ret - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(returns)-1))
}

func sortino(returns []float64, avgHoldBars float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	downside := []float64{}
	for _, ret := range returns {
		if ret < 0 {
			downside = append(downside, ret)
		}
	}
	if len(downside) == 0 {
		return 0
	}
	downsideDev := tradeReturnStdDev(downside)
	return mathutil.SafeDiv(mathutil.Mean(returns), downsideDev) * math.Sqrt(mathutil.SafeDiv(252, math.Max(avgHoldBars, 1)))
}

func cagrFromTrades(trades []Trade) float64 {
	if len(trades) == 0 {
		return 0
	}
	first := trades[0].EntryTime
	last := trades[len(trades)-1].ExitTime
	if last.IsZero() || !last.After(first) {
		return 0
	}
	equity := 1.0
	for _, trade := range trades {
		equity *= 1 + trade.Return
	}
	years := last.Sub(first).Hours() / (24 * 365.25)
	if years <= 0 {
		return 0
	}
	return math.Pow(equity, 1/years) - 1
}

func applyBuyCosts(price float64, cfg Config) float64 {
	return price * (1 + (cfg.SlippageBps+cfg.CommissionBps)/10000)
}

func applySellCosts(price float64, cfg Config) float64 {
	return price * (1 - (cfg.SlippageBps+cfg.CommissionBps)/10000)
}

func closeSeries(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, candle := range candles {
		out[i] = candle.EffectiveClose()
	}
	return out
}

func smaSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 0 {
		return out
	}
	sum := 0.0
	for i, value := range values {
		sum += value
		if i >= period {
			sum -= values[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

func emaSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || period <= 0 {
		return out
	}
	alpha := 2.0 / float64(period+1)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64{}, values...)
	sortFloat64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func sortFloat64s(values []float64) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
