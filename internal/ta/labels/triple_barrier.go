package labels

import "hissebot/internal/ta/features"

func TripleBarrierLabel(bars []features.MarketBar, index int, opts Options) string {
	if index < 0 || index+1 >= len(bars) {
		return "timeout"
	}
	entry := bars[index].Close
	if entry <= 0 {
		return "timeout"
	}
	maxBars := opts.MaxHoldingBars
	if maxBars <= 0 {
		maxBars = 5
	}
	end := index + maxBars
	if end >= len(bars) {
		end = len(bars) - 1
	}
	pt := opts.ProfitTaking
	sl := opts.StopLoss
	for i := index + 1; i <= end; i++ {
		if bars[i].High/entry-1 >= pt {
			return "take_profit"
		}
		if bars[i].Low/entry-1 <= -sl {
			return "stop_loss"
		}
	}
	return "timeout"
}
