package chart

import (
	"strings"
	"testing"

	"hissebot/internal/ta/ohlcv"
)

func TestEvaluateDecisionDoesNotExposeRejectedShortPlanAsBuyLevels(t *testing.T) {
	evaluation := evaluateDecision(DecisionRenderInput{
		LastClose:         371.25,
		OverallScore:      64,
		TimeframeScore:    45,
		TrendBias:         "bearish",
		NearestSupport:    &ohlcv.SupportResistanceLevel{Type: "support", Price: 336.38},
		NearestResistance: &ohlcv.SupportResistanceLevel{Type: "resistance", Price: 390.25},
		Indicators: ohlcv.IndicatorSnapshot{
			RSI14:         44.46,
			MACD:          -1,
			MACDSignal:    1,
			MACDHistogram: -2.86,
			ATR14:         19.25,
			SMA20:         390,
			SMA50:         380,
			EMA20:         386,
		},
		TradePlan: ohlcv.TradePlan{
			Direction:       "short",
			EntryMin:        364.51,
			EntryMax:        389.81,
			TakeProfit1:     326,
			TakeProfit2:     287.49,
			StopLoss:        416.77,
			RiskRewardRatio: 1.29,
			Rejected:        true,
			RejectReason:    "risk reward ratio is below 1.5",
		},
	}, decisionPalette{})

	if evaluation.StopLevel != "336.38" {
		t.Fatalf("stop level = %q", evaluation.StopLevel)
	}
	if evaluation.Target1 != "390.25" || evaluation.Target2 != "Yok (reddedildi)" {
		t.Fatalf("watch levels should use support/resistance instead of rejected short targets: %+v", evaluation)
	}
	if !strings.Contains(evaluation.PlanStatus, "Satış taslak reddedildi") {
		t.Fatalf("plan status should describe rejected short draft, got %q", evaluation.PlanStatus)
	}
	for _, rule := range evaluation.ExitRules {
		if strings.Contains(rule, "416.77") {
			t.Fatalf("exit rule should not use rejected short stop as buy stop: %q", rule)
		}
	}
}

func TestEvaluateDecisionUsesDataDrivenWaitReasonsForBearishSpotSetup(t *testing.T) {
	evaluation := evaluateDecision(DecisionRenderInput{
		LastClose:         371.25,
		OverallScore:      64,
		TimeframeScore:    37,
		TrendBias:         "bearish",
		NearestSupport:    &ohlcv.SupportResistanceLevel{Type: "support", Price: 336.38},
		NearestResistance: &ohlcv.SupportResistanceLevel{Type: "resistance", Price: 390.25},
		Indicators: ohlcv.IndicatorSnapshot{
			RSI14:         44.46,
			MACD:          -1,
			MACDSignal:    1,
			MACDHistogram: -2.86,
			SMA20:         388.96,
			SMA50:         389.45,
			EMA20:         384.62,
		},
		TradePlan: ohlcv.TradePlan{
			Direction:    "neutral",
			Rejected:     true,
			RejectReason: "Aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor",
		},
	}, decisionPalette{})

	if evaluation.ReasonsTitle != "Giriş İçin Teyitler" {
		t.Fatalf("reason title = %q", evaluation.ReasonsTitle)
	}
	assertNoGenericWaitReason(t, evaluation.ReasonsFor)
	assertReasonContains(t, evaluation.ReasonsFor, "Düşüş sinyali")
	assertReasonContains(t, evaluation.ReasonsFor, "390.25")
	allText := strings.ToLower(strings.Join(append(append([]string{evaluation.Comment}, evaluation.Risks...), evaluation.ExitRules...), " "))
	for _, banned := range []string{"short", "marjin"} {
		if strings.Contains(allText, banned) {
			t.Fatalf("decision panel leaked internal risk wording %q in %q", banned, allText)
		}
	}
}

func TestEvaluateDecisionUsesDataDrivenWaitReasonsForRejectedLongPlan(t *testing.T) {
	evaluation := evaluateDecision(DecisionRenderInput{
		LastClose:      371.25,
		OverallScore:   64,
		TimeframeScore: 74,
		TrendBias:      "bullish",
		NearestSupport: &ohlcv.SupportResistanceLevel{Type: "support", Price: 66.70},
		Indicators: ohlcv.IndicatorSnapshot{
			RSI14:         77.66,
			MACD:          10,
			MACDSignal:    8,
			MACDHistogram: 18.74,
			SMA20:         209.03,
			SMA50:         106.15,
			EMA20:         237.06,
		},
		TradePlan: ohlcv.TradePlan{
			Direction:       "long",
			RiskRewardRatio: 1.34,
			Rejected:        true,
			RejectReason:    "risk reward ratio is below 1.5",
		},
	}, decisionPalette{})

	assertNoGenericWaitReason(t, evaluation.ReasonsFor)
	assertReasonContains(t, evaluation.ReasonsFor, "1.34")
	assertReasonContains(t, evaluation.ReasonsFor, "RSI 77.66")
}

func TestEvaluateDecisionUsesDataDrivenWaitReasonsForActiveLongWithWeakMomentum(t *testing.T) {
	evaluation := evaluateDecision(DecisionRenderInput{
		LastClose:         371.25,
		OverallScore:      64,
		TimeframeScore:    80,
		TrendBias:         "bullish",
		NearestSupport:    &ohlcv.SupportResistanceLevel{Type: "support", Price: 339.25},
		NearestResistance: &ohlcv.SupportResistanceLevel{Type: "resistance", Price: 450},
		Indicators: ohlcv.IndicatorSnapshot{
			RSI14:         58.97,
			MACD:          -4,
			MACDSignal:    2,
			MACDHistogram: -6.02,
			SMA20:         358.86,
			SMA50:         264.27,
			EMA20:         354.83,
		},
		TradePlan: ohlcv.TradePlan{
			Direction:       "long",
			EntryMax:        379.05,
			StopLoss:        331.45,
			TakeProfit1:     423.60,
			TakeProfit2:     468.15,
			RiskRewardRatio: 1.68,
		},
	}, decisionPalette{})

	if evaluation.Decision != "BEKLE" {
		t.Fatalf("expected wait until momentum confirms, got %q", evaluation.Decision)
	}
	assertNoGenericWaitReason(t, evaluation.ReasonsFor)
	assertReasonContains(t, evaluation.ReasonsFor, "379.05")
	assertReasonContains(t, evaluation.ReasonsFor, "MACD histogramı -6.02")
}

func assertNoGenericWaitReason(t *testing.T, reasons []string) {
	t.Helper()
	for _, reason := range reasons {
		if strings.Contains(reason, "Şu an giriş için yeterli teknik gerekçe yok") {
			t.Fatalf("generic wait reason leaked into decision output: %+v", reasons)
		}
	}
}

func assertReasonContains(t *testing.T, reasons []string, needle string) {
	t.Helper()
	for _, reason := range reasons {
		if strings.Contains(reason, needle) {
			return
		}
	}
	t.Fatalf("expected reason containing %q, got %+v", needle, reasons)
}
