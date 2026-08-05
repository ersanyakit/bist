# Forecast Precision Veri ve Engine Yol Haritası

Güncelleme: 2026-06-25

Bu plan, mevcut forecast motorunu daha dar ve daha tutarlı karar bantları üretecek hale getirmek için gerekli veri katmanlarını, engine değişikliklerini ve kabul metriklerini tanımlar. Mevcut implementasyon geniş `risk interval` ile yeni `decision interval` alanını ayırır: risk bandı volatilite kapsama bandıdır, karar bandı ise rolling backtest kapanış hata dağılımından kalibre edilen daha dar conformal banttır.

## Mevcut Durum

- `risk interval`: ASELS 2026-06-01 / 2026-06-24 döneminde 18/18 isabet, ortalama genişlik yaklaşık %15.05.
- `decision interval`: aynı dönemde 12/18 isabet, ortalama genişlik yaklaşık %9.17.
- Nokta/scenario close üretimi publish gate tarafından kapalı: rolling close MAPE %2 eşiğinin üstünde ve yön isabeti karar seviyesinde değil.
- Karar bandı statüsü: `candidate_validation_failed`; izleme ve model geliştirme metriği olarak kullanılabilir, fakat karar/emir seviyesi değildir.

## Eksik Veri Katmanları

1. Intraday OHLCV

- 1m, 5m, 15m mumlar.
- Seans içi VWAP, first-hour range, last-hour momentum, opening gap continuation.
- Gün içi realized volatility ve volatility-of-volatility.
- Amaç: T+1 kapanışı sadece günlük mumdan değil, gün içi akışın kapanışa taşıdığı rejimden tahmin etmek.

2. Açılış/Kapanış Müzayedesi

- Açılış fiyatı, teorik eşleşme, kapanış fiyatı, kapanış seansı hacmi.
- Açılış gap kalitesi ve kapanış baskısı feature'ları.
- Amaç: gap günlerinde geniş bandı daha erken daraltmak.

3. Emir Defteri ve Likidite

- En iyi bid/ask, spread, derinlik, imbalance, order book slope.
- Kademelerde bekleyen hacim, iptal/ekleme baskısı, likidite boşluğu.
- Amaç: kısa vadeli yön ve bant genişliğini gerçek likidite rejimine bağlamak.

4. Endeks, Sektör ve Peer Bağlamı

- BIST100, BIST30, sektör endeksi, yakın peer getirileri.
- Relative strength, beta-adjusted return, peer dispersion.
- Amaç: hisse özel hareket ile piyasa/sector hareketini ayırmak.

5. KAP Zaman Damgası ve Olay Sınıfları

- KAP bildirimlerinin yayın saati, tip kodu, şirket etkisi, finansal tablo/event ayrımı.
- NLP tag'leri: finansal sonuç, sözleşme, ihale, yatırım, sermaye artırımı, temettü, yönetim, dava/risk.
- Amaç: haber etkili günleri normal teknik rejimden ayrı challenger modele yönlendirmek.

6. Makro ve Piyasa Akışı

- USDTRY, EURTRY, gösterge faiz, CDS, VIOP endeks kontratı, global futures.
- TCMB/EVDS serileri ve gün içi kur/faiz değişimleri.
- Amaç: savunma, banka, ihracatçı gibi makro duyarlı hisselerde rejim kırılımını yakalamak.

7. Kurumsal Aksiyon ve Temiz Fiyat

- Temettü, bedelli/bedelsiz, bölünme ve fiyat düzeltme katsayıları.
- Adjusted close ve raw close birlikte saklanmalı.
- Amaç: backtest residual dağılımının yapay corporate-action hatalarıyla bozulmasını engellemek.

8. Yatırımcı Akışı

- Yabancı payı, kurum bazlı takas değişimi, aracı kurum dağılımı, açığa satış/veri erişimi varsa ödünç akışı.
- Amaç: hacim anomalisini gerçek alıcı/satıcı kompozisyonuyla doğrulamak.

## Engine Fazları

### Faz 0 - Ölçüm ve Ledger

- `risk interval`, `decision interval`, point/scenario forecast ve actual close ayrı metriklerle saklanır.
- Her forecast satırında as-of veri kesiti, feature hash, model versiyonu ve validation statüsü bulunur.
- Kabul: karşılaştırma raporu risk bandı ile dar karar bandını ayrı hit/miss olarak göstermeli.

### Faz 1 - Conformal Karar Bandı

- Rolling close residual dağılımından q75/q80 bandı hesaplanır.
- Validation geçerse `active`, geçmezse `candidate_validation_failed`.
- Kabul: karar bandı ortalama genişliği risk bandından düşük olmalı; isabet oranı ayrıca raporlanmalı.

### Faz 2 - Rejim Sınıflandırıcı

- Rejimler: sakin, trend, yüksek volatilite, gap, squeeze-breakout, KAP/haber etkili, likidite bozuk.
- Her rejim için ayrı residual quantile ve ayrı feature ağırlığı kullanılmalı.
- Kabul: band kaçıran günlerin en az %80'i doğru rejim sebebiyle etiketlenmeli.

### Faz 3 - Feature Store

- Teknik indikatör, formasyon, destek/direnç, hacim profili, KAP event, intraday, sektör ve makro feature'ları aynı as-of sözleşmesiyle saklanır.
- Feature'lar leak-free olmalı; hedef gün actual bilgisi forecast satırına girmemeli.
- Kabul: aynı as-of tarihiyle tekrar çalıştırılan forecast deterministik olmalı.

### Faz 4 - Challenger Modeller

- Baseline teknik model yanında en az üç challenger gerekir:
- Volatilite rejimi modeli.
- KAP/makro event modeli.
- Likidite/intraday mikro yapı modeli.
- Ensemble yalnız validasyonu geçen challenger'ı karar bandına katmalı.
- Kabul: rolling close MAPE <= %2 ve yön isabeti >= %55 olmadan point/scenario close yayınlanmamalı.

### Faz 5 - Üretim Kapıları

- `active` karar bandı için minimum örnek sayısı: 30.
- Point forecast publish için rolling close MAPE <= %2, direction accuracy >= %55 ve data quality pass.
- High-conviction label için close MAPE <= %1.5, direction accuracy >= %60, decision interval hit-rate >= %70.
- Geniş risk bandı tek başına başarı sayılmamalı; karar kalitesi dar band ve point gate ile ölçülmeli.

## Kabul Komutları

```bash
go run ./cmd/hissebot forecast-walkforward -symbol ASELS -from 2026-06-01 -to 2026-06-24 -replace
go run ./cmd/hissebot verify-forecast -symbol ASELS -from 2026-06-01 -to 2026-06-24 -replace
go run ./cmd/hissebot forecast-error-audit -symbol ASELS -from 2026-06-01 -to 2026-06-24
go run ./cmd/hissebot forecast-compare-report -symbol ASELS -from 2026-06-01 -to 2026-06-24 -format md
go test ./internal/ta/analysis ./cmd/hissebot
```

## Hedef Metrikler

- Risk bandı hit-rate: >= %85, fakat ortalama genişlik ayrıca raporlanmalı.
- Decision interval hit-rate: ilk hedef >= %70, orta hedef >= %75.
- Decision interval ortalama genişliği: risk bandından en az %30 daha dar olmalı.
- Point forecast publish oranı: gate geçmeden artırılmamalı.
- Published point close MAPE: <= %2.
- Published direction accuracy: >= %55, tercih edilen üretim eşiği >= %60.

## Öncelik Sırası

1. Intraday OHLCV + opening/closing auction verisini ekle.
2. KAP event timestamp ve NLP sınıflarını forecast feature set'e bağla.
3. Sektör/endeks/peer relative-strength katmanını ekle.
4. Kurumsal aksiyon adjusted price zincirini backtest'e zorunlu yap.
5. Rejim bazlı conformal karar bandını devreye al.
6. Challenger modelleri publish gate'e bağla.
