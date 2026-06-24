package professional

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/domain"
	"hissebot/internal/domain/marketdata"
	eventbacktest "hissebot/internal/ta/backtest"
	"hissebot/internal/ta/corporateactions"
	"hissebot/internal/ta/docintel"
	"hissebot/internal/ta/localize"
	"hissebot/internal/ta/macro"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/value"
	"hissebot/internal/ta/vapcontext"
	"hissebot/internal/util"
	"hissebot/pkg/mathutil"
)

type Options struct {
	BenchmarkSymbol            string
	DataMode                   string
	RequireVerifiedPublishDate bool
	ValuationAssumptionsFile   string
	MacroGDPFile               string
	TCMBDir                    string
	TCMBEVDSDir                string
	VAPDir                     string
	VAPIndexPortfolioFile      string
	MarketSnapshotFile         string
	PortfolioValue             float64
	RiskPerTradePct            float64
	PeerLimit                  int
	CommissionBps              float64
	SlippageBps                float64
	SkipKAPPDFIngest           bool
}

type SymbolInput struct {
	EquitiesDir            string
	Symbol                 string
	CompanyName            string
	Currency               string
	AssetType              string
	AsOf                   time.Time
	LastClose              float64
	DailyCandles           []ohlcv.Candle
	BenchmarkCandles       []ohlcv.Candle
	BenchmarkError         string
	SectorBenchmarkSymbol  string
	SectorBenchmarkCandles []ohlcv.Candle
	SectorBenchmarkError   string
	OfficialBISTCandles    []ohlcv.Candle
	OfficialBISTError      string
	CorporateActions       corporateactions.ActionSet
	Options                Options
}

type TimeframeInput struct {
	Timeframe           string
	Candles             []ohlcv.Candle
	Indicators          ohlcv.IndicatorSnapshot
	IndicatorSignals    []ohlcv.IndicatorResult
	Patterns            []ohlcv.PatternResult
	PatternScans        []ohlcv.PatternScanResult
	TechnicalValidation TechnicalValidationReport
	TradePlan           ohlcv.TradePlan
	LastClose           float64
	LastVolume          float64
	CorporateActions    corporateactions.ActionSet
	PriceAdjustment     corporateactions.AdjustmentReport
}

type Report struct {
	Coverage            CoverageReport                       `json:"coverage"`
	EvidencePolicy      EvidencePolicyReport                 `json:"evidence_policy"`
	Company             CompanyProfile                       `json:"company"`
	Market              MarketContext                        `json:"market_context"`
	Valuation           ValuationAnalysis                    `json:"valuation"`
	SectorFinancials    SectorFinancialAnalysis              `json:"sector_financials"`
	KAPPDFIngest        KAPPDFIngestSummary                  `json:"kap_pdf_ingest"`
	KAPAssetInventory   KAPAssetInventorySummary             `json:"kap_asset_inventory"`
	CorporateActions    corporateactions.ActionSet           `json:"corporate_actions"`
	Peers               PeerComparison                       `json:"peer_comparison"`
	Disclosure          DisclosureReview                     `json:"disclosure_review"`
	NewsSentiment       EquityNewsSentimentReport            `json:"news_sentiment,omitempty"`
	Scenarios           []Scenario                           `json:"scenarios"`
	DataGovernance      FinancialDataGovernance              `json:"data_governance"`
	FundamentalBacktest eventbacktest.FundamentalEventResult `json:"fundamental_backtest"`
	ValueInvesting      value.Report                         `json:"value_investing"`
	InvestmentResearch  InvestmentResearchReview             `json:"investment_research"`
	CryptoContext       CryptoContextReport                  `json:"crypto_context,omitempty"`
	CommodityContext    CommodityContextReport               `json:"commodity_context,omitempty"`
	TCMBContext         TCMBContextReport                    `json:"tcmb_context,omitempty"`
	TCMBEVDSContext     TCMBEVDSContextReport                `json:"tcmb_evds_context,omitempty"`
	VAPFreeFloat        vapcontext.FreeFloatReport           `json:"vap_free_float,omitempty"`
	VAPIndexPortfolio   vapcontext.IndexPortfolioReport      `json:"vap_index_portfolio,omitempty"`
	BISTOfficial        BISTOfficialContext                  `json:"bist_official_data,omitempty"`
	RawKAPData          *KAPRawDataBundle                    `json:"raw_kap_data,omitempty"`
	DataQuality         float64                              `json:"data_quality"`
}

type TimeframeReport struct {
	Liquidity       LiquidityProfile      `json:"liquidity"`
	PositionSizing  PositionSizing        `json:"position_sizing"`
	Backtest        BacktestResult        `json:"backtest"`
	SignalStats     SignalStats           `json:"signal_stats"`
	Technical       TechnicalEvidence     `json:"technical_evidence"`
	PriceAdjustment PriceAdjustmentReview `json:"price_adjustment_review"`
}

type PriceAdjustmentReview struct {
	AdjustedCandles       int      `json:"adjusted_candles"`
	UnadjustedCandles     int      `json:"unadjusted_candles"`
	PotentialSplitGapBars int      `json:"potential_split_gap_bars"`
	PriceSeries           string   `json:"price_series,omitempty"`
	ActionsConsidered     int      `json:"actions_considered,omitempty"`
	AppliedActions        int      `json:"applied_actions,omitempty"`
	SkippedActions        int      `json:"skipped_actions,omitempty"`
	BacktestSafe          bool     `json:"backtest_safe"`
	Warnings              []string `json:"warnings,omitempty"`
}

type TechnicalEvidence struct {
	Summary            string                    `json:"summary"`
	Score              TechnicalScore            `json:"score"`
	SelectedIndicators []TechnicalIndicator      `json:"selected_indicators"`
	SelectedPatterns   []TechnicalPattern        `json:"selected_patterns"`
	Validation         TechnicalValidationReport `json:"validation"`
	SignalGate         TechnicalSignalGate       `json:"signal_gate"`
	SignalCounts       map[string]int            `json:"signal_counts"`
	PatternCounts      map[string]int            `json:"pattern_counts"`
	Guardrails         []string                  `json:"guardrails"`
}

type TechnicalValidationReport struct {
	Status                    string   `json:"status,omitempty"`
	Score                     float64  `json:"score,omitempty"`
	GateEligible              bool     `json:"gate_eligible"`
	IndicatorFormulaStatus    string   `json:"indicator_formula_status,omitempty"`
	IndicatorChecked          int      `json:"indicator_checked,omitempty"`
	IndicatorComputed         int      `json:"indicator_computed,omitempty"`
	IndicatorProxyOnly        int      `json:"indicator_proxy_only,omitempty"`
	IndicatorExternalRequired int      `json:"indicator_external_required,omitempty"`
	IndicatorWarnings         int      `json:"indicator_warnings,omitempty"`
	IndicatorErrors           int      `json:"indicator_errors,omitempty"`
	PatternStatus             string   `json:"pattern_status,omitempty"`
	PatternChecked            int      `json:"pattern_checked,omitempty"`
	PatternConfirmed          int      `json:"pattern_confirmed,omitempty"`
	PatternCandidates         int      `json:"pattern_candidates,omitempty"`
	PatternDrawn              int      `json:"pattern_drawn,omitempty"`
	PatternNotDrawn           int      `json:"pattern_not_drawn,omitempty"`
	PatternWarnings           int      `json:"pattern_warnings,omitempty"`
	PatternErrors             int      `json:"pattern_errors,omitempty"`
	GeometryPatternDrawings   int      `json:"geometry_pattern_drawings,omitempty"`
	ChartOverlayStatus        string   `json:"chart_overlay_status,omitempty"`
	Summary                   string   `json:"summary,omitempty"`
	Blockers                  []string `json:"blockers,omitempty"`
	Evidence                  []string `json:"evidence,omitempty"`
}

type TechnicalSignalGate struct {
	Status               string   `json:"status"`
	Actionable           bool     `json:"actionable"`
	Direction            string   `json:"direction"`
	Label                string   `json:"label"`
	Score                float64  `json:"score"`
	Timeframe            string   `json:"timeframe,omitempty"`
	EntryMin             float64  `json:"entry_min,omitempty"`
	EntryMax             float64  `json:"entry_max,omitempty"`
	StopLoss             float64  `json:"stop_loss,omitempty"`
	Target1              float64  `json:"target1,omitempty"`
	Target2              float64  `json:"target2,omitempty"`
	RiskRewardRatio      float64  `json:"risk_reward_ratio,omitempty"`
	VolumeConfirmed      bool     `json:"volume_confirmed"`
	VolumeConfirmation   string   `json:"volume_confirmation"`
	PriceStructure       string   `json:"price_structure"`
	BacktestSummary      string   `json:"backtest_summary"`
	ConfirmingIndicators []string `json:"confirming_indicators,omitempty"`
	ConfirmingPatterns   []string `json:"confirming_patterns,omitempty"`
	ConflictingSignals   []string `json:"conflicting_signals,omitempty"`
	Passes               []string `json:"passes,omitempty"`
	Blockers             []string `json:"blockers,omitempty"`
	Evidence             []string `json:"evidence,omitempty"`
}

type TechnicalScore struct {
	Trend          float64 `json:"trend"`
	Momentum       float64 `json:"momentum"`
	Volume         float64 `json:"volume"`
	VolatilityRisk float64 `json:"volatility_risk"`
	Pattern        float64 `json:"pattern"`
	Total          float64 `json:"total"`
}

type TechnicalIndicator struct {
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Group      string   `json:"group"`
	Signal     string   `json:"signal"`
	Value      float64  `json:"value"`
	Confidence float64  `json:"confidence"`
	Source     string   `json:"source"`
	Evidence   []string `json:"evidence"`
}

type TechnicalPattern struct {
	Name                 string   `json:"name"`
	Category             string   `json:"category"`
	Direction            string   `json:"direction"`
	Confidence           float64  `json:"confidence"`
	StartIndex           int      `json:"start_index"`
	EndIndex             int      `json:"end_index"`
	VolumeConfirmed      bool     `json:"volume_confirmed"`
	VolumeConfirmation   string   `json:"volume_confirmation,omitempty"`
	ConfirmingIndicators []string `json:"confirming_indicators,omitempty"`
	InvalidatingSignals  []string `json:"invalidating_signals,omitempty"`
	TradeValue           string   `json:"trade_value,omitempty"`
	ValidationStatus     string   `json:"validation_status,omitempty"`
	ValidationMethod     string   `json:"validation_method,omitempty"`
	ValidationPValue     float64  `json:"validation_p_value,omitempty"`
	ValidationCILow      float64  `json:"validation_ci_low,omitempty"`
	ValidationCIHigh     float64  `json:"validation_ci_high,omitempty"`
	Source               string   `json:"source,omitempty"`
	Evidence             []string `json:"evidence"`
}

type CoverageReport struct {
	Score     float64  `json:"score"`
	Available []string `json:"available"`
	Missing   []string `json:"missing"`
	Warnings  []string `json:"warnings"`
}

type CryptoContextReport struct {
	Computed        bool                 `json:"computed"`
	Path            string               `json:"path,omitempty"`
	AsOf            time.Time            `json:"as_of,omitempty"`
	OnChain         CryptoContextSection `json:"onchain"`
	Derivatives     CryptoContextSection `json:"derivatives"`
	ExchangeFlow    CryptoContextSection `json:"exchange_flow"`
	NewsSentiment   CryptoContextSection `json:"news_sentiment"`
	RequiredActions []string             `json:"required_actions,omitempty"`
	Warnings        []string             `json:"warnings,omitempty"`
}

type CryptoContextSection struct {
	Available bool               `json:"available"`
	Source    string             `json:"source,omitempty"`
	Score     float64            `json:"score,omitempty"`
	Summary   string             `json:"summary,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	Warnings  []string           `json:"warnings,omitempty"`
}

type CommodityContextReport struct {
	Computed                    bool                    `json:"computed"`
	Path                        string                  `json:"path,omitempty"`
	AsOf                        time.Time               `json:"as_of,omitempty"`
	Macro                       CommodityContextSection `json:"macro"`
	FuturesPositioning          CommodityContextSection `json:"futures_positioning"`
	GoldETFPhysicalFlow         CommodityContextSection `json:"gold_etf_physical_flow"`
	CentralBankGeopoliticalNews CommodityContextSection `json:"central_bank_geopolitical_news"`
	RequiredActions             []string                `json:"required_actions,omitempty"`
	Warnings                    []string                `json:"warnings,omitempty"`
}

type CommodityContextSection struct {
	Available bool               `json:"available"`
	Source    string             `json:"source,omitempty"`
	Score     float64            `json:"score,omitempty"`
	Summary   string             `json:"summary,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	Warnings  []string           `json:"warnings,omitempty"`
}

type EvidencePolicyReport struct {
	Mode                    string   `json:"mode"`
	Status                  string   `json:"status"`
	Strict                  bool     `json:"strict"`
	FactualDataAllowed      bool     `json:"factual_data_allowed"`
	ValuationTargetsAllowed bool     `json:"valuation_targets_allowed"`
	ScenarioTargetsAllowed  bool     `json:"scenario_targets_allowed"`
	RecommendationAllowed   bool     `json:"recommendation_allowed"`
	TechnicalPlanAllowed    bool     `json:"technical_plan_allowed"`
	BlockingIssues          []string `json:"blocking_issues,omitempty"`
	Limitations             []string `json:"limitations,omitempty"`
	SuppressedOutputs       []string `json:"suppressed_outputs,omitempty"`
	RequiredEvidence        []string `json:"required_evidence,omitempty"`
	Notes                   []string `json:"notes,omitempty"`
}

type CompanyProfile struct {
	Symbol                   string   `json:"symbol"`
	Name                     string   `json:"name"`
	Sector                   string   `json:"sector"`
	Industry                 string   `json:"industry,omitempty"`
	PeerGroup                string   `json:"peer_group,omitempty"`
	PeerSymbols              []string `json:"peer_symbols,omitempty"`
	SectorSource             string   `json:"sector_source"`
	ClassificationConfidence float64  `json:"classification_confidence,omitempty"`
	ClassificationWarnings   []string `json:"classification_warnings,omitempty"`
	PaidCapital              float64  `json:"paid_capital"`
	RegisteredCapitalCeiling float64  `json:"registered_capital_ceiling"`
}

type MarketContext struct {
	BenchmarkSymbol          string                         `json:"benchmark_symbol"`
	BenchmarkAvailable       bool                           `json:"benchmark_available"`
	BenchmarkError           string                         `json:"benchmark_error,omitempty"`
	SectorBenchmarkSymbol    string                         `json:"sector_benchmark_symbol,omitempty"`
	SectorBenchmarkAvailable bool                           `json:"sector_benchmark_available,omitempty"`
	SectorBenchmarkError     string                         `json:"sector_benchmark_error,omitempty"`
	StockReturn20            float64                        `json:"stock_return_20"`
	StockReturn60            float64                        `json:"stock_return_60"`
	StockReturn120           float64                        `json:"stock_return_120"`
	BenchmarkReturn20        float64                        `json:"benchmark_return_20"`
	BenchmarkReturn60        float64                        `json:"benchmark_return_60"`
	BenchmarkReturn120       float64                        `json:"benchmark_return_120"`
	RelativeStrength20       float64                        `json:"relative_strength_20"`
	RelativeStrength60       float64                        `json:"relative_strength_60"`
	SectorBenchmarkReturn20  float64                        `json:"sector_benchmark_return_20,omitempty"`
	SectorBenchmarkReturn60  float64                        `json:"sector_benchmark_return_60,omitempty"`
	SectorBenchmarkReturn120 float64                        `json:"sector_benchmark_return_120,omitempty"`
	SectorRelativeStrength20 float64                        `json:"sector_relative_strength_20,omitempty"`
	SectorRelativeStrength60 float64                        `json:"sector_relative_strength_60,omitempty"`
	SectorAlpha60            float64                        `json:"sector_alpha_60,omitempty"`
	SectorBeta60             float64                        `json:"sector_beta_60,omitempty"`
	SectorCorrelation60      float64                        `json:"sector_correlation_60,omitempty"`
	Alpha60                  float64                        `json:"alpha_60"`
	Beta60                   float64                        `json:"beta_60"`
	Correlation60            float64                        `json:"correlation_60"`
	GDP                      macro.GDPContext               `json:"gdp,omitempty"`
	LiveSnapshot             *marketdata.LiveMarketSnapshot `json:"live_snapshot,omitempty"`
	Microstructure           *MarketMicrostructureContext   `json:"microstructure,omitempty"`
}

func (m MarketContext) MarshalJSON() ([]byte, error) {
	type marketContextNoGDP struct {
		BenchmarkSymbol          string                         `json:"benchmark_symbol"`
		BenchmarkAvailable       bool                           `json:"benchmark_available"`
		BenchmarkError           string                         `json:"benchmark_error,omitempty"`
		SectorBenchmarkSymbol    string                         `json:"sector_benchmark_symbol,omitempty"`
		SectorBenchmarkAvailable bool                           `json:"sector_benchmark_available,omitempty"`
		SectorBenchmarkError     string                         `json:"sector_benchmark_error,omitempty"`
		StockReturn20            float64                        `json:"stock_return_20"`
		StockReturn60            float64                        `json:"stock_return_60"`
		StockReturn120           float64                        `json:"stock_return_120"`
		BenchmarkReturn20        float64                        `json:"benchmark_return_20"`
		BenchmarkReturn60        float64                        `json:"benchmark_return_60"`
		BenchmarkReturn120       float64                        `json:"benchmark_return_120"`
		RelativeStrength20       float64                        `json:"relative_strength_20"`
		RelativeStrength60       float64                        `json:"relative_strength_60"`
		SectorBenchmarkReturn20  float64                        `json:"sector_benchmark_return_20,omitempty"`
		SectorBenchmarkReturn60  float64                        `json:"sector_benchmark_return_60,omitempty"`
		SectorBenchmarkReturn120 float64                        `json:"sector_benchmark_return_120,omitempty"`
		SectorRelativeStrength20 float64                        `json:"sector_relative_strength_20,omitempty"`
		SectorRelativeStrength60 float64                        `json:"sector_relative_strength_60,omitempty"`
		SectorAlpha60            float64                        `json:"sector_alpha_60,omitempty"`
		SectorBeta60             float64                        `json:"sector_beta_60,omitempty"`
		SectorCorrelation60      float64                        `json:"sector_correlation_60,omitempty"`
		Alpha60                  float64                        `json:"alpha_60"`
		Beta60                   float64                        `json:"beta_60"`
		Correlation60            float64                        `json:"correlation_60"`
		LiveSnapshot             *marketdata.LiveMarketSnapshot `json:"live_snapshot,omitempty"`
		Microstructure           *MarketMicrostructureContext   `json:"microstructure,omitempty"`
	}
	base := marketContextNoGDP{
		BenchmarkSymbol:          m.BenchmarkSymbol,
		BenchmarkAvailable:       m.BenchmarkAvailable,
		BenchmarkError:           m.BenchmarkError,
		SectorBenchmarkSymbol:    m.SectorBenchmarkSymbol,
		SectorBenchmarkAvailable: m.SectorBenchmarkAvailable,
		SectorBenchmarkError:     m.SectorBenchmarkError,
		StockReturn20:            m.StockReturn20,
		StockReturn60:            m.StockReturn60,
		StockReturn120:           m.StockReturn120,
		BenchmarkReturn20:        m.BenchmarkReturn20,
		BenchmarkReturn60:        m.BenchmarkReturn60,
		BenchmarkReturn120:       m.BenchmarkReturn120,
		RelativeStrength20:       m.RelativeStrength20,
		RelativeStrength60:       m.RelativeStrength60,
		SectorBenchmarkReturn20:  m.SectorBenchmarkReturn20,
		SectorBenchmarkReturn60:  m.SectorBenchmarkReturn60,
		SectorBenchmarkReturn120: m.SectorBenchmarkReturn120,
		SectorRelativeStrength20: m.SectorRelativeStrength20,
		SectorRelativeStrength60: m.SectorRelativeStrength60,
		SectorAlpha60:            m.SectorAlpha60,
		SectorBeta60:             m.SectorBeta60,
		SectorCorrelation60:      m.SectorCorrelation60,
		Alpha60:                  m.Alpha60,
		Beta60:                   m.Beta60,
		Correlation60:            m.Correlation60,
		LiveSnapshot:             m.LiveSnapshot,
		Microstructure:           m.Microstructure,
	}
	if !m.GDP.Computed && strings.TrimSpace(m.GDP.DataQualityWarning) == "" {
		return json.Marshal(base)
	}
	type marketContextWithGDP struct {
		marketContextNoGDP
		GDP macro.GDPContext `json:"gdp"`
	}
	return json.Marshal(marketContextWithGDP{marketContextNoGDP: base, GDP: m.GDP})
}

type ValuationAnalysis struct {
	LatestYear        int                `json:"latest_year"`
	LatestQuarter     string             `json:"latest_quarter"`
	SectorModel       string             `json:"sector_model"`
	PaidCapital       float64            `json:"paid_capital"`
	MarketCap         float64            `json:"market_cap"`
	EnterpriseValue   float64            `json:"enterprise_value"`
	TotalDebt         float64            `json:"total_debt"`
	DebtDataAvailable bool               `json:"debt_data_available"`
	NetDebt           float64            `json:"net_debt"`
	SalesTTM          float64            `json:"sales_ttm"`
	EBITTTM           float64            `json:"ebit_ttm"`
	EBITDATTM         float64            `json:"ebitda_ttm"`
	NetIncomeTTM      float64            `json:"net_income_ttm"`
	OperatingCashTTM  float64            `json:"operating_cash_ttm"`
	FreeCashFlowTTM   float64            `json:"free_cash_flow_ttm"`
	Equity            float64            `json:"equity"`
	TotalAssets       float64            `json:"total_assets"`
	Ratios            map[string]float64 `json:"ratios"`
	AllowedRatios     []string           `json:"allowed_ratios,omitempty"`
	SuppressedRatios  []string           `json:"suppressed_ratios,omitempty"`
	SectorMetrics     map[string]float64 `json:"sector_metrics,omitempty"`
	FairValue         FairValueRange     `json:"fair_value"`
	DCF               DCFAnalysis        `json:"dcf"`
	Flags             []string           `json:"flags"`
}

type DCFAnalysis struct {
	Computed            bool          `json:"computed"`
	EnterpriseValue     float64       `json:"enterprise_value"`
	EquityValue         float64       `json:"equity_value"`
	FairValuePerShare   float64       `json:"fair_value_per_share"`
	WACC                float64       `json:"wacc"`
	TerminalGrowth      float64       `json:"terminal_growth"`
	RiskFreeRate        float64       `json:"risk_free_rate"`
	Beta                float64       `json:"beta"`
	EquityRiskPremium   float64       `json:"equity_risk_premium"`
	CostOfEquity        float64       `json:"cost_of_equity"`
	CostOfDebt          float64       `json:"cost_of_debt"`
	TaxRate             float64       `json:"tax_rate"`
	FCFGrowthAssumption float64       `json:"fcf_growth_assumption"`
	AssumptionSource    string        `json:"assumption_source"`
	Sensitivity         []DCFScenario `json:"sensitivity,omitempty"`
	Warnings            []string      `json:"warnings,omitempty"`
}

type DCFScenario struct {
	Name              string  `json:"name"`
	WACC              float64 `json:"wacc"`
	TerminalGrowth    float64 `json:"terminal_growth"`
	FairValuePerShare float64 `json:"fair_value_per_share"`
}

type FairValueRange struct {
	Bear       float64  `json:"bear"`
	Base       float64  `json:"base"`
	Bull       float64  `json:"bull"`
	Drivers    []string `json:"drivers"`
	Confidence float64  `json:"confidence"`
}

type PeerComparison struct {
	Sector          string             `json:"sector"`
	PeerGroup       string             `json:"peer_group,omitempty"`
	PeerCount       int                `json:"peer_count"`
	Peers           []PeerMetric       `json:"peers"`
	Medians         map[string]float64 `json:"medians"`
	Percentiles     map[string]float64 `json:"percentiles"`
	ValuationSignal string             `json:"valuation_signal"`
	Warnings        []string           `json:"warnings,omitempty"`
}

type PeerMetric struct {
	Symbol    string             `json:"symbol"`
	Name      string             `json:"name"`
	Price     float64            `json:"price"`
	MarketCap float64            `json:"market_cap"`
	Ratios    map[string]float64 `json:"ratios"`
	Metrics   map[string]float64 `json:"metrics"`
	Period    string             `json:"period"`
}

type DisclosureReview struct {
	KAPCompanyCardAvailable bool     `json:"kap_company_card_available,omitempty"`
	RecentDisclosureStatus  string   `json:"recent_disclosure_status"`
	RecentDisclosureCount   int      `json:"recent_disclosure_count"`
	LocalCommentCount       int      `json:"local_comment_count"`
	RiskFlags               []string `json:"risk_flags"`
	RequiredSources         []string `json:"required_sources"`
}

type EquityNewsSentimentReport struct {
	Computed        bool                      `json:"computed"`
	SourcePath      string                    `json:"source_path,omitempty"`
	ItemCount       int                       `json:"item_count"`
	RecentItemCount int                       `json:"recent_item_count"`
	PositiveCount   int                       `json:"positive_count"`
	NegativeCount   int                       `json:"negative_count"`
	NeutralCount    int                       `json:"neutral_count"`
	Score           float64                   `json:"score"`
	Signal          string                    `json:"signal,omitempty"`
	Summary         string                    `json:"summary,omitempty"`
	Items           []EquityNewsSentimentItem `json:"items,omitempty"`
	Warnings        []string                  `json:"warnings,omitempty"`
}

type EquityNewsSentimentItem struct {
	Title       string  `json:"title,omitempty"`
	Source      string  `json:"source,omitempty"`
	PublishedAt string  `json:"published_at,omitempty"`
	Score       float64 `json:"score"`
	Label       string  `json:"label"`
}

type FinancialDataGovernance struct {
	AsOf                             time.Time  `json:"as_of,omitempty"`
	DataMode                         string     `json:"data_mode,omitempty"`
	BacktestSafe                     bool       `json:"backtest_safe"`
	ProductionReady                  bool       `json:"production_ready"`
	AvailabilityStatus               string     `json:"availability_status"`
	Source                           string     `json:"source,omitempty"`
	Currency                         string     `json:"currency,omitempty"`
	LatestPeriod                     string     `json:"latest_period,omitempty"`
	LatestPublishDate                *time.Time `json:"latest_publish_date,omitempty"`
	LatestAvailableAt                *time.Time `json:"latest_available_at,omitempty"`
	PublishDateCoverage              float64    `json:"publish_date_coverage"`
	AvailableAtCoverage              float64    `json:"available_at_coverage"`
	VerifiedPublishDateCount         int        `json:"verified_publish_date_count"`
	ConservativeAvailableAtCount     int        `json:"conservative_available_at_count"`
	UnsafeAvailabilityCount          int        `json:"unsafe_availability_count"`
	ProductionEligiblePeriodCount    int        `json:"production_eligible_period_count"`
	ProductionQuarantinedPeriodCount int        `json:"production_quarantined_period_count"`
	FinanciallyConsistent            bool       `json:"financially_consistent"`
	ReconciliationCheckCount         int        `json:"reconciliation_check_count"`
	ReconciliationFailureCount       int        `json:"reconciliation_failure_count"`
	LineageEvents                    int        `json:"lineage_events"`
	StatementVersionStoreAvailable   bool       `json:"statement_version_store_available"`
	StatementVersionCount            int        `json:"statement_version_count"`
	RestatementCount                 int        `json:"restatement_count"`
	UniverseSourceAvailable          bool       `json:"universe_source_available"`
	SurvivorshipBiasRisk             bool       `json:"survivorship_bias_risk"`
	MissingPublishPeriods            []string   `json:"missing_publish_periods,omitempty"`
	MissingAvailableAtPeriods        []string   `json:"missing_available_at_periods,omitempty"`
	UnsafeBacktestPeriods            []string   `json:"unsafe_backtest_periods,omitempty"`
	InvalidChronologyPeriods         []string   `json:"invalid_chronology_periods,omitempty"`
	Warnings                         []string   `json:"warnings,omitempty"`
}

type Scenario struct {
	Name        string   `json:"name"`
	Probability float64  `json:"probability"`
	PriceTarget float64  `json:"price_target"`
	ReturnPct   float64  `json:"return_pct"`
	Drivers     []string `json:"drivers"`
}

type LiquidityProfile struct {
	LastValueTradedTRY      float64  `json:"last_value_traded_try"`
	AverageVolume20         float64  `json:"average_volume_20"`
	AverageValueTraded20TRY float64  `json:"average_value_traded_20_try"`
	MedianValueTraded20TRY  float64  `json:"median_value_traded_20_try"`
	VolumeVsAverage20       float64  `json:"volume_vs_average_20"`
	AmihudIlliquidity20     float64  `json:"amihud_illiquidity_20"`
	Turnover20              float64  `json:"turnover_20"`
	CapacityTRYAt10PctADV   float64  `json:"capacity_try_at_10pct_adv"`
	DaysToExit1MTRY         float64  `json:"days_to_exit_1m_try"`
	Warnings                []string `json:"warnings,omitempty"`
}

type PositionSizing struct {
	PortfolioValue       float64  `json:"portfolio_value"`
	RiskPerTradePct      float64  `json:"risk_per_trade_pct"`
	RiskBudget           float64  `json:"risk_budget"`
	Entry                float64  `json:"entry"`
	Stop                 float64  `json:"stop"`
	RiskPerShare         float64  `json:"risk_per_share"`
	Quantity             int      `json:"quantity"`
	Notional             float64  `json:"notional"`
	PortfolioPct         float64  `json:"portfolio_pct"`
	LiquidityCapNotional float64  `json:"liquidity_cap_notional"`
	MaxByLiquidityQty    int      `json:"max_by_liquidity_qty"`
	Warnings             []string `json:"warnings"`
}

type BacktestResult struct {
	Strategy            string              `json:"strategy"`
	ExecutionModel      string              `json:"execution_model"`
	BacktestSafe        bool                `json:"backtest_safe"`
	AdjustedPriceUsed   bool                `json:"adjusted_price_used"`
	PriceSeries         string              `json:"price_series,omitempty"`
	LookbackBars        int                 `json:"lookback_bars"`
	Trades              int                 `json:"trades"`
	WinRate             float64             `json:"win_rate"`
	AverageReturn       float64             `json:"average_return"`
	MedianReturn        float64             `json:"median_return"`
	ProfitFactor        float64             `json:"profit_factor"`
	MaxDrawdown         float64             `json:"max_drawdown"`
	CAGR                float64             `json:"cagr"`
	Volatility          float64             `json:"volatility"`
	Sharpe              float64             `json:"sharpe"`
	Sortino             float64             `json:"sortino"`
	Exposure            float64             `json:"exposure"`
	InSampleTrades      int                 `json:"in_sample_trades"`
	OutOfSampleTrades   int                 `json:"out_of_sample_trades"`
	OutOfSampleReturn   float64             `json:"out_of_sample_average_return"`
	Expectancy          float64             `json:"expectancy"`
	AvgHoldingBars      float64             `json:"avg_holding_bars"`
	CurrentInMarket     bool                `json:"current_in_market"`
	CommissionBps       float64             `json:"commission_bps"`
	SlippageBps         float64             `json:"slippage_bps"`
	LookaheadViolations int                 `json:"lookahead_violations"`
	CandidateStrategies []BacktestCandidate `json:"candidate_strategies,omitempty"`
}

type BacktestCandidate struct {
	Strategy          string  `json:"strategy"`
	Status            string  `json:"status"`
	Trades            int     `json:"trades"`
	OutOfSampleTrades int     `json:"out_of_sample_trades"`
	Expectancy        float64 `json:"expectancy"`
	OutOfSampleReturn float64 `json:"out_of_sample_return"`
	MaxDrawdown       float64 `json:"max_drawdown"`
	Score             float64 `json:"score"`
}

type SignalStats struct {
	CurrentRegime        string  `json:"current_regime"`
	SampleSize           int     `json:"sample_size"`
	ForwardBars          int     `json:"forward_bars"`
	WinRate              float64 `json:"win_rate"`
	AverageForwardReturn float64 `json:"average_forward_return"`
	MedianForwardReturn  float64 `json:"median_forward_return"`
	ProbabilityScore     float64 `json:"probability_score"`
	InsufficientData     bool    `json:"insufficient_data"`
}

type financialFile struct {
	Ticker         string                            `json:"ticker"`
	Source         string                            `json:"source,omitempty"`
	Currency       string                            `json:"currency,omitempty"`
	FinancialGroup string                            `json:"financial_group,omitempty"`
	FetchedAt      time.Time                         `json:"fetched_at,omitempty"`
	Periods        map[string]domain.FinancialPeriod `json:"periods,omitempty"`
	Lineage        []domain.DataLineageEvent         `json:"lineage,omitempty"`
	Quality        domain.FinancialDataQuality       `json:"quality,omitempty"`
	Data           map[string]financialField         `json:"data"`
}

type financialField struct {
	DescTR string                `json:"desc_tr"`
	DescEN string                `json:"desc_en"`
	Years  map[string][]*float64 `json:"years"`
}

type sectorClassificationFile struct {
	Source  string                          `json:"source"`
	Version string                          `json:"version"`
	Entries map[string]sectorClassification `json:"entries"`
}

type sectorClassification struct {
	Sector      string   `json:"sector"`
	Industry    string   `json:"industry"`
	PeerGroup   string   `json:"peer_group"`
	PeerSymbols []string `json:"peer_symbols"`
	Source      string   `json:"source"`
	Confidence  float64  `json:"confidence"`
}

type period struct {
	Year    int
	Quarter int
}

func AnalyzeSymbol(input SymbolInput) Report {
	opts := normalizeOptions(input.Options)
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		return analyzeCryptoSymbol(input, opts)
	}
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		return analyzeCommoditySymbol(input, opts)
	}
	data := newSymbolDataProvider(input)
	coverage := CoverageReport{}
	profile := data.companyProfile(input)
	fin, finOK := data.loadFinancials(input.Symbol)
	ratioHistory, ratioHistoryOK := data.loadFinancialRatioHistory(input.Symbol)
	asOf := symbolAsOf(input)
	latest := valuationPeriod(fin, opts, asOf)
	versionStore, versionStoreOK := data.loadStatementVersionStore(input.Symbol)
	governance := financialDataGovernance(input.EquitiesDir, fin, latest, asOf, finOK, versionStore, versionStoreOK, opts)
	kapPDFIngest := skippedKAPPDFIngest(input.Symbol)
	if !opts.SkipKAPPDFIngest {
		kapPDFIngest = data.analyzeKAPPDFIngest(input.Symbol)
	}
	fundamentalBacktest := eventbacktest.RunFundamentalEventsWithOptions(input.DailyCandles, fin.Periods, eventbacktest.FundamentalEventOptions{
		RequireVerifiedPublishDate: opts.RequireVerifiedPublishDate,
	})
	valuation := ValuationAnalysis{Ratios: map[string]float64{}}
	sectorFinancials := SectorFinancialAnalysis{
		Applicable: false,
		Profile:    "missing_financial_statements",
		Summary:    "Sektor bazli bilanço yorumu icin finansal tablo bulunamadi.",
		Warnings:   []string{"financial_statements_missing"},
	}
	if finOK {
		coverage.Available = append(coverage.Available, "financial_statements")
		valuation = buildValuation(fin, input.LastClose, latest, financialContextText(profile, fin.FinancialGroup), opts.ValuationAssumptionsFile)
		var histArg financialRatioHistory
		if ratioHistoryOK {
			histArg = ratioHistory
			coverage.Available = append(coverage.Available, "historical_ratio_data")
		} else {
			coverage.Missing = append(coverage.Missing, "historical_ratio_data")
		}
		sectorFinancials = analyzeSectorFinancialsWithHistory(fin, latest, profile, valuation, histArg)
		if sectorFinancials.Profile == "bank" {
			facts, tables, warnings := loadKAPBankRegulatorySources(input.EquitiesDir, input.Symbol, kapPDFIngest.OutputDir)
			sectorFinancials.Warnings = append(sectorFinancials.Warnings, warnings...)
			sectorFinancials = enrichBankRegulatoryMetricsFromKAP(sectorFinancials, facts, tables)
			updateBankRegulatoryValuationFlag(&valuation, sectorFinancials)
		}
		coverage.Available = append(coverage.Available, "sector_financial_interpretation")
		if !strictFinancialTimingEvidenceSafe(governance) {
			coverage.Warnings = append(coverage.Warnings, "financial_statements_not_backtest_safe")
			valuation.Flags = append(valuation.Flags, "financial_publish_date_unverified")
			sectorFinancials.Warnings = append(sectorFinancials.Warnings, "financial_publish_date_unverified")
		}
		profile.PaidCapital = valuation.PaidCapital
	} else {
		coverage.Missing = append(coverage.Missing, "financial_statements")
		coverage.Missing = append(coverage.Missing, "sector_financial_interpretation")
	}
	if profile.PaidCapital == 0 {
		profile.PaidCapital = paidCapitalFromFinancials(fin, latest)
	}
	peers := buildPeerComparison(input.EquitiesDir, input.Symbol, profile, input.LastClose, valuation, opts.PeerLimit, opts.ValuationAssumptionsFile, opts, asOf)
	valuation.FairValue = fairValueFromPeers(input.LastClose, valuation, peers)
	market := buildMarketContext(input, opts)
	vapDir := strings.TrimSpace(opts.VAPDir)
	if vapDir == "" {
		vapDir = filepath.Join(filepath.Dir(filepath.Clean(input.EquitiesDir)), "macro", "vap")
	}
	vapIndexPath := strings.TrimSpace(opts.VAPIndexPortfolioFile)
	if vapIndexPath == "" {
		vapIndexPath = filepath.Join(vapDir, "bist_endeks_portfoy.json")
	}
	vapFreeFloat := vapcontext.LoadFreeFloat(vapDir, input.Symbol, asOf)
	vapIndexPortfolio := vapcontext.LoadIndexPortfolio(vapIndexPath, profile.Sector, profile.Industry, asOf)
	bistOfficial := buildBISTOfficialContext(input.OfficialBISTCandles, input.OfficialBISTError, input.DailyCandles, asOf)
	disclosure := data.buildDisclosureReview(input.Symbol)
	newsSentiment := analyzeEquityNewsSentiment(input.EquitiesDir, input.Symbol, asOf)
	kapAssetInventory := data.analyzeKAPAssetInventory(input.Symbol)
	kapAssetInventoryRelevant := kapAssetInventoryDecisionRelevant(profile, valuation)
	scenarios := buildScenarios(input.LastClose, valuation, peers, market)
	coverage = finalizeCoverage(coverage, input, profile, valuation, peers, market, disclosure)
	if vapFreeFloat.Computed {
		coverage.Available = append(coverage.Available, "vap_free_float_xlsx")
	} else {
		coverage.Missing = append(coverage.Missing, "vap_free_float_xlsx")
		coverage.Warnings = append(coverage.Warnings, vapFreeFloat.Warnings...)
	}
	if vapIndexPortfolio.Computed {
		coverage.Available = append(coverage.Available, "vap_bist_index_portfolio")
	} else {
		coverage.Missing = append(coverage.Missing, "vap_bist_index_portfolio")
		coverage.Warnings = append(coverage.Warnings, vapIndexPortfolio.Warnings...)
	}
	if bistOfficial.Computed {
		coverage.Available = append(coverage.Available, "bist_official_unprocessed_ohlcv")
	} else {
		coverage.Missing = append(coverage.Missing, "bist_official_unprocessed_ohlcv")
		coverage.Warnings = append(coverage.Warnings, bistOfficial.Warnings...)
	}
	if kapPDFIngest.Computed {
		coverage.Available = append(coverage.Available, "kap_pdf_ingest")
		coverage.Warnings = append(coverage.Warnings, kapPDFIngest.Warnings...)
	} else {
		coverage.Missing = append(coverage.Missing, "kap_pdf_ingest")
	}
	if kapAssetInventory.Computed {
		if kapAssetInventoryRelevant {
			coverage.Available = append(coverage.Available, "kap_asset_inventory")
			coverage.Warnings = append(coverage.Warnings, kapAssetInventory.Warnings...)
		} else {
			kapAssetInventory.Warnings = nil
			kapAssetInventory.Summary = fmt.Sprintf("KAP varlık envanteri %s profili için ana karar girdisi değil; KAP PDF kanıtı ayrı ingest katmanında kullanıldı.", emptyString(profile.Sector, "bu şirket"))
		}
	} else if kapAssetInventoryRelevant {
		coverage.Missing = append(coverage.Missing, "kap_asset_inventory")
	}
	if input.CorporateActions.Status == "adjustment_ready" || input.CorporateActions.Status == "candidate_adjustments_review_required" || input.CorporateActions.Status == "events_only" {
		coverage.Available = append(coverage.Available, "corporate_actions")
		coverage.Warnings = append(coverage.Warnings, input.CorporateActions.Warnings...)
	} else if !ohlcv.IsCryptoAssetType(input.AssetType) && !ohlcv.IsCommodityAssetType(input.AssetType) {
		coverage.Missing = append(coverage.Missing, "corporate_actions")
		coverage.Warnings = append(coverage.Warnings, input.CorporateActions.Warnings...)
	}
	tcmbDir := strings.TrimSpace(opts.TCMBDir)
	if tcmbDir == "" {
		tcmbDir = defaultTCMBDirFromEquitiesDir(input.EquitiesDir)
	}
	tcmbCtx := buildTCMBContext(tcmbDir)
	if tcmbCtx.Computed {
		coverage.Available = append(coverage.Available, "tcmb_macro_context")
	} else if !ohlcv.IsCryptoAssetType(input.AssetType) && !ohlcv.IsCommodityAssetType(input.AssetType) {
		coverage.Missing = append(coverage.Missing, "tcmb_macro_context")
		coverage.Warnings = append(coverage.Warnings, tcmbCtx.Warnings...)
	}
	tcmbEVDSDir := strings.TrimSpace(opts.TCMBEVDSDir)
	if tcmbEVDSDir == "" {
		tcmbEVDSDir = defaultTCMBEVDSDirFromEquitiesDir(input.EquitiesDir)
	}
	tcmbEVDSCtx := buildTCMBEVDSContextForSymbol(tcmbEVDSDir, input.Symbol, profile, asOf)
	applyTCMBDocumentEvidenceToForecastImpact(&tcmbEVDSCtx, tcmbCtx)
	if tcmbEVDSCtx.AnalysisReady {
		coverage.Available = append(coverage.Available, "tcmb_evds_series_context")
	} else if !ohlcv.IsCryptoAssetType(input.AssetType) && !ohlcv.IsCommodityAssetType(input.AssetType) {
		coverage.Missing = append(coverage.Missing, "tcmb_evds_series_context")
		coverage.Warnings = append(coverage.Warnings, tcmbEVDSCtx.Warnings...)
	}
	if newsSentiment.Computed {
		coverage.Available = append(coverage.Available, "news_sentiment")
	} else if !ohlcv.IsCryptoAssetType(input.AssetType) && !ohlcv.IsCommodityAssetType(input.AssetType) {
		coverage.Missing = append(coverage.Missing, "news_sentiment")
		coverage.Warnings = append(coverage.Warnings, newsSentiment.Warnings...)
	}
	coverage = recalculateCoverageScore(coverage)
	valueReport := value.Analyze(value.Input{
		EquitiesDir:        input.EquitiesDir,
		Symbol:             input.Symbol,
		Sector:             profile.Sector,
		Industry:           profile.Industry,
		FinancialGroup:     fin.FinancialGroup,
		Currency:           input.Currency,
		CurrentPrice:       input.LastClose,
		PriceHistory:       valuePriceHistoryFromCandles(input.DailyCandles),
		AsOf:               asOf,
		AssumptionsFile:    opts.ValuationAssumptionsFile,
		MacroGDPFile:       opts.MacroGDPFile,
		SkipLegacyDocIntel: kapPDFIngest.Computed,
	})
	if kapPDFIngest.Computed {
		attachKAPPDFIngestEvidenceToValueReport(&valueReport, kapPDFIngest)
	}
	if valueReport.IntrinsicValue.Computed {
		scenarios = buildValueInvestingScenarios(input.LastClose, valueReport)
	}
	investmentResearch := buildInvestmentResearchReview(input.LastClose, profile, valuation, sectorFinancials, kapPDFIngest, kapAssetInventory, peers, scenarios, valueReport, coverage)
	report := Report{
		Coverage:            coverage,
		Company:             profile,
		Market:              market,
		Valuation:           valuation,
		SectorFinancials:    sectorFinancials,
		KAPPDFIngest:        kapPDFIngest,
		KAPAssetInventory:   kapAssetInventory,
		CorporateActions:    input.CorporateActions,
		Peers:               peers,
		Disclosure:          disclosure,
		NewsSentiment:       newsSentiment,
		Scenarios:           scenarios,
		DataGovernance:      governance,
		FundamentalBacktest: fundamentalBacktest,
		ValueInvesting:      valueReport,
		InvestmentResearch:  investmentResearch,
		TCMBContext:         tcmbCtx,
		TCMBEVDSContext:     tcmbEVDSCtx,
		VAPFreeFloat:        vapFreeFloat,
		VAPIndexPortfolio:   vapIndexPortfolio,
		BISTOfficial:        bistOfficial,
		DataQuality:         coverage.Score,
	}
	ApplyStrictEvidencePolicy(&report)
	return report
}

func attachKAPPDFIngestEvidenceToValueReport(report *value.Report, kap KAPPDFIngestSummary) {
	if report == nil || !kap.Computed {
		return
	}
	score := kapPDFEvidenceCoverageScore(kap)
	report.DocumentEvidence = docintel.Report{
		Computed:        true,
		Symbol:          firstNonEmptyString(kap.Symbol, report.Symbol),
		Root:            kap.OutputDir,
		TotalFiles:      kap.TotalDocuments,
		PDFCount:        firstNonZeroInt(kap.SourcePDFCount, kap.TotalDocuments),
		ContentAnalyzed: kap.AnalysisUsableCount,
		ContentReadable: kap.AnalysisUsableCount,
		CoverageScore:   score,
		Summary:         kapPDFEvidenceSummary(kap, score),
		Categories:      kapPDFDocIntelCategories(kap.TypeCounts),
		KeyDocuments:    kapPDFDocIntelDocuments(kap.ImportantDocuments, 12),
		Warnings:        kap.Warnings,
	}
	report.Warnings = removeStringValue(report.Warnings, "legacy_docintel_skipped_kap_pdf_ingest_available")
	report.Missing = removeStringValue(report.Missing, "kap_document_evidence")
	report.FairValue.DataInputs = replaceKAPDocumentEvidenceInput(report.FairValue.DataInputs, kap, score)
	report.Checks = replaceKAPDocumentEvidenceCheck(report.Checks, report.DocumentEvidence.Summary, score)
}

func valuePriceHistoryFromCandles(candles []ohlcv.Candle) []value.PriceObservation {
	out := make([]value.PriceObservation, 0, len(candles))
	for _, candle := range candles {
		closePrice := candle.EffectiveClose()
		if candle.Time.IsZero() || closePrice <= 0 {
			continue
		}
		out = append(out, value.PriceObservation{Time: candle.Time, Close: closePrice})
	}
	return out
}

func kapPDFEvidenceCoverageScore(kap KAPPDFIngestSummary) float64 {
	if kap.DecisionRelevantDocuments > 0 {
		return mathutil.Clamp(100*float64(kap.DecisionRelevantUsableCount)/float64(kap.DecisionRelevantDocuments), 0, 100)
	}
	if kap.TotalDocuments > 0 {
		return mathutil.Clamp(100*float64(kap.AnalysisUsableCount)/float64(kap.TotalDocuments), 0, 100)
	}
	return 0
}

func kapPDFEvidenceSummary(kap KAPPDFIngestSummary, score float64) string {
	if kap.DecisionRelevantDocuments > 0 {
		return fmt.Sprintf("KAP PDF ingest kanıtı bağlı: %d PDF, %d analize uygun, karar-ilgili %d/%d kullanılabilir, skor %.0f/100.",
			kap.TotalDocuments,
			kap.AnalysisUsableCount,
			kap.DecisionRelevantUsableCount,
			kap.DecisionRelevantDocuments,
			score,
		)
	}
	return fmt.Sprintf("KAP PDF ingest kanıtı bağlı: %d PDF, %d analize uygun, skor %.0f/100.", kap.TotalDocuments, kap.AnalysisUsableCount, score)
}

func kapPDFDocIntelCategories(counts []KAPPDFTypeCount) []docintel.CategorySummary {
	out := make([]docintel.CategorySummary, 0, len(counts))
	for _, count := range counts {
		if count.Count <= 0 {
			continue
		}
		out = append(out, docintel.CategorySummary{
			Category: count.Type,
			Label:    count.Label,
			Count:    count.Count,
		})
	}
	return out
}

func kapPDFDocIntelDocuments(docs []KAPPDFDocumentSummary, limit int) []docintel.Document {
	if limit <= 0 || len(docs) == 0 {
		return nil
	}
	if len(docs) > limit {
		docs = docs[:limit]
	}
	out := make([]docintel.Document, 0, len(docs))
	for _, doc := range docs {
		out = append(out, docintel.Document{
			Category:         doc.DocumentType,
			CategoryLabel:    doc.DocumentLabel,
			FileName:         firstNonEmptyString(doc.FileName, filepath.Base(doc.FilePath)),
			Path:             doc.FilePath,
			Extension:        ".pdf",
			TextExtracted:    doc.AnalysisUsable,
			TextChars:        doc.TextLength,
			Purpose:          doc.DocumentLabel,
			ContentSummary:   doc.ContentSnippet,
			ExtractionSource: "kap_pdf_ingest_jsonl",
			ExtractionNote:   doc.ParseStatus,
			Evidence:         compactKapPDFWarnings(doc.Warnings, 4),
		})
	}
	return out
}

func replaceKAPDocumentEvidenceInput(inputs []string, kap KAPPDFIngestSummary, score float64) []string {
	replacement := fmt.Sprintf("KAP belge kanıtı: %d PDF, %d analize uygun, skor %.0f/100", kap.TotalDocuments, kap.AnalysisUsableCount, score)
	if kap.DecisionRelevantDocuments > 0 {
		replacement = fmt.Sprintf("KAP belge kanıtı: %d PDF, %d analize uygun, karar-ilgili %d/%d, skor %.0f/100",
			kap.TotalDocuments,
			kap.AnalysisUsableCount,
			kap.DecisionRelevantUsableCount,
			kap.DecisionRelevantDocuments,
			score,
		)
	}
	replaced := false
	out := make([]string, 0, len(inputs)+1)
	for _, input := range inputs {
		if strings.HasPrefix(input, "KAP belge kanıtı:") {
			if !replaced {
				out = append(out, replacement)
				replaced = true
			}
			continue
		}
		out = append(out, input)
	}
	if !replaced {
		out = append(out, replacement)
	}
	return out
}

func replaceKAPDocumentEvidenceCheck(checks []value.Check, summary string, score float64) []value.Check {
	status := "fail"
	switch {
	case score >= 70:
		status = "pass"
	case score >= 45:
		status = "limited"
	}
	replaced := false
	for i := range checks {
		if checks[i].Name != "kap_document_evidence" {
			continue
		}
		checks[i].Status = status
		checks[i].Score = score
		checks[i].Message = summary
		replaced = true
	}
	if !replaced {
		checks = append(checks, value.Check{Name: "kap_document_evidence", Status: status, Score: score, Message: summary})
	}
	return checks
}

func compactKapPDFWarnings(warnings []string, limit int) []string {
	if limit <= 0 || len(warnings) <= limit {
		return warnings
	}
	return warnings[:limit]
}

func removeStringValue(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value == target {
			continue
		}
		out = append(out, value)
	}
	return out
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func kapAssetInventoryDecisionRelevant(profile CompanyProfile, valuation ValuationAnalysis) bool {
	text := util.SlugTR(strings.Join([]string{
		profile.Sector,
		profile.Industry,
		valuation.SectorModel,
	}, " "))
	return strings.Contains(text, "gyo") ||
		strings.Contains(text, "gayrimenkul") ||
		strings.Contains(text, "reit") ||
		strings.Contains(text, "realestate")
}

func analyzeCryptoSymbol(input SymbolInput, opts Options) Report {
	name := input.CompanyName
	if name == "" {
		name = ohlcv.CryptoDisplayName(input.Symbol)
	}
	if name == "" {
		name = input.Symbol
	}
	cryptoContext := loadCryptoContext(input.EquitiesDir, input.Symbol)
	coverage := CoverageReport{
		Available: []string{
			"tradingview_ohlcv_price_volume",
			"technical_indicators",
			"support_resistance",
			"walk_forward_price_backtest",
		},
	}
	addCryptoCoverage(&coverage, cryptoContext.OnChain, "onchain_mvrv_nupl_sopr_realized_cap")
	addCryptoCoverage(&coverage, cryptoContext.Derivatives, "derivatives_funding_open_interest_liquidations")
	addCryptoCoverage(&coverage, cryptoContext.ExchangeFlow, "exchange_flow_reserve_netflow")
	addCryptoCoverage(&coverage, cryptoContext.NewsSentiment, "crypto_news_sentiment")
	if len(coverage.Missing) > 0 {
		coverage.Warnings = append(coverage.Warnings, "crypto_context_sources_missing")
	}
	coverage.Warnings = append(coverage.Warnings, cryptoContext.Warnings...)
	total := len(coverage.Available) + len(coverage.Missing)
	coverage.Score = 100 * mathutil.SafeDiv(float64(len(coverage.Available)), float64(total))
	profile := CompanyProfile{
		Symbol:                   input.Symbol,
		Name:                     name,
		Sector:                   "Crypto Assets",
		Industry:                 "Digital Asset",
		PeerGroup:                "Major spot crypto pairs",
		SectorSource:             "asset_type_crypto",
		ClassificationConfidence: 1,
		ClassificationWarnings: []string{
			"asset_class_classification_used",
		},
	}
	valuation := ValuationAnalysis{
		SectorModel: "crypto_spot_technical_only",
		Ratios:      map[string]float64{},
		AllowedRatios: []string{
			"technical_range",
			"realized_volatility",
			"trend_momentum",
		},
		SuppressedRatios: []string{
			"PE",
			"PB",
			"EV_Sales",
			"ROE",
			"NetDebt_Eq",
			"DCF",
		},
		FairValue: FairValueRange{
			Bear:       input.LastClose * 0.80,
			Base:       input.LastClose,
			Bull:       input.LastClose * 1.20,
			Drivers:    cryptoFairValueDrivers(cryptoContext),
			Confidence: 0.25,
		},
		Flags: []string{
			"issuer_valuation_not_applicable",
		},
	}
	if !cryptoContext.OnChain.Available {
		valuation.Flags = append(valuation.Flags, "onchain_data_missing")
	}
	if !cryptoContext.Derivatives.Available {
		valuation.Flags = append(valuation.Flags, "derivatives_data_missing")
	}
	market := buildMarketContext(input, opts)
	peers := PeerComparison{
		Sector:          profile.Sector,
		PeerGroup:       profile.PeerGroup,
		ValuationSignal: "not_applicable",
		Medians:         map[string]float64{},
		Percentiles:     map[string]float64{},
	}
	disclosure := DisclosureReview{
		RecentDisclosureStatus: "unavailable",
		RequiredSources:        cryptoRequiredSources(cryptoContext),
		RiskFlags:              cryptoRiskFlags(cryptoContext),
	}
	availabilityStatus := "technical_ohlcv_only"
	source := "tradingview_ohlcv"
	productionReady := false
	if cryptoContextAnyAvailable(cryptoContext) {
		availabilityStatus = "technical_plus_crypto_context"
		source = "tradingview_ohlcv+crypto_context"
		productionReady = len(coverage.Missing) == 0
	}
	governance := FinancialDataGovernance{
		AsOf:                    symbolAsOf(input),
		DataMode:                opts.DataMode,
		BacktestSafe:            true,
		ProductionReady:         productionReady,
		AvailabilityStatus:      availabilityStatus,
		Source:                  source,
		Currency:                input.Currency,
		FinanciallyConsistent:   true,
		UniverseSourceAvailable: true,
		Warnings:                cryptoGovernanceWarnings(cryptoContext),
	}
	report := Report{
		Coverage:         coverage,
		Company:          profile,
		Market:           market,
		Valuation:        valuation,
		SectorFinancials: sectorFinancialNotApplicable(input.AssetType),
		KAPPDFIngest: KAPPDFIngestSummary{
			Computed: false,
			Symbol:   input.Symbol,
			Summary:  "Kripto varliklar icin sirket PDF/bildirim ingest uygulanmaz.",
			Warnings: []string{"issuer_pdf_ingest_not_applicable_to_crypto"},
		},
		Peers:          peers,
		Disclosure:     disclosure,
		Scenarios:      buildCryptoScenarios(input.LastClose),
		DataGovernance: governance,
		CryptoContext:  cryptoContext,
		DataQuality:    coverage.Score,
	}
	ApplyStrictEvidencePolicy(&report)
	return report
}

func addCryptoCoverage(coverage *CoverageReport, section CryptoContextSection, key string) {
	if section.Available {
		coverage.Available = append(coverage.Available, key)
		return
	}
	coverage.Missing = append(coverage.Missing, key)
}

func cryptoFairValueDrivers(context CryptoContextReport) []string {
	drivers := []string{"technical_range_only"}
	if context.OnChain.Available {
		drivers = append(drivers, "onchain_context_available")
	}
	if context.Derivatives.Available {
		drivers = append(drivers, "derivatives_context_available")
	}
	if context.ExchangeFlow.Available {
		drivers = append(drivers, "exchange_flow_context_available")
	}
	if context.NewsSentiment.Available {
		drivers = append(drivers, "news_sentiment_context_available")
	}
	if len(drivers) == 1 {
		drivers = append(drivers, "crypto_context_sources_missing")
	}
	return drivers
}

func cryptoRequiredSources(context CryptoContextReport) []string {
	required := []string{}
	if !context.NewsSentiment.Available {
		required = append(required, "crypto news/headline sentiment feed")
	}
	if !context.OnChain.Available {
		required = append(required, "on-chain MVRV/NUPL/SOPR/realized cap metrics feed")
	}
	if !context.Derivatives.Available {
		required = append(required, "derivatives funding/open-interest/liquidations feed")
	}
	if !context.ExchangeFlow.Available {
		required = append(required, "exchange reserve/netflow feed")
	}
	return required
}

func cryptoRiskFlags(context CryptoContextReport) []string {
	flags := []string{}
	if !context.NewsSentiment.Available {
		flags = append(flags, "crypto_news_feed_not_connected")
	}
	if !context.OnChain.Available {
		flags = append(flags, "onchain_metrics_not_connected")
	}
	if !context.Derivatives.Available {
		flags = append(flags, "derivatives_data_not_connected")
	}
	if !context.ExchangeFlow.Available {
		flags = append(flags, "exchange_flow_not_connected")
	}
	return flags
}

func cryptoGovernanceWarnings(context CryptoContextReport) []string {
	warnings := append([]string{}, context.Warnings...)
	if !cryptoContextAnyAvailable(context) {
		warnings = append(warnings, "crypto_context_json_not_connected")
	}
	if len(context.RequiredActions) > 0 {
		warnings = append(warnings, context.RequiredActions...)
	}
	return warnings
}

func cryptoContextAnyAvailable(context CryptoContextReport) bool {
	return context.OnChain.Available || context.Derivatives.Available || context.ExchangeFlow.Available || context.NewsSentiment.Available
}

func loadCryptoContext(equitiesDir string, symbol string) CryptoContextReport {
	out := CryptoContextReport{
		RequiredActions: defaultCryptoContextActions(),
	}
	for _, path := range cryptoContextPathCandidates(equitiesDir, symbol) {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return CryptoContextReport{
				Path:            path,
				RequiredActions: defaultCryptoContextActions(),
				Warnings:        []string{"crypto_context_parse_failed: " + err.Error()},
			}
		}
		out.Computed = true
		out.Path = path
		out.RequiredActions = missingCryptoContextActions(out)
		return out
	}
	return out
}

func cryptoContextPathCandidates(equitiesDir string, symbol string) []string {
	key := ohlcv.SymbolPathKey(symbol)
	if key == "" {
		return nil
	}
	roots := []string{}
	root := filepath.Clean(strings.TrimSpace(equitiesDir))
	if root == "." || root == "" {
		root = "data/equities"
	}
	switch filepath.Base(root) {
	case "equities":
		roots = append(roots, filepath.Join(filepath.Dir(root), "crypto", key))
	case "crypto":
		roots = append(roots, filepath.Join(root, key))
	default:
		roots = append(roots, filepath.Join(root, "crypto", key), filepath.Join(root, key))
	}
	roots = append(roots, filepath.Join(filepath.Dir(root), "crypto", key))
	seen := map[string]bool{}
	paths := []string{}
	for _, dir := range roots {
		path := filepath.Join(filepath.Clean(dir), "crypto_context.json")
		if seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}

func defaultCryptoContextActions() []string {
	return []string{
		"data/crypto/{SYMBOL}/crypto_context.json içine onchain.available=true ve MVRV/NUPL/SOPR/realized_cap metriklerini ekle",
		"data/crypto/{SYMBOL}/crypto_context.json içine derivatives.available=true ve funding/open_interest/liquidations metriklerini ekle",
		"data/crypto/{SYMBOL}/crypto_context.json içine exchange_flow.available=true ve reserve/netflow metriklerini ekle",
		"data/crypto/{SYMBOL}/crypto_context.json içine news_sentiment.available=true ve haber duyarlılığı özetini ekle",
	}
}

func missingCryptoContextActions(context CryptoContextReport) []string {
	actions := []string{}
	if !context.OnChain.Available {
		actions = append(actions, "crypto_context.onchain bölümüne MVRV/NUPL/SOPR/realized_cap verisi bağlanmalı")
	}
	if !context.Derivatives.Available {
		actions = append(actions, "crypto_context.derivatives bölümüne funding/open_interest/liquidations verisi bağlanmalı")
	}
	if !context.ExchangeFlow.Available {
		actions = append(actions, "crypto_context.exchange_flow bölümüne exchange reserve/netflow verisi bağlanmalı")
	}
	if !context.NewsSentiment.Available {
		actions = append(actions, "crypto_context.news_sentiment bölümüne güncel haber/sentiment özeti bağlanmalı")
	}
	return actions
}

func addCommodityCoverage(coverage *CoverageReport, section CommodityContextSection, key string) {
	if section.Available {
		coverage.Available = append(coverage.Available, key)
		return
	}
	coverage.Missing = append(coverage.Missing, key)
}

func commodityFairValueDrivers(context CommodityContextReport) []string {
	drivers := []string{"not_applicable_without_validated_price_model"}
	if context.Macro.Available {
		drivers = append(drivers, "usd_index_dxy_real_yield_context_available_not_directional_target")
	}
	if context.FuturesPositioning.Available {
		drivers = append(drivers, "futures_cot_open_interest_context_available_not_directional_target")
	}
	if context.GoldETFPhysicalFlow.Available {
		drivers = append(drivers, "gold_etf_physical_flow_context_available_not_directional_target")
	}
	if context.CentralBankGeopoliticalNews.Available {
		drivers = append(drivers, "central_bank_geopolitical_news_context_available_not_directional_target")
	}
	return drivers
}

func commodityRequiredSources(context CommodityContextReport) []string {
	required := []string{}
	if !context.Macro.Available {
		required = append(required, "USD index/DXY and real-yield macro feed")
	}
	if !context.FuturesPositioning.Available {
		required = append(required, "COMEX/COT futures positioning and open-interest feed")
	}
	if !context.GoldETFPhysicalFlow.Available {
		required = append(required, "gold ETF and physical flow feed")
	}
	if !context.CentralBankGeopoliticalNews.Available {
		required = append(required, "central-bank gold reserve and geopolitical news feed")
	}
	return required
}

func commodityRiskFlags(context CommodityContextReport) []string {
	flags := []string{}
	if !context.Macro.Available {
		flags = append(flags, "commodity_macro_feed_not_connected")
	}
	if !context.FuturesPositioning.Available {
		flags = append(flags, "futures_positioning_not_connected")
	}
	if !context.GoldETFPhysicalFlow.Available {
		flags = append(flags, "gold_flow_data_not_connected")
	}
	if !context.CentralBankGeopoliticalNews.Available {
		flags = append(flags, "commodity_news_feed_not_connected")
	}
	return flags
}

func commodityGovernanceWarnings(context CommodityContextReport) []string {
	warnings := append([]string{}, context.Warnings...)
	if !commodityContextAnyAvailable(context) {
		warnings = append(warnings, "commodity_context_json_not_connected")
	}
	if len(context.RequiredActions) > 0 {
		warnings = append(warnings, context.RequiredActions...)
	}
	return warnings
}

func commodityContextAnyAvailable(context CommodityContextReport) bool {
	return context.Macro.Available ||
		context.FuturesPositioning.Available ||
		context.GoldETFPhysicalFlow.Available ||
		context.CentralBankGeopoliticalNews.Available
}

func commodityFairValueConfidence(context CommodityContextReport, coverageScore float64) float64 {
	return 0
}

func loadCommodityContext(equitiesDir string, symbol string) CommodityContextReport {
	out := CommodityContextReport{
		RequiredActions: defaultCommodityContextActions(),
	}
	for _, path := range commodityContextPathCandidates(equitiesDir, symbol) {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return CommodityContextReport{
				Path:            path,
				RequiredActions: defaultCommodityContextActions(),
				Warnings:        []string{"commodity_context_parse_failed: " + err.Error()},
			}
		}
		out.Computed = true
		out.Path = path
		out.RequiredActions = missingCommodityContextActions(out)
		return out
	}
	return out
}

func commodityContextPathCandidates(equitiesDir string, symbol string) []string {
	keys := []string{}
	addKey := func(value string) {
		key := ohlcv.SymbolPathKey(value)
		if key == "" {
			return
		}
		for _, existing := range keys {
			if existing == key {
				return
			}
		}
		keys = append(keys, key)
	}
	addKey(symbol)
	if instrument, ok := ohlcv.CanonicalCommodityInstrument(symbol); ok {
		addKey(instrument.Symbol)
	}
	if len(keys) == 0 {
		return nil
	}
	roots := []string{}
	root := filepath.Clean(strings.TrimSpace(equitiesDir))
	if root == "." || root == "" {
		root = "data/equities"
	}
	for _, key := range keys {
		switch filepath.Base(root) {
		case "equities":
			roots = append(roots, filepath.Join(filepath.Dir(root), "commodities", key), filepath.Join(root, key))
		case "commodities":
			roots = append(roots, filepath.Join(root, key))
		default:
			roots = append(roots, filepath.Join(root, "commodities", key), filepath.Join(root, key))
		}
		roots = append(roots, filepath.Join(filepath.Dir(root), "commodities", key))
	}
	seen := map[string]bool{}
	paths := []string{}
	for _, dir := range roots {
		path := filepath.Join(filepath.Clean(dir), "commodity_context.json")
		if seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}

func defaultCommodityContextActions() []string {
	return []string{
		"data/commodities/{SYMBOL}/commodity_context.json içine macro.available=true ve DXY/USD index + real yield metriklerini ekle",
		"data/commodities/{SYMBOL}/commodity_context.json içine futures_positioning.available=true ve COT/open_interest metriklerini ekle",
		"data/commodities/{SYMBOL}/commodity_context.json içine gold_etf_physical_flow.available=true ve ETF/fiziki altın akışı metriklerini ekle",
		"data/commodities/{SYMBOL}/commodity_context.json içine central_bank_geopolitical_news.available=true ve merkez bankası/jeopolitik veri özetini ekle",
	}
}

func missingCommodityContextActions(context CommodityContextReport) []string {
	actions := []string{}
	if !context.Macro.Available {
		actions = append(actions, "commodity_context.macro bölümüne DXY/USD index ve reel faiz verisi bağlanmalı")
	}
	if !context.FuturesPositioning.Available {
		actions = append(actions, "commodity_context.futures_positioning bölümüne COT ve açık pozisyon verisi bağlanmalı")
	}
	if !context.GoldETFPhysicalFlow.Available {
		actions = append(actions, "commodity_context.gold_etf_physical_flow bölümüne ETF/fiziki altın akışı verisi bağlanmalı")
	}
	if !context.CentralBankGeopoliticalNews.Available {
		actions = append(actions, "commodity_context.central_bank_geopolitical_news bölümüne merkez bankası ve jeopolitik veri bağlanmalı")
	}
	return actions
}

func analyzeCommoditySymbol(input SymbolInput, opts Options) Report {
	name := input.CompanyName
	if name == "" {
		name = ohlcv.CommodityDisplayName(input.Symbol)
	}
	if name == "" {
		name = input.Symbol
	}
	commodityContext := loadCommodityContext(input.EquitiesDir, input.Symbol)
	coverage := CoverageReport{
		Available: []string{
			"tradingview_ohlcv_price_volume",
			"technical_indicators",
			"support_resistance",
			"walk_forward_price_backtest",
		},
	}
	addCommodityCoverage(&coverage, commodityContext.Macro, "usd_index_dxy_real_yield_macro")
	addCommodityCoverage(&coverage, commodityContext.FuturesPositioning, "futures_cot_open_interest_positioning")
	addCommodityCoverage(&coverage, commodityContext.GoldETFPhysicalFlow, "gold_etf_physical_flow")
	addCommodityCoverage(&coverage, commodityContext.CentralBankGeopoliticalNews, "central_bank_geopolitical_news")
	if len(coverage.Missing) > 0 {
		coverage.Warnings = append(coverage.Warnings, "commodity_report_is_technical_until_macro_positioning_and_flow_sources_are_connected")
	}
	coverage.Warnings = append(coverage.Warnings, commodityContext.Warnings...)
	coverage = recalculateCoverageScore(coverage)
	profile := CompanyProfile{
		Symbol:                   input.Symbol,
		Name:                     name,
		Sector:                   "Precious Metals",
		Industry:                 "Spot Gold",
		PeerGroup:                "Gold, USD and real-yield macro benchmarks",
		SectorSource:             "asset_type_commodity",
		ClassificationConfidence: 1,
		ClassificationWarnings:   []string{"asset_class_classification_used"},
	}
	valuation := ValuationAnalysis{
		SectorModel: "commodity_spot_technical_macro_pending",
		Ratios:      map[string]float64{},
		AllowedRatios: []string{
			"technical_range",
			"realized_volatility",
			"trend_momentum",
		},
		SuppressedRatios: []string{
			"PE",
			"PB",
			"EV_Sales",
			"ROE",
			"NetDebt_Eq",
			"DCF",
		},
		FairValue: FairValueRange{
			Bear:       0,
			Base:       0,
			Bull:       0,
			Drivers:    commodityFairValueDrivers(commodityContext),
			Confidence: commodityFairValueConfidence(commodityContext, coverage.Score),
		},
		Flags: []string{
			"issuer_valuation_not_applicable",
		},
	}
	if !commodityContext.Macro.Available {
		valuation.Flags = append(valuation.Flags, "macro_real_yield_data_missing")
	}
	if !commodityContext.FuturesPositioning.Available {
		valuation.Flags = append(valuation.Flags, "futures_positioning_data_missing")
	}
	if !commodityContext.GoldETFPhysicalFlow.Available {
		valuation.Flags = append(valuation.Flags, "etf_physical_flow_data_missing")
	}
	if !commodityContext.CentralBankGeopoliticalNews.Available {
		valuation.Flags = append(valuation.Flags, "central_bank_geopolitical_news_missing")
	}
	market := buildMarketContext(input, opts)
	peers := PeerComparison{
		Sector:          profile.Sector,
		PeerGroup:       profile.PeerGroup,
		ValuationSignal: "not_applicable",
		Medians:         map[string]float64{},
		Percentiles:     map[string]float64{},
	}
	disclosure := DisclosureReview{
		RecentDisclosureStatus: "unavailable",
		RequiredSources:        commodityRequiredSources(commodityContext),
		RiskFlags:              commodityRiskFlags(commodityContext),
	}
	availabilityStatus := "technical_ohlcv_only"
	source := "tradingview_ohlcv"
	productionReady := false
	if commodityContextAnyAvailable(commodityContext) {
		availabilityStatus = "technical_plus_commodity_context"
		source = "tradingview_ohlcv+commodity_context"
		productionReady = len(coverage.Missing) == 0
	}
	governance := FinancialDataGovernance{
		AsOf:                    symbolAsOf(input),
		DataMode:                opts.DataMode,
		BacktestSafe:            true,
		ProductionReady:         productionReady,
		AvailabilityStatus:      availabilityStatus,
		Source:                  source,
		Currency:                input.Currency,
		FinanciallyConsistent:   true,
		UniverseSourceAvailable: true,
		Warnings:                commodityGovernanceWarnings(commodityContext),
	}
	report := Report{
		Coverage:         coverage,
		Company:          profile,
		Market:           market,
		Valuation:        valuation,
		SectorFinancials: sectorFinancialNotApplicable(input.AssetType),
		KAPPDFIngest: KAPPDFIngestSummary{
			Computed: false,
			Symbol:   input.Symbol,
			Summary:  "Emtia/altin varliklari icin sirket PDF/bildirim ingest uygulanmaz.",
			Warnings: []string{"issuer_pdf_ingest_not_applicable_to_commodity"},
		},
		Peers:            peers,
		Disclosure:       disclosure,
		Scenarios:        buildCommodityScenariosWithContext(input.LastClose, commodityContext),
		DataGovernance:   governance,
		CommodityContext: commodityContext,
		DataQuality:      coverage.Score,
	}
	ApplyStrictEvidencePolicy(&report)
	return report
}

func buildCryptoScenarios(lastClose float64) []Scenario {
	if lastClose <= 0 {
		lastClose = 1
	}
	return []Scenario{
		{Name: "bear", Probability: 0.25, PriceTarget: lastClose * 0.80, ReturnPct: -20, Drivers: []string{"spot technical risk band", "high crypto volatility", "missing on-chain/derivatives confirmation"}},
		{Name: "base", Probability: 0.50, PriceTarget: lastClose, ReturnPct: 0, Drivers: []string{"current spot price anchor", "technical trend state", "no fundamental fair value model for crypto spot asset"}},
		{Name: "bull", Probability: 0.25, PriceTarget: lastClose * 1.20, ReturnPct: 20, Drivers: []string{"technical recovery band", "momentum and liquidity improvement required", "on-chain/derivatives confirmation missing"}},
	}
}

func buildCommodityScenarios(lastClose float64) []Scenario {
	return buildCommodityScenariosWithContext(lastClose, CommodityContextReport{})
}

func buildCommodityScenariosWithContext(lastClose float64, context CommodityContextReport) []Scenario {
	return nil
}

func AnalyzeTimeframe(input TimeframeInput, opts Options) TimeframeReport {
	opts = normalizeOptions(opts)
	liquidity := buildLiquidity(input.Candles)
	backtest := backtestTrendMomentum(input.Candles, opts)
	stats := signalStats(input.Candles, 20)
	priceAdjustment := buildPriceAdjustmentReview(input.Candles, input.PriceAdjustment)
	backtest.PriceSeries = priceAdjustment.PriceSeries
	backtest.AdjustedPriceUsed = priceAdjustment.AdjustedCandles > 0 && priceAdjustment.UnadjustedCandles == 0 && priceAdjustment.PriceSeries != "raw"
	if explicitPriceAdjustmentReport(input.PriceAdjustment) && !priceAdjustment.BacktestSafe {
		backtest.BacktestSafe = false
	}
	return TimeframeReport{
		Liquidity:       liquidity,
		PositionSizing:  buildPositionSizing(input, opts, liquidity),
		Backtest:        backtest,
		SignalStats:     stats,
		Technical:       buildTechnicalEvidence(input, backtest, stats, priceAdjustment),
		PriceAdjustment: priceAdjustment,
	}
}

func skippedKAPPDFIngest(symbol string) KAPPDFIngestSummary {
	return KAPPDFIngestSummary{
		Symbol:   strings.ToUpper(strings.TrimSpace(symbol)),
		Summary:  "KAP PDF ingest analiz opsiyonu ile atlandi.",
		Warnings: []string{"kap_pdf_ingest_skipped_by_option"},
	}
}

func buildTechnicalEvidence(input TimeframeInput, backtest BacktestResult, stats SignalStats, priceAdjustment PriceAdjustmentReview) TechnicalEvidence {
	score := technicalScore(input)
	indicators := selectTechnicalIndicators(input.IndicatorSignals, 12)
	patterns := selectTechnicalPatterns(input.Patterns, input.PatternScans, 12)
	guardrails := technicalGuardrails(input, indicators, patterns)
	gate := buildTechnicalSignalGate(input, score, indicators, patterns, backtest, stats, priceAdjustment)
	if gate.Status != "pass" {
		guardrails = appendUniqueString(guardrails, "technical_signal_gate_not_passed")
	}
	if !technicalValidationOK(input.TechnicalValidation) {
		guardrails = appendUniqueString(guardrails, "technical_validation_gate_not_passed")
	}
	return TechnicalEvidence{
		Summary:            technicalSummary(score, indicators, patterns, guardrails),
		Score:              score,
		SelectedIndicators: indicators,
		SelectedPatterns:   patterns,
		Validation:         input.TechnicalValidation,
		SignalGate:         gate,
		SignalCounts:       indicatorSignalCounts(input.IndicatorSignals),
		PatternCounts:      patternDirectionCounts(input.Patterns, input.PatternScans),
		Guardrails:         guardrails,
	}
}

func buildPriceAdjustmentReview(candles []ohlcv.Candle, adjustment corporateactions.AdjustmentReport) PriceAdjustmentReview {
	review := PriceAdjustmentReview{
		BacktestSafe:      true,
		PriceSeries:       adjustment.PriceSeries,
		ActionsConsidered: adjustment.ActionsConsidered,
		AppliedActions:    adjustment.AppliedActions,
		SkippedActions:    adjustment.SkippedActions,
		Warnings:          append([]string(nil), adjustment.Warnings...),
	}
	for i, candle := range candles {
		if candle.IsAdjusted {
			review.AdjustedCandles++
		} else {
			review.UnadjustedCandles++
		}
		if i == 0 {
			continue
		}
		prevClose := candles[i-1].EffectiveClose()
		open := candle.EffectiveOpen()
		if prevClose <= 0 || open <= 0 {
			continue
		}
		gap := math.Abs(open/prevClose - 1)
		if gap >= 0.35 {
			review.PotentialSplitGapBars++
		}
	}
	if adjustment.AdjustedCandles > 0 || adjustment.UnadjustedCandles > 0 || adjustment.ActionsConsidered > 0 || adjustment.PriceSeries != "" {
		review.AdjustedCandles = adjustment.AdjustedCandles
		review.UnadjustedCandles = adjustment.UnadjustedCandles
		review.PotentialSplitGapBars = adjustment.PotentialCorporateGapBars
		review.BacktestSafe = adjustment.BacktestSafe
		if review.PriceSeries == "" {
			review.PriceSeries = "raw"
		}
		return review
	}
	if review.PriceSeries == "" {
		review.PriceSeries = "provider_or_input"
	}
	if review.UnadjustedCandles > 0 {
		review.BacktestSafe = false
		review.Warnings = append(review.Warnings, "unadjusted_ohlcv_candles_present")
	}
	if review.PotentialSplitGapBars > 0 {
		review.BacktestSafe = false
		review.Warnings = append(review.Warnings, "potential_split_or_corporate_action_gap_detected")
	}
	return review
}

func explicitPriceAdjustmentReport(adjustment corporateactions.AdjustmentReport) bool {
	return adjustment.AdjustedCandles > 0 ||
		adjustment.UnadjustedCandles > 0 ||
		adjustment.ActionsConsidered > 0 ||
		adjustment.AppliedActions > 0 ||
		adjustment.SkippedActions > 0 ||
		adjustment.PotentialCorporateGapBars > 0 ||
		adjustment.PriceSeries != "" ||
		len(adjustment.Warnings) > 0
}

func technicalScore(input TimeframeInput) TechnicalScore {
	ind := input.Indicators
	lastClose := input.LastClose
	if lastClose <= 0 && len(input.Candles) > 0 {
		lastClose = input.Candles[len(input.Candles)-1].EffectiveClose()
	}
	lastVolume := input.LastVolume
	if lastVolume <= 0 && len(input.Candles) > 0 {
		lastVolume = input.Candles[len(input.Candles)-1].EffectiveVolume()
	}

	trend := 0.0
	if lastClose > 0 && ind.SMA20 > 0 && lastClose > ind.SMA20 {
		trend += 7
	}
	if ind.SMA20 > 0 && ind.SMA50 > 0 && ind.SMA20 > ind.SMA50 {
		trend += 7
	}
	if ind.SMA50 > 0 && ind.SMA200 > 0 && ind.SMA50 > ind.SMA200 {
		trend += 7
	}
	if ind.MACDHistogram > 0 {
		trend += 6
	}
	if ind.ADX14 >= 25 {
		trend += 5
	} else if ind.ADX14 >= 20 {
		trend += 3
	}
	if lastClose > 0 && ind.Supertrend > 0 && lastClose > ind.Supertrend {
		trend += 3
	}
	trend = mathutil.Clamp(trend, 0, 35)

	momentum := 0.0
	switch {
	case ind.RSI14 >= 45 && ind.RSI14 <= 65:
		momentum += 8
	case ind.RSI14 > 65 && ind.RSI14 <= 75:
		momentum += 5
	case ind.RSI14 >= 30 && ind.RSI14 < 45:
		momentum += 4
	case ind.RSI14 > 0 && ind.RSI14 < 30:
		momentum += 3
	}
	if ind.MACDHistogram > 0 {
		momentum += 6
	}
	if ind.StochasticK >= 20 && ind.StochasticK <= 80 {
		momentum += 4
	} else if ind.StochasticK > 0 && ind.StochasticK < 20 {
		momentum += 2
	}
	if ind.ROC12 > 0 {
		momentum += 4
	}
	if ind.MFI14 >= 40 && ind.MFI14 <= 80 {
		momentum += 3
	}
	momentum = mathutil.Clamp(momentum, 0, 25)

	volume := 0.0
	volumeRatio := 0.0
	if lastVolume > 0 && ind.VolumeSMA20 > 0 {
		volumeRatio = lastVolume / ind.VolumeSMA20
		switch {
		case volumeRatio >= 2:
			volume += 10
		case volumeRatio >= 1:
			volume += 8
		case volumeRatio >= 0.75:
			volume += 4
		}
	}
	if ind.ChaikinMoneyFlow20 > 0 {
		volume += 3
	} else if ind.ChaikinMoneyFlow20 < -0.05 {
		volume -= 2
	}
	if volumeRatio >= 0.75 && ind.ChaikinMoneyFlow20 >= 0 && ind.MFI14 >= 50 && ind.MFI14 <= 80 {
		volume += 2
	}
	volume = mathutil.Clamp(volume, 0, 15)

	volatility := 0.0
	if lastClose > 0 && ind.ATR14 > 0 {
		atrPct := ind.ATR14 / lastClose
		switch {
		case atrPct >= 0.01 && atrPct <= 0.06:
			volatility += 6
		case atrPct > 0 && atrPct <= 0.10:
			volatility += 4
		case atrPct > 0:
			volatility += 1
		}
	}
	if ind.BollingerUpper > 0 && ind.BollingerLower > 0 && ind.BollingerMiddle > 0 {
		width := (ind.BollingerUpper - ind.BollingerLower) / ind.BollingerMiddle
		switch {
		case width >= 0.02 && width <= 0.20:
			volatility += 4
		case width > 0 && width <= 0.35:
			volatility += 2
		}
	}
	volatility = mathutil.Clamp(volatility, 0, 10)

	pattern := 0.0
	patternSeen := map[string]bool{}
	for _, item := range input.Patterns {
		if item.Confidence < 0.5 {
			continue
		}
		key := technicalPatternKey(item.Name, item.Category)
		if patternSeen[key] {
			continue
		}
		patternSeen[key] = true
		switch directionClass(item.Direction) {
		case "bullish":
			pattern += 2.0 * item.Confidence
		case "bearish":
			pattern -= 1.5 * item.Confidence
		default:
			pattern += 0.5 * item.Confidence
		}
		if item.VolumeConfirmed {
			pattern += 0.5
		}
	}
	pattern = mathutil.Clamp(pattern, 0, 15)

	total := mathutil.Clamp(trend+momentum+volume+volatility+pattern, 0, 100)
	return TechnicalScore{
		Trend:          trend,
		Momentum:       momentum,
		Volume:         volume,
		VolatilityRisk: volatility,
		Pattern:        pattern,
		Total:          total,
	}
}

func selectTechnicalIndicators(indicators []ohlcv.IndicatorResult, limit int) []TechnicalIndicator {
	selectedByKey := map[string]TechnicalIndicator{}
	for _, indicator := range indicators {
		if !indicator.Computed || indicator.Confidence < 0.5 || isWeakIndicatorSignal(indicator.Signal) {
			continue
		}
		item := TechnicalIndicator{
			Name:       indicator.Name,
			Category:   indicator.Category,
			Group:      indicator.Group,
			Signal:     indicator.Signal,
			Value:      indicator.Value,
			Confidence: indicator.Confidence,
			Source:     indicator.Source,
			Evidence:   append([]string{}, indicator.Evidence...),
		}
		key := technicalIndicatorKey(item.Name)
		if existing, ok := selectedByKey[key]; ok && technicalIndicatorPriority(existing) >= technicalIndicatorPriority(item) {
			continue
		}
		selectedByKey[key] = item
	}
	selected := make([]TechnicalIndicator, 0, len(selectedByKey))
	for _, item := range selectedByKey {
		selected = append(selected, item)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		left := technicalIndicatorPriority(selected[i])
		right := technicalIndicatorPriority(selected[j])
		if left == right {
			return selected[i].Name < selected[j].Name
		}
		return left > right
	})
	if limit > 0 && len(selected) > limit {
		return balanceTechnicalIndicators(selected, limit)
	}
	if limit > 0 {
		return balanceTechnicalIndicators(selected, limit)
	}
	return selected
}

func selectTechnicalPatterns(patterns []ohlcv.PatternResult, scans []ohlcv.PatternScanResult, limit int) []TechnicalPattern {
	selected := []TechnicalPattern{}
	seen := map[string]bool{}
	for _, pattern := range patterns {
		if pattern.Confidence < 0.5 {
			continue
		}
		key := technicalPatternKey(pattern.Name, pattern.Category)
		if seen[key] {
			continue
		}
		item := TechnicalPattern{
			Name:                 pattern.Name,
			Category:             pattern.Category,
			Direction:            pattern.Direction,
			Confidence:           pattern.Confidence,
			StartIndex:           pattern.StartIndex,
			EndIndex:             pattern.EndIndex,
			VolumeConfirmed:      pattern.VolumeConfirmed,
			VolumeConfirmation:   pattern.VolumeConfirmation,
			ConfirmingIndicators: append([]string{}, pattern.ConfirmingIndicators...),
			InvalidatingSignals:  append([]string{}, pattern.InvalidatingSignals...),
			TradeValue:           pattern.TradeValue,
			ValidationStatus:     pattern.ValidationStatus,
			ValidationMethod:     pattern.ValidationMethod,
			ValidationPValue:     pattern.ValidationPValue,
			ValidationCILow:      pattern.ValidationCILow,
			ValidationCIHigh:     pattern.ValidationCIHigh,
			Evidence:             append([]string{}, pattern.Evidence...),
		}
		selected = append(selected, item)
		seen[key] = true
	}
	for _, scan := range scans {
		if !scan.Matched || !scan.Actionable || scan.Confidence < 0.5 || seen[technicalPatternKey(scan.Name, scan.Category)] {
			continue
		}
		item := TechnicalPattern{
			Name:                 scan.Name,
			Category:             scan.Category,
			Direction:            scan.Direction,
			Confidence:           scan.Confidence,
			VolumeConfirmation:   scan.VolumeConfirmation,
			ConfirmingIndicators: append([]string{}, scan.ConfirmingIndicators...),
			InvalidatingSignals:  append([]string{}, scan.InvalidatingSignals...),
			TradeValue:           scan.TradeValue,
			ValidationStatus:     scan.ValidationStatus,
			ValidationMethod:     scan.ValidationMethod,
			ValidationPValue:     scan.ValidationPValue,
			ValidationCILow:      scan.ValidationCILow,
			ValidationCIHigh:     scan.ValidationCIHigh,
			Source:               scan.Source,
			Evidence:             append([]string{}, scan.Evidence...),
		}
		selected = append(selected, item)
		seen[technicalPatternKey(item.Name, item.Category)] = true
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Confidence == selected[j].Confidence {
			return selected[i].Name < selected[j].Name
		}
		return selected[i].Confidence > selected[j].Confidence
	})
	if limit > 0 && len(selected) > limit {
		return selected[:limit]
	}
	return selected
}

func technicalGuardrails(input TimeframeInput, indicators []TechnicalIndicator, patterns []TechnicalPattern) []string {
	guardrails := []string{}
	entry := (input.TradePlan.EntryMin + input.TradePlan.EntryMax) / 2
	if entry <= 0 {
		entry = input.LastClose
	}
	direction := strings.ToLower(strings.TrimSpace(input.TradePlan.Direction))
	if direction == "long" || direction == "short" {
		if input.TradePlan.StopLoss <= 0 {
			guardrails = append(guardrails, "invalid_stop_loss_non_positive")
		}
		if entry > 0 && direction == "long" && input.TradePlan.StopLoss >= entry {
			guardrails = append(guardrails, "invalid_long_stop_loss_above_entry")
		}
		if entry > 0 && direction == "short" && input.TradePlan.StopLoss <= entry {
			guardrails = append(guardrails, "invalid_short_stop_loss_below_entry")
		}
		if !input.TradePlan.Rejected && input.TradePlan.RiskRewardRatio <= 0 {
			guardrails = append(guardrails, "invalid_risk_reward_ratio")
		}
		if !input.TradePlan.Rejected && math.Abs(input.TradePlan.RiskRewardRatio-1) < 0.000001 {
			guardrails = append(guardrails, "risk_reward_ratio_exactly_one_review_support_resistance")
		}
	}
	lastClose := input.LastClose
	if lastClose <= 0 && len(input.Candles) > 0 {
		lastClose = input.Candles[len(input.Candles)-1].EffectiveClose()
	}
	if lastClose > 0 && input.Indicators.ATR14/lastClose > 0.10 {
		guardrails = append(guardrails, "atr_above_10pct_of_close_high_volatility")
	}
	if input.Indicators.RSI14 > 85 {
		guardrails = append(guardrails, "rsi_extreme_overbought")
	}
	if input.Indicators.RSI14 > 0 && input.Indicators.RSI14 < 15 {
		guardrails = append(guardrails, "rsi_extreme_oversold")
	}
	if len(indicators) == 0 {
		guardrails = append(guardrails, "no_computed_indicator_signal_selected")
	}
	if len(patterns) == 0 {
		guardrails = append(guardrails, "no_confirmed_pattern_selected")
	}
	if !technicalValidationOK(input.TechnicalValidation) {
		guardrails = append(guardrails, "technical_validation_gate_not_passed")
	}
	return guardrails
}

func buildTechnicalSignalGate(input TimeframeInput, score TechnicalScore, indicators []TechnicalIndicator, patterns []TechnicalPattern, backtest BacktestResult, stats SignalStats, priceAdjustment PriceAdjustmentReview) TechnicalSignalGate {
	direction := tradePlanSignalDirection(input.TradePlan.Direction)
	gate := TechnicalSignalGate{
		Status:          "fail",
		Direction:       direction,
		Timeframe:       input.Timeframe,
		Score:           0,
		EntryMin:        input.TradePlan.EntryMin,
		EntryMax:        input.TradePlan.EntryMax,
		StopLoss:        input.TradePlan.StopLoss,
		Target1:         input.TradePlan.TakeProfit1,
		Target2:         input.TradePlan.TakeProfit2,
		RiskRewardRatio: input.TradePlan.RiskRewardRatio,
		PriceStructure:  tradePlanPriceStructure(input.TradePlan),
		BacktestSummary: technicalBacktestSummary(backtest, stats),
		VolumeConfirmed: technicalVolumeConfirmed(input, direction, patterns),
	}
	gate.VolumeConfirmation = technicalVolumeConfirmation(input, gate.VolumeConfirmed)
	gate.ConfirmingIndicators = confirmingTechnicalIndicators(indicators, direction)
	gate.ConfirmingPatterns = confirmingTechnicalPatterns(patterns, direction)
	gate.ConflictingSignals = conflictingTechnicalSignals(indicators, patterns, direction)
	gate.Evidence = technicalSignalGateEvidence(input, score, backtest, stats, priceAdjustment)

	planReady := technicalTradePlanReady(input.TradePlan)
	riskRewardOK := input.TradePlan.RiskRewardRatio >= 1.5
	indicatorOK := len(gate.ConfirmingIndicators) >= 2
	patternOK := len(gate.ConfirmingPatterns) >= 1
	conflictOK := len(gate.ConflictingSignals) == 0
	backtestOK := technicalBacktestOK(backtest)
	statsOK := technicalRegimeStatsOK(stats)
	priceDataOK := priceAdjustment.BacktestSafe
	validationOK := technicalValidationOK(input.TechnicalValidation)

	if planReady {
		gate.Passes = append(gate.Passes, "fiyat yapısı: giriş/stop/hedef planı var")
	} else {
		gate.Blockers = append(gate.Blockers, "fiyat yapısı: uygulanabilir giriş/stop/hedef planı yok")
		if input.TradePlan.RejectReason != "" {
			gate.Blockers = append(gate.Blockers, "trade plan reddi: "+localize.Reason(input.TradePlan.RejectReason))
		}
	}
	if riskRewardOK {
		gate.Passes = append(gate.Passes, fmt.Sprintf("risk/ödül %.2f eşiği geçiyor", input.TradePlan.RiskRewardRatio))
	} else {
		gate.Blockers = append(gate.Blockers, fmt.Sprintf("risk/ödül %.2f; aktif sinyal için en az 1.50 gerekir", input.TradePlan.RiskRewardRatio))
	}
	if gate.VolumeConfirmed {
		gate.Passes = append(gate.Passes, "hacim/para akışı teyidi var")
	} else {
		gate.Blockers = append(gate.Blockers, "hacim teyidi yok")
	}
	if indicatorOK {
		gate.Passes = append(gate.Passes, "aynı yönde en az iki hesaplanmış indikatör teyidi var")
	} else {
		gate.Blockers = append(gate.Blockers, "aynı yönde en az iki hesaplanmış indikatör teyidi yok")
	}
	if patternOK {
		gate.Passes = append(gate.Passes, "aynı yönde doğrulanmış formasyon/fiyat aksiyonu var")
	} else {
		gate.Blockers = append(gate.Blockers, "doğrulanmış formasyon/fiyat aksiyonu yok")
	}
	if conflictOK {
		gate.Passes = append(gate.Passes, "çelişkili indikatör/formasyon baskın değil")
	} else {
		gate.Blockers = append(gate.Blockers, "çelişkili sinyaller var: "+strings.Join(limitStrings(gate.ConflictingSignals, 4), "; "))
	}
	if backtestOK {
		gate.Passes = append(gate.Passes, "backtest/OOS güvenliği geçiyor")
	} else {
		gate.Blockers = append(gate.Blockers, "backtest/OOS istatistiği aktif sinyal için yetersiz")
	}
	if statsOK {
		gate.Passes = append(gate.Passes, "benzer rejim istatistiği destekliyor")
	} else {
		gate.Blockers = append(gate.Blockers, "benzer rejim istatistiği yeterince güçlü değil")
	}
	if priceDataOK {
		gate.Passes = append(gate.Passes, "OHLCV fiyat düzeltme güvenliği geçiyor")
	} else {
		gate.Blockers = append(gate.Blockers, "OHLCV düzeltme/split/bedelli güvenliği geçmiyor")
	}
	if validationOK {
		if input.TechnicalValidation.Status != "" {
			gate.Passes = append(gate.Passes, "indikatör/formasyon doğrulama kapısı geçiyor")
		}
	} else {
		gate.Blockers = append(gate.Blockers, "indikatör/formasyon doğrulama kapısı geçmedi")
		for _, blocker := range limitStrings(input.TechnicalValidation.Blockers, 4) {
			gate.Blockers = append(gate.Blockers, "teknik doğrulama: "+blocker)
		}
	}

	planScore := boolScoreFloat(planReady)
	rrScore := mathutil.Clamp(input.TradePlan.RiskRewardRatio/2.5*100, 0, 100)
	indicatorScore := mathutil.Clamp(float64(len(gate.ConfirmingIndicators))/3*100, 0, 100)
	patternScore := boolScoreFloat(patternOK)
	volumeScore := boolScoreFloat(gate.VolumeConfirmed)
	conflictScore := boolScoreFloat(conflictOK)
	backtestScore := technicalBacktestScore(backtest)
	regimeScore := technicalRegimeStatsScore(stats)
	priceDataScore := boolScoreFloat(priceDataOK)
	validationScore := technicalValidationScore(input.TechnicalValidation)
	gate.Score = mathutil.Clamp(weightedAverage([]weightedPart{
		{planScore, 0.16},
		{rrScore, 0.11},
		{indicatorScore, 0.13},
		{patternScore, 0.11},
		{volumeScore, 0.11},
		{conflictScore, 0.09},
		{backtestScore, 0.11},
		{regimeScore, 0.05},
		{priceDataScore, 0.04},
		{validationScore, 0.09},
	}), 0, 100)
	gate.Status = technicalSignalGateStatus(gate.Score, gate.Blockers, planReady, priceDataOK, validationOK)
	gate.Actionable = gate.Status == "pass"
	gate.Label = technicalSignalGateLabel(gate)
	if planReady && !gate.Actionable {
		gate.PriceStructure = fmt.Sprintf("izleme/paper-trade planı: giriş %.2f-%.2f, stop %.2f, hedef %.2f/%.2f; teknik kanıt kapısı geçmediği için aktif işlem sinyali değildir", input.TradePlan.EntryMin, input.TradePlan.EntryMax, input.TradePlan.StopLoss, input.TradePlan.TakeProfit1, input.TradePlan.TakeProfit2)
		gate.Blockers = append(gate.Blockers, "giriş/stop/hedef seviyeleri izleme seviyesidir; canlı AL/SAT sinyali değildir")
		gate.Evidence = append(gate.Evidence, "technical_gate_not_passed_trade_levels_are_paper_trade_only")
	}
	return gate
}

func tradePlanSignalDirection(direction string) string {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "long", "buy", "bullish":
		return "bullish"
	case "short", "sell", "bearish":
		return "bearish"
	default:
		return "neutral"
	}
}

func tradePlanPriceStructure(plan ohlcv.TradePlan) string {
	if technicalTradePlanReady(plan) {
		return fmt.Sprintf("%s plan: giriş %.2f-%.2f, stop %.2f, hedef %.2f/%.2f", plan.Direction, plan.EntryMin, plan.EntryMax, plan.StopLoss, plan.TakeProfit1, plan.TakeProfit2)
	}
	if plan.RejectReason != "" {
		return "plan reddedildi: " + localize.Reason(plan.RejectReason)
	}
	return "uygulanabilir fiyat yapısı yok"
}

func technicalTradePlanReady(plan ohlcv.TradePlan) bool {
	if plan.Rejected || plan.EntryMin <= 0 || plan.EntryMax <= 0 || plan.StopLoss <= 0 || plan.TakeProfit1 <= 0 {
		return false
	}
	if plan.EntryMin > plan.EntryMax || plan.RiskRewardRatio < 1.5 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(plan.Direction)) {
	case "long":
		return plan.StopLoss < plan.EntryMin && plan.TakeProfit1 > plan.EntryMax
	case "short":
		return plan.StopLoss > plan.EntryMax && plan.TakeProfit1 < plan.EntryMin
	default:
		return false
	}
}

func technicalVolumeConfirmed(input TimeframeInput, direction string, patterns []TechnicalPattern) bool {
	for _, pattern := range patterns {
		if direction != "neutral" && directionClass(pattern.Direction) != direction {
			continue
		}
		if pattern.VolumeConfirmed {
			return true
		}
	}
	ratio := 0.0
	if input.LastVolume > 0 && input.Indicators.VolumeSMA20 > 0 {
		ratio = input.LastVolume / input.Indicators.VolumeSMA20
	}
	switch direction {
	case "bullish":
		return ratio >= 1.15 && input.Indicators.ChaikinMoneyFlow20 > 0.02 ||
			ratio >= 0.95 && input.Indicators.ChaikinMoneyFlow20 > 0.05 && input.Indicators.MFI14 >= 55
	case "bearish":
		return ratio >= 1.15 && input.Indicators.ChaikinMoneyFlow20 < -0.02 ||
			ratio >= 0.95 && input.Indicators.ChaikinMoneyFlow20 < -0.05 && input.Indicators.MFI14 > 0 && input.Indicators.MFI14 <= 45
	default:
		return ratio >= 1.25
	}
}

func technicalVolumeConfirmation(input TimeframeInput, confirmed bool) string {
	ratio := 0.0
	if input.LastVolume > 0 && input.Indicators.VolumeSMA20 > 0 {
		ratio = input.LastVolume / input.Indicators.VolumeSMA20
	}
	status := "yok"
	if confirmed {
		status = "var"
	}
	return fmt.Sprintf("%s | hacim/SMA20 %.2f | CMF20 %.3f | MFI14 %.1f", status, ratio, input.Indicators.ChaikinMoneyFlow20, input.Indicators.MFI14)
}

func confirmingTechnicalIndicators(indicators []TechnicalIndicator, direction string) []string {
	if direction == "neutral" {
		return nil
	}
	out := []string{}
	for _, indicator := range indicators {
		if directionClass(indicator.Signal) != direction {
			continue
		}
		if !actionableTechnicalIndicatorConfirmation(indicator) {
			continue
		}
		out = append(out, fmt.Sprintf("%s (%s, %.0f/100)", indicator.Name, indicator.Signal, indicator.Confidence*100))
	}
	return limitStrings(uniqueTechnicalStrings(out), 8)
}

func actionableTechnicalIndicatorConfirmation(indicator TechnicalIndicator) bool {
	source := strings.ToLower(strings.TrimSpace(indicator.Source))
	category := strings.ToLower(strings.TrimSpace(indicator.Category))
	group := strings.ToLower(strings.TrimSpace(indicator.Group))
	name := strings.ToLower(strings.TrimSpace(indicator.Name))
	if strings.Contains(source, "market_structure") {
		return false
	}
	for _, token := range []string{"smart_money", "wyckoff", "pattern_recognition"} {
		if strings.Contains(category, token) || strings.Contains(group, token) {
			return false
		}
	}
	for _, token := range []string{"fvg", "fair value gap", "balanced price range", "bpr", "order block", "breaker block", "liquidity sweep"} {
		if strings.Contains(name, token) {
			return false
		}
	}
	return true
}

func confirmingTechnicalPatterns(patterns []TechnicalPattern, direction string) []string {
	if direction == "neutral" {
		return nil
	}
	out := []string{}
	for _, pattern := range patterns {
		if directionClass(pattern.Direction) != direction {
			continue
		}
		if pattern.Confidence < 0.6 {
			continue
		}
		if len(pattern.InvalidatingSignals) > len(pattern.ConfirmingIndicators) {
			continue
		}
		volume := "hacim yok"
		if pattern.VolumeConfirmed {
			volume = "hacim var"
		}
		out = append(out, fmt.Sprintf("%s (%s, güven %.0f/100, %s)", pattern.Name, pattern.Direction, pattern.Confidence*100, volume))
	}
	return limitStrings(uniqueTechnicalStrings(out), 6)
}

func conflictingTechnicalSignals(indicators []TechnicalIndicator, patterns []TechnicalPattern, direction string) []string {
	if direction == "neutral" {
		return nil
	}
	opposite := "bearish"
	if direction == "bearish" {
		opposite = "bullish"
	}
	out := []string{}
	for _, indicator := range indicators {
		if directionClass(indicator.Signal) == opposite && indicator.Confidence >= 0.55 {
			out = append(out, fmt.Sprintf("%s karşı yönde (%s)", indicator.Name, indicator.Signal))
		}
	}
	for _, pattern := range patterns {
		if directionClass(pattern.Direction) == opposite && pattern.Confidence >= 0.55 {
			out = append(out, fmt.Sprintf("%s karşı formasyon (%s)", pattern.Name, pattern.Direction))
		}
		for _, invalid := range pattern.InvalidatingSignals {
			if strings.TrimSpace(invalid) != "" {
				out = append(out, pattern.Name+": "+invalid)
			}
		}
	}
	return limitStrings(uniqueTechnicalStrings(out), 8)
}

func technicalBacktestOK(backtest BacktestResult) bool {
	return backtest.BacktestSafe &&
		backtest.LookaheadViolations == 0 &&
		backtest.Trades >= 30 &&
		backtest.OutOfSampleTrades >= 10 &&
		backtest.Expectancy > 0 &&
		backtest.OutOfSampleReturn > 0
}

func technicalBacktestScore(backtest BacktestResult) float64 {
	if !backtest.BacktestSafe || backtest.LookaheadViolations > 0 {
		return 0
	}
	tradeScore := mathutil.Clamp(float64(backtest.Trades)/50*100, 0, 100)
	oosScore := mathutil.Clamp(float64(backtest.OutOfSampleTrades)/15*100, 0, 100)
	expectancyScore := 0.0
	if backtest.Expectancy > 0 {
		expectancyScore = mathutil.Clamp(backtest.Expectancy/0.025*100, 0, 100)
	}
	oosReturnScore := 0.0
	if backtest.OutOfSampleReturn > 0 {
		oosReturnScore = mathutil.Clamp(backtest.OutOfSampleReturn/0.025*100, 0, 100)
	}
	return weightedAverage([]weightedPart{
		{tradeScore, 0.25},
		{oosScore, 0.25},
		{expectancyScore, 0.25},
		{oosReturnScore, 0.25},
	})
}

func technicalRegimeStatsOK(stats SignalStats) bool {
	return !stats.InsufficientData &&
		stats.SampleSize >= 30 &&
		stats.WinRate >= 0.52 &&
		stats.AverageForwardReturn > 0
}

func technicalRegimeStatsScore(stats SignalStats) float64 {
	if stats.InsufficientData || stats.SampleSize <= 0 {
		return 0
	}
	sampleScore := mathutil.Clamp(float64(stats.SampleSize)/50*100, 0, 100)
	winScore := mathutil.Clamp((stats.WinRate-0.45)/0.15*100, 0, 100)
	returnScore := 0.0
	if stats.AverageForwardReturn > 0 {
		returnScore = mathutil.Clamp(stats.AverageForwardReturn/0.03*100, 0, 100)
	}
	return weightedAverage([]weightedPart{{sampleScore, 0.35}, {winScore, 0.35}, {returnScore, 0.30}})
}

func technicalBacktestSummary(backtest BacktestResult, stats SignalStats) string {
	return fmt.Sprintf("%s | %d işlem, %d OOS, expectancy %.2f%%, OOS %.2f%% | rejim %s n=%d win %.1f%% ileri %.2f%%",
		backtest.Strategy,
		backtest.Trades,
		backtest.OutOfSampleTrades,
		backtest.Expectancy*100,
		backtest.OutOfSampleReturn*100,
		emptyString(stats.CurrentRegime, "rejim yok"),
		stats.SampleSize,
		stats.WinRate*100,
		stats.AverageForwardReturn*100,
	)
}

func technicalSignalGateEvidence(input TimeframeInput, score TechnicalScore, backtest BacktestResult, stats SignalStats, priceAdjustment PriceAdjustmentReview) []string {
	out := []string{
		fmt.Sprintf("teknik skor %.1f/100: trend %.1f, momentum %.1f, hacim %.1f, formasyon %.1f", score.Total, score.Trend, score.Momentum, score.Volume, score.Pattern),
		tradePlanPriceStructure(input.TradePlan),
		technicalVolumeConfirmation(input, technicalVolumeConfirmed(input, tradePlanSignalDirection(input.TradePlan.Direction), nil)),
		technicalBacktestSummary(backtest, stats),
		fmt.Sprintf("fiyat düzeltme güvenliği: seri=%s adjusted=%d unadjusted=%d aksiyon=%d uygulanan=%d atlanan=%d gap=%d safe=%t", emptyString(priceAdjustment.PriceSeries, "unknown"), priceAdjustment.AdjustedCandles, priceAdjustment.UnadjustedCandles, priceAdjustment.ActionsConsidered, priceAdjustment.AppliedActions, priceAdjustment.SkippedActions, priceAdjustment.PotentialSplitGapBars, priceAdjustment.BacktestSafe),
	}
	if input.TechnicalValidation.Status != "" {
		summary := input.TechnicalValidation.Summary
		if summary == "" {
			summary = fmt.Sprintf("durum=%s skor=%.0f/100", input.TechnicalValidation.Status, input.TechnicalValidation.Score)
		}
		out = append(out, "teknik doğrulama: "+summary)
	}
	if len(input.TradePlan.Reasoning) > 0 {
		out = append(out, "trade plan gerekçesi: "+strings.Join(limitStrings(input.TradePlan.Reasoning, 3), "; "))
	}
	return out
}

func technicalValidationOK(validation TechnicalValidationReport) bool {
	if validation.Status == "" {
		return true
	}
	return validation.GateEligible && !strings.EqualFold(strings.TrimSpace(validation.Status), "fail")
}

func technicalValidationScore(validation TechnicalValidationReport) float64 {
	if validation.Status == "" && validation.Score == 0 {
		return 100
	}
	return mathutil.Clamp(validation.Score, 0, 100)
}

func technicalSignalGateStatus(score float64, blockers []string, planReady bool, priceDataOK bool, validationOK bool) string {
	if !planReady || !priceDataOK || !validationOK {
		return "fail"
	}
	if score >= 78 && len(blockers) == 0 {
		return "pass"
	}
	if score >= 55 {
		return "limited"
	}
	return "fail"
}

func technicalSignalGateLabel(gate TechnicalSignalGate) string {
	switch gate.Status {
	case "pass":
		return "Aktif işlem sinyali kanıt kapısından geçti"
	case "limited":
		return "İzleme / paper-trade adayı; aktif işlem için eksik kanıt var"
	default:
		return "Aktif işlem sinyali yok; teknik kanıt kapısı geçmedi"
	}
}

func boolScoreFloat(value bool) float64 {
	if value {
		return 100
	}
	return 0
}

type weightedPart struct {
	score  float64
	weight float64
}

func weightedAverage(parts []weightedPart) float64 {
	sum := 0.0
	weight := 0.0
	for _, part := range parts {
		sum += part.score * part.weight
		weight += part.weight
	}
	return mathutil.SafeDiv(sum, weight)
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

func uniqueTechnicalStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		out = appendUniqueString(out, value)
	}
	return out
}

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func emptyString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func technicalSummary(score TechnicalScore, indicators []TechnicalIndicator, patterns []TechnicalPattern, guardrails []string) string {
	parts := []string{fmtFloat(score.Total) + "/100 technical evidence score"}
	if len(indicators) > 0 {
		parts = append(parts, strconv.Itoa(len(indicators))+" selected indicator signals")
	}
	if len(patterns) > 0 {
		parts = append(parts, strconv.Itoa(len(patterns))+" confirmed patterns")
	}
	if len(guardrails) > 0 {
		parts = append(parts, strconv.Itoa(len(guardrails))+" guardrail flags")
	}
	return strings.Join(parts, "; ")
}

func indicatorSignalCounts(indicators []ohlcv.IndicatorResult) map[string]int {
	counts := map[string]int{
		"total":      len(indicators),
		"computed":   0,
		"not_ready":  0,
		"proxy_only": 0,
	}
	for _, indicator := range indicators {
		if indicator.Computed {
			counts["computed"]++
		} else {
			counts["not_ready"]++
		}
		signal := strings.ToLower(strings.TrimSpace(indicator.Signal))
		if signal == "" {
			signal = "unknown"
		}
		counts[signal]++
	}
	return counts
}

func patternDirectionCounts(patterns []ohlcv.PatternResult, scans []ohlcv.PatternScanResult) map[string]int {
	counts := map[string]int{
		"confirmed": len(patterns),
		"catalog":   len(scans),
		"matched":   0,
	}
	for _, pattern := range patterns {
		key := directionClass(pattern.Direction)
		if key == "" {
			key = "neutral"
		}
		counts[key]++
	}
	for _, scan := range scans {
		if !scan.Matched {
			continue
		}
		counts["matched"]++
		key := "scan_" + directionClass(scan.Direction)
		if key == "scan_" {
			key = "scan_neutral"
		}
		counts[key]++
	}
	return counts
}

func isWeakIndicatorSignal(signal string) bool {
	switch strings.ToLower(strings.TrimSpace(signal)) {
	case "", "neutral", "info", "proxy_only", "requires_external_data", "not_computed", "insufficient_data", "unknown":
		return true
	default:
		return false
	}
}

func directionClass(direction string) string {
	value := strings.ToLower(strings.TrimSpace(direction))
	switch {
	case strings.Contains(value, "bull") || strings.Contains(value, "long") || strings.Contains(value, "up"):
		return "bullish"
	case strings.Contains(value, "bear") || strings.Contains(value, "short") || strings.Contains(value, "down"):
		return "bearish"
	default:
		return "neutral"
	}
}

func technicalPatternKey(name, category string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func technicalIndicatorKey(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.NewReplacer("-", " ", "_", " ", ".", " ", "/", " ").Replace(normalized)
	switch {
	case normalized == "adl" || strings.Contains(normalized, "accumulation distribution"):
		return "adl"
	case normalized == "adx" || strings.Contains(normalized, "average directional"):
		return "adx"
	case normalized == "rsi" || strings.Contains(normalized, "rsi") || strings.Contains(normalized, "relative strength index"):
		return "rsi"
	case strings.Contains(normalized, "macd") || strings.Contains(normalized, "moving average convergence divergence"):
		return "macd"
	case strings.Contains(normalized, "bollinger"):
		return "bollinger"
	case strings.Contains(normalized, "stochastic"):
		return "stochastic"
	case strings.Contains(normalized, "money flow") || normalized == "mfi":
		return "money_flow"
	case strings.Contains(normalized, "vwap"):
		return "vwap"
	case strings.Contains(normalized, "volume"):
		return "volume"
	case strings.Contains(normalized, "moving average") ||
		strings.Contains(normalized, "sma") ||
		strings.Contains(normalized, "ema") ||
		normalized == "dema" ||
		normalized == "tema" ||
		normalized == "alma" ||
		strings.Contains(normalized, "arnaud legoux") ||
		strings.Contains(normalized, "hull") ||
		strings.Contains(normalized, "kaufman") ||
		strings.Contains(normalized, "mesa adaptive"):
		return "moving_average"
	default:
		return normalized
	}
}

func balanceTechnicalIndicators(indicators []TechnicalIndicator, limit int) []TechnicalIndicator {
	if limit <= 0 || len(indicators) <= limit {
		return indicators
	}
	balanced := make([]TechnicalIndicator, 0, limit)
	bucketCounts := map[string]int{}
	for _, indicator := range indicators {
		bucket := technicalIndicatorBucket(indicator)
		if bucketCounts[bucket] >= technicalIndicatorBucketLimit(bucket) {
			continue
		}
		balanced = append(balanced, indicator)
		bucketCounts[bucket]++
		if len(balanced) >= limit {
			return balanced
		}
	}
	if len(balanced) == 0 {
		return indicators[:limit]
	}
	seen := map[string]bool{}
	for _, indicator := range balanced {
		seen[technicalIndicatorKey(indicator.Name)] = true
	}
	for _, indicator := range indicators {
		if len(balanced) >= limit {
			break
		}
		key := technicalIndicatorKey(indicator.Name)
		if seen[key] {
			continue
		}
		balanced = append(balanced, indicator)
		seen[key] = true
	}
	return balanced
}

func technicalIndicatorBucket(indicator TechnicalIndicator) string {
	name := strings.ToLower(indicator.Name + " " + indicator.Category + " " + indicator.Group)
	switch {
	case strings.Contains(name, "adx") || strings.Contains(name, "average directional"):
		return "trend_strength"
	case strings.Contains(name, "macd") || strings.Contains(name, "moving average convergence divergence"):
		return "macd"
	case strings.Contains(name, "rsi") ||
		strings.Contains(name, "stochastic") ||
		strings.Contains(name, "awesome") ||
		strings.Contains(name, "cci") ||
		strings.Contains(name, "williams") ||
		strings.Contains(name, "roc"):
		return "momentum"
	case strings.Contains(name, "bollinger") ||
		strings.Contains(name, "atr") ||
		strings.Contains(name, "keltner") ||
		strings.Contains(name, "donchian"):
		return "volatility"
	case strings.Contains(name, "volume") ||
		strings.Contains(name, "money flow") ||
		strings.Contains(name, "accumulation distribution") ||
		strings.Contains(name, "adl") ||
		strings.Contains(name, "obv") ||
		strings.Contains(name, "vwap") ||
		strings.Contains(name, "point of control") ||
		strings.Contains(name, "value area"):
		return "volume"
	case strings.Contains(name, "moving average") ||
		strings.Contains(name, "sma") ||
		strings.Contains(name, "ema") ||
		strings.Contains(name, "dema") ||
		strings.Contains(name, "tema"):
		return "moving_average"
	default:
		category := strings.ToLower(strings.TrimSpace(indicator.Category))
		if category == "" {
			return "other"
		}
		return category
	}
}

func technicalIndicatorBucketLimit(bucket string) int {
	switch bucket {
	case "moving_average", "trend_strength", "macd", "volatility":
		return 1
	case "momentum", "volume":
		return 2
	default:
		return 2
	}
}

func technicalIndicatorPriority(indicator TechnicalIndicator) float64 {
	name := strings.ToLower(indicator.Name + " " + indicator.Category + " " + indicator.Group)
	priority := indicator.Confidence * 100
	switch {
	case strings.Contains(name, "adx") || strings.Contains(name, "average directional"):
		priority += 18
	case strings.Contains(name, "macd") || strings.Contains(name, "moving average convergence divergence"):
		priority += 17
	case strings.Contains(name, "rsi") || strings.Contains(name, "relative strength index"):
		priority += 16
	case strings.Contains(name, "moving average") || strings.Contains(name, "sma") || strings.Contains(name, "ema"):
		priority += 14
	case strings.Contains(name, "bollinger"):
		priority += 12
	case strings.Contains(name, "volume") || strings.Contains(name, "money flow") || strings.Contains(name, "accumulation distribution") || strings.Contains(name, "adl"):
		priority += 10
	case strings.Contains(name, "vwap"):
		priority += 8
	case strings.Contains(name, "stochastic"):
		priority += 7
	}
	if directionClass(indicator.Signal) == "neutral" {
		priority -= 5
	}
	switch {
	case strings.Contains(name, "simple moving average") || strings.Contains(name, "sma"):
		priority += 4
	case strings.Contains(name, "exponential moving average") || strings.Contains(name, " ema"):
		priority += 3
	case strings.Contains(name, "dema") || strings.Contains(name, "double exponential"):
		priority += 1
	case strings.Contains(name, "arnaud legoux") || strings.Contains(name, "kaufman") || strings.Contains(name, "mesa adaptive"):
		priority -= 3
	}
	return priority
}

func fmtFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func normalizeOptions(opts Options) Options {
	if opts.BenchmarkSymbol == "" {
		opts.BenchmarkSymbol = "XU100"
	}
	opts.DataMode = strings.ToLower(strings.TrimSpace(opts.DataMode))
	if opts.DataMode == "" {
		opts.DataMode = "decision"
	}
	if opts.DataMode == "production" {
		opts.RequireVerifiedPublishDate = true
	}
	if opts.PortfolioValue <= 0 {
		opts.PortfolioValue = 100000
	}
	if opts.RiskPerTradePct <= 0 {
		opts.RiskPerTradePct = 1
	}
	if opts.PeerLimit <= 0 {
		opts.PeerLimit = 20
	}
	if opts.CommissionBps <= 0 {
		opts.CommissionBps = 5
	}
	if opts.SlippageBps <= 0 {
		opts.SlippageBps = 10
	}
	return opts
}

func companyProfile(input SymbolInput) CompanyProfile {
	name := input.CompanyName
	if name == "" {
		name = input.Symbol
	}
	profile := CompanyProfile{
		Symbol:                   input.Symbol,
		Name:                     name,
		Sector:                   inferSector(input.Symbol, name),
		SectorSource:             "symbol_name_heuristic",
		ClassificationConfidence: 0.30,
		ClassificationWarnings:   []string{"sector_classification_uses_name_heuristic"},
	}
	if classification, ok := loadSectorClassification(input.EquitiesDir, input.Symbol); ok {
		applySectorClassification(&profile, classification)
	}
	raw, err := os.ReadFile(filepath.Join(input.EquitiesDir, input.Symbol, "equity.json"))
	if err != nil {
		return profile
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return profile
	}
	if value := stringValue(payload["name"]); value != "" {
		profile.Name = value
		if profile.SectorSource == "symbol_name_heuristic" {
			profile.Sector = inferSector(input.Symbol, value)
		}
	}
	if kap, ok := payload["kap_info"].(map[string]any); ok {
		profile.RegisteredCapitalCeiling = numberValue(kap["kayitliSermayeTavani"])
		if sector, ok := officialSectorFromKAP(kap); ok && profile.ClassificationConfidence < 0.80 {
			profile.Sector = sector
			profile.SectorSource = "kap_financial_type"
			profile.ClassificationConfidence = 0.80
			profile.ClassificationWarnings = nil
		} else if sector := inferSectorFromKAP(input.Symbol, profile.Name, kap); sector != "" && profile.ClassificationConfidence < 0.50 {
			profile.Sector = sector
			profile.SectorSource = "kap_title_heuristic"
			profile.ClassificationConfidence = 0.45
			profile.ClassificationWarnings = []string{"sector_classification_uses_kap_title_heuristic"}
		}
	}
	if rawKAP, err := os.ReadFile(filepath.Join(input.EquitiesDir, input.Symbol, "kap.json")); err == nil {
		var kap map[string]any
		if json.Unmarshal(rawKAP, &kap) == nil && len(kap) > 0 && kap["available"] != false {
			if profile.RegisteredCapitalCeiling == 0 {
				profile.RegisteredCapitalCeiling = numberValue(kap["kayitliSermayeTavani"])
			}
			if sector, ok := officialSectorFromKAP(kap); ok && profile.ClassificationConfidence < 0.80 {
				profile.Sector = sector
				profile.SectorSource = "kap_financial_type"
				profile.ClassificationConfidence = 0.80
				profile.ClassificationWarnings = nil
			} else if sector := inferSectorFromKAP(input.Symbol, profile.Name, kap); sector != "" && profile.ClassificationConfidence < 0.50 {
				profile.Sector = sector
				profile.SectorSource = "kap_title_heuristic"
				profile.ClassificationConfidence = 0.45
				profile.ClassificationWarnings = []string{"sector_classification_uses_kap_title_heuristic"}
			}
		}
	}
	return profile
}

func buildValuation(fin financialFile, price float64, latest period, sector string, assumptionsFile string) ValuationAnalysis {
	schema := financialFieldSchemaForContext(sector, fin)
	paidCapital := schemaFieldValueOrZero(fin, schema, fieldPaidCapital, latest)
	equity := schemaFieldValueOrZero(fin, schema, fieldEquity, latest)
	totalAssets := schemaFieldValueOrZero(fin, schema, fieldTotalAssets, latest)
	cash := schemaFieldValueOrZero(fin, schema, fieldCash, latest)
	debt, _, okDebt := schemaSumFieldValues(fin, schema, fieldDebt, latest)
	netDebt := debt - cash
	marketCap := price * paidCapital
	flags := append([]string{}, schema.Warnings...)
	sales, _, okSales := schemaTTM(fin, schema, fieldRevenue, latest)
	if !okSales {
		flags = append(flags, "revenue_ttm_inputs_missing")
	}
	ebit, _, okEBIT := schemaTTM(fin, schema, fieldEBIT, latest)
	if !okEBIT {
		flags = append(flags, "ebit_ttm_inputs_missing")
	}
	amortization, _, okAmortization := schemaTTM(fin, schema, fieldAmortization, latest)
	if !okAmortization {
		flags = append(flags, "amortization_ttm_inputs_missing")
	}
	ebitda := ebit + math.Max(amortization, 0)
	netIncome, _, okNetIncome := schemaTTM(fin, schema, fieldNetIncome, latest)
	if !okNetIncome {
		flags = append(flags, "net_income_ttm_inputs_missing")
	}
	operatingCash, _, okOperatingCash := schemaTTM(fin, schema, fieldOperatingCash, latest)
	if !okOperatingCash {
		flags = append(flags, "operating_cash_ttm_inputs_missing")
	}
	freeCash, _, okFreeCash := schemaTTM(fin, schema, fieldFreeCashFlow, latest)
	if !okFreeCash {
		flags = append(flags, "free_cash_flow_ttm_inputs_missing")
	}
	ratios := map[string]float64{
		"PE":           positiveDiv(marketCap, netIncome),
		"PB":           positiveDiv(marketCap, equity),
		"PS":           positiveDiv(marketCap, sales),
		"EV_Sales":     positiveDiv(marketCap+netDebt, sales),
		"EV_EBIT":      positiveDiv(marketCap+netDebt, ebit),
		"EV_EBITDA":    positiveDiv(marketCap+netDebt, ebitda),
		"ROE":          mathutil.SafeDiv(netIncome, equity),
		"ROA":          mathutil.SafeDiv(netIncome, totalAssets),
		"Net_Margin":   mathutil.SafeDiv(netIncome, sales),
		"FCF_Yield":    mathutil.SafeDiv(freeCash, marketCap),
		"NetDebt_Eq":   mathutil.SafeDiv(netDebt, equity),
		"BookPerShare": mathutil.SafeDiv(equity, paidCapital),
		"EPS":          mathutil.SafeDiv(netIncome, paidCapital),
	}
	if netIncome <= 0 {
		flags = append(flags, "negative_ttm_net_income")
	}
	if freeCash < 0 {
		flags = append(flags, "negative_ttm_free_cash_flow")
	}
	if freeCash < 0 && netIncome > 0 && mathutil.SafeDiv(freeCash, netIncome) < -0.30 {
		flags = append(flags, "income_quality_gap_fcf_negative_net_income_positive")
	}
	if paidCapital <= 0 {
		flags = append(flags, "paid_capital_unavailable")
	}
	valuation := ValuationAnalysis{
		LatestYear:        latest.Year,
		LatestQuarter:     quarterName(latest.Quarter),
		PaidCapital:       paidCapital,
		MarketCap:         marketCap,
		EnterpriseValue:   marketCap + netDebt,
		TotalDebt:         debt,
		DebtDataAvailable: okDebt,
		NetDebt:           netDebt,
		SalesTTM:          sales,
		EBITTTM:           ebit,
		EBITDATTM:         ebitda,
		NetIncomeTTM:      netIncome,
		OperatingCashTTM:  operatingCash,
		FreeCashFlowTTM:   freeCash,
		Equity:            equity,
		TotalAssets:       totalAssets,
		Ratios:            ratios,
		SectorMetrics:     sectorMetrics(sector, equity, totalAssets, netDebt, sales, netIncome, paidCapital),
		Flags:             flags,
	}
	applySectorValuationRules(&valuation, sector)
	applyBankBookValueReconciliation(&valuation)
	valuation.DCF = buildDCF(valuation, sector, assumptionsFile)
	return valuation
}

func buildDCF(v ValuationAnalysis, sector string, assumptionsFile string) DCFAnalysis {
	assumptions := dcfAssumptionsForSector(sector, assumptionsFile)
	out := DCFAnalysis{
		WACC:                assumptions.WACC,
		TerminalGrowth:      assumptions.TerminalGrowth,
		RiskFreeRate:        assumptions.RiskFreeRate,
		Beta:                assumptions.Beta,
		EquityRiskPremium:   assumptions.EquityRiskPremium,
		CostOfEquity:        assumptions.CostOfEquity,
		CostOfDebt:          assumptions.CostOfDebt,
		TaxRate:             assumptions.TaxRate,
		FCFGrowthAssumption: assumptions.FCFGrowth,
		AssumptionSource:    assumptions.Source,
	}
	if dcfNotApplicableForSectorModel(v.SectorModel) {
		out.Warnings = append(out.Warnings, "dcf_not_applicable_for_financial_sector_model")
		return out
	}
	if v.FreeCashFlowTTM <= 0 {
		out.Warnings = append(out.Warnings, "dcf_not_computed_non_positive_fcf")
		return out
	}
	if v.PaidCapital <= 0 {
		out.Warnings = append(out.Warnings, "dcf_not_computed_paid_capital_missing")
		return out
	}
	if out.WACC <= out.TerminalGrowth {
		out.Warnings = append(out.Warnings, "dcf_not_computed_invalid_wacc_terminal_growth")
		return out
	}
	ev := dcfEnterpriseValue(v.FreeCashFlowTTM, assumptions.FCFGrowth, out.WACC, out.TerminalGrowth, 5)
	out.EnterpriseValue = ev
	out.EquityValue = ev - v.NetDebt
	out.FairValuePerShare = mathutil.SafeDiv(out.EquityValue, v.PaidCapital)
	out.Computed = out.FairValuePerShare > 0
	if !out.Computed {
		out.Warnings = append(out.Warnings, "dcf_equity_value_not_positive")
	}
	out.Sensitivity = []DCFScenario{
		dcfScenario("bear", v.FreeCashFlowTTM, v.NetDebt, v.PaidCapital, assumptions.FCFGrowth-0.02, out.WACC+0.02, out.TerminalGrowth-0.01),
		dcfScenario("base", v.FreeCashFlowTTM, v.NetDebt, v.PaidCapital, assumptions.FCFGrowth, out.WACC, out.TerminalGrowth),
		dcfScenario("bull", v.FreeCashFlowTTM, v.NetDebt, v.PaidCapital, assumptions.FCFGrowth+0.02, out.WACC-0.02, out.TerminalGrowth+0.01),
	}
	return out
}

func dcfNotApplicableForSectorModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "bank_equity_model", "insurance_equity_model":
		return true
	default:
		return false
	}
}

func applyBankBookValueReconciliation(v *ValuationAnalysis) {
	if v == nil || v.SectorModel != "bank_equity_model" {
		return
	}
	bookPerShare := v.Ratios["BookPerShare"]
	expected := mathutil.SafeDiv(v.Equity, v.PaidCapital)
	if v.Equity <= 0 || v.PaidCapital <= 0 || bookPerShare <= 0 || expected <= 0 {
		v.Flags = uniqueStrings(append(v.Flags, "bank_book_value_reconciliation_missing"))
		return
	}
	tolerance := math.Max(0.01, math.Abs(expected)*0.01)
	if math.Abs(bookPerShare-expected) > tolerance {
		v.Flags = uniqueStrings(append(v.Flags, "bank_book_value_reconciliation_failed"))
		return
	}
	v.Flags = uniqueStrings(append(v.Flags, "bank_book_value_reconciliation_passed"))
}

type dcfAssumptions struct {
	Source            string
	RiskFreeRate      float64
	Beta              float64
	EquityRiskPremium float64
	CostOfEquity      float64
	CostOfDebt        float64
	TaxRate           float64
	WACC              float64
	TerminalGrowth    float64
	FCFGrowth         float64
}

func dcfAssumptionsForSector(sector string, file string) dcfAssumptions {
	if loaded, ok := loadDCFValuationAssumptions(file, sector); ok {
		return loaded
	}
	normalized := util.SlugTR(sector)
	out := dcfAssumptions{
		Source:            "default_static",
		RiskFreeRate:      0.30,
		Beta:              1.00,
		EquityRiskPremium: 0.08,
		CostOfDebt:        0.35,
		TaxRate:           0.25,
		TerminalGrowth:    0.05,
		FCFGrowth:         0.08,
	}
	out.CostOfEquity = out.RiskFreeRate + out.Beta*out.EquityRiskPremium
	out.WACC = 0.70*out.CostOfEquity + 0.30*out.CostOfDebt*(1-out.TaxRate)
	if strings.Contains(normalized, "bank") || strings.Contains(normalized, "sigorta") {
		out.WACC = out.CostOfEquity
		out.TerminalGrowth = 0.04
		out.FCFGrowth = 0.05
	}
	return out
}

type dcfAssumptionFile struct {
	Default dcfAssumptionRecord            `json:"default"`
	Sectors map[string]dcfAssumptionRecord `json:"sectors"`
}

type dcfAssumptionRecord struct {
	RiskFreeRate      float64 `json:"risk_free_rate"`
	Beta              float64 `json:"beta"`
	EquityRiskPremium float64 `json:"equity_risk_premium"`
	CostOfDebt        float64 `json:"cost_of_debt"`
	TaxRate           float64 `json:"tax_rate"`
	WACC              float64 `json:"wacc"`
	TerminalGrowth    float64 `json:"terminal_growth"`
	FCFGrowth         float64 `json:"fcf_growth"`
}

func loadDCFValuationAssumptions(file string, sector string) (dcfAssumptions, bool) {
	if strings.TrimSpace(file) == "" {
		return dcfAssumptions{}, false
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return dcfAssumptions{}, false
	}
	var payload dcfAssumptionFile
	if json.Unmarshal(raw, &payload) != nil {
		return dcfAssumptions{}, false
	}
	record := payload.Default
	for key, candidate := range payload.Sectors {
		if strings.Contains(strings.ToLower(sector), strings.ToLower(key)) {
			record = candidate
			break
		}
	}
	out := dcfAssumptions{
		Source:            file,
		RiskFreeRate:      record.RiskFreeRate,
		Beta:              record.Beta,
		EquityRiskPremium: record.EquityRiskPremium,
		CostOfDebt:        record.CostOfDebt,
		TaxRate:           record.TaxRate,
		WACC:              record.WACC,
		TerminalGrowth:    record.TerminalGrowth,
		FCFGrowth:         record.FCFGrowth,
	}
	if out.Beta == 0 {
		out.Beta = 1
	}
	if out.EquityRiskPremium == 0 {
		out.EquityRiskPremium = 0.08
	}
	if out.RiskFreeRate == 0 {
		out.RiskFreeRate = 0.30
	}
	if out.CostOfEquity == 0 {
		out.CostOfEquity = out.RiskFreeRate + out.Beta*out.EquityRiskPremium
	}
	if out.TaxRate == 0 {
		out.TaxRate = 0.25
	}
	if out.CostOfDebt == 0 {
		out.CostOfDebt = 0.35
	}
	if out.WACC == 0 {
		out.WACC = 0.70*out.CostOfEquity + 0.30*out.CostOfDebt*(1-out.TaxRate)
	}
	if out.TerminalGrowth == 0 {
		out.TerminalGrowth = 0.05
	}
	if out.FCFGrowth == 0 {
		out.FCFGrowth = 0.08
	}
	return out, true
}

func dcfEnterpriseValue(fcf, growth, wacc, terminalGrowth float64, years int) float64 {
	if years <= 0 || fcf <= 0 || wacc <= terminalGrowth {
		return 0
	}
	value := 0.0
	cashFlow := fcf
	for year := 1; year <= years; year++ {
		cashFlow *= 1 + growth
		value += cashFlow / math.Pow(1+wacc, float64(year))
	}
	terminal := cashFlow * (1 + terminalGrowth) / (wacc - terminalGrowth)
	value += terminal / math.Pow(1+wacc, float64(years))
	return value
}

func dcfScenario(name string, fcf, netDebt, paidCapital, growth, wacc, terminalGrowth float64) DCFScenario {
	ev := dcfEnterpriseValue(fcf, growth, wacc, terminalGrowth, 5)
	return DCFScenario{
		Name:              name,
		WACC:              wacc,
		TerminalGrowth:    terminalGrowth,
		FairValuePerShare: mathutil.SafeDiv(ev-netDebt, paidCapital),
	}
}

func sectorMetrics(sector string, equity, totalAssets, netDebt, sales, netIncome, paidCapital float64) map[string]float64 {
	metrics := map[string]float64{}
	normalized := util.SlugTR(sector)
	if strings.Contains(normalized, "bank") {
		metrics["EquityToAssets"] = mathutil.SafeDiv(equity, totalAssets)
		metrics["AssetsToEquity"] = mathutil.SafeDiv(totalAssets, equity)
		metrics["BookPerShare"] = mathutil.SafeDiv(equity, paidCapital)
	}
	if strings.Contains(normalized, "sigorta") {
		metrics["EquityToAssets"] = mathutil.SafeDiv(equity, totalAssets)
		metrics["NetMargin"] = mathutil.SafeDiv(netIncome, sales)
	}
	if strings.Contains(normalized, "gayrimenkul") {
		metrics["NetDebtToEquity"] = mathutil.SafeDiv(netDebt, equity)
		metrics["BookPerShare"] = mathutil.SafeDiv(equity, paidCapital)
	}
	return metrics
}

type sectorValuationRule struct {
	Model      string
	Allowed    []string
	Suppressed []string
	Flags      []string
}

func applySectorValuationRules(v *ValuationAnalysis, sector string) {
	rule := valuationRuleForSector(sector)
	v.SectorModel = rule.Model
	v.AllowedRatios = append([]string{}, rule.Allowed...)
	v.SuppressedRatios = append([]string{}, rule.Suppressed...)
	for _, key := range rule.Suppressed {
		delete(v.Ratios, key)
	}
	v.Flags = append(v.Flags, rule.Flags...)
}

func valuationRuleForSector(sector string) sectorValuationRule {
	normalized := util.SlugTR(sector)
	switch {
	case strings.Contains(normalized, "bank"):
		return sectorValuationRule{
			Model:      "bank_equity_model",
			Allowed:    []string{"PE", "PB", "ROE", "ROA", "BookPerShare", "EPS"},
			Suppressed: []string{"PS", "EV_Sales", "EV_EBIT", "EV_EBITDA", "FCF_Yield", "NetDebt_Eq"},
			Flags:      []string{"bank_sector_requires_regulatory_capital_and_asset_quality_model"},
		}
	case strings.Contains(normalized, "sigorta"):
		return sectorValuationRule{
			Model:      "insurance_equity_model",
			Allowed:    []string{"PE", "PB", "ROE", "ROA", "BookPerShare", "EPS", "Net_Margin"},
			Suppressed: []string{"EV_Sales", "EV_EBIT", "EV_EBITDA", "FCF_Yield", "NetDebt_Eq"},
			Flags:      []string{"insurance_sector_requires_premium_reserve_and_solvency_model"},
		}
	case strings.Contains(normalized, "gayrimenkul"):
		return sectorValuationRule{
			Model:      "reit_nav_proxy_model",
			Allowed:    []string{"PE", "PB", "PS", "ROE", "ROA", "BookPerShare", "EPS", "NetDebt_Eq"},
			Suppressed: []string{"EV_EBIT", "EV_EBITDA", "FCF_Yield"},
			Flags:      []string{"reit_sector_requires_nav_and_portfolio_appraisal_model"},
		}
	default:
		return sectorValuationRule{
			Model:   "industrial_operating_company_model",
			Allowed: []string{"PE", "PB", "PS", "EV_Sales", "EV_EBIT", "EV_EBITDA", "ROE", "ROA", "Net_Margin", "FCF_Yield", "NetDebt_Eq", "BookPerShare", "EPS"},
		}
	}
}

func buildPeerComparison(equitiesDir, symbol string, profile CompanyProfile, price float64, target ValuationAnalysis, limit int, assumptionsFile string, opts Options, asOf time.Time) PeerComparison {
	out := PeerComparison{
		Sector:      profile.Sector,
		PeerGroup:   profile.PeerGroup,
		Medians:     map[string]float64{},
		Percentiles: map[string]float64{},
	}
	entries, err := os.ReadDir(equitiesDir)
	if err != nil {
		return out
	}
	manualPeers := peerSymbolSet(profile.PeerSymbols)
	bankUniverse := bankPeerUniverse(profile, target)
	excludedNonBankPeers := 0
	peers := []PeerMetric{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ticker := strings.ToUpper(entry.Name())
		if ticker == "" || ticker == symbol {
			continue
		}
		equityPath := filepath.Join(equitiesDir, ticker, "equity.json")
		name, eqPrice := equitySummary(equityPath)
		if len(manualPeers) > 0 {
			if !manualPeers[ticker] {
				continue
			}
		} else if !sameSector(ticker, name, profile.Sector) {
			continue
		}
		if eqPrice <= 0 {
			continue
		}
		fin, ok := loadFinancialsForSymbol(equitiesDir, ticker)
		if !ok {
			continue
		}
		peerSector := inferSector(ticker, name)
		peerIndustry := ""
		if classification, ok := loadSectorClassification(equitiesDir, ticker); ok && classification.Sector != "" {
			peerSector = classification.Sector
			peerIndustry = classification.Industry
		}
		valuationContext := strings.TrimSpace(strings.Join([]string{peerSector, peerIndustry, fin.FinancialGroup}, " "))
		if bankUniverse && !cleanBankPeerCandidate(ticker, name, peerSector, peerIndustry, fin.FinancialGroup) {
			excludedNonBankPeers++
			continue
		}
		valuation := buildValuation(fin, eqPrice, valuationPeriod(fin, opts, asOf), valuationContext, assumptionsFile)
		if valuation.MarketCap <= 0 {
			continue
		}
		peerRatios := map[string]float64{
			"PE": valuation.Ratios["PE"],
			"PB": valuation.Ratios["PB"],
		}
		peerMetrics := map[string]float64{
			"ROE": valuation.Ratios["ROE"],
		}
		if !bankUniverse {
			peerRatios["PS"] = valuation.Ratios["PS"]
			peerRatios["EV_Sales"] = valuation.Ratios["EV_Sales"]
			peerRatios["EV_EBITDA"] = valuation.Ratios["EV_EBITDA"]
			peerMetrics["Net_Margin"] = valuation.Ratios["Net_Margin"]
			peerMetrics["FCF_Yield"] = valuation.Ratios["FCF_Yield"]
		}
		peers = append(peers, PeerMetric{
			Symbol:    ticker,
			Name:      name,
			Price:     eqPrice,
			MarketCap: valuation.MarketCap,
			Ratios:    peerRatios,
			Metrics:   peerMetrics,
			Period:    strconv.Itoa(valuation.LatestYear) + " " + valuation.LatestQuarter,
		})
	}
	if bankUniverse && excludedNonBankPeers > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("bank_peer_non_bank_excluded_%d", excludedNonBankPeers))
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Symbol == symbol {
			return true
		}
		if peers[j].Symbol == symbol {
			return false
		}
		return peers[i].MarketCap > peers[j].MarketCap
	})
	out.PeerCount = len(peers)
	ratioKeys := []string{"PE", "PB", "PS", "EV_Sales", "EV_EBITDA"}
	if bankUniverse {
		ratioKeys = []string{"PE", "PB"}
		out.Warnings = append(out.Warnings, "bank_peer_medians_restricted_to_pe_pb")
	}
	for _, key := range ratioKeys {
		values := []float64{}
		filtered := 0
		for _, peer := range peers {
			value := peer.Ratios[key]
			if peerRatioUsableForMedian(key, value, bankUniverse) {
				values = append(values, value)
			} else if value > 0 && value < 500 && bankPeerOutlierMetric(key, bankUniverse) {
				filtered++
			}
		}
		if filtered > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("bank_peer_outlier_filter_applied_%s_%d", strings.ToLower(key), filtered))
		}
		out.Medians[key] = median(values)
		targetValue := target.Ratios[key]
		if targetValue > 0 {
			out.Percentiles[key] = percentileRank(values, targetValue)
		}
	}
	out.ValuationSignal = valuationSignal(out.Percentiles)
	if len(peers) > limit {
		out.Peers = peers[:limit]
	} else {
		out.Peers = peers
	}
	return out
}

func cleanBankPeerCandidate(ticker, name, sector, industry, financialGroup string) bool {
	text := util.SlugTR(strings.Join([]string{ticker, name, sector, industry, financialGroup}, " "))
	group := strings.ToUpper(strings.TrimSpace(financialGroup))
	excluded := []string{
		"sigorta",
		"insurance",
		"leasing",
		"faktoring",
		"factoring",
		"holding",
		"gayrimenkul",
		"gyo",
		"yatirimortakligi",
		"menkulkiymet",
		"aracikurum",
		"finansalkiralama",
	}
	for _, token := range excluded {
		if strings.Contains(text, token) {
			return false
		}
	}
	return group == "UFRS_K" ||
		strings.Contains(text, "bank") ||
		strings.Contains(text, "banka") ||
		strings.Contains(text, "bankacilik")
}

func bankPeerUniverse(profile CompanyProfile, target ValuationAnalysis) bool {
	text := util.SlugTR(strings.Join([]string{
		profile.Sector,
		profile.Industry,
		profile.PeerGroup,
		target.SectorModel,
	}, " "))
	if strings.Contains(text, "bank") {
		return true
	}
	return containsString(target.Flags, "bank_sector_requires_regulatory_capital_and_asset_quality_model")
}

func bankPeerOutlierMetric(key string, bankUniverse bool) bool {
	if !bankUniverse {
		return false
	}
	switch key {
	case "PE", "PB":
		return true
	default:
		return false
	}
}

func peerRatioUsableForMedian(key string, value float64, bankUniverse bool) bool {
	if value <= 0 || value >= 500 {
		return false
	}
	if !bankUniverse {
		return true
	}
	switch key {
	case "PE":
		return value <= 20
	case "PB":
		return value <= 4
	default:
		return true
	}
}

func buildMarketContext(input SymbolInput, opts Options) MarketContext {
	liveSnapshot := loadLiveMarketSnapshot(input.EquitiesDir, opts.MarketSnapshotFile)
	var microstructure *MarketMicrostructureContext
	if !ohlcv.IsCommodityAssetType(input.AssetType) && !ohlcv.IsCryptoAssetType(input.AssetType) {
		microstructure = loadMarketMicrostructure(input.EquitiesDir, input.Symbol)
		if liveSnapshot == nil {
			liveSnapshot = liveSnapshotFromMarketMicrostructure(input.Symbol, microstructure)
		}
	}
	ctx := MarketContext{
		BenchmarkSymbol:       opts.BenchmarkSymbol,
		BenchmarkError:        input.BenchmarkError,
		SectorBenchmarkSymbol: strings.ToUpper(strings.TrimSpace(input.SectorBenchmarkSymbol)),
		SectorBenchmarkError:  input.SectorBenchmarkError,
		LiveSnapshot:          liveSnapshot,
		Microstructure:        microstructure,
	}
	daily := dailyCandlesFromInput(input)
	stockReturns := returns(closes(daily))
	ctx.StockReturn20 = rocFromCandles(daily, 20)
	ctx.StockReturn60 = rocFromCandles(daily, 60)
	ctx.StockReturn120 = rocFromCandles(daily, 120)
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		ctx.BenchmarkSymbol = "DXY / ABD reel faiz / COMEX COT / altin ETF akisi"
		ctx.BenchmarkError = ""
		ctx.SectorBenchmarkSymbol = ""
		ctx.SectorBenchmarkError = ""
		return ctx
	}
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		ctx.BenchmarkSymbol = "kripto piyasa rejimi / BTC dominansi / DXY / derivatives"
		ctx.BenchmarkError = ""
		ctx.SectorBenchmarkSymbol = ""
		ctx.SectorBenchmarkError = ""
		return ctx
	}
	ctx.GDP = buildGDPContext(input, opts)
	if len(input.BenchmarkCandles) > 0 {
		ctx.BenchmarkAvailable = true
		ctx.BenchmarkReturn20 = rocFromCandles(input.BenchmarkCandles, 20)
		ctx.BenchmarkReturn60 = rocFromCandles(input.BenchmarkCandles, 60)
		ctx.BenchmarkReturn120 = rocFromCandles(input.BenchmarkCandles, 120)
		ctx.RelativeStrength20 = ctx.StockReturn20 - ctx.BenchmarkReturn20
		ctx.RelativeStrength60 = ctx.StockReturn60 - ctx.BenchmarkReturn60
		benchmarkReturns := returns(closes(input.BenchmarkCandles))
		alignedStock, alignedBenchmark := alignFloatSeries(stockReturns, benchmarkReturns)
		stock60 := lastN(alignedStock, 60)
		benchmark60 := lastN(alignedBenchmark, 60)
		ctx.Beta60 = beta(stock60, benchmark60)
		ctx.Correlation60 = correlation(stock60, benchmark60)
		ctx.Alpha60 = mathutil.Mean(stock60)*252 - ctx.Beta60*mathutil.Mean(benchmark60)*252
	}
	if ctx.SectorBenchmarkSymbol != "" && len(input.SectorBenchmarkCandles) > 0 {
		ctx.SectorBenchmarkAvailable = true
		ctx.SectorBenchmarkReturn20 = rocFromCandles(input.SectorBenchmarkCandles, 20)
		ctx.SectorBenchmarkReturn60 = rocFromCandles(input.SectorBenchmarkCandles, 60)
		ctx.SectorBenchmarkReturn120 = rocFromCandles(input.SectorBenchmarkCandles, 120)
		ctx.SectorRelativeStrength20 = ctx.StockReturn20 - ctx.SectorBenchmarkReturn20
		ctx.SectorRelativeStrength60 = ctx.StockReturn60 - ctx.SectorBenchmarkReturn60
		sectorReturns := returns(closes(input.SectorBenchmarkCandles))
		alignedStock, alignedSector := alignFloatSeries(stockReturns, sectorReturns)
		stock60 := lastN(alignedStock, 60)
		sector60 := lastN(alignedSector, 60)
		ctx.SectorBeta60 = beta(stock60, sector60)
		ctx.SectorCorrelation60 = correlation(stock60, sector60)
		ctx.SectorAlpha60 = mathutil.Mean(stock60)*252 - ctx.SectorBeta60*mathutil.Mean(sector60)*252
	}
	return ctx
}

func loadLiveMarketSnapshot(equitiesDir, path string) *marketdata.LiveMarketSnapshot {
	path = strings.TrimSpace(path)
	if path == "" {
		path = filepath.Join(filepath.Dir(equitiesDir), "market", "live_snapshot.json")
	}
	var snapshot marketdata.LiveMarketSnapshot
	if err := util.ReadJSON(path, &snapshot); err != nil {
		return nil
	}
	if !snapshot.HasData() {
		return nil
	}
	return &snapshot
}

func buildGDPContext(input SymbolInput, opts Options) macro.GDPContext {
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		return macro.GDPContext{}
	}
	path := strings.TrimSpace(opts.MacroGDPFile)
	if path == "" {
		path = macro.DefaultGDPPathFromEquitiesDir(input.EquitiesDir)
	}
	dataset, ok, err := macro.LoadGDPDataset(path)
	if err != nil {
		return macro.GDPContext{
			Computed:           false,
			Source:             "TÜİK CİP",
			DataQualityWarning: err.Error(),
			RequiredCaveats:    []string{"GSYH makro etkisi veri dosyası okunamadığı için skorlanmadı."},
		}
	}
	if !ok {
		return macro.GDPContext{
			Computed:           false,
			Source:             "TÜİK CİP",
			SourceURL:          "https://cip.tuik.gov.tr/",
			DataQualityWarning: "TÜİK CİP GSYH veri dosyası bulunamadı: " + path,
			RequiredCaveats:    []string{"Önce `go run ./cmd/hissebot sync tuik-gdp` komutu ile GSYH makro verisi çekilmelidir."},
		}
	}
	return macro.AnalyzeGDP(dataset)
}

func buildDisclosureReview(equitiesDir, symbol string) DisclosureReview {
	out := DisclosureReview{
		RecentDisclosureStatus: "unavailable",
		RequiredSources:        []string{"KAP material-disclosure feed", "news/headline feed"},
	}
	if raw, err := os.ReadFile(filepath.Join(equitiesDir, symbol, "kap.json")); err == nil && len(raw) > 0 {
		var payload map[string]any
		if json.Unmarshal(raw, &payload) == nil && len(payload) > 0 && payload["available"] != false {
			out.KAPCompanyCardAvailable = true
		}
	}
	if raw, err := os.ReadFile(filepath.Join(equitiesDir, symbol, "kap_disclosures.json")); err == nil && len(raw) > 0 {
		out.RecentDisclosureStatus = "available"
		out.RecentDisclosureCount = countJSONRecords(raw)
	}
	var comments []any
	if raw, err := os.ReadFile(filepath.Join(equitiesDir, symbol, "comments.json")); err == nil {
		_ = json.Unmarshal(raw, &comments)
		out.LocalCommentCount = len(comments)
	}
	if !out.KAPCompanyCardAvailable {
		out.RiskFlags = append(out.RiskFlags, "kap_company_card_missing")
	}
	if out.RecentDisclosureStatus != "available" {
		out.RiskFlags = append(out.RiskFlags, "recent_kap_disclosures_not_connected")
	}
	return out
}

func countJSONRecords(raw []byte) int {
	var list []any
	if json.Unmarshal(raw, &list) == nil {
		return len(list)
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return 0
	}
	for _, key := range []string{"disclosures", "value", "items", "data"} {
		if list, ok := object[key].([]any); ok {
			return len(list)
		}
	}
	return 1
}

type equityNewsSentimentRecord struct {
	Title       string
	Summary     string
	Source      string
	PublishedAt time.Time
}

func analyzeEquityNewsSentiment(equitiesDir, symbol string, asOf time.Time) EquityNewsSentimentReport {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := EquityNewsSentimentReport{}
	if symbol == "" {
		out.Warnings = []string{"news_sentiment_symbol_missing"}
		return out
	}
	paths := []string{
		filepath.Join(equitiesDir, symbol, "news_sentiment.json"),
		filepath.Join(equitiesDir, symbol, "kap_disclosures.json"),
	}
	records := []equityNewsSentimentRecord{}
	sourcePaths := []string{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) == 0 {
			continue
		}
		parsed := parseEquityNewsSentimentRecords(raw)
		if len(parsed) == 0 {
			continue
		}
		records = append(records, parsed...)
		sourcePaths = append(sourcePaths, path)
	}
	records = dedupeEquityNewsSentimentRecords(records)
	out.SourcePath = strings.Join(sourcePaths, ";")
	out.ItemCount = len(records)
	if len(records) == 0 {
		out.Warnings = []string{"news_sentiment_source_missing"}
		out.Summary = "KAP/haber duyarlılığı için news_sentiment.json veya kap_disclosures.json bulunamadı."
		return out
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].PublishedAt.After(records[j].PublishedAt)
	})
	if asOf.IsZero() {
		asOf = time.Now()
	}
	totalScore := 0.0
	for _, record := range records {
		if !record.PublishedAt.IsZero() {
			if record.PublishedAt.After(asOf.Add(24 * time.Hour)) {
				continue
			}
			if record.PublishedAt.Before(asOf.AddDate(0, 0, -45)) {
				continue
			}
		}
		score, label := equityHeadlineSentimentScore(record.Title + " " + record.Summary)
		switch label {
		case "positive":
			out.PositiveCount++
		case "negative":
			out.NegativeCount++
		default:
			out.NeutralCount++
		}
		out.RecentItemCount++
		totalScore += score
		if len(out.Items) < 12 {
			out.Items = append(out.Items, EquityNewsSentimentItem{
				Title:       record.Title,
				Source:      record.Source,
				PublishedAt: formatNewsSentimentDate(record.PublishedAt),
				Score:       roundProfessionalMetric(score),
				Label:       label,
			})
		}
	}
	if out.RecentItemCount == 0 {
		out.Warnings = append(out.Warnings, "news_sentiment_recent_window_empty")
		out.Summary = "KAP/haber kaynağı var ancak son 45 günlük pencerede kullanılabilir başlık yok."
		return out
	}
	out.Computed = true
	out.Score = roundProfessionalMetric(mathutil.Clamp((totalScore/float64(out.RecentItemCount))/3*100, -100, 100))
	switch {
	case out.Score >= 15:
		out.Signal = "positive"
	case out.Score <= -15:
		out.Signal = "negative"
	default:
		out.Signal = "neutral"
	}
	out.Summary = fmt.Sprintf("KAP/haber duyarlılığı: %d son kayıt, pozitif=%d negatif=%d nötr=%d, skor %.0f/100, sinyal=%s.",
		out.RecentItemCount,
		out.PositiveCount,
		out.NegativeCount,
		out.NeutralCount,
		out.Score,
		out.Signal,
	)
	return out
}

func dedupeEquityNewsSentimentRecords(records []equityNewsSentimentRecord) []equityNewsSentimentRecord {
	out := make([]equityNewsSentimentRecord, 0, len(records))
	seen := map[string]bool{}
	for _, record := range records {
		key := util.SlugTR(record.Title + "|" + record.Summary + "|" + formatNewsSentimentDate(record.PublishedAt))
		if key == "" {
			key = util.SlugTR(record.Title + "|" + record.Summary)
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, record)
	}
	return out
}

func parseEquityNewsSentimentRecords(raw []byte) []equityNewsSentimentRecord {
	var payload any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	rows := []any{}
	switch typed := payload.(type) {
	case []any:
		rows = typed
	case map[string]any:
		if items, ok := typed["items"].([]any); ok {
			rows = items
		} else if data, ok := typed["data"].([]any); ok {
			rows = data
		} else if disclosures, ok := typed["disclosures"].([]any); ok {
			rows = disclosures
		}
	}
	out := make([]equityNewsSentimentRecord, 0, len(rows))
	for _, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		title := firstNonEmptyString(
			stringValue(obj["title"]),
			stringValue(obj["headline"]),
			stringValue(obj["subject"]),
			stringValue(obj["disclosure_title"]),
			stringValue(obj["notification_type"]),
		)
		summary := firstNonEmptyString(
			stringValue(obj["summary"]),
			stringValue(obj["content"]),
			stringValue(obj["description"]),
			stringValue(obj["text"]),
		)
		if strings.TrimSpace(title+summary) == "" {
			continue
		}
		out = append(out, equityNewsSentimentRecord{
			Title:   title,
			Summary: summary,
			Source:  firstNonEmptyString(stringValue(obj["source"]), stringValue(obj["provider"]), "kap/news"),
			PublishedAt: parseNewsSentimentTime(firstNonEmptyString(
				stringValue(obj["published_at"]),
				stringValue(obj["publishedAt"]),
				stringValue(obj["publish_date"]),
				stringValue(obj["publishDate"]),
				stringValue(obj["disclosure_date"]),
				stringValue(obj["disclosureDate"]),
				stringValue(obj["date"]),
				stringValue(obj["created_at"]),
				stringValue(obj["received_at"]),
			)),
		})
	}
	return out
}

func equityHeadlineSentimentScore(text string) (float64, string) {
	slug := util.SlugTR(text)
	positiveTerms := []string{
		"sozlesme", "yeniis", "ihalekazan", "siparis", "ihracat", "teslimat", "buyume", "yatirim",
		"temettu", "gerialim", "netkarartis", "karartis", "ciroartis", "hasilatartis", "pozitif",
		"contract", "export", "order", "delivery", "growth", "dividend", "buyback",
	}
	negativeTerms := []string{
		"iptal", "ceza", "dava", "sorusturma", "zarar", "temerrut", "iflas", "paysatis", "azalis",
		"dusus", "borcyapilandirma", "uyari", "negatif", "cancel", "penalty", "lawsuit", "loss", "default",
	}
	score := 0.0
	for _, term := range positiveTerms {
		if strings.Contains(slug, term) {
			score++
		}
	}
	for _, term := range negativeTerms {
		if strings.Contains(slug, term) {
			score--
		}
	}
	score = mathutil.Clamp(score, -3, 3)
	switch {
	case score > 0:
		return score, "positive"
	case score < 0:
		return score, "negative"
	default:
		return 0, "neutral"
	}
}

func parseNewsSentimentTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02.01.2006 15:04:05",
		"02.01.2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func formatNewsSentimentDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func roundProfessionalMetric(value float64) float64 {
	return math.Round(value*100) / 100
}

func buildScenarios(lastClose float64, valuation ValuationAnalysis, peers PeerComparison, market MarketContext) []Scenario {
	base := valuation.FairValue.Base
	if base <= 0 {
		base = lastClose
	}
	bear := valuation.FairValue.Bear
	if bear <= 0 {
		bear = lastClose * 0.85
	}
	bull := valuation.FairValue.Bull
	if bull <= 0 {
		bull = lastClose * 1.2
	}
	return []Scenario{
		{Name: "bear", Probability: 0.25, PriceTarget: bear, ReturnPct: pctReturn(lastClose, bear), Drivers: []string{"peer multiple discount", "technical stop/risk case", "negative earnings/cash-flow flags"}},
		{Name: "base", Probability: 0.50, PriceTarget: base, ReturnPct: pctReturn(lastClose, base), Drivers: []string{"peer median valuation", "current trend and liquidity state", "latest financial run-rate"}},
		{Name: "bull", Probability: 0.25, PriceTarget: bull, ReturnPct: pctReturn(lastClose, bull), Drivers: []string{"peer multiple re-rating", "relative strength improvement", "upside technical continuation"}},
	}
}

func buildValueInvestingScenarios(lastClose float64, report value.Report) []Scenario {
	intrinsic := report.IntrinsicValue
	if !intrinsic.Computed {
		return nil
	}
	return []Scenario{
		{Name: "bear", Probability: 0.25, PriceTarget: intrinsic.Bear, ReturnPct: pctReturn(lastClose, intrinsic.Bear), Drivers: []string{"official_intrinsic_value_bear", report.SectorModel.Model, "required_margin_context"}},
		{Name: "base", Probability: 0.50, PriceTarget: intrinsic.Base, ReturnPct: pctReturn(lastClose, intrinsic.Base), Drivers: []string{"official_intrinsic_value_base", report.SectorModel.Model, "margin_of_safety"}},
		{Name: "bull", Probability: 0.25, PriceTarget: intrinsic.Bull, ReturnPct: pctReturn(lastClose, intrinsic.Bull), Drivers: []string{"official_intrinsic_value_bull", report.SectorModel.Model, "upside_case"}},
	}
}

func buildLiquidity(candles []ohlcv.Candle) LiquidityProfile {
	if len(candles) == 0 {
		return LiquidityProfile{Warnings: []string{"liquidity_unavailable_no_candles"}}
	}
	values := make([]float64, 0, minInt(len(candles), 20))
	volumes := make([]float64, 0, minInt(len(candles), 20))
	amihud := []float64{}
	start := maxInt(0, len(candles)-20)
	zeroValueBars := 0
	for i := start; i < len(candles); i++ {
		value := candles[i].EffectiveClose() * candles[i].EffectiveVolume()
		values = append(values, value)
		volumes = append(volumes, candles[i].EffectiveVolume())
		if value <= 0 {
			zeroValueBars++
		}
		if i > 0 && value > 0 {
			ret := math.Abs(mathutil.SafeDiv(candles[i].EffectiveClose()-candles[i-1].EffectiveClose(), candles[i-1].EffectiveClose()))
			amihud = append(amihud, mathutil.SafeDiv(ret, value/1000000))
		}
	}
	last := candles[len(candles)-1]
	avgValue := mathutil.Mean(values)
	warnings := []string{}
	if len(values) < 20 {
		warnings = append(warnings, fmt.Sprintf("liquidity_short_history_%d_bars", len(values)))
	}
	if zeroValueBars > len(values)/2 {
		warnings = append(warnings, fmt.Sprintf("liquidity_zero_value_traded_high_%d_of_%d", zeroValueBars, len(values)))
	}
	daysToExit1M := liquidityExitDays(mathutil.SafeDiv(1000000, avgValue*0.10))
	return LiquidityProfile{
		LastValueTradedTRY:      last.EffectiveClose() * last.EffectiveVolume(),
		AverageVolume20:         mathutil.Mean(volumes),
		AverageValueTraded20TRY: avgValue,
		MedianValueTraded20TRY:  median(values),
		VolumeVsAverage20:       mathutil.SafeDiv(last.EffectiveVolume(), mathutil.Mean(volumes)),
		AmihudIlliquidity20:     mathutil.Mean(amihud),
		CapacityTRYAt10PctADV:   avgValue * 0.10,
		DaysToExit1MTRY:         daysToExit1M,
		Warnings:                warnings,
	}
}

func liquidityExitDays(days float64) float64 {
	if days <= 0 {
		return 0
	}
	if days < 1 {
		return 1
	}
	return math.Ceil(days)
}

func buildPositionSizing(input TimeframeInput, opts Options, liquidity LiquidityProfile) PositionSizing {
	entry := (input.TradePlan.EntryMin + input.TradePlan.EntryMax) / 2
	if entry <= 0 {
		entry = input.LastClose
	}
	stop := input.TradePlan.StopLoss
	riskPerShare := math.Abs(entry - stop)
	riskBudget := opts.PortfolioValue * opts.RiskPerTradePct / 100
	qty := int(math.Floor(mathutil.SafeDiv(riskBudget, riskPerShare)))
	notional := float64(qty) * entry
	liquidityCap := liquidity.CapacityTRYAt10PctADV
	maxLiquidityQty := int(math.Floor(mathutil.SafeDiv(liquidityCap, entry)))
	warnings := []string{}
	if input.TradePlan.Rejected || input.TradePlan.Direction == "neutral" {
		qty = 0
		notional = 0
		warnings = append(warnings, "trade_plan_rejected")
	}
	if riskPerShare <= 0 {
		qty = 0
		notional = 0
		warnings = append(warnings, "invalid_stop_distance")
	}
	if maxLiquidityQty > 0 && qty > maxLiquidityQty {
		qty = maxLiquidityQty
		notional = float64(qty) * entry
		warnings = append(warnings, "position_limited_by_10pct_adv")
	}
	return PositionSizing{
		PortfolioValue:       opts.PortfolioValue,
		RiskPerTradePct:      opts.RiskPerTradePct,
		RiskBudget:           riskBudget,
		Entry:                entry,
		Stop:                 stop,
		RiskPerShare:         riskPerShare,
		Quantity:             qty,
		Notional:             notional,
		PortfolioPct:         100 * mathutil.SafeDiv(notional, opts.PortfolioValue),
		LiquidityCapNotional: liquidityCap,
		MaxByLiquidityQty:    maxLiquidityQty,
		Warnings:             warnings,
	}
}

func backtestTrendMomentum(candles []ohlcv.Candle, opts Options) BacktestResult {
	cfg := eventbacktest.Config{
		CommissionBps:      opts.CommissionBps,
		SlippageBps:        opts.SlippageBps,
		ExecutionDelayBars: 1,
	}
	results := []eventbacktest.Result{
		eventbacktest.RunTrendMomentum(candles, cfg),
		eventbacktest.RunDowntrendMeanReversion(candles, cfg),
	}
	best := results[0]
	bestScore := backtestSelectionScore(best)
	for _, candidate := range results[1:] {
		score := backtestSelectionScore(candidate)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	out := backtestResultFromEvent(best)
	for _, candidate := range results {
		out.CandidateStrategies = append(out.CandidateStrategies, BacktestCandidate{
			Strategy:          candidate.Strategy,
			Status:            backtestCandidateStatus(candidate),
			Trades:            candidate.Trades,
			OutOfSampleTrades: candidate.OutOfSampleTrades,
			Expectancy:        candidate.Expectancy,
			OutOfSampleReturn: candidate.OutOfSampleReturn,
			MaxDrawdown:       candidate.MaxDrawdown,
			Score:             backtestSelectionScore(candidate),
		})
	}
	return out
}

func backtestResultFromEvent(result eventbacktest.Result) BacktestResult {
	return BacktestResult{
		Strategy:            result.Strategy,
		ExecutionModel:      result.ExecutionModel,
		BacktestSafe:        result.LookaheadViolations == 0,
		LookbackBars:        result.LookbackBars,
		Trades:              result.Trades,
		WinRate:             result.WinRate,
		AverageReturn:       result.AverageReturn,
		MedianReturn:        result.MedianReturn,
		ProfitFactor:        result.ProfitFactor,
		MaxDrawdown:         result.MaxDrawdown,
		CAGR:                result.CAGR,
		Volatility:          result.Volatility,
		Sharpe:              result.Sharpe,
		Sortino:             result.Sortino,
		Exposure:            result.Exposure,
		InSampleTrades:      result.InSampleTrades,
		OutOfSampleTrades:   result.OutOfSampleTrades,
		OutOfSampleReturn:   result.OutOfSampleReturn,
		Expectancy:          result.Expectancy,
		AvgHoldingBars:      result.AvgHoldingBars,
		CurrentInMarket:     result.CurrentInMarket,
		CommissionBps:       result.CommissionBps,
		SlippageBps:         result.SlippageBps,
		LookaheadViolations: result.LookaheadViolations,
	}
}

func backtestSelectionScore(result eventbacktest.Result) float64 {
	score := 0.0
	if result.LookaheadViolations == 0 {
		score += 100
	} else {
		score -= 1000
	}
	score += mathutil.Clamp(float64(result.Trades)/30*100, 0, 100) * 0.25
	score += mathutil.Clamp(float64(result.OutOfSampleTrades)/10*100, 0, 100) * 0.25
	score += mathutil.Clamp((result.Expectancy+0.03)/0.08*100, 0, 100) * 0.25
	score += mathutil.Clamp((result.OutOfSampleReturn+0.03)/0.08*100, 0, 100) * 0.25
	if result.MaxDrawdown < -0.30 {
		score -= 25
	}
	return score
}

func backtestCandidateStatus(result eventbacktest.Result) string {
	switch {
	case result.LookaheadViolations > 0:
		return "fail"
	case result.Trades < 20 || result.OutOfSampleTrades < 5:
		return "limited"
	case result.Expectancy <= 0 || result.OutOfSampleReturn <= -0.005:
		return "fail"
	case result.MaxDrawdown < -0.30:
		return "limited"
	default:
		return "pass"
	}
}

func signalStats(candles []ohlcv.Candle, forwardBars int) SignalStats {
	out := SignalStats{ForwardBars: forwardBars}
	if len(candles) < 80+forwardBars {
		out.InsufficientData = true
		return out
	}
	closeValues := closes(candles)
	sma20 := smaSeries(closeValues, 20)
	sma50 := smaSeries(closeValues, 50)
	ema12 := emaSeries(closeValues, 12)
	ema26 := emaSeries(closeValues, 26)
	regimes := make([]string, len(closeValues))
	for i := range closeValues {
		regimes[i] = regime(closeValues, sma20, sma50, ema12, ema26, i)
	}
	current := regimes[len(regimes)-1]
	out.CurrentRegime = current
	forwardReturns := []float64{}
	for i := 60; i+forwardBars < len(closeValues); i++ {
		if regimes[i] != current {
			continue
		}
		forwardReturns = append(forwardReturns, mathutil.SafeDiv(closeValues[i+forwardBars]-closeValues[i], closeValues[i]))
	}
	wins := 0
	for _, ret := range forwardReturns {
		if ret > 0 {
			wins++
		}
	}
	out.SampleSize = len(forwardReturns)
	out.WinRate = mathutil.SafeDiv(float64(wins), float64(len(forwardReturns)))
	out.AverageForwardReturn = mathutil.Mean(forwardReturns)
	out.MedianForwardReturn = median(forwardReturns)
	out.ProbabilityScore = mathutil.Clamp(50+100*out.AverageForwardReturn+20*(out.WinRate-0.5), 0, 100)
	out.InsufficientData = out.SampleSize < 10
	return out
}

func finalizeCoverage(coverage CoverageReport, input SymbolInput, profile CompanyProfile, valuation ValuationAnalysis, peers PeerComparison, market MarketContext, disclosure DisclosureReview) CoverageReport {
	if profile.Sector != "" {
		coverage.Available = append(coverage.Available, "sector_classification")
	}
	if valuation.MarketCap > 0 {
		coverage.Available = append(coverage.Available, "valuation")
	} else {
		coverage.Missing = append(coverage.Missing, "valuation")
	}
	if peers.PeerCount >= 3 {
		coverage.Available = append(coverage.Available, "peer_comparison")
	} else {
		coverage.Missing = append(coverage.Missing, "peer_comparison_min_3")
	}
	if market.BenchmarkAvailable {
		coverage.Available = append(coverage.Available, "benchmark_relative_strength")
	} else {
		coverage.Missing = append(coverage.Missing, "benchmark_relative_strength")
		if input.BenchmarkError != "" {
			coverage.Warnings = append(coverage.Warnings, input.BenchmarkError)
		}
	}
	if market.SectorBenchmarkSymbol != "" {
		if market.SectorBenchmarkAvailable {
			coverage.Available = append(coverage.Available, "sector_benchmark_relative_strength")
		} else {
			coverage.Missing = append(coverage.Missing, "sector_benchmark_relative_strength")
			if input.SectorBenchmarkError != "" {
				coverage.Warnings = append(coverage.Warnings, input.SectorBenchmarkError)
			}
		}
	}
	if market.LiveSnapshot != nil && market.LiveSnapshot.HasData() {
		coverage.Available = append(coverage.Available, "bist_live_websocket_snapshot")
	} else {
		coverage.Missing = append(coverage.Missing, "bist_live_websocket_snapshot")
	}
	if !ohlcv.IsCryptoAssetType(input.AssetType) && !ohlcv.IsCommodityAssetType(input.AssetType) {
		micro := market.Microstructure
		if micro != nil && micro.Computed {
			coverage.Available = append(coverage.Available, "bist_market_microstructure")
		} else {
			coverage.Missing = append(coverage.Missing, "bist_market_microstructure")
		}
		if micro != nil && micro.OrderBook.Available {
			coverage.Available = append(coverage.Available, "order_book_depth")
		} else {
			coverage.Missing = append(coverage.Missing, "order_book_depth")
		}
		if micro != nil && micro.Depth.Available {
			coverage.Available = append(coverage.Available, "kdm2_depth")
		} else {
			coverage.Missing = append(coverage.Missing, "kdm2_depth")
		}
		if micro != nil && micro.BrokerageDistribution.Available {
			coverage.Available = append(coverage.Available, "brokerage_distribution_akd")
		} else {
			coverage.Missing = append(coverage.Missing, "brokerage_distribution_akd")
		}
		if micro != nil && micro.Custody.Available {
			coverage.Available = append(coverage.Available, "custody_takas")
		} else {
			coverage.Missing = append(coverage.Missing, "custody_takas")
		}
		if micro != nil {
			coverage.Warnings = append(coverage.Warnings, micro.Warnings...)
		}
	}
	if market.GDP.Computed {
		coverage.Available = append(coverage.Available, "tuik_gdp_macro_context")
		if market.GDP.DataQualityWarning != "" {
			coverage.Warnings = appendUniqueString(coverage.Warnings, market.GDP.DataQualityWarning)
		}
	} else if !ohlcv.IsCryptoAssetType(input.AssetType) {
		coverage.Missing = append(coverage.Missing, "tuik_gdp_macro_context")
		if market.GDP.DataQualityWarning != "" {
			coverage.Warnings = appendUniqueString(coverage.Warnings, market.GDP.DataQualityWarning)
		}
	}
	if disclosure.RecentDisclosureStatus != "available" {
		coverage.Missing = append(coverage.Missing, "recent_kap_news_disclosures")
	}
	total := len(coverage.Available) + len(coverage.Missing)
	coverage.Score = 100 * mathutil.SafeDiv(float64(len(coverage.Available)), float64(total))
	return coverage
}

func fairValueFromPeers(lastClose float64, valuation ValuationAnalysis, peers PeerComparison) FairValueRange {
	drivers := []string{}
	targets := []float64{}
	if medianPB := peers.Medians["PB"]; medianPB > 0 && valuation.PaidCapital > 0 && valuation.Equity > 0 {
		targets = append(targets, mathutil.SafeDiv((valuation.Equity*medianPB), valuation.PaidCapital))
		drivers = append(drivers, "peer_median_pb")
	}
	if medianPS := peers.Medians["PS"]; medianPS > 0 && valuation.PaidCapital > 0 && valuation.SalesTTM > 0 {
		targets = append(targets, mathutil.SafeDiv((valuation.SalesTTM*medianPS), valuation.PaidCapital))
		drivers = append(drivers, "peer_median_ps")
	}
	if medianPE := peers.Medians["PE"]; medianPE > 0 && valuation.PaidCapital > 0 && valuation.NetIncomeTTM > 0 {
		targets = append(targets, mathutil.SafeDiv((valuation.NetIncomeTTM*medianPE), valuation.PaidCapital))
		drivers = append(drivers, "peer_median_pe")
	}
	if containsString(valuation.Flags, "bank_sector_requires_regulatory_capital_and_asset_quality_model") {
		drivers = append(drivers, "bank_regulatory_metrics_missing_confidence_cap")
	}
	base := median(targets)
	if base <= 0 {
		base = lastClose
	}
	confidence := peerFairValueConfidence(valuation, len(targets), base != lastClose)
	if containsString(valuation.Flags, "bank_sector_requires_regulatory_capital_and_asset_quality_model") {
		confidence = math.Min(confidence, 0.60)
	}
	return FairValueRange{
		Bear:       base * 0.80,
		Base:       base,
		Bull:       base * 1.20,
		Drivers:    drivers,
		Confidence: confidence,
	}
}

func peerFairValueConfidence(valuation ValuationAnalysis, targetCount int, peerDerived bool) float64 {
	if targetCount <= 0 {
		return 0.15
	}
	expected := expectedPeerFairValueDrivers(valuation)
	if expected <= 0 {
		expected = 3
	}
	score := mathutil.SafeDiv(float64(targetCount), float64(expected))
	if !peerDerived {
		score *= 0.5
	}
	return mathutil.Clamp(score, 0.15, 1)
}

func expectedPeerFairValueDrivers(valuation ValuationAnalysis) int {
	allowed := map[string]bool{}
	for _, key := range valuation.AllowedRatios {
		allowed[strings.ToUpper(strings.TrimSpace(key))] = true
	}
	suppressed := map[string]bool{}
	for _, key := range valuation.SuppressedRatios {
		suppressed[strings.ToUpper(strings.TrimSpace(key))] = true
	}
	expected := 0
	for _, key := range []string{"PB", "PE", "PS"} {
		if suppressed[key] {
			continue
		}
		if len(allowed) > 0 && !allowed[key] {
			continue
		}
		expected++
	}
	return expected
}

func loadFinancials(path string) (financialFile, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return financialFile{}, false
	}
	var file financialFile
	if err := json.Unmarshal(raw, &file); err != nil || len(file.Data) == 0 {
		return financialFile{}, false
	}
	return file, true
}

func loadStatementVersionStore(path string) (domain.FinancialStatementVersionStore, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return domain.FinancialStatementVersionStore{}, false
	}
	var store domain.FinancialStatementVersionStore
	if json.Unmarshal(raw, &store) != nil || len(store.Versions) == 0 {
		return domain.FinancialStatementVersionStore{}, false
	}
	return store, true
}

func latestPeriod(fin financialFile) period {
	best := period{}
	for _, field := range fin.Data {
		for yearText, values := range field.Years {
			year, _ := strconv.Atoi(yearText)
			for idx, value := range values {
				if value == nil {
					continue
				}
				q := quarterFromIndex(idx)
				if year > best.Year || year == best.Year && q > best.Quarter {
					best = period{Year: year, Quarter: q}
				}
			}
		}
	}
	return best
}

func valuationPeriod(fin financialFile, opts Options, asOf time.Time) period {
	if opts.RequireVerifiedPublishDate {
		return latestVerifiedPublishDatePeriod(fin, asOf)
	}
	return latestPeriodAsOf(fin, asOf)
}

func latestPeriodAsOf(fin financialFile, asOf time.Time) period {
	if asOf.IsZero() {
		return latestPeriod(fin)
	}
	best := period{}
	for _, field := range fin.Data {
		for yearText, values := range field.Years {
			year, _ := strconv.Atoi(yearText)
			for idx, value := range values {
				if value == nil {
					continue
				}
				q := quarterFromIndex(idx)
				if q == 0 || domain.FiscalPeriodEnd(year, q).After(asOf) {
					continue
				}
				if year > best.Year || year == best.Year && q > best.Quarter {
					best = period{Year: year, Quarter: q}
				}
			}
		}
	}
	if best.Year == 0 {
		return latestPeriod(fin)
	}
	return best
}

func latestVerifiedPublishDatePeriod(fin financialFile, asOf time.Time) period {
	best := period{}
	for _, periodMeta := range fin.Periods {
		if periodMeta.PublishDate == nil {
			continue
		}
		if !asOf.IsZero() && periodMeta.PublishDate.After(asOf) {
			continue
		}
		if !hasFinancialValue(fin, periodMeta.FiscalYear, periodMeta.FiscalQuarter) {
			continue
		}
		if periodMeta.FiscalYear > best.Year || periodMeta.FiscalYear == best.Year && periodMeta.FiscalQuarter > best.Quarter {
			best = period{Year: periodMeta.FiscalYear, Quarter: periodMeta.FiscalQuarter}
		}
	}
	return best
}

func hasFinancialValue(fin financialFile, year int, quarter int) bool {
	index := indexFromQuarter(quarter)
	if index < 0 {
		return false
	}
	yearText := strconv.Itoa(year)
	for _, field := range fin.Data {
		values := field.Years[yearText]
		if index < len(values) && values[index] != nil {
			return true
		}
	}
	return false
}

func symbolAsOf(input SymbolInput) time.Time {
	if !input.AsOf.IsZero() {
		return input.AsOf.UTC()
	}
	if len(input.DailyCandles) > 0 {
		last := input.DailyCandles[len(input.DailyCandles)-1].Time
		if !last.IsZero() {
			return last.UTC()
		}
	}
	return time.Now().UTC()
}

func financialDataGovernance(equitiesDir string, fin financialFile, latest period, asOf time.Time, ok bool, versionStore domain.FinancialStatementVersionStore, versionStoreOK bool, opts Options) FinancialDataGovernance {
	out := FinancialDataGovernance{
		AsOf:                           asOf,
		DataMode:                       opts.DataMode,
		AvailabilityStatus:             "missing_financial_statements",
		Source:                         fin.Source,
		Currency:                       fin.Currency,
		FinanciallyConsistent:          fin.Quality.FinanciallyConsistent,
		ReconciliationCheckCount:       len(fin.Quality.ReconciliationChecks),
		LineageEvents:                  len(fin.Lineage),
		StatementVersionStoreAvailable: versionStoreOK,
		StatementVersionCount:          len(versionStore.Versions),
		UniverseSourceAvailable:        universeSourceAvailable(equitiesDir),
	}
	out.SurvivorshipBiasRisk = !out.UniverseSourceAvailable
	if out.SurvivorshipBiasRisk {
		out.Warnings = append(out.Warnings, "listed_delisted_universe_source_missing")
	}
	for _, check := range fin.Quality.ReconciliationChecks {
		if !check.Passed {
			out.ReconciliationFailureCount++
		}
	}
	for _, version := range versionStore.Versions {
		if version.IsRestatement {
			out.RestatementCount++
		}
	}
	if !ok {
		out.Warnings = []string{"financial_statements_missing"}
		return out
	}
	periods := fin.Periods
	if len(periods) == 0 {
		periods = financialPeriodsFromFields(fin)
		out.Warnings = append(out.Warnings, "financial_period_metadata_missing")
	}
	latestKey := domain.FinancialPeriodKey(latest.Year, latest.Quarter)
	out.LatestPeriod = latestKey
	if latestPeriodMeta, ok := periods[latestKey]; ok {
		out.LatestPublishDate = latestPeriodMeta.PublishDate
		out.LatestAvailableAt = domain.EffectiveFinancialAvailableAt(latestPeriodMeta)
	}
	keys := make([]string, 0, len(periods))
	for key := range periods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	published := 0
	available := 0
	for _, key := range keys {
		period := periods[key]
		availabilityStatus := domain.FinancialPeriodAvailabilityStatus(period)
		chronologyWarnings := domain.FinancialPeriodChronologyWarnings(period)
		if len(chronologyWarnings) > 0 {
			out.InvalidChronologyPeriods = append(out.InvalidChronologyPeriods, key)
			if !containsString(out.UnsafeBacktestPeriods, key) {
				out.UnsafeBacktestPeriods = append(out.UnsafeBacktestPeriods, key)
			}
			for _, warning := range chronologyWarnings {
				if !containsString(out.Warnings, warning) {
					out.Warnings = append(out.Warnings, warning)
				}
			}
		}
		switch availabilityStatus {
		case domain.FinancialAvailabilityVerifiedPublishDate:
			out.VerifiedPublishDateCount++
			out.ProductionEligiblePeriodCount++
		case domain.FinancialAvailabilityConservativeAvailableAt:
			out.ConservativeAvailableAtCount++
			out.ProductionQuarantinedPeriodCount++
		default:
			out.UnsafeAvailabilityCount++
			out.ProductionQuarantinedPeriodCount++
		}
		if period.PublishDate == nil {
			out.MissingPublishPeriods = append(out.MissingPublishPeriods, key)
		} else {
			published++
		}
		availableAt := domain.EffectiveFinancialAvailableAt(period)
		if availableAt == nil {
			out.MissingAvailableAtPeriods = append(out.MissingAvailableAtPeriods, key)
			out.UnsafeBacktestPeriods = append(out.UnsafeBacktestPeriods, key)
			continue
		}
		available++
		if availabilityStatus == domain.FinancialAvailabilityUnsafeUnverifiedAvailable {
			out.UnsafeBacktestPeriods = append(out.UnsafeBacktestPeriods, key)
			continue
		}
		if !asOf.IsZero() && availableAt.After(asOf) {
			out.UnsafeBacktestPeriods = append(out.UnsafeBacktestPeriods, key)
		}
	}
	if len(keys) > 0 {
		out.PublishDateCoverage = float64(published) / float64(len(keys))
		out.AvailableAtCoverage = float64(available) / float64(len(keys))
	}
	if !versionStoreOK {
		out.Warnings = append(out.Warnings, "statement_version_store_missing")
	}
	if len(out.InvalidChronologyPeriods) > 0 && !containsString(out.Warnings, "financial_period_chronology_invalid") {
		out.Warnings = append(out.Warnings, "financial_period_chronology_invalid")
	}
	out.BacktestSafe = len(keys) > 0 && len(out.UnsafeBacktestPeriods) == 0
	currentDecisionSafe := financialCurrentDecisionSafe(out)
	switch {
	case len(keys) == 0:
		out.AvailabilityStatus = "missing_financial_periods"
		out.Warnings = append(out.Warnings, "financial_periods_missing")
	case opts.RequireVerifiedPublishDate && out.BacktestSafe && out.ProductionEligiblePeriodCount > 0 && out.ProductionQuarantinedPeriodCount > 0:
		out.AvailabilityStatus = "production_verified_subset_with_quarantine"
		out.Warnings = append(out.Warnings, "unverified_financial_periods_quarantined_for_production")
	case opts.RequireVerifiedPublishDate && out.BacktestSafe && out.ProductionEligiblePeriodCount > 0:
		out.AvailabilityStatus = "production_verified_publish_dates_only"
	case opts.RequireVerifiedPublishDate:
		out.AvailabilityStatus = "production_blocked_no_verified_publish_dates"
		out.Warnings = append(out.Warnings, "production_financial_publish_dates_missing")
	case out.BacktestSafe && out.PublishDateCoverage == 1:
		out.AvailabilityStatus = "verified_publish_dates"
	case out.BacktestSafe:
		out.AvailabilityStatus = "lookahead_safe_conservative_available_at"
		out.Warnings = append(out.Warnings, "actual_publish_dates_incomplete")
	case currentDecisionSafe:
		out.AvailabilityStatus = "current_period_time_safe_historical_publish_dates_partial"
		out.Warnings = append(out.Warnings, "historical_financial_publish_dates_partial")
	default:
		out.AvailabilityStatus = "unsafe_missing_or_future_available_at"
		out.Warnings = append(out.Warnings, "financial_data_not_backtest_safe")
	}
	if out.UnsafeAvailabilityCount > len(out.MissingAvailableAtPeriods) {
		out.Warnings = append(out.Warnings, "financial_available_at_source_unverified")
	}
	out.ProductionReady = opts.RequireVerifiedPublishDate && out.BacktestSafe && out.ProductionEligiblePeriodCount > 0 && out.UnsafeAvailabilityCount == 0
	out.Warnings = append(out.Warnings, fin.Quality.Warnings...)
	return out
}

func financialCurrentDecisionSafe(gov FinancialDataGovernance) bool {
	if gov.LatestPeriod == "" || !gov.FinanciallyConsistent {
		return false
	}
	if len(gov.InvalidChronologyPeriods) > 0 || containsString(gov.Warnings, "financial_period_chronology_invalid") {
		return false
	}
	if gov.BacktestSafe {
		return true
	}
	asOf := gov.AsOf
	hasLatestTiming := false
	if gov.LatestAvailableAt != nil {
		hasLatestTiming = true
		if !asOf.IsZero() && gov.LatestAvailableAt.After(asOf) {
			return false
		}
	}
	if gov.LatestPublishDate != nil {
		hasLatestTiming = true
		if !asOf.IsZero() && gov.LatestPublishDate.After(asOf) {
			return false
		}
	}
	return hasLatestTiming
}

func universeSourceAvailable(equitiesDir string) bool {
	if strings.TrimSpace(equitiesDir) == "" {
		return false
	}
	dataDir := filepath.Dir(equitiesDir)
	candidates := []string{
		filepath.Join(dataDir, "seed", "listed_universe.json"),
		filepath.Join(dataDir, "seed", "delisted_universe.json"),
		filepath.Join(dataDir, "universe", "listed_universe.json"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func financialPeriodsFromFields(fin financialFile) map[string]domain.FinancialPeriod {
	periods := map[string]domain.FinancialPeriod{}
	for _, field := range fin.Data {
		for yearText, values := range field.Years {
			year, err := strconv.Atoi(yearText)
			if err != nil || year <= 0 {
				continue
			}
			for index, value := range values {
				if value == nil {
					continue
				}
				quarter := domain.FiscalQuarterFromIndex(index)
				key := domain.FinancialPeriodKey(year, quarter)
				if key == "" {
					continue
				}
				if _, ok := periods[key]; !ok {
					periods[key] = domain.NewFinancialPeriod(year, quarter, fin.Source, fin.FinancialGroup, fin.Currency, fin.FetchedAt)
				}
			}
		}
	}
	return periods
}

func fieldValue(fin financialFile, code string, p period) float64 {
	value, _ := fieldValueOK(fin, code, p)
	return value
}

func fieldValueOK(fin financialFile, code string, p period) (float64, bool) {
	field, ok := fin.Data[code]
	if !ok || p.Year == 0 || p.Quarter == 0 {
		return 0, false
	}
	values := field.Years[strconv.Itoa(p.Year)]
	idx := indexFromQuarter(p.Quarter)
	if idx < 0 || idx >= len(values) || values[idx] == nil {
		return 0, false
	}
	return *values[idx], true
}

func ttm(fin financialFile, code string, p period) (float64, bool) {
	if p.Year == 0 || p.Quarter == 0 {
		return 0, false
	}
	latestYTD, ok := fieldValueOK(fin, code, p)
	if !ok {
		return 0, false
	}
	if p.Quarter == 4 {
		return latestYTD, true
	}
	prevFY, ok := fieldValueOK(fin, code, period{Year: p.Year - 1, Quarter: 4})
	if !ok {
		return 0, false
	}
	prevSameQuarter, ok := fieldValueOK(fin, code, period{Year: p.Year - 1, Quarter: p.Quarter})
	if !ok {
		return 0, false
	}
	return latestYTD + prevFY - prevSameQuarter, true
}

func paidCapitalFromFinancials(fin financialFile, p period) float64 {
	return fieldValue(fin, "2OA", p)
}

type equitySummaryFile struct {
	Name  string `json:"name"`
	OHLCV struct {
		Close json.RawMessage `json:"close"`
	} `json:"ohlcv"`
}

func equitySummary(path string) (string, float64) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0
	}
	var payload equitySummaryFile
	if json.Unmarshal(raw, &payload) != nil {
		return "", 0
	}
	return payload.Name, rawNumberValue(payload.OHLCV.Close)
}

func rawNumberValue(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var number float64
	if json.Unmarshal(raw, &number) == nil {
		return number
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func latestPrice(path string) float64 {
	_, close := equitySummary(path)
	return close
}

func equityName(path string) string {
	name, _ := equitySummary(path)
	return name
}

func loadSectorClassification(equitiesDir string, symbol string) (sectorClassification, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" || equitiesDir == "" {
		return sectorClassification{}, false
	}
	for _, path := range sectorClassificationPaths(equitiesDir) {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var payload sectorClassificationFile
		if json.Unmarshal(raw, &payload) != nil {
			continue
		}
		if payload.Entries == nil {
			continue
		}
		if item, ok := payload.Entries[symbol]; ok {
			return item, true
		}
	}
	return sectorClassification{}, false
}

func sectorClassificationPaths(equitiesDir string) []string {
	dataRoot := filepath.Dir(equitiesDir)
	paths := []string{
		filepath.Join(dataRoot, "seed", "sector_classifications.json"),
		filepath.Join(equitiesDir, "_meta", "sector_classifications.json"),
	}
	if override := strings.TrimSpace(os.Getenv("HISSEBOT_SECTOR_CLASSIFICATIONS_FILE")); override != "" {
		paths = append([]string{override}, paths...)
	}
	return paths
}

func applySectorClassification(profile *CompanyProfile, classification sectorClassification) {
	if classification.Sector != "" {
		profile.Sector = classification.Sector
	}
	profile.Industry = classification.Industry
	profile.PeerGroup = classification.PeerGroup
	profile.PeerSymbols = append([]string{}, classification.PeerSymbols...)
	profile.SectorSource = emptyStringFallback(classification.Source, "sector_classification_store")
	profile.ClassificationConfidence = classification.Confidence
	if profile.ClassificationConfidence <= 0 {
		profile.ClassificationConfidence = 0.75
	}
	profile.ClassificationWarnings = nil
}

func peerSymbolSet(symbols []string) map[string]bool {
	out := map[string]bool{}
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol != "" {
			out[symbol] = true
		}
	}
	return out
}

func emptyStringFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func inferSector(symbol, name string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	text := strings.ToUpper(symbol + " " + name)
	switch {
	case strings.HasSuffix(symbol, "GYO") ||
		strings.Contains(symbol, "GMYO") ||
		strings.Contains(text, "GAYRIMENKUL YATIRIM ORTAK") ||
		strings.Contains(text, "GAYRİMENKUL YATIRIM ORTAK"):
		return "Gayrimenkul Yatırım Ortaklığı"
	case strings.Contains(text, "BANK") || strings.Contains(text, "BANKA"):
		return "Banka"
	case strings.Contains(text, "SIGORTA") || strings.Contains(text, "SİGORTA"):
		return "Sigorta"
	case strings.Contains(text, "SAVUNMA") || strings.Contains(text, "ELEKTRONIK") || strings.Contains(text, "ELEKTRONİK"):
		return "Savunma ve Elektronik"
	case strings.Contains(text, "YAZILIM") || strings.Contains(text, "BILISIM") || strings.Contains(text, "BİLİŞİM") || strings.Contains(text, "TEKNOLOJI") || strings.Contains(text, "TEKNOLOJİ"):
		return "Teknoloji"
	case strings.Contains(text, "ENERJI") || strings.Contains(text, "ENERJİ"):
		return "Enerji"
	default:
		return "BIST Genel"
	}
}

func inferSectorFromKAP(symbol, name string, kap map[string]any) string {
	if sector, ok := officialSectorFromKAP(kap); ok {
		return sector
	}
	sector := inferSector(symbol, strings.Join([]string{name, stringValue(kap["kapMemberTitle"])}, " "))
	if sector != "BIST Genel" {
		return sector
	}
	return ""
}

func officialSectorFromKAP(kap map[string]any) (string, bool) {
	financialType := strings.ToUpper(strings.TrimSpace(stringValue(kap["financialType"])))
	switch financialType {
	case "GYO":
		return "Gayrimenkul Yatırım Ortaklığı", true
	case "BANKA", "BNK":
		return "Banka", true
	case "SGR", "SIGORTA", "SİGORTA":
		return "Sigorta", true
	}
	return "", false
}

func sameSector(symbol, name, sector string) bool {
	candidateSector := inferSector(symbol, name)
	switch sector {
	case "Gayrimenkul Yatırım Ortaklığı":
		return candidateSector == sector
	case "Banka":
		return candidateSector == sector
	case "Sigorta":
		return candidateSector == sector
	case "Savunma ve Elektronik":
		return candidateSector == sector
	case "Teknoloji":
		return candidateSector == sector
	case "Enerji":
		return candidateSector == sector
	default:
		return true
	}
}

func valuationSignal(percentiles map[string]float64) string {
	values := []float64{}
	for _, key := range []string{"PE", "PB", "PS", "EV_Sales"} {
		if value := percentiles[key]; value > 0 {
			values = append(values, value)
		}
	}
	avg := mathutil.Mean(values)
	switch {
	case avg > 0 && avg <= 0.35:
		return "discount"
	case avg >= 0.65:
		return "premium"
	case avg > 0:
		return "neutral"
	default:
		return "insufficient_data"
	}
}

func dailyCandlesFromInput(input SymbolInput) []ohlcv.Candle {
	return input.DailyCandles
}

func rocFromCandles(candles []ohlcv.Candle, period int) float64 {
	if len(candles) <= period || period <= 0 {
		return 0
	}
	old := candles[len(candles)-1-period].EffectiveClose()
	last := candles[len(candles)-1].EffectiveClose()
	return 100 * mathutil.SafeDiv(last-old, old)
}

func regime(closeValues, sma20, sma50, ema12, ema26 []float64, i int) string {
	if i < 0 || i >= len(closeValues) {
		return "unknown"
	}
	trend := "sideways"
	if closeValues[i] > sma50[i] && sma20[i] > sma50[i] {
		trend = "uptrend"
	}
	if closeValues[i] < sma50[i] && sma20[i] < sma50[i] {
		trend = "downtrend"
	}
	momentum := "flat_momentum"
	if ema12[i] > ema26[i] {
		momentum = "positive_momentum"
	}
	if ema12[i] < ema26[i] {
		momentum = "negative_momentum"
	}
	return trend + "_" + momentum
}

func closes(candles []ohlcv.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, candle := range candles {
		out[i] = candle.EffectiveClose()
	}
	return out
}

func returns(values []float64) []float64 {
	if len(values) < 2 {
		return nil
	}
	out := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		out[i-1] = mathutil.SafeDiv(values[i]-values[i-1], values[i-1])
	}
	return out
}

func alignFloatSeries(a, b []float64) ([]float64, []float64) {
	n := minInt(len(a), len(b))
	if n <= 0 {
		return nil, nil
	}
	return a[len(a)-n:], b[len(b)-n:]
}

func lastN(values []float64, n int) []float64 {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func beta(stock, benchmark []float64) float64 {
	stock, benchmark = alignFloatSeries(stock, benchmark)
	if len(stock) == 0 {
		return 0
	}
	meanStock := mathutil.Mean(stock)
	meanBenchmark := mathutil.Mean(benchmark)
	cov := 0.0
	variance := 0.0
	for i := range stock {
		cov += (stock[i] - meanStock) * (benchmark[i] - meanBenchmark)
		variance += (benchmark[i] - meanBenchmark) * (benchmark[i] - meanBenchmark)
	}
	return mathutil.SafeDiv(cov, variance)
}

func correlation(a, b []float64) float64 {
	a, b = alignFloatSeries(a, b)
	if len(a) == 0 {
		return 0
	}
	meanA := mathutil.Mean(a)
	meanB := mathutil.Mean(b)
	num := 0.0
	denA := 0.0
	denB := 0.0
	for i := range a {
		da := a[i] - meanA
		db := b[i] - meanB
		num += da * db
		denA += da * da
		denB += db * db
	}
	return mathutil.SafeDiv(num, math.Sqrt(denA*denB))
}

func smaSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	sum := 0.0
	for i, value := range values {
		sum += value
		if i >= period {
			sum -= values[i-period]
		}
		window := i + 1
		if window > period {
			window = period
		}
		out[i] = mathutil.SafeDiv(sum, float64(window))
	}
	return out
}

func emaSeries(values []float64, period int) []float64 {
	if len(values) == 0 || period <= 0 {
		return nil
	}
	out := make([]float64, len(values))
	alpha := 2.0 / float64(period+1)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = values[i]*alpha + out[i-1]*(1-alpha)
	}
	return out
}

func median(values []float64) float64 {
	filtered := []float64{}
	for _, value := range values {
		if !math.IsNaN(value) && !math.IsInf(value, 0) && value != 0 {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return 0
	}
	sort.Float64s(filtered)
	mid := len(filtered) / 2
	if len(filtered)%2 == 1 {
		return filtered[mid]
	}
	return (filtered[mid-1] + filtered[mid]) / 2
}

func percentileRank(values []float64, target float64) float64 {
	filtered := []float64{}
	for _, value := range values {
		if value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0) {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 || target <= 0 {
		return 0
	}
	lessOrEqual := 0
	for _, value := range filtered {
		if value <= target {
			lessOrEqual++
		}
	}
	return mathutil.SafeDiv(float64(lessOrEqual), float64(len(filtered)))
}

func positiveDiv(numerator, denominator float64) float64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return numerator / denominator
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case json.Number:
		out, _ := typed.Float64()
		return out
	case string:
		cleaned := strings.ReplaceAll(typed, ".", "")
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
		out, _ := strconv.ParseFloat(cleaned, 64)
		return out
	default:
		return 0
	}
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func pctReturn(lastClose, target float64) float64 {
	return 100 * mathutil.SafeDiv(target-lastClose, lastClose)
}

func quarterFromIndex(index int) int {
	switch index {
	case 0:
		return 4
	case 1:
		return 3
	case 2:
		return 2
	case 3:
		return 1
	default:
		return 0
	}
}

func indexFromQuarter(quarter int) int {
	switch quarter {
	case 4:
		return 0
	case 3:
		return 1
	case 2:
		return 2
	case 1:
		return 3
	default:
		return -1
	}
}

func quarterName(quarter int) string {
	if quarter <= 0 {
		return ""
	}
	return "Q" + strconv.Itoa(quarter)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
