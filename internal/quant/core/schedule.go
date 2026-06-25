package core

import "time"

type BusinessDayConvention string

const (
	Unadjusted        BusinessDayConvention = "unadjusted"
	Following         BusinessDayConvention = "following"
	ModifiedFollowing BusinessDayConvention = "modified_following"
	Preceding         BusinessDayConvention = "preceding"
)

func GenerateSchedule(start, end time.Time, tenorMonths int, convention BusinessDayConvention) []time.Time {
	if tenorMonths <= 0 || start.IsZero() || end.IsZero() || !start.Before(end) {
		return nil
	}
	out := []time.Time{}
	for d := start; d.Before(end); d = d.AddDate(0, tenorMonths, 0) {
		if d.After(start) {
			out = append(out, AdjustBusinessDay(d, convention))
		}
	}
	out = append(out, AdjustBusinessDay(end, convention))
	return out
}

func AdjustBusinessDay(date time.Time, convention BusinessDayConvention) time.Time {
	if convention == Unadjusted || isBusinessDay(date) {
		return date
	}
	switch convention {
	case Preceding:
		for !isBusinessDay(date) {
			date = date.AddDate(0, 0, -1)
		}
		return date
	case ModifiedFollowing:
		origMonth := date.Month()
		followed := AdjustBusinessDay(date, Following)
		if followed.Month() != origMonth {
			return AdjustBusinessDay(date, Preceding)
		}
		return followed
	default:
		for !isBusinessDay(date) {
			date = date.AddDate(0, 0, 1)
		}
		return date
	}
}

func isBusinessDay(date time.Time) bool {
	weekday := date.Weekday()
	return weekday != time.Saturday && weekday != time.Sunday
}
