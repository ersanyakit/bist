package macro

import "time"

type SeriesID string

const (
	SeriesPolicyRate           SeriesID = "policy_rate"
	SeriesBondYield            SeriesID = "bond_yield"
	SeriesUSDTRY               SeriesID = "usd_try"
	SeriesEURTRY               SeriesID = "eur_try"
	SeriesCPI                  SeriesID = "inflation_cpi"
	SeriesPPI                  SeriesID = "inflation_ppi"
	SeriesGDPGrowth            SeriesID = "gdp_growth"
	SeriesGDPPerCapitaUSD      SeriesID = "gdp_per_capita_usd"
	SeriesGDPPerCapitaTRY      SeriesID = "gdp_per_capita_try"
	SeriesGDPThousandTRY       SeriesID = "gdp_thousand_try"
	SeriesIndustrialProduction SeriesID = "industrial_production"
	SeriesUnemployment         SeriesID = "unemployment"
	SeriesConsumerConfidence   SeriesID = "consumer_confidence"
	SeriesPMI                  SeriesID = "pmi"
)

type Frequency string

const (
	FrequencyDaily     Frequency = "daily"
	FrequencyMonthly   Frequency = "monthly"
	FrequencyQuarterly Frequency = "quarterly"
	FrequencyAnnual    Frequency = "annual"
)

type SourceMeta struct {
	Source      string    `json:"source"`
	SourceURL   string    `json:"source_url,omitempty"`
	License     string    `json:"license,omitempty"`
	DataVersion string    `json:"data_version,omitempty"`
	AsOf        time.Time `json:"as_of"`
	IngestedAt  time.Time `json:"ingested_at"`
}

type Observation struct {
	SeriesID SeriesID   `json:"series_id"`
	Date     time.Time  `json:"date"`
	Value    float64    `json:"value"`
	Unit     string     `json:"unit"`
	Revised  bool       `json:"revised,omitempty"`
	Meta     SourceMeta `json:"meta"`
}

type Series struct {
	ID           SeriesID      `json:"id"`
	Name         string        `json:"name"`
	Frequency    Frequency     `json:"frequency"`
	Unit         string        `json:"unit"`
	Observations []Observation `json:"observations"`
	Meta         SourceMeta    `json:"meta"`
}

type Point struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
	Unit  string    `json:"unit"`
}

type Revision struct {
	SeriesID  SeriesID   `json:"series_id"`
	Date      time.Time  `json:"date"`
	OldValue  float64    `json:"old_value"`
	NewValue  float64    `json:"new_value"`
	RevisedAt time.Time  `json:"revised_at"`
	Meta      SourceMeta `json:"meta"`
}
