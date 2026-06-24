package kap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"hissebot/internal/config"
	"hissebot/internal/storage"
)

const defaultKAPBaseURL = "https://www.kap.org.tr"

type AttachmentSyncOptions struct {
	OutputDir               string
	LegacyOutputDir         string
	EquitiesDir             string
	BaseURL                 string
	Ticker                  string
	FromDate                time.Time
	ToDate                  time.Time
	Limit                   int
	MaxErrors               int
	NewestFirst             bool
	Force                   bool
	DetailsOnly             bool
	Delay                   time.Duration
	Retries                 int
	RateLimitSleep          time.Duration
	TransientErrorSleep     time.Duration
	TransientErrorThreshold int
	ErrorDelay              time.Duration
	Verbose                 bool
	MaxBytes                int64
	MinFreeBytes            int64
	Timeout                 time.Duration
}

type AttachmentSyncResult struct {
	Tickers              int    `json:"tickers"`
	DisclosureCandidates int    `json:"disclosure_candidates"`
	DetailsFetched       int    `json:"details_fetched"`
	DetailsCached        int    `json:"details_cached"`
	FilesDownloaded      int    `json:"files_downloaded"`
	FilesSkipped         int    `json:"files_skipped"`
	Errors               int    `json:"errors"`
	DownloadedBytes      int64  `json:"downloaded_bytes"`
	OutputDir            string `json:"output_dir"`
	StoppedReason        string `json:"stopped_reason,omitempty"`
}

type attachmentCandidate struct {
	Ticker          string
	DisclosureID    string
	DisclosureIndex int
	AttachmentCount int
	Title           string
	PublishedAt     time.Time
}

type attachmentDetailFile struct {
	Source          string                    `json:"source"`
	SourceURL       string                    `json:"source_url"`
	Ticker          string                    `json:"ticker"`
	DisclosureIndex int                       `json:"disclosure_index"`
	FetchedAt       time.Time                 `json:"fetched_at"`
	Raw             []attachmentDetailPayload `json:"raw"`
}

type attachmentDetailPayload struct {
	Disclosure     attachmentDisclosure `json:"disclosure"`
	DisclosureBody []string             `json:"disclosureBody"`
	Attachments    []KAPAttachment      `json:"attachments"`
}

type attachmentDisclosure struct {
	Basic  map[string]any `json:"disclosureBasic"`
	Detail map[string]any `json:"disclosureDetail"`
}

type KAPAttachment struct {
	ObjID         string `json:"objId"`
	FileName      string `json:"fileName"`
	FileExtension string `json:"fileExtension"`
}

type attachmentManifestEntry struct {
	Ticker          string    `json:"ticker"`
	DisclosureIndex int       `json:"disclosure_index"`
	ObjID           string    `json:"obj_id"`
	FileName        string    `json:"file_name"`
	Path            string    `json:"path"`
	URL             string    `json:"url"`
	Size            int64     `json:"size"`
	SHA256          string    `json:"sha256"`
	ContentType     string    `json:"content_type,omitempty"`
	DownloadedAt    time.Time `json:"downloaded_at"`
}

type attachmentFailureEntry struct {
	Ticker          string    `json:"ticker"`
	DisclosureIndex int       `json:"disclosure_index"`
	ObjID           string    `json:"obj_id,omitempty"`
	FileName        string    `json:"file_name,omitempty"`
	Stage           string    `json:"stage"`
	Error           string    `json:"error"`
	FailedAt        time.Time `json:"failed_at"`
}

var errKAPRateLimited = errors.New("kap request rate limited")

func SyncAttachments(ctx context.Context, cfg config.Config, store *storage.EquityStore, opts AttachmentSyncOptions) (AttachmentSyncResult, error) {
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join(cfg.EquitiesDir, "_kap")
	}
	if opts.LegacyOutputDir == "" {
		legacy := filepath.Join(cfg.DataDir, "kap")
		if legacy != opts.OutputDir {
			opts.LegacyOutputDir = legacy
		}
	}
	if opts.EquitiesDir == "" && store != nil {
		opts.EquitiesDir = store.Root()
	}
	if opts.BaseURL == "" {
		opts.BaseURL = defaultKAPBaseURL
	}
	if opts.Delay <= 0 {
		opts.Delay = time.Second
	}
	if opts.ErrorDelay <= 0 {
		opts.ErrorDelay = maxDuration(5*time.Second, opts.Delay)
	}
	if opts.TransientErrorSleep <= 0 {
		opts.TransientErrorSleep = opts.RateLimitSleep
	}
	if opts.TransientErrorSleep <= 0 {
		opts.TransientErrorSleep = 10 * time.Minute
	}
	if opts.TransientErrorThreshold <= 0 {
		opts.TransientErrorThreshold = 5
	}
	if opts.Timeout <= 0 {
		opts.Timeout = cfg.HTTPTimeout
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Minute
	}
	result := AttachmentSyncResult{OutputDir: opts.OutputDir}
	client := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DisableKeepAlives:   true,
			ForceAttemptHTTP2:   false,
			TLSHandshakeTimeout: 30 * time.Second,
		},
	}
	candidates, tickers, err := attachmentCandidates(store, opts)
	if err != nil {
		return result, err
	}
	result.Tickers = tickers
	result.DisclosureCandidates = len(candidates)
	failedDetails := map[int]struct{}{}
	failedObjects := map[string]struct{}{}
	seenObjects := map[string]struct{}{}
	consecutiveTransientErrors := 0
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if opts.Limit > 0 && result.FilesDownloaded >= opts.Limit {
			result.StoppedReason = "limit_reached"
			break
		}
		if opts.MaxBytes > 0 && result.DownloadedBytes >= opts.MaxBytes {
			result.StoppedReason = "max_bytes_reached"
			break
		}
		if opts.MinFreeBytes > 0 {
			free, err := freeBytes(opts.OutputDir)
			if err == nil && free < opts.MinFreeBytes {
				result.StoppedReason = "min_free_bytes_reached"
				break
			}
		}
		if _, failed := failedDetails[candidate.DisclosureIndex]; failed {
			continue
		}
		var detail attachmentDetailFile
		var cached bool
		var err error
		for attempt := 0; ; attempt++ {
			detail, cached, err = loadOrFetchAttachmentDetail(ctx, client, opts, candidate)
			if err == nil {
				consecutiveTransientErrors = 0
				break
			}
			if errors.Is(err, errKAPRateLimited) && opts.RateLimitSleep > 0 {
				sleepOrDone(ctx, opts.RateLimitSleep)
				continue
			}
			if attempt >= opts.Retries {
				break
			}
			sleepOrDone(ctx, retryDelay(opts.Delay, attempt))
		}
		if err != nil {
			if errors.Is(err, errKAPRateLimited) {
				result.StoppedReason = "rate_limited"
				return result, nil
			}
			result.Errors++
			_ = appendFailure(opts.OutputDir, attachmentFailureEntry{
				Ticker:          candidate.Ticker,
				DisclosureIndex: candidate.DisclosureIndex,
				Stage:           "detail",
				Error:           err.Error(),
				FailedAt:        time.Now().UTC(),
			})
			if opts.Verbose {
				fmt.Printf("kap attachments: detail %s/%d failed: %v\n", candidate.Ticker, candidate.DisclosureIndex, err)
			}
			if opts.MaxErrors > 0 && result.Errors >= opts.MaxErrors {
				result.StoppedReason = "max_errors_reached"
				return result, nil
			}
			failedDetails[candidate.DisclosureIndex] = struct{}{}
			consecutiveTransientErrors = sleepAfterAttachmentError(ctx, opts, err, consecutiveTransientErrors)
			continue
		}
		if cached {
			result.DetailsCached++
		} else {
			result.DetailsFetched++
			sleepOrDone(ctx, opts.Delay)
		}
		if opts.DetailsOnly {
			continue
		}
		for _, payload := range detail.Raw {
			for _, attachment := range payload.Attachments {
				if err := ctx.Err(); err != nil {
					return result, err
				}
				if opts.Limit > 0 && result.FilesDownloaded >= opts.Limit {
					result.StoppedReason = "limit_reached"
					break
				}
				if opts.MaxBytes > 0 && result.DownloadedBytes >= opts.MaxBytes {
					result.StoppedReason = "max_bytes_reached"
					break
				}
				objID := strings.TrimSpace(attachment.ObjID)
				if _, failed := failedObjects[objID]; objID != "" && failed {
					result.FilesSkipped++
					continue
				}
				if _, seen := seenObjects[objID]; objID != "" && seen {
					result.FilesSkipped++
					continue
				}
				var downloaded bool
				var size int64
				var err error
				for attempt := 0; ; attempt++ {
					downloaded, size, err = downloadAttachment(ctx, client, opts, candidate, attachment)
					if err == nil {
						break
					}
					if errors.Is(err, errKAPRateLimited) && opts.RateLimitSleep > 0 {
						sleepOrDone(ctx, opts.RateLimitSleep)
						continue
					}
					if attempt >= opts.Retries {
						break
					}
					sleepOrDone(ctx, retryDelay(opts.Delay, attempt))
				}
				if err != nil {
					if errors.Is(err, errKAPRateLimited) {
						result.StoppedReason = "rate_limited"
						return result, nil
					}
					result.Errors++
					_ = appendFailure(opts.OutputDir, attachmentFailureEntry{
						Ticker:          candidate.Ticker,
						DisclosureIndex: candidate.DisclosureIndex,
						ObjID:           attachment.ObjID,
						FileName:        attachment.FileName,
						Stage:           "file",
						Error:           err.Error(),
						FailedAt:        time.Now().UTC(),
					})
					if opts.Verbose {
						fmt.Printf("kap attachments: file %s/%d/%s failed: %v\n", candidate.Ticker, candidate.DisclosureIndex, attachment.ObjID, err)
					}
					if opts.MaxErrors > 0 && result.Errors >= opts.MaxErrors {
						result.StoppedReason = "max_errors_reached"
						return result, nil
					}
					if objID != "" {
						failedObjects[objID] = struct{}{}
					}
					consecutiveTransientErrors = sleepAfterAttachmentError(ctx, opts, err, consecutiveTransientErrors)
					continue
				}
				if downloaded {
					consecutiveTransientErrors = 0
					result.FilesDownloaded++
					result.DownloadedBytes += size
					sleepOrDone(ctx, opts.Delay)
				} else {
					result.FilesSkipped++
				}
				if objID != "" {
					seenObjects[objID] = struct{}{}
				}
			}
			if result.StoppedReason != "" {
				break
			}
		}
		if result.StoppedReason != "" {
			break
		}
	}
	return result, nil
}

func attachmentCandidates(store *storage.EquityStore, opts AttachmentSyncOptions) ([]attachmentCandidate, int, error) {
	tickers := []string{}
	if ticker := storage.NormalizeTicker(opts.Ticker); ticker != "" {
		tickers = []string{ticker}
	} else {
		equities, err := store.List()
		if err != nil {
			return nil, 0, err
		}
		for _, equity := range equities {
			if equity.AssetType == 2 {
				tickers = append(tickers, equity.Ticker)
			}
		}
	}
	out := []attachmentCandidate{}
	seenCandidates := map[string]struct{}{}
	for _, ticker := range tickers {
		disclosures, err := LoadFinancialDisclosures(store.KAPDisclosuresPath(ticker))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return out, len(tickers), err
		}
		for _, disclosure := range disclosures {
			index, count := disclosureAttachmentRef(disclosure)
			if index == 0 || count <= 0 {
				continue
			}
			candidateTicker := firstNonEmpty(storage.NormalizeTicker(disclosure.Ticker), ticker)
			key := fmt.Sprintf("%s:%d", candidateTicker, index)
			if _, seen := seenCandidates[key]; seen {
				continue
			}
			seenCandidates[key] = struct{}{}
			publishedAt := time.Time{}
			if disclosure.PublishDate != nil {
				publishedAt = disclosure.PublishDate.UTC()
			}
			if !timeInRange(publishedAt, opts.FromDate, opts.ToDate) {
				continue
			}
			out = append(out, attachmentCandidate{
				Ticker:          candidateTicker,
				DisclosureID:    disclosure.ID,
				DisclosureIndex: index,
				AttachmentCount: count,
				Title:           disclosure.Title,
				PublishedAt:     publishedAt,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].PublishedAt.Equal(out[j].PublishedAt) {
			if opts.NewestFirst {
				return out[i].PublishedAt.After(out[j].PublishedAt)
			}
			return out[i].PublishedAt.Before(out[j].PublishedAt)
		}
		if out[i].Ticker != out[j].Ticker {
			return out[i].Ticker < out[j].Ticker
		}
		return out[i].DisclosureIndex < out[j].DisclosureIndex
	})
	return out, len(tickers), nil
}

func disclosureAttachmentRef(disclosure FinancialDisclosure) (int, int) {
	index := disclosure.DisclosureIndex
	count := disclosure.AttachmentCount
	if index != 0 && count != 0 {
		return index, count
	}
	flat := flattenRawDisclosure(disclosure.Raw)
	if index == 0 {
		index = firstInt(flat, "disclosure_index", "disclosureIndex")
	}
	if count == 0 {
		count = firstInt(flat, "attachment_count", "attachmentCount")
	}
	return index, count
}

func flattenRawDisclosure(raw map[string]any) map[string]any {
	flat := map[string]any{}
	var walk func(map[string]any)
	walk = func(item map[string]any) {
		for key, value := range item {
			if _, exists := flat[key]; !exists {
				flat[key] = value
			}
			if nested, ok := value.(map[string]any); ok {
				walk(nested)
			}
		}
	}
	if raw != nil {
		walk(raw)
	}
	return flat
}

func loadOrFetchAttachmentDetail(ctx context.Context, client *http.Client, opts AttachmentSyncOptions, candidate attachmentCandidate) (attachmentDetailFile, bool, error) {
	path := detailPath(opts.OutputDir, candidate)
	indexPath := detailIndexPath(opts.OutputDir, candidate.DisclosureIndex)
	if !opts.Force {
		var cached attachmentDetailFile
		if err := readJSON(path, &cached); err == nil && len(cached.Raw) > 0 {
			return cached, true, nil
		}
		if err := readJSON(indexPath, &cached); err == nil && len(cached.Raw) > 0 {
			cached.Ticker = candidate.Ticker
			if err := writeJSON(path, cached); err != nil {
				return attachmentDetailFile{}, false, err
			}
			return cached, true, nil
		}
		if opts.LegacyOutputDir != "" {
			legacyPath := detailPath(opts.LegacyOutputDir, candidate)
			if err := readJSON(legacyPath, &cached); err == nil && len(cached.Raw) > 0 {
				if err := writeJSON(indexPath, cached); err != nil {
					return attachmentDetailFile{}, false, err
				}
				if err := writeJSON(path, cached); err != nil {
					return attachmentDetailFile{}, false, err
				}
				removeIfSet(legacyPath)
				return cached, true, nil
			}
			legacyIndexPath := detailIndexPath(opts.LegacyOutputDir, candidate.DisclosureIndex)
			if err := readJSON(legacyIndexPath, &cached); err == nil && len(cached.Raw) > 0 {
				cached.Ticker = candidate.Ticker
				if err := writeJSON(indexPath, cached); err != nil {
					return attachmentDetailFile{}, false, err
				}
				if err := writeJSON(path, cached); err != nil {
					return attachmentDetailFile{}, false, err
				}
				removeIfSet(legacyIndexPath)
				return cached, true, nil
			}
		}
	}
	url := fmt.Sprintf("%s/tr/api/notification/attachment-detail/%d", strings.TrimRight(opts.BaseURL, "/"), candidate.DisclosureIndex)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return attachmentDetailFile{}, false, err
	}
	setKAPRequestHeaders(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", fmt.Sprintf("%s/tr/Bildirim/%d", strings.TrimRight(opts.BaseURL, "/"), candidate.DisclosureIndex))
	req.Header.Set("Connection", "close")
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		return attachmentDetailFile{}, false, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil && len(raw) == 0 {
		return attachmentDetailFile{}, false, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusTooManyRequests {
			return attachmentDetailFile{}, false, errKAPRateLimited
		}
		return attachmentDetailFile{}, false, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var payload []attachmentDetailPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		if readErr != nil {
			return attachmentDetailFile{}, false, readErr
		}
		return attachmentDetailFile{}, false, err
	}
	detail := attachmentDetailFile{
		Source:          "kap_attachment_detail",
		SourceURL:       url,
		Ticker:          candidate.Ticker,
		DisclosureIndex: candidate.DisclosureIndex,
		FetchedAt:       time.Now().UTC(),
		Raw:             payload,
	}
	if err := writeJSON(indexPath, detail); err != nil {
		return attachmentDetailFile{}, false, err
	}
	if err := writeJSON(path, detail); err != nil {
		return attachmentDetailFile{}, false, err
	}
	return detail, false, nil
}

func downloadAttachment(ctx context.Context, client *http.Client, opts AttachmentSyncOptions, candidate attachmentCandidate, attachment KAPAttachment) (bool, int64, error) {
	if strings.TrimSpace(attachment.ObjID) == "" {
		return false, 0, errors.New("empty attachment objId")
	}
	equityPath := attachmentEquityPath(opts.EquitiesDir, candidate, attachment)
	if equityPath == "" {
		return false, 0, errors.New("empty equities attachment output path")
	}
	legacyPath := ""
	legacyObjectPath := ""
	if opts.LegacyOutputDir != "" {
		legacyPath = attachmentPath(opts.LegacyOutputDir, candidate, attachment)
		legacyObjectPath = attachmentObjectPath(opts.LegacyOutputDir, attachment)
	}
	if !opts.Force {
		if info, err := os.Stat(equityPath); err == nil && info.Size() > 0 {
			removeIfSet(legacyPath)
			removeIfSet(legacyObjectPath)
			return false, 0, nil
		}
		if legacyPath != "" {
			if info, err := os.Stat(legacyPath); err == nil && info.Size() > 0 {
				if err := moveFile(legacyPath, equityPath); err != nil {
					return false, 0, err
				}
				removeIfSet(legacyObjectPath)
				return false, 0, nil
			}
		}
		if legacyObjectPath != "" {
			if info, err := os.Stat(legacyObjectPath); err == nil && info.Size() > 0 {
				if err := moveFile(legacyObjectPath, equityPath); err != nil {
					return false, 0, err
				}
				return false, 0, nil
			}
		}
		newLegacyPath := attachmentPath(opts.OutputDir, candidate, attachment)
		newLegacyObjectPath := attachmentObjectPath(opts.OutputDir, attachment)
		if info, err := os.Stat(newLegacyPath); err == nil && info.Size() > 0 {
			if err := moveFile(newLegacyPath, equityPath); err != nil {
				return false, 0, err
			}
			removeIfSet(newLegacyObjectPath)
			return false, 0, nil
		}
		if info, err := os.Stat(newLegacyObjectPath); err == nil && info.Size() > 0 {
			if err := moveFile(newLegacyObjectPath, equityPath); err != nil {
				return false, 0, err
			}
			return false, 0, nil
		}
	}
	url := fmt.Sprintf("%s/tr/api/file/download/%s", strings.TrimRight(opts.BaseURL, "/"), attachment.ObjID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, 0, err
	}
	setKAPRequestHeaders(req)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", fmt.Sprintf("%s/tr/Bildirim/%d", strings.TrimRight(opts.BaseURL, "/"), candidate.DisclosureIndex))
	req.Header.Set("Connection", "close")
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil && len(raw) == 0 {
		return false, 0, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusTooManyRequests {
			return false, 0, errKAPRateLimited
		}
		return false, 0, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	body, err := unwrapKAPFile(raw)
	if err != nil {
		if readErr != nil {
			return false, 0, readErr
		}
		return false, 0, err
	}
	if readErr != nil && !completeKAPJavaByteArray(raw) {
		return false, 0, readErr
	}
	if opts.Force {
		_ = os.Remove(equityPath)
	}
	if err := writeBytes(equityPath, body); err != nil {
		return false, 0, err
	}
	sum := sha256.Sum256(body)
	entry := attachmentManifestEntry{
		Ticker:          candidate.Ticker,
		DisclosureIndex: candidate.DisclosureIndex,
		ObjID:           attachment.ObjID,
		FileName:        attachment.FileName,
		Path:            equityPath,
		URL:             url,
		Size:            int64(len(body)),
		SHA256:          hex.EncodeToString(sum[:]),
		ContentType:     resp.Header.Get("Content-Type"),
		DownloadedAt:    time.Now().UTC(),
	}
	if err := appendManifest(opts.OutputDir, entry); err != nil {
		return false, 0, err
	}
	return true, int64(len(body)), nil
}

func unwrapKAPFile(raw []byte) ([]byte, error) {
	if completeKAPJavaByteArray(raw) {
		length := int(binary.BigEndian.Uint32(raw[23:27]))
		return raw[27 : 27+length], nil
	}
	for _, sig := range [][]byte{
		[]byte("%PDF"),
		[]byte("PK\x03\x04"),
		[]byte("<?xml"),
		[]byte("<xbrl"),
		[]byte("<html"),
	} {
		if idx := bytes.Index(raw, sig); idx >= 0 {
			return raw[idx:], nil
		}
	}
	return raw, nil
}

func completeKAPJavaByteArray(raw []byte) bool {
	if len(raw) < 27 ||
		raw[0] != 0xac || raw[1] != 0xed || raw[2] != 0x00 || raw[3] != 0x05 ||
		raw[4] != 0x75 || raw[5] != 0x72 || raw[6] != 0x00 || raw[7] != 0x02 ||
		raw[8] != '[' || raw[9] != 'B' {
		return false
	}
	length := int(binary.BigEndian.Uint32(raw[23:27]))
	return length >= 0 && 27+length <= len(raw)
}

func detailPath(root string, candidate attachmentCandidate) string {
	return filepath.Join(root, "details", candidate.Ticker, strconv.Itoa(candidate.DisclosureIndex)+".json")
}

func detailIndexPath(root string, disclosureIndex int) string {
	return filepath.Join(root, "details_by_index", strconv.Itoa(disclosureIndex)+".json")
}

func attachmentPath(root string, candidate attachmentCandidate, attachment KAPAttachment) string {
	year := "unknown"
	if !candidate.PublishedAt.IsZero() {
		year = strconv.Itoa(candidate.PublishedAt.Year())
	}
	name := sanitizeFileName(attachment.FileName)
	if name == "" {
		name = attachment.ObjID
		if ext := strings.Trim(strings.ToLower(attachment.FileExtension), ". "); ext != "" {
			name += "." + ext
		}
	}
	return filepath.Join(root, "attachments", candidate.Ticker, year, strconv.Itoa(candidate.DisclosureIndex), attachment.ObjID+"_"+name)
}

func attachmentEquityPath(root string, candidate attachmentCandidate, attachment KAPAttachment) string {
	if root == "" {
		return ""
	}
	year := "unknown"
	if !candidate.PublishedAt.IsZero() {
		year = strconv.Itoa(candidate.PublishedAt.Year())
	}
	name := sanitizeFileName(attachment.FileName)
	if name == "" {
		name = attachment.ObjID
		if ext := strings.Trim(strings.ToLower(attachment.FileExtension), ". "); ext != "" {
			name += "." + ext
		}
	}
	return filepath.Join(root, candidate.Ticker, "kap", "attachments", year, strconv.Itoa(candidate.DisclosureIndex), attachment.ObjID+"_"+name)
}

func attachmentObjectPath(root string, attachment KAPAttachment) string {
	objID := strings.TrimSpace(attachment.ObjID)
	prefix := "unknown"
	if len(objID) >= 2 {
		prefix = objID[:2]
	}
	ext := strings.ToLower(filepath.Ext(sanitizeFileName(attachment.FileName)))
	if ext == "" {
		if value := strings.Trim(strings.ToLower(attachment.FileExtension), ". "); value != "" {
			ext = "." + value
		}
	}
	if ext == "" {
		ext = ".bin"
	}
	return filepath.Join(root, "objects", prefix, objID+ext)
}

func sanitizeFileName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == 0:
			return '-'
		case unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, value)
	if len([]rune(value)) > 180 {
		runes := []rune(value)
		value = string(runes[:180])
	}
	return strings.Trim(value, ". ")
}

func appendManifest(root string, entry attachmentManifestEntry) error {
	path := filepath.Join(root, "attachments_manifest.jsonl")
	return appendJSONL(path, entry)
}

func appendFailure(root string, entry attachmentFailureEntry) error {
	path := filepath.Join(root, "attachments_failures.jsonl")
	return appendJSONL(path, entry)
}

func appendJSONL(path string, entry any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeBytes(path, append(raw, '\n'))
}

func readJSON(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}

func writeBytes(path string, value []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(value); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func moveFile(src string, dst string) error {
	if info, err := os.Stat(dst); err == nil && info.Size() > 0 {
		return os.Remove(src)
	}
	if info, err := os.Stat(src); err != nil {
		return err
	} else if info.Size() == 0 {
		return fmt.Errorf("source attachment is empty: %s", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := writeBytes(dst, raw); err != nil {
		return err
	}
	return os.Remove(src)
}

func removeIfSet(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

func freeBytes(path string) (int64, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return 0, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

func timeInRange(ts time.Time, from time.Time, to time.Time) bool {
	if ts.IsZero() {
		return true
	}
	if !from.IsZero() && ts.Before(from) {
		return false
	}
	if !to.IsZero() && ts.After(to) {
		return false
	}
	return true
}

func sleepOrDone(ctx context.Context, delay time.Duration) {
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

func sleepAfterAttachmentError(ctx context.Context, opts AttachmentSyncOptions, err error, consecutiveTransientErrors int) int {
	if !isTransientAttachmentError(err) {
		sleepOrDone(ctx, opts.ErrorDelay)
		return 0
	}
	consecutiveTransientErrors++
	if consecutiveTransientErrors >= opts.TransientErrorThreshold {
		sleepOrDone(ctx, opts.TransientErrorSleep)
		return 0
	}
	sleepOrDone(ctx, opts.ErrorDelay)
	return consecutiveTransientErrors
}

func isTransientAttachmentError(err error) bool {
	if err == nil {
		return false
	}
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "unexpected eof") ||
		strings.Contains(value, "connection broken") ||
		strings.Contains(value, "connection reset") ||
		strings.Contains(value, "connection refused") ||
		strings.Contains(value, "timeout") ||
		strings.Contains(value, "temporary") ||
		strings.Contains(value, "tls handshake timeout")
}

func retryDelay(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	factor := attempt + 1
	if factor > 5 {
		factor = 5
	}
	return time.Duration(factor) * base
}

func maxDuration(a time.Duration, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
