package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/saika-m/goload/internal/common"
)

// defined in types.go
// type MetricType string

/*
const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
	Timer     MetricType = "timer"
)
*/

// Collector handles metric collection and aggregation
type Collector struct {
	registry    metrics.Registry
	histograms  map[string]metrics.Histogram
	timers      map[string]metrics.Timer
	counters    map[string]metrics.Counter
	gauges      map[string]metrics.Gauge
	sampleSize  int
	mu          sync.RWMutex
	metricsChan chan *common.Metric
}

// NewCollector creates a new metrics collector
func NewCollector(sampleSize int) *Collector {
	return &Collector{
		registry:    metrics.NewRegistry(),
		histograms:  make(map[string]metrics.Histogram),
		timers:      make(map[string]metrics.Timer),
		counters:    make(map[string]metrics.Counter),
		gauges:      make(map[string]metrics.Gauge),
		sampleSize:  sampleSize,
		metricsChan: make(chan *common.Metric, 10000),
	}
}

// Start begins collecting metrics
func (c *Collector) Start(ctx context.Context) {
	go c.processMetrics(ctx)
}

// processMetrics handles incoming metrics
func (c *Collector) processMetrics(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case metric := <-c.metricsChan:
			c.processMetric(metric)
		}
	}
}

// processMetric processes a single metric
func (c *Collector) processMetric(metric *common.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := generateMetricKey(metric.Name, metric.Labels)

	switch metric.Type {
	case common.Timer:
		timer := c.getOrCreateTimer(key)
		timer.Update(time.Duration(metric.Value))

	case common.Counter:
		counter := c.getOrCreateCounter(key)
		counter.Inc(int64(metric.Value))

	case common.Gauge:
		gauge := c.getOrCreateGauge(key)
		gauge.Update(int64(metric.Value))

	case common.Histogram:
		histogram := c.getOrCreateHistogram(key)
		histogram.Update(int64(metric.Value))
	}
}

// GetMetric retrieves a metric by name and type
func (c *Collector) GetMetric(name string, metricType common.MetricType) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch metricType {
	case common.Timer:
		return c.timers[name]
	case common.Counter:
		return c.counters[name]
	case common.Gauge:
		return c.gauges[name]
	case common.Histogram:
		return c.histograms[name]
	default:
		return nil
	}
}

// GetSnapshot returns a snapshot of all metrics
func (c *Collector) GetSnapshot() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := make(map[string]interface{})

	// Add timer metrics
	for name, timer := range c.timers {
		snapshot[name] = map[string]interface{}{
			"count":    timer.Count(),
			"min":      timer.Min(),
			"max":      timer.Max(),
			"mean":     timer.Mean(),
			"stddev":   timer.StdDev(),
			"p50":      timer.Percentile(0.5),
			"p75":      timer.Percentile(0.75),
			"p95":      timer.Percentile(0.95),
			"p99":      timer.Percentile(0.99),
			"p999":     timer.Percentile(0.999),
			"rate1":    timer.Rate1(),
			"rate5":    timer.Rate5(),
			"rate15":   timer.Rate15(),
			"rateMean": timer.RateMean(),
		}
	}

	// Add counter metrics
	for name, counter := range c.counters {
		snapshot[name] = map[string]interface{}{
			"count": counter.Count(),
		}
	}

	// Add gauge metrics
	for name, gauge := range c.gauges {
		snapshot[name] = map[string]interface{}{
			"value": gauge.Value(),
		}
	}

	// Add histogram metrics
	for name, histogram := range c.histograms {
		snapshot[name] = map[string]interface{}{
			"count":  histogram.Count(),
			"min":    histogram.Min(),
			"max":    histogram.Max(),
			"mean":   histogram.Mean(),
			"stddev": histogram.StdDev(),
			"p50":    histogram.Percentile(0.5),
			"p75":    histogram.Percentile(0.75),
			"p95":    histogram.Percentile(0.95),
			"p99":    histogram.Percentile(0.99),
			"p999":   histogram.Percentile(0.999),
		}
	}

	return snapshot
}

// getOrCreateTimer gets or creates a timer metric
func (c *Collector) getOrCreateTimer(name string) metrics.Timer {
	if timer, exists := c.timers[name]; exists {
		return timer
	}
	timer := metrics.NewTimer()
	c.registry.Register(name, timer)
	c.timers[name] = timer
	return timer
}

// getOrCreateCounter gets or creates a counter metric
func (c *Collector) getOrCreateCounter(name string) metrics.Counter {
	if counter, exists := c.counters[name]; exists {
		return counter
	}
	counter := metrics.NewCounter()
	c.registry.Register(name, counter)
	c.counters[name] = counter
	return counter
}

// getOrCreateGauge gets or creates a gauge metric
func (c *Collector) getOrCreateGauge(name string) metrics.Gauge {
	if gauge, exists := c.gauges[name]; exists {
		return gauge
	}
	gauge := metrics.NewGauge()
	c.registry.Register(name, gauge)
	c.gauges[name] = gauge
	return gauge
}

// getOrCreateHistogram gets or creates a histogram metric
func (c *Collector) getOrCreateHistogram(name string) metrics.Histogram {
	if histogram, exists := c.histograms[name]; exists {
		return histogram
	}
	histogram := metrics.NewHistogram(metrics.NewExpDecaySample(c.sampleSize, 0.015))
	c.registry.Register(name, histogram)
	c.histograms[name] = histogram
	return histogram
}

// generateMetricKey generates a unique key for a metric
func generateMetricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += ";" + k + "=" + v
	}
	return key
}

// RecordResponse records a request response
func (c *Collector) RecordResponse(result *common.RequestResult) {
	labels := map[string]string{
		"test_id":      result.TestID,
		"worker_id":    result.WorkerID,
		"scenario":     result.ScenarioName,
		"request_name": result.RequestName,
	}

	// Record response time
	c.metricsChan <- &common.Metric{
		Name:      "response_time",
		Value:     float64(result.Duration.Milliseconds()),
		Type:      common.Timer,
		Labels:    labels,
		Timestamp: result.StartTime,
	}

	// Record request count
	c.metricsChan <- &common.Metric{
		Name:      "requests_total",
		Value:     1,
		Type:      common.Counter,
		Labels:    labels,
		Timestamp: result.StartTime,
	}

	// Record error count if there was an error
	if result.Error != nil {
		c.metricsChan <- &common.Metric{
			Name:      "errors_total",
			Value:     1,
			Type:      common.Counter,
			Labels:    labels,
			Timestamp: result.StartTime,
		}
	}

	// Record bytes sent/received
	c.metricsChan <- &common.Metric{
		Name:      "bytes_sent",
		Value:     float64(result.BytesSent),
		Type:      common.Counter,
		Labels:    labels,
		Timestamp: result.StartTime,
	}

	c.metricsChan <- &common.Metric{
		Name:      "bytes_received",
		Value:     float64(result.BytesReceived),
		Type:      common.Counter,
		Labels:    labels,
		Timestamp: result.StartTime,
	}
}

// Reset resets all metrics
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry = metrics.NewRegistry()
	c.histograms = make(map[string]metrics.Histogram)
	c.timers = make(map[string]metrics.Timer)
	c.counters = make(map[string]metrics.Counter)
	c.gauges = make(map[string]metrics.Gauge)
}
