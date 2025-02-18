package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/saika-m/goload/internal/common"
	"github.com/saika-m/goload/pkg/config"
)

// Worker represents a worker node in the distributed load testing system
type Worker struct {
	cfg       *config.WorkerConfig
	executor  *Executor
	collector *MetricsCollector
	reporter  *Reporter
	resultsCh chan *common.RequestResult
	metricsCh chan *common.Metric
	mu        sync.RWMutex
}

// NewWorker creates a new worker instance
func NewWorker(cfg *config.WorkerConfig) (*Worker, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create channels for internal communication
	resultsCh := make(chan *common.RequestResult, 10000)
	metricsCh := make(chan *common.Metric, 10000)

	// Create executor
	executor, err := NewExecutor(cfg, resultsCh, metricsCh)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	// Create metrics collector
	collector := NewMetricsCollector(cfg.ID, cfg.HeartbeatInterval)

	// Create reporter
	reporter := NewReporter(cfg.ID, cfg.MasterAddress, collector.GetMetricsChan(), resultsCh)

	return &Worker{
		cfg:       cfg,
		executor:  executor,
		collector: collector,
		reporter:  reporter,
		resultsCh: resultsCh,
		metricsCh: metricsCh,
	}, nil
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	// Start reporter first to establish connection with master
	if err := w.reporter.Start(ctx); err != nil {
		return fmt.Errorf("failed to start reporter: %w", err)
	}

	// Start metrics collector
	go w.collector.Start(ctx)

	// Create a wait group for graceful shutdown
	var wg sync.WaitGroup
	wg.Add(1)

	// Start result processing
	go func() {
		defer wg.Done()
		w.processResults(ctx)
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Cleanup
	if err := w.reporter.Stop(); err != nil {
		return fmt.Errorf("failed to stop reporter: %w", err)
	}

	w.collector.Stop()
	wg.Wait()

	return nil
}

// StartTest starts a new load test
func (w *Worker) StartTest(ctx context.Context, cfg *config.TestConfig, virtualUsers int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.executor.StartTest(ctx, cfg, virtualUsers)
}

// StopTest stops a running test
func (w *Worker) StopTest(testID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.executor.stopTest(testID)
}

// GetStatus returns the current status of the worker
func (w *Worker) GetStatus() (*common.WorkerInfo, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	info := w.cfg.WorkerInfo()
	info.Resources = common.GetResourceStats()

	return &info, nil
}

// processResults processes test results and forwards them to metrics collector
func (w *Worker) processResults(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case result := <-w.resultsCh:
			// Process result in metrics collector
			w.collector.ProcessResult(result)
		}
	}
}
