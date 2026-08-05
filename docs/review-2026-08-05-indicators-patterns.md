# hissebot — İndikatör & Formasyon Doğruluk Denetimi / Yol Haritası

**Tarih:** 2026-08-05
**Kapsam:** `internal/ta/indicators`, `internal/ta/patterns` (+ `generated/`), `internal/ta/formations`
**Yöntem:** 4 paralel derin kod denetimi (formül/algoritma doğruluğu, standart tanımlara karşı satır satır karşılaştırma), sonuçların birleştirilip önceliklendirilmesi.
**İlgili:** `docs/review-2026-06-25.md` (genel mimari/kod kalitesi denetimi — bu doküman onun matematiksel/algoritmik doğruluk odaklı devamı).

**Durum (2026-08-06 itibarıyla): Tüm P0 (5/5), P1 (15/15) ve P2 (8/8) maddeleri düzeltildi ve her biri için pinned-value regresyon testi eklendi.** Ayrıca denetim sırasında bulunan ek bir sorun (candle kronolojik sıra doğrulamasının 1D/1W/1M dışındaki zaman dilimlerinde tamamen atlanması) da düzeltildi. Detaylar için `git log` / ilgili dosyalardaki testlere bakın. P2'deki "iki paralel divergence implementasyonu" maddesi bilinçli olarak değiştirilmedi: `internal/ta/patterns/scanner_matchers.go`'daki `matchOscillatorDivergence` ayrı incelendi, gerçek pivot tabanlı ve makul şekilde sınırlı (80 barlık pencere) bulundu — aktif bir hata değil, sadece `internal/ta/indicators/divergence.go` ile kod tekrarı; birleştirme paket sınırlarını değiştiren daha büyük bir mimari iş olduğundan kapsam dışı bırakıldı.

---

## Özet

İndikatör katmanının **çekirdeği** (RSI, ATR, ADX, MACD, Bollinger, Stochastic/StochasticRSI, CCI, MFI, Williams %R, ROC, OBV, VWAP, Supertrend, Keltner, Pivot Points) Wilder/standart tanımlara doğru şekilde uyuyor — Wilder'ın 1/period yumuşatması ile EMA'nın 2/(n+1) yumuşatması doğru ayrıştırılmış, CCI doğru şekilde ortalama mutlak sapma kullanıyor (stddev değil), Pivot Points ve MarketStructure look-ahead'den doğru şekilde kaçınıyor. Bu sağlam bir temel.

Ancak **4 alanda sistemik, somut doğruluk hataları** var:
1. **Ichimoku bulut sinyalleri** kavramsal olarak yanlış (26 barlık ileri kaydırma uygulanmamış) — trend piyasalarda sinyal yönü tersine dönebiliyor.
2. **Formasyon motorunda (wedge/triangle) geometri hatası** — iki trend çizgisinin farklı zaman noktalarındaki fiyatları çıkarılıyor, bu da wedge kabul/red kararını ve fiyat hedeflerini güvenilmez kılıyor.
3. **Üretilmiş (generated) mum formasyonu kataloğunda yön etiketleme tutarsızlığı** — bazı formasyonlar `neutral` olarak etiketlenmiş ama aynı isim için elle yazılmış dedektör doğru yönü (bullish/bearish) veriyor; hangi path kazanırsa ona göre sinyal kayboluyor.
4. **Test paketi büyük ölçüde totolojik** — "referans" testler üretim kodunun aynı tasarım kararlarını birebir kopyalıyor, yani harici/bağımsız bir doğrulama yok. Yukarıdaki hataların hiçbiri mevcut testlerle yakalanmıyor.

Aşağıdaki liste **P0 (kritik, geniş etki alanı)** → **P1 (orta, somut ama sınırlı etki)** → **P2 (küçük/test altyapısı)** şeklinde önceliklendirilmiştir. Her madde dosya:satır, kanıt ve önerilen düzeltme içerir.

---

## P0 — Kritik (önce bunlar)

### P0-1. Ichimoku Senkou A/B'ye standart 26 barlık ileri kaydırma (forward displacement) uygulanmamış
**Dosya:** `internal/ta/indicators/indicators.go:754-804` (`Ichimoku`, `ichimokuCloudTrend`)

Senkou Span A/B, kijun periyodu kadar (26 bar) **ileri kaydırılarak** çizilir; bugünkü kapanış bugünkü bulutla değil, ~26-52 bar önce hesaplanmış bulutla karşılaştırılmalıdır. Kod, Chikou için bu kaydırmayı doğru uyguluyor (`indicators.go:770-774`) ama Senkou A/B için uygulamıyor — bu da `IchimokuCloudTrend`, `IchimokuKumoTwist`, `IchimokuTKCross`, `IchimokuPriceCloudBreakout` çıktılarının güçlü trendlerde herhangi bir standart grafik platformuna göre **ters** işaret verebilmesine yol açıyor. Bu değerler her `Snapshot()` çağrısında üretiliyor (line 75-83), yani tüm downstream skorlamayı etkiliyor.

**Düzeltme:** `senkouA`/`senkouB` dizilerini üret, kaydırılmış (offset) versiyonlarını `Snapshot` çıktısına koy; `priceCloudBreakout`/`cloudTrend` karşılaştırmasını kaydırılmış bulut ile yap.

### P0-2. RelativeVigorIndex: oran önce alınıp sonra ortalanıyor (standart: önce ortalama, sonra oran)
**Dosya:** `internal/ta/indicators/indicators.go:1491-1522`

Standart RVI = `SMA(pay, N) / SMA(payda, N)`. Kod bunun yerine her bar için `pay[i]/payda[i]` oranını alıp bu oranların SMA'sını dönüyor. Pencere içinde tek bir dar-range bar (BIST'te tatil öncesi düşük hacimli günlerde sık) sonucu ~20 kat saptırabiliyor. Bu bir kenar durum değil, rutin fiyat verisinde her zaman oluşan bir sapma.

**Düzeltme:** Pay ve payda serilerini ayrı ayrı `SMA`/`EMA` ile yumuşat, en son adımda böl.

### P0-3. Formasyon motoru: wedge genişliği hizasız (farklı bar indeksli) fiyatlardan hesaplanıyor
**Dosya:** `internal/ta/formations/engine.go:784-873` (`detectWedge`, `bestWedgeTrendlinePair`)

`upper.line.Start.Price - lower.line.Start.Price` — iki trend çizgisinin `Start` noktaları kendi bağımsız başlangıç barlarına ait, yani aynı zaman kesitinde değiller. Aynı dosyadaki **triangle** dedektörü bunu doğru yapıyor (`lineValue(...)` ile ortak bir indekste değerlendiriyor, `bestTriangleTrendlinePair`) — bu muhtemelen bir kopyala-yapıştır regresyonu. Sonuç: wedge kabul/red kararı ve `convergenceScore` güvenilmez.

**Düzeltme:** Triangle path'indeki `lineValue(candles, maxInt(upper.startIdx, lower.startIdx))` desenini wedge'e de uygula.

### P0-4. Triangle fiyat hedefleri, aynı hizasız hesapla eziliyor (overwrite)
**Dosya:** `internal/ta/formations/engine.go:761-765` (`detectTriangle`)

`patternFromLines` doğru (hizalı) `height` hesaplıyor, ama `detectTriangle` bunu P0-3'teki gibi hizasız bir `height` ile **üzerine yazıyor** ve bu yanlış değer `result.Targets` olarak kullanıcıya (rapor/UI) gidiyor.

**Düzeltme:** `patternFromLines`'ın zaten hesapladığı hizalı `height`'ı kullan; ikinci hesaplamayı sil.

### P0-5. Üretilmiş mum formasyonu kataloğunda yön etiketleme tutarsızlığı (neutral vs. doğru yön)
**Dosya:** `internal/ta/patterns/generated/pattern_three_inside_down.go`, `pattern_three_outside_down.go`, `pattern_concealing_baby_swallow.go`, `pattern_thrusting_line.go`, `pattern_matching_high.go`

Bu 5 formasyon `generated/` kataloğunda `Direction: "neutral"` olarak işaretli, ama `candlestick.go` içindeki elle yazılmış kardeş implementasyonları doğru yönü (bearish/bullish) veriyor — ör. Three Inside Up `bullish` iken Three Inside Down `neutral` (olması gereken: `bearish`). `uniquePatterns()` dedup mantığı (yön tie-break'i olmadan, sadece confidence'a göre) hangi path kazanırsa, gerçek yönlü bir sinyal `neutral_pattern_context_only` etiketiyle actionable çıktıdan sessizce düşebiliyor.

**Düzeltme:** `tools/pattern_catalog_gen`'deki kaynak veriyi (muhtemelen bir isim→yön eşleme tablosu) düzelt, kataloğu yeniden üret; ardından `candlestick.go` ile `generated/*.go` arasında yön tutarlılığını doğrulayan bir test ekle.

---

## P1 — Orta öncelik

| # | Konu | Dosya:satır | Özet |
|---|---|---|---|
| P1-1 | StochasticMomentumIndex aynı "önce oran, sonra yumuşat" hatası | `indicators.go:1644-1662` | Pay/payda ayrı yumuşatılmalı (P0-2 ile aynı hata deseni) |
| P1-2 | KST (Know Sure Thing) standart olmayan yumuşatma periyotları | `indicators.go:1471-1489` | Pring kanonik 10/10/10/15 yerine 10/13/15/20 kullanılmış |
| P1-3 | FibonacciLevels her zaman en yükseğe (high) sabitleniyor | `indicators.go:829-840` | Swing yönü (düşüş mü çıkış mı) tespit edilmiyor; düşüş trendinde seviyeler yanlış referans noktasına bağlanıyor |
| P1-4 | Donchian kanalı güncel barı kendi içine alıyor | `indicators.go:699-715` | Breakout karşılaştırması kendine referanslı hale geliyor (`MarketStructure`'daki `priorWindowHigh/Low` doğru yapıyor, Donchian yapmıyor) |
| P1-5 | Divergence sinyalleri asla "eskimiyor" (staleness yok) | `indicators/divergence.go:109-140` | 40. barda oluşan bir diverjans, 200 bar sonra hâlâ `Bullish=true` dönebiliyor; `analysis/engine.go` skoruna +0.6 olarak giriyor |
| P1-6 | `computeProxyValue` ve 7 yardımcı fonksiyon ölü kod | `indicators/scanner_matchers.go:1914-1962, 2497-2634` | Hiç çağrılmıyor; hesaplanabilecek indikatörler yanlışlıkla `requires_external_data` olarak işaretleniyor |
| P1-7 | `market_structure`/`smart_money` eşiği yanlış ölçekte olabilir (takip gerekli) | `indicators/scanner_matchers.go:2051-2058` | ±0.65 eşiği normalize edilmemiş ham fiyat seviyelerine uygulanıyor olabilir |
| P1-8 | "In Neck" ve "Thrusting Pattern" birebir aynı mantık | `patterns/candlestick.go:127-134` | İkisi de aynı anda tetikleniyor, penetrasyon derinliği ayrımı yok |
| P1-9 | Island Reversal ortadaki barı doğrulamadan atlıyor | `patterns/chart_patterns.go:698-709` | `len(c)-2` hiç kontrol edilmiyor; gerçek "iki taraflı gap" şartı sağlanmadan formasyon tetiklenebiliyor |
| P1-10 | Kicking formasyonu gerekli tam gap'i zorunlu kılmıyor | `patterns/candlestick.go:147-154` | Sadece `open` karşılaştırması yapılıyor, range'lerin örtüşmemesi kontrol edilmiyor |
| P1-11 | Alias matcher path'i (generated katalog) trend bağlamını atlıyor | `patterns/scanner_matchers.go:548-559` | Piercing/Dark Cloud/Tweezer'ın elle yazılmış versiyonu `uptrend`/`downtrend` şartı arıyor, alias path'i aramıyor |
| P1-12 | `clusterLevels` sınırsız kayma ile eğik pivotları tek düz seviyeye birleştiriyor | `formations/engine.go:262-280` | Kümeleme sabit bir çapaya değil, sürekli kayan ortalamaya göre yapılıyor; eğik bir "daha yüksek diplerin" dizisi tek yatay destek olarak raporlanabiliyor |
| P1-13 | Trendline dokunuş sayısı, hacim onaylı dokunuşlarda çift sayılıyor | `formations/engine.go:507-541` | `touch_count` çizilen dokunuş işaretleriyle tutarsız; `Strength` skorunu şişiriyor |
| P1-14 | Çok aşamalı EMA kademelerinde (DEMA/TEMA/TRIX/MACD sinyal/PVO/Klinger/SMI) her aşama SMA ile yeniden tohumlanıyor | `indicators.go` (çoklu konum) | TradingView/pandas-ta ile sayısal olarak eşleşmiyor; dokümante edilmemiş bir tasarım kararı |
| P1-15 | `buildLevels`, `MaxLevels`'a göre kırpma işlemini destek/direnç yeniden sınıflandırmasından **önce** yapıyor | `formations/engine.go:175-207` | Kırılma sonrası rejimlerde gerçek direnç adayları sessizce elenebiliyor |

---

## P2 — Küçük / test altyapısı

- **Test paketi totolojik:** `indicators/indicator_reference_test.go`'daki "referans" fonksiyonlar üretim kodunun aynı tasarım kararlarını (Wilder smoothing, SMA-seeded EMA, vb.) birebir kopyalıyor — yalnızca iki el kopyası arasındaki yazım hatalarını yakalayabilir, tasarım hatalarını (P0-2, P1-1, P1-2 gibi) yakalayamaz. Tek gerçek istisna: `TestBollingerBandsUsePopulationStdDev`.
- Divergence eşleştirmede asimetrik ±1 tolerans (`divergence.go:115-116, 132-133`) — şu an sinyal üretmiyor ama tutarsız.
- Negatif osilatör değerlerinde (MACD histogram gibi) epsilon toleransının işareti ters dönüyor (`divergence.go:120-121, 137-138`) — şu an `Epsilon=1e-9` olduğu için etkisiz, ama `Epsilon` büyütülürse gerçek bir hataya dönüşür.
- `divergence.go` için hiç birim testi yok.
- `DetectSwings`'te eşit high/low'larda (tick-boyutu küçük hisselerde olağan) her iki taraf da diskalifiye oluyor, pivot kaybediliyor (`formations/engine.go:146-164`).
- `formations/engine.go:1108`'de `SafeDiv` yerine korumasız bölme (dosyanın geri kalanının konvansiyonuna aykırı).
- İki paralel/bağımsız diverjans implementasyonu var: `indicators/divergence.go` (doğru, swing-bazlı) ve `patterns/scanner_matchers.go`'daki ölü `divergenceProxy` (naif iki-nokta karşılaştırması). İkincisi ölü kod ama gelecekte yanlışlıkla bağlanırsa yanlış diverjans sinyali üretir — ya silinmeli ya da doğru implementasyona yönlendirilmeli.
- `internal/ta/formations` bir Wyckoff motoru **değil** (sadece klasik geometri: swing/level/trendline/triangle/wedge/channel/breakout). Kod tabanında görülen "Accumulation Schematic Type 1/2", "Absorption" gibi Wyckoff-temalı formasyon isimleri sadece `patterns/generated/` kataloğunda metadata olarak var — bunlara özel geometrik/hacim tabanlı bir dedektör var mı yoksa genel şablon eşleştirmesine mi düştükleri ayrıca doğrulanmalı (bu denetimin kapsamı dışında kaldı).

---

## Önerilen uygulama sırası

1. **P0-1 → P0-5** — beşi de nispeten küçük, izole değişiklikler; her biri kendi testiyle (mevcut testler bunları yakalamıyor, yeni pinned-value testler eklenmeli).
2. **P1-1, P1-2, P1-3, P1-4** — indikatör katmanında kalan orta öncelikli formül düzeltmeleri.
3. **P1-8 → P1-11** — mum/grafik formasyon geometri düzeltmeleri.
4. **P1-12, P1-13, P1-15** — formasyon motoru seviye/dokunuş muhasebesi düzeltmeleri.
5. **P1-5, P1-6, P1-7** — divergence staleness + ölü proxy kod temizliği/bağlanması (tasarım kararı gerektirebilir — kullanıcıyla teyit).
6. **P1-14** — EMA kademeleme kararını dokümante et veya TradingView-uyumlu hale getir (davranış değişikliği, dikkatli ele alınmalı).
7. **P2** — test altyapısını harici/bağımsız referans değerlerle güçlendir; ölü kodu temizle veya bağla.

Her P0/P1 maddesi için: (a) düzeltme, (b) düzeltmeden önce kırmızı, sonra yeşil olan pinned-value bir test eklenecek.
