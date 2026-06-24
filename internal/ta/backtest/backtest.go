package backtest

import (
	"math"

	"hissebot/internal/ta/features"
	"hissebot/internal/ta/validation"
)

type CostTrade struct {
	Date            string  `json:"date"`
	Action          string  `json:"action"`
	Entry           float64 `json:"entry"`
	Exit            float64 `json:"exit"`
	GrossReturn     float64 `json:"gross_return"`
	NetReturn       float64 `json:"net_return"`
	CostPct         float64 `json:"cost_pct"`
	Rejected        bool    `json:"rejected"`
	RejectionReason string  `json:"rejection_reason,omitempty"`
}

type Report struct {
	Strategy           string             `json:"strategy"`
	Symbol             string             `json:"symbol"`
	From               string             `json:"from"`
	To                 string             `json:"to"`
	NetReturn          float64            `json:"net_return"`
	GrossReturn        float64            `json:"gross_return"`
	Sharpe             float64            `json:"sharpe"`
	Sortino            float64            `json:"sortino"`
	MaxDrawdown        float64            `json:"max_drawdown"`
	Turnover           float64            `json:"turnover"`
	HitRatio           float64            `json:"hit_ratio"`
	AverageTradeReturn float64            `json:"average_trade_return"`
	TradeCount         int                `json:"trade_count"`
	RejectedTrades     int                `json:"rejected_trades"`
	RejectionReasons   map[string]int     `json:"rejection_reasons,omitempty"`
	Metrics            validation.Metrics `json:"metrics"`
	Trades             []CostTrade        `json:"trades,omitempty"`
}

func RunCostBacktest(symbol, strategy string, bars []features.MarketBar, cfg CostsConfig) Report {
	report := Report{Symbol: symbol, Strategy: strategy, RejectionReasons: map[string]int{}}
	if len(bars) < 2 {
		return report
	}
	report.From = bars[0].Time.Format("2006-01-02")
	report.To = bars[len(bars)-1].Time.Format("2006-01-02")
	cost := RoundTripCostPct(cfg)
	returns := []float64{}
	for i := 1; i < len(bars); i++ {
		entry := bars[i].Open
		exit := bars[i].Close
		if entry <= 0 || exit <= 0 {
			report.RejectedTrades++
			report.RejectionReasons["invalid_price"]++
			continue
		}
		gross := exit/entry - 1
		action := "buy"
		if bars[i-1].Close > 0 && bars[i-1].Close > bars[i].Open {
			action = "sell"
			gross = entry/exit - 1
		}
		net := gross - cost
		trade := CostTrade{
			Date:        bars[i].Time.Format("2006-01-02"),
			Action:      action,
			Entry:       entry,
			Exit:        exit,
			GrossReturn: gross,
			NetReturn:   net,
			CostPct:     cost,
		}
		report.Trades = append(report.Trades, trade)
		returns = append(returns, net)
		report.GrossReturn += gross
		report.NetReturn += net
		report.Turnover += 1
	}
	report.TradeCount = len(report.Trades)
	if report.TradeCount > 0 {
		report.AverageTradeReturn = report.NetReturn / float64(report.TradeCount)
	}
	report.Metrics = validation.EconomicMetrics(returns)
	report.Sharpe = report.Metrics.Sharpe
	report.Sortino = report.Metrics.Sortino
	report.MaxDrawdown = report.Metrics.MaxDrawdown
	report.HitRatio = report.Metrics.HitRatio
	if math.IsNaN(report.NetReturn) || math.IsInf(report.NetReturn, 0) {
		report.NetReturn = 0
	}
	return report
}
