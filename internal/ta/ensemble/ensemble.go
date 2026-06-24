package ensemble

import (
	"context"

	"hissebot/internal/ta/backtest"
	"hissebot/internal/ta/features"
	"hissebot/internal/ta/forecastpolicy"
	"hissebot/internal/ta/ml"
	"hissebot/internal/ta/validation"
)

type DeterministicInput struct {
	PredictedOpen     float64
	PredictedClose    float64
	ExpectedReturn    float64
	Direction         string
	Confidence        float64
	Model             string
	ValidationMetrics validation.Metrics
}

type Result struct {
	Prediction    ml.ModelPrediction
	Gate          TradeGateDecision
	Fallback      ml.ReportFallback
	Validation    validation.Metrics
	Regime        ml.RegimeSummary
	MetaLabel     ml.MetaLabelSummary
	Contributions []ml.ModelContribution
	Warnings      []string
}

func RunShadow(ctx context.Context, fv features.FeatureVector, deterministic DeterministicInput, models []ml.ForecastModel, cfg ml.RuntimeConfig) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(models) == 0 {
		models = ml.BaselineModels()
	}
	preds := []ml.ModelPrediction{}
	weights := []float64{}
	contributions := []ml.ModelContribution{}
	warnings := []string{}
	regime := DetectRegime(fv)
	for _, model := range models {
		pred, err := model.Predict(ctx, fv)
		if err != nil {
			warnings = append(warnings, model.Name()+":"+err.Error())
			continue
		}
		weight, reason := modelWeight(pred, fv, regime)
		preds = append(preds, pred)
		weights = append(weights, weight)
		contributions = append(contributions, contributionFromPrediction(pred, weight, reason))
	}
	if deterministic.PredictedClose > 0 {
		dpred := deterministicPrediction(fv, deterministic)
		weight, reason := modelWeight(dpred, fv, regime)
		preds = append(preds, dpred)
		weights = append(weights, weight)
		contributions = append(contributions, contributionFromPrediction(dpred, weight, reason))
	}
	fallback := ml.ReportFallback{}
	if len(preds) == 0 {
		fallback.Used = true
		fallback.Reason = "no_ml_prediction_available"
		dpred := deterministicPrediction(fv, deterministic)
		preds = append(preds, dpred)
		weights = append(weights, 1)
		contributions = append(contributions, contributionFromPrediction(dpred, 1, "last_resort_deterministic_fallback"))
	}
	contributions = normalizeContributions(contributions)
	pred := WeightedAverage(preds, weights)
	pred = ml.SanitizePrediction(CalibrateConfidence(pred, deterministic.ValidationMetrics.BrierScore), fv)
	costCfg := backtest.DefaultCostsConfig()
	costCfg.CommissionBps = cfg.Costs.CommissionBps
	costCfg.SlippageBps = cfg.Costs.SlippageBps
	costCfg.SpreadBps = cfg.Costs.SpreadBps
	gateCfg := DefaultTradeGateConfig()
	gateCfg.MinExpectedReturnAfterCost = cfg.TradeGate.MinExpectedReturnAfterCost
	gateCfg.MinDirectionProbability = cfg.TradeGate.MinDirectionProbability
	gateCfg.MaxIntervalWidthATRMultiple = cfg.TradeGate.MaxIntervalWidthATRMultiple
	gateCfg.MinSourceScore = cfg.TradeGate.MinSourceScore
	gateCfg.AllowTradeOnLowQualityData = cfg.TradeGate.AllowTradeOnLowQualityData
	gateCfg.TransactionCostPct = backtest.RoundTripCostPct(costCfg)
	gate := EvaluateTradeGate(pred, fv, deterministic.ValidationMetrics, gateCfg)
	metaCfg := DefaultMetaLabelConfig()
	metaCfg.TransactionCostPct = gateCfg.TransactionCostPct
	metaCfg.MaxIntervalATRWidth = gateCfg.MaxIntervalWidthATRMultiple
	meta := EvaluateMetaLabel(pred, fv, regime, metaCfg)
	gate = ApplyMetaLabelGate(gate, meta)
	if regime.PositionScale > 0 {
		gate.SizeHint *= regime.PositionScale
	}
	return Result{
		Prediction:    pred,
		Gate:          gate,
		Fallback:      fallback,
		Validation:    deterministic.ValidationMetrics,
		Regime:        regime,
		MetaLabel:     meta,
		Contributions: contributions,
		Warnings:      warnings,
	}
}

func deterministicPrediction(fv features.FeatureVector, in DeterministicInput) ml.ModelPrediction {
	ret := in.ExpectedReturn
	if ret == 0 && in.PredictedClose > 0 && fv.Values["last_close"] > 0 {
		ret = in.PredictedClose/fv.Values["last_close"] - 1
	}
	p := ml.ModelPrediction{
		Symbol:                 fv.Symbol,
		AsOf:                   fv.AsOf,
		TargetSession:          forecastpolicy.NextSessionForAssetType(fv.AsOf, fv.Categorical["asset_type"]),
		ModelName:              "deterministic_" + in.Model,
		ModelVersion:           "existing_engine",
		FeatureSetVersion:      fv.FeatureSetVersion,
		PredictedOpen:          in.PredictedOpen,
		PredictedClose:         in.PredictedClose,
		ExpectedReturn:         ret,
		Direction:              normalizeDirection(in.Direction),
		DirectionProbUp:        0.33,
		DirectionProbDown:      0.33,
		DirectionProbFlat:      0.34,
		Confidence:             in.Confidence / 100,
		CalibratedConfidence:   in.Confidence / 100,
		PredictionIntervalLow:  in.PredictedClose * 0.98,
		PredictionIntervalHigh: in.PredictedClose * 1.02,
		Quantiles: map[string]float64{
			"p05": ret - 0.02, "p10": ret - 0.015, "p25": ret - 0.0075,
			"p50": ret, "p75": ret + 0.0075, "p90": ret + 0.015, "p95": ret + 0.02,
		},
		Debug: map[string]any{"source": "existing_deterministic_forecast"},
	}
	if p.Direction == "up" {
		p.DirectionProbUp = 0.45
		p.DirectionProbFlat = 0.30
		p.DirectionProbDown = 0.25
	} else if p.Direction == "down" {
		p.DirectionProbDown = 0.45
		p.DirectionProbFlat = 0.30
		p.DirectionProbUp = 0.25
	}
	return p
}

func ToForecastReport(result Result, fv features.FeatureVector, cfg ml.RuntimeConfig) ml.ForecastReport {
	pred := result.Prediction
	report := ml.ForecastReport{
		Enabled:           cfg.ML.Enabled,
		ShadowMode:        cfg.ML.ShadowMode,
		ModelName:         pred.ModelName,
		ModelVersion:      pred.ModelVersion,
		FeatureSetVersion: pred.FeatureSetVersion,
		PredictedOpen:     pred.PredictedOpen,
		PredictedClose:    pred.PredictedClose,
		ExpectedReturn:    pred.ExpectedReturn,
		Direction:         pred.Direction,
		DirectionProbabilities: ml.DirectionProbabilities{
			Up:   pred.DirectionProbUp,
			Down: pred.DirectionProbDown,
			Flat: pred.DirectionProbFlat,
		},
		Quantiles: pred.Quantiles,
		PredictionInterval: ml.PredictionInterval{
			Low:  pred.PredictionIntervalLow,
			High: pred.PredictionIntervalHigh,
		},
		CalibratedConfidence: pred.CalibratedConfidence,
		Calibration: ml.CalibrationSummary{
			Method:           "probability_clipping_bucket_ready_v1",
			BrierScore:       result.Validation.BrierScore,
			CalibrationError: result.Validation.CalibrationError,
		},
		Regime:             result.Regime,
		MetaLabel:          result.MetaLabel,
		ModelContributions: result.Contributions,
		ValidationSummary: ml.ReportValidationSummary{
			WalkForwardWindows: len(result.Validation.PinballLoss),
			MAE:                result.Validation.MAE,
			RMSE:               result.Validation.RMSE,
			MAPE:               result.Validation.MAPE,
			DirectionAccuracy:  result.Validation.DirectionAccuracy,
			BrierScore:         result.Validation.BrierScore,
			Sharpe:             result.Validation.Sharpe,
			MaxDrawdown:        result.Validation.MaxDrawdown,
			BaselineComparison: map[string]float64{},
		},
		TradeGate: ml.ReportTradeGate{
			Allowed:      result.Gate.Allowed,
			Action:       result.Gate.Action,
			Confidence:   result.Gate.Confidence,
			Reasons:      result.Gate.Reasons,
			RiskWarnings: result.Gate.RiskWarnings,
		},
		DataQuality: ml.ReportDataQuality{
			MissingRatio: fv.Quality.MissingRatio,
			StaleFields:  fv.Quality.StaleFields,
			LeakageFlags: fv.Quality.LeakageFlags,
			SourceScore:  fv.Quality.SourceScore,
		},
		Fallback: result.Fallback,
		Warnings: append(pred.Warnings, result.Warnings...),
		Debug:    pred.Debug,
	}
	return report
}

func normalizeDirection(direction string) string {
	switch direction {
	case "yukselis", "yükseliş", "up", "bullish":
		return "up"
	case "dusus", "düşüş", "down", "bearish":
		return "down"
	default:
		return "flat"
	}
}
