package patterns

import "hissebot/internal/ta/ohlcv"

type patternRuleID string

const (
	ruleABCD                        patternRuleID = "abcd"
	ruleAdamEveBottom               patternRuleID = "adam_eve_bottom"
	ruleAdamEveTop                  patternRuleID = "adam_eve_top"
	ruleAMD                         patternRuleID = "amd"
	ruleAutomaticRally              patternRuleID = "automatic_rally"
	ruleAutomaticReaction           patternRuleID = "automatic_reaction"
	ruleBearTrap                    patternRuleID = "bear_trap"
	ruleBOS                         patternRuleID = "bos"
	ruleBOSBear                     patternRuleID = "bos_bear"
	ruleBOSBull                     patternRuleID = "bos_bull"
	ruleBreakRetest                 patternRuleID = "break_retest"
	ruleBreakdown                   patternRuleID = "breakdown"
	ruleBreaker                     patternRuleID = "breaker"
	ruleBreakout                    patternRuleID = "breakout"
	ruleBroadening                  patternRuleID = "broadening"
	ruleBumpRunBottom               patternRuleID = "bump_run_bottom"
	ruleBumpRunTop                  patternRuleID = "bump_run_top"
	ruleBuyingClimax                patternRuleID = "buying_climax"
	ruleCandlestick                 patternRuleID = "candlestick"
	ruleChannelAscending            patternRuleID = "channel_ascending"
	ruleChannelDescending           patternRuleID = "channel_descending"
	ruleChannelHorizontal           patternRuleID = "channel_horizontal"
	ruleCHOCHBear                   patternRuleID = "choch_bear"
	ruleCHOCHBull                   patternRuleID = "choch_bull"
	ruleCompression                 patternRuleID = "compression"
	ruleComplexHeadShoulders        patternRuleID = "complex_head_shoulders"
	ruleComplexInverseHeadShoulders patternRuleID = "complex_inverse_head_shoulders"
	ruleCupHandle                   patternRuleID = "cup_handle"
	ruleDeadCatBounce               patternRuleID = "dead_cat_bounce"
	ruleDiamondBottom               patternRuleID = "diamond_bottom"
	ruleDiamondTop                  patternRuleID = "diamond_top"
	ruleDiscountZone                patternRuleID = "discount_zone"
	ruleDoubleBottom                patternRuleID = "double_bottom"
	ruleDoubleTop                   patternRuleID = "double_top"
	ruleElliottABC                  patternRuleID = "elliott_abc"
	ruleElliottDiagonal             patternRuleID = "elliott_diagonal"
	ruleElliottExpandedFlat         patternRuleID = "elliott_expanded_flat"
	ruleElliottFlat                 patternRuleID = "elliott_flat"
	ruleElliottImpulse              patternRuleID = "elliott_impulse"
	ruleElliottRunningFlat          patternRuleID = "elliott_running_flat"
	ruleElliottZigzag               patternRuleID = "elliott_zigzag"
	ruleEqualHighs                  patternRuleID = "equal_highs"
	ruleEqualLows                   patternRuleID = "equal_lows"
	ruleEquilibriumZone             patternRuleID = "equilibrium_zone"
	ruleExpansion                   patternRuleID = "expansion"
	ruleFakey                       patternRuleID = "fakey"
	ruleFalseBreakout               patternRuleID = "false_breakout"
	ruleFibonacci                   patternRuleID = "fibonacci"
	ruleFiveZero                    patternRuleID = "five_zero"
	ruleFlagBear                    patternRuleID = "flag_bear"
	ruleFlagBull                    patternRuleID = "flag_bull"
	ruleFVG                         patternRuleID = "fvg"
	ruleGap                         patternRuleID = "gap"
	ruleGapDown                     patternRuleID = "gap_down"
	ruleGapUp                       patternRuleID = "gap_up"
	ruleHarmonicBat                 patternRuleID = "harmonic_bat"
	ruleHarmonicButterfly           patternRuleID = "harmonic_butterfly"
	ruleHarmonicCrab                patternRuleID = "harmonic_crab"
	ruleHarmonicCypher              patternRuleID = "harmonic_cypher"
	ruleHarmonicDeepCrab            patternRuleID = "harmonic_deep_crab"
	ruleHarmonicGartley             patternRuleID = "harmonic_gartley"
	ruleHarmonicGeneric             patternRuleID = "harmonic_generic"
	ruleHarmonicShark               patternRuleID = "harmonic_shark"
	ruleHeadShoulders               patternRuleID = "head_shoulders"
	ruleIndicator                   patternRuleID = "indicator"
	ruleInverseCupHandle            patternRuleID = "inverse_cup_handle"
	ruleInverseHeadShoulders        patternRuleID = "inverse_head_shoulders"
	ruleIslandBottom                patternRuleID = "island_bottom"
	ruleIslandTop                   patternRuleID = "island_top"
	ruleLiquiditySweep              patternRuleID = "liquidity_sweep"
	ruleLPS                         patternRuleID = "lps"
	ruleLPSY                        patternRuleID = "lpsy"
	ruleMarketProfile               patternRuleID = "market_profile"
	ruleMeasuredMoveDown            patternRuleID = "measured_move_down"
	ruleMeasuredMoveUp              patternRuleID = "measured_move_up"
	ruleMeanReversion               patternRuleID = "mean_reversion"
	ruleMitigation                  patternRuleID = "mitigation"
	ruleMotherBar                   patternRuleID = "mother_bar"
	ruleOrderBlockBear              patternRuleID = "order_block_bear"
	ruleOrderBlockBull              patternRuleID = "order_block_bull"
	rulePennantBear                 patternRuleID = "pennant_bear"
	rulePennantBull                 patternRuleID = "pennant_bull"
	rulePipeBottom                  patternRuleID = "pipe_bottom"
	rulePipeTop                     patternRuleID = "pipe_top"
	rulePocketPivot                 patternRuleID = "pocket_pivot"
	rulePointFigure                 patternRuleID = "point_figure"
	rulePremiumZone                 patternRuleID = "premium_zone"
	rulePullback                    patternRuleID = "pullback"
	ruleQuasimodo                   patternRuleID = "quasimodo"
	ruleRange                       patternRuleID = "range"
	ruleRectangle                   patternRuleID = "rectangle"
	ruleRoundedReversal             patternRuleID = "rounded_reversal"
	ruleRoundingBottom              patternRuleID = "rounding_bottom"
	ruleRoundingTop                 patternRuleID = "rounding_top"
	ruleSecondaryTest               patternRuleID = "secondary_test"
	ruleSellingClimax               patternRuleID = "selling_climax"
	ruleSOS                         patternRuleID = "sos"
	ruleSOW                         patternRuleID = "sow"
	ruleSpring                      patternRuleID = "spring"
	ruleStairStep                   patternRuleID = "stair_step"
	ruleThreeDrives                 patternRuleID = "three_drives"
	ruleTrendDown                   patternRuleID = "trend_down"
	ruleTrendExhaustion             patternRuleID = "trend_exhaustion"
	ruleTrendSideways               patternRuleID = "trend_sideways"
	ruleTrendUp                     patternRuleID = "trend_up"
	ruleTrendlineBreak              patternRuleID = "trendline_break"
	ruleTriangleAscending           patternRuleID = "triangle_ascending"
	ruleTriangleDescending          patternRuleID = "triangle_descending"
	ruleTriangleSymmetrical         patternRuleID = "triangle_symmetrical"
	ruleTripleBottom                patternRuleID = "triple_bottom"
	ruleTripleTop                   patternRuleID = "triple_top"
	ruleUpthrust                    patternRuleID = "upthrust"
	ruleVBottom                     patternRuleID = "v_bottom"
	ruleVTop                        patternRuleID = "v_top"
	ruleVolume                      patternRuleID = "volume"
	ruleWedge                       patternRuleID = "wedge"
	ruleWedgeFalling                patternRuleID = "wedge_falling"
	ruleWedgeRising                 patternRuleID = "wedge_rising"
	ruleWyckoffAccumulation         patternRuleID = "wyckoff_accumulation"
	ruleWyckoffComposite            patternRuleID = "wyckoff_composite"
	ruleWyckoffDistribution         patternRuleID = "wyckoff_distribution"
)

type patternRuleMatcher func(patternMatchContext, patternSpec) generatedMatch

type patternRule struct {
	id    patternRuleID
	match patternRuleMatcher
}

type patternMatchContext struct {
	input   ScannerInput
	candles []ohlcv.Candle
	swings  []swing
}

func newPatternMatchContext(input ScannerInput) patternMatchContext {
	candles := input.Candles
	swings := input.chartSwings
	if swings == nil && len(candles) >= 7 {
		swings = detectChartSwings(candles, 3)
	}
	return patternMatchContext{
		input:   input,
		candles: candles,
		swings:  swings,
	}
}

func registeredPatternRule(id patternRuleID) (patternRule, bool) {
	rule, ok := patternRuleRegistry[id]
	return rule, ok
}

func newPatternRule(id patternRuleID, match patternRuleMatcher) patternRule {
	return patternRule{id: id, match: match}
}

func swingRule(fn func([]ohlcv.Candle, []swing) (bool, int, int), direction, evidence string) patternRuleMatcher {
	return func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return adaptMatch(fn, ctx.candles, ctx.swings, direction, spec.Confidence, evidence)
	}
}

func specDirectionSwingRule(fn func([]ohlcv.Candle, []swing) (bool, int, int), evidence string) patternRuleMatcher {
	return func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return adaptMatch(fn, ctx.candles, ctx.swings, spec.Direction, spec.Confidence, evidence)
	}
}

var patternRuleRegistry = map[patternRuleID]patternRule{
	ruleCandlestick: newPatternRule(ruleCandlestick, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchCandlestickAlias(spec, ctx.candles)
	}),
	ruleDoubleTop:                   newPatternRule(ruleDoubleTop, swingRule(matchDouble("high", 2, 0.02, 8, 70, false), "bearish", "two comparable swing highs are visible")),
	ruleDoubleBottom:                newPatternRule(ruleDoubleBottom, swingRule(matchDouble("low", 2, 0.02, 8, 70, true), "bullish", "two comparable swing lows are visible")),
	ruleTripleTop:                   newPatternRule(ruleTripleTop, swingRule(matchTriple("high", 0.02), "bearish", "three comparable swing highs are visible")),
	ruleTripleBottom:                newPatternRule(ruleTripleBottom, swingRule(matchTriple("low", 0.02), "bullish", "three comparable swing lows are visible")),
	ruleHeadShoulders:               newPatternRule(ruleHeadShoulders, swingRule(matchHeadShoulders(false), "bearish", "head-and-shoulders swing sequence is visible")),
	ruleInverseHeadShoulders:        newPatternRule(ruleInverseHeadShoulders, swingRule(matchHeadShoulders(true), "bullish", "inverse head-and-shoulders swing sequence is visible")),
	ruleComplexHeadShoulders:        newPatternRule(ruleComplexHeadShoulders, swingRule(matchComplexHS(false), "bearish", "complex head-and-shoulders swing cluster is visible")),
	ruleComplexInverseHeadShoulders: newPatternRule(ruleComplexInverseHeadShoulders, swingRule(matchComplexHS(true), "bullish", "complex inverse head-and-shoulders swing cluster is visible")),
	ruleTriangleAscending:           newPatternRule(ruleTriangleAscending, swingRule(matchTriangle("ascending"), "bullish", "flat resistance and rising lows are visible")),
	ruleTriangleDescending:          newPatternRule(ruleTriangleDescending, swingRule(matchTriangle("descending"), "bearish", "flat support and falling highs are visible")),
	ruleTriangleSymmetrical:         newPatternRule(ruleTriangleSymmetrical, specDirectionSwingRule(matchTriangle("symmetrical"), "contracting highs and lows are visible")),
	ruleFlagBull:                    newPatternRule(ruleFlagBull, swingRule(matchFlag(true), "bullish", "bullish pole and corrective flag are visible")),
	ruleFlagBear:                    newPatternRule(ruleFlagBear, swingRule(matchFlag(false), "bearish", "bearish pole and corrective flag are visible")),
	rulePennantBull:                 newPatternRule(rulePennantBull, swingRule(matchPennant(true), "bullish", "bullish pole and compact pennant are visible")),
	rulePennantBear:                 newPatternRule(rulePennantBear, swingRule(matchPennant(false), "bearish", "bearish pole and compact pennant are visible")),
	ruleWedgeRising:                 newPatternRule(ruleWedgeRising, swingRule(matchWedge(true), "bearish", "rising narrowing wedge is visible")),
	ruleWedgeFalling:                newPatternRule(ruleWedgeFalling, swingRule(matchWedge(false), "bullish", "falling narrowing wedge is visible")),
	ruleCupHandle:                   newPatternRule(ruleCupHandle, swingRule(matchCup(false), "bullish", "rounded recovery and handle are visible")),
	ruleInverseCupHandle:            newPatternRule(ruleInverseCupHandle, swingRule(matchCup(true), "bearish", "rounded top and handle are visible")),
	ruleRectangle:                   newPatternRule(ruleRectangle, specDirectionSwingRule(matchRectangle, "horizontal range boundaries are visible")),
	ruleRange:                       newPatternRule(ruleRange, specDirectionSwingRule(matchRectangle, "horizontal range boundaries are visible")),
	ruleChannelHorizontal:           newPatternRule(ruleChannelHorizontal, swingRule(matchChannel("horizontal"), "neutral", "horizontal channel slope is visible")),
	ruleChannelAscending:            newPatternRule(ruleChannelAscending, swingRule(matchChannel("ascending"), "bullish", "ascending channel slope is visible")),
	ruleChannelDescending:           newPatternRule(ruleChannelDescending, swingRule(matchChannel("descending"), "bearish", "descending channel slope is visible")),
	ruleRoundingTop:                 newPatternRule(ruleRoundingTop, swingRule(matchRounding(true), "bearish", "rounded top arc is visible")),
	ruleRoundingBottom:              newPatternRule(ruleRoundingBottom, swingRule(matchRounding(false), "bullish", "rounded bottom arc is visible")),
	ruleBroadening:                  newPatternRule(ruleBroadening, specDirectionSwingRule(matchBroadening, "successive swings broaden")),
	ruleDiamondTop:                  newPatternRule(ruleDiamondTop, swingRule(matchDiamond(true), "bearish", "expansion then compression appears near highs")),
	ruleDiamondBottom:               newPatternRule(ruleDiamondBottom, swingRule(matchDiamond(false), "bullish", "expansion then compression appears near lows")),
	ruleABCD:                        newPatternRule(ruleABCD, specDirectionSwingRule(matchABCD, "AB and CD swing legs are comparable")),
	ruleHarmonicGartley:             newPatternRule(ruleHarmonicGartley, specDirectionSwingRule(matchHarmonic("gartley"), "Gartley-like harmonic ratios are visible")),
	ruleHarmonicBat:                 newPatternRule(ruleHarmonicBat, specDirectionSwingRule(matchHarmonic("bat"), "Bat-like harmonic ratios are visible")),
	ruleHarmonicButterfly:           newPatternRule(ruleHarmonicButterfly, specDirectionSwingRule(matchHarmonic("butterfly"), "Butterfly-like harmonic ratios are visible")),
	ruleHarmonicCrab:                newPatternRule(ruleHarmonicCrab, specDirectionSwingRule(matchHarmonic("crab"), "Crab-like harmonic extension is visible")),
	ruleHarmonicDeepCrab:            newPatternRule(ruleHarmonicDeepCrab, specDirectionSwingRule(matchHarmonic("deep_crab"), "Deep Crab-like harmonic extension is visible")),
	ruleHarmonicShark:               newPatternRule(ruleHarmonicShark, specDirectionSwingRule(matchHarmonic("shark"), "Shark-like harmonic ratios are visible")),
	ruleHarmonicCypher:              newPatternRule(ruleHarmonicCypher, specDirectionSwingRule(matchHarmonic("cypher"), "Cypher-like harmonic ratios are visible")),
	ruleHarmonicGeneric:             newPatternRule(ruleHarmonicGeneric, specDirectionSwingRule(matchHarmonic("generic"), "five-point harmonic proportions are visible")),
	ruleThreeDrives:                 newPatternRule(ruleThreeDrives, specDirectionSwingRule(matchThreeDrives, "three comparable drives are visible")),
	ruleFiveZero:                    newPatternRule(ruleFiveZero, specDirectionSwingRule(matchFiveZero, "5-0 swing retracement is visible")),
	ruleElliottImpulse:              newPatternRule(ruleElliottImpulse, specDirectionSwingRule(matchElliottImpulse, "alternating impulse-like swing sequence is visible")),
	ruleElliottABC:                  newPatternRule(ruleElliottABC, specDirectionSwingRule(matchABC, "ABC corrective swing sequence is visible")),
	ruleElliottZigzag:               newPatternRule(ruleElliottZigzag, specDirectionSwingRule(matchABC, "ABC corrective swing sequence is visible")),
	ruleElliottFlat:                 newPatternRule(ruleElliottFlat, specDirectionSwingRule(matchFlatCorrection, "flat correction structure is visible")),
	ruleElliottExpandedFlat:         newPatternRule(ruleElliottExpandedFlat, specDirectionSwingRule(matchExpandedFlat, "expanded flat correction structure is visible")),
	ruleElliottRunningFlat:          newPatternRule(ruleElliottRunningFlat, specDirectionSwingRule(matchRunningFlat, "running flat correction structure is visible")),
	ruleElliottDiagonal:             newPatternRule(ruleElliottDiagonal, specDirectionSwingRule(matchDiagonal, "diagonal-like overlapping swing sequence is visible")),
	ruleWyckoffAccumulation:         newPatternRule(ruleWyckoffAccumulation, swingRule(matchWyckoff(true), "bullish", "range behavior resembles accumulation")),
	ruleWyckoffDistribution:         newPatternRule(ruleWyckoffDistribution, swingRule(matchWyckoff(false), "bearish", "range behavior resembles distribution")),
	ruleSpring:                      newPatternRule(ruleSpring, swingRule(matchSpring, "bullish", "support sweep and reclaim are visible")),
	ruleUpthrust:                    newPatternRule(ruleUpthrust, swingRule(matchUpthrust, "bearish", "resistance sweep and rejection are visible")),
	ruleSOS:                         newPatternRule(ruleSOS, swingRule(matchSOS, "bullish", "strength breakout is visible")),
	ruleSOW:                         newPatternRule(ruleSOW, swingRule(matchSOW, "bearish", "weakness breakdown is visible")),
	ruleLPS:                         newPatternRule(ruleLPS, swingRule(matchLPS, "bullish", "pullback holds after strength")),
	ruleLPSY:                        newPatternRule(ruleLPSY, swingRule(matchLPSY, "bearish", "bounce fails after weakness")),
	ruleBuyingClimax:                newPatternRule(ruleBuyingClimax, swingRule(matchClimax(true), "bearish", "large high-volume up bar appears near highs")),
	ruleSellingClimax:               newPatternRule(ruleSellingClimax, swingRule(matchClimax(false), "bullish", "large high-volume down bar appears near lows")),
	ruleAutomaticRally:              newPatternRule(ruleAutomaticRally, swingRule(matchAutomatic(true), "bullish", "sharp rebound follows weakness")),
	ruleAutomaticReaction:           newPatternRule(ruleAutomaticReaction, swingRule(matchAutomatic(false), "bearish", "sharp reaction follows strength")),
	ruleWyckoffComposite: newPatternRule(ruleWyckoffComposite, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchWyckoffComposite(ctx.candles, ctx.swings, spec)
	}),
	ruleBOSBull: newPatternRule(ruleBOSBull, swingRule(matchBOS(true), "bullish", "close exceeds recent structure high")),
	ruleBOSBear: newPatternRule(ruleBOSBear, swingRule(matchBOS(false), "bearish", "close loses recent structure low")),
	ruleBOS: newPatternRule(ruleBOS, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchStructureBreak(ctx.candles, spec)
	}),
	ruleCHOCHBull: newPatternRule(ruleCHOCHBull, swingRule(matchCHOCH(true), "bullish", "bearish sequence changes into bullish break")),
	ruleCHOCHBear: newPatternRule(ruleCHOCHBear, swingRule(matchCHOCH(false), "bearish", "bullish sequence changes into bearish break")),
	ruleLiquiditySweep: newPatternRule(ruleLiquiditySweep, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchLiquiditySweep(ctx.candles, spec)
	}),
	ruleFVG:            newPatternRule(ruleFVG, specDirectionSwingRule(matchFairValueGap, "three-candle imbalance is visible")),
	ruleOrderBlockBull: newPatternRule(ruleOrderBlockBull, swingRule(matchOrderBlock(true), "bullish", "bullish displacement follows a bearish candle")),
	ruleOrderBlockBear: newPatternRule(ruleOrderBlockBear, swingRule(matchOrderBlock(false), "bearish", "bearish displacement follows a bullish candle")),
	ruleBreaker:        newPatternRule(ruleBreaker, specDirectionSwingRule(matchBreaker, "sweep and structure break occur together")),
	ruleMitigation:     newPatternRule(ruleMitigation, specDirectionSwingRule(matchMitigation, "price revisits a prior displacement zone")),
	ruleEqualHighs:     newPatternRule(ruleEqualHighs, swingRule(matchEqualLiquidity(true), "bearish", "near-equal swing highs are visible")),
	ruleEqualLows:      newPatternRule(ruleEqualLows, swingRule(matchEqualLiquidity(false), "bullish", "near-equal swing lows are visible")),
	ruleBreakRetest: newPatternRule(ruleBreakRetest, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchBreakRetest(ctx.candles, spec)
	}),
	rulePullback: newPatternRule(rulePullback, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchPullback(ctx.candles, spec)
	}),
	ruleFakey: newPatternRule(ruleFakey, func(ctx patternMatchContext, spec patternSpec) generatedMatch { return matchFakey(ctx.candles, spec) }),
	ruleMotherBar: newPatternRule(ruleMotherBar, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchMotherBar(ctx.candles, spec)
	}),
	ruleQuasimodo: newPatternRule(ruleQuasimodo, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchQuasimodo(ctx.candles, ctx.swings, spec)
	}),
	rulePremiumZone: newPatternRule(rulePremiumZone, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchRangeZone(ctx.candles, spec)
	}),
	ruleDiscountZone: newPatternRule(ruleDiscountZone, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchRangeZone(ctx.candles, spec)
	}),
	ruleEquilibriumZone: newPatternRule(ruleEquilibriumZone, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchRangeZone(ctx.candles, spec)
	}),
	ruleMeanReversion: newPatternRule(ruleMeanReversion, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchMeanReversion(ctx.input, spec)
	}),
	ruleDeadCatBounce: newPatternRule(ruleDeadCatBounce, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchDeadCatBounce(ctx.candles, spec)
	}),
	rulePocketPivot: newPatternRule(rulePocketPivot, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchPocketPivot(ctx.candles, spec)
	}),
	ruleTrendExhaustion: newPatternRule(ruleTrendExhaustion, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchTrendExhaustion(ctx.candles, spec)
	}),
	ruleStairStep: newPatternRule(ruleStairStep, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchStairStep(ctx.candles, ctx.swings, spec)
	}),
	ruleSecondaryTest: newPatternRule(ruleSecondaryTest, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchSecondaryTest(ctx.candles, spec)
	}),
	ruleTrendlineBreak: newPatternRule(ruleTrendlineBreak, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchTrendlineBreak(ctx.candles, spec)
	}),
	ruleRoundedReversal: newPatternRule(ruleRoundedReversal, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchRoundedReversal(ctx.candles, ctx.swings, spec)
	}),
	ruleWedge: newPatternRule(ruleWedge, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchWedgeEither(ctx.candles, ctx.swings, spec)
	}),
	ruleAMD:              newPatternRule(ruleAMD, func(ctx patternMatchContext, spec patternSpec) generatedMatch { return matchAMD(ctx.candles, spec) }),
	ruleMeasuredMoveUp:   newPatternRule(ruleMeasuredMoveUp, swingRule(matchMeasuredMove(true), "bullish", "two comparable upward swing legs are visible")),
	ruleMeasuredMoveDown: newPatternRule(ruleMeasuredMoveDown, swingRule(matchMeasuredMove(false), "bearish", "two comparable downward swing legs are visible")),
	ruleBumpRunTop:       newPatternRule(ruleBumpRunTop, swingRule(matchBumpRun(true), "bearish", "accelerating rise reverses")),
	ruleBumpRunBottom:    newPatternRule(ruleBumpRunBottom, swingRule(matchBumpRun(false), "bullish", "accelerating decline reverses")),
	ruleIslandTop:        newPatternRule(ruleIslandTop, swingRule(matchIsland(true), "bearish", "gap island top is visible")),
	ruleIslandBottom:     newPatternRule(ruleIslandBottom, swingRule(matchIsland(false), "bullish", "gap island bottom is visible")),
	rulePipeTop:          newPatternRule(rulePipeTop, swingRule(matchPipe(true), "bearish", "two adjacent wide candles form a top")),
	rulePipeBottom:       newPatternRule(rulePipeBottom, swingRule(matchPipe(false), "bullish", "two adjacent wide candles form a bottom")),
	ruleVTop:             newPatternRule(ruleVTop, swingRule(matchV(true), "bearish", "fast rise and fast reversal are visible")),
	ruleVBottom:          newPatternRule(ruleVBottom, swingRule(matchV(false), "bullish", "fast decline and fast reversal are visible")),
	ruleAdamEveTop:       newPatternRule(ruleAdamEveTop, swingRule(matchAdamEve(true, true), "bearish", "double-top variant is visible")),
	ruleAdamEveBottom:    newPatternRule(ruleAdamEveBottom, swingRule(matchAdamEve(false, true), "bullish", "double-bottom variant is visible")),
	ruleGap: newPatternRule(ruleGap, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchGap(ctx.candles, spec.Direction, spec.Confidence, "price gap/window is visible")
	}),
	ruleGapUp: newPatternRule(ruleGapUp, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchGapDirectional(ctx.candles, true, spec.Confidence, "upside gap is visible")
	}),
	ruleGapDown: newPatternRule(ruleGapDown, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchGapDirectional(ctx.candles, false, spec.Confidence, "downside gap is visible")
	}),
	ruleBreakout:      newPatternRule(ruleBreakout, swingRule(matchBOS(true), "bullish", "close breaks above recent range")),
	ruleBreakdown:     newPatternRule(ruleBreakdown, swingRule(matchBOS(false), "bearish", "close breaks below recent range")),
	ruleFalseBreakout: newPatternRule(ruleFalseBreakout, swingRule(matchUpthrust, "bearish", "upside break fails back inside range")),
	ruleBearTrap:      newPatternRule(ruleBearTrap, swingRule(matchSpring, "bullish", "downside break fails back inside range")),
	ruleCompression: newPatternRule(ruleCompression, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchCompression(ctx.candles, spec.Direction, spec.Confidence)
	}),
	ruleExpansion: newPatternRule(ruleExpansion, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchExpansion(ctx.candles, spec.Direction, spec.Confidence)
	}),
	ruleVolume: newPatternRule(ruleVolume, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchVolumePattern(ctx.candles, spec)
	}),
	ruleMarketProfile: newPatternRule(ruleMarketProfile, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchMarketProfilePattern(ctx.candles, spec)
	}),
	rulePointFigure: newPatternRule(rulePointFigure, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchPointFigurePattern(ctx.candles, spec)
	}),
	ruleTrendUp: newPatternRule(ruleTrendUp, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchTrend(ctx.candles, true, spec.Confidence, "uptrend structure is visible")
	}),
	ruleTrendDown: newPatternRule(ruleTrendDown, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchTrend(ctx.candles, false, spec.Confidence, "downtrend structure is visible")
	}),
	ruleTrendSideways: newPatternRule(ruleTrendSideways, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchSideways(ctx.candles, spec.Confidence)
	}),
	ruleIndicator: newPatternRule(ruleIndicator, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchIndicatorPattern(ctx.input, spec)
	}),
	ruleFibonacci: newPatternRule(ruleFibonacci, func(ctx patternMatchContext, spec patternSpec) generatedMatch {
		return matchFibonacci(ctx.input, spec)
	}),
}
