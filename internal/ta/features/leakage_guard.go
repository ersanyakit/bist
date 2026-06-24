package features

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func GuardFeatureVector(fv FeatureVector) error {
	flags := LeakageFlags(fv)
	if len(flags) == 0 {
		return nil
	}
	return fmt.Errorf("feature leakage guard failed: %s", strings.Join(flags, ","))
}

func LeakageFlags(fv FeatureVector) []string {
	flags := []string{}
	cutoff := endOfAsOf(fv.AsOf)
	for source, ts := range fv.SourceTimestamps {
		if ts.After(cutoff) {
			flags = append(flags, "future_source_timestamp:"+source)
		}
	}
	for name, value := range fv.Values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			flags = append(flags, "non_finite_feature:"+name)
		}
	}
	sort.Strings(flags)
	return flags
}
