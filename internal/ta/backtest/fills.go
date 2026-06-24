package backtest

type Fill struct {
	Date         string  `json:"date"`
	Action       string  `json:"action"`
	Requested    float64 `json:"requested"`
	Filled       float64 `json:"filled"`
	Price        float64 `json:"price"`
	Partial      bool    `json:"partial"`
	RejectReason string  `json:"reject_reason,omitempty"`
}

func SimulateFill(action string, requested, price, volume, maxParticipation float64) Fill {
	fill := Fill{Action: action, Requested: requested, Price: price}
	if requested <= 0 || price <= 0 {
		fill.RejectReason = "invalid_order"
		return fill
	}
	limit := volume * maxParticipation
	if maxParticipation <= 0 || limit <= 0 {
		fill.Filled = requested
		return fill
	}
	if requested > limit {
		fill.Filled = limit
		fill.Partial = true
		return fill
	}
	fill.Filled = requested
	return fill
}
