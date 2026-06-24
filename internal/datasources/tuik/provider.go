package tuik

import (
	"context"
	"time"

	"hissebot/internal/datasources"
	domainmacro "hissebot/internal/domain/macro"
	tuiksvc "hissebot/internal/services/tuik"
)

type Provider struct {
	BaseURL string
	Years   int
	Timeout time.Duration
}

func New(baseURL string, years int, timeout time.Duration) Provider {
	return Provider{BaseURL: baseURL, Years: years, Timeout: timeout}
}

func (p Provider) Info() datasources.ProviderInfo {
	return datasources.ProviderInfo{
		Name:         "TÜİK",
		SourceURL:    firstNonEmpty(p.BaseURL, "https://cip.tuik.gov.tr/"),
		License:      "official public statistics source; revisions must be versioned",
		Capabilities: []string{"gdp", "inflation", "industrial_production", "unemployment", "confidence"},
	}
}

func (p Provider) GetGDPGrowth(ctx context.Context, from time.Time, to time.Time) ([]domainmacro.Observation, error) {
	dataset, err := tuiksvc.FetchGDPDataset(ctx, tuiksvc.GDPOptions{Years: p.Years, Timeout: p.Timeout, BaseURL: p.BaseURL})
	if err != nil {
		return nil, err
	}
	points := dataset.Points
	out := make([]domainmacro.Observation, 0, len(points))
	for i := 0; i < len(points)-1; i++ {
		current := points[i]
		previous := points[i+1]
		if previous.GDPThousandTRY == 0 {
			continue
		}
		date := time.Date(current.Year, 12, 31, 0, 0, 0, 0, time.UTC)
		if !inRange(date, from, to) {
			continue
		}
		out = append(out, domainmacro.Observation{
			SeriesID: domainmacro.SeriesGDPGrowth,
			Date:     date,
			Value:    (current.GDPThousandTRY - previous.GDPThousandTRY) / previous.GDPThousandTRY * 100,
			Unit:     "pct_yoy",
			Meta: domainmacro.SourceMeta{
				Source:      dataset.Source,
				SourceURL:   dataset.SourceURL,
				DataVersion: dataset.FetchedAt,
				AsOf:        date,
				IngestedAt:  time.Now().UTC(),
			},
		})
	}
	return out, nil
}

func (p Provider) GetInflation(context.Context, string, time.Time, time.Time) ([]domainmacro.Observation, error) {
	return nil, datasources.ErrNotConfigured
}

func (p Provider) GetIndustrialProduction(context.Context, time.Time, time.Time) ([]domainmacro.Observation, error) {
	return nil, datasources.ErrNotConfigured
}

func (p Provider) GetPolicyRate(context.Context, time.Time, time.Time) ([]domainmacro.Observation, error) {
	return nil, datasources.ErrUnsupportedCapability
}

func (p Provider) GetFXRate(context.Context, string, time.Time, time.Time) ([]domainmacro.Observation, error) {
	return nil, datasources.ErrUnsupportedCapability
}

func inRange(ts time.Time, from time.Time, to time.Time) bool {
	if !from.IsZero() && ts.Before(from) {
		return false
	}
	if !to.IsZero() && ts.After(to) {
		return false
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
