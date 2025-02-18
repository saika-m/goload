package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/saika-m/goload/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"empty prefix", ""},
		{"test prefix", "test"},
		{"long prefix", "very-long-prefix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1 := common.GenerateID(tt.prefix)
			id2 := common.GenerateID(tt.prefix)

			// Check that IDs have the correct prefix
			if tt.prefix != "" {
				assert.True(t, len(id1) > len(tt.prefix))
				assert.True(t, len(id2) > len(tt.prefix))
				assert.Equal(t, tt.prefix, id1[:len(tt.prefix)])
				assert.Equal(t, tt.prefix, id2[:len(tt.prefix)])
			}

			// Check that generated IDs are unique
			assert.NotEqual(t, id1, id2)
		})
	}
}

func TestCalculatePercentile(t *testing.T) {
	tests := []struct {
		name       string
		durations  []time.Duration
		percentile float64
		expected   time.Duration
	}{
		{
			name: "p95 with sorted data",
			durations: []time.Duration{
				100 * time.Millisecond,
				200 * time.Millisecond,
				300 * time.Millisecond,
				400 * time.Millisecond,
				500 * time.Millisecond,
			},
			percentile: 95,
			expected:   475 * time.Millisecond,
		},
		{
			name: "p50 with unsorted data",
			durations: []time.Duration{
				500 * time.Millisecond,
				100 * time.Millisecond,
				300 * time.Millisecond,
				200 * time.Millisecond,
				400 * time.Millisecond,
			},
			percentile: 50,
			expected:   300 * time.Millisecond,
		},
		{
			name:       "empty slice",
			durations:  []time.Duration{},
			percentile: 95,
			expected:   0,
		},
		{
			name: "single value",
			durations: []time.Duration{
				100 * time.Millisecond,
			},
			percentile: 95,
			expected:   100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.CalculatePercentile(tt.durations, tt.percentile)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name            string
		results         []common.RequestResult
		expectedAvg     time.Duration
		expectedP95     time.Duration
		expectedP99     time.Duration
		expectedErrRate float64
	}{
		{
			name: "normal distribution",
			results: []common.RequestResult{
				{Duration: 100 * time.Millisecond},
				{Duration: 200 * time.Millisecond},
				{Duration: 300 * time.Millisecond},
				{Duration: 400 * time.Millisecond},
				{Duration: 500 * time.Millisecond},
			},
			expectedAvg:     300 * time.Millisecond,
			expectedP95:     475 * time.Millisecond,
			expectedP99:     495 * time.Millisecond,
			expectedErrRate: 0,
		},
		{
			name: "with errors",
			results: []common.RequestResult{
				{Duration: 100 * time.Millisecond, Error: nil},
				{Duration: 200 * time.Millisecond, Error: fmt.Errorf("error")},
				{Duration: 300 * time.Millisecond, Error: nil},
				{Duration: 400 * time.Millisecond, Error: fmt.Errorf("error")},
			},
			expectedAvg:     250 * time.Millisecond,
			expectedP95:     380 * time.Millisecond,
			expectedP99:     396 * time.Millisecond,
			expectedErrRate: 0.5,
		},
		{
			name:            "empty results",
			results:         []common.RequestResult{},
			expectedAvg:     0,
			expectedP95:     0,
			expectedP99:     0,
			expectedErrRate: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			avg, p95, p99, errRate := common.CalculateStats(tt.results)
			assert.Equal(t, tt.expectedAvg, avg)
			assert.Equal(t, tt.expectedP95, p95)
			assert.Equal(t, tt.expectedP99, p99)
			assert.Equal(t, tt.expectedErrRate, errRate)
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			name:     "standard duration",
			input:    "1h30m",
			expected: 90 * time.Minute,
			hasError: false,
		},
		{
			name:     "with days",
			input:    "2d3h30m",
			expected: 51*time.Hour + 30*time.Minute,
			hasError: false,
		},
		{
			name:     "only days",
			input:    "5d",
			expected: 120 * time.Hour,
			hasError: false,
		},
		{
			name:     "complex duration",
			input:    "1d6h30m45s",
			expected: 30*time.Hour + 30*time.Minute + 45*time.Second,
			hasError: false,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := common.ParseDuration(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, duration)
			}
		})
	}
}

func TestMergeHeaders(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]string
		override map[string]string
		expected map[string]string
	}{
		{
			name: "override existing headers",
			base: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
			override: map[string]string{
				"Content-Type": "text/plain",
				"User-Agent":   "test-agent",
			},
			expected: map[string]string{
				"Content-Type": "text/plain",
				"Accept":       "application/json",
				"User-Agent":   "test-agent",
			},
		},
		{
			name:     "empty base",
			base:     map[string]string{},
			override: map[string]string{"Test": "value"},
			expected: map[string]string{"Test": "value"},
		},
		{
			name:     "empty override",
			base:     map[string]string{"Test": "value"},
			override: map[string]string{},
			expected: map[string]string{"Test": "value"},
		},
		{
			name:     "both empty",
			base:     map[string]string{},
			override: map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.MergeHeaders(tt.base, tt.override)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"bytes", 500, "500 B"},
		{"kilobytes", 1500, "1.5 KB"},
		{"megabytes", 1500000, "1.4 MB"},
		{"gigabytes", 1500000000, "1.4 GB"},
		{"zero", 0, "0 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 45 * time.Second, "45s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 2 * time.Hour, "2h"},
		{"complex", 2*time.Hour + 30*time.Minute + 45*time.Second, "2h 30m 45s"},
		{"days", 48 * time.Hour, "2d"},
		{"zero", 0, "0ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.FormatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}
