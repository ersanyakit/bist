package kapingest

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var tickerTokenPattern = regexp.MustCompile(`^[A-Z][A-Z0-9._-]{1,9}$`)
var filenameTickerPattern = regexp.MustCompile(`(^|[^A-Z0-9])([A-Z][A-Z0-9]{2,8})([^A-Z0-9]|$)`)

var skippedPDFScanDirs = map[string]bool{
	"analysis":     true,
	"analyses":     true,
	"report":       true,
	"reports":      true,
	"processed":    true,
	"output":       true,
	"outputs":      true,
	"charts":       true,
	"grafikler":    true,
	"analiz":       true,
	"analizler":    true,
	"backups":      true,
	"_backup":      true,
	"node_modules": true,
}

var nonTickerPathParts = map[string]bool{
	"data":        true,
	"equities":    true,
	"input":       true,
	"output":      true,
	"tmp":         true,
	"kap":         true,
	"attachments": true,
	"financials":  true,
	"raw":         true,
	"tradingview": true,
	"analysis":    true,
	"reports":     true,
	"seed":        true,
	"cache":       true,
}

var filenameTickerExclusions = map[string]bool{
	"KAP":       true,
	"PDF":       true,
	"SPK":       true,
	"BDDK":      true,
	"KGK":       true,
	"IFRS":      true,
	"UFRS":      true,
	"SOLO":      true,
	"KONSOLIDE": true,
	"RAPOR":     true,
	"GENEL":     true,
	"KURUL":     true,
	"FAALIYET":  true,
	"FAALİYET":  true,
	"DENETIM":   true,
	"DENETİM":   true,
	"TUTANAK":   true,
	"ONAYLI":    true,
	"TADIL":     true,
	"TADİL":     true,
	"METNI":     true,
	"METNİ":     true,
	"DAVET":     true,
	"GUNDEM":    true,
	"GÜNDEM":    true,
	"HAZIRUN":   true,
}

func ScanPDFs(inputDir string, limit int) ([]PDFFile, error) {
	inputDir = filepath.Clean(inputDir)
	files := []PDFFile{}
	err := filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipPDFScanDir(inputDir, path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".pdf") {
			files = append(files, PDFFile{
				FilePath: path,
				FileName: d.Name(),
				Ticker:   ExtractTicker(inputDir, path),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return filepath.ToSlash(files[i].FilePath) < filepath.ToSlash(files[j].FilePath)
	})
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}

func shouldSkipPDFScanDir(inputDir, path, name string) bool {
	if filepath.Clean(path) == filepath.Clean(inputDir) {
		return false
	}
	return skippedPDFScanDirs[strings.ToLower(strings.TrimSpace(name))]
}

func ExtractTicker(inputDir string, filePath string) string {
	inputDir = filepath.Clean(inputDir)
	filePath = filepath.Clean(filePath)
	if ticker := tickerFromEquitiesPath(filePath); ticker != "" {
		return ticker
	}
	if base := filepath.Base(inputDir); isTickerPathPart(base) {
		return strings.ToUpper(base)
	}
	if rel, err := filepath.Rel(inputDir, filePath); err == nil && rel != "." {
		for _, part := range pathParts(rel) {
			if isTickerPathPart(part) {
				return strings.ToUpper(part)
			}
		}
	}
	return tickerFromFilename(filepath.Base(filePath))
}

func tickerFromEquitiesPath(path string) string {
	parts := pathParts(path)
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "equities") && isTickerPathPart(parts[i+1]) {
			return strings.ToUpper(parts[i+1])
		}
	}
	return ""
}

func pathParts(path string) []string {
	parts := strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func isTickerPathPart(part string) bool {
	part = strings.ToUpper(strings.TrimSpace(part))
	if part == "" || nonTickerPathParts[strings.ToLower(part)] {
		return false
	}
	if isNumeric(part) {
		return false
	}
	return tickerTokenPattern.MatchString(part)
}

func tickerFromFilename(name string) string {
	normalized := strings.ToUpper(turkishASCII(name))
	for _, match := range filenameTickerPattern.FindAllStringSubmatch(normalized, -1) {
		if len(match) < 3 {
			continue
		}
		candidate := strings.Trim(match[2], " ._-")
		if len(candidate) < 4 || len(candidate) > 8 || filenameTickerExclusions[candidate] || isNumeric(candidate) {
			continue
		}
		return candidate
	}
	return ""
}

func turkishASCII(value string) string {
	replacer := strings.NewReplacer(
		"İ", "I", "ı", "I", "Ş", "S", "ş", "S", "Ğ", "G", "ğ", "G",
		"Ü", "U", "ü", "U", "Ö", "O", "ö", "O", "Ç", "C", "ç", "C",
	)
	return replacer.Replace(value)
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
