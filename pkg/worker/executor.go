package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/saika-m/goload/internal/common"
	"github.com/saika-m/goload/pkg/config"
	"github.com/saika-m/goload/pkg/protocol"
)

// Executor handles the execution of load tests on a worker node
type Executor struct {
	cfg         *config.WorkerConfig
	httpClient  *protocol.HTTPClient
	grpcClient  *protocol.GRPCClient
	wsClient    *protocol.WSClient
	activeTests map[string]*TestExecution
	resultsCh   chan *common.RequestResult
	metricsCh   chan *common.Metric
	mu          sync.RWMutex
}

// TestExecution represents a running test on the worker
type TestExecution struct {
	TestID       string
	Config       *config.TestConfig
	VirtualUsers []*VirtualUser
	StartTime    time.Time
	Context      context.Context
	Cancel       context.CancelFunc
}

// VirtualUser represents a single virtual user in the test
type VirtualUser struct {
	ID          int
	ScenarioIdx int
	Context     context.Context
	Cancel      context.CancelFunc
}

// NewExecutor creates a new test executor
func NewExecutor(cfg *config.WorkerConfig, resultsCh chan *common.RequestResult, metricsCh chan *common.Metric) (*Executor, error) {
	httpClient, err := protocol.NewHTTPClient(cfg.GetConnectionPoolConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	grpcClient := protocol.NewGRPCClient()
	wsClient := protocol.NewWSClient()

	return &Executor{
		cfg:         cfg,
		httpClient:  httpClient,
		grpcClient:  grpcClient,
		wsClient:    wsClient,
		activeTests: make(map[string]*TestExecution),
		resultsCh:   resultsCh,
		metricsCh:   metricsCh,
	}, nil
}

// StartTest begins test execution for the specified number of virtual users
func (e *Executor) StartTest(ctx context.Context, cfg *config.TestConfig, virtualUsers int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Create test execution context
	testCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	execution := &TestExecution{
		TestID:       cfg.TestID,
		Config:       cfg,
		VirtualUsers: make([]*VirtualUser, virtualUsers),
		StartTime:    time.Now(),
		Context:      testCtx,
		Cancel:       cancel,
	}

	// Start virtual users
	for i := 0; i < virtualUsers; i++ {
		vuCtx, vuCancel := context.WithCancel(testCtx)
		vu := &VirtualUser{
			ID:          i,
			ScenarioIdx: i % len(cfg.Scenarios),
			Context:     vuCtx,
			Cancel:      vuCancel,
		}
		execution.VirtualUsers[i] = vu

		go e.runVirtualUser(vu, cfg)
	}

	e.activeTests[cfg.TestID] = execution

	// Monitor test completion
	go func() {
		<-testCtx.Done()
		e.stopTest(cfg.TestID)
	}()

	return nil
}

func (e *Executor) runVirtualUser(vu *VirtualUser, cfg *config.TestConfig) {
	scenario := cfg.Scenarios[vu.ScenarioIdx]

	for {
		select {
		case <-vu.Context.Done():
			return
		default:
			// Execute scenario steps
			for _, step := range scenario.Steps {
				if err := e.executeStep(vu.Context, cfg, scenario.Name, step); err != nil {
					e.resultsCh <- &common.RequestResult{
						WorkerID:     e.cfg.ID,
						TestID:       cfg.TestID,
						ScenarioName: scenario.Name,
						RequestName:  step.Name,
						StartTime:    time.Now(),
						Error:        err,
					}
					continue
				}

				// Simulate think time
				if step.ThinkTime > 0 {
					select {
					case <-vu.Context.Done():
						return
					case <-time.After(step.ThinkTime):
					}
				}
			}
		}
	}
}

func (e *Executor) executeStep(ctx context.Context, cfg *config.TestConfig, scenarioName string, step common.RequestStep) error {
	start := time.Now()
	var result *common.RequestResult

	switch cfg.Protocol {
	case string(common.ProtocolHTTP):
		resp, err := e.httpClient.Execute(ctx, step)
		result = &common.RequestResult{
			WorkerID:     e.cfg.ID,
			TestID:       cfg.TestID,
			ScenarioName: scenarioName,
			RequestName:  step.Name,
			StartTime:    start,
			Duration:     time.Since(start),
			Error:        err,
		}
		if resp != nil {
			result.StatusCode = resp.StatusCode
			result.BytesReceived = resp.BytesReceived
			result.BytesSent = resp.BytesSent
		}

	case string(common.ProtocolGRPC):
		resp, err := e.grpcClient.Execute(ctx, step)
		result = &common.RequestResult{
			WorkerID:     e.cfg.ID,
			TestID:       cfg.TestID,
			ScenarioName: scenarioName,
			RequestName:  step.Name,
			StartTime:    start,
			Duration:     time.Since(start),
			Error:        err,
		}
		if resp != nil {
			result.BytesReceived = resp.BytesReceived
			result.BytesSent = resp.BytesSent
		}

	case string(common.ProtocolWS):
		resp, err := e.wsClient.Execute(ctx, step)
		result = &common.RequestResult{
			WorkerID:     e.cfg.ID,
			TestID:       cfg.TestID,
			ScenarioName: scenarioName,
			RequestName:  step.Name,
			StartTime:    start,
			Duration:     time.Since(start),
			Error:        err,
		}
		if resp != nil {
			result.BytesReceived = resp.BytesReceived
			result.BytesSent = resp.BytesSent
		}

	default:
		return fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}

	// Send result
	e.resultsCh <- result

	// Check assertions
	for _, assertion := range step.Assertions {
		if err := e.validateAssertion(assertion, result); err != nil {
			return fmt.Errorf("assertion failed: %w", err)
		}
	}

	return nil
}

func (e *Executor) validateAssertion(assertion common.Assertion, result *common.RequestResult) error {
	switch assertion.Type {
	case common.AssertStatusCode:
		expected, ok := assertion.Expected.(int)
		if !ok {
			return fmt.Errorf("invalid assertion value type")
		}
		if result.StatusCode != expected {
			return fmt.Errorf("expected status code %d, got %d", expected, result.StatusCode)
		}

	case common.AssertLatency:
		expected, ok := assertion.Expected.(time.Duration)
		if !ok {
			return fmt.Errorf("invalid assertion value type")
		}
		if result.Duration > expected {
			return fmt.Errorf("latency %v exceeded threshold %v", result.Duration, expected)
		}
	}

	return nil
}

// StopTest stops all virtual users for a test
func (e *Executor) stopTest(testID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	execution, exists := e.activeTests[testID]
	if !exists {
		return fmt.Errorf("test not found: %s", testID)
	}

	// Cancel all virtual users
	execution.Cancel()

	// Clean up
	delete(e.activeTests, testID)

	return nil
}

// GetActiveTests returns information about currently running tests
func (e *Executor) GetActiveTests() map[string]int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	active := make(map[string]int)
	for testID, execution := range e.activeTests {
		active[testID] = len(execution.VirtualUsers)
	}
	return active
}
