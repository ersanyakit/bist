package validation

import "time"

func ApplyEmbargo(train []time.Time, validation DateRange, embargoDays int) []time.Time {
	if embargoDays <= 0 {
		return train
	}
	start := validation.From.AddDate(0, 0, -embargoDays)
	end := validation.To.AddDate(0, 0, embargoDays)
	out := make([]time.Time, 0, len(train))
	for _, d := range train {
		if d.Before(start) || d.After(end) {
			out = append(out, d)
		}
	}
	return out
}

func RangesOverlap(a, b DateRange) bool {
	if a.From.IsZero() || a.To.IsZero() || b.From.IsZero() || b.To.IsZero() {
		return false
	}
	return !a.To.Before(b.From) && !b.To.Before(a.From)
}
