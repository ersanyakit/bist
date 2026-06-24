package forecastpolicy

import (
	"math"
	"strings"
	"time"
)

const NextSessionDirectionTolerancePct = 0.05

func NextSessionDirectionToleranceReturn() float64 {
	return NextSessionDirectionTolerancePct / 100
}

func IsNeutralChangePct(changePct float64) bool {
	return math.Abs(changePct) <= NextSessionDirectionTolerancePct
}

func IsNeutralReturn(ret float64) bool {
	return math.Abs(ret) <= NextSessionDirectionToleranceReturn()
}

func DirectionFromChangePct(changePct float64) string {
	switch {
	case changePct > NextSessionDirectionTolerancePct:
		return "up"
	case changePct < -NextSessionDirectionTolerancePct:
		return "down"
	default:
		return "flat"
	}
}

func BiasFromChangePct(changePct float64) string {
	switch DirectionFromChangePct(changePct) {
	case "up":
		return "bullish"
	case "down":
		return "bearish"
	default:
		return "neutral"
	}
}

func TurkishDirectionFromChangePct(changePct float64) string {
	switch DirectionFromChangePct(changePct) {
	case "up":
		return "yükseliş"
	case "down":
		return "düşüş"
	default:
		return "yatay"
	}
}

func DirectionFromReturn(ret float64) string {
	switch {
	case ret > NextSessionDirectionToleranceReturn():
		return "up"
	case ret < -NextSessionDirectionToleranceReturn():
		return "down"
	default:
		return "flat"
	}
}

func AuditDirectionFromPrice(price, lastClose float64) string {
	direction, ok := PriceDirection(price, lastClose)
	if !ok {
		return "unknown"
	}
	switch direction {
	case "up":
		return "yukari"
	case "down":
		return "asagi"
	default:
		return "yatay"
	}
}

func PriceDirection(price, lastClose float64) (string, bool) {
	if price <= 0 || lastClose <= 0 {
		return "", false
	}
	return DirectionFromReturn(price/lastClose - 1), true
}

func DirectionHit(predicted, actual, lastClose float64) (bool, bool) {
	predictedDirection, okPredicted := PriceDirection(predicted, lastClose)
	actualDirection, okActual := PriceDirection(actual, lastClose)
	if !okPredicted || !okActual {
		return false, false
	}
	return predictedDirection == actualDirection, true
}

func NextWeekdaySession(asOf time.Time) time.Time {
	if asOf.IsZero() {
		return time.Time{}
	}
	next := asOf.UTC().AddDate(0, 0, 1)
	for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func NextSessionForAssetType(asOf time.Time, assetType string) time.Time {
	if asOf.IsZero() {
		return time.Time{}
	}
	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "crypto", "kripto":
		return asOf.UTC().AddDate(0, 0, 1)
	default:
		return NextWeekdaySession(asOf)
	}
}
