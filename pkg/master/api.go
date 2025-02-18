package master

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/saika-m/goload/internal/common"
	"github.com/saika-m/goload/pkg/config"
)

// API handles HTTP endpoints for the master node
type API struct {
	master *Master
	router *mux.Router
	server *http.Server
	mu     sync.RWMutex
}

// NewAPI creates a new API server
func NewAPI(master *Master, cfg *config.MasterConfig) *API {
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
	var cfg config.TestConfig
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
