package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Context keys for logging metadata
type contextKey string

const (
	contextKeyRequestID  contextKey = "request_id"
	contextKeyTraceID    contextKey = "trace_id"
	contextKeySpanID     contextKey = "span_id"
	contextKeyUserID     contextKey = "user_id"
	contextKeySessionID  contextKey = "session_id"
	contextKeyOperation  contextKey = "operation"
	contextKeyComponent  contextKey = "component"
	contextKeyStartTime  contextKey = "start_time"
)

// RequestContext holds request-scoped metadata
type RequestContext struct {
	RequestID  string
	TraceID    string
	SpanID     string
	UserID     string
	SessionID  string
	Operation  string
	Component  string
	StartTime  time.Time
	Metadata   map[string]interface{}
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, requestID)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if val := ctx.Value(contextKeyRequestID); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	// Also check for common variations
	if val := ctx.Value("requestId"); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	if val := ctx.Value("req_id"); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKeyTraceID, traceID)
}

// GetTraceID retrieves the trace ID from context
func GetTraceID(ctx context.Context) string {
	if val := ctx.Value(contextKeyTraceID); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	// Also check for common variations
	if val := ctx.Value("trace_id"); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// WithSpanID adds a span ID to the context
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, contextKeySpanID, spanID)
}

// GetSpanID retrieves the span ID from context
func GetSpanID(ctx context.Context) string {
	if val := ctx.Value(contextKeySpanID); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) string {
	if val := ctx.Value(contextKeyUserID); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	// Also check for common variations
	if val := ctx.Value("user_id"); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	if val := ctx.Value("userId"); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// WithSessionID adds a session ID to the context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, contextKeySessionID, sessionID)
}

// GetSessionID retrieves the session ID from context
func GetSessionID(ctx context.Context) string {
	if val := ctx.Value(contextKeySessionID); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// WithOperation adds an operation name to the context
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, contextKeyOperation, operation)
}

// GetOperation retrieves the operation from context
func GetOperation(ctx context.Context) string {
	if val := ctx.Value(contextKeyOperation); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// WithComponent adds a component name to the context
func WithComponent(ctx context.Context, component string) context.Context {
	return context.WithValue(ctx, contextKeyComponent, component)
}

// GetComponent retrieves the component from context
func GetComponent(ctx context.Context) string {
	if val := ctx.Value(contextKeyComponent); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// WithStartTime adds the start time to the context
func WithStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, contextKeyStartTime, startTime)
}

// GetStartTime retrieves the start time from context
func GetStartTime(ctx context.Context) time.Time {
	if val := ctx.Value(contextKeyStartTime); val != nil {
		if t, ok := val.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}

// GetDuration calculates the duration since start time
func GetDuration(ctx context.Context) time.Duration {
	startTime := GetStartTime(ctx)
	if startTime.IsZero() {
		return 0
	}
	return time.Since(startTime)
}

// NewRequestContext creates a new request context with generated IDs
func NewRequestContext(ctx context.Context, operation string) context.Context {
	// Generate request ID if not present
	requestID := GetRequestID(ctx)
	if requestID == "" {
		requestID = GenerateID()
		ctx = WithRequestID(ctx, requestID)
	}
	
	// Generate trace ID if not present
	traceID := GetTraceID(ctx)
	if traceID == "" {
		traceID = GenerateTraceID()
		ctx = WithTraceID(ctx, traceID)
	}
	
	// Generate span ID
	spanID := GenerateSpanID()
	ctx = WithSpanID(ctx, spanID)
	
	// Set operation
	if operation != "" {
		ctx = WithOperation(ctx, operation)
	}
	
	// Set start time
	ctx = WithStartTime(ctx, time.Now())
	
	return ctx
}

// ExtractRequestContext extracts all request context into a struct
func ExtractRequestContext(ctx context.Context) *RequestContext {
	return &RequestContext{
		RequestID:  GetRequestID(ctx),
		TraceID:    GetTraceID(ctx),
		SpanID:     GetSpanID(ctx),
		UserID:     GetUserID(ctx),
		SessionID:  GetSessionID(ctx),
		Operation:  GetOperation(ctx),
		Component:  GetComponent(ctx),
		StartTime:  GetStartTime(ctx),
	}
}

// InjectRequestContext injects a RequestContext into a context
func InjectRequestContext(ctx context.Context, rc *RequestContext) context.Context {
	if rc == nil {
		return ctx
	}
	
	if rc.RequestID != "" {
		ctx = WithRequestID(ctx, rc.RequestID)
	}
	if rc.TraceID != "" {
		ctx = WithTraceID(ctx, rc.TraceID)
	}
	if rc.SpanID != "" {
		ctx = WithSpanID(ctx, rc.SpanID)
	}
	if rc.UserID != "" {
		ctx = WithUserID(ctx, rc.UserID)
	}
	if rc.SessionID != "" {
		ctx = WithSessionID(ctx, rc.SessionID)
	}
	if rc.Operation != "" {
		ctx = WithOperation(ctx, rc.Operation)
	}
	if rc.Component != "" {
		ctx = WithComponent(ctx, rc.Component)
	}
	if !rc.StartTime.IsZero() {
		ctx = WithStartTime(ctx, rc.StartTime)
	}
	
	return ctx
}

// GenerateID generates a random ID for requests
func GenerateID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// GenerateTraceID generates a 128-bit trace ID (W3C Trace Context format)
func GenerateTraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// GenerateSpanID generates a 64-bit span ID (W3C Trace Context format)
func GenerateSpanID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// ParseTraceParent parses a W3C Trace Context traceparent header
func ParseTraceParent(header string) (traceID, spanID string, sampled bool) {
	// Format: version-trace_id-parent_id-trace_flags
	// Example: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
	
	if len(header) < 55 {
		return "", "", false
	}
	
	if header[0:3] != "00-" {
		return "", "", false // Unsupported version
	}
	
	traceID = header[3:35]
	spanID = header[36:52]
	
	if len(header) >= 55 && header[53:55] == "01" {
		sampled = true
	}
	
	return traceID, spanID, sampled
}

// FormatTraceParent formats a W3C Trace Context traceparent header
func FormatTraceParent(traceID, spanID string, sampled bool) string {
	flags := "00"
	if sampled {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", traceID, spanID, flags)
}