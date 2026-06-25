package analysisproviders

import (
	"context"
	"time"

	domainpricequality "hissebot/internal/domain/pricequality"
	"hissebot/internal/services/matriksformations"
	servicepricequality "hissebot/internal/services/pricequality"
)

type PriceQualityProvider struct {
	EquitiesDir          string
	StaleAfter           time.Duration
	ConflictToleranceBps float64
	Now                  func() time.Time
}

func (p PriceQualityProvider) InspectSymbol(ctx context.Context, symbol string) (*domainpricequality.SymbolReport, error) {
	report, err := servicepricequality.InspectSymbol(ctx, symbol, servicepricequality.Options{
		EquitiesDir:          p.EquitiesDir,
		StaleAfter:           p.StaleAfter,
		ConflictToleranceBps: p.ConflictToleranceBps,
		Now:                  p.Now,
	})
	if err != nil {
		return nil, err
	}
	converted := convertPriceQualityReport(report)
	return &converted, nil
}

type MatriksFormationsProvider struct {
	EquitiesDir string
}

func (p MatriksFormationsProvider) LoadTickerSnapshot(ctx context.Context, symbol string) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return matriksformations.LoadTickerSnapshot(p.EquitiesDir, symbol)
}

func convertPriceQualityReport(report servicepricequality.SymbolReport) domainpricequality.SymbolReport {
	out := domainpricequality.SymbolReport{
		Symbol:                report.Symbol,
		Status:                report.Status,
		ReadyForDecision:      report.ReadyForDecision,
		ReadyForVerifiedClose: report.ReadyForVerifiedClose,
		LatestTradingDate:     report.LatestTradingDate,
		Candidates:            convertCloseCandidates(report.Candidates),
		Conflict:              report.Conflict,
		ConflictBps:           report.ConflictBps,
		Stale:                 report.Stale,
		MissingFields:         append([]string{}, report.MissingFields...),
		BlockingReasons:       append([]string{}, report.BlockingReasons...),
	}
	if report.SelectedClose != nil {
		selected := convertCloseCandidate(*report.SelectedClose)
		out.SelectedClose = &selected
	}
	return out
}

func convertCloseCandidates(candidates []servicepricequality.CloseCandidate) []domainpricequality.CloseCandidate {
	out := make([]domainpricequality.CloseCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, convertCloseCandidate(candidate))
	}
	return out
}

func convertCloseCandidate(candidate servicepricequality.CloseCandidate) domainpricequality.CloseCandidate {
	return domainpricequality.CloseCandidate{
		Source:      candidate.Source,
		SourceType:  candidate.SourceType,
		Close:       candidate.Close,
		Timestamp:   cloneTimePtr(candidate.Timestamp),
		TradingDate: candidate.TradingDate,
		FetchedAt:   cloneTimePtr(candidate.FetchedAt),
		Stale:       candidate.Stale,
		Final:       candidate.Final,
		Official:    candidate.Official,
		Path:        candidate.Path,
		Field:       candidate.Field,
	}
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
