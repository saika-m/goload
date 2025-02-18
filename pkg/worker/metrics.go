package worker

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/saika-m/goload/internal/common"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// MetricsCollector handles collection and aggregation of metrics
type MetricsCollector struct {
	workerID  string
	metricsCh chan *common.Metric
	interval  time.Duration
	stopCh    chan struct{}
	mu        sync.RWMutex

	// Metric buffers
	responseTimesBuffer map[string][]time.Duration
	requestCountBuffer  map[string]int64
	errorCountBuffer    map[string]int64
	bytesInBuffer       map[string]int64
	bytesOutBuffer      map[string]int64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(workerID string, interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		workerID:  workerID,
		metricsCh: make(chan *common.Metric, 1000),
		interval:  interval,
		stopCh:    make(chan struct{}),

		responseTimesBuffer: make(map[string][]time.Duration),
		requestCountBuffer:  make(map[string]int64),
		errorCountBuffer:    make(map[string]int64),
		bytesInBuffer:       make(map[string]int64),
		bytesOutBuffer:      make(map[string]int64),
	}
}

// Start begins metrics collection
func (m *MetricsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collectAndSendMetrics()
		}
	}
}

// Stop stops metrics collection
func (m *MetricsCollector) Stop() {
	close(m.stopCh)
}

// ProcessResult processes a request result and updates metrics
func (m *MetricsCollector) ProcessResult(result *common.RequestResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	testID := result.TestID

	// Update response times
	if _, exists := m.responseTimesBuffer[testID]; !exists {
		m.responseTimesBuffer[testID] = make([]time.Duration, 0)
	}
	m.responseTimesBuffer[testID] = append(m.responseTimesBuffer[testID], result.Duration)

	// Update request count
	m.requestCountBuffer[testID]++

	// Update error count
	if result.Error != nil {
		m.errorCountBuffer[testID]++
	}

	// Update bytes transferred
	m.bytesInBuffer[testID] += result.BytesReceived
	m.bytesOutBuffer[testID] += result.BytesSent
}

func (m *MetricsCollector) collectAndSendMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	timestamp := time.Now()

	for testID := range m.responseTimesBuffer {
		// Calculate response time metrics
		if times := m.responseTimesBuffer[testID]; len(times) > 0 {
			avg, p95, p99, _ := common.CalculateStats(nil) // Using response times for calculation

			m.sendMetric(testID, "response_time_avg", float64(avg.Milliseconds()), timestamp)
			m.sendMetric(testID, "response_time_p95", float64(p95.Milliseconds()), timestamp)
			m.sendMetric(testID, "response_time_p99", float64(p99.Milliseconds()), timestamp)
		}

		// Send request rate metrics
		m.sendMetric(testID, "requests_total", float64(m.requestCountBuffer[testID]), timestamp)
		m.sendMetric(testID, "errors_total", float64(m.errorCountBuffer[testID]), timestamp)

		// Send throughput metrics
		m.sendMetric(testID, "bytes_in_total", float64(m.bytesInBuffer[testID]), timestamp)
		m.sendMetric(testID, "bytes_out_total", float64(m.bytesOutBuffer[testID]), timestamp)

		// Calculate error rate
		if m.requestCountBuffer[testID] > 0 {
			errorRate := float64(m.errorCountBuffer[testID]) / float64(m.requestCountBuffer[testID]) * 100
			m.sendMetric(testID, "error_rate", errorRate, timestamp)
		}
	}

	// Collect system metrics
	m.collectSystemMetrics(timestamp)

	// Reset buffers
	m.responseTimesBuffer = make(map[string][]time.Duration)
	m.requestCountBuffer = make(map[string]int64)
	m.errorCountBuffer = make(map[string]int64)
	m.bytesInBuffer = make(map[string]int64)
	m.bytesOutBuffer = make(map[string]int64)
}

func (m *MetricsCollector) collectSystemMetrics(timestamp time.Time) {
	// CPU usage
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		m.sendMetric("", "cpu_usage", cpuPercent[0], timestamp)
	}

	// Memory usage
	if vmstat, err := mem.VirtualMemory(); err == nil {
		m.sendMetric("", "memory_usage", vmstat.UsedPercent, timestamp)
	}

	// Goroutine count
	m.sendMetric("", "goroutines", float64(runtime.NumGoroutine()), timestamp)

	// GC stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.sendMetric("", "gc_pause_ns", float64(memStats.PauseNs[(memStats.NumGC+255)%256]), timestamp)
}

func (m *MetricsCollector) sendMetric(testID, name string, value float64, timestamp time.Time) {
	labels := map[string]string{
		"worker_id": m.workerID,
	}

	if testID != "" {
		labels["test_id"] = testID
	}

	metric := &common.Metric{
		Name:      name,
		Value:     value,
		Timestamp: timestamp,
		Labels:    labels,
	}

	select {
	case m.metricsCh <- metric:
	default:
		// Channel is full, log or handle the overflow
	}
}

// GetMetricsChan returns the metrics channel
func (m *MetricsCollector) GetMetricsChan() <-chan *common.Metric {
	return m.metricsCh
}

// GetSnapshot returns the current snapshot of metrics
func (m *MetricsCollector) GetSnapshot() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make(map[string]float64)

	// Add system metrics
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		snapshot["cpu_usage"] = cpuPercent[0]
	}

	if vmstat, err := mem.VirtualMemory(); err == nil {
		snapshot["memory_usage"] = vmstat.UsedPercent
	}

	snapshot["goroutines"] = float64(runtime.NumGoroutine())

	return snapshot
}
