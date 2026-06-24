package sirketler

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"hissebot/internal/domain"
	"hissebot/internal/storage"
)

var codeSplitPattern = regexp.MustCompile(`[,;/\n]+`)
var codeCleanPattern = regexp.MustCompile(`[^A-Z0-9._-]+`)

type Company struct {
	Code    string `json:"code"`
	RawCode string `json:"raw_code,omitempty"`
	Name    string `json:"name,omitempty"`
	City    string `json:"city,omitempty"`
	Auditor string `json:"auditor,omitempty"`
}

type ImportResult struct {
	XLSXUniqueCodes int
	Created         int
	SkippedExisting int
}

func ImportMissing(ctx context.Context, store *storage.EquityStore, file string) (ImportResult, error) {
	companies, err := ReadCompanies(file)
	if err != nil {
		return ImportResult{}, err
	}

	result := ImportResult{XLSXUniqueCodes: len(companies)}
	for _, company := range companies {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		existing, err := store.Load(company.Code)
		if err != nil {
			return result, err
		}
		if existing.Name != "" || existing.KAPInfo != nil || existing.BilancoInfo != nil || existing.Data != nil {
			result.SkippedExisting++
			continue
		}

		err = store.Save(&domain.Equity{
			Ticker:    company.Code,
			Name:      company.Name,
			AssetType: 2,
			External: map[string]any{
				"xlsx_company": company,
			},
		})
		if err != nil {
			return result, err
		}
		result.Created++
	}

	return result, nil
}

func ReadCompanies(file string) ([]Company, error) {
	reader, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	shared, err := readSharedStrings(&reader.Reader)
	if err != nil {
		return nil, err
	}
	sheet, err := openZipFile(&reader.Reader, "xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	defer sheet.Close()

	rows, err := readRows(sheet, shared)
	if err != nil {
		return nil, err
	}

	byCode := map[string]Company{}
	for _, row := range rows {
		rawCode := strings.TrimSpace(row["A"])
		name := strings.TrimSpace(row["B"])
		if rawCode == "" || rawCode == "Kod" || rawCode == "Şirketler Listesi" {
			continue
		}
		if len([]rune(rawCode)) == 1 && name == "" {
			continue
		}
		for _, code := range splitCodes(rawCode) {
			if len(code) == 1 && name == "" {
				continue
			}
			if _, exists := byCode[code]; exists {
				continue
			}
			byCode[code] = Company{
				Code:    code,
				RawCode: rawCode,
				Name:    name,
				City:    strings.TrimSpace(row["C"]),
				Auditor: strings.TrimSpace(row["D"]),
			}
		}
	}

	companies := make([]Company, 0, len(byCode))
	for _, company := range byCode {
		companies = append(companies, company)
	}
	sort.Slice(companies, func(i, j int) bool {
		return companies[i].Code < companies[j].Code
	})
	return companies, nil
}

func splitCodes(input string) []string {
	input = strings.ToUpper(strings.TrimSpace(input))
	parts := codeSplitPattern.Split(input, -1)
	codes := make([]string, 0, len(parts))
	for _, part := range parts {
		code := codeCleanPattern.ReplaceAllString(strings.TrimSpace(part), "")
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func readSharedStrings(reader *zip.Reader) ([]string, error) {
	file, err := openZipFile(reader, "xl/sharedStrings.xml")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	var values []string
	var inSI bool
	var current strings.Builder
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "si" {
				inSI = true
				current.Reset()
			}
			if inSI && t.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &t); err != nil {
					return nil, err
				}
				current.WriteString(text)
			}
		case xml.EndElement:
			if t.Name.Local == "si" {
				values = append(values, current.String())
				inSI = false
			}
		}
	}
	return values, nil
}

func readRows(reader io.Reader, shared []string) ([]map[string]string, error) {
	decoder := xml.NewDecoder(reader)
	var rows []map[string]string
	var current map[string]string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				current = map[string]string{}
			case "c":
				if current == nil {
					continue
				}
				ref := attr(t, "r")
				cellType := attr(t, "t")
				value, err := readCell(decoder, t, cellType, shared)
				if err != nil {
					return nil, err
				}
				if ref != "" {
					current[columnLetters(ref)] = strings.TrimSpace(value)
				}
			}
		case xml.EndElement:
			if t.Name.Local == "row" && current != nil {
				rows = append(rows, current)
				current = nil
			}
		}
	}
	return rows, nil
}

func readCell(decoder *xml.Decoder, start xml.StartElement, cellType string, shared []string) (string, error) {
	var value string
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "v" || t.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &t); err != nil {
					return "", err
				}
				value += text
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if cellType == "s" && value != "" {
					index := atoi(value)
					if index >= 0 && index < len(shared) {
						return shared[index], nil
					}
				}
				return value, nil
			}
		}
	}
}

func openZipFile(reader *zip.Reader, name string) (io.ReadCloser, error) {
	for _, file := range reader.File {
		if file.Name == name {
			return file.Open()
		}
	}
	return nil, os.ErrNotExist
}

func attr(element xml.StartElement, name string) string {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func columnLetters(ref string) string {
	var b strings.Builder
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func atoi(value string) int {
	n := 0
	for _, r := range strings.TrimSpace(value) {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func (r ImportResult) String() string {
	return fmt.Sprintf("xlsx codes=%d created=%d skipped_existing=%d", r.XLSXUniqueCodes, r.Created, r.SkippedExisting)
}
