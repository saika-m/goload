package common

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LoadTestError represents an error that occurred during load testing
type LoadTestError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *LoadTestError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ErrorCode represents different types of errors
type ErrorCode string

const (
	ErrConfigInvalid     ErrorCode = "CONFIG_INVALID"
	ErrWorkerUnavailable ErrorCode = "WORKER_UNAVAILABLE"
	ErrTestNotFound      ErrorCode = "TEST_NOT_FOUND"
	ErrTestFailed        ErrorCode = "TEST_FAILED"
	ErrInvalidProtocol   ErrorCode = "INVALID_PROTOCOL"
	ErrConnectionFailed  ErrorCode = "CONNECTION_FAILED"
	ErrTimeout           ErrorCode = "TIMEOUT"
)

// Protocol represents supported protocols
type Protocol string

const (
	ProtocolHTTP      Protocol = "HTTP"
	ProtocolGRPC      Protocol = "GRPC"
	ProtocolWebSocket Protocol = "WEBSOCKET"
	ProtocolWS        Protocol = "WS" //alias for ProtocolWebSocket
)

// LoadPattern represents different patterns of load generation
type LoadPattern string

const (
	LoadPatternConstant LoadPattern = "CONSTANT"
	LoadPatternRampUp   LoadPattern = "RAMP_UP"
	LoadPatternStep     LoadPattern = "STEP"
	LoadPatternRandom   LoadPattern = "RANDOM"
)

// Distribution defines how load should be distributed
type Distribution struct {
	Pattern    LoadPattern
	RampUp     time.Duration
	RampDown   time.Duration
	Geographic map[string]float64
}

// ResourceStats contains resource usage statistics
type ResourceStats struct {
	CPUCores    int
	MemoryMB    int64
	CPUUsage    float64
	MemoryUsage float64
}

// WorkerInfo contains information about a worker node
type WorkerInfo struct {
	ID        string
	Hostname  string
	Capacity  int
	Resources ResourceStats
}

// WorkerState represents the current state of a worker
type WorkerState struct {
	Info          *WorkerInfo
	Status        string
	ActiveUsers   int
	CurrentTestID string
	LastHeartbeat time.Time
}

// TestStatus represents the current status of a test
type TestStatus struct {
	TestID          string
	State           TestState
	ActiveUsers     int
	TotalRequests   int64
	ErrorRate       float64
	AvgResponseTime time.Duration
	Message         string
}

// TestState represents the state of a test
type TestState string

const (
	TestStatePending   TestState = "PENDING"
	TestStateRunning   TestState = "RUNNING"
	TestStateCompleted TestState = "COMPLETED"
	TestStateFailed    TestState = "FAILED"
	TestStateCancelled TestState = "CANCELLED"
)

// RequestResult represents the result of a single request
type RequestResult struct {
	WorkerID      string
	TestID        string
	ScenarioName  string
	RequestName   string
	StartTime     time.Time
	Duration      time.Duration
	StatusCode    int
	Error         error
	BytesSent     int64
	BytesReceived int64
}

// MetricsBatch represents a batch of metrics from a worker
type MetricsBatch struct {
	WorkerID  string
	TestID    string
	Metrics   []*Metric
	Timestamp time.Time
}
type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
	Timer     MetricType = "timer"
)

// Metric represents a single metric measurement
type Metric struct {
	Name      string
	Value     float64
	Type      MetricType
	Labels    map[string]string
	Timestamp time.Time
}

// Test represents a load test instance
type Test struct {
	Config    *TestConfig
	Status    *TestStatus
	Workers   map[string]*WorkerState
	StartTime time.Time
	EndTime   time.Time
	Mu        sync.RWMutex
}

// Scheduler defines the interface for worker scheduling and load distribution
type Scheduler interface {
	// AllocateWorkers distributes virtual users across available workers
	AllocateWorkers(test *Test) (map[string]int, error)

	// RebalanceWorkers redistributes load when workers join or leave
	RebalanceWorkers(testID string) error
}

// BaseScheduler provides a basic implementation of the Scheduler interface
type BaseScheduler struct {
	Orchestrator *Orchestrator
	Mu           sync.RWMutex
}

// ScenarioConfig defines a test scenario
type ScenarioConfig struct {
	Name   string
	Weight float64
	Steps  []RequestStep
}

// RequestStep defines a single step in a scenario
type RequestStep struct {
	Name       string
	Method     string
	Path       string
	Headers    map[string]string
	Body       []byte
	ThinkTime  time.Duration
	Assertions []Assertion
}

// Assertion defines a validation check
type Assertion struct {
	Type     AssertionType
	Target   string
	Expected interface{}
}

// AssertionType represents different types of assertions
type AssertionType string

const (
	AssertStatusCode    AssertionType = "STATUS_CODE"
	AssertResponseTime  AssertionType = "RESPONSE_TIME"
	AssertLatency       AssertionType = "LATENCY"
	AssertBodyContains  AssertionType = "BODY_CONTAINS"
	AssertBodyMatches   AssertionType = "BODY_MATCHES"
	AssertHeaderMatches AssertionType = "HEADER_MATCHES"
)

// MasterConfig contains configuration for the master node
type MasterConfig struct {
	ListenAddress     string        `json:"listen_address"`
	APIPort           int           `json:"api_port"`
	MetricsPort       int           `json:"metrics_port"`
	DefaultTimeout    time.Duration `json:"default_timeout"`
	MaxWorkers        int           `json:"max_workers"`
	MaxVirtualUsers   int           `json:"max_virtual_users"`
	ReportingInterval time.Duration `json:"reporting_interval"`
	StoragePath       string        `json:"storage_path"`
	RetentionPeriod   time.Duration `json:"retention_period"`
}

// WorkerConfig contains configuration for a worker node
type WorkerConfig struct {
	ID                string        `json:"id"`
	Hostname          string        `json:"hostname"`
	MasterAddress     string        `json:"master_address"`
	MetricsPort       int           `json:"metrics_port"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	Capacity          int           `json:"capacity"`
	WorkDir           string        `json:"work_dir"`
	MaxRetries        int           `json:"max_retries"`
	MaxCPUPercent     float64       `json:"max_cpu_percent"`
	MaxMemoryPercent  float64       `json:"max_memory_percent"`
	DialTimeout       time.Duration `json:"dial_timeout"`
	KeepAlive         time.Duration `json:"keep_alive"`
	MaxIdleConns      int           `json:"max_idle_conns"`
	IdleConnTimeout   time.Duration `json:"idle_conn_timeout"`
	workerInfo        *WorkerInfo
}

// TestConfig contains configuration for a load test
type TestConfig struct {
	TestID          string            `json:"test_id"`
	VirtualUsers    int               `json:"virtual_users"`
	Duration        time.Duration     `json:"duration"`
	Target          string            `json:"target"`
	Protocol        Protocol          `json:"protocol"`
	Headers         map[string]string `json:"headers"`
	Timeout         time.Duration     `json:"timeout"`
	FollowRedirects bool              `json:"follow_redirects"`
	Scenarios       []ScenarioConfig  `json:"scenarios"`
	Distribution    Distribution      `json:"distribution"`
}

// ConnectionPoolConfig defines connection pool settings
type ConnectionPoolConfig struct {
	MaxIdleConns    int
	IdleConnTimeout time.Duration
	MaxConnsPerHost int
	DialTimeout     time.Duration
	KeepAlive       time.Duration
}

// DefaultMasterConfig returns the default master configuration
func DefaultMasterConfig() *MasterConfig {
	return &MasterConfig{
		ListenAddress:     "0.0.0.0",
		APIPort:           8080,
		MetricsPort:       9090,
		DefaultTimeout:    30 * time.Second,
		MaxWorkers:        100,
		MaxVirtualUsers:   10000,
		ReportingInterval: time.Second,
		StoragePath:       "./data",
		RetentionPeriod:   7 * 24 * time.Hour,
	}
}

// DefaultWorkerConfig returns the default worker configuration
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		MetricsPort:       9091,
		HeartbeatInterval: 5 * time.Second,
		Capacity:          100,
		WorkDir:           "/tmp/goload-worker",
		MaxRetries:        3,
		MaxCPUPercent:     80.0,
		MaxMemoryPercent:  80.0,
		DialTimeout:       5 * time.Second,
		KeepAlive:         30 * time.Second,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
	}
}

// Validate validates the master configuration
func (c *MasterConfig) Validate() error {
	if c.ListenAddress == "" {
		return fmt.Errorf("listen address cannot be empty")
	}
	if c.APIPort <= 0 {
		return fmt.Errorf("invalid API port")
	}
	if c.MetricsPort <= 0 {
		return fmt.Errorf("invalid metrics port")
	}
	if c.MaxWorkers <= 0 {
		return fmt.Errorf("max workers must be positive")
	}
	if c.MaxVirtualUsers <= 0 {
		return fmt.Errorf("max virtual users must be positive")
	}
	return nil
}

// Validate validates the worker configuration
func (c *WorkerConfig) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("worker ID cannot be empty")
	}
	if c.MasterAddress == "" {
		return fmt.Errorf("master address cannot be empty")
	}
	if c.Capacity <= 0 {
		return fmt.Errorf("capacity must be positive")
	}
	if c.WorkDir == "" {
		return fmt.Errorf("work directory cannot be empty")
	}
	if c.MaxCPUPercent <= 0 || c.MaxCPUPercent > 100 {
		return fmt.Errorf("invalid max CPU percentage")
	}
	if c.MaxMemoryPercent <= 0 || c.MaxMemoryPercent > 100 {
		return fmt.Errorf("invalid max memory percentage")
	}
	return nil
}

// Validate validates the test configuration
func (c *TestConfig) Validate() error {
	if c.VirtualUsers <= 0 {
		return fmt.Errorf("virtual users must be positive")
	}
	if c.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if c.Target == "" {
		return fmt.Errorf("target cannot be empty")
	}
	if len(c.Scenarios) == 0 {
		return fmt.Errorf("at least one scenario is required")
	}
	return nil
}

// GetConnectionPoolConfig returns connection pool configuration
func (c *WorkerConfig) GetConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxIdleConns:    c.MaxIdleConns,
		IdleConnTimeout: c.IdleConnTimeout,
		MaxConnsPerHost: c.Capacity,
		DialTimeout:     c.DialTimeout,
		KeepAlive:       c.KeepAlive,
	}
}

func (c *WorkerConfig) WorkerInfo() *WorkerInfo {
	if c.workerInfo == nil {
		c.workerInfo = &WorkerInfo{
			ID:        c.ID,
			Hostname:  c.Hostname,
			Capacity:  c.Capacity,
			Resources: ResourceStats{},
		}
	}
	return c.workerInfo
}

type OrchestrationMethods interface {
	StartTest(ctx context.Context, cfg *TestConfig) (*Test, error)
	GetTest(testID string) (*Test, error)
	StopTest(testID string) error
	GetTestStatus(testID string) (*TestStatus, error)
	RegisterWorker(workerInfo *WorkerInfo) (*WorkerState, error)
	UpdateWorkerHeartbeat(workerID string, stats *ResourceStats) error
	RemoveWorker(workerID string) error
}

type Orchestrator struct {
	Workers              map[string]*WorkerState
	Tests                map[string]*Test
	Scheduler            Scheduler
	Mu                   sync.RWMutex
	OrchestrationMethods // Embed the interface
}
