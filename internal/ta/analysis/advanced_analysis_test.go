package analysis

import (
	"fmt"
	"math"
	"testing"
	"time"

	"hissebot/internal/kapingest"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"
	"hissebot/internal/ta/value"
)

func TestBuildAdvancedAnalysisDeepensFinancialQualityAndValuation(t *testing.T) {
	result := advancedFixture()

	got := BuildAdvancedAnalysis(result)
	if !got.Computed {
		t.Fatalf("advanced analysis not computed: %+v", got)
	}
	if got.FinancialQuality.PiotroskiScore < 80 || len(got.FinancialQuality.PiotroskiChecks) != 9 {
		t.Fatalf("expected real Piotroski checks, got %+v", got.FinancialQuality)
	}
	if got.FinancialQuality.BeneishRiskProxy == "" || got.FinancialQuality.AltmanZStatus == "" || !got.FinancialQuality.DuPont.Computed {
		t.Fatalf("expected Beneish/Altman/DuPont outputs, got %+v", got.FinancialQuality)
	}
	if got.Valuation.BaseFairValue <= 0 || got.Valuation.ModelReliability <= 0 {
		t.Fatalf("expected weighted valuation ensemble, got %+v", got.Valuation)
	}
	for _, name := range []string{"owner_earnings_intrinsic", "dividend_discount", "residual_income"} {
		if !advancedModelExists(got.Valuation.Models, name) {
			t.Fatalf("expected valuation model %q in %+v", name, got.Valuation.Models)
		}
	}
}

func TestBuildAdvancedAnalysisEventStudyUsesAllMatchedEvents(t *testing.T) {
	result := advancedFixture()
	events := make([]kapingest.ExtractedCorporateEvent, 0, 12)
	for i := 0; i < 12; i++ {
		date := time.Date(2026, 1, 2+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		events = append(events, kapingest.ExtractedCorporateEvent{
			DocumentDate: strPtr(date),
			EventType:    "contract",
			Title:        fmt.Sprintf("new contract %d", i+1),
		})
	}
	result.Professional.RawKAPData = &professional.KAPRawDataBundle{
		Computed:        true,
		CorporateEvents: events,
	}

	got := BuildAdvancedAnalysis(result)
	if got.EventStudy.EventCount != 12 {
		t.Fatalf("event count=%d, want 12", got.EventStudy.EventCount)
	}
	if len(got.EventStudy.LatestEvents) != 10 {
		t.Fatalf("latest events len=%d, want 10", len(got.EventStudy.LatestEvents))
	}
	if got.EventStudy.EventWindowSampleCount != 12 {
		t.Fatalf("event window sample=%d, want 12: %+v", got.EventStudy.EventWindowSampleCount, got.EventStudy)
	}
	if got.EventStudy.AvgEventReturn1DPct == 0 || got.EventStudy.AvgEventReturn5DPct == 0 {
		t.Fatalf("expected event-window returns, got %+v", got.EventStudy)
	}
}

func advancedFixture() SymbolAnalysis {
	candles := make([]ohlcv.Candle, 0, 30)
	price := 100.0
	for i := 0; i < 30; i++ {
		price *= 1 + 0.003 + 0.001*math.Sin(float64(i))
		candles = append(candles, ohlcv.Candle{
			Time:          time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			Open:          price * 0.99,
			High:          price * 1.02,
			Low:           price * 0.98,
			Close:         price,
			Volume:        1_000_000 + float64(i)*10_000,
			AdjustedClose: price,
			IsAdjusted:    true,
		})
	}
	return SymbolAnalysis{
		Symbol:    "TEST",
		AssetType: ohlcv.AssetTypeEquity,
		Timeframes: map[string]TimeframeAnalysis{
			"1D": {Timeframe: "1D", Candles: candles, LastClose: price},
		},
		Professional: professional.Report{
			Market: professional.MarketContext{
				BenchmarkAvailable: true,
				StockReturn20:      0.08,
				BenchmarkReturn20:  0.03,
				Beta60:             0.9,
				Alpha60:            0.02,
				Correlation60:      0.6,
				RelativeStrength60: 0.04,
			},
			Valuation: professional.ValuationAnalysis{
				PaidCapital:       100,
				MarketCap:         1200,
				TotalDebt:         100,
				DebtDataAvailable: true,
				NetDebt:           50,
				SalesTTM:          1000,
				EBITTTM:           130,
				EBITDATTM:         170,
				NetIncomeTTM:      100,
				OperatingCashTTM:  140,
				FreeCashFlowTTM:   90,
				Equity:            800,
				TotalAssets:       1200,
				Ratios: map[string]float64{
					"ROE":          0.125,
					"ROA":          0.083,
					"gross_margin": 0.43,
				},
				FairValue: professional.FairValueRange{
					Bear:       12,
					Base:       16,
					Bull:       20,
					Confidence: 70,
					Drivers:    []string{"peer fair value"},
				},
				DCF: professional.DCFAnalysis{
					Computed:          true,
					FairValuePerShare: 18,
					WACC:              0.20,
					TerminalGrowth:    0.05,
					CostOfEquity:      0.21,
					AssumptionSource:  "test",
				},
			},
			Peers: professional.PeerComparison{PeerCount: 4, ValuationSignal: "peer_supported"},
			ValueInvesting: value.Report{
				Computed:   true,
				Confidence: 76,
				IntrinsicValue: value.IntrinsicValueReport{
					Computed:   true,
					Bear:       15,
					Base:       19,
					Bull:       23,
					Confidence: 82,
					Drivers:    []string{"owner earnings"},
				},
				CapitalAllocation: value.CapitalAllocationReport{
					DividendDataAvailable: true,
					DividendYears:         5,
					Score:                 75,
				},
				Assumptions: value.Assumptions{DiscountRate: 0.20, TerminalGrowth: 0.05},
				Years: []value.YearMetric{
					{Year: 2024, Revenue: 900, GrossProfit: 360, OperatingProfit: 100, NetIncome: 70, OperatingCash: 60, FreeCashFlow: 55, DividendsPaid: 30, PaidCapital: 100, Equity: 700, TotalAssets: 1100, Cash: 100, Debt: 150, ROE: 0.10, GrossMargin: 0.40, NetMargin: 0.078},
					{Year: 2025, Revenue: 1000, GrossProfit: 430, OperatingProfit: 130, NetIncome: 100, OperatingCash: 140, FreeCashFlow: 90, DividendsPaid: 40, PaidCapital: 100, Equity: 800, TotalAssets: 1200, Cash: 160, Debt: 100, ROE: 0.125, GrossMargin: 0.43, NetMargin: 0.10},
				},
			},
			InvestmentResearch: professional.InvestmentResearchReview{
				ValuationBridge: professional.ValuationTransparency{
					NAVStatus:          "book_value_nav_proxy",
					BaseIntrinsicValue: 17,
					BearIntrinsicValue: 13,
					BullIntrinsicValue: 21,
					NAVBridge: professional.NAVBridge{
						Status:               "book_value_nav_proxy",
						EstimatedNAVPerShare: 8,
					},
				},
			},
		},
		StatEconomic: StatEconomicAnalysis{
			FinancialQuality: EconomicFinancialQuality{
				Score:                    72,
				AccrualQualityProxyScore: 80,
				EarningsPersistenceScore: 74,
				ManipulationRiskProxy:    "low_proxy",
			},
			Validation: StatisticalValidation{Score: 70},
			Liquidity:  EconomicLiquidityDiagnostics{Score: 70, AverageValue20TRY: 20_000_000, MedianValue20TRY: 18_000_000, CapacityTRYAt10PctADV: 2_000_000},
		},
		Quant: QuantAnalysis{
			Return: QuantReturnMetrics{LastClose: price},
			Risk:   QuantRiskMetrics{AnnualizedVolatilityPct: 25, HistoricalVaR95Pct: 2, RiskBudgetOneDayVaRPer100K: 2500},
		},
	}
}

func advancedModelExists(models []AdvancedValuationModel, name string) bool {
	for _, model := range models {
		if model.Name == name && model.Status != "missing" && model.Status != "missing_data" {
			return true
		}
	}
	return false
}

func strPtr(value string) *string {
	return &value
}
