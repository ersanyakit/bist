# hissebot

PostgreSQL kullanmayan, Go tarafında lokal JSON dosyalarıyla çalışan port. Her hissenin tüm datası kendi klasöründe saklanır.

AI ajanları ve diğer projelerin repo verilerine dosya/HTTP/MCP yaklaşımıyla erişmesi için bkz. `docs/ai_access_guide.md`.

## Komutlar

```bash
go run ./cmd/hissebot seed kap
go run ./cmd/hissebot seed sirketler
go run ./cmd/hissebot financials import
go run ./cmd/hissebot financials calculate
go run ./cmd/hissebot financials run -force
go run ./cmd/hissebot sync tradingview
go run ./cmd/hissebot sync ohlcv
go run ./cmd/hissebot sync charts
go run ./cmd/hissebot sync all-data
go run ./cmd/hissebot sync mkk
go run ./cmd/hissebot sync kap-sectors
go run ./cmd/hissebot sync sectors
go run ./cmd/hissebot sync news
go run ./cmd/hissebot sync kap-disclosures -from 2010-01-01 -disclosure-types all
go run ./cmd/hissebot sync kap-attachments -repeat -pass-delay 5m -newest-first -delay 2s -error-delay 15s -transient-error-sleep 20m -transient-error-threshold 5 -retries 2 -rate-limit-sleep 20m -min-free-bytes 0
WORKERS=4 ./scripts/sync_kap_attachments_parallel.sh
go run ./cmd/hissebot sync kap-document-archive
go run ./cmd/hissebot sync kap-extract -ticker ASELS
go run ./cmd/hissebot analyze -symbol ADEL
go run ./cmd/hissebot analyze -all
go run ./cmd/hissebot audit enterprise -mode research
go run ./tools/analysis_report_audit -path data/equities/ASELS/analysis/2026-06-13/analysis.json -spot-only=true
go run ./cmd/hissebot migrate layout
go run ./cmd/hissebot serve comments
go run ./cmd/hissebot serve api
```

## Veri Düzeni

- `data/equities/{TICKER}/equity.json`: hisse bazlı ana JSON dosyası
- `data/equities/{TICKER}/kap.json`: KAP şirket verisi
- `data/equities/{TICKER}/mkk.json`: MKK eşleşme durumu ve MKK ID bilgisi
- `data/equities/{TICKER}/mkk_company_info.json`: MKK şirket detay cevabı
- `data/equities/{TICKER}/ohlcv.json`: TradingView anlık OHLCV dosyası
- `data/equities/{TICKER}/tradingview/{FEED}.json`: TradingView scanner ham feed dosyaları
- `data/equities/{TICKER}/charts/{INTERVAL}.json`: TradingView chart mum verileri
- `data/equities/{TICKER}/financials/bilanco.json`: birleştirilmiş bilanço verisi
- `data/equities/{TICKER}/financials/bilanco_hesaplari.json`: hesaplanan finansal oranlar
- `data/equities/{TICKER}/analysis/{YYYY-MM-DD}/`: teknik analiz, indikatör, formasyon ve grafik raporları
- `data/equities/{TICKER}/comments.json`: hisseye eşleşen yorumlar
- `data/equities/{TICKER}/kap/attachments/{YEAR}/{DISCLOSURE_INDEX}/`: ilgili hisse klasöründeki tekil KAP PDF/XBRL/Word/Excel ekleri
- `data/equities/_kap/details/{TICKER}/{DISCLOSURE_INDEX}.json`: KAP bildirim detay ve ek listesi cache'i
- `data/equities/_kap/attachments_manifest.jsonl`: indirilen KAP eklerinin path, byte, sha256 ve kaynak kayıtları
- `data/equities/_kap/attachments_failures.jsonl`: indirilemeyen KAP ekleri; sonraki çalıştırmalarda tekrar denenebilir
- `data/equities/_kap/document_registry.json`: Faz 1 belge arşiv kaydı; document_id, KAP metadata, local path, checksum, version ve latest flag içerir
- `data/equities/_kap/extraction_jobs.json`: belge arşivleme job geçmişi
- `data/equities/_kap/extraction_errors.jsonl`: belge arşivleme hataları ve review queue adayları
- `data/equities/{TICKER}/kap/extraction/extraction_result.json`: KAP metin blokları, aday finansal fact'ler, kişi/olay/varlık adayları ve human review queue
- `data/seed/kap_companies.json`: KAP şirket seed verisi
- `data/seed/kap_sectors.json`: KAP `/tr/Sektorler` resmi sektör ağacı ve sembol eşleşmeleri
- `data/seed/investing_track_ids.json`: Investing yorum takip ID listesi
- `data/seed/mkk_name_overrides.json`: MKK isim eşleştirme override listesi
- `data/seed/sector_classifications.json`: KAP/MKK/TradingView/dış kaynaklı sektör, faaliyet alanı ve peer evreni
- `data/seed/tradingview_requests.json`: TradingView scanner request tanımları

Hisseye bağlı çıktıların tamamı `data/equities/{TICKER}/` altında tutulur. `data/seed/` sadece global seed tanımlarını içerir.

## Hisse Klasör Yapısı

```text
data/equities/ASELS/
  equity.json
  kap.json
  mkk.json
  mkk_company_info.json
  ohlcv.json
  tradingview/
    ohlcv.json
  financials/
    bilanco.json
    bilanco_hesaplari.json
  charts/
    5.json
    30.json
    60.json
    180.json
    D.json
    M.json
  analysis/
    2026-06-08/
      analiz.json
      ozet.md
      grafik_1D.png
      grafik_karar_1D.png
      grafik_detay_1D.png
```

`equity.json` içinde ana özet ve ayrı dosyaların referansları bulunur. KAP, MKK, OHLCV, scanner, bilanço, bilanço hesapları, yorumlar ve mum datası aynı hisse klasörü altında ayrı JSON dosyaları olarak saklanır. Henüz çekilmemiş kaynaklar `available:false` durum dosyasıyla işaretlenir; ilgili sync komutu çalışınca gerçek veriyle güncellenir. Eski `financials/raw/` klasörleri legacy cache olabilir; yeni `financials run` akışı bunları üretmez.

## Ortam Değişkenleri

- `HISSEBOT_DATA_DIR`: varsayılan `data`
- `HISSEBOT_COMMAND_TIMEOUT`: varsayılan `30m`
- `HISSEBOT_MKK_COOKIE`: MKK API gerekiyorsa cookie
- `HISSEBOT_KAP_SECTORS_FILE`: varsayılan `data/seed/kap_sectors.json`
- `HISSEBOT_KAP_SECTORS_URL`: varsayılan `https://kap.org.tr/tr/Sektorler`
- `HISSEBOT_SECTOR_CLASSIFICATIONS_FILE`: varsayılan `data/seed/sector_classifications.json`
- `HISSEBOT_ISYATIRIM_COOKIE`: İş Yatırım API gerekiyorsa cookie
- `HISSEBOT_TV_HISTORY_URL`: HTTP chart endpoint'i, UDF `/history` uyumlu URL
- `HISSEBOT_TV_CHART_TRANSPORT`: `auto`, `http` veya `socket`; varsayılan `auto`
- `HISSEBOT_ENDPOINT_URL`: yorum forward endpoint'i, varsayılan `http://127.0.0.1:3001/endpoint`

## Başlangıç

Mevcut seed ve eski bilanço dosyasından lokal JSON üretmek için:

```bash
go run ./cmd/hissebot seed kap
go run ./cmd/hissebot seed sirketler
go run ./cmd/hissebot financials import
go run ./cmd/hissebot financials calculate
```

İş Yatırım bilanço çekimi tek geçişte çalışır: her hisse için yıllar paralel çekilir, veri bellekte birleştirilir ve sadece `financials/bilanco.json` ile `financials/bilanco_hesaplari.json` yazılır. Hisseler batch olarak sırayla işlenir; varsayılan worker sayısı yıl sayısıdır (`19`) ve hisseler arasında `1s` beklenir. Güncel yıl her çalışmada yenilenir; mevcut geçmiş yıl verileri tekrar çekilmez:

```bash
HISSEBOT_COMMAND_TIMEOUT=8h go run ./cmd/hissebot financials run
```

Tüm geçmiş yılları da bilerek baştan okutmak istersen `-force-history` ekleyin:

```bash
HISSEBOT_COMMAND_TIMEOUT=8h go run ./cmd/hissebot financials run -force-history
```

Timeout alan dönemler varsayılan olarak `3` kez yeniden denenir. Rate limit veya bağlantı hataları artarsa `-workers 8` ya da `-workers 4` ile düşürün, hisse arası beklemeyi artırın; inatçı timeoutlarda retry sayısını artırın:

```bash
HISSEBOT_COMMAND_TIMEOUT=8h go run ./cmd/hissebot financials run -workers 8 -ticker-delay 2s -retries 6
```

Teknik analiz, indikatör, formasyon, destek/direnç, risk planı ve grafik raporları aynı binary içindeki `analyze` komutuyla üretilir. Varsayılan veri kaynağı TradingView'dir:

```bash
go run ./cmd/hissebot analyze -symbol ADEL
go run ./cmd/hissebot analyze -symbol ADEL -timeframes 1D,1W,1M
go run ./cmd/hissebot analyze -excel "Şirketler.xlsx" -workers 8
go run ./cmd/hissebot analyze -all -workers 8
```

Analiz çıktıları `output/` altına değil, ilgili hisse klasörüne yazılır: `data/equities/{TICKER}/analysis/{YYYY-MM-DD}/`.

Tek tuş rapor endpoint'i için lokal HTTP servisini başlatın:

```bash
HISSEBOT_COMMAND_TIMEOUT=45m go run ./cmd/hissebot serve reports -addr 127.0.0.1:1453
```

Tarayıcıdan `http://127.0.0.1:1453/` adresindeki form ile tek tuş rapor üretilebilir. Doğrudan API çağrısı:

```bash
curl -X POST http://127.0.0.1:1453/reports \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"BORSK","provider":"tradingview","mode":"production","timeframes":["1D","1W","1M"]}'
```

Üç veri kapısı zorunlu olsun istiyorsanız `require_elite_candidate` gönderin. Bu modda rapor yine üretilir, fakat değer yatırım tezi, kurumsal portföy uygunluğu ve trading edge sinyali aynı anda geçmezse API `status:"rejected"` döner:

```bash
curl -X POST http://127.0.0.1:1453/reports \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"BORSK","provider":"tradingview","mode":"production","timeframes":["1D","1W","1M"],"require_elite_candidate":true}'
```

Tek tuş rapor servisini lokal dışına açacaksanız `HISSEBOT_ENDPOINT_TOKEN` verin ve istekte `Authorization: Bearer <token>` veya `X-Hissebot-Token: <token>` başlığını gönderin.

KAP PDF ingestion pipeline binlerce KAP PDF ekini recursive tarar, SHA256 checkpoint ile resume eder, `pdftotext -layout` ile metin çıkarır ve JSONL üretir:

```bash
go run ./cmd/kap-ingest --input data/equities --output data/processed --workers 4 --limit 10 --llm=false
go run ./cmd/kap-ingest --input data/equities --output data/processed --workers 2 --limit 10 --llm=true
```

Çıktılar append-only JSONL formatındadır: `raw_documents.jsonl`, `processed_files.jsonl`, `extraction_errors.jsonl`; `--llm=true` ile ayrıca `kap_events.jsonl` yazılır. Aynı pipeline lokal operasyon ekranından `http://127.0.0.1:1453/kap-ingest` veya `POST /api/kap-ingest/run` ile çalıştırılabilir:

```bash
curl -X POST http://127.0.0.1:1453/api/kap-ingest/run \
  -H 'Content-Type: application/json' \
  -d '{"input":"data/equities","output":"data/processed","workers":4,"limit":10,"resume":true,"llm":false}'
```

Profesyonel hisse raporu, sembol bazlı `data/processed/{ticker_lower}/raw_documents.jsonl` çıktısı varsa bunu otomatik okur ve `professional.kap_pdf_ingest` alanına ekler. HTML/PDF raporun ilk bölümünde `KAP PDF Raporları` bloğu görünür; Türkçe JSON tarafında aynı veri `profesyonel_analiz.kap_pdf_ingest` içindedir. `quality_score < 0.35` veya `low_text_quality_possible_scanned_pdf` uyarısı olan PDF'ler OCR/review adayıdır; bu kayıtlardan kesin finansal değer çıkarımı yapılmaz.

Fiber v3 JSON API, sektör datasını auth/token gerektirmeden verir:

```bash
go run ./cmd/hissebot serve api -addr 127.0.0.1:1454
curl http://127.0.0.1:1454/api/v1/sectors
curl http://127.0.0.1:1454/api/v1/sectors/ASELS
curl http://127.0.0.1:1454/api/v1/sector-groups
curl http://127.0.0.1:1454/api/v1/sector-classifications/ASELS
```

KAP PDF içerik analizinde LLM kullanılacaksa gerçek provider ayarlarını verin. Bu ayarlar yoksa rapor sahte LLM sonucu üretmez; `llm_analysis.status:"not_configured"` olarak deterministik PDF/KAP metin analiziyle devam eder:

```bash
export HISSEBOT_LLM_PROVIDER=openai
export HISSEBOT_LLM_MODEL=<model-adı>
export HISSEBOT_LLM_API_KEY=<api-key>
# OpenAI-compatible farklı endpoint için:
# export HISSEBOT_LLM_ENDPOINT=https://provider.example/v1/chat/completions
# export HISSEBOT_LLM_DOC_LIMIT=3

go run ./cmd/hissebot analyze -symbol BORSK -timeframes 1D,1W,1M
```

Her analiz JSON'u `institutional_validation` bölümü üretir. Bu bölüm günlük walk-forward/backtest, sinyal başarı istatistiği, sentiment kaynak politikası, peer evreni, açıklanabilirlik ve görsel metin kalitesini `pass/limited/fail` olarak denetler. Tek rapor audit'i:

```bash
go run ./tools/analysis_report_audit -path data/equities/ASELS/analysis/2026-06-13/analysis.json -spot-only=true
```

Kurumsal veri hazırlığı ve genel veri yönetişimi audit'i:

```bash
go run ./cmd/hissebot audit enterprise -mode research
go run ./cmd/hissebot audit enterprise -mode production
```

Test veya offline smoke için mock kaynak kullanılabilir:

```bash
go run ./cmd/hissebot analyze -symbol ADEL -provider mock -timeframes 1D -out .cache/analyze-smoke
```

Eski düz dosya yapısını yeni klasör yapısına taşımak için:

```bash
go run ./cmd/hissebot migrate layout
```

TradingView chart datası varsayılan olarak `5,30,60,180,D,M` periyotlarını çeker. `-bars 0` varsayılandır ve mümkün olan tüm geçmiş mumları parça parça istemek anlamına gelir:

```bash
go run ./cmd/hissebot sync charts
go run ./cmd/hissebot sync charts -ticker ASELS -intervals 5,30,60,180,D,M
go run ./cmd/hissebot sync charts -ticker ASELS -intervals D,60 -bars 1000
```

Sektör/peer evreninde referans kaynak KAP `/tr/Sektorler` sayfasıdır. `sync sectors` varsayılan olarak `data/seed/kap_sectors.json` yoksa KAP'tan çeker, KAP'ta bulunan sembollerde TradingView veya eski override KAP sınıflamasını ezemez. TradingView sadece KAP'ta olmayan semboller için fallback olarak kalır. Bu dosya profesyonel karşılaştırma ve peer medyanı hesaplarında kullanılır:

```bash
go run ./cmd/hissebot sync kap-sectors
go run ./cmd/hissebot sync sectors
go run ./cmd/hissebot sync sectors -refresh-kap
go run ./cmd/hissebot sync sectors -max-peers 50
go run ./cmd/hissebot sync sectors -source-file data/seed/fintables_sectors.csv
go run ./cmd/hissebot sync sectors -tradingview=false
```

HTTP ile mum verisi çekmek için `HISSEBOT_TV_HISTORY_URL` değerini UDF `/history` uyumlu endpoint'e ayarlayın ve `-transport http` kullanın:

```bash
HISSEBOT_TV_HISTORY_URL=https://example.com/history go run ./cmd/hissebot sync charts -ticker ASELS -intervals D,60 -transport http
```

`auto` modunda endpoint tanımlıysa önce HTTP denenir; endpoint yoksa mevcut socket fallback'i kullanılır.

Tüm hisseler için genel veri çekimini tek komutla başlatmak için:

```bash
HISSEBOT_COMMAND_TIMEOUT=8h go run ./cmd/hissebot sync all-data
```

KAP bildirim metadatası ve ek dosyaları büyük hacimlidir. Önce tüm bildirim kategorilerini çekin, sonra ekleri resume destekli indirin:

```bash
HISSEBOT_COMMAND_TIMEOUT=8h go run ./cmd/hissebot sync kap-disclosures -from 2010-01-01 -to 2026-06-13 -chunk-days 90 -member-types IGS -disclosure-types all
HISSEBOT_COMMAND_TIMEOUT=720h go run ./cmd/hissebot sync kap-attachments -repeat -pass-delay 5m -newest-first -delay 2s -error-delay 15s -transient-error-sleep 20m -transient-error-threshold 5 -retries 2 -rate-limit-sleep 20m -min-free-bytes 0
```

`sync kap-attachments` var olan dosyaları tekrar indirmez; yalnızca eksik dosyaları alır. `-repeat` modunda kullanıcı/servis durdurana kadar her pass sonunda `-pass-delay` kadar bekleyip yeniden dener. KAP `429 Request Limit Exceeded` döndürürse `-rate-limit-sleep` kadar bekleyip devam eder; bağlantı/EOF hataları retry sonrası `-error-delay` kadar yavaşlatılır, ardışık geçici hata eşiği aşılırsa `-transient-error-sleep` kadar cooldown uygular. Disk koruması istenirse `-min-free-bytes 0` yerine eşik byte değeri verilebilir.

Paralel KAP eki indirmek için:

```bash
WORKERS=4 ./scripts/sync_kap_attachments_parallel.sh
```

Bu script `data/equities/*/kap_disclosures.json` dosyalarından tüm ticker listesini çıkarır, ticker'ları worker'lara böler, her worker'ı ayrı log dosyasına yazar ve aynı hisseyi iki worker'a vermez. Varsayılan olarak eski tekli LaunchAgent'ı durdurur; kapatmak için `STOP_EXISTING=0` kullanın. Daha uzun ve sürekli çalışma için:

```bash
WORKERS=4 REPEAT=1 DELAY=750ms ./scripts/sync_kap_attachments_parallel.sh
```

İndirilen KAP eklerini çıkarım katmanına geçmeden önce metadata/checksum/version registry'sine alın:

```bash
go run ./cmd/hissebot sync kap-document-archive
go run ./cmd/hissebot sync kap-document-archive -ticker ASELS
```

Bu adım PDF içeriğini parse etmez, OCR çalıştırmaz ve LLM kullanmaz. Sadece Faz 1 belge arşivleme kalite kapısıdır: KAP disclosure metadata, local file path, sha256 checksum, dosya tipi, versiyon ve `is_latest_version` alanlarını üretir. Boş dosyalar belge olarak kabul edilmez; aynı dosya yolu farklı checksum üretirse yeni versiyon açılır. PostgreSQL karşılığı `migrations/002_kap_document_archive_schema.sql`, mimari karar dokümanı `docs/kap_document_analysis_architecture.md` içindedir.

Arşivlenen belgelerden kaynaklı metin/fact/olay/varlık adaylarını üretmek için:

```bash
go run ./cmd/hissebot sync kap-extract -ticker ASELS
go run ./cmd/hissebot sync kap-extract -ticker ASELS -limit 20 -max-chars 200000
```

Bu adım PDF native text, HTML ve XML/XBRL metnini deterministik çıkarır. OCR gereken görüntü belgelerde sahte veri üretmez; `needs_ocr` ve `human_review_queue` üretir. PDF metninden yakalanan finansal değerler, tablo satır/sütun doğrulaması tamamlanmadığı sürece `validation_status:"unknown"` ve `review_required:true` kalır. Çıktı `data/equities/{TICKER}/kap/extraction/extraction_result.json` dosyasına yazılır. PostgreSQL karşılığı `migrations/003_kap_extraction_analysis_schema.sql` içindedir.

Rapor servisinde kaynaklı KAP uçları:

```bash
go run ./cmd/hissebot serve reports
curl "http://127.0.0.1:1453/companies/ASELS/info-card"
curl "http://127.0.0.1:1453/companies/ASELS/financials"
curl "http://127.0.0.1:1453/companies/ASELS/management"
curl "http://127.0.0.1:1453/companies/ASELS/assets"
curl "http://127.0.0.1:1453/companies/ASELS/events"
curl "http://127.0.0.1:1453/companies/ASELS/risks"
curl "http://127.0.0.1:1453/companies/ASELS/analysis/fundamental"
```
