// internal/storage/writer.go
package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/kapingest"
	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/chart"
	"hissebot/internal/ta/ensemble"
	"hissebot/internal/ta/features"
	"hissebot/internal/ta/localize"
	taml "hissebot/internal/ta/ml"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/validation"
	"hissebot/internal/ta/vapcontext"
)

type ReportWriter interface {
	WriteAnalysis(ctx context.Context, equitiesDir string, result analysis.SymbolAnalysis) error
}

type FileReportWriter struct {
	IncludeRawKAPData bool
	RenderPDF         bool
	RenderMedia       bool
}

type ReportWriterOptions struct {
	IncludeRawKAPData bool
	RenderPDF         bool
	SkipMedia         bool
}

func NewReportWriter() *FileReportWriter {
	return NewReportWriterWithOptions(ReportWriterOptions{
		IncludeRawKAPData: envBool("HISSEBOT_INCLUDE_RAW_KAP_DATA"),
		RenderPDF:         !envBool("HISSEBOT_SKIP_PDF"),
		SkipMedia:         envBool("HISSEBOT_SKIP_MEDIA"),
	})
}

func NewReportWriterWithOptions(opts ReportWriterOptions) *FileReportWriter {
	renderMedia := !opts.SkipMedia
	if opts.RenderPDF {
		renderMedia = true
	}
	return &FileReportWriter{
		IncludeRawKAPData: opts.IncludeRawKAPData,
		RenderPDF:         opts.RenderPDF,
		RenderMedia:       renderMedia,
	}
}

func (w *FileReportWriter) WriteAnalysis(ctx context.Context, equitiesDir string, result analysis.SymbolAnalysis) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write analysis canceled: %w", err)
	}
	result = hydrateDerivedReportFields(equitiesDir, result)
	canonical := result
	reportResult := result
	if shouldLoadKAPRawDataForReport(equitiesDir, result) {
		bundle := loadKAPRawDataBundle(equitiesDir, result)
		if bundle.Computed {
			bundle = hydrateStructuredFinancialFacts(equitiesDir, result, bundle)
			reportResult.Professional.RawKAPData = &bundle
			reportResult = refreshReportDecisionFields(reportResult)
			canonical = reportResult
			if w.IncludeRawKAPData {
				canonical.Professional.RawKAPData = &bundle
			} else {
				canonical.Professional.RawKAPData = nil
			}
		}
	}
	targetDir := AnalysisDirForAsset(equitiesDir, reportResult.AssetType, reportResult.Symbol, reportResult.AnalysisDate)
	if err := EnsureDir(targetDir); err != nil {
		return fmt.Errorf("ensure analysis directory: %w", err)
	}
	if err := WriteJSON(filepath.Join(targetDir, "analiz.json"), turkishAnalysis(reportResult)); err != nil {
		return fmt.Errorf("write analysis json: %w", err)
	}
	if err := WriteJSON(filepath.Join(targetDir, "analysis.json"), sanitizeUnpublishedPointForecasts(canonical)); err != nil {
		return fmt.Errorf("write canonical analysis json: %w", err)
	}
	if err := writeResearchArtifacts(targetDir, equitiesDir, reportResult); err != nil {
		return fmt.Errorf("write research artifacts: %w", err)
	}
	if err := WriteText(filepath.Join(targetDir, "ozet.md"), markdownSummary(reportResult)); err != nil {
		return fmt.Errorf("write summary markdown: %w", err)
	}
	if err := WriteText(filepath.Join(targetDir, "summary.md"), markdownSummary(reportResult)); err != nil {
		return fmt.Errorf("write canonical summary markdown: %w", err)
	}
	reportHTML := professionalReportHTML(reportResult)
	if err := WriteText(filepath.Join(targetDir, "rapor.html"), reportHTML); err != nil {
		return fmt.Errorf("write professional report html: %w", err)
	}
	if err := WriteText(filepath.Join(targetDir, "report.html"), reportHTML); err != nil {
		return fmt.Errorf("write canonical professional report html: %w", err)
	}
	if !w.RenderMedia {
		if err := writeReportDataManifest(targetDir, equitiesDir, reportResult); err != nil {
			return fmt.Errorf("write report data manifest: %w", err)
		}
		return nil
	}
	if err := removeChartArtifacts(targetDir); err != nil {
		return fmt.Errorf("remove stale chart artifacts: %w", err)
	}
	reportImages := []reportImage{}
	oneLookPNG, err := oneLookSummaryPNG(reportResult)
	if err != nil {
		return fmt.Errorf("render one-look summary: %w", err)
	}
	if err := WriteBinary(filepath.Join(targetDir, "tek_bakis_ozet.png"), oneLookPNG); err != nil {
		return fmt.Errorf("write one-look summary: %w", err)
	}
	reportImages = append(reportImages, reportImage{
		Timeframe: "00_SUMMARY",
		Title:     fmt.Sprintf("%s Tek Bakış Yatırımcı Özeti", result.Symbol),
		Subtitle:  "Sonuç odaklı tek bakış özeti",
		PNG:       oneLookPNG,
	})
	for timeframe, png := range result.Charts {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("write chart canceled: %w", err)
		}
		if err := WriteBinary(filepath.Join(targetDir, fmt.Sprintf("grafik_%s.png", timeframe)), png); err != nil {
			return fmt.Errorf("write chart %s: %w", timeframe, err)
		}
		reportImages = append(reportImages, reportImage{
			Timeframe: timeframe,
			Title:     fmt.Sprintf("%s %s Teknik Fiyat Grafiği", result.Symbol, localize.Timeframe(timeframe)),
			Subtitle:  "Fiyat grafiği - " + localize.Timeframe(timeframe),
			PNG:       png,
		})
		tf, ok := result.Timeframes[timeframe]
		if ok {
			if err := WriteText(filepath.Join(targetDir, fmt.Sprintf("chart_%s.html", timeframe)), chartHTML(result, tf, fmt.Sprintf("grafik_%s.png", timeframe))); err != nil {
				return fmt.Errorf("write html chart %s: %w", timeframe, err)
			}
		}
	}
	decisionRenderer := chart.NewDecisionRenderer()
	detailRenderer := chart.NewDetailRenderer()
	for _, timeframe := range sortedTimeframeKeys(result.Timeframes) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("write decision chart canceled: %w", err)
		}
		tf := result.Timeframes[timeframe]
		png, err := decisionRenderer.RenderPNG(ctx, chart.DecisionRenderInput{
			Symbol:            result.Symbol,
			CompanyName:       result.CompanyName,
			AnalysisDate:      result.AnalysisDate,
			Currency:          result.Currency,
			Timeframe:         timeframe,
			LastClose:         tf.LastClose,
			LastVolume:        tf.LastVolume,
			OverallScore:      result.OverallScore,
			OverallBias:       result.OverallBias,
			TimeframeScore:    tf.Score,
			TrendBias:         tf.TrendBias,
			Indicators:        tf.Indicators,
			Patterns:          tf.Patterns,
			SupportLevels:     tf.SupportLevels,
			ResistanceLevels:  tf.ResistanceLevels,
			NearestSupport:    tf.NearestSupport,
			NearestResistance: tf.NearestResistance,
			TradePlan:         tf.TradePlan,
			Professional:      result.Professional,
			Behavioral:        result.Behavioral,
			Disclaimer:        result.Disclaimer,
		})
		if err != nil {
			return fmt.Errorf("render decision chart %s: %w", timeframe, err)
		}
		if err := WriteBinary(filepath.Join(targetDir, fmt.Sprintf("grafik_karar_%s.png", timeframe)), png); err != nil {
			return fmt.Errorf("write decision chart %s: %w", timeframe, err)
		}
		reportImages = append(reportImages, reportImage{
			Timeframe: timeframe,
			Title:     fmt.Sprintf("%s %s Karar Grafiği", result.Symbol, localize.Timeframe(timeframe)),
			Subtitle:  "Karar grafiği - " + localize.Timeframe(timeframe),
			PNG:       png,
		})
		detailPNG, err := detailRenderer.RenderPNG(ctx, chart.DetailRenderInput{
			Symbol:           result.Symbol,
			CompanyName:      result.CompanyName,
			AnalysisDate:     result.AnalysisDate,
			Timeframe:        timeframe,
			LastClose:        tf.LastClose,
			TimeframeScore:   tf.Score,
			TrendBias:        tf.TrendBias,
			IndicatorSignals: tf.IndicatorSignals,
			Patterns:         tf.Patterns,
			PatternScans:     tf.PatternScans,
			Disclaimer:       result.Disclaimer,
		})
		if err != nil {
			return fmt.Errorf("render detail chart %s: %w", timeframe, err)
		}
		if err := WriteBinary(filepath.Join(targetDir, fmt.Sprintf("grafik_detay_%s.png", timeframe)), detailPNG); err != nil {
			return fmt.Errorf("write detail chart %s: %w", timeframe, err)
		}
		reportImages = append(reportImages, reportImage{
			Timeframe: timeframe,
			Title:     fmt.Sprintf("%s %s İndikatör/Formasyon Detay Grafiği", result.Symbol, localize.Timeframe(timeframe)),
			Subtitle:  "Detay grafiği - " + localize.Timeframe(timeframe),
			PNG:       detailPNG,
		})
	}
	sort.SliceStable(reportImages, func(i, j int) bool {
		leftType := reportImageSortRank(reportImages[i])
		rightType := reportImageSortRank(reportImages[j])
		if leftType != rightType {
			return leftType < rightType
		}
		if reportImages[i].Timeframe == reportImages[j].Timeframe {
			return reportImages[i].Title < reportImages[j].Title
		}
		leftRank := timeframeSortRank(reportImages[i].Timeframe)
		rightRank := timeframeSortRank(reportImages[j].Timeframe)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return reportImages[i].Timeframe < reportImages[j].Timeframe
	})
	if !w.RenderPDF {
		if err := writeReportDataManifest(targetDir, equitiesDir, reportResult); err != nil {
			return fmt.Errorf("write report data manifest: %w", err)
		}
		return nil
	}
	reportPDF, err := professionalReportPDF(canonical, reportImages)
	if err != nil {
		return fmt.Errorf("render professional report pdf: %w", err)
	}
	if err := WriteBinary(filepath.Join(targetDir, fmt.Sprintf("%s_analiz_%s.pdf", result.Symbol, result.AnalysisDate)), reportPDF); err != nil {
		return fmt.Errorf("write professional report pdf: %w", err)
	}
	if err := WriteBinary(filepath.Join(targetDir, "rapor.pdf"), reportPDF); err != nil {
		return fmt.Errorf("write professional report pdf alias: %w", err)
	}
	if err := writeReportDataManifest(targetDir, equitiesDir, reportResult); err != nil {
		return fmt.Errorf("write report data manifest: %w", err)
	}
	return nil
}

func hydrateDerivedReportFields(equitiesDir string, result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	result = analysis.ApplyBISTEquityTickSizeToTechnicalLevels(result)
	result = hydrateNextSessionForecast(result)
	result = hydrateVAPContext(equitiesDir, result)
	result = hydrateBISTOfficialCoverage(result)
	result = analysis.ApplyFundamentalContextToNextSessionForecast(result)
	result = analysis.ApplyBISTEquityTickSizeToTechnicalLevels(result)
	result = analysis.ApplyNextSessionForecastQualityContext(result)
	result = refreshReportDecisionFields(result)
	return hydrateMLForecast(equitiesDir, result)
}

func hydrateMLForecast(equitiesDir string, result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	cfg := taml.LoadRuntimeConfig("")
	report := taml.ForecastReport{
		Enabled:           cfg.ML.Enabled,
		ShadowMode:        cfg.ML.ShadowMode,
		FeatureSetVersion: cfg.ML.FeatureSetVersion,
		Fallback: taml.ReportFallback{
			Used:   true,
			Reason: "ml_disabled",
		},
		TradeGate: taml.ReportTradeGate{Allowed: false, Action: "no_trade", Confidence: "low", Reasons: []string{"ml_disabled"}},
	}
	if !cfg.ML.Enabled {
		result.MLForecast = report
		return result
	}
	daily, ok := result.Timeframes["1D"]
	if !ok || len(daily.Candles) == 0 {
		report.Fallback.Reason = "daily_candles_missing"
		report.TradeGate.Reasons = []string{"daily_candles_missing"}
		result.MLForecast = report
		return result
	}
	asOf := parseReportAnalysisDate(result.AnalysisDate)
	if asOf.IsZero() {
		asOf = daily.Candles[len(daily.Candles)-1].Time
	}
	fv := features.BuildFromCandles(result.Symbol, asOf, "1d", cfg.ML.FeatureSetVersion, daily.Candles)
	if fv.Categorical == nil {
		fv.Categorical = map[string]string{}
	}
	fv.Categorical["asset_type"] = result.AssetType
	if len(fv.Quality.LeakageFlags) == 0 {
		fv.Quality.LeakageFlags = features.LeakageFlags(fv)
	}
	deterministic := deterministicInputFromForecast(result.NextSessionForecast)
	selection := taml.SelectRuntimeModels(filepath.Dir(equitiesDir), cfg, true)
	ens := ensemble.RunShadow(context.Background(), fv, deterministic, selection.Models, cfg)
	if selection.Fallback.Used {
		ens.Fallback = selection.Fallback
	}
	ens.Warnings = append(ens.Warnings, selection.Warnings...)
	report = ensemble.ToForecastReport(ens, fv, cfg)
	if report.Debug == nil {
		report.Debug = map[string]any{}
	}
	report.Debug["model_registry_path"] = selection.RegistryPath
	report.Debug["model_artifact_selected"] = selection.Selection.Found
	if !fv.Quality.IsTradable {
		report.TradeGate.Allowed = false
		report.TradeGate.Action = "no_trade"
		report.TradeGate.Confidence = "low"
		report.TradeGate.Reasons = appendUniqueString(report.TradeGate.Reasons, "feature_quality_not_tradable")
	}
	result.MLForecast = report
	return result
}

func deterministicInputFromForecast(f analysis.NextSessionForecast) ensemble.DeterministicInput {
	f = analysis.ApplyNextSessionForecastPublishState(f)
	metrics := validation.Metrics{
		MAE:               f.BacktestCloseMAEPct / 100,
		MAPE:              f.BacktestCloseMAEPct / 100,
		DirectionAccuracy: f.BacktestDirectionHitRatePct / 100,
		Samples:           f.BacktestSamples,
	}
	if !f.PointForecastPublishable || f.PublishedPredictedOpen == nil || f.PublishedPredictedClose == nil {
		return ensemble.DeterministicInput{ValidationMetrics: metrics}
	}
	ret := 0.0
	if f.LastClose > 0 && *f.PublishedPredictedClose > 0 {
		ret = *f.PublishedPredictedClose/f.LastClose - 1
	}
	return ensemble.DeterministicInput{
		PredictedOpen:     *f.PublishedPredictedOpen,
		PredictedClose:    *f.PublishedPredictedClose,
		ExpectedReturn:    ret,
		Direction:         analysis.NextSessionDirectionFromReturn(ret),
		Confidence:        f.Confidence,
		Model:             f.Model,
		ValidationMetrics: metrics,
	}
}

func parseReportAnalysisDate(text string) time.Time {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, text); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func sanitizeUnpublishedPointForecasts(result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	result.NextSessionForecast = sanitizeUnpublishedPointForecast(result.NextSessionForecast)
	if len(result.Timeframes) > 0 {
		for key, timeframe := range result.Timeframes {
			timeframe.NextSessionForecast = sanitizeUnpublishedPointForecast(timeframe.NextSessionForecast)
			result.Timeframes[key] = timeframe
		}
	}
	return result
}

func sanitizeUnpublishedPointForecast(forecast analysis.NextSessionForecast) analysis.NextSessionForecast {
	if !forecast.Computed {
		return forecast
	}
	gated := analysis.ApplyNextSessionForecastPublishState(forecast)
	forecast.PointForecastPublishable = gated.PointForecastPublishable
	forecast.PointForecastStatus = gated.PointForecastStatus
	forecast.PointForecastSuppressionReason = gated.PointForecastSuppressionReason
	forecast.PublishedPredictedOpen = gated.PublishedPredictedOpen
	forecast.PublishedPredictedClose = gated.PublishedPredictedClose
	if gated.PointForecastPublishable {
		return forecast
	}
	if !gated.ActualAvailable {
		return forecast
	}
	forecast.PredictedOpen = 0
	forecast.PredictedClose = 0
	forecast.RawPredictedOpen = 0
	forecast.RawPredictedClose = 0
	forecast.TradablePredictedOpen = 0
	forecast.TradablePredictedClose = 0
	forecast.OpenChangePct = 0
	forecast.CloseChangePct = 0
	forecast.PredictedOpenDirection = ""
	forecast.PredictedCloseDirection = ""
	forecast.DirectionTolerancePct = 0
	forecast.UpsideProbabilityPct = 0
	forecast.FlatProbabilityPct = 0
	forecast.DownsideProbabilityPct = 0
	forecast.DirectionBias = ""
	forecast.BiasStrength = ""
	return forecast
}

func refreshReportDecisionFields(result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	result = normalizeReportCoverageScores(result)
	result = applyKAPRawFinancialIntegrityGate(result)
	result.InvestorQA = analysis.BuildInvestorQAReport(result)
	result.DecisionClassification = analysis.ClassifyDecision(result)
	result = analysis.ApplyDecisionClassification(result)
	result.DecisionSupport = analysis.BuildDecisionSupport(result)
	result = analysis.ApplyNextSessionForecastQualityContext(result)
	return result
}

func applyKAPRawFinancialIntegrityGate(result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	raw := result.Professional.RawKAPData
	if raw == nil || !raw.Computed {
		return result
	}
	rows := kapPDFFinancialReadingRows(raw, result.Currency, isBankReport(result))
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		if !kapPDFFinancialReadingBlocksIntegrity(row[0], row[1]) {
			continue
		}
		result.Professional.DataGovernance.FinanciallyConsistent = false
		result.Professional.DataGovernance.Warnings = appendUniqueString(result.Professional.DataGovernance.Warnings, "kap_pdf_financial_reading_requires_review")
		return result
	}
	return result
}

func kapPDFFinancialReadingBlocksIntegrity(title, reading string) bool {
	text := normalizeFinancialText(title + " " + reading)
	if !strings.Contains(text, "hesaplanmadi") {
		return false
	}
	return containsAnyFinancialText(text, []string{
		"ozkaynak aktif", "cari oran", "net borc", "yukumluluk ozkaynak",
		"faaliyet marji", "net marj", "kredi mevduat",
	})
}

func hydrateNextSessionForecast(result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	if forecast, ok := symbolNextSessionForecast(result); ok {
		result.NextSessionForecast = forecast
		return result
	}
	daily, ok := result.Timeframes["1D"]
	if !ok || len(daily.Candles) == 0 {
		return result
	}
	bias := daily.TrendBias
	if strings.TrimSpace(bias) == "" {
		bias = "neutral"
	}
	forecast := analysis.ComputeNextSessionForecast(daily.Candles, daily.Indicators, bias, result.AssetType)
	if !forecast.Computed || forecast.PredictedOpen <= 0 || forecast.PredictedClose <= 0 {
		return result
	}
	daily.NextSessionForecast = forecast
	result.Timeframes["1D"] = daily
	result.NextSessionForecast = forecast
	return result
}

func hydrateVAPContext(equitiesDir string, result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	if !isEquityResult(result) {
		return result
	}
	asOf := reportAsOf(result)
	vapDir := defaultVAPDir(equitiesDir)
	if !result.Professional.VAPFreeFloat.Computed {
		freeFloat := vapcontext.LoadFreeFloat(vapDir, result.Symbol, asOf)
		if freeFloat.Computed || strings.TrimSpace(freeFloat.Summary) != "" {
			result.Professional.VAPFreeFloat = freeFloat
			result.Professional.Coverage = updateVAPCoverage(result.Professional.Coverage, "vap_free_float_xlsx", freeFloat.Computed, freeFloat.Warnings)
		}
	}
	if !result.Professional.VAPIndexPortfolio.Computed {
		portfolio := vapcontext.LoadIndexPortfolio(filepath.Join(vapDir, "bist_endeks_portfoy.json"), result.Professional.Company.Sector, result.Professional.Company.Industry, asOf)
		if portfolio.Computed || strings.TrimSpace(portfolio.Summary) != "" {
			result.Professional.VAPIndexPortfolio = portfolio
			result.Professional.Coverage = updateVAPCoverage(result.Professional.Coverage, "vap_bist_index_portfolio", portfolio.Computed, portfolio.Warnings)
		}
	}
	return result
}

func hydrateBISTOfficialCoverage(result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	if !isEquityResult(result) {
		return result
	}
	const key = "bist_official_unprocessed_ohlcv"
	if result.BISTBulletin.Computed {
		result.Professional.Coverage = updateReportCoverage(result.Professional.Coverage, key, true, result.BISTBulletin.Warnings)
		result.Professional.Coverage.Warnings = removeStringValueLocal(result.Professional.Coverage.Warnings, "bist_official_ohlcv_missing")
		result.Professional.Coverage.Warnings = removeStringValueLocal(result.Professional.Coverage.Warnings, "bist_official_ohlcv_read_error")
		return result
	}
	if len(result.BISTBulletin.Warnings) > 0 {
		result.Professional.Coverage = updateReportCoverage(result.Professional.Coverage, key, false, result.BISTBulletin.Warnings)
	}
	return result
}

type structuredFinancialFactSpec struct {
	MetricID    string
	Key         string
	Label       string
	Statement   string
	SourceLabel string
}

func hydrateStructuredFinancialFacts(equitiesDir string, result analysis.SymbolAnalysis, bundle professional.KAPRawDataBundle) professional.KAPRawDataBundle {
	rows := loadFinancialStatementArtifactRows(equitiesDir, result.Symbol, 0)
	if rows.RowCount == 0 {
		return bundle
	}
	specs := map[string]structuredFinancialFactSpec{
		"1A":  {MetricID: "1A", Key: "current_assets", Label: "Dönen Varlıklar", Statement: "balance_sheet"},
		"1AA": {MetricID: "1AA", Key: "cash", Label: "Nakit ve Nakit Benzerleri", Statement: "balance_sheet"},
		"1BL": {MetricID: "1BL", Key: "total_assets", Label: "TOPLAM VARLIKLAR", Statement: "balance_sheet"},
		"2A":  {MetricID: "2A", Key: "current_liabilities", Label: "Kısa Vadeli Yükümlülükler", Statement: "balance_sheet"},
		"2B":  {MetricID: "2B", Key: "non_current_liabilities", Label: "Uzun Vadeli Yükümlülükler", Statement: "balance_sheet"},
		"2N":  {MetricID: "2N", Key: "equity", Label: "Özkaynaklar", Statement: "balance_sheet"},
		"3C":  {MetricID: "3C", Key: "revenue", Label: "Satış Gelirleri", Statement: "income_statement"},
		"3D":  {MetricID: "3D", Key: "gross_profit", Label: "BRÜT KAR (ZARAR)", Statement: "income_statement"},
		"3DF": {MetricID: "3DF", Key: "operating_profit", Label: "FAALİYET KARI (ZARARI)", Statement: "income_statement"},
		"3L":  {MetricID: "3L", Key: "net_income", Label: "DÖNEM KARI (ZARARI)", Statement: "income_statement"},
		"4C":  {MetricID: "4C", Key: "operating_cash_flow", Label: "İşletme Faaliyetlerinden Kaynaklanan Net Nakit", Statement: "cash_flow_statement"},
		"4CB": {MetricID: "4CB", Key: "free_cash_flow", Label: "Serbest Nakit Akım", Statement: "cash_flow_statement"},
	}
	type debtParts struct {
		short float64
		long  float64
		row   map[string]any
	}
	debtByPeriod := map[string]debtParts{}
	for _, row := range rows.Rows {
		metricID := mapStringValue(row, "metric_id")
		if metricID == "2AA" || metricID == "2BA" {
			period := structuredFinancialFactPeriod(row)
			if period != "" {
				parts := debtByPeriod[period]
				if metricID == "2AA" {
					parts.short = mapFloatValue(row, "value")
				} else {
					parts.long = mapFloatValue(row, "value")
				}
				parts.row = row
				debtByPeriod[period] = parts
			}
			continue
		}
		spec, ok := specs[metricID]
		if !ok {
			continue
		}
		if fact, ok := structuredFinancialFactFromRow(result.Symbol, row, spec); ok {
			bundle.FinancialFacts = append(bundle.FinancialFacts, fact)
		}
	}
	for period, parts := range debtByPeriod {
		total := parts.short + parts.long
		if total <= 0 || parts.row == nil {
			continue
		}
		row := copyReportMap(parts.row)
		row["value"] = total
		row["period_end"] = period
		row["line_item_tr"] = "Toplam finansal borçlar"
		row["line_item_original"] = "Toplam finansal borçlar"
		spec := structuredFinancialFactSpec{MetricID: "2AA+2BA", Key: "financial_debt", Label: "Toplam finansal borçlar", Statement: "balance_sheet"}
		if fact, ok := structuredFinancialFactFromRow(result.Symbol, row, spec); ok {
			fact.Source.Snippet = fmt.Sprintf("Toplam finansal borçlar %.0f = kısa vadeli %.0f + uzun vadeli %.0f", total, parts.short, parts.long)
			bundle.FinancialFacts = append(bundle.FinancialFacts, fact)
		}
	}
	bundle.Counts.FinancialFacts = len(bundle.FinancialFacts)
	return bundle
}

func structuredFinancialFactFromRow(symbol string, row map[string]any, spec structuredFinancialFactSpec) (kapingest.ExtractedFinancialFact, bool) {
	value := mapFloatValue(row, "value")
	period := structuredFinancialFactPeriod(row)
	if value == 0 || period == "" || spec.Key == "" {
		return kapingest.ExtractedFinancialFact{}, false
	}
	label := firstNonEmptyReport(mapStringValue(row, "line_item_tr"), mapStringValue(row, "line_item_original"), spec.Label)
	sourceFile := mapStringValue(row, "source_file")
	currency := firstNonEmptyReport(mapStringValue(row, "currency"), "TRY")
	fact := kapingest.ExtractedFinancialFact{
		ID:                 "structured_" + strings.ToLower(symbol) + "_" + strings.ToLower(spec.MetricID) + "_" + strings.ReplaceAll(period, "-", ""),
		Ticker:             strings.ToUpper(strings.TrimSpace(symbol)),
		SourceFile:         sourceFile,
		Period:             stringPtrForReport(period),
		DocumentDate:       stringPtrForReport(period),
		StatementType:      spec.Statement,
		LineItemOriginal:   label,
		LineItemNormalized: spec.Key,
		Value:              value,
		Currency:           currency,
		Unit:               "unit",
		Source: kapingest.DocumentFactSource{
			Snippet: fmt.Sprintf("%s %.0f", label, value),
		},
		Confidence:     1,
		ReviewRequired: false,
		Certification: kapingest.EvidenceCertification{
			Status:                kapingest.EvidenceStatusCertified,
			Score:                 100,
			AnalysisUsable:        true,
			EvidenceComplete:      true,
			NormalizationComplete: true,
			Reasons:               []string{"structured_financials_bilanco_json", "financial_reconciliation_passed"},
		},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return fact, true
}

func structuredFinancialFactPeriod(row map[string]any) string {
	if value := mapStringValue(row, "period_end"); len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	year := int(mapFloatValue(row, "fiscal_year"))
	quarter := int(mapFloatValue(row, "fiscal_quarter"))
	if year > 0 && quarter >= 1 && quarter <= 4 {
		month := quarter * 3
		return fmt.Sprintf("%04d-%02d-%02d", year, month, lastDayOfMonth(year, time.Month(month)))
	}
	return ""
}

func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func mapStringValue(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func mapFloatValue(row map[string]any, key string) float64 {
	if row == nil {
		return 0
	}
	switch value := row[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		f, _ := value.Float64()
		return f
	default:
		return 0
	}
}

func copyReportMap(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for key, value := range row {
		out[key] = value
	}
	return out
}

func stringPtrForReport(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func reportAsOf(result analysis.SymbolAnalysis) time.Time {
	if daily, ok := result.Timeframes["1D"]; ok && len(daily.Candles) > 0 {
		last := daily.Candles[len(daily.Candles)-1].Time
		if !last.IsZero() {
			return last.UTC()
		}
	}
	for _, tf := range result.Timeframes {
		if len(tf.Candles) > 0 {
			last := tf.Candles[len(tf.Candles)-1].Time
			if !last.IsZero() {
				return last.UTC()
			}
		}
	}
	for _, layout := range []string{"2006-01-02_15-04-05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, result.AnalysisDate, time.UTC); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func defaultVAPDir(equitiesDir string) string {
	if strings.TrimSpace(equitiesDir) == "" {
		return filepath.Join("data", "macro", "vap")
	}
	return filepath.Join(filepath.Dir(filepath.Clean(equitiesDir)), "macro", "vap")
}

func updateVAPCoverage(coverage professional.CoverageReport, key string, computed bool, warnings []string) professional.CoverageReport {
	return updateReportCoverage(coverage, key, computed, warnings)
}

func updateReportCoverage(coverage professional.CoverageReport, key string, computed bool, warnings []string) professional.CoverageReport {
	if computed {
		coverage.Available = appendUniqueString(coverage.Available, key)
		coverage.Missing = removeStringValueLocal(coverage.Missing, key)
	} else {
		coverage.Missing = appendUniqueString(coverage.Missing, key)
	}
	for _, warning := range warnings {
		coverage.Warnings = appendUniqueString(coverage.Warnings, warning)
	}
	return recalculateReportCoverageScore(coverage)
}

func normalizeReportCoverageScores(result analysis.SymbolAnalysis) analysis.SymbolAnalysis {
	result.Professional.Coverage = recalculateReportCoverageScore(result.Professional.Coverage)
	if result.Professional.Coverage.Score > 0 {
		result.Professional.DataQuality = result.Professional.Coverage.Score
	}
	return result
}

func recalculateReportCoverageScore(coverage professional.CoverageReport) professional.CoverageReport {
	total := len(coverage.Available) + len(coverage.Missing)
	if total > 0 {
		coverage.Score = 100 * float64(len(coverage.Available)) / float64(total)
	}
	return coverage
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func removeStringValueLocal(values []string, value string) []string {
	out := values[:0]
	for _, existing := range values {
		if existing != value {
			out = append(out, existing)
		}
	}
	return out
}

func reportImageSortRank(item reportImage) int {
	title := strings.ToLower(item.Title + " " + item.Subtitle)
	switch {
	case strings.Contains(title, "tek bakış") || strings.Contains(title, "sonuç odaklı"):
		return 0
	case strings.Contains(title, "indikatör/formasyon") || strings.Contains(title, "detay"):
		return 90
	case strings.Contains(title, "karar"):
		return 10
	case strings.Contains(title, "fiyat"):
		return 20
	default:
		return 50
	}
}

func removeChartArtifacts(targetDir string) error {
	for _, pattern := range []string{"grafik_*.png", "grafik_detay_*.png", "grafik_karar_*.png", "chart_*.html"} {
		matches, err := filepath.Glob(filepath.Join(targetDir, pattern))
		if err != nil {
			return err
		}
		for _, path := range matches {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func envBool(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func shouldLoadKAPRawDataForReport(equitiesDir string, result analysis.SymbolAnalysis) bool {
	if ohlcv.IsCryptoAssetType(result.AssetType) || ohlcv.IsCommodityAssetType(result.AssetType) {
		return false
	}
	if result.Professional.RawKAPData != nil && result.Professional.RawKAPData.Computed {
		return false
	}
	kap := result.Professional.KAPPDFIngest
	if !kap.Computed {
		return false
	}
	if fileExistsLocal(kap.RawDocumentsPath) || fileExistsLocal(kap.ProcessedFilesPath) {
		return true
	}
	symbol := strings.ToLower(strings.TrimSpace(result.Symbol))
	if symbol == "" {
		return false
	}
	fallbackRawPath := filepath.Join(filepath.Dir(filepath.Clean(equitiesDir)), "processed", symbol, kapingest.RawDocumentsFile)
	return fileExistsLocal(fallbackRawPath)
}

func loadKAPRawDataBundle(equitiesDir string, result analysis.SymbolAnalysis) professional.KAPRawDataBundle {
	symbol := strings.ToUpper(strings.TrimSpace(result.Symbol))
	bundle := professional.KAPRawDataBundle{
		Computed: true,
		Symbol:   symbol,
		SourceFiles: professional.KAPRawDataSourceFiles{
			RawDocumentsPath:     result.Professional.KAPPDFIngest.RawDocumentsPath,
			KAPEventsPath:        filepath.Join(result.Professional.KAPPDFIngest.OutputDir, kapingest.KAPEventsFile),
			AssetInventoryPath:   result.Professional.KAPAssetInventory.InventoryPath,
			ExtractionErrorsPath: result.Professional.KAPPDFIngest.ErrorsPath,
			ProcessedFilesPath:   result.Professional.KAPPDFIngest.ProcessedFilesPath,
		},
	}
	tickerProcessedDir := filepath.Dir(bundle.SourceFiles.AssetInventoryPath)
	if strings.TrimSpace(bundle.SourceFiles.AssetInventoryPath) == "" || tickerProcessedDir == "." {
		tickerProcessedDir = filepath.Join(result.Professional.KAPPDFIngest.OutputDir, "by_ticker", symbol)
	}
	if strings.TrimSpace(tickerProcessedDir) != "." && strings.TrimSpace(tickerProcessedDir) != "" {
		bundle.SourceFiles.DocumentIndexPath = filepath.Join(tickerProcessedDir, kapingest.DocumentIndexFile)
		bundle.SourceFiles.KnowledgeGraphPath = filepath.Join(tickerProcessedDir, kapingest.CompanyKnowledgeGraphFile)
		bundle.SourceFiles.DocumentFactsPath = filepath.Join(tickerProcessedDir, kapingest.DocumentFactsFile)
		bundle.SourceFiles.FinancialFactsPath = filepath.Join(tickerProcessedDir, kapingest.FinancialFactsFile)
		bundle.SourceFiles.FinancialTablesPath = filepath.Join(tickerProcessedDir, kapingest.FinancialTablesFile)
		bundle.SourceFiles.PeoplePath = filepath.Join(tickerProcessedDir, kapingest.PeopleFile)
		bundle.SourceFiles.OwnershipFactsPath = filepath.Join(tickerProcessedDir, kapingest.OwnershipFactsFile)
		bundle.SourceFiles.CorporateEventsPath = filepath.Join(tickerProcessedDir, kapingest.CorporateFactsFile)
	}
	rawPath := strings.TrimSpace(bundle.SourceFiles.RawDocumentsPath)
	if rawPath == "" {
		rawPath = filepath.Join(filepath.Dir(filepath.Clean(equitiesDir)), "processed", strings.ToLower(symbol), kapingest.RawDocumentsFile)
	}
	if rawPath != "" {
		docs, err := readJSONLFile[kapingest.RawDocument](rawPath, func(doc kapingest.RawDocument) bool {
			return kapRawDocumentMatchesSymbol(doc.Ticker, doc.FilePath, doc.FileName, symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "raw_documents_read_failed: "+err.Error())
		} else {
			bundle.RawDocuments = docs
			bundle.SourceFiles.RawDocumentsPath = rawPath
		}
	}

	if fileExistsLocal(bundle.SourceFiles.KAPEventsPath) {
		events, err := readJSONLFile[kapingest.KAPEvent](bundle.SourceFiles.KAPEventsPath, func(event kapingest.KAPEvent) bool {
			return strings.EqualFold(strings.TrimSpace(event.Ticker), symbol) || kapRawDocumentMatchesSymbol("", event.FilePath, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "kap_events_read_failed: "+err.Error())
		} else {
			bundle.KAPEvents = events
		}
	} else {
		bundle.SourceFiles.KAPEventsPath = ""
	}

	eventsPath := assetEventsPathForInventory(bundle.SourceFiles.AssetInventoryPath)
	if eventsPath != "" {
		events, err := readJSONLFile[kapingest.AssetEvent](eventsPath, func(event kapingest.AssetEvent) bool {
			return strings.EqualFold(strings.TrimSpace(event.Ticker), symbol) || kapRawDocumentMatchesSymbol("", event.SourceFile, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "asset_events_read_failed: "+err.Error())
		} else {
			sortAssetEventsByPeriod(events)
			bundle.AssetEvents = events
			bundle.SourceFiles.AssetEventsPath = eventsPath
		}
	}

	inventoryPath := strings.TrimSpace(bundle.SourceFiles.AssetInventoryPath)
	if inventoryPath != "" {
		inventory, err := readJSONFile[kapingest.AssetInventory](inventoryPath)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "asset_inventory_read_failed: "+err.Error())
		} else {
			sortAssetInventoryByPeriod(&inventory)
			bundle.AssetInventory = &inventory
		}
	}
	if fileExistsLocal(bundle.SourceFiles.DocumentIndexPath) {
		index, err := readJSONFile[kapingest.DocumentIndex](bundle.SourceFiles.DocumentIndexPath)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "document_index_read_failed: "+err.Error())
		} else {
			bundle.DocumentIndex = &index
		}
	} else {
		bundle.SourceFiles.DocumentIndexPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.KnowledgeGraphPath) {
		graph, err := readJSONFile[kapingest.CompanyKnowledgeGraph](bundle.SourceFiles.KnowledgeGraphPath)
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "company_knowledge_graph_read_failed: "+err.Error())
		} else {
			bundle.KnowledgeGraph = &graph
		}
	} else {
		bundle.SourceFiles.KnowledgeGraphPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.DocumentFactsPath) {
		facts, err := readJSONLFile[kapingest.DocumentFact](bundle.SourceFiles.DocumentFactsPath, func(fact kapingest.DocumentFact) bool {
			return strings.EqualFold(strings.TrimSpace(fact.Ticker), symbol) || kapRawDocumentMatchesSymbol("", fact.SourceFile, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "document_facts_read_failed: "+err.Error())
		} else {
			sortDocumentFactsByPeriod(facts)
			bundle.DocumentFacts = facts
		}
	} else {
		bundle.SourceFiles.DocumentFactsPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.FinancialFactsPath) {
		facts, err := readJSONLFile[kapingest.ExtractedFinancialFact](bundle.SourceFiles.FinancialFactsPath, func(fact kapingest.ExtractedFinancialFact) bool {
			return strings.EqualFold(strings.TrimSpace(fact.Ticker), symbol) || kapRawDocumentMatchesSymbol("", fact.SourceFile, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "financial_facts_read_failed: "+err.Error())
		} else {
			sortFinancialFactsByPeriod(facts)
			bundle.FinancialFacts = facts
			if bundle.KnowledgeGraph != nil {
				bundle.KnowledgeGraph.ResolvedContradictions = kapingest.ReconcileFinancialValueDifferences(facts)
				bundle.KnowledgeGraph.Contradictions = nil
			}
		}
	} else {
		bundle.SourceFiles.FinancialFactsPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.FinancialTablesPath) {
		tables, err := readJSONLFile[kapingest.ExtractedFinancialTable](bundle.SourceFiles.FinancialTablesPath, func(table kapingest.ExtractedFinancialTable) bool {
			return strings.EqualFold(strings.TrimSpace(table.Ticker), symbol) || kapRawDocumentMatchesSymbol("", table.SourceFile, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "financial_tables_read_failed: "+err.Error())
		} else {
			sortFinancialTablesByPeriod(tables)
			bundle.FinancialTables = tables
		}
	} else {
		bundle.SourceFiles.FinancialTablesPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.PeoplePath) {
		people, err := readJSONLFile[kapingest.ExtractedPerson](bundle.SourceFiles.PeoplePath, func(person kapingest.ExtractedPerson) bool {
			return strings.EqualFold(strings.TrimSpace(person.Ticker), symbol) || kapRawDocumentMatchesSymbol("", person.SourceFile, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "people_read_failed: "+err.Error())
		} else {
			sortPeopleByPeriod(people)
			bundle.People = people
		}
	} else {
		bundle.SourceFiles.PeoplePath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.OwnershipFactsPath) {
		facts, err := readJSONLFile[kapingest.OwnershipFact](bundle.SourceFiles.OwnershipFactsPath, func(fact kapingest.OwnershipFact) bool {
			return strings.EqualFold(strings.TrimSpace(fact.Ticker), symbol) || kapRawDocumentMatchesSymbol("", fact.SourceFile, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "ownership_facts_read_failed: "+err.Error())
		} else {
			sortOwnershipFactsByPeriod(facts)
			bundle.OwnershipFacts = facts
		}
	} else {
		bundle.SourceFiles.OwnershipFactsPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.CorporateEventsPath) {
		events, err := readJSONLFile[kapingest.ExtractedCorporateEvent](bundle.SourceFiles.CorporateEventsPath, func(event kapingest.ExtractedCorporateEvent) bool {
			return strings.EqualFold(strings.TrimSpace(event.Ticker), symbol) || kapRawDocumentMatchesSymbol("", event.SourceFile, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "corporate_events_read_failed: "+err.Error())
		} else {
			sortCorporateEventsByPeriod(events)
			bundle.CorporateEvents = events
		}
	} else {
		bundle.SourceFiles.CorporateEventsPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.ExtractionErrorsPath) {
		errors, err := readJSONLFile[kapingest.ExtractionError](bundle.SourceFiles.ExtractionErrorsPath, func(item kapingest.ExtractionError) bool {
			return kapRawDocumentMatchesSymbol("", item.FilePath, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "extraction_errors_read_failed: "+err.Error())
		} else {
			bundle.ExtractionErrors = errors
		}
	} else {
		bundle.SourceFiles.ExtractionErrorsPath = ""
	}
	if fileExistsLocal(bundle.SourceFiles.ProcessedFilesPath) {
		processed, err := readJSONLFile[kapingest.ProcessedFile](bundle.SourceFiles.ProcessedFilesPath, func(item kapingest.ProcessedFile) bool {
			return strings.EqualFold(strings.TrimSpace(item.Ticker), symbol) || kapRawDocumentMatchesSymbol("", item.FilePath, "", symbol)
		})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, "processed_files_read_failed: "+err.Error())
		} else {
			bundle.ProcessedFiles = processed
		}
	} else {
		bundle.SourceFiles.ProcessedFilesPath = ""
	}
	bundle.Counts = professional.KAPRawDataCounts{
		RawDocuments:     len(bundle.RawDocuments),
		KAPEvents:        len(bundle.KAPEvents),
		AssetEvents:      len(bundle.AssetEvents),
		InventoryItems:   0,
		KnowledgeGraph:   0,
		DocumentFacts:    len(bundle.DocumentFacts),
		FinancialFacts:   len(bundle.FinancialFacts),
		FinancialTables:  len(bundle.FinancialTables),
		People:           len(bundle.People),
		OwnershipFacts:   len(bundle.OwnershipFacts),
		CorporateEvents:  len(bundle.CorporateEvents),
		ExtractionErrors: len(bundle.ExtractionErrors),
		ProcessedFiles:   len(bundle.ProcessedFiles),
	}
	if bundle.AssetInventory != nil {
		bundle.Counts.InventoryItems = len(bundle.AssetInventory.Assets)
	}
	if bundle.KnowledgeGraph != nil {
		bundle.Counts.KnowledgeGraph = 1
	}
	if bundle.Counts.RawDocuments == 0 &&
		bundle.Counts.KAPEvents == 0 &&
		bundle.Counts.AssetEvents == 0 &&
		bundle.Counts.InventoryItems == 0 &&
		bundle.Counts.DocumentFacts == 0 &&
		bundle.Counts.FinancialFacts == 0 &&
		bundle.Counts.FinancialTables == 0 &&
		bundle.Counts.People == 0 &&
		bundle.Counts.OwnershipFacts == 0 &&
		bundle.Counts.CorporateEvents == 0 {
		bundle.Computed = false
		bundle.Warnings = append(bundle.Warnings, "kap_raw_data_empty")
	}
	return bundle
}

func readJSONLFile[T any](path string, keep func(T) bool) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rows := []T{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 128*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row T
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return rows, err
		}
		if keep == nil || keep(row) {
			rows = append(rows, row)
		}
	}
	if err := scanner.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}

func readJSONFile[T any](path string) (T, error) {
	var out T
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}

func fileExistsLocal(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func assetEventsPathForInventory(inventoryPath string) string {
	inventoryPath = strings.TrimSpace(inventoryPath)
	if inventoryPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(inventoryPath), kapingest.AssetEventsFile)
}

func kapRawDocumentMatchesSymbol(ticker, filePath, fileName, symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(ticker), symbol) {
		return true
	}
	slashPath := strings.ToUpper(filepath.ToSlash(filePath))
	if strings.Contains(slashPath, "/EQUITIES/"+symbol+"/") {
		return true
	}
	return strings.Contains(strings.ToUpper(fileName), symbol)
}

func sortAssetEventsByPeriod(events []kapingest.AssetEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		pi := assetEventPeriodKey(events[i])
		pj := assetEventPeriodKey(events[j])
		if pi != pj {
			return pi > pj
		}
		if events[i].AssetName != events[j].AssetName {
			return events[i].AssetName < events[j].AssetName
		}
		if events[i].SourceFile != events[j].SourceFile {
			return events[i].SourceFile < events[j].SourceFile
		}
		return events[i].SHA256 < events[j].SHA256
	})
}

func sortAssetInventoryByPeriod(inventory *kapingest.AssetInventory) {
	if inventory == nil {
		return
	}
	for i := range inventory.Assets {
		sort.SliceStable(inventory.Assets[i].History, func(a, b int) bool {
			pa := inventory.Assets[i].History[a].Period
			pb := inventory.Assets[i].History[b].Period
			if pa != pb {
				return pa < pb
			}
			if inventory.Assets[i].History[a].ExpertiseDate != inventory.Assets[i].History[b].ExpertiseDate {
				return inventory.Assets[i].History[a].ExpertiseDate < inventory.Assets[i].History[b].ExpertiseDate
			}
			return inventory.Assets[i].History[a].SourceFile < inventory.Assets[i].History[b].SourceFile
		})
	}
	sort.SliceStable(inventory.Assets, func(i, j int) bool {
		li := latestInventoryPeriod(inventory.Assets[i])
		lj := latestInventoryPeriod(inventory.Assets[j])
		if li != lj {
			return li > lj
		}
		if inventory.Assets[i].AssetName != inventory.Assets[j].AssetName {
			return inventory.Assets[i].AssetName < inventory.Assets[j].AssetName
		}
		return inventory.Assets[i].AssetType < inventory.Assets[j].AssetType
	})
	sort.SliceStable(inventory.PortfolioSummary.History, func(i, j int) bool {
		if inventory.PortfolioSummary.History[i].Period != inventory.PortfolioSummary.History[j].Period {
			return inventory.PortfolioSummary.History[i].Period < inventory.PortfolioSummary.History[j].Period
		}
		return inventory.PortfolioSummary.History[i].SourceFile < inventory.PortfolioSummary.History[j].SourceFile
	})
}

func sortDocumentFactsByPeriod(facts []kapingest.DocumentFact) {
	sort.SliceStable(facts, func(i, j int) bool {
		pi := stringPtrPeriodKey(facts[i].Period)
		pj := stringPtrPeriodKey(facts[j].Period)
		if pi != pj {
			return pi > pj
		}
		if facts[i].Group != facts[j].Group {
			return facts[i].Group < facts[j].Group
		}
		if facts[i].SourceFile != facts[j].SourceFile {
			return facts[i].SourceFile < facts[j].SourceFile
		}
		return facts[i].ID < facts[j].ID
	})
}

func sortFinancialFactsByPeriod(facts []kapingest.ExtractedFinancialFact) {
	sort.SliceStable(facts, func(i, j int) bool {
		pi := stringPtrPeriodKey(facts[i].Period)
		pj := stringPtrPeriodKey(facts[j].Period)
		if pi != pj {
			return pi > pj
		}
		if facts[i].LineItemNormalized != facts[j].LineItemNormalized {
			return facts[i].LineItemNormalized < facts[j].LineItemNormalized
		}
		if facts[i].SourceFile != facts[j].SourceFile {
			return facts[i].SourceFile < facts[j].SourceFile
		}
		return facts[i].ID < facts[j].ID
	})
}

func sortFinancialTablesByPeriod(tables []kapingest.ExtractedFinancialTable) {
	sort.SliceStable(tables, func(i, j int) bool {
		pi := stringPtrPeriodKey(tables[i].Period)
		pj := stringPtrPeriodKey(tables[j].Period)
		if pi != pj {
			return pi > pj
		}
		if tables[i].TableType != tables[j].TableType {
			return tables[i].TableType < tables[j].TableType
		}
		if tables[i].SourceFile != tables[j].SourceFile {
			return tables[i].SourceFile < tables[j].SourceFile
		}
		return tables[i].ID < tables[j].ID
	})
}

func sortPeopleByPeriod(people []kapingest.ExtractedPerson) {
	sort.SliceStable(people, func(i, j int) bool {
		pi := stringPtrPeriodKey(people[i].Period)
		pj := stringPtrPeriodKey(people[j].Period)
		if pi != pj {
			return pi > pj
		}
		if people[i].NormalizedName != people[j].NormalizedName {
			return people[i].NormalizedName < people[j].NormalizedName
		}
		return people[i].SourceFile < people[j].SourceFile
	})
}

func sortOwnershipFactsByPeriod(facts []kapingest.OwnershipFact) {
	sort.SliceStable(facts, func(i, j int) bool {
		pi := stringPtrPeriodKey(facts[i].Period)
		pj := stringPtrPeriodKey(facts[j].Period)
		if pi != pj {
			return pi > pj
		}
		if facts[i].HolderName != facts[j].HolderName {
			return facts[i].HolderName < facts[j].HolderName
		}
		return facts[i].SourceFile < facts[j].SourceFile
	})
}

func sortCorporateEventsByPeriod(events []kapingest.ExtractedCorporateEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		pi := stringPtrPeriodKey(events[i].Period)
		pj := stringPtrPeriodKey(events[j].Period)
		if pi != pj {
			return pi > pj
		}
		if events[i].EventType != events[j].EventType {
			return events[i].EventType < events[j].EventType
		}
		return events[i].SourceFile < events[j].SourceFile
	})
}

func stringPtrPeriodKey(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func assetEventPeriodKey(event kapingest.AssetEvent) string {
	if event.Period != nil && strings.TrimSpace(*event.Period) != "" {
		return strings.TrimSpace(*event.Period)
	}
	if event.DocumentDate != nil && strings.TrimSpace(*event.DocumentDate) != "" {
		return strings.TrimSpace(*event.DocumentDate)
	}
	return ""
}

func latestInventoryPeriod(item kapingest.AssetInventoryItem) string {
	latest := ""
	for _, point := range item.History {
		if point.Period > latest {
			latest = point.Period
		}
	}
	return latest
}

func AnalysisDir(equitiesDir string, symbol string, analysisDate string) string {
	return AnalysisDirForAsset(equitiesDir, ohlcv.AssetTypeEquity, symbol, analysisDate)
}

func AnalysisDirForAsset(outputRoot string, assetType string, symbol string, analysisDate string) string {
	return filepath.Join(assetOutputRoot(outputRoot, assetType), ohlcv.SymbolPathKey(symbol), "analysis", analysisDate)
}

func assetOutputRoot(outputRoot string, assetType string) string {
	if filepath.Base(outputRoot) == "equities" {
		switch {
		case ohlcv.IsCryptoAssetType(assetType):
			return filepath.Join(filepath.Dir(outputRoot), "crypto")
		case ohlcv.IsCommodityAssetType(assetType):
			return filepath.Join(filepath.Dir(outputRoot), "commodities")
		}
	}
	return outputRoot
}

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json for %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write json file %s: %w", path, err)
	}
	return nil
}

func WriteText(path string, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write text file %s: %w", path, err)
	}
	return nil
}

func WriteBinary(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write binary file %s: %w", path, err)
	}
	return nil
}

func chartHTML(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis, imageName string) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"tr\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString(fmt.Sprintf("<title>%s %s Teknik Grafik</title>\n", result.Symbol, tf.Timeframe))
	b.WriteString("<style>body{font-family:Arial,Helvetica,sans-serif;margin:0;background:#f7f7f5;color:#202124}header{padding:16px 20px;background:#fff;border-bottom:1px solid #ddd}main{padding:16px;display:grid;gap:14px}img{max-width:100%;border:1px solid #d5d8d2;background:#fff}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:12px}.box{background:#fff;border:1px solid #d5d8d2;border-radius:6px;padding:12px}pre{overflow:auto;background:#111;color:#f3f3f3;padding:12px;border-radius:6px}</style>\n")
	b.WriteString("</head>\n<body>\n")
	b.WriteString(fmt.Sprintf("<header><h1>%s %s Teknik Grafik</h1><div>%s</div></header>\n", result.Symbol, localize.Timeframe(tf.Timeframe), result.Disclaimer))
	b.WriteString("<main>\n")
	b.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"%s %s mum grafigi\">\n", imageName, result.Symbol, tf.Timeframe))
	b.WriteString("<div class=\"grid\">\n")
	b.WriteString(fmt.Sprintf("<section class=\"box\"><strong>Skor</strong><p>%.2f / 100 | %s</p></section>\n", tf.Score, localize.Bias(tf.TrendBias)))
	if chartActiveTradePlan(result, tf) {
		b.WriteString(fmt.Sprintf("<section class=\"box\"><strong>Trade Plan</strong><p>%s | RR %.2f | %s</p><p>Entry %s - %s<br>TP1 %s | TP2 %s | Stop %s</p></section>\n", localize.Direction(tf.TradePlan.Direction), tf.TradePlan.RiskRewardRatio, localize.Quality(tf.TradePlan.Quality), reportPriceValue(tf.TradePlan.EntryMin), reportPriceValue(tf.TradePlan.EntryMax), reportPriceValue(tf.TradePlan.TakeProfit1), reportPriceValue(tf.TradePlan.TakeProfit2), reportPriceValue(tf.TradePlan.StopLoss)))
	} else {
		b.WriteString(fmt.Sprintf("<section class=\"box\"><strong>Trade Plan</strong><p>Aktif işlem planı yok | %s</p><p>%s</p></section>\n", localize.Quality(tf.TradePlan.Quality), emptyFallback(timeframeGateTextForReport(result, tf), "Teyit bekleniyor")))
	}
	b.WriteString("<section class=\"box\"><strong>Formasyonlar</strong><ul>")
	for _, pattern := range tf.Patterns {
		if pattern.Confidence >= 0.5 {
			b.WriteString(fmt.Sprintf("<li>%s | %s | %.2f</li>", localize.PatternName(pattern.Name), localize.Direction(pattern.Direction), pattern.Confidence))
		}
	}
	b.WriteString("</ul></section>\n")
	b.WriteString("</div>\n")
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func chartActiveTradePlan(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis) bool {
	return activeSummaryTradePlan(tf.TradePlan) &&
		!timeframeIsDailyContextWindow(tf) &&
		!timeframeIsLongContext(tf) &&
		!reportTradePlanBlocked(result)
}

func sortedTimeframeKeys(timeframes map[string]analysis.TimeframeAnalysis) []string {
	keys := make([]string, 0, len(timeframes))
	for key := range timeframes {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		leftRank := timeframeSortRank(keys[i])
		rightRank := timeframeSortRank(keys[j])
		if leftRank == rightRank {
			return keys[i] < keys[j]
		}
		return leftRank < rightRank
	})
	return keys
}

func timeframeSortRank(timeframe string) int {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "5":
		return 10
	case "15":
		return 20
	case "30":
		return 30
	case "60", "1H":
		return 40
	case "1D", "D":
		return 50
	case "1W", "W":
		return 60
	case "1M", "M":
		return 70
	case "3M":
		return 80
	case "6M":
		return 90
	case "1Y":
		return 100
	case "YTD":
		return 110
	case "ALL":
		return 120
	default:
		return 1000
	}
}

func markdownSummary(result analysis.SymbolAnalysis) string {
	keys := sortedTimeframeKeys(result.Timeframes)
	qa := result.InvestorQA
	var b strings.Builder
	b.WriteString("# ")
	if isCryptoResult(result) {
		b.WriteString("Sade Kripto Analiz Raporu")
	} else if isCommodityResult(result) {
		b.WriteString("Sade Altın/Emtia Analiz Raporu")
	} else {
		b.WriteString("Büyük ve Küçük Yatırımcı Karar Raporu")
	}
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("- Sembol: %s\n", result.Symbol))
	b.WriteString(fmt.Sprintf("- %s: %s\n", entityNameLabel(result), result.CompanyName))
	b.WriteString(fmt.Sprintf("- Varlık tipi: %s\n", assetTypeLabel(result)))
	if result.Exchange != "" {
		b.WriteString(fmt.Sprintf("- Borsa/kaynak: %s\n", result.Exchange))
	}
	b.WriteString(fmt.Sprintf("- Tarih: %s\n", result.AnalysisDate))
	b.WriteString(fmt.Sprintf("- Küçük yatırımcı sinyali: %s\n", retailDecisionLabel(result)))
	if result.DecisionSupport != nil {
		b.WriteString(fmt.Sprintf("- Büyük yatırımcı kararı: %s\n", reportLabel(result.DecisionSupport.Institutional.Decision)))
	}
	b.WriteString(fmt.Sprintf("- Genel skor: %.0f/100\n", result.OverallScore))
	b.WriteString(fmt.Sprintf("- Para birimi: %s\n\n", displayCurrency(result.Currency)))

	if c := result.DecisionClassification; c.SchemaVersion > 0 {
		b.WriteString("## Merkezi Karar Sınıflandırması\n\n")
		b.WriteString(fmt.Sprintf("- Büyük yatırımcı: %s — %s\n", reportLabel(c.Classes.LargeInvestor.Decision), c.Classes.LargeInvestor.Summary))
		b.WriteString(fmt.Sprintf("- Küçük yatırımcı doğrudan AL/SAT: %s — %s\n", reportLabel(c.Classes.RetailDirect.Decision), c.Classes.RetailDirect.Summary))
		b.WriteString(fmt.Sprintf("- Değerleme yayımlanabilir: %s; model içi ayrışma %.1f%% ((en yüksek-en düşük)/en düşük), eşik %.1f%%. Fiyat/baz değer farkı ayrı metrik olarak raporlanır.\n", boolTRForReport(c.ValuationConsistency.Publishable), c.ValuationConsistency.MaxDivergencePct, c.ValuationConsistency.ThresholdPct))
		b.WriteString(fmt.Sprintf("- Sektör modeli: %s\n\n", c.SectorModelAlignment.Reason))
	}

	if result.DecisionSupport != nil {
		institutional := result.DecisionSupport.Institutional
		retail := result.DecisionSupport.Retail
		b.WriteString("## Büyük Yatırımcı Kararı\n\n")
		b.WriteString(fmt.Sprintf("- Karar: %s\n", reportLabel(institutional.Decision)))
		b.WriteString(fmt.Sprintf("- Pozisyon aksiyonu: %s\n", reportLabel(institutional.PositionAction)))
		b.WriteString(fmt.Sprintf("- Sonuç: %s\n", retailText(clarifyInstitutionalDecisionConfidenceText(institutional.OneLineAnswer))))
		b.WriteString("\n## Küçük Yatırımcı Doğrudan AL/SAT Kararı\n\n")
		b.WriteString(fmt.Sprintf("- Yeni pozisyon: %s\n", retailActionTRForReport(retail.NewPositionAction)))
		b.WriteString(fmt.Sprintf("- Elde varsa: %s\n", retailActionTRForReport(retail.ExistingPositionAction)))
		b.WriteString(fmt.Sprintf("- Ana sinyal: %s\n", reportLabel(retail.Signal)))
		b.WriteString(fmt.Sprintf("- Sonuç: %s\n", retailText(clarifyDecisionConfidenceText(retail.OneLineAnswer))))
	}

	b.WriteString("## Kısa Cevap\n\n")
	if qa.Computed && qa.OneLineAnswer != "" {
		b.WriteString("- ")
		b.WriteString(retailText(qa.OneLineAnswer))
		b.WriteString("\n")
	} else {
		b.WriteString("- ")
		b.WriteString(retailDecisionSentence(result))
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("- Karar güveni: %.0f/100. Bu, rapor/veri güveninden ayrı olarak küçük yatırımcı aksiyon cümlesinin ne kadar desteklendiğini gösterir; fiyat garantisi değildir.\n", retailConfidence(result)))
	b.WriteString(fmt.Sprintf("- Son fiyat: %s\n", retailPrice(primaryLastClose(result), result.Currency)))

	b.WriteString("\n## Ne Yapmalı?\n\n")
	for _, line := range retailActionLines(result) {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n## Önemli Fiyat Seviyeleri\n\n")
	for _, line := range retailLevelLines(result) {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	if lines := nextSessionForecastLines(result); len(lines) > 0 {
		b.WriteString("\n## Bir Sonraki Seans Beklentisi\n\n")
		for _, line := range lines {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if lines := vapContextLines(result); len(lines) > 0 {
		b.WriteString("\n## VAP / MKK Piyasa Yapısı\n\n")
		for _, line := range lines {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n## Neden Bu Karar Çıktı?\n\n")
	for _, line := range retailReasonLines(result) {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n## Karar Ne Zaman Değişir?\n\n")
	for _, line := range retailDecisionChangeLines(result) {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n## Zaman Dilimi Özeti\n\n")
	for _, key := range keys {
		tf := result.Timeframes[key]
		b.WriteString(fmt.Sprintf("- %s: %s\n", localize.Timeframe(key), retailTimeframeLineForReport(result, tf, result.Currency)))
	}

	b.WriteString("\n## Başlıca Riskler\n\n")
	for _, risk := range reportRiskReasons(result) {
		b.WriteString("- ")
		b.WriteString(retailText(risk))
		b.WriteString("\n")
	}
	if result.InvestorQA.Computed {
		views := result.InvestorQA.InstitutionalViews
		if views.Computed {
			b.WriteString("\n## Rapor Kullanım Notu\n\n")
			b.WriteString(fmt.Sprintf("- Rapor kalitesi: %s. Yatırım/işlem uygunluğu: %s.\n", institutionalStatusTR(views.OverallQualityStatus), institutionalStatusTR(views.OverallStatus)))
			if isDecisionReadyStatus(result) && result.DecisionSupport != nil {
				ds := result.DecisionSupport
				if ds.Retail.OneLineAnswer != "" {
					b.WriteString("- Küçük yatırımcı kararı: ")
					b.WriteString(clarifyDecisionConfidenceText(ds.Retail.OneLineAnswer))
					b.WriteString("\n")
				}
				if ds.Institutional.OneLineAnswer != "" {
					b.WriteString("- Büyük yatırımcı kararı: ")
					b.WriteString(clarifyInstitutionalDecisionConfidenceText(ds.Institutional.OneLineAnswer))
					b.WriteString("\n")
				}
			} else if views.FinancialTransactionUse.Summary != "" {
				b.WriteString("- ")
				b.WriteString(retailText(views.FinancialTransactionUse.Summary))
				b.WriteString("\n")
			}
		}
		if len(result.InvestorQA.Questions) > 0 {
			b.WriteString("\n## Kısa Soru-Cevap\n\n")
			count := 0
			for _, item := range result.InvestorQA.Questions {
				if !retailQuestionAllowed(item.Question) {
					continue
				}
				b.WriteString(fmt.Sprintf("- %s: %s\n", item.Question, retailText(item.Answer)))
				count++
				if count >= 6 {
					break
				}
			}
		}
	}

	b.WriteString("\n## Detaya Bakmak İsteyenler İçin\n\n")
	for _, key := range keys {
		tf := result.Timeframes[key]
		proTF := tf.Professional
		technical := proTF.Technical
		b.WriteString(fmt.Sprintf("### %s\n", localize.Timeframe(key)))
		b.WriteString(fmt.Sprintf("- Fiyat: %s. Kısa yorum: %s\n", retailPrice(tf.LastClose, result.Currency), timeframePlainComment(tf)))
		b.WriteString(fmt.Sprintf("- Güç durumu: %s. İşlem ilgisi: %s. Günlük hareket riski: %s.\n", retailMomentumLine(tf), retailMoneyFlowLine(tf), retailVolatilityLine(tf)))
		if technical.SignalGate.Status != "" {
			b.WriteString(fmt.Sprintf("- Teyit durumu: %s. %s\n", institutionalStatusTR(technical.SignalGate.Status), retailText(technical.SignalGate.Label)))
			if len(technical.SignalGate.Blockers) > 0 {
				b.WriteString(fmt.Sprintf("- Neden bekleniyor: %s\n", retailSignalGateBlockers(tf, 3)))
			}
		}
		if line := retailBacktestLine(proTF); line != "" {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("- Alım satım kolaylığı: son 20 gün ortalama işlem değeri %s.\n", formatMoney(proTF.Liquidity.AverageValueTraded20TRY, result.Currency)))
	}

	b.WriteString("\n## Basit Terimler\n\n")
	for _, line := range retailGlossaryLines(result) {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n## Uyarı\n\n")
	b.WriteString(result.Disclaimer)
	b.WriteString("\n")
	return b.String()
}

func retailDecisionLabel(result analysis.SymbolAnalysis) string {
	if result.DecisionSupport != nil && result.DecisionSupport.Retail.Signal != "" {
		return reportLabel(result.DecisionSupport.Retail.Signal)
	}
	if result.InvestorQA.Computed {
		for _, item := range result.InvestorQA.ActionMatrix {
			action := strings.ToLower(strings.TrimSpace(item.Action))
			switch {
			case strings.Contains(action, "sat") && item.CurrentSignal:
				return "RİSK AZALT"
			case strings.Contains(action, "al") && item.CurrentSignal:
				return "ALIM ADAYI"
			}
		}
		for _, item := range result.InvestorQA.ActionMatrix {
			action := strings.ToLower(strings.TrimSpace(item.Action))
			if strings.Contains(action, "tut") && item.CurrentSignal {
				return "BEKLE / TUT-İZLE"
			}
		}
		if result.InvestorQA.Decision != "" {
			return reportLabel(result.InvestorQA.Decision)
		}
	}
	return executiveDecision(result)
}

func retailBacktestLine(report professional.TimeframeReport) string {
	bt := report.Backtest
	if bt.Trades == 0 {
		return "Benzer geçmiş durumlar: yeterli örnek yok; geçmiş performans karar girdisi yapılmadı."
	}
	if !bt.BacktestSafe || !report.PriceAdjustment.BacktestSafe {
		return "Benzer geçmiş durumlar: fiyat düzeltme veya zaman güvenliği geçmediği için geçmiş performans sonucu karar girdisi yapılmadı."
	}
	if bt.Trades < 30 || bt.OutOfSampleTrades < 10 {
		return fmt.Sprintf("Benzer geçmiş durumlar: %d örnek ve %d ileri dönem test incelendi; örnek sınırlı olduğu için sonuç karar girdisi yapılmadı.", bt.Trades, bt.OutOfSampleTrades)
	}
	return fmt.Sprintf("Benzer geçmiş durumlar: %d örnek ve %d ileri dönem test incelendi; ortalama sonuç %.2f%%. Tek başına karar sebebi değildir.", bt.Trades, bt.OutOfSampleTrades, bt.AverageReturn*100)
}

func retailDecisionSentence(result analysis.SymbolAnalysis) string {
	return fmt.Sprintf("%s için sade karar %s. Bu sonuç kesin getiri vaadi değil; fiyat teyidi ve risk seviyesi birlikte izlenmelidir.", result.Symbol, retailDecisionLabel(result))
}

func retailConfidence(result analysis.SymbolAnalysis) float64 {
	verification := reportConfidenceFor(result).Score
	if result.InvestorQA.Computed && result.InvestorQA.Confidence > 0 {
		if verification > 0 {
			return math.Min(result.InvestorQA.Confidence, verification)
		}
		return result.InvestorQA.Confidence
	}
	if result.InstitutionalValidation.Score > 0 {
		return result.InstitutionalValidation.Score
	}
	return result.OverallScore
}

func retailPrice(value float64, currency string) string {
	if value <= 0 {
		return "seviye yok"
	}
	if isTLReportCurrency(currency) {
		value = analysis.RoundBISTEquityPriceToTick(value)
	}
	return reportPrice(value, currency)
}

func isTLReportCurrency(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "TRY", "TL":
		return true
	default:
		return false
	}
}

func retailActionLines(result analysis.SymbolAnalysis) []string {
	if result.DecisionSupport != nil && result.DecisionSupport.Retail.Signal != "" {
		decision := result.DecisionSupport.Retail
		lines := []string{
			"Yeni pozisyon: " + retailActionTRForReport(decision.NewPositionAction) + ".",
			"Elinde varsa: " + retailActionTRForReport(decision.ExistingPositionAction) + ".",
		}
		if decision.Trigger != "" {
			lines = append(lines, "Karar/tetikleyici: "+retailText(decision.Trigger)+".")
		}
		if decision.Invalidation != "" {
			lines = append(lines, "Geçersizleşme/risk şartı: "+retailText(decision.Invalidation)+".")
		}
		return lines
	}
	tradePlanBlocked := reportTradePlanBlocked(result)
	if result.InvestorQA.Computed && len(result.InvestorQA.ActionMatrix) > 0 {
		lines := []string{}
		for _, item := range result.InvestorQA.ActionMatrix {
			action := strings.ToLower(strings.TrimSpace(item.Action))
			trigger := retailText(emptyFallback(item.Trigger, "teyit bekleniyor"))
			invalidation := retailText(emptyFallback(item.Invalidation, "risk seviyesi üretilmedi"))
			switch {
			case strings.Contains(action, "sat") || strings.Contains(action, "risk"):
				if item.CurrentSignal {
					lines = append(lines, fmt.Sprintf("Risk azalt: sinyal aktif. Sebep: %s.", trigger))
				} else {
					lines = append(lines, fmt.Sprintf("Risk azalt / sat: şu an doğrudan sinyal yok. Alarm şartı: %s.", trigger))
				}
			case strings.Contains(action, "al"):
				if tradePlanBlocked {
					lines = append(lines, "Yeni alım: yok. AL/SAT kullanım kapısı geçmeden teknik seviye bandı emir planı sayılmaz.")
					continue
				}
				if item.CurrentSignal {
					lines = append(lines, fmt.Sprintf("Yeni alım: aday var; %s. Risk: %s.", trigger, invalidation))
				} else {
					lines = append(lines, fmt.Sprintf("Yeni alım: şu an yok. Alım için beklenen şart: %s.", trigger))
				}
			case strings.Contains(action, "tut") || strings.Contains(action, "izle"):
				if item.CurrentSignal {
					lines = append(lines, fmt.Sprintf("Elinde varsa: takip edilebilir. Risk seviyesi: %s.", invalidation))
				} else {
					lines = append(lines, fmt.Sprintf("Elinde varsa: acele artırma yok; %s.", trigger))
				}
			}
		}
		if len(lines) > 0 {
			return lines
		}
	}
	lines := []string{}
	if len(result.InvestorQA.BuyConditions) > 0 {
		if tradePlanBlocked {
			lines = append(lines, "Yeni alım: yok. İzleme şartı: "+retailText(strings.Join(result.InvestorQA.BuyConditions, "; "))+". AL/SAT kullanım kapısı geçmeden emir planı kurulmaz.")
		} else {
			lines = append(lines, "Yeni alım: şu an teyit bekleniyor. Şart: "+retailText(strings.Join(result.InvestorQA.BuyConditions, "; "))+".")
		}
	}
	if len(result.InvestorQA.ExitConditions) > 0 {
		lines = append(lines, "Risk azalt: "+retailText(strings.Join(result.InvestorQA.ExitConditions, "; "))+".")
	}
	if len(lines) == 0 {
		lines = append(lines, "Yeni işlem: net giriş-stop-hedef planı yok; teyit beklenmeli.")
	}
	return lines
}

func retailLevelLines(result analysis.SymbolAnalysis) []string {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		return []string{"Günlük fiyat seviyesi üretilemedi."}
	}
	lines := []string{
		"Son kapanış: " + retailPrice(tf.LastClose, result.Currency) + ".",
	}
	if tf.NearestResistance != nil {
		lines = append(lines, fmt.Sprintf("Üst eşik: %s. Bunun üzerinde kapanış güçlenme işareti olur.", retailPrice(tf.NearestResistance.Price, result.Currency)))
	}
	if tf.NearestSupport != nil {
		lines = append(lines, fmt.Sprintf("Alt eşik: %s. Bunun altında kapanış risk artırır.", retailPrice(tf.NearestSupport.Price, result.Currency)))
	}
	if level, ok := nextResistance(tf); ok {
		lines = append(lines, "Üst eşik aşılırsa sonraki direnç: "+retailPrice(level.Price, result.Currency)+".")
	}
	if level, ok := nextSupport(tf); ok {
		lines = append(lines, "Alt eşik kırılırsa sonraki destek: "+retailPrice(level.Price, result.Currency)+".")
	}
	if weekly, ok := result.Timeframes["1W"]; ok {
		if weekly.NearestSupport != nil {
			lines = append(lines, "Haftalık ana destek: "+retailPrice(weekly.NearestSupport.Price, result.Currency)+".")
		}
		if weekly.NearestResistance != nil {
			lines = append(lines, "Haftalık ana direnç: "+retailPrice(weekly.NearestResistance.Price, result.Currency)+".")
		}
	}
	// Show rejected trade plan levels as reference (labelled as observation, not active signal).
	plan := tf.TradePlan
	if plan.Rejected && plan.StopLoss > 0 && plan.TakeProfit1 > 0 && plan.EntryMax > 0 {
		lines = append(lines, fmt.Sprintf(
			"Gözlem planı (aktif işlem sinyali değil — RR %.2f < 1.50): giriş %s–%s | stop %s | hedef %s",
			plan.RiskRewardRatio,
			retailPrice(plan.EntryMin, result.Currency),
			retailPrice(plan.EntryMax, result.Currency),
			retailPrice(plan.StopLoss, result.Currency),
			retailPrice(plan.TakeProfit1, result.Currency),
		))
	}
	// Valuation consistency warning.
	valuationConsistency := result.DecisionClassification.ValuationConsistency
	if valuationConsistency.Computed && !valuationConsistency.Publishable && valuationConsistency.MaxDivergencePct > 0 {
		lines = append(lines, fmt.Sprintf(
			"Uyarı: değerleme modelleri arasında %.1f%% model içi ayrışma var — fiyat hedefi yayımlanamaz. Fiyat/baz değer farkı ayrı metrik olarak okunmalıdır.",
			valuationConsistency.MaxDivergencePct,
		))
	}
	return lines
}

func nextResistance(tf analysis.TimeframeAnalysis) (ohlcv.SupportResistanceLevel, bool) {
	if len(tf.ResistanceLevels) == 0 {
		return ohlcv.SupportResistanceLevel{}, false
	}
	start := 0
	if tf.NearestResistance != nil {
		for i, level := range tf.ResistanceLevels {
			if level.Price == tf.NearestResistance.Price {
				start = i + 1
				break
			}
		}
	}
	for i := start; i < len(tf.ResistanceLevels); i++ {
		if tf.ResistanceLevels[i].Price > 0 {
			return tf.ResistanceLevels[i], true
		}
	}
	return ohlcv.SupportResistanceLevel{}, false
}

func nextSupport(tf analysis.TimeframeAnalysis) (ohlcv.SupportResistanceLevel, bool) {
	if len(tf.SupportLevels) == 0 {
		return ohlcv.SupportResistanceLevel{}, false
	}
	start := 0
	if tf.NearestSupport != nil {
		for i, level := range tf.SupportLevels {
			if level.Price == tf.NearestSupport.Price {
				start = i + 1
				break
			}
		}
	}
	for i := start; i < len(tf.SupportLevels); i++ {
		if tf.SupportLevels[i].Price > 0 {
			return tf.SupportLevels[i], true
		}
	}
	return ohlcv.SupportResistanceLevel{}, false
}

func nextSessionForecastLines(result analysis.SymbolAnalysis) []string {
	nsf, ok := symbolNextSessionForecast(result)
	if !ok {
		return nil
	}
	nsf = analysis.ApplyNextSessionForecastPublishState(nsf)
	cur := result.Currency
	if cur == "" {
		cur = "TL"
	}
	lines := []string{
		"Senaryo durumu: " + nextSessionScenarioUsageText(nsf),
		"Açılış fiyat senaryosu: " + nextSessionScenarioForecastText(nsf, "open", cur),
		"Kapanış fiyat senaryosu: " + nextSessionScenarioForecastText(nsf, "close", cur),
		fmt.Sprintf("Beklenen hareket bandı: %s – %s (ATR14 bazlı günlük beklenen aralık)",
			retailPrice(nsf.ExpectedLow, cur), retailPrice(nsf.ExpectedHigh, cur)),
		"Kapanış dağılımı P10/P50/P90: " + nextSessionForecastDistributionText(nsf.CloseP10, nsf.CloseP50, nsf.CloseP90, cur),
		"Yön olasılığı: " + nextSessionForecastProbabilityText(nsf),
		"Invalidasyon seviyesi: " + nextSessionForecastInvalidationText(nsf, cur),
		"Yön beklentisi: " + nextSessionDirectionDisplayText(nsf),
		fmt.Sprintf("Model güveni: %.0f/100 (%s, %d tarihsel açılış/kapanış örneği)", nsf.Confidence, emptyFallback(nsf.ConfidenceLabel, "etiket yok"), nsf.HistoricalSamples),
		"Tahmin kalitesi: " + nextSessionForecastQualityText(nsf),
	}
	if nsf.ForecastFor != "" {
		lines = append([]string{"Tahmin edilen seans: " + nsf.ForecastFor}, lines...)
	}
	if nsf.PivotS1 > 0 && nsf.PivotR1 > 0 {
		lines = append(lines, fmt.Sprintf("Pivot destek S1: %s | Pivot direnç R1: %s",
			retailPrice(nsf.PivotS1, cur), retailPrice(nsf.PivotR1, cur)))
	}
	for _, r := range nsf.BiasReasons {
		lines = append(lines, r)
	}
	return lines
}

func vapContextLines(result analysis.SymbolAnalysis) []string {
	if !isEquityResult(result) {
		return nil
	}
	lines := []string{}
	freeFloat := result.Professional.VAPFreeFloat
	if freeFloat.Computed {
		lines = append(lines, fmt.Sprintf(
			"Fiili dolaşım oranı: %.2f%% (%s); 20 gözlem değişimi %+.2f puan, likidite riski %s, arz sinyali %s.",
			freeFloat.FreeFloatRatioPct,
			emptyFallback(freeFloat.LatestDate, "tarih yok"),
			freeFloat.RatioChange20DPP,
			emptyFallback(freeFloat.LiquidityRisk, "bilinmiyor"),
			emptyFallback(freeFloat.SupplySignal, "bilinmiyor"),
		))
	} else if strings.TrimSpace(freeFloat.Summary) != "" {
		lines = append(lines, "Fiili dolaşım: "+reportText(freeFloat.Summary))
	}

	portfolio := result.Professional.VAPIndexPortfolio
	if portfolio.Computed {
		lines = append(lines, fmt.Sprintf(
			"BIST Endeks Portföyü: %s / %s; portföy değeri %.2f milyon TL, aylık değişim %+.2f%%, göreli momentum %+.2f puan, sinyal %s.",
			emptyFallback(portfolio.SelectedIndex, "endeks yok"),
			emptyFallback(portfolio.LatestMonth, "dönem yok"),
			portfolio.PortfolioValueMTL,
			portfolio.Change1MPct,
			portfolio.RelativeMomentum,
			emptyFallback(portfolio.Signal, "bilinmiyor"),
		))
	} else if strings.TrimSpace(portfolio.Summary) != "" {
		lines = append(lines, "BIST endeks portföyü: "+reportText(portfolio.Summary))
	}
	return lines
}

func symbolNextSessionForecast(result analysis.SymbolAnalysis) (analysis.NextSessionForecast, bool) {
	if result.NextSessionForecast.Computed && result.NextSessionForecast.PredictedOpen > 0 && result.NextSessionForecast.PredictedClose > 0 {
		return result.NextSessionForecast, true
	}
	if daily, ok := result.Timeframes["1D"]; ok {
		forecast := daily.NextSessionForecast
		if forecast.Computed && forecast.PredictedOpen > 0 && forecast.PredictedClose > 0 {
			return forecast, true
		}
	}
	return analysis.NextSessionForecast{}, false
}

func retailReasonLines(result analysis.SymbolAnalysis) []string {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		return []string{"Günlük veri olmadığı için karar sınırlı."}
	}
	lines := []string{}
	if tf.TrendBias == "bearish" {
		lines = append(lines, "Kısa vadeli fiyat yönü zayıf; sistem bu yüzden acele alım önermiyor.")
	} else if tf.TrendBias == "bullish" {
		if tf.Professional.Technical.SignalGate.Status != "" && (!reportStatusPass(tf.Professional.Technical.SignalGate.Status) || !tf.Professional.Technical.SignalGate.Actionable) {
			lines = append(lines, "Kısa vadeli toparlanma denemesi var; teknik sinyal kapısı ve işlem planı geçmediği için yeni alım kurulumu yok.")
		} else {
			lines = append(lines, "Kısa vadeli yön pozitif; yine de alım için giriş ve risk seviyesi doğrulanmalı.")
		}
	} else {
		lines = append(lines, "Kısa vadeli yön kararsız; net taraf seçilmemiş.")
	}
	if tf.LastClose > 0 && tf.Indicators.SMA20 > 0 && tf.LastClose < tf.Indicators.SMA20 {
		lines = append(lines, "Fiyat kısa vadeli ortalama maliyet çizgisinin altında; piyasa henüz güç kazanmış görünmüyor.")
	}
	if tf.LastClose > 0 && tf.Indicators.SMA50 > 0 && tf.LastClose < tf.Indicators.SMA50 {
		lines = append(lines, "Orta vadeli ortalamanın altında kalması, tepki yükselişlerinin kırılgan olduğunu gösteriyor.")
	}
	lines = append(lines, "Kısa vadeli güç: "+retailMomentumLine(tf)+".")
	lines = append(lines, "Alıcı ilgisi: "+retailMoneyFlowLine(tf)+".")
	if !activeSummaryTradePlan(tf.TradePlan) {
		lines = append(lines, "Sistem net giriş-stop-hedef üretmedi; bu yüzden yeni işlem sinyali yok.")
	}
	if line := dataImprovementLine(result); line != "" {
		lines = append(lines, line)
	}
	return lines
}

func retailDecisionChangeLines(result analysis.SymbolAnalysis) []string {
	lines := []string{}
	tradePlanBlocked := reportTradePlanBlocked(result)
	if tradePlanBlocked && len(result.InvestorQA.BuyConditions) > 0 {
		lines = append(lines, "BEKLE kararı AL'a döner: "+retailText(strings.Join(result.InvestorQA.BuyConditions, "; "))+".")
	} else if tradePlanBlocked {
		if tf, ok := result.Timeframes["1D"]; ok && tf.NearestResistance != nil {
			lines = append(lines, "BEKLE kararı AL'a yaklaşır: "+retailPrice(tf.NearestResistance.Price, result.Currency)+" üzerinde kapanış ve hacim teyidi görülürse.")
		}
	} else if len(result.InvestorQA.BuyConditions) > 0 {
		lines = append(lines, "AL'a yaklaşır: "+retailText(strings.Join(result.InvestorQA.BuyConditions, "; "))+".")
	} else if tf, ok := result.Timeframes["1D"]; ok && tf.NearestResistance != nil {
		lines = append(lines, "AL'a yaklaşır: "+retailPrice(tf.NearestResistance.Price, result.Currency)+" üzerinde kapanış görülürse.")
	}
	if len(result.InvestorQA.ExitConditions) > 0 {
		lines = append(lines, "Risk artar: "+retailText(strings.Join(result.InvestorQA.ExitConditions, "; "))+".")
	} else if tf, ok := result.Timeframes["1D"]; ok && tf.NearestSupport != nil {
		lines = append(lines, "Risk artar: "+retailPrice(tf.NearestSupport.Price, result.Currency)+" altında kapanış görülürse.")
	}
	if tf, ok := result.Timeframes["1D"]; ok && tf.Indicators.ChaikinMoneyFlow20 < 0 {
		lines = append(lines, "Ek teyit: alıcı ilgisinin güçlenmesi gerekir.")
	}
	if len(lines) == 0 {
		lines = append(lines, "Karar değişimi için fiyat, hacim ve risk seviyesi birlikte güçlenmeli.")
	}
	return lines
}

func retailActionTRForReport(action string) string {
	switch action {
	case "AL":
		return "AL"
	case "ALMA":
		return "ALMA"
	case "BEKLE":
		return "BEKLE"
	case "SAT_RISKI_AZALT":
		return "SAT / RİSKİ AZALT"
	case "TUT_POZISYONU_ARTIR":
		return "TUT / uygun girişte artır"
	case "TUT_RISKI_IZLE":
		return "TUT / riski izle"
	case "ISLEM_YAPMA":
		return "İŞLEM YAPMA"
	case "RISK_LIMITINI_KORU":
		return "RİSK LİMİTİNİ KORU"
	default:
		return reportLabel(action)
	}
}

func retailTimeframeLine(tf analysis.TimeframeAnalysis, currency string) string {
	decision := "aktif alım planı yok; teyit bekleniyor"
	if activeSummaryTradePlan(tf.TradePlan) {
		decision = "işlem planı var"
	} else if tf.TradePlan.Rejected && tf.TradePlan.RiskRewardRatio > 0 {
		decision = fmt.Sprintf("işlem kapısı kapalı; RR %.2f < 1.50", tf.TradePlan.RiskRewardRatio)
	} else if isSpotLongUnavailableReject(tf.TradePlan) {
		if tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
			decision = "aktif alım planı yok; " + reportPriceValue(tf.NearestResistance.Price) + " üstü kapanış ve hacim teyidi bekleniyor"
		} else {
			decision = "aktif alım planı yok; trend dönüşü ve hacim teyidi bekleniyor"
		}
	}
	return fmt.Sprintf("%s kapanış, teknik görünüm %s, skor %.0f/100; %s. %s", retailPrice(tf.LastClose, currency), retailTrendLabel(tf.TrendBias), tf.Score, decision, timeframePlainComment(tf))
}

func retailTimeframeLineForReport(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis, currency string) string {
	decision := "aktif alım planı yok; teyit bekleniyor"
	if reportTradePlanBlocked(result) && activeSummaryTradePlan(tf.TradePlan) {
		decision = "AL/SAT kapısı kapalı; teknik bant yalnızca izleme/teyit seviyesi"
	} else if activeSummaryTradePlan(tf.TradePlan) {
		decision = "işlem planı var"
	} else if tf.TradePlan.Rejected && tf.TradePlan.RiskRewardRatio > 0 {
		decision = fmt.Sprintf("işlem kapısı kapalı; RR %.2f < 1.50", tf.TradePlan.RiskRewardRatio)
	} else if isSpotLongUnavailableReject(tf.TradePlan) {
		if tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
			decision = "aktif alım planı yok; " + reportPriceValue(tf.NearestResistance.Price) + " üstü kapanış ve hacim teyidi bekleniyor"
		} else {
			decision = "aktif alım planı yok; trend dönüşü ve hacim teyidi bekleniyor"
		}
	}
	return fmt.Sprintf("%s kapanış, teknik görünüm %s, skor %.0f/100; %s. %s", retailPrice(tf.LastClose, currency), retailTrendLabel(tf.TrendBias), tf.Score, decision, timeframePlainCommentForReport(result, tf))
}

func retailQuestionAllowed(question string) bool {
	q := strings.ToLower(question)
	allowed := []string{
		"neden bugün almamalıyız",
		"hangi şart oluşursa",
		"hangi şartta tez bozulur",
		"likidite",
		"bu rapor",
		"temettü",
		"adil değer",
		"analist hedef",
		"değerleme çarpan",
		"52 haftalık",
		"teknik özet",
		"teknik gösterg",
		"hareketli ortalama",
		"rsi",
		"macd",
		"pivot",
		"canlı bist",
	}
	for _, key := range allowed {
		if strings.Contains(q, key) {
			return true
		}
	}
	return false
}

func retailGlossaryLines(result analysis.SymbolAnalysis) []string {
	lines := []string{
		"Destek: fiyatın tutunmaya çalıştığı alt bölge.",
		"Direnç: fiyatın aşmakta zorlanabileceği üst bölge.",
		"Ortalama: fiyatın son günlerdeki yaklaşık maliyet çizgisi.",
		"Alıcı ilgisi: işlem hacmi ve fiyat davranışından gelen destek işareti.",
		"Risk seviyesi: zarar büyümeden dikkat edilmesi gereken alt eşik.",
	}
	if result.InvestorQA.Computed {
		lines = append(lines, "Rapor güveni: verinin yeterliliği ve sinyallerin uyumunu gösterir; kesin kazanç anlamına gelmez.")
	}
	return lines
}

func retailTrendLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bullish":
		return "güçlenen"
	case "bearish":
		return "zayıf"
	case "neutral":
		return "kararsız"
	default:
		return localize.Bias(value)
	}
}

func retailMomentumLine(tf analysis.TimeframeAnalysis) string {
	parts := []string{}
	switch {
	case tf.Indicators.RSI14 > 0 && tf.Indicators.RSI14 < 35:
		parts = append(parts, "zayıf ve aşırı satışa yakın")
	case tf.Indicators.RSI14 > 0 && tf.Indicators.RSI14 < 45:
		parts = append(parts, "zayıf")
	case tf.Indicators.RSI14 >= 45 && tf.Indicators.RSI14 <= 55:
		parts = append(parts, "kararsız")
	case tf.Indicators.RSI14 > 70:
		parts = append(parts, "fazla ısınmış")
	case tf.Indicators.RSI14 > 55:
		parts = append(parts, "pozitif")
	}
	if tf.Indicators.MACDHistogram > 0 {
		parts = append(parts, "kısa vadeli toparlanma denemesi var")
	} else if tf.Indicators.MACDHistogram < 0 {
		parts = append(parts, "ivme henüz zayıf")
	}
	if len(parts) == 0 {
		return "net momentum sinyali yok"
	}
	return strings.Join(parts, "; ")
}

func retailMoneyFlowLine(tf analysis.TimeframeAnalysis) string {
	switch {
	case tf.Indicators.ChaikinMoneyFlow20 > 0.05:
		return "pozitif, alıcı ilgisi destekliyor"
	case tf.Indicators.ChaikinMoneyFlow20 < -0.05:
		return "zayıf, alıcı girişi yeterli değil"
	default:
		return "nötr, güçlü teyit yok"
	}
}

func retailVolatilityLine(tf analysis.TimeframeAnalysis) string {
	if tf.LastClose <= 0 || tf.Indicators.ATR14 <= 0 {
		return "ölçülemedi"
	}
	ratio := tf.Indicators.ATR14 / tf.LastClose
	switch {
	case ratio >= 0.10:
		return "çok yüksek"
	case ratio >= 0.04:
		return "yüksek"
	default:
		return "normal"
	}
}

func retailText(value string) string {
	value = redactInternalReportText(value)
	replacer := strings.NewReplacer(
		"teknik yapı", "fiyat hareketi",
		"Teknik yapı", "Fiyat hareketi",
		"on-chain", "blokzincir verisi",
		"On-chain", "Blokzincir verisi",
		"derivatives", "vadeli işlem verisi",
		"Derivatives", "Vadeli işlem verisi",
		"exchange-flow", "borsa giriş-çıkış verisi",
		"Exchange-flow", "Borsa giriş-çıkış verisi",
		"sentiment", "haber duyarlılığı",
		"Sentiment", "Haber duyarlılığı",
		"Trading edge", "İşlem avantajı",
		"trading edge", "işlem avantajı",
		"production trading", "canlı işlem",
		"bank_regulatory_metrics_missing", "banka çekirdek regülasyon/aktif kalitesi metrikleri eksik",
		"financial_data_not_production_ready", "finansal veri üretim kullanımı hazır değil",
		"walk_forward_sample_size_limited", "geçmiş test örnek sayısı sınırlı",
		"SYR_CET1_NPL_NIM_LCR_kredi_mevduat_structured_veri_eksik", "SYR/CET1/NPL/NIM/LCR/kredi-mevduat yapılandırılmış veri eksik",
		"capital_adequacy_ratio", "sermaye yeterlilik rasyosu",
		"cet1_ratio", "çekirdek sermaye oranı",
		"provision_coverage_ratio", "karşılık kapsam oranı",
		"net_interest_margin", "net faiz marjı",
		"liquidity_coverage_ratio", "likidite karşılama oranı",
		"loan_to_deposit_ratio", "kredi/mevduat oranı",
		"expectancy", "beklenen getiri",
		"out-of-sample", "ayrı test verisi",
		"backtest", "geçmiş test",
		"Backtest", "Geçmiş test",
		"momentum", "kısa vadeli güç",
		"Momentum", "Kısa vadeli güç",
		"para akışının", "alıcı ilgisinin",
		"Para akışının", "Alıcı ilgisinin",
		"para akışı", "alıcı ilgisi",
		"Para akışı", "Alıcı ilgisi",
		"volatilite", "hareketlilik",
		"Volatilite", "Hareketlilik",
		"likidite", "alım satım kolaylığı",
		"Likidite", "Alım satım kolaylığı",
		"technical", "teknik",
		"Technical", "Teknik",
		"teknik kanıt kapısı", "teyit kapısı",
		"Teknik kanıt kapısı", "Teyit kapısı",
		"teknik sinyal kapısı", "teyit kapısı",
		"Teknik sinyal kapısı", "Teyit kapısı",
		"trade plan reddi: short selling is not supported for this instrument", "trade plan reddi: aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor",
		"trade plan reddi: Spot varlıkta short/marjin planı üretilmez; aktif alım planı yok", "trade plan reddi: aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor",
		"trade plan reddi: neutral trend bias does not provide enough directional edge", "yön kararsız olduğu için işlem avantajı yok",
		"trade plan reddi: risk reward ratio is below 1.5", "risk/ödül oranı 1.5 altında",
		"short selling is not supported for this instrument", "aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor",
		"Spot varlıkta short/marjin planı üretilmez; aktif alım planı yok", "Aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor",
		"neutral trend bias does not provide enough directional edge", "yön kararsız olduğu için işlem avantajı yok",
		"risk reward ratio is below 1.5", "risk/ödül oranı 1.5 altında",
		"funding_open_interest_liquidations", "fonlama, açık pozisyon ve tasfiye verileri",
		"exchange_flow_reserve_netflow", "borsa rezerv ve para giriş-çıkış verileri",
		"crypto_news_", "kripto haber ",
		"onchain_mvrv_nupl_sopr_realized_cap", "blokzincir değerleme verileri",
		"usd_index_dxy_real_yield_macro", "DXY, reel faiz ve Fed beklentisi",
		"futures_cot_open_interest_positioning", "vadeli piyasa pozisyonu ve açık pozisyon",
		"gold_etf_physical_flow", "altın ETF ve fiziki talep akışı",
		"central_bank_geopolitical_news", "merkez bankası ve jeopolitik haber akışı",
	)
	return normalizeVisibleCurrencyText(strings.TrimSpace(replacer.Replace(value)))
}

func retailSignalGateBlockers(tf analysis.TimeframeAnalysis, limit int) string {
	blockers := reportLabels(limitStrings(tf.Professional.Technical.SignalGate.Blockers, limit))
	for i, blocker := range blockers {
		blockers[i] = retailSignalGateBlocker(blocker, tf.TradePlan)
	}
	return retailText(strings.Join(blockers, ", "))
}

func retailSignalGateBlocker(blocker string, plan ohlcv.TradePlan) string {
	lower := strings.ToLower(blocker)
	if strings.Contains(lower, "trade plan reddi") || strings.Contains(lower, "plan reddedildi") {
		if plan.RejectReason != "" {
			return "trade plan reddi: " + reportPlanRejectSummary(plan)
		}
	}
	if strings.Contains(lower, "risk/ödül 0.00") && !activeSummaryTradePlan(plan) {
		return "risk/getiri henüz ölçülemedi; giriş/stop/hedef planı yok"
	}
	return blocker
}

func patternMarkdownLine(pattern ohlcv.PatternResult) string {
	return strings.Join([]string{
		"Adı: " + localize.PatternName(pattern.Name),
		"Yönü: " + localize.Direction(pattern.Direction),
		"Güven skoru: " + fmt.Sprintf("%.0f/100", pattern.Confidence*100),
		"Hacim teyidi: " + patternVolumeStatus(pattern),
		"Teyit eden indikatörler: " + compactPatternContext(pattern.ConfirmingIndicators),
		"Geçersiz kılan karşı sinyaller: " + compactPatternContext(pattern.InvalidatingSignals),
		"İşlem değeri: " + patternTradeValueStatus(pattern),
		"İstatistik: " + compactPatternValidation(pattern),
	}, " | ")
}

func compactPatternValidation(pattern ohlcv.PatternResult) string {
	if pattern.ValidationStatus == "" {
		return "yok"
	}
	if pattern.BacktestSampleSize <= 0 {
		return pattern.ValidationStatus
	}
	return fmt.Sprintf("%s n=%d win=%.1f%% exp=%.2fR p=%.3f",
		pattern.ValidationStatus,
		pattern.BacktestSampleSize,
		pattern.BacktestWinRate*100,
		pattern.BacktestExpectancyR,
		pattern.ValidationPValue,
	)
}

func compactPatternContext(values []string) string {
	parts := make([]string, 0, 3)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, value)
		if len(parts) >= 3 {
			break
		}
	}
	if len(parts) == 0 {
		return "yok"
	}
	if len(values) > len(parts) {
		parts = append(parts, fmt.Sprintf("+%d", len(values)-len(parts)))
	}
	return strings.Join(parts, "; ")
}

func reportKindTitle(result analysis.SymbolAnalysis) string {
	if isCryptoResult(result) {
		return "Kripto Teknik Analiz Raporu"
	}
	if isCommodityResult(result) {
		return "Altın/Emtia Teknik Analiz Raporu"
	}
	return "BIST Teknik Analiz Raporu"
}

func entityNameLabel(result analysis.SymbolAnalysis) string {
	if isCryptoResult(result) || isCommodityResult(result) {
		return "Varlık adı"
	}
	return "Şirket adı"
}

func assetTypeLabel(result analysis.SymbolAnalysis) string {
	if isCryptoResult(result) {
		return "Kripto"
	}
	if isCommodityResult(result) {
		return "Altın/Emtia"
	}
	return "Hisse"
}

func isCryptoResult(result analysis.SymbolAnalysis) bool {
	return ohlcv.IsCryptoAssetType(result.AssetType)
}

func isCommodityResult(result analysis.SymbolAnalysis) bool {
	return ohlcv.IsCommodityAssetType(result.AssetType)
}

func isEquityResult(result analysis.SymbolAnalysis) bool {
	return !isCryptoResult(result) && !isCommodityResult(result)
}

func isMarketOnlyResult(result analysis.SymbolAnalysis) bool {
	return isCryptoResult(result) || isCommodityResult(result)
}

func marketAssetLabel(result analysis.SymbolAnalysis) string {
	if isCryptoResult(result) {
		return "Kripto"
	}
	if isCommodityResult(result) {
		return "Altın/emtia"
	}
	return "Varlık"
}

func dataImprovementLine(result analysis.SymbolAnalysis) string {
	missing := missingDataLabels(result)
	if len(missing) == 0 {
		return ""
	}
	if isCryptoResult(result) {
		return "Veri geliştirilebilir: " + strings.Join(missing, ", ") + " bağlanırsa rapor güveni artar; mevcut sonuç fiyat/hacim analizidir."
	}
	if isCommodityResult(result) {
		return "Veri geliştirilebilir: " + strings.Join(missing, ", ") + " bağlanırsa altın analizi güçlenir; mevcut sonuç TradingView fiyat grafiği analizidir."
	}
	return ""
}

func missingDataLabels(result analysis.SymbolAnalysis) []string {
	out := []string{}
	for _, item := range result.Professional.Coverage.Missing {
		item = strings.ToLower(strings.TrimSpace(item))
		switch item {
		case "onchain_mvrv_nupl_sopr_realized_cap":
			out = append(out, "blokzincir değerleme verileri")
		case "derivatives_funding_open_interest_liquidations":
			out = append(out, "fonlama, açık pozisyon ve tasfiye verileri")
		case "exchange_flow_reserve_netflow":
			out = append(out, "borsa rezerv ve giriş-çıkış verileri")
		case "crypto_news_sentiment":
			out = append(out, "kripto haber duyarlılığı")
		case "usd_index_dxy_real_yield_macro":
			out = append(out, "DXY, reel faiz ve Fed beklentisi")
		case "futures_cot_open_interest_positioning":
			out = append(out, "vadeli piyasa pozisyonu ve açık pozisyon")
		case "gold_etf_physical_flow":
			out = append(out, "altın ETF ve fiziki talep akışı")
		case "central_bank_geopolitical_news":
			out = append(out, "merkez bankası ve jeopolitik haber akışı")
		default:
			out = append(out, reportLabel(item))
		}
	}
	return out
}

func activeSummaryTradePlan(plan ohlcv.TradePlan) bool {
	return !plan.Rejected && plan.Direction != "" && plan.Direction != "neutral" && plan.EntryMin > 0 && plan.EntryMax > 0 && plan.StopLoss > 0
}

func turkishAnalysis(result analysis.SymbolAnalysis) map[string]any {
	timeframes := map[string]any{}
	keys := make([]string, 0, len(result.Timeframes))
	for key := range result.Timeframes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		tf := result.Timeframes[key]
		timeframes[localize.Timeframe(key)] = map[string]any{
			"zaman_dilimi":          localize.Timeframe(tf.Timeframe),
			"son_kapanis":           tf.LastClose,
			"son_hacim":             tf.LastVolume,
			"mum_sayisi":            tf.CandleCount,
			"trend_yonu":            localize.Bias(tf.TrendBias),
			"skor":                  tf.Score,
			"gostergeler":           turkishIndicators(tf.Indicators),
			"indicator_taramasi":    turkishIndicatorSignals(tf.IndicatorSignals),
			"formasyon_taramasi":    turkishPatternScans(tf.PatternScans),
			"formasyonlar":          turkishPatterns(tf.Patterns),
			"destek_seviyeleri":     turkishLevels(tf.SupportLevels),
			"direnc_seviyeleri":     turkishLevels(tf.ResistanceLevels),
			"en_yakin_destek":       turkishLevelPtr(tf.NearestSupport),
			"en_yakin_direnc":       turkishLevelPtr(tf.NearestResistance),
			"islem_plani":           turkishTradePlan(tf.TradePlan),
			"sonraki_seans_tahmini": turkishNextSessionForecast(tf.NextSessionForecast),
			"profesyonel":           turkishTimeframeProfessional(tf.Professional),
		}
	}
	nextForecast, _ := symbolNextSessionForecast(result)
	out := map[string]any{
		"sembol":                result.Symbol,
		"varlik_adi":            result.CompanyName,
		"varlik_tipi":           assetTypeLabel(result),
		"ham_varlik_tipi":       result.AssetType,
		"borsa_kaynak":          result.Exchange,
		"analiz_tarihi":         result.AnalysisDate,
		"para_birimi":           result.Currency,
		"fiyat":                 primaryLastClose(result),
		"son_kapanis":           primaryLastClose(result),
		"karar":                 retailDecisionLabel(result),
		"karar_guveni":          retailConfidence(result),
		"genel_skor":            result.OverallScore,
		"genel_yon":             localize.Bias(result.OverallBias),
		"sonraki_seans_tahmini": turkishNextSessionForecast(nextForecast),
		"ml_forecast":           result.MLForecast,
		"zaman_dilimleri":       timeframes,
		"profesyonel_analiz":    turkishProfessional(result),
		"yatirimci_soru_cevap":  turkishInvestorQA(result),
		"davranissal_analiz":    turkishBehavioral(result),
		"kurumsal_dogrulama":    turkishInstitutionalValidation(result.InstitutionalValidation),
		"uyari":                 result.Disclaimer,
	}
	if !isCryptoResult(result) {
		out["sirket_adi"] = result.CompanyName
	}
	if result.MatriksFormations != nil {
		out["matriks_formasyonlari"] = result.MatriksFormations
	}
	if result.BISTBulletin.Computed || len(result.BISTBulletin.Warnings) > 0 {
		out["bist_resmi_bulten"] = turkishBISTBulletin(result.BISTBulletin)
	}
	return out
}

func turkishNextSessionForecast(forecast analysis.NextSessionForecast) map[string]any {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	return map[string]any{
		"hesaplandi":                            forecast.Computed,
		"tahmin_edilen_seans":                   forecast.ForecastFor,
		"son_kapanis":                           forecast.LastClose,
		"senaryo_kullanim_durumu":               nextSessionScenarioUsageText(forecast),
		"karar_sonucu":                          nextSessionDecisionOutcomeText(forecast),
		"nokta_tahmin_yayinlanabilir":           forecast.PointForecastPublishable,
		"nokta_tahmin_yayin_durumu":             nextSessionTurkishPointForecastStatus(forecast),
		"nokta_tahmin_bastirma_nedeni":          nextSessionTurkishPointForecastReason(forecast),
		"yayinlanan_tahmini_acilis":             nextSessionPublishedValue(forecast.PublishedPredictedOpen),
		"yayinlanan_tahmini_kapanis":            nextSessionPublishedValue(forecast.PublishedPredictedClose),
		"tahmini_acilis":                        nextSessionPublishedValue(forecast.PublishedPredictedOpen),
		"tahmini_kapanis":                       nextSessionPublishedValue(forecast.PublishedPredictedClose),
		"senaryo_acilis":                        nextSessionScenarioValue(forecast, "open"),
		"senaryo_kapanis":                       nextSessionScenarioValue(forecast, "close"),
		"ham_tahmini_acilis":                    nextSessionFutureScenarioNumberValue(forecast, forecast.RawPredictedOpen),
		"ham_tahmini_kapanis":                   nextSessionFutureScenarioNumberValue(forecast, forecast.RawPredictedClose),
		"islem_gorebilir_tahmini_acilis":        nextSessionFutureScenarioNumberValue(forecast, forecast.TradablePredictedOpen),
		"islem_gorebilir_tahmini_kapanis":       nextSessionFutureScenarioNumberValue(forecast, forecast.TradablePredictedClose),
		"acilis_degisim_yuzde":                  nextSessionPublishedChangeValue(forecast.PublishedPredictedOpen, forecast.LastClose),
		"kapanis_degisim_yuzde":                 nextSessionPublishedChangeValue(forecast.PublishedPredictedClose, forecast.LastClose),
		"senaryo_acilis_degisim_yuzde":          nextSessionScenarioChangeValue(forecast, "open"),
		"senaryo_kapanis_degisim_yuzde":         nextSessionScenarioChangeValue(forecast, "close"),
		"tahmini_acilis_yonu":                   nextSessionPublishedDirectionValue(forecast, forecast.PredictedOpenDirection),
		"tahmini_kapanis_yonu":                  nextSessionPublishedDirectionValue(forecast, forecast.PredictedCloseDirection),
		"senaryo_acilis_yonu":                   nextSessionScenarioDirectionValue(forecast, forecast.PredictedOpenDirection),
		"senaryo_kapanis_yonu":                  nextSessionScenarioDirectionValue(forecast, forecast.PredictedCloseDirection),
		"yon_toleransi_yuzde":                   nextSessionFutureScenarioNumberValue(forecast, forecast.DirectionTolerancePct),
		"ham_acilis_degisim_yuzde":              nextSessionAuditChangeValue(forecast, forecast.RawPredictedOpen),
		"ham_kapanis_degisim_yuzde":             nextSessionAuditChangeValue(forecast, forecast.RawPredictedClose),
		"islem_gorebilir_acilis_degisim_yuzde":  nextSessionPublishedChangeValue(forecast.PublishedPredictedOpen, forecast.LastClose),
		"islem_gorebilir_kapanis_degisim_yuzde": nextSessionPublishedChangeValue(forecast.PublishedPredictedClose, forecast.LastClose),
		"beklenen_en_dusuk":                     forecast.ExpectedLow,
		"beklenen_en_yuksek":                    forecast.ExpectedHigh,
		"acilis_p10":                            forecast.OpenP10,
		"acilis_p50":                            forecast.OpenP50,
		"acilis_p90":                            forecast.OpenP90,
		"kapanis_p10":                           forecast.CloseP10,
		"kapanis_p50":                           forecast.CloseP50,
		"kapanis_p90":                           forecast.CloseP90,
		"yukari_olasilik_yuzde":                 nextSessionFutureScenarioNumberValue(forecast, forecast.UpsideProbabilityPct),
		"yatay_olasilik_yuzde":                  nextSessionFutureScenarioNumberValue(forecast, forecast.FlatProbabilityPct),
		"asagi_olasilik_yuzde":                  nextSessionFutureScenarioNumberValue(forecast, forecast.DownsideProbabilityPct),
		"dagilim_ornek_sayisi":                  forecast.ForecastDistributionSamples,
		"invalidasyon_seviyesi":                 forecast.InvalidationLevel,
		"invalidasyon_nedeni":                   emptyStringAsNil(forecast.InvalidationReason),
		"ham_beklenen_en_dusuk":                 forecast.RawExpectedLow,
		"ham_beklenen_en_yuksek":                forecast.RawExpectedHigh,
		"islem_gorebilir_beklenen_en_dusuk":     forecast.TradableExpectedLow,
		"islem_gorebilir_beklenen_en_yuksek":    forecast.TradableExpectedHigh,
		"fiyat_adimi":                           forecast.TickSize,
		"yuvarlama_yontemi":                     forecast.RoundingMethod,
		"fiyat_adimi_kurali":                    forecast.PriceStepRule,
		"durum":                                 nextSessionTurkishForecastStatus(forecast),
		"kalite":                                nextSessionTurkishForecastQuality(forecast),
		"yon_beklentisi":                        nextSessionScenarioDirectionValue(forecast, forecast.DirectionBias),
		"yon_gucu":                              nextSessionScenarioDirectionValue(forecast, forecast.BiasStrength),
		"guven":                                 forecast.Confidence,
		"guven_etiketi":                         forecast.ConfidenceLabel,
		"tarihsel_ornek_sayisi":                 forecast.HistoricalSamples,
		"dogrulama_durumu":                      forecast.ValidationStatus,
		"dogrulama_kaynagi":                     forecast.ValidationSource,
		"gerceklesen_var":                       forecast.ActualAvailable,
		"gerceklesen_acilis":                    nextSessionActualValue(forecast, forecast.ActualOpen),
		"gerceklesen_kapanis":                   nextSessionActualValue(forecast, forecast.ActualClose),
		"gerceklesen_kaynak":                    emptyStringAsNil(forecast.ActualSource),
		"gerceklesen_kaynak_dosyasi":            emptyStringAsNil(forecast.ActualSourcePath),
		"kesinlesmis_resmi_sonuc": map[string]any{
			"var":                        forecast.ActualAvailable,
			"otoritatif":                 forecast.ActualAvailable,
			"durum":                      nextSessionOfficialResultStatus(forecast),
			"hesaplama_modu":             nextSessionOfficialCalculationMode(forecast),
			"resmi_acilis":               nextSessionActualValue(forecast, forecast.ActualOpen),
			"resmi_kapanis":              nextSessionActualValue(forecast, forecast.ActualClose),
			"resmi_kaynak":               emptyStringAsNil(forecast.ActualSource),
			"resmi_kaynak_dosyasi":       emptyStringAsNil(forecast.ActualSourcePath),
			"denetim_tahmini_acilis":     nextSessionAuditValue(forecast, forecast.PredictedOpen),
			"denetim_tahmini_kapanis":    nextSessionAuditValue(forecast, forecast.PredictedClose),
			"denetim_acilis_hata_tl":     nextSessionAuditMetricValue(forecast, forecast.OpenForecastErrorTL),
			"denetim_kapanis_hata_tl":    nextSessionAuditMetricValue(forecast, forecast.CloseForecastErrorTL),
			"denetim_acilis_hata_yuzde":  nextSessionAuditMetricValue(forecast, forecast.OpenAbsErrorPctVsActual),
			"denetim_kapanis_hata_yuzde": nextSessionAuditMetricValue(forecast, forecast.CloseAbsErrorPctVsActual),
		},
		"gerceklesen_acilis_hata_yuzde":                  nextSessionAuditMetricValue(forecast, forecast.ActualOpenErrorPct),
		"gerceklesen_kapanis_hata_yuzde":                 nextSessionAuditMetricValue(forecast, forecast.ActualCloseErrorPct),
		"acilis_hata_tl":                                 nextSessionAuditMetricValue(forecast, forecast.OpenForecastErrorTL),
		"kapanis_hata_tl":                                nextSessionAuditMetricValue(forecast, forecast.CloseForecastErrorTL),
		"acilis_mutlak_hata_yuzde_gercege_gore":          nextSessionAuditMetricValue(forecast, forecast.OpenAbsErrorPctVsActual),
		"kapanis_mutlak_hata_yuzde_gercege_gore":         nextSessionAuditMetricValue(forecast, forecast.CloseAbsErrorPctVsActual),
		"acilis_mutlak_hata_yuzde_onceki_kapanisa_gore":  nextSessionAuditMetricValue(forecast, forecast.OpenAbsErrorPctVsPreviousClose),
		"kapanis_mutlak_hata_yuzde_onceki_kapanisa_gore": nextSessionAuditMetricValue(forecast, forecast.CloseAbsErrorPctVsPreviousClose),
		"acilis_yon_uyumu":                               nextSessionOptionalBool(forecast.OpenDirectionHit),
		"kapanis_yon_uyumu":                              nextSessionOptionalBool(forecast.CloseDirectionHit),
		"backtest_ornek_sayisi":                          forecast.BacktestSamples,
		"backtest_kaynagi":                               forecast.BacktestSource,
		"backtest_acilis_mae_yuzde":                      forecast.BacktestOpenMAEPct,
		"backtest_kapanis_mae_yuzde":                     forecast.BacktestCloseMAEPct,
		"backtest_yon_uyum_yuzde":                        forecast.BacktestDirectionHitRatePct,
		"model":                                          forecast.Model,
		"mikroyapi": map[string]any{
			"acilis_eslesme_fiyati":      forecast.OpeningAuctionEquilibriumPrice,
			"order_book_dengesizligi":    forecast.OrderBookImbalance,
			"ihale_hacim_baskisi":        forecast.AuctionVolumePressure,
			"mikroyapi_fiyat_duzeltmesi": forecast.MicrostructureAdjustment,
		},
		"gerekceler": forecast.BiasReasons,
		"uyarilar":   reportLabels(forecast.Warnings),
	}
}

func nextSessionPublishedChangeValue(price *float64, lastClose float64) any {
	if price == nil || *price <= 0 || lastClose <= 0 {
		return nil
	}
	return forecastChangePct(*price, lastClose)
}

func nextSessionScenarioValue(forecast analysis.NextSessionForecast, field string) any {
	return positiveFloatAsAny(nextSessionScenarioPrice(forecast, field))
}

func nextSessionScenarioChangeValue(forecast analysis.NextSessionForecast, field string) any {
	price := nextSessionScenarioPrice(forecast, field)
	if price <= 0 || forecast.LastClose <= 0 {
		return nil
	}
	return forecastChangePct(price, forecast.LastClose)
}

func nextSessionScenarioDirectionValue(forecast analysis.NextSessionForecast, value string) any {
	if forecast.ActualAvailable {
		return nil
	}
	value = strings.TrimSpace(value)
	if value == "" && forecast.Computed {
		value = inferredNextSessionDirection(forecast)
	}
	if value == "" {
		return nil
	}
	return value
}

func nextSessionTurkishPointForecastStatus(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	switch {
	case !forecast.Computed:
		return "hesaplanamadi"
	case forecast.ActualAvailable:
		return "resmi_sonuc_var"
	case forecast.PointForecastPublishable:
		return "karar_kalitesinde_yayinda"
	default:
		return "senaryo_uretildi"
	}
}

func nextSessionTurkishPointForecastReason(forecast analysis.NextSessionForecast) any {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if forecast.PointForecastPublishable {
		return nil
	}
	return nextSessionScenarioUsageText(forecast)
}

func nextSessionTurkishForecastStatus(forecast analysis.NextSessionForecast) string {
	return reportLabel(forecast.Status)
}

func nextSessionTurkishForecastQuality(forecast analysis.NextSessionForecast) string {
	return reportLabel(forecast.Quality)
}

func nextSessionFutureScenarioNumberValue(forecast analysis.NextSessionForecast, value float64) any {
	if forecast.ActualAvailable {
		return nil
	}
	return positiveFloatAsAny(value)
}

func positiveFloatAsAny(value float64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nextSessionAuditValue(forecast analysis.NextSessionForecast, value float64) any {
	if !forecast.PointForecastPublishable || !forecast.ActualAvailable || value <= 0 {
		return nil
	}
	return value
}

func nextSessionAuditChangeValue(forecast analysis.NextSessionForecast, value float64) any {
	if !forecast.PointForecastPublishable || !forecast.ActualAvailable || value <= 0 || forecast.LastClose <= 0 {
		return nil
	}
	return forecastChangePct(value, forecast.LastClose)
}

func nextSessionAuditMetricValue(forecast analysis.NextSessionForecast, value float64) any {
	if !forecast.PointForecastPublishable || !forecast.ActualAvailable || value == 0 {
		return nil
	}
	return value
}

func nextSessionPublishedDirectionValue(forecast analysis.NextSessionForecast, value string) any {
	value = strings.TrimSpace(value)
	if !forecast.PointForecastPublishable || value == "" {
		return nil
	}
	return value
}

func nextSessionPublishedNumberValue(forecast analysis.NextSessionForecast, value float64) any {
	if !forecast.PointForecastPublishable || value == 0 {
		return nil
	}
	return value
}

func turkishBISTBulletin(context analysis.BISTBulletinContext) map[string]any {
	return map[string]any{
		"hesaplandi":                    context.Computed,
		"kaynak":                        context.Source,
		"kayit_sayisi":                  context.RecordCount,
		"kapsam_baslangic":              context.CoverageStart,
		"kapsam_bitis":                  context.CoverageEnd,
		"son_resmi_kayit":               context.LatestRecord,
		"tahmin_seansi_gerceklesen_var": context.ForecastActualAvailable,
		"tahmin_seansi_gerceklesen":     context.ForecastActualRecord,
		"resmi_kapanis_mutabik":         context.OfficialCloseConfirmed,
		"resmi_kapanis_fark_yuzde":      context.OfficialCloseDeltaPct,
		"son_spread_bps":                context.LatestObservedSpreadBps,
		"son_aof":                       context.LatestVWAP,
		"son_acilis_seansi_miktari":     context.LatestOpeningSessionVolume,
		"son_kapanis_seansi_miktari":    context.LatestClosingSessionVolume,
		"uyarilar":                      context.Warnings,
	}
}

func turkishInvestorQA(result analysis.SymbolAnalysis) any {
	qa := result.InvestorQA
	if !isCryptoResult(result) {
		return qa
	}
	return map[string]any{
		"computed":            qa.Computed,
		"symbol":              qa.Symbol,
		"decision":            qa.Decision,
		"decision_label":      qa.DecisionLabel,
		"one_line_answer":     qa.OneLineAnswer,
		"investor_profile":    qa.InvestorProfile,
		"score":               qa.Score,
		"confidence":          qa.Confidence,
		"top_opportunity":     qa.TopOpportunity,
		"top_risk":            qa.TopRisk,
		"buy_conditions":      qa.BuyConditions,
		"exit_conditions":     qa.ExitConditions,
		"action_matrix":       qa.ActionMatrix,
		"questions":           qa.Questions,
		"quality":             qa.Quality,
		"macro":               qa.Macro,
		"liquidity":           qa.Liquidity,
		"crypto_data_context": map[string]any{"score": qa.Governance.Score, "label": qa.Governance.Label, "source_status": qa.Governance.Disclosure, "risk_flags": qa.Governance.RiskFlags, "data_lineage": qa.Governance.DataLineage},
		"scenario":            qa.Scenario,
		"model_risk":          qa.ModelRisk,
		"institutional_views": qa.InstitutionalViews,
		"checks":              qa.Checks,
		"warnings":            qa.Warnings,
	}
}

func turkishInstitutionalValidation(validation analysis.InstitutionalValidation) map[string]any {
	checks := make([]map[string]any, 0, len(validation.Checks))
	for _, check := range validation.Checks {
		checks = append(checks, map[string]any{
			"ad":        check.Name,
			"durum":     localizeInstitutionalStatus(check.Status),
			"seviye":    check.Severity,
			"aciklama":  check.Message,
			"detaylar":  check.Details,
			"ham_durum": check.Status,
		})
	}
	return map[string]any{
		"durum":      localizeInstitutionalStatus(validation.Status),
		"ham_durum":  validation.Status,
		"skor":       validation.Score,
		"mod":        validation.Mode,
		"ozet":       validation.Summary,
		"kontroller": checks,
	}
}

func localizeInstitutionalStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass":
		return "Geçti"
	case "limited":
		return "Sınırlı"
	case "fail":
		return "Başarısız"
	case "not_applicable", "not applicable":
		return "Uygulanmaz"
	default:
		return "Yok"
	}
}

func turkishBehavioral(result analysis.SymbolAnalysis) map[string]any {
	behavioral := result.Behavioral
	sourceCoverage := map[string]any{
		"normalize_haber_sayisi":     behavioral.SourceCoverage.NewsItemCount,
		"yorum_sayisi":               behavioral.SourceCoverage.CommentCount,
		"guncel_metin_sayisi":        behavioral.SourceCoverage.RecentTextCount,
		"analiz_edilen_metin_sayisi": behavioral.SourceCoverage.AnalyzedTextCount,
		"yorum_verisi_var":           localize.Bool(behavioral.SourceCoverage.HasCommentData),
		"guncel_sentiment_var":       localize.Bool(behavioral.SourceCoverage.HasRecentSentiment),
		"uyarilar":                   behavioral.SourceCoverage.Warnings,
	}
	if !isCryptoResult(result) {
		sourceCoverage["kap_aciklama_sayisi"] = behavioral.SourceCoverage.KAPDisclosureCount
	}
	return map[string]any{
		"kaynak_kapsami": sourceCoverage,
		"sentiment": map[string]any{
			"skor":               behavioral.Sentiment.Score,
			"olumsuzluk":         behavioral.Sentiment.Negativity,
			"etiket":             behavioral.Sentiment.Label,
			"olumlu_eslesme":     behavioral.Sentiment.PositiveHits,
			"olumsuz_eslesme":    behavioral.Sentiment.NegativeHits,
			"panik_eslesme":      behavioral.Sentiment.PanicHits,
			"sade_yorum":         behavioral.Sentiment.PlainLanguage,
			"en_guclu_sinyaller": behavioral.Sentiment.TopSignals,
		},
		"kapitulasyon": map[string]any{
			"skor":       behavioral.Capitulation.Score,
			"etiket":     behavioral.Capitulation.Label,
			"bilesenler": behavioral.Capitulation.Components,
			"kanitlar":   behavioral.Capitulation.Evidence,
			"sade_yorum": behavioral.Capitulation.PlainLanguage,
		},
		"tersine_donus": map[string]any{
			"skor":          behavioral.Contrarian.Score,
			"etiket":        behavioral.Contrarian.Label,
			"sinyal":        behavioral.Contrarian.Signal,
			"aksiyon":       behavioral.Contrarian.Action,
			"kalite_kapisi": behavioral.Contrarian.QualityGate,
			"sade_yorum":    behavioral.Contrarian.PlainLanguage,
			"kanitlar":      behavioral.Contrarian.Evidence,
		},
		"proxy_backtest": behavioral.Backtest,
	}
}

func turkishTimeframeProfessional(pro professional.TimeframeReport) map[string]any {
	return map[string]any{
		"teknik_kanit": turkishTechnicalEvidence(pro.Technical),
		"likidite": map[string]any{
			"son_islem_hacmi":           pro.Liquidity.LastValueTradedTRY,
			"ortalama_hacim_20":         pro.Liquidity.AverageVolume20,
			"ortalama_islem_hacmi_20":   pro.Liquidity.AverageValueTraded20TRY,
			"medyan_islem_hacmi_20":     pro.Liquidity.MedianValueTraded20TRY,
			"hacim_ortalama_orani":      pro.Liquidity.VolumeVsAverage20,
			"amihud_likiditesizlik_20":  pro.Liquidity.AmihudIlliquidity20,
			"adv_10_yuzde_kapasite":     pro.Liquidity.CapacityTRYAt10PctADV,
			"son_islem_hacmi_try":       pro.Liquidity.LastValueTradedTRY,
			"adv_10_yuzde_kapasite_try": pro.Liquidity.CapacityTRYAt10PctADV,
		},
		"pozisyon_boyutu": map[string]any{
			"portfoy":                pro.PositionSizing.PortfolioValue,
			"risk_yuzdesi":           pro.PositionSizing.RiskPerTradePct,
			"risk_butcesi":           pro.PositionSizing.RiskBudget,
			"giris":                  pro.PositionSizing.Entry,
			"stop":                   pro.PositionSizing.Stop,
			"lot":                    pro.PositionSizing.Quantity,
			"nominal":                pro.PositionSizing.Notional,
			"portfoy_yuzdesi":        pro.PositionSizing.PortfolioPct,
			"likidite_limit_nominal": pro.PositionSizing.LiquidityCapNotional,
			"uyarilar":               pro.PositionSizing.Warnings,
		},
		"backtest": map[string]any{
			"strateji":             pro.Backtest.Strategy,
			"execution_model":      pro.Backtest.ExecutionModel,
			"backtest_guvenli":     pro.Backtest.BacktestSafe,
			"komisyon_bps":         pro.Backtest.CommissionBps,
			"slippage_bps":         pro.Backtest.SlippageBps,
			"lookahead_ihlali":     pro.Backtest.LookaheadViolations,
			"bar_sayisi":           pro.Backtest.LookbackBars,
			"islem_sayisi":         pro.Backtest.Trades,
			"kazanma_orani":        pro.Backtest.WinRate,
			"ortalama_getiri":      pro.Backtest.AverageReturn,
			"medyan_getiri":        pro.Backtest.MedianReturn,
			"profit_factor":        pro.Backtest.ProfitFactor,
			"maksimum_dusus":       pro.Backtest.MaxDrawdown,
			"cagr":                 pro.Backtest.CAGR,
			"volatilite":           pro.Backtest.Volatility,
			"sharpe":               pro.Backtest.Sharpe,
			"sortino":              pro.Backtest.Sortino,
			"exposure":             pro.Backtest.Exposure,
			"in_sample_islem":      pro.Backtest.InSampleTrades,
			"out_of_sample_islem":  pro.Backtest.OutOfSampleTrades,
			"out_of_sample_getiri": pro.Backtest.OutOfSampleReturn,
			"beklenti":             pro.Backtest.Expectancy,
			"ortalama_tutma_bar":   pro.Backtest.AvgHoldingBars,
		},
		"fiyat_duzeltme_kontrolu": map[string]any{
			"duzeltilmis_mum":             pro.PriceAdjustment.AdjustedCandles,
			"duzeltilmemis_mum":           pro.PriceAdjustment.UnadjustedCandles,
			"potansiyel_bolunme_gap_bari": pro.PriceAdjustment.PotentialSplitGapBars,
			"kullanilan_fiyat_serisi":     pro.PriceAdjustment.PriceSeries,
			"aksiyon_sayisi":              pro.PriceAdjustment.ActionsConsidered,
			"uygulanan_aksiyon":           pro.PriceAdjustment.AppliedActions,
			"atlanan_aksiyon":             pro.PriceAdjustment.SkippedActions,
			"backtest_guvenli":            pro.PriceAdjustment.BacktestSafe,
			"uyarilar":                    pro.PriceAdjustment.Warnings,
		},
		"sinyal_istatistigi": map[string]any{
			"mevcut_rejim":          pro.SignalStats.CurrentRegime,
			"ornek_sayisi":          pro.SignalStats.SampleSize,
			"ileri_bar":             pro.SignalStats.ForwardBars,
			"kazanma_orani":         pro.SignalStats.WinRate,
			"ortalama_ileri_getiri": pro.SignalStats.AverageForwardReturn,
			"medyan_ileri_getiri":   pro.SignalStats.MedianForwardReturn,
			"olasilik_skoru":        pro.SignalStats.ProbabilityScore,
			"veri_yetersiz":         pro.SignalStats.InsufficientData,
		},
	}
}

func turkishTechnicalEvidence(technical professional.TechnicalEvidence) map[string]any {
	return map[string]any{
		"ozet": technical.Summary,
		"skor": map[string]any{
			"toplam":           technical.Score.Total,
			"trend":            technical.Score.Trend,
			"momentum":         technical.Score.Momentum,
			"hacim":            technical.Score.Volume,
			"volatilite_riski": technical.Score.VolatilityRisk,
			"formasyon":        technical.Score.Pattern,
		},
		"secilen_indikatorler": turkishProfessionalIndicators(technical.SelectedIndicators),
		"secilen_formasyonlar": turkishProfessionalPatterns(technical.SelectedPatterns),
		"dogrulama":            turkishTechnicalValidation(technical.Validation),
		"sinyal_kapisi":        turkishTechnicalSignalGate(technical.SignalGate),
		"sinyal_sayilari":      technical.SignalCounts,
		"formasyon_sayilari":   technical.PatternCounts,
		"guardrail_uyarilari":  reportLabels(technical.Guardrails),
	}
}

func turkishTechnicalValidation(validation professional.TechnicalValidationReport) map[string]any {
	if validation.Status == "" {
		return map[string]any{"durum": "Yok", "ham_durum": ""}
	}
	return map[string]any{
		"durum":                      localizeInstitutionalStatus(validation.Status),
		"ham_durum":                  validation.Status,
		"skor":                       validation.Score,
		"sinyal_kapisina_uygun":      localize.Bool(validation.GateEligible),
		"ozet":                       validation.Summary,
		"indikator_formul_durumu":    localizeInstitutionalStatus(validation.IndicatorFormulaStatus),
		"indikator_kontrol_sayisi":   validation.IndicatorChecked,
		"hesaplanan_indikator":       validation.IndicatorComputed,
		"proxy_indikator":            validation.IndicatorProxyOnly,
		"dis_veri_gereken_indikator": validation.IndicatorExternalRequired,
		"indikator_hata":             validation.IndicatorErrors,
		"indikator_uyari":            validation.IndicatorWarnings,
		"formasyon_durumu":           localizeInstitutionalStatus(validation.PatternStatus),
		"formasyon_kontrol_sayisi":   validation.PatternChecked,
		"aktif_formasyon":            validation.PatternConfirmed,
		"aday_formasyon":             validation.PatternCandidates,
		"grafikte_islenen_formasyon": validation.PatternDrawn,
		"grafikte_islenemeyen":       validation.PatternNotDrawn,
		"geometri_cizimi":            validation.GeometryPatternDrawings,
		"grafik_isleme_durumu":       validation.ChartOverlayStatus,
		"engeller":                   reportLabels(validation.Blockers),
		"kanit":                      validation.Evidence,
	}
}

func turkishTechnicalSignalGate(gate professional.TechnicalSignalGate) map[string]any {
	return map[string]any{
		"durum":                    localizeInstitutionalStatus(gate.Status),
		"ham_durum":                gate.Status,
		"islem_sinyali_aktif":      localize.Bool(gate.Actionable),
		"yon":                      localize.Direction(gate.Direction),
		"aciklama":                 gate.Label,
		"skor":                     gate.Score,
		"zaman_dilimi":             localize.Timeframe(gate.Timeframe),
		"giris_alt":                gate.EntryMin,
		"giris_ust":                gate.EntryMax,
		"zarar_kes":                gate.StopLoss,
		"hedef_1":                  gate.Target1,
		"hedef_2":                  gate.Target2,
		"risk_getiri":              gate.RiskRewardRatio,
		"hacim_teyidi":             localize.Bool(gate.VolumeConfirmed),
		"hacim_teyidi_aciklamasi":  gate.VolumeConfirmation,
		"fiyat_yapisi":             gate.PriceStructure,
		"backtest_ozeti":           gate.BacktestSummary,
		"teyit_eden_indikatorler":  gate.ConfirmingIndicators,
		"teyit_eden_formasyonlar":  gate.ConfirmingPatterns,
		"celisen_sinyaller":        gate.ConflictingSignals,
		"gecen_kontroller":         gate.Passes,
		"gecmeyen_kontroller":      gate.Blockers,
		"veriye_dayali_gerekceler": gate.Evidence,
	}
}

func turkishProfessionalIndicators(indicators []professional.TechnicalIndicator) []map[string]any {
	result := make([]map[string]any, 0, len(indicators))
	for _, indicator := range indicators {
		result = append(result, map[string]any{
			"ad":       indicator.Name,
			"kategori": indicator.Category,
			"grup":     indicator.Group,
			"sinyal":   localize.Signal(indicator.Signal),
			"deger":    indicator.Value,
			"guven":    indicator.Confidence,
			"kaynak":   signalSource(indicator.Signal, indicator.Source),
			"kanit":    localize.EvidenceList(indicator.Evidence),
		})
	}
	return result
}

func turkishProfessionalPatterns(patterns []professional.TechnicalPattern) []map[string]any {
	result := make([]map[string]any, 0, len(patterns))
	for _, pattern := range patterns {
		result = append(result, map[string]any{
			"ad":                 localize.PatternName(pattern.Name),
			"kategori":           pattern.Category,
			"yon":                localize.Direction(pattern.Direction),
			"guven":              pattern.Confidence,
			"baslangic_indeksi":  pattern.StartIndex,
			"bitis_indeksi":      pattern.EndIndex,
			"hacimle_dogrulandi": localize.Bool(pattern.VolumeConfirmed),
			"kaynak":             patternSource(pattern.Source),
			"kanit":              localize.EvidenceList(pattern.Evidence),
		})
	}
	return result
}

func turkishProfessional(result analysis.SymbolAnalysis) map[string]any {
	pro := result.Professional
	scenarios := make([]map[string]any, 0, len(pro.Scenarios))
	for _, scenario := range pro.Scenarios {
		scenarios = append(scenarios, map[string]any{
			"ad":           scenario.Name,
			"olasilik":     scenario.Probability,
			"hedef_fiyat":  scenario.PriceTarget,
			"getiri_yuzde": scenario.ReturnPct,
			"gerekceler":   scenario.Drivers,
		})
	}
	if isCryptoResult(result) {
		return map[string]any{
			"rapor_guveni":  turkishReportConfidence(result),
			"veri_kalitesi": pro.DataQuality,
			"kapsam": map[string]any{
				"skor":     pro.Coverage.Score,
				"mevcut":   pro.Coverage.Available,
				"eksik":    pro.Coverage.Missing,
				"uyarilar": pro.Coverage.Warnings,
			},
			"varlik": map[string]any{
				"sembol":     pro.Company.Symbol,
				"ad":         pro.Company.Name,
				"sinif":      pro.Company.Sector,
				"kategori":   pro.Company.Industry,
				"peer_grubu": pro.Company.PeerGroup,
			},
			"piyasa_benchmark": map[string]any{
				"benchmark":      pro.Market.BenchmarkSymbol,
				"benchmark_var":  pro.Market.BenchmarkAvailable,
				"relatif_guc_20": pro.Market.RelativeStrength20,
				"relatif_guc_60": pro.Market.RelativeStrength60,
				"beta_60":        pro.Market.Beta60,
				"korelasyon_60":  pro.Market.Correlation60,
				"alfa_60":        pro.Market.Alpha60,
			},
			"kripto_analiz_kapsami": map[string]any{
				"yaklasim":             pro.Valuation.SectorModel,
				"kullanilan_metrikler": pro.Valuation.AllowedRatios,
				"kapsam_disi":          pro.Valuation.SuppressedRatios,
				"teknik_aralik":        pro.Valuation.FairValue,
				"bayraklar":            pro.Valuation.Flags,
			},
			"kripto_kaynak_kontrolu": map[string]any{
				"kaynak_durumu":     pro.Disclosure.RecentDisclosureStatus,
				"risk_bayraklari":   pro.Disclosure.RiskFlags,
				"gereken_kaynaklar": pro.Disclosure.RequiredSources,
			},
			"kripto_context": pro.CryptoContext,
			"veri_yonetisimi": map[string]any{
				"as_of":            pro.DataGovernance.AsOf,
				"veri_modu":        pro.DataGovernance.DataMode,
				"backtest_guvenli": pro.DataGovernance.BacktestSafe,
				"production_hazir": pro.DataGovernance.ProductionReady,
				"erisim_durumu":    pro.DataGovernance.AvailabilityStatus,
				"kaynak":           pro.DataGovernance.Source,
				"para_birimi":      pro.DataGovernance.Currency,
				"uyarilar":         pro.DataGovernance.Warnings,
			},
			"senaryolar": scenarios,
		}
	}
	if isCommodityResult(result) {
		return map[string]any{
			"rapor_guveni":  turkishReportConfidence(result),
			"veri_kalitesi": pro.DataQuality,
			"kapsam": map[string]any{
				"skor":       pro.Coverage.Score,
				"mevcut":     pro.Coverage.Available,
				"eksik":      pro.Coverage.Missing,
				"eksik_sade": missingDataLabels(result),
				"uyarilar":   pro.Coverage.Warnings,
			},
			"varlik": map[string]any{
				"sembol":     pro.Company.Symbol,
				"ad":         pro.Company.Name,
				"sinif":      pro.Company.Sector,
				"kategori":   pro.Company.Industry,
				"peer_grubu": pro.Company.PeerGroup,
			},
			"piyasa_benchmark": map[string]any{
				"benchmark":      pro.Market.BenchmarkSymbol,
				"benchmark_var":  pro.Market.BenchmarkAvailable,
				"relatif_guc_20": pro.Market.RelativeStrength20,
				"relatif_guc_60": pro.Market.RelativeStrength60,
				"beta_60":        pro.Market.Beta60,
				"korelasyon_60":  pro.Market.Correlation60,
				"alfa_60":        pro.Market.Alpha60,
			},
			"emtia_analiz_kapsami": map[string]any{
				"yaklasim":             pro.Valuation.SectorModel,
				"kullanilan_metrikler": pro.Valuation.AllowedRatios,
				"kapsam_disi":          pro.Valuation.SuppressedRatios,
				"teknik_aralik":        pro.Valuation.FairValue,
				"bayraklar":            pro.Valuation.Flags,
			},
			"emtia_kaynak_kontrolu": map[string]any{
				"kaynak_durumu":     pro.Disclosure.RecentDisclosureStatus,
				"risk_bayraklari":   pro.Disclosure.RiskFlags,
				"gereken_kaynaklar": pro.Disclosure.RequiredSources,
			},
			"veri_yonetisimi": map[string]any{
				"as_of":            pro.DataGovernance.AsOf,
				"veri_modu":        pro.DataGovernance.DataMode,
				"backtest_guvenli": pro.DataGovernance.BacktestSafe,
				"production_hazir": pro.DataGovernance.ProductionReady,
				"erisim_durumu":    pro.DataGovernance.AvailabilityStatus,
				"kaynak":           pro.DataGovernance.Source,
				"para_birimi":      pro.DataGovernance.Currency,
				"uyarilar":         pro.DataGovernance.Warnings,
			},
			"senaryolar": scenarios,
		}
	}
	return map[string]any{
		"rapor_guveni":  turkishReportConfidence(result),
		"veri_kalitesi": pro.DataQuality,
		"kapsam": map[string]any{
			"skor":     pro.Coverage.Score,
			"mevcut":   pro.Coverage.Available,
			"eksik":    pro.Coverage.Missing,
			"uyarilar": pro.Coverage.Warnings,
		},
		"kurumsal_aksiyonlar": map[string]any{
			"durum":                        pro.CorporateActions.Status,
			"aksiyon_sayisi":               len(pro.CorporateActions.Actions),
			"dogrulanmis_aksiyon":          pro.CorporateActions.VerifiedActions,
			"aday_aksiyon":                 pro.CorporateActions.CandidateActions,
			"inceleme_gereken_aksiyon":     pro.CorporateActions.ReviewRequiredActions,
			"duzeltmeye_hazir_aksiyon":     pro.CorporateActions.AdjustmentReadyActions,
			"effective_date_eksik_aksiyon": pro.CorporateActions.MissingEffectiveDateActions,
			"oran_tutar_eksik_aksiyon":     pro.CorporateActions.MissingAdjustmentActions,
			"kaynak_dosyalari":             pro.CorporateActions.SourceFiles,
			"uyarilar":                     pro.CorporateActions.Warnings,
		},
		"sirket": map[string]any{
			"sembol":                 pro.Company.Symbol,
			"ad":                     pro.Company.Name,
			"sektor":                 pro.Company.Sector,
			"odenmis_sermaye":        pro.Company.PaidCapital,
			"kayitli_sermaye_tavani": pro.Company.RegisteredCapitalCeiling,
		},
		"piyasa_benchmark": map[string]any{
			"benchmark":      pro.Market.BenchmarkSymbol,
			"benchmark_var":  pro.Market.BenchmarkAvailable,
			"relatif_guc_20": pro.Market.RelativeStrength20,
			"relatif_guc_60": pro.Market.RelativeStrength60,
			"beta_60":        pro.Market.Beta60,
			"korelasyon_60":  pro.Market.Correlation60,
			"alfa_60":        pro.Market.Alpha60,
		},
		"degerleme": map[string]any{
			"son_donem":            fmt.Sprintf("%d %s", pro.Valuation.LatestYear, pro.Valuation.LatestQuarter),
			"sektor_modeli":        pro.Valuation.SectorModel,
			"piyasa_degeri":        pro.Valuation.MarketCap,
			"firma_degeri":         pro.Valuation.EnterpriseValue,
			"net_borc":             pro.Valuation.NetDebt,
			"satis_ttm":            pro.Valuation.SalesTTM,
			"ebitda_ttm":           pro.Valuation.EBITDATTM,
			"net_kar_ttm":          pro.Valuation.NetIncomeTTM,
			"ozsermaye":            pro.Valuation.Equity,
			"carpanlar":            pro.Valuation.Ratios,
			"kullanilan_carpanlar": pro.Valuation.AllowedRatios,
			"bastirilan_carpanlar": pro.Valuation.SuppressedRatios,
			"sektor_metrikleri":    pro.Valuation.SectorMetrics,
			"dcf":                  pro.Valuation.DCF,
			"makul_deger_araligi":  pro.Valuation.FairValue,
			"bayraklar":            pro.Valuation.Flags,
		},
		"deger_yatirimi":             pro.ValueInvesting,
		"yatirim_arastirma_denetimi": pro.InvestmentResearch,
		"vap_fiili_dolasim": map[string]any{
			"hesaplandi":                pro.VAPFreeFloat.Computed,
			"kaynak_dosya":              pro.VAPFreeFloat.SourcePath,
			"son_tarih":                 pro.VAPFreeFloat.LatestDate,
			"gozlem_sayisi":             pro.VAPFreeFloat.Observations,
			"fiili_dolasimdaki_pay":     pro.VAPFreeFloat.FreeFloatShares,
			"ihracci_sermaye":           pro.VAPFreeFloat.IssuerCapital,
			"fiili_dolasim_orani_yuzde": pro.VAPFreeFloat.FreeFloatRatioPct,
			"oran_degisim_1g_puan":      pro.VAPFreeFloat.RatioChange1DPP,
			"oran_degisim_20g_puan":     pro.VAPFreeFloat.RatioChange20DPP,
			"oran_degisim_60g_puan":     pro.VAPFreeFloat.RatioChange60DPP,
			"pay_degisim_20g_yuzde":     pro.VAPFreeFloat.SharesChange20Pct,
			"likidite_riski":            pro.VAPFreeFloat.LiquidityRisk,
			"arz_sinyali":               pro.VAPFreeFloat.SupplySignal,
			"noktasal_zaman_guvenli":    pro.VAPFreeFloat.PointInTimeSafe,
			"ozet":                      pro.VAPFreeFloat.Summary,
			"uyarilar":                  pro.VAPFreeFloat.Warnings,
		},
		"vap_bist_endeks_portfoyu": map[string]any{
			"hesaplandi":                pro.VAPIndexPortfolio.Computed,
			"kaynak_dosya":              pro.VAPIndexPortfolio.SourcePath,
			"secilen_endeks":            pro.VAPIndexPortfolio.SelectedIndex,
			"son_donem":                 pro.VAPIndexPortfolio.LatestMonth,
			"portfoy_degeri_milyon_tl":  pro.VAPIndexPortfolio.PortfolioValueMTL,
			"degisim_1ay_yuzde":         pro.VAPIndexPortfolio.Change1MPct,
			"degisim_3ay_yuzde":         pro.VAPIndexPortfolio.Change3MPct,
			"degisim_12ay_yuzde":        pro.VAPIndexPortfolio.Change12MPct,
			"bist100_degisim_1ay_yuzde": pro.VAPIndexPortfolio.BIST100Change1M,
			"goreli_momentum_1ay_yuzde": pro.VAPIndexPortfolio.RelativeMomentum,
			"sinyal":                    pro.VAPIndexPortfolio.Signal,
			"noktasal_zaman_guvenli":    pro.VAPIndexPortfolio.PointInTimeSafe,
			"ozet":                      pro.VAPIndexPortfolio.Summary,
			"uyarilar":                  pro.VAPIndexPortfolio.Warnings,
		},
		"kap_pdf_ingest":       pro.KAPPDFIngest,
		"kap_varlik_envanteri": pro.KAPAssetInventory,
		"peer_karsilastirma": map[string]any{
			"sektor":            pro.Peers.Sector,
			"peer_sayisi":       pro.Peers.PeerCount,
			"medyanlar":         pro.Peers.Medians,
			"yuzdelikler":       pro.Peers.Percentiles,
			"degerleme_sinyali": pro.Peers.ValuationSignal,
			"peerler":           pro.Peers.Peers,
		},
		"kap_haber_kontrolu": map[string]any{
			"kap_sirket_karti_var": pro.Disclosure.KAPCompanyCardAvailable,
			"guncel_duyuru_durumu": pro.Disclosure.RecentDisclosureStatus,
			"guncel_duyuru_sayisi": pro.Disclosure.RecentDisclosureCount,
			"yerel_yorum_sayisi":   pro.Disclosure.LocalCommentCount,
			"risk_bayraklari":      pro.Disclosure.RiskFlags,
			"gereken_kaynaklar":    pro.Disclosure.RequiredSources,
		},
		"veri_yonetisimi": map[string]any{
			"as_of":                               pro.DataGovernance.AsOf,
			"veri_modu":                           pro.DataGovernance.DataMode,
			"backtest_guvenli":                    pro.DataGovernance.BacktestSafe,
			"production_hazir":                    pro.DataGovernance.ProductionReady,
			"erisim_durumu":                       pro.DataGovernance.AvailabilityStatus,
			"kaynak":                              pro.DataGovernance.Source,
			"para_birimi":                         pro.DataGovernance.Currency,
			"son_donem":                           pro.DataGovernance.LatestPeriod,
			"son_yayin_tarihi":                    pro.DataGovernance.LatestPublishDate,
			"son_kullanilabilir_tarih":            pro.DataGovernance.LatestAvailableAt,
			"yayin_tarihi_kapsami":                pro.DataGovernance.PublishDateCoverage,
			"kullanilabilir_tarih_kapsami":        pro.DataGovernance.AvailableAtCoverage,
			"dogrulanmis_publish_date_sayisi":     pro.DataGovernance.VerifiedPublishDateCount,
			"konservatif_available_at_sayisi":     pro.DataGovernance.ConservativeAvailableAtCount,
			"guvensiz_availability_sayisi":        pro.DataGovernance.UnsafeAvailabilityCount,
			"production_uygun_donem_sayisi":       pro.DataGovernance.ProductionEligiblePeriodCount,
			"production_karantina_donem_sayisi":   pro.DataGovernance.ProductionQuarantinedPeriodCount,
			"finansal_tutarlilik":                 pro.DataGovernance.FinanciallyConsistent,
			"reconciliation_kontrol_sayisi":       pro.DataGovernance.ReconciliationCheckCount,
			"reconciliation_hata_sayisi":          pro.DataGovernance.ReconciliationFailureCount,
			"lineage_event_sayisi":                pro.DataGovernance.LineageEvents,
			"statement_version_store_var":         pro.DataGovernance.StatementVersionStoreAvailable,
			"statement_version_sayisi":            pro.DataGovernance.StatementVersionCount,
			"restatement_sayisi":                  pro.DataGovernance.RestatementCount,
			"universe_kaynagi_var":                pro.DataGovernance.UniverseSourceAvailable,
			"survivorship_bias_riski":             pro.DataGovernance.SurvivorshipBiasRisk,
			"yayin_tarihi_eksik_donemler":         pro.DataGovernance.MissingPublishPeriods,
			"kullanilabilir_tarih_eksik_donemler": pro.DataGovernance.MissingAvailableAtPeriods,
			"backtest_guvensiz_donemler":          pro.DataGovernance.UnsafeBacktestPeriods,
			"uyarilar":                            pro.DataGovernance.Warnings,
		},
		"fundamental_backtest": map[string]any{
			"execution_model":               pro.FundamentalBacktest.ExecutionModel,
			"verified_publish_date_zorunlu": pro.FundamentalBacktest.RequireVerifiedPublishDate,
			"backtest_guvenli":              pro.FundamentalBacktest.BacktestSafe,
			"event_sayisi":                  pro.FundamentalBacktest.Events,
			"islem_yapilabilir_event":       pro.FundamentalBacktest.TradableEvents,
			"dogrulanmis_publish_event":     pro.FundamentalBacktest.VerifiedPublishDateEvents,
			"konservatif_event":             pro.FundamentalBacktest.ConservativeAvailableAtEvents,
			"guvensiz_event":                pro.FundamentalBacktest.UnsafeAvailabilityEvents,
			"policy_rejected_event":         pro.FundamentalBacktest.PolicyRejectedEvents,
			"publish_date_eksik_event":      pro.FundamentalBacktest.MissingPublishDateEvents,
			"available_at_eksik_event":      pro.FundamentalBacktest.MissingAvailableAtEvents,
			"execution_bar_eksik_event":     pro.FundamentalBacktest.NoExecutionBarEvents,
			"lookahead_ihlali":              pro.FundamentalBacktest.LookaheadViolations,
			"uyarilar":                      pro.FundamentalBacktest.Warnings,
		},
		"senaryolar": scenarios,
	}
}

func turkishReportConfidence(result analysis.SymbolAnalysis) map[string]any {
	confidence := reportConfidenceFor(result)
	items := make([]map[string]any, 0, len(confidence.Items))
	for _, item := range confidence.Items {
		scorePct := 0.0
		if item.Max > 0 {
			scorePct = math.Round(100 * item.Score / item.Max)
		}
		items = append(items, map[string]any{
			"alan":       item.Label,
			"skor":       math.Round(item.Score),
			"maksimum":   item.Max,
			"skor_yuzde": scorePct,
			"durum":      item.Status,
			"detay":      item.Detail,
		})
	}
	scores := map[string]any{
		"coverage_score":                    math.Round(result.Professional.Coverage.Score),
		"parse_quality_score":               math.Round(parseQualityScore(result)),
		"financial_reconciliation_score":    math.Round(financialReconciliationScore(result)),
		"valuation_confidence_score":        math.Round(valuationConfidenceScore(result)),
		"technical_signal_confidence_score": math.Round(technicalSignalConfidenceScore(result)),
		"backtest_confidence_score":         math.Round(backtestConfidenceScore(result)),
		"production_readiness_score":        math.Round(productionReadinessScore(result)),
	}
	if isBankReport(result) {
		scores["bank_metric_completeness_score"] = math.Round(professional.BankRegulatoryMetricCompletenessScore(result.Professional.SectorFinancials))
		scores["bank_missing_metrics"] = professional.MissingBankRegulatoryMetricNames(result.Professional.SectorFinancials)
	} else {
		scores["bank_metric_completeness_score"] = "not_applicable"
	}
	return map[string]any{
		"skor":            confidence.Score,
		"bilesenler":      items,
		"kalite_skorlari": scores,
		"not":             "Rapor güveni fiyat hedefi değildir; veri kapsamı, kanıt kalitesi, mutabakat ve karar kapısı tutarlılığı skorudur.",
	}
}

func parseQualityScore(result analysis.SymbolAnalysis) float64 {
	kap := result.Professional.KAPPDFIngest
	if !kap.Computed || kap.TotalDocuments == 0 {
		return 0
	}
	usable := safeReportRatio(float64(kap.AnalysisUsableCount), float64(kap.TotalDocuments))
	decision := safeReportRatio(float64(kap.DecisionRelevantUsableCount), float64(kap.DecisionRelevantDocuments))
	if kap.DecisionRelevantDocuments == 0 {
		decision = usable
	}
	reviewPenalty := safeReportRatio(float64(kap.ReviewRequiredCount+kap.RejectedCount+kap.LowQualityCount), float64(kap.TotalDocuments)) * 25
	score := kap.AverageQuality*45 + usable*25 + decision*30 - reviewPenalty
	return clampReport(score, 0, 100)
}

func financialReconciliationScore(result analysis.SymbolAnalysis) float64 {
	gov := result.Professional.DataGovernance
	score := 0.0
	if gov.FinanciallyConsistent {
		score += 35
	}
	if gov.ReconciliationCheckCount > 0 {
		score += 20
	}
	if gov.ReconciliationFailureCount == 0 && gov.ReconciliationCheckCount > 0 {
		score += 20
	} else if gov.ReconciliationFailureCount > 0 {
		score -= math.Min(20, float64(gov.ReconciliationFailureCount)*5)
	}
	if gov.LatestPublishDate != nil && gov.LatestAvailableAt != nil {
		score += 15
	}
	if gov.BacktestSafe {
		score += 10
	}
	return clampReport(score, 0, 100)
}

func valuationConfidenceScore(result analysis.SymbolAnalysis) float64 {
	v := result.Professional.ValueInvesting
	score := v.Confidence
	if !v.Computed {
		score = math.Min(score, result.Professional.Valuation.FairValue.Confidence*100)
	}
	if isBankReport(result) && bankCoreMetricsMissingForReport(result) {
		score = math.Min(score, 35)
	}
	return clampReport(score, 0, 100)
}

func technicalSignalConfidenceScore(result analysis.SymbolAnalysis) float64 {
	if daily, ok := result.Timeframes["1D"]; ok {
		gate := daily.Professional.Technical.SignalGate
		score := gate.Score
		if !gate.Actionable {
			score = math.Min(score, 65)
		}
		return clampReport(score, 0, 100)
	}
	item := technicalConfidence(result)
	if item.Max <= 0 {
		return 0
	}
	return clampReport(100*item.Score/item.Max, 0, 100)
}

func backtestConfidenceScore(result analysis.SymbolAnalysis) float64 {
	daily, ok := result.Timeframes["1D"]
	if !ok {
		return 0
	}
	bt := daily.Professional.Backtest
	score := 0.0
	if bt.BacktestSafe && bt.LookaheadViolations == 0 {
		score += 25
	}
	score += clampReport(float64(bt.Trades)/30, 0, 1) * 25
	score += clampReport(float64(bt.OutOfSampleTrades)/10, 0, 1) * 20
	if bt.Expectancy > 0 {
		score += 15
	}
	if bt.OutOfSampleReturn > 0 {
		score += 15
	}
	return clampReport(score, 0, 100)
}

func productionReadinessScore(result analysis.SymbolAnalysis) float64 {
	gov := result.Professional.DataGovernance
	if gov.ProductionReady {
		return 100
	}
	score := 0.0
	if gov.BacktestSafe {
		score += 25
	}
	if gov.FinanciallyConsistent {
		score += 20
	}
	score += clampReport(gov.PublishDateCoverage, 0, 1) * 20
	score += clampReport(gov.AvailableAtCoverage, 0, 1) * 20
	if gov.UnsafeAvailabilityCount == 0 {
		score += 15
	}
	return clampReport(score, 0, 100)
}

func indicatorSignalNames(indicators []ohlcv.IndicatorResult, limit int) []string {
	names := []string{}
	for _, indicator := range indicators {
		if !indicator.Computed || indicator.Confidence < 0.5 || indicator.Signal == "neutral" || indicator.Signal == "info" {
			continue
		}
		names = append(names, fmt.Sprintf("%s (%s %.2f)", indicator.Name, localize.Signal(indicator.Signal), indicator.Confidence))
		if limit > 0 && len(names) >= limit {
			break
		}
	}
	return names
}

func professionalIndicatorNames(indicators []professional.TechnicalIndicator, limit int) []string {
	names := []string{}
	for _, indicator := range indicators {
		names = append(names, fmt.Sprintf("%s (%s %.2f)", indicator.Name, localize.Signal(indicator.Signal), indicator.Confidence))
		if limit > 0 && len(names) >= limit {
			break
		}
	}
	return names
}

func professionalPatternNames(patterns []professional.TechnicalPattern, limit int) []string {
	names := []string{}
	for _, pattern := range patterns {
		names = append(names, fmt.Sprintf("%s (%s %.2f)", localize.PatternName(pattern.Name), localize.Direction(pattern.Direction), pattern.Confidence))
		if limit > 0 && len(names) >= limit {
			break
		}
	}
	return names
}

func turkishIndicators(ind ohlcv.IndicatorSnapshot) map[string]any {
	return map[string]any{
		"hareketli_ortalama_20":       ind.SMA20,
		"hareketli_ortalama_50":       ind.SMA50,
		"hareketli_ortalama_100":      ind.SMA100,
		"hareketli_ortalama_200":      ind.SMA200,
		"ustel_hareketli_ortalama_20": ind.EMA20,
		"ustel_hareketli_ortalama_50": ind.EMA50,
		"rsi_14":                      ind.RSI14,
		"atr_14":                      ind.ATR14,
		"macd":                        ind.MACD,
		"macd_sinyal":                 ind.MACDSignal,
		"macd_histogram":              ind.MACDHistogram,
		"bollinger_ust":               ind.BollingerUpper,
		"bollinger_orta":              ind.BollingerMiddle,
		"bollinger_alt":               ind.BollingerLower,
		"adx_14":                      ind.ADX14,
		"hacim_hareketli_ortalama_20": ind.VolumeSMA20,
		"chaikin_para_akisi_20":       ind.ChaikinMoneyFlow20,
		"stokastik_k":                 ind.StochasticK,
		"stokastik_d":                 ind.StochasticD,
		"ichimoku_tenkan":             ind.IchimokuTenkan,
		"ichimoku_kijun":              ind.IchimokuKijun,
		"ichimoku_senkou_a":           ind.IchimokuSenkouA,
		"ichimoku_senkou_b":           ind.IchimokuSenkouB,
	}
}

func turkishIndicatorSignals(indicators []ohlcv.IndicatorResult) []map[string]any {
	result := make([]map[string]any, 0, len(indicators))
	for _, indicator := range indicators {
		result = append(result, map[string]any{
			"ad":             indicator.Name,
			"kategori":       indicator.Category,
			"grup":           indicator.Group,
			"sinyal":         localize.Signal(indicator.Signal),
			"deger":          indicator.Value,
			"guven":          indicator.Confidence,
			"hesaplandi":     localize.Bool(indicator.Computed),
			"hesap_durumu":   indicatorStatus(indicator),
			"durum_aciklama": indicatorStatusExplanation(indicator),
			"kaynak":         indicatorSource(indicator),
			"kanit":          localize.EvidenceList(indicator.Evidence),
		})
	}
	return result
}

func indicatorStatus(indicator ohlcv.IndicatorResult) string {
	switch {
	case indicator.Signal == "requires_external_data":
		return "hesaplanmadi_dis_veri_gerekli"
	case indicator.Signal == "insufficient_data":
		return "hesaplanmadi_veri_yetersiz"
	case indicator.Signal == "proxy_only":
		return "hesaplanmadi_yaklasik_sinyal_degil"
	case indicator.Computed:
		return "hesaplandi"
	default:
		return "hesaplanmadi"
	}
}

func indicatorSource(indicator ohlcv.IndicatorResult) string {
	return signalSource(indicator.Signal, indicator.Source)
}

func signalSource(signal string, source string) string {
	switch signal {
	case "requires_external_data":
		return "dis_veri_gerekli"
	case "insufficient_data":
		return "veri_yetersiz"
	case "proxy_only":
		return "yaklasik_hesap_sinyal_degil"
	}
	if source == "" {
		return "yerel_ohlcv"
	}
	return source
}

func indicatorStatusExplanation(indicator ohlcv.IndicatorResult) string {
	switch {
	case indicator.Signal == "requires_external_data":
		return "Bu indikatör tek varlık OHLCV verisiyle güvenilir hesaplanamaz; order book, opsiyon, piyasa geneli, on-chain veya profil verisi gibi ek veri ister."
	case indicator.Signal == "insufficient_data":
		return "Bu indikatör için gerekli minimum tamamlanmış mum sayısı yok; değer ve sinyal hesaplanmadı."
	case indicator.Signal == "proxy_only":
		return "Bu indikatör için gerçek veri/formül doğrulaması yok; yaklaşık değer üretildiği için al-sat sinyali olarak kullanılmaz."
	case indicator.Computed:
		return "Bu indikatör mevcut OHLCV verisiyle yerel olarak hesaplandı."
	default:
		return "Bu indikatör için güvenilir hesap üretilemedi."
	}
}

func patternSource(source string) string {
	switch source {
	case "requires_external_data", "market_profile", "point_figure":
		return "dis_veri_gerekli"
	case "":
		return "yerel_ohlcv"
	default:
		return source
	}
}

func turkishPatterns(patterns []ohlcv.PatternResult) []map[string]any {
	result := make([]map[string]any, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.Confidence < 0.5 {
			continue
		}
		result = append(result, map[string]any{
			"ad":                             localize.PatternName(pattern.Name),
			"yon":                            localize.Direction(pattern.Direction),
			"guven_skoru":                    pattern.Confidence,
			"hacim_teyidi":                   patternVolumeStatus(pattern),
			"teyit_eden_indikatorler":        pattern.ConfirmingIndicators,
			"gecersiz_kilan_karsi_sinyaller": pattern.InvalidatingSignals,
			"islem_degeri":                   patternTradeValueStatus(pattern),
			"dogrulama_durumu":               pattern.ValidationStatus,
			"dogrulama_yontemi":              pattern.ValidationMethod,
			"p_degeri":                       pattern.ValidationPValue,
			"guven_araligi_alt":              pattern.ValidationCILow,
			"guven_araligi_ust":              pattern.ValidationCIHigh,
			"baslangic_indeksi":              pattern.StartIndex,
			"bitis_indeksi":                  pattern.EndIndex,
			"kanit":                          localize.EvidenceList(pattern.Evidence),
			"hacimle_dogrulandi":             localize.Bool(pattern.VolumeConfirmed),
		})
	}
	return result
}

func turkishPatternScans(patterns []ohlcv.PatternScanResult) []map[string]any {
	result := make([]map[string]any, 0, len(patterns))
	for _, pattern := range patterns {
		if !pattern.Matched {
			continue
		}
		result = append(result, map[string]any{
			"ad":                             localize.PatternName(pattern.Name),
			"kategori":                       pattern.Category,
			"grup":                           pattern.Group,
			"yon":                            localize.Direction(pattern.Direction),
			"bulundu":                        localize.Bool(pattern.Matched),
			"aktif_islem_sinyali":            localize.Bool(pattern.Actionable),
			"guven":                          pattern.Confidence,
			"ret_nedenleri":                  reportLabels(pattern.RejectionReasons),
			"hacim_teyidi":                   pattern.VolumeConfirmation,
			"teyit_eden_indikatorler":        pattern.ConfirmingIndicators,
			"gecersiz_kilan_karsi_sinyaller": pattern.InvalidatingSignals,
			"islem_degeri":                   pattern.TradeValue,
			"dogrulama_durumu":               pattern.ValidationStatus,
			"dogrulama_yontemi":              pattern.ValidationMethod,
			"p_degeri":                       pattern.ValidationPValue,
			"guven_araligi_alt":              pattern.ValidationCILow,
			"guven_araligi_ust":              pattern.ValidationCIHigh,
			"kaynak":                         patternSource(pattern.Source),
			"kanit":                          localize.EvidenceList(pattern.Evidence),
		})
	}
	return result
}

func patternVolumeStatus(pattern ohlcv.PatternResult) string {
	if pattern.VolumeConfirmed || pattern.VolumeConfirmation == "confirmed" {
		return "var"
	}
	return "yok_sinyal_zayif"
}

func patternTradeValueStatus(pattern ohlcv.PatternResult) string {
	if strings.TrimSpace(pattern.TradeValue) != "" {
		return pattern.TradeValue
	}
	if !pattern.VolumeConfirmed {
		return "weak_no_volume_confirmation"
	}
	if pattern.Actionable {
		return "medium"
	}
	return "rejected_candidate"
}

func turkishLevels(levels []ohlcv.SupportResistanceLevel) []map[string]any {
	result := make([]map[string]any, 0, len(levels))
	for _, level := range levels {
		result = append(result, turkishLevel(level))
	}
	return result
}

func turkishLevelPtr(level *ohlcv.SupportResistanceLevel) any {
	if level == nil {
		return nil
	}
	return turkishLevel(*level)
}

func turkishLevel(level ohlcv.SupportResistanceLevel) map[string]any {
	return map[string]any{
		"tur":              localize.LevelType(level.Type),
		"fiyat":            level.Price,
		"guc":              level.Strength,
		"temas_sayisi":     level.TouchCount,
		"son_temas_tarihi": level.LastTouchedAt.Format("2006-01-02"),
	}
}

func turkishTradePlan(plan ohlcv.TradePlan) map[string]any {
	reasoning := make([]string, 0, len(plan.Reasoning))
	for _, reason := range plan.Reasoning {
		reasoning = append(reasoning, localize.Reason(reason))
	}
	return map[string]any{
		"yon":         localize.Direction(plan.Direction),
		"giris_alt":   plan.EntryMin,
		"giris_ust":   plan.EntryMax,
		"kar_al_1":    plan.TakeProfit1,
		"kar_al_2":    plan.TakeProfit2,
		"zarar_kes":   plan.StopLoss,
		"risk_getiri": plan.RiskRewardRatio,
		"kalite":      localize.Quality(plan.Quality),
		"guven_skoru": tradePlanConfidencePct(plan.ConfidenceScore),
		"reddedildi":  localize.Bool(plan.Rejected),
		"ret_nedeni":  localize.Reason(plan.RejectReason),
		"gerekceler":  reasoning,
	}
}

func tradePlanConfidencePct(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value <= 1 {
		return value * 100
	}
	return value
}
