package bistbulletindb

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestSyncDownloadsRemoteTHBZipAndImportsSQLite(t *testing.T) {
	day := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2026/06/thb202606241.zip" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(thbCSVZipFixture(t, "thb202606241.csv"))
	}))
	defer server.Close()

	root := t.TempDir()
	report, err := Sync(context.Background(), Options{
		DBPath:   filepath.Join(root, "bist_ohlcv.sqlite"),
		RawRoot:  filepath.Join(root, "raw", "bulten_verileri"),
		BaseURL:  server.URL,
		FromDate: day,
		ToDate:   day,
		Session:  1,
		Download: true,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if report.RemoteDownloaded != 1 || report.LocalSourcesImported != 1 || report.RowsStored != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.DatabaseCandles != 1 || report.DatabaseSymbols != 1 {
		t.Fatalf("unexpected db counts: %+v", report)
	}
	extracted := filepath.Join(root, "raw", "bulten_verileri", "2026", "06", "20260624_s1", "extracted", "thb202606241.csv")
	if _, err := os.Stat(extracted); err != nil {
		t.Fatalf("extracted THB file missing: %v", err)
	}
}

func TestScanAndParseXLSXBulletinFile(t *testing.T) {
	root := t.TempDir()
	path := writeBISTBulletinDBXLSXFixture(t, root, "20260619", []string{
		"ASELS.E", "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.", "YILDIZ PAZAR", "PAY PIYASASI",
		"PAY", "PAY", "SUREKLI ISLEM", "410.75", "408.75", "408.75", "400.50", "412.75", "402.50", "402.50",
		"-2.009", "402.50", "402.75", "406.533", "7825864449.25", "19250277", "73994",
	})

	files, err := scanBulletinFiles(root, 1, 0, 0)
	if err != nil {
		t.Fatalf("scanBulletinFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len=%d, want 1", len(files))
	}
	if files[0].path != path || files[0].format != "xlsx" || files[0].session != 1 {
		t.Fatalf("unexpected file entry: %+v", files[0])
	}

	source, records, err := parseBulletinFile(files[0], time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseBulletinFile failed: %v", err)
	}
	if source.SourceFormat != "xlsx" || source.RowsSeen != 1 || source.RowsStored != 1 || source.RowsAnalysisReady != 1 {
		t.Fatalf("unexpected source result: %+v", source)
	}
	if len(records) != 1 {
		t.Fatalf("records len=%d, want 1", len(records))
	}
	rec := records[0]
	if rec.Symbol != "ASELS" || rec.Close != 402.50 || rec.High != 412.75 || rec.Low != 400.50 {
		t.Fatalf("unexpected record prices: %+v", rec)
	}
	if rec.Open == nil || *rec.Open != 408.75 {
		t.Fatalf("unexpected open: %+v", rec.Open)
	}
	if rec.PreviousClose != 410.75 || rec.ValueTraded != 7825864449.25 || rec.TradeCount != 73994 || rec.VWAP != 406.533 {
		t.Fatalf("modern THB fields not parsed: %+v", rec)
	}
	if rec.InstrumentCode != "ASELS.E" || rec.CompanyName == "" || rec.Market != "PAY PIYASASI" {
		t.Fatalf("instrument fields not parsed: %+v", rec)
	}
}

func thbCSVZipFixture(t *testing.T, filename string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"ISLEM KODU;BULTEN ADI;PAZAR;ONCEKI KAPANIS FIYATI;ACILIS FIYATI;EN DUSUK FIYAT;EN YUKSEK FIYAT;KAPANIS FIYATI;TOPLAM ISLEM ADEDI;TOPLAM ISLEM HACMI;TOPLAM SOZLESME SAYISI;A.O.F",
		"INSTRUMENT SERIES CODE;INSTRUMENT NAME;MARKET SEGMENT;PREVIOUS LAST PRICE;OPENING PRICE;LOWEST PRICE;HIGHEST PRICE;CLOSING PRICE;TOTAL TRADED VOLUME;TOTAL TRADED VALUE;TOTAL NUMBER OF CONTRACTS;VWAP",
		"ASELS.E;ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.;PAY PIYASASI;400.00;401.00;399.50;405.00;404.25;123456;50000000;3210;403.00",
		"",
	}, "\n")
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeBISTBulletinDBXLSXFixture(t *testing.T, root, yyyymmdd string, row []string) string {
	t.Helper()
	dir := filepath.Join(root, "bulten_verileri", yyyymmdd[:4], yyyymmdd[4:6], yyyymmdd+"_s1", "extracted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(file.GetActiveSheetIndex())
	header := []string{
		"ISLEM KODU", "BULTEN ADI", "PAZAR GRUBU", "PAZAR", "ENSTRUMAN GRUBU", "ENSTRUMAN TIPI", "ISLEM YONTEMI",
		"ONCEKI KAPANIS FIYATI", "ACILIS FIYATI", "ACILIS SEANSI FIYATI", "EN DUSUK FIYAT", "EN YUKSEK FIYAT",
		"KAPANIS FIYATI", "KAPANIS SEANSI FIYATI", "DEGISIM (%)", "BEKLEYEN EN IYI ALIS", "BEKLEYEN EN IYI SATIS",
		"A.O.F", "TOPLAM ISLEM HACMI", "TOPLAM ISLEM ADEDI", "TOPLAM SOZLESME SAYISI",
	}
	english := []string{
		"INSTRUMENT SERIES CODE", "INSTRUMENT NAME", "MARKET SUB SEGMENT", "MARKET SEGMENT", "INSTRUMENT GROUP",
		"INSTRUMENT TYPE", "TRADING METHOD", "PREVIOUS LAST PRICE", "OPENING PRICE", "OPENING SESSION PRICE",
		"LOWEST PRICE", "HIGHEST PRICE", "CLOSING PRICE", "CLOSING SESSION PRICE", "CHANGE TO PREVIOUS CLOSING (%)",
		"REMAINING BID", "REMAINING ASK", "VWAP", "TOTAL TRADED VALUE", "TOTAL TRADED VOLUME", "TOTAL NUMBER OF CONTRACTS",
	}
	if err := file.SetSheetRow(sheet, "A1", &header); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow(sheet, "A2", &english); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow(sheet, "A3", &row); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "thb"+yyyymmdd+"1.xlsx")
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBulletinDateFromTHBFileSupportsCSVAndXLSX(t *testing.T) {
	for _, base := range []string{"thb202606191.csv", "thb202606191.xlsx"} {
		t.Run(base, func(t *testing.T) {
			date, session, ok := bulletinDateFromTHBFile("", base)
			if !ok {
				t.Fatalf("date not parsed")
			}
			if date.Format("2006-01-02") != "2026-06-19" || session != 1 {
				t.Fatalf("unexpected date/session: %s s%d", date.Format("2006-01-02"), session)
			}
		})
	}
}

func TestParseXLSXAllRowsSkipsNonEquitySuffixes(t *testing.T) {
	root := t.TempDir()
	path := writeBISTBulletinDBXLSXFixture(t, root, "20260619", []string{
		"ASELS.V", "ASELS VARANT", "YAPILANDIRILMIS URUNLER", "PAY PIYASASI",
		"VARANT", "VARANT", "SUREKLI ISLEM", "1", "1", "1", "1", "1", "1", "1",
		"0", "1", "1", "1", "1", "1", "1",
	})
	records, seen, err := parseXLSXAllRows(path, time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseXLSXAllRows failed: %v", err)
	}
	if seen != 1 {
		t.Fatalf("seen=%d, want 1", seen)
	}
	if len(records) != 0 {
		t.Fatalf("warrant row should be skipped: %+v", records)
	}
}

func TestParseXLSXAllRowsRequiresRecognizedHeader(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bulten_verileri", "2026", "06", "20260619_s1", "extracted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(file.GetActiveSheetIndex())
	row := []string{"NOT", "A", "BIST", "HEADER"}
	if err := file.SetSheetRow(sheet, "A1", &row); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "thb202606191.xlsx")
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_, _, err := parseXLSXAllRows(path, time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "unrecognized XLSX header") {
		t.Fatalf("unexpected error: %v", err)
	}
}
