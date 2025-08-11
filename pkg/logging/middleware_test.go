package logging

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRequestInterceptor(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()

	interceptor := NewRequestInterceptor(logger)

	if interceptor == nil {
		t.Error("Expected interceptor to be created")
	}

	if interceptor.logger != logger {
		t.Error("Expected logger to be set")
	}
}

func TestRequestInterceptor_InterceptRequest_Success(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	ctx := context.Background()
	operation := "test-operation"
	called := false

	fn := func(ctx context.Context) error {
		called = true
		
		// Verify request context is available
		requestID := GetRequestID(ctx)
		if requestID == "" {
			t.Error("Expected request ID to be set in context")
		}
		
		operation := GetOperation(ctx)
		if operation != "test-operation" {
			t.Errorf("Expected operation test-operation, got %s", operation)
		}
		
		return nil
	}

	err := interceptor.InterceptRequest(ctx, operation, fn)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !called {
		t.Error("Expected function to be called")
	}

	// Verify logging
	entries := testLogger.GetEntries()
	if len(entries) < 2 {
		t.Errorf("Expected at least 2 log entries, got %d", len(entries))
	}

	// Check start message
	startEntry := testLogger.GetEntriesWithMessage("Request started")
	if len(startEntry) != 1 {
		t.Errorf("Expected 1 start entry, got %d", len(startEntry))
	}

	// Check completion message
	completionEntry := testLogger.GetEntriesWithMessage("Request completed successfully")
	if len(completionEntry) != 1 {
		t.Errorf("Expected 1 completion entry, got %d", len(completionEntry))
	}

	// Verify request ID consistency
	if len(entries) >= 2 {
		if entries[0].RequestID != entries[1].RequestID {
			t.Error("Expected consistent request ID across log entries")
		}
	}
}

func TestRequestInterceptor_InterceptRequest_Error(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	ctx := context.Background()
	operation := "test-operation"
	expectedErr := errors.New("test error")

	fn := func(ctx context.Context) error {
		return expectedErr
	}

	err := interceptor.InterceptRequest(ctx, operation, fn)

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// Verify error logging
	errorEntry := testLogger.GetEntriesWithMessage("Request completed with error")
	if len(errorEntry) != 1 {
		t.Errorf("Expected 1 error entry, got %d", len(errorEntry))
	}

	if len(errorEntry) > 0 && errorEntry[0].Error != "test error" {
		t.Errorf("Expected error message 'test error', got %s", errorEntry[0].Error)
	}
}

func TestRequestInterceptor_InterceptRequest_Panic(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	ctx := context.Background()
	operation := "test-operation"
	panicMsg := "test panic"

	fn := func(ctx context.Context) error {
		panic(panicMsg)
	}

	// Expect panic to be re-thrown
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Error("Expected panic to be re-thrown")
		}
		
		if recovered != panicMsg {
			t.Errorf("Expected panic message %s, got %v", panicMsg, recovered)
		}

		// Verify panic logging
		panicEntry := testLogger.GetEntriesWithMessage("Request panicked")
		if len(panicEntry) != 1 {
			t.Errorf("Expected 1 panic entry, got %d", len(panicEntry))
		}

		// Check that stack trace is logged
		if len(panicEntry) > 0 {
			attrs := panicEntry[0].Attrs
			if stackTrace, ok := attrs["stack_trace"].(string); ok {
				if stackTrace == "" {
					t.Error("Expected stack trace to be logged")
				}
			} else {
				t.Error("Expected stack_trace attribute to be present")
			}
		}
	}()

	interceptor.InterceptRequest(ctx, operation, fn)
}

func TestRequestInterceptor_InterceptWithResponse_Success(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	ctx := context.Background()
	operation := "test-operation"
	expectedResponse := "test response"

	fn := func(ctx context.Context) (interface{}, error) {
		return expectedResponse, nil
	}

	response, err := interceptor.InterceptWithResponse(ctx, operation, fn)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if response != expectedResponse {
		t.Errorf("Expected response %s, got %v", expectedResponse, response)
	}

	// Verify logging
	completionEntry := testLogger.GetEntriesWithMessage("Request completed successfully")
	if len(completionEntry) != 1 {
		t.Errorf("Expected 1 completion entry, got %d", len(completionEntry))
	}
}

func TestRequestInterceptor_InterceptWithResponse_Error(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	ctx := context.Background()
	operation := "test-operation"
	expectedErr := errors.New("test error")

	fn := func(ctx context.Context) (interface{}, error) {
		return nil, expectedErr
	}

	response, err := interceptor.InterceptWithResponse(ctx, operation, fn)

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if response != nil {
		t.Errorf("Expected nil response, got %v", response)
	}

	// Verify error logging
	errorEntry := testLogger.GetEntriesWithMessage("Request completed with error")
	if len(errorEntry) != 1 {
		t.Errorf("Expected 1 error entry, got %d", len(errorEntry))
	}
}

func TestRequestInterceptor_HTTPMiddleware_Success(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request context has been set
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("Expected request ID to be set in request context")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap with middleware
	middleware := interceptor.HTTPMiddleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("User-Agent", "test-agent")

	rr := httptest.NewRecorder()

	// Execute request
	middleware.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "success" {
		t.Errorf("Expected body 'success', got %s", rr.Body.String())
	}

	// Verify logging
	startEntry := testLogger.GetEntriesWithMessage("HTTP request started")
	if len(startEntry) != 1 {
		t.Errorf("Expected 1 start entry, got %d", len(startEntry))
	}

	completionEntry := testLogger.GetEntriesWithMessage("HTTP request completed successfully")
	if len(completionEntry) != 1 {
		t.Errorf("Expected 1 completion entry, got %d", len(completionEntry))
	}

	// Verify log details
	if len(startEntry) > 0 {
		attrs := startEntry[0].Attrs
		if method, ok := attrs["method"].(string); !ok || method != "GET" {
			t.Errorf("Expected method GET in start log, got %v", attrs["method"])
		}
		if path, ok := attrs["path"].(string); !ok || path != "/test" {
			t.Errorf("Expected path /test in start log, got %v", attrs["path"])
		}
		if userAgent, ok := attrs["user_agent"].(string); !ok || userAgent != "test-agent" {
			t.Errorf("Expected user agent test-agent in start log, got %v", attrs["user_agent"])
		}
	}

	if len(completionEntry) > 0 {
		attrs := completionEntry[0].Attrs
		if statusCode, ok := attrs["status_code"].(float64); !ok || int(statusCode) != 200 {
			t.Errorf("Expected status code 200 in completion log, got %v", attrs["status_code"])
		}
	}
}

func TestRequestInterceptor_HTTPMiddleware_Error(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	// Create a test handler that returns an error status
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	})

	// Wrap with middleware
	middleware := interceptor.HTTPMiddleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/notfound", nil)
	rr := httptest.NewRecorder()

	// Execute request
	middleware.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}

	// Verify error logging
	errorEntry := testLogger.GetEntriesWithMessage("HTTP request completed with error")
	if len(errorEntry) != 1 {
		t.Errorf("Expected 1 error entry, got %d", len(errorEntry))
	}

	if len(errorEntry) > 0 {
		attrs := errorEntry[0].Attrs
		if statusCode, ok := attrs["status_code"].(float64); !ok || int(statusCode) != 404 {
			t.Errorf("Expected status code 404 in error log, got %v", attrs["status_code"])
		}
	}
}

func TestRequestInterceptor_HTTPMiddleware_Panic(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	panicMsg := "handler panic"

	// Create a test handler that panics
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(panicMsg)
	})

	// Wrap with middleware
	middleware := interceptor.HTTPMiddleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/panic", nil)
	rr := httptest.NewRecorder()

	// Execute request - middleware should catch panic and return 500
	middleware.ServeHTTP(rr, req)

	// Verify panic logging
	panicEntry := testLogger.GetEntriesWithMessage("HTTP request panicked")
	if len(panicEntry) != 1 {
		t.Errorf("Expected 1 panic entry, got %d", len(panicEntry))
	}

	// Verify panic details
	if len(panicEntry) > 0 {
		attrs := panicEntry[0].Attrs
		if panicVal, ok := attrs["panic"]; !ok || panicVal != panicMsg {
			t.Errorf("Expected panic value %s, got %v", panicMsg, panicVal)
		}
		if stackTrace, ok := attrs["stack_trace"].(string); !ok || stackTrace == "" {
			t.Error("Expected stack trace to be logged")
		}
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rr,
		statusCode:     http.StatusOK,
	}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code 404, got %d", rw.statusCode)
	}

	if !rw.headerWritten {
		t.Error("Expected headerWritten to be true")
	}
}

func TestResponseWriter_Write(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rr,
		statusCode:     http.StatusOK,
	}

	data := []byte("test data")
	n, err := rw.Write(data)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected %d bytes written, got %d", len(data), n)
	}

	if !rw.headerWritten {
		t.Error("Expected headerWritten to be true after write")
	}

	if rr.Body.String() != "test data" {
		t.Errorf("Expected body 'test data', got %s", rr.Body.String())
	}
}

func TestStartTimer(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := context.Background()
	operation := "test-timer"

	timer := StartTimer(ctx, logger, operation)

	if timer == nil {
		t.Error("Expected timer to be created")
	}

	if timer.operation != operation {
		t.Errorf("Expected operation %s, got %s", operation, timer.operation)
	}

	if timer.logger != logger {
		t.Error("Expected logger to be set")
	}

	if timer.startTime.IsZero() {
		t.Error("Expected start time to be set")
	}

	// Verify request context was created if needed
	requestID := GetRequestID(timer.ctx)
	if requestID == "" {
		t.Error("Expected request ID to be set in timer context")
	}

	// Verify start logging
	startEntry := testLogger.GetEntriesWithMessage("Operation started")
	if len(startEntry) != 1 {
		t.Errorf("Expected 1 start entry, got %d", len(startEntry))
	}
}

func TestStartTimer_ExistingContext(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := CreateTestContext()
	originalRequestID := GetRequestID(ctx)
	operation := "test-timer"

	timer := StartTimer(ctx, logger, operation)

	// Verify existing request ID is preserved
	timerRequestID := GetRequestID(timer.ctx)
	if timerRequestID != originalRequestID {
		t.Errorf("Expected request ID %s to be preserved, got %s", originalRequestID, timerRequestID)
	}
}

func TestOperationTimer_End(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := context.Background()
	operation := "test-timer"

	timer := StartTimer(ctx, logger, operation)

	// Wait a bit to measure duration
	time.Sleep(1 * time.Millisecond)

	duration := timer.End()

	if duration <= 0 {
		t.Errorf("Expected positive duration, got %v", duration)
	}

	// Verify completion logging
	completionEntry := testLogger.GetEntriesWithMessage("Operation completed")
	if len(completionEntry) != 1 {
		t.Errorf("Expected 1 completion entry, got %d", len(completionEntry))
	}

	// Verify duration is logged
	if len(completionEntry) > 0 {
		attrs := completionEntry[0].Attrs
		if _, ok := attrs["duration"]; !ok {
			t.Error("Expected duration to be logged")
		}
	}
}

func TestOperationTimer_EndWithError_NoError(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := context.Background()
	operation := "test-timer"

	timer := StartTimer(ctx, logger, operation)

	// Wait a bit to measure duration
	time.Sleep(1 * time.Millisecond)

	duration := timer.EndWithError(nil)

	if duration <= 0 {
		t.Errorf("Expected positive duration, got %v", duration)
	}

	// Verify completion logging (should be debug level for no error)
	completionEntry := testLogger.GetEntriesWithMessage("Operation completed")
	if len(completionEntry) != 1 {
		t.Errorf("Expected 1 completion entry, got %d", len(completionEntry))
	}
}

func TestOperationTimer_EndWithError_WithError(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := context.Background()
	operation := "test-timer"
	expectedErr := errors.New("test error")

	timer := StartTimer(ctx, logger, operation)

	// Wait a bit to measure duration
	time.Sleep(1 * time.Millisecond)

	duration := timer.EndWithError(expectedErr)

	if duration <= 0 {
		t.Errorf("Expected positive duration, got %v", duration)
	}

	// Verify error logging
	errorEntry := testLogger.GetEntriesWithMessage("Operation failed")
	if len(errorEntry) != 1 {
		t.Errorf("Expected 1 error entry, got %d", len(errorEntry))
	}

	// Verify error details
	if len(errorEntry) > 0 {
		attrs := errorEntry[0].Attrs
		if errorMsg, ok := attrs["error"].(string); !ok || errorMsg != "test error" {
			t.Errorf("Expected error 'test error', got %v", attrs["error"])
		}
	}
}

func TestLogLatency_Success(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := CreateTestContext()
	operation := "test-operation"
	duration := 100 * time.Millisecond

	LogLatency(ctx, logger, operation, duration, nil)

	// Verify success logging
	completionEntry := testLogger.GetEntriesWithMessage("Operation completed")
	if len(completionEntry) != 1 {
		t.Errorf("Expected 1 completion entry, got %d", len(completionEntry))
	}

	// Verify duration is logged
	if len(completionEntry) > 0 {
		attrs := completionEntry[0].Attrs
		if _, ok := attrs["duration"]; !ok {
			t.Error("Expected duration to be logged")
		}
		if op, ok := attrs["operation"].(string); !ok || op != operation {
			t.Errorf("Expected operation %s, got %v", operation, attrs["operation"])
		}
	}
}

func TestLogLatency_Error(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := CreateTestContext()
	operation := "test-operation"
	duration := 200 * time.Millisecond
	expectedErr := errors.New("test error")

	LogLatency(ctx, logger, operation, duration, expectedErr)

	// Verify error logging
	errorEntry := testLogger.GetEntriesWithMessage("Operation completed with error")
	if len(errorEntry) != 1 {
		t.Errorf("Expected 1 error entry, got %d", len(errorEntry))
	}

	// Verify error details
	if len(errorEntry) > 0 {
		attrs := errorEntry[0].Attrs
		if errorMsg, ok := attrs["error"].(string); !ok || errorMsg != "test error" {
			t.Errorf("Expected error 'test error', got %v", attrs["error"])
		}
		if op, ok := attrs["operation"].(string); !ok || op != operation {
			t.Errorf("Expected operation %s, got %v", operation, attrs["operation"])
		}
	}
}

func TestHTTPMiddleware_Integration(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	// Create a complex handler chain
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(1 * time.Millisecond)

		// Use the timer within the handler
		timer := StartTimer(r.Context(), logger, "internal-operation")
		time.Sleep(1 * time.Millisecond)
		timer.End()

		// Check request context propagation
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("Expected request ID to be available in handler")
		}

		w.Header().Set("X-Request-ID", requestID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("integration test"))
	})

	// Wrap with middleware
	middleware := interceptor.HTTPMiddleware(handler)

	// Create test request
	req := httptest.NewRequest("POST", "/integration", strings.NewReader("test data"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Execute request
	middleware.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	requestID := rr.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID header to be set")
	}

	// Verify comprehensive logging
	entries := testLogger.GetEntries()
	if len(entries) < 4 {
		t.Errorf("Expected at least 4 log entries (HTTP start, operation start, operation end, HTTP end), got %d", len(entries))
	}

	// Check HTTP start
	httpStart := testLogger.GetEntriesWithMessage("HTTP request started")
	if len(httpStart) != 1 {
		t.Errorf("Expected 1 HTTP start entry, got %d", len(httpStart))
	}

	// Check operation start
	opStart := testLogger.GetEntriesWithMessage("Operation started")
	if len(opStart) != 1 {
		t.Errorf("Expected 1 operation start entry, got %d", len(opStart))
	}

	// Check operation completion
	opEnd := testLogger.GetEntriesWithMessage("Operation completed")
	if len(opEnd) != 1 {
		t.Errorf("Expected 1 operation completion entry, got %d", len(opEnd))
	}

	// Check HTTP completion
	httpEnd := testLogger.GetEntriesWithMessage("HTTP request completed successfully")
	if len(httpEnd) != 1 {
		t.Errorf("Expected 1 HTTP completion entry, got %d", len(httpEnd))
	}

	// Verify request ID consistency across all entries
	for i, entry := range entries {
		if entry.RequestID != requestID {
			t.Errorf("Entry %d has inconsistent request ID: expected %s, got %s", i, requestID, entry.RequestID)
		}
	}
}

// Benchmark tests
func BenchmarkRequestInterceptor_InterceptRequest(b *testing.B) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	ctx := context.Background()
	operation := "benchmark"

	fn := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interceptor.InterceptRequest(ctx, operation, fn)
	}
}

func BenchmarkMiddleware_HTTPMiddleware(b *testing.B) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := interceptor.HTTPMiddleware(handler)

	req := httptest.NewRequest("GET", "/benchmark", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
	}
}

func BenchmarkMiddleware_OperationTimer(b *testing.B) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timer := StartTimer(ctx, logger, "benchmark")
		timer.End()
	}
}

func TestConcurrentMiddlewareAccess(t *testing.T) {
	testLogger := NewTestLogger()
	logger := testLogger.GetLogger()
	interceptor := NewRequestInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("Expected request ID in concurrent request")
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := interceptor.HTTPMiddleware(handler)

	const numRequests = 100
	done := make(chan struct{}, numRequests)

	// Make concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			req := httptest.NewRequest("GET", fmt.Sprintf("/test-%d", id), nil)
			rr := httptest.NewRecorder()

			middleware.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Request %d failed with status %d", id, rr.Code)
			}
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Verify logging integrity
	startEntries := testLogger.GetEntriesWithMessage("HTTP request started")
	endEntries := testLogger.GetEntriesWithMessage("HTTP request completed successfully")

	if len(startEntries) != numRequests {
		t.Errorf("Expected %d start entries, got %d", numRequests, len(startEntries))
	}

	if len(endEntries) != numRequests {
		t.Errorf("Expected %d end entries, got %d", numRequests, len(endEntries))
	}

	// Verify all request IDs are unique
	requestIDs := make(map[string]bool)
	for _, entry := range startEntries {
		if entry.RequestID == "" {
			t.Error("Found empty request ID in concurrent test")
			continue
		}
		if requestIDs[entry.RequestID] {
			t.Errorf("Found duplicate request ID: %s", entry.RequestID)
		}
		requestIDs[entry.RequestID] = true
	}
}