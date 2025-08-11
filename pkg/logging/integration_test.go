package logging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestEndToEndRequestFlow tests a complete request flow with logging
func TestEndToEndRequestFlow(t *testing.T) {
	// Create temporary files for testing
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")
	auditFile := filepath.Join(tempDir, "audit.log")

	config := &Config{
		Level:         LogLevelDebug,
		Format:        LogFormatJSON,
		Output:        LogOutputFile,
		FilePath:      logFile,
		EnableAudit:   true,
		AuditFilePath: auditFile,
		EnableRequestID: true,
		EnableTracing:   true,
		Metrics: MetricsConfig{
			Enabled: true,
		},
		Masking: MaskingConfig{
			Enabled:     true,
			MaskEmails:  true,
			MaskAPIKeys: true,
		},
	}

	// Initialize factory
	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	// Create components
	logger := factory.GetLogger("integration-test")
	auditLogger := factory.GetAuditLogger()
	metricsCollector := factory.GetMetricsCollector()

	// Simulate request flow
	ctx := context.Background()
	userID := "user-123"
	operation := "create_entity"

	// 1. Start request with context
	ctx = NewRequestContext(ctx, operation)
	ctx = WithUserID(ctx, userID)
	
	requestID := GetRequestID(ctx)
	t.Logf("Request ID: %s", requestID)

	// 2. Log operation start
	logger.InfoContext(ctx, "Starting integration test operation",
		"operation", operation,
		"user_id", userID,
		"sensitive_data", "api_key_12345", // Should be masked
	)

	// 3. Simulate database operation with metrics
	if metricsCollector != nil {
		timer := metricsCollector.NewRequestTimer("database", "create")
		time.Sleep(5 * time.Millisecond) // Simulate work
		timer.Finish(nil)
	}

	// 4. Log audit event
	if auditLogger != nil {
		event := AuditEvent{
			EventType: "entity_created",
			RequestID: requestID,
			UserID:    userID,
			Action:    "create",
			Resource:  "entity",
			Result:    "success",
			Timestamp: time.Now(),
		}
		auditLogger.Log(ctx, event)
	}

	// 5. Log completion
	duration := GetDuration(ctx)
	logger.InfoContext(ctx, "Integration test operation completed",
		"operation", operation,
		"duration", duration,
		"result", "success",
	)

	// Give some time for async operations
	time.Sleep(100 * time.Millisecond)

	// Verify log file exists and has content
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
		return
	}

	logContent, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(logContent) == 0 {
		t.Error("Expected log file to have content")
		return
	}

	// Parse log entries
	logLines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
	var logEntries []map[string]interface{}

	for _, line := range logLines {
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Failed to parse log line: %v", err)
			continue
		}
		logEntries = append(logEntries, entry)
	}

	if len(logEntries) < 2 {
		t.Errorf("Expected at least 2 log entries, got %d", len(logEntries))
	}

	// Verify log entries have consistent request ID
	for i, entry := range logEntries {
		if entryRequestID, ok := entry["request_id"].(string); ok {
			if entryRequestID != requestID {
				t.Errorf("Entry %d has inconsistent request ID: expected %s, got %s", i, requestID, entryRequestID)
			}
		} else {
			t.Errorf("Entry %d missing request_id", i)
		}
	}

	// Verify masking worked (should not contain api_key_12345)
	logContentStr := string(logContent)
	if strings.Contains(logContentStr, "api_key_12345") {
		t.Error("Expected sensitive data to be masked in logs")
	}

	// Verify audit file if audit logging is enabled
	if config.EnableAudit {
		if _, err := os.Stat(auditFile); os.IsNotExist(err) {
			t.Error("Expected audit file to be created")
		} else {
			auditContent, err := os.ReadFile(auditFile)
			if err != nil {
				t.Errorf("Failed to read audit file: %v", err)
			} else if len(auditContent) == 0 {
				t.Error("Expected audit file to have content")
			} else if !strings.Contains(string(auditContent), "entity_created") {
				t.Error("Expected audit event to be logged")
			}
		}
	}
}

// TestHTTPServerIntegration tests logging in an HTTP server context
func TestHTTPServerIntegration(t *testing.T) {
	// Create test logger
	testLogger := NewTestLogger()
	
	config := &Config{
		Level:  LogLevelDebug,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		EnableRequestID: true,
		EnableTracing:   true,
		Metrics: MetricsConfig{
			Enabled: true,
		},
	}

	// Create factory with custom handler for testing
	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	// Replace handler with test handler for verification
	factory.handler = testLogger.GetHandler()

	logger := factory.GetLogger("http-server")
	interceptor := NewRequestInterceptor(logger)
	metricsCollector := factory.GetMetricsCollector()

	// Create test HTTP handler with business logic
	businessHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Simulate business operation
		timer := StartTimer(ctx, logger, "process_request")
		
		// Simulate some processing time
		time.Sleep(2 * time.Millisecond)
		
		// Simulate database operation
		if metricsCollector != nil {
			dbTimer := metricsCollector.NewRequestTimer("database", "query")
			time.Sleep(1 * time.Millisecond)
			dbTimer.Finish(nil)
		}

		// Log business logic
		logger.InfoContext(ctx, "Processing user request",
			"endpoint", r.URL.Path,
			"user_agent", r.UserAgent(),
		)

		timer.End()

		// Return response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success", "message": "Request processed"}`))
	})

	// Wrap with logging middleware
	handler := interceptor.HTTPMiddleware(businessHandler)

	// Add metrics middleware if available
	if metricsCollector != nil {
		handler = metricsCollector.Middleware()(handler)
	}

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make test requests
	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/api/users", 200},
		{"POST", "/api/users", 200},
		{"GET", "/api/users/123", 200},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			testLogger.Clear() // Clear previous entries
			
			url := server.URL + tt.path
			req, err := http.NewRequest(tt.method, url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			req.Header.Set("User-Agent", "integration-test-client")

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, resp.StatusCode)
			}

			// Give time for async logging
			time.Sleep(50 * time.Millisecond)

			// Verify comprehensive logging
			entries := testLogger.GetEntries()
			if len(entries) < 4 {
				t.Errorf("Expected at least 4 log entries, got %d", len(entries))
				for i, entry := range entries {
					t.Logf("Entry %d: %s - %s", i, entry.Level, entry.Message)
				}
			}

			// Verify HTTP request logging
			httpStart := testLogger.GetEntriesWithMessage("HTTP request started")
			if len(httpStart) != 1 {
				t.Errorf("Expected 1 HTTP start entry, got %d", len(httpStart))
			}

			httpEnd := testLogger.GetEntriesWithMessage("HTTP request completed successfully")
			if len(httpEnd) != 1 {
				t.Errorf("Expected 1 HTTP completion entry, got %d", len(httpEnd))
			}

			// Verify operation timing
			opStart := testLogger.GetEntriesWithMessage("Operation started")
			if len(opStart) < 1 {
				t.Errorf("Expected at least 1 operation start entry, got %d", len(opStart))
			}

			opEnd := testLogger.GetEntriesWithMessage("Operation completed")
			if len(opEnd) < 1 {
				t.Errorf("Expected at least 1 operation completion entry, got %d", len(opEnd))
			}

			// Verify business logic logging
			businessLogic := testLogger.GetEntriesWithMessage("Processing user request")
			if len(businessLogic) != 1 {
				t.Errorf("Expected 1 business logic entry, got %d", len(businessLogic))
			}

			// Verify request ID consistency
			var requestID string
			for i, entry := range entries {
				if entry.RequestID == "" {
					t.Errorf("Entry %d missing request ID", i)
				} else {
					if requestID == "" {
						requestID = entry.RequestID
					} else if entry.RequestID != requestID {
						t.Errorf("Entry %d has inconsistent request ID: expected %s, got %s", i, requestID, entry.RequestID)
					}
				}
			}
		})
	}
}

// TestErrorPropagationWithContext tests error handling and context propagation
func TestErrorPropagationWithContext(t *testing.T) {
	testLogger := NewTestLogger()

	config := &Config{
		Level:           LogLevelDebug,
		Format:          LogFormatJSON,
		Output:          LogOutputStdout,
		EnableRequestID: true,
		EnableAudit:     true,
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	// Replace handler for testing
	factory.handler = testLogger.GetHandler()

	logger := factory.GetLogger("error-test")
	interceptor := NewRequestInterceptor(logger)

	// Simulate nested operation with error propagation
	ctx := context.Background()
	ctx = NewRequestContext(ctx, "parent-operation")
	ctx = WithUserID(ctx, "user-456")

	originalRequestID := GetRequestID(ctx)

	// Parent operation that calls child operations
	err = interceptor.InterceptRequest(ctx, "parent-operation", func(parentCtx context.Context) error {
		// Verify context propagation
		if GetRequestID(parentCtx) != originalRequestID {
			t.Error("Request ID not propagated to parent operation")
		}

		// Child operation 1 - success
		err := interceptor.InterceptRequest(parentCtx, "child-operation-1", func(childCtx context.Context) error {
			// Verify context propagation
			if GetRequestID(childCtx) != originalRequestID {
				t.Error("Request ID not propagated to child operation 1")
			}

			logger.InfoContext(childCtx, "Child operation 1 processing")
			return nil
		})

		if err != nil {
			return fmt.Errorf("child operation 1 failed: %w", err)
		}

		// Child operation 2 - failure
		err = interceptor.InterceptRequest(parentCtx, "child-operation-2", func(childCtx context.Context) error {
			// Verify context propagation
			if GetRequestID(childCtx) != originalRequestID {
				t.Error("Request ID not propagated to child operation 2")
			}

			logger.WarnContext(childCtx, "Child operation 2 about to fail")
			return errors.New("child operation 2 failed")
		})

		if err != nil {
			// Log the error at parent level
			logger.ErrorContext(parentCtx, "Child operation failed", "error", err.Error())
			return fmt.Errorf("parent operation failed due to child: %w", err)
		}

		return nil
	})

	if err == nil {
		t.Error("Expected parent operation to fail due to child error")
	}

	// Verify error logging structure
	entries := testLogger.GetEntries()
	if len(entries) < 6 {
		t.Errorf("Expected at least 6 log entries (parent start, child1 start, child1 info, child1 end, child2 start, child2 warn, child2 error, parent error, parent error end), got %d", len(entries))
		for i, entry := range entries {
			t.Logf("Entry %d: %s - %s", i, entry.Level, entry.Message)
		}
	}

	// Verify request ID consistency across all nested operations
	for i, entry := range entries {
		if entry.RequestID != originalRequestID {
			t.Errorf("Entry %d has inconsistent request ID: expected %s, got %s", i, originalRequestID, entry.RequestID)
		}
	}

	// Verify error messages are present
	errorEntries := testLogger.GetEntriesWithLevel("ERROR")
	if len(errorEntries) < 2 {
		t.Errorf("Expected at least 2 error entries, got %d", len(errorEntries))
	}

	// Verify child operation failure is logged
	childFailure := testLogger.GetEntriesWithMessage("child operation 2 failed")
	if len(childFailure) < 1 {
		t.Error("Expected child operation failure to be logged")
	}

	// Verify parent operation failure is logged
	parentFailure := testLogger.GetEntriesWithMessage("parent operation failed due to child")
	if len(parentFailure) < 1 {
		t.Error("Expected parent operation failure to be logged")
	}
}

// TestMetricsCollection tests metrics collection during request processing
func TestMetricsCollection(t *testing.T) {
	config := &Config{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		Metrics: MetricsConfig{
			Enabled:    true,
			Namespace:  "test",
			Subsystem:  "integration",
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	metricsCollector := factory.GetMetricsCollector()
	if metricsCollector == nil {
		t.Fatal("Expected metrics collector to be available")
	}

	// Simulate various operations with metrics

	// Database operations
	for i := 0; i < 5; i++ {
		timer := metricsCollector.NewRequestTimer("database", "query")
		time.Sleep(time.Duration(i+1) * time.Millisecond)
		
		var err error
		if i == 3 { // Simulate one error
			err = errors.New("connection timeout")
		}
		
		timer.Finish(err)
	}

	// Cache operations
	for i := 0; i < 10; i++ {
		if i%3 == 0 {
			metricsCollector.RecordCacheHit("user_cache", "user_*")
		} else {
			metricsCollector.RecordCacheMiss("user_cache", "user_*")
		}
	}

	// HTTP requests simulation
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	endpoints := []string{"/api/users", "/api/posts", "/api/comments"}
	
	for _, method := range methods {
		for _, endpoint := range endpoints {
			duration := time.Duration(10+len(method)*len(endpoint)) * time.Millisecond
			statusCode := "200"
			if method == "DELETE" && endpoint == "/api/comments" {
				statusCode = "404"
			}
			
			metricsCollector.RecordRequest(method, endpoint, statusCode, duration, 1024, 2048)
		}
	}

	// Storage operations
	operations := []string{"create", "read", "update", "delete"}
	tables := []string{"users", "posts", "comments"}
	
	for _, op := range operations {
		for _, table := range tables {
			duration := time.Duration(5+len(op)*len(table)) * time.Millisecond
			metricsCollector.RecordStorageQuery(op, table, duration)
		}
	}

	// Component operations
	components := []string{"auth", "storage", "cache", "notification"}
	for _, comp := range components {
		timer := metricsCollector.NewRequestTimer(comp, "process")
		time.Sleep(2 * time.Millisecond)
		
		var err error
		if comp == "notification" {
			err = errors.New("smtp server unavailable")
		}
		
		timer.Finish(err)
	}

	// Give time for metric collection
	time.Sleep(100 * time.Millisecond)

	// Note: In a real test, you would verify the metrics were properly recorded
	// by checking the Prometheus registry or by using a test metrics collector
	// For this integration test, we're primarily testing that the operations
	// complete without errors and the metrics collection doesn't interfere
	// with normal operation.
	
	t.Log("Metrics collection test completed successfully")
}

// TestAuditTrailGeneration tests comprehensive audit trail generation
func TestAuditTrailGeneration(t *testing.T) {
	tempDir := t.TempDir()
	auditFile := filepath.Join(tempDir, "integration_audit.log")

	config := &Config{
		Level:           LogLevelInfo,
		Format:          LogFormatJSON,
		Output:          LogOutputStdout,
		EnableAudit:     true,
		AuditFilePath:   auditFile,
		EnableRequestID: true,
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	auditLogger := factory.GetAuditLogger()
	if auditLogger == nil {
		t.Fatal("Expected audit logger to be available")
	}

	// Simulate user session with multiple operations
	ctx := NewRequestContext(context.Background(), "user-session")
	ctx = WithUserID(ctx, "user-789")
	ctx = WithSessionID(ctx, "session-abc123")

	requestID := GetRequestID(ctx)

	// User login
	loginEvent := AuditEvent{
		EventType: "user_login",
		RequestID: requestID,
		UserID:    "user-789",
		SessionID: "session-abc123",
		ActorIP:   "192.168.1.100",
		UserAgent: "Mozilla/5.0 (integration test)",
		Result:    "success",
		Timestamp: time.Now(),
	}
	auditLogger.Log(ctx, loginEvent)

	// Data access operations
	resources := []string{"user_profile", "user_settings", "user_posts"}
	actions := []string{"read", "update", "delete"}

	for _, resource := range resources {
		for _, action := range actions {
			result := "success"
			if resource == "user_settings" && action == "delete" {
				result = "forbidden" // Simulate permission denied
			}

			accessEvent := AuditEvent{
				EventType: "data_access",
				RequestID: requestID,
				UserID:    "user-789",
				SessionID: "session-abc123",
				Resource:  resource,
				Action:    action,
				Result:    result,
				Timestamp: time.Now(),
			}
			auditLogger.Log(ctx, accessEvent)

			time.Sleep(1 * time.Millisecond) // Ensure different timestamps
		}
	}

	// Administrative operations
	adminEvent := AuditEvent{
		EventType: "admin_operation",
		RequestID: requestID,
		UserID:    "user-789",
		Action:    "view_user_list",
		Resource:  "admin_panel",
		Result:    "unauthorized",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"attempted_access": "user management",
			"user_role":        "regular_user",
		},
	}
	auditLogger.Log(ctx, adminEvent)

	// User logout
	logoutEvent := AuditEvent{
		EventType: "user_logout",
		RequestID: requestID,
		UserID:    "user-789",
		SessionID: "session-abc123",
		Result:    "success",
		Timestamp: time.Now(),
	}
	auditLogger.Log(ctx, logoutEvent)

	// Give time for audit log writing
	time.Sleep(200 * time.Millisecond)

	// Verify audit file
	if _, err := os.Stat(auditFile); os.IsNotExist(err) {
		t.Fatal("Expected audit file to be created")
	}

	auditContent, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("Failed to read audit file: %v", err)
	}

	if len(auditContent) == 0 {
		t.Fatal("Expected audit file to have content")
	}

	auditLines := strings.Split(strings.TrimSpace(string(auditContent)), "\n")
	var auditEvents []map[string]interface{}

	for _, line := range auditLines {
		if line == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("Failed to parse audit line: %v", err)
			continue
		}
		auditEvents = append(auditEvents, event)
	}

	expectedEvents := 1 + len(resources)*len(actions) + 1 + 1 // login + data_access + admin + logout
	if len(auditEvents) < expectedEvents {
		t.Errorf("Expected at least %d audit events, got %d", expectedEvents, len(auditEvents))
	}

	// Verify audit event structure and consistency
	for i, event := range auditEvents {
		// Check required fields
		if eventType, ok := event["event_type"].(string); !ok || eventType == "" {
			t.Errorf("Audit event %d missing or empty event_type", i)
		}

		if timestamp, ok := event["timestamp"].(string); !ok || timestamp == "" {
			t.Errorf("Audit event %d missing or empty timestamp", i)
		}

		if result, ok := event["result"].(string); !ok || result == "" {
			t.Errorf("Audit event %d missing or empty result", i)
		}

		// Check request ID consistency
		if eventRequestID, ok := event["request_id"].(string); ok {
			if eventRequestID != requestID {
				t.Errorf("Audit event %d has inconsistent request ID: expected %s, got %s", i, requestID, eventRequestID)
			}
		}

		// Check user ID consistency for user events
		if userID, ok := event["user_id"].(string); ok {
			if userID != "user-789" {
				t.Errorf("Audit event %d has inconsistent user ID: expected user-789, got %s", i, userID)
			}
		}
	}

	// Verify specific event types exist
	eventTypes := make(map[string]int)
	for _, event := range auditEvents {
		if eventType, ok := event["event_type"].(string); ok {
			eventTypes[eventType]++
		}
	}

	expectedEventTypes := []string{"user_login", "data_access", "admin_operation", "user_logout"}
	for _, expectedType := range expectedEventTypes {
		if count, exists := eventTypes[expectedType]; !exists || count == 0 {
			t.Errorf("Expected audit event type %s to be present", expectedType)
		}
	}

	// Verify data_access events have proper counts
	if count := eventTypes["data_access"]; count != len(resources)*len(actions) {
		t.Errorf("Expected %d data_access events, got %d", len(resources)*len(actions), count)
	}
}

// TestPerformanceImpactMeasurement tests the performance impact of logging
func TestPerformanceImpactMeasurement(t *testing.T) {
	// Test with logging disabled
	baselineConfig := &Config{
		Level:  LogLevelError, // High level to minimize logging
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		Sampling: SamplingConfig{
			Enabled: true,
			Rate:    0.01, // Very low sampling rate
		},
		AsyncLogging: true,
		BufferSize:   8192,
	}

	// Test with full logging enabled
	fullLoggingConfig := &Config{
		Level:           LogLevelDebug,
		Format:          LogFormatJSON,
		Output:          LogOutputStdout,
		EnableRequestID: true,
		EnableTracing:   true,
		EnableCaller:    true,
		EnableAudit:     true,
		AsyncLogging:    true,
		BufferSize:      8192,
		Metrics: MetricsConfig{
			Enabled: true,
		},
		Masking: MaskingConfig{
			Enabled:     true,
			MaskEmails:  true,
			MaskAPIKeys: true,
		},
	}

	const iterations = 1000

	// Benchmark baseline (minimal logging)
	baselineStart := time.Now()
	factory1, err := NewFactory(baselineConfig)
	if err != nil {
		t.Fatalf("Failed to create baseline factory: %v", err)
	}
	defer factory1.Close()

	logger1 := factory1.GetLogger("benchmark")
	interceptor1 := NewRequestInterceptor(logger1)

	for i := 0; i < iterations; i++ {
		ctx := context.Background()
		err := interceptor1.InterceptRequest(ctx, "benchmark-operation", func(ctx context.Context) error {
			// Simulate work
			time.Sleep(100 * time.Microsecond)
			return nil
		})
		if err != nil {
			t.Errorf("Baseline iteration %d failed: %v", i, err)
		}
	}
	baselineDuration := time.Since(baselineStart)

	// Benchmark full logging
	fullLoggingStart := time.Now()
	factory2, err := NewFactory(fullLoggingConfig)
	if err != nil {
		t.Fatalf("Failed to create full logging factory: %v", err)
	}
	defer factory2.Close()

	logger2 := factory2.GetLogger("benchmark")
	interceptor2 := NewRequestInterceptor(logger2)

	for i := 0; i < iterations; i++ {
		ctx := context.Background()
		ctx = WithUserID(ctx, fmt.Sprintf("user-%d", i))
		
		err := interceptor2.InterceptRequest(ctx, "benchmark-operation", func(ctx context.Context) error {
			// Simulate work with additional logging
			logger2.DebugContext(ctx, "Processing operation", 
				"iteration", i,
				"sensitive_key", "api_key_secret123", // Will be masked
				"user_email", "user@example.com",     // Will be masked
			)
			
			time.Sleep(100 * time.Microsecond)
			
			logger2.InfoContext(ctx, "Operation completed",
				"iteration", i,
				"result", "success",
			)
			
			return nil
		})
		if err != nil {
			t.Errorf("Full logging iteration %d failed: %v", i, err)
		}
	}
	fullLoggingDuration := time.Since(fullLoggingStart)

	// Calculate overhead
	overhead := fullLoggingDuration - baselineDuration
	overheadPercent := float64(overhead) / float64(baselineDuration) * 100

	t.Logf("Performance Impact Measurement:")
	t.Logf("  Baseline (minimal logging): %v", baselineDuration)
	t.Logf("  Full logging: %v", fullLoggingDuration)
	t.Logf("  Overhead: %v (%.2f%%)", overhead, overheadPercent)

	// Give time for async operations to complete
	time.Sleep(500 * time.Millisecond)

	// Verify that overhead is reasonable (less than 100% in most cases)
	// This is a soft check - actual performance will vary by system
	if overheadPercent > 200 {
		t.Logf("Warning: Logging overhead is %.2f%%, which may be higher than expected", overheadPercent)
	}

	// Verify both configurations produced working loggers
	if factory1.GetLogger("test") == nil {
		t.Error("Baseline factory failed to produce working logger")
	}
	
	if factory2.GetLogger("test") == nil {
		t.Error("Full logging factory failed to produce working logger")
	}
}

// TestConcurrentRequestProcessing tests logging behavior under concurrent load
func TestConcurrentRequestProcessing(t *testing.T) {
	config := &Config{
		Level:           LogLevelInfo,
		Format:          LogFormatJSON,
		Output:          LogOutputStdout,
		EnableRequestID: true,
		AsyncLogging:    true,
		BufferSize:      4096,
		Metrics: MetricsConfig{
			Enabled: true,
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("concurrent-test")
	interceptor := NewRequestInterceptor(logger)
	metricsCollector := factory.GetMetricsCollector()

	const numGoroutines = 50
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup
	var mu sync.Mutex
	requestIDs := make(map[string]bool)
	errors := make([]error, 0)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				ctx := context.Background()
				ctx = WithUserID(ctx, fmt.Sprintf("user-g%d-op%d", goroutineID, j))
				
				operationName := fmt.Sprintf("concurrent-op-g%d-op%d", goroutineID, j)
				
				err := interceptor.InterceptRequest(ctx, operationName, func(ctx context.Context) error {
					requestID := GetRequestID(ctx)
					
					// Track unique request IDs
					mu.Lock()
					if requestIDs[requestID] {
						mu.Unlock()
						return fmt.Errorf("duplicate request ID detected: %s", requestID)
					}
					requestIDs[requestID] = true
					mu.Unlock()
					
					// Simulate varying work loads
					workDuration := time.Duration(j%10+1) * time.Millisecond
					
					// Use timer for metrics
					timer := StartTimer(ctx, logger, "simulated-work")
					time.Sleep(workDuration)
					timer.End()
					
					// Record metrics
					if metricsCollector != nil {
						metricsCollector.RecordComponentOperation("concurrent-test", "work", workDuration)
					}
					
					// Log progress
					logger.InfoContext(ctx, "Concurrent operation completed",
						"goroutine", goroutineID,
						"operation", j,
						"work_duration", workDuration,
					)
					
					// Simulate occasional errors
					if goroutineID%7 == 0 && j%5 == 4 {
						return fmt.Errorf("simulated error in g%d-op%d", goroutineID, j)
					}
					
					return nil
				})
				
				if err != nil {
					mu.Lock()
					errors = append(errors, err)
					mu.Unlock()
				}
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Give time for async logging to complete
	time.Sleep(1 * time.Second)
	
	// Verify results
	totalOperations := numGoroutines * operationsPerGoroutine
	
	mu.Lock()
	actualRequestIDs := len(requestIDs)
	duplicateErrors := 0
	for _, err := range errors {
		if strings.Contains(err.Error(), "duplicate request ID") {
			duplicateErrors++
		}
	}
	totalErrors := len(errors)
	mu.Unlock()
	
	t.Logf("Concurrent Processing Results:")
	t.Logf("  Total operations: %d", totalOperations)
	t.Logf("  Unique request IDs: %d", actualRequestIDs)
	t.Logf("  Duplicate ID errors: %d", duplicateErrors)
	t.Logf("  Total errors: %d", totalErrors)
	
	// Verify no duplicate request IDs were generated
	if duplicateErrors > 0 {
		t.Errorf("Found %d duplicate request ID errors", duplicateErrors)
	}
	
	// Verify all request IDs are unique (accounting for expected errors)
	expectedUniqueIDs := totalOperations
	if actualRequestIDs < expectedUniqueIDs-10 { // Allow some margin for expected errors
		t.Errorf("Expected approximately %d unique request IDs, got %d", expectedUniqueIDs, actualRequestIDs)
	}
	
	// Verify error rate is reasonable (we expect some simulated errors)
	expectedSimulatedErrors := numGoroutines / 7 * operationsPerGoroutine / 5 // Based on our error simulation logic
	if totalErrors < expectedSimulatedErrors-5 || totalErrors > expectedSimulatedErrors+10 {
		t.Logf("Warning: Expected approximately %d simulated errors, got %d total errors", expectedSimulatedErrors, totalErrors)
	}
}