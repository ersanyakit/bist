package backtest

import "math"

type CostsConfig struct {
	CommissionBps            float64 `json:"commission_bps"`
	SlippageBps              float64 `json:"slippage_bps"`
	SpreadBps                float64 `json:"spread_bps"`
	VolumeParticipationLimit float64 `json:"volume_participation_limit"`
	BISTFeeBps               float64 `json:"bist_fee_bps"`
}

func DefaultCostsConfig() CostsConfig {
	return CostsConfig{
		CommissionBps:            5,
		SlippageBps:              10,
		SpreadBps:                5,
		VolumeParticipationLimit: 0.05,
	}
}

func RoundTripCostPct(cfg CostsConfig) float64 {
	if cfg.CommissionBps == 0 && cfg.SlippageBps == 0 && cfg.SpreadBps == 0 && cfg.BISTFeeBps == 0 {
		cfg = DefaultCostsConfig()
	}
	bps := cfg.CommissionBps + cfg.SlippageBps + cfg.SpreadBps + cfg.BISTFeeBps
	if math.IsNaN(bps) || math.IsInf(bps, 0) || bps < 0 {
		return 0
	}
	return bps / 10000
}
