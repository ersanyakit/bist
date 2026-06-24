# BIST Veri Mimarisi

## Kısa Değerlendirme

Mevcut projede `internal/ta` teknik/rapor motoru ve `internal/services` veri toplama işleri zaten çalışıyor. Bu yüzden kırıcı refactor yerine yeni kurumsal veri sözleşmesi şu ek katmanlarla ayrıldı:

- `internal/domain/*`: fiyat, finansal tablo, makro, şirket referansı ve KAP açıklama modelleri.
- `internal/datasources/*`: KAP, BIST, TCMB, TÜİK, şirket yatırımcı ilişkileri ve lisanslı market data adapter sözleşmeleri. KAP açıklama listesi, TCMB gösterge kurları ve TCMB 1 hafta repo tablosu anahtarsız resmi endpointlerden okunabilir.
- `internal/repositories/*`: kalıcı kayıt interface'leri ve memory test implementasyonu.
- `internal/normalization`: sembol/fiyat normalizasyonu ve kurumsal olay düzeltme katmanı.
- `internal/validation`: OHLCV, finansal tablo ve makro seri kalite kapıları.
- `internal/ingestion`: provider -> normalization -> validation -> repository pipeline.
- `migrations/001_market_financial_macro_schema.sql`: PostgreSQL/time-series uyumlu şema.

## Kaynak Sınıflaması

Ücretsiz/resmi kaynaklarla yapılabilir:

- KAP: finansal tablolar, özel durum açıklamaları, faaliyet ve denetim raporları.
- TCMB: gösterge döviz kurları (`/kurlar/*.xml`) ve 1 hafta repo politika faizi tablosu anahtarsız resmi kaynaktan okunur. EVDS kapsamındaki ek makro/finansal seriler ayrıca adapter olarak genişletilebilir.
- TÜİK/CİP: GSYH, kişi başı GSYH, enflasyon, sanayi üretimi, işsizlik, güven endeksleri.
- Borsa İstanbul referans verileri: şirket/endeks/pazar/kısmen kurumsal olay referansı.
- Şirket yatırımcı ilişkileri: faaliyet raporu, yatırımcı sunumu, segment ve strateji notları.

Lisanslı/profesyonel veri gerekir:

- Canlı BIST fiyatı, intraday/tick veri, derinlik, bid/ask, order book, broker dağılımı, takas, yabancı yatırımcı akışı, redistribution.
- Kurumsal terminal/API düzeyi Bloomberg, Refinitiv, FactSet, S&P Capital IQ, Matriks, Foreks, Finnet, Fintables veri setleri.

## Veri Kalite Kuralları

- OHLCV: negatif fiyat/hacim yok; `high >= open/close/low`; `low <= open/close/high`; duplicate `symbol + timeframe + timestamp` yok.
- Adjusted price: split/bedelsiz/temettü sonrası geriye dönük düzeltme versiyonlanır.
- Finansal tablolar: `assets = liabilities + equity`; negatif özkaynak kritik; pozitif kâr + negatif CFO uyarı.
- Makro: seri id boş olamaz; gözlem yoksa fail; frekansa göre büyük tarih boşluğu limited.
- Her veri noktasında `source`, `source_url`, `as_of`, `data_version`, `ingested_at` saklanır.

## Test Stratejisi

- Unit: OHLCV validation, finansal tablo eşitliği, makro boşluk, corporate action adjustment.
- Integration: mock provider ile OHLCV, finansal tablo ve GSYH ingestion pipeline.
- Golden: teknik indikatör, oran ve kurumsal olay düzeltme beklenen değerleri.
- Edge case: negatif özkaynak, sıfır ciro, banka/sigorta/GYO/holding, eksik nakit akışı, split, rights issue, yüksek enflasyon düzeltmesi.

## Çalıştırma

TÜİK CİP GSYH verisini güncelle:

```bash
go run ./cmd/hissebot sync tuik-gdp -years 10
```

Mock ve mimari testleri çalıştır:

```bash
go test ./internal/domain/... ./internal/datasources/... ./internal/repositories/... ./internal/normalization/... ./internal/validation/... ./internal/ingestion/... ./internal/analysis/...
```
