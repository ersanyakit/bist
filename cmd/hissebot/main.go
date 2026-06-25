package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	documentingestion "hissebot/internal/application/documentingestion"
	"hissebot/internal/audit"
	"hissebot/internal/config"
	"hissebot/internal/enterprise"
	"hissebot/internal/extraction"
	"hissebot/internal/repositories/filedocuments"
	"hissebot/internal/services/bistbulletindb"
	"hissebot/internal/services/analysisreadiness"
	"hissebot/internal/services/classification"
	"hissebot/internal/services/comments"
	"hissebot/internal/services/financials"
	"hissebot/internal/services/kap"
	"hissebot/internal/services/kapsectors"
	"hissebot/internal/services/mkk"
	"hissebot/internal/services/news"
	"hissebot/internal/services/seed"
	"hissebot/internal/services/sirketler"
	"hissebot/internal/services/tradingview"
	"hissebot/internal/services/tuik"
	"hissebot/internal/services/universe"
	"hissebot/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("command required")
	}

	cfg := config.Load()
	store := storage.NewEquityStore(cfg.EquitiesDir)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.CommandTimeout)
	defer cancel()

	switch args[0] {
	case "seed":
		return runSeed(ctx, cfg, store, args[1:])
	case "sync":
		return runSync(ctx, cfg, store, args[1:])
	case "migrate":
		return runMigrate(ctx, cfg, store, args[1:])
	case "financials":
		return runFinancials(ctx, cfg, store, args[1:])
	case "analyze":
		return runAnalyze(ctx, cfg, store, args[1:])
	case "forecast-audit":
		return runForecastAudit(ctx, cfg, store, args[1:])
	case "serve":
		return runServe(ctx, cfg, store, args[1:])
	case "audit":
		return runAudit(ctx, cfg, store, args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runAudit(ctx context.Context, cfg config.Config, store *storage.EquityStore, args []string) error {
	if len(args) == 0 {
		return errors.New("audit subcommand required")
	}
	switch args[0] {
	case "enterprise":
		fs := flag.NewFlagSet("audit enterprise", flag.ExitOnError)
		mode := fs.String("mode", "research", "readiness mode: research or production")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		*mode = strings.ToLower(strings.TrimSpace(*mode))
		if *mode != "research" && *mode != "production" {
			return fmt.Errorf("invalid audit mode %q, expected research or production", *mode)
		}
		if _, err := audit.Append(cfg.AuditLogPath, audit.Event{
			Action:  "enterprise_audit_started",
			Entity:  "enterprise_readiness",
			Details: map[string]any{"mode": *mode},
		}); err != nil {
			return err
		}
		report := enterprise.CheckReadinessWithOptions(ctx, cfg, store, enterprise.ReadinessOptions{Mode: *mode})
		if err := printJSON(report); err != nil {
			return err
		}
		_, _ = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:  "enterprise_audit_completed",
			Entity:  "enterprise_readiness",
			Details: map[string]any{"status": report.Status, "score": report.Score, "mode": *mode},
		})
		if report.Status != "pass" {
			return fmt.Errorf("enterprise readiness failed: score %.1f", report.Score)
		}
		return nil
	case "analysis-readiness", "analysis", "decision-readiness":
		fs := flag.NewFlagSet("audit analysis-readiness", flag.ExitOnError)
		symbol := fs.String("symbol", "", "single BIST symbol, e.g. ASELS")
		all := fs.Bool("all", false, "audit all equities in data/equities")
		limit := fs.Int("limit", 0, "max symbol count, 0 means unlimited")
		mode := fs.String("mode", "research", "readiness mode: research or production")
		dbPath := fs.String("db", bistBulletinDBPath(cfg.DataDir), "BIST official OHLCV SQLite path")
		sectors := fs.String("sectors", cfg.SectorClassificationsFile, "sector classifications JSON file")
		out := fs.String("out", "", "optional JSON output file")
		minBars := fs.Int("min-bars", 0, "minimum official daily bars; 0 uses mode default")
		maxPriceAge := fs.Duration("max-price-age", 0, "maximum official close staleness; 0 uses mode default")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		symbols := splitCSV(*symbol)
		if len(symbols) == 0 && !*all {
			return errors.New("audit analysis-readiness requires -symbol or -all")
		}
		report, err := analysisreadiness.Run(ctx, cfg, store, analysisreadiness.Options{
			Symbols:           symbols,
			All:               *all,
			Limit:             *limit,
			Mode:              *mode,
			BISTDBPath:        *dbPath,
			SectorFile:        *sectors,
			MinDailyBars:      *minBars,
			MaxPriceStaleness: *maxPriceAge,
		})
		if err != nil {
			return err
		}
		if strings.TrimSpace(*out) != "" {
			raw, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			raw = append(raw, '\n')
			if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(*out, raw, 0o644); err != nil {
				return err
			}
		}
		if err := printJSON(report); err != nil {
			return err
		}
		_, _ = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:  "analysis_readiness_audit_completed",
			Entity:  "analysis_readiness",
			Details: map[string]any{"status": report.Status, "score": report.Score, "symbols": report.Symbols, "decision_ready": report.DecisionReady, "limited": report.Limited, "blocked": report.Blocked},
		})
		if report.Status == analysisreadiness.StatusBlocked {
			return fmt.Errorf("analysis readiness blocked: score %.1f blocked=%d", report.Score, report.Blocked)
		}
		return nil
	case "universe":
		report, err := universe.Validate(ctx, cfg, store)
		if err != nil {
			return err
		}
		if err := printJSON(report); err != nil {
			return err
		}
		if report.Status != "pass" {
			return errors.New("universe validation failed")
		}
		return nil
	default:
		return fmt.Errorf("unknown audit subcommand %q", args[0])
	}
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func parseOptionalDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{"2006-01-02", "02.01.2006"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD or DD.MM.YYYY", value)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func runSeed(ctx context.Context, cfg config.Config, store *storage.EquityStore, args []string) error {
	if len(args) == 0 {
		return errors.New("seed subcommand required")
	}
	switch args[0] {
	case "kap":
		fs := flag.NewFlagSet("seed kap", flag.ExitOnError)
		file := fs.String("file", cfg.KAPCompaniesFile, "KAP seed JSON file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return kap.ImportCompanies(ctx, store, *file)
	case "kap-disclosures":
		fs := flag.NewFlagSet("seed kap-disclosures", flag.ExitOnError)
		file := fs.String("file", cfg.KAPDisclosuresFile, "KAP financial disclosures JSON file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := kap.ImportFinancialDisclosures(ctx, store, *file); err != nil {
			return err
		}
		_, err := audit.Append(cfg.AuditLogPath, audit.Event{
			Action:   "seed_kap_disclosures",
			Entity:   "kap_disclosures",
			EntityID: *file,
		})
		return err
	case "tracklist":
		fs := flag.NewFlagSet("seed tracklist", flag.ExitOnError)
		file := fs.String("file", cfg.InvestingTrackIDsFile, "Investing track id JSON file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return seed.ImportTrackIDs(ctx, cfg, *file)
	case "bilanco":
		fs := flag.NewFlagSet("seed bilanco", flag.ExitOnError)
		file := fs.String("file", cfg.LegacyBilancoFile, "legacy tumBilancolar.json file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return financials.ImportLegacyBilanco(ctx, store, *file)
	case "sirketler":
		fs := flag.NewFlagSet("seed sirketler", flag.ExitOnError)
		file := fs.String("file", "Şirketler.xlsx", "Şirketler XLSX file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		result, err := sirketler.ImportMissing(ctx, store, *file)
		if err != nil {
			return err
		}
		fmt.Printf("sirketler: %s\n", result)
		return nil
	case "universe":
		report, err := universe.ExportCurrentUniverse(ctx, cfg, store)
		if err != nil {
			return err
		}
		return printJSON(report)
	default:
		return fmt.Errorf("unknown seed subcommand %q", args[0])
	}
}

func runSync(ctx context.Context, cfg config.Config, store *storage.EquityStore, args []string) error {
	if len(args) == 0 {
		return errors.New("sync subcommand required")
	}
	switch args[0] {
	case "bist-bulletin-db", "bistdb", "official-bist":
		fs := flag.NewFlagSet("sync bist-bulletin-db", flag.ExitOnError)
		dbPath := fs.String("db", bistBulletinDBPath(cfg.DataDir), "BIST resmi bülten SQLite dosyası")
		rawRoot := fs.String("raw", filepath.Join(cfg.DataDir, "bist", "unprocessed", "bulten_verileri"), "BIST THB ham dosya kökü")
		baseURL := fs.String("base-url", bistbulletindb.DefaultBaseURL, "BIST THB ZIP base URL")
		from := fs.String("from", "", "başlangıç tarihi, YYYY-MM-DD veya DD.MM.YYYY")
		to := fs.String("to", "", "bitiş tarihi, YYYY-MM-DD veya DD.MM.YYYY")
		fromYear := fs.Int("from-year", 0, "başlangıç yılı; -from yoksa kullanılır")
		toYear := fs.Int("to-year", 0, "bitiş yılı; -to yoksa kullanılır")
		session := fs.Int("session", 1, "BIST bülten seansı")
		download := fs.Bool("download", true, "eksik resmi THB ZIP dosyalarını BIST'ten indir")
		force := fs.Bool("force", false, "mevcut ham dosya/kaynak durumunu yeniden işle")
		requestDelay := fs.Duration("request-delay", 150*time.Millisecond, "BIST ZIP istekleri arası bekleme")
		timeout := fs.Duration("timeout", cfg.HTTPTimeout, "BIST ZIP istek timeout'u")
		retryMissingAfter := fs.Duration("retry-missing-after", 24*time.Hour, "404 kaynakları yeniden denemeden önce beklenecek süre")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		fromDate, err := parseOptionalDate(*from)
		if err != nil {
			return err
		}
		toDate, err := parseOptionalDate(*to)
		if err != nil {
			return err
		}
		result, err := bistbulletindb.Sync(ctx, bistbulletindb.Options{
			DBPath:            *dbPath,
			RawRoot:           *rawRoot,
			BaseURL:           *baseURL,
			FromDate:          fromDate,
			ToDate:            toDate,
			FromYear:          *fromYear,
			ToYear:            *toYear,
			Session:           *session,
			Download:          *download,
			Force:             *force,
			RequestDelay:      *requestDelay,
			Timeout:           *timeout,
			RetryMissingAfter: *retryMissingAfter,
		})
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
		_, auditErr := audit.Append(cfg.AuditLogPath, audit.Event{
			Action:   "sync_bist_bulletin_db",
			Entity:   "bist_official_ohlcv",
			EntityID: *dbPath,
			Details: map[string]any{
				"remote_downloaded":      result.RemoteDownloaded,
				"remote_missing":         result.RemoteMissing,
				"local_sources_imported": result.LocalSourcesImported,
				"database_candles":       result.DatabaseCandles,
				"database_symbols":       result.DatabaseSymbols,
				"errors":                 result.SourcesFailed + len(result.Errors),
			},
		})
		if err != nil {
			if auditErr != nil {
				return fmt.Errorf("%w; audit append: %v", err, auditErr)
			}
			return err
		}
		return auditErr
	case "tradingview":
		fs := flag.NewFlagSet("sync tradingview", flag.ExitOnError)
		requests := fs.String("requests", cfg.TradingViewRequestsFile, "TradingView request seed JSON file")
		fetch := fs.Bool("fetch", true, "fetch remote scanner data before parsing")
		parse := fs.Bool("parse", true, "parse cached scanner data into equity JSON files")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return tradingview.Sync(ctx, cfg, store, *requests, *fetch, *parse)
	case "ohlcv":
		return tradingview.SyncOHLCV(ctx, cfg, store)
	case "charts":
		fs := flag.NewFlagSet("sync charts", flag.ExitOnError)
		ticker := fs.String("ticker", "", "single ticker to sync, e.g. ASELS")
		intervals := fs.String("intervals", strings.Join(tradingview.DefaultChartIntervals, ","), "comma-separated TradingView intervals")
		bars := fs.Int("bars", 0, "bars per interval, 0 means all available history")
		limit := fs.Int("limit", 0, "max equity count, 0 means all")
		onlyWithOHLCV := fs.Bool("only-with-ohlcv", true, "sync only equities that already have ohlcv data")
		transport := fs.String("transport", cfg.TradingViewChartTransport, "chart transport: auto, http, or socket")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return tradingview.SyncCharts(ctx, cfg, store, tradingview.ChartSyncOptions{
			Ticker:        *ticker,
			Intervals:     tradingview.ParseIntervals(*intervals),
			Bars:          *bars,
			Limit:         *limit,
			OnlyWithOHLCV: *onlyWithOHLCV,
			Transport:     *transport,
		})
	case "kap":
		fs := flag.NewFlagSet("sync kap", flag.ExitOnError)
		file := fs.String("file", cfg.KAPCompaniesFile, "KAP company JSON output/input file")
		url := fs.String("url", "", "KAP company list endpoint override")
		live := fs.Bool("live", true, "fetch live KAP BIST company list before importing")
		memberType := fs.String("member-type", "IGS", "KAP member type, IGS means BIST listed companies")
		state := fs.String("state", "A", "KAP member state, A means active")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *live {
			if err := kap.SyncCompanies(ctx, cfg, store, kap.CompanySyncOptions{
				URL:        *url,
				MemberType: *memberType,
				State:      *state,
				OutputFile: *file,
			}); err != nil {
				return err
			}
			_, err := audit.Append(cfg.AuditLogPath, audit.Event{
				Action:   "sync_kap_companies_live",
				Entity:   "kap_companies",
				EntityID: *file,
				Details:  map[string]any{"member_type": *memberType, "state": *state},
			})
			return err
		}
		return kap.ImportCompanies(ctx, store, *file)
	case "kap-disclosures":
		fs := flag.NewFlagSet("sync kap-disclosures", flag.ExitOnError)
		file := fs.String("file", cfg.KAPDisclosuresFile, "KAP financial disclosures JSON file")
		url := fs.String("url", cfg.KAPDisclosuresURL, "KAP financial disclosures JSON feed URL")
		from := fs.String("from", "", "KAP disclosure start date, YYYY-MM-DD or DD.MM.YYYY")
		to := fs.String("to", "", "KAP disclosure end date, YYYY-MM-DD or DD.MM.YYYY")
		chunkDays := fs.Int("chunk-days", 90, "days per KAP disclosure API request, max 90")
		memberTypes := fs.String("member-types", "IGS", "comma-separated KAP member type codes")
		disclosureTypes := fs.String("disclosure-types", "FR", "comma-separated KAP disclosure type codes; use all to fetch every category")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*url) != "" || strings.TrimSpace(*from) != "" || strings.TrimSpace(*to) != "" {
			fromDate, err := parseOptionalDate(*from)
			if err != nil {
				return err
			}
			toDate, err := parseOptionalDate(*to)
			if err != nil {
				return err
			}
			cfg.KAPDisclosuresURL = *url
			if err := kap.SyncFinancialDisclosures(ctx, cfg, store, kap.FinancialDisclosureSyncOptions{
				URL:             *url,
				FromDate:        fromDate,
				ToDate:          toDate,
				ChunkDays:       *chunkDays,
				MemberTypes:     splitCSV(*memberTypes),
				DisclosureTypes: splitCSV(*disclosureTypes),
			}); err != nil {
				return err
			}
			_, err = audit.Append(cfg.AuditLogPath, audit.Event{
				Action:   "sync_kap_disclosures",
				Entity:   "kap_disclosures",
				EntityID: firstNonEmpty(cfg.KAPDisclosuresURL, "default_kap_disclosure_api"),
			})
			return err
		}
		if err := kap.ImportFinancialDisclosures(ctx, store, *file); err != nil {
			return err
		}
		_, err := audit.Append(cfg.AuditLogPath, audit.Event{
			Action:   "sync_kap_disclosures_file",
			Entity:   "kap_disclosures",
			EntityID: *file,
		})
		return err
	case "kap-sectors":
		fs := flag.NewFlagSet("sync kap-sectors", flag.ExitOnError)
		out := fs.String("out", cfg.KAPSectorsFile, "KAP sector page normalized JSON output file")
		url := fs.String("url", cfg.KAPSectorsURL, "KAP sector page URL")
		timeout := fs.Duration("timeout", cfg.HTTPTimeout, "KAP sector page request timeout")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		result, err := kapsectors.Sync(ctx, cfg, kapsectors.Options{
			URL:        *url,
			OutputPath: *out,
			Timeout:    *timeout,
		})
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		_, err = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:   "sync_kap_sectors",
			Entity:   "kap_sectors",
			EntityID: *out,
			Details:  map[string]any{"rows": result.Rows, "symbols": result.Symbols, "main_sectors": result.MainSectors, "normal_sectors": result.NormalSectors, "source_url": result.SourceURL},
		})
		return err
	case "kap-attachments":
		fs := flag.NewFlagSet("sync kap-attachments", flag.ExitOnError)
		out := fs.String("out", filepath.Join(cfg.EquitiesDir, "_kap"), "KAP detail, manifest and failure output directory")
		ticker := fs.String("ticker", "", "single ticker to sync, e.g. ASELS")
		from := fs.String("from", "", "KAP attachment disclosure start date, YYYY-MM-DD or DD.MM.YYYY")
		to := fs.String("to", "", "KAP attachment disclosure end date, YYYY-MM-DD or DD.MM.YYYY")
		limit := fs.Int("limit", 0, "max new files to download, 0 means unlimited")
		maxErrors := fs.Int("max-errors", 0, "stop after this many detail/file errors, 0 means unlimited")
		maxBytes := fs.Int64("max-bytes", 0, "max bytes to download in this run, 0 means unlimited")
		minFreeBytes := fs.Int64("min-free-bytes", 5<<30, "stop before free disk space goes below this threshold")
		delay := fs.Duration("delay", time.Second, "delay between KAP detail/file requests")
		errorDelay := fs.Duration("error-delay", 5*time.Second, "delay after a failed KAP detail/file after retries are exhausted")
		retries := fs.Int("retries", 2, "retry count after transient KAP detail/file request errors")
		rateLimitSleep := fs.Duration("rate-limit-sleep", 10*time.Minute, "sleep and continue after KAP 429 rate-limit responses; 0 stops on rate limit")
		transientErrorSleep := fs.Duration("transient-error-sleep", 20*time.Minute, "sleep after repeated transient KAP EOF/connection errors")
		transientErrorThreshold := fs.Int("transient-error-threshold", 5, "consecutive transient KAP errors before long cooldown")
		timeout := fs.Duration("timeout", 2*time.Minute, "per-request timeout")
		repeat := fs.Bool("repeat", false, "repeat full passes until the process is stopped")
		passDelay := fs.Duration("pass-delay", 5*time.Minute, "delay between repeated KAP attachment passes")
		newestFirst := fs.Bool("newest-first", false, "download newer disclosure attachments before older archive")
		tickerOrder := fs.Bool("ticker-order", false, "process tickers one by one in alphabetical order and print each completed ticker")
		retryFailedTicker := fs.Bool("retry-failed-ticker", false, "with -ticker-order, retry the current ticker until a pass finishes without attachment errors")
		failedTickerDelay := fs.Duration("failed-ticker-delay", 2*time.Minute, "delay before retrying a failed ticker in -ticker-order mode")
		force := fs.Bool("force", false, "re-download details and files even if already present")
		detailsOnly := fs.Bool("details-only", false, "download/cache only attachment detail JSON, not binary files")
		verbose := fs.Bool("verbose", false, "print per-file KAP attachment errors")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		fromDate, err := parseOptionalDate(*from)
		if err != nil {
			return err
		}
		toDate, err := parseOptionalDate(*to)
		if err != nil {
			return err
		}
		opts := kap.AttachmentSyncOptions{
			OutputDir:               *out,
			Ticker:                  *ticker,
			FromDate:                fromDate,
			ToDate:                  toDate,
			Limit:                   *limit,
			MaxErrors:               *maxErrors,
			NewestFirst:             *newestFirst,
			Force:                   *force,
			DetailsOnly:             *detailsOnly,
			Delay:                   *delay,
			Retries:                 *retries,
			RateLimitSleep:          *rateLimitSleep,
			TransientErrorSleep:     *transientErrorSleep,
			TransientErrorThreshold: *transientErrorThreshold,
			ErrorDelay:              *errorDelay,
			Verbose:                 *verbose,
			MaxBytes:                *maxBytes,
			MinFreeBytes:            *minFreeBytes,
			Timeout:                 *timeout,
		}
		if *tickerOrder && *ticker == "" {
			tickers, err := kapAttachmentTickerOrder(cfg.EquitiesDir)
			if err != nil {
				return err
			}
			if len(tickers) == 0 {
				return errors.New("no tickers found in data/equities/*/kap_disclosures.json")
			}
			for pass := 1; ; pass++ {
				for i, orderedTicker := range tickers {
					for attempt := 1; ; attempt++ {
						tickerOpts := opts
						tickerOpts.Ticker = orderedTicker
						result, err := kap.SyncAttachments(ctx, cfg, store, tickerOpts)
						if err != nil {
							return err
						}
						fmt.Printf("kap attachments A-Z completed pass=%d index=%d/%d ticker=%s attempt=%d files_downloaded=%d files_skipped=%d errors=%d stopped=%s\n", pass, i+1, len(tickers), orderedTicker, attempt, result.FilesDownloaded, result.FilesSkipped, result.Errors, result.StoppedReason)
						if err := printJSON(map[string]any{
							"pass":         pass,
							"ticker":       orderedTicker,
							"attempt":      attempt,
							"index":        i + 1,
							"total":        len(tickers),
							"completed_at": time.Now().UTC(),
							"result":       result,
						}); err != nil {
							return err
						}
						_, err = audit.Append(cfg.AuditLogPath, audit.Event{
							Action:  "sync_kap_attachments_ticker_order",
							Entity:  "kap_attachments",
							Details: map[string]any{"ticker": orderedTicker, "pass": pass, "attempt": attempt, "index": i + 1, "total": len(tickers), "files_downloaded": result.FilesDownloaded, "errors": result.Errors, "bytes": result.DownloadedBytes, "stopped": result.StoppedReason},
						})
						if err != nil {
							return err
						}
						if !*retryFailedTicker || result.Errors == 0 {
							break
						}
						fmt.Printf("kap attachments A-Z retrying pass=%d index=%d/%d ticker=%s next_attempt=%d errors=%d stopped=%s delay=%s\n", pass, i+1, len(tickers), orderedTicker, attempt+1, result.Errors, result.StoppedReason, failedTickerDelay.String())
						if err := sleepContext(ctx, *failedTickerDelay); err != nil {
							return err
						}
					}
				}
				if !*repeat {
					return nil
				}
				if err := sleepContext(ctx, *passDelay); err != nil {
					return err
				}
			}
		}
		for pass := 1; ; pass++ {
			result, err := kap.SyncAttachments(ctx, cfg, store, opts)
			if err != nil {
				return err
			}
			if *repeat {
				if err := printJSON(map[string]any{"pass": pass, "result": result}); err != nil {
					return err
				}
			} else if err := printJSON(result); err != nil {
				return err
			}
			_, err = audit.Append(cfg.AuditLogPath, audit.Event{
				Action:  "sync_kap_attachments",
				Entity:  "kap_attachments",
				Details: map[string]any{"ticker": *ticker, "pass": pass, "files_downloaded": result.FilesDownloaded, "errors": result.Errors, "bytes": result.DownloadedBytes, "stopped": result.StoppedReason},
			})
			if err != nil {
				return err
			}
			if !*repeat {
				return nil
			}
			if err := sleepContext(ctx, *passDelay); err != nil {
				return err
			}
		}
	case "kap-document-archive":
		fs := flag.NewFlagSet("sync kap-document-archive", flag.ExitOnError)
		out := fs.String("out", filepath.Join(cfg.EquitiesDir, "_kap"), "document registry output directory")
		ticker := fs.String("ticker", "", "single ticker to archive, e.g. ASELS")
		limit := fs.Int("limit", 0, "max documents to archive, 0 means unlimited")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		repo, err := filedocuments.New(*out)
		if err != nil {
			return err
		}
		service := documentingestion.Service{Repository: repo}
		result, err := service.ArchiveLocalKAPAttachments(ctx, documentingestion.ArchiveRequest{
			EquitiesDir: cfg.EquitiesDir,
			Ticker:      *ticker,
			Limit:       *limit,
		})
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		_, err = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:   "sync_kap_document_archive",
			Entity:   "documents",
			EntityID: result.Job.JobID,
			Details:  map[string]any{"ticker": *ticker, "documents_saved": result.Job.DocumentsSaved, "documents_skipped": result.Job.DocumentsSkipped, "errors": result.Job.Errors, "status": result.Job.Status},
		})
		return err
	case "kap-extract":
		fs := flag.NewFlagSet("sync kap-extract", flag.ExitOnError)
		registryDir := fs.String("registry", filepath.Join(cfg.EquitiesDir, "_kap"), "document registry directory")
		ticker := fs.String("ticker", "", "single ticker to extract, e.g. ASELS; empty processes all archived tickers")
		limit := fs.Int("limit", 0, "max latest documents per ticker, 0 means unlimited")
		includeNonLatest := fs.Bool("include-non-latest", false, "also process non-latest document versions")
		maxChars := fs.Int("max-chars", 200000, "max extracted text chars per document")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		repo, err := filedocuments.New(*registryDir)
		if err != nil {
			return err
		}
		processor := extraction.Processor{
			Repository:          repo,
			EquitiesDir:         cfg.EquitiesDir,
			MaxCharsPerDocument: *maxChars,
		}
		result, err := processor.ProcessBatch(ctx, extraction.Options{
			Ticker:           *ticker,
			Limit:            *limit,
			IncludeNonLatest: *includeNonLatest,
		})
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		_, err = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:  "sync_kap_extract",
			Entity:  "kap_extraction",
			Details: map[string]any{"ticker": *ticker, "tickers": result.Tickers, "documents_processed": result.DocumentsProcessed, "financial_facts": result.FinancialFacts, "review_items": result.ReviewItems},
		})
		return err
	case "mkk":
		return mkk.Sync(ctx, cfg, store)
	case "sectors":
		fs := flag.NewFlagSet("sync sectors", flag.ExitOnError)
		out := fs.String("out", cfg.SectorClassificationsFile, "sector classification output JSON file")
		sourceFiles := fs.String("source-file", "", "comma-separated external sector JSON/CSV files, e.g. KAP/Fintables/Investing export")
		kapSectors := fs.Bool("kap-sectors", true, "use normalized KAP /tr/Sektorler data as official sector source")
		kapSectorsFile := fs.String("kap-sectors-file", cfg.KAPSectorsFile, "normalized KAP sectors JSON file")
		refreshKAP := fs.Bool("refresh-kap", false, "fetch KAP /tr/Sektorler before building classifications")
		useTradingView := fs.Bool("tradingview", true, "enrich sector/industry from TradingView scanner")
		maxPeers := fs.Int("max-peers", 30, "max peer symbols per equity")
		preserve := fs.Bool("preserve-existing", true, "preserve higher-confidence existing classifications")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *kapSectors && (*refreshKAP || !fileExists(*kapSectorsFile)) {
			if _, err := kapsectors.Sync(ctx, cfg, kapsectors.Options{OutputPath: *kapSectorsFile}); err != nil {
				return err
			}
		}
		result, err := classification.Sync(ctx, cfg, store, classification.Options{
			OutputPath:        *out,
			SourceFiles:       splitCSV(*sourceFiles),
			KAPSectorsFile:    *kapSectorsFile,
			DisableKAPSectors: !*kapSectors,
			UseTradingView:    *useTradingView,
			MaxPeers:          *maxPeers,
			PreserveExisting:  *preserve,
		})
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		_, err = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:  "sync_sectors",
			Entity:  "sector_classifications",
			Details: map[string]any{"entries": result.Entries, "peer_groups": result.PeerGroups, "low_confidence": result.LowConfidence, "kap_sectors": result.KAPSectorClassified, "tradingview": result.TradingViewClassified},
		})
		return err
	case "news":
		fs := flag.NewFlagSet("sync news", flag.ExitOnError)
		ticker := fs.String("ticker", "", "single ticker to sync, e.g. ASELS")
		rss := fs.String("rss", "", "comma-separated RSS/feed URLs for financial news sources")
		limit := fs.Int("limit", 200, "max normalized news/comment items per ticker")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		result, err := news.Sync(ctx, store, news.Options{
			Ticker:  *ticker,
			Limit:   *limit,
			RSSURLs: splitCSV(*rss),
		})
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		_, err = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:  "sync_news",
			Entity:  "news_sentiment",
			Details: map[string]any{"ticker": *ticker, "tickers": result.Tickers, "items": result.Items},
		})
		return err
	case "tuik-gdp":
		fs := flag.NewFlagSet("sync tuik-gdp", flag.ExitOnError)
		out := fs.String("out", cfg.TUIKGDPFile, "TÜİK CİP GSYH makro veri JSON dosyası")
		years := fs.Int("years", 10, "çekilecek yıl sayısı")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		result, err := tuik.SyncGDP(ctx, tuik.GDPOptions{
			OutputPath: *out,
			Years:      *years,
			Timeout:    cfg.HTTPTimeout,
		})
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		_, err = audit.Append(cfg.AuditLogPath, audit.Event{
			Action:   "sync_tuik_gdp",
			Entity:   "macro_gdp",
			EntityID: *out,
			Details:  map[string]any{"latest_year": result.LatestYear, "years": result.Years, "source_url": result.SourceURL},
		})
		return err
	case "all":
		if err := kap.ImportCompanies(ctx, store, cfg.KAPCompaniesFile); err != nil {
			return err
		}
		if err := kap.ImportFinancialDisclosures(ctx, store, cfg.KAPDisclosuresFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := tradingview.Sync(ctx, cfg, store, cfg.TradingViewRequestsFile, true, true); err != nil {
			return err
		}
		if err := tradingview.SyncOHLCV(ctx, cfg, store); err != nil {
			return err
		}
		if err := mkk.Sync(ctx, cfg, store); err != nil {
			return err
		}
		if _, err := kapsectors.Sync(ctx, cfg, kapsectors.Options{}); err != nil {
			return err
		}
		_, err := classification.Sync(ctx, cfg, store, classification.Options{
			OutputPath:       cfg.SectorClassificationsFile,
			KAPSectorsFile:   cfg.KAPSectorsFile,
			UseTradingView:   true,
			PreserveExisting: true,
		})
		return err
	case "all-data":
		fs := flag.NewFlagSet("sync all-data", flag.ExitOnError)
		chartBars := fs.Int("chart-bars", 0, "bars per chart interval, 0 means all available history")
		chartIntervals := fs.String("chart-intervals", strings.Join(tradingview.DefaultChartIntervals, ","), "comma-separated TradingView chart intervals")
		chartLimit := fs.Int("chart-limit", 0, "max equity count for chart sync, 0 means all")
		chartTransport := fs.String("chart-transport", cfg.TradingViewChartTransport, "chart transport: auto, http, or socket")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if _, err := sirketler.ImportMissing(ctx, store, "Şirketler.xlsx"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := kap.ImportCompanies(ctx, store, cfg.KAPCompaniesFile); err != nil {
			return err
		}
		if err := kap.ImportFinancialDisclosures(ctx, store, cfg.KAPDisclosuresFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := tradingview.Sync(ctx, cfg, store, cfg.TradingViewRequestsFile, true, true); err != nil {
			return err
		}
		if err := tradingview.SyncOHLCV(ctx, cfg, store); err != nil {
			return err
		}
		if err := tradingview.SyncCharts(ctx, cfg, store, tradingview.ChartSyncOptions{
			Intervals:     tradingview.ParseIntervals(*chartIntervals),
			Bars:          *chartBars,
			Limit:         *chartLimit,
			OnlyWithOHLCV: true,
			Transport:     *chartTransport,
		}); err != nil {
			return err
		}
		if err := mkk.Sync(ctx, cfg, store); err != nil {
			return err
		}
		if _, err := kapsectors.Sync(ctx, cfg, kapsectors.Options{}); err != nil {
			return err
		}
		_, err := classification.Sync(ctx, cfg, store, classification.Options{
			OutputPath:       cfg.SectorClassificationsFile,
			KAPSectorsFile:   cfg.KAPSectorsFile,
			UseTradingView:   true,
			PreserveExisting: true,
		})
		return err
	default:
		return fmt.Errorf("unknown sync subcommand %q", args[0])
	}
}

func kapAttachmentTickerOrder(equitiesDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(equitiesDir, "*", "kap_disclosures.json"))
	if err != nil {
		return nil, err
	}
	tickers := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		ticker := storage.NormalizeTicker(filepath.Base(filepath.Dir(match)))
		if ticker == "" {
			continue
		}
		if _, ok := seen[ticker]; ok {
			continue
		}
		seen[ticker] = struct{}{}
		tickers = append(tickers, ticker)
	}
	sort.Strings(tickers)
	return tickers, nil
}

func runMigrate(_ context.Context, _ config.Config, store *storage.EquityStore, args []string) error {
	if len(args) == 0 {
		return errors.New("migrate subcommand required")
	}
	switch args[0] {
	case "layout":
		equitiesMoved, chartsMoved, err := store.MigrateToDirectories()
		if err != nil {
			return err
		}
		fmt.Printf("migrate layout: %d equity files moved, %d chart files moved\n", equitiesMoved, chartsMoved)
		return nil
	default:
		return fmt.Errorf("unknown migrate subcommand %q", args[0])
	}
}

func runFinancials(ctx context.Context, cfg config.Config, store *storage.EquityStore, args []string) error {
	if len(args) == 0 {
		return errors.New("financials subcommand required")
	}
	switch args[0] {
	case "fetch":
		fs := flag.NewFlagSet("financials fetch", flag.ExitOnError)
		force := fs.Bool("force", false, "kept for compatibility; current year is refreshed by default")
		forceHistory := fs.Bool("force-history", false, "also overwrite existing historical years")
		ticker := fs.String("ticker", "", "single ticker to fetch, e.g. ASELS")
		limit := fs.Int("limit", 0, "max equity count, 0 means all")
		workers := fs.Int("workers", financials.DefaultFetchWorkers, "parallel year worker count")
		retries := fs.Int("retries", financials.DefaultFetchRetries, "retry count per ticker-year after fetch errors")
		tickerDelay := fs.Duration("ticker-delay", financials.DefaultTickerDelay, "delay between ticker batches")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return financials.Run(ctx, cfg, store, financials.FetchOptions{
			Force:        *force,
			ForceHistory: *forceHistory,
			Ticker:       *ticker,
			Limit:        *limit,
			Workers:      *workers,
			Retries:      *retries,
			TickerDelay:  *tickerDelay,
		})
	case "merge":
		return financials.Merge(ctx, cfg, store)
	case "calculate":
		return financials.Calculate(ctx, store)
	case "metadata":
		updated, resolved, err := kap.RefreshFinancialPublishDateMetadata(ctx, store)
		if err != nil {
			return err
		}
		fmt.Printf("financials metadata: %d equities refreshed, %d periods resolved from KAP disclosures\n", updated, resolved)
		return nil
	case "reconcile":
		report, err := financials.Reconcile(ctx, store)
		if err != nil {
			return err
		}
		if err := printJSON(report); err != nil {
			return err
		}
		if report.Failures > 0 {
			return fmt.Errorf("financial reconciliation failed for %d checks", report.Failures)
		}
		return nil
	case "import":
		fs := flag.NewFlagSet("financials import", flag.ExitOnError)
		file := fs.String("file", cfg.LegacyBilancoFile, "legacy tumBilancolar.json file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return financials.ImportLegacyBilanco(ctx, store, *file)
	case "run":
		fs := flag.NewFlagSet("financials run", flag.ExitOnError)
		force := fs.Bool("force", false, "kept for compatibility; current year is refreshed by default")
		forceHistory := fs.Bool("force-history", false, "also overwrite existing historical years")
		ticker := fs.String("ticker", "", "single ticker to fetch, e.g. ASELS")
		limit := fs.Int("limit", 0, "max equity count, 0 means all")
		workers := fs.Int("workers", financials.DefaultFetchWorkers, "parallel year worker count")
		retries := fs.Int("retries", financials.DefaultFetchRetries, "retry count per ticker-year after fetch errors")
		tickerDelay := fs.Duration("ticker-delay", financials.DefaultTickerDelay, "delay between ticker batches")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return financials.Run(ctx, cfg, store, financials.FetchOptions{
			Force:        *force,
			ForceHistory: *forceHistory,
			Ticker:       *ticker,
			Limit:        *limit,
			Workers:      *workers,
			Retries:      *retries,
			TickerDelay:  *tickerDelay,
		})
	default:
		return fmt.Errorf("unknown financials subcommand %q", args[0])
	}
}

func runServe(ctx context.Context, cfg config.Config, store *storage.EquityStore, args []string) error {
	if len(args) == 0 {
		return errors.New("serve subcommand required")
	}
	switch args[0] {
	case "comments":
		runCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return comments.Serve(runCtx, cfg, store)
	case "reports":
		runCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return runReportServer(runCtx, cfg, store, args[1:])
	case "api":
		runCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		return runAPIServer(runCtx, cfg, store, args[1:])
	default:
		return fmt.Errorf("unknown serve subcommand %q", args[0])
	}
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `hissebot

Commands:
  seed kap                         KAP seed dosyasindan hisse JSON dosyalari olusturur/gunceller
  seed kap-disclosures             KAP finansal bildirimlerini ticker bazli kap_disclosures.json dosyalarina yazar
  seed tracklist                   Investing takip ID listesini cache'e yazar
  seed bilanco                     tumBilancolar.json dosyasini hisse JSON'larina boler
  seed sirketler                   Sirketler.xlsx icindeki eksik kodlari hisse JSON'u olarak ekler
  seed universe                    Mevcut equity JSON setinden listed/delisted universe JSON dosyalarini uretir
  sync tradingview                 TradingView scanner verisini cekip hisse JSON'larina yazar
  sync ohlcv                       TradingView anlik OHLCV verisini hisse JSON'larina yazar
  sync charts                      TradingView chart mum datasini hisse klasorundeki charts altina yazar
  sync bist-bulletin-db            Resmi BIST THB bulten ZIP'lerini indirir ve SQLite OHLCV indeksine isler
  sync all-data                    Tum hisse verilerini ve chart datasini ceker
  sync kap                         KAP BIST sirket listesini canli cekip hisse JSON'larina yazar
  sync kap-disclosures             KAP bildirimlerini ice aktarir; -disclosure-types all tum kategorileri ceker
  sync kap-sectors                 KAP /tr/Sektorler sayfasindan resmi sektor agacini ceker
  sync kap-attachments             KAP bildirim PDF/XBRL/Word/Excel eklerini data/equities altina resume destekli indirir
  sync kap-document-archive        Indirilmis KAP eklerini checksum, metadata ve version ile belge registry'sine yazar
  sync kap-extract                 Arsivli KAP belgelerinden kaynakli text/fact/event/asset adaylarini uretir
  sync mkk                         MKK sirket eslesmesini ve sirket bilgilerini yazar
  sync sectors                     KAP/MKK verisinden sektor, faaliyet alani ve peer evreni dosyasini uretir
  sync news                        KAP/yorum/RSS metinlerini haber-sentiment dosyasina normalize eder
  sync tuik-gdp                    TÜİK CİP kişi başına GSYH ve GSYH verisini macro JSON olarak çeker
  sync all                         KAP + TradingView + MKK
  migrate layout                   data/equities/{TICKER}.json dosyalarini data/equities/{TICKER}/equity.json yapisina tasir
  financials fetch                 Bilancolari tek geciste ceker, birlestirir ve hesaplar
  financials merge                 Cache bilancolari hisse JSON'larina birlestirir
  financials calculate             Finansal oranlari hesaplar
  financials metadata              KAP disclosure cache'inden publish-date metadata'sini ve statement version store'u yeniler
  financials reconcile             Finansal tablo reconciliation kontrollerini calistirir
  financials import                Legacy tumBilancolar.json dosyasini ice aktarir
  financials run                   Bilancolari tek geciste ceker, birlestirir ve hesaplar
  analyze                           Teknik analiz, indikator, formasyon ve grafik raporu uretir
  forecast-audit                   Resmi BIST gerceklesenlerle forecast nokta/range denetimi uretir
  serve comments                   Investing yorum websocket servisinin dosya tabanli hali
  serve reports                    Tek tus PDF/HTML analiz raporu ureten HTTP endpoint'i
  serve api                        Fiber v3 JSON API; sektor endpointleri dahil
  audit enterprise [-mode production] Kurumsal readiness gate kontrollerini JSON olarak calistirir
  audit universe                    Listed/delisted survivorship universe kapsam kontrolunu calistirir

Useful env:
  HISSEBOT_DATA_DIR                default ./data
  HISSEBOT_COMMAND_TIMEOUT         default 30m
  HISSEBOT_MKK_COOKIE              optional
  HISSEBOT_ISYATIRIM_COOKIE        optional
  HISSEBOT_TV_HISTORY_URL          optional, UDF /history compatible HTTP chart endpoint
  HISSEBOT_TV_CHART_TRANSPORT      optional, auto|http|socket, default auto
  HISSEBOT_TUIK_GDP_FILE           optional, default ./data/macro/tuik_gdp.json
  HISSEBOT_ENDPOINT_URL            optional, default http://127.0.0.1:1453/endpoint
`)
}
