package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hissebot/internal/audit"
	appconfig "hissebot/internal/config"
	"hissebot/internal/domain/documents"
	"hissebot/internal/domain/kapextract"
	"hissebot/internal/extraction"
	"hissebot/internal/kapingest"
	"hissebot/internal/repositories/filedocuments"
	corestorage "hissebot/internal/storage"
	"hissebot/internal/ta/analysis"
	tachart "hissebot/internal/ta/chart"
	taconfig "hissebot/internal/ta/config"
	"hissebot/internal/ta/ohlcv"
	reportstorage "hissebot/internal/ta/storage"
)

type reportServerOptions struct {
	Addr       string
	Provider   string
	Mode       string
	Timeframes string
	Limit      int
	Timeout    time.Duration
}

type reportRequest struct {
	Symbol                string   `json:"symbol"`
	Provider              string   `json:"provider"`
	Mode                  string   `json:"mode"`
	Timeframes            []string `json:"timeframes"`
	Limit                 int      `json:"limit"`
	RequireEliteCandidate bool     `json:"require_elite_candidate"`
}

type reportResponse struct {
	Status                  string                `json:"status"`
	Symbol                  string                `json:"symbol"`
	AssetType               string                `json:"asset_type"`
	AnalysisDate            string                `json:"analysis_date"`
	OverallScore            float64               `json:"overall_score"`
	OverallBias             string                `json:"overall_bias"`
	RequireEliteCandidate   bool                  `json:"require_elite_candidate"`
	ReportQualityStatus     string                `json:"report_quality_status"`
	InvestmentQualityStatus string                `json:"investment_quality_status"`
	EliteCandidateStatus    string                `json:"elite_candidate_status"`
	EliteCandidateScore     float64               `json:"elite_candidate_score"`
	EliteCandidateLabel     string                `json:"elite_candidate_label"`
	EliteCandidateSummary   string                `json:"elite_candidate_summary"`
	TransactionUseStatus    string                `json:"transaction_use_status"`
	TransactionUseAnswer    string                `json:"transaction_use_answer"`
	PreflightStatus         string                `json:"preflight_status"`
	PreflightIssues         []string              `json:"preflight_issues,omitempty"`
	RequiredElitePasses     []string              `json:"required_elite_passes,omitempty"`
	FailedElitePasses       []string              `json:"failed_elite_passes,omitempty"`
	AnalysisGates           analysisGateResponses `json:"analysis_gates"`
	OutputDir               string                `json:"output_dir"`
	PDFPath                 string                `json:"pdf_path"`
	HTMLPath                string                `json:"html_path"`
	AnalysisPath            string                `json:"analysis_path"`
}

type analysisGateResponses struct {
	ValueInvesting analysisGateResponse `json:"value_investing"`
	Portfolio      analysisGateResponse `json:"portfolio"`
	TradingEdge    analysisGateResponse `json:"trading_edge"`
}

type analysisGateResponse struct {
	ReportQualityStatus  string  `json:"report_quality_status"`
	ReportQualityScore   float64 `json:"report_quality_score"`
	DecisionStatus       string  `json:"decision_status"`
	DecisionScore        float64 `json:"decision_score"`
	Takeaway             string  `json:"takeaway"`
	TransactionUseStatus string  `json:"transaction_use_status"`
	TransactionUseAnswer string  `json:"transaction_use_answer"`
}

func runReportServer(ctx context.Context, cfg appconfig.Config, store *corestorage.EquityStore, args []string) error {
	defaults := taconfig.Default()
	opts := reportServerOptions{
		Addr:       "127.0.0.1:1453",
		Provider:   defaults.Provider,
		Mode:       "production",
		Timeframes: "1D,1W,1M",
		Limit:      defaults.Limit,
		Timeout:    cfg.CommandTimeout,
	}
	fs := flag.NewFlagSet("serve reports", flag.ExitOnError)
	fs.StringVar(&opts.Addr, "addr", opts.Addr, "HTTP dinleme adresi")
	fs.StringVar(&opts.Provider, "provider", opts.Provider, "varsayılan veri kaynağı: tradingview, bistdb, csv veya mock")
	fs.StringVar(&opts.Mode, "mode", opts.Mode, "varsayılan analiz modu: research veya production")
	fs.StringVar(&opts.Timeframes, "timeframes", opts.Timeframes, "varsayılan zaman dilimleri")
	fs.IntVar(&opts.Limit, "limit", opts.Limit, "timeframe başına mum limiti")
	fs.DurationVar(&opts.Timeout, "timeout", opts.Timeout, "tek rapor üretim zaman aşımı")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateReportServerOptions(opts); err != nil {
		return err
	}
	if cfg.EndpointToken == "" && !isLocalAddr(opts.Addr) {
		return errors.New("non-local report server requires HISSEBOT_ENDPOINT_TOKEN")
	}

	server := &reportServer{
		cfg:   cfg,
		store: store,
		opts:  opts,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/reports", server.handleReports)
	mux.HandleFunc("/kap-ingest", server.handleKAPIngestPage)
	mux.HandleFunc("/api/kap-ingest/run", server.handleKAPIngestRun)
	mux.HandleFunc("/companies/", server.handleCompanyData)
	mux.HandleFunc("/documents/", server.handleDocumentData)
	mux.HandleFunc("/extraction/jobs", server.handleExtractionJobs)
	mux.HandleFunc("/extraction/jobs/", server.handleExtractionJobByID)

	httpServer := &http.Server{
		Addr:              opts.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	log.Printf("report server listening: http://%s", opts.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

type reportServer struct {
	cfg   appconfig.Config
	store *corestorage.EquityStore
	opts  reportServerOptions
}

func reportServerHTML(active string) string {
	if active != "kap" {
		active = "report"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Hissebot Operasyon</title>
<style>
:root{--bg:#f6f7f9;--panel:#fff;--ink:#20242a;--muted:#667085;--line:#d9dee8;--accent:#9e1f68;--accent2:#1f6feb;--soft:#f8e8f2}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.45 Arial,Helvetica,sans-serif}
header{background:var(--panel);border-bottom:1px solid var(--line);padding:18px 28px}h1{margin:0;color:var(--accent);font-size:22px;letter-spacing:0}
main{max-width:980px;margin:28px auto;padding:0 20px}.tabs{display:flex;gap:8px;margin-bottom:14px}.tab{border:1px solid var(--line);background:var(--panel);color:var(--ink);padding:10px 14px;border-radius:6px;cursor:pointer;font-weight:700}.tab.active{border-color:var(--accent);color:var(--accent);background:var(--soft)}
.panel{display:none;background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:20px}.panel.active{display:block}
.grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}label{display:block;color:var(--muted);font-size:12px;margin:0 0 6px}
input{width:100%%;padding:11px 12px;border:1px solid var(--line);border-radius:6px;font:inherit;background:#fff}.check{display:flex;align-items:center;gap:8px;color:var(--ink);font-size:14px}.check input{width:auto}
button.run{margin-top:16px;padding:11px 16px;border:0;border-radius:6px;background:var(--accent);color:#fff;font-weight:700;cursor:pointer}.secondary{background:var(--accent2)!important}
pre{white-space:pre-wrap;overflow:auto;max-height:420px;background:#101828;color:#f5f7fb;padding:14px;border-radius:6px;margin:16px 0 0}
@media(max-width:720px){.grid{grid-template-columns:1fr}.tabs{flex-wrap:wrap}}
</style></head>
<body><header><h1>Hissebot Operasyon</h1></header><main>
<div class="tabs"><button class="tab" data-tab="report">Rapor</button><button class="tab" data-tab="kap">KAP PDF Ingest</button></div>
<section id="panel-report" class="panel"><form id="reportForm"><div class="grid"><div><label>Sembol</label><input name="symbol" value="BORSK" placeholder="BORSK veya BTC" required></div><div><label>Zaman dilimleri</label><input name="timeframes" value="1D,1W,1M"></div></div><label class="check" style="margin-top:12px"><input name="require_elite_candidate" type="checkbox" checked>Üç veri kapısı geçmeden başarılı sayma</label><button class="run">Rapor Oluştur</button></form><pre id="reportOut"></pre></section>
<section id="panel-kap" class="panel"><form id="kapForm"><div class="grid"><div><label>Input</label><input name="input" value="data/equities"></div><div><label>Output</label><input name="output" value="data/processed"></div><div><label>Workers</label><input name="workers" type="number" min="1" max="32" value="4"></div><div><label>Limit</label><input name="limit" type="number" min="0" value="10"></div></div><label class="check" style="margin-top:12px"><input name="resume" type="checkbox" checked>Resume</label><label class="check"><input name="llm" type="checkbox">LLM extraction</label><label class="check"><input name="dry_run" type="checkbox">Dry run</label><button class="run secondary">PDF İşle</button></form><pre id="kapOut"></pre></section>
</main>
<script>
const active=%q;
function selectTab(name){document.querySelectorAll('.tab').forEach(x=>x.classList.toggle('active',x.dataset.tab===name));document.querySelectorAll('.panel').forEach(x=>x.classList.toggle('active',x.id==='panel-'+name));}
document.querySelectorAll('.tab').forEach(x=>x.addEventListener('click',()=>selectTab(x.dataset.tab)));selectTab(active);
document.getElementById('reportForm').addEventListener('submit', async (e)=>{e.preventDefault(); const fd=new FormData(e.target); const out=document.getElementById('reportOut'); out.textContent='Rapor hazırlanıyor...'; const res=await fetch('/reports',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({symbol:fd.get('symbol'),timeframes:String(fd.get('timeframes')).split(',').map(x=>x.trim()).filter(Boolean),require_elite_candidate:fd.get('require_elite_candidate')==='on'})}); out.textContent=JSON.stringify(await res.json(), null, 2);});
document.getElementById('kapForm').addEventListener('submit', async (e)=>{e.preventDefault(); const fd=new FormData(e.target); const out=document.getElementById('kapOut'); out.textContent='PDF ingest çalışıyor...'; const payload={input:fd.get('input'),output:fd.get('output'),workers:Number(fd.get('workers')||4),limit:Number(fd.get('limit')||0),resume:fd.get('resume')==='on',llm:fd.get('llm')==='on',dry_run:fd.get('dry_run')==='on'}; const res=await fetch('/api/kap-ingest/run',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)}); out.textContent=JSON.stringify(await res.json(), null, 2);});
</script></body></html>`, active)
}

func (s *reportServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(reportServerHTML("report")))
}

func (s *reportServer) handleKAPIngestPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/kap-ingest" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(reportServerHTML("kap")))
}

func (s *reportServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeReportJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *reportServer) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST")
		writeReportError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET veya POST kullanın")
		return
	}
	if !s.authorized(r) {
		writeReportError(w, http.StatusUnauthorized, "unauthorized", "geçerli token gerekli")
		return
	}
	req, err := s.decodeReportRequest(r)
	if err != nil {
		writeReportError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.opts.Timeout)
	defer cancel()
	response, err := s.generate(ctx, req)
	if err != nil {
		writeReportError(w, http.StatusInternalServerError, "report_failed", err.Error())
		return
	}
	writeReportJSON(w, http.StatusOK, response)
}

type kapIngestRequest struct {
	InputDir  string `json:"input"`
	OutputDir string `json:"output"`
	Workers   int    `json:"workers"`
	Limit     int    `json:"limit"`
	Resume    *bool  `json:"resume"`
	LLM       bool   `json:"llm"`
	DryRun    bool   `json:"dry_run"`
}

func (s *reportServer) handleKAPIngestRun(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeReportError(w, http.StatusUnauthorized, "unauthorized", "geçerli token gerekli")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeReportError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST kullanın")
		return
	}
	var req kapIngestRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeReportError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if strings.TrimSpace(req.InputDir) == "" {
		req.InputDir = s.store.Root()
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		req.OutputDir = filepath.Join(filepath.Dir(s.store.Root()), "processed")
	}
	if req.Workers <= 0 {
		req.Workers = kapingest.DefaultWorkers
	}
	resume := true
	if req.Resume != nil {
		resume = *req.Resume
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.CommandTimeout)
	defer cancel()
	summary, err := kapingest.Run(ctx, kapingest.Options{
		InputDir:  req.InputDir,
		OutputDir: req.OutputDir,
		Workers:   req.Workers,
		Limit:     req.Limit,
		Resume:    resume,
		LLM:       req.LLM,
		DryRun:    req.DryRun,
	})
	if err != nil {
		writeReportJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"code":    "kap_ingest_failed",
			"message": err.Error(),
			"summary": summary,
		})
		return
	}
	writeReportJSON(w, http.StatusOK, summary)
}

func (s *reportServer) handleCompanyData(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeReportError(w, http.StatusUnauthorized, "unauthorized", "geçerli token gerekli")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "companies" {
		http.NotFound(w, r)
		return
	}
	ticker := ohlcv.NormalizeSymbol(parts[1])
	if ticker == "" {
		writeReportError(w, http.StatusBadRequest, "bad_request", "ticker zorunlu")
		return
	}
	result, err := s.loadExtractionResult(ticker)
	if err != nil {
		writeReportError(w, http.StatusNotFound, "kap_extraction_not_found", err.Error())
		return
	}
	switch parts[2] {
	case "info-card":
		writeReportJSON(w, http.StatusOK, result.CompanyInfoCard)
	case "financials":
		if len(parts) == 4 {
			period := strings.ToUpper(strings.TrimSpace(parts[3]))
			filtered := make([]kapextract.FinancialFact, 0, len(result.FinancialFacts))
			for _, fact := range result.FinancialFacts {
				if strings.EqualFold(fact.Period, period) {
					filtered = append(filtered, fact)
				}
			}
			writeReportJSON(w, http.StatusOK, filtered)
			return
		}
		writeReportJSON(w, http.StatusOK, result.FinancialFacts)
	case "management":
		writeReportJSON(w, http.StatusOK, result.People)
	case "assets":
		if len(parts) == 5 && parts[4] == "evidence-chain" {
			assetID := parts[3]
			assetName := ""
			for _, asset := range result.TrackedAssets {
				if asset.AssetID == assetID {
					assetName = asset.AssetName
					break
				}
			}
			chains := make([]kapextract.EvidenceChain, 0, len(result.EvidenceChains))
			for _, chain := range result.EvidenceChains {
				if chain.EvidenceChainID == assetID || chain.Subject == assetName {
					chains = append(chains, chain)
				}
			}
			writeReportJSON(w, http.StatusOK, chains)
			return
		}
		writeReportJSON(w, http.StatusOK, result.TrackedAssets)
	case "events":
		writeReportJSON(w, http.StatusOK, result.CorporateEvents)
	case "risks":
		writeReportJSON(w, http.StatusOK, map[string]any{
			"ticker":             ticker,
			"review_required":    result.FundamentalAnalysis.ReviewRequired,
			"red_flags":          result.FundamentalAnalysis.RedFlags,
			"human_review_queue": result.HumanReviewQueue,
			"warnings":           result.Warnings,
		})
	case "analysis":
		if len(parts) >= 4 && parts[3] == "fundamental" {
			writeReportJSON(w, http.StatusOK, result.FundamentalAnalysis)
			return
		}
		http.NotFound(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *reportServer) handleDocumentData(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeReportError(w, http.StatusUnauthorized, "unauthorized", "geçerli token gerekli")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "documents" {
		http.NotFound(w, r)
		return
	}
	doc, ok, err := s.findDocument(parts[1])
	if err != nil {
		writeReportError(w, http.StatusInternalServerError, "registry_read_failed", err.Error())
		return
	}
	if !ok {
		writeReportError(w, http.StatusNotFound, "document_not_found", "belge registry içinde bulunamadı")
		return
	}
	if len(parts) == 3 && parts[2] == "sources" {
		result, err := s.loadExtractionResult(doc.Ticker)
		if err != nil {
			writeReportError(w, http.StatusNotFound, "kap_extraction_not_found", err.Error())
			return
		}
		writeReportJSON(w, http.StatusOK, sourcesForDocument(result, doc.DocumentID))
		return
	}
	writeReportJSON(w, http.StatusOK, doc)
}

func (s *reportServer) handleExtractionJobs(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeReportError(w, http.StatusUnauthorized, "unauthorized", "geçerli token gerekli")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeReportError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST kullanın")
		return
	}
	var req struct {
		Ticker           string `json:"ticker"`
		Limit            int    `json:"limit"`
		IncludeNonLatest bool   `json:"include_non_latest"`
		MaxChars         int    `json:"max_chars"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeReportError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	repo, err := filedocuments.New(filepath.Join(s.store.Root(), "_kap"))
	if err != nil {
		writeReportError(w, http.StatusInternalServerError, "registry_read_failed", err.Error())
		return
	}
	processor := extraction.Processor{
		Repository:          repo,
		EquitiesDir:         s.store.Root(),
		MaxCharsPerDocument: req.MaxChars,
	}
	result, err := processor.ProcessBatch(r.Context(), extraction.Options{
		Ticker:           req.Ticker,
		Limit:            req.Limit,
		IncludeNonLatest: req.IncludeNonLatest,
	})
	if err != nil {
		writeReportError(w, http.StatusInternalServerError, "extraction_failed", err.Error())
		return
	}
	writeReportJSON(w, http.StatusOK, result)
}

func (s *reportServer) handleExtractionJobByID(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeReportError(w, http.StatusUnauthorized, "unauthorized", "geçerli token gerekli")
		return
	}
	jobID := strings.TrimPrefix(r.URL.Path, "/extraction/jobs/")
	if jobID == "" {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.store.Root(), "_kap", "extraction_jobs.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		writeReportError(w, http.StatusNotFound, "job_not_found", "extraction job dosyası bulunamadı")
		return
	}
	var jobs []documents.IngestionJob
	if err := json.Unmarshal(raw, &jobs); err != nil {
		writeReportError(w, http.StatusInternalServerError, "job_read_failed", err.Error())
		return
	}
	for _, job := range jobs {
		if job.JobID == jobID {
			writeReportJSON(w, http.StatusOK, job)
			return
		}
	}
	writeReportError(w, http.StatusNotFound, "job_not_found", "job_id bulunamadı")
}

func (s *reportServer) decodeReportRequest(r *http.Request) (reportRequest, error) {
	req := reportRequest{
		Provider: s.opts.Provider,
		Mode:     s.opts.Mode,
		Limit:    s.opts.Limit,
	}
	if r.Method == http.MethodPost {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return reportRequest{}, fmt.Errorf("JSON okunamadı: %w", err)
		}
	}
	q := r.URL.Query()
	if value := q.Get("symbol"); value != "" {
		req.Symbol = value
	}
	if value := q.Get("provider"); value != "" {
		req.Provider = value
	}
	if value := q.Get("mode"); value != "" {
		req.Mode = value
	}
	if value := q.Get("timeframes"); value != "" {
		req.Timeframes = splitCSV(value)
	}
	if value := q.Get("require_elite_candidate"); value != "" {
		requireElite, err := parseReportBool(value)
		if err != nil {
			return reportRequest{}, err
		}
		req.RequireEliteCandidate = requireElite
	}
	if req.Symbol = ohlcv.NormalizeSymbol(req.Symbol); req.Symbol == "" {
		return reportRequest{}, errors.New("symbol zorunlu")
	}
	req.Provider = firstNonEmpty(strings.ToLower(strings.TrimSpace(req.Provider)), s.opts.Provider)
	req.Mode = firstNonEmpty(strings.ToLower(strings.TrimSpace(req.Mode)), s.opts.Mode)
	if req.Mode != "research" && req.Mode != "production" {
		return reportRequest{}, fmt.Errorf("mode geçersiz: %s", req.Mode)
	}
	if len(req.Timeframes) == 0 {
		req.Timeframes = splitCSV(s.opts.Timeframes)
	}
	if req.Limit <= 0 {
		req.Limit = s.opts.Limit
	}
	return req, nil
}

func (s *reportServer) loadExtractionResult(ticker string) (kapextract.ExtractionResult, error) {
	ticker = ohlcv.NormalizeSymbol(ticker)
	path := filepath.Join(s.store.Root(), ticker, "kap", "extraction", "extraction_result.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return kapextract.ExtractionResult{}, fmt.Errorf("%s için KAP çıkarım sonucu yok; önce sync kap-extract -ticker %s çalıştırın", ticker, ticker)
	}
	var result kapextract.ExtractionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return kapextract.ExtractionResult{}, err
	}
	return result, nil
}

func (s *reportServer) findDocument(documentID string) (documents.DocumentMetadata, bool, error) {
	repo, err := filedocuments.New(filepath.Join(s.store.Root(), "_kap"))
	if err != nil {
		return documents.DocumentMetadata{}, false, err
	}
	docs, err := repo.ListDocuments(context.Background(), "")
	if err != nil {
		return documents.DocumentMetadata{}, false, err
	}
	for _, doc := range docs {
		if doc.DocumentID == documentID {
			return doc, true, nil
		}
	}
	return documents.DocumentMetadata{}, false, nil
}

func sourcesForDocument(result kapextract.ExtractionResult, documentID string) map[string]any {
	blocks := []kapextract.TextBlock{}
	tables := []kapextract.DocumentTable{}
	facts := []kapextract.FinancialFact{}
	people := []kapextract.PersonExtraction{}
	events := []kapextract.CorporateEvent{}
	reviews := []kapextract.ReviewItem{}
	for _, block := range result.TextBlocks {
		if block.DocumentID == documentID {
			blocks = append(blocks, block)
		}
	}
	for _, table := range result.Tables {
		if table.DocumentID == documentID {
			tables = append(tables, table)
		}
	}
	for _, fact := range result.FinancialFacts {
		if fact.Source.SourceDocumentID == documentID {
			facts = append(facts, fact)
		}
	}
	for _, person := range result.People {
		if person.Source.SourceDocumentID == documentID {
			people = append(people, person)
		}
	}
	for _, event := range result.CorporateEvents {
		if event.Source.SourceDocumentID == documentID {
			events = append(events, event)
		}
	}
	for _, review := range result.HumanReviewQueue {
		if review.Source.SourceDocumentID == documentID {
			reviews = append(reviews, review)
		}
	}
	return map[string]any{
		"document_id":        documentID,
		"text_blocks":        blocks,
		"document_tables":    tables,
		"financial_facts":    facts,
		"people":             people,
		"corporate_events":   events,
		"human_review_queue": reviews,
	}
}

func (s *reportServer) generate(ctx context.Context, req reportRequest) (reportResponse, error) {
	timeframes, err := taconfig.ParseTimeframes(strings.Join(req.Timeframes, ","))
	if err != nil {
		return reportResponse{}, fmt.Errorf("timeframes: %w", err)
	}
	cfg := taconfig.Default()
	cfg.Provider = req.Provider
	cfg.OutputDir = s.store.Root()
	cfg.Timeframes = timeframes
	cfg.Limit = req.Limit
	cfg.DataMode = req.Mode
	provider, err := buildAnalyzeProvider(cfg)
	if err != nil {
		return reportResponse{}, err
	}
	engine := analysis.NewEngine(provider, tachart.NewPNGRenderer(), analysis.EngineOptions{
		Timeframes:               cfg.Timeframes,
		Limit:                    cfg.Limit,
		EquitiesDir:              cfg.OutputDir,
		DataMode:                 cfg.DataMode,
		BenchmarkSymbol:          cfg.Benchmark,
		ValuationAssumptionsFile: cfg.ValuationAssumptionsFile,
		MacroGDPFile:             s.cfg.TUIKGDPFile,
		PortfolioValue:           cfg.Portfolio,
		RiskPerTradePct:          cfg.RiskPct,
		PeerLimit:                cfg.PeerLimit,
		SkipKAPPDFIngest:         envFlag("HISSEBOT_SKIP_KAP_PDF_INGEST"),
	})
	result, err := engine.AnalyzeSymbol(ctx, analysis.SymbolRequest{Symbol: req.Symbol})
	if err != nil {
		return reportResponse{}, fmt.Errorf("analyze %s: %w", req.Symbol, err)
	}
	writer := reportstorage.NewReportWriter()
	if err := writer.WriteAnalysis(ctx, cfg.OutputDir, result); err != nil {
		return reportResponse{}, fmt.Errorf("write %s: %w", result.Symbol, err)
	}
	outputDir := reportstorage.AnalysisDirForAsset(cfg.OutputDir, result.AssetType, result.Symbol, result.AnalysisDate)
	views := result.InvestorQA.InstitutionalViews
	preflightStatus, preflightIssues := reportPreflight(result)
	responseStatus := "ok"
	if preflightStatus == "fail" {
		responseStatus = "rejected"
	} else if req.RequireEliteCandidate && views.EliteCandidate.Status != "pass" {
		responseStatus = "rejected"
	}
	_, _ = audit.Append(s.cfg.AuditLogPath, audit.Event{
		Action:   "report_endpoint_written",
		Entity:   "analysis_result",
		EntityID: result.Symbol,
		Details: map[string]any{
			"analysis_date":             result.AnalysisDate,
			"asset_type":                result.AssetType,
			"provider":                  req.Provider,
			"timeframes":                timeframes,
			"endpoint_status":           responseStatus,
			"require_elite_candidate":   req.RequireEliteCandidate,
			"elite_candidate_status":    views.EliteCandidate.Status,
			"transaction_use_status":    views.FinancialTransactionUse.Status,
			"report_quality_status":     views.OverallQualityStatus,
			"investment_quality_status": views.OverallStatus,
			"preflight_status":          preflightStatus,
			"preflight_issues":          preflightIssues,
		},
	})
	return reportResponse{
		Status:                  responseStatus,
		Symbol:                  result.Symbol,
		AssetType:               result.AssetType,
		AnalysisDate:            result.AnalysisDate,
		OverallScore:            result.OverallScore,
		OverallBias:             result.OverallBias,
		RequireEliteCandidate:   req.RequireEliteCandidate,
		ReportQualityStatus:     views.OverallQualityStatus,
		InvestmentQualityStatus: views.OverallStatus,
		EliteCandidateStatus:    views.EliteCandidate.Status,
		EliteCandidateScore:     views.EliteCandidate.Score,
		EliteCandidateLabel:     views.EliteCandidate.Label,
		EliteCandidateSummary:   views.EliteCandidate.Summary,
		TransactionUseStatus:    views.FinancialTransactionUse.Status,
		TransactionUseAnswer:    views.FinancialTransactionUse.Answer,
		PreflightStatus:         preflightStatus,
		PreflightIssues:         preflightIssues,
		RequiredElitePasses:     views.EliteCandidate.RequiredPasses,
		FailedElitePasses:       views.EliteCandidate.FailedPasses,
		AnalysisGates: analysisGateResponses{
			ValueInvesting: analysisGateResponse{
				ReportQualityStatus:  views.ValueInvesting.ReportQualityStatus,
				ReportQualityScore:   views.ValueInvesting.ReportQualityScore,
				DecisionStatus:       views.ValueInvesting.Status,
				DecisionScore:        views.ValueInvesting.Score,
				Takeaway:             views.ValueInvesting.Takeaway,
				TransactionUseStatus: views.ValueInvesting.TransactionUseStatus,
				TransactionUseAnswer: views.ValueInvesting.TransactionUseAnswer,
			},
			Portfolio: analysisGateResponse{
				ReportQualityStatus:  views.Portfolio.ReportQualityStatus,
				ReportQualityScore:   views.Portfolio.ReportQualityScore,
				DecisionStatus:       views.Portfolio.Status,
				DecisionScore:        views.Portfolio.Score,
				Takeaway:             views.Portfolio.Takeaway,
				TransactionUseStatus: views.Portfolio.TransactionUseStatus,
				TransactionUseAnswer: views.Portfolio.TransactionUseAnswer,
			},
			TradingEdge: analysisGateResponse{
				ReportQualityStatus:  views.TradingEdge.ReportQualityStatus,
				ReportQualityScore:   views.TradingEdge.ReportQualityScore,
				DecisionStatus:       views.TradingEdge.Status,
				DecisionScore:        views.TradingEdge.Score,
				Takeaway:             views.TradingEdge.Takeaway,
				TransactionUseStatus: views.TradingEdge.TransactionUseStatus,
				TransactionUseAnswer: views.TradingEdge.TransactionUseAnswer,
			},
		},
		OutputDir:    outputDir,
		PDFPath:      filepath.Join(outputDir, "rapor.pdf"),
		HTMLPath:     filepath.Join(outputDir, "rapor.html"),
		AnalysisPath: filepath.Join(outputDir, "analysis.json"),
	}, nil
}

func reportPreflight(result analysis.SymbolAnalysis) (string, []string) {
	issues := []string{}
	if result.Professional.EvidencePolicy.Status == "blocked" {
		issues = append(issues, result.Professional.EvidencePolicy.BlockingIssues...)
	}
	if result.PriceQuality == nil {
		issues = append(issues, "price_quality_report_missing")
	} else if !result.PriceQuality.ReadyForDecision {
		if len(result.PriceQuality.BlockingReasons) > 0 {
			for _, reason := range result.PriceQuality.BlockingReasons {
				issues = append(issues, "price_quality:"+reason)
			}
		} else {
			issues = append(issues, "price_quality_not_ready_for_decision")
		}
	}
	if result.DecisionClassification.Status != "" && result.DecisionClassification.Status != "decision_ready" {
		issues = append(issues, "decision_classification:"+result.DecisionClassification.Status)
	}
	issues = uniqueReportStrings(issues)
	if len(issues) > 0 {
		return "fail", issues
	}
	return "pass", nil
}

func uniqueReportStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *reportServer) authorized(r *http.Request) bool {
	if s.cfg.EndpointToken == "" {
		return true
	}
	token := strings.TrimSpace(r.Header.Get("X-Hissebot-Token"))
	if token == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		token = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return token == s.cfg.EndpointToken
}

func validateReportServerOptions(opts reportServerOptions) error {
	if _, _, err := net.SplitHostPort(opts.Addr); err != nil {
		return fmt.Errorf("addr geçersiz: %w", err)
	}
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode != "research" && mode != "production" {
		return fmt.Errorf("mode geçersiz: %s", opts.Mode)
	}
	if _, err := taconfig.ParseTimeframes(opts.Timeframes); err != nil {
		return err
	}
	return nil
}

func isLocalAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeReportJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func writeReportError(w http.ResponseWriter, status int, code string, message string) {
	writeReportJSON(w, status, map[string]string{
		"status":  "error",
		"code":    code,
		"message": message,
	})
}

func parseReportBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "t", "yes", "y", "on", "evet":
		return true, nil
	case "0", "false", "f", "no", "n", "off", "hayir", "hayır":
		return false, nil
	default:
		return false, fmt.Errorf("bool değer geçersiz: %s", value)
	}
}
