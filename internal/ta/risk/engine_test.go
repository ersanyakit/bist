package risk

import (
	"strings"
	"testing"

	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/supportresistance"
)

func TestBuildTradePlanIgnoresIrrelevantFarLongLevels(t *testing.T) {
	plan, err := BuildTradePlan(Input{
		LastPrice: 100,
		ATR:       3,
		Bias:      "bullish",
		Levels: supportresistance.Result{
			NearestSupport:    srLevel("support", 20),
			NearestResistance: srLevel("resistance", 300),
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.Direction != "long" {
		t.Fatalf("direction = %s, want long", plan.Direction)
	}
	assertTradePlanSane(t, 100, plan)
	if plan.StopLoss <= 85 {
		t.Fatalf("far support leaked into stop loss: stop=%.2f", plan.StopLoss)
	}
	if plan.TakeProfit1 >= 140 {
		t.Fatalf("far resistance leaked into first target: tp1=%.2f", plan.TakeProfit1)
	}
	if !reasonContains(plan.Reasoning, "ignored because it is too far") {
		t.Fatalf("expected far level reasoning, got %+v", plan.Reasoning)
	}
}

func TestBuildTradePlanIgnoresIrrelevantFarShortLevels(t *testing.T) {
	plan, err := BuildTradePlan(Input{
		LastPrice:  100,
		ATR:        3,
		Bias:       "bearish",
		AllowShort: true,
		Levels: supportresistance.Result{
			NearestSupport:    srLevel("support", 10),
			NearestResistance: srLevel("resistance", 180),
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.Direction != "short" {
		t.Fatalf("direction = %s, want short", plan.Direction)
	}
	assertTradePlanSane(t, 100, plan)
	if plan.StopLoss >= 115 {
		t.Fatalf("far resistance leaked into stop loss: stop=%.2f", plan.StopLoss)
	}
	if plan.TakeProfit1 <= 60 {
		t.Fatalf("far support leaked into first target: tp1=%.2f", plan.TakeProfit1)
	}
	if !reasonContains(plan.Reasoning, "ignored because it is too far") {
		t.Fatalf("expected far level reasoning, got %+v", plan.Reasoning)
	}
}

func TestBuildTradePlanRejectsShortWhenShortSellingDisabled(t *testing.T) {
	plan, err := BuildTradePlan(Input{
		LastPrice: 100,
		ATR:       3,
		Bias:      "bearish",
		Levels: supportresistance.Result{
			NearestSupport:    srLevel("support", 95),
			NearestResistance: srLevel("resistance", 105),
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.Direction != "neutral" {
		t.Fatalf("spot equity bearish setup should not create short direction: %+v", plan)
	}
	if !plan.Rejected {
		t.Fatalf("spot equity bearish setup should be rejected: %+v", plan)
	}
	if plan.StopLoss != 0 || plan.EntryMin != 0 || plan.EntryMax != 0 {
		t.Fatalf("rejected spot equity short should not expose active levels: %+v", plan)
	}
	if plan.RejectReason != "short selling is not supported for this instrument" {
		t.Fatalf("reject reason = %q", plan.RejectReason)
	}
}

func TestBuildTradePlanCapsAbnormallyLargeATR(t *testing.T) {
	plan, err := BuildTradePlan(Input{
		LastPrice: 100,
		ATR:       100,
		Bias:      "bullish",
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	assertTradePlanSane(t, 100, plan)
	if plan.EntryMin < 95 || plan.EntryMax > 105 {
		t.Fatalf("entry band should be capped near price, got %.2f - %.2f", plan.EntryMin, plan.EntryMax)
	}
	if plan.StopLoss <= 85 {
		t.Fatalf("large ATR should not create absurd stop loss: %.2f", plan.StopLoss)
	}
}

func TestBuildTradePlanUsesNearbySupportWithoutCreatingAbsurdStop(t *testing.T) {
	plan, err := BuildTradePlan(Input{
		LastPrice: 100,
		ATR:       2,
		Bias:      "bullish",
		Levels: supportresistance.Result{
			NearestSupport:    srLevel("support", 96),
			NearestResistance: srLevel("resistance", 108),
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	assertTradePlanSane(t, 100, plan)
	if plan.StopLoss >= plan.EntryMin {
		t.Fatalf("long stop must be below entry: %+v", plan)
	}
	if plan.StopLoss < 85 {
		t.Fatalf("near support produced over-wide stop: %.2f", plan.StopLoss)
	}
	if !reasonContains(plan.Reasoning, "Nearest support is used") {
		t.Fatalf("expected support reasoning, got %+v", plan.Reasoning)
	}
}

func TestBuildTradePlanExplainsATRTargetWhenLongResistanceIsTooClose(t *testing.T) {
	plan, err := BuildTradePlan(Input{
		LastPrice: 100,
		ATR:       2,
		Bias:      "bullish",
		Levels: supportresistance.Result{
			NearestSupport:    srLevel("support", 96),
			NearestResistance: srLevel("resistance", 101),
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	assertTradePlanSane(t, 100, plan)
	if plan.TakeProfit1 <= 101 {
		t.Fatalf("near resistance should not become a weak first target: %+v", plan)
	}
	if reasonContains(plan.Reasoning, "used as first upside objective") {
		t.Fatalf("near resistance was incorrectly reported as target: %+v", plan.Reasoning)
	}
	if !reasonContains(plan.Reasoning, "ATR based upside objectives") {
		t.Fatalf("expected ATR based target reasoning, got %+v", plan.Reasoning)
	}
}

func TestBuildTradePlanExplainsATRTargetWhenShortSupportIsTooClose(t *testing.T) {
	plan, err := BuildTradePlan(Input{
		LastPrice:  100,
		ATR:        2,
		Bias:       "bearish",
		AllowShort: true,
		Levels: supportresistance.Result{
			NearestSupport:    srLevel("support", 99),
			NearestResistance: srLevel("resistance", 104),
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	assertTradePlanSane(t, 100, plan)
	if plan.TakeProfit1 >= 99 {
		t.Fatalf("near support should not become a weak first target: %+v", plan)
	}
	if reasonContains(plan.Reasoning, "used as first downside objective") {
		t.Fatalf("near support was incorrectly reported as target: %+v", plan.Reasoning)
	}
	if !reasonContains(plan.Reasoning, "ATR based downside objectives") {
		t.Fatalf("expected ATR based target reasoning, got %+v", plan.Reasoning)
	}
}

func assertTradePlanSane(t *testing.T, lastPrice float64, plan ohlcv.TradePlan) {
	t.Helper()
	if plan.EntryMin <= 0 || plan.EntryMax <= 0 || plan.StopLoss <= 0 || plan.TakeProfit1 <= 0 || plan.TakeProfit2 <= 0 {
		t.Fatalf("plan contains non-positive levels: %+v", plan)
	}
	if plan.EntryMin > plan.EntryMax {
		t.Fatalf("entry min exceeds entry max: %+v", plan)
	}
	if pctDistance(lastPrice, plan.EntryMin) > maxEntryDistance+0.001 || pctDistance(lastPrice, plan.EntryMax) > maxEntryDistance+0.001 {
		t.Fatalf("entry band is too far from price %.2f: %+v", lastPrice, plan)
	}
	entryMid := (plan.EntryMin + plan.EntryMax) / 2
	switch plan.Direction {
	case "long":
		if plan.StopLoss >= plan.EntryMin || plan.TakeProfit1 <= plan.EntryMax || plan.TakeProfit2 <= plan.TakeProfit1 {
			t.Fatalf("long plan levels are inconsistent: %+v", plan)
		}
		if (entryMid-plan.StopLoss)/entryMid > maxStopDistance+0.001 {
			t.Fatalf("long stop distance exceeds guardrail: %+v", plan)
		}
	case "short":
		if plan.StopLoss <= plan.EntryMax || plan.TakeProfit1 >= plan.EntryMin || plan.TakeProfit2 >= plan.TakeProfit1 {
			t.Fatalf("short plan levels are inconsistent: %+v", plan)
		}
		if (plan.StopLoss-entryMid)/entryMid > maxStopDistance+0.001 {
			t.Fatalf("short stop distance exceeds guardrail: %+v", plan)
		}
	default:
		t.Fatalf("unexpected direction %q", plan.Direction)
	}
}

func srLevel(kind string, price float64) *ohlcv.SupportResistanceLevel {
	return &ohlcv.SupportResistanceLevel{Type: kind, Price: price, Strength: 0.8, TouchCount: 3}
}

func reasonContains(reasoning []string, needle string) bool {
	for _, reason := range reasoning {
		if strings.Contains(reason, needle) {
			return true
		}
	}
	return false
}
