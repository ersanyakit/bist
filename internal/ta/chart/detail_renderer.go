package chart

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sort"

	"hissebot/internal/ta/localize"
	"hissebot/internal/ta/ohlcv"

	"golang.org/x/image/font"
)

type DetailRenderInput struct {
	Symbol           string
	CompanyName      string
	AnalysisDate     string
	Timeframe        string
	LastClose        float64
	TimeframeScore   float64
	TrendBias        string
	IndicatorSignals []ohlcv.IndicatorResult
	Patterns         []ohlcv.PatternResult
	PatternScans     []ohlcv.PatternScanResult
	Disclaimer       string
}

type DetailRenderer struct {
	width  int
	height int
}

type detailPalette struct {
	background color.Color
	panel      color.Color
	text       color.Color
	muted      color.Color
	grid       color.Color
	good       color.Color
	goodSoft   color.Color
	bad        color.Color
	badSoft    color.Color
	warn       color.Color
	warnSoft   color.Color
	info       color.Color
	infoSoft   color.Color
}

func NewDetailRenderer() *DetailRenderer {
	return &DetailRenderer{width: 1800, height: 1560}
}

func (r *DetailRenderer) RenderPNG(ctx context.Context, input DetailRenderInput) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("detay grafigi cizimi iptal edildi: %w", err)
	}
	fonts, err := loadFonts()
	if err != nil {
		return nil, err
	}
	palette := detailPalette{
		background: rgb(13, 17, 28),
		panel:      rgb(20, 25, 40),
		text:       rgb(209, 214, 228),
		muted:      rgb(120, 130, 155),
		grid:       rgb(38, 44, 65),
		good:       rgb(38, 166, 154),
		goodSoft:   rgb(20, 44, 50),
		bad:        rgb(239, 83, 80),
		badSoft:    rgb(48, 20, 24),
		warn:       rgb(255, 177, 66),
		warnSoft:   rgb(42, 34, 14),
		info:       rgb(91, 143, 249),
		infoSoft:   rgb(20, 30, 55),
	}

	height := maxInt(r.height, detailImageHeight(input))
	img := image.NewRGBA(image.Rect(0, 0, r.width, height))
	fillRect(img, img.Bounds(), palette.background)

	cursor := drawDetailHeader(img, fonts, palette, input)
	cursor = drawDetailMetrics(img, fonts, palette, input, cursor)
	cursor = drawAllIndicatorsPanel(img, fonts, palette, input, cursor)
	cursor = drawAllPatternsPanel(img, fonts, palette, input, cursor)
	drawDetailFooter(img, fonts, palette, input, cursor)

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("detay png olusturulamadi: %w", err)
	}
	return out.Bytes(), nil
}

const maxDetailIndicators = 90
const maxDetailPatterns = 90

func detailImageHeight(input DetailRenderInput) int {
	indicatorColumns := 2
	shown := minInt(len(input.IndicatorSignals), maxDetailIndicators)
	indicatorRows := rowsFor(shown, indicatorColumns)
	patternColumns := 2
	shownPatterns := minInt(len(patternScanItems(input)), maxDetailPatterns)
	patternRows := rowsFor(shownPatterns, patternColumns)
	height := 36
	height += 118
	height += 112
	height += 92 + indicatorRows*42 + 36 // +36 for possible overflow notice
	height += 72
	height += 92 + maxInt(1, patternRows)*46
	height += 122
	// Cap at a sensible maximum
	if height > 5500 {
		height = 5500
	}
	return height
}

// prioritisedIndicators sorts signals: non-neutral first, then alphabetical.
func prioritisedIndicators(signals []ohlcv.IndicatorResult) []ohlcv.IndicatorResult {
	sorted := make([]ohlcv.IndicatorResult, len(signals))
	copy(sorted, signals)
	sort.SliceStable(sorted, func(i, j int) bool {
		ni := sorted[i].Signal == "neutral" || sorted[i].Signal == ""
		nj := sorted[j].Signal == "neutral" || sorted[j].Signal == ""
		if ni != nj {
			return !ni // non-neutral first
		}
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

func drawDetailHeader(img *image.RGBA, fonts fontSet, palette detailPalette, input DetailRenderInput) int {
	drawText(img, fonts.title, 64, 68, palette.text, fmt.Sprintf("%s Detaylı Tarama Sonuçları", input.Symbol))
	subtitle := fmt.Sprintf("%s | %s | %s", emptyFallback(input.CompanyName, "Şirket adı yok"), localize.Timeframe(input.Timeframe), input.AnalysisDate)
	drawText(img, fonts.body, 68, 104, palette.muted, subtitle)
	drawDetailBadge(img, fonts.bold, image.Rect(1380, 36, 1730, 98), localize.Bias(input.TrendBias), signalColor(input.TrendBias, palette), signalSoft(input.TrendBias, palette))
	return 136
}

func drawDetailMetrics(img *image.RGBA, fonts fontSet, palette detailPalette, input DetailRenderInput, y int) int {
	computed, external, proxy, active := indicatorCounts(input.IndicatorSignals)
	matchedPatterns, bullishPatterns, bearishPatterns := patternScanCounts(input)
	metrics := []struct {
		label string
		value string
		color color.Color
	}{
		{"Son Kapanış", fmt.Sprintf("%.2f", input.LastClose), palette.text},
		{"Teknik Skor", fmt.Sprintf("%.2f/100", input.TimeframeScore), palette.info},
		{"Tüm İndikatör", fmt.Sprintf("%d", len(input.IndicatorSignals)), palette.text},
		{"Aktif / Hesaplanan", fmt.Sprintf("%d / %d", active, computed), palette.good},
		{"Sinyal Dışı", fmt.Sprintf("Yaklaşık:%d Dış:%d", proxy, external), palette.warn},
		{"Formasyon", fmt.Sprintf("B:%d Y:%d D:%d T:%d", matchedPatterns, bullishPatterns, bearishPatterns, len(patternScanItems(input))), palette.text},
	}
	x := 64
	for _, metric := range metrics {
		rect := image.Rect(x, y, x+266, y+92)
		drawDetailPanel(img, rect, palette)
		drawText(img, fonts.small, x+18, y+34, palette.muted, metric.label)
		drawText(img, fonts.bold, x+18, y+68, metric.color, metric.value)
		x += 286
	}
	return y + 126
}

func drawAllIndicatorsPanel(img *image.RGBA, fonts fontSet, palette detailPalette, input DetailRenderInput, y int) int {
	columns := 2
	rowHeight := 42
	panelWidth := img.Bounds().Dx() - 128
	signals := prioritisedIndicators(input.IndicatorSignals)
	total := len(signals)
	shown := minInt(total, maxDetailIndicators)
	rows := rowsFor(shown, columns)
	rect := image.Rect(64, y, 64+panelWidth, y+92+rows*rowHeight)
	drawDetailPanel(img, rect, palette)
	title := fmt.Sprintf("İndikatör Sonuçları (%d/%d gösteriliyor)", shown, total)
	if shown == total {
		title = fmt.Sprintf("Tüm İndikatör Sonuçları (%d)", total)
	}
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+38, palette.text, title)
	drawText(img, fonts.small, rect.Min.X+24, rect.Min.Y+66, palette.muted, "Önce anlamlı sinyaller; hesaplanan kayıtlar ile yaklaşık/sinyal dışı ve dış veri kayıtları ayrıdır")

	colGap := 16
	colWidth := (rect.Dx() - 48 - colGap*(columns-1)) / columns
	startY := rect.Min.Y + 92
	for i := 0; i < shown; i++ {
		col := i % columns
		row := i / columns
		x := rect.Min.X + 24 + col*(colWidth+colGap)
		rowY := startY + row*rowHeight
		drawCompactIndicatorRow(img, fonts, palette, x, rowY, colWidth, signals[i])
	}
	bottom := rect.Max.Y
	if shown < total {
		notice := fmt.Sprintf("+ %d indikatör daha (grafik alanı kapasitesi aşıldı)", total-shown)
		drawText(img, fonts.small, rect.Min.X+24, bottom+22, palette.muted, notice)
		bottom += 36
	}
	return bottom + 36
}

func drawCompactIndicatorRow(img *image.RGBA, fonts fontSet, palette detailPalette, x, y, width int, item ohlcv.IndicatorResult) {
	row := image.Rect(x, y, x+width, y+34)
	fillRoundedRect(img, row, 5, rgb(24, 31, 49))
	drawRectOutline(img, row, palette.grid)

	chipColor := signalColor(item.Signal, palette)
	chipSoft := signalSoft(item.Signal, palette)
	drawDetailBadge(img, fonts.small, image.Rect(row.Max.X-112, row.Min.Y+5, row.Max.X-8, row.Max.Y-5), signalText(item.Signal), chipColor, chipSoft)

	nameWidth := width - 246
	drawText(img, fonts.small, x+10, y+22, palette.text, truncateText(fonts.small, item.Name, nameWidth))
	meta := indicatorMeta(item)
	drawText(img, fonts.small, x+nameWidth+18, y+22, palette.muted, truncateText(fonts.small, meta, 118))
}

func indicatorMeta(item ohlcv.IndicatorResult) string {
	if !item.Computed {
		return item.Category
	}
	return fmt.Sprintf("%.2f", item.Value)
}

func drawAllPatternsPanel(img *image.RGBA, fonts fontSet, palette detailPalette, input DetailRenderInput, y int) int {
	allItems := patternScanItems(input)
	// Show matched patterns first, then unmatched
	sort.SliceStable(allItems, func(i, j int) bool {
		return allItems[i].Matched && !allItems[j].Matched
	})
	total := len(allItems)
	shown := minInt(total, maxDetailPatterns)
	items := allItems[:shown]

	columns := 2
	rowHeight := 42
	rows := maxInt(1, rowsFor(len(items), columns))
	panelWidth := img.Bounds().Dx() - 128
	rect := image.Rect(64, y, 64+panelWidth, y+92+rows*rowHeight)
	// Clip to image height
	if rect.Max.Y > img.Bounds().Dy()-60 {
		rect.Max.Y = img.Bounds().Dy() - 60
	}
	drawDetailPanel(img, rect, palette)
	title := fmt.Sprintf("Formasyon Sonuçları (%d/%d gösteriliyor)", shown, total)
	if shown == total {
		title = fmt.Sprintf("Tüm Formasyon Sonuçları (%d)", total)
	}
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+38, palette.text, title)
	drawText(img, fonts.small, rect.Min.X+24, rect.Min.Y+66, palette.muted, "Önce bulunanlar; katalogdaki tüm formasyonlar: bulunanlar ve bulunmayanlar")

	if len(items) == 0 {
		drawText(img, fonts.body, rect.Min.X+24, rect.Min.Y+120, palette.muted, "Formasyon katalog taraması yok.")
		return rect.Max.Y + 36
	}
	colGap := 16
	colWidth := (rect.Dx() - 48 - colGap*(columns-1)) / columns
	startY := rect.Min.Y + 92
	for i, item := range items {
		rowY := startY + (i/columns)*rowHeight
		if rowY+rowHeight > rect.Max.Y {
			break
		}
		col := i % columns
		x := rect.Min.X + 24 + col*(colWidth+colGap)
		drawCompactPatternScanRow(img, fonts, palette, x, rowY, colWidth, item)
	}
	return rect.Max.Y + 36
}

func drawCompactPatternScanRow(img *image.RGBA, fonts fontSet, palette detailPalette, x, y, width int, item ohlcv.PatternScanResult) {
	row := image.Rect(x, y, x+width, y+34)
	fillRoundedRect(img, row, 5, rgb(24, 31, 49))
	drawRectOutline(img, row, palette.grid)

	chipText := "Yok"
	chipColor := palette.muted
	chipSoft := rgb(31, 38, 58)
	if item.Matched {
		chipText = "Bulundu"
		chipColor = signalColor(item.Direction, palette)
		chipSoft = signalSoft(item.Direction, palette)
	}
	drawDetailBadge(img, fonts.small, image.Rect(row.Max.X-88, row.Min.Y+5, row.Max.X-8, row.Max.Y-5), chipText, chipColor, chipSoft)

	name := localize.PatternName(item.Name)
	drawText(img, fonts.small, x+10, y+22, palette.text, truncateText(fonts.small, name, width-236))
	meta := patternScanMeta(item)
	drawText(img, fonts.small, row.Max.X-226, y+22, palette.muted, truncateText(fonts.small, meta, 126))
}

func drawDetailFooter(img *image.RGBA, fonts fontSet, palette detailPalette, input DetailRenderInput, y int) {
	rect := image.Rect(64, y, 1730, y+92)
	drawDetailPanel(img, rect, palette)
	drawText(img, fonts.bold, rect.Min.X+24, rect.Min.Y+36, palette.text, "Okuma Notu")
	text := "Bu görsel tüm indikatörleri ve katalogdaki tüm formasyonları çizer. Yaklaşık veya dış veri isteyen kayıtlar al-sat sinyali sayılmaz."
	drawText(img, fonts.small, rect.Min.X+24, rect.Min.Y+64, palette.text, truncateText(fonts.small, text, rect.Dx()-48))
	drawText(img, fonts.small, rect.Min.X+24, rect.Max.Y-14, palette.muted, emptyFallback(input.Disclaimer, ohlcv.Disclaimer))
}

func patternScanItems(input DetailRenderInput) []ohlcv.PatternScanResult {
	out := append([]ohlcv.PatternScanResult{}, input.PatternScans...)
	if len(out) == 0 {
		for _, pattern := range input.Patterns {
			if pattern.Confidence < 0.5 {
				continue
			}
			out = append(out, ohlcv.PatternScanResult{
				Name:       pattern.Name,
				Category:   pattern.Category,
				Direction:  pattern.Direction,
				Matched:    true,
				Confidence: pattern.Confidence,
				Source:     "legacy",
				Evidence:   pattern.Evidence,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Matched != out[j].Matched {
			return out[i].Matched
		}
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].Category == out[j].Category {
			if out[i].Group == out[j].Group {
				return out[i].Name < out[j].Name
			}
			return out[i].Group < out[j].Group
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func patternScanMeta(item ohlcv.PatternScanResult) string {
	if item.Matched {
		return fmt.Sprintf("%.2f %s", item.Confidence, localize.Direction(item.Direction))
	}
	return localize.Direction(item.Direction)
}

func indicatorCounts(items []ohlcv.IndicatorResult) (computed, external, proxy, active int) {
	for _, item := range items {
		if item.Computed {
			computed++
		}
		if item.Signal == "requires_external_data" {
			external++
		}
		if item.Signal == "proxy_only" {
			proxy++
		}
		if item.Computed && item.Confidence >= 0.5 && item.Signal != "neutral" && item.Signal != "info" {
			active++
		}
	}
	return computed, external, proxy, active
}

func patternScanCounts(input DetailRenderInput) (matched, bullish, bearish int) {
	for _, item := range patternScanItems(input) {
		if !item.Matched {
			continue
		}
		matched++
		if item.Direction == "bullish" {
			bullish++
		}
		if item.Direction == "bearish" {
			bearish++
		}
	}
	return matched, bullish, bearish
}

func rowsFor(count, columns int) int {
	if count <= 0 {
		return 0
	}
	if columns <= 1 {
		return count
	}
	return (count + columns - 1) / columns
}

func drawDetailPanel(img *image.RGBA, rect image.Rectangle, palette detailPalette) {
	drawRoundedBadge(img, rect, 8, palette.grid, palette.panel)
}

func drawDetailBadge(img *image.RGBA, face font.Face, rect image.Rectangle, text string, border, fill color.Color) {
	drawRoundedBadge(img, rect, 6, border, fill)
	drawTextCentered(img, face, rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2+6, border, truncateText(face, text, rect.Dx()-10))
}

func signalColor(signal string, palette detailPalette) color.Color {
	switch signal {
	case "bullish", "oversold", "level_nearby":
		return palette.good
	case "bearish", "overbought":
		return palette.bad
	case "high_volatility", "requires_external_data":
		return palette.warn
	default:
		return palette.info
	}
}

func signalSoft(signal string, palette detailPalette) color.Color {
	switch signal {
	case "bullish", "oversold", "level_nearby":
		return palette.goodSoft
	case "bearish", "overbought":
		return palette.badSoft
	case "high_volatility", "requires_external_data":
		return palette.warnSoft
	default:
		return palette.infoSoft
	}
}

func signalText(signal string) string {
	switch signal {
	case "bullish":
		return "Yükseliş"
	case "bearish":
		return "Düşüş"
	case "oversold":
		return "Aş. Satım"
	case "overbought":
		return "Aş. Alım"
	case "high_volatility":
		return "Yük. Vol"
	case "level_nearby":
		return "Seviye"
	case "requires_external_data":
		return "Dış Veri Gerekir"
	case "proxy_only":
		return "Sinyal Değil"
	case "info":
		return "Bilgi"
	case "neutral":
		return "Nötr"
	default:
		return signal
	}
}

func truncateText(face font.Face, text string, maxWidth int) string {
	if maxWidth <= 0 || font.MeasureString(face, text).Ceil() <= maxWidth {
		return text
	}
	ellipsis := "..."
	for len(text) > 0 && font.MeasureString(face, text+ellipsis).Ceil() > maxWidth {
		text = text[:len(text)-1]
	}
	if text == "" {
		return ellipsis
	}
	return text + ellipsis
}
