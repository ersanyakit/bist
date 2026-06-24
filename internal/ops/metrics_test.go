package ops

import "testing"

func TestMetricsSnapshotIncludesCountersAndGauges(t *testing.T) {
	m := NewMetrics()
	m.Inc("jobs", 1)
	m.Inc("jobs", 2)
	m.SetGauge("queue_depth", 7)
	snapshot := m.Snapshot()
	if snapshot["counter.jobs"] != 3 {
		t.Fatalf("counter.jobs = %.2f", snapshot["counter.jobs"])
	}
	if snapshot["gauge.queue_depth"] != 7 {
		t.Fatalf("gauge.queue_depth = %.2f", snapshot["gauge.queue_depth"])
	}
}
