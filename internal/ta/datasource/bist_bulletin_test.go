package datasource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"hissebot/internal/ta/ohlcv"
)

func TestBISTBulletinProviderReadsCSVAndXLSXArchiveSorted(t *testing.T) {
	root := t.TempDir()
	writeBISTBulletinCSVFixture(t, root, "20260618", []string{
		"ASELS.E", "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.", "YILDIZ PAZAR", "PAY PIYASASI",
		"PAY", "PAY", "SUREKLI ISLEM", "400.00", "400.50", "400.50", "398.00", "405.00", "401.00", "401.00",
		"0.250", "401.00", "401.25", "402.000", "1000000", "2500", "120", "0", "10000", "25", "2", "20000", "50", "3",
	})
	writeBISTBulletinXLSXFixture(t, root, "20260619", []string{
		"ASELS.E", "ASELSAN ELEKTRONIK SANAYI VE TICARET A.S.", "YILDIZ PAZAR", "PAY PIYASASI",
		"PAY", "PAY", "SUREKLI ISLEM", "410.75", "408.75", "408.75", "400.50", "412.75", "402.50", "402.50",
		"-2.009", "402.50", "402.75", "406.533", "7825864449.25", "19250277", "73994", "0", "55289931.25", "135635", "548", "647222927.75", "1594351", "5911",
	})

	provider := NewBISTBulletinProvider(root)
	records, err := provider.FetchDailyBulletinRecords(context.Background(), "ASELS", 0)
	if err != nil {
		t.Fatalf("FetchDailyBulletinRecords failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records len=%d, want 2", len(records))
	}
	if records[0].TradingDate != "2026-06-18" || records[1].TradingDate != "2026-06-19" {
		t.Fatalf("records not sorted ascending: %+v", records)
	}
	if records[0].SourceFormat != "csv" || records[1].SourceFormat != "xlsx" {
		t.Fatalf("unexpected source formats: %s, %s", records[0].SourceFormat, records[1].SourceFormat)
	}
	if records[1].Open != 408.75 || records[1].Close != 402.50 || records[1].OpeningSessionVolume != 135635 {
		t.Fatalf("xlsx fields not parsed: %+v", records[1])
	}

	latest, err := provider.FetchDailyBulletinRecords(context.Background(), "ASELS", 1)
	if err != nil {
		t.Fatalf("limited records failed: %v", err)
	}
	if len(latest) != 1 || latest[0].TradingDate != "2026-06-19" {
		t.Fatalf("limit should return latest record sorted ascending: %+v", latest)
	}

	oneDay, err := provider.FetchDailyBulletinRecordsRange(context.Background(), "ASELS", "2026-06-19", "2026-06-19", 0)
	if err != nil {
		t.Fatalf("date range records failed: %v", err)
	}
	if len(oneDay) != 1 || oneDay[0].TradingDate != "2026-06-19" || oneDay[0].SourceFormat != "xlsx" {
		t.Fatalf("date range should return only 2026-06-19 xlsx record: %+v", oneDay)
	}

	candles, err := provider.FetchOHLCV(context.Background(), ohlcv.Instrument{
		Symbol:    "ASELS",
		AssetType: ohlcv.AssetTypeEquity,
	}, "1D", 0)
	if err != nil {
		t.Fatalf("FetchOHLCV failed: %v", err)
	}
	if len(candles) != 2 {
		t.Fatalf("candles len=%d, want 2", len(candles))
	}
	if candles[1].Open != 408.75 || candles[1].High != 412.75 || candles[1].Low != 400.50 || candles[1].Close != 402.50 {
		t.Fatalf("xlsx candle not parsed: %+v", candles[1])
	}
}

func writeBISTBulletinCSVFixture(t *testing.T, root, yyyymmdd string, row []string) {
	t.Helper()
	dir := filepath.Join(root, "bulten_verileri", yyyymmdd[:4], yyyymmdd[4:6], yyyymmdd+"_s1", "extracted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := stringsJoin(bistBulletinFixtureHeader(), ";") + "\n" +
		stringsJoin(bistBulletinFixtureEnglishHeader(), ";") + "\n" +
		stringsJoin(row, ";") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "thb"+yyyymmdd+"1.csv"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBISTBulletinXLSXFixture(t *testing.T, root, yyyymmdd string, row []string) {
	t.Helper()
	dir := filepath.Join(root, "bulten_verileri", yyyymmdd[:4], yyyymmdd[4:6], yyyymmdd+"_s1", "extracted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(file.GetActiveSheetIndex())
	header := bistBulletinFixtureHeader()
	english := bistBulletinFixtureEnglishHeader()
	if err := file.SetSheetRow(sheet, "A1", &header); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow(sheet, "A2", &english); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow(sheet, "A3", &row); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(filepath.Join(dir, "thb"+yyyymmdd+"1.xlsx")); err != nil {
		t.Fatal(err)
	}
}

func bistBulletinFixtureHeader() []string {
	return []string{
		"ISLEM KODU", "BULTEN ADI", "PAZAR GRUBU", "PAZAR", "ENSTRUMAN GRUBU", "ENSTRUMAN TIPI", "ISLEM YONTEMI",
		"ONCEKI KAPANIS FIYATI", "ACILIS FIYATI", "ACILIS SEANSI FIYATI", "EN DUSUK FIYAT", "EN YUKSEK FIYAT",
		"KAPANIS FIYATI", "KAPANIS SEANSI FIYATI", "DEGISIM (%)", "BEKLEYEN EN IYI ALIS", "BEKLEYEN EN IYI SATIS",
		"A.O.F", "TOPLAM ISLEM HACMI", "TOPLAM ISLEM ADEDI", "TOPLAM SOZLESME SAYISI", "REFERANS FIYAT",
		"ACILIS SEANSI ISLEM HACMI", "ACILIS SEANSI ISLEM MIKTARI", "ACILIS SEANSI SOZLESME SAYISI",
		"KAPANIS SEANSI ISLEM HACMI", "KAPANIS SEANSI ISLEM MIKTARI", "KAPANIS SEANSI SOZLESME SAYISI",
	}
}

func bistBulletinFixtureEnglishHeader() []string {
	return []string{
		"INSTRUMENT SERIES CODE", "INSTRUMENT NAME", "MARKET SUB SEGMENT", "MARKET SEGMENT", "INSTRUMENT GROUP",
		"INSTRUMENT TYPE", "TRADING METHOD", "PREVIOUS LAST PRICE", "OPENING PRICE", "OPENING SESSION PRICE",
		"LOWEST PRICE", "HIGHEST PRICE", "CLOSING PRICE", "CLOSING SESSION PRICE", "CHANGE TO PREVIOUS CLOSING (%)",
		"REMAINING BID", "REMAINING ASK", "VWAP", "TOTAL TRADED VALUE", "TOTAL TRADED VOLUME",
		"TOTAL NUMBER OF CONTRACTS", "REFERENCE PRICE", "TRADED VALUE AT OPENING SESSION",
		"TRADED VOLUME AT OPENING SESSION", "NUMBER OF CONTRACTS AT OPENING SESSION", "TRADED VALUE AT CLOSING SESSION",
		"TRADED VOLUME AT CLOSING SESSION", "NUMBER OF CONTRACTS AT CLOSING SESSION",
	}
}

func stringsJoin(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += sep + value
	}
	return out
}
