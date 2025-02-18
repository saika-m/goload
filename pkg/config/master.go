package config

import (
	"fmt"
	"time"

	"github.com/saika-m/goload/internal/common"
)

// MasterConfig contains configuration for the master node
type MasterConfig struct {
	// Server configuration
	ListenAddress string
	APIPort       int
	MetricsPort   int

	// Test configuration
	DefaultTimeout    time.Duration
	MaxWorkers        int
	MaxVirtualUsers   int
	ReportingInterval time.Duration

	// Storage configuration
	StoragePath     string
	RetentionPeriod time.Duration
}

// TestConfig contains configuration for a load test
type TestConfig struct {
	TestID       string
	VirtualUsers int
	Duration     time.Duration
	Target       string
	Protocol     string

	// HTTP-specific configuration
	Headers         map[string]string
	Timeout         time.Duration
	FollowRedirects bool

	// Test scenarios
	Scenarios []common.ScenarioConfig

	// Distribution configuration
	Geographic       map[string]float64
	RampUpDuration   time.Duration
	RampDownDuration time.Duration
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
		ReportingInterval: 1 * time.Second,
		StoragePath:       "./data",
		RetentionPeriod:   7 * 24 * time.Hour, // 7 days
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

// Validate validates the test configuration
func (c *TestConfig) Validate() error {
	if c.VirtualUsers <= 0 {
		return common.NewLoadTestError(common.ErrConfigInvalid, "virtual users must be positive", nil)
	}
	if c.Duration <= 0 {
		return common.NewLoadTestError(common.ErrConfigInvalid, "duration must be positive", nil)
	}
	if c.Target == "" {
		return common.NewLoadTestError(common.ErrConfigInvalid, "target cannot be empty", nil)
	}

	// Validate scenarios
	if len(c.Scenarios) == 0 {
		return common.NewLoadTestError(common.ErrConfigInvalid, "at least one scenario is required", nil)
	}
	for _, scenario := range c.Scenarios {
		if err := common.ValidateConfig(scenario); err != nil {
			return err
		}
	}

	return nil
}

// WithDefaults returns a copy of the configuration with default values set
func (c *TestConfig) WithDefaults() *TestConfig {
	if c.TestID == "" {
		c.TestID = common.GenerateID("test")
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	if c.Geographic == nil {
		c.Geographic = map[string]float64{"default": 100}
	}
	return c
}
