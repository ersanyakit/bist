# KAP Belge Analiz ve Veri Çıkarım Mimarisi

Bu doküman KAP belge analiz sisteminin veri güvenilirliği sınırlarını ve fazlara ayrılmış uygulama mimarisini tanımlar. Sistem yatırım tavsiyesi üretmez; KAP kaynaklı belgeleri arşivler, kaynak izlenebilirliği kurar, sonraki fazlarda deterministik çıkarım ve insan denetimiyle temel analiz verisi üretir.

## Değişmez Kurallar

- Kaynağı olmayan veri kesin bilgi olarak saklanmaz.
- LLM nihai karar verici değildir; sadece yardımcı çıkarım katmanıdır.
- OCR çıktısı doğrudan finansal gerçek olarak kaydedilmez.
- Finansal veri ancak `source_document_id`, mümkünse sayfa, tablo, satır, sütun, doğrulama sonucu ve confidence skoru ile kabul edilir.
- Konsolide ve solo tablolar ayrı tutulur.
- Düzeltme bildirimleri eski kaydı silmez; yeni sürüm olarak saklanır ve çelişki raporlanır.
- Belirsiz eşleşmeler kesin bilgiye yükseltilmez; `review_required=true` ile insan denetimine düşer.

## Faz 1: Belge Arşivleme

Faz 1 sadece KAP belge arşivleme katmanıdır. PDF okuma, OCR, tablo çıkarımı, LLM analizi ve finansal oran üretimi bu fazda yapılmaz.

Uygulanan akış:

1. `sync kap-disclosures` KAP bildirim metadata dosyalarını hisse bazında üretir.
2. `sync kap-attachments` KAP eklerini resume destekli indirir.
3. `sync kap-document-archive` indirilen ekleri belge registry katmanına alır.
4. Her belge için SHA256 checksum, dosya tipi, KAP bildirim ilişkisi, local path, kaynak URL, sürüm ve latest flag üretilir.
5. Boş veya okunamayan dosyalar belge olarak kabul edilmez; ingestion error kaydına düşer.
6. Aynı belge yolu değişirse yeni checksum ile yeni version kaydı açılır; önceki kayıt korunur.

Kalıcı çıktılar:

- `data/equities/_kap/document_registry.json`
- `data/equities/_kap/extraction_jobs.json`
- `data/equities/_kap/extraction_errors.jsonl`

PostgreSQL karşılığı:

- `migrations/002_kap_document_archive_schema.sql`

## PDF Arşivi Sonrası KAP Servis TODO

Tüm KAP PDF/ek indirme işi tamamlandıktan sonra KAP veri kapsamı aşağıdaki sırayla genişletilecek. Her madde resume destekli sync komutu, raw JSON arşivi, manifest, failure log, test ve veri sözleşmesiyle uygulanmalıdır.

- [ ] `lastDisclosureIndex`: Yayınlanmış son bildirim ID servisi eklenecek; date range taramaya ek olarak ID bazlı incremental kontrol kurulacak.
- [ ] `disclosureDetail`: Her bildirim için tam detay servisi ayrı arşiv katmanı olarak eklenecek; sadece attachment indirme sırasında cache'lenen detayla sınırlı kalmayacak.
- [ ] `disclosures`: Bildirim listesi servisi tüm kategori/member type kombinasyonları için doğrulanacak; eksik kategori, tarih aralığı ve mükerrer bildirim raporu üretilecek.
- [ ] `downloadAttachment`: Mevcut ek indirme tamamlandıktan sonra failure replay, checksum doğrulama ve eksik obje raporu ayrı kalite kapısı olarak çalıştırılacak.
- [ ] `members`: Şirket listesi servisi mevcut `sync kap` akışından ayrıştırılıp raw/company snapshot manifest'i ile kalıcı hale getirilecek.
- [ ] `memberDetail`: Şirket detay servisi eklenecek; her KAP üyesi için tam detay JSON'u saklanacak.
- [ ] `memberSecurities`: Şirket kıymet bilgileri servisi eklenecek; pay, pazar, işlem durumu ve kıymet eşleştirmeleri ayrı referans dosyası olacak.
- [ ] `funds`: Fon listesi servisi eklenecek; fon evreni şirket evreninden ayrı tutulacak.
- [ ] `fundDetail`: Fon detay servisi eklenecek; her fon için tam detay JSON'u saklanacak.
- [ ] `blockedDisclosures`: Erişime kapatılmış bildirimler servisi eklenecek; arşivdeki bildirimlerle çakıştırılıp erişim durumu raporlanacak.
- [ ] `caEventStatus`: Hak kullanım süreç durum servisi eklenecek; temettü, bedelli/bedelsiz, birleşme/bölünme gibi kurumsal olay durumları ayrı zaman serisi olarak tutulacak.
- [ ] KAP servis coverage raporu: Her servis için toplam kayıt, başarılı kayıt, hata, son başarılı sync zamanı, eksik ticker/fon ve tekrar deneme kuyruğu üretilecek.

## Faz 2: Belge Çıkarım Katmanı

Bu fazda PDF/HTML/XML/XBRL işlenir. Öncelik sırası:

1. XBRL veya yapılandırılmış XML
2. HTML tablo ve metin blokları
3. Native PDF text
4. PDF tablo çıkarımı
5. OCR fallback
6. LLM destekli sınıflama veya özet, yalnızca kaynaklı bloklar üzerinde

LLM çıktısı tek başına finansal veri olamaz. LLM sadece şu alanlarda yardımcı olabilir:

- Belge bölümünü sınıflandırma
- Metin bloğunu olay adayı olarak etiketleme
- Varlık, kişi veya dipnot adayı üretme
- Deterministik parser sonuçları arasında çelişki varsa review notu oluşturma

Uygulanan komut:

```bash
go run ./cmd/hissebot sync kap-extract -ticker ASELS
```

Native PDF text tabanlı toplu MVP ingestion komutu:

```bash
go run ./cmd/kap-ingest --input data/equities --output data/processed --workers 4 --limit 100 --llm=false
```

Bu komut registry zorunluluğu olmadan `data/equities` altındaki PDF dosyalarını tarar, SHA256 checkpoint uygular, `pdftotext -layout` ile metin çıkarır, belge tipini keyword tabanlı tahmin eder ve append-only JSONL yazar. OCR ve gelişmiş tablo çıkarımı bu MVP'de yapılmaz; düşük kalite metinler `low_text_quality_possible_scanned_pdf` uyarısıyla review/OCR kuyruğuna uygun işaretlenir.

Çıktı:

- `data/equities/{TICKER}/kap/extraction/extraction_result.json`
- `data/processed/raw_documents.jsonl`
- `data/processed/processed_files.jsonl`
- `data/processed/extraction_errors.jsonl`
- `data/processed/kap_events.jsonl` (`--llm=true`)

Profesyonel analiz mimarisi bu JSONL katmanını rapora dahil eder. Sembol bazlı çıktı varsa öncelik `data/processed/{ticker_lower}/raw_documents.jsonl`; yoksa toplu `data/processed/raw_documents.jsonl` ticker/path filtresiyle okunur. Bu entegrasyon `professional.Report.KAPPDFIngest` alanını üretir ve aynı içerik:

- analiz JSON'unda `professional.kap_pdf_ingest`,
- Türkçe rapor JSON'unda `profesyonel_analiz.kap_pdf_ingest`,
- HTML/PDF raporun ilk sayfasında `KAP PDF Raporları`

olarak görünür. Bu katman belge tipleri, kalite skoru, düşük kalite/OCR adayı sayısı, ingest hata sayısı ve öne çıkan PDF snippet'lerini verir. `quality_score < 0.35` olan metinler finansal değer doğrulaması için kesin kanıt değil, OCR/review adayı kabul edilir.

Bu çıktı kaynaklı metin blokları, heuristik tablo adayları, finansal fact adayları, kişi adayları, olay adayları, duran varlık adayları, evidence chain ve human review queue içerir. PDF metninden çıkarılan finansal fact'ler tablo satır/sütun doğrulaması tamamlanmadan `valid` yapılmaz.

## Faz 3 ve Sonrası

Sonraki fazlarda finansal tablo normalizasyonu, validation engine, şirket info card, kişi rolleri, varlık takibi, evidence chain ve temel analiz raporu ayrı modüller halinde eklenir. Bu modüller Faz 1 registry kayıtlarını tek gerçek belge kaynağı olarak kullanır.

Faz 3-6 için eklenen kalıcı şema `migrations/003_kap_extraction_analysis_schema.sql` içindedir. Bu migration şirket kartı, financial facts, kişi rolleri, ortaklıklar, iştirakler, kurumsal olaylar, duran varlıklar, evidence chain, validation result, confidence score ve human review queue tablolarını tanımlar.

Rapor servisinin okuduğu temel endpointler:

- `GET /companies/{ticker}/info-card`
- `GET /companies/{ticker}/financials`
- `GET /companies/{ticker}/financials/{period}`
- `GET /companies/{ticker}/management`
- `GET /companies/{ticker}/assets`
- `GET /companies/{ticker}/assets/{asset_id}/evidence-chain`
- `GET /companies/{ticker}/events`
- `GET /companies/{ticker}/risks`
- `GET /companies/{ticker}/analysis/fundamental`
- `GET /documents/{document_id}`
- `GET /documents/{document_id}/sources`
- `POST /extraction/jobs`

## Production Kalite Kapıları

- Belge checksum olmadan işlenmez.
- `document_type=OCR_IMAGE` olan kayıtlar doğrudan finansal veriye dönüşmez.
- `review_required=true` olan kayıtlar insan onayı olmadan kesin analiz verisi üretmez.
- Validation bozuksa finansal fact `valid` olamaz.
- Kaynak sayfası veya tablo konumu yoksa confidence skoru otomatik düşer.
- Aynı varlık geçmiş yıllarda geçse bile bugünkü durumu doğrudan belge kanıtı yoksa `unknown` veya `likely_active` kalır.
