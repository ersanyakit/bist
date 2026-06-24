package contrarian

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/ohlcv"
	"hissebot/pkg/mathutil"
)

type Input struct {
	EquityDir          string
	Symbol             string
	AssetType          string
	AsOf               time.Time
	LastClose          float64
	Candles            []ohlcv.Candle
	Indicators         ohlcv.IndicatorSnapshot
	NearestSupport     *ohlcv.SupportResistanceLevel
	NearestResistance  *ohlcv.SupportResistanceLevel
	TrendBias          string
	RequireQualityGate bool
}

type Report struct {
	SourceCoverage SourceCoverage     `json:"source_coverage"`
	Sentiment      SentimentReport    `json:"sentiment"`
	Capitulation   CapitulationReport `json:"capitulation"`
	Contrarian     ContrarianReport   `json:"contrarian"`
	Backtest       BacktestReport     `json:"backtest"`
}

type SourceCoverage struct {
	NewsItemCount      int      `json:"news_item_count"`
	KAPDisclosureCount int      `json:"kap_disclosure_count,omitempty"`
	CommentCount       int      `json:"comment_count"`
	RecentTextCount    int      `json:"recent_text_count"`
	AnalyzedTextCount  int      `json:"analyzed_text_count"`
	HasCommentData     bool     `json:"has_comment_data"`
	HasRecentSentiment bool     `json:"has_recent_sentiment"`
	Warnings           []string `json:"warnings,omitempty"`
}

type SentimentReport struct {
	Score         float64      `json:"score"`
	Negativity    float64      `json:"negativity"`
	Label         string       `json:"label"`
	PositiveHits  int          `json:"positive_hits"`
	NegativeHits  int          `json:"negative_hits"`
	PanicHits     int          `json:"panic_hits"`
	TopSignals    []TextSignal `json:"top_signals,omitempty"`
	PlainLanguage string       `json:"plain_language"`
}

type TextSignal struct {
	Source      string    `json:"source"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	Title       string    `json:"title,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Tone        string    `json:"tone"`
	Score       float64   `json:"score"`
	Keywords    []string  `json:"keywords,omitempty"`
}

type CapitulationReport struct {
	Score         float64          `json:"score"`
	Label         string           `json:"label"`
	Components    []ScoreComponent `json:"components"`
	Evidence      []string         `json:"evidence"`
	PlainLanguage string           `json:"plain_language"`
}

type ScoreComponent struct {
	Label  string  `json:"label"`
	Score  float64 `json:"score"`
	Weight float64 `json:"weight"`
	Detail string  `json:"detail"`
}

type ContrarianReport struct {
	Score         float64     `json:"score"`
	Label         string      `json:"label"`
	Signal        string      `json:"signal"`
	Action        string      `json:"action"`
	QualityGate   QualityGate `json:"quality_gate"`
	PlainLanguage string      `json:"plain_language"`
	Evidence      []string    `json:"evidence"`
}

type QualityGate struct {
	Status              string   `json:"status"`
	CanAffectBuySignal  bool     `json:"can_affect_buy_signal"`
	Reason              string   `json:"reason"`
	Warnings            []string `json:"warnings,omitempty"`
	RequiresWalkForward bool     `json:"requires_walk_forward"`
}

type BacktestReport struct {
	Available       bool    `json:"available"`
	ProxyOnly       bool    `json:"proxy_only"`
	SampleSize      int     `json:"sample_size"`
	ForwardBars     int     `json:"forward_bars"`
	WinRate         float64 `json:"win_rate"`
	AverageReturn   float64 `json:"average_return"`
	MedianReturn    float64 `json:"median_return"`
	PlainLanguage   string  `json:"plain_language"`
	InsufficientWhy string  `json:"insufficient_why,omitempty"`
}

type textItem struct {
	source      string
	publishedAt time.Time
	title       string
	summary     string
}

func Analyze(input Input) Report {
	if len(input.Candles) > 0 && input.LastClose <= 0 {
		input.LastClose = input.Candles[len(input.Candles)-1].EffectiveClose()
	}
	if input.AsOf.IsZero() && len(input.Candles) > 0 {
		input.AsOf = input.Candles[len(input.Candles)-1].Time
	}
	if input.AsOf.IsZero() {
		input.AsOf = time.Now().UTC()
	}
	texts, coverage := loadTextInputs(input.EquityDir, input.AsOf, input.AssetType)
	sentiment := analyzeSentiment(texts, input.AssetType)
	coverage.RecentTextCount = countRecentTexts(texts, input.AsOf, 120)
	coverage.AnalyzedTextCount = len(texts)
	coverage.HasRecentSentiment = coverage.RecentTextCount > 0
	if len(texts) == 0 {
		coverage.Warnings = append(coverage.Warnings, "sentiment_text_data_missing")
	}
	if !coverage.HasCommentData {
		coverage.Warnings = append(coverage.Warnings, "external_comment_or_news_sentiment_missing")
	}
	if !coverage.HasRecentSentiment {
		coverage.Warnings = append(coverage.Warnings, "recent_sentiment_text_missing")
	}
	capitulation := analyzeCapitulation(input)
	backtest := backtestCapitulationProxy(input.Candles, 5)
	contrarian := analyzeContrarian(sentiment, capitulation, backtest, coverage, input)
	return Report{
		SourceCoverage: coverage,
		Sentiment:      sentiment,
		Capitulation:   capitulation,
		Contrarian:     contrarian,
		Backtest:       backtest,
	}
}

func loadTextInputs(equityDir string, asOf time.Time, assetType string) ([]textItem, SourceCoverage) {
	coverage := SourceCoverage{}
	texts := []textItem{}
	if equityDir == "" {
		return texts, coverage
	}
	if newsTexts, count := loadNewsTexts(filepath.Join(equityDir, "news_sentiment.json")); count > 0 {
		coverage.NewsItemCount = count
		texts = append(texts, newsTexts...)
	}
	if !ohlcv.IsCryptoAssetType(assetType) {
		if kapTexts, count := loadKAPTexts(filepath.Join(equityDir, "kap_disclosures.json")); count > 0 {
			coverage.KAPDisclosureCount = count
			texts = append(texts, kapTexts...)
		}
	}
	if commentTexts, count := loadCommentTexts(filepath.Join(equityDir, "comments.json")); count > 0 {
		coverage.CommentCount = count
		coverage.HasCommentData = true
		texts = append(texts, commentTexts...)
	}
	texts = recentOrLatestTexts(texts, asOf, 120, 80)
	return texts, coverage
}

func loadNewsTexts(path string) ([]textItem, int) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0
	}
	var payload struct {
		Items []struct {
			Source      string    `json:"source"`
			Provider    string    `json:"provider"`
			Title       string    `json:"title"`
			Summary     string    `json:"summary"`
			PublishedAt time.Time `json:"published_at"`
			ReceivedAt  time.Time `json:"received_at"`
		} `json:"items"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return nil, 0
	}
	out := make([]textItem, 0, len(payload.Items))
	for _, row := range payload.Items {
		publishedAt := row.PublishedAt
		if publishedAt.IsZero() {
			publishedAt = row.ReceivedAt
		}
		out = append(out, textItem{
			source:      emptyString(row.Provider, row.Source),
			publishedAt: publishedAt,
			title:       row.Title,
			summary:     row.Summary,
		})
	}
	return out, len(payload.Items)
}

func loadKAPTexts(path string) ([]textItem, int) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return nil, 0
	}
	out := make([]textItem, 0, len(rows))
	for _, row := range rows {
		item := textItem{
			source:      "kap",
			publishedAt: timeValue(row["publish_date"]),
			title:       stringValue(row["title"]),
			summary:     stringValue(row["summary"]),
		}
		if item.title == "" && item.summary == "" {
			continue
		}
		out = append(out, item)
	}
	return out, len(rows)
}

func loadCommentTexts(path string) ([]textItem, int) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return nil, 0
	}
	out := make([]textItem, 0, len(rows))
	for _, row := range rows {
		text := firstString(row, "text", "comment", "body", "message", "summary", "content")
		if text == "" {
			continue
		}
		out = append(out, textItem{
			source:      "comment",
			publishedAt: firstTime(row, "published_at", "created_at", "date", "time"),
			title:       firstString(row, "title", "author"),
			summary:     text,
		})
	}
	return out, len(rows)
}

func recentOrLatestTexts(texts []textItem, asOf time.Time, recentDays int, limit int) []textItem {
	sort.SliceStable(texts, func(i, j int) bool {
		return texts[i].publishedAt.After(texts[j].publishedAt)
	})
	recent := []textItem{}
	cutoff := asOf.AddDate(0, 0, -recentDays)
	for _, item := range texts {
		if item.publishedAt.IsZero() || item.publishedAt.Before(cutoff) || item.publishedAt.After(asOf.Add(24*time.Hour)) {
			continue
		}
		recent = append(recent, item)
		if len(recent) >= limit {
			return recent
		}
	}
	if len(recent) > 0 {
		return recent
	}
	if len(texts) > limit {
		return texts[:limit]
	}
	return texts
}

func countRecentTexts(texts []textItem, asOf time.Time, days int) int {
	cutoff := asOf.AddDate(0, 0, -days)
	count := 0
	for _, item := range texts {
		if !item.publishedAt.IsZero() && !item.publishedAt.Before(cutoff) && !item.publishedAt.After(asOf.Add(24*time.Hour)) {
			count++
		}
	}
	return count
}

func analyzeSentiment(texts []textItem, assetType string) SentimentReport {
	if len(texts) == 0 {
		if ohlcv.IsCryptoAssetType(assetType) {
			return SentimentReport{Label: "veri_yok", PlainLanguage: "Kripto haber/sentiment metni bulunmadığı için söylem skoru üretilemedi."}
		}
		return SentimentReport{Label: "veri_yok", PlainLanguage: "KAP/yorum/haber metni bulunmadığı için söylem skoru üretilemedi."}
	}
	positiveWords := []string{"SÖZLEŞME", "SIPARIŞ", "SİPARİŞ", "İHALE", "KAZANDI", "ARTIŞ", "BÜYÜME", "KAR", "KÂR", "TEMETTÜ", "YATIRIM", "ONAY", "TESLİMAT"}
	negativeWords := []string{"ZARAR", "DÜŞÜŞ", "AZALIŞ", "İPTAL", "ERTELEN", "DAVA", "CEZA", "SORUŞTURMA", "RİSK", "OLUMSUZ", "KAYIP", "UYARI", "BORÇ"}
	panicWords := []string{"PANİK", "TABAN", "ÇÖKÜŞ", "SERT SATIŞ", "KAPİTÜLASYON", "KRİZ", "İFLAS", "KORKU", "YIKIM"}
	signals := []TextSignal{}
	totalScore := 0.0
	posHits, negHits, panicHits := 0, 0, 0
	for _, item := range texts {
		text := strings.ToUpper(item.title + " " + item.summary)
		pos := matchingWords(text, positiveWords)
		neg := matchingWords(text, negativeWords)
		panic := matchingWords(text, panicWords)
		posHits += len(pos)
		negHits += len(neg)
		panicHits += len(panic)
		score := float64(len(pos))*8 - float64(len(neg))*10 - float64(len(panic))*18
		totalScore += score
		keywords := append([]string{}, pos...)
		keywords = append(keywords, neg...)
		keywords = append(keywords, panic...)
		if len(keywords) > 0 {
			signals = append(signals, TextSignal{
				Source:      item.source,
				PublishedAt: item.publishedAt,
				Title:       item.title,
				Summary:     trimText(item.summary, 180),
				Tone:        toneFromScore(score),
				Score:       score,
				Keywords:    keywords,
			})
		}
	}
	score := mathutil.Clamp(totalScore/math.Sqrt(float64(len(texts))+1), -100, 100)
	negativity := mathutil.Clamp(float64(negHits)*10+float64(panicHits)*18-float64(posHits)*5, 0, 100)
	sort.SliceStable(signals, func(i, j int) bool {
		return math.Abs(signals[i].Score) > math.Abs(signals[j].Score)
	})
	if len(signals) > 8 {
		signals = signals[:8]
	}
	label := "nötr"
	switch {
	case score <= -25:
		label = "olumsuz"
	case score >= 25:
		label = "olumlu"
	}
	return SentimentReport{
		Score:         math.Round(score*10) / 10,
		Negativity:    math.Round(negativity*10) / 10,
		Label:         label,
		PositiveHits:  posHits,
		NegativeHits:  negHits,
		PanicHits:     panicHits,
		TopSignals:    signals,
		PlainLanguage: sentimentPlainLanguage(label, negativity, len(texts)),
	}
}

func analyzeCapitulation(input Input) CapitulationReport {
	candles := input.Candles
	if len(candles) == 0 || input.LastClose <= 0 {
		return CapitulationReport{Label: "veri_yok", PlainLanguage: "Fiyat verisi yetersiz olduğu için dip/kapitülasyon skoru üretilemedi."}
	}
	last := candles[len(candles)-1]
	prevClose := input.LastClose
	if len(candles) > 1 {
		prevClose = candles[len(candles)-2].EffectiveClose()
	}
	components := []ScoreComponent{}
	evidence := []string{}

	supportScore := 0.0
	if input.NearestSupport != nil && input.NearestSupport.Price > 0 && input.NearestSupport.Price < input.LastClose {
		distance := (input.LastClose - input.NearestSupport.Price) / input.LastClose
		switch {
		case distance <= 0.015:
			supportScore = 20
		case distance <= 0.035:
			supportScore = 15
		case distance <= 0.06:
			supportScore = 9
		}
		if supportScore > 0 {
			evidence = append(evidence, fmt.Sprintf("Fiyat en yakın desteğe %.1f%% mesafede.", distance*100))
		}
		components = append(components, ScoreComponent{"Desteğe yakınlık", supportScore, 20, fmt.Sprintf("Destek %.2f", input.NearestSupport.Price)})
	} else {
		components = append(components, ScoreComponent{"Desteğe yakınlık", 0, 20, "Yakın destek hesaplanamadı"})
	}

	high60 := rollingHigh(candles, 60)
	drawdownScore := 0.0
	if high60 > 0 {
		drawdown := input.LastClose/high60 - 1
		switch {
		case drawdown <= -0.25:
			drawdownScore = 20
		case drawdown <= -0.15:
			drawdownScore = 15
		case drawdown <= -0.08:
			drawdownScore = 8
		}
		if drawdownScore > 0 {
			evidence = append(evidence, fmt.Sprintf("Son 60 bar zirvesinden düşüş %.1f%%.", drawdown*100))
		}
		components = append(components, ScoreComponent{"Zirveden uzaklık", drawdownScore, 20, fmt.Sprintf("60 bar zirvesi %.2f", high60)})
	}

	oscScore := 0.0
	if input.Indicators.RSI14 > 0 {
		switch {
		case input.Indicators.RSI14 < 30:
			oscScore += 12
		case input.Indicators.RSI14 < 40:
			oscScore += 9
		case input.Indicators.RSI14 < 45:
			oscScore += 5
		}
	}
	if input.Indicators.MFI14 > 0 && input.Indicators.MFI14 < 35 {
		oscScore += 8
	}
	oscScore = mathutil.Clamp(oscScore, 0, 20)
	if oscScore > 0 {
		evidence = append(evidence, fmt.Sprintf("RSI %.1f ve MFI %.1f zayıf/soğuk bölge sinyali veriyor.", input.Indicators.RSI14, input.Indicators.MFI14))
	}
	components = append(components, ScoreComponent{"RSI/MFI soğuma", oscScore, 20, fmt.Sprintf("RSI %.1f | MFI %.1f", input.Indicators.RSI14, input.Indicators.MFI14)})

	volumeScore := 0.0
	volumeRatio := 0.0
	if input.Indicators.VolumeSMA20 > 0 {
		volumeRatio = last.EffectiveVolume() / input.Indicators.VolumeSMA20
		if last.EffectiveClose() < prevClose {
			switch {
			case volumeRatio >= 2:
				volumeScore = 15
			case volumeRatio >= 1.35:
				volumeScore = 10
			case volumeRatio >= 1.1:
				volumeScore = 5
			}
		}
	}
	if volumeScore > 0 {
		evidence = append(evidence, fmt.Sprintf("Düşüş gününde hacim 20 günlük ortalamanın %.1f katı.", volumeRatio))
	}
	components = append(components, ScoreComponent{"Hacimli satış", volumeScore, 15, fmt.Sprintf("Hacim/ort. %.2f", volumeRatio)})

	volatilityScore := 0.0
	atrPct := 0.0
	if input.Indicators.ATR14 > 0 {
		atrPct = input.Indicators.ATR14 / input.LastClose
		switch {
		case atrPct >= 0.06:
			volatilityScore = 10
		case atrPct >= 0.035:
			volatilityScore = 7
		case atrPct >= 0.02:
			volatilityScore = 4
		}
	}
	components = append(components, ScoreComponent{"Volatilite artışı", volatilityScore, 10, fmt.Sprintf("ATR%% %.1f", atrPct*100)})

	rangeScore := 0.0
	low60 := rollingLow(candles, 60)
	if low60 > 0 && input.LastClose <= low60*1.05 {
		rangeScore += 8
		evidence = append(evidence, "Fiyat son 60 bar dip bölgesine yakın.")
	}
	if input.Indicators.BollingerLower > 0 && input.Indicators.BollingerUpper > input.Indicators.BollingerLower {
		position := (input.LastClose - input.Indicators.BollingerLower) / (input.Indicators.BollingerUpper - input.Indicators.BollingerLower)
		if position <= 0.2 {
			rangeScore += 7
			evidence = append(evidence, "Fiyat Bollinger alt banda yakın.")
		}
	}
	rangeScore = mathutil.Clamp(rangeScore, 0, 15)
	components = append(components, ScoreComponent{"Dip bandı konumu", rangeScore, 15, fmt.Sprintf("60 bar dip %.2f", low60)})

	total := 0.0
	for _, component := range components {
		total += component.Score
	}
	label := "zayıf"
	switch {
	case total >= 70:
		label = "yüksek"
	case total >= 45:
		label = "orta"
	}
	return CapitulationReport{
		Score:         math.Round(total*10) / 10,
		Label:         label,
		Components:    components,
		Evidence:      evidence,
		PlainLanguage: capitulationPlainLanguage(label, total),
	}
}

func analyzeContrarian(sentiment SentimentReport, capitulation CapitulationReport, backtest BacktestReport, coverage SourceCoverage, input Input) ContrarianReport {
	score := capitulation.Score*0.62 + sentiment.Negativity*0.23
	if input.NearestSupport != nil && input.LastClose > input.NearestSupport.Price {
		score += 8
	}
	if input.Indicators.MACDHistogram > 0 && input.Indicators.ChaikinMoneyFlow20 > 0 {
		score += 7
	}
	if !coverage.HasCommentData || !coverage.HasRecentSentiment {
		score = math.Min(score, 68)
	}
	score = mathutil.Clamp(score, 0, 100)
	gate := qualityGate(coverage, backtest)
	label := "sinyal_yok"
	signal := "Dip/kapitülasyon sinyali zayıf"
	action := "İşlem kararı üretme"
	switch {
	case score >= 70:
		label = "tersine_dönüş_adayı"
		signal = "Dip/kapitülasyon sonrası tersine dönüş adayı"
		if input.Indicators.MACDHistogram > 0 && input.Indicators.ChaikinMoneyFlow20 > 0 && gate.CanAffectBuySignal {
			action = "Teyitli takip; aktif alım planı teknik stopla değerlendirilebilir"
		} else {
			action = "Teyit bekle; tek başına alım sinyali değildir"
		}
	case score >= 45:
		label = "dip_bölgesi_takip"
		signal = "Dip bölgesi izleme sinyali"
		action = "Teyit bekle"
	}
	evidence := []string{
		sentiment.PlainLanguage,
		capitulation.PlainLanguage,
		gate.Reason,
	}
	return ContrarianReport{
		Score:         math.Round(score*10) / 10,
		Label:         label,
		Signal:        signal,
		Action:        action,
		QualityGate:   gate,
		PlainLanguage: contrarianPlainLanguage(score, action, input),
		Evidence:      evidence,
	}
}

func qualityGate(coverage SourceCoverage, backtest BacktestReport) QualityGate {
	warnings := append([]string{}, coverage.Warnings...)
	if !backtest.Available {
		warnings = append(warnings, "capitulation_proxy_backtest_insufficient")
		return QualityGate{
			Status:              "fail",
			CanAffectBuySignal:  false,
			Reason:              "Dip/kapitülasyon sinyali için yeterli walk-forward örnek yok; karar sadece açıklayıcı bilgi olarak kullanılır.",
			Warnings:            warnings,
			RequiresWalkForward: true,
		}
	}
	if backtest.SampleSize < 20 || !coverage.HasCommentData || !coverage.HasRecentSentiment {
		return QualityGate{
			Status:              "limited",
			CanAffectBuySignal:  false,
			Reason:              "Sinyal proxy backtest içeriyor ancak haber/yorum sentiment kapsamı sınırlı; alım kararı üretmez, sadece takip uyarısıdır.",
			Warnings:            warnings,
			RequiresWalkForward: true,
		}
	}
	if backtest.WinRate < 0.50 {
		return QualityGate{
			Status:              "fail",
			CanAffectBuySignal:  false,
			Reason:              "Geçmiş benzer dip sinyallerinde başarı oranı zayıf; karar katmanına alım yönlü etki verilmez.",
			Warnings:            warnings,
			RequiresWalkForward: true,
		}
	}
	return QualityGate{
		Status:              "pass",
		CanAffectBuySignal:  true,
		Reason:              "Sentiment kapsamı ve proxy walk-forward örnekleri yeterli; sinyal karar katmanına sınırlı ağırlıkla girebilir.",
		Warnings:            warnings,
		RequiresWalkForward: true,
	}
}

func backtestCapitulationProxy(candles []ohlcv.Candle, forwardBars int) BacktestReport {
	if len(candles) < 90 {
		return BacktestReport{ProxyOnly: true, ForwardBars: forwardBars, InsufficientWhy: "en az 90 mum gerekli", PlainLanguage: "Dip sinyali için yeterli geçmiş mum yok."}
	}
	returns := []float64{}
	for i := 60; i+forwardBars < len(candles); i++ {
		closePrice := candles[i].EffectiveClose()
		if closePrice <= 0 {
			continue
		}
		high60 := rollingHighAt(candles, i, 60)
		low20 := rollingLowAt(candles, i, 20)
		rsi := rsiAt(candles, i, 14)
		volSMA := volumeSMAAt(candles, i, 20)
		volumeSpike := volSMA > 0 && candles[i].EffectiveVolume()/volSMA >= 1.25
		nearLow := low20 > 0 && closePrice <= low20*1.035
		drawdown := 0.0
		if high60 > 0 {
			drawdown = closePrice/high60 - 1
		}
		downDay := i > 0 && closePrice < candles[i-1].EffectiveClose()
		if nearLow && drawdown <= -0.08 && rsi > 0 && rsi < 44 && (volumeSpike || downDay) {
			future := candles[i+forwardBars].EffectiveClose()/closePrice - 1
			returns = append(returns, future)
		}
	}
	if len(returns) == 0 {
		return BacktestReport{ProxyOnly: true, ForwardBars: forwardBars, InsufficientWhy: "geçmişte benzer proxy sinyal bulunmadı", PlainLanguage: "Geçmiş mumlarda benzer dip sinyali örneği bulunamadı."}
	}
	wins := 0
	for _, ret := range returns {
		if ret > 0 {
			wins++
		}
	}
	avg := mathutil.Mean(returns)
	med := median(returns)
	return BacktestReport{
		Available:     true,
		ProxyOnly:     true,
		SampleSize:    len(returns),
		ForwardBars:   forwardBars,
		WinRate:       float64(wins) / float64(len(returns)),
		AverageReturn: avg,
		MedianReturn:  med,
		PlainLanguage: fmt.Sprintf("Geçmişte benzer proxy dip sinyali %d kez oluştu; %d bar sonra pozitif kapanış oranı %.1f%%.", len(returns), forwardBars, float64(wins)/float64(len(returns))*100),
	}
}

func matchingWords(text string, words []string) []string {
	matches := []string{}
	for _, word := range words {
		if strings.Contains(text, word) {
			matches = append(matches, word)
		}
	}
	return matches
}

func toneFromScore(score float64) string {
	switch {
	case score < 0:
		return "olumsuz"
	case score > 0:
		return "olumlu"
	default:
		return "nötr"
	}
}

func sentimentPlainLanguage(label string, negativity float64, count int) string {
	if label == "veri_yok" {
		return "Söylem/sentiment verisi yok."
	}
	return fmt.Sprintf("İncelenen %d metinde söylem %s; olumsuzluk skoru %.0f/100.", count, label, negativity)
}

func capitulationPlainLanguage(label string, score float64) string {
	switch label {
	case "yüksek":
		return fmt.Sprintf("Fiyat yapısı güçlü dip/kapitülasyon adayı görünümü veriyor; skor %.0f/100.", score)
	case "orta":
		return fmt.Sprintf("Fiyat yapısı dip bölgesi izleme sinyali veriyor; skor %.0f/100.", score)
	default:
		return fmt.Sprintf("Dip/kapitülasyon kanıtı zayıf; skor %.0f/100.", score)
	}
}

func contrarianPlainLanguage(score float64, action string, input Input) string {
	momentum := "momentum teyidi yok"
	if input.Indicators.MACDHistogram > 0 && input.Indicators.ChaikinMoneyFlow20 > 0 {
		momentum = "momentum ve para akışı teyidi var"
	}
	return fmt.Sprintf("Tersine dönüş skoru %.0f/100. %s; %s.", score, action, momentum)
}

func rollingHigh(candles []ohlcv.Candle, window int) float64 {
	return rollingHighAt(candles, len(candles)-1, window)
}

func rollingLow(candles []ohlcv.Candle, window int) float64 {
	return rollingLowAt(candles, len(candles)-1, window)
}

func rollingHighAt(candles []ohlcv.Candle, end int, window int) float64 {
	start := end - window + 1
	if start < 0 {
		start = 0
	}
	high := 0.0
	for i := start; i <= end && i < len(candles); i++ {
		value := candles[i].EffectiveHigh()
		if value <= 0 {
			value = candles[i].EffectiveClose()
		}
		if value > high {
			high = value
		}
	}
	return high
}

func rollingLowAt(candles []ohlcv.Candle, end int, window int) float64 {
	start := end - window + 1
	if start < 0 {
		start = 0
	}
	low := 0.0
	for i := start; i <= end && i < len(candles); i++ {
		value := candles[i].EffectiveLow()
		if value <= 0 {
			value = candles[i].EffectiveClose()
		}
		if low == 0 || value < low {
			low = value
		}
	}
	return low
}

func rsiAt(candles []ohlcv.Candle, end int, period int) float64 {
	if end-period < 0 {
		return 0
	}
	gain := 0.0
	loss := 0.0
	for i := end - period + 1; i <= end; i++ {
		diff := candles[i].EffectiveClose() - candles[i-1].EffectiveClose()
		if diff > 0 {
			gain += diff
		} else {
			loss -= diff
		}
	}
	if gain == 0 && loss == 0 {
		return 50
	}
	if loss == 0 {
		return 100
	}
	rs := gain / loss
	return 100 - 100/(1+rs)
}

func volumeSMAAt(candles []ohlcv.Candle, end int, window int) float64 {
	start := end - window + 1
	if start < 0 {
		start = 0
	}
	values := []float64{}
	for i := start; i <= end && i < len(candles); i++ {
		if value := candles[i].EffectiveVolume(); value > 0 {
			values = append(values, value)
		}
	}
	return mathutil.Mean(values)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64{}, values...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstTime(row map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		if value := timeValue(row[key]); !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func emptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func timeValue(value any) time.Time {
	text := stringValue(value)
	if text == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z07:00", "02.01.2006 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func trimText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}
