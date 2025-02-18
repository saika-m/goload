package metrics

import (
	"context"
	"sort"
	"sync"
	"time"
)

// AggregatedMetric represents an aggregated metric value
type AggregatedMetric struct {
	Count     int64
	Sum       float64
	Min       float64
	Max       float64
	Mean      float64
	P50       float64
	P90       float64
	P95       float64
	P99       float64
	Rate      float64
	Timestamp time.Time
}

// Aggregator handles metric aggregation across workers
type Aggregator struct {
	mu           sync.RWMutex
	metrics      map[string][]float64
	aggregations map[string]*AggregatedMetric
	windowSize   time.Duration
	updateCh     chan struct{}
}

// NewAggregator creates a new metric aggregator
func NewAggregator(windowSize time.Duration) *Aggregator {
	return &Aggregator{
		metrics:      make(map[string][]float64),
		aggregations: make(map[string]*AggregatedMetric),
		windowSize:   windowSize,
		updateCh:     make(chan struct{}, 1),
	}
}

// Start begins the aggregation process
func (a *Aggregator) Start(ctx context.Context) {
	ticker := time.NewTicker(a.windowSize)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.aggregate()
		case <-a.updateCh:
			a.aggregate()
		}
	}
}

// AddMetric adds a new metric value
func (a *Aggregator) AddMetric(key string, value float64, timestamp time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.metrics[key]; !exists {
		a.metrics[key] = make([]float64, 0)
	}
	a.metrics[key] = append(a.metrics[key], value)

	// Trigger update if this is the first value
	if len(a.metrics[key]) == 1 {
		select {
		case a.updateCh <- struct{}{}:
		default:
		}
	}
}

// aggregate performs the actual metric aggregation
func (a *Aggregator) aggregate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	for key, values := range a.metrics {
		if len(values) == 0 {
			continue
		}

		// Sort values for percentile calculations
		sort.Float64s(values)

		// Calculate statistics
		count := int64(len(values))
		sum := 0.0
		min := values[0]
		max := values[len(values)-1]

		for _, v := range values {
			sum += v
		}

		mean := sum / float64(count)
		p50 := percentile(values, 0.5)
		p90 := percentile(values, 0.9)
		p95 := percentile(values, 0.95)
		p99 := percentile(values, 0.99)
		rate := float64(count) / a.windowSize.Seconds()

		a.aggregations[key] = &AggregatedMetric{
			Count:     count,
			Sum:       sum,
			Min:       min,
			Max:       max,
			Mean:      mean,
			P50:       p50,
			P90:       p90,
			P95:       p95,
			P99:       p99,
			Rate:      rate,
			Timestamp: now,
		}
	}

	// Reset raw metrics
	a.metrics = make(map[string][]float64)
}

// GetAggregation returns the aggregated metrics for a key
func (a *Aggregator) GetAggregation(key string) *AggregatedMetric {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.aggregations[key]
}

// GetAllAggregations returns all aggregated metrics
func (a *Aggregator) GetAllAggregations() map[string]*AggregatedMetric {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Create a copy to avoid concurrent access issues
	result := make(map[string]*AggregatedMetric, len(a.aggregations))
	for k, v := range a.aggregations {
		result[k] = &AggregatedMetric{
			Count:     v.Count,
			Sum:       v.Sum,
			Min:       v.Min,
			Max:       v.Max,
			Mean:      v.Mean,
			P50:       v.P50,
			P90:       v.P90,
			P95:       v.P95,
			P99:       v.P99,
			Rate:      v.Rate,
			Timestamp: v.Timestamp,
		}
	}
	return result
}

// percentile calculates the given percentile from sorted values
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	rank := p * float64(len(values)-1)
	rankInt := int(rank)
	rankFrac := rank - float64(rankInt)

	if rankInt >= len(values)-1 {
		return values[len(values)-1]
	}

	return values[rankInt] + rankFrac*(values[rankInt+1]-values[rankInt])
}

// ClearOldMetrics removes metrics older than the specified duration
func (a *Aggregator) ClearOldMetrics(maxAge time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	threshold := time.Now().Add(-maxAge)

	for key, agg := range a.aggregations {
		if agg.Timestamp.Before(threshold) {
			delete(a.aggregations, key)
		}
	}
}

// MergeAggregations combines aggregations from multiple sources
func (a *Aggregator) MergeAggregations(others ...*Aggregator) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, other := range others {
		other.mu.RLock()
		for key, values := range other.metrics {
			if _, exists := a.metrics[key]; !exists {
				a.metrics[key] = make([]float64, 0, len(values))
			}
			a.metrics[key] = append(a.metrics[key], values...)
		}
		other.mu.RUnlock()
	}

	// Trigger reaggregation
	select {
	case a.updateCh <- struct{}{}:
	default:
	}
}

// GetRateForKey returns the current rate for a specific metric key
func (a *Aggregator) GetRateForKey(key string) float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if agg, exists := a.aggregations[key]; exists {
		return agg.Rate
	}
	return 0
}

// GetStatistics returns detailed statistics for a metric key
func (a *Aggregator) GetStatistics(key string) map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	agg, exists := a.aggregations[key]
	if !exists {
		return nil
	}

	return map[string]float64{
		"count": float64(agg.Count),
		"sum":   agg.Sum,
		"min":   agg.Min,
		"max":   agg.Max,
		"mean":  agg.Mean,
		"p50":   agg.P50,
		"p90":   agg.P90,
		"p95":   agg.P95,
		"p99":   agg.P99,
		"rate":  agg.Rate,
	}
}

// CreateSnapshot creates a point-in-time snapshot of all metrics
func (a *Aggregator) CreateSnapshot() map[string]map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	snapshot := make(map[string]map[string]interface{})

	for key, agg := range a.aggregations {
		snapshot[key] = map[string]interface{}{
			"count":     agg.Count,
			"sum":       agg.Sum,
			"min":       agg.Min,
			"max":       agg.Max,
			"mean":      agg.Mean,
			"p50":       agg.P50,
			"p90":       agg.P90,
			"p95":       agg.P95,
			"p99":       agg.P99,
			"rate":      agg.Rate,
			"timestamp": agg.Timestamp,
		}
	}

	return snapshot
}

// Reset clears all current metrics and aggregations
func (a *Aggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.metrics = make(map[string][]float64)
	a.aggregations = make(map[string]*AggregatedMetric)
}

// SetWindowSize updates the aggregation window size
func (a *Aggregator) SetWindowSize(windowSize time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.windowSize = windowSize
}

// GetWindowSize returns the current aggregation window size
func (a *Aggregator) GetWindowSize() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.windowSize
}
