# hissebot — API & Komut Sözleşmeleri

> Otomatik üretildi: BMAD Document Project (kapsamlı tarama) — 2026-06-25

İki arayüz vardır: **CLI alt-komutları** (birincil) ve **HTTP sunucuları** (`serve api` / `serve reports`).

## 1. CLI Alt-Komutları

Giriş: `cmd/hissebot/main.go` — `run()` içinde `args[0]` üzerinde flat `switch`. Her komut kendi `flag.FlagSet`'ini ayrıştırır.

### Veri tohumlama (seed)
| Komut | İş |
|---|---|
| `seed kap` | KAP şirket verisi tohumla |
| `seed sirketler` | Şirket listesi tohumla |

### Senkronizasyon (sync) — dış kaynaklardan veri çekme
| Komut | Kaynak |
|---|---|
| `sync tradingview` | TradingView scanner feed |
| `sync ohlcv` | Anlık OHLCV |
| `sync charts` | Mum (candle) verisi |
| `sync bist-bulletin-db -from -to` | Resmi BIST bülten DB |
| `sync mkk` | MKK eşleşme/şirket bilgisi |
| `sync kap-sectors` / `sync sectors` | Sektör sınıflandırma |
| `sync news` | Haber |
| `sync kap-disclosures -from -disclosure-types` | KAP bildirim metadata |
| `sync kap-attachments [-repeat -delay -retries ...]` | KAP PDF/XBRL/Word/Excel ekleri (dayanıklı, retry'lı worker) |
| `sync kap-document-archive` | Faz 1 belge arşivi |
| `sync kap-extract -ticker X` | Belge zekası çıkarımı |
| `sync tuik-gdp` | TÜİK GSYİH |
| `sync all` / `sync all-data` | Tüm kaynaklar |

### Finansallar
| Komut | İş |
|---|---|
| `financials import` | Ham bilanço içeri al |
| `financials calculate` | Oranları hesapla → `bilanco_hesaplari.json` |
| `financials run -force` | Import + calculate zinciri |
| `financials fetch` / `merge` / `metadata` / `reconcile` | Alt adımlar |

### Analiz
| Komut | İş |
|---|---|
| `analyze -symbol X [-provider bistdb -timeframes 1D,1W,1M]` | Tek hisse analizi |
| `analyze -all` | Tüm evren |
| `forecast-audit` | Forecast doğruluk denetimi |

### Denetim (audit)
| Komut | İş |
|---|---|
| `audit enterprise -mode research` | Kurumsal denetim |
| `audit analysis-readiness -symbol X` | Analiz hazırlık denetimi |
| `analysis-readiness` / `analysis` / `decision-readiness` | Karar hazırlık |

### Bakım & servis
| Komut | İş |
|---|---|
| `migrate layout` | Veri klasör düzeni göçü |
| `universe` | Hisse evreni yönetimi |
| `serve comments` | Yorum sunucusu |
| `serve reports` | Rapor sunucusu |
| `serve api` | HTTP API sunucusu |
| `help` / `-h` / `--help` | Yardım |

## 2. HTTP Endpoint'leri

`serve api` (Fiber v3, `cmd/hissebot/api_server.go`) ve rapor sunucusu (`report_server.go`) tarafından sunulur. codebase-memory graf çıkarımından tespit edilen rotalar:

| Metot | Yol | Amaç |
|---|---|---|
| GET | `/healthz` | Sağlık kontrolü |
| GET | `/sectors` | Tüm sektörler |
| GET | `/sectors/:symbol` | Hisse sektörü |
| GET | `/sector-groups` | Sektör grupları |
| GET | `/sector-classifications` | Sektör sınıflandırmaları |
| GET | `/sector-classifications/:symbol` | Hisse sınıflandırması |
| ANY | `/` | Kök |
| ANY | `/reports` | Rapor indeksi |
| ANY | `/companies/` | Şirketler |
| ANY | `/documents/` | Belgeler |
| ANY | `/kap-ingest`, `/api/kap-ingest/run` | KAP ingestion tetikleme |
| ANY | `/extraction/jobs`, `/extraction/jobs/` | Çıkarım job'ları |

> Not: Handler imzaları graf çıkarımında boş göründü; tam istek/yanıt şemaları için `api_server.go` ve `report_server.go` okunmalı. Kimlik doğrulama/oturum deseni tespit edilmedi (yerel/iç kullanım sunucusu).

## 3. Kullanım Örnekleri

```bash
# Tek hisse uçtan uca
go run ./cmd/hissebot sync all-data
go run ./cmd/hissebot financials run -force
go run ./cmd/hissebot analyze -symbol ASELS -provider bistdb -timeframes 1D,1W,1M

# KAP ekleri dayanıklı worker
go run ./cmd/hissebot sync kap-attachments -repeat -delay 2s -retries 2 -newest-first
WORKERS=4 ./scripts/sync_kap_attachments_parallel.sh

# Sunucu
go run ./cmd/hissebot serve api
curl localhost:PORT/healthz
```
