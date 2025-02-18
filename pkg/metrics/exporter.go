package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ExportFormat defines the format for metric export
type ExportFormat string

const (
	FormatJSON       ExportFormat = "json"
	FormatPrometheus ExportFormat = "prometheus"
	FormatInfluxDB   ExportFormat = "influxdb"
)

// Exporter handles exporting metrics to various backends
type Exporter struct {
	aggregator *Aggregator
	format     ExportFormat
	interval   time.Duration
	endpoint   string
	httpClient *http.Client
	registry   *prometheus.Registry
	metrics    map[string]*prometheus.GaugeVec
	mu         sync.RWMutex
}

// NewExporter creates a new metrics exporter
func NewExporter(aggregator *Aggregator, format ExportFormat, endpoint string, interval time.Duration) *Exporter {
	registry := prometheus.NewRegistry()

	return &Exporter{
		aggregator: aggregator,
		format:     format,
		endpoint:   endpoint,
		interval:   interval,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		registry:   registry,
		metrics:    make(map[string]*prometheus.GaugeVec),
	}
}

// Start begins the metrics export process
func (e *Exporter) Start(ctx context.Context) error {
	// Start HTTP server for Prometheus metrics if needed
	if e.format == FormatPrometheus {
		go e.startPrometheusServer()
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := e.export(); err != nil {
				// Log error but continue running
				fmt.Printf("Error exporting metrics: %v\n", err)
			}
		}
	}
}

// export performs the actual metric export
func (e *Exporter) export() error {
	metrics := e.aggregator.GetAllAggregations()

	switch e.format {
	case FormatJSON:
		return e.exportJSON(metrics)
	case FormatPrometheus:
		return e.updatePrometheusMetrics(metrics)
	case FormatInfluxDB:
		return e.exportInfluxDB(metrics)
	default:
		return fmt.Errorf("unsupported export format: %s", e.format)
	}
}

// exportJSON exports metrics in JSON format
func (e *Exporter) exportJSON(metrics map[string]*AggregatedMetric) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	req, err := http.NewRequest("POST", e.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// updatePrometheusMetrics updates Prometheus metrics
func (e *Exporter) updatePrometheusMetrics(metrics map[string]*AggregatedMetric) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for key, metric := range metrics {
		// Create gauge vectors if they don't exist
		if _, exists := e.metrics[key]; !exists {
			e.metrics[key] = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: sanitizePrometheusName(key),
					Help: fmt.Sprintf("Metric %s", key),
				},
				[]string{"type"},
			)
			e.registry.MustRegister(e.metrics[key])
		}

		// Update gauge values
		e.metrics[key].With(prometheus.Labels{"type": "count"}).Set(float64(metric.Count))
		e.metrics[key].With(prometheus.Labels{"type": "min"}).Set(metric.Min)
		e.metrics[key].With(prometheus.Labels{"type": "max"}).Set(metric.Max)
		e.metrics[key].With(prometheus.Labels{"type": "mean"}).Set(metric.Mean)
		e.metrics[key].With(prometheus.Labels{"type": "p50"}).Set(metric.P50)
		e.metrics[key].With(prometheus.Labels{"type": "p90"}).Set(metric.P90)
		e.metrics[key].With(prometheus.Labels{"type": "p95"}).Set(metric.P95)
		e.metrics[key].With(prometheus.Labels{"type": "p99"}).Set(metric.P99)
		e.metrics[key].With(prometheus.Labels{"type": "rate"}).Set(metric.Rate)
	}

	return nil
}

// startPrometheusServer starts the Prometheus metrics server
func (e *Exporter) startPrometheusServer() {
	http.Handle("/metrics", promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{}))
	http.ListenAndServe(e.endpoint, nil)
}

// exportInfluxDB exports metrics to InfluxDB
func (e *Exporter) exportInfluxDB(metrics map[string]*AggregatedMetric) error {
	points := make([]influxdb.Point, 0, len(metrics))

	for key, metric := range metrics {
		tags := map[string]string{
			"metric": key,
		}

		fields := map[string]interface{}{
			"count": metric.Count,
			"min":   metric.Min,
			"max":   metric.Max,
			"mean":  metric.Mean,
			"p50":   metric.P50,
			"p90":   metric.P90,
			"p95":   metric.P95,
			"p99":   metric.P99,
			"rate":  metric.Rate,
		}

		point := influxdb.Point{
			Measurement: "load_test_metrics",
			Tags:        tags,
			Fields:      fields,
			Time:        metric.Timestamp,
		}

		points = append(points, point)
	}

	// Write batch to InfluxDB
	// Note: Implementation depends on the specific InfluxDB client version
	// This is a placeholder for the actual implementation
	return nil
}

// sanitizePrometheusName sanitizes metric names for Prometheus
func sanitizePrometheusName(name string) string {
	// Replace invalid characters with underscores
	// Only [a-zA-Z0-9_] are valid in Prometheus metric names
	replacer := strings.NewReplacer(
		"-", "_",
		".", "_",
		" ", "_",
		",", "_",
		";", "_",
		"=", "_",
	)
	return replacer.Replace(name)
}

// SetExportFormat updates the export format
func (e *Exporter) SetExportFormat(format ExportFormat) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.format = format
}

// SetEndpoint updates the export endpoint
func (e *Exporter) SetEndpoint(endpoint string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.endpoint = endpoint
}

// SetInterval updates the export interval
func (e *Exporter) SetInterval(interval time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.interval = interval
}
