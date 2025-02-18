package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/saika-m/goload/pkg/config"
	"github.com/saika-m/goload/pkg/master"
	"github.com/saika-m/goload/pkg/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (*master.Master, context.Context, context.CancelFunc) {
	// Setup master
	masterCfg := &config.MasterConfig{
		ListenAddress: "localhost",
		APIPort:       18080,
		MetricsPort:   19090,
	}

	m, err := master.NewMaster(masterCfg)
	require.NoError(t, err)

	// Start master
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := m.Start(ctx)
		require.NoError(t, err)
	}()

	// Allow master to start
	time.Sleep(1 * time.Second)

	return m, ctx, cancel
}

func createWorker(t *testing.T, masterAddr string, id string) *worker.Worker {
	workerCfg := &config.WorkerConfig{
		MasterAddress: masterAddr,
		ID:            id,
		Capacity:      100,
	}

	w, err := worker.NewWorker(workerCfg)
	require.NoError(t, err)
	return w
}

func TestCompleteLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	m, ctx, cancel := setupTestEnvironment(t)
	defer cancel()

	// Create and start worker
	w := createWorker(t, "localhost:18080", "test-worker")
	go func() {
		err := w.Start(ctx)
		require.NoError(t, err)
	}()

	// Allow worker to register
	time.Sleep(1 * time.Second)

	// Create test configuration
	testCfg := &config.TestConfig{
		VirtualUsers: 10,
		Duration:     5 * time.Second,
		Target:       "http://httpbin.org",
		Protocol:     "HTTP",
		Scenarios: []config.ScenarioConfig{
			{
				Name:   "simple-get",
				Weight: 1.0,
				Steps: []config.RequestStep{
					{
						Name:   "get-request",
						Method: "GET",
						Path:   "/get",
					},
				},
			},
		},
	}

	// Start test
	test, err := m.StartTest(context.Background(), testCfg)
	require.NoError(t, err)

	// Wait for test to complete
	time.Sleep(7 * time.Second)

	// Get test status
	status, err := m.GetTestStatus(test.Config.TestID)
	require.NoError(t, err)

	// Verify test completed successfully
	assert.Equal(t, "COMPLETED", string(status.State))
	assert.Greater(t, status.TotalRequests, int64(0))
	assert.Less(t, status.ErrorRate, 0.1)
}

func TestWorkerFailover(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	m, ctx, cancel := setupTestEnvironment(t)
	defer cancel()

	// Create multiple workers
	workers := make([]*worker.Worker, 3)
	for i := 0; i < 3; i++ {
		workers[i] = createWorker(t, "localhost:18080", fmt.Sprintf("worker-%d", i))
		go func(w *worker.Worker) {
			err := w.Start(ctx)
			require.NoError(t, err)
		}(workers[i])
	}

	// Allow workers to register
	time.Sleep(2 * time.Second)

	// Start a test
	testCfg := &config.TestConfig{
		VirtualUsers: 30,
		Duration:     20 * time.Second,
		Target:       "http://httpbin.org",
		Protocol:     "HTTP",
		Scenarios: []config.ScenarioConfig{
			{
				Name:   "failover-test",
				Weight: 1.0,
				Steps: []config.RequestStep{
					{
						Name:   "get-request",
						Method: "GET",
						Path:   "/get",
					},
				},
			},
		},
	}

	test, err := m.StartTest(context.Background(), testCfg)
	require.NoError(t, err)

	// Wait for test to stabilize
	time.Sleep(5 * time.Second)

	// Get initial status
	initialStatus, err := m.GetTestStatus(test.Config.TestID)
	require.NoError(t, err)

	// Simulate worker failure by stopping one worker
	workers[0].Stop()

	// Wait for rebalancing
	time.Sleep(5 * time.Second)

	// Get status after failure
	failureStatus, err := m.GetTestStatus(test.Config.TestID)
	require.NoError(t, err)

	// Verify that the test continues with redistributed load
	assert.Equal(t, initialStatus.ActiveUsers, failureStatus.ActiveUsers)
	assert.Less(t, failureStatus.ErrorRate, 0.15) // Allow slightly higher error rate during failover
}

func TestLoadDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	m, ctx, cancel := setupTestEnvironment(t)
	defer cancel()

	// Create workers with different capacities
	workerConfigs := []struct {
		id       string
		capacity int
	}{
		{"worker-small", 50},
		{"worker-medium", 100},
		{"worker-large", 200},
	}

	workers := make([]*worker.Worker, len(workerConfigs))
	for i, cfg := range workerConfigs {
		workerCfg := &config.WorkerConfig{
			MasterAddress: "localhost:18080",
			ID:            cfg.id,
			Capacity:      cfg.capacity,
		}

		w, err := worker.NewWorker(workerCfg)
		require.NoError(t, err)
		workers[i] = w

		go func(worker *worker.Worker) {
			err := worker.Start(ctx)
			require.NoError(t, err)
		}(w)
	}

	// Allow workers to register
	time.Sleep(2 * time.Second)

	// Start a test that will use all workers
	testCfg := &config.TestConfig{
		VirtualUsers: 300, // Sum of all worker capacities
		Duration:     30 * time.Second,
		Target:       "http://httpbin.org",
		Protocol:     "HTTP",
		Scenarios: []config.ScenarioConfig{
			{
				Name:   "distribution-test",
				Weight: 1.0,
				Steps: []config.RequestStep{
					{
						Name:      "get-request",
						Method:    "GET",
						Path:      "/get",
						ThinkTime: 100 * time.Millisecond,
					},
				},
			},
		},
	}

	test, err := m.StartTest(context.Background(), testCfg)
	require.NoError(t, err)

	// Wait for test to stabilize
	time.Sleep(5 * time.Second)

	// Monitor worker load distribution
	for i := 0; i < 3; i++ { // Check distribution multiple times
		workerStats := make(map[string]int)

		for _, worker := range workers {
			info, err := worker.GetStatus()
			require.NoError(t, err)
			workerStats[info.ID] = info.Resources.ActiveUsers
		}

		// Verify load distribution proportional to capacity
		assert.InDelta(t, 50, workerStats["worker-small"], 10)
		assert.InDelta(t, 100, workerStats["worker-medium"], 15)
		assert.InDelta(t, 200, workerStats["worker-large"], 25)

		time.Sleep(5 * time.Second)
	}

	// Wait for test completion
	time.Sleep(20 * time.Second)

	// Verify test completed successfully
	status, err := m.GetTestStatus(test.Config.TestID)
	require.NoError(t, err)

	assert.Equal(t, "COMPLETED", string(status.State))
	assert.Less(t, status.ErrorRate, 0.1)

	// Verify total throughput
	assert.Greater(t, status.TotalRequests, int64(0))
	expectedMinRequests := int64(300 * 30) // VirtualUsers * Seconds
	assert.Greater(t, status.TotalRequests, expectedMinRequests)
}
