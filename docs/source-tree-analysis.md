# hissebot — Kaynak Ağacı Analizi

> Otomatik üretildi: BMAD Document Project (kapsamlı tarama) — 2026-06-25

## Açıklamalı Dizin Haritası

```
bist/
├── cmd/
│   ├── hissebot/                  # ⭐ GİRİŞ NOKTASI
│   │   ├── main.go                #   Alt-komut yönlendirici (1088 satır, flat dispatch)
│   │   ├── api_server.go          #   `serve api` — Fiber v3 HTTP API
│   │   ├── report_server.go       #   `serve reports` / `serve comments` sunucusu
│   │   ├── analyze.go             #   `analyze` komutu orkestrasyonu
│   │   ├── forecast_audit.go      #   `forecast-audit` (2187 satır)
│   │   └── data/seed/             #   kap_sectors.json (versiyonlanmış seed)
│   └── kap-ingest/main.go         #   Ayrı KAP belge ingestion komutu
│
├── internal/                      # Tüm iş mantığı (dışa kapalı)
│   ├── domain/                    # ⭐ Saf DDD aggregate'leri (altyapı import ETMEZ)
│   │   ├── marketdata/            #   OHLCV, OrderBook, LiveMarketSnapshot, CorporateAction
│   │   ├── financials/            #   Bilanço/oran modelleri
│   │   ├── stocks/                #   Stock, OwnershipStake, Subsidiary, FXPosition
│   │   ├── disclosures/           #   KAP bildirim, MaterialEvent
│   │   ├── documents/, kapextract/, macro/
│   │
│   ├── services/                  # ⭐ Dış kaynak veri toplayıcıları (34 dosya)
│   │   ├── tradingview/           #   OHLCV, charts, scanner feed (charts.go panik içerir)
│   │   ├── kap/                   #   KAP bildirim/şirket
│   │   ├── tcmb/                  #   TCMB EVDS makro
│   │   ├── tuik/                  #   TÜİK GSYİH/enflasyon
│   │   ├── mkk/, news/, financials/, pricequality/, matriksformations/
│   │
│   ├── storage/equity_store.go    # ⭐ Dosya tabanlı kalıcılık (JSON I/O)
│   ├── repositories/              #   Port arayüzleri (contracts) + memory/filedocuments adaptör
│   │
│   ├── ta/                        # ⭐⭐ TEKNİK ANALİZ MOTORU (722 dosya, %66)
│   │   ├── analysis/engine.go     #   Ana analiz motoru (6057 satır)
│   │   ├── storage/               #   professional_report.go (9549!), writer.go (4007)
│   │   ├── professional/          #   professional.go (5196), AnalyzeSymbol
│   │   ├── investorqa/            #   investorqa.go (5121)
│   │   ├── indicators/            #   indicators.go + scanner_matchers.go (2646)
│   │   ├── patterns/              #   scanner_matchers.go (2172) + generated/ (codegen)
│   │   ├── formations/            #   engine.go (2217)
│   │   ├── chart/renderer.go      #   Görsel grafik render (2201)
│   │   ├── forecastpolicy/, ensemble/, ml/, backtest/, risk/, value/
│   │   └── contrarian/, corporateactions/, macro/, supportresistance/ ...
│   │
│   ├── quant/                     # ⭐ Kuantitatif matematik (13 dosya)
│   │   ├── portfolio/             #   BlackLitterman, posterior returns
│   │   ├── rates/                 #   Discount curve bootstrap, Vasicek kalibrasyonu
│   │   ├── volatility/            #   Vol yüzeyi
│   │   ├── instruments/, solver/  #   Bond future CTD, bisection
│   │
│   ├── kapingest/                 # KAP belge ingestion + document_intelligence.go (4038)
│   ├── analysis/                  # İnce skorlama facade'ları (fundamental/risk/technical/valuation)
│   ├── confidence/score.go        # Güven skoru + 0.75 inceleme kapısı
│   ├── wsclient/                  # Canlı market WebSocket istemcisi
│   ├── config/config.go           # 51 alanlı Config + Load()
│   ├── audit/, security/, validation/, normalization/, extraction/,
│   └── ingestion/, enterprise/, kapfinance/, ops/, datasources/, util/
│
├── pkg/                           # Dışa AÇIK paylaşılan yardımcılar
│   ├── mathutil/                  #   Max/Min/Clamp/SafeDiv (en yüksek fan-in)
│   └── useragent/                 #   HTTP user-agent rotasyonu
│
├── tools/                         # Codegen + audit (ana binary'ye dahil değil)
│   ├── indicator_catalog_gen/     #   İndikatör kataloğu üreteci
│   ├── pattern_catalog_gen/       #   Patern kataloğu üreteci
│   ├── pattern_audit_report/, analysis_report_audit/
│
├── migrations/                    # Referans PostgreSQL şeması (çalışma zamanında kullanılmaz)
│   ├── 001_market_financial_macro_schema.sql
│   ├── 002_kap_document_archive_schema.sql
│   └── 003_kap_extraction_analysis_schema.sql
│
├── ops/launchagents/              # macOS launchd plist (KAP attachment worker)
├── scripts/sync_kap_attachments_parallel.sh
├── .github/workflows/             # kap-attachment-worker.yml (tek workflow)
├── go.mod / go.sum                # Modül: hissebot, Go 1.26
├── prompt.json                    # 80KB — LLM prompt verisi (git-tracked)
└── README.md                      # Komut listesi + veri düzeni (Türkçe)
```

## Kritik Giriş Noktaları

| Amaç | Dosya | Tetikleyici |
|---|---|---|
| CLI / tüm komutlar | `cmd/hissebot/main.go` | `go run ./cmd/hissebot <komut>` |
| HTTP API | `cmd/hissebot/api_server.go` | `serve api` |
| Rapor/yorum sunucusu | `cmd/hissebot/report_server.go` | `serve reports`, `serve comments` |
| Analiz motoru | `internal/ta/analysis/engine.go` | `analyze` komutu |
| KAP ek worker | `cmd/hissebot` (`sync kap-attachments`) | launchd / GitHub Actions |

## Yapısal Notlar

- **`internal/ta` bir mega-pakettir** (722/978 dosya). Sorumluluk bazı kavramlar için birden çok yerde tekrarlanır: risk üç yerde (`ta/risk`, `analysis/risk`, `quant`), analiz iki yerde (`ta/analysis` ağır vs `internal/analysis` ince). Yeni metrik eklerken kanonik ev belirsiz.
- **Codegen katmanı:** `internal/ta/{indicators,patterns}/generated/` codebase-memory indekslemesinden hariç tutuldu (üretilmiş lookup tabloları).
- `tools/` ve `cmd/kap-ingest` ana binary'den bağımsız çalıştırılır.
