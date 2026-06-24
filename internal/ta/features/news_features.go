package features

func addNewsFeatures(fv *FeatureVector) {
	for _, key := range []string{
		"kap_event_count_1d", "kap_event_count_3d", "kap_event_count_7d", "kap_event_count_30d",
		"material_disclosure_flag", "earnings_event_flag", "dividend_or_capital_increase_flag",
		"sentiment_score", "sentiment_confidence", "news_volume_zscore", "kap_risk_flags",
	} {
		if _, ok := fv.Values[key]; !ok {
			fv.Values[key] = 0
		}
	}
}
