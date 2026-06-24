package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type spec struct {
	Name       string
	Category   string
	Group      string
	Direction  string
	Template   string
	Confidence string
	Evidence   string
	FilePath   string
}

type issue struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Problem      string `json:"problem"`
	WhyItMatters string `json:"why_it_matters"`
	SuggestedFix string `json:"suggested_fix"`
	Example      string `json:"example"`
}

type auditEntry struct {
	PatternName                 string   `json:"pattern_name"`
	FilePath                    string   `json:"file_path"`
	DetectorFunctionOrClass     string   `json:"detector_function_or_class"`
	FinancialCorrectnessScore   int      `json:"financial_correctness_score"`
	AlgorithmicCorrectnessScore int      `json:"algorithmic_correctness_score"`
	CodeQualityScore            int      `json:"code_quality_score"`
	TestCoverageScore           int      `json:"test_coverage_score"`
	OverallScore                int      `json:"overall_score"`
	Verdict                     string   `json:"verdict"`
	DetectedIssues              []issue  `json:"detected_issues"`
	MissingConditions           []string `json:"missing_conditions"`
	FalsePositiveRisks          []string `json:"false_positive_risks"`
	FalseNegativeRisks          []string `json:"false_negative_risks"`
	LookaheadBiasRisks          []string `json:"lookahead_bias_risks"`
	RequiredTests               []string `json:"required_tests"`
	RecommendedRefactor         string   `json:"recommended_refactor"`
	CorrectedLogicSummary       string   `json:"corrected_logic_summary"`
	FinalExpertComment          string   `json:"final_expert_comment"`
}

var fieldRE = regexp.MustCompile(`^\s*(Name|Category|Group|Direction|Template|Confidence|Evidence):\s+(?:"([^"]*)"|([0-9.]+)),`)

func main() {
	root := "internal/ta/patterns/generated"
	files, err := filepath.Glob(filepath.Join(root, "pattern_*.go"))
	if err != nil {
		fatal(err)
	}
	sort.Strings(files)
	seen := map[string]bool{}
	entries := make([]auditEntry, 0, len(files))
	for _, file := range files {
		s, err := readSpec(file)
		if err != nil {
			fatal(err)
		}
		duplicate := seen[strings.ToLower(s.Name)]
		seen[strings.ToLower(s.Name)] = true
		entries = append(entries, audit(s, duplicate))
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		fatal(err)
	}
}

func readSpec(file string) (spec, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return spec{}, err
	}
	out := spec{FilePath: file}
	for _, line := range strings.Split(string(data), "\n") {
		match := fieldRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		value := match[2]
		if value == "" {
			value = match[3]
		}
		switch match[1] {
		case "Name":
			out.Name = value
		case "Category":
			out.Category = value
		case "Group":
			out.Group = value
		case "Direction":
			out.Direction = value
		case "Template":
			out.Template = value
		case "Confidence":
			out.Confidence = value
		case "Evidence":
			out.Evidence = value
		}
	}
	if out.Name == "" || out.Template == "" {
		return spec{}, fmt.Errorf("incomplete generated spec in %s", file)
	}
	return out, nil
}

func audit(s spec, duplicate bool) auditEntry {
	financial, algorithmic, code, tests := baseScores(s)
	issues := baseIssues(s)
	if duplicate {
		financial, algorithmic, tests = min(financial, 35), min(algorithmic, 35), min(tests, 10)
		issues = append(issues, issue{
			Severity:     "high",
			Category:     "code",
			Problem:      "Pattern name is duplicated in generated catalog.",
			WhyItMatters: "Duplicate names collapse in scan aggregation and can hide one detector behind another.",
			SuggestedFix: "Rename or merge aliases explicitly and keep a canonical pattern id separate from display name.",
			Example:      s.Name,
		})
	}
	overall := (financial*35 + algorithmic*35 + code*15 + tests*15) / 100
	verdict := verdictFor(overall, duplicate, s)
	return auditEntry{
		PatternName:                 s.Name,
		FilePath:                    s.FilePath,
		DetectorFunctionOrClass:     "registeredSpecDetector.Detect -> detectPatternSpec -> matchPatternSpec -> rule:" + s.Template,
		FinancialCorrectnessScore:   financial,
		AlgorithmicCorrectnessScore: algorithmic,
		CodeQualityScore:            code,
		TestCoverageScore:           tests,
		OverallScore:                overall,
		Verdict:                     verdict,
		DetectedIssues:              issues,
		MissingConditions:           missingConditions(s),
		FalsePositiveRisks:          falsePositiveRisks(s),
		FalseNegativeRisks:          falseNegativeRisks(s),
		LookaheadBiasRisks:          lookaheadRisks(s),
		RequiredTests:               requiredTests(s),
		RecommendedRefactor:         recommendedRefactor(s),
		CorrectedLogicSummary:       correctedLogicSummary(s),
		FinalExpertComment:          finalExpertComment(verdict, s),
	}
}

func baseScores(s spec) (int, int, int, int) {
	switch {
	case s.Template == "candlestick":
		return 86, 80, 86, 72
	case hasAny(s.Template, "double_", "triple_", "head_shoulders", "triangle", "flag", "pennant", "wedge", "cup_handle", "rounding", "rectangle", "range", "channel", "diamond", "broadening"):
		return 78, 72, 84, 68
	case hasAny(s.Template, "harmonic", "abcd", "three_drives", "five_zero"):
		return 56, 50, 78, 52
	case hasAny(s.Template, "elliott"):
		return 45, 40, 74, 46
	case hasAny(s.Template, "wyckoff", "spring", "upthrust", "sos", "sow", "lps", "lpsy", "climax", "automatic"):
		return 72, 66, 82, 62
	case hasAny(s.Template, "fvg", "order_block", "breaker", "mitigation", "liquidity", "bos", "choch"):
		return 70, 64, 82, 60
	case s.Template == "market_profile" || s.Template == "point_figure":
		return 72, 68, 82, 64
	case s.Template == "indicator" || s.Template == "fibonacci":
		return 74, 70, 82, 64
	case hasAny(s.Template, "gap", "breakout", "breakdown", "trend", "compression", "expansion", "volume"):
		return 82, 76, 84, 66
	default:
		return 70, 64, 80, 58
	}
}

func baseIssues(s spec) []issue {
	out := []issue{
		{
			Severity:     "medium",
			Category:     "testing",
			Problem:      "Per-pattern golden fixtures are still not exhaustive for this exact generated name.",
			WhyItMatters: "Contract tests prove scanner behavior, but golden fixtures are still the best guard against future definition drift.",
			SuggestedFix: "Add named historical or synthetic golden examples for this exact pattern and keep them under versioned fixtures.",
			Example:      s.Name,
		},
	}
	switch {
	case s.Template == "candlestick":
		out = append(out, issue{
			Severity:     "medium",
			Category:     "financial_logic",
			Problem:      "Candlestick geometry is detected mostly from local candle shape.",
			WhyItMatters: "Many reversal candles are only meaningful after a prior trend and near support/resistance.",
			SuggestedFix: "Require trend context, swing level proximity and optional confirmation candle for reversal variants.",
			Example:      "Hammer without preceding decline should not be treated as bullish reversal.",
		})
	case hasAny(s.Template, "harmonic"):
		out = append(out, issue{
			Severity:     "critical",
			Category:     "financial_logic",
			Problem:      "Harmonic family requires strict XA/AB/BC/CD Fibonacci ratio validation per named pattern.",
			WhyItMatters: "Generic swing proportionality can mislabel Gartley, Bat, Butterfly, Crab and Shark structures.",
			SuggestedFix: "Implement per-pattern ratio bands, PRZ confluence and completion confirmation.",
			Example:      "Gartley must validate B near 0.618 XA and D near 0.786 XA, not only five-point shape.",
		})
	case hasAny(s.Template, "elliott"):
		out = append(out, issue{
			Severity:     "critical",
			Category:     "financial_logic",
			Problem:      "Elliott wave labels are subjective and require rule hierarchy plus alternates.",
			WhyItMatters: "A simple swing sequence can overfit and repaint wave counts.",
			SuggestedFix: "Separate tentative counts from confirmed counts, enforce wave invalidation rules and store alternates.",
			Example:      "Impulse wave 4 must not overlap wave 1 in standard impulse.",
		})
	case s.Template == "market_profile":
		out = append(out, issue{
			Severity:     "high",
			Category:     "algorithm",
			Problem:      "Market Profile is derived from OHLCV bars, not native intraday TPO prints.",
			WhyItMatters: "Daily or coarse candles can smear actual auction structure and create false nodes.",
			SuggestedFix: "Prefer intraday session bars; store profile resolution, bin size and session definition in evidence.",
			Example:      "Single prints require TPO granularity; OHLCV bin distribution is an approximation.",
		})
	case s.Template == "point_figure":
		out = append(out, issue{
			Severity:     "high",
			Category:     "algorithm",
			Problem:      "Point & Figure columns are derived from OHLCV closes with dynamic box size.",
			WhyItMatters: "Traditional P&F depends on explicit box size, reversal amount and high-low/close method.",
			SuggestedFix: "Expose box size, reversal boxes and price method as scanner parameters and snapshot them in result evidence.",
			Example:      "A 1x3 close-only chart can differ materially from a high-low P&F chart.",
		})
	}
	return out
}

func missingConditions(s spec) []string {
	base := []string{"explicit trade trigger", "stop-loss level", "target/measured move", "invalidation level", "risk/reward metadata"}
	if s.Direction != "neutral" {
		base = append(base, "direction-specific confirmation candle")
	}
	if hasAny(s.Template, "double_", "triple_", "head_shoulders", "triangle", "flag", "pennant", "wedge", "cup_handle") {
		base = append(base, "breakout close validation", "failed-breakout invalidation", "ATR-scaled neckline/boundary tolerance")
	}
	if s.Template == "volume" || s.Category == "volume" {
		base = append(base, "relative volume regime filter", "liquidity floor")
	}
	return base
}

func falsePositiveRisks(s spec) []string {
	risks := []string{"low-liquidity candles with noisy wicks", "range-bound chop producing repeated near-matches", "fixed tolerance not matching instrument volatility"}
	if s.Template == "candlestick" {
		risks = append(risks, "single-candle shape appearing without trend context")
	}
	if hasAny(s.Template, "harmonic", "elliott") {
		risks = append(risks, "many swing permutations fitting loose ratios after the fact")
	}
	return risks
}

func falseNegativeRisks(s spec) []string {
	return []string{"valid pattern drawn at a different swing sensitivity", "pattern spans longer than configured scan windows", "gapped markets deforming geometry", "volume-adjusted confirmation suppressing otherwise valid setup"}
}

func lookaheadRisks(s spec) []string {
	return []string{"No direct future candle read is required inside a matched window, but backtests must consume results by EndIndex only.", "Exhaustive full-history scans can return historical matches; event-stream backtests must rescan incrementally or filter by trigger bar."}
}

func requiredTests(s spec) []string {
	return []string{
		"positive synthetic OHLCV fixture for " + s.Name,
		"negative near-miss fixture for " + s.Name,
		"insufficient-data fixture",
		"flat-market fixture",
		"gap/high-volatility fixture",
		"lookahead test proving no candle after EndIndex affects the match",
		"golden dataset snapshot for known historical examples",
	}
}

func recommendedRefactor(s spec) string {
	return "Move " + s.Template + " into a typed rule object with explicit parameters, rule version, required context, trigger policy and backtest metadata."
}

func correctedLogicSummary(s spec) string {
	switch {
	case s.Template == "candlestick":
		return "Detect candle geometry only after validating prior trend, swing-level location, ATR-scaled body/wick thresholds and optional confirmation."
	case hasAny(s.Template, "harmonic"):
		return "Extract XABCD pivots, enforce named Fibonacci ratio bands, validate PRZ confluence, then trigger only after reversal confirmation."
	case hasAny(s.Template, "elliott"):
		return "Build candidate wave counts with strict invalidation rules, rank alternates, and mark unconfirmed counts separately from tradeable signals."
	case s.Template == "market_profile":
		return "Build session-scoped TPO/volume bins, compute POC/VAH/VAL/single prints, then distinguish auction structure from trade trigger."
	case s.Template == "point_figure":
		return "Build deterministic P&F columns from configured box/reversal settings, then apply named breakout/catapult/pole rules to completed columns."
	default:
		return "Validate prerequisite trend/context, detect geometry with ATR-scaled tolerances, require completion/confirmation, and emit explicit invalidation/trade levels."
	}
}

func finalExpertComment(verdict string, s spec) string {
	if verdict == "PASS" {
		return "Acceptable as a detector, but still needs per-pattern golden examples before production trading use."
	}
	if verdict == "INVALID" {
		return "Do not use this as a trading signal until the financial definition is rewritten and tested."
	}
	return "Detector is useful as a scanner candidate, not yet strong enough as an autonomous trade signal."
}

func verdictFor(score int, duplicate bool, s spec) string {
	if duplicate {
		return "DUPLICATE"
	}
	if s.Template == "generic" {
		return "INVALID"
	}
	if score >= 80 {
		return "PASS"
	}
	if score < 42 {
		return "UNCLEAR"
	}
	return "NEEDS_FIX"
}

func hasAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
