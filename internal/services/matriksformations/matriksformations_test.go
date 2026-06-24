package matriksformations

import (
	"net/url"
	"testing"
	"time"
)

const formationFixture = `
<html>
<body>
<form method="post" action="./Default.aspx">
  <input type="hidden" name="__VIEWSTATE" value="view-state" />
  <input type="hidden" name="__EVENTVALIDATION" value="event-validation" />
  <input type="radio" name="RBL_formStatus" value="1" checked="checked" />
  <input type="radio" name="RBL_formStatus" value="0" />
  <select name="DDL_index">
    <option value="-1">Tum Piyasalar</option>
    <option value="4" selected="selected">Bist Tum</option>
  </select>
  <select name="DDL_periodType">
    <option value="-1" selected="selected">Tum</option>
    <option value="D">Gunluk</option>
  </select>
  <table>
    <tr id="LV_formations__itemrow_0">
      <td><span id="LV_formations_SembolLabel_0">ASELS</span></td>
      <td><span id="LV_formations_PeriodTypeLabel_0">Gunluk</span></td>
      <td><a href="Graph.aspx?ID=123"><span id="LV_formations_formationTypeLabel_0">Triangles Asc.</span></a></td>
      <td><img id="LV_formations_directionArrow_0" alt="Artis" /></td>
      <td id="LV_formations_statusTD_0">Aktif</td>
      <td><span id="LV_formations_priceDifferencePercentLabel_0">%12,50</span></td>
      <td><span id="LV_formations_maxPossibleGainLabel_0">%18,25</span></td>
      <td><span id="LV_formations_confirmationDateLabel_0">18/06/2026 10:30:00</span></td>
      <td><span id="LV_formations_strengthLabel_0">83,4</span></td>
      <td><span id="LV_formations_maxLineErrLabel_0">0,35</span></td>
      <td><span id="LV_formations_updateDateLabel_0">18/06/2026 11:00:00</span></td>
    </tr>
  </table>
  <input type="submit" name="DP_formations$ctl02$ctl00" value="Sonraki" />
</form>
</body>
</html>`

func TestParseFormValues(t *testing.T) {
	values, err := parseFormValues(formationFixture)
	if err != nil {
		t.Fatalf("parseFormValues() error = %v", err)
	}
	assertValue(t, values, "__VIEWSTATE", "view-state")
	assertValue(t, values, "__EVENTVALIDATION", "event-validation")
	assertValue(t, values, "RBL_formStatus", "1")
	assertValue(t, values, "DDL_index", "4")
	assertValue(t, values, "DDL_periodType", "-1")
}

func TestParseFormationRecords(t *testing.T) {
	fetchedAt := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	records, err := parseFormationRecords(formationFixture, "active", "4", DefaultMarketLabel, DefaultURL, 2, fetchedAt)
	if err != nil {
		t.Fatalf("parseFormationRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	record := records[0]
	if record.Symbol != "ASELS" || record.Ticker != "ASELS" {
		t.Fatalf("record symbol = %q ticker = %q, want ASELS", record.Symbol, record.Ticker)
	}
	if record.Set != "active" || record.Timeframe != "1D" || record.Direction != "bullish" {
		t.Fatalf("unexpected set/timeframe/direction: %#v", record)
	}
	if record.FormationType != "Triangles Asc." || record.CanonicalPatternName != "Ascending Triangle" {
		t.Fatalf("unexpected formation names: %#v", record)
	}
	if record.GraphID != "123" || record.GraphURL == "" {
		t.Fatalf("graph fields not parsed: id=%q url=%q", record.GraphID, record.GraphURL)
	}
	if record.PriceDifferencePercent == nil || *record.PriceDifferencePercent != 12.5 {
		t.Fatalf("price difference = %#v, want 12.5", record.PriceDifferencePercent)
	}
	if record.ConfirmationAt == nil || record.UpdatedAt == nil {
		t.Fatalf("expected parsed Matriks dates: %#v", record)
	}
	if record.SourcePage != 2 || record.SourceRow != 0 || !record.FetchedAt.Equal(fetchedAt) {
		t.Fatalf("source metadata mismatch: %#v", record)
	}
}

func TestHasNextPage(t *testing.T) {
	if !hasNextPage(formationFixture) {
		t.Fatal("hasNextPage() = false, want true")
	}
	disabled := `<input type="submit" name="DP_formations$ctl02$ctl00" value="Sonraki" disabled="disabled" />`
	if hasNextPage(disabled) {
		t.Fatal("hasNextPage(disabled) = true, want false")
	}
}

func assertValue(t *testing.T, values url.Values, key, want string) {
	t.Helper()
	if got := values.Get(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
