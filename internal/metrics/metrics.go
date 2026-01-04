package metrics

import (
	"sort"
	"sync"
	"time"
)

type Metrics struct {
	mutex         sync.RWMutex
	requests      map[string]int64
	selections    map[string]int64
	responseTimes map[string][]time.Duration
	statusCodes   map[string]map[int]int64
	healthStatus  map[string]bool
	startTime     time.Time
}

type Snapshot struct {
	TotalRequests int64                     `json:"total_requests"`
	Uptime        time.Duration             `json:"uptime"`
	Backends      map[string]BackendMetrics `json:"backends"`
	Algorithm     string                    `json:"algorithm"`
}

type BackendMetrics struct {
	Requests    int64         `json:"requests"`
	Selections  int64         `json:"selections"`
	Healthy     bool          `json:"healthy"`
	AvgResponse time.Duration `json:"avg_response"`
	P50Response time.Duration `json:"p50_response"`
	P95Response time.Duration `json:"p95_response"`
	P99Response time.Duration `json:"p99_response"`
	StatusCodes map[int]int64 `json:"status_codes"`
}

func (m *Metrics) IncrementRequests(backend string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.requests[backend]++
}

func (m *Metrics) RecordBackendSelection(backend string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.selections[backend]++
}

func (m *Metrics) RecordResponse(backend string, duration time.Duration, statusCode int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.responseTimes[backend] = append(m.responseTimes[backend], duration)

	if len(m.responseTimes[backend]) > 1000 {
		m.responseTimes[backend] = m.responseTimes[backend][1:]
	}

	if m.statusCodes[backend] == nil {
		m.statusCodes[backend] = make(map[int]int64)
	}
	m.statusCodes[backend][statusCode]++
}

func (m *Metrics) UpdateHealthStatus(backend string, healthy bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.healthStatus[backend] = healthy
}

func (m *Metrics) Snapshot(algorithm string) Snapshot {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	snap := Snapshot{
		Uptime:    time.Since(m.startTime),
		Backends:  make(map[string]BackendMetrics),
		Algorithm: algorithm,
	}

	// Collect all unique backend URLs
	allBackends := make(map[string]bool)
	for backend := range m.requests {
		allBackends[backend] = true
	}
	for backend := range m.selections {
		allBackends[backend] = true
	}
	for backend := range m.responseTimes {
		allBackends[backend] = true
	}
	for backend := range m.healthStatus {
		allBackends[backend] = true
	}

	for backend := range allBackends {
		snap.TotalRequests += m.requests[backend]

		bm := BackendMetrics{
			Requests:    m.requests[backend],
			Selections:  m.selections[backend],
			Healthy:     m.healthStatus[backend],
			StatusCodes: m.statusCodes[backend],
		}

		durations := m.responseTimes[backend]
		if len(durations) > 0 {
			sorted := make([]time.Duration, len(durations))
			copy(sorted, durations)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i] < sorted[j]
			})

			bm.AvgResponse = average(sorted)
			bm.P50Response = percentile(sorted, 0.50)
			bm.P95Response = percentile(sorted, 0.95)
			bm.P99Response = percentile(sorted, 0.99)
		}

		snap.Backends[backend] = bm
	}

	return snap
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests:      make(map[string]int64),
		selections:    make(map[string]int64),
		responseTimes: make(map[string][]time.Duration),
		statusCodes:   make(map[string]map[int]int64),
		healthStatus:  make(map[string]bool),
		startTime:     time.Now(),
	}
}

func average(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}

	return sum / time.Duration(len(durations))
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}

	index := int(float64(len(sorted)) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}
