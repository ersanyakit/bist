package patterns

import (
	"context"
	"sort"
	"strings"

	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/patterns/generated"
)

type patternSpec struct {
	Name       string
	Category   string
	Group      string
	Direction  string
	Template   string
	Rule       patternRuleID
	Confidence float64
	Evidence   string
}

type patternRuleOverride struct {
	template  string
	direction string
	evidence  string
}

var registeredSpecs []patternSpec

var exactPatternRuleOverrides = map[string]patternRuleOverride{
	"absorption":                    {template: "volume", evidence: "absorption volume-spread behavior matched"},
	"amd pattern":                   {template: "amd", evidence: "accumulation-manipulation-distribution sequence matched"},
	"back up to the creek":          {template: "lps", direction: "bullish", evidence: "Wyckoff creek retest holds after a strength breakout"},
	"back up to the ice":            {template: "lpsy", direction: "bearish", evidence: "Wyckoff ice retest fails after a weakness breakdown"},
	"blow off top":                  {template: "buying_climax", direction: "bearish", evidence: "exhaustive high-volume advance matched"},
	"bos":                           {template: "bos", evidence: "break of structure rule matched"},
	"break and retest":              {template: "break_retest", evidence: "structure break and retest rule matched"},
	"buying exhaustion":             {template: "buying_climax", direction: "bearish", evidence: "buying exhaustion volume-spread rule matched"},
	"cause and effect":              {template: "range", evidence: "cause-building trading range matched"},
	"cheat area":                    {template: "compression", evidence: "tight constructive compression rule matched"},
	"composite operator":            {template: "wyckoff_composite", evidence: "Wyckoff composite-operator range behavior matched"},
	"creek":                         {template: "sos", direction: "bullish", evidence: "Wyckoff creek breakout rule matched"},
	"dead cat bounce":               {template: "dead_cat_bounce", direction: "bearish", evidence: "weak rebound after sharp decline matched"},
	"demand zone rejection":         {template: "spring", direction: "bullish", evidence: "demand-zone sweep and rejection matched"},
	"ascending scallop":             {template: "rounding_bottom", direction: "bullish", evidence: "ascending scallop rounded-bottom behavior matched"},
	"discount zone":                 {template: "discount_zone", direction: "bullish", evidence: "price is in the discount zone of the recent dealing range"},
	"descending scallop":            {template: "rounding_top", direction: "bearish", evidence: "descending scallop rounded-top behavior matched"},
	"equilibrium seviyesi":          {template: "equilibrium_zone", direction: "neutral", evidence: "price is near the recent equilibrium zone"},
	"external liquidity":            {template: "liquidity_sweep", evidence: "external liquidity sweep rule matched"},
	"fakey pattern":                 {template: "fakey", evidence: "inside-bar false break rule matched"},
	"failed move":                   {template: "false_breakout", evidence: "failed move back inside the range matched"},
	"fall through the ice":          {template: "sow", direction: "bearish", evidence: "Wyckoff ice breakdown rule matched"},
	"high resistance liquidity run": {template: "liquidity_sweep", evidence: "high-resistance liquidity-run rule matched"},
	"hrlr":                          {template: "liquidity_sweep", evidence: "high-resistance liquidity-run rule matched"},
	"ice":                           {template: "sow", direction: "bearish", evidence: "Wyckoff ice support-loss rule matched"},
	"idm":                           {template: "liquidity_sweep", evidence: "inducement liquidity sweep rule matched"},
	"inducement":                    {template: "liquidity_sweep", evidence: "inducement liquidity sweep rule matched"},
	"inside bar fakey":              {template: "fakey", evidence: "inside-bar false break rule matched"},
	"inside bar range":              {template: "mother_bar", evidence: "mother-bar/inside-bar range rule matched"},
	"internal liquidity":            {template: "liquidity_sweep", evidence: "internal liquidity sweep rule matched"},
	"inverted ascending scallop":    {template: "rounding_top", direction: "bearish", evidence: "inverted ascending scallop rounded-top behavior matched"},
	"inverted descending scallop":   {template: "rounding_bottom", direction: "bullish", evidence: "inverted descending scallop rounded-bottom behavior matched"},
	"inverted roof pattern":         {template: "rounding_bottom", direction: "bullish", evidence: "inverted roof rounded-bottom rule matched"},
	"jump across the creek":         {template: "sos", direction: "bullish", evidence: "Wyckoff jump-across-the-creek breakout matched"},
	"low resistance liquidity run":  {template: "liquidity_sweep", evidence: "low-resistance liquidity-run rule matched"},
	"lps":                           {template: "lps", direction: "bullish", evidence: "last point of support rule matched"},
	"lrlr":                          {template: "liquidity_sweep", evidence: "low-resistance liquidity-run rule matched"},
	"mean reversion setup":          {template: "mean_reversion", evidence: "mean reversion rule matched"},
	"mother bar":                    {template: "mother_bar", evidence: "mother-bar containment rule matched"},
	"optimal trade entry":           {template: "fibonacci", evidence: "optimal-trade-entry Fibonacci zone matched"},
	"ote":                           {template: "fibonacci", evidence: "optimal-trade-entry Fibonacci zone matched"},
	"over and under pattern":        {template: "quasimodo", evidence: "over-and-under swing failure rule matched"},
	"overthrow":                     {template: "upthrust", direction: "bearish", evidence: "overthrow above resistance rejected"},
	"pocket pivot":                  {template: "pocket_pivot", direction: "bullish", evidence: "pocket-pivot volume breakout rule matched"},
	"power of three":                {template: "amd", evidence: "accumulation-manipulation-distribution sequence matched"},
	"preliminary supply":            {template: "buying_climax", direction: "bearish", evidence: "preliminary supply volume-spread rule matched"},
	"preliminary support":           {template: "selling_climax", direction: "bullish", evidence: "preliminary support volume-spread rule matched"},
	"premium zone":                  {template: "premium_zone", direction: "bearish", evidence: "price is in the premium zone of the recent dealing range"},
	"ps":                            {template: "selling_climax", direction: "bullish", evidence: "preliminary support volume-spread rule matched"},
	"psy":                           {template: "buying_climax", direction: "bearish", evidence: "preliminary supply volume-spread rule matched"},
	"pullback":                      {template: "pullback", evidence: "trend pullback rule matched"},
	"quasimodo pattern":             {template: "quasimodo", evidence: "Quasimodo swing failure rule matched"},
	"roof pattern":                  {template: "rounding_top", direction: "bearish", evidence: "roof rounded-top rule matched"},
	"rounded reversal":              {template: "rounded_reversal", evidence: "rounded reversal rule matched"},
	"secondary test":                {template: "secondary_test", evidence: "Wyckoff secondary-test rule matched"},
	"selling exhaustion":            {template: "selling_climax", direction: "bullish", evidence: "selling exhaustion volume-spread rule matched"},
	"shakeout":                      {template: "spring", direction: "bullish", evidence: "shakeout below support reclaimed"},
	"silver bullet setup":           {template: "fvg", evidence: "fair-value-gap displacement rule matched"},
	"st":                            {template: "secondary_test", evidence: "Wyckoff secondary-test rule matched"},
	"stair step pattern":            {template: "stair_step", evidence: "stair-step trend structure matched"},
	"supply zone rejection":         {template: "upthrust", direction: "bearish", evidence: "supply-zone sweep and rejection matched"},
	"support resistance flip":       {template: "break_retest", evidence: "support/resistance flip retest rule matched"},
	"three falling peaks":           {template: "trend_down", direction: "bearish", evidence: "successive falling peaks matched"},
	"three rising valleys":          {template: "trend_up", direction: "bullish", evidence: "successive rising valleys matched"},
	"throwback":                     {template: "break_retest", direction: "bullish", evidence: "throwback retest after breakout matched"},
	"throwover":                     {template: "upthrust", direction: "bearish", evidence: "throwover above resistance rejected"},
	"trend exhaustion":              {template: "trend_exhaustion", evidence: "trend exhaustion rule matched"},
	"trendline break":               {template: "trendline_break", evidence: "trendline break rule matched"},
	"trendline retest":              {template: "break_retest", evidence: "trendline retest rule matched"},
	"undercut":                      {template: "spring", direction: "bullish", evidence: "undercut below support reclaimed"},
	"undercut and rally":            {template: "spring", direction: "bullish", evidence: "undercut-and-rally rule matched"},
	"wedge":                         {template: "wedge", evidence: "rising or falling wedge rule matched"},
	"wedge continuation":            {template: "wedge", evidence: "continuation wedge compression rule matched"},
}

func registerPatternSpec(spec patternSpec) {
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return
	}
	if spec.Category == "" {
		spec.Category = "pattern"
	}
	if spec.Direction == "" {
		spec.Direction = "neutral"
	}
	if spec.Template == "" {
		spec.Template = "generic"
	}
	if spec.Confidence <= 0 {
		spec.Confidence = 0.62
	}
	if spec.Evidence == "" {
		spec.Evidence = "pattern-specific scanner rule matched current market structure"
	}
	spec = applyPatternRuleOverrides(spec)
	spec = bindPatternRule(spec)
	registeredSpecs = append(registeredSpecs, spec)
}

func registeredPatternDetectors() []PatternDetector {
	specs := registeredPatternSpecs()
	detectors := make([]PatternDetector, 0, len(specs))
	for _, spec := range specs {
		detectors = append(detectors, registeredSpecDetector{spec: spec})
	}
	return detectors
}

func registeredPatternSpecs() []patternSpec {
	seen := map[string]bool{}
	specs := make([]patternSpec, 0, len(registeredSpecs)+len(generated.Specs))
	for _, spec := range registeredSpecs {
		appendUniqueSpec(&specs, seen, spec)
	}
	for _, spec := range generated.Specs {
		appendUniqueSpec(&specs, seen, patternSpec{
			Name:       spec.Name,
			Category:   spec.Category,
			Group:      spec.Group,
			Direction:  spec.Direction,
			Template:   spec.Template,
			Confidence: spec.Confidence,
			Evidence:   spec.Evidence,
		})
	}
	sort.SliceStable(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

func appendUniqueSpec(specs *[]patternSpec, seen map[string]bool, spec patternSpec) {
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return
	}
	if spec.Category == "" {
		spec.Category = "pattern"
	}
	if spec.Direction == "" {
		spec.Direction = "neutral"
	}
	if spec.Template == "" {
		spec.Template = "generic"
	}
	if spec.Confidence <= 0 {
		spec.Confidence = 0.62
	}
	if spec.Evidence == "" {
		spec.Evidence = "pattern-specific scanner rule matched current market structure"
	}
	spec = applyPatternRuleOverrides(spec)
	spec = bindPatternRule(spec)
	key := strings.ToLower(spec.Name)
	if seen[key] {
		return
	}
	seen[key] = true
	*specs = append(*specs, spec)
}

type registeredSpecDetector struct {
	spec patternSpec
}

func (d registeredSpecDetector) Name() string { return d.spec.Name }

func (d registeredSpecDetector) Detect(_ context.Context, input ScannerInput) ([]ohlcv.PatternResult, error) {
	return detectPatternSpec(input, d.spec)
}

func applyPatternRuleOverrides(spec patternSpec) patternSpec {
	override, ok := exactPatternRuleOverrides[normalizePatternText(spec.Name)]
	if !ok {
		return spec
	}
	if override.template != "" {
		spec.Template = override.template
	}
	if override.direction != "" {
		spec.Direction = override.direction
	}
	if override.evidence != "" {
		spec.Evidence = override.evidence
	}
	return spec
}

func bindPatternRule(spec patternSpec) patternSpec {
	if spec.Rule == "" {
		spec.Rule = patternRuleID(spec.Template)
	}
	if spec.Template == "" && spec.Rule != "" {
		spec.Template = string(spec.Rule)
	}
	return spec
}

func patternTemplateHasScannerRule(template string) bool {
	_, ok := registeredPatternRule(patternRuleID(template))
	return ok
}
