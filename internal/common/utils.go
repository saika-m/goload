package common

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// GenerateID generates a random ID
func GenerateID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return prefix + "-" + hex.EncodeToString(bytes)
}

// GetHostInfo returns information about the current host
func GetHostInfo() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return hostname, nil
}

// GetLocalIP returns the non-loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// Check the address type and if it is not a loopback
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// GetResourceStats returns current resource usage statistics
func GetResourceStats() ResourceStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return ResourceStats{
		CPUCores:    runtime.NumCPU(),
		MemoryMB:    int64(m.Sys) / 1024 / 1024,
		CPUUsage:    getCPUUsage(),
		MemoryUsage: float64(m.Alloc) / float64(m.Sys),
	}
}

// getCPUUsage returns the current CPU usage percentage
func getCPUUsage() float64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// This is a simple approximation. For more accurate measurements,
	// you would need to use platform-specific APIs or libraries like gopsutil
	return float64(stats.NumGC) / float64(runtime.NumCPU()) * 100
}

// CalculatePercentile calculates the nth percentile from a slice of durations
func CalculatePercentile(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Sort durations
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate index
	index := int(float64(len(sorted)-1) * percentile / 100)
	return sorted[index]
}

// CalculateStats calculates basic statistics from a slice of request results
func CalculateStats(results []RequestResult) (avg, p95, p99 time.Duration, errorRate float64) {
	if len(results) == 0 {
		return 0, 0, 0, 0
	}

	var total time.Duration
	var errors int

	durations := make([]time.Duration, len(results))
	for i, result := range results {
		total += result.Duration
		durations[i] = result.Duration
		if result.Error != nil {
			errors++
		}
	}

	avg = total / time.Duration(len(results))
	p95 = CalculatePercentile(durations, 95)
	p99 = CalculatePercentile(durations, 99)
	errorRate = float64(errors) / float64(len(results))

	return
}

// ParseDuration parses a duration string with support for days
func ParseDuration(s string) (time.Duration, error) {
	// Regular expression to match duration components
	re := regexp.MustCompile(`^((\d+)d)?((\d+)h)?((\d+)m)?((\d+)s)?((\d+)ms)?$`)
	matches := re.FindStringSubmatch(s)

	if matches == nil {
		// Try standard parsing if our custom format doesn't match
		return time.ParseDuration(s)
	}

	var duration time.Duration

	// Parse days
	if matches[2] != "" {
		days, _ := strconv.Atoi(matches[2])
		duration += time.Duration(days) * 24 * time.Hour
	}

	// Parse hours
	if matches[4] != "" {
		hours, _ := strconv.Atoi(matches[4])
		duration += time.Duration(hours) * time.Hour
	}

	// Parse minutes
	if matches[6] != "" {
		minutes, _ := strconv.Atoi(matches[6])
		duration += time.Duration(minutes) * time.Minute
	}

	// Parse seconds
	if matches[8] != "" {
		seconds, _ := strconv.Atoi(matches[8])
		duration += time.Duration(seconds) * time.Second
	}

	// Parse milliseconds
	if matches[10] != "" {
		ms, _ := strconv.Atoi(matches[10])
		duration += time.Duration(ms) * time.Millisecond
	}

	return duration, nil
}

// ValidateConfig validates a scenario configuration
func ValidateConfig(scenario ScenarioConfig) error {
	if scenario.Name == "" {
		return NewLoadTestError(ErrConfigInvalid, "scenario name cannot be empty", nil)
	}
	if len(scenario.Steps) == 0 {
		return NewLoadTestError(ErrConfigInvalid, "scenario must have at least one step", nil)
	}

	totalWeight := scenario.Weight
	if totalWeight <= 0 {
		return NewLoadTestError(ErrConfigInvalid, "scenario weight must be positive", nil)
	}

	for _, step := range scenario.Steps {
		if step.Name == "" {
			return NewLoadTestError(ErrConfigInvalid, "step name cannot be empty", nil)
		}
		if step.Method == "" {
			return NewLoadTestError(ErrConfigInvalid, "step method cannot be empty", nil)
		}
		if step.Path == "" {
			return NewLoadTestError(ErrConfigInvalid, "step path cannot be empty", nil)
		}
	}

	return nil
}

// MergeHeaders merges two sets of headers, with override taking precedence
func MergeHeaders(base, override map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// NewLoadTestError creates a new LoadTestError
func NewLoadTestError(code ErrorCode, message string, err error) *LoadTestError {
	return &LoadTestError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// FormatBytes formats a byte count into a human-readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats a duration into a human-readable string
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	parts := []string{}

	if d >= 24*time.Hour {
		days := d / (24 * time.Hour)
		d -= days * 24 * time.Hour
		parts = append(parts, fmt.Sprintf("%dd", days))
	}

	if d >= time.Hour {
		hours := d / time.Hour
		d -= hours * time.Hour
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}

	if d >= time.Minute {
		minutes := d / time.Minute
		d -= minutes * time.Minute
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	if d >= time.Second {
		seconds := d / time.Second
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	return strings.Join(parts, " ")
}
