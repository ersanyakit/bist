package tcmb

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Sync fetches TCMB documents (PDFs) for each category and writes them to OutputDir.
func Sync(ctx context.Context, opts Options) (SyncResult, error) {
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("data", "macro", "tcmb")
	}
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	if opts.Delay <= 0 {
		opts.Delay = DefaultDelay
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}

	categories := resolveCategories(opts.Categories)
	if len(categories) == 0 {
		return SyncResult{}, fmt.Errorf("tcmb sync: no categories selected")
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return SyncResult{}, fmt.Errorf("tcmb: create output dir: %w", err)
	}

	existing, err := loadManifest(opts.OutputDir)
	if err != nil {
		return SyncResult{}, fmt.Errorf("tcmb: load manifest: %w", err)
	}

	client := newTCMBClient(opts.Timeout)
	result := SyncResult{
		OutputDir:    opts.OutputDir,
		ManifestPath: filepath.Join(opts.OutputDir, ManifestFile),
	}
	totalDownloaded := 0

	for _, cat := range categories {
		if err := ctx.Err(); err != nil {
			break
		}
		stat, err := syncCategory(ctx, client, opts, cat, existing, &totalDownloaded)
		if err != nil {
			log.Printf("tcmb: category %s error: %v", cat.ID, err)
			stat.Errors++
		}
		result.ByCategory = append(result.ByCategory, stat)
		result.Downloaded += stat.Downloaded
		result.Skipped += stat.Skipped
		result.Errors += stat.Errors
		result.Categories = append(result.Categories, cat.ID)
	}

	return result, nil
}

func syncCategory(ctx context.Context, client *http.Client, opts Options, cat Category, existing map[string]struct{}, totalDownloaded *int) (CategoryStat, error) {
	stat := CategoryStat{ID: cat.ID}

	catDir := filepath.Join(opts.OutputDir, cat.ID)
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		return stat, fmt.Errorf("create category dir %s: %w", catDir, err)
	}

	listingURL := strings.TrimRight(opts.BaseURL, "/") + cat.ListingPath
	if opts.Verbose {
		log.Printf("tcmb: scraping %s -> %s", cat.ID, listingURL)
	}

	docURLs, err := collectDocumentLinks(ctx, client, opts.BaseURL, listingURL, cat, opts.Verbose)
	if err != nil {
		return stat, fmt.Errorf("collect document links for %s: %w", cat.ID, err)
	}

	for _, docURL := range docURLs {
		if err := ctx.Err(); err != nil {
			break
		}
		if opts.Limit > 0 && *totalDownloaded >= opts.Limit {
			break
		}

		if !opts.Force {
			key := manifestKey(cat.ID, docURL)
			if _, ok := existing[key]; ok {
				stat.Skipped++
				continue
			}
		}

		sleepContext(ctx, opts.Delay)

		filename := documentFilename(docURL, "", nil)
		localPath := filepath.Join(catDir, filename)

		if !opts.Force {
			if info, err := os.Stat(localPath); err == nil && info.Size() > 0 {
				stat.Skipped++
				continue
			}
		}

		body, ct, err := fetchDocument(ctx, client, opts.BaseURL, docURL)
		if err != nil {
			if opts.Verbose {
				log.Printf("tcmb: download error %s: %v", docURL, err)
			}
			_ = appendFailure(opts.OutputDir, FailureEntry{
				Category: cat.ID,
				URL:      docURL,
				Error:    err.Error(),
				FailedAt: time.Now().UTC(),
			})
			stat.Errors++
			continue
		}

		if len(body) == 0 {
			stat.Errors++
			continue
		}

		if !isAllowedDocumentContent(cat, ct, body, docURL) {
			if opts.Verbose {
				log.Printf("tcmb: skipping unsupported content %s (content-type: %s)", docURL, ct)
			}
			stat.Skipped++
			continue
		}
		filename = documentFilename(docURL, ct, body)
		localPath = filepath.Join(catDir, filename)

		if err := os.WriteFile(localPath, body, 0o644); err != nil {
			stat.Errors++
			continue
		}

		sum := sha256.Sum256(body)
		entry := ManifestEntry{
			Category:    cat.ID,
			URL:         docURL,
			LocalPath:   localPath,
			Size:        int64(len(body)),
			SHA256:      hex.EncodeToString(sum[:]),
			ContentType: ct,
			FetchedAt:   time.Now().UTC(),
		}
		if err := appendManifest(opts.OutputDir, entry); err != nil {
			log.Printf("tcmb: manifest write error: %v", err)
		}
		existing[manifestKey(cat.ID, docURL)] = struct{}{}

		stat.Downloaded++
		*totalDownloaded++

		if opts.Verbose {
			log.Printf("tcmb: downloaded %s -> %s (%d bytes)", docURL, localPath, len(body))
		}
	}

	return stat, nil
}

// collectPDFLinks crawls the listing page and sub-pages up to maxDepth to find PDF URLs.
func collectPDFLinks(ctx context.Context, client *http.Client, baseURL, listingURL string, maxDepth int, verbose bool) ([]string, error) {
	return collectDocumentLinks(ctx, client, baseURL, listingURL, Category{FileExts: []string{".pdf"}, MaxDepth: maxDepth}, verbose)
}

// collectDocumentLinks crawls the listing page and sub-pages up to maxDepth to find downloadable document URLs.
func collectDocumentLinks(ctx context.Context, client *http.Client, baseURL, listingURL string, cat Category, verbose bool) ([]string, error) {
	seen := map[string]struct{}{}
	docSeen := map[string]struct{}{}
	var docs []string
	maxDepth := cat.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	allowHTML := categoryAllowsExt(cat, ".html")

	addDoc := func(url string) {
		if strings.TrimSpace(url) == "" {
			return
		}
		if _, ok := docSeen[url]; ok {
			return
		}
		docSeen[url] = struct{}{}
		docs = append(docs, url)
	}

	var crawl func(pageURL string, depth int) error
	crawl = func(pageURL string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, ok := seen[pageURL]; ok {
			return nil
		}
		seen[pageURL] = struct{}{}

		raw, err := fetchPage(ctx, client, baseURL, pageURL)
		if err != nil {
			if verbose {
				log.Printf("tcmb: fetch page error %s: %v", pageURL, err)
			}
			return nil
		}
		if allowHTML {
			addDoc(pageURL)
		}

		for _, pdfURL := range extractPDFLinks(baseURL, raw) {
			addDoc(pdfURL)
		}

		if depth < maxDepth {
			// Parse current path from pageURL
			currentPath := ""
			if parsed, err := parseURLPath(pageURL); err == nil {
				currentPath = parsed
			}
			for _, subURL := range extractSubpageLinks(baseURL, currentPath, raw) {
				_ = crawl(subURL, depth+1)
			}
		}
		return nil
	}

	if err := crawl(listingURL, 1); err != nil {
		return nil, err
	}
	return docs, nil
}

func parseURLPath(rawURL string) (string, error) {
	// Simple path extraction
	after := strings.TrimPrefix(rawURL, "https://")
	after = strings.TrimPrefix(after, "http://")
	idx := strings.Index(after, "/")
	if idx < 0 {
		return "", fmt.Errorf("no path in URL")
	}
	return after[idx:], nil
}

// manifest helpers

func manifestKey(category, url string) string {
	return category + "|" + url
}

func loadManifest(dir string) (map[string]struct{}, error) {
	path := filepath.Join(dir, ManifestFile)
	out := map[string]struct{}{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ManifestEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		out[manifestKey(entry.Category, entry.URL)] = struct{}{}
	}
	return out, scanner.Err()
}

func appendManifest(dir string, entry ManifestEntry) error {
	return appendJSONL(filepath.Join(dir, ManifestFile), entry)
}

func appendFailure(dir string, entry FailureEntry) error {
	return appendJSONL(filepath.Join(dir, FailuresFile), entry)
}

func appendJSONL(path string, value any) error {
	line, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}

func resolveCategories(ids []string) []Category {
	all := DefaultCategories()
	if len(ids) == 0 {
		return all
	}
	byID := make(map[string]Category, len(all))
	for _, cat := range all {
		byID[cat.ID] = cat
	}
	var out []Category
	for _, id := range ids {
		if cat, ok := byID[id]; ok {
			out = append(out, cat)
		}
	}
	return out
}

func categoryAllowsExt(cat Category, ext string) bool {
	ext = strings.ToLower(strings.TrimSpace(ext))
	for _, item := range cat.FileExts {
		if strings.ToLower(strings.TrimSpace(item)) == ext {
			return true
		}
	}
	return false
}

func sleepContext(ctx context.Context, delay time.Duration) {
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
