package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/kapingest"
	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/localize"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
)

func writeResearchArtifacts(targetDir, equitiesDir string, result analysis.SymbolAnalysis) error {
	if isMarketOnlyResearchAsset(result.AssetType) {
		return writeMarketResearchArtifacts(targetDir, result)
	}
	artifacts := map[string]any{
		"company_master_profile.json":          companyMasterProfileArtifact(result),
		"source_evidence_index.json":           sourceEvidenceIndexArtifact(equitiesDir, result),
		"kap_pdf_reportable_data_index.json":   kapPDFReportableDataArtifact(result),
		"kap_pdf_financial_analysis.json":      kapPDFFinancialAnalysisArtifact(result),
		"financial_statements_normalized.json": financialStatementsNormalizedArtifact(equitiesDir, result),
		"financial_quality_report.json":        financialQualityArtifact(result),
		"sector_metrics.json":                  sectorMetricsArtifact(result),
		"peer_comparison.json":                 peerComparisonArtifact(result),
		"valuation_model.json":                 valuationModelArtifact(result),
		"buffett_value_checklist.json":         buffettValueChecklistArtifact(result),
		"investment_thesis.json":               investmentThesisArtifact(result),
		"decision_support_standard.json":       decisionSupportArtifact(result),
		"risk_matrix.json":                     riskMatrixArtifact(result),
		"catalyst_calendar.json":               catalystCalendarArtifact(result),
		"technical_trade_plan.json":            technicalTradePlanArtifact(result),
		"investment_committee_memo.json":       investmentCommitteeMemoArtifact(equitiesDir, result),
		"quality_control_report.json":          qualityControlArtifact(equitiesDir, result),
	}
	for name, payload := range artifacts {
		if err := WriteJSON(filepath.Join(targetDir, name), payload); err != nil {
			return err
		}
	}
	return nil
}

func isMarketOnlyResearchAsset(assetType string) bool {
	return ohlcv.IsCommodityAssetType(assetType) || ohlcv.IsCryptoAssetType(assetType)
}

var equityOnlyResearchArtifactNames = []string{
	"company_master_profile.json",
	"source_evidence_index.json",
	"kap_pdf_reportable_data_index.json",
	"kap_pdf_financial_analysis.json",
	"financial_statements_normalized.json",
	"financial_quality_report.json",
	"sector_metrics.json",
	"peer_comparison.json",
	"valuation_model.json",
	"buffett_value_checklist.json",
	"investment_thesis.json",
	"decision_support_standard.json",
	"catalyst_calendar.json",
	"investment_committee_memo.json",
	"quality_control_report.json",
}

func writeMarketResearchArtifacts(targetDir string, result analysis.SymbolAnalysis) error {
	for _, name := range equityOnlyResearchArtifactNames {
		if err := os.Remove(filepath.Join(targetDir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	artifacts := map[string]any{
		"market_research_context.json": marketResearchContextArtifact(result),
		"risk_matrix.json":             marketRiskMatrixArtifact(result),
		"technical_trade_plan.json":    technicalTradePlanArtifact(result),
		"data_quality_report.json":     marketDataQualityArtifact(result),
	}
	for name, payload := range artifacts {
		if err := WriteJSON(filepath.Join(targetDir, name), payload); err != nil {
			return err
		}
	}
	return nil
}

func marketResearchContextArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	out := map[string]any{
		"schema_version":   1,
		"symbol":           result.Symbol,
		"asset_type":       result.AssetType,
		"analysis_date":    result.AnalysisDate,
		"currency":         result.Currency,
		"data_source":      "tradingview_ohlcv",
		"profile":          pro.Company,
		"coverage":         pro.Coverage,
		"market_context":   pro.Market,
		"scenarios":        pro.Scenarios,
		"investor_summary": result.InvestorQA.OneLineAnswer,
		"decision": map[string]any{
			"decision":        result.InvestorQA.Decision,
			"decision_label":  result.InvestorQA.DecisionLabel,
			"top_risk":        result.InvestorQA.TopRisk,
			"buy_conditions":  result.InvestorQA.BuyConditions,
			"exit_conditions": result.InvestorQA.ExitConditions,
		},
	}
	if ohlcv.IsCommodityAssetType(result.AssetType) {
		out["commodity_context"] = pro.CommodityContext
		out["macro_framework"] = []string{"DXY / USD gücü", "ABD reel faizleri", "COMEX/COT vadeli pozisyon", "altın ETF/fiziki akış", "merkez bankası ve jeopolitik haber akışı"}
	} else if ohlcv.IsCryptoAssetType(result.AssetType) {
		out["crypto_context"] = pro.CryptoContext
		out["macro_framework"] = []string{"DXY / risk iştahı", "BTC dominansı", "funding / open interest", "on-chain", "exchange-flow", "haber/sentiment"}
	}
	return out
}

func marketRiskMatrixArtifact(result analysis.SymbolAnalysis) map[string]any {
	rows := []map[string]any{}
	add := func(category, risk, mitigation string, early []string) {
		if strings.TrimSpace(risk) == "" {
			return
		}
		rows = append(rows, map[string]any{
			"category":                 category,
			"risk":                     risk,
			"probability":              "medium",
			"impact":                   "medium",
			"probability_score":        2,
			"impact_score":             2,
			"severity_score":           4,
			"severity":                 "medium",
			"owner":                    riskOwner(category),
			"manual_review":            false,
			"mitigation":               mitigation,
			"early_warning_indicators": early,
		})
	}
	for _, warning := range result.Professional.Coverage.Warnings {
		add("data_quality", warning, "Eksik kaynaklar veri entegrasyon listesine eklenmeli; sinyal tek başına emir sayılmamalı.", result.Professional.Coverage.Missing)
	}
	for _, missing := range result.Professional.Coverage.Missing {
		add("data_gap", missing, "Eksik piyasa verisi bağlanana kadar rapor teknik karar desteği olarak kullanılmalı.", nil)
	}
	for _, key := range sortedTimeframeKeys(result.Timeframes) {
		tf := result.Timeframes[key]
		for _, guardrail := range tf.Professional.Technical.Guardrails {
			add("technical_"+key, guardrail, "İşlem planı invalidation koşullarına uyulmalı.", []string{fmt.Sprintf("%s score %.1f", key, tf.Score)})
		}
	}
	return map[string]any{
		"schema_version": 1,
		"symbol":         result.Symbol,
		"asset_type":     result.AssetType,
		"risk_count":     len(rows),
		"risks":          rows,
	}
}

func marketDataQualityArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	return map[string]any{
		"schema_version":    1,
		"symbol":            result.Symbol,
		"asset_type":        result.AssetType,
		"data_quality":      pro.DataQuality,
		"coverage_score":    pro.Coverage.Score,
		"available_data":    pro.Coverage.Available,
		"data_to_improve":   pro.Coverage.Missing,
		"warnings":          pro.Coverage.Warnings,
		"data_governance":   pro.DataGovernance,
		"corporate_actions": pro.CorporateActions,
		"model_risk":        result.InvestorQA.ModelRisk,
	}
}

func companyMasterProfileArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	return map[string]any{
		"schema_version": 1,
		"symbol":         result.Symbol,
		"company_name":   firstNonEmptyReport(pro.Company.Name, result.CompanyName),
		"asset_type":     result.AssetType,
		"currency":       result.Currency,
		"analysis_date":  result.AnalysisDate,
		"sector":         pro.Company.Sector,
		"industry":       pro.Company.Industry,
		"peer_group":     pro.Company.PeerGroup,
		"business_model": map[string]any{
			"value_source":              pro.InvestmentResearch.InvestmentStory.ValueSource,
			"mispricing_question":       pro.InvestmentResearch.InvestmentStory.MispricingQuestion,
			"detected_models":           pro.KAPPDFIngest.TypeCounts,
			"classification_confidence": pro.Company.ClassificationConfidence,
		},
		"capital": map[string]any{
			"paid_capital":               pro.Company.PaidCapital,
			"registered_capital_ceiling": pro.Company.RegisteredCapitalCeiling,
			"market_cap":                 pro.Valuation.MarketCap,
		},
		"data_governance": pro.DataGovernance,
	}
}

func sourceEvidenceIndexArtifact(equitiesDir string, result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	rows := []map[string]any{}
	add := func(category, metricID, label, sourceFile, sha string, page, line int, tableID, snippet string, confidence float64, review bool) {
		if strings.TrimSpace(metricID+label+sourceFile+snippet) == "" {
			return
		}
		rows = append(rows, map[string]any{
			"category":         category,
			"metric_id":        metricID,
			"label":            label,
			"source_file":      sourceFile,
			"sha256":           sha,
			"page_number":      page,
			"line_number":      line,
			"table_name":       tableID,
			"quote":            snippet,
			"confidence_score": confidence,
			"manual_review":    review,
		})
	}
	for _, doc := range limitKAPPDFDocuments(pro.KAPPDFIngest.ImportantDocuments, 50) {
		add("kap_pdf_document", doc.FileName, doc.DocumentLabel, doc.FilePath, "", 0, 0, "", doc.ContentSnippet, doc.QualityScore, !doc.AnalysisUsable)
	}
	structuredFinancials := loadFinancialStatementArtifactRows(equitiesDir, result.Symbol, 100)
	rows = append(rows, structuredFinancialEvidenceRows(structuredFinancials.Rows, 100)...)
	if pro.RawKAPData != nil {
		for _, fact := range limitFinancialFacts(pro.RawKAPData.FinancialFacts, 250) {
			add("financial_fact", fact.ID, firstNonEmptyReport(fact.LineItemNormalized, fact.LineItemOriginal), fact.SourceFile, fact.SHA256, fact.Source.Page, fact.Source.Line, fact.Source.TableID, fact.Source.Snippet, fact.Confidence, fact.ReviewRequired || !fact.Certification.AnalysisUsable)
		}
		for _, fact := range limitDocumentFacts(pro.RawKAPData.DocumentFacts, 250) {
			add("document_fact", fact.ID, firstNonEmptyReport(fact.NormalizedKey, fact.Label), fact.SourceFile, fact.SHA256, fact.Source.Page, fact.Source.Line, fact.Source.TableID, fact.Source.Snippet, fact.Confidence, fact.ReviewRequired)
		}
		for _, event := range limitCorporateEvents(pro.RawKAPData.CorporateEvents, 150) {
			add("corporate_event", event.ID, event.EventType, event.SourceFile, event.SHA256, event.Source.Page, event.Source.Line, event.Source.TableID, event.Source.Snippet, event.Confidence, event.ReviewRequired)
		}
	}
	fullIndexFiles := allKAPSourcePaths(pro.RawKAPData, pro.KAPPDFIngest.RawDocumentsPath)
	if structuredFinancials.SourcePath != "" {
		fullIndexFiles["structured_financials"] = structuredFinancials.SourcePath
	}
	if structuredFinancials.RawDir != "" {
		fullIndexFiles["structured_financials_raw_dir"] = structuredFinancials.RawDir
	}
	return map[string]any{
		"schema_version":       1,
		"symbol":               result.Symbol,
		"status":               evidenceIndexStatus(rows, pro.RawKAPData != nil || structuredFinancials.RowCount > 0),
		"full_index_files":     fullIndexFiles,
		"indexed_sample_count": len(rows),
		"evidence_rows":        rows,
		"note":                 "Bu dosya rapor klasorunde kompakt evidence indeksi verir; tam satirlar full_index_files altindaki JSONL kaynaklarindadir.",
	}
}

func financialStatementsNormalizedArtifact(equitiesDir string, result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	status := "partial_from_professional_valuation"
	structuredFinancials := loadFinancialStatementArtifactRows(equitiesDir, result.Symbol, 500)
	metricRows := append([]map[string]any{}, structuredFinancials.Rows...)
	if len(metricRows) < 500 {
		metricRows = append(metricRows, normalizedFinancialMetricRows(pro.RawKAPData, 500-len(metricRows))...)
	}
	tableRows := normalizedFinancialTableRows(pro.RawKAPData, 100)
	if structuredFinancials.RowCount > 0 && len(tableRows) > 0 {
		status = "normalized_structured_financials_with_pdf_tables"
	} else if structuredFinancials.RowCount > 0 {
		status = "normalized_from_structured_financials"
	} else if pro.RawKAPData == nil || (len(metricRows) == 0 && len(tableRows) == 0) {
		status = "not_fully_normalized"
	} else if len(metricRows) > 0 && len(tableRows) > 0 {
		status = "normalized_sample_with_source_evidence"
	} else if len(metricRows) > 0 {
		status = "financial_metric_rows_available_table_blocks_missing"
	}
	return map[string]any{
		"schema_version": 1,
		"symbol":         result.Symbol,
		"status":         status,
		"latest_period":  pro.DataGovernance.LatestPeriod,
		"currency":       result.Currency,
		"normalization_requirements": []string{
			"period normalization",
			"unit normalization",
			"currency normalization",
			"consolidated_or_standalone flag",
			"audited_or_unaudited flag",
			"restatement tracking",
		},
		"statement_coverage": map[string]any{
			"financial_tables":                   tableCount(result),
			"financial_facts":                    factCount(result),
			"financial_tables_source_path":       sourcePath(pro.RawKAPData, "financial_tables"),
			"financial_facts_source_path":        sourcePath(pro.RawKAPData, "financial_facts"),
			"structured_financial_source":        structuredFinancials.SourcePath,
			"structured_financial_raw_dir":       structuredFinancials.RawDir,
			"structured_financial_rows":          structuredFinancials.RowCount,
			"structured_financial_periods":       structuredFinancials.PeriodCount,
			"structured_financial_line_items":    structuredFinancials.LineItemCount,
			"structured_financial_latest_period": structuredFinancials.LatestPeriod,
			"structured_financial_quality":       structuredFinancials.Quality,
			"structured_financial_lineage":       structuredFinancials.Lineage,
			"structured_financial_warning":       structuredFinancials.Warning,
			"status_by_statement_type":           statementNormalizationStatus(pro.RawKAPData),
			"certification_summary":              financialCertificationSummary(pro.RawKAPData),
			"sample_metric_rows":                 len(metricRows),
			"sample_table_rows":                  len(tableRows),
			"manual_review_required_sample":      financialReviewQueue(pro.RawKAPData, 100),
		},
		"structured_financials": map[string]any{
			"source_path":      structuredFinancials.SourcePath,
			"raw_dir":          structuredFinancials.RawDir,
			"row_count":        structuredFinancials.RowCount,
			"sample_row_count": len(structuredFinancials.Rows),
			"period_count":     structuredFinancials.PeriodCount,
			"line_item_count":  structuredFinancials.LineItemCount,
			"latest_period":    structuredFinancials.LatestPeriod,
			"quality":          structuredFinancials.Quality,
			"lineage":          structuredFinancials.Lineage,
			"warning":          structuredFinancials.Warning,
		},
		"normalized_metric_rows": metricRows,
		"normalized_table_rows":  tableRows,
		"valuation_inputs": map[string]any{
			"sales_ttm":          pro.Valuation.SalesTTM,
			"ebit_ttm":           pro.Valuation.EBITTTM,
			"ebitda_ttm":         pro.Valuation.EBITDATTM,
			"net_income_ttm":     pro.Valuation.NetIncomeTTM,
			"operating_cash_ttm": pro.Valuation.OperatingCashTTM,
			"free_cash_flow_ttm": pro.Valuation.FreeCashFlowTTM,
			"equity":             pro.Valuation.Equity,
			"total_assets":       pro.Valuation.TotalAssets,
			"net_debt":           pro.Valuation.NetDebt,
		},
	}
}

func kapPDFReportableDataArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	raw := pro.RawKAPData
	status := "raw_kap_data_not_loaded"
	if raw != nil && raw.Computed {
		status = "ready"
	} else if pro.KAPPDFIngest.Computed {
		status = "pdf_ingest_summary_only"
	}
	return map[string]any{
		"schema_version": 1,
		"symbol":         result.Symbol,
		"status":         status,
		"reporting_contract": []string{
			"PDF'den cikarilan her veri sinifi reportable_categories altinda sayilir ve tam kaynak dosyasiyla adreslenir.",
			"HTML rapor okunabilirlik icin ornek ve oncelikli satirlari gosterir; tam satir envanteri JSON/JSONL kaynaklarindadir.",
			"review_required veya dusuk guven satirlari kaybolmaz; review_queues altinda raporlanabilir kalite kuyruğuna alinir.",
		},
		"pdf_ingest": map[string]any{
			"computed":             pro.KAPPDFIngest.Computed,
			"total_documents":      pro.KAPPDFIngest.TotalDocuments,
			"analysis_usable":      pro.KAPPDFIngest.AnalysisUsableCount,
			"review_required":      pro.KAPPDFIngest.ReviewRequiredCount,
			"rejected":             pro.KAPPDFIngest.RejectedCount,
			"ocr_used":             pro.KAPPDFIngest.OCRUsedCount,
			"average_quality":      pro.KAPPDFIngest.AverageQuality,
			"raw_documents_path":   firstNonEmptyReport(pro.KAPPDFIngest.RawDocumentsPath, sourcePath(raw, "raw_documents")),
			"document_type_counts": pro.KAPPDFIngest.TypeCounts,
		},
		"reportable_categories":        kapReportableCategories(raw),
		"full_source_files":            allKAPSourcePaths(raw, pro.KAPPDFIngest.RawDocumentsPath),
		"document_coverage":            kapDocumentCoverage(raw),
		"fact_coverage":                kapFactCoverage(raw),
		"entity_and_ownership":         kapEntityCoverage(raw),
		"event_and_asset_coverage":     kapEventAssetCoverage(raw),
		"review_queues":                kapReportableReviewQueues(raw, 120),
		"known_unextracted_gap_policy": "Bir PDF'de olup hicbir kategoriye dusmeyen alan, extractor kapsamina yeni kategori/fact olarak eklenmelidir; bu artefakt kategori bosluklarini gorunur kilar.",
	}
}

func kapPDFFinancialAnalysisArtifact(result analysis.SymbolAnalysis) map[string]any {
	raw := result.Professional.RawKAPData
	status := "raw_kap_data_not_loaded"
	if raw != nil && raw.Computed && len(raw.FinancialFacts) > 0 {
		status = "ready"
	} else if raw != nil && raw.Computed {
		status = "financial_facts_missing"
	}
	return map[string]any{
		"schema_version": 1,
		"symbol":         result.Symbol,
		"status":         status,
		"source_files": map[string]string{
			"financial_facts":  sourcePath(raw, "financial_facts"),
			"financial_tables": sourcePath(raw, "financial_tables"),
			"document_index":   sourcePath(raw, "document_index"),
		},
		"selection_method": []string{
			"Ana finansal metrikler normalize kalem, orijinal satir metni, tablo tipi, donem, guven skoru ve review durumuyla puanlanir.",
			"Ayni metrik/donem icin en yuksek kanit skorlu satir secilir.",
			"Her metrik son donem, onceki donem, degisim ve kaynak kaniti ile raporlanir.",
		},
		"financial_metric_rows": kapPDFFinancialMetricRows(raw, result.Currency),
		"financial_readings":    kapPDFFinancialReadingRows(raw, result.Currency, isBankReport(result)),
		"company_info_rows":     kapPDFCompanyInfoRows(result),
	}
}

func kapReportableCategories(raw *professional.KAPRawDataBundle) []map[string]any {
	if raw == nil {
		return nil
	}
	rawValue := reflect.ValueOf(*raw)
	rawType := rawValue.Type()
	rows := []map[string]any{}
	for i := 0; i < rawType.NumField(); i++ {
		field := rawType.Field(i)
		kind := jsonTagName(field)
		if !isReportableKAPRawField(kind) {
			continue
		}
		fieldValue := rawValue.Field(i)
		count := kapReportableFieldCount(fieldValue)
		source := sourcePath(raw, kind)
		if count == 0 && source == "" && isReflectZero(fieldValue) {
			continue
		}
		rows = append(rows, map[string]any{
			"kind":                  kind,
			"label":                 reportableKindLabel(kind),
			"analysis_role":         reportableAnalysisRole(kind),
			"observed_fields":       reportableObservedFields(field.Type, 16),
			"count":                 count,
			"status":                kapReportableCategoryStatus(count, source, raw != nil),
			"full_source_path":      source,
			"full_export_available": source != "",
			"reportable":            raw != nil && (count > 0 || source != ""),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		leftCount, _ := rows[i]["count"].(int)
		rightCount, _ := rows[j]["count"].(int)
		if leftCount == rightCount {
			return fmt.Sprint(rows[i]["kind"]) < fmt.Sprint(rows[j]["kind"])
		}
		return leftCount > rightCount
	})
	return rows
}

func kapDocumentCoverage(raw *professional.KAPRawDataBundle) map[string]any {
	out := map[string]any{
		"documents":               0,
		"analysis_usable":         0,
		"review_required":         0,
		"rejected_or_not_usable":  0,
		"document_type_guess":     []map[string]any{},
		"extraction_method":       []map[string]any{},
		"parse_status":            []map[string]any{},
		"quality_gate_status":     []map[string]any{},
		"quality_score_buckets":   []map[string]any{},
		"indexed_document_sample": []map[string]any{},
	}
	if raw == nil {
		return out
	}
	typeCounts := map[string]int{}
	methodCounts := map[string]int{}
	statusCounts := map[string]int{}
	gateCounts := map[string]int{}
	bucketCounts := map[string]int{}
	docs := raw.RawDocuments
	if raw.DocumentIndex != nil && len(raw.DocumentIndex.Documents) > 0 {
		out["documents"] = len(raw.DocumentIndex.Documents)
		samples := []map[string]any{}
		for _, doc := range raw.DocumentIndex.Documents {
			incStringCount(typeCounts, firstNonEmptyReport(doc.DocumentTypeGuess, "unknown"))
			incStringCount(methodCounts, firstNonEmptyReport(doc.ExtractionMethod, "unknown"))
			incStringCount(statusCounts, firstNonEmptyReport(doc.ParseStatus, "ok"))
			incStringCount(gateCounts, firstNonEmptyReport(doc.QualityGate.Status, "unknown"))
			incStringCount(bucketCounts, qualityBucket(doc.QualityScore))
			if doc.AnalysisUsable {
				out["analysis_usable"] = out["analysis_usable"].(int) + 1
			}
			if doc.HumanReviewRequired {
				out["review_required"] = out["review_required"].(int) + 1
			}
			if !doc.AnalysisUsable {
				out["rejected_or_not_usable"] = out["rejected_or_not_usable"].(int) + 1
			}
			if len(samples) < 50 {
				samples = append(samples, map[string]any{
					"document_id":           doc.DocumentID,
					"file_name":             doc.FileName,
					"file_path":             doc.FilePath,
					"document_type_guess":   doc.DocumentTypeGuess,
					"period":                stringPtrValue(doc.Period),
					"document_date":         stringPtrValue(doc.DocumentDate),
					"quality_score":         doc.QualityScore,
					"analysis_usable":       doc.AnalysisUsable,
					"human_review_required": doc.HumanReviewRequired,
				})
			}
		}
		out["indexed_document_sample"] = samples
	} else {
		out["documents"] = len(docs)
		for _, doc := range docs {
			incStringCount(typeCounts, firstNonEmptyReport(doc.DocumentTypeGuess, "unknown"))
			incStringCount(methodCounts, firstNonEmptyReport(doc.ExtractionMethod, "unknown"))
			incStringCount(statusCounts, firstNonEmptyReport(doc.ParseStatus, "ok"))
			incStringCount(gateCounts, firstNonEmptyReport(doc.QualityGate.Status, "unknown"))
			incStringCount(bucketCounts, qualityBucket(doc.QualityScore))
			if doc.AnalysisUsable {
				out["analysis_usable"] = out["analysis_usable"].(int) + 1
			}
			if doc.HumanReviewRequired {
				out["review_required"] = out["review_required"].(int) + 1
			}
			if !doc.AnalysisUsable {
				out["rejected_or_not_usable"] = out["rejected_or_not_usable"].(int) + 1
			}
		}
	}
	out["document_type_guess"] = topStringCountRows(typeCounts, 50)
	out["extraction_method"] = topStringCountRows(methodCounts, 20)
	out["parse_status"] = topStringCountRows(statusCounts, 20)
	out["quality_gate_status"] = topStringCountRows(gateCounts, 20)
	out["quality_score_buckets"] = topStringCountRows(bucketCounts, 20)
	return out
}

func kapFactCoverage(raw *professional.KAPRawDataBundle) map[string]any {
	out := map[string]any{
		"document_facts_total":       0,
		"financial_facts_total":      0,
		"financial_tables_total":     0,
		"document_fact_groups":       []map[string]any{},
		"document_fact_kinds":        []map[string]any{},
		"financial_statement_types":  []map[string]any{},
		"financial_line_items_top":   []map[string]any{},
		"financial_table_types":      []map[string]any{},
		"financial_table_rows_total": 0,
		"certification_summary":      financialCertificationSummary(raw),
	}
	if raw == nil {
		return out
	}
	docGroups := map[string]int{}
	docKinds := map[string]int{}
	statementTypes := map[string]int{}
	lineItems := map[string]int{}
	tableTypes := map[string]int{}
	tableRows := 0
	for _, fact := range raw.DocumentFacts {
		incStringCount(docGroups, firstNonEmptyReport(fact.Group, "unknown"))
		incStringCount(docKinds, firstNonEmptyReport(fact.Kind, "unknown"))
	}
	for _, fact := range raw.FinancialFacts {
		incStringCount(statementTypes, firstNonEmptyReport(fact.StatementType, "unknown"))
		incStringCount(lineItems, firstNonEmptyReport(fact.LineItemNormalized, fact.LineItemOriginal, "unknown"))
	}
	for _, table := range raw.FinancialTables {
		incStringCount(tableTypes, firstNonEmptyReport(table.TableType, "unknown"))
		tableRows += len(table.Rows)
	}
	out["document_facts_total"] = len(raw.DocumentFacts)
	out["financial_facts_total"] = len(raw.FinancialFacts)
	out["financial_tables_total"] = len(raw.FinancialTables)
	out["document_fact_groups"] = topStringCountRows(docGroups, 50)
	out["document_fact_kinds"] = topStringCountRows(docKinds, 50)
	out["financial_statement_types"] = topStringCountRows(statementTypes, 50)
	out["financial_line_items_top"] = topStringCountRows(lineItems, 100)
	out["financial_table_types"] = topStringCountRows(tableTypes, 50)
	out["financial_table_rows_total"] = tableRows
	return out
}

func kapEntityCoverage(raw *professional.KAPRawDataBundle) map[string]any {
	out := map[string]any{
		"people_total":          0,
		"ownership_facts_total": 0,
		"people_roles":          []map[string]any{},
		"ownership_with_ratio":  0,
		"ownership_with_amount": 0,
		"manual_review":         0,
	}
	if raw == nil {
		return out
	}
	roles := map[string]int{}
	review := 0
	withRatio := 0
	withAmount := 0
	for _, person := range raw.People {
		incStringCount(roles, firstNonEmptyReport(person.Role, "unknown"))
		if person.ReviewRequired {
			review++
		}
	}
	for _, fact := range raw.OwnershipFacts {
		if fact.ReviewRequired {
			review++
		}
		if fact.ShareRatio != nil {
			withRatio++
		}
		if fact.ShareAmount != nil {
			withAmount++
		}
	}
	out["people_total"] = len(raw.People)
	out["ownership_facts_total"] = len(raw.OwnershipFacts)
	out["people_roles"] = topStringCountRows(roles, 50)
	out["ownership_with_ratio"] = withRatio
	out["ownership_with_amount"] = withAmount
	out["manual_review"] = review
	return out
}

func kapEventAssetCoverage(raw *professional.KAPRawDataBundle) map[string]any {
	out := map[string]any{
		"kap_events_total":       0,
		"corporate_events_total": 0,
		"asset_events_total":     0,
		"asset_inventory_items":  0,
		"kap_event_categories":   []map[string]any{},
		"corporate_event_types":  []map[string]any{},
		"asset_types":            []map[string]any{},
		"asset_cities":           []map[string]any{},
		"asset_statuses":         []map[string]any{},
	}
	if raw == nil {
		return out
	}
	kapEventCategories := map[string]int{}
	corporateTypes := map[string]int{}
	assetTypes := map[string]int{}
	assetCities := map[string]int{}
	assetStatuses := map[string]int{}
	for _, event := range raw.KAPEvents {
		incStringCount(kapEventCategories, firstNonEmptyReport(event.EventCategory, event.DocumentClass, "unknown"))
	}
	for _, event := range raw.CorporateEvents {
		incStringCount(corporateTypes, firstNonEmptyReport(event.EventType, "unknown"))
	}
	for _, event := range raw.AssetEvents {
		incStringCount(assetTypes, firstNonEmptyReport(event.AssetType, "unknown"))
		incStringCount(assetCities, firstNonEmptyReport(event.City, "unknown"))
		incStringCount(assetStatuses, firstNonEmptyReport(event.Status, "unknown"))
	}
	out["kap_events_total"] = len(raw.KAPEvents)
	out["corporate_events_total"] = len(raw.CorporateEvents)
	out["asset_events_total"] = len(raw.AssetEvents)
	out["asset_inventory_items"] = rawInventoryItemCount(raw)
	out["kap_event_categories"] = topStringCountRows(kapEventCategories, 50)
	out["corporate_event_types"] = topStringCountRows(corporateTypes, 50)
	out["asset_types"] = topStringCountRows(assetTypes, 50)
	out["asset_cities"] = topStringCountRows(assetCities, 50)
	out["asset_statuses"] = topStringCountRows(assetStatuses, 50)
	return out
}

func kapReportableReviewQueues(raw *professional.KAPRawDataBundle, limit int) []map[string]any {
	if raw == nil {
		return nil
	}
	rows := []map[string]any{}
	add := func(kind, id, sourceFile, reason string, confidence float64) {
		if limit > 0 && len(rows) >= limit {
			return
		}
		rows = append(rows, map[string]any{
			"kind":             kind,
			"id":               id,
			"source_file":      sourceFile,
			"reason":           reason,
			"confidence_score": confidence,
		})
	}
	if raw.DocumentIndex != nil {
		for _, doc := range raw.DocumentIndex.Documents {
			if doc.HumanReviewRequired || !doc.AnalysisUsable {
				add("document", doc.DocumentID, doc.FilePath, firstNonEmptyReport(strings.Join(doc.Warnings, "; "), strings.Join(doc.QualityGate.Reasons, "; "), "document_quality_review"), doc.QualityScore)
			}
		}
	}
	for _, fact := range raw.DocumentFacts {
		if fact.ReviewRequired || fact.Confidence > 0 && fact.Confidence < 0.65 {
			add("document_fact", fact.ID, fact.SourceFile, firstNonEmptyReport(strings.Join(fact.Warnings, "; "), "low_confidence_or_review_required"), fact.Confidence)
		}
	}
	for _, fact := range raw.FinancialFacts {
		if fact.ReviewRequired || !fact.Certification.AnalysisUsable || fact.Confidence > 0 && fact.Confidence < 0.65 {
			add("financial_fact", fact.ID, fact.SourceFile, firstNonEmptyReport(strings.Join(fact.Certification.Reasons, "; "), strings.Join(fact.Warnings, "; "), "low_confidence_or_review_required"), fact.Confidence)
		}
	}
	for _, table := range raw.FinancialTables {
		if table.ReviewRequired || !table.Certification.AnalysisUsable || table.Confidence > 0 && table.Confidence < 0.65 {
			add("financial_table", table.ID, table.SourceFile, firstNonEmptyReport(strings.Join(table.Certification.Reasons, "; "), strings.Join(table.Warnings, "; "), "low_confidence_or_review_required"), table.Confidence)
		}
	}
	for _, person := range raw.People {
		if person.ReviewRequired || person.Confidence > 0 && person.Confidence < 0.75 {
			add("person", person.ID, person.SourceFile, "low_confidence_or_review_required", person.Confidence)
		}
	}
	for _, fact := range raw.OwnershipFacts {
		if fact.ReviewRequired || fact.Confidence > 0 && fact.Confidence < 0.75 {
			add("ownership_fact", fact.ID, fact.SourceFile, "low_confidence_or_review_required", fact.Confidence)
		}
	}
	for _, event := range raw.CorporateEvents {
		if event.ReviewRequired || event.Confidence > 0 && event.Confidence < 0.75 {
			add("corporate_event", event.ID, event.SourceFile, "low_confidence_or_review_required", event.Confidence)
		}
	}
	for _, item := range raw.ExtractionErrors {
		add("extraction_error", item.Stage, item.FilePath, item.Error, 0)
	}
	return rows
}

func financialQualityArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	return map[string]any{
		"schema_version":    1,
		"symbol":            result.Symbol,
		"summary":           pro.InvestmentResearch.FinancialQuality.Summary,
		"metrics":           pro.InvestmentResearch.FinancialQuality.Metrics,
		"red_flags":         pro.InvestmentResearch.FinancialQuality.RedFlags,
		"need_to_explain":   pro.InvestmentResearch.FinancialQuality.NeedToExplain,
		"ratios":            pro.Valuation.Ratios,
		"sector_metrics":    pro.Valuation.SectorMetrics,
		"sector_financials": pro.SectorFinancials,
	}
}

func sectorMetricsArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	return map[string]any{
		"schema_version":            1,
		"symbol":                    result.Symbol,
		"sector":                    pro.Company.Sector,
		"industry":                  pro.Company.Industry,
		"sector_model":              pro.Valuation.SectorModel,
		"allowed_ratios":            pro.Valuation.AllowedRatios,
		"suppressed_ratios":         pro.Valuation.SuppressedRatios,
		"sector_metrics":            pro.Valuation.SectorMetrics,
		"sector_financial_analysis": pro.SectorFinancials,
		"business_model_tags":       businessModelTags(result),
	}
}

func peerComparisonArtifact(result analysis.SymbolAnalysis) map[string]any {
	return map[string]any{
		"schema_version":  1,
		"symbol":          result.Symbol,
		"peer_comparison": result.Professional.Peers,
	}
}

func valuationModelArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	return map[string]any{
		"schema_version":     1,
		"symbol":             result.Symbol,
		"recommendation":     pro.InvestmentResearch.InstitutionalMemo.Recommendation,
		"evidence_policy":    pro.EvidencePolicy,
		"valuation_bridge":   pro.InvestmentResearch.ValuationBridge,
		"value_investing":    pro.ValueInvesting,
		"valuation_analysis": pro.Valuation,
		"scenarios":          pro.Scenarios,
		"method_limitations": pro.InvestmentResearch.ValuationBridge.Limitations,
	}
}

func buffettValueChecklistArtifact(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	return map[string]any{
		"schema_version":       1,
		"symbol":               result.Symbol,
		"analysis_date":        result.AnalysisDate,
		"currency":             result.Currency,
		"buffett_checklist":    pro.ValueInvesting.BuffettChecklist,
		"value_investing":      pro.ValueInvesting,
		"investment_decision":  pro.InvestmentResearch.DecisionFramework,
		"institutional_memo":   pro.InvestmentResearch.InstitutionalMemo,
		"financial_quality":    pro.InvestmentResearch.FinancialQuality,
		"valuation_bridge":     pro.InvestmentResearch.ValuationBridge,
		"source_evidence_note": "Kriterlerin kaynak dosyaları source_evidence_index.json ve financial_statements_normalized.json içinde izlenir.",
	}
}

func investmentThesisArtifact(result analysis.SymbolAnalysis) map[string]any {
	review := result.Professional.InvestmentResearch
	return map[string]any{
		"schema_version":          1,
		"symbol":                  result.Symbol,
		"investment_story":        review.InvestmentStory,
		"decision_framework":      review.DecisionFramework,
		"open_research_questions": review.OpenResearchQuestions,
		"institutional_memo":      review.InstitutionalMemo,
	}
}

func decisionSupportArtifact(result analysis.SymbolAnalysis) map[string]any {
	return map[string]any{
		"schema_version":   1,
		"symbol":           result.Symbol,
		"asset_type":       result.AssetType,
		"analysis_date":    result.AnalysisDate,
		"decision_support": result.DecisionSupport,
		"disclaimer":       firstNonEmptyReport(result.Disclaimer, ohlcv.Disclaimer),
	}
}

func riskMatrixArtifact(result analysis.SymbolAnalysis) map[string]any {
	rows := []map[string]any{}
	add := func(category, risk, probability, impact, mitigation string, early []string) {
		if strings.TrimSpace(risk) == "" {
			return
		}
		probabilityScore := riskLevelScore(probability)
		impactScore := riskLevelScore(impact)
		severityScore := probabilityScore * impactScore
		rows = append(rows, map[string]any{
			"category":                 category,
			"risk":                     risk,
			"probability":              probability,
			"impact":                   impact,
			"probability_score":        probabilityScore,
			"impact_score":             impactScore,
			"severity_score":           severityScore,
			"severity":                 riskSeverityLabel(severityScore),
			"owner":                    riskOwner(category),
			"manual_review":            severityScore >= 6 || strings.Contains(category, "committee") || strings.Contains(category, "data_quality"),
			"mitigation":               mitigation,
			"early_warning_indicators": early,
		})
	}
	review := result.Professional.InvestmentResearch
	for _, blocker := range review.InstitutionalMemo.BlockingIssues {
		add("committee_blocker", blocker, "medium", "high", "Komite oncesi required_fixes kapatilmali.", review.InstitutionalMemo.RequiredFixes)
	}
	for _, risk := range review.FinancialQuality.RedFlags {
		add("financial", risk, "medium", "high", "Finansal tablo kalemi kaynak kanitiyla ayriştirilmali.", review.FinancialQuality.NeedToExplain)
	}
	for _, risk := range result.Professional.Disclosure.RiskFlags {
		add("kap_disclosure", risk, "medium", "medium", "KAP olayi ve ilgili dokuman kaniti takip edilmeli.", nil)
	}
	for _, warning := range result.Professional.KAPPDFIngest.Warnings {
		add("data_quality", warning, "high", "medium", "PDF kalite kapisi ve insan inceleme kuyruğu ile temizlenmeli.", nil)
	}
	for _, key := range sortedTimeframeKeys(result.Timeframes) {
		tf := result.Timeframes[key]
		for _, guardrail := range tf.Professional.Technical.Guardrails {
			add("technical_"+key, guardrail, "medium", "medium", "Islem plani invalidation kosullarina uyulmali.", []string{fmt.Sprintf("%s score %.1f", key, tf.Score)})
		}
	}
	highSeverity := 0
	manualReview := 0
	for _, row := range rows {
		if row["severity"] == "high" || row["severity"] == "critical" {
			highSeverity++
		}
		if row["manual_review"] == true {
			manualReview++
		}
	}
	return map[string]any{
		"schema_version":      1,
		"symbol":              result.Symbol,
		"risk_count":          len(rows),
		"high_severity_count": highSeverity,
		"manual_review_count": manualReview,
		"risk_scale":          "probability_score 1-3 x impact_score 1-3",
		"risks":               rows,
	}
}

func catalystCalendarArtifact(result analysis.SymbolAnalysis) map[string]any {
	review := result.Professional.InvestmentResearch
	events := []map[string]any{}
	for _, catalyst := range review.InvestmentStory.Catalysts {
		events = append(events, map[string]any{"type": "positive_catalyst", "description": catalyst, "date": nil, "source": "investment_story"})
	}
	for _, condition := range review.DecisionFramework.BuyConditions {
		events = append(events, map[string]any{"type": "buy_trigger", "description": condition, "date": nil, "source": "decision_framework"})
	}
	for _, condition := range review.DecisionFramework.SellConditions {
		events = append(events, map[string]any{"type": "negative_trigger", "description": condition, "date": nil, "source": "decision_framework"})
	}
	if result.Professional.RawKAPData != nil {
		for _, event := range limitCorporateEvents(result.Professional.RawKAPData.CorporateEvents, 100) {
			events = append(events, map[string]any{
				"type":        event.EventType,
				"description": event.Title,
				"date":        event.DocumentDate,
				"period":      event.Period,
				"source_file": event.SourceFile,
				"confidence":  event.Confidence,
			})
		}
	}
	return map[string]any{
		"schema_version": 1,
		"symbol":         result.Symbol,
		"events":         events,
	}
}

func technicalTradePlanArtifact(result analysis.SymbolAnalysis) map[string]any {
	plans := []map[string]any{}
	for _, key := range sortedTimeframeKeys(result.Timeframes) {
		tf := result.Timeframes[key]
		plan := tf.TradePlan
		plans = append(plans, map[string]any{
			"timeframe":       key,
			"trend_regime":    tf.TrendBias,
			"momentum_regime": technicalMomentumRegime(tf),
			"volume_regime":   technicalVolumeRegime(tf),
			"volatility_regime": map[string]any{
				"atr14": tf.Indicators.ATR14,
			},
			"score": tf.Score,
			"support_resistance": map[string]any{
				"nearest_support":    tf.NearestSupport,
				"nearest_resistance": tf.NearestResistance,
				"supports":           tf.SupportLevels,
				"resistances":        tf.ResistanceLevels,
			},
			"actionable_plan":    actionableTradePlan(key, result, plan),
			"trade_plan":         plan,
			"technical_evidence": tf.Professional.Technical,
			"backtest":           tf.Professional.Backtest,
			"price_adjustment":   tf.Professional.PriceAdjustment,
			"position_sizing":    tf.Professional.PositionSizing,
			"liquidity":          tf.Professional.Liquidity,
		})
	}
	return map[string]any{
		"schema_version": 1,
		"symbol":         result.Symbol,
		"asset_type":     result.AssetType,
		"disclaimer":     firstNonEmptyReport(result.Disclaimer, ohlcv.Disclaimer),
		"plans":          plans,
	}
}

func investmentCommitteeMemoArtifact(equitiesDir string, result analysis.SymbolAnalysis) map[string]any {
	review := result.Professional.InvestmentResearch
	memo := review.InstitutionalMemo
	return map[string]any{
		"schema_version":           1,
		"symbol":                   result.Symbol,
		"recommendation":           memo.Recommendation,
		"workflow_status":          memo.WorkflowStatus,
		"position_size_suggestion": memo.PositionSizeSuggestion,
		"investment_horizon":       memo.InvestmentHorizon,
		"expected_return_pct":      memo.ExpectedReturnPct,
		"downside_risk_pct":        memo.DownsideRiskPct,
		"risk_reward_ratio":        memo.RiskRewardRatio,
		"liquidity_consideration":  memo.LiquidityConsideration,
		"portfolio_fit":            memo.PortfolioFit,
		"approval_conditions":      memo.ApprovalConditions,
		"rejection_conditions":     memo.RejectionConditions,
		"key_assumptions":          memo.KeyAssumptions,
		"memo":                     memo,
		"decision_framework":       review.DecisionFramework,
		"readiness":                review.Readiness,
		"valuation_bridge":         review.ValuationBridge,
		"quality_gate":             qualityControlArtifact(equitiesDir, result),
	}
}

func qualityControlArtifact(equitiesDir string, result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	structuredFinancials := loadFinancialStatementArtifactRows(equitiesDir, result.Symbol, 1)
	manualReview := []string{}
	manualReview = append(manualReview, pro.InvestmentResearch.InstitutionalMemo.BlockingIssues...)
	manualReview = append(manualReview, pro.Coverage.Missing...)
	manualReview = append(manualReview, pro.KAPPDFIngest.Warnings...)
	if pro.KAPPDFIngest.ReviewRequiredCount > 0 {
		manualReview = append(manualReview, fmt.Sprintf("kap_pdf_review_required_%d", pro.KAPPDFIngest.ReviewRequiredCount))
	}
	if pro.KAPPDFIngest.RejectedCount > 0 {
		manualReview = append(manualReview, fmt.Sprintf("kap_pdf_rejected_%d", pro.KAPPDFIngest.RejectedCount))
	}
	resolvedContradictions := []any{}
	if pro.RawKAPData != nil && pro.RawKAPData.KnowledgeGraph != nil {
		for _, resolved := range pro.RawKAPData.KnowledgeGraph.ResolvedContradictions {
			resolvedContradictions = append(resolvedContradictions, resolved)
		}
	}
	return map[string]any{
		"schema_version":     1,
		"symbol":             result.Symbol,
		"data_quality_score": pro.DataQuality,
		"coverage_score":     pro.Coverage.Score,
		"confidence_score":   reportConfidenceFor(result).Score,
		"evidence_policy":    pro.EvidencePolicy,
		"pdf_quality": map[string]any{
			"documents":       pro.KAPPDFIngest.TotalDocuments,
			"analysis_usable": pro.KAPPDFIngest.AnalysisUsableCount,
			"review_required": pro.KAPPDFIngest.ReviewRequiredCount,
			"rejected":        pro.KAPPDFIngest.RejectedCount,
			"ocr_used":        pro.KAPPDFIngest.OCRUsedCount,
			"average_quality": pro.KAPPDFIngest.AverageQuality,
		},
		"structured_financials": map[string]any{
			"source_path":     structuredFinancials.SourcePath,
			"raw_dir":         structuredFinancials.RawDir,
			"row_count":       structuredFinancials.RowCount,
			"period_count":    structuredFinancials.PeriodCount,
			"line_item_count": structuredFinancials.LineItemCount,
			"latest_period":   structuredFinancials.LatestPeriod,
			"quality":         structuredFinancials.Quality,
			"warning":         structuredFinancials.Warning,
		},
		"missing_data":             pro.Coverage.Missing,
		"decision_support_status":  decisionSupportStatus(result),
		"decision_support_missing": decisionSupportMissingKeys(result),
		"manual_review_required":   uniqueReportStrings(manualReview),
		"kap_data_reconciliation": map[string]any{
			"resolved":       len(resolvedContradictions),
			"resolved_items": resolvedContradictions,
		},
		"warnings":                   uniqueReportStrings(append(append([]string{}, pro.Coverage.Warnings...), pro.InvestmentResearch.Warnings...)),
		"acceptance_criteria_status": acceptanceCriteriaStatus(structuredFinancials, result),
	}
}

func decisionSupportStatus(result analysis.SymbolAnalysis) string {
	if result.DecisionSupport == nil {
		return "not_applicable"
	}
	return result.DecisionSupport.Status
}

func decisionSupportMissingKeys(result analysis.SymbolAnalysis) []string {
	if result.DecisionSupport == nil {
		return nil
	}
	out := []string{}
	for _, item := range result.DecisionSupport.MissingInputs {
		if strings.TrimSpace(item.Key) != "" {
			out = append(out, item.Key)
		}
	}
	return uniqueReportStrings(out)
}

func acceptanceCriteriaStatus(structuredFinancials financialStatementArtifactRows, result analysis.SymbolAnalysis) []map[string]any {
	pro := result.Professional
	structuredFinancialsUsable := structuredFinancials.RowCount > 0 && structuredFinancials.Quality.BacktestSafe && structuredFinancials.Quality.FinanciallyConsistent
	return []map[string]any{
		{"criterion": "metric_source_evidence", "status": statusBool(structuredFinancials.RowCount > 0 || sourcePath(pro.RawKAPData, "financial_facts") != "" || len(pro.KAPPDFIngest.ImportantDocuments) > 0)},
		{"criterion": "sector_specific_analysis", "status": statusBool(len(pro.Valuation.AllowedRatios) > 0 || pro.SectorFinancials.Applicable)},
		{"criterion": "financial_statement_normalization", "status": statusBool(structuredFinancialsUsable || (pro.DataGovernance.FinanciallyConsistent && pro.DataGovernance.ReconciliationFailureCount == 0 && hasCertifiedFinancialRows(pro.RawKAPData)))},
		{"criterion": "peer_comparison", "status": statusBool(pro.Peers.PeerCount > 0)},
		{"criterion": "strict_evidence_policy", "status": statusBool(pro.EvidencePolicy.Status == "pass")},
		{"criterion": "valuation_assumptions", "status": statusBool(pro.EvidencePolicy.ValuationTargetsAllowed)},
		{"criterion": "low_confidence_data_not_core_evidence", "status": statusBool(pro.KAPPDFIngest.RejectedCount == 0 || pro.KAPPDFIngest.AnalysisUsableCount < pro.KAPPDFIngest.TotalDocuments)},
		{"criterion": "investment_committee_memo", "status": statusBool(pro.EvidencePolicy.RecommendationAllowed && pro.InvestmentResearch.InstitutionalMemo.Recommendation != "")},
		{"criterion": "quality_report", "status": "pass"},
	}
}

func normalizedFinancialMetricRows(raw *professional.KAPRawDataBundle, limit int) []map[string]any {
	if raw == nil {
		return nil
	}
	rows := []map[string]any{}
	for _, fact := range limitFinancialFacts(raw.FinancialFacts, limit) {
		rows = append(rows, map[string]any{
			"metric_id":            fact.ID,
			"period":               stringPtrValue(fact.Period),
			"document_date":        stringPtrValue(fact.DocumentDate),
			"statement_type":       firstNonEmptyReport(fact.StatementType, "unknown"),
			"line_item_original":   fact.LineItemOriginal,
			"line_item_normalized": fact.LineItemNormalized,
			"value":                fact.Value,
			"currency":             fact.Currency,
			"unit":                 fact.Unit,
			"normalized_value":     kapMetricSignedValue(fact),
			"normalized_currency":  kapMetricCurrency(fact, fact.Currency),
			"consolidation_scope":  fact.ConsolidationScope,
			"audit_status":         fact.AuditStatus,
			"source_file":          fact.SourceFile,
			"sha256":               fact.SHA256,
			"page_number":          fact.Source.Page,
			"line_number":          fact.Source.Line,
			"table_id":             fact.Source.TableID,
			"row_index":            fact.Source.RowIndex,
			"snippet":              fact.Source.Snippet,
			"confidence_score":     fact.Confidence,
			"review_required":      fact.ReviewRequired,
			"decision_usable":      fact.Certification.AnalysisUsable,
			"certification":        fact.Certification,
			"warnings":             fact.Warnings,
		})
	}
	return rows
}

func normalizedFinancialTableRows(raw *professional.KAPRawDataBundle, limit int) []map[string]any {
	if raw == nil {
		return nil
	}
	if limit > 0 && len(raw.FinancialTables) > limit {
		rawTables := raw.FinancialTables[:limit]
		return normalizedFinancialTableRowsFromSlice(rawTables)
	}
	return normalizedFinancialTableRowsFromSlice(raw.FinancialTables)
}

func normalizedFinancialTableRowsFromSlice(tables []kapingest.ExtractedFinancialTable) []map[string]any {
	rows := []map[string]any{}
	for _, table := range tables {
		rows = append(rows, map[string]any{
			"table_id":            table.ID,
			"period":              stringPtrValue(table.Period),
			"document_date":       stringPtrValue(table.DocumentDate),
			"table_type":          firstNonEmptyReport(table.TableType, "unknown"),
			"currency":            table.Currency,
			"unit":                table.Unit,
			"consolidation_scope": table.ConsolidationScope,
			"audit_status":        table.AuditStatus,
			"row_count":           len(table.Rows),
			"source_file":         table.SourceFile,
			"sha256":              table.SHA256,
			"page_number":         table.Source.Page,
			"line_number":         table.Source.Line,
			"table_source_id":     table.Source.TableID,
			"snippet":             table.Source.Snippet,
			"confidence_score":    table.Confidence,
			"review_required":     table.ReviewRequired,
			"decision_usable":     table.Certification.AnalysisUsable,
			"certification":       table.Certification,
			"warnings":            table.Warnings,
			"sample_table_rows":   limitFinancialTableRows(table.Rows, 5),
		})
	}
	return rows
}

type financialStatementArtifactRows struct {
	Rows          []map[string]any
	SourcePath    string
	RawDir        string
	RowCount      int
	PeriodCount   int
	LineItemCount int
	LatestPeriod  string
	Quality       domain.FinancialDataQuality
	Lineage       []domain.DataLineageEvent
	Warning       string
}

func loadFinancialStatementArtifactRows(equitiesDir, symbol string, limit int) financialStatementArtifactRows {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := financialStatementArtifactRows{}
	if strings.TrimSpace(equitiesDir) == "" || symbol == "" {
		out.Warning = "structured_financials_path_missing"
		return out
	}
	bilancoPath := ""
	for _, candidate := range financialStatementArtifactCandidatePaths(equitiesDir, symbol) {
		if _, err := os.Stat(candidate); err == nil {
			bilancoPath = candidate
			break
		}
	}
	if bilancoPath == "" {
		bilancoPath = filepath.Join(equitiesDir, symbol, "financials", "bilanco.json")
	}
	rawDir := filepath.Join(equitiesDir, symbol, "financials", "raw")
	out.SourcePath = bilancoPath
	out.RawDir = rawDir
	data, err := os.ReadFile(bilancoPath)
	if err != nil {
		out.Warning = "structured_financials_read_failed: " + err.Error()
		return out
	}
	var info domain.BilancoInfo
	if err := json.Unmarshal(data, &info); err != nil {
		out.Warning = "structured_financials_json_invalid: " + err.Error()
		return out
	}
	domain.NormalizeBilancoInfo(&info, symbol)
	out.Quality = info.Quality
	out.Lineage = limitFinancialLineage(info.Lineage, 20)
	out.LineItemCount = len(info.Data)
	out.PeriodCount = len(info.Periods)

	codes := make([]string, 0, len(info.Data))
	for code := range info.Data {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	rows := make([]map[string]any, 0)
	for _, code := range codes {
		field := info.Data[code]
		years := make([]string, 0, len(field.Years))
		for year := range field.Years {
			years = append(years, year)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(years)))
		for _, yearText := range years {
			year, err := strconv.Atoi(strings.TrimSpace(yearText))
			if err != nil || year <= 0 {
				continue
			}
			values := field.Years[yearText]
			for index, value := range values {
				if value == nil {
					continue
				}
				quarter := domain.FiscalQuarterFromIndex(index)
				periodKey := domain.FinancialPeriodKey(year, quarter)
				if periodKey == "" {
					continue
				}
				period := info.Periods[periodKey]
				if period.Key == "" {
					period = domain.NewFinancialPeriod(year, quarter, info.Source, info.FinancialGroup, info.Currency, info.FetchedAt)
				}
				periodEnd := period.PeriodEnd
				if periodEnd.IsZero() {
					periodEnd = domain.FiscalPeriodEnd(year, quarter)
				}
				currency := firstNonEmptyReport(period.Currency, info.Currency, "TRY")
				financialGroup := firstNonEmptyReport(period.FinancialGroup, info.FinancialGroup)
				decisionUsable := period.BacktestSafe && info.Quality.BacktestSafe && info.Quality.FinanciallyConsistent
				confidence := 1.0
				if !info.Quality.FinanciallyConsistent {
					confidence = 0.85
				}
				if !period.BacktestSafe {
					confidence = 0.70
				}
				certificationStatus := "certified_structured_financials"
				if !decisionUsable {
					certificationStatus = "review_required_structured_financials"
				}
				warnings := append([]string{}, period.Warnings...)
				row := map[string]any{
					"metric_id":             code,
					"period":                periodKey,
					"fiscal_year":           year,
					"fiscal_quarter":        quarter,
					"period_end":            formatReportTime(periodEnd),
					"report_date":           formatReportTimePtr(period.ReportDate),
					"publish_date":          formatReportTimePtr(period.PublishDate),
					"available_at":          formatReportTimePtr(domain.EffectiveFinancialAvailableAt(period)),
					"availability_source":   period.AvailabilitySource,
					"statement_type":        "structured_financial_statement",
					"line_item_original":    firstNonEmptyReport(field.DescTR, field.DescEN, code),
					"line_item_normalized":  code,
					"line_item_tr":          field.DescTR,
					"line_item_en":          field.DescEN,
					"value":                 *value,
					"currency":              currency,
					"unit":                  "currency_unit",
					"scale":                 "unit",
					"financial_group":       financialGroup,
					"source":                firstNonEmptyReport(info.Source, "financials/bilanco.json"),
					"source_file":           bilancoPath,
					"raw_source_file":       structuredFinancialRawPath(rawDir, year),
					"source_evidence":       "structured_financials_bilanco_json",
					"backtest_safe":         period.BacktestSafe,
					"quality_backtest_safe": info.Quality.BacktestSafe,
					"confidence_score":      confidence,
					"review_required":       !decisionUsable,
					"decision_usable":       decisionUsable,
					"certification": map[string]any{
						"status":          certificationStatus,
						"analysis_usable": decisionUsable,
						"source":          "financials/bilanco.json",
						"reasons":         structuredFinancialCertificationReasons(info.Quality, period),
					},
					"warnings":         uniqueReportStrings(warnings),
					"quality_warnings": uniqueReportStrings(info.Quality.Warnings),
				}
				rows = append(rows, row)
				out.RowCount++
				if periodKey > out.LatestPeriod {
					out.LatestPeriod = periodKey
				}
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		leftPeriod, _ := rows[i]["period"].(string)
		rightPeriod, _ := rows[j]["period"].(string)
		if leftPeriod == rightPeriod {
			leftMetric, _ := rows[i]["metric_id"].(string)
			rightMetric, _ := rows[j]["metric_id"].(string)
			return leftMetric < rightMetric
		}
		return leftPeriod > rightPeriod
	})
	if limit > 0 && len(rows) > limit {
		out.Rows = rows[:limit]
	} else {
		out.Rows = rows
	}
	return out
}

func financialStatementArtifactCandidatePaths(equitiesDir, symbol string) []string {
	symbolUpper := strings.ToUpper(strings.TrimSpace(symbol))
	symbolLower := strings.ToLower(symbolUpper)
	if symbolUpper == "" {
		return nil
	}
	paths := []string{
		filepath.Join(equitiesDir, symbolUpper, "financials", "bilanco.json"),
		filepath.Join(equitiesDir, symbolUpper, "financials", "kap_bilanco.json"),
	}
	dataDir := filepath.Dir(filepath.Clean(equitiesDir))
	paths = append(paths,
		filepath.Join(dataDir, "processed", "by_ticker", symbolUpper, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", "by_ticker", symbolLower, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolUpper, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", symbolLower, "by_ticker", symbolLower, "kap_financials", "bilanco.json"),
		filepath.Join(dataDir, "processed", symbolLower, "kap_financials", "bilanco.json"),
		filepath.Join("data", "processed", "by_ticker", symbolUpper, "kap_financials", "bilanco.json"),
		filepath.Join("data", "processed", "by_ticker", symbolLower, "kap_financials", "bilanco.json"),
	)
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func structuredFinancialEvidenceRows(metricRows []map[string]any, limit int) []map[string]any {
	rows := []map[string]any{}
	for _, metric := range metricRows {
		if limit > 0 && len(rows) >= limit {
			break
		}
		metricID, _ := metric["metric_id"].(string)
		label, _ := metric["line_item_original"].(string)
		sourceFile, _ := metric["source_file"].(string)
		if strings.TrimSpace(metricID+label+sourceFile) == "" {
			continue
		}
		period, _ := metric["period"].(string)
		currency, _ := metric["currency"].(string)
		confidence, _ := metric["confidence_score"].(float64)
		reviewRequired, _ := metric["review_required"].(bool)
		quote := strings.TrimSpace(fmt.Sprintf("%s %s = %v %s", label, period, metric["value"], currency))
		rows = append(rows, map[string]any{
			"category":         "structured_financial_statement",
			"metric_id":        metricID,
			"label":            label,
			"source_file":      sourceFile,
			"raw_source_file":  metric["raw_source_file"],
			"sha256":           "",
			"page_number":      0,
			"line_number":      0,
			"table_name":       "financials/bilanco.json",
			"period":           period,
			"value":            metric["value"],
			"currency":         currency,
			"quote":            quote,
			"confidence_score": confidence,
			"manual_review":    reviewRequired,
			"certification":    metric["certification"],
		})
	}
	return rows
}

func structuredFinancialRawPath(rawDir string, year int) string {
	if strings.TrimSpace(rawDir) == "" || year <= 0 {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(rawDir, fmt.Sprintf("%04d-*.json", year)))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

func structuredFinancialCertificationReasons(quality domain.FinancialDataQuality, period domain.FinancialPeriod) []string {
	reasons := []string{}
	if quality.FinanciallyConsistent {
		reasons = append(reasons, "financial_reconciliation_passed")
	} else {
		reasons = append(reasons, "financial_reconciliation_requires_review")
	}
	if period.BacktestSafe {
		reasons = append(reasons, "period_available_at_or_publish_date_verified")
	} else {
		reasons = append(reasons, "period_backtest_safety_requires_review")
	}
	return reasons
}

func limitFinancialLineage(values []domain.DataLineageEvent, limit int) []domain.DataLineageEvent {
	if limit <= 0 || len(values) <= limit {
		return append([]domain.DataLineageEvent{}, values...)
	}
	return append([]domain.DataLineageEvent{}, values[len(values)-limit:]...)
}

func formatReportTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatReportTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatReportTime(*value)
}

func limitFinancialTableRows(values []kapingest.FinancialTableRow, limit int) []kapingest.FinancialTableRow {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func statementNormalizationStatus(raw *professional.KAPRawDataBundle) map[string]map[string]int {
	out := map[string]map[string]int{}
	ensure := func(statementType string) map[string]int {
		statementType = strings.TrimSpace(statementType)
		if statementType == "" {
			statementType = "unknown"
		}
		if _, ok := out[statementType]; !ok {
			out[statementType] = map[string]int{}
		}
		return out[statementType]
	}
	if raw == nil {
		return out
	}
	for _, fact := range raw.FinancialFacts {
		row := ensure(fact.StatementType)
		row["financial_facts"]++
		if fact.ReviewRequired {
			row["review_required"]++
		}
	}
	for _, table := range raw.FinancialTables {
		row := ensure(table.TableType)
		row["financial_tables"]++
		if table.ReviewRequired {
			row["review_required"]++
		}
	}
	return out
}

func financialCertificationSummary(raw *professional.KAPRawDataBundle) map[string]int {
	out := map[string]int{
		"financial_facts_total":      0,
		"financial_facts_certified":  0,
		"financial_facts_review":     0,
		"financial_facts_rejected":   0,
		"financial_tables_total":     0,
		"financial_tables_certified": 0,
		"financial_tables_review":    0,
		"financial_tables_rejected":  0,
	}
	if raw == nil {
		return out
	}
	for _, fact := range raw.FinancialFacts {
		out["financial_facts_total"]++
		switch fact.Certification.Status {
		case kapingest.EvidenceStatusCertified:
			out["financial_facts_certified"]++
		case kapingest.EvidenceStatusRejected:
			out["financial_facts_rejected"]++
		default:
			out["financial_facts_review"]++
		}
	}
	for _, table := range raw.FinancialTables {
		out["financial_tables_total"]++
		switch table.Certification.Status {
		case kapingest.EvidenceStatusCertified:
			out["financial_tables_certified"]++
		case kapingest.EvidenceStatusRejected:
			out["financial_tables_rejected"]++
		default:
			out["financial_tables_review"]++
		}
	}
	return out
}

func hasCertifiedFinancialRows(raw *professional.KAPRawDataBundle) bool {
	if raw == nil {
		return false
	}
	for _, fact := range raw.FinancialFacts {
		if fact.Certification.Status == kapingest.EvidenceStatusCertified && fact.Certification.AnalysisUsable {
			return true
		}
	}
	for _, table := range raw.FinancialTables {
		if table.Certification.Status == kapingest.EvidenceStatusCertified && table.Certification.AnalysisUsable {
			return true
		}
	}
	return false
}

func financialReviewQueue(raw *professional.KAPRawDataBundle, limit int) []map[string]any {
	if raw == nil {
		return nil
	}
	out := []map[string]any{}
	add := func(kind, id, sourceFile, period, reason string, confidence float64) {
		if limit > 0 && len(out) >= limit {
			return
		}
		out = append(out, map[string]any{
			"kind":             kind,
			"id":               id,
			"source_file":      sourceFile,
			"period":           period,
			"reason":           reason,
			"confidence_score": confidence,
		})
	}
	for _, fact := range raw.FinancialFacts {
		if fact.ReviewRequired || !fact.Certification.AnalysisUsable || fact.Confidence > 0 && fact.Confidence < 0.65 {
			add("financial_fact", fact.ID, fact.SourceFile, stringPtrValue(fact.Period), firstNonEmptyReport(strings.Join(fact.Certification.Reasons, "; "), strings.Join(fact.Warnings, "; "), "low_confidence_or_review_required"), fact.Confidence)
		}
	}
	for _, table := range raw.FinancialTables {
		if table.ReviewRequired || !table.Certification.AnalysisUsable || table.Confidence > 0 && table.Confidence < 0.65 {
			add("financial_table", table.ID, table.SourceFile, stringPtrValue(table.Period), firstNonEmptyReport(strings.Join(table.Certification.Reasons, "; "), strings.Join(table.Warnings, "; "), "low_confidence_or_review_required"), table.Confidence)
		}
	}
	return out
}

func riskLevelScore(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical", "very_high", "high":
		return 3
	case "medium", "moderate":
		return 2
	case "low":
		return 1
	default:
		return 2
	}
}

func riskSeverityLabel(score int) string {
	switch {
	case score >= 9:
		return "critical"
	case score >= 6:
		return "high"
	case score >= 3:
		return "medium"
	default:
		return "low"
	}
}

func riskOwner(category string) string {
	category = strings.ToLower(category)
	switch {
	case strings.Contains(category, "committee"):
		return "investment_committee"
	case strings.Contains(category, "financial"):
		return "equity_research"
	case strings.Contains(category, "kap"):
		return "research_and_legal"
	case strings.Contains(category, "data_quality"):
		return "data_engineering"
	case strings.Contains(category, "technical"):
		return "trading_desk"
	default:
		return "research"
	}
}

func actionableTradePlan(timeframe string, result analysis.SymbolAnalysis, plan ohlcv.TradePlan) map[string]any {
	action := "WAIT"
	executionReady := tradePlanExecutionReady(timeframe, result, plan)
	contextOnly := tradePlanContextOnly(timeframe)
	switch {
	case contextOnly:
		action = "CONTEXT_ONLY"
	case executionReady && strings.EqualFold(plan.Direction, "long"):
		action = "BUY_ON_CONFIRMATION"
	case executionReady && strings.EqualFold(plan.Direction, "short"):
		action = "SELL_OR_HEDGE_ON_CONFIRMATION"
	}
	reasoning := append([]string(nil), plan.Reasoning...)
	for i, reason := range reasoning {
		reasoning[i] = localize.Reason(reason)
	}
	return map[string]any{
		"action":                 action,
		"execution_ready":        executionReady,
		"context_only":           contextOnly,
		"decision_timeframe":     tradePlanDecisionTimeframe(timeframe),
		"blocked_by_report_gate": reportDecisionGateKnown(result) && !directBuySignalAllowed(result),
		"entry_zone":             map[string]float64{"min": plan.EntryMin, "max": plan.EntryMax},
		"stop_loss_zone":         map[string]float64{"stop": plan.StopLoss},
		"take_profit_zone":       map[string]float64{"target_1": plan.TakeProfit1, "target_2": plan.TakeProfit2},
		"risk_reward_ratio":      plan.RiskRewardRatio,
		"risk_reward_status":     riskRewardStatus(plan.RiskRewardRatio),
		"invalidating_condition": tradePlanInvalidation(plan),
		"signal_confidence":      plan.ConfidenceScore,
		"quality":                plan.Quality,
		"rejected":               plan.Rejected,
		"reject_reason":          localize.Reason(plan.RejectReason),
		"reasoning":              reasoning,
	}
}

func tradePlanExecutionReady(timeframe string, result analysis.SymbolAnalysis, plan ohlcv.TradePlan) bool {
	if !tradePlanDecisionTimeframe(timeframe) {
		return false
	}
	if reportDecisionGateKnown(result) && !directBuySignalAllowed(result) {
		return false
	}
	if plan.Rejected || plan.EntryMin <= 0 || plan.EntryMax <= 0 || plan.StopLoss <= 0 || plan.TakeProfit1 <= 0 {
		return false
	}
	if plan.EntryMin > plan.EntryMax || plan.RiskRewardRatio < 1.5 {
		return false
	}
	return strings.EqualFold(plan.Direction, "long") || strings.EqualFold(plan.Direction, "short")
}

func tradePlanDecisionTimeframe(timeframe string) bool {
	return strings.EqualFold(strings.TrimSpace(timeframe), "1D")
}

func tradePlanContextOnly(timeframe string) bool {
	return !tradePlanDecisionTimeframe(timeframe)
}

func riskRewardStatus(value float64) string {
	switch {
	case value >= 2.5:
		return "strong"
	case value >= 1.5:
		return "acceptable"
	case value > 0:
		return "weak"
	default:
		return "missing"
	}
}

func tradePlanInvalidation(plan ohlcv.TradePlan) string {
	if plan.Rejected {
		return firstNonEmptyReport(localize.Reason(plan.RejectReason), "Plan reddedildi; yeni teyit beklenmeli.")
	}
	if plan.StopLoss <= 0 {
		return "Stop seviyesi yok; işlem planı uygulanamaz."
	}
	switch strings.ToLower(plan.Direction) {
	case "long":
		return fmt.Sprintf("%.2f altı kapanış veya hacim teyidinin kaybolması planı geçersiz kılar.", plan.StopLoss)
	case "short":
		return fmt.Sprintf("%.2f üzeri kapanış veya momentum dönüşü planı geçersiz kılar.", plan.StopLoss)
	default:
		return "Yönsüz plan; teknik teyit gelmeden işlem yapılmaz."
	}
}

func technicalMomentumRegime(tf analysis.TimeframeAnalysis) string {
	switch {
	case tf.Indicators.RSI14 >= 70:
		return "overbought"
	case tf.Indicators.RSI14 > 0 && tf.Indicators.RSI14 <= 30:
		return "oversold"
	case tf.Indicators.MACD > tf.Indicators.MACDSignal:
		return "bullish"
	case tf.Indicators.MACD < tf.Indicators.MACDSignal:
		return "bearish"
	default:
		return "neutral"
	}
}

func technicalVolumeRegime(tf analysis.TimeframeAnalysis) string {
	switch {
	case tf.Indicators.ChaikinMoneyFlow20 > 0.05:
		return "accumulation"
	case tf.Indicators.ChaikinMoneyFlow20 < -0.05:
		return "distribution"
	case tf.LastVolume > 0:
		return "neutral"
	default:
		return "insufficient_data"
	}
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func statusBool(ok bool) string {
	if ok {
		return "pass"
	}
	return "needs_work"
}

func evidenceIndexStatus(rows []map[string]any, rawLoaded bool) string {
	if len(rows) > 0 && rawLoaded {
		return "indexed_with_raw_links"
	}
	if len(rows) > 0 {
		return "compact_index_only"
	}
	return "missing_evidence"
}

func jsonTagName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return strings.ToLower(field.Name)
	}
	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return strings.ToLower(field.Name)
	}
	return name
}

func isReportableKAPRawField(kind string) bool {
	switch kind {
	case "", "-", "computed", "symbol", "source_files", "counts", "warnings":
		return false
	default:
		return true
	}
}

func kapReportableFieldCount(value reflect.Value) int {
	if !value.IsValid() {
		return 0
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return value.Len()
	case reflect.Struct:
		if count, ok := reportableStructPrimaryCount(value); ok {
			return count
		}
		total := 0
		valueType := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := valueType.Field(i)
			if !field.IsExported() {
				continue
			}
			name := jsonTagName(field)
			if name == "warnings" || name == "source_files" {
				continue
			}
			child := value.Field(i)
			for child.Kind() == reflect.Interface || child.Kind() == reflect.Pointer {
				if child.IsNil() {
					child = reflect.Value{}
					break
				}
				child = child.Elem()
			}
			if !child.IsValid() {
				continue
			}
			if child.Kind() == reflect.Slice || child.Kind() == reflect.Array || child.Kind() == reflect.Map {
				total += child.Len()
			}
		}
		if total > 0 {
			return total
		}
		if !value.IsZero() {
			return 1
		}
	}
	return 0
}

func reportableStructPrimaryCount(value reflect.Value) (int, bool) {
	valueType := value.Type()
	nodes := -1
	edges := -1
	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		if !field.IsExported() {
			continue
		}
		name := jsonTagName(field)
		child := value.Field(i)
		for child.Kind() == reflect.Interface || child.Kind() == reflect.Pointer {
			if child.IsNil() {
				child = reflect.Value{}
				break
			}
			child = child.Elem()
		}
		if !child.IsValid() || (child.Kind() != reflect.Slice && child.Kind() != reflect.Array && child.Kind() != reflect.Map) {
			continue
		}
		switch name {
		case "assets", "documents":
			return child.Len(), true
		case "nodes":
			nodes = child.Len()
		case "edges":
			edges = child.Len()
		}
	}
	if nodes >= 0 || edges >= 0 {
		return maxInt(nodes, 0) + maxInt(edges, 0), true
	}
	return 0, false
}

func reportableObservedFields(valueType reflect.Type, limit int) []string {
	for valueType.Kind() == reflect.Pointer || valueType.Kind() == reflect.Slice || valueType.Kind() == reflect.Array {
		valueType = valueType.Elem()
	}
	if valueType.Kind() != reflect.Struct {
		return nil
	}
	fields := []string{}
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if !field.IsExported() {
			continue
		}
		name := jsonTagName(field)
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, name)
		if limit > 0 && len(fields) >= limit {
			break
		}
	}
	return fields
}

func reportableKindLabel(kind string) string {
	return strings.ReplaceAll(kind, "_", " ")
}

func reportableAnalysisRole(kind string) string {
	lower := strings.ToLower(kind)
	switch {
	case strings.Contains(lower, "financial"):
		return "financial_statement_analysis"
	case strings.Contains(lower, "ownership"):
		return "ownership_and_capital_structure"
	case strings.Contains(lower, "asset"):
		return "asset_portfolio_analysis"
	case strings.Contains(lower, "event"):
		return "kap_event_timeline"
	case strings.Contains(lower, "people"):
		return "governance_and_management"
	case strings.Contains(lower, "document"):
		return "source_document_evidence"
	case strings.Contains(lower, "knowledge"):
		return "cross_document_relationships"
	case strings.Contains(lower, "error") || strings.Contains(lower, "processed"):
		return "data_quality_control"
	default:
		return "source_evidence"
	}
}

func isReflectZero(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return true
		}
		value = value.Elem()
	}
	return value.IsZero()
}

func allKAPSourcePaths(raw *professional.KAPRawDataBundle, fallbackRawDocumentsPath string) map[string]string {
	paths := map[string]string{}
	if raw != nil {
		filesValue := reflect.ValueOf(raw.SourceFiles)
		filesType := filesValue.Type()
		for i := 0; i < filesType.NumField(); i++ {
			tag := jsonTagName(filesType.Field(i))
			if !strings.HasSuffix(tag, "_path") {
				continue
			}
			path := strings.TrimSpace(filesValue.Field(i).String())
			if path == "" {
				continue
			}
			paths[strings.TrimSuffix(tag, "_path")] = path
		}
	}
	if strings.TrimSpace(fallbackRawDocumentsPath) != "" {
		paths["raw_documents"] = fallbackRawDocumentsPath
	}
	return paths
}

func sourcePath(raw *professional.KAPRawDataBundle, kind string) string {
	if raw == nil {
		return ""
	}
	filesValue := reflect.ValueOf(raw.SourceFiles)
	filesType := filesValue.Type()
	wants := sourcePathTagCandidates(kind)
	for i := 0; i < filesType.NumField(); i++ {
		if stringInSlice(jsonTagName(filesType.Field(i)), wants) {
			return strings.TrimSpace(filesValue.Field(i).String())
		}
	}
	return ""
}

func sourcePathTagCandidates(kind string) []string {
	candidates := []string{kind + "_path"}
	if strings.HasPrefix(kind, "company_") {
		candidates = append(candidates, strings.TrimPrefix(kind, "company_")+"_path")
	}
	return candidates
}

func stringInSlice(value string, values []string) bool {
	for _, item := range values {
		if value == item {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func kapReportableCategoryCount(raw *professional.KAPRawDataBundle, kind string) int {
	if raw == nil {
		return 0
	}
	rawValue := reflect.ValueOf(*raw)
	rawType := rawValue.Type()
	for i := 0; i < rawType.NumField(); i++ {
		if jsonTagName(rawType.Field(i)) == kind {
			return kapReportableFieldCount(rawValue.Field(i))
		}
	}
	return 0
}

func rawInventoryItemCount(raw *professional.KAPRawDataBundle) int {
	if raw == nil {
		return 0
	}
	if raw.Counts.InventoryItems > 0 {
		return raw.Counts.InventoryItems
	}
	if raw.AssetInventory == nil {
		return 0
	}
	if raw.AssetInventory.AssetCount > 0 {
		return raw.AssetInventory.AssetCount
	}
	return len(raw.AssetInventory.Assets)
}

func kapReportableCategoryStatus(count int, sourcePath string, rawLoaded bool) string {
	switch {
	case !rawLoaded:
		return "raw_data_not_loaded"
	case count > 0 && sourcePath != "":
		return "fully_reportable"
	case count > 0:
		return "loaded_without_source_path"
	case sourcePath != "":
		return "source_available_no_rows"
	default:
		return "not_available"
	}
}

func incStringCount(counts map[string]int, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	counts[key]++
}

func topStringCountRows(counts map[string]int, limit int) []map[string]any {
	type row struct {
		Key   string
		Count int
	}
	items := make([]row, 0, len(counts))
	for key, count := range counts {
		items = append(items, row{Key: key, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Key < items[j].Key
		}
		return items[i].Count > items[j].Count
	})
	out := []map[string]any{}
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		out = append(out, map[string]any{"key": item.Key, "count": item.Count})
	}
	return out
}

func qualityBucket(score float64) string {
	switch {
	case score >= 0.85:
		return "0.85-1.00"
	case score >= 0.70:
		return "0.70-0.84"
	case score >= 0.50:
		return "0.50-0.69"
	case score > 0:
		return "0.01-0.49"
	default:
		return "unknown"
	}
}

func limitKAPPDFDocuments(values []professional.KAPPDFDocumentSummary, limit int) []professional.KAPPDFDocumentSummary {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitFinancialFacts(values []kapingest.ExtractedFinancialFact, limit int) []kapingest.ExtractedFinancialFact {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitDocumentFacts(values []kapingest.DocumentFact, limit int) []kapingest.DocumentFact {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitCorporateEvents(values []kapingest.ExtractedCorporateEvent, limit int) []kapingest.ExtractedCorporateEvent {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func businessModelTags(result analysis.SymbolAnalysis) []any {
	if result.Professional.RawKAPData == nil || result.Professional.RawKAPData.DocumentIndex == nil {
		return nil
	}
	out := []any{}
	for _, doc := range result.Professional.RawKAPData.DocumentIndex.Documents {
		for _, tag := range doc.BusinessModels {
			out = append(out, tag)
		}
		if len(out) >= 100 {
			return out
		}
	}
	return out
}

func tableCount(result analysis.SymbolAnalysis) int {
	if result.Professional.RawKAPData == nil {
		return 0
	}
	return len(result.Professional.RawKAPData.FinancialTables)
}

func factCount(result analysis.SymbolAnalysis) int {
	if result.Professional.RawKAPData == nil {
		return 0
	}
	return len(result.Professional.RawKAPData.FinancialFacts)
}

func firstNonEmptyReport(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueReportStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
