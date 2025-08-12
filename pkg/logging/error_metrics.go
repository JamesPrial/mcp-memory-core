package logging

import (
	"context"
	"sync"
	"time"
)

// ErrorMetrics provides error metrics collection
type ErrorMetrics struct {
	mu                sync.RWMutex
	errorCounts       map[string]int64           // error_code -> count
	errorsByOperation map[string]map[string]int64 // operation -> error_code -> count
	errorsByComponent map[string]map[string]int64 // component -> error_code -> count
	errorRates        *TimeWindowCounter
	totalErrors       int64
	enabled           bool
}

// TimeWindowCounter tracks counts over time windows
type TimeWindowCounter struct {
	mu       sync.RWMutex
	windows  []timeWindow
	size     int
	interval time.Duration
}

type timeWindow struct {
	timestamp time.Time
	count     int64
}

// NewErrorMetrics creates a new error metrics collector
func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		errorCounts:       make(map[string]int64),
		errorsByOperation: make(map[string]map[string]int64),
		errorsByComponent: make(map[string]map[string]int64),
		errorRates:        NewTimeWindowCounter(60, time.Minute), // 60 1-minute windows
		enabled:           true,
	}
}

// NewTimeWindowCounter creates a new time window counter
func NewTimeWindowCounter(windowCount int, interval time.Duration) *TimeWindowCounter {
	return &TimeWindowCounter{
		windows:  make([]timeWindow, 0, windowCount),
		size:     windowCount,
		interval: interval,
	}
}

// RecordError records an error occurrence
func (m *ErrorMetrics) RecordError(ctx context.Context, errorCode, operation, component string) {
	if !m.enabled {
		return
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Increment total errors
	m.totalErrors++
	
	// Increment error code count
	m.errorCounts[errorCode]++
	
	// Increment by operation
	if operation != "" {
		if m.errorsByOperation[operation] == nil {
			m.errorsByOperation[operation] = make(map[string]int64)
		}
		m.errorsByOperation[operation][errorCode]++
	}
	
	// Increment by component
	if component != "" {
		if m.errorsByComponent[component] == nil {
			m.errorsByComponent[component] = make(map[string]int64)
		}
		m.errorsByComponent[component][errorCode]++
	}
	
	// Record in time window
	m.errorRates.Increment()
}

// GetErrorCounts returns error counts by code
func (m *ErrorMetrics) GetErrorCounts() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	counts := make(map[string]int64, len(m.errorCounts))
	for code, count := range m.errorCounts {
		counts[code] = count
	}
	return counts
}

// GetErrorsByOperation returns error counts by operation
func (m *ErrorMetrics) GetErrorsByOperation() map[string]map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]map[string]int64, len(m.errorsByOperation))
	for op, counts := range m.errorsByOperation {
		result[op] = make(map[string]int64, len(counts))
		for code, count := range counts {
			result[op][code] = count
		}
	}
	return result
}

// GetErrorsByComponent returns error counts by component
func (m *ErrorMetrics) GetErrorsByComponent() map[string]map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]map[string]int64, len(m.errorsByComponent))
	for comp, counts := range m.errorsByComponent {
		result[comp] = make(map[string]int64, len(counts))
		for code, count := range counts {
			result[comp][code] = count
		}
	}
	return result
}

// GetErrorRate returns the error rate over the specified duration
func (m *ErrorMetrics) GetErrorRate(duration time.Duration) float64 {
	return m.errorRates.GetRate(duration)
}

// GetTotalErrors returns the total error count
func (m *ErrorMetrics) GetTotalErrors() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalErrors
}

// Reset resets all metrics
func (m *ErrorMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.errorCounts = make(map[string]int64)
	m.errorsByOperation = make(map[string]map[string]int64)
	m.errorsByComponent = make(map[string]map[string]int64)
	m.totalErrors = 0
	m.errorRates.Reset()
}

// Enable enables metrics collection
func (m *ErrorMetrics) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable disables metrics collection
func (m *ErrorMetrics) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// IsEnabled returns whether metrics collection is enabled
func (m *ErrorMetrics) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Increment increments the counter for the current time window
func (c *TimeWindowCounter) Increment() {
	c.IncrementBy(1)
}

// IncrementBy increments the counter by the specified amount
func (c *TimeWindowCounter) IncrementBy(amount int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	c.cleanOldWindows(now)
	
	// Find or create current window
	currentWindow := now.Truncate(c.interval)
	
	if len(c.windows) == 0 || c.windows[len(c.windows)-1].timestamp.Before(currentWindow) {
		// Create new window
		if len(c.windows) >= c.size {
			// Remove oldest window
			copy(c.windows, c.windows[1:])
			c.windows = c.windows[:len(c.windows)-1]
		}
		c.windows = append(c.windows, timeWindow{
			timestamp: currentWindow,
			count:     amount,
		})
	} else {
		// Update existing window
		c.windows[len(c.windows)-1].count += amount
	}
}

// GetRate returns the rate over the specified duration
func (c *TimeWindowCounter) GetRate(duration time.Duration) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-duration)
	
	var total int64
	var windows int
	
	for _, window := range c.windows {
		if window.timestamp.After(cutoff) {
			total += window.count
			windows++
		}
	}
	
	if windows == 0 {
		return 0
	}
	
	// Return errors per second
	return float64(total) / duration.Seconds()
}

// Reset resets the counter
func (c *TimeWindowCounter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.windows = c.windows[:0]
}

func (c *TimeWindowCounter) cleanOldWindows(now time.Time) {
	cutoff := now.Add(-time.Duration(c.size) * c.interval)
	
	for i, window := range c.windows {
		if window.timestamp.After(cutoff) {
			if i > 0 {
				copy(c.windows, c.windows[i:])
				c.windows = c.windows[:len(c.windows)-i]
			}
			break
		}
	}
}

// ErrorSeverity represents error severity levels
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ErrorCategory represents error categories
type ErrorCategory string

const (
	CategoryStorage    ErrorCategory = "storage"
	CategoryValidation ErrorCategory = "validation"
	CategoryBusiness   ErrorCategory = "business"
	CategoryAuth       ErrorCategory = "auth"
	CategoryTransport  ErrorCategory = "transport"
	CategorySystem     ErrorCategory = "system"
)

// ErrorMetadata provides structured error metadata
type ErrorMetadata struct {
	Code      string        `json:"code"`
	Category  ErrorCategory `json:"category"`
	Severity  ErrorSeverity `json:"severity"`
	Source    string        `json:"source"`
	Operation string        `json:"operation"`
	Component string        `json:"component"`
}

// GetErrorMetadata extracts metadata from an error code
func GetErrorMetadata(errorCode, operation, component string) ErrorMetadata {
	metadata := ErrorMetadata{
		Code:      errorCode,
		Operation: operation,
		Component: component,
		Source:    "unknown",
	}
	
	// Determine category and severity based on error code prefix
	switch {
	case hasPrefix(errorCode, "STORAGE_"):
		metadata.Category = CategoryStorage
		metadata.Severity = SeverityHigh
		metadata.Source = "database"
		
	case hasPrefix(errorCode, "VALIDATION_"):
		metadata.Category = CategoryValidation
		metadata.Severity = SeverityLow
		metadata.Source = "input"
		
	case hasPrefix(errorCode, "ENTITY_", "RELATION_", "OBSERVATION_", "INVALID_OPERATION", "STATE_CONFLICT"):
		metadata.Category = CategoryBusiness
		metadata.Severity = SeverityMedium
		metadata.Source = "application"
		
	case hasPrefix(errorCode, "AUTH_"):
		metadata.Category = CategoryAuth
		metadata.Severity = SeverityMedium
		metadata.Source = "security"
		
	case hasPrefix(errorCode, "TRANSPORT_"):
		metadata.Category = CategoryTransport
		metadata.Severity = SeverityMedium
		metadata.Source = "network"
		
	case hasPrefix(errorCode, "INTERNAL", "NOT_IMPLEMENTED", "SERVICE_UNAVAILABLE", 
		"CONTEXT_", "PANIC", "CONFIGURATION", "RESOURCE_EXHAUSTED"):
		metadata.Category = CategorySystem
		metadata.Severity = SeverityCritical
		metadata.Source = "system"
		
	default:
		metadata.Category = CategorySystem
		metadata.Severity = SeverityMedium
		metadata.Source = "unknown"
	}
	
	return metadata
}

func hasPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// Global metrics instance with thread-safe initialization
var (
	globalErrorMetrics     *ErrorMetrics
	globalErrorMetricsOnce sync.Once
)

// getGlobalErrorMetrics returns the global metrics instance, initializing it if necessary
func getGlobalErrorMetrics() *ErrorMetrics {
	globalErrorMetricsOnce.Do(func() {
		globalErrorMetrics = NewErrorMetrics()
	})
	return globalErrorMetrics
}

// RecordError records an error using the global metrics
func RecordError(ctx context.Context, errorCode, operation, component string) {
	getGlobalErrorMetrics().RecordError(ctx, errorCode, operation, component)
}

// GetGlobalMetrics returns the global metrics instance
func GetGlobalMetrics() *ErrorMetrics {
	return getGlobalErrorMetrics()
}