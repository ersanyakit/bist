package validation

import (
	"testing"
	"time"
)

func TestWalkForwardWindowsMaintainChronology(t *testing.T) {
	from, _ := time.Parse("2006-01-02", "2020-01-01")
	to, _ := time.Parse("2006-01-02", "2026-06-19")
	windows := GenerateWindows(WalkForwardConfig{From: from, To: to, TrainYears: 5, ValidationMonths: 3, TestMonths: 3, EmbargoDays: 1})
	if len(windows) == 0 {
		t.Fatal("expected windows")
	}
	for _, w := range windows {
		if !w.TrainTo.Before(w.ValidationFrom) || !w.ValidationTo.Before(w.TestFrom) {
			t.Fatalf("window chronology broken: %+v", w)
		}
	}
}

func TestEmbargoRemovesOverlappingDates(t *testing.T) {
	base, _ := time.Parse("2006-01-02", "2026-06-10")
	train := []time.Time{base, base.AddDate(0, 0, 1), base.AddDate(0, 0, 5)}
	validation := DateRange{From: base.AddDate(0, 0, 1), To: base.AddDate(0, 0, 2)}
	got := ApplyEmbargo(train, validation, 1)
	if len(got) != 1 || !got[0].Equal(base.AddDate(0, 0, 5)) {
		t.Fatalf("embargo result=%v", got)
	}
}
