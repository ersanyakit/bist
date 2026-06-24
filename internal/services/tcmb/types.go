package tcmb

import "time"

const (
	DefaultBaseURL = "https://www.tcmb.gov.tr"
	ManifestFile   = "manifest.jsonl"
	FailuresFile   = "failures.jsonl"
	TextIndexFile  = "text_index.jsonl"
	DefaultDelay   = 800 * time.Millisecond
	DefaultTimeout = 45 * time.Second
	maxBodyBytes   = 64 << 20 // 64 MB
)

// Category defines a TCMB document category to scrape.
type Category struct {
	ID          string
	NameTR      string
	ListingPath string // relative path on tcmb.gov.tr
	SubPaths    []string
	FileExts    []string // e.g. [".pdf"]
	MaxDepth    int      // link-following depth
}

// DefaultCategories returns the categories relevant for stock analysis.
func DefaultCategories() []Category {
	return []Category{
		{
			ID:     "ppk_kararlari",
			NameTR: "PPK Para Politikası Kurulu Kararları",
			ListingPath: "/wps/wcm/connect/tr/tcmb%2Btr/main%2Bmenu/temel%2Bfaaliyetler/" +
				"para%2Bpolitikasi/ppk",
			FileExts: []string{".pdf", ".html"},
			MaxDepth: 2,
		},
		{
			ID:          "faiz_kararlari",
			NameTR:      "Faiz Kararları Duyuruları",
			ListingPath: "/wps/wcm/connect/tr/tcmb%2Btr/main%2Bmenu/duyurular/basin",
			FileExts:    []string{".pdf", ".html"},
			MaxDepth:    2,
		},
		{
			ID:          "basin_duyurulari",
			NameTR:      "Basın Duyuruları",
			ListingPath: "/wps/wcm/connect/tr/tcmb%2Btr/main%2Bmenu/duyurular/basin",
			FileExts:    []string{".pdf", ".html"},
			MaxDepth:    2,
		},
		{
			ID:     "enflasyon_raporu",
			NameTR: "Enflasyon Raporu",
			ListingPath: "/wps/wcm/connect/TR/TCMB+TR/Main+Menu/" +
				"Yayinlar/Raporlar/Enflasyon+Raporu/",
			FileExts: []string{".pdf"},
			MaxDepth: 3,
		},
		{
			ID:     "para_politikasi_metinleri",
			NameTR: "Para Politikası Metinleri",
			ListingPath: "/wps/wcm/connect/TR/TCMB+TR/Main+Menu/" +
				"Yayinlar/Para+Politikasi+Metinleri/",
			FileExts: []string{".pdf"},
			MaxDepth: 2,
		},
		{
			ID:     "yillik_rapor",
			NameTR: "Yıllık Rapor",
			ListingPath: "/wps/wcm/connect/TR/TCMB+TR/Main+Menu/" +
				"Yayinlar/Raporlar/Yillik+Rapor/",
			FileExts: []string{".pdf"},
			MaxDepth: 2,
		},
		{
			ID:     "finansal_istikrar_raporu",
			NameTR: "Finansal İstikrar Raporu",
			ListingPath: "/wps/wcm/connect/TR/TCMB+TR/Main+Menu/" +
				"Yayinlar/Raporlar/Finansal+Istikrar+Raporu/",
			FileExts: []string{".pdf"},
			MaxDepth: 2,
		},
		{
			ID:     "baskanlarin_konusmalari",
			NameTR: "Başkanların Konuşmaları",
			ListingPath: "/wps/wcm/connect/TR/TCMB%2BTR/Main%2BMenu/" +
				"Duyurular/Baskanin%2BKonusmalari",
			FileExts: []string{".pdf", ".html"},
			MaxDepth: 2,
		},
	}
}

// Options for the TCMB sync.
type Options struct {
	OutputDir  string
	BaseURL    string
	Categories []string // empty = all DefaultCategories
	Delay      time.Duration
	Timeout    time.Duration
	Limit      int // max downloads per run; 0 = unlimited
	Force      bool
	Verbose    bool
}

// SyncResult is returned after a sync run.
type SyncResult struct {
	OutputDir    string         `json:"output_dir"`
	ManifestPath string         `json:"manifest_path"`
	Categories   []string       `json:"categories"`
	Downloaded   int            `json:"downloaded"`
	Skipped      int            `json:"skipped"`
	Errors       int            `json:"errors"`
	ByCategory   []CategoryStat `json:"by_category"`
}

// CategoryStat reports per-category stats.
type CategoryStat struct {
	ID         string `json:"id"`
	Downloaded int    `json:"downloaded"`
	Skipped    int    `json:"skipped"`
	Errors     int    `json:"errors"`
}

// ManifestEntry is written to manifest.jsonl after each successful download.
type ManifestEntry struct {
	Category    string    `json:"category"`
	Title       string    `json:"title,omitempty"`
	URL         string    `json:"url"`
	LocalPath   string    `json:"local_path"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	ContentType string    `json:"content_type,omitempty"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// FailureEntry is written to failures.jsonl on error.
type FailureEntry struct {
	Category string    `json:"category"`
	URL      string    `json:"url"`
	Error    string    `json:"error"`
	FailedAt time.Time `json:"failed_at"`
}

// DocumentTextExtractOptions controls conversion of downloaded TCMB documents into text.
type DocumentTextExtractOptions struct {
	OutputDir  string
	Categories []string
	Timeout    time.Duration
	Limit      int
	Force      bool
}

// DocumentTextExtractResult summarizes TCMB document text extraction.
type DocumentTextExtractResult struct {
	OutputDir     string             `json:"output_dir"`
	TextIndexPath string             `json:"text_index_path"`
	Documents     int                `json:"documents"`
	Extracted     int                `json:"extracted"`
	Skipped       int                `json:"skipped"`
	Errors        int                `json:"errors"`
	ByCategory    []TextCategoryStat `json:"by_category,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
}

// TextCategoryStat reports per-category text extraction stats.
type TextCategoryStat struct {
	ID        string `json:"id"`
	Documents int    `json:"documents"`
	Extracted int    `json:"extracted"`
	Skipped   int    `json:"skipped"`
	Errors    int    `json:"errors"`
}

// DocumentTextIndexEntry is written to text_index.jsonl for every manifest document.
type DocumentTextIndexEntry struct {
	Category    string    `json:"category"`
	Title       string    `json:"title,omitempty"`
	URL         string    `json:"url"`
	LocalPath   string    `json:"local_path"`
	TextPath    string    `json:"text_path,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	Status      string    `json:"status"`
	TextLength  int       `json:"text_length,omitempty"`
	ExtractedAt time.Time `json:"extracted_at"`
	Warnings    []string  `json:"warnings,omitempty"`
}
