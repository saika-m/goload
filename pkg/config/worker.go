package config

import (
	"fmt"
	"time"

	"github.com/saika-m/goload/internal/common"
)

// WorkerConfig contains configuration for a worker node
type WorkerConfig struct {
	// Worker identification
	ID       string
	Hostname string

	// Connection settings
	MasterAddress     string
	MetricsPort       int
	HeartbeatInterval time.Duration

	// Performance settings
	Capacity   int
	WorkDir    string
	MaxRetries int

	// Resource limits
	MaxCPUPercent    float64
	MaxMemoryPercent float64

	// Network settings
	DialTimeout     time.Duration
	KeepAlive       time.Duration
	MaxIdleConns    int
	IdleConnTimeout time.Duration
}

// DefaultWorkerConfig returns the default worker configuration
func DefaultWorkerConfig() *WorkerConfig {
	hostname, _ := common.GetHostInfo()

	return &WorkerConfig{
		ID:                common.GenerateID("worker"),
		Hostname:          hostname,
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

// WorkerInfo returns information about the worker
func (c *WorkerConfig) WorkerInfo() common.WorkerInfo {
	return common.WorkerInfo{
		ID:        c.ID,
		Hostname:  c.Hostname,
		Capacity:  c.Capacity,
		Resources: common.GetResourceStats(),
	}
}

// WithDefaults returns a copy of the configuration with default values set
func (c *WorkerConfig) WithDefaults() *WorkerConfig {
	defaultCfg := DefaultWorkerConfig()

	if c.ID == "" {
		c.ID = defaultCfg.ID
	}
	if c.Hostname == "" {
		c.Hostname = defaultCfg.Hostname
	}
	if c.MetricsPort == 0 {
		c.MetricsPort = defaultCfg.MetricsPort
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = defaultCfg.HeartbeatInterval
	}
	if c.Capacity == 0 {
		c.Capacity = defaultCfg.Capacity
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaultCfg.MaxRetries
	}
	if c.MaxCPUPercent == 0 {
		c.MaxCPUPercent = defaultCfg.MaxCPUPercent
	}
	if c.MaxMemoryPercent == 0 {
		c.MaxMemoryPercent = defaultCfg.MaxMemoryPercent
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = defaultCfg.DialTimeout
	}
	if c.KeepAlive == 0 {
		c.KeepAlive = defaultCfg.KeepAlive
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = defaultCfg.MaxIdleConns
	}
	if c.IdleConnTimeout == 0 {
		c.IdleConnTimeout = defaultCfg.IdleConnTimeout
	}

	return c
}

// ConnectionPoolConfig returns the connection pool configuration
type ConnectionPoolConfig struct {
	MaxIdleConns    int
	IdleConnTimeout time.Duration
	MaxConnsPerHost int
	DialTimeout     time.Duration
	KeepAlive       time.Duration
}

// GetConnectionPoolConfig returns the connection pool configuration
func (c *WorkerConfig) GetConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxIdleConns:    c.MaxIdleConns,
		IdleConnTimeout: c.IdleConnTimeout,
		MaxConnsPerHost: c.Capacity, // One connection per virtual user
		DialTimeout:     c.DialTimeout,
		KeepAlive:       c.KeepAlive,
	}
}
