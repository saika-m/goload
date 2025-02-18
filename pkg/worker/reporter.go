package worker

import (
	"context"
	"sync"
	"time"

	"github.com/saika-m/goload/internal/common"
	"google.golang.org/grpc"
)

// Reporter handles sending metrics and results back to the master node
type Reporter struct {
	workerID     string
	masterAddr   string
	conn         *grpc.ClientConn
	client       LoadTestClient
	metricsCh    <-chan *common.Metric
	resultsCh    <-chan *common.RequestResult
	batchSize    int
	batchTimeout time.Duration
	mu           sync.RWMutex
}

// NewReporter creates a new reporter
func NewReporter(workerID, masterAddr string, metricsCh <-chan *common.Metric, resultsCh <-chan *common.RequestResult) *Reporter {
	return &Reporter{
		workerID:     workerID,
		masterAddr:   masterAddr,
		metricsCh:    metricsCh,
		resultsCh:    resultsCh,
		batchSize:    100,
		batchTimeout: 1 * time.Second,
	}
}

// Start begins reporting metrics and results to the master
func (r *Reporter) Start(ctx context.Context) error {
	// Connect to master
	conn, err := grpc.Dial(r.masterAddr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	r.conn = conn
	r.client = NewLoadTestClient(conn)

	// Start reporting goroutines
	go r.reportMetrics(ctx)
	go r.reportResults(ctx)

	// Start heartbeat
	go r.sendHeartbeat(ctx)

	return nil
}

// Stop stops the reporter
func (r *Reporter) Stop() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *Reporter) reportMetrics(ctx context.Context) {
	batch := make([]*common.Metric, 0, r.batchSize)
	ticker := time.NewTicker(r.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				r.sendMetricsBatch(ctx, batch)
			}
			return
		case metric := <-r.metricsCh:
			batch = append(batch, metric)
			if len(batch) >= r.batchSize {
				r.sendMetricsBatch(ctx, batch)
				batch = make([]*common.Metric, 0, r.batchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.sendMetricsBatch(ctx, batch)
				batch = make([]*common.Metric, 0, r.batchSize)
			}
		}
	}
}

func (r *Reporter) reportResults(ctx context.Context) {
	batch := make([]*common.RequestResult, 0, r.batchSize)
	ticker := time.NewTicker(r.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				r.sendResultsBatch(ctx, batch)
			}
			return
		case result := <-r.resultsCh:
			batch = append(batch, result)
			if len(batch) >= r.batchSize {
				r.sendResultsBatch(ctx, batch)
				batch = make([]*common.RequestResult, 0, r.batchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.sendResultsBatch(ctx, batch)
				batch = make([]*common.RequestResult, 0, r.batchSize)
			}
		}
	}
}

func (r *Reporter) sendMetricsBatch(ctx context.Context, batch []*common.Metric) {
	metricsBatch := &common.MetricsBatch{
		WorkerID:  r.workerID,
		Metrics:   batch,
		Timestamp: time.Now(),
	}

	// Convert to protobuf message
	pbBatch := convertMetricsBatchToProto(metricsBatch)

	// Send to master
	_, err := r.client.ReportMetrics(ctx, pbBatch)
	if err != nil {
		// Handle error (retry, log, etc.)
		return
	}
}

func (r *Reporter) sendResultsBatch(ctx context.Context, batch []*common.RequestResult) {
	// Convert to protobuf message
	pbBatch := convertResultsBatchToProto(batch)

	// Send to master
	_, err := r.client.ReportResults(ctx, pbBatch)
	if err != nil {
		// Handle error (retry, log, etc.)
		return
	}
}

func (r *Reporter) sendHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := common.GetResourceStats()
			heartbeat := &HeartbeatRequest{
				WorkerId:  r.workerID,
				Timestamp: time.Now().UnixNano(),
				Resources: &ResourceStats{
					CpuUsage:    stats.CPUUsage,
					MemoryUsage: stats.MemoryUsage,
				},
			}

			_, err := r.client.Heartbeat(ctx, heartbeat)
			if err != nil {
				// Handle error (retry, log, etc.)
				continue
			}
		}
	}
}

// Helper functions to convert between domain types and protobuf messages
func convertMetricsBatchToProto(batch *common.MetricsBatch) *MetricsBatchProto {
	pbMetrics := make([]*MetricProto, len(batch.Metrics))
	for i, metric := range batch.Metrics {
		pbMetrics[i] = &MetricProto{
			Name:      metric.Name,
			Value:     metric.Value,
			Labels:    metric.Labels,
			Timestamp: metric.Timestamp.UnixNano(),
		}
	}

	return &MetricsBatchProto{
		WorkerId:  batch.WorkerID,
		Metrics:   pbMetrics,
		Timestamp: batch.Timestamp.UnixNano(),
	}
}

func convertResultsBatchToProto(batch []*common.RequestResult) *ResultsBatchProto {
	pbResults := make([]*RequestResultProto, len(batch))
	for i, result := range batch {
		pbResults[i] = &RequestResultProto{
			WorkerId:      result.WorkerID,
			TestId:        result.TestID,
			ScenarioName:  result.ScenarioName,
			RequestName:   result.RequestName,
			StartTime:     result.StartTime.UnixNano(),
			Duration:      result.Duration.Nanoseconds(),
			StatusCode:    int32(result.StatusCode),
			Error:         result.Error.Error(),
			BytesSent:     result.BytesSent,
			BytesReceived: result.BytesReceived,
		}
	}

	return &ResultsBatchProto{
		Results: pbResults,
	}
}

// RequestResultProto represents a protobuf message for request results
type RequestResultProto struct {
	WorkerId      string
	TestId        string
	ScenarioName  string
	RequestName   string
	StartTime     int64
	Duration      int64
	StatusCode    int32
	Error         string
	BytesSent     int64
	BytesReceived int64
}

// ResultsBatchProto represents a batch of request results
type ResultsBatchProto struct {
	Results []*RequestResultProto
}

// MetricProto represents a protobuf message for metrics
type MetricProto struct {
	Name      string
	Value     float64
	Labels    map[string]string
	Timestamp int64
}

// MetricsBatchProto represents a batch of metrics
type MetricsBatchProto struct {
	WorkerId  string
	Metrics   []*MetricProto
	Timestamp int64
}

// HeartbeatRequest represents a worker heartbeat message
type HeartbeatRequest struct {
	WorkerId  string
	Timestamp int64
	Resources *ResourceStats
}

// ResourceStats represents resource usage statistics
type ResourceStats struct {
	CpuUsage    float64
	MemoryUsage float64
}

// LoadTestClient interface defines the gRPC client methods
type LoadTestClient interface {
	ReportMetrics(ctx context.Context, in *MetricsBatchProto, opts ...grpc.CallOption) (*MetricsResponse, error)
	ReportResults(ctx context.Context, in *ResultsBatchProto, opts ...grpc.CallOption) (*ResultsResponse, error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
}

// MetricsResponse represents the response from reporting metrics
type MetricsResponse struct {
	Success bool
	Message string
}

// ResultsResponse represents the response from reporting results
type ResultsResponse struct {
	Success bool
	Message string
}

// HeartbeatResponse represents the response from a heartbeat
type HeartbeatResponse struct {
	Success bool
	Message string
}

// NewLoadTestClient creates a new LoadTest gRPC client
func NewLoadTestClient(conn *grpc.ClientConn) LoadTestClient {
	return &loadTestClient{conn: conn}
}

// loadTestClient implements the LoadTestClient interface
type loadTestClient struct {
	conn *grpc.ClientConn
}

func (c *loadTestClient) ReportMetrics(ctx context.Context, in *MetricsBatchProto, opts ...grpc.CallOption) (*MetricsResponse, error) {
	return &MetricsResponse{Success: true}, nil
}

func (c *loadTestClient) ReportResults(ctx context.Context, in *ResultsBatchProto, opts ...grpc.CallOption) (*ResultsResponse, error) {
	return &ResultsResponse{Success: true}, nil
}

func (c *loadTestClient) Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error) {
	return &HeartbeatResponse{Success: true}, nil
}
