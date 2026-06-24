package kapingest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hissebot/internal/kapfinance"
	"hissebot/internal/util"
)

const (
	CanonicalFinancialsDir = "kap_financials"
	CanonicalBilancoFile   = "bilanco.json"
)

type CanonicalFinancialsSummary struct {
	Tickers       int      `json:"tickers"`
	FilesWritten  int      `json:"files_written"`
	FactsRead     int      `json:"facts_read"`
	FactsAccepted int      `json:"facts_accepted"`
	FactsRejected int      `json:"facts_rejected"`
	OutputFiles   []string `json:"output_files,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

func BuildCanonicalFinancialsFromOutput(outputDir string, now func() time.Time) (CanonicalFinancialsSummary, error) {
	summary := CanonicalFinancialsSummary{}
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		summary.Warnings = append(summary.Warnings, "kap_canonical_output_dir_missing")
		return summary, nil
	}
	entries, err := os.ReadDir(filepath.Join(outputDir, "by_ticker"))
	if err != nil {
		if os.IsNotExist(err) {
			return summary, nil
		}
		return summary, err
	}
	generatedAt := time.Now().UTC()
	if now != nil {
		generatedAt = now().UTC()
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ticker := strings.ToUpper(strings.TrimSpace(entry.Name()))
		if ticker == "" {
			continue
		}
		path := filepath.Join(outputDir, "by_ticker", entry.Name(), FinancialFactsFile)
		facts, err := readCanonicalExtractedFinancialFacts(path)
		if err != nil {
			summary.Warnings = append(summary.Warnings, "kap_canonical_read_failed:"+path)
			continue
		}
		if len(facts) == 0 {
			continue
		}
		reconciledFacts := reconciledCanonicalFinancialFacts(facts)
		filteredByReconciliation := len(facts) - len(reconciledFacts)
		info, build := kapfinance.BuildBilanco(kapfinance.BuildOptions{
			Ticker:         ticker,
			Source:         "kapingest_document_intelligence",
			FinancialGroup: "kap_pdf_document_intelligence",
			GeneratedAt:    generatedAt,
		}, canonicalFinancialFacts(reconciledFacts))
		summary.Tickers++
		summary.FactsRead += len(facts)
		summary.FactsAccepted += build.FactsAccepted
		summary.FactsRejected += build.FactsRejected + filteredByReconciliation
		if filteredByReconciliation > 0 {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("kap_canonical_reconciled_competing_values_%d:%s", filteredByReconciliation, ticker))
		}
		summary.Warnings = append(summary.Warnings, build.Warnings...)
		if info == nil {
			continue
		}
		out := filepath.Join(outputDir, "by_ticker", entry.Name(), CanonicalFinancialsDir, CanonicalBilancoFile)
		if err := util.WriteJSON(out, info); err != nil {
			return summary, err
		}
		summary.FilesWritten++
		summary.OutputFiles = append(summary.OutputFiles, out)
	}
	return summary, nil
}

func readCanonicalExtractedFinancialFacts(path string) ([]ExtractedFinancialFact, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 8*1024*1024)
	out := []ExtractedFinancialFact{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var fact ExtractedFinancialFact
		if err := json.Unmarshal([]byte(line), &fact); err != nil {
			continue
		}
		out = append(out, fact)
	}
	return out, scanner.Err()
}

func canonicalFinancialFacts(facts []ExtractedFinancialFact) []kapfinance.Fact {
	out := make([]kapfinance.Fact, 0, len(facts))
	for _, fact := range facts {
		out = append(out, kapfinance.Fact{
			ID:                          fact.ID,
			Ticker:                      fact.Ticker,
			Period:                      stringPtrValue(fact.Period),
			DocumentDate:                stringPtrValue(fact.DocumentDate),
			LineItemOriginal:            fact.LineItemOriginal,
			LineItemNormalized:          fact.LineItemNormalized,
			StatementType:               fact.StatementType,
			Value:                       fact.Value,
			Currency:                    fact.Currency,
			Unit:                        fact.Unit,
			SourceFile:                  fact.SourceFile,
			SourceDocumentID:            fact.SHA256,
			SourcePage:                  fact.Source.Page,
			SourceTableID:               fact.Source.TableID,
			SourceText:                  firstNonEmptyAsset(fact.Source.Snippet, strings.Join(fact.Source.Cells, " ")),
			Confidence:                  fact.Confidence,
			ReviewRequired:              fact.ReviewRequired,
			CertificationStatus:         fact.Certification.Status,
			CertificationAnalysisUsable: fact.Certification.AnalysisUsable,
			CertificationScore:          fact.Certification.Score,
			ConsolidationScope:          fact.ConsolidationScope,
			AuditStatus:                 fact.AuditStatus,
			CreatedAt:                   parseCanonicalCreatedAt(fact.CreatedAt),
		})
	}
	return out
}

func readCanonicalFinancialFacts(path string) ([]kapfinance.Fact, error) {
	facts, err := readCanonicalExtractedFinancialFacts(path)
	if err != nil {
		return nil, err
	}
	return canonicalFinancialFacts(reconciledCanonicalFinancialFacts(facts)), nil
}

func reconciledCanonicalFinancialFacts(facts []ExtractedFinancialFact) []ExtractedFinancialFact {
	if len(facts) == 0 {
		return nil
	}
	resolved := ReconcileFinancialValueDifferences(facts)
	if len(resolved) == 0 {
		return facts
	}
	selected := map[string]KnowledgeContradictionResolution{}
	for _, item := range resolved {
		selected[item.Key] = item
	}
	out := make([]ExtractedFinancialFact, 0, len(facts))
	for _, fact := range facts {
		key := financialValueReconciliationKey(fact)
		item, hasResolution := selected[key]
		if hasResolution && !financialFactMatchesResolution(fact, item) {
			continue
		}
		out = append(out, fact)
	}
	return out
}

func financialValueReconciliationKey(fact ExtractedFinancialFact) string {
	return strings.Join([]string{
		strings.TrimSpace(stringPtrValue(fact.Period)),
		strings.TrimSpace(fact.StatementType),
		strings.TrimSpace(fact.LineItemNormalized),
		strings.TrimSpace(fact.Currency),
		strings.TrimSpace(fact.Unit),
	}, "|")
}

func financialFactMatchesResolution(fact ExtractedFinancialFact, item KnowledgeContradictionResolution) bool {
	base := math.Max(math.Abs(fact.Value), math.Abs(item.SelectedValue))
	tolerance := math.Max(1, base*0.000001)
	return math.Abs(fact.Value-item.SelectedValue) <= tolerance
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func parseCanonicalCreatedAt(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return time.Time{}
}
