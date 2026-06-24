package storage

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hissebot/internal/kapingest"
	"hissebot/internal/ta/analysis"
	"hissebot/internal/ta/docintel"
	"hissebot/internal/ta/investorqa"
	"hissebot/internal/ta/localize"
	taml "hissebot/internal/ta/ml"
	"hissebot/internal/ta/ohlcv"
	"hissebot/internal/ta/professional"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type reportConfidenceItem struct {
	Label  string
	Score  float64
	Max    float64
	Status string
	Detail string
}

type reportConfidence struct {
	Score float64
	Items []reportConfidenceItem
}

type reportConfidenceGap struct {
	Area       string
	Current    string
	Target     string
	Impact     string
	Action     string
	MissingPts float64
}

type reportImage struct {
	Timeframe string
	Title     string
	Subtitle  string
	PNG       []byte
}

func professionalReportHTML(result analysis.SymbolAnalysis) string {
	confidence := reportConfidenceFor(result)
	pro := result.Professional
	keys := sortedTimeframeKeys(result.Timeframes)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"tr\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString(fmt.Sprintf("<title>%s Entegre Analiz Raporu</title>\n", html.EscapeString(result.Symbol)))
	b.WriteString(`<style>
:root{--bg:#f6f7f9;--ink:#1f2933;--muted:#667085;--line:#d0d5dd;--panel:#fff;--accent:#0f766e;--accent-dark:#134e4a;--accent-soft:#e6f4f1;--accent-pale:#f8fafc;--green:#13795b;--red:#b42318;--amber:#b54708;--blue:#175cd3}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--ink);font:13px/1.42 Arial,Helvetica,sans-serif}header{background:#fff;border-top:4px solid var(--accent);border-bottom:1px solid var(--line);color:var(--ink);padding:18px 32px 16px}header h1{margin:0 0 5px;font-size:24px;letter-spacing:0;color:var(--accent-dark)}header p{margin:0;color:var(--muted)}.wrap{max-width:1120px;margin:0 auto;padding:16px 20px 32px}.grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px}.card,.section{background:var(--panel);border:1px solid var(--line);border-radius:6px;box-shadow:0 1px 2px rgba(16,24,40,.04)}.card{padding:12px}.card .k{color:var(--muted);font-size:11px;text-transform:uppercase}.card .v{font-size:18px;font-weight:700;margin-top:5px}.section{padding:16px;margin-top:12px}h2{font-size:17px;margin:0 0 10px;color:var(--accent-dark)}h3{font-size:14px;margin:12px 0 6px;color:var(--accent-dark)}.badge{display:inline-block;border-radius:6px;padding:4px 8px;font-weight:700;font-size:12px}.good{background:#ecfdf3;color:var(--green)}.bad{background:#fef3f2;color:var(--red)}.warn{background:#fffaeb;color:var(--amber)}.info{background:#eff8ff;color:var(--blue)}.one-look{position:relative;padding-bottom:34px}.ol-head{display:flex;justify-content:space-between;gap:12px;align-items:flex-start}.ol-head p{margin:0;color:var(--muted)}.ol-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));border:1px solid var(--line);border-radius:6px;overflow:hidden}.ol-cell{padding:10px;border-right:1px solid var(--line);background:#fff}.ol-cell:last-child{border-right:0}.ol-cell .k{color:var(--muted);font-size:10px;text-transform:uppercase}.ol-cell .v{font-size:16px;font-weight:700;margin-top:4px;color:var(--ink)}.ol-actions{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin-top:10px}.ol-action{border:1px solid var(--line);border-left:4px solid var(--line);border-radius:6px;padding:10px;background:#fff}.ol-action.good{border-left-color:var(--green);background:#fff}.ol-action.warn{border-left-color:var(--amber);background:#fff}.ol-action.bad{border-left-color:var(--red);background:#fff}.ol-action .k{font-weight:700;margin-bottom:4px}.ol-cols{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin-top:10px}.ol-list{margin:0;padding-left:16px}.ol-disclaimer{position:absolute;right:16px;bottom:10px;color:var(--muted);font-size:11px;font-weight:700}table{width:100%;border-collapse:collapse}th,td{border-bottom:1px solid var(--line);padding:7px 8px;text-align:left;vertical-align:top}th{font-size:11px;color:var(--accent-dark);background:var(--accent-pale)}tr:nth-child(even) td{background:#fbfcfd}.num{text-align:right;white-space:nowrap}.muted{color:var(--muted)}ul{margin:6px 0 0 16px;padding:0}li{margin:3px 0}.note{border-left:3px solid var(--accent);padding:8px 10px;background:var(--accent-soft)}.risk{border-left-color:var(--red);background:#fef3f2}.small{font-size:11px}.two{display:grid;grid-template-columns:1fr 1fr;gap:10px}@media(max-width:760px){.ol-grid,.ol-actions,.ol-cols{grid-template-columns:1fr}.ol-cell{border-right:0;border-bottom:1px solid var(--line)}.ol-cell:last-child{border-bottom:0}.ol-head{display:block}.ol-disclaimer{position:static;margin-top:10px;text-align:right}}@media print{body{background:#fff}.wrap{max-width:none;padding:10mm}.section,.card{break-inside:avoid}.grid{grid-template-columns:repeat(4,1fr)}.ol-grid{grid-template-columns:repeat(4,1fr)}.ol-actions,.ol-cols{grid-template-columns:repeat(3,1fr)}}
</style></head><body>`)
	b.WriteString("<header><h1>")
	b.WriteString(html.EscapeString(result.Symbol))
	b.WriteString(" Entegre Analiz Raporu</h1><p>")
	b.WriteString(html.EscapeString(emptyFallback(result.CompanyName, entityNameLabel(result)+" yok")))
	b.WriteString(" | ")
	b.WriteString(html.EscapeString(analysisDateText(result.AnalysisDate)))
	b.WriteString(" | ")
	if isCryptoResult(result) {
		b.WriteString("Kripto fiyat verisi, teknik analiz, volatilite ve veri kapsamı birlikte değerlendirilmiştir. | E DENEYSELDİR</p></header><main class=\"wrap\">\n")
	} else if isCommodityResult(result) {
		b.WriteString("TradingView altın/emtia fiyat grafiği, teknik analiz, volatilite ve veri kapsamı birlikte değerlendirilmiştir. | E DENEYSELDİR</p></header><main class=\"wrap\">\n")
	} else {
		b.WriteString("Fiyat, teknik analiz, bilanço ve değerleme birlikte değerlendirilmiştir. | E DENEYSELDİR</p></header><main class=\"wrap\">\n")
	}

	b.WriteString("<div class=\"grid\">")
	b.WriteString(metricCardHTML("Karar", executiveDecision(result), executiveDecisionClass(result)))
	b.WriteString(metricCardHTML("Son Kapanış", reportPrice(primaryLastClose(result), result.Currency), "info"))
	b.WriteString(metricCardHTML("Entegre Skor", fmt.Sprintf("%.1f/100", result.OverallScore), scoreClass(result.OverallScore)))
	dataCoverage := reportDataCoverageScore(result)
	b.WriteString(metricCardHTML("Veri Kapsamı", fmt.Sprintf("%.0f/100", dataCoverage), scoreClass(dataCoverage)))
	b.WriteString(metricCardHTML("Karar / Model Güveni", fmt.Sprintf("%.0f/100", confidence.Score), scoreClass(confidence.Score)))
	if forecast, ok := symbolNextSessionForecast(result); ok {
		forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
		b.WriteString(metricCardHTML("Açılış Fiyat Senaryosu", nextSessionScenarioForecastCardText(forecast, "open", result.Currency), nextSessionPointForecastClass(forecast)))
		b.WriteString(metricCardHTML("Kapanış Fiyat Senaryosu", nextSessionScenarioForecastCardText(forecast, "close", result.Currency), nextSessionPointForecastClass(forecast)))
		b.WriteString(metricCardHTML("Ertesi Kapanış Yönü", nextSessionCloseDirectionCardText(forecast), nextSessionDirectionClass(forecast)))
		b.WriteString(metricCardHTML("Ertesi Seans Yönü", nextSessionDirectionDisplayText(forecast), nextSessionDirectionClass(forecast)))
	}
	b.WriteString("</div>\n")

	writeOneLookSummaryHTML(&b, result, confidence)
	writeNextSessionForecastHTML(&b, result)
	writeMLForecastHTML(&b, result)
	writeBISTBulletinContextHTML(&b, result)

	writeCentralClassificationHTML(&b, result)
	writeInstitutionalDecisionHTML(&b, result)
	writeRetailDecisionHTML(&b, result)
	writeDecisionSupportHTML(&b, result)
	if isEquityResult(result) {
		writeVAPContextHTML(&b, result)
		if isBankReport(result) {
			writeKAPAssetReferenceHTML(&b, result)
		}
		writeKAPAssetInventoryHTML(&b, result)
		writeInvestmentResearchHTML(&b, result)
		writeValueInvestingHTML(&b, result)
	}
	writeInstitutionalPersonaViewsHTML(&b, result)
	writeInvestorQAHTML(&b, result)

	b.WriteString("<section class=\"section\"><h2>Karar Özeti</h2><div class=\"note\"><strong>Önemli:</strong> Rapor güven skoru fiyat tahmini değildir; kullanılan verinin ve hesap tutarlılığının denetim skorudur.</div>")
	b.WriteString("<p>")
	b.WriteString(html.EscapeString(executiveDecisionText(result)))
	b.WriteString("</p>")
	b.WriteString("<div class=\"two\"><div><h3>Alım İçin Gerekli Teyitler</h3><ul>")
	for _, reason := range reportWaitReasons(result) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(reason))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div><div><h3>Başlıca Riskler</h3><ul>")
	for _, risk := range reportRiskReasons(result) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(risk))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div></div></section>\n")

	b.WriteString("<section class=\"section\"><h2>Rapor Güvenlik ve Doğrulama Kapısı</h2><table><tbody>")
	writeHTMLRow(&b, "Durum", institutionalStatusTR(result.InstitutionalValidation.Status))
	writeHTMLRow(&b, "Skor", fmt.Sprintf("%.0f/100", result.InstitutionalValidation.Score))
	writeHTMLRow(&b, "Özet", result.InstitutionalValidation.Summary)
	b.WriteString("</tbody></table><table><thead><tr><th>Kontrol</th><th>Durum</th><th>Açıklama</th></tr></thead><tbody>")
	for _, check := range result.InstitutionalValidation.Checks {
		b.WriteString("<tr><td>")
		b.WriteString(html.EscapeString(check.Name))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(institutionalStatusTR(check.Status)))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(check.Message))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table></section>\n")

	b.WriteString("<section class=\"section\"><h2>")
	b.WriteString(html.EscapeString(fundamentalSectionTitle(result)))
	b.WriteString("</h2><table><tbody>")
	for _, row := range fundamentalRows(result) {
		writeHTMLRow(&b, row[0], row[1])
	}
	b.WriteString("</tbody></table></section>\n")

	b.WriteString("<section class=\"section\"><h2>Dip/Kapitülasyon ve Haber Duygu Tonu</h2><table><tbody>")
	writeHTMLRow(&b, "Söylem tonu", behavioralSentimentText(result))
	writeHTMLRow(&b, "Dip/kapitülasyon skoru", fmt.Sprintf("%s | %.0f/100", result.Behavioral.Capitulation.Label, result.Behavioral.Capitulation.Score))
	writeHTMLRow(&b, "Tersine dönüş sinyali", fmt.Sprintf("%s | %.0f/100", result.Behavioral.Contrarian.Signal, result.Behavioral.Contrarian.Score))
	writeHTMLRow(&b, "Kalite kapısı", fmt.Sprintf("%s | %s", result.Behavioral.Contrarian.QualityGate.Status, result.Behavioral.Contrarian.QualityGate.Reason))
	writeHTMLRow(&b, "Sade yorum", behavioralPlainText(result))
	b.WriteString("</tbody></table></section>\n")

	b.WriteString("<section class=\"section\"><h2>Zaman Dilimi ve İşlem Kapısı</h2><table><thead><tr><th>Zaman</th><th>Son Bar</th><th>Son Kapanış</th><th>Mum</th><th>Teknik Görünüm</th><th>Teknik Skor</th><th>İşlem Kapısı</th><th>Sade Yorum</th></tr></thead><tbody>")
	for _, key := range keys {
		tf := result.Timeframes[key]
		b.WriteString("<tr><td>")
		b.WriteString(html.EscapeString(localize.Timeframe(key)))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(html.EscapeString(timeframeLastBarText(tf)))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(html.EscapeString(reportPriceValue(tf.LastClose)))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(html.EscapeString(timeframeCandleWindowText(tf)))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(timeframeTechnicalAppearanceForReport(result, tf)))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(fmt.Sprintf("%.1f", tf.Score))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(timeframeGateTextForReport(result, tf)))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(timeframePlainCommentForReport(result, tf)))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table></section>\n")

	b.WriteString("<section class=\"section\"><h2>")
	if isCryptoResult(result) {
		b.WriteString("Teknik Bant Senaryoları")
	} else if isCommodityResult(result) {
		b.WriteString("Altın/Emtia Teknik Senaryoları")
	} else {
		b.WriteString("Senaryolar")
	}
	b.WriteString("</h2>")
	if isEquityResult(result) && result.Professional.ValueInvesting.IntrinsicValue.Computed {
		b.WriteString("<div class=\"note\"><strong>Resmi senaryo seti:</strong> Bu bölüm değer yatırım içsel değer aralığını kullanır. Peer çarpanından türeyen makul değer ayrı kontrol girdisidir; tek başına hedef fiyat değildir.</div>")
	}
	b.WriteString("<table><thead><tr><th>Senaryo</th><th class=\"num\">Hedef</th><th class=\"num\">Getiri</th><th class=\"num\">Olasılık</th><th>Ne Anlama Gelir</th></tr></thead><tbody>")
	for _, scenario := range pro.Scenarios {
		b.WriteString("<tr><td>")
		b.WriteString(html.EscapeString(strings.ToUpper(scenario.Name)))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(fmt.Sprintf("%.2f", scenario.PriceTarget))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(formatPct(scenario.ReturnPct))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(formatPct(scenario.Probability * 100))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(reportText(strings.Join(scenario.Drivers, ", "))))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table></section>\n")

	b.WriteString("<section class=\"section\"><h2>Veriye Dayalı Okuma Notları</h2><ul>")
	for _, note := range reportReadingNotes(result) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(reportText(note)))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></section>\n")

	b.WriteString("<section class=\"section\"><h2>Terimler Basitçe Ne Demek?</h2><table><tbody>")
	writeHTMLRow(&b, "MACD negatif", "Kısa vadeli fiyat ivmesi zayıf demektir. Tek başına kesin satış değildir; alım için hızlanma teyidinin eksik olduğunu söyler.")
	writeHTMLRow(&b, "Chaikin para akışı negatif", "Fiyattaki yükseliş denemesi yeterli alıcı girişiyle desteklenmiyor olabilir.")
	writeHTMLRow(&b, "RSI yüksek", rsiTermText(result))
	writeHTMLRow(&b, "Risk/getiri", "Olası kazanç ile stopa kadar alınan riskin oranıdır. 1.50 altı zayıf kabul edilir.")
	b.WriteString("</tbody></table></section>\n")

	if isEquityResult(result) {
		writeSourceAppendixHTML(&b, result)
	}

	writeReportConfidenceGapHTML(&b, result, confidence)

	b.WriteString("<section class=\"section\"><h2>Doğrulama Skoru</h2><table><thead><tr><th>Kontrol</th><th class=\"num\">Puan</th><th>Durum</th><th>Açıklama</th></tr></thead><tbody>")
	for _, item := range confidence.Items {
		b.WriteString("<tr><td>")
		b.WriteString(html.EscapeString(item.Label))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(fmt.Sprintf("%.1f / %.0f", item.Score, item.Max))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(item.Status))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(item.Detail))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table><p class=\"small muted\">")
	b.WriteString(html.EscapeString(result.Disclaimer))
	b.WriteString("</p></section>\n")

	writeTechnicalAppendixHTML(&b, result, keys)

	b.WriteString("</main></body></html>\n")
	return b.String()
}

func writeMLForecastHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	report := result.MLForecast
	if !report.Enabled && report.Fallback.Reason == "" {
		return
	}
	b.WriteString("<section class=\"section\"><h2>ML Shadow Forecast ve Trade Gate</h2>")
	b.WriteString("<div class=\"note\"><strong>ML katmanı:</strong> Deterministik forecast kararını değiştirmeden gölge modda olasılık, aralık, veri kalitesi ve trade izni üretir; yatırım tavsiyesi değildir.</div>")
	b.WriteString("<table><tbody>")
	writeHTMLRow(b, "Durum", fmt.Sprintf("enabled=%t | shadow=%t | fallback=%t %s", report.Enabled, report.ShadowMode, report.Fallback.Used, emptyFallback(report.Fallback.Reason, "")))
	writeHTMLRow(b, "Model", strings.TrimSpace(report.ModelName+" "+report.ModelVersion))
	writeHTMLRow(b, "Feature set", report.FeatureSetVersion)
	writeHTMLRow(b, "Tahmini açılış / kapanış", fmt.Sprintf("%s / %s", reportPrice(report.PredictedOpen, result.Currency), reportPrice(report.PredictedClose, result.Currency)))
	writeHTMLRow(b, "Beklenen getiri", formatPct(report.ExpectedReturn*100))
	writeHTMLRow(b, "Yön olasılıkları", fmt.Sprintf("Yukarı %.0f%% / Yatay %.0f%% / Aşağı %.0f%%", report.DirectionProbabilities.Up*100, report.DirectionProbabilities.Flat*100, report.DirectionProbabilities.Down*100))
	writeHTMLRow(b, "Tahmin aralığı", fmt.Sprintf("%s - %s", reportPrice(report.PredictionInterval.Low, result.Currency), reportPrice(report.PredictionInterval.High, result.Currency)))
	writeHTMLRow(b, "Kalibre güven", fmt.Sprintf("%.0f%%", report.CalibratedConfidence*100))
	writeHTMLRow(b, "Rejim", mlRegimeText(report.Regime))
	writeHTMLRow(b, "Meta-label", mlMetaLabelText(report.MetaLabel))
	writeHTMLRow(b, "Trade gate", fmt.Sprintf("allowed=%t | action=%s | confidence=%s", report.TradeGate.Allowed, emptyFallback(report.TradeGate.Action, "no_trade"), emptyFallback(report.TradeGate.Confidence, "low")))
	if len(report.TradeGate.Reasons) > 0 {
		writeHTMLRow(b, "No-trade nedenleri", strings.Join(report.TradeGate.Reasons, ", "))
	}
	if len(report.TradeGate.RiskWarnings) > 0 {
		writeHTMLRow(b, "Risk uyarıları", strings.Join(report.TradeGate.RiskWarnings, ", "))
	}
	writeHTMLRow(b, "Veri kalitesi", fmt.Sprintf("missing %.0f%% | source %.0f%%", report.DataQuality.MissingRatio*100, report.DataQuality.SourceScore*100))
	if len(report.DataQuality.LeakageFlags) > 0 {
		writeHTMLRow(b, "Leakage flags", strings.Join(report.DataQuality.LeakageFlags, ", "))
	}
	if len(report.Warnings) > 0 {
		writeHTMLRow(b, "Uyarılar", strings.Join(report.Warnings, ", "))
	}
	b.WriteString("</tbody></table>")
	if len(report.ModelContributions) > 0 {
		writeHTMLTable(b, "Model Katkı Özeti", []string{"Model", "Ağırlık", "Yön", "Getiri", "Neden"}, mlContributionRows(report.ModelContributions, 6))
	}
	b.WriteString("</section>\n")
}

func mlRegimeText(regime taml.RegimeSummary) string {
	parts := []string{}
	if regime.VolatilityRegime != "" {
		parts = append(parts, regime.VolatilityRegime)
	}
	if regime.TrendRegime != "" {
		parts = append(parts, regime.TrendRegime)
	}
	if regime.MarketRegime != "" {
		parts = append(parts, regime.MarketRegime)
	}
	if regime.PositionScale > 0 {
		parts = append(parts, fmt.Sprintf("size %.0f%%", regime.PositionScale*100))
	}
	if len(parts) == 0 {
		return "rejim hesaplanmadı"
	}
	if len(regime.Reasons) > 0 {
		parts = append(parts, strings.Join(regime.Reasons, ", "))
	}
	return strings.Join(parts, " | ")
}

func mlMetaLabelText(meta taml.MetaLabelSummary) string {
	if meta.Method == "" && meta.Probability == 0 {
		return "meta-label hesaplanmadı"
	}
	return fmt.Sprintf("label=%d | allowed=%t | probability=%.0f%% | %s", meta.Label, meta.Allowed, meta.Probability*100, emptyFallback(meta.Reason, meta.Method))
}

func mlContributionRows(contributions []taml.ModelContribution, limit int) [][]string {
	if limit <= 0 || limit > len(contributions) {
		limit = len(contributions)
	}
	rows := make([][]string, 0, limit)
	for i := 0; i < limit; i++ {
		c := contributions[i]
		rows = append(rows, []string{
			emptyFallback(c.ModelName, "model"),
			fmt.Sprintf("%.0f%%", c.NormalizedWeight*100),
			emptyFallback(c.Direction, "flat"),
			formatPct(c.ExpectedReturn * 100),
			emptyFallback(c.ContributionReason, c.Family),
		})
	}
	return rows
}

func writeNextSessionForecastHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	forecast, ok := symbolNextSessionForecast(result)
	if !ok {
		return
	}
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	b.WriteString("<section class=\"section\"><h2>Bir Sonraki Seans Senaryo ve Risk Kapısı</h2><div class=\"note\"><strong>Hesaplanan fiyat/yön senaryosu:</strong> Açılış, kapanış, yön, bant ve olasılık değerleri model senaryosu olarak üretilir; karar/emir girdisi olarak ayrıca teknik kapıdan geçirilir.</div>")
	if !forecast.PointForecastPublishable {
		b.WriteString("<div class=\"note warn\"><strong>Senaryo kullanım durumu:</strong> ")
		b.WriteString(html.EscapeString(nextSessionScenarioUsageText(forecast)))
		b.WriteString("</div>")
	}
	b.WriteString("<table><tbody>")
	writeHTMLRow(b, "Tahmin edilen seans", emptyFallback(forecast.ForecastFor, "Takvim tarihi belirlenemedi"))
	writeHTMLRow(b, "Son kapanış", reportPrice(forecast.LastClose, result.Currency))
	if forecast.ActualAvailable {
		writeHTMLRow(b, "Kesinleşmiş resmi açılış", reportPrice(forecast.ActualOpen, result.Currency))
		writeHTMLRow(b, "Kesinleşmiş resmi kapanış", reportPrice(forecast.ActualClose, result.Currency))
		writeHTMLRow(b, "Kesinleşmiş resmi yön", fmt.Sprintf("%s / %s", nextSessionActualDirectionText(forecast.ActualOpen, forecast.LastClose), nextSessionActualDirectionText(forecast.ActualClose, forecast.LastClose)))
		writeHTMLRow(b, "Resmi sonuç kaynağı", emptyFallback(forecast.ActualSourcePath, forecast.ActualSource))
		writeHTMLRow(b, "Kesinleşmiş seans notu", "Karar fiyatı resmi BIST bültenidir; model nokta fiyatı ana raporda yayınlanmaz.")
	}
	writeHTMLRow(b, "Senaryo kullanım durumu", nextSessionScenarioUsageText(forecast))
	writeHTMLRow(b, "Karar sonucu", nextSessionDecisionOutcomeText(forecast))
	writeHTMLRow(b, "Açılış fiyat senaryosu", nextSessionScenarioForecastText(forecast, "open", result.Currency))
	writeHTMLRow(b, "Kapanış fiyat senaryosu", nextSessionScenarioForecastText(forecast, "close", result.Currency))
	writeHTMLRow(b, "Beklenen açılış yönü", nextSessionForecastDirectionText(forecast, forecast.PredictedOpenDirection, forecast.OpenChangePct))
	writeHTMLRow(b, "Beklenen kapanış yönü", nextSessionForecastDirectionText(forecast, forecast.PredictedCloseDirection, forecast.CloseChangePct))
	writeHTMLRow(b, "Beklenen gün içi bant", nextSessionExpectedBandText(forecast, result.Currency))
	writeHTMLRow(b, "Açılış dağılımı P10 / P50 / P90", nextSessionForecastDistributionTextForForecast(forecast, true, result.Currency))
	writeHTMLRow(b, "Kapanış dağılımı P10 / P50 / P90", nextSessionForecastDistributionTextForForecast(forecast, false, result.Currency))
	writeHTMLRow(b, "Yön olasılığı", nextSessionForecastProbabilityText(forecast))
	writeHTMLRow(b, "Invalidasyon seviyesi", nextSessionForecastInvalidationText(forecast, result.Currency))
	if forecast.ActualAvailable && forecast.RawExpectedLow > 0 && forecast.RawExpectedHigh > 0 && (forecast.RawExpectedLow != forecast.ExpectedLow || forecast.RawExpectedHigh != forecast.ExpectedHigh) {
		writeHTMLRow(b, "Ham beklenen bant", fmt.Sprintf("%s – %s", reportPrice(forecast.RawExpectedLow, result.Currency), reportPrice(forecast.RawExpectedHigh, result.Currency)))
	}
	if forecast.TickSize > 0 {
		writeHTMLRow(b, "Fiyat adımı", fmt.Sprintf("%s | %s | %s", reportPrice(forecast.TickSize, result.Currency), emptyFallback(forecast.RoundingMethod, "yuvarlama yok"), emptyFallback(forecast.PriceStepRule, "kural yok")))
	}
	if validation := nextSessionForecastValidationText(forecast); validation != "" {
		writeHTMLRow(b, "Doğrulama", validation)
	}
	writeHTMLRow(b, "Mikroyapı bağlamı", nextSessionForecastMicrostructureText(forecast))
	writeHTMLRow(b, "Yön / güç", nextSessionDirectionDisplayText(forecast))
	writeHTMLRow(b, "Model güveni", fmt.Sprintf("%.0f/100 | %s | %d tarihsel örnek", forecast.Confidence, emptyFallback(forecast.ConfidenceLabel, "etiket yok"), forecast.HistoricalSamples))
	writeHTMLRow(b, "Tahmin kalitesi", nextSessionForecastQualityText(forecast))
	writeHTMLRow(b, "Model", forecast.Model)
	if daily, ok := result.Timeframes["1D"]; ok {
		writeHTMLRow(b, "İşlem planı referansı", reportPlanTextForReport(result, daily))
		if daily.TradePlan.ConfidenceScore > 0 {
			writeHTMLRow(b, "Plan güveni", fmt.Sprintf("%.0f/100 | %s", tradePlanConfidencePct(daily.TradePlan.ConfidenceScore), localize.Quality(daily.TradePlan.Quality)))
		}
	}
	b.WriteString("</tbody></table><h3>Hesaplamayı Etkileyen Sinyaller</h3><ul>")
	for _, reason := range forecast.BiasReasons {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(reason))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></section>\n")
}

func nextSessionForecastValidationText(forecast analysis.NextSessionForecast) string {
	parts := []string{}
	if forecast.ValidationSource != "" {
		parts = append(parts, "kaynak: "+forecast.ValidationSource)
	}
	switch forecast.ValidationStatus {
	case "actual_session_observed":
		parts = append(parts, fmt.Sprintf("gerçekleşen açılış/kapanış: %s / %s", reportPriceValue(forecast.ActualOpen), reportPriceValue(forecast.ActualClose)))
	case "rolling_backtest_actual_session_not_observed":
		parts = append(parts, "resmi sonuç geldikten sonra gerçekleşen fiyatla audit edilecek")
	case "actual_session_not_observed":
		parts = append(parts, "resmi sonuç geldikten sonra gerçekleşen fiyatla audit edilecek")
	case "":
	default:
		parts = append(parts, forecast.ValidationStatus)
	}
	if forecast.BacktestSamples > 0 {
		parts = append(parts, fmt.Sprintf("geçmiş performans: %d örnek, açılış MAE %.2f%%, kapanış MAE %.2f%%, yön uyumu %.1f%%",
			forecast.BacktestSamples,
			forecast.BacktestOpenMAEPct,
			forecast.BacktestCloseMAEPct,
			forecast.BacktestDirectionHitRatePct,
		))
	}
	return strings.Join(parts, " | ")
}

func nextSessionActualDirectionText(price, lastClose float64) string {
	if price <= 0 || lastClose <= 0 {
		return "belirsiz"
	}
	diff := price - lastClose
	if math.Abs(diff) < 0.000001 {
		return "yatay"
	}
	if diff > 0 {
		return "yukarı"
	}
	return "aşağı"
}

func nextSessionDirectionDisplayText(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if forecast.ActualAvailable {
		return fmt.Sprintf("Resmi: %s / %s", nextSessionActualDirectionText(forecast.ActualOpen, forecast.LastClose), nextSessionActualDirectionText(forecast.ActualClose, forecast.LastClose))
	}
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionDirectionSuppressionText(forecast)
	}
	direction := strings.TrimSpace(forecast.DirectionBias)
	if direction == "" {
		direction = inferredNextSessionDirection(forecast)
	}
	strength := strings.TrimSpace(forecast.BiasStrength)
	if strength == "" {
		return nextSessionScenarioQualifier(forecast, direction)
	}
	return nextSessionScenarioQualifier(forecast, direction+" / "+strength)
}

func nextSessionCloseDirectionCardText(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if forecast.ActualAvailable {
		return "Resmi: " + nextSessionActualDirectionText(forecast.ActualClose, forecast.LastClose)
	}
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionDirectionSuppressionText(forecast)
	}
	direction := strings.TrimSpace(forecast.PredictedCloseDirection)
	if direction == "" {
		direction = inferredNextSessionDirection(forecast)
	}
	if forecast.CloseChangePct != 0 {
		return nextSessionScenarioQualifier(forecast, fmt.Sprintf("%s (%+.2f%%)", direction, forecast.CloseChangePct))
	}
	return nextSessionScenarioQualifier(forecast, direction)
}

func nextSessionForecastDirectionText(forecast analysis.NextSessionForecast, direction string, changePct float64) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if forecast.ActualAvailable {
		return fmt.Sprintf("Resmi gerçekleşen: %s", nextSessionActualDirectionText(forecast.ActualClose, forecast.LastClose))
	}
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionDirectionSuppressionText(forecast)
	}
	direction = strings.TrimSpace(direction)
	if direction == "" {
		direction = inferredNextSessionDirection(forecast)
	}
	return nextSessionScenarioQualifier(forecast, fmt.Sprintf("%s | son kapanışa göre %+.2f%%", direction, changePct))
}

func nextSessionDirectionPublishable(forecast analysis.NextSessionForecast) bool {
	return forecast.Computed && forecast.PointForecastPublishable
}

func nextSessionForwardScenarioPublishable(forecast analysis.NextSessionForecast) bool {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	return forecast.Computed && !forecast.ActualAvailable && forecast.PointForecastPublishable
}

func nextSessionDirectionSuppressionText(forecast analysis.NextSessionForecast) string {
	reason := strings.TrimSpace(forecast.PointForecastSuppressionReason)
	if reason == "" {
		reason = "forecast_not_publishable"
	}
	return "Yön yayınlanmadı: " + reportLabel(reason)
}

func nextSessionForwardScenarioSuppressionText(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	reason := strings.TrimSpace(forecast.PointForecastSuppressionReason)
	if reason == "" {
		reason = "forecast_not_publishable"
	}
	return "Yayınlanmadı: " + reportLabel(reason)
}

func nextSessionScenarioQualifier(forecast analysis.NextSessionForecast, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		text = "belirsiz"
	}
	if forecast.PointForecastPublishable {
		return text
	}
	return text + " | senaryo, karar/emir fiyatı değil"
}

func inferredNextSessionDirection(forecast analysis.NextSessionForecast) string {
	base := forecast.LastClose
	price := forecast.PredictedClose
	if forecast.TradablePredictedClose > 0 {
		price = forecast.TradablePredictedClose
	}
	if base <= 0 || price <= 0 {
		return "yön belirsiz"
	}
	changePct := (price/base - 1) * 100
	switch analysis.NextSessionDirectionFromReturn(changePct / 100) {
	case "up":
		return "yukarı"
	case "down":
		return "aşağı"
	default:
		return "yatay"
	}
}

func nextSessionDirectionClass(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if !nextSessionDirectionPublishable(forecast) {
		return "warn"
	}
	text := normalizeFinancialText(nextSessionDirectionDisplayText(forecast))
	switch {
	case strings.Contains(text, "asagi") || strings.Contains(text, "negatif") || strings.Contains(text, "dus"):
		return "bad"
	case strings.Contains(text, "yukari") || strings.Contains(text, "pozitif") || strings.Contains(text, "yuksel"):
		return "good"
	default:
		return "warn"
	}
}

func nextSessionPointForecastClass(forecast analysis.NextSessionForecast) string {
	if forecast.PointForecastPublishable {
		return "info"
	}
	return "warn"
}

func nextSessionPublishedForecastCardText(price *float64, currency string) string {
	if price == nil || *price <= 0 {
		return "Yayınlanmadı"
	}
	return reportPrice(*price, currency)
}

func nextSessionScenarioForecastCardText(forecast analysis.NextSessionForecast, field, currency string) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if forecast.ActualAvailable {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "open":
			return "Resmi: " + reportPrice(forecast.ActualOpen, currency)
		case "close":
			return "Resmi: " + reportPrice(forecast.ActualClose, currency)
		}
	}
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionForwardScenarioSuppressionText(forecast)
	}
	price := nextSessionScenarioPrice(forecast, field)
	if price <= 0 {
		return "Senaryo yok"
	}
	text := reportPrice(price, currency)
	if !forecast.PointForecastPublishable {
		text += " senaryo"
	}
	return text
}

func nextSessionScenarioForecastText(forecast analysis.NextSessionForecast, field, currency string) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if forecast.ActualAvailable {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "open":
			return "Resmi gerçekleşen: " + reportPrice(forecast.ActualOpen, currency)
		case "close":
			return "Resmi gerçekleşen: " + reportPrice(forecast.ActualClose, currency)
		}
	}
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionForwardScenarioSuppressionText(forecast)
	}
	price := nextSessionScenarioPrice(forecast, field)
	if price <= 0 {
		return "Senaryo fiyatı üretilemedi"
	}
	label := "model senaryosu"
	if forecast.PointForecastPublishable {
		label = "yayınlanabilir nokta tahmin"
	}
	return forecastPriceWithChangeText(price, forecast.LastClose, currency, label)
}

func nextSessionScenarioPrice(forecast analysis.NextSessionForecast, field string) float64 {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if forecast.ActualAvailable {
		return 0
	}
	if !forecast.PointForecastPublishable {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "open":
		if forecast.PublishedPredictedOpen != nil && *forecast.PublishedPredictedOpen > 0 {
			return *forecast.PublishedPredictedOpen
		}
		if forecast.TradablePredictedOpen > 0 {
			return forecast.TradablePredictedOpen
		}
		if forecast.PredictedOpen > 0 {
			return forecast.PredictedOpen
		}
		return forecast.OpenP50
	case "close":
		if forecast.PublishedPredictedClose != nil && *forecast.PublishedPredictedClose > 0 {
			return *forecast.PublishedPredictedClose
		}
		if forecast.TradablePredictedClose > 0 {
			return forecast.TradablePredictedClose
		}
		if forecast.PredictedClose > 0 {
			return forecast.PredictedClose
		}
		return forecast.CloseP50
	default:
		return 0
	}
}

func nextSessionPublishedForecastText(price *float64, lastClose float64, currency string) string {
	if price == nil || *price <= 0 {
		return "Yayınlanmadı; beklenen bant ve dağılım senaryosu kullanılmalı"
	}
	return forecastPriceWithChangeText(*price, lastClose, currency, "yayınlanabilir nokta tahmin")
}

func nextSessionExpectedBandText(forecast analysis.NextSessionForecast, currency string) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionForwardScenarioSuppressionText(forecast)
	}
	if forecast.ExpectedLow <= 0 || forecast.ExpectedHigh <= 0 {
		return "Beklenen bant üretilemedi"
	}
	return fmt.Sprintf("%s – %s", reportPrice(forecast.ExpectedLow, currency), reportPrice(forecast.ExpectedHigh, currency))
}

func nextSessionForecastDistributionTextForForecast(forecast analysis.NextSessionForecast, openDistribution bool, currency string) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionForwardScenarioSuppressionText(forecast)
	}
	if openDistribution {
		return nextSessionForecastDistributionText(forecast.OpenP10, forecast.OpenP50, forecast.OpenP90, currency)
	}
	return nextSessionForecastDistributionText(forecast.CloseP10, forecast.CloseP50, forecast.CloseP90, currency)
}

func nextSessionForecastDistributionText(p10, p50, p90 float64, currency string) string {
	if p10 <= 0 || p50 <= 0 || p90 <= 0 {
		return "Dağılım üretilemedi; beklenen bant senaryosu kullanılmalı"
	}
	return fmt.Sprintf("%s / %s / %s", reportPrice(p10, currency), reportPrice(p50, currency), reportPrice(p90, currency))
}

func nextSessionForecastProbabilityText(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if !nextSessionForwardScenarioPublishable(forecast) {
		return "Olasılık dağılımı yayınlanmadı: " + strings.TrimPrefix(nextSessionForwardScenarioSuppressionText(forecast), "Yayınlanmadı: ")
	}
	if forecast.UpsideProbabilityPct == 0 && forecast.FlatProbabilityPct == 0 && forecast.DownsideProbabilityPct == 0 {
		return "Olasılık dağılımı üretilemedi"
	}
	text := fmt.Sprintf("Yukarı %.0f%% / Yatay %.0f%% / Aşağı %.0f%% | %d örnek",
		forecast.UpsideProbabilityPct,
		forecast.FlatProbabilityPct,
		forecast.DownsideProbabilityPct,
		forecast.ForecastDistributionSamples,
	)
	if !forecast.PointForecastPublishable {
		text += " | senaryo, karar/emir girdisi değil"
	}
	return text
}

func nextSessionForecastInvalidationText(forecast analysis.NextSessionForecast, currency string) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if !nextSessionForwardScenarioPublishable(forecast) {
		return nextSessionForwardScenarioSuppressionText(forecast)
	}
	if forecast.InvalidationLevel <= 0 {
		return "Net invalidasyon seviyesi üretilemedi; işlem planındaki stop seviyesi esas alınmalı"
	}
	reason := reportLabel(forecast.InvalidationReason)
	if reason == "Yok" {
		reason = "senaryo geçersizleşme seviyesi"
	}
	return fmt.Sprintf("%s | %s", reportPrice(forecast.InvalidationLevel, currency), reason)
}

func nextSessionPointForecastPublishText(forecast analysis.NextSessionForecast) string {
	if forecast.PointForecastPublishable {
		return "Yayınlandı"
	}
	reason := strings.TrimSpace(forecast.PointForecastSuppressionReason)
	if reason == "" {
		reason = "forecast_not_publishable"
	}
	status := strings.TrimSpace(forecast.PointForecastStatus)
	if status == "" {
		status = "not_published"
	}
	return "Yayınlanmadı (" + status + "): " + reportLabel(reason)
}

func nextSessionScenarioUsageText(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if !forecast.Computed {
		return "Fiyat/yön senaryosu üretilemedi"
	}
	if forecast.ActualAvailable {
		return "Resmi sonuç var; model senaryosu audit için saklanır, ana raporda resmi fiyat kullanılır"
	}
	if forecast.PointForecastPublishable {
		return "Fiyat/yön senaryosu karar kalitesi kapısından geçti"
	}
	return "Fiyat/yön senaryosu yayınlanmadı: " + reportLabel(forecast.PointForecastSuppressionReason)
}

func nextSessionDecisionOutcomeText(forecast analysis.NextSessionForecast) string {
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	if !forecast.Computed {
		return "Karar yok: fiyat/yön senaryosu üretilemedi"
	}
	if forecast.ActualAvailable {
		if forecast.CloseDirectionHit != nil && *forecast.CloseDirectionHit {
			return "Gerçekleşen seans: resmi sonuç kullanılır; model kapanış yönü resmi sonuçla uyumlu"
		}
		if forecast.CloseDirectionHit != nil {
			return "Gerçekleşen seans: resmi sonuç kullanılır; model kapanış yönü resmi sonuçla uyumsuz"
		}
		return "Gerçekleşen seans: resmi fiyat kullanılır; model yalnız audit içindir"
	}
	if forecast.PointForecastPublishable {
		return "Karar/emir kapısı açık: forecast karar kalitesi eşiğini geçti"
	}
	return "Karar/emir kapısı kapalı: forecast fiyat/yön sayıları yayınlanmadı"
}

func nextSessionForecastQualityText(forecast analysis.NextSessionForecast) string {
	quality := reportLabel(forecast.Quality)
	if quality == "Yok" {
		quality = "Belirtilmedi"
	}
	status := reportLabel(forecast.Status)
	if status == "Yok" {
		status = "Durum yok"
	}
	return quality + " | " + status
}

func forecastPriceWithChangeText(price, lastClose float64, currency, note string) string {
	text := reportPrice(price, currency)
	if lastClose > 0 && price > 0 {
		text += fmt.Sprintf(" | son kapanışa göre %+.2f%%", forecastChangePct(price, lastClose))
	}
	if strings.TrimSpace(note) != "" {
		text += " | " + note
	}
	return text
}

func forecastChangePct(price, lastClose float64) float64 {
	if price <= 0 || lastClose <= 0 {
		return 0
	}
	return math.Round(10000*(price/lastClose-1)) / 100
}

func firstPositiveReportFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func nextSessionForecastMicrostructureText(forecast analysis.NextSessionForecast) string {
	parts := []string{}
	if forecast.OpeningAuctionEquilibriumPrice != nil {
		parts = append(parts, "açılış eşleşme "+reportPriceValue(*forecast.OpeningAuctionEquilibriumPrice))
	}
	if forecast.OrderBookImbalance != nil {
		parts = append(parts, fmt.Sprintf("order book dengesizliği %.2f", *forecast.OrderBookImbalance))
	}
	if forecast.AuctionVolumePressure != nil {
		parts = append(parts, fmt.Sprintf("ihale hacim baskısı %.2f", *forecast.AuctionVolumePressure))
	}
	if forecast.MicrostructureAdjustment != nil {
		parts = append(parts, fmt.Sprintf("mikroyapı düzeltmesi %.2f", *forecast.MicrostructureAdjustment))
	}
	if len(parts) == 0 {
		return "sayısal açılış ihalesi/order book düzeltmesi yok; mikroyapı verisi rapor bağlamında izlenir"
	}
	return strings.Join(parts, " | ")
}

func writeBISTBulletinContextHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	context := result.BISTBulletin
	if !context.Computed && len(context.Warnings) == 0 {
		return
	}
	b.WriteString("<section class=\"section\"><h2>BIST Resmi Bülten Doğrulaması</h2><table><tbody>")
	writeHTMLRow(b, "Durum", yesNoReport(context.Computed))
	writeHTMLRow(b, "Kaynak", emptyFallback(context.Source, "BIST THB resmi bülten"))
	writeHTMLRow(b, "Kapsam", fmt.Sprintf("%s - %s | %d kayıt", emptyFallback(context.CoverageStart, "başlangıç yok"), emptyFallback(context.CoverageEnd, "bitiş yok"), context.RecordCount))
	if context.LatestRecord.TradingDate != "" {
		writeHTMLRow(b, "Son resmi seans", fmt.Sprintf("%s | Açılış %s | Kapanış %s | AOF %s",
			context.LatestRecord.TradingDate,
			reportPrice(context.LatestRecord.Open, result.Currency),
			reportPrice(context.LatestRecord.Close, result.Currency),
			reportPrice(context.LatestRecord.VWAP, result.Currency),
		))
		writeHTMLRow(b, "Son seans mikroyapı", fmt.Sprintf("bekleyen alış/satış %s / %s | spread %.2f bps",
			reportPrice(context.LatestRecord.RemainingBid, result.Currency),
			reportPrice(context.LatestRecord.RemainingAsk, result.Currency),
			context.LatestObservedSpreadBps,
		))
		writeHTMLRow(b, "Seans hacimleri", fmt.Sprintf("açılış %.0f adet | kapanış %.0f adet | toplam %.0f adet",
			context.LatestOpeningSessionVolume,
			context.LatestClosingSessionVolume,
			context.LatestRecord.Volume,
		))
	}
	if context.OfficialCloseConfirmed {
		writeHTMLRow(b, "Resmi kapanış mutabakatı", fmt.Sprintf("mutabık | fark %+.2f%%", context.OfficialCloseDeltaPct))
	} else if context.OfficialCloseDeltaPct != 0 {
		writeHTMLRow(b, "Resmi kapanış mutabakatı", fmt.Sprintf("fark %+.2f%%", context.OfficialCloseDeltaPct))
	}
	if context.ForecastActualAvailable {
		writeHTMLRow(b, "Tahmin seansı gerçekleşeni", fmt.Sprintf("%s | açılış %s | kapanış %s",
			context.ForecastActualRecord.TradingDate,
			reportPrice(context.ForecastActualRecord.Open, result.Currency),
			reportPrice(context.ForecastActualRecord.Close, result.Currency),
		))
	} else {
		writeHTMLRow(b, "Tahmin seansı gerçekleşeni", "Resmi sonuç geldikten sonra model senaryosuyla karşılaştırılacak")
	}
	for _, warning := range context.Warnings {
		writeHTMLRow(b, "Uyarı", warning)
	}
	b.WriteString("</tbody></table></section>\n")
}

func writeVAPContextHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	freeFloat := result.Professional.VAPFreeFloat
	portfolio := result.Professional.VAPIndexPortfolio
	b.WriteString("<section class=\"section\"><h2>VAP / MKK Piyasa Yapısı</h2><div class=\"two\"><div><h3>Fiili Dolaşım</h3><table><tbody>")
	writeHTMLRow(b, "Durum", yesNoReport(freeFloat.Computed))
	writeHTMLRow(b, "Son tarih", emptyFallback(freeFloat.LatestDate, "Veri yok"))
	writeHTMLRow(b, "Fiili dolaşım oranı", fmt.Sprintf("%.2f%%", freeFloat.FreeFloatRatioPct))
	writeHTMLRow(b, "20 gözlem değişimi", fmt.Sprintf("%+.2f puan", freeFloat.RatioChange20DPP))
	writeHTMLRow(b, "Likidite riski", emptyFallback(freeFloat.LiquidityRisk, "Bilinmiyor"))
	writeHTMLRow(b, "Arz sinyali", emptyFallback(freeFloat.SupplySignal, "Bilinmiyor"))
	writeHTMLRow(b, "Özet", freeFloat.Summary)
	b.WriteString("</tbody></table></div><div><h3>BIST Endeks Portföyü</h3><table><tbody>")
	writeHTMLRow(b, "Durum", yesNoReport(portfolio.Computed))
	writeHTMLRow(b, "Seçilen endeks", emptyFallback(portfolio.SelectedIndex, "Veri yok"))
	writeHTMLRow(b, "Son dönem", emptyFallback(portfolio.LatestMonth, "Veri yok"))
	writeHTMLRow(b, "Portföy değeri", fmt.Sprintf("%.2f milyon TL", portfolio.PortfolioValueMTL))
	writeHTMLRow(b, "Aylık değişim", fmt.Sprintf("%+.2f%%", portfolio.Change1MPct))
	writeHTMLRow(b, "BIST 100'e göre fark", fmt.Sprintf("%+.2f puan", portfolio.RelativeMomentum))
	writeHTMLRow(b, "Sinyal", emptyFallback(portfolio.Signal, "Bilinmiyor"))
	writeHTMLRow(b, "Özet", portfolio.Summary)
	b.WriteString("</tbody></table></div></div></section>\n")
}

func yesNoReport(value bool) string {
	if value {
		return "Mevcut"
	}
	return "Eksik"
}

func writeCentralClassificationHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	c := result.DecisionClassification
	if c.SchemaVersion == 0 {
		return
	}
	b.WriteString("<section class=\"section\"><h2>Merkezi Karar Sınıflandırması</h2><div class=\"note\"><strong>")
	b.WriteString(html.EscapeString(reportLabel(c.Status)))
	b.WriteString(":</strong> ")
	b.WriteString(html.EscapeString(reportText(c.Summary)))
	b.WriteString("</div>")
	classes := []analysis.DecisionClassResult{
		c.Classes.LargeInvestor,
		c.Classes.RetailDirect,
		c.Classes.ValueInvesting,
		c.Classes.TradingEdge,
		c.Classes.InstitutionalPortfolio,
		c.Classes.AutomaticOrder,
		c.Classes.ResearchReport,
	}
	classRows := [][]string{}
	for _, class := range classes {
		classRows = append(classRows, []string{
			reportLabel(class.Key),
			institutionalStatusTR(class.Status),
			boolTRForReport(class.Qualified),
			reportLabel(class.Decision),
			strings.Join(nonEmptyReportStrings(class.FailedGateExplanations, class.FailedGateLabels), " | "),
		})
	}
	writeHTMLTable(b, "Tek Kaynaktan Üretilen Rapor Sınıfları", []string{"Sınıf", "Durum", "Geçti mi?", "Karar", "Geçmeyen kontroller"}, classRows)
	gateRows := [][]string{}
	for _, gate := range c.Gates {
		gateRows = append(gateRows, []string{
			emptyFallback(gate.Label, reportLabel(gate.Key)),
			institutionalStatusTR(gate.Status),
			boolTRForReport(gate.Passed),
			fmt.Sprintf("%.0f/100", gate.Score),
			strings.TrimSpace(gate.Reason + " " + gate.Explanation),
		})
	}
	writeHTMLTable(b, "Ortak Kontrol Sonuçları", []string{"Kontrol", "Durum", "Geçti mi?", "Skor", "Ne anlama geliyor?"}, gateRows)
	writeHTMLTable(b, "Model Tutarlılığı", []string{"Alan", "Sonuç"}, [][]string{
		{"Değerleme yayımlanabilir mi?", boolTRForReport(c.ValuationConsistency.Publishable)},
		{"Model ayrışması", fmt.Sprintf("%.1f%% / eşik %.1f%%", c.ValuationConsistency.MaxDivergencePct, c.ValuationConsistency.ThresholdPct)},
		{"Sektör modeli", c.SectorModelAlignment.Reason},
		{"Etkin model güven skoru", fmt.Sprintf("%.0f/100", c.EffectiveModelRisk)},
	})
	b.WriteString("</section>\n")
}

func nonEmptyReportStrings(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func professionalReportPDF(result analysis.SymbolAnalysis, images []reportImage) ([]byte, error) {
	pages, err := renderReportPages(result)
	if err != nil {
		return nil, err
	}
	pages = append(pages, renderReportImagePages(result, images)...)
	return buildImagePDF(pages)
}

func metricCardHTML(label, value, class string) string {
	return fmt.Sprintf("<section class=\"card\"><div class=\"k\">%s</div><div class=\"v\"><span class=\"badge %s\">%s</span></div></section>", html.EscapeString(label), html.EscapeString(class), html.EscapeString(value))
}

func oneLookSummaryPNG(result analysis.SymbolAnalysis) ([]byte, error) {
	fonts, err := loadReportFonts()
	if err != nil {
		return nil, err
	}
	img := renderOneLookSummaryImage(result, reportConfidenceFor(result), fonts)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeHTMLRow(b *strings.Builder, label, value string) {
	b.WriteString("<tr><th>")
	b.WriteString(html.EscapeString(label))
	b.WriteString("</th><td>")
	b.WriteString(html.EscapeString(reportText(emptyFallback(value, "Yok"))))
	b.WriteString("</td></tr>")
}

func writeOneLookSummaryHTML(b *strings.Builder, result analysis.SymbolAnalysis, confidence reportConfidence) {
	expectation, expectationClass, expectationText := oneLookExpectation(result)
	b.WriteString("<section class=\"section one-look\"><div class=\"ol-head\"><div><h2>Tek Bakış Karar Paneli</h2><p>")
	b.WriteString(html.EscapeString(reportText(oneLookPlainDecision(result))))
	b.WriteString("</p></div><span class=\"badge ")
	b.WriteString(html.EscapeString(expectationClass))
	b.WriteString("\">")
	b.WriteString(html.EscapeString(expectation))
	b.WriteString("</span></div>")

	b.WriteString("<div class=\"ol-grid\">")
	writeOneLookCellHTML(b, "Karar", executiveDecision(result))
	writeOneLookCellHTML(b, "Beklenti", expectationText)
	writeOneLookCellHTML(b, "Son kapanış", reportPrice(primaryLastClose(result), result.Currency))
	writeOneLookCellHTML(b, "Veri kapsamı", fmt.Sprintf("%.0f/100", reportDataCoverageScore(result)))
	if forecast, ok := symbolNextSessionForecast(result); ok {
		writeOneLookCellHTML(b, "Ertesi seans yönü", nextSessionDirectionDisplayText(forecast))
	}
	b.WriteString("</div>")

	b.WriteString("<div class=\"ol-actions\">")
	for _, card := range oneLookActionCards(result) {
		className := card.Class
		if className == "" {
			className = "info"
		}
		b.WriteString("<div class=\"ol-action ")
		b.WriteString(html.EscapeString(className))
		b.WriteString("\"><div class=\"k\">")
		b.WriteString(html.EscapeString(card.Label))
		b.WriteString("</div><div>")
		b.WriteString(html.EscapeString(reportText(card.Value)))
		b.WriteString("</div></div>")
	}
	b.WriteString("</div>")

	b.WriteString("<div class=\"ol-cols\">")
	writeOneLookListHTML(b, "Destek / Direnç", oneLookLevelLines(result), 4)
	writeOneLookListHTML(b, "Yön Beklentisi", oneLookDirectionLines(result), 4)
	writeOneLookListHTML(b, "Karar Verisi / Sınırlamalar", oneLookDataLines(result), 4)
	b.WriteString("</div><div class=\"ol-disclaimer\">Yatırım tavsiyesi değildir</div></section>\n")
}

func writeOneLookCellHTML(b *strings.Builder, label, value string) {
	b.WriteString("<div class=\"ol-cell\"><div class=\"k\">")
	b.WriteString(html.EscapeString(label))
	b.WriteString("</div><div class=\"v\">")
	b.WriteString(html.EscapeString(reportText(emptyFallback(value, "Yok"))))
	b.WriteString("</div></div>")
}

func writeOneLookListHTML(b *strings.Builder, title string, lines []string, limit int) {
	b.WriteString("<div><h3>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h3><ul class=\"ol-list\">")
	for _, line := range limitStrings(lines, limit) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(reportText(line)))
		b.WriteString("</li>")
	}
	if len(lines) == 0 {
		b.WriteString("<li>Bu bölüm için öne çıkan veri yok.</li>")
	}
	b.WriteString("</ul></div>")
}

func writeRetailDecisionHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	b.WriteString("<section class=\"section\"><h2>Küçük Yatırımcı Doğrudan AL/SAT Kararı</h2>")
	b.WriteString("<div class=\"note\"><strong>")
	if result.DecisionSupport != nil && result.DecisionSupport.Retail.Signal != "" {
		b.WriteString(html.EscapeString(result.DecisionSupport.Retail.Signal))
		b.WriteString(":</strong> ")
		b.WriteString(html.EscapeString(reportText(clarifyDecisionConfidenceText(result.DecisionSupport.Retail.OneLineAnswer))))
	} else if result.InvestorQA.Computed && result.InvestorQA.OneLineAnswer != "" {
		b.WriteString(html.EscapeString(retailDecisionLabel(result)))
		b.WriteString(":</strong> ")
		b.WriteString(html.EscapeString(retailText(result.InvestorQA.OneLineAnswer)))
	} else {
		b.WriteString(html.EscapeString(retailDecisionLabel(result)))
		b.WriteString(":</strong> ")
		b.WriteString(html.EscapeString(retailDecisionSentence(result)))
	}
	b.WriteString("</div>")
	b.WriteString("<div class=\"two\"><div><h3>Ne Yapmalı?</h3><ul>")
	for _, line := range retailActionLines(result) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(line))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div><div><h3>Önemli Seviyeler</h3><ul>")
	for _, line := range retailLevelLines(result) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(line))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div></div>")
	b.WriteString("<div class=\"two\"><div><h3>Neden?</h3><ul>")
	for _, line := range limitStrings(retailReasonLines(result), 5) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(line))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div><div><h3>Karar Ne Zaman Değişir?</h3><ul>")
	for _, line := range retailDecisionChangeLines(result) {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(line))
		b.WriteString("</li>")
	}
	b.WriteString("</ul></div></div>")
	b.WriteString("<p class=\"small muted\">Veri kapsamı ")
	b.WriteString(html.EscapeString(fmt.Sprintf("%.0f/100", reportDataCoverageScore(result))))
	b.WriteString("; karar/model güveni ")
	reportConfidence := reportConfidenceFor(result)
	b.WriteString(html.EscapeString(fmt.Sprintf("%.0f/100", reportConfidence.Score)))
	b.WriteString(". Bu değerler fiyat garantisi değildir; veri kapsamı kaynak varlığını, karar/model güveni ise yatırım sinyali ve model tutarlılığını gösterir. AL/SAT veya canlı işlem uygunluğu ayrı aksiyon kapılarında değerlendirilir.</p>")
	b.WriteString("</section>\n")
}

func writeInstitutionalDecisionHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	if result.DecisionSupport == nil || result.DecisionSupport.Institutional.Decision == "" {
		return
	}
	decision := result.DecisionSupport.Institutional
	b.WriteString("<section class=\"section\"><h2>Büyük Yatırımcı Karar Raporu</h2><div class=\"note\"><strong>")
	b.WriteString(html.EscapeString(reportLabel(decision.Decision)))
	b.WriteString(":</strong> ")
	b.WriteString(html.EscapeString(reportText(clarifyInstitutionalDecisionConfidenceText(decision.OneLineAnswer))))
	b.WriteString("</div><table><tbody>")
	writeHTMLRow(b, "Karar", reportLabel(decision.Decision))
	writeHTMLRow(b, "Pozisyon aksiyonu", reportLabel(decision.PositionAction))
	writeHTMLRow(b, "Pozisyon açılabilir mi?", boolTRForReport(decision.CanOpenPosition))
	writeHTMLRow(b, "Skor / güven", fmt.Sprintf("%.0f/100 | %.0f/100", decision.Score, decision.Confidence))
	writeHTMLRow(b, "Karar gerekçeleri", strings.Join(decision.DecisionReasons, "; "))
	writeHTMLRow(b, "Kararı sınırlayan riskler", strings.Join(decision.BlockingReasons, "; "))
	writeHTMLRow(b, "Onay koşulları", strings.Join(decision.ApprovalConditions, "; "))
	b.WriteString("</tbody></table></section>\n")
}

func writeDecisionSupportHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	support := result.DecisionSupport
	if support == nil {
		return
	}
	b.WriteString("<section class=\"section\"><h2>Karar Destek Kullanım Standardı</h2>")
	b.WriteString("<div class=\"note\"><strong>")
	b.WriteString(html.EscapeString(reportLabel(support.Status)))
	b.WriteString(":</strong> ")
	b.WriteString(html.EscapeString(reportText(support.Summary)))
	b.WriteString("</div>")
	writeHTMLTable(b, "Kullanım Matrisi", []string{"Kullanım tipi", "Uygun mu?", "Durum", "Gerekçe"}, decisionSupportUseCaseRows(support))
	writeHTMLTable(b, "Aksiyon Kapıları", []string{"Kapı", "Durum", "Gereken", "Mevcut", "Neden"}, decisionSupportGateRows(support, 10))
	writeHTMLTable(b, "İyileştirme ve Canlı Kullanım Hazırlığı", []string{"Öncelik", "Alan", "Aksiyon", "Yatırımcıya açık kabul ölçütü"}, decisionSupportActionRows(support, 6))
	b.WriteString("</section>\n")
}

func decisionSupportUseCaseRows(support *analysis.DecisionSupportReport) [][]string {
	if support == nil {
		return nil
	}
	rows := [][]string{}
	for _, item := range support.UseCaseMatrix {
		allowed := "Hayır"
		if item.Allowed {
			allowed = "Evet"
		}
		rows = append(rows, []string{item.UseCase, allowed, institutionalStatusTR(item.Status), item.Reason})
	}
	return rows
}

func decisionSupportGateRows(support *analysis.DecisionSupportReport, limit int) [][]string {
	if support == nil {
		return nil
	}
	rows := [][]string{}
	for _, gate := range support.ActionGates {
		if limit > 0 && len(rows) >= limit {
			break
		}
		rows = append(rows, []string{
			reportLabel(gate.Name),
			institutionalStatusTR(gate.Status),
			gate.Required,
			emptyFallback(gate.Current, "Yok"),
			emptyFallback(gate.Reason, strings.Join(gate.NextSteps, "; ")),
		})
	}
	return rows
}

func decisionSupportActionRows(support *analysis.DecisionSupportReport, limit int) [][]string {
	if support == nil {
		return nil
	}
	rows := [][]string{}
	for _, step := range support.CompletionActions {
		if limit > 0 && len(rows) >= limit {
			break
		}
		criteria := investorSafeAcceptanceCriteria(step.AcceptanceCriteria)
		rows = append(rows, []string{
			fmt.Sprintf("%d", step.Priority),
			reportLabel(step.Area),
			step.Action,
			criteria,
		})
	}
	return rows
}

func investorSafeAcceptanceCriteria(criteria []string) string {
	out := []string{}
	for _, item := range criteria {
		item = redactInternalReportText(item)
		if item == "" || strings.Contains(item, "yatırımcı raporunda gösterilmez") {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return "Eksik veri tamamlandığında ilgili kalite kapısı geçmeli."
	}
	return strings.Join(limitStrings(out, 2), "; ")
}

func writeValueInvestingHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	v := result.Professional.ValueInvesting
	b.WriteString("<section class=\"section\"><h2>Fiyat / İçsel Değer / Güvenlik Marjı</h2><table><tbody>")
	for _, row := range valueInvestingRows(result) {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table>")
	if len(v.Checks) > 0 {
		b.WriteString("<h3>Değer Yatırım Kontrolleri</h3><table><thead><tr><th>Kontrol</th><th>Durum</th><th class=\"num\">Skor</th><th>Açıklama</th></tr></thead><tbody>")
		for _, check := range v.Checks {
			b.WriteString("<tr><td>")
			b.WriteString(html.EscapeString(valueCheckLabel(check.Name)))
			b.WriteString("</td><td>")
			b.WriteString(html.EscapeString(institutionalStatusTR(check.Status)))
			b.WriteString("</td><td class=\"num\">")
			b.WriteString(html.EscapeString(fmt.Sprintf("%.0f/100", check.Score)))
			b.WriteString("</td><td>")
			b.WriteString(html.EscapeString(check.Message))
			b.WriteString("</td></tr>")
		}
		b.WriteString("</tbody></table>")
	}
	writeHTMLTable(b, "Buffett / Değer Yatırımı Gereksinim Matrisi", []string{"Sütun", "Gereksinim", "Durum", "Kanıt / Eksik"}, buffettRequirementRows(result, 0))
	b.WriteString("</section>\n")
}

func writeInvestmentResearchHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	review := result.Professional.InvestmentResearch
	if !review.Computed {
		return
	}
	b.WriteString("<section class=\"section\"><h2>Yatırım Kararı Denetimi</h2>")
	b.WriteString("<div class=\"note\"><strong>Yatırım hikayesi:</strong> ")
	b.WriteString(html.EscapeString(review.InvestmentStory.CoreThesis))
	b.WriteString("</div><table><tbody>")
	for _, row := range investmentResearchRows(result) {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table>")
	writeHTMLTable(b, "Yatırımcı İhtiyacı Karşılama", []string{"Yatırımcı tipi", "Karşılama", "Yorum"}, investmentReadinessRows(review))
	writeHTMLTable(b, valuationTransparencyTitle(result), []string{"Konu", "Durum"}, valuationTransparencyRows(result))
	writeHTMLTable(b, "Varlık Doğrulama", []string{"Konu", "Durum"}, assetDueDiligenceRows(review))
	writeHTMLTable(b, "Finansal Kalite Köprüsü", []string{"Metrik", "Değer", "Durum", "Yorum"}, financialQualityRows(result))
	writeHTMLTable(b, "Karar Değişim Koşulları", []string{"Başlık", "Koşul"}, decisionFrameworkRows(review))
	writeHTMLTable(b, "Teknik Sinyal Önceliği", []string{"Başlık", "Öncelikli okuma"}, technicalPriorityRows(result))
	b.WriteString("</section>\n")
}

func writeSourceAppendixHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	hasSources := result.Professional.KAPPDFIngest.Computed ||
		result.Professional.KAPAssetInventory.Computed ||
		(result.Professional.RawKAPData != nil && result.Professional.RawKAPData.Computed) ||
		len(result.Professional.ValueInvesting.DocumentEvidence.KeyDocuments) > 0
	if !hasSources {
		return
	}
	b.WriteString("<section class=\"section\"><h2>Kaynaklar ve PDF Ekleri</h2><div class=\"note\"><strong>Okuma notu:</strong> Bu bölüm finansal yorumun kanıt deposudur. Ham PDF listeleri, OCR/AI çözüm kayıtları ve satır seviyesinde çıkarımlar karar metninin üstüne karışmasın diye raporun sonuna alınmıştır.</div></section>\n")
	writeKAPPDFFinancialAnalysisHTML(b, result)
	if len(result.Professional.ValueInvesting.DocumentEvidence.KeyDocuments) > 0 {
		b.WriteString("<section class=\"section\"><h2>KAP PDF / İçerik Analizi</h2>")
		writeHTMLTable(b, "Öne Çıkan Kaynak Belgeler", []string{"Belge", "Tür", "Amaç", "İçerik", "Değerleme Etkisi"}, documentEvidenceRows(result, 12))
		b.WriteString("</section>\n")
	}
	writeKAPPDFIngestHTML(b, result)
	writeKAPRawDataHTML(b, result)
}

func writeKAPPDFIngestHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	kap := result.Professional.KAPPDFIngest
	if !kap.Computed {
		return
	}
	b.WriteString("<section class=\"section\"><h2>KAP PDF Raporları</h2><table><tbody>")
	for _, row := range kapPDFIngestRows(kap) {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table>")
	writeHTMLTable(b, "Öne Çıkan PDF Kanıtları", []string{"Belge", "Tür", "Kalite", "İçerik sinyali"}, kapPDFIngestDocumentRows(kap, 8))
	writeHTMLTable(b, "Tüm PDF Listesi", []string{"Belge", "Tür", "Kalite", "Metin", "Uyarı"}, kapPDFAllDocumentRows(kap))
	b.WriteString("</section>\n")
}

func writeKAPAssetInventoryHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	inventory := result.Professional.KAPAssetInventory
	if !inventory.Computed {
		return
	}
	b.WriteString("<section class=\"section\"><h2>KAP Varlık Envanteri</h2><table><tbody>")
	for _, row := range kapAssetInventoryRows(inventory) {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table>")
	writeHTMLTable(b, "Varlık Envanteri", []string{"Varlık", "Tür", "Lokasyon", "Ada/Parsel", "Alan", "KDV hariç", "KDV dahil", "Defter/GUD", "Özet değer", "Yİ-ÜFE güncel değer", "Kira", "Dönem", "Güven", "Kaynak"}, kapAssetInventoryItemRows(inventory))
	b.WriteString("</section>\n")
}

func writeKAPAssetReferenceHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	inventory := result.Professional.KAPAssetInventory
	if !inventory.Computed {
		return
	}
	rows := [][]string{
		{"Analiz rolü", "Referans bilgi; banka değerlemesinde ana karar girdisi değildir."},
		{"Değerleme etkisi", "Düşük. Banka değerlemesi özkaynak, sürdürülebilir ROE, aktif kalitesi, sermaye yeterliliği ve fonlama yapısıyla okunur."},
		{"Ana karar girdisi", "Hayır"},
		{"Kapsam", fmt.Sprintf("%d olay | %d birleşik satır | %d rapor satırı", inventory.EventCount, inventory.RawAssetCount, inventory.DisplayAssetCount)},
		{"Kullanım notu", "PDF içindeki gayrimenkul, tesis veya ekspertiz satırları banka için teminat/operasyonel ek bilgi olabilir; doğrudan hedef fiyat kanıtı gibi kullanılmaz."},
	}
	b.WriteString("<section class=\"section\"><h2>KAP Varlık Envanteri (Referans)</h2><table><tbody>")
	for _, row := range rows {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table></section>\n")
}

func writeKAPRawDataHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	raw := result.Professional.RawKAPData
	if raw == nil || !raw.Computed {
		return
	}
	b.WriteString("<section class=\"section\"><h2>KAP Ham Veri İndeksi</h2>")
	b.WriteString("<div class=\"note\"><strong>Kapsam:</strong> Bu bölüm KAP PDF metinlerinden çıkarılan kişi, ortaklık, kurumsal olay, finansal tablo ve bilgi grafiği adaylarını özetler. İç veri dosyaları ve operasyon kayıtları yatırımcı raporunda gösterilmez.</div>")
	b.WriteString("<table><tbody>")
	for _, row := range kapRawDataSummaryRows(*raw) {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table>")
	writeHTMLTable(b, "KAP PDF Veri Kapsamı", []string{"Veri alanı", "Adet", "Analiz rolü", "Durum", "Kaynak durumu", "Gözlenen alanlar"}, kapRawReportableCatalogRows(raw))
	writeHTMLTable(b, "Finansal Tablo Blokları", []string{"Tablo tipi", "Adet", "Son dönem", "Kaynak / örnek"}, kapRawFinancialTableRows(*raw, 40))
	writeHTMLTable(b, "Ham Finansal Satır Adayları (Denetim)", []string{"Kalem", "Tablo", "Değer", "Dönem", "Güven", "Kaynak"}, kapRawFinancialFactRows(*raw, 80))
	writeHTMLTable(b, "Yönetim / Kişi Adayları", []string{"Kişi", "Rol", "Unvan", "Kayıt", "Son dönem", "Güven"}, kapRawPeopleRows(*raw, 120))
	ownershipTitle := "Ortaklık / Sermaye Adayları"
	if isBankReport(result) {
		ownershipTitle = "Ortaklık / Sermaye Adayları (Denetim; Resmi Ana Ortaklık Değil)"
	}
	writeHTMLTable(b, ownershipTitle, []string{"Pay sahibi", "Pay oranı", "Pay adedi/tutar", "Kayıt", "Son dönem", "Kaynak"}, kapRawOwnershipRows(*raw, 120))
	writeHTMLTable(b, "Kurumsal Olay Adayları", []string{"Olay", "Başlık", "Tutar", "Kayıt", "Son dönem", "Kaynak"}, kapRawCorporateEventRows(*raw, 140))
	writeHTMLTable(b, "Belge İlişki Ağı / Veri Mutabakatı", []string{"Başlık", "Değer", "Açıklama"}, kapRawKnowledgeGraphRows(*raw, 30))
	b.WriteString("</section>\n")
}

func writeKAPPDFFinancialAnalysisHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	raw := result.Professional.RawKAPData
	if raw == nil || !raw.Computed || len(raw.FinancialFacts) == 0 {
		return
	}
	rows := kapPDFFinancialMetricRows(raw, result.Currency)
	if len(rows) == 0 {
		return
	}
	b.WriteString("<section class=\"section\"><h2>PDF Kaynaklı Finansal Analiz</h2>")
	b.WriteString("<div class=\"note\"><strong>Okuma yöntemi:</strong> Bu bölüm PDF finansal tablo satırlarından ana bilanço, gelir tablosu ve nakit akışı kalemlerini seçer; her satır son dönem, önceki dönem, değişim ve kaynak kanıtı ile raporlanır.</div>")
	writeHTMLTable(b, "Ana Finansal Tablo Kalemleri", []string{"Kalem", "Tablo", "Son dönem", "Son değer", "Önceki dönem", "Değişim", "Kanıt"}, rows)
	writeHTMLTable(b, "Finansal Okuma", []string{"Başlık", "Okuma", "Kanıt"}, kapPDFFinancialReadingRows(raw, result.Currency, isBankReport(result)))
	writeHTMLTable(b, "KAP Şirket Bilgisi", []string{"Başlık", "Değer", "Kaynak"}, kapPDFCompanyInfoRows(result))
	b.WriteString("</section>\n")
}

func writeInvestorQAHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	qa := result.InvestorQA
	if !qa.Computed {
		return
	}
	b.WriteString("<section class=\"section\"><h2>Yatırımcı Soru-Cevap / Komite Özeti</h2><table><tbody>")
	for _, row := range investorQASummaryRows(result) {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table>")
	if len(qa.ActionMatrix) > 0 {
		writeHTMLTable(b, "AL / TUT-İZLE / SAT Karar Matrisi", []string{"Aksiyon", "Mevcut", "Durum", "Güven / Skor", "Seviyeler", "Tetikleyici", "Geçersiz Kılan Şart", "Kanıt / Engel"}, actionMatrixRows(result, qa.ActionMatrix))
	}
	b.WriteString("<h3>En Çok Sorulan Sorular</h3><table><thead><tr><th>Soru</th><th>Cevap</th><th>Durum</th><th class=\"num\">Güven</th></tr></thead><tbody>")
	for _, item := range qa.Questions {
		b.WriteString("<tr><td>")
		b.WriteString(html.EscapeString(item.Question))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(retailText(item.Answer)))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(institutionalStatusTR(item.Status)))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(html.EscapeString(fmt.Sprintf("%.0f/100", item.Confidence)))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table></section>\n")
}

func actionMatrixRows(result analysis.SymbolAnalysis, items []investorqa.ActionSignal) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		current := boolTRForReport(item.CurrentSignal)
		status := institutionalStatusTR(item.Status) + " - " + item.Label
		levels := actionSignalLevels(item, result.Currency)
		trigger := item.Trigger
		invalidation := item.Invalidation
		evidence := actionSignalEvidenceBlockers(item)
		if reportTradePlanBlocked(result) && actionSignalIsBuy(item) {
			current = "Hayır"
			status = "Kapalı - AL/SAT kullanım kapısı geçmedi"
			levels = "Emir seviyesi değil; sadece izleme/teyit seviyesi"
			trigger = "Önce AL/SAT kullanım kapısı, finansal/kurumsal kanıt ve teknik sinyal kapısı geçmeli."
			if invalidation == "" {
				invalidation = "Kapı geçmeden yeni alım kurulmaz."
			}
			evidence = strings.TrimSpace("Karar destek kapısı kapalı. " + evidence)
		}
		rows = append(rows, []string{
			reportLabel(item.Action),
			current,
			status,
			fmt.Sprintf("güven %.0f/100 | skor %.0f/100", item.Confidence, item.Score),
			levels,
			trigger,
			invalidation,
			evidence,
		})
	}
	return rows
}

func actionSignalIsBuy(item investorqa.ActionSignal) bool {
	action := strings.ToLower(strings.TrimSpace(item.Action))
	return strings.Contains(action, "al") || strings.Contains(action, "buy")
}

func actionSignalLevels(item investorqa.ActionSignal, currency string) string {
	parts := []string{}
	switch {
	case item.EntryMin > 0 && item.EntryMax > 0:
		parts = append(parts, fmt.Sprintf("giriş %s-%s", retailPrice(item.EntryMin, currency), retailPrice(item.EntryMax, currency)))
	case item.EntryMin > 0:
		parts = append(parts, fmt.Sprintf("giriş %s", retailPrice(item.EntryMin, currency)))
	case item.EntryMax > 0:
		parts = append(parts, fmt.Sprintf("giriş %s", retailPrice(item.EntryMax, currency)))
	}
	if item.StopLoss > 0 {
		parts = append(parts, fmt.Sprintf("stop %s", retailPrice(item.StopLoss, currency)))
	}
	switch {
	case item.Target1 > 0 && item.Target2 > 0:
		parts = append(parts, fmt.Sprintf("hedef %s/%s", retailPrice(item.Target1, currency), retailPrice(item.Target2, currency)))
	case item.Target1 > 0:
		parts = append(parts, fmt.Sprintf("hedef %s", retailPrice(item.Target1, currency)))
	case item.Target2 > 0:
		parts = append(parts, fmt.Sprintf("hedef %s", retailPrice(item.Target2, currency)))
	}
	if item.RiskRewardRatio > 0 {
		parts = append(parts, fmt.Sprintf("R/R %.2f", item.RiskRewardRatio))
	}
	if len(parts) == 0 {
		return "Seviye üretilmedi; şart bekleniyor"
	}
	return strings.Join(parts, " | ")
}

func actionSignalEvidenceBlockers(item investorqa.ActionSignal) string {
	parts := []string{}
	if len(item.Evidence) > 0 {
		parts = append(parts, "kanıt: "+strings.Join(limitStrings(item.Evidence, 3), "; "))
	}
	if len(item.Blockers) > 0 {
		parts = append(parts, "engel: "+strings.Join(reportLabels(limitStrings(item.Blockers, 3)), "; "))
	}
	if len(parts) == 0 {
		return "Kanıt/engel yok"
	}
	return strings.Join(parts, " | ")
}

func boolTRForReport(value bool) string {
	if value {
		return "Evet"
	}
	return "Hayır"
}

func writeInstitutionalPersonaViewsHTML(b *strings.Builder, result analysis.SymbolAnalysis) {
	views := result.InvestorQA.InstitutionalViews
	if !views.Computed {
		return
	}
	b.WriteString("<section class=\"section\"><h2>Rapor Kalite ve Aksiyon Kapıları</h2><table><tbody>")
	for _, row := range institutionalPersonaSummaryRows(result) {
		writeHTMLRow(b, row[0], row[1])
	}
	b.WriteString("</tbody></table><h3>Profil Kararları</h3><table><thead><tr><th>Profil</th><th>Rapor Kalitesi</th><th>Yatırım/İşlem Uygunluğu</th><th class=\"num\">Skor</th><th>Ana Engel</th><th>Kanıt</th></tr></thead><tbody>")
	for _, view := range institutionalPersonaList(result) {
		b.WriteString("<tr><td>")
		b.WriteString(html.EscapeString(view.Name))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(institutionalStatusTR(view.ReportQualityStatus) + " - " + view.ReportQualityLabel))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(institutionalStatusTR(view.Status) + " - " + view.DecisionLabel))
		b.WriteString("</td><td class=\"num\">")
		b.WriteString(html.EscapeString(fmt.Sprintf("kalite %.0f/100 | tez %.0f/100", view.ReportQualityScore, view.Score)))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(reportText(firstStringOr(view.Blockers, "Kritik engel yok"))))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(reportText(joinEvidence(view.Evidence, 3))))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table></section>\n")
}

func investorQASummaryRows(result analysis.SymbolAnalysis) [][]string {
	qa := result.InvestorQA
	rows := [][]string{
		{"Karar", qa.Decision + " - " + qa.DecisionLabel},
		{"Tek cümle cevap", qa.OneLineAnswer},
		{"Yatırımcı profili", qa.InvestorProfile},
		{"Skor / güven", fmt.Sprintf("%.0f/100 | %.0f/100", qa.Score, qa.Confidence)},
		{"Ana fırsat", qa.TopOpportunity},
		{"Ana risk", qa.TopRisk},
		{"Alım şartları", strings.Join(qa.BuyConditions, "; ")},
		{"Tez bozulma şartları", strings.Join(qa.ExitConditions, "; ")},
		{investorQAQualityLabel(result), fmt.Sprintf("%.0f/100 | %s", qa.Quality.Score, reportLabel(qa.Quality.Label))},
		{"Makro / piyasa", fmt.Sprintf("%.0f/100 | %s", qa.Macro.Score, qa.Macro.Regime)},
		{"Likidite", fmt.Sprintf("%.0f/100 | %s", qa.Liquidity.Score, qa.Liquidity.InstitutionalFit)},
		{"Yönetişim", fmt.Sprintf("%.0f/100 | %s", qa.Governance.Score, qa.Governance.DataLineage)},
		{"Model riski", fmt.Sprintf("%.0f/100 | %s", qa.ModelRisk.Score, institutionalStatusTR(qa.ModelRisk.Status))},
	}
	if isEquityResult(result) {
		rows = append(rows[:10], append([][]string{{"GSYH makro etkisi", gdpImpactSummary(result)}}, rows[10:]...)...)
	}
	return rows
}

func investorQAQualityLabel(result analysis.SymbolAnalysis) string {
	if isMarketOnlyResult(result) {
		return "Veri kapsamı"
	}
	return "Kalite"
}

func gdpImpactSummary(result analysis.SymbolAnalysis) string {
	gdp := result.Professional.Market.GDP
	if !gdp.Computed {
		return emptyFallback(gdp.DataQualityWarning, "TÜİK CİP GSYH verisi rapora bağlı değil")
	}
	summary := fmt.Sprintf("%d | kişi başı %.0f $ / %.0f TL | GSYH %.0f bin TL | skor %.0f/100 | %s",
		gdp.LatestYear,
		gdp.PerCapitaUSD,
		gdp.PerCapitaTRY,
		gdp.GDPThousandTRY,
		gdp.Score,
		gdp.Regime,
	)
	if gdp.ObservationLagYears > 0 || gdp.FreshnessStatus != "" {
		parts := []string{}
		if gdp.ReferenceYear > 0 && gdp.LatestYear > 0 {
			parts = append(parts, fmt.Sprintf("referans yıl %d, son gerçekleşen gözlem %d", gdp.ReferenceYear, gdp.LatestYear))
		}
		if gdp.ObservationLagYears > 0 {
			parts = append(parts, fmt.Sprintf("%d yıl gözlem gecikmesi", gdp.ObservationLagYears))
		}
		if gdp.FreshnessStatus != "" {
			parts = append(parts, "durum "+gdp.FreshnessStatus)
		}
		if len(parts) > 0 {
			summary += " | " + strings.Join(parts, ", ")
		}
	}
	if gdp.DataQualityWarning != "" {
		summary += " | Uyarı: " + gdp.DataQualityWarning
	}
	if isBankReport(result) {
		summary += " | banka için GSYH yardımcı bağlamdır; faiz, mevduat maliyeti, NPL, karşılıklar ve sermaye yeterliliğiyle doğrulanmalıdır"
	}
	return summary
}

func institutionalPersonaSummaryRows(result analysis.SymbolAnalysis) [][]string {
	views := result.InvestorQA.InstitutionalViews
	if isMarketOnlyResult(result) {
		assetLabel := marketAssetLabel(result)
		return [][]string{
			{"Rapor kalite durumu", institutionalStatusTR(views.OverallQualityStatus)},
			{"Yatırım/işlem uygunluğu", institutionalStatusTR(views.OverallStatus)},
			{assetLabel + " aksiyon kapıları", institutionalStatusTR(views.EliteCandidate.Status) + " | " + views.EliteCandidate.Label},
			{"Aksiyon kapısı özeti", views.EliteCandidate.Summary},
			{assetLabel + " al/sat karar desteği", institutionalStatusTR(views.FinancialTransactionUse.Status) + " | " + views.FinancialTransactionUse.Answer},
			{"İşlem kullanım özeti", views.FinancialTransactionUse.Summary},
			{"Başarı şartı", institutionalPersonaSuccessRule(result)},
			{"Özet", views.Summary},
			{"Kalite özeti", views.QualitySummary},
			{"Kurumsal " + strings.ToLower(assetLabel) + " portföy çerçevesi", views.Portfolio.FrameworkCommentary},
			{"Kurumsal " + strings.ToLower(assetLabel) + " portföy net sonuç", views.Portfolio.Takeaway},
			{"Kurumsal " + strings.ToLower(assetLabel) + " portföy işlem kullanımı", views.Portfolio.TransactionUseAnswer},
			{"Kurumsal " + strings.ToLower(assetLabel) + " portföy rapor kalitesi", views.Portfolio.QualityAnswer},
			{"Kurumsal " + strings.ToLower(assetLabel) + " portföy cevabı", views.Portfolio.OneLineAnswer},
			{"İstatistiksel işlem avantajı çerçevesi", views.TradingEdge.FrameworkCommentary},
			{"İstatistiksel işlem avantajı net sonuç", views.TradingEdge.Takeaway},
			{"İstatistiksel işlem avantajı işlem kullanımı", views.TradingEdge.TransactionUseAnswer},
			{"İstatistiksel işlem avantajı rapor kalitesi", views.TradingEdge.QualityAnswer},
			{"İstatistiksel işlem avantajı cevabı", views.TradingEdge.OneLineAnswer},
			{"Gerekli aksiyonlar", personaRequiredActionSummary(result)},
		}
	}
	return [][]string{
		{"Rapor kalite durumu", institutionalStatusTR(views.OverallQualityStatus)},
		{"Yatırım/işlem uygunluğu", institutionalStatusTR(views.OverallStatus)},
		{"Üç aksiyon kapısı", institutionalStatusTR(views.EliteCandidate.Status) + " | " + views.EliteCandidate.Label},
		{"Aksiyon kapısı özeti", views.EliteCandidate.Summary},
		{"Finansal al/sat karar desteği", institutionalStatusTR(views.FinancialTransactionUse.Status) + " | " + views.FinancialTransactionUse.Answer},
		{"Al/sat kullanım özeti", views.FinancialTransactionUse.Summary},
		{"Başarı şartı", institutionalPersonaSuccessRule(result)},
		{"Özet", views.Summary},
		{"Kalite özeti", views.QualitySummary},
		{"Değer yatırım çerçevesi", views.ValueInvesting.FrameworkCommentary},
		{"Değer yatırım net sonuç", views.ValueInvesting.Takeaway},
		{"Değer yatırım al/sat kullanımı", views.ValueInvesting.TransactionUseAnswer},
		{"Değer yatırım rapor kalitesi", views.ValueInvesting.QualityAnswer},
		{"Değer yatırım cevabı", views.ValueInvesting.OneLineAnswer},
		{"Kurumsal portföy çerçevesi", views.Portfolio.FrameworkCommentary},
		{"Kurumsal portföy net sonuç", views.Portfolio.Takeaway},
		{"Kurumsal portföy al/sat kullanımı", views.Portfolio.TransactionUseAnswer},
		{"Kurumsal portföy rapor kalitesi", views.Portfolio.QualityAnswer},
		{"Kurumsal portföy cevabı", views.Portfolio.OneLineAnswer},
		{"İstatistiksel işlem avantajı çerçevesi", views.TradingEdge.FrameworkCommentary},
		{"İstatistiksel işlem avantajı net sonuç", views.TradingEdge.Takeaway},
		{"İstatistiksel işlem avantajı al/sat kullanımı", views.TradingEdge.TransactionUseAnswer},
		{"İstatistiksel işlem avantajı rapor kalitesi", views.TradingEdge.QualityAnswer},
		{"İstatistiksel işlem avantajı cevabı", views.TradingEdge.OneLineAnswer},
		{"Gerekli aksiyonlar", personaRequiredActionSummary(result)},
	}
}

func institutionalPersonaCompactRows(result analysis.SymbolAnalysis) [][]string {
	views := result.InvestorQA.InstitutionalViews
	if isMarketOnlyResult(result) {
		assetLabel := marketAssetLabel(result)
		return [][]string{
			{"Rapor kalite durumu", institutionalStatusTR(views.OverallQualityStatus)},
			{"Yatırım/işlem uygunluğu", institutionalStatusTR(views.OverallStatus)},
			{assetLabel + " aksiyon kapıları", institutionalStatusTR(views.EliteCandidate.Status) + " | " + views.EliteCandidate.Label},
			{assetLabel + " al/sat karar desteği", institutionalStatusTR(views.FinancialTransactionUse.Status) + " | " + views.FinancialTransactionUse.Answer},
			{"Kalite özeti", views.QualitySummary},
			{"Başarı şartı", institutionalPersonaSuccessRule(result)},
		}
	}
	return [][]string{
		{"Rapor kalite durumu", institutionalStatusTR(views.OverallQualityStatus)},
		{"Yatırım/işlem uygunluğu", institutionalStatusTR(views.OverallStatus)},
		{"Üç aksiyon kapısı", institutionalStatusTR(views.EliteCandidate.Status) + " | " + views.EliteCandidate.Label},
		{"Finansal al/sat karar desteği", institutionalStatusTR(views.FinancialTransactionUse.Status) + " | " + views.FinancialTransactionUse.Answer},
		{"Kalite özeti", views.QualitySummary},
		{"Başarı şartı", institutionalPersonaSuccessRule(result)},
	}
}

func institutionalPersonaSuccessRule(result analysis.SymbolAnalysis) string {
	if isCryptoResult(result) {
		return "Kripto rapor kalitesi kurumsal portföy ve istatistiksel işlem avantajı kalite kapılarında Geçti olmalı; canlı işlem için blokzincir, türev piyasa, borsa giriş-çıkış ve haber/duygu tonu teyidi ayrıca aranmalıdır."
	}
	if isCommodityResult(result) {
		return "Altın/emtia rapor kalitesi kurumsal portföy ve istatistiksel işlem avantajı kalite kapılarında Geçti olmalı; canlı işlem için DXY/reel faiz, vadeli pozisyon ve fon akışı teyidi ayrıca aranmalıdır."
	}
	return "Rapor kalitesi değer yatırım, kurumsal portföy ve istatistiksel işlem avantajı kalite kapılarında Geçti olmalı; elit yatırım adayı sayılması için değer yatırım tezi, kurumsal portföy uygunluğu ve istatistiksel işlem avantajı sinyalinin üçü de Geçti olmalıdır."
}

func institutionalPersonaCommentaryRows(result analysis.SymbolAnalysis) []string {
	views := result.InvestorQA.InstitutionalViews
	if isMarketOnlyResult(result) {
		label := strings.ToLower(marketAssetLabel(result))
		return []string{
			"Kurumsal " + label + " portföy: " + views.Portfolio.FrameworkCommentary + " Net sonuç: " + views.Portfolio.Takeaway,
			"İstatistiksel işlem avantajı: " + views.TradingEdge.FrameworkCommentary + " Net sonuç: " + views.TradingEdge.Takeaway,
		}
	}
	return []string{
		"Değer yatırım: " + views.ValueInvesting.FrameworkCommentary + " Net sonuç: " + views.ValueInvesting.Takeaway,
		"Kurumsal portföy: " + views.Portfolio.FrameworkCommentary + " Net sonuç: " + views.Portfolio.Takeaway,
		"İstatistiksel işlem avantajı: " + views.TradingEdge.FrameworkCommentary + " Net sonuç: " + views.TradingEdge.Takeaway,
	}
}

func institutionalPersonaTableRows(result analysis.SymbolAnalysis) [][]string {
	rows := [][]string{}
	for _, view := range institutionalPersonaList(result) {
		rows = append(rows, []string{
			view.Name,
			institutionalStatusTR(view.ReportQualityStatus),
			institutionalStatusTR(view.Status),
			fmt.Sprintf("kalite %.0f/100 | tez %.0f/100", view.ReportQualityScore, view.Score),
			view.DecisionLabel,
			firstStringOr(view.Blockers, "Kritik engel yok"),
		})
	}
	return rows
}

func institutionalPersonaList(result analysis.SymbolAnalysis) []investorqa.PersonaView {
	views := result.InvestorQA.InstitutionalViews
	if isMarketOnlyResult(result) {
		return []investorqa.PersonaView{views.Portfolio, views.TradingEdge}
	}
	return []investorqa.PersonaView{views.ValueInvesting, views.Portfolio, views.TradingEdge}
}

func firstStringOr(values []string, fallback string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return fallback
}

func joinEvidence(items []investorqa.EvidenceItem, limit int) string {
	if limit <= 0 || len(items) == 0 {
		return "Kanıt yok"
	}
	out := []string{}
	for _, item := range items {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(item.Label) == "" && strings.TrimSpace(item.Value) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(item.Label)+": "+strings.TrimSpace(item.Value))
	}
	if len(out) == 0 {
		return "Kanıt yok"
	}
	return strings.Join(out, "; ")
}

func personaRequiredActionSummary(result analysis.SymbolAnalysis) string {
	actions := []string{}
	for _, view := range institutionalPersonaList(result) {
		if len(view.RequiredActions) == 0 {
			continue
		}
		actions = append(actions, view.Name+": "+strings.Join(view.RequiredActions, "; "))
	}
	if len(actions) == 0 {
		return "Ek aksiyon yok"
	}
	return strings.Join(actions, " | ")
}

func investorQAQuestionRows(result analysis.SymbolAnalysis) [][]string {
	rows := [][]string{}
	for _, item := range result.InvestorQA.Questions {
		rows = append(rows, []string{
			item.Question,
			retailText(item.Answer),
			institutionalStatusTR(item.Status),
			fmt.Sprintf("%.0f/100", item.Confidence),
		})
	}
	return rows
}

func valueInvestingRows(result analysis.SymbolAnalysis) [][]string {
	v := result.Professional.ValueInvesting
	if !v.Computed && v.Decision == "" && v.Summary == "" {
		return [][]string{{"Durum", "İçsel değer katmanı bu raporda üretilemedi."}}
	}
	intrinsic := "Pozitif içsel değer üretilemedi"
	if v.IntrinsicValue.Computed {
		intrinsic = fmt.Sprintf("Kötümser %.2f | Baz %.2f | İyimser %.2f %s", v.IntrinsicValue.Bear, v.IntrinsicValue.Base, v.IntrinsicValue.Bull, displayCurrency(result.Currency))
	} else if v.IntrinsicValue.Base > 0 {
		intrinsic = fmt.Sprintf("Taslak baz değer %.2f %s; güvenlik marjı için yeterli değil", v.IntrinsicValue.Base, displayCurrency(result.Currency))
	}
	margin := "Hesaplanamadı"
	if v.MarginOfSafety.Computed {
		margin = fmt.Sprintf("%.1f%% | gereken %.1f%%", v.MarginOfSafety.BasePct, v.MarginOfSafety.RequiredPct)
	} else if v.MarginOfSafety.RequiredPct > 0 {
		margin = fmt.Sprintf("Hesaplanamadı | gereken %.1f%%", v.MarginOfSafety.RequiredPct)
	}
	rows := [][]string{
		{"Durum", emptyFallback(v.DecisionLabel, "İçsel değer güvenilir hesaplanamadı")},
		{"Ucuz mu, pahalı mı?", fairValueConclusionText(result)},
		{"Sade cevap", emptyFallback(v.Summary, "sahibine kalan nakit, normalize serbest nakit akımı veya sektör modeli için veri/güven yetersiz.")},
		{"Güncel fiyat", fmt.Sprintf("%.2f %s", v.CurrentPrice, displayCurrency(result.Currency))},
		{"Olması gereken değer", fairValueRangeText(result)},
		{"Fiyat/değer farkı", fairValueGapText(result)},
		{"İçsel değer aralığı", intrinsic},
		{"Güvenlik marjı", margin},
		{"Model", emptyFallback(v.SectorModel.Label, "Model belirlenemedi")},
		{"Model gerekçesi", emptyFallback(v.SectorModel.Reason, "Sektör modeli gerekçesi yok.")},
		{"KAP PDF / belge kanıtı", documentEvidenceSummary(result)},
		{"TÜİK GSYH etkisi", gdpImpactSummary(result)},
		{"Değerleme güveni / kalite", fmt.Sprintf("%.0f/100 | %.0f/100", v.Confidence, v.QualityScore)},
		{"Buffett filtresi", buffettChecklistSummaryText(result)},
		{"Sahibine kalan nakit", ownerEarningsReportText(result)},
		{"Normalize serbest nakit akımı", normalizedFCFReportText(result)},
		{"Sermaye tahsisi", valueCapitalAllocationText(result)},
		{"1 Dolar testi", retainedEarningsTestText(result)},
		{"Rekabet gücü", fmt.Sprintf("%s | 5 yıllık özkaynak kârlılığı %.1f%% | 5 yıllık yatırılan sermaye kârlılığı %.1f%% | marj istikrarı %.0f/100 | skor %.0f/100", reportLabel(v.Moat.Label), v.Moat.AverageROE5Y*100, v.Moat.AverageROIC5Y*100, v.Moat.MarginStability*100, v.Moat.Score)},
		{"Varsayımlar", fmt.Sprintf("iskonto %.1f%% | terminal büyüme %.1f%% | sahibine kalan nakit büyümesi %.1f%% | vergi %.1f%%", v.Assumptions.DiscountRate*100, v.Assumptions.TerminalGrowth*100, v.Assumptions.OwnerEarningsGrowth*100, v.Assumptions.TaxRate*100)},
		{"Kullanılan değer girdileri", strings.Join(v.FairValue.DataInputs, "; ")},
	}
	if len(v.Warnings) > 0 {
		rows = append(rows, []string{"Uyarılar", strings.Join(valueWarningTexts(v.Warnings), ", ")})
	}
	if len(v.Missing) > 0 {
		rows = append(rows, []string{"Eksik veri", strings.Join(valueWarningTexts(v.Missing), ", ")})
	}
	return rows
}

func retainedEarningsTestText(result analysis.SymbolAnalysis) string {
	test := result.Professional.ValueInvesting.RetainedEarnings
	if !test.Computed {
		if len(test.Warnings) > 0 {
			return "Hesaplanamadı: " + strings.Join(reportLabels(test.Warnings), ", ")
		}
		return "Hesaplanamadı; 5 yıllık piyasa değeri ve dağıtılmamış kâr serisi gerekli."
	}
	return fmt.Sprintf("katsayı %.2fx | piyasa değeri değişimi %s | içeride tutulan kâr %s | yöntem: %s", test.Ratio, formatMoney(test.MarketCapChange5Y, result.Currency), formatMoney(test.RetainedEarnings5Y, result.Currency), reportLabel(test.Method))
}

func buffettChecklistSummaryText(result analysis.SymbolAnalysis) string {
	checklist := result.Professional.ValueInvesting.BuffettChecklist
	if !checklist.Computed {
		return "Buffett değer yatırımı matrisi üretilemedi."
	}
	text := fmt.Sprintf("%s | skor %.0f/100 | kapsam %.0f/100 | direkt AL uygunluğu: %s", emptyFallback(checklist.StatusLabel, reportLabel(checklist.Status)), checklist.Score, checklist.CoveragePct, localize.Bool(checklist.BuyEligible))
	if len(checklist.BlockingIssues) > 0 {
		text += " | engeller: " + strings.Join(reportLabels(limitStrings(checklist.BlockingIssues, 6)), ", ")
	}
	if len(checklist.MissingData) > 0 {
		text += " | eksik veri: " + strings.Join(reportLabels(limitStrings(checklist.MissingData, 6)), ", ")
	}
	return text
}

func buffettRequirementRows(result analysis.SymbolAnalysis, limit int) [][]string {
	requirements := result.Professional.ValueInvesting.BuffettChecklist.Requirements
	if limit > 0 && len(requirements) > limit {
		requirements = requirements[:limit]
	}
	rows := make([][]string, 0, len(requirements))
	for _, item := range requirements {
		detailParts := []string{}
		if strings.TrimSpace(item.Value) != "" {
			detailParts = append(detailParts, item.Value)
		}
		if strings.TrimSpace(item.Evidence) != "" {
			detailParts = append(detailParts, item.Evidence)
		}
		if item.Threshold != "" {
			detailParts = append(detailParts, "eşik: "+item.Threshold)
		}
		if len(item.Missing) > 0 {
			detailParts = append(detailParts, "eksik: "+strings.Join(reportLabels(item.Missing), ", "))
		}
		rows = append(rows, []string{
			reportLabel(item.Pillar),
			item.Label,
			institutionalStatusTR(item.Status),
			reportText(strings.Join(detailParts, " | ")),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"Değer yatırım", "Buffett matrisi", "Yok", "Bu raporda Buffett gereksinim matrisi üretilemedi."})
	}
	return rows
}

func valueCapitalAllocationText(result analysis.SymbolAnalysis) string {
	v := result.Professional.ValueInvesting
	leverage := fmt.Sprintf("net borç/özsermaye %.2f", v.CapitalAllocation.NetDebtToEquity)
	dividendText := capitalAllocationDividendText(result)
	if isBankReport(result) || result.Professional.Valuation.SectorModel == "bank_equity_model" || valuationRatioSuppressedForReport(result.Professional.Valuation, "NetDebt_Eq") {
		leverage = "net borç/özsermaye uygulanmaz"
		return fmt.Sprintf("5Y ödenmiş sermaye değişimi %.1f%% | ekonomik sulanma olarak sayılmadı; bedelli/bedelsiz/split/nominal düzeltme sınıflaması gerekir | 10Y temettü %s | %s | skor %.0f/100", v.CapitalAllocation.Dilution5YPct, dividendText, leverage, v.CapitalAllocation.Score)
	}
	return fmt.Sprintf("5Y pay sulanması %.1f%% | 10Y temettü %s | %s | skor %.0f/100", v.CapitalAllocation.Dilution5YPct, dividendText, leverage, v.CapitalAllocation.Score)
}

func capitalAllocationDividendText(result analysis.SymbolAnalysis) string {
	v := result.Professional.ValueInvesting.CapitalAllocation
	if !v.DividendDataAvailable && v.Dividends10Y == 0 {
		return "ölçülemedi; structured temettü alanları eksik"
	}
	text := formatMoney(v.Dividends10Y, result.Currency)
	if v.DividendDataAvailable {
		text += fmt.Sprintf(" | süreklilik 5Y %.0f%%, 10Y %.0f%%", v.DividendContinuity5Y*100, v.DividendContinuity10Y*100)
	}
	return text
}

func ownerEarningsReportText(result analysis.SymbolAnalysis) string {
	v := result.Professional.ValueInvesting
	if isBankReport(result) {
		return fmt.Sprintf("Uygulanmaz. Bankada klasik owner earnings ana girdi değildir; kâr kalitesi net faiz geliri, ücret-komisyon geliri, ticari kâr/zarar, karşılık giderleri, NPL ve sermaye yeterliliğiyle ölçülür. Skor %.0f/100.", v.OwnerEarnings.Score)
	}
	return fmt.Sprintf("son 12 ay %s | 5 yıllık normalize %s | skor %.0f/100 | uygulanır: %s", formatMoney(v.OwnerEarnings.TTM, result.Currency), formatMoney(v.OwnerEarnings.Normalized5Y, result.Currency), v.OwnerEarnings.Score, localize.Bool(v.OwnerEarnings.Applicable))
}

func normalizedFCFReportText(result analysis.SymbolAnalysis) string {
	v := result.Professional.ValueInvesting
	if isBankReport(result) {
		return fmt.Sprintf("Uygulanmaz. Bankada serbest nakit akımı 0 veya eksik görünmesi kalite bozukluğu değildir; NIM, NPL, kredi/mevduat, LCR, karşılık gideri ve mevduat maliyeti bağlanmalıdır. Skor %.0f/100.", v.NormalizedFCF.Score)
	}
	return fmt.Sprintf("son 12 ay %s | 5 yıllık medyan %s | 10 yıllık medyan %s | skor %.0f/100 | uygulanır: %s", formatMoney(v.NormalizedFCF.TTM, result.Currency), formatMoney(v.NormalizedFCF.Median5Y, result.Currency), formatMoney(v.NormalizedFCF.Median10Y, result.Currency), v.NormalizedFCF.Score, localize.Bool(v.NormalizedFCF.Applicable))
}

func fairValueConclusionText(result analysis.SymbolAnalysis) string {
	v := result.Professional.ValueInvesting
	fair := v.FairValue
	if fair.Computed {
		return fmt.Sprintf("%s | fiyat/değer farkı %.1f%% | baz potansiyel %.1f%%", fair.Label, fair.PriceToFairValuePct, fair.UpsideDownsidePct)
	}
	return reportText(emptyFallback(fair.Explanation, emptyFallback(v.DecisionLabel, "Fiyat/değer farkı hesaplanamadı")))
}

func fairValueRangeText(result analysis.SymbolAnalysis) string {
	fair := result.Professional.ValueInvesting.FairValue
	if fair.Computed || fair.FairValueBase > 0 {
		return fmt.Sprintf("Kötümser %.2f | Baz %.2f | İyimser %.2f %s", fair.FairValueBear, fair.FairValueBase, fair.FairValueBull, displayCurrency(result.Currency))
	}
	return "Hesaplanamadı"
}

func fairValueGapText(result analysis.SymbolAnalysis) string {
	fair := result.Professional.ValueInvesting.FairValue
	if !fair.Computed {
		return "Hesaplanamadı"
	}
	text := fmt.Sprintf("Fiyat baz içsel değere göre %.1f%%; baz senaryoya göre yukarı/aşağı alan %.1f%%; gereken marj %.1f%%", fair.PriceToFairValuePct, fair.UpsideDownsidePct, fair.RequiredMarginPct)
	if warning := valuationAuditWarningForReport(result); warning != "" {
		text += ". " + warning
	}
	return text
}

func investmentResearchRows(result analysis.SymbolAnalysis) [][]string {
	review := result.Professional.InvestmentResearch
	rows := [][]string{
		{"Özet", reportText(review.Summary)},
		{"Kurumsal not kararı", fmt.Sprintf("%s | skor %.0f/100 | direkt alım: %s | komite: %s | yayınlanabilir: %s",
			reportLabel(review.InstitutionalMemo.Recommendation),
			review.InstitutionalMemo.ReadinessScore,
			localize.Bool(review.InstitutionalMemo.DirectBuyEligible),
			localize.Bool(review.InstitutionalMemo.InvestmentCommitteeReady),
			localize.Bool(review.InstitutionalMemo.BrokeragePublishableReady),
		)},
		{"Değer nereden geliyor?", review.InvestmentStory.ValueSource},
		{"Piyasanın cevaplaması gereken soru", review.InvestmentStory.MispricingQuestion},
		{"Karar", reportLabel(review.DecisionFramework.CurrentDecision)},
		{"Karar gerekçesi", reportText(strings.Join(review.DecisionFramework.DecisionBasis, " "))},
	}
	if warning := valuationAuditWarningForReport(result); warning != "" {
		rows = append(rows, []string{"Değerleme model denetimi", warning})
	}
	if len(review.InstitutionalMemo.BlockingIssues) > 0 {
		rows = append(rows, []string{"Kurumsal engel", strings.Join(reportLabels(review.InstitutionalMemo.BlockingIssues), " | ")})
	}
	if len(review.InstitutionalMemo.RequiredFixes) > 0 {
		rows = append(rows, []string{"Komiteye girmeden zorunlu düzeltme", strings.Join(reportTexts(limitStrings(review.InstitutionalMemo.RequiredFixes, 6)), " | ")})
	}
	if len(review.OpenResearchQuestions) > 0 {
		rows = append(rows, []string{"Açık araştırma soruları", strings.Join(reportTexts(limitStrings(review.OpenResearchQuestions, 8)), " | ")})
	}
	if len(review.Warnings) > 0 {
		rows = append(rows, []string{"Araştırma uyarıları", strings.Join(reportLabels(review.Warnings), ", ")})
	}
	return rows
}

func valuationAuditWarningForReport(result analysis.SymbolAnalysis) string {
	v := result.Professional.ValueInvesting
	if !v.Computed || v.IntrinsicValue.Base <= 0 {
		return ""
	}
	if peerBase := result.Professional.Valuation.FairValue.Base; peerBase > 0 {
		divergencePct := math.Abs(peerBase-v.IntrinsicValue.Base) / v.IntrinsicValue.Base * 100
		if divergencePct >= 100 {
			return fmt.Sprintf("İçsel değer baz %.2f %s ile peer/model kontrol değeri %.2f %s arasında %.1f%% model kontrol ayrışması var; bu alan resmi hedef fiyat değil, hisse adedi/ölçek/sermaye düzeltmesi ve normalize nakit akımı denetimi gerektiren model uyarısıdır.", v.IntrinsicValue.Base, displayCurrency(result.Currency), peerBase, displayCurrency(result.Currency), divergencePct)
		}
	}
	if v.CurrentPrice > 0 {
		priceGapPct := (v.CurrentPrice/v.IntrinsicValue.Base - 1) * 100
		if priceGapPct >= 250 && (v.NormalizedFCF.Score < 55 || v.CapitalAllocation.Dilution5YPct > 10) {
			return "Fiyat/içsel değer farkı çok yüksek; zayıf FCF dönüşümü veya sermaye hareketi etkisi sınıflanmadan bu değer yatırım kararı olarak kullanılamaz."
		}
	}
	return ""
}

func investmentReadinessRows(review professional.InvestmentResearchReview) [][]string {
	rows := [][]string{}
	for _, item := range review.Readiness {
		rows = append(rows, []string{reportLabel(item.Segment), fmt.Sprintf("%.0f/100", item.CoveragePct), item.Comment})
	}
	return rows
}

func valuationTransparencyRows(result analysis.SymbolAnalysis) [][]string {
	bridge := result.Professional.InvestmentResearch.ValuationBridge
	rows := [][]string{
		{"Model", emptyFallback(bridge.Model, "Model yok")},
		{"Yöntem", reportLabel(emptyFallback(bridge.Method, "Yöntem yok"))},
		{"Formül", emptyFallback(bridge.Formula, "Formül açıklaması yok")},
		{"Fiyat / Baz değer", fmt.Sprintf("%.2f / %.2f %s | fark %.1f%%", bridge.CurrentPrice, bridge.BaseIntrinsicValue, displayCurrency(result.Currency), bridge.PriceToBasePct)},
		{"Güvenli alım eşiği", fmt.Sprintf("%.2f %s | gereken marj %.1f%%", bridge.BuyBelowPrice, displayCurrency(result.Currency), bridge.RequiredMarginPct)},
	}
	if bridge.NAVStatus != "" && bridge.NAVStatus != "not_applicable" {
		rows = append(rows, []string{"NAD durumu", reportLabel(bridge.NAVStatus)})
	}
	if bridge.NAVBridge.Status != "" && bridge.NAVBridge.Status != "not_applicable" {
		rows = append(rows, []string{"Kısmi NAD köprüsü", navBridgeText(bridge.NAVBridge, result.Currency)})
	}
	if len(bridge.PrimaryInputs) > 0 {
		rows = append(rows, []string{"Kullanılan girdiler", strings.Join(reportValuationInputLabels(bridge.PrimaryInputs), ", ")})
	}
	if len(bridge.MissingInputs) > 0 {
		rows = append(rows, []string{"Eksik ana girdiler", strings.Join(reportTexts(bridge.MissingInputs), " | ")})
	}
	if len(bridge.Limitations) > 0 {
		rows = append(rows, []string{"Model sınırı", reportText(strings.Join(bridge.Limitations, " "))})
	}
	return rows
}

func valuationTransparencyTitle(result analysis.SymbolAnalysis) string {
	bridge := result.Professional.InvestmentResearch.ValuationBridge
	if bridge.NAVStatus != "" && bridge.NAVStatus != "not_applicable" {
		return "GYO / Değerleme Şeffaflığı"
	}
	return "Değerleme Şeffaflığı"
}

func navBridgeText(bridge professional.NAVBridge, currency string) string {
	parts := []string{
		reportLabel(bridge.Status),
		"veri: " + reportLabel(bridge.DataQuality),
	}
	if bridge.SelectedPortfolioValueTRY > 0 {
		parts = append(parts, "portföy "+formatMoney(bridge.SelectedPortfolioValueTRY, currency))
	}
	if bridge.NetDebtTRY != 0 {
		parts = append(parts, "net borç "+formatMoney(bridge.NetDebtTRY, currency))
	}
	if bridge.EstimatedNAVTRY > 0 {
		parts = append(parts, "proxy NAD "+formatMoney(bridge.EstimatedNAVTRY, currency))
	}
	if bridge.EstimatedNAVPerShare > 0 {
		parts = append(parts, fmt.Sprintf("hisse başına %.2f %s", bridge.EstimatedNAVPerShare, currency))
	}
	if bridge.MarketCapToNAVPremiumPct != 0 {
		parts = append(parts, fmt.Sprintf("PD/NAD farkı %.1f%%", bridge.MarketCapToNAVPremiumPct))
	}
	return strings.Join(parts, " | ")
}

func assetDueDiligenceRows(review professional.InvestmentResearchReview) [][]string {
	assets := review.AssetDueDiligence
	isGYO := investmentResearchIsGYO(review)
	rows := [][]string{
		{"Envanter kapsamı", fmt.Sprintf("%d olay | %d ham satır | %d rapor satırı", assets.EventCount, assets.RawAssetCount, assets.DisplayAssetCount)},
		{"Değerlemeye bağlanma", reportLabel(assets.ValuationLinkedStatus)},
	}
	if isGYO {
		rows = append(rows,
			[]string{"Kira / proje", fmt.Sprintf("%d kira ilişkili varlık | %d proje", assets.RentalAssetCount, assets.ProjectCount)},
			[]string{"Portföy toplamı", fmt.Sprintf("tarihsel toplam satırı %d | güncel toplam var: %s", assets.PortfolioHistoryCount, localize.Bool(assets.PortfolioTotalsAvailable))},
		)
	}
	if len(assets.Findings) > 0 {
		rows = append(rows, []string{"Bulgular", strings.Join(assets.Findings, " ")})
	}
	if len(assets.RequiredChecks) > 0 {
		rows = append(rows, []string{"Yatırımcı için zorunlu kontroller", strings.Join(assets.RequiredChecks, " | ")})
	}
	return rows
}

func investmentResearchIsGYO(review professional.InvestmentResearchReview) bool {
	return review.ValuationBridge.NAVStatus != "" && review.ValuationBridge.NAVStatus != "not_applicable"
}

func financialQualityRows(result analysis.SymbolAnalysis) [][]string {
	rows := [][]string{}
	for _, item := range result.Professional.InvestmentResearch.FinancialQuality.Metrics {
		value := formatFinancialQualityMetric(item, result.Currency)
		rows = append(rows, []string{item.Name, value, reportLabel(item.Status), reportText(item.Comment)})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"Durum", "Yok", reportLabel("not_available"), "Finansal kalite köprüsü üretilemedi."})
	}
	return rows
}

func decisionFrameworkRows(review professional.InvestmentResearchReview) [][]string {
	framework := review.DecisionFramework
	rows := [][]string{
		{"AL'a dönebilmesi için", reportText(strings.Join(framework.BuyConditions, " | "))},
		{"BEKLE/Takip koşulu", reportText(strings.Join(framework.HoldConditions, " | "))},
		{"SAT/Risk azalt koşulu", reportText(strings.Join(framework.SellConditions, " | "))},
		{"Geçersiz kılan durumlar", reportText(strings.Join(framework.Invalidation, " | "))},
	}
	return rows
}

func technicalPriorityRows(result analysis.SymbolAnalysis) [][]string {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		keys := sortedTimeframeKeys(result.Timeframes)
		if len(keys) > 0 {
			tf = result.Timeframes[keys[0]]
			ok = true
		}
	}
	if !ok {
		return [][]string{{"Durum", "Teknik zaman dilimi bulunamadı."}}
	}
	score := tf.Professional.Technical.Score
	gate := tf.Professional.Technical.SignalGate
	rows := [][]string{
		{"Baskın teknik tablo", fmt.Sprintf("%s | teknik skor %.1f/100 | trend %.0f, fiyat ivmesi %.0f, hacim %.0f, oynaklık riski %.0f, formasyon %.0f", localize.Timeframe(tf.Timeframe), tf.Score, score.Trend, score.Momentum, score.Volume, score.VolatilityRisk, score.Pattern)},
	}
	if gate.Status != "" {
		rows = append(rows, []string{"Teknik sinyal kapısı", technicalSignalGateSummary(tf)})
	}
	if tf.Professional.Technical.Validation.Status != "" {
		rows = append(rows, []string{"İndikatör/formasyon doğrulaması", technicalValidationSummary(tf.Professional.Technical.Validation)})
	}
	indicators := []string{}
	for _, item := range tf.Professional.Technical.SelectedIndicators {
		if len(indicators) >= 6 {
			break
		}
		indicators = append(indicators, fmt.Sprintf("%s: %s (%.0f/100)", item.Name, reportLabel(item.Signal), item.Confidence*100))
	}
	if len(indicators) > 0 {
		rows = append(rows, []string{"Öncelikli indikatörler", strings.Join(indicators, " | ")})
	}
	patterns := []string{}
	for _, item := range tf.Professional.Technical.SelectedPatterns {
		if len(patterns) >= 5 {
			break
		}
		patterns = append(patterns, fmt.Sprintf("%s: %s (%.0f/100)", item.Name, reportLabel(item.Direction), item.Confidence*100))
	}
	if len(patterns) > 0 {
		rows = append(rows, []string{"Öncelikli formasyonlar", strings.Join(patterns, " | ")})
	}
	if len(tf.Professional.Technical.Guardrails) > 0 {
		rows = append(rows, []string{"Teknik kalite kapısı", strings.Join(reportLabels(tf.Professional.Technical.Guardrails), ", ")})
	}
	return rows
}

func formatFinancialQualityMetric(item professional.FinancialQualityMetric, currency string) string {
	switch item.Unit {
	case "TRY":
		return formatMoney(item.Value, currency)
	case "%":
		return formatPct(item.Value)
	case "x":
		return fmt.Sprintf("%.2fx", item.Value)
	default:
		return fmt.Sprintf("%.2f", item.Value)
	}
}

func limitStrings(items []string, limit int) []string {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func documentEvidenceSummary(result analysis.SymbolAnalysis) string {
	doc := result.Professional.ValueInvesting.DocumentEvidence
	if !doc.Computed {
		return emptyFallback(doc.Summary, "KAP PDF/ek kanıtı bulunamadı")
	}
	return doc.Summary
}

func documentEvidenceRows(result analysis.SymbolAnalysis, limit int) [][]string {
	docs := result.Professional.ValueInvesting.DocumentEvidence.KeyDocuments
	rows := make([][]string, 0, len(docs))
	for i, doc := range docs {
		if limit > 0 && i >= limit {
			break
		}
		date := ""
		if !doc.PublishedAt.IsZero() {
			date = doc.PublishedAt.Format("2006-01-02")
		} else if doc.Year > 0 {
			date = fmt.Sprintf("%d", doc.Year)
		}
		if doc.DisclosureYear > 0 && doc.DisclosureYear != doc.Year {
			date = strings.TrimSpace(date + " / dönem " + fmt.Sprintf("%d", doc.DisclosureYear))
		}
		rows = append(rows, []string{
			cleanDocumentName(doc.FileName),
			doc.CategoryLabel,
			documentPurposeText(doc.Purpose, date, doc.DisclosureTitle),
			documentContentText(doc),
			emptyFallback(doc.ReportImpact, joinStringEvidence(doc.Evidence, 2)),
		})
	}
	return rows
}

func kapPDFIngestRows(kap professional.KAPPDFIngestSummary) [][]string {
	rows := [][]string{
		{"İşlenen PDF", fmt.Sprintf("%d belge | %d işlenmiş kayıt | %d hata", kap.TotalDocuments, kap.UniqueProcessed, kap.ErrorCount)},
		{"Metin kalitesi", fmt.Sprintf("ortalama %.2f | analize uygun %d | analiz dışı %d | AI çözüm bekleyen %d | reddedilen %d | OCR %d", kap.AverageQuality, kap.AnalysisUsableCount, kap.ReviewRequiredCount, kapPDFReviewOnlyCount(kap), kap.RejectedCount, kap.OCRUsedCount)},
		{"Belge dağılımı", kapPDFTypeCountsText(kap.TypeCounts, 8)},
		{"Rapor etkisi", kap.Summary},
	}
	if kap.SourcePDFCount > 0 {
		rows = append([][]string{{
			"Kaynak PDF kapsamı",
			fmt.Sprintf("%d kaynak PDF | %d benzersiz dosya izi | %d tekrarlı dosya | %d rapor metni | %d eksik dosya izi",
				kap.SourcePDFCount,
				kap.SourceUniqueHashes,
				kap.DuplicatePDFCount,
				kap.TotalDocuments,
				kap.MissingUniqueHashes,
			),
		}}, rows...)
	}
	if kap.OutputDir != "" {
		rows = append(rows, []string{"İşlem çıktısı", kap.OutputDir})
	}
	return rows
}

func kapPDFIngestDocumentRows(kap professional.KAPPDFIngestSummary, limit int) [][]string {
	rows := make([][]string, 0, len(kap.ImportantDocuments))
	for i, doc := range kap.ImportantDocuments {
		if limit > 0 && i >= limit {
			break
		}
		quality := fmt.Sprintf("%.2f", doc.QualityScore)
		if len(doc.Warnings) > 0 {
			quality += " | " + strings.Join(doc.Warnings, ", ")
		}
		rows = append(rows, []string{
			cleanDocumentName(doc.FileName),
			emptyFallback(doc.DocumentLabel, doc.DocumentType),
			quality,
			emptyFallback(doc.ContentSnippet, "Metin çıkarıldı; seçili kısa snippet yok."),
		})
	}
	return rows
}

func kapPDFAllDocumentRows(kap professional.KAPPDFIngestSummary) [][]string {
	docs := kap.Documents
	if len(docs) == 0 {
		docs = kap.ImportantDocuments
	}
	rows := make([][]string, 0, len(docs))
	for _, doc := range docs {
		warnings := "-"
		if len(doc.Warnings) > 0 {
			warnings = strings.Join(doc.Warnings, ", ")
		}
		rows = append(rows, []string{
			cleanDocumentName(doc.FileName),
			emptyFallback(doc.DocumentLabel, doc.DocumentType),
			fmt.Sprintf("%.2f", doc.QualityScore),
			fmt.Sprintf("%d karakter", doc.TextLength),
			warnings,
		})
	}
	return rows
}

func kapAssetInventoryRows(inventory professional.KAPAssetInventorySummary) [][]string {
	rows := [][]string{
		{"Envanter kapsamı", fmt.Sprintf("%d varlık olayı | %d ham birleşik satır | %d raporda gösterilen varlık", inventory.EventCount, inventory.RawAssetCount, inventory.DisplayAssetCount)},
		{"Rapor etkisi", inventory.Summary},
		{"Kira/proje sayımı", fmt.Sprintf("%d kira ilişkili varlık | %d proje", inventory.TotalRentalAssets, inventory.TotalProjects)},
		{"Portföy toplamı", kapAssetPortfolioSummaryText(inventory.PortfolioSummary)},
		{"Endeksli değerleme", kapAssetValueIndexSummaryText(inventory.ValueIndex)},
		{"Tam envanter JSON", inventory.InventoryPath},
	}
	if inventory.EventCount > inventory.DisplayAssetCount && inventory.DisplayAssetCount > 0 {
		rows = append(rows, []string{"Gösterim notu", "HTML tablo birleşik varlık envanterini gösterir; ham olay kayıtları, kaynak snippet'leri ve tüm tarihçe tam envanter JSON içinde denetlenir."})
	}
	if len(inventory.Warnings) > 0 {
		rows = append(rows, []string{"Uyarılar", strings.Join(reportLabels(inventory.Warnings), ", ")})
	}
	return rows
}

func kapAssetInventoryItemRows(inventory professional.KAPAssetInventorySummary) [][]string {
	rows := make([][]string, 0, len(inventory.Assets))
	for _, asset := range inventory.Assets {
		rows = append(rows, []string{
			kapAssetNameCell(asset),
			kapAssetTypeLabel(asset.AssetType),
			kapAssetLocationText(asset),
			emptyFallback(asset.ParcelInfo, "Yok"),
			kapAssetAreaText(asset),
			kapAssetTypedValueText(asset.LatestExpertiseExclVAT),
			kapAssetTypedValueText(asset.LatestExpertiseInclVAT),
			kapAssetTypedValueText(asset.LatestBookValueTRY),
			kapAssetValueText(asset),
			kapAssetIndexedValueText(asset),
			kapAssetRentText(asset),
			emptyFallback(asset.LatestPeriod, emptyFallback(asset.ExpertiseDate, "Yok")),
			fmt.Sprintf("%.0f/100", asset.Confidence*100),
			kapAssetSourceText(asset),
		})
	}
	if len(rows) == 0 {
		return [][]string{{"Varlık satırı yok", "Yok", "Yok", "Yok", "Yok", "Yok", "Yok", "Yok", "Yok", "Yok", "Yok", "Yok", "0/100", "Yok"}}
	}
	return rows
}

func kapAssetInventoryPDFRows(inventory professional.KAPAssetInventorySummary) [][]string {
	rows := make([][]string, 0, len(inventory.Assets))
	for _, asset := range inventory.Assets {
		areaValue := kapAssetAreaText(asset)
		value := kapAssetValueText(asset)
		if value != "Yok" {
			if areaValue == "Yok" {
				areaValue = value
			} else {
				areaValue += " | " + value
			}
		}
		indexedValue := kapAssetIndexedValueText(asset)
		if indexedValue != "Yok" {
			areaValue = strings.TrimSpace(areaValue + " | güncel " + indexedValue)
		}
		rows = append(rows, []string{
			asset.AssetName,
			kapAssetTypeLabel(asset.AssetType),
			kapAssetLocationText(asset),
			areaValue,
			kapAssetRentText(asset),
			fmt.Sprintf("%.0f/100", asset.Confidence*100),
		})
	}
	if len(rows) == 0 {
		return [][]string{{"Güvenli varlık satırı yok", "Yok", "Yok", "Yok", "Yok", "0/100"}}
	}
	return rows
}

func kapRawDataSummaryRows(raw professional.KAPRawDataBundle) [][]string {
	rows := [][]string{
		{"Ham PDF dokümanı", fmt.Sprintf("%d", raw.Counts.RawDocuments)},
		{"Belge satırı / finansal satır adayı", fmt.Sprintf("%d / %d", raw.Counts.DocumentFacts, raw.Counts.FinancialFacts)},
		{"Finansal tablo bloğu", fmt.Sprintf("%d", raw.Counts.FinancialTables)},
		{"Kişi / ortaklık / kurumsal olay", fmt.Sprintf("%d / %d / %d", raw.Counts.People, raw.Counts.OwnershipFacts, raw.Counts.CorporateEvents)},
		{"Varlık olayı / envanter", fmt.Sprintf("%d / %d", raw.Counts.AssetEvents, raw.Counts.InventoryItems)},
		{"Veri çıkarma hatası", fmt.Sprintf("%d", raw.Counts.ExtractionErrors)},
	}
	if raw.KnowledgeGraph != nil {
		rows = append(rows,
			[]string{"Belge ilişki ağı", fmt.Sprintf("%d düğüm | %d ilişki | %d otomatik mutabakat", len(raw.KnowledgeGraph.Nodes), len(raw.KnowledgeGraph.Edges), len(raw.KnowledgeGraph.ResolvedContradictions))},
			[]string{"KAP sektörü", emptyFallback(raw.KnowledgeGraph.Sector.Sector, emptyFallback(raw.KnowledgeGraph.Sector.MainSector, "Yok"))},
		)
	}
	if raw.SourceFiles.FinancialTablesPath != "" {
		rows = append(rows, []string{"Finansal tablo kaynak bağlantısı", "Var"})
	}
	if raw.SourceFiles.KnowledgeGraphPath != "" {
		rows = append(rows, []string{"Belge ilişki ağı", "Var"})
	}
	return rows
}

func kapRawReportableCatalogRows(raw *professional.KAPRawDataBundle) [][]string {
	rows := [][]string{}
	for _, item := range kapReportableCategories(raw) {
		count := 0
		switch value := item["count"].(type) {
		case int:
			count = value
		case float64:
			count = int(value)
		}
		rows = append(rows, []string{
			reportLabel(fmt.Sprintf("%v", item["kind"])),
			fmt.Sprintf("%d", count),
			reportLabel(fmt.Sprintf("%v", item["analysis_role"])),
			reportLabel(fmt.Sprintf("%v", item["status"])),
			reportableSourceStatus(item["full_source_path"]),
			kapRawObservedFieldsText(item["observed_fields"]),
		})
	}
	return rows
}

func reportableSourceStatus(value any) string {
	text := strings.TrimSpace(fmt.Sprintf("%v", value))
	if text == "" || text == "<nil>" {
		return "Yok"
	}
	return "Bağlı"
}

func kapRawObservedFieldsText(value any) string {
	fields, ok := value.([]string)
	if !ok {
		if generic, ok := value.([]any); ok {
			for _, item := range generic {
				fields = append(fields, fmt.Sprint(item))
			}
		}
	}
	if len(fields) == 0 {
		return "Yok"
	}
	if len(fields) > 8 {
		fields = fields[:8]
	}
	for i, field := range fields {
		fields[i] = reportLabel(field)
	}
	return strings.Join(fields, ", ")
}

type kapCanonicalMetric struct {
	Key               string
	Label             string
	Statement         string
	Terms             []string
	PreferFirstAmount bool
}

type kapMetricPoint struct {
	Metric kapCanonicalMetric
	Fact   kapingest.ExtractedFinancialFact
	Period string
	Score  float64
}

type kapMetricComparison struct {
	Metric   kapCanonicalMetric
	Latest   *kapMetricPoint
	Previous *kapMetricPoint
	ByPeriod map[string]kapMetricPoint
}

func kapPDFFinancialMetricRows(raw *professional.KAPRawDataBundle, currency string) [][]string {
	rows := [][]string{}
	points := kapFinancialComparisonMap(raw)
	targetPeriod := kapPrimaryFinancialPeriod(points)
	for _, metric := range kapFinancialMetricAliases() {
		comp := points[metric.Key]
		if comp.Latest == nil {
			continue
		}
		latest := comp.Latest
		if targetPeriod != "" {
			if point := metricAtPeriod(points, metric.Key, targetPeriod); point != nil {
				latest = point
			} else {
				rows = append(rows, []string{
					metric.Label,
					reportLabel(metric.Statement),
					targetPeriod,
					"Eksik: bu döneme ait güvenilir ana tablo satırı bulunamadı",
					"Yok",
					"Hesaplanmadı",
					sourcePath(raw, "financial_facts"),
				})
				continue
			}
		}
		previousText := "Yok"
		changeText := "Yok"
		if previous := kapMetricPreviousForComparison(comp, latest); previous != nil {
			previousText = kapMetricValueText(previous.Fact, currency)
			changeText = kapMetricChangeText(kapMetricSignedValue(latest.Fact), kapMetricSignedValue(previous.Fact))
		}
		rows = append(rows, []string{
			metric.Label,
			reportLabel(firstNonEmptyReport(latest.Fact.StatementType, metric.Statement)),
			emptyFallback(latest.Period, "Dönem yok"),
			kapMetricValueText(latest.Fact, currency),
			previousText,
			changeText,
			kapRawSourceText(latest.Fact.SourceFile, latest.Fact.Source.Snippet),
		})
	}
	return rows
}

func kapPDFFinancialMetricPDFRows(raw *professional.KAPRawDataBundle, currency string) [][]string {
	fullRows := kapPDFFinancialMetricRows(raw, currency)
	rows := make([][]string, 0, len(fullRows))
	for _, row := range fullRows {
		if len(row) < 7 {
			continue
		}
		rows = append(rows, []string{row[0], row[2], row[3], row[5], row[6]})
	}
	return rows
}

func kapPDFFinancialReadingRows(raw *professional.KAPRawDataBundle, currency string, isBank bool) [][]string {
	points := kapFinancialComparisonMap(raw)
	rows := [][]string{}
	targetPeriod := kapPrimaryFinancialPeriod(points)
	add := func(title, reading, evidence string) {
		if strings.TrimSpace(reading) == "" {
			return
		}
		rows = append(rows, []string{title, reading, evidence})
	}
	if period, metrics, ok := metricsAtReportPeriod(points, targetPeriod, "total_assets", "equity"); ok {
		totalAssets := metrics["total_assets"]
		equity := metrics["equity"]
		totalAssetsValue := kapMetricSignedValue(totalAssets.Fact)
		equityValue := kapMetricSignedValue(equity.Fact)
		if totalAssetsValue > 0 && equityValue > 0 {
			ratio := equityValue / totalAssetsValue
			if ratio < 0 || ratio > 1.05 {
				add("Sermaye yapısı", kapFinancialInvalidRatioText("özkaynak/aktif", period, ratio), kapRawSourceText(equity.Fact.SourceFile, equity.Fact.Source.Snippet))
			} else {
				add("Sermaye yapısı", fmt.Sprintf("Özkaynak / aktif oranı %s. Dönem %s; toplam varlık %s, özkaynak %s.", formatPct(ratio*100), period, kapMetricValueText(totalAssets.Fact, currency), kapMetricValueText(equity.Fact, currency)), kapRawSourceText(equity.Fact.SourceFile, equity.Fact.Source.Snippet))
			}
		}
	} else if latestMetric(points, "total_assets") != nil || latestMetric(points, "equity") != nil {
		add("Sermaye yapısı", kapFinancialPeriodMismatchText(points, "total_assets", "equity"), sourcePath(raw, "financial_facts"))
	}
	if isBank {
		add("Banka metrik kapsamı", "Klasik FCF, net borç/özsermaye, FD/Satış ve FD/FAVÖK banka için ana metrik değildir. Tam banka raporu için SYR, çekirdek sermaye, NPL, NIM, LCR, kredi/mevduat, karşılık giderleri, mevduat maliyeti ve kur pozisyonu structured veri olarak bağlanmalıdır.", sourcePath(raw, "financial_facts"))
		if period, metrics, ok := metricsAtReportPeriod(points, targetPeriod, "loans", "deposits"); ok {
			loans := metrics["loans"]
			deposits := metrics["deposits"]
			depositsValue := kapMetricSignedValue(deposits.Fact)
			if depositsValue != 0 {
				ratio := kapMetricSignedValue(loans.Fact) / depositsValue
				add("Kredi / mevduat proxy", fmt.Sprintf("Kredi/mevduat proxy %.2fx. Dönem %s; bu yalnızca PDF finansal satırlarından türetilen sınırlı fonlama göstergesidir.", ratio, period), kapRawSourceText(loans.Fact.SourceFile, loans.Fact.Source.Snippet))
			}
		} else if latestMetric(points, "loans") != nil || latestMetric(points, "deposits") != nil {
			add("Kredi / mevduat proxy", kapFinancialPeriodMismatchText(points, "loans", "deposits"), sourcePath(raw, "financial_facts"))
		}
		if netIncome := latestMetric(points, "net_income"); netIncome != nil {
			add("Kâr kalitesi", fmt.Sprintf("Net dönem karı/zararı %s. Bankada bu satır net faiz geliri, ücret-komisyon geliri, ticari kâr/zarar ve karşılık giderleriyle köprülenmelidir.", kapMetricValueText(netIncome.Fact, currency)), kapRawSourceText(netIncome.Fact.SourceFile, netIncome.Fact.Source.Snippet))
		}
		return rows
	}
	if period, metrics, ok := metricsAtReportPeriod(points, targetPeriod, "cash", "financial_debt", "equity"); ok {
		cash := metrics["cash"]
		debt := metrics["financial_debt"]
		equity := metrics["equity"]
		cashValue := kapMetricSignedValue(cash.Fact)
		debtValue := kapMetricSignedValue(debt.Fact)
		equityValue := kapMetricSignedValue(equity.Fact)
		if equityValue != 0 {
			netDebt := debtValue - cashValue
			add("Borçluluk", fmt.Sprintf("Net borç = finansal borç - nakit; dönem %s için yaklaşık %s; net borç / özkaynak %.2fx. Brüt finansal borç %s, nakit %s.", period, formatMoney(netDebt, currency), netDebt/equityValue, kapMetricValueText(debt.Fact, currency), formatMoney(cashValue, currency)), kapRawSourceText(debt.Fact.SourceFile, debt.Fact.Source.Snippet))
		}
	} else if latestMetric(points, "financial_debt") != nil || latestMetric(points, "cash") != nil {
		add("Borçluluk", kapFinancialPeriodMismatchText(points, "cash", "financial_debt", "equity"), sourcePath(raw, "financial_facts"))
	}
	if period, metrics, ok := metricsAtReportPeriod(points, targetPeriod, "current_assets", "current_liabilities"); ok {
		currentAssets := metrics["current_assets"]
		currentLiabilities := metrics["current_liabilities"]
		currentAssetsValue := kapMetricSignedValue(currentAssets.Fact)
		currentLiabilitiesValue := kapMetricSignedValue(currentLiabilities.Fact)
		if currentLiabilitiesValue != 0 {
			add("Likidite", fmt.Sprintf("Cari oran yaklaşık %.2fx. Dönem %s; dönen varlık %s, kısa vadeli yükümlülük %s.", currentAssetsValue/currentLiabilitiesValue, period, kapMetricValueText(currentAssets.Fact, currency), kapMetricValueText(currentLiabilities.Fact, currency)), kapRawSourceText(currentAssets.Fact.SourceFile, currentAssets.Fact.Source.Snippet))
		}
	} else if latestMetric(points, "current_assets") != nil || latestMetric(points, "current_liabilities") != nil {
		add("Likidite", kapFinancialPeriodMismatchText(points, "current_assets", "current_liabilities"), sourcePath(raw, "financial_facts"))
	}
	if period, metrics, ok := metricsAtReportPeriod(points, targetPeriod, "current_liabilities", "non_current_liabilities", "equity"); ok {
		currentLiabilities := metrics["current_liabilities"]
		longLiabilities := metrics["non_current_liabilities"]
		equity := metrics["equity"]
		totalLiabilities := kapMetricSignedValue(currentLiabilities.Fact) + kapMetricSignedValue(longLiabilities.Fact)
		equityValue := kapMetricSignedValue(equity.Fact)
		if equityValue != 0 {
			add("Yükümlülük yapısı", fmt.Sprintf("Toplam yükümlülük yaklaşık %s; dönem %s için yükümlülük / özkaynak %.2fx.", formatMoney(totalLiabilities, currency), period, totalLiabilities/equityValue), kapRawSourceText(currentLiabilities.Fact.SourceFile, currentLiabilities.Fact.Source.Snippet))
		}
	} else if latestMetric(points, "current_liabilities") != nil || latestMetric(points, "non_current_liabilities") != nil || latestMetric(points, "equity") != nil {
		add("Yükümlülük yapısı", kapFinancialPeriodMismatchText(points, "current_liabilities", "non_current_liabilities", "equity"), sourcePath(raw, "financial_facts"))
	}
	if period, metrics, ok := metricsAtReportPeriod(points, targetPeriod, "revenue", "operating_profit"); ok {
		revenue := metrics["revenue"]
		opProfit := metrics["operating_profit"]
		revenueValue := kapMetricSignedValue(revenue.Fact)
		if revenueValue != 0 {
			margin := kapMetricSignedValue(opProfit.Fact) / revenueValue * 100
			add("Operasyonel karlılık", fmt.Sprintf("Faaliyet marjı %s. Dönem %s; hasılat %s, faaliyet karı/zararı %s.", formatPct(margin), period, kapMetricValueText(revenue.Fact, currency), kapMetricValueText(opProfit.Fact, currency)), kapRawSourceText(opProfit.Fact.SourceFile, opProfit.Fact.Source.Snippet))
		}
	} else if latestMetric(points, "revenue") != nil || latestMetric(points, "operating_profit") != nil {
		add("Operasyonel karlılık", kapFinancialPeriodMismatchText(points, "revenue", "operating_profit"), sourcePath(raw, "financial_facts"))
	}
	if period, metrics, ok := metricsAtReportPeriod(points, targetPeriod, "revenue", "net_income"); ok {
		revenue := metrics["revenue"]
		netIncome := metrics["net_income"]
		revenueValue := kapMetricSignedValue(revenue.Fact)
		if revenueValue != 0 {
			margin := kapMetricSignedValue(netIncome.Fact) / revenueValue * 100
			add("Net karlılık", fmt.Sprintf("Net marj %s. Dönem %s; net dönem karı/zararı %s.", formatPct(margin), period, kapMetricValueText(netIncome.Fact, currency)), kapRawSourceText(netIncome.Fact.SourceFile, netIncome.Fact.Source.Snippet))
		}
	} else if latestMetric(points, "revenue") != nil || latestMetric(points, "net_income") != nil {
		add("Net karlılık", kapFinancialPeriodMismatchText(points, "revenue", "net_income"), sourcePath(raw, "financial_facts"))
	}
	if revenueComp := points["revenue"]; revenueComp.Latest != nil {
		if previous := kapMetricPreviousForComparison(revenueComp, revenueComp.Latest); previous != nil {
			add("Hasılat trendi", fmt.Sprintf("Hasılat %s döneminden %s dönemine %s değişti.", previous.Period, revenueComp.Latest.Period, kapMetricChangeText(kapMetricSignedValue(revenueComp.Latest.Fact), kapMetricSignedValue(previous.Fact))), kapRawSourceText(revenueComp.Latest.Fact.SourceFile, revenueComp.Latest.Fact.Source.Snippet))
		}
	}
	if cfo, fcf := latestMetric(points, "operating_cash_flow"), latestMetric(points, "free_cash_flow"); cfo != nil || fcf != nil {
		parts := []string{}
		source := ""
		if cfo != nil {
			parts = append(parts, "operasyonel nakit akışı "+kapMetricValueText(cfo.Fact, currency))
			source = kapRawSourceText(cfo.Fact.SourceFile, cfo.Fact.Source.Snippet)
		}
		if fcf != nil {
			parts = append(parts, "serbest nakit akışı "+kapMetricValueText(fcf.Fact, currency))
			source = kapRawSourceText(fcf.Fact.SourceFile, fcf.Fact.Source.Snippet)
		}
		add("Nakit üretimi", strings.Join(parts, " | "), source)
	}
	if len(rows) == 0 {
		return [][]string{{"Durum", "PDF finansal fact satırları var ancak ana bilanço sözlüğüyle güvenilir eşleşme bulunamadı.", sourcePath(raw, "financial_facts")}}
	}
	return rows
}

func kapPDFCompanyInfoRows(result analysis.SymbolAnalysis) [][]string {
	raw := result.Professional.RawKAPData
	if raw == nil {
		return nil
	}
	rows := [][]string{}
	sector := firstNonEmptyReport(result.Professional.Company.Sector, result.Professional.Company.Industry)
	if raw.DocumentIndex != nil {
		sector = firstNonEmptyReport(raw.DocumentIndex.Sector.Sector, raw.DocumentIndex.Sector.MainSector, sector)
	}
	if raw.KnowledgeGraph != nil {
		sector = firstNonEmptyReport(raw.KnowledgeGraph.Sector.Sector, raw.KnowledgeGraph.Sector.MainSector, sector)
	}
	if sector != "" {
		rows = append(rows, []string{"KAP sektör bağlamı", sector, sourcePath(raw, "document_index")})
	}
	if raw.DocumentIndex != nil {
		reviewOnly := raw.DocumentIndex.Counts.ReviewRequiredDocs - raw.DocumentIndex.Counts.RejectedDocs
		if reviewOnly < 0 {
			reviewOnly = raw.DocumentIndex.Counts.ReviewRequiredDocs
		}
		rows = append(rows, []string{"Belge kapsamı", fmt.Sprintf("%d belge | %d analize uygun | %d analiz dışı (%d AI çözüm bekleyen, %d reddedilen)", raw.DocumentIndex.Counts.Documents, raw.DocumentIndex.Counts.AnalysisUsableDocs, raw.DocumentIndex.Counts.ReviewRequiredDocs, reviewOnly, raw.DocumentIndex.Counts.RejectedDocs), sourcePath(raw, "document_index")})
		models := kapBusinessModelText(raw, 8)
		if models != "" {
			rows = append(rows, []string{"İş modeli sinyalleri", models, sourcePath(raw, "document_index")})
		}
	}
	if len(raw.OwnershipFacts) > 0 {
		if isBankReport(result) {
			rows = append(rows, []string{"Ortaklık adayları", "PDF dipnotlarından çıkarılan pay/iştirak satırları resmi ISCTR ana ortaklık yapısı olarak kullanılmaz. Resmi ortaklık için KAP genel bilgi formu, MKK fiili dolaşım, yıllık faaliyet raporu ortaklık tablosu veya yatırımcı ilişkileri kaynağı gerekir.", sourcePath(raw, "ownership_facts")})
		} else {
			rows = append(rows, []string{"Ortaklık yapısı", kapOwnershipSummaryText(raw, 6), sourcePath(raw, "ownership_facts")})
		}
	}
	if len(raw.People) > 0 {
		rows = append(rows, []string{"Yönetim / kişiler", kapPeopleSummaryText(raw, 6), sourcePath(raw, "people")})
	}
	if len(raw.CorporateEvents) > 0 {
		rows = append(rows, []string{"KAP olayları", kapCorporateEventSummaryText(raw, 6), sourcePath(raw, "corporate_events")})
	}
	if len(raw.AssetEvents) > 0 {
		rows = append(rows, []string{"Varlık / tesis sinyalleri", kapAssetEventSummaryText(raw, 6), sourcePath(raw, "asset_events")})
	}
	if raw.KnowledgeGraph != nil {
		rows = append(rows, []string{"Çapraz belge mutabakatı", fmt.Sprintf("%d düğüm | %d ilişki | %d otomatik mutabakat", len(raw.KnowledgeGraph.Nodes), len(raw.KnowledgeGraph.Edges), len(raw.KnowledgeGraph.ResolvedContradictions)), sourcePath(raw, "knowledge_graph")})
	}
	return rows
}

func kapFinancialComparisonMap(raw *professional.KAPRawDataBundle) map[string]kapMetricComparison {
	out := map[string]kapMetricComparison{}
	for _, comp := range kapFinancialComparisons(raw) {
		out[comp.Metric.Key] = comp
	}
	return out
}

func latestMetric(points map[string]kapMetricComparison, key string) *kapMetricPoint {
	comp := points[key]
	return comp.Latest
}

func kapPrimaryFinancialPeriod(points map[string]kapMetricComparison) string {
	for _, keys := range [][]string{
		{"equity", "current_liabilities", "revenue", "net_income"},
		{"equity", "revenue"},
		{"equity"},
		{"revenue"},
	} {
		if period, ok := latestCommonMetricPeriod(points, keys...); ok {
			return period
		}
	}
	periods := []string{}
	for _, comp := range points {
		if comp.Latest != nil && strings.TrimSpace(comp.Latest.Period) != "" {
			periods = append(periods, strings.TrimSpace(comp.Latest.Period))
		}
	}
	sort.Strings(periods)
	if len(periods) == 0 {
		return ""
	}
	return periods[len(periods)-1]
}

func metricsAtReportPeriod(points map[string]kapMetricComparison, targetPeriod string, keys ...string) (string, map[string]*kapMetricPoint, bool) {
	targetPeriod = strings.TrimSpace(targetPeriod)
	if targetPeriod == "" {
		return metricsAtCommonPeriod(points, keys...)
	}
	out := map[string]*kapMetricPoint{}
	for _, key := range keys {
		point := metricAtPeriod(points, key, targetPeriod)
		if point == nil {
			return "", nil, false
		}
		out[key] = point
	}
	return targetPeriod, out, true
}

func latestCommonMetricPeriod(points map[string]kapMetricComparison, keys ...string) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}
	common := map[string]bool{}
	for idx, key := range keys {
		periods := kapMetricComparisonPeriods(points[key])
		if len(periods) == 0 {
			return "", false
		}
		if idx == 0 {
			for _, period := range periods {
				common[period] = true
			}
			continue
		}
		for period := range common {
			if !containsReportString(periods, period) {
				delete(common, period)
			}
		}
		if len(common) == 0 {
			return "", false
		}
	}
	periods := make([]string, 0, len(common))
	for period := range common {
		periods = append(periods, period)
	}
	sort.Strings(periods)
	if len(periods) == 0 {
		return "", false
	}
	return periods[len(periods)-1], true
}

func containsReportString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func kapMetricComparisonPeriods(comp kapMetricComparison) []string {
	seen := map[string]bool{}
	for period := range comp.ByPeriod {
		period = strings.TrimSpace(period)
		if period != "" {
			seen[period] = true
		}
	}
	for _, point := range []*kapMetricPoint{comp.Latest, comp.Previous} {
		if point == nil {
			continue
		}
		period := strings.TrimSpace(point.Period)
		if period != "" {
			seen[period] = true
		}
	}
	periods := make([]string, 0, len(seen))
	for period := range seen {
		periods = append(periods, period)
	}
	sort.Strings(periods)
	return periods
}

func metricAtPeriod(points map[string]kapMetricComparison, key, period string) *kapMetricPoint {
	comp := points[key]
	if comp.ByPeriod != nil {
		if point, ok := comp.ByPeriod[period]; ok {
			return &point
		}
	}
	for _, point := range []*kapMetricPoint{comp.Latest, comp.Previous} {
		if point != nil && point.Period == period {
			return point
		}
	}
	return nil
}

func kapMetricPreviousBeforePeriod(comp kapMetricComparison, period string) *kapMetricPoint {
	period = strings.TrimSpace(period)
	if period == "" {
		return comp.Previous
	}
	periods := kapMetricComparisonPeriods(comp)
	sort.Strings(periods)
	for i := len(periods) - 1; i >= 0; i-- {
		if periods[i] >= period {
			continue
		}
		if point, ok := comp.ByPeriod[periods[i]]; ok {
			return &point
		}
	}
	if comp.Previous != nil && comp.Previous.Period < period {
		return comp.Previous
	}
	return nil
}

func kapMetricPreviousForComparison(comp kapMetricComparison, latest *kapMetricPoint) *kapMetricPoint {
	if latest == nil {
		return nil
	}
	if kapMetricRequiresSameSeasonComparison(comp, latest.Fact) {
		return kapMetricSameSeasonPreviousYear(comp, latest.Period)
	}
	return kapMetricPreviousBeforePeriod(comp, latest.Period)
}

func kapMetricRequiresSameSeasonComparison(comp kapMetricComparison, fact kapingest.ExtractedFinancialFact) bool {
	statement := strings.ToLower(strings.TrimSpace(firstNonEmptyReport(fact.StatementType, comp.Metric.Statement)))
	return strings.Contains(statement, "income") ||
		strings.Contains(statement, "gelir") ||
		strings.Contains(statement, "cash_flow") ||
		strings.Contains(statement, "nakit")
}

func kapMetricSameSeasonPreviousYear(comp kapMetricComparison, period string) *kapMetricPoint {
	target := kapPreviousYearSameSeasonPeriod(period)
	if target == "" || comp.ByPeriod == nil {
		return nil
	}
	if point, ok := comp.ByPeriod[target]; ok {
		return &point
	}
	return nil
}

func kapPreviousYearSameSeasonPeriod(period string) string {
	period = strings.TrimSpace(period)
	if len(period) >= len("2006-01-02") {
		if parsed, err := time.Parse("2006-01-02", period[:len("2006-01-02")]); err == nil {
			return parsed.AddDate(-1, 0, 0).Format("2006-01-02")
		}
	}
	if len(period) == len("2006-Q1") && period[4] == '-' && (period[5] == 'Q' || period[5] == 'q') {
		year, err := strconv.Atoi(period[:4])
		if err == nil {
			return fmt.Sprintf("%04d-%s", year-1, period[5:])
		}
	}
	return ""
}

func metricsAtCommonPeriod(points map[string]kapMetricComparison, keys ...string) (string, map[string]*kapMetricPoint, bool) {
	period, ok := latestCommonMetricPeriod(points, keys...)
	if !ok {
		return "", nil, false
	}
	out := map[string]*kapMetricPoint{}
	for _, key := range keys {
		point := metricAtPeriod(points, key, period)
		if point == nil {
			return "", nil, false
		}
		out[key] = point
	}
	return period, out, true
}

func kapFinancialPeriodMismatchText(points map[string]kapMetricComparison, keys ...string) string {
	parts := []string{}
	for _, key := range keys {
		comp := points[key]
		label := firstNonEmptyReport(comp.Metric.Label, key)
		period := "yok"
		if comp.Latest != nil {
			period = emptyFallback(comp.Latest.Period, "dönem yok")
		}
		parts = append(parts, fmt.Sprintf("%s=%s", label, period))
	}
	target := kapPrimaryFinancialPeriod(points)
	targetText := ""
	if target != "" {
		targetText = " Rapor hedef dönemi: " + target + "."
	}
	return "Hesaplanmadı: bu oran için tüm kalemlerin aynı finansal dönemde olması zorunlu." + targetText + " Mevcut kalem dönemleri: " + strings.Join(parts, ", ") + ". Eksik veya dönem uyumsuz veri düzeltilmeden oran rapora alınmaz."
}

func kapFinancialInvalidRatioText(name, period string, ratio float64) string {
	return fmt.Sprintf("Hesaplanmadı: %s %.2fx çıktı ve finansal denetim sınırı dışında. Kaynak satırları aynı dönem (%s) olsa bile ana tablo satırı/dipnot eşleşmesi insan incelemesi gerektirir.", name, ratio, emptyFallback(period, "dönem yok"))
}

func kapFinancialComparisons(raw *professional.KAPRawDataBundle) []kapMetricComparison {
	if raw == nil {
		return nil
	}
	out := []kapMetricComparison{}
	reconciled := kapReconciledFinancialValueMap(raw)
	for _, metric := range kapFinancialMetricAliases() {
		byPeriod := map[string]kapMetricPoint{}
		for _, fact := range raw.FinancialFacts {
			if financialFactRejected(fact) {
				continue
			}
			if item, ok := reconciled[kapFinancialFactReconciliationKey(fact)]; ok {
				if !kapFinancialFactMatchesResolution(fact, item) {
					continue
				}
			}
			score, ok := kapFinancialMetricMatchScore(metric, fact)
			if !ok {
				continue
			}
			period := strings.TrimSpace(stringPtrValue(fact.Period))
			if period == "" {
				period = strings.TrimSpace(stringPtrValue(fact.DocumentDate))
			}
			if period == "" {
				continue
			}
			lineScore, ok := kapFinancialMetricLineQualityScore(metric, fact)
			if !ok {
				continue
			}
			score += lineScore + fact.Confidence
			if fact.ReviewRequired {
				score -= 0.5
			}
			if kapFinancialFactStructuredCertified(fact) {
				score += 20
			}
			if fact.Certification.AnalysisUsable {
				score += 0.25
			}
			if _, ok := reconciled[kapFinancialFactReconciliationKey(fact)]; ok {
				score += 10
			}
			current, exists := byPeriod[period]
			if !exists || kapFinancialMetricPointBetter(metric, fact, score, current) {
				byPeriod[period] = kapMetricPoint{Metric: metric, Fact: fact, Period: period, Score: score}
			}
		}
		if len(byPeriod) == 0 {
			continue
		}
		periods := make([]string, 0, len(byPeriod))
		for period := range byPeriod {
			periods = append(periods, period)
		}
		sort.Strings(periods)
		latest := byPeriod[periods[len(periods)-1]]
		comp := kapMetricComparison{Metric: metric, Latest: &latest, ByPeriod: byPeriod}
		if len(periods) > 1 {
			previous := byPeriod[periods[len(periods)-2]]
			comp.Previous = &previous
		}
		out = append(out, comp)
	}
	return out
}

func kapFinancialFactStructuredCertified(fact kapingest.ExtractedFinancialFact) bool {
	for _, reason := range fact.Certification.Reasons {
		if strings.EqualFold(strings.TrimSpace(reason), "structured_financials_bilanco_json") {
			return true
		}
	}
	return strings.Contains(normalizeFinancialText(fact.Source.Snippet), "structured financials bilanco json")
}

func kapReconciledFinancialValueMap(raw *professional.KAPRawDataBundle) map[string]kapingest.KnowledgeContradictionResolution {
	out := map[string]kapingest.KnowledgeContradictionResolution{}
	if raw == nil || raw.KnowledgeGraph == nil {
		return out
	}
	for _, item := range raw.KnowledgeGraph.ResolvedContradictions {
		if item.Type != "" && item.Type != "financial_value_conflict" {
			continue
		}
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		out[key] = item
	}
	return out
}

func kapFinancialFactReconciliationKey(fact kapingest.ExtractedFinancialFact) string {
	return strings.Join([]string{
		strings.TrimSpace(stringPtrValue(fact.Period)),
		strings.TrimSpace(fact.StatementType),
		strings.TrimSpace(fact.LineItemNormalized),
		strings.TrimSpace(fact.Currency),
		strings.TrimSpace(fact.Unit),
	}, "|")
}

func kapFinancialFactMatchesResolution(fact kapingest.ExtractedFinancialFact, item kapingest.KnowledgeContradictionResolution) bool {
	base := math.Max(math.Abs(fact.Value), math.Abs(item.SelectedValue))
	tolerance := math.Max(1, base*0.000001)
	return math.Abs(fact.Value-item.SelectedValue) <= tolerance
}

func kapFinancialMetricAliases() []kapCanonicalMetric {
	return []kapCanonicalMetric{
		{Key: "total_assets", Label: "Toplam varlıklar", Statement: "balance", Terms: []string{"total_assets", "toplam varlik", "toplam varlık", "toplam aktif"}, PreferFirstAmount: true},
		{Key: "current_assets", Label: "Dönen varlıklar", Statement: "balance", Terms: []string{"current_assets", "donen varlik", "dönen varlık"}, PreferFirstAmount: true},
		{Key: "cash", Label: "Nakit ve benzerleri", Statement: "balance", Terms: []string{"cash_and_cash_equivalents", "nakit ve nakit benzerleri", "nakit ve nakit benzer"}, PreferFirstAmount: true},
		{Key: "equity", Label: "Özkaynaklar", Statement: "balance", Terms: []string{"equity", "ozkaynak", "özkaynak", "ana ortaklığa ait özkaynak"}, PreferFirstAmount: true},
		{Key: "current_liabilities", Label: "Kısa vadeli yükümlülükler", Statement: "balance", Terms: []string{"current_liabilities", "kisa vadeli yukumluluk", "kısa vadeli yükümlülük"}, PreferFirstAmount: true},
		{Key: "non_current_liabilities", Label: "Uzun vadeli yükümlülükler", Statement: "balance", Terms: []string{"non_current_liabilities", "uzun vadeli yukumluluk", "uzun vadeli yükümlülük"}, PreferFirstAmount: true},
		{Key: "financial_debt", Label: "Finansal borç", Statement: "balance", Terms: []string{"financial_debt", "toplam finansal borc", "toplam finansal borç", "finansal borc", "finansal borç", "banka kredileri", "borrowings"}, PreferFirstAmount: true},
		{Key: "revenue", Label: "Hasılat", Statement: "income", Terms: []string{"revenue", "hasilat", "hasılat", "satis gelirleri", "satış gelirleri"}, PreferFirstAmount: true},
		{Key: "gross_profit", Label: "Brüt kar/zarar", Statement: "income", Terms: []string{"gross_profit", "brut kar", "brüt kar"}, PreferFirstAmount: true},
		{Key: "operating_profit", Label: "Faaliyet karı/zararı", Statement: "income", Terms: []string{"operating_profit", "faaliyet kari", "faaliyet karı", "esas faaliyet kari"}, PreferFirstAmount: true},
		{Key: "net_income", Label: "Net dönem karı/zararı", Statement: "income", Terms: []string{"net_income", "net donem kari", "net dönem karı", "donem kari zarari", "dönem karı zararı"}, PreferFirstAmount: true},
		{Key: "operating_cash_flow", Label: "Operasyonel nakit akışı", Statement: "cash", Terms: []string{"operating_cash_flow", "isletme faaliyetlerinden nakit akislari", "işletme faaliyetlerinden nakit akışları", "isletme faaliyetlerinden nakit akisi", "faaliyetlerden elde edilen nakit akislari", "faaliyetlerden elde edilen nakit"}, PreferFirstAmount: true},
		{Key: "free_cash_flow", Label: "Serbest nakit akışı", Statement: "cash", Terms: []string{"free_cash_flow", "serbest nakit akisi", "serbest nakit akışı", "serbest nakit"}, PreferFirstAmount: true},
	}
}

func kapFinancialMetricMatchScore(metric kapCanonicalMetric, fact kapingest.ExtractedFinancialFact) (float64, bool) {
	statement := normalizeFinancialText(fact.StatementType)
	if metric.Statement != "" && !strings.Contains(statement, normalizeFinancialText(metric.Statement)) {
		if !(metric.Statement == "cash" && strings.Contains(statement, "cash")) {
			return 0, false
		}
	}
	normalized := normalizeFinancialText(fact.LineItemNormalized)
	original := normalizeFinancialText(fact.LineItemOriginal)
	best := 0.0
	for _, term := range metric.Terms {
		key := normalizeFinancialText(term)
		switch {
		case normalized == key:
			best = math.Max(best, 4)
		case strings.Contains(normalized, key) || strings.Contains(key, normalized) && len(normalized) > 3:
			best = math.Max(best, 2.5)
		case strings.Contains(original, key):
			best = math.Max(best, 1.5)
		}
	}
	if best == 0 {
		return 0, false
	}
	return best, true
}

func kapFinancialMetricLineQualityScore(metric kapCanonicalMetric, fact kapingest.ExtractedFinancialFact) (float64, bool) {
	rawOriginal := strings.TrimSpace(fact.LineItemOriginal)
	original := normalizeFinancialText(fact.LineItemOriginal)
	normalized := normalizeFinancialText(fact.LineItemNormalized)
	snippet := normalizeFinancialText(fact.Source.Snippet)
	text := strings.TrimSpace(original + " " + normalized + " " + snippet)
	if strings.TrimSpace(text) == "" || fact.Value == 0 {
		return 0, false
	}
	if metric.Statement == "balance" && containsAnyFinancialText(text, []string{
		"nakit akislari", "nakit giris", "nakit cikis", "elden cikar", "alimindan kaynaklanan",
		"satisindan kaynaklanan", "duzeltmeler", "kayip kazanc",
	}) {
		return 0, false
	}
	score := 0.0
	switch metric.Key {
	case "total_assets":
		if !(normalized == "total assets" || strings.Contains(original, "toplam varlik") || strings.Contains(original, "toplam aktif")) {
			return 0, false
		}
		if kapFinancialLineLooksFormula(rawOriginal) {
			return 0, false
		}
		switch {
		case original == "toplam varliklar" || original == "toplam aktif":
			score += 4.2
		case strings.Contains(original, "toplam varlik") || strings.Contains(original, "toplam aktif"):
			score += 1.2
		}
	case "current_assets":
		if !(normalized == "current assets" || strings.Contains(original, "donen varlik")) {
			return 0, false
		}
		if containsAnyFinancialText(original, []string{"diger", "iliskili", "stok", "ticari", "sozlesme"}) || kapFinancialLineLooksFormula(rawOriginal) {
			return 0, false
		}
		if original == "donen varliklar" {
			score += 4.2
		}
	case "cash":
		if !(normalized == "cash and cash equivalents" || strings.Contains(original, "nakit ve nakit benzer")) {
			return 0, false
		}
		if containsAnyFinancialText(text, []string{"nakit akislari", "nakit akisi", "donem basi", "donem sonu", "net artis", "net azalis", "enflasyon"}) {
			return 0, false
		}
		if strings.Contains(original, "nakit ve nakit benzer") {
			score += 2
		}
	case "equity":
		if !(normalized == "equity" || strings.Contains(original, "ozkaynak")) {
			return 0, false
		}
		if containsAnyFinancialText(original, []string{"itibariyla", "raporlanan konsolide", "kayitli degerlere gore"}) {
			score -= 1
		}
		if original == "ozkaynaklar" {
			score += 4.2
		}
	case "current_liabilities":
		if !(normalized == "current liabilities" || strings.Contains(original, "kisa vadeli yukumluluk")) {
			return 0, false
		}
		if containsAnyFinancialText(original, []string{"diger", "iliskili", "musteri sozles", "parasal olan", "faaliyetlerle ilgili"}) || kapFinancialLineLooksFormula(rawOriginal) {
			return 0, false
		}
		if original == "kisa vadeli yukumlulukler" {
			score += 4.2
		}
	case "non_current_liabilities":
		if !(normalized == "non current liabilities" || strings.Contains(original, "uzun vadeli yukumluluk")) {
			return 0, false
		}
		if containsAnyFinancialText(original, []string{"diger", "iliskili", "musteri sozles", "parasal olan", "faaliyetlerle ilgili"}) || kapFinancialLineLooksFormula(rawOriginal) {
			return 0, false
		}
		if original == "uzun vadeli yukumlulukler" {
			score += 4.2
		}
	case "financial_debt":
		if containsAnyFinancialText(original, []string{"odenmesi", "azalis", "artis", "faiz", "gider"}) {
			return 0, false
		}
		switch {
		case original == "toplam finansal borclar" || original == "toplam finansal borc":
			score += 4
		case strings.Contains(original, "toplam finansal borc"):
			score += 2
		case containsAnyFinancialText(original, []string{"toplam kisa vadeli finansal borc", "toplam uzun vadeli finansal borc"}):
			score += 0.8
		}
	case "revenue":
		if containsAnyFinancialText(original, []string{"gelecek ay", "gelecek yil", "ertelenmis", "diger"}) {
			return 0, false
		}
		if kapFinancialFactValueParenthesized(fact) {
			return 0, false
		}
		if original == "hasilat" || original == "satis gelirleri" {
			score += 2
		}
	case "gross_profit":
		if original == "brut kar zarar" || strings.Contains(original, "brut kar") {
			score += 1.8
		}
	case "operating_profit":
		if containsAnyFinancialText(original, []string{"finansman gideri oncesi"}) {
			score -= 0.6
		}
		if strings.Contains(original, "faaliyet kari") || strings.Contains(original, "faaliyet kar zarar") {
			score += 1.8
		}
	case "net_income":
		if containsAnyFinancialText(original, []string{"kontrol gucu olmayan"}) {
			return 0, false
		}
		if containsAnyFinancialText(original, []string{"ana ortakliga ait", "surdurulen faaliyetler"}) {
			score -= 0.4
		}
		if strings.Contains(original, "net donem kari") || strings.Contains(original, "donem kari zarari") {
			score += 1.8
		}
	case "operating_cash_flow":
		if containsAnyFinancialText(original, []string{"yatirim faaliyet", "finansman faaliyet"}) {
			return 0, false
		}
		if strings.Contains(original, "isletme faaliyetlerinden") {
			score += 2
		}
	case "free_cash_flow":
		if !containsAnyFinancialText(text, []string{"serbest nakit", "free cash flow"}) {
			return 0, false
		}
	}
	numberCount := kapFinancialFactNumberCount(fact)
	numberIndex := kapFinancialFactNumberIndex(fact)
	if metric.PreferFirstAmount {
		switch {
		case numberIndex == 0:
			score += 0.9
		case numberIndex == 1:
			score += 0.25
		case numberIndex > 1:
			score -= 0.45
		}
	}
	if metric.Statement == "balance" {
		switch {
		case numberCount <= 1:
			score += 0.6
		case numberCount == 2:
			score += 0.2
		case numberCount > 3:
			score -= 0.8
		}
	}
	if score < -0.75 {
		return 0, false
	}
	return score, true
}

func kapFinancialMetricPointBetter(metric kapCanonicalMetric, candidate kapingest.ExtractedFinancialFact, candidateScore float64, current kapMetricPoint) bool {
	if candidateScore > current.Score+0.05 {
		return true
	}
	if candidateScore < current.Score-0.05 {
		return false
	}
	candidateIndex := kapFinancialFactNumberIndex(candidate)
	currentIndex := kapFinancialFactNumberIndex(current.Fact)
	if metric.PreferFirstAmount && candidateIndex >= 0 && (currentIndex < 0 || candidateIndex < currentIndex) {
		return true
	}
	if metric.PreferFirstAmount && candidateIndex >= 0 && candidateIndex == currentIndex && math.Abs(candidate.Value) > math.Abs(current.Fact.Value) {
		return true
	}
	return false
}

func kapFinancialLineLooksFormula(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	normalized := normalizeFinancialText(value)
	return strings.Contains(value, "+") ||
		strings.Contains(value, "=") ||
		strings.Contains(value, "(") ||
		strings.Contains(normalized, " 1 ") ||
		strings.Contains(normalized, " 2 ") ||
		strings.Contains(normalized, " 3 ") ||
		strings.Contains(normalized, " 4 ") ||
		strings.Contains(normalized, " 5 ") ||
		strings.Contains(normalized, " 6 ") ||
		strings.Contains(normalized, " 7 ") ||
		strings.Contains(normalized, " 8 ") ||
		strings.Contains(normalized, " 9 ")
}

func containsAnyFinancialText(value string, terms []string) bool {
	value = normalizeFinancialText(value)
	for _, term := range terms {
		if strings.Contains(value, normalizeFinancialText(term)) {
			return true
		}
	}
	return false
}

func kapFinancialFactNumberCount(fact kapingest.ExtractedFinancialFact) int {
	return len(kapFinancialNumberTokens(fact.Source.Snippet))
}

func kapFinancialFactNumberIndex(fact kapingest.ExtractedFinancialFact) int {
	want := digitsOnly(fmt.Sprintf("%.0f", math.Abs(fact.Value)))
	if want == "" {
		return -1
	}
	for idx, token := range kapFinancialNumberTokens(fact.Source.Snippet) {
		if token == want {
			return idx
		}
	}
	return -1
}

func kapFinancialNumberTokens(snippet string) []string {
	tokens := []string{}
	for _, field := range strings.Fields(snippet) {
		digits := digitsOnly(field)
		if len(digits) < 4 {
			continue
		}
		tokens = append(tokens, digits)
	}
	return tokens
}

func digitsOnly(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func financialFactRejected(fact kapingest.ExtractedFinancialFact) bool {
	return strings.EqualFold(fact.Certification.Status, "rejected")
}

func normalizeFinancialText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"ı", "i", "İ", "i", "ğ", "g", "ü", "u", "ş", "s", "ö", "o", "ç", "c",
		"_", " ", "-", " ", "’", " ", "'", " ", "\"", " ", ".", " ", ",", " ",
		";", " ", ":", " ", "/", " ", "\\", " ", "(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
		"+", " ", "%", " ",
	)
	value = replacer.Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func kapMetricValueText(fact kapingest.ExtractedFinancialFact, currency string) string {
	unitCurrency := kapMetricCurrency(fact, currency)
	text := formatMoney(kapMetricSignedValue(fact), unitCurrency)
	if strings.TrimSpace(fact.Unit) != "" && !strings.EqualFold(fact.Unit, "unit") {
		text += " (kaynak birim: " + fact.Unit + ")"
	}
	return text
}

func kapMetricSignedValue(fact kapingest.ExtractedFinancialFact) float64 {
	if fact.Value == 0 {
		return 0
	}
	value := math.Abs(fact.Value) * kapMetricUnitMultiplier(fact.Unit)
	if kapFinancialFactValueParenthesized(fact) {
		return -value
	}
	if fact.Value < 0 {
		return -value
	}
	return value
}

func kapMetricCurrency(fact kapingest.ExtractedFinancialFact, fallback string) string {
	unit := strings.ToLower(strings.TrimSpace(fact.Unit))
	switch {
	case strings.Contains(unit, "try") || strings.Contains(unit, "tl") || strings.Contains(unit, "turk"):
		return "TRY"
	case strings.Contains(unit, "usd") || strings.Contains(unit, "dolar"):
		return "USD"
	case strings.Contains(unit, "eur") || strings.Contains(unit, "euro"):
		return "EUR"
	default:
		return firstNonEmptyReport(fact.Currency, fallback)
	}
}

func kapMetricUnitMultiplier(unit string) float64 {
	normalized := normalizeFinancialText(unit)
	switch {
	case normalized == "":
		return 1
	case strings.Contains(normalized, "thousand") || strings.Contains(normalized, "bin"):
		return 1_000
	case strings.Contains(normalized, "million") || strings.Contains(normalized, "milyon"):
		return 1_000_000
	case strings.Contains(normalized, "billion") || strings.Contains(normalized, "milyar"):
		return 1_000_000_000
	default:
		return 1
	}
}

func kapFinancialFactValueParenthesized(fact kapingest.ExtractedFinancialFact) bool {
	want := digitsOnly(fmt.Sprintf("%.0f", math.Abs(fact.Value)))
	if want == "" {
		return false
	}
	for _, field := range strings.Fields(fact.Source.Snippet) {
		if digitsOnly(field) == want && strings.Contains(field, "(") {
			return true
		}
	}
	return false
}

func kapMetricChangeText(latest, previous float64) string {
	if previous == 0 {
		return "Önceki dönem 0; yüzde değişim hesaplanmadı"
	}
	change := (latest - previous) / math.Abs(previous) * 100
	return formatPct(change)
}

func kapBusinessModelText(raw *professional.KAPRawDataBundle, limit int) string {
	counts := map[string]int{}
	if raw == nil || raw.DocumentIndex == nil {
		return ""
	}
	sector := firstNonEmptyReport(raw.DocumentIndex.Sector.Sector, raw.DocumentIndex.Sector.MainSector)
	for _, doc := range raw.DocumentIndex.Documents {
		for _, tag := range doc.BusinessModels {
			if tag.ReviewRequired || tag.Confidence < 0.5 {
				continue
			}
			if !kapBusinessModelReportableForSector(sector, tag.Tag) {
				continue
			}
			incReportCount(counts, tag.Tag)
		}
	}
	return reportCountText(counts, limit)
}

func kapBusinessModelReportableForSector(sector, tag string) bool {
	sector = strings.ToLower(strings.TrimSpace(sector))
	if !strings.Contains(sector, "savunma") {
		return true
	}
	switch strings.TrimSpace(tag) {
	case "defense", "industrial_manufacturing", "machinery_electrical_equipment", "engineering_architecture", "r_and_d", "technology_services", "software", "aviation", "infrastructure":
		return true
	default:
		return false
	}
}

func kapOwnershipSummaryText(raw *professional.KAPRawDataBundle, limit int) string {
	counts := map[string]int{}
	for _, fact := range raw.OwnershipFacts {
		if !kapOwnershipFactReportable(fact) {
			continue
		}
		label := fact.HolderName
		if fact.ShareRatio != nil {
			label += " " + kapRawOptionalPercent(fact.ShareRatio)
		}
		incReportCount(counts, label)
	}
	if len(counts) == 0 {
		return "Doğrulanmış ortaklık satırı yok; resmi ortaklık için KAP genel bilgi formu, faaliyet raporu ortaklık tablosu, MKK/VAP veya yatırımcı ilişkileri kaynağı gerekir."
	}
	return reportCountText(counts, limit)
}

func kapPeopleSummaryText(raw *professional.KAPRawDataBundle, limit int) string {
	counts := map[string]int{}
	for _, person := range raw.People {
		if person.ReviewRequired || person.Confidence < 0.75 {
			continue
		}
		name := firstNonEmptyReport(person.FullName, person.NormalizedName)
		if !kapPersonNameReportable(name) {
			continue
		}
		label := name
		if person.Role != "" {
			label += " / " + reportLabel(person.Role)
		}
		incReportCount(counts, label)
	}
	return reportCountText(counts, limit)
}

func kapCorporateEventSummaryText(raw *professional.KAPRawDataBundle, limit int) string {
	counts := map[string]int{}
	for _, event := range raw.CorporateEvents {
		if event.ReviewRequired || event.Confidence < 0.75 {
			continue
		}
		label := firstNonEmptyReport(event.EventType, "corporate_event")
		if event.Title != "" {
			label += ": " + shortReportText(event.Title, 80)
		}
		incReportCount(counts, label)
	}
	return reportCountText(counts, limit)
}

func kapAssetEventSummaryText(raw *professional.KAPRawDataBundle, limit int) string {
	counts := map[string]int{}
	for _, event := range raw.AssetEvents {
		if event.Confidence < 0.7 {
			continue
		}
		label := firstNonEmptyReport(event.AssetName, event.AssetType, "asset_event")
		if event.Location != "" {
			label += " / " + event.Location
		}
		incReportCount(counts, label)
	}
	return reportCountText(counts, limit)
}

func incReportCount(counts map[string]int, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	counts[key]++
}

func reportCountText(counts map[string]int, limit int) string {
	type item struct {
		Key   string
		Count int
	}
	items := make([]item, 0, len(counts))
	for key, count := range counts {
		items = append(items, item{Key: key, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Key < items[j].Key
		}
		return items[i].Count > items[j].Count
	})
	parts := []string{}
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%d)", item.Key, item.Count))
	}
	return strings.Join(parts, " | ")
}

type kapRawGroupedRow struct {
	Key        string
	Cells      []string
	Count      int
	Period     string
	Confidence float64
	Source     string
}

func kapRawFinancialTableRows(raw professional.KAPRawDataBundle, limit int) [][]string {
	grouped := map[string]kapRawGroupedRow{}
	for _, table := range raw.FinancialTables {
		key := emptyFallback(table.TableType, "unknown")
		item := grouped[key]
		item.Key = key
		item.Count++
		period := kapRawPeriod(table.Period)
		if period > item.Period {
			item.Period = period
		}
		if table.Confidence >= item.Confidence {
			item.Confidence = table.Confidence
			item.Source = kapRawSourceText(table.SourceFile, table.Source.Snippet)
		}
		grouped[key] = item
	}
	items := kapRawSortedGroupedRows(grouped)
	rows := [][]string{}
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		rows = append(rows, []string{reportLabel(item.Key), fmt.Sprintf("%d", item.Count), emptyFallback(item.Period, "Yok"), item.Source})
	}
	return rows
}

func kapRawFinancialFactRows(raw professional.KAPRawDataBundle, limit int) [][]string {
	grouped := map[string]kapRawGroupedRow{}
	for _, fact := range raw.FinancialFacts {
		if strings.EqualFold(fact.Certification.Status, "rejected") {
			continue
		}
		label := emptyFallback(fact.LineItemNormalized, fact.LineItemOriginal)
		if label == "" {
			continue
		}
		period := kapRawPeriod(fact.Period)
		key := strings.Join([]string{period, fact.StatementType, label}, "|")
		item := grouped[key]
		item.Key = key
		item.Count++
		item.Period = period
		if fact.Confidence >= item.Confidence {
			item.Confidence = fact.Confidence
			value := formatReportNumber(fact.Value)
			if fact.Currency != "" {
				value += " " + displayCurrency(fact.Currency)
			}
			if fact.Unit != "" {
				value += " " + fact.Unit
			}
			item.Cells = []string{
				reportLabel(label),
				reportLabel(fact.StatementType),
				value,
				emptyFallback(period, "Yok"),
				fmt.Sprintf("%.0f/100", fact.Confidence*100),
				kapRawSourceText(fact.SourceFile, fact.Source.Snippet),
			}
		}
		grouped[key] = item
	}
	items := kapRawSortedFinancialFactRows(grouped)
	rows := [][]string{}
	for _, item := range items {
		if len(item.Cells) == 0 {
			continue
		}
		rows = append(rows, item.Cells)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func kapRawPeopleRows(raw professional.KAPRawDataBundle, limit int) [][]string {
	grouped := map[string]kapRawGroupedRow{}
	preferredNames := kapPreferredPersonNames(raw.People)
	for _, person := range raw.People {
		if person.ReviewRequired || person.Confidence < 0.86 {
			continue
		}
		name := emptyFallback(person.FullName, person.NormalizedName)
		if !kapPersonNameReportable(name) {
			continue
		}
		if preferred := preferredNames[kapPersonPrefixKey(name)]; preferred != "" && !strings.EqualFold(preferred, name) && len(strings.Fields(preferred)) > len(strings.Fields(name)) {
			continue
		}
		key := strings.ToLower(person.NormalizedName + "|" + person.Role + "|" + person.Title)
		item := grouped[key]
		item.Key = key
		item.Count++
		period := kapRawPeriod(person.Period)
		if period > item.Period {
			item.Period = period
		}
		if person.Confidence >= item.Confidence {
			item.Confidence = person.Confidence
			item.Cells = []string{
				name,
				reportLabel(person.Role),
				emptyFallback(person.Title, "Yok"),
				fmt.Sprintf("%d", item.Count),
				emptyFallback(period, "Yok"),
				fmt.Sprintf("%.0f/100", person.Confidence*100),
			}
		}
		grouped[key] = item
	}
	items := kapRawSortedGroupedRows(grouped)
	minCount := 1
	if len(grouped) > 20 {
		minCount = 4
	}
	rows := [][]string{}
	for _, item := range items {
		if len(item.Cells) == 0 {
			continue
		}
		if item.Count < minCount {
			continue
		}
		item.Cells[3] = fmt.Sprintf("%d", item.Count)
		item.Cells[4] = emptyFallback(item.Period, item.Cells[4])
		rows = append(rows, item.Cells)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func kapPreferredPersonNames(people []kapingest.ExtractedPerson) map[string]string {
	preferred := map[string]string{}
	for _, person := range people {
		name := emptyFallback(person.FullName, person.NormalizedName)
		if !kapPersonNameReportable(name) {
			continue
		}
		parts := strings.Fields(name)
		if len(parts) < 3 {
			continue
		}
		key := kapPersonPrefixKey(name)
		existing := preferred[key]
		if existing == "" || len([]rune(name)) > len([]rune(existing)) {
			preferred[key] = name
		}
	}
	return preferred
}

func kapPersonPrefixKey(name string) string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(name)))
	if len(parts) < 2 {
		return strings.Join(parts, " ")
	}
	return parts[0] + " " + parts[1]
}

func kapPersonNameReportable(name string) bool {
	name = strings.TrimSpace(name)
	parts := strings.Fields(name)
	if len(parts) < 2 || len(parts) > 4 {
		return false
	}
	normalized := " " + normalizeFinancialText(name) + " "
	for _, token := range []string{
		" a s ", " akademisi ", " aldigi ", " anonim ", " ar ge ", " arabian ",
		" arastirma ", " aselsan ", " asosye ", " bank ", " bilim ", " birligi ",
		" bites ", " company ", " defense ", " destek ", " dis ", " egitim ",
		" elektronik ", " electronics ", " engineers ", " espor ", " faaliyet ",
		" federasyonu ", " finans ", " futbol ", " ge ", " gelistirme ",
		" global ", " guclendirme ", " haberlesme ", " hassas ", " havacilik ",
		" hava ", " haziran ", " hizmet ", " hizlandirici ", " holding ",
		" hukuk ", " ihtisas ", " ihracatcilari ", " ieee ", " ilac ",
		" islam ", " isletme ", " katar ", " kulub ", " laboratuvari ",
		" lcc ", " limited ", " llc ", " ltd ", " makina ", " mart ",
		" mayis ", " meos ", " mikroelektronik ", " motor ", " mudurlugu ",
		" muhendisligi ", " musavirligi ", " nisan ", " optik ", " organize ",
		" ogretim ", " polonya ", " psc ", " qstp ", " san ", " saudi ",
		" savunma ", " sektor ", " silah ", " silahli ", " sivas ", " sportif ",
		" sube ", " subesi ", " sirketi ", " tarla ", " tasarim ", " tedarik ",
		" teknik ", " teknoloji ", " teknolojileri ", " tubitak ", " turk ",
		" universite ", " universitesi ", " vakfi ", " yardimciligi ", " yazilim ",
		" yer ", " yonetimi ", " yurutme ", " zinciri ",
	} {
		if strings.Contains(normalized, token) {
			return false
		}
	}
	for _, part := range parts {
		if len([]rune(strings.Trim(part, ".,;:()[]{}'’"))) < 2 {
			return false
		}
	}
	return true
}

func kapRawOwnershipRows(raw professional.KAPRawDataBundle, limit int) [][]string {
	grouped := map[string]kapRawGroupedRow{}
	for _, fact := range raw.OwnershipFacts {
		if !kapOwnershipFactReportable(fact) {
			continue
		}
		holder := strings.TrimSpace(fact.HolderName)
		if holder == "" {
			continue
		}
		key := strings.ToLower(holder)
		item := grouped[key]
		item.Key = key
		item.Count++
		period := kapRawPeriod(fact.Period)
		if period > item.Period {
			item.Period = period
		}
		if fact.Confidence >= item.Confidence {
			item.Confidence = fact.Confidence
			item.Cells = []string{
				holder,
				kapRawOptionalPercent(fact.ShareRatio),
				kapRawOptionalNumber(fact.ShareAmount),
				fmt.Sprintf("%d", item.Count),
				emptyFallback(period, "Yok"),
				kapRawSourceText(fact.SourceFile, fact.Source.Snippet),
			}
		}
		grouped[key] = item
	}
	items := kapRawSortedGroupedRows(grouped)
	rows := [][]string{}
	for _, item := range items {
		if len(item.Cells) == 0 {
			continue
		}
		item.Cells[3] = fmt.Sprintf("%d", item.Count)
		item.Cells[4] = emptyFallback(item.Period, item.Cells[4])
		rows = append(rows, item.Cells)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func kapOwnershipFactReportable(fact kapingest.OwnershipFact) bool {
	holder := strings.TrimSpace(fact.HolderName)
	if holder == "" || fact.ReviewRequired || (fact.Confidence > 0 && fact.Confidence < 0.72) {
		return false
	}
	if fact.ShareRatio == nil && fact.ShareAmount == nil {
		return false
	}
	if fact.ShareRatio != nil && (*fact.ShareRatio < 0 || *fact.ShareRatio > 100) {
		return false
	}
	if !kapOwnershipHolderNameReportable(holder) {
		return false
	}
	text := normalizeFinancialText(holder + " " + fact.Source.Snippet)
	if !kapOwnershipContextReportable(text) {
		return false
	}
	blocked := []string{
		"bir savunma", "sirketine donusum", "donusum", "bu surec", "sirketin finansal",
		"teknoloji savunma ve guvenlik", "sirketidir", "yas", "faaliyet raporu", "finansal tablolar",
		"performansini etkileyen ana", "sirket in performansini", "teknolojiler alanina giris",
		"mali tablolarinda", "yonetim kurulunun", "sirket islerine", "butcelenmis kar",
		"kar dagitimi karari", "rekabet sac ayak", "surdurulebilir buyume", "musterilerle urun",
	}
	return !containsAnyFinancialText(text, blocked)
}

func kapOwnershipContextReportable(text string) bool {
	if containsAnyFinancialText(text, []string{
		"ortaklik yapisi", "ortaklar", "pay sahibi", "pay sahip", "sermaye ve ortaklik",
		"sermaye yapisi", "sermayesindeki pay", "hissedarlik", "halka acik kismini",
		"halka aciklik", "fiili dolasim", "tsk gv", "tskgv", "vakfi", "vakfı",
	}) {
		return true
	}
	return containsAnyFinancialText(text, []string{
		"emeklilik fonu", "yatirim fonu", "holding", "anonim sirketi",
		"limited sirketi", "bankasi", "varlik fonu", "kamu", "vakif",
	})
}

func kapOwnershipHolderNameReportable(holder string) bool {
	holder = strings.TrimSpace(holder)
	if holder == "" {
		return false
	}
	parts := strings.Fields(holder)
	if len(parts) > 7 {
		return false
	}
	if strings.Contains(holder, "+") {
		return false
	}
	if len(parts) > 0 {
		first := strings.Trim(parts[0], ".,;:()[]{}'’\"")
		if first != "" && first[0] >= '0' && first[0] <= '9' {
			return false
		}
	}
	hasLetter := false
	for _, r := range holder {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || strings.ContainsRune("ÇĞİÖŞÜçğıöşü", r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}
	lower := normalizeFinancialText(holder)
	if kapRawDigitCount(lower) > 0 && len([]rune(holder)) <= 8 {
		return false
	}
	for _, blocked := range []string{
		"yas", "year", "age", "surec", "donusum", "finansal", "teknoloji savunma",
		"guvenlik sirketi", "rapor", "tablo", "faaliyet", "sonucunda", "performans",
		"etkileyen", "mali tablo", "yonetim kurulu", "sirket isleri", "sirketin",
		"sirket in", "giris", "butcelenmis", "kar dagitimi",
	} {
		if strings.Contains(lower, blocked) {
			return false
		}
	}
	return true
}

func kapRawCorporateEventRows(raw professional.KAPRawDataBundle, limit int) [][]string {
	grouped := map[string]kapRawGroupedRow{}
	for _, event := range raw.CorporateEvents {
		if event.ReviewRequired || event.Confidence < 0.86 {
			continue
		}
		eventType := emptyFallback(event.EventType, "corporate_event")
		title := emptyFallback(event.Title, eventType)
		if !kapRawCorporateEventReportable(eventType, title) {
			continue
		}
		key := strings.ToLower(eventType + "|" + title)
		item := grouped[key]
		item.Key = key
		item.Count++
		period := kapRawPeriod(event.Period)
		if period > item.Period {
			item.Period = period
		}
		if event.Confidence >= item.Confidence {
			item.Confidence = event.Confidence
			amount := "Yok"
			if event.Amount != nil {
				amount = formatReportNumber(*event.Amount)
				if event.Currency != "" {
					amount += " " + displayCurrency(event.Currency)
				}
			}
			item.Cells = []string{
				reportLabel(eventType),
				shortReportText(title, 180),
				amount,
				fmt.Sprintf("%d", item.Count),
				emptyFallback(period, "Yok"),
				kapRawSourceText(event.SourceFile, event.Source.Snippet),
			}
		}
		grouped[key] = item
	}
	items := kapRawSortedGroupedRows(grouped)
	rows := [][]string{}
	for _, item := range items {
		if len(item.Cells) == 0 {
			continue
		}
		item.Cells[3] = fmt.Sprintf("%d", item.Count)
		item.Cells[4] = emptyFallback(item.Period, item.Cells[4])
		rows = append(rows, item.Cells)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func kapRawCorporateEventReportable(eventType, title string) bool {
	lower := strings.ToLower(title)
	for _, blocked := range []string{
		"iştirakler ve/veya iş ortaklıkları pay alımı",
		"sermaye artırımı sebebiyle oluşan",
		"maddi duran varlık satışı",
		"yatırım amaçlı gayrimenkul satış kârı",
		"emisyon priminden",
		"hisse senetleri ihraç primleri",
		"finansal tablolarında yer alan",
		"finansal tablolarda yer alan",
	} {
		if strings.Contains(lower, blocked) {
			return false
		}
	}
	if strings.Contains(strings.ToLower(eventType), "capital") && strings.HasPrefix(lower, "sermaye artırımı ") && kapRawDigitCount(title) >= 12 {
		return false
	}
	return true
}

func kapRawDigitCount(value string) int {
	count := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			count++
		}
	}
	return count
}

func kapRawKnowledgeGraphRows(raw professional.KAPRawDataBundle, limit int) [][]string {
	graph := raw.KnowledgeGraph
	if graph == nil {
		return nil
	}
	rows := [][]string{
		{"Düğüm", fmt.Sprintf("%d", len(graph.Nodes)), "Şirket, belge, sektör, iş modeli, finansal kalem, kişi, ortak ve olay düğümleri"},
		{"İlişki", fmt.Sprintf("%d", len(graph.Edges)), "Belge-kanıt, şirket-finansal satır ve kişi/ortak/olay ilişkileri"},
		{"Veri mutabakatı", fmt.Sprintf("%d değer farkı otomatik çözüldü", len(graph.ResolvedContradictions)), "Aynı dönem/kalem için farklı adaylar varsa sistem kaynak önceliği, belge tarihi, sertifika, tablo kanıtı ve güven puanına göre tek değer seçer."},
		{"Tekrarlı finansal satır", fmt.Sprintf("%d", graph.DuplicateMergeSummary.DuplicateFactRows), "Aynı/benzer kaynak satır tekrarları; ham veri silinmeden işaretlenir"},
	}
	for i, item := range graph.ResolvedContradictions {
		if limit > 0 && i >= limit {
			break
		}
		rows = append(rows, []string{
			"Otomatik çözülen değer farkı",
			kapResolvedContradictionText(item),
			reportText(item.Reason),
		})
	}
	return rows
}

func kapResolvedContradictionText(item kapingest.KnowledgeContradictionResolution) string {
	value := formatReportNumber(item.SelectedValue)
	if item.Currency != "" {
		value += " " + displayCurrency(item.Currency)
	}
	if item.Unit != "" {
		value += " " + item.Unit
	}
	if item.SelectedSourceFile != "" {
		value += " | seçilen kaynak: " + cleanDocumentName(item.SelectedSourceFile)
	}
	if item.Confidence > 0 {
		value += fmt.Sprintf(" | güven %.0f/100", item.Confidence*100)
	}
	return value
}

func kapRawSortedGroupedRows(grouped map[string]kapRawGroupedRow) []kapRawGroupedRow {
	items := make([]kapRawGroupedRow, 0, len(grouped))
	for _, item := range grouped {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		if items[i].Period != items[j].Period {
			return items[i].Period > items[j].Period
		}
		return items[i].Key < items[j].Key
	})
	return items
}

func kapRawSortedFinancialFactRows(grouped map[string]kapRawGroupedRow) []kapRawGroupedRow {
	items := make([]kapRawGroupedRow, 0, len(grouped))
	for _, item := range grouped {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		pi := kapRawFinancialFactPriority(items[i].Key)
		pj := kapRawFinancialFactPriority(items[j].Key)
		if pi != pj {
			return pi > pj
		}
		if items[i].Period != items[j].Period {
			return items[i].Period > items[j].Period
		}
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Key < items[j].Key
	})
	return items
}

func kapRawFinancialFactPriority(key string) int {
	slug := strings.ToLower(key)
	priority := 0
	for _, term := range []string{"toplam varlik", "ozkaynak", "nakit", "finansal borc", "net borc", "hasilat", "brut kar", "faaliyet kari", "net donem kari", "yatirim amacli gayrimenkul", "nakit akis"} {
		if strings.Contains(slug, term) {
			priority += 10
		}
	}
	if strings.Contains(slug, "balance_sheet") || strings.Contains(slug, "income_statement") || strings.Contains(slug, "cash_flow") {
		priority += 4
	}
	return priority
}

func kapRawPeriod(period *string) string {
	if period == nil {
		return ""
	}
	return strings.TrimSpace(*period)
}

func kapRawOptionalPercent(value *float64) string {
	if value == nil {
		return "Yok"
	}
	return fmt.Sprintf("%.2f%%", *value)
}

func kapRawOptionalNumber(value *float64) string {
	if value == nil {
		return "Yok"
	}
	return formatReportNumber(*value)
}

func kapRawSourceText(path, snippet string) string {
	name := cleanDocumentName(path)
	snippet = shortReportText(snippet, 180)
	if name == "" {
		return emptyFallback(snippet, "Yok")
	}
	if snippet == "" {
		return name
	}
	return name + " | " + snippet
}

func shortReportText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}

func kapAssetNameCell(asset professional.KAPAssetInventoryItem) string {
	name := strings.TrimSpace(asset.AssetName)
	if name == "" {
		name = "İsimsiz varlık"
	}
	return name
}

func kapAssetPortfolioSummaryText(summary professional.KAPAssetPortfolioSummary) string {
	parts := []string{}
	if summary.TotalRealEstateValueExclVATTRY != nil {
		parts = append(parts, "KDV hariç "+formatMoney(*summary.TotalRealEstateValueExclVATTRY, "TRY"))
	}
	if summary.TotalRealEstateValueInclVATTRY != nil {
		parts = append(parts, "KDV dahil "+formatMoney(*summary.TotalRealEstateValueInclVATTRY, "TRY"))
	}
	if summary.TotalBookValueTRY != nil {
		parts = append(parts, "toplam portföy/defter "+formatMoney(*summary.TotalBookValueTRY, "TRY"))
	}
	if summary.HistoryCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tarihsel toplam satırı", summary.HistoryCount))
	}
	if len(parts) == 0 {
		return "Güvenilir portföy toplamı bulunamadı"
	}
	return strings.Join(parts, " | ")
}

func kapAssetValueIndexSummaryText(summary professional.KAPAssetValueIndexSummary) string {
	if !summary.Computed {
		if summary.DataQualityWarning != "" {
			return summary.DataQualityWarning
		}
		return "Yİ-ÜFE endeks verisi bağlı değil; tarihi değerler güncel TL'ye taşınmadı"
	}
	parts := []string{}
	if summary.SeriesLabel != "" {
		parts = append(parts, summary.SeriesLabel)
	}
	if summary.LatestPeriod != "" {
		parts = append(parts, "son endeks "+summary.LatestPeriod)
	}
	if summary.IndexableAssetCount > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d varlık endekslendi", summary.IndexedAssetCount, summary.IndexableAssetCount))
	}
	if summary.Source != "" {
		parts = append(parts, summary.Source)
	}
	if len(parts) == 0 {
		return "Yİ-ÜFE endeks verisi bağlı"
	}
	return strings.Join(parts, " | ")
}

func kapAssetLocationText(asset professional.KAPAssetInventoryItem) string {
	parts := []string{}
	if asset.Location != "" {
		parts = append(parts, asset.Location)
	}
	if asset.City != "" || asset.District != "" {
		city := strings.TrimSpace(strings.Join([]string{asset.District, asset.City}, " "))
		parts = append(parts, city)
	}
	if len(parts) == 0 {
		return "Yok"
	}
	return strings.Join(parts, " | ")
}

func kapAssetAreaText(asset professional.KAPAssetInventoryItem) string {
	if asset.AreaM2 == nil {
		return "Yok"
	}
	return fmt.Sprintf("%.0f m2", *asset.AreaM2)
}

func kapAssetValueText(asset professional.KAPAssetInventoryItem) string {
	if asset.LatestValueTRY == nil {
		return "Yok"
	}
	source := asset.ValueSource
	switch source {
	case "expertise_excl_vat_try":
		source = "ekspertiz KDV hariç"
	case "expertise_incl_vat_try":
		source = "ekspertiz KDV dahil"
	case "book_value_try":
		source = "defter değeri"
	default:
		source = "değer"
	}
	parts := []string{formatMoneyExact(*asset.LatestValueTRY, "TRY"), source}
	if kapAssetHasWarning(asset, "asset_value_scale_suspicious_low") {
		parts = append(parts, "birim/ölçek doğrulanmalı")
	}
	if kapAssetHasWarning(asset, "asset_valuation_report_code_requires_ownership_review") {
		parts = append(parts, "mülkiyet/rapor kodu doğrulanmalı")
	}
	return strings.Join(parts, " | ")
}

func kapAssetTypedValueText(value *float64) string {
	if value == nil {
		return "Yok"
	}
	return formatMoneyExact(*value, "TRY")
}

func kapAssetIndexedValueText(asset professional.KAPAssetInventoryItem) string {
	if asset.IndexedValueTRY == nil {
		if asset.LatestValueTRY == nil {
			return "Yok"
		}
		switch asset.IndexedValueWarning {
		case "asset_value_base_period_missing":
			return "Hesaplanamadı | değer tarihi yok"
		case "asset_value_inflation_index_missing_for_period":
			return "Hesaplanamadı | dönem endeksi yok"
		default:
			return "Hesaplanamadı | Yİ-ÜFE verisi yok"
		}
	}
	parts := []string{formatMoneyExact(*asset.IndexedValueTRY, "TRY")}
	if asset.IndexedValueAsOf != "" {
		parts = append(parts, asset.IndexedValueAsOf)
	}
	if asset.IndexedValueBasePeriod != "" && asset.IndexedValueFactor > 0 {
		parts = append(parts, fmt.Sprintf("%s=>x%.4f", asset.IndexedValueBasePeriod, asset.IndexedValueFactor))
	}
	if asset.IndexedValueSource != "" {
		parts = append(parts, asset.IndexedValueSource)
	}
	if kapAssetHasWarning(asset, "asset_value_scale_suspicious_low") {
		parts = append(parts, "birim/ölçek doğrulanmalı")
	}
	return strings.Join(parts, " | ")
}

func kapAssetHasWarning(asset professional.KAPAssetInventoryItem, warning string) bool {
	for _, item := range asset.Warnings {
		if item == warning {
			return true
		}
	}
	return false
}

func kapAssetRentText(asset professional.KAPAssetInventoryItem) string {
	parts := []string{}
	if asset.MonthlyRentTRY != nil {
		parts = append(parts, "aylık "+formatMoney(*asset.MonthlyRentTRY, "TRY"))
	}
	if asset.AnnualRentTRY != nil {
		parts = append(parts, "yıllık "+formatMoney(*asset.AnnualRentTRY, "TRY"))
	}
	if asset.AnnualRentUSD != nil {
		parts = append(parts, fmt.Sprintf("yıllık %.0f USD", *asset.AnnualRentUSD))
	}
	if len(parts) == 0 {
		return "Yok"
	}
	return strings.Join(parts, " | ")
}

func kapAssetSourceText(asset professional.KAPAssetInventoryItem) string {
	source := strings.TrimSpace(asset.SourceFile)
	if source == "" {
		return "Yok"
	}
	base := filepath.Base(source)
	if base == "." || base == string(filepath.Separator) {
		return source
	}
	return base
}

func kapAssetTypeLabel(assetType string) string {
	switch assetType {
	case "land":
		return "Arsa/arazi"
	case "hotel":
		return "Otel"
	case "office":
		return "İş merkezi/ofis"
	case "shop":
		return "Dükkan/mağaza"
	case "factory":
		return "Fabrika/tesis"
	case "project":
		return "Proje"
	case "usage_right":
		return "Kullanım/üst hakkı"
	case "subsidiary":
		return "İştirak"
	case "financial_asset":
		return "Finansal varlık"
	default:
		return emptyFallback(assetType, "Diğer")
	}
}

func kapPDFTypeCountsText(items []professional.KAPPDFTypeCount, limit int) string {
	parts := []string{}
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		parts = append(parts, fmt.Sprintf("%s: %d", item.Label, item.Count))
	}
	if len(parts) == 0 {
		return "Belge tipi sınıflandırılamadı"
	}
	return strings.Join(parts, ", ")
}

func kapPDFIngestStatusText(kap professional.KAPPDFIngestSummary) string {
	if !kap.Computed {
		return emptyFallback(kap.Summary, "KAP PDF veri içe aktarma çıktısı bulunamadı.")
	}
	if kap.SourcePDFCount > 0 {
		return fmt.Sprintf("%d kaynak PDF | %d benzersiz metin | %d analize uygun | %d analiz dışı (%d AI çözüm bekleyen, %d reddedilen) | %d tekrarlı dosya | %s | OCR %d | hata %d",
			kap.SourcePDFCount,
			kap.TotalDocuments,
			kap.AnalysisUsableCount,
			kap.ReviewRequiredCount,
			kapPDFReviewOnlyCount(kap),
			kap.RejectedCount,
			kap.DuplicatePDFCount,
			kapPDFTypeCountsText(kap.TypeCounts, 4),
			kap.OCRUsedCount,
			kap.ErrorCount,
		)
	}
	return fmt.Sprintf("%d PDF | %d analize uygun | %d analiz dışı (%d AI çözüm bekleyen, %d reddedilen) | %s | OCR %d | hata %d", kap.TotalDocuments, kap.AnalysisUsableCount, kap.ReviewRequiredCount, kapPDFReviewOnlyCount(kap), kap.RejectedCount, kapPDFTypeCountsText(kap.TypeCounts, 4), kap.OCRUsedCount, kap.ErrorCount)
}

func kapPDFReviewOnlyCount(kap professional.KAPPDFIngestSummary) int {
	reviewOnly := kap.ReviewRequiredCount - kap.RejectedCount
	if reviewOnly < 0 {
		return kap.ReviewRequiredCount
	}
	return reviewOnly
}

func kapPDFIngestLines(result analysis.SymbolAnalysis) []string {
	kap := result.Professional.KAPPDFIngest
	if !kap.Computed {
		return nil
	}
	lines := []string{
		fmt.Sprintf("KAP PDF veri içe aktarma: %d benzersiz PDF rapora dahil edildi; ortalama metin kalite skoru %.2f.", kap.TotalDocuments, kap.AverageQuality),
		fmt.Sprintf("Belge dağılımı: %s.", kapPDFTypeCountsText(kap.TypeCounts, 5)),
	}
	if kap.SourcePDFCount > 0 {
		lines = append(lines, fmt.Sprintf("Kaynak kapsamı: %d PDF dosyası, %d benzersiz dosya izi, %d tekrarlı dosya, %d eksik dosya izi.",
			kap.SourcePDFCount,
			kap.SourceUniqueHashes,
			kap.DuplicatePDFCount,
			kap.MissingUniqueHashes,
		))
	}
	if kap.ReviewRequiredCount > 0 || kap.RejectedCount > 0 || kap.ErrorCount > 0 {
		lines = append(lines, fmt.Sprintf("Kalite kapısı: analiz dışı %d kayıt var (%d AI çözüm bekleyen, %d reddedilen), veri içe aktarma hatası %d; bu kayıtlar AI vision/OCR ile yeniden çözümlenmelidir.", kap.ReviewRequiredCount, kapPDFReviewOnlyCount(kap), kap.RejectedCount, kap.ErrorCount))
	}
	return lines
}

func kapAssetInventoryLines(result analysis.SymbolAnalysis) []string {
	inventory := result.Professional.KAPAssetInventory
	if !inventory.Computed {
		return nil
	}
	lines := []string{
		fmt.Sprintf("KAP varlık envanteri: %d varlık olayı, %d ham envanter satırı, %d güvenli/tekilleştirilmiş HTML satırı.", inventory.EventCount, inventory.RawAssetCount, inventory.DisplayAssetCount),
		"Portföy toplamı: " + kapAssetPortfolioSummaryText(inventory.PortfolioSummary) + ".",
		"Endeksli değerleme: " + kapAssetValueIndexSummaryText(inventory.ValueIndex) + ".",
	}
	if len(inventory.Warnings) > 0 {
		lines = append(lines, "Uyarılar: "+strings.Join(inventory.Warnings, ", "))
	}
	return lines
}

func investmentResearchPDFLines(result analysis.SymbolAnalysis) []string {
	review := result.Professional.InvestmentResearch
	if !review.Computed {
		return nil
	}
	lines := []string{
		emptyFallback(review.Summary, "Yatırım kararı denetimi üretilemedi."),
		fmt.Sprintf("Kurumsal not: %s, skor %.0f/100, direkt alım uygun: %s, komiteye hazır: %s.",
			reportLabel(review.InstitutionalMemo.Recommendation),
			review.InstitutionalMemo.ReadinessScore,
			localize.Bool(review.InstitutionalMemo.DirectBuyEligible),
			localize.Bool(review.InstitutionalMemo.InvestmentCommitteeReady),
		),
		"Yatırım hikayesi: " + review.InvestmentStory.CoreThesis,
		"Değer kaynağı: " + review.InvestmentStory.ValueSource,
		"Piyasa sorusu: " + review.InvestmentStory.MispricingQuestion,
	}
	if len(review.InstitutionalMemo.BlockingIssues) > 0 {
		lines = append(lines, "Kurumsal engel: "+strings.Join(reportLabels(limitStrings(review.InstitutionalMemo.BlockingIssues, 5)), " | "))
	}
	if len(review.InstitutionalMemo.RequiredFixes) > 0 {
		lines = append(lines, "Komite öncesi zorunlu düzeltme: "+strings.Join(reportTexts(limitStrings(review.InstitutionalMemo.RequiredFixes, 4)), " | "))
	}
	if len(review.DecisionFramework.DecisionBasis) > 0 {
		lines = append(lines, "Karar gerekçesi: "+strings.Join(reportTexts(limitStrings(review.DecisionFramework.DecisionBasis, 3)), " "))
	}
	if len(review.OpenResearchQuestions) > 0 {
		lines = append(lines, "Eksik doğrulamalar: "+strings.Join(reportTexts(limitStrings(review.OpenResearchQuestions, 5)), " | "))
	}
	return lines
}

func documentPurposeText(purpose, date, title string) string {
	parts := []string{}
	if date != "" {
		parts = append(parts, date)
	}
	if title != "" {
		parts = append(parts, title)
	}
	if purpose != "" {
		parts = append(parts, purpose)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " | ")
}

func documentContentText(doc docintel.Document) string {
	parts := []string{}
	if doc.ContentSummary != "" {
		parts = append(parts, doc.ContentSummary)
	}
	if doc.LLMAnalysis != nil && doc.LLMAnalysis.Computed {
		if doc.LLMAnalysis.Summary != "" {
			parts = append(parts, "LLM özeti: "+doc.LLMAnalysis.Summary)
		}
		if len(doc.LLMAnalysis.RiskFlags) > 0 {
			parts = append(parts, "LLM riskleri: "+strings.Join(doc.LLMAnalysis.RiskFlags, ", "))
		}
	}
	if len(doc.Topics) > 0 {
		parts = append(parts, "Konular: "+strings.Join(doc.Topics, ", "))
	}
	source := doc.ExtractionSource
	if source == "pdf_text" {
		source = "PDF metni"
	} else if source == "kap_disclosure_body" {
		source = "KAP bildirim gövdesi"
	}
	if source != "" {
		parts = append(parts, "Kaynak: "+source)
	}
	if doc.ExtractionNote != "" {
		parts = append(parts, "Not: "+doc.ExtractionNote)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func documentLLMStatusText(llm docintel.LLMReport) string {
	switch llm.Status {
	case "complete":
		return fmt.Sprintf("LLM: gerçek çağrı ile %d belge analiz edildi.", llm.DocumentsAnalyzed)
	case "partial":
		return fmt.Sprintf("LLM: kısmi, %d/%d belge analiz edildi.", llm.DocumentsAnalyzed, llm.DocumentsRequested)
	case "missing_api_key":
		return "LLM: API anahtarı olmadığı için çalışmadı."
	case "missing_model":
		return "LLM: model adı olmadığı için çalışmadı."
	case "missing_endpoint":
		return "LLM: endpoint olmadığı için çalışmadı."
	case "failed":
		return "LLM: yapılandırıldı fakat geçerli analiz dönmedi."
	default:
		return "LLM: yapılandırılmadı; sahte LLM sonucu üretilmedi."
	}
}

func joinStringEvidence(items []string, limit int) string {
	cleaned := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		cleaned = append(cleaned, item)
		if limit > 0 && len(cleaned) == limit {
			break
		}
	}
	if len(cleaned) == 0 {
		return "-"
	}
	return strings.Join(cleaned, "; ")
}

func cleanDocumentName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".jsonl") {
		return "İç veri kaynağı"
	}
	if len([]rune(name)) <= 110 {
		return name
	}
	runes := []rune(name)
	return string(runes[:107]) + "..."
}

func valueCheckLabel(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "intrinsic_value":
		return "İçsel değer"
	case "fair_value_conclusion":
		return "Ucuz/pahalı cevabı"
	case "owner_earnings":
		return "Sahibine kalan nakit"
	case "normalized_fcf":
		return "Normalize serbest nakit akımı"
	case "capital_allocation":
		return "Sermaye tahsisi"
	case "moat":
		return "Rekabet gücü"
	case "kap_document_evidence":
		return "KAP PDF / belge kanıtı"
	case "tuik_gdp_macro":
		return "TÜİK GSYH makro etkisi"
	case "buffett_value_checklist":
		return "Buffett değer yatırımı filtresi"
	default:
		return reportLabel(name)
	}
}

func valueWarningTexts(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "paid_capital_dilution_above_10pct_5y":
			out = append(out, "son 5 yılda pay sulanması %10 üzerinde")
		case "paid_capital_growth_requires_bonus_split_rights_issue_classification":
			out = append(out, "ödenmiş sermaye artışı bedelli/bedelsiz/split/nominal düzeltme olarak sınıflandırılmalı")
		case "net_debt_to_equity_above_1":
			out = append(out, "net borç/özsermaye 1 üzerinde")
		case "owner_earnings_ttm_inputs_missing":
			out = append(out, "sahibine kalan nakit için son 12 ay girdileri eksik")
		case "free_cash_flow_ttm_inputs_missing":
			out = append(out, "serbest nakit akımı için son 12 ay girdileri eksik")
		case "owner_earnings_not_primary_for_sector_model":
			out = append(out, "bu sektör modelinde sahibine kalan nakit ana girdi değil")
		case "fcf_not_primary_for_sector_model":
			out = append(out, "bu sektör modelinde serbest nakit akımı ana girdi değil")
		case "financial_statements":
			out = append(out, "finansal tablolar")
		case "financial_periods":
			out = append(out, "finansal dönemler")
		case "strict_evidence_policy_suppressed":
			out = append(out, "sıkı kanıt politikası nedeniyle değerleme bastırıldı")
		case "bank_regulatory_metrics_missing":
			out = append(out, reportLabel("bank_regulatory_metrics_missing"))
		default:
			out = append(out, reportLabel(item))
		}
	}
	return out
}

func writeTechnicalAppendixHTML(b *strings.Builder, result analysis.SymbolAnalysis, keys []string) {
	for _, key := range keys {
		tf := result.Timeframes[key]
		title := "Teknik Ek - " + localize.Timeframe(key)
		b.WriteString("<section class=\"section\"><h2>")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</h2>")

		b.WriteString("<h3>Kapsam ve Skor Dağılımı</h3><table><tbody>")
		for _, row := range technicalScopeRows(result, tf) {
			writeHTMLRow(b, row[0], row[1])
		}
		for _, row := range technicalScoreRows(tf) {
			writeHTMLRow(b, row[0], row[1])
		}
		b.WriteString("</tbody></table>")

		b.WriteString("<h3>Çekirdek İndikatör Değerleri</h3><table><tbody>")
		for _, row := range coreIndicatorRows(tf) {
			writeHTMLRow(b, row[0], row[1])
		}
		b.WriteString("</tbody></table>")

		writeHTMLTable(b, "Hesaplanan İndikatör Sinyalleri", []string{"İndikatör", "Kategori", "Sinyal", "Değer", "Güven", "Kanıt"}, indicatorResultRows(tf, 0))
		writeHTMLTable(b, "Aktif Formasyon Sonuçları", []string{"Adı", "Yönü", "Güven Skoru", "Hacim Teyidi", "Teyit Eden İndikatörler", "Geçersiz Kılan Karşı Sinyaller", "İşlem Değeri"}, activePatternRows(tf.Patterns, 0))
		writeHTMLTable(b, "İşlem Sinyali Olmayan İzleme / Elenen Formasyonlar", []string{"Formasyon", "Kategori", "Yön", "Güven", "Neden", "Kanıt"}, candidatePatternRows(tf.PatternCandidates, 10))

		b.WriteString("</section>\n")
	}
}

func writeHTMLTable(b *strings.Builder, title string, headers []string, rows [][]string) {
	b.WriteString("<h3>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h3><table><thead><tr>")
	for _, header := range headers {
		b.WriteString("<th>")
		b.WriteString(html.EscapeString(header))
		b.WriteString("</th>")
	}
	b.WriteString("</tr></thead><tbody>")
	if len(rows) == 0 {
		b.WriteString("<tr><td colspan=\"")
		b.WriteString(fmt.Sprintf("%d", len(headers)))
		b.WriteString("\">Bu bölüm için rapora alınacak doğrulanmış sonuç yok.</td></tr>")
	}
	for _, row := range rows {
		b.WriteString("<tr>")
		for _, value := range row {
			b.WriteString("<td>")
			b.WriteString(html.EscapeString(reportText(value)))
			b.WriteString("</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
}

func executiveDecision(result analysis.SymbolAnalysis) string {
	if result.DecisionSupport != nil && result.DecisionSupport.Retail.Signal != "" {
		return reportLabel(result.DecisionSupport.Retail.Signal)
	}
	daily, ok := result.Timeframes["1D"]
	if ok && activeSummaryTradePlan(daily.TradePlan) && result.OverallScore >= 65 {
		return "ALIM ADAYI"
	}
	if result.OverallScore >= 55 {
		return "TAKİP / BEKLE"
	}
	return "BEKLE"
}

func executiveDecisionClass(result analysis.SymbolAnalysis) string {
	if result.DecisionSupport != nil {
		switch result.DecisionSupport.Retail.Signal {
		case "AL":
			return "good"
		case "SAT":
			return "bad"
		case "BEKLE":
			return "warn"
		}
	}
	if isEquityResult(result) && !directBuySignalAllowed(result) {
		if result.OverallScore >= 45 {
			return "warn"
		}
		return "bad"
	}
	return decisionClass(result.OverallScore)
}

func directBuySignalAllowed(result analysis.SymbolAnalysis) bool {
	if !reportDecisionGateKnown(result) {
		return true
	}
	if result.DecisionSupport != nil {
		if result.DecisionSupport.Retail.Signal != "" {
			return strings.EqualFold(result.DecisionSupport.Retail.Signal, "AL") && result.DecisionSupport.Retail.Actionable
		}
		for _, item := range result.DecisionSupport.UseCaseMatrix {
			useCase := strings.ToLower(strings.TrimSpace(item.UseCase))
			if strings.Contains(useCase, "al/sat") || (strings.Contains(useCase, "al") && strings.Contains(useCase, "sat")) {
				return item.Allowed && reportStatusPass(item.Status)
			}
		}
	}
	views := result.InvestorQA.InstitutionalViews
	if views.Computed {
		if !reportStatusPass(views.FinancialTransactionUse.Status) {
			return false
		}
		if views.TradingEdge.TransactionUseStatus != "" && !reportStatusPass(views.TradingEdge.TransactionUseStatus) {
			return false
		}
	}
	if result.InstitutionalValidation.Status != "" && !reportStatusPass(result.InstitutionalValidation.Status) {
		return false
	}
	if daily, ok := result.Timeframes["1D"]; ok {
		gate := daily.Professional.Technical.SignalGate
		if gate.Status != "" && (!reportStatusPass(gate.Status) || !gate.Actionable) {
			return false
		}
	}
	return true
}

func reportTradePlanBlocked(result analysis.SymbolAnalysis) bool {
	if isDecisionReadyStatus(result) {
		return false
	}
	return reportDecisionGateKnown(result) && !directBuySignalAllowed(result)
}

func reportDecisionGateKnown(result analysis.SymbolAnalysis) bool {
	return result.DecisionSupport != nil ||
		result.InvestorQA.Computed ||
		result.InstitutionalValidation.Status != ""
}

func isDecisionReadyStatus(result analysis.SymbolAnalysis) bool {
	// Primary: use the central classification (single source of truth).
	if result.DecisionClassification.SchemaVersion > 0 {
		return result.DecisionClassification.Classes.LargeInvestor.Qualified ||
			result.DecisionClassification.Classes.RetailDirect.Qualified
	}
	// Fallback for backwards compatibility when Classification is not yet set.
	if result.DecisionSupport == nil {
		return false
	}
	s := result.DecisionSupport.Status
	return s == "decision_ready" || s == "decision_ready_with_execution_limits" || s == "decision_issued_with_limitations"
}

func reportStatusPass(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass", "passed", "geçti", "gecti":
		return true
	default:
		return false
	}
}

func executiveDecisionText(result analysis.SymbolAnalysis) string {
	pro := result.Professional
	if isCryptoResult(result) {
		return fmt.Sprintf(
			"%s için karar %s. Entegre skor %.1f/100 ve bu skor fiyat, teknik yapı, destek/direnç, fiyat ivmesi, para akışı, oynaklık, formasyon ve geçmiş fiyat testi katmanlarından gelir. Blokzincir, türev piyasa, borsa giriş-çıkış ve kripto haber/duygu tonu kaynakları bağlanmadan bu rapor kurumsal ana işlem kararı olamaz.",
			result.Symbol,
			executiveDecision(result),
			result.OverallScore,
		)
	}
	if isCommodityResult(result) {
		return fmt.Sprintf(
			"%s için karar %s. Entegre skor %.1f/100 ve bu skor TradingView fiyat grafiği, destek/direnç, fiyat ivmesi, para akışı, oynaklık, formasyon ve geçmiş fiyat testi katmanlarından gelir. DXY/reel faiz, vadeli pozisyon, ETF/fiziki akış ve haber kaynakları bağlanmadan bu rapor ana yatırım kararı değil, teknik karar desteğidir.",
			result.Symbol,
			executiveDecision(result),
			result.OverallScore,
		)
	}
	if result.Professional.ValueInvesting.Computed {
		v := result.Professional.ValueInvesting
		decisionQualifier := ""
		if isEquityResult(result) && !directBuySignalAllowed(result) && !isDecisionReadyStatus(result) {
			decisionQualifier = " Bu karar doğrudan AL sinyali değildir; AL/SAT kullanım kapısı geçmeden yeni alım önerisi sayılmaz."
		}
		return fmt.Sprintf(
			"%s için karar %s. Güncel fiyat %.2f %s, baz içsel değer %.2f %s ve güvenlik marjı %.1f%%. Değer yatırım kararı: %s. Şirket kalite skoru %.0f/100, değerleme güveni %.0f/100. Teknik analiz giriş zamanlaması için ek katmandır; ana değer yatırım cevabı içsel değer ve güvenlik marjıdır.%s%s",
			result.Symbol,
			executiveDecision(result),
			v.CurrentPrice,
			displayCurrency(result.Currency),
			v.IntrinsicValue.Base,
			displayCurrency(result.Currency),
			v.MarginOfSafety.BasePct,
			v.DecisionLabel,
			v.QualityScore,
			v.Confidence,
			macroDecisionClause(result),
			decisionQualifier,
		)
	}
	decisionQualifier := ""
	if isEquityResult(result) && !directBuySignalAllowed(result) && !isDecisionReadyStatus(result) {
		decisionQualifier = " AL/SAT kullanım kapısı geçmediği için bu çıktı takip/radar niteliğindedir."
	}
	return fmt.Sprintf(
		"%s için karar %s. Entegre skor %.1f/100. Bu skor mum yapısı, destek/direnç, fiyat ivmesi, para akışı, bilanço kalitesi, borçluluk, kârlılık, makro fiyat etkisi ve benzer şirket değerleme sinyalini birlikte tartar. Finansal tarafta veri kalitesi %.0f/100, sektör %s, benzer şirket değerleme sinyali %s. Bu sonuç kesin getiri vaadi değildir; alım ancak fiyat teyidi, hacim ve zarar kes disiplini ile anlamlıdır.%s%s",
		result.Symbol,
		executiveDecision(result),
		result.OverallScore,
		pro.DataQuality,
		emptyFallback(pro.Company.Sector, "belirsiz"),
		valuationSignalTR(pro.Peers.ValuationSignal),
		macroDecisionClause(result),
		decisionQualifier,
	)
}

func macroDecisionClause(result analysis.SymbolAnalysis) string {
	impact := result.Professional.TCMBEVDSContext.ForecastImpact
	if !impact.Computed {
		return ""
	}
	return fmt.Sprintf(
		" Makro fiyat etkisi: %s, şiddet %s, güven %.0f/100, karar kullanımı %s.",
		reportText(impact.Label),
		reportText(impact.Severity),
		impact.Confidence,
		reportText(impact.DecisionUse),
	)
}

func reportWaitReasons(result analysis.SymbolAnalysis) []string {
	daily, ok := result.Timeframes["1D"]
	if !ok {
		return []string{"Günlük zaman dilimi yok; karar için ana fiyat teyidi eksik."}
	}
	reasons := []string{}
	tradePlanBlocked := reportTradePlanBlocked(result)
	if tradePlanBlocked {
		reasons = append(reasons, "AL/SAT sinyali kullanım kapısı geçmedi; üst karar takip/radar seviyesinde kalmalı.")
	}
	if tradePlanBlocked && activeSummaryTradePlan(daily.TradePlan) {
		reasons = append(reasons, "Teknik giriş bandı emir planı olarak kullanılmaz; önce kurumsal kanıt, finansal veri ve teknik sinyal kapıları geçmeli.")
	} else if activeSummaryTradePlan(daily.TradePlan) {
		reasons = append(reasons, fmt.Sprintf("%s üstü kapanış ve güçlü hacim görülmeli.", retailPrice(daily.TradePlan.EntryMax, result.Currency)))
	} else if daily.NearestResistance != nil {
		reasons = append(reasons, fmt.Sprintf("%s direnci üstünde kapanış görülmeli.", retailPrice(daily.NearestResistance.Price, result.Currency)))
	}
	if daily.Indicators.MACDHistogram < 0 {
		reasons = append(reasons, fmt.Sprintf("MACD histogramı %.2f; kısa vadeli ivme zayıf, alım için hızlanma teyidi eksik.", daily.Indicators.MACDHistogram))
	}
	if daily.Indicators.ChaikinMoneyFlow20 < 0 {
		reasons = append(reasons, "Para akışı zayıf; yükseliş yeterli alıcı girişiyle desteklenmeli.")
	}
	if baseReturn, ok := reportScenarioReturn(result, "base"); ok && baseReturn < 0 {
		reasons = append(reasons, fmt.Sprintf("Temel senaryo %.1f%% aşağı alan gösteriyor; değerleme güvenli marj vermiyor.", math.Abs(baseReturn)))
	}
	if len(reasons) == 0 {
		if isCryptoResult(result) {
			reasons = append(reasons, "Fiyat, hacim, momentum ve kripto veri teyitleri aynı anda güçlenmeli.")
		} else if isCommodityResult(result) {
			reasons = append(reasons, "Fiyat, momentum, para akışı ve altın makro teyitleri aynı anda güçlenmeli.")
		} else {
			reasons = append(reasons, "Fiyat, hacim, momentum ve finansal kalite aynı anda teyit üretmeli.")
		}
	}
	return reasons
}

func reportRiskReasons(result analysis.SymbolAnalysis) []string {
	pro := result.Professional
	risks := []string{}
	if isCryptoResult(result) {
		if len(pro.Coverage.Missing) > 0 {
			risks = append(risks, dataImprovementLine(result))
		}
		if daily, ok := result.Timeframes["1D"]; ok {
			if daily.TrendBias == "bearish" {
				risks = append(risks, "Günlük trend düşüşte.")
			}
			if daily.Indicators.MACDHistogram < 0 {
				risks = append(risks, "Kısa vadeli ivme zayıf; MACD alımı doğrulamıyor.")
			}
			if daily.Indicators.ATR14 > 0 && daily.LastClose > 0 && daily.Indicators.ATR14/daily.LastClose > 0.06 {
				risks = append(risks, cryptoEntityLabel(result)+" oynaklığı yüksek; stop mesafesi ve pozisyon boyutu bu oynaklığa göre ayarlanmalı.")
			}
		}
		if len(risks) == 0 {
			risks = append(risks, "Belirgin ana teknik risk düşük; yine de kripto piyasası 24/7 olduğu için stop ve teyit şartı korunmalı.")
		}
		return risks
	}
	if isCommodityResult(result) {
		if len(pro.Coverage.Missing) > 0 {
			risks = append(risks, dataImprovementLine(result))
		}
		if daily, ok := result.Timeframes["1D"]; ok {
			if daily.TrendBias == "bearish" {
				risks = append(risks, "Günlük fiyat yönü zayıf.")
			}
			if daily.Indicators.MACDHistogram < 0 {
				risks = append(risks, "Kısa vadeli ivme zayıf; alım teyidi yok.")
			}
			if daily.Indicators.ATR14 > 0 && daily.LastClose > 0 && daily.Indicators.ATR14/daily.LastClose > 0.035 {
				risks = append(risks, entityDisplayLabel(result)+" oynaklığı yüksek; stop mesafesi ve pozisyon boyutu buna göre ayarlanmalı.")
			}
		}
		if len(risks) == 0 {
			risks = append(risks, "Belirgin ana teknik risk düşük; yine de stop ve teyit şartı korunmalı.")
		}
		return risks
	}
	if strings.EqualFold(pro.Peers.ValuationSignal, "premium") {
		risks = append(risks, "Benzer şirket çarpanlarına göre primli/pahalı bölge.")
	}
	if pro.Peers.PeerCount == 0 || pro.Company.Sector == "BIST Genel" {
		risks = append(risks, "Sektör/benzer şirket karşılaştırması sınırlı; değerleme yorumu temkinli okunmalı.")
	}
	if !isBankReport(result) && !valuationRatioSuppressedForReport(pro.Valuation, "NetDebt_Eq") && pro.Valuation.Ratios["NetDebt_Eq"] > 1 {
		risks = append(risks, fmt.Sprintf("Net borç/özsermaye %.2f; bilanço riski yüksek.", pro.Valuation.Ratios["NetDebt_Eq"]))
	}
	if daily, ok := result.Timeframes["1D"]; ok {
		if daily.TrendBias == "bearish" {
			risks = append(risks, "Günlük trend düşüşte.")
		}
		if daily.Indicators.MACDHistogram < 0 {
			risks = append(risks, "Kısa vadeli ivme zayıf; MACD alımı doğrulamıyor.")
		}
	}
	if len(risks) == 0 {
		risks = append(risks, "Belirgin ana risk düşük; yine de stop ve teyit şartı korunmalı.")
	}
	return risks
}

func behavioralPlainText(result analysis.SymbolAnalysis) string {
	behavioral := result.Behavioral
	if behavioral.Contrarian.Score <= 0 && behavioral.Capitulation.Score <= 0 {
		return "Haber/yorum ve dip sinyali için yeterli veri yok; karar üzerinde etkisi yok."
	}
	return fmt.Sprintf("%s %s", behavioral.Contrarian.PlainLanguage, behavioral.Contrarian.QualityGate.Reason)
}

func behavioralSentimentText(result analysis.SymbolAnalysis) string {
	sentiment := result.Behavioral.Sentiment
	label := strings.TrimSpace(sentiment.Label)
	if label == "" {
		label = "veri sınırlı"
	}
	switch {
	case sentiment.Negativity >= 80:
		label = "negatif risk yüksek"
	case sentiment.Negativity >= 60 && strings.EqualFold(label, "nötr"):
		label = "nötr görünüyor; negatif risk yüksek"
	}
	return fmt.Sprintf("%s | negatif risk %.0f/100", label, sentiment.Negativity)
}

func fundamentalSectionTitle(result analysis.SymbolAnalysis) string {
	if isCryptoResult(result) {
		return "Kripto Veri Kapsamı"
	}
	if isCommodityResult(result) {
		return "Altın/Emtia Veri Kapsamı"
	}
	return "Bilanço ve Değerleme Özeti"
}

func fundamentalRows(result analysis.SymbolAnalysis) [][]string {
	pro := result.Professional
	if isCryptoResult(result) {
		return [][]string{
			{"Varlık sınıfı", emptyFallback(pro.Company.Sector, "Kripto")},
			{"Endüstri", emptyFallback(pro.Company.Industry, "Dijital varlık")},
			{"Veri kapsamı", fmt.Sprintf("%.0f/100", pro.DataQuality)},
			{"Teknik veri", joinReportLabels(pro.Coverage.Available)},
			{"Eksik kripto kaynakları", joinReportLabels(pro.Coverage.Missing)},
			{"Kapsam dışı geleneksel metrikler", "Geleneksel finansal tablo ve çarpan değerlemesi bu kripto raporuna dahil edilmedi."},
			{"Analiz yaklaşımı", "Bu raporda " + cryptoEntityLabel(result) + " için fiyat, teknik yapı, volatilite, likidite ve veri kapsamı okunur."},
			{"Para birimi", displayCurrency(result.Currency)},
		}
	}
	if isCommodityResult(result) {
		return [][]string{
			{"Varlık sınıfı", emptyFallback(pro.Company.Sector, "Altın/emtia")},
			{"Endüstri", emptyFallback(pro.Company.Industry, "Spot altın")},
			{"Veri kalitesi", fmt.Sprintf("%.0f/100", pro.DataQuality)},
			{"Teknik veri", joinReportLabels(pro.Coverage.Available)},
			{"Geliştirilecek kaynaklar", emptyFallback(strings.Join(missingDataLabels(result), ", "), "Yok")},
			{"Kapsam dışı geleneksel metrikler", "Şirket bilançosu, KAP ve hisse çarpan değerlemesi altın/emtia raporuna dahil edilmedi."},
			{"Analiz yaklaşımı", "Bu raporda " + entityDisplayLabel(result) + " için TradingView fiyat grafiği, teknik yapı, volatilite, likidite ve veri kapsamı okunur."},
			{"Para birimi", displayCurrency(result.Currency)},
		}
	}
	ratioLabel := "F/K | PD/DD | FD/Satış"
	ratioText := fmt.Sprintf("%s | %s | %s", valuationRatioText(pro.Valuation, "PE", "x"), valuationRatioText(pro.Valuation, "PB", "x"), valuationRatioText(pro.Valuation, "EV_Sales", "x"))
	if isBankReport(result) {
		ratioLabel = "Banka çarpanları"
		ratioText = fmt.Sprintf("F/K %s | PD/DD %s | FD/Satış A.D.", valuationRatioText(pro.Valuation, "PE", "x"), valuationRatioText(pro.Valuation, "PB", "x"))
	}
	rows := [][]string{
		{"Sektör / benzer şirket grubu", pro.Company.Sector},
		{"Veri kalitesi", fmt.Sprintf("%.0f/100", pro.DataQuality)},
		{"KAP PDF rapor kanıtı", kapPDFIngestStatusText(pro.KAPPDFIngest)},
		{"Sektör bilanço profili", sectorFinancialProfileText(pro)},
		{"Sektör bilanço yorumu", emptyFallback(pro.SectorFinancials.Summary, "Sektör bazlı bilanço yorumu üretilemedi.")},
		{"Finansal dönem", valuationPeriodText(pro.Valuation)},
		{"Piyasa değeri", formatMoney(pro.Valuation.MarketCap, result.Currency)},
		{"Net borç", valuationNetDebtText(pro.Valuation, result.Currency)},
		{"Özsermaye", formatMoney(pro.Valuation.Equity, result.Currency)},
		{ratioLabel, ratioText},
		{valuationReturnRowLabel(pro.Valuation), valuationReturnRowText(pro.Valuation)},
		{"Benzer şirket değerleme sinyali", valuationSignalTR(pro.Peers.ValuationSignal)},
	}
	if len(pro.Valuation.SuppressedRatios) > 0 {
		rows = append(rows, []string{"Kapsam dışı değerleme metrikleri", valuationSuppressedRatioText(pro.Valuation)})
	}
	for _, metric := range pro.SectorFinancials.Metrics {
		rows = append(rows, []string{metric.Label, sectorFinancialMetricText(metric)})
		if len(rows) >= 18 {
			break
		}
	}
	if len(pro.SectorFinancials.SuppressedMetric) > 0 {
		rows = append(rows, []string{"Sektör dışı bırakılan metrikler", strings.Join(pro.SectorFinancials.SuppressedMetric, ", ")})
	}
	gdp := pro.Market.GDP
	if gdp.Computed {
		rows = append(rows,
			[]string{"GSYH makro etkisi", gdpImpactSummary(result)},
			[]string{"GSYH yorumu", gdp.EquityImpact},
			[]string{"GSYH veri kaynağı", gdp.Source + " | " + gdp.Methodology},
		)
	} else if gdp.DataQualityWarning != "" {
		rows = append(rows, []string{"GSYH makro etkisi", gdp.DataQualityWarning})
	}
	return rows
}

func valuationPeriodText(valuation professional.ValuationAnalysis) string {
	if valuation.LatestYear == 0 && valuation.LatestQuarter == "" {
		return "Yok"
	}
	return strings.TrimSpace(fmt.Sprintf("%d %s", valuation.LatestYear, valuation.LatestQuarter))
}

func valuationNetDebtText(valuation professional.ValuationAnalysis, currency string) string {
	if valuation.SectorModel == "bank_equity_model" || valuationRatioSuppressedForReport(valuation, "NetDebt_Eq") {
		return "Uygulanmaz (banka/finansal kuruluş bilançosunda sanayi tipi net borç metriği kullanılmaz)"
	}
	return formatMoney(valuation.NetDebt, currency)
}

func valuationReturnRowLabel(valuation professional.ValuationAnalysis) string {
	if valuation.SectorModel == "bank_equity_model" {
		return "ROE | ROA"
	}
	return "ROE | Net borç/özsermaye"
}

func valuationReturnRowText(valuation professional.ValuationAnalysis) string {
	roe := valuationRatioText(valuation, "ROE", "%")
	if valuation.SectorModel == "bank_equity_model" {
		return fmt.Sprintf("%s | %s", roe, valuationRatioText(valuation, "ROA", "%"))
	}
	return fmt.Sprintf("%s | %s", roe, valuationRatioText(valuation, "NetDebt_Eq", "x"))
}

func valuationRatioText(valuation professional.ValuationAnalysis, key, unit string) string {
	if valuationRatioSuppressedForReport(valuation, key) {
		return "A.D."
	}
	value, ok := valuation.Ratios[key]
	if !ok || value == 0 {
		return "A.D."
	}
	switch unit {
	case "%":
		return formatPct(value * 100)
	case "x":
		return fmt.Sprintf("%.2f", value)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}

func valuationSuppressedRatioText(valuation professional.ValuationAnalysis) string {
	labels := make([]string, 0, len(valuation.SuppressedRatios))
	for _, key := range valuation.SuppressedRatios {
		labels = append(labels, valuationRatioLabel(key))
	}
	return strings.Join(labels, ", ")
}

func valuationRatioLabel(key string) string {
	switch key {
	case "PS":
		return "Fiyat/Satış"
	case "EV_Sales":
		return "FD/Satış"
	case "EV_EBIT":
		return "FD/Faaliyet kârı"
	case "EV_EBITDA":
		return "FD/FAVÖK"
	case "FCF_Yield":
		return "Serbest nakit akımı verimi"
	case "NetDebt_Eq":
		return "Net borç/özsermaye"
	default:
		return reportLabel(key)
	}
}

func valuationRatioSuppressedForReport(valuation professional.ValuationAnalysis, key string) bool {
	for _, suppressed := range valuation.SuppressedRatios {
		if suppressed == key {
			return true
		}
	}
	return false
}

func isBankReport(result analysis.SymbolAnalysis) bool {
	text := strings.ToLower(strings.Join([]string{
		result.Professional.Valuation.SectorModel,
		result.Professional.SectorFinancials.Profile,
		result.Professional.SectorFinancials.ProfileLabel,
		result.Professional.Company.Sector,
		result.Professional.Company.Industry,
	}, " "))
	return strings.Contains(text, "bank") || strings.Contains(text, "banka")
}

func bankCoreMetricsMissingForReport(result analysis.SymbolAnalysis) bool {
	for _, flag := range result.Professional.Valuation.Flags {
		if flag == "bank_sector_requires_regulatory_capital_and_asset_quality_model" ||
			flag == "bank_book_value_reconciliation_missing" ||
			flag == "bank_book_value_reconciliation_failed" {
			return true
		}
	}
	if !isBankReport(result) {
		return false
	}
	return !professional.BankRegulatoryMetricsComplete(result.Professional.SectorFinancials)
}

func sectorFinancialProfileText(pro professional.Report) string {
	sf := pro.SectorFinancials
	if !sf.Applicable {
		return emptyFallback(sf.Summary, "Uygulanamaz")
	}
	parts := []string{emptyFallback(sf.ProfileLabel, sf.Profile)}
	if sf.Score > 0 {
		parts = append(parts, fmt.Sprintf("%.0f/100", sf.Score))
	}
	if sf.Sector != "" {
		parts = append(parts, sf.Sector)
	}
	return strings.Join(parts, " | ")
}

func sectorFinancialMetricText(metric professional.SectorFinancialMetric) string {
	value := fmt.Sprintf("%.2f", metric.Value)
	if metric.Unit == "%" {
		value = fmt.Sprintf("%.1f%%", metric.Value*100)
	} else if metric.Unit != "" {
		value += metric.Unit
	}
	parts := []string{value, metric.Status}
	if metric.Interpretation != "" {
		parts = append(parts, metric.Interpretation)
	}
	return strings.Join(parts, " | ")
}

func technicalScopeRows(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis) [][]string {
	technical := tf.Professional.Technical
	signalCounts := technical.SignalCounts
	patternCounts := technical.PatternCounts
	computed := intCount(signalCounts, "computed", countComputedIndicators(tf.IndicatorSignals))
	totalSignals := intCount(signalCounts, "total", len(tf.IndicatorSignals))
	notReady := intCount(signalCounts, "not_ready", maxReportInt(0, totalSignals-computed))
	activePatterns := intCount(patternCounts, "confirmed", len(tf.Patterns))
	catalogPatterns := intCount(patternCounts, "catalog", len(tf.PatternScans))
	rawCandidates := len(tf.PatternCandidates)
	if rawCandidates == 0 {
		rawCandidates = countMatchedPatternScans(tf.PatternScans)
	}
	currentCandidates := countCurrentPatternCandidates(tf.PatternCandidates)
	return [][]string{
		{"Zaman dilimi", fmt.Sprintf("%s | %d mum | son kapanış %s | son hacim %s", localize.Timeframe(tf.Timeframe), tf.CandleCount, reportPriceValue(tf.LastClose), formatReportNumber(tf.LastVolume))},
		{"İndikatör kapsamı", fmt.Sprintf("%d tarandı | %d hesaplandı | %d sinyal dışı/veri eksik", totalSignals, computed, notReady)},
		{"Formasyon kapsamı", fmt.Sprintf("%d katalog taraması | %d ham aday | %d güncel aday | %d aktif/işlenebilir sonuç", catalogPatterns, rawCandidates, currentCandidates, activePatterns)},
		{"Doğrulama kapsamı", technicalValidationScopeText(technical.Validation)},
		{"Destek / direnç", supportResistanceSummary(result, tf)},
		{"İşlem planı", reportPlanTextForReport(result, tf)},
	}
}

func technicalScoreRows(tf analysis.TimeframeAnalysis) [][]string {
	score := tf.Professional.Technical.Score
	rows := [][]string{
		{"Teknik kanıt skoru", fmt.Sprintf("Toplam %.1f/100 | trend %.1f | momentum %.1f | hacim %.1f | oynaklık riski %.1f | formasyon %.1f", score.Total, score.Trend, score.Momentum, score.Volume, score.VolatilityRisk, score.Pattern)},
		{"Teknik özet", technicalEvidenceSummaryTR(tf)},
	}
	if tf.Professional.Technical.SignalGate.Status != "" {
		rows = append(rows, []string{"Aktif işlem sinyali kapısı", technicalSignalGateSummary(tf)})
	}
	if tf.Professional.Technical.Validation.Status != "" {
		rows = append(rows, []string{"İndikatör/formasyon doğrulama kapısı", technicalValidationSummary(tf.Professional.Technical.Validation)})
	}
	return rows
}

func technicalValidationScopeText(validation professional.TechnicalValidationReport) string {
	if validation.Status == "" {
		return "Teknik doğrulama raporu üretilmedi."
	}
	return fmt.Sprintf("indikatör %s: %d kontrol, %d hesaplanan, %d proxy/dış veri | formasyon %s: %d aktif, %d çizildi, %d çizilemedi | grafik %s",
		institutionalStatusTR(validation.IndicatorFormulaStatus),
		validation.IndicatorChecked,
		validation.IndicatorComputed,
		validation.IndicatorProxyOnly+validation.IndicatorExternalRequired,
		institutionalStatusTR(validation.PatternStatus),
		validation.PatternConfirmed,
		validation.PatternDrawn,
		validation.PatternNotDrawn,
		reportLabel(validation.ChartOverlayStatus),
	)
}

func technicalValidationSummary(validation professional.TechnicalValidationReport) string {
	if validation.Status == "" {
		return "Teknik doğrulama raporu üretilmedi."
	}
	parts := []string{
		institutionalStatusTR(validation.Status),
		fmt.Sprintf("%.0f/100", validation.Score),
	}
	if validation.Summary != "" {
		parts = append(parts, reportText(validation.Summary))
	} else {
		parts = append(parts, technicalValidationScopeText(validation))
	}
	if len(validation.Blockers) > 0 {
		parts = append(parts, "engeller: "+strings.Join(reportLabels(limitStrings(validation.Blockers, 3)), "; "))
	}
	return strings.Join(parts, " | ")
}

func technicalEvidenceSummaryTR(tf analysis.TimeframeAnalysis) string {
	technical := tf.Professional.Technical
	score := technical.Score
	if technical.SignalGate.Status != "" {
		return fmt.Sprintf("%.1f/100 teknik kanıt skoru; aktif sinyal kapısı %s, %.0f/100. %s", score.Total, institutionalStatusTR(technical.SignalGate.Status), technical.SignalGate.Score, technical.SignalGate.Label)
	}
	selectedIndicators := len(technical.SelectedIndicators)
	if selectedIndicators == 0 {
		selectedIndicators = countComputedIndicators(tf.IndicatorSignals)
	}
	selectedPatterns := len(technical.SelectedPatterns)
	if selectedPatterns == 0 {
		selectedPatterns = len(tf.Patterns)
	}
	if score.Total > 0 || selectedIndicators > 0 || selectedPatterns > 0 {
		return fmt.Sprintf("%.1f/100 teknik kanıt skoru; %d hesaplanan/seçili indikatör ve %d aktif formasyon değerlendirildi.", score.Total, selectedIndicators, selectedPatterns)
	}
	return timeframePlainComment(tf)
}

func technicalSignalGateSummary(tf analysis.TimeframeAnalysis) string {
	gate := tf.Professional.Technical.SignalGate
	parts := []string{
		institutionalStatusTR(gate.Status),
		fmt.Sprintf("%.0f/100", gate.Score),
		gate.Label,
	}
	if gate.Direction != "" {
		parts = append(parts, "yön "+reportLabel(gate.Direction))
	}
	if gate.RiskRewardRatio > 0 {
		parts = append(parts, fmt.Sprintf("R/R %.2f", gate.RiskRewardRatio))
	}
	if gate.VolumeConfirmation != "" {
		parts = append(parts, "hacim: "+gate.VolumeConfirmation)
	}
	if len(gate.Blockers) > 0 {
		parts = append(parts, "engeller: "+retailSignalGateBlockers(tf, 4))
	}
	return strings.Join(parts, " | ")
}

func coreIndicatorRows(tf analysis.TimeframeAnalysis) [][]string {
	ind := tf.Indicators
	rows := [][]string{
		{"Trend ortalamaları", fmt.Sprintf("SMA20 %.2f | SMA50 %.2f | SMA100 %.2f | SMA200 %.2f | EMA20 %.2f | EMA50 %.2f", ind.SMA20, ind.SMA50, ind.SMA100, ind.SMA200, ind.EMA20, ind.EMA50)},
		{"Momentum", fmt.Sprintf("RSI14 %.2f | MACD %.4f | MACD sinyal %.4f | MACD histogram %.4f | ROC12 %.2f", ind.RSI14, ind.MACD, ind.MACDSignal, ind.MACDHistogram, ind.ROC12)},
		{"Oynaklık", fmt.Sprintf("ATR14 %.2f | Bollinger alt/orta/üst %.2f / %.2f / %.2f | ADX14 %.2f", ind.ATR14, ind.BollingerLower, ind.BollingerMiddle, ind.BollingerUpper, ind.ADX14)},
		{"Hacim / para akışı", fmt.Sprintf("Hacim SMA20 %s | OBV %s | MFI14 %.2f | Chaikin Money Flow %.4f | VWAP %.2f", formatReportNumber(ind.VolumeSMA20), formatReportNumber(ind.OBV), ind.MFI14, ind.ChaikinMoneyFlow20, ind.VWAP)},
		{"Stokastik / CCI / Williams", fmt.Sprintf("Stoch RSI K/D %.2f / %.2f | Stochastic K/D %.2f / %.2f | CCI20 %.2f | WilliamsR14 %.2f", ind.StochRSIK, ind.StochRSID, ind.StochasticK, ind.StochasticD, ind.CCI20, ind.WilliamsR14)},
		{"Ichimoku", fmt.Sprintf("Tenkan %.2f | Kijun %.2f | Senkou A/B %.2f / %.2f | Chikou %.2f | bulut trend %.0f | TK cross %.0f", ind.IchimokuTenkan, ind.IchimokuKijun, ind.IchimokuSenkouA, ind.IchimokuSenkouB, ind.IchimokuChikou, ind.IchimokuCloudTrend, ind.IchimokuTKCross)},
		{"Pivot / kanallar", fmt.Sprintf("Pivot %.2f | R1 %.2f | R2 %.2f | S1 %.2f | S2 %.2f | Donchian alt/üst %.2f / %.2f | Keltner alt/orta/üst %.2f / %.2f / %.2f", ind.PivotPoint, ind.PivotR1, ind.PivotR2, ind.PivotS1, ind.PivotS2, ind.DonchianLower, ind.DonchianUpper, ind.KeltnerLower, ind.KeltnerMiddle, ind.KeltnerUpper)},
	}
	return rows
}

func indicatorResultRows(tf analysis.TimeframeAnalysis, limit int) [][]string {
	rows := [][]string{}
	for _, item := range tf.IndicatorSignals {
		if !item.Computed {
			continue
		}
		rows = append(rows, []string{
			item.Name,
			reportLabel(item.Category),
			localize.Signal(item.Signal),
			formatReportNumber(item.Value),
			formatReportPercent(item.Confidence * 100),
			reportEvidenceText(item.Evidence, item.Source),
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func activePatternRows(patterns []ohlcv.PatternResult, limit int) [][]string {
	rows := [][]string{}
	for _, pattern := range patterns {
		rows = append(rows, []string{
			localize.PatternName(pattern.Name),
			localize.Direction(pattern.Direction),
			formatReportPercent(pattern.Confidence * 100),
			patternVolumeConfirmationText(pattern),
			patternContextText(pattern.ConfirmingIndicators),
			patternContextText(pattern.InvalidatingSignals),
			patternTradeValueText(pattern),
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func candidatePatternRows(patterns []ohlcv.PatternResult, limit int) [][]string {
	rows := [][]string{}
	for _, pattern := range patterns {
		if pattern.Actionable || hasReportPatternRejection(pattern, "not_current_completed_pattern") {
			continue
		}
		rows = append(rows, []string{
			localize.PatternName(pattern.Name),
			reportLabel(pattern.Category),
			localize.Direction(pattern.Direction),
			formatReportPercent(pattern.Confidence * 100),
			patternRejectionText(pattern.RejectionReasons),
			reportEvidenceText(pattern.Evidence, pattern.Resolution),
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func mapMetricRows(prefix string, values map[string]float64, limit int) [][]string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := [][]string{}
	for _, key := range keys {
		rows = append(rows, []string{prefix + " - " + reportLabel(key), formatReportNumber(values[key])})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows
}

func supportResistanceSummary(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis) string {
	support := "yok"
	if tf.NearestSupport != nil {
		support = fmt.Sprintf("%s | güç %.2f | temas %d", retailPrice(tf.NearestSupport.Price, result.Currency), tf.NearestSupport.Strength, tf.NearestSupport.TouchCount)
	}
	resistance := "yok"
	if tf.NearestResistance != nil {
		resistance = fmt.Sprintf("%s | güç %.2f | temas %d", retailPrice(tf.NearestResistance.Price, result.Currency), tf.NearestResistance.Strength, tf.NearestResistance.TouchCount)
	}
	return "En yakın destek: " + support + " | en yakın direnç: " + resistance
}

func patternVolumeConfirmationText(pattern ohlcv.PatternResult) string {
	if pattern.VolumeConfirmed || pattern.VolumeConfirmation == "confirmed" {
		return "Var"
	}
	return "Yok - sinyal zayıf sayılır"
}

func patternContextText(values []string) string {
	if len(values) == 0 {
		return "Yok"
	}
	parts := make([]string, 0, 3)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, value)
		if len(parts) >= 3 {
			break
		}
	}
	if len(parts) == 0 {
		return "Yok"
	}
	if len(values) > len(parts) {
		parts = append(parts, fmt.Sprintf("+%d", len(values)-len(parts)))
	}
	return strings.Join(parts, " | ")
}

func patternTradeValueText(pattern ohlcv.PatternResult) string {
	validation := patternValidationSummary(pattern)
	withValidation := func(value string) string {
		if validation == "" {
			return value
		}
		return value + " | " + validation
	}
	switch pattern.TradeValue {
	case "strong":
		return withValidation("Güçlü doğrulanmış teknik sinyal")
	case "medium":
		return withValidation("Orta güçte doğrulanmış teknik sinyal")
	case "weak_no_volume_confirmation":
		return withValidation("Zayıf - hacim teyidi yok")
	case "weak_conflicted":
		return withValidation("Zayıf - karşı sinyaller baskın")
	case "rejected_candidate":
		return withValidation("İşlem sinyali değil - aday elendi")
	case "context_only":
		return withValidation("Yönsüz bağlam - işlem sinyali değil")
	case "weak":
		return withValidation("Zayıf teknik sinyal")
	default:
		if !pattern.VolumeConfirmed {
			return withValidation("Zayıf - hacim teyidi yok")
		}
		return withValidation(patternTradeMeaning(pattern))
	}
}

func patternValidationSummary(pattern ohlcv.PatternResult) string {
	if pattern.ValidationStatus == "" {
		return ""
	}
	if pattern.BacktestSampleSize <= 0 {
		return "istatistik: " + reportLabel(pattern.ValidationStatus)
	}
	return fmt.Sprintf("istatistik: %s, örnek=%d, kazanma %.1f%%, beklenen getiri %.2fR, p=%.3f",
		reportLabel(pattern.ValidationStatus),
		pattern.BacktestSampleSize,
		pattern.BacktestWinRate*100,
		pattern.BacktestExpectancyR,
		pattern.ValidationPValue,
	)
}

func patternTradeMeaning(pattern ohlcv.PatternResult) string {
	status := "Sinyal"
	if pattern.Actionable {
		status = "Aktif"
	}
	if pattern.Direction == "bearish" {
		status += " | spot alım için risk/uyarı"
	} else if pattern.Direction == "bullish" {
		status += " | alım lehine teknik kanıt"
	} else {
		status += " | yönsüz bağlam"
	}
	if pattern.BacktestReady {
		status += fmt.Sprintf(" | geçmiş test hazır, örnek %d, kazanma %.1f%%, beklenen getiri %.2fR", pattern.BacktestSampleSize, pattern.BacktestWinRate*100, pattern.BacktestExpectancyR)
	} else {
		status += " | geçmiş test hazır değil"
	}
	if pattern.VolumeConfirmed {
		status += " | hacim teyitli"
	} else {
		status += " | hacim teyidi yok"
	}
	return status
}

func patternEvidenceAndLevels(pattern ohlcv.PatternResult) string {
	parts := []string{}
	if !pattern.StartTime.IsZero() || !pattern.EndTime.IsZero() {
		parts = append(parts, fmt.Sprintf("Tarih %s - %s", formatReportDate(pattern.StartTime), formatReportDate(pattern.EndTime)))
	}
	if pattern.EntryMin > 0 || pattern.EntryMax > 0 || pattern.StopLoss > 0 || pattern.Target1 > 0 || pattern.Target2 > 0 {
		parts = append(parts, fmt.Sprintf("Seviye: giriş %s-%s, zarar kes/geçersiz %s, hedef %s/%s, risk/getiri %.2f", reportPriceValue(pattern.EntryMin), reportPriceValue(pattern.EntryMax), reportPriceValue(math.Max(pattern.StopLoss, pattern.InvalidationLevel)), reportPriceValue(pattern.Target1), reportPriceValue(pattern.Target2), pattern.RiskRewardRatio))
	}
	if evidence := reportEvidenceText(pattern.Evidence, pattern.RuleVersion); evidence != "" {
		parts = append(parts, evidence)
	}
	return strings.Join(parts, " | ")
}

func reportEvidenceText(evidence []string, fallback string) string {
	parts := []string{}
	for _, value := range evidence {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, localize.Evidence(value))
		if len(parts) >= 2 {
			break
		}
	}
	if len(parts) == 0 {
		return reportLabel(fallback)
	}
	return strings.Join(parts, "; ")
}

func patternRejectionText(reasons []string) string {
	if len(reasons) == 0 {
		return "Ret gerekçesi yok; düşük öncelikli aday."
	}
	parts := []string{}
	for _, reason := range reasons {
		parts = append(parts, patternRejectReasonTR(reason))
		if len(parts) >= 3 {
			break
		}
	}
	return strings.Join(parts, "; ")
}

func patternRejectReasonTR(reason string) string {
	switch reason {
	case "not_current_completed_pattern":
		return "Son tamamlanan mumda güncel formasyon değil"
	case "neutral_pattern_context_only":
		return "Yönsüz bağlam sinyali; tek başına işlem gerekçesi değil"
	case "not_directional_trade_setup":
		return "Alım/satım yönü için yeterli işlem kurulumu üretmiyor"
	case "backtest_metadata_not_ready":
		return "Geçmiş performans verisi hazır değil"
	case "calibrated_confidence_below_threshold":
		return "Kalibre güven eşiğin altında"
	case "directional_conflict_unresolved":
		return "Yön çatışması çözülemedi"
	case "directional_conflict_loser":
		return "Aynı grupta daha güçlü karşıt/benzer sinyal var"
	case "lower_priority_alias_or_duplicate":
		return "Daha güçlü eş/benzer formasyon tarafından elendi"
	case "actionable_signal_limit_exceeded":
		return "Aktif sinyal limiti aşıldığı için rapora öncelikli alınmadı"
	case "scientific_validation_insufficient_sample":
		return "Bilimsel doğrulama için yeterli tarihsel örnek yok"
	case "scientific_validation_not_statistically_significant":
		return "İstatistiksel anlamlılık eşiğini geçmedi"
	case "scientific_validation_no_positive_expectancy":
		return "Tarihsel testte pozitif beklenen getiri üretmedi"
	case "scientific_validation_no_historical_occurrences":
		return "Aynı kural için geçmişte doğrulanabilir tekrar bulunamadı"
	case "scientific_validation_external_data_required":
		return "Bu yapı dış veri gerektirdiği için OHLCV ile bilimsel doğrulanmadı"
	case "scientific_validation_no_reproducible_rule":
		return "Bu ad için tekrarlanabilir kesin kural yok"
	case "scientific_validation_not_run":
		return "Bilimsel doğrulama çalışmadı"
	default:
		return localize.Reason(reason)
	}
}

func reportLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Yok"
	}
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "backtest_samples_below_30:"):
		return "Nokta fiyat için en az 30 benzer geçmiş örnek yok (" + strings.TrimPrefix(lower, "backtest_samples_below_30:") + " örnek)"
	case strings.HasPrefix(lower, "backtest_direction_hit_below_55pct:"):
		return "Son backtest yön uyumu %55 eşiğinin altında (" + strings.TrimPrefix(lower, "backtest_direction_hit_below_55pct:") + "%)"
	case strings.HasPrefix(lower, "backtest_close_mae_above_1_25pct:"):
		return "Kapanış MAE %1.25 eşiğinin üstünde (" + strings.TrimPrefix(lower, "backtest_close_mae_above_1_25pct:") + "%)"
	case strings.HasPrefix(lower, "backtest_close_mape_above_2pct:"):
		return "Kapanış MAPE %2 eşiğinin üstünde (" + strings.TrimPrefix(lower, "backtest_close_mape_above_2pct:") + "%); fiyat/yön sayıları yayınlanmadı"
	case lower == "backtest_close_mae_missing":
		return "Backtest kapanış MAE metriği yok; nokta fiyat yayınlanmadı"
	case strings.HasPrefix(lower, "forecast_confidence_below_55:"):
		return "Forecast güveni %55 altında (" + strings.TrimPrefix(lower, "forecast_confidence_below_55:") + "/100)"
	}
	switch lower {
	case "forecast_not_publishable":
		return "Forecast yayınlama kapısı kapalı"
	case "forecast_model_validation_failed_not_decision_grade":
		return "Model geçmiş doğrulaması zayıf; fiyat/yön sayıları yayınlanmadı"
	case "not_decision_grade":
		return "Karar kalitesi değil; fiyat/yön sayıları yayınlanmadı"
	case "model_validation_failed":
		return "Geçmiş doğrulama zayıf"
	case "technical_decision_context_failed":
		return "Teknik karar bağlamı zayıf"
	case "forecast_quality_provisional":
		return "Forecast kalitesi provisional; nokta fiyat yayınlanmadı"
	case "forecast_not_decision_grade":
		return "Karar kalitesi değil; fiyat/yön sayıları yayınlanmadı"
	case "technical_decision_gate_not_passed":
		return "Teknik karar kapısı geçmedi"
	case "official_actual_available":
		return "Resmi gerçekleşen mevcut; model fiyatı yalnız audit içindir"
	case "forecast_not_computed":
		return "Forecast hesaplanmadı"
	case "yukselis_senaryosu_destek_altinda_gecersiz":
		return "Yükseliş senaryosu bu destek/bant altında geçersizleşir"
	case "dusus_senaryosu_direnc_ustunde_gecersiz":
		return "Düşüş senaryosu bu direnç/bant üstünde geçersizleşir"
	case "trend":
		return "Trend"
	case "momentum":
		return "Momentum"
	case "volume":
		return "Hacim"
	case "volatility":
		return "Oynaklık"
	case "support_resistance", "support resistance":
		return "Destek / direnç"
	case "price_action", "price action":
		return "Fiyat aksiyonu"
	case "chart":
		return "Grafik"
	case "classic_chart", "classic chart":
		return "Klasik grafik"
	case "candlestick":
		return "Mum formasyonu"
	case "indicator":
		return "İndikatör"
	case "wyckoff":
		return "Wyckoff"
	case "harmonic":
		return "Harmonik"
	case "volume_spread", "volume spread":
		return "Hacim / spread"
	case "statistical":
		return "İstatistik"
	case "cycle":
		return "Döngü"
	case "ta_lib", "ta lib":
		return "TA-Lib"
	case "other":
		return "Diğer"
	case "insufficient_data":
		return "Yetersiz veri"
	case "watch":
		return "Takip / izleme"
	case "hold":
		return "Bekle"
	case "tut_izle", "tut izle":
		return "TUT / İZLE"
	case "al":
		return "AL"
	case "buy":
		return "Al"
	case "sell":
		return "SAT"
	case "bekle":
		return "BEKLE"
	case "onayla":
		return "ONAYLA"
	case "reddet":
		return "REDDET"
	case "large_investor":
		return "Büyük yatırımcı"
	case "retail_direct_al_sat":
		return "Küçük yatırımcı doğrudan AL/SAT"
	case "value_investing":
		return "Değer yatırım"
	case "business":
		return "İşletme"
	case "management":
		return "Yönetim / sermaye tahsisi"
	case "financial":
		return "Finansal kalite"
	case "market":
		return "Piyasa / değerleme"
	case "buffett_deger_yatirimi_filtresi_gecmedi":
		return "Buffett değer yatırımı filtresi geçmedi"
	case "business_model_understandable":
		return "İş modeli anlaşılır değil"
	case "consistent_operating_history":
		return "Tutarlı faaliyet geçmişi eksik"
	case "consistent_operating_history_limited":
		return "Faaliyet geçmişi sınırlı"
	case "long_term_prospects":
		return "Uzun vadeli büyüme zemini zayıf"
	case "long_term_prospects_limited":
		return "Uzun vadeli büyüme zemini sınırlı"
	case "capital_allocation_discipline":
		return "Sermaye tahsisi zayıf"
	case "capital_allocation_discipline_limited":
		return "Sermaye tahsisi sınırlı"
	case "dilution_and_shareholder_alignment":
		return "Pay sulanması / hissedar hizalanması sorunu"
	case "dilution_and_shareholder_alignment_limited":
		return "Sermaye hareketi sınıflaması gerekli"
	case "one_dollar_retained_earnings_test":
		return "1 Dolar dağıtılmamış kâr testi eksik"
	case "roe_threshold":
		return "ROE eşiği geçmedi"
	case "roe_threshold_limited":
		return "ROE eşiği sınırlı"
	case "gross_margin_threshold":
		return "Brüt marj eşiği geçmedi"
	case "gross_margin_threshold_limited":
		return "Brüt marj eşiği sınırlı"
	case "net_margin_threshold":
		return "Net marj eşiği geçmedi"
	case "net_margin_threshold_limited":
		return "Net marj eşiği sınırlı"
	case "owner_earnings_quality":
		return "Owner earnings kalitesi geçmedi"
	case "owner_earnings_quality_limited":
		return "Owner earnings kalitesi sınırlı"
	case "capex_intensity":
		return "Sermaye harcaması yoğunluğu geçmedi"
	case "capex_intensity_limited":
		return "Sermaye harcaması yoğunluğu sınırlı"
	case "cash_vs_debt":
		return "Nakit/borç dayanıklılığı geçmedi"
	case "cash_vs_debt_limited":
		return "Nakit/borç dayanıklılığı sınırlı"
	case "moat_proxy":
		return "Ekonomik hendek proxy'si zayıf"
	case "moat_proxy_limited":
		return "Ekonomik hendek proxy'si sınırlı"
	case "intrinsic_value_computed":
		return "İçsel değer hesaplanamadı"
	case "margin_of_safety":
		return "Güvenlik marjı yok"
	case "margin_of_safety_limited":
		return "Güvenlik marjı sınırlı"
	case "valuation_assumptions":
		return "Değerleme varsayımları eksik"
	case "sector_model":
		return "sektör modeli"
	case "5y_financial_history":
		return "en az 5 yıllık finansal geçmiş"
	case "5y_revenue_history":
		return "5 yıllık gelir geçmişi"
	case "5y_roe_history":
		return "5 yıllık ROE geçmişi"
	case "5y_gross_margin_history":
		return "5 yıllık brüt marj geçmişi"
	case "5y_net_margin_history":
		return "5 yıllık net marj geçmişi"
	case "capital_movement_classification":
		return "bedelli/bedelsiz/split sermaye hareketi sınıflaması"
	case "retained_earnings_history":
		return "dağıtılmamış kâr geçmişi"
	case "market_cap_history":
		return "5 yıllık piyasa değeri geçmişi"
	case "market_cap_change_5y_divided_by_retained_earnings_proxy":
		return "5Y piyasa değeri değişimi / içeride tutulan kâr proxy"
	case "retained_earnings_test_requires_6y_financial_history":
		return "1 Dolar testi için 6 yıllık finansal geçmiş gerekli"
	case "retained_earnings_test_paid_capital_missing":
		return "1 Dolar testi için ödenmiş sermaye eksik"
	case "retained_earnings_test_market_price_history_missing":
		return "1 Dolar testi için 5 yıllık fiyat geçmişi eksik"
	case "retained_earnings_test_retained_earnings_non_positive":
		return "1 Dolar testi için içeride tutulan kâr pozitif değil"
	case "retained_earnings_test_dividend_history_missing_proxy_uses_net_income":
		return "temettü geçmişi eksik; proxy net kârı kullanıyor"
	case "positive_owner_earnings":
		return "pozitif owner earnings"
	case "capex_or_net_income_history":
		return "CapEx veya net kâr geçmişi"
	case "cash_and_debt_history":
		return "nakit ve borç geçmişi"
	case "intrinsic_value_inputs":
		return "içsel değer girdileri"
	case "current_price_or_intrinsic_value":
		return "güncel fiyat veya içsel değer girdisi"
	case "valuation_assumptions_source":
		return "değerleme varsayım kaynağı"
	case "trading_edge":
		return "İstatistiksel işlem avantajı"
	case "institutional_portfolio":
		return "Kurumsal portföy"
	case "automatic_order":
		return "Otomatik emir"
	case "research_report":
		return "Araştırma raporu"
	case "evidence_policy":
		return "Kanıt politikası"
	case "decision_price":
		return "Karar fiyatı"
	case "verified_final_close":
		return "Resmi/final kapanış"
	case "financial_integrity":
		return "Finansal mutabakat"
	case "valuation_consistency":
		return "Değerleme model tutarlılığı"
	case "sector_model_alignment":
		return "Sektör-model uyumu"
	case "value_thesis":
		return "Değer yatırım tezi"
	case "technical_signal":
		return "Teknik sinyal"
	case "active_trade_plan":
		return "Aktif giriş/stop/hedef planı"
	case "macro_regime":
		return "Makro rejim"
	case "model_risk":
		return "Model güveni"
	case "market_microstructure":
		return "Piyasa mikroyapısı"
	case "automatic_execution":
		return "Otomatik emir execution"
	case "al_sat_sinyali_hazir":
		return "AL/SAT SİNYALİ HAZIR"
	case "portfoye_al":
		return "PORTFÖYE AL"
	case "portfoye_alma":
		return "PORTFÖYE ALMA"
	case "islem_sinyali_hazir":
		return "İŞLEM SİNYALİ HAZIR"
	case "islem_acma":
		return "İŞLEM AÇMA"
	case "emir_acik":
		return "EMİR AÇIK"
	case "emir_kapali":
		return "EMİR KAPALI"
	case "yayimla":
		return "YAYIMLA"
	case "yayimlama":
		return "YAYIMLAMA"
	case "komiteye_sun", "komiteye sun":
		return "KOMİTEYE SUN"
	case "pozisyon_ac", "pozisyon ac":
		return "POZİSYON AÇ"
	case "yeni_pozisyon_acma", "yeni pozisyon acma", "pozisyon_acma", "pozisyon acma":
		return "YENİ POZİSYON AÇMA"
	case "pozisyon_acma_riski_azalt", "pozisyon acma riski azalt":
		return "POZİSYON AÇMA / RİSKİ AZALT"
	case "sinirli_pozisyon_taslagi", "sinirli pozisyon taslagi":
		return "SINIRLI POZİSYON TASLAĞI"
	case "sat_risk_azalt", "sat risk azalt":
		return "SAT / RİSK AZALT"
	case "avoid":
		return "Uzak dur"
	case "not_available":
		return "Yok"
	case "no_confirmed_pattern":
		return "Aktif formasyon yok"
	case "pass", "passed":
		return "Geçti"
	case "fail", "failed":
		return "Geçmedi"
	case "warning":
		return "Uyarı"
	case "review", "review_required":
		return "İnsan kontrolü gerekli"
	case "limited":
		return "Sınırlı"
	case "blocked":
		return "Kapalı"
	case "in_scope":
		return "Kapsam içinde"
	case "out_of_scope":
		return "Kapsam dışında"
	case "radar_watch_only":
		return "Takip / radar amaçlı"
	case "decision_support_ready":
		return "Karar destek için hazır"
	case "production_ready_research":
		return "Araştırma için production hazır"
	case "data_collection_required":
		return "Veri tamamlama gerekli"
	case "decision_ready":
		return "Karar hazır"
	case "decision_ready_with_execution_limits":
		return "Karar hazır; otomatik emir sınırlı"
	case "decision_issued_with_limitations":
		return "Karar üretildi; güven sınırlı"
	case "decision_unavailable":
		return "Karar üretilemedi"
	case "official_final_close":
		return "Resmi final kapanış"
	case "price_source_reconciliation":
		return "Fiyat kaynak mutabakatı"
	case "verified_price_close":
		return "Doğrulanmış kapanış fiyatı"
	case "verified_publish_dates":
		return "Yayın tarihleri doğrulanmış"
	case "lookahead_safe_conservative_available_at":
		return "Geriye dönük bakışa karşı korumalı; tarih zinciri kısmi"
	case "current_period_time_safe_historical_publish_dates_partial":
		return "Güncel dönem zaman güvenli; tarihsel yayın tarihleri kısmi"
	case "unsafe_missing_or_future_available_at":
		return "Zaman güvenliği yok veya gelecek tarih riski var"
	case "production_verified_subset_with_quarantine":
		return "Production için doğrulanmış alt küme; kalan dönemler karantinada"
	case "production_verified_publish_dates_only":
		return "Production için yayın tarihleri doğrulanmış"
	case "production_blocked_no_verified_publish_dates":
		return "Production kapalı; doğrulanmış yayın tarihi yok"
	case "missing_financial_statements":
		return "Finansal tablolar eksik"
	case "missing_financial_periods":
		return "Finansal dönem metadata'sı eksik"
	case "rejected":
		return "Reddedildi"
	case "duplicate":
		return "Tekrarlı kayıt"
	case "ingest":
		return "Veri içe aktarma"
	case "hash":
		return "Benzersiz dosya izi"
	case "source":
		return "Kaynak"
	case "source_file":
		return "Kaynak dosya"
	case "ticker":
		return "Sembol"
	case "generated_at":
		return "Üretim zamanı"
	case "counts":
		return "Sayımlar"
	case "nodes":
		return "Düğümler"
	case "edges":
		return "İlişkiler"
	case "duplicate_merge_summary":
		return "Tekrarlı kayıt birleştirme özeti"
	case "contradictions":
		return "Eski veri mutabakat alanı"
	case "resolved_reconciliations":
		return "Otomatik çözülen mutabakat kayıtları"
	case "sha256":
		return "Dosya izi"
	case "document_date":
		return "Belge tarihi"
	case "period":
		return "Dönem"
	case "event_type":
		return "Olay türü"
	case "title":
		return "Başlık"
	case "statement_type":
		return "Tablo türü"
	case "table_type":
		return "Tablo türü"
	case "line_item_original":
		return "Orijinal kalem adı"
	case "line_item_normalized":
		return "Standart kalem adı"
	case "currency":
		return "Para birimi"
	case "full_name":
		return "Ad soyad"
	case "normalized_name":
		return "Standart ad"
	case "holder_name":
		return "Pay sahibi"
	case "share_amount":
		return "Pay tutarı/adedi"
	case "company_knowledge_graph":
		return "Belge ilişki ağı"
	case "raw_documents":
		return "Ham PDF belgeleri"
	case "financial_statement":
		return "Finansal tablo"
	case "balance_sheet":
		return "Bilanço"
	case "income_statement":
		return "Gelir tablosu"
	case "cash_flow", "cash_flow_statement":
		return "Nakit akış tablosu"
	case "valuation_report":
		return "Değerleme raporu"
	case "annual_report":
		return "Faaliyet raporu"
	case "interim_activity_report":
		return "Ara dönem faaliyet raporu"
	case "material_event":
		return "Özel durum açıklaması"
	case "asset_event":
		return "Varlık olayı"
	case "knowledge_graph":
		return "Belge ilişki ağı"
	case "document_index":
		return "Belge dizini"
	case "financial_facts":
		return "Finansal satır adayları"
	case "financial_tables":
		return "Finansal tablo blokları"
	case "people":
		return "Kişiler / yönetim"
	case "ownership_facts":
		return "Ortaklık kayıtları"
	case "corporate_events":
		return "Kurumsal olaylar"
	case "cross_document_relationships", "cross document relationships":
		return "Belgeler arası ilişki kontrolü"
	case "kap_event_timeline", "kap event timeline":
		return "KAP olay zaman çizelgesi"
	case "financial_statement_analysis", "financial statement analysis":
		return "Finansal tablo analizi"
	case "ownership_and_capital_structure", "ownership and capital structure":
		return "Ortaklık ve sermaye yapısı"
	case "governance_and_management", "governance and management":
		return "Yönetim ve kurumsal yapı"
	case "fully_reportable", "fully reportable":
		return "Raporda tam gösterilebilir"
	case "loaded_without_source_path", "loaded without source path":
		return "Kaynak yolu olmadan yüklendi"
	case "financial_value_conflict":
		return "Finansal değer farkı"
	case "bist_official_unprocessed_ohlcv":
		return "BIST resmi günlük bülten OHLCV"
	case "bist_official_ohlcv_missing":
		return "BIST resmi bülten OHLCV eksik"
	case "bist_official_ohlcv_read_error":
		return "BIST resmi bülten OHLCV okunamadı"
	case "bist_official_not_point_in_time_safe":
		return "BIST resmi bülten zaman güvenliği eksik"
	case "bist_official_analysis_close_not_confirmed":
		return "Analiz kapanışı BIST resmi bültenle doğrulanamadı"
	case "vap_free_float_xlsx":
		return "VAP fiili dolaşım XLSX"
	case "vap_bist_index_portfolio":
		return "VAP BIST endeks portföyü"
	case "bist_live_websocket_snapshot":
		return "BIST canlı piyasa görüntüsü"
	case "bist_market_microstructure":
		return "BIST mikroyapı verisi"
	case "brokerage_distribution_akd":
		return "Aracı kurum dağılımı"
	case "custody_takas":
		return "Takas/saklama dağılımı"
	case "tuik_gdp_macro_context":
		return "TÜİK büyüme makro bağlamı"
	case "tuik_inflation_indices":
		return "TÜİK enflasyon endeksleri"
	case "tcmb_macro_context":
		return "TCMB makro metin bağlamı"
	case "tcmb_evds_series_context":
		return "TCMB EVDS seri bağlamı"
	case "recent_kap_news_disclosures":
		return "Son KAP ve haber açıklamaları"
	case "kap_asset_inventory":
		return "KAP varlık envanteri"
	case "benchmark_relative_strength":
		return "BIST benchmark göreli güç"
	case "sector_benchmark_relative_strength":
		return "Sektör benchmark göreli güç"
	case "kap_pdf_financial_reading_requires_review":
		return "KAP PDF finansal okuma insan kontrolü gerektiriyor"
	case "bist_price_step_applied_to_tradable_forecast":
		return "Tahmin fiyatları BIST fiyat adımına yuvarlandı"
	case "forecast_prices_cross_multiple_bist_tick_bands":
		return "Tahmin fiyatları birden fazla BIST fiyat adımı bandından geçiyor"
	case "low":
		return "düşük"
	case "medium":
		return "orta"
	case "high":
		return "yüksek"
	case "peer_comparison":
		return "Benzer şirket karşılaştırması"
	case "peer_comparison_min_3":
		return "En az 3 benzer şirket gerekli"
	case "peer_median_pb":
		return "benzer şirketlerin PD/DD medyanı"
	case "peer_median_ps":
		return "benzer şirketlerin FD/Satış veya satış çarpanı medyanı"
	case "peer_median_pe":
		return "benzer şirketlerin F/K medyanı"
	case "peer_multiple_discount":
		return "benzer şirketlere göre düşük çarpan"
	case "peer_median_valuation":
		return "benzer şirket medyan değerlemesi"
	case "peer_multiple_re_rating":
		return "benzer şirket çarpanlarına yakınsama"
	case "current_trend_and_liquidity_state":
		return "mevcut trend ve işlem hacmi durumu"
	case "latest_financial_run_rate":
		return "son finansal performansın yıllıklandırılmış temposu"
	case "relative_strength_improvement":
		return "endekse göre güçlenme"
	case "upside_technical_continuation":
		return "yukarı yönlü teknik devam ihtimali"
	case "technical_stop_risk_case":
		return "teknik zarar kes / risk senaryosu"
	case "gyo_nav_proxy":
		return "GYO için yaklaşık NAD/defter değeri modeli"
	case "suppressed_by_strict_evidence_policy":
		return "Sıkı kanıt politikası nedeniyle hesap bastırıldı"
	case "nav_not_reconciled_portfolio_totals_missing":
		return "Tam NAD mutabakatı yok; güncel portföy toplamı eksik"
	case "nav_not_reconciled":
		return "NAD tam mutabık değil"
	case "missing_portfolio_total":
		return "güncel portföy toplamı eksik"
	case "not_linked_to_valuation_portfolio_totals_missing":
		return "Varlık listesi değerlemeye bağlanmadı; güncel portföy toplamı eksik"
	case "bank_residual_income":
		return "Banka özsermaye getirisi modeli"
	case "book_per_share", "book per share", "bookvaluepershare", "book_value_per_share":
		return "hisse başına defter değeri"
	case "pb":
		return "PD/DD"
	case "pe":
		return "F/K"
	case "roe":
		return "ROE"
	case "roa":
		return "ROA"
	case "nim":
		return "Net faiz marjı"
	case "npl":
		return "Takipteki kredi oranı"
	case "lcr":
		return "Likidite karşılama oranı"
	case "capital_adequacy_ratio":
		return "sermaye yeterlilik rasyosu"
	case "cet1_ratio":
		return "çekirdek sermaye oranı"
	case "provision_coverage_ratio":
		return "karşılık kapsam oranı"
	case "net_interest_margin":
		return "net faiz marjı"
	case "liquidity_coverage_ratio":
		return "likidite karşılama oranı"
	case "loan_to_deposit_ratio":
		return "kredi/mevduat oranı"
	case "bank_regulatory_metrics_missing", "bank_specific_regulatory_metrics_missing":
		return "Banka çekirdek regülasyon/aktif kalitesi metrikleri eksik"
	case "financial_period_chronology_invalid":
		return "Finansal dönem tarih sırası geçersiz"
	case "publish_date_before_period_end":
		return "Yayın tarihi dönem sonundan önce görünüyor"
	case "report_date_before_period_end":
		return "Rapor tarihi dönem sonundan önce görünüyor"
	case "available_at_before_period_end":
		return "Kullanılabilir tarih dönem sonundan önce görünüyor"
	case "financial_data_not_backtest_safe":
		return "Finansal veri geçmiş test için güvenli değil"
	case "statement_version_store_missing":
		return "Finansal tablo sürüm deposu eksik"
	case "listed_delisted_universe_source_missing":
		return "Listelenen/kottan çıkan şirket evreni kaynağı eksik"
	case "actual_publish_dates_incomplete":
		return "Gerçek yayın tarihleri eksik"
	case "historical_financial_publish_dates_partial":
		return "Tarihsel finansal yayın tarihleri kısmi"
	case "financial_available_at_source_unverified":
		return "Finansal kullanılabilir tarih kaynağı doğrulanmadı"
	case "production_financial_publish_dates_missing":
		return "Production finansal yayın tarihleri eksik"
	case "unverified_financial_periods_quarantined_for_production":
		return "Doğrulanmamış finansal dönemler production için karantinada"
	case "asset_value_scale_suspicious_low":
		return "Varlık değerinde birim/ölçek doğrulanmalı"
	case "asset_valuation_report_code_requires_ownership_review":
		return "Değerleme raporu kodu mülkiyet eşleşmesi gerektiriyor"
	case "asset_inventory_full_json_contains_more_events_than_html_summary":
		return "Tam JSON, HTML özetinden daha fazla varlık olayı içeriyor"
	case "asset_inventory_cross_ticker_valuation_rows_filtered":
		return "Başka sembol kodlu değerleme satırları ana varlık tablosundan filtrelendi"
	case "asset_inventory_strict_display_filter_applied":
		return "Sıkı varlık gösterim filtresi uygulandı"
	case "asset_inventory_display_assets_empty":
		return "Güvenli gösterilebilir varlık satırı yok"
	case "asset_inventory_portfolio_vat_totals_missing":
		return "Portföy KDV dahil/hariç toplamı eksik"
	case "portfolio_totals_available_but_nav_bridge_required":
		return "Portföy toplamı var; NAD köprüsü yine doğrulanmalı"
	case "full_nav_reconciled":
		return "Tam NAD mutabakatı var"
	case "negative":
		return "negatif"
	case "weak":
		return "zayıf"
	case "critical":
		return "kritik"
	case "controlled":
		return "kontrollü"
	case "strong":
		return "güçlü"
	case "neutral":
		return "nötr"
	case "bullish":
		return "yükseliş"
	case "bearish":
		return "düşüş"
	case "oversold":
		return "aşırı satım"
	case "overbought":
		return "aşırı alım"
	case "context_required":
		return "sektör bağlamı gerekli"
	case "no_confirmed_pattern_selected":
		return "Doğrulanmış formasyon seçilmedi"
	case "technical_signal_gate_not_passed":
		return "Teknik sinyal kapısı geçilmedi"
	case "negative_ttm_net_income":
		return "son 12 ay net kâr negatif"
	case "negative_ttm_free_cash_flow":
		return "son 12 ay serbest nakit akımı negatif"
	case "negative_fcf":
		return "serbest nakit akımı negatif"
	case "negative_earnings_cash_flow_flags":
		return "kâr ve nakit akışı tarafında negatif uyarılar"
	case "statistically_validated":
		return "istatistiksel doğrulandı"
	case "insufficient_sample":
		return "örnek yetersiz"
	case "insufficient_history":
		return "tarihçe yetersiz"
	case "not_statistically_significant":
		return "istatistiksel anlamlı değil"
	case "no_positive_expectancy":
		return "pozitif beklenen getiri yok"
	case "no_historical_occurrences":
		return "geçmiş tekrar yok"
	case "external_data_required":
		return "dış veri gerekli"
	case "no_reproducible_rule":
		return "tekrarlanabilir kural yok"
	case "not_tested", "not_statistically_validated":
		return "doğrulanmadı"
	case "rapor_karari_dogrudan_al_degildir":
		return "Rapor doğrudan AL kararı vermiyor"
	case "tam_nad_mutabakati_yok":
		return "Tam NAD mutabakatı yok"
	case "kira_doluluk_eslesmesi_yok":
		return "Kira ve doluluk verisi varlık bazında eşleşmedi"
	case "varlik_envanteri_degerlemeye_tam_bagli_degildir":
		return "Varlık envanteri değerleme modeline tam bağlanmadı"
	case "finansal_kalite_kirmizi_bayraklari_var":
		return "Finansal kalite kırmızı bayrakları var"
	case "gereken_guvenlik_marji_yok":
		return "Gereken güvenlik marjı yok"
	case "model_guveni_kurumsal_esigin_altinda":
		return "Model güveni kurumsal eşik altında"
	case "strict_evidence_policy_failed":
		return "Sıkı kanıt politikası geçilmedi"
	case "valuation_confidence_below_70":
		return "Değerleme güveni 70/100 altında"
	case "pdf_parse_kalite_kapisi_tam_gecilmedi":
		return "PDF okuma kalite kapısı tam geçilmedi"
	case "investment_research_nav_not_reconciled":
		return "NAD tam mutabık değil"
	case "investment_research_rental_mapping_missing":
		return "Kira/doluluk eşlemesi eksik"
	case "investment_research_financial_quality_flags_present":
		return "Finansal kalite uyarıları var"
	case "onchain_mvrv_nupl_sopr_realized_cap":
		return "blokzincir değerleme verileri"
	case "derivatives_funding_open_interest_liquidations":
		return "fonlama, açık pozisyon ve tasfiye verileri"
	case "exchange_flow_reserve_netflow":
		return "borsa rezerv ve giriş-çıkış verileri"
	case "crypto_news_sentiment":
		return "kripto haber duyarlılığı"
	case "usd_index_dxy_real_yield_macro":
		return "DXY, reel faiz ve Fed beklentisi"
	case "futures_cot_open_interest_positioning":
		return "vadeli piyasa pozisyonu ve açık pozisyon"
	case "gold_etf_physical_flow":
		return "altın ETF ve fiziki talep akışı"
	case "central_bank_geopolitical_news":
		return "merkez bankası ve jeopolitik haber akışı"
	case "tradingview_ohlcv_price_volume":
		return "TradingView fiyat ve hacim verisi"
	case "technical_indicators":
		return "teknik göstergeler"
	case "walk_forward_price_backtest":
		return "geçmiş fiyat testi"
	case "tradingview_ohlcv":
		return "TradingView fiyat verisi"
	case "tradingview_ohlcv+crypto_context":
		return "TradingView fiyat verisi ve kripto ek verileri"
	case "tradingview_ohlcv+commodity_context":
		return "TradingView fiyat verisi ve altın/emtia ek verileri"
	}
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	return value
}

func reportLabels(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, reportLabel(value))
	}
	return out
}

func reportValuationInputLabels(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if key, raw, ok := strings.Cut(value, "="); ok {
			out = append(out, reportLabel(key)+" "+strings.TrimSpace(raw))
			continue
		}
		out = append(out, reportLabel(value))
	}
	return out
}

func joinReportLabels(values []string) string {
	labels := reportLabels(values)
	if len(labels) == 0 {
		return "Yok"
	}
	return strings.Join(labels, ", ")
}

func reportText(value string) string {
	value = redactInternalReportText(value)
	replacements := []string{
		"kap_pdf_ingest_missing", "kap_pdf_ingest_missing",
		"Trading edge", "İstatistiksel işlem avantajı",
		"trading edge", "istatistiksel işlem avantajı",
		"Walk-forward backtest", "İleri yürütmeli geçmiş test",
		"walk-forward backtest", "ileri yürütmeli geçmiş test",
		"Walk-forward", "İleri yürütmeli test",
		"walk-forward", "ileri yürütmeli test",
		"out-of-sample", "ileri dönem test",
		"Out-of-sample", "İleri dönem test",
		"backtest", "geçmiş test",
		"Backtest", "Geçmiş test",
		"OOS", "ileri dönem test",
		"expectancy", "beklenen getiri",
		"Expectancy", "Beklenen getiri",
		"Owner earnings", "Sahibine kalan nakit",
		"owner earnings", "sahibine kalan nakit",
		"normalized FCF", "normalize serbest nakit akımı",
		"Normalized FCF", "Normalize serbest nakit akımı",
		"normalize FCF", "normalize serbest nakit akımı",
		"Normalize FCF", "Normalize serbest nakit akımı",
		"FCF", "serbest nakit akımı",
		"TTM", "son 12 ay",
		"5Y", "5 yıllık",
		"10Y", "10 yıllık",
		"Bear case", "Kötümser senaryo",
		"Base case", "Baz senaryo",
		"Bull case", "İyimser senaryo",
		"Bear ", "Kötümser ",
		"Base ", "Baz ",
		"Bull ", "İyimser ",
		"bear ", "kötümser ",
		"bull ", "iyimser ",
		"Knowledge Graph", "Belge ilişki ağı",
		"Knowledge graph", "Belge ilişki ağı",
		"knowledge graph", "belge ilişki ağı",
		"Graph JSON", "İlişki ağı JSON",
		"solution", "çözüm",
		"Solution", "Çözüm",
		"review required", "AI çözüm gerekli",
		"review", "AI çözüm",
		"rejected", "reddedilen",
		"duplicate", "tekrarlı kayıt",
		"processed", "işlenmiş",
		"ingest", "veri içe aktarma",
		"hash", "dosya izi",
		"asset event", "varlık olayı",
		"dedupe", "tekilleştirme",
		"Raw PDF", "Ham PDF",
		"balance_sheet", "Bilanço",
		"income_statement", "Gelir tablosu",
		"cash_flow_statement", "Nakit akış tablosu",
		"balance sheet", "bilanço",
		"income statement", "gelir tablosu",
		"cash flow statement", "nakit akış tablosu",
		"board of directors", "yönetim kurulu",
		"dividend", "temettü",
		"financial value conflict", "finansal değer farkı",
		"financial fact", "finansal satır adayı",
		"Financial fact", "Finansal satır adayı",
		"Belge fact", "Belge satırı",
		"cross document relationships", "belgeler arası ilişki kontrolü",
		"kap event timeline", "KAP olay zaman çizelgesi",
		"financial statement analysis", "finansal tablo analizi",
		"ownership and capital structure", "ortaklık ve sermaye yapısı",
		"governance and management", "yönetim ve kurumsal yapı",
		"fully reportable", "raporda tam gösterilebilir",
		"loaded without source path", "kaynak yolu olmadan yüklendi",
		"Kurumsal memo", "Kurumsal not",
		"memo", "not",
		"Required fixes", "Zorunlu düzeltmeler",
		"required fixes", "zorunlu düzeltmeler",
		"radar_watch_only", reportLabel("radar_watch_only"),
		"radar watch only", reportLabel("radar_watch_only"),
		"decision_support_ready", reportLabel("decision_support_ready"),
		"production_ready_research", reportLabel("production_ready_research"),
		"data_collection_required", reportLabel("data_collection_required"),
		"bank_residual_income", reportLabel("bank_residual_income"),
		"bank residual income", reportLabel("bank_residual_income"),
		"book_per_share", reportLabel("book_per_share"),
		"lookahead_safe_conservative_available_at", reportLabel("lookahead_safe_conservative_available_at"),
		"current_period_time_safe_historical_publish_dates_partial", reportLabel("current_period_time_safe_historical_publish_dates_partial"),
		"unsafe_missing_or_future_available_at", reportLabel("unsafe_missing_or_future_available_at"),
		"production_ready", "production kullanımı hazır",
		"current_decision_safe", "güncel karar zaman güvenli",
		"backtest_safe", "geçmiş test zaman güvenli",
		"financial_period_chronology_invalid", reportLabel("financial_period_chronology_invalid"),
		"publish_date_before_period_end", reportLabel("publish_date_before_period_end"),
		"available_at_before_period_end", reportLabel("available_at_before_period_end"),
		"bank_regulatory_metrics_missing", reportLabel("bank_regulatory_metrics_missing"),
		"bank_specific_regulatory_metrics_missing", reportLabel("bank_specific_regulatory_metrics_missing"),
		"bank_regulatory_metrics_missing_confidence_cap", "banka çekirdek metrik eksiği nedeniyle güven sınırı",
		"strict_evidence_policy_suppressed", "sıkı kanıt politikası nedeniyle değerleme bastırıldı",
		"walk_forward_sample_size_limited", "geçmiş test örnek sayısı sınırlı",
		"financial_data_not_production_ready", "finansal veri üretim kullanımı hazır değil",
		"SYR_CET1_NPL_NIM_LCR_kredi_mevduat_structured_veri_eksik", "SYR/CET1/NPL/NIM/LCR/kredi-mevduat yapılandırılmış veri eksik",
		"capital_adequacy_ratio", reportLabel("capital_adequacy_ratio"),
		"cet1_ratio", reportLabel("cet1_ratio"),
		"provision_coverage_ratio", reportLabel("provision_coverage_ratio"),
		"net_interest_margin", reportLabel("net_interest_margin"),
		"liquidity_coverage_ratio", reportLabel("liquidity_coverage_ratio"),
		"loan_to_deposit_ratio", reportLabel("loan_to_deposit_ratio"),
		"peer_multiple_discount", reportLabel("peer_multiple_discount"),
		"peer_median_valuation", reportLabel("peer_median_valuation"),
		"peer_multiple_re_rating", reportLabel("peer_multiple_re_rating"),
		"peer_comparison_min_3", reportLabel("peer_comparison_min_3"),
		"peer_comparison", reportLabel("peer_comparison"),
		"peer multiple discount", reportLabel("peer_multiple_discount"),
		"peer median valuation", reportLabel("peer_median_valuation"),
		"peer multiple re-rating", reportLabel("peer_multiple_re_rating"),
		"peer comparison", reportLabel("peer_comparison"),
		"Peer", "Benzer şirket",
		"peer", "benzer şirket",
		"moat", "rekabet gücü",
		"Moat", "Rekabet gücü",
		"drivers", "etkenler",
		"driver", "etken",
		"production trading", "canlı/otomatik işlem",
		"production", "canlı/otomatik kullanım",
		"Production", "Canlı/otomatik kullanım",
		"trading system", "işlem sistemi",
		"Trading system", "İşlem sistemi",
		"on-chain", "blokzincir",
		"On-chain", "Blokzincir",
		"derivatives", "türev piyasa",
		"Derivatives", "Türev piyasa",
		"exchange-flow", "borsa giriş-çıkış",
		"Exchange-flow", "Borsa giriş-çıkış",
		"gate", "kontrol kapısı",
		"Gate", "Kontrol kapısı",
		"sentiment", "haber/duygu tonu",
		"Sentiment", "Haber/duygu tonu",
		"liquidity", "işlem hacmi ve alım-satım kolaylığı",
		"Liquidity", "İşlem hacmi ve alım-satım kolaylığı",
		"volatility", "oynaklık",
		"Volatility", "Oynaklık",
		"momentum", "fiyat ivmesi",
		"Momentum", "Fiyat ivmesi",
		"validation", "doğrulama",
		"Validation", "Doğrulama",
		"confidence", "güven",
		"Confidence", "Güven",
		"score", "skor",
		"Score", "Skor",
		"breakout", "kırılım",
		"Breakout", "Kırılım",
		"retest", "yeniden test",
		"Retest", "Yeniden test",
		"pullback", "geri çekilme",
		"Pullback", "Geri çekilme",
		"stop loss", "zarar kes",
		"Stop loss", "Zarar kes",
		"entry", "giriş",
		"Entry", "Giriş",
		"target", "hedef",
		"Target", "Hedef",
		"INSUFFICIENT_DATA", reportLabel("insufficient_data"),
		"INSUFFICIENT DATA", reportLabel("insufficient_data"),
		"nav_not_reconciled_portfolio_totals_missing", reportLabel("nav_not_reconciled_portfolio_totals_missing"),
		"nav not reconciled portfolio totals missing", reportLabel("nav_not_reconciled_portfolio_totals_missing"),
		"nav_not_reconciled", reportLabel("nav_not_reconciled"),
		"nav not reconciled", reportLabel("nav_not_reconciled"),
		"missing_portfolio_total", reportLabel("missing_portfolio_total"),
		"missing portfolio total", reportLabel("missing_portfolio_total"),
		"not_linked_to_valuation_portfolio_totals_missing", reportLabel("not_linked_to_valuation_portfolio_totals_missing"),
		"not linked to valuation portfolio totals missing", reportLabel("not_linked_to_valuation_portfolio_totals_missing"),
		"gyo_nav_proxy", reportLabel("gyo_nav_proxy"),
		"suppressed_by_strict_evidence_policy", reportLabel("suppressed_by_strict_evidence_policy"),
		"no_confirmed_pattern_selected", reportLabel("no_confirmed_pattern_selected"),
		"technical_validation_gate_not_passed", "İndikatör/formasyon doğrulama kapısı geçmedi",
		"negative earnings/cash-flow flags", reportLabel("negative_earnings_cash_flow_flags"),
		"negative_ttm_net_income", reportLabel("negative_ttm_net_income"),
		"negative_ttm_free_cash_flow", reportLabel("negative_ttm_free_cash_flow"),
		"negative_fcf", reportLabel("negative_fcf"),
		"onchain_mvrv_nupl_sopr_realized_cap", reportLabel("onchain_mvrv_nupl_sopr_realized_cap"),
		"derivatives_funding_open_interest_liquidations", reportLabel("derivatives_funding_open_interest_liquidations"),
		"exchange_flow_reserve_netflow", reportLabel("exchange_flow_reserve_netflow"),
		"crypto_news_sentiment", reportLabel("crypto_news_sentiment"),
		"usd_index_dxy_real_yield_macro", reportLabel("usd_index_dxy_real_yield_macro"),
		"futures_cot_open_interest_positioning", reportLabel("futures_cot_open_interest_positioning"),
		"gold_etf_physical_flow", reportLabel("gold_etf_physical_flow"),
		"central_bank_geopolitical_news", reportLabel("central_bank_geopolitical_news"),
		"tradingview_ohlcv_price_volume", reportLabel("tradingview_ohlcv_price_volume"),
		"technical_indicators", reportLabel("technical_indicators"),
		"support_resistance", reportLabel("support_resistance"),
		"walk_forward_price_backtest", reportLabel("walk_forward_price_backtest"),
		"bist_official_unprocessed_ohlcv", reportLabel("bist_official_unprocessed_ohlcv"),
		"bist_official_ohlcv_missing", reportLabel("bist_official_ohlcv_missing"),
		"bist_official_ohlcv_read_error", reportLabel("bist_official_ohlcv_read_error"),
		"bist_official_not_point_in_time_safe", reportLabel("bist_official_not_point_in_time_safe"),
		"bist_official_analysis_close_not_confirmed", reportLabel("bist_official_analysis_close_not_confirmed"),
		"vap_free_float_xlsx", reportLabel("vap_free_float_xlsx"),
		"vap_bist_index_portfolio", reportLabel("vap_bist_index_portfolio"),
		"bist_market_microstructure", reportLabel("bist_market_microstructure"),
		"bist_live_websocket_snapshot", reportLabel("bist_live_websocket_snapshot"),
		"brokerage_distribution_akd", reportLabel("brokerage_distribution_akd"),
		"custody_takas", reportLabel("custody_takas"),
		"tuik_gdp_macro_context", reportLabel("tuik_gdp_macro_context"),
		"tuik_inflation_indices", reportLabel("tuik_inflation_indices"),
		"tcmb_macro_context", reportLabel("tcmb_macro_context"),
		"tcmb_evds_series_context", reportLabel("tcmb_evds_series_context"),
		"recent_kap_news_disclosures", reportLabel("recent_kap_news_disclosures"),
		"kap_asset_inventory", reportLabel("kap_asset_inventory"),
		"benchmark_relative_strength", reportLabel("benchmark_relative_strength"),
		"sector_benchmark_relative_strength", reportLabel("sector_benchmark_relative_strength"),
		"kap_pdf_financial_reading_requires_review", reportLabel("kap_pdf_financial_reading_requires_review"),
		"bist_price_step_applied_to_tradable_forecast", reportLabel("bist_price_step_applied_to_tradable_forecast"),
		"forecast_prices_cross_multiple_bist_tick_bands", reportLabel("forecast_prices_cross_multiple_bist_tick_bands"),
	}
	return normalizeVisibleCurrencyText(strings.TrimSpace(strings.NewReplacer(replacements...).Replace(value)))
}

func redactInternalReportText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "go run") ||
		strings.Contains(lower, "cmd/hissebot") ||
		strings.Contains(lower, "cmd/kap-ingest") ||
		strings.Contains(lower, "raw_documents") {
		return "İç veri güncelleme adımı gerekli; operasyon komutu yatırımcı raporunda gösterilmez."
	}
	if reportTextLooksLikePath(value) ||
		strings.Contains(lower, ".json") ||
		strings.Contains(lower, ".jsonl") ||
		strings.Contains(lower, "data/equities") ||
		strings.Contains(lower, "data/processed") {
		return "İç veri kaynağı raporda gizlendi."
	}
	return value
}

func reportTextLooksLikePath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\n\r|") {
		return false
	}
	if strings.HasPrefix(trimmed, "data/") || strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") || strings.HasPrefix(trimmed, "/") {
		return strings.Contains(trimmed, "/")
	}
	return strings.Count(trimmed, "/") >= 2
}

func reportTexts(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, reportText(value))
	}
	return out
}

func formatReportNumber(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.2f milyar", value/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.2f milyon", value/1_000_000)
	case abs >= 10_000:
		return fmt.Sprintf("%.0f", value)
	case abs >= 100:
		return fmt.Sprintf("%.2f", value)
	case abs >= 10:
		return fmt.Sprintf("%.3f", value)
	case abs >= 1:
		return fmt.Sprintf("%.4f", value)
	default:
		return fmt.Sprintf("%.6f", value)
	}
}

func reportPrice(value float64, currency string) string {
	return reportPriceValue(value) + " " + displayCurrency(currency)
}

func reportPriceValue(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1:
		return fmt.Sprintf("%.2f", value)
	case abs >= 0.1:
		return fmt.Sprintf("%.3f", value)
	case abs >= 0.01:
		return fmt.Sprintf("%.4f", value)
	case abs >= 0.001:
		return fmt.Sprintf("%.5f", value)
	default:
		return fmt.Sprintf("%.6f", value)
	}
}

func formatReportPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func formatReportDate(value interface{ Format(string) string }) string {
	if value == nil {
		return "yok"
	}
	formatted := value.Format("2006-01-02")
	if formatted == "0001-01-01" {
		return "yok"
	}
	return formatted
}

func countComputedIndicators(indicators []ohlcv.IndicatorResult) int {
	count := 0
	for _, item := range indicators {
		if item.Computed {
			count++
		}
	}
	return count
}

func countMatchedPatternScans(scans []ohlcv.PatternScanResult) int {
	count := 0
	for _, scan := range scans {
		if scan.Matched {
			count++
		}
	}
	return count
}

func countCurrentPatternCandidates(patterns []ohlcv.PatternResult) int {
	count := 0
	for _, pattern := range patterns {
		if pattern.Actionable || !hasReportPatternRejection(pattern, "not_current_completed_pattern") {
			count++
		}
	}
	return count
}

func hasReportPatternRejection(pattern ohlcv.PatternResult, reason string) bool {
	for _, rejection := range pattern.RejectionReasons {
		if strings.EqualFold(strings.TrimSpace(rejection), reason) {
			return true
		}
	}
	return false
}

func intCount(values map[string]int, key string, fallback int) int {
	if values == nil {
		return fallback
	}
	if value, ok := values[key]; ok {
		return value
	}
	return fallback
}

func timeframePlainComment(tf analysis.TimeframeAnalysis) string {
	if timeframeIsDailyContextWindow(tf) {
		return "Bu satır günlük serinin bağlam penceresidir; kapanış son günlük fiyatla aynı olabilir. Emir planı için Günlük ve Haftalık kapılar kullanılmalı."
	}
	if timeframeIsLongContext(tf) {
		if tf.TradePlan.Rejected && tf.TradePlan.RiskRewardRatio > 0 {
			return fmt.Sprintf("Uzun dönem bar devam ediyor; eğilim %s fakat işlem kapısı kapalı. RR %.2f, gereken en az 1.50; giriş için günlük/haftalık teyit gerekir.", retailTrendLabel(tf.TrendBias), tf.TradePlan.RiskRewardRatio)
		}
		if activeSummaryTradePlan(tf.TradePlan) {
			return "Uzun dönem eğilim bağlamı olumlu; bu satır doğrudan emir değil, günlük/haftalık giriş ve stop teyidi ister."
		}
	}
	if activeSummaryTradePlan(tf.TradePlan) {
		if tf.Indicators.MACDHistogram < 0 {
			return "Alım planı var fakat kısa vadeli ivme zayıf; giriş için kapanış ve hacim teyidi beklenmeli."
		}
		return "Alım planı var; giriş yalnızca teyit ve stop disipliniyle anlamlı."
	}
	if tf.TradePlan.Rejected {
		if isSpotLongUnavailableReject(tf.TradePlan) {
			if tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
				return fmt.Sprintf("Düşüş baskısı var; yeni alım için %s üstü kapanış, hacim ve momentum teyidi beklenir.", reportPriceValue(tf.NearestResistance.Price))
			}
			return "Düşüş baskısı var; yeni alım için trend dönüşü, hacim ve momentum teyidi beklenir."
		}
		if tf.TradePlan.RiskRewardRatio > 0 {
			return fmt.Sprintf("Teknik görünüm %s olsa da işlem kapısı kapalı; RR %.2f, gereken en az 1.50.", retailTrendLabel(tf.TrendBias), tf.TradePlan.RiskRewardRatio)
		}
		return "Aktif alım planı yok; fiyat, hacim ve momentum teyidi beklenir."
	}
	return "Aktif alım planı yok; fiyat teyidi beklenir."
}

func timeframePlainCommentForReport(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis) string {
	if technicalSignalBlockedForReport(tf) {
		if strings.EqualFold(strings.TrimSpace(tf.TrendBias), "bullish") || strings.EqualFold(strings.TrimSpace(tf.TrendBias), "up") {
			return "Teknik görünüm toparlanma denemesi; hacim/formasyon/işlem kapısı geçmediği için aktif işlem kurulumu yok."
		}
	}
	if reportTradePlanBlocked(result) && activeSummaryTradePlan(tf.TradePlan) {
		if timeframeIsLongContext(tf) {
			return "Uzun dönem eğilim bağlamı izlenebilir; AL/SAT kapısı geçmediği için bu satır emir planı değildir."
		}
		return "Teknik seviye bandı izleme amaçlıdır; AL/SAT kapısı geçmediği için aktif alım planı değildir."
	}
	return timeframePlainComment(tf)
}

func timeframeTechnicalAppearanceForReport(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis) string {
	if reportTradePlanBlocked(result) && technicalSignalBlockedForReport(tf) {
		if strings.EqualFold(strings.TrimSpace(tf.TrendBias), "bullish") || strings.EqualFold(strings.TrimSpace(tf.TrendBias), "up") {
			return "Toparlanma denemesi / işlem yok"
		}
		return localize.Bias(tf.TrendBias) + " / teyitsiz"
	}
	return localize.Bias(tf.TrendBias)
}

func technicalSignalBlockedForReport(tf analysis.TimeframeAnalysis) bool {
	gate := tf.Professional.Technical.SignalGate
	if gate.Status == "" {
		return false
	}
	return !reportStatusPass(gate.Status) || !gate.Actionable
}

func timeframeGateText(tf analysis.TimeframeAnalysis) string {
	if timeframeIsDailyContextWindow(tf) {
		return "Bağlam: günlük veri penceresi; doğrudan işlem kapısı üretmez"
	}
	if timeframeIsLongContext(tf) {
		if tf.TradePlan.Rejected && tf.TradePlan.RiskRewardRatio > 0 {
			return fmt.Sprintf("Kapalı: uzun dönem bağlamı; RR %.2f < 1.50, günlük/haftalık teyit gerekir", tf.TradePlan.RiskRewardRatio)
		}
		if activeSummaryTradePlan(tf.TradePlan) {
			return "Bağlam: uzun dönem eğilim olumlu; emir için günlük/haftalık kapı kullanılmalı"
		}
	}
	if activeSummaryTradePlan(tf.TradePlan) {
		return reportPlanText(tf.TradePlan)
	}
	if tf.TradePlan.Rejected && tf.TradePlan.RiskRewardRatio > 0 {
		return fmt.Sprintf("Kapalı: risk/getiri %.2f; aktif sinyal için en az 1.50 gerekir", tf.TradePlan.RiskRewardRatio)
	}
	if isSpotLongUnavailableReject(tf.TradePlan) {
		if tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
			return "Kapalı: aktif alım planı yok; " + reportPriceValue(tf.NearestResistance.Price) + " üstü kapanış ve hacim teyidi beklenir"
		}
		return "Kapalı: aktif alım planı yok; trend dönüşü ve hacim teyidi beklenir"
	}
	if tf.Professional.Technical.SignalGate.Status != "" && !tf.Professional.Technical.SignalGate.Actionable {
		return "Kapalı: teknik sinyal kapısı geçmedi; teyit beklenir"
	}
	if tf.TradePlan.RejectReason != "" {
		return "Kapalı: " + reportPlanRejectSummary(tf.TradePlan)
	}
	return "Kapalı: aktif işlem planı yok; teyit beklenir"
}

func timeframeGateTextForReport(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis) string {
	if reportTradePlanBlocked(result) && activeSummaryTradePlan(tf.TradePlan) {
		if timeframeIsDailyContextWindow(tf) {
			return timeframeGateText(tf)
		}
		if timeframeIsLongContext(tf) {
			return "Bağlam: eğilim izlenebilir; AL/SAT kapısı kapalı olduğu için emir üretmez"
		}
		if tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
			return "Kapalı: AL/SAT kapısı geçmedi; " + retailPrice(tf.NearestResistance.Price, result.Currency) + " üstü kapanış yalnızca izleme teyididir"
		}
		return "Kapalı: AL/SAT kapısı geçmedi; teknik giriş bandı emir planı değildir"
	}
	if activeSummaryTradePlan(tf.TradePlan) {
		return reportPlanTextWithCurrency(tf.TradePlan, result.Currency)
	}
	if isSpotLongUnavailableReject(tf.TradePlan) && tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
		return "Kapalı: aktif alım planı yok; " + retailPrice(tf.NearestResistance.Price, result.Currency) + " üstü kapanış ve hacim teyidi beklenir"
	}
	return timeframeGateText(tf)
}

func timeframeIsDailyContextWindow(tf analysis.TimeframeAnalysis) bool {
	switch strings.ToUpper(strings.TrimSpace(tf.Timeframe)) {
	case "YTD", "ALL":
		return true
	default:
		return false
	}
}

func timeframeIsLongContext(tf analysis.TimeframeAnalysis) bool {
	switch strings.ToUpper(strings.TrimSpace(tf.Timeframe)) {
	case "1M", "3M", "6M", "1Y":
		return true
	default:
		return false
	}
}

func timeframeLastBarText(tf analysis.TimeframeAnalysis) string {
	if len(tf.Candles) == 0 {
		return "-"
	}
	last := tf.Candles[len(tf.Candles)-1].Time
	if last.IsZero() {
		return "-"
	}
	return last.Format("2006-01-02")
}

func timeframeCandleWindowText(tf analysis.TimeframeAnalysis) string {
	count := tf.CandleCount
	if count <= 0 {
		count = len(tf.Candles)
	}
	if count <= 0 {
		return "-"
	}
	unit := "bar"
	switch strings.ToUpper(strings.TrimSpace(tf.Timeframe)) {
	case "YTD":
		unit = "günlük bar; yıl içi pencere"
	case "ALL":
		unit = "günlük bar; son veri penceresi"
	case "1D":
		unit = "günlük bar"
	case "1W":
		unit = "haftalık bar"
	case "1M":
		unit = "aylık bar"
	case "3M":
		unit = "3 aylık bar"
	case "6M":
		unit = "6 aylık bar"
	case "1Y":
		unit = "yıllık bar"
	}
	return fmt.Sprintf("%d %s", count, unit)
}

func reportPlanText(plan ohlcv.TradePlan) string {
	if activeSummaryTradePlan(plan) {
		return fmt.Sprintf("Giriş %s-%s | Stop %s | Hedef %s/%s | RR %.2f", reportPriceValue(plan.EntryMin), reportPriceValue(plan.EntryMax), reportPriceValue(plan.StopLoss), reportPriceValue(plan.TakeProfit1), reportPriceValue(plan.TakeProfit2), plan.RiskRewardRatio)
	}
	if plan.Rejected && plan.RiskRewardRatio > 0 {
		return fmt.Sprintf("Kapalı: RR %.2f < 1.50 | %s", plan.RiskRewardRatio, reportPlanRejectSummary(plan))
	}
	if plan.RejectReason != "" {
		return "Kapalı: " + reportPlanRejectSummary(plan)
	}
	return "Kapalı: aktif işlem planı yok"
}

func reportPlanTextWithCurrency(plan ohlcv.TradePlan, currency string) string {
	if activeSummaryTradePlan(plan) {
		return fmt.Sprintf("Giriş %s-%s | Stop %s | Hedef %s/%s | RR %.2f", retailPrice(plan.EntryMin, currency), retailPrice(plan.EntryMax, currency), retailPrice(plan.StopLoss, currency), retailPrice(plan.TakeProfit1, currency), retailPrice(plan.TakeProfit2, currency), plan.RiskRewardRatio)
	}
	return reportPlanText(plan)
}

func reportPlanTextForReport(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis) string {
	if reportTradePlanBlocked(result) && activeSummaryTradePlan(tf.TradePlan) {
		return "Kapalı: AL/SAT kapısı geçmedi; teknik giriş bandı yalnızca izleme/teyit bilgisidir"
	}
	return reportPlanTextWithCurrency(tf.TradePlan, result.Currency)
}

func reportPlanRejectSummary(plan ohlcv.TradePlan) string {
	if isSpotLongUnavailableReject(plan) {
		return "aktif alım planı yok; düşüş sinyali spot alım kurulumu üretmiyor"
	}
	return localize.Reason(plan.RejectReason)
}

func isSpotLongUnavailableReject(plan ohlcv.TradePlan) bool {
	reason := strings.ToLower(localize.Reason(plan.RejectReason) + " " + plan.RejectReason + " " + strings.Join(plan.Reasoning, " "))
	return strings.Contains(reason, "short") ||
		strings.Contains(reason, "marjin") ||
		strings.Contains(reason, "spot varlık") ||
		strings.Contains(reason, "spot varlik") ||
		strings.Contains(reason, "spot long") ||
		strings.Contains(reason, "düşüş sinyali") ||
		strings.Contains(reason, "dusus sinyali") ||
		strings.Contains(reason, "düşüş yönlü teknik kanıt") ||
		strings.Contains(reason, "dusus yonlu teknik kanit") ||
		strings.Contains(reason, "bearish evidence")
}

func reportScenarioReturn(result analysis.SymbolAnalysis, name string) (float64, bool) {
	for _, scenario := range result.Professional.Scenarios {
		if strings.EqualFold(scenario.Name, name) {
			return scenario.ReturnPct, true
		}
	}
	return 0, false
}

func reportReadingNotes(result analysis.SymbolAnalysis) []string {
	daily, ok := result.Timeframes["1D"]
	if !ok {
		return []string{"Günlük veri yok; okuma notları yalnızca sınırlı zaman dilimi verisine göre okunmalıdır."}
	}
	notes := []string{}
	add := func(text string) {
		if strings.TrimSpace(text) == "" || len(notes) >= 7 {
			return
		}
		notes = append(notes, text)
	}

	ind := daily.Indicators
	switch {
	case ind.MACDHistogram < 0:
		add(fmt.Sprintf("Günlük MACD histogramı %.4f: kısa vadeli momentum negatif; alım için hızlanma teyidi henüz yok.", ind.MACDHistogram))
	case ind.MACDHistogram > 0:
		add(fmt.Sprintf("Günlük MACD histogramı %.4f: kısa vadeli momentum pozitif; yine de hacim ve fiyat teyidi birlikte aranmalı.", ind.MACDHistogram))
	default:
		add("Günlük MACD histogramı nötr; momentum tek başına karar üretmiyor.")
	}
	if isEquityResult(result) && result.Professional.KAPPDFIngest.Computed {
		add("KAP PDF rapor kanıtı: " + result.Professional.KAPPDFIngest.Summary)
	}
	if ind.RSI14 > 0 {
		assetName := "hisse"
		if isCryptoResult(result) {
			assetName = "varlık"
		} else if isCommodityResult(result) {
			assetName = "altın/emtia"
		}
		switch {
		case ind.RSI14 >= 70:
			add(fmt.Sprintf("RSI14 %.2f: %s kısa vadede ısınmış; yeni alım için soğuma veya yatay güç toplama beklenmeli.", ind.RSI14, assetName))
		case ind.RSI14 <= 30:
			add(fmt.Sprintf("RSI14 %.2f: aşırı satım bölgesi; tepki potansiyeli olabilir fakat tek başına alım sinyali değildir.", ind.RSI14))
		case ind.RSI14 < 45:
			add(fmt.Sprintf("RSI14 %.2f: momentum zayıf bölgede; dönüş için RSI ve fiyatın birlikte toparlanması gerekir.", ind.RSI14))
		case ind.RSI14 <= 55:
			add(fmt.Sprintf("RSI14 %.2f: nötr bölgede; güçlü yön teyidi yok.", ind.RSI14))
		default:
			add(fmt.Sprintf("RSI14 %.2f: momentum pozitif bölgede; sinyalin hacim ve trendle desteklenmesi gerekir.", ind.RSI14))
		}
	}
	switch {
	case ind.ChaikinMoneyFlow20 < -0.05:
		add(fmt.Sprintf("Chaikin para akışı %.4f: alıcı para girişi zayıf/negatif; yükseliş denemesi teyitsiz kalıyor.", ind.ChaikinMoneyFlow20))
	case ind.ChaikinMoneyFlow20 > 0.05:
		add(fmt.Sprintf("Chaikin para akışı %.4f: para akışı pozitif; teknik dönüş için destekleyici unsur.", ind.ChaikinMoneyFlow20))
	}
	if daily.LastVolume > 0 && ind.VolumeSMA20 > 0 {
		ratio := daily.LastVolume / ind.VolumeSMA20
		if ratio < 0.8 {
			add(fmt.Sprintf("Son hacim 20 günlük ortalamanın %.2f katı; hacim teyidi zayıf.", ratio))
		} else if ratio >= 1.2 {
			add(fmt.Sprintf("Son hacim 20 günlük ortalamanın %.2f katı; hareket hacimle teyit alıyor.", ratio))
		} else {
			add(fmt.Sprintf("Son hacim 20 günlük ortalamanın %.2f katı; hacim tarafı nötr.", ratio))
		}
	}
	plan := daily.TradePlan
	if activeSummaryTradePlan(plan) {
		gate := daily.Professional.Technical.SignalGate
		if reportTradePlanBlocked(result) {
			add(fmt.Sprintf("İzleme/paper-trade seviyesi var: giriş %s-%s; AL/SAT kullanım kapısı geçmediği için aktif alım planı değildir.", retailPrice(plan.EntryMin, result.Currency), retailPrice(plan.EntryMax, result.Currency)))
		} else if gate.Status != "" && !gate.Actionable {
			add(fmt.Sprintf("İzleme/paper-trade seviyesi var: giriş %s-%s, stop %s, hedef %s/%s, risk/getiri %.2f; teknik sinyal kapısı geçmediği için aktif alım planı değildir.", retailPrice(plan.EntryMin, result.Currency), retailPrice(plan.EntryMax, result.Currency), retailPrice(plan.StopLoss, result.Currency), retailPrice(plan.TakeProfit1, result.Currency), retailPrice(plan.TakeProfit2, result.Currency), plan.RiskRewardRatio))
		} else {
			add(fmt.Sprintf("Aktif alım planı var: giriş %s-%s, stop %s, hedef %s/%s, risk/getiri %.2f.", retailPrice(plan.EntryMin, result.Currency), retailPrice(plan.EntryMax, result.Currency), retailPrice(plan.StopLoss, result.Currency), retailPrice(plan.TakeProfit1, result.Currency), retailPrice(plan.TakeProfit2, result.Currency), plan.RiskRewardRatio))
		}
		if plan.RiskRewardRatio < 1.5 {
			add(fmt.Sprintf("Risk/getiri %.2f: potansiyel kazanç alınan riski yeterince telafi etmiyor; plan zayıf kabul edilir.", plan.RiskRewardRatio))
		}
	} else if plan.Rejected {
		add(fmt.Sprintf("Aktif alım planı yok: %s. Bu nedenle stop/hedef seviyeleri işlem önerisi olarak yazılmaz.", reportPlanRejectSummary(plan)))
	} else {
		add("Aktif alım planı yok; fiyat, momentum ve hacim aynı anda teyit vermeden işlem planı üretilmez.")
	}
	bt := daily.Professional.Backtest
	if bt.LookbackBars > 0 || bt.Trades > 0 {
		add(fmt.Sprintf("İleri yürütmeli geçmiş test: %d işlem, kazanma %s, beklenen getiri %s, ileri dönem test ortalaması %s. Bu değerler pozitif değilse sinyal kurumsal işlem kararı olamaz.", bt.Trades, formatPct(bt.WinRate*100), formatPct(bt.Expectancy*100), formatPct(bt.OutOfSampleReturn*100)))
	}
	if result.InstitutionalValidation.Status != "" {
		add(fmt.Sprintf("Rapor güvenlik ve doğrulama kapısı: %s. %s", institutionalStatusTR(result.InstitutionalValidation.Status), emptyFallback(result.InstitutionalValidation.Summary, "Doğrulama özeti yok.")))
	}
	if result.InvestorQA.Computed {
		add("Yatırımcı soru-cevap özeti: " + result.InvestorQA.OneLineAnswer)
	}
	if isEquityResult(result) && result.Professional.SectorFinancials.Applicable && result.Professional.SectorFinancials.Summary != "" {
		add("Sektör bazlı bilanço yorumu: " + result.Professional.SectorFinancials.Summary)
	}
	if isEquityResult(result) && len(result.Professional.SectorFinancials.Warnings) > 0 {
		add("Sektör bilanço uyarıları: " + strings.Join(result.Professional.SectorFinancials.Warnings, ", "))
	}
	if isEquityResult(result) && result.Professional.ValueInvesting.Computed {
		v := result.Professional.ValueInvesting
		add(fmt.Sprintf("Değer yatırım özeti: baz içsel değer %.2f %s, güvenlik marjı %.1f%%, karar %s.", v.IntrinsicValue.Base, displayCurrency(result.Currency), v.MarginOfSafety.BasePct, v.DecisionLabel))
	} else if isEquityResult(result) && result.Professional.ValueInvesting.Decision != "" {
		v := result.Professional.ValueInvesting
		if isBankReport(result) {
			add(fmt.Sprintf("Değer yatırım özeti: %s. Bankada owner earnings/FCF ana girdi değildir; SYR, CET1, NPL, NIM, LCR, kredi/mevduat ve mevduat maliyeti tamamlanmadan içsel değer ve AL/SAT kapısı kapalı kalır. Sermaye tahsisi %.0f/100.", v.DecisionLabel, v.CapitalAllocation.Score))
		} else {
			add(fmt.Sprintf("Değer yatırım özeti: %s sahibine kalan nakit son 12 ay %s, 5 yıllık normalize serbest nakit akımı %s, sermaye tahsisi %.0f/100, rekabet gücü %.0f/100.", v.DecisionLabel, formatMoney(v.OwnerEarnings.TTM, result.Currency), formatMoney(v.NormalizedFCF.Median5Y, result.Currency), v.CapitalAllocation.Score, v.Moat.Score))
		}
	}
	if isMarketOnlyResult(result) && len(result.Professional.Coverage.Missing) > 0 {
		add(dataImprovementLine(result))
	}
	if isEquityResult(result) && isBankReport(result) && (result.Professional.Valuation.Ratios["ROE"] < 0 || result.Professional.Valuation.NetIncomeTTM < 0) {
		add(fmt.Sprintf("Banka bilanço uyarısı: ROE %s, net kâr %s; değerleme hedefleri aktif kalitesi, karşılık giderleri ve sermaye yeterliliğiyle birlikte okunmalı.", formatPct(result.Professional.Valuation.Ratios["ROE"]*100), formatMoney(result.Professional.Valuation.NetIncomeTTM, result.Currency)))
	} else if isEquityResult(result) && (result.Professional.Valuation.Ratios["ROE"] < 0 || result.Professional.Valuation.FreeCashFlowTTM < 0 || result.Professional.Valuation.NetIncomeTTM < 0) {
		add(fmt.Sprintf("Bilanço uyarısı: ROE %s, net kâr %s, serbest nakit akımı %s; değerleme hedefleri bu kalite riskiyle birlikte okunmalı.", formatPct(result.Professional.Valuation.Ratios["ROE"]*100), formatMoney(result.Professional.Valuation.NetIncomeTTM, result.Currency), formatMoney(result.Professional.Valuation.FreeCashFlowTTM, result.Currency)))
	}
	if len(notes) == 0 {
		return []string{"Bu rapor için dinamik okuma notu üretecek yeterli veri yok."}
	}
	return notes
}

func reportConfidenceFor(result analysis.SymbolAnalysis) reportConfidence {
	items := []reportConfidenceItem{}
	if isCryptoResult(result) {
		items = []reportConfidenceItem{
			technicalConfidence(result),
			cryptoDataConfidence(result),
			investorQAConfidence(result),
			institutionalValidationConfidence(result),
			presentationConfidence(),
		}
	} else if isCommodityResult(result) {
		items = []reportConfidenceItem{
			technicalConfidence(result),
			commodityDataConfidence(result),
			investorQAConfidence(result),
			institutionalValidationConfidence(result),
			presentationConfidence(),
		}
	} else {
		items = []reportConfidenceItem{
			technicalConfidence(result),
			financialGovernanceConfidence(result),
			sectorPeerConfidence(result),
			kapPDFEvidenceConfidence(result),
			valueInvestingConfidence(result),
			investorQAConfidence(result),
			valuationConfidence(result),
			institutionalValidationConfidence(result),
			presentationConfidence(),
		}
		if isBankReport(result) {
			items = append(items[:4], append([]reportConfidenceItem{bankMetricCompletenessConfidence(result)}, items[4:]...)...)
		}
	}
	total := 0.0
	maxTotal := 0.0
	for _, item := range items {
		total += item.Score
		maxTotal += item.Max
	}
	score := 0.0
	if maxTotal > 0 {
		score = 100 * total / maxTotal
	}
	switch strings.ToLower(strings.TrimSpace(result.InstitutionalValidation.Status)) {
	case "fail":
		score = math.Min(score, 59)
	case "limited":
		score = math.Min(score, 79)
	}
	if isEquityResult(result) && isBankReport(result) && bankCoreMetricsMissingForReport(result) {
		score = math.Min(score, 69)
	}
	if isEquityResult(result) && !result.Professional.DataGovernance.ProductionReady {
		score = math.Min(score, 82)
	}
	return reportConfidence{Score: math.Round(score), Items: items}
}

func reportDataCoverageScore(result analysis.SymbolAnalysis) float64 {
	score := result.Professional.Coverage.Score
	if score <= 0 {
		score = result.Professional.DataQuality
	}
	return math.Round(clampReport(score, 0, 100))
}

func bankMetricCompletenessConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	sf := result.Professional.SectorFinancials
	completeness := professional.BankRegulatoryMetricCompletenessScore(sf)
	score := clampReport(completeness/100*20, 0, 20)
	missing := professional.MissingBankRegulatoryMetricNames(sf)
	detail := "Banka ana metrikleri sertifikalı KAP kaynaklarıyla tamamlandı."
	if len(missing) > 0 {
		detail = "Eksik/sertifikasız banka metrikleri: " + strings.Join(bankRegulatoryMetricLabels(missing), ", ") + "."
	}
	return reportConfidenceItem{
		Label:  "Banka ana metrik tamlığı",
		Score:  score,
		Max:    20,
		Status: statusFor(score, 20),
		Detail: detail,
	}
}

func bankRegulatoryMetricLabels(names []string) []string {
	labels := make([]string, 0, len(names))
	for _, name := range names {
		switch strings.TrimSpace(name) {
		case "capital_adequacy_ratio":
			labels = append(labels, "SYR")
		case "cet1_ratio":
			labels = append(labels, "CET1/çekirdek sermaye")
		case "npl_ratio":
			labels = append(labels, "NPL/takipteki kredi oranı")
		case "provision_coverage_ratio":
			labels = append(labels, "karşılık kapsamı")
		case "net_interest_margin":
			labels = append(labels, "NIM/net faiz marjı")
		case "liquidity_coverage_ratio":
			labels = append(labels, "LCR/likidite karşılama oranı")
		case "loan_to_deposit_ratio":
			labels = append(labels, "kredi/mevduat oranı")
		case "deposit_cost":
			labels = append(labels, "mevduat maliyeti")
		case "loan_deposit_spread":
			labels = append(labels, "kredi/mevduat spread'i")
		default:
			labels = append(labels, name)
		}
	}
	return labels
}

func kapPDFEvidenceConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	kap := result.Professional.KAPPDFIngest
	if !kap.Computed || kap.TotalDocuments == 0 {
		return reportConfidenceItem{
			Label:  "KAP PDF kanıt kapsamı",
			Score:  0,
			Max:    15,
			Status: "Zayıf",
			Detail: "KAP PDF ingest çıktısı yok; PDF ekleri rapor kanıt zincirine bağlanmadı.",
		}
	}
	score := 3.0
	if kap.SourcePDFCount == 0 || kap.MissingUniqueHashes == 0 {
		score += 2
	}
	usableRatio := safeReportRatio(float64(kap.AnalysisUsableCount), float64(kap.TotalDocuments))
	score += clampReport(usableRatio/0.45, 0, 1) * 4
	decisionRelevantRatio := safeReportRatio(float64(kap.DecisionRelevantUsableCount), float64(kap.DecisionRelevantDocuments))
	if kap.DecisionRelevantDocuments == 0 {
		decisionRelevantRatio = usableRatio
	}
	score += clampReport(decisionRelevantRatio/0.80, 0, 1) * 4
	score += clampReport(kap.AverageQuality, 0, 1) * 2
	rejectedRatio := safeReportRatio(float64(kap.RejectedCount), float64(kap.TotalDocuments))
	reviewRatio := safeReportRatio(float64(kap.ReviewRequiredCount), float64(kap.TotalDocuments))
	lowQualityRatio := safeReportRatio(float64(kap.LowQualityCount), float64(kap.TotalDocuments))
	score -= clampReport(rejectedRatio/0.10, 0, 1) * 2
	score -= clampReport(reviewRatio/0.50, 0, 1) * 3
	score -= clampReport(lowQualityRatio/0.50, 0, 1) * 2
	score = clampReport(score, 0, 15)
	return reportConfidenceItem{
		Label:  "KAP PDF kanıt kapsamı",
		Score:  score,
		Max:    15,
		Status: statusFor(score, 15),
		Detail: fmt.Sprintf("%d PDF, %d analize uygun, karar-ilgili %d/%d kullanılabilir, review %.1f%%, low-quality %.1f%%, rejected %.1f%%.",
			kap.TotalDocuments,
			kap.AnalysisUsableCount,
			kap.DecisionRelevantUsableCount,
			kap.DecisionRelevantDocuments,
			reviewRatio*100,
			lowQualityRatio*100,
			rejectedRatio*100,
		),
	}
}

func cryptoDataConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	coverage := result.Professional.Coverage
	score := clampReport(coverage.Score/100*25, 0, 25)
	status := statusFor(score, 25)
	detail := "Kripto veri kapsamı: " + joinReportLabels(coverage.Available)
	if len(coverage.Missing) > 0 {
		detail += ". Geliştirilecek veri: " + strings.Join(missingDataLabels(result), ", ")
	}
	return reportConfidenceItem{Label: "Kripto veri kapsamı", Score: score, Max: 25, Status: status, Detail: detail}
}

func commodityDataConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	coverage := result.Professional.Coverage
	score := clampReport(coverage.Score/100*25, 0, 25)
	status := statusFor(score, 25)
	detail := "Altın/emtia veri kapsamı: " + joinReportLabels(coverage.Available)
	if len(coverage.Missing) > 0 {
		detail += ". Geliştirilecek veri: " + strings.Join(missingDataLabels(result), ", ")
	}
	return reportConfidenceItem{Label: "Altın/emtia veri kapsamı", Score: score, Max: 25, Status: status, Detail: detail}
}

func investorQAConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	qa := result.InvestorQA
	if !qa.Computed {
		return reportConfidenceItem{Label: "Yatırımcı soru-cevap", Score: 0, Max: 20, Status: "Zayıf", Detail: "Yatırımcı soru-cevap katmanı üretilmedi."}
	}
	score := clampReport(qa.Confidence/100*10+qa.Score/100*6+mathutilLikeQuestionCoverage(len(qa.Questions))*4, 0, 20)
	return reportConfidenceItem{
		Label:  "Yatırımcı soru-cevap",
		Score:  score,
		Max:    20,
		Status: statusFor(score, 20),
		Detail: fmt.Sprintf("%d soru cevaplandı; karar %s, güven %.0f/100.", len(qa.Questions), qa.Decision, qa.Confidence),
	}
}

func mathutilLikeQuestionCoverage(count int) float64 {
	if count >= 16 {
		return 1
	}
	return clampReport(float64(count)/16, 0, 1)
}

func valueInvestingConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	v := result.Professional.ValueInvesting
	if !v.Computed {
		return reportConfidenceItem{Label: "İçsel değer/güvenlik marjı", Score: 0, Max: 25, Status: "Zayıf", Detail: "İçsel değer güvenilir hesaplanamadı; güvenlik marjı kararı üretilemedi."}
	}
	requiredMargin := v.MarginOfSafety.RequiredPct
	if requiredMargin <= 0 {
		requiredMargin = 25
	}
	score := v.Confidence/100*15 + v.QualityScore/100*7
	if v.IntrinsicValue.Computed && v.MarginOfSafety.Computed {
		score += 3
	}
	score = clampReport(score, 0, 25)
	detail := fmt.Sprintf("Baz içsel değer %.2f, güvenlik marjı %.1f%%, karar: %s.", v.IntrinsicValue.Base, v.MarginOfSafety.BasePct, v.DecisionLabel)
	if isBankReport(result) && bankCoreMetricsMissingForReport(result) {
		score = math.Min(score, 15)
		detail += " Banka ana metrikleri eksik olduğu için güven sınırlı."
	}
	return reportConfidenceItem{
		Label:  "İçsel değer/güvenlik marjı",
		Score:  score,
		Max:    25,
		Status: statusFor(score, 25),
		Detail: detail,
	}
}

func technicalConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	total := 0
	pass := 0
	validationLimited := 0
	validationFailed := 0
	for _, tf := range result.Timeframes {
		total++
		if tf.LastClose <= 0 {
			continue
		}
		ok := true
		if tf.NearestSupport != nil && tf.NearestSupport.Price >= tf.LastClose {
			ok = false
		}
		if tf.NearestResistance != nil && tf.NearestResistance.Price <= tf.LastClose {
			ok = false
		}
		if activeSummaryTradePlan(tf.TradePlan) && tf.TradePlan.StopLoss >= tf.TradePlan.EntryMax {
			ok = false
		}
		validation := tf.Professional.Technical.Validation
		switch strings.ToLower(strings.TrimSpace(validation.Status)) {
		case "fail":
			ok = false
			validationFailed++
		case "limited":
			ok = false
			validationLimited++
		}
		if ok {
			pass++
		}
	}
	score := 0.0
	if total > 0 {
		score = 30 * float64(pass) / float64(total)
	}
	detail := fmt.Sprintf("%d/%d zaman dilimi destek, direnç, işlem planı ve teknik doğrulama yönünden tutarlı.", pass, total)
	if validationFailed > 0 || validationLimited > 0 {
		detail += fmt.Sprintf(" Teknik doğrulama sınırlı/geçmedi: sınırlı %d, geçmedi %d.", validationLimited, validationFailed)
	}
	return reportConfidenceItem{Label: "Fiyat/teknik tutarlılık", Score: score, Max: 30, Status: statusFor(score, 30), Detail: detail}
}

func financialGovernanceConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	pro := result.Professional
	score := clampReport(pro.DataQuality/100*10, 0, 10)
	if pro.DataGovernance.BacktestSafe {
		score += 5
	}
	if pro.DataGovernance.ProductionReady {
		score += 5
	}
	if pro.DataGovernance.FinanciallyConsistent {
		score += 5
	}
	if !pro.DataGovernance.ProductionReady {
		score = math.Min(score, 17)
	}
	detail := fmt.Sprintf("Ham veri kalitesi %.0f/100, son dönem %s, durum %s, üretim hazır: %s.", pro.DataQuality, emptyFallback(pro.DataGovernance.LatestPeriod, "yok"), reportLabel(pro.DataGovernance.AvailabilityStatus), localize.Bool(pro.DataGovernance.ProductionReady))
	if len(pro.DataGovernance.InvalidChronologyPeriods) > 0 {
		detail += " Tarih sırası geçersiz dönemler: " + strings.Join(pro.DataGovernance.InvalidChronologyPeriods, ", ") + "."
	}
	return reportConfidenceItem{Label: "Finansal veri yönetişimi", Score: clampReport(score, 0, 25), Max: 25, Status: statusFor(score, 25), Detail: detail}
}

func sectorPeerConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	pro := result.Professional
	score := 0.0
	detail := "Sektör sınıflandırması belirsiz."
	if pro.Company.Sector != "" && pro.Company.Sector != "BIST Genel" {
		score += 8
		detail = "Sektör sınıflandırması " + pro.Company.Sector + "."
	}
	if pro.Peers.PeerCount >= 8 {
		score += 12
	} else if pro.Peers.PeerCount >= 3 {
		score += 8
	} else if pro.Peers.PeerCount > 0 {
		score += 4
	}
	if pro.Company.Sector == "Gayrimenkul Yatırım Ortaklığı" && !strings.HasSuffix(result.Symbol, "GYO") {
		score = 0
		detail = "Sektör sınıflandırması sembolle çelişiyor; benzer şirket karşılaştırması güvenilir değil."
	}
	if isBankReport(result) && pro.SectorFinancials.Score > 0 && pro.SectorFinancials.Score < 80 {
		score = math.Min(score, 13)
		detail += fmt.Sprintf(" Banka sektör profili %.0f/100 olduğu için doğrulama kısmi.", pro.SectorFinancials.Score)
	}
	if isBankReport(result) && len(pro.Peers.Warnings) > 0 {
		score = math.Min(score, 13)
		detail += " Banka peer medyanları uç değer filtresiyle sınırlı güvene indirildi."
	}
	return reportConfidenceItem{Label: "Sektör/benzer şirket doğrulaması", Score: clampReport(score, 0, 20), Max: 20, Status: statusFor(score, 20), Detail: fmt.Sprintf("%s Benzer şirket sayısı: %d.", detail, pro.Peers.PeerCount)}
}

func valuationConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	pro := result.Professional
	score := 0.0
	if pro.Valuation.FairValue.Confidence > 0 {
		score += pro.Valuation.FairValue.Confidence * 6
	}
	if len(pro.Valuation.FairValue.Drivers) >= 2 {
		score += 4
	} else if len(pro.Valuation.FairValue.Drivers) == 1 {
		score += 2
	}
	if pro.Peers.ValuationSignal != "" && pro.Peers.ValuationSignal != "insufficient_data" {
		score += 3
	}
	if len(pro.Scenarios) == 3 {
		score += 2
	}
	detail := fmt.Sprintf("Makul değer güveni %.0f%%, sürücüler: %s.", pro.Valuation.FairValue.Confidence*100, strings.Join(pro.Valuation.FairValue.Drivers, ", "))
	if isBankReport(result) && bankCoreMetricsMissingForReport(result) {
		score = math.Min(score, 7)
		detail += " Banka ana metrikleri eksik olduğu için değerleme/senaryo güveni sınırlı."
	}
	return reportConfidenceItem{Label: "Değerleme/senaryo hesabı", Score: clampReport(score, 0, 15), Max: 15, Status: statusFor(score, 15), Detail: detail}
}

func presentationConfidence() reportConfidenceItem {
	return reportConfidenceItem{Label: "Rapor sunumu", Score: 10, Max: 10, Status: "Geçti", Detail: "Markdown işaretleri, ham JSON ve pipe tabloları ana PDF/HTML raporuna yazılmaz."}
}

func institutionalValidationConfidence(result analysis.SymbolAnalysis) reportConfidenceItem {
	validation := result.InstitutionalValidation
	score := clampReport(validation.Score/100*15, 0, 15)
	switch strings.ToLower(strings.TrimSpace(validation.Status)) {
	case "fail":
		score = 0
	case "limited":
		score = math.Min(score, 9)
	}
	return reportConfidenceItem{
		Label:  "Rapor güvenlik kapısı",
		Score:  score,
		Max:    15,
		Status: institutionalStatusTR(validation.Status),
		Detail: emptyFallback(validation.Summary, "Doğrulama raporu yok."),
	}
}

func statusFor(score, max float64) string {
	ratio := 0.0
	if max > 0 {
		ratio = score / max
	}
	switch {
	case ratio >= 0.85:
		return "Geçti"
	case ratio >= 0.65:
		return "Kısmi"
	default:
		return "Zayıf"
	}
}

func writeReportConfidenceGapHTML(b *strings.Builder, result analysis.SymbolAnalysis, confidence reportConfidence) {
	gaps := reportConfidenceGaps(result, confidence)
	if len(gaps) == 0 {
		return
	}
	rows := make([][]string, 0, len(gaps))
	for _, gap := range gaps {
		rows = append(rows, []string{
			gap.Area,
			gap.Current,
			gap.Target,
			gap.Impact,
			gap.Action,
			fmt.Sprintf("%.1f", gap.MissingPts),
		})
	}
	b.WriteString("<section class=\"section\"><h2>Rapor Güveni 100 İçin Eksikler</h2>")
	writeHTMLTable(b, "Tam Puan İçin Eksik Alanlar", []string{"Alan", "Mevcut", "100 İçin Gereken", "Etkisi", "Aksiyon", "Eksik puan"}, rows)
	b.WriteString("</section>\n")
}

func reportConfidenceGaps(result analysis.SymbolAnalysis, confidence reportConfidence) []reportConfidenceGap {
	gaps := []reportConfidenceGap{}
	maxTotal := 0.0
	for _, item := range confidence.Items {
		maxTotal += item.Max
	}
	for _, item := range confidence.Items {
		if item.Max <= 0 {
			continue
		}
		missing := item.Max - item.Score
		if missing <= 0.05 {
			continue
		}
		impact := item.Detail
		if maxTotal > 0 {
			impact = fmt.Sprintf("Rapor güvenine etkisi %.1f puan. %s", 100*missing/maxTotal, item.Detail)
		}
		gaps = append(gaps, reportConfidenceGap{
			Area:       item.Label,
			Current:    fmt.Sprintf("%.1f / %.0f | %s", item.Score, item.Max, item.Status),
			Target:     fmt.Sprintf("%.1f / %.0f", item.Max, item.Max),
			Impact:     impact,
			Action:     reportConfidenceGapAction(result, item.Label),
			MissingPts: missing,
		})
	}
	if isEquityResult(result) && !directBuySignalAllowed(result) && !isDecisionReadyStatus(result) {
		gaps = append(gaps, reportConfidenceGap{
			Area:       "Karar kullanım kapısı",
			Current:    "AL/SAT sinyali: hayır",
			Target:     "AL/SAT sinyali pass; production trading ayrı kapı olarak pass",
			Impact:     "Skor açığı değildir; üst kararın doğrudan AL etiketi almasını bloklar.",
			Action:     "Walk-forward ve teknik sinyal kapısını tamamla; en az 30 geçmiş işlem, 10 OOS işlem, pozitif maliyet sonrası getiri ve actionable=true teknik sinyal olmadan AL etiketi üretme.",
			MissingPts: 0,
		})
	}
	sort.SliceStable(gaps, func(i, j int) bool {
		return gaps[i].MissingPts > gaps[j].MissingPts
	})
	return gaps
}

func reportConfidenceGapAction(result analysis.SymbolAnalysis, label string) string {
	slug := strings.ToLower(label)
	switch {
	case strings.Contains(slug, "banka ana metrik"):
		return "KAP PDF/XBRL çıkarımından SYR, CET1, NPL, karşılık kapsamı, NIM, LCR, kredi/mevduat, mevduat maliyeti ve kredi/mevduat spread'i için source_document_id, sayfa, tablo/satır, orijinal/normalize değer ve kap_certified kanıtı üret."
	case strings.Contains(slug, "kap pdf"):
		return "KAP PDF/ek içerikleri belge bazında okunabilir hale getirilmeli; finansal tablo, yönetim açıklaması ve karar etkisi olan belgeler kaynak kanıtıyla rapora bağlanmalı."
	case strings.Contains(slug, "finansal"):
		return "Yapılandırılmış finansal tablolar tamamlanmalı; yayın tarihi, erişilebilirlik tarihi ve mutabakat zinciri doğrulanmalı."
	case strings.Contains(slug, "teknik"), strings.Contains(slug, "fiyat"):
		return "Grafik formasyon, hacim ve indikatör onaylarını kalite kapısına bağla; hedef: daily_technical_signal actionable=true ve net formasyon/teyit seçimi."
	case strings.Contains(slug, "sektör"), strings.Contains(slug, "benzer"):
		return "Sektör sınıflandırması ve benzer şirket evreni resmi kaynaklarla yenilenmeli; sınıflandırma güveni yüksek ve karşılaştırılabilir şirket sayısı yeterli olmalı."
	case strings.Contains(slug, "içsel"):
		if isBankReport(result) {
			return "Banka net kâr köprüsünü çıkar: net faiz geliri, ücret-komisyon, ticari kâr/zarar, karşılık giderleri, iştirak/tek seferlik gelirler ve sermaye artışı sınıflaması rapora bağlanmalı."
		}
		return "Net kâr, faaliyet kârı, finansman gideri, FCF ve tek seferlik kalemleri ayrıştır; güvenlik marjı varsayımlarını belge kanıtıyla bağla."
	case strings.Contains(slug, "değerleme"):
		return "Tek resmi değerleme setini içsel değer aralığı yap; peer/model çarpanını ayrı kontrol seti olarak tut ve SSS/senaryo çıktılarında çift hedef üretme."
	case strings.Contains(slug, "yatırımcı"):
		if isBankReport(result) {
			return "SYR, çekirdek sermaye, NPL, NIM, net faiz/ücret büyümesi, karşılık gideri/krediler, LCR, kredi/mevduat spread'i ve YP pozisyonu sorularını yapılandırılmış cevaplara bağla."
		}
		return "Yatırımcı QA cevaplarını kaynak kanıtı, model riski ve eksik finansal kırılımlarla tamamla."
	case strings.Contains(slug, "güvenlik"):
		return "Trading Edge Standardı için trades >= 30, OOS >= 10, beklenen getiri ve maliyet sonrası getiri pozitif, sinyal istatistiği yeterli olmalı."
	default:
		return "Eksik veri kaynağını tamamla ve raporu yeniden üret."
	}
}

func safeReportRatio(numerator, denominator float64) float64 {
	if denominator == 0 || math.IsNaN(numerator) || math.IsNaN(denominator) || math.IsInf(numerator, 0) || math.IsInf(denominator, 0) {
		return 0
	}
	return numerator / denominator
}

func scoreClass(score float64) string {
	switch {
	case score >= 75:
		return "good"
	case score >= 55:
		return "warn"
	default:
		return "bad"
	}
}

func decisionClass(score float64) string {
	if score >= 65 {
		return "good"
	}
	if score >= 55 {
		return "warn"
	}
	return "bad"
}

func primaryLastClose(result analysis.SymbolAnalysis) float64 {
	if tf, ok := result.Timeframes["1D"]; ok {
		return tf.LastClose
	}
	if result.NextSessionForecast.LastClose > 0 {
		return result.NextSessionForecast.LastClose
	}
	keys := sortedTimeframeKeys(result.Timeframes)
	if len(keys) == 0 {
		return 0
	}
	return result.Timeframes[keys[0]].LastClose
}

func formatMoney(value float64, currency string) string {
	currency = displayCurrency(currency)
	if value == 0 {
		return "0 " + currency
	}
	abs := math.Abs(value)
	unit := ""
	divisor := 1.0
	switch {
	case abs >= 1_000_000_000_000:
		unit = " trilyon"
		divisor = 1_000_000_000_000
	case abs >= 1_000_000_000:
		unit = " milyar"
		divisor = 1_000_000_000
	case abs >= 1_000_000:
		unit = " milyon"
		divisor = 1_000_000
	}
	return fmt.Sprintf("%.2f%s %s", value/divisor, unit, currency)
}

func formatMoneyExact(value float64, currency string) string {
	return fmt.Sprintf("%.2f %s", value, displayCurrency(currency))
}

func displayCurrency(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "TRY":
		return "TL"
	default:
		return strings.TrimSpace(currency)
	}
}

func normalizeVisibleCurrencyText(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "TRY") {
		return "TL"
	}
	replacer := strings.NewReplacer(
		" TRY", " TL",
		"(TRY", "(TL",
		"/TRY", "/TL",
		"|TRY", "|TL",
		"TRY)", "TL)",
		"TRY,", "TL,",
		"TRY.", "TL.",
		"TRY;", "TL;",
		"TRY:", "TL:",
	)
	return replacer.Replace(value)
}

func formatPct(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func clampReport(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func valuationSignalTR(signal string) string {
	switch strings.ToLower(strings.TrimSpace(signal)) {
	case "discount":
		return "ucuz/iskontolu"
	case "premium":
		return "pahalı/primli"
	case "neutral":
		return "nötr"
	case "not_applicable", "not applicable":
		return "uygulanmaz"
	default:
		return "veri yetersiz"
	}
}

type reportFonts struct {
	title font.Face
	h1    font.Face
	h2    font.Face
	body  font.Face
	small font.Face
}

func loadReportFonts() (reportFonts, error) {
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return reportFonts{}, err
	}
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return reportFonts{}, err
	}
	face := func(f *opentype.Font, size float64) (font.Face, error) {
		return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 144, Hinting: font.HintingFull})
	}
	title, err := face(bold, 24)
	if err != nil {
		return reportFonts{}, err
	}
	h1, err := face(bold, 18)
	if err != nil {
		return reportFonts{}, err
	}
	h2, err := face(bold, 14)
	if err != nil {
		return reportFonts{}, err
	}
	body, err := face(regular, 12)
	if err != nil {
		return reportFonts{}, err
	}
	small, err := face(regular, 9.5)
	if err != nil {
		return reportFonts{}, err
	}
	return reportFonts{title: title, h1: h1, h2: h2, body: body, small: small}, nil
}

func renderReportPages(result analysis.SymbolAnalysis) ([]image.Image, error) {
	fonts, err := loadReportFonts()
	if err != nil {
		return nil, err
	}
	confidence := reportConfidenceFor(result)
	pages := []image.Image{renderRetailDecisionPDFPage(result, confidence, fonts), renderReportPageOne(result, confidence, fonts)}
	if isEquityResult(result) && result.Professional.InvestmentResearch.Computed {
		pages = append(pages, renderInvestmentResearchPage(result, fonts))
	}
	pages = append(pages, renderReportPageTwo(result, fonts))
	if _, ok := symbolNextSessionForecast(result); ok {
		pages = append(pages, renderReportTablePages(result, fonts,
			"Seans senaryosu",
			"Bir Sonraki Seans Dağılım / Yön / Risk Kapısı",
			[]string{"Alan", "Hesaplanan değer"},
			[]int{70, 430},
			nextSessionForecastPDFRows(result),
		)...)
	}
	if result.BISTBulletin.Computed || len(result.BISTBulletin.Warnings) > 0 {
		pages = append(pages, renderReportTablePages(result, fonts,
			"BIST resmi bülten doğrulaması",
			"THB Resmi Açılış / Kapanış / Mikroyapı Kaynağı",
			[]string{"Alan", "Bülten sonucu"},
			[]int{70, 430},
			bistBulletinContextPDFRows(result),
		)...)
	}
	if isEquityResult(result) {
		pages = append(pages, renderReportTablePages(result, fonts,
			"VAP / MKK piyasa yapısı",
			"Fiili Dolaşım ve BIST Endeks Portföyü",
			[]string{"Alan", "VAP sonucu"},
			[]int{70, 430},
			vapContextPDFRows(result),
		)...)
	}
	if result.InvestorQA.Computed {
		pages = append(pages, renderInvestorQAPage(result, fonts))
		if result.InvestorQA.InstitutionalViews.Computed {
			pages = append(pages, renderInstitutionalPersonaViewsPage(result, fonts))
		}
		pages = append(pages, renderReportTablePages(result, fonts,
			"Yatırımcı soru-cevap",
			"En Çok Sorulan Sorular",
			[]string{"Soru", "Cevap", "Durum", "Güven"},
			[]int{70, 360, 940, 1060},
			investorQAQuestionRows(result),
		)...)
	}
	if isEquityResult(result) {
		pages = append(pages, renderValueInvestingPage(result, fonts))
	}
	pages = append(pages,
		renderReportPageThree(result, confidence, fonts),
		renderReportPageFour(result, confidence, fonts),
	)
	if isEquityResult(result) {
		pages = appendSourceAppendixPDFPages(pages, result, fonts)
	}
	pages = append(pages, renderTechnicalAppendixPages(result, fonts)...)
	return pages, nil
}

func nextSessionForecastPDFRows(result analysis.SymbolAnalysis) [][]string {
	forecast, ok := symbolNextSessionForecast(result)
	if !ok {
		return nil
	}
	forecast = analysis.ApplyNextSessionForecastPublishState(forecast)
	rows := [][]string{
		{"Tahmin edilen seans", emptyFallback(forecast.ForecastFor, "Belirlenemedi")},
		{"Son kapanış", reportPrice(forecast.LastClose, result.Currency)},
		{"Senaryo kullanım durumu", nextSessionScenarioUsageText(forecast)},
		{"Karar sonucu", nextSessionDecisionOutcomeText(forecast)},
		{"Açılış fiyat senaryosu", nextSessionScenarioForecastText(forecast, "open", result.Currency)},
		{"Kapanış fiyat senaryosu", nextSessionScenarioForecastText(forecast, "close", result.Currency)},
		{"Beklenen bant", nextSessionExpectedBandText(forecast, result.Currency)},
		{"Açılış dağılımı P10 / P50 / P90", nextSessionForecastDistributionTextForForecast(forecast, true, result.Currency)},
		{"Kapanış dağılımı P10 / P50 / P90", nextSessionForecastDistributionTextForForecast(forecast, false, result.Currency)},
		{"Yön olasılığı", nextSessionForecastProbabilityText(forecast)},
		{"Invalidasyon seviyesi", nextSessionForecastInvalidationText(forecast, result.Currency)},
		{"Yön / güç", nextSessionDirectionDisplayText(forecast)},
		{"Model güveni", fmt.Sprintf("%.0f/100 | %s | %d tarihsel örnek", forecast.Confidence, emptyFallback(forecast.ConfidenceLabel, "etiket yok"), forecast.HistoricalSamples)},
		{"Tahmin kalitesi", nextSessionForecastQualityText(forecast)},
		{"Model", forecast.Model},
	}
	if forecast.ActualAvailable && forecast.RawExpectedLow > 0 && forecast.RawExpectedHigh > 0 && (forecast.RawExpectedLow != forecast.ExpectedLow || forecast.RawExpectedHigh != forecast.ExpectedHigh) {
		rows = append(rows, []string{"Ham beklenen bant", fmt.Sprintf("%s - %s", reportPrice(forecast.RawExpectedLow, result.Currency), reportPrice(forecast.RawExpectedHigh, result.Currency))})
	}
	if forecast.TickSize > 0 {
		rows = append(rows, []string{"Fiyat adımı", fmt.Sprintf("%s | %s | %s", reportPrice(forecast.TickSize, result.Currency), emptyFallback(forecast.RoundingMethod, "yuvarlama yok"), emptyFallback(forecast.PriceStepRule, "kural yok"))})
	}
	if validation := nextSessionForecastValidationText(forecast); validation != "" {
		rows = append(rows, []string{"Doğrulama", validation})
	}
	rows = append(rows, []string{"Mikroyapı bağlamı", nextSessionForecastMicrostructureText(forecast)})
	if daily, ok := result.Timeframes["1D"]; ok {
		rows = append(rows, []string{"İşlem planı referansı", reportPlanTextForReport(result, daily)})
		if daily.TradePlan.ConfidenceScore > 0 {
			rows = append(rows, []string{"Plan güveni", fmt.Sprintf("%.0f/100 | %s", tradePlanConfidencePct(daily.TradePlan.ConfidenceScore), localize.Quality(daily.TradePlan.Quality))})
		}
	}
	for _, reason := range forecast.BiasReasons {
		rows = append(rows, []string{"Sinyal", reason})
	}
	return rows
}

func bistBulletinContextPDFRows(result analysis.SymbolAnalysis) [][]string {
	context := result.BISTBulletin
	rows := [][]string{
		{"Durum", yesNoReport(context.Computed)},
		{"Kaynak", emptyFallback(context.Source, "BIST THB resmi bülten")},
		{"Kapsam", fmt.Sprintf("%s - %s | %d kayıt", emptyFallback(context.CoverageStart, "başlangıç yok"), emptyFallback(context.CoverageEnd, "bitiş yok"), context.RecordCount)},
	}
	if context.LatestRecord.TradingDate != "" {
		rows = append(rows,
			[]string{"Son resmi seans", fmt.Sprintf("%s | Açılış %s | Kapanış %s | AOF %s",
				context.LatestRecord.TradingDate,
				reportPrice(context.LatestRecord.Open, result.Currency),
				reportPrice(context.LatestRecord.Close, result.Currency),
				reportPrice(context.LatestRecord.VWAP, result.Currency),
			)},
			[]string{"Son seans mikroyapı", fmt.Sprintf("bekleyen alış/satış %s / %s | spread %.2f bps",
				reportPrice(context.LatestRecord.RemainingBid, result.Currency),
				reportPrice(context.LatestRecord.RemainingAsk, result.Currency),
				context.LatestObservedSpreadBps,
			)},
			[]string{"Seans hacimleri", fmt.Sprintf("açılış %.0f adet | kapanış %.0f adet | toplam %.0f adet",
				context.LatestOpeningSessionVolume,
				context.LatestClosingSessionVolume,
				context.LatestRecord.Volume,
			)},
		)
	}
	if context.OfficialCloseConfirmed {
		rows = append(rows, []string{"Resmi kapanış mutabakatı", fmt.Sprintf("mutabık | fark %+.2f%%", context.OfficialCloseDeltaPct)})
	} else if context.OfficialCloseDeltaPct != 0 {
		rows = append(rows, []string{"Resmi kapanış mutabakatı", fmt.Sprintf("fark %+.2f%%", context.OfficialCloseDeltaPct)})
	}
	if context.ForecastActualAvailable {
		rows = append(rows, []string{"Tahmin seansı gerçekleşeni", fmt.Sprintf("%s | açılış %s | kapanış %s",
			context.ForecastActualRecord.TradingDate,
			reportPrice(context.ForecastActualRecord.Open, result.Currency),
			reportPrice(context.ForecastActualRecord.Close, result.Currency),
		)})
	} else {
		rows = append(rows, []string{"Tahmin seansı gerçekleşeni", "Resmi sonuç geldikten sonra model senaryosuyla karşılaştırılacak"})
	}
	for _, warning := range context.Warnings {
		rows = append(rows, []string{"Uyarı", warning})
	}
	return rows
}

func vapContextPDFRows(result analysis.SymbolAnalysis) [][]string {
	freeFloat := result.Professional.VAPFreeFloat
	portfolio := result.Professional.VAPIndexPortfolio
	return [][]string{
		{"Fiili dolaşım durumu", yesNoReport(freeFloat.Computed)},
		{"Fiili dolaşım son tarih", emptyFallback(freeFloat.LatestDate, "Veri yok")},
		{"Fiili dolaşım oranı", fmt.Sprintf("%.2f%%", freeFloat.FreeFloatRatioPct)},
		{"20 gözlem oran değişimi", fmt.Sprintf("%+.2f puan", freeFloat.RatioChange20DPP)},
		{"Likidite riski / arz sinyali", emptyFallback(freeFloat.LiquidityRisk, "Bilinmiyor") + " / " + emptyFallback(freeFloat.SupplySignal, "Bilinmiyor")},
		{"Fiili dolaşım özeti", freeFloat.Summary},
		{"Endeks portföyü durumu", yesNoReport(portfolio.Computed)},
		{"Seçilen endeks / dönem", emptyFallback(portfolio.SelectedIndex, "Veri yok") + " / " + emptyFallback(portfolio.LatestMonth, "Veri yok")},
		{"Portföy değeri", fmt.Sprintf("%.2f milyon TL", portfolio.PortfolioValueMTL)},
		{"Aylık değişim / göreli fark", fmt.Sprintf("%+.2f%% / %+.2f puan", portfolio.Change1MPct, portfolio.RelativeMomentum)},
		{"Endeks portföyü sinyali", emptyFallback(portfolio.Signal, "Bilinmiyor")},
		{"Endeks portföyü özeti", portfolio.Summary},
	}
}

func appendSourceAppendixPDFPages(pages []image.Image, result analysis.SymbolAnalysis, fonts reportFonts) []image.Image {
	if result.Professional.RawKAPData != nil && result.Professional.RawKAPData.Computed {
		pages = append(pages, renderReportTablePages(result, fonts,
			"Kaynaklar ve PDF ekleri",
			"Ana Finansal Tablo Kalemleri",
			[]string{"Kalem", "Son dönem", "Son değer", "Değişim", "Kanıt"},
			[]int{70, 285, 420, 585, 705},
			kapPDFFinancialMetricPDFRows(result.Professional.RawKAPData, result.Currency),
		)...)
	}
	if len(result.Professional.ValueInvesting.DocumentEvidence.KeyDocuments) > 0 {
		pages = append(pages, renderDocumentEvidencePage(result, fonts))
	}
	if result.Professional.KAPPDFIngest.Computed {
		pages = append(pages, renderReportTablePages(result, fonts,
			"Kaynaklar ve PDF ekleri",
			"Tüm PDF Listesi",
			[]string{"Belge", "Tür", "Kalite", "Metin", "Uyarı"},
			[]int{70, 430, 610, 720, 840},
			kapPDFAllDocumentRows(result.Professional.KAPPDFIngest),
		)...)
	}
	if result.Professional.KAPAssetInventory.Computed {
		pages = append(pages, renderReportTablePages(result, fonts,
			"Kaynaklar ve PDF ekleri",
			"KAP Varlık Envanteri",
			[]string{"Varlık", "Tür", "Lokasyon", "Alan / Değer", "Kira", "Güven"},
			[]int{70, 285, 430, 720, 935, 1060},
			kapAssetInventoryPDFRows(result.Professional.KAPAssetInventory),
		)...)
	}
	if result.Professional.RawKAPData != nil && result.Professional.RawKAPData.Computed {
		pages = append(pages, renderReportTablePages(result, fonts,
			"Kaynaklar ve PDF ekleri",
			"KAP PDF Ham Veri Kapsamı",
			[]string{"Veri alanı", "Adet", "Rol", "Durum", "Kaynak", "Alanlar"},
			[]int{70, 245, 315, 470, 610, 970},
			kapRawReportableCatalogRows(result.Professional.RawKAPData),
		)...)
	}
	return pages
}

func newReportPage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1240, 1754))
	fillReportRect(img, img.Bounds(), reportBg())
	return img
}

func renderOneLookSummaryImage(result analysis.SymbolAnalysis, confidence reportConfidence, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Tek bakış yatırımcı özeti")
	y := 190
	expectation, expectationClass, expectationText := oneLookExpectation(result)
	drawReportCards(img, fonts, []reportCard{
		{"Beklenti", expectation, expectationClass},
		{"Sade karar", retailDecisionLabel(result), executiveDecisionClass(result)},
		{"Son fiyat", reportPrice(primaryLastClose(result), result.Currency), "info"},
		{"Karar güveni", fmt.Sprintf("%.0f/100", retailConfidence(result)), scoreClass(retailConfidence(result))},
	}, y)
	y += 132
	y = drawReportSection(img, fonts, 70, y, 1100, "Sonuç", []string{
		expectationText,
		oneLookPlainDecision(result),
	}) + 20
	y = drawOneLookActionMatrix(img, fonts, result, 70, y) + 20
	leftBottom := drawReportBulletsSection(img, fonts, 70, y, 535, "Destek / Direnç", oneLookLevelLines(result))
	rightBottom := drawReportBulletsSection(img, fonts, 635, y, 535, "Yön Beklentisi", oneLookDirectionLines(result))
	y = maxReportInt(leftBottom, rightBottom) + 20
	leftBottom = drawReportBulletsSection(img, fonts, 70, y, 535, "Neden?", limitStrings(retailReasonLines(result), 4))
	rightBottom = drawReportBulletsSection(img, fonts, 635, y, 535, "Önce Tamamlanacak Veri", oneLookDataLines(result))
	_ = maxReportInt(leftBottom, rightBottom)
	warning := "Yatırım tavsiyesi değildir"
	warningWidth := font.MeasureString(fonts.small, warning).Ceil()
	drawReportText(img, fonts.small, 1170-warningWidth, 1688, reportClassColor("bad"), warning)
	return img
}

func renderRetailDecisionPDFPage(result analysis.SymbolAnalysis, confidence reportConfidence, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Sade karar")
	y := 205
	drawReportCards(img, fonts, []reportCard{
		{"Sade karar", retailDecisionLabel(result), executiveDecisionClass(result)},
		{"Son fiyat", reportPrice(primaryLastClose(result), result.Currency), "info"},
		{"Genel skor", fmt.Sprintf("%.1f/100", result.OverallScore), scoreClass(result.OverallScore)},
		{"Karar güveni", fmt.Sprintf("%.0f/100", retailConfidence(result)), scoreClass(retailConfidence(result))},
	}, y)
	y += 170
	answer := retailDecisionSentence(result)
	if result.InvestorQA.Computed && result.InvestorQA.OneLineAnswer != "" {
		answer = retailText(result.InvestorQA.OneLineAnswer)
	}
	y = drawReportSection(img, fonts, 70, y, 1100, "Kısa Cevap", []string{
		answer,
		fmt.Sprintf("Bu skor fiyat garantisi değildir; kullanılan verinin karar için ne kadar yeterli olduğunu gösterir. Son fiyat: %s.", retailPrice(primaryLastClose(result), result.Currency)),
	}) + 24
	leftBottom := drawReportBulletsSection(img, fonts, 70, y, 535, "Ne Yapmalı?", pdfRetailActionLines(result))
	rightBottom := drawReportBulletsSection(img, fonts, 635, y, 535, "Önemli Seviyeler", limitStrings(retailLevelLines(result), 5))
	y = maxReportInt(leftBottom, rightBottom) + 28
	leftBottom = drawReportBulletsSection(img, fonts, 70, y, 535, "Neden?", limitStrings(retailReasonLines(result), 4))
	rightBottom = drawReportBulletsSection(img, fonts, 635, y, 535, "Karar Ne Zaman Değişir?", pdfRetailDecisionChangeLines(result))
	y = maxReportInt(leftBottom, rightBottom) + 30
	if line := pdfDataImprovementLine(result); line != "" && y < 1500 {
		drawReportSection(img, fonts, 70, y, 1100, "Veri Nasıl İyileşir?", []string{line})
	}
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func pdfDataImprovementLine(result analysis.SymbolAnalysis) string {
	missing := missingDataLabels(result)
	if len(missing) == 0 {
		return ""
	}
	if isCommodityResult(result) {
		return "Veri geliştirilebilir: " + strings.Join(missing, ", ") + ". Bu kaynaklar bağlanırsa altın analizi güçlenir."
	}
	if isCryptoResult(result) {
		return "Veri geliştirilebilir: " + strings.Join(missing, ", ") + ". Bu kaynaklar bağlanırsa rapor güveni artar."
	}
	return dataImprovementLine(result)
}

func pdfRetailActionLines(result analysis.SymbolAnalysis) []string {
	return compactPDFBullets(retailActionLines(result), 3, 150)
}

func pdfRetailDecisionChangeLines(result analysis.SymbolAnalysis) []string {
	return compactPDFBullets(retailDecisionChangeLines(result), 3, 150)
}

func oneLookExpectation(result analysis.SymbolAnalysis) (string, string, string) {
	if result.DecisionSupport != nil {
		switch result.DecisionSupport.Retail.Signal {
		case "AL":
			return "POZİTİF / AL", "good", clarifyDecisionConfidenceText(result.DecisionSupport.Retail.OneLineAnswer)
		case "SAT":
			return "NEGATİF / SAT", "bad", clarifyDecisionConfidenceText(result.DecisionSupport.Retail.OneLineAnswer)
		case "BEKLE":
			return "NÖTR / BEKLE", "warn", clarifyDecisionConfidenceText(result.DecisionSupport.Retail.OneLineAnswer)
		}
	}
	daily, ok := result.Timeframes["1D"]
	if !ok {
		return "NÖTR", "warn", "Günlük teknik veri olmadığı için yön beklentisi sınırlı; önce fiyat verisi tamamlanmalıdır."
	}
	switch {
	case daily.TrendBias == "bullish" && result.OverallScore >= 60:
		return "POZİTİF", "good", "Kısa vadeli fiyat yapısı yukarı tarafa eğilimli; yine de teyit ve risk seviyesi birlikte izlenmelidir."
	case daily.TrendBias == "bearish" || result.OverallScore < 45:
		return "NEGATİF", "bad", "Kısa vadeli yapı zayıf; destek kırılımı veya momentum kaybı riski öne çıkıyor."
	default:
		return "NÖTR", "warn", "Yön kararsız; yukarı veya aşağı senaryo için belirgin kapanış teyidi beklenmelidir."
	}
}

func oneLookPlainDecision(result analysis.SymbolAnalysis) string {
	if result.DecisionSupport != nil && result.DecisionSupport.Retail.OneLineAnswer != "" {
		return clarifyDecisionConfidenceText(result.DecisionSupport.Retail.OneLineAnswer)
	}
	label := retailDecisionLabel(result)
	if label == "" {
		label = "TAKİP"
	}
	return fmt.Sprintf("Sade karar: %s. Bu görsel, kararın nedenini tek bakışta okumak içindir; kesin getiri vaadi değildir.", label)
}

func clarifyDecisionConfidenceText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	replacer := strings.NewReplacer(
		"; güven ", "; sinyal güveni ",
		", güven ", ", sinyal güveni ",
	)
	return replacer.Replace(text)
}

func clarifyInstitutionalDecisionConfidenceText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	return strings.NewReplacer(
		", güven ", ", kurumsal karar güveni ",
		"; güven ", "; kurumsal karar güveni ",
	).Replace(text)
}

func drawOneLookActionMatrix(img *image.RGBA, fonts reportFonts, result analysis.SymbolAnalysis, x, y int) int {
	width := 1100
	height := 132
	rect := image.Rect(x, y, x+width, y+height)
	drawReportRoundedPanel(img, rect, 8, reportPanel(), reportLine())
	fillReportRoundedRect(img, image.Rect(x+18, y+20, x+23, y+42), 2, reportAccent())
	drawReportText(img, fonts.h1, x+34, y+37, reportAccentDark(), "AL / TUT / SAT")
	drawReportText(img, fonts.small, x+34, y+60, reportMuted(), "Özet karar satırı; kapı geçmeden teknik seviye emir planı değildir.")

	cards := oneLookActionCards(result)
	cardY := y + 76
	cardW := 332
	gap := 22
	for i, card := range cards {
		cardX := x + 22 + i*(cardW+gap)
		cardRect := image.Rect(cardX, cardY, cardX+cardW, cardY+40)
		drawReportRoundedPanel(img, cardRect, 6, reportAccentPale(), reportLineSoft())
		fillReportRoundedRect(img, image.Rect(cardX+1, cardY+1, cardX+6, cardY+39), 3, reportClassColor(card.Class))
		drawReportText(img, fonts.small, cardX+16, cardY+24, reportMuted(), strings.ToUpper(card.Label))
		valueFace := fonts.h2
		if font.MeasureString(valueFace, card.Value).Ceil() > 175 {
			valueFace = fonts.body
		}
		drawReportText(img, valueFace, cardX+138, cardY+26, reportClassColor(card.Class), card.Value)
	}
	return rect.Max.Y
}

func oneLookActionCards(result analysis.SymbolAnalysis) []reportCard {
	if result.DecisionSupport != nil && result.DecisionSupport.Retail.Signal != "" {
		decision := result.DecisionSupport.Retail
		buyClass := "warn"
		if decision.Signal == "AL" && decision.Actionable {
			buyClass = "good"
		} else if decision.NewPositionAction == "ALMA" {
			buyClass = "bad"
		}
		holdClass := "info"
		if decision.Signal == "BEKLE" {
			holdClass = "warn"
		}
		sellClass := "info"
		if decision.Signal == "SAT" {
			sellClass = "bad"
		}
		return []reportCard{
			{Label: "YENİ POZİSYON", Value: retailActionTRForReport(decision.NewPositionAction), Class: buyClass},
			{Label: "ELDE VARSA", Value: retailActionTRForReport(decision.ExistingPositionAction), Class: holdClass},
			{Label: "ANA SİNYAL", Value: decision.Signal, Class: sellClass},
		}
	}
	cards := []reportCard{
		{Label: "AL", Value: "YOK", Class: "bad"},
		{Label: "TUT / İZLE", Value: "RADAR", Class: "warn"},
		{Label: "SAT / RİSK", Value: "ALARM YOK", Class: "info"},
	}
	if result.InvestorQA.Computed && len(result.InvestorQA.ActionMatrix) > 0 {
		for _, item := range result.InvestorQA.ActionMatrix {
			action := strings.ToLower(strings.TrimSpace(item.Action))
			switch {
			case strings.Contains(action, "al") || strings.Contains(action, "buy"):
				cards[0] = oneLookActionCard("AL", item, reportTradePlanBlocked(result))
			case strings.Contains(action, "tut") || strings.Contains(action, "izle") || strings.Contains(action, "hold"):
				cards[1] = oneLookActionCard("TUT / İZLE", item, false)
			case strings.Contains(action, "sat") || strings.Contains(action, "risk") || strings.Contains(action, "sell"):
				cards[2] = oneLookActionCard("SAT / RİSK", item, false)
			}
		}
		return cards
	}

	if reportTradePlanBlocked(result) {
		cards[0] = reportCard{Label: "AL", Value: "Kapalı", Class: "bad"}
		cards[1] = reportCard{Label: "TUT / İZLE", Value: "RADAR", Class: "warn"}
		cards[2] = reportCard{Label: "SAT / RİSK", Value: "DESTEK KIRILIRSA", Class: "warn"}
		return cards
	}
	if tf, ok := result.Timeframes["1D"]; ok && activeSummaryTradePlan(tf.TradePlan) {
		cards[0] = reportCard{Label: "AL", Value: "TEYİTLİ ADAY", Class: "good"}
		cards[1] = reportCard{Label: "TUT / İZLE", Value: "TAŞI / İZLE", Class: "info"}
		cards[2] = reportCard{Label: "SAT / RİSK", Value: "STOP İZLE", Class: "warn"}
	}
	return cards
}

func oneLookActionCard(label string, item investorqa.ActionSignal, forcedClosed bool) reportCard {
	if forcedClosed {
		return reportCard{Label: label, Value: "Kapalı", Class: "bad"}
	}
	action := strings.ToLower(strings.TrimSpace(item.Action))
	status := strings.ToLower(strings.TrimSpace(item.Status))
	switch {
	case strings.Contains(action, "al") || strings.Contains(action, "buy"):
		if item.CurrentSignal && status == "pass" {
			return reportCard{Label: label, Value: "AKTİF", Class: "good"}
		}
		if item.CurrentSignal {
			return reportCard{Label: label, Value: "ŞARTLI", Class: "warn"}
		}
		return reportCard{Label: label, Value: "YOK", Class: "bad"}
	case strings.Contains(action, "sat") || strings.Contains(action, "risk") || strings.Contains(action, "sell"):
		if item.CurrentSignal {
			return reportCard{Label: label, Value: "AKTİF", Class: "bad"}
		}
		if item.Trigger != "" {
			return reportCard{Label: label, Value: "ALARM ŞARTI", Class: "warn"}
		}
		return reportCard{Label: label, Value: "ALARM YOK", Class: "info"}
	case strings.Contains(action, "tut") || strings.Contains(action, "izle") || strings.Contains(action, "hold"):
		if item.CurrentSignal {
			return reportCard{Label: label, Value: "AKTİF", Class: "warn"}
		}
		return reportCard{Label: label, Value: "İZLE", Class: "info"}
	default:
		if item.CurrentSignal {
			return reportCard{Label: label, Value: "AKTİF", Class: "warn"}
		}
		return reportCard{Label: label, Value: "YOK", Class: "info"}
	}
}

func oneLookLevelLines(result analysis.SymbolAnalysis) []string {
	tf, ok := result.Timeframes["1D"]
	if !ok {
		return []string{"Günlük destek/direnç seviyesi üretilemedi."}
	}
	lines := []string{
		"Son fiyat: " + retailPrice(primaryLastClose(result), result.Currency) + ".",
	}
	if tf.NearestSupport != nil && tf.NearestSupport.Price > 0 {
		lines = append(lines, "Yakın destek: "+retailPrice(tf.NearestSupport.Price, result.Currency)+"; altında risk artar.")
	}
	if tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
		lines = append(lines, "Yakın direnç: "+retailPrice(tf.NearestResistance.Price, result.Currency)+"; üstünde kapanış güçlenme teyididir.")
	}
	if level, ok := nextSupport(tf); ok {
		lines = append(lines, "Sonraki destek: "+retailPrice(level.Price, result.Currency)+".")
	}
	if level, ok := nextResistance(tf); ok {
		lines = append(lines, "Sonraki direnç: "+retailPrice(level.Price, result.Currency)+".")
	}
	if activeSummaryTradePlan(tf.TradePlan) && !reportTradePlanBlocked(result) {
		lines = append(lines, fmt.Sprintf("Plan: giriş %s-%s, stop %s.", retailPrice(tf.TradePlan.EntryMin, result.Currency), retailPrice(tf.TradePlan.EntryMax, result.Currency), retailPrice(tf.TradePlan.StopLoss, result.Currency)))
	} else if tf.TradePlan.StopLoss > 0 {
		lines = append(lines, "Stop/iptal seviyesi: "+retailPrice(tf.TradePlan.StopLoss, result.Currency)+".")
	}
	return compactPDFBullets(lines, 5, 132)
}

func oneLookDirectionLines(result analysis.SymbolAnalysis) []string {
	lines := []string{}
	if tf, ok := result.Timeframes["1D"]; ok {
		appearance := timeframeTechnicalAppearanceForReport(result, tf)
		if technicalSignalBlockedForReport(tf) {
			lines = append(lines, fmt.Sprintf("Günlük teknik: %s, skor %.1f/100; teknik sinyal kapısı geçmedi.", appearance, tf.Score))
		} else {
			lines = append(lines, fmt.Sprintf("Günlük teknik: %s, skor %.1f/100.", appearance, tf.Score))
		}
	}
	if line := oneLookMacroForecastLine(result); line != "" {
		lines = append(lines, line)
	}
	for _, line := range limitStrings(oneLookUpsideLines(result), 2) {
		lines = append(lines, "Yukarı: "+line)
	}
	for _, line := range limitStrings(oneLookDownsideLines(result), 2) {
		lines = append(lines, "Aşağı: "+line)
	}
	if len(lines) == 0 {
		lines = append(lines, "Yön için kapanış, hacim ve momentum teyidi birlikte izlenmeli.")
	}
	return compactPDFBullets(lines, 5, 132)
}

func oneLookMacroForecastLine(result analysis.SymbolAnalysis) string {
	impact := result.Professional.TCMBEVDSContext.ForecastImpact
	if !impact.Computed {
		return ""
	}
	prefix := "Makro: "
	switch impact.Direction {
	case "positive":
		prefix += "yukarı beklentiyi destekler"
	case "negative":
		prefix += "yukarı beklentiyi baskılar"
	case "neutral":
		prefix += "belirgin yön üretmez"
	default:
		prefix += reportText(impact.Label)
	}
	if impact.DecisionUse == "blocking_headwind" {
		return fmt.Sprintf("%s; AL kapısı makro ters rüzgar nedeniyle kapalı.", prefix)
	}
	return fmt.Sprintf("%s; makro güveni %.0f/100, etki %+.1f puan.", prefix, impact.Confidence, impact.ScoreAdjustment)
}

func oneLookUpsideLines(result analysis.SymbolAnalysis) []string {
	lines := []string{}
	seen := map[string]bool{}
	add := func(line string) {
		keys := oneLookScenarioLineKeys("up", line)
		hasPriceKey := false
		for _, key := range keys {
			if strings.Contains(key, ":price:") {
				hasPriceKey = true
				if seen[key] {
					return
				}
			}
		}
		if !hasPriceKey {
			for _, key := range keys {
				if seen[key] {
					return
				}
			}
		}
		if len(keys) == 0 {
			key := strings.ToLower(strings.TrimSpace(line))
			if key == "" || seen[key] {
				return
			}
			keys = append(keys, key)
		}
		for _, key := range keys {
			if key == "" {
				continue
			}
			seen[key] = true
		}
		if strings.TrimSpace(line) == "" {
			return
		}
		lines = append(lines, line)
	}
	if tf, ok := result.Timeframes["1D"]; ok {
		if tf.NearestResistance != nil && tf.NearestResistance.Price > 0 {
			add(fmt.Sprintf("%s üstü kapanış yukarı senaryoyu güçlendirir; alıcı ilgisinin pozitife dönmesi için ana teyit budur.", retailPrice(tf.NearestResistance.Price, result.Currency)))
		}
		if activeSummaryTradePlan(tf.TradePlan) && !reportTradePlanBlocked(result) {
			add(fmt.Sprintf("İlk hedef bölgesi %s; ikinci hedef %s.", retailPrice(tf.TradePlan.TakeProfit1, result.Currency), retailPrice(tf.TradePlan.TakeProfit2, result.Currency)))
		}
	}
	if len(result.InvestorQA.BuyConditions) > 0 {
		for _, condition := range result.InvestorQA.BuyConditions {
			if oneLookValueEvidenceCondition(result, condition) && !reportValueInvestingMarginPass(result) {
				continue
			}
			add(retailText(condition))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "Yukarı yön için kapanış, hacim ve momentum birlikte güçlenmeli.")
	}
	return compactPDFBullets(lines, 4, 150)
}

func oneLookScenarioLineKey(direction string, line string) string {
	keys := oneLookScenarioLineKeys(direction, line)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func oneLookScenarioLineKeys(direction string, line string) []string {
	keys := []string{}
	number := firstPriceTokenKey(line)
	if number != "" {
		keys = append(keys, direction+":price:"+number)
	}
	normalized := normalizeFinancialText(line)
	if strings.Contains(normalized, "alici ilgisinin pozitife donmesi") ||
		strings.Contains(normalized, "alici ilgisinin guclenmesi") ||
		strings.Contains(normalized, "para akisinin pozitife donmesi") {
		keys = append(keys, direction+":buyer_interest_positive")
	}
	if strings.Contains(normalized, "macd histograminin pozitife donmesi") {
		keys = append(keys, direction+":macd_positive")
	}
	return keys
}

func oneLookValueEvidenceCondition(result analysis.SymbolAnalysis, line string) bool {
	if isCommodityResult(result) || isCryptoResult(result) {
		return false
	}
	normalized := normalizeFinancialText(line)
	return strings.Contains(normalized, "icsel deger") ||
		strings.Contains(normalized, "guvenlik marji") ||
		strings.Contains(normalized, "guvenilir deger")
}

func firstPriceTokenKey(line string) string {
	fields := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(line, ",", "."), "TL", " "))
	for _, field := range fields {
		field = strings.Trim(field, " ;:()[]{}|")
		if field == "" {
			continue
		}
		hasDigit := false
		for _, r := range field {
			if r >= '0' && r <= '9' {
				hasDigit = true
				continue
			}
			if r == '.' {
				continue
			}
			hasDigit = false
			break
		}
		if hasDigit {
			return field
		}
	}
	return ""
}

func oneLookDownsideLines(result analysis.SymbolAnalysis) []string {
	lines := []string{}
	if tf, ok := result.Timeframes["1D"]; ok {
		if tf.NearestSupport != nil && tf.NearestSupport.Price > 0 {
			lines = append(lines, fmt.Sprintf("%s altı kapanış aşağı riskini artırır.", retailPrice(tf.NearestSupport.Price, result.Currency)))
		}
		if tf.TradePlan.StopLoss > 0 {
			lines = append(lines, fmt.Sprintf("Risk disiplini için stop/iptal bölgesi %s.", retailPrice(tf.TradePlan.StopLoss, result.Currency)))
		}
	}
	if len(result.InvestorQA.ExitConditions) > 0 {
		lines = append(lines, retailText(strings.Join(limitStrings(result.InvestorQA.ExitConditions, 2), "; ")))
	}
	if len(lines) == 0 {
		lines = append(lines, "Aşağı risk için destek kırılımı ve zayıf hacim birlikte izlenmeli.")
	}
	return compactPDFBullets(lines, 4, 150)
}

func oneLookDataLines(result analysis.SymbolAnalysis) []string {
	lines := []string{}
	if result.PriceQuality != nil && !result.PriceQuality.ReadyForDecision {
		lines = append(lines, "Güncel karar fiyatı henüz kaynaklarla mutabık değil.")
	}
	if !isCommodityResult(result) && !isCryptoResult(result) && !reportValueInvestingMarginComputed(result) {
		lines = append(lines, "Pozitif/güvenilir içsel değer ve güvenlik marjı kanıtı tamamlanmalı.")
	}
	for _, item := range decisionSupportMissingInputs(result) {
		lines = append(lines, item)
		if len(lines) >= 3 {
			break
		}
	}
	if len(lines) == 0 {
		missing := missingDataLabels(result)
		for _, item := range limitStrings(missing, 3) {
			lines = append(lines, item+" tamamlanmalı.")
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "Karar için kullanılan ana veri setleri mevcut; yalnız kararın risk ve değişim şartlarını izle.")
	}
	return compactPDFBullets(lines, 4, 150)
}

func reportValueInvestingMarginComputed(result analysis.SymbolAnalysis) bool {
	v := result.Professional.ValueInvesting
	return v.Computed &&
		v.IntrinsicValue.Computed &&
		v.IntrinsicValue.Base > 0 &&
		v.MarginOfSafety.Computed &&
		v.MarginOfSafety.RequiredPct > 0
}

func reportValueInvestingMarginPass(result analysis.SymbolAnalysis) bool {
	v := result.Professional.ValueInvesting
	return reportValueInvestingMarginComputed(result) &&
		v.MarginOfSafety.BasePct >= v.MarginOfSafety.RequiredPct
}

func decisionSupportMissingInputs(result analysis.SymbolAnalysis) []string {
	if result.DecisionSupport == nil {
		return nil
	}
	out := []string{}
	for _, item := range result.DecisionSupport.MissingInputs {
		if item.Key == "" {
			continue
		}
		out = append(out, reportLabel(item.Key))
	}
	return out
}

func compactPDFBullets(lines []string, limit int, maxRunes int) []string {
	lines = limitStrings(lines, limit)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, compactPDFBullet(line, maxRunes))
	}
	return out
}

func compactPDFBullet(line string, maxRunes int) string {
	line = strings.TrimSpace(retailText(line))
	if maxRunes <= 0 || len([]rune(line)) <= maxRunes {
		return line
	}
	parts := strings.Split(line, ";")
	if len(parts) > 1 {
		short := strings.TrimSpace(strings.Join(parts[:minReportInt(2, len(parts))], "; "))
		if short != "" && !strings.HasSuffix(short, ".") {
			short += "."
		}
		if len([]rune(short)) <= maxRunes {
			return short
		}
	}
	sentences := strings.Split(line, ". ")
	if len(sentences) > 1 {
		short := strings.TrimSpace(strings.Join(sentences[:minReportInt(2, len(sentences))], ". "))
		if short != "" && !strings.HasSuffix(short, ".") {
			short += "."
		}
		if len([]rune(short)) <= maxRunes {
			return short
		}
	}
	runes := []rune(line)
	if len(runes) <= maxRunes {
		return line
	}
	return strings.TrimSpace(string(runes[:maxReportInt(1, maxRunes-1)])) + "."
}

func renderReportPageOne(result analysis.SymbolAnalysis, confidence reportConfidence, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Entegre karar özeti")
	y := 205
	drawReportCards(img, fonts, []reportCard{
		{"Karar", executiveDecision(result), executiveDecisionClass(result)},
		{"Son kapanış", reportPrice(primaryLastClose(result), result.Currency), "info"},
		{"Entegre skor", fmt.Sprintf("%.1f/100", result.OverallScore), scoreClass(result.OverallScore)},
		{"Doğrulama skoru", fmt.Sprintf("%.0f/100", confidence.Score), scoreClass(confidence.Score)},
	}, y)
	y += 170
	if isEquityResult(result) {
		if result.Professional.KAPPDFIngest.Computed {
			y = drawReportSection(img, fonts, 70, y, 1100, "KAP PDF Raporları", kapPDFIngestLines(result))
			y += 24
		}
		if result.Professional.KAPAssetInventory.Computed {
			y = drawReportSection(img, fonts, 70, y, 1100, "KAP Varlık Envanteri", kapAssetInventoryLines(result))
			y += 24
		}
		y = drawReportSection(img, fonts, 70, y, 1100, "Fiyat / İçsel Değer / Güvenlik Marjı", valueInvestingLines(result))
		y += 24
	}
	y = drawReportSection(img, fonts, 70, y, 1100, "Karar Özeti", []string{executiveDecisionText(result)})
	y += 24
	bulletY := y
	leftBottom := drawReportBulletsSection(img, fonts, 70, bulletY, 535, "Alım İçin Gerekli Teyitler", reportWaitReasons(result))
	rightBottom := drawReportBulletsSection(img, fonts, 635, bulletY, 535, "Başlıca Riskler", reportRiskReasons(result))
	y = maxReportInt(leftBottom, rightBottom) + 44
	drawFinancialTable(img, fonts, result, 70, y)
	return img
}

func renderReportPageTwo(result analysis.SymbolAnalysis, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Zaman dilimi ve işlem planı")
	y := 205
	drawReportText(img, fonts.h1, 70, y, reportInk(), "Zaman Dilimi Özeti")
	y += 36
	xs := []int{70, 185, 305, 430, 535, 735, 930}
	headers := []string{"Zaman", "Son bar", "Son kapanış", "Mum", "Skor", "Plan", "Sade yorum"}
	drawReportTableHeader(img, fonts, xs, y, headers)
	y += 42
	for _, key := range sortedTimeframeKeys(result.Timeframes) {
		tf := result.Timeframes[key]
		row := []string{
			localize.Timeframe(key),
			timeframeLastBarText(tf),
			reportPriceValue(tf.LastClose),
			timeframeCandleWindowText(tf),
			fmt.Sprintf("%.1f", tf.Score),
			timeframeGateTextForReport(result, tf),
			timeframePlainCommentForReport(result, tf),
		}
		y = drawReportTableRow(img, fonts, xs, y, row)
		if y > 1450 {
			break
		}
	}
	y += 34
	drawReportSection(img, fonts, 70, y, 1100, "Veriye Dayalı Okuma Notları", reportReadingNotes(result))
	return img
}

func renderInvestmentResearchPage(result analysis.SymbolAnalysis, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Yatırım hikayesi ve model denetimi")
	y := 205
	y = drawReportSection(img, fonts, 70, y, 1100, "Yatırım Kararı Denetimi", investmentResearchPDFLines(result)) + 34
	y = drawReportKeyValueTable(img, fonts, valuationTransparencyTitle(result), valuationTransparencyRows(result), 70, y, 1100) + 34
	decisionRows := decisionFrameworkRows(result.Professional.InvestmentResearch)
	if len(decisionRows) > 3 {
		decisionRows = decisionRows[:3]
	}
	y = drawReportKeyValueTable(img, fonts, "Karar Değişim Koşulları", decisionRows, 70, y, 1100) + 34
	technicalRows := technicalPriorityRows(result)
	if len(technicalRows) > 3 {
		technicalRows = technicalRows[:3]
	}
	drawReportKeyValueTable(img, fonts, "Teknik Sinyal Önceliği", technicalRows, 70, y, 1100)
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func renderInvestorQAPage(result analysis.SymbolAnalysis, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Yatırımcı komitesi özeti")
	y := 205
	y = drawReportSection(img, fonts, 70, y, 1100, "Tek Cümle Cevap", []string{retailText(result.InvestorQA.OneLineAnswer)}) + 40
	y = drawReportKeyValueTable(img, fonts, "Komite Kontrol Paneli", investorQASummaryRows(result), 70, y, 1100)
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func renderInstitutionalPersonaViewsPage(result analysis.SymbolAnalysis, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Rapor kalite ve aksiyon kapıları")
	y := 205
	y = drawReportSection(img, fonts, 70, y, 1100, "Veri Kapısı Yorumu", institutionalPersonaCommentaryRows(result)) + 36
	y = drawReportKeyValueTable(img, fonts, "Profil Durum Özeti", institutionalPersonaCompactRows(result), 70, y, 1100) + 42
	drawReportText(img, fonts.h1, 70, y, reportAccentDark(), "Profil Kararları")
	y += 34
	xs := []int{70, 205, 350, 480, 650, 880}
	drawReportTableHeader(img, fonts, xs, y, []string{"Profil", "Rapor", "Uygunluk", "Skorlar", "Karar", "Ana Engel"})
	y += 42
	for _, row := range institutionalPersonaTableRows(result) {
		y = drawReportTableRow(img, fonts, xs, y, row)
		if y > 1510 {
			break
		}
	}
	y += 28
	if y < 1490 {
		notes := []string{}
		for _, view := range institutionalPersonaList(result) {
			if len(view.RequiredActions) == 0 {
				continue
			}
			notes = append(notes, view.Name+": "+strings.Join(view.RequiredActions, "; "))
		}
		if len(notes) > 0 {
			drawReportSection(img, fonts, 70, y, 1100, "Gerekli Ek Çalışmalar", notes)
		}
	}
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func renderValueInvestingPage(result analysis.SymbolAnalysis, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Değer yatırım detayı")
	y := 205
	y = drawReportKeyValueTable(img, fonts, "Fiyat / İçsel Değer / Güvenlik Marjı", valueInvestingRows(result), 70, y, 1100) + 42
	v := result.Professional.ValueInvesting
	if len(v.Checks) > 0 {
		drawReportText(img, fonts.h1, 70, y, reportAccentDark(), "Değer Yatırım Kontrolleri")
		y += 34
		xs := []int{70, 320, 470, 590}
		drawReportTableHeader(img, fonts, xs, y, []string{"Kontrol", "Durum", "Skor", "Açıklama"})
		y += 42
		for _, check := range v.Checks {
			y = drawReportTableRow(img, fonts, xs, y, []string{
				valueCheckLabel(check.Name),
				institutionalStatusTR(check.Status),
				fmt.Sprintf("%.0f/100", check.Score),
				check.Message,
			})
			if y > 1540 {
				break
			}
		}
	}
	if y < 1420 && len(v.BuffettChecklist.Requirements) > 0 {
		y += 28
		drawReportText(img, fonts.h1, 70, y, reportAccentDark(), "Buffett Gereksinim Matrisi")
		y += 34
		xs := []int{70, 250, 560, 690}
		drawReportTableHeader(img, fonts, xs, y, []string{"Sütun", "Gereksinim", "Durum", "Kanıt / Eksik"})
		y += 42
		for _, row := range buffettRequirementRows(result, 8) {
			y = drawReportTableRow(img, fonts, xs, y, row)
			if y > 1540 {
				break
			}
		}
	}
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func renderDocumentEvidencePage(result analysis.SymbolAnalysis, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "KAP PDF içerik analizi")
	y := 205
	doc := result.Professional.ValueInvesting.DocumentEvidence
	y = drawReportSection(img, fonts, 70, y, 1100, "Belge Kapsamı", []string{
		emptyFallback(doc.Summary, "KAP PDF/ek içerik analizi üretilemedi."),
		documentLLMStatusText(doc.LLMAnalysis),
		"Bu sayfa PDF metni okunabiliyorsa PDF içeriğinden, okunamıyorsa KAP bildirim gövdesi ve metadata alanlarından amaç/içerik/etki çıkarır.",
	}) + 34
	drawReportText(img, fonts.h1, 70, y, reportAccentDark(), "Öne Çıkan Belgeler")
	y += 34
	xs := []int{70, 300, 470, 785}
	drawReportTableHeader(img, fonts, xs, y, []string{"Belge", "Tür", "Amaç / İçerik", "Rapor Etkisi"})
	y += 42
	for i, item := range doc.KeyDocuments {
		if i >= 10 || y > 1540 {
			break
		}
		content := item.Purpose
		if item.ContentSummary != "" {
			content += " " + item.ContentSummary
		}
		if item.LLMAnalysis != nil && item.LLMAnalysis.Computed && item.LLMAnalysis.Summary != "" {
			content += " LLM özeti: " + item.LLMAnalysis.Summary
		}
		if item.ExtractionSource != "" {
			content += " Kaynak: " + reportLabel(item.ExtractionSource)
		}
		if item.ExtractionNote != "" {
			content += " Not: " + item.ExtractionNote
		}
		y = drawReportTableRow(img, fonts, xs, y, []string{
			cleanDocumentName(item.FileName),
			item.CategoryLabel,
			emptyFallback(content, "-"),
			emptyFallback(item.ReportImpact, "-"),
		})
	}
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func renderReportPageThree(result analysis.SymbolAnalysis, confidence reportConfidence, fonts reportFonts) image.Image {
	img := newReportPage()
	subtitle := "Değerleme, terimler ve doğrulama"
	if isCryptoResult(result) {
		subtitle = "Kripto veri kapsamı, terimler ve doğrulama"
	} else if isCommodityResult(result) {
		subtitle = "Altın/emtia veri kapsamı, terimler ve doğrulama"
	}
	drawReportHeader(img, fonts, result, subtitle)
	y := 205
	drawScenarioTable(img, fonts, result, 70, y)
	y += 330
	y = drawReportSection(img, fonts, 70, y, 1100, "Dip/Kapitülasyon ve Haber Duygu Tonu", []string{
		behavioralPlainText(result),
		fmt.Sprintf("Söylem: %s, dip skoru: %.0f/100, tersine dönüş skoru: %.0f/100.", result.Behavioral.Sentiment.Label, result.Behavioral.Capitulation.Score, result.Behavioral.Contrarian.Score),
	}) + 26
	y = drawReportSection(img, fonts, 70, y, 1100, "Terimler Basitçe Ne Demek?", []string{
		"MACD negatif: Kısa vadeli fiyat ivmesi zayıf. Alım için hızlanma teyidi eksik.",
		"Chaikin para akışı negatif: Yükseliş denemesi yeterli para girişiyle desteklenmiyor olabilir.",
		rsiTermText(result),
		"Risk/getiri: Olası kazanç ile stopa kadar alınan riskin oranıdır.",
	}) + 26
	drawConfidenceTable(img, fonts, confidence, 70, y)
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func renderReportPageFour(result analysis.SymbolAnalysis, confidence reportConfidence, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Rapor güvenlik kapısı")
	y := 205
	validation := result.InstitutionalValidation
	y = drawReportSection(img, fonts, 70, y, 1100, "Rapor Güvenlik ve Doğrulama Kapısı", []string{
		fmt.Sprintf("Durum: %s | Skor: %.0f/100.", institutionalStatusTR(validation.Status), validation.Score),
		emptyFallback(validation.Summary, "Doğrulama sonucu yok."),
	}) + 26
	drawInstitutionalValidationTable(img, fonts, validation, 70, y)
	y += 520
	drawConfidenceTable(img, fonts, confidence, 70, y)
	drawReportWrappedText(img, fonts.small, 70, 1668, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
	return img
}

func renderTechnicalAppendixPages(result analysis.SymbolAnalysis, fonts reportFonts) []image.Image {
	pages := []image.Image{}
	for _, key := range sortedTimeframeKeys(result.Timeframes) {
		tf := result.Timeframes[key]
		pages = append(pages, renderTechnicalOverviewPage(result, tf, fonts))
		pages = append(pages, renderReportTablePages(result, fonts,
			"İndikatör ekleri - "+localize.Timeframe(key),
			"Hesaplanan İndikatör Sinyalleri",
			[]string{"İndikatör", "Kategori", "Sinyal", "Değer", "Güven", "Kanıt"},
			[]int{70, 300, 455, 610, 750, 860},
			indicatorResultRows(tf, 0),
		)...)
		pages = append(pages, renderReportTablePages(result, fonts,
			"Formasyon ekleri - "+localize.Timeframe(key),
			"Aktif Formasyon Sonuçları",
			[]string{"Adı", "Yönü", "Güven", "Hacim", "Teyit İnd.", "Karşı Sinyal", "İşlem Değeri"},
			[]int{70, 250, 330, 420, 540, 765, 995},
			activePatternRows(tf.Patterns, 0),
		)...)
		pages = append(pages, renderReportTablePages(result, fonts,
			"Aday formasyonlar - "+localize.Timeframe(key),
			"En Güçlü Aday / Elenen Formasyonlar",
			[]string{"Formasyon", "Kategori", "Yön", "Güven", "Neden", "Kanıt"},
			[]int{70, 285, 420, 535, 650, 885},
			candidatePatternRows(tf.PatternCandidates, 20),
		)...)
	}
	return pages
}

func renderTechnicalOverviewPage(result analysis.SymbolAnalysis, tf analysis.TimeframeAnalysis, fonts reportFonts) image.Image {
	img := newReportPage()
	drawReportHeader(img, fonts, result, "Teknik ek - "+localize.Timeframe(tf.Timeframe))
	y := 205
	y = drawReportSection(img, fonts, 70, y, 1100, "Teknik Ek Okuma Notu", []string{
		"Bu ek sayfalarda yalnızca hesaplanan indikatörler, aktif formasyonlar ve kurumsal filtreye takılan güçlü adaylar gösterilir. Dış veri gerektiren veya proxy olarak kalan satırlar karar sinyali gibi yazılmaz.",
	}) + 46
	y = drawReportKeyValueTable(img, fonts, "Kapsam ve Skor", technicalScopeRows(result, tf), 70, y, 1100) + 40
	y = drawReportKeyValueTable(img, fonts, "Teknik Skor Dağılımı", technicalScoreRows(tf), 70, y, 1100) + 40
	drawReportKeyValueTable(img, fonts, "Çekirdek İndikatör Değerleri", coreIndicatorRows(tf), 70, y, 1100)
	return img
}

func renderReportTablePages(result analysis.SymbolAnalysis, fonts reportFonts, subtitle, title string, headers []string, xs []int, rows [][]string) []image.Image {
	pages := []image.Image{}
	newPage := func() (*image.RGBA, int) {
		img := newReportPage()
		drawReportHeader(img, fonts, result, subtitle)
		y := 205
		drawReportText(img, fonts.h1, 70, y, reportAccentDark(), title)
		y += 34
		drawReportTableHeader(img, fonts, xs, y, headers)
		return img, y + 42
	}
	img, y := newPage()
	if len(rows) == 0 {
		drawReportTableRow(img, fonts, xs, y, []string{"Kayıt yok", "Bu bölüm için rapora alınacak doğrulanmış sonuç yok."})
		return []image.Image{img}
	}
	for _, row := range rows {
		if y > 1570 {
			pages = append(pages, img)
			img, y = newPage()
		}
		y = drawReportTableRow(img, fonts, xs, y, row)
	}
	pages = append(pages, img)
	return pages
}

func renderReportImagePages(result analysis.SymbolAnalysis, images []reportImage) []image.Image {
	if len(images) == 0 {
		return nil
	}
	fonts, err := loadReportFonts()
	if err != nil {
		return nil
	}
	pages := []image.Image{}
	for _, item := range images {
		if len(item.PNG) == 0 {
			continue
		}
		decoded, err := png.Decode(bytes.NewReader(item.PNG))
		if err != nil {
			continue
		}
		img := newReportPage()
		subtitle := item.Subtitle
		if subtitle == "" {
			subtitle = "Grafik"
		}
		drawReportHeader(img, fonts, result, subtitle)
		drawReportText(img, fonts.h1, 70, 205, reportAccentDark(), emptyFallback(item.Title, "Teknik Grafik"))
		if item.Timeframe != "" {
			drawReportText(img, fonts.body, 70, 237, reportMuted(), "Zaman dilimi: "+localize.Timeframe(item.Timeframe))
		}
		frame := image.Rect(70, 272, 1170, 1575)
		drawReportRoundedPanel(img, frame, 16, reportPanel(), reportLine())
		drawReportImageInside(img, decoded, frame.Inset(18))
		drawReportWrappedText(img, fonts.small, 70, 1642, 1100, reportMuted(), emptyFallback(result.Disclaimer, ohlcv.Disclaimer), 18)
		pages = append(pages, img)
	}
	return pages
}

func drawReportImageInside(dst *image.RGBA, src image.Image, frame image.Rectangle) {
	srcBounds := src.Bounds()
	if srcBounds.Empty() || frame.Empty() {
		return
	}
	scaleX := float64(frame.Dx()) / float64(srcBounds.Dx())
	scaleY := float64(frame.Dy()) / float64(srcBounds.Dy())
	scale := math.Min(scaleX, scaleY)
	if scale <= 0 {
		return
	}
	width := int(math.Round(float64(srcBounds.Dx()) * scale))
	height := int(math.Round(float64(srcBounds.Dy()) * scale))
	targetX := frame.Min.X + (frame.Dx()-width)/2
	targetY := frame.Min.Y + 24
	if height > frame.Dy() {
		targetY = frame.Min.Y
	}
	target := image.Rect(
		targetX,
		targetY,
		targetX+width,
		targetY+height,
	)
	fillReportRoundedRect(dst, target.Inset(-8), 10, color.White)
	xdraw.CatmullRom.Scale(dst, target, src, srcBounds, xdraw.Over, nil)
	strokeReportRect(dst, target, reportLineSoft())
}

type reportCard struct {
	Label string
	Value string
	Class string
}

func drawReportHeader(img *image.RGBA, fonts reportFonts, result analysis.SymbolAnalysis, subtitle string) {
	fillReportRect(img, image.Rect(0, 0, 1240, 148), reportPanel())
	fillReportRect(img, image.Rect(0, 0, 1240, 6), reportAccent())
	fillReportRect(img, image.Rect(0, 147, 1240, 148), reportLine())
	drawReportText(img, fonts.title, 70, 62, reportAccentDark(), result.Symbol+" Entegre Analiz Raporu")
	drawReportText(img, fonts.body, 72, 104, reportMuted(), emptyFallback(result.CompanyName, entityNameLabel(result)+" yok")+" | "+analysisDateText(result.AnalysisDate)+" | "+subtitle)
	pill := image.Rect(940, 45, 1170, 84)
	drawReportRoundedPanel(img, pill, 8, reportAccentSoft(), reportLine())
	drawReportText(img, fonts.small, pill.Min.X+22, pill.Min.Y+25, reportAccentDark(), "YAPAY ZEKA ANALİZİ")
}

func drawReportCards(img *image.RGBA, fonts reportFonts, cards []reportCard, y int) {
	x := 70
	for _, card := range cards {
		rect := image.Rect(x, y, x+260, y+96)
		drawReportRoundedPanel(img, rect, 8, reportPanel(), reportLine())
		fillReportRoundedRect(img, image.Rect(x+1, y+1, x+7, y+95), 4, reportClassColor(card.Class))
		drawReportText(img, fonts.small, x+18, y+30, reportMuted(), strings.ToUpper(card.Label))
		valueFace := fonts.h2
		if font.MeasureString(valueFace, card.Value).Ceil() > 220 {
			valueFace = fonts.body
		}
		if font.MeasureString(valueFace, card.Value).Ceil() > 220 {
			drawReportWrappedText(img, valueFace, x+16, y+62, 224, reportClassColor(card.Class), card.Value, 22)
		} else {
			drawReportText(img, valueFace, x+16, y+65, reportClassColor(card.Class), card.Value)
		}
		x += 280
	}
}

func drawReportSection(img *image.RGBA, fonts reportFonts, x, y, width int, title string, paragraphs []string) int {
	paragraphs = reportTexts(paragraphs)
	contentHeight := 0
	for _, paragraph := range paragraphs {
		contentHeight += measureReportWrappedHeight(fonts.body, width-40, paragraph, 22) + 8
	}
	height := 64 + contentHeight + 16
	rect := image.Rect(x, y, x+width, y+height)
	drawReportRoundedPanel(img, rect, 8, reportPanel(), reportLine())
	fillReportRoundedRect(img, image.Rect(x+18, y+19, x+23, y+41), 2, reportAccent())
	drawReportText(img, fonts.h1, x+34, y+36, reportAccentDark(), title)
	cursor := y + 62
	for _, paragraph := range paragraphs {
		cursor = drawReportWrappedText(img, fonts.body, x+20, cursor, width-40, reportInk(), paragraph, 22)
		cursor += 8
	}
	return rect.Max.Y
}

func measureReportWrappedHeight(face font.Face, maxWidth int, text string, lineHeight int) int {
	words := strings.Fields(text)
	if len(words) == 0 {
		return lineHeight
	}
	lines := 1
	line := ""
	for _, word := range words {
		next := strings.TrimSpace(line + " " + word)
		if line != "" && font.MeasureString(face, next).Ceil() > maxWidth {
			lines++
			line = word
			continue
		}
		line = next
	}
	return lines * lineHeight
}

func drawReportBulletsSection(img *image.RGBA, fonts reportFonts, x, y, width int, title string, items []string) int {
	items = reportTexts(items)
	contentHeight := 0
	for _, item := range items {
		contentHeight += measureReportWrappedHeight(fonts.body, width-56, item, 21) + 7
	}
	height := maxReportInt(250, 74+contentHeight)
	if height > 385 {
		height = 385
	}
	rect := image.Rect(x, y, x+width, y+height)
	drawReportRoundedPanel(img, rect, 8, reportPanel(), reportLine())
	drawReportText(img, fonts.h2, x+18, y+32, reportAccentDark(), title)
	cursor := y + 58
	hidden := 0
	for i, item := range items {
		itemHeight := measureReportWrappedHeight(fonts.body, width-56, item, 21) + 7
		if cursor+itemHeight > rect.Max.Y-18 {
			hidden = len(items) - i
			break
		}
		fillReportRoundedRect(img, image.Rect(x+20, cursor-9, x+27, cursor-2), 3, reportAccent())
		cursor = drawReportWrappedText(img, fonts.body, x+38, cursor, width-56, reportInk(), item, 21)
		cursor += 7
	}
	if hidden > 0 && cursor <= rect.Max.Y-18 {
		drawReportText(img, fonts.small, x+38, rect.Max.Y-18, reportMuted(), fmt.Sprintf("+ %d madde detay sayfalarda", hidden))
	}
	return rect.Max.Y
}

func drawFinancialTable(img *image.RGBA, fonts reportFonts, result analysis.SymbolAnalysis, x, y int) {
	drawReportText(img, fonts.h1, x, y, reportAccentDark(), fundamentalSectionTitle(result))
	rows := fundamentalRows(result)
	cursor := y + 34
	table := image.Rect(x, cursor, x+1100, cursor+46*len(rows))
	drawReportRoundedPanel(img, table, 12, reportPanel(), reportLine())
	for i, row := range rows {
		var bg color.Color = reportPanel()
		if i%2 == 0 {
			bg = reportAccentPale()
		}
		rowRect := image.Rect(x+1, cursor, x+1099, cursor+46)
		fillReportRect(img, rowRect, bg)
		fillReportRect(img, image.Rect(x+16, cursor+45, x+1084, cursor+46), reportLineSoft())
		drawReportText(img, fonts.body, x+18, cursor+30, reportMuted(), reportText(row[0]))
		drawReportText(img, fonts.body, x+430, cursor+30, reportInk(), reportText(row[1]))
		cursor += 46
	}
}

func rsiTermText(result analysis.SymbolAnalysis) string {
	if isCryptoResult(result) {
		return "RSI yüksek: Varlık kısa vadede ısınmış olabilir; yeni alım için soğuma beklenebilir."
	}
	if isCommodityResult(result) {
		return "RSI yüksek: Altın/emtia kısa vadede ısınmış olabilir; yeni alım için soğuma beklenebilir."
	}
	return "RSI yüksek: Hisse kısa vadede ısınmış olabilir; yeni alım için soğuma beklenebilir."
}

func cryptoEntityLabel(result analysis.SymbolAnalysis) string {
	if strings.TrimSpace(result.Symbol) != "" {
		return result.Symbol
	}
	return "kripto varlık"
}

func entityDisplayLabel(result analysis.SymbolAnalysis) string {
	if strings.TrimSpace(result.CompanyName) != "" {
		return result.CompanyName
	}
	if strings.TrimSpace(result.Symbol) != "" {
		return result.Symbol
	}
	if isCommodityResult(result) {
		return "altın/emtia"
	}
	if isCryptoResult(result) {
		return "kripto varlık"
	}
	return "varlık"
}

func valueInvestingLines(result analysis.SymbolAnalysis) []string {
	v := result.Professional.ValueInvesting
	if !v.Computed {
		return []string{
			"İçsel değer güvenilir hesaplanamadı; güvenlik marjı kararı verilemiyor.",
			emptyFallback(v.Summary, "sahibine kalan nakit, normalize serbest nakit akımı veya sektör modeli için veri/güven yetersiz."),
			fmt.Sprintf("Güncel fiyat %.2f %s | sahibine kalan nakit son 12 ay %s | 5 yıllık normalize serbest nakit akımı %s | sermaye tahsisi %.0f/100 | rekabet gücü %.0f/100.", v.CurrentPrice, displayCurrency(result.Currency), formatMoney(v.OwnerEarnings.TTM, result.Currency), formatMoney(v.NormalizedFCF.Median5Y, result.Currency), v.CapitalAllocation.Score, v.Moat.Score),
		}
	}
	return []string{
		fmt.Sprintf("Güncel fiyat %.2f %s | baz içsel değer %.2f %s | güvenlik marjı %.1f%%.", v.CurrentPrice, displayCurrency(result.Currency), v.IntrinsicValue.Base, displayCurrency(result.Currency), v.MarginOfSafety.BasePct),
		fmt.Sprintf("İçsel değer aralığı: kötümser %.2f, baz %.2f, iyimser %.2f. Gereken marj %.1f%%.", v.IntrinsicValue.Bear, v.IntrinsicValue.Base, v.IntrinsicValue.Bull, v.MarginOfSafety.RequiredPct),
		fmt.Sprintf("Değer yatırım kararı: %s. Kalite %.0f/100, güven %.0f/100, model: %s.", v.DecisionLabel, v.QualityScore, v.Confidence, v.SectorModel.Label),
	}
}

func drawReportKeyValueTable(img *image.RGBA, fonts reportFonts, title string, rows [][]string, x, y, width int) int {
	drawReportText(img, fonts.h1, x, y, reportAccentDark(), title)
	cursor := y + 34
	if len(rows) == 0 {
		rows = [][]string{{"Kayıt", "Bu bölüm için rapora alınacak doğrulanmış sonuç yok."}}
	}
	rowHeights := make([]int, len(rows))
	totalHeight := 0
	for i, row := range rows {
		value := ""
		if len(row) > 1 {
			value = reportText(row[1])
		}
		height := maxReportInt(44, measureReportWrappedHeight(fonts.small, width-360, value, 18)+24)
		rowHeights[i] = height
		totalHeight += height
	}
	table := image.Rect(x, cursor, x+width, cursor+totalHeight)
	drawReportRoundedPanel(img, table, 12, reportPanel(), reportLine())
	for i, row := range rows {
		var bg color.Color = reportPanel()
		if i%2 == 0 {
			bg = reportAccentPale()
		}
		height := rowHeights[i]
		fillReportRect(img, image.Rect(x+1, cursor, x+width-1, cursor+height), bg)
		fillReportRect(img, image.Rect(x+16, cursor+height-1, x+width-16, cursor+height), reportLineSoft())
		label := ""
		value := ""
		if len(row) > 0 {
			label = reportText(row[0])
		}
		if len(row) > 1 {
			value = reportText(row[1])
		}
		drawReportWrappedText(img, fonts.small, x+18, cursor+27, 300, reportMuted(), label, 18)
		drawReportWrappedText(img, fonts.small, x+350, cursor+27, width-372, reportInk(), value, 18)
		cursor += height
	}
	return table.Max.Y
}

func drawReportTableHeader(img *image.RGBA, fonts reportFonts, xs []int, y int, headers []string) {
	fillReportRoundedRect(img, image.Rect(xs[0], y, 1170, y+36), 8, reportAccentSoft())
	for i, header := range headers {
		drawReportText(img, fonts.small, xs[i]+8, y+24, reportAccentDark(), header)
	}
}

func drawReportTableRow(img *image.RGBA, fonts reportFonts, xs []int, y int, row []string) int {
	height := 72
	fillReportRect(img, image.Rect(xs[0], y, 1170, y+height), reportPanel())
	fillReportRect(img, image.Rect(xs[0]+8, y+height-1, 1162, y+height), reportLineSoft())
	maxY := y + 26
	for i, value := range row {
		value = reportText(value)
		cursor := drawReportWrappedText(img, fonts.small, xs[i]+8, y+24, reportColumnWidth(xs, i), reportInk(), value, 18)
		if cursor > maxY {
			maxY = cursor
		}
	}
	if maxY+16 > y+height {
		height = maxY + 16 - y
	}
	return y + height
}

func reportColumnWidth(xs []int, index int) int {
	const tableRight = 1170
	if index < 0 || index >= len(xs) {
		return 120
	}
	right := tableRight
	if index+1 < len(xs) {
		right = xs[index+1]
	}
	width := right - xs[index] - 16
	if width < 80 {
		return 80
	}
	return width
}

func drawScenarioTable(img *image.RGBA, fonts reportFonts, result analysis.SymbolAnalysis, x, y int) {
	title := "Değerleme Senaryoları"
	if isCryptoResult(result) {
		title = "Teknik Bant Senaryoları"
	} else if isCommodityResult(result) {
		title = "Altın/Emtia Teknik Senaryoları"
	}
	drawReportText(img, fonts.h1, x, y, reportAccentDark(), title)
	headers := []string{"Senaryo", "Hedef", "Getiri", "Olasılık", "Sürücüler"}
	xs := []int{x, x + 150, x + 290, x + 430, x + 570}
	drawReportTableHeader(img, fonts, xs, y+34, headers)
	cursor := y + 76
	for _, scenario := range result.Professional.Scenarios {
		row := []string{
			strings.ToUpper(scenario.Name),
			fmt.Sprintf("%.2f", scenario.PriceTarget),
			formatPct(scenario.ReturnPct),
			formatPct(scenario.Probability * 100),
			reportText(strings.Join(scenario.Drivers, ", ")),
		}
		cursor = drawReportTableRow(img, fonts, xs, cursor, row)
	}
}

func drawConfidenceTable(img *image.RGBA, fonts reportFonts, confidence reportConfidence, x, y int) {
	drawReportText(img, fonts.h1, x, y, reportAccentDark(), fmt.Sprintf("Doğrulama Skoru: %.0f/100", confidence.Score))
	headers := []string{"Kontrol", "Puan", "Durum", "Açıklama"}
	xs := []int{x, x + 300, x + 430, x + 560}
	drawReportTableHeader(img, fonts, xs, y+34, headers)
	cursor := y + 76
	for _, item := range confidence.Items {
		row := []string{
			item.Label,
			fmt.Sprintf("%.1f / %.0f", item.Score, item.Max),
			item.Status,
			item.Detail,
		}
		cursor = drawReportTableRow(img, fonts, xs, cursor, row)
	}
}

func drawInstitutionalValidationTable(img *image.RGBA, fonts reportFonts, validation analysis.InstitutionalValidation, x, y int) {
	drawReportText(img, fonts.h1, x, y, reportAccentDark(), "Kontrol Sonuçları")
	headers := []string{"Kontrol", "Durum", "Açıklama"}
	xs := []int{x, x + 300, x + 450}
	drawReportTableHeader(img, fonts, xs, y+34, headers)
	cursor := y + 76
	for _, check := range validation.Checks {
		row := []string{
			check.Name,
			institutionalStatusTR(check.Status),
			check.Message,
		}
		cursor = drawReportTableRow(img, fonts, xs, cursor, row)
		if cursor > 1240 {
			break
		}
	}
}

func reportClassColor(class string) color.Color {
	switch class {
	case "good":
		return reportRGB(19, 121, 91)
	case "bad":
		return reportRGB(180, 35, 24)
	case "warn":
		return reportRGB(181, 71, 8)
	case "info":
		return reportRGB(23, 92, 211)
	default:
		return reportAccentDark()
	}
}

func institutionalStatusTR(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "info":
		return "Bilgi"
	case "critical":
		return "Kritik"
	case "pass":
		return "Geçti"
	case "limited":
		return "Sınırlı"
	case "missing":
		return "Eksik"
	case "fail":
		return "Başarısız"
	case "not_applicable", "not applicable":
		return "Uygulanmaz"
	default:
		return "Yok"
	}
}

func fillReportRect(img *image.RGBA, rect image.Rectangle, c color.Color) {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func strokeReportRect(img *image.RGBA, rect image.Rectangle, c color.Color) {
	fillReportRect(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+1), c)
	fillReportRect(img, image.Rect(rect.Min.X, rect.Max.Y-1, rect.Max.X, rect.Max.Y), c)
	fillReportRect(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+1, rect.Max.Y), c)
	fillReportRect(img, image.Rect(rect.Max.X-1, rect.Min.Y, rect.Max.X, rect.Max.Y), c)
}

func drawReportRoundedPanel(img *image.RGBA, rect image.Rectangle, radius int, fill, border color.Color) {
	fillReportRoundedRect(img, rect, radius, border)
	inner := rect.Inset(1)
	if inner.Empty() {
		return
	}
	fillReportRoundedRect(img, inner, maxReportInt(0, radius-1), fill)
}

func fillReportRoundedRect(img *image.RGBA, rect image.Rectangle, radius int, c color.Color) {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	radius = minReportInt(radius, minReportInt(rect.Dx()/2, rect.Dy()/2))
	if radius <= 0 {
		fillReportRect(img, rect, c)
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if reportRoundedRectContains(x, y, rect, radius) {
				img.Set(x, y, c)
			}
		}
	}
}

func reportRoundedRectContains(x, y int, rect image.Rectangle, radius int) bool {
	if x >= rect.Min.X+radius && x < rect.Max.X-radius {
		return true
	}
	if y >= rect.Min.Y+radius && y < rect.Max.Y-radius {
		return true
	}
	cx := rect.Min.X + radius
	if x >= rect.Max.X-radius {
		cx = rect.Max.X - radius - 1
	}
	cy := rect.Min.Y + radius
	if y >= rect.Max.Y-radius {
		cy = rect.Max.Y - radius - 1
	}
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= radius*radius
}

func drawReportText(img *image.RGBA, face font.Face, x, y int, c color.Color, text string) {
	d := &font.Drawer{Dst: img, Src: &image.Uniform{C: c}, Face: face, Dot: fixed.P(x, y)}
	d.DrawString(text)
}

func drawReportWrappedText(img *image.RGBA, face font.Face, x, y, maxWidth int, c color.Color, text string, lineHeight int) int {
	words := strings.Fields(text)
	line := ""
	cursor := y
	for _, word := range words {
		next := strings.TrimSpace(line + " " + word)
		if line != "" && font.MeasureString(face, next).Ceil() > maxWidth {
			drawReportText(img, face, x, cursor, c, line)
			cursor += lineHeight
			line = word
			continue
		}
		line = next
	}
	if line != "" {
		drawReportText(img, face, x, cursor, c, line)
		cursor += lineHeight
	}
	return cursor
}

func reportRGB(r, g, b uint8) color.Color {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func reportBg() color.Color {
	return reportRGB(246, 247, 249)
}

func reportPanel() color.Color {
	return reportRGB(255, 255, 255)
}

func reportInk() color.Color {
	return reportRGB(31, 41, 51)
}

func reportMuted() color.Color {
	return reportRGB(102, 112, 133)
}

func reportLine() color.Color {
	return reportRGB(208, 213, 221)
}

func reportLineSoft() color.Color {
	return reportRGB(234, 236, 240)
}

func reportAccent() color.Color {
	return reportRGB(15, 118, 110)
}

func reportAccentDark() color.Color {
	return reportRGB(19, 78, 74)
}

func reportAccentSoft() color.Color {
	return reportRGB(230, 244, 241)
}

func reportAccentPale() color.Color {
	return reportRGB(248, 250, 252)
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func analysisDateText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02_15-04-05") && value[10] == '_' {
		return value[:10] + " " + strings.ReplaceAll(value[11:], "-", ":")
	}
	return value
}

func minReportInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxReportInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func buildImagePDF(pages []image.Image) ([]byte, error) {
	const pageWidth = 595.28
	const pageHeight = 841.89
	var encodedPages [][]byte
	for _, page := range pages {
		encoded, err := encodePDFRGBImage(page)
		if err != nil {
			return nil, err
		}
		encodedPages = append(encodedPages, encoded)
	}

	var out bytes.Buffer
	offsets := []int{0}
	write := func(format string, args ...any) {
		fmt.Fprintf(&out, format, args...)
	}
	obj := func(id int, body string) {
		offsets = append(offsets, out.Len())
		write("%d 0 obj\n%s\nendobj\n", id, body)
	}
	write("%%PDF-1.4\n%%\xE2\xE3\xCF\xD3\n")
	totalObjects := 2 + len(pages)*3
	kids := []string{}
	for i := range pages {
		pageObj := 3 + i*3
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObj))
	}
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	obj(2, fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(pages), strings.Join(kids, " ")))
	for i, page := range pages {
		pageObj := 3 + i*3
		imageObj := pageObj + 1
		contentObj := pageObj + 2
		name := fmt.Sprintf("Im%d", i+1)
		bounds := page.Bounds()
		obj(pageObj, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /%s %d 0 R >> >> /Contents %d 0 R >>", pageWidth, pageHeight, name, imageObj, contentObj))
		offsets = append(offsets, out.Len())
		write("%d 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n", imageObj, bounds.Dx(), bounds.Dy(), len(encodedPages[i]))
		out.Write(encodedPages[i])
		write("\nendstream\nendobj\n")
		content := fmt.Sprintf("q\n%.2f 0 0 %.2f 0 0 cm\n/%s Do\nQ\n", pageWidth, pageHeight, name)
		obj(contentObj, fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content))
	}
	xref := out.Len()
	write("xref\n0 %d\n", totalObjects+1)
	write("0000000000 65535 f \n")
	for i := 1; i <= totalObjects; i++ {
		write("%010d 00000 n \n", offsets[i])
	}
	write("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", totalObjects+1, xref)
	return out.Bytes(), nil
}

func encodePDFRGBImage(img image.Image) ([]byte, error) {
	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	bounds := img.Bounds()
	row := make([]byte, 0, bounds.Dx()*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row = row[:0]
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			row = append(row, byte(r>>8), byte(g>>8), byte(b>>8))
		}
		if _, err := zw.Write(row); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
