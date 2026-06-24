package kapingest

import (
	"errors"
	"fmt"
	"strings"
)

var allowedDocumentClasses = map[string]bool{
	DocumentInterimActivityReport: true,
	DocumentAnnualReport:          true,
	DocumentFinancialStatement:    true,
	DocumentValuationReport:       true,
	DocumentMaterialEvent:         true,
	DocumentGeneralAssembly:       true,
	DocumentDividend:              true,
	DocumentCapitalIncrease:       true,
	DocumentShareBuyback:          true,
	DocumentBoardDecision:         true,
	DocumentAuditReport:           true,
	DocumentCorporateGovernance:   true,
	DocumentUnknown:               true,
}

var allowedImpactGeneral = map[string]bool{
	"positive":  true,
	"negative":  true,
	"neutral":   true,
	"mixed":     true,
	"uncertain": true,
}

var allowedImpactShortTerm = map[string]bool{
	"positive":  true,
	"negative":  true,
	"neutral":   true,
	"uncertain": true,
}

func ValidateKAPEvent(event *KAPEvent) ([]string, error) {
	if event == nil {
		return nil, errors.New("nil kap event")
	}
	warnings := []string{}
	event.Ticker = strings.ToUpper(strings.TrimSpace(event.Ticker))
	if event.Ticker == "" {
		warnings = append(warnings, "ticker_missing")
	}
	event.FilePath = strings.TrimSpace(event.FilePath)
	if event.FilePath == "" {
		return warnings, errors.New("file_path is required")
	}
	event.SHA256 = strings.TrimSpace(event.SHA256)
	if event.SHA256 == "" {
		return warnings, errors.New("sha256 is required")
	}
	event.DocumentClass = strings.TrimSpace(event.DocumentClass)
	if event.DocumentClass == "" {
		event.DocumentClass = DocumentUnknown
	}
	if !allowedDocumentClasses[event.DocumentClass] {
		return warnings, fmt.Errorf("invalid document_class %q", event.DocumentClass)
	}
	event.FinancialProfile = strings.TrimSpace(event.FinancialProfile)
	if event.FinancialProfile == "" {
		event.FinancialProfile = "unknown"
	}
	event.EventCategory = strings.TrimSpace(event.EventCategory)
	if event.EventCategory == "" {
		event.EventCategory = event.DocumentClass
	}
	if event.Impact.Fundamental == "" {
		event.Impact.Fundamental = "uncertain"
	}
	if event.Impact.ShortTermPrice == "" {
		event.Impact.ShortTermPrice = "uncertain"
	}
	if event.Impact.LongTerm == "" {
		event.Impact.LongTerm = "uncertain"
	}
	if !allowedImpactGeneral[event.Impact.Fundamental] {
		return warnings, fmt.Errorf("invalid impact.fundamental %q", event.Impact.Fundamental)
	}
	if !allowedImpactShortTerm[event.Impact.ShortTermPrice] {
		return warnings, fmt.Errorf("invalid impact.short_term_price %q", event.Impact.ShortTermPrice)
	}
	if !allowedImpactGeneral[event.Impact.LongTerm] {
		return warnings, fmt.Errorf("invalid impact.long_term %q", event.Impact.LongTerm)
	}
	if event.Impact.Confidence < 0 || event.Impact.Confidence > 1 {
		return warnings, fmt.Errorf("impact.confidence must be between 0 and 1: %.4f", event.Impact.Confidence)
	}
	if event.FinancialHighlights == nil {
		event.FinancialHighlights = map[string]any{}
	}
	if event.PortfolioHighlights == nil {
		event.PortfolioHighlights = map[string]any{}
	}
	return warnings, nil
}
