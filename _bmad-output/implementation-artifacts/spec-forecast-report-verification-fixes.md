---
title: 'Forecast Report Verification Fixes'
type: 'bugfix'
created: '2026-06-24'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/investigations/forecast-report-miscalculation-investigation.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Forecast raporlarında fiyat tahmini ve yön alanları doğrulanabilir değil: yayınlanmaması gereken senaryo değerleri tahmin gibi görünüyor, deterministic forecast ML ensemble içine gate olmadan akıyor, yön eşikleri katmanlar arasında farklı çalışıyor ve ML hedef tarihi takvim gününe göre kayabiliyor.

**Approach:** Forecast yön politikasını tek kaynakta toparla, point forecast publish gate başarısızsa ham senaryo değerlerini karar/ML tahmini gibi kullandırma, ML hedef seansını işlem günü mantığına yaklaştır ve audit/report alanlarını yayınlanmış tahmin ile senaryo ayrımını açıkça koruyacak şekilde test et.

## Boundaries & Constraints

**Always:** Mevcut JSON şemasını gereksiz kırma; mevcut alanları tamamen silmek yerine mümkünse `nil`/zero ve status alanlarıyla güvenli hale getir. BIST fiyat adımı yuvarlama davranışını koru. Publish gate başarısızsa kullanıcıya karar verilebilir fiyat tahmini sunma. Direction policy forecast, audit, ML fallback ve rapor rendering tarafında aynı tolerans değerini kullanmalı.

**Ask First:** Report schema’dan eski `tahmini_*` veya `predicted_*` alanlarını tamamen kaldırmak gerekirse kullanıcıya sor. ML model seçim, registry formatı veya eğitim datası üretim formatı değişecekse kullanıcıya sor. BIST tatil takvimi için yeni dış bağımlılık veya network gerektiren veri kaynağı eklenecekse kullanıcıya sor.

**Never:** Exact price prediction’ı ana başarı metriği gibi güçlendirme. Publish gate’i bypass ederek daha fazla tahmini görünür yapma. Büyük mimari rewrite, model yeniden eğitimi veya veri migrasyonu yapma.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Publish gate blocks point forecast | `PointForecastPublishable=false`, actual session not observed | Decision forecast open/close and ML deterministic anchor do not expose raw predicted prices as tradable point forecasts; status/reason remains visible | Report keeps scenario metadata but no tradeable/published point price |
| Publish gate passes | Sufficient backtest, confidence, quality, technical gate | `PublishedPredictedOpen/Close` remain set and decision forecast can carry those values | Existing BIST tick rounding and quality warnings stay intact |
| Small move near neutral band | Price move within shared direction tolerance | Forecast, audit and report direction classify as neutral/flat/yatay consistently | Invalid or missing prices return unknown/uncertain according to existing caller convention |
| Friday or weekend boundary | ML feature vector `AsOf` falls before weekend for equity | ML and deterministic ensemble target the next weekday session instead of calendar next day | Crypto/commodity behavior is not broadened unless existing asset-type context supports it |

</frozen-after-approval>

## Code Map

- `internal/ta/analysis/engine.go` -- Owns `NextSessionForecast`, publish gate, decision forecast sync, forecast direction tolerance and next-session date helper.
- `internal/ta/forecastpolicy/forecastpolicy.go` -- Shared next-session direction tolerance, direction classification and weekday target-session policy.
- `internal/ta/storage/writer.go` -- Hydrates reports, creates deterministic ML input, sanitizes unpublished point forecasts, and renders Turkish forecast JSON.
- `internal/ta/storage/report_data_manifest.go` -- Emits manifest fields that currently mix published forecast and scenario values.
- `cmd/hissebot/forecast_audit.go` -- Computes audit direction and exact-hit metrics.
- `internal/ta/ml/model.go` -- Creates baseline ML predictions and direction classification.
- `internal/ta/ensemble/ensemble.go` -- Converts deterministic forecast into ensemble prediction and assigns target session.
- `internal/ta/ensemble/contribution.go` -- Weights deterministic forecast contribution.
- `internal/ta/labels/labels.go` -- Builds training labels whose default direction threshold must match the forecast policy.
- `internal/ta/analysis/engine_test.go`, `internal/ta/storage/*_test.go`, `cmd/hissebot/forecast_audit_test.go`, `internal/ta/ml/*_test.go`, `internal/ta/ensemble/*_test.go` -- Focused regression coverage.

## Tasks & Acceptance

**Execution:**
- [x] `internal/ta/analysis/engine.go` -- Add/export shared next-session direction tolerance helpers and use them in forecast direction, technical direction classification, probability neutral band and JSON direction helpers.
- [x] `cmd/hissebot/forecast_audit.go` -- Replace local hard-coded neutral tolerance with the shared analysis direction policy.
- [x] `internal/ta/ml/model.go` and `internal/ta/ensemble/ensemble.go` -- Use shared direction tolerance for fallback direction and move target-session calculation away from raw calendar `AddDate(0,0,1)` where a weekday next-session helper is available.
- [x] `internal/ta/storage/writer.go` -- Make `deterministicInputFromForecast` return an empty deterministic input when the point forecast is not publishable; make decision/report rendering prefer published values for point forecast fields.
- [x] `internal/ta/storage/report_data_manifest.go` -- Keep scenario fields clearly separate from published forecast fields and avoid exposing blocked raw values as plain `predicted_*`.
- [x] `internal/ta/labels/labels.go` -- Align default training-label direction threshold with the shared next-session forecast policy.
- [x] Tests -- Add or adjust unit tests for blocked publish gate, shared neutral direction threshold, ML deterministic anchor suppression, and target-session weekday handling.

**Acceptance Criteria:**
- Given a forecast blocked by publish gate, when the report and ML forecast are hydrated, then raw predicted prices do not become decision forecast point prices or deterministic ML anchor values.
- Given a forecast that passes publish gate, when the report is rendered, then published predicted open/close are still available and existing tick rounding behavior is preserved.
- Given the same small price move near the neutral tolerance, when forecast direction, audit direction and ML fallback direction are computed, then all classify the move according to one shared tolerance policy.
- Given an equity ML prediction created on a Friday, when target session is calculated without a holiday calendar, then it targets the following Monday rather than Saturday.

## Spec Change Log

## Design Notes

The core invariant is: scenario values may remain for explanatory context, but only `PublishedPredictedOpen/Close` can feed decision-grade point forecast outputs. If a downstream component needs a deterministic anchor, it should receive one only after the same publish gate has passed.

## Verification

**Commands:**
- `go test ./internal/ta/analysis ./cmd/hissebot ./internal/ta/ml ./internal/ta/ensemble ./internal/ta/labels ./internal/ml/offline` -- expected: all tests pass.
- `go test ./internal/ta/storage ./internal/ta/professional ./internal/ta/value ./internal/ta/validation` -- expected: all tests pass.

## Suggested Review Order

**Direction Policy**

- Single source for neutral bands, direction hits and weekday sessions.
  [`forecastpolicy.go:8`](../../internal/ta/forecastpolicy/forecastpolicy.go#L8)

- Technical forecast direction now uses the shared tolerance.
  [`engine.go:2733`](../../internal/ta/analysis/engine.go#L2733)

- Audit direction and hit checks no longer carry local thresholds.
  [`forecast_audit.go:1026`](../../cmd/hissebot/forecast_audit.go#L1026)

**Publish Gate**

- Decision forecast emits point prices only when trade signal is allowed.
  [`engine.go:3196`](../../internal/ta/analysis/engine.go#L3196)

- Deterministic ML anchor is empty unless point forecast is publishable.
  [`writer.go:327`](../../internal/ta/storage/writer.go#L327)

**Report Surface**

- Turkish JSON separates published forecasts from scenario values.
  [`writer.go:2514`](../../internal/ta/storage/writer.go#L2514)

- Manifest keeps `predicted_*` published and `scenario_*` explanatory.
  [`report_data_manifest.go:416`](../../internal/ta/storage/report_data_manifest.go#L416)

**Session And Labels**

- Baseline ML targets next session by asset type.
  [`model.go:292`](../../internal/ta/ml/model.go#L292)

- Ensemble deterministic target uses the same session policy.
  [`ensemble.go:108`](../../internal/ta/ensemble/ensemble.go#L108)

- Training label default threshold matches forecast direction tolerance.
  [`labels.go:36`](../../internal/ta/labels/labels.go#L36)

**Regression Tests**

- Blocked forecasts cannot leak decision point prices.
  [`engine_test.go:182`](../../internal/ta/analysis/engine_test.go#L182)

- Blocked forecasts cannot become deterministic ML anchors.
  [`writer_test.go:121`](../../internal/ta/storage/writer_test.go#L121)

- Weekday/crypto target-session behavior is covered.
  [`model_test.go:41`](../../internal/ta/ml/model_test.go#L41)

- Report JSON keeps published and scenario fields distinct.
  [`professional_report_test.go:750`](../../internal/ta/storage/professional_report_test.go#L750)
