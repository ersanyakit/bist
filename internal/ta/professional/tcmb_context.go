package professional

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type TCMBContextReport struct {
	Computed                      bool                   `json:"computed"`
	ManifestPath                  string                 `json:"manifest_path,omitempty"`
	TextIndexPath                 string                 `json:"text_index_path,omitempty"`
	DocumentCount                 int                    `json:"document_count"`
	TextDocumentCount             int                    `json:"text_document_count"`
	TextUsableCount               int                    `json:"text_usable_count"`
	Categories                    []TCMBCategoryStat     `json:"categories,omitempty"`
	TextCategories                []TCMBTextCategoryStat `json:"text_categories,omitempty"`
	RequiredCategoriesMissing     []string               `json:"required_categories_missing,omitempty"`
	RequiredTextCategoriesMissing []string               `json:"required_text_categories_missing,omitempty"`
	LatestFetchAt                 time.Time              `json:"latest_fetch_at,omitempty"`
	LatestTextExtractedAt         time.Time              `json:"latest_text_extracted_at,omitempty"`
	Summary                       string                 `json:"summary"`
	Warnings                      []string               `json:"warnings,omitempty"`
}

type TCMBCategoryStat struct {
	ID            string    `json:"id"`
	DocumentCount int       `json:"document_count"`
	LatestFetchAt time.Time `json:"latest_fetch_at,omitempty"`
}

type TCMBTextCategoryStat struct {
	ID                      string    `json:"id"`
	TextDocumentCount       int       `json:"text_document_count"`
	UsableTextDocumentCount int       `json:"usable_text_document_count"`
	LatestExtractedAt       time.Time `json:"latest_extracted_at,omitempty"`
}

type tcmbManifestEntry struct {
	Category  string    `json:"category"`
	URL       string    `json:"url"`
	FetchedAt time.Time `json:"fetched_at"`
}

type tcmbTextIndexEntry struct {
	Category    string    `json:"category"`
	Status      string    `json:"status"`
	TextLength  int       `json:"text_length"`
	ExtractedAt time.Time `json:"extracted_at"`
}

type tcmbTextIndexSummary struct {
	Path              string
	DocumentCount     int
	UsableCount       int
	CategoryCounts    map[string]int
	CategoryUsable    map[string]int
	CategoryLatest    map[string]time.Time
	LatestExtractedAt time.Time
	Warnings          []string
}

func requiredTCMBDocumentCategoryIDs() []string {
	return []string{
		"ppk_kararlari",
		"faiz_kararlari",
		"basin_duyurulari",
		"enflasyon_raporu",
		"para_politikasi_metinleri",
		"yillik_rapor",
		"finansal_istikrar_raporu",
		"baskanlarin_konusmalari",
	}
}

func buildTCMBContext(tcmbDir string) TCMBContextReport {
	tcmbDir = strings.TrimSpace(tcmbDir)
	if tcmbDir == "" {
		return TCMBContextReport{
			Summary:  "TCMB PDF dizini yapılandırılmamış.",
			Warnings: []string{"tcmb_dir_not_configured"},
		}
	}

	manifestPath := filepath.Join(tcmbDir, "manifest.jsonl")
	f, err := os.Open(manifestPath)
	if os.IsNotExist(err) {
		return TCMBContextReport{
			ManifestPath: manifestPath,
			Summary:      fmt.Sprintf("TCMB manifest bulunamadı: %s — önce `sync tcmb` komutunu çalıştırın.", manifestPath),
			Warnings:     []string{"tcmb_manifest_missing"},
		}
	}
	if err != nil {
		return TCMBContextReport{
			ManifestPath: manifestPath,
			Summary:      "TCMB manifest okunamadı: " + err.Error(),
			Warnings:     []string{"tcmb_manifest_read_error"},
		}
	}
	defer f.Close()

	catCounts := map[string]int{}
	catLatest := map[string]time.Time{}
	var latestFetchAt time.Time
	total := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry tcmbManifestEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		total++
		catCounts[entry.Category]++
		if entry.FetchedAt.After(catLatest[entry.Category]) {
			catLatest[entry.Category] = entry.FetchedAt
		}
		if entry.FetchedAt.After(latestFetchAt) {
			latestFetchAt = entry.FetchedAt
		}
	}
	if err := scanner.Err(); err != nil {
		return TCMBContextReport{
			ManifestPath: manifestPath,
			Summary:      "TCMB manifest okuma hatası: " + err.Error(),
			Warnings:     []string{"tcmb_manifest_scan_error"},
		}
	}

	if total == 0 {
		return TCMBContextReport{
			ManifestPath: manifestPath,
			Summary:      "TCMB manifest boş — henüz PDF indirilmemiş, `sync tcmb` komutunu çalıştırın.",
			Warnings:     []string{"tcmb_no_documents"},
		}
	}

	var cats []TCMBCategoryStat
	for id, count := range catCounts {
		cats = append(cats, TCMBCategoryStat{
			ID:            id,
			DocumentCount: count,
			LatestFetchAt: catLatest[id],
		})
	}
	sort.Slice(cats, func(i, j int) bool {
		return cats[i].ID < cats[j].ID
	})

	missing := []string{}
	for _, id := range requiredTCMBDocumentCategoryIDs() {
		if catCounts[id] == 0 {
			missing = append(missing, id)
		}
	}
	textSummary := loadTCMBTextIndex(filepath.Join(tcmbDir, "text_index.jsonl"))
	warnings := append([]string{}, textSummary.Warnings...)
	for _, id := range missing {
		warnings = append(warnings, "tcmb_category_missing:"+id)
	}
	textMissing := []string{}
	for _, id := range requiredTCMBDocumentCategoryIDs() {
		if textSummary.CategoryUsable[id] == 0 {
			textMissing = append(textMissing, id)
		}
	}
	for _, id := range textMissing {
		warnings = append(warnings, "tcmb_text_category_missing:"+id)
	}
	textCats := tcmbTextCategories(textSummary)
	computed := len(missing) == 0 && len(textMissing) == 0
	summary := fmt.Sprintf(
		"TCMB: %d belge, %d kategori; %d text kaydı, %d okunabilir text. Son belge: %s.",
		total,
		len(cats),
		textSummary.DocumentCount,
		textSummary.UsableCount,
		latestFetchAt.Format("2006-01-02"),
	)
	if len(missing) > 0 {
		summary = fmt.Sprintf("TCMB: %d belge var ancak zorunlu kategori eksik: %s.", total, strings.Join(missing, ", "))
	} else if textSummary.DocumentCount == 0 {
		summary = fmt.Sprintf("TCMB: %d belge var ancak text index yok veya boş; `sync tcmb-extract` çalışmadan PDF/HTML kanıtı analizde kullanılamaz.", total)
	} else if len(textMissing) > 0 {
		summary = fmt.Sprintf("TCMB: %d belge ve %d text kaydı var ancak zorunlu kategorilerde okunabilir text eksik: %s.", total, textSummary.DocumentCount, strings.Join(textMissing, ", "))
	}

	return TCMBContextReport{
		Computed:                      computed,
		ManifestPath:                  manifestPath,
		TextIndexPath:                 textSummary.Path,
		DocumentCount:                 total,
		TextDocumentCount:             textSummary.DocumentCount,
		TextUsableCount:               textSummary.UsableCount,
		Categories:                    cats,
		TextCategories:                textCats,
		RequiredCategoriesMissing:     missing,
		RequiredTextCategoriesMissing: textMissing,
		LatestFetchAt:                 latestFetchAt,
		LatestTextExtractedAt:         textSummary.LatestExtractedAt,
		Summary:                       summary,
		Warnings:                      warnings,
	}
}

func loadTCMBTextIndex(path string) tcmbTextIndexSummary {
	summary := tcmbTextIndexSummary{
		Path:           path,
		CategoryCounts: map[string]int{},
		CategoryUsable: map[string]int{},
		CategoryLatest: map[string]time.Time{},
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		summary.Warnings = append(summary.Warnings, "tcmb_text_index_missing")
		return summary
	}
	if err != nil {
		summary.Warnings = append(summary.Warnings, "tcmb_text_index_read_error")
		return summary
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry tcmbTextIndexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			summary.Warnings = append(summary.Warnings, "tcmb_text_index_row_parse_error")
			continue
		}
		summary.DocumentCount++
		summary.CategoryCounts[entry.Category]++
		if entry.ExtractedAt.After(summary.CategoryLatest[entry.Category]) {
			summary.CategoryLatest[entry.Category] = entry.ExtractedAt
		}
		if entry.ExtractedAt.After(summary.LatestExtractedAt) {
			summary.LatestExtractedAt = entry.ExtractedAt
		}
		if strings.EqualFold(entry.Status, "ok") && entry.TextLength >= 200 {
			summary.UsableCount++
			summary.CategoryUsable[entry.Category]++
		}
	}
	if err := scanner.Err(); err != nil {
		summary.Warnings = append(summary.Warnings, "tcmb_text_index_scan_error")
	}
	return summary
}

func tcmbTextCategories(summary tcmbTextIndexSummary) []TCMBTextCategoryStat {
	out := make([]TCMBTextCategoryStat, 0, len(summary.CategoryCounts))
	for id, count := range summary.CategoryCounts {
		out = append(out, TCMBTextCategoryStat{
			ID:                      id,
			TextDocumentCount:       count,
			UsableTextDocumentCount: summary.CategoryUsable[id],
			LatestExtractedAt:       summary.CategoryLatest[id],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func defaultTCMBDirFromEquitiesDir(equitiesDir string) string {
	if strings.TrimSpace(equitiesDir) == "" {
		return filepath.Join("data", "macro", "tcmb")
	}
	return filepath.Join(filepath.Dir(filepath.Clean(equitiesDir)), "macro", "tcmb")
}
