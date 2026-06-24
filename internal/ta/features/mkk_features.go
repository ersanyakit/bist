package features

func addMKKFeatures(fv *FeatureVector) {
	for _, key := range []string{"float_change", "free_float_ratio", "free_float_market_cap"} {
		if _, ok := fv.Values[key]; !ok {
			fv.Values[key] = 0
		}
	}
}
