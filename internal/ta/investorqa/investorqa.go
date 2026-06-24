package investorqa

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"hissebot/internal/ta/contrarian"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/pkg/mathutil"
)

type Input struct {
	Symbol            string
	CompanyName       string
	Currency          string
	AssetType         string
	OverallScore      float64
	OverallBias       string
	Professional      professional.Report
	Behavioral        contrarian.Report
	PriceVerification PriceVerification
	Timeframes        map[string]Timeframe
}

type Timeframe struct {
	Timeframe         string
	LastClose         float64
	LastVolume        float64
	Score             float64
	TrendBias         string
	Indicators        ohlcv.IndicatorSnapshot
	NearestSupport    *ohlcv.SupportResistanceLevel
	NearestResistance *ohlcv.SupportResistanceLevel
	TradePlan         ohlcv.TradePlan
	Liquidity         professional.LiquidityProfile
	Backtest          professional.BacktestResult
	SignalStats       professional.SignalStats
	TechnicalGate     professional.TechnicalSignalGate
	Range52W          PriceRange
}

type PriceVerification struct {
	Known                 bool     `json:"known"`
	Status                string   `json:"status,omitempty"`
	ReadyForDecision      bool     `json:"ready_for_decision"`
	ReadyForVerifiedClose bool     `json:"ready_for_verified_close"`
	LatestTradingDate     string   `json:"latest_trading_date,omitempty"`
	SelectedClose         float64  `json:"selected_close,omitempty"`
	SelectedTradingDate   string   `json:"selected_trading_date,omitempty"`
	BlockingReasons       []string `json:"blocking_reasons,omitempty"`
	MissingFields         []string `json:"missing_fields,omitempty"`
}

type PriceRange struct {
	Label       string
	Low         float64
	High        float64
	Current     float64
	PositionPct float64
	SampleSize  int
}

type Report struct {
	Computed           bool                      `json:"computed"`
	Symbol             string                    `json:"symbol"`
	Decision           string                    `json:"decision"`
	DecisionLabel      string                    `json:"decision_label"`
	OneLineAnswer      string                    `json:"one_line_answer"`
	InvestorProfile    string                    `json:"investor_profile"`
	Score              float64                   `json:"score"`
	Confidence         float64                   `json:"confidence"`
	TopOpportunity     string                    `json:"top_opportunity"`
	TopRisk            string                    `json:"top_risk"`
	BuyConditions      []string                  `json:"buy_conditions,omitempty"`
	ExitConditions     []string                  `json:"exit_conditions,omitempty"`
	ActionMatrix       []ActionSignal            `json:"action_matrix,omitempty"`
	Questions          []QuestionAnswer          `json:"questions"`
	Quality            QualityReport             `json:"quality"`
	Macro              MacroReport               `json:"macro"`
	Liquidity          LiquidityReport           `json:"liquidity"`
	Governance         GovernanceReport          `json:"governance"`
	Scenario           ScenarioReport            `json:"scenario"`
	ModelRisk          ModelRiskReport           `json:"model_risk"`
	InstitutionalViews InstitutionalPersonaViews `json:"institutional_views"`
	Checks             []Check                   `json:"checks,omitempty"`
	Warnings           []string                  `json:"warnings,omitempty"`
}

type ActionSignal struct {
	Action          string   `json:"action"`
	Label           string   `json:"label"`
	CurrentSignal   bool     `json:"current_signal"`
	Status          string   `json:"status"`
	TimeHorizon     string   `json:"time_horizon"`
	Confidence      float64  `json:"confidence"`
	Score           float64  `json:"score"`
	EntryMin        float64  `json:"entry_min,omitempty"`
	EntryMax        float64  `json:"entry_max,omitempty"`
	StopLoss        float64  `json:"stop_loss,omitempty"`
	Target1         float64  `json:"target1,omitempty"`
	Target2         float64  `json:"target2,omitempty"`
	RiskRewardRatio float64  `json:"risk_reward_ratio,omitempty"`
	Trigger         string   `json:"trigger"`
	Invalidation    string   `json:"invalidation"`
	PositionRule    string   `json:"position_rule"`
	Evidence        []string `json:"evidence,omitempty"`
	Blockers        []string `json:"blockers,omitempty"`
}

type QuestionAnswer struct {
	Question   string   `json:"question"`
	Answer     string   `json:"answer"`
	Status     string   `json:"status"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
}

type QualityReport struct {
	Score          float64  `json:"score"`
	Label          string   `json:"label"`
	Profitability  float64  `json:"profitability"`
	CashConversion float64  `json:"cash_conversion"`
	BalanceSheet   float64  `json:"balance_sheet"`
	CapitalPolicy  float64  `json:"capital_policy"`
	Moat           float64  `json:"moat"`
	RedFlags       []string `json:"red_flags,omitempty"`
	Strengths      []string `json:"strengths,omitempty"`
}

type MacroReport struct {
	Score               float64  `json:"score"`
	Regime              string   `json:"regime"`
	Benchmark           string   `json:"benchmark"`
	RelativeStrength20  float64  `json:"relative_strength_20"`
	RelativeStrength60  float64  `json:"relative_strength_60"`
	Alpha60             float64  `json:"alpha_60"`
	Beta60              float64  `json:"beta_60"`
	Correlation60       float64  `json:"correlation_60"`
	GDPScore            float64  `json:"gdp_score,omitempty"`
	GDPRegime           string   `json:"gdp_regime,omitempty"`
	GDPImpact           string   `json:"gdp_impact,omitempty"`
	GDPInterpretation   string   `json:"gdp_interpretation,omitempty"`
	GDPLatestYear       int      `json:"gdp_latest_year,omitempty"`
	GDPPerCapitaUSD     float64  `json:"gdp_per_capita_usd,omitempty"`
	GDPPerCapitaTRY     float64  `json:"gdp_per_capita_try,omitempty"`
	GDPThousandTRY      float64  `json:"gdp_thousand_try,omitempty"`
	Sensitivity         string   `json:"sensitivity"`
	RequiredMacroChecks []string `json:"required_macro_checks,omitempty"`
}

type LiquidityReport struct {
	Score               float64  `json:"score"`
	Label               string   `json:"label"`
	AverageValueTraded  float64  `json:"average_value_traded"`
	CapacityAt10PctADV  float64  `json:"capacity_at_10pct_adv"`
	DaysToExit1M        float64  `json:"days_to_exit_1m"`
	VolumeVsAverage20   float64  `json:"volume_vs_average_20"`
	AmihudIlliquidity20 float64  `json:"amihud_illiquidity_20"`
	InstitutionalFit    string   `json:"institutional_fit"`
	Warnings            []string `json:"warnings,omitempty"`
}

type GovernanceReport struct {
	Score        float64  `json:"score"`
	Label        string   `json:"label"`
	KAPCard      bool     `json:"kap_card,omitempty"`
	Disclosure   string   `json:"disclosure"`
	RecentCount  int      `json:"recent_count"`
	CommentCount int      `json:"comment_count"`
	RiskFlags    []string `json:"risk_flags,omitempty"`
	DataLineage  string   `json:"data_lineage"`
}

type ScenarioReport struct {
	Score             float64       `json:"score"`
	Bear              ScenarioPoint `json:"bear"`
	Base              ScenarioPoint `json:"base"`
	Bull              ScenarioPoint `json:"bull"`
	ThesisBreak       []string      `json:"thesis_break,omitempty"`
	UpsideTriggers    []string      `json:"upside_triggers,omitempty"`
	DownsideTriggers  []string      `json:"downside_triggers,omitempty"`
	PositioningAnswer string        `json:"positioning_answer"`
}

type ScenarioPoint struct {
	Price     float64  `json:"price"`
	ReturnPct float64  `json:"return_pct"`
	Drivers   []string `json:"drivers,omitempty"`
}

type ModelRiskReport struct {
	Score              float64  `json:"score"`
	Status             string   `json:"status"`
	DataCoverage       float64  `json:"data_coverage"`
	DataQuality        float64  `json:"data_quality"`
	BacktestQuality    float64  `json:"backtest_quality"`
	Explainability     float64  `json:"explainability"`
	PrimaryLimitations []string `json:"primary_limitations,omitempty"`
	AuditTrail         []string `json:"audit_trail,omitempty"`
}

type InstitutionalPersonaViews struct {
	Computed                bool               `json:"computed"`
	OverallStatus           string             `json:"overall_status"`
	OverallQualityStatus    string             `json:"overall_quality_status"`
	EliteCandidate          EliteGate          `json:"elite_candidate"`
	FinancialTransactionUse TransactionUseGate `json:"financial_transaction_use"`
	Summary                 string             `json:"summary"`
	QualitySummary          string             `json:"quality_summary"`
	ValueInvesting          PersonaView        `json:"value_investing_gate"`
	Portfolio               PersonaView        `json:"portfolio_gate"`
	TradingEdge             PersonaView        `json:"trading_edge_gate"`
}

func (v InstitutionalPersonaViews) MarshalJSON() ([]byte, error) {
	if strings.EqualFold(v.ValueInvesting.Status, "not_applicable") {
		type marketOnlyViews struct {
			Computed                bool               `json:"computed"`
			OverallStatus           string             `json:"overall_status"`
			OverallQualityStatus    string             `json:"overall_quality_status"`
			EliteCandidate          EliteGate          `json:"elite_candidate"`
			FinancialTransactionUse TransactionUseGate `json:"financial_transaction_use"`
			Summary                 string             `json:"summary"`
			QualitySummary          string             `json:"quality_summary"`
			Portfolio               PersonaView        `json:"portfolio_gate"`
			TradingEdge             PersonaView        `json:"trading_edge_gate"`
		}
		return json.Marshal(marketOnlyViews{
			Computed:                v.Computed,
			OverallStatus:           v.OverallStatus,
			OverallQualityStatus:    v.OverallQualityStatus,
			EliteCandidate:          v.EliteCandidate,
			FinancialTransactionUse: v.FinancialTransactionUse,
			Summary:                 v.Summary,
			QualitySummary:          v.QualitySummary,
			Portfolio:               v.Portfolio,
			TradingEdge:             v.TradingEdge,
		})
	}
	type viewsAlias InstitutionalPersonaViews
	return json.Marshal(viewsAlias(v))
}

type EliteGate struct {
	Computed       bool     `json:"computed"`
	Status         string   `json:"status"`
	Score          float64  `json:"score"`
	Label          string   `json:"label"`
	Summary        string   `json:"summary"`
	RequiredPasses []string `json:"required_passes,omitempty"`
	FailedPasses   []string `json:"failed_passes,omitempty"`
}

type TransactionUseGate struct {
	Computed       bool     `json:"computed"`
	Status         string   `json:"status"`
	Answer         string   `json:"answer"`
	Summary        string   `json:"summary"`
	RequiredPasses []string `json:"required_passes,omitempty"`
	FailedPasses   []string `json:"failed_passes,omitempty"`
}

type PersonaView struct {
	Name                 string         `json:"name"`
	Lens                 string         `json:"lens"`
	Decision             string         `json:"decision"`
	DecisionLabel        string         `json:"decision_label"`
	Status               string         `json:"status"`
	Score                float64        `json:"score"`
	Confidence           float64        `json:"confidence"`
	ReportQualityStatus  string         `json:"report_quality_status"`
	ReportQualityScore   float64        `json:"report_quality_score"`
	ReportQualityLabel   string         `json:"report_quality_label"`
	ReportQualityReasons []string       `json:"report_quality_reasons,omitempty"`
	OneLineAnswer        string         `json:"one_line_answer"`
	QualityAnswer        string         `json:"quality_answer"`
	FrameworkCommentary  string         `json:"framework_commentary"`
	Takeaway             string         `json:"takeaway"`
	TransactionUseStatus string         `json:"transaction_use_status"`
	TransactionUseAnswer string         `json:"transaction_use_answer"`
	MustHave             []string       `json:"must_have,omitempty"`
	Passes               []string       `json:"passes,omitempty"`
	Blockers             []string       `json:"blockers,omitempty"`
	RequiredActions      []string       `json:"required_actions,omitempty"`
	Evidence             []EvidenceItem `json:"evidence,omitempty"`
}

type EvidenceItem struct {
	Label  string  `json:"label"`
	Value  string  `json:"value"`
	Status string  `json:"status"`
	Score  float64 `json:"score,omitempty"`
}

type Check struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Score   float64 `json:"score"`
	Message string  `json:"message"`
}

func Analyze(input Input) Report {
	symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
	daily := input.primaryTimeframe()
	report := Report{
		Computed:        true,
		Symbol:          symbol,
		InvestorProfile: investorProfile(input),
		Quality:         analyzeQuality(input),
		Macro:           analyzeMacro(input),
		Liquidity:       analyzeLiquidity(input, daily),
		Governance:      analyzeGovernance(input),
		Scenario:        analyzeScenario(input, daily),
		ModelRisk:       analyzeModelRisk(input, daily),
	}
	report.Score = score(input, report)
	report.Confidence = confidence(input, report, daily)
	report.Decision, report.DecisionLabel = decision(input, report, daily)
	report.BuyConditions = buyConditions(input, report, daily)
	report.ExitConditions = exitConditions(input, report, daily)
	report.TopOpportunity = topOpportunity(input, report)
	report.TopRisk = topRisk(input, report, daily)
	report.OneLineAnswer = oneLineAnswer(input, report)
	report.InstitutionalViews = institutionalPersonaViews(input, report, daily)
	report.ActionMatrix = actionMatrix(input, report, daily)
	report.Questions = questions(input, report, daily)
	report.Checks = checks(input, report)
	report.Warnings = warnings(report)
	return report
}

func (input Input) primaryTimeframe() Timeframe {
	if tf, ok := input.Timeframes["1D"]; ok {
		return tf
	}
	keys := make([]string, 0, len(input.Timeframes))
	for key := range input.Timeframes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return Timeframe{}
	}
	return input.Timeframes[keys[0]]
}

func investorProfile(input Input) string {
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		return "Kripto teknik izleme; fiyat, likidite, volatilite, on-chain ve derivatives teyidi gerekir."
	}
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		return "Altın/emtia teknik izleme; fiyat, likidite, volatilite, DXY/reel faiz ve vadeli pozisyon teyidi gerekir."
	}
	return "Uzun vadeli değer yatırımcısı + kurumsal risk komitesi."
}

func analyzeQuality(input Input) QualityReport {
	if isMarketOnlyAssetType(input.AssetType) {
		score := weighted([]weightedScore{
			{input.Professional.Coverage.Score, 0.45},
			{input.Professional.DataQuality, 0.30},
			{boolScore(len(input.Professional.Coverage.Available) > 0), 0.15},
			{boolScore(len(input.Professional.Coverage.Missing) == 0), 0.10},
		})
		out := QualityReport{
			Score:         mathutil.Clamp(score, 0, 100),
			Label:         label(score),
			Profitability: input.Professional.Coverage.Score,
			BalanceSheet:  input.Professional.DataQuality,
		}
		if len(input.Professional.Coverage.Available) > 0 {
			out.Strengths = append(out.Strengths, "fiyat ve teknik veri kapsamı mevcut")
		}
		if len(input.Professional.Coverage.Missing) > 0 {
			out.RedFlags = append(out.RedFlags, strings.ToLower(marketAssetLabel(input.AssetType))+" kaynak kapsamı geliştirilmeli")
		}
		return out
	}
	pro := input.Professional
	v := pro.ValueInvesting
	isBank := isBankInput(input)
	roe := pro.Valuation.Ratios["ROE"]
	netMargin := pro.Valuation.Ratios["Net_Margin"]
	netDebtEq := pro.Valuation.Ratios["NetDebt_Eq"]
	profitability := thresholdScore(roe, []threshold{{0.25, 100}, {0.18, 85}, {0.12, 70}, {0.08, 55}, {0.03, 35}})
	if profitability == 0 && netMargin > 0 {
		profitability = thresholdScore(netMargin, []threshold{{0.20, 85}, {0.12, 70}, {0.06, 55}, {0.02, 35}})
	}
	cashConversion := 0.0
	if pro.Valuation.NetIncomeTTM > 0 {
		cashConversion = mathutil.Clamp(pro.Valuation.FreeCashFlowTTM/pro.Valuation.NetIncomeTTM*70+30, 0, 100)
	} else if pro.Valuation.FreeCashFlowTTM > 0 {
		cashConversion = 45
	}
	balance := 50.0
	switch {
	case netDebtEq <= 0:
		balance = 90
	case netDebtEq <= 0.5:
		balance = 75
	case netDebtEq <= 1:
		balance = 55
	case netDebtEq <= 2:
		balance = 35
	default:
		balance = 15
	}
	capital := v.CapitalAllocation.Score
	if capital == 0 && v.Computed {
		capital = 40
	}
	moat := v.Moat.Score
	if moat == 0 {
		moat = thresholdScore(roe, []threshold{{0.18, 80}, {0.12, 65}, {0.08, 50}, {0.03, 30}})
	}
	score := weighted([]weightedScore{
		{profitability, 0.24},
		{cashConversion, 0.22},
		{balance, 0.20},
		{capital, 0.17},
		{moat, 0.17},
	})
	out := QualityReport{
		Score:          score,
		Label:          label(score),
		Profitability:  profitability,
		CashConversion: cashConversion,
		BalanceSheet:   balance,
		CapitalPolicy:  capital,
		Moat:           moat,
	}
	if roe < 0 {
		out.RedFlags = append(out.RedFlags, "ROE negatif")
	}
	if pro.Valuation.FreeCashFlowTTM < 0 && !isBank {
		out.RedFlags = append(out.RedFlags, "serbest nakit akımı negatif")
	}
	if netDebtEq > 1 && !isBank {
		out.RedFlags = append(out.RedFlags, "net borç/özsermaye yüksek")
	}
	if isBank {
		out.RedFlags = append(out.RedFlags, "SYR/NPL/NIM/LCR metrikleri structured veriyle tamamlanmalı")
		out.CashConversion = 55
		out.BalanceSheet = math.Min(out.BalanceSheet, 65)
		if bankCoreMetricsMissing(pro) {
			out.Score = math.Min(out.Score, 68)
			out.Label = label(out.Score)
			out.RedFlags = append(out.RedFlags, "ana banka metrikleri eksik olduğu için finansal kalite skoru sınırlı")
		}
	}
	if v.CapitalAllocation.Dilution5YPct > 10 {
		if isBank {
			out.RedFlags = append(out.RedFlags, "ödenmiş sermaye artışı bedelli/bedelsiz/split olarak sınıflandırılmalı")
		} else {
			out.RedFlags = append(out.RedFlags, "son 5 yılda pay sulanması yüksek")
		}
	}
	if profitability >= 70 {
		out.Strengths = append(out.Strengths, "kârlılık ortalamanın üzerinde")
	}
	if moat >= 70 {
		out.Strengths = append(out.Strengths, "moat proxy güçlü")
	}
	return out
}

func analyzeMacro(input Input) MacroReport {
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		score := commodityMacroScore(input.Professional)
		out := MacroReport{
			Score:       score,
			Regime:      commodityMacroRegime(score),
			Benchmark:   "DXY / ABD reel faiz / COMEX COT / altin ETF akisi",
			Sensitivity: "DXY ve ABD reel faizine yüksek; COT pozisyonu, ETF/fiziki akış, merkez bankası talebi ve jeopolitik haber akışıyla birlikte okunmalı",
			RequiredMacroChecks: []string{
				"DXY / USD gücü",
				"ABD reel faizleri",
				"Fed faiz beklentisi",
				"COMEX/COT vadeli pozisyon",
				"altın ETF/fiziki talep akışı",
				"merkez bankası alımları ve jeopolitik haber akışı",
			},
		}
		return out
	}
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		score := mathutil.Clamp(weighted([]weightedScore{
			{input.Professional.Coverage.Score, 0.45},
			{input.Professional.DataQuality, 0.30},
			{boolScore(input.Professional.CryptoContext.OnChain.Available), 0.10},
			{boolScore(input.Professional.CryptoContext.Derivatives.Available), 0.10},
			{boolScore(input.Professional.CryptoContext.NewsSentiment.Available), 0.05},
		}), 0, 100)
		return MacroReport{
			Score:       score,
			Regime:      cryptoMacroRegime(score),
			Benchmark:   "kripto piyasa rejimi / BTC dominansi / DXY / funding-open interest",
			Sensitivity: "kripto likiditesi, DXY, risk iştahı, funding/open interest ve haber akışıyla birlikte okunmalı",
			RequiredMacroChecks: []string{
				"DXY / risk iştahı",
				"BTC dominansı ve toplam kripto piyasa değeri",
				"funding / open interest",
				"exchange-flow ve on-chain teyit",
				"haber/sentiment akışı",
			},
		}
	}
	market := input.Professional.Market
	score := 50.0
	score += mathutil.Clamp(market.RelativeStrength20*120, -18, 18)
	score += mathutil.Clamp(market.RelativeStrength60*80, -18, 18)
	score += mathutil.Clamp(market.Alpha60*80, -12, 12)
	if market.Beta60 > 1.4 {
		score -= 8
	}
	if !market.BenchmarkAvailable {
		score -= 15
	}
	gdp := market.GDP
	if gdp.Computed {
		score += mathutil.Clamp((gdp.Score-50)*0.25, -8, 8)
	} else if !isMarketOnlyAssetType(input.AssetType) {
		score -= 5
	}
	score = mathutil.Clamp(score, 0, 100)
	regime := "nötr"
	switch {
	case score >= 70:
		regime = "göreli güçlü / risk iştahı destekleyici"
	case score <= 40:
		regime = "göreli zayıf / makro teyit sınırlı"
	}
	sensitivity := "orta"
	if market.Beta60 >= 1.3 {
		sensitivity = "yüksek beta; piyasa düşüşlerinde daha oynak"
	} else if market.Beta60 > 0 && market.Beta60 <= 0.7 {
		sensitivity = "düşük beta; piyasa dalgasına hassasiyet sınırlı"
	}
	out := MacroReport{
		Score:              score,
		Regime:             regime,
		Benchmark:          market.BenchmarkSymbol,
		RelativeStrength20: market.RelativeStrength20,
		RelativeStrength60: market.RelativeStrength60,
		Alpha60:            market.Alpha60,
		Beta60:             market.Beta60,
		Correlation60:      market.Correlation60,
		GDPScore:           gdp.Score,
		GDPRegime:          gdp.Regime,
		GDPImpact:          gdp.EquityImpact,
		GDPInterpretation:  gdp.Interpretation,
		GDPLatestYear:      gdp.LatestYear,
		GDPPerCapitaUSD:    gdp.PerCapitaUSD,
		GDPPerCapitaTRY:    gdp.PerCapitaTRY,
		GDPThousandTRY:     gdp.GDPThousandTRY,
		Sensitivity:        sensitivity,
	}
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		out.RequiredMacroChecks = []string{"DXY / USD gücü", "ABD reel faizleri", "Fed faiz beklentisi", "COMEX/COT vadeli pozisyon", "altın ETF/fiziki talep akışı", "merkez bankası alımları ve jeopolitik haber akışı"}
	} else if isBankInput(input) {
		out.Score = math.Min(out.Score, 62)
		out.Regime = "banka makro teyidi sınırlı; GSYH tek başına yatırım sinyali değildir"
		out.Sensitivity = "faiz seviyesi, mevduat maliyeti, kredi büyümesi, karşılık döngüsü, regülasyon ve sermaye yeterliliğine yüksek duyarlı"
		out.RequiredMacroChecks = []string{
			"TCMB politika faizi ve mevduat/kredi spreadi",
			"TÜFE/enflasyon beklentisi",
			"CDS ve yabancı risk iştahı",
			"kredi büyümesi ve kredi/mevduat oranı",
			"NPL, karşılık giderleri ve aktif kalitesi",
			"SYR/CET1 sermaye tamponu",
			"bankacılık regülasyonu ve swap/fonlama maliyeti",
		}
		if gdp.Computed {
			out.RequiredMacroChecks = append(out.RequiredMacroChecks, "TÜİK CİP GSYH trendi sadece yardımcı bağlam olarak kullanılır")
		} else if gdp.DataQualityWarning != "" {
			out.RequiredMacroChecks = append(out.RequiredMacroChecks, gdp.DataQualityWarning)
		}
	} else if !ohlcv.IsCryptoAssetType(input.AssetType) {
		out.RequiredMacroChecks = []string{"TCMB faiz/enflasyon rejimi", "USD/TRY hassasiyeti", "CDS ve yabancı risk iştahı", "sektöre özel emtia/regülasyon etkisi"}
		if gdp.Computed {
			out.RequiredMacroChecks = append(out.RequiredMacroChecks, "TÜİK CİP GSYH trendi ve şirket bilanço büyümesi eşleşmesi")
		} else if gdp.DataQualityWarning != "" {
			out.RequiredMacroChecks = append(out.RequiredMacroChecks, gdp.DataQualityWarning)
		}
	}
	return out
}

func commodityMacroScore(pro professional.Report) float64 {
	ctx := pro.CommodityContext
	if ctx.Computed || ctx.Macro.Available || ctx.FuturesPositioning.Available || ctx.GoldETFPhysicalFlow.Available || ctx.CentralBankGeopoliticalNews.Available {
		return mathutil.Clamp(weighted([]weightedScore{
			{sectionScore(ctx.Macro), 0.35},
			{sectionScore(ctx.FuturesPositioning), 0.25},
			{sectionScore(ctx.GoldETFPhysicalFlow), 0.20},
			{sectionScore(ctx.CentralBankGeopoliticalNews), 0.20},
		}), 0, 100)
	}
	return mathutil.Clamp(weighted([]weightedScore{
		{pro.Coverage.Score, 0.50},
		{pro.DataQuality, 0.30},
		{35, 0.20},
	}), 0, 100)
}

func sectionScore(section professional.CommodityContextSection) float64 {
	if !section.Available {
		return 20
	}
	if section.Score > 0 {
		return section.Score
	}
	return 70
}

func commodityMacroRegime(score float64) string {
	switch {
	case score >= 70:
		return "altın makro veri kapsamı güçlü; bu tek başına yön teyidi değildir"
	case score <= 40:
		return "altın makro teyidi zayıf veya eksik; fiyat sinyali tek başına yeterli değil"
	default:
		return "altın makro teyidi karışık; fiyat grafiği ek piyasa verileriyle doğrulanmalı"
	}
}

func cryptoMacroRegime(score float64) string {
	switch {
	case score >= 70:
		return "kripto piyasa teyidi güçlü; teknik sinyal on-chain/derivatives verisiyle destekleniyor"
	case score <= 40:
		return "kripto piyasa teyidi zayıf veya eksik; teknik sinyal tek başına yeterli değil"
	default:
		return "kripto piyasa teyidi karışık; teknik sinyal ek veriyle doğrulanmalı"
	}
}

func analyzeLiquidity(input Input, daily Timeframe) LiquidityReport {
	liq := daily.Liquidity
	score := 0.0
	switch {
	case liq.AverageValueTraded20TRY >= 500_000_000:
		score += 55
	case liq.AverageValueTraded20TRY >= 100_000_000:
		score += 45
	case liq.AverageValueTraded20TRY >= 25_000_000:
		score += 32
	case liq.AverageValueTraded20TRY >= 5_000_000:
		score += 18
	default:
		score += 8
	}
	switch {
	case liq.CapacityTRYAt10PctADV >= 50_000_000:
		score += 25
	case liq.CapacityTRYAt10PctADV >= 10_000_000:
		score += 18
	case liq.CapacityTRYAt10PctADV >= 2_000_000:
		score += 10
	default:
		score += 4
	}
	if liq.DaysToExit1MTRY > 0 && liq.DaysToExit1MTRY <= 1 {
		score += 10
	} else if liq.DaysToExit1MTRY <= 3 {
		score += 5
	}
	if liq.VolumeVsAverage20 >= 1.2 {
		score += 5
	}
	if liq.AmihudIlliquidity20 > 0 && liq.AmihudIlliquidity20 < 0.00000001 {
		score += 5
	}
	score = mathutil.Clamp(score, 0, 100)
	out := LiquidityReport{
		Score:               score,
		Label:               label(score),
		AverageValueTraded:  liq.AverageValueTraded20TRY,
		CapacityAt10PctADV:  liq.CapacityTRYAt10PctADV,
		DaysToExit1M:        liq.DaysToExit1MTRY,
		VolumeVsAverage20:   liq.VolumeVsAverage20,
		AmihudIlliquidity20: liq.AmihudIlliquidity20,
	}
	switch {
	case score >= 70:
		out.InstitutionalFit = "kurumsal pozisyon için likidite kabul edilebilir"
	case score >= 45:
		out.InstitutionalFit = "orta ölçekli pozisyon için sınırlı"
	default:
		out.InstitutionalFit = "büyük para için likidite zayıf"
		out.Warnings = append(out.Warnings, "ADV ve çıkış kapasitesi düşük")
	}
	return out
}

func analyzeGovernance(input Input) GovernanceReport {
	disclosure := input.Professional.Disclosure
	gov := input.Professional.DataGovernance
	if isMarketOnlyAssetType(input.AssetType) {
		score := 35.0
		if len(input.Professional.Coverage.Available) > 0 {
			score += 20
		}
		if input.Professional.Coverage.Score >= 50 {
			score += 15
		}
		if gov.UniverseSourceAvailable {
			score += 10
		}
		if gov.BacktestSafe {
			score += 10
		}
		if len(disclosure.RiskFlags) > 0 {
			score -= math.Min(float64(len(disclosure.RiskFlags))*4, 20)
		}
		score = mathutil.Clamp(score, 0, 100)
		return GovernanceReport{
			Score:        score,
			Label:        label(score),
			Disclosure:   empty(disclosure.RecentDisclosureStatus, strings.ToLower(marketAssetLabel(input.AssetType))+" kaynakları sınırlı"),
			RecentCount:  disclosure.RecentDisclosureCount,
			CommentCount: disclosure.LocalCommentCount,
			RiskFlags:    append([]string{}, disclosure.RiskFlags...),
			DataLineage:  marketDataLineageText(input.AssetType, input.Professional),
		}
	}
	score := 40.0
	if disclosure.KAPCompanyCardAvailable {
		score += 15
	}
	if strings.EqualFold(disclosure.RecentDisclosureStatus, "available") {
		score += 10
	}
	if gov.FinanciallyConsistent {
		score += 15
	}
	if gov.LineageEvents > 0 {
		score += 10
	}
	if gov.RestatementCount == 0 {
		score += 5
	} else {
		score -= math.Min(float64(gov.RestatementCount)*6, 18)
	}
	if len(disclosure.RiskFlags) > 0 {
		score -= math.Min(float64(len(disclosure.RiskFlags))*6, 24)
	}
	score = mathutil.Clamp(score, 0, 100)
	return GovernanceReport{
		Score:        score,
		Label:        label(score),
		KAPCard:      disclosure.KAPCompanyCardAvailable,
		Disclosure:   empty(disclosure.RecentDisclosureStatus, "yok"),
		RecentCount:  disclosure.RecentDisclosureCount,
		CommentCount: disclosure.LocalCommentCount,
		RiskFlags:    append([]string{}, disclosure.RiskFlags...),
		DataLineage:  dataLineageText(gov),
	}
}

func analyzeScenario(input Input, daily Timeframe) ScenarioReport {
	out := ScenarioReport{}
	hasScenario := false
	for _, scenario := range input.Professional.Scenarios {
		if scenario.PriceTarget > 0 {
			hasScenario = true
		}
		point := ScenarioPoint{Price: scenario.PriceTarget, ReturnPct: scenario.ReturnPct, Drivers: append([]string{}, scenario.Drivers...)}
		switch strings.ToLower(scenario.Name) {
		case "bear":
			out.Bear = point
		case "base":
			out.Base = point
		case "bull":
			out.Bull = point
		}
	}
	if !hasScenario {
		out.Score = 0
		out.DownsideTriggers = append(out.DownsideTriggers, "senaryo hedefleri kanıt kapısı geçmediği için üretilmedi")
	} else {
		baseScore := 50 + mathutil.Clamp(out.Base.ReturnPct, -30, 30)
		if input.Professional.ValueInvesting.MarginOfSafety.Computed {
			baseScore += mathutil.Clamp(input.Professional.ValueInvesting.MarginOfSafety.BasePct/2, -15, 20)
		}
		out.Score = mathutil.Clamp(baseScore, 0, 100)
	}
	if !isMarketOnlyAssetType(input.AssetType) && !input.Professional.ValueInvesting.Computed {
		out.Score = math.Min(out.Score, 55)
		out.DownsideTriggers = append(out.DownsideTriggers, "içsel değer doğrulanmadığı için senaryo işlem tezi sayılmaz")
	}
	if daily.NearestSupport != nil {
		out.ThesisBreak = append(out.ThesisBreak, fmt.Sprintf("%.2f altı kapanış destek kırılımı riski yaratır", daily.NearestSupport.Price))
	}
	if daily.TradePlan.StopLoss > 0 {
		out.ThesisBreak = append(out.ThesisBreak, fmt.Sprintf("%.2f stop seviyesi altında plan geçersizleşir", daily.TradePlan.StopLoss))
	}
	if input.Professional.ValueInvesting.Computed && input.Professional.ValueInvesting.MarginOfSafety.BasePct < 0 {
		out.ThesisBreak = append(out.ThesisBreak, "fiyat içsel değerin üstünde kaldıkça değer yatırım tezi zayıf")
	}
	if daily.NearestResistance != nil {
		out.UpsideTriggers = append(out.UpsideTriggers, fmt.Sprintf("%.2f üstü kapanış teknik teyit sağlar", daily.NearestResistance.Price))
	}
	if daily.Indicators.MACDHistogram > 0 {
		out.UpsideTriggers = append(out.UpsideTriggers, "MACD ivmesi pozitif")
	} else {
		out.DownsideTriggers = append(out.DownsideTriggers, "MACD ivmesi negatif")
	}
	if daily.Indicators.ChaikinMoneyFlow20 < 0 {
		out.DownsideTriggers = append(out.DownsideTriggers, "para akışı negatif")
	}
	out.PositioningAnswer = "aktif alım planı yok; teyit gelmeden pozisyon büyütülmez"
	if daily.TradePlan.Direction == "long" && !daily.TradePlan.Rejected && technicalGateAllowsAction(daily) && !priceVerificationBlocksAction(input) {
		out.PositioningAnswer = fmt.Sprintf("aktif alım planı: giriş %.2f-%.2f, stop %.2f, hedef %.2f/%.2f", daily.TradePlan.EntryMin, daily.TradePlan.EntryMax, daily.TradePlan.StopLoss, daily.TradePlan.TakeProfit1, daily.TradePlan.TakeProfit2)
	} else if daily.TradePlan.Direction == "long" && !daily.TradePlan.Rejected {
		reason := "teknik kanıt kapısı geçmediği için aktif işlem sinyali değildir"
		if priceVerificationBlocksAction(input) {
			reason = "resmi/final kapanış doğrulanmadığı için aktif işlem sinyali değildir"
		}
		out.PositioningAnswer = fmt.Sprintf("izleme/paper-trade seviyesi: giriş %.2f-%.2f, stop %.2f, hedef %.2f/%.2f; %s", daily.TradePlan.EntryMin, daily.TradePlan.EntryMax, daily.TradePlan.StopLoss, daily.TradePlan.TakeProfit1, daily.TradePlan.TakeProfit2, reason)
	}
	return out
}

func scenarioTargetsAvailable(report Report) bool {
	return report.Scenario.Bear.Price > 0 || report.Scenario.Base.Price > 0 || report.Scenario.Bull.Price > 0
}

func scenarioQuestionAnswer(input Input, report Report) string {
	if !scenarioTargetsAvailable(report) {
		reason := "Kanıt kapısı geçmediği için bear/base/bull hedef fiyatı üretilmedi; eksik kanıt tamamlanmadan senaryo yatırım tezi sayılmaz."
		if len(input.Professional.EvidencePolicy.BlockingIssues) > 0 {
			reason += " Blokajlar: " + strings.Join(input.Professional.EvidencePolicy.BlockingIssues, ", ") + "."
		}
		return reason
	}
	return fmt.Sprintf("Base hedef %.2f %s, getiri %.1f%%. Bear %.1f%%, bull %.1f%%.", report.Scenario.Base.Price, input.Currency, report.Scenario.Base.ReturnPct, report.Scenario.Bear.ReturnPct, report.Scenario.Bull.ReturnPct)
}

func scenarioQuestionStatus(report Report) string {
	if !scenarioTargetsAvailable(report) {
		return "fail"
	}
	return status(report.Scenario.Score)
}

func scenarioQuestionConfidence(report Report) float64 {
	if !scenarioTargetsAvailable(report) {
		return 0
	}
	return report.Scenario.Score
}

func analyzeModelRisk(input Input, daily Timeframe) ModelRiskReport {
	backtestQuality := 0.0
	if daily.Backtest.BacktestSafe {
		backtestQuality += 35
	}
	if daily.Backtest.Trades >= 30 {
		backtestQuality += 30
	} else if daily.Backtest.Trades >= 10 {
		backtestQuality += 18
	}
	if daily.Backtest.OutOfSampleTrades >= 8 {
		backtestQuality += 20
	}
	if daily.Backtest.Expectancy > 0 && daily.Backtest.OutOfSampleReturn > 0 {
		backtestQuality += 15
	}
	explainability := 55.0
	if len(input.Professional.Coverage.Available) > 0 {
		explainability += 15
	}
	if len(input.Professional.ValueInvesting.Checks) > 0 || isMarketOnlyAssetType(input.AssetType) {
		explainability += 15
	}
	if len(daily.TradePlan.Reasoning) > 0 || daily.TradePlan.Rejected {
		explainability += 15
	}
	if isMarketOnlyAssetType(input.AssetType) {
		technicalGateScore := daily.TechnicalGate.Score
		if technicalGateScore <= 0 {
			technicalGateScore = daily.Score
		}
		signalStatsScore := daily.SignalStats.ProbabilityScore
		if signalStatsScore <= 0 {
			if daily.SignalStats.InsufficientData || daily.SignalStats.SampleSize == 0 {
				signalStatsScore = 35
			} else {
				signalStatsScore = 50
			}
		}
		dataReliability := weighted([]weightedScore{
			{input.Professional.Coverage.Score, 0.55},
			{input.Professional.DataQuality, 0.45},
		})
		score := weighted([]weightedScore{
			{dataReliability, 0.25},
			{backtestQuality, 0.25},
			{mathutil.Clamp(technicalGateScore, 0, 100), 0.25},
			{mathutil.Clamp(signalStatsScore, 0, 100), 0.15},
			{mathutil.Clamp(explainability, 0, 100), 0.10},
		})
		contextReport := "market_context_report"
		if ohlcv.IsCommodityAssetType(input.AssetType) {
			contextReport = "commodity_context_report"
		} else if ohlcv.IsCryptoAssetType(input.AssetType) {
			contextReport = "crypto_context_report"
		}
		out := ModelRiskReport{
			Score:           mathutil.Clamp(score, 0, 100),
			Status:          status(score),
			DataCoverage:    input.Professional.Coverage.Score,
			DataQuality:     input.Professional.DataQuality,
			BacktestQuality: mathutil.Clamp(backtestQuality, 0, 100),
			Explainability:  mathutil.Clamp(explainability, 0, 100),
			AuditTrail: []string{
				"tradingview_ohlcv",
				"technical_indicator_snapshot",
				"support_resistance_engine",
				contextReport,
			},
		}
		if len(input.Professional.Coverage.Missing) > 0 {
			out.PrimaryLimitations = append(out.PrimaryLimitations, input.Professional.Coverage.Missing...)
		}
		if daily.Backtest.Trades < 30 {
			out.PrimaryLimitations = append(out.PrimaryLimitations, "walk_forward_sample_size_limited")
		}
		if daily.Backtest.OutOfSampleTrades < 8 {
			out.PrimaryLimitations = append(out.PrimaryLimitations, "walk_forward_oos_sample_limited")
		}
		if daily.Backtest.OutOfSampleReturn <= 0 {
			out.PrimaryLimitations = append(out.PrimaryLimitations, "walk_forward_oos_return_not_positive")
		}
		if !technicalGateAllowsAction(daily) {
			out.PrimaryLimitations = append(out.PrimaryLimitations, "technical_signal_gate_not_passed")
		}
		if daily.SignalStats.SampleSize > 0 && daily.SignalStats.WinRate < 0.45 {
			out.PrimaryLimitations = append(out.PrimaryLimitations, "similar_regime_success_rate_weak")
		}
		return out
	}
	score := weighted([]weightedScore{
		{input.Professional.Coverage.Score, 0.24},
		{input.Professional.DataQuality, 0.24},
		{backtestQuality, 0.24},
		{mathutil.Clamp(explainability, 0, 100), 0.28},
	})
	out := ModelRiskReport{
		Score:           score,
		Status:          status(score),
		DataCoverage:    input.Professional.Coverage.Score,
		DataQuality:     input.Professional.DataQuality,
		BacktestQuality: mathutil.Clamp(backtestQuality, 0, 100),
		Explainability:  mathutil.Clamp(explainability, 0, 100),
		AuditTrail: []string{
			"tradingview_ohlcv",
			"technical_indicator_snapshot",
			"support_resistance_engine",
			"professional_value_and_governance_report",
		},
	}
	if isMarketOnlyAssetType(input.AssetType) {
		contextReport := "market_context_report"
		if ohlcv.IsCommodityAssetType(input.AssetType) {
			contextReport = "commodity_context_report"
		} else if ohlcv.IsCryptoAssetType(input.AssetType) {
			contextReport = "crypto_context_report"
		}
		out.AuditTrail = []string{
			"tradingview_ohlcv",
			"technical_indicator_snapshot",
			"support_resistance_engine",
			contextReport,
		}
	}
	if len(input.Professional.Coverage.Missing) > 0 {
		out.PrimaryLimitations = append(out.PrimaryLimitations, input.Professional.Coverage.Missing...)
	}
	if daily.Backtest.Trades < 30 {
		out.PrimaryLimitations = append(out.PrimaryLimitations, "walk_forward_sample_size_limited")
	}
	if !currentFinancialDataDecisionSafe(input.Professional.DataGovernance) && !isMarketOnlyAssetType(input.AssetType) {
		out.PrimaryLimitations = append(out.PrimaryLimitations, "financial_data_not_production_ready")
		out.Score = math.Min(out.Score, 79)
		out.Status = status(out.Score)
	}
	if isBankInput(input) && bankCoreMetricsMissing(input.Professional) {
		out.Score = math.Min(out.Score, 64)
		out.Status = status(out.Score)
		out.PrimaryLimitations = append(out.PrimaryLimitations, "bank_regulatory_metrics_missing")
		out.PrimaryLimitations = append(out.PrimaryLimitations, "SYR_CET1_NPL_NIM_LCR_kredi_mevduat_structured_veri_eksik")
	}
	if priceVerificationBlocksAction(input) {
		out.Score = math.Min(out.Score, 64)
		out.Status = status(out.Score)
		out.PrimaryLimitations = append(out.PrimaryLimitations, "price_close_not_verified")
	}
	return out
}

func score(input Input, report Report) float64 {
	if isMarketOnlyAssetType(input.AssetType) {
		return weighted([]weightedScore{
			{input.OverallScore, 0.42},
			{report.Liquidity.Score, 0.18},
			{report.Macro.Score, 0.14},
			{report.Scenario.Score, 0.12},
			{report.ModelRisk.Score, 0.14},
		})
	}
	valueScore := input.Professional.ValueInvesting.ValueScore
	if valueScore == 0 && input.Professional.ValueInvesting.Computed {
		valueScore = 50
	}
	return weighted([]weightedScore{
		{valueScore, 0.26},
		{report.Quality.Score, 0.22},
		{report.Liquidity.Score, 0.14},
		{report.Macro.Score, 0.10},
		{report.Governance.Score, 0.10},
		{report.Scenario.Score, 0.10},
		{report.ModelRisk.Score, 0.08},
	})
}

func confidence(input Input, report Report, daily Timeframe) float64 {
	if isMarketOnlyAssetType(input.AssetType) {
		technicalGateScore := daily.TechnicalGate.Score
		if technicalGateScore <= 0 {
			technicalGateScore = daily.Score
		}
		signalStatsScore := daily.SignalStats.ProbabilityScore
		if signalStatsScore <= 0 {
			if daily.SignalStats.InsufficientData || daily.SignalStats.SampleSize == 0 {
				signalStatsScore = 35
			} else {
				signalStatsScore = 50
			}
		}
		dataScore := math.Min(report.ModelRisk.DataCoverage, report.ModelRisk.DataQuality)
		if dataScore <= 0 {
			dataScore = report.Quality.Score
		}
		score := weighted([]weightedScore{
			{report.ModelRisk.Score, 0.32},
			{mathutil.Clamp(technicalGateScore, 0, 100), 0.23},
			{report.ModelRisk.BacktestQuality, 0.18},
			{mathutil.Clamp(signalStatsScore, 0, 100), 0.12},
			{mathutil.Clamp(dataScore, 0, 100), 0.15},
		})
		if !technicalGateAllowsAction(daily) {
			score = math.Min(score, 64)
		}
		if daily.Backtest.OutOfSampleTrades < 8 || daily.Backtest.OutOfSampleReturn <= 0 {
			score = math.Min(score, 69)
		}
		if daily.SignalStats.SampleSize > 0 && daily.SignalStats.WinRate < 0.45 {
			score = math.Min(score, 69)
		}
		if report.ModelRisk.Score < 65 && len(report.ModelRisk.PrimaryLimitations) > 0 {
			score = math.Min(score, 65)
		}
		return mathutil.Clamp(score, 0, 100)
	}
	score := weighted([]weightedScore{
		{report.ModelRisk.Score, 0.45},
		{report.Governance.Score, 0.20},
		{report.Liquidity.Score, 0.15},
		{report.Quality.Score, 0.20},
	})
	if isBankInput(input) && bankCoreMetricsMissing(input.Professional) {
		score = math.Min(score, 70)
	}
	if priceVerificationBlocksAction(input) {
		score = math.Min(score, 64)
	}
	return score
}

func decision(input Input, report Report, daily Timeframe) (string, string) {
	if isMarketOnlyAssetType(input.AssetType) {
		if input.OverallScore >= 65 && technicalGateAllowsAction(daily) {
			return "ALIM_ADAYI", "Teknik teyit var fakat " + strings.ToLower(marketAssetLabel(input.AssetType)) + " ek veri kapsamı sınırlı"
		}
		return "BEKLE", marketAssetLabel(input.AssetType) + " için teknik yapı ve veri kapsamı kesin alım kararı vermiyor"
	}
	if priceVerificationBlocksAction(input) {
		return "BEKLE", "Güncel kapanış/karar fiyatı kaynaklarla mutabık değil; yeni işlem açma"
	}
	v := input.Professional.ValueInvesting
	if isBankInput(input) && bankCoreMetricsMissing(input.Professional) {
		return "BEKLE", "Banka ana metrikleri (SYR/CET1/NPL/NIM/LCR/kredi-mevduat, mevduat maliyeti ve spread) karar güvenini düşürüyor; yeni pozisyon açma"
	}
	switch {
	case !v.Computed:
		if technicalGateAllowsAction(daily) && !priceVerificationBlocksAction(input) && input.OverallScore >= 55 {
			return "AL", "Teknik sinyal ve fiyat kalitesi olumlu; içsel değer eksik — koşullu alım, güveni sınırlı tut"
		}
		if technicalGateAllowsAction(daily) && input.OverallScore >= 45 {
			return "ALIM_ADAYI", "Teknik yapı pozitif; içsel değer ve güvenlik marjı eksik — teyit ve finansal kanıt bekle"
		}
		return "BEKLE", "İçsel değer ve güvenlik marjı kanıtı eksik; rapor ön izleme niteliğindedir"
	case report.Quality.Score < 40:
		return "RED", "Finansal kalite ve risk profili zayıf"
	case v.MarginOfSafety.BasePct >= v.MarginOfSafety.RequiredPct && report.Quality.Score >= 60 && report.ModelRisk.Score >= 60:
		if technicalGateAllowsAction(daily) {
			return "AL", "İçsel değer, kalite ve teknik giriş birlikte teyit veriyor"
		}
		return "ALIM_ADAYI", "Değer yatırım koşulu var; teknik kanıt kapısı ve giriş teyidi bekleniyor"
	case v.MarginOfSafety.BasePct >= 0 && report.Quality.Score >= 50:
		return "TAKIP", "Fiyat içsel değere yakın; güvenlik marjı sınırlı"
	default:
		return "BEKLE", "Güvenlik marjı ve teyitler yeterli değil"
	}
}

func technicalGateAllowsAction(daily Timeframe) bool {
	if strings.TrimSpace(daily.TechnicalGate.Status) != "" {
		return daily.TechnicalGate.Status == "pass" && daily.TechnicalGate.Actionable
	}
	return activeTradePlan(daily.TradePlan)
}

func buyConditions(input Input, report Report, daily Timeframe) []string {
	out := []string{}
	if priceVerificationBlocksAction(input) {
		out = append(out, "resmi/final kapanışın doğrulanması")
	}
	if daily.NearestResistance != nil {
		out = append(out, fmt.Sprintf("%.2f üstü kapanış", daily.NearestResistance.Price))
	}
	if daily.Indicators.MACDHistogram <= 0 {
		out = append(out, "MACD histogramının pozitife dönmesi")
	}
	if daily.Indicators.ChaikinMoneyFlow20 <= 0 {
		out = append(out, "para akışının pozitife dönmesi")
	}
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if !v.MarginOfSafety.Computed || v.MarginOfSafety.BasePct < v.MarginOfSafety.RequiredPct {
			out = append(out, "içsel değere göre yeterli güvenlik marjı")
		}
		if report.Quality.Score < 60 {
			out = append(out, "finansal kalite skorunun 60 üstüne çıkması")
		}
	}
	if len(out) == 0 {
		out = append(out, "mevcut tez korunmalı ve risk/getiri bozulmamalı")
	}
	return out
}

func exitConditions(input Input, report Report, daily Timeframe) []string {
	out := append([]string{}, report.Scenario.ThesisBreak...)
	if report.ModelRisk.Score < 45 {
		out = append(out, "model risk skoru düşük kalırsa karar geçersiz sayılır")
	}
	if report.Liquidity.Score < 35 {
		out = append(out, "likidite kurumsal pozisyon için yetersiz kalırsa işlem yapılmaz")
	}
	if len(out) == 0 {
		if ohlcv.IsCryptoAssetType(input.AssetType) {
			out = append(out, "ana destek kırılımı, para akışı bozulması veya kripto haber/on-chain risk sinyali")
			return out
		}
		if ohlcv.IsCommodityAssetType(input.AssetType) {
			out = append(out, "ana destek kırılımı, para akışı bozulması veya DXY/reel faiz/vadeli pozisyon risk sinyali")
			return out
		}
		out = append(out, "ana destek kırılımı, para akışı bozulması veya tez kıran KAP/haber")
	}
	return out
}

func actionMatrix(input Input, report Report, daily Timeframe) []ActionSignal {
	return []ActionSignal{
		buyActionSignal(input, report, daily),
		holdActionSignal(input, report, daily),
		sellActionSignal(input, report, daily),
	}
}

func buyActionSignal(input Input, report Report, daily Timeframe) ActionSignal {
	activePlan := activeTradePlan(daily.TradePlan)
	actionablePlan := activePlan && technicalGateAllowsAction(daily) && !priceVerificationBlocksAction(input)
	signal := ActionSignal{
		Action:       "AL",
		Status:       "fail",
		TimeHorizon:  "Kısa/orta vade teknik giriş + temel değer teyidi",
		Confidence:   actionConfidence(report, daily),
		Score:        actionScore(input, report, daily),
		Trigger:      buyTrigger(input, report, daily),
		Invalidation: actionInvalidation(report, daily),
		PositionRule: "Yeni pozisyon ancak tetikleyici, risk/getiri ve veri kalitesi birlikte uygunsa açılır; tek başına gösterge sinyali emir değildir.",
		Evidence:     actionEvidence(input, report, daily),
		Blockers:     buyActionBlockers(input, report, daily),
	}
	switch report.Decision {
	case "AL":
		signal.CurrentSignal = true
		signal.Status = "pass"
		signal.Label = "AL sinyali aktif"
	case "ALIM_ADAYI":
		signal.CurrentSignal = true
		signal.Status = "limited"
		signal.Label = "Şartlı AL adayı; son tetikleyici bekleniyor"
	case "TAKIP":
		signal.Status = "limited"
		signal.Label = "AL yok; izleme listesi için şartlı aday"
	default:
		signal.Label = "AL yok; alım şartları tamamlanmadı"
	}
	if actionablePlan {
		signal.EntryMin = daily.TradePlan.EntryMin
		signal.EntryMax = daily.TradePlan.EntryMax
		signal.StopLoss = daily.TradePlan.StopLoss
		signal.Target1 = daily.TradePlan.TakeProfit1
		signal.Target2 = daily.TradePlan.TakeProfit2
		signal.RiskRewardRatio = daily.TradePlan.RiskRewardRatio
	}
	if !isMarketOnlyAssetType(input.AssetType) && actionablePlan {
		v := input.Professional.ValueInvesting
		entry := actionEntryReference(signal, daily)
		if signal.Target1 == 0 && v.IntrinsicValue.Base > entry && entry > 0 {
			signal.Target1 = v.IntrinsicValue.Base
		}
		if signal.Target2 == 0 && v.IntrinsicValue.Bull > maxFloat(signal.Target1, entry) {
			signal.Target2 = v.IntrinsicValue.Bull
		}
	}
	if signal.StopLoss == 0 && daily.NearestSupport != nil && actionablePlan {
		signal.StopLoss = daily.NearestSupport.Price
	}
	if signal.RiskRewardRatio == 0 && (signal.EntryMin > 0 || signal.EntryMax > 0) {
		signal.RiskRewardRatio = computeRiskReward(actionEntryReference(signal, daily), signal.StopLoss, firstPositive(signal.Target1, signal.Target2))
	}
	if !activePlan && daily.TradePlan.RejectReason != "" {
		signal.Blockers = appendUnique(signal.Blockers, "teknik trade planı reddedildi: "+readableTradeRejectReason(input.AssetType, daily.TradePlan.RejectReason))
	}
	if activePlan && !actionablePlan {
		signal.Blockers = appendUnique(signal.Blockers, "giriş/stop/hedef izleme seviyesidir; teknik kanıt kapısı geçmediği için AL sinyali değildir")
		signal.Evidence = appendUnique(signal.Evidence, tradingPlanEvidence(input.AssetType, daily))
	}
	return sanitizeActionSignal(signal)
}

func holdActionSignal(input Input, report Report, daily Timeframe) ActionSignal {
	current := report.Decision == "BEKLE" || report.Decision == "TAKIP" || report.Decision == "ALIM_ADAYI"
	signal := ActionSignal{
		Action:        "TUT_IZLE",
		CurrentSignal: current,
		Status:        "limited",
		TimeHorizon:   "Pozisyon varsa izleme; yeni pozisyon için tetikleyici bekleme",
		Confidence:    actionConfidence(report, daily),
		Score:         weighted([]weightedScore{{report.Score, 0.35}, {report.Confidence, 0.25}, {report.ModelRisk.Score, 0.20}, {report.Liquidity.Score, 0.20}}),
		Trigger:       holdTrigger(input, report, daily),
		Invalidation:  actionInvalidation(report, daily),
		PositionRule:  "Pozisyon varsa ana destek ve tez kırılımı izlenir; yeni alım için AL satırındaki tetikleyici beklenir.",
		Evidence:      actionEvidence(input, report, daily),
		Blockers:      holdActionBlockers(report),
	}
	switch report.Decision {
	case "BEKLE":
		signal.Label = "TUT/İZLE; yeni alım için teyit eksik"
	case "TAKIP":
		signal.Label = "İzleme listesi; değer veya teknik teyit güçlenmeli"
	case "ALIM_ADAYI":
		signal.Label = "TUT/İZLE; şartlı alım tetikleyiciye bağlı"
	case "AL":
		signal.CurrentSignal = false
		signal.Status = "pass"
		signal.Label = "AL kararı varken tutma ancak pozisyon yönetimi satırıdır"
	case "RED":
		signal.CurrentSignal = false
		signal.Status = "fail"
		signal.Label = "TUT için zayıf; risk azaltma satırı öne çıkar"
	default:
		signal.Label = "İzle; karar için ek veri gerekir"
	}
	if daily.NearestSupport != nil {
		signal.StopLoss = daily.NearestSupport.Price
	}
	if daily.NearestResistance != nil {
		signal.Target1 = daily.NearestResistance.Price
	}
	if !isMarketOnlyAssetType(input.AssetType) && input.Professional.ValueInvesting.IntrinsicValue.Base > 0 {
		signal.Target2 = input.Professional.ValueInvesting.IntrinsicValue.Base
	}
	return sanitizeActionSignal(signal)
}

func sellActionSignal(input Input, report Report, daily Timeframe) ActionSignal {
	current := report.Decision == "RED" || supportBreakdownActive(daily)
	signal := ActionSignal{
		Action:        "SAT_RISK_AZALT",
		CurrentSignal: current,
		Status:        "limited",
		TimeHorizon:   "Risk azaltma / yeni alım yapmama / stop disiplini",
		Confidence:    actionConfidence(report, daily),
		Score:         sellRiskScore(input, report, daily),
		Trigger:       sellTrigger(input, report, daily),
		Invalidation:  sellInvalidation(input, report, daily),
		PositionRule:  "Mevcut pozisyon varsa stop ve tez kırılımı disiplinle izlenir; yeni alım bu satır aktifken yapılmaz.",
		Evidence:      sellEvidence(input, report, daily),
		Blockers:      sellBlockers(report),
	}
	if current {
		signal.Status = "pass"
		signal.Label = "SAT / risk azalt sinyali aktif"
	} else {
		signal.Label = "Doğrudan SAT yok; sadece risk şartları izlenir"
	}
	if daily.NearestSupport != nil {
		signal.StopLoss = daily.NearestSupport.Price
		signal.Target1 = daily.NearestSupport.Price
	}
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if v.IntrinsicValue.Bear > 0 {
			signal.Target2 = v.IntrinsicValue.Bear
		}
	}
	return sanitizeActionSignal(signal)
}

func supportBreakdownActive(daily Timeframe) bool {
	return daily.LastClose > 0 && daily.NearestSupport != nil && daily.NearestSupport.Price > 0 && daily.LastClose < daily.NearestSupport.Price
}

func activeTradePlan(plan ohlcv.TradePlan) bool {
	return plan.Direction == "long" &&
		!plan.Rejected &&
		plan.EntryMin > 0 &&
		plan.EntryMax > 0 &&
		plan.StopLoss > 0 &&
		(plan.TakeProfit1 > 0 || plan.TakeProfit2 > 0)
}

func actionConfidence(report Report, daily Timeframe) float64 {
	tradeConfidence := daily.TradePlan.ConfidenceScore
	if tradeConfidence == 0 {
		tradeConfidence = daily.Score
	}
	return weighted([]weightedScore{
		{report.Confidence, 0.45},
		{report.ModelRisk.Score, 0.25},
		{tradeConfidence, 0.20},
		{report.Liquidity.Score, 0.10},
	})
}

func actionScore(input Input, report Report, daily Timeframe) float64 {
	if isMarketOnlyAssetType(input.AssetType) {
		return weighted([]weightedScore{
			{daily.Score, 0.34},
			{report.Liquidity.Score, 0.20},
			{report.ModelRisk.Score, 0.20},
			{report.Macro.Score, 0.14},
			{report.Scenario.Score, 0.12},
		})
	}
	v := input.Professional.ValueInvesting
	marginScore := 0.0
	if v.MarginOfSafety.Computed && v.MarginOfSafety.RequiredPct > 0 {
		marginScore = mathutil.Clamp(v.MarginOfSafety.BasePct/v.MarginOfSafety.RequiredPct*100, 0, 100)
	}
	return weighted([]weightedScore{
		{marginScore, 0.25},
		{report.Quality.Score, 0.20},
		{daily.Score, 0.20},
		{report.ModelRisk.Score, 0.15},
		{report.Liquidity.Score, 0.10},
		{report.Macro.Score, 0.10},
	})
}

func sellRiskScore(input Input, report Report, daily Timeframe) float64 {
	risk := weighted([]weightedScore{
		{100 - report.Quality.Score, 0.25},
		{100 - report.ModelRisk.Score, 0.25},
		{100 - daily.Score, 0.20},
		{100 - report.Macro.Score, 0.15},
		{100 - report.Liquidity.Score, 0.15},
	})
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if v.MarginOfSafety.Computed && v.MarginOfSafety.BasePct < 0 {
			risk = mathutil.Clamp(risk+15, 0, 100)
		}
	}
	return risk
}

func buyTrigger(input Input, report Report, daily Timeframe) string {
	conditions := append([]string{}, report.BuyConditions...)
	if activeTradePlan(daily.TradePlan) && technicalGateAllowsAction(daily) {
		conditions = append([]string{}, fmt.Sprintf("%.2f-%.2f giriş bölgesi, stop %.2f, hedef %.2f/%.2f", daily.TradePlan.EntryMin, daily.TradePlan.EntryMax, daily.TradePlan.StopLoss, daily.TradePlan.TakeProfit1, daily.TradePlan.TakeProfit2))
	} else if activeTradePlan(daily.TradePlan) {
		conditions = append([]string{
			"teknik kanıt kapısının pass/actionable olması",
			"walk-forward ve benzer rejim istatistiğinin aktif işlem eşiğini geçmesi",
		}, conditions...)
	}
	if len(conditions) == 0 {
		return "Alım tetikleyicisi üretilemedi; veri ve teknik plan güçlenmeli."
	}
	return strings.Join(limitStrings(conditions, 4), "; ")
}

func holdTrigger(input Input, report Report, daily Timeframe) string {
	parts := []string{}
	if daily.NearestSupport != nil {
		parts = append(parts, fmt.Sprintf("%.2f ana destek üstünde kalması", daily.NearestSupport.Price))
	}
	if daily.NearestResistance != nil {
		parts = append(parts, fmt.Sprintf("%.2f direnç üstü kapanış gelirse AL satırı yeniden değerlendirilir", daily.NearestResistance.Price))
	}
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if v.MarginOfSafety.Computed {
			parts = append(parts, fmt.Sprintf("güvenlik marjı %.1f%%; gereken %.1f%%", v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct))
		}
	}
	if len(parts) == 0 {
		return "Pozisyon izlenir; yeni işlem için fiyat, değerleme ve risk tetikleyicisi beklenir."
	}
	return strings.Join(limitStrings(parts, 4), "; ")
}

func sellTrigger(input Input, report Report, daily Timeframe) string {
	parts := append([]string{}, report.ExitConditions...)
	if daily.NearestSupport != nil {
		parts = append([]string{fmt.Sprintf("%.2f desteği altında kapanış veya stop ihlali", daily.NearestSupport.Price)}, parts...)
	}
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if v.MarginOfSafety.Computed && v.MarginOfSafety.BasePct < 0 {
			parts = append(parts, fmt.Sprintf("fiyat baz içsel değerin üstünde; güvenlik marjı %.1f%%", v.MarginOfSafety.BasePct))
		}
	}
	if len(parts) == 0 {
		return "Tez kırılımı veya ana destek ihlali görülürse risk azaltılır."
	}
	return strings.Join(limitStrings(parts, 4), "; ")
}

func actionInvalidation(report Report, daily Timeframe) string {
	parts := append([]string{}, report.ExitConditions...)
	if daily.NearestSupport != nil {
		parts = append([]string{fmt.Sprintf("%.2f ana desteğinin altında kapanış", daily.NearestSupport.Price)}, parts...)
	}
	if len(parts) == 0 {
		return "Ana destek, para akışı veya yatırım tezi bozulursa karar geçersiz olur."
	}
	return strings.Join(limitStrings(parts, 4), "; ")
}

func sellInvalidation(input Input, report Report, daily Timeframe) string {
	parts := []string{}
	if daily.NearestResistance != nil {
		parts = append(parts, fmt.Sprintf("%.2f direnç üstü güçlü kapanış ve hacim teyidi", daily.NearestResistance.Price))
	}
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if v.MarginOfSafety.Computed {
			parts = append(parts, fmt.Sprintf("güvenlik marjının gereken %.1f%% eşiğini geçmesi", v.MarginOfSafety.RequiredPct))
		}
	}
	if !isMarketOnlyAssetType(input.AssetType) && report.Quality.Score < 60 {
		parts = append(parts, "finansal kalite skorunun 60 üstüne çıkması")
	}
	if len(parts) == 0 {
		return "Risk gerekçeleri ortadan kalkarsa SAT/risk azalt satırı pasifleşir."
	}
	return strings.Join(limitStrings(parts, 4), "; ")
}

func actionEvidence(input Input, report Report, daily Timeframe) []string {
	out := []string{
		fmt.Sprintf("genel karar %s: %s", report.Decision, report.DecisionLabel),
		fmt.Sprintf("güven %.0f/100, model risk %.0f/100, likidite %.0f/100", report.Confidence, report.ModelRisk.Score, report.Liquidity.Score),
		fmt.Sprintf("teknik skor %.1f/100, trend %s, RSI %.1f, MACD histogram %.4f", daily.Score, trendBiasTR(daily.TrendBias), daily.Indicators.RSI14, daily.Indicators.MACDHistogram),
	}
	if priceVerificationBlocksAction(input) {
		out = append(out, priceVerificationEvidence(input))
	}
	if daily.TechnicalGate.Status != "" {
		out = append(out, fmt.Sprintf("teknik kanıt kapısı %s: %.0f/100, %s", statusLabelTR(daily.TechnicalGate.Status), daily.TechnicalGate.Score, daily.TechnicalGate.Label))
	}
	if activeTradePlan(daily.TradePlan) && technicalGateAllowsAction(daily) && !priceVerificationBlocksAction(input) {
		out = append(out, fmt.Sprintf("trade plan: giriş %.2f-%.2f, stop %.2f, hedef %.2f/%.2f, R/R %.2f", daily.TradePlan.EntryMin, daily.TradePlan.EntryMax, daily.TradePlan.StopLoss, daily.TradePlan.TakeProfit1, daily.TradePlan.TakeProfit2, daily.TradePlan.RiskRewardRatio))
	} else if activeTradePlan(daily.TradePlan) {
		out = append(out, fmt.Sprintf("paper-trade izleme seviyesi: giriş %.2f-%.2f, stop %.2f, hedef %.2f/%.2f, R/R %.2f; aktif işlem kanıtı değildir", daily.TradePlan.EntryMin, daily.TradePlan.EntryMax, daily.TradePlan.StopLoss, daily.TradePlan.TakeProfit1, daily.TradePlan.TakeProfit2, daily.TradePlan.RiskRewardRatio))
	} else if daily.TradePlan.RejectReason != "" {
		out = append(out, "trade plan reddi: "+readableTradeRejectReason(input.AssetType, daily.TradePlan.RejectReason))
	}
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if v.IntrinsicValue.Computed || v.MarginOfSafety.Computed {
			out = append(out, fmt.Sprintf("içsel değer baz %.2f, fiyat %.2f, güvenlik marjı %.1f%% / gereken %.1f%%", v.IntrinsicValue.Base, v.CurrentPrice, v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct))
		}
		out = append(out, fmt.Sprintf("finansal kalite %.0f/100, nakit dönüşümü %.0f/100, bilanço %.0f/100", report.Quality.Score, report.Quality.CashConversion, report.Quality.BalanceSheet))
	}
	return limitStrings(out, 6)
}

func readableTradeRejectReason(assetType, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	lower := strings.ToLower(reason)
	if strings.Contains(lower, "short selling is not supported") {
		if ohlcv.IsCommodityAssetType(assetType) {
			return "spot altın/emtia modunda aktif alım planı yok; düşüş kanıtı otomatik AL sinyali sayılmaz"
		}
		if ohlcv.IsCryptoAssetType(assetType) {
			return "spot kripto modunda aktif alım planı yok; düşüş kanıtı otomatik AL sinyali sayılmaz"
		}
		return "spot modda aktif alım planı yok"
	}
	if strings.Contains(lower, "neutral trend bias") {
		return "trend kararsız olduğu için yön avantajı yeterli değil"
	}
	if strings.Contains(lower, "risk reward ratio is below") {
		return "risk/ödül oranı işlem eşiğinin altında"
	}
	return reason
}

func sellEvidence(input Input, report Report, daily Timeframe) []string {
	qualityLabel := "kalite"
	if isMarketOnlyAssetType(input.AssetType) {
		qualityLabel = "veri kapsamı"
	}
	out := []string{
		"ana risk: " + report.TopRisk,
		fmt.Sprintf("risk skoru %.0f/100; %s %.0f/100, model risk %.0f/100, teknik %.0f/100", sellRiskScore(input, report, daily), qualityLabel, report.Quality.Score, report.ModelRisk.Score, daily.Score),
	}
	out = append(out, limitStrings(report.ExitConditions, 3)...)
	if !isMarketOnlyAssetType(input.AssetType) {
		v := input.Professional.ValueInvesting
		if v.MarginOfSafety.Computed {
			out = append(out, fmt.Sprintf("güvenlik marjı %.1f%%; gereken %.1f%%", v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct))
		}
	}
	return limitStrings(out, 6)
}

func buyActionBlockers(input Input, report Report, daily Timeframe) []string {
	out := []string{}
	if priceVerificationBlocksAction(input) {
		out = append(out, priceVerificationBlocker(input))
	}
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		if report.ModelRisk.DataCoverage < 60 {
			out = append(out, "kripto veri kapsamı canlı işlem için sınırlı")
		}
	} else if ohlcv.IsCommodityAssetType(input.AssetType) {
		if report.ModelRisk.DataCoverage < 60 {
			out = append(out, "altın/emtia veri kapsamı canlı işlem için sınırlı")
		}
	} else {
		v := input.Professional.ValueInvesting
		if !v.Computed {
			out = append(out, "içsel değer/güvenlik marjı üretilemedi")
		} else if !v.MarginOfSafety.Computed || v.MarginOfSafety.BasePct < v.MarginOfSafety.RequiredPct {
			out = append(out, "gereken güvenlik marjı yok")
		}
		if report.Quality.Score < 60 {
			out = append(out, "finansal kalite 60 eşiğini geçmiyor")
		}
	}
	if report.ModelRisk.Score < 60 {
		out = append(out, "model/veri riski 60 eşiğini geçmiyor")
	}
	if report.Liquidity.Score < 45 {
		out = append(out, "likidite işlem için sınırlı")
	}
	if !activeTradePlan(daily.TradePlan) {
		out = append(out, "aktif giriş/stop/hedef planı yok")
	}
	if daily.TechnicalGate.Status != "" && daily.TechnicalGate.Status != "pass" {
		out = append(out, "teknik kanıt kapısı geçmedi: "+daily.TechnicalGate.Label)
		out = append(out, limitStrings(daily.TechnicalGate.Blockers, 3)...)
	}
	if len(report.InstitutionalViews.EliteCandidate.FailedPasses) > 0 {
		out = append(out, limitStrings(report.InstitutionalViews.EliteCandidate.FailedPasses, 3)...)
	}
	return appendUniqueMany(nil, out)
}

func holdActionBlockers(report Report) []string {
	out := []string{}
	if report.Decision == "RED" {
		out = append(out, "rapor kararı RED; tutma yerine risk azaltma değerlendirilir")
	}
	if report.ModelRisk.Score < 45 {
		out = append(out, "model riski düşük; pasif tutma için bile ek doğrulama gerekir")
	}
	if report.Liquidity.Score < 35 {
		out = append(out, "likidite çıkış riskini artırıyor")
	}
	return out
}

func sellBlockers(report Report) []string {
	if report.Decision == "AL" || report.Decision == "ALIM_ADAYI" {
		return []string{"alım tezi aktif/aday; SAT için destek kırılımı veya tez bozulması gerekir"}
	}
	if report.Decision == "BEKLE" || report.Decision == "TAKIP" {
		return []string{"doğrudan SAT için kritik kırılım yok; izleme ve stop disiplini gerekir"}
	}
	return nil
}

func appendUniqueMany(dst []string, values []string) []string {
	for _, value := range values {
		dst = appendUnique(dst, value)
	}
	return dst
}

func appendUnique(values []string, value string) []string {
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

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}

func actionEntryReference(signal ActionSignal, daily Timeframe) float64 {
	switch {
	case signal.EntryMin > 0 && signal.EntryMax > 0:
		return (signal.EntryMin + signal.EntryMax) / 2
	case signal.EntryMin > 0:
		return signal.EntryMin
	case signal.EntryMax > 0:
		return signal.EntryMax
	case daily.LastClose > 0:
		return daily.LastClose
	default:
		return 0
	}
}

func computeRiskReward(entry, stop, target float64) float64 {
	if entry <= 0 || stop <= 0 || target <= 0 || entry <= stop || target <= entry {
		return 0
	}
	return mathutil.SafeDiv(target-entry, entry-stop)
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sanitizeActionSignal(signal ActionSignal) ActionSignal {
	signal.Confidence = mathutil.Clamp(signal.Confidence, 0, 100)
	signal.Score = mathutil.Clamp(signal.Score, 0, 100)
	signal.RiskRewardRatio = mathutil.Clamp(signal.RiskRewardRatio, 0, 100)
	signal.Evidence = appendUniqueMany(nil, signal.Evidence)
	signal.Blockers = appendUniqueMany(nil, signal.Blockers)
	if signal.Status == "" {
		signal.Status = "limited"
	}
	if signal.Label == "" {
		signal.Label = signal.Action
	}
	return signal
}

func topOpportunity(input Input, report Report) string {
	if !isMarketOnlyAssetType(input.AssetType) && input.Professional.ValueInvesting.Computed && input.Professional.ValueInvesting.MarginOfSafety.BasePct > 0 {
		return fmt.Sprintf("içsel değere göre %.1f%% güvenlik marjı", input.Professional.ValueInvesting.MarginOfSafety.BasePct)
	}
	if len(report.Quality.Strengths) > 0 {
		return report.Quality.Strengths[0]
	}
	if report.Macro.Score >= 65 {
		return "piyasaya göre göreli güç"
	}
	return "fiyat teyidi oluşursa takip edilebilir teknik bölge"
}

func isBankProfessional(pro professional.Report) bool {
	if strings.EqualFold(strings.TrimSpace(pro.SectorFinancials.Profile), "bank") ||
		strings.EqualFold(strings.TrimSpace(pro.Valuation.SectorModel), "bank_equity_model") ||
		strings.EqualFold(strings.TrimSpace(pro.ValueInvesting.SectorModel.Model), "bank_equity_model") {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		pro.Company.Industry,
		pro.SectorFinancials.ProfileLabel,
		pro.ValueInvesting.SectorModel.Label,
	}, " "))
	return strings.Contains(text, "bank") || strings.Contains(text, "banka")
}

func isBankInput(input Input) bool {
	return isBankProfessional(input.Professional)
}

func bankCoreMetricsMissing(pro professional.Report) bool {
	if stringInSlice(pro.Valuation.Flags, "bank_sector_requires_regulatory_capital_and_asset_quality_model") {
		return true
	}
	if !isBankProfessional(pro) {
		return false
	}
	for _, warning := range pro.SectorFinancials.Warnings {
		w := strings.ToLower(warning)
		if strings.Contains(w, "bank") || strings.Contains(w, "syr") || strings.Contains(w, "npl") || strings.Contains(w, "nim") || strings.Contains(w, "lcr") {
			return true
		}
	}
	return true
}

func stringInSlice(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func topRisk(input Input, report Report, daily Timeframe) string {
	if priceVerificationBlocksAction(input) {
		return "resmi/final kapanış doğrulanmadı"
	}
	if !isMarketOnlyAssetType(input.AssetType) && !input.Professional.ValueInvesting.Computed {
		return "içsel değer ve güvenlik marjı hesaplanamıyor"
	}
	if len(report.Quality.RedFlags) > 0 {
		return report.Quality.RedFlags[0]
	}
	if daily.Indicators.ChaikinMoneyFlow20 < 0 {
		return "para akışı negatif"
	}
	if len(report.ModelRisk.PrimaryLimitations) > 0 {
		return report.ModelRisk.PrimaryLimitations[0]
	}
	return "fiyat teyidi gelmeden işlem açma riski"
}

func priceVerificationBlocksAction(input Input) bool {
	if isMarketOnlyAssetType(input.AssetType) {
		return false
	}
	return input.PriceVerification.Known && !input.PriceVerification.ReadyForDecision && !input.PriceVerification.ReadyForVerifiedClose
}

func priceVerificationBlocker(input Input) string {
	pv := input.PriceVerification
	parts := []string{"resmi/final kapanış doğrulanmadı"}
	if pv.Status != "" {
		parts = append(parts, "durum "+pv.Status)
	}
	if pv.LatestTradingDate != "" {
		parts = append(parts, "son tarih "+pv.LatestTradingDate)
	}
	if pv.SelectedClose > 0 {
		selectedDate := pv.SelectedTradingDate
		if selectedDate == "" {
			selectedDate = pv.LatestTradingDate
		}
		if selectedDate != "" {
			parts = append(parts, fmt.Sprintf("seçili kapanış %.2f (%s)", pv.SelectedClose, selectedDate))
		} else {
			parts = append(parts, fmt.Sprintf("seçili kapanış %.2f", pv.SelectedClose))
		}
	}
	if len(pv.BlockingReasons) > 0 {
		parts = append(parts, strings.Join(limitStrings(pv.BlockingReasons, 3), ", "))
	}
	return strings.Join(parts, "; ")
}

func priceVerificationEvidence(input Input) string {
	if !priceVerificationBlocksAction(input) {
		return "kapanış fiyatı verified"
	}
	return priceVerificationBlocker(input) + "; AL/SAT ve ertesi seans başarı ölçümü kapalı"
}

func oneLineAnswer(input Input, report Report) string {
	return fmt.Sprintf("%s: %s. Fırsat: %s. Ana risk: %s. Güven %.0f/100.",
		report.DecisionLabel,
		report.Decision,
		report.TopOpportunity,
		report.TopRisk,
		report.Confidence,
	)
}

func institutionalPersonaViews(input Input, report Report, daily Timeframe) InstitutionalPersonaViews {
	views := InstitutionalPersonaViews{
		Computed:       true,
		ValueInvesting: valueInvestingGateView(input, report),
		Portfolio:      portfolioGateView(input, report, daily),
		TradingEdge:    tradingEdgeGateView(input, report, daily),
	}
	statuses := []string{views.ValueInvesting.Status, views.Portfolio.Status, views.TradingEdge.Status}
	qualityStatuses := []string{views.ValueInvesting.ReportQualityStatus, views.Portfolio.ReportQualityStatus, views.TradingEdge.ReportQualityStatus}
	views.OverallStatus = combinedPersonaStatus(statuses)
	views.OverallQualityStatus = combinedPersonaStatus(qualityStatuses)
	views.EliteCandidate = eliteGate(input, views)
	views.FinancialTransactionUse = financialTransactionUseGate(input, views)
	views.QualitySummary = personaQualitySummary(input, views)
	views.Summary = personaSummary(input, views)
	return views
}

func valueInvestingGateView(input Input, report Report) PersonaView {
	view := PersonaView{
		Name:   "Değer Yatırım Standardı",
		Lens:   "İçsel değer, güvenlik marjı, owner earnings, moat ve sermaye disiplini",
		Status: "fail",
		MustHave: []string{
			"pozitif ve güvenilir içsel değer aralığı",
			"gereken marjın üzerinde güvenlik marjı",
			"5-10 yıllık owner earnings / FCF tutarlılığı",
			"kalıcı ROE/ROIC ve marj istikrarı",
			"pay sulandırmayan sermaye tahsisi",
		},
	}
	if isMarketOnlyAssetType(input.AssetType) {
		view.Lens = "Fiyat grafiği, likidite, makro/piyasa teyidi ve risk disiplini"
		view.MustHave = []string{
			marketAssetLabel(input.AssetType) + " için şirket finansalı temelli değer yatırım kapısı uygulanmaz",
			"Karar fiyat, likidite, teknik yapı, makro/piyasa teyidi ve risk yönetimiyle verilir",
		}
		view.Status = "not_applicable"
		view.Decision = "UYGULANMAZ"
		view.DecisionLabel = "Geleneksel finansal tablo temelli içsel değer çerçevesi " + strings.ToLower(marketAssetLabel(input.AssetType)) + " varlığına uygulanmaz"
		view.Score = 0
		view.Confidence = report.ModelRisk.Score
		view.Blockers = []string{"bu varlık şirket değildir; finansal tablo temelli içsel değer kapısı kullanılmaz"}
		view = applyValueInvestingReportQuality(input, report, view)
		view.OneLineAnswer = view.DecisionLabel
		view.QualityAnswer = personaQualityAnswer(view)
		view.FrameworkCommentary, view.Takeaway = valueInvestingGateCommentary(view)
		view.TransactionUseStatus, view.TransactionUseAnswer = personaTransactionUse(view)
		return view
	}
	v := input.Professional.ValueInvesting
	isBank := isBankInput(input)
	bankMissingCore := isBank && bankCoreMetricsMissing(input.Professional)
	if isBank {
		view.Lens = "İçsel değer, güvenlik marjı, sürdürülebilir ROE, aktif kalitesi ve sermaye/fonlama metrikleri"
		view.MustHave = []string{
			"pozitif ve güvenilir içsel değer aralığı",
			"gereken marjın üzerinde güvenlik marjı",
			"SYR, çekirdek sermaye, NPL, NIM ve LCR kanıtı",
			"sürdürülebilir ROE ve aktif kalitesi",
			"sermaye hareketlerinin bedelli/bedelsiz/split olarak sınıflanması",
		}
	}
	marginScore := 0.0
	if v.MarginOfSafety.Computed && v.MarginOfSafety.RequiredPct > 0 {
		marginScore = mathutil.Clamp(v.MarginOfSafety.BasePct/v.MarginOfSafety.RequiredPct*100, 0, 120)
	}
	ownerEarningsScore := v.OwnerEarnings.Score
	normalizedFCFScore := v.NormalizedFCF.Score
	if isBank {
		ownerEarningsScore = 60
		normalizedFCFScore = 60
	}
	view.Score = weighted([]weightedScore{
		{boolScore(v.IntrinsicValue.Computed && v.MarginOfSafety.Computed), 0.18},
		{marginScore, 0.20},
		{ownerEarningsScore, 0.16},
		{normalizedFCFScore, 0.14},
		{report.Quality.Score, 0.14},
		{v.Moat.Score, 0.10},
		{v.CapitalAllocation.Score, 0.08},
	})
	view.Confidence = weighted([]weightedScore{
		{v.Confidence, 0.45},
		{v.DataQuality, 0.25},
		{report.ModelRisk.Score, 0.20},
		{report.Governance.Score, 0.10},
	})
	view.Evidence = []EvidenceItem{
		{"İçsel değer", intrinsicEvidence(input), statusBool(v.IntrinsicValue.Computed), v.IntrinsicValue.Confidence},
		{"Güvenlik marjı", marginEvidence(input), marginStatus(input), marginScore},
		{"Owner earnings", fmt.Sprintf("TTM %s %s | 5Y normalize %s %s | skor %.0f/100", money(v.OwnerEarnings.TTM), input.Currency, money(v.OwnerEarnings.Normalized5Y), input.Currency, v.OwnerEarnings.Score), status(v.OwnerEarnings.Score), v.OwnerEarnings.Score},
		{"Normalize FCF", fmt.Sprintf("TTM %s %s | 5Y medyan %s %s | skor %.0f/100", money(v.NormalizedFCF.TTM), input.Currency, money(v.NormalizedFCF.Median5Y), input.Currency, v.NormalizedFCF.Score), status(v.NormalizedFCF.Score), v.NormalizedFCF.Score},
		{"Moat", fmt.Sprintf("ROE5Y %.1f%% | ROIC5Y %.1f%% | skor %.0f/100", v.Moat.AverageROE5Y*100, v.Moat.AverageROIC5Y*100, v.Moat.Score), status(v.Moat.Score), v.Moat.Score},
		{"Sermaye tahsisi", fmt.Sprintf("5Y pay sulanması %.1f%% | skor %.0f/100", v.CapitalAllocation.Dilution5YPct, v.CapitalAllocation.Score), status(v.CapitalAllocation.Score), v.CapitalAllocation.Score},
	}
	if isBank {
		view.Evidence[2] = EvidenceItem{"Owner earnings", "Bankada klasik owner earnings ana girdi değildir; kâr kalitesi net faiz, ücret-komisyon, karşılık ve aktif kalitesiyle ölçülür.", "not_applicable", ownerEarningsScore}
		view.Evidence[3] = EvidenceItem{"Normalize FCF", "Bankada klasik FCF ana girdi değildir; NIM, NPL, LCR, kredi/mevduat ve fonlama maliyeti bağlanmalıdır.", "not_applicable", normalizedFCFScore}
		view.Evidence[5] = EvidenceItem{"Sermaye hareketleri", fmt.Sprintf("5Y ödenmiş sermaye değişimi %.1f%% | ekonomik sulanma sayılmadı; bedelli/bedelsiz/split sınıflaması gerekir | skor %.0f/100", v.CapitalAllocation.Dilution5YPct, v.CapitalAllocation.Score), status(v.CapitalAllocation.Score), v.CapitalAllocation.Score}
	}
	if v.IntrinsicValue.Computed && v.MarginOfSafety.Computed {
		view.Passes = append(view.Passes, "içsel değer ve güvenlik marjı hesaplanmış")
	} else {
		view.Blockers = append(view.Blockers, "içsel değer veya güvenlik marjı güvenilir hesaplanamıyor")
	}
	if v.MarginOfSafety.Computed && v.MarginOfSafety.BasePct >= v.MarginOfSafety.RequiredPct {
		view.Passes = append(view.Passes, fmt.Sprintf("güvenlik marjı gereken %.1f%% eşiğini geçiyor", v.MarginOfSafety.RequiredPct))
	} else {
		view.Blockers = append(view.Blockers, "gereken güvenlik marjı yok")
		view.RequiredActions = append(view.RequiredActions, valueInvestingGateRequiredMarginAction(input))
	}
	if isBank {
		view.Passes = append(view.Passes, "owner earnings bankada ana değerleme girdisi değildir")
	} else if v.OwnerEarnings.Score >= 55 {
		view.Passes = append(view.Passes, "owner earnings kabul edilebilir")
	} else {
		view.Blockers = append(view.Blockers, "owner earnings kalitesi zayıf veya dalgalı")
	}
	if isBank {
		view.Passes = append(view.Passes, "normalize FCF bankada ana değerleme girdisi değildir")
	} else if v.NormalizedFCF.Score >= 55 {
		view.Passes = append(view.Passes, "normalize FCF kabul edilebilir")
	} else {
		view.Blockers = append(view.Blockers, "normalize FCF kalitesi zayıf")
	}
	if report.Quality.Score >= 60 && v.Moat.Score >= 60 {
		view.Passes = append(view.Passes, "finansal kalite ve moat eşiği geçiyor")
	} else {
		view.Blockers = append(view.Blockers, "finansal kalite/moat eşiği tam geçmiyor")
	}
	if v.CapitalAllocation.Dilution5YPct > 10 {
		if isBank {
			view.RequiredActions = append(view.RequiredActions, fmt.Sprintf("ödenmiş sermaye değişimi %.1f%%; bedelli/bedelsiz/split/nominal düzeltme ayrımı komite notunda sınıflanmalı", v.CapitalAllocation.Dilution5YPct))
		} else {
			view.Blockers = append(view.Blockers, "son 5 yılda pay sulanması yüksek")
			view.RequiredActions = append(view.RequiredActions, fmt.Sprintf("pay sulanması %.1f%%; değer yatırım tezi için yeni sermaye artışı/sulanma etkisi komite notunda açıklanmalı", v.CapitalAllocation.Dilution5YPct))
		}
	}
	if !isBank && lowCashConversionRisk(input) {
		view.Blockers = append(view.Blockers, "net kâr nakde dönüşüm kalitesi zayıf")
		view.RequiredActions = append(view.RequiredActions, "Net kâr, faaliyet kârı, işletme sermayesi, capex ve serbest nakit akımı köprüsünü dönem bazında mutabık hale getir")
		view.Score = math.Min(view.Score, 54)
		view.Confidence = math.Min(view.Confidence, 64)
	}
	if ok, reason := valuationModelAuditIssue(input); ok {
		view.Blockers = append(view.Blockers, "değerleme modeli derin denetim gerektiriyor")
		view.RequiredActions = append(view.RequiredActions, reason)
		view.Score = math.Min(view.Score, 54)
		view.Confidence = math.Min(view.Confidence, 60)
	}
	if defenseCompanyInput(input) && input.Professional.Peers.PeerCount < 5 {
		view.Blockers = append(view.Blockers, "savunma şirketi için peer evreni dar")
		view.RequiredActions = append(view.RequiredActions, "ASELS benzeri savunma/elektronik şirketleri için yerli savunma, global defense, teknoloji ve kamu sözleşmeli peer evrenlerini ayrı kur")
		view.Score = math.Min(view.Score, 59)
	}
	if bankMissingCore {
		view.Blockers = append(view.Blockers, "ana banka metrikleri eksik")
		view.RequiredActions = append(view.RequiredActions, "SYR, çekirdek sermaye, NPL, NIM, LCR ve karşılık gideri verilerini bağla")
		view.Score = math.Min(view.Score, 64)
		view.Confidence = math.Min(view.Confidence, 74)
	}
	view.Status = strictPersonaStatus(view.Score, view.Blockers, []string{
		"içsel değer veya güvenlik marjı güvenilir hesaplanamıyor",
		"gereken güvenlik marjı yok",
		"owner earnings kalitesi zayıf veya dalgalı",
		"normalize FCF kalitesi zayıf",
		"net kâr nakde dönüşüm kalitesi zayıf",
		"değerleme modeli derin denetim gerektiriyor",
		"ana banka metrikleri eksik",
	})
	switch view.Status {
	case "pass":
		view.Decision = "KOMITEYE_UYGUN"
		view.DecisionLabel = "Değer yatırım standardında yatırım komitesine girebilir"
	case "limited":
		view.Decision = "IZLE"
		view.DecisionLabel = "Değer yatırım standardında izleme listesi; karar için ek kanıt gerekir"
	default:
		view.Decision = "KULLANMAZ"
		view.DecisionLabel = "Değer yatırım standardında yatırım kararı için kullanmaz"
	}
	view = applyValueInvestingReportQuality(input, report, view)
	view.OneLineAnswer = fmt.Sprintf("%s: skor %.0f/100. Ana engel: %s.", view.DecisionLabel, view.Score, firstOr(view.Blockers, "kritik engel yok"))
	view.QualityAnswer = personaQualityAnswer(view)
	view.FrameworkCommentary, view.Takeaway = valueInvestingGateCommentary(view)
	view.TransactionUseStatus, view.TransactionUseAnswer = personaTransactionUse(view)
	return view
}

func portfolioGateView(input Input, report Report, daily Timeframe) PersonaView {
	isMarketOnly := isMarketOnlyAssetType(input.AssetType)
	view := PersonaView{
		Name:   "Kurumsal Portföy Standardı",
		Lens:   "Kurumsal portföy uygunluğu, likidite, benchmark, risk, veri yönetişimi ve ölçeklenebilirlik",
		Status: "fail",
		MustHave: []string{
			"yüksek ADV ve çıkış kapasitesi",
			"benchmark/faktör riski ve beta bilgisi",
			"sektör, peer ve veri soy ağacı",
			"kurumsal veri yönetişimi",
			"portföy/stres riski için yeterli model güveni",
		},
	}
	if isMarketOnly {
		view.MustHave = []string{
			"yüksek spot likidite ve çıkış kapasitesi",
			marketMacroMustHave(input.AssetType),
			strings.ToLower(marketAssetLabel(input.AssetType)) + " veri kapsamı: ek piyasa verileri ve haber akışı",
			"kurumsal veri yönetişimi",
			"portföy/stres riski için yeterli model güveni",
		}
	}
	coverageScore := input.Professional.Coverage.Score
	peerScore := 0.0
	switch {
	case isMarketOnly && input.Professional.Company.Sector != "" && input.Professional.Company.ClassificationConfidence >= 0.60:
		peerScore = 70
	case isMarketOnly && input.Professional.Company.Sector != "":
		peerScore = 55
	case isMarketOnly:
		peerScore = 35
	case input.Professional.Peers.PeerCount >= 10 && input.Professional.Company.ClassificationConfidence >= 0.85:
		peerScore = 90
	case input.Professional.Peers.PeerCount >= 3 && input.Professional.Company.ClassificationConfidence >= 0.75:
		peerScore = 65
	case input.Professional.Peers.PeerCount > 0:
		peerScore = 45
	default:
		peerScore = 20
	}
	view.Score = weighted([]weightedScore{
		{report.Liquidity.Score, 0.26},
		{report.Governance.Score, 0.18},
		{report.ModelRisk.Score, 0.18},
		{report.Macro.Score, 0.14},
		{coverageScore, 0.12},
		{peerScore, 0.12},
	})
	view.Confidence = weighted([]weightedScore{
		{report.ModelRisk.Score, 0.40},
		{report.Governance.Score, 0.24},
		{coverageScore, 0.20},
		{report.Liquidity.Score, 0.16},
	})
	classificationLabel := "Sektör/peer"
	classificationDetail := fmt.Sprintf("%s / %s | peer %d | sınıflama %.2f", empty(input.Professional.Company.Sector, "sektör yok"), empty(input.Professional.Company.Industry, "endüstri yok"), input.Professional.Peers.PeerCount, input.Professional.Company.ClassificationConfidence)
	if isMarketOnly {
		classificationLabel = marketAssetLabel(input.AssetType) + " varlık sınıfı"
		classificationDetail = fmt.Sprintf("%s / %s | veri kapsamı %.0f/100", empty(input.Professional.Company.Sector, marketAssetLabel(input.AssetType)), empty(input.Professional.Company.Industry, "Piyasa varlığı"), report.ModelRisk.DataCoverage)
	}
	macroLabel := "Benchmark riski"
	macroDetail := fmt.Sprintf("%s | beta %.2f | korelasyon %.2f | RS20 %.1f puan", empty(input.Professional.Market.BenchmarkSymbol, "benchmark yok"), report.Macro.Beta60, report.Macro.Correlation60, report.Macro.RelativeStrength20)
	if isMarketOnly {
		macroLabel = marketMacroEvidenceLabel(input.AssetType)
		macroDetail = marketMacroEvidenceDetail(input, report)
	}
	view.Evidence = []EvidenceItem{
		{"Likidite", liquidityAnswer(report.Liquidity, input.Currency), status(report.Liquidity.Score), report.Liquidity.Score},
		{"Kapasite", fmt.Sprintf("10%% ADV kapasite %s %s | 1M çıkış %.2f gün", money(report.Liquidity.CapacityAt10PctADV), input.Currency, report.Liquidity.DaysToExit1M), status(report.Liquidity.Score), report.Liquidity.Score},
		{macroLabel, macroDetail, status(report.Macro.Score), report.Macro.Score},
		{classificationLabel, classificationDetail, status(peerScore), peerScore},
		{"Veri yönetişimi", report.Governance.DataLineage, status(report.Governance.Score), report.Governance.Score},
		{"Model riski", fmt.Sprintf("veri kapsamı %.0f/100 | backtest %.0f/100 | açıklanabilirlik %.0f/100", report.ModelRisk.DataCoverage, report.ModelRisk.BacktestQuality, report.ModelRisk.Explainability), status(report.ModelRisk.Score), report.ModelRisk.Score},
	}
	if report.Liquidity.Score >= 70 {
		view.Passes = append(view.Passes, "likidite kurumsal ön eleme için yeterli")
	} else {
		view.Blockers = append(view.Blockers, "kurumsal pozisyon ölçeği için likidite sınırlı")
	}
	if report.Governance.Score >= 65 {
		view.Passes = append(view.Passes, "veri yönetişimi kabul edilebilir")
	} else {
		if isMarketOnlyAssetType(input.AssetType) {
			view.Blockers = append(view.Blockers, strings.ToLower(marketAssetLabel(input.AssetType))+" veri kaynakları ve kaynak güveni sınırlı")
		} else {
			view.Blockers = append(view.Blockers, "veri yönetişimi veya KAP/kaynak güveni sınırlı")
		}
	}
	if report.ModelRisk.Score >= 65 {
		view.Passes = append(view.Passes, "model risk skoru portföy ön elemesine yeterli")
	} else {
		view.Blockers = append(view.Blockers, "model risk skoru kurumsal portföy eşiğinin altında")
	}
	if coverageScore >= 85 || isMarketOnlyAssetType(input.AssetType) && coverageScore >= 50 {
		view.Passes = append(view.Passes, "veri kapsamı raporlanmış")
	} else {
		view.Blockers = append(view.Blockers, "veri kapsamı kurumsal portföy raporu için eksik")
	}
	if report.Macro.Score < 45 {
		if isMarketOnly {
			view.Blockers = append(view.Blockers, marketMacroBlocker(input.AssetType))
			view.RequiredActions = append(view.RequiredActions, marketMacroRequiredAction(input.AssetType, report.Macro.Score))
		} else {
			view.Blockers = append(view.Blockers, "makro/benchmark rejimi desteklemiyor")
			view.RequiredActions = append(view.RequiredActions, fmt.Sprintf("makro/benchmark skoru %.0f/100; portföy onayı için skor 45 üstüne, RS20 tercihen 0 üstüne çıkmalı veya stres testi bu zayıflığı telafi etmeli", report.Macro.Score))
		}
	}
	if !isMarketOnlyAssetType(input.AssetType) && !input.Professional.ValueInvesting.Computed {
		view.Blockers = append(view.Blockers, "portföy yatırım tezi için içsel değer/güvenlik marjı eksik")
	}
	if isBankInput(input) && bankCoreMetricsMissing(input.Professional) {
		view.Blockers = append(view.Blockers, "ana banka metrikleri eksik")
		view.RequiredActions = append(view.RequiredActions, "Kurumsal portföy kararı için SYR, NPL, NIM, LCR ve fonlama/veri zincirini tamamla")
		view.Score = math.Min(view.Score, 74)
		view.Confidence = math.Min(view.Confidence, 74)
	}
	if !isMarketOnly && !currentFinancialDataDecisionSafe(input.Professional.DataGovernance) {
		view.Blockers = append(view.Blockers, "finansal veri üretim kullanımı hazır değil")
		view.RequiredActions = append(view.RequiredActions, "Publish-date/available-at zincirini ve finansal mutabakatı production kullanım için tamamla")
		view.Score = math.Min(view.Score, 79)
		view.Confidence = math.Min(view.Confidence, 79)
	}
	if priceVerificationBlocksAction(input) {
		view.Blockers = append(view.Blockers, priceVerificationBlocker(input))
		view.RequiredActions = append(view.RequiredActions, "Resmi/lisanslı final kapanışı içe aktar ve fiyat kaynaklarını mutabık hale getir")
		view.Score = math.Min(view.Score, 64)
		view.Confidence = math.Min(view.Confidence, 64)
	}
	if defenseCompanyInput(input) {
		view.Blockers = append(view.Blockers, "savunma şirketi özel sürücüleri structured değil")
		view.RequiredActions = append(view.RequiredActions, "Savunma bütçesi, sipariş bakiyesi/backlog, ihracat payı, USD/EUR gelir-maliyet kırılımı, kur farkı, proje teslim takvimi ve tahsilat sürelerini structured veri olarak bağla")
		view.Score = math.Min(view.Score, 64)
		view.Confidence = math.Min(view.Confidence, 64)
	}
	view.Status = portfolioPersonaStatus(view.Score, view.Blockers)
	switch view.Status {
	case "pass":
		view.Decision = "PORTFOY_ON_ELEME"
		view.DecisionLabel = "Kurumsal portföy ön elemesine uygun"
	case "limited":
		view.Decision = "BEKLE"
		view.DecisionLabel = "Büyük yatırımcı kararı: BEKLE; yeni pozisyon açma"
	default:
		view.Decision = "PORTFOY_KULLANMAZ"
		view.DecisionLabel = "Kurumsal portföy kararı için kullanmaz"
	}
	view = applyPortfolioReportQuality(input, report, view)
	view.OneLineAnswer = fmt.Sprintf("%s: skor %.0f/100. Ana engel: %s.", view.DecisionLabel, view.Score, firstOr(view.Blockers, "kritik engel yok"))
	view.QualityAnswer = personaQualityAnswer(view)
	view.FrameworkCommentary, view.Takeaway = portfolioGateCommentary(view)
	view.TransactionUseStatus, view.TransactionUseAnswer = personaTransactionUse(view)
	_ = daily
	return view
}

func marketMacroMustHave(assetType string) string {
	if ohlcv.IsCommodityAssetType(assetType) {
		return "DXY, ABD reel faizi, COT pozisyonu ve ETF/fiziki akış teyidi"
	}
	if ohlcv.IsCryptoAssetType(assetType) {
		return "BTC dominansı, DXY/risk iştahı, funding/open interest ve on-chain/exchange-flow teyidi"
	}
	return "piyasa rejimi ve tamamlayıcı veri teyidi"
}

func marketMacroEvidenceLabel(assetType string) string {
	if ohlcv.IsCommodityAssetType(assetType) {
		return "Altın makro teyidi"
	}
	if ohlcv.IsCryptoAssetType(assetType) {
		return "Kripto piyasa teyidi"
	}
	return "Piyasa teyidi"
}

func marketMacroEvidenceDetail(input Input, report Report) string {
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		ctx := input.Professional.CommodityContext
		parts := []string{
			sectionAvailabilityLabel("DXY/reel faiz", ctx.Macro.Available, ctx.Macro.Score),
			sectionAvailabilityLabel("COT/open interest", ctx.FuturesPositioning.Available, ctx.FuturesPositioning.Score),
			sectionAvailabilityLabel("ETF/fiziki akış", ctx.GoldETFPhysicalFlow.Available, ctx.GoldETFPhysicalFlow.Score),
			sectionAvailabilityLabel("merkez bankası/haber", ctx.CentralBankGeopoliticalNews.Available, ctx.CentralBankGeopoliticalNews.Score),
		}
		return fmt.Sprintf("%s | makro skor %.0f/100", strings.Join(parts, " | "), report.Macro.Score)
	}
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		ctx := input.Professional.CryptoContext
		parts := []string{
			cryptoSectionAvailabilityLabel("on-chain", ctx.OnChain.Available, ctx.OnChain.Score),
			cryptoSectionAvailabilityLabel("derivatives", ctx.Derivatives.Available, ctx.Derivatives.Score),
			cryptoSectionAvailabilityLabel("exchange-flow", ctx.ExchangeFlow.Available, ctx.ExchangeFlow.Score),
			cryptoSectionAvailabilityLabel("haber/sentiment", ctx.NewsSentiment.Available, ctx.NewsSentiment.Score),
		}
		return fmt.Sprintf("%s | piyasa skor %.0f/100", strings.Join(parts, " | "), report.Macro.Score)
	}
	return fmt.Sprintf("%s | skor %.0f/100", empty(report.Macro.Benchmark, "piyasa teyidi yok"), report.Macro.Score)
}

func marketMacroQuestionAnswer(input Input, report Report) string {
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		return fmt.Sprintf("%s. %s.", report.Macro.Regime, marketMacroEvidenceDetail(input, report))
	}
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		return fmt.Sprintf("%s. %s.", report.Macro.Regime, marketMacroEvidenceDetail(input, report))
	}
	return fmt.Sprintf("%s. RS20 %.1f puan, beta %.2f.", report.Macro.Regime, report.Macro.RelativeStrength20, report.Macro.Beta60)
}

func sectionAvailabilityLabel(label string, available bool, score float64) string {
	if !available {
		return label + ": eksik"
	}
	if score > 0 {
		return fmt.Sprintf("%s: %.0f/100", label, score)
	}
	return label + ": var"
}

func cryptoSectionAvailabilityLabel(label string, available bool, score float64) string {
	if !available {
		return label + ": eksik"
	}
	if score > 0 {
		return fmt.Sprintf("%s: %.0f/100", label, score)
	}
	return label + ": var"
}

func marketMacroBlocker(assetType string) string {
	if ohlcv.IsCommodityAssetType(assetType) {
		return "DXY/reel faiz, COT veya ETF/fiziki akış teyidi portföy onayı için zayıf"
	}
	if ohlcv.IsCryptoAssetType(assetType) {
		return "on-chain, derivatives veya exchange-flow teyidi portföy onayı için zayıf"
	}
	return "piyasa teyidi portföy onayı için zayıf"
}

func marketMacroRequiredAction(assetType string, score float64) string {
	if ohlcv.IsCommodityAssetType(assetType) {
		return fmt.Sprintf("makro emtia skoru %.0f/100; portföy onayı için DXY/reel faiz, COMEX/COT, ETF/fiziki akış ve haber teyidi birlikte güçlenmeli", score)
	}
	if ohlcv.IsCryptoAssetType(assetType) {
		return fmt.Sprintf("kripto piyasa skoru %.0f/100; portföy onayı için on-chain, funding/open interest, exchange-flow ve haber teyidi birlikte güçlenmeli", score)
	}
	return fmt.Sprintf("piyasa teyidi %.0f/100; portföy onayı için ek stres testi gerekir", score)
}

func tradingEdgeGateView(input Input, report Report, daily Timeframe) PersonaView {
	bt := daily.Backtest
	stats := daily.SignalStats
	view := PersonaView{
		Name:   "Trading Edge Standardı",
		Lens:   "İstatistiksel edge, walk-forward, işlem maliyeti, volatilite, likidite ve giriş/çıkış disiplini",
		Status: "fail",
		MustHave: []string{
			"zaman güvenli walk-forward backtest",
			"yeterli işlem ve out-of-sample örneği",
			"pozitif expectancy ve OOS getiri",
			"işlem maliyeti/slippage dahil sonuç",
			"aktif giriş, stop ve hedef planı",
		},
	}
	backtestSafety := boolScore(bt.BacktestSafe && bt.LookaheadViolations == 0)
	tradeSample := thresholdScore(float64(bt.Trades), []threshold{{80, 100}, {50, 85}, {30, 70}, {20, 50}, {10, 30}})
	oosSample := thresholdScore(float64(bt.OutOfSampleTrades), []threshold{{25, 100}, {15, 80}, {10, 65}, {5, 40}})
	expectancyScore := signedReturnScore(bt.Expectancy, 0.04)
	oosReturnScore := signedReturnScore(bt.OutOfSampleReturn, 0.04)
	profitFactorScore := thresholdScore(bt.ProfitFactor, []threshold{{1.8, 100}, {1.4, 80}, {1.2, 65}, {1.05, 50}})
	drawdownScore := drawdownQuality(bt.MaxDrawdown)
	regimeScore := 0.0
	if !stats.InsufficientData && stats.SampleSize > 0 {
		regimeScore = weighted([]weightedScore{
			{thresholdScore(float64(stats.SampleSize), []threshold{{80, 100}, {50, 80}, {30, 60}, {15, 40}}), 0.30},
			{mathutil.Clamp(stats.WinRate*100, 0, 100), 0.35},
			{signedReturnScore(stats.AverageForwardReturn, 0.04), 0.35},
		})
	}
	activePlanScore := 0.0
	if daily.TradePlan.Direction == "long" && !daily.TradePlan.Rejected && daily.TradePlan.EntryMax > 0 && daily.TradePlan.StopLoss > 0 && daily.TradePlan.TakeProfit1 > 0 {
		activePlanScore = 100
	}
	costScore := boolScore(bt.CommissionBps >= 0 && bt.SlippageBps >= 0)
	view.Score = weighted([]weightedScore{
		{backtestSafety, 0.13},
		{tradeSample, 0.12},
		{oosSample, 0.10},
		{expectancyScore, 0.16},
		{oosReturnScore, 0.14},
		{profitFactorScore, 0.10},
		{drawdownScore, 0.08},
		{regimeScore, 0.10},
		{activePlanScore, 0.04},
		{costScore, 0.03},
	})
	view.Confidence = weighted([]weightedScore{
		{backtestSafety, 0.35},
		{tradeSample, 0.20},
		{oosSample, 0.20},
		{report.ModelRisk.Score, 0.15},
		{report.Liquidity.Score, 0.10},
	})
	view.Evidence = []EvidenceItem{
		{"Backtest güvenliği", fmt.Sprintf("strateji %s | safe=%s | lookahead %d | model %s", empty(bt.Strategy, "yok"), boolTR(bt.BacktestSafe), bt.LookaheadViolations, empty(bt.ExecutionModel, "yok")), statusBool(bt.BacktestSafe && bt.LookaheadViolations == 0), backtestSafety},
		{"Örnek sayısı", fmt.Sprintf("%d işlem | %d OOS işlem", bt.Trades, bt.OutOfSampleTrades), status(weighted([]weightedScore{{tradeSample, 0.6}, {oosSample, 0.4}})), weighted([]weightedScore{{tradeSample, 0.6}, {oosSample, 0.4}})},
		{"Expectancy", fmt.Sprintf("%.2f%%", bt.Expectancy*100), statusForPositive(bt.Expectancy), expectancyScore},
		{"OOS getiri", fmt.Sprintf("%.2f%%", bt.OutOfSampleReturn*100), statusForPositive(bt.OutOfSampleReturn), oosReturnScore},
		{"Profit factor / drawdown", fmt.Sprintf("PF %.2f | max DD %.2f%%", bt.ProfitFactor, bt.MaxDrawdown*100), status(weighted([]weightedScore{{profitFactorScore, 0.5}, {drawdownScore, 0.5}})), weighted([]weightedScore{{profitFactorScore, 0.5}, {drawdownScore, 0.5}})},
		{"Rejim istatistiği", fmt.Sprintf("%s | örnek %d | kazanma %.1f%% | ileri getiri %.2f%%", empty(stats.CurrentRegime, "rejim yok"), stats.SampleSize, stats.WinRate*100, stats.AverageForwardReturn*100), status(regimeScore), regimeScore},
		{"İşlem planı", tradingPlanEvidence(input.AssetType, daily), statusBool(activePlanScore == 100), activePlanScore},
		{"Maliyet", fmt.Sprintf("komisyon %.1f bps | slippage %.1f bps", bt.CommissionBps, bt.SlippageBps), statusBool(costScore == 100), costScore},
	}
	if bt.BacktestSafe && bt.LookaheadViolations == 0 {
		view.Passes = append(view.Passes, "backtest zaman güvenli")
	} else {
		view.Blockers = append(view.Blockers, "backtest zaman güvenliği geçmiyor")
	}
	if bt.Trades >= 30 && bt.OutOfSampleTrades >= 10 {
		view.Passes = append(view.Passes, "işlem ve OOS örnek sayısı kabul edilebilir")
	} else {
		view.Blockers = append(view.Blockers, "işlem veya OOS örnek sayısı yetersiz")
		view.RequiredActions = append(view.RequiredActions, fmt.Sprintf("Trading edge kapısı için en az 30 işlem ve 10 OOS işlem gerekir; mevcut %d işlem ve %d OOS işlem", bt.Trades, bt.OutOfSampleTrades))
	}
	if bt.Expectancy > 0 {
		view.Passes = append(view.Passes, "expectancy pozitif")
	} else {
		view.Blockers = append(view.Blockers, "expectancy pozitif değil")
	}
	if bt.OutOfSampleReturn > 0 {
		view.Passes = append(view.Passes, "out-of-sample getiri pozitif")
	} else {
		view.Blockers = append(view.Blockers, "out-of-sample getiri pozitif değil")
	}
	if stats.SampleSize >= 30 && stats.WinRate >= 0.52 && stats.AverageForwardReturn > 0 {
		view.Passes = append(view.Passes, "benzer rejim istatistiği destekliyor")
	} else {
		view.Blockers = append(view.Blockers, "benzer rejim istatistiği yeterince güçlü değil")
	}
	if activePlanScore == 100 {
		view.Passes = append(view.Passes, "giriş/stop/hedef planı aktif")
	} else {
		view.Blockers = append(view.Blockers, "aktif giriş/stop/hedef planı yok")
		view.RequiredActions = append(view.RequiredActions, "canlı al/sat için veriyle üretilmiş long giriş, zarar kes ve hedef planı aktif olmalı")
	}
	if bt.MaxDrawdown < -0.30 {
		view.Blockers = append(view.Blockers, "max drawdown hedge fund eşiği için yüksek")
	}
	if report.Liquidity.Score < 45 {
		view.Blockers = append(view.Blockers, "likidite aktif trading için sınırlı")
	}
	if priceVerificationBlocksAction(input) {
		view.Blockers = append(view.Blockers, priceVerificationBlocker(input))
		view.RequiredActions = append(view.RequiredActions, "Günlük sinyal yalnızca resmi/final kapanış verified olduktan sonra canlı AL/SAT adayı olabilir")
		view.Score = math.Min(view.Score, 54)
		view.Confidence = math.Min(view.Confidence, 54)
	}
	if bt.Expectancy <= 0 || bt.OutOfSampleReturn <= 0 {
		view.RequiredActions = append(view.RequiredActions, "sinyali işlem masasına almadan önce expectancy, OOS ve maliyet sonrası getiri pozitif olmalı")
	}
	view.Status = tradingPersonaStatus(view.Score, view.Blockers)
	switch view.Status {
	case "pass":
		view.Decision = "TRADING_ADAYI"
		view.DecisionLabel = "Aktif trading araştırma masasına aday"
	case "limited":
		view.Decision = "IZLE"
		view.DecisionLabel = "Trading radarı; canlı işlem için ek istatistik gerekir"
	default:
		view.Decision = "TRADING_KULLANMAZ"
		view.DecisionLabel = "Aktif trading sistemi için kullanmaz"
	}
	view = applyTradingEdgeReportQuality(report, daily, view)
	view.OneLineAnswer = fmt.Sprintf("%s: skor %.0f/100. Ana engel: %s.", view.DecisionLabel, view.Score, firstOr(view.Blockers, "kritik engel yok"))
	view.QualityAnswer = personaQualityAnswer(view)
	view.FrameworkCommentary, view.Takeaway = tradingEdgeGateCommentary(view)
	view.TransactionUseStatus, view.TransactionUseAnswer = personaTransactionUse(view)
	_ = input
	return view
}

func combinedPersonaStatus(statuses []string) string {
	fail := false
	limited := false
	passCount := 0
	applicable := 0
	for _, status := range statuses {
		switch status {
		case "not_applicable":
			continue
		case "pass":
			passCount++
			applicable++
		case "limited":
			limited = true
			applicable++
		default:
			fail = true
			applicable++
		}
	}
	if applicable == 0 {
		return "not_applicable"
	}
	if fail {
		return "fail"
	}
	if limited || passCount < applicable {
		return "limited"
	}
	return "pass"
}

func personaSummary(input Input, views InstitutionalPersonaViews) string {
	if isMarketOnlyAssetType(input.AssetType) {
		return fmt.Sprintf("%s yatırım/işlem uygunluğu: kurumsal portföy %s, trading edge %s; genel %s. Rapor kalitesi: %s.",
			marketAssetLabel(input.AssetType),
			institutionalStatusText(views.Portfolio.Status),
			institutionalStatusText(views.TradingEdge.Status),
			institutionalStatusText(views.OverallStatus),
			institutionalStatusText(views.OverallQualityStatus),
		)
	}
	return fmt.Sprintf("Yatırım/işlem uygunluğu: değer yatırım %s, kurumsal portföy %s, trading edge %s; genel %s. Rapor kalitesi: %s.",
		institutionalStatusText(views.ValueInvesting.Status),
		institutionalStatusText(views.Portfolio.Status),
		institutionalStatusText(views.TradingEdge.Status),
		institutionalStatusText(views.OverallStatus),
		institutionalStatusText(views.OverallQualityStatus),
	)
}

func personaQualitySummary(input Input, views InstitutionalPersonaViews) string {
	if isMarketOnlyAssetType(input.AssetType) {
		return fmt.Sprintf("Rapor kalitesi: kurumsal portföy %s, trading edge %s; genel %s.",
			institutionalStatusText(views.Portfolio.ReportQualityStatus),
			institutionalStatusText(views.TradingEdge.ReportQualityStatus),
			institutionalStatusText(views.OverallQualityStatus),
		)
	}
	return fmt.Sprintf("Rapor kalitesi: değer yatırım %s, kurumsal portföy %s, trading edge %s; genel %s.",
		institutionalStatusText(views.ValueInvesting.ReportQualityStatus),
		institutionalStatusText(views.Portfolio.ReportQualityStatus),
		institutionalStatusText(views.TradingEdge.ReportQualityStatus),
		institutionalStatusText(views.OverallQualityStatus),
	)
}

func eliteGate(input Input, views InstitutionalPersonaViews) EliteGate {
	assetName := "hisse"
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		assetName = "kripto varlık"
	} else if ohlcv.IsCommodityAssetType(input.AssetType) {
		assetName = "altın/emtia"
	}
	gate := EliteGate{
		Computed: true,
		RequiredPasses: []string{
			"Değer yatırım tezi Geçti",
			"Kurumsal portföy uygunluğu Geçti",
			"Trading edge sinyali Geçti",
		},
	}
	if isMarketOnlyAssetType(input.AssetType) {
		gate.RequiredPasses = []string{
			"Kurumsal " + strings.ToLower(marketAssetLabel(input.AssetType)) + " portföy uygunluğu Geçti",
			"Trading edge sinyali Geçti",
		}
	}
	var statuses []struct {
		label  string
		status string
		score  float64
	}
	if isMarketOnlyAssetType(input.AssetType) {
		statuses = []struct {
			label  string
			status string
			score  float64
		}{
			{"Kurumsal " + strings.ToLower(marketAssetLabel(input.AssetType)) + " portföy uygunluğu", views.Portfolio.Status, views.Portfolio.Score},
			{"Trading edge sinyali", views.TradingEdge.Status, views.TradingEdge.Score},
		}
	} else {
		statuses = []struct {
			label  string
			status string
			score  float64
		}{
			{"Değer yatırım tezi", views.ValueInvesting.Status, views.ValueInvesting.Score},
			{"Kurumsal portföy uygunluğu", views.Portfolio.Status, views.Portfolio.Score},
			{"Trading edge sinyali", views.TradingEdge.Status, views.TradingEdge.Score},
		}
	}
	total := 0.0
	for _, item := range statuses {
		total += item.score
		if item.status != "pass" {
			gate.FailedPasses = append(gate.FailedPasses, item.label+" "+institutionalStatusText(item.status))
		}
	}
	gate.Score = mathutil.Clamp(mathutil.SafeDiv(total, float64(len(statuses))), 0, 100)
	if len(gate.FailedPasses) == 0 {
		gate.Status = "pass"
		gate.Label = "Üç aksiyon kapısı geçti"
		if isMarketOnlyAssetType(input.AssetType) {
			gate.Label = marketAssetLabel(input.AssetType) + " aksiyon kapıları geçti"
		}
		gate.Summary = "Bu " + assetName + " kurumsal portföy uygunluğu ve trading edge sinyali açısından aksiyon kapılarını geçiyor."
		return gate
	}
	gate.Status = "fail"
	gate.Label = "Üç aksiyon kapısı geçmedi"
	if isMarketOnlyAssetType(input.AssetType) {
		gate.Label = marketAssetLabel(input.AssetType) + " aksiyon kapıları geçmedi"
	}
	gate.Summary = "Bu " + assetName + " için rapor üretildi; ancak AL/işlem adayı sayılması için gereken aksiyon kapıları aynı anda geçmedi. Geçmeyen alanlar: " + strings.Join(gate.FailedPasses, "; ") + ". Bu, raporun kalitesinin değil, mevcut yatırım/işlem tezinin yetersiz olduğunu gösterir."
	return gate
}

func financialTransactionUseGate(input Input, views InstitutionalPersonaViews) TransactionUseGate {
	gate := TransactionUseGate{
		Computed:       true,
		RequiredPasses: views.EliteCandidate.RequiredPasses,
		FailedPasses:   views.EliteCandidate.FailedPasses,
	}
	if views.EliteCandidate.Status == "pass" {
		gate.Status = "pass"
		if isMarketOnlyAssetType(input.AssetType) {
			gate.Answer = "Evet; " + strings.ToLower(marketAssetLabel(input.AssetType)) + " işlem komitesi için aday veri seti olabilir, ancak tek başına otomatik emir değildir."
			gate.Summary = "Kurumsal portföy uygunluğu ve trading edge sinyali geçtiği için rapor " + strings.ToLower(marketAssetLabel(input.AssetType)) + " risk/işlem masasına taşınabilir; nihai işlem portföy limiti, saklama/uyum ve risk onayına bağlıdır."
			return gate
		}
		gate.Answer = "Evet; rapor kurumsal al/sat karar sürecinde kullanılabilir adaydır, ancak tek başına otomatik emir değildir."
		gate.Summary = "Değer yatırım tezi, kurumsal portföy uygunluğu ve trading edge sinyali aynı anda geçtiği için rapor işlem komitesi/risk masasına taşınabilir; nihai emir portföy limiti, compliance ve risk onayına bağlıdır."
		return gate
	}
	if len(gate.FailedPasses) == 0 {
		gate.FailedPasses = []string{"üç aksiyon kapısı geçmedi"}
	}
	if priceVerificationBlocksAction(input) {
		gate.FailedPasses = appendUnique(gate.FailedPasses, "resmi/final kapanış doğrulanmadı")
	}
	if !isMarketOnlyAssetType(input.AssetType) && !input.Professional.DataGovernance.ProductionReady {
		gate.FailedPasses = appendUnique(gate.FailedPasses, "finansal veri production kullanımına hazır değil")
	}
	if views.OverallQualityStatus == "pass" {
		gate.Status = "limited"
		if isMarketOnlyAssetType(input.AssetType) {
			gate.Answer = "Hayır; doğrudan " + strings.ToLower(marketAssetLabel(input.AssetType)) + " al/sat kararı için kullanılamaz. Rapor yalnızca ön değerlendirme ve risk izleme girdisidir."
			gate.Summary = "Rapor bazı kalite kontrollerini geçse de aksiyon kapıları geçmediği için canlı işlem kararı üretmez. Geçmeyen kapılar: " + strings.Join(gate.FailedPasses, "; ") + "."
			return gate
		}
		gate.Answer = "Karar: BEKLE; yeni pozisyon açma. Mevcut pozisyonda risk seviyesini izle."
		gate.Summary = "Karar raporu üretildi; mevcut yatırım/işlem tezi aksiyon kapılarını geçmedi. Geçmeyen kapılar: " + strings.Join(gate.FailedPasses, "; ") + "."
		return gate
	}
	gate.Status = "fail"
	if isMarketOnlyAssetType(input.AssetType) {
		gate.Answer = "Hayır; rapor kalite kapıları geçmediği için " + strings.ToLower(marketAssetLabel(input.AssetType)) + " al/sat karar desteği seviyesinde değildir."
		gate.Summary = "Rapor kalite kapıları geçmedi; geçmeyen kapılar: " + strings.Join(gate.FailedPasses, "; ") + ". Bu çıktı işlem açmak için değil, hangi teyitlerin gerektiğini görmek için kullanılmalıdır."
		return gate
	}
	gate.Answer = "Karar: REDDET; yeni pozisyon açma."
	gate.Summary = "Karar kalite kapıları geçmediği için yatırım tezi reddedildi. Geçmeyen kapılar: " + strings.Join(gate.FailedPasses, "; ") + "."
	return gate
}

func personaTransactionUse(view PersonaView) (string, string) {
	blocker := firstOr(view.Blockers, "kritik engel yok")
	switch view.Name {
	case "Değer Yatırım Standardı":
		switch view.Status {
		case "pass":
			return "pass", "Evet; uzun vadeli alım komitesine aday olarak kullanır, fakat tek başına otomatik emir değildir."
		case "limited":
			return "limited", "Doğrudan alım için kullanmaz; izleme listesi ve ek değer/kalite kanıtı için kullanır. Ana engel: " + blocker + "."
		case "not_applicable":
			return "not_applicable", "Hayır; bu varlık şirket finansalı temelli değer yatırım çerçevesine uygun değil."
		default:
			return "fail", "Hayır; alım işlemi için kullanmaz. Ana engel: " + blocker + "."
		}
	case "Kurumsal Portföy Standardı":
		switch view.Status {
		case "pass":
			return "limited", "Ön eleme ve pozisyon taslağı için evet; tek başına doğrudan AL/SAT emri için hayır. Nihai işlem için değer yatırım tezi, trading edge, portföy limiti ve compliance kapıları ayrıca geçmelidir."
		case "limited":
			return "limited", "Karar: BEKLE; yeni kurumsal pozisyon açma. Mevcut pozisyonu risk limiti ve stres testiyle koru. Ana engel: " + blocker + "."
		default:
			return "fail", "Hayır; kurumsal portföy işlemi için kullanmaz. Ana engel: " + blocker + "."
		}
	case "Trading Edge Standardı":
		switch view.Status {
		case "pass":
			return "pass", "Evet; trading araştırma masasına aday sinyal olarak kullanabilir, canlı işlem öncesi risk limiti ve execution kontrolü gerekir."
		case "limited":
			return "limited", "Canlı al/sat için kullanmaz; en fazla izleme veya paper-trade adayı olur. Ana engel: " + blocker + "."
		default:
			return "fail", "Hayır; canlı al/sat sinyali olarak kullanmaz. Ana engel: " + blocker + "."
		}
	default:
		if view.Status == "pass" {
			return "pass", "Evet; karar sürecinde kullanılabilir, nihai işlem onaya bağlıdır."
		}
		return view.Status, "Hayır; işlem kullanımı için yeterli kapı geçmedi. Ana engel: " + blocker + "."
	}
}

func applyValueInvestingReportQuality(input Input, report Report, view PersonaView) PersonaView {
	reasons := []string{}
	if isMarketOnlyAssetType(input.AssetType) {
		view.ReportQualityScore = weighted([]weightedScore{
			{boolScore(view.DecisionLabel != ""), 0.40},
			{boolScore(len(view.Blockers) > 0), 0.30},
			{report.ModelRisk.Score, 0.30},
		})
		view.ReportQualityReasons = []string{"Geleneksel finansal tablo temelli içsel değer çerçevesinin bu varlığa neden uygulanmadığı açıklandı."}
		view.ReportQualityStatus = qualityStatus(view.ReportQualityScore)
		view.ReportQualityLabel = qualityLabel(view.ReportQualityStatus)
		return view
	}
	v := input.Professional.ValueInvesting
	isBank := isBankInput(input)
	bankMissingCore := isBank && bankCoreMetricsMissing(input.Professional)
	evidenceScore := completenessScore(len(view.Evidence), 6)
	frameworkScore := 0.0
	if v.SectorModel.Model != "" {
		frameworkScore += 20
	}
	if isBank {
		frameworkScore += 20
		if !bankMissingCore {
			frameworkScore += 20
		}
	} else {
		if v.OwnerEarnings.Applicable || v.OwnerEarnings.TTM != 0 || v.OwnerEarnings.TotalYears > 0 {
			frameworkScore += 20
		}
		if v.NormalizedFCF.Applicable || v.NormalizedFCF.TTM != 0 || v.NormalizedFCF.Median5Y != 0 {
			frameworkScore += 20
		}
	}
	if v.Moat.Score > 0 {
		frameworkScore += 20
	}
	if v.CapitalAllocation.Score > 0 {
		frameworkScore += 20
	}
	explanationScore := 55.0
	if len(view.Blockers) > 0 || view.Status == "pass" {
		explanationScore += 25
	}
	if len(view.RequiredActions) > 0 || view.Status == "pass" {
		explanationScore += 20
	}
	dataScore := weighted([]weightedScore{
		{v.DataQuality, 0.45},
		{v.Confidence, 0.25},
		{report.ModelRisk.Score, 0.20},
		{report.Governance.Score, 0.10},
	})
	view.ReportQualityScore = weighted([]weightedScore{
		{evidenceScore, 0.24},
		{frameworkScore, 0.24},
		{explanationScore, 0.22},
		{dataScore, 0.22},
		{boolScore(v.Summary != ""), 0.08},
	})
	if bankMissingCore {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 74)
		reasons = append(reasons, "Banka çekirdek metrikleri eksik: SYR, çekirdek sermaye, NPL, NIM, LCR ve karşılık gideri olmadan değer yatırım raporu tam kanıt sayılmaz.")
	}
	if !isBank && lowCashConversionRisk(input) {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 74)
		reasons = append(reasons, "Net kâr pozitif olsa da FCF/net kâr dönüşümü zayıf; değerleme güveni nakit akımı köprüsü mutabakatı tamamlanmadan tam geçmez.")
	}
	if v.CapitalAllocation.Dilution5YPct > 10 {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 74)
		reasons = append(reasons, "Ödenmiş sermaye değişimi bedelli/bedelsiz/split/nominal düzeltme olarak sınıflandırılmadan hisse başına değerleme tam güvenli değildir.")
	}
	if ok, reason := valuationModelAuditIssue(input); ok {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 64)
		reasons = append(reasons, reason)
	}
	if defenseCompanyInput(input) && input.Professional.Peers.PeerCount < 5 {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 74)
		reasons = append(reasons, "Savunma şirketlerinde peer evreni en az yerli savunma, global defense, teknoloji ve kamu sözleşmeli şirket ayrımlarıyla genişletilmeden model güveni tam geçmez.")
	}
	if evidenceScore >= 80 {
		if isBank {
			reasons = append(reasons, "Banka değerleme çerçevesi içsel değer, güvenlik marjı, sürdürülebilir ROE, aktif kalitesi ve sermaye/fonlama kanıtlarıyla değerlendirilir.")
		} else {
			reasons = append(reasons, "Değer yatırım kanıt seti tam: içsel değer, güvenlik marjı, owner earnings, FCF, moat ve sermaye tahsisi raporda var.")
		}
	}
	if !v.IntrinsicValue.Computed || !v.MarginOfSafety.Computed {
		reasons = append(reasons, "İçsel değer/güvenlik marjı hesaplanamıyorsa bu yatırım başarısı değil, doğru ret gerekçesi olarak raporlanıyor.")
	}
	if len(view.RequiredActions) > 0 {
		reasons = append(reasons, "Eksik kanıtlar aksiyon listesine bağlandı.")
	}
	view.ReportQualityStatus = qualityStatus(view.ReportQualityScore)
	view.ReportQualityLabel = qualityLabel(view.ReportQualityStatus)
	view.ReportQualityReasons = reasons
	return view
}

func applyPortfolioReportQuality(input Input, report Report, view PersonaView) PersonaView {
	isMarketOnly := isMarketOnlyAssetType(input.AssetType)
	bankMissingCore := isBankInput(input) && bankCoreMetricsMissing(input.Professional)
	coverageScore := input.Professional.Coverage.Score
	peerExplained := boolScore(input.Professional.Company.Sector != "" && (input.Professional.Peers.PeerCount > 0 || isMarketOnly))
	view.ReportQualityScore = weighted([]weightedScore{
		{completenessScore(len(view.Evidence), 6), 0.24},
		{report.Liquidity.Score, 0.16},
		{report.Governance.Score, 0.16},
		{report.ModelRisk.Score, 0.18},
		{coverageScore, 0.12},
		{peerExplained, 0.08},
		{boolScore(len(view.Blockers) > 0 || view.Status == "pass"), 0.06},
	})
	reason := "Likidite, kapasite, benchmark/beta, sektör/peer, veri yönetişimi ve model riski ayrı kanıtlarla raporlandı."
	if isMarketOnly {
		if ohlcv.IsCommodityAssetType(input.AssetType) {
			reason = "Likidite, kapasite, DXY/reel faiz, COT, ETF/fiziki akış, altın/emtia veri kapsamı, veri yönetişimi ve model riski ayrı kanıtlarla raporlandı."
		} else {
			reason = "Likidite, kapasite, kripto piyasa teyidi, on-chain/derivatives/exchange-flow, veri kapsamı, veri yönetişimi ve model riski ayrı kanıtlarla raporlandı."
		}
	}
	reasons := []string{reason}
	if len(view.RequiredActions) > 0 {
		reasons = append(reasons, "Portföy/stres testi için gerekli ek çalışma ayrıca yazıldı.")
	}
	if bankMissingCore {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 74)
		reasons = append(reasons, "Banka çekirdek metrikleri eksik olduğu için portföy rapor kalitesi tam geçer sayılamaz.")
	}
	if !isMarketOnly && !currentFinancialDataDecisionSafe(input.Professional.DataGovernance) {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 79)
		reasons = append(reasons, "Finansal veri production kullanımına hazır olmadığı için portföy raporu karar girdisi olarak sınırlıdır.")
	}
	if priceVerificationBlocksAction(input) {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 64)
		reasons = append(reasons, "Resmi/final kapanış doğrulanmadığı için portföy raporu işlem veya pozisyon kararı olarak tam geçmez.")
	}
	if defenseCompanyInput(input) {
		view.ReportQualityScore = math.Min(view.ReportQualityScore, 74)
		reasons = append(reasons, "Savunma şirketi için backlog, ihracat, kur kırılımı, kamu bütçesi ve proje teslimat verileri structured olarak bağlanmadan portföy raporu tam geçmez.")
	}
	view.ReportQualityStatus = qualityStatus(view.ReportQualityScore)
	view.ReportQualityLabel = qualityLabel(view.ReportQualityStatus)
	view.ReportQualityReasons = reasons
	return view
}

func applyTradingEdgeReportQuality(report Report, daily Timeframe, view PersonaView) PersonaView {
	bt := daily.Backtest
	stats := daily.SignalStats
	metricsPresent := 0.0
	if bt.Trades > 0 {
		metricsPresent += 20
	}
	if bt.OutOfSampleTrades > 0 {
		metricsPresent += 20
	}
	if bt.ExecutionModel != "" {
		metricsPresent += 15
	}
	if bt.CommissionBps >= 0 && bt.SlippageBps >= 0 {
		metricsPresent += 15
	}
	if stats.SampleSize > 0 && stats.ForwardBars > 0 {
		metricsPresent += 15
	}
	if daily.TradePlan.RejectReason != "" || len(daily.TradePlan.Reasoning) > 0 || (daily.TradePlan.Direction == "long" && !daily.TradePlan.Rejected) {
		metricsPresent += 15
	}
	backtestIntegrity := boolScore(bt.BacktestSafe && bt.LookaheadViolations == 0)
	view.ReportQualityScore = weighted([]weightedScore{
		{completenessScore(len(view.Evidence), 8), 0.24},
		{metricsPresent, 0.24},
		{backtestIntegrity, 0.20},
		{report.ModelRisk.Score, 0.14},
		{report.Liquidity.Score, 0.08},
		{boolScore(len(view.Blockers) > 0 || view.Status == "pass"), 0.10},
	})
	view.ReportQualityReasons = []string{
		"Backtest güvenliği, işlem/OOS sayısı, expectancy, OOS getiri, drawdown, rejim istatistiği, maliyet ve plan durumu ayrı kanıtlarla raporlandı.",
	}
	if view.Status != "pass" {
		view.ReportQualityReasons = append(view.ReportQualityReasons, "Sinyal kötü ise rapor bunu trading başarısı gibi göstermiyor; blokaj gerekçesi yazıyor.")
	}
	view.ReportQualityStatus = qualityStatus(view.ReportQualityScore)
	view.ReportQualityLabel = qualityLabel(view.ReportQualityStatus)
	return view
}

func personaQualityAnswer(view PersonaView) string {
	return fmt.Sprintf("%s rapor kalitesi %s: %.0f/100. %s",
		view.Name,
		institutionalStatusText(view.ReportQualityStatus),
		view.ReportQualityScore,
		firstOr(view.ReportQualityReasons, "Kanıt seti ve açıklanabilirlik değerlendirildi."),
	)
}

func valueInvestingGateCommentary(view PersonaView) (string, string) {
	isBankFramework := valueInvestingViewUsesBankFramework(view)
	switch {
	case view.Status == "not_applicable":
		return "Bu varlık şirket finansalı temelli değer çerçevesine uygun değil; rapor fiyat, likidite, makro/piyasa teyidi ve risk yönetimini ayrı okuduğu için doğru çerçevede kalıyor.",
			"Geleneksel değer çerçevesi uygulanmaz; rapor bunu yatırım başarısı gibi göstermemeli."
	case isBankFramework && view.ReportQualityStatus == "pass" && view.Status == "pass":
		return "Banka değerleme komitesi için içsel değer, güvenlik marjı, sürdürülebilir ROE, aktif kalitesi ve sermaye/fonlama kanıtları birlikte okunuyor; tez eşikleri geçiyor.",
			"Rapor banka çerçevesinde komiteye taşınabilir; sermaye yeterliliği ve aktif kalitesi izlenmeli."
	case isBankFramework && view.ReportQualityStatus == "pass" && view.Status == "limited":
		return "Rapor banka değerleme çerçevesini doğru kullanıyor; ancak karar için sermaye yeterliliği, aktif kalitesi veya güvenlik marjı kanıtı tam değil.",
			"Rapor ön değerlendirme için kullanılabilir; banka çekirdek metrikleri tamamlanmadan alım kararı verilmemeli."
	case isBankFramework:
		return "Rapor banka değer yatırım çerçevesi için eksik; klasik owner earnings/FCF yerine SYR, NPL, NIM, LCR, karşılık gideri ve sürdürülebilir ROE kanıtları bağlanmalı.",
			"Banka değerleme kalitesi güçlenmeden değer yatırım kararı verilmemeli."
	case view.ReportQualityStatus == "pass" && view.Status == "pass":
		return "Değer yatırım komitesi için gereken kanıt seti tamam: içsel değer, güvenlik marjı, owner earnings, FCF, moat ve sermaye tahsisi birlikte okunuyor; yatırım tezi de eşikleri geçiyor.",
			"Rapor komiteye taşınabilir; güvenlik marjı ve iş kalitesi birlikte destek veriyor."
	case view.ReportQualityStatus == "pass" && view.Status == "limited":
		return "Rapor değer yatırım standardını karşılıyor; temel değer, güvenlik marjı, nakit akışı ve kalite soruları doğru sırayla cevaplanıyor. Ancak karar için güvenlik marjı veya kalite kanıtı henüz tam değil.",
			"Rapor başarılı; yatırım kararı için ek değer/kalite teyidi gerekir."
	case view.ReportQualityStatus == "pass":
		return "Rapor değer yatırım standardındaki ana soruları eksiksiz soruyor ve zayıf tezi net biçimde reddediyor: içsel değer/güvenlik marjı, owner earnings/FCF ve sermaye tahsisi yatırım için yeterli değil.",
			"Rapor başarılı; hisse için değer yatırım alım tezi başarısız."
	default:
		return "Rapor değer yatırım çerçevesi için eksik; değer, güvenlik marjı veya nakit akışı kanıtı daha açık bağlanmalı.",
			"Rapor kalitesi güçlenmeden değer yatırım kararı verilmemeli."
	}
}

func valueInvestingViewUsesBankFramework(view PersonaView) bool {
	text := strings.ToLower(strings.Join(append(append([]string{}, view.MustHave...), view.Lens, view.Name), " "))
	return strings.Contains(text, "syr") || strings.Contains(text, "npl") || strings.Contains(text, "nim") || strings.Contains(text, "lcr") || strings.Contains(text, "banka")
}

func portfolioGateCommentary(view PersonaView) (string, string) {
	mustHave := strings.Join(view.MustHave, " ")
	switch {
	case view.ReportQualityStatus == "pass" && view.Status == "pass":
		if strings.Contains(mustHave, "DXY") {
			return "Rapor kurumsal altın/emtia komitesi için gerekli zemini sağlıyor: likidite, kapasite, DXY/reel faiz, COT, ETF/fiziki akış, veri yönetişimi ve model riski birlikte değerlendiriliyor.",
				"Portföy komitesine taşınabilir; pozisyon boyutu stres testiyle belirlenmeli."
		}
		if strings.Contains(mustHave, "on-chain") {
			return "Rapor kurumsal kripto komitesi için gerekli zemini sağlıyor: likidite, kapasite, piyasa rejimi, on-chain, derivatives, exchange-flow, veri kapsamı ve model riski birlikte değerlendiriliyor.",
				"Portföy komitesine taşınabilir; pozisyon boyutu stres testiyle belirlenmeli."
		}
		return "Rapor kurumsal portföy komitesi için gerekli zemini sağlıyor: likidite, kapasite, benchmark/beta, veri yönetişimi, model riski ve portföy etkisi birlikte değerlendiriliyor.",
			"Portföy komitesine taşınabilir; pozisyon boyutu stres testiyle belirlenmeli."
	case view.ReportQualityStatus == "pass" && view.Status == "limited":
		if strings.Contains(mustHave, "DXY") {
			return "Rapor kurumsal altın/emtia komitesi için gerekli ön bilgiyi sağlıyor: likidite, kapasite, DXY/reel faiz, COT, ETF/fiziki akış ve model riski açık. Ancak portföy onayı için makro/pozisyon/akış teyidi güçlenmeli.",
				"Rapor başarılı; büyük altın/emtia pozisyon kararı ek stres ve makro teyide bağlı."
		}
		if strings.Contains(strings.Join(view.Blockers, " "), "kripto veri kaynakları") {
			return "Rapor kurumsal kripto komitesi için gerekli ön bilgiyi sağlıyor: likidite, kapasite, piyasa rejimi, veri kapsamı ve model riski açık. Ancak portföy onayı için on-chain, derivatives, exchange-flow ve haber/sentiment teyidi güçlenmeli.",
				"Rapor başarılı; büyük kripto pozisyon kararı ek stres ve kripto veri teyidine bağlı."
		}
		return "Rapor kurumsal komite için gerekli ön bilgiyi sağlıyor: likidite, kapasite, benchmark/beta, veri yönetişimi ve model riski açık. Ancak portföy onayı için stres testi ve değerleme kanıtı güçlenmeli.",
			"Rapor başarılı; büyük kurumsal pozisyon kararı ek stres/değerleme kanıtına bağlı."
	case view.ReportQualityStatus == "pass":
		if strings.Contains(mustHave, "DXY") {
			return "Rapor kurumsal risk dilini doğru kuruyor ve portföy uygunluğu zayıfsa bunu gizlemiyor; likidite, DXY/reel faiz, COT, ETF/fiziki akış, veri yönetişimi ve model riski ayrı ayrı gösteriliyor.",
				"Rapor başarılı; bu altın/emtia görünümü için portföy uygunluğu başarısız."
		}
		if strings.Contains(mustHave, "on-chain") {
			return "Rapor kurumsal risk dilini doğru kuruyor ve portföy uygunluğu zayıfsa bunu gizlemiyor; likidite, piyasa rejimi, kripto veri kapsamı ve model riski ayrı ayrı gösteriliyor.",
				"Rapor başarılı; bu kripto görünümü için portföy uygunluğu başarısız."
		}
		return "Rapor kurumsal risk dilini doğru kuruyor ve portföy uygunluğu zayıfsa bunu gizlemiyor; likidite, benchmark, veri yönetişimi ve model riski ayrı ayrı gösteriliyor.",
			"Rapor başarılı; bu varlık için portföy uygunluğu başarısız."
	default:
		return "Kurumsal portföy formatı için likidite, benchmark, veri yönetişimi veya risk kanıtı daha açık bağlanmalı.",
			"Rapor kalitesi güçlenmeden portföy komitesine taşınmamalı."
	}
}

func tradingEdgeGateCommentary(view PersonaView) (string, string) {
	switch {
	case view.ReportQualityStatus == "pass" && view.Status == "pass":
		return "Rapor aktif trading masası için gerekli edge metriklerini veriyor ve sinyal istatistikleri geçiyor: walk-forward, OOS, expectancy, maliyet ve plan birlikte destekli.",
			"Trading araştırma masasına aday; canlıya geçmeden risk limitleri uygulanmalı."
	case view.ReportQualityStatus == "pass" && view.Status == "limited":
		return "Rapor trading için gerekli istatistikleri açık veriyor; fakat edge, örnek sayısı veya aktif plan canlı işlem için tam yeterli değil.",
			"Rapor başarılı; izleme/paper-trade için uygun, production trading için ek istatistik gerekir."
	case view.ReportQualityStatus == "pass":
		return "Rapor trading masası standardındaki edge metriklerini gösteriyor ve sinyal yetersizliğini saklamıyor: expectancy, OOS getiri, örnek sayısı veya aktif plan zayıfsa işlem açılmaz.",
			"Rapor başarılı; bu sinyal production trading için başarısız."
	default:
		return "Trading formatı için backtest, OOS, maliyet, drawdown ve aktif plan kanıtı daha açık olmalı.",
			"Rapor kalitesi güçlenmeden trading sinyali üretmemeli."
	}
}

func completenessScore(count int, want int) float64 {
	if want <= 0 {
		return 0
	}
	return mathutil.Clamp(float64(count)/float64(want)*100, 0, 100)
}

func qualityStatus(score float64) string {
	switch {
	case score >= 75:
		return "pass"
	case score >= 55:
		return "limited"
	default:
		return "fail"
	}
}

func qualityLabel(status string) string {
	switch status {
	case "pass":
		return "Profesyonel rapor kalite kapısı geçti"
	case "limited":
		return "Rapor kalitesi sınırlı; ek veri/kanıt gerekir"
	default:
		return "Rapor kalitesi profesyonel karar desteği için yetersiz"
	}
}

func strictPersonaStatus(score float64, blockers []string, hardBlockers []string) string {
	for _, blocker := range blockers {
		for _, hard := range hardBlockers {
			if blocker == hard {
				return "fail"
			}
		}
	}
	if score >= 75 && len(blockers) == 0 {
		return "pass"
	}
	if score >= 55 {
		return "limited"
	}
	return "fail"
}

func portfolioPersonaStatus(score float64, blockers []string) string {
	if score >= 75 && len(blockers) == 0 {
		return "pass"
	}
	if score >= 55 {
		return "limited"
	}
	return "fail"
}

func tradingPersonaStatus(score float64, blockers []string) string {
	hard := map[string]bool{
		"backtest zaman güvenliği geçmiyor":         true,
		"expectancy pozitif değil":                  true,
		"out-of-sample getiri pozitif değil":        true,
		"işlem veya OOS örnek sayısı yetersiz":      true,
		"aktif giriş/stop/hedef planı yok":          true,
		"max drawdown hedge fund eşiği için yüksek": true,
	}
	for _, blocker := range blockers {
		if hard[blocker] {
			return "fail"
		}
	}
	if score >= 78 && len(blockers) == 0 {
		return "pass"
	}
	if score >= 58 {
		return "limited"
	}
	return "fail"
}

func lowCashConversionRisk(input Input) bool {
	v := input.Professional.Valuation
	if v.NetIncomeTTM <= 0 {
		return false
	}
	ratio := v.FreeCashFlowTTM / v.NetIncomeTTM
	return ratio < 0.40
}

func valuationModelAuditIssue(input Input) (bool, string) {
	v := input.Professional.ValueInvesting
	if !v.Computed || v.IntrinsicValue.Base <= 0 {
		return false, ""
	}
	if peerBase := input.Professional.Valuation.FairValue.Base; peerBase > 0 {
		divergencePct := math.Abs(peerBase-v.IntrinsicValue.Base) / v.IntrinsicValue.Base * 100
		if divergencePct > 50 {
			return true, fmt.Sprintf("İçsel değer baz %.2f %s ile peer/model kontrol değeri %.2f %s arasında %.1f%% model kontrol ayrışması var; hisse adedi, ölçek, sermaye hareketleri, normalize FCF ve model varsayımları mutabık hale getirilmeli", v.IntrinsicValue.Base, input.Currency, peerBase, input.Currency, divergencePct)
		}
	}
	if v.CurrentPrice > 0 {
		priceGapPct := (v.CurrentPrice/v.IntrinsicValue.Base - 1) * 100
		if priceGapPct >= 250 && (lowCashConversionRisk(input) || v.CapitalAllocation.Dilution5YPct > 10) {
			return true, fmt.Sprintf("Fiyat/baz değer farkı %.1f%%; zayıf FCF dönüşümü veya sermaye artışı etkisi varken bu fark resmi değerleme sonucu değil, fiyat-baz denetim sinyali olarak gösterilmeli", priceGapPct)
		}
	}
	return false, ""
}

func defenseCompanyInput(input Input) bool {
	text := strings.ToUpper(strings.Join([]string{
		input.Symbol,
		input.CompanyName,
		input.Professional.Company.Sector,
		input.Professional.Company.Industry,
		input.Professional.Peers.PeerGroup,
		input.Professional.Peers.Sector,
	}, " "))
	return strings.Contains(text, "SAVUNMA") ||
		strings.Contains(text, "DEFENSE") ||
		strings.Contains(text, "ASELS")
}

func intrinsicEvidence(input Input) string {
	v := input.Professional.ValueInvesting
	if v.IntrinsicValue.Computed {
		return fmt.Sprintf("bear %.2f | base %.2f | bull %.2f %s", v.IntrinsicValue.Bear, v.IntrinsicValue.Base, v.IntrinsicValue.Bull, input.Currency)
	}
	return "pozitif/güvenilir içsel değer üretilemedi"
}

func marginEvidence(input Input) string {
	v := input.Professional.ValueInvesting
	if !v.MarginOfSafety.Computed {
		return fmt.Sprintf("hesaplanamadı | gereken %.1f%%", v.MarginOfSafety.RequiredPct)
	}
	return fmt.Sprintf("%.1f%% | gereken %.1f%%", v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct)
}

func valueInvestingGateRequiredMarginAction(input Input) string {
	v := input.Professional.ValueInvesting
	if v.IntrinsicValue.Computed && v.MarginOfSafety.RequiredPct > 0 {
		maxBuyPrice := v.IntrinsicValue.Base * (1 - v.MarginOfSafety.RequiredPct/100)
		return fmt.Sprintf("Değer yatırım alımı için fiyat %.2f %s veya altına inmeli ya da baz içsel değer %.2f %s üstüne yükselip %.1f%% güvenlik marjı oluşmalı", maxBuyPrice, input.Currency, v.CurrentPrice/(1-v.MarginOfSafety.RequiredPct/100), input.Currency, v.MarginOfSafety.RequiredPct)
	}
	return "fiyat/içsel değer farkı netleşene kadar değer yatırım kararı verme"
}

func tradingPlanEvidence(assetType string, daily Timeframe) string {
	if daily.TradePlan.Direction == "long" && !daily.TradePlan.Rejected {
		if !technicalGateAllowsAction(daily) {
			return fmt.Sprintf("izleme/paper-trade: giriş %.2f-%.2f | stop %.2f | hedef %.2f/%.2f; aktif işlem sinyali değildir", daily.TradePlan.EntryMin, daily.TradePlan.EntryMax, daily.TradePlan.StopLoss, daily.TradePlan.TakeProfit1, daily.TradePlan.TakeProfit2)
		}
		return fmt.Sprintf("giriş %.2f-%.2f | stop %.2f | hedef %.2f/%.2f", daily.TradePlan.EntryMin, daily.TradePlan.EntryMax, daily.TradePlan.StopLoss, daily.TradePlan.TakeProfit1, daily.TradePlan.TakeProfit2)
	}
	reason := strings.TrimSpace(daily.TradePlan.RejectReason)
	if reason == "" {
		reason = "aktif plan yok"
	}
	return readableTradeRejectReason(assetType, reason)
}

func boolScore(value bool) float64 {
	if value {
		return 100
	}
	return 0
}

func statusBool(value bool) string {
	if value {
		return "pass"
	}
	return "fail"
}

func statusForPositive(value float64) string {
	if value > 0 {
		return "pass"
	}
	return "fail"
}

func signedReturnScore(value, excellent float64) float64 {
	if value <= 0 {
		return 0
	}
	return mathutil.Clamp(value/excellent*100, 0, 100)
}

func drawdownQuality(value float64) float64 {
	dd := math.Abs(value)
	switch {
	case dd == 0:
		return 50
	case dd <= 0.10:
		return 100
	case dd <= 0.20:
		return 80
	case dd <= 0.30:
		return 55
	case dd <= 0.45:
		return 25
	default:
		return 0
	}
}

func firstOr(values []string, fallback string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return fallback
}

func institutionalStatusText(status string) string {
	switch status {
	case "pass":
		return "geçti"
	case "limited":
		return "sınırlı"
	case "not_applicable":
		return "uygulanmaz"
	default:
		return "geçmedi"
	}
}

func questions(input Input, report Report, daily Timeframe) []QuestionAnswer {
	pro := input.Professional
	v := pro.ValueInvesting
	qa := []QuestionAnswer{}
	add := func(q, a, status string, confidence float64, evidence ...string) {
		qa = append(qa, QuestionAnswer{Question: q, Answer: a, Status: status, Confidence: mathutil.Clamp(confidence, 0, 100), Evidence: evidence})
	}
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		add("Bu altın/emtia varlığı neyi temsil ediyor?", fmt.Sprintf("%s / %s. %s", empty(pro.Company.Sector, "Altın/emtia"), empty(pro.Company.Industry, "Spot altın"), empty(pro.Company.PeerGroup, "Makro benchmark grubu sınırlı")), "info", 80)
		add("Bugünkü fiyat teknik olarak avantajlı mı?", "Bu rapor şirket bilançosu veya içsel değer hesaplamaz; TradingView fiyat grafiği, trend, momentum, volatilite, likidite ve veri kapsamını okur.", "limited", report.ModelRisk.Score)
		add("Neden radarımıza girmeli?", report.TopOpportunity, "info", report.Score)
		add("Neden bugün almamalıyız?", report.TopRisk, riskStatus(report.TopRisk), 85)
		add("Hangi şart oluşursa alım düşünülür?", strings.Join(report.BuyConditions, "; "), "info", 80)
		add("Hangi şartta tez bozulur?", strings.Join(report.ExitConditions, "; "), "critical", 85)
		add("Altın veri kapsamı yeterli mi?", fmt.Sprintf("Kaynak kapsamı %.0f/100. Bu analiz doğruluğu değildir; sinyal sınırlamaları: %s.", report.ModelRisk.DataCoverage, joinOr(report.ModelRisk.PrimaryLimitations, "kritik sınırlama yok")), status(report.ModelRisk.DataCoverage), report.ModelRisk.DataCoverage)
		add("DXY, reel faiz ve vadeli pozisyon teyidi var mı?", fmt.Sprintf("%s. Bu kaynaklar bağlam sağlar; tek başına yön/hedef üretmez.", marketMacroEvidenceDetail(input, report)), status(report.ModelRisk.DataCoverage), report.ModelRisk.DataCoverage)
		add("Likidite büyük pozisyon için yeterli mi?", liquidityAnswer(report.Liquidity, input.Currency), status(report.Liquidity.Score), report.Liquidity.Score)
		add("Teknik zamanlama uygun mu?", technicalAnswer(daily), technicalStatus(daily), daily.Score)
		add("Teknik özet zaman dilimlerinde ne diyor?", timeframeTechnicalSummaryAnswer(input), "info", timeframeTechnicalSummaryConfidence(input))
		add("Teknik göstergeler ne söylüyor?", technicalIndicatorBreadthAnswer(daily), technicalIndicatorBreadthStatus(daily), technicalIndicatorBreadthConfidence(daily))
		add("Hareketli ortalamalar ne durumda?", movingAverageAnswer(daily), movingAverageStatus(daily), movingAverageConfidence(daily))
		add("RSI, MACD ve Stochastic ne söylüyor?", oscillatorFAQAnswer(daily), oscillatorFAQStatus(daily), technicalIndicatorBreadthConfidence(daily))
		add("Pivot noktaları hangi seviyelerde?", pivotAnswer(daily, input.Currency), pivotStatus(daily), pivotConfidence(daily))
		add("Makro/piyasa rejimi destekliyor mu?", marketMacroQuestionAnswer(input, report), status(report.Macro.Score), report.Macro.Score)
		add("Altın/emtia veri yönetişimi güvenilir mi?", report.Governance.DataLineage, status(report.Governance.Score), report.Governance.Score)
		add("En olası senaryo ne?", scenarioQuestionAnswer(input, report), scenarioQuestionStatus(report), scenarioQuestionConfidence(report))
		add("Bu rapora ne kadar güvenebilirim?", fmt.Sprintf("Model risk %.0f/100, veri kapsamı %.0f/100, backtest kalitesi %.0f/100. Ana sınırlamalar: %s.", report.ModelRisk.Score, report.ModelRisk.DataCoverage, report.ModelRisk.BacktestQuality, joinOr(report.ModelRisk.PrimaryLimitations, "kritik sınırlama yok")), status(report.ModelRisk.Score), report.ModelRisk.Score)
		add("Kurumsal portföy standardı rapor kalitesi yeterli mi?", report.InstitutionalViews.Portfolio.QualityAnswer, report.InstitutionalViews.Portfolio.ReportQualityStatus, report.InstitutionalViews.Portfolio.ReportQualityScore)
		add("Trading edge standardı rapor kalitesi yeterli mi?", report.InstitutionalViews.TradingEdge.QualityAnswer, report.InstitutionalViews.TradingEdge.ReportQualityStatus, report.InstitutionalViews.TradingEdge.ReportQualityScore)
		add("Kurumsal altın/emtia portföy çerçevesi ne söylüyor?", report.InstitutionalViews.Portfolio.FrameworkCommentary+" Net sonuç: "+report.InstitutionalViews.Portfolio.Takeaway, report.InstitutionalViews.Portfolio.ReportQualityStatus, report.InstitutionalViews.Portfolio.ReportQualityScore)
		add("Trading edge çerçevesi ne söylüyor?", report.InstitutionalViews.TradingEdge.FrameworkCommentary+" Net sonuç: "+report.InstitutionalViews.TradingEdge.Takeaway, report.InstitutionalViews.TradingEdge.ReportQualityStatus, report.InstitutionalViews.TradingEdge.ReportQualityScore)
		add("Altın/emtia aksiyon kapıları geçti mi?", report.InstitutionalViews.EliteCandidate.Summary, report.InstitutionalViews.EliteCandidate.Status, report.InstitutionalViews.EliteCandidate.Score)
		add("Bu rapor altın al/sat karar desteği için kullanılabilir mi?", report.InstitutionalViews.FinancialTransactionUse.Summary, report.InstitutionalViews.FinancialTransactionUse.Status, report.InstitutionalViews.EliteCandidate.Score)
		add("Kurumsal portföy standardı ön eleme için yeterli mi?", report.InstitutionalViews.Portfolio.OneLineAnswer, report.InstitutionalViews.Portfolio.Status, report.InstitutionalViews.Portfolio.Confidence)
		add("Trading edge standardı bu sinyali kullanır mı?", report.InstitutionalViews.TradingEdge.OneLineAnswer, report.InstitutionalViews.TradingEdge.Status, report.InstitutionalViews.TradingEdge.Confidence)
		return qa
	}
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		add("Bu kripto varlık neyi temsil ediyor?", fmt.Sprintf("%s / %s. %s", empty(pro.Company.Sector, "Kripto varlık"), empty(pro.Company.Industry, "Dijital varlık"), empty(pro.Company.PeerGroup, "Kripto peer grubu sınırlı")), "info", 80)
		add("Bugünkü fiyat teknik olarak avantajlı mı?", "Bu rapor geleneksel finansal tablo temelli içsel değer hesaplamaz; fiyat, trend, momentum, volatilite, likidite ve veri kapsamını okur.", "limited", report.ModelRisk.Score)
		add("Güvenlik marjı var mı?", marginAnswer(input), marginStatus(input), v.Confidence)
		add("Neden radarımıza girmeli?", report.TopOpportunity, "info", report.Score)
		add("Neden bugün almamalıyız?", report.TopRisk, riskStatus(report.TopRisk), 85)
		add("Hangi şart oluşursa alım düşünülür?", strings.Join(report.BuyConditions, "; "), "info", 80)
		add("Hangi şartta tez bozulur?", strings.Join(report.ExitConditions, "; "), "critical", 85)
		add("Kripto veri kapsamı yeterli mi?", fmt.Sprintf("Veri kapsamı %.0f/100. Eksik kaynaklar: %s.", report.ModelRisk.DataCoverage, joinOr(report.ModelRisk.PrimaryLimitations, "kritik sınırlama yok")), status(report.ModelRisk.DataCoverage), report.ModelRisk.DataCoverage)
		add("On-chain ve derivatives teyidi var mı?", "Bu raporda on-chain, funding/open interest, liquidation ve exchange-flow kaynakları bağlı değilse sinyal teknik fiyat okuması olarak kalır.", status(report.ModelRisk.DataCoverage), report.ModelRisk.DataCoverage)
		add("Likidite büyük pozisyon için yeterli mi?", liquidityAnswer(report.Liquidity, input.Currency), status(report.Liquidity.Score), report.Liquidity.Score)
		add("Teknik zamanlama uygun mu?", technicalAnswer(daily), technicalStatus(daily), daily.Score)
		add("Teknik özet zaman dilimlerinde ne diyor?", timeframeTechnicalSummaryAnswer(input), "info", timeframeTechnicalSummaryConfidence(input))
		add("Teknik göstergeler ne söylüyor?", technicalIndicatorBreadthAnswer(daily), technicalIndicatorBreadthStatus(daily), technicalIndicatorBreadthConfidence(daily))
		add("Hareketli ortalamalar ne durumda?", movingAverageAnswer(daily), movingAverageStatus(daily), movingAverageConfidence(daily))
		add("RSI, MACD ve Stochastic ne söylüyor?", oscillatorFAQAnswer(daily), oscillatorFAQStatus(daily), technicalIndicatorBreadthConfidence(daily))
		add("Pivot noktaları hangi seviyelerde?", pivotAnswer(daily, input.Currency), pivotStatus(daily), pivotConfidence(daily))
		add("Makro/piyasa rejimi destekliyor mu?", marketMacroQuestionAnswer(input, report), status(report.Macro.Score), report.Macro.Score)
		add("Kripto veri yönetişimi güvenilir mi?", report.Governance.DataLineage, status(report.Governance.Score), report.Governance.Score)
		add("En olası senaryo ne?", scenarioQuestionAnswer(input, report), scenarioQuestionStatus(report), scenarioQuestionConfidence(report))
		add("Bu rapora ne kadar güvenebilirim?", fmt.Sprintf("Model risk %.0f/100, veri kapsamı %.0f/100, backtest kalitesi %.0f/100. Ana sınırlamalar: %s.", report.ModelRisk.Score, report.ModelRisk.DataCoverage, report.ModelRisk.BacktestQuality, joinOr(report.ModelRisk.PrimaryLimitations, "kritik sınırlama yok")), status(report.ModelRisk.Score), report.ModelRisk.Score)
		add("Kurumsal portföy standardı rapor kalitesi yeterli mi?", report.InstitutionalViews.Portfolio.QualityAnswer, report.InstitutionalViews.Portfolio.ReportQualityStatus, report.InstitutionalViews.Portfolio.ReportQualityScore)
		add("Trading edge standardı rapor kalitesi yeterli mi?", report.InstitutionalViews.TradingEdge.QualityAnswer, report.InstitutionalViews.TradingEdge.ReportQualityStatus, report.InstitutionalViews.TradingEdge.ReportQualityScore)
		add("Kurumsal kripto portföy çerçevesi ne söylüyor?", report.InstitutionalViews.Portfolio.FrameworkCommentary+" Net sonuç: "+report.InstitutionalViews.Portfolio.Takeaway, report.InstitutionalViews.Portfolio.ReportQualityStatus, report.InstitutionalViews.Portfolio.ReportQualityScore)
		add("Trading edge çerçevesi ne söylüyor?", report.InstitutionalViews.TradingEdge.FrameworkCommentary+" Net sonuç: "+report.InstitutionalViews.TradingEdge.Takeaway, report.InstitutionalViews.TradingEdge.ReportQualityStatus, report.InstitutionalViews.TradingEdge.ReportQualityScore)
		add("Kripto aksiyon kapıları geçti mi?", report.InstitutionalViews.EliteCandidate.Summary, report.InstitutionalViews.EliteCandidate.Status, report.InstitutionalViews.EliteCandidate.Score)
		add("Bu rapor kripto al/sat karar desteği için kullanılabilir mi?", report.InstitutionalViews.FinancialTransactionUse.Summary, report.InstitutionalViews.FinancialTransactionUse.Status, report.InstitutionalViews.EliteCandidate.Score)
		add("Kurumsal portföy standardı ön eleme için yeterli mi?", report.InstitutionalViews.Portfolio.OneLineAnswer, report.InstitutionalViews.Portfolio.Status, report.InstitutionalViews.Portfolio.Confidence)
		add("Trading edge standardı bu sinyali kullanır mı?", report.InstitutionalViews.TradingEdge.OneLineAnswer, report.InstitutionalViews.TradingEdge.Status, report.InstitutionalViews.TradingEdge.Confidence)
		add("Trading edge standardı canlı sinyal için yeterli mi?", report.InstitutionalViews.TradingEdge.TransactionUseAnswer, report.InstitutionalViews.TradingEdge.TransactionUseStatus, report.InstitutionalViews.TradingEdge.Confidence)
		return qa
	}
	add("Bu şirket/varlık ne iş yapıyor?", fmt.Sprintf("%s / %s. %s", empty(pro.Company.Sector, "Sektör yok"), empty(pro.Company.Industry, "Endüstri yok"), empty(pro.Company.PeerGroup, "Peer grubu sınırlı")), "info", 80)
	if input.PriceVerification.Known {
		status := "pass"
		conf := 90.0
		answer := "Evet; resmi/final kapanış doğrulandı ve otomatik emir kapısı bu fiyat kanıtı açısından hazır."
		if !input.PriceVerification.ReadyForVerifiedClose && input.PriceVerification.ReadyForDecision {
			status = "warn"
			conf = 80
			answer = "Karar fiyatı kaynak mutabakatıyla kullanılabilir; ancak otomatik emir için resmi/final kapanış doğrulaması ayrı geçmedi."
		}
		if priceVerificationBlocksAction(input) {
			status = "fail"
			conf = 95
			answer = priceVerificationBlocker(input) + ". Bu nedenle günlük beklenti ve AL/SAT çıktısı ön izleme seviyesinde kalır."
		}
		add("Son kapanış karar ve otomatik emir için doğrulandı mı?", answer, status, conf)
	}
	valueConfidence := valueFAQConfidence(input, report)
	if v.Computed {
		add("Bugünkü fiyat gerçek değere göre ucuz mu?", fmt.Sprintf("Baz içsel değer %.2f %s, fiyat %.2f %s, güvenlik marjı %.1f%%.", v.IntrinsicValue.Base, input.Currency, v.CurrentPrice, input.Currency, v.MarginOfSafety.BasePct), statusForMargin(v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct), valueConfidence)
	} else {
		add("Bugünkü fiyat gerçek değere göre ucuz mu?", "Pozitif/güvenilir içsel değer üretilemedi; ucuz denemez.", "fail", valueConfidence)
	}
	add("Güvenlik marjı var mı?", marginAnswer(input), marginStatus(input), valueConfidence)
	add("Neden radarımıza girmeli?", report.TopOpportunity, "info", report.Score)
	add("Neden bugün almamalıyız?", report.TopRisk, riskStatus(report.TopRisk), 85)
	add("Hangi şart oluşursa alım düşünülür?", strings.Join(report.BuyConditions, "; "), "info", 80)
	add("Hangi şartta tez bozulur?", strings.Join(report.ExitConditions, "; "), "critical", 85)
	add("Finansal kalite nasıl?", fmt.Sprintf("Kalite %.0f/100: kârlılık %.0f, nakit dönüşümü %.0f, bilanço %.0f, sermaye politikası %.0f, moat %.0f.", report.Quality.Score, report.Quality.Profitability, report.Quality.CashConversion, report.Quality.BalanceSheet, report.Quality.CapitalPolicy, report.Quality.Moat), status(report.Quality.Score), report.Quality.Score)
	add("Nakit akışı kârı destekliyor mu?", cashAnswer(pro), cashStatus(pro), report.Quality.CashConversion)
	add("Borç riski ne durumda?", debtAnswer(pro), debtStatus(pro), report.Quality.BalanceSheet)
	add("Rakiplerine göre pahalı mı?", peerAnswer(pro), peerStatus(pro), 70)
	add("Adil değer aralığı ne söylüyor?", fairValueRangeAnswer(input), fairValueRangeStatus(input), fairValueRangeConfidence(input))
	add("Adil değer belirsizliği ne kadar?", fairValueUncertaintyAnswer(input), fairValueUncertaintyStatus(input), fairValueRangeConfidence(input))
	add("Adil değer kaç modelle destekleniyor?", fairValueModelCoverageAnswer(input), fairValueModelCoverageStatus(input), fairValueModelCoverageConfidence(input))
	add("Analist hedefleri veya model hedefleri var mı?", analystTargetAnswer(input), analystTargetStatus(input), analystTargetConfidence(input))
	add("Değerleme çarpanları ne söylüyor?", valuationMultiplesAnswer(pro), valuationMultiplesStatus(pro), valuationMultiplesConfidence(pro))
	add("52 haftalık fiyat aralığı ne söylüyor?", priceRangeAnswer(daily, input.Currency), priceRangeStatus(daily), priceRangeConfidence(daily))
	answer, dividendStatus, dividendConfidence, dividendEvidence := dividendDateAnswer(pro, input.Currency)
	add("Temettü tarihi ve ödeme bilgisi var mı?", answer, dividendStatus, dividendConfidence, dividendEvidence...)
	answer, dividendStatus, dividendConfidence, dividendEvidence = dividendFrequencyAnswer(pro)
	add("Temettü ne sıklıkla ödeniyor?", answer, dividendStatus, dividendConfidence, dividendEvidence...)
	answer, dividendStatus, dividendConfidence, dividendEvidence = dividendSafetyAnswer(pro, input.Currency)
	add("Temettü sürdürülebilir mi?", answer, dividendStatus, dividendConfidence, dividendEvidence...)
	add("Teknik zamanlama uygun mu?", technicalAnswer(daily), technicalStatus(daily), daily.Score)
	add("Teknik özet zaman dilimlerinde ne diyor?", timeframeTechnicalSummaryAnswer(input), "info", timeframeTechnicalSummaryConfidence(input))
	add("Teknik göstergeler ne söylüyor?", technicalIndicatorBreadthAnswer(daily), technicalIndicatorBreadthStatus(daily), technicalIndicatorBreadthConfidence(daily))
	add("Hareketli ortalamalar ne durumda?", movingAverageAnswer(daily), movingAverageStatus(daily), movingAverageConfidence(daily))
	add("RSI, MACD ve Stochastic ne söylüyor?", oscillatorFAQAnswer(daily), oscillatorFAQStatus(daily), technicalIndicatorBreadthConfidence(daily))
	add("Pivot noktaları hangi seviyelerde?", pivotAnswer(daily, input.Currency), pivotStatus(daily), pivotConfidence(daily))
	add("Büyük para girebilir mi?", liquidityAnswer(report.Liquidity, input.Currency), status(report.Liquidity.Score), report.Liquidity.Score)
	add("Makro/piyasa rejimi destekliyor mu?", fmt.Sprintf("%s. RS20 %.1f puan, beta %.2f.", report.Macro.Regime, report.Macro.RelativeStrength20, report.Macro.Beta60), status(report.Macro.Score), report.Macro.Score)
	add("Canlı BIST soket verisi raporda var mı?", liveMarketSocketAnswer(input), liveMarketSocketStatus(input), liveMarketSocketConfidence(input))
	if input.Professional.Market.GDP.Computed {
		if isBankInput(input) {
			answer := strings.TrimSpace(report.Macro.GDPInterpretation + " " + report.Macro.GDPImpact)
			if answer != "" {
				answer += " "
			}
			answer += "Bankada GSYH tek başına pozitif yatırım sinyali değildir; faiz, mevduat maliyeti, kredi büyümesi, NPL, karşılıklar, regülasyon ve sermaye yeterliliğiyle birlikte doğrulanmalıdır."
			add("GSYH hisseyi nasıl etkiliyor?", answer, "limited", math.Min(report.Macro.GDPScore, 55))
		} else {
			add("GSYH hisseyi nasıl etkiliyor?", report.Macro.GDPInterpretation+" "+report.Macro.GDPImpact, status(report.Macro.GDPScore), report.Macro.GDPScore)
		}
	} else if !ohlcv.IsCryptoAssetType(input.AssetType) {
		warning := input.Professional.Market.GDP.DataQualityWarning
		if strings.TrimSpace(warning) == "" {
			warning = "TÜİK CİP GSYH verisi rapora bağlı değil."
		}
		add("GSYH hisseyi nasıl etkiliyor?", warning+" Bu veri olmadan makro büyüme etkisi al/sat kararına dahil edilmez.", "limited", 35)
	}
	add("Yönetim/veri yönetişimi güvenilir mi?", fmt.Sprintf("%s. KAP kartı: %s, açıklama durumu: %s.", report.Governance.DataLineage, boolTR(report.Governance.KAPCard), report.Governance.Disclosure), status(report.Governance.Score), report.Governance.Score)
	add("En olası senaryo ne?", scenarioQuestionAnswer(input, report), scenarioQuestionStatus(report), scenarioQuestionConfidence(report))
	add("Bu rapora ne kadar güvenebilirim?", fmt.Sprintf("Model risk %.0f/100, veri kapsamı %.0f/100, backtest kalitesi %.0f/100. Ana sınırlamalar: %s.", report.ModelRisk.Score, report.ModelRisk.DataCoverage, report.ModelRisk.BacktestQuality, joinOr(report.ModelRisk.PrimaryLimitations, "kritik sınırlama yok")), status(report.ModelRisk.Score), report.ModelRisk.Score)
	add("Değer yatırım standardı rapor kalitesi yeterli mi?", report.InstitutionalViews.ValueInvesting.QualityAnswer, report.InstitutionalViews.ValueInvesting.ReportQualityStatus, report.InstitutionalViews.ValueInvesting.ReportQualityScore)
	add("Kurumsal portföy standardı rapor kalitesi yeterli mi?", report.InstitutionalViews.Portfolio.QualityAnswer, report.InstitutionalViews.Portfolio.ReportQualityStatus, report.InstitutionalViews.Portfolio.ReportQualityScore)
	add("Trading edge standardı rapor kalitesi yeterli mi?", report.InstitutionalViews.TradingEdge.QualityAnswer, report.InstitutionalViews.TradingEdge.ReportQualityStatus, report.InstitutionalViews.TradingEdge.ReportQualityScore)
	add("Değer yatırım çerçevesi ne söylüyor?", report.InstitutionalViews.ValueInvesting.FrameworkCommentary+" Net sonuç: "+report.InstitutionalViews.ValueInvesting.Takeaway, report.InstitutionalViews.ValueInvesting.ReportQualityStatus, report.InstitutionalViews.ValueInvesting.ReportQualityScore)
	add("Kurumsal portföy çerçevesi ne söylüyor?", report.InstitutionalViews.Portfolio.FrameworkCommentary+" Net sonuç: "+report.InstitutionalViews.Portfolio.Takeaway, report.InstitutionalViews.Portfolio.ReportQualityStatus, report.InstitutionalViews.Portfolio.ReportQualityScore)
	add("Trading edge çerçevesi ne söylüyor?", report.InstitutionalViews.TradingEdge.FrameworkCommentary+" Net sonuç: "+report.InstitutionalViews.TradingEdge.Takeaway, report.InstitutionalViews.TradingEdge.ReportQualityStatus, report.InstitutionalViews.TradingEdge.ReportQualityScore)
	add("Üç aksiyon kapısı geçti mi?", report.InstitutionalViews.EliteCandidate.Summary, report.InstitutionalViews.EliteCandidate.Status, report.InstitutionalViews.EliteCandidate.Score)
	add("Bu rapor finansal al/sat karar desteği için kullanılabilir mi?", report.InstitutionalViews.FinancialTransactionUse.Summary, report.InstitutionalViews.FinancialTransactionUse.Status, report.InstitutionalViews.EliteCandidate.Score)
	add("Değer yatırım standardı yatırım kararı için yeterli mi?", report.InstitutionalViews.ValueInvesting.OneLineAnswer, report.InstitutionalViews.ValueInvesting.Status, report.InstitutionalViews.ValueInvesting.Confidence)
	add("Değer yatırım standardı alım işlemi için yeterli mi?", report.InstitutionalViews.ValueInvesting.TransactionUseAnswer, report.InstitutionalViews.ValueInvesting.TransactionUseStatus, report.InstitutionalViews.ValueInvesting.Confidence)
	add("Kurumsal portföy standardı ön eleme için yeterli mi?", report.InstitutionalViews.Portfolio.OneLineAnswer, report.InstitutionalViews.Portfolio.Status, report.InstitutionalViews.Portfolio.Confidence)
	add("Kurumsal portföy standardı al/sat işlemi için yeterli mi?", report.InstitutionalViews.Portfolio.TransactionUseAnswer, report.InstitutionalViews.Portfolio.TransactionUseStatus, report.InstitutionalViews.Portfolio.Confidence)
	add("Trading edge standardı bu sinyali kullanır mı?", report.InstitutionalViews.TradingEdge.OneLineAnswer, report.InstitutionalViews.TradingEdge.Status, report.InstitutionalViews.TradingEdge.Confidence)
	add("Trading edge standardı canlı al/sat sinyali için yeterli mi?", report.InstitutionalViews.TradingEdge.TransactionUseAnswer, report.InstitutionalViews.TradingEdge.TransactionUseStatus, report.InstitutionalViews.TradingEdge.Confidence)
	return qa
}

func checks(input Input, report Report) []Check {
	if ohlcv.IsCryptoAssetType(input.AssetType) {
		return []Check{
			{"crypto_data_coverage", status(report.ModelRisk.DataCoverage), report.ModelRisk.DataCoverage, "OHLCV, teknik veri ve eksik on-chain/derivatives/exchange-flow kaynakları ölçüldü."},
			{"macro", status(report.Macro.Score), report.Macro.Score, "DXY/risk iştahı, on-chain, derivatives, exchange-flow ve haber/sentiment teyitleri okundu."},
			{"liquidity", status(report.Liquidity.Score), report.Liquidity.Score, "ADV, kapasite ve çıkış riski ölçüldü."},
			{"crypto_data_governance", status(report.Governance.Score), report.Governance.Score, "Kripto kaynak kapsamı ve veri soy ağacı kontrol edildi."},
			{"scenario", status(report.Scenario.Score), report.Scenario.Score, "Bear/base/bull ve teknik tez kırılım koşulları üretildi."},
			{"model_risk", status(report.ModelRisk.Score), report.ModelRisk.Score, "Veri kapsamı, backtest ve açıklanabilirlik ölçüldü."},
		}
	}
	if ohlcv.IsCommodityAssetType(input.AssetType) {
		return []Check{
			{"commodity_data_coverage", status(report.ModelRisk.DataCoverage), report.ModelRisk.DataCoverage, "OHLCV, teknik veri, DXY/reel faiz, vadeli pozisyon ve fon akışı kaynak kapsamı ölçüldü."},
			{"macro", status(report.Macro.Score), report.Macro.Score, "DXY/reel faiz, COT, ETF/fiziki akış ve haber/merkez bankası teyitleri okundu."},
			{"liquidity", status(report.Liquidity.Score), report.Liquidity.Score, "ADV, kapasite ve çıkış riski ölçüldü."},
			{"commodity_data_governance", status(report.Governance.Score), report.Governance.Score, "Altın/emtia kaynak kapsamı ve veri soy ağacı kontrol edildi."},
			{"scenario", status(report.Scenario.Score), report.Scenario.Score, "Bear/base/bull hedefleri yalnızca kanıt kapısı geçerse üretilir; mevcut sinyalde hedef üretilmez."},
			{"model_risk", status(report.ModelRisk.Score), report.ModelRisk.Score, "Veri kapsamı, backtest ve açıklanabilirlik ölçüldü."},
		}
	}
	checks := []Check{
		{"quality", status(report.Quality.Score), report.Quality.Score, "Finansal kalite, nakit dönüşümü, bilanço ve moat birlikte ölçüldü."},
		{"macro", status(report.Macro.Score), report.Macro.Score, "Piyasa rejimi ve benchmark göreli güç okundu."},
		{"liquidity", status(report.Liquidity.Score), report.Liquidity.Score, "ADV, kapasite ve çıkış riski ölçüldü."},
		{"governance", status(report.Governance.Score), report.Governance.Score, "KAP, açıklama ve veri soy ağacı kontrol edildi."},
		{"scenario", status(report.Scenario.Score), report.Scenario.Score, "Bear/base/bull ve tez kırılım koşulları üretildi."},
		{"model_risk", status(report.ModelRisk.Score), report.ModelRisk.Score, "Veri kapsamı, backtest ve açıklanabilirlik ölçüldü."},
	}
	if input.PriceVerification.Known {
		priceStatus := "pass"
		priceScore := 100.0
		priceMessage := "Resmi/final kapanış ve kaynak mutabakatı geçerli."
		if !input.PriceVerification.ReadyForVerifiedClose && input.PriceVerification.ReadyForDecision {
			priceStatus = "warn"
			priceScore = 70
			priceMessage = "Karar fiyatı geçerli; otomatik emir için resmi/final kapanış doğrulaması ayrı geçmedi."
		}
		if priceVerificationBlocksAction(input) {
			priceStatus = "fail"
			priceScore = 0
			priceMessage = priceVerificationBlocker(input)
		}
		checks = append([]Check{{"verified_price_close", priceStatus, priceScore, priceMessage}}, checks...)
	}
	return checks
}

func warnings(report Report) []string {
	out := []string{}
	if report.Liquidity.Score < 45 {
		out = append(out, "liquidity_limited")
	}
	if report.ModelRisk.Score < 60 {
		out = append(out, "model_risk_limited")
	}
	if report.Quality.Score < 45 {
		out = append(out, "quality_limited")
	}
	return out
}

func priceRangeAnswer(daily Timeframe, currency string) string {
	r := daily.Range52W
	if !priceRangeComputed(r) {
		return "52 haftalık fiyat aralığı hesaplanamadı; günlük OHLCV kapsamı yetersiz."
	}
	return fmt.Sprintf("%s aralığı %.2f-%.2f %s; son fiyat %.2f %s ve aralığın %.0f%% diliminde (%s).",
		empty(r.Label, "52 hafta"),
		r.Low,
		r.High,
		currency,
		r.Current,
		currency,
		r.PositionPct,
		priceRangePositionLabel(r.PositionPct),
	)
}

func priceRangeStatus(daily Timeframe) string {
	if !priceRangeComputed(daily.Range52W) {
		return "fail"
	}
	if daily.Range52W.SampleSize < 120 {
		return "limited"
	}
	return "info"
}

func priceRangeConfidence(daily Timeframe) float64 {
	if !priceRangeComputed(daily.Range52W) {
		return 25
	}
	return mathutil.Clamp(float64(daily.Range52W.SampleSize)/252*85, 35, 85)
}

func priceRangeComputed(r PriceRange) bool {
	return r.Low > 0 && r.High > 0 && r.Current > 0 && r.High >= r.Low
}

func priceRangePositionLabel(position float64) string {
	switch {
	case position <= 15:
		return "dip bölgesine yakın"
	case position <= 35:
		return "alt banda yakın"
	case position < 65:
		return "orta bantta"
	case position < 85:
		return "üst banda yakın"
	default:
		return "tepe bölgesine yakın"
	}
}

func fairValueRangeAnswer(input Input) string {
	bear, base, bull, confidence, source, ok := fairValueRangeParts(input)
	if !ok {
		return "Adil değer aralığı hesaplanamadı; içsel değer, DCF veya peer hedefi için yeterli güvenilir veri yok."
	}
	current := input.Professional.ValueInvesting.CurrentPrice
	if current <= 0 {
		current = input.primaryTimeframe().LastClose
	}
	gap := 0.0
	if current > 0 && base > 0 {
		gap = (base - current) / current * 100
	}
	return fmt.Sprintf("%s aralığı bear %.2f, base %.2f, bull %.2f %s. Son fiyata göre baz potansiyel %.1f%%; model güveni %.0f/100.",
		source,
		bear,
		base,
		bull,
		input.Currency,
		gap,
		confidence,
	)
}

func fairValueRangeStatus(input Input) string {
	v := input.Professional.ValueInvesting
	if v.FairValue.Computed {
		switch v.FairValue.Status {
		case "undervalued_with_margin":
			return "pass"
		case "undervalued_limited_margin", "fair_value_band":
			return "limited"
		case "overvalued":
			return "fail"
		}
	}
	_, base, _, confidence, _, ok := fairValueRangeParts(input)
	if !ok || base <= 0 {
		return "fail"
	}
	if confidence >= 70 {
		return "pass"
	}
	return "limited"
}

func fairValueRangeConfidence(input Input) float64 {
	_, _, _, confidence, _, ok := fairValueRangeParts(input)
	if !ok {
		return 25
	}
	return mathutil.Clamp(confidence, 0, 100)
}

func fairValueUncertaintyAnswer(input Input) string {
	bear, base, bull, confidence, source, ok := fairValueRangeParts(input)
	if !ok || base <= 0 {
		return "Belirsizlik ölçülemedi; adil değer aralığı üretilemedi."
	}
	spread := mathutil.SafeDiv(bull-bear, base) * 100
	return fmt.Sprintf("%s için değer bandı genişliği %.1f%% ve güven %.0f/100. Bant genişliği yüksekse hedef fiyat tek sayı olarak değil aralık ve şartlarla okunmalı.", source, spread, confidence)
}

func fairValueUncertaintyStatus(input Input) string {
	bear, base, bull, confidence, _, ok := fairValueRangeParts(input)
	if !ok || base <= 0 {
		return "fail"
	}
	spread := mathutil.SafeDiv(bull-bear, base) * 100
	if confidence >= 70 && spread <= 45 {
		return "pass"
	}
	if confidence >= 40 || spread <= 70 {
		return "limited"
	}
	return "fail"
}

func fairValueRangeParts(input Input) (bear, base, bull, confidence float64, source string, ok bool) {
	pro := input.Professional
	v := pro.ValueInvesting
	if v.FairValue.Computed || v.FairValue.FairValueBase > 0 {
		return v.FairValue.FairValueBear, v.FairValue.FairValueBase, v.FairValue.FairValueBull, valueConfidence(v.Confidence), "içsel değer", true
	}
	if pro.Valuation.FairValue.Base > 0 {
		return pro.Valuation.FairValue.Bear, pro.Valuation.FairValue.Base, pro.Valuation.FairValue.Bull, valueConfidence(pro.Valuation.FairValue.Confidence), "peer/model adil değer", true
	}
	if pro.Valuation.DCF.Computed && pro.Valuation.DCF.FairValuePerShare > 0 {
		bear = pro.Valuation.DCF.FairValuePerShare
		base = pro.Valuation.DCF.FairValuePerShare
		bull = pro.Valuation.DCF.FairValuePerShare
		for _, scenario := range pro.Valuation.DCF.Sensitivity {
			switch strings.ToLower(scenario.Name) {
			case "bear":
				bear = scenario.FairValuePerShare
			case "base":
				base = scenario.FairValuePerShare
			case "bull":
				bull = scenario.FairValuePerShare
			}
		}
		return bear, base, bull, 55, "DCF", true
	}
	return 0, 0, 0, 0, "", false
}

func fairValueModelCoverageAnswer(input Input) string {
	models := valuationModelNames(input.Professional)
	if len(models) == 0 {
		return "Adil değer modeli çalışmadı; finansal tablo, DCF, peer, senaryo veya KAP kanıtı yeterli değil."
	}
	suffix := " Harici analist konsensüs hedefleri ayrı kaynak olarak bağlı değilse bu sayı analist modeli değil, rapor içi model/girdi kapsamıdır."
	return fmt.Sprintf("%d model/girdi bağlı: %s.%s", len(models), strings.Join(models, "; "), suffix)
}

func fairValueModelCoverageStatus(input Input) string {
	count := len(valuationModelNames(input.Professional))
	switch {
	case count >= 4:
		return "pass"
	case count > 0:
		return "limited"
	default:
		return "fail"
	}
}

func fairValueModelCoverageConfidence(input Input) float64 {
	count := len(valuationModelNames(input.Professional))
	return mathutil.Clamp(float64(count)*18+20, 20, 90)
}

func valuationModelNames(pro professional.Report) []string {
	models := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range models {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		models = append(models, value)
	}
	if pro.ValueInvesting.IntrinsicValue.Computed {
		add(empty(pro.ValueInvesting.IntrinsicValue.Method, empty(pro.ValueInvesting.SectorModel.Label, "içsel değer modeli")))
	}
	if pro.Valuation.DCF.Computed {
		add("DCF")
	}
	if len(pro.Valuation.FairValue.Drivers) > 0 {
		add("peer çarpanlarından adil değer")
	}
	if pro.Peers.PeerCount > 0 {
		add(fmt.Sprintf("peer evreni (%d şirket)", pro.Peers.PeerCount))
	}
	if len(pro.Scenarios) > 0 {
		add("bear/base/bull senaryo")
	}
	if pro.KAPAssetInventory.Computed {
		if isBankProfessional(pro) {
			add("KAP varlık/envanter kanıtı (referans)")
		} else {
			add("KAP varlık/envanter kanıtı")
		}
	}
	if pro.ValueInvesting.DocumentEvidence.Computed {
		add("KAP doküman kanıtı")
	}
	return models
}

func analystTargetAnswer(input Input) string {
	pro := input.Professional
	parts := []string{}
	isBank := isBankProfessional(pro)
	if len(pro.Scenarios) > 0 {
		scenarios := []string{}
		for _, scenario := range pro.Scenarios {
			if scenario.PriceTarget > 0 {
				scenarios = append(scenarios, fmt.Sprintf("%s %.2f %s (%+.1f%%)", scenario.Name, scenario.PriceTarget, input.Currency, scenario.ReturnPct))
			}
		}
		if len(scenarios) > 0 {
			if isBank {
				parts = append(parts, "resmi içsel değer senaryoları: "+strings.Join(scenarios, ", "))
			} else {
				parts = append(parts, "rapor senaryoları: "+strings.Join(scenarios, ", "))
			}
		}
	}
	if pro.Valuation.FairValue.Base > 0 {
		if isBank {
			parts = append(parts, fmt.Sprintf("peer/model çarpan kontrol değeri %.2f %s", pro.Valuation.FairValue.Base, input.Currency))
		} else {
			parts = append(parts, fmt.Sprintf("peer/model baz hedef %.2f %s", pro.Valuation.FairValue.Base, input.Currency))
		}
	}
	if pro.Valuation.DCF.Computed && pro.Valuation.DCF.FairValuePerShare > 0 {
		parts = append(parts, fmt.Sprintf("DCF adil değer %.2f %s", pro.Valuation.DCF.FairValuePerShare, input.Currency))
	}
	if len(parts) == 0 {
		return "Harici analist hedefleri rapora bağlı değil; rapor içi model hedefi de güvenilir üretilemedi."
	}
	if isBank {
		return strings.Join(parts, "; ") + ". Banka raporunda resmi senaryo seti içsel değer katmanıdır; peer/model çarpan kontrol değeri analist hedefi veya otomatik al tavsiyesi değildir."
	}
	return strings.Join(parts, "; ") + ". Harici analist konsensüsü ayrı veri kaynağı olarak bağlı değilse bu hedefler analist tavsiyesi sayılmaz."
}

func analystTargetStatus(input Input) string {
	if len(input.Professional.Scenarios) == 0 && input.Professional.Valuation.FairValue.Base <= 0 && !input.Professional.Valuation.DCF.Computed {
		return "fail"
	}
	if fairValueRangeConfidence(input) >= 70 {
		return "pass"
	}
	return "limited"
}

func analystTargetConfidence(input Input) float64 {
	base := fairValueRangeConfidence(input)
	if len(input.Professional.Scenarios) > 0 {
		base += 10
	}
	return mathutil.Clamp(base, 25, 85)
}

func valuationMultiplesAnswer(pro professional.Report) string {
	if isBankProfessional(pro) {
		parts := []string{}
		for _, item := range []struct {
			label string
			key   string
		}{
			{"F/K", "PE"},
			{"PD/DD", "PB"},
			{"ROE", "ROE"},
			{"ROA", "ROA"},
		} {
			parts = append(parts, bankValuationMultipleText(pro, item.label, item.key))
		}
		return strings.Join(parts, "; ") + ". Bankada F/S, FD/Satış, FD/FAVÖK, FD/FVÖK ve net borç çarpanları ana karar girdisi değildir. Peer sinyali: " + peerAnswer(pro)
	}
	parts := []string{}
	for _, item := range []struct {
		label string
		key   string
	}{
		{"F/S", "PS"},
		{"FD/Satış", "EV_Sales"},
		{"F/K", "PE"},
		{"FD/FAVÖK", "EV_EBITDA"},
		{"FD/FVÖK", "EV_EBIT"},
		{"PD/DD", "PB"},
	} {
		parts = append(parts, valuationMultipleText(pro, item.label, item.key))
	}
	if len(parts) == 0 {
		return "Değerleme çarpanları hesaplanamadı; piyasa değeri, satış, kâr, FAVÖK/FVÖK veya özkaynak girdileri eksik."
	}
	return strings.Join(parts, "; ") + ". Peer sinyali: " + peerAnswer(pro)
}

func bankValuationMultipleText(pro professional.Report, label, key string) string {
	value := pro.Valuation.Ratios[key]
	if value <= 0 {
		value = pro.Valuation.SectorMetrics[key]
	}
	if value <= 0 {
		return label + " hesaplanamadı"
	}
	switch key {
	case "ROE", "ROA":
		return fmt.Sprintf("%s %.1f%%", label, value*100)
	case "BookPerShare", "EPS":
		return fmt.Sprintf("%s %.2f", label, value)
	}
	if median := pro.Peers.Medians[key]; median > 0 {
		diff := (value/median - 1) * 100
		return fmt.Sprintf("%s %.2fx (peer medyan %.2fx, %+.1f%%)", label, value, median, diff)
	}
	return fmt.Sprintf("%s %.2fx", label, value)
}

func valuationMultipleText(pro professional.Report, label, key string) string {
	value := pro.Valuation.Ratios[key]
	if value <= 0 {
		return label + " hesaplanamadı"
	}
	if median := pro.Peers.Medians[key]; median > 0 {
		diff := (value/median - 1) * 100
		return fmt.Sprintf("%s %.2fx (peer medyan %.2fx, %+.1f%%)", label, value, median, diff)
	}
	return fmt.Sprintf("%s %.2fx", label, value)
}

func valuationMultiplesStatus(pro professional.Report) string {
	return peerStatus(pro)
}

func valuationMultiplesConfidence(pro professional.Report) float64 {
	if isBankProfessional(pro) {
		count := 0
		for _, key := range []string{"PE", "PB", "ROE", "ROA", "BookPerShare", "EPS"} {
			if pro.Valuation.Ratios[key] > 0 || pro.Valuation.SectorMetrics[key] > 0 {
				count++
			}
		}
		return mathutil.Clamp(float64(count)*13+10, 25, 90)
	}
	count := 0
	for _, key := range []string{"PS", "EV_Sales", "PE", "EV_EBITDA", "EV_EBIT", "PB"} {
		if pro.Valuation.Ratios[key] > 0 {
			count++
		}
	}
	return mathutil.Clamp(float64(count)*12+20, 25, 85)
}

type dividendEventSummary struct {
	Title        string
	DocumentDate string
	Period       string
	Amount       float64
	Currency     string
	Snippet      string
	Confidence   float64
	Status       string
	Review       bool
}

func dividendDateAnswer(pro professional.Report, currency string) (string, string, float64, []string) {
	if event, ok := latestDividendEvent(pro); ok {
		date := firstNonEmptyString(event.DocumentDate, event.Period, "tarih/dönem çıkarılamadı")
		amount := ""
		if event.Amount > 0 {
			amountCurrency := empty(event.Currency, currency)
			amount = fmt.Sprintf(", tutar %s %s", money(event.Amount), amountCurrency)
		}
		review := ""
		status := "info"
		if event.Review || event.Status == "review_required" || event.Status == "candidate" {
			review = " Kayıt review/candidate statüsünde olduğu için temettüsüz işlem tarihi, ödeme tarihi ve verim kesinleşmeden yatırım kararına taşınmaz."
			status = "limited"
		}
		answer := fmt.Sprintf("KAP/PDF kurumsal olaylarında temettü kaydı var: %s. Tarih/dönem: %s%s.%s Temettüsüz işlem tarihi, ödeme tarihi ve temettü verimi ayrı yapılandırılmış alan olarak bağlı değilse bu cevap belge satırıyla sınırlıdır.", empty(event.Title, "temettü olayı"), date, amount, review)
		return answer, status, mathutil.Clamp(valueConfidence(event.Confidence), 35, 85), evidenceFromSnippet(event.Snippet)
	}
	capital := pro.ValueInvesting.CapitalAllocation
	if capital.Dividends10Y > 0 {
		answer := fmt.Sprintf("Kesin temettüsüz işlem/ödeme tarihi rapora bağlı değil. Finansal tablolarda son 10 yılda toplam temettü %s %s ve 10Y dağıtım oranı %.1f%% görünüyor.", money(capital.Dividends10Y), currency, capital.DividendPayoutRatio10Y*100)
		return answer, "limited", mathutil.Clamp(pro.ValueInvesting.Confidence, 35, 75), nil
	}
	return "Temettüsüz işlem tarihi, ödeme tarihi ve temettü verimi rapora bağlı değil; finansal tablolardan sürdürülebilir temettü akışı da teyit edilemiyor.", "fail", 30, nil
}

func dividendFrequencyAnswer(pro professional.Report) (string, string, float64, []string) {
	paidYears, totalYears := dividendPaidYearStats(pro)
	corpPaidYears, corpTotalYears, corpEvidence := corporateDividendYearStats(pro)
	if paidYears == 0 && corpPaidYears > 0 {
		cadence := "temettü geçmişi var; ödeme sıklığı serisi tamamlanmadı"
		if corpPaidYears >= 3 {
			cadence = "yıllık eğilime aday; yapılandırılmış seri tamamlanmalı"
		}
		answer := fmt.Sprintf("KAP/PDF kurumsal olaylarında %d farklı yılda temettü kaydı/adayı bulundu; geçmiş frekans %s. 1M/3M/6M/12M frekans kodu ayrı temettü takvimi verisi olarak bağlı değil.", corpPaidYears, cadence)
		confidence := mathutil.Clamp(float64(corpPaidYears)*15+30, 35, 75)
		if corpTotalYears > 0 {
			confidence = mathutil.Clamp(mathutil.SafeDiv(float64(corpPaidYears), float64(corpTotalYears))*70+20, 35, 80)
		}
		return answer, "limited", confidence, evidenceFromSnippet(corpEvidence)
	}
	if paidYears > 0 && totalYears > 0 {
		ratio := mathutil.SafeDiv(float64(paidYears), float64(totalYears))
		cadence := "düzensiz"
		switch {
		case ratio >= 0.8:
			cadence = "düzenli/yıllık eğilime yakın"
		case ratio >= 0.45:
			cadence = "aralıklı"
		}
		status := "limited"
		if ratio >= 0.8 {
			status = "pass"
		}
		answer := fmt.Sprintf("Son %d finansal yılın %d yılında temettü ödemesi görünüyor; geçmiş frekans %s. 1M/3M/6M/12M frekans kodu ayrı temettü takvimi verisi olarak bağlı değil.", totalYears, paidYears, cadence)
		return answer, status, mathutil.Clamp(ratio*80+10, 35, 90), nil
	}
	if event, ok := latestDividendEvent(pro); ok {
		answer := "KAP/PDF içinde temettü olayı bulundu, ancak geçmiş ödeme sıklığı serisi çıkarılmadı. Son kanıt: " + empty(event.Title, "temettü olayı")
		return answer, "limited", mathutil.Clamp(valueConfidence(event.Confidence), 35, 75), evidenceFromSnippet(event.Snippet)
	}
	return "Temettü ödeme sıklığı hesaplanamadı; geçmiş temettü serisi veya yapılandırılmış temettü takvimi rapora bağlı değil.", "fail", 25, nil
}

func dividendSafetyAnswer(pro professional.Report, currency string) (string, string, float64, []string) {
	capital := pro.ValueInvesting.CapitalAllocation
	paidYears, totalYears := dividendPaidYearStats(pro)
	if capital.Dividends10Y <= 0 && paidYears == 0 {
		if event, ok := latestDividendEvent(pro); ok {
			answer := "Temettü güvenliği ölçülemedi; ancak KAP/PDF kurumsal olaylarında temettü kaydı/adayı var. Finansal tablodaki temettü nakit akışı boş olduğu için bu durum 'temettü yok/0' sayılmaz; payout ratio, ödeme tarihi, brüt/net temettü ve son 5/10 yıl temettü serisi ayrı bağlanmalıdır."
			return answer, "limited", mathutil.Clamp(valueConfidence(event.Confidence), 35, 70), evidenceFromSnippet(event.Snippet)
		}
		return "Temettü güvenliği ölçülemedi; son yıllarda finansal tablolarda anlamlı nakit temettü akışı görünmüyor.", "fail", 30, nil
	}
	payout := capital.DividendPayoutRatio10Y
	answer := fmt.Sprintf("10Y toplam temettü %s %s, 10Y dağıtım oranı %.1f%%, TTM FCF %s %s. Son %d/%d yılda ödeme görünüyor.",
		money(capital.Dividends10Y),
		currency,
		payout*100,
		money(pro.Valuation.FreeCashFlowTTM),
		currency,
		paidYears,
		totalYears,
	)
	status := "limited"
	switch {
	case payout > 0 && payout <= 0.75 && pro.Valuation.FreeCashFlowTTM > 0:
		status = "pass"
	case payout > 1 || pro.Valuation.FreeCashFlowTTM <= 0:
		status = "fail"
	}
	confidence := mathutil.Clamp(pro.ValueInvesting.Confidence*0.6+pro.ValueInvesting.CapitalAllocation.Score*0.4, 30, 90)
	return answer, status, confidence, nil
}

func latestDividendEvent(pro professional.Report) (dividendEventSummary, bool) {
	if event, ok := latestCorporateDividendEvent(pro); ok {
		return event, true
	}
	if pro.RawKAPData == nil {
		return dividendEventSummary{}, false
	}
	var best dividendEventSummary
	bestKey := ""
	found := false
	for _, event := range pro.RawKAPData.CorporateEvents {
		text := strings.ToLower(event.EventType + " " + event.Title)
		if !shareholderDividendText(text) {
			continue
		}
		key := firstNonEmptyString(stringPtrValue(event.DocumentDate), stringPtrValue(event.Period), event.CreatedAt, event.ID)
		if found && key <= bestKey {
			continue
		}
		amount := 0.0
		if event.Amount != nil {
			amount = *event.Amount
		}
		best = dividendEventSummary{
			Title:        event.Title,
			DocumentDate: stringPtrValue(event.DocumentDate),
			Period:       stringPtrValue(event.Period),
			Amount:       amount,
			Currency:     event.Currency,
			Snippet:      event.Source.Snippet,
			Confidence:   event.Confidence,
		}
		bestKey = key
		found = true
	}
	return best, found
}

func latestCorporateDividendEvent(pro professional.Report) (dividendEventSummary, bool) {
	var best dividendEventSummary
	bestKey := ""
	found := false
	for _, action := range pro.CorporateActions.Actions {
		text := strings.ToLower(strings.Join([]string{action.Type, action.Title}, " "))
		if !shareholderDividendText(text) {
			continue
		}
		date := ""
		if action.EffectiveDate != nil {
			date = action.EffectiveDate.Format("2006-01-02")
		} else if action.AnnouncementDate != nil {
			date = action.AnnouncementDate.Format("2006-01-02")
		}
		dateWeight := "0"
		if date != "" {
			dateWeight = "1"
		}
		key := dateWeight + "|" + firstNonEmptyString(date, action.ID, action.Title)
		if found && key <= bestKey {
			continue
		}
		amount := 0.0
		if action.CashAmount != nil {
			amount = *action.CashAmount
		}
		snippet := firstNonEmptyString(action.Title, action.SourceFile, action.SourceDataset, action.Source)
		best = dividendEventSummary{
			Title:        action.Title,
			DocumentDate: date,
			Amount:       amount,
			Currency:     action.Currency,
			Snippet:      snippet,
			Confidence:   action.Confidence,
			Status:       action.Status,
			Review:       action.ReviewRequired,
		}
		bestKey = key
		found = true
	}
	return best, found
}

func corporateDividendYearStats(pro professional.Report) (paidYears int, totalYears int, evidence string) {
	years := map[int]bool{}
	for _, action := range pro.CorporateActions.Actions {
		text := strings.ToLower(strings.Join([]string{action.Type, action.Title}, " "))
		if !shareholderDividendText(text) {
			continue
		}
		year := 0
		if action.EffectiveDate != nil {
			year = action.EffectiveDate.Year()
		} else if action.AnnouncementDate != nil {
			year = action.AnnouncementDate.Year()
		}
		if year <= 0 {
			continue
		}
		years[year] = true
		if evidence == "" {
			evidence = firstNonEmptyString(action.Title, action.SourceFile, action.SourceDataset, action.Source)
		}
	}
	return len(years), len(years), evidence
}

func shareholderDividendText(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	includesDividend := strings.Contains(text, "dividend") || strings.Contains(text, "temett") || strings.Contains(text, "kar pay") || strings.Contains(text, "kâr pay")
	if !includesDividend {
		return false
	}
	for _, excluded := range []string{
		"çalışan",
		"calisan",
		"personel",
		"işçi",
		"isci",
		"toplu iş",
		"toplu is",
		"kar payı için ayrılan karşılık",
		"kâr payı için ayrılan karşılık",
		"kar payi icin ayrilan karsilik",
	} {
		if strings.Contains(text, excluded) {
			return false
		}
	}
	return true
}

func dividendPaidYearStats(pro professional.Report) (paidYears int, totalYears int) {
	for _, year := range pro.ValueInvesting.Years {
		if year.Year == 0 {
			continue
		}
		totalYears++
		if year.DividendsPaid > 0 {
			paidYears++
		}
	}
	return paidYears, totalYears
}

func evidenceFromSnippet(snippet string) []string {
	snippet = strings.TrimSpace(snippet)
	if snippet == "" {
		return nil
	}
	return []string{snippet}
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func valueConfidence(value float64) float64 {
	if value > 0 && value <= 1 {
		return value * 100
	}
	return value
}

func valueFAQConfidence(input Input, report Report) float64 {
	v := input.Professional.ValueInvesting
	if !v.Computed || !v.IntrinsicValue.Computed || !v.MarginOfSafety.Computed {
		return 25
	}
	if strings.EqualFold(input.Professional.EvidencePolicy.Status, "blocked") || !input.Professional.EvidencePolicy.ValuationTargetsAllowed {
		return math.Min(report.ModelRisk.Score, 35)
	}
	if isBankInput(input) && bankCoreMetricsMissing(input.Professional) {
		return math.Min(v.Confidence, 60)
	}
	return v.Confidence
}

func marginAnswer(input Input) string {
	v := input.Professional.ValueInvesting
	if isMarketOnlyAssetType(input.AssetType) {
		return marketAssetLabel(input.AssetType) + " için bu rapor güvenlik marjını geleneksel finansal tablo temelli içsel değer modeliyle hesaplamaz."
	}
	if !v.MarginOfSafety.Computed {
		return "Güvenlik marjı hesaplanamadı."
	}
	if v.MarginOfSafety.BasePct >= v.MarginOfSafety.RequiredPct {
		return fmt.Sprintf("Evet; %.1f%% marj var, gereken %.1f%%.", v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct)
	}
	return fmt.Sprintf("Hayır; %.1f%% marj var, gereken %.1f%%.", v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct)
}

func marginStatus(input Input) string {
	v := input.Professional.ValueInvesting
	if isMarketOnlyAssetType(input.AssetType) {
		return "not_applicable"
	}
	if !v.MarginOfSafety.Computed {
		return "fail"
	}
	return statusForMargin(v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct)
}

func statusForMargin(margin, required float64) string {
	if margin >= required {
		return "pass"
	}
	if margin >= 0 {
		return "limited"
	}
	return "fail"
}

func cashAnswer(pro professional.Report) string {
	if isBankProfessional(pro) {
		parts := []string{"Bankada klasik serbest nakit akımı dönüşüm oranı uygulanmaz; kâr kalitesi net faiz geliri, net ücret-komisyon geliri, ticari kâr/zarar, karşılık giderleri, takipteki krediler, NIM ve sermaye yeterliliğiyle ölçülür"}
		if pro.Valuation.NetIncomeTTM > 0 {
			parts = append(parts, fmt.Sprintf("TTM net kâr %s", money(pro.Valuation.NetIncomeTTM)))
		}
		parts = append(parts, "SYR, CET1, NPL, NIM ve LCR yapılandırılmış veri olarak tamamlanmadan bu cevap sınırlıdır")
		return strings.Join(parts, ". ") + "."
	}
	if pro.Valuation.NetIncomeTTM <= 0 && pro.Valuation.FreeCashFlowTTM <= 0 {
		return "Net kâr ve FCF pozitif değil; kâr kalitesi teyitsiz."
	}
	if pro.Valuation.NetIncomeTTM > 0 {
		ratio := pro.Valuation.FreeCashFlowTTM / pro.Valuation.NetIncomeTTM
		if ratio < 0.40 {
			return fmt.Sprintf("Muhasebe kârlılığı pozitif görünüyor; fakat FCF/net kâr oranı %.2f olduğu için nakde dönüşüm zayıf. FCF %s, net kâr %s.", ratio, money(pro.Valuation.FreeCashFlowTTM), money(pro.Valuation.NetIncomeTTM))
		}
		return fmt.Sprintf("FCF/net kâr oranı %.2f. FCF %s, net kâr %s.", ratio, money(pro.Valuation.FreeCashFlowTTM), money(pro.Valuation.NetIncomeTTM))
	}
	return "FCF pozitif fakat net kâr bazında kalite oranı hesaplanamıyor."
}

func cashStatus(pro professional.Report) string {
	if isBankProfessional(pro) {
		if pro.Valuation.NetIncomeTTM > 0 {
			return "limited"
		}
		return "fail"
	}
	if pro.Valuation.FreeCashFlowTTM > 0 && pro.Valuation.NetIncomeTTM > 0 && pro.Valuation.FreeCashFlowTTM/pro.Valuation.NetIncomeTTM >= 0.7 {
		return "pass"
	}
	if pro.Valuation.FreeCashFlowTTM > 0 {
		return "limited"
	}
	return "fail"
}

func debtAnswer(pro professional.Report) string {
	if isBankProfessional(pro) {
		return "Bankada sanayi tipi net borç/özsermaye, EV ve FD çarpanları uygulanmaz. Fonlama ve bilanço riski kredi/mevduat, LCR, sermaye yeterliliği, mevduat maliyeti, yabancı para pozisyonu ve aktif kalitesiyle izlenmelidir."
	}
	nde := pro.Valuation.Ratios["NetDebt_Eq"]
	return fmt.Sprintf("Net borç/özsermaye %.2f, net borç %s.", nde, money(pro.Valuation.NetDebt))
}

func debtStatus(pro professional.Report) string {
	if isBankProfessional(pro) {
		return "not_applicable"
	}
	nde := pro.Valuation.Ratios["NetDebt_Eq"]
	if nde <= 0.5 {
		return "pass"
	}
	if nde <= 1 {
		return "limited"
	}
	return "fail"
}

func peerAnswer(pro professional.Report) string {
	switch strings.ToLower(pro.Peers.ValuationSignal) {
	case "discount":
		return fmt.Sprintf("Peer evrenine göre iskontolu görünüyor; peer sayısı %d.", pro.Peers.PeerCount)
	case "premium":
		return fmt.Sprintf("Peer evrenine göre primli/pahalı görünüyor; peer sayısı %d.", pro.Peers.PeerCount)
	case "neutral":
		return fmt.Sprintf("Peer evrenine göre nötr; peer sayısı %d.", pro.Peers.PeerCount)
	case "not_applicable":
		return "Bu varlıkta geleneksel peer çarpanları uygulanmaz."
	default:
		return fmt.Sprintf("Peer değerleme güveni sınırlı; peer sayısı %d.", pro.Peers.PeerCount)
	}
}

func peerStatus(pro professional.Report) string {
	switch strings.ToLower(pro.Peers.ValuationSignal) {
	case "discount":
		return "pass"
	case "neutral":
		return "limited"
	case "premium":
		return "fail"
	case "not_applicable":
		return "not_applicable"
	default:
		return "limited"
	}
}

func liveMarketSocketAnswer(input Input) string {
	snapshot := input.Professional.Market.LiveSnapshot
	if snapshot == nil || !snapshot.HasData() {
		return "Canlı BIST websocket snapshot rapora bağlı değil. Önce `sync market-ws` komutu ile data/market/live_snapshot.json ve data/market/ws datasetleri üretilirse endeks, sembol, izleyici ve socket veri kapsamı rapora dahil edilir."
	}
	parts := []string{}
	if snapshot.Source != "" {
		parts = append(parts, "kaynak "+snapshot.Source)
	}
	if snapshot.SourceHost != "" {
		parts = append(parts, "host "+snapshot.SourceHost)
	}
	if !snapshot.UpdatedAt.IsZero() {
		parts = append(parts, "timestamp "+snapshot.UpdatedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if snapshot.ActiveUsers > 0 {
		parts = append(parts, fmt.Sprintf("aktif kullanıcı %d", snapshot.ActiveUsers))
	}
	hasQuote := false
	if len(snapshot.Indices) > 0 {
		keys := make([]string, 0, len(snapshot.Indices))
		for key := range snapshot.Indices {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		indexParts := []string{}
		for _, key := range keys {
			quote := snapshot.Indices[key]
			if quote.Last > 0 {
				indexParts = append(indexParts, fmt.Sprintf("%s %.2f", key, quote.Last))
				hasQuote = true
			}
		}
		if len(indexParts) > 0 {
			parts = append(parts, "endeksler: "+strings.Join(limitStrings(indexParts, 5), ", "))
		}
	}
	symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
	if count := snapshot.ViewerCounts[symbol]; count > 0 {
		parts = append(parts, fmt.Sprintf("%s izleyici sayısı %d", symbol, count))
	}
	if item, ok := snapshot.Symbols[symbol]; ok && item.Last > 0 {
		parts = append(parts, fmt.Sprintf("%s canlı fiyat %.2f %s", symbol, item.Last, input.Currency))
		hasQuote = true
	}
	if snapshot.MQTTMessages > 0 {
		parts = append(parts, fmt.Sprintf("mqtt mesajı %d", snapshot.MQTTMessages))
	}
	if len(snapshot.Datasets) > 0 {
		parts = append(parts, "socket datasetleri: "+strings.Join(datasetNamesWithCounts(snapshot.Datasets, 6), ", "))
	}
	if len(snapshot.RequestCounts) > 0 {
		parts = append(parts, "gönderilen istekler: "+strings.Join(requestNamesWithCounts(snapshot.RequestCounts, 6), ", "))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("snapshot bağlı; %d endeks, %d sembol, %d izleyici kaydı var", len(snapshot.Indices), len(snapshot.Symbols), len(snapshot.ViewerCounts)))
	}
	if !hasQuote {
		return strings.Join(parts, "; ") + ". Bu kayıt socket/abonelik sağlığı kanıtıdır; sembol fiyatı, veri tazeliği ve gecikme doğrulanmadığı için gerçek zamanlı al/sat kanıtı sayılmaz."
	}
	return strings.Join(parts, "; ") + ". Bu canlı veri anlık piyasa bağlamıdır; tek başına al/sat kararı değildir."
}

func liveMarketSocketStatus(input Input) string {
	snapshot := input.Professional.Market.LiveSnapshot
	if snapshot == nil || !snapshot.HasData() {
		return "limited"
	}
	if len(snapshot.Indices) > 0 || len(snapshot.Symbols) > 0 {
		return "pass"
	}
	return "limited"
}

func liveMarketSocketConfidence(input Input) float64 {
	snapshot := input.Professional.Market.LiveSnapshot
	if snapshot == nil || !snapshot.HasData() {
		return 35
	}
	score := 35.0
	if snapshot.ActiveUsers > 0 {
		score += 10
	}
	score += math.Min(float64(len(snapshot.Indices))*6, 25)
	score += math.Min(float64(len(snapshot.Symbols))*0.5, 20)
	score += math.Min(float64(len(snapshot.ViewerCounts))*0.5, 10)
	score += math.Min(float64(len(snapshot.Datasets))*3, 15)
	return mathutil.Clamp(score, 35, 90)
}

func datasetNamesWithCounts(values map[string][]json.RawMessage, limit int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range limitStrings(keys, limit) {
		out = append(out, fmt.Sprintf("%s(%d)", key, len(values[key])))
	}
	return out
}

func requestNamesWithCounts(values map[string]int, limit int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range limitStrings(keys, limit) {
		out = append(out, fmt.Sprintf("%s(%d)", key, values[key]))
	}
	return out
}

type qaTechnicalSignal struct {
	Name     string
	Value    float64
	Label    string
	Vote     int
	Computed bool
}

func timeframeTechnicalSummaryAnswer(input Input) string {
	keys := orderedQuestionTimeframes(input.Timeframes)
	if len(keys) == 0 {
		return "Teknik zaman dilimi verisi yok; kısa, orta ve uzun vade ayrı ayrı okunamadı."
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		tf := input.Timeframes[key]
		parts = append(parts, fmt.Sprintf("%s %s (skor %.0f/100)", timeframeLabelTR(key), technicalTone(tf), tf.Score))
	}
	return strings.Join(limitStrings(parts, 10), "; ") + ". Kısa ve uzun vadeler ters yöndeyse rapor acele işlem yerine teyit bekler."
}

func timeframeTechnicalSummaryConfidence(input Input) float64 {
	return mathutil.Clamp(float64(len(input.Timeframes))*12+30, 30, 90)
}

func orderedQuestionTimeframes(timeframes map[string]Timeframe) []string {
	keys := make([]string, 0, len(timeframes))
	for key := range timeframes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		wi := timeframeOrderWeight(keys[i])
		wj := timeframeOrderWeight(keys[j])
		if wi == wj {
			return keys[i] < keys[j]
		}
		return wi < wj
	})
	return keys
}

func timeframeOrderWeight(key string) int {
	switch strings.ToUpper(strings.TrimSpace(key)) {
	case "1", "1MIN":
		return 10
	case "5", "5M", "5MIN":
		return 20
	case "15", "15M", "15MIN":
		return 30
	case "30", "30M", "30MIN":
		return 40
	case "60", "1H":
		return 50
	case "240", "4H":
		return 60
	case "300", "5H":
		return 70
	case "1D", "D":
		return 80
	case "1W", "W":
		return 90
	case "1M", "M":
		return 100
	default:
		return 500
	}
}

func timeframeLabelTR(key string) string {
	switch strings.ToUpper(strings.TrimSpace(key)) {
	case "1", "1MIN":
		return "1 dakika"
	case "5", "5M", "5MIN":
		return "5 dakika"
	case "15", "15M", "15MIN":
		return "15 dakika"
	case "30", "30M", "30MIN":
		return "30 dakika"
	case "60", "1H":
		return "Saatlik"
	case "240", "4H":
		return "4 saatlik"
	case "300", "5H":
		return "5 saatlik"
	case "1D", "D":
		return "Günlük"
	case "1W", "W":
		return "Haftalık"
	case "1M", "M":
		return "Aylık"
	default:
		return key
	}
}

func technicalTone(tf Timeframe) string {
	switch {
	case tf.Score >= 70:
		return "Güçlü Al"
	case tf.Score >= 56:
		return "Al"
	case tf.Score <= 30:
		return "Güçlü Sat"
	case tf.Score <= 44:
		return "Sat"
	default:
		if strings.EqualFold(tf.TrendBias, "bullish") {
			return "Al'a yakın"
		}
		if strings.EqualFold(tf.TrendBias, "bearish") {
			return "Sat'a yakın"
		}
		return "Nötr"
	}
}

func technicalIndicatorBreadthAnswer(daily Timeframe) string {
	signals := technicalIndicatorSignals(daily)
	buy, sell, neutral := technicalSignalCounts(signals)
	if buy+sell+neutral == 0 {
		return "Teknik gösterge verisi hesaplanamadı."
	}
	highlights := []string{}
	for _, signal := range signals {
		if signal.Computed {
			highlights = append(highlights, fmt.Sprintf("%s %.3f: %s", signal.Name, signal.Value, signal.Label))
		}
	}
	return fmt.Sprintf("%d olumlu/al, %d zayıf/sat, %d nötr veya uyarı sinyali var. Öne çıkanlar: %s. Aşırı alış/aşırı satış tek başına emir değil, sadece dikkat bölgesidir.",
		buy,
		sell,
		neutral,
		strings.Join(limitStrings(highlights, 8), "; "),
	)
}

func technicalIndicatorBreadthStatus(daily Timeframe) string {
	buy, sell, neutral := technicalSignalCounts(technicalIndicatorSignals(daily))
	if buy+sell+neutral == 0 {
		return "fail"
	}
	if buy >= sell+2 {
		return "pass"
	}
	if sell >= buy+2 {
		return "fail"
	}
	return "limited"
}

func technicalIndicatorBreadthConfidence(daily Timeframe) float64 {
	signals := technicalIndicatorSignals(daily)
	computed := 0
	for _, signal := range signals {
		if signal.Computed {
			computed++
		}
	}
	return mathutil.Clamp(float64(computed)*6+25, 25, 90)
}

func technicalSignalCounts(signals []qaTechnicalSignal) (buy, sell, neutral int) {
	for _, signal := range signals {
		if !signal.Computed {
			continue
		}
		switch {
		case signal.Vote > 0:
			buy++
		case signal.Vote < 0:
			sell++
		default:
			neutral++
		}
	}
	return buy, sell, neutral
}

func technicalIndicatorSignals(daily Timeframe) []qaTechnicalSignal {
	ind := daily.Indicators
	additional := ind.AdditionalIndicators
	signals := []qaTechnicalSignal{
		rsiTechnicalSignal(ind.RSI14),
		stochasticTechnicalSignal("STOCH(14,3)", ind.StochasticK, ind.StochasticD),
		stochasticTechnicalSignal("STOCHRSI(14)", ind.StochRSIK, ind.StochRSID),
		macdTechnicalSignal(ind.MACD, ind.MACDSignal, ind.MACDHistogram),
		adxTechnicalSignal(ind.ADX14),
		williamsTechnicalSignal(ind.WilliamsR14),
		cciTechnicalSignal(ind.CCI20),
		atrTechnicalSignal(ind.ATR14, daily.LastClose),
		signedTechnicalSignal("Highs/Lows(14)", additional["Highs/Lows(14)"]),
		ultimateOscillatorSignal(additional["Ultimate Oscillator"]),
		signedTechnicalSignal("ROC", ind.ROC12),
		bullBearPowerSignal(additional["Elder Ray Bull Power"], additional["Elder Ray Bear Power"]),
	}
	return signals
}

func rsiTechnicalSignal(value float64) qaTechnicalSignal {
	return boundedOscillatorSignal("RSI(14)", value, 30, 45, 55, 70)
}

func stochasticTechnicalSignal(name string, k, d float64) qaTechnicalSignal {
	if k <= 0 && d <= 0 {
		return qaTechnicalSignal{Name: name}
	}
	if k <= 20 {
		return qaTechnicalSignal{Name: name, Value: k, Label: "aşırı satış bölgesi", Vote: 0, Computed: true}
	}
	if k >= 80 {
		return qaTechnicalSignal{Name: name, Value: k, Label: "aşırı alış bölgesi", Vote: 0, Computed: true}
	}
	if d > 0 && k > d {
		return qaTechnicalSignal{Name: name, Value: k, Label: "olumlu", Vote: 1, Computed: true}
	}
	if d > 0 && k < d {
		return qaTechnicalSignal{Name: name, Value: k, Label: "zayıf", Vote: -1, Computed: true}
	}
	return qaTechnicalSignal{Name: name, Value: k, Label: "nötr", Computed: true}
}

func macdTechnicalSignal(macd, signal, histogram float64) qaTechnicalSignal {
	if macd == 0 && signal == 0 && histogram == 0 {
		return qaTechnicalSignal{Name: "MACD(12,26)"}
	}
	if histogram > 0 || macd > signal {
		return qaTechnicalSignal{Name: "MACD(12,26)", Value: macd, Label: "olumlu", Vote: 1, Computed: true}
	}
	if histogram < 0 || macd < signal {
		return qaTechnicalSignal{Name: "MACD(12,26)", Value: macd, Label: "zayıf", Vote: -1, Computed: true}
	}
	return qaTechnicalSignal{Name: "MACD(12,26)", Value: macd, Label: "nötr", Computed: true}
}

func adxTechnicalSignal(value float64) qaTechnicalSignal {
	if value <= 0 {
		return qaTechnicalSignal{Name: "ADX(14)"}
	}
	switch {
	case value >= 50:
		return qaTechnicalSignal{Name: "ADX(14)", Value: value, Label: "trend çok güçlü; yönü ayrıca kontrol et", Vote: 0, Computed: true}
	case value >= 25:
		return qaTechnicalSignal{Name: "ADX(14)", Value: value, Label: "trend belirgin", Vote: 0, Computed: true}
	default:
		return qaTechnicalSignal{Name: "ADX(14)", Value: value, Label: "trend zayıf", Vote: 0, Computed: true}
	}
}

func williamsTechnicalSignal(value float64) qaTechnicalSignal {
	if value == 0 {
		return qaTechnicalSignal{Name: "Williams %R"}
	}
	switch {
	case value <= -80:
		return qaTechnicalSignal{Name: "Williams %R", Value: value, Label: "aşırı satış bölgesi", Vote: 0, Computed: true}
	case value >= -20:
		return qaTechnicalSignal{Name: "Williams %R", Value: value, Label: "aşırı alış bölgesi", Vote: 0, Computed: true}
	case value > -50:
		return qaTechnicalSignal{Name: "Williams %R", Value: value, Label: "olumlu", Vote: 1, Computed: true}
	default:
		return qaTechnicalSignal{Name: "Williams %R", Value: value, Label: "zayıf", Vote: -1, Computed: true}
	}
}

func cciTechnicalSignal(value float64) qaTechnicalSignal {
	if value == 0 {
		return qaTechnicalSignal{Name: "CCI"}
	}
	switch {
	case value >= 100:
		return qaTechnicalSignal{Name: "CCI", Value: value, Label: "güçlü/ısınmış", Vote: 1, Computed: true}
	case value <= -100:
		return qaTechnicalSignal{Name: "CCI", Value: value, Label: "zayıf", Vote: -1, Computed: true}
	default:
		return qaTechnicalSignal{Name: "CCI", Value: value, Label: "nötr", Vote: 0, Computed: true}
	}
}

func atrTechnicalSignal(value, lastClose float64) qaTechnicalSignal {
	if value <= 0 {
		return qaTechnicalSignal{Name: "ATR(14)"}
	}
	ratio := mathutil.SafeDiv(value, lastClose)
	switch {
	case ratio >= 0.06:
		return qaTechnicalSignal{Name: "ATR(14)", Value: value, Label: "hareketlilik yüksek", Vote: 0, Computed: true}
	case ratio >= 0.03:
		return qaTechnicalSignal{Name: "ATR(14)", Value: value, Label: "hareketlilik orta", Vote: 0, Computed: true}
	default:
		return qaTechnicalSignal{Name: "ATR(14)", Value: value, Label: "hareketlilik düşük", Vote: 0, Computed: true}
	}
}

func ultimateOscillatorSignal(value float64) qaTechnicalSignal {
	return boundedOscillatorSignal("Ultimate Oscillator", value, 30, 45, 55, 70)
}

func boundedOscillatorSignal(name string, value, oversold, weak, strong, overbought float64) qaTechnicalSignal {
	if value <= 0 {
		return qaTechnicalSignal{Name: name}
	}
	switch {
	case value <= oversold:
		return qaTechnicalSignal{Name: name, Value: value, Label: "aşırı satış bölgesi", Vote: 0, Computed: true}
	case value >= overbought:
		return qaTechnicalSignal{Name: name, Value: value, Label: "aşırı alış bölgesi", Vote: 0, Computed: true}
	case value <= weak:
		return qaTechnicalSignal{Name: name, Value: value, Label: "zayıf", Vote: -1, Computed: true}
	case value >= strong:
		return qaTechnicalSignal{Name: name, Value: value, Label: "olumlu", Vote: 1, Computed: true}
	default:
		return qaTechnicalSignal{Name: name, Value: value, Label: "nötr", Vote: 0, Computed: true}
	}
}

func signedTechnicalSignal(name string, value float64) qaTechnicalSignal {
	if value > 0 {
		return qaTechnicalSignal{Name: name, Value: value, Label: "olumlu", Vote: 1, Computed: true}
	}
	if value < 0 {
		return qaTechnicalSignal{Name: name, Value: value, Label: "zayıf", Vote: -1, Computed: true}
	}
	return qaTechnicalSignal{Name: name, Value: value, Label: "nötr", Vote: 0, Computed: true}
}

func bullBearPowerSignal(bull, bear float64) qaTechnicalSignal {
	if bull == 0 && bear == 0 {
		return qaTechnicalSignal{Name: "Bull/Bear Power(13)"}
	}
	value := bull + bear
	if bull > math.Abs(bear) && bull > 0 {
		return qaTechnicalSignal{Name: "Bull/Bear Power(13)", Value: value, Label: "olumlu", Vote: 1, Computed: true}
	}
	if math.Abs(bear) >= bull && bear < 0 {
		return qaTechnicalSignal{Name: "Bull/Bear Power(13)", Value: value, Label: "zayıf", Vote: -1, Computed: true}
	}
	return qaTechnicalSignal{Name: "Bull/Bear Power(13)", Value: value, Label: "nötr", Vote: 0, Computed: true}
}

type qaMovingAverageSignal struct {
	Period   int
	Kind     string
	Value    float64
	Vote     int
	Computed bool
}

func movingAverageAnswer(daily Timeframe) string {
	signals := movingAverageSignals(daily)
	buy, sell, computed := movingAverageCounts(signals)
	if computed == 0 {
		return "Hareketli ortalama hesaplanamadı."
	}
	parts := []string{}
	for _, period := range []int{5, 10, 20, 50, 100, 200} {
		sma, smaOK := movingAverageValue(daily.Indicators, period, false)
		ema, emaOK := movingAverageValue(daily.Indicators, period, true)
		if !smaOK && !emaOK {
			continue
		}
		smaText := "-"
		emaText := "-"
		if smaOK {
			smaText = fmt.Sprintf("%.3f %s", sma, buySellLabel(daily.LastClose, sma))
		}
		if emaOK {
			emaText = fmt.Sprintf("%.3f %s", ema, buySellLabel(daily.LastClose, ema))
		}
		parts = append(parts, fmt.Sprintf("MA%d basit %s, üssel %s", period, smaText, emaText))
	}
	return fmt.Sprintf("Hareketli ortalamalarda %d al/olumlu ve %d sat/zayıf okuma var. %s.", buy, sell, strings.Join(parts, "; "))
}

func movingAverageStatus(daily Timeframe) string {
	buy, sell, computed := movingAverageCounts(movingAverageSignals(daily))
	if computed == 0 {
		return "fail"
	}
	if buy >= sell+2 {
		return "pass"
	}
	if sell >= buy+2 {
		return "fail"
	}
	return "limited"
}

func movingAverageConfidence(daily Timeframe) float64 {
	_, _, computed := movingAverageCounts(movingAverageSignals(daily))
	return mathutil.Clamp(float64(computed)*7+20, 25, 90)
}

func movingAverageSignals(daily Timeframe) []qaMovingAverageSignal {
	out := []qaMovingAverageSignal{}
	for _, period := range []int{5, 10, 20, 50, 100, 200} {
		for _, exponential := range []bool{false, true} {
			value, ok := movingAverageValue(daily.Indicators, period, exponential)
			kind := "Temel"
			if exponential {
				kind = "Üssel"
			}
			signal := qaMovingAverageSignal{Period: period, Kind: kind, Value: value, Computed: ok}
			if ok {
				if daily.LastClose >= value {
					signal.Vote = 1
				} else {
					signal.Vote = -1
				}
			}
			out = append(out, signal)
		}
	}
	return out
}

func movingAverageValue(ind ohlcv.IndicatorSnapshot, period int, exponential bool) (float64, bool) {
	var value float64
	switch {
	case period == 5 && !exponential:
		value = ind.SMA5
	case period == 10 && !exponential:
		value = ind.SMA10
	case period == 20 && !exponential:
		value = ind.SMA20
	case period == 50 && !exponential:
		value = ind.SMA50
	case period == 100 && !exponential:
		value = ind.SMA100
	case period == 200 && !exponential:
		value = ind.SMA200
	case period == 5 && exponential:
		value = ind.EMA5
	case period == 10 && exponential:
		value = ind.EMA10
	case period == 20 && exponential:
		value = ind.EMA20
	case period == 50 && exponential:
		value = ind.EMA50
	case period == 100 && exponential:
		value = ind.EMA100
	case period == 200 && exponential:
		value = ind.EMA200
	}
	return value, value > 0
}

func movingAverageCounts(signals []qaMovingAverageSignal) (buy, sell, computed int) {
	for _, signal := range signals {
		if !signal.Computed {
			continue
		}
		computed++
		if signal.Vote > 0 {
			buy++
		} else if signal.Vote < 0 {
			sell++
		}
	}
	return buy, sell, computed
}

func buySellLabel(current, average float64) string {
	if current >= average {
		return "Al"
	}
	return "Sat"
}

func oscillatorFAQAnswer(daily Timeframe) string {
	ind := daily.Indicators
	rsi := rsiTechnicalSignal(ind.RSI14)
	macd := macdTechnicalSignal(ind.MACD, ind.MACDSignal, ind.MACDHistogram)
	stoch := stochasticTechnicalSignal("Stochastic", ind.StochasticK, ind.StochasticD)
	stochRSI := stochasticTechnicalSignal("StochRSI", ind.StochRSIK, ind.StochRSID)
	return fmt.Sprintf("RSI %.3f: %s. MACD %.3f: %s. Stochastic %.3f: %s. StochRSI %.3f: %s.",
		ind.RSI14,
		empty(rsi.Label, "ölçülemedi"),
		ind.MACD,
		empty(macd.Label, "ölçülemedi"),
		ind.StochasticK,
		empty(stoch.Label, "ölçülemedi"),
		ind.StochRSIK,
		empty(stochRSI.Label, "ölçülemedi"),
	)
}

func oscillatorFAQStatus(daily Timeframe) string {
	return technicalIndicatorBreadthStatus(daily)
}

func pivotAnswer(daily Timeframe, currency string) string {
	lines := []string{}
	for _, family := range []struct {
		Key   string
		Label string
	}{
		{"Classic Pivot Points", "Klasik"},
		{"Fibonacci Pivot Points", "Fibonacci"},
		{"Camarilla Pivot Points", "Camarilla"},
		{"Woodie Pivot Points", "Woodie"},
		{"DeMark Pivot Points", "Demark"},
	} {
		if line, ok := pivotFamilyLine(daily, family.Key, family.Label, currency); ok {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return "Pivot noktası hesaplanamadı."
	}
	return strings.Join(lines, " | ") + ". Destek alt tarafı, direnç üst tarafı anlatır; tek başına al/sat emri değildir."
}

func pivotFamilyLine(daily Timeframe, keyPrefix, label, currency string) (string, bool) {
	tools := daily.Indicators.SupportTools
	values := []string{}
	found := false
	for _, key := range []string{"S3", "S2", "S1", "Pivot", "R1", "R2", "R3"} {
		value := tools[keyPrefix+" "+key]
		if key == "Pivot" && value <= 0 {
			value = tools[keyPrefix]
		}
		if keyPrefix == "Classic Pivot Points" {
			switch key {
			case "Pivot":
				if value <= 0 {
					value = daily.Indicators.PivotPoint
				}
			case "R1":
				if value <= 0 {
					value = daily.Indicators.PivotR1
				}
			case "R2":
				if value <= 0 {
					value = daily.Indicators.PivotR2
				}
			case "S1":
				if value <= 0 {
					value = daily.Indicators.PivotS1
				}
			case "S2":
				if value <= 0 {
					value = daily.Indicators.PivotS2
				}
			}
		}
		if value > 0 {
			found = true
			values = append(values, fmt.Sprintf("%s %s", key, priceLevelText(value, currency)))
		} else {
			values = append(values, key+" -")
		}
	}
	return label + ": " + strings.Join(values, ", "), found
}

func pivotStatus(daily Timeframe) string {
	if daily.Indicators.PivotPoint > 0 || daily.Indicators.SupportTools["Classic Pivot Points"] > 0 {
		return "info"
	}
	return "fail"
}

func pivotConfidence(daily Timeframe) float64 {
	count := 0
	for _, prefix := range []string{"Classic Pivot Points", "Fibonacci Pivot Points", "Camarilla Pivot Points", "Woodie Pivot Points", "DeMark Pivot Points"} {
		for _, key := range []string{"S3", "S2", "S1", "Pivot", "R1", "R2", "R3"} {
			if daily.Indicators.SupportTools[prefix+" "+key] > 0 {
				count++
			}
		}
	}
	if count == 0 && daily.Indicators.PivotPoint > 0 {
		count = 5
	}
	return mathutil.Clamp(float64(count)*3+25, 25, 90)
}

func priceLevelText(value float64, currency string) string {
	if strings.TrimSpace(currency) == "" {
		return fmt.Sprintf("%.3f", value)
	}
	return fmt.Sprintf("%.3f %s", value, currency)
}

func technicalAnswer(daily Timeframe) string {
	if daily.TechnicalGate.Status != "" {
		return fmt.Sprintf("Günlük skor %.1f/100, eğilim %s, RSI %.1f, MACD histogram %.4f, para akışı %.4f. Teknik sinyal kapısı %s: %.0f/100; %s.", daily.Score, trendBiasTR(daily.TrendBias), daily.Indicators.RSI14, daily.Indicators.MACDHistogram, daily.Indicators.ChaikinMoneyFlow20, statusLabelTR(daily.TechnicalGate.Status), daily.TechnicalGate.Score, daily.TechnicalGate.Label)
	}
	return fmt.Sprintf("Günlük skor %.1f/100, eğilim %s, RSI %.1f, MACD histogram %.4f, para akışı %.4f.", daily.Score, trendBiasTR(daily.TrendBias), daily.Indicators.RSI14, daily.Indicators.MACDHistogram, daily.Indicators.ChaikinMoneyFlow20)
}

func trendBiasTR(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bullish", "long":
		return "yükseliş"
	case "bearish", "short":
		return "düşüş"
	case "neutral", "flat", "":
		return "nötr"
	default:
		return value
	}
}

func statusLabelTR(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pass":
		return "Geçti"
	case "limited":
		return "Sınırlı"
	case "fail":
		return "Başarısız"
	case "not_applicable", "not applicable":
		return "Uygulanmaz"
	default:
		return empty(value, "Yok")
	}
}

func technicalStatus(daily Timeframe) string {
	if daily.TechnicalGate.Status != "" {
		return daily.TechnicalGate.Status
	}
	if daily.Score >= 60 && daily.TrendBias == "bullish" {
		return "pass"
	}
	if daily.Score >= 45 {
		return "limited"
	}
	return "fail"
}

func liquidityAnswer(liq LiquidityReport, currency string) string {
	return fmt.Sprintf("%s. 20g ortalama işlem hacmi %s %s, 10%% ADV kapasite %s %s.", liq.InstitutionalFit, money(liq.AverageValueTraded), currency, money(liq.CapacityAt10PctADV), currency)
}

type weightedScore struct {
	score  float64
	weight float64
}

func weighted(values []weightedScore) float64 {
	sum := 0.0
	weight := 0.0
	for _, value := range values {
		sum += value.score * value.weight
		weight += value.weight
	}
	return mathutil.Clamp(mathutil.SafeDiv(sum, weight), 0, 100)
}

type threshold struct {
	min   float64
	score float64
}

func thresholdScore(value float64, thresholds []threshold) float64 {
	for _, threshold := range thresholds {
		if value >= threshold.min {
			return threshold.score
		}
	}
	return 0
}

func label(score float64) string {
	switch status(score) {
	case "pass":
		return "güçlü"
	case "limited":
		return "sınırlı"
	default:
		return "zayıf"
	}
}

func status(score float64) string {
	switch {
	case score >= 70:
		return "pass"
	case score >= 45:
		return "limited"
	default:
		return "fail"
	}
}

func riskStatus(value string) string {
	if strings.Contains(strings.ToLower(value), "hesaplanam") || strings.Contains(strings.ToLower(value), "negatif") || strings.Contains(strings.ToLower(value), "zayıf") {
		return "fail"
	}
	return "limited"
}

func dataLineageText(gov professional.FinancialDataGovernance) string {
	if gov.LineageEvents > 0 && gov.FinanciallyConsistent {
		return "veri soy ağacı ve finansal tutarlılık var"
	}
	if gov.FinanciallyConsistent {
		return "finansal tutarlılık var; soy ağacı sınırlı"
	}
	return "finansal veri tutarlılığı sınırlı"
}

func currentFinancialDataDecisionSafe(gov professional.FinancialDataGovernance) bool {
	if gov.ProductionReady {
		return true
	}
	if !gov.FinanciallyConsistent || gov.ReconciliationFailureCount > 0 {
		return false
	}
	if strings.TrimSpace(gov.LatestPeriod) == "" {
		return false
	}
	if gov.LatestPublishDate == nil && gov.LatestAvailableAt == nil {
		return false
	}
	if len(gov.InvalidChronologyPeriods) > 0 {
		return false
	}
	if !gov.BacktestSafe && gov.UnsafeAvailabilityCount > 0 {
		return false
	}
	return true
}

func cryptoDataLineageText(pro professional.Report) string {
	available := len(pro.Coverage.Available)
	missing := len(pro.Coverage.Missing)
	switch {
	case missing == 0 && available > 0:
		return "kripto fiyat, likidite ve tamamlayıcı veri kaynakları bağlı"
	case available > 0:
		return "kripto fiyat ve teknik veri bağlı; on-chain, derivatives, exchange-flow veya haber/sentiment kapsamı sınırlı"
	default:
		return "kripto veri soy ağacı sınırlı; canlı işlem için kaynak kapsamı güçlendirilmeli"
	}
}

func marketDataLineageText(assetType string, pro professional.Report) string {
	if ohlcv.IsCryptoAssetType(assetType) {
		return cryptoDataLineageText(pro)
	}
	available := len(pro.Coverage.Available)
	missing := len(pro.Coverage.Missing)
	switch {
	case missing == 0 && available > 0:
		return "altın/emtia fiyat, likidite, makro, pozisyon ve akış kaynakları bağlı"
	case available > 0:
		return "TradingView fiyat ve teknik veri bağlı; DXY/reel faiz, vadeli pozisyon, fon akışı veya haber kapsamı sınırlı"
	default:
		return "altın/emtia veri soy ağacı sınırlı; canlı işlem için kaynak kapsamı güçlendirilmeli"
	}
}

func isMarketOnlyAssetType(assetType string) bool {
	return ohlcv.IsCryptoAssetType(assetType) || ohlcv.IsCommodityAssetType(assetType)
}

func marketAssetLabel(assetType string) string {
	if ohlcv.IsCryptoAssetType(assetType) {
		return "Kripto"
	}
	if ohlcv.IsCommodityAssetType(assetType) {
		return "Altın/emtia"
	}
	return "Varlık"
}

func joinOr(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	limit := len(values)
	if limit > 4 {
		limit = 4
	}
	return strings.Join(values[:limit], ", ")
}

func empty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolTR(value bool) string {
	if value {
		return "evet"
	}
	return "hayır"
}

func money(value float64) string {
	abs := math.Abs(value)
	unit := ""
	divisor := 1.0
	switch {
	case abs >= 1_000_000_000_000:
		unit = " trilyon"
		divisor = 1_000_000_000_000
	case abs >= 1_000_000_000:
		unit = " milyar"
		divisor = 1_000_000_000
	case abs >= 1_000_000:
		unit = " milyon"
		divisor = 1_000_000
	}
	return fmt.Sprintf("%.2f%s", value/divisor, unit)
}
