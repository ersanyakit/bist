package analysis

import (
	"context"
	"time"

	"hissebot/internal/domain/financials"
	"hissebot/internal/domain/macro"
	"hissebot/internal/domain/marketdata"
)

type Input struct {
	Symbol      string
	AsOf        time.Time
	Prices      []marketdata.OHLCV
	Financials  financials.Statements
	MacroSeries []macro.Series
	MissingData []string
}

type Output struct {
	Symbol                string    `json:"symbol"`
	AnalysisDate          time.Time `json:"analysis_date"`
	DataQualityScore      float64   `json:"data_quality_score"`
	TechnicalScore        float64   `json:"technical_score"`
	FundamentalScore      float64   `json:"fundamental_score"`
	ValuationScore        float64   `json:"valuation_score"`
	RiskScore             float64   `json:"risk_score"`
	MacroSensitivityScore float64   `json:"macro_sensitivity_score"`
	OverallScore          float64   `json:"overall_score"`
	InvestmentView        string    `json:"investment_view"`
	ConfidenceLevel       float64   `json:"confidence_level"`
	MissingData           []string  `json:"missing_data,omitempty"`
	Warnings              []string  `json:"warnings,omitempty"`
	Explanation           string    `json:"explanation"`
}

type AnalysisService interface {
	Analyze(ctx context.Context, input Input) (Output, error)
}

type TechnicalAnalysisService interface {
	ScoreTechnical(ctx context.Context, prices []marketdata.OHLCV) (float64, []string, error)
}

type FundamentalAnalysisService interface {
	ScoreFundamental(ctx context.Context, statements financials.Statements) (float64, []string, error)
}

type ValuationService interface {
	ScoreValuation(ctx context.Context, statements financials.Statements, currentPrice float64) (float64, []string, error)
}

type RiskAnalysisService interface {
	ScoreRisk(ctx context.Context, input Input) (float64, []string, error)
}
