package formations

type Options struct {
	Symbol               string
	Timeframe            string
	AnalysisDate         string
	PivotLookback        int
	LevelLookback        int
	MaxLevels            int
	ConsolidationWindow  int
	TrendlineSwingLimit  int
	MinTrendlineSpanBars int
}

type Result struct {
	Symbol            string                   `json:"symbol"`
	Timeframe         string                   `json:"timeframe"`
	CurrentPrice      float64                  `json:"current_price"`
	AnalysisDate      string                   `json:"analysis_date"`
	Trend             TrendSummary             `json:"trend"`
	MovingAverages    MovingAverages           `json:"moving_averages"`
	SupportResistance SupportResistanceSummary `json:"support_resistance"`
	Trendlines        []TrendlineResult        `json:"trendlines"`
	Patterns          []PatternResult          `json:"patterns"`
	BreakoutAnalysis  BreakoutAnalysis         `json:"breakout_analysis"`
	Scenarios         []Scenario               `json:"scenarios"`
	DrawingObjects    DrawingObjects           `json:"drawing_objects"`
	Summary           Summary                  `json:"summary"`
}

type TrendSummary struct {
	Primary    string  `json:"primary"`
	Secondary  string  `json:"secondary"`
	Confidence float64 `json:"confidence"`
}

type MovingAverages struct {
	EMA20  MASummary `json:"ema20"`
	EMA50  MASummary `json:"ema50"`
	Signal string    `json:"signal"`
}

type MASummary struct {
	Current  float64 `json:"current"`
	Position string  `json:"position"`
	Slope    string  `json:"slope"`
}

type SupportResistanceSummary struct {
	Supports    []Level `json:"supports"`
	Resistances []Level `json:"resistances"`
}

type Level struct {
	Price         float64 `json:"price"`
	TouchCount    int     `json:"touch_count"`
	Strength      float64 `json:"strength"`
	LastTouchDate string  `json:"last_touch_date"`
	Type          string  `json:"type"`
}

type TrendlineResult struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Slope      float64   `json:"slope"`
	TouchCount int       `json:"touch_count"`
	Strength   float64   `json:"strength"`
	Start      TimePrice `json:"start"`
	End        TimePrice `json:"end"`
	Status     string    `json:"status"`
}

type PatternResult struct {
	Name           string      `json:"name"`
	Category       string      `json:"category"`
	Confidence     float64     `json:"confidence"`
	Status         string      `json:"status"`
	StartDate      string      `json:"start_date"`
	EndDate        string      `json:"end_date"`
	UpperLine      PatternLine `json:"upper_line"`
	LowerLine      PatternLine `json:"lower_line"`
	MainSupport    float64     `json:"main_support"`
	MainResistance float64     `json:"main_resistance"`
	BreakoutLevel  float64     `json:"breakout_level"`
	BreakdownLevel float64     `json:"breakdown_level"`
	Targets        []float64   `json:"targets"`
	InvalidLevel   float64     `json:"invalid_level"`
}

type PatternLine struct {
	Start TimePrice `json:"start"`
	End   TimePrice `json:"end"`
}

type BreakoutAnalysis struct {
	Status             string  `json:"status"`
	Level              float64 `json:"level"`
	CloseConfirmation  bool    `json:"close_confirmation"`
	VolumeConfirmation bool    `json:"volume_confirmation"`
	RetestDetected     bool    `json:"retest_detected"`
}

type Scenario struct {
	Name             string      `json:"name"`
	Condition        string      `json:"condition"`
	ProbabilityScore float64     `json:"probability_score"`
	TargetLevels     []float64   `json:"target_levels"`
	InvalidLevel     float64     `json:"invalid_level"`
	PathPoints       []TimePrice `json:"path_points"`
}

type DrawingObjects struct {
	Lines       []LineObject  `json:"lines"`
	Paths       []PathObject  `json:"paths"`
	Labels      []LabelObject `json:"labels"`
	Fills       []FillBand    `json:"fills,omitempty"`
	TouchPoints []TimePrice   `json:"touch_points,omitempty"`
}

type FillBand struct {
	ID              string  `json:"id"`
	Color           string  `json:"color"`
	Opacity         uint8   `json:"opacity"`
	UpperStartTime  string  `json:"upper_start_time"`
	UpperStartPrice float64 `json:"upper_start_price"`
	UpperEndTime    string  `json:"upper_end_time"`
	UpperEndPrice   float64 `json:"upper_end_price"`
	LowerStartTime  string  `json:"lower_start_time"`
	LowerStartPrice float64 `json:"lower_start_price"`
	LowerEndTime    string  `json:"lower_end_time"`
	LowerEndPrice   float64 `json:"lower_end_price"`
}

type LineObject struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Color      string  `json:"color"`
	Width      int     `json:"width"`
	Style      string  `json:"style"`
	Price      float64 `json:"price,omitempty"`
	StartTime  string  `json:"start_time,omitempty"`
	StartPrice float64 `json:"start_price,omitempty"`
	EndTime    string  `json:"end_time,omitempty"`
	EndPrice   float64 `json:"end_price,omitempty"`
	Label      string  `json:"label"`
}

type PathObject struct {
	ID     string      `json:"id"`
	Type   string      `json:"type"`
	Color  string      `json:"color"`
	Width  int         `json:"width"`
	Style  string      `json:"style"`
	Points []TimePrice `json:"points"`
	Label  string      `json:"label"`
}

type LabelObject struct {
	Text  string  `json:"text"`
	Time  string  `json:"time"`
	Price float64 `json:"price"`
}

type Summary struct {
	ShortComment     string `json:"short_comment"`
	BullishCondition string `json:"bullish_condition"`
	BearishCondition string `json:"bearish_condition"`
	RiskNote         string `json:"risk_note"`
}

type TimePrice struct {
	Time  string  `json:"time"`
	Price float64 `json:"price"`
}

type SwingPoint struct {
	Index  int
	Time   string
	Price  float64
	Kind   string
	Volume float64
}

type trendLineCandidate struct {
	line       TrendlineResult
	startIdx   int
	endIdx     int
	startPrice float64
	endPrice   float64
}
