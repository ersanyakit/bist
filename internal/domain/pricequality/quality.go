package pricequality

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	StatusReadyForVerifiedClose = "ready_for_verified_close"
	StatusProvisionalLastPrice  = "provisional_last_price_only"
	StatusStaleOrConflicting    = "stale_or_conflicting"
	StatusMissingPriceData      = "missing_price_data"

	DefaultConflictToleranceBps = 10.0
)

type SymbolReport struct {
	Symbol                string           `json:"symbol"`
	Status                string           `json:"status"`
	ReadyForDecision      bool             `json:"ready_for_decision"`
	ReadyForVerifiedClose bool             `json:"ready_for_verified_close"`
	LatestTradingDate     string           `json:"latest_trading_date,omitempty"`
	SelectedClose         *CloseCandidate  `json:"selected_close,omitempty"`
	Candidates            []CloseCandidate `json:"candidates,omitempty"`
	Conflict              bool             `json:"conflict,omitempty"`
	ConflictBps           float64          `json:"conflict_bps,omitempty"`
	Stale                 bool             `json:"stale,omitempty"`
	MissingFields         []string         `json:"missing_fields,omitempty"`
	BlockingReasons       []string         `json:"blocking_reasons,omitempty"`
}

type CloseCandidate struct {
	Source      string     `json:"source"`
	SourceType  string     `json:"source_type"`
	Close       float64    `json:"close"`
	Timestamp   *time.Time `json:"timestamp,omitempty"`
	TradingDate string     `json:"trading_date,omitempty"`
	FetchedAt   *time.Time `json:"fetched_at,omitempty"`
	Stale       bool       `json:"stale,omitempty"`
	Final       bool       `json:"final,omitempty"`
	Official    bool       `json:"official,omitempty"`
	Path        string     `json:"path,omitempty"`
	Field       string     `json:"field,omitempty"`
}

func ReconcileWithAnalysisClose(report SymbolReport, close float64, candleTime time.Time, fetchedAt time.Time, source string, toleranceBps float64) SymbolReport {
	if close <= 0 || strings.TrimSpace(report.Symbol) == "" {
		return report
	}
	if toleranceBps <= 0 {
		toleranceBps = DefaultConflictToleranceBps
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "analysis_provider"
	}
	candidate := CloseCandidate{
		Source:      source,
		SourceType:  "analysis_ohlcv",
		Close:       close,
		TradingDate: tradingDate(candleTime),
		Field:       "timeframes.1D.candles[-1].close",
	}
	if !candleTime.IsZero() {
		ts := candleTime.UTC()
		candidate.Timestamp = &ts
	}
	if !fetchedAt.IsZero() {
		ts := fetchedAt.UTC()
		candidate.FetchedAt = &ts
	}
	candidates := append([]CloseCandidate{}, report.Candidates...)
	candidates = append(candidates, candidate)
	candidates = dedupeCandidates(candidates)
	sortCandidates(candidates)
	return finalizeSymbolReport(SymbolReport{
		Symbol:        report.Symbol,
		MissingFields: append([]string{}, report.MissingFields...),
	}, candidates, toleranceBps)
}

func ReconcileWithOfficialFinalClose(report SymbolReport, close float64, tradingDate string, sourceTimestamp time.Time, fetchedAt time.Time, source string, path string, toleranceBps float64) SymbolReport {
	if close <= 0 || strings.TrimSpace(report.Symbol) == "" {
		return report
	}
	if toleranceBps <= 0 {
		toleranceBps = DefaultConflictToleranceBps
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "official_close"
	}
	candidate := CloseCandidate{
		Source:      source,
		SourceType:  "official_final_close",
		Close:       close,
		TradingDate: strings.TrimSpace(tradingDate),
		Final:       true,
		Official:    true,
		Field:       "close",
	}
	if strings.TrimSpace(path) != "" {
		candidate.Path = filepath.Clean(path)
	}
	if !sourceTimestamp.IsZero() {
		ts := sourceTimestamp.UTC()
		candidate.Timestamp = &ts
		if candidate.TradingDate == "" {
			candidate.TradingDate = tradingDateFromTime(ts)
		}
	}
	if !fetchedAt.IsZero() {
		ts := fetchedAt.UTC()
		candidate.FetchedAt = &ts
	}
	candidates := append([]CloseCandidate{}, report.Candidates...)
	candidates = append(candidates, candidate)
	candidates = dedupeCandidates(candidates)
	sortCandidates(candidates)
	return finalizeSymbolReport(SymbolReport{
		Symbol:        report.Symbol,
		MissingFields: removeString(report.MissingFields, "official_final_close"),
	}, candidates, toleranceBps)
}

func finalizeSymbolReport(report SymbolReport, candidates []CloseCandidate, conflictToleranceBps float64) SymbolReport {
	report.Candidates = candidates
	if len(candidates) == 0 {
		report.Status = StatusMissingPriceData
		report.BlockingReasons = append(report.BlockingReasons, "no_local_price_candidate")
		return report
	}
	report.SelectedClose = selectClose(candidates)
	report.LatestTradingDate = selectedTradingDate(report.SelectedClose, candidates)
	report.Stale = report.SelectedClose == nil || report.SelectedClose.Stale
	report.Conflict, report.ConflictBps = conflictOnTradingDate(candidates, report.LatestTradingDate, conflictToleranceBps)
	if report.SelectedClose == nil || !report.SelectedClose.Official || !report.SelectedClose.Final {
		report.BlockingReasons = append(report.BlockingReasons, "official_final_close_missing")
	}
	if report.Stale {
		report.BlockingReasons = append(report.BlockingReasons, "stale_price_source_present")
	}
	if report.Conflict {
		report.BlockingReasons = append(report.BlockingReasons, "price_sources_conflict")
	}
	report.ReadyForDecision = report.SelectedClose != nil && !report.Stale && !report.Conflict
	switch {
	case report.ReadyForDecision && report.SelectedClose.Official && report.SelectedClose.Final:
		report.Status = StatusReadyForVerifiedClose
		report.ReadyForVerifiedClose = true
	case !report.ReadyForDecision:
		report.Status = StatusStaleOrConflicting
	default:
		report.Status = StatusProvisionalLastPrice
	}
	return report
}

func selectClose(candidates []CloseCandidate) *CloseCandidate {
	if len(candidates) == 0 {
		return nil
	}
	for _, candidate := range candidates {
		if candidate.Official && candidate.Final && !candidate.Stale {
			copy := candidate
			return &copy
		}
	}
	for _, candidate := range candidates {
		if !candidate.Stale {
			copy := candidate
			return &copy
		}
	}
	copy := candidates[0]
	return &copy
}

func sortCandidates(candidates []CloseCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		left := candidateTime(candidates[i])
		right := candidateTime(candidates[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		if candidates[i].Official != candidates[j].Official {
			return candidates[i].Official
		}
		if candidates[i].Final != candidates[j].Final {
			return candidates[i].Final
		}
		return candidates[i].SourceType < candidates[j].SourceType
	})
}

func candidateTime(candidate CloseCandidate) time.Time {
	if candidate.Timestamp != nil {
		return candidate.Timestamp.UTC()
	}
	if candidate.FetchedAt != nil {
		return candidate.FetchedAt.UTC()
	}
	return time.Time{}
}

func selectedTradingDate(selected *CloseCandidate, candidates []CloseCandidate) string {
	if selected != nil && selected.TradingDate != "" {
		return selected.TradingDate
	}
	for _, candidate := range candidates {
		if candidate.TradingDate != "" {
			return candidate.TradingDate
		}
	}
	return ""
}

func conflictOnTradingDate(candidates []CloseCandidate, date string, toleranceBps float64) (bool, float64) {
	if date == "" {
		return false, 0
	}
	values := []float64{}
	for _, candidate := range candidates {
		if candidate.TradingDate == date && candidate.Close > 0 {
			values = append(values, candidate.Close)
		}
	}
	if len(values) < 2 {
		return false, 0
	}
	sort.Float64s(values)
	minValue := values[0]
	maxValue := values[len(values)-1]
	if minValue <= 0 {
		return false, 0
	}
	bps := ((maxValue - minValue) / minValue) * 10_000
	return bps > toleranceBps, bps
}

func dedupeCandidates(candidates []CloseCandidate) []CloseCandidate {
	out := []CloseCandidate{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.Close <= 0 {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%.8f", candidate.SourceType, candidate.Path, candidate.Field, candidate.TradingDate, candidate.Close)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	return out
}

func tradingDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return value.UTC().Format("2006-01-02")
	}
	return value.In(loc).Format("2006-01-02")
}

func tradingDateFromTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func removeString(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == target {
			continue
		}
		out = append(out, value)
	}
	return out
}
