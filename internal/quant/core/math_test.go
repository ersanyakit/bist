package core

import (
	"math"
	"testing"
	"time"
)

func TestNormalAndMatrixUtilities(t *testing.T) {
	if math.Abs(NormalCDF(0)-0.5) > 1e-12 {
		t.Fatalf("NormalCDF(0) mismatch")
	}
	if math.Abs(NormalInvCDF(0.975)-1.959963986) > 1e-6 {
		t.Fatalf("NormalInvCDF(0.975) mismatch")
	}
	inv, err := MatrixInverse([][]float64{{4, 7}, {2, 6}})
	if err != nil {
		t.Fatalf("MatrixInverse: %v", err)
	}
	if math.Abs(inv[0][0]-0.6) > 1e-12 || math.Abs(inv[1][1]-0.4) > 1e-12 {
		t.Fatalf("unexpected inverse: %+v", inv)
	}
}

func TestGenerateScheduleAdjustsBusinessDays(t *testing.T) {
	start := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	schedule := GenerateSchedule(start, end, 3, ModifiedFollowing)
	if len(schedule) != 2 {
		t.Fatalf("schedule length=%d", len(schedule))
	}
	if schedule[0].Weekday() == time.Saturday || schedule[0].Weekday() == time.Sunday {
		t.Fatalf("business day was not adjusted: %s", schedule[0])
	}
}
