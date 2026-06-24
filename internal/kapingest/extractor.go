package kapingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"hissebot/internal/util"
)

type PDFTextExtractor struct {
	Timeout         time.Duration
	DisableOCR      bool
	OCRTimeout      time.Duration
	OCRLanguages    string
	EnableVision    bool
	VisionCommand   string
	VisionTimeout   time.Duration
	VisionMaxPages  int
	VisionRenderDPI int
}

func (e PDFTextExtractor) ExtractText(ctx context.Context, filePath string) (string, string, []string, error) {
	text, method, warnings, err := e.extractWithPDFToText(ctx, filePath)
	if err != nil {
		if e.DisableOCR {
			if visionText, visionMethod, visionWarnings, visionErr := e.extractWithVision(ctx, filePath, "", method, warnings); visionErr == nil {
				return visionText, visionMethod, visionWarnings, nil
			} else if e.EnableVision {
				warnings = append(warnings, "visionlm_fallback_failed: "+visionErr.Error())
			}
			return "", method, dedupeStrings(warnings), err
		}
		ocrText, ocrWarnings, ocrErr := e.extractWithOCR(ctx, filePath)
		if ocrErr == nil && strings.TrimSpace(ocrText) != "" {
			warnings = append(warnings, "pdftotext_failed_ocr_used")
			warnings = append(warnings, ocrWarnings...)
			return e.maybeImproveWithVision(ctx, filePath, ocrText, "ocr", warnings)
		}
		if ocrErr != nil {
			warnings = append(warnings, "ocr_fallback_failed: "+ocrErr.Error())
		}
		if visionText, visionMethod, visionWarnings, visionErr := e.extractWithVision(ctx, filePath, "", method, warnings); visionErr == nil {
			return visionText, visionMethod, visionWarnings, nil
		} else if e.EnableVision {
			warnings = append(warnings, "visionlm_fallback_failed: "+visionErr.Error())
		}
		return "", "pdftotext", dedupeStrings(warnings), err
	}

	quality := AssessTextQuality(text)
	ocrAuditNeeded := shouldRunOCRAudit(text, filePath, warnings)
	if e.DisableOCR {
		if ocrAuditNeeded {
			warnings = append(warnings, "ocr_audit_required_but_disabled")
		}
		return e.maybeImproveWithVision(ctx, filePath, text, method, warnings)
	}
	if !ocrAuditNeeded {
		return e.maybeImproveWithVision(ctx, filePath, text, method, warnings)
	}

	ocrText, ocrWarnings, ocrErr := e.extractWithOCR(ctx, filePath)
	if ocrErr != nil {
		warnings = append(warnings, "ocr_fallback_failed: "+ocrErr.Error())
		return e.maybeImproveWithVision(ctx, filePath, text, "pdftotext", warnings)
	}
	selectedText, selectedMethod, selectedWarnings := selectOCRCandidate(text, method, warnings, quality, ocrText, ocrWarnings)
	return e.maybeImproveWithVision(ctx, filePath, selectedText, selectedMethod, selectedWarnings)
}

func shouldRunOCRAudit(text, filePath string, warnings []string) bool {
	quality := AssessTextQuality(text)
	if strings.TrimSpace(text) == "" || quality.Score < RejectedTextQualityThreshold {
		return true
	}
	if warningHasPrefix(warnings, "coordinate_tsv_failed") || containsRawWarning(warnings, "invalid_utf8_replaced") {
		return true
	}
	if criticalKAPActionCandidate(text, filepath.Base(filePath)) && criticalKAPActionCoverage(text) < 3 {
		return true
	}
	return false
}

func selectOCRCandidate(baseText, baseMethod string, baseWarnings []string, baseQuality QualityResult, ocrText string, ocrWarnings []string) (string, string, []string) {
	warnings := dedupeStrings(append(append([]string{}, baseWarnings...), ocrWarnings...))
	ocrQuality := AssessTextQuality(ocrText)
	if strings.TrimSpace(ocrText) == "" {
		warnings = append(warnings, "ocr_fallback_no_text")
		return baseText, baseMethod, dedupeStrings(warnings)
	}
	if ocrQuality.Score > baseQuality.Score && ocrQuality.TextLength >= baseQuality.TextLength {
		warnings = append(warnings, "pdftotext_low_quality_ocr_used")
		return ocrText, "pdftotext+ocr", dedupeStrings(warnings)
	}
	if ocrAddsCriticalCoverage(baseText, ocrText) {
		warnings = append(warnings, "ocr_audit_text_appended")
		combined := NormalizeExtractedText(baseText + "\n\n###OCR_AUDIT_TEXT###\n" + ocrText)
		method := strings.TrimSuffix(baseMethod, "+") + "+ocr_audit"
		return combined, method, dedupeStrings(warnings)
	}
	warnings = append(warnings, "ocr_fallback_no_quality_improvement")
	return baseText, baseMethod, dedupeStrings(warnings)
}

func criticalKAPActionCandidate(text, fileName string) bool {
	docType := ClassifyDocument(text, fileName)
	switch NormalizeDocumentType(docType) {
	case DocumentCapitalIncrease, DocumentDividendDistribution, DocumentMaterialDisclosure, DocumentBoardDecision:
		return true
	}
	slug := utilSlugForExtraction(fileName + " " + text)
	return strings.Contains(slug, "sermaye artir") ||
		strings.Contains(slug, "bedelli") ||
		strings.Contains(slug, "bedelsiz") ||
		strings.Contains(slug, "ruchan hakki") ||
		strings.Contains(slug, "kar payi") ||
		strings.Contains(slug, "temettu")
}

func criticalKAPActionCoverage(text string) int {
	slug := utilSlugForExtraction(text)
	score := 0
	for _, token := range []string{"sermaye artir", "bedelli", "bedelsiz", "ruchan hakki", "kar payi", "temettu", "odeme tarihi", "hak kullanim", "spk"} {
		if strings.Contains(slug, token) {
			score++
		}
	}
	if extractPercentValue(text) != nil {
		score++
	}
	if extractDateString(text) != nil {
		score++
	}
	if len(extractMoneyAmounts(text)) > 0 {
		score++
	}
	return score
}

func ocrAddsCriticalCoverage(baseText, ocrText string) bool {
	baseCoverage := criticalKAPActionCoverage(baseText)
	ocrCoverage := criticalKAPActionCoverage(ocrText)
	if criticalKAPActionCandidate(ocrText, "") && ocrCoverage >= baseCoverage+2 {
		return true
	}
	if !criticalKAPActionCandidate(baseText, "") {
		return criticalKAPActionCandidate(ocrText, "") && ocrCoverage >= 3
	}
	return false
}

func utilSlugForExtraction(text string) string {
	return util.SlugTR(legacyPDFTurkishReplacer.Replace(NormalizeExtractedText(text)))
}

func (e PDFTextExtractor) maybeImproveWithVision(ctx context.Context, filePath, text, method string, warnings []string) (string, string, []string, error) {
	if !e.shouldTryVision(text, warnings) {
		return text, method, dedupeStrings(warnings), nil
	}
	visionText, visionMethod, visionWarnings, err := e.extractWithVision(ctx, filePath, text, method, warnings)
	if err != nil {
		if e.EnableVision {
			warnings = append(warnings, "visionlm_fallback_failed: "+err.Error())
		}
		return text, method, dedupeStrings(warnings), nil
	}
	return visionText, visionMethod, visionWarnings, nil
}

func (e PDFTextExtractor) shouldTryVision(text string, warnings []string) bool {
	if !e.EnableVision {
		return false
	}
	if strings.TrimSpace(e.VisionCommand) == "" {
		return true
	}
	quality := AssessTextQuality(text)
	return strings.TrimSpace(text) == "" ||
		quality.Score < TrustedTextQualityThreshold ||
		warningHasPrefix(warnings, "ocr_fallback_failed") ||
		warningHasPrefix(warnings, "ocr_timeout_partial") ||
		(criticalKAPActionCandidate(text, "") && criticalKAPActionCoverage(text) < 3)
}

func (e PDFTextExtractor) extractWithVision(ctx context.Context, filePath, baseText, baseMethod string, baseWarnings []string) (string, string, []string, error) {
	command := strings.TrimSpace(e.VisionCommand)
	if command == "" {
		return "", "", nil, errors.New("visionlm command is empty")
	}
	timeout := e.VisionTimeout
	if timeout <= 0 {
		timeout = DefaultVisionTimeout
	}
	visionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "kap-vision-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("vision temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	warnings := append([]string{}, baseWarnings...)
	imageDir := filepath.Join(tmpDir, "pages")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		return "", "", nil, fmt.Errorf("vision image dir: %w", err)
	}
	images, renderErr := e.renderVisionPages(visionCtx, filePath, imageDir)
	if renderErr != nil {
		warnings = append(warnings, "visionlm_render_failed: "+renderErr.Error())
	} else {
		warnings = append(warnings, fmt.Sprintf("visionlm_pages_rendered_%d", len(images)))
	}

	stdout, stderr, err := e.runVisionCommand(visionCtx, command, filePath, imageDir, len(images))
	if err != nil {
		if errors.Is(visionCtx.Err(), context.DeadlineExceeded) {
			return "", "", nil, fmt.Errorf("visionlm timeout after %s", timeout)
		}
		if strings.TrimSpace(stderr) != "" {
			return "", "", nil, fmt.Errorf("visionlm command failed: %w: %s", err, strings.TrimSpace(stderr))
		}
		return "", "", nil, fmt.Errorf("visionlm command failed: %w", err)
	}
	visionText := NormalizeExtractedText(stdout)
	if strings.TrimSpace(visionText) == "" {
		return "", "", nil, errors.New("visionlm produced no text")
	}
	if !utf8.ValidString(visionText) {
		visionText = strings.ToValidUTF8(visionText, "")
		warnings = append(warnings, "visionlm_invalid_utf8_replaced")
	}

	candidateText := visionText
	method := "visionlm"
	if strings.TrimSpace(baseText) != "" {
		candidateText = NormalizeExtractedText(baseText + "\n\n###VISIONLM_TEXT###\n" + visionText)
		method = strings.TrimSuffix(baseMethod, "+") + "+visionlm"
	}
	baseQuality := AssessTextQuality(baseText)
	visionQuality := AssessTextQuality(visionText)
	candidateQuality := AssessTextQuality(candidateText)
	if strings.TrimSpace(baseText) == "" ||
		visionQuality.Score >= TrustedTextQualityThreshold ||
		candidateQuality.Score > baseQuality.Score ||
		baseQuality.Score < TrustedTextQualityThreshold {
		warnings = append(warnings, "visionlm_fallback_used")
		return candidateText, method, dedupeStrings(warnings), nil
	}
	warnings = append(warnings, "visionlm_fallback_no_quality_improvement")
	return baseText, baseMethod, dedupeStrings(warnings), nil
}

func (e PDFTextExtractor) renderVisionPages(ctx context.Context, filePath, imageDir string) ([]string, error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return nil, errors.New("pdftoppm bulunamadı; VisionLM sayfa render için Poppler/pdftoppm kurulu olmalı")
	}
	maxPages := e.VisionMaxPages
	if maxPages <= 0 {
		maxPages = DefaultVisionMaxPages
	}
	dpi := e.VisionRenderDPI
	if dpi <= 0 {
		dpi = DefaultVisionRenderDPI
	}
	prefix := filepath.Join(imageDir, "page")
	renderCmd := exec.CommandContext(ctx, "pdftoppm", "-png", "-r", strconv.Itoa(dpi), "-f", "1", "-l", strconv.Itoa(maxPages), filePath, prefix)
	var stderr bytes.Buffer
	renderCmd.Stderr = &stderr
	if err := renderCmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("pdftoppm failed: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("pdftoppm failed: %w", err)
	}
	images, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, fmt.Errorf("vision page glob: %w", err)
	}
	sort.Slice(images, func(i, j int) bool {
		return ocrPageIndex(images[i]) < ocrPageIndex(images[j])
	})
	if len(images) == 0 {
		return nil, errors.New("pdftoppm produced no vision page images")
	}
	return images, nil
}

func (e PDFTextExtractor) runVisionCommand(ctx context.Context, command, filePath, imageDir string, renderedPages int) (string, string, error) {
	cmd := shellCommand(ctx, command)
	maxPages := e.VisionMaxPages
	if maxPages <= 0 {
		maxPages = DefaultVisionMaxPages
	}
	cmd.Env = append(os.Environ(),
		"KAP_VISION_PDF="+filePath,
		"KAP_VISION_IMAGE_DIR="+imageDir,
		"KAP_VISION_RENDERED_PAGES="+strconv.Itoa(renderedPages),
		"KAP_VISION_MAX_PAGES="+strconv.Itoa(maxPages),
		"KAP_VISION_PROMPT="+visionExtractionPrompt(),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

func visionExtractionPrompt() string {
	return "KAP PDF sayfalarından görünen tüm metni ve tabloları Markdown olarak çıkar. Özetleme yapma. Sayfa numarasını koru. Finansal tablo, dönem, para birimi, birim, konsolide/solo ve denetim ifadelerini aynen yaz. Sermaye artırımı, bedelli, bedelsiz, rüçhan hakkı, hak kullanım tarihi, ödeme tarihi, oran, kullanım fiyatı, pay başına tutar ve temettü alanlarını özellikle aynen aktar. Okunamayan hücreleri tahmin etme; [OKUNAMADI] olarak işaretle. Tablo hücrelerini satır/sütun ilişkisi bozulmadan aktar."
}

func (e PDFTextExtractor) extractWithPDFToText(ctx context.Context, filePath string) (string, string, []string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", "pdftotext", nil, errors.New("pdftotext bulunamadı; Poppler/pdftotext kurulu olmalı")
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = DefaultExtractTimeout
	}
	extractCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(extractCtx, "pdftotext", "-layout", filePath, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(extractCtx.Err(), context.DeadlineExceeded) {
			return "", "pdftotext", nil, fmt.Errorf("pdftotext timeout after %s", timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", "pdftotext", nil, fmt.Errorf("pdftotext failed: %w: %s", err, msg)
		}
		return "", "pdftotext", nil, fmt.Errorf("pdftotext failed: %w", err)
	}
	text := stdout.String()
	warnings := []string{}
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "")
		warnings = append(warnings, "invalid_utf8_replaced")
	}
	method := "pdftotext"
	if tsvText, err := e.extractCoordinateTSVText(ctx, filePath); err == nil && strings.TrimSpace(tsvText) != "" {
		text = text + "\n\n###COORDINATE_TABLE_TEXT###\n" + tsvText
		method = "pdftotext+tsv"
		warnings = append(warnings, "coordinate_tsv_text_appended")
	} else if err != nil {
		warnings = append(warnings, "coordinate_tsv_failed: "+err.Error())
	}
	return NormalizeExtractedText(text), method, warnings, nil
}

func (e PDFTextExtractor) extractCoordinateTSVText(ctx context.Context, filePath string) (string, error) {
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = DefaultExtractTimeout
	}
	tsvCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(tsvCtx, "pdftotext", "-tsv", filePath, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(tsvCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("pdftotext -tsv timeout after %s", timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("pdftotext -tsv failed: %w: %s", err, msg)
		}
		return "", fmt.Errorf("pdftotext -tsv failed: %w", err)
	}
	text := stdout.String()
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "")
	}
	return coordinateRowsFromTSV(text), nil
}

func (e PDFTextExtractor) extractWithOCR(ctx context.Context, filePath string) (string, []string, error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return "", nil, errors.New("pdftoppm bulunamadı; OCR fallback için Poppler/pdftoppm kurulu olmalı")
	}
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", nil, errors.New("tesseract bulunamadı; OCR fallback için Tesseract kurulu olmalı")
	}

	timeout := e.OCRTimeout
	if timeout <= 0 {
		timeout = DefaultOCRTimeout
	}
	ocrCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "kap-ocr-*")
	if err != nil {
		return "", nil, fmt.Errorf("ocr temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	prefix := filepath.Join(tmpDir, "page")
	renderCmd := exec.CommandContext(ocrCtx, "pdftoppm", "-png", "-r", "220", filePath, prefix)
	var renderStderr bytes.Buffer
	renderCmd.Stderr = &renderStderr
	if err := renderCmd.Run(); err != nil {
		if errors.Is(ocrCtx.Err(), context.DeadlineExceeded) {
			return "", nil, fmt.Errorf("pdftoppm timeout after %s", timeout)
		}
		msg := strings.TrimSpace(renderStderr.String())
		if msg != "" {
			return "", nil, fmt.Errorf("pdftoppm failed: %w: %s", err, msg)
		}
		return "", nil, fmt.Errorf("pdftoppm failed: %w", err)
	}

	images, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return "", nil, fmt.Errorf("ocr page glob: %w", err)
	}
	sort.Slice(images, func(i, j int) bool {
		return ocrPageIndex(images[i]) < ocrPageIndex(images[j])
	})
	if len(images) == 0 {
		return "", nil, errors.New("pdftoppm produced no page images")
	}

	language, warnings := e.tesseractLanguage(ctx)
	var pages []string
	for _, imagePath := range images {
		pageText, err := runTesseractPage(ocrCtx, imagePath, language)
		if err != nil {
			if errors.Is(ocrCtx.Err(), context.DeadlineExceeded) {
				if len(pages) > 0 {
					warnings = append(warnings, "ocr_timeout_partial")
					break
				}
				return "", warnings, fmt.Errorf("tesseract timeout after %s", timeout)
			}
			warnings = append(warnings, "ocr_page_failed: "+filepath.Base(imagePath))
			continue
		}
		pages = append(pages, pageText)
	}
	if len(pages) == 0 {
		return "", warnings, errors.New("tesseract produced no text")
	}
	warnings = append(warnings, "ocr_fallback_used")
	text := strings.Join(pages, "\n\f\n")
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "")
		warnings = append(warnings, "ocr_invalid_utf8_replaced")
	}
	return NormalizeExtractedText(text), dedupeStrings(warnings), nil
}

func (e PDFTextExtractor) tesseractLanguage(ctx context.Context) (string, []string) {
	requested := strings.TrimSpace(e.OCRLanguages)
	if requested != "" {
		return requested, nil
	}
	langs := availableTesseractLanguages(ctx)
	warnings := []string{}
	hasTur := langs["tur"]
	hasEng := langs["eng"]
	switch {
	case hasTur && hasEng:
		return "tur+eng", nil
	case hasTur:
		return "tur", nil
	case hasEng:
		warnings = append(warnings, "ocr_turkish_language_missing")
		return "eng", warnings
	default:
		warnings = append(warnings, "ocr_language_list_unavailable")
		return "eng", warnings
	}
}

func availableTesseractLanguages(ctx context.Context) map[string]bool {
	langs := map[string]bool{}
	listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(listCtx, "tesseract", "--list-langs")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return langs
	}
	for _, line := range strings.Split(stdout.String()+"\n"+stderr.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "list of available") {
			continue
		}
		langs[line] = true
	}
	return langs
}

func runTesseractPage(ctx context.Context, imagePath, language string) (string, error) {
	cmd := exec.CommandContext(ctx, "tesseract", imagePath, "stdout", "-l", language, "--psm", "6")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("tesseract failed: %w: %s", err, msg)
		}
		return "", fmt.Errorf("tesseract failed: %w", err)
	}
	return stdout.String(), nil
}

type pdfTSVWord struct {
	Page   int
	Left   float64
	Top    float64
	Width  float64
	Height float64
	Text   string
}

func coordinateRowsFromTSV(tsv string) string {
	words := parsePDFTSVWords(tsv)
	if len(words) == 0 {
		return ""
	}
	sort.SliceStable(words, func(i, j int) bool {
		if words[i].Page != words[j].Page {
			return words[i].Page < words[j].Page
		}
		if pdfAbs(words[i].Top-words[j].Top) > 2.4 {
			return words[i].Top < words[j].Top
		}
		return words[i].Left < words[j].Left
	})

	lines := [][]pdfTSVWord{}
	current := []pdfTSVWord{}
	currentPage := -1
	currentTop := 0.0
	currentHeight := 0.0
	flush := func() {
		if len(current) == 0 {
			return
		}
		lines = append(lines, current)
		current = nil
	}
	for _, word := range words {
		tolerance := maxFloat(2.4, currentHeight*0.55)
		if len(current) == 0 || word.Page != currentPage || pdfAbs(word.Top-currentTop) > tolerance {
			flush()
			current = []pdfTSVWord{word}
			currentPage = word.Page
			currentTop = word.Top
			currentHeight = word.Height
			continue
		}
		current = append(current, word)
		currentTop = (currentTop*float64(len(current)-1) + word.Top) / float64(len(current))
		currentHeight = maxFloat(currentHeight, word.Height)
	}
	flush()

	var b strings.Builder
	for _, line := range lines {
		text := coordinateLineText(line)
		if strings.TrimSpace(text) == "" {
			continue
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func parsePDFTSVWords(tsv string) []pdfTSVWord {
	lines := strings.Split(tsv, "\n")
	if len(lines) < 2 {
		return nil
	}
	header := strings.Split(strings.TrimRight(lines[0], "\r"), "\t")
	columns := map[string]int{}
	for i, name := range header {
		columns[strings.TrimSpace(name)] = i
	}
	required := []string{"level", "page_num", "left", "top", "width", "height", "text"}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return nil
		}
	}
	out := []pdfTSVWord{}
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) <= columns["text"] || fieldAt(fields, columns["level"]) != "5" {
			continue
		}
		text := strings.TrimSpace(strings.Join(fields[columns["text"]:], "\t"))
		if text == "" || strings.HasPrefix(text, "###") {
			continue
		}
		page, err := strconv.Atoi(fieldAt(fields, columns["page_num"]))
		if err != nil {
			continue
		}
		left, ok := parsePDFTSVFloat(fieldAt(fields, columns["left"]))
		if !ok {
			continue
		}
		top, ok := parsePDFTSVFloat(fieldAt(fields, columns["top"]))
		if !ok {
			continue
		}
		width, ok := parsePDFTSVFloat(fieldAt(fields, columns["width"]))
		if !ok {
			continue
		}
		height, ok := parsePDFTSVFloat(fieldAt(fields, columns["height"]))
		if !ok {
			continue
		}
		out = append(out, pdfTSVWord{Page: page, Left: left, Top: top, Width: width, Height: height, Text: text})
	}
	return out
}

func coordinateLineText(words []pdfTSVWord) string {
	sort.SliceStable(words, func(i, j int) bool {
		if words[i].Left == words[j].Left {
			return words[i].Text < words[j].Text
		}
		return words[i].Left < words[j].Left
	})
	avgHeight := 0.0
	for _, word := range words {
		avgHeight += word.Height
	}
	avgHeight = avgHeight / float64(maxInt(len(words), 1))
	cellGap := maxFloat(13, avgHeight*1.65)
	var b strings.Builder
	lastRight := 0.0
	for i, word := range words {
		if i > 0 {
			gap := word.Left - lastRight
			if gap >= cellGap {
				b.WriteString("\t")
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString(word.Text)
		lastRight = maxFloat(lastRight, word.Left+word.Width)
	}
	cells := strings.Split(b.String(), "\t")
	for i, cell := range cells {
		cells[i] = strings.Join(strings.Fields(cell), " ")
	}
	return strings.Join(cells, "\t")
}

func fieldAt(fields []string, idx int) string {
	if idx < 0 || idx >= len(fields) {
		return ""
	}
	return strings.TrimSpace(fields[idx])
}

func parsePDFTSVFloat(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed, err == nil
}

func pdfAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

var ocrPageSuffixRE = regexp.MustCompile(`-(\d+)\.png$`)

func ocrPageIndex(path string) int {
	matches := ocrPageSuffixRE.FindStringSubmatch(filepath.Base(path))
	if len(matches) != 2 {
		return 0
	}
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return value
}

var blankLinesPattern = regexp.MustCompile(`\n{4,}`)
var wideSpacesPattern = regexp.MustCompile(`[ \t]{8,}`)

func NormalizeExtractedText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\x00", "")
	text = strings.ReplaceAll(text, "\u00a0", " ")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.ReplaceAll(line, "\t", "    ")
		line = strings.TrimRight(line, " ")
		line = wideSpacesPattern.ReplaceAllString(line, "    ")
		lines[i] = line
	}
	text = strings.TrimSpace(strings.Join(lines, "\n"))
	text = blankLinesPattern.ReplaceAllString(text, "\n\n\n")
	return text
}
