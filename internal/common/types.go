package common

import (
	"fmt"
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

// Metric represents a single metric measurement
type Metric struct {
	Name      string
	Value     float64
	Labels    map[string]string
	Type      string
	Timestamp time.Time
}

// WorkerInfo contains information about a worker node
type WorkerInfo struct {
	ID        string
	Hostname  string
	Capacity  int
	Resources ResourceStats
}

// ResourceStats contains resource usage statistics
type ResourceStats struct {
	CPUCores    int
	MemoryMB    int64
	CPUUsage    float64
	MemoryUsage float64
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

// AssertionType represents different types of assertions
type AssertionType string

const (
	AssertStatusCode    AssertionType = "STATUS_CODE"
	AssertResponseTime  AssertionType = "RESPONSE_TIME"
	AssertBodyContains  AssertionType = "BODY_CONTAINS"
	AssertBodyMatches   AssertionType = "BODY_MATCHES"
	AssertHeaderMatches AssertionType = "HEADER_MATCHES"
)

// Assertion defines a validation check
type Assertion struct {
	Type     AssertionType
	Target   string
	Expected interface{}
}

// MetricType represents different types of metrics
type MetricType string

const (
	MetricTypeCounter   MetricType = "COUNTER"
	MetricTypeGauge     MetricType = "GAUGE"
	MetricTypeHistogram MetricType = "HISTOGRAM"
	MetricTypeTimer     MetricType = "TIMER"
)

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
