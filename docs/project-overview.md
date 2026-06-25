# hissebot — Proje Genel Bakış

> Otomatik üretildi: BMAD Document Project (kapsamlı tarama) — 2026-06-25
> Birincil dil: Go 1.26 · Tür: Backend (monolith) · ~165k satır / 978 Go dosyası

## Amaç

`hissebot`, **BIST (Borsa İstanbul)** hisseleri için uçtan uca veri toplama, finansal/teknik analiz ve raporlama motoru. PostgreSQL kullanmayan, veriyi **hisse bazlı yerel JSON dosyalarında** saklayan bir Go portudur. Her hissenin tüm verisi kendi klasöründe (`data/equities/{TICKER}/`) tutulur.

Sistem üç ana işi yapar:
1. **Veri toplama (ingestion):** TradingView, KAP, MKK, TÜİK, TCMB/EVDS, IS Yatırım gibi kaynaklardan fiyat, finansal tablo, kurumsal aksiyon ve KAP bildirim/eklerini çeker.
2. **Analiz:** Teknik indikatörler, formasyon/patern taraması, risk skorlama, forecasting ve kuantitatif (portföy/oran/volatilite) hesaplar.
3. **Raporlama ve servis:** Profesyonel analiz raporları üretir, HTTP API ve yorum/rapor sunucularıyla servis eder.

## Teknoloji Yığını

| Kategori | Teknoloji | Sürüm | Not |
|---|---|---|---|
| Dil | Go | 1.26 | Modül adı `hissebot` |
| HTTP framework | gofiber/fiber | v3.3.0 | `serve api` sunucusu |
| WebSocket | gorilla/websocket | v1.5.3 | Canlı market verisi (`internal/wsclient`) |
| Excel | xuri/excelize | v2.8.1 | Rapor/finansal tablo I/O |
| PDF | ledongthuc/pdf | — | KAP belge ayrıştırma |
| XLS (legacy) | extrame/xls | v0.0.1 | Eski bilanço dosyaları |
| SQLite | mattn/go-sqlite3 | v1.14.32 | (dolaylı) |
| Görsel | golang.org/x/image | v0.18.0 | Grafik render (`internal/ta/chart`) |

> **Önemli:** Çalışma zamanı kalıcılığı **dosya tabanlıdır** (JSON). `migrations/*.sql` içindeki PostgreSQL şeması **kavramsal/referans** veri modelidir; üretimde aktif bir veritabanı bağlantısı yoktur. Bkz. `data-models.md`.

## Mimari Tip

- **Depo tipi:** Monolith (tek Go modülü)
- **Giriş noktası:** `cmd/hissebot/main.go` — alt-komut yönlendirici (CLI) + `serve api`/`serve reports`/`serve comments` HTTP sunucuları
- **Katmanlar:** `domain` (saf DDD aggregate'leri) → `services`/`storage` (veri toplama + dosya kalıcılığı) → `ta`/`quant`/`analysis` (hesaplama) → `cmd` (orkestrasyon)
- **Ağırlık merkezi:** `internal/ta` paketi tek başına kod tabanının %66'sı (108k satır, 722 dosya, 30 alt-paket)

## Depo Yapısı (üst seviye)

```
bist/
├── cmd/
│   ├── hissebot/        # Ana CLI + HTTP sunucuları (giriş noktası)
│   └── kap-ingest/      # KAP belge ingestion ayrı komutu
├── internal/            # Tüm iş mantığı (22 alt-paket, bkz. source-tree-analysis.md)
├── pkg/                 # Dışa açık paylaşılan yardımcılar (mathutil, useragent)
├── tools/               # Codegen + audit araçları (indikatör/patern katalog üreteçleri)
├── migrations/          # Referans PostgreSQL şeması (3 dosya)
├── ops/                 # launchd plist (macOS KAP worker)
├── scripts/             # Paralel KAP ek indirme scripti
├── docs/                # Bu dokümantasyon
└── reports/             # Üretilen audit/patern raporları (versiyonlanmış)
```

## Belgeler

- [Mimari](./architecture.md) — katmanlar, bağımlılıklar, hesaplama hattı, mimari riskler
- [Kaynak Ağacı Analizi](./source-tree-analysis.md) — dizin dizin açıklamalı harita
- [API & Komut Sözleşmeleri](./api-contracts.md) — CLI alt-komutları + HTTP endpoint'leri
- [Veri Modelleri](./data-models.md) — domain aggregate'leri, dosya düzeni, referans SQL şeması
- [Geliştirme Rehberi](./development-guide.md) — kurulum, build, test, çalıştırma
- [Dağıtım Rehberi](./deployment-guide.md) — KAP worker, CI/CD, launchd

### Mevcut belgeler (proje deposundan)
- [AI Erişim Rehberi](./ai_access_guide.md) — AI ajanlarının repo verisine erişimi
- [Analiz Motoru Yol Haritası](./analysis_engine_roadmap.md) — quant/stat-ekonomik kapsam ve fazlar
- [QuantLib / Fincept Araştırma Planı](./quantlib_fincept_research_plan.md) — akademik model doğrulama ve entegrasyon fazları
- [BIST Veri Mimarisi](./bist_data_architecture.md)
- [KAP Belge Analiz Mimarisi](./kap_document_analysis_architecture.md)
- [Kod İncelemesi (2026-06-25)](./review-2026-06-25.md) — mimari/kalite bulguları
