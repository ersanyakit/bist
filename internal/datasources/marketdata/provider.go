package marketdata

import (
	"context"
	"time"

	"hissebot/internal/datasources"
	domainmd "hissebot/internal/domain/marketdata"
)

type LicensedProvider struct {
	Name    string
	BaseURL string
	APIKey  string
}

func NewLicensedProvider(name string, baseURL string, apiKey string) LicensedProvider {
	return LicensedProvider{Name: name, BaseURL: baseURL, APIKey: apiKey}
}

func (p LicensedProvider) Info() datasources.ProviderInfo {
	name := p.Name
	if name == "" {
		name = "licensed-market-data"
	}
	return datasources.ProviderInfo{
		Name:         name,
		SourceURL:    p.BaseURL,
		License:      "licensed BIST market data feed required for live, tick, bid/ask, depth, broker distribution and redistribution",
		RequiresKey:  true,
		Capabilities: []string{"ohlcv", "latest_price", "index_ohlcv", "sector_index_ohlcv", "order_book_depth"},
	}
}

func (p LicensedProvider) GetOHLCV(context.Context, string, string, time.Time, time.Time) ([]domainmd.OHLCV, error) {
	return nil, datasources.ErrLicensedDataRequired
}

func (p LicensedProvider) GetLatestPrice(context.Context, string) (domainmd.PriceSnapshot, error) {
	return domainmd.PriceSnapshot{}, datasources.ErrLicensedDataRequired
}

func (p LicensedProvider) GetIndexOHLCV(context.Context, string, string, time.Time, time.Time) ([]domainmd.OHLCV, error) {
	return nil, datasources.ErrLicensedDataRequired
}

func (p LicensedProvider) GetSectorIndexOHLCV(context.Context, string, string, time.Time, time.Time) ([]domainmd.OHLCV, error) {
	return nil, datasources.ErrLicensedDataRequired
}

func (p LicensedProvider) GetOrderBook(context.Context, string) (domainmd.OrderBook, error) {
	return domainmd.OrderBook{}, datasources.ErrLicensedDataRequired
}
