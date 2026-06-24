package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appconfig "hissebot/internal/config"
	corestorage "hissebot/internal/storage"
	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/datasource"
	"hissebot/internal/ta/forecastpolicy"
	"hissebot/internal/ta/ohlcv"
)

type forecastAuditReport struct {
	SchemaVersion        int                            `json:"schema_version"`
	Name                 string                         `json:"name"`
	Symbol               string                         `json:"symbol"`
	AsOfDate             string                         `json:"as_of_date"`
	ForecastSession      string                         `json:"forecast_session"`
	GeneratedAt          string                         `json:"generated_at"`
	Status               string                         `json:"status"`
	ReportDecisionStatus string                         `json:"report_decision_status"`
	ModelDecisionStatus  string                         `json:"model_decision_status"`
	LastObservedRecord   datasource.DailyBulletinRecord `json:"last_observed_record"`
	ActualRecord         datasource.DailyBulletinRecord `json:"actual_record,omitempty"`
	ActualAvailable      bool                           `json:"actual_available"`
	OfficialResult       forecastAuditOfficialResult    `json:"official_result"`
	Forecast             analysis.NextSessionForecast   `json:"forecast"`
	Validation           forecastAuditValidation        `json:"validation"`
	ModelValidation      forecastAuditModelValidation   `json:"model_validation"`
	DataScope            forecastAuditDataScope         `json:"data_scope"`
	FilesUsed            []forecastAuditFile            `json:"files_used"`
	GeneratedFiles       []forecastAuditFile            `json:"generated_files"`
	Interpretation       []string                       `json:"interpretation"`
}

type forecastAuditOfficialResult struct {
	Available        bool    `json:"available"`
	Authoritative    bool    `json:"authoritative"`
	Status           string  `json:"status"`
	CalculationMode  string  `json:"calculation_mode"`
	Open             float64 `json:"open,omitempty"`
	Close            float64 `json:"close,omitempty"`
	OpenDirection    string  `json:"open_direction,omitempty"`
	CloseDirection   string  `json:"close_direction,omitempty"`
	Source           string  `json:"source,omitempty"`
	SourcePath       string  `json:"source_path,omitempty"`
	SourceTradingDay string  `json:"source_trading_day,omitempty"`
}

type forecastAuditValidation struct {
	OpenDirection        string  `json:"open_direction"`
	CloseDirection       string  `json:"close_direction"`
	ActualOpenDirection  string  `json:"actual_open_direction,omitempty"`
	ActualCloseDirection string  `json:"actual_close_direction,omitempty"`
	OpenDirectionHit     *bool   `json:"open_direction_hit,omitempty"`
	CloseDirectionHit    *bool   `json:"close_direction_hit,omitempty"`
	OpenErrorTL          float64 `json:"open_error_tl,omitempty"`
	CloseErrorTL         float64 `json:"close_error_tl,omitempty"`
	OpenAbsErrorPct      float64 `json:"open_abs_error_pct_vs_actual,omitempty"`
	CloseAbsErrorPct     float64 `json:"close_abs_error_pct_vs_actual,omitempty"`
}

type forecastAuditModelValidation struct {
	WindowSessions                  int                          `json:"window_sessions"`
	Baseline                        forecastAuditBacktestMetrics `json:"baseline"`
	Microstructure                  forecastAuditBacktestMetrics `json:"microstructure"`
	MicrostructureTriggeredSamples  int                          `json:"microstructure_triggered_samples"`
	MicrostructureTriggerHitRatePct float64                      `json:"microstructure_trigger_hit_rate_pct,omitempty"`
}

type forecastAuditBacktestMetrics struct {
	Samples             int     `json:"samples"`
	OpenMAEPct          float64 `json:"open_mae_pct"`
	CloseMAEPct         float64 `json:"close_mae_pct"`
	DirectionHitRatePct float64 `json:"direction_hit_rate_pct"`
}

type forecastAuditRangeReport struct {
	SchemaVersion  int                       `json:"schema_version"`
	Name           string                    `json:"name"`
	Symbol         string                    `json:"symbol"`
	FromDate       string                    `json:"from_date"`
	ToDate         string                    `json:"to_date"`
	GeneratedAt    string                    `json:"generated_at"`
	Summary        forecastAuditRangeSummary `json:"summary"`
	Rows           []forecastAuditRangeRow   `json:"rows"`
	FilesUsed      []forecastAuditFile       `json:"files_used"`
	GeneratedFiles []forecastAuditFile       `json:"generated_files"`
}

type forecastAuditRangeSummary struct {
	Rows                     int                              `json:"rows"`
	OfficialObservedRows     int                              `json:"official_observed_rows"`
	OfficialResultHitPct     float64                          `json:"official_result_hit_pct"`
	ForecastContext          string                           `json:"forecast_context,omitempty"`
	FullContextRows          int                              `json:"full_context_rows"`
	TechnicalFallbackRows    int                              `json:"technical_fallback_rows"`
	OfficialOpenExactHitPct  float64                          `json:"official_open_exact_hit_pct"`
	OfficialCloseExactHitPct float64                          `json:"official_close_exact_hit_pct"`
	ReportDecisionStatus     string                           `json:"report_decision_status"`
	PublishableReportStatus  string                           `json:"publishable_report_status"`
	ForecastQualityGrade     string                           `json:"forecast_quality_grade,omitempty"`
	ForecastQualityNotes     []string                         `json:"forecast_quality_notes,omitempty"`
	RegimePerformance        []forecastAuditRegimePerformance `json:"regime_performance,omitempty"`
	ModelPublishedRows       int                              `json:"model_published_rows"`
	ModelSuppressedRows      int                              `json:"model_suppressed_rows"`
	ModelPublishedPct        float64                          `json:"model_published_pct"`
	ModelSuppressedPct       float64                          `json:"model_suppressed_pct"`
	TradeAllowedRows         int                              `json:"trade_allowed_rows"`
	TradeAllowedPct          float64                          `json:"trade_allowed_pct"`
	OpenExactHits            int                              `json:"open_exact_hits"`
	CloseExactHits           int                              `json:"close_exact_hits"`
	OpenExactHitPct          float64                          `json:"open_exact_hit_pct"`
	CloseExactHitPct         float64                          `json:"close_exact_hit_pct"`
	OpenPriceWrongPct        float64                          `json:"open_price_wrong_pct"`
	ClosePriceWrongPct       float64                          `json:"close_price_wrong_pct"`
	OpenWithin050Pct         float64                          `json:"open_within_0_50_pct"`
	CloseWithin050Pct        float64                          `json:"close_within_0_50_pct"`
	OpenWithin100Pct         float64                          `json:"open_within_1_00_pct"`
	CloseWithin100Pct        float64                          `json:"close_within_1_00_pct"`
	OpenWithin200Pct         float64                          `json:"open_within_2_00_pct"`
	CloseWithin200Pct        float64                          `json:"close_within_2_00_pct"`
	OpenDirectionHits        int                              `json:"open_direction_hits"`
	CloseDirectionHits       int                              `json:"close_direction_hits"`
	OpenDirectionHitPct      float64                          `json:"open_direction_hit_pct"`
	CloseDirectionHitPct     float64                          `json:"close_direction_hit_pct"`
	OpenDirectionWrongPct    float64                          `json:"open_direction_wrong_pct"`
	CloseDirectionWrongPct   float64                          `json:"close_direction_wrong_pct"`
	OpenMAEPct               float64                          `json:"open_mae_pct"`
	CloseMAEPct              float64                          `json:"close_mae_pct"`
	OpenAccuracyPct          float64                          `json:"open_accuracy_pct"`
	CloseAccuracyPct         float64                          `json:"close_accuracy_pct"`
	OpenClosenessScorePct    float64                          `json:"open_closeness_score_pct"`
	CloseClosenessScorePct   float64                          `json:"close_closeness_score_pct"`
	OpenErrorPct             float64                          `json:"open_error_pct"`
	CloseErrorPct            float64                          `json:"close_error_pct"`
	FirstActualDate          string                           `json:"first_actual_date,omitempty"`
	LastActualDate           string                           `json:"last_actual_date,omitempty"`
}

type forecastAuditRegimePerformance struct {
	Regime               string  `json:"regime"`
	VolatilityRegime     string  `json:"volatility_regime"`
	ExpectedDirection    string  `json:"expected_direction"`
	Rows                 int     `json:"rows"`
	CloseMAEPct          float64 `json:"close_mae_pct"`
	CloseDirectionHitPct float64 `json:"close_direction_hit_pct"`
	CloseWithin100Pct    float64 `json:"close_within_1_00_pct"`
	TradeAllowedPct      float64 `json:"trade_allowed_pct"`
	ModelPublishedPct    float64 `json:"model_published_pct"`
}

type forecastAuditRangeRow struct {
	AsOfDate                        string   `json:"as_of_date"`
	ActualDate                      string   `json:"actual_date"`
	OfficialResultAvailable         bool     `json:"official_result_available"`
	OfficialResultAccuracyPct       float64  `json:"official_result_accuracy_pct"`
	ReportDecisionStatus            string   `json:"report_decision_status"`
	ModelDecisionStatus             string   `json:"model_decision_status"`
	ActualOpen                      float64  `json:"actual_open"`
	ActualClose                     float64  `json:"actual_close"`
	PredictedOpen                   float64  `json:"predicted_open"`
	PredictedClose                  float64  `json:"predicted_close"`
	ScenarioPredictedOpen           float64  `json:"scenario_predicted_open,omitempty"`
	ScenarioPredictedClose          float64  `json:"scenario_predicted_close,omitempty"`
	ModelForecastPublishable        bool     `json:"model_forecast_publishable"`
	ModelForecastPublishStatus      string   `json:"model_forecast_publish_status"`
	ModelForecastSuppressionReason  string   `json:"model_forecast_suppression_reason,omitempty"`
	PublishedPredictedOpen          *float64 `json:"published_predicted_open,omitempty"`
	PublishedPredictedClose         *float64 `json:"published_predicted_close,omitempty"`
	ActualOpenKurus                 int64    `json:"actual_open_kurus"`
	ActualCloseKurus                int64    `json:"actual_close_kurus"`
	PredictedOpenKurus              int64    `json:"predicted_open_kurus"`
	PredictedCloseKurus             int64    `json:"predicted_close_kurus"`
	ScenarioPredictedOpenKurus      int64    `json:"scenario_predicted_open_kurus,omitempty"`
	ScenarioPredictedCloseKurus     int64    `json:"scenario_predicted_close_kurus,omitempty"`
	ActualOpenDirection             string   `json:"actual_open_direction"`
	ActualCloseDirection            string   `json:"actual_close_direction"`
	PredictedOpenDirection          string   `json:"predicted_open_direction"`
	PredictedCloseDirection         string   `json:"predicted_close_direction"`
	ScenarioPredictedOpenDirection  string   `json:"scenario_predicted_open_direction,omitempty"`
	ScenarioPredictedCloseDirection string   `json:"scenario_predicted_close_direction,omitempty"`
	OpenExactHit                    bool     `json:"open_exact_hit"`
	CloseExactHit                   bool     `json:"close_exact_hit"`
	OpenDirectionHit                bool     `json:"open_direction_hit"`
	CloseDirectionHit               bool     `json:"close_direction_hit"`
	OpenErrorKurus                  int64    `json:"open_error_kurus"`
	CloseErrorKurus                 int64    `json:"close_error_kurus"`
	OpenErrorTL                     float64  `json:"open_error_tl"`
	CloseErrorTL                    float64  `json:"close_error_tl"`
	OpenErrorPct                    float64  `json:"open_error_pct"`
	CloseErrorPct                   float64  `json:"close_error_pct"`
	OpenAbsErrorPct                 float64  `json:"open_abs_error_pct"`
	CloseAbsErrorPct                float64  `json:"close_abs_error_pct"`
	OpenAccuracyPct                 float64  `json:"open_accuracy_pct"`
	CloseAccuracyPct                float64  `json:"close_accuracy_pct"`
	OpenWrongPct                    float64  `json:"open_wrong_pct"`
	CloseWrongPct                   float64  `json:"close_wrong_pct"`
	OpenClosenessScorePct           float64  `json:"open_closeness_score_pct"`
	CloseClosenessScorePct          float64  `json:"close_closeness_score_pct"`
	LastClose                       float64  `json:"last_close"`
	Model                           string   `json:"model"`
	ModelWarnings                   []string `json:"model_warnings,omitempty"`
	ForecastContext                 string   `json:"forecast_context,omitempty"`
	ForecastContextWarnings         []string `json:"forecast_context_warnings,omitempty"`
	TradeSignalAllowed              bool     `json:"trade_signal_allowed"`
	DecisionConfidence              string   `json:"decision_confidence,omitempty"`
	DirectionModelUnreliable        bool     `json:"direction_model_unreliable,omitempty"`
	ExpectedIntradayDirection       string   `json:"expected_intraday_direction,omitempty"`
	VolatilityRegime                string   `json:"volatility_regime,omitempty"`
	BacktestCloseMAPE               float64  `json:"backtest_close_mape,omitempty"`
	BacktestDirectionAccuracy       float64  `json:"backtest_direction_accuracy,omitempty"`
	ErrorCauses                     []string `json:"error_causes,omitempty"`
	AsOfSourcePath                  string   `json:"as_of_source_path,omitempty"`
	ActualSourcePath                string   `json:"actual_source_path,omitempty"`
}

type forecastAuditRangeForecastResult struct {
	Forecast analysis.NextSessionForecast
	Context  string
	Warnings []string
}

type forecastAuditRangeForecastBuilder func(ctx context.Context, symbol string, asOf datasource.DailyBulletinRecord, forecastFor string, prefixRecords []datasource.DailyBulletinRecord) (forecastAuditRangeForecastResult, error)

type forecastAuditDataScope struct {
	Source                      string `json:"source"`
	RecordsLoaded               int    `json:"records_loaded"`
	CandlesUsedForForecast      int    `json:"candles_used_for_forecast"`
	FirstForecastCandle         string `json:"first_forecast_candle,omitempty"`
	LastForecastCandle          string `json:"last_forecast_candle,omitempty"`
	AsOfRecordSourcePath        string `json:"as_of_record_source_path,omitempty"`
	ActualRecordSourcePath      string `json:"actual_record_source_path,omitempty"`
	OnlyUsesDataThroughAsOf     bool   `json:"only_uses_data_through_as_of"`
	ActualUsedOnlyForValidation bool   `json:"actual_used_only_for_validation"`
}

type forecastAuditFile struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Bytes  int64  `json:"bytes,omitempty"`
}

func runForecastAudit(ctx context.Context, cfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	fs := flag.NewFlagSet("forecast-audit", flag.ExitOnError)
	symbol := fs.String("symbol", "", "BIST sembolu, ornek: ASELS")
	asOfText := fs.String("as-of", "", "tahminin uretilecegi son resmi seans tarihi (YYYY-MM-DD)")
	actualText := fs.String("actual-date", "", "dogrulama icin gerceklesen seans tarihi; bos ise sonraki is gunu")
	fromText := fs.String("from", "", "aralik tablo baslangic tarihi (YYYY-MM-DD); verilirse sonrasindaki her resmi seans icin tablo uretir")
	toText := fs.String("to", "", "aralik tablo bitis tarihi (YYYY-MM-DD); bos ise bugun")
	dataDir := fs.String("data", cfg.DataDir, "veri kok dizini")
	outDir := fs.String("out", store.Root(), "equities kok cikti klasoru")
	limit := fs.Int("limit", 0, "BIST bulteninden okunacak son resmi gun sayisi; 0 tum gecmis")
	if err := fs.Parse(args); err != nil {
		return err
	}
	normalized := ohlcv.NormalizeSymbol(*symbol)
	if normalized == "" {
		return fmt.Errorf("forecast-audit: -symbol is required")
	}
	if *limit < 0 {
		*limit = 0
	}
	provider := datasource.NewBISTBulletinDBProvider(bistBulletinDBPath(*dataDir))
	if strings.TrimSpace(*fromText) != "" {
		return runForecastAuditRange(ctx, provider, normalized, *fromText, *toText, *outDir, *limit)
	}
	asOf, err := parseOptionalDate(*asOfText)
	if err != nil {
		return err
	}
	if asOf.IsZero() {
		return fmt.Errorf("forecast-audit: -as-of is required")
	}
	actualDate := strings.TrimSpace(*actualText)
	if actualDate == "" {
		actualDate = nextForecastAuditBusinessDay(asOf)
	} else if parsed, err := parseOptionalDate(actualDate); err == nil {
		actualDate = parsed.Format("2006-01-02")
	} else {
		return err
	}
	records, err := provider.FetchDailyBulletinRecordsRange(ctx, normalized, "", actualDate, *limit)
	if err != nil {
		return fmt.Errorf("fetch BIST bulletin records: %w", err)
	}
	report, err := buildForecastAuditReport(normalized, asOf, actualDate, records)
	if err != nil {
		return err
	}
	targetDir := filepath.Join(*outDir, normalized, "analysis", fmt.Sprintf("%s_forecast_audit_%s", asOf.Format("2006-01-02"), time.Now().Format("15-04-05")))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("ensure forecast audit dir: %w", err)
	}
	report.GeneratedFiles = []forecastAuditFile{
		forecastAuditFileInfo("forecast_audit.json", filepath.Join(targetDir, "forecast_audit.json")),
		forecastAuditFileInfo("report.html", filepath.Join(targetDir, "report.html")),
		forecastAuditFileInfo("rapor.html", filepath.Join(targetDir, "rapor.html")),
		forecastAuditFileInfo("rapor_veri_manifesti.json", filepath.Join(targetDir, "rapor_veri_manifesti.json")),
	}
	if err := writeForecastAuditJSON(filepath.Join(targetDir, "forecast_audit.json"), report); err != nil {
		return err
	}
	htmlReport := forecastAuditHTML(report)
	if err := os.WriteFile(filepath.Join(targetDir, "report.html"), []byte(htmlReport), 0o644); err != nil {
		return fmt.Errorf("write forecast audit report.html: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "rapor.html"), []byte(htmlReport), 0o644); err != nil {
		return fmt.Errorf("write forecast audit rapor.html: %w", err)
	}
	report.GeneratedFiles = []forecastAuditFile{
		forecastAuditFileInfo("forecast_audit.json", filepath.Join(targetDir, "forecast_audit.json")),
		forecastAuditFileInfo("report.html", filepath.Join(targetDir, "report.html")),
		forecastAuditFileInfo("rapor.html", filepath.Join(targetDir, "rapor.html")),
		forecastAuditFileInfo("rapor_veri_manifesti.json", filepath.Join(targetDir, "rapor_veri_manifesti.json")),
	}
	if err := writeForecastAuditJSON(filepath.Join(targetDir, "rapor_veri_manifesti.json"), forecastAuditManifest(report)); err != nil {
		return err
	}
	report.GeneratedFiles = []forecastAuditFile{
		forecastAuditFileInfo("forecast_audit.json", filepath.Join(targetDir, "forecast_audit.json")),
		forecastAuditFileInfo("report.html", filepath.Join(targetDir, "report.html")),
		forecastAuditFileInfo("rapor.html", filepath.Join(targetDir, "rapor.html")),
		forecastAuditFileInfo("rapor_veri_manifesti.json", filepath.Join(targetDir, "rapor_veri_manifesti.json")),
	}
	if err := writeForecastAuditJSON(filepath.Join(targetDir, "forecast_audit.json"), report); err != nil {
		return err
	}
	if err := writeForecastAuditJSON(filepath.Join(targetDir, "rapor_veri_manifesti.json"), forecastAuditManifest(report)); err != nil {
		return err
	}
	fmt.Printf("forecast audit written for %s: %s\n", normalized, targetDir)
	return nil
}

func runForecastAuditRange(ctx context.Context, provider datasource.BulletinRecordRangeProvider, symbol, fromText, toText, outDir string, limit int) error {
	from, err := parseOptionalDate(fromText)
	if err != nil {
		return err
	}
	if from.IsZero() {
		return fmt.Errorf("forecast-audit range: -from is required")
	}
	to := time.Now()
	if strings.TrimSpace(toText) != "" {
		to, err = parseOptionalDate(toText)
		if err != nil {
			return err
		}
	}
	if to.Before(from) {
		return fmt.Errorf("forecast-audit range: -to %s is before -from %s", to.Format("2006-01-02"), from.Format("2006-01-02"))
	}
	if limit < 0 {
		limit = 0
	}
	records, err := provider.FetchDailyBulletinRecordsRange(ctx, symbol, "", to.Format("2006-01-02"), limit)
	if err != nil {
		return fmt.Errorf("fetch BIST bulletin range records: %w", err)
	}
	report, err := buildForecastAuditRangeReportWithBuilder(ctx, symbol, from, to, records, newForecastAuditContextAwareForecastBuilder(outDir))
	if err != nil {
		return err
	}
	targetDir := filepath.Join(outDir, symbol, "analysis", fmt.Sprintf("%s_%s_forecast_audit_range_%s", from.Format("2006-01-02"), to.Format("2006-01-02"), time.Now().Format("15-04-05")))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("ensure forecast audit range dir: %w", err)
	}
	markdown := forecastAuditRangeMarkdown(report)
	htmlReport := forecastAuditRangeHTML(report)
	if err := writeForecastAuditJSON(filepath.Join(targetDir, "forecast_audit_range.json"), report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "forecast_audit_range.md"), []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("write forecast audit range markdown: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "report.html"), []byte(htmlReport), 0o644); err != nil {
		return fmt.Errorf("write forecast audit range html: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "rapor.html"), []byte(htmlReport), 0o644); err != nil {
		return fmt.Errorf("write forecast audit range rapor html: %w", err)
	}
	report.GeneratedFiles = []forecastAuditFile{
		forecastAuditFileInfo("forecast_audit_range.json", filepath.Join(targetDir, "forecast_audit_range.json")),
		forecastAuditFileInfo("forecast_audit_range.md", filepath.Join(targetDir, "forecast_audit_range.md")),
		forecastAuditFileInfo("report.html", filepath.Join(targetDir, "report.html")),
		forecastAuditFileInfo("rapor.html", filepath.Join(targetDir, "rapor.html")),
	}
	if err := writeForecastAuditJSON(filepath.Join(targetDir, "forecast_audit_range.json"), report); err != nil {
		return err
	}
	fmt.Printf("forecast audit range written for %s: %s\n\n%s", symbol, targetDir, markdown)
	return nil
}

func buildForecastAuditRangeReport(symbol string, from, to time.Time, records []datasource.DailyBulletinRecord) (forecastAuditRangeReport, error) {
	return buildForecastAuditRangeReportWithBuilder(context.Background(), symbol, from, to, records, nil)
}

func buildForecastAuditRangeReportWithBuilder(ctx context.Context, symbol string, from, to time.Time, records []datasource.DailyBulletinRecord, builder forecastAuditRangeForecastBuilder) (forecastAuditRangeReport, error) {
	fromDate := from.Format("2006-01-02")
	toDate := to.Format("2006-01-02")
	if ctx == nil {
		ctx = context.Background()
	}
	if builder == nil {
		builder = forecastAuditTechnicalContextForecast
	}
	rows := []forecastAuditRangeRow{}
	files := forecastAuditRecordFiles("bist_history", records)
	for actualIdx := 1; actualIdx < len(records); actualIdx++ {
		actual := records[actualIdx]
		if actual.TradingDate < fromDate || actual.TradingDate > toDate {
			continue
		}
		if actual.Open <= 0 || actual.Close <= 0 {
			continue
		}
		prefixRecords := append([]datasource.DailyBulletinRecord{}, records[:actualIdx]...)
		asOf := prefixRecords[len(prefixRecords)-1]
		prefixCandles := forecastAuditCandles(prefixRecords)
		if len(prefixCandles) < 30 {
			continue
		}
		forecastResult, err := builder(ctx, symbol, asOf, actual.TradingDate, prefixRecords)
		if err != nil || !forecastResult.Forecast.Computed {
			continue
		}
		forecast := forecastResult.Forecast
		forecast.ForecastFor = actual.TradingDate
		forecast = analysis.AttachActualToNextSessionForecast(forecast, actual.Open, actual.Close, "bist_thb_official_bulletin", actual.SourcePath)
		actualOpenKurus := forecastAuditPriceKurus(actual.Open)
		actualCloseKurus := forecastAuditPriceKurus(actual.Close)
		scenarioPredictedOpen := forecast.PredictedOpen
		scenarioPredictedClose := forecast.PredictedClose
		scenarioPredictedOpenKurus := forecastAuditPriceKurus(scenarioPredictedOpen)
		scenarioPredictedCloseKurus := forecastAuditPriceKurus(scenarioPredictedClose)
		openErrorKurus := actualOpenKurus - scenarioPredictedOpenKurus
		closeErrorKurus := actualCloseKurus - scenarioPredictedCloseKurus
		openErrorPct := forecastAuditAbsErrorPctFromKurus(openErrorKurus, actualOpenKurus)
		closeErrorPct := forecastAuditAbsErrorPctFromKurus(closeErrorKurus, actualCloseKurus)
		publishable, publishStatus, suppressReason := forecastAuditForecastPublishState(forecast)
		var publishedOpen, publishedClose *float64
		predictedOpen := 0.0
		predictedClose := 0.0
		predictedOpenKurus := int64(0)
		predictedCloseKurus := int64(0)
		predictedOpenDirection := ""
		predictedCloseDirection := ""
		if publishable {
			openValue := scenarioPredictedOpen
			closeValue := scenarioPredictedClose
			publishedOpen = &openValue
			publishedClose = &closeValue
			predictedOpen = openValue
			predictedClose = closeValue
			predictedOpenKurus = scenarioPredictedOpenKurus
			predictedCloseKurus = scenarioPredictedCloseKurus
			predictedOpenDirection = forecastAuditDirection(scenarioPredictedOpen, forecast.LastClose)
			predictedCloseDirection = forecastAuditDirection(scenarioPredictedClose, forecast.LastClose)
		}
		row := forecastAuditRangeRow{
			AsOfDate:                        asOf.TradingDate,
			ActualDate:                      actual.TradingDate,
			OfficialResultAvailable:         true,
			OfficialResultAccuracyPct:       100,
			ReportDecisionStatus:            "official_actual_verified",
			ModelDecisionStatus:             forecastAuditModelDecisionStatus(forecast),
			ActualOpen:                      actual.Open,
			ActualClose:                     actual.Close,
			PredictedOpen:                   predictedOpen,
			PredictedClose:                  predictedClose,
			ScenarioPredictedOpen:           scenarioPredictedOpen,
			ScenarioPredictedClose:          scenarioPredictedClose,
			ModelForecastPublishable:        publishable,
			ModelForecastPublishStatus:      publishStatus,
			ModelForecastSuppressionReason:  suppressReason,
			PublishedPredictedOpen:          publishedOpen,
			PublishedPredictedClose:         publishedClose,
			ActualOpenKurus:                 actualOpenKurus,
			ActualCloseKurus:                actualCloseKurus,
			PredictedOpenKurus:              predictedOpenKurus,
			PredictedCloseKurus:             predictedCloseKurus,
			ScenarioPredictedOpenKurus:      scenarioPredictedOpenKurus,
			ScenarioPredictedCloseKurus:     scenarioPredictedCloseKurus,
			ActualOpenDirection:             forecastAuditDirection(actual.Open, forecast.LastClose),
			ActualCloseDirection:            forecastAuditDirection(actual.Close, forecast.LastClose),
			PredictedOpenDirection:          predictedOpenDirection,
			PredictedCloseDirection:         predictedCloseDirection,
			ScenarioPredictedOpenDirection:  forecastAuditDirection(scenarioPredictedOpen, forecast.LastClose),
			ScenarioPredictedCloseDirection: forecastAuditDirection(scenarioPredictedClose, forecast.LastClose),
			OpenExactHit:                    openErrorKurus == 0,
			CloseExactHit:                   closeErrorKurus == 0,
			OpenErrorKurus:                  openErrorKurus,
			CloseErrorKurus:                 closeErrorKurus,
			OpenErrorTL:                     forecastAuditKurusTL(openErrorKurus),
			CloseErrorTL:                    forecastAuditKurusTL(closeErrorKurus),
			OpenErrorPct:                    openErrorPct,
			CloseErrorPct:                   closeErrorPct,
			OpenAbsErrorPct:                 openErrorPct,
			CloseAbsErrorPct:                closeErrorPct,
			OpenAccuracyPct:                 forecastAuditExactAccuracyPct(openErrorKurus),
			CloseAccuracyPct:                forecastAuditExactAccuracyPct(closeErrorKurus),
			OpenWrongPct:                    forecastAuditExactWrongPct(openErrorKurus),
			CloseWrongPct:                   forecastAuditExactWrongPct(closeErrorKurus),
			OpenClosenessScorePct:           forecastAuditClosenessScorePct(openErrorPct),
			CloseClosenessScorePct:          forecastAuditClosenessScorePct(closeErrorPct),
			LastClose:                       forecast.LastClose,
			Model:                           forecast.Model,
			ModelWarnings:                   forecast.Warnings,
			ForecastContext:                 forecastResult.Context,
			ForecastContextWarnings:         forecastResult.Warnings,
			TradeSignalAllowed:              forecast.DecisionForecast.TradeSignalAllowed,
			DecisionConfidence:              forecast.DecisionForecast.Confidence,
			DirectionModelUnreliable:        forecast.DirectionModelUnreliable || forecast.DecisionForecast.DirectionModelUnreliable,
			ExpectedIntradayDirection:       forecast.DecisionForecast.ExpectedIntradayDirection,
			VolatilityRegime:                forecast.DecisionForecast.VolatilityRegime,
			BacktestCloseMAPE:               forecast.BacktestMetrics.CloseMAPE,
			BacktestDirectionAccuracy:       forecast.BacktestMetrics.DirectionAccuracy,
			AsOfSourcePath:                  asOf.SourcePath,
			ActualSourcePath:                actual.SourcePath,
		}
		if forecast.OpenDirectionHit != nil {
			row.OpenDirectionHit = *forecast.OpenDirectionHit
		}
		if forecast.CloseDirectionHit != nil {
			row.CloseDirectionHit = *forecast.CloseDirectionHit
		}
		row.ErrorCauses = forecastAuditRangeErrorCauses(row)
		rows = append(rows, row)
		files = append(files, forecastAuditFileInfo("as_of_"+asOf.TradingDate, asOf.SourcePath))
		files = append(files, forecastAuditFileInfo("actual_"+actual.TradingDate, actual.SourcePath))
	}
	if len(rows) == 0 {
		return forecastAuditRangeReport{}, fmt.Errorf("forecast-audit range: no rows for %s between %s and %s", symbol, fromDate, toDate)
	}
	summary := buildForecastAuditRangeSummary(rows)
	return forecastAuditRangeReport{
		SchemaVersion: 5,
		Name:          "Son 1 ay resmi gerceklesen ve model tahmin denetimi",
		Symbol:        symbol,
		FromDate:      fromDate,
		ToDate:        toDate,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Summary:       summary,
		Rows:          rows,
		FilesUsed:     dedupeForecastAuditFiles(files),
	}, nil
}

func buildForecastAuditRangeSummary(rows []forecastAuditRangeRow) forecastAuditRangeSummary {
	if len(rows) == 0 {
		return forecastAuditRangeSummary{}
	}
	summary := forecastAuditRangeSummary{Rows: len(rows), FirstActualDate: rows[0].ActualDate, LastActualDate: rows[len(rows)-1].ActualDate}
	contextCounts := map[string]int{}
	regimeStats := map[string]*forecastAuditRegimeAccumulator{}
	openAbs := 0.0
	closeAbs := 0.0
	openAccuracy := 0.0
	closeAccuracy := 0.0
	openCloseness := 0.0
	closeCloseness := 0.0
	openWithin050 := 0
	closeWithin050 := 0
	openWithin100 := 0
	closeWithin100 := 0
	openWithin200 := 0
	closeWithin200 := 0
	for _, row := range rows {
		if row.OfficialResultAvailable {
			summary.OfficialObservedRows++
		}
		contextCounts[row.ForecastContext]++
		if row.ForecastContext == "ohlcv_fast_patterns_bist_kap_scope" {
			summary.FullContextRows++
		} else {
			summary.TechnicalFallbackRows++
		}
		if row.ModelForecastPublishable {
			summary.ModelPublishedRows++
		} else {
			summary.ModelSuppressedRows++
		}
		if row.TradeSignalAllowed {
			summary.TradeAllowedRows++
		}
		if row.OpenExactHit {
			summary.OpenExactHits++
		}
		if row.CloseExactHit {
			summary.CloseExactHits++
		}
		if row.OpenErrorPct <= 0.50 {
			openWithin050++
		}
		if row.CloseErrorPct <= 0.50 {
			closeWithin050++
		}
		if row.OpenErrorPct <= 1.00 {
			openWithin100++
		}
		if row.CloseErrorPct <= 1.00 {
			closeWithin100++
		}
		if row.OpenErrorPct <= 2.00 {
			openWithin200++
		}
		if row.CloseErrorPct <= 2.00 {
			closeWithin200++
		}
		if row.OpenDirectionHit {
			summary.OpenDirectionHits++
		}
		if row.CloseDirectionHit {
			summary.CloseDirectionHits++
		}
		openAbs += row.OpenErrorPct
		closeAbs += row.CloseErrorPct
		openAccuracy += row.OpenAccuracyPct
		closeAccuracy += row.CloseAccuracyPct
		openCloseness += row.OpenClosenessScorePct
		closeCloseness += row.CloseClosenessScorePct
		acc := regimeStats[forecastAuditRangeRegimeKey(row)]
		if acc == nil {
			acc = &forecastAuditRegimeAccumulator{
				key:               forecastAuditRangeRegimeKey(row),
				volatilityRegime:  normalizedForecastAuditBucket(row.VolatilityRegime, "unknown_vol"),
				expectedDirection: normalizedForecastAuditBucket(row.ExpectedIntradayDirection, "unknown_direction"),
			}
			regimeStats[acc.key] = acc
		}
		acc.rows++
		acc.closeAbs += row.CloseErrorPct
		if row.CloseDirectionHit {
			acc.closeDirectionHits++
		}
		if row.CloseErrorPct <= 1.00 {
			acc.closeWithin100++
		}
		if row.TradeSignalAllowed {
			acc.tradeAllowed++
		}
		if row.ModelForecastPublishable {
			acc.modelPublished++
		}
	}
	summary.OfficialResultHitPct = forecastAuditPct(summary.OfficialObservedRows, len(rows))
	summary.OfficialOpenExactHitPct = summary.OfficialResultHitPct
	summary.OfficialCloseExactHitPct = summary.OfficialResultHitPct
	if summary.OfficialResultHitPct == 100 {
		summary.ReportDecisionStatus = "official_actual_verified"
	} else {
		summary.ReportDecisionStatus = "official_actual_partial"
	}
	summary.ModelPublishedPct = forecastAuditPct(summary.ModelPublishedRows, len(rows))
	summary.ModelSuppressedPct = forecastAuditPct(summary.ModelSuppressedRows, len(rows))
	switch {
	case summary.ModelPublishedRows == 0:
		summary.PublishableReportStatus = "no_publishable_forecast"
	case summary.ModelPublishedRows < len(rows):
		summary.PublishableReportStatus = "partial_publishable_forecast"
	default:
		summary.PublishableReportStatus = "publishable_forecast"
	}
	summary.TradeAllowedPct = forecastAuditPct(summary.TradeAllowedRows, len(rows))
	summary.OpenExactHitPct = forecastAuditPct(summary.OpenExactHits, len(rows))
	summary.CloseExactHitPct = forecastAuditPct(summary.CloseExactHits, len(rows))
	summary.OpenPriceWrongPct = roundAuditMetric(100 - summary.OpenExactHitPct)
	summary.ClosePriceWrongPct = roundAuditMetric(100 - summary.CloseExactHitPct)
	summary.OpenWithin050Pct = forecastAuditPct(openWithin050, len(rows))
	summary.CloseWithin050Pct = forecastAuditPct(closeWithin050, len(rows))
	summary.OpenWithin100Pct = forecastAuditPct(openWithin100, len(rows))
	summary.CloseWithin100Pct = forecastAuditPct(closeWithin100, len(rows))
	summary.OpenWithin200Pct = forecastAuditPct(openWithin200, len(rows))
	summary.CloseWithin200Pct = forecastAuditPct(closeWithin200, len(rows))
	summary.OpenDirectionHitPct = forecastAuditPct(summary.OpenDirectionHits, len(rows))
	summary.CloseDirectionHitPct = forecastAuditPct(summary.CloseDirectionHits, len(rows))
	summary.OpenDirectionWrongPct = roundAuditMetric(100 - summary.OpenDirectionHitPct)
	summary.CloseDirectionWrongPct = roundAuditMetric(100 - summary.CloseDirectionHitPct)
	summary.OpenMAEPct = roundAuditMetric(openAbs / float64(len(rows)))
	summary.CloseMAEPct = roundAuditMetric(closeAbs / float64(len(rows)))
	summary.OpenErrorPct = summary.OpenMAEPct
	summary.CloseErrorPct = summary.CloseMAEPct
	summary.OpenAccuracyPct = roundAuditMetric(openAccuracy / float64(len(rows)))
	summary.CloseAccuracyPct = roundAuditMetric(closeAccuracy / float64(len(rows)))
	summary.OpenClosenessScorePct = roundAuditMetric(openCloseness / float64(len(rows)))
	summary.CloseClosenessScorePct = roundAuditMetric(closeCloseness / float64(len(rows)))
	summary.ForecastContext = forecastAuditRangeContextSummary(contextCounts)
	summary.RegimePerformance = forecastAuditRegimePerformanceRows(regimeStats)
	summary.ForecastQualityGrade, summary.ForecastQualityNotes = forecastAuditRangeQuality(summary)
	return summary
}

type forecastAuditRegimeAccumulator struct {
	key                string
	volatilityRegime   string
	expectedDirection  string
	rows               int
	closeAbs           float64
	closeDirectionHits int
	closeWithin100     int
	tradeAllowed       int
	modelPublished     int
}

func forecastAuditRegimePerformanceRows(stats map[string]*forecastAuditRegimeAccumulator) []forecastAuditRegimePerformance {
	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]forecastAuditRegimePerformance, 0, len(keys))
	for _, key := range keys {
		acc := stats[key]
		if acc == nil || acc.rows == 0 {
			continue
		}
		out = append(out, forecastAuditRegimePerformance{
			Regime:               acc.key,
			VolatilityRegime:     acc.volatilityRegime,
			ExpectedDirection:    acc.expectedDirection,
			Rows:                 acc.rows,
			CloseMAEPct:          roundAuditMetric(acc.closeAbs / float64(acc.rows)),
			CloseDirectionHitPct: forecastAuditPct(acc.closeDirectionHits, acc.rows),
			CloseWithin100Pct:    forecastAuditPct(acc.closeWithin100, acc.rows),
			TradeAllowedPct:      forecastAuditPct(acc.tradeAllowed, acc.rows),
			ModelPublishedPct:    forecastAuditPct(acc.modelPublished, acc.rows),
		})
	}
	return out
}

func forecastAuditRangeRegimeKey(row forecastAuditRangeRow) string {
	return normalizedForecastAuditBucket(row.VolatilityRegime, "unknown_vol") + "/" + normalizedForecastAuditBucket(row.ExpectedIntradayDirection, "unknown_direction")
}

func normalizedForecastAuditBucket(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func forecastAuditPct(count, total int) float64 {
	if total <= 0 {
		return 0
	}
	return roundAuditMetric(100 * float64(count) / float64(total))
}

func forecastAuditRangeQuality(summary forecastAuditRangeSummary) (string, []string) {
	notes := []string{"Exact fiyat isabeti ikincil denetim metriğidir; kalite band, MAE, yön ve publish gate ile okunur."}
	switch {
	case summary.Rows == 0:
		return "unknown", notes
	case summary.ModelPublishedRows == 0 || summary.TradeAllowedRows == 0:
		notes = append(notes, "Model bu aralıkta karar/emir kalitesinde tahmin yayınlamadı; yayın kararı Yayınlanmadı/no-trade olarak verilir.")
		return "scenario_only", notes
	case summary.CloseMAEPct <= 1.00 && summary.CloseDirectionHitPct >= 60 && summary.CloseWithin100Pct >= 60:
		notes = append(notes, "Kapanış hatası, yön uyumu ve 1% band isabeti karar destek için güçlü görünüyor.")
		return "strong", notes
	case summary.CloseMAEPct <= 1.50 && summary.CloseDirectionHitPct >= 55 && summary.CloseWithin200Pct >= 70:
		notes = append(notes, "Model gate ile kullanılabilir; 1% bandı sınırlıysa pozisyon boyutu ve güven düşük tutulmalı.")
		return "usable_with_gate", notes
	default:
		if summary.CloseMAEPct > 1.50 {
			notes = append(notes, fmt.Sprintf("Kapanış MAE %.2f%%; fiyat bandı riski yüksek.", summary.CloseMAEPct))
		}
		if summary.CloseDirectionHitPct < 55 {
			notes = append(notes, fmt.Sprintf("Kapanış yön uyumu %.2f%%; yön modeli güvenilir değil.", summary.CloseDirectionHitPct))
		}
		return "weak", notes
	}
}

func forecastAuditTechnicalContextForecast(ctx context.Context, symbol string, asOf datasource.DailyBulletinRecord, forecastFor string, prefixRecords []datasource.DailyBulletinRecord) (forecastAuditRangeForecastResult, error) {
	if err := ctx.Err(); err != nil {
		return forecastAuditRangeForecastResult{}, err
	}
	prefixCandles := forecastAuditCandles(prefixRecords)
	forecast, err := analysis.ComputeNextSessionForecastFromCandles(prefixCandles, ohlcv.AssetTypeEquity)
	if err != nil {
		return forecastAuditRangeForecastResult{}, err
	}
	forecast.ForecastFor = forecastFor
	forecast = stripForecastAuditActual(forecast)
	forecast = analysis.ApplyBISTBulletinRecordsToNextSessionForecast(forecast, prefixRecords, ohlcv.AssetTypeEquity, symbol)
	forecast = analysis.ApplyBISTBulletinBacktestToNextSessionForecast(forecast, prefixRecords, ohlcv.AssetTypeEquity)
	return forecastAuditRangeForecastResult{
		Forecast: forecast,
		Context:  "technical_ohlcv_bist_only",
		Warnings: []string{
			"full_context_analysis_engine_not_used",
			"kap_professional_fundamental_overlay_not_included",
		},
	}, nil
}

func newForecastAuditContextAwareForecastBuilder(outDir string) forecastAuditRangeForecastBuilder {
	kapCache := map[string][]forecastAuditKAPDisclosure{}
	kapLoadWarnings := map[string]string{}
	return func(ctx context.Context, symbol string, asOf datasource.DailyBulletinRecord, forecastFor string, prefixRecords []datasource.DailyBulletinRecord) (forecastAuditRangeForecastResult, error) {
		result, err := forecastAuditTechnicalContextForecast(ctx, symbol, asOf, forecastFor, prefixRecords)
		if err != nil {
			return forecastAuditRangeForecastResult{}, err
		}
		result.Context = "ohlcv_fast_patterns_bist_kap_scope"
		result.Warnings = []string{
			"forecast_source_includes_ohlcv_indicators_fast_pattern_suite_support_resistance_trade_plan_bist_context",
			"full_pattern_catalog_runs_in_main_analysis_engine_not_monthly_audit",
			"official_actual_used_only_after_forecast_for_validation",
		}
		normalized := ohlcv.NormalizeSymbol(symbol)
		kapDisclosures, ok := kapCache[normalized]
		if !ok {
			kapDisclosures, kapLoadWarnings[normalized] = forecastAuditLoadKAPDisclosures(outDir, normalized)
			kapCache[normalized] = kapDisclosures
		}
		kapLoadWarning := kapLoadWarnings[normalized]
		result.Warnings = append(result.Warnings, forecastAuditKAPDisclosureWarningsFrom(kapDisclosures, kapLoadWarning, symbol, asOf.TradingDate)...)
		return result, nil
	}
}

func stripForecastAuditActual(f analysis.NextSessionForecast) analysis.NextSessionForecast {
	f.ActualAvailable = false
	f.ActualOpen = 0
	f.ActualClose = 0
	f.ActualSource = ""
	f.ActualSourcePath = ""
	f.ActualOpenErrorPct = 0
	f.ActualCloseErrorPct = 0
	f.OpenForecastErrorTL = 0
	f.CloseForecastErrorTL = 0
	f.OpenAbsErrorPctVsActual = 0
	f.CloseAbsErrorPctVsActual = 0
	f.OpenDirectionHit = nil
	f.CloseDirectionHit = nil
	if f.ValidationStatus == "actual_observed" {
		f.ValidationStatus = ""
	}
	if f.ValidationSource == "bist_thb_official_bulletin" {
		f.ValidationSource = ""
	}
	return f
}

type forecastAuditKAPDisclosure struct {
	ID                 string `json:"id"`
	Ticker             string `json:"ticker"`
	Title              string `json:"title"`
	DisclosureClass    string `json:"disclosure_class"`
	DisclosureType     string `json:"disclosure_type"`
	DisclosureCategory string `json:"disclosure_category"`
	PublishDate        string `json:"publish_date"`
}

func forecastAuditKAPDisclosureWarnings(equitiesDir, symbol, asOfDate string) []string {
	symbol = ohlcv.NormalizeSymbol(symbol)
	if symbol == "" || strings.TrimSpace(equitiesDir) == "" || strings.TrimSpace(asOfDate) == "" {
		return []string{"kap_disclosure_scope_unavailable"}
	}
	disclosures, loadWarning := forecastAuditLoadKAPDisclosures(equitiesDir, symbol)
	return forecastAuditKAPDisclosureWarningsFrom(disclosures, loadWarning, symbol, asOfDate)
}

func forecastAuditLoadKAPDisclosures(equitiesDir, symbol string) ([]forecastAuditKAPDisclosure, string) {
	symbol = ohlcv.NormalizeSymbol(symbol)
	if symbol == "" || strings.TrimSpace(equitiesDir) == "" {
		return nil, "kap_disclosure_scope_unavailable"
	}
	path := filepath.Join(equitiesDir, symbol, "kap_disclosures.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "kap_disclosures_file_missing_or_unreadable:" + path
	}
	var disclosures []forecastAuditKAPDisclosure
	if err := json.Unmarshal(raw, &disclosures); err != nil {
		return nil, "kap_disclosures_json_invalid:" + err.Error()
	}
	return disclosures, ""
}

func forecastAuditKAPDisclosureWarningsFrom(disclosures []forecastAuditKAPDisclosure, loadWarning, symbol, asOfDate string) []string {
	symbol = ohlcv.NormalizeSymbol(symbol)
	if strings.TrimSpace(loadWarning) != "" {
		return []string{loadWarning}
	}
	if symbol == "" || strings.TrimSpace(asOfDate) == "" {
		return []string{"kap_disclosure_scope_unavailable"}
	}
	asOf, err := time.ParseInLocation("2006-01-02", asOfDate, time.UTC)
	if err != nil {
		return []string{"kap_disclosure_asof_invalid:" + asOfDate}
	}
	cutoff := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	totalKnown := 0
	recent30 := 0
	eventHits := []string{}
	for _, disclosure := range disclosures {
		if disclosure.Ticker != "" && !strings.EqualFold(ohlcv.NormalizeSymbol(disclosure.Ticker), symbol) {
			continue
		}
		published, ok := forecastAuditParseKAPPublishDate(disclosure.PublishDate)
		if !ok || published.After(cutoff) {
			continue
		}
		totalKnown++
		if cutoff.Sub(published) <= 30*24*time.Hour {
			recent30++
			if forecastAuditKAPTitleLooksPriceRelevant(disclosure.Title) && len(eventHits) < 3 {
				eventHits = append(eventHits, fmt.Sprintf("%s:%s", published.Format("2006-01-02"), forecastAuditShortText(disclosure.Title, 80)))
			}
		}
	}
	warnings := []string{
		fmt.Sprintf("kap_disclosures_asof_known:%d", totalKnown),
		fmt.Sprintf("kap_disclosures_recent_30d:%d", recent30),
	}
	if len(eventHits) > 0 {
		warnings = append(warnings, "kap_recent_price_relevant_events:"+strings.Join(eventHits, " | "))
	} else {
		warnings = append(warnings, "kap_recent_price_relevant_events:none")
	}
	return warnings
}

func forecastAuditParseKAPPublishDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), true
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "02.01.2006 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func forecastAuditKAPTitleLooksPriceRelevant(title string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	if title == "" {
		return false
	}
	keywords := []string{
		"sözleşme", "sozlesme", "ihale", "sipariş", "siparis", "teslimat", "ödeme", "odeme",
		"satış", "satis", "anlaşma", "anlasma", "yatırım", "yatirim", "kar payı", "temettü",
		"sermaye", "bedelsiz", "bedelli", "geri alım", "geri alim", "ceza", "dava", "iptal",
	}
	for _, keyword := range keywords {
		if strings.Contains(title, keyword) {
			return true
		}
	}
	return false
}

func forecastAuditShortText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}

func forecastAuditRangeContextSummary(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		if strings.TrimSpace(key) == "" {
			key = "unknown"
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}

func buildForecastAuditReport(symbol string, asOf time.Time, actualDate string, records []datasource.DailyBulletinRecord) (forecastAuditReport, error) {
	asOfDate := asOf.Format("2006-01-02")
	forecastRecords := make([]datasource.DailyBulletinRecord, 0, len(records))
	var asOfRecord datasource.DailyBulletinRecord
	var actualRecord datasource.DailyBulletinRecord
	for _, record := range records {
		if record.TradingDate <= asOfDate {
			forecastRecords = append(forecastRecords, record)
			if record.TradingDate == asOfDate {
				asOfRecord = record
			}
		}
		if record.TradingDate == actualDate {
			actualRecord = record
		}
	}
	if len(forecastRecords) < 30 {
		return forecastAuditReport{}, fmt.Errorf("forecast-audit: insufficient BIST records through %s: got %d", asOfDate, len(forecastRecords))
	}
	if asOfRecord.TradingDate == "" {
		asOfRecord = forecastRecords[len(forecastRecords)-1]
	}
	candles := forecastAuditCandles(forecastRecords)
	forecast, err := analysis.ComputeNextSessionForecastFromCandles(candles, ohlcv.AssetTypeEquity)
	if err != nil {
		return forecastAuditReport{}, err
	}
	forecast.ForecastFor = actualDate
	forecast = stripForecastAuditActual(forecast)
	forecast = analysis.ApplyBISTBulletinRecordsToNextSessionForecast(forecast, forecastRecords, ohlcv.AssetTypeEquity, symbol)
	forecast = analysis.ApplyBISTBulletinBacktestToNextSessionForecast(forecast, forecastRecords, ohlcv.AssetTypeEquity)
	status := "actual_missing"
	actualAvailable := actualRecord.Open > 0 && actualRecord.Close > 0
	if actualAvailable {
		status = "actual_observed"
		forecast = analysis.AttachActualToNextSessionForecast(forecast, actualRecord.Open, actualRecord.Close, "bist_thb_official_bulletin", actualRecord.SourcePath)
	}
	validation := buildForecastAuditValidation(forecast)
	officialResult := buildForecastAuditOfficialResult(forecast, actualRecord, actualAvailable)
	modelValidation := buildForecastAuditModelValidation(symbol, forecastRecords, 60)
	reportDecisionStatus := forecastAuditReportDecisionStatus(actualAvailable)
	modelDecisionStatus := forecastAuditModelDecisionStatus(forecast)
	scope := forecastAuditDataScope{
		Source:                      "bist_thb_official_bulletin",
		RecordsLoaded:               len(records),
		CandlesUsedForForecast:      len(candles),
		AsOfRecordSourcePath:        asOfRecord.SourcePath,
		ActualRecordSourcePath:      actualRecord.SourcePath,
		OnlyUsesDataThroughAsOf:     true,
		ActualUsedOnlyForValidation: actualAvailable,
	}
	if len(candles) > 0 {
		scope.FirstForecastCandle = candles[0].Time.Format("2006-01-02")
		scope.LastForecastCandle = candles[len(candles)-1].Time.Format("2006-01-02")
	}
	files := []forecastAuditFile{
		forecastAuditFileInfo("as_of_bist_bulletin", asOfRecord.SourcePath),
	}
	files = append(files, forecastAuditRecordFiles("forecast_history", forecastRecords)...)
	if actualRecord.SourcePath != "" {
		files = append(files, forecastAuditFileInfo("actual_bist_bulletin", actualRecord.SourcePath))
	}
	return forecastAuditReport{
		SchemaVersion:        1,
		Name:                 "Sonraki seans geriye donuk tahmin denetimi",
		Symbol:               symbol,
		AsOfDate:             asOfDate,
		ForecastSession:      forecast.ForecastFor,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		Status:               status,
		ReportDecisionStatus: reportDecisionStatus,
		ModelDecisionStatus:  modelDecisionStatus,
		LastObservedRecord:   asOfRecord,
		ActualRecord:         actualRecord,
		ActualAvailable:      actualAvailable,
		OfficialResult:       officialResult,
		Forecast:             forecast,
		Validation:           validation,
		ModelValidation:      modelValidation,
		DataScope:            scope,
		FilesUsed:            dedupeForecastAuditFiles(files),
		Interpretation:       forecastAuditInterpretation(forecast, validation, actualAvailable, asOfDate),
	}, nil
}

func buildForecastAuditModelValidation(symbol string, records []datasource.DailyBulletinRecord, window int) forecastAuditModelValidation {
	out := forecastAuditModelValidation{WindowSessions: window}
	if len(records) < 30 || window <= 0 {
		return out
	}
	start := len(records) - window
	if start < 25 {
		start = 25
	}
	baselineOpenAbs := 0.0
	baselineCloseAbs := 0.0
	baselineDirectionHits := 0
	microOpenAbs := 0.0
	microCloseAbs := 0.0
	microDirectionHits := 0
	triggerHits := 0
	for nextIdx := start; nextIdx < len(records); nextIdx++ {
		actual := records[nextIdx]
		if actual.Open <= 0 || actual.Close <= 0 {
			continue
		}
		prefixRecords := append([]datasource.DailyBulletinRecord{}, records[:nextIdx]...)
		prefixCandles := forecastAuditCandles(prefixRecords)
		if len(prefixCandles) < 25 {
			continue
		}
		baseline, err := analysis.ComputeNextSessionForecastFromCandles(prefixCandles, ohlcv.AssetTypeEquity)
		if err != nil || !baseline.Computed || baseline.PredictedOpen <= 0 || baseline.PredictedClose <= 0 || baseline.LastClose <= 0 {
			continue
		}
		micro := analysis.ApplyBISTBulletinRecordsToNextSessionForecastForAudit(baseline, prefixRecords, ohlcv.AssetTypeEquity, symbol)
		baselineOpenAbs += math.Abs(100 * (baseline.PredictedOpen/actual.Open - 1))
		baselineCloseAbs += math.Abs(100 * (baseline.PredictedClose/actual.Close - 1))
		if forecastAuditDirectionHit(baseline.PredictedClose, baseline.LastClose, actual.Close) {
			baselineDirectionHits++
		}
		microOpenAbs += math.Abs(100 * (micro.PredictedOpen/actual.Open - 1))
		microCloseAbs += math.Abs(100 * (micro.PredictedClose/actual.Close - 1))
		if forecastAuditDirectionHit(micro.PredictedClose, micro.LastClose, actual.Close) {
			microDirectionHits++
		}
		if strings.Contains(strings.ToLower(micro.Model), "bist_microstructure_v2") {
			out.MicrostructureTriggeredSamples++
			if forecastAuditDirectionHit(micro.PredictedClose, micro.LastClose, actual.Close) {
				triggerHits++
			}
		}
		out.Baseline.Samples++
		out.Microstructure.Samples++
	}
	if out.Baseline.Samples > 0 {
		out.Baseline.OpenMAEPct = roundAuditMetric(baselineOpenAbs / float64(out.Baseline.Samples))
		out.Baseline.CloseMAEPct = roundAuditMetric(baselineCloseAbs / float64(out.Baseline.Samples))
		out.Baseline.DirectionHitRatePct = roundAuditMetric(100 * float64(baselineDirectionHits) / float64(out.Baseline.Samples))
	}
	if out.Microstructure.Samples > 0 {
		out.Microstructure.OpenMAEPct = roundAuditMetric(microOpenAbs / float64(out.Microstructure.Samples))
		out.Microstructure.CloseMAEPct = roundAuditMetric(microCloseAbs / float64(out.Microstructure.Samples))
		out.Microstructure.DirectionHitRatePct = roundAuditMetric(100 * float64(microDirectionHits) / float64(out.Microstructure.Samples))
	}
	if out.MicrostructureTriggeredSamples > 0 {
		out.MicrostructureTriggerHitRatePct = roundAuditMetric(100 * float64(triggerHits) / float64(out.MicrostructureTriggeredSamples))
	}
	return out
}

func forecastAuditCandles(records []datasource.DailyBulletinRecord) []ohlcv.Candle {
	candles := make([]ohlcv.Candle, 0, len(records))
	for _, record := range records {
		if record.Close <= 0 || record.TradingDate == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", record.TradingDate)
		if err != nil {
			continue
		}
		open := record.Open
		if open <= 0 {
			open = record.Close
		}
		low := record.Low
		if low <= 0 {
			low = math.Min(open, record.Close)
		}
		high := record.High
		if high <= 0 {
			high = math.Max(open, record.Close)
		}
		candles = append(candles, ohlcv.Candle{
			Time:           date.UTC(),
			Open:           open,
			High:           high,
			Low:            low,
			Close:          record.Close,
			Volume:         record.Volume,
			AdjustedOpen:   open,
			AdjustedHigh:   high,
			AdjustedLow:    low,
			AdjustedClose:  record.Close,
			AdjustedVolume: record.Volume,
		})
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].Time.Before(candles[j].Time) })
	return candles
}

func buildForecastAuditValidation(f analysis.NextSessionForecast) forecastAuditValidation {
	out := forecastAuditValidation{
		OpenDirection:  forecastAuditDirection(f.PredictedOpen, f.LastClose),
		CloseDirection: forecastAuditDirection(f.PredictedClose, f.LastClose),
	}
	if f.ActualAvailable {
		out.ActualOpenDirection = forecastAuditDirection(f.ActualOpen, f.LastClose)
		out.ActualCloseDirection = forecastAuditDirection(f.ActualClose, f.LastClose)
		out.OpenDirectionHit = f.OpenDirectionHit
		out.CloseDirectionHit = f.CloseDirectionHit
		out.OpenErrorTL = f.OpenForecastErrorTL
		out.CloseErrorTL = f.CloseForecastErrorTL
		out.OpenAbsErrorPct = f.OpenAbsErrorPctVsActual
		out.CloseAbsErrorPct = f.CloseAbsErrorPctVsActual
	}
	return out
}

func buildForecastAuditOfficialResult(f analysis.NextSessionForecast, actual datasource.DailyBulletinRecord, actualAvailable bool) forecastAuditOfficialResult {
	out := forecastAuditOfficialResult{
		Available:       false,
		Authoritative:   false,
		Status:          "pending_actual_session",
		CalculationMode: "point_in_time_forecast_only",
	}
	if !actualAvailable {
		return out
	}
	out.Available = true
	out.Authoritative = true
	out.Status = "official_actual_observed"
	out.CalculationMode = "bist_official_actual_overrides_forecast_for_observed_session"
	out.Open = actual.Open
	out.Close = actual.Close
	out.OpenDirection = forecastAuditDirection(actual.Open, f.LastClose)
	out.CloseDirection = forecastAuditDirection(actual.Close, f.LastClose)
	out.Source = "bist_thb_official_bulletin"
	out.SourcePath = actual.SourcePath
	out.SourceTradingDay = actual.TradingDate
	return out
}

func forecastAuditDirection(price, lastClose float64) string {
	return forecastpolicy.AuditDirectionFromPrice(price, lastClose)
}

func forecastAuditDirectionHit(predictedClose, lastClose, actualClose float64) bool {
	hit, ok := forecastpolicy.DirectionHit(predictedClose, actualClose, lastClose)
	return ok && hit
}

func roundAuditMetric(value float64) float64 {
	return math.Round(value*100) / 100
}

func forecastAuditPriceKurus(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Round(value * 100))
}

func forecastAuditKurusTL(value int64) float64 {
	return float64(value) / 100
}

func forecastAuditAbsErrorPctFromKurus(errorKurus, actualKurus int64) float64 {
	if actualKurus <= 0 {
		return 0
	}
	return roundAuditMetric(100 * math.Abs(float64(errorKurus)) / float64(actualKurus))
}

func forecastAuditExactAccuracyPct(errorKurus int64) float64 {
	if errorKurus == 0 {
		return 100
	}
	return 0
}

func forecastAuditExactWrongPct(errorKurus int64) float64 {
	if errorKurus == 0 {
		return 0
	}
	return 100
}

func forecastAuditClosenessScorePct(errorPct float64) float64 {
	return roundAuditMetric(math.Max(0, 100-errorPct))
}

func forecastAuditPublishedPriceText(value *float64) string {
	if value == nil || *value <= 0 {
		return "Yayınlanmadı"
	}
	return fmt.Sprintf("%.2f", *value)
}

func forecastAuditModelDecisionText(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "model_not_decision_grade":
		return "senaryo; karar/emir degil"
	case "technical_decision_failed":
		return "teknik kapi zayif"
	case "model_provisional":
		return "sinirli senaryo"
	case "":
		return "belirsiz"
	default:
		return status
	}
}

func forecastAuditScenarioUsageText(row forecastAuditRangeRow) string {
	if row.ModelForecastPublishable {
		return "karar/emir icin yayinlandi"
	}
	return forecastAuditPublishStatusText(row)
}

func forecastAuditDecisionOutcomeText(row forecastAuditRangeRow) string {
	if !row.ModelForecastPublishable {
		return "dogru kullanim: karar/emir kapali"
	}
	if row.CloseDirectionHit && row.CloseErrorPct <= 1.25 {
		return "karar desteklendi: yon uyumlu, kapanis sapmasi esik icinde"
	}
	if row.CloseDirectionHit {
		return "sinirli: yon uyumlu, kapanis sapmasi yuksek"
	}
	return "uygunsuz: kapanis yonu resmi sonucla uyusmadi"
}

func forecastAuditDirectionAgreementText(value bool) string {
	if value {
		return "uyumlu"
	}
	return "uyumsuz"
}

func forecastAuditDirectionTruthText(value bool) string {
	if value {
		return "Doğru"
	}
	return "Yanlış"
}

func forecastAuditYesNoText(value bool) string {
	if value {
		return "Evet"
	}
	return "Hayır"
}

func forecastAuditOverallResultText(row forecastAuditRangeRow) string {
	switch {
	case row.OpenExactHit && row.CloseExactHit:
		return "Tam isabet (ikincil exact)"
	case !row.ModelForecastPublishable || !row.TradeSignalAllowed:
		return "Yayınlanmadı/no-trade: " + forecastAuditPrimaryOutcomeText(row)
	case row.CloseErrorPct <= 1.00 && row.CloseDirectionHit:
		return "Band içinde + yön uyumlu"
	case row.CloseErrorPct <= 1.00:
		return "Fiyat bandı içinde, yön riski var"
	case row.CloseDirectionHit:
		return "Yön uyumlu, fiyat bandı dışında"
	default:
		return "Band ve yön uyumsuz"
	}
}

func forecastAuditPrimaryOutcomeText(row forecastAuditRangeRow) string {
	switch {
	case row.CloseErrorPct <= 1.00 && row.CloseDirectionHit:
		return "band içinde + yön uyumlu"
	case row.CloseErrorPct <= 1.00:
		return "fiyat bandı içinde, yön riski var"
	case row.CloseDirectionHit:
		return "yön uyumlu, fiyat bandı dışında"
	default:
		return "band ve yön uyumsuz"
	}
}

func forecastAuditBandText(errorPct float64) string {
	switch {
	case errorPct <= 0.50:
		return "0.5% içi"
	case errorPct <= 1.00:
		return "1% içi"
	case errorPct <= 2.00:
		return "2% içi"
	default:
		return "band dışı"
	}
}

func forecastAuditMovementText(direction string) string {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "yukari", "yükseliş", "up", "positive":
		return "olumlu"
	case "asagi", "aşağı", "düşüş", "down", "negative":
		return "olumsuz"
	case "yatay", "flat", "neutral":
		return "yatay"
	case "":
		return "belirsiz"
	default:
		return direction
	}
}

func forecastAuditMovementPairText(openDirection, closeDirection string) string {
	return fmt.Sprintf("A: %s; K: %s", forecastAuditMovementText(openDirection), forecastAuditMovementText(closeDirection))
}

func forecastAuditDirectionTruthPairText(row forecastAuditRangeRow) string {
	return fmt.Sprintf("A: %s; K: %s", forecastAuditDirectionTruthText(row.OpenDirectionHit), forecastAuditDirectionTruthText(row.CloseDirectionHit))
}

func forecastAuditCompactDecisionText(row forecastAuditRangeRow) string {
	if !row.ModelForecastPublishable {
		return "Yayınlanmadı / doğru"
	}
	if row.CloseDirectionHit && row.CloseErrorPct <= 1.25 {
		return "Açık / destekli"
	}
	if row.CloseDirectionHit {
		return "Açık / riskli"
	}
	return "Açık / yanlış"
}

func forecastAuditCompactNote(row forecastAuditRangeRow) string {
	notes := []string{}
	if !row.ModelForecastPublishable {
		notes = append(notes, "emir yok")
	}
	if !row.OpenDirectionHit {
		notes = append(notes, "A yön yanlış")
	}
	if !row.CloseDirectionHit {
		notes = append(notes, "K yön yanlış")
	}
	if row.CloseErrorPct > 1.25 {
		notes = append(notes, fmt.Sprintf("K sapma %.2f%%", row.CloseErrorPct))
	}
	if len(notes) == 0 {
		return "model uyumlu"
	}
	return strings.Join(notes, "; ")
}

func forecastAuditPriceDetectionSourceText(row forecastAuditRangeRow) string {
	if row.OfficialResultAvailable {
		return "resmi BIST sonucu"
	}
	return "model senaryosu"
}

func forecastAuditScenarioNote(row forecastAuditRangeRow) string {
	notes := []string{}
	if row.OfficialResultAvailable {
		notes = append(notes, "gecmis seans: fiyat/yon tespiti resmi sonuc ile yapildi")
	}
	if row.ModelForecastPublishable {
		notes = append(notes, "model senaryosu karar/emir kapisini gecti")
	} else {
		notes = append(notes, "model fiyatı yayınlanmadı; iç senaryo sadece audit amaçlıdır")
	}
	if !row.OpenDirectionHit {
		notes = append(notes, "model acilis yonu resmi sonucla uyusmadi")
	}
	if !row.CloseDirectionHit {
		notes = append(notes, "model kapanis yonu resmi sonucla uyusmadi")
	}
	if row.OpenErrorKurus != 0 {
		notes = append(notes, fmt.Sprintf("model acilis farki %+0.2f TL", row.OpenErrorTL))
	}
	if row.CloseErrorKurus != 0 {
		notes = append(notes, fmt.Sprintf("model kapanis farki %+0.2f TL", row.CloseErrorTL))
	}
	for _, warning := range row.ModelWarnings {
		switch strings.ToLower(strings.TrimSpace(warning)) {
		case "bist_bulletin_overlay_validation_failed":
			notes = append(notes, "BIST analog katmani rolling dogrulama zayif oldugu icin devre disi")
		case "bist_bulletin_overlay_damped_by_validation":
			notes = append(notes, "BIST analog fiyat hareketi zayif dogrulama nedeniyle son kapanisa yaklastirildi")
		case "forecast_model_validation_failed_not_decision_grade":
			notes = append(notes, "model gecmis dogrulama esiklerini gecmedi")
		}
	}
	if len(notes) == 0 {
		return "kontrol notu yok"
	}
	return strings.Join(notes, "; ")
}

func forecastAuditDirectionHitText(value bool) string {
	if value {
		return "uyumlu"
	}
	return "uyumsuz"
}

func forecastAuditRangeErrorCauses(row forecastAuditRangeRow) []string {
	causes := []string{}
	if !row.ModelForecastPublishable {
		causes = append(causes, "model fiyatı yayınlanmadı; iç senaryo karar/emir girdisi değil: "+forecastAuditSuppressionReasonText(row))
	}
	if !row.TradeSignalAllowed {
		causes = append(causes, fmt.Sprintf("trade sinyali kapalı: son 20 kapanış MAPE %.2f%%, yön doğruluğu %.2f%%", row.BacktestCloseMAPE, row.BacktestDirectionAccuracy))
	}
	if row.DirectionModelUnreliable {
		causes = append(causes, "yön modeli güvenilmez: son 20 yön doğruluğu %55 eşiğinin altında")
	}
	if !row.OpenExactHit {
		causes = append(causes, fmt.Sprintf("model açılış tahmini resmi açılıştan farklı: %+d kuruş", row.OpenErrorKurus))
	}
	if !row.CloseExactHit {
		causes = append(causes, fmt.Sprintf("model kapanış tahmini resmi kapanıştan farklı: %+d kuruş", row.CloseErrorKurus))
	}
	if !row.OpenDirectionHit {
		causes = append(causes, "model açılış yönü resmi yöne uymadı")
	}
	if !row.CloseDirectionHit {
		causes = append(causes, "model kapanış yönü resmi yöne uymadı")
	}
	if strings.Contains(strings.ToLower(row.Model), "bist_microstructure_v2") {
		causes = append(causes, "BIST mikro yapı düzeltmesi tetiklendi")
	}
	for _, warning := range row.ModelWarnings {
		switch strings.ToLower(strings.TrimSpace(warning)) {
		case "bist_bulletin_overlay_validation_failed":
			causes = append(causes, "BIST analog/mikro yapı katmanı rolling doğrulama zayıf olduğu için devre dışı bırakıldı")
		case "bist_bulletin_overlay_damped_by_validation":
			causes = append(causes, "BIST analog/mikro yapı fiyat hareketi zayıf rolling doğrulama nedeniyle son kapanışa yaklaştırıldı")
		case "forecast_model_validation_failed_not_decision_grade":
			causes = append(causes, "model geçmiş doğrulaması zayıf; tahmin karar/emir girdisi değil")
		}
	}
	if len(causes) == 0 {
		causes = append(causes, "tüm kontroller geçti")
	}
	return causes
}

func forecastAuditForecastPublishState(f analysis.NextSessionForecast) (bool, string, string) {
	f.ActualAvailable = false
	f.ActualOpen = 0
	f.ActualClose = 0
	f.ActualSource = ""
	f.ActualSourcePath = ""
	gated := analysis.ApplyNextSessionForecastPublishState(f)
	if gated.PointForecastPublishable {
		return true, "published", ""
	}
	status := strings.TrimSpace(gated.PointForecastStatus)
	if status == "" {
		status = "not_published"
	}
	reason := strings.TrimSpace(gated.PointForecastSuppressionReason)
	if reason == "" {
		reason = "forecast_not_publishable"
	}
	return false, status, forecastAuditPublishSuppressionText(reason)
}

func forecastAuditPublishSuppressionText(reason string) string {
	lower := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.HasPrefix(lower, "backtest_samples_below_30:"):
		return "nokta fiyat için en az 30 benzer geçmiş örnek yok: " + strings.TrimPrefix(lower, "backtest_samples_below_30:")
	case strings.HasPrefix(lower, "backtest_direction_hit_below_55pct:"):
		return "backtest yön uyumu %55 eşiğinin altında: " + strings.TrimPrefix(lower, "backtest_direction_hit_below_55pct:") + "%"
	case strings.HasPrefix(lower, "backtest_close_mae_above_1_25pct:"):
		return "backtest kapanış MAE %1.25 eşiğinin üstünde: " + strings.TrimPrefix(lower, "backtest_close_mae_above_1_25pct:") + "%"
	case strings.HasPrefix(lower, "backtest_close_mape_above_2pct:"):
		return "son 20 kapanış MAPE %2 yayın eşiğinin üstünde: " + strings.TrimPrefix(lower, "backtest_close_mape_above_2pct:") + "%"
	case strings.HasPrefix(lower, "forecast_confidence_below_55:"):
		return "model güveni %55 altında: " + strings.TrimPrefix(lower, "forecast_confidence_below_55:") + "/100"
	}
	switch lower {
	case "forecast_not_decision_grade":
		return "senaryo üretildi; karar/emir girdisi değil"
	case "technical_decision_gate_not_passed":
		return "teknik karar kapısı geçmedi"
	case "forecast_quality_provisional":
		return "forecast kalitesi provisional"
	case "forecast_not_computed":
		return "model fiyatı hesaplanmadı"
	default:
		return reason
	}
}

func forecastAuditReportDecisionStatus(actualAvailable bool) string {
	if actualAvailable {
		return "official_actual_verified"
	}
	return "official_actual_pending"
}

func forecastAuditModelDecisionStatus(f analysis.NextSessionForecast) string {
	status := strings.TrimSpace(f.Status)
	quality := strings.TrimSpace(f.Quality)
	switch {
	case strings.EqualFold(status, "model_validation_failed") || strings.EqualFold(quality, "not_decision_grade"):
		return "model_not_decision_grade"
	case strings.EqualFold(status, "technical_decision_context_failed"):
		return "technical_decision_failed"
	case strings.EqualFold(quality, "provisional"):
		return "model_provisional"
	case status != "" || quality != "":
		if status == "" {
			return quality
		}
		if quality == "" {
			return status
		}
		return status + "/" + quality
	default:
		return "model_status_unknown"
	}
}

func forecastAuditInterpretation(f analysis.NextSessionForecast, v forecastAuditValidation, actualAvailable bool, asOfDate string) []string {
	items := []string{
		fmt.Sprintf("Tahmin %s kapanisindan sonra uretilmistir; forecast mumlari %s seansinda biter.", formatAuditPrice(f.LastClose), asOfDate),
		fmt.Sprintf("Tahmin edilen fiyat yonu: acilis %s, kapanis %s.", v.OpenDirection, v.CloseDirection),
	}
	publishable, _, suppressReason := forecastAuditForecastPublishState(f)
	if publishable {
		items = append(items, fmt.Sprintf("Nokta fiyat yayin kapisi gecti: acilis/kapanis %s / %s.", formatAuditPrice(f.PredictedOpen), formatAuditPrice(f.PredictedClose)))
	} else {
		items = append(items, "Karar/emir kapisi gecmedi: "+suppressReason+". Senaryo fiyatlari yalniz model denetimi olarak gosterilir.")
	}
	if actualAvailable {
		items = append(items,
			"Resmi BIST gerceklesen sonucu mevcut oldugu icin raporun kesinlesen fiyat sonucu model tahmini degil, resmi bulten satiridir.",
			fmt.Sprintf("Gerceklesen acilis/kapanis: %s / %s.", formatAuditPrice(f.ActualOpen), formatAuditPrice(f.ActualClose)),
			fmt.Sprintf("Acilis hata: %s TL, %.2f%%; kapanis hata: %s TL, %.2f%%.", formatSignedAuditPrice(v.OpenErrorTL), v.OpenAbsErrorPct, formatSignedAuditPrice(v.CloseErrorTL), v.CloseAbsErrorPct),
		)
	}
	return items
}

func forecastAuditHTML(report forecastAuditReport) string {
	f := report.Forecast
	v := report.Validation
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"tr\"><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(report.Symbol + " forecast audit"))
	b.WriteString("</title><style>body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;margin:32px;color:#17202a}table{border-collapse:collapse;width:100%;margin:16px 0}th,td{border:1px solid #d9dee7;padding:8px;text-align:left}th{background:#f4f6f8}.ok{color:#117a37}.bad{color:#b42318}.muted{color:#667085}code{background:#f4f6f8;padding:2px 4px}</style></head><body>")
	b.WriteString("<h1>" + html.EscapeString(report.Symbol) + " Sonraki Seans Geriye Donuk Tahmin Denetimi</h1>")
	b.WriteString("<p class=\"muted\">As-of: " + html.EscapeString(report.AsOfDate) + " | Tahmin edilen seans: " + html.EscapeString(report.ForecastSession) + " | Durum: " + html.EscapeString(report.Status) + "</p>")
	b.WriteString("<h2>Kesinlesmis Resmi Sonuc</h2><table><tr><th>Alan</th><th>Deger</th></tr>")
	forecastAuditRow(&b, "Rapor karar uygunlugu", report.ReportDecisionStatus)
	forecastAuditRow(&b, "Model karar durumu", report.ModelDecisionStatus)
	forecastAuditRow(&b, "Hesaplama modu", report.OfficialResult.CalculationMode)
	forecastAuditRow(&b, "Resmi sonuc durumu", report.OfficialResult.Status)
	forecastAuditRow(&b, "Resmi acilis", formatAuditPrice(report.OfficialResult.Open))
	forecastAuditRow(&b, "Resmi kapanis", formatAuditPrice(report.OfficialResult.Close))
	forecastAuditRow(&b, "Resmi yon", report.OfficialResult.OpenDirection+" / "+report.OfficialResult.CloseDirection)
	forecastAuditRow(&b, "Resmi kaynak", report.OfficialResult.SourcePath)
	b.WriteString("</table>")
	b.WriteString("<h2>Tahmin ve Gerceklesen</h2><table><tr><th>Alan</th><th>Deger</th></tr>")
	forecastAuditRow(&b, "Son resmi kapanis", formatAuditPrice(f.LastClose))
	forecastAuditRow(&b, "Ham acilis tahmini", formatAuditPrice(f.RawPredictedOpen))
	forecastAuditRow(&b, "Islem gorebilir acilis tahmini", formatAuditPrice(f.PredictedOpen)+" | "+fmt.Sprintf("%.2f%%", f.OpenChangePct))
	forecastAuditRow(&b, "Gerceklesen acilis", formatAuditPrice(f.ActualOpen))
	forecastAuditRow(&b, "Acilis hata", formatSignedAuditPrice(v.OpenErrorTL)+" | "+fmt.Sprintf("%.2f%%", v.OpenAbsErrorPct))
	forecastAuditRow(&b, "Ham kapanis tahmini", formatAuditPrice(f.RawPredictedClose))
	forecastAuditRow(&b, "Islem gorebilir kapanis tahmini", formatAuditPrice(f.PredictedClose)+" | "+fmt.Sprintf("%.2f%%", f.CloseChangePct))
	forecastAuditRow(&b, "Gerceklesen kapanis", formatAuditPrice(f.ActualClose))
	forecastAuditRow(&b, "Kapanis hata", formatSignedAuditPrice(v.CloseErrorTL)+" | "+fmt.Sprintf("%.2f%%", v.CloseAbsErrorPct))
	forecastAuditRow(&b, "Tahmin yonu", v.OpenDirection+" / "+v.CloseDirection)
	forecastAuditRow(&b, "Gerceklesen yon", v.ActualOpenDirection+" / "+v.ActualCloseDirection)
	forecastAuditRow(&b, "Yon uyumu", forecastAuditBoolText(v.OpenDirectionHit)+" / "+forecastAuditBoolText(v.CloseDirectionHit))
	forecastAuditRow(&b, "Fiyat adimi", fmt.Sprintf("%.3f TL | %s", f.TickSize, f.RoundingMethod))
	b.WriteString("</table>")
	b.WriteString("<h2>Model Dogrulama</h2><table><tr><th>Model</th><th>Ornek</th><th>Acilis MAE</th><th>Kapanis MAE</th><th>Yon uyumu</th></tr>")
	forecastAuditModelRow(&b, "Baseline", report.ModelValidation.Baseline)
	forecastAuditModelRow(&b, "BIST tarihsel + mikro yapı", report.ModelValidation.Microstructure)
	b.WriteString("</table>")
	b.WriteString("<p class=\"muted\">Mikro yapı kuralı tetiklenen ornek: " + fmt.Sprintf("%d", report.ModelValidation.MicrostructureTriggeredSamples) + "; tetiklenenlerde kapanis yon uyumu: " + fmt.Sprintf("%.2f%%", report.ModelValidation.MicrostructureTriggerHitRatePct) + "</p>")
	b.WriteString("<h2>Veri Kapsami</h2><table><tr><th>Alan</th><th>Deger</th></tr>")
	forecastAuditRow(&b, "Kaynak", report.DataScope.Source)
	forecastAuditRow(&b, "Forecast mum araligi", report.DataScope.FirstForecastCandle+" - "+report.DataScope.LastForecastCandle)
	forecastAuditRow(&b, "Forecast mum sayisi", fmt.Sprintf("%d", report.DataScope.CandlesUsedForForecast))
	forecastAuditRow(&b, "As-of bulten dosyasi", report.DataScope.AsOfRecordSourcePath)
	forecastAuditRow(&b, "Gerceklesen bulten dosyasi", report.DataScope.ActualRecordSourcePath)
	b.WriteString("</table><h2>Yorum</h2><ul>")
	for _, item := range report.Interpretation {
		b.WriteString("<li>" + html.EscapeString(item) + "</li>")
	}
	b.WriteString("</ul></body></html>")
	return b.String()
}

func forecastAuditPublishedOpenText(row forecastAuditRangeRow) string {
	if row.PublishedPredictedOpen != nil {
		return fmt.Sprintf("%.2f", *row.PublishedPredictedOpen)
	}
	if row.ModelForecastPublishable && row.PredictedOpen > 0 {
		return fmt.Sprintf("%.2f", row.PredictedOpen)
	}
	return "Yayınlanmadı"
}

func forecastAuditPublishedCloseText(row forecastAuditRangeRow) string {
	if row.PublishedPredictedClose != nil {
		return fmt.Sprintf("%.2f", *row.PublishedPredictedClose)
	}
	if row.ModelForecastPublishable && row.PredictedClose > 0 {
		return fmt.Sprintf("%.2f", row.PredictedClose)
	}
	return "Yayınlanmadı"
}

func forecastAuditInternalScenarioPairText(row forecastAuditRangeRow) string {
	openValue := row.ScenarioPredictedOpen
	closeValue := row.ScenarioPredictedClose
	if openValue <= 0 {
		openValue = row.PredictedOpen
	}
	if closeValue <= 0 {
		closeValue = row.PredictedClose
	}
	if openValue <= 0 && closeValue <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f / %.2f", openValue, closeValue)
}

func forecastAuditSuppressionReasonText(row forecastAuditRangeRow) string {
	reason := strings.TrimSpace(row.ModelForecastSuppressionReason)
	if reason == "" {
		reason = strings.TrimSpace(row.ModelForecastPublishStatus)
	}
	if reason == "" {
		return "forecast_not_publishable"
	}
	return reason
}

func forecastAuditPublishStatusText(row forecastAuditRangeRow) string {
	if row.ModelForecastPublishable {
		return "Yayınlandı"
	}
	return "Yayınlanmadı: " + forecastAuditSuppressionReasonText(row)
}

func forecastAuditRangePublishGateNote(summary forecastAuditRangeSummary) string {
	switch {
	case summary.Rows == 0:
		return ""
	case summary.ModelPublishedRows == 0:
		return "Bu aralıkta modelin yayınlanabilir fiyat/yön tahmini yoktur; tüm satırlar publish gate tarafından kapatıldı. İç model senaryosu sadece JSON scenario_* audit alanlarında tutulur, karar/emir girdisi değildir."
	case summary.ModelSuppressedRows > 0:
		return fmt.Sprintf("%d satır publish gate tarafından kapatıldı; yayınlanan tahmin kolonları yalnızca kapıyı geçen fiyatları gösterir.", summary.ModelSuppressedRows)
	default:
		return ""
	}
}

func forecastAuditRangeCauseText(row forecastAuditRangeRow) string {
	causes := row.ErrorCauses
	if len(causes) == 0 {
		causes = forecastAuditRangeErrorCauses(row)
	}
	return strings.Join(limitForecastAuditStrings(causes, 4), "; ")
}

func limitForecastAuditStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	out := append([]string{}, values[:limit]...)
	out = append(out, fmt.Sprintf("%d neden daha var", len(values)-limit))
	return out
}

func forecastAuditRangeMarkdown(report forecastAuditRangeReport) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s %s - %s resmi gerceklesen ve model tahmin denetimi\n\n", report.Symbol, report.Summary.FirstActualDate, report.Summary.LastActualDate))
	b.WriteString(fmt.Sprintf("Birincil kalite: %s | Kapanış 1%% bandı %.2f%% | Kapanış 2%% bandı %.2f%% | Kapanış MAE %.2f%% | Kapanış yön %.2f%% | Trade açık %.2f%%\n\n",
		report.Summary.ForecastQualityGrade,
		report.Summary.CloseWithin100Pct,
		report.Summary.CloseWithin200Pct,
		report.Summary.CloseMAEPct,
		report.Summary.CloseDirectionHitPct,
		report.Summary.TradeAllowedPct,
	))
	b.WriteString(fmt.Sprintf("Açılış kalite: 1%% bandı %.2f%% | 2%% bandı %.2f%% | MAE %.2f%% | yön %.2f%% | Model yayın %.2f%% / baskı %.2f%%\n\n",
		report.Summary.OpenWithin100Pct,
		report.Summary.OpenWithin200Pct,
		report.Summary.OpenMAEPct,
		report.Summary.OpenDirectionHitPct,
		report.Summary.ModelPublishedPct,
		report.Summary.ModelSuppressedPct,
	))
	b.WriteString(fmt.Sprintf("İkincil exact denetim: Açılış birebir %.2f%% | Kapanış birebir %.2f%%. Exact fiyat isabeti ana başarı metriği değildir.\n\n",
		report.Summary.OpenExactHitPct,
		report.Summary.CloseExactHitPct,
	))
	b.WriteString(fmt.Sprintf("Model kaynağı: %s | Kapsamlı satır: %d | Teknik fallback: %d\n\n",
		report.Summary.ForecastContext,
		report.Summary.FullContextRows,
		report.Summary.TechnicalFallbackRows,
	))
	if note := forecastAuditRangePublishGateNote(report.Summary); note != "" {
		b.WriteString("> " + note + "\n\n")
	}
	if len(report.Summary.ForecastQualityNotes) > 0 {
		b.WriteString("Kalite notları:\n")
		for _, note := range report.Summary.ForecastQualityNotes {
			b.WriteString("- " + note + "\n")
		}
		b.WriteString("\n")
	}
	if len(report.Summary.RegimePerformance) > 0 {
		b.WriteString("## Rejim Performansı\n\n")
		b.WriteString("| Rejim | Satır | Kapanış MAE | Kapanış Yön | Kapanış 1% Band | Trade Açık |\n")
		b.WriteString("|---|---:|---:|---:|---:|---:|\n")
		for _, regime := range report.Summary.RegimePerformance {
			b.WriteString(fmt.Sprintf("| %s | %d | %.2f%% | %.2f%% | %.2f%% | %.2f%% |\n",
				regime.Regime,
				regime.Rows,
				regime.CloseMAEPct,
				regime.CloseDirectionHitPct,
				regime.CloseWithin100Pct,
				regime.TradeAllowedPct,
			))
		}
		b.WriteString("\n")
	}
	b.WriteString("Evet/Hayır exact alanları ikincildir. Trade Hayır ise sistem al/sat üretmez. Yayınlanan tahmin kolonları publish gate geçmeyen satırlarda fiyat göstermez.\n\n")
	b.WriteString("| Gün | Açılış | Kapanış | Yayın A | Yayın K | Sonuç | A Band | K Band | K Yön | Trade? | Güven | Kapı | Son 20 Kapanış MAPE | Son 20 Yön |\n")
	b.WriteString("|---|---:|---:|---|---|---|---|---|---|---|---|---|---:|---:|\n")
	for _, row := range report.Rows {
		b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %s | %s | %s | %s | %s | %s | %s | %s | %s | %.2f%% | %.2f%% |\n",
			row.ActualDate,
			row.ActualOpen,
			row.ActualClose,
			forecastAuditPublishedOpenText(row),
			forecastAuditPublishedCloseText(row),
			forecastAuditOverallResultText(row),
			forecastAuditBandText(row.OpenErrorPct),
			forecastAuditBandText(row.CloseErrorPct),
			forecastAuditDirectionHitText(row.CloseDirectionHit),
			forecastAuditYesNoText(row.TradeSignalAllowed),
			emptyForecastAuditValue(row.DecisionConfidence, "-"),
			forecastAuditPublishStatusText(row),
			row.BacktestCloseMAPE,
			row.BacktestDirectionAccuracy,
		))
	}
	b.WriteString("\n## Gün Gün Hata Nedenleri\n\n")
	b.WriteString("| Gün | Hata nedeni |\n")
	b.WriteString("|---|---|\n")
	for _, row := range report.Rows {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", row.ActualDate, forecastAuditRangeCauseText(row)))
	}
	return b.String()
}

func forecastAuditRangeHTML(report forecastAuditRangeReport) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"tr\"><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(report.Symbol + " forecast audit range"))
	b.WriteString("</title><style>body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;margin:32px;color:#17202a}table{border-collapse:collapse;width:100%;margin:16px 0}th,td{border:1px solid #d9dee7;padding:8px;text-align:left;vertical-align:top}th{background:#f4f6f8}.muted{color:#667085}.ok{color:#117a37;font-weight:600}.bad{color:#b42318;font-weight:600}</style></head><body>")
	b.WriteString("<h1>" + html.EscapeString(report.Symbol) + " Son 1 Ay Resmi Gerceklesen ve Model Tahmin Denetimi</h1>")
	b.WriteString("<p class=\"muted\">" + html.EscapeString(fmt.Sprintf("Birincil kalite: %s | Kapanış 1%% bandı %.2f%% | Kapanış 2%% bandı %.2f%% | Kapanış MAE %.2f%% | Kapanış yön %.2f%% | Trade açık %.2f%%",
		report.Summary.ForecastQualityGrade,
		report.Summary.CloseWithin100Pct,
		report.Summary.CloseWithin200Pct,
		report.Summary.CloseMAEPct,
		report.Summary.CloseDirectionHitPct,
		report.Summary.TradeAllowedPct,
	)) + "</p>")
	b.WriteString("<p class=\"muted\">" + html.EscapeString(fmt.Sprintf("İkincil exact denetim: Açılış birebir %.2f%% | Kapanış birebir %.2f%%. Exact fiyat isabeti ana başarı metriği değildir.",
		report.Summary.OpenExactHitPct,
		report.Summary.CloseExactHitPct,
	)) + "</p>")
	b.WriteString("<p class=\"muted\">" + html.EscapeString(fmt.Sprintf("Model kaynağı: %s | Kapsamlı satır: %d | Teknik fallback: %d",
		report.Summary.ForecastContext,
		report.Summary.FullContextRows,
		report.Summary.TechnicalFallbackRows,
	)) + "</p>")
	if note := forecastAuditRangePublishGateNote(report.Summary); note != "" {
		b.WriteString("<p class=\"muted\"><strong>Yayın kapısı:</strong> " + html.EscapeString(note) + "</p>")
	}
	if len(report.Summary.ForecastQualityNotes) > 0 {
		b.WriteString("<h2>Kalite Notları</h2><ul>")
		for _, note := range report.Summary.ForecastQualityNotes {
			b.WriteString("<li>" + html.EscapeString(note) + "</li>")
		}
		b.WriteString("</ul>")
	}
	if len(report.Summary.RegimePerformance) > 0 {
		b.WriteString("<h2>Rejim Performansı</h2><table><thead><tr><th>Rejim</th><th>Satır</th><th>Kapanış MAE</th><th>Kapanış Yön</th><th>Kapanış 1% Band</th><th>Trade Açık</th></tr></thead><tbody>")
		for _, regime := range report.Summary.RegimePerformance {
			b.WriteString("<tr>")
			rangeCell(&b, regime.Regime)
			rangeCell(&b, fmt.Sprintf("%d", regime.Rows))
			rangeCell(&b, fmt.Sprintf("%.2f%%", regime.CloseMAEPct))
			rangeCell(&b, fmt.Sprintf("%.2f%%", regime.CloseDirectionHitPct))
			rangeCell(&b, fmt.Sprintf("%.2f%%", regime.CloseWithin100Pct))
			rangeCell(&b, fmt.Sprintf("%.2f%%", regime.TradeAllowedPct))
			b.WriteString("</tr>")
		}
		b.WriteString("</tbody></table>")
	}
	b.WriteString("<p class=\"muted\">Evet/Hayır exact alanları ikincildir. Trade Hayır ise sistem al/sat üretmez. Yayınlanan tahmin kolonları publish gate geçmeyen satırlarda fiyat göstermez.</p>")
	b.WriteString("<table><thead><tr><th>Gün</th><th>Açılış</th><th>Kapanış</th><th>Yayın A</th><th>Yayın K</th><th>Sonuç</th><th>A Band</th><th>K Band</th><th>K Yön</th><th>Trade?</th><th>Güven</th><th>Kapı</th><th>Son 20 Kapanış MAPE</th><th>Son 20 Yön</th></tr></thead><tbody>")
	for _, row := range report.Rows {
		b.WriteString("<tr>")
		rangeCell(&b, row.ActualDate)
		rangeCell(&b, fmt.Sprintf("%.2f", row.ActualOpen))
		rangeCell(&b, fmt.Sprintf("%.2f", row.ActualClose))
		rangeCell(&b, forecastAuditPublishedOpenText(row))
		rangeCell(&b, forecastAuditPublishedCloseText(row))
		rangeCell(&b, forecastAuditOverallResultText(row))
		rangeCell(&b, forecastAuditBandText(row.OpenErrorPct))
		rangeCell(&b, forecastAuditBandText(row.CloseErrorPct))
		rangeCell(&b, forecastAuditDirectionHitText(row.CloseDirectionHit))
		rangeCell(&b, forecastAuditYesNoText(row.TradeSignalAllowed))
		rangeCell(&b, emptyForecastAuditValue(row.DecisionConfidence, "-"))
		rangeCell(&b, forecastAuditPublishStatusText(row))
		rangeCell(&b, fmt.Sprintf("%.2f%%", row.BacktestCloseMAPE))
		rangeCell(&b, fmt.Sprintf("%.2f%%", row.BacktestDirectionAccuracy))
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	b.WriteString("<h2>Gün Gün Hata Nedenleri</h2><table><thead><tr><th>Gün</th><th>Hata nedeni</th></tr></thead><tbody>")
	for _, row := range report.Rows {
		b.WriteString("<tr>")
		rangeCell(&b, row.ActualDate)
		rangeCell(&b, forecastAuditRangeCauseText(row))
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table></body></html>")
	return b.String()
}

func rangeCell(b *strings.Builder, value string) {
	b.WriteString("<td>" + html.EscapeString(value) + "</td>")
}

func rangeBoolCell(b *strings.Builder, value bool) {
	className := "bad"
	if value {
		className = "ok"
	}
	b.WriteString("<td class=\"" + className + "\">" + forecastAuditBoolValueText(value) + "</td>")
}

func rangeHitCell(b *strings.Builder, value bool) {
	className := "bad"
	if value {
		className = "ok"
	}
	b.WriteString("<td class=\"" + className + "\">" + forecastAuditDirectionHitText(value) + "</td>")
}

func forecastAuditRow(b *strings.Builder, label, value string) {
	b.WriteString("<tr><td>" + html.EscapeString(label) + "</td><td><code>" + html.EscapeString(value) + "</code></td></tr>")
}

func forecastAuditModelRow(b *strings.Builder, label string, metrics forecastAuditBacktestMetrics) {
	b.WriteString("<tr><td>" + html.EscapeString(label) + "</td><td>" + fmt.Sprintf("%d", metrics.Samples) + "</td><td>" + fmt.Sprintf("%.2f%%", metrics.OpenMAEPct) + "</td><td>" + fmt.Sprintf("%.2f%%", metrics.CloseMAEPct) + "</td><td>" + fmt.Sprintf("%.2f%%", metrics.DirectionHitRatePct) + "</td></tr>")
}

func forecastAuditManifest(report forecastAuditReport) map[string]any {
	return map[string]any{
		"schema_version":         1,
		"name":                   "Forecast audit veri manifesti",
		"symbol":                 report.Symbol,
		"as_of_date":             report.AsOfDate,
		"forecast_for":           report.ForecastSession,
		"report_decision_status": report.ReportDecisionStatus,
		"model_decision_status":  report.ModelDecisionStatus,
		"data_scope":             report.DataScope,
		"files_used":             report.FilesUsed,
		"generated_files":        report.GeneratedFiles,
		"official_result":        report.OfficialResult,
		"forecast":               report.Forecast,
		"validation":             report.Validation,
		"model_validation":       report.ModelValidation,
	}
}

func writeForecastAuditJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal forecast audit json: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func forecastAuditFileInfo(label, path string) forecastAuditFile {
	info := forecastAuditFile{Label: label, Path: path}
	if strings.TrimSpace(path) == "" {
		return info
	}
	if stat, err := os.Stat(path); err == nil {
		info.Exists = true
		info.Bytes = stat.Size()
	}
	return info
}

func forecastAuditRecordFiles(labelPrefix string, records []datasource.DailyBulletinRecord) []forecastAuditFile {
	files := make([]forecastAuditFile, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.SourcePath) == "" {
			continue
		}
		label := labelPrefix
		if strings.TrimSpace(record.TradingDate) != "" {
			label += "_" + record.TradingDate
		}
		files = append(files, forecastAuditFileInfo(label, record.SourcePath))
	}
	return files
}

func dedupeForecastAuditFiles(files []forecastAuditFile) []forecastAuditFile {
	out := make([]forecastAuditFile, 0, len(files))
	seen := map[string]bool{}
	for _, file := range files {
		key := strings.TrimSpace(file.Path)
		if key == "" {
			key = strings.TrimSpace(file.Label)
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, file)
	}
	return out
}

func nextForecastAuditBusinessDay(asOf time.Time) string {
	next := asOf.AddDate(0, 0, 1)
	for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
		next = next.AddDate(0, 0, 1)
	}
	return next.Format("2006-01-02")
}

func formatAuditPrice(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f TL", value)
}

func formatSignedAuditPrice(value float64) string {
	if value == 0 {
		return "0.00"
	}
	return fmt.Sprintf("%+.2f", value)
}

func forecastAuditBoolText(value *bool) string {
	if value == nil {
		return "yok"
	}
	return forecastAuditBoolValueText(*value)
}

func forecastAuditBoolValueText(value bool) string {
	if value {
		return "uyumlu"
	}
	return "uyumsuz"
}

func emptyForecastAuditValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}
