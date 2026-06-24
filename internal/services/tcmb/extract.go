package tcmb

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

const minUsableTCMBTextLength = 200

var (
	tcmbHTMLScriptStyleRE = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<noscript[^>]*>.*?</noscript>`)
	tcmbHTMLTagRE         = regexp.MustCompile(`(?s)<[^>]+>`)
	tcmbHTMLSpaceRE       = regexp.MustCompile(`[ \t\r\f\v]+`)
	tcmbHTMLNewlineRE     = regexp.MustCompile(`\n{3,}`)
)

// ExtractDocumentTexts converts downloaded TCMB PDF/HTML files into a searchable text index.
func ExtractDocumentTexts(ctx context.Context, opts DocumentTextExtractOptions) (DocumentTextExtractResult, error) {
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("data", "macro", "tcmb")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	manifestPath := filepath.Join(opts.OutputDir, ManifestFile)
	entries, err := readTCMBManifestForText(manifestPath, opts.Categories)
	if err != nil {
		return DocumentTextExtractResult{}, err
	}

	if err := os.MkdirAll(filepath.Join(opts.OutputDir, "text"), 0o755); err != nil {
		return DocumentTextExtractResult{}, fmt.Errorf("tcmb text: create text dir: %w", err)
	}

	indexPath := filepath.Join(opts.OutputDir, TextIndexFile)
	indexFile, err := os.Create(indexPath)
	if err != nil {
		return DocumentTextExtractResult{}, fmt.Errorf("tcmb text: create index: %w", err)
	}
	defer indexFile.Close()
	writer := bufio.NewWriter(indexFile)
	defer writer.Flush()

	stats := map[string]*TextCategoryStat{}
	result := DocumentTextExtractResult{
		OutputDir:     opts.OutputDir,
		TextIndexPath: indexPath,
	}

	for i, entry := range entries {
		if opts.Limit > 0 && i >= opts.Limit {
			break
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}

		stat := stats[entry.Category]
		if stat == nil {
			stat = &TextCategoryStat{ID: entry.Category}
			stats[entry.Category] = stat
		}
		stat.Documents++
		result.Documents++

		indexEntry, extracted, skipped := extractSingleTCMBText(ctx, opts, entry)
		if extracted {
			result.Extracted++
			stat.Extracted++
		}
		if skipped {
			result.Skipped++
			stat.Skipped++
		}
		if indexEntry.Status == "error" || indexEntry.Status == "missing_file" || indexEntry.Status == "unsupported" {
			result.Errors++
			stat.Errors++
		}
		if len(indexEntry.Warnings) > 0 {
			result.Warnings = appendUniqueStrings(result.Warnings, indexEntry.Warnings...)
		}
		raw, err := json.Marshal(indexEntry)
		if err != nil {
			return result, fmt.Errorf("tcmb text: marshal index entry: %w", err)
		}
		if _, err := writer.Write(append(raw, '\n')); err != nil {
			return result, fmt.Errorf("tcmb text: write index: %w", err)
		}
	}

	for _, stat := range stats {
		result.ByCategory = append(result.ByCategory, *stat)
	}
	sort.Slice(result.ByCategory, func(i, j int) bool {
		return result.ByCategory[i].ID < result.ByCategory[j].ID
	})
	sort.Strings(result.Warnings)
	return result, nil
}

func readTCMBManifestForText(manifestPath string, categories []string) ([]ManifestEntry, error) {
	f, err := os.Open(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("tcmb text: manifest not found: %s", manifestPath)
	}
	if err != nil {
		return nil, fmt.Errorf("tcmb text: open manifest: %w", err)
	}
	defer f.Close()

	selected := map[string]bool{}
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category != "" {
			selected[category] = true
		}
	}

	seen := map[string]bool{}
	var entries []ManifestEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ManifestEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if len(selected) > 0 && !selected[entry.Category] {
			continue
		}
		key := entry.Category + "|" + entry.URL + "|" + entry.LocalPath
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("tcmb text: scan manifest: %w", err)
	}
	return entries, nil
}

func extractSingleTCMBText(ctx context.Context, opts DocumentTextExtractOptions, entry ManifestEntry) (DocumentTextIndexEntry, bool, bool) {
	now := time.Now().UTC()
	localPath := resolveTCMBLocalPath(opts.OutputDir, entry.LocalPath)
	textPath := tcmbTextPath(opts.OutputDir, entry)
	indexEntry := DocumentTextIndexEntry{
		Category:    entry.Category,
		Title:       entry.Title,
		URL:         entry.URL,
		LocalPath:   localPath,
		TextPath:    textPath,
		ContentType: entry.ContentType,
		Status:      "pending",
		ExtractedAt: now,
	}

	if _, err := os.Stat(localPath); err != nil {
		indexEntry.Status = "missing_file"
		indexEntry.Warnings = append(indexEntry.Warnings, "local_file_missing")
		return indexEntry, false, false
	}

	if !opts.Force {
		if raw, err := os.ReadFile(textPath); err == nil && len(bytes.TrimSpace(raw)) > 0 {
			indexEntry.TextLength = len(bytes.TrimSpace(raw))
			indexEntry.Status = tcmbTextStatus(indexEntry.TextLength)
			return indexEntry, false, true
		}
	}

	if err := os.MkdirAll(filepath.Dir(textPath), 0o755); err != nil {
		indexEntry.Status = "error"
		indexEntry.Warnings = append(indexEntry.Warnings, "text_dir_create_failed:"+err.Error())
		return indexEntry, false, false
	}

	var text string
	var err error
	switch {
	case isTCMBPDF(entry):
		text, err = tcmbPDFToText(ctx, localPath, opts.Timeout)
	case isTCMBHTML(entry):
		text, err = tcmbHTMLFileToText(localPath)
	default:
		indexEntry.Status = "unsupported"
		indexEntry.Warnings = append(indexEntry.Warnings, "unsupported_document_type")
		return indexEntry, false, false
	}
	if err != nil {
		indexEntry.Status = "error"
		indexEntry.Warnings = append(indexEntry.Warnings, err.Error())
		return indexEntry, false, false
	}
	text = strings.TrimSpace(text)
	if err := os.WriteFile(textPath, []byte(text+"\n"), 0o644); err != nil {
		indexEntry.Status = "error"
		indexEntry.Warnings = append(indexEntry.Warnings, "text_write_failed:"+err.Error())
		return indexEntry, false, false
	}
	indexEntry.TextLength = len(text)
	indexEntry.Status = tcmbTextStatus(indexEntry.TextLength)
	if indexEntry.Status == "short_text" {
		indexEntry.Warnings = append(indexEntry.Warnings, "text_too_short")
	}
	return indexEntry, true, false
}

func resolveTCMBLocalPath(outputDir, localPath string) string {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return localPath
	}
	if filepath.IsAbs(localPath) {
		return localPath
	}
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}
	candidate := filepath.Join(outputDir, localPath)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return localPath
}

func tcmbTextPath(outputDir string, entry ManifestEntry) string {
	base := filepath.Base(strings.TrimSpace(entry.LocalPath))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = filepath.Base(strings.TrimSpace(entry.URL))
	}
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	base = sanitizeTCMBTextFilename(base)
	if base == "" {
		base = "tcmb_document"
	}
	sum := sha256.Sum256([]byte(entry.Category + "|" + entry.URL + "|" + entry.LocalPath))
	hash := hex.EncodeToString(sum[:])[:12]
	return filepath.Join(outputDir, "text", sanitizeTCMBTextFilename(entry.Category), base+"_"+hash+".txt")
}

func sanitizeTCMBTextFilename(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	prevDash := false
	for _, r := range value {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if ok {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func isTCMBPDF(entry ManifestEntry) bool {
	ct := strings.ToLower(entry.ContentType)
	ext := strings.ToLower(filepath.Ext(entry.LocalPath))
	url := strings.ToLower(entry.URL)
	return strings.Contains(ct, "pdf") || ext == ".pdf" || strings.Contains(url, ".pdf")
}

func isTCMBHTML(entry ManifestEntry) bool {
	ct := strings.ToLower(entry.ContentType)
	ext := strings.ToLower(filepath.Ext(entry.LocalPath))
	url := strings.ToLower(entry.URL)
	return strings.Contains(ct, "html") || ext == ".html" || ext == ".htm" || strings.Contains(url, ".html") || strings.Contains(url, ".htm")
}

func tcmbPDFToText(ctx context.Context, path string, timeout time.Duration) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", errors.New("pdftotext_missing")
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "pdftotext", "-layout", path, "-")
	out, err := cmd.Output()
	if runCtx.Err() == context.DeadlineExceeded {
		return "", errors.New("pdftotext_timeout")
	}
	if err != nil {
		return "", fmt.Errorf("pdftotext_failed:%v", err)
	}
	return string(out), nil
}

func tcmbHTMLFileToText(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("html_read_failed:%v", err)
	}
	return StripHTMLText(string(raw)), nil
}

// StripHTMLText converts a downloaded TCMB HTML document into plain text.
func StripHTMLText(raw string) string {
	raw = tcmbHTMLScriptStyleRE.ReplaceAllString(raw, " ")
	raw = strings.ReplaceAll(raw, "</p>", "</p>\n")
	raw = strings.ReplaceAll(raw, "</div>", "</div>\n")
	raw = strings.ReplaceAll(raw, "<br>", "<br>\n")
	raw = strings.ReplaceAll(raw, "<br/>", "<br/>\n")
	raw = strings.ReplaceAll(raw, "<br />", "<br />\n")
	raw = tcmbHTMLTagRE.ReplaceAllString(raw, " ")
	raw = html.UnescapeString(raw)
	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(tcmbHTMLSpaceRE.ReplaceAllString(line, " "))
	}
	return strings.TrimSpace(tcmbHTMLNewlineRE.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

func tcmbTextStatus(textLength int) string {
	if textLength >= minUsableTCMBTextLength {
		return "ok"
	}
	return "short_text"
}

func appendUniqueStrings(values []string, more ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			seen[value] = true
		}
	}
	for _, value := range more {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	return values
}
