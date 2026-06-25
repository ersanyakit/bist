package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/professional"
)

const reportDataManifestFileName = "rapor_veri_manifesti.json"

func writeReportDataManifest(targetDir, equitiesDir string, result analysis.SymbolAnalysis) error {
	return WriteJSON(filepath.Join(targetDir, reportDataManifestFileName), reportDataManifest(targetDir, equitiesDir, result))
}

func reportDataManifest(targetDir, equitiesDir string, result analysis.SymbolAnalysis) map[string]any {
	confidence := reportConfidenceFor(result)
	structuredFinancials := loadFinancialStatementArtifactRows(equitiesDir, result.Symbol, 100)
	sourceIndex := reportManifestSourceEvidenceIndex(equitiesDir, result)
	return map[string]any{
		"schema_version": 1,
		"name":           "Rapor veri manifesti",
		"description":    "Bu dosya rapor uretilirken kullanilan ana veri kaynaklarini, dosya yollarini, kapsami ve kritik metrik kaynaklarini denetim icin ozetler.",
		"symbol":         result.Symbol,
		"asset_type":     result.AssetType,
		"company_name":   result.CompanyName,
		"analysis_date":  result.AnalysisDate,
		"currency":       result.Currency,
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"report_scope": map[string]any{
			"same_payload_as_report_html": true,
			"report_result_hydrated":      true,
			"note":                        "Manifest HTML raporun kullandigi hydrate edilmis final analiz verisiyle uretilir.",
		},
		"report_files": reportManifestGeneratedFiles(targetDir, result),
		"coverage": map[string]any{
			"score":     reportDataCoverageScore(result),
			"available": result.Professional.Coverage.Available,
			"missing":   result.Professional.Coverage.Missing,
			"warnings":  result.Professional.Coverage.Warnings,
		},
		"confidence": map[string]any{
			"data_coverage_score":       reportDataCoverageScore(result),
			"decision_model_confidence": confidence.Score,
			"confidence_items":          confidence.Items,
			"model_risk":                result.InvestorQA.ModelRisk,
			"decision":                  result.InvestorQA.DecisionLabel,
			"note":                      "Veri kapsami ile karar/model guveni ayridir; karar kapilari veya degerleme tutarsizligi basarisizsa karar guveni dusuk kalabilir.",
		},
		"primary_data_sources":    reportManifestPrimaryDataSources(equitiesDir, result, structuredFinancials),
		"critical_metric_sources": reportManifestCriticalMetricSources(result, sourceIndex),
		"source_evidence_summary": reportManifestSourceEvidence(sourceIndex, result),
		"audit_files":             reportManifestAuditFiles(targetDir, result),
	}
}

func reportManifestGeneratedFiles(targetDir string, result analysis.SymbolAnalysis) []map[string]any {
	names := []string{
		"analiz.json",
		"analysis.json",
		"ozet.md",
		"summary.md",
		"rapor.html",
		"report.html",
		"tek_bakis_ozet.png",
		"rapor.pdf",
		fmt.Sprintf("%s_analiz_%s.pdf", result.Symbol, result.AnalysisDate),
	}
	for _, timeframe := range sortedTimeframeKeys(result.Timeframes) {
		names = append(names,
			fmt.Sprintf("grafik_%s.png", timeframe),
			fmt.Sprintf("chart_%s.html", timeframe),
			fmt.Sprintf("grafik_karar_%s.png", timeframe),
			fmt.Sprintf("grafik_detay_%s.png", timeframe),
		)
	}
	out := make([]map[string]any, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if strings.TrimSpace(name) == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, reportManifestFileInfo(name, filepath.Join(targetDir, name)))
	}
	return out
}

func reportManifestPrimaryDataSources(equitiesDir string, result analysis.SymbolAnalysis, structured financialStatementArtifactRows) map[string]any {
	pro := result.Professional
	return map[string]any{
		"ohlcv_timeframes":                  reportManifestOHLCVTimeframes(result),
		"bist_official_bulletin":            reportManifestBISTBulletin(result),
		"bist_official_ohlcv_context":       reportManifestBISTOfficialContext(pro.BISTOfficial),
		"structured_financial_statements":   reportManifestStructuredFinancials(structured),
		"kap_pdf_ingest":                    reportManifestKAPPDFIngest(pro.KAPPDFIngest),
		"kap_raw_data_files":                reportManifestKAPRawFileSources(pro.RawKAPData, pro.KAPPDFIngest.RawDocumentsPath),
		"vap_free_float":                    reportManifestVAPFreeFloat(pro),
		"vap_index_portfolio":               reportManifestVAPIndexPortfolio(pro),
		"macro_gdp":                         reportManifestGDP(pro),
		"market_microstructure":             reportManifestMicrostructure(pro),
		"ml_forecast":                       result.MLForecast,
		"data_governance":                   reportManifestDataGovernance(pro),
		"financial_statement_candidate_dir": filepath.Join(equitiesDir, strings.ToUpper(strings.TrimSpace(result.Symbol)), "financials"),
	}
}

func reportManifestOHLCVTimeframes(result analysis.SymbolAnalysis) map[string]any {
	out := map[string]any{}
	for _, key := range sortedTimeframeKeys(result.Timeframes) {
		tf := result.Timeframes[key]
		count := tf.CandleCount
		if count <= 0 {
			count = len(tf.Candles)
		}
		item := map[string]any{
			"computed":     count > 0,
			"candle_count": count,
			"last_close":   tf.LastClose,
			"last_volume":  tf.LastVolume,
			"score":        tf.Score,
			"trend_bias":   tf.TrendBias,
			"source":       "analysis_timeframes.ohlcv_candles",
		}
		if len(tf.Candles) > 0 {
			first := tf.Candles[0]
			last := tf.Candles[len(tf.Candles)-1]
			item["first_bar"] = formatReportTime(first.Time)
			item["last_bar"] = formatReportTime(last.Time)
			item["last_open"] = last.EffectiveOpen()
			item["last_high"] = last.EffectiveHigh()
			item["last_low"] = last.EffectiveLow()
			item["last_close_from_candle"] = last.EffectiveClose()
			item["last_volume_from_candle"] = last.EffectiveVolume()
			item["adjusted_price_series"] = last.IsAdjusted
		}
		out[key] = item
	}
	return out
}

func reportManifestBISTBulletin(result analysis.SymbolAnalysis) map[string]any {
	b := result.BISTBulletin
	return map[string]any{
		"computed":                         b.Computed,
		"source":                           b.Source,
		"source_file":                      reportManifestFileInfo("bist_bulletin_source", b.Source),
		"record_count":                     b.RecordCount,
		"coverage_start":                   b.CoverageStart,
		"coverage_end":                     b.CoverageEnd,
		"latest_record":                    b.LatestRecord,
		"latest_record_source_file":        reportManifestFileInfo("bist_latest_record_source", b.LatestRecord.SourcePath),
		"forecast_actual_available":        b.ForecastActualAvailable,
		"forecast_actual_record":           b.ForecastActualRecord,
		"forecast_actual_source_file":      reportManifestFileInfo("bist_forecast_actual_source", b.ForecastActualRecord.SourcePath),
		"official_close_confirmed":         b.OfficialCloseConfirmed,
		"official_close_delta_pct":         b.OfficialCloseDeltaPct,
		"latest_observed_spread_bps":       b.LatestObservedSpreadBps,
		"latest_opening_session_volume":    b.LatestOpeningSessionVolume,
		"latest_closing_session_volume":    b.LatestClosingSessionVolume,
		"latest_vwap":                      b.LatestVWAP,
		"warnings":                         b.Warnings,
		"used_for_next_session_validation": b.ForecastActualAvailable || b.OfficialCloseConfirmed || b.RecordCount > 0,
	}
}

func reportManifestBISTOfficialContext(ctx professional.BISTOfficialContext) map[string]any {
	return map[string]any{
		"computed":               ctx.Computed,
		"source":                 ctx.Source,
		"as_of":                  formatReportTime(ctx.AsOf),
		"latest_date":            ctx.LatestDate,
		"candles":                ctx.Candles,
		"last_open":              ctx.LastOpen,
		"last_high":              ctx.LastHigh,
		"last_low":               ctx.LastLow,
		"last_close":             ctx.LastClose,
		"last_volume":            ctx.LastVolume,
		"analysis_close":         ctx.AnalysisClose,
		"close_difference_bps":   ctx.CloseDifferenceBps,
		"price_confirmed":        ctx.PriceConfirmed,
		"point_in_time_safe":     ctx.PointInTimeSafe,
		"summary":                ctx.Summary,
		"warnings":               ctx.Warnings,
		"source_path_candidates": []string{"data/bist/unprocessed", "data/bist/bulten_verileri"},
	}
}

func reportManifestStructuredFinancials(structured financialStatementArtifactRows) map[string]any {
	return map[string]any{
		"computed":        structured.RowCount > 0,
		"source_path":     structured.SourcePath,
		"source_file":     reportManifestFileInfo("financials/bilanco.json", structured.SourcePath),
		"raw_dir":         structured.RawDir,
		"raw_dir_status":  reportManifestFileInfo("financials/raw", structured.RawDir),
		"row_count":       structured.RowCount,
		"period_count":    structured.PeriodCount,
		"line_item_count": structured.LineItemCount,
		"latest_period":   structured.LatestPeriod,
		"quality":         structured.Quality,
		"lineage":         structured.Lineage,
		"warning":         structured.Warning,
		"sample_rows":     reportManifestLimitMaps(structured.Rows, 25),
	}
}

func reportManifestKAPPDFIngest(summary professional.KAPPDFIngestSummary) map[string]any {
	return map[string]any{
		"computed":                       summary.Computed,
		"symbol":                         summary.Symbol,
		"output_dir":                     summary.OutputDir,
		"raw_documents":                  reportManifestFileInfo("kap_raw_documents", summary.RawDocumentsPath),
		"processed_files":                reportManifestFileInfo("kap_processed_files", summary.ProcessedFilesPath),
		"errors":                         reportManifestFileInfo("kap_errors", summary.ErrorsPath),
		"source_pdf_count":               summary.SourcePDFCount,
		"source_unique_hash_count":       summary.SourceUniqueHashes,
		"duplicate_pdf_count":            summary.DuplicatePDFCount,
		"total_documents":                summary.TotalDocuments,
		"unique_processed":               summary.UniqueProcessed,
		"analysis_usable_count":          summary.AnalysisUsableCount,
		"review_required_count":          summary.ReviewRequiredCount,
		"rejected_count":                 summary.RejectedCount,
		"decision_relevant_documents":    summary.DecisionRelevantDocuments,
		"decision_relevant_usable_count": summary.DecisionRelevantUsableCount,
		"average_quality":                summary.AverageQuality,
		"type_counts":                    summary.TypeCounts,
		"important_documents_sample":     limitKAPPDFDocuments(summary.ImportantDocuments, 20),
		"summary":                        summary.Summary,
		"warnings":                       summary.Warnings,
		"source_status_note":             "PDF belgelerinin tam kanit satirlari source_evidence_index.json ve kap_pdf_* artifact dosyalarindadir.",
	}
}

func reportManifestKAPRawFileSources(raw *professional.KAPRawDataBundle, fallbackRawDocumentsPath string) map[string]any {
	paths := allKAPSourcePaths(raw, fallbackRawDocumentsPath)
	keys := make([]string, 0, len(paths))
	for key := range paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	files := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		files = append(files, reportManifestFileInfo(key, paths[key]))
	}
	out := map[string]any{
		"computed": raw != nil && raw.Computed,
		"files":    files,
	}
	if raw != nil {
		out["symbol"] = raw.Symbol
		out["financial_facts"] = len(raw.FinancialFacts)
		out["financial_tables"] = len(raw.FinancialTables)
		out["document_facts"] = len(raw.DocumentFacts)
		out["corporate_events"] = len(raw.CorporateEvents)
		out["people"] = len(raw.People)
		out["ownership_facts"] = len(raw.OwnershipFacts)
		out["raw_documents"] = len(raw.RawDocuments)
		out["processed_files"] = len(raw.ProcessedFiles)
		out["warnings"] = raw.Warnings
	}
	return out
}

func reportManifestVAPFreeFloat(pro professional.Report) map[string]any {
	v := pro.VAPFreeFloat
	return map[string]any{
		"computed":                    v.Computed,
		"source_path":                 v.SourcePath,
		"source_file":                 reportManifestFileInfo("vap_free_float_xlsx", v.SourcePath),
		"as_of":                       formatReportTime(v.AsOf),
		"latest_date":                 v.LatestDate,
		"observations":                v.Observations,
		"free_float_shares":           v.FreeFloatShares,
		"issuer_capital":              v.IssuerCapital,
		"free_float_ratio_pct":        v.FreeFloatRatioPct,
		"ratio_change_20d_pp":         v.RatioChange20DPP,
		"shares_change_20d_pct":       v.SharesChange20Pct,
		"liquidity_risk":              v.LiquidityRisk,
		"supply_signal":               v.SupplySignal,
		"point_in_time_safe":          v.PointInTimeSafe,
		"summary":                     v.Summary,
		"warnings":                    v.Warnings,
		"used_in_professional_report": v.Computed,
	}
}

func reportManifestVAPIndexPortfolio(pro professional.Report) map[string]any {
	v := pro.VAPIndexPortfolio
	return map[string]any{
		"computed":                    v.Computed,
		"source_path":                 v.SourcePath,
		"source_file":                 reportManifestFileInfo("vap_bist_index_portfolio", v.SourcePath),
		"as_of":                       formatReportTime(v.AsOf),
		"selected_index":              v.SelectedIndex,
		"latest_month":                v.LatestMonth,
		"portfolio_value_mtl":         v.PortfolioValueMTL,
		"change_1m_pct":               v.Change1MPct,
		"change_3m_pct":               v.Change3MPct,
		"change_12m_pct":              v.Change12MPct,
		"bist100_value_mtl":           v.BIST100ValueMTL,
		"relative_momentum_1m_pct":    v.RelativeMomentum,
		"signal":                      v.Signal,
		"point_in_time_safe":          v.PointInTimeSafe,
		"summary":                     v.Summary,
		"warnings":                    v.Warnings,
		"used_in_professional_report": v.Computed,
	}
}

func reportManifestGDP(pro professional.Report) map[string]any {
	g := pro.Market.GDP
	return map[string]any{
		"computed":              g.Computed,
		"source":                g.Source,
		"source_url":            g.SourceURL,
		"metadata_url":          g.MetadataURL,
		"fetched_at":            g.FetchedAt,
		"reference_year":        g.ReferenceYear,
		"latest_year":           g.LatestYear,
		"previous_year":         g.PreviousYear,
		"observation_lag_years": g.ObservationLagYears,
		"freshness_status":      g.FreshnessStatus,
		"score":                 g.Score,
		"regime":                g.Regime,
		"equity_impact":         g.EquityImpact,
		"required_caveats":      g.RequiredCaveats,
		"data_quality_warning":  g.DataQualityWarning,
	}
}

func reportManifestMicrostructure(pro professional.Report) map[string]any {
	m := pro.Market.Microstructure
	if m == nil {
		return map[string]any{"computed": false, "status": "missing"}
	}
	sourceFiles := make([]map[string]any, 0, len(m.SourceFiles))
	for _, path := range m.SourceFiles {
		sourceFiles = append(sourceFiles, reportManifestFileInfo(filepath.Base(path), path))
	}
	return map[string]any{
		"computed":                     m.Computed,
		"source":                       m.Source,
		"updated_at":                   formatReportTime(m.UpdatedAt),
		"status":                       m.Status,
		"score":                        m.Score,
		"source_files":                 sourceFiles,
		"quote_available":              m.Quote.Available,
		"order_book_available":         m.OrderBook.Available,
		"order_book_spread_bps":        m.OrderBook.SpreadBps,
		"order_book_imbalance_top5":    m.OrderBook.ImbalanceTop5,
		"depth_available":              m.Depth.Available,
		"akd_available":                m.BrokerageDistribution.Available,
		"takas_available":              m.Custody.Available,
		"equilibrium_available":        m.Equilibrium.Available,
		"equilibrium_price":            m.Equilibrium.Price,
		"automatic_order_ready":        m.Liquidity.AutomaticOrderReady,
		"automatic_order_blockers":     m.Liquidity.AutomaticOrderBlockers,
		"microstructure_complete":      m.Liquidity.MicrostructureComplete,
		"decision_usable":              m.Liquidity.DecisionUsable,
		"warnings":                     m.Warnings,
		"used_for_opening_adjustments": m.Equilibrium.Available || m.OrderBook.Available || m.Quote.Available,
	}
}

func reportManifestDataGovernance(pro professional.Report) map[string]any {
	g := pro.DataGovernance
	return map[string]any{
		"as_of":                               formatReportTime(g.AsOf),
		"data_mode":                           g.DataMode,
		"backtest_safe":                       g.BacktestSafe,
		"production_ready":                    g.ProductionReady,
		"availability_status":                 g.AvailabilityStatus,
		"source":                              g.Source,
		"currency":                            g.Currency,
		"latest_period":                       g.LatestPeriod,
		"latest_publish_date":                 formatReportTimePtr(g.LatestPublishDate),
		"latest_available_at":                 formatReportTimePtr(g.LatestAvailableAt),
		"publish_date_coverage":               g.PublishDateCoverage,
		"available_at_coverage":               g.AvailableAtCoverage,
		"verified_publish_date_count":         g.VerifiedPublishDateCount,
		"conservative_available_at_count":     g.ConservativeAvailableAtCount,
		"unsafe_availability_count":           g.UnsafeAvailabilityCount,
		"production_eligible_period_count":    g.ProductionEligiblePeriodCount,
		"production_quarantined_period_count": g.ProductionQuarantinedPeriodCount,
		"financially_consistent":              g.FinanciallyConsistent,
		"reconciliation_check_count":          g.ReconciliationCheckCount,
		"reconciliation_failure_count":        g.ReconciliationFailureCount,
		"statement_version_store_available":   g.StatementVersionStoreAvailable,
		"statement_version_count":             g.StatementVersionCount,
		"restatement_count":                   g.RestatementCount,
		"universe_source_available":           g.UniverseSourceAvailable,
		"survivorship_bias_risk":              g.SurvivorshipBiasRisk,
		"missing_publish_periods":             g.MissingPublishPeriods,
		"missing_available_at_periods":        g.MissingAvailableAtPeriods,
		"unsafe_backtest_periods":             g.UnsafeBacktestPeriods,
		"invalid_chronology_periods":          g.InvalidChronologyPeriods,
		"warnings":                            g.Warnings,
	}
}

func reportManifestCriticalMetricSources(result analysis.SymbolAnalysis, sourceIndex map[string]any) map[string]any {
	raw := result.Professional.RawKAPData
	return map[string]any{
		"next_session_forecast": reportManifestNextSessionForecast(result),
		"financial_metric_rows": reportManifestFinancialMetricRows(raw, result.Currency),
		"financial_readings":    reportManifestFinancialReadingRows(raw, result.Currency, isBankReport(result)),
		"source_evidence_rows":  reportManifestSourceEvidenceRows(sourceIndex, 50),
	}
}

func reportManifestNextSessionForecast(result analysis.SymbolAnalysis) map[string]any {
	f := analysis.ApplyNextSessionForecastPublishState(result.NextSessionForecast)
	return map[string]any{
		"computed":                           f.Computed,
		"forecast_for":                       f.ForecastFor,
		"model":                              f.Model,
		"last_close":                         f.LastClose,
		"point_forecast_publishable":         f.PointForecastPublishable,
		"point_forecast_status":              f.PointForecastStatus,
		"point_forecast_suppression_reason":  emptyStringAsNil(f.PointForecastSuppressionReason),
		"published_predicted_open":           nextSessionPublishedValue(f.PublishedPredictedOpen),
		"published_predicted_close":          nextSessionPublishedValue(f.PublishedPredictedClose),
		"raw_predicted_open":                 nextSessionScenarioForecastManifestValue(f, f.RawPredictedOpen),
		"raw_predicted_close":                nextSessionScenarioForecastManifestValue(f, f.RawPredictedClose),
		"tradable_predicted_open":            nextSessionScenarioForecastManifestValue(f, f.TradablePredictedOpen),
		"tradable_predicted_close":           nextSessionScenarioForecastManifestValue(f, f.TradablePredictedClose),
		"predicted_open":                     nextSessionPointForecastManifestValue(f, f.PredictedOpen),
		"predicted_close":                    nextSessionPointForecastManifestValue(f, f.PredictedClose),
		"scenario_predicted_open":            nextSessionScenarioForecastManifestValue(f, f.PredictedOpen),
		"scenario_predicted_close":           nextSessionScenarioForecastManifestValue(f, f.PredictedClose),
		"open_change_pct":                    nextSessionPointForecastManifestValue(f, f.OpenChangePct),
		"close_change_pct":                   nextSessionPointForecastManifestValue(f, f.CloseChangePct),
		"scenario_open_change_pct":           nextSessionScenarioForecastManifestValue(f, f.OpenChangePct),
		"scenario_close_change_pct":          nextSessionScenarioForecastManifestValue(f, f.CloseChangePct),
		"predicted_open_direction":           nextSessionPointForecastManifestText(f, f.PredictedOpenDirection),
		"predicted_close_direction":          nextSessionPointForecastManifestText(f, f.PredictedCloseDirection),
		"scenario_predicted_open_direction":  nextSessionScenarioForecastManifestText(f, f.PredictedOpenDirection),
		"scenario_predicted_close_direction": nextSessionScenarioForecastManifestText(f, f.PredictedCloseDirection),
		"direction_tolerance_pct":            nextSessionScenarioForecastManifestValue(f, f.DirectionTolerancePct),
		"open_p10":                           nextSessionScenarioForecastManifestValue(f, f.OpenP10),
		"open_p50":                           nextSessionScenarioForecastManifestValue(f, f.OpenP50),
		"open_p90":                           nextSessionScenarioForecastManifestValue(f, f.OpenP90),
		"close_p10":                          nextSessionScenarioForecastManifestValue(f, f.CloseP10),
		"close_p50":                          nextSessionScenarioForecastManifestValue(f, f.CloseP50),
		"close_p90":                          nextSessionScenarioForecastManifestValue(f, f.CloseP90),
		"upside_probability_pct":             nextSessionScenarioForecastManifestValue(f, f.UpsideProbabilityPct),
		"flat_probability_pct":               nextSessionScenarioForecastManifestValue(f, f.FlatProbabilityPct),
		"downside_probability_pct":           nextSessionScenarioForecastManifestValue(f, f.DownsideProbabilityPct),
		"distribution_samples":               nextSessionScenarioForecastManifestValue(f, float64(f.ForecastDistributionSamples)),
		"invalidation_level":                 nextSessionScenarioForecastManifestValue(f, f.InvalidationLevel),
		"invalidation_reason":                nextSessionScenarioForecastManifestText(f, f.InvalidationReason),
		"raw_expected_low":                   nextSessionScenarioForecastManifestValue(f, f.RawExpectedLow),
		"raw_expected_high":                  nextSessionScenarioForecastManifestValue(f, f.RawExpectedHigh),
		"tradable_expected_low":              nextSessionScenarioForecastManifestValue(f, f.TradableExpectedLow),
		"tradable_expected_high":             nextSessionScenarioForecastManifestValue(f, f.TradableExpectedHigh),
		"tick_size":                          f.TickSize,
		"rounding_method":                    f.RoundingMethod,
		"price_step_rule":                    f.PriceStepRule,
		"direction":                          nextSessionDirectionDisplayText(f),
		"direction_bias":                     nextSessionScenarioForecastManifestText(f, f.DirectionBias),
		"bias_strength":                      nextSessionScenarioForecastManifestText(f, f.BiasStrength),
		"confidence":                         f.Confidence,
		"confidence_label":                   f.ConfidenceLabel,
		"quality":                            f.Quality,
		"status":                             f.Status,
		"validation_status":                  f.ValidationStatus,
		"validation_source":                  f.ValidationSource,
		"actual_available":                   f.ActualAvailable,
		"actual_open":                        nextSessionActualValue(f, f.ActualOpen),
		"actual_close":                       nextSessionActualValue(f, f.ActualClose),
		"actual_source":                      emptyStringAsNil(f.ActualSource),
		"actual_source_path":                 emptyStringAsNil(f.ActualSourcePath),
		"actual": map[string]any{
			"available":              f.ActualAvailable,
			"source":                 emptyStringAsNil(f.ActualSource),
			"official_bulletin_file": emptyStringAsNil(f.ActualSourcePath),
			"open":                   nextSessionActualValue(f, f.ActualOpen),
			"close":                  nextSessionActualValue(f, f.ActualClose),
		},
		"official_result": map[string]any{
			"available":                 f.ActualAvailable,
			"authoritative":             f.ActualAvailable,
			"status":                    nextSessionOfficialResultStatus(f),
			"calculation_mode":          nextSessionOfficialCalculationMode(f),
			"open":                      nextSessionActualValue(f, f.ActualOpen),
			"close":                     nextSessionActualValue(f, f.ActualClose),
			"source":                    emptyStringAsNil(f.ActualSource),
			"official_bulletin_file":    emptyStringAsNil(f.ActualSourcePath),
			"predicted_open_for_audit":  nextSessionPointForecastManifestValue(f, f.PredictedOpen),
			"predicted_close_for_audit": nextSessionPointForecastManifestValue(f, f.PredictedClose),
		},
		"validation": map[string]any{
			"open_error_tl":                     nextSessionPointForecastManifestActualValue(f, f.OpenForecastErrorTL),
			"close_error_tl":                    nextSessionPointForecastManifestActualValue(f, f.CloseForecastErrorTL),
			"open_error_pct_vs_actual":          nextSessionPointForecastManifestActualValue(f, f.OpenAbsErrorPctVsActual),
			"close_error_pct_vs_actual":         nextSessionPointForecastManifestActualValue(f, f.CloseAbsErrorPctVsActual),
			"open_error_pct_vs_previous_close":  nextSessionPointForecastManifestActualValue(f, f.OpenAbsErrorPctVsPreviousClose),
			"close_error_pct_vs_previous_close": nextSessionPointForecastManifestActualValue(f, f.CloseAbsErrorPctVsPreviousClose),
			"open_signed_error_pct_vs_actual":   nextSessionPointForecastManifestActualValue(f, f.ActualOpenErrorPct),
			"close_signed_error_pct_vs_actual":  nextSessionPointForecastManifestActualValue(f, f.ActualCloseErrorPct),
			"open_direction_hit":                nextSessionOptionalBool(f.OpenDirectionHit),
			"close_direction_hit":               nextSessionOptionalBool(f.CloseDirectionHit),
		},
		"actual_open_error_pct":             nextSessionPointForecastManifestActualValue(f, f.ActualOpenErrorPct),
		"actual_close_error_pct":            nextSessionPointForecastManifestActualValue(f, f.ActualCloseErrorPct),
		"open_forecast_error_tl":            nextSessionPointForecastManifestActualValue(f, f.OpenForecastErrorTL),
		"close_forecast_error_tl":           nextSessionPointForecastManifestActualValue(f, f.CloseForecastErrorTL),
		"open_abs_error_pct_vs_actual":      nextSessionPointForecastManifestActualValue(f, f.OpenAbsErrorPctVsActual),
		"close_abs_error_pct_vs_actual":     nextSessionPointForecastManifestActualValue(f, f.CloseAbsErrorPctVsActual),
		"open_direction_hit":                nextSessionOptionalBool(f.OpenDirectionHit),
		"close_direction_hit":               nextSessionOptionalBool(f.CloseDirectionHit),
		"backtest_samples":                  f.BacktestSamples,
		"backtest_source":                   f.BacktestSource,
		"backtest_open_mae_pct":             f.BacktestOpenMAEPct,
		"backtest_close_mae_pct":            f.BacktestCloseMAEPct,
		"backtest_direction_hit_rate_pct":   f.BacktestDirectionHitRatePct,
		"opening_auction_equilibrium_price": f.OpeningAuctionEquilibriumPrice,
		"order_book_imbalance":              f.OrderBookImbalance,
		"auction_volume_pressure":           f.AuctionVolumePressure,
		"microstructure_adjustment":         f.MicrostructureAdjustment,
		"bias_reasons":                      f.BiasReasons,
		"warnings":                          f.Warnings,
	}
}

func reportManifestFinancialMetricRows(raw *professional.KAPRawDataBundle, currency string) []map[string]any {
	rows := kapPDFFinancialMetricRows(raw, currency)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{}
		if len(row) > 0 {
			item["metric"] = row[0]
		}
		if len(row) > 1 {
			item["statement"] = row[1]
		}
		if len(row) > 2 {
			item["period"] = row[2]
		}
		if len(row) > 3 {
			item["latest_value"] = row[3]
		}
		if len(row) > 4 {
			item["previous_value"] = row[4]
		}
		if len(row) > 5 {
			item["change"] = row[5]
		}
		if len(row) > 6 {
			item["source_evidence"] = row[6]
		}
		out = append(out, item)
	}
	return out
}

func reportManifestFinancialReadingRows(raw *professional.KAPRawDataBundle, currency string, isBank bool) []map[string]any {
	rows := kapPDFFinancialReadingRows(raw, currency, isBank)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{}
		if len(row) > 0 {
			item["title"] = row[0]
		}
		if len(row) > 1 {
			item["reading"] = row[1]
		}
		if len(row) > 2 {
			item["source_evidence"] = row[2]
		}
		out = append(out, item)
	}
	return out
}

func reportManifestSourceEvidenceIndex(equitiesDir string, result analysis.SymbolAnalysis) map[string]any {
	if isMarketOnlyResearchAsset(result.AssetType) {
		return map[string]any{
			"status": "market_only_asset",
			"note":   "Emtia/kripto raporlarında KAP/PDF evidence indeksi kullanılmaz.",
		}
	}
	return sourceEvidenceIndexArtifact(equitiesDir, result)
}

func reportManifestSourceEvidence(index map[string]any, result analysis.SymbolAnalysis) map[string]any {
	if isMarketOnlyResearchAsset(result.AssetType) {
		return map[string]any{
			"status": "market_only_asset",
			"note":   "Emtia/kripto raporlarında KAP/PDF evidence indeksi kullanılmaz.",
		}
	}
	return map[string]any{
		"status":               index["status"],
		"full_index_files":     index["full_index_files"],
		"indexed_sample_count": index["indexed_sample_count"],
		"evidence_row_sample":  reportManifestSourceEvidenceRows(index, 25),
		"note":                 index["note"],
	}
}

func reportManifestSourceEvidenceRows(index map[string]any, limit int) []map[string]any {
	rawRows, _ := index["evidence_rows"].([]map[string]any)
	return reportManifestLimitMaps(rawRows, limit)
}

func reportManifestAuditFiles(targetDir string, result analysis.SymbolAnalysis) []map[string]any {
	names := []string{
		"source_evidence_index.json",
		"kap_pdf_reportable_data_index.json",
		"kap_pdf_financial_analysis.json",
		"financial_statements_normalized.json",
		"financial_quality_report.json",
		"sector_metrics.json",
		"peer_comparison.json",
		"valuation_model.json",
		"investment_thesis.json",
		"decision_support_standard.json",
		"quant_risk_report.json",
		"stat_economic_report.json",
		"data_quality_report.json",
		"price_reconciliation_report.json",
		"corporate_action_audit.json",
		"point_in_time_lineage.json",
		"factor_model_report.json",
		"relative_strength_report.json",
		"active_return_decomposition.json",
		"volatility_regime_report.json",
		"tail_risk_report.json",
		"stress_test_report.json",
		"macro_sensitivity_report.json",
		"macro_regime_report.json",
		"macro_scenario_stress.json",
		"financial_quality_scorecard.json",
		"accounting_risk_report.json",
		"sector_specific_financial_report.json",
		"valuation_ensemble_report.json",
		"valuation_sensitivity_table.json",
		"fair_value_bridge.json",
		"event_study_report.json",
		"kap_materiality_score.json",
		"news_event_impact_report.json",
		"forecast_validation_report.json",
		"model_monitoring_report.json",
		"champion_challenger_report.json",
		"liquidity_impact_report.json",
		"portfolio_fit_report.json",
		"position_capacity_report.json",
		"decision_audit_trail.json",
		"model_registry_snapshot.json",
		"production_readiness_report.json",
		"human_review_queue.json",
		"risk_matrix.json",
		"catalyst_calendar.json",
		"technical_trade_plan.json",
		"investment_committee_memo.json",
		"quality_control_report.json",
		"market_research_context.json",
	}
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		info := reportManifestFileInfo(name, filepath.Join(targetDir, name))
		if exists, _ := info["exists"].(bool); exists || isMarketOnlyResearchAsset(result.AssetType) || strings.Contains(name, "kap_") || strings.Contains(name, "financial") {
			out = append(out, info)
		}
	}
	return out
}

func reportManifestFileInfo(label, path string) map[string]any {
	path = strings.TrimSpace(path)
	info := map[string]any{
		"label":  label,
		"path":   path,
		"exists": false,
	}
	if path == "" {
		info["status"] = "path_missing"
		return info
	}
	stat, err := os.Stat(path)
	if err != nil {
		info["status"] = "not_found"
		info["error"] = err.Error()
		return info
	}
	info["exists"] = true
	info["status"] = "ok"
	info["bytes"] = stat.Size()
	info["modified_at"] = stat.ModTime().UTC().Format(time.RFC3339)
	info["is_dir"] = stat.IsDir()
	return info
}

func reportManifestLimitMaps(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func nextSessionActualValue(forecast analysis.NextSessionForecast, value float64) any {
	if !forecast.ActualAvailable {
		return nil
	}
	return value
}

func nextSessionPointForecastManifestValue(forecast analysis.NextSessionForecast, value float64) any {
	if !forecast.PointForecastPublishable || value == 0 {
		return nil
	}
	return value
}

func nextSessionPointForecastManifestText(forecast analysis.NextSessionForecast, value string) any {
	value = strings.TrimSpace(value)
	if !forecast.PointForecastPublishable || value == "" {
		return nil
	}
	return value
}

func nextSessionScenarioForecastManifestValue(forecast analysis.NextSessionForecast, value float64) any {
	if !nextSessionForwardScenarioPublishable(forecast) || value == 0 {
		return nil
	}
	return value
}

func nextSessionScenarioForecastManifestText(forecast analysis.NextSessionForecast, value string) any {
	value = strings.TrimSpace(value)
	if !nextSessionForwardScenarioPublishable(forecast) || value == "" {
		return nil
	}
	return value
}

func nextSessionPointForecastManifestActualValue(forecast analysis.NextSessionForecast, value float64) any {
	if !forecast.PointForecastPublishable || !forecast.ActualAvailable {
		return nil
	}
	return value
}

func nextSessionPublishedValue(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nextSessionOfficialResultStatus(forecast analysis.NextSessionForecast) string {
	if forecast.ActualAvailable {
		return "official_actual_observed"
	}
	return "pending_actual_session"
}

func nextSessionOfficialCalculationMode(forecast analysis.NextSessionForecast) string {
	if forecast.ActualAvailable {
		return "bist_official_actual_overrides_forecast_for_observed_session"
	}
	return "point_in_time_forecast_only"
}

func nextSessionOptionalBool(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func emptyStringAsNil(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
