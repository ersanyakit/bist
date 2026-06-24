package kapsectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"hissebot/internal/config"
	"hissebot/internal/storage"
	"hissebot/internal/util"
)

const SourceName = "kap_sector_page"

type Options struct {
	URL        string
	OutputPath string
	Timeout    time.Duration
}

type Result struct {
	OutputPath    string `json:"output_path"`
	SourceURL     string `json:"source_url"`
	Rows          int    `json:"rows"`
	Symbols       int    `json:"symbols"`
	MainSectors   int    `json:"main_sectors"`
	NormalSectors int    `json:"normal_sectors"`
}

type File struct {
	Source       string                   `json:"source"`
	SourceURL    string                   `json:"source_url"`
	GeneratedAt  time.Time                `json:"generated_at"`
	Summary      Summary                  `json:"summary"`
	SectorTitles []SectorTitle            `json:"sector_titles"`
	Entries      map[string]CompanySector `json:"entries"`
}

type Summary struct {
	Rows          int `json:"rows"`
	Symbols       int `json:"symbols"`
	MainSectors   int `json:"main_sectors"`
	NormalSectors int `json:"normal_sectors"`
}

type SectorTitle struct {
	MainSector   string   `json:"main_sector"`
	NormalSector []string `json:"normal_sector,omitempty"`
}

type CompanySector struct {
	Symbol        string             `json:"symbol"`
	Title         string             `json:"title"`
	MainSector    string             `json:"main_sector"`
	Sector        string             `json:"sector"`
	MainSectorNo  string             `json:"main_sector_no,omitempty"`
	SectorNo      string             `json:"sector_no,omitempty"`
	MainSectorOID string             `json:"main_sector_oid,omitempty"`
	SectorOID     string             `json:"sector_oid,omitempty"`
	MKKMemberOID  string             `json:"mkk_member_oid,omitempty"`
	KAPTypes      []string           `json:"kap_types,omitempty"`
	AllSectors    []SectorMembership `json:"all_sectors"`
}

type SectorMembership struct {
	Level         string   `json:"level"`
	MainSector    string   `json:"main_sector"`
	Sector        string   `json:"sector,omitempty"`
	MainSectorNo  string   `json:"main_sector_no,omitempty"`
	SectorNo      string   `json:"sector_no,omitempty"`
	MainSectorOID string   `json:"main_sector_oid,omitempty"`
	SectorOID     string   `json:"sector_oid,omitempty"`
	KAPTypes      []string `json:"kap_types,omitempty"`
}

type pagePayload struct {
	SectorTitles []rawSectorTitle `json:"sectorTitles"`
	Data         []rawSectorNode  `json:"data"`
}

type rawSectorTitle struct {
	MainSector   string   `json:"mainSector"`
	NormalSector []string `json:"normalSector"`
}

type rawSectorNode struct {
	Title    string                   `json:"title"`
	Children map[string]rawSectorNode `json:"children"`
	Content  []rawSectorRecord        `json:"content"`
}

type rawSectorRecord struct {
	MainSectorName string   `json:"mainSectorName"`
	MainSectorOID  string   `json:"mainSectorOid"`
	MainSectorNo   string   `json:"mainSectorNo"`
	SectorName     string   `json:"sectorName"`
	SectorOID      string   `json:"sectorOid"`
	SectorNo       string   `json:"sectorNo"`
	MKKMemberOID   string   `json:"mkkMemberOid"`
	StockCode      string   `json:"stockCode"`
	Title          string   `json:"title"`
	KAPTypes       []string `json:"kapTypes"`
}

func Sync(ctx context.Context, cfg config.Config, opts Options) (Result, error) {
	opts = normalizeOptions(cfg, opts)
	file, err := Fetch(ctx, opts.URL, opts.Timeout)
	if err != nil {
		return Result{}, err
	}
	if err := util.WriteJSON(opts.OutputPath, file); err != nil {
		return Result{}, err
	}
	return Result{
		OutputPath:    opts.OutputPath,
		SourceURL:     file.SourceURL,
		Rows:          file.Summary.Rows,
		Symbols:       file.Summary.Symbols,
		MainSectors:   file.Summary.MainSectors,
		NormalSectors: file.Summary.NormalSectors,
	}, nil
}

func Fetch(ctx context.Context, url string, timeout time.Duration) (File, error) {
	if strings.TrimSpace(url) == "" {
		return File{}, errors.New("kap sectors url is empty")
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return File{}, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "hissebot-go/1.0")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return File{}, fmt.Errorf("kap sectors fetch: %w", err)
	}
	data, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return File{}, readErr
	}
	if closeErr != nil {
		return File{}, closeErr
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return File{}, fmt.Errorf("kap sectors fetch: status %d: %s", resp.StatusCode, truncateForError(data, 500))
	}
	return ExtractFromHTML(data, url, time.Now().UTC())
}

func Load(path string) (File, error) {
	var file File
	if err := util.ReadJSON(path, &file); err != nil {
		return File{}, err
	}
	if file.Entries == nil {
		file.Entries = map[string]CompanySector{}
	}
	return file, nil
}

func ExtractFromHTML(raw []byte, sourceURL string, generatedAt time.Time) (File, error) {
	payloadText, err := extractEmbeddedPayload(string(raw))
	if err != nil {
		return File{}, err
	}
	var payload pagePayload
	if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
		return File{}, fmt.Errorf("kap sector payload decode: %w", err)
	}
	if len(payload.Data) == 0 {
		return File{}, errors.New("kap sector payload has no data nodes")
	}

	file := File{
		Source:       SourceName,
		SourceURL:    sourceURL,
		GeneratedAt:  generatedAt,
		SectorTitles: normalizeSectorTitles(payload.SectorTitles),
		Entries:      map[string]CompanySector{},
	}
	rows := 0
	walkNodes(payload.Data, "", func(record rawSectorRecord, parentMain string) {
		rows++
		for _, symbol := range splitStockCodes(record.StockCode) {
			mergeRecord(file.Entries, symbol, record, parentMain)
		}
	})
	for symbol, entry := range file.Entries {
		if entry.Sector == "" {
			entry.Sector = entry.MainSector
		}
		entry.AllSectors = uniqueMemberships(entry.AllSectors)
		file.Entries[symbol] = entry
	}
	file.Summary = Summary{
		Rows:          rows,
		Symbols:       len(file.Entries),
		MainSectors:   len(file.SectorTitles),
		NormalSectors: countNormalSectors(file.SectorTitles),
	}
	return file, nil
}

func normalizeOptions(cfg config.Config, opts Options) Options {
	if strings.TrimSpace(opts.URL) == "" {
		opts.URL = cfg.KAPSectorsURL
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		opts.OutputPath = cfg.KAPSectorsFile
	}
	if opts.Timeout <= 0 {
		opts.Timeout = cfg.HTTPTimeout
	}
	return opts
}

func normalizeSectorTitles(raw []rawSectorTitle) []SectorTitle {
	out := make([]SectorTitle, 0, len(raw))
	for _, item := range raw {
		title := strings.TrimSpace(item.MainSector)
		if title == "" {
			continue
		}
		normals := make([]string, 0, len(item.NormalSector))
		for _, normal := range item.NormalSector {
			normal = strings.TrimSpace(normal)
			if normal != "" {
				normals = append(normals, normal)
			}
		}
		out = append(out, SectorTitle{MainSector: title, NormalSector: normals})
	}
	return out
}

func extractEmbeddedPayload(page string) (string, error) {
	normalized := strings.ReplaceAll(page, `\"`, `"`)
	normalized = strings.ReplaceAll(normalized, `\\`, `\`)
	marker := `"sectorTitles"`
	markerIndex := strings.Index(normalized, marker)
	if markerIndex < 0 {
		return "", errors.New("kap sector payload marker sectorTitles not found")
	}
	start := strings.LastIndex(normalized[:markerIndex], "{")
	if start < 0 {
		return "", errors.New("kap sector payload object start not found")
	}
	end, err := findJSONObjectEnd(normalized, start)
	if err != nil {
		return "", err
	}
	return normalized[start:end], nil
}

func findJSONObjectEnd(value string, start int) (int, error) {
	inString := false
	escaped := false
	depth := 0
	for i := start; i < len(value); i++ {
		ch := value[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
			if depth < 0 {
				return 0, errors.New("kap sector payload has invalid brace depth")
			}
		}
	}
	return 0, errors.New("kap sector payload object end not found")
}

func walkNodes(nodes []rawSectorNode, parentMain string, visit func(rawSectorRecord, string)) {
	for _, node := range nodes {
		title := strings.TrimSpace(node.Title)
		nextParent := parentMain
		if parentMain == "" && title != "" {
			nextParent = title
		}
		for _, record := range node.Content {
			visit(record, parentMain)
		}
		if len(node.Children) == 0 {
			continue
		}
		keys := make([]string, 0, len(node.Children))
		for key := range node.Children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		children := make([]rawSectorNode, 0, len(keys))
		for _, key := range keys {
			children = append(children, node.Children[key])
		}
		walkNodes(children, nextParent, visit)
	}
}

func mergeRecord(entries map[string]CompanySector, symbol string, record rawSectorRecord, parentMain string) {
	mainSector := firstNonEmpty(record.MainSectorName, parentMain)
	sector := strings.TrimSpace(record.SectorName)
	level := "main"
	if sector != "" {
		level = "normal"
	}
	membership := SectorMembership{
		Level:         level,
		MainSector:    mainSector,
		Sector:        sector,
		MainSectorNo:  strings.TrimSpace(record.MainSectorNo),
		SectorNo:      strings.TrimSpace(record.SectorNo),
		MainSectorOID: strings.TrimSpace(record.MainSectorOID),
		SectorOID:     strings.TrimSpace(record.SectorOID),
		KAPTypes:      append([]string{}, record.KAPTypes...),
	}
	entry := entries[symbol]
	if entry.Symbol == "" {
		entry.Symbol = symbol
	}
	entry.Title = firstNonEmpty(entry.Title, record.Title)
	entry.MKKMemberOID = firstNonEmpty(entry.MKKMemberOID, record.MKKMemberOID)
	entry.KAPTypes = appendUniqueStrings(entry.KAPTypes, record.KAPTypes...)
	entry.AllSectors = append(entry.AllSectors, membership)
	if mainSector != "" {
		entry.MainSector = firstNonEmpty(entry.MainSector, mainSector)
	}
	if level == "main" {
		entry.MainSectorNo = firstNonEmpty(entry.MainSectorNo, record.MainSectorNo)
		entry.MainSectorOID = firstNonEmpty(entry.MainSectorOID, record.MainSectorOID)
	}
	if level == "normal" {
		entry.Sector = firstNonEmpty(entry.Sector, sector)
		entry.SectorNo = firstNonEmpty(entry.SectorNo, record.SectorNo)
		entry.SectorOID = firstNonEmpty(entry.SectorOID, record.SectorOID)
	}
	entries[symbol] = entry
}

func splitStockCodes(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		symbol := storage.NormalizeTicker(part)
		if symbol != "" {
			out = append(out, symbol)
		}
	}
	return out
}

func uniqueMemberships(values []SectorMembership) []SectorMembership {
	seen := map[string]bool{}
	out := make([]SectorMembership, 0, len(values))
	for _, value := range values {
		key := strings.Join([]string{value.Level, value.MainSector, value.Sector, value.MainSectorNo, value.SectorNo}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func countNormalSectors(titles []SectorTitle) int {
	total := 0
	for _, title := range titles {
		total += len(title.NormalSector)
	}
	return total
}

func truncateForError(data []byte, limit int) string {
	value := strings.TrimSpace(string(data))
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func appendUniqueStrings(values []string, next ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values)+len(next))
	for _, value := range append(values, next...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
