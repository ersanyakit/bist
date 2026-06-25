# hissebot — Veri Modelleri

> Otomatik üretildi: BMAD Document Project (kapsamlı tarama) — 2026-06-25

## Önemli: İki katmanlı veri modeli

1. **Çalışma zamanı (aktif):** Hisse bazlı **yerel JSON dosyaları**. Üretimde PostgreSQL **kullanılmaz**.
2. **Referans şema (pasif):** `migrations/*.sql` — kavramsal PostgreSQL şeması; veri modelini dokümante eder ama runtime'da bağlanılmaz.

## 1. Dosya Tabanlı Düzen (gerçek kalıcılık)

Kök: `data/equities/{TICKER}/` — her hissenin tüm verisi kendi klasöründe. `internal/storage/equity_store.go` okur/yazar.

| Yol | İçerik |
|---|---|
| `equity.json` | Hisse ana JSON dosyası (aggregate kök) |
| `kap.json` | KAP şirket verisi |
| `mkk.json` / `mkk_company_info.json` | MKK eşleşme + şirket detayı |
| `ohlcv.json` | TradingView anlık OHLCV |
| `tradingview/{FEED}.json` | TradingView scanner ham feed'leri |
| `charts/{INTERVAL}.json` | TradingView mum verisi |
| `financials/bilanco.json` | Birleştirilmiş bilanço |
| `financials/bilanco_hesaplari.json` | Hesaplanan finansal oranlar |
| `analysis/{YYYY-MM-DD}/` | Teknik analiz, indikatör, formasyon, grafik raporları |
| `comments.json` | Hisseye eşleşen yorumlar |
| `kap/attachments/{YEAR}/{INDEX}/` | Tekil KAP PDF/XBRL/Word/Excel ekleri |

### Merkezi KAP cache (`data/equities/_kap/`)
| Yol | İçerik |
|---|---|
| `details/{TICKER}/{INDEX}.json` | KAP bildirim detay + ek listesi cache'i |
| `attachments_manifest.jsonl` | İndirilen eklerin path/byte/sha256/kaynak kayıtları |
| `attachments_failures.jsonl` | İndirilemeyen ekler (tekrar denenir) |
| `document_registry.json` | Faz 1 belge arşiv kaydı (document_id, metadata, checksum, version, latest flag) |
| `extraction_jobs.json` | Belge arşivleme job geçmişi |

## 2. Domain Aggregate'leri (`internal/domain`)

Saf Go struct'ları, altyapı import etmez. Temel tipler:

**`marketdata/models.go`** — `OHLCV`, `PriceSnapshot`, `OrderBook`, `OrderBookLevel`, `TradeSummary`, `CorporateAction`, `IndexConstituent`, `LiveIndexQuote`, `LiveSymbolSnapshot`, `LiveMarketSnapshot`, `MQTTPublishSample`, `DataCapability`

**`stocks/models.go`** — `Stock`, `OwnershipStake`, `Subsidiary`, `SegmentBreakdown`, `FXPosition`, `DebtMaturityBucket`, `WorkingCapitalBreakdown`, `ReferenceData`

**`financials/models.go`** — `Series`, `Observation`, `Point`, `Revision`, `Period`, `SourceMeta`

**`disclosures/models.go`** — `Disclosure`, `MaterialEvent`

**`documents/`, `kapextract/`, `macro/`** — belge metadata, ingestion job/error, makro serileri

Tüm paketlerde ortak `SourceMeta` deseni: her veri parçası kaynağını (source, source_url, as_of_date, data_version) taşır → izlenebilirlik.

## 3. Referans SQL Şeması (`migrations/`)

Çalışma zamanında uygulanmaz; veri modelinin kanonik tanımıdır.

### `001_market_financial_macro_schema.sql`
- `stocks` (ticker PK, isin, company_name, sector, free_float_ratio, shares_outstanding, as_of_date, data_version)
- `stock_prices` (ticker+timeframe+ts+data_version PK; OHLCV + vwap + trade_count). OHLCV tutarlılığı CHECK kısıtlarıyla zorlanır (`high >= open/close/low`, `low <= ...`, değerler `>= 0`).
- Finansal + makro tabloları.

### `002_kap_document_archive_schema.sql`
KAP belge arşivi (document_registry'nin SQL karşılığı): document_id, metadata, local path, checksum, version, latest flag.

### `003_kap_extraction_analysis_schema.sql`
KAP belge zekası çıkarım + analiz şeması.

## 4. Veri Versiyonlama & İzlenebilirlik

- Her kayıt `data_version` ve `as_of_date` taşır → tarihsel revizyon takibi.
- `SourceMeta` ile her veri parçasının kaynağı geri izlenebilir.
- `attachments_manifest.jsonl` sha256 ile içerik bütünlüğünü doğrular.
- KAP belgeleri `version` + `latest` flag ile sürümlenir (aynı bildirimin güncellemeleri).
