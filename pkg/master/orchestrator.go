package master

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/saika-m/goload/internal/common"
	"github.com/saika-m/goload/pkg/config"
)

// Test represents a running or completed load test
type Test struct {
	Config    *config.TestConfig
	Status    common.TestStatus
	StartTime time.Time
	EndTime   time.Time
	Workers   map[string]*WorkerState
	Results   []*common.RequestResult
	mu        sync.RWMutex
}

// WorkerState tracks the state of a worker node
type WorkerState struct {
	Info          *common.WorkerInfo
	LastHeartbeat time.Time
	ActiveUsers   int
	Status        string
	CurrentTestID string
}

// Orchestrator manages the execution of load tests
type Orchestrator struct {
	cfg       *config.MasterConfig
	tests     map[string]*Test
	workers   map[string]*WorkerState
	scheduler *Scheduler
	metricsCh chan common.MetricsBatch
	resultsCh chan *common.RequestResult
	mu        sync.RWMutex
}

// NewOrchestrator creates a new test orchestrator
func NewOrchestrator(cfg *config.MasterConfig) *Orchestrator {
	return &Orchestrator{
		cfg:       cfg,
		tests:     make(map[string]*Test),
		workers:   make(map[string]*WorkerState),
		metricsCh: make(chan common.MetricsBatch, 1000),
		resultsCh: make(chan *common.RequestResult, 1000),
	}
}

// StartTest begins a new load test
func (o *Orchestrator) StartTest(ctx context.Context, cfg *config.TestConfig) (*Test, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Validate test configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Create new test
	test := &Test{
		Config: cfg.WithDefaults(),
		Status: common.TestStatus{
			TestID: cfg.TestID,
			State:  common.TestStatePending,
		},
		StartTime: time.Now(),
		Workers:   make(map[string]*WorkerState),
		Results:   make([]*common.RequestResult, 0),
	}

	// Store test
	o.tests[test.Config.TestID] = test

	// Start test goroutine
	go o.runTest(ctx, test)

	return test, nil
}

func (o *Orchestrator) runTest(ctx context.Context, test *Test) {
	// Create test context with timeout
	testCtx, cancel := context.WithTimeout(ctx, test.Config.Duration)
	defer cancel()

	// Update test state
	o.updateTestState(test, common.TestStateRunning, "Test started")

	// Calculate worker distribution
	workers, err := o.scheduler.AllocateWorkers(test)
	if err != nil {
		o.updateTestState(test, common.TestStateFailed, fmt.Sprintf("Failed to allocate workers: %v", err))
		return
	}

	// Start test on workers
	var wg sync.WaitGroup
	errCh := make(chan error, len(workers))

	for workerID, count := range workers {
		wg.Add(1)
		go func(id string, users int) {
			defer wg.Done()
			if err := o.startWorkerTest(testCtx, test, id, users); err != nil {
				errCh <- fmt.Errorf("worker %s failed: %w", id, err)
			}
		}(workerID, count)
	}

	// Monitor test progress
	go o.monitorTest(testCtx, test)

	// Wait for completion or failure
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-testCtx.Done():
		// Test duration completed
		o.completeTest(test)
	case err := <-errCh:
		// Test failed
		o.updateTestState(test, common.TestStateFailed, err.Error())
	case <-done:
		// All workers completed
		o.completeTest(test)
	}
}

func (o *Orchestrator) completeTest(test *Test) {
	test.mu.Lock()
	defer test.mu.Unlock()

	test.EndTime = time.Now()
	test.Status.State = common.TestStateCompleted
	test.Status.Message = "Test completed successfully"

	// Calculate final statistics
	avg, p95, p99, errorRate := common.CalculateStats(test.Results)
	test.Status.AvgResponseTime = avg
	test.Status.ErrorRate = errorRate

	// TODO: Generate and store test report
}

func (o *Orchestrator) startWorkerTest(ctx context.Context, test *Test, workerID string, users int) error {
	worker, exists := o.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found")
	}

	// Update worker state
	worker.CurrentTestID = test.Config.TestID
	worker.ActiveUsers = users
	worker.Status = "running"

	// Start test on worker
	// TODO: Implement gRPC call to worker to start test

	return nil
}

func (o *Orchestrator) monitorTest(ctx context.Context, test *Test) {
	ticker := time.NewTicker(o.cfg.ReportingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case metric := <-o.metricsCh:
			if metric.TestID == test.Config.TestID {
				o.processMetrics(test, metric)
			}
		case result := <-o.resultsCh:
			if result.TestID == test.Config.TestID {
				o.processResult(test, result)
			}
		case <-ticker.C:
			o.updateTestStats(test)
		}
	}
}

func (o *Orchestrator) processMetrics(test *Test, batch common.MetricsBatch) {
	test.mu.Lock()
	defer test.mu.Unlock()

	// Update test status with metrics
	// TODO: Implement metrics processing
}

func (o *Orchestrator) processResult(test *Test, result *common.RequestResult) {
	test.mu.Lock()
	defer test.mu.Unlock()

	test.Results = append(test.Results, result)
	test.Status.TotalRequests++
	if result.Error != nil {
		// Update error statistics
	}
}

func (o *Orchestrator) updateTestStats(test *Test) {
	test.mu.Lock()
	defer test.mu.Unlock()

	// Calculate current statistics
	if len(test.Results) > 0 {
		avg, _, _, errorRate := common.CalculateStats(test.Results)
		test.Status.AvgResponseTime = avg
		test.Status.ErrorRate = errorRate
	}
}

func (o *Orchestrator) updateTestState(test *Test, state common.TestState, message string) {
	test.mu.Lock()
	defer test.mu.Unlock()

	test.Status.State = state
	test.Status.Message = message
}

// StopTest stops a running test
func (o *Orchestrator) StopTest(testID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	test, exists := o.tests[testID]
	if !exists {
		return fmt.Errorf("test not found")
	}

	// Update test state
	o.updateTestState(test, common.TestStateCancelled, "Test stopped by user")

	// Stop test on all workers
	for workerID := range test.Workers {
		if worker, exists := o.workers[workerID]; exists {
			worker.Status = "idle"
			worker.ActiveUsers = 0
			worker.CurrentTestID = ""
		}
	}

	return nil
}

// GetTest returns information about a specific test
func (o *Orchestrator) GetTest(testID string) (*Test, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	test, exists := o.tests[testID]
	if !exists {
		return nil, fmt.Errorf("test not found")
	}

	return test, nil
}

// GetTestStatus returns the current status of a test
func (o *Orchestrator) GetTestStatus(testID string) (*common.TestStatus, error) {
	test, err := o.GetTest(testID)
	if err != nil {
		return nil, err
	}

	test.mu.RLock()
	defer test.mu.RUnlock()

	return &test.Status, nil
}
