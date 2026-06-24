package chart

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"

	"hissebot/internal/ta/contrarian"
	"hissebot/internal/ta/localize"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"

	"golang.org/x/image/font"
)

type DecisionRenderInput struct {
	Symbol            string
	CompanyName       string
	AnalysisDate      string
	Currency          string
	Timeframe         string
	LastClose         float64
	LastVolume        float64
	OverallScore      float64
	OverallBias       string
	TimeframeScore    float64
	TrendBias         string
	Indicators        ohlcv.IndicatorSnapshot
	Patterns          []ohlcv.PatternResult
	SupportLevels     []ohlcv.SupportResistanceLevel
	ResistanceLevels  []ohlcv.SupportResistanceLevel
	NearestSupport    *ohlcv.SupportResistanceLevel
	NearestResistance *ohlcv.SupportResistanceLevel
	TradePlan         ohlcv.TradePlan
	Professional      professional.Report
	Behavioral        contrarian.Report
	Disclaimer        string
}

type DecisionRenderer struct {
	width  int
	height int
}

type decisionPalette struct {
	background color.Color
	panel      color.Color
	text       color.Color
	muted      color.Color
	grid       color.Color
	accent     color.Color
	accentSoft color.Color
	good       color.Color
	goodSoft   color.Color
	warn       color.Color
	warnSoft   color.Color
	bad        color.Color
	badSoft    color.Color
}

type decisionEvaluation struct {
	Decision      string
	SignalImpact  float64
	Quality       string
	PlanStatus    string
	EntryLevel    string
	EntryTrigger  string
	StopLabel     string
	StopLevel     string
	Target1Label  string
	Target1       string
	Target2Label  string
	Target2       string
	RiskReward    string
	ReasonsTitle  string
	ReasonsFor    []string
	Risks         []string
	ExitRules     []string
	ExitTitle     string
	ExitIntro     string
	Comment       string
	Result        string
	Components    []decisionComponent
	DecisionColor color.Color
	DecisionSoft  color.Color
}

type decisionComponent struct {
	Label  string
	Score  float64
	Weight float64
}

func NewDecisionRenderer() *DecisionRenderer {
	return &DecisionRenderer{width: 1800, height: 1300}
}

func (r *DecisionRenderer) RenderPNG(ctx context.Context, input DecisionRenderInput) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("karar grafigi cizimi iptal edildi: %w", err)
	}
	fonts, err := loadFonts()
	if err != nil {
		return nil, err
	}

	palette := decisionPalette{
		background: rgb(13, 17, 28),
		panel:      rgb(20, 25, 40),
		text:       rgb(209, 214, 228),
		muted:      rgb(120, 130, 155),
		grid:       rgb(38, 44, 65),
		accent:     rgb(91, 143, 249),
		accentSoft: rgb(20, 30, 55),
		good:       rgb(38, 166, 154),
		goodSoft:   rgb(20, 44, 50),
		warn:       rgb(255, 177, 66),
		warnSoft:   rgb(42, 34, 14),
		bad:        rgb(239, 83, 80),
		badSoft:    rgb(48, 20, 24),
	}
	evaluation := evaluateDecision(input, palette)

	img := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	fillRect(img, img.Bounds(), palette.background)

	drawDecisionHeader(img, fonts, palette, input, evaluation)
	drawDecisionMetrics(img, fonts, palette, input, evaluation)
	drawDecisionGauge(img, fonts, palette, evaluation)
	drawDecisionPlan(img, fonts, palette, evaluation)
	drawDecisionComponents(img, fonts, palette, evaluation)
	drawDecisionReasons(img, fonts, palette, evaluation)
	drawDecisionExitRules(img, fonts, palette, evaluation)
	drawDecisionComment(img, fonts, palette, evaluation, input.Disclaimer)

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("karar png olusturulamadi: %w", err)
	}
	return out.Bytes(), nil
}

func evaluateDecision(input DecisionRenderInput, palette decisionPalette) decisionEvaluation {
	components := decisionComponents(input)
	impact := 0.0
	for _, component := range components {
		impact += component.Score
	}
	impact = clampFloat(impact, 0, 100)

	plan := input.TradePlan
	clearEntry := activeLongPlan(plan)
	forcedWait := !clearEntry ||
		input.OverallScore < 50 ||
		input.TrendBias == "bearish" ||
		plan.Rejected ||
		plan.Direction != "long" ||
		importantAveragesBroken(input) ||
		macdNegative(input.Indicators) ||
		input.Indicators.ChaikinMoneyFlow20 < 0 ||
		(plan.RiskRewardRatio > 0 && plan.RiskRewardRatio < 1.5) ||
		financialGateFailed(input)

	decision := "BEKLE"
	if !forcedWait {
		switch {
		case impact >= 80 && perfectBuyConditions(input):
			decision = "MÜKEMMEL AL"
		case impact >= 65 && strongBuyConditions(input):
			decision = "GÜÇLÜ AL"
		case impact >= 55 && normalBuyConditions(input):
			decision = "NORMAL AL"
		}
	}

	quality := buyQuality(decision, impact)
	color := palette.warn
	soft := palette.warnSoft
	if strings.Contains(decision, "AL") {
		color = palette.good
		soft = palette.goodSoft
	}

	entryLevel := "Henüz oluşmadı"
	if clearEntry {
		entryLevel = fmt.Sprintf("%.2f üzeri kapanış teyidi", plan.EntryMax)
	}
	return decisionEvaluation{
		Decision:      decision,
		SignalImpact:  impact,
		Quality:       quality,
		PlanStatus:    planStatus(plan),
		EntryLevel:    entryLevel,
		EntryTrigger:  entryTrigger(input, clearEntry),
		StopLabel:     stopLabel(clearEntry),
		StopLevel:     stopLevel(input, clearEntry),
		Target1Label:  target1Label(clearEntry),
		Target1:       targetLevel(input, 1, clearEntry),
		Target2Label:  target2Label(clearEntry),
		Target2:       targetLevel(input, 2, clearEntry),
		RiskReward:    riskRewardText(plan.RiskRewardRatio),
		ReasonsTitle:  reasonsTitle(decision),
		ReasonsFor:    reasonsFor(input, decision),
		Risks:         risks(input),
		ExitRules:     exitRules(input),
		ExitTitle:     exitTitle(clearEntry),
		ExitIntro:     exitIntro(clearEntry),
		Comment:       professionalComment(input, decision, impact),
		Result:        resultText(decision, clearEntry),
		Components:    components,
		DecisionColor: color,
		DecisionSoft:  soft,
	}
}

func decisionComponents(input DecisionRenderInput) []decisionComponent {
	return []decisionComponent{
		{Label: "Trend ve skor", Weight: 18, Score: trendScore(input)},
		{Label: "Destek / direnç", Weight: 12, Score: supportResistanceScore(input)},
		{Label: "Ortalamalar", Weight: 12, Score: movingAverageScore(input)},
		{Label: "Momentum", Weight: 16, Score: momentumScore(input)},
		{Label: "Para akışı / hacim", Weight: 10, Score: moneyFlowScore(input)},
		{Label: "Formasyon", Weight: 8, Score: patternQualityScore(input)},
		{Label: "Bilanço kalitesi", Weight: 14, Score: financialQualityScore(input)},
		{Label: "Değerleme / senaryo", Weight: 10, Score: valuationScenarioScore(input)},
	}
}

func trendScore(input DecisionRenderInput) float64 {
	bias := 0.5
	if input.TrendBias == "bullish" {
		bias = 1
	}
	if input.TrendBias == "bearish" {
		bias = 0.12
	}
	scorePart := clampFloat(input.OverallScore/100, 0, 1)
	return (scorePart*0.6 + bias*0.4) * 18
}

func supportResistanceScore(input DecisionRenderInput) float64 {
	if input.NearestSupport == nil || input.NearestResistance == nil || input.LastClose <= 0 {
		return 5
	}
	risk := math.Abs(input.LastClose - input.NearestSupport.Price)
	reward := math.Abs(input.NearestResistance.Price - input.LastClose)
	rr := safeDiv(reward, risk)
	positionScore := 1 - safeDiv(input.LastClose-input.NearestSupport.Price, input.NearestResistance.Price-input.NearestSupport.Price)
	return clampFloat((clampFloat(rr/2.5, 0, 1)*0.65+clampFloat(positionScore, 0, 1)*0.35)*12, 0, 12)
}

func movingAverageScore(input DecisionRenderInput) float64 {
	last := input.LastClose
	ind := input.Indicators
	if last <= 0 {
		return 0
	}
	values := []float64{ind.SMA20, ind.SMA50, ind.SMA100, ind.SMA200, ind.EMA20, ind.EMA50}
	points := 0.0
	available := 0.0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		available++
		if last > value {
			points++
		}
	}
	return safeDiv(points, available) * 12
}

func momentumScore(input DecisionRenderInput) float64 {
	ind := input.Indicators
	score := 0.0
	if ind.RSI14 >= 55 && ind.RSI14 <= 70 {
		score += 5.6
	} else if ind.RSI14 >= 45 && ind.RSI14 < 55 {
		score += 4
	} else if ind.RSI14 >= 40 {
		score += 2.4
	}
	if !macdNegative(ind) {
		score += 5.6
	}
	if ind.StochasticK > ind.StochasticD && ind.StochasticK > 20 && ind.StochasticK < 85 {
		score += 3.2
	}
	if ind.ADX14 >= 18 {
		score += 1.6
	}
	return clampFloat(score, 0, 16)
}

func moneyFlowScore(input DecisionRenderInput) float64 {
	score := 0.0
	if input.Indicators.ChaikinMoneyFlow20 > 0 {
		score += 4
	}
	if input.LastVolume > 0 && input.Indicators.VolumeSMA20 > 0 && input.LastVolume >= input.Indicators.VolumeSMA20 {
		score += 3.5
	}
	if anyVolumeConfirmed(input.Patterns) {
		score += 2.5
	}
	return clampFloat(score, 0, 10)
}

func patternQualityScore(input DecisionRenderInput) float64 {
	score := 4.0
	for _, pattern := range input.Patterns {
		if pattern.Confidence < 0.5 {
			continue
		}
		if pattern.Direction == "bullish" {
			score += pattern.Confidence * 2.4
		}
		if pattern.Direction == "bearish" {
			score -= pattern.Confidence * 2
		}
	}
	return clampFloat(score, 0, 8)
}

func financialQualityScore(input DecisionRenderInput) float64 {
	if !hasProfessionalContext(input) {
		return 0
	}
	pro := input.Professional
	score := clampFloat(pro.DataQuality/100, 0, 1) * 3
	if pro.DataGovernance.BacktestSafe {
		score += 1.5
	}
	if pro.DataGovernance.ProductionReady {
		score += 1.5
	}
	roe := pro.Valuation.Ratios["ROE"]
	switch {
	case roe >= 0.18:
		score += 3
	case roe >= 0.10:
		score += 2.3
	case roe > 0:
		score += 1.2
	}
	netDebtEq := pro.Valuation.Ratios["NetDebt_Eq"]
	switch {
	case netDebtEq <= 0:
		score += 2
	case netDebtEq <= 0.5:
		score += 1.7
	case netDebtEq <= 1:
		score += 0.8
	}
	if pro.Valuation.Ratios["Net_Margin"] > 0 {
		score += 1
	}
	if pro.Valuation.FreeCashFlowTTM > 0 {
		score += 1
	}
	return clampFloat(score, 0, 14)
}

func valuationScenarioScore(input DecisionRenderInput) float64 {
	if !hasProfessionalContext(input) {
		return 0
	}
	pro := input.Professional
	score := 0.0
	switch strings.ToLower(pro.Peers.ValuationSignal) {
	case "discount":
		score += 3
	case "neutral":
		score += 2
	case "premium":
		score += 0.5
	default:
		score += 1
	}
	baseReturn, ok := scenarioReturn(pro.Scenarios, "base")
	if ok {
		switch {
		case baseReturn >= 20:
			score += 4
		case baseReturn >= 8:
			score += 3
		case baseReturn >= 0:
			score += 2
		case baseReturn > -10:
			score += 0.8
		}
	}
	if pro.Valuation.FairValue.Confidence >= 0.65 {
		score += 1.5
	} else if pro.Valuation.FairValue.Confidence > 0 {
		score += 0.8
	}
	if pro.Market.RelativeStrength60 > 0 {
		score += 1.5
	} else if pro.Market.RelativeStrength20 > 0 {
		score += 0.8
	}
	return clampFloat(score, 0, 10)
}

func hasProfessionalContext(input DecisionRenderInput) bool {
	pro := input.Professional
	return pro.DataQuality > 0 ||
		pro.Valuation.MarketCap > 0 ||
		pro.Valuation.Equity > 0 ||
		pro.Company.Sector != "" ||
		len(pro.Scenarios) > 0
}

func financialGateFailed(input DecisionRenderInput) bool {
	if !hasProfessionalContext(input) {
		return false
	}
	pro := input.Professional
	if pro.DataQuality > 0 && pro.DataQuality < 65 {
		return true
	}
	if pro.DataGovernance.DataMode == "production" && !pro.DataGovernance.ProductionReady {
		return true
	}
	if strings.EqualFold(pro.Peers.ValuationSignal, "premium") {
		if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok && baseReturn < 0 {
			return true
		}
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok && baseReturn <= -15 && pro.Valuation.FairValue.Confidence >= 0.5 {
		return true
	}
	return false
}

func financiallyNormal(input DecisionRenderInput) bool {
	if !hasProfessionalContext(input) {
		return false
	}
	return financialQualityScore(input) >= 6 && valuationScenarioScore(input) >= 3
}

func financiallyStrong(input DecisionRenderInput) bool {
	if !hasProfessionalContext(input) {
		return false
	}
	return financialQualityScore(input) >= 9 && valuationScenarioScore(input) >= 5
}

func scenarioReturn(scenarios []professional.Scenario, name string) (float64, bool) {
	for _, scenario := range scenarios {
		if strings.EqualFold(scenario.Name, name) {
			return scenario.ReturnPct, true
		}
	}
	return 0, false
}

func normalBuyConditions(input DecisionRenderInput) bool {
	plan := input.TradePlan
	return input.OverallScore >= 55 &&
		input.OverallScore <= 80 &&
		input.Indicators.RSI14 >= 40 &&
		input.LastClose > 0 &&
		(input.LastClose > input.Indicators.EMA20 || input.LastClose > input.Indicators.SMA20) &&
		plan.RiskRewardRatio >= 1.5 &&
		financiallyNormal(input)
}

func strongBuyConditions(input DecisionRenderInput) bool {
	plan := input.TradePlan
	return input.OverallScore >= 65 &&
		input.TrendBias != "bearish" &&
		input.LastClose > input.Indicators.SMA20 &&
		input.LastClose > input.Indicators.SMA50 &&
		!macdNegative(input.Indicators) &&
		input.Indicators.RSI14 >= 50 &&
		input.Indicators.ChaikinMoneyFlow20 > 0 &&
		plan.RiskRewardRatio >= 2 &&
		financiallyStrong(input)
}

func perfectBuyConditions(input DecisionRenderInput) bool {
	ind := input.Indicators
	plan := input.TradePlan
	return input.OverallScore >= 80 &&
		input.TrendBias == "bullish" &&
		input.LastClose > ind.SMA20 &&
		input.LastClose > ind.SMA50 &&
		input.LastClose > ind.SMA100 &&
		input.LastClose > ind.SMA200 &&
		!macdNegative(ind) &&
		ind.RSI14 >= 55 && ind.RSI14 <= 70 &&
		ind.ADX14 >= 20 &&
		ind.ChaikinMoneyFlow20 > 0 &&
		plan.RiskRewardRatio >= 2.5 &&
		plan.StopLoss > 0 &&
		financialQualityScore(input) >= 12 &&
		valuationScenarioScore(input) >= 7
}

func importantAveragesBroken(input DecisionRenderInput) bool {
	last := input.LastClose
	ind := input.Indicators
	if last <= 0 {
		return true
	}
	broken := 0
	checked := 0
	for _, avg := range []float64{ind.SMA20, ind.SMA50, ind.EMA20} {
		if avg <= 0 {
			continue
		}
		checked++
		if last < avg {
			broken++
		}
	}
	return checked > 0 && broken >= 2
}

func macdNegative(ind ohlcv.IndicatorSnapshot) bool {
	if ind.MACD == 0 && ind.MACDSignal == 0 && ind.MACDHistogram == 0 {
		return false
	}
	return ind.MACDHistogram < 0 || ind.MACD < ind.MACDSignal
}

func entryTrigger(input DecisionRenderInput, clearEntry bool) string {
	if clearEntry {
		return fmt.Sprintf("%.2f üzerinde kapanış ve hacim teyidi", input.TradePlan.EntryMax)
	}
	if input.NearestResistance != nil {
		return fmt.Sprintf("%.2f üstü kapanış + hacim", input.NearestResistance.Price)
	}
	return "EMA20 üstü kapanış + momentum"
}

func stopLevel(input DecisionRenderInput, activeLong bool) string {
	if activeLong && input.TradePlan.StopLoss > 0 {
		return fmt.Sprintf("%.2f", input.TradePlan.StopLoss)
	}
	if activeLong && input.NearestSupport != nil && input.Indicators.ATR14 > 0 {
		return fmt.Sprintf("%.2f", input.NearestSupport.Price-input.Indicators.ATR14*0.35)
	}
	if input.NearestSupport != nil {
		return fmt.Sprintf("%.2f", input.NearestSupport.Price)
	}
	return "Hesaplanamadı"
}

func targetLevel(input DecisionRenderInput, index int, activeLong bool) string {
	if !activeLong {
		if index == 1 && input.NearestResistance != nil {
			return fmt.Sprintf("%.2f", input.NearestResistance.Price)
		}
		if index == 2 {
			if spotLongUnavailablePlan(input.TradePlan) {
				return "Yok (spot varlık)"
			}
			if input.TradePlan.Rejected {
				return "Yok (reddedildi)"
			}
			return "Yok"
		}
		return "Hesaplanamadı"
	}
	if index == 1 {
		if input.TradePlan.TakeProfit1 > 0 {
			return fmt.Sprintf("%.2f", input.TradePlan.TakeProfit1)
		}
		if input.NearestResistance != nil {
			return fmt.Sprintf("%.2f", input.NearestResistance.Price)
		}
	}
	if input.TradePlan.TakeProfit2 > 0 {
		return fmt.Sprintf("%.2f", input.TradePlan.TakeProfit2)
	}
	if len(input.ResistanceLevels) > 1 {
		return fmt.Sprintf("%.2f", input.ResistanceLevels[1].Price)
	}
	return "Hesaplanamadı"
}

func stopLabel(activeLong bool) string {
	if activeLong {
		return "Alım zarar kes"
	}
	return "İzlenecek destek"
}

func target1Label(activeLong bool) string {
	if activeLong {
		return "Alım hedef 1"
	}
	return "İzlenecek direnç"
}

func target2Label(activeLong bool) string {
	if activeLong {
		return "Alım hedef 2"
	}
	return "İşlem planı"
}

func activeLongPlan(plan ohlcv.TradePlan) bool {
	return plan.Direction == "long" && !plan.Rejected && plan.EntryMax > 0
}

func planStatus(plan ohlcv.TradePlan) string {
	direction := localize.Direction(plan.Direction)
	if plan.Direction == "" {
		direction = "Yok"
	}
	if plan.Rejected && (plan.Direction == "neutral" || plan.Direction == "") {
		return "Aktif alım planı yok"
	}
	if plan.Rejected {
		return fmt.Sprintf("%s taslak reddedildi", direction)
	}
	if plan.Direction == "neutral" || plan.Direction == "" {
		return "Aktif plan yok"
	}
	return fmt.Sprintf("%s planı aktif", direction)
}

func riskRewardText(value float64) string {
	if value <= 0 {
		return "Hesaplanamadı"
	}
	return fmt.Sprintf("%.2f", value)
}

func buyQuality(decision string, impact float64) string {
	if decision == "BEKLE" {
		return "Düşük"
	}
	switch {
	case impact >= 80:
		return "Mükemmel"
	case impact >= 70:
		return "Çok İyi"
	case impact >= 65:
		return "İyi"
	case impact >= 55:
		return "Orta"
	default:
		return "Düşük"
	}
}

func reasonsFor(input DecisionRenderInput, decision string) []string {
	if decision == "BEKLE" {
		return waitReasons(input)
	}
	reasons := []string{}
	if input.NearestSupport != nil && input.LastClose >= input.NearestSupport.Price {
		reasons = append(reasons, fmt.Sprintf("Fiyat en yakın destek %.2f üzerinde tutunuyor.", input.NearestSupport.Price))
	}
	if !macdNegative(input.Indicators) {
		reasons = append(reasons, "MACD negatif bölgede değil ve momentum lehine çalışıyor.")
	}
	if input.Indicators.RSI14 >= 50 {
		reasons = append(reasons, fmt.Sprintf("RSI %.2f ile güç kazanma bölgesinde.", input.Indicators.RSI14))
	}
	if input.LastClose > input.Indicators.SMA20 && input.LastClose > input.Indicators.SMA50 {
		reasons = append(reasons, "Fiyat MA20 ve MA50 üzerinde.")
	}
	if input.TradePlan.RiskRewardRatio >= 1.5 {
		reasons = append(reasons, fmt.Sprintf("Risk/getiri %.2f ile kabul edilebilir eşik üzerinde.", input.TradePlan.RiskRewardRatio))
	}
	reasons = append(reasons, financialPositiveReasons(input)...)
	if len(reasons) == 0 {
		if isCryptoProfessional(input) {
			reasons = append(reasons, "Alım için fiyat, momentum, risk/getiri ve kripto veri teyitleri birlikte yeterli değil.")
		} else {
			reasons = append(reasons, "Alım için fiyat, momentum, risk/getiri ve bilanço teyitleri birlikte yeterli değil.")
		}
	}
	return reasons
}

func reasonsTitle(decision string) string {
	if decision == "BEKLE" {
		return "Giriş İçin Teyitler"
	}
	return "Neden Girmeliyim"
}

func waitReasons(input DecisionRenderInput) []string {
	reasons := []string{}
	plan := input.TradePlan
	if plan.Direction == "long" && !plan.Rejected {
		reasons = append(reasons, fmt.Sprintf("Alım planı var; %.2f üstü kapanış ve hacim teyidi bekleniyor.", plan.EntryMax))
	}
	if spotLongUnavailablePlan(plan) {
		reasons = append(reasons, "Düşüş sinyali spot varlık için alım kurulumuna çevrilmedi.")
	}
	if plan.Rejected && plan.RiskRewardRatio > 0 && plan.RiskRewardRatio < 1.5 {
		reasons = append(reasons, fmt.Sprintf("Risk/getiri %.2f; en az 1.50 eşiğine çıkmalı.", plan.RiskRewardRatio))
	}
	if input.TrendBias == "bearish" {
		if input.NearestResistance != nil {
			reasons = append(reasons, fmt.Sprintf("Trend düşüşte; %.2f direnci üstünde kapanış görülmeli.", input.NearestResistance.Price))
		} else {
			reasons = append(reasons, "Trend düşüşte; önce yukarı yönlü yapı kırılımı görülmeli.")
		}
	} else if input.TrendBias == "bullish" {
		reasons = append(reasons, "Ana eğilim pozitif; giriş için momentum ve risk/getiri teyidi aranıyor.")
	} else {
		reasons = append(reasons, "Trend kararsız; yön teyidi oluşmadan giriş ertelenir.")
	}
	if importantAveragesBroken(input) {
		reasons = append(reasons, movingAverageRecoveryText(input))
	} else if input.LastClose > 0 && input.Indicators.SMA20 > 0 && input.LastClose > input.Indicators.SMA20 {
		reasons = append(reasons, fmt.Sprintf("Fiyat SMA20 %.2f üzerinde; bu desteğin korunması izlenmeli.", input.Indicators.SMA20))
	}
	if macdNegative(input.Indicators) {
		reasons = append(reasons, fmt.Sprintf("MACD histogramı %.2f; kısa vadeli ivme zayıf, alım için hızlanma teyidi eksik.", input.Indicators.MACDHistogram))
	}
	if input.Indicators.RSI14 >= 70 {
		reasons = append(reasons, fmt.Sprintf("RSI %.2f aşırı sıcak; soğuma veya yatay güç toplama beklenmeli.", input.Indicators.RSI14))
	} else if input.Indicators.RSI14 > 0 && input.Indicators.RSI14 < 45 {
		reasons = append(reasons, fmt.Sprintf("RSI %.2f zayıf bölgede; 45 üstü toparlanma izlenmeli.", input.Indicators.RSI14))
	}
	if input.NearestSupport != nil {
		reasons = append(reasons, fmt.Sprintf("%.2f desteği kaybedilirse bekleme kararı güçlenir.", input.NearestSupport.Price))
	}
	reasons = append(reasons, financialWaitReasons(input)...)
	reasons = append(reasons, behavioralWaitReasons(input)...)
	return limitReasons(reasons, 4)
}

func movingAverageRecoveryText(input DecisionRenderInput) string {
	levels := []string{}
	if input.Indicators.EMA20 > 0 && input.LastClose < input.Indicators.EMA20 {
		levels = append(levels, fmt.Sprintf("EMA20 %.2f", input.Indicators.EMA20))
	}
	if input.Indicators.SMA20 > 0 && input.LastClose < input.Indicators.SMA20 {
		levels = append(levels, fmt.Sprintf("SMA20 %.2f", input.Indicators.SMA20))
	}
	if input.Indicators.SMA50 > 0 && input.LastClose < input.Indicators.SMA50 {
		levels = append(levels, fmt.Sprintf("SMA50 %.2f", input.Indicators.SMA50))
	}
	if len(levels) == 0 {
		return "Kısa vadeli ortalama teyidi netleşmeli."
	}
	return fmt.Sprintf("Fiyat %s altında; ortalama üstü kapanış beklenmeli.", strings.Join(levels, ", "))
}

func limitReasons(reasons []string, limit int) []string {
	if len(reasons) == 0 {
		return []string{"Veri teyidi eksik; fiyat, hacim ve momentum birlikte doğrulanmalı."}
	}
	if len(reasons) <= limit {
		return reasons
	}
	return reasons[:limit]
}

func risks(input DecisionRenderInput) []string {
	risks := []string{}
	if input.TradePlan.Rejected {
		risks = append(risks, tradePlanRejectDisplay(input.TradePlan))
	}
	if input.TrendBias == "bearish" {
		risks = append(risks, "Trend yönü düşüşte.")
	}
	if importantAveragesBroken(input) {
		risks = append(risks, "Fiyat önemli ortalamaların altında baskı görüyor.")
	}
	if macdNegative(input.Indicators) {
		risks = append(risks, "Kısa vadeli ivme zayıf; MACD alım hızlanmasını henüz doğrulamıyor.")
	}
	if input.Indicators.RSI14 < 45 {
		risks = append(risks, fmt.Sprintf("RSI %.2f ile 45 altında.", input.Indicators.RSI14))
	}
	if input.Indicators.ChaikinMoneyFlow20 < 0 {
		risks = append(risks, "Para akışı zayıf; yükseliş denemesi yeterli alıcı girişiyle desteklenmiyor.")
	}
	if input.TradePlan.RiskRewardRatio > 0 && input.TradePlan.RiskRewardRatio < 1.5 {
		risks = append(risks, fmt.Sprintf("Risk/getiri %.2f ile zayıf.", input.TradePlan.RiskRewardRatio))
	}
	for _, pattern := range input.Patterns {
		if pattern.Direction == "bearish" && pattern.Confidence >= 0.5 {
			risks = append(risks, fmt.Sprintf("%s formasyonu satış baskısı üretiyor.", localize.PatternName(pattern.Name)))
			break
		}
	}
	risks = append(risks, financialRiskReasons(input)...)
	risks = append(risks, behavioralRiskReasons(input)...)
	if len(risks) == 0 {
		risks = append(risks, "Belirgin ana risk yok; yine de stop ve teyit şartı korunmalı.")
	}
	return risks
}

func exitRules(input DecisionRenderInput) []string {
	if !activeLongPlan(input.TradePlan) {
		rules := []string{
			"Aktif alım pozisyonu yok; zarar kes seviyesi uygulanmaz.",
			"Fiyat direnç üstünde kapanış ve hacim teyidi üretmeden alım girişi yok.",
		}
		if input.TradePlan.Direction == "short" && !input.TradePlan.Rejected {
			rules = append(rules, "Günlük satış planı alım yerine risk/korunma senaryosu olarak izlenir.")
		}
		if spotLongUnavailablePlan(input.TradePlan) {
			rules = append(rules, "Düşüş senaryosu spot varlıkta aktif alım planına çevrilmez.")
		} else if input.TradePlan.Rejected {
			rules = append(rules, "Reddedilmiş taslak plan aktif işlem seviyesi sayılmaz.")
		}
		rules = append(rules,
			"Günlük trend düşüşte kaldıkça alım için bekle kararı korunur.",
			"MACD/RSI toparlanmadan alım sinyali üretilmez.",
		)
		return rules
	}
	stop := stopLevel(input, true)
	return []string{
		fmt.Sprintf("Fiyat %s zarar kes seviyesinin altına kapanırsa.", stop),
		"Fiyat güçlü destek altına kapanırsa.",
		"MACD yeniden negatif kesişim üretirse.",
		"RSI 45 altına sarkarsa.",
		"CMF negatife dönerse veya hacimli düşüş mumu oluşursa.",
		"Fiyat EMA20 altına kapanır ve trend tekrar düşüşe dönerse.",
	}
}

func exitTitle(activeLong bool) string {
	if activeLong {
		return "Sat Sinyali Gelince Çıkış Kuralı"
	}
	return "Bekleme Disiplini"
}

func exitIntro(activeLong bool) string {
	if activeLong {
		return "Aşağıdaki şartlardan 2 veya fazlası oluşursa sat sinyali kabul edilir:"
	}
	return "Aktif alım oluşmadan aşağıdaki koşullar korunur:"
}

func financialPositiveReasons(input DecisionRenderInput) []string {
	if !hasProfessionalContext(input) {
		return nil
	}
	pro := input.Professional
	reasons := []string{}
	if isCryptoProfessional(input) {
		if pro.DataQuality >= 70 {
			reasons = append(reasons, fmt.Sprintf("Kripto veri kapsamı %.0f/100; teknik fiyat okuması için yeterli temel kapsam var.", pro.DataQuality))
		}
		return reasons
	}
	if pro.DataQuality >= 85 && pro.DataGovernance.ProductionReady {
		reasons = append(reasons, fmt.Sprintf("Finansal veri kalitesi %.0f/100 ve üretim modunda kullanılabilir.", pro.DataQuality))
	}
	if roe := pro.Valuation.Ratios["ROE"]; roe >= 0.10 {
		reasons = append(reasons, fmt.Sprintf("Özsermaye kârlılığı %.1f%% ile pozitif finansal kalite desteği veriyor.", roe*100))
	}
	if netDebtEq := pro.Valuation.Ratios["NetDebt_Eq"]; netDebtEq >= 0 && netDebtEq <= 0.5 {
		reasons = append(reasons, fmt.Sprintf("Net borç/özsermaye %.2f; borçluluk kontrol edilebilir seviyede.", netDebtEq))
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok && baseReturn > 0 {
		reasons = append(reasons, fmt.Sprintf("Temel senaryo hedefi son fiyata göre %.1f%% yukarı alan gösteriyor.", baseReturn))
	}
	return reasons
}

func financialWaitReasons(input DecisionRenderInput) []string {
	if !hasProfessionalContext(input) {
		return nil
	}
	pro := input.Professional
	reasons := []string{}
	if isCryptoProfessional(input) {
		if pro.DataQuality > 0 && pro.DataQuality < 70 {
			reasons = append(reasons, fmt.Sprintf("Kripto veri kapsamı %.0f/100; on-chain/derivatives/exchange-flow kaynakları tamamlanmadan karar güveni sınırlı.", pro.DataQuality))
		}
		if len(pro.Coverage.Missing) > 0 {
			reasons = append(reasons, "Eksik kripto kaynakları: "+strings.Join(pro.Coverage.Missing, ", "))
		}
		return reasons
	}
	if pro.DataQuality > 0 && pro.DataQuality < 85 {
		reasons = append(reasons, fmt.Sprintf("Finansal veri kalitesi %.0f/100; eksik kaynak tamamlanmadan karar güveni sınırlı.", pro.DataQuality))
	}
	if pro.DataGovernance.DataMode == "production" && !pro.DataGovernance.ProductionReady {
		reasons = append(reasons, "Bilanço verisi production kullanım için hazır değil; finansal teyit beklenmeli.")
	}
	if strings.EqualFold(pro.Peers.ValuationSignal, "premium") {
		reasons = append(reasons, "Benzer şirket çarpanlarına göre pahalı bölgede; güvenli alım marjı zayıf.")
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok && baseReturn < 0 {
		reasons = append(reasons, fmt.Sprintf("Temel senaryo hedefi son fiyattan %.1f%% aşağıda; değerleme desteği yok.", math.Abs(baseReturn)))
	}
	if roe := pro.Valuation.Ratios["ROE"]; roe > 0 && roe < 0.08 {
		reasons = append(reasons, fmt.Sprintf("Özsermaye kârlılığı %.1f%%; finansal kalite güçlü alım için zayıf.", roe*100))
	}
	return reasons
}

func financialRiskReasons(input DecisionRenderInput) []string {
	if !hasProfessionalContext(input) {
		return nil
	}
	pro := input.Professional
	risks := []string{}
	if isCryptoProfessional(input) {
		if len(pro.Coverage.Missing) > 0 {
			risks = append(risks, "On-chain, derivatives veya exchange-flow kaynakları eksik; rapor teknik fiyat okumasıyla sınırlı.")
		}
		if pro.DataQuality > 0 && pro.DataQuality < 70 {
			risks = append(risks, fmt.Sprintf("Kripto veri kapsamı %.0f/100; rapor güveni sınırlı.", pro.DataQuality))
		}
		return risks
	}
	if pro.Company.Sector == "BIST Genel" || pro.Peers.PeerCount == 0 {
		risks = append(risks, "Sektör/peer karşılaştırması sınırlı; değerleme yorumu daha temkinli okunmalı.")
	}
	if pro.DataQuality > 0 && pro.DataQuality < 85 {
		risks = append(risks, fmt.Sprintf("Finansal veri kalitesi %.0f/100; rapor güveni düşer.", pro.DataQuality))
	}
	if strings.EqualFold(pro.Peers.ValuationSignal, "premium") {
		risks = append(risks, "Peer değerleme sinyali pahalı bölgeyi gösteriyor.")
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok && baseReturn < 0 {
		risks = append(risks, fmt.Sprintf("Temel senaryo getirisi %.1f%%; yukarı potansiyel değerleme ile desteklenmiyor.", baseReturn))
	}
	if netDebtEq := pro.Valuation.Ratios["NetDebt_Eq"]; netDebtEq > 1 {
		risks = append(risks, fmt.Sprintf("Net borç/özsermaye %.2f; bilanço riski yüksek.", netDebtEq))
	}
	return risks
}

func financialSummary(input DecisionRenderInput) string {
	if !hasProfessionalContext(input) {
		return "Finansal veri bağlı değil; karar yalnızca fiyat sinyaliyle güçlendirilemez."
	}
	pro := input.Professional
	if isCryptoProfessional(input) {
		items := []string{}
		if pro.DataQuality > 0 {
			items = append(items, fmt.Sprintf("kripto veri kapsamı %.0f/100", pro.DataQuality))
		}
		if len(pro.Coverage.Missing) > 0 {
			items = append(items, "eksik kaynak "+strings.Join(pro.Coverage.Missing, ", "))
		}
		if len(items) == 0 {
			return "Kripto veri kapsamı teknik karar için ek teyit üretmedi."
		}
		return "Kripto veri tarafında " + strings.Join(items, ", ") + "."
	}
	items := []string{}
	if pro.DataQuality > 0 {
		items = append(items, fmt.Sprintf("finansal veri kalitesi %.0f/100", pro.DataQuality))
	}
	if pro.Company.Sector != "" {
		items = append(items, fmt.Sprintf("sektör %s", pro.Company.Sector))
	}
	if pro.Peers.ValuationSignal != "" {
		items = append(items, fmt.Sprintf("peer değerleme sinyali %s", valuationSignalTR(pro.Peers.ValuationSignal)))
	}
	if roe := pro.Valuation.Ratios["ROE"]; roe != 0 {
		items = append(items, fmt.Sprintf("ROE %.1f%%", roe*100))
	}
	if baseReturn, ok := scenarioReturn(pro.Scenarios, "base"); ok {
		items = append(items, fmt.Sprintf("temel senaryo %.1f%%", baseReturn))
	}
	if len(items) == 0 {
		return "Finansal model karar için yeterli ek teyit üretmedi."
	}
	return "Finansal tarafta " + strings.Join(items, ", ") + "."
}

func isCryptoProfessional(input DecisionRenderInput) bool {
	pro := input.Professional
	return strings.EqualFold(pro.Company.Sector, "Crypto Assets") ||
		strings.EqualFold(pro.Valuation.SectorModel, "crypto_spot_technical_only")
}

func behavioralWaitReasons(input DecisionRenderInput) []string {
	behavioral := input.Behavioral
	if behavioral.Contrarian.Score <= 0 && behavioral.Capitulation.Score <= 0 {
		return nil
	}
	reasons := []string{}
	switch behavioral.Contrarian.Label {
	case "tersine_dönüş_adayı":
		reasons = append(reasons, "Olumsuz söylem ve dip bölgesi birlikte tersine dönüş adayı gösteriyor; momentum teyidi gelmeden alım sinyali sayılmaz.")
	case "dip_bölgesi_takip":
		reasons = append(reasons, "Fiyat dip bölgesi takibinde; alım için para akışı ve kapanış teyidi beklenmeli.")
	}
	if !behavioral.Contrarian.QualityGate.CanAffectBuySignal && behavioral.Contrarian.QualityGate.Reason != "" {
		reasons = append(reasons, behavioral.Contrarian.QualityGate.Reason)
	}
	return reasons
}

func behavioralRiskReasons(input DecisionRenderInput) []string {
	behavioral := input.Behavioral
	if behavioral.Contrarian.Score <= 0 && behavioral.Capitulation.Score <= 0 {
		return nil
	}
	risks := []string{}
	if behavioral.SourceCoverage.HasRecentSentiment && behavioral.Sentiment.Label == "olumsuz" {
		risks = append(risks, "Piyasa söylemi olumsuz; dip arayışı teyit olmadan erken alım riski taşır.")
	}
	if behavioral.Contrarian.QualityGate.Status != "" && behavioral.Contrarian.QualityGate.Status != "pass" {
		risks = append(risks, "Dip/sentiment sinyali kalite kapısından tam geçmedi; karar üzerindeki etkisi sınırlı.")
	}
	return risks
}

func behavioralSummary(input DecisionRenderInput) string {
	behavioral := input.Behavioral
	if behavioral.Contrarian.Score <= 0 && behavioral.Capitulation.Score <= 0 {
		return "Dip/sentiment modeli yeterli veri üretmedi."
	}
	return fmt.Sprintf("Dip/sentiment tarafında %s, skor %.0f/100; kalite kapısı %s.",
		behavioral.Contrarian.Signal,
		behavioral.Contrarian.Score,
		emptyFallback(behavioral.Contrarian.QualityGate.Status, "yok"),
	)
}

func valuationSignalTR(signal string) string {
	switch strings.ToLower(strings.TrimSpace(signal)) {
	case "discount":
		return "ucuz/iskontolu"
	case "premium":
		return "pahalı/primli"
	case "neutral":
		return "nötr"
	default:
		return "veri yetersiz"
	}
}

func professionalComment(input DecisionRenderInput, decision string, impact float64) string {
	financial := financialSummary(input)
	behavioral := behavioralSummary(input)
	if decision == "BEKLE" {
		if input.TradePlan.Direction == "short" {
			return fmt.Sprintf("Fiyat tablosu disiplinli alım için yeterli netlik üretmiyor. Sinyal etki oranı %.0f%%. Günlük taslak satış yönlü olduğu için aktif alım stopu veya hedefi üretilmez. %s %s Net giriş için direnç üstü kapanış, güçlü hacim ve MA20/EMA20 üstü teyit beklenmeli.", impact, financial, behavioral)
		}
		if spotLongUnavailablePlan(input.TradePlan) {
			return fmt.Sprintf("Fiyat tablosu disiplinli alım için yeterli netlik üretmiyor. Sinyal etki oranı %.0f%%; düşüş kanıtları spot varlıkta alım kurulumuna çevrilmez. %s %s Aktif alım stopu/hedefi yok; net giriş için direnç üstü kapanış, güçlü hacim ve MA20/EMA20 üstü teyit beklenmeli.", impact, financial, behavioral)
		}
		return fmt.Sprintf("Fiyat ve finansal tablo disiplinli alım için yeterli netlik üretmiyor. Sinyal etki oranı %.0f%% ve işlem planı teyit gerektiriyor. %s %s Net giriş için direnç üstü kapanış, güçlü hacim veya MA20/EMA20 üstü güçlü kapanış beklenmeli.", impact, financial, behavioral)
	}
	return fmt.Sprintf("Fiyat, risk/getiri ve finansal kalite birlikte %s sınıfında; sinyal etki oranı %.0f%%. %s %s Giriş seviyesi teyit sonrası izlenmeli; stop bozulursa senaryo geçersiz sayılmalı.", decision, impact, financial, behavioral)
}

func resultText(decision string, clearEntry bool) string {
	if decision == "BEKLE" || !clearEntry {
		return "Alım sinyali üretilmedi; teyit gelene kadar BEKLE kararı korunur."
	}
	return fmt.Sprintf("%s teknik olarak izlenebilir; giriş yalnızca teyit ve stop disipliniyle anlamlıdır.", decision)
}

func anyVolumeConfirmed(patterns []ohlcv.PatternResult) bool {
	for _, pattern := range patterns {
		if pattern.VolumeConfirmed {
			return true
		}
	}
	return false
}

func drawDecisionHeader(img *image.RGBA, fonts fontSet, palette decisionPalette, input DecisionRenderInput, evaluation decisionEvaluation) {
	title := fmt.Sprintf("%s Teknik Karar Raporu", input.Symbol)
	drawText(img, fonts.title, 64, 68, palette.text, title)
	subtitle := fmt.Sprintf("%s | %s | %s", emptyFallback(input.CompanyName, "Şirket adı yok"), localize.Timeframe(input.Timeframe), input.AnalysisDate)
	drawText(img, fonts.body, 68, 104, palette.muted, subtitle)

	rect := image.Rect(1348, 36, 1730, 98)
	drawRoundedBadge(img, rect, 8, evaluation.DecisionColor, evaluation.DecisionSoft)
	drawTextCentered(img, fonts.bold, rect.Min.X+rect.Dx()/2, 75, evaluation.DecisionColor, evaluation.Decision)
}

func drawDecisionMetrics(img *image.RGBA, fonts fontSet, palette decisionPalette, input DecisionRenderInput, evaluation decisionEvaluation) {
	metrics := []struct {
		label string
		value string
		color color.Color
	}{
		{"Son Kapanış", fmt.Sprintf("%.2f", input.LastClose), palette.text},
		{"Genel Yön", localize.Bias(input.OverallBias), palette.text},
		{"Teknik Skor", fmt.Sprintf("%.2f/100", input.TimeframeScore), palette.text},
		{"Sinyal Etki", fmt.Sprintf("%.0f%%", evaluation.SignalImpact), evaluation.DecisionColor},
	}
	x := 64
	for _, metric := range metrics {
		rect := image.Rect(x, 136, x+400, 232)
		fillRoundedRect(img, rect, 8, palette.panel)
		drawRoundedBadge(img, rect, 8, palette.grid, palette.panel)
		drawText(img, fonts.small, x+22, 172, palette.muted, metric.label)
		drawText(img, fonts.bold, x+22, 206, metric.color, metric.value)
		x += 424
	}
}

func drawDecisionGauge(img *image.RGBA, fonts fontSet, palette decisionPalette, evaluation decisionEvaluation) {
	rect := image.Rect(64, 258, 1730, 382)
	drawDecisionPanel(img, rect, palette)
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+38, palette.text, "Sinyal Etki Oranı")
	bar := image.Rect(rect.Min.X+24, rect.Min.Y+64, rect.Max.X-24, rect.Min.Y+82)
	fillRoundedRect(img, bar, 9, rgb(32, 38, 58))
	segments := []struct {
		max   float64
		color color.Color
	}{
		{39, palette.bad},
		{54, palette.warn},
		{64, rgb(91, 143, 249)},
		{79, palette.good},
		{100, rgb(38, 166, 154)},
	}
	start := bar.Min.X
	for _, segment := range segments {
		end := bar.Min.X + int(math.Round((segment.max/100)*float64(bar.Dx())))
		fillRoundedRect(img, image.Rect(start, bar.Min.Y, end, bar.Max.Y), 9, blendOnDark(segment.color, 0.75))
		start = end
	}
	pointerX := bar.Min.X + int(math.Round((evaluation.SignalImpact/100)*float64(bar.Dx())))
	drawLineWidth(img, pointerX, bar.Min.Y-8, pointerX, bar.Max.Y+14, 3, evaluation.DecisionColor)
	drawTextCentered(img, fonts.small, pointerX, bar.Min.Y-10, evaluation.DecisionColor, fmt.Sprintf("%.0f%%", evaluation.SignalImpact))
	drawText(img, fonts.small, rect.Min.X+24, rect.Max.Y-16, palette.muted, "0-39 Zayıf | 40-54 Riskli | 55-64 Normal AL adayı | 65-79 Güçlü AL adayı | 80-100 Mükemmel AL adayı")
}

func drawDecisionPlan(img *image.RGBA, fonts fontSet, palette decisionPalette, evaluation decisionEvaluation) {
	rect := image.Rect(64, 412, 586, 790)
	drawDecisionPanel(img, rect, palette)
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+38, palette.text, "Karar ve İşlem Planı")
	rows := []struct {
		label string
		value string
	}{
		{"Karar", evaluation.Decision},
		{"Plan durumu", evaluation.PlanStatus},
		{"Net alım", evaluation.EntryLevel},
		{"Alım şartı", evaluation.EntryTrigger},
		{evaluation.StopLabel, evaluation.StopLevel},
		{evaluation.Target1Label, evaluation.Target1},
		{evaluation.Target2Label, evaluation.Target2},
		{"Risk/getiri", evaluation.RiskReward},
	}
	y := rect.Min.Y + 76
	valueX := rect.Min.X + 210
	for _, row := range rows {
		drawText(img, fonts.small, rect.Min.X+24, y, palette.muted, row.label)
		y = drawDecisionWrappedText(img, fonts.small, valueX, y, rect.Max.X-valueX-24, palette.text, row.value, 18)
		y += 10
	}
}

func drawDecisionComponents(img *image.RGBA, fonts fontSet, palette decisionPalette, evaluation decisionEvaluation) {
	rect := image.Rect(618, 412, 1138, 790)
	drawDecisionPanel(img, rect, palette)
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+38, palette.text, "Sinyal Bileşenleri")
	y := rect.Min.Y + 78
	for _, component := range evaluation.Components {
		drawText(img, fonts.small, rect.Min.X+24, y, palette.text, component.Label)
		drawTextRight(img, fonts.small, rect.Max.X-24, y, palette.muted, fmt.Sprintf("%.1f / %.0f", component.Score, component.Weight))
		bar := image.Rect(rect.Min.X+24, y+10, rect.Max.X-24, y+19)
		fillRoundedRect(img, bar, 5, rgb(32, 38, 58))
		fillWidth := int(math.Round(safeDiv(component.Score, component.Weight) * float64(bar.Dx())))
		fillRoundedRect(img, image.Rect(bar.Min.X, bar.Min.Y, bar.Min.X+fillWidth, bar.Max.Y), 5, evaluation.DecisionColor)
		y += 38
	}
}

func drawDecisionReasons(img *image.RGBA, fonts fontSet, palette decisionPalette, evaluation decisionEvaluation) {
	left := image.Rect(1170, 412, 1730, 614)
	drawDecisionPanel(img, left, palette)
	marker := palette.good
	if evaluation.Decision == "BEKLE" {
		marker = palette.warn
	}
	drawText(img, fonts.bold, left.Min.X+24, left.Min.Y+38, palette.text, evaluation.ReasonsTitle)
	drawDecisionBullets(img, fonts.small, left.Min.X+24, left.Min.Y+70, left.Dx()-48, evaluation.ReasonsFor, marker, palette.text, 4)

	right := image.Rect(1170, 626, 1730, 834)
	drawDecisionPanel(img, right, palette)
	drawText(img, fonts.bold, right.Min.X+24, right.Min.Y+38, palette.text, "Riskler")
	drawDecisionBullets(img, fonts.small, right.Min.X+24, right.Min.Y+70, right.Dx()-48, evaluation.Risks, palette.bad, palette.text, 4)
}

func drawDecisionExitRules(img *image.RGBA, fonts fontSet, palette decisionPalette, evaluation decisionEvaluation) {
	rect := image.Rect(64, 858, 866, 1240)
	drawDecisionPanel(img, rect, palette)
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+38, palette.text, evaluation.ExitTitle)
	drawText(img, fonts.small, rect.Min.X+24, rect.Min.Y+66, palette.muted, evaluation.ExitIntro)
	drawDecisionBullets(img, fonts.small, rect.Min.X+24, rect.Min.Y+96, rect.Dx()-48, evaluation.ExitRules, palette.bad, palette.text, 6)
}

func drawDecisionComment(img *image.RGBA, fonts fontSet, palette decisionPalette, evaluation decisionEvaluation, disclaimer string) {
	rect := image.Rect(898, 858, 1730, 1240)
	drawDecisionPanel(img, rect, palette)
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+38, palette.text, "Profesyonel Yorum")
	y := drawDecisionWrappedText(img, fonts.body, rect.Min.X+24, rect.Min.Y+76, rect.Dx()-48, palette.text, evaluation.Comment, 26)
	y += 18
	drawText(img, fonts.bold, rect.Min.X+24, y, palette.text, "Sonuç")
	y = drawDecisionWrappedText(img, fonts.small, rect.Min.X+24, y+30, rect.Dx()-48, evaluation.DecisionColor, evaluation.Result, 21)
	y += 20
	if y <= rect.Max.Y-18 {
		drawDecisionWrappedText(img, fonts.small, rect.Min.X+24, y, rect.Dx()-48, palette.muted, emptyFallback(disclaimer, ohlcv.Disclaimer), 18)
	}
}

func drawDecisionPanel(img *image.RGBA, rect image.Rectangle, palette decisionPalette) {
	drawRoundedBadge(img, rect, 8, palette.grid, palette.panel)
}

func drawDecisionBullets(img *image.RGBA, face font.Face, x, y, width int, items []string, markerColor, textColor color.Color, limit int) int {
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	cursor := y
	for i := 0; i < limit; i++ {
		fillRoundedRect(img, image.Rect(x, cursor-10, x+7, cursor-3), 3, markerColor)
		cursor = drawDecisionWrappedText(img, face, x+18, cursor, width-18, textColor, items[i], 18)
		cursor += 8
	}
	return cursor
}

func drawDecisionWrappedText(img *image.RGBA, face font.Face, x, y, maxWidth int, c color.Color, text string, lineHeight int) int {
	words := stringsFields(text)
	line := ""
	cursor := y
	for _, word := range words {
		next := stringsTrimSpace(line + " " + word)
		if line != "" && font.MeasureString(face, next).Ceil() > maxWidth {
			drawText(img, face, x, cursor, c, line)
			cursor += lineHeight
			line = word
			continue
		}
		line = next
	}
	if line != "" {
		drawText(img, face, x, cursor, c, line)
		cursor += lineHeight
	}
	return cursor
}

func tradePlanRejectDisplay(plan ohlcv.TradePlan) string {
	if spotLongUnavailablePlan(plan) {
		return "Aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor."
	}
	return localize.Reason(plan.RejectReason)
}

func spotLongUnavailablePlan(plan ohlcv.TradePlan) bool {
	reason := strings.ToLower(localize.Reason(plan.RejectReason) + " " + plan.RejectReason + " " + strings.Join(plan.Reasoning, " "))
	return strings.Contains(reason, "short") ||
		strings.Contains(reason, "marjin") ||
		strings.Contains(reason, "spot varlık") ||
		strings.Contains(reason, "spot varlik") ||
		strings.Contains(reason, "spot long") ||
		strings.Contains(reason, "düşüş sinyali") ||
		strings.Contains(reason, "dusus sinyali") ||
		strings.Contains(reason, "düşüş kanıt") ||
		strings.Contains(reason, "dusus kanit") ||
		strings.Contains(reason, "bearish evidence")
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func safeDiv(numerator, denominator float64) float64 {
	if math.Abs(denominator) < 0.000001 {
		return 0
	}
	return numerator / denominator
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
