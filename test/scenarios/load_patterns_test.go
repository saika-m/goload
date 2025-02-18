package scenarios

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/saika-m/goload/pkg/config"
	"github.com/saika-m/goload/pkg/master"
	"github.com/saika-m/goload/pkg/worker"
	"github.com/saika-m/goload/test/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*master.Master, []*worker.Worker, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	// Setup master
	m, err := master.NewMaster(fixtures.MasterConfigs["default"])
	require.NoError(t, err)

	// Start master in background
	go func() {
		err := m.Start(ctx)
		require.NoError(t, err)
	}()

	// Create workers
	workers := make([]*worker.Worker, 3)
	for i := 0; i < 3; i++ {
		cfg := fixtures.WorkerConfigs["default"]
		cfg.ID = fmt.Sprintf("worker-%d", i)
		w, err := worker.NewWorker(cfg)
		require.NoError(t, err)

		// Start worker in background
		go func() {
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		workers[i] = w
	}

	// Allow time for workers to register
	time.Sleep(2 * time.Second)

	return m, workers, ctx, cancel
}

func TestConstantLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	m, workers, ctx, cancel := setupTest(t)
	defer cancel()

	cfg := &config.TestConfig{
		VirtualUsers: 100,
		Duration:     30 * time.Second,
		Target:       "http://httpbin.org",
		Protocol:     "HTTP",
		Distribution: config.Distribution{
			Pattern: config.LoadPatternConstant,
		},
		Scenarios: []config.ScenarioConfig{
			{
				Name:   "constant-load",
				Weight: 1.0,
				Steps: []config.RequestStep{
					{
						Name:      "get-request",
						Method:    "GET",
						Path:      "/get",
						ThinkTime: time.Second,
					},
				},
			},
		},
	}

	// Start test
	test, err := m.StartTest(ctx, cfg)
	require.NoError(t, err)

	// Monitor test progress
	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastActiveUsers int
	steadyStateReached := false

	for {
		select {
		case <-ticker.C:
			status, err := m.GetTestStatus(test.Config.TestID)
			require.NoError(t, err)

			if !steadyStateReached && status.ActiveUsers == cfg.VirtualUsers {
				steadyStateReached = true
			}

			if steadyStateReached {
				assert.InDelta(t, lastActiveUsers, status.ActiveUsers, float64(cfg.VirtualUsers)*0.1)
			}

			lastActiveUsers = status.ActiveUsers

			if status.State == "COMPLETED" {
				// Verify test duration
				assert.InDelta(t, cfg.Duration.Seconds(), time.Since(start).Seconds(), 5.0)
				// Verify success rate
				assert.Less(t, status.ErrorRate, 0.05) // Less than 5% error rate
				return
			}
		case <-ctx.Done():
			t.Fatal("Test cancelled")
			return
		}
	}
}

func TestRampUpLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	m, workers, ctx, cancel := setupTest(t)
	defer cancel()

	cfg := &config.TestConfig{
		VirtualUsers: 100,
		Duration:     60 * time.Second,
		Target:       "http://httpbin.org",
		Protocol:     "HTTP",
		Distribution: config.Distribution{
			Pattern:  config.LoadPatternRampUp,
			RampUp:   30 * time.Second,
			RampDown: 10 * time.Second,
		},
		Scenarios: []config.ScenarioConfig{
			{
				Name:   "ramp-up-load",
				Weight: 1.0,
				Steps: []config.RequestStep{
					{
						Name:      "get-request",
						Method:    "GET",
						Path:      "/get",
						ThinkTime: time.Second,
					},
				},
			},
		},
	}

	// Start test
	test, err := m.StartTest(ctx, cfg)
	require.NoError(t, err)

	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastActiveUsers int
	phase := "ramp-up"

	for {
		select {
		case <-ticker.C:
			status, err := m.GetTestStatus(test.Config.TestID)
			require.NoError(t, err)

			elapsed := time.Since(start)

			switch phase {
			case "ramp-up":
				if elapsed < cfg.Distribution.RampUp {
					assert.Greater(t, status.ActiveUsers, lastActiveUsers)
				} else {
					phase = "steady"
				}
			case "steady":
				if elapsed < cfg.Duration-cfg.Distribution.RampDown {
					assert.InDelta(t, cfg.VirtualUsers, status.ActiveUsers, float64(cfg.VirtualUsers)*0.1)
				} else {
					phase = "ramp-down"
				}
			case "ramp-down":
				assert.Less(t, status.ActiveUsers, lastActiveUsers)
			}

			lastActiveUsers = status.ActiveUsers

			if status.State == "COMPLETED" {
				assert.InDelta(t, cfg.Duration.Seconds(), elapsed.Seconds(), 5.0)
				assert.Less(t, status.ErrorRate, 0.05)
				return
			}
		case <-ctx.Done():
			t.Fatal("Test cancelled")
			return
		}
	}
}

func TestStepLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	m, workers, ctx, cancel := setupTest(t)
	defer cancel()

	cfg := &config.TestConfig{
		VirtualUsers: 100,
		Duration:     90 * time.Second,
		Target:       "http://httpbin.org",
		Protocol:     "HTTP",
		Distribution: config.Distribution{
			Pattern: config.LoadPatternStep,
			Steps: []config.LoadStep{
				{Users: 20, Duration: 30 * time.Second},
				{Users: 50, Duration: 30 * time.Second},
				{Users: 100, Duration: 30 * time.Second},
			},
		},
		Scenarios: []config.ScenarioConfig{
			{
				Name:   "step-load",
				Weight: 1.0,
				Steps: []config.RequestStep{
					{
						Name:      "get-request",
						Method:    "GET",
						Path:      "/get",
						ThinkTime: time.Second,
					},
				},
			},
		},
	}

	test, err := m.StartTest(ctx, cfg)
	require.NoError(t, err)

	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	currentStep := 0

	for {
		select {
		case <-ticker.C:
			status, err := m.GetTestStatus(test.Config.TestID)
			require.NoError(t, err)

			elapsed := time.Since(start)
			expectedStep := int(elapsed.Seconds() / 30)

			if expectedStep < len(cfg.Distribution.Steps) && expectedStep != currentStep {
				currentStep = expectedStep
				expectedUsers := cfg.Distribution.Steps[currentStep].Users
				assert.InDelta(t, expectedUsers, status.ActiveUsers, float64(expectedUsers)*0.1)
			}

			if status.State == "COMPLETED" {
				assert.InDelta(t, cfg.Duration.Seconds(), elapsed.Seconds(), 5.0)
				assert.Less(t, status.ErrorRate, 0.05)
				return
			}
		case <-ctx.Done():
			t.Fatal("Test cancelled")
			return
		}
	}
}
