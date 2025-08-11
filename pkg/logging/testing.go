package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestLogger captures logs for testing and verification
type TestLogger struct {
	mu      sync.RWMutex
	entries []TestLogEntry
	buffer  *bytes.Buffer
}

// TestLogEntry represents a captured log entry
type TestLogEntry struct {
	Time      time.Time              `json:"time"`
	Level     string                 `json:"level"`
	Message   string                 `json:"msg"`
	Component string                 `json:"component,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	Operation string                 `json:"operation,omitempty"`
	Duration  *time.Duration         `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Attrs     map[string]interface{} `json:"attrs,omitempty"`
}

// NewTestLogger creates a new test logger that captures log output
func NewTestLogger() *TestLogger {
	return &TestLogger{
		entries: make([]TestLogEntry, 0),
		buffer:  bytes.NewBuffer(nil),
	}
}

// GetHandler returns a slog.Handler that writes to this test logger
func (tl *TestLogger) GetHandler() slog.Handler {
	return slog.NewJSONHandler(tl.buffer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Capture all attributes for testing
			return a
		},
	})
}

// GetLogger returns a slog.Logger that writes to this test logger
func (tl *TestLogger) GetLogger() *slog.Logger {
	return slog.New(tl.GetHandler())
}

// GetEntries returns all captured log entries
func (tl *TestLogger) GetEntries() []TestLogEntry {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	
	// Parse buffer content if needed
	tl.parseBuffer()
	
	entries := make([]TestLogEntry, len(tl.entries))
	copy(entries, tl.entries)
	return entries
}

// GetEntriesWithLevel returns log entries matching the specified level
func (tl *TestLogger) GetEntriesWithLevel(level string) []TestLogEntry {
	entries := tl.GetEntries()
	var filtered []TestLogEntry
	
	for _, entry := range entries {
		if strings.EqualFold(entry.Level, level) {
			filtered = append(filtered, entry)
		}
	}
	
	return filtered
}

// GetEntriesWithMessage returns log entries containing the specified message
func (tl *TestLogger) GetEntriesWithMessage(message string) []TestLogEntry {
	entries := tl.GetEntries()
	var filtered []TestLogEntry
	
	for _, entry := range entries {
		if strings.Contains(entry.Message, message) {
			filtered = append(filtered, entry)
		}
	}
	
	return filtered
}

// GetEntriesWithComponent returns log entries matching the specified component
func (tl *TestLogger) GetEntriesWithComponent(component string) []TestLogEntry {
	entries := tl.GetEntries()
	var filtered []TestLogEntry
	
	for _, entry := range entries {
		if entry.Component == component {
			filtered = append(filtered, entry)
		}
	}
	
	return filtered
}

// parseBuffer parses the JSON log buffer and converts to entries
func (tl *TestLogger) parseBuffer() {
	if tl.buffer.Len() == 0 {
		return
	}
	
	content := tl.buffer.String()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		var rawEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rawEntry); err != nil {
			continue
		}
		
		entry := TestLogEntry{
			Attrs: make(map[string]interface{}),
		}
		
		// Parse standard fields
		if timeStr, ok := rawEntry["time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				entry.Time = t
			}
		}
		
		if level, ok := rawEntry["level"].(string); ok {
			entry.Level = level
		}
		
		if msg, ok := rawEntry["msg"].(string); ok {
			entry.Message = msg
		}
		
		if component, ok := rawEntry["component"].(string); ok {
			entry.Component = component
		}
		
		if requestID, ok := rawEntry["request_id"].(string); ok {
			entry.RequestID = requestID
		}
		
		if traceID, ok := rawEntry["trace_id"].(string); ok {
			entry.TraceID = traceID
		}
		
		if userID, ok := rawEntry["user_id"].(string); ok {
			entry.UserID = userID
		}
		
		if operation, ok := rawEntry["operation"].(string); ok {
			entry.Operation = operation
		}
		
		if errorStr, ok := rawEntry["error"].(string); ok {
			entry.Error = errorStr
		}
		
		if durationStr, ok := rawEntry["duration"].(string); ok {
			if d, err := time.ParseDuration(durationStr); err == nil {
				entry.Duration = &d
			}
		}
		
		// Capture all other attributes
		for key, value := range rawEntry {
			if !isStandardField(key) {
				entry.Attrs[key] = value
			}
		}
		
		tl.entries = append(tl.entries, entry)
	}
	
	// Clear buffer after parsing
	tl.buffer.Reset()
}

// isStandardField checks if a field is a standard log field
func isStandardField(field string) bool {
	standardFields := []string{
		"time", "level", "msg", "component", "request_id", "trace_id", 
		"user_id", "operation", "error", "duration",
	}
	
	for _, std := range standardFields {
		if field == std {
			return true
		}
	}
	return false
}

// Clear resets all captured entries and buffer
func (tl *TestLogger) Clear() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	
	tl.entries = tl.entries[:0]
	tl.buffer.Reset()
}

// Count returns the total number of captured entries
func (tl *TestLogger) Count() int {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	
	tl.parseBuffer()
	return len(tl.entries)
}

// Test Assertion Helpers

// AssertLogged verifies that a log entry with the specified level and message was captured
func (tl *TestLogger) AssertLogged(t *testing.T, level, message string) {
	t.Helper()
	
	entries := tl.GetEntries()
	for _, entry := range entries {
		if strings.EqualFold(entry.Level, level) && strings.Contains(entry.Message, message) {
			return
		}
	}
	
	t.Errorf("Expected log entry with level=%s message=%s not found. Captured entries:", level, message)
	for i, entry := range entries {
		t.Errorf("  [%d] %s: %s", i, entry.Level, entry.Message)
	}
}

// AssertNotLogged verifies that no log entry with the specified level and message was captured
func (tl *TestLogger) AssertNotLogged(t *testing.T, level, message string) {
	t.Helper()
	
	entries := tl.GetEntries()
	for _, entry := range entries {
		if strings.EqualFold(entry.Level, level) && strings.Contains(entry.Message, message) {
			t.Errorf("Unexpected log entry found with level=%s message=%s", level, message)
			return
		}
	}
}

// AssertLogCount verifies the expected number of log entries
func (tl *TestLogger) AssertLogCount(t *testing.T, expected int) {
	t.Helper()
	
	count := tl.Count()
	if count != expected {
		t.Errorf("Expected %d log entries, got %d", expected, count)
	}
}

// AssertLogCountWithLevel verifies the expected number of log entries for a specific level
func (tl *TestLogger) AssertLogCountWithLevel(t *testing.T, level string, expected int) {
	t.Helper()
	
	entries := tl.GetEntriesWithLevel(level)
	count := len(entries)
	if count != expected {
		t.Errorf("Expected %d log entries with level=%s, got %d", expected, level, count)
	}
}

// AssertRequestID verifies that all entries have the same request ID
func (tl *TestLogger) AssertRequestID(t *testing.T, expectedRequestID string) {
	t.Helper()
	
	entries := tl.GetEntries()
	for i, entry := range entries {
		if entry.RequestID == "" {
			t.Errorf("Entry [%d] missing request_id", i)
		} else if entry.RequestID != expectedRequestID {
			t.Errorf("Entry [%d] has request_id=%s, expected %s", i, entry.RequestID, expectedRequestID)
		}
	}
}

// AssertHasRequestID verifies that entries have request IDs
func (tl *TestLogger) AssertHasRequestID(t *testing.T) {
	t.Helper()
	
	entries := tl.GetEntries()
	for i, entry := range entries {
		if entry.RequestID == "" {
			t.Errorf("Entry [%d] missing request_id", i)
		}
	}
}

// AssertComponent verifies that entries have the expected component
func (tl *TestLogger) AssertComponent(t *testing.T, expectedComponent string) {
	t.Helper()
	
	entries := tl.GetEntries()
	for i, entry := range entries {
		if entry.Component != expectedComponent {
			t.Errorf("Entry [%d] has component=%s, expected %s", i, entry.Component, expectedComponent)
		}
	}
}

// MockAuditLogger provides a mock implementation for testing
type MockAuditLogger struct {
	mu     sync.RWMutex
	events []AuditEvent
}


// NewMockAuditLogger creates a new mock audit logger
func NewMockAuditLogger() *MockAuditLogger {
	return &MockAuditLogger{
		events: make([]AuditEvent, 0),
	}
}

// LogEvent simulates logging an audit event
func (mal *MockAuditLogger) LogEvent(eventType string, details map[string]interface{}) {
	mal.mu.Lock()
	defer mal.mu.Unlock()
	
	event := AuditEvent{
		EventType: eventType,
		Timestamp: time.Now(),
		Details:   details,
	}
	
	// Extract common fields from details
	if requestID, ok := details["request_id"].(string); ok {
		event.RequestID = requestID
	}
	
	if userID, ok := details["user_id"].(string); ok {
		event.UserID = userID
	}
	
	if resource, ok := details["resource"].(string); ok {
		event.Resource = resource
	}
	
	if action, ok := details["action"].(string); ok {
		event.Action = action
	}
	
	if result, ok := details["result"].(string); ok {
		event.Result = result
	}
	
	mal.events = append(mal.events, event)
}

// GetEvents returns all captured audit events
func (mal *MockAuditLogger) GetEvents() []AuditEvent {
	mal.mu.RLock()
	defer mal.mu.RUnlock()
	
	events := make([]AuditEvent, len(mal.events))
	copy(events, mal.events)
	return events
}

// Clear resets all captured events
func (mal *MockAuditLogger) Clear() {
	mal.mu.Lock()
	defer mal.mu.Unlock()
	
	mal.events = mal.events[:0]
}

// Close is a no-op for the mock
func (mal *MockAuditLogger) Close() error {
	return nil
}

// MockMetricsCollector provides a mock implementation for testing
type MockMetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]float64
	labels  map[string]map[string]string
}

// NewMockMetricsCollector creates a new mock metrics collector
func NewMockMetricsCollector() *MockMetricsCollector {
	return &MockMetricsCollector{
		metrics: make(map[string]float64),
		labels:  make(map[string]map[string]string),
	}
}

// RecordMetric records a metric value
func (mmc *MockMetricsCollector) RecordMetric(name string, value float64, labels map[string]string) {
	mmc.mu.Lock()
	defer mmc.mu.Unlock()
	
	mmc.metrics[name] = value
	if labels != nil {
		mmc.labels[name] = labels
	}
}

// GetMetric returns the recorded value for a metric
func (mmc *MockMetricsCollector) GetMetric(name string) (float64, bool) {
	mmc.mu.RLock()
	defer mmc.mu.RUnlock()
	
	value, exists := mmc.metrics[name]
	return value, exists
}

// GetMetricLabels returns the labels for a metric
func (mmc *MockMetricsCollector) GetMetricLabels(name string) (map[string]string, bool) {
	mmc.mu.RLock()
	defer mmc.mu.RUnlock()
	
	labels, exists := mmc.labels[name]
	return labels, exists
}

// Clear resets all metrics
func (mmc *MockMetricsCollector) Clear() {
	mmc.mu.Lock()
	defer mmc.mu.Unlock()
	
	mmc.metrics = make(map[string]float64)
	mmc.labels = make(map[string]map[string]string)
}

// Close is a no-op for the mock
func (mmc *MockMetricsCollector) Close() error {
	return nil
}

// Test Helper Functions

// CreateTestContext creates a context with test values for logging
func CreateTestContext() context.Context {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-request-123")
	ctx = WithTraceID(ctx, "test-trace-456")
	ctx = WithUserID(ctx, "test-user-789")
	ctx = WithOperation(ctx, "test-operation")
	ctx = WithComponent(ctx, "test-component")
	ctx = WithStartTime(ctx, time.Now())
	return ctx
}

// CreateTestContextWithValues creates a context with custom test values
func CreateTestContextWithValues(requestID, traceID, userID, operation, component string) context.Context {
	ctx := context.Background()
	if requestID != "" {
		ctx = WithRequestID(ctx, requestID)
	}
	if traceID != "" {
		ctx = WithTraceID(ctx, traceID)
	}
	if userID != "" {
		ctx = WithUserID(ctx, userID)
	}
	if operation != "" {
		ctx = WithOperation(ctx, operation)
	}
	if component != "" {
		ctx = WithComponent(ctx, component)
	}
	ctx = WithStartTime(ctx, time.Now())
	return ctx
}

// SimulateRequest simulates a request lifecycle with logging
func SimulateRequest(t *testing.T, logger *slog.Logger, operation string) context.Context {
	t.Helper()
	
	ctx := NewRequestContext(context.Background(), operation)
	
	// Start request
	logger.InfoContext(ctx, "Request started", slog.String("operation", operation))
	
	// Simulate processing
	time.Sleep(1 * time.Millisecond)
	
	// Complete request
	duration := GetDuration(ctx)
	logger.InfoContext(ctx, "Request completed", 
		slog.String("operation", operation),
		slog.Duration("duration", duration))
	
	return ctx
}

// CreateTestFactory creates a factory configured for testing
func CreateTestFactory(config *Config) (*Factory, *TestLogger, error) {
	if config == nil {
		config = &Config{
			Level:  LogLevelDebug,
			Format: LogFormatJSON,
			Output: LogOutputStdout,
		}
	}
	
	testLogger := NewTestLogger()
	
	// Create factory but replace the handler with our test handler
	factory, err := NewFactory(config)
	if err != nil {
		return nil, nil, err
	}
	
	// Replace the handler with test handler
	factory.handler = testLogger.GetHandler()
	
	return factory, testLogger, nil
}

// AssertValidTraceParent validates W3C Trace Context format
func AssertValidTraceParent(t *testing.T, traceParent string) {
	t.Helper()
	
	if len(traceParent) != 55 {
		t.Errorf("Invalid trace parent length: expected 55, got %d", len(traceParent))
		return
	}
	
	parts := strings.Split(traceParent, "-")
	if len(parts) != 4 {
		t.Errorf("Invalid trace parent format: expected 4 parts, got %d", len(parts))
		return
	}
	
	if parts[0] != "00" {
		t.Errorf("Invalid trace parent version: expected '00', got '%s'", parts[0])
	}
	
	if len(parts[1]) != 32 {
		t.Errorf("Invalid trace ID length: expected 32, got %d", len(parts[1]))
	}
	
	if len(parts[2]) != 16 {
		t.Errorf("Invalid span ID length: expected 16, got %d", len(parts[2]))
	}
	
	if len(parts[3]) != 2 {
		t.Errorf("Invalid trace flags length: expected 2, got %d", len(parts[3]))
	}
}

// Test Configuration Helpers

// TestConfigs provides various configurations for testing
var TestConfigs = struct {
	Development *Config
	Production  *Config
	Minimal     *Config
	FullFeature *Config
}{
	Development: &Config{
		Level:  LogLevelDebug,
		Format: LogFormatText,
		Output: LogOutputStdout,
		EnableRequestID:  true,
		EnableTracing:    true,
		EnableStackTrace: true,
		EnableCaller:     true,
		PrettyPrint:      true,
		Masking: MaskingConfig{
			Enabled: false,
		},
		Sampling: SamplingConfig{
			Enabled: false,
		},
	},
	Production: &Config{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		AsyncLogging:     true,
		EnableRequestID:  true,
		EnableTracing:    true,
		EnableAudit:      true,
		AuditFilePath:    "test_audit.log",
		EnableStackTrace: false,
		EnableCaller:     false,
		PrettyPrint:      false,
		Sampling: SamplingConfig{
			Enabled:      true,
			Rate:         0.1,
			BurstSize:    100,
			AlwaysErrors: true,
		},
		Masking: MaskingConfig{
			Enabled:          true,
			MaskEmails:       true,
			MaskPhoneNumbers: true,
			MaskCreditCards:  true,
			MaskSSN:          true,
			MaskAPIKeys:      true,
		},
		Metrics: MetricsConfig{
			Enabled: true,
		},
	},
	Minimal: &Config{
		Level:  LogLevelError,
		Format: LogFormatText,
		Output: LogOutputStderr,
		Masking: MaskingConfig{
			Enabled: false,
		},
		Sampling: SamplingConfig{
			Enabled: false,
		},
	},
	FullFeature: &Config{
		Level:            LogLevelDebug,
		Format:           LogFormatJSON,
		Output:           LogOutputStdout,
		AsyncLogging:     true,
		BufferSize:       2048,
		EnableRequestID:  true,
		EnableTracing:    true,
		EnableAudit:      true,
		AuditFilePath:    "test_full_audit.log",
		EnableStackTrace: true,
		EnableCaller:     true,
		ComponentLevels: map[string]LogLevel{
			"test-component": LogLevelWarn,
			"storage":        LogLevelError,
		},
		Sampling: SamplingConfig{
			Enabled:      true,
			Rate:         0.5,
			BurstSize:    50,
			AlwaysErrors: true,
		},
		Masking: MaskingConfig{
			Enabled:          true,
			Fields:           []string{"password", "secret"},
			Patterns:         []string{`\b\d{4}-\d{4}-\d{4}-\d{4}\b`},
			MaskEmails:       true,
			MaskPhoneNumbers: true,
			MaskCreditCards:  true,
			MaskSSN:          true,
			MaskAPIKeys:      true,
		},
		Metrics: MetricsConfig{
			Enabled:    true,
			Namespace:  "test",
			Subsystem:  "logging",
		},
	},
}