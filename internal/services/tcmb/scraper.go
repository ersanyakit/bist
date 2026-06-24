package tcmb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	// Matches href="/wps/wcm/connect/..." or full https://www.tcmb.gov.tr/wps/...
	hrefPattern = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
	// Matches /wps/wcm/connect/{uuid}/{filename}.pdf patterns
	wcmPathPattern = regexp.MustCompile(`(?i)/wps/wcm/connect/[^"'\s>]+\.pdf`)
	// Also match direct .pdf hrefs anywhere on page
	anyPDFPattern = regexp.MustCompile(`(?i)href=["']([^"']+\.pdf[^"']*)["']`)
	// Pagination: detect links to next pages
	pagePattern = regexp.MustCompile(`(?i)href=["']([^"']+(?:page|sayfa)[^"']*)["']`)
)

func newTCMBClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{Timeout: timeout}
}

func fetchPage(ctx context.Context, client *http.Client, baseURL, pageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9")
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, pageURL)
	}
	limited := io.LimitReader(resp.Body, maxBodyBytes)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func fetchDocument(ctx context.Context, client *http.Client, baseURL, docURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, docURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/pdf,text/html,application/xhtml+xml,*/*")
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, docURL)
	}
	limited := io.LimitReader(resp.Body, maxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	return body, ct, nil
}

// extractPDFLinks returns absolute PDF URLs found in raw HTML.
func extractPDFLinks(baseURL string, raw []byte) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(raw string) {
		href := strings.TrimSpace(raw)
		if href == "" || href == "#" {
			return
		}
		abs := absoluteURL(baseURL, href)
		if abs == "" {
			return
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}

	// Match explicit .pdf hrefs
	for _, m := range anyPDFPattern.FindAllSubmatch(raw, -1) {
		if len(m) >= 2 {
			add(string(m[1]))
		}
	}

	// Match WCM connect paths that end with .pdf (sometimes not in href attribute directly)
	for _, m := range wcmPathPattern.FindAll(raw, -1) {
		add(string(m))
	}

	return out
}

// extractSubpageLinks returns links to listing sub-pages (pagination, year archives).
func extractSubpageLinks(baseURL, currentPath string, raw []byte) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(href string) {
		href = strings.TrimSpace(href)
		if href == "" || href == "#" || strings.HasPrefix(href, "javascript") {
			return
		}
		abs := absoluteURL(baseURL, href)
		if abs == "" || !strings.HasPrefix(abs, baseURL) {
			return
		}
		if _, ok := seen[abs]; ok {
			return
		}
		// Only follow sub-paths of the current category path
		parsedCurrent, err := url.Parse(baseURL + currentPath)
		if err != nil {
			return
		}
		parsedAbs, err := url.Parse(abs)
		if err != nil {
			return
		}
		if !strings.HasPrefix(strings.ToLower(parsedAbs.Path), strings.ToLower(parsedCurrent.Path)) {
			return
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}

	for _, m := range hrefPattern.FindAllSubmatch(raw, -1) {
		if len(m) >= 2 {
			href := string(m[1])
			// Follow links that look like year sub-pages or pagination
			lower := strings.ToLower(href)
			if strings.Contains(lower, "/wps/wcm/connect/tr/") &&
				!strings.HasSuffix(lower, ".pdf") &&
				!strings.HasSuffix(lower, ".doc") &&
				!strings.HasSuffix(lower, ".xls") {
				add(href)
			}
		}
	}

	// Pagination links
	for _, m := range pagePattern.FindAllSubmatch(raw, -1) {
		if len(m) >= 2 {
			add(string(m[1]))
		}
	}

	return out
}

func absoluteURL(baseURL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return strings.TrimRight(baseURL, "/") + href
	}
	return ""
}

// slugFromURL derives a safe filename from a URL path.
func slugFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return sanitizeFilename(rawURL)
	}
	base := path.Base(parsed.Path)
	if base == "" || base == "/" || base == "." {
		// Use last meaningful path segment
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				base = parts[i]
				break
			}
		}
	}
	return sanitizeFilename(base)
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	var sb strings.Builder
	for _, r := range name {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			sb.WriteRune('_')
		} else {
			sb.WriteRune(r)
		}
	}
	result := sb.String()
	if result == "" {
		return "document"
	}
	return result
}

func isPDFContent(contentType string, body []byte) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(ct, "pdf") {
		return true
	}
	if len(body) >= 4 && string(body[:4]) == "%PDF" {
		return true
	}
	return false
}

func isHTMLContent(contentType string, body []byte) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(ct, "html") {
		return true
	}
	trimmed := strings.TrimSpace(strings.ToLower(string(firstBytes(body, 256))))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}

func isAllowedDocumentContent(cat Category, contentType string, body []byte, docURL string) bool {
	if categoryAllowsExt(cat, ".pdf") && isPDFContent(contentType, body) {
		return true
	}
	if categoryAllowsExt(cat, ".html") && isHTMLContent(contentType, body) {
		return true
	}
	return false
}

func documentFilename(rawURL, contentType string, body []byte) string {
	name := slugFromURL(rawURL)
	ext := strings.ToLower(path.Ext(name))
	if ext == "" || ext == "." {
		switch {
		case isPDFContent(contentType, body):
			name += ".pdf"
		case isHTMLContent(contentType, body):
			name += ".html"
		default:
			name += ".bin"
		}
	}
	return name
}

func firstBytes(body []byte, limit int) []byte {
	if len(body) <= limit {
		return body
	}
	return body[:limit]
}
