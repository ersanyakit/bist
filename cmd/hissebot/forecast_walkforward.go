package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	"hissebot/internal/ta/ohlcv"
)

const (
	forecastLedgerSchemaVersion                = 2
	nextSessionScenarioPointMinBacktestSamples = 30
	nextSessionScenarioPointMaxCloseMAPEPct    = 2.0
	nextSessionScenarioPointMinDirectionHitPct = 50.0
)

type forecastLedgerEntry struct {
	SchemaVersion                   int                          `json:"schema_version"`
	EntryID                         string                       `json:"entry_id"`
	Symbol                          string                       `json:"symbol"`
	AsOfDate                        string                       `json:"as_of_date"`
	TargetDate                      string                       `json:"target_date"`
	Target                          string                       `json:"target"`
	GeneratedAt                     string                       `json:"generated_at"`
	Status                          string                       `json:"status"`
	ForecastAgent                   string                       `json:"forecast_agent"`
	VerifierAgent                   string                       `json:"verifier_agent,omitempty"`
	DataPolicy                      string                       `json:"data_policy"`
	ModelVersion                    string                       `json:"model_version"`
	Model                           string                       `json:"model,omitempty"`
	ForecastContext                 string                       `json:"forecast_context,omitempty"`
	LastClose                       float64                      `json:"last_close"`
	PredictedClose                  float64                      `json:"predicted_close"`
	ScenarioPredictedClose          float64                      `json:"scenario_predicted_close,omitempty"`
	RawPredictedClose               float64                      `json:"raw_predicted_close,omitempty"`
	PublishedPredictedClose         *float64                     `json:"published_predicted_close,omitempty"`
	IntervalLow                     float64                      `json:"interval_low,omitempty"`
	IntervalHigh                    float64                      `json:"interval_high,omitempty"`
	DecisionIntervalLow             float64                      `json:"decision_interval_low,omitempty"`
	DecisionIntervalHigh            float64                      `json:"decision_interval_high,omitempty"`
	DecisionIntervalWidthPct        float64                      `json:"decision_interval_width_pct,omitempty"`
	DecisionIntervalStatus          string                       `json:"decision_interval_status,omitempty"`
	DecisionIntervalReason          string                       `json:"decision_interval_reason,omitempty"`
	Confidence                      float64                      `json:"confidence"`
	ConfidenceLabel                 string                       `json:"confidence_label,omitempty"`
	PredictedCloseDirection         string                       `json:"predicted_close_direction,omitempty"`
	ScenarioPredictedCloseDirection string                       `json:"scenario_predicted_close_direction,omitempty"`
	ScenarioPointAvailable          bool                         `json:"scenario_point_available"`
	ScenarioPointStatus             string                       `json:"scenario_point_status,omitempty"`
	ScenarioPointSuppressionReason  string                       `json:"scenario_point_suppression_reason,omitempty"`
	PointForecastPublishable        bool                         `json:"point_forecast_publishable"`
	PointForecastStatus             string                       `json:"point_forecast_status,omitempty"`
	PointForecastSuppressionReason  string                       `json:"point_forecast_suppression_reason,omitempty"`
	TradeSignalAllowed              bool                         `json:"trade_signal_allowed"`
	DecisionConfidence              string                       `json:"decision_confidence,omitempty"`
	ExpectedIntradayDirection       string                       `json:"expected_intraday_direction,omitempty"`
	VolatilityRegime                string                       `json:"volatility_regime,omitempty"`
	BacktestSamples                 int                          `json:"backtest_samples,omitempty"`
	BacktestCloseMAPE               float64                      `json:"backtest_close_mape,omitempty"`
	BacktestDirectionAccuracy       float64                      `json:"backtest_direction_accuracy,omitempty"`
	DirectionModelUnreliable        bool                         `json:"direction_model_unreliable,omitempty"`
	DataSnapshotHash                string                       `json:"data_snapshot_hash"`
	RecordsThroughAsOf              int                          `json:"records_through_as_of"`
	AsOfSourcePath                  string                       `json:"as_of_source_path,omitempty"`
	FirstForecastRecordDate         string                       `json:"first_forecast_record_date,omitempty"`
	LastForecastRecordDate          string                       `json:"last_forecast_record_date,omitempty"`
	OnlyUsesDataThroughAsOf         bool                         `json:"only_uses_data_through_as_of"`
	ActualUsedOnlyForVerification   bool                         `json:"actual_used_only_for_verification"`
	FeatureScopes                   []string                     `json:"feature_scopes,omitempty"`
	Warnings                        []string                     `json:"warnings,omitempty"`
	RawForecast                     analysis.NextSessionForecast `json:"raw_forecast,omitempty"`
}

type forecastVerificationEvent struct {
	SchemaVersion                  int      `json:"schema_version"`
	VerificationID                 string   `json:"verification_id"`
	EntryID                        string   `json:"entry_id"`
	Symbol                         string   `json:"symbol"`
	AsOfDate                       string   `json:"as_of_date"`
	TargetDate                     string   `json:"target_date"`
	VerifiedAt                     string   `json:"verified_at"`
	Status                         string   `json:"status"`
	VerificationMode               string   `json:"verification_mode,omitempty"`
	VerifierAgent                  string   `json:"verifier_agent"`
	PredictedClose                 float64  `json:"predicted_close,omitempty"`
	ScenarioPredictedClose         float64  `json:"scenario_predicted_close,omitempty"`
	ActualClose                    float64  `json:"actual_close,omitempty"`
	LastClose                      float64  `json:"last_close,omitempty"`
	IntervalLow                    float64  `json:"interval_low,omitempty"`
	IntervalHigh                   float64  `json:"interval_high,omitempty"`
	DecisionIntervalLow            float64  `json:"decision_interval_low,omitempty"`
	DecisionIntervalHigh           float64  `json:"decision_interval_high,omitempty"`
	DecisionIntervalWidthPct       float64  `json:"decision_interval_width_pct,omitempty"`
	DecisionIntervalStatus         string   `json:"decision_interval_status,omitempty"`
	DecisionIntervalReason         string   `json:"decision_interval_reason,omitempty"`
	CloseErrorTL                   float64  `json:"close_error_tl,omitempty"`
	CloseAbsErrorPct               float64  `json:"close_abs_error_pct,omitempty"`
	CloseDirectionHit              *bool    `json:"close_direction_hit,omitempty"`
	ScenarioCloseErrorTL           float64  `json:"scenario_close_error_tl,omitempty"`
	ScenarioCloseAbsErrorPct       float64  `json:"scenario_close_abs_error_pct,omitempty"`
	ScenarioCloseDirectionHit      *bool    `json:"scenario_close_direction_hit,omitempty"`
	IntervalHit                    *bool    `json:"interval_hit,omitempty"`
	DecisionIntervalHit            *bool    `json:"decision_interval_hit,omitempty"`
	OfficialCloseSource            string   `json:"official_close_source,omitempty"`
	OfficialCloseSourcePath        string   `json:"official_close_source_path,omitempty"`
	DataSnapshotHash               string   `json:"data_snapshot_hash,omitempty"`
	Model                          string   `json:"model,omitempty"`
	ForecastContext                string   `json:"forecast_context,omitempty"`
	PointForecastPublishable       bool     `json:"point_forecast_publishable"`
	ScenarioPointAvailable         bool     `json:"scenario_point_available"`
	ScenarioPointStatus            string   `json:"scenario_point_status,omitempty"`
	ScenarioPointSuppressionReason string   `json:"scenario_point_suppression_reason,omitempty"`
	SuppressionReason              string   `json:"suppression_reason,omitempty"`
	Confidence                     float64  `json:"confidence,omitempty"`
	BacktestCloseMAPE              float64  `json:"backtest_close_mape,omitempty"`
	BacktestDirectionAccuracy      float64  `json:"backtest_direction_accuracy,omitempty"`
	Warnings                       []string `json:"warnings,omitempty"`
}

type forecastErrorAuditReport struct {
	SchemaVersion  int                        `json:"schema_version"`
	Symbol         string                     `json:"symbol"`
	FromDate       string                     `json:"from_date,omitempty"`
	ToDate         string                     `json:"to_date,omitempty"`
	GeneratedAt    string                     `json:"generated_at"`
	ThresholdPct   float64                    `json:"threshold_pct"`
	Summary        forecastErrorAuditSummary  `json:"summary"`
	Events         []forecastErrorAttribution `json:"events,omitempty"`
	ScenarioEvents []forecastErrorAttribution `json:"scenario_events,omitempty"`
}

type forecastErrorAuditSummary struct {
	VerifiedEvents             int     `json:"verified_events"`
	PublishedEvents            int     `json:"published_events,omitempty"`
	ScenarioOnlyEvents         int     `json:"scenario_only_events,omitempty"`
	BandOnlyEvents             int     `json:"band_only_events,omitempty"`
	ErrorEvents                int     `json:"error_events"`
	ScenarioErrorEvents        int     `json:"scenario_error_events,omitempty"`
	AverageAbsErrorPct         float64 `json:"average_abs_error_pct,omitempty"`
	MaxAbsErrorPct             float64 `json:"max_abs_error_pct,omitempty"`
	ScenarioAverageAbsErrorPct float64 `json:"scenario_average_abs_error_pct,omitempty"`
	ScenarioMaxAbsErrorPct     float64 `json:"scenario_max_abs_error_pct,omitempty"`
	DirectionMisses            int     `json:"direction_misses,omitempty"`
	ScenarioDirectionMisses    int     `json:"scenario_direction_misses,omitempty"`
	IntervalMisses             int     `json:"interval_misses,omitempty"`
	ScenarioIntervalMisses     int     `json:"scenario_interval_misses,omitempty"`
	PrimaryRecommendation      string  `json:"primary_recommendation,omitempty"`
}

type forecastErrorAttribution struct {
	EventType         string   `json:"event_type,omitempty"`
	EntryID           string   `json:"entry_id"`
	Symbol            string   `json:"symbol"`
	AsOfDate          string   `json:"as_of_date"`
	TargetDate        string   `json:"target_date"`
	VerificationMode  string   `json:"verification_mode,omitempty"`
	ForecastPublished bool     `json:"forecast_published"`
	PctError          float64  `json:"pct_error"`
	PrimaryCause      string   `json:"primary_cause"`
	SecondaryCauses   []string `json:"secondary_causes,omitempty"`
	Evidence          []string `json:"evidence,omitempty"`
	RecommendedFix    string   `json:"recommended_fix,omitempty"`
}

type forecastActualVsAIReport struct {
	SchemaVersion          int                       `json:"schema_version"`
	Symbol                 string                    `json:"symbol"`
	FromDate               string                    `json:"from_date,omitempty"`
	ToDate                 string                    `json:"to_date,omitempty"`
	GeneratedAt            string                    `json:"generated_at"`
	SourceVerificationPath string                    `json:"source_verification_path,omitempty"`
	Summary                forecastActualVsAISummary `json:"summary"`
	Rows                   []forecastActualVsAIRow   `json:"rows"`
}

type forecastActualVsAISummary struct {
	TotalEvents                     int     `json:"total_events"`
	VerifiedEvents                  int     `json:"verified_events"`
	PendingEvents                   int     `json:"pending_events,omitempty"`
	PublishedPointEvents            int     `json:"published_point_events,omitempty"`
	ScenarioOnlyEvents              int     `json:"scenario_only_events,omitempty"`
	BandOnlyEvents                  int     `json:"band_only_events,omitempty"`
	IntervalHits                    int     `json:"interval_hits,omitempty"`
	IntervalMisses                  int     `json:"interval_misses,omitempty"`
	DecisionIntervalEvents          int     `json:"decision_interval_events,omitempty"`
	DecisionIntervalHits            int     `json:"decision_interval_hits,omitempty"`
	DecisionIntervalMisses          int     `json:"decision_interval_misses,omitempty"`
	AverageRiskIntervalWidthPct     float64 `json:"average_risk_interval_width_pct,omitempty"`
	AverageDecisionIntervalWidthPct float64 `json:"average_decision_interval_width_pct,omitempty"`
	AveragePublishedAbsErrorPct     float64 `json:"average_published_abs_error_pct,omitempty"`
	MaxPublishedAbsErrorPct         float64 `json:"max_published_abs_error_pct,omitempty"`
	AverageScenarioOnlyAbsErrorPct  float64 `json:"average_scenario_only_abs_error_pct,omitempty"`
	MaxScenarioOnlyAbsErrorPct      float64 `json:"max_scenario_only_abs_error_pct,omitempty"`
	PublishedDirectionHits          int     `json:"published_direction_hits,omitempty"`
	PublishedDirectionMisses        int     `json:"published_direction_misses,omitempty"`
	ScenarioOnlyDirectionHits       int     `json:"scenario_only_direction_hits,omitempty"`
	ScenarioOnlyDirectionMisses     int     `json:"scenario_only_direction_misses,omitempty"`
	RowsWithoutPointClosePrediction int     `json:"rows_without_point_close_prediction,omitempty"`
}

type forecastActualVsAIRow struct {
	AsOfDate                       string   `json:"as_of_date"`
	TargetDate                     string   `json:"target_date"`
	Status                         string   `json:"status"`
	VerificationMode               string   `json:"verification_mode"`
	LastClose                      float64  `json:"last_close,omitempty"`
	ActualClose                    *float64 `json:"actual_close,omitempty"`
	AIReportedClose                *float64 `json:"ai_reported_close,omitempty"`
	PublishedPredictedClose        *float64 `json:"published_predicted_close,omitempty"`
	ScenarioPredictedClose         *float64 `json:"scenario_predicted_close,omitempty"`
	IntervalLow                    float64  `json:"interval_low,omitempty"`
	IntervalHigh                   float64  `json:"interval_high,omitempty"`
	IntervalHit                    *bool    `json:"interval_hit,omitempty"`
	DecisionIntervalLow            float64  `json:"decision_interval_low,omitempty"`
	DecisionIntervalHigh           float64  `json:"decision_interval_high,omitempty"`
	DecisionIntervalHit            *bool    `json:"decision_interval_hit,omitempty"`
	DecisionIntervalWidthPct       float64  `json:"decision_interval_width_pct,omitempty"`
	DecisionIntervalStatus         string   `json:"decision_interval_status,omitempty"`
	DecisionIntervalReason         string   `json:"decision_interval_reason,omitempty"`
	RiskIntervalWidthPct           float64  `json:"risk_interval_width_pct,omitempty"`
	PublishedCloseAbsErrorPct      *float64 `json:"published_close_abs_error_pct,omitempty"`
	ScenarioOnlyCloseAbsErrorPct   *float64 `json:"scenario_only_close_abs_error_pct,omitempty"`
	CloseDirectionHit              *bool    `json:"close_direction_hit,omitempty"`
	SuppressionReason              string   `json:"suppression_reason,omitempty"`
	ScenarioPointSuppressionReason string   `json:"scenario_point_suppression_reason,omitempty"`
	OfficialCloseSource            string   `json:"official_close_source,omitempty"`
}

func runForecastWalkforward(ctx context.Context, cfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	fs := flag.NewFlagSet("forecast-walkforward", flag.ExitOnError)
	symbol := fs.String("symbol", "ASELS", "BIST sembolu, ornek: ASELS")
	fromText := fs.String("from", "2026-06-01", "walk-forward baslangic target tarihi (YYYY-MM-DD)")
	toText := fs.String("to", "", "walk-forward bitis target tarihi; bos ise bugun")
	dataDir := fs.String("data", cfg.DataDir, "veri kok dizini")
	outDir := fs.String("out", store.Root(), "equities kok cikti klasoru")
	ledgerPath := fs.String("ledger", "", "opsiyonel forecast ledger JSONL yolu")
	target := fs.String("target", "close_t1", "tahmin hedefi; su an yalnizca close_t1 desteklenir")
	limit := fs.Int("limit", 0, "BIST bulteninden okunacak son resmi gun sayisi; 0 tum gecmis")
	forceAppend := fs.Bool("force-append", false, "var olan as_of/target/snapshot kaydini tekrar append et")
	replace := fs.Bool("replace", false, "mevcut ledger JSONL dosyasini bastan yaz")
	if err := fs.Parse(args); err != nil {
		return err
	}
	normalized := ohlcv.NormalizeSymbol(*symbol)
	if normalized == "" {
		return errors.New("forecast-walkforward: -symbol is required")
	}
	if strings.ToLower(strings.TrimSpace(*target)) != "close_t1" {
		return fmt.Errorf("forecast-walkforward: unsupported -target %q; only close_t1 is supported", *target)
	}
	from, to, err := forecastWalkforwardDateRange(*fromText, *toText)
	if err != nil {
		return err
	}
	provider := datasource.NewBISTBulletinDBProvider(bistBulletinDBPath(*dataDir))
	records, err := provider.FetchDailyBulletinRecordsRange(ctx, normalized, "", to.Format("2006-01-02"), *limit)
	if err != nil {
		return fmt.Errorf("fetch BIST bulletin records: %w", err)
	}
	entries, err := buildForecastLedgerEntries(ctx, normalized, from, to, records, newForecastAuditContextAwareForecastBuilder(*outDir))
	if err != nil {
		return err
	}
	path := strings.TrimSpace(*ledgerPath)
	if path == "" {
		path = defaultForecastLedgerPath(*outDir, normalized, from.Format("2006-01-02"))
	}
	written, skipped, err := appendForecastLedgerEntries(path, entries, *forceAppend, *replace)
	if err != nil {
		return err
	}
	fmt.Printf("forecast ledger updated for %s: %s (written=%d skipped=%d)\n", normalized, path, written, skipped)
	return nil
}

func runVerifyForecast(ctx context.Context, cfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	fs := flag.NewFlagSet("verify-forecast", flag.ExitOnError)
	symbol := fs.String("symbol", "ASELS", "BIST sembolu, ornek: ASELS")
	fromText := fs.String("from", "2026-06-01", "dogrulanacak target baslangic tarihi (YYYY-MM-DD)")
	toText := fs.String("to", "", "dogrulanacak target bitis tarihi; bos ise bugun")
	dataDir := fs.String("data", cfg.DataDir, "veri kok dizini")
	outDir := fs.String("out", store.Root(), "equities kok cikti klasoru")
	ledgerPath := fs.String("ledger", "", "forecast ledger JSONL yolu")
	verificationPath := fs.String("verification", "", "opsiyonel verification JSONL yolu")
	limit := fs.Int("limit", 0, "BIST bulteninden okunacak son resmi gun sayisi; 0 tum gecmis")
	forceAppend := fs.Bool("force-append", false, "var olan verification kaydini tekrar append et")
	replace := fs.Bool("replace", false, "mevcut verification JSONL dosyasini bastan yaz")
	if err := fs.Parse(args); err != nil {
		return err
	}
	normalized := ohlcv.NormalizeSymbol(*symbol)
	if normalized == "" {
		return errors.New("verify-forecast: -symbol is required")
	}
	from, to, err := forecastWalkforwardDateRange(*fromText, *toText)
	if err != nil {
		return err
	}
	ledger := strings.TrimSpace(*ledgerPath)
	if ledger == "" {
		ledger = defaultForecastLedgerPath(*outDir, normalized, from.Format("2006-01-02"))
	}
	entries, err := readForecastLedgerEntries(ledger)
	if err != nil {
		return err
	}
	provider := datasource.NewBISTBulletinDBProvider(bistBulletinDBPath(*dataDir))
	records, err := provider.FetchDailyBulletinRecordsRange(ctx, normalized, "", to.Format("2006-01-02"), *limit)
	if err != nil {
		return fmt.Errorf("fetch BIST bulletin records: %w", err)
	}
	events := buildForecastVerificationEvents(entries, records, from, to)
	out := strings.TrimSpace(*verificationPath)
	if out == "" {
		out = defaultForecastVerificationPath(ledger)
	}
	written, skipped, err := appendForecastVerificationEvents(out, events, *forceAppend, *replace)
	if err != nil {
		return err
	}
	fmt.Printf("forecast verification updated for %s: %s (written=%d skipped=%d)\n", normalized, out, written, skipped)
	return nil
}

func runForecastErrorAudit(ctx context.Context, cfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	_ = ctx
	_ = cfg
	fs := flag.NewFlagSet("forecast-error-audit", flag.ExitOnError)
	symbol := fs.String("symbol", "ASELS", "BIST sembolu, ornek: ASELS")
	fromText := fs.String("from", "2026-06-01", "audit baslangic target tarihi (YYYY-MM-DD)")
	toText := fs.String("to", "", "audit bitis target tarihi; bos ise bugun")
	outDir := fs.String("out", store.Root(), "equities kok cikti klasoru")
	ledgerPath := fs.String("ledger", "", "forecast ledger JSONL yolu")
	verificationPath := fs.String("verification", "", "verification JSONL yolu")
	reportPath := fs.String("report", "", "opsiyonel forecast_error_attribution.json yolu")
	threshold := fs.Float64("threshold-pct", 1.0, "hata atfi icin minimum mutlak kapanis hata yuzdesi")
	if err := fs.Parse(args); err != nil {
		return err
	}
	normalized := ohlcv.NormalizeSymbol(*symbol)
	if normalized == "" {
		return errors.New("forecast-error-audit: -symbol is required")
	}
	from, to, err := forecastWalkforwardDateRange(*fromText, *toText)
	if err != nil {
		return err
	}
	ledger := strings.TrimSpace(*ledgerPath)
	if ledger == "" {
		ledger = defaultForecastLedgerPath(*outDir, normalized, from.Format("2006-01-02"))
	}
	verification := strings.TrimSpace(*verificationPath)
	if verification == "" {
		verification = defaultForecastVerificationPath(ledger)
	}
	events, err := readForecastVerificationEvents(verification)
	if err != nil {
		return err
	}
	report := buildForecastErrorAuditReport(normalized, from, to, events, *threshold)
	target := strings.TrimSpace(*reportPath)
	if target == "" {
		target = filepath.Join(filepath.Dir(verification), "forecast_error_attribution.json")
	}
	if err := writeForecastAuditJSON(target, report); err != nil {
		return err
	}
	jsonlPath := strings.TrimSuffix(target, filepath.Ext(target)) + ".jsonl"
	jsonlEvents := append([]forecastErrorAttribution{}, report.Events...)
	jsonlEvents = append(jsonlEvents, report.ScenarioEvents...)
	if err := writeForecastErrorAttributionJSONL(jsonlPath, jsonlEvents); err != nil {
		return err
	}
	fmt.Printf("forecast error audit written for %s: %s\n", normalized, target)
	return nil
}

func runForecastCompareReport(ctx context.Context, cfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	_ = ctx
	_ = cfg
	fs := flag.NewFlagSet("forecast-compare-report", flag.ExitOnError)
	symbol := fs.String("symbol", "ASELS", "BIST sembolu, ornek: ASELS")
	fromText := fs.String("from", "2026-06-01", "rapor baslangic target tarihi (YYYY-MM-DD)")
	toText := fs.String("to", "", "rapor bitis target tarihi; bos ise bugun")
	outDir := fs.String("out", store.Root(), "equities kok cikti klasoru")
	ledgerPath := fs.String("ledger", "", "forecast ledger JSONL yolu")
	verificationPath := fs.String("verification", "", "verification JSONL yolu")
	reportPath := fs.String("report", "", "opsiyonel rapor cikti yolu")
	format := fs.String("format", "md", "rapor formati: md, csv veya json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	normalized := ohlcv.NormalizeSymbol(*symbol)
	if normalized == "" {
		return errors.New("forecast-compare-report: -symbol is required")
	}
	from, to, err := forecastWalkforwardDateRange(*fromText, *toText)
	if err != nil {
		return err
	}
	reportFormat, err := normalizeForecastCompareFormat(*format)
	if err != nil {
		return err
	}
	ledger := strings.TrimSpace(*ledgerPath)
	if ledger == "" {
		ledger = defaultForecastLedgerPath(*outDir, normalized, from.Format("2006-01-02"))
	}
	verification := strings.TrimSpace(*verificationPath)
	if verification == "" {
		verification = defaultForecastVerificationPath(ledger)
	}
	events, err := readForecastVerificationEvents(verification)
	if err != nil {
		return err
	}
	report := buildForecastActualVsAIReport(normalized, from, to, verification, events)
	target := strings.TrimSpace(*reportPath)
	if target == "" {
		target = defaultForecastCompareReportPath(verification, reportFormat)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	var raw string
	switch reportFormat {
	case "json":
		if err := writeForecastAuditJSON(target, report); err != nil {
			return err
		}
	case "csv":
		raw = forecastActualVsAICSV(report)
		if err := os.WriteFile(target, []byte(raw), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	default:
		raw = forecastActualVsAIMarkdown(report)
		if err := os.WriteFile(target, []byte(raw), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}
	fmt.Printf("forecast actual-vs-ai report written for %s: %s\n", normalized, target)
	fmt.Printf("summary: verified=%d published_point=%d scenario_only=%d band_only=%d interval_hits=%d interval_misses=%d decision_interval_hits=%d decision_interval_misses=%d\n",
		report.Summary.VerifiedEvents,
		report.Summary.PublishedPointEvents,
		report.Summary.ScenarioOnlyEvents,
		report.Summary.BandOnlyEvents,
		report.Summary.IntervalHits,
		report.Summary.IntervalMisses,
		report.Summary.DecisionIntervalHits,
		report.Summary.DecisionIntervalMisses,
	)
	return nil
}

func forecastWalkforwardDateRange(fromText, toText string) (time.Time, time.Time, error) {
	from, err := parseOptionalDate(fromText)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from.IsZero() {
		return time.Time{}, time.Time{}, errors.New("-from is required")
	}
	to := time.Now()
	if strings.TrimSpace(toText) != "" {
		to, err = parseOptionalDate(toText)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("-to %s is before -from %s", to.Format("2006-01-02"), from.Format("2006-01-02"))
	}
	return from, to, nil
}

func buildForecastLedgerEntries(ctx context.Context, symbol string, from, to time.Time, records []datasource.DailyBulletinRecord, builder forecastAuditRangeForecastBuilder) ([]forecastLedgerEntry, error) {
	if builder == nil {
		builder = forecastAuditTechnicalContextForecast
	}
	fromDate := from.Format("2006-01-02")
	toDate := to.Format("2006-01-02")
	out := []forecastLedgerEntry{}
	for actualIdx := 1; actualIdx < len(records); actualIdx++ {
		actual := records[actualIdx]
		if actual.TradingDate < fromDate || actual.TradingDate > toDate {
			continue
		}
		prefixRecords := append([]datasource.DailyBulletinRecord{}, records[:actualIdx]...)
		asOf := prefixRecords[len(prefixRecords)-1]
		if len(forecastAuditCandles(prefixRecords)) < 30 {
			continue
		}
		forecastResult, err := builder(ctx, symbol, asOf, actual.TradingDate, prefixRecords)
		if err != nil {
			return nil, err
		}
		if !forecastResult.Forecast.Computed || forecastResult.Forecast.PredictedClose <= 0 {
			continue
		}
		out = append(out, forecastLedgerEntryFromForecast(symbol, asOf, actual.TradingDate, prefixRecords, forecastResult))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("forecast-walkforward: no forecast rows for %s between %s and %s", symbol, fromDate, toDate)
	}
	return out, nil
}

func forecastLedgerEntryFromForecast(symbol string, asOf datasource.DailyBulletinRecord, targetDate string, prefixRecords []datasource.DailyBulletinRecord, forecastResult forecastAuditRangeForecastResult) forecastLedgerEntry {
	forecast := stripForecastAuditActual(forecastResult.Forecast)
	forecast.ForecastFor = targetDate
	low, high := forecastCloseInterval(forecast)
	publishable, publishStatus, suppressReason := forecastAuditForecastPublishState(forecast)
	scenarioPointAvailable, scenarioPointStatus, scenarioPointSuppressReason := forecastScenarioPointState(forecast, publishable)
	scenarioPredictedClose := 0.0
	scenarioPredictedCloseDirection := ""
	if scenarioPointAvailable {
		scenarioPredictedClose = forecast.PredictedClose
		scenarioPredictedCloseDirection = forecast.PredictedCloseDirection
	}
	predictedClose := forecast.PredictedClose
	predictedCloseDirection := forecast.PredictedCloseDirection
	publishedPredictedClose := forecast.PublishedPredictedClose
	if publishable {
		if (publishedPredictedClose == nil || *publishedPredictedClose <= 0) && predictedClose > 0 {
			publishedClose := predictedClose
			publishedPredictedClose = &publishedClose
		}
	}
	entry := forecastLedgerEntry{
		SchemaVersion:                   forecastLedgerSchemaVersion,
		Symbol:                          symbol,
		AsOfDate:                        asOf.TradingDate,
		TargetDate:                      targetDate,
		Target:                          "close_t1",
		GeneratedAt:                     time.Now().UTC().Format(time.RFC3339),
		Status:                          "forecasted",
		ForecastAgent:                   "forecast_agent_v1",
		VerifierAgent:                   "verifier_agent_v1",
		DataPolicy:                      "point_in_time_as_of_close_only",
		ModelVersion:                    "next_session_forecast_v1+bist_bulletin_overlay",
		Model:                           forecast.Model,
		ForecastContext:                 forecastResult.Context,
		LastClose:                       forecast.LastClose,
		PredictedClose:                  predictedClose,
		ScenarioPredictedClose:          scenarioPredictedClose,
		RawPredictedClose:               forecast.RawPredictedClose,
		PublishedPredictedClose:         publishedPredictedClose,
		IntervalLow:                     low,
		IntervalHigh:                    high,
		DecisionIntervalLow:             forecast.DecisionIntervalLow,
		DecisionIntervalHigh:            forecast.DecisionIntervalHigh,
		DecisionIntervalWidthPct:        forecast.DecisionIntervalWidthPct,
		DecisionIntervalStatus:          forecast.DecisionIntervalStatus,
		DecisionIntervalReason:          forecast.DecisionIntervalReason,
		Confidence:                      forecast.Confidence,
		ConfidenceLabel:                 forecast.ConfidenceLabel,
		PredictedCloseDirection:         predictedCloseDirection,
		ScenarioPredictedCloseDirection: scenarioPredictedCloseDirection,
		ScenarioPointAvailable:          scenarioPointAvailable,
		ScenarioPointStatus:             scenarioPointStatus,
		ScenarioPointSuppressionReason:  scenarioPointSuppressReason,
		PointForecastPublishable:        publishable,
		PointForecastStatus:             publishStatus,
		PointForecastSuppressionReason:  suppressReason,
		TradeSignalAllowed:              forecast.DecisionForecast.TradeSignalAllowed,
		DecisionConfidence:              forecast.DecisionForecast.Confidence,
		ExpectedIntradayDirection:       forecast.DecisionForecast.ExpectedIntradayDirection,
		VolatilityRegime:                forecast.DecisionForecast.VolatilityRegime,
		BacktestSamples:                 forecast.BacktestMetrics.Samples,
		BacktestCloseMAPE:               forecast.BacktestMetrics.CloseMAPE,
		BacktestDirectionAccuracy:       forecast.BacktestMetrics.DirectionAccuracy,
		DirectionModelUnreliable:        forecast.DirectionModelUnreliable || forecast.DecisionForecast.DirectionModelUnreliable,
		RecordsThroughAsOf:              len(prefixRecords),
		AsOfSourcePath:                  asOf.SourcePath,
		OnlyUsesDataThroughAsOf:         true,
		ActualUsedOnlyForVerification:   true,
		FeatureScopes:                   forecastFeatureScopes(forecastResult.Context),
		Warnings:                        append(append([]string{}, forecast.Warnings...), forecastResult.Warnings...),
		RawForecast:                     forecast,
	}
	if len(prefixRecords) > 0 {
		entry.FirstForecastRecordDate = prefixRecords[0].TradingDate
		entry.LastForecastRecordDate = prefixRecords[len(prefixRecords)-1].TradingDate
	}
	entry.DataSnapshotHash = forecastLedgerSnapshotHash(symbol, asOf.TradingDate, targetDate, prefixRecords)
	entry.EntryID = forecastLedgerEntryID(entry)
	return entry
}

func forecastCloseInterval(f analysis.NextSessionForecast) (float64, float64) {
	low := firstNonZeroForecastValue(f.DecisionForecast.CloseRangeLow, f.CloseP10, f.TradableExpectedLow, f.ExpectedLow, f.RawExpectedLow)
	high := firstNonZeroForecastValue(f.DecisionForecast.CloseRangeHigh, f.CloseP90, f.TradableExpectedHigh, f.ExpectedHigh, f.RawExpectedHigh)
	if low > 0 && high > 0 && low > high {
		low, high = high, low
	}
	return low, high
}

func firstNonZeroForecastValue(values ...float64) float64 {
	for _, value := range values {
		if value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0) {
			return value
		}
	}
	return 0
}

func forecastScenarioPointState(f analysis.NextSessionForecast, pointPublishable bool) (bool, string, string) {
	if !f.Computed || f.PredictedClose <= 0 {
		return false, "not_available", "scenario_point_not_computed"
	}
	if pointPublishable {
		return true, "published_point", ""
	}
	metrics := f.BacktestMetrics
	status := "research_point"
	reason := ""
	if metrics.Samples < nextSessionScenarioPointMinBacktestSamples {
		reason = fmt.Sprintf("scenario_point_backtest_samples_below_%d:%d", nextSessionScenarioPointMinBacktestSamples, metrics.Samples)
		return false, "band_only_validation_failed", reason
	} else if metrics.CloseMAPE <= 0 {
		reason = "scenario_point_backtest_close_mape_missing"
		return false, "band_only_validation_failed", reason
	} else if metrics.CloseMAPE > nextSessionScenarioPointMaxCloseMAPEPct {
		reason = fmt.Sprintf("scenario_point_close_mape_above_%.2fpct:%.2f", nextSessionScenarioPointMaxCloseMAPEPct, metrics.CloseMAPE)
		return false, "band_only_validation_failed", reason
	} else if metrics.DirectionAccuracy > 0 && metrics.DirectionAccuracy < nextSessionScenarioPointMinDirectionHitPct {
		reason = fmt.Sprintf("scenario_point_direction_hit_below_%.0fpct:%.2f", nextSessionScenarioPointMinDirectionHitPct, metrics.DirectionAccuracy)
		return false, "band_only_validation_failed", reason
	} else if f.DirectionModelUnreliable || f.DecisionForecast.DirectionModelUnreliable {
		reason = "scenario_point_direction_model_unreliable"
		return false, "band_only_validation_failed", reason
	}
	return true, status, reason
}

func forecastFeatureScopes(context string) []string {
	scopes := []string{"ohlcv", "indicators", "next_session_forecast", "bist_official_bulletin", "quant_backtest"}
	if strings.Contains(strings.ToLower(context), "kap") {
		scopes = append(scopes, "kap_scope")
	}
	return scopes
}

func forecastLedgerSnapshotHash(symbol, asOfDate, targetDate string, records []datasource.DailyBulletinRecord) string {
	type recordDigest struct {
		Date       string  `json:"date"`
		Open       float64 `json:"open,omitempty"`
		High       float64 `json:"high,omitempty"`
		Low        float64 `json:"low,omitempty"`
		Close      float64 `json:"close,omitempty"`
		Volume     float64 `json:"volume,omitempty"`
		Value      float64 `json:"value_traded,omitempty"`
		SourcePath string  `json:"source_path,omitempty"`
	}
	payload := struct {
		Symbol     string         `json:"symbol"`
		AsOfDate   string         `json:"as_of_date"`
		TargetDate string         `json:"target_date"`
		Records    []recordDigest `json:"records"`
	}{Symbol: symbol, AsOfDate: asOfDate, TargetDate: targetDate}
	for _, record := range records {
		payload.Records = append(payload.Records, recordDigest{
			Date:       record.TradingDate,
			Open:       record.Open,
			High:       record.High,
			Low:        record.Low,
			Close:      record.Close,
			Volume:     record.Volume,
			Value:      record.ValueTraded,
			SourcePath: record.SourcePath,
		})
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func forecastLedgerEntryID(entry forecastLedgerEntry) string {
	raw := strings.Join([]string{
		entry.Symbol,
		entry.AsOfDate,
		entry.TargetDate,
		entry.Target,
		entry.DataSnapshotHash,
		fmt.Sprintf("%.4f", firstNonZeroForecastValue(entry.ScenarioPredictedClose, entry.PredictedClose)),
		entry.ModelVersion,
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func defaultForecastLedgerPath(outDir, symbol, fromDate string) string {
	return filepath.Join(outDir, symbol, "analysis", "forecast_ledger", fmt.Sprintf("%s_%s_forward.jsonl", symbol, fromDate))
}

func defaultForecastVerificationPath(ledgerPath string) string {
	return strings.TrimSuffix(ledgerPath, filepath.Ext(ledgerPath)) + "_verification.jsonl"
}

func defaultForecastCompareReportPath(verificationPath, format string) string {
	ext := ".md"
	switch format {
	case "csv":
		ext = ".csv"
	case "json":
		ext = ".json"
	}
	return filepath.Join(filepath.Dir(verificationPath), "forecast_actual_vs_ai"+ext)
}

func normalizeForecastCompareFormat(format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return "md", nil
	}
	switch format {
	case "md", "markdown":
		return "md", nil
	case "csv":
		return "csv", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("forecast-compare-report: unsupported -format %q; expected md, csv or json", format)
	}
}

func appendForecastLedgerEntries(path string, entries []forecastLedgerEntry, force bool, replace bool) (int, int, error) {
	existing, err := readForecastLedgerEntryIDs(path)
	if err != nil {
		return 0, 0, err
	}
	if replace {
		existing = map[string]bool{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, 0, err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if replace {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	written, skipped := 0, 0
	enc := json.NewEncoder(file)
	for _, entry := range entries {
		if !force && existing[entry.EntryID] {
			skipped++
			continue
		}
		if err := enc.Encode(entry); err != nil {
			return written, skipped, err
		}
		existing[entry.EntryID] = true
		written++
	}
	return written, skipped, nil
}

func readForecastLedgerEntryIDs(path string) (map[string]bool, error) {
	entries, err := readForecastLedgerEntriesAllowMissing(path)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, entry := range entries {
		out[entry.EntryID] = true
	}
	return out, nil
}

func readForecastLedgerEntries(path string) ([]forecastLedgerEntry, error) {
	entries, err := readForecastLedgerEntriesAllowMissing(path)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("forecast ledger is empty or missing: %s", path)
	}
	return entries, nil
}

func readForecastLedgerEntriesAllowMissing(path string) ([]forecastLedgerEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	out := []forecastLedgerEntry{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry forecastLedgerEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		if entry.EntryID == "" {
			entry.EntryID = forecastLedgerEntryID(entry)
		}
		out = append(out, entry)
	}
	return out, scanner.Err()
}

func buildForecastVerificationEvents(entries []forecastLedgerEntry, records []datasource.DailyBulletinRecord, from, to time.Time) []forecastVerificationEvent {
	recordsByDate := map[string]datasource.DailyBulletinRecord{}
	for _, record := range records {
		recordsByDate[record.TradingDate] = record
	}
	fromDate := from.Format("2006-01-02")
	toDate := to.Format("2006-01-02")
	out := []forecastVerificationEvent{}
	for _, entry := range entries {
		if entry.TargetDate < fromDate || entry.TargetDate > toDate {
			continue
		}
		scenarioPredictedClose := 0.0
		if entry.ScenarioPointAvailable {
			scenarioPredictedClose = entry.ScenarioPredictedClose
		}
		verificationMode := "published_point"
		if !entry.PointForecastPublishable || entry.PredictedClose <= 0 {
			verificationMode = "scenario_only"
			if !entry.ScenarioPointAvailable || scenarioPredictedClose <= 0 {
				verificationMode = "band_only"
			}
		}
		event := forecastVerificationEvent{
			SchemaVersion:                  forecastLedgerSchemaVersion,
			EntryID:                        entry.EntryID,
			Symbol:                         entry.Symbol,
			AsOfDate:                       entry.AsOfDate,
			TargetDate:                     entry.TargetDate,
			VerifiedAt:                     time.Now().UTC().Format(time.RFC3339),
			Status:                         "pending_verification",
			VerificationMode:               verificationMode,
			VerifierAgent:                  "verifier_agent_v1",
			PredictedClose:                 entry.PredictedClose,
			ScenarioPredictedClose:         scenarioPredictedClose,
			LastClose:                      entry.LastClose,
			IntervalLow:                    entry.IntervalLow,
			IntervalHigh:                   entry.IntervalHigh,
			DecisionIntervalLow:            entry.DecisionIntervalLow,
			DecisionIntervalHigh:           entry.DecisionIntervalHigh,
			DecisionIntervalWidthPct:       entry.DecisionIntervalWidthPct,
			DecisionIntervalStatus:         entry.DecisionIntervalStatus,
			DecisionIntervalReason:         entry.DecisionIntervalReason,
			DataSnapshotHash:               entry.DataSnapshotHash,
			Model:                          entry.Model,
			ForecastContext:                entry.ForecastContext,
			PointForecastPublishable:       entry.PointForecastPublishable,
			ScenarioPointAvailable:         entry.ScenarioPointAvailable,
			ScenarioPointStatus:            entry.ScenarioPointStatus,
			ScenarioPointSuppressionReason: entry.ScenarioPointSuppressionReason,
			SuppressionReason:              entry.PointForecastSuppressionReason,
			Confidence:                     entry.Confidence,
			BacktestCloseMAPE:              entry.BacktestCloseMAPE,
			BacktestDirectionAccuracy:      entry.BacktestDirectionAccuracy,
			Warnings:                       append([]string{}, entry.Warnings...),
		}
		actual, ok := recordsByDate[entry.TargetDate]
		if ok && actual.Close > 0 {
			event.Status = "verified"
			event.ActualClose = actual.Close
			event.OfficialCloseSource = "bist_thb_official_bulletin"
			event.OfficialCloseSourcePath = actual.SourcePath
			actualCloseKurus := forecastAuditPriceKurus(actual.Close)
			if scenarioPredictedClose > 0 {
				scenarioErrorKurus := actualCloseKurus - forecastAuditPriceKurus(scenarioPredictedClose)
				event.ScenarioCloseErrorTL = forecastAuditKurusTL(scenarioErrorKurus)
				event.ScenarioCloseAbsErrorPct = forecastAuditAbsErrorPctFromKurus(scenarioErrorKurus, actualCloseKurus)
				scenarioDirectionHit := forecastAuditDirectionHit(scenarioPredictedClose, entry.LastClose, actual.Close)
				event.ScenarioCloseDirectionHit = &scenarioDirectionHit
				event.CloseErrorTL = event.ScenarioCloseErrorTL
				event.CloseAbsErrorPct = event.ScenarioCloseAbsErrorPct
				event.CloseDirectionHit = &scenarioDirectionHit
			}
			if entry.PointForecastPublishable && entry.PredictedClose > 0 {
				errorKurus := actualCloseKurus - forecastAuditPriceKurus(entry.PredictedClose)
				event.CloseErrorTL = forecastAuditKurusTL(errorKurus)
				event.CloseAbsErrorPct = forecastAuditAbsErrorPctFromKurus(errorKurus, actualCloseKurus)
				directionHit := forecastAuditDirectionHit(entry.PredictedClose, entry.LastClose, actual.Close)
				event.CloseDirectionHit = &directionHit
			} else if scenarioPredictedClose > 0 {
				event.Status = "verified_scenario_only"
			} else {
				event.Status = "verified_band_only"
			}
			intervalHit := forecastIntervalHit(actual.Close, entry.IntervalLow, entry.IntervalHigh)
			event.IntervalHit = &intervalHit
			if entry.DecisionIntervalLow > 0 && entry.DecisionIntervalHigh > 0 {
				decisionIntervalHit := forecastIntervalHit(actual.Close, entry.DecisionIntervalLow, entry.DecisionIntervalHigh)
				event.DecisionIntervalHit = &decisionIntervalHit
			}
		}
		event.VerificationID = forecastVerificationID(event)
		out = append(out, event)
	}
	return out
}

func forecastIntervalHit(actual, low, high float64) bool {
	if actual <= 0 || low <= 0 || high <= 0 {
		return false
	}
	if low > high {
		low, high = high, low
	}
	return actual >= low && actual <= high
}

func forecastIntervalWidthPct(low, high, base float64) float64 {
	if low <= 0 || high <= 0 || base <= 0 {
		return 0
	}
	if low > high {
		low, high = high, low
	}
	return roundAuditMetric(100 * (high - low) / base)
}

func forecastVerificationID(event forecastVerificationEvent) string {
	raw := strings.Join([]string{event.EntryID, event.Symbol, event.AsOfDate, event.TargetDate, event.Status}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func appendForecastVerificationEvents(path string, events []forecastVerificationEvent, force bool, replace bool) (int, int, error) {
	existing, err := readForecastVerificationIDs(path)
	if err != nil {
		return 0, 0, err
	}
	if replace {
		existing = map[string]bool{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, 0, err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if replace {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	written, skipped := 0, 0
	enc := json.NewEncoder(file)
	for _, event := range events {
		if !force && existing[event.VerificationID] {
			skipped++
			continue
		}
		if err := enc.Encode(event); err != nil {
			return written, skipped, err
		}
		existing[event.VerificationID] = true
		written++
	}
	return written, skipped, nil
}

func readForecastVerificationIDs(path string) (map[string]bool, error) {
	events, err := readForecastVerificationEventsAllowMissing(path)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, event := range events {
		out[event.VerificationID] = true
	}
	return out, nil
}

func readForecastVerificationEvents(path string) ([]forecastVerificationEvent, error) {
	events, err := readForecastVerificationEventsAllowMissing(path)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("forecast verification is empty or missing: %s", path)
	}
	return events, nil
}

func readForecastVerificationEventsAllowMissing(path string) ([]forecastVerificationEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	out := []forecastVerificationEvent{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event forecastVerificationEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		if event.VerificationID == "" {
			event.VerificationID = forecastVerificationID(event)
		}
		out = append(out, event)
	}
	return out, scanner.Err()
}

func buildForecastErrorAuditReport(symbol string, from, to time.Time, events []forecastVerificationEvent, thresholdPct float64) forecastErrorAuditReport {
	if thresholdPct <= 0 {
		thresholdPct = 1
	}
	fromDate := from.Format("2006-01-02")
	toDate := to.Format("2006-01-02")
	report := forecastErrorAuditReport{
		SchemaVersion: forecastLedgerSchemaVersion,
		Symbol:        symbol,
		FromDate:      fromDate,
		ToDate:        toDate,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		ThresholdPct:  thresholdPct,
	}
	publishedAbsTotal := 0.0
	scenarioAbsTotal := 0.0
	for _, event := range events {
		if event.Symbol != symbol || event.TargetDate < fromDate || event.TargetDate > toDate || !strings.HasPrefix(event.Status, "verified") {
			continue
		}
		errorPct := forecastVerificationAuditErrorPct(event)
		report.Summary.VerifiedEvents++
		if forecastVerificationIsBandOnly(event) {
			report.Summary.BandOnlyEvents++
			if event.IntervalHit != nil && !*event.IntervalHit {
				report.Summary.ScenarioIntervalMisses++
				report.ScenarioEvents = append(report.ScenarioEvents, forecastErrorAttributionFromEvent(event))
			}
		} else if forecastVerificationIsScenarioOnly(event) {
			report.Summary.ScenarioOnlyEvents++
			scenarioAbsTotal += errorPct
			if errorPct > report.Summary.ScenarioMaxAbsErrorPct {
				report.Summary.ScenarioMaxAbsErrorPct = errorPct
			}
			if event.CloseDirectionHit != nil && !*event.CloseDirectionHit {
				report.Summary.ScenarioDirectionMisses++
			}
			if event.IntervalHit != nil && !*event.IntervalHit {
				report.Summary.ScenarioIntervalMisses++
			}
			if errorPct >= thresholdPct {
				report.ScenarioEvents = append(report.ScenarioEvents, forecastErrorAttributionFromEvent(event))
			}
		} else {
			report.Summary.PublishedEvents++
			publishedAbsTotal += errorPct
			if errorPct > report.Summary.MaxAbsErrorPct {
				report.Summary.MaxAbsErrorPct = errorPct
			}
			if event.CloseDirectionHit != nil && !*event.CloseDirectionHit {
				report.Summary.DirectionMisses++
			}
			if event.IntervalHit != nil && !*event.IntervalHit {
				report.Summary.IntervalMisses++
			}
			if errorPct >= thresholdPct {
				report.Events = append(report.Events, forecastErrorAttributionFromEvent(event))
			}
		}
	}
	report.Summary.ErrorEvents = len(report.Events)
	report.Summary.ScenarioErrorEvents = len(report.ScenarioEvents)
	if report.Summary.PublishedEvents > 0 {
		report.Summary.AverageAbsErrorPct = roundAuditMetric(publishedAbsTotal / float64(report.Summary.PublishedEvents))
	}
	if report.Summary.ScenarioOnlyEvents > 0 {
		report.Summary.ScenarioAverageAbsErrorPct = roundAuditMetric(scenarioAbsTotal / float64(report.Summary.ScenarioOnlyEvents))
	}
	report.Summary.MaxAbsErrorPct = roundAuditMetric(report.Summary.MaxAbsErrorPct)
	report.Summary.ScenarioMaxAbsErrorPct = roundAuditMetric(report.Summary.ScenarioMaxAbsErrorPct)
	if len(report.Events) > 0 {
		report.Summary.PrimaryRecommendation = report.Events[0].RecommendedFix
	} else if len(report.ScenarioEvents) > 0 {
		report.Summary.PrimaryRecommendation = report.ScenarioEvents[0].RecommendedFix
	}
	return report
}

func forecastVerificationIsScenarioOnly(event forecastVerificationEvent) bool {
	mode := strings.TrimSpace(event.VerificationMode)
	if mode != "" {
		return mode == "scenario_only"
	}
	return !event.PointForecastPublishable && event.ScenarioPointAvailable && event.ScenarioPredictedClose > 0
}

func forecastVerificationIsBandOnly(event forecastVerificationEvent) bool {
	mode := strings.TrimSpace(event.VerificationMode)
	if mode != "" {
		return mode == "band_only"
	}
	return !event.PointForecastPublishable && (!event.ScenarioPointAvailable || event.ScenarioPredictedClose <= 0)
}

func forecastVerificationAuditErrorPct(event forecastVerificationEvent) float64 {
	if forecastVerificationIsBandOnly(event) {
		return 0
	}
	if event.VerificationMode == "published_point" || (event.PointForecastPublishable && event.PredictedClose > 0) {
		return event.CloseAbsErrorPct
	}
	return event.ScenarioCloseAbsErrorPct
}

func buildForecastActualVsAIReport(symbol string, from, to time.Time, verificationPath string, events []forecastVerificationEvent) forecastActualVsAIReport {
	fromDate := from.Format("2006-01-02")
	toDate := to.Format("2006-01-02")
	report := forecastActualVsAIReport{
		SchemaVersion:          forecastLedgerSchemaVersion,
		Symbol:                 symbol,
		FromDate:               fromDate,
		ToDate:                 toDate,
		GeneratedAt:            time.Now().UTC().Format(time.RFC3339),
		SourceVerificationPath: verificationPath,
	}
	publishedAbsTotal := 0.0
	publishedAbsCount := 0
	scenarioAbsTotal := 0.0
	scenarioAbsCount := 0
	riskWidthTotal := 0.0
	riskWidthCount := 0
	decisionWidthTotal := 0.0
	decisionWidthCount := 0
	for _, event := range events {
		if event.Symbol != symbol || event.TargetDate < fromDate || event.TargetDate > toDate {
			continue
		}
		mode := forecastVerificationEffectiveMode(event)
		row := forecastActualVsAIRow{
			AsOfDate:                       event.AsOfDate,
			TargetDate:                     event.TargetDate,
			Status:                         event.Status,
			VerificationMode:               mode,
			LastClose:                      event.LastClose,
			IntervalLow:                    event.IntervalLow,
			IntervalHigh:                   event.IntervalHigh,
			IntervalHit:                    copyForecastBoolPtr(event.IntervalHit),
			DecisionIntervalLow:            event.DecisionIntervalLow,
			DecisionIntervalHigh:           event.DecisionIntervalHigh,
			DecisionIntervalHit:            copyForecastBoolPtr(event.DecisionIntervalHit),
			DecisionIntervalWidthPct:       event.DecisionIntervalWidthPct,
			DecisionIntervalStatus:         event.DecisionIntervalStatus,
			DecisionIntervalReason:         event.DecisionIntervalReason,
			RiskIntervalWidthPct:           forecastIntervalWidthPct(event.IntervalLow, event.IntervalHigh, event.LastClose),
			SuppressionReason:              event.SuppressionReason,
			ScenarioPointSuppressionReason: event.ScenarioPointSuppressionReason,
			OfficialCloseSource:            event.OfficialCloseSource,
		}
		if strings.HasPrefix(event.Status, "verified") && event.ActualClose > 0 {
			row.ActualClose = forecastFloatPtr(event.ActualClose)
			report.Summary.VerifiedEvents++
		} else {
			report.Summary.PendingEvents++
		}
		switch mode {
		case "published_point":
			report.Summary.PublishedPointEvents++
			if event.PredictedClose > 0 {
				row.AIReportedClose = forecastFloatPtr(event.PredictedClose)
				row.PublishedPredictedClose = forecastFloatPtr(event.PredictedClose)
			}
			if event.CloseAbsErrorPct > 0 || row.ActualClose != nil {
				row.PublishedCloseAbsErrorPct = forecastFloatPtr(event.CloseAbsErrorPct)
				publishedAbsTotal += event.CloseAbsErrorPct
				publishedAbsCount++
				if event.CloseAbsErrorPct > report.Summary.MaxPublishedAbsErrorPct {
					report.Summary.MaxPublishedAbsErrorPct = event.CloseAbsErrorPct
				}
			}
			if event.CloseDirectionHit != nil {
				row.CloseDirectionHit = copyForecastBoolPtr(event.CloseDirectionHit)
				if *event.CloseDirectionHit {
					report.Summary.PublishedDirectionHits++
				} else {
					report.Summary.PublishedDirectionMisses++
				}
			}
		case "scenario_only":
			report.Summary.ScenarioOnlyEvents++
			if event.ScenarioPredictedClose > 0 {
				row.AIReportedClose = forecastFloatPtr(event.ScenarioPredictedClose)
				row.ScenarioPredictedClose = forecastFloatPtr(event.ScenarioPredictedClose)
			}
			if event.ScenarioCloseAbsErrorPct > 0 || row.ActualClose != nil {
				row.ScenarioOnlyCloseAbsErrorPct = forecastFloatPtr(event.ScenarioCloseAbsErrorPct)
				scenarioAbsTotal += event.ScenarioCloseAbsErrorPct
				scenarioAbsCount++
				if event.ScenarioCloseAbsErrorPct > report.Summary.MaxScenarioOnlyAbsErrorPct {
					report.Summary.MaxScenarioOnlyAbsErrorPct = event.ScenarioCloseAbsErrorPct
				}
			}
			if event.ScenarioCloseDirectionHit != nil {
				row.CloseDirectionHit = copyForecastBoolPtr(event.ScenarioCloseDirectionHit)
				if *event.ScenarioCloseDirectionHit {
					report.Summary.ScenarioOnlyDirectionHits++
				} else {
					report.Summary.ScenarioOnlyDirectionMisses++
				}
			}
		case "band_only":
			report.Summary.BandOnlyEvents++
			report.Summary.RowsWithoutPointClosePrediction++
		}
		if event.IntervalHit != nil {
			if *event.IntervalHit {
				report.Summary.IntervalHits++
			} else {
				report.Summary.IntervalMisses++
			}
		}
		if row.RiskIntervalWidthPct > 0 {
			riskWidthTotal += row.RiskIntervalWidthPct
			riskWidthCount++
		}
		if event.DecisionIntervalHit != nil {
			report.Summary.DecisionIntervalEvents++
			if *event.DecisionIntervalHit {
				report.Summary.DecisionIntervalHits++
			} else {
				report.Summary.DecisionIntervalMisses++
			}
		}
		if row.DecisionIntervalWidthPct > 0 {
			decisionWidthTotal += row.DecisionIntervalWidthPct
			decisionWidthCount++
		}
		report.Summary.TotalEvents++
		report.Rows = append(report.Rows, row)
	}
	if publishedAbsCount > 0 {
		report.Summary.AveragePublishedAbsErrorPct = roundAuditMetric(publishedAbsTotal / float64(publishedAbsCount))
	}
	if scenarioAbsCount > 0 {
		report.Summary.AverageScenarioOnlyAbsErrorPct = roundAuditMetric(scenarioAbsTotal / float64(scenarioAbsCount))
	}
	if riskWidthCount > 0 {
		report.Summary.AverageRiskIntervalWidthPct = roundAuditMetric(riskWidthTotal / float64(riskWidthCount))
	}
	if decisionWidthCount > 0 {
		report.Summary.AverageDecisionIntervalWidthPct = roundAuditMetric(decisionWidthTotal / float64(decisionWidthCount))
	}
	report.Summary.MaxPublishedAbsErrorPct = roundAuditMetric(report.Summary.MaxPublishedAbsErrorPct)
	report.Summary.MaxScenarioOnlyAbsErrorPct = roundAuditMetric(report.Summary.MaxScenarioOnlyAbsErrorPct)
	sort.SliceStable(report.Rows, func(i, j int) bool {
		if report.Rows[i].TargetDate == report.Rows[j].TargetDate {
			return report.Rows[i].AsOfDate < report.Rows[j].AsOfDate
		}
		return report.Rows[i].TargetDate < report.Rows[j].TargetDate
	})
	return report
}

func forecastVerificationEffectiveMode(event forecastVerificationEvent) string {
	if forecastVerificationIsBandOnly(event) {
		return "band_only"
	}
	if forecastVerificationIsScenarioOnly(event) {
		return "scenario_only"
	}
	if strings.TrimSpace(event.VerificationMode) != "" {
		return event.VerificationMode
	}
	if event.PredictedClose > 0 {
		return "published_point"
	}
	return "unknown"
}

func forecastFloatPtr(value float64) *float64 {
	out := value
	return &out
}

func copyForecastBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func forecastActualVsAIMarkdown(report forecastActualVsAIReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s Actual vs AI Forecast Report\n\n", report.Symbol)
	fmt.Fprintf(&b, "- Period: %s to %s\n", report.FromDate, report.ToDate)
	fmt.Fprintf(&b, "- Generated at: %s\n", report.GeneratedAt)
	if report.SourceVerificationPath != "" {
		fmt.Fprintf(&b, "- Verification source: `%s`\n", report.SourceVerificationPath)
	}
	fmt.Fprintf(&b, "- Verified rows: %d\n", report.Summary.VerifiedEvents)
	fmt.Fprintf(&b, "- Published point rows: %d\n", report.Summary.PublishedPointEvents)
	fmt.Fprintf(&b, "- Scenario-only rows: %d\n", report.Summary.ScenarioOnlyEvents)
	fmt.Fprintf(&b, "- Band-only rows: %d\n", report.Summary.BandOnlyEvents)
	fmt.Fprintf(&b, "- Interval hits/misses: %d/%d\n", report.Summary.IntervalHits, report.Summary.IntervalMisses)
	if report.Summary.AverageRiskIntervalWidthPct > 0 {
		fmt.Fprintf(&b, "- Avg risk band width pct: %.2f\n", report.Summary.AverageRiskIntervalWidthPct)
	}
	if report.Summary.DecisionIntervalEvents > 0 {
		fmt.Fprintf(&b, "- Decision interval hits/misses: %d/%d\n", report.Summary.DecisionIntervalHits, report.Summary.DecisionIntervalMisses)
		fmt.Fprintf(&b, "- Avg decision band width pct: %.2f\n", report.Summary.AverageDecisionIntervalWidthPct)
	}
	if report.Summary.PublishedPointEvents > 0 {
		fmt.Fprintf(&b, "- Published point avg/max abs error pct: %.2f/%.2f\n", report.Summary.AveragePublishedAbsErrorPct, report.Summary.MaxPublishedAbsErrorPct)
	}
	if report.Summary.ScenarioOnlyEvents > 0 {
		fmt.Fprintf(&b, "- Scenario-only avg/max abs error pct: %.2f/%.2f\n", report.Summary.AverageScenarioOnlyAbsErrorPct, report.Summary.MaxScenarioOnlyAbsErrorPct)
	}
	if report.Summary.RowsWithoutPointClosePrediction > 0 {
		fmt.Fprintf(&b, "- Rows without point close prediction: %d\n", report.Summary.RowsWithoutPointClosePrediction)
	}
	b.WriteString("\n")
	b.WriteString("| Target date | As of | Mode | Last close | Actual close | AI close | Published close | Scenario close | Decision band | Decision hit | Risk band | Risk hit | Abs error pct | Reason |\n")
	b.WriteString("|---|---|---|---:|---:|---:|---:|---:|---|---|---|---|---:|---|\n")
	for _, row := range report.Rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			forecastMarkdownEscape(row.TargetDate),
			forecastMarkdownEscape(row.AsOfDate),
			forecastMarkdownEscape(row.VerificationMode),
			forecastFormatPositiveFloat(row.LastClose),
			forecastFormatOptionalFloat(row.ActualClose),
			forecastFormatOptionalFloat(row.AIReportedClose),
			forecastFormatOptionalFloat(row.PublishedPredictedClose),
			forecastFormatOptionalFloat(row.ScenarioPredictedClose),
			forecastMarkdownEscape(forecastFormatBand(row.DecisionIntervalLow, row.DecisionIntervalHigh)),
			forecastFormatOptionalBool(row.DecisionIntervalHit),
			forecastMarkdownEscape(forecastFormatBand(row.IntervalLow, row.IntervalHigh)),
			forecastFormatOptionalBool(row.IntervalHit),
			forecastFormatOptionalFloat(forecastCompareRowErrorPct(row)),
			forecastMarkdownEscape(forecastCompareRowReason(row)),
		)
	}
	return b.String()
}

func forecastActualVsAICSV(report forecastActualVsAIReport) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{
		"target_date",
		"as_of_date",
		"status",
		"verification_mode",
		"last_close",
		"actual_close",
		"ai_close",
		"published_predicted_close",
		"scenario_predicted_close",
		"decision_interval_low",
		"decision_interval_high",
		"decision_interval_hit",
		"decision_interval_width_pct",
		"decision_interval_status",
		"decision_interval_reason",
		"interval_low",
		"interval_high",
		"interval_hit",
		"risk_interval_width_pct",
		"abs_error_pct",
		"close_direction_hit",
		"suppression_reason",
		"scenario_point_suppression_reason",
		"official_close_source",
	})
	for _, row := range report.Rows {
		_ = w.Write([]string{
			row.TargetDate,
			row.AsOfDate,
			row.Status,
			row.VerificationMode,
			forecastFormatCSVFloat(row.LastClose),
			forecastFormatOptionalCSVFloat(row.ActualClose),
			forecastFormatOptionalCSVFloat(row.AIReportedClose),
			forecastFormatOptionalCSVFloat(row.PublishedPredictedClose),
			forecastFormatOptionalCSVFloat(row.ScenarioPredictedClose),
			forecastFormatCSVFloat(row.DecisionIntervalLow),
			forecastFormatCSVFloat(row.DecisionIntervalHigh),
			forecastFormatOptionalBool(row.DecisionIntervalHit),
			forecastFormatCSVFloat(row.DecisionIntervalWidthPct),
			row.DecisionIntervalStatus,
			row.DecisionIntervalReason,
			forecastFormatCSVFloat(row.IntervalLow),
			forecastFormatCSVFloat(row.IntervalHigh),
			forecastFormatOptionalBool(row.IntervalHit),
			forecastFormatCSVFloat(row.RiskIntervalWidthPct),
			forecastFormatOptionalCSVFloat(forecastCompareRowErrorPct(row)),
			forecastFormatOptionalBool(row.CloseDirectionHit),
			row.SuppressionReason,
			row.ScenarioPointSuppressionReason,
			row.OfficialCloseSource,
		})
	}
	w.Flush()
	return b.String()
}

func forecastCompareRowErrorPct(row forecastActualVsAIRow) *float64 {
	if row.PublishedCloseAbsErrorPct != nil {
		return row.PublishedCloseAbsErrorPct
	}
	return row.ScenarioOnlyCloseAbsErrorPct
}

func forecastCompareRowReason(row forecastActualVsAIRow) string {
	if strings.TrimSpace(row.ScenarioPointSuppressionReason) != "" {
		return row.ScenarioPointSuppressionReason
	}
	if strings.TrimSpace(row.SuppressionReason) != "" {
		return row.SuppressionReason
	}
	if row.VerificationMode == "band_only" {
		return "point_close_prediction_not_available"
	}
	return ""
}

func forecastFormatBand(low, high float64) string {
	if low <= 0 || high <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f-%.2f", low, high)
}

func forecastFormatPositiveFloat(value float64) string {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "-"
	}
	return fmt.Sprintf("%.2f", value)
}

func forecastFormatOptionalFloat(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *value)
}

func forecastFormatCSVFloat(value float64) string {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return ""
	}
	return fmt.Sprintf("%.6f", value)
}

func forecastFormatOptionalCSVFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.6f", *value)
}

func forecastFormatOptionalBool(value *bool) string {
	if value == nil {
		return "-"
	}
	if *value {
		return "yes"
	}
	return "no"
}

func forecastMarkdownEscape(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func forecastErrorAttributionFromEvent(event forecastVerificationEvent) forecastErrorAttribution {
	errorPct := forecastVerificationAuditErrorPct(event)
	bandOnly := forecastVerificationIsBandOnly(event)
	scenarioOnly := forecastVerificationIsScenarioOnly(event)
	predictedClose := event.PredictedClose
	predictedLabel := "predicted_close"
	eventType := "published_forecast_error"
	if bandOnly {
		eventType = "band_interval_miss"
		predictedLabel = "band_only"
		predictedClose = 0
	}
	if scenarioOnly {
		eventType = "scenario_drift"
		predictedClose = firstNonZeroForecastValue(event.ScenarioPredictedClose, event.PredictedClose)
		predictedLabel = "scenario_predicted_close"
	}
	causes := []string{}
	evidence := []string{
		fmt.Sprintf("close_abs_error_pct=%.2f", errorPct),
	}
	if bandOnly {
		evidence = append(evidence, fmt.Sprintf("%s actual_close=%.2f", predictedLabel, event.ActualClose))
		if event.ScenarioPointSuppressionReason != "" {
			evidence = append(evidence, "scenario_point_suppression_reason="+event.ScenarioPointSuppressionReason)
		}
		causes = append(causes, "band_only_interval_miss")
	} else {
		evidence = append(evidence, fmt.Sprintf("%s=%.2f actual_close=%.2f", predictedLabel, predictedClose, event.ActualClose))
	}
	if scenarioOnly {
		causes = append(causes, "suppressed_scenario_residual_error")
		evidence = append(evidence, "not_counted_as_published_forecast_error")
		if event.SuppressionReason != "" {
			evidence = append(evidence, "suppression_reason="+event.SuppressionReason)
		}
	} else if event.SuppressionReason != "" {
		causes = append(causes, "forecast_publish_gate_suppressed")
		evidence = append(evidence, "suppression_reason="+event.SuppressionReason)
	}
	if event.CloseDirectionHit != nil && !*event.CloseDirectionHit {
		causes = append(causes, "direction_model_miss")
		if scenarioOnly {
			evidence = append(evidence, "scenario direction did not match official close direction")
		} else {
			evidence = append(evidence, "predicted direction did not match official close direction")
		}
	}
	if event.IntervalHit != nil && !*event.IntervalHit {
		causes = append(causes, "prediction_interval_miss")
		evidence = append(evidence, fmt.Sprintf("interval=[%.2f,%.2f]", event.IntervalLow, event.IntervalHigh))
	}
	if event.BacktestCloseMAPE > 2 {
		causes = append(causes, "rolling_close_mape_high")
		evidence = append(evidence, fmt.Sprintf("backtest_close_mape=%.2f", event.BacktestCloseMAPE))
	}
	if event.BacktestDirectionAccuracy > 0 && event.BacktestDirectionAccuracy < 55 {
		causes = append(causes, "rolling_direction_accuracy_low")
		evidence = append(evidence, fmt.Sprintf("backtest_direction_accuracy=%.2f", event.BacktestDirectionAccuracy))
	}
	for _, warning := range event.Warnings {
		lower := strings.ToLower(warning)
		if strings.Contains(lower, "full_context") || strings.Contains(lower, "kap") || strings.Contains(lower, "limited") {
			causes = append(causes, "data_context_limited")
			evidence = append(evidence, "warning="+warning)
			break
		}
	}
	if len(causes) == 0 {
		causes = append(causes, "unexplained_residual_error")
	}
	primary := causes[0]
	return forecastErrorAttribution{
		EventType:         eventType,
		EntryID:           event.EntryID,
		Symbol:            event.Symbol,
		AsOfDate:          event.AsOfDate,
		TargetDate:        event.TargetDate,
		VerificationMode:  event.VerificationMode,
		ForecastPublished: !scenarioOnly && !bandOnly,
		PctError:          errorPct,
		PrimaryCause:      primary,
		SecondaryCauses:   dedupeForecastStrings(causes[1:]),
		Evidence:          dedupeForecastStrings(evidence),
		RecommendedFix:    forecastRecommendedFix(primary),
	}
}

func forecastRecommendedFix(cause string) string {
	switch cause {
	case "band_only_interval_miss":
		return "Band-only modda interval kacirdigi icin volatilite bandi ve tail-risk kalibrasyonunu genislet."
	case "suppressed_scenario_residual_error":
		return "Publish gate dogru calisti; scenario model drift'i icin KAP/makro/likidite ve volatilite rejimi challenger'i ekle."
	case "direction_model_miss":
		return "Regime ve BIST faktor yon agirliklarini challenger modelde yeniden test et."
	case "prediction_interval_miss":
		return "Volatilite rejimi ve prediction interval kalibrasyonunu genislet."
	case "rolling_close_mape_high":
		return "Close forecast publish gate esigini sikilastir ve zayif pencereyi challenger'a aktar."
	case "rolling_direction_accuracy_low":
		return "Yon tahminini karar sinyalinden ayir; direction confidence cap uygula."
	case "forecast_publish_gate_suppressed":
		return "Point forecast yayin kapisini gecmeyen gunlerde fiyat yerine band/senaryo raporla."
	case "data_context_limited":
		return "KAP, bilanço, makro ve full analysis snapshot kapsamini forecast ledger'a bagla."
	default:
		return "Residual hata icin feature attribution ve manuel olay incelemesi calistir."
	}
}

func dedupeForecastStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
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

func writeForecastErrorAttributionJSONL(path string, events []forecastErrorAttribution) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	for _, event := range events {
		if err := enc.Encode(event); err != nil {
			return err
		}
	}
	return nil
}
