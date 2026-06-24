package ops

import "sync"

type Metrics struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
}

func NewMetrics() *Metrics {
	return &Metrics{counters: map[string]float64{}, gauges: map[string]float64{}}
}

func (m *Metrics) Inc(name string, delta float64) {
	if delta == 0 {
		delta = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += delta
}

func (m *Metrics) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

func (m *Metrics) Snapshot() map[string]float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]float64{}
	for key, value := range m.counters {
		out["counter."+key] = value
	}
	for key, value := range m.gauges {
		out["gauge."+key] = value
	}
	return out
}
