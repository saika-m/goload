package master

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/saika-m/goload/internal/common"
)

// Orchestrator manages the overall load test execution
type orchestratorImpl struct {
	*common.Orchestrator
	config *common.MasterConfig
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(cfg *common.MasterConfig) *common.Orchestrator {
	o := &orchestratorImpl{
		Orchestrator: &common.Orchestrator{
			Workers: make(map[string]*common.WorkerState),
			Tests:   make(map[string]*common.Test),
			Mu:      sync.RWMutex{},
		},
		config: cfg,
	}

	// Set the orchestration methods
	o.Orchestrator.OrchestrationMethods = o

	// Initialize scheduler with the common orchestrator
	o.Orchestrator.Scheduler = NewScheduler(o.Orchestrator)

	return o.Orchestrator
}

// StartTest starts a new load test
func (o *orchestratorImpl) StartTest(ctx context.Context, cfg *common.TestConfig) (*common.Test, error) {
	o.Mu.Lock()
	defer o.Mu.Unlock()

	// Validate test configuration
	if err := cfg.Validate(); err != nil {
		return nil, &common.LoadTestError{
			Code:    common.ErrConfigInvalid,
			Message: "invalid test configuration",
			Err:     err,
		}
	}

	// Create new test instance
	test := &common.Test{
		Config: cfg,
		Status: &common.TestStatus{
			TestID:  cfg.TestID,
			State:   common.TestStatePending,
			Message: "Initializing test",
		},
		Workers:   make(map[string]*common.WorkerState),
		StartTime: time.Now(),
	}

	// Allocate workers
	allocation, err := o.Scheduler.AllocateWorkers(test)
	if err != nil {
		return nil, &common.LoadTestError{
			Code:    common.ErrWorkerUnavailable,
			Message: "failed to allocate workers",
			Err:     err,
		}
	}

	// Update test and worker states
	for workerID, count := range allocation {
		worker := o.Workers[workerID]
		worker.Status = "running"
		worker.ActiveUsers = count
		worker.CurrentTestID = cfg.TestID

		test.Workers[workerID] = &common.WorkerState{
			Info:        worker.Info,
			Status:      "running",
			ActiveUsers: count,
		}
	}

	test.Status.State = common.TestStateRunning
	test.Status.Message = "Test running"
	o.Tests[cfg.TestID] = test

	return test, nil
}

// GetTest retrieves information about a specific test
func (o *orchestratorImpl) GetTest(testID string) (*common.Test, error) {
	o.Mu.RLock()
	defer o.Mu.RUnlock()

	test, exists := o.Tests[testID]
	if !exists {
		return nil, &common.LoadTestError{
			Code:    common.ErrTestNotFound,
			Message: fmt.Sprintf("test not found: %s", testID),
		}
	}

	return test, nil
}

// StopTest stops a running test
func (o *orchestratorImpl) StopTest(testID string) error {
	o.Mu.Lock()
	defer o.Mu.Unlock()

	test, exists := o.Tests[testID]
	if !exists {
		return &common.LoadTestError{
			Code:    common.ErrTestNotFound,
			Message: fmt.Sprintf("test not found: %s", testID),
		}
	}

	// Update test status
	test.Status.State = common.TestStateCancelled
	test.Status.Message = "Test stopped by user"
	test.EndTime = time.Now()

	// Update worker states
	for workerID, worker := range test.Workers {
		if w, exists := o.Workers[workerID]; exists {
			w.Status = "idle"
			w.ActiveUsers = 0
			w.CurrentTestID = ""
		}
		worker.Status = "idle"
		worker.ActiveUsers = 0
	}

	return nil
}

// GetTestStatus gets the current status of a test
func (o *orchestratorImpl) GetTestStatus(testID string) (*common.TestStatus, error) {
	o.Mu.RLock()
	defer o.Mu.RUnlock()

	test, exists := o.Tests[testID]
	if !exists {
		return nil, &common.LoadTestError{
			Code:    common.ErrTestNotFound,
			Message: fmt.Sprintf("test not found: %s", testID),
		}
	}

	return test.Status, nil
}

// RegisterWorker registers a new worker node
func (o *orchestratorImpl) RegisterWorker(workerInfo *common.WorkerInfo) (*common.WorkerState, error) {
	o.Mu.Lock()
	defer o.Mu.Unlock()

	if len(o.Workers) >= o.config.MaxWorkers {
		return nil, fmt.Errorf("maximum number of workers reached")
	}

	worker := &common.WorkerState{
		Info:          workerInfo,
		Status:        "ready",
		LastHeartbeat: time.Now(),
	}

	o.Workers[workerInfo.ID] = worker
	return worker, nil
}

// UpdateWorkerHeartbeat updates the last heartbeat time for a worker
func (o *orchestratorImpl) UpdateWorkerHeartbeat(workerID string, stats *common.ResourceStats) error {
	o.Mu.Lock()
	defer o.Mu.Unlock()

	worker, exists := o.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	worker.LastHeartbeat = time.Now()
	worker.Info.Resources = *stats

	return nil
}

// RemoveWorker removes a worker from the orchestrator
func (o *orchestratorImpl) RemoveWorker(workerID string) error {
	o.Mu.Lock()
	defer o.Mu.Unlock()

	worker, exists := o.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// If worker is running a test, mark it as failed
	if worker.CurrentTestID != "" {
		if test, exists := o.Tests[worker.CurrentTestID]; exists {
			test.Status.State = common.TestStateFailed
			test.Status.Message = fmt.Sprintf("Worker %s failed", workerID)
		}
	}

	delete(o.Workers, workerID)
	return nil
}
