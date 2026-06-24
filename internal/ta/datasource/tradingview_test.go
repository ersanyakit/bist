package datasource

import (
	"testing"

	"hissebot/internal/ta/ohlcv"
)

func TestTradingViewInstrumentDetectsCrypto(t *testing.T) {
	got := tradingViewInstrument("BTC", "BIST")
	if got.Symbol != "BTCUSDT" || got.Exchange != "BINANCE" || got.Currency != "USDT" || got.AssetType != ohlcv.AssetTypeCrypto {
		t.Fatalf("BTC instrument = %+v", got)
	}

	got = tradingViewInstrument("COINBASE:BTCUSD", "BIST")
	if got.Symbol != "BTCUSD" || got.Exchange != "COINBASE" || got.Currency != "USD" || got.AssetType != ohlcv.AssetTypeCrypto {
		t.Fatalf("COINBASE BTC instrument = %+v", got)
	}
}

func TestTradingViewInstrumentDetectsGoldCommodity(t *testing.T) {
	got := tradingViewInstrument("XAU", "BIST")
	if got.Symbol != "XAUUSD" || got.Exchange != "OANDA" || got.Currency != "USD" || got.AssetType != ohlcv.AssetTypeCommodity {
		t.Fatalf("XAU instrument = %+v", got)
	}

	got = tradingViewInstrument("OANDA:XAUUSD", "BIST")
	if got.Symbol != "XAUUSD" || got.Exchange != "OANDA" || got.Currency != "USD" || got.AssetType != ohlcv.AssetTypeCommodity {
		t.Fatalf("OANDA XAUUSD instrument = %+v", got)
	}
}

func TestTradingViewInstrumentKeepsBISTEquity(t *testing.T) {
	got := tradingViewInstrument("ASELS", "BIST")
	if got.Symbol != "ASELS" || got.Exchange != "BIST" || got.Currency != "TRY" || got.AssetType != ohlcv.AssetTypeEquity {
		t.Fatalf("ASELS instrument = %+v", got)
	}
}
