package sirketler

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitCodesNormalizesDelimitedCompanyCodes(t *testing.T) {
	got := splitCodes(" eupwr, asels / thyao\nabc.def ")
	want := []string{"EUPWR", "ASELS", "THYAO", "ABC.DEF"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCodes() = %#v, want %#v", got, want)
	}
}

func TestReadRowsResolvesSharedStringsAndInlineValues(t *testing.T) {
	xml := `<worksheet><sheetData><row><c r="A1" t="s"><v>0</v></c><c r="B1" t="inlineStr"><is><t>Europower</t></is></c><c r="C1"><v>Ankara</v></c></row></sheetData></worksheet>`

	rows, err := readRows(strings.NewReader(xml), []string{"EUPWR"})
	if err != nil {
		t.Fatalf("readRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("readRows() returned %d rows, want 1", len(rows))
	}
	if rows[0]["A"] != "EUPWR" || rows[0]["B"] != "Europower" || rows[0]["C"] != "Ankara" {
		t.Fatalf("readRows() row = %#v", rows[0])
	}
}

func TestColumnLettersAtoiAndImportResultString(t *testing.T) {
	if got := columnLetters("AB123"); got != "AB" {
		t.Fatalf("columnLetters() = %q, want AB", got)
	}
	if got := atoi("123"); got != 123 {
		t.Fatalf("atoi(valid) = %d, want 123", got)
	}
	if got := atoi("12x"); got != -1 {
		t.Fatalf("atoi(invalid) = %d, want -1", got)
	}
	if got := (ImportResult{XLSXUniqueCodes: 3, Created: 2, SkippedExisting: 1}).String(); got != "xlsx codes=3 created=2 skipped_existing=1" {
		t.Fatalf("ImportResult.String() = %q", got)
	}
}
