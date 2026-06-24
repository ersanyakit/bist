package features

func addFundamentalFeatures(fv *FeatureVector) {
	defaults := []string{
		"revenue_growth", "net_income_growth", "ebitda_growth", "gross_margin",
		"operating_margin", "net_margin", "roe", "roa", "debt_equity",
		"net_debt_ebitda", "current_ratio", "quick_ratio", "fcf_margin",
		"pe", "pb", "ev_ebitda", "financial_quality_score", "buffett_checklist_score",
	}
	for _, key := range defaults {
		if _, ok := fv.Values[key]; !ok {
			fv.Values[key] = 0
		}
	}
	fv.Quality.StaleFields = append(fv.Quality.StaleFields, "fundamental_release_timestamp_not_loaded")
}
