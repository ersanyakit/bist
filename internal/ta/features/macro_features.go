package features

func addMacroFeatures(fv *FeatureVector) {
	for _, key := range []string{
		"usdtry_change", "interest_rate_level", "interest_rate_change",
		"inflation_proxy", "bist100_return", "sector_index_return", "market_volatility_proxy",
	} {
		if _, ok := fv.Values[key]; !ok {
			fv.Values[key] = 0
		}
	}
	fv.Quality.StaleFields = append(fv.Quality.StaleFields, "macro_release_timestamp_not_loaded")
}
