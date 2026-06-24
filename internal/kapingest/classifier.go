package kapingest

import (
	"strings"

	"hissebot/internal/util"
)

const (
	DocumentInterimActivityReport  = "interim_activity_report"
	DocumentAnnualReport           = "annual_report"
	DocumentActivityReport         = "activity_report"
	DocumentFinancialStatement     = "financial_statement"
	DocumentIndependentAuditReport = "independent_audit_report"
	DocumentValuationReport        = "valuation_report"
	DocumentMaterialEvent          = "material_event"
	DocumentMaterialDisclosure     = "material_disclosure"
	DocumentGeneralAssembly        = "general_assembly"
	DocumentArticlesOfAssociation  = "articles_of_association"
	DocumentDividend               = "dividend"
	DocumentDividendDistribution   = "dividend_distribution"
	DocumentCapitalIncrease        = "capital_increase"
	DocumentShareBuyback           = "share_buyback"
	DocumentBoardDecision          = "board_decision"
	DocumentAuditReport            = "audit_report"
	DocumentCorporateGovernance    = "corporate_governance"
	DocumentCreditRating           = "credit_rating"
	DocumentInvestorPresentation   = "investor_presentation"
	DocumentLegalCase              = "legal_case"
	DocumentTenderContract         = "tender_contract"
	DocumentInvestmentProject      = "investment_project"
	DocumentSustainabilityReport   = "sustainability_report"
	DocumentProspectus             = "prospectus"
	DocumentIssueDocument          = "issue_document"
	DocumentOther                  = "other"
	DocumentUnknown                = "unknown"
)

func ClassifyDocument(text string, fileName string) string {
	slug := util.SlugTR(fileName + " " + firstRunes(text, 20000))
	has := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(slug, util.SlugTR(value)) {
				return true
			}
		}
		return false
	}
	switch {
	case has("ara donem faaliyet raporu", "ara dönem faaliyet raporu", "1.donem faaliyet raporu", "q1 activity report", "q2 activity report", "q3 activity report"):
		return DocumentInterimActivityReport
	case has("yillik faaliyet raporu", "yıllık faaliyet raporu", "annual report") && !has("ara donem", "ara dönem"):
		return DocumentAnnualReport
	case has("faaliyet raporu", "activity report") && !has("ara donem", "ara dönem"):
		return DocumentActivityReport
	case has("finansal rapor", "finansal durum tablosu", "bilanço", "bilanco") && has("kar veya zarar", "gelir tablosu", "nakit akis", "nakit akış"):
		return DocumentFinancialStatement
	case has("bagimsiz denetim raporu", "bağımsız denetim raporu", "denetci raporu", "denetçi raporu", "independent auditor"):
		return DocumentIndependentAuditReport
	case has("gayrimenkul degerleme raporu", "gayrimenkul değerleme raporu", "ekspertiz degeri", "ekspertiz değeri", "degerleme raporu", "değerleme raporu"):
		return DocumentValuationReport
	case has("derecelendirme raporu", "kredi derecelendirme", "credit rating"):
		return DocumentCreditRating
	case has("yatirimci sunumu", "investor presentation", "sunum"):
		return DocumentInvestorPresentation
	case has("surdurulebilirlik raporu", "sürdürülebilirlik raporu", "sustainability report"):
		return DocumentSustainabilityReport
	case has("izahname", "prospektus", "prospectus"):
		return DocumentProspectus
	case has("ihrac belgesi", "ihraç belgesi", "borclanma araci ihraci", "borçlanma aracı ihracı"):
		return DocumentIssueDocument
	case has("kar payi dagitim", "kâr payı dağıtım", "temettu", "temettü"):
		return DocumentDividendDistribution
	case has("sermaye artirimi", "sermaye artırımı", "sermaye artisi", "sermaye artışı", "kayitli sermaye tavani", "kayıtlı sermaye tavanı", "sermaye tavani artisi", "sermaye tavanı artışı", "bedelsiz", "bedelli", "ruchan hakki", "rüçhan hakkı"):
		return DocumentCapitalIncrease
	case has("esas sozlesme", "esas sözleşme", "ana sozlesme", "ana sözleşme", "tadil metni"):
		return DocumentArticlesOfAssociation
	case has("pay geri alim", "pay geri alım", "geri alim programi", "geri alım programı"):
		return DocumentShareBuyback
	case has("bagimsiz uye aday listesi", "bağımsız üye aday listesi", "bagimsiz uye listesi", "bağımsız üye listesi", "bagimsiz uye secimi", "bağımsız üye seçimi", "bagimsiz aday listesi", "bağımsız aday listesi", "bagimsiz yonetim kurulu aday", "bağımsız yönetim kurulu aday"):
		return DocumentCorporateGovernance
	case has("genel kurul", "olagan genel kurul", "olağan genel kurul", "hazirun", "toplanti tutanagi", "toplantı tutanağı"):
		return DocumentGeneralAssembly
	case has("yonetim kurulu karari", "yönetim kurulu kararı", "board decision"):
		return DocumentBoardDecision
	case has("kurumsal yonetim", "kurumsal yönetim", "corporate governance"):
		return DocumentCorporateGovernance
	case has("dava", "hukuki surec", "hukuki süreç", "icra takibi"):
		return DocumentLegalCase
	case has("ihale", "sozlesme", "sözleşme", "tender", "contract"):
		return DocumentTenderContract
	case has("yatirim projesi", "kapasite artisi", "kapasite artışı", "tesvik belgesi", "teşvik belgesi"):
		return DocumentInvestmentProject
	case has("ozel durum aciklamasi", "özel durum açıklaması", "material event", "material disclosure"):
		return DocumentMaterialDisclosure
	default:
		return DocumentOther
	}
}

func NormalizeDocumentType(value string) string {
	switch strings.TrimSpace(value) {
	case DocumentDividend:
		return DocumentDividendDistribution
	case DocumentMaterialEvent:
		return DocumentMaterialDisclosure
	case DocumentAuditReport:
		return DocumentIndependentAuditReport
	case DocumentUnknown:
		return DocumentOther
	default:
		if strings.TrimSpace(value) == "" {
			return DocumentOther
		}
		return strings.TrimSpace(value)
	}
}

func firstRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
