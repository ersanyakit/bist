package bist

import (
	"context"
	"time"

	"hissebot/internal/datasources"
	"hissebot/internal/domain/marketdata"
	"hissebot/internal/domain/stocks"
)

type Provider struct {
	BaseURL string
	APIKey  string
}

func New(baseURL string, apiKey string) Provider {
	return Provider{BaseURL: baseURL, APIKey: apiKey}
}

func (p Provider) Info() datasources.ProviderInfo {
	return datasources.ProviderInfo{
		Name:         "Borsa İstanbul",
		SourceURL:    firstNonEmpty(p.BaseURL, "https://www.borsaistanbul.com"),
		License:      "reference data may be public; live prices, depth and redistribution require BIST/professional data license",
		RequiresKey:  p.APIKey != "",
		Capabilities: []string{"reference_data", "indexes", "corporate_actions", "licensed_live_market_data"},
	}
}

func (p Provider) GetCorporateActions(context.Context, string, time.Time, time.Time) ([]marketdata.CorporateAction, error) {
	return nil, datasources.ErrNotConfigured
}

func (p Provider) GetStock(context.Context, string) (stocks.Stock, error) {
	return stocks.Stock{}, datasources.ErrNotConfigured
}

func (p Provider) GetReferenceData(context.Context, string) (stocks.ReferenceData, error) {
	return stocks.ReferenceData{}, datasources.ErrNotConfigured
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
