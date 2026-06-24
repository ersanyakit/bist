// internal/localize/localize.go
package localize

import "strings"

func Bias(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bullish":
		return "Yükseliş"
	case "bearish":
		return "Düşüş"
	case "neutral":
		return "Nötr"
	default:
		return value
	}
}

func Direction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "long":
		return "Alış"
	case "short":
		return "Satış"
	case "neutral":
		return "Nötr"
	case "bullish":
		return "Yükseliş"
	case "bearish":
		return "Düşüş"
	default:
		return value
	}
}

func Signal(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bullish":
		return "Yükseliş"
	case "bearish":
		return "Düşüş"
	case "neutral":
		return "Nötr"
	case "info":
		return "Bilgi"
	case "oversold":
		return "Aşırı Satım"
	case "overbought":
		return "Aşırı Alım"
	case "high_volatility":
		return "Yüksek Oynaklık"
	case "near_level", "level_nearby":
		return "Seviyeye Yakın"
	case "strong_trend":
		return "Güçlü Trend"
	case "emerging_trend":
		return "Gelişen Trend"
	case "weak_trend":
		return "Zayıf Trend"
	case "proxy_only":
		return "Yaklaşık Hesap (Sinyal Değil)"
	case "requires_external_data":
		return "Hesaplanmadı: Dış Veri Gerekir"
	default:
		return value
	}
}

func Quality(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "Yüksek"
	case "medium":
		return "Orta"
	case "low":
		return "Düşük"
	default:
		return value
	}
}

func LevelType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "support":
		return "Destek"
	case "resistance":
		return "Direnç"
	default:
		return value
	}
}

func Timeframe(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "1D":
		return "Günlük"
	case "1W":
		return "Haftalık"
	case "1M":
		return "Aylık"
	case "3M":
		return "3 Aylık"
	case "6M":
		return "6 Aylık"
	case "1Y":
		return "Yıllık"
	case "YTD":
		return "Yıl Başı"
	case "ALL":
		return "Tüm Veri"
	default:
		return value
	}
}

func Bool(value bool) string {
	if value {
		return "Evet"
	}
	return "Hayır"
}

func Reason(value string) string {
	switch value {
	case "neutral trend bias does not provide enough directional edge":
		return "Nötr eğilim yeterli yön avantajı sağlamıyor"
	case "short selling is not supported for this instrument":
		return "Spot varlıkta short/marjin planı üretilmez; aktif alım planı yok"
	case "Bearish evidence does not create a spot equity long setup.", "Bearish evidence does not create a spot long setup.":
		return "Düşüş yönlü teknik kanıtlar spot varlık için alım kurulumu oluşturmaz."
	case "risk reward ratio is below 1.5":
		return "Risk/getiri oranı 1,5 seviyesinin altında"
	case "Nearest support is used as long risk reference.":
		return "En yakın destek, alış işlemi için risk referansı olarak kullanıldı."
	case "Nearest resistance is used as first upside objective.":
		return "En yakın direnç, ilk yukarı hedef olarak kullanıldı."
	case "Nearest resistance is too close to entry, so ATR based upside objectives are used.":
		return "En yakın direnç girişe çok yakın olduğu için yukarı hedefler ATR bazlı hesaplandı."
	case "Nearest resistance is used as short risk reference.":
		return "En yakın direnç, satış işlemi için risk referansı olarak kullanıldı."
	case "Nearest support is used as first downside objective.":
		return "En yakın destek, ilk aşağı hedef olarak kullanıldı."
	case "Nearest support is too close to entry, so ATR based downside objectives are used.":
		return "En yakın destek girişe çok yakın olduğu için aşağı hedefler ATR bazlı hesaplandı."
	case "ATR based entry, stop and targets are used because nearby levels are not available.":
		return "Yakın seviye bulunamadığı için giriş, zarar kes ve hedefler ATR bazlı hesaplandı."
	case "Nearest support is ignored because it is too far from the last price.":
		return "En yakın destek güncel fiyata çok uzak olduğu için işlem planı hesabında kullanılmadı."
	case "Nearest resistance is ignored because it is too far from the last price.":
		return "En yakın direnç güncel fiyata çok uzak olduğu için işlem planı hesabında kullanılmadı."
	case "Stop loss is normalized to a bounded ATR distance from entry.":
		return "Zarar kes seviyesi, giriş fiyatına göre makul ATR mesafesine sınırlandı."
	case "First target is normalized to an ATR based objective.":
		return "İlk hedef, makul ATR bazlı hedefe sınırlandı."
	case "Second target is normalized to an ATR based objective.":
		return "İkinci hedef, makul ATR bazlı hedefe sınırlandı."
	case "First target is floored to a positive price level.":
		return "İlk hedef pozitif fiyat seviyesinde kalacak şekilde düzeltildi."
	case "Second target is floored to a positive price level.":
		return "İkinci hedef pozitif fiyat seviyesinde kalacak şekilde düzeltildi."
	case "Trade plan is rejected because generated levels are not internally consistent.":
		return "Üretilen seviyeler kendi içinde tutarlı olmadığı için işlem planı reddedildi."
	case "Trend, momentum and pattern evidence are mixed.":
		return "Trend, momentum ve formasyon sinyalleri karışık."
	default:
		return value
	}
}

func Evidence(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "candlestick geometry matched the named setup":
		return "Mumların gövde ve fitil yapısı bu formasyonun temel şartlarıyla uyumlu."
	case "indicator snapshot matched the named setup":
		return "İndikatör verileri bu teknik yapının şartlarıyla uyumlu."
	case "volume and spread behavior matched the named setup":
		return "Hacim ve fiyat aralığı davranışı bu yapıyı destekliyor."
	case "ohlcv proxy matched the named auction-profile setup":
		return "Gerçek profil verisi olmadığı için bu yapı profesyonel formasyon sinyali olarak kullanılmaz."
	case "ohlcv proxy matched the named point-and-figure setup":
		return "Gerçek Point & Figure verisi olmadığı için bu yapı profesyonel formasyon sinyali olarak kullanılmaz."
	case "price structure matched the named setup":
		return "Fiyat yapısı bu formasyonun beklenen yapısıyla uyumlu."
	case "fallback scanner observed enough price movement to evaluate this named setup":
		return "Tarama motoru bu formasyonu değerlendirmek için yeterli fiyat hareketi gördü."
	case "completion bar volume is below the required twenty percent premium over volume sma20":
		return "Formasyonu tamamlayan mumun hacmi, 20 dönemlik ortalama hacmin en az %20 üstüne çıkmadı; hacim teyidi zayıf."
	case "completion bar volume is at least twenty percent above volume sma20":
		return "Formasyonu tamamlayan mumun hacmi, 20 dönemlik ortalama hacmin en az %20 üstünde; hacim teyidi güçlü."
	case "counterattack or meeting-line close alignment matched":
		return "Kapanışlar karşı atak/meeting line yapısına uygun hizalandı."
	case "dominant wick reversal candle matched":
		return "Belirgin fitilli dönüş mumu yapısı görüldü."
	case "long lower shadow matched":
		return "Uzun alt fitil görüldü; aşağıdan alıcı tepkisi işareti olabilir."
	case "long lower wick after a falling short term trend":
		return "Kısa vadeli düşüşten sonra uzun alt fitil oluştu; dipten tepki ihtimali verir."
	case "short body candle matched":
		return "Küçük gövdeli mum oluştu; piyasada kararsızlık olduğunu gösterir."
	case "small body with balanced upper and lower wicks":
		return "Küçük gövde ve dengeli üst-alt fitiller var; alıcı-satıcı dengesi kararsız."
	case "small body with unusually long upper and lower shadows":
		return "Küçük gövdeyle birlikte uzun üst ve alt fitiller var; oynak ve kararsız piyasa davranışı görülüyor."
	case "harami or inside-bar containment matched":
		return "İç bar/harami yapısı oluştu; fiyat önceki mum aralığı içinde sıkıştı."
	case "nested same-color body matched":
		return "Aynı renkli iç içe gövdeler görüldü; kısa vadeli sıkışma veya duraksama işareti olabilir."
	case "neck or thrusting recovery matched":
		return "Boyun/itme tipi toparlanma yapısı görüldü; tepki sınırlı kalabilir."
	case "bearish close cuts below the midpoint of the prior bullish body":
		return "Düşüş kapanışı önceki yükseliş mumunun orta noktasının altına indi; satış baskısı arttı."
	case "bullish displacement follows a bearish candle":
		return "Düşüş mumundan sonra güçlü yukarı yönlü hareket geldi; alıcıların devreye girdiğini gösterir."
	case "last bearish candle before a bullish displacement is visible":
		return "Güçlü yükselişten önceki son düşüş mumu görüldü; olası talep bölgesi işareti olabilir."
	case "last bullish candle before a bearish displacement is visible":
		return "Güçlü düşüşten önceki son yükseliş mumu görüldü; olası arz bölgesi işareti olabilir."
	case "morning-star style reversal matched":
		return "Sabah yıldızı benzeri dönüş yapısı görüldü; düşüş sonrası toparlanma ihtimali verir."
	case "belt-hold body and open location matched":
		return "Kuşak tutma yapısında açılış ve gövde şartları uyumlu."
	case "three advancing bullish candles matched":
		return "Arka arkaya ilerleyen üç yükseliş mumu görüldü."
	case "three consecutive strong bullish candles with rising closes":
		return "Kapanışları yükselen üç güçlü pozitif mum oluştu; alıcı momentumu belirgin."
	case "weakening bullish three-candle advance matched":
		return "Üç mumluk yükseliş var ancak güç kaybı işaretleri görülüyor."
	case "three-outside confirmation matched":
		return "Üç dış mum teyidi oluştu; önceki yapının yönü güçlendi."
	case "three candle sequence leaves an imbalance gap":
		return "Üç mumluk hareket fiyat dengesizliği/boşluk bıraktı."
	case "three-candle imbalance is visible":
		return "Üç mumluk yapıda fiyat dengesizliği görülüyor."
	case "bullish pole and corrective flag are visible":
		return "Güçlü yükseliş direği sonrası düzeltme bayrağı görülüyor."
	case "sharp rally is followed by a tight descending consolidation":
		return "Sert yükselişin ardından daralan aşağı eğimli dinlenme oluştu; bayrak yapısını destekler."
	case "sharp selloff is followed by a tight ascending consolidation":
		return "Sert düşüşün ardından daralan yukarı eğimli dinlenme oluştu; düşüş devam riski taşıyabilir."
	case "support and resistance slopes are both rising":
		return "Destek ve direnç çizgileri birlikte yükseliyor; yükselen kanal yapısı var."
	case "support is flat while swing highs fall":
		return "Destek yatay kalırken tepeler alçalıyor; alçalan üçgen baskısı oluşuyor."
	case "falling narrowing wedge is visible":
		return "Aşağı eğimli daralan kama yapısı görülüyor."
	case "falling range narrows into compression":
		return "Düşen fiyat aralığı daralarak sıkışmaya dönüştü."
	case "close breaks above recent range":
		return "Kapanış yakın dönem fiyat aralığının üstüne çıktı."
	case "close breaks above recent resistance with strong range expansion":
		return "Kapanış son direncin üstüne güçlü fiyat aralığı genişlemesiyle çıktı."
	case "close exceeds recent structure high":
		return "Kapanış yakın dönem yapı tepesini aştı; yapı yukarı kırıldı."
	case "generic breakout structure matched":
		return "Genel kırılım yapısı oluştu."
	case "strength breakout is visible":
		return "Güçlü kırılım işareti görülüyor."
	case "pullback holds above prior support after strength":
		return "Güçlü hareket sonrası geri çekilme önceki desteğin üstünde tutundu."
	case "middle swing low is below two similar shoulders":
		return "Orta dip, iki benzer omuz seviyesinin altında kaldı; ters omuz baş omuz yapısına işaret eder."
	case "multiple inverse shoulders cluster around a dominant head":
		return "Belirgin baş çevresinde birden fazla ters omuz kümelenmesi var."
	case "three comparable swing lows are visible":
		return "Birbirine yakın üç salınım dibi görülüyor."
	case "two adjacent wide candles form a bottom":
		return "Yan yana iki geniş mum dip yapısı oluşturdu."
	case "two adjacent wide candles form a bottom reversal":
		return "Yan yana iki geniş mum dip dönüşü ihtimali oluşturdu."
	case "two adjacent wide candles form a top":
		return "Yan yana iki geniş mum tepe yapısı oluşturdu."
	case "two adjacent wide candles form a top reversal":
		return "Yan yana iki geniş mum tepe dönüşü ihtimali oluşturdu."
	case "uptrend structure is visible":
		return "Yükselen trend yapısı görülüyor."
	case "point-and-figure proxy breakout matched":
		return "Gerçek Point & Figure verisi olmadan kırılım formasyonu hesaplanmadı."
	case "auction-profile proxy breakout matched":
		return "Gerçek auction/profile verisi olmadan profil formasyonu hesaplanmadı."
	case "price and volume diverged over recent window":
		return "Yakın dönemde fiyat ve hacim arasında uyumsuzluk oluştu."
	case "volume dried up below recent average":
		return "Hacim son dönem ortalamasının altına indi; ilgi zayıflıyor olabilir."
	case "volume expanded above recent average":
		return "Hacim son dönem ortalamasının üstüne çıktı; hareket daha güçlü teyit alıyor."
	case "volume is near recent average":
		return "Hacim son dönem ortalamasına yakın."
	case "volume average comparison is not available":
		return "Hacim ortalamasıyla karşılaştırma için yeterli veri yok."
	case "volume-derived indicator needs specific confirmation":
		return "Hacim türevi gösterge yön için kendi özel teyidini ister."
	case "large high-volume up bar appears near highs":
		return "Zirve bölgesine yakın yüksek hacimli güçlü yükseliş mumu oluştu."
	case "large up bar occurs near recent high with expanded volume":
		return "Yakın zirve civarında artan hacimli büyük yükseliş mumu oluştu."
	case "wide high-volume bar matched":
		return "Geniş aralıklı ve yüksek hacimli mum görüldü."
	case "narrow spread bar matched":
		return "Dar fiyat aralıklı mum oluştu; hareket sıkışıyor olabilir."
	case "sma50 is above sma200":
		return "50 dönemlik ortalama 200 dönemlik ortalamanın üstünde; orta vadeli trend olumlu."
	case "rsi condition matched":
		return "RSI şartı formasyonla uyumlu."
	case "macd condition matched":
		return "MACD şartı formasyonla uyumlu."
	case "ichimoku condition matched":
		return "Ichimoku şartı formasyonla uyumlu."
	case "bollinger band interaction matched":
		return "Bollinger bandı ile fiyat etkileşimi formasyonla uyumlu."
	case "bollinger expansion or breakout matched":
		return "Bollinger bantlarında genişleme veya kırılım şartı oluştu."
	case "moving-average condition matched":
		return "Hareketli ortalama şartı sağlandı."
	case "rising method or mat-hold structure matched":
		return "Yükselen üçlü yöntem / mat tutma yapısı uyumlu."
	case "5-0 swing retracement is visible":
		return "5-0 harmonik yapısına benzer geri çekilme görülüyor."
	case "abc correction has a strong middle leg":
		return "ABC düzeltmesinde orta bacak belirgin güçlü."
	case "abc correction stays within a flat range":
		return "ABC düzeltmesi yatay bir aralık içinde kalıyor."
	case "abc corrective swing sequence is visible":
		return "ABC düzeltme salınım dizisi görülüyor."
	case "alternating impulse-like swing sequence is visible":
		return "İtki dalgasına benzeyen dönüşümlü salınım dizisi görülüyor."
	case "five point swing reversal sequence has a mid retracement":
		return "Beş noktalı dönüş dizisinde orta geri çekilme yapısı var."
	case "flat correction fails to fully retrace the preceding move":
		return "Yatay düzeltme önceki hareketi tamamen geri alamadı."
	case "flat correction slightly exceeds the prior swing extreme":
		return "Yatay düzeltme önceki salınım ucunu hafifçe aştı."
	case "flat correction structure is visible":
		return "Yatay düzeltme yapısı görülüyor."
	case "shark-like harmonic ratios are visible":
		return "Köpekbalığı harmonik formasyonuna benzeyen oranlar görülüyor."
	case "volatile harmonic five point sequence resembles shark proportions":
		return "Oynak beş noktalı harmonik yapı Köpekbalığı oranlarına benziyor."
	case "bearish divergence proxy matched":
		return "Negatif uyumsuzluk benzeri sinyal oluştu."
	case "bullish divergence proxy matched":
		return "Pozitif uyumsuzluk benzeri sinyal oluştu."
	case "no divergence proxy matched":
		return "Belirgin bir uyumsuzluk sinyali görülmedi."
	case "cross condition evaluated from moving averages":
		return "Hareketli ortalamalar arasındaki kesişim şartı değerlendirildi."
	case "derived level is not near current price":
		return "Türetilen teknik seviye güncel fiyata yakın değil."
	case "indicator value is neutral":
		return "İndikatör değeri nötr bölgede."
	case "indicator value was computed for informational use":
		return "İndikatör değeri bilgi amaçlı hesaplandı."
	case "exact indicator formula is not implemented; ohlcv proxy value is kept for audit only":
		return "Bu indikatörün tam formülü uygulanmadığı için OHLCV tabanlı yaklaşık değer yalnızca denetim amacıyla tutuldu."
	case "adx indicates trend strength, not direction":
		return "ADX trendin gücünü ölçer; tek başına yön sinyali değildir."
	case "bounded momentum oscillator value evaluated":
		return "Sınırlı aralıkta çalışan momentum osilatörü değerlendirildi."
	case "centered momentum indicator value evaluated":
		return "Sıfır merkezli momentum indikatörü değerlendirildi."
	case "trend indicator is not price-scaled; directional price comparison was skipped":
		return "Bu trend indikatörü fiyat ölçeğinde olmadığı için fiyatla doğrudan yön karşılaştırması yapılmadı."
	case "cci oscillator value evaluated":
		return "CCI osilatörü değerlendirildi."
	case "williams %r oscillator value evaluated":
		return "Williams %R osilatörü değerlendirildi."
	case "bollinger %b value evaluated":
		return "Bollinger %B değeri değerlendirildi."
	case "completion bar volume is below the twenty percent premium over volume sma20":
		return "Formasyonu tamamlayan mumun hacmi, Volume SMA20'nin %20 üstü teyit eşiğine ulaşmadı."
	case "volume behavior is part of the matched setup":
		return "Hacim davranışı eşleşen teknik yapının parçası."
	case "large high-volume down bar appears near lows":
		return "Dip bölgesine yakın yüksek hacimli geniş düşüş mumu oluştu."
	case "pullback holds after strength":
		return "Güçlü hareket sonrası geri çekilme tutundu."
	case "bearish sequence changes into bullish break":
		return "Düşüş dizilimi yukarı yönlü yapı kırılımına döndü."
	case "ichimoku tenkan/kijun cross matched":
		return "Ichimoku Tenkan/Kijun kesişimi şartı oluştu."
	case "successive swings broaden":
		return "Ardışık salınımlar genişledi; genişleyen yapı oluştu."
	case "structure break was retested and held":
		return "Yapı kırılımı yeniden test edildi ve korundu."
	case "rounded recovery and handle are visible":
		return "Yuvarlanan toparlanma ve kulp benzeri dinlenme yapısı görülüyor."
	case "price broke the ichimoku cloud":
		return "Fiyat Ichimoku bulutunu kırdı."
	case "moving averages expanded into a directional ribbon":
		return "Hareketli ortalamalar yönlü bir şerit halinde açıldı."
	case "weakness breakdown is visible":
		return "Zayıflıkla gelen aşağı kırılım görülüyor."
	case "horizontal range boundaries are visible":
		return "Yatay fiyat aralığının sınırları görülüyor."
	case "fast decline and fast reversal are visible":
		return "Hızlı düşüş sonrası hızlı dönüş yapısı görülüyor."
	case "double-bottom variant is visible":
		return "Çift dip varyasyonu görülüyor."
	case "volume expanded while price held the upper part of a base":
		return "Fiyat tabanın üst bölümünde tutunurken hacim genişledi."
	case "upside gap is visible":
		return "Yukarı yönlü fiyat boşluğu görülüyor."
	case "successive swing highs and lows form a stair-step trend":
		return "Ardışık tepe ve dipler merdiven tipi trend yapısı oluşturuyor."
	case "recent range expanded versus prior range":
		return "Son fiyat aralığı önceki aralığa göre genişledi."
	case "price held a moving-average pullback":
		return "Fiyat hareketli ortalama geri çekilmesini korudu."
	case "moving averages crossed on the latest bar":
		return "Hareketli ortalamalar son mumda kesişti."
	case "liquidity sweep and reclaim/rejection matched":
		return "Likidite süpürmesi sonrası geri kazanım veya ret yapısı oluştu."
	case "contracting highs and lows are visible":
		return "Daralan tepe ve dipler görülüyor."
	case "close broke the latest market structure boundary":
		return "Kapanış son piyasa yapısı sınırını kırdı."
	case "rsi momentum condition matched":
		return "RSI momentum şartı oluştu."
	case "ichimoku cloud support matched":
		return "Ichimoku bulut desteği şartı oluştu."
	case "up candle volume exceeded recent down-volume and broke a short base":
		return "Yükseliş mumunun hacmi son düşüş hacmini aştı ve kısa tabanı kırdı."
	case "two comparable upward swing legs are visible":
		return "Birbirine yakın iki yukarı salınım bacağı görülüyor."
	case "two bearish candles follow an up candle near highs":
		return "Zirveye yakın yükseliş mumunu iki düşüş mumu izledi."
	case "trend pullback held near sma20":
		return "Trend geri çekilmesi SMA20 yakınında tutundu."
	case "sharp rebound follows weakness":
		return "Zayıflık sonrası sert tepki yükselişi geldi."
	case "rising or falling wedge compression matched":
		return "Yükselen veya düşen kama sıkışması oluştu."
	case "expansion then compression appears near lows":
		return "Dip bölgesine yakın önce genişleme sonra sıkışma görülüyor."
	case "downside break fails back inside range":
		return "Aşağı kırılım başarısız oldu ve fiyat tekrar aralığa döndü."
	case "close is in the discount portion of the recent range":
		return "Kapanış son fiyat aralığının iskontolu alt bölümünde."
	case "bounce holds below prior resistance after weakness":
		return "Zayıflık sonrası tepki, önceki direnç altında kaldı."
	case "requires market-wide breadth data, not available in single-symbol OHLCV":
		return "Bu gösterge piyasa geneli genişlik verisi ister; tek varlık OHLCV verisiyle hesaplanmaz."
	case "requires sentiment/options/positioning data, not available in single-symbol OHLCV":
		return "Bu gösterge duyarlılık, opsiyon veya pozisyonlanma verisi ister; tek varlık OHLCV verisiyle hesaplanmaz."
	case "requires order book, bid/ask or footprint data, not available in OHLCV":
		return "Bu gösterge emir defteri, alış/satış kotasyonu veya footprint verisi ister; OHLCV ile hesaplanmaz."
	case "requires options chain/greeks data, not available in OHLCV":
		return "Bu gösterge opsiyon zinciri veya Greeks verisi ister; OHLCV ile hesaplanmaz."
	case "requires exchange/on-chain crypto data, not available in OHLCV":
		return "Bu gösterge borsa veya on-chain kripto verisi ister; OHLCV ile hesaplanmaz."
	case "requires intraday TPO/session profile data; OHLCV proxy was not sufficient":
		return "Bu gösterge gün içi TPO/seans profil verisi ister; günlük OHLCV ile güvenilir hesaplanmaz."
	case "requires external data not available in current scanner input":
		return "Bu gösterge mevcut tarama girdisinde bulunmayan ek veri ister."
	case "market-structure proxy is active":
		return "Piyasa yapısı sinyali aktif."
	case "market-structure proxy is not directional":
		return "Piyasa yapısı sinyali yön belirtmiyor."
	case "momentum oscillator value evaluated":
		return "Momentum osilatörü değeri değerlendirildi."
	case "negative indicator value":
		return "İndikatör değeri negatif bölgede."
	case "positive indicator value":
		return "İndikatör değeri pozitif bölgede."
	case "price is above trend indicator value":
		return "Fiyat trend göstergesinin üstünde."
	case "price is below trend indicator value":
		return "Fiyat trend göstergesinin altında."
	case "price is above volume-weighted reference level":
		return "Fiyat hacim ağırlıklı referans seviyenin üstünde."
	case "price is below volume-weighted reference level":
		return "Fiyat hacim ağırlıklı referans seviyenin altında."
	case "price level comparison is not available":
		return "Fiyat/seviye karşılaştırması için yeterli veri yok."
	case "price is on the reference level":
		return "Fiyat referans seviyesinde."
	case "price is near derived support/resistance level":
		return "Fiyat türetilen destek/direnç seviyesine yakın."
	case "volatility condition is elevated or compressed setup is active":
		return "Oynaklık yükselmiş veya sıkışma yapısı aktif."
	case "volatility condition is not extreme":
		return "Oynaklık aşırı bir bölgede değil."
	case "volume contracted below recent average":
		return "Hacim son dönem ortalamasının altına daraldı."
	case "macd line is above signal and histogram is positive":
		return "MACD çizgisi sinyal çizgisinin üstünde ve histogram pozitif."
	case "macd line is below signal and histogram is negative":
		return "MACD çizgisi sinyal çizgisinin altında ve histogram negatif."
	case "macd histogram is positive":
		return "MACD histogramı pozitif."
	case "macd histogram is negative":
		return "MACD histogramı negatif."
	case "macd line and histogram are neutral":
		return "MACD çizgisi ve histogram nötr bölgede."
	case "ichimoku cloud state is bullish":
		return "Ichimoku bulut durumu yükseliş yönünde."
	case "ichimoku cloud state is bearish":
		return "Ichimoku bulut durumu düşüş yönünde."
	case "ichimoku cloud state is neutral":
		return "Ichimoku bulut durumu nötr."
	case "chaikin money flow is positive":
		return "Chaikin para akışı pozitif."
	case "chaikin money flow is negative":
		return "Chaikin para akışı negatif."
	case "chaikin money flow is near neutral":
		return "Chaikin para akışı nötre yakın."
	case "cumulative volume indicator needs slope confirmation":
		return "Kümülatif hacim göstergesi yön için eğim teyidi ister."
	case "moving average ribbon is bullish":
		return "Hareketli ortalama şeridi yükseliş yönünde."
	case "moving average ribbon is bearish":
		return "Hareketli ortalama şeridi düşüş yönünde."
	case "moving average ribbon is neutral":
		return "Hareketli ortalama şeridi nötr."
	case "time projection indicator is not a price level":
		return "Zaman projeksiyonu fiyat seviyesi değildir; işlem yönü üretmez."
	case "oscillator value evaluated":
		return "Osilatör değeri değerlendirildi."
	case "no nearby level was derived":
		return "Yakında anlamlı bir teknik seviye türetilemedi."
	case "requires exchange/on-chain crypto data, not available in ohlcv":
		return "Bu gösterge için borsa/on-chain kripto verisi gerekir; OHLCV verisi yeterli değildir."
	case "requires intraday tpo/session profile data; ohlcv proxy was not sufficient":
		return "Bu gösterge için gün içi TPO/seans profil verisi gerekir; OHLCV verisi yeterli değildir."
	case "requires market-wide breadth data, not available in single-symbol ohlcv":
		return "Bu gösterge için piyasa geneli genişlik verisi gerekir; tek varlık OHLCV verisi yeterli değildir."
	case "requires options chain/greeks data, not available in ohlcv":
		return "Bu gösterge için opsiyon zinciri/greeks verisi gerekir; OHLCV verisi yeterli değildir."
	case "requires order book, bid/ask or footprint data, not available in ohlcv":
		return "Bu gösterge için emir defteri, alış/satış veya footprint verisi gerekir; OHLCV verisi yeterli değildir."
	case "requires sentiment/options/positioning data, not available in single-symbol ohlcv":
		return "Bu gösterge için duyarlılık, opsiyon veya pozisyonlanma verisi gerekir; tek varlık OHLCV verisi yeterli değildir."
	default:
		return evidenceFallback(value)
	}
}

func EvidenceList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, Evidence(value))
	}
	return result
}

func evidenceFallback(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(normalized, "volume"):
		return "Hacim davranışı teknik yapıyla birlikte değerlendirildi."
	case strings.Contains(normalized, "moving average") || strings.Contains(normalized, "sma") || strings.Contains(normalized, "ema"):
		return "Hareketli ortalama şartı teknik yapı içinde değerlendirildi."
	case strings.Contains(normalized, "ichimoku"):
		return "Ichimoku şartı teknik yapı içinde değerlendirildi."
	case strings.Contains(normalized, "rsi") || strings.Contains(normalized, "macd") || strings.Contains(normalized, "oscillator"):
		return "Momentum indikatörü şartı değerlendirildi."
	case strings.Contains(normalized, "support") || strings.Contains(normalized, "resistance") || strings.Contains(normalized, "level"):
		return "Destek/direnç seviyesi teknik yapı içinde değerlendirildi."
	case strings.Contains(normalized, "break") || strings.Contains(normalized, "structure"):
		return "Fiyat yapısı ve kırılım şartı değerlendirildi."
	case strings.Contains(normalized, "candle") || strings.Contains(normalized, "wick") || strings.Contains(normalized, "body"):
		return "Mum gövdesi ve fitil yapısı değerlendirildi."
	default:
		return "Tarama motoru bu teknik şart için ek kanıt üretti."
	}
}

func PatternName(value string) string {
	replacements := []string{
		"Back Up to the Creek", "Dereye Geri Dönüş",
		"Jump Across the Creek", "Dere Üstü Kırılım",
		"Back Up to the Ice", "Buz Seviyesine Geri Dönüş",
		"Back Bullish to the Creek", "Dereye Geri Dönüş",
		"Back to the Creek", "Dereye Geri Dönüş",
		"Creek", "Dere Direnci",
		"Break and Retest", "Kırılım ve Retest",
		"Breakout Pullback", "Kırılım Geri Çekilmesi",
		"Support Resistance Flip", "Destek Direnç Dönüşümü",
		"Higher High", "Yükselen Tepe",
		"Higher Low", "Yükselen Dip",
		"Lower High", "Alçalan Tepe",
		"Lower Low", "Alçalan Dip",
		"Pullback", "Geri Çekilme",
		"Throwback", "Kırılım Sonrası Geri Çekilme",
		"Parabolic Advance", "Parabolik Yükseliş",
		"Spike Bottom", "Sivri Dip",
		"Spike Top", "Sivri Tepe",
		"Broadening Bottom", "Genişleyen Dip",
		"Broadening Top", "Genişleyen Tepe",
		"Volume Breakout", "Hacim Kırılımı",
		"Volume Expansion", "Hacim Genişlemesi",
		"Volume Climax", "Hacim Zirvesi",
		"Wide Spread Bar", "Geniş Aralıklı Bar",
		"Narrow Spread Bar", "Dar Aralıklı Bar",
		"Effort vs Result", "Efor Sonuç Uyumsuzluğu",
		"Market Structure Shift", "Piyasa Yapısı Değişimi",
		"Breakout Pattern", "Kırılım Formasyonu",
		"Pivot Point Breakout", "Pivot Kırılımı",
		"Squeeze Breakout", "Sıkışma Kırılımı",
		"Unique Three River Bottom", "Benzersiz Üç Nehir Dibi",
		"Re-distribution", "Yeniden Dağıtım",
		"Bullish", "Yükseliş",
		"Bearish", "Düşüş",
		"Engulfing", "Yutan",
		"Harami", "Harami",
		"Doji", "Doji",
		"Long Legged", "Uzun Bacaklı",
		"Marubozu", "Marubozu",
		"Hammer", "Çekiç",
		"Hanging Man", "Asılı Adam",
		"Inverted Hammer", "Ters Çekiç",
		"Shooting Star", "Kayan Yıldız",
		"Morning Star", "Sabah Yıldızı",
		"Evening Star", "Akşam Yıldızı",
		"Three White Soldiers", "Üç Beyaz Asker",
		"Three Black Crows", "Üç Siyah Karga",
		"Piercing Line", "Delici Çizgi",
		"Dark Cloud Cover", "Kara Bulut Örtüsü",
		"Tweezer Top", "Cımbız Tepe",
		"Tweezer Bottom", "Cımbız Dip",
		"Belt Hold", "Kuşak Tutma",
		"Closing", "Kapanış",
		"Opening", "Açılış",
		"Upper Shadow", "Üst Gölge",
		"Lower Shadow", "Alt Gölge",
		"Short Line Candle", "Kısa Mum",
		"Long Line Candle", "Uzun Mum",
		"Separating Lines", "Ayrılan Çizgiler",
		"Kicking", "Tekme",
		"Counterattack", "Karşı Atak",
		"Abandoned Baby", "Terk Edilmiş Bebek",
		"Tri Star", "Üçlü Yıldız",
		"Side by Side White Lines", "Yan Yana Beyaz Çizgiler",
		"Mat Hold", "Mat Tutma",
		"Flag", "Bayrak",
		"Pennant", "Flama",
		"Double Top", "Çift Tepe",
		"Double Bottom", "Çift Dip",
		"Triple Top", "Üçlü Tepe",
		"Triple Bottom", "Üçlü Dip",
		"Head and Shoulders", "Omuz Baş Omuz",
		"Inverse Head and Shoulders", "Ters Omuz Baş Omuz",
		"Ascending Triangle", "Yükselen Üçgen",
		"Descending Triangle", "Alçalan Üçgen",
		"Symmetrical Triangle", "Simetrik Üçgen",
		"Rectangle", "Dikdörtgen",
		"Wedge", "Kama",
		"Cup and Handle", "Fincan Kulp",
		"Break of Structure", "Yapı Kırılımı",
		"Change of Character", "Karakter Değişimi",
		"Order Block", "Emir Bloğu",
		"Last Point of Support", "Son Destek Noktası",
		"Last Point of Supply", "Son Arz Noktası",
		"Rectangle Range", "Dikdörtgen Aralık",
		"Horizontal Channel", "Yatay Kanal",
		"Ascending Channel", "Yükselen Kanal",
		"Descending Channel", "Alçalan Kanal",
		"Rounding Bottom", "Yuvarlanan Dip",
		"Rounding Top", "Yuvarlanan Tepe",
		"Broadening Formation", "Genişleyen Formasyon",
		"Diamond Top", "Elmas Tepe",
		"Diamond Bottom", "Elmas Dip",
		"ABCD Pattern", "ABCD Formasyonu",
		"Gartley Pattern", "Gartley Formasyonu",
		"Bat Pattern", "Yarasa Formasyonu",
		"Butterfly Pattern", "Kelebek Formasyonu",
		"Crab Pattern", "Yengeç Formasyonu",
		"Deep Crab Pattern", "Derin Yengeç Formasyonu",
		"Shark Pattern", "Köpekbalığı Formasyonu",
		"Cypher Pattern", "Cypher Formasyonu",
		"Three Drives Pattern", "Üç Sürüş Formasyonu",
		"5-0 Pattern", "5-0 Formasyonu",
		"Impulse Wave 1-2-3-4-5", "İtki Dalgası 1-2-3-4-5",
		"Corrective ABC", "Düzeltici ABC",
		"Zigzag Correction", "Zigzag Düzeltmesi",
		"Flat Correction", "Yatay Düzeltme",
		"Expanded Flat", "Genişleyen Yatay",
		"Running Flat", "Koşan Yatay",
		"Triangle Correction", "Üçgen Düzeltmesi",
		"Leading Diagonal", "Öncü Diyagonal",
		"Ending Diagonal", "Biten Diyagonal",
		"Accumulation Schematic", "Akümülasyon Şeması",
		"Distribution Schematic", "Dağıtım Şeması",
		"Spring", "Yay",
		"Upthrust", "Yukarı Tuzak",
		"Sign of Strength", "Güç Belirtisi",
		"Sign of Weakness", "Zayıflık Belirtisi",
		"Buying Climax", "Alım Zirvesi",
		"Selling Climax", "Satış Zirvesi",
		"No Demand Bar", "Talep Yok Barı",
		"No Supply Bar", "Arz Yok Barı",
		"Price Volume Divergence", "Fiyat Hacim Uyumsuzluğu",
		"Volume Divergence", "Hacim Uyumsuzluğu",
		"Volume Dry-Up", "Hacim Kuruması",
		"Automatic Rally", "Otomatik Tepki Yükselişi",
		"Automatic Reaction", "Otomatik Tepki Düşüşü",
		"Liquidity Grab", "Likidite Süpürmesi",
		"Stop Hunt", "Stop Avı",
		"Fair Value Gap", "Adil Değer Boşluğu",
		"Breaker Block", "Kırıcı Blok",
		"Mitigation Block", "Azaltım Bloğu",
		"Equal Highs Liquidity", "Eşit Tepeler Likiditesi",
		"Equal Lows Liquidity", "Eşit Dipler Likiditesi",
		"Complex Head and Shoulders", "Karmaşık Omuz Baş Omuz",
		"Complex Inverse Head and Shoulders", "Karmaşık Ters Omuz Baş Omuz",
		"Measured Move Up", "Ölçülü Yukarı Hareket",
		"Measured Move Down", "Ölçülü Aşağı Hareket",
		"Bump and Run Reversal Top", "Hızlanma ve Dönüş Tepesi",
		"Bump and Run Reversal Bottom", "Hızlanma ve Dönüş Dibi",
		"Island Reversal Top", "Ada Dönüş Tepesi",
		"Island Reversal Bottom", "Ada Dönüş Dibi",
		"Pipe Top", "Boru Tepe",
		"Pipe Bottom", "Boru Dip",
		"V Bottom", "V Dip",
		"V Top", "V Tepe",
		"Adam and Eve Double Bottom", "Adem ve Havva Çift Dip",
		"Adam and Eve Double Top", "Adem ve Havva Çift Tepe",
		"Eve and Eve Double Bottom", "Havva ve Havva Çift Dip",
		"Adam and Adam Double Top", "Adem ve Adem Çift Tepe",
		"Rickshaw Man", "Çekçek Adam",
		"High Wave Candle", "Yüksek Dalga Mumu",
		"Matching Low", "Eşleşen Dip",
		"Matching High", "Eşleşen Tepe",
		"On Neck", "Boyunda",
		"In Neck", "Boyun İçinde",
		"Thrusting Pattern", "İtme Formasyonu",
		"Homing Pigeon", "Yuvalanan Güvercin",
		"Upside Gap Two Crows", "Yukarı Boşluk İki Karga",
		"Downside Gap Two Rabbits", "Aşağı Boşluk İki Tavşan",
		"Three Inside Up", "Üç İç Yükseliş",
		"Three Inside Down", "Üç İç Düşüş",
		"Three Outside Up", "Üç Dış Yükseliş",
		"Three Outside Down", "Üç Dış Düşüş",
		"Three Stars in the South", "Güneyde Üç Yıldız",
		"Three Advancing White Soldiers", "İlerleyen Üç Beyaz Asker",
		"Identical Three Crows", "Özdeş Üç Karga",
		"Deliberation", "Kararsız İlerleme",
		"Advance Block", "İlerleme Bloğu",
		"Two Crows", "İki Karga",
		"Unique Three River Bottom", "Benzersiz Üç Nehir Dibi",
		"Upside Tasuki Gap", "Yukarı Tasuki Boşluğu",
		"Downside Tasuki Gap", "Aşağı Tasuki Boşluğu",
		"Rising Window", "Yükselen Pencere",
		"Falling Window", "Düşen Pencere",
		"Upside Gap Three Methods", "Yukarı Boşluk Üç Yöntem",
		"Downside Gap Three Methods", "Aşağı Boşluk Üç Yöntem",
		"Ladder Bottom", "Merdiven Dibi",
		"Concealing Baby Swallow", "Gizleyen Bebek Kırlangıç",
		"Stick Sandwich", "Çubuk Sandviç",
		"Ascending", "Yükselen",
		"Descending", "Alçalan",
		"Horizontal", "Yatay",
		"Rounding", "Yuvarlanan",
		"Channel", "Kanal",
		"Range", "Aralık",
		"Formation", "Formasyon",
		"Pattern", "Formasyonu",
		"Correction", "Düzeltme",
		"Reversal", "Dönüş",
		"Top", "Tepe",
		"Bottom", "Dip",
		"Up", "Yükseliş",
		"Down", "Düşüş",
	}
	return strings.NewReplacer(replacements...).Replace(value)
}
