# hissebot — Dokümantasyon İndeksi

> Otomatik üretildi: BMAD Document Project (kapsamlı tarama) — 2026-06-25
> 👉 Bu dosya AI destekli geliştirme için **birincil giriş noktasıdır**.

## Proje Genel Bakış

- **Tür:** Monolith — Backend
- **Birincil Dil:** Go 1.26 (modül `hissebot`)
- **Mimari:** Katmanlı; dosya tabanlı kalıcılık (PostgreSQL'siz)
- **Boyut:** ~165k satır, 978 Go dosyası, 128 test dosyası
- **Amaç:** BIST (Borsa İstanbul) hisseleri için veri toplama → finansal/teknik analiz → raporlama motoru

## Hızlı Referans

- **Teknoloji:** Go 1.26, gofiber/fiber v3 (HTTP), gorilla/websocket, excelize, ledongthuc/pdf
- **Giriş noktası:** `cmd/hissebot/main.go` (CLI alt-komut yönlendirici + `serve api/reports/comments`)
- **Ağırlık merkezi:** `internal/ta` (kod tabanının %66'sı — teknik analiz motoru)
- **Kalıcılık:** `data/equities/{TICKER}/*.json` (hisse bazlı dosyalar)
- **Tipik akış:** `sync all-data` → `financials run -force` → `analyze -symbol X`

## Üretilen Dokümantasyon

- [Proje Genel Bakış](./project-overview.md) — amaç, teknoloji yığını, depo yapısı
- [Mimari](./architecture.md) — katmanlar, bağımlılık grafiği, veri akışı, mimari riskler
- [Kaynak Ağacı Analizi](./source-tree-analysis.md) — açıklamalı dizin haritası, giriş noktaları
- [API & Komut Sözleşmeleri](./api-contracts.md) — CLI alt-komutları + HTTP endpoint'leri
- [Veri Modelleri](./data-models.md) — dosya düzeni, domain aggregate'leri, referans SQL şeması
- [Geliştirme Rehberi](./development-guide.md) — kurulum, konfig, build, test, konvansiyonlar
- [Dağıtım Rehberi](./deployment-guide.md) — KAP worker, GitHub Actions, launchd, sunucular

## Mevcut Dokümantasyon (proje deposundan)

- [AI Erişim Rehberi](./ai_access_guide.md) — AI ajanlarının dosya/HTTP/MCP ile repo verisine erişimi
- [Analiz Motoru Yol Haritası](./analysis_engine_roadmap.md) — quant/stat-ekonomik kapsam, faz planı
- [QuantLib / Fincept Araştırma Planı](./quantlib_fincept_research_plan.md) — akademik doğrulama, reference validator, native Go/sidecar entegrasyon fazları
- [Forecast Precision Veri Yol Haritası](./forecast_precision_data_roadmap.md) — dar karar bandı, eksik veri katmanları, validation/publish gate metrikleri
- [BIST Veri Mimarisi](./bist_data_architecture.md) — veri kaynakları ve akış
- [KAP Belge Analiz Mimarisi](./kap_document_analysis_architecture.md) — KAP belge zekası
- [Kod İncelemesi (2026-06-25)](./review-2026-06-25.md) — mimari/kalite bulguları + düzeltmeler

## Başlangıç

```bash
go mod download
go build ./...
go run ./cmd/hissebot help

# Tek hisse uçtan uca
go run ./cmd/hissebot sync all-data
go run ./cmd/hissebot financials run -force
go run ./cmd/hissebot analyze -symbol ASELS

# Test
go test -race ./...
```

## Brownfield PRD İçin

Yeni özellik planlarken PRD workflow'una bu indeksi girdi olarak verin: `docs/index.md`.

- Sadece analiz/hesaplama özellikleri → `architecture.md` (`internal/ta`, `internal/quant`)
- Veri toplama özellikleri → `architecture.md` (`internal/services`) + `data-models.md`
- API özellikleri → `api-contracts.md` + `cmd/hissebot/api_server.go`
