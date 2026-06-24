package disclosures

import "time"

type SourceMeta struct {
	Source      string    `json:"source"`
	SourceURL   string    `json:"source_url,omitempty"`
	DataVersion string    `json:"data_version,omitempty"`
	AsOf        time.Time `json:"as_of"`
	IngestedAt  time.Time `json:"ingested_at"`
}

type Disclosure struct {
	ID             string     `json:"id"`
	Symbol         string     `json:"symbol"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary,omitempty"`
	Body           string     `json:"body,omitempty"`
	Category       string     `json:"category,omitempty"`
	DisclosureType string     `json:"disclosure_type,omitempty"`
	PublishedAt    time.Time  `json:"published_at"`
	URL            string     `json:"url,omitempty"`
	Meta           SourceMeta `json:"meta"`
}

type MaterialEvent struct {
	ID           string     `json:"id"`
	Symbol       string     `json:"symbol"`
	EventType    string     `json:"event_type"`
	Title        string     `json:"title"`
	Impact       string     `json:"impact,omitempty"`
	PublishedAt  time.Time  `json:"published_at"`
	DisclosureID string     `json:"disclosure_id,omitempty"`
	Meta         SourceMeta `json:"meta"`
}
