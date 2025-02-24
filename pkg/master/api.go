package master

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/saika-m/goload/internal/common"
)

// Master represents the master node in the distributed load testing system
type Master struct {
	config       *common.MasterConfig
	orchestrator *common.Orchestrator
	mu           sync.RWMutex
}

// NewMaster creates a new master instance
func NewMaster(cfg *common.MasterConfig) (*Master, error) {
	if cfg == nil {
		cfg = common.DefaultMasterConfig()
	}

	orchestrator := NewOrchestrator(cfg)

	return &Master{
		config:       cfg,
		orchestrator: orchestrator,
	}, nil
}

// StartTest starts a new load test
func (m *Master) StartTest(ctx context.Context, cfg *common.TestConfig) (*common.Test, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return m.orchestrator.StartTest(ctx, cfg)
}

// GetTest retrieves a test by ID
func (m *Master) GetTest(testID string) (*common.Test, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.orchestrator.GetTest(testID)
}

// StopTest stops a running test
func (m *Master) StopTest(testID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.orchestrator.StopTest(testID)
}

// GetTestStatus gets the current status of a test
func (m *Master) GetTestStatus(testID string) (*common.TestStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.orchestrator.GetTestStatus(testID)
}

// RegisterWorker registers a new worker node
func (m *Master) RegisterWorker(ctx context.Context, info *common.WorkerInfo) (*common.WorkerState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker := &common.WorkerState{
		Info:          info,
		LastHeartbeat: time.Now(),
		Status:        "ready",
	}

	m.orchestrator.Workers[info.ID] = worker
	return worker, nil
}

// GetWorker retrieves information about a specific worker
func (m *Master) GetWorker(workerID string) (*common.WorkerState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	worker, exists := m.orchestrator.Workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker not found: %s", workerID)
	}

	return worker, nil
}

// UpdateWorkerStatus updates a worker's status and resource stats
func (m *Master) UpdateWorkerStatus(workerID string, stats *common.ResourceStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker, exists := m.orchestrator.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	worker.LastHeartbeat = time.Now()
	worker.Info.Resources = *stats

	return nil
}

// GetMetrics returns current metrics for all running tests
func (m *Master) GetMetrics() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})

	// Aggregate test metrics
	for testID, test := range m.orchestrator.Tests {
		test.Mu.RLock()
		metrics[testID] = map[string]interface{}{
			"status":            test.Status,
			"active_users":      test.Status.ActiveUsers,
			"total_requests":    test.Status.TotalRequests,
			"error_rate":        test.Status.ErrorRate,
			"avg_response_time": test.Status.AvgResponseTime.Milliseconds(),
			"start_time":        test.StartTime,
			"end_time":          test.EndTime,
		}
		test.Mu.RUnlock()
	}

	// Add worker metrics
	workerMetrics := make(map[string]interface{})
	for workerID, worker := range m.orchestrator.Workers {
		workerMetrics[workerID] = map[string]interface{}{
			"status":         worker.Status,
			"active_users":   worker.ActiveUsers,
			"cpu_usage":      worker.Info.Resources.CPUUsage,
			"memory_usage":   worker.Info.Resources.MemoryUsage,
			"last_heartbeat": worker.LastHeartbeat,
		}
	}
	metrics["workers"] = workerMetrics

	return metrics, nil
}

type API struct {
	master *Master
	router *mux.Router
	server *http.Server
}

// NewAPI creates a new API server
func NewAPI(master *Master, cfg *common.MasterConfig) *API {
	api := &API{
		master: master,
		router: mux.NewRouter(),
	}

	// Setup routes
	api.setupRoutes()

	// Create HTTP server
	api.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.APIPort),
		Handler: api.router,
	}

	return api
}

// Start starts the API server
func (a *API) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("api server error: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		return a.server.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func (a *API) setupRoutes() {
	// API versioning
	v1 := a.router.PathPrefix("/api/v1").Subrouter()

	// Test management
	v1.HandleFunc("/tests", a.handleCreateTest).Methods("POST")
	v1.HandleFunc("/tests/{id}", a.handleGetTest).Methods("GET")
	v1.HandleFunc("/tests/{id}", a.handleStopTest).Methods("DELETE")
	v1.HandleFunc("/tests/{id}/status", a.handleGetTestStatus).Methods("GET")

	// Worker management
	v1.HandleFunc("/workers", a.handleRegisterWorker).Methods("POST")
	v1.HandleFunc("/workers/{id}", a.handleGetWorker).Methods("GET")
	v1.HandleFunc("/workers/{id}/heartbeat", a.handleWorkerHeartbeat).Methods("POST")

	// Metrics
	v1.HandleFunc("/metrics", a.handleGetMetrics).Methods("GET")
}

func (a *API) handleCreateTest(w http.ResponseWriter, r *http.Request) {
	var cfg common.TestConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := cfg.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	test, err := a.master.StartTest(r.Context(), &cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(test)
}

func (a *API) handleGetTest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	testID := vars["id"]

	test, err := a.master.GetTest(testID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(test)
}

func (a *API) handleStopTest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	testID := vars["id"]

	if err := a.master.StopTest(testID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) handleGetTestStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	testID := vars["id"]

	status, err := a.master.GetTestStatus(testID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func (a *API) handleRegisterWorker(w http.ResponseWriter, r *http.Request) {
	var info common.WorkerInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	worker, err := a.master.RegisterWorker(r.Context(), &info)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(worker)
}

func (a *API) handleGetWorker(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workerID := vars["id"]

	worker, err := a.master.GetWorker(workerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(worker)
}

func (a *API) handleWorkerHeartbeat(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workerID := vars["id"]

	var stats common.ResourceStats
	if err := json.NewDecoder(r.Body).Decode(&stats); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.master.UpdateWorkerStatus(workerID, &stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := a.master.GetMetrics()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(metrics)
}
