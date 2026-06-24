package reportaudit

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

type Options struct {
	SpotOnly bool
}

type Report struct {
	Status            string  `json:"status"`
	Symbol            string  `json:"symbol"`
	CheckedTimeframes int     `json:"checked_timeframes"`
	Issues            []Issue `json:"issues"`
}

type Issue struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

type analysisFile struct {
	Symbol                  string                       `json:"symbol"`
	Timeframes              map[string]timeframeAnalysis `json:"timeframes"`
	Professional            professionalAnalysis         `json:"professional"`
	InstitutionalValidation institutionalValidation      `json:"institutional_validation"`
}

type professionalAnalysis struct {
	Company companyProfile `json:"company"`
	Peers   peerComparison `json:"peer_comparison"`
	Quality float64        `json:"data_quality"`
}

type institutionalValidation struct {
	Status string                    `json:"status"`
	Score  float64                   `json:"score"`
	Checks []institutionalAuditCheck `json:"checks"`
}

type institutionalAuditCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type companyProfile struct {
	Sector       string `json:"sector"`
	SectorSource string `json:"sector_source"`
}

type peerComparison struct {
	Sector    string       `json:"sector"`
	PeerCount int          `json:"peer_count"`
	Peers     []peerMetric `json:"peers"`
}

type peerMetric struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

type timeframeAnalysis struct {
	LastClose         float64                  `json:"last_close"`
	SupportLevels     []supportResistanceLevel `json:"support_levels"`
	ResistanceLevels  []supportResistanceLevel `json:"resistance_levels"`
	NearestSupport    *supportResistanceLevel  `json:"nearest_support"`
	NearestResistance *supportResistanceLevel  `json:"nearest_resistance"`
	TradePlan         tradePlan                `json:"trade_plan"`
}

type supportResistanceLevel struct {
	Type  string  `json:"type"`
	Price float64 `json:"price"`
}

type tradePlan struct {
	Direction       string  `json:"direction"`
	EntryMin        float64 `json:"entry_min"`
	EntryMax        float64 `json:"entry_max"`
	TakeProfit1     float64 `json:"take_profit1"`
	TakeProfit2     float64 `json:"take_profit2"`
	StopLoss        float64 `json:"stop_loss"`
	RiskRewardRatio float64 `json:"risk_reward_ratio"`
	Rejected        bool    `json:"rejected"`
	RejectReason    string  `json:"reject_reason"`
}

func ValidateFile(path string, options Options) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("read analysis file: %w", err)
	}
	return ValidateJSON(data, options)
}

func ValidateJSON(data []byte, options Options) (Report, error) {
	var analysis analysisFile
	if err := json.Unmarshal(data, &analysis); err != nil {
		return Report{}, fmt.Errorf("decode analysis json: %w", err)
	}
	report := Report{Status: "pass", Symbol: analysis.Symbol}
	if analysis.Symbol == "" {
		report.add("fail", "symbol", "symbol is empty")
	}
	if len(analysis.Timeframes) == 0 {
		report.add("fail", "timeframes", "no timeframes found")
	}
	for timeframe, tf := range analysis.Timeframes {
		report.CheckedTimeframes++
		validateTimeframe(&report, timeframe, tf, options)
	}
	validateProfessional(&report, analysis)
	validateInstitutional(&report, analysis.InstitutionalValidation)
	if report.hasFailures() {
		report.Status = "fail"
	}
	return report, nil
}

func validateInstitutional(report *Report, validation institutionalValidation) {
	if strings.TrimSpace(validation.Status) == "" {
		report.add("fail", "institutional_validation", "institutional validation section is missing")
		return
	}
	switch strings.ToLower(strings.TrimSpace(validation.Status)) {
	case "pass":
	case "limited":
		report.add("warn", "institutional_validation.status", "institutional validation is limited; report must not be used as a production trading system")
	case "fail":
		report.add("fail", "institutional_validation.status", "institutional validation failed")
	default:
		report.add("fail", "institutional_validation.status", "unknown institutional validation status")
	}
	if validation.Score < 80 {
		report.add("fail", "institutional_validation.score", "institutional validation score is below 80")
	}
	if len(validation.Checks) == 0 {
		report.add("fail", "institutional_validation.checks", "institutional validation has no checks")
	}
	for i, check := range validation.Checks {
		if strings.TrimSpace(check.Name) == "" || strings.TrimSpace(check.Status) == "" {
			report.add("fail", fmt.Sprintf("institutional_validation.checks.%d", i), "institutional validation check is incomplete")
			continue
		}
		if strings.EqualFold(check.Status, "fail") {
			report.add("fail", fmt.Sprintf("institutional_validation.checks.%d.%s", i, check.Name), check.Message)
		}
	}
}

func validateProfessional(report *Report, analysis analysisFile) {
	pro := analysis.Professional
	if pro.Company.Sector == "" && pro.Peers.Sector == "" && pro.Quality == 0 {
		report.add("fail", "professional", "professional financial section is missing")
		return
	}
	sector := pro.Company.Sector
	if sector == "" {
		sector = pro.Peers.Sector
	}
	if sector == "Gayrimenkul Yatırım Ortaklığı" && !isREITSymbol(analysis.Symbol) {
		report.add("fail", "professional.company.sector", "non-GYO symbol classified as real-estate investment trust")
	}
	if sector != "" && pro.Peers.Sector != "" && sector != pro.Peers.Sector {
		report.add("fail", "professional.peer_comparison.sector", "company sector and peer sector do not match")
	}
	if sector != "Gayrimenkul Yatırım Ortaklığı" {
		for i, peer := range pro.Peers.Peers {
			if isREITSymbol(peer.Symbol) || containsREITName(peer.Name) {
				report.add("fail", fmt.Sprintf("professional.peer_comparison.peers.%d", i), "non-GYO peer set contains a GYO peer")
			}
		}
	}
	if pro.Quality > 0 && pro.Quality < 65 {
		report.add("fail", "professional.data_quality", "financial data quality is below report threshold")
	}
}

func validateTimeframe(report *Report, timeframe string, tf timeframeAnalysis, options Options) {
	base := "timeframes." + timeframe
	if tf.LastClose <= 0 || math.IsNaN(tf.LastClose) || math.IsInf(tf.LastClose, 0) {
		report.add("fail", base+".last_close", "last close must be positive")
		return
	}
	for i, level := range tf.SupportLevels {
		path := fmt.Sprintf("%s.support_levels.%d", base, i)
		if level.Type != "support" {
			report.add("fail", path+".type", "support level has wrong type")
		}
		if level.Price <= 0 || level.Price >= tf.LastClose {
			report.add("fail", path+".price", "support must be positive and below last close")
		}
	}
	for i, level := range tf.ResistanceLevels {
		path := fmt.Sprintf("%s.resistance_levels.%d", base, i)
		if level.Type != "resistance" {
			report.add("fail", path+".type", "resistance level has wrong type")
		}
		if level.Price <= tf.LastClose {
			report.add("fail", path+".price", "resistance must be above last close")
		}
	}
	if tf.NearestSupport != nil && tf.NearestSupport.Price >= tf.LastClose {
		report.add("fail", base+".nearest_support.price", "nearest support must be below last close")
	}
	if tf.NearestResistance != nil && tf.NearestResistance.Price <= tf.LastClose {
		report.add("fail", base+".nearest_resistance.price", "nearest resistance must be above last close")
	}
	validateTradePlan(report, base+".trade_plan", tf.TradePlan, options)
}

func validateTradePlan(report *Report, path string, plan tradePlan, options Options) {
	if options.SpotOnly && plan.Direction == "short" {
		report.add("fail", path+".direction", "spot-only analysis must not expose a short trade plan")
	}
	if plan.Direction == "neutral" {
		if hasActiveLevels(plan) {
			report.add("fail", path, "neutral plan must not expose active entry, target or stop levels")
		}
		return
	}
	if plan.Direction != "long" && plan.Direction != "short" && plan.Direction != "" {
		report.add("fail", path+".direction", "unknown trade plan direction")
	}
	if plan.Rejected {
		return
	}
	if plan.Direction == "long" {
		validateLongPlan(report, path, plan)
	}
	if plan.Direction == "short" {
		validateShortPlan(report, path, plan)
	}
}

func validateLongPlan(report *Report, path string, plan tradePlan) {
	if plan.EntryMin <= 0 || plan.EntryMax <= 0 || plan.StopLoss <= 0 || plan.TakeProfit1 <= 0 || plan.TakeProfit2 <= 0 {
		report.add("fail", path, "active long plan must have positive entry, target and stop levels")
		return
	}
	if plan.EntryMin > plan.EntryMax {
		report.add("fail", path, "entry min exceeds entry max")
	}
	if plan.StopLoss >= plan.EntryMin {
		report.add("fail", path+".stop_loss", "long stop must be below entry")
	}
	if plan.TakeProfit1 <= plan.EntryMax || plan.TakeProfit2 <= plan.TakeProfit1 {
		report.add("fail", path, "long targets must be above entry and ordered")
	}
	validateRiskReward(report, path, plan)
}

func validateShortPlan(report *Report, path string, plan tradePlan) {
	if plan.EntryMin <= 0 || plan.EntryMax <= 0 || plan.StopLoss <= 0 || plan.TakeProfit1 <= 0 || plan.TakeProfit2 <= 0 {
		report.add("fail", path, "active short plan must have positive entry, target and stop levels")
		return
	}
	if plan.EntryMin > plan.EntryMax {
		report.add("fail", path, "entry min exceeds entry max")
	}
	if plan.StopLoss <= plan.EntryMax {
		report.add("fail", path+".stop_loss", "short stop must be above entry")
	}
	if plan.TakeProfit1 >= plan.EntryMin || plan.TakeProfit2 >= plan.TakeProfit1 {
		report.add("fail", path, "short targets must be below entry and ordered")
	}
	validateRiskReward(report, path, plan)
}

func validateRiskReward(report *Report, path string, plan tradePlan) {
	entry := (plan.EntryMin + plan.EntryMax) / 2
	risk := math.Abs(entry - plan.StopLoss)
	reward := math.Abs(plan.TakeProfit1 - entry)
	if risk <= 0 {
		report.add("fail", path+".risk_reward_ratio", "risk denominator must be positive")
		return
	}
	expected := reward / risk
	if math.Abs(expected-plan.RiskRewardRatio) > 0.05 {
		report.add("fail", path+".risk_reward_ratio", "risk/reward ratio does not match entry, target and stop")
	}
}

func hasActiveLevels(plan tradePlan) bool {
	return plan.EntryMin > 0 || plan.EntryMax > 0 || plan.TakeProfit1 > 0 || plan.TakeProfit2 > 0 || plan.StopLoss > 0
}

func (report *Report) add(severity, path, message string) {
	report.Issues = append(report.Issues, Issue{Severity: severity, Path: path, Message: message})
}

func (report Report) hasFailures() bool {
	for _, issue := range report.Issues {
		if issue.Severity == "fail" {
			return true
		}
	}
	return false
}

func isREITSymbol(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	return strings.HasSuffix(symbol, "GYO") || strings.Contains(symbol, "GMYO")
}

func containsREITName(name string) bool {
	upper := strings.ToUpper(name)
	return strings.Contains(upper, "GAYRIMENKUL YATIRIM ORTAK") || strings.Contains(upper, "GAYRİMENKUL YATIRIM ORTAK")
}
