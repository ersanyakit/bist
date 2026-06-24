package ohlcv

import "testing"

func TestCandleEffectiveCloseUsesAdjustedCloseOnlyWhenValid(t *testing.T) {
	candle := Candle{
		Close:         105,
		AdjustedClose: 100,
		IsAdjusted:    true,
	}
	if got := candle.EffectiveClose(); got != 100 {
		t.Fatalf("EffectiveClose() = %v, want adjusted close 100", got)
	}

	candle.AdjustedClose = 0
	if got := candle.EffectiveClose(); got != 105 {
		t.Fatalf("EffectiveClose() with zero adjusted close = %v, want raw close 105", got)
	}

	candle.AdjustedClose = 99
	candle.IsAdjusted = false
	if got := candle.EffectiveClose(); got != 105 {
		t.Fatalf("EffectiveClose() with IsAdjusted=false = %v, want raw close 105", got)
	}
}

func TestNormalizeSymbolRemovesExchangeAndSuffix(t *testing.T) {
	cases := map[string]string{
		" bist:eupwr.is ": "EUPWR",
		"EUPWR.IS":        "EUPWR",
		"eupwr":           "EUPWR",
	}

	for input, want := range cases {
		if got := NormalizeSymbol(input); got != want {
			t.Fatalf("NormalizeSymbol(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCryptoSymbolInferenceAndCanonicalPair(t *testing.T) {
	cases := []struct {
		input string
		pair  string
		quote string
	}{
		{input: "BTC", pair: "BTCUSDT", quote: "USDT"},
		{input: "BINANCE:BTCUSDT", pair: "BTCUSDT", quote: "USDT"},
		{input: "COINBASE:BTCUSD", pair: "BTCUSD", quote: "USD"},
		{input: "0GUSDT", pair: "0GUSDT", quote: "USDT"},
		{input: "BINANCE:1INCHUSDT", pair: "1INCHUSDT", quote: "USDT"},
	}
	for _, tc := range cases {
		pair, quote, ok := CanonicalCryptoPair(tc.input)
		if !ok {
			t.Fatalf("CanonicalCryptoPair(%q) not ok", tc.input)
		}
		if pair != tc.pair || quote != tc.quote {
			t.Fatalf("CanonicalCryptoPair(%q) = %s/%s, want %s/%s", tc.input, pair, quote, tc.pair, tc.quote)
		}
		if got := InferAssetTypeFromSymbol(tc.input); got != AssetTypeCrypto {
			t.Fatalf("InferAssetTypeFromSymbol(%q) = %s, want crypto", tc.input, got)
		}
	}
	if got := InferAssetTypeFromSymbol("ASELS"); got != AssetTypeEquity {
		t.Fatalf("InferAssetTypeFromSymbol(ASELS) = %s, want equity", got)
	}
}

func TestCommoditySymbolInference(t *testing.T) {
	cases := []struct {
		input    string
		symbol   string
		exchange string
		currency string
	}{
		{input: "XAU", symbol: "XAUUSD", exchange: "OANDA", currency: "USD"},
		{input: "OANDA:XAUUSD", symbol: "XAUUSD", exchange: "OANDA", currency: "USD"},
		{input: "XAUTRY", symbol: "XAUTRY", exchange: "FX_IDC", currency: "TRY"},
	}
	for _, tc := range cases {
		if got := InferAssetTypeFromSymbol(tc.input); got != AssetTypeCommodity {
			t.Fatalf("InferAssetTypeFromSymbol(%q) = %s, want commodity", tc.input, got)
		}
		instrument, ok := CanonicalCommodityInstrument(tc.input)
		if !ok {
			t.Fatalf("CanonicalCommodityInstrument(%q) not ok", tc.input)
		}
		if instrument.Symbol != tc.symbol || instrument.Exchange != tc.exchange || instrument.Currency != tc.currency {
			t.Fatalf("CanonicalCommodityInstrument(%q) = %+v", tc.input, instrument)
		}
		if _, _, ok := CanonicalCryptoPair(tc.input); ok {
			t.Fatalf("CanonicalCryptoPair(%q) should not classify gold as crypto", tc.input)
		}
	}
}

func TestSymbolPathKeyRemovesExchangeSeparators(t *testing.T) {
	if got := SymbolPathKey("BINANCE:BTCUSDT"); got != "BINANCE_BTCUSDT" {
		t.Fatalf("SymbolPathKey() = %s", got)
	}
}
