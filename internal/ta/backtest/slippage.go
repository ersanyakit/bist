package backtest

func EstimateSlippagePct(cfg CostsConfig, volatilityPct, participation float64) float64 {
	base := cfg.SlippageBps / 10000
	if participation > cfg.VolumeParticipationLimit && cfg.VolumeParticipationLimit > 0 {
		base += (participation - cfg.VolumeParticipationLimit) * 0.5
	}
	if volatilityPct > 0 {
		base += volatilityPct * 0.05
	}
	if base < 0 {
		return 0
	}
	return base
}
