---
title: 'Forecast Quality Calibration'
type: 'feature'
created: '2026-06-24'
status: 'done'
baseline_commit: '7c6cf7e05ccccaad13da8b4c7935e3ccda5f3789'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/spec-forecast-report-verification-fixes.md'
  - '{project-root}/_bmad-output/implementation-artifacts/investigations/forecast-report-miscalculation-investigation.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Kullanıcı engine'in hisse fiyatını yüzde yüz tahmin etmesini istiyor; bu piyasa doğası gereği gerçekçi değil, fakat mevcut audit/rapor dili hâlâ exact fiyat başarısını fazla öne çıkarıyor ve modelin ne zaman güvenilir, ne zaman sadece senaryo olduğunu yeterince görünür yapmıyor.

**Approach:** Engine'i "kesin fiyat bilen" yapı yerine doğrulanabilir kalite sistemi olarak güçlendir: birincil başarıyı hata bandı, yön uyumu, rejim ve trade kapısı üzerinden raporla; exact fiyat isabetini ikincil denetim metriği yap; rejim bazlı performans kırılımı ve kalite notları ekle.

## Boundaries & Constraints

**Always:** Tek nokta fiyat tahmini yerine aralık ve tolerans-bandı metriklerini öne çıkar. Publish gate başarısızsa al/sat ya da emir anlamına gelecek çıktı üretme. Existing forecast üretim modelini, BIST fiyat adımı yuvarlamasını ve önceki `tahmini_*`/`senaryo_*` ayrımını koru. JSON şemasını geriye uyumlu genişlet; eski alanları kaldırma.

**Ask First:** Yeni dış veri kaynağı, ücretli API, model yeniden eğitimi, veri migrasyonu veya otomatik emir entegrasyonu gerekirse kullanıcıya sor.

**Never:** "%100 başarı" vaat eden alan adı, rapor metni veya karar mantığı ekleme. Exact price hit metriğini ana başarı skoru yapma. Suppressed forecast'i trade edilebilir hale getirme.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Forecast audit range | Birden fazla resmi BIST gerçekleşen satırı | Summary birincil olarak band hit, MAE/MAPE, yön uyumu, publish/trade oranı ve kalite notu gösterir | Exact-hit alanları kalır ama ikincil olarak etiketlenir |
| Rejim farkı | Audit satırları farklı volatilite/trend rejimlerinde oluşur | Rapor rejim bazlı satır sayısı, kapanış MAE, yön uyumu ve band hit üretir | Rejim yoksa `unknown` bucket kullanılır |
| Publish gate blocked | Forecast backtest/güven eşiğini geçmez | `no_trade`/senaryo-only durumu summary ve satır nedenlerinde açık görünür | Fiyat senaryosu karar fiyatına dönmez |
| High error but correct direction | Yön uyumlu fakat hata bandı dışı | Sonuç "yön uyumlu, fiyat bandı dışında" gibi ölçülü değerlendirilir | "başarılı" diye işaretlenmez |

</frozen-after-approval>

## Code Map

- `cmd/hissebot/forecast_audit.go` -- Range audit summary, row metrics, markdown/html rendering, model validation table.
- `cmd/hissebot/forecast_audit_test.go` -- Regression tests for exact-hit, suppression, and range audit output.
- `internal/ta/analysis/engine.go` -- Backtest metrics and publish gate already expose band hit and direction metrics.
- `internal/ta/forecastpolicy/forecastpolicy.go` -- Shared direction tolerance used by audit and forecast logic.

## Tasks & Acceptance

**Execution:**
- [x] `cmd/hissebot/forecast_audit.go` -- Add primary quality summary fields: close/open within 0.5%, 1%, 2%, average closeness, model published/suppressed ratio, trade allowed ratio, and a textual quality grade.
- [x] `cmd/hissebot/forecast_audit.go` -- Add regime performance buckets for range audit rows using volatility/trend labels available on forecast rows; include sample count, close MAE, close direction hit, close within 1% and trade allowed percentage.
- [x] `cmd/hissebot/forecast_audit.go` -- Rewrite markdown/html range summaries so exact price hit is explicitly secondary and band/MAE/direction/publish gate are primary.
- [x] `cmd/hissebot/forecast_audit.go` -- Improve row outcome text to distinguish exact hit, within-band hit, direction-only hit, and failed/no-trade cases.
- [x] `cmd/hissebot/forecast_audit_test.go` -- Add tests for primary band metrics, secondary exact metrics, regime bucket output, and no-trade interpretation.

**Acceptance Criteria:**
- Given a range audit with non-exact but close predictions, when summary is generated, then `*_within_1_00_pct` and closeness metrics can show success while exact-hit remains secondary.
- Given rows from multiple regimes, when range audit JSON is produced, then `regime_performance` contains per-regime sample, MAE, direction hit, within-band, and trade allowed metrics.
- Given a suppressed forecast row, when markdown/html is rendered, then the report says it is scenario/no-trade rather than a failed trade signal.
- Given a forecast direction hit with close error outside 1%, when row outcome is rendered, then it is classified as direction-only/price-band risk, not full success.

## Spec Change Log

## Design Notes

Quality grading should be conservative and explainable. A useful starting rule: strong only when close MAE is low, direction hit is acceptable, 1% band hit is high, and suppressed/no-trade behavior is visible. This is a reporting and gating improvement, not a promise that the model can predict every price.

## Verification

**Commands:**
- `go test ./cmd/hissebot` -- expected: all forecast audit tests pass.
- `go test ./internal/ta/analysis ./internal/ta/storage ./internal/ta/ml ./internal/ta/ensemble ./internal/ta/labels` -- expected: no regression in forecast/report packages.

## Suggested Review Order

**Primary Quality Model**

- Start with the expanded summary contract and exported JSON fields.
  [`forecast_audit.go:102`](../../cmd/hissebot/forecast_audit.go#L102)

- Review the single summary builder for band, direction, publish, and regime metrics.
  [`forecast_audit.go:536`](../../cmd/hissebot/forecast_audit.go#L536)

- Check conservative quality labels and no-trade/scenario classification thresholds.
  [`forecast_audit.go:730`](../../cmd/hissebot/forecast_audit.go#L730)

**Report Semantics**

- Validate row-level wording for exact, band, direction-only, and no-trade outcomes.
  [`forecast_audit.go:1314`](../../cmd/hissebot/forecast_audit.go#L1314)

- Confirm Markdown leads with band quality and labels exact hit as secondary.
  [`forecast_audit.go:1658`](../../cmd/hissebot/forecast_audit.go#L1658)

- Confirm HTML mirrors the same primary quality and regime messaging.
  [`forecast_audit.go:1732`](../../cmd/hissebot/forecast_audit.go#L1732)

**Regression Coverage**

- Verify non-exact close predictions can still score through quality bands.
  [`forecast_audit_test.go:154`](../../cmd/hissebot/forecast_audit_test.go#L154)

- Verify direction-only and no-trade rows are not reported as full success.
  [`forecast_audit_test.go:226`](../../cmd/hissebot/forecast_audit_test.go#L226)
