# Hisse Analiz Motoru Roadmap

Bu belge, mevcut hisse analiz motorunun durumunu ve dogruluk/tutarlilik icin kalan isleri fazlara boler. Amac tek bir "AL/SAT" skoru uretmek degil; veri guvenilirligi, teknik/quant/fundamental/makro kanit, validasyon ve portfoy riski ayni anda tutarliysa karar uretmektir.

## Mevcut Durum

Motor su katmanlari uretir:

- Teknik analiz: indikator, formasyon, destek/direnc, trade plan, next-session forecast.
- Professional analiz: veri yonetisimi, KAP/PDF kaniti, sektor/peer, valuation, makro baglam, likidite, backtest ve yatirimci soru-cevap.
- Quant analiz: getiri, volatilite, VaR/CVaR, drawdown, Sharpe/Sortino, benchmark beta/alpha/korelasyon.
- Statistical/economic analiz: veri butunlugu, faktor modeli, EWMA/GARCH vol rejimi, stres, makro duyarlilik, finansal kalite, likidite ve validasyon bilesik skoru.
- Karar destek: buy/sell/hold cevabini veri kapilari, production guvenligi ve kanit politikasi ile sinirlar.

Mevcut ciktilar:

- `analysis.json`: canonical analiz kontrati.
- `analiz.json`: Turkce kullanici cikti kontrati.
- `quant_risk_report.json`: quant risk/getiri raporu.
- `stat_economic_report.json`: istatistiksel/ekonomik tutarlilik raporu.
- `decision_support_standard.json`: karar kapilari ve minimum gereksinimler.
- `risk_matrix.json`: temel, teknik, quant ve stat/economic risk kayitlari.

## Kalan Ana Eksikler

Kalan eksikler dort gruba ayrilir:

- Veri ve zaman guvenligi: point-in-time zincir, resmi fiyat mutabakati, corporate action audit, lisansli benchmark/mikroyapi verisi.
- Ekonometrik model kalitesi: BIST faktorleri, makro duyarlilik katsayilari, rejim modelleri, model karsilastirma testleri.
- Fundamental/fair value derinligi: sektor bazli finansal kalite, Piotroski/Beneish/Altman, DCF/RI/DDM/peer/NAV ensemble, KAP event study.
- Operasyonel karar guvenligi: model monitoring, drift, calibration, portfoy optimizasyonu, market impact ve production checklist.

## Faz 0 - Stabilizasyon ve Kontrat Kilidi

Amaç: Mevcut quant ve stat/economic katmanlari icin JSON kontratini kilitlemek.

Yapilacaklar:

- `analysis.json` icin schema snapshot testi ekle.
- `quant` ve `stat_economic` alanlari icin geriye uyumluluk testi ekle.
- `analiz.json` Turkce alan adlari icin fixture testi ekle.
- `quant_risk_report.json` ve `stat_economic_report.json` artifact varlik testi ekle.
- `decision_support_standard.json` icinde `quant_risk_gate` ve `stat_economic_consistency_gate` kapilarinin her equity raporunda uretildigini test et.

Kabul kriterleri:

- `go test ./internal/ta/analysis ./internal/ta/storage ./internal/quant/...` yesil.
- Rapor JSON kontrati bilincli migration olmadan degismez.
- Eksik veri durumunda alanlar sifir degerle degil, `computed:false` ve uyarilarla gelir.

## Faz 1 - Veri Guvenilirligi ve Point-in-Time Katmani

Amaç: Hisse analizinin yanlis veya gelecegi bilen veriyle uretilmesini engellemek.

Eksik algoritmalar:

- Point-in-time data lineage scorer.
- OHLCV source reconciliation: TradingView, BIST DB, resmi bulten, local cache.
- Corporate action adjustment audit: temettu, bedelli, bedelsiz, bolunme, nominal sermaye degisimi.
- Survivorship bias detector.
- Duplicate/missing candle repair policy.
- Official close confidence model.
- Outlier quarantine: fiyat/hacim/split kaynakli mi, gercek piyasa hareketi mi?

Veri ihtiyaci:

- BIST resmi kapanis ve bulten kayitlari.
- Corporate action tarihleri ve oranlari.
- KAP sermaye/temettu bildirimleri.
- Veri dosyalari icin `fetched_at`, `source_timestamp`, `available_at`.

Uretilecek ciktilar:

- `data_quality_report.json`
- `price_reconciliation_report.json`
- `corporate_action_audit.json`
- `point_in_time_lineage.json`

Kabul kriterleri:

- Verified close yoksa production trade gate acilmaz.
- Corporate action uyumsuzlugu varsa teknik/quant skor sinirlanir.
- Financial publish date/available-at eksikse backtest-safe sinyali uretilmez.

## Faz 2 - BIST Faktor Modeli

Amaç: Hissenin getirisini piyasa, sektor ve stil faktorlerine ayirmak.

Eksik algoritmalar:

- BIST CAPM: beta, alpha, residual volatility.
- Sector-relative model: XU100 + sektor endeksi ayrisimi.
- BIST style factors: size, value, momentum, quality, low-volatility, liquidity.
- Rolling beta/alpha stability.
- Newey-West HAC robust standard errors.
- Factor exposure confidence score.
- Active return decomposition.

Veri ihtiyaci:

- XU100 ve sektor endeksleri.
- Hisse piyasa degeri, defter degeri, karlilik, aktif buyume.
- Likidite/turnover serileri.
- En az 252 gunluk fiyat gecmisi, ideal 3-5 yil.

Uretilecek ciktilar:

- `factor_model_report.json`
- `relative_strength_report.json`
- `active_return_decomposition.json`

Kabul kriterleri:

- Her hisse icin "hisse ozel alfa mi, sektor/piyasa etkisi mi" ayrimi yapilir.
- Beta/alpha katsayilari confidence ile gelir.
- Faktor datasindan biri eksikse model partial computed olur, sessizce sifir yazmaz.

## Faz 3 - Volatilite, Rejim ve Kuyruk Riski

Amaç: Riskin sabit olmadigini kabul eden piyasa rejimi uretmek.

Eksik algoritmalar:

- EWMA volatility production modeli.
- GARCH/EGARCH/GJR-GARCH kalibrasyonu.
- Volatility clustering testi.
- Markov regime switching: bull/range/bear ve low/normal/high vol.
- Cornish-Fisher VaR.
- Extreme Value Theory / POT tail risk.
- Bootstrap stress simulation.
- Gap risk ve limit-up/limit-down riski.

Veri ihtiyaci:

- Gunluk OHLCV.
- Mümkünse intraday veya seans ici veri.
- BIST devre kesici ve tavan/taban kosullari.

Uretilecek ciktilar:

- `volatility_regime_report.json`
- `tail_risk_report.json`
- `stress_test_report.json`

Kabul kriterleri:

- Yüksek vol rejiminde trade plan risk boyutu otomatik dusurulur.
- CVaR ve stress senaryosu karar destek metninde gorunur.
- Volatilite modeli backtest metrikleriyle izlenir.

## Faz 4 - Makroekonomik Duyarlilik

Amaç: Türkiye piyasasinda faiz, kur, enflasyon, buyume ve likidite etkisini hisse bazinda modellemek.

Eksik algoritmalar:

- Sector macro exposure map.
- TCMB politika faizi ve getiri egrisi duyarliligi.
- Kur duyarliligi: ihracatci, ithalatci, doviz borcu, doviz geliri ayrimi.
- Enflasyon pass-through proxy.
- CDS/risk primi ve equity risk premium modeli.
- VAR/BVAR makro rejim modeli.
- MIDAS regression: aylik/ceyreklik makro veriyi gunluk fiyatla birlestirme.
- Macro surprise model: beklenti-gerceklesen farki.

Veri ihtiyaci:

- TCMB EVDS: faiz, kur, rezerv, kredi, mevduat, para arzı.
- TÜİK: enflasyon, sanayi uretimi, issizlik, GDP.
- BIST sektor endeksleri.
- Sirket finansallarinda doviz pozisyonu ve borc yapisi.

Uretilecek ciktilar:

- `macro_sensitivity_report.json`
- `macro_regime_report.json`
- `macro_scenario_stress.json`

Kabul kriterleri:

- Banka, GYO, ihracatci, holding, sanayi ve teknoloji farkli makro profillerle degerlenir.
- Makro ters ruzgar varsa next-session ve trade gate metninde acik uyarilir.
- Makro veri point-in-time degilse production gate limited/fail olur.

## Faz 5 - Finansal Kalite ve Muhasebe Riski

Amaç: Finansal tablolarin sadece oranlarini degil, kalitesini ve manipülasyon riskini olcmek.

Eksik algoritmalar:

- Piotroski F-Score.
- Beneish M-Score.
- Altman Z-Score veya sektor uyarlamasi.
- DuPont decomposition.
- Accrual quality.
- Earnings persistence.
- Revenue quality ve margin stability.
- Debt sustainability.
- Cash conversion cycle.
- Sector-specific scorecards: banka, sigorta, GYO, holding, sanayi.

Veri ihtiyaci:

- KAP finansal tablolarinin normalize edilmis kalemleri.
- 5-10 yillik bilanço/gelir/nakit akimi gecmisi.
- Bankalar icin SYR, CET1, NPL, LCR, kredi/mevduat, NIM.
- GYO icin portfoy/NAV detaylari.

Uretilecek ciktilar:

- `financial_quality_scorecard.json`
- `accounting_risk_report.json`
- `sector_specific_financial_report.json`

Kabul kriterleri:

- Finansal kalite dusukse valuation confidence sinirlanir.
- Banka/GYO/holding icin yanlis sanayi orani kullanilmaz.
- Red flag varsa investment committee memo bunu acik yazar.

## Faz 6 - Değerleme Ensemble

Amaç: Tek hedef fiyat yerine model ailesiyle adil deger araligi uretmek.

Eksik algoritmalar:

- DCF scenario ensemble.
- Residual income model.
- Dividend discount model.
- EV/EBITDA, P/E, P/BV, P/S peer model.
- Sum-of-parts proxy.
- NAV proxy / full NAV, ozellikle GYO icin.
- Monte Carlo valuation.
- Sensitivity grid: WACC, growth, margin, FX, inflation, terminal growth.
- Model reliability weighting.

Veri ihtiyaci:

- Finansal tablo gecmisi.
- Peer evreni ve sektor carpanlari.
- WACC girdileri: risk-free, ERP, beta, borc maliyeti.
- KAP varlik/portfoy detaylari.

Uretilecek ciktilar:

- `valuation_ensemble_report.json`
- `valuation_sensitivity_table.json`
- `fair_value_bridge.json`

Kabul kriterleri:

- Fair value tek sayi degil bear/base/bull araligi olarak gelir.
- Model girdisi eksikse hedef fiyat degil, eksik veri listesi uretilir.
- Degerleme confidence karar destek kapisini etkiler.

## Faz 7 - KAP Event Study ve Haber Etki Modeli

Amaç: KAP/haber olaylarinin fiyat, hacim ve volatilite uzerindeki etkisini olcmek.

Eksik algoritmalar:

- Event study abnormal return.
- Earnings surprise model.
- Dividend/capital increase impact model.
- Contract/tender/order announcement impact classifier.
- Buyback impact model.
- Legal/regulatory risk event classifier.
- Pre/post volume and volatility shift.
- NLP sentiment and materiality score.

Veri ihtiyaci:

- KAP bildirim tarih/saat, konu, ek dosya, metin.
- Event publish timestamp ve market open/close alignment.
- Olay oncesi/sonrasi fiyat/hacim pencereleri.

Uretilecek ciktilar:

- `event_study_report.json`
- `kap_materiality_score.json`
- `news_event_impact_report.json`

Kabul kriterleri:

- Olay etkisi sadece haber basligi olarak kalmaz, abnormal return ile olculur.
- Publish time kapanis sonrasiysa ilk uygulanabilir bar dogru secilir.
- Event kaynakli sinyal ile teknik sinyal ayri gosterilir.

## Faz 8 - Tahmin Validasyonu ve Model Monitoring

Amaç: Modelin dogru oldugunu varsaymak yerine surekli olcmek.

Eksik algoritmalar:

- Walk-forward validation.
- Purged/embargoed cross-validation.
- Diebold-Mariano forecast comparison.
- Brier score ve calibration curve.
- Prediction interval coverage.
- Regime bazli hit rate.
- Data drift ve model drift.
- Feature importance stability.
- Champion/challenger model registry.

Veri ihtiyaci:

- Eski forecast kayitlari.
- Gerceklesen resmi acilis/kapanis.
- Model versiyonu, feature set versiyonu, data snapshot hash.

Uretilecek ciktilar:

- `forecast_validation_report.json`
- `model_monitoring_report.json`
- `champion_challenger_report.json`

Kabul kriterleri:

- Nokta tahmin publish edilecekse validation gate gecmek zorunda.
- Model regime bazinda zayifsa karar metni bunu saklamaz.
- Yeni model eskisinden iyi degilse otomatik production'a gecmez.

## Faz 9 - Likidite, Market Impact ve Portfoy Katmani

Amaç: Iyi gorunen hissenin gercek portfoyde tasinabilir olup olmadigini olcmek.

Eksik algoritmalar:

- Amihud illiquidity production kalibrasyonu.
- Bid-ask spread model.
- Order book imbalance.
- Slippage estimator.
- Market impact model.
- Capacity model: ADV yuzdesi, cikis gunu, maksimum notional.
- Portfolio optimizer entegrasyonu: min variance, max Sharpe, risk parity.
- Portfolio marginal/incremental VaR.
- Concentration and correlation risk.

Veri ihtiyaci:

- Canli veya gecikmeli order book.
- Islem hacmi/deger serileri.
- Portfoy mevcut pozisyonlari.
- BIST sektor ve benchmark korelasyon matrisi.

Uretilecek ciktilar:

- `liquidity_impact_report.json`
- `portfolio_fit_report.json`
- `position_capacity_report.json`

Kabul kriterleri:

- Likidite dusukse buyuk yatirimci karari otomatik limited/fail olur.
- Pozisyon boyutu VaR, stop mesafesi ve ADV kapasitesiyle sinirlanir.
- Portfoy etkisi hesaplanmadan kurumsal "pozisyon ac" onayi verilmez.

## Faz 10 - Production Karar Orkestrasyonu

Amaç: Tum katmanlari deterministik, denetlenebilir ve surumlenebilir karar motoruna baglamak.

Eksik algoritmalar:

- Evidence-weighted ensemble decision.
- Gate hierarchy: data -> validation -> risk -> valuation -> execution.
- Human review queue.
- Model/version registry.
- Reproducible report hash.
- Audit trail: hangi veri, hangi model, hangi karar.
- API contract versioning.

Veri ihtiyaci:

- Tum kaynaklar icin source hash.
- Model parametreleri ve versiyon metadata.
- Kullanici/kurum risk profili.
- Report approval workflow.

Uretilecek ciktilar:

- `decision_audit_trail.json`
- `model_registry_snapshot.json`
- `production_readiness_report.json`
- `human_review_queue.json`

Kabul kriterleri:

- Karar tekrar uretildiginde ayni veri/model versiyonuyla ayni sonuc verir.
- Production mode, research mode'dan daha katı kapilar kullanir.
- Eksik veri varsa model tahmin uydurmaz; eksigi ve etkisini raporlar.

## Fazlar Arasi Oncelik

Onerilen sira:

1. Faz 0: Kontrat kilidi.
2. Faz 1: Veri guvenilirligi.
3. Faz 8: Validasyon ve monitoring.
4. Faz 2: Faktor modeli.
5. Faz 4: Makro duyarlilik.
6. Faz 5: Finansal kalite.
7. Faz 3: Rejim/kuyruk riski.
8. Faz 7: Event study.
9. Faz 9: Likidite/portfoy.
10. Faz 6 ve Faz 10: Degerleme ensemble ve production orkestrasyon.

Bu sira bilerek secildi: once verinin ve validasyonun guvenli olmasi gerekir. Aksi halde daha karmasik modeller sadece daha karmasik hatalar uretir.

## Definition of Done

Bir faz tamamlandi sayilmaz; su dort kosul ayni anda gecmelidir:

- Kod: Deterministik Go implementasyonu ve testleri var.
- Veri: Gerekli kaynaklar `computed/available/missing` olarak acik raporlaniyor.
- Rapor: `analysis.json`, `analiz.json` ve ilgili artifact dosyasi alanlari doluyor.
- Karar: Decision support kapilari yeni katmani dikkate aliyor.

## Riskler

- Lisansli piyasa verisi eksikse order book, spread ve market impact modelleri proxy kalir.
- KAP PDF/XBRL normalizasyonu eksikse finansal kalite skorunda sektor bazli yanilma olur.
- Makro veriler point-in-time degilse backtest sonucu gercekci olmaz.
- Faz 6 degerleme modelleri veri kalitesi duzelmeden uygulanirsa yanlis hedef fiyat guveni yaratir.

