package quant

type ModuleSupport struct {
	Name        string `json:"name"`
	Package     string `json:"package"`
	Function    string `json:"function"`
	AssetScope  string `json:"asset_scope"`
	DecisionUse string `json:"decision_use"`
}

func SupportedModules() []ModuleSupport {
	return []ModuleSupport{
		{
			Name:        "Return and momentum features",
			Package:     "internal/quant/core",
			Function:    "Returns/Mean/Quantile",
			AssetScope:  "equity,crypto",
			DecisionUse: "price_forecast_feature",
		},
		{
			Name:        "Realized volatility",
			Package:     "internal/quant/core",
			Function:    "StdDev/Variance",
			AssetScope:  "equity,crypto",
			DecisionUse: "risk_gate_and_position_size",
		},
		{
			Name:        "VaR and CVaR",
			Package:     "internal/quant/portfolio",
			Function:    "HistoricalVaR/HistoricalCVaR/ParametricVaR",
			AssetScope:  "equity,crypto",
			DecisionUse: "risk_gate_and_stop_validation",
		},
		{
			Name:        "Drawdown and risk-adjusted return",
			Package:     "internal/quant/portfolio",
			Function:    "PerformanceRatios",
			AssetScope:  "equity,crypto",
			DecisionUse: "risk_reward_filter",
		},
		{
			Name:        "Benchmark beta and relative strength",
			Package:     "internal/ta/professional",
			Function:    "buildMarketContext",
			AssetScope:  "equity,crypto",
			DecisionUse: "market_factor_filter",
		},
		{
			Name:        "Equity quant profile",
			Package:     "internal/quant/equity",
			Function:    "Evaluate",
			AssetScope:  "equity",
			DecisionUse: "verified_close_fundamental_benchmark_gate",
		},
		{
			Name:        "Crypto quant profile",
			Package:     "internal/quant/crypto",
			Function:    "Evaluate",
			AssetScope:  "crypto",
			DecisionUse: "twenty_four_seven_volatility_and_market_structure_gate",
		},
		{
			Name:        "Stress scenarios",
			Package:     "internal/ta/analysis",
			Function:    "buildTailStress",
			AssetScope:  "equity,crypto",
			DecisionUse: "downside_scenario_filter",
		},
		{
			Name:        "Liquidity and data integrity",
			Package:     "internal/ta/analysis",
			Function:    "buildLiquidityMicrostructure/buildDataIntegrity",
			AssetScope:  "equity,crypto",
			DecisionUse: "tradability_gate",
		},
	}
}
