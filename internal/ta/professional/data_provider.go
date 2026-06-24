package professional

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"hissebot/internal/domain"
)

type symbolDataProvider interface {
	companyProfileProvider
	financialDataProvider
	disclosureProvider
	documentEvidenceProvider
}

type companyProfileProvider interface {
	companyProfile(input SymbolInput) CompanyProfile
}

type financialDataProvider interface {
	loadFinancials(symbol string) (financialFile, bool)
	loadStatementVersionStore(symbol string) (domain.FinancialStatementVersionStore, bool)
	loadFinancialRatioHistory(symbol string) (financialRatioHistory, bool)
}

type disclosureProvider interface {
	buildDisclosureReview(symbol string) DisclosureReview
}

type documentEvidenceProvider interface {
	analyzeKAPPDFIngest(symbol string) KAPPDFIngestSummary
	analyzeKAPAssetInventory(symbol string) KAPAssetInventorySummary
}

// financialRatioHistory holds pre-calculated ratios from bilanco_hesaplari.json.
// Structure: ratioName → year → {Q1, Q2, Q3, Q4}
type financialRatioHistory map[string]map[string]map[string]*float64

type fileSymbolDataProvider struct {
	equitiesDir string
}

type financialCandidatePath struct {
	Path     string
	Fallback bool
}

func newSymbolDataProvider(input SymbolInput) symbolDataProvider {
	return fileSymbolDataProvider{equitiesDir: input.EquitiesDir}
}

func (p fileSymbolDataProvider) companyProfile(input SymbolInput) CompanyProfile {
	return companyProfile(input)
}

func (p fileSymbolDataProvider) loadFinancials(symbol string) (financialFile, bool) {
	return loadFinancialsForSymbol(p.equitiesDir, symbol)
}

func (p fileSymbolDataProvider) loadStatementVersionStore(symbol string) (domain.FinancialStatementVersionStore, bool) {
	return loadStatementVersionStore(filepath.Join(p.equitiesDir, symbol, "financials", "statement_versions.json"))
}

func (p fileSymbolDataProvider) buildDisclosureReview(symbol string) DisclosureReview {
	return buildDisclosureReview(p.equitiesDir, symbol)
}

func (p fileSymbolDataProvider) analyzeKAPPDFIngest(symbol string) KAPPDFIngestSummary {
	return analyzeKAPPDFIngest(p.equitiesDir, symbol)
}

func (p fileSymbolDataProvider) analyzeKAPAssetInventory(symbol string) KAPAssetInventorySummary {
	return analyzeKAPAssetInventory(p.equitiesDir, symbol)
}

func (p fileSymbolDataProvider) loadFinancialRatioHistory(symbol string) (financialRatioHistory, bool) {
	symbolUpper := strings.ToUpper(strings.TrimSpace(symbol))
	if symbolUpper == "" {
		return nil, false
	}
	path := filepath.Join(p.equitiesDir, symbolUpper, "financials", "bilanco_hesaplari.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var history financialRatioHistory
	if err := json.Unmarshal(raw, &history); err != nil {
		return nil, false
	}
	return history, true
}

func loadFinancialsForSymbol(equitiesDir, symbol string) (financialFile, bool) {
	for _, candidate := range financialCandidatePaths(equitiesDir, symbol) {
		if fin, ok := loadFinancials(candidate.Path); ok {
			if candidate.Fallback && !financialFallbackEligible(fin) {
				continue
			}
			return fin, true
		}
	}
	return financialFile{}, false
}

func financialCandidatePaths(equitiesDir, symbol string) []financialCandidatePath {
	symbolUpper := strings.ToUpper(strings.TrimSpace(symbol))
	symbolLower := strings.ToLower(symbolUpper)
	if symbolUpper == "" {
		return nil
	}
	paths := []financialCandidatePath{
		{Path: filepath.Join(equitiesDir, symbolUpper, "financials", "bilanco.json")},
		{Path: filepath.Join(equitiesDir, symbolUpper, "financials", "kap_bilanco.json"), Fallback: true},
	}
	dataDir := filepath.Dir(filepath.Clean(equitiesDir))
	paths = append(paths,
		financialCandidatePath{Path: filepath.Join(dataDir, "processed", "by_ticker", symbolUpper, "kap_financials", "bilanco.json"), Fallback: true},
		financialCandidatePath{Path: filepath.Join(dataDir, "processed", "by_ticker", symbolLower, "kap_financials", "bilanco.json"), Fallback: true},
		financialCandidatePath{Path: filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolUpper, "kap_financials", "bilanco.json"), Fallback: true},
		financialCandidatePath{Path: filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolLower, "kap_financials", "bilanco.json"), Fallback: true},
		financialCandidatePath{Path: filepath.Join(dataDir, "processed", symbolLower, "kap_financials", "bilanco.json"), Fallback: true},
		financialCandidatePath{Path: filepath.Join("data", "processed", "by_ticker", symbolUpper, "kap_financials", "bilanco.json"), Fallback: true},
		financialCandidatePath{Path: filepath.Join("data", "processed", "by_ticker", symbolLower, "kap_financials", "bilanco.json"), Fallback: true},
	)
	out := make([]financialCandidatePath, 0, len(paths))
	seen := map[string]struct{}{}
	for _, candidate := range paths {
		if _, ok := seen[candidate.Path]; ok {
			continue
		}
		seen[candidate.Path] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func financialFallbackEligible(fin financialFile) bool {
	if len(fin.Data) == 0 {
		return false
	}
	quality := fin.Quality
	if !quality.FinanciallyConsistent {
		return false
	}
	if quality.PeriodCount <= 0 {
		return false
	}
	if len(quality.InvalidChronologyPeriods) > 0 || len(quality.UnsafeBacktestPeriods) > 0 {
		return false
	}
	if quality.PublishDateCoverage <= 0 && quality.AvailableAtCoverage <= 0 && !quality.BacktestSafe {
		return false
	}
	return true
}
