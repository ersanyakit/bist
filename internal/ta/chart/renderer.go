// internal/chart/renderer.go
package chart

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"sort"
	"strings"
	"time"

	"hissebot/internal/ta/formations"
	"hissebot/internal/ta/localize"
	"hissebot/internal/ta/ohlcv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type RenderInput struct {
	Symbol     string                         `json:"symbol"`
	Timeframe  string                         `json:"timeframe"`
	Candles    []ohlcv.Candle                 `json:"candles"`
	Indicators ohlcv.IndicatorSnapshot        `json:"indicators"`
	Levels     []ohlcv.SupportResistanceLevel `json:"levels"`
	Patterns   []ohlcv.PatternResult          `json:"patterns"`
	TradePlan  ohlcv.TradePlan                `json:"trade_plan"`
	Drawings   formations.DrawingObjects      `json:"drawings"`
	Disclaimer string                         `json:"disclaimer"`
}

type ChartRenderer interface {
	RenderPNG(ctx context.Context, input RenderInput) ([]byte, error)
}

type PNGRenderer struct {
	width  int
	height int
}

var chartDateLocation = time.FixedZone("TRT", 3*60*60)

func NewPNGRenderer() *PNGRenderer {
	return &PNGRenderer{width: 2800, height: 1600}
}

func (r *PNGRenderer) RenderPNG(ctx context.Context, input RenderInput) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("grafik cizimi iptal edildi: %w", err)
	}
	if len(input.Candles) == 0 {
		return nil, fmt.Errorf("grafik icin mum verisi yok")
	}

	img := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	fillRect(img, img.Bounds(), rgb(13, 17, 28))

	fonts, err := loadFonts()
	if err != nil {
		return nil, err
	}

	palette := chartPalette{
		text:              rgb(209, 214, 228),
		muted:             rgb(120, 130, 155),
		grid:              rgb(38, 44, 65),
		axis:              rgb(100, 115, 150),
		up:                rgb(38, 166, 154),
		down:              rgb(239, 83, 80),
		volume:            rgb(90, 100, 130),
		support:           rgb(38, 166, 154),
		supportSurface:    rgb(20, 44, 50),
		resistance:        rgb(239, 83, 80),
		resistanceSurface: rgb(48, 20, 24),
		entry:             rgb(91, 143, 249),
		target:            rgb(38, 166, 154),
		stop:              rgb(239, 83, 80),
		ma20:              rgb(255, 177, 66),
		ma50:              rgb(91, 143, 249),
		ma200:             rgb(176, 190, 197),
		panel:             rgb(20, 25, 40),
	}
	geometryMode := hasTrendlineDrawings(input)
	lastCandle := input.Candles[len(input.Candles)-1]
	input.Levels = normalizeLevelsByPrice(input.Levels, lastCandle.EffectiveClose())

	layout := chartLayout{
		left:         70,
		top:          175,
		priceHeight:  960,
		volumeGap:    52,
		volumeHeight: 220,
		rightPanel:   650,
		bottom:       110,
	}
	layout.chartWidth = r.width - layout.left - layout.rightPanel - 220
	layout.priceBottom = layout.top + layout.priceHeight
	layout.volumeTop = layout.priceBottom + layout.volumeGap
	layout.volumeBottom = layout.volumeTop + layout.volumeHeight
	projectionSlots := futureProjectionSlots(input.Candles, input.Drawings, input.Timeframe)
	xDenominator := maxInt(len(input.Candles)-1+projectionSlots, 1)

	drawTitle(img, fonts, palette, input)
	drawPlotBackground(img, layout, palette)

	priceAxis := newPriceAxis(input)
	maxVolume := volumeMax(input.Candles)
	toX := func(index int) int {
		if len(input.Candles) == 1 && projectionSlots == 0 {
			return layout.left + layout.chartWidth/2
		}
		return layout.left + int(math.Round(float64(index)*float64(layout.chartWidth)/float64(xDenominator)))
	}
	toY := func(price float64) int {
		return priceAxis.toY(layout, price)
	}
	toVolY := func(volume float64) int {
		if maxVolume <= 0 {
			return layout.volumeBottom
		}
		return layout.volumeBottom - int(math.Round(volume*float64(layout.volumeHeight)/maxVolume))
	}

	drawPriceGrid(img, fonts, layout, palette, priceAxis, toY)
	drawTimeLabels(img, fonts, layout, palette, input.Candles, toX)
	drawVolumeBars(img, layout, palette, input.Candles, toX, toVolY)
	drawCandles(img, layout, palette, input.Candles, toX, toY)
	drawPatternMarkers(img, fonts, layout, palette, input.Candles, input.Patterns, toX)
	if !geometryMode {
		drawMovingAverage(img, layout, palette, input.Candles, toX, toY, 20, palette.ma20)
		drawMovingAverage(img, layout, palette, input.Candles, toX, toY, 50, palette.ma50)
		drawMovingAverage(img, layout, palette, input.Candles, toX, toY, 200, palette.ma200)
		drawLevels(img, layout, palette, input.Levels, toY)
	}
	drawGeometryOverlays(img, fonts, layout, input, toX, toY, projectionSlots)
	if activeTradePlan(input.TradePlan) {
		drawTradePlan(img, layout, palette, input.TradePlan, toY)
	}
	if !geometryMode {
		drawLastClose(img, layout, palette, lastCandle.EffectiveClose(), toY)
	}
	if !geometryMode {
		drawLevelCallouts(img, fonts, layout, palette, input.Candles, input.Levels, toX, toY)
	}
	drawCurrentPriceMarker(img, fonts, layout, palette, lastCandle.EffectiveClose(), currentPriceColor(palette, input.Candles), toY, toX(len(input.Candles)-1))
	drawLegend(img, fonts, layout, palette, geometryMode)
	drawInfoPanel(img, fonts, layout, palette, input)

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("png olusturulamadi: %w", err)
	}
	return out.Bytes(), nil
}

type fontSet struct {
	title font.Face
	bold  font.Face
	body  font.Face
	small font.Face
}

func loadFonts() (fontSet, error) {
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return fontSet{}, fmt.Errorf("grafik yazı tipi yuklenemedi: %w", err)
	}
	makeFace := func(size float64) (font.Face, error) {
		return opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 96, Hinting: font.HintingFull})
	}
	title, err := makeFace(42)
	if err != nil {
		return fontSet{}, err
	}
	bold, err := makeFace(23)
	if err != nil {
		return fontSet{}, err
	}
	body, err := makeFace(19)
	if err != nil {
		return fontSet{}, err
	}
	small, err := makeFace(16)
	if err != nil {
		return fontSet{}, err
	}
	return fontSet{title: title, bold: bold, body: body, small: small}, nil
}

type chartLayout struct {
	left         int
	top          int
	priceHeight  int
	priceBottom  int
	volumeGap    int
	volumeTop    int
	volumeHeight int
	volumeBottom int
	chartWidth   int
	rightPanel   int
	bottom       int
}

type chartPalette struct {
	text              color.Color
	muted             color.Color
	grid              color.Color
	axis              color.Color
	up                color.Color
	down              color.Color
	volume            color.Color
	support           color.Color
	supportSurface    color.Color
	resistance        color.Color
	resistanceSurface color.Color
	entry             color.Color
	target            color.Color
	stop              color.Color
	ma20              color.Color
	ma50              color.Color
	ma200             color.Color
	panel             color.Color
}

func drawTitle(img *image.RGBA, fonts fontSet, palette chartPalette, input RenderInput) {
	title := fmt.Sprintf("%s %s Mum Grafiği", input.Symbol, localize.Timeframe(input.Timeframe))
	drawText(img, fonts.title, 64, 68, palette.text, title)
	drawText(img, fonts.body, 68, 104, palette.muted, "Destek/direnç seviyeleri ve mum grafiği | Yatırım tavsiyesi değildir")
	if len(input.Candles) > 0 {
		drawCurrentPriceBadgeRight(img, fonts, palette, img.Bounds().Max.X-64, 42, input.Candles[len(input.Candles)-1].EffectiveClose(), currentPriceColor(palette, input.Candles))
	}
}

func drawCurrentPriceBadgeRight(img *image.RGBA, fonts fontSet, palette chartPalette, rightX, y int, price float64, marker color.Color) {
	text := "Güncel fiyat: " + formatPrice(price)
	width := font.MeasureString(fonts.body, text).Ceil() + 34
	drawCurrentPriceBadge(img, fonts, palette, rightX-width, y, price, marker)
}

func drawCurrentPriceBadge(img *image.RGBA, fonts fontSet, palette chartPalette, x, y int, price float64, marker color.Color) {
	text := "Güncel fiyat: " + formatPrice(price)
	width := font.MeasureString(fonts.body, text).Ceil() + 34
	rect := image.Rect(x, y-25, x+width, y+8)
	fillRect(img, rect, marker)
	drawRectOutline(img, rect, withAlpha(rgb(0, 0, 0), 100))
	drawText(img, fonts.body, x+17, y, rgb(255, 255, 255), text)
}

func drawPlotBackground(img *image.RGBA, layout chartLayout, palette chartPalette) {
	fillRect(img, image.Rect(layout.left, layout.top, layout.left+layout.chartWidth, layout.priceBottom), palette.panel)
	fillRect(img, image.Rect(layout.left, layout.volumeTop, layout.left+layout.chartWidth, layout.volumeBottom), palette.panel)
	drawRectOutline(img, image.Rect(layout.left, layout.top, layout.left+layout.chartWidth, layout.priceBottom), palette.grid)
	drawRectOutline(img, image.Rect(layout.left, layout.volumeTop, layout.left+layout.chartWidth, layout.volumeBottom), palette.grid)
}

type priceAxis struct {
	min    float64
	max    float64
	log    bool
	logMin float64
	logMax float64
}

func newPriceAxis(input RenderInput) priceAxis {
	minPrice, maxPrice := priceBounds(input)
	if minPrice == math.MaxFloat64 || maxPrice == -math.MaxFloat64 {
		return priceAxis{min: 0, max: 1}
	}
	if minPrice > 0 && maxPrice/minPrice >= 8 {
		logMin := math.Log(minPrice)
		logMax := math.Log(maxPrice)
		padding := math.Max((logMax-logMin)*0.08, 0.02)
		return priceAxis{
			min:    math.Exp(logMin - padding),
			max:    math.Exp(logMax + padding),
			log:    true,
			logMin: logMin - padding,
			logMax: logMax + padding,
		}
	}
	padding := math.Max((maxPrice-minPrice)*0.08, maxPrice*0.005)
	minPrice -= padding
	maxPrice += padding
	if minPrice > 0 {
		minPrice = math.Max(minPrice, maxPrice*0.0001)
	}
	return priceAxis{min: minPrice, max: maxPrice}
}

func (axis priceAxis) toY(layout chartLayout, price float64) int {
	if axis.log {
		if price <= 0 || math.Abs(axis.logMax-axis.logMin) < 0.000001 {
			return layout.top + layout.priceHeight/2
		}
		logPrice := math.Log(clampFloat(price, axis.min, axis.max))
		return layout.top + int(math.Round((axis.logMax-logPrice)*float64(layout.priceHeight)/(axis.logMax-axis.logMin)))
	}
	if math.Abs(axis.max-axis.min) < 0.000001 {
		return layout.top + layout.priceHeight/2
	}
	return layout.top + int(math.Round((axis.max-price)*float64(layout.priceHeight)/(axis.max-axis.min)))
}

func (axis priceAxis) gridValue(index, steps int) float64 {
	if steps <= 0 {
		return axis.max
	}
	ratio := float64(index) / float64(steps)
	if axis.log {
		return math.Exp(axis.logMax - ratio*(axis.logMax-axis.logMin))
	}
	return axis.max - (axis.max-axis.min)*ratio
}

func priceBounds(input RenderInput) (float64, float64) {
	minPrice := math.MaxFloat64
	maxPrice := -math.MaxFloat64
	for _, candle := range input.Candles {
		minPrice = math.Min(minPrice, candle.EffectiveLow())
		maxPrice = math.Max(maxPrice, candle.EffectiveHigh())
	}
	if !hasTrendlineDrawings(input) {
		for _, level := range input.Levels {
			if level.Price > 0 {
				minPrice = math.Min(minPrice, level.Price)
				maxPrice = math.Max(maxPrice, level.Price)
			}
		}
	}
	for _, line := range input.Drawings.Lines {
		for _, value := range []float64{line.Price, line.StartPrice, line.EndPrice} {
			if value > 0 {
				minPrice = math.Min(minPrice, value)
				maxPrice = math.Max(maxPrice, value)
			}
		}
	}
	for _, path := range input.Drawings.Paths {
		for _, point := range path.Points {
			if point.Price > 0 {
				minPrice = math.Min(minPrice, point.Price)
				maxPrice = math.Max(maxPrice, point.Price)
			}
		}
	}
	if activeTradePlan(input.TradePlan) {
		for _, value := range []float64{
			input.TradePlan.EntryMin,
			input.TradePlan.EntryMax,
			input.TradePlan.TakeProfit1,
			input.TradePlan.TakeProfit2,
			input.TradePlan.StopLoss,
		} {
			if value > 0 {
				minPrice = math.Min(minPrice, value)
				maxPrice = math.Max(maxPrice, value)
			}
		}
	}
	return minPrice, maxPrice
}

func volumeMax(candles []ohlcv.Candle) float64 {
	maxVolume := 0.0
	for _, candle := range candles {
		maxVolume = math.Max(maxVolume, candle.EffectiveVolume())
	}
	return maxVolume
}

func drawPriceGrid(img *image.RGBA, fonts fontSet, layout chartLayout, palette chartPalette, axis priceAxis, toY func(float64) int) {
	for i := 0; i <= 6; i++ {
		price := axis.gridValue(i, 6)
		y := toY(price)
		drawLineWidth(img, layout.left, y, layout.left+layout.chartWidth, y, 1, palette.grid)
		drawText(img, fonts.small, priceAxisLabelX(layout), y+6, palette.muted, formatPrice(price))
	}
	drawText(img, fonts.small, priceAxisLabelX(layout), layout.volumeTop-10, palette.muted, "Hacim")
}

func priceAxisLabelX(layout chartLayout) int {
	return layout.left + layout.chartWidth + 12
}

func drawTimeLabels(img *image.RGBA, fonts fontSet, layout chartLayout, palette chartPalette, candles []ohlcv.Candle, toX func(int) int) {
	steps := 6
	if len(candles) < steps {
		steps = len(candles)
	}
	for i := 0; i < steps; i++ {
		index := int(math.Round(float64(i) * float64(len(candles)-1) / float64(maxInt(steps-1, 1))))
		x := toX(index)
		drawLineWidth(img, x, layout.top, x, layout.priceBottom, 1, rgba(38, 44, 65, 100))
		label := chartDateLabel(candles[index].Time)
		drawTextCentered(img, fonts.small, x, layout.volumeBottom+34, palette.muted, label)
	}
}

func drawVolumeBars(img *image.RGBA, layout chartLayout, palette chartPalette, candles []ohlcv.Candle, toX func(int) int, toVolY func(float64) int) {
	width := volumeBarWidth(layout.chartWidth, len(candles))
	for i, candle := range candles {
		x := toX(i)
		top := toVolY(candle.EffectiveVolume())
		barColor := blendOnDark(palette.volume, 0.48)
		if candle.EffectiveClose() >= candle.EffectiveOpen() {
			barColor = blendOnDark(palette.up, 0.42)
		}
		if candle.EffectiveClose() < candle.EffectiveOpen() {
			barColor = blendOnDark(palette.down, 0.42)
		}
		fillRect(img, image.Rect(x-width/2, top, x+width/2+1, layout.volumeBottom), barColor)
	}
}

func drawCandles(img *image.RGBA, layout chartLayout, palette chartPalette, candles []ohlcv.Candle, toX func(int) int, toY func(float64) int) {
	width := candleBodyWidth(layout.chartWidth, len(candles))
	wickWidth := 1
	if width >= 7 {
		wickWidth = 2
	}
	for i, candle := range candles {
		open := candle.EffectiveOpen()
		closePrice := candle.EffectiveClose()
		high := candle.EffectiveHigh()
		low := candle.EffectiveLow()
		x := toX(i)
		candleColor := palette.down
		if closePrice >= open {
			candleColor = palette.up
		}
		wickColor := candleWickColor(candleColor)
		outlineColor := candleOutlineColor(candleColor)
		drawLineWidth(img, x, toY(high), x, toY(low), wickWidth, wickColor)

		openY := toY(open)
		closeY := toY(closePrice)
		top := minInt(openY, closeY)
		bottom := maxInt(openY, closeY)
		left := x - width/2
		right := x + width/2
		if bottom-top <= 1 {
			drawLineWidth(img, left, top, right, top, 1, outlineColor)
			continue
		}
		if bottom-top < 2 {
			bottom = top + 2
		}
		body := image.Rect(left, top, right+1, bottom+1)
		fillRect(img, body, candleColor)
		drawRectOutline(img, body, outlineColor)
	}
}

func drawPatternMarkers(img *image.RGBA, fonts fontSet, layout chartLayout, palette chartPalette, candles []ohlcv.Candle, patterns []ohlcv.PatternResult, toX func(int) int) {
	if len(candles) == 0 || len(patterns) == 0 {
		return
	}
	shown := 0
	for _, pattern := range patterns {
		if shown >= 5 || !chartPatternWindowDrawable(candles, pattern) {
			continue
		}
		accent := patternMarkerColor(pattern, palette)
		x0 := toX(pattern.StartIndex)
		x1 := toX(pattern.EndIndex)
		if x1 < x0 {
			x0, x1 = x1, x0
		}
		if x1-x0 < 18 {
			center := (x0 + x1) / 2
			x0 = center - 9
			x1 = center + 9
		}
		x0 = maxInt(layout.left, x0)
		x1 = minInt(layout.left+layout.chartWidth, x1)
		if x1 <= x0 {
			continue
		}
		window := image.Rect(x0, layout.top+8, x1, layout.priceBottom-8)
		blendRect(img, window, accent, 0.10)
		drawDashedLineWidth(img, x0, layout.top+8, x0, layout.priceBottom-8, 8, 8, 1, blendOnDark(accent, 0.85))
		drawDashedLineWidth(img, x1, layout.top+8, x1, layout.priceBottom-8, 8, 8, 1, blendOnDark(accent, 0.85))

		label := fmt.Sprintf("%s %.0f", chartPatternLabel(pattern.Name), pattern.Confidence*100)
		badgeWidth := minInt(230, maxInt(94, font.MeasureString(fonts.small, label).Ceil()+22))
		badgeX := clampInt(x0+6, layout.left+8, layout.left+layout.chartWidth-badgeWidth-8)
		badgeY := layout.top + 28 + shown*30
		badge := image.Rect(badgeX, badgeY-20, badgeX+badgeWidth, badgeY+7)
		fillRoundedRect(img, badge, 5, blendOnDark(accent, 0.34))
		drawRectOutline(img, badge, blendOnDark(accent, 0.78))
		drawText(img, fonts.small, badgeX+10, badgeY, rgb(232, 238, 247), truncateText(fonts.small, label, badgeWidth-18))
		shown++
	}
}

func chartPatternWindowDrawable(candles []ohlcv.Candle, pattern ohlcv.PatternResult) bool {
	if len(candles) == 0 || strings.TrimSpace(pattern.Name) == "" {
		return false
	}
	if pattern.StartIndex < 0 || pattern.EndIndex < pattern.StartIndex || pattern.EndIndex >= len(candles) {
		return false
	}
	return true
}

func patternMarkerColor(pattern ohlcv.PatternResult, palette chartPalette) color.Color {
	switch strings.ToLower(strings.TrimSpace(pattern.Direction)) {
	case "bullish":
		return palette.support
	case "bearish":
		return palette.resistance
	default:
		return palette.entry
	}
}

func chartPatternLabel(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	if name == "" {
		return "Formasyon"
	}
	return name
}

func drawMovingAverage(img *image.RGBA, layout chartLayout, palette chartPalette, candles []ohlcv.Candle, toX func(int) int, toY func(float64) int, period int, lineColor color.Color) {
	if len(candles) < period {
		return
	}
	values := movingAverage(candles, period)
	lastX, lastY := 0, 0
	hasLast := false
	for i, value := range values {
		if value <= 0 {
			hasLast = false
			continue
		}
		x := toX(i)
		y := toY(value)
		if hasLast {
			drawLineWidth(img, lastX, lastY, x, y, 3, lineColor)
		}
		lastX, lastY = x, y
		hasLast = true
	}
}

func drawLevels(img *image.RGBA, layout chartLayout, palette chartPalette, levels []ohlcv.SupportResistanceLevel, toY func(float64) int) {
	for _, level := range levels {
		if level.Price <= 0 {
			continue
		}
		lineColor := palette.support
		if level.Type == "resistance" {
			lineColor = palette.resistance
		}
		y := toY(level.Price)
		if y < layout.top || y > layout.priceBottom {
			continue
		}
		drawLevelGuideLine(img, layout.left, layout.left+layout.chartWidth, y, lineColor)
	}
}

func normalizeLevelsByPrice(levels []ohlcv.SupportResistanceLevel, current float64) []ohlcv.SupportResistanceLevel {
	if current <= 0 {
		return levels
	}
	out := make([]ohlcv.SupportResistanceLevel, 0, len(levels))
	buffer := math.Max(math.Abs(current)*0.001, 0.01)
	for _, level := range levels {
		if level.Price <= 0 {
			continue
		}
		if level.Price > current+buffer {
			level.Type = "resistance"
		} else if level.Price < current-buffer {
			level.Type = "support"
		}
		out = append(out, level)
	}
	return out
}

func drawLevelGuideLine(img *image.RGBA, x0, x1, y int, accent color.Color) {
	drawLineWidth(img, x0, y, x1, y, 1, levelBaseLineColor(accent))
	drawDashedLineWidth(img, x0, y, x1, y, 26, 18, 1, levelLineColor(accent))
}

func drawGeometryOverlays(img *image.RGBA, fonts fontSet, layout chartLayout, input RenderInput, toX func(int) int, toY func(float64) int, projectionSlots int) {
	if len(input.Drawings.Lines) == 0 && len(input.Drawings.Paths) == 0 &&
		len(input.Drawings.Fills) == 0 && len(input.Drawings.TouchPoints) == 0 {
		return
	}
	index := candleTimeIndex(input.Candles)
	slotDuration := projectionSlotDuration(input.Timeframe, input.Candles)

	// Draw channel fills first (behind lines)
	for _, fill := range input.Drawings.Fills {
		drawChannelFill(img, layout, index, input.Candles, toX, toY, fill)
	}

	for _, line := range input.Drawings.Lines {
		lineColor := drawingColor(line.Color)
		width := line.Width
		if width <= 0 {
			width = 2
		}
		switch line.Type {
		case "horizontal":
			continue
		case "trendline":
			startIdx, okStart := drawingTimeToIndex(index, input.Candles, line.StartTime)
			endIdx, okEnd := drawingTimeToIndex(index, input.Candles, line.EndTime)
			if !okStart || !okEnd || line.StartPrice <= 0 || line.EndPrice <= 0 {
				continue
			}
			x0 := toX(startIdx)
			y0 := toY(line.StartPrice)
			x1, y1 := projectedLineEnd(layout, input.Candles, toX, toY, startIdx, line.StartPrice, endIdx, line.EndPrice)
			drawDrawingLine(img, x0, y0, x1, y1, width, line.Style, lineColor)
		}
	}

	// Draw touch point markers on top of lines
	drawTouchMarkers(img, layout, index, input.Candles, toX, toY, input.Drawings.TouchPoints)

	for _, path := range input.Drawings.Paths {
		if len(path.Points) < 2 {
			continue
		}
		lineColor := drawingColor(path.Color)
		width := path.Width
		if width <= 0 {
			width = 2
		}
		var lastX, lastY int
		hasLast := false
		for i, point := range path.Points {
			if point.Price <= 0 {
				hasLast = false
				continue
			}
			x, ok := drawingTimeToPlotX(index, input.Candles, point.Time, toX, projectionSlots, slotDuration)
			if !ok {
				x = layout.left + layout.chartWidth
			}
			y := toY(point.Price)
			if hasLast {
				if path.Type == "scenario_path" && i == len(path.Points)-1 {
					drawArrow(img, lastX, lastY, x, y, width, lineColor)
				} else {
					drawDrawingLine(img, lastX, lastY, x, y, width, path.Style, lineColor)
				}
			}
			lastX, lastY = x, y
			hasLast = true
		}
	}
}

func projectedLineEnd(layout chartLayout, candles []ohlcv.Candle, toX func(int) int, toY func(float64) int, startIdx int, startPrice float64, endIdx int, endPrice float64) (int, int) {
	if len(candles) == 0 || endIdx <= startIdx {
		return toX(endIdx), toY(endPrice)
	}
	lastIdx := len(candles) - 1
	if endIdx < lastIdx-2 {
		return toX(endIdx), toY(endPrice)
	}
	slope := (endPrice - startPrice) / float64(endIdx-startIdx)
	projectBars := maxInt(6, len(candles)/18)
	projectedPrice := endPrice + slope*float64(projectBars)
	return layout.left + layout.chartWidth, toY(projectedPrice)
}

func drawChannelFill(img *image.RGBA, layout chartLayout, index map[string]int, candles []ohlcv.Candle, toX func(int) int, toY func(float64) int, fill formations.FillBand) {
	uStartIdx, okUS := drawingTimeToIndex(index, candles, fill.UpperStartTime)
	uEndIdx, okUE := drawingTimeToIndex(index, candles, fill.UpperEndTime)
	lStartIdx, okLS := drawingTimeToIndex(index, candles, fill.LowerStartTime)
	lEndIdx, okLE := drawingTimeToIndex(index, candles, fill.LowerEndTime)
	if !okUS || !okUE || !okLS || !okLE {
		return
	}
	if fill.UpperStartPrice <= 0 || fill.UpperEndPrice <= 0 || fill.LowerStartPrice <= 0 || fill.LowerEndPrice <= 0 {
		return
	}
	if !parallelFillLines(uStartIdx, uEndIdx, fill.UpperStartPrice, fill.UpperEndPrice, lStartIdx, lEndIdx, fill.LowerStartPrice, fill.LowerEndPrice) {
		return
	}
	// Resolve projected endpoints
	ux1, uy1 := projectedLineEnd(layout, candles, toX, toY, uStartIdx, fill.UpperStartPrice, uEndIdx, fill.UpperEndPrice)
	lx1, ly1 := projectedLineEnd(layout, candles, toX, toY, lStartIdx, fill.LowerStartPrice, lEndIdx, fill.LowerEndPrice)
	ux0 := toX(uStartIdx)
	lx0 := toX(lStartIdx)
	xStart := maxInt(ux0, lx0)
	xEnd := minInt(ux1, lx1)
	xStart = maxInt(xStart, layout.left)
	xEnd = minInt(xEnd, layout.left+layout.chartWidth)
	if xEnd <= xStart {
		return
	}

	// Parse fill color
	fillColor := drawingColor(fill.Color)
	fr, fg, fb, _ := fillColor.RGBA()
	fa := fill.Opacity

	// Upper line: y = uy0 + (uy1-uy0)*(x-ux0)/(ux1-ux0)
	uy0 := toY(fill.UpperStartPrice)
	ly0 := toY(fill.LowerStartPrice)

	for x := xStart; x <= xEnd; x++ {
		// Interpolate upper and lower y for this x
		upperY := interpolateY(ux0, uy0, ux1, uy1, x)
		lowerY := interpolateY(lx0, ly0, lx1, ly1, x)
		yMin := minInt(upperY, lowerY)
		yMax := maxInt(upperY, lowerY)
		yMin = maxInt(yMin, layout.top)
		yMax = minInt(yMax, layout.top+layout.priceHeight)
		for y := yMin; y <= yMax; y++ {
			// Alpha-blend with existing pixel
			existing := img.RGBAAt(x, y)
			alpha := float64(fa) / 255.0
			nr := uint8(float64(fr>>8)*alpha + float64(existing.R)*(1-alpha))
			ng := uint8(float64(fg>>8)*alpha + float64(existing.G)*(1-alpha))
			nb := uint8(float64(fb>>8)*alpha + float64(existing.B)*(1-alpha))
			img.SetRGBA(x, y, color.RGBA{R: nr, G: ng, B: nb, A: 255})
		}
	}
}

func parallelFillLines(uStartIdx, uEndIdx int, uStartPrice, uEndPrice float64, lStartIdx, lEndIdx int, lStartPrice, lEndPrice float64) bool {
	uSpan := uEndIdx - uStartIdx
	lSpan := lEndIdx - lStartIdx
	if uSpan <= 0 || lSpan <= 0 {
		return false
	}
	upperSlope := (uEndPrice - uStartPrice) / float64(uSpan)
	lowerSlope := (lEndPrice - lStartPrice) / float64(lSpan)
	denom := math.Max(math.Max(math.Abs(upperSlope), math.Abs(lowerSlope)), math.Max(uEndPrice, lEndPrice)*0.00002)
	return math.Abs(upperSlope-lowerSlope)/denom <= 0.35
}

func interpolateY(x0, y0, x1, y1, x int) int {
	if x1 == x0 {
		return y0
	}
	return y0 + (y1-y0)*(x-x0)/(x1-x0)
}

func drawTouchMarkers(img *image.RGBA, layout chartLayout, index map[string]int, candles []ohlcv.Candle, toX func(int) int, toY func(float64) int, points []formations.TimePrice) {
	markerColor := rgba(255, 255, 100, 200)
	borderColor := rgba(180, 140, 0, 230)
	for _, pt := range points {
		if pt.Price <= 0 {
			continue
		}
		idx, ok := drawingTimeToIndex(index, candles, pt.Time)
		if !ok {
			continue
		}
		cx := toX(idx)
		cy := toY(pt.Price)
		if cx < layout.left || cx > layout.left+layout.chartWidth {
			continue
		}
		if cy < layout.top || cy > layout.top+layout.priceHeight {
			continue
		}
		// Draw filled circle (radius 5)
		r := 5
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r {
					setPixel(img, cx+dx, cy+dy, markerColor)
				}
			}
		}
		// Draw circle border
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				dist := dx*dx + dy*dy
				if dist >= (r-1)*(r-1) && dist <= r*r {
					setPixel(img, cx+dx, cy+dy, borderColor)
				}
			}
		}
	}
}

func drawDrawingLine(img *image.RGBA, x0, y0, x1, y1, width int, style string, c color.Color) {
	if style == "dashed" {
		drawDashedLineWidth(img, x0, y0, x1, y1, 22, 14, width, c)
		return
	}
	drawLineWidth(img, x0, y0, x1, y1, width, c)
}

func drawGeometryBadge(img *image.RGBA, face font.Face, x, y int, label string, price float64, accent color.Color) {
	text := fmt.Sprintf("%s %s", truncatePlain(label, 18), formatPrice(price))
	width := font.MeasureString(face, text).Ceil() + 18
	rect := image.Rect(x, y-16, x+width, y+8)
	fillRoundedRect(img, rect, 5, rgb(28, 34, 55))
	drawRectOutline(img, rect, accent)
	drawText(img, face, rect.Min.X+8, y+1, accent, text)
}

func drawingColor(name string) color.Color {
	// Support hex colors like "#2196F3"
	if len(name) == 7 && name[0] == '#' {
		var r, g, b uint8
		fmt.Sscanf(name[1:3], "%02x", &r)
		fmt.Sscanf(name[3:5], "%02x", &g)
		fmt.Sscanf(name[5:7], "%02x", &b)
		return rgb(r, g, b)
	}
	switch name {
	case "yellow":
		return rgb(242, 203, 34)
	case "red":
		return rgb(226, 48, 72)
	case "cyan":
		return rgb(33, 178, 190)
	case "green":
		return rgb(0, 168, 104)
	case "blue":
		return rgb(44, 104, 170)
	case "orange":
		return rgb(255, 152, 0)
	case "purple":
		return rgb(156, 39, 176)
	case "gray":
		return rgb(158, 158, 158)
	default:
		return rgb(236, 199, 46)
	}
}

func hasTrendlineDrawings(input RenderInput) bool {
	for _, line := range input.Drawings.Lines {
		if line.Type == "trendline" && line.StartPrice > 0 && line.EndPrice > 0 {
			return true
		}
	}
	return false
}

func candleTimeIndex(candles []ohlcv.Candle) map[string]int {
	out := make(map[string]int, len(candles)*3)
	for i, candle := range candles {
		if candle.Time.IsZero() {
			continue
		}
		out[chartCandleDate(candle.Time)] = i
		out[candle.Time.Format("2006-01-02")] = i
		out[candle.Time.Format(time.RFC3339)] = i
	}
	return out
}

func chartCandleDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(chartDateLocation).Format("2006-01-02")
}

func chartDateLabel(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(chartDateLocation).Format("02.01.2006")
}

func drawingTimeToIndex(index map[string]int, candles []ohlcv.Candle, value string) (int, bool) {
	if idx, ok := index[value]; ok {
		return idx, true
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, value)
	}
	if err != nil {
		return 0, false
	}
	bestIdx := 0
	bestDistance := time.Duration(1<<63 - 1)
	for i, candle := range candles {
		if candle.Time.IsZero() {
			continue
		}
		distance := candle.Time.Sub(parsed)
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestIdx = i
			bestDistance = distance
		}
	}
	return bestIdx, len(candles) > 0
}

func drawingTimeToPlotX(index map[string]int, candles []ohlcv.Candle, value string, toX func(int) int, projectionSlots int, slotDuration time.Duration) (int, bool) {
	if idx, ok := index[value]; ok {
		return toX(idx), true
	}
	parsed, err := parseDrawingTime(value)
	if err != nil || len(candles) == 0 {
		return 0, false
	}
	last := candles[len(candles)-1].Time
	if !last.IsZero() && parsed.After(last) && projectionSlots > 0 {
		if slotDuration <= 0 {
			slotDuration = 24 * time.Hour
		}
		slotsAfterLast := int(math.Ceil(float64(parsed.Sub(last)) / float64(slotDuration)))
		slotsAfterLast = clampInt(slotsAfterLast, 1, projectionSlots)
		return toX(len(candles) - 1 + slotsAfterLast), true
	}
	idx, ok := drawingTimeToIndex(index, candles, value)
	if !ok {
		return 0, false
	}
	return toX(idx), true
}

func futureProjectionSlots(candles []ohlcv.Candle, drawings formations.DrawingObjects, timeframe string) int {
	if len(candles) == 0 || len(drawings.Paths) == 0 {
		return 0
	}
	last := candles[len(candles)-1].Time
	if last.IsZero() {
		return 0
	}
	slotDuration := projectionSlotDuration(timeframe, candles)
	maxSlots := 0
	for _, path := range drawings.Paths {
		for _, point := range path.Points {
			parsed, err := parseDrawingTime(point.Time)
			if err != nil || !parsed.After(last) {
				continue
			}
			slots := int(math.Ceil(float64(parsed.Sub(last)) / float64(slotDuration)))
			maxSlots = maxInt(maxSlots, slots)
		}
	}
	if maxSlots <= 0 {
		return 0
	}
	maxSlots = maxInt(maxSlots, minFutureProjectionSlots(timeframe))
	return minInt(maxSlots, maxFutureProjectionSlots(len(candles)))
}

func minFutureProjectionSlots(timeframe string) int {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1D", "D":
		return 24
	case "1W", "W":
		return 12
	case "1M", "M":
		return 8
	default:
		return 18
	}
}

func maxFutureProjectionSlots(candleCount int) int {
	if candleCount <= 0 {
		return 24
	}
	return clampInt(candleCount/5, 24, 72)
}

func projectionSlotDuration(timeframe string, candles []ohlcv.Candle) time.Duration {
	switch strings.ToUpper(strings.TrimSpace(timeframe)) {
	case "1D", "D":
		return 24 * time.Hour
	case "1W", "W":
		return 7 * 24 * time.Hour
	case "1M", "M":
		return 30 * 24 * time.Hour
	}
	if len(candles) >= 2 {
		last := candles[len(candles)-1].Time
		prev := candles[len(candles)-2].Time
		if !last.IsZero() && !prev.IsZero() && last.After(prev) {
			gap := last.Sub(prev)
			if gap > 0 && gap < 45*24*time.Hour {
				return gap
			}
		}
	}
	return 24 * time.Hour
}

func parseDrawingTime(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}

func truncatePlain(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func drawLevelCallouts(img *image.RGBA, fonts fontSet, layout chartLayout, palette chartPalette, candles []ohlcv.Candle, levels []ohlcv.SupportResistanceLevel, toX func(int) int, toY func(float64) int) {
	placements := make([]levelCalloutPlacement, 0, len(levels))
	for _, level := range levels {
		if level.Price <= 0 {
			continue
		}
		y := toY(level.Price)
		if y < layout.top || y > layout.priceBottom {
			continue
		}
		target := levelTouchAnchor(candles, level, toX, toY)
		switch level.Type {
		case "support":
			placements = append(placements, levelCalloutPlacement{
				Level:      level,
				Label:      "Destek",
				Target:     target,
				Accent:     palette.support,
				Surface:    palette.supportSurface,
				PreferredX: levelCalloutX(layout, target.X),
				PreferredY: target.Y,
			})
		case "resistance":
			placements = append(placements, levelCalloutPlacement{
				Level:      level,
				Label:      "Direnç",
				Target:     target,
				Accent:     palette.resistance,
				Surface:    palette.resistanceSurface,
				PreferredX: levelCalloutX(layout, target.X),
				PreferredY: target.Y,
			})
		}
	}
	sort.SliceStable(placements, func(i, j int) bool {
		if placements[i].PreferredY == placements[j].PreferredY {
			return placements[i].Level.Price > placements[j].Level.Price
		}
		return placements[i].PreferredY < placements[j].PreferredY
	})
	occupied := make([]image.Rectangle, 0, len(placements))
	for _, placement := range placements {
		rect := placeLevelCallout(layout, placement, occupied)
		occupied = append(occupied, rect)
		drawLevelCalloutRect(img, fonts.small, rect, placement.Target, placement.Label, placement.Level.Price, placement.Accent, placement.Surface)
	}
}

const levelCalloutWidth = 162
const levelCalloutHeight = 27
const levelCalloutGap = 8

type levelCalloutPlacement struct {
	Level      ohlcv.SupportResistanceLevel
	Label      string
	Target     image.Point
	Accent     color.Color
	Surface    color.Color
	PreferredX int
	PreferredY int
}

func levelCalloutX(layout chartLayout, targetX int) int {
	const gap = 28
	leftBound := layout.left + 12
	rightBound := layout.left + layout.chartWidth - 12
	chartMiddle := layout.left + layout.chartWidth/2
	if targetX < chartMiddle {
		if targetX+gap+levelCalloutWidth <= rightBound {
			return targetX + gap
		}
		return maxInt(leftBound, targetX-gap-levelCalloutWidth)
	}
	if targetX-gap-levelCalloutWidth >= leftBound {
		return targetX - gap - levelCalloutWidth
	}
	return minInt(rightBound-levelCalloutWidth, targetX+gap)
}

func placeLevelCallout(layout chartLayout, placement levelCalloutPlacement, occupied []image.Rectangle) image.Rectangle {
	xCandidates := levelCalloutXCandidates(layout, placement.Target.X, placement.PreferredX)
	yCandidates := levelCalloutYCandidates(layout, placement.PreferredY)
	best := image.Rectangle{}
	bestScore := math.MaxFloat64
	for _, y := range yCandidates {
		for _, x := range xCandidates {
			rect := levelCalloutRect(x, y)
			if rect.Min.X < layout.left+8 || rect.Max.X > layout.left+layout.chartWidth-8 {
				continue
			}
			if rect.Min.Y < layout.top+8 || rect.Max.Y > layout.priceBottom-8 {
				continue
			}
			score := levelCalloutScore(rect, placement)
			if overlapsAny(rect, occupied, levelCalloutGap) {
				score += 100000 + overlapPenalty(rect, occupied)
			}
			if score < bestScore {
				best = rect
				bestScore = score
			}
			if bestScore < 100000 && !overlapsAny(best, occupied, levelCalloutGap) {
				return best
			}
		}
	}
	if best.Empty() {
		return levelCalloutRect(placement.PreferredX, clampInt(placement.PreferredY, layout.top+levelCalloutHeight, layout.priceBottom-levelCalloutHeight))
	}
	return best
}

func levelCalloutXCandidates(layout chartLayout, targetX, preferredX int) []int {
	leftBound := layout.left + 12
	rightBound := layout.left + layout.chartWidth - 12 - levelCalloutWidth
	centerLeft := layout.left + layout.chartWidth/3 - levelCalloutWidth/2
	centerRight := layout.left + 2*layout.chartWidth/3 - levelCalloutWidth/2
	candidates := []int{
		preferredX,
		clampInt(targetX+34, leftBound, rightBound),
		clampInt(targetX-34-levelCalloutWidth, leftBound, rightBound),
		clampInt(centerLeft, leftBound, rightBound),
		clampInt(centerRight, leftBound, rightBound),
		leftBound,
		rightBound,
	}
	return uniqueInts(candidates)
}

func levelCalloutYCandidates(layout chartLayout, preferredY int) []int {
	minY := layout.top + levelCalloutHeight/2 + 10
	maxY := layout.priceBottom - levelCalloutHeight/2 - 10
	step := levelCalloutHeight + levelCalloutGap
	candidates := []int{clampInt(preferredY, minY, maxY)}
	maxSteps := maxInt(1, (maxY-minY)/step+1)
	for distance := 1; distance <= maxSteps; distance++ {
		offset := distance * step
		candidates = append(candidates,
			clampInt(preferredY-offset, minY, maxY),
			clampInt(preferredY+offset, minY, maxY),
		)
	}
	return uniqueInts(candidates)
}

func levelCalloutRect(x, centerY int) image.Rectangle {
	return image.Rect(x, centerY-levelCalloutHeight/2, x+levelCalloutWidth, centerY+levelCalloutHeight/2)
}

func levelCalloutScore(rect image.Rectangle, placement levelCalloutPlacement) float64 {
	centerX := rect.Min.X + rect.Dx()/2
	centerY := rect.Min.Y + rect.Dy()/2
	return math.Abs(float64(centerY-placement.PreferredY))*2 +
		math.Abs(float64(rect.Min.X-placement.PreferredX))*0.35 +
		math.Hypot(float64(centerX-placement.Target.X), float64(centerY-placement.Target.Y))*0.05
}

func overlapsAny(rect image.Rectangle, occupied []image.Rectangle, gap int) bool {
	for _, other := range occupied {
		if expandedRect(rect, gap).Overlaps(expandedRect(other, gap)) {
			return true
		}
	}
	return false
}

func overlapPenalty(rect image.Rectangle, occupied []image.Rectangle) float64 {
	penalty := 0.0
	for _, other := range occupied {
		intersection := rect.Intersect(other)
		if !intersection.Empty() {
			penalty += float64(intersection.Dx()*intersection.Dy()) + 5000
		}
	}
	return penalty
}

func expandedRect(rect image.Rectangle, gap int) image.Rectangle {
	return image.Rect(rect.Min.X-gap, rect.Min.Y-gap, rect.Max.X+gap, rect.Max.Y+gap)
}

func uniqueInts(values []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func drawLevelCalloutRect(img *image.RGBA, face font.Face, rect image.Rectangle, target image.Point, label string, price float64, accent, surface color.Color) {
	text := fmt.Sprintf("%s %s", label, formatPrice(price))
	textColor := levelTextColor(accent)
	drawRoundedBadge(img, rect, 6, levelBorderColor(accent), surface)
	labelY := rect.Min.Y + rect.Dy()/2
	drawText(img, face, rect.Min.X+10, labelY+6, textColor, text)

	startX := rect.Max.X
	if target.X < rect.Min.X+levelCalloutWidth/2 {
		startX = rect.Min.X
	}
	drawArrow(img, startX, labelY, target.X, target.Y, 1, textColor)
}

func levelTouchAnchor(candles []ohlcv.Candle, level ohlcv.SupportResistanceLevel, toX func(int) int, toY func(float64) int) image.Point {
	index := levelTouchIndex(candles, level)
	if index < 0 {
		return image.Pt(0, toY(level.Price))
	}
	return image.Pt(toX(index), toY(levelTouchPrice(candles[index], level)))
}

func levelTouchIndex(candles []ohlcv.Candle, level ohlcv.SupportResistanceLevel) int {
	if len(candles) == 0 {
		return -1
	}
	if !level.LastTouchedAt.IsZero() {
		for i := len(candles) - 1; i >= 0; i-- {
			if candles[i].Time.Equal(level.LastTouchedAt) {
				return i
			}
		}
	}
	selected := 0
	bestDistance := math.MaxFloat64
	for i, candle := range candles {
		distance := math.Abs(levelTouchPrice(candle, level) - level.Price)
		if distance <= bestDistance {
			selected = i
			bestDistance = distance
		}
	}
	return selected
}

func levelTouchPrice(candle ohlcv.Candle, level ohlcv.SupportResistanceLevel) float64 {
	if level.Type == "resistance" {
		return candle.EffectiveHigh()
	}
	return candle.EffectiveLow()
}

func levelTextColor(accent color.Color) color.Color {
	if isResistanceAccent(accent) {
		return rgb(239, 83, 80)
	}
	return rgb(38, 166, 154)
}

func levelBorderColor(accent color.Color) color.Color {
	return blendOnDark(levelTextColor(accent), 0.80)
}

func levelLineColor(accent color.Color) color.Color {
	return blendOnDark(accent, 0.65)
}

func levelBaseLineColor(accent color.Color) color.Color {
	return blendOnDark(accent, 0.30)
}

func isResistanceAccent(accent color.Color) bool {
	r, g, b, _ := accent.RGBA()
	rr := uint8(r >> 8)
	gg := uint8(g >> 8)
	_ = b
	return rr > 180 && gg < 120
}

func blendOnDark(c color.Color, alpha float64) color.Color {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: uint8(math.Round(13*(1-alpha) + float64(uint8(r>>8))*alpha)),
		G: uint8(math.Round(17*(1-alpha) + float64(uint8(g>>8))*alpha)),
		B: uint8(math.Round(28*(1-alpha) + float64(uint8(b>>8))*alpha)),
		A: 255,
	}
}

func blendOnWhite(c color.Color, alpha float64) color.Color {
	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: blendChannelOnWhite(uint8(r>>8), alpha),
		G: blendChannelOnWhite(uint8(g>>8), alpha),
		B: blendChannelOnWhite(uint8(b>>8), alpha),
		A: 255,
	}
}

func blendChannelOnWhite(value uint8, alpha float64) uint8 {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	blended := 255*(1-alpha) + float64(value)*alpha
	return uint8(math.Round(blended))
}

func drawTradePlan(img *image.RGBA, layout chartLayout, palette chartPalette, plan ohlcv.TradePlan, toY func(float64) int) {
	lines := []struct {
		value float64
		label string
		color color.Color
	}{
		{plan.EntryMin, "Giriş Alt", palette.entry},
		{plan.EntryMax, "Giriş Üst", palette.entry},
		{plan.TakeProfit1, "Kar Al 1", palette.target},
		{plan.TakeProfit2, "Kar Al 2", palette.target},
		{plan.StopLoss, "Zarar Kes", palette.stop},
	}
	for _, item := range lines {
		if item.value <= 0 {
			continue
		}
		y := toY(item.value)
		if y < layout.top || y > layout.priceBottom {
			continue
		}
		drawDashedLineWidth(img, layout.left, y, layout.left+layout.chartWidth, y, 10, 14, 1, tradePlanLineColor(item.color))
	}
}

func activeTradePlan(plan ohlcv.TradePlan) bool {
	return !plan.Rejected && plan.Direction != "" && plan.Direction != "neutral"
}

func tradePlanLineColor(c color.Color) color.Color {
	return blendOnDark(c, 0.72)
}

func drawLastClose(img *image.RGBA, layout chartLayout, palette chartPalette, price float64, toY func(float64) int) {
	y := toY(price)
	drawLineWidth(img, layout.left, y, layout.left+layout.chartWidth, y, 2, blendOnDark(palette.axis, 0.70))
}

func drawCurrentPriceMarker(img *image.RGBA, fonts fontSet, layout chartLayout, palette chartPalette, price float64, marker color.Color, toY func(float64) int, anchorX int) {
	y := toY(price)
	drawDashedLineWidth(img, layout.left, y, layout.left+layout.chartWidth, y, 5, 9, 1, blendOnDark(marker, 0.82))
	text := "Güncel " + formatPrice(price)
	width := font.MeasureString(fonts.small, text).Ceil() + 24
	x := priceAxisLabelX(layout)
	rect := image.Rect(x, y-17, x+width, y+17)
	fillRect(img, rect, marker)
	drawRectOutline(img, rect, withAlpha(rgb(0, 0, 0), 90))
	drawText(img, fonts.small, x+11, y+6, rgb(255, 255, 255), text)
	fillRect(img, image.Rect(layout.left+layout.chartWidth-4, y-5, layout.left+layout.chartWidth+5, y+5), marker)
}

func currentPriceColor(palette chartPalette, candles []ohlcv.Candle) color.Color {
	if len(candles) == 0 {
		return palette.entry
	}
	candle := candles[len(candles)-1]
	closePrice := candle.EffectiveClose()
	comparePrice := candle.EffectiveOpen()
	if len(candles) >= 2 {
		comparePrice = candles[len(candles)-2].EffectiveClose()
	}
	switch {
	case closePrice < comparePrice:
		return palette.stop
	case closePrice > comparePrice:
		return palette.support
	default:
		return palette.entry
	}
}

func drawLegend(img *image.RGBA, fonts fontSet, layout chartLayout, palette chartPalette, geometryMode bool) {
	x := layout.left
	y := 130
	type legendItem struct {
		label string
		color color.Color
	}
	items := []legendItem{}
	if geometryMode {
		items = []legendItem{
			{"Destek trend", drawingColor("yellow")},
			{"Direnç trend", drawingColor("red")},
			{"Mavi senaryo", drawingColor("cyan")},
		}
	} else {
		items = []legendItem{
			{"HO20", palette.ma20},
			{"HO50", palette.ma50},
			{"HO200", palette.ma200},
			{"Destek", palette.support},
			{"Direnç", palette.resistance},
		}
	}
	for _, item := range items {
		fillRect(img, image.Rect(x, y-11, x+18, y-2), item.color)
		drawText(img, fonts.small, x+24, y, palette.muted, item.label)
		x += font.MeasureString(fonts.small, item.label).Ceil() + 54
	}
}

func drawInfoPanel(img *image.RGBA, fonts fontSet, layout chartLayout, palette chartPalette, input RenderInput) {
	x := layout.left + layout.chartWidth + 170
	y := layout.top
	w := layout.rightPanel - 40
	panelRight := x + w

	fillRect(img, image.Rect(x, y, panelRight, layout.volumeBottom), palette.panel)
	drawRectOutline(img, image.Rect(x, y, panelRight, layout.volumeBottom), palette.grid)

	last := input.Candles[len(input.Candles)-1]
	price := last.EffectiveClose()
	priceColor := palette.up
	changeSign := "▲"
	var changeStr string
	if len(input.Candles) >= 2 {
		prev := input.Candles[len(input.Candles)-2].EffectiveClose()
		if prev > 0 {
			pct := (price - prev) / prev * 100
			if pct < 0 {
				priceColor = palette.down
				changeSign = "▼"
				changeStr = fmt.Sprintf("%s %.2f%%", changeSign, math.Abs(pct))
			} else {
				changeStr = fmt.Sprintf("%s +%.2f%%", changeSign, pct)
			}
		}
	}

	cursor := y + 18
	// Hero price block
	drawText(img, fonts.title, x+16, cursor+46, priceColor, fmt.Sprintf("%.2f", price))
	if changeStr != "" {
		drawText(img, fonts.body, x+16, cursor+72, priceColor, changeStr)
	}
	cursor += 98

	// ── Teknik Göstergeler ──
	cursor = drawPanelSection(img, fonts, x, cursor, panelRight, palette, "Teknik Göstergeler")
	rsi := input.Indicators.RSI14
	rsiColor := palette.muted
	if rsi >= 70 {
		rsiColor = palette.down
	} else if rsi <= 30 {
		rsiColor = palette.up
	} else if rsi > 55 {
		rsiColor = palette.up
	}
	drawPanelLabelValue(img, fonts, x+16, cursor, w-32, palette, "RSI 14", fmt.Sprintf("%.1f", rsi), rsiColor)
	drawRSIMiniBar(img, x+16, cursor+6, w-108, rsi, palette)
	cursor += 36
	drawPanelLabelValue(img, fonts, x+16, cursor, w-32, palette, "ATR 14", fmt.Sprintf("%.2f", input.Indicators.ATR14), palette.muted)
	cursor += 32

	// ── İşlem Planı ──
	cursor = drawPanelSection(img, fonts, x, cursor+4, panelRight, palette, "İşlem Planı")
	if !activeTradePlan(input.TradePlan) {
		if input.TradePlan.Rejected && input.TradePlan.Direction != "neutral" && input.TradePlan.RiskRewardRatio > 0 {
			cursor = drawWrappedText(img, fonts.small, x+16, cursor, w-32, palette.muted,
				fmt.Sprintf("Reddedilen taslak: %s | RR %.2f", localize.Direction(input.TradePlan.Direction), input.TradePlan.RiskRewardRatio)) + 10
		} else {
			drawText(img, fonts.small, x+16, cursor, palette.muted, "Aktif işlem planı yok.")
			cursor += 28
		}
	} else {
		dirColor := palette.up
		if input.TradePlan.Direction == "short" {
			dirColor = palette.down
		}
		dirLabel := localize.Direction(input.TradePlan.Direction)
		bw := font.MeasureString(fonts.bold, dirLabel).Ceil() + 28
		fillRoundedRect(img, image.Rect(x+16, cursor-18, x+16+bw, cursor+8), 5, blendOnDark(dirColor, 0.30))
		drawRectOutline(img, image.Rect(x+16, cursor-18, x+16+bw, cursor+8), blendOnDark(dirColor, 0.65))
		drawText(img, fonts.bold, x+16+14, cursor, dirColor, dirLabel)
		qualLabel := localize.Quality(input.TradePlan.Quality)
		drawTextRight(img, fonts.small, panelRight-16, cursor, palette.muted, qualLabel)
		cursor += 32

		if input.TradePlan.RiskRewardRatio > 0 {
			drawPanelLabelValue(img, fonts, x+16, cursor, w-32, palette, "Risk/Getiri", fmt.Sprintf("%.2f", input.TradePlan.RiskRewardRatio), palette.text)
			cursor += 26
		}
		tradeRows := []struct {
			label string
			value float64
			col   color.Color
		}{
			{"Giriş alt", input.TradePlan.EntryMin, palette.entry},
			{"Giriş üst", input.TradePlan.EntryMax, palette.entry},
			{"Zarar kes", input.TradePlan.StopLoss, palette.stop},
			{"Kar al 1", input.TradePlan.TakeProfit1, palette.target},
			{"Kar al 2", input.TradePlan.TakeProfit2, palette.target},
		}
		for _, row := range tradeRows {
			if row.value <= 0 {
				continue
			}
			drawPanelMetric(img, fonts, x+16, cursor, w-32, row.label, row.value, row.col, palette)
			cursor += 26
		}
		if input.TradePlan.RejectReason != "" {
			cursor += 4
			cursor = drawWrappedText(img, fonts.small, x+16, cursor, w-32, palette.stop, tradePlanRejectDisplay(input.TradePlan)) + 6
		}
	}

	if hasDrawableSummary(input.Drawings) {
		cursor = drawPanelSection(img, fonts, x, cursor+4, panelRight, palette, "Teknik Çizim")
		cursor = drawGeometryPanel(img, fonts, x+16, cursor, w-32, input.Drawings, palette)
	}

	cursor = drawPanelSection(img, fonts, x, cursor+4, panelRight, palette, "Destekler")
	cursor = drawPanelLevels(img, fonts, x+16, cursor, w-32, input.Levels, "support", palette.support, palette.supportSurface, palette, 3)

	cursor = drawPanelSection(img, fonts, x, cursor+4, panelRight, palette, "Dirençler")
	cursor = drawPanelLevels(img, fonts, x+16, cursor, w-32, input.Levels, "resistance", palette.resistance, palette.resistanceSurface, palette, 3)
	_ = cursor
}

func drawPanelSection(img *image.RGBA, fonts fontSet, x, y, rightEdge int, palette chartPalette, title string) int {
	lineY := y + 8
	drawLineWidth(img, x+8, lineY, rightEdge-8, lineY, 1, palette.grid)
	tw := font.MeasureString(fonts.small, title).Ceil()
	labelX := x + (rightEdge-x-tw)/2
	fillRect(img, image.Rect(labelX-6, lineY-10, labelX+tw+6, lineY+10), palette.panel)
	drawText(img, fonts.small, labelX, lineY+5, palette.muted, title)
	return y + 34
}

func drawPanelLabelValue(img *image.RGBA, fonts fontSet, x, y, width int, palette chartPalette, label, value string, valueColor color.Color) {
	drawText(img, fonts.small, x, y, palette.muted, label)
	drawTextRight(img, fonts.small, x+width, y, valueColor, value)
}

func drawRSIMiniBar(img *image.RGBA, x, y, barWidth int, rsi float64, palette chartPalette) {
	if barWidth < 20 {
		return
	}
	filled := int(math.Round(float64(barWidth) * rsi / 100.0))
	filled = maxInt(0, minInt(filled, barWidth))
	barColor := palette.up
	if rsi >= 70 {
		barColor = palette.down
	} else if rsi <= 30 {
		barColor = palette.up
	}
	fillRect(img, image.Rect(x, y+2, x+barWidth, y+6), blendOnDark(palette.grid, 0.80))
	if filled > 0 {
		fillRect(img, image.Rect(x, y+2, x+filled, y+6), blendOnDark(barColor, 0.80))
	}
}

func drawPanelMetric(img *image.RGBA, fonts fontSet, x, y, width int, label string, value float64, accent color.Color, palette chartPalette) {
	fillRect(img, image.Rect(x, y-16, x+4, y+4), accent)
	drawText(img, fonts.small, x+12, y, palette.muted, label)
	valueText := fmt.Sprintf("%.2f", value)
	drawTextRight(img, fonts.small, x+width, y, accent, valueText)
}

func hasDrawableSummary(drawings formations.DrawingObjects) bool {
	return len(drawings.Lines) > 0 || len(drawings.Paths) > 0 || len(drawings.Fills) > 0 || len(drawings.TouchPoints) > 0
}

func drawGeometryPanel(img *image.RGBA, fonts fontSet, x, y, width int, drawings formations.DrawingObjects, palette chartPalette) int {
	shown := 0
	for _, line := range drawings.Lines {
		if line.Type != "trendline" || line.StartPrice <= 0 || line.EndPrice <= 0 {
			continue
		}
		accent := drawingColor(line.Color)
		y = drawGeometryInfoRow(img, fonts, x, y, width, geometryLineDescription(line), formatPrice(line.EndPrice), accent, blendOnDark(accent, 0.22), palette)
		shown++
		if shown >= 4 {
			break
		}
	}
	pathShown := 0
	for _, path := range drawings.Paths {
		if len(path.Points) < 2 {
			continue
		}
		accent := drawingColor(path.Color)
		y = drawGeometryInfoRow(img, fonts, x, y, width, geometryPathDescription(path), geometryPathValue(path), accent, blendOnDark(accent, 0.18), palette)
		shown++
		pathShown++
		if pathShown >= 3 {
			break
		}
	}
	if len(drawings.TouchPoints) > 0 {
		y = drawGeometryInfoRow(img, fonts, x, y, width, "Temas noktaları", fmt.Sprintf("%d", len(drawings.TouchPoints)), rgb(242, 203, 34), blendOnDark(rgb(242, 203, 34), 0.16), palette)
		shown++
	}
	if shown == 0 {
		drawText(img, fonts.small, x, y, palette.muted, "Çizim yok")
		y += 27
	}
	return y
}

func drawGeometryInfoRow(img *image.RGBA, fonts fontSet, x, y, width int, label, value string, accent, surface color.Color, palette chartPalette) int {
	row := image.Rect(x-6, y-20, x+width+6, y+8)
	fillRoundedRect(img, row, 5, surface)
	fillRoundedRect(img, image.Rect(x, y-17, x+8, y+5), 3, accent)
	valueWidth := font.MeasureString(fonts.small, value).Ceil()
	labelWidth := maxInt(40, width-valueWidth-34)
	drawText(img, fonts.small, x+16, y, palette.muted, truncateText(fonts.small, label, labelWidth))
	drawTextRight(img, fonts.small, x+width, y, accent, value)
	return y + 30
}

func geometryLineDescription(line formations.LineObject) string {
	label := strings.TrimSpace(line.Label)
	if label == "" {
		label = strings.TrimSpace(line.ID)
	}
	label = localizedGeometryLabel(label)
	label = strings.ReplaceAll(label, "kanali", "kanalı")
	label = strings.ReplaceAll(label, "Kisa", "Kısa")
	label = strings.ReplaceAll(label, "dusen", "düşen")
	label = strings.ReplaceAll(label, "ust", "üst")
	label = strings.ReplaceAll(label, "bandi", "bandı")
	label = strings.ReplaceAll(label, "destegi", "desteği")
	if label == "" {
		return "Trend çizgisi"
	}
	return truncatePlain(label, 30)
}

func geometryPathDescription(path formations.PathObject) string {
	label := strings.TrimSpace(path.Label)
	if label == "" {
		label = strings.TrimSpace(path.ID)
	}
	label = localizedGeometryPathLabel(label)
	if label == "" {
		return "Senaryo oku"
	}
	return truncatePlain(label, 34)
}

func geometryPathValue(path formations.PathObject) string {
	if len(path.Points) == 0 {
		return ""
	}
	start := path.Points[0].Price
	end := path.Points[len(path.Points)-1].Price
	if start <= 0 || end <= 0 {
		return fmt.Sprintf("%d nokta", len(path.Points))
	}
	return fmt.Sprintf("%.0f -> %.0f", start, end)
}

func localizedGeometryPathLabel(label string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(label), " ", "_"))
	switch {
	case strings.Contains(normalized, "bullish_support_reaction"):
		return "Mavi ok: destekten tepki"
	case strings.Contains(normalized, "bullish_breakout"):
		return "Mavi ok: direnç kırılımı"
	case strings.Contains(normalized, "bearish_support_loss"):
		return "Kırmızı ok: destek kaybı"
	case strings.Contains(normalized, "scenario"):
		return "Senaryo oku"
	}
	return label
}

func localizedGeometryLabel(label string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(label), " ", "_"))
	switch {
	case strings.Contains(normalized, "symmetrical_triangle") && strings.Contains(normalized, "upper"):
		return "Üçgen üst direnci"
	case strings.Contains(normalized, "symmetrical_triangle") && strings.Contains(normalized, "lower"):
		return "Üçgen alt desteği"
	case strings.Contains(normalized, "falling_wedge") && strings.Contains(normalized, "upper"):
		return "Düşen kama üst direnci"
	case strings.Contains(normalized, "falling_wedge") && strings.Contains(normalized, "lower"):
		return "Düşen kama alt çizgisi"
	case strings.Contains(normalized, "rising_wedge") && strings.Contains(normalized, "upper"):
		return "Yükselen kama üst çizgisi"
	case strings.Contains(normalized, "rising_wedge") && strings.Contains(normalized, "lower"):
		return "Yükselen kama alt desteği"
	case strings.Contains(normalized, "ascending_channel") && strings.Contains(normalized, "upper"):
		return "Kanal üst direnci"
	case strings.Contains(normalized, "ascending_channel") && strings.Contains(normalized, "lower"):
		return "Kanal alt desteği"
	case strings.Contains(normalized, "descending_channel") && strings.Contains(normalized, "upper"):
		return "Düşen kanal üst direnci"
	case strings.Contains(normalized, "descending_channel") && strings.Contains(normalized, "lower"):
		return "Düşen kanal alt desteği"
	}
	return label
}

func drawPanelLevels(img *image.RGBA, fonts fontSet, x, y, width int, levels []ohlcv.SupportResistanceLevel, kind string, accent, surface color.Color, palette chartPalette, limit int) int {
	shown := 0
	for _, level := range levels {
		if level.Type != kind || level.Price <= 0 {
			continue
		}
		fillRoundedRect(img, image.Rect(x-4, y-18, x+width+4, y+7), 4, surface)
		fillRect(img, image.Rect(x, y-16, x+4, y+4), accent)
		priceText := formatPrice(level.Price)
		pw := font.MeasureString(fonts.small, priceText).Ceil()
		drawText(img, fonts.small, x+10, y, palette.muted, panelLevelDescription(kind, level))
		drawText(img, fonts.small, x+width-pw, y, accent, priceText)
		y += 28
		shown++
		if shown >= limit {
			break
		}
	}
	if shown == 0 {
		drawText(img, fonts.small, x, y, palette.muted, "Seviye bulunamadı")
		y += 27
	}
	return y
}

func panelLevelDescription(kind string, level ohlcv.SupportResistanceLevel) string {
	label := "Pivot seviye"
	switch kind {
	case "support":
		label = "Pivot destek"
	case "resistance":
		label = "Pivot direnç"
	}
	touches := level.TouchCount
	if touches < 0 {
		touches = 0
	}
	return fmt.Sprintf("%s | %d temas | güç %.0f/100", label, touches, math.Round(level.Strength*100))
}

func movingAverage(candles []ohlcv.Candle, period int) []float64 {
	values := make([]float64, len(candles))
	sum := 0.0
	for i, candle := range candles {
		sum += candle.EffectiveClose()
		if i >= period {
			sum -= candles[i-period].EffectiveClose()
		}
		if i >= period-1 {
			values[i] = sum / float64(period)
		}
	}
	return values
}

func candleBodyWidth(chartWidth, count int) int {
	if count <= 0 {
		return 4
	}
	width := int(math.Round(float64(chartWidth) / float64(count) * 0.72))
	return maxInt(3, minInt(width, 11))
}

func volumeBarWidth(chartWidth, count int) int {
	if count <= 0 {
		return 3
	}
	width := int(math.Round(float64(chartWidth) / float64(count) * 0.58))
	return maxInt(2, minInt(width, 10))
}

func candleWickColor(c color.Color) color.Color {
	return darkenColor(c, 0.82)
}

func candleOutlineColor(c color.Color) color.Color {
	return darkenColor(c, 0.74)
}

func darkenColor(c color.Color, factor float64) color.Color {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: uint8(math.Round(float64(uint8(r>>8)) * factor)),
		G: uint8(math.Round(float64(uint8(g>>8)) * factor)),
		B: uint8(math.Round(float64(uint8(b>>8)) * factor)),
		A: 255,
	}
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.Color) {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	draw.Draw(img, rect, image.NewUniform(c), image.Point{}, draw.Src)
}

func blendRect(img *image.RGBA, rect image.Rectangle, c color.Color, alpha float64) {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	r, g, b, _ := c.RGBA()
	cr, cg, cb := float64(uint8(r>>8)), float64(uint8(g>>8)), float64(uint8(b>>8))
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			existing := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Round(cr*alpha + float64(existing.R)*(1-alpha))),
				G: uint8(math.Round(cg*alpha + float64(existing.G)*(1-alpha))),
				B: uint8(math.Round(cb*alpha + float64(existing.B)*(1-alpha))),
				A: 255,
			})
		}
	}
}

func drawRoundedBadge(img *image.RGBA, rect image.Rectangle, radius int, border, fill color.Color) {
	fillRoundedRect(img, rect, radius, border)
	inner := rect.Inset(1)
	if inner.Empty() {
		return
	}
	fillRoundedRect(img, inner, maxInt(0, radius-1), fill)
}

func fillRoundedRect(img *image.RGBA, rect image.Rectangle, radius int, c color.Color) {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	radius = minInt(radius, minInt(rect.Dx()/2, rect.Dy()/2))
	if radius <= 0 {
		fillRect(img, rect, c)
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if roundedRectContains(x, y, rect, radius) {
				img.Set(x, y, c)
			}
		}
	}
}

func roundedRectContains(x, y int, rect image.Rectangle, radius int) bool {
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

func drawRectOutline(img *image.RGBA, rect image.Rectangle, c color.Color) {
	drawLine(img, rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y, c)
	drawLine(img, rect.Max.X, rect.Min.Y, rect.Max.X, rect.Max.Y, c)
	drawLine(img, rect.Max.X, rect.Max.Y, rect.Min.X, rect.Max.Y, c)
	drawLine(img, rect.Min.X, rect.Max.Y, rect.Min.X, rect.Min.Y, c)
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	dx := absInt(x1 - x0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -absInt(y1 - y0)
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		setPixel(img, x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func drawLineWidth(img *image.RGBA, x0, y0, x1, y1, width int, c color.Color) {
	if width <= 1 {
		drawLine(img, x0, y0, x1, y1, c)
		return
	}
	radius := width / 2
	if absInt(x1-x0) >= absInt(y1-y0) {
		for offset := -radius; offset <= radius; offset++ {
			drawLine(img, x0, y0+offset, x1, y1+offset, c)
		}
		return
	}
	for offset := -radius; offset <= radius; offset++ {
		drawLine(img, x0+offset, y0, x1+offset, y1, c)
	}
}

func drawDashedLine(img *image.RGBA, x0, y0, x1, y1, dash, gap int, c color.Color) {
	drawDashedLineWidth(img, x0, y0, x1, y1, dash, gap, 1, c)
}

func drawDashedLineWidth(img *image.RGBA, x0, y0, x1, y1, dash, gap, width int, c color.Color) {
	dx := x1 - x0
	dy := y1 - y0
	length := math.Sqrt(float64(dx*dx + dy*dy))
	if length < 1 {
		return
	}
	// Walk along the line in pixel steps, alternating dash and gap
	stepX := float64(dx) / length
	stepY := float64(dy) / length
	pos := 0.0
	drawing := true
	for pos < length {
		segLen := float64(dash)
		if !drawing {
			segLen = float64(gap)
		}
		segEnd := math.Min(pos+segLen, length)
		if segEnd <= pos {
			break
		}
		if drawing {
			ax := x0 + int(math.Round(pos*stepX))
			ay := y0 + int(math.Round(pos*stepY))
			bx := x0 + int(math.Round(segEnd*stepX))
			by := y0 + int(math.Round(segEnd*stepY))
			drawLineWidth(img, ax, ay, bx, by, width, c)
		}
		pos = segEnd
		drawing = !drawing
	}
}

func drawArrow(img *image.RGBA, x0, y0, x1, y1, width int, c color.Color) {
	drawLineWidth(img, x0, y0, x1, y1, width, c)
	angle := math.Atan2(float64(y1-y0), float64(x1-x0))
	size := 9.0
	for _, delta := range []float64{math.Pi * 0.78, -math.Pi * 0.78} {
		x2 := x1 + int(math.Round(math.Cos(angle+delta)*size))
		y2 := y1 + int(math.Round(math.Sin(angle+delta)*size))
		drawLineWidth(img, x1, y1, x2, y2, width, c)
	}
}

func drawText(img *image.RGBA, face font.Face, x, y int, c color.Color, text string) {
	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
}

func drawTextCentered(img *image.RGBA, face font.Face, x, y int, c color.Color, text string) {
	width := font.MeasureString(face, text).Ceil()
	drawText(img, face, x-width/2, y, c, text)
}

func drawTextRight(img *image.RGBA, face font.Face, x, y int, c color.Color, text string) {
	width := font.MeasureString(face, text).Ceil()
	drawText(img, face, x-width, y, c, text)
}

func drawLabel(img *image.RGBA, face font.Face, x, y int, c color.Color, text string) {
	width := font.MeasureString(face, text).Ceil()
	rect := image.Rect(x-5, y-15, x+width+6, y+4)
	fillRect(img, rect, withAlpha(rgb(255, 255, 255), 225))
	drawRectOutline(img, rect, withAlpha(c, 180))
	drawText(img, face, x, y, c, text)
}

func drawLabelRight(img *image.RGBA, face font.Face, x, y int, c color.Color, text string) {
	width := font.MeasureString(face, text).Ceil()
	drawLabel(img, face, x-width, y, c, text)
}

func reserveLabelRow(desired, minY, maxY int, occupied *[]int) int {
	const spacing = 32
	candidates := []int{desired}
	for offset := spacing; offset <= 220; offset += spacing {
		candidates = append(candidates, desired-offset, desired+offset)
	}
	for _, candidate := range candidates {
		if candidate < minY {
			candidate = minY
		}
		if candidate > maxY {
			candidate = maxY
		}
		if labelRowAvailable(candidate, *occupied, spacing) {
			*occupied = append(*occupied, candidate)
			return candidate
		}
	}
	clamped := maxInt(minY, minInt(desired, maxY))
	*occupied = append(*occupied, clamped)
	return clamped
}

func labelRowAvailable(candidate int, occupied []int, spacing int) bool {
	for _, row := range occupied {
		if absInt(candidate-row) < spacing {
			return false
		}
	}
	return true
}

func drawWrappedText(img *image.RGBA, face font.Face, x, y, maxWidth int, c color.Color, text string) int {
	words := stringsFields(text)
	line := ""
	cursor := y
	for _, word := range words {
		next := stringsTrimSpace(line + " " + word)
		if line != "" && font.MeasureString(face, next).Ceil() > maxWidth {
			drawText(img, face, x, cursor, c, line)
			cursor += 17
			line = word
			continue
		}
		line = next
	}
	if line != "" {
		drawText(img, face, x, cursor, c, line)
		cursor += 17
	}
	return cursor
}

func setPixel(img *image.RGBA, x, y int, c color.Color) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return
	}
	img.Set(x, y, c)
}

func formatPrice(value float64) string {
	absValue := math.Abs(value)
	switch {
	case absValue >= 100:
		return fmt.Sprintf("%.1f", value)
	case absValue >= 1:
		return fmt.Sprintf("%.2f", value)
	case absValue >= 0.1:
		return fmt.Sprintf("%.3f", value)
	case absValue >= 0.01:
		return fmt.Sprintf("%.4f", value)
	case absValue >= 0.001:
		return fmt.Sprintf("%.5f", value)
	default:
		return fmt.Sprintf("%.6f", value)
	}
}

func rgb(r, g, b uint8) color.Color {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func rgba(r, g, b, a uint8) color.Color {
	return color.RGBA{R: r, G: g, B: b, A: a}
}

func withAlpha(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func stringsFields(value string) []string {
	var fields []string
	for _, field := range splitBySpace(value) {
		if field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

func stringsTrimSpace(value string) string {
	for len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	for len(value) > 0 && value[len(value)-1] == ' ' {
		value = value[:len(value)-1]
	}
	return value
}

func splitBySpace(value string) []string {
	var parts []string
	start := 0
	inWord := false
	for i, r := range value {
		if r == ' ' || r == '\t' || r == '\n' {
			if inWord {
				parts = append(parts, value[start:i])
				inWord = false
			}
			continue
		}
		if !inWord {
			start = i
			inWord = true
		}
	}
	if inWord {
		parts = append(parts, value[start:])
	}
	return parts
}
