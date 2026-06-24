// internal/excel/reader.go
package excel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"hissebot/internal/ta/ohlcv"

	"github.com/xuri/excelize/v2"
)

type Company struct {
	Symbol      string `json:"symbol"`
	CompanyName string `json:"company_name"`
	Market      string `json:"market"`
	Sector      string `json:"sector"`
	ISIN        string `json:"isin"`
	Currency    string `json:"currency"`
}

var ErrMissingColumn = errors.New("missing required excel column")

func ReadCompanies(ctx context.Context, path string) ([]Company, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled before opening excel: %w", err)
	}
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open excel file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("excel workbook has no sheets: %w", excelize.ErrSheetNameBlank)
	}

	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("read rows from first sheet: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("excel sheet is empty: %w", excelize.ErrSheetNameBlank)
	}

	headers, headerRow, err := findHeaderRow(rows)
	if err != nil {
		return nil, err
	}
	symbolIdx, ok := headers["symbol"]
	if !ok {
		return nil, fmt.Errorf("missing required column symbol: %w", ErrMissingColumn)
	}
	nameIdx, ok := headers["company_name"]
	if !ok {
		return nil, fmt.Errorf("missing required column company_name: %w", ErrMissingColumn)
	}

	var companies []Company
	for rowIndex, row := range rows[headerRow+1:] {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled while reading row %d: %w", headerRow+rowIndex+2, err)
		}
		companyName := strings.TrimSpace(cell(row, nameIdx))
		if companyName == "" {
			continue
		}
		currency := cellByHeader(row, headers, "currency")
		if currency == "" {
			currency = "TRY"
		}
		for _, symbol := range splitSymbols(cell(row, symbolIdx)) {
			companies = append(companies, Company{
				Symbol:      symbol,
				CompanyName: companyName,
				Market:      cellByHeader(row, headers, "market"),
				Sector:      cellByHeader(row, headers, "sector"),
				ISIN:        cellByHeader(row, headers, "isin"),
				Currency:    strings.ToUpper(strings.TrimSpace(currency)),
			})
		}
	}
	return companies, nil
}

func findHeaderRow(rows [][]string) (map[string]int, int, error) {
	for rowIndex, row := range rows {
		headers := map[string]int{}
		for idx, header := range row {
			if key := canonicalHeader(header); key != "" {
				headers[key] = idx
			}
		}
		if _, ok := headers["symbol"]; !ok {
			continue
		}
		if _, ok := headers["company_name"]; !ok {
			continue
		}
		return headers, rowIndex, nil
	}
	return nil, -1, fmt.Errorf("missing required columns symbol and company_name: %w", ErrMissingColumn)
}

func canonicalHeader(value string) string {
	normalized := normalizeHeader(value)
	switch normalized {
	case "symbol", "ticker", "kod", "sembol", "hisse_kodu":
		return "symbol"
	case "company_name", "company", "name", "sirket_unvani", "sirket_adi", "firma_unvani", "unvan":
		return "company_name"
	case "market", "pazar":
		return "market"
	case "sector", "sektor":
		return "sector"
	case "isin":
		return "isin"
	case "currency", "para_birimi":
		return "currency"
	default:
		return normalized
	}
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		"ç", "c",
		"ğ", "g",
		"ı", "i",
		"ö", "o",
		"ş", "s",
		"ü", "u",
	).Replace(value)

	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if b.Len() > 0 && !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func splitSymbols(value string) []string {
	value = strings.NewReplacer(";", ",", "/", ",").Replace(value)
	parts := strings.Split(value, ",")
	symbols := make([]string, 0, len(parts))
	for _, part := range parts {
		symbol := ohlcv.NormalizeSymbol(part)
		if symbol == "" {
			continue
		}
		symbols = append(symbols, symbol)
	}
	return symbols
}

func cell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func cellByHeader(row []string, headers map[string]int, key string) string {
	idx, ok := headers[key]
	if !ok {
		return ""
	}
	return cell(row, idx)
}
