package backtest

type RiskConfig struct {
	MaxPositionSize        float64 `json:"max_position_size"`
	MaxDailyTurnover       float64 `json:"max_daily_turnover"`
	LiquidityCap           float64 `json:"liquidity_cap"`
	VolumeParticipationCap float64 `json:"volume_participation_cap"`
}

func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		MaxPositionSize:        1,
		MaxDailyTurnover:       1,
		LiquidityCap:           0.10,
		VolumeParticipationCap: 0.05,
	}
}

func CheckRisk(size, volumeParticipation float64, cfg RiskConfig) (bool, string) {
	if cfg.MaxPositionSize <= 0 {
		cfg = DefaultRiskConfig()
	}
	if size > cfg.MaxPositionSize {
		return false, "max_position_size_exceeded"
	}
	if cfg.VolumeParticipationCap > 0 && volumeParticipation > cfg.VolumeParticipationCap {
		return false, "volume_participation_cap_exceeded"
	}
	return true, ""
}
