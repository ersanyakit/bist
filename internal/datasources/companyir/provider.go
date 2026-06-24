package companyir

import (
	"context"

	"hissebot/internal/datasources"
	"hissebot/internal/domain/stocks"
)

type Provider struct {
	BaseURL string
}

func New(baseURL string) Provider {
	return Provider{BaseURL: baseURL}
}

func (p Provider) Info() datasources.ProviderInfo {
	return datasources.ProviderInfo{
		Name:         "company-investor-relations",
		SourceURL:    p.BaseURL,
		License:      "company-published investor relations material; archive and source URL required",
		Capabilities: []string{"annual_reports", "investor_presentations", "segment_data", "strategy_notes"},
	}
}

func (p Provider) GetStock(context.Context, string) (stocks.Stock, error) {
	return stocks.Stock{}, datasources.ErrNotConfigured
}

func (p Provider) GetReferenceData(context.Context, string) (stocks.ReferenceData, error) {
	return stocks.ReferenceData{}, datasources.ErrNotConfigured
}
