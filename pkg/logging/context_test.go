package logging

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-123"

	ctx = WithRequestID(ctx, requestID)
	retrieved := GetRequestID(ctx)

	if retrieved != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, retrieved)
	}
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(context.Context) context.Context
		expected string
	}{
		{
			name: "standard key",
			setup: func(ctx context.Context) context.Context {
				return WithRequestID(ctx, "standard-123")
			},
			expected: "standard-123",
		},
		{
			name: "requestId variation",
			setup: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "requestId", "variation-123")
			},
			expected: "variation-123",
		},
		{
			name: "req_id variation",
			setup: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "req_id", "req-123")
			},
			expected: "req-123",
		},
		{
			name: "empty context",
			setup: func(ctx context.Context) context.Context {
				return ctx
			},
			expected: "",
		},
		{
			name: "non-string value",
			setup: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, contextKeyRequestID, 12345)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = tt.setup(ctx)
			result := GetRequestID(ctx)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "test-trace-456"

	ctx = WithTraceID(ctx, traceID)
	retrieved := GetTraceID(ctx)

	if retrieved != traceID {
		t.Errorf("Expected trace ID %s, got %s", traceID, retrieved)
	}
}

func TestGetTraceID(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(context.Context) context.Context
		expected string
	}{
		{
			name: "standard key",
			setup: func(ctx context.Context) context.Context {
				return WithTraceID(ctx, "trace-123")
			},
			expected: "trace-123",
		},
		{
			name: "trace_id variation",
			setup: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "trace_id", "trace-456")
			},
			expected: "trace-456",
		},
		{
			name: "empty context",
			setup: func(ctx context.Context) context.Context {
				return ctx
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = tt.setup(ctx)
			result := GetTraceID(ctx)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWithSpanID(t *testing.T) {
	ctx := context.Background()
	spanID := "test-span-789"

	ctx = WithSpanID(ctx, spanID)
	retrieved := GetSpanID(ctx)

	if retrieved != spanID {
		t.Errorf("Expected span ID %s, got %s", spanID, retrieved)
	}
}

func TestGetSpanID(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	result := GetSpanID(ctx)
	if result != "" {
		t.Errorf("Expected empty span ID, got %s", result)
	}

	// Test with span ID
	spanID := "span-123"
	ctx = WithSpanID(ctx, spanID)
	result = GetSpanID(ctx)
	if result != spanID {
		t.Errorf("Expected span ID %s, got %s", spanID, result)
	}
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	userID := "user-123"

	ctx = WithUserID(ctx, userID)
	retrieved := GetUserID(ctx)

	if retrieved != userID {
		t.Errorf("Expected user ID %s, got %s", userID, retrieved)
	}
}

func TestGetUserID(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(context.Context) context.Context
		expected string
	}{
		{
			name: "standard key",
			setup: func(ctx context.Context) context.Context {
				return WithUserID(ctx, "user-123")
			},
			expected: "user-123",
		},
		{
			name: "user_id variation",
			setup: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "user_id", "user-456")
			},
			expected: "user-456",
		},
		{
			name: "userId variation",
			setup: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "userId", "user-789")
			},
			expected: "user-789",
		},
		{
			name: "empty context",
			setup: func(ctx context.Context) context.Context {
				return ctx
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = tt.setup(ctx)
			result := GetUserID(ctx)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWithSessionID(t *testing.T) {
	ctx := context.Background()
	sessionID := "session-123"

	ctx = WithSessionID(ctx, sessionID)
	retrieved := GetSessionID(ctx)

	if retrieved != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, retrieved)
	}
}

func TestGetSessionID(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	result := GetSessionID(ctx)
	if result != "" {
		t.Errorf("Expected empty session ID, got %s", result)
	}

	// Test with session ID
	sessionID := "session-123"
	ctx = WithSessionID(ctx, sessionID)
	result = GetSessionID(ctx)
	if result != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, result)
	}
}

func TestWithOperation(t *testing.T) {
	ctx := context.Background()
	operation := "test-operation"

	ctx = WithOperation(ctx, operation)
	retrieved := GetOperation(ctx)

	if retrieved != operation {
		t.Errorf("Expected operation %s, got %s", operation, retrieved)
	}
}

func TestGetOperation(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	result := GetOperation(ctx)
	if result != "" {
		t.Errorf("Expected empty operation, got %s", result)
	}

	// Test with operation
	operation := "create_entity"
	ctx = WithOperation(ctx, operation)
	result = GetOperation(ctx)
	if result != operation {
		t.Errorf("Expected operation %s, got %s", operation, result)
	}
}

func TestWithComponent(t *testing.T) {
	ctx := context.Background()
	component := "test-component"

	ctx = WithComponent(ctx, component)
	retrieved := GetComponent(ctx)

	if retrieved != component {
		t.Errorf("Expected component %s, got %s", component, retrieved)
	}
}

func TestGetComponent(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	result := GetComponent(ctx)
	if result != "" {
		t.Errorf("Expected empty component, got %s", result)
	}

	// Test with component
	component := "database"
	ctx = WithComponent(ctx, component)
	result = GetComponent(ctx)
	if result != component {
		t.Errorf("Expected component %s, got %s", component, result)
	}
}

func TestWithStartTime(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now()

	ctx = WithStartTime(ctx, startTime)
	retrieved := GetStartTime(ctx)

	if !retrieved.Equal(startTime) {
		t.Errorf("Expected start time %v, got %v", startTime, retrieved)
	}
}

func TestGetStartTime(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	result := GetStartTime(ctx)
	if !result.IsZero() {
		t.Errorf("Expected zero time, got %v", result)
	}

	// Test with start time
	startTime := time.Now()
	ctx = WithStartTime(ctx, startTime)
	result = GetStartTime(ctx)
	if !result.Equal(startTime) {
		t.Errorf("Expected start time %v, got %v", startTime, result)
	}

	// Test with invalid value type
	ctx = context.WithValue(ctx, contextKeyStartTime, "not-a-time")
	result = GetStartTime(ctx)
	if !result.IsZero() {
		t.Errorf("Expected zero time for invalid value, got %v", result)
	}
}

func TestGetDuration(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	duration := GetDuration(ctx)
	if duration != 0 {
		t.Errorf("Expected zero duration, got %v", duration)
	}

	// Test with start time
	startTime := time.Now()
	ctx = WithStartTime(ctx, startTime)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	duration = GetDuration(ctx)
	if duration <= 0 {
		t.Errorf("Expected positive duration, got %v", duration)
	}

	if duration < 10*time.Millisecond {
		t.Errorf("Expected duration >= 10ms, got %v", duration)
	}
}

func TestNewRequestContext(t *testing.T) {
	ctx := context.Background()
	operation := "test-operation"

	newCtx := NewRequestContext(ctx, operation)

	// Test that all required fields are set
	requestID := GetRequestID(newCtx)
	if requestID == "" {
		t.Error("Expected request ID to be generated")
	}

	traceID := GetTraceID(newCtx)
	if traceID == "" {
		t.Error("Expected trace ID to be generated")
	}

	spanID := GetSpanID(newCtx)
	if spanID == "" {
		t.Error("Expected span ID to be generated")
	}

	retrievedOperation := GetOperation(newCtx)
	if retrievedOperation != operation {
		t.Errorf("Expected operation %s, got %s", operation, retrievedOperation)
	}

	startTime := GetStartTime(newCtx)
	if startTime.IsZero() {
		t.Error("Expected start time to be set")
	}

	// Test that existing values are preserved
	existingCtx := WithRequestID(ctx, "existing-request")
	existingCtx = WithTraceID(existingCtx, "existing-trace")

	newCtx = NewRequestContext(existingCtx, operation)

	if GetRequestID(newCtx) != "existing-request" {
		t.Error("Expected existing request ID to be preserved")
	}

	if GetTraceID(newCtx) != "existing-trace" {
		t.Error("Expected existing trace ID to be preserved")
	}
}

func TestExtractRequestContext(t *testing.T) {
	ctx := CreateTestContextWithValues("req-123", "trace-456", "user-789", "test-op", "test-comp")

	rc := ExtractRequestContext(ctx)

	if rc == nil {
		t.Fatal("Expected request context to be extracted")
	}

	if rc.RequestID != "req-123" {
		t.Errorf("Expected request ID req-123, got %s", rc.RequestID)
	}

	if rc.TraceID != "trace-456" {
		t.Errorf("Expected trace ID trace-456, got %s", rc.TraceID)
	}

	if rc.UserID != "user-789" {
		t.Errorf("Expected user ID user-789, got %s", rc.UserID)
	}

	if rc.Operation != "test-op" {
		t.Errorf("Expected operation test-op, got %s", rc.Operation)
	}

	if rc.Component != "test-comp" {
		t.Errorf("Expected component test-comp, got %s", rc.Component)
	}

	if rc.StartTime.IsZero() {
		t.Error("Expected start time to be set")
	}
}

func TestInjectRequestContext(t *testing.T) {
	ctx := context.Background()

	rc := &RequestContext{
		RequestID:  "req-123",
		TraceID:    "trace-456",
		SpanID:     "span-789",
		UserID:     "user-abc",
		SessionID:  "session-def",
		Operation:  "test-op",
		Component:  "test-comp",
		StartTime:  time.Now(),
	}

	newCtx := InjectRequestContext(ctx, rc)

	if GetRequestID(newCtx) != rc.RequestID {
		t.Errorf("Expected request ID %s, got %s", rc.RequestID, GetRequestID(newCtx))
	}

	if GetTraceID(newCtx) != rc.TraceID {
		t.Errorf("Expected trace ID %s, got %s", rc.TraceID, GetTraceID(newCtx))
	}

	if GetSpanID(newCtx) != rc.SpanID {
		t.Errorf("Expected span ID %s, got %s", rc.SpanID, GetSpanID(newCtx))
	}

	if GetUserID(newCtx) != rc.UserID {
		t.Errorf("Expected user ID %s, got %s", rc.UserID, GetUserID(newCtx))
	}

	if GetSessionID(newCtx) != rc.SessionID {
		t.Errorf("Expected session ID %s, got %s", rc.SessionID, GetSessionID(newCtx))
	}

	if GetOperation(newCtx) != rc.Operation {
		t.Errorf("Expected operation %s, got %s", rc.Operation, GetOperation(newCtx))
	}

	if GetComponent(newCtx) != rc.Component {
		t.Errorf("Expected component %s, got %s", rc.Component, GetComponent(newCtx))
	}

	startTime := GetStartTime(newCtx)
	if !startTime.Equal(rc.StartTime) {
		t.Errorf("Expected start time %v, got %v", rc.StartTime, startTime)
	}

	// Test with nil context
	nilCtx := InjectRequestContext(ctx, nil)
	if nilCtx != ctx {
		t.Error("Expected original context to be returned when rc is nil")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}

	if id2 == "" {
		t.Error("Expected non-empty ID")
	}

	if id1 == id2 {
		t.Error("Expected different IDs to be generated")
	}

	// Check that ID is hexadecimal
	if len(id1) != 16 { // 8 bytes = 16 hex characters
		t.Errorf("Expected ID length of 16, got %d", len(id1))
	}

	for _, char := range id1 {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			t.Errorf("Expected hexadecimal character, got %c", char)
		}
	}
}

func TestGenerateTraceID(t *testing.T) {
	traceID1 := GenerateTraceID()
	traceID2 := GenerateTraceID()

	if traceID1 == "" {
		t.Error("Expected non-empty trace ID")
	}

	if traceID2 == "" {
		t.Error("Expected non-empty trace ID")
	}

	if traceID1 == traceID2 {
		t.Error("Expected different trace IDs to be generated")
	}

	// Check that trace ID is 128-bit (32 hex characters)
	if len(traceID1) != 32 {
		t.Errorf("Expected trace ID length of 32, got %d", len(traceID1))
	}

	for _, char := range traceID1 {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			t.Errorf("Expected hexadecimal character, got %c", char)
		}
	}
}

func TestGenerateSpanID(t *testing.T) {
	spanID1 := GenerateSpanID()
	spanID2 := GenerateSpanID()

	if spanID1 == "" {
		t.Error("Expected non-empty span ID")
	}

	if spanID2 == "" {
		t.Error("Expected non-empty span ID")
	}

	if spanID1 == spanID2 {
		t.Error("Expected different span IDs to be generated")
	}

	// Check that span ID is 64-bit (16 hex characters)
	if len(spanID1) != 16 {
		t.Errorf("Expected span ID length of 16, got %d", len(spanID1))
	}

	for _, char := range spanID1 {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			t.Errorf("Expected hexadecimal character, got %c", char)
		}
	}
}

func TestParseTraceParent(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		expectTrace string
		expectSpan  string
		expectSample bool
	}{
		{
			name:         "valid sampled",
			header:       "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			expectTrace:  "4bf92f3577b34da6a3ce929d0e0e4736",
			expectSpan:   "00f067aa0ba902b7",
			expectSample: true,
		},
		{
			name:         "valid not sampled",
			header:       "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
			expectTrace:  "4bf92f3577b34da6a3ce929d0e0e4736",
			expectSpan:   "00f067aa0ba902b7",
			expectSample: false,
		},
		{
			name:         "too short",
			header:       "00-4bf92f35",
			expectTrace:  "",
			expectSpan:   "",
			expectSample: false,
		},
		{
			name:         "wrong version",
			header:       "01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			expectTrace:  "",
			expectSpan:   "",
			expectSample: false,
		},
		{
			name:         "empty",
			header:       "",
			expectTrace:  "",
			expectSpan:   "",
			expectSample: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceID, spanID, sampled := ParseTraceParent(tt.header)

			if traceID != tt.expectTrace {
				t.Errorf("Expected trace ID %s, got %s", tt.expectTrace, traceID)
			}

			if spanID != tt.expectSpan {
				t.Errorf("Expected span ID %s, got %s", tt.expectSpan, spanID)
			}

			if sampled != tt.expectSample {
				t.Errorf("Expected sampled %v, got %v", tt.expectSample, sampled)
			}
		})
	}
}

func TestFormatTraceParent(t *testing.T) {
	tests := []struct {
		name     string
		traceID  string
		spanID   string
		sampled  bool
		expected string
	}{
		{
			name:     "sampled",
			traceID:  "4bf92f3577b34da6a3ce929d0e0e4736",
			spanID:   "00f067aa0ba902b7",
			sampled:  true,
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "not sampled",
			traceID:  "4bf92f3577b34da6a3ce929d0e0e4736",
			spanID:   "00f067aa0ba902b7",
			sampled:  false,
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTraceParent(tt.traceID, tt.spanID, tt.sampled)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestTraceParentRoundTrip(t *testing.T) {
	// Test that parse and format are inverse operations
	originalHeader := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	traceID, spanID, sampled := ParseTraceParent(originalHeader)
	reconstructed := FormatTraceParent(traceID, spanID, sampled)

	if reconstructed != originalHeader {
		t.Errorf("Round trip failed: expected %s, got %s", originalHeader, reconstructed)
	}
}

func TestRequestContextLifecycle(t *testing.T) {
	// Simulate a complete request lifecycle
	ctx := context.Background()

	// Start request
	ctx = NewRequestContext(ctx, "create_entity")
	requestID := GetRequestID(ctx)
	
	// Add user context
	ctx = WithUserID(ctx, "user-123")
	ctx = WithSessionID(ctx, "session-456")

	// Simulate processing time
	time.Sleep(1 * time.Millisecond)

	// Extract context for audit
	rc := ExtractRequestContext(ctx)

	// Verify all fields are present
	if rc.RequestID != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, rc.RequestID)
	}

	if rc.UserID != "user-123" {
		t.Errorf("Expected user ID user-123, got %s", rc.UserID)
	}

	if rc.SessionID != "session-456" {
		t.Errorf("Expected session ID session-456, got %s", rc.SessionID)
	}

	if rc.Operation != "create_entity" {
		t.Errorf("Expected operation create_entity, got %s", rc.Operation)
	}

	// Verify duration calculation
	duration := GetDuration(ctx)
	if duration <= 0 {
		t.Errorf("Expected positive duration, got %v", duration)
	}

	// Test context injection into a new context
	newCtx := InjectRequestContext(context.Background(), rc)
	if GetRequestID(newCtx) != requestID {
		t.Errorf("Expected request ID %s after injection, got %s", requestID, GetRequestID(newCtx))
	}
}

// Benchmark tests
func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateID()
	}
}

func BenchmarkGenerateTraceID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateTraceID()
	}
}

func BenchmarkGenerateSpanID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateSpanID()
	}
}

func BenchmarkNewRequestContext(b *testing.B) {
	ctx := context.Background()
	operation := "benchmark_operation"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewRequestContext(ctx, operation)
	}
}

func BenchmarkExtractRequestContext(b *testing.B) {
	ctx := CreateTestContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractRequestContext(ctx)
	}
}

func BenchmarkInjectRequestContext(b *testing.B) {
	ctx := context.Background()
	rc := ExtractRequestContext(CreateTestContext())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InjectRequestContext(ctx, rc)
	}
}

func BenchmarkParseTraceParent(b *testing.B) {
	header := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseTraceParent(header)
	}
}

func BenchmarkFormatTraceParent(b *testing.B) {
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"
	sampled := true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatTraceParent(traceID, spanID, sampled)
	}
}

func TestConcurrentContextOperations(t *testing.T) {
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			for j := 0; j < numOperations; j++ {
				// Create context with unique values
				ctx := context.Background()
				requestID := fmt.Sprintf("req-%d-%d", id, j)
				userID := fmt.Sprintf("user-%d-%d", id, j)

				ctx = WithRequestID(ctx, requestID)
				ctx = WithUserID(ctx, userID)
				ctx = NewRequestContext(ctx, fmt.Sprintf("op-%d-%d", id, j))

				// Verify values
				if GetRequestID(ctx) != requestID {
					t.Errorf("Request ID mismatch in goroutine %d iteration %d", id, j)
				}

				if GetUserID(ctx) != userID {
					t.Errorf("User ID mismatch in goroutine %d iteration %d", id, j)
				}

				// Test extract/inject cycle
				rc := ExtractRequestContext(ctx)
				newCtx := InjectRequestContext(context.Background(), rc)

				if GetRequestID(newCtx) != requestID {
					t.Errorf("Request ID mismatch after inject in goroutine %d iteration %d", id, j)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}