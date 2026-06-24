package backtest

type PortfolioState struct {
	Cash       float64 `json:"cash"`
	Position   float64 `json:"position"`
	Equity     float64 `json:"equity"`
	Turnover   float64 `json:"turnover"`
	TradeCount int     `json:"trade_count"`
}

func (p PortfolioState) Mark(price float64) PortfolioState {
	p.Equity = p.Cash + p.Position*price
	return p
}
