# AI Access Guide

Bu dokuman, AI ajanlari ve baska projelerin bu repodaki veri ve kodlara guvenli, tekrar edilebilir ve semantik olarak tutarli erismesi icin sozlesmedir. Ana ilke: veri uretimi mevcut Go komutlariyla yapilir, AI tarafinda okuma ve ozetleme dosya/HTTP/MCP katmanlari uzerinden yapilir.

## Kisa Ozet

- Repo lokal JSON dosyalariyla calisir; PostgreSQL zorunlu degildir.
- Hisse verileri `data/equities/{TICKER}/` altinda, kripto verileri `data/crypto/{SYMBOL}/` altinda tutulur.
- Teknik analiz icin canonical dosya `analysis/{YYYY-MM-DD}/analysis.json` dosyasidir.
- Turkce rapor uyumlu dosya `analysis/{YYYY-MM-DD}/analiz.json` dosyasidir.
- Rapor grafikleri ayni analiz klasorundeki `grafik_*.png`, `chart_*.html`, `rapor.html` ve `rapor.pdf` dosyalaridir.
- KAP belge arama ve deterministik cikarmalar `data/equities/_kap/` ve `data/equities/{TICKER}/kap/extraction/` altindadir.
- AI ajanlari dosyalari okumali, veri uretmek icin ise `go run ./cmd/hissebot ...` komutlarini veya rapor HTTP servisini kullanmalidir.
- Uzak veya cok ajanli erisim icin onerilen katman MCP server'dir; kaynak ve tool sozlesmesi bu dokumanin sonunda tanimlidir.

## Mevcut Veri Kapsami

Bu workspace icin son gozleme gore:

- `data/equities/`: 1079 sembol klasoru.
- `data/equities/*/ohlcv.json`: yaklasik 600 hisse icin OHLCV dosyasi.
- `data/equities/*/financials/bilanco.json`: yaklasik 1077 hisse icin finansal veri.
- `data/equities/*/analysis/`: analiz uretilmis hisse klasorleri.
- `data/crypto/`: BTCUSDT ve CHZUSDT gibi kripto varlik klasorleri.

Bu sayilar repo guncellendikce degisir; AI ajanlari sayiya guvenmek yerine dizin taramasi yapmalidir.

## Ana Dizinler

| Yol | Amac | AI icin kullanim |
| --- | --- | --- |
| `data/equities/{TICKER}/equity.json` | Hisse ana karti ve kaynak referanslari | Sembol, unvan, kaynak varligi kontrolu |
| `data/equities/{TICKER}/ohlcv.json` | TradingView scanner OHLCV ozeti | Hizli fiyat verisi |
| `data/equities/{TICKER}/tradingview/{FEED}.json` | TradingView ham feed dosyalari | Kaynak dogrulama |
| `data/equities/{TICKER}/charts/{INTERVAL}.json` | TradingView chart mumlari | Intraday/gunluk mum inceleme |
| `data/equities/{TICKER}/financials/bilanco.json` | Birlesik finansal tablolar | Temel analiz |
| `data/equities/{TICKER}/financials/bilanco_hesaplari.json` | Hesaplanmis oranlar | Rasyo ve skor analizi |
| `data/equities/{TICKER}/kap.json` | KAP sirket kaydi | Sirket metadata |
| `data/equities/{TICKER}/kap_disclosures.json` | KAP bildirim listesi | Bildirim zaman cizelgesi |
| `data/equities/{TICKER}/kap/attachments/` | PDF/XBRL/Word/Excel ekleri | Belge inceleme, kaynak kanit |
| `data/equities/{TICKER}/kap/extraction/extraction_result.json` | Belge metin/fact/olay/varlik cikarmalari | Deterministik KAP bilgi tabani |
| `data/equities/{TICKER}/analysis/{DATE}/analysis.json` | Canonical teknik/profesyonel analiz | AI icin ana analiz sozlesmesi |
| `data/equities/{TICKER}/analysis/{DATE}/analiz.json` | Turkce analiz gorunumu | Kullaniciya yakin ozet |
| `data/crypto/{SYMBOL}/analysis/{DATE}/analysis.json` | Kripto teknik analiz | BTC/altcoin analizleri |
| `data/seed/kap_sectors.json` | KAP `/tr/Sektorler` resmi sektor agaci ve sembol eslesmeleri | Sektor icin referans kaynak |
| `data/seed/sector_classifications.json` | KAP referansli sektor, industry ve peer evreni | Analiz/peer lookup |
| `data/seed/*.json` | Global seed, sektor, peer, varsayim verileri | Evren ve lookup verisi |
| `data/macro/*.json` | Makro veriler | Makro katman |
| `data/audit/` | Audit loglari | Veri uretim gecmisi |
| `reports/` | Ara audit/rapor ciktilari | Sistem denetimi |

## Canonical Analysis JSON

AI ajanlari teknik analiz icin once su dosyayi okumalidir:

```text
data/equities/{TICKER}/analysis/{YYYY-MM-DD}/analysis.json
data/crypto/{SYMBOL}/analysis/{YYYY-MM-DD}/analysis.json
```

Onemli alanlar:

| Alan | Aciklama |
| --- | --- |
| `symbol` | Normalize sembol |
| `exchange` | Kaynak borsa, ornek `BIST`, `BINANCE` |
| `asset_type` | `equity` veya `crypto` |
| `company_name` | Sirket/varlik adi |
| `analysis_date` | Analiz uretim tarihi |
| `currency` | Para birimi |
| `timeframes` | `1D`, `1W`, `1M` gibi periyot analizleri |
| `overall_score` | Entegre skor |
| `overall_bias` | `bullish`, `bearish`, `neutral` |
| `professional` | Kurumsal/profesyonel rapor katmani |
| `behavioral` | Tersine donus/sentiment/kapitulasyon katmani |
| `investor_qa` | Yatirimci sorularina cevap katmani |
| `institutional_validation` | Kalite kapilari |
| `disclaimer` | Risk uyarisi |

`timeframes.{TF}` altindaki temel alanlar:

| Alan | Aciklama |
| --- | --- |
| `candles` | OHLCV mum listesi |
| `last_close`, `last_volume` | Son kapanis ve hacim |
| `indicators` | SMA/EMA/RSI/MACD/ATR/Ichimoku ve ek indikatorler |
| `indicator_signals` | Taranmis indikator sinyalleri |
| `patterns` | Aksiyonlanabilir/one cikan formasyonlar |
| `pattern_candidates` | Eleme sonrasi adaylar |
| `pattern_scans` | Tum pattern tarama sonuclari |
| `support_levels` | Destek seviyeleri |
| `resistance_levels` | Direnc seviyeleri |
| `nearest_support`, `nearest_resistance` | En yakin aktif destek/direnc |
| `trade_plan` | Risk motoru sonucu; `rejected:true` olabilir |
| `professional` | Timeframe bazli profesyonel metrikler |
| `trend_bias` | Timeframe yon egilimi |
| `score` | Timeframe skoru |

## OHLCV Candle Sozlesmesi

Mum nesnesi:

```json
{
  "time": "2026-06-14T00:00:00Z",
  "open": 64458.01,
  "high": 65666,
  "low": 63678.83,
  "close": 65585.42,
  "volume": 12479.92649,
  "adjusted_open": 0,
  "adjusted_high": 0,
  "adjusted_low": 0,
  "adjusted_close": 0,
  "adjusted_volume": 0,
  "is_adjusted": false
}
```

AI ajanlari fiyat hesaplarken `is_adjusted:true` ve adjusted alanlari pozitifse adjusted degerleri kullanmalidir. Aksi halde normal `open/high/low/close/volume` alanlari esas alinmalidir.

## Teknik Analiz Okuma Sirasi

Bir AI ajani tek sembol analizi icin su sirayi izlemelidir:

1. Sembol tipini belirle:
   - Hisse: `data/equities/{TICKER}/`
   - Kripto: `data/crypto/{SYMBOL}/`
2. En guncel analiz klasorunu bul:
   - `analysis/` altindaki `YYYY-MM-DD` klasorlerini tarihe gore sirala.
3. `analysis.json` oku.
4. Kullanici Turkce ozet istiyorsa `analiz.json`, `ozet.md` veya `summary.md` oku.
5. Grafik gerekirse ayni klasorden `grafik_{TF}.png`, `grafik_karar_{TF}.png`, `grafik_detay_{TF}.png` veya `chart_{TF}.html` kullan.
6. Analiz yoksa veri uretim komutunu calistir veya rapor HTTP servisinden rapor uret.

Ornek:

```bash
go run ./cmd/hissebot analyze -symbol ASELS -timeframes 1D,1W,1M
go run ./cmd/hissebot analyze -symbol BTCUSDT -timeframes 1D,1W,1M
```

Kripto analizleri, `-out data/equities` kullanilsa bile `data/crypto/{SYMBOL}/analysis/{DATE}/` altina yazilir.

## Temel Analiz ve KAP Okuma Sirasi

Hisse icin temel veri okurken:

1. `equity.json` ile sirket varligini ve metadata durumunu kontrol et.
2. `financials/bilanco.json` ile finansal tabloları oku.
3. `financials/bilanco_hesaplari.json` ile hesaplanmis rasyolari oku.
4. `kap_disclosures.json` ile bildirim gecmisini oku.
5. Belge kaynakli kanit gerekiyorsa `kap/extraction/extraction_result.json` oku.
6. Dosya yoksa once asagidaki veri uretim komutlari gerekir.

Komutlar:

```bash
go run ./cmd/hissebot financials run -ticker ASELS
go run ./cmd/hissebot sync kap-disclosures -from 2010-01-01 -disclosure-types all
go run ./cmd/hissebot sync kap-attachments -ticker ASELS
go run ./cmd/hissebot sync kap-document-archive -ticker ASELS
go run ./cmd/hissebot sync kap-extract -ticker ASELS
```

KAP extract sonucu deterministik aday veri uretir. `validation_status:"unknown"` veya `review_required:true` olan degerler kesin finansal veri gibi sunulmamalidir.

## Kod Haritasi

| Yol | Sorumluluk |
| --- | --- |
| `cmd/hissebot/main.go` | CLI komut router'i |
| `cmd/hissebot/analyze.go` | Teknik/profesyonel analiz komutu |
| `cmd/hissebot/report_server.go` | Lokal HTTP rapor servisi |
| `internal/ta/analysis/engine.go` | Analiz motoru, timeframe skoru, rapor orkestrasyonu |
| `internal/ta/ohlcv/types.go` | OHLCV, indikator, pattern, destek/direnc, trade plan sozlesmeleri |
| `internal/ta/indicators/` | Indikator hesaplari ve sinyal tarama |
| `internal/ta/patterns/` | Formasyon tarama motoru |
| `internal/ta/supportresistance/` | Destek/direnc hesaplari |
| `internal/ta/risk/` | Trade plan ve risk motoru |
| `internal/ta/chart/` | PNG grafik renderer'lari |
| `internal/ta/storage/writer.go` | Analiz JSON/HTML/PDF/PNG yazimi |
| `internal/ta/professional/` | Kurumsal/profesyonel analiz katmani |
| `internal/ta/investorqa/` | Yatirimci soru-cevap katmani |
| `internal/extraction/` | KAP belge metin/fact/olay/varlik cikarma |
| `internal/storage/` | Hisse dosya store'u |
| `internal/services/tradingview/` | TradingView feed/chart isleme |
| `tools/analysis_report_audit/` | Analysis JSON kalite denetimi |

## HTTP Erisim

Lokal rapor servisi:

```bash
HISSEBOT_COMMAND_TIMEOUT=45m go run ./cmd/hissebot serve reports -addr 127.0.0.1:1453
```

Rapor uret:

```bash
curl -X POST http://127.0.0.1:1453/reports \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"ASELS","provider":"tradingview","mode":"production","timeframes":["1D","1W","1M"]}'
```

KAP extract endpointleri:

```bash
curl "http://127.0.0.1:1453/companies/ASELS/info-card"
curl "http://127.0.0.1:1453/companies/ASELS/financials"
curl "http://127.0.0.1:1453/companies/ASELS/management"
curl "http://127.0.0.1:1453/companies/ASELS/assets"
curl "http://127.0.0.1:1453/companies/ASELS/events"
curl "http://127.0.0.1:1453/companies/ASELS/risks"
curl "http://127.0.0.1:1453/companies/ASELS/analysis/fundamental"
```

KAP PDF ingestion endpointi:

```bash
curl -X POST http://127.0.0.1:1453/api/kap-ingest/run \
  -H 'Content-Type: application/json' \
  -d '{"input":"data/equities","output":"data/processed","workers":4,"limit":10,"resume":true,"llm":false,"dry_run":false}'
```

Bu endpoint `internal/kapingest` pipeline'ini çalıştırır. `llm:false` sadece `raw_documents.jsonl`, `processed_files.jsonl` ve `extraction_errors.jsonl` üretir. `llm:true` ayrıca `kap_events.jsonl` üretir; gerçek LLM client bağlanmadığında event etkisi `uncertain` kalan deterministik MVP çıkarımıdır. AI ajanları `quality_score < 0.35` veya `low_text_quality_possible_scanned_pdf` uyarısı olan kayıtları OCR/review adayı saymalı, bu metinlerden kesin finansal veri üretmemelidir.

Teknik/fundamental rapor motoru sembol bazlı ingest çıktısını otomatik tüketir. Beklenen öncelikli yol `data/processed/{ticker_lower}/raw_documents.jsonl`; toplu çıktı kullanılıyorsa `data/processed/raw_documents.jsonl` içinden ticker/path filtresi yapılır. Rapor JSON'unda makine okunur alanlar:

- `professional.kap_pdf_ingest`
- Türkçe JSON için `profesyonel_analiz.kap_pdf_ingest`
- HTML/PDF ilk sayfa için `KAP PDF Raporları`

Bu alan belge tipi dağılımı, ortalama metin kalite skoru, düşük kalite/OCR adayı sayısı, ingest hata sayısı ve öne çıkan PDF kanıtlarını içerir. AI ajanları bu alanı belge kapsamı kanıtı olarak kullanmalı; düşük kalite kayıtları kesin bilanço/değerleme verisi gibi yorumlamamalıdır.

Tek tus rapor endpointi lokal disina acilacaksa `HISSEBOT_ENDPOINT_TOKEN` kullanir. Fiber v3 sektor API'si auth/token gerektirmez.

## HTTP API

Fiber v3 JSON API sektor datasini diger projelere dogrudan acar:

```bash
go run ./cmd/hissebot serve api -addr 127.0.0.1:1454
```

| Endpoint | Donus |
| --- | --- |
| `GET /healthz` | Servis durumu |
| `GET /api/v1/sectors` | KAP resmi sektor agaci ve sembol eslesmeleri |
| `GET /api/v1/sectors/{SYMBOL}` | Tek sembol icin KAP sektor kaydi ve turetilmis siniflandirma |
| `GET /api/v1/sector-groups` | KAP ana sektor/alt sektor bazli sembol gruplari |
| `GET /api/v1/sector-classifications` | Analiz motorunun kullandigi sektor/peer dosyasi |
| `GET /api/v1/sector-classifications/{SYMBOL}` | Tek sembol icin sektor/peer kaydi |

Sektor icin referans KAP'tir. `sector_classifications.json` icinde `source:"kap_sector_page"` bulunan kayitlarda TradingView veya eski override KAP verisini ezmemelidir.

## AI Davranis Kurallari

- Kesin yatirim onerisi verme; analizlerde `disclaimer` alanini koru.
- `trade_plan.rejected:true` ise bunu aktif islem plani gibi yorumlama.
- `asset_type:"crypto"` icin on-chain, derivatives veya exchange-flow yoksa bunu veri siniri olarak belirt.
- `pattern_candidates` ve `pattern_scans` alanlarini kesin sinyal degil, aday/tarama sonucu olarak ele al.
- Destek/direnc seviyelerinde `touch_count` ve `strength` alanlarini birlikte kullan.
- KAP extract icindeki review gerektiren alanlari dogrulanmis veri gibi sunma.
- Dosya yoksa veri yok de; sahte veri uretme.
- Buyuk dosyalarda once ozet alanlari oku, sonra gerekli alt bolume in.
- Yazma islemlerinde mevcut dosyalari silme veya uzerine yazma; yeni cikti icin tarihli/ayri dosya kullan.

## Onerilen MCP Server Tasarimi

AI ve baska projelerin bu repo ile standart protokol uzerinden calismasi icin MCP server eklenebilir. Ilk surum salt-okuma agirlikli olmalidir; veri uretimi kontrollu tool olarak ayrica sunulmalidir.

### Resource URI Sozlesmesi

| URI | Donus |
| --- | --- |
| `hissebot://catalog/equities` | Hisse sembol listesi ve temel dosya varliklari |
| `hissebot://catalog/crypto` | Kripto sembol listesi |
| `hissebot://equity/{symbol}/profile` | `equity.json`, KAP/MKK ozetleri |
| `hissebot://equity/{symbol}/ohlcv` | `ohlcv.json` veya chart datasindan normalize OHLCV |
| `hissebot://equity/{symbol}/financials` | `financials/bilanco.json` ve oran dosyasi ozeti |
| `hissebot://equity/{symbol}/kap-disclosures` | KAP bildirim listesi |
| `hissebot://equity/{symbol}/kap-extraction` | `kap/extraction/extraction_result.json` |
| `hissebot://sectors/kap` | `data/seed/kap_sectors.json` |
| `hissebot://sectors/classifications` | `data/seed/sector_classifications.json` |
| `hissebot://sectors/{symbol}` | Sembol bazli KAP sektor ve siniflandirma kaydi |
| `hissebot://analysis/{asset_type}/{symbol}/latest` | En guncel `analysis.json` |
| `hissebot://analysis/{asset_type}/{symbol}/{date}` | Tarihli `analysis.json` |
| `hissebot://analysis/{asset_type}/{symbol}/{date}/{timeframe}` | Tek timeframe analizi |
| `hissebot://chart/{asset_type}/{symbol}/{date}/{timeframe}` | Grafik dosya referanslari |
| `hissebot://seed/{name}` | `data/seed/{name}.json` |
| `hissebot://macro/{name}` | `data/macro/{name}.json` |

### Tool Sozlesmesi

| Tool | Parametreler | Islev |
| --- | --- | --- |
| `list_symbols` | `asset_type?`, `has_ohlcv?`, `has_analysis?` | Sembol katalogu |
| `get_latest_analysis` | `symbol`, `asset_type?`, `timeframe?` | En guncel analiz |
| `get_ohlcv` | `symbol`, `asset_type?`, `timeframe?`, `limit?` | Normalize mum verisi |
| `get_financials` | `symbol`, `sections?` | Finansal tablo/rasyo |
| `search_kap_disclosures` | `symbol`, `from?`, `to?`, `query?`, `limit?` | KAP bildirim arama |
| `get_kap_extraction` | `symbol`, `sections?` | Extract sonucunu bolumlu dondurme |
| `get_sectors` | `symbol?`, `group_by?` | KAP referansli sektor ve peer verisi |
| `generate_analysis` | `symbol`, `timeframes`, `provider?`, `mode?`, `limit?` | Mevcut analiz motorunu calistirma |
| `audit_analysis` | `analysis_path`, `spot_only?` | Audit tool'unu calistirma |

### MCP Guvenlik Modeli

- Varsayilan mod `read_only:true` olmali.
- `generate_analysis` ve `audit_analysis` gibi komut calistiran tool'lar `allow_commands:true` olmadan acilmamali.
- Path traversal engellenmeli: kullanici path degil sembol/URI vermeli.
- Resource cevaplari buyukse sayfalama veya `summary:true` destegi olmali.
- Binary dosyalar dogrudan JSON icine base64 gomulmemeli; dosya referansi ve MIME metadata donmeli.
- Token/API key gibi ortam degiskenleri resource cevabina dahil edilmemeli.

### Onerilen Paket Yapisi

```text
cmd/hissebot-mcp/
  main.go
internal/mcp/
  server.go
  resources.go
  tools.go
  catalog.go
  schemas.go
```

MCP server, mevcut business logic'i tekrar yazmamalidir. Dosya okuma icin `internal/storage`, analiz uretimi icin `internal/ta/analysis`, rapor yazimi icin `internal/ta/storage` kullanilmalidir.

## Ornek AI Gorevleri

### En guncel BTC teknik durumunu oku

1. `hissebot://analysis/crypto/BTCUSDT/latest` resource'unu oku.
2. `timeframes.1D.last_close`, `nearest_support`, `nearest_resistance`, `indicators.ema20`, `indicators.ema50` alanlarini cikar.
3. `patterns` icinde `confidence >= 0.55` olanlari ozetle.
4. `disclaimer` metnini koru.

### ASELS temel ve teknik raporu hazirla

1. `hissebot://equity/ASELS/profile`
2. `hissebot://equity/ASELS/financials`
3. `hissebot://analysis/equity/ASELS/latest`
4. KAP kaniti gerekiyorsa `hissebot://equity/ASELS/kap-extraction`
5. Veri kalite uyarilarini `institutional_validation` ve KAP `review_required` alanlarindan yaz.

### Yeni rapor uret

1. MCP tool: `generate_analysis`
2. Parametre:

```json
{
  "symbol": "BTCUSDT",
  "timeframes": ["1D", "1W", "1M"],
  "provider": "tradingview",
  "mode": "production",
  "limit": 260
}
```

3. Donen `analysis_path`, `html_path`, `pdf_path` referanslarini kullan.

## Bakim Notlari

- Yeni veri dosyasi eklendiginde bu rehberdeki "Ana Dizinler" tablosu guncellenmeli.
- `analysis.SymbolAnalysis` veya `ohlcv` tipleri degisirse "Canonical Analysis JSON" ve "OHLCV Candle Sozlesmesi" bolumleri guncellenmeli.
- MCP implementasyonu yapildiginda resource URI'leri ve tool parametreleri bu dokumandaki sozlesmeyi bozmamali; gerekiyorsa yeni versiyon `v2` olarak eklenmeli.
- Bu dokuman AI ajanlari icin kaynak sozlesmesidir; kullaniciya sunulan rapor metinleri `rapor.html`, `ozet.md` ve `analiz.json` icinden uretilmelidir.
