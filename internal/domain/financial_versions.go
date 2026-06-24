package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

const FinancialStatementStoreVersion = "financial-statement-store-v1"

type FinancialStatementVersionStore struct {
	Ticker        string                      `json:"ticker"`
	SchemaVersion string                      `json:"schema_version"`
	UpdatedAt     time.Time                   `json:"updated_at"`
	Latest        map[string]string           `json:"latest"`
	Versions      []FinancialStatementVersion `json:"versions"`
}

type FinancialStatementVersion struct {
	ID                 string             `json:"id"`
	Ticker             string             `json:"ticker"`
	PeriodKey          string             `json:"period_key"`
	FiscalYear         int                `json:"fiscal_year"`
	FiscalQuarter      int                `json:"fiscal_quarter"`
	Revision           int                `json:"revision"`
	IsRestatement      bool               `json:"is_restatement"`
	RestatesVersionID  string             `json:"restates_version_id,omitempty"`
	Source             string             `json:"source,omitempty"`
	FinancialGroup     string             `json:"financial_group,omitempty"`
	Currency           string             `json:"currency,omitempty"`
	FetchedAt          time.Time          `json:"fetched_at,omitempty"`
	PublishDate        *time.Time         `json:"publish_date,omitempty"`
	AvailableAt        *time.Time         `json:"available_at,omitempty"`
	AvailabilitySource string             `json:"availability_source,omitempty"`
	SourceDocumentID   string             `json:"source_document_id,omitempty"`
	FactDigest         string             `json:"fact_digest"`
	Facts              map[string]float64 `json:"facts"`
	Lineage            []DataLineageEvent `json:"lineage,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

func BuildStatementVersions(info *BilancoInfo, now time.Time) []FinancialStatementVersion {
	if info == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	NormalizeBilancoInfo(info, info.Ticker)
	byPeriod := map[string]FinancialStatementVersion{}
	for code, field := range info.Data {
		for yearText, values := range field.Years {
			year, err := strconv.Atoi(yearText)
			if err != nil || year <= 0 {
				continue
			}
			for index, value := range values {
				if value == nil {
					continue
				}
				quarter := FiscalQuarterFromIndex(index)
				key := FinancialPeriodKey(year, quarter)
				if key == "" {
					continue
				}
				version := byPeriod[key]
				if version.Facts == nil {
					period := info.Periods[key]
					version = FinancialStatementVersion{
						Ticker:             info.Ticker,
						PeriodKey:          key,
						FiscalYear:         year,
						FiscalQuarter:      quarter,
						Source:             firstNonEmpty(period.Source, info.Source),
						FinancialGroup:     firstNonEmpty(period.FinancialGroup, info.FinancialGroup),
						Currency:           firstNonEmpty(period.Currency, info.Currency),
						FetchedAt:          firstNonZeroTime(period.FetchedAt, info.FetchedAt),
						PublishDate:        period.PublishDate,
						AvailableAt:        EffectiveFinancialAvailableAt(period),
						AvailabilitySource: period.AvailabilitySource,
						SourceDocumentID:   period.SourceDocumentID,
						Facts:              map[string]float64{},
						Lineage:            append([]DataLineageEvent{}, info.Lineage...),
						CreatedAt:          now,
					}
				}
				version.Facts[code] = *value
				byPeriod[key] = version
			}
		}
	}
	keys := make([]string, 0, len(byPeriod))
	for key := range byPeriod {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]FinancialStatementVersion, 0, len(keys))
	for _, key := range keys {
		version := byPeriod[key]
		version.FactDigest = digestFacts(version.Facts)
		out = append(out, version)
	}
	return out
}

func UpsertStatementVersions(store FinancialStatementVersionStore, ticker string, incoming []FinancialStatementVersion, now time.Time) FinancialStatementVersionStore {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if store.Ticker == "" {
		store.Ticker = ticker
	}
	if store.SchemaVersion == "" {
		store.SchemaVersion = FinancialStatementStoreVersion
	}
	if store.Latest == nil {
		store.Latest = map[string]string{}
	}
	for _, version := range incoming {
		if version.Ticker == "" {
			version.Ticker = ticker
		}
		latestID := store.Latest[version.PeriodKey]
		latest, ok := findStatementVersion(store.Versions, latestID)
		if ok && latest.FactDigest == version.FactDigest {
			updateStatementVersionMetadata(store.Versions, latestID, version)
			continue
		}
		version.Revision = 1
		if ok {
			version.Revision = latest.Revision + 1
			version.IsRestatement = true
			version.RestatesVersionID = latest.ID
		}
		version.CreatedAt = now
		version.ID = statementVersionID(version)
		store.Versions = append(store.Versions, version)
		store.Latest[version.PeriodKey] = version.ID
	}
	store.UpdatedAt = now
	return store
}

func updateStatementVersionMetadata(versions []FinancialStatementVersion, id string, incoming FinancialStatementVersion) {
	if id == "" {
		return
	}
	for i := range versions {
		if versions[i].ID != id {
			continue
		}
		if incoming.Source != "" {
			versions[i].Source = incoming.Source
		}
		if incoming.FinancialGroup != "" {
			versions[i].FinancialGroup = incoming.FinancialGroup
		}
		if incoming.Currency != "" {
			versions[i].Currency = incoming.Currency
		}
		if !incoming.FetchedAt.IsZero() {
			versions[i].FetchedAt = incoming.FetchedAt
		}
		if incoming.PublishDate != nil {
			versions[i].PublishDate = incoming.PublishDate
		}
		if incoming.AvailableAt != nil {
			versions[i].AvailableAt = incoming.AvailableAt
		}
		if incoming.AvailabilitySource != "" {
			versions[i].AvailabilitySource = incoming.AvailabilitySource
		}
		if incoming.SourceDocumentID != "" {
			versions[i].SourceDocumentID = incoming.SourceDocumentID
		}
		if len(incoming.Lineage) > 0 {
			versions[i].Lineage = append([]DataLineageEvent{}, incoming.Lineage...)
		}
		return
	}
}

func findStatementVersion(versions []FinancialStatementVersion, id string) (FinancialStatementVersion, bool) {
	if id == "" {
		return FinancialStatementVersion{}, false
	}
	for _, version := range versions {
		if version.ID == id {
			return version, true
		}
	}
	return FinancialStatementVersion{}, false
}

func statementVersionID(version FinancialStatementVersion) string {
	raw := fmt.Sprintf("%s|%s|%d|%s", version.Ticker, version.PeriodKey, version.Revision, version.FactDigest)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:24]
}

func digestFacts(facts map[string]float64) string {
	keys := make([]string, 0, len(facts))
	for key := range facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]float64, len(keys))
	for _, key := range keys {
		ordered[key] = facts[key]
	}
	raw, _ := json.Marshal(ordered)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}
