# QuantLib / Fincept Araştırma ve Entegrasyon Planı

Bu belge nihai akademik inceleme raporu değildir. Mevcut `hissebot` kod tabanına göre uyarlanmış araştırma, doğrulama ve entegrasyon planıdır. Amaç, hisse analiz motoruna yeni model eklerken üç şeyi zorunlu hale getirmektir:

- Model iddiası kanıtlı olsun: ürün dokümanı, orijinal makale, resmi/bağımsız uygulama ve test fixture ayrıştırılsın.
- Sonuç yeniden üretilebilir olsun: aynı veri, model versiyonu ve as-of tarihiyle aynı rapor üretilebilsin.
- Mimari bakım yapılabilir kalsın: native Go çekirdek, sidecar adapter ve doğrulama katmanı birbirine karışmasın.

## Mevcut Projeye Göre Ana Karar

Bu repo zaten BIST odaklı veri toplama, KAP/PDF işleme, TCMB/EVDS, teknik analiz, quant risk, professional rapor ve decision gate üretiyor. Bu nedenle Fincept veya QuantLib kapsamını doğrudan kopyalamayacağız.

Uygun strateji:

- Native Go çekirdek: hisse/fundamental/statistik/risk/portföy modelleri.
- QuantLib sidecar: opsiyonel, ağır türev, eğri, Heston/SABR/Dupire ve fixed-income işleri.
- Fincept adapter: üretim motoru değil; endpoint envanteri, benchmark ve gap-analysis katmanı.
- Akademik validator: model kabul testlerini ve referans fixture'ları yöneten bağımsız katman.

## Mevcut Paket Eşleşmesi

| Alan | Mevcut paketler | Durum | Plan |
|---|---|---|---|
| Veri toplama | `internal/services/*`, `internal/datasources/*`, `internal/ingestion` | KAP, TCMB, TÜİK, BIST/TradingView akışı var | Resmi/açık veri kaynaklarını validator fixture kaynağı olarak sınıflandır |
| KAP belge zekası | `internal/kapingest`, `internal/kapfinance`, `internal/domain/kapextract` | PDF/metin/fact extraction var | Finansal kalite modelleri için canonical satır kapsamını genişlet |
| Quant çekirdek | `internal/quant/*` | Core, portfolio, rates, fixedincome, options, volatility var | Akademik doğrulama, tolerans ve fixture disiplini ekle |
| Analiz motoru | `internal/ta/analysis` | `advanced_analysis`, quant/stat-economic/decision gates var | Yeni modelleri önce artifact/gate kontratına bağla |
| Professional rapor | `internal/ta/professional`, `internal/ta/storage` | Kurumsal rapor ve yatırımcı QA var | Yeni model çıktıları raporda `computed/missing/confidence` ile görünmeli |
| Validasyon | `internal/ta/validation`, `internal/validation`, `internal/confidence` | Walk-forward, calibration, leakage guard, confidence var | Reference validator ve sertifikalı istatistik testleri ekle |
| ML/forecast | `internal/ta/ml`, `internal/ta/ensemble` | Baseline/champion-challenger temeli var | ML'den önce deterministik model validation gate'i güçlendir |

## Faz 0 - Araştırma Envanteri ve İddia Defteri

Amaç: Fincept/QuantLib/model iddialarını kod yazmadan önce kayıt altına almak.

Yapılacaklar:

- `docs/research_inventory/` altında kaynak envanteri formatı tanımla.
- Fincept endpoint iddialarını ürün dokümanı olarak işaretle; akademik otorite kabul etme.
- Her iddia için `claim`, `source_url`, `model_family`, `tier/credit_conflict`, `verification_status` alanları tut.
- Çelişkili endpoint metadata'sını ayrı `inconsistencies.md` dosyasına yaz.
- Model ailelerini mevcut roadmap fazlarına bağla: fundamental, factor, volatility, valuation, portfolio, sidecar.

Kabul kriterleri:

- Yeni model issue'su kaynak iddiası olmadan başlamaz.
- Fincept iddiası en az bir akademik/orijinal kaynak ve bir uygulama referansı ile eşleşmeden `accepted` olmaz.

## Faz 1 - Reference Validator Omurgası

Amaç: Model eklemeden önce sonuçları kıyaslayacak test altyapısını kurmak.

Mevcut dayanak:

- `internal/quant/core`
- `internal/ta/validation`
- `internal/confidence`
- `internal/repositories`

Yapılacaklar:

- `internal/quant/validation` veya `internal/reference` altında genel validator sözleşmesi oluştur.
- Fixture formatı: input JSON, expected JSON, tolerance spec, source metadata.
- Mutlak hata, göreli hata, bps hata, RMSE, max abs error metriklerini standartlaştır.
- NIST StRD gibi sertifikalı istatistik fixture'ları için adapter formatı hazırla.
- QuantLib test sonuçları sidecar gelmeden önce dosya fixture olarak kabul edilebilsin.

Kabul kriterleri:

- Her yeni quant/fundamental modelinde `reference_fixture` testi olur.
- Toleranslar test içinde gömülü değil, fixture metadata'sında görünür.
- `go test ./internal/quant/... ./internal/ta/validation/...` yeşil kalır.

## Faz 2 - Veri Güvenliği ve Kaynak Hiyerarşisi

Amaç: Model doğruluğundan önce verinin karar için kullanılabilir olduğunu kanıtlamak.

Mevcut dayanak:

- `docs/bist_data_architecture.md`
- `internal/services/kap`, `internal/services/tcmb`, `internal/services/tuik`
- `internal/services/pricequality`
- `internal/ta/analysis` price-quality gate
- `internal/domain/pricequality`

Yapılacaklar:

- KAP, TCMB EVDS, TÜİK, BIST resmi bülten ve TradingView cache'lerini güvenilirlik sınıflarına ayır.
- Her model input'u için `source`, `source_timestamp`, `available_at`, `fetched_at`, `data_version` zorunlu olsun.
- Finansal tablo modellerinde publish-date ve point-in-time güvenliği yoksa backtest-safe sinyal üretme.
- BIST resmi kapanış yoksa production trade gate açılmasın.
- Lisanslı veri gerektiren alanları açıkça `licensed_required` olarak işaretle.

Kabul kriterleri:

- `analysis.json` ve rapor artifact'ları eksik veri etkisini açık gösterir.
- Veri kaynağı yokken model sıfır değerle başarılı görünmez.

## Faz 3 - Native Go Hisse/Fundamental Çekirdek

Amaç: Projenin asıl kullanım alanı olan BIST hisse analizinde vendor bağımsız çekirdeği güçlendirmek.

Mevcut dayanak:

- `internal/ta/analysis/advanced_analysis.go`
- `internal/ta/professional/investment_research.go`
- `internal/kapfinance`
- `internal/analysis/{fundamental,risk,technical,valuation}`

Öncelikli modeller:

- Piotroski F-Score tamamlama.
- Beneish M-Score tam canonical input kapsamı.
- Altman Z-Score sektör uyarlamaları: sanayi, finans dışı, banka dışı özel durumlar.
- Ohlson O-Score araştırma/prototip.
- Merton structural credit model araştırma/prototip.
- DuPont, accrual quality, earnings persistence, cash conversion cycle.

Yapılacaklar:

- Canonical KAP finansal satır eksiklerini `kapfinance` sözlüğüne ekle.
- Her skor için `computed`, `missing_inputs`, `sector_applicability`, `confidence` alanı üret.
- Banka, sigorta, GYO, holding ve sanayi şirketlerini aynı formülle zorlamayı engelle.
- Model formülünü rapor metnine değil, testlenebilir pure Go fonksiyonlara koy.

Kabul kriterleri:

- Sektör uyumsuz model `not_applicable` döner.
- Eksik input hedef fiyat veya AL/SAT sinyali uydurmaz.
- `go test ./internal/kapfinance ./internal/ta/analysis ./internal/ta/professional` yeşil.

## Faz 4 - Faktör, Regresyon ve İstatistik Katmanı

Amaç: Hisse performansını piyasa, sektör ve stil faktörlerine ayırmak.

Mevcut dayanak:

- `internal/quant/core`
- `internal/ta/analysis/stat_economic_integration.go`
- `internal/ta/validation`

Yapılacaklar:

- BIST CAPM: beta, alpha, residual volatility.
- Sektör-relative model: XU100 + sektör endeksi ayrışımı.
- Style factors: size, value, momentum, quality, low-volatility, liquidity.
- Newey-West HAC standard error.
- Rolling beta/alpha stability.
- NIST StRD ile lineer/nonlinear regression doğrulaması.
- Kenneth French global faktörleri yalnızca global benchmark/fixture için kullan; BIST faktörleri yerel veriden üret.

Kabul kriterleri:

- Faktör datası eksikse `partial` döner.
- Beta/alpha confidence ve sample window raporlanır.
- Regression fixture'ları deterministik toleransla geçer.

## Faz 5 - Zaman Serisi, Volatilite ve Kuyruk Riski

Amaç: Sabit volatilite varsayımıyla karar üretmeyi bırakıp rejim ve kuyruk riskini ölçmek.

Mevcut dayanak:

- `internal/quant/volatility`
- `internal/quant/portfolio`
- `internal/ta/analysis` quant/stat-economic raporları
- `internal/ta/validation`

Yapılacaklar:

- EWMA volatility modelini production baseline yap.
- GARCH/EGARCH/GJR-GARCH araştırmasını native Go prototype olarak başlat.
- Cornish-Fisher VaR, historical VaR/CVaR, EVT/POT kuyruk riski.
- Kupiec VaR backtest ve traffic-light tipi risk raporu.
- Bootstrap stress simulation ve limit-up/limit-down gap riski.

Kabul kriterleri:

- Yüksek volatilite rejimi pozisyon/risk bütçesini otomatik sınırlar.
- VaR/CVaR sadece sayı değil, backtest sonucu ve sample window ile gelir.
- Model kalibrasyonu başarısızsa fallback baseline ve uyarı görünür.

## Faz 6 - Değerleme, Makro ve Eğri Modelleri

Amaç: Değerleme ensemble'ını akademik ve veri güvenliği kontrollü hale getirmek.

Mevcut dayanak:

- `advanced_analysis.valuation_ensemble`
- `internal/ta/professional`
- `internal/services/tcmb`
- `internal/quant/rates`, `internal/quant/fixedincome`

Yapılacaklar:

- DCF/WACC/CAPM input zincirini açık kaynaklara bağla: risk-free, ERP, beta, borç maliyeti.
- TCMB EVDS makro serilerini valuation assumptions ve macro regime ile eşleştir.
- Nelson-Siegel/Svensson curve fitting araştırma/prototipini `internal/quant/rates` altında planla.
- Monte Carlo valuation ve sensitivity grid ekle: WACC, terminal growth, margin, FX, inflation.
- GYO için NAV/portfolio appraisal reconciliation'ı güçlendir.

Kabul kriterleri:

- Fair value tek nokta değil bear/base/bull aralığıdır.
- WACC ve terminal growth input kaynağı raporda görünür.
- Eksik risk-free/ERP/beta varsa valuation confidence sınırlanır.

## Faz 7 - QuantLib Sidecar ve Fincept Adapter

Amaç: Native Go çekirdeği bozmadan ağır finans modellerini izole adapter olarak bağlamak.

Sidecar kapsamı:

- Heston/SABR/Dupire/local volatility.
- Exotic options ve Greeks.
- Yield curve bootstrap, multi-curve, fixed-income calibration.
- QuantLib test suite fixture karşılaştırmaları.

Fincept adapter kapsamı:

- Endpoint envanteri.
- Benchmark/oracle karşılaştırması.
- Rate-limit, tier, credit ve metadata tutarsızlık kaydı.
- Üretim karar motoruna doğrudan authority olarak bağlanmama.

Mimari öneri:

- `internal/adapters/quantlib`: sidecar client.
- `internal/adapters/fincept`: REST client ve claim inventory mapper.
- `internal/valuation`: `ValuationEngine` portları.
- Sidecar process başına izole valuation context; global evaluation-date state paylaşımı yok.

Kabul kriterleri:

- QuantLib sidecar kapalıyken native Go analiz çalışmaya devam eder.
- Sidecar sonucu sadece reference/advanced model olarak girer; veri ve validation gate'i atlayamaz.
- Fincept sonucu kaynak metadata'sı olmadan rapora yazılmaz.

## Faz 8 - Model Monitoring, CI ve Release Gate

Amaç: Model doğru varsayılmasın; sürekli ölçülsün.

Mevcut dayanak:

- `internal/ta/validation`
- `internal/ta/ml`
- `internal/ta/ensemble`
- `.github/workflows/test.yml`
- `.golangci.yml`

Yapılacaklar:

- Model registry: model adı, versiyon, parametre hash, fixture version.
- Champion/challenger kararları için deterministic comparison.
- Diebold-Mariano forecast comparison araştırma/prototip.
- Brier score, calibration curve, prediction interval coverage.
- Data drift ve feature drift raporu.
- `go test`, `go test -race`, `golangci-lint`, golden fixtures ve benchmark job'ları.

Kabul kriterleri:

- Yeni model PR'ı fixture ve validation raporu olmadan merge edilmez.
- Model output değişirse snapshot/golden farkı bilinçli onay ister.
- Production mode research mode'dan daha katı gate kullanır.

## Öncelik Sırası

Bu proje için önerilen uygulama sırası:

1. Faz 0: Araştırma envanteri ve iddia defteri.
2. Faz 1: Reference validator omurgası.
3. Faz 2: Veri güvenliği ve kaynak hiyerarşisi.
4. Faz 3: Native Go hisse/fundamental çekirdek.
5. Faz 8: Monitoring ve CI release gate.
6. Faz 4: Faktör/regresyon katmanı.
7. Faz 5: Volatilite/kuyruk riski.
8. Faz 6: Değerleme/makro/eğri modelleri.
9. Faz 7: QuantLib sidecar ve Fincept adapter.

Bu sıra bilerek vendor entegrasyonunu sona koyar. Önce veri ve native Go çekirdek güvenli olmalıdır; aksi halde sidecar veya Fincept sadece daha hızlı yanlış sonuç üretir.

## Definition of Done

Bir model veya faz tamamlandı sayılmaz; aşağıdaki koşulların tamamı geçmelidir:

- Kaynak: Orijinal/kanonik kaynak ve uygulama referansı kayıtlı.
- Kod: Pure Go core veya adapter portu testlenebilir.
- Veri: Input kaynakları `available/missing/partial` olarak raporlanıyor.
- Test: Fixture, tolerans ve edge-case testleri var.
- Rapor: `analysis.json`, Türkçe rapor ve ilgili artifact yeni alanı açık gösteriyor.
- Karar: Decision gate yeni modelin güvenini ve eksiklerini dikkate alıyor.

## Açık Riskler

- QuantLib'in resmi Go wrapper'ı olmadığı için sidecar/cgo kararı erken prototip ister.
- Fincept dokümantasyonundaki tier/credit çelişkileri otomatik client üretimini riskli kılar.
- BIST order book, intraday ve bazı benchmark verileri lisanslı olabilir.
- KAP PDF canonical satır kapsamı eksikse finansal kalite modelleri proxy kalır.
- Çok fazla model eklemek karar kalitesini artırmaz; validation gate olmayan model raporda authority olamaz.
