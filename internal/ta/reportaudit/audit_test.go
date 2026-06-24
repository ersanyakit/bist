package reportaudit

import "testing"

func TestValidateJSONPassesSpotOnlyNeutralPlanWithDataDrivenLevels(t *testing.T) {
	data := []byte(`{
		"symbol": "TEST",
		"professional": {
			"data_quality": 100,
			"company": {"sector": "Savunma ve Elektronik"},
			"peer_comparison": {
				"sector": "Savunma ve Elektronik",
				"peer_count": 2,
				"peers": [
					{"symbol":"KAREL","name":"KAREL ELEKTRONİK SANAYİ VE TİCARET A.Ş."}
				]
			}
		},
		"institutional_validation": {
			"status": "pass",
			"score": 100,
			"checks": [
				{"name":"walk_forward_backtest","status":"pass","message":"ok"},
				{"name":"signal_success_rate","status":"pass","message":"ok"},
				{"name":"sentiment_source_policy","status":"pass","message":"ok"},
				{"name":"peer_universe","status":"pass","message":"ok"},
				{"name":"explainability_audit","status":"pass","message":"ok"},
				{"name":"visual_text_quality","status":"pass","message":"ok"}
			]
		},
		"timeframes": {
			"1D": {
				"last_close": 100,
				"support_levels": [{"type":"support","price":95}],
				"resistance_levels": [{"type":"resistance","price":105}],
				"nearest_support": {"type":"support","price":95},
				"nearest_resistance": {"type":"resistance","price":105},
				"trade_plan": {
					"direction": "neutral",
					"rejected": true,
					"reject_reason": "short selling is not supported for this instrument"
				}
			}
		}
	}`)
	report, err := ValidateJSON(data, Options{SpotOnly: true})
	if err != nil {
		t.Fatalf("validate json: %v", err)
	}
	if report.Status != "pass" {
		t.Fatalf("status = %s issues=%+v", report.Status, report.Issues)
	}
}

func TestValidateJSONFailsWrongSideSupportAndSpotShortPlan(t *testing.T) {
	data := []byte(`{
		"symbol": "TEST",
		"professional": {
			"data_quality": 100,
			"company": {"sector": "Savunma ve Elektronik"},
			"peer_comparison": {
				"sector": "Savunma ve Elektronik",
				"peer_count": 2,
				"peers": [
					{"symbol":"KAREL","name":"KAREL ELEKTRONİK SANAYİ VE TİCARET A.Ş."}
				]
			}
		},
		"timeframes": {
			"1D": {
				"last_close": 100,
				"support_levels": [{"type":"support","price":105}],
				"resistance_levels": [{"type":"resistance","price":95}],
				"nearest_support": {"type":"support","price":105},
				"nearest_resistance": {"type":"resistance","price":95},
				"trade_plan": {
					"direction": "short",
					"entry_min": 98,
					"entry_max": 102,
					"take_profit1": 90,
					"take_profit2": 80,
					"stop_loss": 110,
					"risk_reward_ratio": 1,
					"rejected": false
				}
			}
		}
	}`)
	report, err := ValidateJSON(data, Options{SpotOnly: true})
	if err != nil {
		t.Fatalf("validate json: %v", err)
	}
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %+v", report)
	}
	if len(report.Issues) < 5 {
		t.Fatalf("expected multiple data-driven issues, got %+v", report.Issues)
	}
}

func TestValidateJSONFailsNonGYOClassifiedAsREIT(t *testing.T) {
	data := []byte(`{
		"symbol": "ASELS",
		"professional": {
			"data_quality": 100,
			"company": {"sector": "Gayrimenkul Yatırım Ortaklığı"},
			"peer_comparison": {
				"sector": "Gayrimenkul Yatırım Ortaklığı",
				"peer_count": 2,
				"peers": [
					{"symbol":"TRGYO","name":"TORUNLAR GAYRİMENKUL YATIRIM ORTAKLIĞI A.Ş."}
				]
			}
		},
		"timeframes": {
			"1D": {
				"last_close": 100,
				"support_levels": [{"type":"support","price":95}],
				"resistance_levels": [{"type":"resistance","price":105}],
				"nearest_support": {"type":"support","price":95},
				"nearest_resistance": {"type":"resistance","price":105},
				"trade_plan": {"direction": "neutral", "rejected": true}
			}
		}
	}`)
	report, err := ValidateJSON(data, Options{SpotOnly: true})
	if err != nil {
		t.Fatalf("validate json: %v", err)
	}
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %+v", report)
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "professional.company.sector" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sector issue, got %+v", report.Issues)
	}
}
