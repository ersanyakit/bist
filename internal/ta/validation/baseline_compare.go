package validation

func BaselinePassed(model Metrics, baselines map[string]Metrics) (bool, []string) {
	reasons := []string{}
	for name, base := range baselines {
		if base.Samples == 0 {
			continue
		}
		if model.DirectionAccuracy > 0 && base.DirectionAccuracy > 0 && model.DirectionAccuracy <= base.DirectionAccuracy {
			reasons = append(reasons, "direction_accuracy_not_above_"+name)
		}
		if model.MAE > 0 && base.MAE > 0 && model.MAE >= base.MAE {
			reasons = append(reasons, "mae_not_below_"+name)
		}
		if model.Sharpe > 0 && base.Sharpe > 0 && model.Sharpe <= base.Sharpe {
			reasons = append(reasons, "sharpe_not_above_"+name)
		}
	}
	return len(reasons) == 0, reasons
}
