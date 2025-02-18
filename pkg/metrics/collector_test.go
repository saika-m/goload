package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/saika-m/goload/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector(1000)
	assert.NotNil(t, collector)
	assert.Equal(t, 1000, collector.sampleSize)
}

func TestProcessMetric(t *testing.T) {
	collector := NewCollector(1000)

	metric := &common.Metric{
		Name:  "test_metric",
		Value: 100.0,
		Type:  "counter",
		Labels: map[string]string{
			"test": "true",
		},
		Timestamp: time.Now(),
	}

	collector.processMetric(metric)

	// Verify metric was processed
	snapshot := collector.GetSnapshot()
	assert.Contains(t, snapshot, "test_metric")
}

func TestGetSnapshot(t *testing.T) {
	collector := NewCollector(1000)

	// Add test metrics
	metrics := []*common.Metric{
		{
			Name:  "response_time",
			Value: 100.0,
			Type:  "timer",
			Labels: map[string]string{
				"endpoint": "/test",
			},
		},
		{
			Name:  "requests_total",
			Value: 1.0,
			Type:  "counter",
			Labels: map[string]string{
				"endpoint": "/test",
			},
		},
	}

	for _, m := range metrics {
		collector.processMetric(m)
	}

	snapshot := collector.GetSnapshot()
	assert.Contains(t, snapshot, "response_time")
	assert.Contains(t, snapshot, "requests_total")
}

func TestCollectorConcurrency(t *testing.T) {
	collector := NewCollector(1000)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start collector
	go collector.Start(ctx)

	// Send metrics concurrently
	const numGoroutines = 10
	const numMetricsPerGoroutine = 100

	done := make(chan bool)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			for j := 0; j < numMetricsPerGoroutine; j++ {
				collector.metricsChan <- &common.Metric{
					Name:  "test_metric",
					Value: float64(j),
					Type:  "counter",
					Labels: map[string]string{
						"routine_id": string(routineID),
					},
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify metrics were processed
	snapshot := collector.GetSnapshot()
	assert.Contains(t, snapshot, "test_metric")
}

func TestRecordResponse(t *testing.T) {
	collector := NewCollector(1000)

	result := &common.RequestResult{
		WorkerID:      "worker1",
		TestID:        "test1",
		ScenarioName:  "scenario1",
		RequestName:   "request1",
		StartTime:     time.Now(),
		Duration:      100 * time.Millisecond,
		StatusCode:    200,
		BytesSent:     1000,
		BytesReceived: 2000,
	}

	collector.RecordResponse(result)

	// Verify metrics were recorded
	snapshot := collector.GetSnapshot()
	assert.Contains(t, snapshot, "response_time")
	assert.Contains(t, snapshot, "requests_total")
	assert.Contains(t, snapshot, "bytes_sent")
	assert.Contains(t, snapshot, "bytes_received")
}

func TestReset(t *testing.T) {
	collector := NewCollector(1000)

	// Add some metrics
	collector.processMetric(&common.Metric{
		Name:  "test_metric",
		Value: 100.0,
		Type:  "counter",
	})

	// Verify metrics exist
	snapshot := collector.GetSnapshot()
	assert.NotEmpty(t, snapshot)

	// Reset collector
	collector.Reset()

	// Verify metrics were cleared
	snapshot = collector.GetSnapshot()
	assert.Empty(t, snapshot)
}
