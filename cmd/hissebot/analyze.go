package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hissebot/internal/adapters/analysisproviders"
	"hissebot/internal/audit"
	appconfig "hissebot/internal/config"
	corestorage "hissebot/internal/storage"
	"hissebot/internal/ta/analysis"
	tachart "hissebot/internal/ta/chart"
	taconfig "hissebot/internal/ta/config"
	"hissebot/internal/ta/datasource"
	"hissebot/internal/ta/excel"
	"hissebot/internal/ta/ohlcv"
	reportstorage "hissebot/internal/ta/storage"
)

type analyzeJob struct {
	symbol      string
	companyName string
	currency    string
}

type analyzeJobResult struct {
	symbol string
	err    error
}

func runAnalyze(ctx context.Context, appCfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	cfg := taconfig.Default()
	cfg.OutputDir = store.Root()
	all := false
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	fs.StringVar(&cfg.Symbol, "symbol", "", "tek BIST sembolu, ornek: ADEL")
	fs.StringVar(&cfg.ExcelPath, "excel", "", "symbol/company_name veya Kod/Sirket Unvani kolonlu Excel dosyasi")
	fs.BoolVar(&all, "all", false, "data/equities altindaki tum hisse senetlerini analiz et")
	fs.StringVar(&cfg.Provider, "provider", cfg.Provider, "veri kaynagi: tradingview, bistdb, csv veya mock")
	fs.StringVar(&cfg.DataDir, "data", cfg.DataDir, "veri kok dizini; bistdb icin data/bist/bist_ohlcv.sqlite kullanilir")
	fs.StringVar(&cfg.OutputDir, "out", cfg.OutputDir, "equities kok cikti klasoru")
	fs.StringVar(&cfg.TimeframesCSV, "timeframes", strings.Join(cfg.Timeframes, ","), "virgulle ayrilmis zaman dilimleri")
	fs.IntVar(&cfg.Workers, "workers", cfg.Workers, "toplu analiz isci sayisi")
	fs.IntVar(&cfg.Limit, "limit", cfg.Limit, "timeframe basina mum limiti")
	fs.StringVar(&cfg.DataMode, "mode", cfg.DataMode, "analiz veri modu: research veya production")
	fs.StringVar(&cfg.Benchmark, "benchmark", cfg.Benchmark, "profesyonel analiz icin benchmark sembolu")
	fs.StringVar(&cfg.ValuationAssumptionsFile, "valuation-assumptions", cfg.ValuationAssumptionsFile, "DCF/WACC varsayimlari JSON dosyasi")
	fs.StringVar(&cfg.MacroGDPFile, "macro-gdp", appCfg.TUIKGDPFile, "TÜİK CİP GSYH makro veri JSON dosyası")
	fs.Float64Var(&cfg.Portfolio, "portfolio", cfg.Portfolio, "pozisyon boyutu hesabinda varsayilan portfoy buyuklugu")
	fs.Float64Var(&cfg.RiskPct, "risk-pct", cfg.RiskPct, "islem basina portfoy risk yuzdesi")
	fs.IntVar(&cfg.PeerLimit, "peer-limit", cfg.PeerLimit, "rapora yazilacak maksimum peer sayisi")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.DataMode = strings.ToLower(strings.TrimSpace(cfg.DataMode))
	if cfg.DataMode != "research" && cfg.DataMode != "production" {
		return fmt.Errorf("invalid analysis mode %q, expected research or production", cfg.DataMode)
	}

	timeframes, err := taconfig.ParseTimeframes(cfg.TimeframesCSV)
	if err != nil {
		return fmt.Errorf("parse timeframes: %w", err)
	}
	cfg.Timeframes = timeframes

	provider, err := buildAnalyzeProvider(cfg)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	engine := analysis.NewEngine(provider, tachart.NewPNGRenderer(), analysis.EngineOptions{
		Timeframes:               cfg.Timeframes,
		Limit:                    cfg.Limit,
		EquitiesDir:              cfg.OutputDir,
		DataMode:                 cfg.DataMode,
		BenchmarkSymbol:          cfg.Benchmark,
		ValuationAssumptionsFile: cfg.ValuationAssumptionsFile,
		MacroGDPFile:             cfg.MacroGDPFile,
		PortfolioValue:           cfg.Portfolio,
		RiskPerTradePct:          cfg.RiskPct,
		PeerLimit:                cfg.PeerLimit,
		SkipKAPPDFIngest:         envFlag("HISSEBOT_SKIP_KAP_PDF_INGEST"),
		PriceQualityProvider:     analysisproviders.PriceQualityProvider{EquitiesDir: cfg.OutputDir},
		FormationsProvider:       analysisproviders.MatriksFormationsProvider{EquitiesDir: cfg.OutputDir},
	})
	writer := reportstorage.NewReportWriter()

	if cfg.Symbol != "" {
		symbol := ohlcv.NormalizeSymbol(cfg.Symbol)
		result, err := engine.AnalyzeSymbol(ctx, analysis.SymbolRequest{Symbol: symbol})
		if err != nil {
			return fmt.Errorf("analyze symbol %s: %w", symbol, err)
		}
		if err := writer.WriteAnalysis(ctx, cfg.OutputDir, result); err != nil {
			return fmt.Errorf("write analysis %s: %w", symbol, err)
		}
		_, _ = audit.Append(appCfg.AuditLogPath, audit.Event{
			Action:   "analysis_written",
			Entity:   "analysis_result",
			EntityID: result.Symbol,
			Details:  map[string]any{"analysis_date": result.AnalysisDate, "asset_type": result.AssetType, "provider": cfg.Provider, "timeframes": cfg.Timeframes},
		})
		log.Printf("analysis written for %s: %s", result.Symbol, reportstorage.AnalysisDirForAsset(cfg.OutputDir, result.AssetType, result.Symbol, result.AnalysisDate))
		return nil
	}

	companies, err := analyzeCompanies(ctx, store, cfg.ExcelPath, all)
	if err != nil {
		return err
	}
	if len(companies) == 0 {
		return fmt.Errorf("analyze input: %w", taconfig.ErrNoSymbols)
	}

	if err := runAnalyzeBatch(ctx, engine, writer, cfg.OutputDir, workersOrMin(cfg.Workers), companies); err != nil {
		return err
	}
	_, _ = audit.Append(appCfg.AuditLogPath, audit.Event{
		Action:  "analysis_batch_completed",
		Entity:  "analysis_result",
		Details: map[string]any{"symbols": len(companies), "provider": cfg.Provider, "timeframes": cfg.Timeframes},
	})
	return nil
}

func analyzeCompanies(ctx context.Context, store *corestorage.EquityStore, excelPath string, all bool) ([]analyzeJob, error) {
	if all {
		equities, err := store.List()
		if err != nil {
			return nil, fmt.Errorf("list equities: %w", err)
		}
		jobs := make([]analyzeJob, 0, len(equities))
		for _, equity := range equities {
			if equity.AssetType != 2 {
				continue
			}
			jobs = append(jobs, analyzeJob{
				symbol:      equity.Ticker,
				companyName: equity.Name,
				currency:    "TRY",
			})
		}
		return jobs, nil
	}
	if excelPath == "" {
		return nil, fmt.Errorf("missing input: %w", taconfig.ErrInputRequired)
	}
	companies, err := excel.ReadCompanies(ctx, excelPath)
	if err != nil {
		return nil, fmt.Errorf("read excel %s: %w", excelPath, err)
	}
	if len(companies) == 0 {
		return nil, fmt.Errorf("read excel %s: %w", excelPath, taconfig.ErrNoSymbols)
	}
	jobs := make([]analyzeJob, 0, len(companies))
	for _, company := range companies {
		jobs = append(jobs, analyzeJob{symbol: company.Symbol, companyName: company.CompanyName, currency: company.Currency})
	}
	return jobs, nil
}

func workersOrMin(workers int) int {
	if workers < taconfig.MinWorkers {
		workers = taconfig.MinWorkers
	}
	return workers
}

func runAnalyzeBatch(ctx context.Context, engine *analysis.Engine, writer reportstorage.ReportWriter, equitiesDir string, workers int, companies []analyzeJob) error {
	jobs := make(chan analyzeJob)
	results := make(chan analyzeJobResult)
	var wg sync.WaitGroup
	var logMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for item := range jobs {
				req := analysis.SymbolRequest{
					Symbol:      item.symbol,
					CompanyName: item.companyName,
					Currency:    item.currency,
				}
				result, err := engine.AnalyzeSymbol(ctx, req)
				if err == nil {
					err = writer.WriteAnalysis(ctx, equitiesDir, result)
				}
				if err != nil {
					logMu.Lock()
					log.Printf("analyze worker %d failed %s: %v", workerID, item.symbol, err)
					logMu.Unlock()
					results <- analyzeJobResult{symbol: item.symbol, err: err}
					continue
				}
				logMu.Lock()
				log.Printf("analyze worker %d wrote %s: %s", workerID, result.Symbol, reportstorage.AnalysisDirForAsset(equitiesDir, result.AssetType, result.Symbol, result.AnalysisDate))
				logMu.Unlock()
				results <- analyzeJobResult{symbol: item.symbol}
			}
		}(i + 1)
	}

	go func() {
		defer close(jobs)
		for _, item := range companies {
			select {
			case <-ctx.Done():
				return
			case jobs <- item:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	total := 0
	failed := 0
	for result := range results {
		total++
		if result.err != nil {
			failed++
		}
	}
	log.Printf("analysis batch completed: total=%d success=%d failed=%d", total, total-failed, failed)
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("batch interrupted: %w", err)
	}
	return nil
}

func buildAnalyzeProvider(cfg taconfig.Config) (datasource.MarketDataProvider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "tradingview", "tv":
		return datasource.NewTradingViewProvider(), nil
	case "bist", "bistdb", "bist-db", "bist_bulletin_db", "bist-bulletin-db", "official", "official-bist":
		return datasource.NewBISTBulletinDBProvider(bistBulletinDBPath(cfg.DataDir)), nil
	case "mock":
		return datasource.NewMockProvider(), nil
	case "csv":
		return datasource.NewCSVProvider(cfg.DataDir), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q: %w", cfg.Provider, taconfig.ErrInvalidProvider)
	}
}

func bistBulletinDBPath(dataDir string) string {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = "data"
	}
	ext := strings.ToLower(filepath.Ext(dataDir))
	if ext == ".sqlite" || ext == ".sqlite3" || ext == ".db" {
		return dataDir
	}
	return filepath.Join(dataDir, "bist", "bist_ohlcv.sqlite")
}

func envFlag(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
