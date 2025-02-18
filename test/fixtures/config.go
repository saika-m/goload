package fixtures

import (
	"time"

	"github.com/saika-m/goload/pkg/config"
)

// TestConfigs provides test configurations for different scenarios
var TestConfigs = map[string]*config.TestConfig{
	"simple-http": {
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
	},
	"complex-http": {
		VirtualUsers: 50,
		Duration:     30 * time.Second,
		Target:       "http://httpbin.org",
		Protocol:     "HTTP",
		Scenarios: []config.ScenarioConfig{
			{
				Name:   "mixed-requests",
				Weight: 0.7,
				Steps: []config.RequestStep{
					{
						Name:      "get-request",
						Method:    "GET",
						Path:      "/get",
						ThinkTime: 1 * time.Second,
					},
					{
						Name:   "post-request",
						Method: "POST",
						Path:   "/post",
						Body:   []byte(`{"test": true}`),
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						ThinkTime: 2 * time.Second,
					},
				},
			},
		},
	},
}

// WorkerConfigs provides test worker configurations
var WorkerConfigs = map[string]*config.WorkerConfig{
	"default": {
		ID:            "test-worker",
		MasterAddress: "localhost:8080",
		Capacity:      100,
		MaxRetries:    3,
	},
	"high-capacity": {
		ID:            "high-capacity-worker",
		MasterAddress: "localhost:8080",
		Capacity:      1000,
		MaxRetries:    5,
	},
}

// MasterConfigs provides test master configurations
var MasterConfigs = map[string]*config.MasterConfig{
	"default": {
		ListenAddress: "localhost",
		APIPort:       8080,
		MetricsPort:   9090,
	},
	"custom": {
		ListenAddress: "0.0.0.0",
		APIPort:       18080,
		MetricsPort:   19090,
	},
}

// MockResponses provides mock HTTP responses for testing
var MockResponses = map[string]string{
	"success": `{"status": "success", "code": 200}`,
	"error":   `{"status": "error", "code": 500, "message": "Internal Server Error"}`,
}

// MockMetrics provides mock metrics data for testing
var MockMetrics = []struct {
	Name   string
	Value  float64
	Labels map[string]string
}{
	{
		Name:  "response_time",
		Value: 100.0,
		Labels: map[string]string{
			"endpoint": "/test",
			"method":   "GET",
		},
	},
	{
		Name:  "requests_total",
		Value: 1000.0,
		Labels: map[string]string{
			"status": "success",
		},
	},
}
