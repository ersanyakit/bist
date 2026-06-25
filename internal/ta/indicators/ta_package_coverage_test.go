package indicators

import (
	"context"
	"math"
	"testing"
)

func TestPythonTAPackageIndicatorCoverage(t *testing.T) {
	candles := indicatorTestCandles(260)
	cls := closes(candles)
	typ := typicalPrices(candles)
	vol := volumes(candles)
	additional := AdditionalIndicators(candles)

	checkFinite := func(name string, values ...float64) {
		t.Helper()
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Fatalf("%s returned invalid value %.8f", name, value)
			}
		}
	}
	checkAdditional := func(name string) {
		t.Helper()
		value, ok := additional[name]
		if !ok {
			t.Fatalf("missing AdditionalIndicators[%q]", name)
		}
		checkFinite(name, value)
	}

	stochK, stochD := StochasticOscillator(candles, 14, 3)
	stochRSIK, stochRSID := StochasticRSI(cls, 14, 14, 3, 3)
	macd, macdSignal, macdHist := MACD(cls, 12, 26, 9)
	upper, middle, lower := BollingerBands(cls, 20, 2)
	keltnerUpper, keltnerMiddle, keltnerLower := Keltner(candles, 20, 10, 2)
	donchianUpper, donchianLower := Donchian(candles, 20)
	tenkan, kijun, senkouA, senkouB, chikou, cloudTrend, kumoTwist, tkCross, breakout := Ichimoku(candles, 9, 26, 52)
	aroonUp, aroonDown := Aroon(candles, 25)
	vortexPlus, vortexMinus := Vortex(candles, 14)
	pvo, pvoSignal, pvoHist := PVOLines(vol, 12, 26, 9)

	checkFinite("calculate_rsi", RSI(cls, 14))
	checkFinite("calculate_stochastic", stochK, stochD)
	checkFinite("calculate_stoch_rsi", stochRSIK, stochRSID)
	checkFinite("calculate_williams_r", WilliamsR(candles, 14))
	checkFinite("calculate_awesome_oscillator", AwesomeOscillator(candles))
	checkFinite("calculate_kama", KAMA(cls, 10, 2, 30))
	checkFinite("calculate_roc", ROC(cls, 12))
	checkFinite("calculate_tsi", TrueStrengthIndex(cls, 25, 13))
	checkFinite("calculate_ultimate_oscillator", UltimateOscillator(candles))
	checkFinite("calculate_ppo", PPO(cls, 12, 26))
	checkFinite("calculate_pvo", pvo, pvoSignal, pvoHist)

	checkFinite("calculate_adi", AccumulationDistribution(candles))
	checkFinite("calculate_obv", OBV(candles))
	checkFinite("calculate_cmf", ChaikinMoneyFlow(candles, 20))
	checkFinite("calculate_force_index", ForceIndex(candles, 13))
	checkFinite("calculate_eom", EaseOfMovement(candles, 14))
	checkFinite("calculate_vpt", VolumePriceTrend(candles))
	checkFinite("calculate_nvi", NegativeVolumeIndex(candles))
	checkFinite("calculate_vwap", VWAP(candles))
	checkFinite("calculate_mfi", MFI(candles, 14))

	checkFinite("calculate_atr", ATR(candles, 14))
	checkFinite("calculate_bollinger_bands", upper, middle, lower)
	checkFinite("calculate_keltner_channel", keltnerUpper, keltnerMiddle, keltnerLower)
	checkFinite("calculate_donchian_channel", donchianUpper, donchianLower)
	checkFinite("calculate_ulcer_index", UlcerIndex(cls, 14))

	checkFinite("calculate_sma", SMA(cls, 20))
	checkFinite("calculate_ema", EMA(cls, 20))
	checkFinite("calculate_wma", WMA(cls, 20))
	checkFinite("calculate_macd", macd, macdSignal, macdHist)
	checkFinite("calculate_trix", TRIX(cls, 15))
	checkFinite("calculate_mass_index", MassIndex(candles, 25))
	checkFinite("calculate_ichimoku", tenkan, kijun, senkouA, senkouB, chikou, cloudTrend, kumoTwist, tkCross, breakout)
	checkAdditional("Know Sure Thing")
	checkFinite("calculate_dpo", DPO(cls, 20))
	checkFinite("calculate_cci", CCI(typ, 20))
	checkFinite("calculate_adx", ADX(candles, 14))
	checkFinite("calculate_vortex", vortexPlus, vortexMinus)
	checkFinite("calculate_psar", ParabolicSAR(candles))
	checkFinite("calculate_stc", SchaffTrendCycle(cls))
	checkFinite("calculate_aroon", aroonUp, aroonDown)

	checkFinite("calculate_daily_return", DailyReturn(cls))
	checkFinite("calculate_daily_log_return", DailyLogReturn(cls))
	checkFinite("calculate_cumulative_return", CumulativeReturn(cls))

	checkAdditional("Percentage Volume Oscillator")
	checkAdditional("PVO Signal Line")
	checkAdditional("PVO Histogram")
}

func TestPythonTAPackageCatalogNamesAreRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, name := range RegisteredIndicatorNames() {
		names[name] = true
	}
	for _, name := range []string{
		"PVO",
		"Percentage Volume Oscillator",
		"Daily Return",
		"Daily Log Return",
		"Cumulative Return",
	} {
		if !names[name] {
			t.Fatalf("registered indicator names missing %q", name)
		}
	}
}

func TestIndicatorInvalidPeriodsDoNotPanic(t *testing.T) {
	candles := indicatorTestCandles(40)
	cls := closes(candles)
	typ := typicalPrices(candles)
	vol := volumes(candles)

	checkNoPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%s panicked: %v", name, r)
			}
		}()
		fn()
	}

	checkNoPanic("MACD", func() { MACD(cls, 0, 26, 9) })
	checkNoPanic("BollingerBands", func() { BollingerBands(cls, 0, 2) })
	checkNoPanic("StochasticRSI", func() { StochasticRSI(cls, 14, 0, 3, 3) })
	checkNoPanic("StochasticOscillator", func() { StochasticOscillator(candles, 0, 3) })
	checkNoPanic("CCI", func() { CCI(typ, 0) })
	checkNoPanic("WilliamsR", func() { WilliamsR(candles, 0) })
	checkNoPanic("SupertrendLastTwo", func() { SupertrendLastTwo(candles, 0, 3) })
	checkNoPanic("Donchian", func() { Donchian(candles, 0) })
	checkNoPanic("Keltner", func() { Keltner(candles, 0, 10, 2) })
	checkNoPanic("ChaikinMoneyFlow", func() { ChaikinMoneyFlow(candles, 0) })
	checkNoPanic("Ichimoku", func() { Ichimoku(candles, 0, 26, 52) })
	checkNoPanic("KAMA", func() { KAMA(cls, 0, 2, 30) })
	checkNoPanic("DEMA", func() { DEMA(cls, 0) })
	checkNoPanic("TEMA", func() { TEMA(cls, 0) })
	checkNoPanic("TRIX", func() { TRIX(cls, 0) })
	checkNoPanic("Momentum", func() { Momentum(cls, 0) })
	checkNoPanic("TrueStrengthIndex", func() { TrueStrengthIndex(cls, 0, 13) })
	checkNoPanic("DPO", func() { DPO(cls, 0) })
	checkNoPanic("PPO", func() { PPO(cls, 0, 26) })
	checkNoPanic("PVO", func() { PVO(vol, 0, 26) })
	checkNoPanic("PVOLines", func() { PVOLines(vol, 12, 0, 9) })
	checkNoPanic("StochasticMomentumIndex", func() { StochasticMomentumIndex(candles, 0, 3) })
	checkNoPanic("MassIndex", func() { MassIndex(candles, 0) })
	checkNoPanic("Vortex", func() { Vortex(candles, 0) })
	checkNoPanic("ChandeKrollStop", func() { ChandeKrollStop(candles, 0, 9, 1.5) })
	checkNoPanic("ChandeMomentumOscillator", func() { ChandeMomentumOscillator(cls, 0) })
	checkNoPanic("ElderRay", func() { ElderRay(candles, 0) })
	checkNoPanic("FisherTransform", func() { FisherTransform(candles, 0) })
	checkNoPanic("HistoricalVolatility", func() { HistoricalVolatility(cls, 0) })
	checkNoPanic("ChaikinVolatility", func() { ChaikinVolatility(candles, 0, 10) })
	checkNoPanic("RelativeVolatilityIndex", func() { RelativeVolatilityIndex(cls, 0) })
}

func TestReturnIndicatorsUsePercentUnits(t *testing.T) {
	values := []float64{100, 110, 121}
	assertClose(t, "DailyReturn", DailyReturn(values), 10, 1e-12)
	assertClose(t, "DailyLogReturn", DailyLogReturn(values), 100*math.Log(121.0/110.0), 1e-12)
	assertClose(t, "CumulativeReturn", CumulativeReturn(values), 21, 1e-12)
}

func TestPVOIsComputedByScanner(t *testing.T) {
	candles := indicatorTestCandles(260)
	snapshot, err := Snapshot(candles)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	output, err := ScanIndicators(context.Background(), ScannerInput{
		Timeframe: "1D",
		Candles:   candles,
		Snapshot:  snapshot,
		LastClose: candles[len(candles)-1].EffectiveClose(),
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, name := range []string{"PVO", "Percentage Volume Oscillator"} {
		result, ok := findIndicator(output.Indicators, name)
		if !ok {
			t.Fatalf("scanner missing %q", name)
		}
		if !result.Computed {
			t.Fatalf("%s should be computed: %+v", name, result)
		}
	}
}
